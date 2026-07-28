package curation

import (
	"context"
	"strings"
	"testing"
)

type fakeExecutor struct {
	response string
	prompt   string
}

func (f *fakeExecutor) Exec(_ context.Context, _, _, prompt string) (string, error) {
	f.prompt = prompt
	return f.response, nil
}

func TestGeneratorRejectsInventedCandidate(t *testing.T) {
	client := &fakeExecutor{response: `{
	  "status":"candidate_selected",
	  "place_id":"place_1",
	  "place_label":"Spain",
	  "recommended_candidate_id":"invented",
	  "recommended_geometry_type":"Polygon",
	  "spatial_precision":"modern_country",
	  "confidence":"high",
	  "rationale":"test",
	  "ambiguities":[],
	  "required_review_checks":[],
	  "suggested_aliases":[]
	}`}
	g := NewGenerator(client, Config{DefaultProvider: "openai", DefaultModel: "test"})
	_, err := g.Generate(context.Background(), DraftRequest{
		PlaceID: "place_1", PlaceLabel: "Spain",
		GazetteerCandidates: []GazetteerCandidate{{ID: "candidate_1"}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown gazetteer candidate") {
		t.Fatalf("expected candidate validation error, got %v", err)
	}
	if !strings.Contains(client.prompt, "Never invent coordinates") {
		t.Fatalf("prompt is missing anti-fabrication constraint")
	}
}

func TestGeneratorAcceptsSuppliedCandidate(t *testing.T) {
	client := &fakeExecutor{response: `Some wrapper text
	{"status":"candidate_selected","place_id":"place_1","place_label":"Spain",
	"recommended_candidate_id":"candidate_1","recommended_geometry_type":"MultiPolygon",
	"spatial_precision":"modern_country","confidence":"medium","rationale":"Supplied match.",
	"ambiguities":["Historical scope differs."],"required_review_checks":["Confirm scope."],"suggested_aliases":[]}`}
	g := NewGenerator(client, Config{DefaultProvider: "openai", DefaultModel: "test"})
	got, err := g.Generate(context.Background(), DraftRequest{
		PlaceID: "place_1", PlaceLabel: "Spain",
		GazetteerCandidates: []GazetteerCandidate{{ID: "candidate_1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Draft.RecommendedCandidateID == nil || *got.Draft.RecommendedCandidateID != "candidate_1" {
		t.Fatalf("unexpected draft: %+v", got.Draft)
	}
	if got.AIProvenance.PromptHash == "" || got.AIProvenance.Provider != "openai" ||
		got.AIProvenance.Model != "test" || got.AIProvenance.GeneratedAt.IsZero() {
		t.Fatalf("missing AI provenance: %+v", got.AIProvenance)
	}
}
