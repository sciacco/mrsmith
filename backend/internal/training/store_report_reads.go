package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// I due prospetti di report ratificati (#163, slice 4 del task 7). Sola
// lettura: nessuna tabella propria, nessuna scrittura verso Arak.

// EconomicReportLocals carica ogni voce di spesa, eventi annullati compresi
// (flag eventCancelled): il PO vivo si risolve al chiamante (#163 punto 1).
// Nessun parametro: il filtro per anno di competenza e client-side sui dati
// vivi restituiti dopo l'idratazione.
func (s *SQLStore) EconomicReportLocals(ctx context.Context) ([]economicReportLocal, error) {
	const query = `
SELECT
  ee.id::text,
  ee.event_id::text,
  ee.rda_id,
  c.title,
  (ev.cancelled_at IS NOT NULL),
  ee.created_at::text,
  COUNT(eee.enrollment_id)
FROM training.event_expense ee
JOIN training.training_event ev ON ev.id = ee.event_id
JOIN training.course c ON c.id = ev.course_id
LEFT JOIN training.event_expense_enrollment eee ON eee.expense_id = ee.id
GROUP BY ee.id, ev.id, c.id
ORDER BY ee.created_at, ee.id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list training economic report expenses: %w", err)
	}
	defer rows.Close()

	result := make([]economicReportLocal, 0)
	for rows.Next() {
		var row economicReportLocal
		if err := rows.Scan(
			&row.ExpenseID,
			&row.EventID,
			&row.POID,
			&row.CourseTitle,
			&row.EventCancelled,
			&row.CreatedAt,
			&row.CoveredEnrollments,
		); err != nil {
			return nil, fmt.Errorf("scan training economic report expense: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// DeliveredReportRows carica le iscrizioni concluse (completed/
// partially_completed, evento non annullato) con data di riferimento nel
// periodo [from, to] (#163 punto 2). Data di riferimento = COALESCE
// (actual_end, MAX(ends_at) delle sessioni con partecipazione completata,
// actual_start); ore = COALESCE(hours_actual, course.default_hours),
// entrambe nullable quando i fatti a monte non le determinano. Teams riusa
// personTeamsJSON (store_people_reads.go): stesse appartenenze attive
// correnti mostrate nella scheda persona.
func (s *SQLStore) DeliveredReportRows(ctx context.Context, from, to time.Time) ([]DeliveredReportRow, error) {
	query := `
WITH base AS (
  SELECT
    en.id AS enrollment_id,
    e.id AS employee_id,
    concat(e.last_name, ' ', e.first_name) AS employee_name,` +
		personTeamsJSON + ` AS teams,
    c.id AS course_id,
    c.title AS course_title,
    COALESCE((
      SELECT json_agg(sa.name ORDER BY sa.name)
      FROM training.course_skill_area csa
      JOIN training.skill_area sa ON sa.id = csa.skill_area_id
      WHERE csa.course_id = c.id
    ), '[]')::text AS skill_area_names,
    en.event_id AS event_id,
    en.delivery_status AS delivery_status,
    COALESCE(en.learning_outcome, '') AS learning_outcome,
    COALESCE(
      en.actual_end,
      (
        SELECT MAX(s.ends_at)::date
        FROM training.enrollment_session es
        JOIN training.training_session s ON s.id = es.session_id
        WHERE es.enrollment_id = en.id AND es.participation_status = 'completed'
      ),
      en.actual_start
    ) AS reference_date,
    COALESCE(en.hours_actual, c.default_hours) AS hours
  FROM training.enrollment en
  JOIN training.employee e ON e.id = en.employee_id
  JOIN training.training_event ev ON ev.id = en.event_id
  JOIN training.course c ON c.id = ev.course_id
  WHERE en.delivery_status IN ('completed', 'partially_completed')
    AND ev.cancelled_at IS NULL
)
SELECT
  enrollment_id::text, employee_id::text, employee_name, teams,
  course_id::text, course_title, skill_area_names, event_id::text,
  delivery_status, learning_outcome, reference_date::text, hours
FROM base
WHERE reference_date BETWEEN $1::date AND $2::date
ORDER BY reference_date, employee_name, enrollment_id
LIMIT 5000`
	rows, err := s.db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("list training delivered report rows: %w", err)
	}
	defer rows.Close()

	result := make([]DeliveredReportRow, 0)
	for rows.Next() {
		var (
			row       DeliveredReportRow
			teams     string
			areaNames string
			hours     sql.NullInt64
		)
		if err := rows.Scan(
			&row.EnrollmentID,
			&row.EmployeeID,
			&row.EmployeeName,
			&teams,
			&row.CourseID,
			&row.CourseTitle,
			&areaNames,
			&row.EventID,
			&row.DeliveryStatus,
			&row.LearningOutcome,
			&row.ReferenceDate,
			&hours,
		); err != nil {
			return nil, fmt.Errorf("scan training delivered report row: %w", err)
		}
		if row.Teams, err = decodeJSONSlice[PersonTeamRef](teams); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(areaNames), &row.SkillAreaNames); err != nil {
			return nil, fmt.Errorf("decode training delivered report skill areas: %w", err)
		}
		row.Hours = nullInt(hours)
		result = append(result, row)
	}
	return result, rows.Err()
}
