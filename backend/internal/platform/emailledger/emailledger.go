// Package emailledger is the tracked send path for direct, user-triggered
// emails. Every attempt — accepted by the SMTP relay or failed — is recorded
// in mrsmith.email_send, correlated to the originating app entity, so apps can
// answer "how many emails went out for this item, when, by whom, to whom"
// before offering a resend. Policy-driven notification emails keep their own
// pipeline (internal/notifications) and do not go through the ledger.
package emailledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sciacco/mrsmith/internal/platform/email"
)

var ErrNotConfigured = errors.New("email ledger not configured")

const (
	StatusAccepted = "accepted" // accepted by the SMTP relay, not delivered
	StatusFailed   = "failed"
)

// Correlation ties a send to the app entity it belongs to. All four fields are
// required; the typical UI counter filters on the full tuple.
type Correlation struct {
	App        string // e.g. "rda"
	EntityType string // e.g. "po"
	EntityID   string // e.g. "123"
	Purpose    string // e.g. "invio-fornitore"
}

func (c Correlation) normalized() Correlation {
	return Correlation{
		App:        strings.TrimSpace(c.App),
		EntityType: strings.TrimSpace(c.EntityType),
		EntityID:   strings.TrimSpace(c.EntityID),
		Purpose:    strings.TrimSpace(c.Purpose),
	}
}

func (c Correlation) validate() error {
	if c.App == "" || c.EntityType == "" || c.EntityID == "" || c.Purpose == "" {
		return fmt.Errorf("email ledger correlation requires app, entity type, entity id and purpose")
	}
	return nil
}

// Actor is the authenticated user who triggered the send.
type Actor struct {
	Subject string // Keycloak claims subject
	Email   string // Keycloak claims email
}

// SendRecord is one row of mrsmith.email_send.
type SendRecord struct {
	ID          int64
	Correlation Correlation
	Actor       Actor
	To          []string
	Cc          []string
	Bcc         []string
	Subject     string
	MessageID   string
	Status      string // StatusAccepted | StatusFailed
	Error       string
	SentAt      time.Time
}

type Service struct {
	db     *sql.DB
	client *email.Client
	logger *slog.Logger
}

// New returns a nil-safe Service. db may be nil (ledger degraded: sends are
// refused, an untrackable send must not leave); client disabled is recorded as
// a failed attempt.
func New(db *sql.DB, client *email.Client, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{db: db, client: client, logger: logger}
}

func (s *Service) Enabled() bool {
	return s != nil && s.db != nil && s.client.Enabled()
}

// Send delivers msg synchronously through the shared SMTP client and records
// the attempt. The returned record reflects what was persisted; on SMTP
// failure the error is returned alongside the failed record. A send that
// succeeded but could not be recorded still returns success — the discrepancy
// is observable via diagnostic events.
func (s *Service) Send(ctx context.Context, msg email.Message, corr Correlation, actor Actor) (SendRecord, error) {
	corr = corr.normalized()
	if err := corr.validate(); err != nil {
		return SendRecord{}, err
	}
	if s == nil || s.db == nil {
		return SendRecord{}, ErrNotConfigured
	}

	if msg.MessageID == "" {
		id, err := email.NewMessageID(msg.From)
		if err != nil {
			return SendRecord{}, err
		}
		msg.MessageID = id
	}

	record := SendRecord{
		Correlation: corr,
		Actor: Actor{
			Subject: strings.TrimSpace(actor.Subject),
			Email:   strings.TrimSpace(actor.Email),
		},
		To:        cleanAddresses(msg.To),
		Cc:        cleanAddresses(msg.Cc),
		Bcc:       cleanAddresses(msg.Bcc),
		Subject:   msg.Subject,
		MessageID: msg.MessageID,
		Status:    StatusAccepted,
		SentAt:    time.Now().UTC(),
	}

	sendErr := s.client.Send(ctx, msg)
	if sendErr != nil {
		record.Status = StatusFailed
		record.Error = sendErr.Error()
		s.logError(ctx, "email send failed", record, sendErr)
	}

	if err := s.insert(ctx, &record); err != nil {
		s.logError(ctx, "email ledger insert failed", record, err)
		if sendErr == nil {
			// The email is out; the caller must not surface it as failed.
			return record, nil
		}
	}
	return record, sendErr
}

