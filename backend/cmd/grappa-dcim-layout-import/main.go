package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/sciacco/mrsmith/internal/grappadcim"
	"github.com/sciacco/mrsmith/internal/platform/database"
)

func main() {
	code, err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "grappa-dcim-layout-import: %v\n", err)
	}
	os.Exit(code)
}

func run() (int, error) {
	var (
		sourcePath   = flag.String("source", "artifacts/mappe/totali.json", "sorgente mappe layout-grid-v1")
		dsn          = flag.String("dsn", os.Getenv("GRAPPA_DSN"), "DSN MySQL Grappa; default da GRAPPA_DSN")
		reportPath   = flag.String("report", "", "percorso report JSON opzionale; '-' scrive JSON su stdout")
		reportMDPath = flag.String("report-md", "", "percorso report Markdown opzionale; se vuoto e --report termina con .json, usa lo stesso basename .md")
		apply        = flag.Bool("apply", false, "scrive sul database; senza questo flag esegue dry-run")
		dryRunFlag   = flag.Bool("dry-run", false, "esplicita il dry-run; e' il default")
	)
	flag.Parse()

	if *apply && *dryRunFlag {
		return 1, errors.New("usare --apply oppure --dry-run, non entrambi")
	}
	if strings.TrimSpace(*dsn) == "" {
		return 1, errors.New("GRAPPA_DSN o --dsn e' obbligatorio")
	}

	resolvedSource, err := resolveSourcePath(*sourcePath)
	if err != nil {
		return 1, err
	}

	db, err := database.New(database.Config{Driver: "mysql", DSN: *dsn})
	if err != nil {
		return 1, err
	}
	defer db.Close()

	dryRun := !*apply
	report, importErr := grappadcim.RunLayoutGridImport(context.Background(), db, grappadcim.LayoutGridImportOptions{
		SourcePath:  resolvedSource,
		SourceLabel: filepath.ToSlash(strings.TrimSpace(*sourcePath)),
		DryRun:      dryRun,
	})

	printSummary(report)

	jsonPath := strings.TrimSpace(*reportPath)
	mdPath := strings.TrimSpace(*reportMDPath)
	if mdPath == "" {
		mdPath = defaultMarkdownReportPath(jsonPath)
	}
	if jsonPath != "" {
		if err := writeJSONReport(jsonPath, report); err != nil {
			return 1, err
		}
		if jsonPath != "-" {
			fmt.Fprintf(os.Stdout, "report JSON: %s\n", jsonPath)
		}
	}
	if mdPath != "" {
		if err := writeMarkdownReport(mdPath, report); err != nil {
			return 1, err
		}
		fmt.Fprintf(os.Stdout, "report Markdown: %s\n", mdPath)
	}

	if importErr != nil {
		return 1, importErr
	}
	return report.RecommendedExitCode, nil
}

func resolveSourcePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "artifacts/mappe/totali.json"
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat source: %w", err)
	}
	if !filepath.IsAbs(path) {
		alt := filepath.Join("..", path)
		if _, err := os.Stat(alt); err == nil {
			return alt, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat source: %w", err)
		}
	}
	return "", fmt.Errorf("source not found: %s", path)
}

func printSummary(report grappadcim.LayoutGridImportReport) {
	mode := report.Mode
	if mode == "" {
		mode = "dry-run"
	}
	if !report.OK {
		fmt.Fprintf(os.Stdout, "layout-grid %s: errore: %s\n", mode, report.Error)
		if report.Summary.DatacentersInSource > 0 {
			fmt.Fprintf(os.Stdout, "sorgente letta: %d datacenter\n", report.Summary.DatacentersInSource)
		}
		return
	}
	fmt.Fprintf(os.Stdout,
		"layout-grid %s: datacenter %d/%d risolti, blocchi insert/update/unchanged %d/%d/%d, warning %d\n",
		mode,
		report.Summary.DatacentersImported,
		report.Summary.DatacentersInSource,
		report.Summary.BlocksInserted,
		report.Summary.BlocksUpdated,
		report.Summary.BlocksUnchanged,
		report.Summary.Warnings,
	)
	if report.Summary.DatacentersUnresolved > 0 {
		fmt.Fprintf(os.Stdout, "datacenter non risolti: %d\n", report.Summary.DatacentersUnresolved)
	}
	if report.Summary.MissingPositionCells > 0 || report.Summary.PlenumsUnlinked > 0 {
		fmt.Fprintf(os.Stdout, "celle incomplete: posizioni %d, plenum %d\n", report.Summary.MissingPositionCells, report.Summary.PlenumsUnlinked)
	}
	if report.RecommendedExitCode == 2 {
		fmt.Fprintln(os.Stdout, "exit consigliato: 2 (warning da verificare)")
	}
}

func writeJSONReport(path string, report grappadcim.LayoutGridImportReport) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	payload = append(payload, '\n')
	if path == "-" {
		_, err := os.Stdout.Write(payload)
		return err
	}
	if err := ensureParentDir(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write report JSON: %w", err)
	}
	return nil
}

