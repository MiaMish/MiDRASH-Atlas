package httpapi

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"midrash-atlas/apps/api/internal/curationpilot"
	"midrash-atlas/apps/api/internal/gazetteerpilot"
	"midrash-atlas/apps/api/internal/locationstore"
	"midrash-atlas/packages/atlas"
	"midrash-atlas/packages/provenance"
)

type locationOverview struct {
	PlaceID             string            `json:"place_id"`
	PlaceLabel          string            `json:"place_label"`
	Aliases             []string          `json:"aliases"`
	HasValidCoordinates bool              `json:"has_valid_coordinates"`
	CoordinateStatus    string            `json:"coordinate_status"`
	ReviewedByHuman     bool              `json:"reviewed_by_human"`
	ChangedByHuman      bool              `json:"changed_by_human"`
	Geometry            json.RawMessage   `json:"geometry,omitempty"`
	GeometryVariantID   string            `json:"geometry_variant_id,omitempty"`
	GeometryLabel       string            `json:"geometry_label,omitempty"`
	SpatialPrecision    string            `json:"spatial_precision,omitempty"`
	InterpretationNote  string            `json:"interpretation_note,omitempty"`
	GeometrySource      string            `json:"geometry_source,omitempty"`
	ReviewStatus        string            `json:"review_status"`
	LatestRevision      int               `json:"latest_revision,omitempty"`
	AIProvenance        *provenance.AIRun `json:"ai_provenance,omitempty"`
	AIStatus            string            `json:"ai_status"`
	AIComment           string            `json:"ai_comment,omitempty"`
}

func (s *server) locationOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET required"))
		return
	}
	var placeDocument struct {
		Places []atlas.PlaceConcept `json:"places"`
	}
	if err := readJSONFile(filepath.Join(s.cfg.ExportDir, "places.json"), &placeDocument); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	locations := make(map[string]*locationOverview, len(placeDocument.Places))
	for _, place := range placeDocument.Places {
		item := &locationOverview{
			PlaceID: place.ID, PlaceLabel: place.PreferredLabel, Aliases: place.Aliases,
			CoordinateStatus: "no_geometry", ReviewStatus: "missing_geometry",
			AIStatus: "not_attempted",
		}
		for _, variant := range place.GeometryVariants {
			if !validGeoJSON(variant.Geometry) {
				continue
			}
			item.setGeometry(variant.Geometry, variant.ID, variant.Label, variant.SpatialPrecision,
				variant.InterpretationNote, "curated_geometry", variant.ReviewStatus)
			item.ReviewedByHuman = isHumanReviewed(variant.ReviewStatus)
			if item.ReviewedByHuman {
				item.CoordinateStatus = "reviewed_by_human"
			} else {
				item.CoordinateStatus = "curated_unreviewed"
			}
			break
		}
		locations[place.ID] = item
	}
	s.applyPilotCandidates(locations)
	revisions, err := s.locations.ListCurrentAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	latest := map[string]locationstore.GeometryRevision{}
	for _, revision := range revisions {
		current, ok := latest[revision.PlaceID]
		if !ok || revision.CreatedAt.After(current.CreatedAt) {
			latest[revision.PlaceID] = revision
		}
	}
	for placeID, revision := range latest {
		item := locations[placeID]
		if item == nil || !validGeoJSON(revision.Geometry) {
			continue
		}
		item.setGeometry(revision.Geometry, revision.GeometryVariantID, revision.Label,
			revision.SpatialPrecision, revision.InterpretationNote, "human_revision", revision.ReviewStatus)
		item.LatestRevision = revision.Revision
		item.AIProvenance = revision.AIProvenance
		item.ReviewedByHuman = isHumanReviewed(revision.ReviewStatus)
		item.ChangedByHuman = revision.HumanAction == "changed"
		switch {
		case item.ChangedByHuman && item.ReviewedByHuman:
			item.CoordinateStatus = "changed_and_reviewed_by_human"
		case item.ChangedByHuman:
			item.CoordinateStatus = "changed_by_human"
		case item.ReviewedByHuman:
			item.CoordinateStatus = "reviewed_by_human"
		default:
			item.CoordinateStatus = "human_draft"
		}
	}
	out := make([]locationOverview, 0, len(locations))
	for _, item := range locations {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].PlaceLabel) < strings.ToLower(out[j].PlaceLabel)
	})
	writeJSON(w, http.StatusOK, map[string]any{"locations": out})
}

