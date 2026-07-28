package atlas

import (
	"encoding/json"

	"midrash-atlas/packages/provenance"
)

type Input struct {
	RequestedMMSIDs []string      `json:"requested_mms_ids"`
	Records         []InputRecord `json:"records"`
}

type InputRecord struct {
	MMSID                  string            `json:"mms_id"`
	RecordKind             string            `json:"record_kind"`
	ParentMMSIDs           []string          `json:"parent_mms_ids"`
	Title                  []string          `json:"title"`
	AlternativeTitles      []string          `json:"alternative_titles"`
	Extent                 []string          `json:"extent"`
	DateDisplay            []string          `json:"date_display"`
	ProductionPlaceDisplay []string          `json:"production_place_display"`
	Places                 []InputPlace      `json:"places"`
	Languages              []string          `json:"languages"`
	ScriptStyles           []string          `json:"script_styles"`
	ProvenanceNotes        []string          `json:"provenance_notes"`
	ColophonNotes          []string          `json:"colophon_notes"`
	PhysicalNotes          []string          `json:"physical_notes"`
	Contents               []string          `json:"contents"`
	GeneralNotes           []string          `json:"general_notes"`
	CurrentOwners          []InputOwner      `json:"current_owners"`
	Shelfmarks             []InputShelfmark  `json:"shelfmarks"`
	DigitalObjects         []json.RawMessage `json:"digital_objects"`
	DigitalServices        []json.RawMessage `json:"digital_services"`
	ResolverURL            string            `json:"resolver_url"`
	PublicRecordURL        string            `json:"public_record_url"`
	SourceModified         string            `json:"source_modified"`
	FixedField008          string            `json:"fixed_field_008"`
	MARC                   InputMARC         `json:"marc"`
}

type InputMARC struct {
	Fields []InputMARCField `json:"fields"`
}

type InputMARCField struct {
	Tag       string              `json:"tag"`
	Subfields []InputMARCSubfield `json:"subfields"`
}

type InputMARCSubfield struct {
	Code  string `json:"code"`
	Value string `json:"value"`
}

type InputPlace struct {
	Name        string  `json:"name"`
	Role        string  `json:"role"`
	Language    string  `json:"language"`
	AuthorityID *string `json:"authority_id"`
}

type InputOwner struct {
	Name     string   `json:"name"`
	Locality string   `json:"locality"`
	Country  string   `json:"country"`
	Roles    []string `json:"roles"`
	Language string   `json:"language"`
}

type InputShelfmark struct {
	Repository string `json:"repository"`
	Locality   string `json:"locality"`
	Country    string `json:"country"`
	Shelfmark  string `json:"shelfmark"`
	Language   string `json:"language"`
}

type AtlasRecord struct {
	ID                   string                 `json:"id"`
	PhysicalID           string                 `json:"physical_id"`
	Kind                 string                 `json:"kind"`
	ParentIDs            []string               `json:"parent_ids,omitempty"`
	ParentRecords        []CatalogRecordSummary `json:"parent_records,omitempty"`
	Titles               []string               `json:"titles"`
	PrimaryTitles        []string               `json:"primary_titles,omitempty"`
	AlternativeTitles    []string               `json:"alternative_titles,omitempty"`
	Languages            []string               `json:"languages,omitempty"`
	ScriptStyles         []string               `json:"script_styles,omitempty"`
	Extent               []string               `json:"extent,omitempty"`
	Dimensions           []string               `json:"dimensions,omitempty"`
	Contributors         []Contributor          `json:"contributors,omitempty"`
	Shelfmarks           []Shelfmark            `json:"shelfmarks,omitempty"`
	HiburLinks           []HiburLink            `json:"hibur_links,omitempty"`
	Digitized            bool                   `json:"digitized"`
	PublicRecordURL      string                 `json:"public_record_url,omitempty"`
	ResolverURL          string                 `json:"resolver_url,omitempty"`
	SourceModified       string                 `json:"source_modified,omitempty"`
	ProvenanceNotes      []string               `json:"provenance_notes,omitempty"`
	ColophonNotes        []string               `json:"colophon_notes,omitempty"`
	PhysicalNotes        []string               `json:"physical_notes,omitempty"`
	Contents             []string               `json:"contents,omitempty"`
	GeneralNotes         []string               `json:"general_notes,omitempty"`
	CurrentOwners        []Owner                `json:"current_owners,omitempty"`
	GeoAssertionIDs      []string               `json:"geo_assertion_ids,omitempty"`
	TemporalAssertionIDs []string               `json:"temporal_assertion_ids,omitempty"`
}

