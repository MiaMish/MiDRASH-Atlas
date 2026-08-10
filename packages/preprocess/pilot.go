package preprocess

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"midrash-atlas/packages/canonical"
)

const (
	pilotWorkID = "1.15:1:1.0"
	pilotItemID = "990001967550205171"
)

type nliNormalizedDocument struct {
	Records []nliNormalizedRecord `json:"records"`
}

type nliNormalizedRecord struct {
	MMSID                  string                   `json:"mms_id"`
	RecordKind             string                   `json:"record_kind"`
	ParentMMSIDs           []string                 `json:"parent_mms_ids"`
	Title                  []string                 `json:"title"`
	AlternativeTitles      []string                 `json:"alternative_titles"`
	DateDisplay            []string                 `json:"date_display"`
	ProductionPlaceDisplay []string                 `json:"production_place_display"`
	Places                 []nliNormalizedPlace     `json:"places"`
	Extent                 []string                 `json:"extent"`
	Languages              []string                 `json:"languages"`
	ScriptStyles           []string                 `json:"script_styles"`
	PhysicalNotes          []string                 `json:"physical_notes"`
	Contents               []string                 `json:"contents"`
	GeneralNotes           []string                 `json:"general_notes"`
	ProvenanceNotes        []string                 `json:"provenance_notes"`
	ColophonNotes          []string                 `json:"colophon_notes"`
	CurrentOwners          []nliNormalizedOwner     `json:"current_owners"`
	Shelfmarks             []nliNormalizedShelfmark `json:"shelfmarks"`
	CatalogReferences      []nliCatalogReference    `json:"catalog_references"`
	Rights                 []nliFieldObject         `json:"rights"`
	DigitalServices        []nliFieldObject         `json:"digital_services"`
	ResolverURL            string                   `json:"resolver_url"`
	PublicRecordURL        string                   `json:"public_record_url"`
	SourceModified         string                   `json:"source_modified"`
}

type nliNormalizedPlace struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Language string `json:"language"`
}
type nliNormalizedOwner struct {
	Name     string   `json:"name"`
	Locality string   `json:"locality"`
	Country  string   `json:"country"`
	Language string   `json:"language"`
	Roles    []string `json:"roles"`
}
type nliNormalizedShelfmark struct {
	Repository string `json:"repository"`
	Locality   string `json:"locality"`
	Country    string `json:"country"`
	Shelfmark  string `json:"shelfmark"`
	Language   string `json:"language"`
}
type nliCatalogReference struct {
	Catalog     string `json:"catalog"`
	Entry       string `json:"entry"`
	AuthorityID string `json:"authority_id"`
}
type nliFieldObject struct {
	Subfields []nliSubfield `json:"subfields"`
}
type nliSubfield struct {
	Code  string `json:"code"`
	Value string `json:"value"`
}

type pilotPlaceSeedDocument struct {
	Places []pilotPlaceSeed `json:"places"`
}

type pilotPlaceSeed struct {
	Label            string  `json:"label"`
	VariantID        string  `json:"variant_id"`
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	SpatialPrecision string  `json:"spatial_precision"`
	ReviewStatus     string  `json:"review_status"`
	Note             string  `json:"note"`
}

