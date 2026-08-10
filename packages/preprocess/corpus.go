package preprocess

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	legacyatlas "midrash-atlas/packages/atlas"
	"midrash-atlas/packages/canonical"
)

// expandCorpus replaces the featured slice's entity lists with the complete
// validated project catalogue. Hand-structured pilot assertions are retained;
// bulk records add only deterministic, source-supported assertions.
func expandCorpus(doc *canonical.Document, cfg Config, nliPath string, ontologyRows []csvRow, index workIndex, decisions Decisions, placeSeeds map[string]pilotPlaceSeed) error {
	idStubs := map[int]string{}
	if cfg.HiburIDStubsCSV != "" {
		rows, _, err := readSource("generated_hibur_stub_ids", cfg.HiburIDStubsCSV)
		if err != nil {
			return err
		}
		throwaway := Report{Metrics: map[string]int{}}
		idStubs = validateOntologyIDStubs(rows, &throwaway)
	}

	stubRows := []csvRow{}
	if cfg.StubHiburimCSV != "" {
		rows, _, err := readSource("manuscript_reference_hibur_stubs", cfg.StubHiburimCSV)
		if err != nil {
			return err
		}
		stubRows = rows
		throwaway := Report{Metrics: map[string]int{}}
		validateAndIndexStubs(stubRows, index, &throwaway)
	}

	worksByID := map[string]canonical.Work{}
	for _, row := range ontologyRows {
		if !meaningful(row.values) || decisions.excludedOntologyRow(row.number) {
			continue
		}
		id := value(row, "ID חדש?")
		if id == "" {
			id = idStubs[row.number]
		}
		if id == "" {
			continue
		}
		work := canonicalWork(row, id)
		if existing, ok := worksByID[id]; ok {
			existing.Aliases = appendUnique(existing.Aliases, work.HebrewTitle)
			for _, alias := range work.Aliases {
				existing.Aliases = appendUnique(existing.Aliases, alias)
			}
			existing.Evidence = append(existing.Evidence, work.Evidence...)
			worksByID[id] = existing
		} else {
			worksByID[id] = work
		}
	}
	for _, row := range stubRows {
		if !meaningful(row.values) {
			continue
		}
		id := value(row, "stub_id")
		worksByID[id] = canonical.Work{
			ID: id, Layer: canonical.LayerWork, Title: value(row, "common_english_title"), HebrewTitle: value(row, "hebrew_label"),
			Aliases:     []string{value(row, "hebrew_label")},
			Identifiers: []canonical.Identifier{{Scheme: "temporary_manuscript_reference_stub_id", Value: id}},
			Evidence:    []canonical.Evidence{evidence("source-manuscript-work-stubs", row, "comment", value(row, "comment"), "curated_stub_record")},
		}
	}
	doc.Works = valuesSortedWorks(worksByID)

	nliRecords, err := readAllNLIRecords(nliPath)
	if err != nil {
		return err
	}
	nliByID := map[string]nliNormalizedRecord{}
	for _, record := range nliRecords {
		nliByID[record.MMSID] = record
	}

	manuscriptRows, _, err := readSource("manuscripts", cfg.ManuscriptsCSV)
	if err != nil {
		return err
	}
	rowsByMMS := map[string][]csvRow{}
	for _, row := range manuscriptRows {
		if !meaningful(row.values) || decisions.excludedManuscriptRow(row.number) {
			continue
		}
		id := value(row, "מספר מערכת")
		if len(id) == 18 && allDigits(id) {
			rowsByMMS[id] = append(rowsByMMS[id], row)
		}
	}

	items := make([]canonical.Item, 0, len(rowsByMMS)+320)
	for id, rows := range rowsByMMS {
		items = append(items, canonicalManuscript(id, rows, nliByID[id], index))
	}

	editionRows, _, err := readSource("printed_editions", cfg.EditionsCSV)
	if err != nil {
		return err
	}
	editionWorkIDs := map[int][]string{}
	for _, row := range editionRows {
		if !meaningful(row.values) {
			continue
		}
		label := value(row, "יצירה")
		if override, ok := decisions.editionWorkOverride(row.number); ok {
			editionWorkIDs[row.number] = []string{override.TargetWorkID}
		} else {
			editionWorkIDs[row.number] = resolvedWorkIDs(index.resolve(label))
		}
		items = append(items, canonicalEditionForWorks(row, editionWorkIDs[row.number]))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind == canonical.ItemManuscript
		}
		return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
	})
	doc.Items = items

	placesByID := map[string]canonical.Place{}
	for _, place := range doc.Places {
		placesByID[place.ID] = place
	}
	addPlace := func(label string) string {
		label = strings.TrimSpace(label)
		if label == "" {
			return ""
		}
		id := stablePlaceID(label)
		if _, exists := placesByID[id]; exists {
			return id
		}
		place := canonical.Place{ID: id, Label: label}
		if seed, ok := placeSeeds[label]; ok {
			raw := fmt.Sprintf("%s: %.6f, %.6f", label, seed.Latitude, seed.Longitude)
			place.DisplayGeometry = &canonical.DisplayGeometry{
				VariantID: seed.VariantID, Type: "Point", Coordinates: []float64{seed.Longitude, seed.Latitude},
				SpatialPrecision: seed.SpatialPrecision, ReviewStatus: seed.ReviewStatus, InterpretationNote: seed.Note,
				Evidence: []canonical.Evidence{simpleEvidence("source-pilot-place-seeds", label, "latitude; longitude", raw, "curated_poc_seed")},
			}
		}
		placesByID[id] = place
		return id
	}

	events := make([]canonical.Event, 0, len(doc.Events)+len(items)+len(editionRows))
	for _, event := range doc.Events {
		if event.Type != "publication" {
			events = append(events, event)
		}
	}
	for id, rows := range rowsByMMS {
		if id == pilotItemID {
			continue
		}
		record := nliByID[id]
		appendProduction := func(source nliNormalizedRecord, origin, suffix string) {
			dateRaw := first(source.DateDisplay)
			writingPlaces := []nliNormalizedPlace{}
			for _, place := range source.Places {
				if place.Role == "place of writing" {
					writingPlaces = append(writingPlaces, place)
				}
			}
			if dateRaw == "" && len(writingPlaces) == 0 {
				return
			}
			raw := strings.Trim(strings.Join(append([]string{dateRaw}, placeNames(writingPlaces)...), " | "), " |")
			ev := simpleEvidence("source-nli-normalized", source.MMSID, "260$c; 751", raw, origin)
			label := "Catalogue production assertion for " + manuscriptTitle(rows, record)
			if origin != "direct" {
				label += " (from parent record " + source.MMSID + ")"
			}
			production := canonical.Event{ID: "event:item:" + id + ":production" + suffix, Layer: canonical.LayerItem, Type: "production", Label: label, Subject: canonical.EntityRef{Layer: canonical.LayerItem, ID: id}, Evidence: []canonical.Evidence{ev}, ReviewState: origin}
			if dateRaw != "" {
				parsed := legacyatlas.ParseDate(dateRaw)
				production.Times = []canonical.TemporalAssertion{{Value: canonical.TemporalValue{
					Kind: parsed.Kind, Display: dateRaw, Year: parsed.Year, StartYear: parsed.StartYear, EndYear: parsed.EndYear,
					Century: parsed.Century, StartCentury: parsed.StartCentury, EndCentury: parsed.EndCentury,
					Part: parsed.Part, Approximate: parsed.Approximate, Uncertain: parsed.Uncertain,
				}, Confidence: origin, Evidence: []canonical.Evidence{ev}}}
			}
			for _, place := range writingPlaces {
				production.Places = append(production.Places, canonical.PlaceAssertion{PlaceID: addPlace(place.Name), RawName: place.Name, Role: "production_place", Confidence: origin, Evidence: []canonical.Evidence{ev}})
			}
			events = append(events, production)
		}
		appendProduction(record, "direct", "")
		for _, parentID := range record.ParentMMSIDs {
			if parent, ok := nliByID[parentID]; ok {
				origin := "parent_alternative"
				if len(record.DateDisplay) == 0 && !hasWritingPlace(record.Places) {
					origin = "parent_fallback"
				}
				appendProduction(parent, origin, ":parent:"+parentID)
			}
		}
		if owner, ok := firstOwner(record); ok {
			label := joinPlace(owner.Locality, owner.Country)
			if label == "" {
				label = owner.Name
			}
			raw := strings.TrimSpace(strings.Join([]string{owner.Name, owner.Locality, owner.Country}, ", "))
			ev := simpleEvidence("source-nli-normalized", id, "710; 942", raw, "normalized_marc_fields")
			events = append(events, canonical.Event{ID: "event:item:" + id + ":current-custody", Layer: canonical.LayerItem, Type: "custody", Label: "Current custody at " + owner.Name, Subject: canonical.EntityRef{Layer: canonical.LayerItem, ID: id}, Places: []canonical.PlaceAssertion{{PlaceID: addPlace(label), RawName: label, Role: "current_repository", Confidence: "source_assertion", Evidence: []canonical.Evidence{ev}}}, Evidence: []canonical.Evidence{ev}, ReviewState: "unreviewed_source_assertion"})
		}
	}

	creationRows, _, err := readSource("work_creation", cfg.CreationCSV)
	if err != nil {
		return err
	}
	for _, row := range creationRows {
		if !meaningful(row.values) || value(row, "קטגוריה") == "מבוא" {
			continue
		}
		ids := resolvedWorkIDs(index.resolve(value(row, "Title")))
		for _, id := range ids {
			if id == pilotWorkID {
				continue
			}
			raw := value(row, "זמן המדרש")
			ev := evidence("source-work-creation", row, "זמן המדרש", raw, "unstructured_source_narrative")
			events = append(events, canonical.Event{ID: fmt.Sprintf("event:work:%s:formation-narrative:%d", id, row.number), Layer: canonical.LayerWork, Type: "formation_narrative", Label: raw, Subject: canonical.EntityRef{Layer: canonical.LayerWork, ID: id}, Evidence: []canonical.Evidence{ev}, ReviewState: "awaiting_structuring"})
		}
	}

	primaryEdition := map[string]string{}
	for _, row := range editionRows {
		if !meaningful(row.values) {
			continue
		}
		ids := editionWorkIDs[row.number]
		year := editionYear(row, decisions)
		placeLabel := value(row, "place")
		key := strings.Join(ids, "+") + "|" + placeLabel + "|" + strconv.Itoa(year)
		id := stableEditionID(row)
		isReprint := value(row, "סוג דפוס") == "כפולה" || value(row, "סוג דפוס ") == "כפולה"
		related := []canonical.RelatedEntity{}
		if isReprint {
			if primary := primaryEdition[key]; primary != "" {
				related = append(related, canonical.RelatedEntity{EntityRef: canonical.EntityRef{Layer: canonical.LayerItem, ID: primary}, Relation: "reprint_of"})
			}
		} else if primaryEdition[key] == "" {
			primaryEdition[key] = id
		}
		raw := value(row, "דפוס")
		ev := evidence("source-printed-editions", row, "דפוס; place; year", raw+" | "+placeLabel+" | "+strconv.Itoa(year), "direct_csv_fields")
		event := canonical.Event{ID: "event:item:" + id + ":publication", Layer: canonical.LayerItem, Type: "publication", Label: raw, Subject: canonical.EntityRef{Layer: canonical.LayerItem, ID: id}, Related: related, Evidence: []canonical.Evidence{ev}, ReviewState: "unreviewed_source_assertion"}
		if placeLabel != "" {
			event.Places = []canonical.PlaceAssertion{{PlaceID: addPlace(placeLabel), RawName: placeLabel, Role: "publication_place", Confidence: "source_assertion", Evidence: []canonical.Evidence{ev}}}
		}
		if year > 0 {
			event.Times = []canonical.TemporalAssertion{{Value: canonical.TemporalValue{Kind: "year", Display: strconv.Itoa(year), Year: intPtr(year)}, Confidence: "source_assertion", Evidence: []canonical.Evidence{ev}}}
		}
		events = append(events, event)
	}
	doc.Events = events

	doc.Places = make([]canonical.Place, 0, len(placesByID))
	for _, place := range placesByID {
		doc.Places = append(doc.Places, place)
	}
	sort.Slice(doc.Places, func(i, j int) bool { return doc.Places[i].Label < doc.Places[j].Label })
	doc.Sources = []canonical.Source{
		{ID: "source-hibur-ontology", Type: "csv", Label: "Hiburim ontology", Citation: cfg.HiburimCSV},
		{ID: "source-manuscript-work-stubs", Type: "csv", Label: "Clearly marked manuscript-reference work stubs", Citation: cfg.StubHiburimCSV},
		{ID: "source-midrash-manuscripts", Type: "csv", Label: "Midrash manuscript list", Citation: cfg.ManuscriptsCSV},
		{ID: "source-work-creation", Type: "csv", Label: "Mayim work-creation narratives", Citation: cfg.CreationCSV},
		{ID: "source-printed-editions", Type: "csv", Label: "Mayim printed editions", Citation: cfg.EditionsCSV},
		{ID: "source-nli-normalized", Type: "normalized_marc", Label: "Normalized NLI manuscript catalogue", Citation: nliPath},
		{ID: "source-pilot-place-seeds", Type: "curated_display_config", Label: "Unreviewed pilot display positions", Citation: cfg.PilotPlacesJSON},
	}
	doc.LayerStates = []canonical.LayerState{
		{Layer: canonical.LayerWork, Status: "partial", Note: "Complete work catalogue loaded; creation narratives are preserved, while only reviewed structured formation assertions are mapped."},
		{Layer: canonical.LayerItem, Status: "partial", Note: "Complete manuscript and edition catalogue loaded with NLI enrichment; prose provenance and PDF biography extraction remain separate curation steps."},
		{Layer: canonical.LayerText, Status: "pending", Note: "Sefaria passage selection and NER are intentionally deferred to the text-layer step."},
	}
	return nil
}

