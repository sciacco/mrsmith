package training

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var errAnotherPlanOpenRace = errors.New("another training plan opened concurrently")

// PlanningOverview returns the current-year plan summary + ranked suggestion queue.
func (s *SQLStore) PlanningOverview(ctx context.Context, principal Principal, year int, team string) (PlanningResponse, error) {
	if !principal.IsPeopleAdmin {
		return PlanningResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	resp := PlanningResponse{Year: year, TeamScope: team, Suggestions: []PlanningSuggestion{}}

	summary, err := s.planSummary(ctx, year)
	if err != nil {
		return resp, err
	}
	resp.Plan = summary

	if summary == nil || summary.Status == "closed" {
		return resp, nil
	}

	suggestions, err := s.BuildSuggestions(ctx, summary.PlanID, year, team)
	if err != nil {
		return resp, err
	}
	resp.Suggestions = suggestions
	return resp, nil
}

func (s *SQLStore) ListTrainingPlansForPlanning(ctx context.Context, principal Principal, status string) (TrainingPlansResponse, error) {
	if !principal.IsPeopleAdmin {
		return TrainingPlansResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && !isPlanStatus(status) {
		return TrainingPlansResponse{}, validationError("invalid_status", "stato piano non valido")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, year, status::text, COALESCE(budget_total, 0)::float8, created_at
FROM training.training_plan
WHERE ($1 = '' OR status::text = $1)
ORDER BY created_at DESC, year DESC`, status)
	if err != nil {
		return TrainingPlansResponse{}, fmt.Errorf("list training plans: %w", err)
	}
	defer rows.Close()

	resp := TrainingPlansResponse{Plans: []TrainingPlanListRow{}}
	for rows.Next() {
		var row TrainingPlanListRow
		var createdAt time.Time
		if err := rows.Scan(&row.ID, &row.Year, &row.Status, &row.BudgetTotal, &createdAt); err != nil {
			return TrainingPlansResponse{}, fmt.Errorf("scan training plan list row: %w", err)
		}
		row.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		resp.Plans = append(resp.Plans, row)
	}
	return resp, rows.Err()
}

func isPlanStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "draft", "open", "frozen", "closed":
		return true
	default:
		return false
	}
}

// planSummary loads the plan for `year`, the budget consumption and prev-year flag.
func (s *SQLStore) planSummary(ctx context.Context, year int) (*PlanningSummary, error) {
	const q = `
SELECT
  tp.id::text,
  tp.year,
  tp.status::text,
  COALESCE(tp.budget_total, 0)::float8 AS budget_total,
  COALESCE(tp.notes, '') AS notes,
  COALESCE(SUM(COALESCE(en.cost_actual, en.cost_planned, c.default_cost, 0)), 0)::float8 AS budget_spent,
  COUNT(en.id) AS enrollments_planned
FROM training.training_plan tp
LEFT JOIN training.enrollment en ON en.training_plan_id = tp.id AND en.status NOT IN ('cancelled')
LEFT JOIN training.course c ON c.id = en.course_id
WHERE tp.year = $1
GROUP BY tp.id, tp.year, tp.status, tp.budget_total, tp.notes`

	var summary PlanningSummary
	err := s.db.QueryRowContext(ctx, q, year).Scan(
		&summary.PlanID,
		&summary.Year,
		&summary.Status,
		&summary.BudgetTotal,
		&summary.Notes,
		&summary.BudgetSpent,
		&summary.EnrollmentsPlanned,
	)
	if errors.Is(err, sql.ErrNoRows) {
		hasPrev, err := s.planExistsForYear(ctx, year-1)
		if err != nil {
			return nil, err
		}
		return &PlanningSummary{
			Year:            year,
			Status:          "missing",
			HasPrevYearPlan: hasPrev,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("planning: load summary: %w", err)
	}

	summary.BudgetResidual = summary.BudgetTotal - summary.BudgetSpent
	if summary.BudgetTotal > 0 {
		summary.BudgetPct = (summary.BudgetSpent / summary.BudgetTotal) * 100
	}
	summary.CalendarAlignment = budgetAlignmentForDate(summary.BudgetPct, time.Now())

	hasPrev, err := s.planExistsForYear(ctx, year-1)
	if err != nil {
		return &summary, err
	}
	summary.HasPrevYearPlan = hasPrev
	return &summary, nil
}

func (s *SQLStore) planExistsForYear(ctx context.Context, year int) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM training.training_plan WHERE year = $1)`, year).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("planning: prev plan check: %w", err)
	}
	return exists, nil
}

// budgetAlignmentForDate compares actual spent % with the expected % based on calendar progress.
// Returns one of: in_linea, in_ritardo (spent below expected → risk of underuse), in_anticipo.
func budgetAlignmentForDate(spentPct float64, now time.Time) string {
	dayOfYear := now.YearDay()
	totalDays := 365
	if y := now.Year(); y%4 == 0 && (y%100 != 0 || y%400 == 0) {
		totalDays = 366
	}
	expected := float64(dayOfYear) / float64(totalDays) * 100
	delta := spentPct - expected
	switch {
	case delta < -15:
		return "in_ritardo"
	case delta > 15:
		return "in_anticipo"
	default:
		return "in_linea"
	}
}

// BuildSuggestions aggregates gaps from compliance, expirations, skill targets, employee requests.
// Each suggestion has a stable signature so dismiss persistence survives recomputation.
func (s *SQLStore) BuildSuggestions(ctx context.Context, planID string, year int, team string) ([]PlanningSuggestion, error) {
	dismissed, err := s.loadDismissedSignatures(ctx, planID)
	if err != nil {
		return nil, err
	}

	out := make([]PlanningSuggestion, 0, 16)

	compliance, err := s.suggestionsCompliance(ctx, team)
	if err != nil {
		return nil, err
	}
	out = append(out, compliance...)

	expiring, err := s.suggestionsExpiring(ctx, team)
	if err != nil {
		return nil, err
	}
	out = append(out, expiring...)

	skills, err := s.suggestionsSkillGap(ctx, team)
	if err != nil {
		return nil, err
	}
	out = append(out, skills...)

	requests, err := s.suggestionsEmployeeRequests(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, requests...)

	final := make([]PlanningSuggestion, 0, len(out))
	for _, sug := range out {
		if _, ok := dismissed[sug.ID]; ok {
			sug.Dismissed = true
			continue
		}
		final = append(final, sug)
	}

	sort.SliceStable(final, func(i, j int) bool {
		return severityRank(final[i].Severity) < severityRank(final[j].Severity)
	})
	return final, nil
}

func severityRank(severity string) int {
	switch severity {
	case "critical":
		return 0
	case "warning":
		return 1
	case "info":
		return 2
	}
	return 99
}

func suggestionSignature(parts ...string) string {
	h := sha1.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte("|"))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func scanTextArray(value *pgtype.FlatArray[string]) sql.Scanner {
	return pgtype.NewMap().SQLScanner(value)
}

// suggestionsCompliance — aggregate missing mandatory rule coverage by course (rule).
func (s *SQLStore) suggestionsCompliance(ctx context.Context, team string) ([]PlanningSuggestion, error) {
	const q = `
SELECT
  g.rule_id::text,
  rule.name,
  g.course_id::text,
  g.course_title,
  COALESCE(g.compliance_framework, ''),
  COUNT(*) AS affected,
  array_agg(g.employee_id::text ORDER BY g.last_name, g.first_name) AS emp_ids,
  c.default_cost::float8,
  c.default_hours,
  CASE
    WHEN rule.population_target->>'kind' = 'custom_group' THEN rule.population_target->>'id'
    ELSE ''
  END AS source_custom_group_id
FROM training.v_mandatory_compliance_gap g
JOIN training.mandatory_rules rule ON rule.id = g.rule_id
JOIN training.course c ON c.id = g.course_id
LEFT JOIN training.team_membership tm
  ON tm.employee_id = g.employee_id
  AND tm.start_date <= now()
  AND (tm.end_date IS NULL OR tm.end_date >= now())
LEFT JOIN training.team t ON t.id = tm.team_id
WHERE g.compliance_status = 'missing_or_expired'
  AND ($1 = '' OR t.code = $1)
GROUP BY g.rule_id, rule.name, g.course_id, g.course_title, g.compliance_framework, c.default_cost, c.default_hours, rule.population_target
ORDER BY affected DESC`
	rows, err := s.db.QueryContext(ctx, q, team)
	if err != nil {
		return nil, fmt.Errorf("planning: compliance suggestions: %w", err)
	}
	defer rows.Close()
	result := make([]PlanningSuggestion, 0)
	for rows.Next() {
		var ruleID, ruleName, courseID, title, framework, sourceCustomGroupID string
		var affected int
		var empIDs pgtype.FlatArray[string]
		var cost sql.NullFloat64
		var hours sql.NullInt32
		if err := rows.Scan(&ruleID, &ruleName, &courseID, &title, &framework, &affected, scanTextArray(&empIDs), &cost, &hours, &sourceCustomGroupID); err != nil {
			return nil, fmt.Errorf("scan compliance suggestion: %w", err)
		}
		costVal := 0.0
		if cost.Valid {
			costVal = cost.Float64
		}
		sig := suggestionSignature("compliance", ruleID)
		displayTitle := strings.TrimSpace(ruleName)
		if displayTitle == "" {
			displayTitle = title
		}
		title2 := fmt.Sprintf("%d persone senza %s", affected, displayTitle)
		desc := framework
		if desc == "" {
			desc = "Regola non coperta"
		}
		sug := PlanningSuggestion{
			ID:                   sig,
			Severity:             "critical",
			Origin:               "compliance",
			Title:                title2,
			Description:          desc,
			AffectedCount:        affected,
			AffectedEmployeeIDs:  []string(empIDs),
			SuggestedCourseID:    courseID,
			SuggestedCourseName:  title,
			EstimatedCost:        costVal * float64(affected),
			AlternativeCourseIDs: []string{},
			RuleID:               ruleID,
			SourceCustomGroupID:  sourceCustomGroupID,
		}
		if hours.Valid {
			hv := int(hours.Int32)
			sug.SuggestedCourseHrs = &hv
		}
		if cost.Valid {
			c := cost.Float64
			sug.SuggestedCourseCost = &c
		}
		result = append(result, sug)
	}
	return result, rows.Err()
}

// suggestionsExpiring — aggregate certs expiring within 90 days by certification.
func (s *SQLStore) suggestionsExpiring(ctx context.Context, team string) ([]PlanningSuggestion, error) {
	const q = `
SELECT
  vec.cert_code,
  vec.cert_name,
  c.id::text                                      AS certification_id,
  COUNT(*)                                        AS affected,
  array_agg(vec.employee_id::text ORDER BY vec.last_name, vec.first_name) AS emp_ids,
  course.id::text                                 AS course_id,
  course.title                                    AS course_title,
  course.default_cost::float8                     AS course_cost,
  course.default_hours                            AS course_hours
FROM training.v_expiring_certifications vec
JOIN training.certification c ON c.code = vec.cert_code
LEFT JOIN training.course course ON course.leads_to_cert_id = c.id AND course.is_active
LEFT JOIN training.team_membership tm
  ON tm.employee_id = vec.employee_id
  AND tm.start_date <= now()
  AND (tm.end_date IS NULL OR tm.end_date >= now())
LEFT JOIN training.team t ON t.id = tm.team_id
WHERE vec.days_to_expiry <= 90
  AND ($1 = '' OR t.code = $1)
GROUP BY vec.cert_code, vec.cert_name, c.id, course.id, course.title, course.default_cost, course.default_hours`
	rows, err := s.db.QueryContext(ctx, q, team)
	if err != nil {
		return nil, fmt.Errorf("planning: expiring suggestions: %w", err)
	}
	defer rows.Close()
	result := make([]PlanningSuggestion, 0)
	for rows.Next() {
		var certCode, certName, certID string
		var affected int
		var empIDs pgtype.FlatArray[string]
		var courseID, courseTitle sql.NullString
		var courseCost sql.NullFloat64
		var courseHours sql.NullInt32
		if err := rows.Scan(&certCode, &certName, &certID, &affected, scanTextArray(&empIDs), &courseID, &courseTitle, &courseCost, &courseHours); err != nil {
			return nil, fmt.Errorf("scan expiring suggestion: %w", err)
		}
		sig := suggestionSignature("expiring", certID)
		sug := PlanningSuggestion{
			ID:                  sig,
			Severity:            "warning",
			Origin:              "expiring",
			Title:               fmt.Sprintf("%d certificazioni %s in scadenza", affected, certName),
			Description:         "Rinnovo entro 90 giorni",
			AffectedCount:       affected,
			AffectedEmployeeIDs: []string(empIDs),
		}
		if courseID.Valid {
			sug.SuggestedCourseID = courseID.String
		}
		if courseTitle.Valid {
			sug.SuggestedCourseName = courseTitle.String
		}
		if courseCost.Valid {
			c := courseCost.Float64
			sug.SuggestedCourseCost = &c
			sug.EstimatedCost = c * float64(affected)
		}
		if courseHours.Valid {
			h := int(courseHours.Int32)
			sug.SuggestedCourseHrs = &h
		}
		result = append(result, sug)
	}
	return result, rows.Err()
}

// suggestionsSkillGap — completed enrollments where employee declared levelToBe > levelAsIs and target not yet reached.
// Aggregated by the next target course in the same skill area.
func (s *SQLStore) suggestionsSkillGap(ctx context.Context, team string) ([]PlanningSuggestion, error) {
	const q = `
WITH gaps AS (
  SELECT
    en.employee_id,
    c.skill_area_id,
    sa.name AS skill_area_name,
    MAX(en.level_to_be) AS target_level
  FROM training.enrollment en
  JOIN training.course c ON c.id = en.course_id
  JOIN training.skill_area sa ON sa.id = c.skill_area_id
  LEFT JOIN training.team_membership tm
    ON tm.employee_id = en.employee_id
    AND tm.start_date <= now()
    AND (tm.end_date IS NULL OR tm.end_date >= now())
  LEFT JOIN training.team t ON t.id = tm.team_id
  WHERE en.status = 'completed'
    AND en.level_to_be IS NOT NULL
    AND en.level_as_is IS NOT NULL
    AND en.level_to_be > en.level_as_is
    AND ($1 = '' OR t.code = $1)
  GROUP BY en.employee_id, c.skill_area_id, sa.name
)
SELECT
  g.skill_area_id::text,
  g.skill_area_name,
  COUNT(DISTINCT g.employee_id) AS affected,
  array_agg(DISTINCT g.employee_id::text) AS emp_ids
FROM gaps g
GROUP BY g.skill_area_id, g.skill_area_name
HAVING COUNT(DISTINCT g.employee_id) > 0`
	rows, err := s.db.QueryContext(ctx, q, team)
	if err != nil {
		return nil, fmt.Errorf("planning: skill gap suggestions: %w", err)
	}
	defer rows.Close()
	result := make([]PlanningSuggestion, 0)
	for rows.Next() {
		var skillID, skillName string
		var affected int
		var empIDs pgtype.FlatArray[string]
		if err := rows.Scan(&skillID, &skillName, &affected, scanTextArray(&empIDs)); err != nil {
			return nil, fmt.Errorf("scan skill gap suggestion: %w", err)
		}
		sig := suggestionSignature("skill_gap", skillID)
		sug := PlanningSuggestion{
			ID:                  sig,
			Severity:            "warning",
			Origin:              "skill_gap",
			Title:               fmt.Sprintf("%d dipendenti con gap dichiarato su %s", affected, skillName),
			Description:         "Target di livello non raggiunto",
			AffectedCount:       affected,
			AffectedEmployeeIDs: []string(empIDs),
		}
		result = append(result, sug)
	}
	return result, rows.Err()
}

// suggestionsEmployeeRequests — pending requests aggregated as single item.
func (s *SQLStore) suggestionsEmployeeRequests(ctx context.Context) ([]PlanningSuggestion, error) {
	const q = `
SELECT COUNT(*), COALESCE(array_agg(id::text ORDER BY created_at DESC), ARRAY[]::text[])
FROM training.training_request
WHERE status IN ('submitted', 'under_review')`
	var affected int
	var ids pgtype.FlatArray[string]
	if err := s.db.QueryRowContext(ctx, q).Scan(&affected, scanTextArray(&ids)); err != nil {
		return nil, fmt.Errorf("planning: employee request suggestions: %w", err)
	}
	if affected == 0 {
		return []PlanningSuggestion{}, nil
	}
	sig := suggestionSignature("employee_request", "queue")
	return []PlanningSuggestion{{
		ID:                  sig,
		Severity:            "info",
		Origin:              "employee_request",
		Title:               fmt.Sprintf("%d richieste employee in attesa", affected),
		Description:         "Da approvare o rifiutare",
		AffectedCount:       affected,
		AffectedEmployeeIDs: []string(ids),
	}}, nil
}

func (s *SQLStore) loadDismissedSignatures(ctx context.Context, planID string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	if strings.TrimSpace(planID) == "" {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT signature FROM training.planning_suggestion_dismiss WHERE plan_id = $1::uuid`, planID)
	if err != nil {
		return nil, fmt.Errorf("planning: load dismissed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sig string
		if err := rows.Scan(&sig); err != nil {
			return nil, fmt.Errorf("scan dismissed signature: %w", err)
		}
		out[sig] = struct{}{}
	}
	return out, rows.Err()
}

// DismissSuggestion records a dismiss for plan_id + signature.
func (s *SQLStore) DismissSuggestion(ctx context.Context, principal Principal, planID, signature string) error {
	if !principal.IsPeopleAdmin {
		return forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(planID) == "" || strings.TrimSpace(signature) == "" {
		return validationError("missing_required_fields", "piano e suggerimento obbligatori")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		actorID := s.actorEmployeeID(ctx, tx, principal)
		_, err := tx.ExecContext(ctx, `
INSERT INTO training.planning_suggestion_dismiss (plan_id, signature, dismissed_by)
VALUES ($1::uuid, $2, $3::uuid)
ON CONFLICT (plan_id, signature) DO NOTHING`, planID, signature, nullableUUIDPtr(actorID))
		if err != nil {
			return fmt.Errorf("dismiss suggestion: %w", err)
		}
		return emitPlanAuditEvent(ctx, tx, principal, planID, PlanAuditSuggestionDismiss, map[string]any{
			"suggestion_id": signature,
		})
	})
}

// CreatePlan creates a new training_plan row. If `duplicateFrom` is non-nil, copies budget_total from that year.
func (s *SQLStore) CreatePlan(ctx context.Context, principal Principal, input CreatePlanInput) (TrainingPlanRow, error) {
	if !principal.IsPeopleAdmin {
		return TrainingPlanRow{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if input.Year < 2020 || input.Year > 2100 {
		return TrainingPlanRow{}, validationError("invalid_year", "anno non valido")
	}

	budget := input.BudgetTotal
	if input.DuplicateFrom != nil && budget == nil {
		var prev sql.NullFloat64
		err := s.db.QueryRowContext(ctx, `SELECT budget_total FROM training.training_plan WHERE year = $1`, *input.DuplicateFrom).Scan(&prev)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return TrainingPlanRow{}, fmt.Errorf("planning: duplicate src: %w", err)
		}
		if prev.Valid {
			b := prev.Float64
			budget = &b
		}
	}

	var row TrainingPlanRow
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
INSERT INTO training.training_plan (year, status, budget_total)
VALUES ($1, 'draft'::training.plan_status, $2)
RETURNING id::text, year, status::text, budget_total::float8`,
			input.Year, budget,
		).Scan(&row.ID, &row.Year, &row.Status, &row.BudgetTotal)
		if err != nil {
			if isUniqueViolation(err, "training_plan_year_key") {
				return conflictError("plan_year_exists", fmt.Sprintf("esiste gia un piano per il %d", input.Year))
			}
			return fmt.Errorf("planning: insert plan: %w", err)
		}
		afterSnap, err := entitySnapshot(ctx, tx, "training_plan", row.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_plan", row.ID, "create", nil, afterSnap); err != nil {
			return err
		}
		return emitPlanAuditEvent(ctx, tx, principal, row.ID, PlanAuditPlanCreated, map[string]any{
			"year":         row.Year,
			"status":       row.Status,
			"budget_total": row.BudgetTotal,
		})
	})
	return row, err
}

