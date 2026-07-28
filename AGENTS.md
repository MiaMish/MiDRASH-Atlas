# Midrash Atlas agent guide

## Repository map

- `apps/api`: Go CLI and HTTP API.
- `apps/web`: React/Vite UI.
- `packages/atlas`: evidence-preserving normalization and export.
- `packages/gazetteer`: policy-compliant cached gazetteer acquisition.
- `packages/llm`: OpenAI/Ollama HTTP clients.
- `packages/provenance`: shared AI provenance schema.
- `configs/atlas`: curated source-type and geometry seed configuration.
- `data/source`: project-owned CSV/RDF inputs.
- `data/raw`: cached upstream responses.
- `data/derived`: normalized upstream data.
- `data/generated/atlas`: generated frontend/API documents.
- `data/runtime/atlas.sqlite`: shared PoC curation database; WAL/SHM sidecars
  remain ignored.
- `tools/nli`: NLI retrieval scripts.

## Invariants

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

## Commands

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go test ./...
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go vet ./...
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go run ./apps/api/cmd/atlas export
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go run ./apps/api/cmd/atlas gazetteer-pilot
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go run ./apps/api/cmd/atlas gazetteer-pilot --all-places
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go run ./apps/api/cmd/atlas curation-pilot
GOCACHE=/private/tmp/midrash-atlas-go-cache /opt/homebrew/bin/go run ./apps/api/cmd/atlas curation-pilot --only-ai-not-checked --skip-human-reviewed
npm run web:typecheck
npm run web:build
```

The checked-in derived gazetteer document currently covers all generated
places. Running `gazetteer-pilot` without `--all-places` intentionally rebuilds
the original ten-place sample at the same output path.

The asdf Go 1.21 installation on the current machine is incomplete. Use the
Homebrew Go binary; `go.mod` may automatically obtain the declared Go toolchain.
