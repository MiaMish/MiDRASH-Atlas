package canonical

import (
	"fmt"
	"strings"
)

func (d Document) ValidateStructure() error {
	if d.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version must be %q", SchemaVersion)
	}
	seen := map[string]string{}
	workIDs, itemIDs, textIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	placeIDs, agentIDs, sourceIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, source := range d.Sources {
		if strings.TrimSpace(source.ID) == "" {
			return fmt.Errorf("source id is required")
		}
		if sourceIDs[source.ID] {
			return fmt.Errorf("duplicate source id %q", source.ID)
		}
		sourceIDs[source.ID] = true
	}
	for _, place := range d.Places {
		if strings.TrimSpace(place.ID) == "" || strings.TrimSpace(place.Label) == "" {
			return fmt.Errorf("place id and label are required")
		}
		placeIDs[place.ID] = true
		if geometry := place.DisplayGeometry; geometry != nil {
			if geometry.Type != "Point" || len(geometry.Coordinates) != 2 || strings.TrimSpace(geometry.VariantID) == "" || strings.TrimSpace(geometry.ReviewStatus) == "" {
				return fmt.Errorf("place %q has invalid display geometry", place.ID)
			}
		}
	}
	for _, agent := range d.Agents {
		if strings.TrimSpace(agent.ID) == "" || strings.TrimSpace(agent.Label) == "" {
			return fmt.Errorf("agent id and label are required")
		}
		agentIDs[agent.ID] = true
	}
	layerStates := map[Layer]bool{}
	for _, state := range d.LayerStates {
		if !state.Layer.Valid() {
			return fmt.Errorf("layer state has invalid layer %q", state.Layer)
		}
		if layerStates[state.Layer] {
			return fmt.Errorf("duplicate layer state %q", state.Layer)
		}
		if state.Status != "ready" && state.Status != "partial" && state.Status != "pending" {
			return fmt.Errorf("layer %q has invalid status %q", state.Layer, state.Status)
		}
		layerStates[state.Layer] = true
	}
	check := func(kind, id string, layer, expected Layer) error {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("%s id is required", kind)
		}
		if layer != expected {
			return fmt.Errorf("%s %q layer must be %q", kind, id, expected)
		}
		key := string(layer) + ":" + id
		if previous, ok := seen[key]; ok {
			return fmt.Errorf("duplicate %s id %q also used by %s", kind, id, previous)
		}
		seen[key] = kind
		return nil
	}
	for _, work := range d.Works {
		if err := check("work", work.ID, work.Layer, LayerWork); err != nil {
			return err
		}
		if strings.TrimSpace(work.Title) == "" {
			return fmt.Errorf("work %q title is required", work.ID)
		}
		workIDs[work.ID] = true
	}
	for _, item := range d.Items {
		if err := check("item", item.ID, item.Layer, LayerItem); err != nil {
			return err
		}
		if item.Kind != ItemManuscript && item.Kind != ItemPrintedEdition {
			return fmt.Errorf("item %q has invalid kind %q", item.ID, item.Kind)
		}
		if strings.TrimSpace(item.Title) == "" {
			return fmt.Errorf("item %q title is required", item.ID)
		}
		itemIDs[item.ID] = true
	}
	for _, passage := range d.Texts {
		if err := check("text", passage.ID, passage.Layer, LayerText); err != nil {
			return err
		}
		if strings.TrimSpace(passage.WorkID) == "" {
			return fmt.Errorf("text %q work_id is required", passage.ID)
		}
		if passage.Status == "ready" && strings.TrimSpace(passage.Content) == "" {
			return fmt.Errorf("ready text %q content is required", passage.ID)
		}
		textIDs[passage.ID] = true
		for _, mention := range passage.Mentions {
			if mention.Layer != LayerText {
				return fmt.Errorf("text mention %q layer must be %q", mention.ID, LayerText)
			}
		}
	}
	for _, item := range d.Items {
		if item.WorkLinks == nil {
			return fmt.Errorf("item %q work_links must be an array, even when empty", item.ID)
		}
		for _, link := range item.WorkLinks {
			if !workIDs[link.WorkID] {
				return fmt.Errorf("item %q links unknown work %q", item.ID, link.WorkID)
			}
		}
		for _, part := range item.Parts {
			for _, workID := range part.WorkIDs {
				if !workIDs[workID] {
					return fmt.Errorf("item part %q links unknown work %q", part.ID, workID)
				}
			}
		}
		for _, attribute := range item.Attributes {
			if strings.TrimSpace(attribute.Key) == "" || strings.TrimSpace(attribute.Label) == "" || len(attribute.Values) == 0 {
				return fmt.Errorf("item %q has invalid attribute", item.ID)
			}
		}
	}
	for _, passage := range d.Texts {
		if !workIDs[passage.WorkID] {
			return fmt.Errorf("text %q links unknown work %q", passage.ID, passage.WorkID)
		}
		if passage.ItemID != "" && !itemIDs[passage.ItemID] {
			return fmt.Errorf("text %q links unknown item %q", passage.ID, passage.ItemID)
		}
	}
	entityExists := func(ref EntityRef) bool {
		switch ref.Layer {
		case LayerWork:
			return workIDs[ref.ID]
		case LayerItem:
			return itemIDs[ref.ID]
		case LayerText:
			return textIDs[ref.ID]
		default:
			return false
		}
	}
	eventIDs := map[string]bool{}
	for _, event := range d.Events {
		if strings.TrimSpace(event.ID) == "" {
			return fmt.Errorf("event id is required")
		}
		if eventIDs[event.ID] {
			return fmt.Errorf("duplicate event id %q", event.ID)
		}
		eventIDs[event.ID] = true
		if !event.Layer.Valid() {
			return fmt.Errorf("event %q has invalid layer %q", event.ID, event.Layer)
		}
		if event.Subject.Layer != event.Layer {
			return fmt.Errorf("event %q layer %q differs from subject layer %q", event.ID, event.Layer, event.Subject.Layer)
		}
		if strings.TrimSpace(event.Subject.ID) == "" {
			return fmt.Errorf("event %q subject id is required", event.ID)
		}
		if !entityExists(event.Subject) {
			return fmt.Errorf("event %q has unknown subject %s:%s", event.ID, event.Subject.Layer, event.Subject.ID)
		}
		for _, related := range event.Related {
			if strings.TrimSpace(related.Relation) == "" || !entityExists(related.EntityRef) {
				return fmt.Errorf("event %q has invalid related entity %s:%s", event.ID, related.Layer, related.ID)
			}
		}
		for _, participation := range event.Agents {
			if !agentIDs[participation.AgentID] {
				return fmt.Errorf("event %q links unknown agent %q", event.ID, participation.AgentID)
			}
		}
		for _, assertion := range event.Places {
			if assertion.PlaceID != "" && !placeIDs[assertion.PlaceID] {
				return fmt.Errorf("event %q links unknown place %q", event.ID, assertion.PlaceID)
			}
		}
		if len(event.Evidence) == 0 {
			return fmt.Errorf("event %q must preserve evidence", event.ID)
		}
	}
	checkEvidence := func(context string, evidence []Evidence) error {
		for _, item := range evidence {
			if !sourceIDs[item.SourceID] {
				return fmt.Errorf("%s evidence links unknown source %q", context, item.SourceID)
			}
			if strings.TrimSpace(item.Raw) == "" {
				return fmt.Errorf("%s evidence raw value is required", context)
			}
		}
		return nil
	}
	for _, work := range d.Works {
		if err := checkEvidence("work "+work.ID, work.Evidence); err != nil {
			return err
		}
	}
	for _, item := range d.Items {
		if err := checkEvidence("item "+item.ID, item.Evidence); err != nil {
			return err
		}
		for _, part := range item.Parts {
			if err := checkEvidence("item part "+part.ID, part.Evidence); err != nil {
				return err
			}
		}
		for _, attribute := range item.Attributes {
			if err := checkEvidence("item attribute "+item.ID+":"+attribute.Key, attribute.Evidence); err != nil {
				return err
			}
		}
	}
	for _, passage := range d.Texts {
		if err := checkEvidence("text "+passage.ID, passage.Evidence); err != nil {
			return err
		}
		for _, mention := range passage.Mentions {
			if err := checkEvidence("text mention "+mention.ID, mention.Evidence); err != nil {
				return err
			}
		}
	}
	for _, event := range d.Events {
		if err := checkEvidence("event "+event.ID, event.Evidence); err != nil {
			return err
		}
		for _, place := range event.Places {
			if err := checkEvidence("event place "+event.ID, place.Evidence); err != nil {
				return err
			}
		}
		for _, temporal := range event.Times {
			if err := checkEvidence("event time "+event.ID, temporal.Evidence); err != nil {
				return err
			}
		}
	}
	for _, place := range d.Places {
		if place.DisplayGeometry != nil {
			if err := checkEvidence("place display geometry "+place.ID, place.DisplayGeometry.Evidence); err != nil {
				return err
			}
		}
	}
	return nil
}
