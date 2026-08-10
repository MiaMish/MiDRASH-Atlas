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
  ai_status:
    | "not_attempted"
    | "candidate_proposed"
    | "candidate_unavailable"
    | "needs_candidates"
    | "ambiguous"
    | "technical_failure";
  ai_comment?: string;
  ai_history: AIAuditEntry[];
};

export type AIAuditEntry = {
  run_id: string;
  generated_at: string;
  provider: string;
  model: string;
  status:
    | "candidate_proposed"
    | "needs_candidates"
    | "ambiguous"
    | "technical_failure"
    | "skipped_human_reviewed"
    | "skipped_ai_already_checked"
    | "skipped";
  comment?: string;
  error?: string;
  prompt_hash?: string;
  current: boolean;
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

export type TemporalValue = {
  kind: string;
  display?: string;
  date?: string;
  year?: number;
  start_year?: number;
  end_year?: number;
  century?: number;
  start_century?: number;
  end_century?: number;
  part?: string;
  approximate: boolean;
  uncertain: boolean;
};

export type AtlasTemporalAssertion = {
  id: string;
  source_type_id: string;
  scope: {
    target_record_id: string;
    source_record_id: string;
    physical_id: string;
    origin: string;
  };
  value: TemporalValue;
  confidence: string;
  evidence: {
    raw: string;
    marc_tag?: string;
    marc_subfield?: string;
    catalog: string;
    extraction: string;
    review_status: string;
  };
  effective_interval?: { start_year: number; end_year: number };
};

export type AtlasRecord = {
  id: string;
  physical_id: string;
  kind: string;
  parent_ids?: string[];
  parent_records?: CatalogRecordSummary[];
  titles: string[];
  primary_titles?: string[];
  alternative_titles?: string[];
  languages?: string[];
  script_styles?: string[];
  extent?: string[];
  dimensions?: string[];
  contributors?: Array<{
    name: string;
    role?: string;
    marc_tag: string;
  }>;
  shelfmarks?: Array<{
    repository?: string;
    locality?: string;
    country?: string;
    shelfmark?: string;
  }>;
  digitized: boolean;
  public_record_url?: string;
  resolver_url?: string;
  source_modified?: string;
  provenance_notes?: string[];
  colophon_notes?: string[];
  physical_notes?: string[];
  contents?: string[];
  general_notes?: string[];
  current_owners?: Array<{
    name: string;
    locality?: string;
    country?: string;
    roles?: string[];
  }>;
};

export type CatalogRecordSummary = {
  id: string;
  kind: string;
  titles: string[];
  primary_titles?: string[];
  alternative_titles?: string[];
};

export type AtlasEvent = {
  assertion: {
    id: string;
    source_type_id: string;
    scope: {
      target_record_id: string;
      source_record_id: string;
      physical_id: string;
      origin: string;
    };
    place_id: string;
    place_raw: string;
    role?: string;
    confidence: string;
    evidence: {
      raw: string;
      marc_tag?: string;
      marc_subfield?: string;
      marc_role?: string;
      catalog: string;
      extraction: string;
      review_status: string;
    };
  };
  record: AtlasRecord;
  source_record?: CatalogRecordSummary;
  hiburim: Array<{ id: string; label: string; english?: string }>;
  temporal_assertions: AtlasTemporalAssertion[];
};

export type AtlasFeature = {
  type: "Feature";
  id: string;
  geometry: GeoJSONGeometry;
  properties: {
    place_id: string;
    place_label: string;
    coordinate_status: LocationOverview["coordinate_status"];
    reviewed_by_human: boolean;
    changed_by_human: boolean;
    spatial_precision?: string;
    geometry_source?: string;
    ai_status: LocationOverview["ai_status"];
    assertion_count: number;
    record_count: number;
    contained_place_ids: string[];
    events: AtlasEvent[];
  };
};

export type AtlasUnmappedGroup = {
  place_id: string;
  place_label: string;
  coordinate_status: string;
  ai_status: LocationOverview["ai_status"];
  assertion_count: number;
  record_count: number;
  events: AtlasEvent[];
};

export type AtlasView = {
  schema_version: string;
  time_bounds: { min_year: number; max_year: number };
  applied_filters: AtlasFilters;
  summary: {
    matched_assertions: number;
    mapped_assertions: number;
    unmapped_assertions: number;
    mapped_places: number;
    matched_records: number;
  };
  facets: {
    geo_sources: Array<{
      id: string;
      label: string;
      short_description: string;
      default_enabled: boolean;
      count: number;
    }>;
    origins: Array<{ id: string; label: string; count: number }>;
    location_statuses: Array<{ id: string; label: string; count: number }>;
    hiburim: Array<{ id: string; label: string; english?: string; count: number }>;
  };
  features: AtlasFeature[];
  unmapped_groups: AtlasUnmappedGroup[];
};

export type AtlasFilters = {
  start_year?: number;
  end_year?: number;
  circa_years: number;
  include_undated: boolean;
  geo_source_ids: string[];
  origins: string[];
  hibur_ids: string[];
  location_statuses: string[];
};

export async function fetchAtlasView(
  filters: AtlasFilters,
  signal?: AbortSignal,
): Promise<AtlasView> {
  const query = new URLSearchParams();
  if (filters.start_year !== undefined) query.set("start-year", String(filters.start_year));
  if (filters.end_year !== undefined) query.set("end-year", String(filters.end_year));
  query.set("circa-years", String(filters.circa_years));
  query.set("include-undated", String(filters.include_undated));
  if (filters.geo_source_ids.length) query.set("geo-source", filters.geo_source_ids.join(","));
  if (filters.origins.length) query.set("origin", filters.origins.join(","));
  if (filters.hibur_ids.length) query.set("hibur-id", filters.hibur_ids.join(","));
  if (filters.location_statuses.length) {
    query.set("location-status", filters.location_statuses.join(","));
  }
  const response = await fetch(`/api/v1/atlas-view?${query.toString()}`, { signal });
  if (!response.ok) throw new Error(await response.text());
  return response.json() as Promise<AtlasView>;
}

export type CanonicalLayer = "work" | "item" | "text";

export type CanonicalEvidence = {
  source_id: string;
  source_record?: string;
  field?: string;
  raw: string;
  extraction: string;
  review_status: string;
};

export type CanonicalWork = {
  id: string;
  layer: "work";
  title: string;
  hebrew_title?: string;
  aliases?: string[];
  families?: string[];
  identifiers?: Array<{ scheme: string; value: string }>;
  evidence?: CanonicalEvidence[];
};

export type CanonicalItem = {
  id: string;
  layer: "item";
  kind: "manuscript" | "printed_edition";
  title: string;
  work_links: Array<{
    work_id: string;
    relation: string;
    raw_label?: string;
    source_ref?: string;
  }> | null;
  parts?: Array<{
    id: string;
    kind: string;
    label: string;
    range?: string;
    work_ids?: string[];
    evidence?: CanonicalEvidence[];
  }>;
  attributes?: Array<{
    key: string;
    label: string;
    values: string[];
    evidence: CanonicalEvidence[];
  }>;
  identifiers?: Array<{ scheme: string; value: string }>;
  evidence?: CanonicalEvidence[];
};

export type CanonicalText = {
  id: string;
  layer: "text";
  work_id: string;
  item_id?: string;
  citation: string;
  language: string;
  content: string;
  status: string;
};

export type CanonicalEvent = {
  id: string;
  layer: CanonicalLayer;
  type: string;
  label: string;
  subject: { layer: CanonicalLayer; id: string };
  related?: Array<{ layer: CanonicalLayer; id: string; relation: string }>;
  agents?: Array<{ agent_id: string; role: string }>;
  places?: Array<{
    place_id?: string;
    raw_name: string;
    role: string;
    confidence?: string;
    evidence: CanonicalEvidence[];
  }>;
  times?: Array<{
    value: TemporalValue;
    confidence?: string;
    evidence: CanonicalEvidence[];
  }>;
  evidence: CanonicalEvidence[];
  review_state: string;
};

export type CanonicalPlace = {
  id: string;
  label: string;
  display_geometry?: {
    variant_id: string;
    type: "Point";
    coordinates: [number, number];
    spatial_precision: string;
    review_status: string;
    interpretation_note: string;
    evidence: CanonicalEvidence[];
  };
};

export type CanonicalAtlas = {
  schema_version: string;
  layer_states: Array<{ layer: CanonicalLayer; status: string; note?: string }>;
  works: CanonicalWork[];
  items: CanonicalItem[];
  texts: CanonicalText[];
  events: CanonicalEvent[];
  places: CanonicalPlace[];
  agents: Array<{ id: string; label: string; aliases?: string[] }>;
  sources: Array<{ id: string; type: string; label: string; citation?: string; url?: string }>;
};

export async function fetchCanonicalAtlas(signal?: AbortSignal): Promise<CanonicalAtlas> {
  const response = await fetch("/api/v1/atlas", { signal });
  if (!response.ok) throw new Error(await response.text());
  return response.json() as Promise<CanonicalAtlas>;
}
