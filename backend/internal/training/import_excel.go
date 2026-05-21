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
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
)

var trainingImportSheets = map[string]struct{}{
	"Team ":                       {},
	"Team Tecnici - individuali":  {},
	"Team Tecnici - integrazione": {},
	"Per budget ":                 {},
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
		if _, ok := trainingImportSheets[sheet]; !ok {
			continue
		}
		rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})
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
		summary, err := store.ImportTrainingRowsDetailed(ctx, principal, response.Rows)
		if err != nil {
			return ImportDryRunResponse{}, err
		}
		response.Summary.CreatedEnrollments = summary.CreatedEnrollments
		response.Summary.UpdatedEnrollments = summary.UpdatedEnrollments
		response.Summary.UnchangedEnrollments = summary.UnchangedEnrollments
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
			Sheet:            sheet,
			Row:              i + 1,
			EmployeeName:     employeeName,
			EmployeeEmail:    employeeEmail,
			CourseTitle:      courseTitle,
			TeamName:         cleanImportLabel(valueByHeaders(row, headers, "team")),
			SkillAreaName:    cleanImportLabel(valueByHeaders(row, headers, "area formazione", "area formativa", "skill area")),
			VendorName:       cleanImportLabel(valueByHeaders(row, headers, "fornitore", "vendor")),
			CourseURL:        valueByHeaders(row, headers, "link al corso", "course url", "url"),
			Priority:         boundedIntFromText(valueByHeaders(row, headers, "priorita'", "priorita", "priority"), 1, 5),
			LevelAsIs:        boundedIntFromText(valueByHeaders(row, headers, "livello as is", "level as is"), 0, 5),
			LevelToBe:        boundedIntFromText(valueByHeaders(row, headers, "livello to be", "level to be"), 0, 5),
			PlannedStart:     dateFromText(valueByHeaders(row, headers, "data inizio", "planned start")),
			PlannedEnd:       dateFromText(valueByHeaders(row, headers, "data fine", "planned end")),
			HoursPlanned:     positiveIntFromText(valueByHeaders(row, headers, "h totali", "ore", "hours")),
			CostPlanned:      nonNegativeFloatFromText(valueByHeaders(row, headers, "costo", "cost")),
			Motivation:       valueByHeaders(row, headers, "motivazione > obiettivo", "motivazione", "motivation"),
			Objective:        valueByHeaders(row, headers, "obiettivo formativo", "obiettivo", "objective"),
			Notes:            joinImportNotes(valueByHeaders(row, headers, "note"), valueByHeaders(row, headers, "to do", "todo")),
			EnrollmentStatus: enrollmentStatusFromText(valueByHeaders(row, headers, "stato", "status")),
			Year:             rowYear,
			Status:           "candidate",
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

func boundedIntFromText(value string, minValue int, maxValue int) *int {
	parsed, ok := intValueFromText(value)
	if !ok || parsed < minValue || parsed > maxValue {
		return nil
	}
	return &parsed
}

func positiveIntFromText(value string) *int {
	parsed, ok := intValueFromText(value)
	if !ok || parsed <= 0 {
		return nil
	}
	return &parsed
}

func intValueFromText(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return 0, false
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed, true
	}
	parsedFloat, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
	if err != nil || parsedFloat != float64(int(parsedFloat)) {
		return 0, false
	}
	return int(parsedFloat), true
}

func nonNegativeFloatFromText(value string) *float64 {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return nil
	}
	value = normalizedDecimalText(value)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return nil
	}
	return &parsed
}

func normalizedDecimalText(value string) string {
	value = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsDigit(r), r == '.', r == ',', r == '-':
			return r
		default:
			return -1
		}
	}, value)
	if strings.Contains(value, ".") && strings.Contains(value, ",") {
		if strings.LastIndex(value, ",") > strings.LastIndex(value, ".") {
			value = strings.ReplaceAll(value, ".", "")
			return strings.ReplaceAll(value, ",", ".")
		}
		return strings.ReplaceAll(value, ",", "")
	}
	if strings.Contains(value, ",") {
		return strings.ReplaceAll(value, ",", ".")
	}
	if strings.Count(value, ".") == 1 {
		parts := strings.Split(value, ".")
		if len(parts) == 2 && len(parts[1]) == 3 {
			return parts[0] + parts[1]
		}
	}
	return value
}

func dateFromText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return ""
	}
	if serial, err := strconv.ParseFloat(value, 64); err == nil {
		date, err := excelize.ExcelDateToTime(serial, false)
		if err == nil {
			return date.Format("2006-01-02")
		}
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006", "2/1/2006", "02/01/06", "2/1/06", "02-01-2006", "2-1-2006", "02-01-06", "2-1-06"} {
		date, err := time.Parse(layout, value)
		if err == nil {
			return date.Format("2006-01-02")
		}
	}
	return ""
}