// BuildCorpus creates the application-ready atlas catalogue while retaining
// the hand-structured Vatican ebr. 44 / Midrash Proverbs featured slice.
func BuildCorpus(cfg Config, nliNormalizedPath string) (canonical.Document, error) {
	decisions, err := readDecisions(cfg.DecisionsJSON)
	if err != nil {
		return canonical.Document{}, err
	}
	ontologyRows, _, err := readSource("hiburim", cfg.HiburimCSV)
	if err != nil {
		return canonical.Document{}, err
	}
	idStubs := map[int]string{}
	if cfg.HiburIDStubsCSV != "" {
		rows, _, err := readSource("generated_hibur_stub_ids", cfg.HiburIDStubsCSV)
		if err != nil {
			return canonical.Document{}, err
		}
		throwaway := Report{Metrics: map[string]int{}}
		idStubs = validateOntologyIDStubs(rows, &throwaway)
		if throwaway.HasErrors() {
			return canonical.Document{}, fmt.Errorf("generated hibur stub IDs are invalid")
		}
	}
	throwaway := Report{Metrics: map[string]int{}}
	_, index := validateOntology(ontologyRows, idStubs, decisions, &throwaway)
	for _, merged := range decisions.MergedInterpretations {
		if merged.Resolution == "preserve_as_multi_work_assertion" {
			index.approvedMergedAssertions[normalize(merged.Label)] = true
		}
	}

	manuscriptRows, _, err := readSource("manuscripts", cfg.ManuscriptsCSV)
	if err != nil {
		return canonical.Document{}, err
	}
	manuscriptRow, ok := findRow(manuscriptRows, "מספר מערכת", pilotItemID)
	if !ok {
		return canonical.Document{}, fmt.Errorf("pilot manuscript %s not found", pilotItemID)
	}

	workRowsByID := map[string]csvRow{}
	workLinks := make([]canonical.WorkLink, 0)
	for _, rawLabel := range splitLabels(value(manuscriptRow, "חיבורים - טור זמני")) {
		match := index.resolve(rawLabel)
		if match.kind != matchExact && match.kind != matchAlias {
			return canonical.Document{}, fmt.Errorf("pilot manuscript work %q resolves as %s", rawLabel, match.kind)
		}
		candidate := match.candidates[0]
		row, ok := rowByNumber(ontologyRows, candidate.row)
		if !ok {
			return canonical.Document{}, fmt.Errorf("ontology row %d not found for %q", candidate.row, rawLabel)
		}
		workRowsByID[candidate.id] = row
		workLinks = append(workLinks, canonical.WorkLink{
			WorkID: candidate.id, Relation: "contains_work", RawLabel: rawLabel,
			SourceRef: fmt.Sprintf("manuscripts:row:%d", manuscriptRow.number),
		})
	}

	workIDs := make([]string, 0, len(workRowsByID))
	for id := range workRowsByID {
		workIDs = append(workIDs, id)
	}
	sort.Strings(workIDs)
	works := make([]canonical.Work, 0, len(workIDs))
	for _, id := range workIDs {
		works = append(works, canonicalWork(workRowsByID[id], id))
	}

	creationRows, _, err := readSource("work_creation", cfg.CreationCSV)
	if err != nil {
		return canonical.Document{}, err
	}
	creationRow, ok := findRow(creationRows, "ID מערכת <רשימת חיבורים מדרשיים>", pilotWorkID)
	if !ok {
		return canonical.Document{}, fmt.Errorf("pilot work creation row %s not found", pilotWorkID)
	}

	editionRows, _, err := readSource("printed_editions", cfg.EditionsCSV)
	if err != nil {
		return canonical.Document{}, err
	}
	var pilotEditions []csvRow
	for _, row := range editionRows {
		if value(row, "קוד יצירה") == pilotWorkID {
			pilotEditions = append(pilotEditions, row)
		}
	}
	if len(pilotEditions) == 0 {
		return canonical.Document{}, fmt.Errorf("no printed editions found for pilot work %s", pilotWorkID)
	}

	nliRecord, err := readPilotNLIRecord(nliNormalizedPath)
	if err != nil {
		return canonical.Document{}, err
	}
	placeSeeds, err := readPilotPlaceSeeds(cfg.PilotPlacesJSON)
	if err != nil {
		return canonical.Document{}, err
	}

	sources := []canonical.Source{
		{ID: "source-hibur-ontology", Type: "csv", Label: "Hiburim ontology", Citation: cfg.HiburimCSV},
		{ID: "source-midrash-manuscripts", Type: "csv", Label: "Midrash manuscript list", Citation: cfg.ManuscriptsCSV},
		{ID: "source-work-creation", Type: "csv", Label: "Mayim work-creation narratives", Citation: cfg.CreationCSV, URL: value(creationRow, "URL")},
		{ID: "source-printed-editions", Type: "csv", Label: "Mayim printed editions", Citation: cfg.EditionsCSV},
		{ID: "source-nli-normalized", Type: "normalized_marc", Label: "Normalized NLI manuscript record", Citation: nliNormalizedPath, URL: nliRecord.PublicRecordURL},
		{ID: "source-pilot-place-seeds", Type: "curated_display_config", Label: "Unreviewed pilot display positions", Citation: cfg.PilotPlacesJSON},
	}

	manuscriptEvidence := []canonical.Evidence{
		evidence("source-midrash-manuscripts", manuscriptRow, "כינוי כתב היד בכתיב", value(manuscriptRow, "כינוי כתב היד בכתיב"), "direct_csv_field"),
		evidence("source-nli-normalized", csvRow{number: 0}, "mms_id", nliRecord.MMSID, "normalized_marc_field"),
	}
	manuscript := canonical.Item{
		ID: pilotItemID, Layer: canonical.LayerItem, Kind: canonical.ItemManuscript,
		Title: "Vatican Library, Ms. ebr. 44", WorkLinks: workLinks,
		Identifiers: []canonical.Identifier{
			{Scheme: "nli_mms_id", Value: pilotItemID},
			{Scheme: "midrash_mss_id", Value: value(manuscriptRow, "Midrash_MSS ID")},
			{Scheme: "shelfmark", Value: "Vatican Library, Ms. ebr. 44"},
		},
		Evidence: manuscriptEvidence,
		Parts:    pilotManuscriptParts(),
		Attributes: []canonical.ItemAttribute{
			itemAttribute("extent", "Extent", nliRecord.Extent, "source-nli-normalized", pilotItemID, "300$a", "normalized_marc_field"),
			itemAttribute("languages", "Languages", nliRecord.Languages, "source-nli-normalized", pilotItemID, "041$a", "normalized_marc_field"),
			itemAttribute("script_styles", "Script styles", nliRecord.ScriptStyles, "source-nli-normalized", pilotItemID, "958$a", "normalized_marc_field"),
			itemAttribute("physical_notes", "Physical description", nliRecord.PhysicalNotes, "source-nli-normalized", pilotItemID, "340$a", "normalized_marc_field"),
			itemAttribute("digitization_note", "Digitization note", []string{value(manuscriptRow, "הערות לתעתוק MiDRASH")}, "source-midrash-manuscripts", fmt.Sprintf("row:%d", manuscriptRow.number), "הערות לתעתוק MiDRASH", "direct_csv_field"),
			itemAttribute("project_note", "Project note", []string{value(manuscriptRow, "הערות")}, "source-midrash-manuscripts", fmt.Sprintf("row:%d", manuscriptRow.number), "הערות", "direct_csv_field"),
		},
	}
	items := []canonical.Item{manuscript}
	for _, row := range pilotEditions {
		items = append(items, canonicalEdition(row))
	}

	placesByID := map[string]canonical.Place{}
	addPlace := func(label string) string {
		id := stablePlaceID(label)
		seed, ok := placeSeeds[label]
		if !ok {
			placesByID[id] = canonical.Place{ID: id, Label: label}
			return id
		}
		raw := fmt.Sprintf("%s: %.6f, %.6f", label, seed.Latitude, seed.Longitude)
		placesByID[id] = canonical.Place{ID: id, Label: label, DisplayGeometry: &canonical.DisplayGeometry{
			VariantID: seed.VariantID, Type: "Point", Coordinates: []float64{seed.Longitude, seed.Latitude},
			SpatialPrecision: seed.SpatialPrecision, ReviewStatus: seed.ReviewStatus, InterpretationNote: seed.Note,
			Evidence: []canonical.Evidence{simpleEvidence("source-pilot-place-seeds", label, "latitude; longitude", raw, "curated_poc_seed")},
		}}
		return id
	}
	landOfIsraelID := addPlace("Land of Israel")
	vaticanCityID := addPlace("Vatican City, Vatican City State")

	agents := []canonical.Agent{
		{ID: "agent:eliahu-kapsali", Label: "Eliahu ben Elkanah Kapsali", Aliases: []string{"קפשאלי, אליהו בן אלקנה"}},
		{ID: "agent:a-fugger", Label: "A. Fugger", Aliases: []string{"פוגר, א."}},
		{ID: "agent:vatican-library", Label: "Vatican Library"},
	}

	creationRaw := value(creationRow, "זמן המדרש")
	creationEvidence := evidence("source-work-creation", creationRow, "זמן המדרש", creationRaw, "manual_structuring")
	events := []canonical.Event{
		{
			ID: "event:work:midrash-proverbs:formation", Layer: canonical.LayerWork,
			Type: "formation", Label: "Formation of Midrash Proverbs in the Land of Israel, 860–940",
			Subject:  canonical.EntityRef{Layer: canonical.LayerWork, ID: pilotWorkID},
			Places:   []canonical.PlaceAssertion{{PlaceID: landOfIsraelID, RawName: "ארץ ישראל", Role: "formation_place", Confidence: "source_assertion", Evidence: []canonical.Evidence{creationEvidence}}},
			Times:    []canonical.TemporalAssertion{{Value: canonical.TemporalValue{Kind: "range", Display: "860–940", StartYear: intPtr(860), EndYear: intPtr(940)}, Confidence: "source_assertion", Evidence: []canonical.Evidence{creationEvidence}}},
			Evidence: []canonical.Evidence{creationEvidence}, ReviewState: "unreviewed_source_assertion",
		},
		{
			ID: "event:item:vatican-ebr-44:production", Layer: canonical.LayerItem,
			Type: "production", Label: "Production dated by the NLI catalogue to the fourteenth century",
			Subject:  canonical.EntityRef{Layer: canonical.LayerItem, ID: pilotItemID},
			Times:    []canonical.TemporalAssertion{{Value: canonical.TemporalValue{Kind: "century", Display: "14th century", StartYear: intPtr(1301), EndYear: intPtr(1400), Century: intPtr(14)}, Confidence: "medium", Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "260$c", first(nliRecord.DateDisplay), "deterministic_parser")}}},
			Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "260$c", first(nliRecord.DateDisplay), "deterministic_parser")}, ReviewState: "unreviewed",
		},
		{
			ID: "event:item:vatican-ebr-44:acquisition-1541", Layer: canonical.LayerItem,
			Type: "acquisition", Label: "A. Fugger acquired the manuscript from Eliahu Kapsali on 8 November 1541",
			Subject:  canonical.EntityRef{Layer: canonical.LayerItem, ID: pilotItemID},
			Agents:   []canonical.AgentParticipation{{AgentID: "agent:eliahu-kapsali", Role: "seller_or_previous_owner"}, {AgentID: "agent:a-fugger", Role: "buyer"}},
			Times:    []canonical.TemporalAssertion{{Value: canonical.TemporalValue{Kind: "exact_date", Display: "8 November 1541", Date: "1541-11-08", Year: intPtr(1541)}, Confidence: "high", Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "561$a", nliRecord.ProvenanceNotes[1], "manual_structuring")}}},
			Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "561$a", nliRecord.ProvenanceNotes[1], "manual_structuring")}, ReviewState: "unreviewed",
		},
		{
			ID: "event:item:vatican-ebr-44:damaged-sale-note", Layer: canonical.LayerItem,
			Type: "sale", Label: "A damaged ownership note records another sale with most details lost",
			Subject:  canonical.EntityRef{Layer: canonical.LayerItem, ID: pilotItemID},
			Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "561$a", nliRecord.ProvenanceNotes[2], "manual_structuring")}, ReviewState: "unreviewed",
		},
		{
			ID: "event:item:vatican-ebr-44:current-custody", Layer: canonical.LayerItem,
			Type: "custody", Label: "Current custody at the Vatican Library",
			Subject:  canonical.EntityRef{Layer: canonical.LayerItem, ID: pilotItemID},
			Agents:   []canonical.AgentParticipation{{AgentID: "agent:vatican-library", Role: "current_owner"}},
			Places:   []canonical.PlaceAssertion{{PlaceID: vaticanCityID, RawName: "Vatican City, Vatican City State", Role: "current_repository", Confidence: "high", Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "710/942", "Vatican Library, Vatican City, Vatican City State", "normalized_marc_field")}}},
			Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "710/942", "Vatican Library, Vatican City, Vatican City State", "normalized_marc_field")}, ReviewState: "unreviewed",
		},
	}

	primary1990ID := ""
	for _, row := range pilotEditions {
		if row.number == 64 {
			primary1990ID = stableEditionID(row)
		}
	}
	for _, row := range pilotEditions {
		placeLabel := value(row, "place")
		placeID := addPlace(placeLabel)
		year, _ := strconv.Atoi(value(row, "year"))
		if year == 0 {
			year, _ = strconv.Atoi(value(row, " year"))
		}
		if override, ok := decisions.editionYearOverride(row.number); ok {
			year = override.CorrectedYear
		}
		editionID := stableEditionID(row)
		related := []canonical.RelatedEntity{}
		if value(row, "סוג דפוס") == "כפולה" || value(row, "סוג דפוס ") == "כפולה" {
			if primary1990ID != "" && primary1990ID != editionID {
				related = append(related, canonical.RelatedEntity{EntityRef: canonical.EntityRef{Layer: canonical.LayerItem, ID: primary1990ID}, Relation: "reprint_of"})
			}
		}
		raw := value(row, "דפוס")
		ev := evidence("source-printed-editions", row, "דפוס", raw, "direct_csv_field")
		events = append(events, canonical.Event{
			ID: "event:item:" + editionID + ":publication", Layer: canonical.LayerItem,
			Type: "publication", Label: raw, Subject: canonical.EntityRef{Layer: canonical.LayerItem, ID: editionID}, Related: related,
			Places:   []canonical.PlaceAssertion{{PlaceID: placeID, RawName: placeLabel, Role: "publication_place", Confidence: "source_assertion", Evidence: []canonical.Evidence{ev}}},
			Times:    []canonical.TemporalAssertion{{Value: canonical.TemporalValue{Kind: "year", Display: strconv.Itoa(year), Year: intPtr(year)}, Confidence: "source_assertion", Evidence: []canonical.Evidence{ev}}},
			Evidence: []canonical.Evidence{ev}, ReviewState: "unreviewed_source_assertion",
		})
	}

	places := make([]canonical.Place, 0, len(placesByID))
	for _, place := range placesByID {
		places = append(places, place)
	}
	sort.Slice(places, func(i, j int) bool { return places[i].ID < places[j].ID })

	doc := canonical.Document{
		SchemaVersion: canonical.SchemaVersion,
		LayerStates: []canonical.LayerState{
			{Layer: canonical.LayerWork, Status: "ready", Note: "Formation assertion structured from the Mayim narrative."},
			{Layer: canonical.LayerItem, Status: "partial", Note: "Core manuscript, contents, catalogue events, and editions are ready; PDF biography extraction remains pending."},
			{Layer: canonical.LayerText, Status: "pending", Note: "Sefaria passage selection and NER are intentionally deferred to the text-layer step."},
		},
		Works: works, Items: items, Texts: []canonical.Text{}, Events: events,
		Places: places, Agents: agents, Sources: sources,
	}
	if err := expandCorpus(&doc, cfg, nliNormalizedPath, ontologyRows, index, decisions, placeSeeds); err != nil {
		return canonical.Document{}, err
	}
	if err := doc.ValidateStructure(); err != nil {
		return canonical.Document{}, fmt.Errorf("validate canonical pilot: %w", err)
	}
	return doc, nil
}

