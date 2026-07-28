# Midrash Atlas PoC

This repository retrieves NLI MARC records and converts them into an
evidence-preserving atlas dataset. The PoC does not select one authoritative
location or date for a manuscript. It exports competing geographic and temporal
assertions so researchers can compare source strategies in the UI.

## Repository layout

```text
apps/
  api/                  Go HTTP API and CLI
  web/                  React/Vite UI
packages/
  atlas/                MARC normalization and atlas export
  gazetteer/            Cached candidate acquisition
  llm/                  OpenAI/Ollama provider clients
  provenance/           Shared AI provenance model
configs/atlas/          Curated source and geometry configuration
data/
  source/               Project CSV and RDF inputs
  raw/                  Cached NLI MARCXML
  derived/              Normalized NLI data
  generated/atlas/      UI/API export
  runtime/              Shared SQLite curation database
tools/nli/              NLI retrieval tooling
```

## Generate the atlas data

The machine's asdf Go 1.21 installation is currently incomplete. The Homebrew
Go binary works:

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas export
```

Outputs are written to `data/generated/atlas/`.

## Run the small API

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas serve
```

The shared audit store is `data/runtime/atlas.sqlite`. It is committed so the
PoC team sees the same human review history. SQLite WAL/SHM sidecar files remain
ignored; the API checkpoints the WAL when it closes.

## Run the React UI

```bash
npm install
npm run web:dev
```

Vite proxies `/api` to the Go service on port 8080. The initial UI contains a
location-curation workspace with a draggable point editor, GeoJSON
polygon/multipolygon preview, required change reason, and revision history.

## LLM-assisted curation drafts

The API loads `.env` without overriding existing process variables:

```text
OPENAI_API_KEY=...
OLLAMA_BASE_URL=...
CURATION_LLM_PROVIDER=ollama
CURATION_LLM_MODEL=qwen3.5:35b
```

`CURATION_LLM_PROVIDER` and `CURATION_LLM_MODEL` are optional. They default to
Ollama with `qwen3.5:35b`; OpenAI remains available as an override. The draft
endpoint is `POST /api/v1/curation/geometry-draft`. The model may select only from
gazetteer candidates supplied by the caller; it cannot create an authoritative
geometry or save a revision.

Every AI-produced JSON result includes provider, model, UTC generation time,
purpose, and prompt hash under `ai_provenance`. Gazetteer lookup is used only to
populate or refresh curation candidates. Normal atlas rendering reads persisted,
reviewed geometry and never performs runtime geocoding.

Run the cached ten-place gazetteer and Ollama draft pilots:

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas gazetteer-pilot

GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas curation-pilot
```

The second command creates review drafts only; it never accepts or saves a
geometry. See the pilot report for the candidate and refusal results.

Static resources are under `/api/v1/`. A policy-derived temporal view is:

```text
/api/v1/temporal-view?circa-years=10
  &source-type=nli_260_date
  &origin=direct
```

The stored record keeps `1460` plus `approximate: true`; only this view expands
it to `1450–1470`. This makes the policy changeable without rewriting data.

## Documentation

- [NLI retrieval](docs/nli-retrieval.md)
- [Atlas data model](docs/atlas-data-model.md)
- [PoC architecture and UI contract](docs/poc-architecture.md)
- [Location curation and audit](docs/location-curation.md)
- [Gazetteer and curation pilot](docs/gazetteer-curation-pilot.md)
- [Decision log](docs/decisions.md)
