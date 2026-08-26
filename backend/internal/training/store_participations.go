package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

func (s *SQLStore) AssignEnrollmentSession(ctx context.Context, principal Principal, enrollmentID, sessionID string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	enrollmentID = strings.TrimSpace(enrollmentID)
	sessionID = strings.TrimSpace(sessionID)
	if enrollmentID == "" || sessionID == "" {
		return ActionResponse{}, validationError("missing_id", "id iscrizione e id sessione obbligatori")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		// Verifica e prenotazione della capienza sono atomiche: lock FOR
		// UPDATE sulla riga della sessione, sempre prima del lock
		// sull'iscrizione (stesso ordine in tutte le operazioni).
		var sessionEventID string
		var maxCapacity sql.NullInt64
		var eventCancelled bool
		err := tx.QueryRowContext(ctx, `
SELECT s.event_id::text, s.max_capacity, ev.cancelled_at IS NOT NULL
FROM training.training_session s
JOIN training.training_event ev ON ev.id = s.event_id
WHERE s.id = $1::uuid
FOR UPDATE OF s`, sessionID).Scan(&sessionEventID, &maxCapacity, &eventCancelled)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("session_not_found", "sessione non trovata")
		}
		if err != nil {
			return fmt.Errorf("load training session for assignment: %w", err)
		}
		if eventCancelled {
			return conflictError("event_cancelled", "un evento annullato non accetta nuove assegnazioni")
		}

		var enrollmentEventID, deliveryStatus, employeeStatus string
		err = tx.QueryRowContext(ctx, `
SELECT en.event_id::text, en.delivery_status, e.status::text
FROM training.enrollment en
JOIN training.employee e ON e.id = en.employee_id
WHERE en.id = $1::uuid
FOR UPDATE OF en`, enrollmentID).Scan(&enrollmentEventID, &deliveryStatus, &employeeStatus)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("enrollment_not_found", "iscrizione non trovata")
		}
		if err != nil {
			return fmt.Errorf("load training enrollment for assignment: %w", err)
		}
		if enrollmentEventID != sessionEventID {
			return validationError("different_event", "iscrizione e sessione devono appartenere allo stesso evento")
		}
		if deliveryStatus == deliveryCancelled {
			return conflictError("enrollment_cancelled", "l'iscrizione annullata non accetta nuove assegnazioni finche non viene riaperta")
		}
		if deliveryStatus != deliveryPlanned && deliveryStatus != deliveryInProgress {
			return conflictError("enrollment_not_assignable", "si assegnano solo iscrizioni pianificate o in corso")
		}
		if employeeStatus != "active" {
			return validationError("person_not_active", "la persona non e attiva")
		}

		if maxCapacity.Valid {
			occupancy, err := s.sessionOccupancy(ctx, tx, sessionID)
			if err != nil {
				return err
			}
			if int64(occupancy) >= maxCapacity.Int64 {
				return conflictError(
					"session_full",
					fmt.Sprintf("capienza %d raggiunta (occupazione corrente %d)", maxCapacity.Int64, occupancy),
				)
			}
		}

		actorID := s.actorEmployeeID(ctx, tx, principal)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO training.enrollment_session (enrollment_id, session_id, assigned_by)
