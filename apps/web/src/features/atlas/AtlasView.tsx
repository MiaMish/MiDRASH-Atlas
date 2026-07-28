import { useEffect, useMemo, useState } from "react";
import {
  fetchAtlasView,
  type AtlasEvent,
  type AtlasFeature,
  type AtlasFilters,
  type AtlasView as AtlasViewData,
} from "../../api";
import { AtlasMap } from "./AtlasMap";

const defaultFilters: AtlasFilters = {
  circa_years: 10,
  include_undated: true,
  geo_source_ids: ["nli_751_writing_place"],
  origins: [],
  hibur_ids: [],
  location_statuses: [],
};

function toggleValue(values: string[], value: string) {
  return values.includes(value)
    ? values.filter((item) => item !== value)
    : [...values, value];
}

function displayDate(event: AtlasEvent) {
  const values = (event.temporal_assertions ?? [])
    .map((assertion) => {
      if (assertion.effective_interval) {
        const { start_year: start, end_year: end } = assertion.effective_interval;
        return start === end ? String(start) : `${start}–${end}`;
      }
      return assertion.evidence.raw;
    })
    .filter(Boolean);
  return [...new Set(values)].join("; ") || "Undated";
}

function shelfmark(event: AtlasEvent) {
  const first = event.record.shelfmarks?.[0];
  if (!first) return "";
  return [first.repository, first.shelfmark].filter(Boolean).join(" · ");
}

function sourceName(event: AtlasEvent, data: AtlasViewData) {
  return (
    data.facets.geo_sources.find((source) => source.id === event.assertion.source_type_id)?.label ??
    event.assertion.source_type_id
  );
}

