package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"midrash-atlas/packages/atlas"
)

type atlasViewResponse struct {
	SchemaVersion string             `json:"schema_version"`
	TimeBounds    yearBounds         `json:"time_bounds"`
	Applied       atlasViewFilters   `json:"applied_filters"`
	Summary       atlasViewSummary   `json:"summary"`
	Facets        atlasViewFacets    `json:"facets"`
	Features      []atlasViewFeature `json:"features"`
}

type yearBounds struct {
	MinYear int `json:"min_year"`
	MaxYear int `json:"max_year"`
}

type atlasViewFilters struct {
	StartYear        *int     `json:"start_year,omitempty"`
	EndYear          *int     `json:"end_year,omitempty"`
	CircaYears       int      `json:"circa_years"`
	IncludeUndated   bool     `json:"include_undated"`
	GeoSourceIDs     []string `json:"geo_source_ids,omitempty"`
	Origins          []string `json:"origins,omitempty"`
	HiburIDs         []string `json:"hibur_ids,omitempty"`
	LocationStatuses []string `json:"location_statuses,omitempty"`
}

type atlasViewSummary struct {
	MatchedAssertions  int `json:"matched_assertions"`
	MappedAssertions   int `json:"mapped_assertions"`
	UnmappedAssertions int `json:"unmapped_assertions"`
	MappedPlaces       int `json:"mapped_places"`
	MatchedRecords     int `json:"matched_records"`
}

type atlasViewFacets struct {
	GeoSources       []sourceTypeView `json:"geo_sources"`
	Origins          []facetCount     `json:"origins"`
	LocationStatuses []facetCount     `json:"location_statuses"`
	Hiburim          []hiburFacet     `json:"hiburim"`
}

type sourceTypeView struct {
	ID               string `json:"id"`
	Label            string `json:"label"`
	ShortDescription string `json:"short_description"`
	DefaultEnabled   bool   `json:"default_enabled"`
	Count            int    `json:"count"`
}

type facetCount struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type hiburFacet struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	English string `json:"english,omitempty"`
	Count   int    `json:"count"`
}

type atlasViewFeature struct {
	Type       string                   `json:"type"`
	ID         string                   `json:"id"`
	Geometry   json.RawMessage          `json:"geometry"`
	Properties atlasViewPlaceProperties `json:"properties"`
}

type atlasViewPlaceProperties struct {
	PlaceID           string           `json:"place_id"`
	PlaceLabel        string           `json:"place_label"`
	CoordinateStatus  string           `json:"coordinate_status"`
	ReviewedByHuman   bool             `json:"reviewed_by_human"`
	ChangedByHuman    bool             `json:"changed_by_human"`
	SpatialPrecision  string           `json:"spatial_precision,omitempty"`
	GeometrySource    string           `json:"geometry_source,omitempty"`
	AIStatus          string           `json:"ai_status"`
	AssertionCount    int              `json:"assertion_count"`
	RecordCount       int              `json:"record_count"`
	ContainedPlaceIDs []string         `json:"contained_place_ids"`
	Events            []atlasViewEvent `json:"events"`
}

type atlasViewEvent struct {
	Assertion          atlas.GeoAssertion           `json:"assertion"`
	Record             atlas.AtlasRecord            `json:"record"`
	Hiburim            []atlas.Hibur                `json:"hiburim"`
	TemporalAssertions []atlasViewTemporalAssertion `json:"temporal_assertions"`
}

type atlasViewTemporalAssertion struct {
	atlas.TemporalAssertion
	EffectiveInterval *atlas.EffectiveInterval `json:"effective_interval,omitempty"`
}

type sourceTypeDocument struct {
	SourceTypes []struct {
		ID               string `json:"id"`
		Dimension        string `json:"dimension"`
		Label            string `json:"label"`
		ShortDescription string `json:"short_description"`
		DefaultEnabled   bool   `json:"default_enabled"`
	} `json:"source_types"`
}

