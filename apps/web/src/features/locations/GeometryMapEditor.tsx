import { GeoJSON, MapContainer, Marker, TileLayer, useMap, useMapEvents } from "react-leaflet";
import { geoJSON, type DragEndEvent } from "leaflet";
import type { Feature, Geometry } from "geojson";
import { useEffect } from "react";
import type { GeoJSONGeometry } from "../../api";

type Props = {
  geometry: GeoJSONGeometry | null;
  onGeometryChange: (geometry: GeoJSONGeometry) => void;
};

function Recenter({ geometry }: { geometry: GeoJSONGeometry | null }) {
  const map = useMap();
  useEffect(() => {
    if (!geometry) return;
    if (geometry.type === "Point") {
      const [longitude, latitude] = geometry.coordinates as [number, number];
      map.setView([latitude, longitude], Math.max(map.getZoom(), 6));
      return;
    }
    const feature: Feature = {
      type: "Feature",
      properties: {},
      geometry: geometry as Geometry,
    };
    const bounds = geoJSON(feature).getBounds();
    if (bounds.isValid()) {
      map.fitBounds(bounds, { padding: [28, 28], maxZoom: 8 });
    }
  }, [geometry, map]);
  return null;
}

function AddPointOnClick({
  enabled,
  onGeometryChange,
}: {
  enabled: boolean;
  onGeometryChange: Props["onGeometryChange"];
}) {
  useMapEvents({
    click(event) {
      if (enabled) {
        onGeometryChange({
          type: "Point",
          coordinates: [event.latlng.lng, event.latlng.lat],
        });
      }
    },
  });
  return null;
}

export function GeometryMapEditor({ geometry, onGeometryChange }: Props) {
  const point =
    geometry?.type === "Point"
      ? (geometry.coordinates as [number, number])
      : ([0, 20] as [number, number]);
  const feature: Feature | null = geometry ? {
    type: "Feature",
    properties: {},
    geometry: geometry as Geometry,
  } : null;

  return (
    <MapContainer center={[point[1], point[0]]} zoom={3} className="location-map">
      <TileLayer
        attribution="&copy; OpenStreetMap contributors"
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <Recenter geometry={geometry} />
      <AddPointOnClick enabled={!geometry} onGeometryChange={onGeometryChange} />
      {!geometry ? (
        <div className="map-empty-note">No valid coordinates · click the map to add a point</div>
      ) : geometry.type === "Point" ? (
        <Marker
          draggable
          position={[point[1], point[0]]}
          eventHandlers={{
            dragend: (event: DragEndEvent) => {
              const next = event.target.getLatLng();
              onGeometryChange({
                type: "Point",
                coordinates: [next.lng, next.lat],
              });
            },
          }}
        />
      ) : (
        <GeoJSON
          key={JSON.stringify(geometry)}
          data={feature!}
        />
      )}
    </MapContainer>
  );
}
