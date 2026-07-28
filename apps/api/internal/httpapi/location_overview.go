package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	AIHistory           []aiAuditEntry    `json:"ai_history"`
}

type aiAuditEntry struct {
	RunID       string    `json:"run_id"`
	GeneratedAt time.Time `json:"generated_at"`
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	Status      string    `json:"status"`
	Comment     string    `json:"comment,omitempty"`
	Error       string    `json:"error,omitempty"`
	PromptHash  string    `json:"prompt_hash,omitempty"`
	Current     bool      `json:"current"`
}

func (s *server) locationOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET required"))
		return
	}
	out, err := s.buildLocationOverview(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"locations": out})
}

func (s *server) buildLocationOverview(ctx context.Context) ([]locationOverview, error) {
	var placeDocument struct {
		Places []atlas.PlaceConcept `json:"places"`
	}
	if err := readJSONFile(filepath.Join(s.cfg.ExportDir, "places.json"), &placeDocument); err != nil {
		return nil, err
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
	revisions, err := s.locations.ListCurrentAll(ctx)
	if err != nil {
		return nil, err
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
	return out, nil
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
	applyAIHistory(locations, drafts)
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

func applyAIHistory(locations map[string]*locationOverview, output curationpilot.Output) {
	for _, run := range output.Runs {
		for _, draft := range run.Results {
			item := locations[draft.PlaceID]
			if item == nil {
				continue
			}
			entry := aiAuditEntry{
				RunID: run.ID, GeneratedAt: run.StartedAt,
				Provider: run.Provider, Model: run.Model,
				Status: "technical_failure", Error: draft.Error,
			}
			if draft.Result != nil {
				entry.GeneratedAt = draft.Result.AIProvenance.GeneratedAt
				entry.Provider = draft.Result.AIProvenance.Provider
				entry.Model = draft.Result.AIProvenance.Model
				entry.Status = aiStatus(draft)
				entry.Comment = draft.Result.Draft.Rationale
				entry.PromptHash = draft.Result.AIProvenance.PromptHash
			}
			item.AIHistory = append(item.AIHistory, entry)
		}
		for _, skipped := range run.Skipped {
			if item := locations[skipped.PlaceID]; item != nil {
				status := skipped.Code
				if status == "" {
					status = "skipped"
				}
				item.AIHistory = append(item.AIHistory, aiAuditEntry{
					RunID: run.ID, GeneratedAt: run.StartedAt,
					Provider: run.Provider, Model: run.Model,
					Status: status, Comment: skipped.Reason,
				})
			}
		}
	}
	for _, item := range locations {
		if len(item.AIHistory) == 0 {
			for _, draft := range output.Drafts {
				if draft.PlaceID != item.PlaceID {
					continue
				}
				entry := aiAuditEntry{
					RunID: "legacy_current_result", Status: aiStatus(draft), Error: draft.Error,
					Current: true,
				}
				if draft.Result != nil {
					entry.GeneratedAt = draft.Result.AIProvenance.GeneratedAt
					entry.Provider = draft.Result.AIProvenance.Provider
					entry.Model = draft.Result.AIProvenance.Model
					entry.Comment = draft.Result.Draft.Rationale
					entry.PromptHash = draft.Result.AIProvenance.PromptHash
				}
				item.AIHistory = append(item.AIHistory, entry)
				break
			}
		}
	}
	currentRunByPlace := map[string]string{}
	for _, draft := range output.Drafts {
		for index := len(output.Runs) - 1; index >= 0; index-- {
			for _, result := range output.Runs[index].Results {
				if result.PlaceID == draft.PlaceID {
					currentRunByPlace[draft.PlaceID] = output.Runs[index].ID
					break
				}
			}
			if currentRunByPlace[draft.PlaceID] != "" {
				break
			}
		}
	}
	for placeID, item := range locations {
		for index := range item.AIHistory {
			if item.AIHistory[index].RunID == currentRunByPlace[placeID] {
				item.AIHistory[index].Current = true
			}
		}
	}
}

func aiStatus(draft curationpilot.PlaceDraft) string {
	if draft.Error != "" || draft.Result == nil {
		return "technical_failure"
	}
	switch draft.Result.Draft.Status {
	case "candidate_selected":
		return "candidate_proposed"
	case "needs_candidates", "ambiguous":
		return draft.Result.Draft.Status
	default:
		return "technical_failure"
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