func joinImportNotes(values ...string) string {
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "/" {
			result = append(result, value)
		}
	}
	return strings.Join(result, "\n")
}

func enrollmentStatusFromText(value string) string {
	value = normalizeImportValue(value)
	switch value {
	case "", "/":
		return ""
	case "in corso", "incorso", "started", "start", "running":
		return string(EnrollmentInProgress)
	case "completata", "completato", "completo", "completa", "done", "completed", "conclusa", "concluso", "terminata", "terminato":
		return string(EnrollmentCompleted)
	case "approvata", "approvato", "approved":
		return string(EnrollmentApproved)
	case "proposta", "proposto", "proposed", "da fare", "pianificata", "pianificato":
		return string(EnrollmentProposed)
	case "non superata", "non superato", "bocciata", "bocciato", "failed":
		return string(EnrollmentFailed)
	case "annullata", "annullato", "cancelled", "cancellata", "cancellato":
		return string(EnrollmentCancelled)
	case "scaduta", "scaduto", "expired":
		return string(EnrollmentExpired)
	default:
		return ""
	}
}

func (s *SQLStore) ImportTrainingRows(ctx context.Context, principal Principal, rows []ImportRow) (int, int, error) {
	summary, err := s.ImportTrainingRowsDetailed(ctx, principal, rows)
	return summary.CreatedEnrollments, summary.UpdatedEnrollments, err
}

func RecomputeTrainingImportSummary(response *ImportDryRunResponse) {
	var summary ImportSummary
	summary.CreatedEnrollments = response.Summary.CreatedEnrollments
	summary.UpdatedEnrollments = response.Summary.UpdatedEnrollments
	summary.UnchangedEnrollments = response.Summary.UnchangedEnrollments
	summary.ParsedRows = len(response.Rows)
	for _, row := range response.Rows {
		if row.Status == "candidate" {
			summary.CandidateRows++
			if strings.TrimSpace(row.EmployeeEmail) == "" {
				summary.AmbiguousRows++
			}
		} else {
			summary.SkippedRows++
		}
	}
	response.Summary = summary
}

func (s *SQLStore) ImportTrainingRowsDetailed(ctx context.Context, principal Principal, rows []ImportRow) (ImportSummary, error) {
	if !principal.IsPeopleAdmin {
		return ImportSummary{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	var summary ImportSummary
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
			teamID, err := s.upsertImportTeam(ctx, tx, row.TeamName)
			if err != nil {
				return err
			}
			if teamID != "" {
				if err := s.upsertImportTeamMembership(ctx, tx, employeeID, teamID); err != nil {
					return err
				}
			}
			courseID, err := s.upsertImportCourse(ctx, tx, row)
			if err != nil {
				return err
			}
			planID, err := s.upsertImportPlan(ctx, tx, row.Year)
			if err != nil {
				return err
			}
			result, err := s.upsertImportEnrollment(ctx, tx, employeeID, courseID, planID, row)
			if err != nil {
				return err
			}
			switch result {
			case "created":
				summary.CreatedEnrollments++
			case "updated":
				summary.UpdatedEnrollments++
			case "unchanged":
				summary.UnchangedEnrollments++
			}
		}
		return nil
	})
	return summary, err
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

func (s *SQLStore) upsertImportCourse(ctx context.Context, tx *sql.Tx, row ImportRow) (string, error) {
	var id string
	title := strings.TrimSpace(row.CourseTitle)
	err := tx.QueryRowContext(ctx, `SELECT id::text FROM training.course WHERE lower(title) = lower($1) LIMIT 1`, strings.TrimSpace(title)).Scan(&id)
	if err == nil {
		if err := s.updateImportCourseDetails(ctx, tx, id, row); err != nil {
			return "", err
		}
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("load training import course: %w", err)
	}
	vendorID, err := s.upsertImportVendor(ctx, tx, row.VendorName)
	if err != nil {
		return "", err
	}
	skillAreaID, err := s.upsertImportSkillArea(ctx, tx, row.SkillAreaName)
	if err != nil {
		return "", err
	}
	const stmt = `
INSERT INTO training.course (
  title,
  vendor_id,
  skill_area_id,
  delivery_mode,
  provider_kind,
  default_hours,
  default_cost,
  course_url,
  is_active
)
VALUES ($1::text, NULLIF($2::text, '')::uuid, NULLIF($3::text, '')::uuid, 'mixed', 'external', $4::integer, $5::numeric, NULLIF($6::text, ''), true)
RETURNING id::text`
	if err := tx.QueryRowContext(ctx, stmt, title, vendorID, skillAreaID, row.HoursPlanned, row.CostPlanned, row.CourseURL).Scan(&id); err != nil {
		return "", fmt.Errorf("create training import course: %w", err)
	}
	return id, nil
}

