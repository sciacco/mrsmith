package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/sciacco/mrsmith/internal/binocolo"
	"github.com/sciacco/mrsmith/internal/platform/config"
	"github.com/sciacco/mrsmith/internal/platform/database"
	"github.com/sciacco/mrsmith/internal/platform/llm"
)

func main() {
	var (
		sampleSize  = flag.Int("sample-size", 10, "numero aziende ready da selezionare dal DB")
		iterations  = flag.Int("iterations", 10, "iterazioni LLM per azienda")
		concurrency = flag.Int("concurrency", 10, "richieste LLM parallele")
		outDir      = flag.String("out", "", "directory output artefatti (default artifacts/llm-evals/ma-deep-brief/...)")
		modelID     = flag.String("model-id", "", "uuid modello mrsmith.llm_model; vuoto = default dello scope ma_deep_brief")
		promptID    = flag.String("prompt-id", "", "uuid prompt mrsmith.llm_prompt; vuoto = default dello scope ma_deep_brief")
		dsn         = flag.String("dsn", "", "ANISETTA_DSN override; vuoto = env/config")
		manifest    = flag.String("manifest", "", "case_manifest.json da riusare per lo stesso campione")
		rescore     = flag.String("rescore", "", "outputs.jsonl esistente da rivalutare senza chiamate LLM")
		companyKeys = flag.String("company-keys", "", "company_key comma-separated; alternativa manuale al manifest")
		dryRun      = flag.Bool("dry-run", false, "scrive manifest/input senza chiamare l'LLM")
	)
	flag.Parse()

	cfg := config.Load()
	if *dsn == "" {
		*dsn = cfg.AnisettaDSN
	}
	if *dsn == "" {
		log.Fatal("ANISETTA_DSN non configurato (usa --dsn o env ANISETTA_DSN)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.New(database.Config{Driver: "postgres", DSN: *dsn})
	if err != nil {
		log.Fatalf("connect anisetta: %v", err)
	}
	defer db.Close()

	if strings.TrimSpace(*rescore) != "" {
		manifestPath := resolveEvalArtifactPath(*manifest)
		if strings.TrimSpace(manifestPath) == "" {
			log.Fatal("--manifest è obbligatorio con --rescore")
		}
		report, err := binocolo.RescoreMADeepBriefEval(ctx, db, manifestPath, resolveEvalArtifactPath(*rescore), resolveEvalArtifactPath(*outDir))
		if err != nil {
			log.Fatal(err)
		}
		printReport(report)
		return
	}

	keys, err := evalCompanyKeys(*manifest, *companyKeys)
	if err != nil {
		log.Fatal(err)
	}

	report, err := binocolo.RunMADeepBriefEval(ctx, db, llm.New(db), binocolo.MADeepBriefEvalOptions{
		SampleSize:  *sampleSize,
		Iterations:  *iterations,
		Concurrency: *concurrency,
		OutputDir:   *outDir,
		ModelID:     *modelID,
		PromptID:    *promptID,
		CompanyKeys: keys,
		DryRun:      *dryRun,
		Progress:    os.Stderr,
	})
	if err != nil {
		log.Fatal(err)
	}

	printReport(report)
}

func printReport(report *binocolo.MADeepBriefEvalReport) {
	fmt.Printf("Experiment: %s\n", report.ExperimentID)
	fmt.Printf("Output dir: %s\n", report.OutputDir)
	fmt.Printf("Cases: %d\n", report.Cases)
	if report.DryRun {
		fmt.Println("Dry-run: nessuna chiamata LLM eseguita")
	} else {
		fmt.Printf("Runs: %d\n", report.Runs)
	}
	fmt.Printf("Manifest: %s\n", report.ManifestPath)
	if !report.DryRun && report.OutputsPath != "" {
		fmt.Printf("Outputs: %s\n", report.OutputsPath)
	}
	if report.SummaryCSVPath != "" {
		fmt.Printf("Summary: %s\n", report.SummaryCSVPath)
	}
	if report.DashboardPath != "" {
		fmt.Printf("Dashboard: %s\n", report.DashboardPath)
	}
}

func evalCompanyKeys(manifestPath, csvKeys string) ([]string, error) {
	manifestPath = resolveEvalArtifactPath(manifestPath)
	var keys []string
	if strings.TrimSpace(csvKeys) != "" {
		for _, part := range strings.Split(csvKeys, ",") {
			if key := strings.TrimSpace(part); key != "" {
				keys = append(keys, key)
			}
		}
	}
	if strings.TrimSpace(manifestPath) == "" {
		return keys, nil
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var doc struct {
		Cases []struct {
			CompanyKey string `json:"companyKey"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	for _, c := range doc.Cases {
		if key := strings.TrimSpace(c.CompanyKey); key != "" {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func resolveEvalArtifactPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if strings.HasPrefix(path, "artifacts/") {
		candidate := "../" + path
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return path
}
