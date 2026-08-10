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

func TestCanonicalAtlasEndpoint(t *testing.T) {
	preparedDir := t.TempDir()
	writeFixture(t, filepath.Join(preparedDir, "atlas-canonical.json"), `{"schema_version":"2.0.0-alpha.1","works":[{"id":"1.15:1:1.0"}]}`)
	store := openStore(t)
	handler := New(Config{PreparedDir: preparedDir}, store, fakeGenerator{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/atlas", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !bytes.Contains(res.Body.Bytes(), []byte(`"id":"1.15:1:1.0"`)) {
		t.Fatalf("canonical pilot unavailable: %d %s", res.Code, res.Body.String())
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
			AIStatus            string `json:"ai_status"`
			AIHistory           []struct {
				Current bool `json:"current"`
			} `json:"ai_history"`
		} `json:"locations"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Locations) != 2 || got.Locations[0].PlaceID != "place_spain" ||
		!got.Locations[0].HasValidCoordinates || got.Locations[0].CoordinateStatus != "unreviewed_candidate" ||
		got.Locations[0].AIStatus != "candidate_proposed" || len(got.Locations[0].AIHistory) != 1 ||
		!got.Locations[0].AIHistory[0].Current {
		t.Fatalf("pilot geometry was not exposed correctly: %+v", got.Locations)
	}
	if got.Locations[1].HasValidCoordinates || got.Locations[1].CoordinateStatus != "no_geometry" ||
		got.Locations[1].AIStatus != "not_attempted" {
		t.Fatalf("missing geometry status was not preserved: %+v", got.Locations[1])
	}
}

func TestLocationOverviewMarksStaleAIRecommendationAsCheckedButUnavailable(t *testing.T) {
	root := t.TempDir()
	exportDir := filepath.Join(root, "generated")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(exportDir, "places.json"), `{"places":[{
	  "id":"place_bukhoro","preferred_label":"Bukhoro (Uzbekistan)",
	  "aliases":["Bukhoro (Uzbekistan)"],"geometry_variants":[],"review_status":"needs_geometry_review"
	}]}`)
	pilotPath := filepath.Join(root, "pilot.json")
	writeFixture(t, pilotPath, `{"places":[{
	  "place_id":"place_bukhoro","place_label":"Bukhoro (Uzbekistan)","candidate_count":1,
	  "curation_request":{"place_id":"place_bukhoro","place_label":"Bukhoro (Uzbekistan)",
	    "gazetteer_candidates":[{
	      "id":"gaz_current","source":"nominatim_openstreetmap","display_name":"Bukhara District",
	      "geometry":{"type":"Point","coordinates":[64.4,39.8]}
	    }]}
	}]}`)
	draftsPath := filepath.Join(root, "drafts.json")
	writeFixture(t, draftsPath, `{
	  "generated_at":"2026-07-28T12:01:00Z",
	  "requested_provider":"ollama","requested_model":"qwen3.5:35b",
	  "drafts":[{
	    "place_id":"place_bukhoro","place_label":"Bukhoro (Uzbekistan)","candidate_count":2,
	    "result":{
	      "draft":{"status":"candidate_selected","place_id":"place_bukhoro",
	        "place_label":"Bukhoro (Uzbekistan)","recommended_candidate_id":"gaz_from_previous_run",
	        "recommended_geometry_type":"Polygon","spatial_precision":"locality","confidence":"high",
	        "rationale":"The city candidate matched.","ambiguities":[],"required_review_checks":[],
	        "suggested_aliases":["Bukhara"]},
	      "ai_provenance":{"provider":"ollama","model":"qwen3.5:35b",
	        "generated_at":"2026-07-28T12:01:00Z","purpose":"modern_geometry_curation_draft",
	        "prompt_hash":"hash"},"prompt":"prompt"
	    }
	  }]
	}`)
	handler := New(Config{
		ExportDir: exportDir, GazetteerPilotPath: pilotPath, CurationDraftsPath: draftsPath,
	}, openStore(t), fakeGenerator{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/location-overview", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status %d: %s", res.Code, res.Body.String())
	}
	var got struct {
		Locations []struct {
			HasValidCoordinates bool   `json:"has_valid_coordinates"`
			AIStatus            string `json:"ai_status"`
		} `json:"locations"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Locations) != 1 || got.Locations[0].HasValidCoordinates ||
		got.Locations[0].AIStatus != "candidate_unavailable" {
		t.Fatalf("stale AI candidate status is wrong: %+v", got.Locations)
	}
}

