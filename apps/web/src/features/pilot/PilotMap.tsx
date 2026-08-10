import { useEffect, useMemo } from "react";
import { CircleMarker, MapContainer, Popup, TileLayer, useMap } from "react-leaflet";
import { latLngBounds } from "leaflet";
import type {
  CanonicalEvent,
  CanonicalLayer,
  CanonicalPlace,
} from "../../api";

type MapPoint = {
  place: CanonicalPlace;
  events: CanonicalEvent[];
  coordinates: [number, number];
};

type Props = {
  layer: CanonicalLayer;
  events: CanonicalEvent[];
  places: CanonicalPlace[];
  selectedEventId: string;
  onSelectEvent: (event: CanonicalEvent) => void;
};

const layerColors: Record<CanonicalLayer, string> = {
  work: "#a44b32",
  item: "#176b68",
  text: "#a77a27",
};

function FitPoints({ points }: { points: MapPoint[] }) {
  const map = useMap();
  useEffect(() => {
    if (!points.length) return;
    if (points.length === 1) {
      map.setView(points[0].coordinates, 4, { animate: false });
      return;
    }
    map.fitBounds(latLngBounds(points.map((point) => point.coordinates)), {
      padding: [52, 52],
      maxZoom: 5,
      animate: false,
    });
  }, [map, points]);
  return null;
}

function eventDate(event: CanonicalEvent) {
  return event.times?.map((time) => time.value.display).filter(Boolean).join(" · ") || "Undated";
}

export function PilotMap({
  layer,
  events,
  places,
  selectedEventId,
  onSelectEvent,
}: Props) {
  const points = useMemo(() => {
    const placeById = new Map(places.map((place) => [place.id, place]));
    const grouped = new Map<string, MapPoint>();
    for (const event of events) {
      for (const assertion of event.places ?? []) {
        if (!assertion.place_id) continue;
        const place = placeById.get(assertion.place_id);
        const coordinates = place?.display_geometry?.coordinates;
        if (!place || !coordinates) continue;
        const current = grouped.get(place.id);
        if (current) {
          if (!current.events.some((item) => item.id === event.id)) current.events.push(event);
        } else {
          grouped.set(place.id, {
            place,
            events: [event],
            coordinates: [coordinates[1], coordinates[0]],
          });
        }
      }
    }
    return [...grouped.values()];
  }, [events, places]);

  return (
    <div className={`pilot-map-frame layer-${layer}`}>
      <MapContainer
        center={[37, 8]}
        zoom={2}
        className="pilot-map"
        scrollWheelZoom
        aria-label={`${layer} layer map`}
      >
        <TileLayer
          attribution="&copy; OpenStreetMap contributors"
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        <FitPoints points={points} />
        {points.map((point) => {
          const selected = point.events.some((event) => event.id === selectedEventId);
          return (
            <CircleMarker
              key={`${layer}-${point.place.id}`}
              center={point.coordinates}
              radius={selected ? 13 : 8 + Math.min(point.events.length, 4)}
              pathOptions={{
                color: selected ? "#18201e" : layerColors[layer],
                fillColor: layerColors[layer],
                fillOpacity: selected ? 0.92 : 0.72,
                opacity: 1,
                weight: selected ? 3 : 2,
              }}
            >
              <Popup>
                <div className="pilot-map-popup">
                  <strong>{point.place.label}</strong>
                  <small>{point.place.display_geometry?.spatial_precision.replaceAll("_", " ")}</small>
                  {point.events.map((event) => (
                    <button key={event.id} type="button" onClick={() => onSelectEvent(event)}>
                      <span>{event.label}</span>
                      <small>{eventDate(event)}</small>
                    </button>
                  ))}
                </div>
              </Popup>
            </CircleMarker>
          );
        })}
      </MapContainer>
      <div className="pilot-map-caption">
        <span><i style={{ background: layerColors[layer] }} /> {layer} assertions</span>
        <span>Display points are approximate and unreviewed</span>
      </div>
      {!points.length && (
        <div className="pilot-map-empty">
          <span>Layer {layer}</span>
          <strong>No mapped assertions yet</strong>
          <p>{layer === "text"
            ? "The selected passage and its named-entity mentions will appear here after the NER step."
            : "No assertion in this view has both a matching place and display geometry under the active filters."}</p>
        </div>
      )}
    </div>
  );
}
