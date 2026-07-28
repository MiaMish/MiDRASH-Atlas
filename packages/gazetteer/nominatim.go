package gazetteer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	NominatimSourceID = "nominatim_openstreetmap"
	defaultBaseURL    = "https://nominatim.openstreetmap.org"
)

type NominatimConfig struct {
	BaseURL      string
	CacheDir     string
	UserAgent    string
	MinimumDelay time.Duration
	HTTPClient   *http.Client
}

type Nominatim struct {
	cfg         NominatimConfig
	httpClient  *http.Client
	mu          sync.Mutex
	lastRequest time.Time
}

type AcquisitionProvenance struct {
	Source      string            `json:"source"`
	Query       string            `json:"query"`
	RetrievedAt time.Time         `json:"retrieved_at"`
	CacheKey    string            `json:"cache_key"`
	Parameters  map[string]string `json:"parameters"`
	License     string            `json:"license"`
	Attribution string            `json:"attribution"`
}

type Candidate struct {
	ID          string          `json:"id"`
	Source      string          `json:"source"`
	DisplayName string          `json:"display_name"`
	AuthorityID string          `json:"authority_id"`
	Category    string          `json:"category,omitempty"`
	PlaceType   string          `json:"place_type,omitempty"`
	Importance  float64         `json:"importance,omitempty"`
	BoundingBox json.RawMessage `json:"bounding_box,omitempty"`
	Geometry    json.RawMessage `json:"geometry"`
	Properties  json.RawMessage `json:"source_properties,omitempty"`
}

type SearchResult struct {
	Provenance AcquisitionProvenance `json:"acquisition_provenance"`
	Candidates []Candidate           `json:"candidates"`
	FromCache  bool                  `json:"from_cache"`
}

type cacheEnvelope struct {
	Provenance AcquisitionProvenance `json:"acquisition_provenance"`
	Response   json.RawMessage       `json:"response"`
}

type nominatimResponse struct {
	Type     string `json:"type"`
	License  string `json:"licence"`
	Features []struct {
		Type       string          `json:"type"`
		Properties json.RawMessage `json:"properties"`
		BBox       json.RawMessage `json:"bbox"`
		Geometry   json.RawMessage `json:"geometry"`
	} `json:"features"`
}

type nominatimProperties struct {
	PlaceID     any     `json:"place_id"`
	OSMType     string  `json:"osm_type"`
	OSMID       any     `json:"osm_id"`
	DisplayName string  `json:"display_name"`
	Category    string  `json:"category"`
	Type        string  `json:"type"`
	Importance  float64 `json:"importance"`
}

func NewNominatim(cfg NominatimConfig) (*Nominatim, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultBaseURL
	}
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("nominatim base URL must include scheme and host")
	}
	if strings.TrimSpace(cfg.CacheDir) == "" {
		return nil, fmt.Errorf("nominatim cache directory is required")
	}
	if strings.TrimSpace(cfg.UserAgent) == "" {
		return nil, fmt.Errorf("nominatim identifying User-Agent is required")
	}
	if cfg.MinimumDelay < time.Second {
		cfg.MinimumDelay = 1100 * time.Millisecond
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 45 * time.Second}
	}
	return &Nominatim{cfg: cfg, httpClient: httpClient}, nil
}