func canonicalWork(row csvRow, id string) canonical.Work {
	aliases := []string{}
	for _, field := range []string{"אונטולוגיה", "מתוך טבלת רישום מדרשים", "שטראק-שטמברגר", "אנציקלופדיה יודאיקה (מ\"ד הר)", "ויקיפדיה", "פרוייקט השו\"ת", "רייזל", "פרידברג", "Friedberg", "English Reference MAZAL", "Transcribed Title"} {
		if alias := value(row, field); usableValue(alias) {
			aliases = appendUnique(aliases, alias)
		}
	}
	identifiers := []canonical.Identifier{{Scheme: "midrash_atlas_work_id", Value: id}}
	if mazal := value(row, "MAZAL ID"); mazal != "" {
		identifiers = append(identifiers, canonical.Identifier{Scheme: "nli_mazal_id", Value: mazal})
	}
	families := splitCommaValues(value(row, "Family"))
	return canonical.Work{
		ID: id, Layer: canonical.LayerWork, Title: value(row, "Common English Title"), HebrewTitle: value(row, "אונטולוגיה"),
		Aliases: aliases, Families: families, Identifiers: identifiers,
		Evidence: []canonical.Evidence{evidence("source-hibur-ontology", row, "ID חדש?; Common English Title", value(row, "אונטולוגיה")+" | "+id+" | "+value(row, "Common English Title"), "direct_csv_fields")},
	}
}

