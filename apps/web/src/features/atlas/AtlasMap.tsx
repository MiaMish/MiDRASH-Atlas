import { useEffect, useMemo, useState } from "react";
import {
  GeoJSON,
  MapContainer,
  Rectangle,
  TileLayer,
  useMap,
  useMapEvents,
} from "react-leaflet";
import {
  circleMarker,
  geoJSON,
  latLngBounds,
  type LatLng,
  type PathOptions,
} from "leaflet";
import type { Feature, FeatureCollection, Geometry } from "geojson";
import type { AtlasFeature } from "../../api";
import type { SpatialBounds } from "./spatial";

type Props = {
  features: AtlasFeature[];
  selectedPlaceId: string;
  containedPlaceIds: string[];
  areaPlaceIds: string[];
  selectedBounds: SpatialBounds | null;
  onSelectBounds: (bounds: SpatialBounds | null) => void;
  onSelectPlace: (placeId: string) => void;
};

const statusColors: Record<string, string> = {
  reviewed_by_human: "#1f6b57",
  changed_and_reviewed_by_human: "#1f6b57",
  changed_by_human: "#a56d12",
  human_draft: "#a56d12",
  unreviewed_candidate: "#a65332",
  curated_unreviewed: "#a65332",
};

function featureStyle(
  feature: AtlasFeature,
  selectedPlaceId: string,
  containedPlaceIds: Set<string>,
  areaPlaceIds: Set<string>,
): PathOptions {
  const selected = feature.properties.place_id === selectedPlaceId;
  const contained = containedPlaceIds.has(feature.properties.place_id);
  const inArea = areaPlaceIds.has(feature.properties.place_id);
  const color = statusColors[feature.properties.coordinate_status] ?? "#6d685f";
  return {
    color: inArea ? "#245f8f" : color,
    fillColor: color,
    fillOpacity: selected ? 0.42 : inArea ? 0.36 : 0.22,
    opacity: 0.9,
    weight: selected ? 4 : inArea || contained ? 3 : 1.5,
    dashArray: contained ? "5 4" : undefined,
  };
}

function FitFeatures({ data }: { data: FeatureCollection }) {
  const map = useMap();
  useEffect(() => {
    if (!data.features.length) return;
    const bounds = geoJSON(data).getBounds();
    if (bounds.isValid()) {
      map.fitBounds(bounds, { padding: [32, 32], maxZoom: 7 });
    }
  }, [data, map]);
  return null;
}

function RectangleSelector({
  active,
  selectedBounds,
  onComplete,
}: {
  active: boolean;
  selectedBounds: SpatialBounds | null;
  onComplete: (bounds: SpatialBounds) => void;
}) {
  const map = useMap();
  const [start, setStart] = useState<LatLng | null>(null);
  const [current, setCurrent] = useState<LatLng | null>(null);

  useEffect(() => {
    if (active) {
      map.dragging.disable();
      map.getContainer().classList.add("drawing-area");
    } else {
      map.dragging.enable();
      map.getContainer().classList.remove("drawing-area");
      setStart(null);
      setCurrent(null);
    }
    return () => {
      map.dragging.enable();
      map.getContainer().classList.remove("drawing-area");
    };
  }, [active, map]);

  useMapEvents({
    mousedown(event) {
      if (!active) return;
      setStart(event.latlng);
      setCurrent(event.latlng);
    },
    mousemove(event) {
      if (active && start) setCurrent(event.latlng);
    },
    mouseup(event) {
      if (!active || !start) return;
      const bounds = latLngBounds(start, event.latlng);
      onComplete({
        west: bounds.getWest(),
        south: bounds.getSouth(),
        east: bounds.getEast(),
        north: bounds.getNorth(),
      });
      setStart(null);
      setCurrent(null);
    },
  });

  const preview = start && current ? latLngBounds(start, current) : null;
  const persisted = selectedBounds
    ? latLngBounds(
      [selectedBounds.south, selectedBounds.west],
      [selectedBounds.north, selectedBounds.east],
    )
    : null;
  return (
    <>
      {persisted && (
        <Rectangle
          bounds={persisted}
          pathOptions={{ color: "#245f8f", fillOpacity: 0.08, weight: 2 }}
        />
      )}
      {preview && (
        <Rectangle
          bounds={preview}
          pathOptions={{ color: "#245f8f", dashArray: "5 4", fillOpacity: 0.12, weight: 2 }}
        />
      )}
    </>
  );
}

