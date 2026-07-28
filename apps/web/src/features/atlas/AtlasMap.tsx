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

function featureStyle(feature: AtlasFeature, selectedPlaceId: string): PathOptions {
  const selected = feature.properties.place_id === selectedPlaceId;
  const color = statusColors[feature.properties.coordinate_status] ?? "#6d685f";
  return {
    color,
    fillColor: color,
    fillOpacity: selected ? 0.42 : 0.22,
    opacity: 0.9,
    weight: selected ? 4 : 1.5,
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

export function AtlasMap({ features, selectedPlaceId, onSelectPlace }: Props) {
  const data: FeatureCollection = useMemo(
    () => ({
      type: "FeatureCollection",
      features: features.map(
        (item): Feature => ({
          type: "Feature",
          id: item.id,
          geometry: item.geometry as Geometry,
          properties: item.properties,
        }),
      ),
    }),
    [features],
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
            return atlasFeature ? featureStyle(atlasFeature, selectedPlaceId) : {};
          }}
          pointToLayer={(feature, latlng) => {
            const atlasFeature = feature.id ? featureByID.get(String(feature.id)) : undefined;
            const count = atlasFeature?.properties.record_count ?? 1;
            return circleMarker(latlng, {
              ...(atlasFeature
                ? featureStyle(atlasFeature, selectedPlaceId)
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