func canonicalEdition(row csvRow) canonical.Item {
	id := stableEditionID(row)
	editionType := value(row, "סוג דפוס")
	if editionType == "" {
		editionType = value(row, "סוג דפוס ")
	}
	rawEditionType := editionType
	if editionType == "" {
		editionType = "unclassified"
		rawEditionType = "(blank)"
	}
	return canonical.Item{
		ID: id, Layer: canonical.LayerItem, Kind: canonical.ItemPrintedEdition, Title: value(row, "דפוס"),
		WorkLinks:   []canonical.WorkLink{{WorkID: pilotWorkID, Relation: "edition_of", RawLabel: value(row, "יצירה"), SourceRef: fmt.Sprintf("printed_editions:row:%d", row.number)}},
		Identifiers: []canonical.Identifier{{Scheme: "derived_edition_id", Value: id}, {Scheme: "source_csv_row", Value: strconv.Itoa(row.number)}},
		Attributes:  []canonical.ItemAttribute{{Key: "edition_type", Label: "Edition type", Values: []string{editionType}, Evidence: []canonical.Evidence{simpleEvidence("source-printed-editions", fmt.Sprintf("row:%d", row.number), "סוג דפוס", rawEditionType, "direct_csv_field")}}},
		Evidence:    []canonical.Evidence{evidence("source-printed-editions", row, "דפוס", value(row, "דפוס"), "direct_csv_field")},
	}
}

