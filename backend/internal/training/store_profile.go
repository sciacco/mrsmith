package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *SQLStore) GetPersonProfile(ctx context.Context, employeeID string, currentYear int) (PersonProfile, error) {
	identity, err := s.personIdentity(ctx, employeeID)
	if err != nil {
		return PersonProfile{}, err
	}
	compliance, err := s.personCompliance(ctx, employeeID)
	if err != nil {
		return PersonProfile{}, err
	}
	enrollments, err := s.personEnrollmentsForYear(ctx, employeeID, currentYear)
	if err != nil {
		return PersonProfile{}, err
	}
	certs, err := s.personCertifications(ctx, employeeID)
	if err != nil {
		return PersonProfile{}, err
	}
	history, err := s.ListHistoryByYear(ctx, employeeID)
	if err != nil {
		return PersonProfile{}, err
	}
	skillAreas, err := s.DeriveSkillAreas(ctx, employeeID)
	if err != nil {
		return PersonProfile{}, err
	}
	suggestions, err := s.MatchGapsToCatalog(ctx, employeeID)
	if err != nil {
		return PersonProfile{}, err
	}
	return PersonProfile{
		IdentityMin:            identity,
		Compliance:             compliance,
		EnrollmentsCurrentYear: enrollments,
		Certifications:         certs,
		HistoryByYear:          history,
		SkillAreas:             skillAreas,
		Suggestions:            suggestions,
	}, nil
}

func (s *SQLStore) personIdentity(ctx context.Context, employeeID string) (PersonIdentityMin, error) {
	const q = `
SELECT
  e.id::text,
  e.last_name || ' ' || e.first_name,
  e.first_name,
  e.last_name,
  e.email::text,
  e.status::text,
  COALESCE(t.id::text, ''),
  COALESCE(t.name, ''),
  COALESCE(t.code, ''),
  COALESCE(e.notes, '')
FROM training.employee e
LEFT JOIN training.team_membership tm
  ON tm.employee_id = e.id
  AND tm.start_date <= now()
  AND (tm.end_date IS NULL OR tm.end_date >= now())
LEFT JOIN training.team t ON t.id = tm.team_id
WHERE e.id = $1::uuid
ORDER BY tm.start_date DESC NULLS LAST, tm.created_at DESC NULLS LAST
LIMIT 1`
	var identity PersonIdentityMin
	err := s.db.QueryRowContext(ctx, q, employeeID).Scan(
		&identity.ID,
		&identity.Name,
		&identity.FirstName,
		&identity.LastName,
		&identity.Email,
		&identity.Status,
		&identity.TeamID,
		&identity.TeamName,
		&identity.TeamCode,
		&identity.Notes,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return identity, notFoundError("employee_not_found", "persona non trovata")
	}
	if err != nil {
		return identity, fmt.Errorf("load training person identity: %w", err)
	}
	return identity, nil
}

func (s *SQLStore) personCompliance(ctx context.Context, employeeID string) (PersonComplianceSection, error) {
	const q = `
SELECT
  course_id::text,
  course_title,
  COALESCE(compliance_framework, ''),
  compliance_status,
  COALESCE(last_valid_awarded_on::text, last_valid_completed_on::text, '')
FROM training.v_mandatory_compliance_gap
WHERE employee_id = $1::uuid
ORDER BY compliance_status DESC, course_title`
	rows, err := s.db.QueryContext(ctx, q, employeeID)
	if err != nil {
		return PersonComplianceSection{}, fmt.Errorf("list person compliance: %w", err)
	}
	defer rows.Close()

	section := PersonComplianceSection{
		MandatoryRules: []PersonComplianceMandatoryRule{},
		OpenGaps:       []PersonComplianceMandatoryRule{},
		ExpiringCerts:  []ExpiringCertificationRow{},
	}
	for rows.Next() {
		var rule PersonComplianceMandatoryRule
		if err := rows.Scan(&rule.CourseID, &rule.CourseTitle, &rule.ComplianceFramework, &rule.Status, &rule.LastValidAwardedOn); err != nil {
			return section, fmt.Errorf("scan person compliance: %w", err)
		}
		section.MandatoryRules = append(section.MandatoryRules, rule)
		if rule.Status == "missing_or_expired" {
			section.OpenGaps = append(section.OpenGaps, rule)
		}
	}
	if err := rows.Err(); err != nil {
		return section, err
	}
	if total := len(section.MandatoryRules); total > 0 {
		compliant := total - len(section.OpenGaps)
		section.CoveragePct = float64(compliant) / float64(total) * 100
	}

	expiring, err := s.personExpiringCertifications(ctx, employeeID)
	if err != nil {
		return section, err
	}
	section.ExpiringCerts = expiring
	return section, nil
}

