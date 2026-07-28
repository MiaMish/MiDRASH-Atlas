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

function sameGeometry(left: GeoJSONGeometry | null, right?: GeoJSONGeometry) {
  return JSON.stringify(left) === JSON.stringify(right ?? null);
}

export function LocationEditor() {
  const [locations, setLocations] = useState<LocationOverview[]>([]);
  const [placeId, setPlaceId] = useState("");
  const [geometry, setGeometry] = useState<GeoJSONGeometry | null>(null);
  const [geometryText, setGeometryText] = useState("");
  const [variantId, setVariantId] = useState("");
  const [label, setLabel] = useState("");
  const [precision, setPrecision] = useState("locality");
  const [interpretationNote, setInterpretationNote] = useState("");
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
      missing: locations.filter((item) => !item.has_valid_coordinates).length,
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
    setVariantId(next.geometry_variant_id ?? `modern_${next.place_id}`);
    setLabel(next.geometry_label ?? `Modern display geometry for ${next.place_label}`);
    setPrecision(next.spatial_precision ?? "locality");
    setInterpretationNote(next.interpretation_note ?? "");
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
        geometry_variant_id: variantId,
        label,
        geometry,
        spatial_precision: precision,
        interpretation_source: location.geometry_source ?? "modern_gazetteer_geometry",
        interpretation_note: interpretationNote,
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

  const currentStatus = location ? statusCopy[location.coordinate_status] : statusCopy.no_geometry;
  const geometryChanged = location ? !sameGeometry(geometry, location.geometry) : false;

  return (
    <>
      <section className="location-summary" aria-label="Location coverage">
        <div><strong>{counts.total}</strong><span>place concepts</span></div>
        <div><strong>{counts.mapped}</strong><span>with valid coordinates</span></div>
        <div><strong>{counts.reviewed}</strong><span>human-reviewed</span></div>
        <div><strong>{counts.missing}</strong><span>without coordinates</span></div>
      </section>

      <section className="location-workspace">
        <aside className="location-form">
          <label>
            Place concept
            <select value={placeId} onChange={(event) => setPlaceId(event.target.value)}>
              {locations.map((item) => (
                <option key={item.place_id} value={item.place_id}>
                  {statusCopy[item.coordinate_status].option} {item.place_label} —{" "}
                  {statusCopy[item.coordinate_status].label}
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
            <span><b>—</b> No coordinates</span>
          </div>

          <div className="source-card">
            <strong>{location?.geometry_source?.replaceAll("_", " ") ?? "No geometry source"}</strong>
            <p>
              Modern display geometry is not a claim that the same boundary applied historically.
            </p>
          </div>
          <label>
            Variant ID
            <input value={variantId} onChange={(event) => setVariantId(event.target.value)} />
          </label>
          <label>
            Label
            <input value={label} onChange={(event) => setLabel(event.target.value)} />
          </label>
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
            Interpretation note
            <textarea
              rows={3}
              value={interpretationNote}
              onChange={(event) => setInterpretationNote(event.target.value)}
            />
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
            {history.length === 0 ? (
              <p>No human revisions have been saved for this place.</p>
            ) : (
              <ol className="audit-list">
                {history.map((revision) => (
                  <li key={revision.id}>
                    <strong>{revision.geometry_variant_id} · revision {revision.revision}</strong>
                    <span>{new Date(revision.created_at).toLocaleString()}</span>
                    <p>{revision.change_reason}</p>
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
