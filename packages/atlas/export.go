package atlas

import (
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type ExportConfig struct {
	InputJSON       string
	ManuscriptsCSV  string
	HiburimCSV      string
	SourceTypes     string
	PlaceGeometries string
	OutputDir       string
}

type exportState struct {
	records       []AtlasRecord
	geo           []GeoAssertion
	temporal      []TemporalAssertion
	places        map[string]*PlaceConcept
	review        []ReviewItem
	hiburim       []Hibur
	hiburAliases  map[string]string
	mentionsByMMS map[string][]string
}

func Export(cfg ExportConfig) error {
	inputData, err := os.ReadFile(cfg.InputJSON)
	if err != nil {
		return err
	}
	var input Input
	if err := json.Unmarshal(inputData, &input); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}
	hiburim, aliases, err := loadHiburim(cfg.HiburimCSV)
	if err != nil {
		return err
	}
	mentions, err := loadManuscriptMentions(cfg.ManuscriptsCSV)
	if err != nil {
		return err
	}
	state := &exportState{
		places:        map[string]*PlaceConcept{},
		hiburim:       hiburim,
		hiburAliases:  aliases,
		mentionsByMMS: mentions,
	}
	byID := make(map[string]InputRecord, len(input.Records))
	for _, r := range input.Records {
		byID[r.MMSID] = r
	}
	for _, id := range input.RequestedMMSIDs {
		r, ok := byID[id]
		if !ok {
			state.review = append(state.review, ReviewItem{
				ID: stableID("review", "missing-record", id), Kind: "missing_record",
				RecordID: id, Reason: "Requested MMS ID is absent from the normalized NLI export.",
			})
			continue
		}
		state.addRecord(r, byID)
	}
	if err := state.applyGeometrySeeds(cfg.PlaceGeometries); err != nil {
		return err
	}
	sortOutputs(state)
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return err
	}
	sourceData, err := os.ReadFile(cfg.SourceTypes)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, "source_types.json"), sourceData, 0o644); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.OutputDir, "records.json"), map[string]any{
		"schema_version": "1.0.0", "records": state.records,
	}); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.OutputDir, "hiburim.json"), map[string]any{
		"schema_version": "1.0.0", "hiburim": state.hiburim,
	}); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.OutputDir, "geo_assertions.json"), map[string]any{
		"schema_version": "1.0.0", "assertions": state.geo,
	}); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.OutputDir, "temporal_assertions.json"), map[string]any{
		"schema_version": "1.0.0", "assertions": state.temporal,
	}); err != nil {
		return err
	}
	places := make([]PlaceConcept, 0, len(state.places))
	for _, p := range state.places {
		places = append(places, *p)
	}
	sort.Slice(places, func(i, j int) bool { return places[i].ID < places[j].ID })
	if err := writeJSON(filepath.Join(cfg.OutputDir, "places.json"), map[string]any{
		"schema_version": "1.0.0",
		"model_note":     "A place concept may have multiple time-bounded geometry variants supplied by different interpretive authorities.",
		"places":         places,
	}); err != nil {
		return err
	}
	features := make([]Feature, 0, len(state.geo))
	for _, a := range state.geo {
		features = append(features, Feature{
			Type: "Feature", ID: a.ID, Geometry: json.RawMessage("null"), Properties: a,
		})
	}
	if err := writeJSON(filepath.Join(cfg.OutputDir, "events.geojson"), FeatureCollection{
		Type: "FeatureCollection", Features: features,
	}); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(cfg.OutputDir, "review_queue.json"), map[string]any{
		"schema_version": "1.0.0", "items": state.review,
	}); err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.OutputDir, "manifest.json"), buildManifest(state, places))
}

