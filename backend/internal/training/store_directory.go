package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *SQLStore) TrainingPlanIDByYear(ctx context.Context, year int) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id::text FROM training.training_plan WHERE year = $1 LIMIT 1`, year).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", notFoundError("plan_not_found", fmt.Sprintf("piano formativo %d non trovato", year))
	}
	if err != nil {
		return "", fmt.Errorf("load training plan by year: %w", err)
	}
	return id, nil
}

func (s *SQLStore) ListPeopleDirectory(ctx context.Context, principal Principal, filters PeopleDirectoryFilters) ([]PersonSummary, error) {
	effectiveYear := effectiveDirectoryYear(filters.Year, time.Now())
	const q = `
WITH active_emp AS (
  SELECT
    e.id,
    e.first_name,
    e.last_name,
    e.email::text AS email,
    tm.team_id
  FROM training.employee e
  LEFT JOIN training.team_membership tm
    ON tm.employee_id = e.id
    AND tm.start_date <= now()
    AND (tm.end_date IS NULL OR tm.end_date >= now())
  WHERE e.status = 'active'
),
year_plan AS (
  SELECT id
  FROM training.training_plan
  WHERE year = $1
  LIMIT 1
),
active_enrollments AS (
  SELECT employee_id, COUNT(*) AS active_count
  FROM training.enrollment en
  JOIN year_plan yp ON yp.id = en.training_plan_id
  WHERE en.status IN ('proposed','approved','in_progress')
  GROUP BY employee_id
),
history AS (
  SELECT employee_id, COUNT(*) AS hist_count
  FROM training.enrollment
  GROUP BY employee_id
),
required_mandatory AS (
  SELECT
    ae.id AS employee_id,
    rule.id AS rule_id,
    c.id AS course_id,
    c.leads_to_cert_id,
    c.recurrence_interval
  FROM active_emp ae
  JOIN training.v_mandatory_rule_population population
    ON population.employee_id = ae.id
  JOIN training.mandatory_rules rule
    ON rule.id = population.rule_id
  JOIN training.course c
    ON c.id = rule.course_id
    AND c.is_active
),
mandatory_gaps AS (
  SELECT employee_id, rule_id, course_id
  FROM training.v_mandatory_compliance_gap
  WHERE compliance_status <> 'compliant'
),
gap_summary AS (
  SELECT
    mg.employee_id,
    COUNT(*) AS gap_count,
    BOOL_OR(NOT EXISTS (
      SELECT 1
      FROM training.enrollment en
      JOIN year_plan yp ON yp.id = en.training_plan_id
      WHERE en.employee_id = mg.employee_id
        AND en.course_id = mg.course_id
        AND en.status IN ('proposed','approved','in_progress')
    )) AS da_pianificare
  FROM mandatory_gaps mg
  GROUP BY employee_id
),
failed AS (
  SELECT en.employee_id, COUNT(*) AS failed_count
  FROM training.enrollment en
  JOIN year_plan yp ON yp.id = en.training_plan_id
  WHERE en.status = 'failed'
  GROUP BY en.employee_id
),
cert_expiring_events AS (
  SELECT
    ca.employee_id,
    ca.expires_on AS deadline
  FROM training.certification_award ca
  WHERE ca.outcome = 'passed_exam'
    AND ca.expires_on IS NOT NULL
    AND ca.expires_on > CURRENT_DATE
    AND ca.expires_on <= CURRENT_DATE + INTERVAL '60 days'
),
mandatory_recurrence_events AS (
  SELECT
    rm.employee_id,
    (latest.awarded_on + rm.recurrence_interval)::date AS deadline
  FROM required_mandatory rm
  JOIN LATERAL (
    SELECT ca.awarded_on
    FROM training.certification_award ca
    WHERE ca.employee_id = rm.employee_id
      AND ca.certification_id = rm.leads_to_cert_id
      AND ca.outcome = 'passed_exam'
    ORDER BY ca.awarded_on DESC
    LIMIT 1
  ) latest ON true
  WHERE rm.leads_to_cert_id IS NOT NULL
    AND rm.recurrence_interval IS NOT NULL
    AND latest.awarded_on + rm.recurrence_interval > CURRENT_DATE
    AND latest.awarded_on + rm.recurrence_interval <= CURRENT_DATE + INTERVAL '60 days'
),
expiring_events AS (
  SELECT employee_id, 'cert' AS deadline_type, deadline, 'Cert in scadenza' AS deadline_label
  FROM cert_expiring_events
  UNION ALL
  SELECT employee_id, 'mandatory_due' AS deadline_type, deadline, 'Ricorrenza obbligatoria' AS deadline_label
  FROM mandatory_recurrence_events
),
expiring AS (
  SELECT employee_id, COUNT(*) AS exp_count
  FROM expiring_events
  GROUP BY employee_id
),
next_course_end AS (
  SELECT employee_id, MIN(planned_end) AS next_planned_end
  FROM training.enrollment en
  JOIN year_plan yp ON yp.id = en.training_plan_id
  WHERE en.status IN ('approved','in_progress')
    AND en.planned_end IS NOT NULL
    AND en.planned_end >= CURRENT_DATE
  GROUP BY en.employee_id
),
deadline_events AS (
  SELECT employee_id, deadline_type, deadline, deadline_label
  FROM expiring_events
  UNION ALL
  SELECT employee_id, 'course_end' AS deadline_type, next_planned_end AS deadline, 'Fine corso prevista' AS deadline_label
  FROM next_course_end
),
next_deadline AS (
  SELECT DISTINCT ON (employee_id)
    employee_id,
    deadline_type,
    deadline::text AS deadline_date,
    deadline_label
  FROM deadline_events
  ORDER BY employee_id, deadline, deadline_type
)
SELECT
  ae.id::text,
  ae.last_name || ' ' || ae.first_name AS name,
  ae.email,
  COALESCE(t.code, '') AS team_code,
  COALESCE(g.gap_count, 0) AS gaps_open,
  COALESCE(ac.active_count, 0) AS active_enrollments_count,
  COALESCE(ex.exp_count, 0) AS expiring_certs_count,
  COALESCE(h.hist_count, 0) AS hist_count,
  COALESCE(g.da_pianificare, false) AS da_pianificare,
  COALESCE(g.gap_count, 0) > 0 AS compliance_gap,
  COALESCE(ex.exp_count, 0) > 0 AS scadenze_imminenti,
  COALESCE(f.failed_count, 0) > 0 AS failed_recente,
  COALESCE(ac.active_count, 0) = 0 AS senza_formazione_attiva,
  nd.deadline_type,
  nd.deadline_date,
  nd.deadline_label