func TestAtlasViewJoinsAndFiltersMappedManuscriptEvents(t *testing.T) {
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
	writeFixture(t, filepath.Join(exportDir, "records.json"), `{"records":[
	  {
	    "id":"record_one","physical_id":"record_one","kind":"manuscript","titles":["Test manuscript"],
	    "hibur_links":[{"hibur_id":"hibur_one","raw_label":"Test work","match_status":"exact_alias"}],
	    "digitized":true,"geo_assertion_ids":["geo_one"],"temporal_assertion_ids":["time_one"]
	  },
	  {
	    "id":"record_undated","physical_id":"record_undated","kind":"manuscript","titles":null,
	    "parent_ids":["parent_one"],
	    "parent_records":[{
	      "id":"parent_one","kind":"manuscript","titles":["Parent manuscript"],
	      "primary_titles":["Parent manuscript"]
	    }],
	    "hibur_links":null,"digitized":false,"geo_assertion_ids":["geo_undated"],
	    "temporal_assertion_ids":null
	  },
	  {
	    "id":"record_unmapped","physical_id":"record_unmapped","kind":"manuscript",
	    "titles":["Unmapped manuscript"],"hibur_links":null,"digitized":false,
	    "geo_assertion_ids":["geo_unmapped"],"temporal_assertion_ids":null
	  }
	]}`)
	writeFixture(t, filepath.Join(exportDir, "geo_assertions.json"), `{"assertions":[
	  {
	    "id":"geo_one","source_type_id":"nli_751_writing_place",
	    "scope":{"target_record_id":"record_one","source_record_id":"record_one","physical_id":"record_one","origin":"direct"},
	    "place_id":"place_spain","place_raw":"Spain","role":"place of writing","confidence":"high",
	    "evidence":{"raw":"Spain","catalog":"NLI","extraction":"structured_field","review_status":"unreviewed"}
	  },
	  {
	    "id":"geo_undated","source_type_id":"nli_751_writing_place",
	    "scope":{"target_record_id":"record_undated","source_record_id":"parent_one","physical_id":"parent_one","origin":"parent_fallback"},
	    "place_id":"place_spain","place_raw":"Spain","role":"place of writing","confidence":"high",
	    "evidence":{"raw":"Spain","catalog":"NLI","extraction":"structured_field","review_status":"unreviewed"}
	  },
	  {
	    "id":"geo_unmapped","source_type_id":"nli_751_writing_place",
	    "scope":{"target_record_id":"record_unmapped","source_record_id":"record_unmapped","physical_id":"record_unmapped","origin":"direct"},
	    "place_id":"place_unknown","place_raw":"Unknown","role":"place of writing","confidence":"high",
	    "evidence":{"raw":"Unknown","catalog":"NLI","extraction":"structured_field","review_status":"unreviewed"}
	  }
	]}`)
	writeFixture(t, filepath.Join(exportDir, "temporal_assertions.json"), `{"assertions":[{
	  "id":"time_one","source_type_id":"nli_260_date",
	  "scope":{"target_record_id":"record_one","source_record_id":"record_one","physical_id":"record_one","origin":"direct"},
	  "value":{"kind":"year","year":1460,"approximate":true,"uncertain":false},
	  "confidence":"medium",
	  "evidence":{"raw":"1460 circa","catalog":"NLI","extraction":"deterministic_parser","review_status":"unreviewed"}
	}]}`)
	writeFixture(t, filepath.Join(exportDir, "hiburim.json"), `{"hiburim":[{
	  "id":"hibur_one","label":"Test work","english":"Test Work","aliases":["Test work"]
	}]}`)
	writeFixture(t, filepath.Join(exportDir, "source_types.json"), `{"source_types":[{
	  "id":"nli_751_writing_place","dimension":"geo","label":"NLI: place of writing",
	  "short_description":"Explicit writing place.","default_enabled":true
	}]}`)
	pilotPath := filepath.Join(root, "pilot.json")
	writeFixture(t, pilotPath, `{
	  "schema_version":"1.0.0","places":[{
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
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/atlas-view?geo-source=nli_751_writing_place&start-year=1450&end-year=1470&include-undated=false", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status %d: %s", res.Code, res.Body.String())
	}
	var got struct {
		Summary struct {
			MappedAssertions int `json:"mapped_assertions"`
			MappedPlaces     int `json:"mapped_places"`
			MatchedRecords   int `json:"matched_records"`
		} `json:"summary"`
		Features []struct {
			Properties struct {
				PlaceID string `json:"place_id"`
				Events  []struct {
					Hiburim []struct {
						ID string `json:"id"`
					} `json:"hiburim"`
					Temporal []struct {
						Effective struct {
							StartYear int `json:"start_year"`
							EndYear   int `json:"end_year"`
						} `json:"effective_interval"`
					} `json:"temporal_assertions"`
				} `json:"events"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Summary.MappedAssertions != 1 || got.Summary.MappedPlaces != 1 ||
		got.Summary.MatchedRecords != 1 || len(got.Features) != 1 ||
		got.Features[0].Properties.PlaceID != "place_spain" ||
		got.Features[0].Properties.Events[0].Hiburim[0].ID != "hibur_one" ||
		got.Features[0].Properties.Events[0].Temporal[0].Effective.StartYear != 1450 ||
		got.Features[0].Properties.Events[0].Temporal[0].Effective.EndYear != 1470 {
		t.Fatalf("atlas join/filter result is wrong: %+v", got)
	}

	req = httptest.NewRequest(http.MethodGet,
		"/api/v1/atlas-view?geo-source=nli_751_writing_place&include-undated=true", nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status %d: %s", res.Code, res.Body.String())
	}
	if !bytes.Contains(res.Body.Bytes(), []byte(`"id":"record_undated"`)) ||
		!bytes.Contains(res.Body.Bytes(), []byte(`"source_record":{"id":"parent_one"`)) ||
		!bytes.Contains(res.Body.Bytes(), []byte(`"hiburim":[]`)) ||
		!bytes.Contains(res.Body.Bytes(), []byte(`"temporal_assertions":[]`)) ||
		!bytes.Contains(res.Body.Bytes(), []byte(`"unmapped_groups":[{"place_id":"place_unknown"`)) ||
		!bytes.Contains(res.Body.Bytes(), []byte(`"id":"record_unmapped"`)) {
		t.Fatalf("undated event collections must be JSON arrays: %s", res.Body.String())
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