func (s *exportState) addRecord(r InputRecord, byID map[string]InputRecord) {
	physicalID := r.MMSID
	if len(r.ParentMMSIDs) > 0 {
		physicalID = r.ParentMMSIDs[0]
	}
	out := AtlasRecord{
		ID: r.MMSID, PhysicalID: physicalID, Kind: r.RecordKind,
		ParentIDs: r.ParentMMSIDs, Titles: append(append([]string{}, r.Title...), r.AlternativeTitles...),
		PrimaryTitles: r.Title, AlternativeTitles: r.AlternativeTitles,
		Languages: r.Languages, ScriptStyles: r.ScriptStyles,
		Extent: r.Extent, Dimensions: marcValues(r, "300", "c"),
		Contributors:    marcContributors(r),
		Digitized:       len(r.DigitalObjects)+len(r.DigitalServices) > 0,
		PublicRecordURL: r.PublicRecordURL, ResolverURL: r.ResolverURL,
		SourceModified: r.SourceModified, ProvenanceNotes: r.ProvenanceNotes, ColophonNotes: r.ColophonNotes,
		PhysicalNotes: r.PhysicalNotes, Contents: r.Contents, GeneralNotes: r.GeneralNotes,
	}
	for _, owner := range r.CurrentOwners {
		out.CurrentOwners = append(out.CurrentOwners, Owner{
			Name: owner.Name, Locality: owner.Locality, Country: owner.Country, Roles: owner.Roles,
		})
	}
	for _, parentID := range r.ParentMMSIDs {
		if parent, ok := byID[parentID]; ok {
			out.ParentRecords = append(out.ParentRecords, CatalogRecordSummary{
				ID: parent.MMSID, Kind: parent.RecordKind,
				Titles:        append(append([]string{}, parent.Title...), parent.AlternativeTitles...),
				PrimaryTitles: parent.Title, AlternativeTitles: parent.AlternativeTitles,
			})
		}
	}
	for _, sh := range r.Shelfmarks {
		out.Shelfmarks = append(out.Shelfmarks, Shelfmark{
			Repository: sh.Repository, Locality: sh.Locality, Country: sh.Country, Shelfmark: sh.Shelfmark,
		})
	}
	for _, raw := range s.mentionsByMMS[r.MMSID] {
		key := normalizeLabel(raw)
		link := HiburLink{RawLabel: raw, MatchStatus: "unresolved"}
		if id, ok := s.hiburAliases[key]; ok {
			link.HiburID, link.MatchStatus = id, "exact_alias"
		} else {
			s.review = append(s.review, ReviewItem{
				ID: stableID("review", "hibur", r.MMSID, raw), Kind: "hibur_match",
				Raw: raw, RecordID: r.MMSID, Reason: "No exact ontology alias match.",
			})
		}
		out.HiburLinks = append(out.HiburLinks, link)
	}

	directGeoTypes := geoTypesPresent(r)
	directTemporalTypes := temporalTypesPresent(r)
	s.addGeoForSource(&out, r, r, "direct")
	s.addTemporalForSource(&out, r, r, "direct")
	for _, parentID := range r.ParentMMSIDs {
		parent, ok := byID[parentID]
		if !ok {
			continue
		}
		originFor := func(sourceType string, direct map[string]bool) string {
			if direct[sourceType] {
				return "parent_alternative"
			}
			return "parent_fallback"
		}
		s.addGeoForSourceByOrigin(&out, r, parent, originFor, directGeoTypes)
		s.addTemporalForSourceByOrigin(&out, r, parent, originFor, directTemporalTypes)
	}
	for _, note := range r.ColophonNotes {
		s.review = append(s.review, ReviewItem{
			ID: stableID("review", "colophon", r.MMSID, note), Kind: "colophon_extraction",
			Raw: note, RecordID: r.MMSID,
			Reason: "Review for place, date, scribe, and event candidates before creating assertions.",
		})
	}
	for _, note := range r.ProvenanceNotes {
		s.review = append(s.review, ReviewItem{
			ID: stableID("review", "provenance", r.MMSID, note), Kind: "provenance_extraction",
			Raw: note, RecordID: r.MMSID,
			Reason: "Review for time-ordered ownership and movement events before creating assertions.",
		})
	}
	s.records = append(s.records, out)
}

func marcValues(record InputRecord, tag, code string) []string {
	var values []string
	for _, field := range record.MARC.Fields {
		if field.Tag != tag {
			continue
		}
		for _, subfield := range field.Subfields {
			if subfield.Code == code && strings.TrimSpace(subfield.Value) != "" {
				values = append(values, subfield.Value)
			}
		}
	}
	return values
}

