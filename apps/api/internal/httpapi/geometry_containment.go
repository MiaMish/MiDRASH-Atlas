package httpapi

import "encoding/json"

type position [2]float64

type decodedGeometry struct {
	kind         string
	point        position
	polygons     [][][]position
	samplePoints []position
	bounds       geometryBounds
}

type geometryBounds struct {
	minX, minY float64
	maxX, maxY float64
}

func geometryWithin(candidateRaw, containerRaw json.RawMessage) bool {
	candidate, ok := decodeGeometry(candidateRaw)
	if !ok {
		return false
	}
	container, ok := decodeGeometry(containerRaw)
	if !ok {
		return false
	}
	return decodedGeometryWithin(candidate, container)
}

func decodedGeometryWithin(candidate, container decodedGeometry) bool {
	if len(container.polygons) == 0 || len(candidate.samplePoints) == 0 {
		return false
	}
	if candidate.bounds.minX < container.bounds.minX ||
		candidate.bounds.maxX > container.bounds.maxX ||
		candidate.bounds.minY < container.bounds.minY ||
		candidate.bounds.maxY > container.bounds.maxY {
		return false
	}
	for _, point := range candidate.samplePoints {
		if !pointInPolygons(point, container.polygons) {
			return false
		}
	}
	return true
}

func decodeGeometry(raw json.RawMessage) (decodedGeometry, bool) {
	var envelope struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return decodedGeometry{}, false
	}
	switch envelope.Type {
	case "Point":
		var coordinates []float64
		if json.Unmarshal(envelope.Coordinates, &coordinates) != nil || len(coordinates) < 2 {
			return decodedGeometry{}, false
		}
		point := position{coordinates[0], coordinates[1]}
		points := []position{point}
		return decodedGeometry{
			kind: envelope.Type, point: point, samplePoints: points,
			bounds: boundsForPoints(points),
		}, true
	case "Polygon":
		var polygon [][]position
		if json.Unmarshal(envelope.Coordinates, &polygon) != nil || len(polygon) == 0 {
			return decodedGeometry{}, false
		}
		points := polygonSamplePoints([][][]position{polygon})
		return decodedGeometry{
			kind: envelope.Type, polygons: [][][]position{polygon},
			samplePoints: points, bounds: boundsForPoints(points),
		}, true
	case "MultiPolygon":
		var polygons [][][]position
		if json.Unmarshal(envelope.Coordinates, &polygons) != nil || len(polygons) == 0 {
			return decodedGeometry{}, false
		}
		points := polygonSamplePoints(polygons)
		return decodedGeometry{
			kind: envelope.Type, polygons: polygons,
			samplePoints: points, bounds: boundsForPoints(points),
		}, true
	default:
		return decodedGeometry{}, false
	}
}

func boundsForPoints(points []position) geometryBounds {
	bounds := geometryBounds{
		minX: points[0][0], maxX: points[0][0],
		minY: points[0][1], maxY: points[0][1],
	}
	for _, point := range points[1:] {
		if point[0] < bounds.minX {
			bounds.minX = point[0]
		}
		if point[0] > bounds.maxX {
			bounds.maxX = point[0]
		}
		if point[1] < bounds.minY {
			bounds.minY = point[1]
		}
		if point[1] > bounds.maxY {
			bounds.maxY = point[1]
		}
	}
	return bounds
}

func polygonSamplePoints(polygons [][][]position) []position {
	var points []position
	for _, polygon := range polygons {
		for _, ring := range polygon {
			for index, point := range ring {
				points = append(points, point)
				if index > 0 {
					previous := ring[index-1]
					points = append(points, position{
						(previous[0] + point[0]) / 2,
						(previous[1] + point[1]) / 2,
					})
				}
			}
		}
	}
	return points
}

func pointInPolygons(point position, polygons [][][]position) bool {
	for _, polygon := range polygons {
		if pointInPolygon(point, polygon) {
			return true
		}
	}
	return false
}

func pointInPolygon(point position, polygon [][]position) bool {
	if len(polygon) == 0 || !pointInRing(point, polygon[0]) {
		return false
	}
	for _, hole := range polygon[1:] {
		if pointInRing(point, hole) && !pointOnRing(point, hole) {
			return false
		}
	}
	return true
}

func pointInRing(point position, ring []position) bool {
	if pointOnRing(point, ring) {
		return true
	}
	inside := false
	for index, current := range ring {
		previous := ring[(index+len(ring)-1)%len(ring)]
		if (current[1] > point[1]) == (previous[1] > point[1]) {
			continue
		}
		x := (previous[0]-current[0])*(point[1]-current[1])/
			(previous[1]-current[1]) + current[0]
		if point[0] < x {
			inside = !inside
		}
	}
	return inside
}

func pointOnRing(point position, ring []position) bool {
	const epsilon = 1e-9
	for index, current := range ring {
		previous := ring[(index+len(ring)-1)%len(ring)]
		lengthSquared := (current[0]-previous[0])*(current[0]-previous[0]) +
			(current[1]-previous[1])*(current[1]-previous[1])
		if lengthSquared <= epsilon {
			distanceSquared := (point[0]-previous[0])*(point[0]-previous[0]) +
				(point[1]-previous[1])*(point[1]-previous[1])
			if distanceSquared <= epsilon {
				return true
			}
			continue
		}
		cross := (point[1]-previous[1])*(current[0]-previous[0]) -
			(point[0]-previous[0])*(current[1]-previous[1])
		if cross < -epsilon || cross > epsilon {
			continue
		}
		dot := (point[0]-previous[0])*(current[0]-previous[0]) +
			(point[1]-previous[1])*(current[1]-previous[1])
		if dot >= -epsilon && dot <= lengthSquared+epsilon {
			return true
		}
	}
	return false
}