export function AtlasView() {
  const [filters, setFilters] = useState<AtlasFilters>(defaultFilters);
  const [data, setData] = useState<AtlasViewData | null>(null);
  const [selectedPlaceId, setSelectedPlaceId] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      setLoading(true);
      fetchAtlasView(filters, controller.signal)
        .then((next) => {
          if (!active) return;
          setData(next);
          setError("");
          if (filters.start_year === undefined || filters.end_year === undefined) {
            setFilters((current) => ({
              ...current,
              start_year: current.start_year ?? next.time_bounds.min_year,
              end_year: current.end_year ?? next.time_bounds.max_year,
            }));
          }
        })
        .catch((reason: Error) => {
          if (active && reason.name !== "AbortError") setError(reason.message);
        })
        .finally(() => {
          if (active) setLoading(false);
        });
    }, 220);
    return () => {
      active = false;
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [filters]);

  useEffect(() => {
    if (!data?.features.length) {
      setSelectedPlaceId("");
      return;
    }
    if (!data.features.some((feature) => feature.id === selectedPlaceId)) {
      setSelectedPlaceId(data.features[0].id);
    }
  }, [data, selectedPlaceId]);

  const selected = useMemo(
    () => data?.features.find((feature) => feature.id === selectedPlaceId),
    [data, selectedPlaceId],
  );
  const contained = useMemo(() => {
    const ids = new Set(selected?.properties.contained_place_ids ?? []);
    return data?.features.filter((feature) => ids.has(feature.id)) ?? [];
  }, [data, selected]);
  const bounds = data?.time_bounds ?? { min_year: 800, max_year: 2000 };
  const startYear = filters.start_year ?? bounds.min_year;
  const endYear = filters.end_year ?? bounds.max_year;

  function reset() {
    setFilters({
      ...defaultFilters,
      start_year: bounds.min_year,
      end_year: bounds.max_year,
    });
  }

  return (
    <section className="atlas-workspace">
      <div className="atlas-toolbar">
        <div className="atlas-metrics" aria-label="Filtered atlas summary">
          <span><strong>{data?.summary.mapped_places ?? 0}</strong> mapped places</span>
          <span><strong>{data?.summary.matched_records ?? 0}</strong> records</span>
          <span><strong>{data?.summary.unmapped_assertions ?? 0}</strong> unmapped assertions</span>
        </div>
        <div className="time-filter">
          <div className="time-heading">
            <label htmlFor="atlas-start-year">From <strong>{startYear}</strong></label>
            <span>Temporal window</span>
            <label htmlFor="atlas-end-year">To <strong>{endYear}</strong></label>
          </div>
          <div className="dual-range">
            <input
              id="atlas-start-year"
              type="range"
              min={bounds.min_year}
              max={bounds.max_year}
              value={startYear}
              onChange={(event) => {
                const value = Math.min(Number(event.target.value), endYear);
                setFilters((current) => ({ ...current, start_year: value }));
              }}
            />
            <input
              id="atlas-end-year"
              type="range"
              min={bounds.min_year}
              max={bounds.max_year}
              value={endYear}
              onChange={(event) => {
                const value = Math.max(Number(event.target.value), startYear);
                setFilters((current) => ({ ...current, end_year: value }));
              }}
            />
          </div>
        </div>
        <button className="secondary compact-button" type="button" onClick={reset}>Reset filters</button>
      </div>

      <div className="atlas-layout">
        <aside className="atlas-filters">
          <div className="rail-heading">
            <div>
              <p className="eyebrow">Evidence strategy</p>
              <h2>Filters</h2>
            </div>
            {loading && <span className="loading-label">Updating…</span>}
          </div>

          <fieldset>
            <legend>Geographic sources</legend>
            {data?.facets.geo_sources.map((source) => (
              <label className="filter-check" key={source.id}>
                <input
                  type="checkbox"
                  checked={filters.geo_source_ids.includes(source.id)}
                  onChange={() =>
                    setFilters((current) => ({
                      ...current,
                      geo_source_ids: toggleValue(current.geo_source_ids, source.id),
                    }))
                  }
                />
                <span>
                  <strong>{source.label}</strong>
                  <small>{source.count} assertions · {source.short_description}</small>
                </span>
              </label>
            ))}
            <p className="filter-hint">No selection includes every geographic source.</p>
          </fieldset>

          <fieldset>
            <legend>Parent/child origin</legend>
            {data?.facets.origins.map((origin) => (
              <label className="filter-check compact" key={origin.id}>
                <input
                  type="checkbox"
                  checked={filters.origins.includes(origin.id)}
                  onChange={() =>
                    setFilters((current) => ({
                      ...current,
                      origins: toggleValue(current.origins, origin.id),
                    }))
                  }
                />
                <span><strong>{origin.label}</strong><small>{origin.count}</small></span>
              </label>
            ))}
            <p className="filter-hint">No selection includes direct and parent-derived assertions.</p>
          </fieldset>

          <label>
            Hibur
            <select
              value={filters.hibur_ids[0] ?? ""}
              onChange={(event) =>
                setFilters((current) => ({
                  ...current,
                  hibur_ids: event.target.value ? [event.target.value] : [],
                }))
              }
            >
              <option value="">All linked works</option>
              {data?.facets.hiburim.map((hibur) => (
                <option value={hibur.id} key={hibur.id}>
                  {hibur.label}{hibur.english ? ` · ${hibur.english}` : ""} ({hibur.count})
                </option>
              ))}
            </select>
          </label>

          <label>
            Geometry review state
            <select
              value={filters.location_statuses[0] ?? ""}
              onChange={(event) =>
                setFilters((current) => ({
                  ...current,
                  location_statuses: event.target.value ? [event.target.value] : [],
                }))
              }
            >
              <option value="">All mapped states</option>
              {data?.facets.location_statuses.map((status) => (
                <option value={status.id} key={status.id}>
                  {status.label} ({status.count})
                </option>
              ))}
            </select>
          </label>

          <label className="filter-check compact">
            <input
              type="checkbox"
              checked={filters.include_undated}
              onChange={(event) =>
                setFilters((current) => ({ ...current, include_undated: event.target.checked }))
              }
            />
            <span><strong>Include undated records</strong></span>
          </label>

          <label>
            Circa policy: ±{filters.circa_years} years
            <input
              type="range"
              min={0}
              max={50}
              step={5}
              value={filters.circa_years}
              onChange={(event) =>
                setFilters((current) => ({ ...current, circa_years: Number(event.target.value) }))
              }
            />
          </label>
          {error && <p className="atlas-error" role="alert">{error}</p>}
        </aside>

        <AtlasMap
          features={data?.features ?? []}
          selectedPlaceId={selectedPlaceId}
          containedPlaceIds={selected?.properties.contained_place_ids ?? []}
          onSelectPlace={setSelectedPlaceId}
        />

        <AtlasDrilldown feature={selected} contained={contained} data={data} />
      </div>
    </section>
  );
}