func (s *server) applyPilotCandidates(locations map[string]*locationOverview) {
	if strings.TrimSpace(s.cfg.GazetteerPilotPath) == "" || strings.TrimSpace(s.cfg.CurationDraftsPath) == "" {
		return
	}
	var candidates gazetteerpilot.Output
	var drafts curationpilot.Output
	if readJSONFile(s.cfg.GazetteerPilotPath, &candidates) != nil ||
		readJSONFile(s.cfg.CurationDraftsPath, &drafts) != nil {
		return
	}
	candidatesByPlace := make(map[string]gazetteerpilot.OutputPlace, len(candidates.Places))
	for _, place := range candidates.Places {
		candidatesByPlace[place.PlaceID] = place
	}
	for _, draft := range drafts.Drafts {
		item := locations[draft.PlaceID]
		if item == nil {
			continue
		}
		if draft.Error != "" {
			item.AIStatus = "technical_failure"
			item.AIComment = "The AI curation request failed before producing a usable draft."
			continue
		}
		if draft.Result == nil {
			item.AIStatus = "technical_failure"
			item.AIComment = "The AI curation request produced no result."
			continue
		}
		item.AIProvenance = &draft.Result.AIProvenance
		item.AIComment = draft.Result.Draft.Rationale
		switch draft.Result.Draft.Status {
		case "candidate_selected":
			item.AIStatus = "candidate_proposed"
		case "needs_candidates":
			item.AIStatus = "needs_candidates"
			continue
		case "ambiguous":
			item.AIStatus = "ambiguous"
			continue
		default:
			item.AIStatus = "technical_failure"
			continue
		}
		if item.HasValidCoordinates || draft.Result.Draft.RecommendedCandidateID == nil {
			continue
		}
		pilotPlace, ok := candidatesByPlace[draft.PlaceID]
		if !ok {
			continue
		}
		for _, candidate := range pilotPlace.CurationRequest.GazetteerCandidates {
			if candidate.ID != *draft.Result.Draft.RecommendedCandidateID || !validGeoJSON(candidate.Geometry) {
				continue
			}
			item.setGeometry(candidate.Geometry, "pilot_"+candidate.ID,
				"Unreviewed gazetteer candidate for "+item.PlaceLabel,
				draft.Result.Draft.SpatialPrecision, draft.Result.Draft.Rationale,
				"ollama_gazetteer_draft", "unreviewed")
			item.CoordinateStatus = "unreviewed_candidate"
			break
		}
	}
}

func (item *locationOverview) setGeometry(
	geometry json.RawMessage,
	variantID, label, precision, note, source, reviewStatus string,
) {
	item.Geometry = append(json.RawMessage(nil), geometry...)
	item.GeometryVariantID = variantID
	item.GeometryLabel = label
	item.SpatialPrecision = precision
	item.InterpretationNote = note
	item.GeometrySource = source
	item.ReviewStatus = reviewStatus
	item.HasValidCoordinates = true
}

func isHumanReviewed(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "reviewed", "reviewed_by_human", "approved":
		return true
	default:
		return false
	}
}

func readJSONFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func validGeoJSON(raw json.RawMessage) bool {
	var geometry struct {
		Type        string `json:"type"`
		Coordinates any    `json:"coordinates"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &geometry) != nil {
		return false
	}
	switch geometry.Type {
	case "Point":
		return validPosition(geometry.Coordinates)
	case "Polygon":
		return validCoordinates(geometry.Coordinates, 2)
	case "MultiPolygon":
		return validCoordinates(geometry.Coordinates, 3)
	default:
		return false
	}
}

func validCoordinates(value any, depth int) bool {
	if depth == 0 {
		return validPosition(value)
	}
	values, ok := value.([]any)
	if !ok || len(values) == 0 {
		return false
	}
	for _, child := range values {
		if !validCoordinates(child, depth-1) {
			return false
		}
	}
	return true
}

func validPosition(value any) bool {
	position, ok := value.([]any)
	if !ok || len(position) < 2 {
		return false
	}
	longitude, lonOK := position[0].(float64)
	latitude, latOK := position[1].(float64)
	return lonOK && latOK && !math.IsNaN(longitude) && !math.IsNaN(latitude) &&
		longitude >= -180 && longitude <= 180 && latitude >= -90 && latitude <= 90
}
