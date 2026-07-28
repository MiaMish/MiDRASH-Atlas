# Decision log

## 2026-07-28 — Preserve competing assertions

The PoC will export multiple geo and temporal assertions rather than calculate
one canonical manuscript location/date. This lets the team compare explicit
writing place, related place, colophon/provenance extraction, external
catalogues, and researcher interpretations.

## 2026-07-28 — Make parent inheritance explicit

Child and parent statements remain separate. Each assertion identifies both its
target and source record and is marked `direct`, `parent_fallback`, or
`parent_alternative`.

## 2026-07-28 — Keep approximation policy out of stored semantics

`1460 circa` is stored as year `1460` with `approximate: true`. The application
may interpret this as ±10 years or another configurable interval. The applied
policy is returned alongside derived API views.

## 2026-07-28 — Model contextual geometry variants

Places can have several points or polygons with validity intervals and named
interpretive sources. Modern gazetteer geometry is a fallback, not a claim about
historical boundaries. Scholarly disagreement is representable rather than
flattened.

## 2026-07-28 — Small Go backend, React frontend

The backend exposes stateless HTTP handlers and repository interfaces suitable
for later wrapping in serverless infrastructure. React owns interactive filters
and drill-down. Leaflet is the initial renderer behind an isolated component.

## 2026-07-28 — LLM output is a draft, never a write

The geometry-curation prompt may recommend only an explicitly supplied
gazetteer candidate. It must not invent coordinates or authority IDs. The API
validates the returned candidate ID. Applying the draft is a separate human UI
action that creates an audited geometry revision.

## 2026-07-28 — Ollama is the default curation provider

Geometry-curation drafts default to the configured Ollama server using
`qwen3.5:35b`. This keeps routine draft generation on the project’s cheaper
infrastructure. OpenAI and other installed Ollama models remain explicit
configuration or request-level overrides.

## 2026-07-28 — Geometry edits are append-only revisions

The UI does not overwrite geometry. Each save records a new revision with a
server-generated UTC timestamp, change reason, source `ui`, and a link to the
revision it supersedes. `actor_id` is nullable until authentication is added.
Generated NLI data and runtime curation remain separate.

## 2026-07-28 — External gazetteers are not rendering dependencies

Gazetteer calls populate or refresh curation candidates. Results are cached and
reviewed before becoming geometry revisions. The public atlas reads persisted
geometry and never performs per-record runtime geocoding.

## 2026-07-28 — AI provenance is mandatory

Every AI-produced JSON result records provider, model, UTC generation time,
purpose, and prompt hash. If an AI draft informs a saved geometry revision, the
same provenance object travels with that revision. Deterministic exports state
that no AI was used.

## 2026-07-28 — Candidate acquisition and AI review are separate stages

Raw Nominatim results and acquisition provenance are cached before any LLM call.
Ollama receives only that candidate set plus project evidence and produces a
separate, validated review draft. Neither stage updates accepted geometry. A
human acceptance is the only action that may append a geometry revision.

## 2026-07-28 — Human review and human change are separate facts

The current geometry records both an explicit review status and the human
action that created its revision. Moving or replacing coordinates does not
automatically mark them reviewed. Reviewing an unchanged candidate creates an
audited revision without falsely claiming that the curator changed it.

## 2026-07-28 — Expose one modern-place variant in the PoC

The UI fixes the editable variant to `modern_place` and hides internal IDs and
labels. General variant support remains in the data model for later historical,
temporal, disputed, or source-specific geometries, but is not a curator concern
until those workflows exist.

## 2026-07-28 — Share the PoC SQLite audit database

`data/runtime/atlas.sqlite` is committed so team members share human review
history during the PoC. SQLite WAL and SHM sidecars remain ignored, and the API
checkpoints the WAL on close. This is intentionally temporary; concurrent
collaboration should later move to a service-backed database.

## 2026-07-28 — AI reruns replace current state but append history

The current AI draft for a place may be regenerated with another provider or
model. The `drafts` collection is replaced only for processed places, while
every run and result remains append-only under `runs`. A CLI flag skips
human-reviewed `modern_place` entries by consulting the shared SQLite database.
AI reruns never overwrite human geometry revisions.

## Open decisions for the team

- Which assertion-source priority profiles should ship as presets?
- Does parent data represent fallback, contextual evidence, or both for each
  field type?
- What default `circa` window should the UI use?
- Which historical gazetteer or scholarly region datasets can be licensed and
  cited?
- How should fuzzy or transitional regions be rendered and queried?
- Who may mark extracted colophon/provenance assertions as reviewed?
- Which production database adapter should replace local SQLite?
