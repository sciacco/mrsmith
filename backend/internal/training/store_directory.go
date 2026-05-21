package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
active_enrollments AS (
  SELECT employee_id, COUNT(*) AS active_count
  FROM training.enrollment en
  JOIN training.training_plan tp ON tp.id = en.training_plan_id
  WHERE en.status IN ('proposed','approved','in_progress')
    AND ($1 = 0 OR tp.year = $1)
  GROUP BY employee_id
),
history AS (
  SELECT employee_id, COUNT(*) AS hist_count
  FROM training.enrollment
  GROUP BY employee_id
),
gaps AS (
  SELECT employee_id, COUNT(*) FILTER (WHERE compliance_status = 'missing_or_expired') AS gap_count
  FROM training.v_mandatory_compliance_gap
  GROUP BY employee_id
),
expiring AS (
  SELECT employee_id, COUNT(*) AS exp_count, MIN(expires_on) AS soonest_cert_expiry
  FROM training.v_expiring_certifications
  WHERE days_to_expiry <= 60
  GROUP BY employee_id
),
next_course_end AS (
  SELECT employee_id, MIN(planned_end) AS next_planned_end
  FROM training.enrollment
  WHERE status IN ('approved','in_progress')
    AND planned_end IS NOT NULL
    AND planned_end >= CURRENT_DATE
  GROUP BY employee_id
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
  ex.soonest_cert_expiry::text AS soonest_cert_expiry,
  nc.next_planned_end::text AS next_planned_end
FROM active_emp ae
LEFT JOIN training.team t ON t.id = ae.team_id
LEFT JOIN active_enrollments ac ON ac.employee_id = ae.id
LEFT JOIN history h ON h.employee_id = ae.id
LEFT JOIN gaps g ON g.employee_id = ae.id
LEFT JOIN expiring ex ON ex.employee_id = ae.id
LEFT JOIN next_course_end nc ON nc.employee_id = ae.id
WHERE ($2 = '' OR t.code = $2)
ORDER BY ae.last_name, ae.first_name
LIMIT 500`

	rows, err := s.db.QueryContext(ctx, q, filters.Year, filters.Team)
	if err != nil {
		return nil, fmt.Errorf("list training people directory: %w", err)
	}
	defer rows.Close()

	result := make([]PersonSummary, 0)
	for rows.Next() {
		var (
			summary       PersonSummary
			soonestCert   sql.NullString
			nextCourseEnd sql.NullString
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
			&soonestCert,
			&nextCourseEnd,
		); err != nil {
			return nil, fmt.Errorf("scan training person summary: %w", err)
		}
		summary.ComplianceStatus = derivePersonComplianceStatus(summary)
		summary.NextDeadline = pickNextDeadline(soonestCert, nextCourseEnd)
		summary.PriorityScore = computePersonPriorityScore(summary)
		if filterMatches(summary, filters) {
			result = append(result, summary)
		}
	}
	return result, rows.Err()
}

func derivePersonComplianceStatus(summary PersonSummary) string {
	if summary.GapsOpen > 0 {
		return "con_gap"
	}
	if summary.HistoricalEnrollments == 0 {
		return "nuovo_assunto"
	}
	if summary.ActiveEnrollmentsCount == 0 {
		return "senza_piano"
	}
	return "a_norma"
}

func pickNextDeadline(certExpiry sql.NullString, courseEnd sql.NullString) *PersonNextDeadline {
	switch {
	case certExpiry.Valid && courseEnd.Valid:
		if certExpiry.String <= courseEnd.String {
			return &PersonNextDeadline{Type: "cert", Date: certExpiry.String, Label: "Cert in scadenza"}
		}
		return &PersonNextDeadline{Type: "course_end", Date: courseEnd.String, Label: "Fine corso prevista"}
	case certExpiry.Valid:
		return &PersonNextDeadline{Type: "cert", Date: certExpiry.String, Label: "Cert in scadenza"}
	case courseEnd.Valid:
		return &PersonNextDeadline{Type: "course_end", Date: courseEnd.String, Label: "Fine corso prevista"}
	default:
		return nil
	}
}

func computePersonPriorityScore(summary PersonSummary) float64 {
	score := 0.0
	score += float64(summary.GapsOpen) * 100
	score += float64(summary.ExpiringCertsCount) * 40
	if summary.ComplianceStatus == "nuovo_assunto" {
		score += 30
	}
	if summary.ActiveEnrollmentsCount == 0 {
		score += 10
	}
	return score
}

func filterMatches(summary PersonSummary, filters PeopleDirectoryFilters) bool {
	switch filters.Filter {
	case "":
		return true
	case "a_norma":
		return summary.ComplianceStatus == "a_norma"
	case "con_gap":
		return summary.ComplianceStatus == "con_gap"
	case "senza_piano":
		return summary.ComplianceStatus == "senza_piano"
	case "nuovo_assunto":
		return summary.ComplianceStatus == "nuovo_assunto"
	default:
		return true
	}
}
