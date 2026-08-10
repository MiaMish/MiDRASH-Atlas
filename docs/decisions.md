# Decision log

## 2026-08-07 — Make Work, Item, and Text explicit atlas layers

The replacement atlas model has three first-class layers: `work` for the
Hibbur/composition, `item` for manuscripts and printed editions and their event
biographies, and `text` for passages and named-entity mentions. All events,
assertions, map observations, filters, and drill-down results identify their
layer. Shared places, agents, dates, and evidence connect the layers without
collapsing them. In particular, a textual place mention is not historical
evidence that a work or item was present there.

## 2026-08-07 — Put heterogeneous cleanup before the atlas boundary

The atlas consumes validated canonical data. CSV cleanup, identifier repair,
cross-source reconciliation, narrative extraction, and MARC transformation
belong to a standalone preprocessing flow. MARC is one input among many and
does not define the future application contract. Raw evidence and competing
claims remain preserved throughout preprocessing.

## 2026-08-07 — Prove the replacement with one complete vertical slice

The first replacement slice centers on Midrash Proverbs and Vatican Ebr. 44,
covering work formation, manuscript biography, printed editions, and textual
NER. The old implementation remains only until this slice works end to end; it
will then be removed instead of supported as a second model.

## 2026-08-07 — Treat the two Hanukkah titles as one work

`מעשה חנוכה` (Story of Hanukkah) and `אגדת חנוכה` (Aggadat Hanukkah) are
alternate names for the same work. Preserve both ontology source rows and
titles, but resolve both to canonical work ID `150:T`.

## 2026-08-07 — Create explicit temporary works for unresolved manuscript labels

When a manuscript contents label does not resolve to the supplied ontology,
preprocessing creates a clearly marked `stub:mss:*` hibur rather than dropping
the relationship or silently mapping it. Each stub cites all manuscript source
rows and available system IDs. Questions needed to replace the stubs with
scholarly decisions are recorded in `docs/Qs to Eliezer.md`.

## 2026-08-07 — Interpret source-marked duplicate editions as reprints

Printed-edition rows marked `כפולה` remain separate source assertions and
separate item records. Preprocessing will group them with an explicit
`reprint_of` relationship; it must not collapse or discard them.

## 2026-08-07 — Keep project ontology identity authoritative over merged interpretations

When an interpreter groups works that the hibur ontology defines separately,
the works remain separate canonical entities. Preserve the interpreter's
grouping as a sourced multi-work assertion. In particular, Reizel's label
`ספרי אסופות של מדרשים` and ID `54` relate to both `אוצר המדרשים`
(`3:18:1.0`) and `בתי מדרשות` (`3:19:1.0`) without merging those works.

## 2026-08-07 — Use visibly temporary IDs for incomplete ontology rows

Ontology rows needed by other sources but lacking `ID חדש?` receive curated
IDs beginning `stub:ontology:`. These IDs are stored outside the raw ontology
CSV, include exact usage references, and must be replaced when permanent IDs
are supplied. Unused incomplete rows may remain unchanged and excluded.

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
shown with explicit review status; a curator action is required before they
become geometry revisions. The public atlas reads local configured/cached state
and SQLite revisions and never performs per-record runtime geocoding.

## 2026-07-28 — AI provenance is mandatory

Every AI-produced JSON result records provider, model, UTC generation time,
purpose, and prompt hash. If an AI draft informs a saved geometry revision, the
same provenance object travels with that revision. Deterministic exports state
that no AI was used.

## 2026-07-28 — Candidate acquisition and AI review are separate stages

Raw Nominatim results and acquisition provenance are cached before any LLM call.
Ollama receives only that candidate set plus project evidence and produces a
separate, validated review draft. Neither stage updates accepted geometry. Only
an explicit save in the human curation workflow may append a geometry revision,
whether as an unreviewed human draft or as a reviewed result.

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

## 2026-07-28 — Group the atlas by place and drill into assertions

The map transfers one feature per place, with matching manuscript/assertion
events nested for drill-down. This avoids repeating large polygons and keeps
multiple source statements visible. The default map source is the explicit NLI
writing place; related places and current repositories are opt-in. Unmapped
matching assertions remain in the summary count and open in their own
drill-down.

## 2026-07-28 — Keep no-geometry records explorable

Filtered assertions without valid geometry are returned as `unmapped_groups`,
grouped by place concept with the same manuscript and evidence payload as mapped
events. The UI opens them from the unmapped summary count and reports whether AI
has not run, declined a match, selected an unavailable candidate, or failed
technically. The system does not fabricate coordinates merely to put these
records on the map.

## 2026-07-28 — Separate readable metadata from MARC provenance

The drill-down uses the target record's `245$a` title and descriptive values.
MARC tags and source record IDs remain available in a collapsed catalog-field
provenance section rather than being appended to every visible value. When
geographic evidence comes from a parent, the parent record is identified
separately so its title does not replace the analytic child's title.

## 2026-07-28 — Treat spatial selection as display-geometry analysis

Selecting a broad geometry includes mapped place geometries spatially contained
by it, and box selection includes geometries intersecting the drawn rectangle.
Smaller contained polygons render above their containers for direct clicking.
These operations use the current modern display geometries and do not claim
that the same containment or boundaries applied historically.

## Open decisions for the team

- Which assertion-source priority profiles should ship as presets?
- Does parent data represent fallback, contextual evidence, or both for each
  field type?
- Should the PoC's current ±10-year default `circa` window remain the project
  default?
- Which historical gazetteer or scholarly region datasets can be licensed and
  cited?
- How should fuzzy or transitional regions be rendered and queried?
- Who may mark extracted colophon/provenance assertions as reviewed?
- Which production database adapter should replace local SQLite?