function AtlasDrilldown({
  feature,
  contained,
  data,
}: {
  feature?: AtlasFeature;
  contained: AtlasFeature[];
  data: AtlasViewData | null;
}) {
  if (!feature || !data) {
    return (
      <aside className="atlas-drilldown empty">
        <p>Select a mapped place to inspect its manuscript evidence.</p>
      </aside>
    );
  }
  const properties = feature.properties;
  const displayedFeatures = [feature, ...contained];
  const recordIDs = new Set(
    displayedFeatures.flatMap((item) =>
      (item.properties.events ?? []).map((event) => event.record.id),
    ),
  );
  const assertionCount = displayedFeatures.reduce(
    (total, item) => total + item.properties.assertion_count,
    0,
  );
  return (
    <aside className="atlas-drilldown">
      <div className="drilldown-heading">
        <p className="eyebrow">Place drill-down</p>
        <h2>{properties.place_label}</h2>
        <p>
          {recordIDs.size} record{recordIDs.size === 1 ? "" : "s"} ·{" "}
          {assertionCount} assertion{assertionCount === 1 ? "" : "s"}
        </p>
        {contained.length > 0 && (
          <p className="containment-summary">
            Includes {contained.length} mapped place{contained.length === 1 ? "" : "s"} contained
            by this modern geometry.
          </p>
        )}
      </div>
      <div className="place-status-line">
        <span>{properties.coordinate_status.replaceAll("_", " ")}</span>
        <span>{properties.spatial_precision?.replaceAll("_", " ") ?? "unknown precision"}</span>
      </div>
      <div className="drilldown-events">
        <h3>{properties.place_label} assertions</h3>
        <AtlasEventList events={properties.events ?? []} data={data} />
        {contained.map((containedFeature) => (
          <section className="contained-place" key={containedFeature.id}>
            <div className="contained-place-heading">
              <h3>{containedFeature.properties.place_label}</h3>
              <span>
                {containedFeature.properties.record_count} record
                {containedFeature.properties.record_count === 1 ? "" : "s"}
              </span>
            </div>
            <p>Spatially contained by the selected modern display geometry.</p>
            <AtlasEventList events={containedFeature.properties.events ?? []} data={data} />
          </section>
        ))}
      </div>
    </aside>
  );
}

function AtlasEventList({ events, data }: { events: AtlasEvent[]; data: AtlasViewData }) {
  return (
    <>
      {events.map((event) => (
          <details key={event.assertion.id}>
            <summary>
              <span>
                <strong>{event.record.titles?.[0] ?? event.record.id}</strong>
                <small>{displayDate(event)}</small>
              </span>
            </summary>
            <dl>
              <div><dt>Geo source</dt><dd>{sourceName(event, data)}</dd></div>
              <div><dt>Origin</dt><dd>{event.assertion.scope.origin.replaceAll("_", " ")}</dd></div>
              <div><dt>Catalog evidence</dt><dd>{event.assertion.evidence.raw}</dd></div>
              <div><dt>Hibur</dt><dd>
                {(event.hiburim ?? []).length
                  ? (event.hiburim ?? [])
                    .map((hibur) => hibur.english || hibur.label)
                    .join("; ")
                  : "No linked Hibur"}
              </dd></div>
              {shelfmark(event) && <div><dt>Shelfmark</dt><dd>{shelfmark(event)}</dd></div>}
              <div><dt>Record type</dt><dd>{event.record.kind.replaceAll("_", " ")}</dd></div>
            </dl>
            <div className="record-actions">
              {event.record.public_record_url && (
                <a href={event.record.public_record_url} target="_blank" rel="noreferrer">
                  NLI manuscript record
                </a>
              )}
              {event.record.resolver_url && (
                <a href={event.record.resolver_url} target="_blank" rel="noreferrer">
                  Digital object
                </a>
              )}
            </div>
          </details>
      ))}
    </>
  );
}
