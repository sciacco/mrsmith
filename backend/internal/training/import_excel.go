package training

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

var trainingImportSheets = map[string]struct{}{
	"Team ":                       {},
	"Team Tecnici - individuali":  {},
	"Team Tecnici - integrazione": {},
	"Per budget ":                 {},
	"Certificazioni ":             {},
	"Formazione 2024_2025":        {},
}

func ParseTrainingImport(ctx context.Context, filename string, body io.Reader, commit bool, store *SQLStore, principal Principal) (ImportDryRunResponse, error) {
	raw, err := io.ReadAll(io.LimitReader(body, 64<<20))
	if err != nil {
		return ImportDryRunResponse{}, fmt.Errorf("read training import file: %w", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return ImportDryRunResponse{}, fmt.Errorf("open training import workbook: %w", err)
	}
	defer f.Close()

	response := ImportDryRunResponse{
		OK:       true,
		DryRun:   !commit,
		FileName: filepath.Base(filename),
		Sheets:   []ImportSheet{},
		Warnings: []ImportWarning{},
		Rows:     []ImportRow{},
	}

	seen := map[string]int{}
	for _, sheet := range f.GetSheetList() {
		if _, ok := trainingImportSheets[sheet]; !ok && !strings.Contains(strings.ToLower(sheet), "formazione") {
			continue
		}
		rows, err := f.GetRows(sheet)
		if err != nil {
			response.Warnings = append(response.Warnings, ImportWarning{Sheet: sheet, Code: "sheet_read_failed", Message: err.Error()})
			continue
		}
		response.Sheets = append(response.Sheets, ImportSheet{Name: sheet, Rows: len(rows)})
		parsed, warnings := parseTrainingSheet(sheet, rows)
		response.Warnings = append(response.Warnings, warnings...)
		for _, row := range parsed {
			if row.Status != "candidate" {
				response.Rows = append(response.Rows, row)
				continue
			}
			key := importDedupKey(row)
			if existingIndex, exists := seen[key]; exists {
				response.Warnings = append(response.Warnings, ImportWarning{
					Sheet:   row.Sheet,
					Row:     row.Row,
					Code:    "duplicate_candidate",
					Message: "riga duplicata nella chiave dipendente/corso/anno",
				})
				if isBudgetImportSheet(row.Sheet) && !isBudgetImportSheet(response.Rows[existingIndex].Sheet) {
					response.Rows[existingIndex] = row
				}
				continue
			}
			seen[key] = len(response.Rows)
			response.Rows = append(response.Rows, row)
		}
	}
	response.Summary.ParsedRows = len(response.Rows)
	for _, row := range response.Rows {
		if row.Status == "candidate" {
			response.Summary.CandidateRows++
			if strings.TrimSpace(row.EmployeeEmail) == "" {
				response.Summary.AmbiguousRows++
			}
		} else {
			response.Summary.SkippedRows++
		}
	}

	if commit {
		created, updated, err := store.ImportTrainingRows(ctx, principal, response.Rows)
		if err != nil {
			return ImportDryRunResponse{}, err
		}
		response.Summary.CreatedEnrollments = created
		response.Summary.UpdatedEnrollments = updated
	}
	return response, nil
}

func importDedupKey(row ImportRow) string {
	employee := normalizeImportValue(row.EmployeeEmail)
	if employee == "" {
		employee = normalizeImportValue(row.EmployeeName)
	}
	return strings.Join([]string{
		employee,
		normalizeImportValue(row.CourseTitle),
		strconv.Itoa(row.Year),
	}, "|")
}

func normalizeImportValue(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func isBudgetImportSheet(sheet string) bool {
	return strings.EqualFold(strings.TrimSpace(sheet), "Per budget")
}

func parseTrainingSheet(sheet string, rows [][]string) ([]ImportRow, []ImportWarning) {
	result := []ImportRow{}
	warnings := []ImportWarning{}
	headerIndex := -1
	headers := map[string]int{}
	for i, row := range rows {
		candidate := headerMap(row)
		if hasAnyHeader(candidate, "employee", "dipendente", "persona", "nome") && hasAnyHeader(candidate, "course", "corso", "formazione", "titolo") {
			headerIndex = i
			headers = candidate
			break
		}
	}
	if headerIndex == -1 {
		headers = defaultTrainingHeaders()
		headerIndex = 0
		warnings = append(warnings, ImportWarning{Sheet: sheet, Code: "header_not_found", Message: "intestazioni non riconosciute, uso mapping conservativo"})
	}
	year := 2026
	if strings.Contains(sheet, "2024_2025") || strings.Contains(sheet, "2025") {
		year = 2025
	}
	for i := headerIndex + 1; i < len(rows); i++ {
		row := rows[i]
		employeeName := valueByHeaders(row, headers, "employee", "dipendente", "persona", "nome e cognome", "nome")
		employeeEmail := valueByHeaders(row, headers, "email", "mail")
		courseTitle := valueByHeaders(row, headers, "course", "corso", "formazione", "titolo", "certificazione")
		rowYear := intFromText(valueByHeaders(row, headers, "anno", "year"), year)
		if strings.TrimSpace(employeeName) == "" && strings.TrimSpace(employeeEmail) == "" && strings.TrimSpace(courseTitle) == "" {
			continue
		}
		if strings.TrimSpace(courseTitle) == "" {
			warnings = append(warnings, ImportWarning{Sheet: sheet, Row: i + 1, Code: "missing_course", Message: "corso mancante"})
			result = append(result, ImportRow{Sheet: sheet, Row: i + 1, EmployeeName: employeeName, EmployeeEmail: employeeEmail, Year: rowYear, Status: "skipped"})
			continue
		}
		if strings.TrimSpace(employeeEmail) == "" && strings.TrimSpace(employeeName) == "" {
			warnings = append(warnings, ImportWarning{Sheet: sheet, Row: i + 1, Code: "missing_employee", Message: "dipendente mancante"})
			result = append(result, ImportRow{Sheet: sheet, Row: i + 1, CourseTitle: courseTitle, Year: rowYear, Status: "skipped"})
			continue
		}
		if strings.TrimSpace(employeeEmail) == "" {
			warnings = append(warnings, ImportWarning{Sheet: sheet, Row: i + 1, Code: "employee_match_required", Message: "manca email, serve match HR univoco"})
		}
		result = append(result, ImportRow{
			Sheet:         sheet,
			Row:           i + 1,
			EmployeeName:  employeeName,
			EmployeeEmail: employeeEmail,
			CourseTitle:   courseTitle,
			Year:          rowYear,
			Status:        "candidate",
		})
	}
	return result, warnings
}

func headerMap(row []string) map[string]int {
	headers := map[string]int{}
	for i, cell := range row {
		key := normalizeImportHeader(cell)
		if key != "" {
			headers[key] = i
		}
	}
	return headers
}

func defaultTrainingHeaders() map[string]int {
	return map[string]int{
		"employee": 0,
		"course":   1,
		"year":     2,
	}
}

func normalizeImportHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func hasAnyHeader(headers map[string]int, names ...string) bool {
	for key := range headers {
		for _, name := range names {
			if strings.Contains(key, name) {
				return true
			}
		}
	}
	return false
}

func valueByHeaders(row []string, headers map[string]int, names ...string) string {
	for _, name := range names {
		if index, ok := headers[name]; ok && index >= 0 && index < len(row) {
			return strings.TrimSpace(row[index])
		}
	}
	bestIndex := len(row)
	for key, index := range headers {
		if index < 0 || index >= len(row) {
			continue
		}
		for _, name := range names {
			if strings.Contains(key, name) && index < bestIndex {
				bestIndex = index
			}
		}
	}
	if bestIndex < len(row) {
		return strings.TrimSpace(row[bestIndex])
	}
	return ""
}

func intFromText(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func (s *SQLStore) ImportTrainingRows(ctx context.Context, principal Principal, rows []ImportRow) (int, int, error) {
	if !principal.IsPeopleAdmin {
		return 0, 0, forbiddenError("people_role_required", "azione riservata a People")
	}
	created := 0
	updated := 0
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL training.allow_status_override = 'true'`); err != nil {
			return fmt.Errorf("enable training import status override: %w", err)
		}
		for _, row := range rows {
			if row.Status != "candidate" {
				continue
			}
			employeeID, err := s.matchImportEmployee(ctx, tx, row)
			if err != nil {
				return err
			}
			courseID, err := s.upsertImportCourse(ctx, tx, row.CourseTitle)
			if err != nil {
				return err
			}
			planID, err := s.upsertImportPlan(ctx, tx, row.Year)
			if err != nil {
				return err
			}
			inserted, err := s.upsertImportEnrollment(ctx, tx, employeeID, courseID, planID, row)
			if err != nil {
				return err
			}
			if inserted {
				created++
			} else {
				updated++
			}
		}
		return nil
	})
	return created, updated, err
}

func (s *SQLStore) matchImportEmployee(ctx context.Context, tx *sql.Tx, row ImportRow) (string, error) {
	if strings.TrimSpace(row.EmployeeEmail) != "" {
		return s.employeeIDByEmail(ctx, tx, row.EmployeeEmail)
	}
	parts := strings.Fields(strings.ToLower(row.EmployeeName))
	if len(parts) == 0 {
		return "", validationError("employee_match_required", "dipendente mancante")
	}
	pattern := "%" + strings.Join(parts, "%") + "%"
	const q = `
SELECT id::text
FROM training.employee
WHERE lower(first_name || ' ' || last_name) LIKE $1
   OR lower(last_name || ' ' || first_name) LIKE $1
LIMIT 2`
	rows, err := tx.QueryContext(ctx, q, pattern)
	if err != nil {
		return "", fmt.Errorf("match training import employee: %w", err)
	}
	defer rows.Close()
	matches := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		matches = append(matches, id)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(matches) != 1 {
		return "", validationError("employee_match_ambiguous", "match HR assente o ambiguo")
	}
	return matches[0], nil
}

func (s *SQLStore) upsertImportCourse(ctx context.Context, tx *sql.Tx, title string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id::text FROM training.course WHERE lower(title) = lower($1) LIMIT 1`, strings.TrimSpace(title)).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("load training import course: %w", err)
	}
	const stmt = `
INSERT INTO training.course (title, delivery_mode, provider_kind, is_active)
VALUES ($1, 'mixed', 'external', true)
RETURNING id::text`
	if err := tx.QueryRowContext(ctx, stmt, strings.TrimSpace(title)).Scan(&id); err != nil {
		return "", fmt.Errorf("create training import course: %w", err)
	}
	return id, nil
}

func (s *SQLStore) upsertImportPlan(ctx context.Context, tx *sql.Tx, year int) (string, error) {
	var id string
	status := "draft"
	if year <= 2025 {
		status = "closed"
	}
	const stmt = `
INSERT INTO training.training_plan (year, status, closed_at)
VALUES ($1, $2::training.plan_status, CASE WHEN $2::training.plan_status = 'closed'::training.plan_status THEN now() ELSE NULL END)
ON CONFLICT (year) DO UPDATE
SET status = CASE
      WHEN EXCLUDED.status = 'closed'::training.plan_status THEN EXCLUDED.status
      ELSE training.training_plan.status
    END,
    closed_at = CASE
      WHEN EXCLUDED.status = 'closed'::training.plan_status THEN COALESCE(training.training_plan.closed_at, now())
      ELSE training.training_plan.closed_at
    END
RETURNING id::text`
	if err := tx.QueryRowContext(ctx, stmt, year, status).Scan(&id); err != nil {
		return "", fmt.Errorf("upsert training import plan: %w", err)
	}
	return id, nil
}

func (s *SQLStore) upsertImportEnrollment(ctx context.Context, tx *sql.Tx, employeeID string, courseID string, planID string, row ImportRow) (bool, error) {
	status := "proposed"
	if row.Year <= 2025 {
		status = "completed"
	}
	const stmt = `
WITH existing AS (
  SELECT en.id
  FROM training.enrollment en
  WHERE en.employee_id = $1::uuid
    AND en.course_id = $2::uuid
    AND en.training_plan_id = $3::uuid
  LIMIT 1
), updated AS (
  UPDATE training.enrollment en
  SET course_title_snapshot = c.title,
      status = CASE
        WHEN $4::training.enrollment_status = 'completed'::training.enrollment_status THEN 'completed'::training.enrollment_status
        ELSE en.status
      END
  FROM training.course c
  WHERE en.id IN (SELECT id FROM existing)
    AND c.id = en.course_id
  RETURNING false AS inserted
), inserted AS (
  INSERT INTO training.enrollment (
    employee_id,
    course_id,
    training_plan_id,
    status,
    course_title_snapshot,
    motivation
  )
  SELECT $1::uuid, $2::uuid, $3::uuid, $4::training.enrollment_status, c.title, 'Import storico'
  FROM training.course c
  WHERE c.id = $2::uuid
    AND NOT EXISTS (SELECT 1 FROM existing)
  RETURNING true AS inserted
)
SELECT inserted FROM inserted
UNION ALL
SELECT inserted FROM updated
LIMIT 1`
	var inserted bool
	if err := tx.QueryRowContext(ctx, stmt, employeeID, courseID, planID, status).Scan(&inserted); err != nil {
		return false, fmt.Errorf("upsert training import enrollment: %w", err)
	}
	return inserted, nil
}
