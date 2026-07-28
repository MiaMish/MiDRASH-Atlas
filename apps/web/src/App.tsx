import { useState } from "react";
import { AtlasView } from "./features/atlas/AtlasView";
import { LocationEditor } from "./features/locations/LocationEditor";

type Workspace = "atlas" | "locations";

export function App() {
  const [workspace, setWorkspace] = useState<Workspace>("atlas");
  return (
    <main className="app-shell">
      <header className="app-header atlas-header">
        <div>
          <p className="eyebrow">Midrash Atlas · research workspace</p>
          <h1>{workspace === "atlas" ? "Manuscript atlas" : "Location curation"}</h1>
        </div>
        <div className="header-side">
          <nav className="workspace-tabs" aria-label="Workspace">
            <button
              type="button"
              className={workspace === "atlas" ? "active" : ""}
              aria-pressed={workspace === "atlas"}
              onClick={() => setWorkspace("atlas")}
            >
              Atlas
            </button>
            <button
              type="button"
              className={workspace === "locations" ? "active" : ""}
              aria-pressed={workspace === "locations"}
              onClick={() => setWorkspace("locations")}
            >
              Location curation
            </button>
          </nav>
          <p className="header-note">
            {workspace === "atlas"
              ? "Explore where manuscript evidence appears across time, sources, and linked works."
              : "Review modern display geometry without collapsing historical interpretations."}
          </p>
        </div>
      </header>
      {workspace === "atlas" ? <AtlasView /> : <LocationEditor />}
    </main>
  );
}
