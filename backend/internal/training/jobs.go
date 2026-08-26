package training

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/notifications"
	"github.com/sciacco/mrsmith/internal/platform/directory"
	"github.com/sciacco/mrsmith/pkg/factorial"
)

type JobRunner struct {
	store          *SQLStore
	notifier       notifications.Notifier
	logger         *slog.Logger
	trainingAppURL string
	windows        []int
	directory      directory.Provider
	syncDirectory  bool
	factorial      FactorialSyncDeps
	syncFactorial  bool
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

// WithDirectorySync enables the periodic anagrafica reconciliation. It stays
// off unless explicitly enabled: the database is shared across environments.
func (r *JobRunner) WithDirectorySync(provider directory.Provider, enabled bool) *JobRunner {
	if r == nil {
		return r
	}
	r.directory = provider
	r.syncDirectory = enabled && provider != nil
	return r
}

// WithFactorialSync enables the periodic Factorial training reconciliation.
// It stays off unless directory sync is enabled and both the Factorial
// client and the technical (author) employee id are configured: the run
// depends on a fresh directory reconciliation and cannot export to Factorial
// without an author identity.
func (r *JobRunner) WithFactorialSync(client *factorial.Client, technicalEmployeeID string, enabled bool) *JobRunner {
	if r == nil {
		return r
	}
	r.factorial = FactorialSyncDeps{Directory: r.directory, Factorial: client, TechnicalEmployeeID: technicalEmployeeID, Logger: r.logger}
	ready := r.syncDirectory && client != nil && strings.TrimSpace(technicalEmployeeID) != ""
	r.syncFactorial = enabled && ready
	if enabled && !ready {
		r.logger.Info("training factorial sync disabled: prerequisites not met",
			"directory_sync_enabled", r.syncDirectory,
			"factorial_client_configured", client != nil,
			"author_employee_id_configured", strings.TrimSpace(technicalEmployeeID) != "")
	}
	return r
}

func (r *JobRunner) RunOnce(ctx context.Context) (JobRunResponse, error) {
	if r == nil || r.store == nil {
		return JobRunResponse{}, serviceUnavailableError("training_database_not_configured", "database Training non configurato")
	}
	certifications, err := r.notifyExpiringCertifications(ctx)
	if err != nil {
		r.logger.Warn("training certification notification job failed", "error", err)
	}
	// RunFactorialSync esegue il directory sync come proprio prerequisito
	// (stessa chiave di lock, in sequenza): quando e' abilitato, il blocco
	// standalone qui sotto salterebbe duplicando la reconciliation ogni tick.
	if r.syncDirectory && !r.syncFactorial {
		if _, err := r.store.RunDirectorySync(ctx, r.directory, "job", false); err != nil {
			r.logger.Warn("training directory sync job failed", "error", err)
		}
	}
	if r.syncFactorial {
		if _, err := r.store.RunFactorialSync(ctx, r.factorial, "job", false); err != nil {
			r.logger.Warn("training factorial sync job failed", "error", err)
		}
	}
	return JobRunResponse{
		OK:                         true,
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

type notificationCertification struct {
	AwardID           string
	EmployeeName      string
	EmployeeEmail     string
	CertificationCode string
	CertificationName string
	ExpiresOn         string
	DaysToExpiry      int
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