func pilotManuscriptParts() []canonical.ItemPart {
	return []canonical.ItemPart{
		{ID: "part:vatican-ebr-44:tanhuma", Kind: "contents_unit", Label: "Midrash Tanhuma", Range: "1a–289b", WorkIDs: []string{"1.1-5:2:3.0"}, Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "505$a", "1) דף 1א-289ב: מדרש תנחומא.", "normalized_marc_field")}},
		{ID: "part:vatican-ebr-44:maaseh-torah", Kind: "contents_unit", Label: "Ma'aseh Torah of Rabbenu HaKadosh", Range: "298b–302b", WorkIDs: []string{"351:T"}, Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "505$a", "4) דף 298ב-302ב: מעשה תורה לרבינו הקדוש.", "normalized_marc_field")}},
		{ID: "part:vatican-ebr-44:yehoshua-ben-levi", Kind: "contents_unit", Label: "Ma'aseh Rabbi Yehoshua ben Levi", Range: "303a–304b", WorkIDs: []string{"166:T"}, Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "505$a", "5) דף 303א-304ב: מעשה רבי יהושע בן לוי.", "normalized_marc_field")}},
		{ID: "part:vatican-ebr-44:ben-sira", Kind: "contents_unit", Label: "Alphabet of Ben Sira", Range: "304b–322b", WorkIDs: []string{"73:T"}, Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "505$a", "6) דף 304ב-322ב: אלפא ביתא דבן סירא.", "normalized_marc_field")}},
		{ID: "part:vatican-ebr-44:yetzirat-havalad", Kind: "contents_unit", Label: "The Creation of the Embryo", Range: "323a–324b", WorkIDs: []string{"184:T"}, Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "505$a", "8) דף 323א-324ב: יצירת הולד.", "normalized_marc_field")}},
		{ID: "part:vatican-ebr-44:midrash-proverbs", Kind: "contents_unit", Label: "Midrash Proverbs", Range: "324b–355b", WorkIDs: []string{pilotWorkID}, Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "505$a", "9) דף 324ב-355ב: מדרש משלי.", "normalized_marc_field")}},
		{ID: "part:vatican-ebr-44:avot-rabbi-natan-a", Kind: "contents_unit", Label: "Avot de-Rabbi Natan – Version A", Range: "357b–374b", WorkIDs: []string{"2:1:2.0"}, Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", pilotItemID, "505$a", "11) דף 357ב-374ב: אבות דרבי נתן נוסח א.", "normalized_marc_field")}},
	}
}