func (s *server) atlasView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET required"))
		return
	}
	filters, err := parseAtlasFilters(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	response, err := s.buildAtlasView(r.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func parseAtlasFilters(r *http.Request) (atlasViewFilters, error) {
	filters := atlasViewFilters{
		CircaYears:       10,
		IncludeUndated:   true,
		GeoSourceIDs:     splitFilter(r.URL.Query().Get("geo-source")),
		Origins:          splitFilter(r.URL.Query().Get("origin")),
		HiburIDs:         splitFilter(r.URL.Query().Get("hibur-id")),
		LocationStatuses: splitFilter(r.URL.Query().Get("location-status")),
	}
	for name, target := range map[string]**int{
		"start-year": &filters.StartYear,
		"end-year":   &filters.EndYear,
	} {
		if raw := strings.TrimSpace(r.URL.Query().Get(name)); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				return filters, fmt.Errorf("%s must be an integer", name)
			}
			*target = &value
		}
	}
	if filters.StartYear != nil && filters.EndYear != nil && *filters.StartYear > *filters.EndYear {
		return filters, errors.New("start-year cannot be after end-year")
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("circa-years")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 || value > 100 {
			return filters, errors.New("circa-years must be between 0 and 100")
		}
		filters.CircaYears = value
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("include-undated")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return filters, errors.New("include-undated must be true or false")
		}
		filters.IncludeUndated = value
	}
	return filters, nil
}

