package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"midrash-atlas/apps/api/internal/curation"
	"midrash-atlas/apps/api/internal/curationpilot"
	"midrash-atlas/apps/api/internal/gazetteerpilot"
	"midrash-atlas/apps/api/internal/httpapi"
	"midrash-atlas/apps/api/internal/locationstore"
	"midrash-atlas/packages/atlas"
	"midrash-atlas/packages/gazetteer"
	"midrash-atlas/packages/llm"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "export":
		export(os.Args[2:])
	case "serve":
		serve(os.Args[2:])
	case "gazetteer-pilot":
		gazetteerPilot(os.Args[2:])
	case "curation-pilot":
		curationPilot(os.Args[2:])
	default:
		usage()
	}
}

func curationPilot(args []string) {
	fs := flag.NewFlagSet("curation-pilot", flag.ExitOnError)
	cfg := curationpilot.Config{}
	var placeIDs string
	fs.StringVar(&cfg.InputPath, "input", "data/derived/gazetteer/nominatim-pilot.json", "cached gazetteer pilot")
	fs.StringVar(&cfg.OutputPath, "output", "data/derived/gazetteer/nominatim-pilot-drafts.json", "LLM draft output")
	fs.StringVar(&cfg.Provider, "provider", "", "LLM provider (default: CURATION_LLM_PROVIDER or ollama)")
	fs.StringVar(&cfg.Model, "model", "", "LLM model (default: CURATION_LLM_MODEL or qwen3.5:35b)")
	fs.StringVar(&placeIDs, "place-ids", "", "optional comma-separated place IDs")
	_ = fs.Parse(args)
	loadDotEnv(".env")
	cfg.Provider = firstNonempty(cfg.Provider, os.Getenv("CURATION_LLM_PROVIDER"), llm.ProviderOllama)
	cfg.Model = firstNonempty(cfg.Model, os.Getenv("CURATION_LLM_MODEL"), "qwen3.5:35b")
	if strings.TrimSpace(placeIDs) != "" {
		cfg.PlaceIDs = strings.Split(placeIDs, ",")
	}
	llmClient := llm.NewClient(llm.Config{
		OpenAIAPIKey:  os.Getenv("OPENAI_API_KEY"),
		OllamaBaseURL: os.Getenv("OLLAMA_BASE_URL"),
	})
	generator := curation.NewGenerator(llmClient, curation.Config{
		DefaultProvider: cfg.Provider,
		DefaultModel:    cfg.Model,
	})
	output, err := curationpilot.Run(context.Background(), generator, cfg)
	if err != nil {
		log.Fatal(err)
	}
	successes := 0
	for _, draft := range output.Drafts {
		if draft.Result != nil {
			successes++
		}
	}
	fmt.Printf("Curation pilot wrote %d/%d successful drafts to %s\n", successes, len(output.Drafts), cfg.OutputPath)
}

func gazetteerPilot(args []string) {
	fs := flag.NewFlagSet("gazetteer-pilot", flag.ExitOnError)
	cfg := gazetteerpilot.Config{}
	var baseURL, userAgent string
	fs.StringVar(&cfg.PilotConfigPath, "config", "configs/atlas/gazetteer_pilot.json", "pilot selection")
	fs.StringVar(&cfg.PlacesPath, "places", "data/generated/atlas/places.json", "places export")
	fs.StringVar(&cfg.GeoAssertionsPath, "geo-assertions", "data/generated/atlas/geo_assertions.json", "geo assertions export")
	fs.StringVar(&cfg.TemporalPath, "temporal-assertions", "data/generated/atlas/temporal_assertions.json", "temporal assertions export")
	fs.StringVar(&cfg.CacheDir, "cache", "data/raw/gazetteer/nominatim", "raw response cache")
	fs.StringVar(&cfg.OutputPath, "output", "data/derived/gazetteer/nominatim-pilot.json", "derived pilot output")
	fs.StringVar(&baseURL, "base-url", "https://nominatim.openstreetmap.org", "Nominatim base URL")
	fs.StringVar(&userAgent, "user-agent", "midrash-atlas-poc/0.1 (manuscript research; cached curation pilot)", "identifying User-Agent")
	_ = fs.Parse(args)
	client, err := gazetteer.NewNominatim(gazetteer.NominatimConfig{
		BaseURL: baseURL, CacheDir: cfg.CacheDir, UserAgent: userAgent,
	})
	if err != nil {
		log.Fatal(err)
	}
	output, err := gazetteerpilot.Run(context.Background(), client, cfg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Gazetteer pilot wrote %d places to %s\n", len(output.Places), cfg.OutputPath)
}

func export(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	cfg := atlas.ExportConfig{}
	fs.StringVar(&cfg.InputJSON, "input", "data/derived/nli_marc_normalized.json", "normalized NLI JSON")
	fs.StringVar(&cfg.ManuscriptsCSV, "manuscripts", "data/source/midrashim_from_google_sheets.csv", "manuscript project CSV")
	fs.StringVar(&cfg.HiburimCSV, "hiburim", "data/source/hiburim_from_google_sheets.csv", "Hibur ontology CSV")
	fs.StringVar(&cfg.SourceTypes, "source-types", "configs/atlas/source_types.json", "source type definitions")
	fs.StringVar(&cfg.PlaceGeometries, "place-geometries", "configs/atlas/place_geometries.json", "curated contextual geometry variants")
	fs.StringVar(&cfg.OutputDir, "output", "data/generated/atlas", "output directory")
	_ = fs.Parse(args)
	if err := atlas.Export(cfg); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Atlas export written to %s\n", cfg.OutputDir)
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	var dir, addr, databasePath, gazetteerPilotPath, curationDraftsPath string
	fs.StringVar(&dir, "dir", "data/generated/atlas", "export directory")
	fs.StringVar(&addr, "addr", "127.0.0.1:8080", "listen address")
	fs.StringVar(&databasePath, "database", "data/runtime/atlas.sqlite", "SQLite audit database")
	fs.StringVar(&gazetteerPilotPath, "gazetteer-pilot", "data/derived/gazetteer/nominatim-pilot.json", "cached gazetteer candidates")
	fs.StringVar(&curationDraftsPath, "curation-drafts", "data/derived/gazetteer/nominatim-pilot-drafts.json", "LLM curation drafts")
	_ = fs.Parse(args)
	loadDotEnv(".env")
	store, err := locationstore.OpenSQLite(databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	llmClient := llm.NewClient(llm.Config{
		OpenAIAPIKey:  os.Getenv("OPENAI_API_KEY"),
		OllamaBaseURL: os.Getenv("OLLAMA_BASE_URL"),
	})
	generator := curation.NewGenerator(llmClient, curation.Config{
		DefaultProvider: firstNonempty(os.Getenv("CURATION_LLM_PROVIDER"), llm.ProviderOllama),
		DefaultModel:    firstNonempty(os.Getenv("CURATION_LLM_MODEL"), "qwen3.5:35b"),
	})
	mux := httpapi.New(httpapi.Config{
		ExportDir: dir, GazetteerPilotPath: gazetteerPilotPath, CurationDraftsPath: curationDraftsPath,
	}, store, generator)
	log.Printf("serving atlas API on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: atlas <export|serve|gazetteer-pilot|curation-pilot> [options]")
	os.Exit(2)
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		key = strings.TrimSpace(key)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		_ = os.Setenv(key, value)
	}
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