func readPilotNLIRecord(path string) (nliNormalizedRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nliNormalizedRecord{}, fmt.Errorf("read normalized NLI source %s: %w", path, err)
	}
	var document nliNormalizedDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nliNormalizedRecord{}, fmt.Errorf("decode normalized NLI source %s: %w", path, err)
	}
	for _, record := range document.Records {
		if record.MMSID == pilotItemID {
			if len(record.DateDisplay) == 0 || len(record.ProvenanceNotes) < 3 {
				return nliNormalizedRecord{}, fmt.Errorf("pilot NLI record lacks required date or provenance evidence")
			}
			return record, nil
		}
	}
	return nliNormalizedRecord{}, fmt.Errorf("pilot NLI record %s not found", pilotItemID)
}

func readPilotPlaceSeeds(path string) (map[string]pilotPlaceSeed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pilot place display config %s: %w", path, err)
	}
	var document pilotPlaceSeedDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode pilot place display config %s: %w", path, err)
	}
	out := make(map[string]pilotPlaceSeed, len(document.Places))
	for _, place := range document.Places {
		if place.Label == "" || place.VariantID == "" || place.ReviewStatus == "" || place.Note == "" {
			return nil, fmt.Errorf("pilot place display config contains an incomplete place seed")
		}
		out[place.Label] = place
	}
	return out, nil
}

