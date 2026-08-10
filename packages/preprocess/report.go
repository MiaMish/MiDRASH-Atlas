package preprocess

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

type SourceSummary struct {
	ID             string `json:"id"`
	Path           string `json:"path"`
	SHA256         string `json:"sha256"`
	Rows           int    `json:"rows"`
	MeaningfulRows int    `json:"meaningful_rows"`
}

type Issue struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	SourceID string   `json:"source_id"`
	Field    string   `json:"field,omitempty"`
	Value    string   `json:"value,omitempty"`
	Rows     []int    `json:"rows,omitempty"`
	Count    int      `json:"count"`
	Message  string   `json:"message"`
}

type Summary struct {
	Status             string `json:"status"`
	ErrorGroups        int    `json:"error_groups"`
	WarningGroups      int    `json:"warning_groups"`
	InfoGroups         int    `json:"info_groups"`
	ErrorOccurrences   int    `json:"error_occurrences"`
	WarningOccurrences int    `json:"warning_occurrences"`
	InfoOccurrences    int    `json:"info_occurrences"`
}

type Report struct {
	SchemaVersion string          `json:"schema_version"`
	Summary       Summary         `json:"summary"`
	Sources       []SourceSummary `json:"sources"`
	Metrics       map[string]int  `json:"metrics"`
	Issues        []Issue         `json:"issues"`
}

func (r Report) HasErrors() bool { return r.Summary.ErrorGroups > 0 }
