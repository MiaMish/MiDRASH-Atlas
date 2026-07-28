# NLI manuscript retrieval notes

## Recommended bibliographic source

Use the National Library of Israel's public Alma SRU endpoint:

```text
https://nli.alma.exlibrisgroup.com/view/sru/972NNL_INST
```

An exact MMS-ID query returning MARCXML is:

```text
?version=1.2
&operation=searchRetrieve
&recordSchema=marcxml
&query=alma.mms_id=990001929610205171
```

The request does not require the NLI Open Library API key. The endpoint returned
HTTP 200, `text/xml`, CORS `*`, and one MARCXML record in the July 2026 probe.

The Open Library Search and NLI-hosted IIIF services are useful when available,
but both were blocked by NLI's Cloudflare configuration from ordinary HTTP
clients during the probe. SRU was not blocked.

## Identifier layers

- The spreadsheet's 18-digit `מספר מערכת` is an Alma MMS ID.
- The public record URL is
  `https://www.nli.org.il/en/manuscripts/NNL_ALEPH{MMS_ID}/NLI`.
- Primo's `sourceRecord` display takes another `NNL_ALEPH...` identifier. It is
  a discovery/display identifier and should not replace the MARC `001` MMS ID.
- `987...` values are generally authority identifiers, not manuscript IDs.
- Alma delivery-service PIDs in `AVE $8` are neither MMS IDs nor IIIF manifest
  IDs.

## Parent and analytic-child records

NLI catalogs composite manuscripts using separate bibliographic records:

- The parent record represents the physical codex or composite manuscript.
- Analytic child records represent individual works or components within it.
- A child links to its host with MARC `773 $w={parent MMS ID}`.
- The parent commonly describes its contents in repeated `505 $a` fields.
- The Primo source-record screen merges parent fields into a child display and
  suffixes them with `P` (`245P`, `260P`, `300P`, `505P`, and so on). These are
  display-time enrichments, not ordinary MARC tags returned by SRU.

Example:

```text
child  990001967600205171  מעשה רבי יהושע בן לוי
  773 $w 990001967550205171

parent 990001967550205171  קובץ
  505 $a 1) דף 1א-289ב: מדרש תנחומא
  ...
  505 $a 14) דף 384א-384ב: חידות
```

To find children from a known parent, query:

```text
query=alma.other_system_number={parent MMS ID}
```

The result must be filtered to records whose `773 $w` exactly equals the parent
ID. For Vatican ebr. 44 this returned fourteen analytic children.

## Fields relevant to the atlas

| MARC field | Use | Caution |
|---|---|---|
| `001` | Stable manuscript/component MMS ID | Primary key for NLI retrieval |
| `245` | Record title | Child and parent titles have different scopes |
| `260 $a/$c` | Production-place display and date display | Preserve the source strings before normalization |
| `008` | Machine-oriented date/language codes | Approximate dates still require interpretation |
| `300`, `340` | Extent and physical/codicological notes | May differ between child and parent |
| `505` | Parent contents list | Textual enumeration; not a substitute for child links |
| `561` | Provenance and ownership-event notes | Often contains dates and places embedded in prose |
| `700` | People and roles, including former owners | Role is important |
| `710` | Corporate body/current owner | This is present repository geography, not production geography |
| `751` | Place, with role in `$e` | `place of writing` is production geography; `related place` is weaker |
| `773 $w` | Child-to-parent link | Follow to retrieve physical-manuscript context |
| `740` | Alternative title | Useful for matching Hiburim |
| `903` | Rights/access metadata | Preserve subfields |
| `942` indicator 1 | Repository and shelfmark | NLI-local field |
| `942` indicator 3 | External catalogue reference | NLI-local field |
| `957` | Colophon transcription/note | Often supplies the evidence for date, scribe, and writing place |
| `958` | Script style | NLI-local field |
| `907` | Rosetta digital representation/file data | Includes `IE...` representation and `FL...` file identifiers |
| `AVA` | Physical availability/holding enrichment | On a child it may refer to the parent's holding |
| `AVE` | Digital-service availability | `$8` is a delivery-service PID, not a URL |