func (s *server) buildAtlasView(ctx context.Context, filters atlasViewFilters) (atlasViewResponse, error) {
	var recordDocument struct {
		Records []atlas.AtlasRecord `json:"records"`
	}
	var geoDocument struct {
		Assertions []atlas.GeoAssertion `json:"assertions"`
	}
	var temporalDocument struct {
		Assertions []atlas.TemporalAssertion `json:"assertions"`
	}
	var hiburDocument struct {
		Hiburim []atlas.Hibur `json:"hiburim"`
	}
	var sourceDocument sourceTypeDocument
	for path, target := range map[string]any{
		"records.json":             &recordDocument,
		"geo_assertions.json":      &geoDocument,
		"temporal_assertions.json": &temporalDocument,
		"hiburim.json":             &hiburDocument,
		"source_types.json":        &sourceDocument,
	} {
		if err := readJSONFile(filepath.Join(s.cfg.ExportDir, path), target); err != nil {
			return atlasViewResponse{}, err
		}
	}
	locations, err := s.buildLocationOverview(ctx)
	if err != nil {
		return atlasViewResponse{}, err
	}

	recordsByID := make(map[string]atlas.AtlasRecord, len(recordDocument.Records))
	for _, record := range recordDocument.Records {
		recordsByID[record.ID] = record
	}
	hiburByID := make(map[string]atlas.Hibur, len(hiburDocument.Hiburim))
	for _, hibur := range hiburDocument.Hiburim {
		hiburByID[hibur.ID] = hibur
	}
	temporalByTarget := map[string][]atlasViewTemporalAssertion{}
	bounds := yearBounds{}
	for _, assertion := range temporalDocument.Assertions {
		item := atlasViewTemporalAssertion{TemporalAssertion: assertion}
		if interval, ok := atlas.ApplyTemporalPolicy(assertion.Value, filters.CircaYears); ok {
			item.EffectiveInterval = &interval
			if bounds.MinYear == 0 || interval.StartYear < bounds.MinYear {
				bounds.MinYear = interval.StartYear
			}
			if interval.EndYear > bounds.MaxYear {
				bounds.MaxYear = interval.EndYear
			}
		}
		temporalByTarget[assertion.Scope.TargetRecordID] =
			append(temporalByTarget[assertion.Scope.TargetRecordID], item)
	}
	locationByID := make(map[string]locationOverview, len(locations))
	for _, location := range locations {
		locationByID[location.PlaceID] = location
	}

	sourceCounts := map[string]int{}
	originCounts := map[string]int{}
	statusCounts := map[string]int{}
	hiburCounts := map[string]int{}
	type placeGroup struct {
		location locationOverview
		events   []atlasViewEvent
		records  map[string]bool
	}
	groups := map[string]*placeGroup{}
	matchedRecords := map[string]bool{}
	response := atlasViewResponse{
		SchemaVersion: "1.0.0", TimeBounds: bounds, Applied: filters,
		Features: []atlasViewFeature{},
		Facets: atlasViewFacets{
			GeoSources:       []sourceTypeView{},
			Origins:          []facetCount{},
			LocationStatuses: []facetCount{},
			Hiburim:          []hiburFacet{},
		},
	}
	for _, assertion := range geoDocument.Assertions {
		sourceCounts[assertion.SourceTypeID]++
		originCounts[assertion.Scope.Origin]++
		record, ok := recordsByID[assertion.Scope.TargetRecordID]
		if !ok {
			continue
		}
		for _, link := range record.HiburLinks {
			hiburCounts[link.HiburID]++
		}
		location := locationByID[assertion.PlaceID]
		statusCounts[location.CoordinateStatus]++
		if !matchesSet(assertion.SourceTypeID, filters.GeoSourceIDs) ||
			!matchesSet(assertion.Scope.Origin, filters.Origins) ||
			!matchesSet(location.CoordinateStatus, filters.LocationStatuses) ||
			!recordMatchesHibur(record, filters.HiburIDs) {
			continue
		}
		temporal := temporalByTarget[record.ID]
		if temporal == nil {
			temporal = []atlasViewTemporalAssertion{}
		}
		if !matchesTime(temporal, filters) {
			continue
		}
		response.Summary.MatchedAssertions++
		matchedRecords[record.ID] = true
		if !location.HasValidCoordinates {
			response.Summary.UnmappedAssertions++
			continue
		}
		response.Summary.MappedAssertions++
		group := groups[assertion.PlaceID]
		if group == nil {
			group = &placeGroup{location: location, records: map[string]bool{}}
			groups[assertion.PlaceID] = group
		}
		hiburim := []atlas.Hibur{}
		for _, link := range record.HiburLinks {
			if hibur, ok := hiburByID[link.HiburID]; ok {
				hiburim = append(hiburim, hibur)
			}
		}
		group.events = append(group.events, atlasViewEvent{
			Assertion: assertion, Record: record, Hiburim: hiburim,
			TemporalAssertions: temporal,
		})
		group.records[record.ID] = true
	}
	response.Summary.MatchedRecords = len(matchedRecords)
	response.Summary.MappedPlaces = len(groups)
	for _, group := range groups {
		sort.Slice(group.events, func(i, j int) bool {
			return firstTitle(group.events[i].Record) < firstTitle(group.events[j].Record)
		})
		response.Features = append(response.Features, atlasViewFeature{
			Type: "Feature", ID: group.location.PlaceID, Geometry: group.location.Geometry,
			Properties: atlasViewPlaceProperties{
				PlaceID: group.location.PlaceID, PlaceLabel: group.location.PlaceLabel,
				CoordinateStatus: group.location.CoordinateStatus,
				ReviewedByHuman:  group.location.ReviewedByHuman,
				ChangedByHuman:   group.location.ChangedByHuman,
				SpatialPrecision: group.location.SpatialPrecision,
				GeometrySource:   group.location.GeometrySource,
				AIStatus:         group.location.AIStatus,
				AssertionCount:   len(group.events), RecordCount: len(group.records),
				ContainedPlaceIDs: []string{},
				Events:            group.events,
			},
		})
	}
	sort.Slice(response.Features, func(i, j int) bool {
		return response.Features[i].Properties.PlaceLabel < response.Features[j].Properties.PlaceLabel
	})
	decodedGeometries := make([]decodedGeometry, len(response.Features))
	validGeometries := make([]bool, len(response.Features))
	for index := range response.Features {
		decodedGeometries[index], validGeometries[index] =
			decodeGeometry(response.Features[index].Geometry)
	}
	for containerIndex := range response.Features {
		if !validGeometries[containerIndex] {
			continue
		}
		for candidateIndex := range response.Features {
			if containerIndex == candidateIndex || !validGeometries[candidateIndex] {
				continue
			}
			if decodedGeometryWithin(
				decodedGeometries[candidateIndex],
				decodedGeometries[containerIndex],
			) {
				response.Features[containerIndex].Properties.ContainedPlaceIDs = append(
					response.Features[containerIndex].Properties.ContainedPlaceIDs,
					response.Features[candidateIndex].Properties.PlaceID,
				)
			}
		}
	}
	for _, source := range sourceDocument.SourceTypes {
		if source.Dimension != "geo" {
			continue
		}
		response.Facets.GeoSources = append(response.Facets.GeoSources, sourceTypeView{
			ID: source.ID, Label: source.Label, ShortDescription: source.ShortDescription,
			DefaultEnabled: source.DefaultEnabled, Count: sourceCounts[source.ID],
		})
	}
	sort.Slice(response.Facets.GeoSources, func(i, j int) bool {
		return response.Facets.GeoSources[i].Label < response.Facets.GeoSources[j].Label
	})
	for id, count := range originCounts {
		response.Facets.Origins = append(response.Facets.Origins, facetCount{
			ID: id, Label: strings.ReplaceAll(id, "_", " "), Count: count,
		})
	}
	for id, count := range statusCounts {
		response.Facets.LocationStatuses = append(response.Facets.LocationStatuses, facetCount{
			ID: id, Label: strings.ReplaceAll(id, "_", " "), Count: count,
		})
	}
	for id, count := range hiburCounts {
		hibur, ok := hiburByID[id]
		if !ok {
			continue
		}
		response.Facets.Hiburim = append(response.Facets.Hiburim, hiburFacet{
			ID: id, Label: hibur.Label, English: hibur.English, Count: count,
		})
	}
	sort.Slice(response.Facets.Hiburim, func(i, j int) bool {
		return response.Facets.Hiburim[i].Label < response.Facets.Hiburim[j].Label
	})
	sort.Slice(response.Facets.Origins, func(i, j int) bool {
		return response.Facets.Origins[i].ID < response.Facets.Origins[j].ID
	})
	sort.Slice(response.Facets.LocationStatuses, func(i, j int) bool {
		return response.Facets.LocationStatuses[i].ID < response.Facets.LocationStatuses[j].ID
	})
	return response, nil
}

func splitFilter(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}

func matchesSet(value string, selected []string) bool {
	if len(selected) == 0 {
		return true
	}
	for _, item := range selected {
		if item == value {
			return true
		}
	}
	return false
}

func recordMatchesHibur(record atlas.AtlasRecord, selected []string) bool {
	if len(selected) == 0 {
		return true
	}
	for _, link := range record.HiburLinks {
		if matchesSet(link.HiburID, selected) {
			return true
		}
	}
	return false
}

func matchesTime(assertions []atlasViewTemporalAssertion, filters atlasViewFilters) bool {
	hasDated := false
	for _, assertion := range assertions {
		if assertion.EffectiveInterval == nil {
			continue
		}
		hasDated = true
		interval := assertion.EffectiveInterval
		if filters.StartYear != nil && interval.EndYear < *filters.StartYear {
			continue
		}
		if filters.EndYear != nil && interval.StartYear > *filters.EndYear {
			continue
		}
		return true
	}
	return !hasDated && filters.IncludeUndated
}

func firstTitle(record atlas.AtlasRecord) string {
	if len(record.Titles) > 0 {
		return record.Titles[0]
	}
	return record.ID
}
