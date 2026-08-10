// Package preprocess validates heterogeneous source data before it crosses the
// canonical atlas boundary.
package preprocess

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const ReportSchemaVersion = "1.0.0"

type Config struct {
	HiburimCSV      string
	HiburIDStubsCSV string
	StubHiburimCSV  string
	ManuscriptsCSV  string
	CreationCSV     string
	EditionsCSV     string
	DecisionsJSON   string
	PilotPlacesJSON string
}

type csvRow struct {
	number int
	values map[string]string
}

type workCandidate struct {
	row          int
	id           string
	ontologyName string
	title        string
	aliases      []string
}

type workIndex struct {
	byOntology               map[string][]*workCandidate
	byAlias                  map[string][]*workCandidate
	duplicateIDs             map[string]bool
	approvedMergedAssertions map[string]bool
}

type matchKind string

const (
	matchExact     matchKind = "exact"
	matchAlias     matchKind = "alias"
	matchMissing   matchKind = "missing"
	matchAmbiguous matchKind = "ambiguous"
	matchInvalidID matchKind = "invalid_id"
	matchMerged    matchKind = "merged_interpretation"
)

type matchResult struct {
	kind       matchKind
	candidates []*workCandidate
}

func Validate(cfg Config) (Report, error) {
	report := Report{SchemaVersion: ReportSchemaVersion, Metrics: map[string]int{}}
	decisions, err := readDecisions(cfg.DecisionsJSON)
	if err != nil {
		return report, err
	}

	ontologyRows, source, err := readSource("hiburim", cfg.HiburimCSV)
	if err != nil {
		return report, err
	}
	report.Sources = append(report.Sources, source)
	idStubs := map[int]string{}
	if cfg.HiburIDStubsCSV != "" {
		stubIDRows, stubIDSource, err := readSource("generated_hibur_stub_ids", cfg.HiburIDStubsCSV)
		if err != nil {
			return report, err
		}
		report.Sources = append(report.Sources, stubIDSource)
		idStubs = validateOntologyIDStubs(stubIDRows, &report)
	}
	works, index := validateOntology(ontologyRows, idStubs, decisions, &report)
	for _, merged := range decisions.MergedInterpretations {
		if merged.Resolution == "preserve_as_multi_work_assertion" {
			index.approvedMergedAssertions[normalize(merged.Label)] = true
		}
	}
	report.Metrics["works.source_rows"] = len(works)
	if cfg.StubHiburimCSV != "" {
		stubRows, stubSource, err := readSource("manuscript_reference_hibur_stubs", cfg.StubHiburimCSV)
		if err != nil {
			return report, err
		}
		report.Sources = append(report.Sources, stubSource)
		validateAndIndexStubs(stubRows, index, &report)
	}

	manuscriptRows, source, err := readSource("manuscripts", cfg.ManuscriptsCSV)
	if err != nil {
		return report, err
	}
	report.Sources = append(report.Sources, source)
	validateManuscripts(manuscriptRows, index, decisions, &report)

	creationRows, source, err := readSource("work_creation", cfg.CreationCSV)
	if err != nil {
		return report, err
	}
	report.Sources = append(report.Sources, source)
	validateCreation(creationRows, index, &report)

	editionRows, source, err := readSource("printed_editions", cfg.EditionsCSV)
	if err != nil {
		return report, err
	}
	report.Sources = append(report.Sources, source)
	validateEditions(editionRows, index, decisions, &report)

	sort.Slice(report.Issues, func(i, j int) bool {
		a, b := report.Issues[i], report.Issues[j]
		if severityOrder(a.Severity) != severityOrder(b.Severity) {
			return severityOrder(a.Severity) < severityOrder(b.Severity)
		}
		if a.SourceID != b.SourceID {
			return a.SourceID < b.SourceID
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Value < b.Value
	})
	for _, issue := range report.Issues {
		switch issue.Severity {
		case SeverityError:
			report.Summary.ErrorGroups++
			report.Summary.ErrorOccurrences += issue.Count
		case SeverityWarning:
			report.Summary.WarningGroups++
			report.Summary.WarningOccurrences += issue.Count
		case SeverityInfo:
			report.Summary.InfoGroups++
			report.Summary.InfoOccurrences += issue.Count
		}
	}
	if report.Summary.ErrorGroups > 0 {
		report.Summary.Status = "invalid"
	} else if report.Summary.WarningGroups > 0 {
		report.Summary.Status = "valid_with_warnings"
	} else {
		report.Summary.Status = "valid"
	}
	return report, nil
}

func validateOntology(rows []csvRow, idStubs map[int]string, decisions Decisions, report *Report) ([]*workCandidate, workIndex) {
	aliasFields := []string{
		"אונטולוגיה", "מתוך טבלת רישום מדרשים", "שטראק-שטמברגר",
		"אנציקלופדיה יודאיקה (מ\"ד הר)", "ויקיפדיה", "פרוייקט השו\"ת",
		"רייזל", "פרידברג", "Friedberg", "מ. ביאליק-לרנר",
		"English Reference MAZAL", "Common English Title", "Transcribed Title",
	}
	var works []*workCandidate
	ids := map[string][]int{}
	index := workIndex{
		byOntology: map[string][]*workCandidate{}, byAlias: map[string][]*workCandidate{},
		duplicateIDs: map[string]bool{}, approvedMergedAssertions: map[string]bool{},
	}
	for _, row := range rows {
		if !meaningful(row.values) {
			continue
		}
		if decisions.excludedOntologyRow(row.number) {
			addIssue(report, SeverityInfo, "unused_incomplete_ontology_row_excluded", "hiburim", "row", strconv.Itoa(row.number), []int{row.number}, 1,
				"A curator chose to preserve this unused incomplete source row as-is and exclude it from canonical export for now.")
			continue
		}
		candidate := &workCandidate{
			row: row.number, id: value(row, "ID חדש?"),
			ontologyName: value(row, "אונטולוגיה"), title: value(row, "Common English Title"),
		}
		if candidate.id == "" && idStubs[row.number] != "" {
			candidate.id = idStubs[row.number]
			addIssue(report, SeverityInfo, "generated_ontology_stub_id_applied", "hiburim", "ID חדש?", candidate.id, []int{row.number}, 1,
				"A clearly marked temporary ID supplies a missing ontology ID until scholarly review assigns a permanent value.")
		}
		for _, field := range aliasFields {
			name := value(row, field)
			if usableValue(name) {
				candidate.aliases = appendUnique(candidate.aliases, name)
			}
		}
		works = append(works, candidate)
		if candidate.id == "" {
			addIssue(report, SeverityError, "work_missing_internal_id", "hiburim", "ID חדש?", candidate.title, []int{row.number}, 1,
				"A meaningful work row must have the requested canonical internal ID.")
		} else {
			ids[candidate.id] = append(ids[candidate.id], row.number)
		}
		if candidate.title == "" {
			addIssue(report, SeverityError, "work_missing_common_english_title", "hiburim", "Common English Title", candidate.ontologyName, []int{row.number}, 1,
				"A work needs the canonical UI title from Common English Title.")
		}
		if candidate.ontologyName == "" {
			addIssue(report, SeverityWarning, "work_missing_ontology_join_key", "hiburim", "אונטולוגיה", candidate.title, []int{row.number}, 1,
				"The preferred manuscript join key is empty; aliases may still identify the work.")
		} else {
			key := normalize(candidate.ontologyName)
			index.byOntology[key] = append(index.byOntology[key], candidate)
		}
		for _, alias := range candidate.aliases {
			key := normalize(alias)
			index.byAlias[key] = appendCandidate(index.byAlias[key], candidate)
		}
	}
	for id, rowNumbers := range ids {
		if len(rowNumbers) > 1 {
			if decisions.approvedSharedID(id) {
				addIssue(report, SeverityInfo, "approved_same_work_rows", "hiburim", "ID חדש?", id, rowNumbers, len(rowNumbers),
					"A curator confirmed that these source rows are alternate names for the same canonical work.")
				report.Metrics["works.approved_shared_internal_ids"]++
				continue
			}
			index.duplicateIDs[id] = true
			addIssue(report, SeverityError, "duplicate_work_internal_id", "hiburim", "ID חדש?", id, rowNumbers, len(rowNumbers),
				"The canonical internal ID identifies more than one work.")
		}
	}
	for alias, candidates := range index.byAlias {
		validRows := distinctCandidateRows(candidates)
		if len(validRows) > 1 {
			rows := make([]int, 0, len(validRows))
			for row := range validRows {
				rows = append(rows, row)
			}
			sort.Ints(rows)
			addIssue(report, SeverityWarning, "ambiguous_work_alias", "hiburim", "aliases", alias, rows, len(rows),
				"The normalized alias occurs on more than one work and cannot be used as an automatic join key.")
		}
	}
	report.Metrics["works.with_internal_id"] = len(ids)
	report.Metrics["works.duplicate_internal_ids"] = len(index.duplicateIDs)
	return works, index
}

func validateOntologyIDStubs(rows []csvRow, report *Report) map[int]string {
	stubs := map[int]string{}
	seenIDs := map[string]int{}
	for _, row := range rows {
		if !meaningful(row.values) {
			continue
		}
		sourceRow, err := strconv.Atoi(value(row, "ontology_source_row"))
		id := value(row, "generated_stub_id")
		status := value(row, "status")
		if err != nil || sourceRow < 2 || !strings.HasPrefix(id, "stub:ontology:") || status != "stub_id_for_missing_ontology_id" {
			addIssue(report, SeverityError, "invalid_generated_hibur_stub_id", "generated_hibur_stub_ids", "generated_stub_id", id, []int{row.number}, 1,
				"A generated ontology ID requires a source row, a stub:ontology:* ID, and explicit stub status.")
			continue
		}
		if previousRow, exists := seenIDs[id]; exists {
			addIssue(report, SeverityError, "duplicate_generated_hibur_stub_id", "generated_hibur_stub_ids", "generated_stub_id", id, []int{previousRow, row.number}, 2,
				"A generated ontology stub ID must identify exactly one ontology row.")
			continue
		}
		if _, exists := stubs[sourceRow]; exists {
			addIssue(report, SeverityError, "duplicate_generated_hibur_stub_source_row", "generated_hibur_stub_ids", "ontology_source_row", strconv.Itoa(sourceRow), []int{row.number}, 1,
				"An ontology source row may have only one generated stub ID.")
			continue
		}
		stubs[sourceRow] = id
		seenIDs[id] = row.number
	}
	report.Metrics["works.generated_ontology_stub_ids"] = len(stubs)
	return stubs
}

func validateAndIndexStubs(rows []csvRow, index workIndex, report *Report) {
	count := 0
	for _, row := range rows {
		if !meaningful(row.values) {
			continue
		}
		id := value(row, "stub_id")
		label := value(row, "hebrew_label")
		title := value(row, "common_english_title")
		status := value(row, "status")
		comment := value(row, "comment")
		if id == "" || label == "" || title == "" || status != "stub_created_from_manuscript_reference" || comment == "" {
			addIssue(report, SeverityError, "invalid_manuscript_reference_stub", "manuscript_reference_hibur_stubs", "stub", label, []int{row.number}, 1,
				"A manuscript-derived stub needs an ID, Hebrew label, clearly marked UI title, stub status, and explanatory comment.")
			continue
		}
		candidate := &workCandidate{row: -row.number, id: id, ontologyName: label, title: title, aliases: []string{label, title}}
		key := normalize(label)
		index.byOntology[key] = append(index.byOntology[key], candidate)
		for _, alias := range candidate.aliases {
			aliasKey := normalize(alias)
			index.byAlias[aliasKey] = appendCandidate(index.byAlias[aliasKey], candidate)
		}
		count++
	}
	report.Metrics["works.manuscript_reference_stubs"] = count
	if count > 0 {
		addIssue(report, SeverityInfo, "manuscript_reference_stubs_loaded", "manuscript_reference_hibur_stubs", "status", "stub_created_from_manuscript_reference", nil, count,
			"Temporary hibur records were loaded from unresolved manuscript contents labels and remain explicitly marked as stubs.")
	}
}

func validateManuscripts(rows []csvRow, index workIndex, decisions Decisions, report *Report) {
	report.Metrics["manuscripts.source_rows"] = countMeaningful(rows)
	groups := map[string][]int{}
	rowsWithLinks, linkCount, exactCount, aliasCount := 0, 0, 0, 0
	unresolvedCount, ambiguousCount, invalidIDCount, mergedCount := 0, 0, 0, 0
	for _, row := range rows {
		if !meaningful(row.values) {
			continue
		}
		if decisions.excludedManuscriptRow(row.number) {
			addIssue(report, SeverityInfo, "empty_manuscript_placeholder_excluded", "manuscripts", "row", strconv.Itoa(row.number), []int{row.number}, 1,
				"A curator excluded this empty project placeholder from canonical item export.")
			continue
		}
		if value(row, "מספר מערכת") == "" {
			addIssue(report, SeverityError, "manuscript_missing_system_id", "manuscripts", "מספר מערכת", value(row, "כינוי כתב היד בכתיב"), []int{row.number}, 1,
				"A manuscript requires a stable system identifier before it can become an item.")
		}
		if value(row, "Midrash_MSS ID") == "" {
			groups["missing-local-id"] = append(groups["missing-local-id"], row.number)
		}
		raw := value(row, "חיבורים - טור זמני")
		if raw == "" {
			continue
		}
		rowsWithLinks++
		for _, label := range splitLabels(raw) {
			linkCount++
			result := index.resolve(label)
			switch result.kind {
			case matchExact:
				exactCount++
			case matchAlias:
				aliasCount++
				groups["alias:"+label] = append(groups["alias:"+label], row.number)
			case matchMissing:
				unresolvedCount++
				groups["unresolved:"+label] = append(groups["unresolved:"+label], row.number)
			case matchAmbiguous:
				ambiguousCount++
				groups["ambiguous:"+label] = append(groups["ambiguous:"+label], row.number)
			case matchInvalidID:
				invalidIDCount++
				groups["invalid-id:"+label] = append(groups["invalid-id:"+label], row.number)
			case matchMerged:
				mergedCount++
				groups["merged:"+label] = append(groups["merged:"+label], row.number)
			}
		}
	}
	report.Metrics["manuscripts.rows_with_work_links"] = rowsWithLinks
	report.Metrics["manuscripts.work_links"] = linkCount
	report.Metrics["manuscripts.work_links_exact_ontology"] = exactCount
	report.Metrics["manuscripts.work_links_via_alias"] = aliasCount
	report.Metrics["manuscripts.work_links_unresolved"] = unresolvedCount
	report.Metrics["manuscripts.work_links_ambiguous"] = ambiguousCount
	report.Metrics["manuscripts.work_links_invalid_target_id"] = invalidIDCount
	report.Metrics["manuscripts.work_links_merged_interpretation"] = mergedCount
	addJoinIssues(groups, "manuscripts", "חיבורים - טור זמני", report)
	if rowNumbers := groups["missing-local-id"]; len(rowNumbers) > 0 {
		addIssue(report, SeverityWarning, "manuscript_missing_project_id", "manuscripts", "Midrash_MSS ID", "", rowNumbers, len(rowNumbers),
			"The project-local manuscript ID is absent; the system ID remains available as the canonical item ID.")
	}
}

func validateCreation(rows []csvRow, index workIndex, report *Report) {
	report.Metrics["work_creation.source_rows"] = countMeaningful(rows)
	groups := map[string][]int{}
	narratives, exactCount, aliasCount := 0, 0, 0
	unresolvedCount, ambiguousCount, invalidIDCount, mergedCount := 0, 0, 0, 0
	for _, row := range rows {
		if !meaningful(row.values) {
			continue
		}
		if value(row, "קטגוריה") == "מבוא" {
			report.Metrics["work_creation.context_rows"]++
			continue
		}
		narratives++
		title := value(row, "Title")
		if title == "" {
			addIssue(report, SeverityError, "creation_missing_work_title", "work_creation", "Title", "", []int{row.number}, 1,
				"A work-creation narrative requires a work title.")
			continue
		}
		if value(row, "זמן המדרש") == "" {
			addIssue(report, SeverityError, "creation_missing_narrative", "work_creation", "זמן המדרש", title, []int{row.number}, 1,
				"The raw geo-temporal formation narrative must be preserved.")
		}
		result := index.resolve(title)
		switch result.kind {
		case matchExact:
			exactCount++
		case matchAlias:
			aliasCount++
			groups["alias:"+title] = append(groups["alias:"+title], row.number)
		case matchMissing:
			unresolvedCount++
			groups["unresolved:"+title] = append(groups["unresolved:"+title], row.number)
		case matchAmbiguous:
			ambiguousCount++
			groups["ambiguous:"+title] = append(groups["ambiguous:"+title], row.number)
		case matchInvalidID:
			invalidIDCount++
			groups["invalid-id:"+title] = append(groups["invalid-id:"+title], row.number)
		case matchMerged:
			mergedCount++
			groups["merged:"+title] = append(groups["merged:"+title], row.number)
		}
	}
	report.Metrics["work_creation.narratives"] = narratives
	report.Metrics["work_creation.exact_ontology_joins"] = exactCount
	report.Metrics["work_creation.alias_joins"] = aliasCount
	report.Metrics["work_creation.unresolved_joins"] = unresolvedCount
	report.Metrics["work_creation.ambiguous_joins"] = ambiguousCount
	report.Metrics["work_creation.invalid_target_id_joins"] = invalidIDCount
	report.Metrics["work_creation.merged_interpretation_joins"] = mergedCount
	addJoinIssues(groups, "work_creation", "Title", report)
}

func validateEditions(rows []csvRow, index workIndex, decisions Decisions, report *Report) {
	report.Metrics["printed_editions.source_rows"] = countMeaningful(rows)
	groups := map[string][]int{}
	exactCount, aliasCount, derivedIDs := 0, 0, 0
	unresolvedCount, ambiguousCount, invalidIDCount, mergedCount := 0, 0, 0, 0
	seenIDs := map[string][]int{}
	for _, row := range rows {
		if !meaningful(row.values) {
			continue
		}
		title := value(row, "יצירה")
		if override, ok := decisions.editionWorkOverride(row.number); ok {
			title = override.WorkLabel
			addIssue(report, SeverityInfo, "edition_work_override_applied", "printed_editions", "יצירה", title, []int{row.number}, 1,
				"A curated work mapping was derived from the edition's bibliographic description.")
		}
		if title == "" {
			addIssue(report, SeverityError, "edition_missing_work_title", "printed_editions", "יצירה", value(row, "דפוס"), []int{row.number}, 1,
				"A printed edition must identify the work it embodies.")
		} else {
			result := index.resolve(title)
			switch result.kind {
			case matchExact:
				exactCount++
			case matchAlias:
				aliasCount++
				groups["alias:"+title] = append(groups["alias:"+title], row.number)
			case matchMissing:
				unresolvedCount++
				groups["unresolved:"+title] = append(groups["unresolved:"+title], row.number)
			case matchAmbiguous:
				ambiguousCount++
				groups["ambiguous:"+title] = append(groups["ambiguous:"+title], row.number)
			case matchInvalidID:
				invalidIDCount++
				groups["invalid-id:"+title] = append(groups["invalid-id:"+title], row.number)
			case matchMerged:
				mergedCount++
				groups["merged:"+title] = append(groups["merged:"+title], row.number)
			}
		}
		year := value(row, "year")
		if year == "" {
			year = value(row, " year")
		}
		if override, ok := decisions.editionYearOverride(row.number); ok {
			year = strconv.Itoa(override.CorrectedYear)
			addIssue(report, SeverityInfo, "edition_year_override_applied", "printed_editions", "year", year, []int{row.number}, 1,
				"A curated year correction resolves a conflict between the numeric year and the bibliographic description.")
		}
		parsedYear, err := strconv.Atoi(year)
		if err != nil || len(year) != 4 || parsedYear < 1400 || parsedYear > 2100 {
			addIssue(report, SeverityError, "edition_invalid_year", "printed_editions", "year", year, []int{row.number}, 1,
				"A publication event requires a plausible four-digit Gregorian year.")
		}
		if value(row, "סוג דפוס") == "" && value(row, "סוג דפוס ") == "" {
			groups["blank-type"] = append(groups["blank-type"], row.number)
		}
		if value(row, "סוג דפוס") == "כפולה" || value(row, "סוג דפוס ") == "כפולה" {
			groups["source-duplicate"] = append(groups["source-duplicate"], row.number)
		}
		stableID := stableEditionID(row)
		seenIDs[stableID] = append(seenIDs[stableID], row.number)
		derivedIDs++
	}
	for id, rowNumbers := range seenIDs {
		if len(rowNumbers) > 1 {
			addIssue(report, SeverityError, "duplicate_derived_edition_id", "printed_editions", "derived_id", id, rowNumbers, len(rowNumbers),
				"The deterministic edition identity is not unique.")
		}
	}
	report.Metrics["printed_editions.derived_item_ids"] = derivedIDs
	report.Metrics["printed_editions.exact_ontology_joins"] = exactCount
	report.Metrics["printed_editions.alias_joins"] = aliasCount
	report.Metrics["printed_editions.unresolved_joins"] = unresolvedCount
	report.Metrics["printed_editions.ambiguous_joins"] = ambiguousCount
	report.Metrics["printed_editions.invalid_target_id_joins"] = invalidIDCount
	report.Metrics["printed_editions.merged_interpretation_joins"] = mergedCount
	addJoinIssues(groups, "printed_editions", "יצירה", report)
	if rowNumbers := groups["blank-type"]; len(rowNumbers) > 0 {
		addIssue(report, SeverityWarning, "edition_missing_type", "printed_editions", "סוג דפוס", "", rowNumbers, len(rowNumbers),
			"The edition can be retained, but its subtype is not classified.")
	}
	if rowNumbers := groups["source-duplicate"]; len(rowNumbers) > 0 {
		if decisions.MarkedDuplicateEditions.InterpretAs == "reprint" && decisions.MarkedDuplicateEditions.Identity == "retain_separate_source_assertions" {
			addIssue(report, SeverityInfo, "edition_marked_reprint", "printed_editions", "סוג דפוס", "כפולה", rowNumbers, len(rowNumbers),
				"A curator decided to retain these as separate edition assertions and group them through a reprint-of relationship.")
		} else {
			addIssue(report, SeverityWarning, "edition_marked_duplicate", "printed_editions", "סוג דפוס", "כפולה", rowNumbers, len(rowNumbers),
				"The source marks these edition rows as duplicates; preprocessing must reconcile them before canonical export.")
		}
	}
}

func (index workIndex) resolve(label string) matchResult {
	key := normalize(label)
	candidates := distinctCandidates(index.byOntology[key])
	kind := matchExact
	if len(candidates) == 0 {
		candidates = distinctCandidates(index.byAlias[key])
		kind = matchAlias
	}
	if len(candidates) == 0 {
		return matchResult{kind: matchMissing}
	}
	if len(candidates) > 1 {
		if index.approvedMergedAssertions[key] {
			return matchResult{kind: matchMerged, candidates: candidates}
		}
		return matchResult{kind: matchAmbiguous, candidates: candidates}
	}
	if candidates[0].id == "" || index.duplicateIDs[candidates[0].id] {
		return matchResult{kind: matchInvalidID, candidates: candidates}
	}
	return matchResult{kind: kind, candidates: candidates}
}

func addJoinIssues(groups map[string][]int, sourceID, field string, report *Report) {
	keys := make([]string, 0, len(groups))
	for key := range groups {
		if strings.Contains(key, ":") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.SplitN(key, ":", 2)
		kind, label := parts[0], parts[1]
		rows := uniqueSorted(groups[key])
		switch kind {
		case "alias":
			addIssue(report, SeverityWarning, "noncanonical_work_join", sourceID, field, label, rows, len(groups[key]),
				"The work label resolves through an alias but does not exactly match the preferred ontology join key.")
		case "unresolved":
			addIssue(report, SeverityError, "unresolved_work_join", sourceID, field, label, rows, len(groups[key]),
				"The work label does not resolve to any ontology name or accepted alias.")
		case "ambiguous":
			addIssue(report, SeverityError, "ambiguous_work_join", sourceID, field, label, rows, len(groups[key]),
				"The work label resolves to more than one ontology row.")
		case "invalid-id":
			addIssue(report, SeverityError, "work_join_target_has_invalid_id", sourceID, field, label, rows, len(groups[key]),
				"The work label resolves, but its target has a missing or duplicated canonical ID.")
		case "merged":
			addIssue(report, SeverityInfo, "merged_interpretation_preserved", sourceID, field, label, rows, len(groups[key]),
				"The source interpreter merges multiple ontology-authoritative works; preserve this as a multi-work assertion without merging the canonical works.")
		}
	}
}

func addIssue(report *Report, severity Severity, code, sourceID, field, issueValue string, rows []int, count int, message string) {
	report.Issues = append(report.Issues, Issue{
		Severity: severity, Code: code, SourceID: sourceID, Field: field,
		Value: issueValue, Rows: uniqueSorted(rows), Count: count, Message: message,
	})
}

func readSource(id, path string) ([]csvRow, SourceSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, SourceSummary{}, fmt.Errorf("read %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\ufeff")))
	r.FieldsPerRecord = -1
	headers, err := r.Read()
	if err != nil {
		return nil, SourceSummary{}, fmt.Errorf("read headers from %s: %w", path, err)
	}
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}
	var rows []csvRow
	for rowNumber := 2; ; rowNumber++ {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, SourceSummary{}, fmt.Errorf("read %s row %d: %w", path, rowNumber, err)
		}
		values := map[string]string{}
		for i, raw := range record {
			if i >= len(headers) || headers[i] == "" {
				continue
			}
			values[headers[i]] = strings.TrimSpace(raw)
		}
		rows = append(rows, csvRow{number: rowNumber, values: values})
	}
	return rows, SourceSummary{
		ID: id, Path: path, SHA256: hex.EncodeToString(sum[:]),
		Rows: len(rows), MeaningfulRows: countMeaningful(rows),
	}, nil
}

