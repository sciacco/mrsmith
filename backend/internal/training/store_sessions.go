package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *SQLStore) CreateSession(ctx context.Context, principal Principal, eventID string, input SessionInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return ActionResponse{}, validationError("missing_id", "id evento obbligatorio")
	}
	normalized, err := normalizeSessionInput(input)
	if err != nil {
		return ActionResponse{}, err
	}

	var response ActionResponse
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var eventCancelled bool
		err := tx.QueryRowContext(ctx, `
SELECT cancelled_at IS NOT NULL
FROM training.training_event
WHERE id = $1::uuid
FOR UPDATE`, eventID).Scan(&eventCancelled)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("event_not_found", "evento non trovato")
		}
		if err != nil {
			return fmt.Errorf("load training event for session: %w", err)
		}
		if eventCancelled {
			return conflictError("event_cancelled", "un evento annullato non accetta nuove sessioni")
		}

		const stmt = `
INSERT INTO training.training_session (
  event_id,
  schedule_type,
  starts_at,
  ends_at,
  due_at,
  max_capacity,
  notes
) VALUES (
  $1::uuid,
  $2,
  NULLIF($3, '')::timestamptz,
  NULLIF($4, '')::timestamptz,
  NULLIF($5, '')::timestamptz,
  $6,
  NULLIF($7, '')
)
RETURNING id::text`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			eventID,
			normalized.ScheduleType,
			normalized.StartsAt,
			normalized.EndsAt,
			normalized.DueAt,
			normalized.MaxCapacity,
			normalized.Notes,
		).Scan(&response.ID); err != nil {
			return fmt.Errorf("create training session: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "training_session", response.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_session", response.ID, "create", nil, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) UpdateSession(ctx context.Context, principal Principal, id string, input SessionInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id sessione obbligatorio")
	}
	normalized, err := normalizeSessionInput(input)
	if err != nil {
		return ActionResponse{}, err
	}

	var response ActionResponse
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		// Lock della riga sessione: la verifica di capienza usa lo stesso
		// lock dell'assegnazione, cosi i due gesti si serializzano.
		var exists string
		err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM training.training_session
WHERE id = $1::uuid
FOR UPDATE`, id).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("session_not_found", "sessione non trovata")
		}
		if err != nil {
			return fmt.Errorf("load training session for update: %w", err)
		}
		before, err := entitySnapshot(ctx, tx, "training_session", id)
		if err != nil {
			return err
		}

		if normalized.MaxCapacity != nil {
			occupancy, err := s.sessionOccupancy(ctx, tx, id)
			if err != nil {
				return err
			}
			if *normalized.MaxCapacity < occupancy {
				return validationError(
					"capacity_below_occupancy",
					fmt.Sprintf("capienza richiesta %d inferiore all'occupazione corrente %d", *normalized.MaxCapacity, occupancy),
				)
			}
		}

		const stmt = `
UPDATE training.training_session
SET schedule_type = $2,
    starts_at = NULLIF($3, '')::timestamptz,
    ends_at = NULLIF($4, '')::timestamptz,
    due_at = NULLIF($5, '')::timestamptz,
    max_capacity = $6,
    notes = NULLIF($7, ''),
    updated_at = now()
WHERE id = $1::uuid
RETURNING id::text`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			id,
			normalized.ScheduleType,
			normalized.StartsAt,
			normalized.EndsAt,
			normalized.DueAt,
			normalized.MaxCapacity,
			normalized.Notes,
		).Scan(&response.ID); err != nil {
			return fmt.Errorf("update training session: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "training_session", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_session", id, "update", before, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) DeleteSession(ctx context.Context, principal Principal, id string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id sessione obbligatorio")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var exists string
		err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM training.training_session
WHERE id = $1::uuid
FOR UPDATE`, id).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("session_not_found", "sessione non trovata")
		}
		if err != nil {
			return fmt.Errorf("load training session for delete: %w", err)
		}
		before, err := entitySnapshot(ctx, tx, "training_session", id)
		if err != nil {
			return err
		}

		var assignedCount, startedCount int
		if err := tx.QueryRowContext(ctx, `
SELECT
  COUNT(*) FILTER (WHERE participation_status = 'assigned'),
  COUNT(*) FILTER (WHERE participation_status <> 'assigned')
FROM training.enrollment_session
WHERE session_id = $1::uuid`, id).Scan(&assignedCount, &startedCount); err != nil {
			return fmt.Errorf("count training session participations: %w", err)
		}
		if startedCount > 0 {
			return conflictError("session_has_participations", "la sessione ha presenze avviate o registrate: non si puo eliminare")
		}
		if assignedCount > 0 {
			return conflictError("session_has_assignments", "rimuovi prima le assegnazioni della sessione")
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM training.training_session WHERE id = $1::uuid`, id); err != nil {
			return fmt.Errorf("delete training session: %w", err)
		}
		if err := s.audit(ctx, tx, principal, "training_session", id, "delete", before, nil); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id}
		return nil
	})
	return response, err
}

type normalizedSessionInput struct {
	ScheduleType string
	StartsAt     string
	EndsAt       string
	DueAt        string
	MaxCapacity  *int
	Notes        string
}

func normalizeSessionInput(input SessionInput) (normalizedSessionInput, error) {
	normalized := normalizedSessionInput{
		ScheduleType: strings.TrimSpace(input.ScheduleType),
		MaxCapacity:  input.MaxCapacity,
		Notes:        strings.TrimSpace(input.Notes),
	}
	if normalized.ScheduleType != "scheduled" && normalized.ScheduleType != "self_paced" {
		return normalized, validationError("schedule_type_invalid", "tipo sessione obbligatorio: scheduled o self_paced")
	}
	var err error
	if normalized.StartsAt, err = normalizeOptionalTimestamp(input.StartsAt); err != nil {
		return normalized, validationError("invalid_starts_at", "data di inizio non valida (RFC 3339)")
	}
	if normalized.EndsAt, err = normalizeOptionalTimestamp(input.EndsAt); err != nil {
		return normalized, validationError("invalid_ends_at", "data di fine non valida (RFC 3339)")
	}
	if normalized.DueAt, err = normalizeOptionalTimestamp(input.DueAt); err != nil {
		return normalized, validationError("invalid_due_at", "scadenza non valida (RFC 3339)")
	}
	if normalized.StartsAt != "" && normalized.EndsAt != "" {
		starts, _ := time.Parse(time.RFC3339, normalized.StartsAt)
		ends, _ := time.Parse(time.RFC3339, normalized.EndsAt)
		if ends.Before(starts) {
			return normalized, validationError("session_ends_before_starts", "la fine non puo precedere l'inizio")
		}
	}
	if normalized.MaxCapacity != nil && *normalized.MaxCapacity <= 0 {
		return normalized, validationError("invalid_max_capacity", "la capienza deve essere positiva")
	}
	return normalized, nil
}

func normalizeOptionalTimestamp(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if _, err := time.Parse(time.RFC3339, raw); err != nil {
		return "", err
	}
	return raw, nil
}