func readAllNLIRecords(path string) ([]nliNormalizedRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read normalized NLI source %s: %w", path, err)
	}
	var document nliNormalizedDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode normalized NLI source %s: %w", path, err)
	}
	return document.Records, nil
}

func valuesSortedWorks(byID map[string]canonical.Work) []canonical.Work {
	works := make([]canonical.Work, 0, len(byID))
	for _, work := range byID {
		works = append(works, work)
	}
	sort.Slice(works, func(i, j int) bool { return strings.ToLower(works[i].Title) < strings.ToLower(works[j].Title) })
	return works
}

func resolvedWorkIDs(result matchResult) []string {
	if result.kind != matchExact && result.kind != matchAlias && result.kind != matchMerged {
		return nil
	}
	ids := []string{}
	for _, candidate := range result.candidates {
		if candidate.id != "" {
			ids = appendUnique(ids, candidate.id)
		}
	}
	return ids
}

func canonicalManuscript(id string, rows []csvRow, record nliNormalizedRecord, index workIndex) canonical.Item {
	links := []canonical.WorkLink{}
	seenLinks := map[string]bool{}
	for _, row := range rows {
		for _, rawLabel := range splitLabels(value(row, "חיבורים - טור זמני")) {
			for _, workID := range resolvedWorkIDs(index.resolve(rawLabel)) {
				if seenLinks[workID] {
					continue
				}
				seenLinks[workID] = true
				links = append(links, canonical.WorkLink{WorkID: workID, Relation: "contains_work", RawLabel: rawLabel, SourceRef: fmt.Sprintf("manuscripts:row:%d", row.number)})
			}
		}
	}
	title := manuscriptTitle(rows, record)
	evidenceItems := []canonical.Evidence{}
	if record.MMSID != "" {
		evidenceItems = append(evidenceItems, simpleEvidence("source-nli-normalized", id, "mms_id", id, "normalized_marc_field"))
	}
	for _, row := range rows {
		evidenceItems = append(evidenceItems, evidence("source-midrash-manuscripts", row, "כינוי כתב היד בכתיב", nonblank(value(row, "כינוי כתב היד בכתיב"), id), "direct_csv_field"))
	}
	item := canonical.Item{ID: id, Layer: canonical.LayerItem, Kind: canonical.ItemManuscript, Title: title, WorkLinks: links, Evidence: evidenceItems, Identifiers: []canonical.Identifier{{Scheme: "nli_mms_id", Value: id}}}
	for _, row := range rows {
		if local := value(row, "Midrash_MSS ID"); local != "" {
			item.Identifiers = append(item.Identifiers, canonical.Identifier{Scheme: "midrash_mss_id", Value: local})
		}
	}
	if record.PublicRecordURL != "" {
		item.Identifiers = append(item.Identifiers, canonical.Identifier{Scheme: "nli_public_record_url", Value: record.PublicRecordURL})
	}
	if record.ResolverURL != "" {
		item.Identifiers = append(item.Identifiers, canonical.Identifier{Scheme: "nli_digital_resolver_url", Value: record.ResolverURL})
	}
	addNLIAttribute(&item, "catalogue_title", "NLI catalogue title", record.Title, id, "245")
	addNLIAttribute(&item, "alternative_titles", "Alternative titles", record.AlternativeTitles, id, "740")
	addNLIAttribute(&item, "catalogue_date", "Catalogue date", record.DateDisplay, id, "260$c")
	addNLIAttribute(&item, "extent", "Extent", record.Extent, id, "300$a")
	addNLIAttribute(&item, "languages", "Languages", record.Languages, id, "041$a")
	addNLIAttribute(&item, "script_styles", "Script styles", record.ScriptStyles, id, "958$a")
	addNLIAttribute(&item, "physical_notes", "Physical description", record.PhysicalNotes, id, "340$a")
	addNLIAttribute(&item, "general_notes", "Catalogue notes", record.GeneralNotes, id, "500")
	addNLIAttribute(&item, "provenance_notes", "Provenance notes", record.ProvenanceNotes, id, "561$a")
	addNLIAttribute(&item, "colophon_notes", "Colophon notes", record.ColophonNotes, id, "957")
	addNLIAttribute(&item, "catalog_references", "Catalogue references", catalogReferenceValues(record.CatalogReferences), id, "942 ind.3")
	addNLIAttribute(&item, "rights", "Rights", fieldObjectValues(record.Rights), id, "903")
	addNLIAttribute(&item, "digital_services", "Digital services", fieldObjectValues(record.DigitalServices), id, "AVE")
	if len(record.ParentMMSIDs) > 0 {
		addNLIAttribute(&item, "parent_records", "Parent manuscript records", record.ParentMMSIDs, id, "773$w")
	}
	for _, row := range rows {
		addCSVAttribute(&item, "digitization_note", "Project digitization note", value(row, "הערות לתעתוק MiDRASH"), row, "הערות לתעתוק MiDRASH")
		addCSVAttribute(&item, "project_note", "Project note", value(row, "הערות"), row, "הערות")
	}
	for i, content := range record.Contents {
		item.Parts = append(item.Parts, canonical.ItemPart{ID: fmt.Sprintf("part:%s:%d", id, i+1), Kind: "catalogued_contents_unit", Label: content, Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", id, "505$a", content, "normalized_marc_field")}})
	}
	return item
}

func manuscriptTitle(rows []csvRow, record nliNormalizedRecord) string {
	if len(record.Shelfmarks) > 0 {
		shelf := record.Shelfmarks[0]
		if shelf.Repository != "" && shelf.Shelfmark != "" {
			return shelf.Repository + ", " + shelf.Shelfmark
		}
	}
	for _, row := range rows {
		if title := value(row, "כינוי כתב היד בכתיב"); title != "" {
			return title
		}
	}
	if title := first(record.Title); title != "" {
		return title
	}
	return record.MMSID
}

func canonicalEditionForWorks(row csvRow, workIDs []string) canonical.Item {
	item := canonicalEdition(row)
	item.WorkLinks = make([]canonical.WorkLink, 0, len(workIDs))
	for _, id := range workIDs {
		item.WorkLinks = append(item.WorkLinks, canonical.WorkLink{WorkID: id, Relation: "edition_of", RawLabel: value(row, "יצירה"), SourceRef: fmt.Sprintf("printed_editions:row:%d", row.number)})
	}
	return item
}

func addNLIAttribute(item *canonical.Item, key, label string, values []string, record, field string) {
	clean := []string{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			clean = append(clean, value)
		}
	}
	if len(clean) == 0 {
		return
	}
	item.Attributes = append(item.Attributes, canonical.ItemAttribute{Key: key, Label: label, Values: clean, Evidence: []canonical.Evidence{simpleEvidence("source-nli-normalized", record, field, strings.Join(clean, " | "), "normalized_marc_fields")}})
}

func addCSVAttribute(item *canonical.Item, key, label, raw string, row csvRow, field string) {
	if strings.TrimSpace(raw) == "" {
		return
	}
	item.Attributes = append(item.Attributes, canonical.ItemAttribute{Key: fmt.Sprintf("%s_row_%d", key, row.number), Label: label, Values: []string{raw}, Evidence: []canonical.Evidence{evidence("source-midrash-manuscripts", row, field, raw, "direct_csv_field")}})
}

func catalogReferenceValues(refs []nliCatalogReference) []string {
	out := []string{}
	for _, ref := range refs {
		out = append(out, strings.TrimSpace(ref.Catalog+" · "+ref.Entry))
	}
	return out
}
func fieldObjectValues(objects []nliFieldObject) []string {
	out := []string{}
	for _, object := range objects {
		parts := []string{}
		for _, field := range object.Subfields {
			if field.Value != "" {
				parts = append(parts, field.Code+": "+field.Value)
			}
		}
		if len(parts) > 0 {
			out = append(out, strings.Join(parts, " · "))
		}
	}
	return out
}
func firstOwner(record nliNormalizedRecord) (nliNormalizedOwner, bool) {
	if len(record.CurrentOwners) == 0 {
		return nliNormalizedOwner{}, false
	}
	return record.CurrentOwners[0], true
}
func joinPlace(locality, country string) string {
	if locality == "" {
		return country
	}
	if country == "" {
		return locality
	}
	return locality + ", " + country
}
func placeNames(places []nliNormalizedPlace) []string {
	out := []string{}
	for _, place := range places {
		out = append(out, place.Name)
	}
	return out
}
func hasWritingPlace(places []nliNormalizedPlace) bool {
	for _, place := range places {
		if place.Role == "place of writing" {
			return true
		}
	}
	return false
}
func nonblank(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
func allDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}
func editionYear(row csvRow, decisions Decisions) int {
	raw := value(row, "year")
	if raw == "" {
		raw = value(row, " year")
	}
	year, _ := strconv.Atoi(raw)
	if override, ok := decisions.editionYearOverride(row.number); ok {
		year = override.CorrectedYear
	}
	return year
}
