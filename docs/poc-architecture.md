# PoC architecture and UI contract

## Components

```text
NLI SRU ──> Python retrieval/normalization ──┐
                                             ├──> Go atlas export
project CSVs + atlas configuration ──────────┘          |
                                             evidence-preserving JSON
                                                        |
                                      Go HTTP API + shared SQLite revisions
                                                        |
                                                   React/Vite
                                                        |
                                                      Leaflet
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
| `events.geojson` | Assertion transport view; geometry is always `null` in the current exporter |
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

The current UI implements multi-select geographic source checkboxes. With one
source enabled it behaves as a single-source view; with several enabled it
preserves every matching assertion. Symbology currently communicates geometry
review state, not evidence source.

Two richer modes remain planned:

1. **source comparison symbology**: visually distinguish two or more enabled
   evidence sources;
2. **priority profile**: select the highest-priority available source for each
   target while retaining an “alternatives available” indicator.

A future profile should separately configure:

- enabled geo assertion source types and their ordering;
- scope precedence (`direct`, `parent_fallback`, `parent_alternative`);
- geometry interpretation sources;
- temporal source types;
- the runtime `circa_years` policy;
- whether uncertain/unreviewed assertions are shown.

Multiple places from the same winning source must remain multiple assertions. A
future priority profile must not arbitrarily select only the first.

## Map abstraction

The React atlas currently depends directly on React Leaflet, but Leaflet-specific
behavior is isolated in `apps/web/src/features/atlas/AtlasMap.tsx`. Filters and
drill-down consume the backend contract rather than Leaflet objects.
Location-editing map behavior is separately isolated in
`GeometryMapEditor.tsx`.

A formal `MapAdapter` does not exist yet. Replacing Leaflet would require
rewriting those two map components, while the API, filters, and drill-down
contracts could remain unchanged.

## Filters

The implemented atlas filters are:

- time interval;
- configurable `circa` expansion and inclusion of undated records;
- one or more geographic source types;
- one or more parent/child origins;
- one Hibur at a time;
- one geometry review state at a time.

The backend accepts comma-separated Hibur and location-status values even
though the current selects expose one value. Temporal source, Hibur family,
geometry interpretation source, place/country, script, language, digitized
status, confidence, and uncertainty filters remain planned.

The summary currently displays mapped places, unique target records, and
unmapped assertions. Undated records are controlled by a checkbox but do not
have a separate summary count. Missing geometry remains in the record total and
opens through the unmapped-assertions drill-down.

## Drill-down

The current UI provides:

1. a place drill-down, including mapped geometries spatially contained by the
   selected geometry;
2. a rectangular area drill-down for intersecting mapped geometries;
3. an unmapped drill-down grouped by unresolved place concept;
4. assertion cards containing the target record, linked Hiburim, all target
   temporal assertions, raw place evidence, source description,
   parent/direct origin, descriptive metadata, MARC provenance, NLI link, and
   digital-object link.

Counts currently group unique target record IDs separately from assertions.
`physical_id` is retained in the data for a future physical-manuscript counting
mode; there is no counting-mode toggle yet.

## Joined atlas endpoint

`GET /api/v1/atlas-view` joins:

- geographic assertions and source descriptions;
- effective temporal intervals produced by the runtime circa policy;
- manuscript/component records and shelfmarks;
- many-to-many Hibur links;
- current reviewed, human-edited, or AI-draft modern geometry.

The response groups mapped events by place, avoiding repeated country or region
polygons. Each feature retains nested event records for drill-down. It also
returns `unmapped_groups`, grouped by unresolved place concept with the same
record and assertion detail but no fabricated geometry. Supported query
parameters are `start-year`, `end-year`, `circa-years`,
`include-undated`, `geo-source`, `origin`, `hibur-id`, and
`location-status`. Comma-separated values implement multi-selection.

The UI defaults to `nli_751_writing_place`. Current repositories and related
places remain available but are not silently mixed into the historical default.
Mapped and unmapped assertion counts are returned together. The unmapped count
opens a dedicated drill-down in the UI and respects the same filters as the
map.