func value(row csvRow, field string) string {
	return strings.TrimSpace(row.values[strings.TrimSpace(field)])
}

func meaningful(values map[string]string) bool {
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			return true
		}
	}
	return false
}

func countMeaningful(rows []csvRow) int {
	count := 0
	for _, row := range rows {
		if meaningful(row.values) {
			count++
		}
	}
	return count
}

func usableValue(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.HasPrefix(s, "<") && s != "לא נמצא"
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune(".;:,", r) })
	return strings.Join(strings.Fields(s), " ")
}

func splitLabels(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' })
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func appendUnique(values []string, candidate string) []string {
	for _, item := range values {
		if item == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func appendCandidate(values []*workCandidate, candidate *workCandidate) []*workCandidate {
	for _, item := range values {
		if item.row == candidate.row {
			return values
		}
	}
	return append(values, candidate)
}

func distinctCandidates(values []*workCandidate) []*workCandidate {
	seen := map[int]bool{}
	var out []*workCandidate
	for _, candidate := range values {
		if !seen[candidate.row] {
			seen[candidate.row] = true
			out = append(out, candidate)
		}
	}
	return out
}

func distinctCandidateRows(values []*workCandidate) map[int]bool {
	out := map[int]bool{}
	for _, candidate := range values {
		out[candidate.row] = true
	}
	return out
}

func uniqueSorted(values []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Ints(out)
	return out
}

func stableEditionID(row csvRow) string {
	parts := []string{value(row, "יצירה"), value(row, "דפוס"), value(row, "place"), value(row, "year")}
	if parts[3] == "" {
		parts[3] = value(row, " year")
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return "edition_" + hex.EncodeToString(sum[:])[:16]
}

func severityOrder(severity Severity) int {
	switch severity {
	case SeverityError:
		return 0
	case SeverityWarning:
		return 1
	default:
		return 2
	}
}