func (s *SQLStore) personExpiringCertifications(ctx context.Context, employeeID string) ([]ExpiringCertificationRow, error) {
	const q = `
SELECT last_name || ' ' || first_name, email::text, cert_code, cert_name, expires_on::text, days_to_expiry
FROM training.v_expiring_certifications
WHERE employee_id = $1::uuid
ORDER BY days_to_expiry`
	rows, err := s.db.QueryContext(ctx, q, employeeID)
	if err != nil {
		return nil, fmt.Errorf("list person expiring certs: %w", err)
	}
	defer rows.Close()
	result := make([]ExpiringCertificationRow, 0)
	for rows.Next() {
		var row ExpiringCertificationRow
		if err := rows.Scan(&row.EmployeeName, &row.EmployeeEmail, &row.CertificationCode, &row.CertificationName, &row.ExpiresOn, &row.DaysToExpiry); err != nil {
			return nil, fmt.Errorf("scan person expiring cert: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) personEnrollmentsForYear(ctx context.Context, employeeID string, year int) ([]PlanEnrollment, error) {
	const q = `
SELECT
  en.id::text,
  e.last_name || ' ' || e.first_name AS employee_name,
  e.email::text,
  COALESCE(t.code, ''),
  COALESCE(en.course_title_snapshot, c.title),
  COALESCE(en.vendor_name_snapshot, COALESCE(v.name, '')),
  COALESCE(sa.name, ''),
  en.status::text,
  tp.year,
  en.priority,
  en.level_as_is,
  en.level_to_be,
  COALESCE(en.planned_start::text, ''),
  COALESCE(en.planned_end::text, ''),
  en.hours_planned,
  en.cost_planned::float8,
  COALESCE(en.motivation, ''),
  COALESCE(en.objective, ''),
  COALESCE(en.notes, ''),
  c.is_compliance_course,
  COALESCE(c.compliance_framework, ''),
  COALESCE(applicable_rule.id, '') <> '',
  COALESCE(applicable_rule.id, ''),
  COALESCE(applicable_rule.name, '')
FROM training.enrollment en
JOIN training.employee e ON e.id = en.employee_id
JOIN training.training_plan tp ON tp.id = en.training_plan_id
JOIN training.course c ON c.id = en.course_id
LEFT JOIN training.vendor v ON v.id = c.vendor_id
LEFT JOIN training.skill_area sa ON sa.id = c.skill_area_id
LEFT JOIN training.team_membership tm
  ON tm.employee_id = en.employee_id
  AND tm.start_date <= now()
  AND (tm.end_date IS NULL OR tm.end_date >= now())
LEFT JOIN training.team t ON t.id = tm.team_id
LEFT JOIN LATERAL (
  SELECT rule.id::text, rule.name
  FROM training.mandatory_rules rule
  JOIN training.v_mandatory_rule_population population
    ON population.rule_id = rule.id
   AND population.employee_id = en.employee_id
  WHERE rule.is_active
    AND rule.course_id = en.course_id
  ORDER BY CASE WHEN rule.id = en.mandatory_rule_id THEN 0 ELSE 1 END, rule.name
  LIMIT 1
) applicable_rule ON true
WHERE en.employee_id = $1::uuid
  AND tp.year = $2
ORDER BY en.created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, employeeID, year)
	if err != nil {
		return nil, fmt.Errorf("list person enrollments: %w", err)
	}
	defer rows.Close()
	result := make([]PlanEnrollment, 0)
	for rows.Next() {
		var (
			row          PlanEnrollment
			priority     sql.NullInt64
			levelAsIs    sql.NullInt64
			levelToBe    sql.NullInt64
			hoursPlanned sql.NullInt64
			costPlanned  sql.NullFloat64
		)
		if err := rows.Scan(
			&row.ID,
			&row.EmployeeName,
			&row.EmployeeEmail,
			&row.TeamCode,
			&row.CourseTitle,
			&row.VendorName,
			&row.SkillAreaName,
			&row.Status,
			&row.Year,
			&priority,
			&levelAsIs,
			&levelToBe,
			&row.PlannedStart,
			&row.PlannedEnd,
			&hoursPlanned,
			&costPlanned,
			&row.Motivation,
			&row.Objective,
			&row.Notes,
			&row.ComplianceRelated,
			&row.ComplianceFramework,
			&row.RequiredByRule,
			&row.MandatoryRuleID,
			&row.MandatoryRuleName,
		); err != nil {
			return nil, fmt.Errorf("scan person enrollment: %w", err)
		}
		row.Priority = nullInt(priority)
		row.LevelAsIs = nullInt(levelAsIs)
		row.LevelToBe = nullInt(levelToBe)
		row.HoursPlanned = nullInt(hoursPlanned)
		row.CostPlanned = nullFloat(costPlanned)
		row.DocumentValidated = false
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) personCertifications(ctx context.Context, employeeID string) ([]CertificationRow, error) {
	const q = `
SELECT
  vc.cert_code,
  vc.cert_name,
  vc.outcome,
  vc.awarded_on::text,
  COALESCE(vc.expires_on::text, ''),
  vc.current_status,
  vc.validation_source,
  vc.last_name || ' ' || vc.first_name,
  e.email::text,
  ca.id::text AS award_id
FROM training.v_employee_certifications vc
JOIN training.certification_award ca ON ca.employee_id = vc.employee_id AND ca.awarded_on = vc.awarded_on
JOIN training.employee e ON e.id = vc.employee_id
WHERE vc.employee_id = $1::uuid
ORDER BY vc.awarded_on DESC`
	rows, err := s.db.QueryContext(ctx, q, employeeID)
	if err != nil {
		return nil, fmt.Errorf("list person certifications: %w", err)
	}
	defer rows.Close()
	result := make([]CertificationRow, 0)
	for rows.Next() {
		var row CertificationRow
		if err := rows.Scan(
			&row.CertificationCode,
			&row.CertificationName,
			&row.Outcome,
			&row.AwardedOn,
			&row.ExpiresOn,
			&row.CurrentStatus,
			&row.ValidationSource,
			&row.EmployeeName,
			&row.EmployeeEmail,
			&row.AwardID,
		); err != nil {
			return nil, fmt.Errorf("scan person certification: %w", err)
		}
		row.DocumentValidated = false
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) ListHistoryByYear(ctx context.Context, employeeID string) ([]PersonHistoryYearRow, error) {
	const q = `
SELECT
  tp.year,
  COUNT(*) FILTER (WHERE en.status = 'completed') AS completed_count,
  COUNT(*) FILTER (WHERE en.status = 'failed') AS failed_count,
  COALESCE(SUM(en.hours_actual), 0)::float8 AS hours_total,
  COALESCE(SUM(en.cost_actual), 0)::float8 AS cost_total
FROM training.enrollment en
JOIN training.training_plan tp ON tp.id = en.training_plan_id
WHERE en.employee_id = $1::uuid
GROUP BY tp.year
ORDER BY tp.year DESC`
	rows, err := s.db.QueryContext(ctx, q, employeeID)
	if err != nil {
		return nil, fmt.Errorf("list person history by year: %w", err)
	}
	defer rows.Close()
	result := make([]PersonHistoryYearRow, 0)
	for rows.Next() {
		var row PersonHistoryYearRow
		if err := rows.Scan(&row.Year, &row.CompletedCount, &row.FailedCount, &row.HoursTotal, &row.CostTotal); err != nil {
			return nil, fmt.Errorf("scan person history row: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) DeriveSkillAreas(ctx context.Context, employeeID string) ([]PersonSkillArea, error) {
	areasByID := map[string]*PersonSkillArea{}

	const coursesQ = `
SELECT
  sa.id::text,
  sa.name,
  c.title
FROM training.enrollment en
JOIN training.course c ON c.id = en.course_id
JOIN training.skill_area sa ON sa.id = c.skill_area_id
WHERE en.employee_id = $1::uuid
  AND en.status = 'completed'`
	rows, err := s.db.QueryContext(ctx, coursesQ, employeeID)
	if err != nil {
		return nil, fmt.Errorf("derive person skill areas (courses): %w", err)
	}
	for rows.Next() {
		var id, name, title string
		if err := rows.Scan(&id, &name, &title); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan person skill course: %w", err)
		}
		area, ok := areasByID[id]
		if !ok {
			area = &PersonSkillArea{SkillAreaID: id, Name: name, Evidence: PersonSkillEvidence{CoursesCompleted: []string{}, Certs: []string{}}}
			areasByID[id] = area
		}
		area.Evidence.CoursesCompleted = append(area.Evidence.CoursesCompleted, title)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	const certsQ = `
SELECT
  sa.id::text,
  sa.name,
  cert.name
FROM training.certification_award ca
JOIN training.certification cert ON cert.id = ca.certification_id
JOIN training.skill_area sa ON sa.id = cert.skill_area_id
WHERE ca.employee_id = $1::uuid
  AND ca.outcome = 'passed_exam'
  AND (ca.expires_on IS NULL OR ca.expires_on > CURRENT_DATE)`
	certRows, err := s.db.QueryContext(ctx, certsQ, employeeID)
	if err != nil {
		return nil, fmt.Errorf("derive person skill areas (certs): %w", err)
	}
	defer certRows.Close()
	for certRows.Next() {
		var id, name, certName string
		if err := certRows.Scan(&id, &name, &certName); err != nil {
			return nil, fmt.Errorf("scan person skill cert: %w", err)
		}
		area, ok := areasByID[id]
		if !ok {
			area = &PersonSkillArea{SkillAreaID: id, Name: name, Evidence: PersonSkillEvidence{CoursesCompleted: []string{}, Certs: []string{}}}
			areasByID[id] = area
		}
		area.Evidence.Certs = append(area.Evidence.Certs, certName)
	}
	if err := certRows.Err(); err != nil {
		return nil, err
	}

	result := make([]PersonSkillArea, 0, len(areasByID))
	for _, area := range areasByID {
		area.DerivedLevel = deriveSkillLevel(len(area.Evidence.CoursesCompleted), len(area.Evidence.Certs))
		result = append(result, *area)
	}
	return result, nil
}

func deriveSkillLevel(courses, certs int) string {
	if certs >= 2 || courses >= 4 {
		return "avanzato"
	}
	if certs >= 1 || courses >= 2 {
		return "intermedio"
	}
	if courses >= 1 {
		return "base"
	}
	return "none"
}

func (s *SQLStore) MatchGapsToCatalog(ctx context.Context, employeeID string) ([]PersonSuggestion, error) {
	const q = `
SELECT
  g.course_title AS gap_title,
  g.compliance_framework,
  c.id::text,
  c.title,
  COALESCE(v.name, ''),
  COALESCE(sa.name, ''),
  c.delivery_mode::text,
  c.provider_kind::text,
  c.default_hours,
  c.default_cost::float8,
  COALESCE(c.course_url, ''),
  COALESCE(c.description, ''),
  c.is_compliance_course,
  COALESCE(c.recurrence_interval::text, ''),
  COALESCE(c.compliance_framework, ''),
  c.is_active
FROM training.v_mandatory_compliance_gap g
JOIN training.course c
  ON c.id = g.course_id
  AND c.is_active
LEFT JOIN training.vendor v ON v.id = c.vendor_id
LEFT JOIN training.skill_area sa ON sa.id = c.skill_area_id
WHERE g.employee_id = $1::uuid
  AND g.compliance_status = 'missing_or_expired'
ORDER BY g.course_title`
	rows, err := s.db.QueryContext(ctx, q, employeeID)
	if err != nil {
		return nil, fmt.Errorf("match person gaps to catalog: %w", err)
	}
	defer rows.Close()
	suggestions := make([]PersonSuggestion, 0)
	for rows.Next() {
		var (
			gapTitle         string
			framework        string
			course           CatalogCourse
			defaultHours     sql.NullInt64
			defaultCost      sql.NullFloat64
			recurrenceString string
		)
		if err := rows.Scan(
			&gapTitle,
			&framework,
			&course.ID,
			&course.Title,
			&course.VendorName,
			&course.SkillAreaName,
			&course.DeliveryMode,
			&course.ProviderKind,
			&defaultHours,
			&defaultCost,
			&course.CourseURL,
			&course.Description,
			&course.ComplianceRelated,
			&recurrenceString,
			&course.ComplianceFramework,
			&course.Active,
		); err != nil {
			return nil, fmt.Errorf("scan person gap match: %w", err)
		}
		course.DefaultHours = nullInt(defaultHours)
		course.DefaultCost = nullFloat(defaultCost)
		suggestions = append(suggestions, PersonSuggestion{
			Gap: PersonGap{
				Type:        "compliance",
				Description: gapTitle + suffixWithFramework(framework),
			},
			RecommendedCourses: []CatalogCourse{course},
		})
	}
	return suggestions, rows.Err()
}

func suffixWithFramework(framework string) string {
	if framework == "" {
		return ""
	}
	return " · " + framework
}
