# Location curation and audit

## Two separate actions

LLM assistance and scholarly approval are deliberately separated:

1. `POST /api/v1/curation/geometry-draft` creates a non-authoritative draft.
2. A curator reviews or changes the geometry in the React editor.
3. `POST /api/v1/locations/{place_id}/geometries` saves an audited revision.

The LLM endpoint never writes to the geometry store.

## Draft prompt contract

The curation request supplies:

- place ID, label, and aliases;
- relevant geo assertions and temporal context;
- existing geometry variants;
- zero or more externally retrieved gazetteer candidates;
- optional project-specific instructions;
- optional provider and model overrides.

The prompt permits the model to return only one of the supplied candidate IDs.
When no safe match exists, it must return `needs_candidates` or `ambiguous`.
The server validates the candidate ID after parsing the model output.

The response includes the exact prompt plus an `ai_provenance` object containing
provider, model, UTC generation time, purpose, and SHA-256 prompt hash. If a
curator later applies the draft, that whole object is stored with the geometry
revision.

Supported providers:

- `openai`, using `OPENAI_API_KEY` and the Responses API;
- `ollama`, using `OLLAMA_BASE_URL` and `/api/generate`.

Provider/model defaults are controlled by `CURATION_LLM_PROVIDER` and
`CURATION_LLM_MODEL`. They default to `ollama` and `qwen3.5:35b`. Secrets are
never returned by the API.

The location overview distinguishes five AI states: `not_attempted`,
`candidate_proposed`, `needs_candidates`, `ambiguous`, and
`technical_failure`. A place without coordinates therefore does not conceal
whether AI curation has never run or ran without producing a safe match.
The audit panel also lists retained AI runs with provider, model, timestamp,
result status, comment, technical error, and which result is current.

## Geometry revision model

Every save includes:

- place ID and the fixed PoC variant ID `modern_place`;
- complete GeoJSON `Point`, `Polygon`, or `MultiPolygon`;
- validity years and spatial precision;
- interpretation source and note;
- confidence and review status;
- a human action of `draft`, `reviewed`, or `changed`;
- required human change reason;
- optional AI provenance object;
- optional actor ID.

The server adds:

- immutable revision ID;
- monotonically increasing revision number per place/variant;
- `supersedes_revision_id`;
- source `ui`;
- UTC `created_at`.

There is no update or delete endpoint. Corrections create another revision.

The React editor intentionally hides variant ID and geometry label. During this
PoC it edits only `modern_place`. The database model keeps variant support
because later work may need modern reference geometry, time-specific historical
polygons, disputed scholarly interpretations, or source-specific alternatives,
but those choices should not burden the current curation task.

`review_status: reviewed_by_human` means a person explicitly inspected the
geometry and marked it suitable for atlas display. `human_action: changed`
means the coordinates differ from the geometry loaded into the editor. A
geometry may therefore be human-changed but still unreviewed, or both changed
and reviewed.

## Location coverage view

`GET /api/v1/location-overview` merges the best available geometry for every
place concept in this order:

1. latest human revision;
2. configured geometry variant;
3. selected but unreviewed gazetteer/LLM pilot candidate;
4. no geometry.

Each item states whether its coordinates are structurally valid, whether a
human reviewed them, whether a human changed them, and which source supplied
the displayed geometry. The React dropdown uses the same status values and is
alphabetized by preferred place label.

## Gazetteer execution boundary

Gazetteer lookup is an ingestion/curation operation, not a normal atlas-runtime
dependency. Candidate responses are cached, reviewed, and converted to
persisted geometry revisions. Map rendering reads approved stored geometry.

A future curator-only “refresh candidates” action may run a gazetteer lookup,
but ordinary filters, map loads, and drill-down requests must never contact the
gazetteer service.

## API

```text
GET  /api/v1/locations/{place_id}/geometries
GET  /api/v1/locations/{place_id}/geometries?history=true
GET  /api/v1/locations/{place_id}/geometries?history=true&variant-id={id}
GET  /api/v1/location-overview
POST /api/v1/locations/{place_id}/geometries
POST /api/v1/curation/geometry-draft
```

The local implementation uses `data/runtime/atlas.sqlite`. The
database is committed for a shared PoC audit history; WAL and SHM sidecars are
ignored and checkpointed on clean API shutdown. The `locationstore.Store`
interface remains the portability boundary for a later cloud database.

## Authorization extension

`actor_id` is nullable today, and the HTTP handler currently clears any
caller-supplied value. Once authentication exists, the API must derive it from
the verified identity rather than accept it from an untrusted request body.
