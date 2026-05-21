package training

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"path/filepath"
	"strings"
)

type EmployeeImportResponse struct {
	OK       bool                    `json:"ok"`
	DryRun   bool                    `json:"dryRun"`
	FileName string                  `json:"fileName"`
	Summary  EmployeeImportSummary   `json:"summary"`
	Warnings []EmployeeImportWarning `json:"warnings"`
	Rows     []EmployeeImportRow     `json:"rows"`
}

type EmployeeImportSummary struct {
	ParsedRows         int `json:"parsedRows"`
	CandidateRows      int `json:"candidateRows"`
	SkippedRows        int `json:"skippedRows"`
	InvalidRows        int `json:"invalidRows"`
	DuplicateRows      int `json:"duplicateRows"`
	CreatedEmployees   int `json:"createdEmployees,omitempty"`
	UpdatedEmployees   int `json:"updatedEmployees,omitempty"`
	UnchangedEmployees int `json:"unchangedEmployees,omitempty"`
}

type EmployeeImportWarning struct {
	Row     int    `json:"row"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type EmployeeImportRow struct {
	Row       int    `json:"row"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Email     string `json:"email,omitempty"`
	Status    string `json:"status"`
}

func ParseEmployeeCSVImport(ctx context.Context, filename string, body io.Reader, commit bool, store *SQLStore, principal Principal) (EmployeeImportResponse, error) {
	reader := csv.NewReader(io.LimitReader(body, 8<<20))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return EmployeeImportResponse{}, fmt.Errorf("read employee import csv: %w", err)
	}

	response := EmployeeImportResponse{
		OK:       true,
		DryRun:   !commit,
		FileName: filepath.Base(filename),
		Warnings: []EmployeeImportWarning{},
		Rows:     []EmployeeImportRow{},
	}
	if len(records) == 0 {
		return response, nil
	}

	header := employeeImportHeader(records[0])
	if !header.valid() {
		return EmployeeImportResponse{}, validationError("employee_import_header_invalid", "intestazioni CSV dipendenti non riconosciute")
	}

	seenEmails := map[string]struct{}{}
	for index, record := range records[1:] {
		rowNumber := index + 2
		firstName := csvValue(record, header.firstName)
		lastName := csvValue(record, header.lastName)
		email := normalizeEmail(csvValue(record, header.email))
		if firstName == "" && lastName == "" && email == "" {
			response.Summary.SkippedRows++
			continue
		}

		row := EmployeeImportRow{
			Row:       rowNumber,
			FirstName: firstName,
			LastName:  lastName,
			Email:     email,
			Status:    "candidate",
		}
		response.Summary.ParsedRows++

		if firstName == "" || lastName == "" || email == "" {
			row.Status = "skipped"
			response.Summary.InvalidRows++
			response.Summary.SkippedRows++
			response.Warnings = append(response.Warnings, EmployeeImportWarning{
				Row:     rowNumber,
				Code:    "employee_required_fields_missing",
				Message: "nome, cognome ed email sono obbligatori",
			})
			response.Rows = append(response.Rows, row)
			continue
		}
		if !validImportEmail(email) {
			row.Status = "skipped"
			response.Summary.InvalidRows++
			response.Summary.SkippedRows++
			response.Warnings = append(response.Warnings, EmployeeImportWarning{
				Row:     rowNumber,
				Code:    "employee_email_invalid",
				Message: "email dipendente non valida",
			})
			response.Rows = append(response.Rows, row)
			continue
		}
		if _, exists := seenEmails[email]; exists {
			row.Status = "skipped"
			response.Summary.DuplicateRows++
			response.Summary.InvalidRows++
			response.Summary.SkippedRows++
			response.Warnings = append(response.Warnings, EmployeeImportWarning{
				Row:     rowNumber,
				Code:    "employee_email_duplicate",
				Message: "email duplicata nel CSV dipendenti",
			})
			response.Rows = append(response.Rows, row)
			continue
		}

		seenEmails[email] = struct{}{}
		response.Summary.CandidateRows++
		response.Rows = append(response.Rows, row)
	}

	if commit {
		summary, err := store.ImportEmployees(ctx, principal, response.Rows)
		if err != nil {
			return EmployeeImportResponse{}, err
		}
		response.Summary.CreatedEmployees = summary.CreatedEmployees
		response.Summary.UpdatedEmployees = summary.UpdatedEmployees
		response.Summary.UnchangedEmployees = summary.UnchangedEmployees
	} else if store != nil {
		summary, err := store.PlanImportEmployees(ctx, principal, response.Rows)
		if err != nil {
			return EmployeeImportResponse{}, err
		}
		response.Summary.CreatedEmployees = summary.CreatedEmployees
		response.Summary.UpdatedEmployees = summary.UpdatedEmployees
		response.Summary.UnchangedEmployees = summary.UnchangedEmployees
	}
	return response, nil
}

type employeeImportHeaderMap struct {
	firstName int
	lastName  int
	email     int
}

