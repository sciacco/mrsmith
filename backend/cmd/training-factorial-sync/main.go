// Command training-factorial-sync esegue da riga di comando la run completa
// di sincronizzazione Factorial (#141, slice 7/8 = #149): stesso pattern del
// vecchio comando CSV di bootstrap, rimosso (dry-run default, --commit
// esplicito, exit code non zero su run fallita, report JSON opzionale).
// Config da env di processo, mai da backend/.env. Nessun retry: il retry
// operativo e' la run successiva.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/sciacco/mrsmith/internal/platform/database"
	"github.com/sciacco/mrsmith/internal/platform/directory/factorialdir"
	"github.com/sciacco/mrsmith/internal/training"
	"github.com/sciacco/mrsmith/pkg/factorial"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "training-factorial-sync: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		dsn        = flag.String("dsn", os.Getenv("ANISETTA_DSN"), "DSN PostgreSQL Anisetta; default da ANISETTA_DSN")
		reportPath = flag.String("report", "", "percorso report JSON opzionale")
		commit     = flag.Bool("commit", false, "scrive su database e Factorial; senza questo flag esegue dry-run")
		dryRunFlag = flag.Bool("dry-run", false, "esplicita il dry-run; e' il default")
	)
	flag.Parse()

	if *commit && *dryRunFlag {
		return errors.New("usare --commit oppure --dry-run, non entrambi")
	}
	if *dsn == "" {
		return errors.New("ANISETTA_DSN o --dsn e' obbligatorio")
	}
	apiKey := os.Getenv("FACTORIAL_API_KEY")
	if apiKey == "" {
		return errors.New("FACTORIAL_API_KEY e' obbligatorio")
	}
	technicalEmployeeID := os.Getenv("FACTORIAL_TRAINING_AUTHOR_EMPLOYEE_ID")
	if technicalEmployeeID == "" {
		return errors.New("FACTORIAL_TRAINING_AUTHOR_EMPLOYEE_ID e' obbligatorio")
	}

	db, err := database.New(database.Config{Driver: "postgres", DSN: *dsn})
	if err != nil {
		return err
	}
	defer db.Close()
	store := training.NewSQLStore(db)

	factorialOpts := []factorial.Option{
		factorial.WithAPIKey(apiKey),
		factorial.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
	}
	if baseURL := os.Getenv("FACTORIAL_BASE_URL"); baseURL != "" {
		factorialOpts = append(factorialOpts, factorial.WithBaseURL(baseURL))
	}
	factorialCli := factorial.New(factorialOpts...)
	directoryProvider := factorialdir.New(factorialCli)

	deps := training.FactorialSyncDeps{
		Directory:           directoryProvider,
		Factorial:           factorialCli,
		TechnicalEmployeeID: technicalEmployeeID,
	}
	report, runErr := store.RunFactorialSync(context.Background(), deps, "cli", !*commit)
	printSummary(report)

	if *reportPath != "" {
		if writeErr := writeReport(*reportPath, report); writeErr != nil {
			if runErr != nil {
				return runErr
			}
			return writeErr
		}
		fmt.Fprintf(os.Stdout, "report: %s\n", *reportPath)
	}
	return runErr
}

func printSummary(report training.FactorialSyncRun) {
	mode := "dry-run"
	if !report.DryRun {
		mode = "commit"
	}
	fmt.Fprintf(os.Stdout, "factorial sync %s: outcome=%s duration=%dms sessions_without_class=%d\n",
		mode, report.Outcome, report.DurationMS, report.SessionsWithoutClass)
	fmt.Fprintf(os.Stdout, "  tombstone: access_destroyed=%d sessions_deleted=%d reset=%d warnings=%d\n",
		report.Tombstone.AccessDestroyed, report.Tombstone.SessionsDeleted, report.Tombstone.ResetCount, len(report.Tombstone.Warnings))
	rdaExceptions := fmt.Sprintf("%d", report.Inbound.RDAExceptions)
	if report.DryRun {
		rdaExceptions = "n/d"
	}
	fmt.Fprintf(os.Stdout, "  inbound: conflicts=%d warnings=%d rda_exceptions=%s\n",
		len(report.Inbound.Conflicts), len(report.Inbound.Warnings), rdaExceptions)
	fmt.Fprintf(os.Stdout, "  outbound: conflicts=%d warnings=%d planned=%d\n",
		len(report.Outbound.Conflicts), len(report.Outbound.Warnings), len(report.Outbound.Planned))
	if report.Error != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n", report.Error)
	}
}

func writeReport(path string, report training.FactorialSyncRun) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	payload = append(payload, '\n')
	if path == "-" {
		_, err := os.Stdout.Write(payload)
		return err
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}