func validateUpdatePlanInput(input UpdatePlanInput) error {
	if input.BudgetTotal == nil && input.Notes == nil {
		return validationError("empty_update", "nessuna modifica indicata")
	}
	if input.BudgetTotal != nil && *input.BudgetTotal < 0 {
		return validationError("invalid_budget", "budget non valido")
	}
	return nil
}

func (s *SQLStore) UpdatePlan(ctx context.Context, principal Principal, planID string, input UpdatePlanInput) (UpdatePlanResponse, error) {
	if !principal.IsPeopleAdmin {
		return UpdatePlanResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(planID) == "" {
		return UpdatePlanResponse{}, validationError("missing_plan_id", "id piano obbligatorio")
	}
	if err := validateUpdatePlanInput(input); err != nil {
		return UpdatePlanResponse{}, err
	}
	response := UpdatePlanResponse{OK: true, PlanID: planID}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var year int
		var status string
		var beforeBudget sql.NullFloat64
		var beforeNotes string
		var spent float64
		err := tx.QueryRowContext(ctx, `
SELECT year, status::text, budget_total::float8, COALESCE(notes, '')
FROM training.training_plan
WHERE id = $1::uuid
FOR UPDATE`, planID).Scan(&year, &status, &beforeBudget, &beforeNotes)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("plan_not_found", "piano non trovato")
		}
		if err != nil {
			return fmt.Errorf("load training plan for update: %w", err)
		}
		if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(COALESCE(en.cost_actual, en.cost_planned, c.default_cost, 0)), 0)::float8
