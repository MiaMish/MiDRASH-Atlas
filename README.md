# Midrash Atlas PoC

This repository retrieves NLI MARC records and converts them into an
evidence-preserving atlas dataset. The PoC does not select one authoritative
location or date for a manuscript. It exports competing geographic and temporal
assertions so researchers can compare source strategies in the UI.

![Annotated atlas workspace](docs/images/atlas-overview-annotated.jpg)

## What the PoC includes

- An interactive Leaflet atlas with a configurable temporal window.
- Independent geographic evidence layers rather than one silently merged
  location.
- Filters for Hibur, parent/child evidence origin, geometry review state, and
  undated records.
- Place, containment, bounding-box, and unmapped-record drill-downs.
- Rich manuscript metadata with MARC provenance available on demand.
- A geometry-curation workspace with human review state and append-only audit
  history.
- Cached gazetteer acquisition and Ollama-first LLM curation drafts.

See the [annotated feature tour](docs/feature-tour.md) for screenshots of the
main workflows.

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
  raw/                  Cached NLI MARCXML and gazetteer responses
  derived/              Normalized NLI data and curation-pilot artifacts
  generated/atlas/      UI/API export
  runtime/              Shared SQLite curation database
tools/nli/              NLI retrieval tooling
```

## Quick start

The API and UI run as separate local processes.

Terminal 1:

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas serve
```

Terminal 2:

```bash
npm install
npm run web:dev
```

Open [http://localhost:5173](http://localhost:5173). Vite proxies `/api` to the
Go service at `http://127.0.0.1:8080`.

The repository contains the shared PoC SQLite database at
`data/runtime/atlas.sqlite`. Do not delete it when cleaning generated files.

## Generate the atlas data

The machine's asdf Go 1.21 installation is currently incomplete. The Homebrew
Go binary works:

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas export
```

Outputs are written to `data/generated/atlas/`.

## Run the API

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas serve
```

The shared audit store is committed so the PoC team sees the same human review
history. SQLite WAL/SHM sidecar files remain ignored; the API checkpoints the
WAL when it closes.

## Run the React UI

```bash
npm install
npm run web:dev
```

Vite proxies `/api` to the Go service on port 8080. The default workspace is an
interactive manuscript atlas with a time range, evidence-source, parent/child,
Hibur, review-state, and undated filters. Selecting a mapped place opens its
manuscript/Hibur/date/evidence drill-down. A second workspace contains the
audited location editor.

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
populate or refresh curation candidates. Normal atlas rendering reads local
configured geometry, cached AI/gazetteer drafts, and audited SQLite revisions;
it never performs runtime geocoding. Unreviewed candidates remain visibly
labelled as such.

Refresh cached candidates for every generated place concept, then run Ollama
only for places that have never been AI-checked:

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas gazetteer-pilot --all-places

GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas curation-pilot \
  --only-ai-not-checked \
  --skip-human-reviewed
```

The curation command creates review drafts only; it never accepts or saves a
geometry. Running `gazetteer-pilot` without `--all-places` rebuilds the original
ten-place sample and replaces the same derived candidate file, so use the
all-place form for the current checked-in workflow. See the pilot report for
the original sample and current all-place results.

Rerun with another provider/model while protecting human-reviewed places:

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas curation-pilot \
  --provider ollama \
  --model <another-installed-model> \
  --skip-human-reviewed
```

Each rerun replaces the current AI draft for the places it processes and
appends the full prior/current results to the `runs` audit history. It never
overwrites a human geometry revision.

Long-running Go commands emit timestamped progress to stdout. Gazetteer and
curation runs report every place, cache/skip state, result status, and duration.
The NLI Python retriever reports batch and relationship progress to stderr so
JSON written to stdout remains valid.

Static resources are under `/api/v1/`. A policy-derived temporal view is:

```text
/api/v1/temporal-view?circa-years=10
  &source-type=nli_260_date
  &origin=direct
```

The stored record keeps `1460` plus `approximate: true`; only this view expands
it to `1450–1470`. This makes the policy changeable without rewriting data.

The joined atlas endpoint is:

```text
/api/v1/atlas-view
  ?geo-source=nli_751_writing_place
  &start-year=1200
  &end-year=1600
  &include-undated=false
```

It groups matching assertions by place so polygons are transferred once, while
retaining nested manuscript, Hibur, temporal, raw-evidence, and origin details
for drill-down. Assertions without valid geometry are returned in
`unmapped_groups` rather than being discarded or assigned artificial
coordinates.

## Verification

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go test ./...

npm run web:typecheck
npm run web:build
```

## Documentation

- [Annotated feature tour](docs/feature-tour.md)
- [NLI retrieval](docs/nli-retrieval.md)
- [Atlas data model](docs/atlas-data-model.md)
- [PoC architecture and UI contract](docs/poc-architecture.md)
- [Location curation and audit](docs/location-curation.md)
- [Gazetteer and curation pilot](docs/gazetteer-curation-pilot.md)
- [Decision log](docs/decisions.md)
