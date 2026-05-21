package training

import (
	"bytes"
	"context"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseTrainingImportDryRunDeduplicatesAndWarns(t *testing.T) {
	var workbook bytes.Buffer
	file := excelize.NewFile()
	sheet := "Team "
	file.SetSheetName(file.GetSheetName(0), sheet)
	rows := [][]any{
		{"Dipendente", "Email", "Corso", "Anno"},
		{"Rossi Marco", "marco.rossi@example.com", "Terraform Associate", "2026"},
		{"Rossi Marco", "marco.rossi@example.com", "Terraform Associate", "2026"},
		{"Bianchi Laura", "", "Kubernetes", "2026"},
		{"", "", "Sicurezza", "2026"},
	}
	for rowIndex, row := range rows {
		for colIndex, value := range row {
			cell, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
			if err != nil {
				t.Fatalf("CoordinatesToCellName: %v", err)
			}
			if err := file.SetCellValue(sheet, cell, value); err != nil {
				t.Fatalf("SetCellValue: %v", err)
			}
		}
	}
	if err := file.Write(&workbook); err != nil {
		t.Fatalf("Write workbook: %v", err)
	}

	result, err := ParseTrainingImport(context.Background(), "training.xlsx", &workbook, false, nil, Principal{IsPeopleAdmin: true})
	if err != nil {
		t.Fatalf("ParseTrainingImport: %v", err)
	}
	if !result.DryRun {
		t.Fatal("expected dry-run response")
	}
	if result.Summary.CandidateRows != 2 {
		t.Fatalf("CandidateRows = %d, want 2", result.Summary.CandidateRows)
	}
	if result.Summary.SkippedRows != 1 {
		t.Fatalf("SkippedRows = %d, want 1", result.Summary.SkippedRows)
	}
	if result.Summary.AmbiguousRows != 1 {
		t.Fatalf("AmbiguousRows = %d, want 1", result.Summary.AmbiguousRows)
	}
	assertImportWarning(t, result.Warnings, "duplicate_candidate")
	assertImportWarning(t, result.Warnings, "employee_match_required")
	assertImportWarning(t, result.Warnings, "missing_employee")
}

func TestParseTrainingImportPrefersBudgetSheetDuplicate(t *testing.T) {
	var workbook bytes.Buffer
	file := excelize.NewFile()
	teamSheet := "Team "
	budgetSheet := "Per budget "
	file.SetSheetName(file.GetSheetName(0), teamSheet)
	writeImportRows(t, file, teamSheet, [][]any{
		{"Dipendente", "Email", "Corso", "Anno"},
		{"Rossi Marco", "marco.rossi@example.com", "Terraform Associate", "2026"},
	})
	if _, err := file.NewSheet(budgetSheet); err != nil {
		t.Fatalf("NewSheet: %v", err)
	}
	writeImportRows(t, file, budgetSheet, [][]any{
		{"Dipendente", "Email", "Corso", "Anno"},
		{"Marco Rossi", "marco.rossi@example.com", "Terraform Associate", "2026"},
	})
	if err := file.Write(&workbook); err != nil {
		t.Fatalf("Write workbook: %v", err)
	}

	result, err := ParseTrainingImport(context.Background(), "training.xlsx", &workbook, false, nil, Principal{IsPeopleAdmin: true})
	if err != nil {
		t.Fatalf("ParseTrainingImport: %v", err)
	}
	if result.Summary.CandidateRows != 1 {
		t.Fatalf("CandidateRows = %d, want 1", result.Summary.CandidateRows)
	}
	if got := result.Rows[0].Sheet; got != budgetSheet {
		t.Fatalf("Sheet = %q, want %q", got, budgetSheet)
	}
	if got := result.Rows[0].EmployeeName; got != "Marco Rossi" {
		t.Fatalf("EmployeeName = %q, want budget row", got)
	}
	assertImportWarning(t, result.Warnings, "duplicate_candidate")
}

func writeImportRows(t *testing.T, file *excelize.File, sheet string, rows [][]any) {
	t.Helper()
	for rowIndex, row := range rows {
		for colIndex, value := range row {
			cell, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
			if err != nil {
				t.Fatalf("CoordinatesToCellName: %v", err)
			}
			if err := file.SetCellValue(sheet, cell, value); err != nil {
				t.Fatalf("SetCellValue: %v", err)
			}
		}
	}
}

func assertImportWarning(t *testing.T, warnings []ImportWarning, code string) {
	t.Helper()
	for _, warning := range warnings {
		if warning.Code == code {
			return
		}
	}
	t.Fatalf("warning %q not found in %#v", code, warnings)
}