func (client *Nominatim) Search(ctx context.Context, query string) (SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return SearchResult{}, fmt.Errorf("gazetteer query is empty")
	}
	parameters := map[string]string{
		"q": query, "format": "geojson", "limit": "5",
		"addressdetails": "1", "namedetails": "1",
		"polygon_geojson": "1", "polygon_threshold": "0.01",
		"accept-language": "en",
	}
	cacheKey := searchCacheKey(parameters)
	cachePath := filepath.Join(client.cfg.CacheDir, cacheKey+".json")
	if result, ok, err := readCache(cachePath); err != nil {
		return SearchResult{}, err
	} else if ok {
		return decodeResult(result, true)
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	if remaining := client.cfg.MinimumDelay - time.Since(client.lastRequest); remaining > 0 {
		select {
		case <-ctx.Done():
			return SearchResult{}, ctx.Err()
		case <-time.After(remaining):
		}
	}
	endpoint, err := url.Parse(strings.TrimRight(client.cfg.BaseURL, "/") + "/search")
	if err != nil {
		return SearchResult{}, err
	}
	values := endpoint.Query()
	for key, value := range parameters {
		values.Set(key, value)
	}
	endpoint.RawQuery = values.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return SearchResult{}, err
	}
	request.Header.Set("User-Agent", client.cfg.UserAgent)
	request.Header.Set("Accept", "application/geo+json, application/json")
	response, err := client.httpClient.Do(request)
	client.lastRequest = time.Now()
	if err != nil {
		return SearchResult{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return SearchResult{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return SearchResult{}, fmt.Errorf("nominatim returned %d: %s", response.StatusCode, truncate(string(body), 500))
	}
	var parsedResponse nominatimResponse
	if err := json.Unmarshal(body, &parsedResponse); err != nil {
		return SearchResult{}, fmt.Errorf("decode nominatim response: %w", err)
	}
	provenance := AcquisitionProvenance{
		Source: NominatimSourceID, Query: query, RetrievedAt: time.Now().UTC(),
		CacheKey: cacheKey, Parameters: parameters, License: parsedResponse.License,
		Attribution: "Data © OpenStreetMap contributors, ODbL 1.0",
	}
	envelope := cacheEnvelope{Provenance: provenance, Response: append(json.RawMessage(nil), body...)}
	if err := writeCache(cachePath, envelope); err != nil {
		return SearchResult{}, err
	}
	return decodeResult(envelope, false)
}

func decodeResult(envelope cacheEnvelope, fromCache bool) (SearchResult, error) {
	var response nominatimResponse
	if err := json.Unmarshal(envelope.Response, &response); err != nil {
		return SearchResult{}, err
	}
	result := SearchResult{Provenance: envelope.Provenance, FromCache: fromCache}
	for _, feature := range response.Features {
		var properties nominatimProperties
		if err := json.Unmarshal(feature.Properties, &properties); err != nil {
			return SearchResult{}, err
		}
		authorityID := osmAuthorityID(properties.OSMType, properties.OSMID, properties.PlaceID)
		result.Candidates = append(result.Candidates, Candidate{
			ID: stableCandidateID(authorityID), Source: NominatimSourceID,
			DisplayName: properties.DisplayName, AuthorityID: authorityID,
			Category: properties.Category, PlaceType: properties.Type,
			Importance: properties.Importance, BoundingBox: feature.BBox,
			Geometry: feature.Geometry, Properties: feature.Properties,
		})
	}
	return result, nil
}

func readCache(path string) (cacheEnvelope, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cacheEnvelope{}, false, nil
	}
	if err != nil {
		return cacheEnvelope{}, false, err
	}
	var envelope cacheEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return cacheEnvelope{}, false, err
	}
	return envelope, true, nil
}

func writeCache(path string, envelope cacheEnvelope) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(filepath.Dir(path), "nominatim-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func searchCacheKey(parameters map[string]string) string {
	keys := []string{"q", "format", "limit", "addressdetails", "namedetails", "polygon_geojson", "polygon_threshold", "accept-language"}
	var parts []string
	for _, key := range keys {
		parts = append(parts, key+"="+parameters[key])
	}
	hash := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(hash[:])
}

func osmAuthorityID(osmType string, osmID, placeID any) string {
	id := scalarString(osmID)
	if osmType != "" && id != "" {
		return "osm:" + strings.ToLower(osmType) + ":" + id
	}
	return "nominatim:place:" + scalarString(placeID)
}

func scalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case json.Number:
		return typed.String()
	default:
		return fmt.Sprint(value)
	}
}

func stableCandidateID(authorityID string) string {
	hash := sha256.Sum256([]byte(authorityID))
	return "gaz_" + hex.EncodeToString(hash[:])[:16]
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
