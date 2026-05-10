package support

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/sciacco/mrsmith/internal/auth"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) CreateRequest(ctx context.Context, input CreateRequestInput) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("support database not configured")
	}

	contextJSON, err := json.Marshal(input.Context)
	if err != nil {
		return 0, fmt.Errorf("marshal support context: %w", err)
	}
	rolesJSON, err := json.Marshal(input.Requester.Roles)
	if err != nil {
		return 0, fmt.Errorf("marshal requester roles: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin support request transaction: %w", err)
	}
	defer rollbackTx(tx)

	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO mrsmith.support_request (
			status,
			priority,
			app_id,
			app_name,
			page_url,
			page_path,
			message,
			requester_subject,
			requester_name,
			requester_email,
			requester_roles,
			technical_context_included,
			email_notification_status
		)
		VALUES (
			'open',
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10::jsonb, $11,
			$12
		)
		RETURNING id
	`,
		input.Priority,
		input.AppID,
		input.AppName,
		input.PageURL,
		input.PagePath,
		input.Message,
		input.Requester.Subject,
		input.Requester.Name,
		input.Requester.Email,
		string(rolesJSON),
		input.TechnicalContextIncluded,
		emailNotificationPending,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert support request: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mrsmith.support_request_context (request_id, context)
		VALUES ($1, $2::jsonb)
	`, id, string(contextJSON)); err != nil {
		return 0, fmt.Errorf("insert support request context: %w", err)
	}

	eventPayload, err := json.Marshal(map[string]any{
		"priority": input.Priority,
		"app_id":   input.AppID,
	})
	if err != nil {
		return 0, fmt.Errorf("marshal support event payload: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mrsmith.support_request_event (
			request_id,
			event_type,
			actor_subject,
			actor_name,
			actor_email,
			payload
		)
		VALUES ($1, 'created', $2, $3, $4, $5::jsonb)
	`, id, input.Requester.Subject, input.Requester.Name, input.Requester.Email, string(eventPayload)); err != nil {
		return 0, fmt.Errorf("insert support request event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit support request: %w", err)
	}
	return id, nil
}

func (s *SQLStore) UpdateEmailStatus(ctx context.Context, id int64, status string, actor auth.Claims) error {
	if s == nil || s.db == nil {
		return errors.New("support database not configured")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin support email status transaction: %w", err)
	}
	defer rollbackTx(tx)

	if _, err := tx.ExecContext(ctx, `
		UPDATE mrsmith.support_request
		SET email_notification_status = $2,
		    updated_at = now()
		WHERE id = $1
	`, id, status); err != nil {
		return fmt.Errorf("update support email status: %w", err)
	}

	payload, err := json.Marshal(map[string]string{"status": status})
	if err != nil {
		return fmt.Errorf("marshal support email event payload: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mrsmith.support_request_event (
			request_id,
			event_type,
			actor_subject,
			actor_name,
			actor_email,
			payload
		)
		VALUES ($1, 'email_notification', $2, $3, $4, $5::jsonb)
	`, id, actor.Subject, actor.Name, actor.Email, string(payload)); err != nil {
		return fmt.Errorf("insert support email event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit support email status: %w", err)
	}
	return nil
}

func (s *SQLStore) GetStringListConfig(ctx context.Context, namespace string, key string) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("support database not configured")
	}

	var raw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT value
		FROM mrsmith.runtime_config
		WHERE namespace = $1 AND key = $2
	`, namespace, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read runtime config %s.%s: %w", namespace, key, err)
	}
	return parseStringListConfig(raw)
}

func parseStringListConfig(raw []byte) ([]string, error) {
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return normalizeEmailRecipients(list), nil
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return normalizeEmailRecipients(strings.Split(single, ",")), nil
	}

	return nil, fmt.Errorf("runtime config value must be a JSON array of strings")
}

func normalizeEmailRecipients(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		address, err := mail.ParseAddress(trimmed)
		if err != nil || address.Address == "" {
			continue
		}
		normalized := strings.ToLower(address.Address)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, address.Address)
	}
	return result
}

func rollbackTx(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}
