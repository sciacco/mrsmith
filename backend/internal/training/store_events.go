package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (s *SQLStore) CreateEvent(ctx context.Context, principal Principal, input EventInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if err := validateEventInput(input); err != nil {
		return ActionResponse{}, err
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.ensureCourseExists(ctx, tx, input.CourseID); err != nil {
			return err
		}
		if err := s.ensureVendorExists(ctx, tx, input.VendorID); err != nil {
			return err
		}
		// origin='direct' imposto dal server; i campi sorgente riservati
		// (regola, richiesta, Factorial) non sono mai scrivibili dal client.
		const stmt = `
INSERT INTO training.training_event (
  course_id,
  vendor_id,
  agreed_price,
  agreed_conditions,
  origin,
  notes
) VALUES ($1::uuid, $2::uuid, $3, NULLIF($4, ''), 'direct', NULLIF($5, ''))
RETURNING id::text`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			strings.TrimSpace(input.CourseID),
			nullableUUID(input.VendorID),
			input.AgreedPrice,
			strings.TrimSpace(input.AgreedConditions),
			strings.TrimSpace(input.Notes),
		).Scan(&response.ID); err != nil {
			return fmt.Errorf("create training event: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "training_event", response.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_event", response.ID, "create", nil, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) UpdateEvent(ctx context.Context, principal Principal, id string, input EventInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id evento obbligatorio")
	}
	if err := validateEventInput(input); err != nil {
		return ActionResponse{}, err
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var (
			currentCourseID string
			origin          string
			hasSource       bool
		)
		err := tx.QueryRowContext(ctx, `
SELECT course_id::text, origin, (source_rule_id IS NOT NULL OR source_request_id IS NOT NULL)
FROM training.training_event
WHERE id = $1::uuid
FOR UPDATE`, id).Scan(&currentCourseID, &origin, &hasSource)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("event_not_found", "evento non trovato")
		}
		if err != nil {
			return fmt.Errorf("load training event for update: %w", err)
		}
		before, err := entitySnapshot(ctx, tx, "training_event", id)
		if err != nil {
			return err
		}

		newCourseID := strings.TrimSpace(input.CourseID)
		if newCourseID != currentCourseID {
			// Il corso di un evento nato da una regola o da una richiesta
			// accolta e un invariante (D7/D8): la tornata porta il corso
			// della regola, l'evento dell'accoglimento il corso accettato.
			// Senza questo blocco la copertura della regola e la coerenza
			// della richiesta chiusa si corromperebbero con un solo PUT.
			if origin == "rule" || hasSource {
				return conflictError("event_course_bound_to_source", "il corso non si puo cambiare: l'evento nasce da una regola o da una richiesta accolta")
			}
			// Il corso dell'evento e modificabile finche l'evento non ha
			// sessioni e nessuna iscrizione e in stato terminale di
			// erogazione; le cancelled non bloccano.
			var hasSessions, hasTerminal bool
			if err := tx.QueryRowContext(ctx, `
SELECT
  EXISTS (SELECT 1 FROM training.training_session s WHERE s.event_id = $1::uuid),
  EXISTS (
    SELECT 1 FROM training.enrollment en
    WHERE en.event_id = $1::uuid
      AND en.delivery_status IN ('completed', 'partially_completed', 'not_attended')
  )`, id).Scan(&hasSessions, &hasTerminal); err != nil {
				return fmt.Errorf("check training event course change: %w", err)
			}
			if hasSessions {
				return conflictError("event_has_sessions", "il corso non si puo cambiare: l'evento ha gia sessioni")
			}
			if hasTerminal {
				return conflictError("event_has_terminal_enrollments", "il corso non si puo cambiare: esistono iscrizioni con erogazione terminata")
			}
			if err := s.ensureCourseExists(ctx, tx, newCourseID); err != nil {
				return err
			}
		}
		if err := s.ensureVendorExists(ctx, tx, input.VendorID); err != nil {
			return err
		}

		const stmt = `
UPDATE training.training_event
SET course_id = $2::uuid,
    vendor_id = $3::uuid,
    agreed_price = $4,
    agreed_conditions = NULLIF($5, ''),
    notes = NULLIF($6, ''),
    updated_at = now()
WHERE id = $1::uuid
RETURNING id::text`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			id,
			newCourseID,
			nullableUUID(input.VendorID),
			input.AgreedPrice,
			strings.TrimSpace(input.AgreedConditions),
			strings.TrimSpace(input.Notes),
		).Scan(&response.ID); err != nil {
			return fmt.Errorf("update training event: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "training_event", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_event", id, "update", before, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) CancelEvent(ctx context.Context, principal Principal, id string, input ReasonInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id evento obbligatorio")
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return ActionResponse{}, validationError("reason_required", "motivazione obbligatoria")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var alreadyCancelled bool
		err := tx.QueryRowContext(ctx, `
SELECT cancelled_at IS NOT NULL
FROM training.training_event
WHERE id = $1::uuid
FOR UPDATE`, id).Scan(&alreadyCancelled)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("event_not_found", "evento non trovato")
		}
		if err != nil {
			return fmt.Errorf("load training event for cancel: %w", err)
		}
		if alreadyCancelled {
			return conflictError("event_already_cancelled", "evento gia annullato")
		}
		before, err := entitySnapshot(ctx, tx, "training_event", id)
		if err != nil {
			return err
		}

		actorID := s.actorEmployeeID(ctx, tx, principal)
		if _, err := tx.ExecContext(ctx, `
UPDATE training.training_event
SET cancelled_at = now(),
    cancelled_by = $2::uuid,
    cancellation_reason = $3,
    updated_at = now()
WHERE id = $1::uuid`, id, nullableUUIDPtr(actorID), reason); err != nil {
			return fmt.Errorf("cancel training event: %w", err)
		}

		// Iscrizioni planned/in_progress -> cancelled con la stessa
		// motivazione; le terminali restano intatte; sessioni e
		// partecipazioni sono conservate.
		const cancelEnrollments = `
WITH candidates AS (
  SELECT en.*
  FROM training.enrollment en
  WHERE en.event_id = $1::uuid
    AND en.delivery_status IN ('planned', 'in_progress')
  FOR UPDATE
), updated AS (
  UPDATE training.enrollment en
  SET delivery_status = 'cancelled',
      cancellation_reason = $2,
      updated_at = now()
  FROM candidates c
  WHERE en.id = c.id
  RETURNING en.*
)
INSERT INTO training.audit_log (
  actor_id,
  entity_type,
  entity_id,
  action,
  before_state,
  after_state,
  correlation_id
)
SELECT $3::uuid, 'enrollment', c.id, 'cancel_by_event', to_jsonb(c), to_jsonb(u), gen_random_uuid()
FROM candidates c
JOIN updated u ON u.id = c.id`
		if _, err := tx.ExecContext(ctx, cancelEnrollments, id, reason, nullableUUIDPtr(actorID)); err != nil {
			return fmt.Errorf("cancel training event enrollments: %w", err)
		}

		after, err := entitySnapshot(ctx, tx, "training_event", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_event", id, "cancel", before, after); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id, Status: "cancelled"}
		return nil
	})
	return response, err
}