FROM training.enrollment en
LEFT JOIN training.course c ON c.id = en.course_id
WHERE en.training_plan_id = $1::uuid
  AND en.status <> 'cancelled'`, planID).Scan(&spent); err != nil {
			return fmt.Errorf("load training plan spent: %w", err)
		}
		if status != "draft" && status != "open" {
			return conflictError("plan_not_editable", "piano non modificabile")
		}
		if input.BudgetTotal != nil && *input.BudgetTotal < spent {
			response.Warnings = append(response.Warnings, "budget_below_spent")
		}

		before, err := entitySnapshot(ctx, tx, "training_plan", planID)
		if err != nil {
			return err
		}
		var budgetValue any
		if input.BudgetTotal != nil {
			budgetValue = *input.BudgetTotal
		}
		notesValue := ""
		if input.Notes != nil {
			notesValue = strings.TrimSpace(*input.Notes)
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE training.training_plan
SET budget_total = CASE WHEN $2::boolean THEN $3::numeric ELSE budget_total END,
    notes = CASE WHEN $4::boolean THEN NULLIF($5, '') ELSE notes END,
    updated_at = now()
WHERE id = $1::uuid`,
			planID,
			input.BudgetTotal != nil,
			budgetValue,
			input.Notes != nil,
			notesValue,
		); err != nil {
			return fmt.Errorf("update training plan: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "training_plan", planID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_plan", planID, "update", before, after); err != nil {
			return err
		}
		if input.BudgetTotal != nil {
			var beforeValue any
			if beforeBudget.Valid {
				beforeValue = beforeBudget.Float64
			}
			if err := emitPlanAuditEvent(ctx, tx, principal, planID, PlanAuditBudgetChanged, map[string]any{
				"year":   year,
				"from":   beforeValue,
				"to":     *input.BudgetTotal,
				"spent":  spent,
				"status": status,
			}); err != nil {
				return err
			}
		}
		if input.Notes != nil && strings.TrimSpace(beforeNotes) != notesValue {
			if err := emitPlanAuditEvent(ctx, tx, principal, planID, PlanAuditNotesChanged, map[string]any{
				"year":   year,
				"status": status,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return response, err
}

func canDeletePlan(status string, enrollments int) bool {
	return status == "draft" && enrollments == 0
}

func (s *SQLStore) DeletePlan(ctx context.Context, principal Principal, planID string) error {
	if !principal.IsPeopleAdmin {
		return forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(planID) == "" {
		return validationError("missing_plan_id", "id piano obbligatorio")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var year int
		var status string
		var budget sql.NullFloat64
		var enrollments int
		err := tx.QueryRowContext(ctx, `
SELECT year, status::text, budget_total::float8
FROM training.training_plan
WHERE id = $1::uuid
FOR UPDATE`, planID).Scan(&year, &status, &budget)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("plan_not_found", "piano non trovato")
		}
		if err != nil {
			return fmt.Errorf("load training plan for delete: %w", err)
		}
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM training.enrollment
WHERE training_plan_id = $1::uuid`, planID).Scan(&enrollments); err != nil {
			return fmt.Errorf("count training plan enrollments: %w", err)
		}
		if !canDeletePlan(status, enrollments) {
			return conflictError("plan_delete_not_allowed", "puoi eliminare solo bozze senza iscrizioni")
		}
		before, err := entitySnapshot(ctx, tx, "training_plan", planID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_plan", planID, "delete", before, nil); err != nil {
			return err
		}
		var budgetValue any
		if budget.Valid {
			budgetValue = budget.Float64
		}
		if err := emitPlanAuditEvent(ctx, tx, principal, planID, PlanAuditPlanDeleted, map[string]any{
			"year":         year,
			"status":       status,
			"budget_total": budgetValue,
		}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM training.training_plan WHERE id = $1::uuid`, planID); err != nil {
			return fmt.Errorf("delete training plan: %w", err)
		}
		return nil
	})
}

func (s *SQLStore) anotherOpenPlanError(ctx context.Context, q sqlRunner, currentPlanID string) error {
	var row TrainingPlanListRow
	var createdAt time.Time
	err := q.QueryRowContext(ctx, `
SELECT id::text, year, status::text, COALESCE(budget_total, 0)::float8, created_at
FROM training.training_plan
WHERE status = 'open'::training.plan_status
  AND id <> $1::uuid
ORDER BY created_at DESC
LIMIT 1`, currentPlanID).Scan(&row.ID, &row.Year, &row.Status, &row.BudgetTotal, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return conflictError("another_plan_open", "esiste gia un piano aperto")
	}
	if err != nil {
		return fmt.Errorf("load open training plan conflict: %w", err)
	}
	row.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return anotherPlanOpenError{existing: row, message: fmt.Sprintf("esiste gia un piano aperto per il %d", row.Year)}
}

func isOneOpenPlanUniqueViolation(err error) bool {
	return isUniqueViolation(err, "idx_training_one_open_plan")
}

// TransitionPlan: draft → open, open → closed (expires enrollments), closed → reopened (back to open).
func (s *SQLStore) TransitionPlan(ctx context.Context, principal Principal, planID, target string) (TransitionPlanResponse, error) {
	if !principal.IsPeopleAdmin {
		return TransitionPlanResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(planID) == "" {
		return TransitionPlanResponse{}, validationError("missing_plan_id", "id piano obbligatorio")
	}

	var response TransitionPlanResponse
	response.PlanID = planID
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var current string
		if err := tx.QueryRowContext(ctx, `SELECT status::text FROM training.training_plan WHERE id = $1::uuid FOR UPDATE`, planID).Scan(&current); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return notFoundError("plan_not_found", "piano non trovato")
			}
			return fmt.Errorf("load plan status: %w", err)
		}

		newStatus, expireEnrollments, err := planNextStatus(current, target)
		if err != nil {
			return err
		}
		if newStatus == "open" && current != "open" {
			if err := s.anotherOpenPlanError(ctx, tx, planID); err != nil {
				if _, ok := asAppError(err); ok {
					// No existing open plan was found. Continue and let the unique index
					// cover the race between this check and the status update.
				} else if _, ok := asAnotherPlanOpenError(err); ok {
					return err
				} else {
					return err
				}
			}
		}

		if expireEnrollments {
			// Force-expire any active enrollment via override (system action triggered by admin).
			if _, err := tx.ExecContext(ctx, `SELECT set_config('training.allow_status_override','true',true)`); err != nil {
				return fmt.Errorf("set override: %w", err)
			}
			res, err := tx.ExecContext(ctx, `
UPDATE training.enrollment
SET status = 'expired'::training.enrollment_status,
    updated_at = now()
WHERE training_plan_id = $1::uuid
  AND status IN ('proposed', 'approved')`, planID)
			if err != nil {
				return fmt.Errorf("expire enrollments: %w", err)
			}
			affected, _ := res.RowsAffected()
			response.ExpiredEnrollmentsCount = int(affected)
		}

		before, err := entitySnapshot(ctx, tx, "training_plan", planID)
		if err != nil {
			return err
		}
		setOpenedAt := ""
		setClosedAt := ""
		switch newStatus {
		case "open":
			if current == "draft" {
				setOpenedAt = ", opened_at = now()"
			}
		case "closed":
			setClosedAt = ", closed_at = now()"
		}
		stmt := fmt.Sprintf(`UPDATE training.training_plan SET status = $2::training.plan_status, updated_at = now()%s%s WHERE id = $1::uuid RETURNING status::text`,
			setOpenedAt, setClosedAt)
		if err := tx.QueryRowContext(ctx, stmt, planID, newStatus).Scan(&response.Status); err != nil {
			if isOneOpenPlanUniqueViolation(err) {
				return errAnotherPlanOpenRace
			}
			return fmt.Errorf("update plan status: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "training_plan", planID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_plan", planID, "transition:"+target, before, after); err != nil {
			return err
		}
		if current != response.Status {
			if err := emitPlanAuditEvent(ctx, tx, principal, planID, PlanAuditStatusChanged, map[string]any{
				"from":                      current,
				"to":                        response.Status,
				"target":                    target,
				"expired_enrollments_count": response.ExpiredEnrollmentsCount,
			}); err != nil {
				return err
			}
		}
		response.OK = true
		return nil
	})
	if errors.Is(err, errAnotherPlanOpenRace) {
		return response, s.anotherOpenPlanError(ctx, s.db, planID)
	}
	return response, err
}

