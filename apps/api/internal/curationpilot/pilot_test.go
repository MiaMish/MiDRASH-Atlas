package curationpilot

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"midrash-atlas/apps/api/internal/curation"
	"midrash-atlas/packages/provenance"
)

type fakeGenerator struct {
	model string
}

func (generator fakeGenerator) Generate(_ context.Context, request curation.DraftRequest) (curation.Result, error) {
	return curation.Result{
		Draft: curation.Draft{
			Status: "needs_candidates", PlaceID: request.PlaceID, PlaceLabel: request.PlaceLabel,
			Confidence: "low", Rationale: "No safe candidate.",
		},
		AIProvenance: provenance.AIRun{
			Provider: "ollama", Model: generator.model, GeneratedAt: time.Now().UTC(),
			Purpose: "modern_geometry_curation_draft", PromptHash: "hash",
		},
		Prompt: "prompt",
	}, nil
}

func TestRunReplacesCurrentDraftAndAppendsAuditHistory(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "candidates.json")
	outputPath := filepath.Join(root, "drafts.json")
	writeTestJSON(t, inputPath, map[string]any{
		"schema_version": "1.0.0",
		"places": []any{
			map[string]any{
				"place_id": "place_one", "place_label": "One", "case": "first",
				"curation_request": map[string]any{"place_id": "place_one", "place_label": "One"},
			},
			map[string]any{
				"place_id": "place_two", "place_label": "Two", "case": "second",
				"curation_request": map[string]any{"place_id": "place_two", "place_label": "Two"},
			},
		},
	})
	legacyTime := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	writeTestJSON(t, outputPath, Output{
		SchemaVersion: "1.0.0", GeneratedAt: legacyTime,
		Purpose:              "modern_geometry_curation_draft_pilot",
		SourceCandidatesPath: inputPath, ReviewPolicy: "draft",
		RequestedProvider: "ollama", RequestedModel: "old-model",
		Drafts: []PlaceDraft{{
			PlaceID: "place_one", PlaceLabel: "One", Result: &curation.Result{
				Draft: curation.Draft{
					Status: "ambiguous", PlaceID: "place_one", PlaceLabel: "One",
					Confidence: "low", Rationale: "Old result.",
				},
				AIProvenance: provenance.AIRun{
					Provider: "ollama", Model: "old-model", GeneratedAt: legacyTime,
					Purpose: "modern_geometry_curation_draft", PromptHash: "old-hash",
				},
				Prompt: "old prompt",
			},
		}},
	})

	output, err := Run(context.Background(), fakeGenerator{model: "new-model"}, Config{
		InputPath: inputPath, OutputPath: outputPath, Provider: "ollama", Model: "new-model",
		SkipHumanReviewed: true, SkipPlaceIDs: []string{"place_two"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if output.SchemaVersion != "1.1.0" || len(output.Drafts) != 1 ||
		output.Drafts[0].Result.AIProvenance.Model != "new-model" {
		t.Fatalf("latest draft was not replaced: %+v", output.Drafts)
	}
	if len(output.Runs) != 2 || !output.Runs[0].Imported ||
		output.Runs[0].Results[0].Result.AIProvenance.Model != "old-model" {
		t.Fatalf("legacy result was not retained in audit history: %+v", output.Runs)
	}
	current := output.Runs[1]
	if current.CompletedAt == nil || len(current.Results) != 1 || len(current.Skipped) != 1 ||
		current.Skipped[0].PlaceID != "place_two" ||
		current.Skipped[0].Code != "skipped_human_reviewed" {
		t.Fatalf("current run audit is incomplete: %+v", current)
	}
	var persisted Output
	if err := readJSON(outputPath, &persisted); err != nil {
		t.Fatal(err)
	}
	if len(persisted.Runs) != 2 || persisted.Runs[1].CompletedAt == nil {
		t.Fatalf("audit history was not checkpointed: %+v", persisted.Runs)
	}

	var progressEvents []Progress
	second, err := Run(context.Background(), fakeGenerator{model: "third-model"}, Config{
		InputPath: inputPath, OutputPath: outputPath, Provider: "ollama", Model: "third-model",
		OnlyAINotChecked: true,
		OnProgress: func(progress Progress) {
			progressEvents = append(progressEvents, progress)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Drafts) != 2 || len(second.Runs) != 3 {
		t.Fatalf("only-not-checked run did not preserve and extend current drafts: %+v", second)
	}
	onlyNew := second.Runs[2]
	if !onlyNew.OnlyAINotChecked || len(onlyNew.Results) != 1 ||
		onlyNew.Results[0].PlaceID != "place_two" || len(onlyNew.Skipped) != 1 ||
		onlyNew.Skipped[0].PlaceID != "place_one" ||
		onlyNew.Skipped[0].Code != "skipped_ai_already_checked" {
		t.Fatalf("only-not-checked filtering is wrong: %+v", onlyNew)
	}
	if len(progressEvents) != 3 ||
		progressEvents[0].Phase != "skipped" ||
		progressEvents[1].Phase != "started" ||
		progressEvents[2].Phase != "completed" {
		t.Fatalf("progress events are incomplete: %+v", progressEvents)
	}
}

func writeTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
