# Midrash Atlas agent guide

## Repository map

- `apps/api`: Go CLI and HTTP API.
- `apps/web`: React/Vite UI.
- `packages/atlas`: evidence-preserving normalization and export.
- `packages/canonical`: the layer-explicit contract consumed by the future atlas.
- `packages/preprocess`: validation and preparation of heterogeneous source data.
- `packages/gazetteer`: policy-compliant cached gazetteer acquisition.
- `packages/llm`: OpenAI/Ollama HTTP clients.
- `packages/provenance`: shared AI provenance schema.
- `configs/atlas`: curated source-type and geometry seed configuration.
- `configs/preprocess`: auditable source-reconciliation decisions.
- `data/source`: project-owned CSV/RDF inputs.
- `data/raw`: cached upstream responses.
- `data/derived`: normalized upstream data.
- `data/generated/atlas`: generated frontend/API documents.
- `data/prepared`: validated, application-ready canonical data and validation reports.
- `data/sources_new/hiburim_from_manuscript_references.csv`: explicit temporary
  work stubs created from otherwise unresolved manuscript contents labels.
- `data/sources_new/hibur_generated_stub_ids.csv`: clearly temporary IDs for
  ontology rows required by other sources but missing `ID חדש?`.
- `data/runtime/atlas.sqlite`: shared PoC curation database; WAL/SHM sidecars
  remain ignored.
- `tools/nli`: NLI retrieval scripts.
- `tools/preprocess`: standalone preprocessing CLI; MARC is only one possible input.

## Invariants

- The domain has three explicit layers: `work` (Hibbur/composition), `item`
  (manuscript or printed edition), and `text` (passages and their mentions).
- Every domain entity, event, assertion, map observation, filter, and drill-down
  result must identify its layer. Shared places, agents, times, and evidence may
  connect layers but must not collapse them.
- A place mentioned by a text is a textual mention, not automatically a claim
  that the work or an item was historically present there.
- Items relate many-to-many to works. Manuscript parts and event subjects must
  remain representable rather than flattening a composite codex into one work.
- The atlas application consumes validated canonical data. MARC extraction,
  source reconciliation, ID repair, narrative structuring, and data cleaning
  belong to the separate preprocessing flow.
- Preserve raw evidence; do not replace competing assertions with one truth.
- Distinguish `direct`, `parent_fallback`, and `parent_alternative`.
- Store approximation semantics; apply `circa` windows only at runtime.
- An LLM curation result is a draft and must never write geometry directly.
- Every AI-produced JSON value must include provider, model, UTC generation
  time, purpose, and prompt hash.
- AI curation reruns update the current draft but append an immutable run audit;
  never discard earlier provider/model results.
- Geometry corrections append revisions; do not overwrite audit history.
- The location editor currently exposes one fixed geometry variant,
  `modern_place`; keep general variant support internal for later historical or
  interpretive alternatives.
- External gazetteers are ingestion/curation dependencies, never map-render
  runtime dependencies.
- Current repository geography must remain separate from historical geography.
- Manuscript-derived `stub:mss:*` works are temporary and must remain visibly
  marked until a scholar supplies an ontology identity or category decision.
- Source edition rows marked `כפולה` represent retained reprint assertions;
  group them with `reprint_of` rather than collapsing them.
- The project hibur ontology is authoritative for work identity. Preserve an
  interpreter's broader grouping as a sourced multi-work assertion; never merge
  ontology works merely because an external source gives them one name or ID.

## Commands

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go test ./...
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go vet ./...
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go run ./apps/api/cmd/atlas export
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go run ./apps/api/cmd/atlas gazetteer-pilot
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go run ./apps/api/cmd/atlas gazetteer-pilot --all-places
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go run ./apps/api/cmd/atlas curation-pilot
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go run ./apps/api/cmd/atlas curation-pilot --only-ai-not-checked --skip-human-reviewed
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go run ./tools/preprocess
npm run web:typecheck
npm run web:build
```

The checked-in derived gazetteer document currently covers all generated
places. Running `gazetteer-pilot` without `--all-places` intentionally rebuilds
the original ten-place sample at the same output path.

The asdf Go 1.21 installation on the current machine is incomplete. Use the
Homebrew Go binary; `go.mod` may automatically obtain the declared Go toolchain.
