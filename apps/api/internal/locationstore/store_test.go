package locationstore

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"midrash-atlas/packages/provenance"
)

func TestSQLiteKeepsAppendOnlyAuditHistory(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "atlas.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	input := SaveRevision{
		PlaceID: "place_spain", GeometryVariantID: "modern_spain",
		Label: "Modern Spain", Geometry: json.RawMessage(`{"type":"Point","coordinates":[-3.7,40.4]}`),
		SpatialPrecision: "modern_country", InterpretationSource: "modern_gazetteer_geometry",
		Confidence: "medium", ReviewStatus: "draft", HumanAction: "changed", ChangeReason: "Initial UI placement",
		AIProvenance: &provenance.AIRun{
			Provider: "ollama", Model: "qwen3.5:35b",
			GeneratedAt: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
			Purpose:     "modern_geometry_curation_draft", PromptHash: "abc123",
		},
	}
	first, err := store.SaveRevision(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	input.Geometry = json.RawMessage(`{"type":"Point","coordinates":[-3.6,40.3]}`)
	input.ChangeReason = "Adjusted through location editor"
	second, err := store.SaveRevision(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if second.Revision != 2 || second.SupersedesRevisionID == nil || *second.SupersedesRevisionID != first.ID {
		t.Fatalf("revision chain not preserved: first=%+v second=%+v", first, second)
	}
	history, err := store.ListRevisions(context.Background(), input.PlaceID, input.GeometryVariantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].Revision != 2 || history[1].Revision != 1 {
		t.Fatalf("unexpected history: %+v", history)
	}
	if history[0].AIProvenance == nil || history[0].AIProvenance.Provider != "ollama" ||
		history[0].AIProvenance.Model != "qwen3.5:35b" {
		t.Fatalf("AI provenance was not preserved: %+v", history[0].AIProvenance)
	}
	current, err := store.ListCurrent(context.Background(), input.PlaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(current) != 1 || current[0].Revision != 2 {
		t.Fatalf("unexpected current values: %+v", current)
	}
	if current[0].HumanAction != "changed" {
		t.Fatalf("human action was not preserved: %+v", current[0])
	}
}

func TestSQLiteRejectsCoordinatesOutsideWorldBounds(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "atlas.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.SaveRevision(context.Background(), SaveRevision{
		PlaceID: "place_bad", GeometryVariantID: "modern_bad", Label: "Bad point",
		Geometry:         json.RawMessage(`{"type":"Point","coordinates":[999,95]}`),
		SpatialPrecision: "locality", InterpretationSource: "ui",
		Confidence: "low", ReviewStatus: "draft", HumanAction: "changed",
		ChangeReason: "test invalid coordinates",
	})
	if err == nil {
		t.Fatal("expected invalid coordinates to be rejected")
	}
}
