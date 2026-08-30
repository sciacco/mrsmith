package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (s *SQLStore) ListEvents(ctx context.Context) ([]EventListRow, error) {
	const q = `
SELECT
  ev.id::text,
  ev.course_id::text,
  c.title,
  COALESCE(ev.vendor_id::text, ''),
  COALESCE(v.name, ''),
  ev.agreed_price::float8,
  ev.origin,
  COALESCE(ev.cancelled_at::text, ''),
  (SELECT COUNT(*) FROM training.training_session s WHERE s.event_id = ev.id),
  (SELECT COUNT(*) FROM training.enrollment en WHERE en.event_id = ev.id AND en.delivery_status <> 'cancelled'),
  (SELECT COUNT(*) FROM training.enrollment en WHERE en.event_id = ev.id AND en.delivery_status = 'cancelled'),
  cond.cancelled,
  cond.without_sessions,
  cond.unassigned_enrollments,
  cond.in_progress,
  cond.needs_reconciliation,
  cond.concluded,
  ev.created_at::text,
  ev.updated_at::text
FROM training.training_event ev
JOIN training.course c ON c.id = ev.course_id
LEFT JOIN training.vendor v ON v.id = ev.vendor_id
JOIN training.v_event_operational_condition cond ON cond.event_id = ev.id
ORDER BY ev.created_at DESC, ev.id
LIMIT 5000`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training events: %w", err)
	}
	defer rows.Close()

	result := make([]EventListRow, 0)
	for rows.Next() {
		var row EventListRow
		var price sql.NullFloat64
		if err := rows.Scan(
			&row.ID,
			&row.CourseID,
			&row.CourseTitle,
			&row.VendorID,
			&row.VendorName,
			&price,
			&row.Origin,
			&row.CancelledAt,
			&row.SessionsCount,
			&row.EnrollmentsCount,
			&row.CancelledEnrollmentsCount,
			&row.Flags.Cancelled,
			&row.Flags.WithoutSessions,
			&row.Flags.UnassignedEnrollments,
			&row.Flags.InProgress,
			&row.Flags.NeedsReconciliation,
			&row.Flags.Concluded,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan training event: %w", err)
		}
		row.AgreedPrice = nullFloat(price)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) GetEventDetail(ctx context.Context, id string) (EventDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return EventDetail{}, validationError("missing_id", "id evento obbligatorio")
	}

	const headerQuery = `
SELECT
  ev.id::text,
  ev.course_id::text,
  c.title,
  COALESCE(ev.vendor_id::text, ''),
  COALESCE(v.name, ''),
  ev.agreed_price::float8,
  COALESCE(ev.agreed_conditions, ''),
  ev.origin,
  COALESCE(ev.source_rule_id::text, ''),
  COALESCE(ev.source_request_id::text, ''),
  COALESCE(ev.rule_deadline::text, ''),
  COALESCE(ev.factorial_class_id, ''),
  COALESCE(ev.cancelled_at::text, ''),
  COALESCE(ev.cancellation_reason, ''),
  COALESCE(ev.notes, ''),
  cond.cancelled,
  cond.without_sessions,
  cond.unassigned_enrollments,
  cond.in_progress,
  cond.needs_reconciliation,
  cond.concluded,
  ev.created_at::text,
  ev.updated_at::text
FROM training.training_event ev
JOIN training.course c ON c.id = ev.course_id
LEFT JOIN training.vendor v ON v.id = ev.vendor_id
JOIN training.v_event_operational_condition cond ON cond.event_id = ev.id
WHERE ev.id = $1::uuid`
	var detail EventDetail
	var price sql.NullFloat64
	err := s.db.QueryRowContext(ctx, headerQuery, id).Scan(
		&detail.ID,
		&detail.CourseID,
		&detail.CourseTitle,
		&detail.VendorID,
		&detail.VendorName,
		&price,
		&detail.AgreedConditions,
		&detail.Origin,
		&detail.SourceRuleID,
		&detail.SourceRequestID,
		&detail.RuleDeadline,
		&detail.FactorialClassID,
		&detail.CancelledAt,
		&detail.CancellationReason,
		&detail.Notes,
		&detail.Flags.Cancelled,
		&detail.Flags.WithoutSessions,
		&detail.Flags.UnassignedEnrollments,
		&detail.Flags.InProgress,
		&detail.Flags.NeedsReconciliation,
		&detail.Flags.Concluded,
		&detail.CreatedAt,
		&detail.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return EventDetail{}, notFoundError("event_not_found", "evento non trovato")
	}
	if err != nil {
		return EventDetail{}, fmt.Errorf("load training event detail: %w", err)
	}
	detail.AgreedPrice = nullFloat(price)

	if detail.Sessions, err = s.eventSessions(ctx, id); err != nil {
		return EventDetail{}, err
	}
	if detail.Enrollments, err = s.eventEnrollments(ctx, id); err != nil {
		return EventDetail{}, err
	}
	if detail.Participations, err = s.eventParticipations(ctx, id); err != nil {
		return EventDetail{}, err
	}
	return detail, nil
}

