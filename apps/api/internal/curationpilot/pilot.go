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
	InputPath         string
	OutputPath        string
	PlaceIDs          []string
	Provider          string
	Model             string
	SkipPlaceIDs      []string
	SkipHumanReviewed bool
	OnlyAINotChecked  bool
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
	Runs                 []RunAudit   `json:"runs"`
}

type PlaceDraft struct {
	PlaceID        string           `json:"place_id"`
	PlaceLabel     string           `json:"place_label"`
	Case           string           `json:"case"`
	CandidateCount int              `json:"candidate_count"`
	Result         *curation.Result `json:"result,omitempty"`
	Error          string           `json:"error,omitempty"`
}

type RunAudit struct {
	ID                string         `json:"id"`
	StartedAt         time.Time      `json:"started_at"`
	CompletedAt       *time.Time     `json:"completed_at,omitempty"`
	Provider          string         `json:"provider"`
	Model             string         `json:"model"`
	RequestedPlaceIDs []string       `json:"requested_place_ids,omitempty"`
	SkipHumanReviewed bool           `json:"skip_human_reviewed"`
	OnlyAINotChecked  bool           `json:"only_ai_not_checked"`
	Imported          bool           `json:"imported,omitempty"`
	Skipped           []SkippedPlace `json:"skipped,omitempty"`
	Results           []PlaceDraft   `json:"results"`
}

type SkippedPlace struct {
	PlaceID    string `json:"place_id"`
	PlaceLabel string `json:"place_label"`
	Code       string `json:"code"`
	Reason     string `json:"reason"`
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
	skipped := make(map[string]bool, len(cfg.SkipPlaceIDs))
	for _, id := range cfg.SkipPlaceIDs {
		if id = strings.TrimSpace(id); id != "" {
			skipped[id] = true
		}
	}
	output, err := loadOrInitializeOutput(cfg)
	if err != nil {
		return Output{}, err
	}
	now := time.Now().UTC()
	run := RunAudit{
		ID:        "curation_run_" + now.Format("20060102T150405.000000000Z"),
		StartedAt: now, Provider: cfg.Provider, Model: cfg.Model,
		RequestedPlaceIDs: append([]string(nil), cfg.PlaceIDs...),
		SkipHumanReviewed: cfg.SkipHumanReviewed,
		OnlyAINotChecked:  cfg.OnlyAINotChecked,
	}
	output.Runs = append(output.Runs, run)
	runIndex := len(output.Runs) - 1
	alreadyChecked := make(map[string]bool, len(output.Drafts))
	for _, draft := range output.Drafts {
		alreadyChecked[draft.PlaceID] = true
	}
	targetCount := 0
	for _, place := range input.Places {
		if len(selected) > 0 && !selected[place.PlaceID] {
			continue
		}
		targetCount++
		if skipped[place.PlaceID] {
			output.Runs[runIndex].Skipped = append(output.Runs[runIndex].Skipped, SkippedPlace{
				PlaceID: place.PlaceID, PlaceLabel: place.PlaceLabel,
				Code:   "skipped_human_reviewed",
				Reason: "current modern_place geometry is reviewed by a human",
			})
			continue
		}
		if cfg.OnlyAINotChecked && alreadyChecked[place.PlaceID] {
			output.Runs[runIndex].Skipped = append(output.Runs[runIndex].Skipped, SkippedPlace{
				PlaceID: place.PlaceID, PlaceLabel: place.PlaceLabel,
				Code:   "skipped_ai_already_checked",
				Reason: "a current AI draft already exists",
			})
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
		output.Drafts = upsertDraft(output.Drafts, item)
		output.Runs[runIndex].Results = append(output.Runs[runIndex].Results, item)
		output.GeneratedAt = time.Now().UTC()
		if err := writeJSONAtomic(cfg.OutputPath, output); err != nil {
			return Output{}, err
		}
	}
	if targetCount == 0 {
		return Output{}, fmt.Errorf("no pilot places matched the requested place IDs")
	}
	completedAt := time.Now().UTC()
	output.GeneratedAt = completedAt
	output.Runs[runIndex].CompletedAt = &completedAt
	if err := writeJSONAtomic(cfg.OutputPath, output); err != nil {
		return Output{}, err
	}
	return output, nil
}

func loadOrInitializeOutput(cfg Config) (Output, error) {
	var output Output
	if err := readJSON(cfg.OutputPath, &output); err == nil {
		if len(output.Runs) == 0 && len(output.Drafts) > 0 {
			completedAt := output.GeneratedAt
			output.Runs = append(output.Runs, RunAudit{
				ID:        "curation_run_imported_" + completedAt.Format("20060102T150405Z"),
				StartedAt: completedAt, CompletedAt: &completedAt,
				Provider: output.RequestedProvider, Model: output.RequestedModel,
				Imported: true, Results: append([]PlaceDraft(nil), output.Drafts...),
			})
		}
	} else if !os.IsNotExist(err) {
		return Output{}, err
	} else {
		output = Output{
			Purpose:              "modern_geometry_curation_draft_pilot",
			SourceCandidatesPath: cfg.InputPath,
			ReviewPolicy:         "LLM results are review drafts only and never update accepted geometry.",
		}
	}
	output.SchemaVersion = "1.1.0"
	output.SourceCandidatesPath = cfg.InputPath
	output.RequestedProvider = cfg.Provider
	output.RequestedModel = cfg.Model
	return output, nil
}

func upsertDraft(drafts []PlaceDraft, item PlaceDraft) []PlaceDraft {
	for index := range drafts {
		if drafts[index].PlaceID == item.PlaceID {
			drafts[index] = item
			return drafts
		}
	}
	return append(drafts, item)
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
