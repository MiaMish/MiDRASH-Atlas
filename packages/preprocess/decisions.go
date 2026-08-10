package preprocess

import (
	"encoding/json"
	"fmt"
	"os"
)

type Decisions struct {
	SchemaVersion           string                  `json:"schema_version"`
	SameWorkInternalIDs     []SameWorkDecision      `json:"same_work_internal_ids"`
	ExcludedOntologyRows    []ExcludedOntologyRow   `json:"excluded_unused_ontology_rows"`
	MergedInterpretations   []MergedInterpretation  `json:"merged_interpretations"`
	ExcludedManuscriptRows  []ExcludedManuscriptRow `json:"excluded_manuscript_rows"`
	EditionWorkOverrides    []EditionWorkOverride   `json:"edition_work_overrides"`
	EditionYearOverrides    []EditionYearOverride   `json:"edition_year_overrides"`
	MarkedDuplicateEditions EditionDuplicatePolicy  `json:"marked_duplicate_editions"`
}

type SameWorkDecision struct {
	InternalID     string `json:"internal_id"`
	PreferredTitle string `json:"preferred_title"`
	Decision       string `json:"decision"`
	Comment        string `json:"comment"`
}

type EditionDuplicatePolicy struct {
	InterpretAs string `json:"interpret_as"`
	Identity    string `json:"identity"`
	Grouping    string `json:"grouping"`
	Comment     string `json:"comment"`
}

type ExcludedOntologyRow struct {
	Row    int    `json:"row"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

type MergedInterpretation struct {
	Label         string   `json:"label"`
	Interpreter   string   `json:"interpreter"`
	TargetWorkIDs []string `json:"target_work_ids"`
	Resolution    string   `json:"resolution"`
	Comment       string   `json:"comment"`
}

type ExcludedManuscriptRow struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

type EditionWorkOverride struct {
	Row            int    `json:"row"`
	RawDescription string `json:"raw_description"`
	WorkLabel      string `json:"work_label"`
	TargetWorkID   string `json:"target_work_id"`
	Reason         string `json:"reason"`
}

type EditionYearOverride struct {
	Row           int    `json:"row"`
	SourceYear    int    `json:"source_year"`
	CorrectedYear int    `json:"corrected_year"`
	Reason        string `json:"reason"`
}

func readDecisions(path string) (Decisions, error) {
	if path == "" {
		return Decisions{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Decisions{}, fmt.Errorf("read preprocessing decisions %s: %w", path, err)
	}
	var decisions Decisions
	if err := json.Unmarshal(data, &decisions); err != nil {
		return Decisions{}, fmt.Errorf("decode preprocessing decisions %s: %w", path, err)
	}
	return decisions, nil
}

func (d Decisions) approvedSharedID(id string) bool {
	for _, decision := range d.SameWorkInternalIDs {
		if decision.InternalID == id && decision.Decision == "same_work_alternate_titles" {
			return true
		}
	}
	return false
}

func (d Decisions) excludedOntologyRow(row int) bool {
	for _, decision := range d.ExcludedOntologyRows {
		if decision.Row == row {
			return true
		}
	}
	return false
}

func (d Decisions) excludedManuscriptRow(row int) bool {
	for _, decision := range d.ExcludedManuscriptRows {
		if decision.Row == row {
			return true
		}
	}
	return false
}

func (d Decisions) editionWorkOverride(row int) (EditionWorkOverride, bool) {
	for _, decision := range d.EditionWorkOverrides {
		if decision.Row == row {
			return decision, true
		}
	}
	return EditionWorkOverride{}, false
}

func (d Decisions) editionYearOverride(row int) (EditionYearOverride, bool) {
	for _, decision := range d.EditionYearOverrides {
		if decision.Row == row {
			return decision, true
		}
	}
	return EditionYearOverride{}, false
}