func planNextStatus(current, target string) (string, bool, error) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "open":
		if current == "draft" || current == "frozen" {
			return "open", false, nil
		}
		if current == "open" {
			return "open", false, nil
		}
	case "closed":
		if current == "open" || current == "frozen" {
			return "closed", true, nil
		}
	case "reopened":
		if current == "closed" {
			return "open", false, nil
		}
	case "frozen":
		if current == "open" {
			return "frozen", false, nil
		}
	}
	return "", false, validationError("invalid_transition", fmt.Sprintf("transizione non consentita: %s → %s", current, target))
}

// CourseDefaults loads default cost+hours for a course (used by bulk plan from suggestion).
func (s *SQLStore) CourseDefaults(ctx context.Context, courseID string) (cost *float64, hours *int, err error) {
	var c sql.NullFloat64
	var h sql.NullInt32
	err = s.db.QueryRowContext(ctx, `SELECT default_cost, default_hours FROM training.course WHERE id = $1::uuid`, courseID).Scan(&c, &h)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, notFoundError("course_not_found", "corso non trovato")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load course defaults: %w", err)
	}
	if c.Valid {
		v := c.Float64
		cost = &v
	}
	if h.Valid {
		v := int(h.Int32)
		hours = &v
	}
	return cost, hours, nil
}