VALUES ($1::uuid, $2::uuid, $3::uuid)`, enrollmentID, sessionID, nullableUUIDPtr(actorID)); err != nil {
			if isUniqueViolation(err, "") {
				return conflictError("already_assigned", "l'iscrizione e gia assegnata alla sessione")
			}
			return fmt.Errorf("assign training enrollment session: %w", err)
		}
		after, err := enrollmentSessionSnapshot(ctx, tx, enrollmentID, sessionID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "enrollment_session", enrollmentID, "assign", nil, after); err != nil {
			return err
		}
		status, err := s.reconcileDeliveryStatus(ctx, tx, principal, enrollmentID, nil)
		if err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: enrollmentID, Status: status}
		return nil
	})
	return response, err
}

func (s *SQLStore) RemoveEnrollmentSession(ctx context.Context, principal Principal, enrollmentID, sessionID string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	enrollmentID = strings.TrimSpace(enrollmentID)
	sessionID = strings.TrimSpace(sessionID)
	if enrollmentID == "" || sessionID == "" {
		return ActionResponse{}, validationError("missing_id", "id iscrizione e id sessione obbligatori")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		deliveryStatus, err := s.lockEnrollmentDeliveryStatus(ctx, tx, enrollmentID)
		if err != nil {
			return err
		}
		if deliveryStatus == deliveryCancelled {
			return conflictError("enrollment_cancelled", "le assegnazioni di un'iscrizione annullata restano nello storico")
		}
		participationStatus, err := s.participationStatus(ctx, tx, enrollmentID, sessionID)
		if err != nil {
			return err
		}
		if participationStatus != participationAssigned {
			return conflictError("participation_started", "una presenza avviata o registrata si corregge, non si cancella")
		}
		before, err := enrollmentSessionSnapshot(ctx, tx, enrollmentID, sessionID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
DELETE FROM training.enrollment_session
WHERE enrollment_id = $1::uuid AND session_id = $2::uuid`, enrollmentID, sessionID); err != nil {
			return fmt.Errorf("remove training enrollment session: %w", err)
		}
		if err := s.audit(ctx, tx, principal, "enrollment_session", enrollmentID, "remove", before, nil); err != nil {
			return err
		}
		status, err := s.reconcileDeliveryStatus(ctx, tx, principal, enrollmentID, nil)
		if err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: enrollmentID, Status: status}
		return nil
	})
	return response, err
}

