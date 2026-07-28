# Atlas data model

## Principle: assertions, not canonical facts

A manuscript, component, or Hibur does not own one timeless `place` and `date`.
It is connected to evidence-bearing assertions. Parallel assertions can
disagree without overwriting one another.

The core entities are:

- **physical manuscript**: the codex or physical holding unit;
- **component**: an analytic child record representing content within a physical
  manuscript;
- **Hibur**: a conceptual work or literary unit;
- **geo assertion**: a source-backed relationship between a target record and a
  place concept;
- **temporal assertion**: a source-backed date expression;
- **place concept**: an identity such as Yemen, Cairo, or a historical locality;
- **geometry variant**: one time-bounded spatial interpretation of a place.

## Geographic assertions

Every geographic assertion carries:

- `source_type_id`, whose UI description is in `source_types.json`;
- `target_record_id`, the record being mapped;
- `source_record_id`, the MARC record containing the evidence;
- `physical_id`, used for deduplicated physical-manuscript counts;
- `origin`: `direct`, `parent_fallback`, or `parent_alternative`;
- `place_id` and the unmodified `place_raw`;
- the MARC field/role, extraction method, review status, and catalogue update
  timestamp.

`parent_fallback` means the child lacks that source type and the assertion comes
from its physical parent. `parent_alternative` means both scopes contain that
kind of evidence; neither is silently discarded.

## Temporal assertions

The stored value expresses what the source says:

```json
{
  "kind": "year",
  "year": 1460,
  "approximate": true,
  "uncertain": false
}
```

It does not store a fabricated `1450–1470` interval. A runtime policy can expand
an approximate year for filtering or visualization. Century ranges are stored
because their bounds are intrinsic to the catalogued expression:

```json
{
  "kind": "century",
  "century": 14,
  "start_year": 1301,
  "end_year": 1400,
  "approximate": false
}
```

The original date string always remains in `evidence.raw`.

## Contextual places and geometries

A place concept is not assigned one permanent centroid. It has zero or more
geometry variants:

```json
{
  "id": "geometry_...",
  "label": "Yemen according to source X, 1200–1300",
  "geometry": {"type": "Polygon", "coordinates": []},
  "valid_from_year": 1200,
  "valid_to_year": 1300,
  "spatial_precision": "interpreted_region",
  "interpretation_source": "historical_gazetteer_geometry",
  "interpretation_note": "Scope and boundary rationale...",
  "confidence": "medium",
  "review_status": "reviewed"
}
```

Variants may be points, polygons, multipolygons, or—later—fuzzy regions. Two
scholarly interpretations may overlap or conflict. Selection therefore depends
on:

1. the geo assertion;
2. its effective temporal interval;
3. a selected geometry-source profile;
4. optional scholarly/context filters.

When no historical geometry is available, a modern gazetteer point or polygon
may be offered as a visibly labelled display fallback. It must never masquerade
as a historical boundary.

Curated seed variants are maintained in `configs/atlas/place_geometries.json`, keyed by
place label rather than generated ID. The exporter merges them into
`data/generated/atlas/places.json`. A seed may look like:

```json
{
  "place_label": "Yemen (Republic)",
  "aliases": ["Yemen"],
  "geometry_variants": [{
    "id": "geometry_yemen_modern_osm",
    "label": "Modern Yemen administrative boundary",
    "geometry": {"type": "MultiPolygon", "coordinates": []},
    "spatial_precision": "modern_country_boundary",
    "interpretation_source": "modern_gazetteer_geometry",
    "interpretation_note": "Display fallback; not a historical boundary.",
    "confidence": "high",
    "review_status": "reviewed"
  }]
}
```

## Catalogue context

Catalogue metadata is neither automatically primary nor secondary evidence.
For example, a colophon transcription can mediate primary text, while a place
assignment may be a cataloguer's interpretation. Assertions therefore record
`evidence_nature`, catalogue identity, record timestamp, raw statement,
extraction method, and review status. Historical catalogues should be identified
as sources in their own right rather than merged into the NLI statement.

Any assertion created or materially interpreted by AI includes an
`ai_provenance` array in its evidence:

```json
{
  "provider": "ollama",
  "model": "qwen3.5:35b",
  "generated_at": "2026-07-28T12:00:00Z",
  "purpose": "provenance_place_candidate_extraction",
  "prompt_hash": "sha256..."
}
```

Deterministic parsing does not fabricate an AI entry. The export manifest states
`ai_used: false` while the current atlas JSON is produced entirely by Go rules.

## Hibur relationships

The project spreadsheet creates many-to-many `hibur_links`. Exact alias matches
are linked to ontology IDs. Unmatched labels remain present with
`match_status: unresolved` and enter the review queue. A physical-manuscript
count should group by `physical_id`; a manifestation/component count may use
record IDs.

## Atlas projection

The runtime atlas view is a projection rather than another canonical dataset.
It filters assertion events, then groups them into one GeoJSON feature per place
to avoid repeating large polygons. Each feature contains:

- the current modern display geometry and its human/AI review state;
- all matching geo assertions for that place;
- the target manuscript or analytic record;
- linked Hiburim;
- stored temporal assertions plus policy-derived effective intervals.

Assertions whose place has no usable geometry are returned separately in
`unmapped_groups`. They retain the same event payload and are grouped by place
concept, so the UI can expose them without assigning misleading coordinates.

Filtering an approximate `1460` with a ±10 policy uses `1450–1470` in the view,
while the nested assertion still contains the original `1460` and
`approximate: true`.

### Drill-down field provenance

The atlas does not generate descriptive manuscript metadata with AI. The
drill-down reads the target NLI record in `records.json`, produced from the
lossless normalized MARC:

| UI field | MARC source |
| --- | --- |
| Primary title | `245$a` on the target record |
| Alternative title | `740$a` |
| Language | `041$a` |
| Extent and dimensions | `300$a` and `300$c` |
| Associated people | `100$a/$e` and `700$a/$e` |
| General, physical, provenance, contents, and colophon notes | `500$a`, `340$a`, `561$a`, `505$a`, and `957$a` |
| Script style | NLI local field `958$a` |
| Place evidence | the geo assertion's recorded MARC tag/subfield, normally `751$a` |
| Date evidence | each temporal assertion's MARC tag/subfield and parent/child origin |

For an analytic child, the displayed title remains the child's `245$a`. If a
geographic assertion originated on its physical parent, the event separately
identifies that source record and title. This preserves the distinction between
“what this segment is” and “which catalog record supplied the place.”