func findRow(rows []csvRow, field, wanted string) (csvRow, bool) {
	for _, row := range rows {
		if value(row, field) == wanted {
			return row, true
		}
	}
	return csvRow{}, false
}

func rowByNumber(rows []csvRow, number int) (csvRow, bool) {
	for _, row := range rows {
		if row.number == number {
			return row, true
		}
	}
	return csvRow{}, false
}

func evidence(sourceID string, row csvRow, field, raw, extraction string) canonical.Evidence {
	record := ""
	if row.number > 0 {
		record = fmt.Sprintf("row:%d", row.number)
	}
	return canonical.Evidence{SourceID: sourceID, SourceRecord: record, Field: field, Raw: raw, Extraction: extraction, ReviewStatus: "unreviewed"}
}

func simpleEvidence(sourceID, record, field, raw, extraction string) canonical.Evidence {
	return canonical.Evidence{SourceID: sourceID, SourceRecord: record, Field: field, Raw: raw, Extraction: extraction, ReviewStatus: "unreviewed"}
}

func itemAttribute(key, label string, values []string, sourceID, record, field, extraction string) canonical.ItemAttribute {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			clean = append(clean, value)
		}
	}
	raw := strings.Join(clean, " | ")
	if raw == "" {
		raw = "(blank)"
		clean = []string{"unavailable"}
	}
	return canonical.ItemAttribute{Key: key, Label: label, Values: clean, Evidence: []canonical.Evidence{simpleEvidence(sourceID, record, field, raw, extraction)}}
}

func stablePlaceID(label string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(label))))
	return "place:" + hex.EncodeToString(sum[:])[:12]
}

func splitCommaValues(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func intPtr(value int) *int { return &value }
