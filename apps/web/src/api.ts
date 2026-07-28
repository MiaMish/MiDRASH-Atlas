export type GeoJSONGeometry = {
  type: "Point" | "Polygon" | "MultiPolygon";
  coordinates: unknown;
};

export type AIProvenance = {
  provider: string;
  model: string;
  generated_at: string;
  purpose: string;
  prompt_hash: string;
};

export type GeometryRevision = {
  id: string;
  place_id: string;
  geometry_variant_id: string;
  revision: number;
  supersedes_revision_id?: string;
  label: string;
  geometry: GeoJSONGeometry;
  valid_from_year?: number;
  valid_to_year?: number;
  spatial_precision: string;
  interpretation_source: string;
  interpretation_note?: string;
  confidence: string;
  review_status: string;
  human_action?: "draft" | "reviewed" | "changed";
  change_reason: string;
  actor_id?: string;
  source: string;
  ai_provenance?: AIProvenance;
  created_at: string;
};

export type Place = {
  id: string;
  preferred_label: string;
  aliases: string[];
  review_status: string;
};

export type LocationOverview = {
  place_id: string;
  place_label: string;
  aliases: string[];
  has_valid_coordinates: boolean;
  coordinate_status:
    | "no_geometry"
    | "unreviewed_candidate"
    | "curated_unreviewed"
    | "human_draft"
    | "reviewed_by_human"
    | "changed_by_human"
    | "changed_and_reviewed_by_human";
  reviewed_by_human: boolean;
  changed_by_human: boolean;
  geometry?: GeoJSONGeometry;
  geometry_variant_id?: string;
  geometry_label?: string;
  spatial_precision?: string;
  interpretation_note?: string;
  geometry_source?: string;
  review_status: string;
  latest_revision?: number;
  ai_provenance?: AIProvenance;
};

export async function fetchLocationOverview(): Promise<LocationOverview[]> {
  const response = await fetch("/api/v1/location-overview");
  if (!response.ok) throw new Error(await response.text());
  const body = (await response.json()) as { locations: LocationOverview[] };
  return body.locations;
}

export async function fetchPlaces(): Promise<Place[]> {
  const response = await fetch("/api/v1/places");
  if (!response.ok) throw new Error(await response.text());
  const body = (await response.json()) as { places: Place[] };
  return body.places;
}

export async function fetchGeometryRevisions(
  placeId: string,
  history = false,
): Promise<GeometryRevision[]> {
  const response = await fetch(
    `/api/v1/locations/${encodeURIComponent(placeId)}/geometries?history=${history}`,
  );
  if (!response.ok) throw new Error(await response.text());
  const body = (await response.json()) as { revisions: GeometryRevision[] };
  return body.revisions ?? [];
}

export type SaveGeometryInput = Omit<
  GeometryRevision,
  "id" | "place_id" | "revision" | "supersedes_revision_id" | "created_at" | "source"
>;

export async function saveGeometryRevision(
  placeId: string,
  input: SaveGeometryInput,
): Promise<GeometryRevision> {
  const response = await fetch(
    `/api/v1/locations/${encodeURIComponent(placeId)}/geometries`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
  if (!response.ok) throw new Error(await response.text());
  return response.json() as Promise<GeometryRevision>;
}
