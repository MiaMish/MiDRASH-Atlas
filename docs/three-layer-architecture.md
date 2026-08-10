# Three-layer atlas architecture

## Product model

The Midrash Atlas presents three connected but non-interchangeable layers:

1. **Work (`work`)** — the Hibbur or composition: its identity, titles,
   families, formation, redaction, and competing scholarly accounts of where
   and when those processes occurred.
2. **Item (`item`)** — a physical or bibliographic object. In the PoC the item
   kinds are manuscripts and printed editions. An item has a biography made of
   events such as production, copying, annotation, censorship, acquisition,
   sale, transfer, binding, and custody. A composite item may contain several
   works and may have independently produced parts.
3. **Text (`text`)** — passages belonging to a work or to a particular textual
   witness. Named entities extracted from a passage describe what the text
   mentions. They do not by themselves establish that the work or item was
   historically located at the mentioned place.

Places, agents, time spans, and evidence are shared resources. Every entity,
event, assertion, map observation, filter, and drill-down result nevertheless
declares the layer whose question it answers.

## Evidence and events

The atlas preserves assertions rather than resolving disagreement into one
fact. A layer-scoped event has a subject, event type, optional related entities,
agent roles, place assertions, temporal assertions, and one or more evidence
records. Assertions preserve the original source value, extraction method,
review status, and confidence. AI-produced values additionally carry the shared
AI provenance record.

The same place can therefore participate in different claims without those
claims becoming equivalent:

- a work-formation assertion says a composition was redacted in a place;
- an item event says a manuscript was copied, sold, or held in a place;
- a text mention says that a passage names a place.

## Preprocessing boundary

The application does not clean CSV or MARC records. A standalone preprocessing
flow accepts heterogeneous project sources, validates identifiers and joins,
preserves source evidence, structures approved assertions, and emits canonical
application-ready data under `data/prepared`.

MARC is one upstream source among many. The current NLI retrieval and MARC
normalization code remains temporarily available while the replacement is
built, but no MARC-specific field or tag belongs in the new atlas API contract.

Preprocessing has two outputs:

1. a machine-readable validation report that blocks strict generation while
   errors remain; and
2. a canonical three-layer document containing only validated relationships.

Generated map and UI documents are projections of that canonical document, not
an additional source of truth.

## UI contract

Work, Item, and Text organize the available **geo-temporal views**; they are not
three separate application tabs. The primary atlas control answers “what do you
want to see?” and groups choices such as work formation, edition publication,
current item repository, and places mentioned in text beneath their owning
layer. The map shows all matching assertions by default. Filters may narrow the
work, item kind, and time range, while selecting an entity opens its details
without silently narrowing the map.

An entity biography is a drill-down rather than another map layer. For example,
opening an item's story reveals its ordered production, ownership, sale,
annotation, censorship, and custody events. Cross-layer relationships appear
inside entity details. Each active view retains its own legend, counts, and
language so the meaning of every map mark remains explicit; a later comparison
mode may deliberately overlay compatible views.

## Initial vertical slice

The first end-to-end demonstration centers on **Vatican Ebr. 44**
(`990001967550205171`) and its seven ontology-linked works. Its edition subset
contains every edition-source row that exactly identifies **Midrash Proverbs**
(`1.15:1:1.0`); it is not an edition sample for all seven manuscript works:

- work formation in the Land of Israel, 860–940, retained as a sourced claim;
- manuscript contents and item-biography events, including its dated 1541
  acquisition evidence and current Vatican custody;
- related printed editions as item records with publication events; and
- selected Sefaria passages with place/person mentions, initially including a
  passage that mentions Babylon, Egypt, and Jerusalem.

The slice must show the complete interaction before the migration expands to
the full corpus. Once it replaces the current behavior, the old MARC-centered
application data and implementation will be removed rather than maintained as
a parallel legacy path.

## Full catalogue expansion

The canonical export now includes all 167 usable ontology works, eight clearly
marked manuscript-reference stubs, 340 manuscripts with normalized NLI
catalogue evidence, and 317 printed editions. Vatican Ebr. 44 and Midrash
Proverbs remain the featured hand-structured example rather than the boundary
of the dataset.

Bulk NLI enrichment creates production and current-custody assertions only from
explicit catalogue fields. Unstructured creation and provenance narratives are
preserved as evidence awaiting curation. A place assertion without approved
display geometry remains visible in coverage counts and entity detail, but does
not receive a guessed map marker.