type CatalogRecordSummary struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	Titles            []string `json:"titles"`
	PrimaryTitles     []string `json:"primary_titles,omitempty"`
	AlternativeTitles []string `json:"alternative_titles,omitempty"`
}

type Contributor struct {
	Name    string `json:"name"`
	Role    string `json:"role,omitempty"`
	MARCTag string `json:"marc_tag"`
}

type Owner struct {
	Name     string   `json:"name"`
	Locality string   `json:"locality,omitempty"`
	Country  string   `json:"country,omitempty"`
	Roles    []string `json:"roles,omitempty"`
}

type Shelfmark struct {
	Repository string `json:"repository,omitempty"`
	Locality   string `json:"locality,omitempty"`
	Country    string `json:"country,omitempty"`
	Shelfmark  string `json:"shelfmark,omitempty"`
}

type Hibur struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	English     string   `json:"english,omitempty"`
	Family      []string `json:"family,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	OntologyKey string   `json:"ontology_key,omitempty"`
}

type HiburLink struct {
	HiburID     string `json:"hibur_id,omitempty"`
	RawLabel    string `json:"raw_label"`
	MatchStatus string `json:"match_status"`
}

type AssertionScope struct {
	TargetRecordID string `json:"target_record_id"`
	SourceRecordID string `json:"source_record_id"`
	PhysicalID     string `json:"physical_id"`
	Origin         string `json:"origin"` // direct | parent_fallback
}

type Evidence struct {
	Raw             string             `json:"raw"`
	MARCTag         string             `json:"marc_tag,omitempty"`
	MARCSubfield    string             `json:"marc_subfield,omitempty"`
	MARCRole        string             `json:"marc_role,omitempty"`
	Catalog         string             `json:"catalog"`
	CatalogRecordAt string             `json:"catalog_record_updated_at,omitempty"`
	Extraction      string             `json:"extraction"`
	ReviewStatus    string             `json:"review_status"`
	AIProvenance    []provenance.AIRun `json:"ai_provenance,omitempty"`
}

type GeoAssertion struct {
	ID           string         `json:"id"`
	SourceTypeID string         `json:"source_type_id"`
	Scope        AssertionScope `json:"scope"`
	PlaceID      string         `json:"place_id"`
	PlaceRaw     string         `json:"place_raw"`
	Role         string         `json:"role,omitempty"`
	Confidence   string         `json:"confidence"`
	Evidence     Evidence       `json:"evidence"`
}

type TemporalValue struct {
	Kind         string `json:"kind"` // year | range | century | century_range | unparsed
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

type TemporalAssertion struct {
	ID           string         `json:"id"`
	SourceTypeID string         `json:"source_type_id"`
	Scope        AssertionScope `json:"scope"`
	Value        TemporalValue  `json:"value"`
	Confidence   string         `json:"confidence"`
	Evidence     Evidence       `json:"evidence"`
}

type PlaceConcept struct {
	ID               string            `json:"id"`
	PreferredLabel   string            `json:"preferred_label"`
	Aliases          []string          `json:"aliases"`
	AuthorityIDs     []string          `json:"authority_ids,omitempty"`
	GeometryVariants []GeometryVariant `json:"geometry_variants"`
	ReviewStatus     string            `json:"review_status"`
}

type GeometryVariant struct {
	ID                   string          `json:"id"`
	Label                string          `json:"label"`
	Geometry             json.RawMessage `json:"geometry,omitempty"`
	ValidFromYear        *int            `json:"valid_from_year,omitempty"`
	ValidToYear          *int            `json:"valid_to_year,omitempty"`
	SpatialPrecision     string          `json:"spatial_precision"`
	InterpretationSource string          `json:"interpretation_source"`
	InterpretationNote   string          `json:"interpretation_note,omitempty"`
	Confidence           string          `json:"confidence"`
	ReviewStatus         string          `json:"review_status"`
}

type GeometrySeedDocument struct {
	Places []GeometrySeed `json:"places"`
}

type GeometrySeed struct {
	PlaceLabel string            `json:"place_label"`
	Aliases    []string          `json:"aliases,omitempty"`
	Variants   []GeometryVariant `json:"geometry_variants"`
}

type ReviewItem struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Raw      string `json:"raw"`
	RecordID string `json:"record_id,omitempty"`
	Reason   string `json:"reason"`
}

type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

type Feature struct {
	Type       string          `json:"type"`
	ID         string          `json:"id"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties any             `json:"properties"`
}
