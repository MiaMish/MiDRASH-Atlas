package preprocess

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateReportsLayerBoundaryProblems(t *testing.T) {
	dir := t.TempDir()
	write := func(name, contents string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cfg := Config{
		HiburimCSV:     write("works.csv", "אונטולוגיה,ID חדש?,Common English Title,ויקיפדיה\nמדרש א,work-1,Work A,כינוי א\nמדרש ב,work-1,Work B,\n,work-3,Work C,\n"),
		ManuscriptsCSV: write("manuscripts.csv", "Midrash_MSS ID,מספר מערכת,כינוי כתב היד בכתיב,חיבורים - טור זמני\n1,mms-1,Item,מדרש א; לא ידוע\n"),
		CreationCSV:    write("creation.csv", "קטגוריה,Title,זמן המדרש\nמדרש,Work C,במאה השלישית\n"),
		EditionsCSV:    write("editions.csv", "דפוס,סוג דפוס ,יצירה,place, year\nEdition,,מדרש א,Rome,1900\n"),
	}
	report, err := Validate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !report.HasErrors() {
		t.Fatal("expected validation errors")
	}
	wanted := map[string]bool{
		"duplicate_work_internal_id": false,
		"unresolved_work_join":       false,
		"noncanonical_work_join":     false,
		"edition_missing_type":       false,
	}
	for _, issue := range report.Issues {
		if _, ok := wanted[issue.Code]; ok {
			wanted[issue.Code] = true
		}
	}
	for code, found := range wanted {
		if !found {
			t.Errorf("missing expected issue %s", code)
		}
	}
}

func TestValidateAcceptsCleanSources(t *testing.T) {
	dir := t.TempDir()
	write := func(name, contents string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cfg := Config{
		HiburimCSV:     write("works.csv", "אונטולוגיה,ID חדש?,Common English Title\nמדרש א,work-1,Work A\n"),
		ManuscriptsCSV: write("manuscripts.csv", "Midrash_MSS ID,מספר מערכת,כינוי כתב היד בכתיב,חיבורים - טור זמני\n1,mms-1,Item,מדרש א\n"),
		CreationCSV:    write("creation.csv", "קטגוריה,Title,זמן המדרש\nמדרש,מדרש א,במאה השלישית\n"),
		EditionsCSV:    write("editions.csv", "דפוס,סוג דפוס ,יצירה,place, year\nEdition,דפוס חשוב,מדרש א,Rome,1900\n"),
	}
	report, err := Validate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Status != "valid" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestValidateAppliesCuratedSameWorkStubsAndReprintPolicy(t *testing.T) {
	dir := t.TempDir()
	write := func(name, contents string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cfg := Config{
		HiburimCSV:      write("works.csv", "אונטולוגיה,ID חדש?,Common English Title,רייזל\nמעשה חנוכה,150:T,Story of Hanukkah,\nאגדת חנוכה,150:T,Aggadat Hanukkah,\nאוצר המדרשים,work-a,Otzar,ספרי אסופות של מדרשים\nבתי מדרשות,work-b,Batei,ספרי אסופות של מדרשים\nספרי,,Sifrei,\n"),
		HiburIDStubsCSV: write("id-stubs.csv", "ontology_source_row,generated_stub_id,status\n6,stub:ontology:sifrei,stub_id_for_missing_ontology_id\n"),
		StubHiburimCSV:  write("stubs.csv", "stub_id,hebrew_label,common_english_title,status,comment\nstub:mss:x,תוכן לא מזוהה,Stub: unidentified,stub_created_from_manuscript_reference,Created from manuscript row 2\n"),
		ManuscriptsCSV:  write("manuscripts.csv", "Midrash_MSS ID,מספר מערכת,כינוי כתב היד בכתיב,חיבורים - טור זמני\n1,mms-1,Item,תוכן לא מזוהה\n"),
		CreationCSV:     write("creation.csv", "קטגוריה,Title,זמן המדרש\nמדרש,מעשה חנוכה,ימי הביניים\nמדרש,ספרי אסופות של מדרשים,ימי הביניים\n"),
		EditionsCSV:     write("editions.csv", "דפוס,סוג דפוס ,יצירה,place, year\nEdition,כפולה,אגדת חנוכה,Rome,1900\n"),
		DecisionsJSON: write("decisions.json", `{
			"schema_version":"1.0.0",
			"same_work_internal_ids":[{"internal_id":"150:T","decision":"same_work_alternate_titles"}],
			"merged_interpretations":[{"label":"ספרי אסופות של מדרשים","interpreter":"Reizel","target_work_ids":["work-a","work-b"],"resolution":"preserve_as_multi_work_assertion"}],
			"marked_duplicate_editions":{"interpret_as":"reprint","identity":"retain_separate_source_assertions","grouping":"reprint_of"}
		}`),
	}
	report, err := Validate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if report.HasErrors() {
		t.Fatalf("curated resolutions should not produce errors: %+v", report.Issues)
	}
	wanted := map[string]bool{
		"approved_same_work_rows":            false,
		"generated_ontology_stub_id_applied": false,
		"manuscript_reference_stubs_loaded":  false,
		"merged_interpretation_preserved":    false,
		"edition_marked_reprint":             false,
	}
	for _, issue := range report.Issues {
		if _, ok := wanted[issue.Code]; ok {
			wanted[issue.Code] = true
		}
	}
	for code, found := range wanted {
		if !found {
			t.Errorf("missing expected curated result %s", code)
		}
	}
}

func TestBuildCheckedInCanonicalCorpus(t *testing.T) {
	root := filepath.Join("..", "..")
	cfg := Config{
		HiburimCSV:      filepath.Join(root, "data/sources_new/list_of_hiburin_ontology.csv"),
		HiburIDStubsCSV: filepath.Join(root, "data/sources_new/hibur_generated_stub_ids.csv"),
		StubHiburimCSV:  filepath.Join(root, "data/sources_new/hiburim_from_manuscript_references.csv"),
		ManuscriptsCSV:  filepath.Join(root, "data/sources_new/Midrash Mss.csv"),
		CreationCSV:     filepath.Join(root, "data/sources_new/mayim_all_categories_creation.csv"),
		EditionsCSV:     filepath.Join(root, "data/sources_new/mayim_all_categories_editions.csv"),
		DecisionsJSON:   filepath.Join(root, "configs/preprocess/curation-decisions.json"),
		PilotPlacesJSON: filepath.Join(root, "configs/preprocess/pilot-place-display.json"),
	}
	doc, err := BuildCorpus(cfg, filepath.Join(root, "data/derived/nli_marc_normalized.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Works) != 175 {
		t.Fatalf("expected 167 ontology works and eight explicit manuscript-reference stubs, got %d", len(doc.Works))
	}
	if len(doc.Items) != 657 {
		t.Fatalf("expected 340 valid manuscript items and 317 editions, got %d", len(doc.Items))
	}
	if len(doc.Texts) != 0 {
		t.Fatalf("text layer should remain pending, got %d texts", len(doc.Texts))
	}
	if len(doc.Events) != 1110 {
		t.Fatalf("expected complete deterministic work/item event export, got %d", len(doc.Events))
	}
	if err := doc.ValidateStructure(); err != nil {
		t.Fatal(err)
	}
}
