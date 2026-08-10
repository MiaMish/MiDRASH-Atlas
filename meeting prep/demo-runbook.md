# Midrash Atlas demo runbook

Use this as a short, evidence-first walkthrough of the Atlas UI.  The best
story is: *we do not hide disagreement or uncertainty in manuscript metadata;
we make the evidence, the choices, and the review trail inspectable.*

## Before you begin

Run the API and UI in separate terminals from the repository root:

```bash
# Terminal 1
GOCACHE=/private/tmp/midrash-atlas-go-cache \
  /opt/homebrew/bin/go run ./apps/api/cmd/atlas serve
```

```bash
# Terminal 2
npm run web:dev
```

Open <http://localhost:5173>. Confirm the Atlas summary shows non-zero data.
If it shows `0 mapped places`, the API is not running or is not reachable on
port 8080.

## Suggested 6–8 minute walkthrough

### 1. Open with the atlas, not a claim (about 45 seconds)

![Atlas controls annotated](images/atlas-overview-annotated.jpg)

Say:

> Midrash Atlas turns catalog metadata into an inspectable historical atlas.
> Instead of choosing one supposedly definitive place or date, it keeps the
> competing catalog assertions visible and lets the researcher examine them.

Point to the summary and the map. Emphasize that the three counts distinguish
mapped places, manuscript records, and assertions without geometry.

### 2. Narrow the research question with filters (about 1 minute)

![Atlas filters and controls](images/atlas-overview.jpg)

Show these controls in order:

1. Set a smaller **temporal window**.
2. Select one **geographic source**; explain that several sources can be
   enabled together rather than silently merged.
3. Choose a **parent/child origin** or **Hibur** to focus the view.
4. Toggle **Include undated records** and adjust the **circa** policy.

Say:

> The filters change a display view; they do not rewrite the stored evidence.
> A date recorded as circa remains a circa date, while the UI applies a
> transparent temporal window for exploration.

### 3. Inspect a place and a manuscript (about 2 minutes)

Click a visible map geometry, then open a manuscript card from its drill-down.

![Manuscript drill-down annotated](images/manuscript-drilldown-annotated.jpg)

Point out:

- the original geographic statement and source layer;
- whether the evidence came directly from the manuscript or from a parent;
- the date assertions and linked Hiburim;
- the catalog metadata and expandable MARC provenance;
- the NLI/digital-object links where available.

Say:

> Every result can be traced back to the particular catalog evidence that put
> it on the map. The interface shows provenance rather than turning inference
> into a hidden fact.

### 4. Make absence visible (about 1 minute)

Click **unmapped assertions** in the summary.

![Unmapped evidence annotated](images/unmapped-records-annotated.jpg)

Say:

> Missing or ambiguous geometry is a research result, not something we erase
> by assigning a made-up point. These records stay queryable and are grouped by
> the unresolved place concept.

### 5. Show the curation workflow (about 2 minutes)

Use the top workspace switcher to open **Location curation**.

![Location curation annotated](images/location-curation-annotated.jpg)

Walk through:

1. Select a place and read its status before changing anything.
2. Show the geometry source and map preview.
3. Explain the distinction between unreviewed, human-changed, and
   human-reviewed geometry.
4. Open the audit history.

Say:

> Automated gazetteer and LLM work can suggest candidates, but it never
> becomes authoritative by itself. Human decisions are preserved in an
> append-only audit trail, and normal atlas rendering uses the saved review
> state rather than doing live geocoding.

## Closing (20 seconds)

> The project makes manuscript geography useful for exploration while keeping
> its ambiguity, source strategy, and human editorial decisions explicit.

## Demo recovery notes

| If you see | Do this |
| --- | --- |
| `0 mapped places` / `0 records` | Start the Go API command above; the Vite UI proxies `/api` to port 8080. |
| Map is blank but counts are present | Reset filters, then refresh once. |
| A location is not on the map | Open **unmapped assertions**; do not claim it has a verified geometry. |
| Asked why one manuscript appears more than once | Explain that each retained evidence assertion can point to a different place or source strategy. |

## Avoid in the demo

- Do not describe a catalog place statement as a verified historical location.
- Do not call AI-generated candidates authoritative.
- Do not imply that the temporal display policy changes the underlying date.