func marcContributors(record InputRecord) []Contributor {
	var contributors []Contributor
	for _, field := range record.MARC.Fields {
		if field.Tag != "100" && field.Tag != "700" {
			continue
		}
		var name, role string
		for _, subfield := range field.Subfields {
			switch subfield.Code {
			case "a":
				if name == "" {
					name = subfield.Value
				}
			case "e":
				if role == "" {
					role = subfield.Value
				}
			}
		}
		if strings.TrimSpace(name) != "" {
			contributors = append(contributors, Contributor{Name: name, Role: role, MARCTag: field.Tag})
		}
	}
	return contributors
}

func (s *exportState) addGeoForSource(out *AtlasRecord, target, source InputRecord, origin string) {
	s.addGeoForSourceByOrigin(out, target, source, func(string, map[string]bool) string { return origin }, nil)
}

func (s *exportState) addGeoForSourceByOrigin(out *AtlasRecord, target, source InputRecord, originFor func(string, map[string]bool) string, direct map[string]bool) {
	add := func(sourceType, raw, role, tag, subfield, confidence string) {
		if strings.TrimSpace(raw) == "" {
			return
		}
		placeID := s.ensurePlace(raw)
		origin := originFor(sourceType, direct)
		id := stableID("geo", target.MMSID, source.MMSID, sourceType, raw)
		a := GeoAssertion{
			ID: id, SourceTypeID: sourceType, PlaceID: placeID, PlaceRaw: raw, Role: role, Confidence: confidence,
			Scope: AssertionScope{TargetRecordID: target.MMSID, SourceRecordID: source.MMSID, PhysicalID: out.PhysicalID, Origin: origin},
			Evidence: Evidence{
				Raw: raw, MARCTag: tag, MARCSubfield: subfield, MARCRole: role,
				Catalog: "National Library of Israel Alma catalogue", CatalogRecordAt: source.SourceModified,
				Extraction: "structured_field", ReviewStatus: "unreviewed",
			},
		}
		s.geo = append(s.geo, a)
		out.GeoAssertionIDs = append(out.GeoAssertionIDs, id)
	}
	for _, p := range source.Places {
		sourceType, confidence := "nli_751_related_place", "medium"
		if p.Role == "place of writing" {
			sourceType, confidence = "nli_751_writing_place", "high"
		}
		add(sourceType, p.Name, p.Role, "751", "a", confidence)
	}
	for _, raw := range source.ProductionPlaceDisplay {
		add("nli_260_production_place", raw, "production place display", "260", "a", "medium")
	}
	for _, owner := range source.CurrentOwners {
		raw := strings.TrimSpace(strings.Join(nonempty(owner.Locality, owner.Country), ", "))
		if raw != "" {
			add("nli_current_repository", raw, "current repository", "710/942", "", "high")
		}
	}
}

func (s *exportState) addTemporalForSource(out *AtlasRecord, target, source InputRecord, origin string) {
	s.addTemporalForSourceByOrigin(out, target, source, func(string, map[string]bool) string { return origin }, nil)
}

func (s *exportState) addTemporalForSourceByOrigin(out *AtlasRecord, target, source InputRecord, originFor func(string, map[string]bool) string, direct map[string]bool) {
	add := func(sourceType, raw, tag, subfield, extraction string, value TemporalValue) {
		if strings.TrimSpace(raw) == "" {
			return
		}
		origin := originFor(sourceType, direct)
		id := stableID("time", target.MMSID, source.MMSID, sourceType, raw)
		confidence := "high"
		if value.Uncertain || value.Kind == "unparsed" {
			confidence = "low"
		} else if value.Approximate || strings.Contains(value.Kind, "century") {
			confidence = "medium"
		}
		a := TemporalAssertion{
			ID: id, SourceTypeID: sourceType, Value: value, Confidence: confidence,
			Scope: AssertionScope{TargetRecordID: target.MMSID, SourceRecordID: source.MMSID, PhysicalID: out.PhysicalID, Origin: origin},
			Evidence: Evidence{
				Raw: raw, MARCTag: tag, MARCSubfield: subfield,
				Catalog: "National Library of Israel Alma catalogue", CatalogRecordAt: source.SourceModified,
				Extraction: extraction, ReviewStatus: "unreviewed",
			},
		}
		s.temporal = append(s.temporal, a)
		out.TemporalAssertionIDs = append(out.TemporalAssertionIDs, id)
		if value.Kind == "unparsed" {
			s.review = append(s.review, ReviewItem{
				ID: stableID("review", "date", target.MMSID, source.MMSID, raw), Kind: "date_parse",
				Raw: raw, RecordID: target.MMSID, Reason: "Date parser did not produce a structured value.",
			})
		}
	}
	for _, raw := range source.DateDisplay {
		add("nli_260_date", raw, "260", "c", "deterministic_parser", ParseDate(raw))
	}
	if raw, value, ok := parse008(source.FixedField008); ok {
		add("nli_008_date_candidate", raw, "008", "", "fixed_field_parser", value)
	}
}

