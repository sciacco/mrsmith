package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	if db == nil {
		return nil
	}
	return &SQLStore{db: db}
}

func (s *SQLStore) GetEmployeeByEmail(ctx context.Context, email string) (*Employee, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("training database not configured")
	}
	const q = `
SELECT id::text, first_name, last_name, email::text, status::text
FROM training.employee
WHERE email = $1
LIMIT 1`
	var employee Employee
	if err := s.db.QueryRowContext(ctx, q, email).Scan(
		&employee.ID,
		&employee.FirstName,
		&employee.LastName,
		&employee.Email,
		&employee.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load training employee: %w", err)
	}
	return &employee, nil
}

func (s *SQLStore) Workspace(ctx context.Context, principal Principal) (WorkspaceResponse, error) {
	me := MeResponse{Principal: principal}
	employee, err := s.GetEmployeeByEmail(ctx, principal.Email)
	if err != nil {
		return WorkspaceResponse{}, err
	}
	me.Employee = employee
	me.OnboardingPending = employee == nil

	if employee == nil && !principal.IsPeopleAdmin {
		return WorkspaceResponse{Me: me}, nil
	}

	plan, err := s.ListPlanEnrollments(ctx, principal)
	if err != nil {
		return WorkspaceResponse{}, err
	}
	requests, err := s.ListRequests(ctx, principal)
	if err != nil {
		return WorkspaceResponse{}, err
	}
	catalog, err := s.ListCatalogCourses(ctx)
	if err != nil {
		return WorkspaceResponse{}, err
	}
	certs, err := s.ListCertifications(ctx, principal)
	if err != nil {
		return WorkspaceResponse{}, err
	}
	var planBudget []PlanBudgetRow
	if principal.IsPeopleAdmin {
		planBudget, err = s.ListPlanBudget(ctx)
		if err != nil {
			return WorkspaceResponse{}, err
		}
	}
	expiring, err := s.ListExpiringCertifications(ctx, principal)
	if err != nil {
		return WorkspaceResponse{}, err
	}
	gaps, err := s.ListComplianceGaps(ctx, principal)
	if err != nil {
		return WorkspaceResponse{}, err
	}
	var masterData *CatalogMasterData
	if principal.IsPeopleAdmin {
		loaded, err := s.ListCatalogMasterData(ctx)
		if err != nil {
			return WorkspaceResponse{}, err
		}
		masterData = &loaded
	}

	return WorkspaceResponse{
		Me:                      me,
		Plan:                    plan,
		Requests:                requests,
		Catalog:                 catalog,
		Certifications:          certs,
		PlanBudget:              planBudget,
		ExpiringCertifications:  expiring,
		MandatoryComplianceGaps: gaps,
		MasterData:              masterData,
	}, nil
}