func (s *SQLStore) updateImportCourseDetails(ctx context.Context, tx *sql.Tx, id string, row ImportRow) error {
	vendorID, err := s.upsertImportVendor(ctx, tx, row.VendorName)
	if err != nil {
		return err
	}
	skillAreaID, err := s.upsertImportSkillArea(ctx, tx, row.SkillAreaName)
	if err != nil {
		return err
	}
	const stmt = `
UPDATE training.course
SET
  vendor_id = COALESCE(vendor_id, NULLIF($2::text, '')::uuid),
  skill_area_id = COALESCE(skill_area_id, NULLIF($3::text, '')::uuid),
  default_hours = COALESCE(default_hours, $4::integer),
  default_cost = COALESCE(default_cost, $5::numeric),
  course_url = COALESCE(NULLIF(course_url, ''), NULLIF($6::text, ''))
WHERE id = $1::uuid`
	if _, err := tx.ExecContext(ctx, stmt, id, vendorID, skillAreaID, row.HoursPlanned, row.CostPlanned, strings.TrimSpace(row.CourseURL)); err != nil {
		return fmt.Errorf("update training import course details: %w", err)
	}
	return nil
}

func (s *SQLStore) upsertImportVendor(ctx context.Context, tx *sql.Tx, name string) (string, error) {
	name = cleanImportLabel(name)
	if name == "" {
		return "", nil
	}
	var id string
	const stmt = `
INSERT INTO training.vendor (name, name_normalized, is_active)
VALUES ($1::text, $1::citext, true)
ON CONFLICT (name_normalized) DO UPDATE
SET is_active = true
RETURNING id::text`
	if err := tx.QueryRowContext(ctx, stmt, name).Scan(&id); err != nil {
		return "", fmt.Errorf("upsert training import vendor: %w", err)
	}
	return id, nil
}

func (s *SQLStore) upsertImportTeam(ctx context.Context, tx *sql.Tx, name string) (string, error) {
	name = cleanImportLabel(name)
	if name == "" {
		return "", nil
	}
	var id string
	const stmt = `
INSERT INTO training.team (code, name, is_active)
VALUES ($1::text, $2::text, true)
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name,
    is_active = true
RETURNING id::text`
	if err := tx.QueryRowContext(ctx, stmt, importCodeFromName(name, "TEAM"), name).Scan(&id); err != nil {
		return "", fmt.Errorf("upsert training import team: %w", err)
	}
	return id, nil
}

func (s *SQLStore) upsertImportTeamMembership(ctx context.Context, tx *sql.Tx, employeeID string, teamID string) error {
	const stmt = `
INSERT INTO training.team_membership (employee_id, team_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT (employee_id, team_id) WHERE end_date IS NULL DO NOTHING`
	if _, err := tx.ExecContext(ctx, stmt, employeeID, teamID); err != nil {
		return fmt.Errorf("upsert training import team membership: %w", err)
	}
	return nil
}

func (s *SQLStore) upsertImportSkillArea(ctx context.Context, tx *sql.Tx, name string) (string, error) {
	name = cleanImportLabel(name)
	if name == "" {
		return "", nil
	}
	var id string
	const stmt = `
INSERT INTO training.skill_area (code, name, is_active)
VALUES ($1, $2, true)
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name,
    is_active = true
RETURNING id::text`
	if err := tx.QueryRowContext(ctx, stmt, importCodeFromName(name, "AREA"), name).Scan(&id); err != nil {
		return "", fmt.Errorf("upsert training import skill area: %w", err)
	}
	return id, nil
}

func cleanImportLabel(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "/" {
		return ""
	}
	return value
}

