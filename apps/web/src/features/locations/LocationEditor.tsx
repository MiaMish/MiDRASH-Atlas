import { useEffect, useMemo, useState } from "react";
import {
  fetchGeometryRevisions,
  fetchLocationOverview,
  saveGeometryRevision,
  type GeoJSONGeometry,
  type GeometryRevision,
  type LocationOverview,
} from "../../api";
import { GeometryMapEditor } from "./GeometryMapEditor";

const statusCopy: Record<
  LocationOverview["coordinate_status"],
  { option: string; label: string; detail: string; tone: string }
> = {
  no_geometry: {
    option: "—",
    label: "No valid coordinates",
    detail: "This place cannot be shown on the map yet.",
    tone: "missing",
  },
  unreviewed_candidate: {
    option: "●",
    label: "Coordinates available · not human-reviewed",
    detail: "An AI-assisted gazetteer candidate is displayed for review.",
    tone: "unreviewed",
  },
  curated_unreviewed: {
    option: "●",
    label: "Coordinates available · not human-reviewed",
    detail: "A configured geometry is displayed, but still needs human review.",
    tone: "unreviewed",
  },
  human_draft: {
    option: "◆",
    label: "Changed by a human · not reviewed",
    detail: "A person changed this geometry, but it is still a draft.",
    tone: "changed",
  },
  reviewed_by_human: {
    option: "✓",
    label: "Reviewed by a human",
    detail: "A person reviewed these coordinates without changing them.",
    tone: "reviewed",
  },
  changed_by_human: {
    option: "◆",
    label: "Changed by a human · not reviewed",
    detail: "A person changed this geometry, but it is still a draft.",
    tone: "changed",
  },
  changed_and_reviewed_by_human: {
    option: "✓◆",
    label: "Changed and reviewed by a human",
    detail: "A person changed these coordinates and marked them reviewed.",
    tone: "reviewed",
  },
};

function presentationFor(location: LocationOverview) {
  if (location.has_valid_coordinates) return statusCopy[location.coordinate_status];
  switch (location.ai_status) {
    case "needs_candidates":
      return {
        option: "!",
        label: "AI checked · better candidates needed",
        detail: "The AI reviewed the available evidence but found no usable location candidate.",
        tone: "ai-unresolved",
      };
    case "ambiguous":
      return {
        option: "?",
        label: "AI checked · location is ambiguous",
        detail: "The AI found multiple plausible locations and correctly declined to guess.",
        tone: "ai-unresolved",
      };
    case "technical_failure":
      return {
        option: "×",
        label: "AI attempt failed",
        detail: "The curation request failed technically and should be retried.",
        tone: "ai-failed",
      };
    default:
      return {
        option: "—",
        label: "Not mapped · AI not checked yet",
        detail: "No AI curation attempt or human geometry exists for this place.",
        tone: "missing",
      };
  }
}

function sameGeometry(left: GeoJSONGeometry | null, right?: GeoJSONGeometry) {
  return JSON.stringify(left) === JSON.stringify(right ?? null);
}

