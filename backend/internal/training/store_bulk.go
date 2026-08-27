package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ── Assegnazioni massive (evento -> sessioni) ──

// bulkTargetSession e una sessione bersaglio con la capienza gia letta sotto
// lock: nil = illimitata.
type bulkTargetSession struct {
	ID          string
	MaxCapacity *int64
	Occupancy   int
}

// BulkAssignSessions (#153): una sola transazione, stesso lock FOR UPDATE e
// stesse regole di eleggibilita della singola assegnazione (AssignEnrollmentSession).
func (s *SQLStore) BulkAssignSessions(ctx context.Context, principal Principal, eventID string, input BulkAssignmentsInput) (BulkAssignmentsResponse, error) {
	if !principal.IsPeopleAdmin {
		return BulkAssignmentsResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return BulkAssignmentsResponse{}, validationError("missing_id", "id evento obbligatorio")
	}
	mode := strings.TrimSpace(input.Mode)
	if mode != "all_to_all" && mode != "distribute" && mode != "fill_session" {
		return BulkAssignmentsResponse{}, validationError("invalid_mode", "modalita non valida")
	}
	sessionID := strings.TrimSpace(input.SessionID)
	if mode == "fill_session" && sessionID == "" {
		return BulkAssignmentsResponse{}, validationError("session_id_required", "sessionId obbligatorio per fill_session")
	}

	response := BulkAssignmentsResponse{PerSession: map[string]int{}}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := lockEventNotCancelled(ctx, tx, eventID); err != nil {
			return err
		}
		requestedSessionIDs := input.SessionIDs
		if mode == "fill_session" {
			requestedSessionIDs = []string{sessionID}
		}
		sessions, err := s.lockBulkTargetSessions(ctx, tx, eventID, requestedSessionIDs)
		if err != nil {
			return err
		}
		enrollmentIDs, err := s.lockEligibleEnrollments(ctx, tx, eventID, mode != "all_to_all")
		if err != nil {
			return err
		}
		if mode == "all_to_all" {
			return s.bulkAssignAllToAll(ctx, tx, principal, sessions, enrollmentIDs, &response)
		}
		return s.bulkAssignDistribute(ctx, tx, principal, sessions, enrollmentIDs, &response)
	})
	if err != nil {
		return BulkAssignmentsResponse{}, err
	}
	response.OK = true
	return response, nil
}

// lockEventNotCancelled: stesso controllo di CreateEventEnrollment/CreateSession.
func lockEventNotCancelled(ctx context.Context, tx *sql.Tx, eventID string) error {
	var cancelled bool
	err := tx.QueryRowContext(ctx, `
SELECT cancelled_at IS NOT NULL FROM training.training_event
WHERE id = $1::uuid FOR UPDATE`, eventID).Scan(&cancelled)
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundError("event_not_found", "evento non trovato")
	}
	if err != nil {
		return fmt.Errorf("load training event for bulk operation: %w", err)
	}
	if cancelled {
		return conflictError("event_cancelled", "un evento annullato non accetta nuove iscrizioni o assegnazioni")
	}
	return nil
}