// UpdateParticipationStatus corregge liberamente la presenza, anche
// all'indietro e tra stati terminali, senza motivazione obbligatoria.
// Ogni modifica e auditata e fa ricalcolare lo stato dell'iscrizione.
func (s *SQLStore) UpdateParticipationStatus(ctx context.Context, principal Principal, enrollmentID, sessionID string, input ParticipationInput, startGate enrollmentStartGate) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	enrollmentID = strings.TrimSpace(enrollmentID)
	sessionID = strings.TrimSpace(sessionID)
	if enrollmentID == "" || sessionID == "" {
		return ActionResponse{}, validationError("missing_id", "id iscrizione e id sessione obbligatori")
	}
	newStatus := strings.TrimSpace(input.ParticipationStatus)
	if !validParticipationStatus(newStatus) {
		return ActionResponse{}, validationError("invalid_participation_status", "stato di presenza non valido")
	}
	if startGate == nil {
		return ActionResponse{}, fmt.Errorf("update participation status requires enrollment start gate")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		// Serialize the start gate with every event-expense mutation before
		// taking the enrollment lock. The preliminary read deliberately does
		// not lock the enrollment: the event row is this operation's aggregate
		// lock and establishes the common lock order.
		var eventID string
		err := tx.QueryRowContext(ctx, `
SELECT event_id::text
FROM training.enrollment
WHERE id = $1::uuid`, enrollmentID).Scan(&eventID)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("enrollment_not_found", "iscrizione non trovata")
		}
		if err != nil {
			return fmt.Errorf("load training enrollment event for participation update: %w", err)
		}
		if err := ensureEventExists(ctx, tx, eventID); err != nil {
			return err
		}

		deliveryStatus, err := s.lockEnrollmentDeliveryStatus(ctx, tx, enrollmentID)
		if err != nil {
			return err
		}
		if deliveryStatus == deliveryCancelled {
			return conflictError("enrollment_cancelled", "l'iscrizione annullata non accetta modifiche alle presenze finche non viene riaperta")
		}
		before, err := enrollmentSessionSnapshot(ctx, tx, enrollmentID, sessionID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE training.enrollment_session
SET participation_status = $3,
    updated_at = now()
WHERE enrollment_id = $1::uuid AND session_id = $2::uuid`, enrollmentID, sessionID, newStatus); err != nil {
			return fmt.Errorf("update training participation status: %w", err)
		}
		after, err := enrollmentSessionSnapshot(ctx, tx, enrollmentID, sessionID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "enrollment_session", enrollmentID, "update_participation", before, after); err != nil {
			return err
		}
		status, err := s.reconcileDeliveryStatus(ctx, tx, principal, enrollmentID, startGate)
		if err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: enrollmentID, Status: status}
		return nil
	})
	return response, err
}

// enrollmentStartGate verifies the live economic coverage only for the PATCH
// participation path. Assign/remove callers deliberately pass nil.
type enrollmentStartGate func(ctx context.Context, purchaseOrderIDs []int64) error

// reconcileDeliveryStatus rilegge le partecipazioni dell'iscrizione e salva
// lo stato calcolato nella stessa transazione della modifica. La riga
// dell'iscrizione deve essere gia bloccata (FOR UPDATE) dal chiamante.
func (s *SQLStore) reconcileDeliveryStatus(ctx context.Context, tx *sql.Tx, principal Principal, enrollmentID string, startGate enrollmentStartGate) (string, error) {
	var current string
	if err := tx.QueryRowContext(ctx, `
SELECT delivery_status
FROM training.enrollment
WHERE id = $1::uuid`, enrollmentID).Scan(&current); err != nil {
		return "", fmt.Errorf("load training enrollment delivery status: %w", err)
	}
	if current == deliveryCancelled {
		// Gli stati decisi non si ricalcolano: le mutazioni sono bloccate a
		// monte, arrivare qui e un errore di programmazione.
		return "", fmt.Errorf("reconcile delivery status called on cancelled enrollment %s", enrollmentID)
	}
	statuses, err := s.participationStatuses(ctx, tx, enrollmentID)
	if err != nil {
		return "", err
	}
	computed := computeDeliveryStatus(statuses)
	if computed == current {
		return current, nil
	}
	if startGate != nil && requiresEnrollmentStartGate(current, computed) {
		purchaseOrderIDs, err := s.enrollmentPurchaseOrderIDs(ctx, tx, enrollmentID)
		if err != nil {
			return "", err
		}
		if err := startGate(ctx, purchaseOrderIDs); err != nil {
			return "", err
		}
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE training.enrollment
SET delivery_status = $2,
    updated_at = now()
WHERE id = $1::uuid`, enrollmentID, computed); err != nil {
		return "", fmt.Errorf("save training enrollment delivery status: %w", err)
	}
	beforePayload, err := json.Marshal(map[string]string{"delivery_status": current})
	if err != nil {
		return "", fmt.Errorf("marshal training delivery status audit: %w", err)
	}
	afterPayload, err := json.Marshal(map[string]string{"delivery_status": computed})
	if err != nil {
		return "", fmt.Errorf("marshal training delivery status audit: %w", err)
	}
	if err := s.audit(ctx, tx, principal, "enrollment", enrollmentID, "reconcile_delivery_status", beforePayload, afterPayload); err != nil {
		return "", err
	}
	return computed, nil
}

// requiresEnrollmentStartGate limits economic validation to a real first
// delivery start. In particular, corrections and direct terminal transitions
// remain historical operations and must not revalidate a PO.
func requiresEnrollmentStartGate(current, computed string) bool {
	return current == deliveryPlanned && computed == deliveryInProgress
}