Repository coordinates seen in Primo's enriched source-record display come from
authority expansion. They describe the present repository. They must not be
used as the manuscript's place of production unless another field explicitly
supports that interpretation.

## IIIF and delivery links

An Alma OpenURL resolver can be built from the MMS ID:

```text
https://nli.alma.exlibrisgroup.com/view/uresolver/972NNL_INST/openurl
  ?rft.mms_id={MMS_ID}
  &svc_dat=viewit
  &is_new_ui=true
  &rft_dat=language=eng
```

The resolver may expose more than one service. For Vatican ebr. 34 it resolved
one service to the Vatican Library's IIIF Presentation 2 manifest:

```text
https://digi.vatlib.it/iiif/MSS_Vat.ebr.34/manifest.json
```

Therefore, NLI is often acting as an aggregator and resolver. The final IIIF
manifest can belong to the current custodian rather than NLI. Treat the
manifest URL as discovered delivery metadata, not as a predictable derivative
of the NLI MMS ID.

For NLI-hosted scans, `907 $c` is the Rosetta `IE...` representation ID used by
the delivery viewer and `907 $d` is an `FL...` file ID. The corresponding IIIF
candidate URLs are:

```text
https://iiif.nli.org.il/IIIFv21/{IE_ID}/manifest
https://iiif.nli.org.il/IIIFv21/{FL_ID}/info.json
```

Availability and access policy still need to be checked. For externally hosted
manuscripts, the Alma resolver may instead redirect to the custodian's manifest,
as it does for Vatican ebr. 34.

## Initial coverage sample

A July 2026 run over the first forty unique valid MMS IDs in the project sheet
returned all forty records with no SRU errors:

- date display: 40/40
- current repository: 40/40
- shelfmark: 40/40
- script style: 37/40
- provenance notes: 23/40
- digital services: 19/40
- analytic child records: 4/40, with four parents followed successfully
- `751` place assertions: 4/40

No record in this sample used `260 $a` for production place. Production
geography should therefore be taken primarily from `751` where the role is
`place of writing`, then from colophon/provenance notes with an explicit
evidence and confidence layer. Current-repository cities must remain separate.

## Full project-sheet retrieval

The first full run read the manuscript spreadsheet, discarded the single
non-numeric value in the system-number column, deduplicated repeated records,
and requested 340 unique MMS IDs in batches of forty.

Results:

- 340/340 requested records returned
- 0 SRU errors
- 91 requested records are analytic children
- 81 additional parent records were followed
- 421 total MARC records were preserved
- date display: 310/340 directly; 316/340 after parent fallback
- current owner and shelfmark: 340/340
- script style: 303/340 directly; 310/340 after parent fallback
- provenance notes: 166/340
- colophon notes: 106/340
- explicit place assertions: 65/340 directly; 77/340 after parent fallback
- explicit `place of writing`: 52/340 after parent fallback, covering 40
  distinct place labels
- Rosetta digital-object data: 274/340 directly; 279/340 after parent fallback
- Sfardata identifiers: 76/340

The raw SRU cache is in `data/raw/nli_sru/` and the normalized, lossless JSON
export is `data/derived/nli_marc_normalized.json`. The normalized records retain the
complete parsed MARC fields in addition to convenience fields.

Parent data should be inherited contextually at query/UI time rather than
copied destructively into child records. A child represents a work-bearing
segment; its parent represents the physical codex.

## Prototype

Fetch one record:

```bash
python3 tools/nli/nli_sru.py 990001929610205171 --output data/vatican-34.json
```

Fetch a parent plus its analytic children:

```bash
python3 tools/nli/nli_sru.py 990001967550205171 \
  --related children \
  --output data/vatican-44.json
```

Fetch IDs from the project spreadsheet:

```bash
python3 tools/nli/nli_sru.py \
  --csv data/source/midrashim_from_google_sheets.csv \
  --limit 10 \
  --output data/nli-sample.json
```

Raw SRU responses are cached under `data/raw/nli_sru` by default. Keep these
source responses alongside normalized data so later parsing changes are
auditable.
