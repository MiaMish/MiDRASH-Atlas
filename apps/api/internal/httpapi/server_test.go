package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"midrash-atlas/apps/api/internal/curation"
	"midrash-atlas/apps/api/internal/locationstore"
	"midrash-atlas/packages/provenance"
)

type fakeGenerator struct{}

func (fakeGenerator) Generate(_ context.Context, request curation.DraftRequest) (curation.Result, error) {
	return curation.Result{
		Draft: curation.Draft{
			Status: "needs_candidates", PlaceID: request.PlaceID, PlaceLabel: request.PlaceLabel,
			Confidence: "low", Rationale: "No supplied candidate.",
		},
		AIProvenance: provenance.AIRun{
			Provider: "fake", Model: "fake", GeneratedAt: time.Now().UTC(),
			Purpose: "modern_geometry_curation_draft", PromptHash: "hash",
		},
		Prompt: "prompt",
	}, nil
}

func TestTemporalViewAppliesCircaPolicy(t *testing.T) {
	exportDir := t.TempDir()
	fixture := `{"schema_version":"1.0.0","assertions":[{
	  "id":"time_1","source_type_id":"nli_260_date",
	  "scope":{"target_record_id":"ms_1","source_record_id":"ms_1","physical_id":"ms_1","origin":"direct"},
	  "value":{"kind":"year","year":1460,"approximate":true,"uncertain":false},
	  "confidence":"medium",
	  "evidence":{"raw":"1460 circa","catalog":"test","extraction":"test","review_status":"unreviewed"}
	}]}`
	if err := os.WriteFile(filepath.Join(exportDir, "temporal_assertions.json"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	store := openStore(t)
	handler := New(Config{ExportDir: exportDir}, store, fakeGenerator{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/temporal-view?circa-years=10", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status %d: %s", res.Code, res.Body.String())
	}
	var got struct {
		Assertions []struct {
			Value struct {
				Year int `json:"year"`
			} `json:"value"`
			Effective struct {
				StartYear int `json:"start_year"`
				EndYear   int `json:"end_year"`
			} `json:"effective_interval"`
		} `json:"assertions"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Assertions[0].Value.Year != 1460 || got.Assertions[0].Effective.StartYear != 1450 || got.Assertions[0].Effective.EndYear != 1470 {
		t.Fatalf("stored and derived dates are wrong: %+v", got)
	}
}

func TestLocationEndpointCreatesAuditedRevisions(t *testing.T) {
	store := openStore(t)
	handler := New(Config{ExportDir: t.TempDir()}, store, fakeGenerator{})
	body := []byte(`{
	  "geometry_variant_id":"modern_spain",
	  "label":"Modern Spain",
	  "geometry":{"type":"Point","coordinates":[-3.7,40.4]},
	  "spatial_precision":"modern_country",
	  "interpretation_source":"modern_gazetteer_geometry",
	  "confidence":"medium",
	  "review_status":"draft",
	  "human_action":"changed",
	  "change_reason":"Moved in location editor"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/locations/place_spain/geometries", bytes.NewReader(body))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", res.Code, res.Body.String())
	}
	var created locationstore.GeometryRevision
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Source != "ui" || created.HumanAction != "changed" || created.CreatedAt.IsZero() || created.Revision != 1 {
		t.Fatalf("missing audit metadata: %+v", created)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/locations/place_spain/geometries?history=true", nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte(`"change_reason":"Moved in location editor"`)) {
		t.Fatalf("history unavailable: %d %s", res.Code, res.Body.String())
	}
}

func TestGeometryDraftEndpoint(t *testing.T) {
	store := openStore(t)
	handler := New(Config{ExportDir: t.TempDir()}, store, fakeGenerator{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/curation/geometry-draft",
		bytes.NewBufferString(`{"place_id":"place_spain","place_label":"Spain"}`))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte(`"status":"needs_candidates"`)) {
		t.Fatalf("unexpected draft response: %d %s", res.Code, res.Body.String())
	}
}

func TestLocationOverviewIncludesUnreviewedPilotGeometry(t *testing.T) {
	root := t.TempDir()
	exportDir := filepath.Join(root, "generated")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(exportDir, "places.json"), `{
	  "places":[
	    {"id":"place_spain","preferred_label":"Spain","aliases":["Spain"],"geometry_variants":[],"review_status":"needs_geometry_review"},
	    {"id":"place_unknown","preferred_label":"Unknown","aliases":[],"geometry_variants":[],"review_status":"needs_geometry_review"}
	  ]
	}`)
	pilotPath := filepath.Join(root, "pilot.json")
	writeFixture(t, pilotPath, `{
	  "schema_version":"1.0.0",
	  "generated_at":"2026-07-28T12:00:00Z",
	  "purpose":"modern_geometry_gazetteer_pilot",
	  "gazetteer":{"source":"nominatim_openstreetmap","runtime_use":false,"cache_policy":"cached","review_policy":"human"},
	  "places":[{
	    "place_id":"place_spain","place_label":"Spain","case":"country","candidate_count":1,
	    "curation_request":{"place_id":"place_spain","place_label":"Spain","gazetteer_candidates":[{
	      "id":"gaz_spain","source":"nominatim_openstreetmap","display_name":"Spain",
	      "geometry":{"type":"Point","coordinates":[-3.7,40.4]}
	    }]}
	  }]
	}`)
	draftsPath := filepath.Join(root, "drafts.json")
	writeFixture(t, draftsPath, `{
	  "schema_version":"1.0.0","generated_at":"2026-07-28T12:01:00Z",
	  "purpose":"modern_geometry_curation_draft_pilot","source_candidates_path":"pilot.json",
	  "review_policy":"draft","requested_provider":"ollama","requested_model":"qwen3.5:35b",
	  "drafts":[{
	    "place_id":"place_spain","place_label":"Spain","case":"country","candidate_count":1,
	    "result":{
	      "draft":{"status":"candidate_selected","place_id":"place_spain","place_label":"Spain",
	        "recommended_candidate_id":"gaz_spain","recommended_geometry_type":"Point",
	        "spatial_precision":"modern_country","confidence":"high","rationale":"Matching country.",
	        "ambiguities":[],"required_review_checks":[],"suggested_aliases":[]},
	      "ai_provenance":{"provider":"ollama","model":"qwen3.5:35b",
	        "generated_at":"2026-07-28T12:01:00Z","purpose":"modern_geometry_curation_draft","prompt_hash":"hash"},
	      "prompt":"prompt"
	    }
	  }]
	}`)
	store := openStore(t)
	handler := New(Config{
		ExportDir: exportDir, GazetteerPilotPath: pilotPath, CurationDraftsPath: draftsPath,
	}, store, fakeGenerator{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/location-overview", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status %d: %s", res.Code, res.Body.String())
	}
	var got struct {
		Locations []struct {
			PlaceID             string `json:"place_id"`
			HasValidCoordinates bool   `json:"has_valid_coordinates"`
			CoordinateStatus    string `json:"coordinate_status"`
		} `json:"locations"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Locations) != 2 || got.Locations[0].PlaceID != "place_spain" ||
		!got.Locations[0].HasValidCoordinates || got.Locations[0].CoordinateStatus != "unreviewed_candidate" {
		t.Fatalf("pilot geometry was not exposed correctly: %+v", got.Locations)
	}
	if got.Locations[1].HasValidCoordinates || got.Locations[1].CoordinateStatus != "no_geometry" {
		t.Fatalf("missing geometry status was not preserved: %+v", got.Locations[1])
	}
}

func writeFixture(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func openStore(t *testing.T) *locationstore.SQLite {
	t.Helper()
	store, err := locationstore.OpenSQLite(filepath.Join(t.TempDir(), "atlas.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