// BulkPlanFromSuggestion creates N enrollments in `approved` state for the open plan of `year`.
// Each enrollment is created in `proposed`, then transitioned to `approved` via the state machine.
func (s *SQLStore) BulkPlanFromSuggestion(ctx context.Context, principal Principal, year int, input BulkPlanFromSuggestionInput) (BulkAssignResponse, error) {
	if !principal.IsPeopleAdmin {
		return BulkAssignResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(input.CourseID) == "" {
		return BulkAssignResponse{}, validationError("missing_course_id", "corso obbligatorio")
	}
	if len(input.EmployeeIDs) == 0 && strings.TrimSpace(input.SourceCustomGroupID) != "" {
		ids, err := s.CustomGroupMemberIDs(ctx, s.db, input.SourceCustomGroupID)
		if err != nil {
			return BulkAssignResponse{}, err
		}
		input.EmployeeIDs = ids
	}
	if len(input.EmployeeIDs) == 0 {
		return BulkAssignResponse{}, validationError("missing_employee_ids", "indica almeno una persona")
	}
	planID, err := s.TrainingPlanIDByYear(ctx, year)
	if err != nil {
		return BulkAssignResponse{}, err
	}

	response := BulkAssignResponse{Failures: []BulkAssignFailure{}}
	for _, employeeID := range input.EmployeeIDs {
		empID := strings.TrimSpace(employeeID)
		created, err := s.CreateEnrollment(ctx, principal, EnrollmentInput{
			EmployeeID:          empID,
			CourseID:            input.CourseID,
			TrainingPlanID:      planID,
			PlannedStart:        input.PlanParams.PlannedStart,
			PlannedEnd:          input.PlanParams.PlannedEnd,
			HoursPlanned:        input.PlanParams.HoursPlanned,
			CostPlanned:         input.PlanParams.CostPlanned,
			MandatoryRuleID:     input.MandatoryRuleID,
			SourceCustomGroupID: input.SourceCustomGroupID,
		})
		if err != nil {
			response.Failed++
			response.Failures = append(response.Failures, makeFailure(empID, err))
			continue
		}
		if _, err := s.TransitionEnrollment(ctx, principal, created.ID, EnrollmentTransitionInput{Transition: "approve"}); err != nil {
			response.Failed++
			response.Failures = append(response.Failures, makeFailure(empID, err))
			continue
		}
		response.Created++
	}
	eventType := bulkPlanAuditEventType(input.SuggestionID)
	payload := map[string]any{
		"suggestion_id":          normalizeSuggestionID(input.SuggestionID),
		"course_id":              input.CourseID,
		"employee_count":         len(input.EmployeeIDs),
		"created":                response.Created,
		"failed":                 response.Failed,
		"mandatory_rule_id":      strings.TrimSpace(input.MandatoryRuleID),
		"source_custom_group_id": strings.TrimSpace(input.SourceCustomGroupID),
	}
	if normalizeSuggestionID(input.SuggestionID) == "" {
		delete(payload, "suggestion_id")
	}
	if strings.TrimSpace(input.MandatoryRuleID) == "" {
		delete(payload, "mandatory_rule_id")
	}
	if strings.TrimSpace(input.SourceCustomGroupID) == "" {
		delete(payload, "source_custom_group_id")
	}
	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		return emitPlanAuditEvent(ctx, tx, principal, planID, eventType, payload)
	}); err != nil {
		return response, err
	}
	return response, nil
}

