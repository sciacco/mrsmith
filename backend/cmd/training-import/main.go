package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/sciacco/mrsmith/internal/platform/database"
	"github.com/sciacco/mrsmith/internal/training"
)

type importReport struct {
	OK         bool                             `json:"ok"`
	DryRun     bool                             `json:"dryRun"`
	StartedAt  string                           `json:"startedAt"`
	FinishedAt string                           `json:"finishedAt"`
	Error      string                           `json:"error,omitempty"`
	Employees  *training.EmployeeImportResponse `json:"employees,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "training-import: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		employeesCSV = flag.String("employees-csv", "", "CSV dipendenti con colonne Nome,Cognome,Email")
		dsn          = flag.String("dsn", os.Getenv("ANISETTA_DSN"), "DSN PostgreSQL Anisetta; default da ANISETTA_DSN")
		operator     = flag.String("operator", "training-import@localhost", "email operatore usata per autorizzazione/audit applicativo")
		reportPath   = flag.String("report", "", "percorso report JSON opzionale")
		commit       = flag.Bool("commit", false, "scrive sul database; senza questo flag esegue dry-run")
		dryRunFlag   = flag.Bool("dry-run", false, "esplicita il dry-run; e' il default")
	)
	flag.Parse()

	if *commit && *dryRunFlag {
		return errors.New("usare --commit oppure --dry-run, non entrambi")
	}
	if *employeesCSV == "" {
		return errors.New("specificare --employees-csv")
	}
	if *commit && *dsn == "" {
		return errors.New("ANISETTA_DSN o --dsn e' obbligatorio con --commit")
	}

	ctx := context.Background()
	var store *training.SQLStore
	var closeDB func() error
	if *dsn != "" {
		db, err := database.New(database.Config{Driver: "postgres", DSN: *dsn})
		if err != nil {
			return err
		}
		store = training.NewSQLStore(db)
		closeDB = db.Close
	}
	if closeDB != nil {
		defer closeDB()
	}

	principal := training.Principal{
		Subject:       "training-import-cli",
		Email:         *operator,
		Name:          "Training Import CLI",
		Roles:         []string{"people"},
		IsPeopleAdmin: true,
	}
	dryRun := !*commit
	report := importReport{
		OK:        true,
		DryRun:    dryRun,
		StartedAt: time.Now().Format(time.RFC3339),
	}

	file, err := os.Open(*employeesCSV)
	if err != nil {
		return fmt.Errorf("open employees csv: %w", err)
	}
	result, err := training.ParseEmployeeCSVImport(ctx, *employeesCSV, file, *commit, store, principal)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close employees csv: %w", closeErr)
	}
	report.Employees = &result
	printEmployeeSummary(result)

	report.FinishedAt = time.Now().Format(time.RFC3339)
	if *reportPath != "" {
		if err := writeReport(*reportPath, report); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "report: %s\n", *reportPath)
	}
	return nil
}

func printEmployeeSummary(result training.EmployeeImportResponse) {
	mode := "dry-run"
	if !result.DryRun {
		mode = "commit"
	}
	fmt.Fprintf(
		os.Stdout,
		"employees %s: %d candidate, %d skipped, %d invalid, %d duplicate",
		mode,
		result.Summary.CandidateRows,
		result.Summary.SkippedRows,
		result.Summary.InvalidRows,
		result.Summary.DuplicateRows,
	)
	if !result.DryRun || employeeWriteCounts(result.Summary) > 0 {
		fmt.Fprintf(
			os.Stdout,
			", %d created, %d updated, %d unchanged",
			result.Summary.CreatedEmployees,
			result.Summary.UpdatedEmployees,
			result.Summary.UnchangedEmployees,
		)
	}
	fmt.Fprintln(os.Stdout)
	if len(result.Warnings) > 0 {
		fmt.Fprintf(os.Stdout, "employees warnings: %d\n", len(result.Warnings))
	}
}

func employeeWriteCounts(summary training.EmployeeImportSummary) int {
	return summary.CreatedEmployees + summary.UpdatedEmployees + summary.UnchangedEmployees
}

func writeReport(path string, report importReport) error {
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
