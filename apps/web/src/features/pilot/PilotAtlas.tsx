import { useEffect, useMemo, useState } from "react";
import {
  fetchCanonicalAtlas,
  type CanonicalEvent,
  type CanonicalEvidence,
  type CanonicalItem,
  type CanonicalLayer,
  type CanonicalAtlas,
  type CanonicalWork,
} from "../../api";
import { PilotMap } from "./PilotMap";

type GeoViewID = "work_formation" | "item_publication" | "item_repository" | "text_mentions";

type GeoView = {
  id: GeoViewID;
  layer: CanonicalLayer;
  label: string;
  description: string;
  pending?: boolean;
  matches: (event: CanonicalEvent) => boolean;
};

const geoViews: GeoView[] = [
  {
    id: "work_formation",
    layer: "work",
    label: "Formation / redaction",
    description: "Where and when a work was formed",
    matches: (event) => event.layer === "work" && (event.type === "formation" || event.type === "formation_narrative"),
  },
  {
    id: "item_publication",
    layer: "item",
    label: "Publication places",
    description: "Where printed editions were published",
    matches: (event) => event.layer === "item" && event.type === "publication",
  },
  {
    id: "item_repository",
    layer: "item",
    label: "Current repository",
    description: "Where physical objects are held now",
    matches: (event) => event.layer === "item" && event.places?.some((place) => place.role === "current_repository") === true,
  },
  {
    id: "text_mentions",
    layer: "text",
    label: "Places mentioned",
    description: "Places named inside selected passages",
    pending: true,
    matches: (event) => event.layer === "text",
  },
];

const layerLabels: Record<CanonicalLayer, string> = {
  work: "Work",
  item: "Item",
  text: "Text",
};

function eventDate(event: CanonicalEvent) {
  return event.times?.map((time) => time.value.display).filter(Boolean).join(" · ") || "Undated";
}

function eventYear(event: CanonicalEvent) {
  const value = event.times?.[0]?.value;
  return value?.year ?? value?.start_year;
}

function readable(value: string) {
  return value.replaceAll("_", " ");
}

function EvidenceBlock({ evidence, data }: { evidence: CanonicalEvidence[]; data: CanonicalAtlas }) {
  if (!evidence.length) return null;
  return (
    <div className="evidence-stack">
      {evidence.map((item, index) => {
        const source = data.sources.find((candidate) => candidate.id === item.source_id);
        return (
          <details key={`${item.source_id}-${item.source_record}-${index}`}>
            <summary>
              <span>{source?.label ?? item.source_id}</span>
              <small>{[item.source_record, item.field].filter(Boolean).join(" · ")}</small>
            </summary>
            <blockquote>{item.raw}</blockquote>
            <p>{readable(item.extraction)} · {readable(item.review_status)}</p>
          </details>
        );
      })}
    </div>
  );
}

function LayerBadge({ layer }: { layer: CanonicalLayer }) {
  return <span className={`layer-badge ${layer}`}>{layerLabels[layer]} layer</span>;
}

function WorkInspector({ work, events, data }: { work: CanonicalWork; events: CanonicalEvent[]; data: CanonicalAtlas }) {
  const linkedItems = data.items.filter((item) => item.work_links?.some((link) => link.work_id === work.id));
  return (
    <>
      <LayerBadge layer="work" />
      <p className="inspector-kicker">Hibur · {work.id}</p>
      <h2>{work.title}</h2>
      {work.hebrew_title && <p className="hebrew-title" dir="rtl">{work.hebrew_title}</p>}
      <dl className="fact-grid">
        <div><dt>Formation assertions</dt><dd>{events.length}</dd></div>
        <div><dt>Linked items</dt><dd>{linkedItems.length}</dd></div>
        <div><dt>Families</dt><dd>{work.families?.join(" · ") || "Not classified"}</dd></div>
        <div><dt>Known aliases</dt><dd>{work.aliases?.slice(0, 5).join(" · ") || "None"}</dd></div>
      </dl>
      {!events.length && <p className="data-gap">This work is in Vatican ebr. 44, but the current creation CSV has no geo-temporal assertion for it.</p>}
      {events[0] && (
        <section className="inspector-section">
          <p className="section-label">Formation assertion</p>
          <h3>{events[0].label}</h3>
          <p>{eventDate(events[0])} · {events[0].places?.map((place) => place.raw_name).join(", ")}</p>
          <EvidenceBlock evidence={events[0].evidence} data={data} />
        </section>
      )}
      <section className="inspector-section">
        <p className="section-label">Items containing this work</p>
        <ul className="plain-list">
          {linkedItems.map((item) => <li key={item.id}><span>{item.kind === "manuscript" ? "Manuscript" : "Edition"}</span>{item.title}</li>)}
        </ul>
      </section>
    </>
  );
}

