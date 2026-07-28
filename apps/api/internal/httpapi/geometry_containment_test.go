package httpapi

import (
	"encoding/json"
	"testing"
)

func TestGeometryWithinPolygonAndMultiPolygon(t *testing.T) {
	egypt := json.RawMessage(`{
	  "type":"MultiPolygon",
	  "coordinates":[[[[24,22],[37,22],[37,32],[24,32],[24,22]]]]
	}`)
	cairo := json.RawMessage(`{
	  "type":"Polygon",
	  "coordinates":[[[31.2,29.7],[31.9,29.7],[31.9,30.4],[31.2,30.4],[31.2,29.7]]]
	}`)
	outside := json.RawMessage(`{"type":"Point","coordinates":[40,30]}`)
	if !geometryWithin(cairo, egypt) {
		t.Fatal("expected Cairo polygon to be within Egypt multipolygon")
	}
	if geometryWithin(egypt, cairo) {
		t.Fatal("a containing country must not be within its city")
	}
	if geometryWithin(outside, egypt) {
		t.Fatal("outside point was incorrectly contained")
	}
}

func TestGeometryWithinRespectsPolygonHoles(t *testing.T) {
	container := json.RawMessage(`{
	  "type":"Polygon",
	  "coordinates":[
	    [[0,0],[10,0],[10,10],[0,10],[0,0]],
	    [[4,4],[6,4],[6,6],[4,6],[4,4]]
	  ]
	}`)
	inHole := json.RawMessage(`{"type":"Point","coordinates":[5,5]}`)
	if geometryWithin(inHole, container) {
		t.Fatal("point inside a polygon hole must not be contained")
	}
}