export function LocationEditor() {
  const [locations, setLocations] = useState<LocationOverview[]>([]);
  const [placeId, setPlaceId] = useState("");
  const [geometry, setGeometry] = useState<GeoJSONGeometry | null>(null);
  const [geometryText, setGeometryText] = useState("");
  const [precision, setPrecision] = useState("locality");
  const [changeReason, setChangeReason] = useState("");
  const [reviewedByHuman, setReviewedByHuman] = useState(false);
  const [history, setHistory] = useState<GeometryRevision[]>([]);
  const [status, setStatus] = useState("Loading locations…");

  const location = useMemo(
    () => locations.find((item) => item.place_id === placeId),
    [locations, placeId],
  );
  const counts = useMemo(
    () => ({
      total: locations.length,
      mapped: locations.filter((item) => item.has_valid_coordinates).length,
      reviewed: locations.filter((item) => item.reviewed_by_human).length,
      aiUnresolved: locations.filter(
        (item) =>
          !item.has_valid_coordinates &&
          ["needs_candidates", "ambiguous", "technical_failure"].includes(item.ai_status),
      ).length,
      notAttempted: locations.filter((item) => item.ai_status === "not_attempted").length,
    }),
    [locations],
  );

  useEffect(() => {
    void reloadLocations();
  }, []);

  useEffect(() => {
    if (!location) return;
    loadLocation(location);
    void refreshHistory(location.place_id);
  }, [location?.place_id, location?.latest_revision]);

  async function reloadLocations(preferredPlaceId?: string) {
    try {
      const items = await fetchLocationOverview();
      items.sort((a, b) =>
        a.place_label.localeCompare(b.place_label, undefined, { sensitivity: "base" }),
      );
      setLocations(items);
      const requested = preferredPlaceId ?? placeId;
      const selected =
        items.find((item) => item.place_id === requested) ??
        items.find((item) => item.has_valid_coordinates) ??
        items[0];
      setPlaceId(selected?.place_id ?? "");
      if (selected?.place_id === requested) loadLocation(selected);
      setStatus("");
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Could not load locations.");
    }
  }

  function loadLocation(next: LocationOverview) {
    setGeometry(next.geometry ?? null);
    setGeometryText(next.geometry ? JSON.stringify(next.geometry, null, 2) : "");
    setPrecision(next.spatial_precision ?? "locality");
    setReviewedByHuman(next.reviewed_by_human);
  }

  function updateGeometry(next: GeoJSONGeometry) {
    setGeometry(next);
    setGeometryText(JSON.stringify(next, null, 2));
  }

  function applyGeometryText() {
    try {
      const parsed = JSON.parse(geometryText) as GeoJSONGeometry;
      if (!["Point", "Polygon", "MultiPolygon"].includes(parsed.type)) {
        throw new Error("Only Point, Polygon, and MultiPolygon are supported.");
      }
      updateGeometry(parsed);
      setStatus("Geometry preview updated; it is not saved yet.");
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Invalid GeoJSON geometry.");
    }
  }

  async function refreshHistory(id = placeId) {
    if (!id) return;
    try {
      setHistory(await fetchGeometryRevisions(id, true));
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Could not load history.");
    }
  }

  async function save() {
    if (!location || !geometry) {
      setStatus("Add valid coordinates before saving.");
      return;
    }
    if (!changeReason.trim()) {
      setStatus("A change or review reason is required for the audit trail.");
      return;
    }
    const changed = !sameGeometry(geometry, location.geometry);
    const humanAction = changed ? "changed" : reviewedByHuman ? "reviewed" : "draft";
    try {
      const revision = await saveGeometryRevision(placeId, {
        geometry_variant_id: "modern_place",
        label: `Modern place for ${location.place_label}`,
        geometry,
        spatial_precision: precision,
        interpretation_source: location.geometry_source ?? "modern_gazetteer_geometry",
        interpretation_note: location.ai_comment ?? location.interpretation_note ?? "",
        confidence: "medium",
        review_status: reviewedByHuman ? "reviewed_by_human" : "draft",
        human_action: humanAction,
        change_reason: changeReason,
        ai_provenance: location.ai_provenance,
      });
      setChangeReason("");
      setStatus(
        `Saved ${humanAction === "reviewed" ? "human review" : "geometry revision"} ${revision.revision} at ${new Date(
          revision.created_at,
        ).toLocaleString()}.`,
      );
      await reloadLocations(placeId);
      await refreshHistory(placeId);
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Save failed.");
    }
  }

  const currentStatus = location ? presentationFor(location) : statusCopy.no_geometry;
  const geometryChanged = location ? !sameGeometry(geometry, location.geometry) : false;

  return (
    <>
      <section className="location-summary" aria-label="Location coverage">
        <div><strong>{counts.total}</strong><span>place concepts</span></div>
        <div><strong>{counts.mapped}</strong><span>with valid coordinates</span></div>
        <div><strong>{counts.reviewed}</strong><span>human-reviewed</span></div>
        <div><strong>{counts.aiUnresolved}</strong><span>AI checked · unresolved</span></div>
        <div><strong>{counts.notAttempted}</strong><span>not checked by AI</span></div>
      </section>

      <section className="location-workspace">
        <aside className="location-form">
          <label>
            Place concept
            <select value={placeId} onChange={(event) => setPlaceId(event.target.value)}>
              {locations.map((item) => (
                <option key={item.place_id} value={item.place_id}>
                  {presentationFor(item).option} {item.place_label} — {presentationFor(item).label}
                </option>
              ))}
            </select>
          </label>

          <div className={`coordinate-status ${currentStatus.tone}`} role="status">
            <span className="status-mark" aria-hidden="true">{currentStatus.option}</span>
            <div>
              <strong>{currentStatus.label}</strong>
              <p>{currentStatus.detail}</p>
            </div>
          </div>

          <div className="status-legend" aria-label="Coordinate status legend">
            <span><b>✓</b> Human-reviewed</span>
            <span><b>◆</b> Human changed</span>
            <span><b>●</b> Unreviewed coordinates</span>
            <span><b>!</b> AI needs better candidates</span>
            <span><b>?</b> AI found an ambiguous place</span>
            <span><b>×</b> AI request failed technically</span>
            <span><b>—</b> AI not checked</span>
          </div>

          <div className="source-card">
            <strong>{location?.geometry_source?.replaceAll("_", " ") ?? "No geometry source"}</strong>
            <p>
              Modern display geometry is not a claim that the same boundary applied historically.
            </p>
          </div>
          <label>
            Spatial precision
            <select value={precision} onChange={(event) => setPrecision(event.target.value)}>
              <option value="locality">Locality</option>
              <option value="region">Region</option>
              <option value="modern_country">Modern country</option>
              <option value="interpreted_region">Interpreted region</option>
              <option value="unknown">Unknown</option>
            </select>
          </label>
          <label>
            GeoJSON geometry
            <textarea
              className="geometry-json"
              rows={9}
              placeholder="Click the map to create a point, or paste valid GeoJSON."
              value={geometryText}
              onChange={(event) => setGeometryText(event.target.value)}
            />
          </label>
          <button className="secondary" type="button" onClick={applyGeometryText}>
            Preview GeoJSON
          </button>

          <label className="review-toggle">
            <input
              type="checkbox"
              checked={reviewedByHuman}
              disabled={!geometry}
              onChange={(event) => setReviewedByHuman(event.target.checked)}
            />
            <span>
              <strong>Reviewed by a human</strong>
              <small>I inspected this geometry and consider it suitable for the atlas display.</small>
            </span>
          </label>

          {geometryChanged && (
            <p className="unsaved-change">◆ Coordinates have been changed in this editor.</p>
          )}
          <label>
            Why are you changing or reviewing this?
            <textarea
              rows={2}
              required
              value={changeReason}
              onChange={(event) => setChangeReason(event.target.value)}
            />
          </label>
          <button type="button" disabled={!geometry} onClick={() => void save()}>
            {reviewedByHuman ? "Save and mark human-reviewed" : "Save audited draft"}
          </button>
          <p className="status" aria-live="polite">{status}</p>
        </aside>

        <div className="location-canvas">
          <div className="map-wrap">
            <GeometryMapEditor geometry={geometry} onGeometryChange={updateGeometry} />
            <div className={`map-status-chip ${currentStatus.tone}`}>{currentStatus.label}</div>
          </div>
          <section className="audit-panel">
            <div className="panel-heading">
              <div>
                <p className="eyebrow">Append-only history</p>
                <h2>Audit trail</h2>
              </div>
              <button className="text-button" type="button" onClick={() => void refreshHistory()}>
                Refresh
              </button>
            </div>
            {(location?.ai_history?.length ?? 0) > 0 && (
              <section className="ai-run-history">
                <h3>AI curation runs</h3>
                {[...(location?.ai_history ?? [])].reverse().map((entry) => (
                  <article className="ai-audit-comment" key={`${entry.run_id}-${entry.status}`}>
                    <div>
                      <strong>
                        {entry.current ? "Current · " : ""}
                        {entry.status.replaceAll("_", " ")}
                      </strong>
                      <span>
                        {entry.provider || "unknown provider"}/{entry.model || "unknown model"}
                        {entry.generated_at
                          ? ` · ${new Date(entry.generated_at).toLocaleString()}`
                          : ""}
                      </span>
                    </div>
                    {entry.comment && <p>{entry.comment}</p>}
                    {entry.error && <p className="ai-error">Technical error: {entry.error}</p>}
                  </article>
                ))}
              </section>
            )}
            {history.length === 0 ? (
              <p>No human revisions have been saved for this place.</p>
            ) : (
              <ol className="audit-list">
                {history.map((revision) => (
                  <li key={revision.id}>
                    <strong>{revision.geometry_variant_id} · revision {revision.revision}</strong>
                    <span>{new Date(revision.created_at).toLocaleString()}</span>
                    <p>{revision.change_reason}</p>
                    {revision.ai_provenance && revision.interpretation_note && (
                      <div className="ai-history-comment">
                        <b>AI comment:</b> {revision.interpretation_note}
                      </div>
                    )}
                    <small>
                      {revision.review_status === "reviewed_by_human" ? "✓ reviewed by human" : "not reviewed"}
                      {revision.human_action === "changed" ? " · ◆ coordinates changed by human" : ""}
                      {revision.actor_id ? ` · actor: ${revision.actor_id}` : " · anonymous"}
                      {revision.ai_provenance
                        ? ` · informed by ${revision.ai_provenance.provider}/${revision.ai_provenance.model}`
                        : " · no AI provenance"}
                    </small>
                  </li>
                ))}
              </ol>
            )}
          </section>
        </div>
      </section>
    </>
  );
}
