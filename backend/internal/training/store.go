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
	return LookupResponse{
		Employees:      employees,
		Teams:          teams,
		Vendors:        vendors,
		SkillAreas:     skillAreas,
		Courses:        courses,
		Certifications: certifications,
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
