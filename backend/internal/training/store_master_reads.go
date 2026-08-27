package training

import (
	"context"
	"database/sql"
	"fmt"
)

// Anagrafiche (#152, slice 1 del task 6): team, fornitori, aree di
// competenza e certificazioni. Includono gli elementi inattivi (flag
// active); le lookups dei form restano invariate (store.go).

// ListTeams aggrega i lead attivi (D5: role='lead', end_date IS NULL) in
// JSON lato query, stesso pattern di store_people_reads.go.
func (s *SQLStore) ListTeams(ctx context.Context) ([]TeamListRow, error) {
	const q = `
SELECT
  t.id::text,
  t.code,
  t.name,
  t.is_active,
  COALESCE(t.external_id, '') <> '',
  (SELECT COUNT(*) FROM training.team_membership tm WHERE tm.team_id = t.id AND tm.end_date IS NULL),
  COALESCE((
    SELECT json_agg(json_build_object('employeeId', lead.employee_id::text, 'name', concat(le.last_name, ' ', le.first_name)) ORDER BY concat(le.last_name, ' ', le.first_name))
    FROM training.team_membership lead
    JOIN training.employee le ON le.id = lead.employee_id
    WHERE lead.team_id = t.id AND lead.role = 'lead' AND lead.end_date IS NULL
  ), '[]')::text
FROM training.team t
ORDER BY t.name
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training teams: %w", err)
	}
	defer rows.Close()

	result := make([]TeamListRow, 0)
	for rows.Next() {
		var row TeamListRow
		var leads string
		if err := rows.Scan(&row.ID, &row.Code, &row.Name, &row.Active, &row.ManagedBySync, &row.ActiveMembers, &leads); err != nil {
			return nil, fmt.Errorf("scan training team: %w", err)
		}
		if row.Leads, err = decodeJSONSlice[TeamLeadRef](leads); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) ListVendors(ctx context.Context) ([]VendorListRow, error) {
	const q = `
SELECT id::text, name, COALESCE(website, ''), COALESCE(notes, ''), is_active
FROM training.vendor
ORDER BY name
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training vendors: %w", err)
	}
	defer rows.Close()

	result := make([]VendorListRow, 0)
	for rows.Next() {
		var row VendorListRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Website, &row.Notes, &row.Active); err != nil {
			return nil, fmt.Errorf("scan training vendor: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) ListSkillAreas(ctx context.Context) ([]SkillAreaListRow, error) {
	const q = `
SELECT
  sa.id::text,
  sa.code,
  sa.name,
  COALESCE(sa.description, ''),
  sa.is_active,
  COALESCE(sa.custom_group_id::text, ''),
  COALESCE(g.name, ''),
  COALESCE(sa.parent_id::text, '')
FROM training.skill_area sa
LEFT JOIN training.custom_groups g ON g.id = sa.custom_group_id
ORDER BY sa.name
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training skill areas: %w", err)
	}
	defer rows.Close()

	result := make([]SkillAreaListRow, 0)
	for rows.Next() {
		var row SkillAreaListRow
		if err := rows.Scan(&row.ID, &row.Code, &row.Name, &row.Description, &row.Active, &row.CustomGroupID, &row.CustomGroupName, &row.ParentID); err != nil {
			return nil, fmt.Errorf("scan training skill area: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// ListCertificationCatalog e l'anagrafica certificazioni (distinta dalla
// vista per persona di store.go ListCertifications): typicalValidityMonths
// e derivato dall'interval typical_validity, sempre scritto in mesi interi
// dall'upsert (monthsInterval).
func (s *SQLStore) ListCertificationCatalog(ctx context.Context) ([]CertificationCatalogRow, error) {
	const q = `
SELECT
  cert.id::text,
  cert.code,
  cert.name,
  COALESCE(cert.description, ''),
  cert.is_active,
  COALESCE(cert.issuer_vendor_id::text, ''),
  COALESCE(v.name, ''),
  COALESCE(cert.skill_area_id::text, ''),
  COALESCE(sa.name, ''),
  (EXTRACT(YEAR FROM cert.typical_validity) * 12 + EXTRACT(MONTH FROM cert.typical_validity))::int
FROM training.certification cert
LEFT JOIN training.vendor v ON v.id = cert.issuer_vendor_id
LEFT JOIN training.skill_area sa ON sa.id = cert.skill_area_id
ORDER BY cert.name
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training certification catalog: %w", err)
	}
	defer rows.Close()

	result := make([]CertificationCatalogRow, 0)
	for rows.Next() {
		var (
			row    CertificationCatalogRow
			months sql.NullInt64
		)
		if err := rows.Scan(
			&row.ID, &row.Code, &row.Name, &row.Description, &row.Active,
			&row.IssuerVendorID, &row.IssuerVendorName, &row.SkillAreaID, &row.SkillAreaName, &months,
		); err != nil {
			return nil, fmt.Errorf("scan training certification catalog: %w", err)
		}
		row.TypicalValidityMonths = nullInt(months)
		result = append(result, row)
	}
	return result, rows.Err()
}