func writeMarkdownReport(path string, report grappadcim.LayoutGridImportReport) error {
	payload := []byte(renderMarkdownReport(report))
	if err := ensureParentDir(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write report Markdown: %w", err)
	}
	return nil
}

func defaultMarkdownReportPath(jsonPath string) string {
	jsonPath = strings.TrimSpace(jsonPath)
	if jsonPath == "" || jsonPath == "-" {
		return ""
	}
	ext := filepath.Ext(jsonPath)
	if strings.EqualFold(ext, ".json") {
		return strings.TrimSuffix(jsonPath, ext) + ".md"
	}
	return jsonPath + ".md"
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	return nil
}

func renderMarkdownReport(report grappadcim.LayoutGridImportReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Grappa DCIM Layout Grid Import Report\n\n")
	fmt.Fprintf(&b, "- Source: `%s`\n", report.Source)
	fmt.Fprintf(&b, "- Mode: `%s`\n", report.Mode)
	fmt.Fprintf(&b, "- Started: `%s`\n", report.StartedAt)
	fmt.Fprintf(&b, "- Finished: `%s`\n", report.FinishedAt)
	fmt.Fprintf(&b, "- Importer: `%s`\n", report.ImporterVersion)
	fmt.Fprintf(&b, "- Input checksum: `%s`\n", report.InputChecksum)
	fmt.Fprintf(&b, "- Recommended exit code: `%d`\n", report.RecommendedExitCode)
	if report.Error != "" {
		fmt.Fprintf(&b, "- Error: `%s`\n", report.Error)
	}
	fmt.Fprintf(&b, "\n## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n|---|---:|\n")
	fmt.Fprintf(&b, "| Datacenter in source | %d |\n", report.Summary.DatacentersInSource)
	fmt.Fprintf(&b, "| Datacenter resolved | %d |\n", report.Summary.DatacentersImported)
	fmt.Fprintf(&b, "| Datacenter unresolved | %d |\n", report.Summary.DatacentersUnresolved)
	fmt.Fprintf(&b, "| Blocks in source | %d |\n", report.Summary.BlocksInSource)
	fmt.Fprintf(&b, "| Blocks inserted | %d |\n", report.Summary.BlocksInserted)
	fmt.Fprintf(&b, "| Blocks updated | %d |\n", report.Summary.BlocksUpdated)
	fmt.Fprintf(&b, "| Blocks unchanged | %d |\n", report.Summary.BlocksUnchanged)
	fmt.Fprintf(&b, "| Blocks with unresolved islet | %d |\n", report.Summary.BlocksWithUnresolvedIslet)
	fmt.Fprintf(&b, "| Position cells | %d |\n", report.Summary.PositionCells)
	fmt.Fprintf(&b, "| Missing position cells | %d |\n", report.Summary.MissingPositionCells)
	fmt.Fprintf(&b, "| Plenum cells | %d |\n", report.Summary.PlenumCells)
	fmt.Fprintf(&b, "| Plenums linked | %d |\n", report.Summary.PlenumsLinked)
	fmt.Fprintf(&b, "| Plenums unlinked | %d |\n", report.Summary.PlenumsUnlinked)
	fmt.Fprintf(&b, "| Warnings | %d |\n", report.Summary.Warnings)

	fmt.Fprintf(&b, "\n## Datacenters\n\n")
	fmt.Fprintf(&b, "| Source | Type | Status | DB ID | Blocks |\n|---|---|---|---:|---:|\n")
	for _, dc := range report.Datacenters {
		id := ""
		if dc.DatacenterID != nil {
			id = fmt.Sprintf("%d", *dc.DatacenterID)
		}
		fmt.Fprintf(&b, "| %s | %s → %s | %s | %s | %d/%d |\n", mdEscape(dc.SourceName), mdEscape(dc.SourceType), mdEscape(dc.NormalizedType), mdEscape(dc.Status), id, dc.BlocksProcessed, dc.BlocksInSource)
	}

	if len(report.Warnings) > 0 {
		fmt.Fprintf(&b, "\n## Warnings\n\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "- %s\n", mdEscape(warning))
		}
	}

	fmt.Fprintf(&b, "\n## Blocks\n\n")
	for _, dc := range report.Datacenters {
		if len(dc.Blocks) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n", mdEscape(dc.SourceName))
		fmt.Fprintf(&b, "| Order | Block | Islet | Action | Position cells | Missing positions | Plenums linked | Plenums unlinked |\n")
		fmt.Fprintf(&b, "|---:|---|---|---|---:|---:|---:|---:|\n")
		for _, block := range dc.Blocks {
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %d | %d | %d | %d |\n",
				block.DisplayOrder,
				mdEscape(block.Title),
				mdEscape(block.IsletName),
				mdEscape(block.Action),
				block.PositionCells,
				len(block.MissingPositions),
				len(block.LinkedPlenums),
				len(block.UnlinkedPlenums),
			)
		}
		fmt.Fprintf(&b, "\n")
	}
	return b.String()
}

func mdEscape(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