// enrollmentPurchaseOrderIDs returns the distinct POs explicitly covering an
// enrollment. It locks the corresponding expense rows until the caller
// transaction ends, so the gate's validated coverage cannot change before its
// enrollment start commits. DISTINCT cannot be combined with FOR UPDATE in
// PostgreSQL, therefore duplicates are removed below.
func (s *SQLStore) enrollmentPurchaseOrderIDs(ctx context.Context, tx *sql.Tx, enrollmentID string) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT ee.rda_id
FROM training.event_expense_enrollment eee
JOIN training.event_expense ee ON ee.id = eee.expense_id
WHERE eee.enrollment_id = $1::uuid
ORDER BY ee.rda_id
FOR UPDATE OF ee`, enrollmentID)
	if err != nil {
		return nil, fmt.Errorf("list training enrollment purchase orders: %w", err)
	}
	defer rows.Close()

	ids := make([]int64, 0)
	seen := make(map[int64]struct{})
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan training enrollment purchase order: %w", err)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate training enrollment purchase orders: %w", err)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (s *SQLStore) participationStatuses(ctx context.Context, q sqlRunner, enrollmentID string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `
SELECT participation_status
FROM training.enrollment_session
WHERE enrollment_id = $1::uuid`, enrollmentID)
	if err != nil {
		return nil, fmt.Errorf("list training participation statuses: %w", err)
	}
	defer rows.Close()

	statuses := make([]string, 0)
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return nil, fmt.Errorf("scan training participation status: %w", err)
		}
		statuses = append(statuses, status)
	}
	return statuses, rows.Err()
}

func (s *SQLStore) participationStatus(ctx context.Context, q sqlRunner, enrollmentID, sessionID string) (string, error) {
	var status string
	err := q.QueryRowContext(ctx, `
SELECT participation_status
FROM training.enrollment_session
WHERE enrollment_id = $1::uuid AND session_id = $2::uuid`, enrollmentID, sessionID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", notFoundError("participation_not_found", "assegnazione non trovata")
	}
	if err != nil {
		return "", fmt.Errorf("load training participation status: %w", err)
	}
	return status, nil
}

// sessionOccupancy conta l'occupazione corrente della sessione:
// partecipazioni assigned/in_progress di iscrizioni non annullate.
// completed e not_attended non occupano; le partecipazioni di iscrizioni
// annullate restano nello storico e non occupano.
func (s *SQLStore) sessionOccupancy(ctx context.Context, q sqlRunner, sessionID string) (int, error) {
	var occupancy int
	err := q.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM training.enrollment_session es
JOIN training.enrollment en ON en.id = es.enrollment_id
WHERE es.session_id = $1::uuid
  AND es.participation_status IN ('assigned', 'in_progress')
  AND en.delivery_status <> 'cancelled'`, sessionID).Scan(&occupancy)
	if err != nil {
		return 0, fmt.Errorf("count training session occupancy: %w", err)
	}
	return occupancy, nil
}

// enrollmentSessionSnapshot e la variante di entitySnapshot per la chiave
// composita di enrollment_session.
func enrollmentSessionSnapshot(ctx context.Context, q sqlRunner, enrollmentID, sessionID string) (json.RawMessage, error) {
	const query = `
SELECT to_jsonb(row)
FROM (
  SELECT *
  FROM training.enrollment_session
  WHERE enrollment_id = $1::uuid AND session_id = $2::uuid
) row`
	var raw []byte
	err := q.QueryRowContext(ctx, query, enrollmentID, sessionID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFoundError("participation_not_found", "assegnazione non trovata")
	}
	if err != nil {
		return nil, fmt.Errorf("load training enrollment_session snapshot: %w", err)
	}
	return json.RawMessage(raw), nil
}

// snapshotWithMeta aggiunge metadati d'azione (es. la motivazione di una
// riapertura) allo snapshot destinato all'audit.
func snapshotWithMeta(snapshot json.RawMessage, meta map[string]any) (json.RawMessage, error) {
	values := map[string]any{}
	if len(snapshot) > 0 {
		if err := json.Unmarshal(snapshot, &values); err != nil {
			return nil, fmt.Errorf("decode training audit snapshot: %w", err)
		}
	}
	for key, value := range meta {
		values[key] = value
	}
	merged, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode training audit snapshot: %w", err)
	}
	return merged, nil
}

func validParticipationStatus(status string) bool {
	switch status {
	case participationAssigned, participationInProgress, participationCompleted, participationNotAttended:
		return true
	default:
		return false
	}
}