FROM active_emp ae
LEFT JOIN training.team t ON t.id = ae.team_id
LEFT JOIN active_enrollments ac ON ac.employee_id = ae.id
LEFT JOIN history h ON h.employee_id = ae.id
LEFT JOIN gap_summary g ON g.employee_id = ae.id
LEFT JOIN expiring ex ON ex.employee_id = ae.id
LEFT JOIN failed f ON f.employee_id = ae.id
LEFT JOIN next_deadline nd ON nd.employee_id = ae.id
WHERE ($2 = '' OR t.code = $2)
  AND ($3 = '' OR ae.last_name || ' ' || ae.first_name ILIKE '%' || $3 || '%' OR ae.email::text ILIKE '%' || $3 || '%')
  AND ($4 = '' OR EXISTS (
    SELECT 1
    FROM training.custom_group_members group_filter
    WHERE group_filter.group_id = $4::uuid
      AND group_filter.employee_id = ae.id
  ))
ORDER BY ae.last_name, ae.first_name
LIMIT 500`

	rows, err := s.db.QueryContext(ctx, q, effectiveYear, filters.Team, filters.Search, filters.Group)
	if err != nil {
		return nil, fmt.Errorf("list training people directory: %w", err)
	}
	defer rows.Close()

	result := make([]PersonSummary, 0)
	for rows.Next() {
		var (
			summary       PersonSummary
			deadlineType  sql.NullString
			deadlineDate  sql.NullString
			deadlineLabel sql.NullString
		)
		if err := rows.Scan(
			&summary.ID,
			&summary.Name,
			&summary.Email,
			&summary.TeamCode,
			&summary.GapsOpen,
			&summary.ActiveEnrollmentsCount,
			&summary.ExpiringCertsCount,
			&summary.HistoricalEnrollments,
			&summary.Flags.DaPianificare,
			&summary.Flags.ComplianceGap,
			&summary.Flags.ScadenzeImminenti,
			&summary.Flags.FailedRecente,
			&summary.Flags.SenzaFormazioneAttiva,
			&deadlineType,
			&deadlineDate,
			&deadlineLabel,
		); err != nil {
			return nil, fmt.Errorf("scan training person summary: %w", err)
		}
		summary.NextDeadline = personNextDeadline(deadlineType, deadlineDate, deadlineLabel)
		summary.PriorityScore = computePersonPriorityScore(summary)
		if filterMatches(summary, filters) {
			result = append(result, summary)
		}
	}
	return result, rows.Err()
}

func effectiveDirectoryYear(requested int, now time.Time) int {
	if requested > 0 {
		return requested
	}
	return now.Year()
}

func personNextDeadline(deadlineType sql.NullString, deadlineDate sql.NullString, deadlineLabel sql.NullString) *PersonNextDeadline {
	if !deadlineType.Valid || !deadlineDate.Valid {
		return nil
	}
	label := deadlineLabel.String
	if label == "" {
		label = "Scadenza"
	}
	return &PersonNextDeadline{Type: deadlineType.String, Date: deadlineDate.String, Label: label}
}

func computePersonPriorityScore(summary PersonSummary) float64 {
	score := 0.0
	switch dominantPersonFlag(summary.Flags) {
	case "da_pianificare":
		score = 5000
	case "compliance_gap":
		score = 4000
	case "scadenze_imminenti":
		score = 3000
	case "failed_recente":
		score = 2500
	case "senza_formazione_attiva":
		score = 1000
	}
	score += float64(summary.GapsOpen) * 10
	score += float64(summary.ExpiringCertsCount) * 5
	if summary.Flags.FailedRecente {
		score += 3
	}
	if summary.Flags.SenzaFormazioneAttiva {
		score += 1
	}
	return score
}

func dominantPersonFlag(flags PersonFlags) string {
	switch {
	case flags.DaPianificare:
		return "da_pianificare"
	case flags.ComplianceGap:
		return "compliance_gap"
	case flags.ScadenzeImminenti:
		return "scadenze_imminenti"
	case flags.FailedRecente:
		return "failed_recente"
	case flags.SenzaFormazioneAttiva:
		return "senza_formazione_attiva"
	default:
		return ""
	}
}

func filterMatches(summary PersonSummary, filters PeopleDirectoryFilters) bool {
	switch filters.Filter {
	case "":
		return true
	case "da_pianificare":
		return summary.Flags.DaPianificare
	case "compliance_gap":
		return summary.Flags.ComplianceGap
	case "scadenze_imminenti":
		return summary.Flags.ScadenzeImminenti
	case "failed_recente":
		return summary.Flags.FailedRecente
	case "senza_formazione_attiva":
		return summary.Flags.SenzaFormazioneAttiva
	default:
		return true
	}
}