func (s *SQLStore) eventSessions(ctx context.Context, eventID string) ([]SessionDetail, error) {
	const q = `
SELECT
  s.id::text,
  COALESCE(s.schedule_type, ''),
  COALESCE(s.starts_at::text, ''),
  COALESCE(s.ends_at::text, ''),
  COALESCE(s.due_at::text, ''),
  s.max_capacity,
  (
    SELECT COUNT(*)
    FROM training.enrollment_session es
    JOIN training.enrollment en ON en.id = es.enrollment_id
    WHERE es.session_id = s.id
      AND es.participation_status IN ('assigned', 'in_progress')
      AND en.delivery_status <> 'cancelled'
  ),
  COALESCE(s.notes, ''),
  COALESCE(s.factorial_session_id, ''),
  COALESCE(s.topic, ''),
  COALESCE(s.modality, ''),
  COALESCE(s.duration_hours::float8, 0),
  COALESCE(s.location, ''),
  s.created_at::text,
  s.updated_at::text
FROM training.training_session s
WHERE s.event_id = $1::uuid
ORDER BY s.starts_at NULLS LAST, s.due_at NULLS LAST, s.created_at, s.id`
	rows, err := s.db.QueryContext(ctx, q, eventID)
	if err != nil {
		return nil, fmt.Errorf("list training event sessions: %w", err)
	}
	defer rows.Close()

	result := make([]SessionDetail, 0)
	for rows.Next() {
		var row SessionDetail
		var capacity sql.NullInt64
		var durationHours float64
		if err := rows.Scan(
			&row.ID,
			&row.ScheduleType,
			&row.StartsAt,
			&row.EndsAt,
			&row.DueAt,
			&capacity,
			&row.Occupancy,
			&row.Notes,
			&row.FactorialSessionID,
			&row.Topic,
			&row.Modality,
			&durationHours,
			&row.Location,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan training event session: %w", err)
		}
		if durationHours > 0 {
			row.DurationHours = &durationHours
		}
		row.MaxCapacity = nullInt(capacity)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) eventEnrollments(ctx context.Context, eventID string) ([]EnrollmentDetail, error) {
	const q = `
SELECT
  en.id::text,
  e.id::text,
  concat(e.last_name, ' ', e.first_name),
  e.email::text,
  en.delivery_status,
  COALESCE(en.learning_outcome, ''),
  en.origin,
  COALESCE(en.objective, ''),
  COALESCE(en.notes, ''),
  COALESCE(en.actual_start::text, ''),
  COALESCE(en.actual_end::text, ''),
  en.hours_actual,
  COALESCE(en.cancellation_reason, ''),
  en.created_at::text,
  en.updated_at::text
FROM training.enrollment en
JOIN training.employee e ON e.id = en.employee_id
WHERE en.event_id = $1::uuid
ORDER BY e.last_name, e.first_name, en.created_at, en.id`
	rows, err := s.db.QueryContext(ctx, q, eventID)
	if err != nil {
		return nil, fmt.Errorf("list training event enrollments: %w", err)
	}
	defer rows.Close()

	result := make([]EnrollmentDetail, 0)
	for rows.Next() {
		var row EnrollmentDetail
		var hours sql.NullInt64
		if err := rows.Scan(
			&row.ID,
			&row.EmployeeID,
			&row.EmployeeName,
			&row.EmployeeEmail,
			&row.DeliveryStatus,
			&row.LearningOutcome,
			&row.Origin,
			&row.Objective,
			&row.Notes,
			&row.ActualStart,
			&row.ActualEnd,
			&hours,
			&row.CancellationReason,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan training event enrollment: %w", err)
		}
		row.HoursActual = nullInt(hours)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) eventParticipations(ctx context.Context, eventID string) ([]ParticipationRow, error) {
	const q = `
SELECT
  es.enrollment_id::text,
  es.session_id::text,
  es.participation_status,
  COALESCE(es.completed_hours::float8, 0),
  es.assigned_at::text,
  es.updated_at::text
FROM training.enrollment_session es
JOIN training.enrollment en ON en.id = es.enrollment_id
WHERE en.event_id = $1::uuid
ORDER BY es.assigned_at, es.enrollment_id, es.session_id`
	rows, err := s.db.QueryContext(ctx, q, eventID)
	if err != nil {
		return nil, fmt.Errorf("list training event participations: %w", err)
	}
	defer rows.Close()

	result := make([]ParticipationRow, 0)
	for rows.Next() {
		var row ParticipationRow
		var completedHours float64
		if err := rows.Scan(
			&row.EnrollmentID,
			&row.SessionID,
			&row.ParticipationStatus,
			&completedHours,
			&row.AssignedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan training event participation: %w", err)
		}
		if completedHours > 0 {
			row.CompletedHours = &completedHours
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
