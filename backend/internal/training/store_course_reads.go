package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Catalogo corsi (#152, slice 1 del task 6). Include i corsi inattivi (flag
// active); la lookup corsi dei form resta invariata (store.go).

const courseListColumns = `
  c.id::text,
  c.title,
  COALESCE((
    SELECT json_agg(json_build_object('id', sa.id::text, 'name', sa.name) ORDER BY sa.name)
    FROM training.course_skill_area csa
    JOIN training.skill_area sa ON sa.id = csa.skill_area_id
    WHERE csa.course_id = c.id
  ), '[]')::text,
  COALESCE(c.vendor_id::text, ''),
  COALESCE(v.name, ''),
  c.delivery_mode::text,
  c.provider_kind::text,
  c.default_hours,
  c.default_cost::float8,
  COALESCE(c.leads_to_cert_id::text, ''),
  COALESCE(cert.name, ''),
  c.is_compliance_course,
  COALESCE(c.compliance_framework, ''),
  c.is_active,
  COALESCE(c.factorial_training_id, ''),
  COALESCE(to_json(c.tags)::text, '[]'),
  COALESCE(c.reminder_text, ''),
  COALESCE(c.reminder_at::text, ''),
  COALESCE(c.suspended_at::text, ''),
  COALESCE((
    SELECT json_agg(json_build_object('employeeId', e.id::text, 'name', concat(e.last_name, ' ', e.first_name)) ORDER BY e.last_name, e.first_name)
    FROM training.course_trainer ct
    JOIN training.employee e ON e.id = ct.employee_id
    WHERE ct.course_id = c.id
  ), '[]')::text,
  c.updated_at::text`

const courseListFrom = `
FROM training.course c
LEFT JOIN training.vendor v ON v.id = c.vendor_id
LEFT JOIN training.certification cert ON cert.id = c.leads_to_cert_id`

func (s *SQLStore) ListCourses(ctx context.Context) ([]CourseListRow, error) {
	q := "SELECT" + courseListColumns + courseListFrom + "\nORDER BY c.title\nLIMIT 1000"
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training courses: %w", err)
	}
	defer rows.Close()

	result := make([]CourseListRow, 0)
	for rows.Next() {
		var row CourseListRow
		var hours sql.NullInt64
		var cost sql.NullFloat64
		var areasRaw, tagsRaw, trainersRaw string
		if err := rows.Scan(
			&row.ID, &row.Title, &areasRaw, &row.VendorID, &row.VendorName,
			&row.DeliveryMode, &row.ProviderKind, &hours, &cost, &row.LeadsToCertID, &row.LeadsToCertName,
			&row.ComplianceRelated, &row.ComplianceFramework, &row.Active, &row.FactorialTrainingID, &tagsRaw,
			&row.ReminderText, &row.ReminderAt, &row.SuspendedAt, &trainersRaw, &row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan training course: %w", err)
		}
		if err := json.Unmarshal([]byte(areasRaw), &row.SkillAreas); err != nil {
			return nil, fmt.Errorf("decode training course skill areas: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsRaw), &row.Tags); err != nil {
			return nil, fmt.Errorf("decode training course tags: %w", err)
		}
		if err := json.Unmarshal([]byte(trainersRaw), &row.Trainers); err != nil {
			return nil, fmt.Errorf("decode training course trainers: %w", err)
		}
		row.DefaultHours = nullInt(hours)
		row.DefaultCost = nullFloat(cost)
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetCourseDetail aggrega regole ed eventi collegati in JSON lato query
// (stesso pattern di store_people_reads.go): un'unica riga invece di N+1
// letture.
func (s *SQLStore) GetCourseDetail(ctx context.Context, id string) (CourseDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return CourseDetail{}, validationError("missing_id", "id corso obbligatorio")
	}
	q := "SELECT" + courseListColumns + `,
  COALESCE(c.description, ''),
  COALESCE(c.course_url, ''),
  COALESCE(c.notes, ''),
  COALESCE(c.suspension_reason, ''),
  COALESCE(c.visibility_target::text, ''),
  COALESCE((
    SELECT json_agg(cvp.employee_id::text ORDER BY cvp.employee_id)
    FROM training.course_visibility_person cvp
    WHERE cvp.course_id = c.id
  ), '[]')::text,
  COALESCE((
    SELECT json_agg(json_build_object('id', r.id::text, 'name', r.name, 'isActive', r.is_active) ORDER BY r.is_active DESC, r.name)
    FROM training.training_rule r WHERE r.course_id = c.id
  ), '[]')::text,
  COALESCE((
    SELECT json_agg(json_build_object(
      'id', ev.id::text, 'createdAt', ev.created_at::text, 'cancelledAt', COALESCE(ev.cancelled_at::text, ''),
      'enrollmentsCount', (SELECT COUNT(*) FROM training.enrollment en WHERE en.event_id = ev.id),
      'sessionsCount', (SELECT COUNT(*) FROM training.training_session s WHERE s.event_id = ev.id)
    ) ORDER BY ev.created_at DESC)
    FROM training.training_event ev WHERE ev.course_id = c.id
  ), '[]')::text` + courseListFrom + `
WHERE c.id = $1::uuid`
	var (
		detail                                 CourseDetail
		hours                                  sql.NullInt64
		cost                                   sql.NullFloat64
		areasRaw, tagsRaw, rulesRaw, eventsRaw string
		trainersRaw, visibilityRaw, peopleRaw  string
	)
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&detail.ID, &detail.Title, &areasRaw, &detail.VendorID, &detail.VendorName,
		&detail.DeliveryMode, &detail.ProviderKind, &hours, &cost, &detail.LeadsToCertID, &detail.LeadsToCertName,
		&detail.ComplianceRelated, &detail.ComplianceFramework, &detail.Active, &detail.FactorialTrainingID, &tagsRaw,
		&detail.ReminderText, &detail.ReminderAt, &detail.SuspendedAt, &trainersRaw, &detail.UpdatedAt,
		&detail.Description, &detail.CourseURL, &detail.Notes, &detail.SuspensionReason,
		&visibilityRaw, &peopleRaw, &rulesRaw, &eventsRaw,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return CourseDetail{}, notFoundError("course_not_found", "corso non trovato")
	}
	if err != nil {
		return CourseDetail{}, fmt.Errorf("load training course detail: %w", err)
	}
	if err := json.Unmarshal([]byte(areasRaw), &detail.SkillAreas); err != nil {
		return CourseDetail{}, fmt.Errorf("decode training course skill areas: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsRaw), &detail.Tags); err != nil {
		return CourseDetail{}, fmt.Errorf("decode training course tags: %w", err)
	}
	if err := json.Unmarshal([]byte(trainersRaw), &detail.Trainers); err != nil {
		return CourseDetail{}, fmt.Errorf("decode training course trainers: %w", err)
	}
	if visibilityRaw != "" {
		var vis CourseVisibilityInput
		if err := json.Unmarshal([]byte(visibilityRaw), &vis); err != nil {
			return CourseDetail{}, fmt.Errorf("decode training course visibility: %w", err)
		}
		if vis.Kind == "people" {
			if err := json.Unmarshal([]byte(peopleRaw), &vis.EmployeeIDs); err != nil {
				return CourseDetail{}, fmt.Errorf("decode training course visibility people: %w", err)
			}
		}
		detail.Visibility = &vis
	}
	detail.DefaultHours = nullInt(hours)
	detail.DefaultCost = nullFloat(cost)
	if detail.Rules, err = decodeJSONSlice[CourseRuleRef](rulesRaw); err != nil {
		return CourseDetail{}, err
	}
	if detail.Events, err = decodeJSONSlice[CourseEventRef](eventsRaw); err != nil {
		return CourseDetail{}, err
	}
	return detail, nil
}