func (s *SQLStore) ListPlanEnrollments(ctx context.Context, principal Principal) ([]PlanEnrollment, error) {
	const q = `
SELECT
  en.id::text,
  concat(e.last_name, ' ', e.first_name),
  e.email::text,
  COALESCE(t.code, ''),
  COALESCE(t.name, ''),
  c.title,
  COALESCE(v.name, ''),
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
  COALESCE(NULLIF(en.motivation, 'Import storico'), ''),
  COALESCE(en.objective, ''),
  COALESCE(en.notes, ''),
  COALESCE(doc.id, ''),
  COALESCE(doc.filename, ''),
  COALESCE(doc.is_validated, false),
  c.is_compliance_course,
  COALESCE(c.compliance_framework, ''),
  COALESCE(applicable_rule.id, '') <> '',
  COALESCE(applicable_rule.id, ''),
  COALESCE(applicable_rule.name, '')
FROM training.enrollment en
JOIN training.employee e ON e.id = en.employee_id
JOIN training.course c ON c.id = en.course_id
JOIN training.training_plan tp ON tp.id = en.training_plan_id
LEFT JOIN training.vendor v ON v.id = c.vendor_id
LEFT JOIN training.skill_area sa ON sa.id = c.skill_area_id
LEFT JOIN training.team_membership tm
  ON tm.employee_id = e.id
 AND tm.start_date <= CURRENT_TIMESTAMP
 AND (tm.end_date IS NULL OR tm.end_date >= CURRENT_TIMESTAMP)
LEFT JOIN training.team t ON t.id = tm.team_id
LEFT JOIN LATERAL (
  SELECT d.id::text, d.filename, d.is_validated
  FROM training.document d
  WHERE d.enrollment_id = en.id
  ORDER BY d.uploaded_at DESC, d.id DESC
  LIMIT 1
) doc ON true
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
WHERE ($1::boolean OR e.email = $2)
ORDER BY tp.year DESC, e.last_name, e.first_name, c.title
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q, principal.IsPeopleAdmin, principal.Email)
	if err != nil {
		return nil, fmt.Errorf("list training enrollments: %w", err)
	}
	defer rows.Close()

	result := make([]PlanEnrollment, 0)
	for rows.Next() {
		var row PlanEnrollment
		var priority sql.NullInt64
		var levelAsIs sql.NullInt64
		var levelToBe sql.NullInt64
		var hours sql.NullInt64
		var cost sql.NullFloat64
		if err := rows.Scan(
			&row.ID,
			&row.EmployeeName,
			&row.EmployeeEmail,
			&row.TeamCode,
			&row.TeamName,
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
			&hours,
			&cost,
			&row.Motivation,
			&row.Objective,
			&row.Notes,
			&row.DocumentID,
			&row.DocumentFilename,
			&row.DocumentValidated,
			&row.ComplianceRelated,
			&row.ComplianceFramework,
			&row.RequiredByRule,
			&row.MandatoryRuleID,
			&row.MandatoryRuleName,
		); err != nil {
			return nil, fmt.Errorf("scan training enrollment: %w", err)
		}
		row.Priority = nullInt(priority)
		row.LevelAsIs = nullInt(levelAsIs)
		row.LevelToBe = nullInt(levelToBe)
		row.HoursPlanned = nullInt(hours)
		row.CostPlanned = nullFloat(cost)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) ListRequests(ctx context.Context, principal Principal) ([]TrainingRequest, error) {
	const q = `
SELECT
  tr.id::text,
  concat(e.last_name, ' ', e.first_name),
  e.email::text,
  COALESCE(tr.course_id::text, ''),
  COALESCE(c.title, ''),
  COALESCE(tr.free_text_title, ''),
  COALESCE(sa.name, ''),
  tr.motivation,
  tr.desired_year,
  tr.status,
  tr.created_at::text
FROM training.training_request tr
JOIN training.employee e ON e.id = tr.employee_id
LEFT JOIN training.course c ON c.id = tr.course_id
LEFT JOIN training.skill_area sa ON sa.id = tr.skill_area_id
WHERE ($1::boolean OR e.email = $2)
ORDER BY tr.created_at DESC
LIMIT 300`
	rows, err := s.db.QueryContext(ctx, q, principal.IsPeopleAdmin, principal.Email)
	if err != nil {
		return nil, fmt.Errorf("list training requests: %w", err)
	}
	defer rows.Close()

	result := make([]TrainingRequest, 0)
	for rows.Next() {
		var row TrainingRequest
		var year sql.NullInt64
		if err := rows.Scan(
			&row.ID,
			&row.EmployeeName,
			&row.EmployeeEmail,
			&row.CourseID,
			&row.CourseTitle,
			&row.FreeTextTitle,
			&row.SkillAreaName,
			&row.Motivation,
			&year,
			&row.Status,
			&row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan training request: %w", err)
		}
		row.DesiredYear = nullInt(year)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) ListCatalogCourses(ctx context.Context) ([]CatalogCourse, error) {
	const q = `
SELECT
  c.id::text,
  c.title,
  COALESCE(c.vendor_id::text, ''),
  COALESCE(v.name, ''),
  COALESCE(c.skill_area_id::text, ''),
  COALESCE(sa.name, ''),
  COALESCE(c.leads_to_cert_id::text, ''),
  COALESCE(cert.name, ''),
  c.delivery_mode::text,
  c.provider_kind::text,
  c.default_hours,
  c.default_cost::float8,
  COALESCE(c.course_url, ''),
  COALESCE(c.description, ''),
  c.is_compliance_course,
  CASE
    WHEN c.recurrence_interval IS NULL THEN NULL
    ELSE EXTRACT(YEAR FROM c.recurrence_interval)::int * 12 + EXTRACT(MONTH FROM c.recurrence_interval)::int
  END,
  COALESCE(c.compliance_framework, ''),
  c.is_active
FROM training.course c
LEFT JOIN training.vendor v ON v.id = c.vendor_id
LEFT JOIN training.skill_area sa ON sa.id = c.skill_area_id
LEFT JOIN training.certification cert ON cert.id = c.leads_to_cert_id
ORDER BY c.is_active DESC, c.title
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training catalog courses: %w", err)
	}
	defer rows.Close()

	result := make([]CatalogCourse, 0)
	for rows.Next() {
		var row CatalogCourse
		var hours sql.NullInt64
		var cost sql.NullFloat64
		var recurrenceMonths sql.NullInt64
		if err := rows.Scan(
			&row.ID,
			&row.Title,
			&row.VendorID,
			&row.VendorName,
			&row.SkillAreaID,
			&row.SkillAreaName,
			&row.LeadsToCertID,
			&row.CertificationName,
			&row.DeliveryMode,
			&row.ProviderKind,
			&hours,
			&cost,
			&row.CourseURL,
			&row.Description,
			&row.ComplianceRelated,
			&recurrenceMonths,
			&row.ComplianceFramework,
			&row.Active,
		); err != nil {
			return nil, fmt.Errorf("scan training catalog course: %w", err)
		}
		row.DefaultHours = nullInt(hours)
		row.DefaultCost = nullFloat(cost)
		row.RecurrenceMonths = nullInt(recurrenceMonths)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) ListCatalogMasterData(ctx context.Context) (CatalogMasterData, error) {
	vendors, err := s.listVendors(ctx)
	if err != nil {
		return CatalogMasterData{}, err
	}
	teams, err := s.listTeams(ctx)
	if err != nil {
		return CatalogMasterData{}, err
	}
	skillAreas, err := s.listSkillAreas(ctx)
	if err != nil {
		return CatalogMasterData{}, err
	}
	certifications, err := s.listCatalogCertifications(ctx)
	if err != nil {
		return CatalogMasterData{}, err
	}
	plans, err := s.listTrainingPlans(ctx)
	if err != nil {
		return CatalogMasterData{}, err
	}
	mandatoryRules, err := s.listMandatoryRules(ctx)
	if err != nil {
		return CatalogMasterData{}, err
	}
	return CatalogMasterData{
		Vendors:        vendors,
		Teams:          teams,
		SkillAreas:     skillAreas,
		Certifications: certifications,
		Plans:          plans,
		MandatoryRules: mandatoryRules,
	}, nil
}