func parse008(raw string) (string, TemporalValue, bool) {
	if len(raw) < 11 {
		return "", TemporalValue{}, false
	}
	year := raw[7:11]
	for _, r := range year {
		if !unicode.IsDigit(r) {
			return "", TemporalValue{}, false
		}
	}
	var y int
	fmt.Sscanf(year, "%d", &y)
	return year, TemporalValue{Kind: "year", Year: intp(y)}, true
}

func geoTypesPresent(r InputRecord) map[string]bool {
	m := map[string]bool{}
	for _, p := range r.Places {
		if p.Role == "place of writing" {
			m["nli_751_writing_place"] = true
		} else {
			m["nli_751_related_place"] = true
		}
	}
	if len(r.ProductionPlaceDisplay) > 0 {
		m["nli_260_production_place"] = true
	}
	if len(r.CurrentOwners) > 0 {
		m["nli_current_repository"] = true
	}
	return m
}

func temporalTypesPresent(r InputRecord) map[string]bool {
	m := map[string]bool{}
	if len(r.DateDisplay) > 0 {
		m["nli_260_date"] = true
	}
	if _, _, ok := parse008(r.FixedField008); ok {
		m["nli_008_date_candidate"] = true
	}
	return m
}

func (s *exportState) ensurePlace(raw string) string {
	key := normalizeLabel(raw)
	id := stableID("place", key)
	if _, ok := s.places[id]; !ok {
		s.places[id] = &PlaceConcept{
			ID: id, PreferredLabel: strings.TrimSpace(raw), Aliases: []string{strings.TrimSpace(raw)},
			GeometryVariants: []GeometryVariant{}, ReviewStatus: "needs_geometry_review",
		}
	}
	return id
}

func (s *exportState) applyGeometrySeeds(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document GeometrySeedDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("decode place geometries: %w", err)
	}
	for _, seed := range document.Places {
		id := stableID("place", normalizeLabel(seed.PlaceLabel))
		p, ok := s.places[id]
		if !ok {
			s.review = append(s.review, ReviewItem{
				ID:   stableID("review", "geometry-place", seed.PlaceLabel),
				Kind: "geometry_place_match", Raw: seed.PlaceLabel,
				Reason: "Curated geometry label does not match an exported place concept.",
			})
			continue
		}
		for _, alias := range seed.Aliases {
			p.Aliases = appendUnique(p.Aliases, alias)
		}
		p.GeometryVariants = append(p.GeometryVariants, seed.Variants...)
		if len(p.GeometryVariants) > 0 {
			p.ReviewStatus = "has_geometry_candidates"
		}
	}
	return nil
}

