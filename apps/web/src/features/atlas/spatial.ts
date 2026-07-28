import type { GeoJSONGeometry } from "../../api";

export type SpatialBounds = {
  west: number;
  south: number;
  east: number;
  north: number;
};

type Position = [number, number];

function position(value: unknown): Position | null {
  if (
    !Array.isArray(value) ||
    value.length < 2 ||
    typeof value[0] !== "number" ||
    typeof value[1] !== "number"
  ) {
    return null;
  }
  return [value[0], value[1]];
}

function inBounds([x, y]: Position, bounds: SpatialBounds) {
  return x >= bounds.west && x <= bounds.east && y >= bounds.south && y <= bounds.north;
}

function onSegment(point: Position, start: Position, end: Position) {
  const epsilon = 1e-9;
  const cross =
    (point[1] - start[1]) * (end[0] - start[0]) -
    (point[0] - start[0]) * (end[1] - start[1]);
  if (Math.abs(cross) > epsilon) return false;
  return (
    point[0] >= Math.min(start[0], end[0]) - epsilon &&
    point[0] <= Math.max(start[0], end[0]) + epsilon &&
    point[1] >= Math.min(start[1], end[1]) - epsilon &&
    point[1] <= Math.max(start[1], end[1]) + epsilon
  );
}

function orientation(first: Position, second: Position, third: Position) {
  return (
    (second[1] - first[1]) * (third[0] - second[0]) -
    (second[0] - first[0]) * (third[1] - second[1])
  );
}

function segmentsIntersect(
  firstStart: Position,
  firstEnd: Position,
  secondStart: Position,
  secondEnd: Position,
) {
  const o1 = orientation(firstStart, firstEnd, secondStart);
  const o2 = orientation(firstStart, firstEnd, secondEnd);
  const o3 = orientation(secondStart, secondEnd, firstStart);
  const o4 = orientation(secondStart, secondEnd, firstEnd);
  if ((o1 > 0) !== (o2 > 0) && (o3 > 0) !== (o4 > 0)) return true;
  return (
    (Math.abs(o1) < 1e-9 && onSegment(secondStart, firstStart, firstEnd)) ||
    (Math.abs(o2) < 1e-9 && onSegment(secondEnd, firstStart, firstEnd)) ||
    (Math.abs(o3) < 1e-9 && onSegment(firstStart, secondStart, secondEnd)) ||
    (Math.abs(o4) < 1e-9 && onSegment(firstEnd, secondStart, secondEnd))
  );
}

function pointInRing(pointValue: Position, ring: unknown[]) {
  const points = ring.map(position).filter((item): item is Position => item !== null);
  if (points.length < 3) return false;
  let inside = false;
  for (let index = 0, previous = points.length - 1; index < points.length; previous = index++) {
    const currentPoint = points[index];
    const previousPoint = points[previous];
    if (onSegment(pointValue, previousPoint, currentPoint)) return true;
    if (
      (currentPoint[1] > pointValue[1]) !== (previousPoint[1] > pointValue[1]) &&
      pointValue[0] <
        ((previousPoint[0] - currentPoint[0]) *
          (pointValue[1] - currentPoint[1])) /
          (previousPoint[1] - currentPoint[1]) +
          currentPoint[0]
    ) {
      inside = !inside;
    }
  }
  return inside;
}

function pointInPolygon(pointValue: Position, polygon: unknown[]) {
  if (!Array.isArray(polygon[0]) || !pointInRing(pointValue, polygon[0])) return false;
  return !polygon.slice(1).some(
    (ring) => Array.isArray(ring) && pointInRing(pointValue, ring),
  );
}

function polygonIntersectsBounds(polygon: unknown[], bounds: SpatialBounds) {
  const corners: Position[] = [
    [bounds.west, bounds.south],
    [bounds.east, bounds.south],
    [bounds.east, bounds.north],
    [bounds.west, bounds.north],
  ];
  if (corners.some((corner) => pointInPolygon(corner, polygon))) return true;
  const boxEdges = corners.map(
    (corner, index): [Position, Position] => [corner, corners[(index + 1) % corners.length]],
  );
  for (const rawRing of polygon) {
    if (!Array.isArray(rawRing)) continue;
    const ring = rawRing.map(position).filter((item): item is Position => item !== null);
    if (ring.some((pointValue) => inBounds(pointValue, bounds))) return true;
    for (let index = 1; index < ring.length; index++) {
      if (
        boxEdges.some(([start, end]) =>
          segmentsIntersect(ring[index - 1], ring[index], start, end),
        )
      ) {
        return true;
      }
    }
  }
  return false;
}

export function geometryIntersectsBounds(
  geometry: GeoJSONGeometry,
  bounds: SpatialBounds,
) {
  if (geometry.type === "Point") {
    const pointValue = position(geometry.coordinates);
    return pointValue ? inBounds(pointValue, bounds) : false;
  }
  if (geometry.type === "Polygon") {
    return Array.isArray(geometry.coordinates)
      ? polygonIntersectsBounds(geometry.coordinates, bounds)
      : false;
  }
  if (!Array.isArray(geometry.coordinates)) return false;
  return geometry.coordinates.some(
    (polygon) => Array.isArray(polygon) && polygonIntersectsBounds(polygon, bounds),
  );
}