// lockBulkTargetSessions blocca tutte le sessioni dell'evento (ordine
// cablato: starts_at, poi due_at, poi id) e ne restituisce il sottoinsieme
// richiesto, nello stesso ordine. requested vuoto = tutte.
func (s *SQLStore) lockBulkTargetSessions(ctx context.Context, tx *sql.Tx, eventID string, requested []string) ([]bulkTargetSession, error) {
	want := make(map[string]bool, len(requested))
	for _, raw := range requested {
		if id := strings.TrimSpace(raw); id != "" {
			want[id] = true
		}
	}
	rows, err := tx.QueryContext(ctx, `
SELECT id::text, max_capacity
FROM training.training_session
WHERE event_id = $1::uuid
ORDER BY starts_at NULLS LAST, due_at NULLS LAST, id
FOR UPDATE`, eventID)
	if err != nil {
		return nil, fmt.Errorf("list training sessions for bulk assignment: %w", err)
	}
	defer rows.Close()
	selected := make([]bulkTargetSession, 0)
	found := 0
	for rows.Next() {
		var id string
		var maxCapacity sql.NullInt64
		if err := rows.Scan(&id, &maxCapacity); err != nil {
			return nil, fmt.Errorf("scan training session for bulk assignment: %w", err)
		}
		if len(want) > 0 {
			if !want[id] {
				continue
			}
			found++
		}
		session := bulkTargetSession{ID: id}
		if maxCapacity.Valid {
			v := maxCapacity.Int64
			session.MaxCapacity = &v
		}
		selected = append(selected, session)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if found != len(want) {
		return nil, validationError("session_not_in_event", "una o piu sessioni non appartengono all'evento")
	}
	if len(selected) == 0 {
		return nil, validationError("session_required", "almeno una sessione richiesta")
	}
	for i := range selected {
		occupancy, err := s.sessionOccupancy(ctx, tx, selected[i].ID)
		if err != nil {
			return nil, err
		}
		selected[i].Occupancy = occupancy
	}
	return selected, nil
}

// lockEligibleEnrollments blocca e ordina (created_at, poi id) le iscrizioni
// eleggibili: non annullate, di persone attive, in planned|in_progress (le
// regole della singola assegnazione). onlyUnassigned = nessuna assegnazione
// gia presente (distribute/fill_session).
func (s *SQLStore) lockEligibleEnrollments(ctx context.Context, tx *sql.Tx, eventID string, onlyUnassigned bool) ([]string, error) {
	query := `
SELECT en.id::text
FROM training.enrollment en
JOIN training.employee e ON e.id = en.employee_id
WHERE en.event_id = $1::uuid
  AND en.delivery_status IN ('planned', 'in_progress')
  AND e.status = 'active'`
	if onlyUnassigned {
		query += `
  AND NOT EXISTS (SELECT 1 FROM training.enrollment_session es WHERE es.enrollment_id = en.id)`
	}
	query += `
ORDER BY en.created_at, en.id
FOR UPDATE OF en`
	rows, err := tx.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("list training eligible enrollments for bulk assignment: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan training eligible enrollment for bulk assignment: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// bulkAssignAllToAll: ogni iscrizione eleggibile su ogni sessione indicata;
// le coppie gia esistenti sono saltate senza errore. Capienza insufficiente
// su una sessione rifiuta l'intera chiamata con i numeri.
func (s *SQLStore) bulkAssignAllToAll(ctx context.Context, tx *sql.Tx, principal Principal, sessions []bulkTargetSession, enrollmentIDs []string, response *BulkAssignmentsResponse) error {
	for _, session := range sessions {
		existing, err := s.existingSessionEnrollments(ctx, tx, session.ID)
		if err != nil {
			return err
		}
		pending := make([]string, 0, len(enrollmentIDs))
		for _, id := range enrollmentIDs {
			if !existing[id] {
				pending = append(pending, id)
			}
		}
		if session.MaxCapacity != nil {
			available := *session.MaxCapacity - int64(session.Occupancy)
			if available < 0 {
				available = 0
			}
			if int64(len(pending)) > available {
				return validationError("session_capacity_insufficient", fmt.Sprintf(
					"sessione %s: richiesti %d, disponibili %d", session.ID, len(pending), available))
			}
		}
		for _, enrollmentID := range pending {
			if err := s.insertBulkAssignment(ctx, tx, principal, enrollmentID, session.ID); err != nil {
				return err
			}
			response.Assigned++
			response.PerSession[session.ID]++
		}
	}
	return nil
}

// bulkAssignDistribute riempie le sessioni nell'ordine cablato con le
// iscrizioni senza alcuna assegnazione, fino a esaurimento della capienza
// residua (-1 = illimitata). Copre anche fill_session (una sola sessione:
// stesso algoritmo). Posti complessivi insufficienti rifiutano l'intera
// chiamata: il fallimento arriva prima di qualunque scrittura per quella
// iscrizione e il rollback della transazione annulla le precedenti.
func (s *SQLStore) bulkAssignDistribute(ctx context.Context, tx *sql.Tx, principal Principal, sessions []bulkTargetSession, enrollmentIDs []string, response *BulkAssignmentsResponse) error {
	remaining := make([]int64, len(sessions))
	var totalAvailable int64
	for i, session := range sessions {
		if session.MaxCapacity == nil {
			remaining[i] = -1
			continue
		}
		available := *session.MaxCapacity - int64(session.Occupancy)
		if available < 0 {
			available = 0
		}
		remaining[i] = available
		totalAvailable += available
	}

	sessionIndex := 0
	for _, enrollmentID := range enrollmentIDs {
		for sessionIndex < len(sessions) && remaining[sessionIndex] == 0 {
			sessionIndex++
		}
		if sessionIndex >= len(sessions) {
			return validationError("bulk_capacity_insufficient", fmt.Sprintf("richiesti %d, disponibili %d", len(enrollmentIDs), totalAvailable))
		}
		session := sessions[sessionIndex]
		if err := s.insertBulkAssignment(ctx, tx, principal, enrollmentID, session.ID); err != nil {
			return err
		}
		if remaining[sessionIndex] > 0 {
			remaining[sessionIndex]--
		}
		response.Assigned++
		response.PerSession[session.ID]++
	}
	return nil
}

func (s *SQLStore) existingSessionEnrollments(ctx context.Context, tx *sql.Tx, sessionID string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT enrollment_id::text FROM training.enrollment_session
WHERE session_id = $1::uuid`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list training session assignments for bulk all-to-all: %w", err)
	}
	defer rows.Close()
	existing := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan training session assignment for bulk all-to-all: %w", err)
		}
		existing[id] = true
	}
	return existing, rows.Err()
}

// insertBulkAssignment e il nucleo scrittura+audit+ricalcolo di
// AssignEnrollmentSession, riusato senza il suo lock/validazione (gia fatti
// a monte dal chiamante bulk, stesso ordine sessione->iscrizione).
func (s *SQLStore) insertBulkAssignment(ctx context.Context, tx *sql.Tx, principal Principal, enrollmentID, sessionID string) error {
	actorID := s.actorEmployeeID(ctx, tx, principal)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO training.enrollment_session (enrollment_id, session_id, assigned_by)
VALUES ($1::uuid, $2::uuid, $3::uuid)`, enrollmentID, sessionID, nullableUUIDPtr(actorID)); err != nil {
		return fmt.Errorf("bulk assign training enrollment session: %w", err)
	}
	after, err := enrollmentSessionSnapshot(ctx, tx, enrollmentID, sessionID)
	if err != nil {
		return err
	}
	if err := s.audit(ctx, tx, principal, "enrollment_session", enrollmentID, "assign", nil, after); err != nil {
		return err
	}
	_, err = s.reconcileDeliveryStatus(ctx, tx, principal, enrollmentID, nil)
	return err
}

// ── Presenze massive (per sessione) ──

// BulkUpdateParticipation applica lo stesso stato a piu coppie
// iscrizione/sessione nella stessa transazione, ognuna attraverso il nucleo
// di UpdateParticipationStatus (scrittura, audit, ricalcolo delivery_status
// col gate economico #142, confinato alle transizioni reali planned ->
// in_progress). Il gate non fa fail-fast: un blocco rifiuta l'intera
// chiamata (rollback), ma solo dopo aver valutato tutte le iscrizioni, cosi
// il 409 elenca tutti i PO bloccanti e tutte le iscrizioni interessate.
func (s *SQLStore) BulkUpdateParticipation(ctx context.Context, principal Principal, sessionID string, input BulkParticipationInput, startGate enrollmentStartGate) (BulkParticipationResponse, error) {
	if !principal.IsPeopleAdmin {
		return BulkParticipationResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return BulkParticipationResponse{}, validationError("missing_id", "id sessione obbligatorio")
	}
	newStatus := strings.TrimSpace(input.ParticipationStatus)
	if !validParticipationStatus(newStatus) {
		return BulkParticipationResponse{}, validationError("invalid_participation_status", "stato di presenza non valido")
	}
	if startGate == nil {
		return BulkParticipationResponse{}, fmt.Errorf("bulk update participation status requires enrollment start gate")
	}

	response := BulkParticipationResponse{}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		// Lettura preliminare senza lock: il lock aggregato dell'operazione e
		// quello sulla riga evento (via ensureEventExists), che stabilisce
		// l'ordine comune evento->sessione->iscrizione, lo stesso di
		// UpdateParticipationStatus e BulkAssignSessions.
		var eventID string
		err := tx.QueryRowContext(ctx, `
SELECT event_id::text FROM training.training_session
WHERE id = $1::uuid`, sessionID).Scan(&eventID)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("session_not_found", "sessione non trovata")
		}
		if err != nil {
			return fmt.Errorf("load training session for bulk participation: %w", err)
		}
		if err := ensureEventExists(ctx, tx, eventID); err != nil {
			return err
		}
		enrollmentIDs, err := s.bulkParticipationTargets(ctx, tx, sessionID, input.EnrollmentIDs)
		if err != nil {
			return err
		}
		// Il gate economico #142 non fa fail-fast: ogni blocco 409 viene
		// accumulato (iscrizione + PO bloccanti dedotti) e il ciclo prosegue
		// sulle iscrizioni successive, perche la chiamata Arak per PO e gia
		// pagata dal percorso felice (una per iscrizione planned->in_progress)
		// e il 409 deve elencare tutti i blocchi, non solo il primo. Gli errori
		// tecnici o non-409 del gate (503/404/errori) interrompono subito come
		// nel percorso singolo.
		blockedEnrollmentIDs := make([]string, 0)
		seenBlockers := make(map[string]bool)
		blockers := make([]string, 0)
		for _, enrollmentID := range enrollmentIDs {
			if err := s.applyBulkParticipation(ctx, tx, principal, enrollmentID, sessionID, newStatus, startGate); err != nil {
				appErr, ok := asAppError(err)
				if !ok || appErr.status != 409 || appErr.code != "event_expense_po_not_approved" {
					return err
				}
				blockedEnrollmentIDs = append(blockedEnrollmentIDs, enrollmentID)
				for _, part := range strings.Split(strings.TrimPrefix(appErr.message, "avvio iscrizione bloccato: "), "; ") {
					if part == "" || seenBlockers[part] {
						continue
					}
					seenBlockers[part] = true
					blockers = append(blockers, part)
				}
				continue
			}
			response.Updated++
		}
		if len(blockedEnrollmentIDs) > 0 {
			return conflictError("event_expense_po_not_approved", fmt.Sprintf(
				"avvio bloccato per %d iscrizioni (%s): %s",
				len(blockedEnrollmentIDs), strings.Join(blockedEnrollmentIDs, ", "), strings.Join(blockers, "; ")))
		}
		response.OK = true
		return nil
	})
	if err != nil {
		return BulkParticipationResponse{}, err
	}
	return response, nil
}