function ItemInspector({
  item,
  data,
  onOpenStory,
}: {
  item: CanonicalItem;
  data: CanonicalAtlas;
  onOpenStory: () => void;
}) {
  const workTitle = (id: string) => data.works.find((work) => work.id === id)?.title ?? id;
  const storyEvents = data.events.filter((event) => event.layer === "item" && event.subject.id === item.id);
  return (
    <>
      <LayerBadge layer="item" />
      <p className="inspector-kicker">{readable(item.kind)} · {item.id}</p>
      <h2>{item.title}</h2>
      <p className="inspector-summary">Selection shows details only; it does not narrow the shared map.</p>
      <button className="story-button" type="button" onClick={onOpenStory} disabled={!storyEvents.length}>
        <span>Open item story</span>
        <small>{storyEvents.length} event{storyEvents.length === 1 ? "" : "s"} across time</small>
      </button>
      <dl className="fact-grid compact">
        {item.attributes?.map((attribute) => (
          <div key={attribute.key}><dt>{attribute.label}</dt><dd>{attribute.values.join(" · ")}</dd></div>
        ))}
      </dl>
      <section className="inspector-section">
        <p className="section-label">Works embodied</p>
        <ul className="plain-list">
          {(item.work_links ?? []).map((link) => (
            <li key={`${link.work_id}-${link.raw_label}`}><span>{readable(link.relation)}</span>{workTitle(link.work_id)}</li>
          ))}
        </ul>
      </section>
      {item.parts?.length ? (
        <section className="inspector-section">
          <p className="section-label">Contents structure</p>
          <ol className="contents-list">
            {item.parts.map((part) => <li key={part.id}><span>{part.range}</span><strong>{part.label}</strong></li>)}
          </ol>
        </section>
      ) : null}
    </>
  );
}

function TextInspector({ data }: { data: CanonicalAtlas }) {
  const state = data.layer_states.find((item) => item.layer === "text");
  return (
    <>
      <LayerBadge layer="text" />
      <p className="inspector-kicker">Geo-temporal view · pending</p>
      <h2>Places mentioned in passages</h2>
      <p className="inspector-summary">A textual mention will be mapped as textual evidence—not as the location of a work or object.</p>
      <div className="pending-panel"><span>Current status</span><strong>{state?.status ?? "pending"}</strong><p>{state?.note}</p></div>
    </>
  );
}

