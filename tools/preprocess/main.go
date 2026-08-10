package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"midrash-atlas/packages/preprocess"
)

func main() {
	var cfg preprocess.Config
	var output string
	var canonicalOutput string
	var nliNormalized string
	var strict bool
	flag.StringVar(&cfg.HiburimCSV, "hiburim", "data/sources_new/list_of_hiburin_ontology.csv", "Hibbur ontology CSV")
	flag.StringVar(&cfg.HiburIDStubsCSV, "hibur-id-stubs", "data/sources_new/hibur_generated_stub_ids.csv", "temporary IDs for ontology rows with missing IDs")
	flag.StringVar(&cfg.StubHiburimCSV, "hibur-stubs", "data/sources_new/hiburim_from_manuscript_references.csv", "curated manuscript-reference hibur stubs CSV")
	flag.StringVar(&cfg.ManuscriptsCSV, "manuscripts", "data/sources_new/Midrash Mss.csv", "manuscript items CSV")
	flag.StringVar(&cfg.CreationCSV, "creation", "data/sources_new/mayim_all_categories_creation.csv", "work-creation narratives CSV")
	flag.StringVar(&cfg.EditionsCSV, "editions", "data/sources_new/mayim_all_categories_editions.csv", "printed-edition items CSV")
	flag.StringVar(&cfg.DecisionsJSON, "decisions", "configs/preprocess/curation-decisions.json", "auditable curation decisions JSON")
	flag.StringVar(&cfg.PilotPlacesJSON, "pilot-places", "configs/preprocess/pilot-place-display.json", "unreviewed pilot map display positions")
	flag.StringVar(&output, "output", "data/prepared/validation-report.json", "validation report path")
	flag.StringVar(&canonicalOutput, "canonical-output", "data/prepared/atlas-canonical.json", "canonical full-corpus atlas output path")
	flag.StringVar(&nliNormalized, "nli-normalized", "data/derived/nli_marc_normalized.json", "normalized NLI catalogue source")
	flag.BoolVar(&strict, "strict", false, "exit unsuccessfully when validation errors remain")
	flag.Parse()

	report, err := preprocess.Validate(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := writeJSON(output, report); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("validation %s: %d error groups (%d occurrences), %d warning groups (%d occurrences); wrote %s\n",
		report.Summary.Status,
		report.Summary.ErrorGroups, report.Summary.ErrorOccurrences,
		report.Summary.WarningGroups, report.Summary.WarningOccurrences,
		output)
	if strict && report.HasErrors() {
		os.Exit(1)
	}
	if report.HasErrors() {
		fmt.Printf("skipped canonical atlas export because validation errors remain\n")
		return
	}
	corpus, err := preprocess.BuildCorpus(cfg, nliNormalized)
	if err != nil {
		log.Fatal(err)
	}
	if err := writeJSON(canonicalOutput, corpus); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote canonical atlas with %d works, %d items, %d texts, and %d events to %s\n",
		len(corpus.Works), len(corpus.Items), len(corpus.Texts), len(corpus.Events), canonicalOutput)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