func importCodeFromName(name string, fallback string) string {
	name = cleanImportLabel(name)
	if name == "" {
		return fallback
	}
	parts := []rune{}
	lastUnderscore := false
	for _, r := range strings.ToUpper(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			parts = append(parts, r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			parts = append(parts, '_')
			lastUnderscore = true
		}
	}
	code := strings.Trim(string(parts), "_")
	if code == "" {
		return fallback
	}
	return code
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

func (s *SQLStore) upsertImportEnrollment(ctx context.Context, tx *sql.Tx, employeeID string, courseID string, planID string, row ImportRow) (string, error) {
	status := "proposed"
	statusFromImport := false
	if strings.TrimSpace(row.EnrollmentStatus) != "" {
		status = row.EnrollmentStatus
		statusFromImport = true
	} else if row.Year <= 2025 {
		status = "completed"
		statusFromImport = true
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
  SET priority = COALESCE($5::smallint, en.priority),
      level_as_is = COALESCE($6::smallint, en.level_as_is),
      level_to_be = COALESCE($7::smallint, en.level_to_be),
      planned_start = COALESCE(NULLIF($8::text, '')::date, en.planned_start),
      planned_end = COALESCE(NULLIF($9::text, '')::date, en.planned_end),
      hours_planned = COALESCE($10::integer, en.hours_planned),
      cost_planned = COALESCE($11::numeric, en.cost_planned),
      motivation = CASE
        WHEN NULLIF($12::text, '') IS NOT NULL AND (en.motivation IS NULL OR btrim(en.motivation) = '' OR en.motivation = 'Import storico') THEN NULLIF($12::text, '')
        WHEN en.motivation = 'Import storico' THEN NULL
        ELSE en.motivation
      END,
      objective = COALESCE(NULLIF($13::text, ''), NULLIF(en.objective, '')),
      notes = COALESCE(NULLIF($14::text, ''), NULLIF(en.notes, '')),
      course_title_snapshot = c.title,
      vendor_name_snapshot = COALESCE(v.name, en.vendor_name_snapshot),
      status = CASE
        WHEN $15::boolean THEN $4::training.enrollment_status
        ELSE en.status
      END
  FROM training.course c
  LEFT JOIN training.vendor v ON v.id = c.vendor_id
  WHERE en.id IN (SELECT id FROM existing)
    AND c.id = en.course_id
    AND (
      en.course_title_snapshot IS DISTINCT FROM c.title
      OR (v.name IS NOT NULL AND en.vendor_name_snapshot IS DISTINCT FROM v.name)
      OR ($5::smallint IS NOT NULL AND en.priority IS DISTINCT FROM $5::smallint)
      OR ($6::smallint IS NOT NULL AND en.level_as_is IS DISTINCT FROM $6::smallint)
      OR ($7::smallint IS NOT NULL AND en.level_to_be IS DISTINCT FROM $7::smallint)
      OR (NULLIF($8::text, '') IS NOT NULL AND en.planned_start IS DISTINCT FROM NULLIF($8::text, '')::date)
      OR (NULLIF($9::text, '') IS NOT NULL AND en.planned_end IS DISTINCT FROM NULLIF($9::text, '')::date)
      OR ($10::integer IS NOT NULL AND en.hours_planned IS DISTINCT FROM $10::integer)
      OR ($11::numeric IS NOT NULL AND en.cost_planned IS DISTINCT FROM $11::numeric)
      OR (NULLIF($12::text, '') IS NOT NULL AND (en.motivation IS NULL OR btrim(en.motivation) = '' OR en.motivation = 'Import storico'))
      OR en.motivation = 'Import storico'
      OR (NULLIF($13::text, '') IS NOT NULL AND NULLIF(en.objective, '') IS DISTINCT FROM NULLIF($13::text, ''))
      OR (NULLIF($14::text, '') IS NOT NULL AND NULLIF(en.notes, '') IS DISTINCT FROM NULLIF($14::text, ''))
      OR ($15::boolean AND en.status <> $4::training.enrollment_status)
    )
  RETURNING 'updated' AS result
), inserted AS (
  INSERT INTO training.enrollment (
    employee_id,
    course_id,
    training_plan_id,
    status,
    course_title_snapshot,
    vendor_name_snapshot,
    priority,
    level_as_is,
    level_to_be,
    planned_start,
    planned_end,
    hours_planned,
    cost_planned,
    motivation,
    objective,
    notes
  )
  SELECT
    $1::uuid,
    $2::uuid,
    $3::uuid,
    $4::training.enrollment_status,
    c.title,
    v.name,
    $5::smallint,
    $6::smallint,
    $7::smallint,
    NULLIF($8::text, '')::date,
    NULLIF($9::text, '')::date,
    $10::integer,
    $11::numeric,
    NULLIF($12::text, ''),
    NULLIF($13::text, ''),
    NULLIF($14::text, '')
  FROM training.course c
  LEFT JOIN training.vendor v ON v.id = c.vendor_id
  WHERE c.id = $2::uuid
    AND NOT EXISTS (SELECT 1 FROM existing)
  RETURNING 'created' AS result
)
SELECT result FROM inserted
UNION ALL
SELECT result FROM updated
UNION ALL
SELECT 'unchanged' AS result
WHERE EXISTS (SELECT 1 FROM existing)
  AND NOT EXISTS (SELECT 1 FROM updated)
LIMIT 1`
	var result string
	if err := tx.QueryRowContext(
		ctx,
		stmt,
		employeeID,
		courseID,
		planID,
		status,
		row.Priority,
		row.LevelAsIs,
		row.LevelToBe,
		row.PlannedStart,
		row.PlannedEnd,
		row.HoursPlanned,
		row.CostPlanned,
		strings.TrimSpace(row.Motivation),
		strings.TrimSpace(row.Objective),
		strings.TrimSpace(row.Notes),
		statusFromImport,
	).Scan(&result); err != nil {
		return "", fmt.Errorf("upsert training import enrollment: %w", err)
	}
	return result, nil
}