export function PilotAtlas() {
  const [data, setData] = useState<CanonicalAtlas | null>(null);
  const [error, setError] = useState("");
  const [viewId, setViewId] = useState<GeoViewID>("work_formation");
  const [workFilter, setWorkFilter] = useState("all");
  const [itemKind, setItemKind] = useState<"all" | CanonicalItem["kind"]>("all");
  const [fromYear, setFromYear] = useState("");
  const [toYear, setToYear] = useState("");
  const [includeUndated, setIncludeUndated] = useState(true);
  const [entitySearch, setEntitySearch] = useState("");
  const [selectedWorkId, setSelectedWorkId] = useState("1.15:1:1.0");
  const [selectedItemId, setSelectedItemId] = useState("990001967550205171");
  const [selectedEventId, setSelectedEventId] = useState("event:work:midrash-proverbs:formation");
  const [storyOpen, setStoryOpen] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    fetchCanonicalAtlas(controller.signal).then(setData).catch((reason: Error) => {
      if (reason.name !== "AbortError") setError(reason.message);
    });
    return () => controller.abort();
  }, []);

  const activeView = geoViews.find((view) => view.id === viewId) ?? geoViews[0];
  const filteredItems = useMemo(() => data?.items.filter((item) => {
    if (itemKind !== "all" && item.kind !== itemKind) return false;
    return workFilter === "all" || item.work_links?.some((link) => link.work_id === workFilter);
  }) ?? [], [data, itemKind, workFilter]);

  const visibleEvents = useMemo(() => {
    if (!data) return [];
    const allowedItems = new Set(filteredItems.map((item) => item.id));
    return data.events.filter((event) => {
      if (!activeView.matches(event)) return false;
      if (event.layer === "work" && workFilter !== "all" && event.subject.id !== workFilter) return false;
      if (event.layer === "item" && !allowedItems.has(event.subject.id)) return false;
      const year = eventYear(event);
      if (year === undefined) return includeUndated;
      if (fromYear && year < Number(fromYear)) return false;
      if (toYear && year > Number(toYear)) return false;
      return true;
    });
  }, [activeView, data, filteredItems, fromYear, includeUndated, toYear, workFilter]);

  const selectedWork = data?.works.find((work) => work.id === selectedWorkId);
  const selectedItem = data?.items.find((item) => item.id === selectedItemId);
  const selectedEvent = data?.events.find((event) => event.id === selectedEventId);
  const selectedWorkEvents = data?.events.filter((event) => event.layer === "work" && event.subject.id === selectedWorkId) ?? [];
  const storyEvents = data?.events
    .filter((event) => event.layer === "item" && event.subject.id === selectedItemId)
    .sort((a, b) => (eventYear(a) ?? Number.MAX_SAFE_INTEGER) - (eventYear(b) ?? Number.MAX_SAFE_INTEGER)) ?? [];

  function selectView(next: GeoViewID) {
    const nextView = geoViews.find((view) => view.id === next) ?? geoViews[0];
    setViewId(next);
    setStoryOpen(false);
    setSelectedEventId("");
    if (nextView.layer === "work") setSelectedWorkId(workFilter === "all" ? "1.15:1:1.0" : workFilter);
  }

  function selectMapEvent(event: CanonicalEvent) {
    setSelectedEventId(event.id);
    if (event.layer === "work") setSelectedWorkId(event.subject.id);
    if (event.layer === "item") setSelectedItemId(event.subject.id);
  }

  if (error) return <main className="pilot-loading"><strong>Could not load the canonical pilot.</strong><p>{error}</p></main>;
  if (!data) return <main className="pilot-loading"><span>Midrash Atlas</span><strong>Preparing the atlas…</strong></main>;

  const displayPlaceIDs = new Set(data.places.filter((place) => place.display_geometry).map((place) => place.id));
  const assertedEventCount = visibleEvents.filter((event) => event.places?.some((place) => place.place_id)).length;
  const mappedEventCount = visibleEvents.filter((event) => event.places?.some((place) => place.place_id && displayPlaceIDs.has(place.place_id))).length;
  const entityCount = activeView.layer === "work" ? (workFilter === "all" ? data.works.length : 1) : activeView.layer === "item" ? filteredItems.length : 0;
  const assertedEntityCount = new Set(visibleEvents.filter((event) => event.places?.some((place) => place.place_id)).map((event) => event.subject.id)).size;
  const mappedEntityCount = new Set(visibleEvents.filter((event) => event.places?.some((place) => place.place_id && displayPlaceIDs.has(place.place_id))).map((event) => event.subject.id)).size;
  const normalizedSearch = entitySearch.trim().toLocaleLowerCase();
  const catalogueWorks = data.works.filter((work) => (workFilter === "all" || work.id === workFilter) && (!normalizedSearch || `${work.title} ${work.hebrew_title ?? ""}`.toLocaleLowerCase().includes(normalizedSearch)));
  const catalogueItems = filteredItems.filter((item) => !normalizedSearch || item.title.toLocaleLowerCase().includes(normalizedSearch));

  return (
    <main className={`pilot-shell atlas-view active-${activeView.layer}`}>
      <header className="pilot-header atlas-header">
        <div className="brand-block"><span className="brand-mark">MA</span><div><p>Midrash Atlas</p><small>Evidence across composition, object, and text</small></div></div>
        <div className="pilot-title"><p className="eyebrow">Canonical atlas · complete project catalogue</p><h1>Choose the geography.<br /><em>Then explore the evidence.</em></h1></div>
        <div className="pilot-status"><span>Current corpus</span><strong>{data.works.length} works · {data.items.length} items</strong><small>Vatican ebr. 44 remains the featured example</small></div>
      </header>

      <section className="view-picker" aria-label="Geo-temporal view">
        <div className="view-picker-title"><span>Map view</span><strong>What do you want to see?</strong></div>
        {(["work", "item", "text"] as CanonicalLayer[]).map((layer) => (
          <fieldset key={layer} className={`view-group ${layer}`}>
            <legend>{layerLabels[layer]} layer</legend>
            {geoViews.filter((view) => view.layer === layer).map((view) => (
              <button key={view.id} type="button" className={viewId === view.id ? "active" : ""} onClick={() => selectView(view.id)} aria-pressed={viewId === view.id}>
                <strong>{view.label}</strong><small>{view.description}</small>{view.pending && <i>Pending</i>}
              </button>
            ))}
          </fieldset>
        ))}
      </section>

      <section className="atlas-workspace">
        <aside className="atlas-filters">
          <div><p className="eyebrow">Refine this view</p><h2>Filters</h2></div>
          <label><span>Work embodied</span><select value={workFilter} onChange={(event) => setWorkFilter(event.target.value)}><option value="all">All {data.works.length} works</option>{data.works.map((work) => <option key={work.id} value={work.id}>{work.title}</option>)}</select></label>
          {activeView.layer === "item" && <label><span>Item type</span><select value={itemKind} onChange={(event) => setItemKind(event.target.value as typeof itemKind)}><option value="all">All items</option><option value="manuscript">Manuscripts</option><option value="printed_edition">Printed editions</option></select></label>}
          <div className="year-filter"><span>Time range</span><label><small>From</small><input inputMode="numeric" value={fromYear} onChange={(event) => setFromYear(event.target.value.replace(/\D/g, ""))} placeholder="e.g. 1500" /></label><label><small>To</small><input inputMode="numeric" value={toYear} onChange={(event) => setToYear(event.target.value.replace(/\D/g, ""))} placeholder="e.g. 2000" /></label></div>
          <label className="check-filter"><input type="checkbox" checked={includeUndated} onChange={(event) => setIncludeUndated(event.target.checked)} /><span>Include undated assertions</span></label>
          <label><span>Find in catalogue</span><input type="search" value={entitySearch} onChange={(event) => setEntitySearch(event.target.value)} placeholder={activeView.layer === "item" ? "Repository, shelfmark…" : "English or Hebrew title…"} /></label>
          <button className="clear-filters" type="button" onClick={() => { setWorkFilter("all"); setItemKind("all"); setFromYear(""); setToYear(""); setIncludeUndated(true); setEntitySearch(""); }}>Clear filters</button>

          <div className="coverage-block"><p className="section-label">Current coverage</p><strong>{assertedEntityCount} of {entityCount} entities have place assertions</strong><p>{mappedEntityCount} currently have reviewable display geometry. {activeView.id === "work_formation" && entityCount > assertedEntityCount ? "Creation narratives are loaded, but only Midrash Proverbs has been manually converted into a structured place-and-time assertion so far." : "Unmapped assertions remain visible as catalogue evidence rather than receiving guessed coordinates."}</p></div>
          <div className="entity-browser"><p className="section-label">Catalogue in scope · {activeView.layer === "work" ? catalogueWorks.length : activeView.layer === "item" ? catalogueItems.length : 0}</p>{activeView.layer === "work" && catalogueWorks.map((work) => { const workEvents = data.events.filter((event) => event.layer === "work" && event.subject.id === work.id); const mapped = workEvents.some((event) => event.places?.length); return <button key={work.id} type="button" className={selectedWorkId === work.id ? "selected" : ""} onClick={() => setSelectedWorkId(work.id)}><strong>{work.title}</strong><small>{mapped ? "Structured formation place" : workEvents.length ? "Narrative awaiting structuring" : "No formation source"}</small></button>; })}{activeView.layer === "item" && catalogueItems.map((item) => <button key={item.id} type="button" className={selectedItemId === item.id ? "selected" : ""} onClick={() => { setSelectedItemId(item.id); setStoryOpen(false); }}><strong>{item.title}</strong><small>{item.kind === "manuscript" ? "Manuscript" : "Printed edition"}</small></button>)}{activeView.layer === "text" && <p className="data-gap">No passage has been selected and processed yet.</p>}</div>
        </aside>

        <section className="map-stage atlas-map-stage">
          <div className="map-stage-heading"><div><p className="eyebrow">{layerLabels[activeView.layer]} layer · geo-temporal view</p><h2>{activeView.label}</h2><p>{activeView.description}</p></div><div className="map-metrics"><span><strong>{mappedEventCount}</strong> mapped</span><span><strong>{assertedEventCount - mappedEventCount}</strong> awaiting geometry</span><span><strong>{visibleEvents.length - assertedEventCount}</strong> narrative only</span></div></div>
          <PilotMap layer={activeView.layer} events={visibleEvents} places={data.places} selectedEventId={selectedEventId} onSelectEvent={selectMapEvent} />
          {storyOpen && selectedItem && <section className="story-drilldown"><div className="story-heading"><div><p className="eyebrow">Item drill-down</p><h2>The story of {selectedItem.title}</h2></div><button type="button" onClick={() => setStoryOpen(false)}>Close story</button></div><div className="event-timeline">{storyEvents.map((event) => <button key={event.id} type="button" className={selectedEventId === event.id ? "selected" : ""} onClick={() => setSelectedEventId(event.id)}><i /><span>{eventDate(event)}</span><strong>{event.label}</strong><small>{readable(event.type)}{event.places?.[0]?.raw_name ? ` · ${event.places[0].raw_name}` : ""}</small></button>)}</div></section>}
        </section>

        <aside className="inspector atlas-inspector">
          {activeView.layer === "work" && selectedWork && <WorkInspector work={selectedWork} events={selectedWorkEvents} data={data} />}
          {activeView.layer === "item" && selectedItem && <ItemInspector item={selectedItem} data={data} onOpenStory={() => setStoryOpen(true)} />}
          {activeView.layer === "text" && <TextInspector data={data} />}
          {selectedEvent && activeView.matches(selectedEvent) && <section className="selected-event-card"><p className="section-label">Selected map assertion</p><span>{eventDate(selectedEvent)} · {readable(selectedEvent.type)}</span><h3>{selectedEvent.label}</h3><EvidenceBlock evidence={selectedEvent.evidence} data={data} /></section>}
        </aside>
      </section>
    </main>
  );
}
