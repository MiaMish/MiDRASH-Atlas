package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"midrash-atlas/apps/api/internal/curation"
	"midrash-atlas/apps/api/internal/locationstore"
	"midrash-atlas/packages/atlas"
)

type Config struct {
	ExportDir          string
	PreparedDir        string
	GazetteerPilotPath string
	CurationDraftsPath string
}

type DraftGenerator interface {
	Generate(ctx context.Context, request curation.DraftRequest) (curation.Result, error)
}

type server struct {
	cfg       Config
	locations locationstore.Store
	curation  DraftGenerator
}

func New(cfg Config, locations locationstore.Store, generator DraftGenerator) http.Handler {
	s := &server{cfg: cfg, locations: locations, curation: generator}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/api/v1/curation/geometry-draft", s.geometryDraft)
	mux.HandleFunc("/api/v1/location-overview", s.locationOverview)
	mux.HandleFunc("/api/v1/atlas-view", s.atlasView)
	mux.HandleFunc("/api/v1/atlas", s.canonicalAtlas)
	mux.HandleFunc("/api/v1/locations/", s.locationsRoute)
	mux.HandleFunc("/api/v1/", s.atlasResources)
	return withCORS(mux)
}

func (s *server) canonicalAtlas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET required"))
		return
	}
	path := filepath.Join(s.cfg.PreparedDir, "atlas-canonical.json")
	if s.cfg.PreparedDir == "" {
		writeError(w, http.StatusNotFound, errors.New("canonical pilot is not configured"))
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, path)
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) atlasResources(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	if name == "temporal-view" {
		s.temporalView(w, r)
		return
	}
	allowed := map[string]string{
		"manifest": "manifest.json", "records": "records.json", "hiburim": "hiburim.json",
		"places": "places.json", "geo-assertions": "geo_assertions.json",
		"temporal-assertions": "temporal_assertions.json", "source-types": "source_types.json",
		"events": "events.geojson",
	}
	file, ok := allowed[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	http.ServeFile(w, r, filepath.Join(s.cfg.ExportDir, file))
}

func (s *server) temporalView(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(filepath.Join(s.cfg.ExportDir, "temporal_assertions.json"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var document struct {
		SchemaVersion string                    `json:"schema_version"`
		Assertions    []atlas.TemporalAssertion `json:"assertions"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	circaYears := 10
	if raw := r.URL.Query().Get("circa-years"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > 100 {
			writeError(w, http.StatusBadRequest, errors.New("circa-years must be between 0 and 100"))
			return
		}
		circaYears = n
	}
	sourceType, origin := r.URL.Query().Get("source-type"), r.URL.Query().Get("origin")
	type viewAssertion struct {
		atlas.TemporalAssertion
		EffectiveInterval *atlas.EffectiveInterval `json:"effective_interval"`
	}
	out := make([]viewAssertion, 0, len(document.Assertions))
	for _, a := range document.Assertions {
		if sourceType != "" && a.SourceTypeID != sourceType {
			continue
		}
		if origin != "" && a.Scope.Origin != origin {
			continue
		}
		item := viewAssertion{TemporalAssertion: a}
		if interval, ok := atlas.ApplyTemporalPolicy(a.Value, circaYears); ok {
			item.EffectiveInterval = &interval
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema_version": "1.0.0",
		"policy":         map[string]int{"circa_years": circaYears},
		"assertions":     out,
	})
}

func (s *server) geometryDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("POST required"))
		return
	}
	var request curation.DraftRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.curation.Generate(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) locationsRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/locations/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "geometries" {
		http.NotFound(w, r)
		return
	}
	placeID := parts[0]
	switch r.Method {
	case http.MethodGet:
		history := r.URL.Query().Get("history") == "true"
		var (
			revisions []locationstore.GeometryRevision
			err       error
		)
		if history {
			revisions, err = s.locations.ListRevisions(r.Context(), placeID, r.URL.Query().Get("variant-id"))
		} else {
			revisions, err = s.locations.ListCurrent(r.Context(), placeID)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"place_id": placeID, "revisions": revisions})
	case http.MethodPost:
		var input locationstore.SaveRevision
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if input.PlaceID != "" && input.PlaceID != placeID {
			writeError(w, http.StatusBadRequest, errors.New("body place_id does not match URL"))
			return
		}
		input.PlaceID = placeID
		input.Source = "ui"
		// Authentication is not implemented yet. Never trust a caller-supplied actor.
		input.ActorID = nil
		revision, err := s.locations.SaveRevision(r.Context(), input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, revision)
	default:
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET or POST required"))
	}
}

func decodeJSON(r *http.Request, out any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
