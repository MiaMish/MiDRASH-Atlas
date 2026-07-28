# Gazetteer and geometry-curation pilot

## Purpose and execution boundary

This pilot tests whether place assertions extracted from manuscript metadata can
be turned into reviewable modern display-geometry candidates. It does not create
historical boundaries and does not choose authoritative geometry.

Nominatim is used only during ingestion or curation. The atlas UI and its
runtime API must read persisted, reviewed geometry and must never geocode places
as part of map rendering, filtering, or drill-down.

## Stage 1: acquire and cache candidates

The selected sample is in `configs/atlas/gazetteer_pilot.json`. It covers ten
different matching situations: countries, cities, broad regions, transliterated
names, ambiguous localities, and an unresolved Hebrew/Arabic-script label.

Run:

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas gazetteer-pilot
```

The client:

- sends requests sequentially, with at least 1.1 seconds between uncached calls;
- uses an identifying `User-Agent`;
- asks for GeoJSON geometry rather than centroids alone;
- caches the exact response and acquisition metadata under
  `data/raw/gazetteer/nominatim/`;
- stores query parameters, retrieval time, attribution, licence, authority IDs,
  bounding boxes, and candidate geometry;
- reuses the cache for identical queries.

The derived review input is
`data/derived/gazetteer/nominatim-pilot.json`. It combines each candidate set
with aliases, relevant geo assertions, temporal context, and existing geometry
variants. The artifact explicitly records `runtime_use: false`.

## Stage 2: generate review-only Ollama drafts

Run all selected places:

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas curation-pilot
```

Run a subset:

```bash
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas curation-pilot \
  --place-ids place_61abf0b2aa13,place_bd72015066a5
```

The command defaults to `ollama` and `qwen3.5:35b`, loads `OLLAMA_BASE_URL`
from `.env`, disables model thinking for strict JSON output, and checkpoints the
output after every place. Provider and model can be overridden with flags or
the `CURATION_LLM_PROVIDER` and `CURATION_LLM_MODEL` environment variables.

Results are written to
`data/derived/gazetteer/nominatim-pilot-drafts.json`. Every successful model
result includes:

- provider and model;
- UTC generation timestamp;
- purpose;
- SHA-256 hash of the exact prompt;
- the exact prompt itself.

The model can select only a candidate ID supplied in the cached input. The Go
validator rejects invented IDs, unsupported statuses, malformed output, and a
selection paired with an ambiguous/no-candidate status. This command never
writes to `configs/atlas/place_geometries.json` or the SQLite revision store.

## Results from 2026-07-28

| Place | Candidates | Ollama draft | Interpretation |
|---|---:|---|---|
| Spain | 1 | selected | modern-country `MultiPolygon` fallback |
| Yemen | 1 | selected | modern-country `MultiPolygon` fallback |
| North Africa | 2 | needs candidates | returned results were unrelated places in Barcelona and Ohio |
| Cairo | 2 | selected | locality candidate; alternatives still require human review |
| Istanbul | 2 | selected | regional geometry; precision choice requires review |
| Bukhara | 2 | selected | locality rather than surrounding administrative region |
| Al-Tawilah | 3 | ambiguous | several same-name localities; model refused to guess |
| Feodosia | 1 | selected | locality polygon |
| Heraklion | 1 | selected | locality point |
| מגדלא אלצפקיין | 0 | needs candidates | unresolved; requires research or another gazetteer |

All ten drafts used `ollama` / `qwen3.5:35b` and have complete AI provenance.
Seven selected a candidate and three safely declined. These are prompt-quality
results, not accepted curation decisions.

## Lessons and next review

- A modern gazetteer works reasonably for countries and well-known cities.
- Text matching alone is unsafe for broad regions and repeated locality names.
- Polygon availability is uneven; a point may be the only returned geometry.
- The manuscript date and catalog context help explain a match, but do not turn
  modern geometry into a historical boundary.
- Human review needs to show the candidate label, geometry, authority link,
  source assertion, temporal context, model rationale, and ambiguity warnings
  together.

The next team-facing action is to review these ten cases and mark each proposed
candidate as accept, reject, or research-required. Only an explicit acceptance
should create an append-only geometry revision.
