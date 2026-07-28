import { useEffect, useMemo } from "react";
import { GeoJSON, MapContainer, TileLayer, useMap } from "react-leaflet";
import {
  circleMarker,
  geoJSON,
  type PathOptions,
} from "leaflet";
import type { Feature, FeatureCollection, Geometry } from "geojson";
import type { AtlasFeature } from "../../api";

type Props = {
  features: AtlasFeature[];
  selectedPlaceId: string;
  containedPlaceIds: string[];
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
): PathOptions {
  const selected = feature.properties.place_id === selectedPlaceId;
  const contained = containedPlaceIds.has(feature.properties.place_id);
  const color = statusColors[feature.properties.coordinate_status] ?? "#6d685f";
  return {
    color,
    fillColor: color,
    fillOpacity: selected ? 0.42 : 0.22,
    opacity: 0.9,
    weight: selected ? 4 : contained ? 3 : 1.5,
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

export function AtlasMap({
  features,
  selectedPlaceId,
  containedPlaceIds,
  onSelectPlace,
}: Props) {
  const containedIDs = useMemo(() => new Set(containedPlaceIds), [containedPlaceIds]);
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
          key={`${selectedPlaceId}-${features.map((feature) => feature.id).join(",")}`}
          data={data}
          style={(feature) => {
            const atlasFeature = feature?.id ? featureByID.get(String(feature.id)) : undefined;
            return atlasFeature
              ? featureStyle(atlasFeature, selectedPlaceId, containedIDs)
              : {};
          }}
          pointToLayer={(feature, latlng) => {
            const atlasFeature = feature.id ? featureByID.get(String(feature.id)) : undefined;
            const count = atlasFeature?.properties.record_count ?? 1;
            return circleMarker(latlng, {
              ...(atlasFeature
                ? featureStyle(atlasFeature, selectedPlaceId, containedIDs)
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
            layer.on("click", () => onSelectPlace(atlasFeature.properties.place_id));
          }}
        />
      </MapContainer>
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
