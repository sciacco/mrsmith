package training

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
)

func (s *SQLStore) Overview(ctx context.Context, year int, team string) (OverviewResponse, error) {
	resp := OverviewResponse{Year: year, TeamScope: team}

	esecuzione, err := s.overviewEsecuzione(ctx, year, team)
	if err != nil {
		return resp, err
	}
	resp.Esecuzione = esecuzione

	compliance, err := s.overviewCompliance(ctx, year, team)
	if err != nil {
		return resp, err
	}
	resp.Compliance = compliance

	budget, err := s.overviewBudget(ctx, year, team)
	if err != nil {
		return resp, err
	}
	resp.Budget = budget

	engagement, err := s.overviewEngagement(ctx, year, team)
	if err != nil {
		return resp, err
	}
	resp.Engagement = engagement

	return resp, nil
}

func (s *SQLStore) overviewEsecuzione(ctx context.Context, year int, team string) (OverviewFamily, error) {
	const q = `
SELECT
  COUNT(*) FILTER (WHERE en.status IN ('proposed','approved','in_progress','completed','failed','cancelled','expired')) AS total,
  COUNT(*) FILTER (WHERE en.status = 'completed') AS completed
FROM training.enrollment en
JOIN training.training_plan tp ON tp.id = en.training_plan_id
LEFT JOIN training.team_membership tm
  ON tm.employee_id = en.employee_id
  AND tm.start_date <= now()
  AND (tm.end_date IS NULL OR tm.end_date >= now())
LEFT JOIN training.team t ON t.id = tm.team_id
WHERE tp.year = $1 AND ($2 = '' OR t.code = $2)`
	var total, completed int
	if err := s.db.QueryRowContext(ctx, q, year, team).Scan(&total, &completed); err != nil {
		return OverviewFamily{}, fmt.Errorf("overview esecuzione: %w", err)
	}
	pct := 0
	if total > 0 {
		pct = completed * 100 / total
	}

	exceptions, err := s.overviewOverdueEnrollments(ctx, year, team)
	if err != nil {
		return OverviewFamily{}, err
	}

	return OverviewFamily{
		Value:      fmt.Sprintf("%d%%", pct),
		Exceptions: exceptions,
	}, nil
}

func (s *SQLStore) overviewOverdueEnrollments(ctx context.Context, year int, team string) ([]OverviewException, error) {
	const q = `
SELECT
  en.id::text,
  c.title,
  e.last_name || ' ' || e.first_name,
  (CURRENT_DATE - en.planned_end) AS ritardo_gg
FROM training.enrollment en
JOIN training.training_plan tp ON tp.id = en.training_plan_id
JOIN training.course c ON c.id = en.course_id
JOIN training.employee e ON e.id = en.employee_id
LEFT JOIN training.team_membership tm
  ON tm.employee_id = en.employee_id
  AND tm.start_date <= now()
  AND (tm.end_date IS NULL OR tm.end_date >= now())
LEFT JOIN training.team t ON t.id = tm.team_id
WHERE tp.year = $1
  AND ($2 = '' OR t.code = $2)
  AND en.status IN ('approved','in_progress')
  AND en.planned_end IS NOT NULL
  AND en.planned_end < CURRENT_DATE
ORDER BY ritardo_gg DESC
LIMIT 3`
	rows, err := s.db.QueryContext(ctx, q, year, team)
	if err != nil {
		return nil, fmt.Errorf("overview overdue enrollments: %w", err)
	}
	defer rows.Close()
	result := make([]OverviewException, 0)
	for rows.Next() {
		var (
			id          string
			courseTitle string
			personName  string
			ritardo     int
		)
		if err := rows.Scan(&id, &courseTitle, &personName, &ritardo); err != nil {
			return nil, fmt.Errorf("scan overview overdue: %w", err)
		}
		severity := "warning"
		if ritardo > 30 {
			severity = "critical"
		}
		drilldown := buildDrilldownURL("/pipeline", map[string]string{"stato": "in_progress", "ritardo_gg": ">0"})
		result = append(result, OverviewException{
			ID:           id,
			Severity:     severity,
			Title:        fmt.Sprintf("%s · %s · %dgg ritardo", courseTitle, personName, ritardo),
			DrilldownURL: drilldown,
		})
	}
	return result, rows.Err()
}

func (s *SQLStore) overviewCompliance(ctx context.Context, year int, team string) (OverviewFamily, error) {
	const q = `
SELECT
  COUNT(*) AS total,
  COUNT(*) FILTER (WHERE compliance_status = 'compliant') AS compliant
FROM training.v_mandatory_compliance_gap g
LEFT JOIN training.team_membership tm
  ON tm.employee_id = g.employee_id
  AND tm.start_date <= now()
  AND (tm.end_date IS NULL OR tm.end_date >= now())
LEFT JOIN training.team t ON t.id = tm.team_id
WHERE ($1 = '' OR t.code = $1)`
	var total, compliant int
	if err := s.db.QueryRowContext(ctx, q, team).Scan(&total, &compliant); err != nil {
		return OverviewFamily{}, fmt.Errorf("overview compliance: %w", err)
	}
	pct := 0
	if total > 0 {
		pct = compliant * 100 / total
	}

	exceptions, err := s.overviewComplianceExceptions(ctx, team)
	if err != nil {
		return OverviewFamily{}, err
	}

	_ = year
	return OverviewFamily{
		Value:      fmt.Sprintf("%d%%", pct),
		Exceptions: exceptions,
	}, nil
}

