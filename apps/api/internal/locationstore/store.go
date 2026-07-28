package locationstore

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"midrash-atlas/packages/provenance"

	_ "modernc.org/sqlite"
)

type Store interface {
	SaveRevision(context.Context, SaveRevision) (GeometryRevision, error)
	ListRevisions(context.Context, string, string) ([]GeometryRevision, error)
	ListCurrent(context.Context, string) ([]GeometryRevision, error)
	ListCurrentAll(context.Context) ([]GeometryRevision, error)
	Close() error
}

type SaveRevision struct {
	PlaceID              string            `json:"place_id"`
	GeometryVariantID    string            `json:"geometry_variant_id"`
	Label                string            `json:"label"`
	Geometry             json.RawMessage   `json:"geometry"`
	ValidFromYear        *int              `json:"valid_from_year,omitempty"`
	ValidToYear          *int              `json:"valid_to_year,omitempty"`
	SpatialPrecision     string            `json:"spatial_precision"`
	InterpretationSource string            `json:"interpretation_source"`
	InterpretationNote   string            `json:"interpretation_note,omitempty"`
	Confidence           string            `json:"confidence"`
	ReviewStatus         string            `json:"review_status"`
	HumanAction          string            `json:"human_action,omitempty"`
	ChangeReason         string            `json:"change_reason"`
	ActorID              *string           `json:"actor_id,omitempty"`
	Source               string            `json:"source,omitempty"`
	AIProvenance         *provenance.AIRun `json:"ai_provenance,omitempty"`
}

