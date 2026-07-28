# PoC architecture and UI contract

## Components

```text
NLI SRU + project CSVs
          |
       Go export
          |
  evidence-preserving JSON
          |
  small Go HTTP handler
          |
 React application
          |
 replaceable map adapter
          |
 Leaflet initially
```

The Go HTTP handlers are stateless with respect to process memory. Read-only
atlas JSON can still be served from a CDN. Location revisions use a repository
interface backed locally by pure-Go SQLite. A serverless deployment can replace
that repository with DynamoDB, Cosmos DB, or PostgreSQL without changing the
HTTP or UI contracts.

## Export files

| File | Purpose |
|---|---|
| `manifest.json` | Counts and QA summary |
| `source_types.json` | UI labels, descriptions, defaults, and priorities |
| `records.json` | Manuscripts/components, Hibur links, filter metadata |
| `hiburim.json` | Hibur ontology subset |
| `geo_assertions.json` | All geographic evidence candidates |
| `temporal_assertions.json` | Stored date semantics |
| `places.json` | Place concepts and contextual geometry variants |
| `events.geojson` | GeoJSON view; geometry is null until reviewed variants exist |
| `review_queue.json` | Unparsed or interpretive work requiring review |

`configs/atlas/place_geometries.json` is the version-controlled seed input for
geometry variants; `data/generated/atlas/places.json` is generated output.
Interactive changes are append-only revisions in the shared SQLite store and
are not written over generated source data.

Gazetteer lookup sits on the ingestion/curation side of the boundary. Normal map
rendering reads persisted geometry and does not call an external geocoder.
Every AI-produced JSON object records provider, model, UTC generation time,
purpose, and prompt hash.

## Source selection

The UI should support three modes:

1. **single source**: show one selected `source_type_id`;
2. **comparison**: show two or more sources with distinct symbology;
3. **priority profile**: select the highest-priority available source for each
   target, while retaining an “alternatives available” indicator.

A profile should separately configure:

- enabled geo assertion source types and their ordering;
- scope precedence (`direct`, `parent_fallback`, `parent_alternative`);
- geometry interpretation sources;
- temporal source types;
- the runtime `circa_years` policy;
- whether uncertain/unreviewed assertions are shown.

Multiple places from the same winning source remain multiple assertions. A
priority profile must not arbitrarily select only the first.

## Map abstraction

React components should depend on a small `MapAdapter`, not directly on Leaflet:

```ts
interface MapAdapter {
  setFeatures(features: GeoJSON.FeatureCollection): void;
  setSelectedFeature(id: string | null): void;
  fitToFeatures(): void;
  onFeatureClick(handler: (id: string) => void): () => void;
  destroy(): void;
}
```

The initial implementation can wrap Leaflet. Replacing it with MapLibre should
not change filters, assertion selection, drill-down, or API contracts.

The React atlas is under `apps/web/src/features/atlas/`. Leaflet-specific atlas
behavior is isolated in `AtlasMap.tsx`; filters and drill-down consume the
backend contract rather than Leaflet objects. Location-editing map behavior
remains isolated in `GeometryMapEditor.tsx`.

## Filters

First-class filters:

- time interval and temporal source;
- Hibur and family;
- geo source type and scope origin;
- geometry interpretation source;
- place/country;
- script style and language;
- digitized status;
- confidence, uncertainty, and review status.

The UI must display mapped, unmapped, and undated counts. Missing geometry must
not silently remove a record from the result total.

## Drill-down

1. place/cluster/region;
2. physical manuscripts in the active filter;
3. selected manuscript;
4. analytic components and Hibur links;
5. all competing geo and temporal assertions;
6. raw evidence, source description, parent/direct origin, NLI link, and digital
   object link.

Default counts group by `physical_id`. A visible toggle may count components or
manifestations instead.

## Joined atlas endpoint

`GET /api/v1/atlas-view` joins:

- geographic assertions and source descriptions;
- effective temporal intervals produced by the runtime circa policy;
- manuscript/component records and shelfmarks;
- many-to-many Hibur links;
- current reviewed, human-edited, or AI-draft modern geometry.

The response groups events by place, avoiding repeated country or region
polygons. Each feature retains nested event records for drill-down. Supported
query parameters are `start-year`, `end-year`, `circa-years`,
`include-undated`, `geo-source`, `origin`, `hibur-id`, and
`location-status`. Comma-separated values implement multi-selection.

The UI defaults to `nli_751_writing_place`. Current repositories and related
places remain available but are not silently mixed into the historical default.
Mapped and unmapped assertion counts are returned together so missing geometry
is visible rather than dropped.
