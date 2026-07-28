package gazetteer

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestNominatimCachesExactResponse(t *testing.T) {
	var calls atomic.Int32
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		if request.Header.Get("User-Agent") != "midrash-atlas-test/1.0" {
			t.Fatalf("missing identifying user agent")
		}
		if request.URL.Query().Get("polygon_geojson") != "1" ||
			request.URL.Query().Get("polygon_threshold") != "0.01" {
			t.Fatalf("polygon request parameters missing: %s", request.URL.RawQuery)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/geo+json"}},
			Body: io.NopCloser(strings.NewReader(`{
		  "type":"FeatureCollection",
		  "licence":"Data © OpenStreetMap contributors, ODbL 1.0",
		  "features":[{
		    "type":"Feature",
		    "properties":{
		      "place_id":123,"osm_type":"relation","osm_id":456,
		      "display_name":"Spain","category":"boundary","type":"administrative",
		      "importance":0.9
		    },
		    "bbox":[-10,35,4,44],
		    "geometry":{"type":"Polygon","coordinates":[[[-10,35],[4,35],[4,44],[-10,35]]]}
		  }]
		}`)),
		}, nil
	})}
	client, err := NewNominatim(NominatimConfig{
		BaseURL: "https://nominatim.test", CacheDir: filepath.Join(t.TempDir(), "cache"),
		UserAgent: "midrash-atlas-test/1.0", HTTPClient: httpClient,
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := client.Search(context.Background(), "Spain")
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.Search(context.Background(), "Spain")
	if err != nil {
		t.Fatal(err)
	}
	if first.FromCache || !second.FromCache || calls.Load() != 1 {
		t.Fatalf("cache failure: first=%v second=%v calls=%d", first.FromCache, second.FromCache, calls.Load())
	}
	if len(second.Candidates) != 1 || second.Candidates[0].AuthorityID != "osm:relation:456" {
		t.Fatalf("unexpected candidate: %+v", second.Candidates)
	}
	if second.Provenance.RetrievedAt.IsZero() || second.Provenance.Query != "Spain" {
		t.Fatalf("missing acquisition provenance: %+v", second.Provenance)
	}
}