func loadHiburim(path string) ([]Hibur, map[string]string, error) {
	rows, headers, err := readCSV(path)
	if err != nil {
		return nil, nil, err
	}
	var out []Hibur
	aliases := map[string]string{}
	for _, row := range rows {
		get := func(name string) string {
			i := indexOf(headers, name)
			if i < 0 || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		label := firstNonempty(get("אונטולוגיה"), get("מתוך טבלת רישום מדרשים"), get("פרוייקט השו\"ת"), get("Common English Title"), get("Transcribed Title"))
		if label == "" {
			continue
		}
		id := get("ID חדש?")
		if id == "" {
			id = stableID("hibur", label)
		}
		var names []string
		for i, value := range row {
			if i >= len(headers) || strings.HasPrefix(headers[i], "ID") || strings.Contains(headers[i], "מספר") {
				continue
			}
			value = strings.TrimSpace(value)
			if value == "" || strings.HasPrefix(value, "<") || isNumeric(value) {
				continue
			}
			names = appendUnique(names, value)
		}
		families := splitTrim(get("Family"), ",")
		h := Hibur{ID: id, Label: label, English: get("Common English Title"), Family: families, Aliases: names, OntologyKey: get("אונטולוגיה")}
		out = append(out, h)
		for _, name := range names {
			key := normalizeLabel(name)
			if key != "" {
				if _, exists := aliases[key]; !exists {
					aliases[key] = id
				}
			}
		}
	}
	return out, aliases, nil
}

func loadManuscriptMentions(path string) (map[string][]string, error) {
	rows, headers, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	idIdx, hIdx := indexOf(headers, "מספר מערכת"), indexOf(headers, "חיבורים - טור זמני")
	for _, row := range rows {
		if idIdx < 0 || hIdx < 0 || idIdx >= len(row) || hIdx >= len(row) {
			continue
		}
		id := strings.TrimSpace(row[idIdx])
		for _, raw := range splitTrim(row[hIdx], ",") {
			out[id] = appendUnique(out[id], raw)
		}
	}
	return out, nil
}

func readCSV(path string) ([][]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	headers, err := r.Read()
	if err != nil {
		return nil, nil, err
	}
	var rows [][]string
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if len(row) < len(headers) {
			row = append(row, make([]string, len(headers)-len(row))...)
		}
		rows = append(rows, row)
	}
	return rows, headers, nil
}

func indexOf(headers []string, name string) int {
	for i, h := range headers {
		if strings.TrimSpace(h) == strings.TrimSpace(name) {
			return i
		}
	}
	return -1
}

func stableID(parts ...string) string {
	h := sha1.Sum([]byte(strings.Join(parts, "\x1f")))
	return parts[0] + "_" + hex.EncodeToString(h[:])[:12]
}

func normalizeLabel(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.Trim(s, ".;: ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func appendUnique(xs []string, x string) []string {
	for _, existing := range xs {
		if existing == x {
			return xs
		}
	}
	return append(xs, x)
}

func splitTrim(s, sep string) []string {
	var out []string
	for _, item := range strings.Split(s, sep) {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func nonempty(xs ...string) []string {
	var out []string
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			out = append(out, strings.TrimSpace(x))
		}
	}
	return out
}

func firstNonempty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return strings.TrimSpace(x)
		}
	}
	return ""
}

func isNumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) && r != '.' {
			return false
		}
	}
	return s != ""
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func sortOutputs(s *exportState) {
	sort.Slice(s.records, func(i, j int) bool { return s.records[i].ID < s.records[j].ID })
	sort.Slice(s.geo, func(i, j int) bool { return s.geo[i].ID < s.geo[j].ID })
	sort.Slice(s.temporal, func(i, j int) bool { return s.temporal[i].ID < s.temporal[j].ID })
	sort.Slice(s.review, func(i, j int) bool { return s.review[i].ID < s.review[j].ID })
	sort.Slice(s.hiburim, func(i, j int) bool { return s.hiburim[i].ID < s.hiburim[j].ID })
}

func buildManifest(s *exportState, places []PlaceConcept) map[string]any {
	geoByType, timeByType, originCounts, reviewByKind := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	for _, a := range s.geo {
		geoByType[a.SourceTypeID]++
		originCounts["geo:"+a.Scope.Origin]++
	}
	unparsed := 0
	for _, a := range s.temporal {
		timeByType[a.SourceTypeID]++
		originCounts["temporal:"+a.Scope.Origin]++
		if a.Value.Kind == "unparsed" {
			unparsed++
		}
	}
	for _, r := range s.review {
		reviewByKind[r.Kind]++
	}
	geometryVariants := 0
	for _, p := range places {
		geometryVariants += len(p.GeometryVariants)
	}
	return map[string]any{
		"schema_version": "1.0.0",
		"generation_provenance": map[string]any{
			"method":  "deterministic_go_export",
			"ai_used": false,
		},
		"counts": map[string]any{
			"records": len(s.records), "hiburim": len(s.hiburim), "places": len(places),
			"geo_assertions": len(s.geo), "temporal_assertions": len(s.temporal),
			"geometry_variants": geometryVariants, "unparsed_dates": unparsed, "review_items": len(s.review),
		},
		"geo_by_source_type": geoByType, "temporal_by_source_type": timeByType,
		"assertions_by_origin": originCounts, "review_by_kind": reviewByKind,
	}
}
