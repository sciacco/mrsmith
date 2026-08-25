package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (s *SQLStore) CreateEventEnrollment(ctx context.Context, principal Principal, eventID string, input EnrollmentCreateInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return ActionResponse{}, validationError("missing_id", "id evento obbligatorio")
	}
	employeeID := strings.TrimSpace(input.EmployeeID)
	if employeeID == "" {
		return ActionResponse{}, validationError("employee_required", "persona obbligatoria")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
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
			return fmt.Errorf("load training event for enrollment: %w", err)
		}
		if eventCancelled {
			return conflictError("event_cancelled", "un evento annullato non accetta nuove iscrizioni")
		}
		if err := s.ensureEmployeeActive(ctx, tx, employeeID); err != nil {
			return err
		}

		const stmt = `
INSERT INTO training.enrollment (
  employee_id,
  event_id,
  delivery_status,
  origin,
  objective,
  notes
) VALUES ($1::uuid, $2::uuid, 'planned', 'direct', NULLIF($3, ''), NULLIF($4, ''))
RETURNING id::text, delivery_status`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			employeeID,
			eventID,
			strings.TrimSpace(input.Objective),
			strings.TrimSpace(input.Notes),
		).Scan(&response.ID, &response.Status); err != nil {
			if isUniqueViolation(err, "") {
				return conflictError("already_enrolled", "la persona e gia iscritta all'evento")
			}
			return fmt.Errorf("create training enrollment: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "enrollment", response.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "enrollment", response.ID, "create", nil, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

// UpdateEnrollmentFacts e l'update unico dei fatti senza effetti di stato:
// obiettivo, note, date effettive, ore e esito. I fatti restano modificabili
// anche sull'iscrizione annullata (il consuntivo arriva spesso dopo
// l'annullamento); l'annullamento congela solo i fatti di erogazione.
func (s *SQLStore) UpdateEnrollmentFacts(ctx context.Context, principal Principal, id string, input EnrollmentFactsInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id iscrizione obbligatorio")
	}
	actualStart, err := parseOptionalDate(input.ActualStart)
	if err != nil {
		return ActionResponse{}, validationError("invalid_actual_start", "data inizio effettiva non valida")
	}
	actualEnd, err := parseOptionalDate(input.ActualEnd)
	if err != nil {
		return ActionResponse{}, validationError("invalid_actual_end", "data fine effettiva non valida")
	}
	if actualStart != nil && actualEnd != nil && actualEnd.Before(*actualStart) {
		return ActionResponse{}, validationError("actual_end_before_start", "la fine effettiva non puo precedere l'inizio")
	}
	if input.HoursActual != nil && *input.HoursActual < 0 {
		return ActionResponse{}, validationError("invalid_hours_actual", "le ore effettive non possono essere negative")
	}
	outcome := strings.TrimSpace(input.LearningOutcome)
	if !validLearningOutcome(outcome) {
		return ActionResponse{}, validationError("invalid_learning_outcome", "esito non valido")
	}

	var response ActionResponse
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := s.lockEnrollmentDeliveryStatus(ctx, tx, id); err != nil {
			return err
		}
		before, err := entitySnapshot(ctx, tx, "enrollment", id)
		if err != nil {
			return err
		}
		const stmt = `
UPDATE training.enrollment
SET objective = NULLIF($2, ''),
    notes = NULLIF($3, ''),
    actual_start = NULLIF($4, '')::date,
    actual_end = NULLIF($5, '')::date,
    hours_actual = $6,
    learning_outcome = NULLIF($7, ''),
    updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, delivery_status`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			id,
			strings.TrimSpace(input.Objective),
			strings.TrimSpace(input.Notes),
			strings.TrimSpace(input.ActualStart),
			strings.TrimSpace(input.ActualEnd),
			input.HoursActual,
			outcome,
		).Scan(&response.ID, &response.Status); err != nil {
			return fmt.Errorf("update training enrollment facts: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "enrollment", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "enrollment", id, "update_facts", before, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) CancelEnrollment(ctx context.Context, principal Principal, id string, input ReasonInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id iscrizione obbligatorio")
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return ActionResponse{}, validationError("reason_required", "motivazione obbligatoria")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		status, err := s.lockEnrollmentDeliveryStatus(ctx, tx, id)
		if err != nil {
			return err
		}
		if status != deliveryPlanned && status != deliveryInProgress {
			return conflictError("enrollment_not_cancellable", "si annullano solo iscrizioni pianificate o in corso: i terminali di fatto si correggono dalle presenze")
		}
		before, err := entitySnapshot(ctx, tx, "enrollment", id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE training.enrollment
SET delivery_status = 'cancelled',
    cancellation_reason = $2,
    updated_at = now()
WHERE id = $1::uuid`, id, reason); err != nil {
			return fmt.Errorf("cancel training enrollment: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "enrollment", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "enrollment", id, "cancel", before, after); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id, Status: deliveryCancelled}
		return nil
	})
	return response, err
}

// CompleteEnrollmentHistorical registra una formazione gia avvenuta senza
// tracciamento a sessioni. Ammesso solo su iscrizione planned senza alcuna
// partecipazione: non essendoci partecipazioni, nessun ricalcolo potra mai
// scavalcare lo stato.
func (s *SQLStore) CompleteEnrollmentHistorical(ctx context.Context, principal Principal, id string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id iscrizione obbligatorio")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		status, err := s.lockEnrollmentDeliveryStatus(ctx, tx, id)
		if err != nil {
			return err
		}
		if status != deliveryPlanned {
			return conflictError("historical_requires_planned", "lo storico-completato e ammesso solo su iscrizioni pianificate")
		}
		statuses, err := s.participationStatuses(ctx, tx, id)
		if err != nil {
			return err
		}
		if len(statuses) > 0 {
			return conflictError("historical_requires_no_participations", "lo storico-completato e ammesso solo senza partecipazioni: registra le presenze nelle sessioni")
		}
		before, err := entitySnapshot(ctx, tx, "enrollment", id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE training.enrollment
SET delivery_status = 'completed',
    updated_at = now()
WHERE id = $1::uuid`, id); err != nil {
			return fmt.Errorf("complete training enrollment historically: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "enrollment", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "enrollment", id, "complete_historical", before, after); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id, Status: deliveryCompleted}
		return nil
	})
	return response, err
}

// ReopenEnrollment esiste solo per gli stati «decisi»: cancelled e
// completato-da-storico (completed senza partecipazioni). Riporta
// l'iscrizione allo stato ricalcolato dalle partecipazioni presenti
// (senza partecipazioni: planned). Vietata se l'evento e annullato.
func (s *SQLStore) ReopenEnrollment(ctx context.Context, principal Principal, id string, input ReasonInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id iscrizione obbligatorio")
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return ActionResponse{}, validationError("reason_required", "motivazione obbligatoria")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		// Lock nell'ordine evento -> iscrizione, lo stesso di CancelEvent:
		// serializza la riapertura con l'annullamento dell'evento (senza il
		// lock sull'evento un annullamento concorrente salterebbe questa
		// iscrizione, ancora cancelled, e la riapertura la riattiverebbe su
		// un evento ormai annullato). event_id e immutabile, quindi leggerlo
		// senza lock prima di bloccare l'evento e sicuro.
		var eventID string
		err := tx.QueryRowContext(ctx, `
SELECT event_id::text
FROM training.enrollment
WHERE id = $1::uuid`, id).Scan(&eventID)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("enrollment_not_found", "iscrizione non trovata")
		}
		if err != nil {
			return fmt.Errorf("load training enrollment for reopen: %w", err)
		}
		var eventCancelled bool
		if err := tx.QueryRowContext(ctx, `
SELECT cancelled_at IS NOT NULL
FROM training.training_event
WHERE id = $1::uuid
FOR UPDATE`, eventID).Scan(&eventCancelled); err != nil {
			return fmt.Errorf("load training event for reopen: %w", err)
		}
		if eventCancelled {
			return conflictError("event_cancelled", "un evento annullato non accetta riaperture di iscrizioni")
		}
		status, err := s.lockEnrollmentDeliveryStatus(ctx, tx, id)
		if err != nil {
			return err
		}
		statuses, err := s.participationStatuses(ctx, tx, id)
		if err != nil {
			return err
		}
		decided := status == deliveryCancelled || (status == deliveryCompleted && len(statuses) == 0)
		if !decided {
			return conflictError("enrollment_not_reopenable", "si riaprono solo iscrizioni annullate o completate da storico: gli stati di fatto si correggono dalle presenze")
		}
		before, err := entitySnapshot(ctx, tx, "enrollment", id)
		if err != nil {
			return err
		}
		computed := computeDeliveryStatus(statuses)
		if _, err := tx.ExecContext(ctx, `
UPDATE training.enrollment
SET delivery_status = $2,
    cancellation_reason = NULL,
    updated_at = now()
WHERE id = $1::uuid`, id, computed); err != nil {
			return fmt.Errorf("reopen training enrollment: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "enrollment", id)
		if err != nil {
			return err
		}
		// La motivazione della riapertura non ha una colonna: viaggia
		// nell'audit insieme allo snapshot.
		afterWithReason, err := snapshotWithMeta(after, map[string]any{"reopen_reason": reason})
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "enrollment", id, "reopen", before, afterWithReason); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id, Status: computed}
		return nil
	})
	return response, err
}

func (s *SQLStore) lockEnrollmentDeliveryStatus(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	var status string
	err := tx.QueryRowContext(ctx, `
SELECT delivery_status
FROM training.enrollment
WHERE id = $1::uuid
FOR UPDATE`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", notFoundError("enrollment_not_found", "iscrizione non trovata")
	}
	if err != nil {
		return "", fmt.Errorf("load training enrollment: %w", err)
	}
	return status, nil
}

func (s *SQLStore) ensureEmployeeActive(ctx context.Context, q sqlRunner, employeeID string) error {
	var status string
	err := q.QueryRowContext(ctx, `
SELECT status::text
FROM training.employee
WHERE id = $1::uuid`, employeeID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundError("employee_not_found", "persona non trovata")
	}
	if err != nil {
		return fmt.Errorf("load training employee status: %w", err)
	}
	if status != "active" {
		return validationError("person_not_active", "la persona non e attiva")
	}
	return nil
}

func validLearningOutcome(outcome string) bool {
	switch outcome {
	case "", "passed", "failed", "not_taken", "not_required":
		return true
	default:
		return false
	}
}
