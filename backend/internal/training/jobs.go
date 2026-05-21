package training

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/notifications"
)

type JobRunner struct {
	store          *SQLStore
	notifier       notifications.Notifier
	logger         *slog.Logger
	trainingAppURL string
	windows        []int
}

func NewJobRunner(store *SQLStore, notifier notifications.Notifier, logger *slog.Logger, trainingAppURL string) *JobRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &JobRunner{
		store:          store,
		notifier:       notifier,
		logger:         logger.With("component", "training", "worker", "jobs"),
		trainingAppURL: trainingAppURL,
		windows:        []int{90, 30, 7},
	}
}

func (r *JobRunner) RunOnce(ctx context.Context) (JobRunResponse, error) {
	if r == nil || r.store == nil {
		return JobRunResponse{}, serviceUnavailableError("training_database_not_configured", "database Training non configurato")
	}
	expired, err := r.store.ExpireClosedPlanEnrollments(ctx)
	if err != nil {
		return JobRunResponse{}, err
	}
	compliance, err := r.notifyComplianceGaps(ctx)
	if err != nil {
		r.logger.Warn("training compliance notification job failed", "error", err)
	}
	certifications, err := r.notifyExpiringCertifications(ctx)
	if err != nil {
		r.logger.Warn("training certification notification job failed", "error", err)
	}
	return JobRunResponse{
		OK:                         true,
		ExpiredEnrollments:         expired,
		ComplianceNotifications:    compliance,
		CertificationNotifications: certifications,
	}, nil
}