// applyBulkParticipation e il nucleo di UpdateParticipationStatus (lock,
// scrittura, audit, ricalcolo col gate) per una coppia, senza riaprire una
// transazione propria.
func (s *SQLStore) applyBulkParticipation(ctx context.Context, tx *sql.Tx, principal Principal, enrollmentID, sessionID, newStatus string, startGate enrollmentStartGate) error {
	deliveryStatus, err := s.lockEnrollmentDeliveryStatus(ctx, tx, enrollmentID)
	if err != nil {
		return err
	}
	if deliveryStatus == deliveryCancelled {
		return conflictError("enrollment_cancelled", fmt.Sprintf(
			"iscrizione %s annullata: non accetta modifiche alle presenze finche non viene riaperta", enrollmentID))
	}
	before, err := enrollmentSessionSnapshot(ctx, tx, enrollmentID, sessionID)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE training.enrollment_session
SET participation_status = $3, updated_at = now()
WHERE enrollment_id = $1::uuid AND session_id = $2::uuid`, enrollmentID, sessionID, newStatus); err != nil {
		return fmt.Errorf("bulk update training participation status: %w", err)
	}
	after, err := enrollmentSessionSnapshot(ctx, tx, enrollmentID, sessionID)
	if err != nil {
		return err
	}
	if err := s.audit(ctx, tx, principal, "enrollment_session", enrollmentID, "update_participation", before, after); err != nil {
		return err
	}
	// L'errore del gate (409 event_expense_po_not_approved incluso) risale
	// invariato: e BulkUpdateParticipation a decidere se accumularlo e
	// proseguire o interrompere subito.
	_, err = s.reconcileDeliveryStatus(ctx, tx, principal, enrollmentID, startGate)
	return err
}

// bulkParticipationTargets legge, in un'unica interrogazione, tutte le
// relazioni della sessione (created_at, poi id: ordine cablato) con lo stato
// dell'iscrizione. Senza enrollmentIds: solo le non annullate. Con
// enrollmentIds: verifica che ciascuna appartenga alla sessione (422
// altrimenti); una cancellata resta nell'insieme e si blochera piu avanti
// come le altre correzioni sull'iscrizione annullata.
func (s *SQLStore) bulkParticipationTargets(ctx context.Context, tx *sql.Tx, sessionID string, requested []string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT es.enrollment_id::text, en.delivery_status <> 'cancelled'
FROM training.enrollment_session es
JOIN training.enrollment en ON en.id = es.enrollment_id
WHERE es.session_id = $1::uuid
ORDER BY en.created_at, en.id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list training session participations for bulk update: %w", err)
	}
	defer rows.Close()
	type row struct {
		id     string
		active bool
	}
	all := make([]row, 0)
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.active); err != nil {
			return nil, fmt.Errorf("scan training session participation for bulk update: %w", err)
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(requested) == 0 {
		ids := make([]string, 0, len(all))
		for _, r := range all {
			if r.active {
				ids = append(ids, r.id)
			}
		}
		return ids, nil
	}
	present := make(map[string]bool, len(all))
	for _, r := range all {
		present[r.id] = true
	}
	want := make(map[string]bool, len(requested))
	for _, raw := range requested {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if !present[id] {
			return nil, validationError("enrollment_not_in_session", fmt.Sprintf("iscrizione %s non assegnata alla sessione", id))
		}
		want[id] = true
	}
	ids := make([]string, 0, len(want))
	for _, r := range all {
		if want[r.id] {
			ids = append(ids, r.id)
		}
	}
	return ids, nil
}

// ── Iscrizioni massive (evento -> persone) ──

// BulkCreateEventEnrollments crea iscrizioni planned/direct per piu persone
// nella stessa transazione, con le regole della creazione singola
// (CreateEventEnrollment) e la semantica di salto del gesto "alimenta
// platea" (#140): gia iscritta o non attiva non genera errore, solo uno
// scarto motivato.
func (s *SQLStore) BulkCreateEventEnrollments(ctx context.Context, principal Principal, eventID string, input BulkEnrollInput) (BulkEnrollResponse, error) {
	if !principal.IsPeopleAdmin {
		return BulkEnrollResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return BulkEnrollResponse{}, validationError("missing_id", "id evento obbligatorio")
	}
	employeeIDs := make([]string, 0, len(input.EmployeeIDs))
	seen := make(map[string]bool, len(input.EmployeeIDs))
	for _, raw := range input.EmployeeIDs {
		id := strings.TrimSpace(raw)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		employeeIDs = append(employeeIDs, id)
	}
	if len(employeeIDs) == 0 {
		return BulkEnrollResponse{}, validationError("employees_required", "almeno una persona richiesta")
	}
	objective := strings.TrimSpace(input.Objective)
	notes := strings.TrimSpace(input.Notes)

	response := BulkEnrollResponse{Created: make([]BulkEnrollCreatedRow, 0), Skipped: make([]BulkEnrollSkippedRow, 0)}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := lockEventNotCancelled(ctx, tx, eventID); err != nil {
			return err
		}
		for _, employeeID := range employeeIDs {
			var status string
			err := tx.QueryRowContext(ctx, `
SELECT status::text FROM training.employee WHERE id = $1::uuid`, employeeID).Scan(&status)
			if errors.Is(err, sql.ErrNoRows) {
				return notFoundError("employee_not_found", "persona non trovata")
			}
			if err != nil {
				return fmt.Errorf("load training employee status for bulk enrollment: %w", err)
			}
			if status != "active" {
				response.Skipped = append(response.Skipped, BulkEnrollSkippedRow{EmployeeID: employeeID, Reason: "inactive"})
				continue
			}

			var enrollmentID string
			const stmt = `
INSERT INTO training.enrollment (employee_id, event_id, delivery_status, origin, objective, notes)
VALUES ($1::uuid, $2::uuid, 'planned', 'direct', NULLIF($3, ''), NULLIF($4, ''))
ON CONFLICT (employee_id, event_id) DO NOTHING
RETURNING id::text`
			err = tx.QueryRowContext(ctx, stmt, employeeID, eventID, objective, notes).Scan(&enrollmentID)
			if errors.Is(err, sql.ErrNoRows) {
				response.Skipped = append(response.Skipped, BulkEnrollSkippedRow{EmployeeID: employeeID, Reason: "already_enrolled"})
				continue
			}
			if err != nil {
				return fmt.Errorf("create training bulk enrollment: %w", err)
			}
			after, err := entitySnapshot(ctx, tx, "enrollment", enrollmentID)
			if err != nil {
				return err
			}
			if err := s.audit(ctx, tx, principal, "enrollment", enrollmentID, "create", nil, after); err != nil {
				return err
			}
			response.Created = append(response.Created, BulkEnrollCreatedRow{EnrollmentID: enrollmentID, EmployeeID: employeeID})
		}
		response.OK = true
		return nil
	})
	if err != nil {
		return BulkEnrollResponse{}, err
	}
	return response, nil
}