func makeFailure(id string, err error) BulkAssignFailure {
	f := BulkAssignFailure{EmployeeID: id, Message: err.Error()}
	if appErr, ok := asAppError(err); ok {
		f.Code = appErr.code
		f.Message = appErr.message
	}
	return f
}

// BulkReviewEmployeeRequests transitions N training_request to accepted/rejected (and converts accepted to enrollments if courseID provided).
func (s *SQLStore) BulkReviewEmployeeRequests(ctx context.Context, principal Principal, year int, input BulkReviewEmployeeRequestsInput) (BulkReviewEmployeeRequestsResponse, error) {
	if !principal.IsPeopleAdmin {
		return BulkReviewEmployeeRequestsResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if len(input.RequestIDs) == 0 {
		return BulkReviewEmployeeRequestsResponse{}, validationError("missing_request_ids", "indica almeno una richiesta")
	}
	target := strings.ToLower(strings.TrimSpace(input.Target))
	if target != "approved" && target != "rejected" {
		return BulkReviewEmployeeRequestsResponse{}, validationError("invalid_target", "target deve essere approved o rejected")
	}
	if target == "rejected" && strings.TrimSpace(input.Motivation) == "" {
		return BulkReviewEmployeeRequestsResponse{}, validationError("motivation_required", "motivazione obbligatoria per il rifiuto")
	}

	resp := BulkReviewEmployeeRequestsResponse{Failures: []BulkAssignFailure{}}

	for _, rid := range input.RequestIDs {
		id := strings.TrimSpace(rid)
		if target == "rejected" {
			if _, err := s.TransitionTrainingRequest(ctx, principal, id, TrainingRequestTransitionInput{Transition: "reject", Reason: input.Motivation}); err != nil {
				resp.Failed++
				resp.Failures = append(resp.Failures, makeFailure(id, err))
				continue
			}
			resp.Succeeded++
			continue
		}

		if _, err := s.TransitionTrainingRequest(ctx, principal, id, TrainingRequestTransitionInput{Transition: "start_review"}); err != nil {
			if !isAlreadyInState(err) {
				resp.Failed++
				resp.Failures = append(resp.Failures, makeFailure(id, err))
				continue
			}
		}
		if _, err := s.TransitionTrainingRequest(ctx, principal, id, TrainingRequestTransitionInput{Transition: "accept"}); err != nil {
			resp.Failed++
			resp.Failures = append(resp.Failures, makeFailure(id, err))
			continue
		}
		planID, err := s.TrainingPlanIDByYear(ctx, year)
		if err == nil {
			if _, err := s.TransitionTrainingRequest(ctx, principal, id, TrainingRequestTransitionInput{Transition: "convert", TrainingPlanID: planID}); err != nil && !isAlreadyInState(err) {
				resp.Failed++
				resp.Failures = append(resp.Failures, makeFailure(id, err))
				continue
			}
		}
		resp.Succeeded++
	}
	if planID, err := s.TrainingPlanIDByYear(ctx, year); err == nil {
		if auditErr := s.withTx(ctx, func(tx *sql.Tx) error {
			return emitPlanAuditEvent(ctx, tx, principal, planID, PlanAuditBulkReviewApplied, map[string]any{
				"request_count": len(input.RequestIDs),
				"target":        target,
				"succeeded":     resp.Succeeded,
				"failed":        resp.Failed,
			})
		}); auditErr != nil {
			return resp, auditErr
		}
	}
	return resp, nil
}

func isAlreadyInState(err error) bool {
	if err == nil {
		return false
	}
	if appErr, ok := asAppError(err); ok {
		return appErr.code == "INVALID_TRANSITION"
	}
	return false
}