func (r *JobRunner) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := r.RunOnce(ctx); err != nil {
			r.logger.Warn("training jobs failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *JobRunner) notifyComplianceGaps(ctx context.Context) (int, error) {
	if r.notifier == nil {
		return 0, nil
	}
	gaps, err := r.store.NotificationComplianceGaps(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, gap := range gaps {
		_, err := r.notifier.Notify(ctx, notifications.NotifyInput{
			TypeKey:    "training.compliance_gap",
			Title:      "Formazione obbligatoria da pianificare",
			Body:       fmt.Sprintf("%s richiede una pianificazione o un aggiornamento.", gap.CourseTitle),
			EntityType: "training_compliance_gap",
			EntityID:   gap.EmployeeID + ":" + gap.CourseID,
			DedupeKey:  fmt.Sprintf("training:compliance_gap:%s:%s", gap.EmployeeID, gap.CourseID),
			DeepLink:   r.deepLink("/report"),
			Metadata: map[string]any{
				"employee_id": gap.EmployeeID,
				"course_id":   gap.CourseID,
				"course":      gap.CourseTitle,
				"status":      gap.ComplianceStatus,
			},
			Recipients: []notifications.Recipient{{
				Email: gap.EmployeeEmail,
				Name:  gap.EmployeeName,
			}},
		})
		if err == nil {
			count++
		}
	}
	return count, nil
}

func (r *JobRunner) notifyExpiringCertifications(ctx context.Context) (int, error) {
	if r.notifier == nil {
		return 0, nil
	}
	rows, err := r.store.NotificationExpiringCertifications(ctx, r.windows)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		_, err := r.notifier.Notify(ctx, notifications.NotifyInput{
			TypeKey:    "training.certificate_expiring",
			Title:      "Certificazione in scadenza",
			Body:       fmt.Sprintf("%s scade il %s.", row.CertificationName, row.ExpiresOn),
			EntityType: "training_certification_award",
			EntityID:   row.AwardID,
			DedupeKey:  fmt.Sprintf("training:certificate_expiring:%s:%d", row.AwardID, row.DaysToExpiry),
			DeepLink:   r.deepLink("/certificazioni"),
			Metadata: map[string]any{
				"award_id":           row.AwardID,
				"certification_code": row.CertificationCode,
				"expires_on":         row.ExpiresOn,
				"days_to_expiry":     row.DaysToExpiry,
			},
			Recipients: []notifications.Recipient{{
				Email: row.EmployeeEmail,
				Name:  row.EmployeeName,
			}},
		})
		if err == nil {
			count++
		}
	}
	return count, nil
}

func (r *JobRunner) deepLink(path string) string {
	base := strings.TrimRight(strings.TrimSpace(r.trainingAppURL), "/")
	if base == "" {
		base = "/apps/training"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

type notificationGap struct {
	EmployeeID       string
	EmployeeName     string
	EmployeeEmail    string
	CourseID         string
	CourseTitle      string
	ComplianceStatus string
}

type notificationCertification struct {
	AwardID           string
	EmployeeName      string
	EmployeeEmail     string
	CertificationCode string
	CertificationName string
	ExpiresOn         string
	DaysToExpiry      int
}

func (s *SQLStore) ExpireClosedPlanEnrollments(ctx context.Context) (int, error) {
	const stmt = `
WITH candidates AS (
  SELECT en.*
  FROM training.enrollment en
  JOIN training.training_plan tp ON tp.id = en.training_plan_id
  WHERE tp.status = 'closed'
    AND en.status IN ('proposed', 'approved')
), updated AS (
  UPDATE training.enrollment en
  SET status = 'expired'::training.enrollment_status
  FROM candidates c
  WHERE en.id = c.id
  RETURNING en.*
)
INSERT INTO training.audit_log (
  entity_type,
  entity_id,
  action,
  before_state,
  after_state,
  correlation_id
)
SELECT
  'enrollment',
  c.id,
  'transition:expire',
  to_jsonb(c),
  to_jsonb(u),
  gen_random_uuid()
FROM candidates c
JOIN updated u ON u.id = c.id`
	result, err := s.db.ExecContext(ctx, stmt)
	if err != nil {
		return 0, fmt.Errorf("expire training enrollments: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

func (s *SQLStore) NotificationComplianceGaps(ctx context.Context) ([]notificationGap, error) {
	const q = `
SELECT
  g.employee_id::text,
  concat(g.last_name, ' ', g.first_name),
  e.email::text,
  g.course_id::text,
  g.course_title,
  g.compliance_status
FROM training.v_mandatory_compliance_gap g
JOIN training.employee e ON e.id = g.employee_id
WHERE g.compliance_status <> 'compliant'
  AND e.status = 'active'
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training compliance notifications: %w", err)
	}
	defer rows.Close()
	result := []notificationGap{}
	for rows.Next() {
		var row notificationGap
		if err := rows.Scan(&row.EmployeeID, &row.EmployeeName, &row.EmployeeEmail, &row.CourseID, &row.CourseTitle, &row.ComplianceStatus); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) NotificationExpiringCertifications(ctx context.Context, windows []int) ([]notificationCertification, error) {
	windowList := notificationWindowList(windows)
	q := fmt.Sprintf(`
SELECT
  ca.id::text,
  concat(e.last_name, ' ', e.first_name),
  e.email::text,
  c.code,
  c.name,
  ca.expires_on::text,
  (ca.expires_on - CURRENT_DATE)::int
FROM training.certification_award ca
JOIN training.employee e ON e.id = ca.employee_id
JOIN training.certification c ON c.id = ca.certification_id
WHERE ca.outcome = 'passed_exam'
  AND ca.expires_on IS NOT NULL
  AND (ca.expires_on - CURRENT_DATE)::int IN (%s)
  AND e.status = 'active'`, windowList)
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training certification notifications: %w", err)
	}
	defer rows.Close()
	result := []notificationCertification{}
	for rows.Next() {
		var row notificationCertification
		if err := rows.Scan(&row.AwardID, &row.EmployeeName, &row.EmployeeEmail, &row.CertificationCode, &row.CertificationName, &row.ExpiresOn, &row.DaysToExpiry); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func notificationWindowList(windows []int) string {
	if len(windows) == 0 {
		windows = []int{90, 30, 7}
	}
	values := make([]string, 0, len(windows))
	seen := make(map[int]struct{}, len(windows))
	for _, window := range windows {
		if window <= 0 || window > 3650 {
			continue
		}
		if _, ok := seen[window]; ok {
			continue
		}
		seen[window] = struct{}{}
		values = append(values, fmt.Sprint(window))
	}
	if len(values) == 0 {
		return "90, 30, 7"
	}
	return strings.Join(values, ", ")
}
