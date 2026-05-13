package diagnostics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) InsertEvents(ctx context.Context, events []Event) error {
	if s == nil || s.db == nil {
		return errors.New("diagnostics database not configured")
	}
	if len(events) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin diagnostic event transaction: %w", err)
	}
	defer rollbackTx(tx)

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO mrsmith.diagnostic_event (
			observed_at,
			level,
			message,
			component,
			operation,
			request_id,
			method,
			path,
			status,
			auth_subject,
			error,
			source_file,
			source_line,
			source_function,
			attrs,
			stack,
			dropped_before
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15::jsonb, $16, $17
		)
	`)
	if err != nil {
		return fmt.Errorf("prepare diagnostic event insert: %w", err)
	}
	defer stmt.Close()

	for _, event := range events {
		attrsJSON, err := attrsForStorage(event.Attrs)
		if err != nil {
			return err
		}
		var status any
		if event.Status != nil {
			status = *event.Status
		}
		if _, err := stmt.ExecContext(ctx,
			event.ObservedAt,
			event.Level,
			event.Message,
			event.Component,
			event.Operation,
			event.RequestID,
			event.Method,
			event.Path,
			status,
			event.AuthSubject,
			event.Error,
			event.SourceFile,
			event.SourceLine,
			event.SourceFunction,
			string(attrsJSON),
			event.Stack,
			event.DroppedBefore,
		); err != nil {
			return fmt.Errorf("insert diagnostic event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit diagnostic events: %w", err)
	}
	return nil
}

func (s *SQLStore) ListEvents(ctx context.Context, filter ListFilter) ([]Event, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("diagnostics database not configured")
	}
	query, args := buildListQuery(filter)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list diagnostic events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate diagnostic events: %w", err)
	}
	return events, nil
}

func (s *SQLStore) GetEvent(ctx context.Context, id int64) (Event, bool, error) {
	if s == nil || s.db == nil {
		return Event{}, false, errors.New("diagnostics database not configured")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT
			id,
			observed_at,
			level,
			message,
			component,
			operation,
			request_id,
			method,
			path,
			status,
			auth_subject,
			error,
			source_file,
			source_line,
			source_function,
			attrs,
			stack,
			dropped_before,
			created_at
		FROM mrsmith.diagnostic_event
		WHERE id = $1
	`, id)
	event, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, false, nil
	}
	if err != nil {
		return Event{}, false, err
	}
	return event, true, nil
}

func (s *SQLStore) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("diagnostics database not configured")
	}
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM mrsmith.diagnostic_event
		WHERE observed_at < $1
	`, before)
	if err != nil {
		return 0, fmt.Errorf("delete old diagnostic events: %w", err)
	}
	deleted, _ := result.RowsAffected()
	return deleted, nil
}

type eventScanner interface {
	Scan(dest ...any) error
}

func scanEvent(scanner eventScanner) (Event, error) {
	var event Event
	var status sql.NullInt64
	var attrsRaw []byte
	if err := scanner.Scan(
		&event.ID,
		&event.ObservedAt,
		&event.Level,
		&event.Message,
		&event.Component,
		&event.Operation,
		&event.RequestID,
		&event.Method,
		&event.Path,
		&status,
		&event.AuthSubject,
		&event.Error,
		&event.SourceFile,
		&event.SourceLine,
		&event.SourceFunction,
		&attrsRaw,
		&event.Stack,
		&event.DroppedBefore,
		&event.CreatedAt,
	); err != nil {
		return Event{}, err
	}
	if status.Valid {
		parsed := int(status.Int64)
		event.Status = &parsed
	}
	event.Attrs = attrsFromStorage(attrsRaw)
	return event, nil
}

func buildListQuery(filter ListFilter) (string, []any) {
	conditions := []string{"observed_at >= $1"}
	args := []any{filter.Since}
	addCondition := func(condition string, arg any) {
		args = append(args, arg)
		conditions = append(conditions, strings.Replace(condition, "?", "$"+strconv.Itoa(len(args)), 1))
	}

	if !filter.Before.IsZero() {
		addCondition("observed_at < ?", filter.Before)
	}
	if filter.Level != "" {
		addCondition("level = ?", strings.ToUpper(filter.Level))
	}
	if filter.Component != "" {
		addCondition("component = ?", filter.Component)
	}
	if filter.Operation != "" {
		addCondition("operation = ?", filter.Operation)
	}
	if filter.RequestID != "" {
		addCondition("request_id = ?", filter.RequestID)
	}
	if filter.Path != "" {
		addCondition("path ILIKE ?", "%"+filter.Path+"%")
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	args = append(args, limit)
	query := `
		SELECT
			id,
			observed_at,
			level,
			message,
			component,
			operation,
			request_id,
			method,
			path,
			status,
			auth_subject,
			error,
			source_file,
			source_line,
			source_function,
			attrs,
			stack,
			dropped_before,
			created_at
		FROM mrsmith.diagnostic_event
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY observed_at DESC, id DESC
		LIMIT $` + strconv.Itoa(len(args))
	return query, args
}

func rollbackTx(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}
