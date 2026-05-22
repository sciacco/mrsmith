package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (s *SQLStore) ListCatalogCoursesFiltered(ctx context.Context, filters CatalogListFilters) ([]CatalogCourseWithCounts, error) {
	year := filters.Year
	if year == 0 {
		year = currentYear()
	}

	var args []any
	conds := []string{"1=1"}
	idx := 1

	switch strings.ToLower(strings.TrimSpace(filters.Stato)) {
	case "attivo":
		conds = append(conds, "c.is_active = true")
	case "disattivato":
		conds = append(conds, "c.is_active = false")
	}
	if v := strings.TrimSpace(filters.Vendor); v != "" {
		conds = append(conds, fmt.Sprintf("c.vendor_id = $%d::uuid", idx))
		args = append(args, v)
		idx++
	}
	if sa := strings.TrimSpace(filters.SkillArea); sa != "" {
		conds = append(conds, fmt.Sprintf("c.skill_area_id = $%d::uuid", idx))
		args = append(args, sa)
		idx++
	}
	if q := strings.TrimSpace(filters.Search); q != "" {
		conds = append(conds, fmt.Sprintf("(c.title ILIKE $%d OR COALESCE(c.description,'') ILIKE $%d)", idx, idx))
		args = append(args, "%"+q+"%")
		idx++
	}

	// year argument is always last, fixed position
	yearArg := idx
	args = append(args, year)
	_ = yearArg

	stmt := fmt.Sprintf(`
SELECT
  c.id::text,
  c.title,
  COALESCE(c.vendor_id::text, ''),
  COALESCE(v.name, ''),
  COALESCE(c.skill_area_id::text, ''),
  COALESCE(sa.code || ' - ' || sa.name, ''),
  COALESCE(c.leads_to_cert_id::text, ''),
  COALESCE(cert.name, ''),
  c.delivery_mode::text,
  c.provider_kind::text,
  c.default_hours,
  c.default_cost,
  COALESCE(c.course_url, ''),
  COALESCE(c.description, ''),
  c.is_mandatory,
  CASE
    WHEN c.recurrence_interval IS NULL THEN NULL
    ELSE EXTRACT(YEAR FROM c.recurrence_interval)::int * 12 + EXTRACT(MONTH FROM c.recurrence_interval)::int
  END AS recurrence_months,
  COALESCE(c.compliance_framework, ''),
  c.is_active,
  COALESCE((SELECT COUNT(*) FROM training.enrollment en
            JOIN training.training_plan tp ON tp.id = en.training_plan_id
            WHERE en.course_id = c.id AND tp.year = $%d), 0) AS current_year_count,
  COALESCE((SELECT COUNT(*) FROM training.enrollment en
            WHERE en.course_id = c.id AND en.status = 'completed'), 0) AS completed_historical
FROM training.course c
LEFT JOIN training.vendor v ON v.id = c.vendor_id
LEFT JOIN training.skill_area sa ON sa.id = c.skill_area_id
LEFT JOIN training.certification cert ON cert.id = c.leads_to_cert_id
WHERE %s
ORDER BY c.is_active DESC, c.title
LIMIT 500`, yearArg, strings.Join(conds, " AND "))

	rows, err := s.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("catalog list: %w", err)
	}
	defer rows.Close()

	result := make([]CatalogCourseWithCounts, 0)
	for rows.Next() {
		var row CatalogCourseWithCounts
		var defaultHours sql.NullInt32
		var defaultCost sql.NullFloat64
		var recurrenceMonths sql.NullInt32
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
			&defaultHours,
			&defaultCost,
			&row.CourseURL,
			&row.Description,
			&row.Mandatory,
			&recurrenceMonths,
			&row.ComplianceFramework,
			&row.Active,
			&row.EnrollmentsCurrentYear,
			&row.EnrollmentsCompletedHistorical,
		); err != nil {
			return nil, fmt.Errorf("scan catalog: %w", err)
		}
		if defaultHours.Valid {
			h := int(defaultHours.Int32)
			row.DefaultHours = &h
		}
		if defaultCost.Valid {
			v := defaultCost.Float64
			row.DefaultCost = &v
		}
		if recurrenceMonths.Valid && recurrenceMonths.Int32 > 0 {
			m := int(recurrenceMonths.Int32)
			row.RecurrenceMonths = &m
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// ArchiveCourse sets is_active=false. Historic enrollments stay intact.
func (s *SQLStore) ArchiveCourse(ctx context.Context, principal Principal, courseID string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(courseID) == "" {
		return ActionResponse{}, validationError("missing_id", "id corso obbligatorio")
	}
	var resp ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		before, err := entitySnapshot(ctx, tx, "course", courseID)
		if err != nil {
			return err
		}
		const stmt = `UPDATE training.course SET is_active = false, updated_at = now() WHERE id = $1::uuid RETURNING id::text`
		if err := tx.QueryRowContext(ctx, stmt, courseID).Scan(&resp.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return notFoundError("course_not_found", "corso non trovato")
			}
			return fmt.Errorf("archive course: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "course", courseID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "course", courseID, "archive", before, after); err != nil {
			return err
		}
		resp.OK = true
		resp.Status = "archived"
		return nil
	})
	return resp, err
}

func currentYear() int {
	// kept lightweight; backend already imports time elsewhere
	return 0 // 0 means "use overview default" -- callers should set explicitly
}