func (s *SQLStore) listVendors(ctx context.Context) ([]VendorRow, error) {
	const q = `
SELECT id::text, name, COALESCE(website, ''), COALESCE(notes, ''), is_active
FROM training.vendor
ORDER BY is_active DESC, name
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training vendors: %w", err)
	}
	defer rows.Close()

	result := make([]VendorRow, 0)
	for rows.Next() {
		var row VendorRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Website, &row.Notes, &row.Active); err != nil {
			return nil, fmt.Errorf("scan training vendor: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) listTeams(ctx context.Context) ([]TeamRow, error) {
	const q = `
SELECT id::text, code, name, COALESCE(description, ''), is_active
FROM training.team
ORDER BY is_active DESC, code
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training teams: %w", err)
	}
	defer rows.Close()

	result := make([]TeamRow, 0)
	for rows.Next() {
		var row TeamRow
		if err := rows.Scan(&row.ID, &row.Code, &row.Name, &row.Description, &row.Active); err != nil {
			return nil, fmt.Errorf("scan training team: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) listSkillAreas(ctx context.Context) ([]SkillAreaRow, error) {
	const q = `
SELECT
  sa.id::text,
  sa.code,
  sa.name,
  COALESCE(sa.parent_id::text, ''),
  COALESCE(parent.name, ''),
  COALESCE(sa.description, ''),
  sa.is_active
FROM training.skill_area sa
LEFT JOIN training.skill_area parent ON parent.id = sa.parent_id
ORDER BY sa.is_active DESC, sa.code
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training skill areas: %w", err)
	}
	defer rows.Close()

	result := make([]SkillAreaRow, 0)
	for rows.Next() {
		var row SkillAreaRow
		if err := rows.Scan(&row.ID, &row.Code, &row.Name, &row.ParentID, &row.ParentLabel, &row.Description, &row.Active); err != nil {
			return nil, fmt.Errorf("scan training skill area: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) listCatalogCertifications(ctx context.Context) ([]CatalogCertRow, error) {
	const q = `
SELECT
  cert.id::text,
  cert.code,
  cert.name,
  COALESCE(cert.issuer_vendor_id::text, ''),
  COALESCE(v.name, ''),
  COALESCE(cert.skill_area_id::text, ''),
  COALESCE(sa.name, ''),
  CASE
    WHEN cert.typical_validity IS NULL THEN NULL
    ELSE EXTRACT(YEAR FROM cert.typical_validity)::int * 12 + EXTRACT(MONTH FROM cert.typical_validity)::int
  END,
  COALESCE(cert.description, ''),
  cert.is_active
FROM training.certification cert
LEFT JOIN training.vendor v ON v.id = cert.issuer_vendor_id
LEFT JOIN training.skill_area sa ON sa.id = cert.skill_area_id
ORDER BY cert.is_active DESC, cert.code
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training catalog certifications: %w", err)
	}
	defer rows.Close()

	result := make([]CatalogCertRow, 0)
	for rows.Next() {
		var row CatalogCertRow
		var months sql.NullInt64
		if err := rows.Scan(
			&row.ID,
			&row.Code,
			&row.Name,
			&row.IssuerVendorID,
			&row.IssuerVendorName,
			&row.SkillAreaID,
			&row.SkillAreaLabel,
			&months,
			&row.Description,
			&row.Active,
		); err != nil {
			return nil, fmt.Errorf("scan training catalog certification: %w", err)
		}
		row.TypicalValidityMonths = nullInt(months)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) listTrainingPlans(ctx context.Context) ([]TrainingPlanRow, error) {
	const q = `
SELECT id::text, year, status::text, budget_total::float8, COALESCE(notes, '')
FROM training.training_plan
ORDER BY year DESC
LIMIT 200`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training plans: %w", err)
	}
	defer rows.Close()

	result := make([]TrainingPlanRow, 0)
	for rows.Next() {
		var row TrainingPlanRow
		var budget sql.NullFloat64
		if err := rows.Scan(&row.ID, &row.Year, &row.Status, &budget, &row.Notes); err != nil {
			return nil, fmt.Errorf("scan training plan: %w", err)
		}
		row.BudgetTotal = nullFloat(budget)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) listMandatoryRules(ctx context.Context) ([]MandatoryRuleRow, error) {
	const q = `
SELECT
  rule.id::text,
  rule.course_id::text,
  course.title,
  CASE WHEN rule.population_target->>'kind' = 'team' THEN COALESCE(rule.population_target->>'id', '') ELSE '' END,
  COALESCE(team.name, ''),
  '',
  COALESCE(rule.notes, ''),
  rule.is_active
FROM training.mandatory_rules rule
JOIN training.course course ON course.id = rule.course_id
LEFT JOIN training.team team
  ON rule.population_target->>'kind' = 'team'
 AND team.id = (rule.population_target->>'id')::uuid
ORDER BY rule.is_active DESC, course.title, team.code NULLS FIRST
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training mandatory rules: %w", err)
	}
	defer rows.Close()

	result := make([]MandatoryRuleRow, 0)
	for rows.Next() {
		var row MandatoryRuleRow
		if err := rows.Scan(
			&row.ID,
			&row.CourseID,
			&row.CourseTitle,
			&row.TeamID,
			&row.TeamLabel,
			&row.RoleFilter,
			&row.Notes,
			&row.Active,
		); err != nil {
			return nil, fmt.Errorf("scan training mandatory rule: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) ListCertifications(ctx context.Context, principal Principal) ([]CertificationRow, error) {
	const q = `
SELECT
  award_id::text,
  concat(vc.last_name, ' ', vc.first_name),
  e.email::text,
  cert_code,
  cert_name,
  outcome::text,
  awarded_on::text,
  COALESCE(expires_on::text, ''),
  current_status,
  validation_source::text,
  COALESCE(doc.id, ''),
  COALESCE(doc.filename, ''),
  COALESCE(doc.is_validated, false)
FROM training.v_employee_certifications vc
JOIN training.employee e ON e.id = vc.employee_id
LEFT JOIN LATERAL (
  SELECT d.id::text, d.filename, d.is_validated
  FROM training.document d
  WHERE d.certification_award_id = vc.award_id
  ORDER BY d.uploaded_at DESC
  LIMIT 1
) doc ON true
WHERE ($1::boolean OR vc.employee_id = (
  SELECT id FROM training.employee WHERE email = $2 LIMIT 1
))
ORDER BY vc.last_name, vc.first_name, cert_name, awarded_on DESC
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q, principal.IsPeopleAdmin, principal.Email)
	if err != nil {
		return nil, fmt.Errorf("list training certifications: %w", err)
	}
	defer rows.Close()

	result := make([]CertificationRow, 0)
	for rows.Next() {
		var row CertificationRow
		if err := rows.Scan(
			&row.AwardID,
			&row.EmployeeName,
			&row.EmployeeEmail,
			&row.CertificationCode,
			&row.CertificationName,
			&row.Outcome,
			&row.AwardedOn,
			&row.ExpiresOn,
			&row.CurrentStatus,
			&row.ValidationSource,
			&row.DocumentID,
			&row.DocumentFilename,
			&row.DocumentValidated,
		); err != nil {
			return nil, fmt.Errorf("scan training certification: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) ListPlanBudget(ctx context.Context) ([]PlanBudgetRow, error) {
	const q = `
SELECT year, COALESCE(team_code, ''), enrollments_count, cost_total::float8, hours_total::float8
FROM training.v_plan_budget
ORDER BY year DESC, team_code
LIMIT 200`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training plan budget: %w", err)
	}
	defer rows.Close()

	result := make([]PlanBudgetRow, 0)
	for rows.Next() {
		var row PlanBudgetRow
		var cost sql.NullFloat64
		var hours sql.NullFloat64
		if err := rows.Scan(&row.Year, &row.TeamCode, &row.EnrollmentsCount, &cost, &hours); err != nil {
			return nil, fmt.Errorf("scan training plan budget: %w", err)
		}
		row.CostTotal = nullFloat(cost)
		row.HoursTotal = nullFloat(hours)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) ListExpiringCertifications(ctx context.Context, principal Principal) ([]ExpiringCertificationRow, error) {
	const q = `
SELECT
  concat(last_name, ' ', first_name),
  email::text,
  cert_code,
  cert_name,
  expires_on::text,
  days_to_expiry::int
FROM training.v_expiring_certifications
WHERE ($1::boolean OR employee_id = (
  SELECT id FROM training.employee WHERE email = $2 LIMIT 1
))
ORDER BY days_to_expiry ASC, last_name, first_name
LIMIT 300`
	rows, err := s.db.QueryContext(ctx, q, principal.IsPeopleAdmin, principal.Email)
	if err != nil {
		return nil, fmt.Errorf("list expiring training certifications: %w", err)
	}
	defer rows.Close()

	result := make([]ExpiringCertificationRow, 0)
	for rows.Next() {
		var row ExpiringCertificationRow
		if err := rows.Scan(
			&row.EmployeeName,
			&row.EmployeeEmail,
			&row.CertificationCode,
			&row.CertificationName,
			&row.ExpiresOn,
			&row.DaysToExpiry,
		); err != nil {
			return nil, fmt.Errorf("scan expiring training certification: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) ListComplianceGaps(ctx context.Context, principal Principal) ([]ComplianceGapRow, error) {
	const q = `
SELECT
  concat(last_name, ' ', first_name),
  course_title,
  COALESCE(compliance_framework, ''),
  COALESCE(last_valid_awarded_on::text, last_valid_completed_on::text, ''),
  compliance_status
FROM training.v_mandatory_compliance_gap
WHERE $1::boolean OR employee_id = (
  SELECT id FROM training.employee WHERE email = $2 LIMIT 1
)
ORDER BY compliance_status DESC, last_name, first_name, course_title
LIMIT 300`
	rows, err := s.db.QueryContext(ctx, q, principal.IsPeopleAdmin, principal.Email)
	if err != nil {
		return nil, fmt.Errorf("list training compliance gaps: %w", err)
	}
	defer rows.Close()

	result := make([]ComplianceGapRow, 0)
	for rows.Next() {
		var row ComplianceGapRow
		if err := rows.Scan(
			&row.EmployeeName,
			&row.CourseTitle,
			&row.ComplianceFramework,
			&row.LastValidAwardedOn,
			&row.ComplianceStatus,
		); err != nil {
			return nil, fmt.Errorf("scan training compliance gap: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) Lookups(ctx context.Context, principal Principal) (LookupResponse, error) {
	var employees []LookupItem
	if principal.IsPeopleAdmin {
		var err error
		employees, err = s.employeeLookup(ctx)
		if err != nil {
			return LookupResponse{}, err
		}
	}
	teams, err := s.lookup(ctx, "training.team", "name", "")
	if err != nil {
		return LookupResponse{}, err
	}
	vendors, err := s.lookup(ctx, "training.vendor", "name", "")
	if err != nil {
		return LookupResponse{}, err
	}
	skillAreas, err := s.lookup(ctx, "training.skill_area", "name", "")
	if err != nil {
		return LookupResponse{}, err
	}
	courses, err := s.courseLookup(ctx)
	if err != nil {
		return LookupResponse{}, err
	}
	certifications, err := s.lookup(ctx, "training.certification", "name", "code")
	if err != nil {
		return LookupResponse{}, err
	}
	plans, err := s.plansLookup(ctx)
	if err != nil {
		return LookupResponse{}, err
	}
	return LookupResponse{
		Employees:      employees,
		Teams:          teams,
		Vendors:        vendors,
		SkillAreas:     skillAreas,
		Courses:        courses,
		Certifications: certifications,
		Plans:          plans,
	}, nil
}

func (s *SQLStore) employeeLookup(ctx context.Context) ([]LookupItem, error) {
	const q = `
SELECT id::text, last_name || ' ' || first_name || ' - ' || email::text, status = 'active'
FROM training.employee
ORDER BY status = 'active' DESC, last_name, first_name
LIMIT 1000`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("load training employee lookup: %w", err)
	}
	defer rows.Close()
	items := make([]LookupItem, 0)
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Label, &item.Active); err != nil {
			return nil, fmt.Errorf("scan training employee lookup: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLStore) courseLookup(ctx context.Context) ([]LookupItem, error) {
	const q = `
SELECT id::text, title, is_active, is_compliance_course, COALESCE(compliance_framework, '')
FROM training.course
ORDER BY is_active DESC, title
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("load training course lookup: %w", err)
	}
	defer rows.Close()

	items := make([]LookupItem, 0)
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Label, &item.Active, &item.ComplianceRelated, &item.ComplianceFramework); err != nil {
			return nil, fmt.Errorf("scan training course lookup: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLStore) lookup(ctx context.Context, table, labelColumn, codeColumn string) ([]LookupItem, error) {
	labelExpr := labelColumn
	if codeColumn != "" {
		labelExpr = fmt.Sprintf("%s || ' - ' || %s", codeColumn, labelColumn)
	}
	q := fmt.Sprintf("SELECT id::text, %s, is_active FROM %s ORDER BY is_active DESC, %s LIMIT 500", labelExpr, table, labelColumn)
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("load training lookup %s: %w", table, err)
	}
	defer rows.Close()

	items := make([]LookupItem, 0)
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Label, &item.Active); err != nil {
			return nil, fmt.Errorf("scan training lookup %s: %w", table, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLStore) plansLookup(ctx context.Context) ([]LookupItem, error) {
	const q = `SELECT id::text, year::text || ' - ' || status::text, status <> 'closed' FROM training.training_plan ORDER BY year DESC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("load training plans lookup: %w", err)
	}
	defer rows.Close()

	items := make([]LookupItem, 0)
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Label, &item.Active); err != nil {
			return nil, fmt.Errorf("scan training plans lookup: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func nullInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func nullFloat(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}