func (s *SQLStore) overviewComplianceExceptions(ctx context.Context, team string) ([]OverviewException, error) {
	const q = `
SELECT
  g.employee_id::text,
  g.last_name || ' ' || g.first_name,
  COUNT(*) AS gaps
FROM training.v_mandatory_compliance_gap g
LEFT JOIN training.team_membership tm
  ON tm.employee_id = g.employee_id
  AND tm.start_date <= now()
  AND (tm.end_date IS NULL OR tm.end_date >= now())
LEFT JOIN training.team t ON t.id = tm.team_id
WHERE g.compliance_status = 'missing_or_expired'
  AND ($1 = '' OR t.code = $1)
GROUP BY g.employee_id, g.last_name, g.first_name
ORDER BY gaps DESC
LIMIT 3`
	rows, err := s.db.QueryContext(ctx, q, team)
	if err != nil {
		return nil, fmt.Errorf("overview compliance exceptions: %w", err)
	}
	defer rows.Close()
	result := make([]OverviewException, 0)
	for rows.Next() {
		var (
			id    string
			name  string
			gaps  int
		)
		if err := rows.Scan(&id, &name, &gaps); err != nil {
			return nil, fmt.Errorf("scan overview compliance exc: %w", err)
		}
		drilldown := buildDrilldownURL("/persone/"+id, nil)
		result = append(result, OverviewException{
			ID:           id,
			Severity:     "critical",
			Title:        fmt.Sprintf("%s · %d gap aperti", name, gaps),
			DrilldownURL: drilldown,
		})
	}
	return result, rows.Err()
}

func (s *SQLStore) overviewBudget(ctx context.Context, year int, team string) (OverviewFamily, error) {
	const q = `
WITH plan AS (
  SELECT COALESCE(budget_total, 0)::float8 AS plan_total
  FROM training.training_plan
  WHERE year = $1
  LIMIT 1
),
spent AS (
  SELECT COALESCE(SUM(COALESCE(en.cost_actual, en.cost_planned, c.default_cost, 0)), 0)::float8 AS spent_total
  FROM training.enrollment en
  JOIN training.training_plan tp ON tp.id = en.training_plan_id
  JOIN training.course c ON c.id = en.course_id
  LEFT JOIN training.team_membership tm
    ON tm.employee_id = en.employee_id
    AND tm.start_date <= now()
    AND (tm.end_date IS NULL OR tm.end_date >= now())
  LEFT JOIN training.team t ON t.id = tm.team_id
  WHERE tp.year = $1 AND ($2 = '' OR t.code = $2)
)
SELECT plan.plan_total, spent.spent_total FROM plan, spent`
	var plan, spent float64
	row := s.db.QueryRowContext(ctx, q, year, team)
	if err := row.Scan(&plan, &spent); err != nil && err != sql.ErrNoRows {
		return OverviewFamily{}, fmt.Errorf("overview budget: %w", err)
	}
	spentPct := 0.0
	if plan > 0 {
		spentPct = spent / plan * 100
	}
	value := fmt.Sprintf("€%.0fk / €%.0fk", spent/1000, plan/1000)
	pct := spentPct
	alignment := "in_linea"
	if spentPct > 90 {
		alignment = "in_ritardo"
	} else if spentPct < 30 {
		alignment = "in_anticipo"
	}
	return OverviewFamily{
		Value:             value,
		SpentPct:          &pct,
		CalendarAlignment: alignment,
		Exceptions:        []OverviewException{},
	}, nil
}

func (s *SQLStore) overviewEngagement(ctx context.Context, year int, team string) (OverviewFamily, error) {
	const q = `
WITH active_emp AS (
  SELECT e.id, tm.team_id
  FROM training.employee e
  LEFT JOIN training.team_membership tm
    ON tm.employee_id = e.id
    AND tm.start_date <= now()
    AND (tm.end_date IS NULL OR tm.end_date >= now())
  WHERE e.status = 'active'
),
filtered_emp AS (
  SELECT ae.id
  FROM active_emp ae
  LEFT JOIN training.team t ON t.id = ae.team_id
  WHERE $2 = '' OR t.code = $2
),
involved AS (
  SELECT DISTINCT employee_id FROM training.enrollment en
  JOIN training.training_plan tp ON tp.id = en.training_plan_id
  WHERE tp.year = $1
),
courses_per AS (
  SELECT employee_id, COUNT(*) AS cnt FROM training.enrollment en
  JOIN training.training_plan tp ON tp.id = en.training_plan_id
  WHERE tp.year = $1
  GROUP BY employee_id
)
SELECT
  (SELECT COUNT(*) FROM filtered_emp) AS total_emp,
  (SELECT COUNT(DISTINCT i.employee_id) FROM involved i JOIN filtered_emp f ON f.id = i.employee_id) AS involved_emp,
  (SELECT MIN(cnt) FROM courses_per p JOIN filtered_emp f ON f.id = p.employee_id) AS min_courses,
  (SELECT MAX(cnt) FROM courses_per p JOIN filtered_emp f ON f.id = p.employee_id) AS max_courses`
	var total, involved int
	var minCourses, maxCourses sql.NullInt64
	if err := s.db.QueryRowContext(ctx, q, year, team).Scan(&total, &involved, &minCourses, &maxCourses); err != nil {
		return OverviewFamily{}, fmt.Errorf("overview engagement: %w", err)
	}
	pct := 0
	if total > 0 {
		pct = involved * 100 / total
	}
	family := OverviewFamily{
		Value:      fmt.Sprintf("%d%%", pct),
		Exceptions: []OverviewException{},
	}
	if minCourses.Valid {
		v := int(minCourses.Int64)
		family.MinCoursesPerPerson = &v
	}
	if maxCourses.Valid {
		v := int(maxCourses.Int64)
		family.MaxCoursesPerPerson = &v
	}
	return family, nil
}

func buildDrilldownURL(path string, params map[string]string) string {
	if len(params) == 0 {
		return path
	}
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	return path + "?" + values.Encode()
}
