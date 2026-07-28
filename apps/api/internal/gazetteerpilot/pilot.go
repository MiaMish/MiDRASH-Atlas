package gazetteerpilot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"midrash-atlas/apps/api/internal/curation"
	"midrash-atlas/packages/atlas"
	"midrash-atlas/packages/gazetteer"
)

type Config struct {
	PilotConfigPath   string
	PlacesPath        string
	GeoAssertionsPath string
	TemporalPath      string
	OutputPath        string
	CacheDir          string
}

type pilotConfig struct {
	SchemaVersion string       `json:"schema_version"`
	Description   string       `json:"description"`
	Places        []PilotPlace `json:"places"`
}

type PilotPlace struct {
	PlaceID    string `json:"place_id"`
	PlaceLabel string `json:"place_label"`
	Query      string `json:"query"`
	Case       string `json:"case"`
}

type Output struct {
	SchemaVersion string        `json:"schema_version"`
	GeneratedAt   time.Time     `json:"generated_at"`
	Purpose       string        `json:"purpose"`
	Gazetteer     GazetteerInfo `json:"gazetteer"`
	Places        []OutputPlace `json:"places"`
}

type GazetteerInfo struct {
	Source       string `json:"source"`
	RuntimeUse   bool   `json:"runtime_use"`
	CachePolicy  string `json:"cache_policy"`
	ReviewPolicy string `json:"review_policy"`
}

type OutputPlace struct {
	PlaceID               string                          `json:"place_id"`
	PlaceLabel            string                          `json:"place_label"`
	Case                  string                          `json:"case"`
	AcquisitionProvenance gazetteer.AcquisitionProvenance `json:"acquisition_provenance"`
	FromCache             bool                            `json:"from_cache"`
	CandidateCount        int                             `json:"candidate_count"`
	CurationRequest       curation.DraftRequest           `json:"curation_request"`
}

func Run(ctx context.Context, client *gazetteer.Nominatim, cfg Config) (Output, error) {
	var pilot pilotConfig
	if err := readJSON(cfg.PilotConfigPath, &pilot); err != nil {
		return Output{}, err
	}
	var placeDocument struct {
		Places []atlas.PlaceConcept `json:"places"`
	}
	if err := readJSON(cfg.PlacesPath, &placeDocument); err != nil {
		return Output{}, err
	}
	var geoDocument struct {
		Assertions []atlas.GeoAssertion `json:"assertions"`
	}
	if err := readJSON(cfg.GeoAssertionsPath, &geoDocument); err != nil {
		return Output{}, err
	}
	var temporalDocument struct {
		Assertions []atlas.TemporalAssertion `json:"assertions"`
	}
	if err := readJSON(cfg.TemporalPath, &temporalDocument); err != nil {
		return Output{}, err
	}
	placeByID := map[string]atlas.PlaceConcept{}
	for _, place := range placeDocument.Places {
		placeByID[place.ID] = place
	}
	geoByPlace := map[string][]atlas.GeoAssertion{}
	targetsByPlace := map[string]map[string]bool{}
	for _, assertion := range geoDocument.Assertions {
		geoByPlace[assertion.PlaceID] = append(geoByPlace[assertion.PlaceID], assertion)
		if targetsByPlace[assertion.PlaceID] == nil {
			targetsByPlace[assertion.PlaceID] = map[string]bool{}
		}
		targetsByPlace[assertion.PlaceID][assertion.Scope.TargetRecordID] = true
	}
	temporalByTarget := map[string][]atlas.TemporalAssertion{}
	for _, assertion := range temporalDocument.Assertions {
		temporalByTarget[assertion.Scope.TargetRecordID] = append(temporalByTarget[assertion.Scope.TargetRecordID], assertion)
	}

	output := Output{
		SchemaVersion: "1.0.0", GeneratedAt: time.Now().UTC(),
		Purpose: "modern_geometry_gazetteer_pilot",
		Gazetteer: GazetteerInfo{
			Source: gazetteer.NominatimSourceID, RuntimeUse: false,
			CachePolicy:  "Raw responses are cached durably and identical queries are not repeated.",
			ReviewPolicy: "Candidates are drafts; no geometry is accepted without human review.",
		},
	}
	for _, selected := range pilot.Places {
		place, ok := placeByID[selected.PlaceID]
		if !ok {
			return Output{}, fmt.Errorf("pilot place %s is absent from places export", selected.PlaceID)
		}
		result, err := client.Search(ctx, selected.Query)
		if err != nil {
			return Output{}, fmt.Errorf("search %s (%s): %w", selected.PlaceLabel, selected.Query, err)
		}
		var candidates []curation.GazetteerCandidate
		for _, candidate := range result.Candidates {
			candidates = append(candidates, curation.GazetteerCandidate{
				ID: candidate.ID, Source: candidate.Source, DisplayName: candidate.DisplayName,
				Geometry: candidate.Geometry, BoundingBox: candidate.BoundingBox,
				AuthorityID: candidate.AuthorityID, License: result.Provenance.License,
			})
		}
		var geoContext []json.RawMessage
		for _, assertion := range geoByPlace[selected.PlaceID] {
			geoContext = append(geoContext, mustRaw(assertion))
		}
		var temporalContext []json.RawMessage
		for targetID := range targetsByPlace[selected.PlaceID] {
			for _, assertion := range temporalByTarget[targetID] {
				temporalContext = append(temporalContext, mustRaw(assertion))
			}
		}
		sort.Slice(temporalContext, func(i, j int) bool { return string(temporalContext[i]) < string(temporalContext[j]) })
		var variants []json.RawMessage
		for _, variant := range place.GeometryVariants {
			variants = append(variants, mustRaw(variant))
		}
		output.Places = append(output.Places, OutputPlace{
			PlaceID: selected.PlaceID, PlaceLabel: selected.PlaceLabel, Case: selected.Case,
			AcquisitionProvenance: result.Provenance, FromCache: result.FromCache,
			CandidateCount: len(candidates),
			CurationRequest: curation.DraftRequest{
				PlaceID: selected.PlaceID, PlaceLabel: selected.PlaceLabel, Aliases: place.Aliases,
				GeoAssertions: geoContext, TemporalContext: temporalContext,
				ExistingVariants: variants, GazetteerCandidates: candidates,
				AdditionalInstructions: "Pilot case: " + selected.Case,
			},
		})
	}
	if err := writeJSON(cfg.OutputPath, output); err != nil {
		return Output{}, err
	}
	return output, nil
}

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func mustRaw(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