export function AtlasMap({
  features,
  selectedPlaceId,
  containedPlaceIds,
  areaPlaceIds,
  selectedBounds,
  onSelectBounds,
  onSelectPlace,
}: Props) {
  const [drawingArea, setDrawingArea] = useState(false);
  const containedIDs = useMemo(() => new Set(containedPlaceIds), [containedPlaceIds]);
  const areaIDs = useMemo(() => new Set(areaPlaceIds), [areaPlaceIds]);
  const orderedFeatures = useMemo(() => {
    const containmentDepth = new Map(features.map((feature) => [feature.id, 0]));
    for (const container of features) {
      for (const placeId of container.properties.contained_place_ids ?? []) {
        containmentDepth.set(placeId, (containmentDepth.get(placeId) ?? 0) + 1);
      }
    }
    return features
      .map((feature, index) => ({ feature, index }))
      .sort((left, right) => {
        const depthDifference =
          (containmentDepth.get(left.feature.id) ?? 0) -
          (containmentDepth.get(right.feature.id) ?? 0);
        if (depthDifference !== 0) return depthDifference;
        const leftIsPoint = left.feature.geometry.type === "Point" ? 1 : 0;
        const rightIsPoint = right.feature.geometry.type === "Point" ? 1 : 0;
        return leftIsPoint - rightIsPoint || left.index - right.index;
      })
      .map(({ feature }) => feature);
  }, [features]);
  const data: FeatureCollection = useMemo(
    () => ({
      type: "FeatureCollection",
      features: orderedFeatures.map(
        (item): Feature => ({
          type: "Feature",
          id: item.id,
          geometry: item.geometry as Geometry,
          properties: item.properties,
        }),
      ),
    }),
    [orderedFeatures],
  );
  const featureByID = useMemo(
    () => new Map(features.map((feature) => [feature.id, feature])),
    [features],
  );

  return (
    <div className="atlas-map-wrap">
      <MapContainer center={[29, 18]} zoom={2} className="atlas-map">
        <TileLayer
          attribution="&copy; OpenStreetMap contributors"
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        <FitFeatures data={data} />
        <GeoJSON
          key={`${selectedPlaceId}-${drawingArea}-${features.map((feature) => feature.id).join(",")}`}
          data={data}
          style={(feature) => {
            const atlasFeature = feature?.id ? featureByID.get(String(feature.id)) : undefined;
            return atlasFeature
              ? featureStyle(atlasFeature, selectedPlaceId, containedIDs, areaIDs)
              : {};
          }}
          pointToLayer={(feature, latlng) => {
            const atlasFeature = feature.id ? featureByID.get(String(feature.id)) : undefined;
            const count = atlasFeature?.properties.record_count ?? 1;
            return circleMarker(latlng, {
              ...(atlasFeature
                ? featureStyle(atlasFeature, selectedPlaceId, containedIDs, areaIDs)
                : { color: "#6d685f", fillColor: "#6d685f" }),
              radius: Math.min(18, 6 + Math.sqrt(count) * 2),
            });
          }}
          onEachFeature={(feature, layer) => {
            const atlasFeature = feature.id ? featureByID.get(String(feature.id)) : undefined;
            if (!atlasFeature) return;
            layer.bindTooltip(
              `${atlasFeature.properties.place_label} · ${atlasFeature.properties.record_count} record${
                atlasFeature.properties.record_count === 1 ? "" : "s"
              }`,
            );
            layer.on("click", () => {
              if (!drawingArea) onSelectPlace(atlasFeature.properties.place_id);
            });
          }}
        />
        <RectangleSelector
          active={drawingArea}
          selectedBounds={selectedBounds}
          onComplete={(bounds) => {
            onSelectBounds(bounds);
            setDrawingArea(false);
          }}
        />
      </MapContainer>
      <div className="atlas-spatial-tools">
        <button
          className={drawingArea ? "active" : ""}
          type="button"
          onClick={() => setDrawingArea((currentValue) => !currentValue)}
        >
          {drawingArea ? "Cancel drawing" : "Select area"}
        </button>
        {selectedBounds && (
          <button type="button" onClick={() => onSelectBounds(null)}>Clear area</button>
        )}
      </div>
      {features.length === 0 && (
        <div className="atlas-empty">No mapped places match the current filters.</div>
      )}
      <div className="atlas-map-legend" aria-label="Geometry review legend">
        <span><i className="reviewed" /> Human-reviewed</span>
        <span><i className="changed" /> Human changed</span>
        <span><i className="unreviewed" /> Unreviewed geometry</span>
      </div>
    </div>
  );
}
