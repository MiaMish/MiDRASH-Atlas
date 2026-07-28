package curationpilot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"midrash-atlas/apps/api/internal/curation"
	"midrash-atlas/apps/api/internal/gazetteerpilot"
)

type Generator interface {
	Generate(context.Context, curation.DraftRequest) (curation.Result, error)
}

type Config struct {
	InputPath  string
	OutputPath string
	PlaceIDs   []string
	Provider   string
	Model      string
}

type Output struct {
	SchemaVersion        string       `json:"schema_version"`
	GeneratedAt          time.Time    `json:"generated_at"`
	Purpose              string       `json:"purpose"`
	SourceCandidatesPath string       `json:"source_candidates_path"`
	ReviewPolicy         string       `json:"review_policy"`
	RequestedProvider    string       `json:"requested_provider"`
	RequestedModel       string       `json:"requested_model"`
	Drafts               []PlaceDraft `json:"drafts"`
}

type PlaceDraft struct {
	PlaceID        string           `json:"place_id"`
	PlaceLabel     string           `json:"place_label"`
	Case           string           `json:"case"`
	CandidateCount int              `json:"candidate_count"`
	Result         *curation.Result `json:"result,omitempty"`
	Error          string           `json:"error,omitempty"`
}

func Run(ctx context.Context, generator Generator, cfg Config) (Output, error) {
	var input gazetteerpilot.Output
	if err := readJSON(cfg.InputPath, &input); err != nil {
		return Output{}, err
	}
	selected := make(map[string]bool, len(cfg.PlaceIDs))
	for _, id := range cfg.PlaceIDs {
		if id = strings.TrimSpace(id); id != "" {
			selected[id] = true
		}
	}
	output := Output{
		SchemaVersion:        "1.0.0",
		GeneratedAt:          time.Now().UTC(),
		Purpose:              "modern_geometry_curation_draft_pilot",
		SourceCandidatesPath: cfg.InputPath,
		ReviewPolicy:         "LLM results are review drafts only and never update accepted geometry.",
		RequestedProvider:    cfg.Provider,
		RequestedModel:       cfg.Model,
	}
	for _, place := range input.Places {
		if len(selected) > 0 && !selected[place.PlaceID] {
			continue
		}
		request := place.CurationRequest
		request.Provider = cfg.Provider
		request.Model = cfg.Model
		item := PlaceDraft{
			PlaceID:        place.PlaceID,
			PlaceLabel:     place.PlaceLabel,
			Case:           place.Case,
			CandidateCount: place.CandidateCount,
		}
		result, err := generator.Generate(ctx, request)
		if err != nil {
			item.Error = err.Error()
		} else {
			item.Result = &result
		}
		output.Drafts = append(output.Drafts, item)
		output.GeneratedAt = time.Now().UTC()
		if err := writeJSONAtomic(cfg.OutputPath, output); err != nil {
			return Output{}, err
		}
	}
	if len(output.Drafts) == 0 {
		return Output{}, fmt.Errorf("no pilot places matched the requested place IDs")
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

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