func validateEventInput(input EventInput) error {
	if strings.TrimSpace(input.CourseID) == "" {
		return validationError("course_required", "corso obbligatorio")
	}
	if input.AgreedPrice != nil && *input.AgreedPrice < 0 {
		return validationError("invalid_agreed_price", "il prezzo pattuito non puo essere negativo")
	}
	return nil
}

func (s *SQLStore) ensureCourseExists(ctx context.Context, q sqlRunner, courseID string) error {
	var exists bool
	err := q.QueryRowContext(ctx, `
SELECT EXISTS (SELECT 1 FROM training.course WHERE id = $1::uuid)`, strings.TrimSpace(courseID)).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check training course: %w", err)
	}
	if !exists {
		return validationError("course_not_found", "corso non trovato")
	}
	return nil
}

func (s *SQLStore) ensureVendorExists(ctx context.Context, q sqlRunner, vendorID string) error {
	vendorID = strings.TrimSpace(vendorID)
	if vendorID == "" {
		return nil
	}
	var exists bool
	err := q.QueryRowContext(ctx, `
SELECT EXISTS (SELECT 1 FROM training.vendor WHERE id = $1::uuid)`, vendorID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check training vendor: %w", err)
	}
	if !exists {
		return validationError("vendor_not_found", "fornitore non trovato")
	}
	return nil
}