// Count returns the number of accepted sends for the correlation tuple.
func (s *Service) Count(ctx context.Context, corr Correlation) (int, error) {
	corr = corr.normalized()
	if err := corr.validate(); err != nil {
		return 0, err
	}
	if s == nil || s.db == nil {
		return 0, ErrNotConfigured
	}
	var count int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM mrsmith.email_send
WHERE app = $1 AND entity_type = $2 AND entity_id = $3 AND purpose = $4
  AND status = $5`,
		corr.App, corr.EntityType, corr.EntityID, corr.Purpose, StatusAccepted,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count email sends: %w", err)
	}
	return count, nil
}

// LatestAccepted returns the newest accepted send for the correlation tuple.
func (s *Service) LatestAccepted(ctx context.Context, corr Correlation) (SendRecord, bool, error) {
	corr = corr.normalized()
	if err := corr.validate(); err != nil {
		return SendRecord{}, false, err
	}
	if s == nil || s.db == nil {
		return SendRecord{}, false, ErrNotConfigured
	}

	record := SendRecord{Correlation: corr}
	var to, cc, bcc pgtype.FlatArray[string]
	err := s.db.QueryRowContext(ctx, `
SELECT id, actor_subject, actor_email,
       recipients_to, recipients_cc, recipients_bcc,
       subject, message_id, status, error, sent_at
FROM mrsmith.email_send
WHERE app = $1 AND entity_type = $2 AND entity_id = $3 AND purpose = $4
  AND status = $5
ORDER BY sent_at DESC, id DESC
LIMIT 1`, corr.App, corr.EntityType, corr.EntityID, corr.Purpose, StatusAccepted).Scan(
		&record.ID, &record.Actor.Subject, &record.Actor.Email,
		scanTextArray(&to), scanTextArray(&cc), scanTextArray(&bcc),
		&record.Subject, &record.MessageID, &record.Status, &record.Error, &record.SentAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return SendRecord{}, false, nil
	}
	if err != nil {
		return SendRecord{}, false, fmt.Errorf("latest accepted email send: %w", err)
	}
	record.To, record.Cc, record.Bcc = to, cc, bcc
	return record, true, nil
}

// List returns the send history for the correlation tuple, newest first, all
// statuses included. limit <= 0 falls back to a sane default.
func (s *Service) List(ctx context.Context, corr Correlation, limit int) ([]SendRecord, error) {
	corr = corr.normalized()
	if err := corr.validate(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, ErrNotConfigured
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, actor_subject, actor_email,
       recipients_to, recipients_cc, recipients_bcc,
       subject, message_id, status, error, sent_at
FROM mrsmith.email_send
WHERE app = $1 AND entity_type = $2 AND entity_id = $3 AND purpose = $4
ORDER BY sent_at DESC, id DESC
LIMIT $5`,
		corr.App, corr.EntityType, corr.EntityID, corr.Purpose, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list email sends: %w", err)
	}
	defer rows.Close()

	records := make([]SendRecord, 0)
	for rows.Next() {
		record := SendRecord{Correlation: corr}
		var to, cc, bcc pgtype.FlatArray[string]
		if err := rows.Scan(
			&record.ID,
			&record.Actor.Subject,
			&record.Actor.Email,
			scanTextArray(&to),
			scanTextArray(&cc),
			scanTextArray(&bcc),
			&record.Subject,
			&record.MessageID,
			&record.Status,
			&record.Error,
			&record.SentAt,
		); err != nil {
			return nil, fmt.Errorf("scan email send: %w", err)
		}
		record.To = to
		record.Cc = cc
		record.Bcc = bcc
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate email sends: %w", err)
	}
	return records, nil
}

func (s *Service) insert(ctx context.Context, record *SendRecord) error {
	err := s.db.QueryRowContext(ctx, `
INSERT INTO mrsmith.email_send (
  app, entity_type, entity_id, purpose,
  actor_subject, actor_email,
  recipients_to, recipients_cc, recipients_bcc,
  subject, message_id, status, error, sent_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id`,
		record.Correlation.App, record.Correlation.EntityType, record.Correlation.EntityID, record.Correlation.Purpose,
		record.Actor.Subject, record.Actor.Email,
		record.To, record.Cc, record.Bcc,
		record.Subject, record.MessageID, record.Status, record.Error, record.SentAt,
	).Scan(&record.ID)
	if err != nil {
		return fmt.Errorf("insert email send: %w", err)
	}
	return nil
}

// logError feeds the shared diagnostics pipeline: the root logger is wrapped by
// diagnostics.NewSlogHandler, so Error records land in mrsmith.diagnostic_event.
func (s *Service) logError(ctx context.Context, message string, record SendRecord, err error) {
	s.logger.ErrorContext(ctx, message,
		"component", "email",
		"operation", "ledger_send",
		"app", record.Correlation.App,
		"entity_type", record.Correlation.EntityType,
		"entity_id", record.Correlation.EntityID,
		"purpose", record.Correlation.Purpose,
		"actor_email", record.Actor.Email,
		"message_id", record.MessageID,
		"error", err,
	)
}

func cleanAddresses(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func scanTextArray(value *pgtype.FlatArray[string]) sql.Scanner {
	return pgtype.NewMap().SQLScanner(value)
}
