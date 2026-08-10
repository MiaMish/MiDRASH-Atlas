// Package canonical defines the application-ready Midrash Atlas data contract.
// It contains no CSV, MARC, or source-specific parsing rules.
package canonical

import "midrash-atlas/packages/provenance"

const SchemaVersion = "2.0.0-alpha.1"

type Layer string

const (
	LayerWork Layer = "work"
	LayerItem Layer = "item"
	LayerText Layer = "text"
)

func (l Layer) Valid() bool {
	return l == LayerWork || l == LayerItem || l == LayerText
}

type Document struct {
	SchemaVersion string       `json:"schema_version"`
	LayerStates   []LayerState `json:"layer_states"`
	Works         []Work       `json:"works"`
	Items         []Item       `json:"items"`
	Texts         []Text       `json:"texts"`
	Events        []Event      `json:"events"`
	Places        []Place      `json:"places"`
	Agents        []Agent      `json:"agents"`
	Sources       []Source     `json:"sources"`
}

type LayerState struct {
	Layer  Layer  `json:"layer"`
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
}

type Work struct {
	ID          string       `json:"id"`
	Layer       Layer        `json:"layer"`
	Title       string       `json:"title"`
	HebrewTitle string       `json:"hebrew_title,omitempty"`
	Aliases     []string     `json:"aliases,omitempty"`
	Families    []string     `json:"families,omitempty"`
	Identifiers []Identifier `json:"identifiers,omitempty"`
	Evidence    []Evidence   `json:"evidence,omitempty"`
}

type ItemKind string

const (
	ItemManuscript     ItemKind = "manuscript"
	ItemPrintedEdition ItemKind = "printed_edition"
)

type Item struct {
	ID          string          `json:"id"`
	Layer       Layer           `json:"layer"`
	Kind        ItemKind        `json:"kind"`
	Title       string          `json:"title"`
	WorkLinks   []WorkLink      `json:"work_links"`
	Parts       []ItemPart      `json:"parts,omitempty"`
	Identifiers []Identifier    `json:"identifiers,omitempty"`
	Attributes  []ItemAttribute `json:"attributes,omitempty"`
	Evidence    []Evidence      `json:"evidence,omitempty"`
}

type ItemAttribute struct {
	Key      string     `json:"key"`
	Label    string     `json:"label"`
	Values   []string   `json:"values"`
	Evidence []Evidence `json:"evidence"`
}

type WorkLink struct {
	WorkID    string `json:"work_id"`
	Relation  string `json:"relation"`
	RawLabel  string `json:"raw_label,omitempty"`
	SourceRef string `json:"source_ref,omitempty"`
}

type ItemPart struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	Label    string     `json:"label"`
	Range    string     `json:"range,omitempty"`
	WorkIDs  []string   `json:"work_ids,omitempty"`
	Evidence []Evidence `json:"evidence,omitempty"`
}

type Text struct {
	ID       string        `json:"id"`
	Layer    Layer         `json:"layer"`
	WorkID   string        `json:"work_id"`
	ItemID   string        `json:"item_id,omitempty"`
	Citation string        `json:"citation"`
	Language string        `json:"language"`
	Content  string        `json:"content"`
	Status   string        `json:"status"`
	Mentions []TextMention `json:"mentions,omitempty"`
	Evidence []Evidence    `json:"evidence,omitempty"`
}

type MentionKind string

const (
	MentionPlace  MentionKind = "place"
	MentionPerson MentionKind = "person"
	MentionGroup  MentionKind = "group"
	MentionOther  MentionKind = "other"
)

type TextMention struct {
	ID         string      `json:"id"`
	Layer      Layer       `json:"layer"`
	Kind       MentionKind `json:"kind"`
	Surface    string      `json:"surface"`
	Start      int         `json:"start"`
	End        int         `json:"end"`
	EntityID   string      `json:"entity_id,omitempty"`
	Confidence string      `json:"confidence,omitempty"`
	Evidence   []Evidence  `json:"evidence,omitempty"`
}

type EntityRef struct {
	Layer Layer  `json:"layer"`
	ID    string `json:"id"`
}

type Event struct {
	ID          string               `json:"id"`
	Layer       Layer                `json:"layer"`
	Type        string               `json:"type"`
	Label       string               `json:"label"`
	Subject     EntityRef            `json:"subject"`
	Related     []RelatedEntity      `json:"related,omitempty"`
	Agents      []AgentParticipation `json:"agents,omitempty"`
	Places      []PlaceAssertion     `json:"places,omitempty"`
	Times       []TemporalAssertion  `json:"times,omitempty"`
	Evidence    []Evidence           `json:"evidence"`
	ReviewState string               `json:"review_state"`
}

type RelatedEntity struct {
	EntityRef
	Relation string `json:"relation"`
}

type AgentParticipation struct {
	AgentID string `json:"agent_id"`
	Role    string `json:"role"`
}

type PlaceAssertion struct {
	PlaceID    string     `json:"place_id,omitempty"`
	RawName    string     `json:"raw_name"`
	Role       string     `json:"role"`
	Confidence string     `json:"confidence,omitempty"`
	Evidence   []Evidence `json:"evidence"`
}

type TemporalAssertion struct {
	Value      TemporalValue `json:"value"`
	Confidence string        `json:"confidence,omitempty"`
	Evidence   []Evidence    `json:"evidence"`
}

type TemporalValue struct {
	Kind         string `json:"kind"`
	Display      string `json:"display"`
	Date         string `json:"date,omitempty"`
	Year         *int   `json:"year,omitempty"`
	StartYear    *int   `json:"start_year,omitempty"`
	EndYear      *int   `json:"end_year,omitempty"`
	Century      *int   `json:"century,omitempty"`
	StartCentury *int   `json:"start_century,omitempty"`
	EndCentury   *int   `json:"end_century,omitempty"`
	Part         string `json:"part,omitempty"`
	Approximate  bool   `json:"approximate"`
	Uncertain    bool   `json:"uncertain"`
}

type Place struct {
	ID              string           `json:"id"`
	Label           string           `json:"label"`
	Aliases         []string         `json:"aliases,omitempty"`
	Identifiers     []Identifier     `json:"identifiers,omitempty"`
	DisplayGeometry *DisplayGeometry `json:"display_geometry,omitempty"`
}

type DisplayGeometry struct {
	VariantID          string     `json:"variant_id"`
	Type               string     `json:"type"`
	Coordinates        []float64  `json:"coordinates"`
	SpatialPrecision   string     `json:"spatial_precision"`
	ReviewStatus       string     `json:"review_status"`
	InterpretationNote string     `json:"interpretation_note"`
	Evidence           []Evidence `json:"evidence"`
}

type Agent struct {
	ID          string       `json:"id"`
	Label       string       `json:"label"`
	Aliases     []string     `json:"aliases,omitempty"`
	Identifiers []Identifier `json:"identifiers,omitempty"`
}

type Identifier struct {
	Scheme string `json:"scheme"`
	Value  string `json:"value"`
}

type Source struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Label       string `json:"label"`
	Citation    string `json:"citation,omitempty"`
	URL         string `json:"url,omitempty"`
	RetrievedAt string `json:"retrieved_at,omitempty"`
}

type Evidence struct {
	SourceID     string             `json:"source_id"`
	SourceRecord string             `json:"source_record,omitempty"`
	Field        string             `json:"field,omitempty"`
	Raw          string             `json:"raw"`
	Extraction   string             `json:"extraction"`
	ReviewStatus string             `json:"review_status"`
	AIProvenance []provenance.AIRun `json:"ai_provenance,omitempty"`
}