func (h employeeImportHeaderMap) valid() bool {
	return h.firstName >= 0 && h.lastName >= 0 && h.email >= 0
}

func employeeImportHeader(record []string) employeeImportHeaderMap {
	header := employeeImportHeaderMap{firstName: -1, lastName: -1, email: -1}
	for index, value := range record {
		switch normalizeImportHeader(strings.TrimPrefix(value, "\ufeff")) {
		case "nome", "first name", "first_name", "firstname":
			header.firstName = index
		case "cognome", "last name", "last_name", "lastname", "surname":
			header.lastName = index
		case "email", "mail", "e-mail":
			header.email = index
		}
	}
	return header
}

func csvValue(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return strings.Join(strings.Fields(strings.TrimSpace(record[index])), " ")
}

func validImportEmail(value string) bool {
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	address, err := mail.ParseAddress(value)
	return err == nil && strings.EqualFold(address.Address, value)
}

func (s *SQLStore) ImportEmployees(ctx context.Context, principal Principal, rows []EmployeeImportRow) (EmployeeImportSummary, error) {
	if !principal.IsPeopleAdmin {
		return EmployeeImportSummary{}, forbiddenError("people_role_required", "azione riservata a People")
	}

	for _, row := range rows {
		if row.Status == "skipped" {
			return EmployeeImportSummary{}, validationError("employee_import_invalid_rows", "correggere il CSV dipendenti prima del commit")
		}
	}

	var summary EmployeeImportSummary
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		for _, row := range rows {
			if row.Status != "candidate" {
				continue
			}
			result, err := s.upsertImportEmployee(ctx, tx, row)
			if err != nil {
				return err
			}
			switch result {
			case "created":
				summary.CreatedEmployees++
			case "updated":
				summary.UpdatedEmployees++
			case "unchanged":
				summary.UnchangedEmployees++
			}
		}
		return nil
	})
	return summary, err
}

func (s *SQLStore) PlanImportEmployees(ctx context.Context, principal Principal, rows []EmployeeImportRow) (EmployeeImportSummary, error) {
	if !principal.IsPeopleAdmin {
		return EmployeeImportSummary{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if s == nil || s.db == nil {
		return EmployeeImportSummary{}, errors.New("training database not configured")
	}

	var summary EmployeeImportSummary
	for _, row := range rows {
		if row.Status != "candidate" {
			continue
		}
		result, err := s.planImportEmployee(ctx, row)
		if err != nil {
			return EmployeeImportSummary{}, err
		}
		switch result {
		case "created":
			summary.CreatedEmployees++
		case "updated":
			summary.UpdatedEmployees++
		case "unchanged":
			summary.UnchangedEmployees++
		}
	}
	return summary, nil
}

func (s *SQLStore) upsertImportEmployee(ctx context.Context, tx *sql.Tx, row EmployeeImportRow) (string, error) {
	var existing struct {
		id        string
		firstName string
		lastName  string
		status    string
	}
	err := tx.QueryRowContext(ctx, `
SELECT id::text, first_name, last_name, status::text
FROM training.employee
WHERE email = $1
LIMIT 1`, row.Email).Scan(&existing.id, &existing.firstName, &existing.lastName, &existing.status)
	if errors.Is(err, sql.ErrNoRows) {
		_, err := tx.ExecContext(ctx, `
INSERT INTO training.employee (first_name, last_name, email, status)
VALUES ($1, $2, $3, 'active')`, row.FirstName, row.LastName, row.Email)
		if err != nil {
			return "", fmt.Errorf("create training import employee: %w", err)
		}
		return "created", nil
	}
	if err != nil {
		return "", fmt.Errorf("load training import employee: %w", err)
	}

	if existing.firstName == row.FirstName && existing.lastName == row.LastName && existing.status == "active" {
		return "unchanged", nil
	}

	_, err = tx.ExecContext(ctx, `
UPDATE training.employee
SET first_name = $1,
    last_name = $2,
    status = 'active'::training.employee_status
WHERE id = $3::uuid`, row.FirstName, row.LastName, existing.id)
	if err != nil {
		return "", fmt.Errorf("update training import employee: %w", err)
	}
	return "updated", nil
}

func (s *SQLStore) planImportEmployee(ctx context.Context, row EmployeeImportRow) (string, error) {
	var existing struct {
		firstName string
		lastName  string
		status    string
	}
	err := s.db.QueryRowContext(ctx, `
SELECT first_name, last_name, status::text
FROM training.employee
WHERE email = $1
LIMIT 1`, row.Email).Scan(&existing.firstName, &existing.lastName, &existing.status)
	if errors.Is(err, sql.ErrNoRows) {
		return "created", nil
	}
	if err != nil {
		return "", fmt.Errorf("plan training import employee: %w", err)
	}
	if existing.firstName == row.FirstName && existing.lastName == row.LastName && existing.status == "active" {
		return "unchanged", nil
	}
	return "updated", nil
}
