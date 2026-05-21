package training

import "testing"

func TestResolveTrainingImportEmployeesMatchesCompoundFirstNamesByFirstToken(t *testing.T) {
	response := ImportDryRunResponse{
		Rows: []ImportRow{
			{Sheet: "Per budget ", Row: 26, EmployeeName: "Eva Grimaldi", CourseTitle: "ISO 14001", Year: 2026, Status: "candidate"},
			{Sheet: "Per budget ", Row: 27, EmployeeName: "Sofia Manzoni", CourseTitle: "ISO 14001", Year: 2026, Status: "candidate"},
		},
	}
	employees := []EmployeeImportRow{
		{FirstName: "Eva Maria", LastName: "Grimaldi", Email: "eva.grimaldi@example.com", Status: "candidate"},
		{FirstName: "Sofia Maria", LastName: "Manzoni", Email: "sofia.manzoni@example.com", Status: "candidate"},
	}

	report := ResolveTrainingImportEmployees(&response, employees)

	if report.MatchedRows != 2 {
		t.Fatalf("MatchedRows = %d, want 2", report.MatchedRows)
	}
	if report.UnmatchedRows != 0 {
		t.Fatalf("UnmatchedRows = %d, want 0", report.UnmatchedRows)
	}
	if response.Rows[0].EmployeeEmail != "eva.grimaldi@example.com" {
		t.Fatalf("Eva email = %q", response.Rows[0].EmployeeEmail)
	}
	if response.Rows[1].EmployeeEmail != "sofia.manzoni@example.com" {
		t.Fatalf("Sofia email = %q", response.Rows[1].EmployeeEmail)
	}
	for _, mapping := range report.Mappings {
		if mapping.Strategy != "partial_name" {
			t.Fatalf("mapping strategy = %q, want partial_name", mapping.Strategy)
		}
	}
}

func TestResolveTrainingImportEmployeesSkipsNessunoRows(t *testing.T) {
	response := ImportDryRunResponse{
		Rows: []ImportRow{
			{Sheet: "Per budget ", Row: 4, EmployeeName: "Nessuno TLC", CourseTitle: "Corso ad Hoc con nostro fornitore", Year: 2026, Status: "candidate"},
		},
	}

	report := ResolveTrainingImportEmployees(&response, nil)

	if report.SkippedNonPersonRows != 1 {
		t.Fatalf("SkippedNonPersonRows = %d, want 1", report.SkippedNonPersonRows)
	}
	if report.UnmatchedRows != 0 {
		t.Fatalf("UnmatchedRows = %d, want 0", report.UnmatchedRows)
	}
	if response.Rows[0].Status != "skipped" {
		t.Fatalf("row status = %q, want skipped", response.Rows[0].Status)
	}
}
