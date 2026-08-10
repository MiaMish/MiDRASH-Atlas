package canonical

import "testing"

func TestValidateStructureAcceptsLayeredDocument(t *testing.T) {
	doc := Document{
		SchemaVersion: SchemaVersion,
		Sources:       []Source{{ID: "source-1", Type: "test", Label: "Test source"}},
		Works:         []Work{{ID: "work-1", Layer: LayerWork, Title: "Work"}},
		Items:         []Item{{ID: "item-1", Layer: LayerItem, Kind: ItemManuscript, Title: "Item", WorkLinks: []WorkLink{}}},
		Texts:         []Text{{ID: "text-1", Layer: LayerText, WorkID: "work-1", Citation: "1:1"}},
		Events: []Event{{
			ID: "event-1", Layer: LayerItem, Type: "acquisition",
			Subject:  EntityRef{Layer: LayerItem, ID: "item-1"},
			Evidence: []Evidence{{SourceID: "source-1", Raw: "raw", Extraction: "manual"}},
		}},
	}
	if err := doc.ValidateStructure(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateStructureRejectsCrossLayerEventSubject(t *testing.T) {
	doc := Document{
		SchemaVersion: SchemaVersion,
		Events: []Event{{
			ID: "event-1", Layer: LayerWork, Type: "formation",
			Subject:  EntityRef{Layer: LayerItem, ID: "item-1"},
			Evidence: []Evidence{{SourceID: "source-1", Raw: "raw", Extraction: "manual"}},
		}},
	}
	if err := doc.ValidateStructure(); err == nil {
		t.Fatal("expected cross-layer subject error")
	}
}
