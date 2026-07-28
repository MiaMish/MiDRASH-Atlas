import { LocationEditor } from "./features/locations/LocationEditor";

export function App() {
  return (
    <main className="app-shell">
      <header className="app-header">
        <div>
          <p className="eyebrow">Midrash Atlas · research workspace</p>
          <h1>Location curation</h1>
        </div>
        <p className="header-note">
          Review modern display geometry without collapsing historical interpretations.
        </p>
      </header>
      <LocationEditor />
    </main>
  );
}