type GeometryRevision struct {
	ID                   string            `json:"id"`
	PlaceID              string            `json:"place_id"`
	GeometryVariantID    string            `json:"geometry_variant_id"`
	Revision             int               `json:"revision"`
	SupersedesRevisionID *string           `json:"supersedes_revision_id,omitempty"`
	Label                string            `json:"label"`
	Geometry             json.RawMessage   `json:"geometry"`
	ValidFromYear        *int              `json:"valid_from_year,omitempty"`
	ValidToYear          *int              `json:"valid_to_year,omitempty"`
	SpatialPrecision     string            `json:"spatial_precision"`
	InterpretationSource string            `json:"interpretation_source"`
	InterpretationNote   string            `json:"interpretation_note,omitempty"`
	Confidence           string            `json:"confidence"`
	ReviewStatus         string            `json:"review_status"`
	HumanAction          string            `json:"human_action,omitempty"`
	ChangeReason         string            `json:"change_reason"`
	ActorID              *string           `json:"actor_id,omitempty"`
	Source               string            `json:"source"`
	AIProvenance         *provenance.AIRun `json:"ai_provenance,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
}

type SQLite struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLite, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &SQLite{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLite) Close() error { return s.db.Close() }

func (s *SQLite) migrate() error {
	_, err := s.db.Exec(`
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS place_geometry_revisions (
  id TEXT PRIMARY KEY,
  place_id TEXT NOT NULL,
  geometry_variant_id TEXT NOT NULL,
  revision INTEGER NOT NULL,
  supersedes_revision_id TEXT NULL,
  label TEXT NOT NULL,
  geometry_json TEXT NOT NULL,
  valid_from_year INTEGER NULL,
  valid_to_year INTEGER NULL,
  spatial_precision TEXT NOT NULL,
  interpretation_source TEXT NOT NULL,
  interpretation_note TEXT NOT NULL DEFAULT '',
  confidence TEXT NOT NULL,
  review_status TEXT NOT NULL,
  human_action TEXT NOT NULL DEFAULT '',
  change_reason TEXT NOT NULL,
  actor_id TEXT NULL,
  source TEXT NOT NULL,
  ai_provider TEXT NOT NULL DEFAULT '',
  ai_model TEXT NOT NULL DEFAULT '',
  ai_generated_at TEXT NOT NULL DEFAULT '',
  ai_purpose TEXT NOT NULL DEFAULT '',
  curation_draft_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  UNIQUE (place_id, geometry_variant_id, revision),
  FOREIGN KEY (supersedes_revision_id) REFERENCES place_geometry_revisions(id)
);
CREATE INDEX IF NOT EXISTS idx_geometry_revision_lookup
  ON place_geometry_revisions(place_id, geometry_variant_id, revision DESC);
CREATE INDEX IF NOT EXISTS idx_geometry_revision_created
  ON place_geometry_revisions(created_at DESC);
`)
	if err != nil {
		return err
	}
	for _, column := range []struct {
		name       string
		definition string
	}{
		{"ai_provider", "TEXT NOT NULL DEFAULT ''"},
		{"ai_model", "TEXT NOT NULL DEFAULT ''"},
		{"ai_generated_at", "TEXT NOT NULL DEFAULT ''"},
		{"ai_purpose", "TEXT NOT NULL DEFAULT ''"},
		{"human_action", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := s.ensureColumn("place_geometry_revisions", column.name, column.definition); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLite) SaveRevision(ctx context.Context, input SaveRevision) (GeometryRevision, error) {
	if err := validate(input); err != nil {
		return GeometryRevision{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GeometryRevision{}, err
	}
	defer tx.Rollback()
	var revision int
	var supersedes sql.NullString
	err = tx.QueryRowContext(ctx, `
SELECT revision, id
FROM place_geometry_revisions
WHERE place_id = ? AND geometry_variant_id = ?
ORDER BY revision DESC
LIMIT 1`, input.PlaceID, input.GeometryVariantID).Scan(&revision, &supersedes)
	if err != nil && err != sql.ErrNoRows {
		return GeometryRevision{}, err
	}
	revision++
	now := time.Now().UTC()
	id, err := newID("grev")
	if err != nil {
		return GeometryRevision{}, err
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "ui"
	}
	var supersedesValue any
	if supersedes.Valid {
		supersedesValue = supersedes.String
	}
	aiProvider, aiModel, aiGeneratedAt, aiPurpose, aiPromptHash := "", "", "", "", ""
	if input.AIProvenance != nil {
		if err := input.AIProvenance.Validate(); err != nil {
			return GeometryRevision{}, err
		}
		aiProvider = input.AIProvenance.Provider
		aiModel = input.AIProvenance.Model
		aiGeneratedAt = input.AIProvenance.GeneratedAt.UTC().Format(time.RFC3339Nano)
		aiPurpose = input.AIProvenance.Purpose
		aiPromptHash = input.AIProvenance.PromptHash
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO place_geometry_revisions (
  id, place_id, geometry_variant_id, revision, supersedes_revision_id,
  label, geometry_json, valid_from_year, valid_to_year, spatial_precision,
  interpretation_source, interpretation_note, confidence, review_status,
  human_action, change_reason, actor_id, source, ai_provider, ai_model, ai_generated_at,
  ai_purpose, curation_draft_hash, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, input.PlaceID, input.GeometryVariantID, revision, supersedesValue,
		input.Label, string(input.Geometry), input.ValidFromYear, input.ValidToYear,
		input.SpatialPrecision, input.InterpretationSource, input.InterpretationNote,
		input.Confidence, input.ReviewStatus, input.HumanAction, input.ChangeReason, input.ActorID,
		source, aiProvider, aiModel, aiGeneratedAt, aiPurpose, aiPromptHash,
		now.Format(time.RFC3339Nano))
	if err != nil {
		return GeometryRevision{}, err
	}
	if err := tx.Commit(); err != nil {
		return GeometryRevision{}, err
	}
	var supersedesPtr *string
	if supersedes.Valid {
		value := supersedes.String
		supersedesPtr = &value
	}
	return GeometryRevision{
		ID: id, PlaceID: input.PlaceID, GeometryVariantID: input.GeometryVariantID,
		Revision: revision, SupersedesRevisionID: supersedesPtr, Label: input.Label,
		Geometry: input.Geometry, ValidFromYear: input.ValidFromYear, ValidToYear: input.ValidToYear,
		SpatialPrecision: input.SpatialPrecision, InterpretationSource: input.InterpretationSource,
		InterpretationNote: input.InterpretationNote, Confidence: input.Confidence,
		ReviewStatus: input.ReviewStatus, HumanAction: input.HumanAction, ChangeReason: input.ChangeReason,
		ActorID: input.ActorID, Source: source, AIProvenance: input.AIProvenance,
		CreatedAt: now,
	}, nil
}

func (s *SQLite) ListRevisions(ctx context.Context, placeID, variantID string) ([]GeometryRevision, error) {
	query := `SELECT id, place_id, geometry_variant_id, revision, supersedes_revision_id,
label, geometry_json, valid_from_year, valid_to_year, spatial_precision,
interpretation_source, interpretation_note, confidence, review_status,
human_action, change_reason, actor_id, source, ai_provider, ai_model, ai_generated_at,
ai_purpose, curation_draft_hash, created_at
FROM place_geometry_revisions WHERE place_id = ?`
	args := []any{placeID}
	if strings.TrimSpace(variantID) != "" {
		query += ` AND geometry_variant_id = ?`
		args = append(args, variantID)
	}
	query += ` ORDER BY geometry_variant_id, revision DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func (s *SQLite) ListCurrent(ctx context.Context, placeID string) ([]GeometryRevision, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id, r.place_id, r.geometry_variant_id, r.revision, r.supersedes_revision_id,
r.label, r.geometry_json, r.valid_from_year, r.valid_to_year, r.spatial_precision,
r.interpretation_source, r.interpretation_note, r.confidence, r.review_status,
r.human_action, r.change_reason, r.actor_id, r.source, r.ai_provider, r.ai_model,
r.ai_generated_at, r.ai_purpose, r.curation_draft_hash, r.created_at
FROM place_geometry_revisions r
JOIN (
  SELECT place_id, geometry_variant_id, MAX(revision) AS revision
  FROM place_geometry_revisions
  WHERE place_id = ?
  GROUP BY place_id, geometry_variant_id
) latest
ON r.place_id = latest.place_id
AND r.geometry_variant_id = latest.geometry_variant_id
AND r.revision = latest.revision
ORDER BY r.geometry_variant_id`, placeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func (s *SQLite) ListCurrentAll(ctx context.Context) ([]GeometryRevision, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id, r.place_id, r.geometry_variant_id, r.revision, r.supersedes_revision_id,
r.label, r.geometry_json, r.valid_from_year, r.valid_to_year, r.spatial_precision,
r.interpretation_source, r.interpretation_note, r.confidence, r.review_status,
r.human_action, r.change_reason, r.actor_id, r.source, r.ai_provider, r.ai_model,
r.ai_generated_at, r.ai_purpose, r.curation_draft_hash, r.created_at
FROM place_geometry_revisions r
JOIN (
  SELECT place_id, geometry_variant_id, MAX(revision) AS revision
  FROM place_geometry_revisions
  GROUP BY place_id, geometry_variant_id
) latest
ON r.place_id = latest.place_id
AND r.geometry_variant_id = latest.geometry_variant_id
AND r.revision = latest.revision
ORDER BY r.place_id, r.geometry_variant_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

type rowScanner interface {
	Scan(...any) error
}

func scanRows(rows *sql.Rows) ([]GeometryRevision, error) {
	var out []GeometryRevision
	for rows.Next() {
		item, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanRevision(row rowScanner) (GeometryRevision, error) {
	var item GeometryRevision
	var geometry, created string
	var supersedes, actor sql.NullString
	var aiProvider, aiModel, aiGeneratedAt, aiPurpose, aiPromptHash string
	var validFrom, validTo sql.NullInt64
	err := row.Scan(
		&item.ID, &item.PlaceID, &item.GeometryVariantID, &item.Revision, &supersedes,
		&item.Label, &geometry, &validFrom, &validTo, &item.SpatialPrecision,
		&item.InterpretationSource, &item.InterpretationNote, &item.Confidence,
		&item.ReviewStatus, &item.HumanAction, &item.ChangeReason, &actor, &item.Source,
		&aiProvider, &aiModel, &aiGeneratedAt, &aiPurpose, &aiPromptHash, &created,
	)
	if err != nil {
		return GeometryRevision{}, err
	}
	item.Geometry = json.RawMessage(geometry)
	if supersedes.Valid {
		item.SupersedesRevisionID = &supersedes.String
	}
	if actor.Valid {
		item.ActorID = &actor.String
	}
	if validFrom.Valid {
		value := int(validFrom.Int64)
		item.ValidFromYear = &value
	}
	if validTo.Valid {
		value := int(validTo.Int64)
		item.ValidToYear = &value
	}
	if aiProvider != "" {
		generatedAt, parseErr := time.Parse(time.RFC3339Nano, aiGeneratedAt)
		if parseErr != nil {
			return GeometryRevision{}, parseErr
		}
		item.AIProvenance = &provenance.AIRun{
			Provider: aiProvider, Model: aiModel, GeneratedAt: generatedAt,
			Purpose: aiPurpose, PromptHash: aiPromptHash,
		}
	}
	item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	return item, err
}

func (s *SQLite) ensureColumn(table, name, definition string) error {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var columnName, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if columnName == name {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec("ALTER TABLE " + table + " ADD COLUMN " + name + " " + definition)
	return err
}

func validate(input SaveRevision) error {
	required := map[string]string{
		"place_id": input.PlaceID, "geometry_variant_id": input.GeometryVariantID,
		"label": input.Label, "spatial_precision": input.SpatialPrecision,
		"interpretation_source": input.InterpretationSource, "confidence": input.Confidence,
		"review_status": input.ReviewStatus, "change_reason": input.ChangeReason,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if input.ValidFromYear != nil && input.ValidToYear != nil && *input.ValidFromYear > *input.ValidToYear {
		return fmt.Errorf("valid_from_year cannot be after valid_to_year")
	}
	switch input.HumanAction {
	case "", "draft", "reviewed", "changed":
	default:
		return fmt.Errorf("human_action must be draft, reviewed, or changed")
	}
	var geometry struct {
		Type        string `json:"type"`
		Coordinates any    `json:"coordinates"`
	}
	if err := json.Unmarshal(input.Geometry, &geometry); err != nil {
		return fmt.Errorf("geometry must be valid GeoJSON geometry: %w", err)
	}
	switch geometry.Type {
	case "Point":
		if !validPosition(geometry.Coordinates) {
			return fmt.Errorf("Point coordinates must be finite [longitude, latitude] within world bounds")
		}
	case "Polygon":
		if !validCoordinateTree(geometry.Coordinates, 2) {
			return fmt.Errorf("Polygon coordinates are empty, malformed, or outside world bounds")
		}
	case "MultiPolygon":
		if !validCoordinateTree(geometry.Coordinates, 3) {
			return fmt.Errorf("MultiPolygon coordinates are empty, malformed, or outside world bounds")
		}
	default:
		return fmt.Errorf("unsupported geometry type %q", geometry.Type)
	}
	return nil
}

func validCoordinateTree(value any, depth int) bool {
	if depth == 0 {
		return validPosition(value)
	}
	values, ok := value.([]any)
	if !ok || len(values) == 0 {
		return false
	}
	for _, child := range values {
		if !validCoordinateTree(child, depth-1) {
			return false
		}
	}
	return true
}

func validPosition(value any) bool {
	position, ok := value.([]any)
	if !ok || len(position) < 2 {
		return false
	}
	longitude, lonOK := position[0].(float64)
	latitude, latOK := position[1].(float64)
	return lonOK && latOK && !math.IsNaN(longitude) && !math.IsInf(longitude, 0) &&
		!math.IsNaN(latitude) && !math.IsInf(latitude, 0) &&
		longitude >= -180 && longitude <= 180 && latitude >= -90 && latitude <= 90
}

func newID(prefix string) (string, error) {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(bytes[:]), nil
}
