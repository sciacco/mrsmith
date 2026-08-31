package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Regole formative (#140, §4-Regole, decisioni D6/D7). La semantica di
// platea, natura del bisogno, copertura e tornate vive nel nucleo condiviso
// store_coverage.go (D3): qui si validano gli input, si scrivono le regole e
// si eseguono i gesti di tornata.

// normalizedRuleInput e l'input regola gia validato nella forma (D6): XOR
// platea/posizioni, ricorrenza e ancora in coppia, scadenza obbligatoria.
type normalizedRuleInput struct {
	Name             string
	CourseID         string
	Target           *populationTarget // nil per le regole a posizioni
	PersonIDs        []string          // solo kind people, senza doppioni
	SeatCount        *int
	IsMandatory      bool
	Deadline         time.Time
	RecurrenceMonths *int
	RecurrenceAnchor string // "" senza ricorrenza
	Notes            string
}

func normalizeRuleInput(input RuleInput) (normalizedRuleInput, error) {
	normalized := normalizedRuleInput{
		Name:             strings.TrimSpace(input.Name),
		CourseID:         strings.TrimSpace(input.CourseID),
		SeatCount:        input.SeatCount,
		IsMandatory:      input.IsMandatory,
		RecurrenceMonths: input.RecurrenceMonths,
		RecurrenceAnchor: strings.TrimSpace(input.RecurrenceAnchor),
		Notes:            strings.TrimSpace(input.Notes),
	}
	if normalized.Name == "" {
		return normalized, validationError("name_required", "nome obbligatorio")
	}
	if normalized.CourseID == "" {
		return normalized, validationError("course_required", "corso obbligatorio")
	}
	deadlineRaw := strings.TrimSpace(input.Deadline)
	if deadlineRaw == "" {
		return normalized, validationError("deadline_required", "scadenza obbligatoria")
	}
	deadline, err := time.Parse("2006-01-02", deadlineRaw)
	if err != nil {
		return normalized, validationError("invalid_deadline", "scadenza non valida")
	}
	normalized.Deadline = dateOnly(deadline)

	// Platea XOR posizioni (D6): oltre al CHECK del db, messaggi espliciti.
	if input.Population != nil && input.SeatCount != nil {
		return normalized, validationError("population_and_seats_exclusive", "platea e posizioni sono alternative: indicarne una sola")
	}
	if input.Population == nil && input.SeatCount == nil {
		return normalized, validationError("population_or_seats_required", "indicare la platea oppure il numero di posizioni")
	}
	if input.SeatCount != nil && *input.SeatCount <= 0 {
		return normalized, validationError("invalid_seat_count", "il numero di posizioni deve essere positivo")
	}
	if input.Population != nil {
		target := &populationTarget{
			Kind: strings.TrimSpace(input.Population.Kind),
			ID:   strings.TrimSpace(input.Population.ID),
		}
		if err := target.validate(); err != nil {
			return normalized, err
		}
		normalized.Target = target
		seen := make(map[string]bool, len(input.Population.PersonIDs))
		for _, personID := range input.Population.PersonIDs {
			personID = strings.TrimSpace(personID)
			if personID == "" {
				return normalized, validationError("missing_id", "id persona obbligatorio")
			}
			if seen[personID] {
				continue
			}
			seen[personID] = true
			normalized.PersonIDs = append(normalized.PersonIDs, personID)
		}
	}

	// Ricorrenza e ancora sempre in coppia (D6).
	if (normalized.RecurrenceMonths == nil) != (normalized.RecurrenceAnchor == "") {
		return normalized, validationError("recurrence_pair_required", "ricorrenza in mesi e ancora vanno indicate in coppia")
	}
	if normalized.RecurrenceMonths != nil {
		if *normalized.RecurrenceMonths <= 0 {
			return normalized, validationError("invalid_recurrence_months", "i mesi di ricorrenza devono essere positivi")
		}
		if normalized.RecurrenceAnchor != anchorCalendar && normalized.RecurrenceAnchor != anchorCompletion {
			return normalized, validationError("invalid_recurrence_anchor", "ancora di ricorrenza non supportata")
		}
	}
	return normalized, nil
}

// populationJSON serializza il target per la colonna jsonb; nil per le
// regole a posizioni. Il kind all|people non porta la chiave id (CHECK
// chk_rule_population_id).
func (n normalizedRuleInput) populationJSON() (any, error) {
	if n.Target == nil {
		return nil, nil
	}
	raw, err := json.Marshal(n.Target)
	if err != nil {
		return nil, fmt.Errorf("marshal training rule population target: %w", err)
	}
	return raw, nil
}

// ensureCourseActive e la validazione D6 delle regole: il corso deve
// esistere ED essere attivo (piu severa di ensureCourseExists).
func (s *SQLStore) ensureCourseActive(ctx context.Context, q sqlRunner, courseID string) error {
	var active bool
	err := q.QueryRowContext(ctx, `
SELECT is_active
FROM training.course
WHERE id = $1::uuid`, strings.TrimSpace(courseID)).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return validationError("course_not_found", "corso non trovato")
	}
	if err != nil {
		return fmt.Errorf("check training course active: %w", err)
	}
	if !active {
		return validationError("course_inactive", "corso non attivo")
	}
	return nil
}

// rulePersonIDs carica le righe training_rule_person registrate (tutte,
// anche di persone non piu attive: la platea risolta filtra a parte).
func (s *SQLStore) rulePersonIDs(ctx context.Context, q sqlRunner, ruleID string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `
SELECT employee_id::text
FROM training.training_rule_person
WHERE rule_id = $1::uuid
ORDER BY 1`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("list training rule people: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan training rule person: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]bool, len(a))
	for _, value := range a {
		seen[value] = true
	}
	for _, value := range b {
		if !seen[value] {
			return false
		}
	}
	return true
}

// replaceRulePeople sostituisce per intero le righe training_rule_person
// della regola (kind people); con un elenco vuoto le elimina (kind cambiato).
// Le righe non hanno un id proprio: il cambiamento viene tracciato con un
// audit dedicato sull'entita regola.
func (s *SQLStore) replaceRulePeople(ctx context.Context, tx *sql.Tx, principal Principal, ruleID string, employeeIDs []string) error {
	current, err := s.rulePersonIDs(ctx, tx, ruleID)
	if err != nil {
		return err
	}
	if equalStringSets(current, employeeIDs) {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM training.training_rule_person
WHERE rule_id = $1::uuid`, ruleID); err != nil {
		return fmt.Errorf("delete training rule people: %w", err)
	}
	for _, employeeID := range employeeIDs {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO training.training_rule_person (rule_id, employee_id)
VALUES ($1::uuid, $2::uuid)`, ruleID, employeeID); err != nil {
			return fmt.Errorf("insert training rule person: %w", err)
		}
	}
	before, err := json.Marshal(map[string]any{"employeeIds": current})
	if err != nil {
		return fmt.Errorf("marshal training rule people audit: %w", err)
	}
	after, err := json.Marshal(map[string]any{"employeeIds": employeeIDs})
	if err != nil {
		return fmt.Errorf("marshal training rule people audit: %w", err)
	}
	return s.audit(ctx, tx, principal, "training_rule", ruleID, "set_people", before, after)
}

func (s *SQLStore) CreateRule(ctx context.Context, principal Principal, input RuleInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	normalized, err := normalizeRuleInput(input)
	if err != nil {
		return ActionResponse{}, err
	}
	populationJSON, err := normalized.populationJSON()
	if err != nil {
		return ActionResponse{}, err
	}

	var response ActionResponse
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.ensureCourseActive(ctx, tx, normalized.CourseID); err != nil {
			return err
		}
		if err := s.validatePopulation(ctx, tx, normalized.Target, normalized.PersonIDs); err != nil {
			return err
		}
		const stmt = `
INSERT INTO training.training_rule (
  name,
  course_id,
  population_target,
  seat_count,
  is_mandatory,
  deadline,
  recurrence_months,
  recurrence_anchor,
  notes
) VALUES ($1, $2::uuid, $3::jsonb, $4, $5, $6::date, $7, $8, NULLIF($9, ''))
RETURNING id::text`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			normalized.Name,
			normalized.CourseID,
			populationJSON,
			normalized.SeatCount,
			normalized.IsMandatory,
			normalized.Deadline,
			normalized.RecurrenceMonths,
			nullableText(normalized.RecurrenceAnchor),
			normalized.Notes,
		).Scan(&response.ID); err != nil {
			return fmt.Errorf("create training rule: %w", err)
		}
		if normalized.Target != nil && normalized.Target.Kind == populationKindPeople {
			if err := s.replaceRulePeople(ctx, tx, principal, response.ID, normalized.PersonIDs); err != nil {
				return err
			}
		}
		after, err := entitySnapshot(ctx, tx, "training_rule", response.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_rule", response.ID, "create", nil, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

// UpdateRule e una sostituzione completa (idioma PUT del pacchetto); lo
// stato attivo si governa solo con i gesti activate/deactivate.
func (s *SQLStore) UpdateRule(ctx context.Context, principal Principal, id string, input RuleInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id regola obbligatorio")
	}
	normalized, err := normalizeRuleInput(input)
	if err != nil {
		return ActionResponse{}, err
	}
	populationJSON, err := normalized.populationJSON()
	if err != nil {
		return ActionResponse{}, err
	}

	var response ActionResponse
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var currentCourseID string
		err := tx.QueryRowContext(ctx, `
SELECT course_id::text
FROM training.training_rule
WHERE id = $1::uuid
FOR UPDATE`, id).Scan(&currentCourseID)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("rule_not_found", "regola non trovata")
		}
		if err != nil {
			return fmt.Errorf("load training rule for update: %w", err)
		}
		before, err := entitySnapshot(ctx, tx, "training_rule", id)
		if err != nil {
			return err
		}

		if normalized.CourseID != currentCourseID {
			// D6: il corso e immutabile quando la regola ha gia eventi
			// collegati; il corso nuovo deve esistere ed essere attivo. Il
			// corso invariato non viene rivalidato: una regola viva resta
			// modificabile (scadenza, platea, posizioni) anche se il suo
			// corso e stato archiviato nel frattempo.
			var hasEvents bool
			if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM training.training_event ev
  WHERE ev.source_rule_id = $1::uuid
)`, id).Scan(&hasEvents); err != nil {
				return fmt.Errorf("check training rule events: %w", err)
			}
			if hasEvents {
				return conflictError("rule_has_events", "il corso non si puo cambiare: la regola ha gia eventi collegati")
			}
			if err := s.ensureCourseActive(ctx, tx, normalized.CourseID); err != nil {
				return err
			}
		}
		if err := s.validatePopulation(ctx, tx, normalized.Target, normalized.PersonIDs); err != nil {
			return err
		}

		const stmt = `
UPDATE training.training_rule
SET name = $2,
    course_id = $3::uuid,
    population_target = $4::jsonb,
    seat_count = $5,
    is_mandatory = $6,
    deadline = $7::date,
    recurrence_months = $8,
    recurrence_anchor = $9,
    notes = NULLIF($10, ''),
    updated_at = now()
WHERE id = $1::uuid
RETURNING id::text`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			id,
			normalized.Name,
			normalized.CourseID,
			populationJSON,
			normalized.SeatCount,
			normalized.IsMandatory,
			normalized.Deadline,
			normalized.RecurrenceMonths,
			nullableText(normalized.RecurrenceAnchor),
			normalized.Notes,
		).Scan(&response.ID); err != nil {
			return fmt.Errorf("update training rule: %w", err)
		}

		// Kind people: replace completo delle righe in transazione; con un
		// kind diverso l'elenco nuovo e vuoto e le righe vengono eliminate.
		newPeople := []string(nil)
		if normalized.Target != nil && normalized.Target.Kind == populationKindPeople {
			newPeople = normalized.PersonIDs
		}
		if err := s.replaceRulePeople(ctx, tx, principal, id, newPeople); err != nil {
			return err
		}

		after, err := entitySnapshot(ctx, tx, "training_rule", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_rule", id, "update", before, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

// SetRuleActive attiva o disattiva la regola (gesto, pattern ArchiveCourse).
func (s *SQLStore) SetRuleActive(ctx context.Context, principal Principal, id string, active bool) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id regola obbligatorio")
	}
	action, status := "activate", "activated"
	if !active {
		action, status = "deactivate", "deactivated"
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		before, err := entitySnapshot(ctx, tx, "training_rule", id)
		if appErr, ok := asAppError(err); ok && appErr.code == "entity_not_found" {
			return notFoundError("rule_not_found", "regola non trovata")
		}
		if err != nil {
			return err
		}
		if active {
			facts, err := s.loadRuleFacts(ctx, tx, id)
			if err != nil {
				return err
			}
			if facts.Target != nil && facts.Target.Kind == populationKindSkillArea {
				if _, err := s.skillAreaGroupID(ctx, tx, facts.Target.ID); err != nil {
					return err
				}
			}
		}
		if err := tx.QueryRowContext(ctx, `
UPDATE training.training_rule
SET is_active = $2,
    updated_at = now()
WHERE id = $1::uuid
RETURNING id::text`, id, active).Scan(&response.ID); err != nil {
			return fmt.Errorf("set training rule active: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "training_rule", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_rule", id, action, before, after); err != nil {
			return err
		}
		response.OK = true
		response.Status = status
		return nil
	})
	return response, err
}

// insertRuleEnrollment iscrive una persona a un evento di tornata
// (origin='rule', planned). ON CONFLICT sull'unicita persona+evento: una
// riga gia presente — anche annullata: l'annullamento e una decisione che i
// gesti di regola non scavalcano — non genera doppioni ne nuove scritture.
func (s *SQLStore) insertRuleEnrollment(ctx context.Context, tx *sql.Tx, principal Principal, eventID, ruleID, employeeID string) (string, bool, error) {
	const stmt = `
INSERT INTO training.enrollment (
  employee_id,
  event_id,
  delivery_status,
  origin,
  source_rule_id
) VALUES ($1::uuid, $2::uuid, 'planned', 'rule', $3::uuid)
ON CONFLICT (employee_id, event_id) DO NOTHING
RETURNING id::text`
	var enrollmentID string
	err := tx.QueryRowContext(ctx, stmt, employeeID, eventID, ruleID).Scan(&enrollmentID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("create training rule enrollment: %w", err)
	}
	after, err := entitySnapshot(ctx, tx, "enrollment", enrollmentID)
	if err != nil {
		return "", false, err
	}
	if err := s.audit(ctx, tx, principal, "enrollment", enrollmentID, "create_from_rule", nil, after); err != nil {
		return "", false, err
	}
	return enrollmentID, true, nil
}

// CreateRuleEvent e il gesto "crea evento da regola" (D7): nuova tornata con
// origin='rule', corso della regola e scadenza = prossima tornata. Le regole
// a platea generano le iscrizioni delle persone non coperte; quelle a
// posizioni nascono con l'evento vuoto (People collega le iscrizioni).
func (s *SQLStore) CreateRuleEvent(ctx context.Context, principal Principal, ruleID string) (RuleEventResponse, error) {
	if !principal.IsPeopleAdmin {
		return RuleEventResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	ruleID = strings.TrimSpace(ruleID)
	if ruleID == "" {
		return RuleEventResponse{}, validationError("missing_id", "id regola obbligatorio")
	}

	var response RuleEventResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		// Lock della regola: serializza la creazione concorrente di tornate.
		var lockedID string
		err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM training.training_rule
WHERE id = $1::uuid
FOR UPDATE`, ruleID).Scan(&lockedID)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("rule_not_found", "regola non trovata")
		}
		if err != nil {
			return fmt.Errorf("lock training rule: %w", err)
		}
		facts, err := s.loadRuleFacts(ctx, tx, ruleID)
		if err != nil {
			return err
		}
		if !facts.IsActive {
			return conflictError("rule_inactive", "la regola e disattivata: nessuna nuova tornata")
		}
		deadline, ok, err := s.nextRoundDeadline(ctx, tx, facts)
		if err != nil {
			return err
		}
		if !ok {
			return conflictError("no_next_round", "la regola non ha una prossima tornata condivisa")
		}
		// Una sola tornata non annullata per scadenza (D7).
		var duplicate bool
		if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM training.training_event ev
  WHERE ev.origin = 'rule'
    AND ev.source_rule_id = $1::uuid
    AND ev.cancelled_at IS NULL
    AND ev.rule_deadline = $2::date
)`, facts.ID, deadline).Scan(&duplicate); err != nil {
			return fmt.Errorf("check training rule round: %w", err)
		}
		if duplicate {
			return conflictError("round_already_exists", "esiste gia una tornata non annullata con questa scadenza")
		}

		const insertEvent = `
INSERT INTO training.training_event (
  course_id,
  title,
  origin,
  source_rule_id,
  rule_deadline
) VALUES (
  $1::uuid,
  (SELECT c.title FROM training.course c WHERE c.id = $1::uuid),
  'rule', $2::uuid, $3::date)
RETURNING id::text`
		if err := tx.QueryRowContext(ctx, insertEvent, facts.CourseID, facts.ID, deadline).Scan(&response.ID); err != nil {
			return fmt.Errorf("create training rule event: %w", err)
		}
		if err := copyCourseTrainers(ctx, tx, response.ID, facts.CourseID); err != nil {
			return err
		}
		afterEvent, err := entitySnapshot(ctx, tx, "training_event", response.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_event", response.ID, "create_from_rule", nil, afterEvent); err != nil {
			return err
		}

		if facts.Target != nil {
			// Regola a platea: iscrizioni solo per i membri non coperti
			// (semantica D4). L'evento e appena nato in questa transazione:
			// nessuno puo gia esservi iscritto.
			population, err := s.resolvePopulation(ctx, tx, facts.ID, facts.Target)
			if err != nil {
				return err
			}
			covered, err := s.coveredEmployees(ctx, tx, facts, &deadline)
			if err != nil {
				return err
			}
			for _, employeeID := range population {
				if covered[employeeID] {
					continue
				}
				if _, inserted, err := s.insertRuleEnrollment(ctx, tx, principal, response.ID, facts.ID, employeeID); err != nil {
					return err
				} else if inserted {
					response.EnrollmentsCreated++
				}
			}
		}
		response.OK = true
		return nil
	})
	return response, err
}

// eventEnrolledEmployees carica le persone con una riga di iscrizione
// sull'evento, incluse le annullate: l'unicita persona+evento copre anche le
// cancelled e i gesti di regola non le scavalcano (vedi insertRuleEnrollment),
// quindi il gesto di alimentazione e la coda della platea non alimentata le
// trattano allo stesso modo — non aggiungibili (D7).
func (s *SQLStore) eventEnrolledEmployees(ctx context.Context, q sqlRunner, eventID string) (map[string]bool, error) {
	rows, err := q.QueryContext(ctx, `
SELECT en.employee_id::text
FROM training.enrollment en
WHERE en.event_id = $1::uuid`, eventID)
	if err != nil {
		return nil, fmt.Errorf("list training event enrolled employees: %w", err)
	}
	defer rows.Close()

	enrolled := make(map[string]bool)
	for rows.Next() {
		var employeeID string
		if err := rows.Scan(&employeeID); err != nil {
			return nil, fmt.Errorf("scan training event enrolled employee: %w", err)
		}
		enrolled[employeeID] = true
	}
	return enrolled, rows.Err()
}

// employeeRef carica nome visualizzato ed email di una persona.
func (s *SQLStore) employeeRef(ctx context.Context, q sqlRunner, employeeID string) (name, email string, err error) {
	err = q.QueryRowContext(ctx, `
SELECT concat(e.last_name, ' ', e.first_name), e.email::text
FROM training.employee e
WHERE e.id = $1::uuid`, employeeID).Scan(&name, &email)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", notFoundError("employee_not_found", "persona non trovata")
	}
	if err != nil {
		return "", "", fmt.Errorf("load training employee ref: %w", err)
	}
	return name, email, nil
}

// FeedRuleEvent e il gesto "alimenta platea" (D7): aggiunge all'evento della
// tornata i membri attuali della platea non coperti e senza alcuna iscrizione
// sull'evento — anche annullata: l'annullamento e una decisione che il gesto
// non scavalca. Solo regole a platea; le regole a posizioni non hanno una
// platea da alimentare.
func (s *SQLStore) FeedRuleEvent(ctx context.Context, principal Principal, eventID string) (FeedEventResponse, error) {
	if !principal.IsPeopleAdmin {
		return FeedEventResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return FeedEventResponse{}, validationError("missing_id", "id evento obbligatorio")
	}

	response := FeedEventResponse{EventID: eventID, Added: make([]FeedAddedRow, 0)}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var (
			origin       string
			sourceRuleID sql.NullString
			ruleDeadline sql.NullTime
			cancelled    bool
		)
		err := tx.QueryRowContext(ctx, `
SELECT ev.origin, ev.source_rule_id::text, ev.rule_deadline, ev.cancelled_at IS NOT NULL
FROM training.training_event ev
WHERE ev.id = $1::uuid
FOR UPDATE`, eventID).Scan(&origin, &sourceRuleID, &ruleDeadline, &cancelled)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("event_not_found", "evento non trovato")
		}
		if err != nil {
			return fmt.Errorf("load training event for feed: %w", err)
		}
		if origin != "rule" || !sourceRuleID.Valid || strings.TrimSpace(sourceRuleID.String) == "" {
			return conflictError("event_not_from_rule", "l'evento non e una tornata di regola")
		}
		if cancelled {
			return conflictError("event_cancelled", "un evento annullato non accetta nuove iscrizioni")
		}
		var lockedRuleID string
		if err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM training.training_rule
WHERE id = $1::uuid
FOR UPDATE`, sourceRuleID.String).Scan(&lockedRuleID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return notFoundError("rule_not_found", "regola non trovata")
			}
			return fmt.Errorf("lock training rule for feed: %w", err)
		}
		facts, err := s.loadRuleFacts(ctx, tx, sourceRuleID.String)
		if err != nil {
			return err
		}
		if !facts.IsActive {
			return conflictError("rule_inactive", "la regola e disattivata: non si possono aggiungere persone")
		}
		if facts.Target == nil {
			return conflictError("rule_without_population", "la regola e a posizioni: l'alimentazione riguarda solo le regole a platea")
		}
		current, err := s.currentRound(ctx, tx, facts.ID)
		if err != nil {
			return err
		}
		if current == nil || current.EventID != eventID {
			return conflictError("event_not_current_round", "si possono aggiungere persone solo all'evento piu recente non annullato della regola")
		}
		var roundDeadline *time.Time
		if ruleDeadline.Valid {
			value := dateOnly(ruleDeadline.Time)
			roundDeadline = &value
		}

		population, err := s.resolvePopulation(ctx, tx, facts.ID, facts.Target)
		if err != nil {
			return err
		}
		covered, err := s.coveredEmployees(ctx, tx, facts, roundDeadline)
		if err != nil {
			return err
		}
		enrolled, err := s.eventEnrolledEmployees(ctx, tx, eventID)
		if err != nil {
			return err
		}

		addedEmployeeIDs := make([]string, 0)
		for _, employeeID := range population {
			if covered[employeeID] || enrolled[employeeID] {
				continue
			}
			enrollmentID, inserted, err := s.insertRuleEnrollment(ctx, tx, principal, eventID, facts.ID, employeeID)
			if err != nil {
				return err
			}
			if !inserted {
				continue
			}
			name, _, err := s.employeeRef(ctx, tx, employeeID)
			if err != nil {
				return err
			}
			response.Added = append(response.Added, FeedAddedRow{
				EnrollmentID: enrollmentID,
				EmployeeID:   employeeID,
				EmployeeName: name,
			})
			addedEmployeeIDs = append(addedEmployeeIDs, employeeID)
		}
		if len(addedEmployeeIDs) > 0 {
			payload, err := json.Marshal(map[string]any{"addedEmployeeIds": addedEmployeeIDs})
			if err != nil {
				return fmt.Errorf("marshal training feed audit: %w", err)
			}
			if err := s.audit(ctx, tx, principal, "training_event", eventID, "feed", nil, payload); err != nil {
				return err
			}
		}
		response.OK = true
		return nil
	})
	if err != nil {
		return FeedEventResponse{}, err
	}
	return response, err
}

// ruleStatus riunisce i campi calcolati di una regola (lista e dettaglio).
type ruleStatus struct {
	Round        *roundInfo
	NextDeadline *time.Time
	Population   []string        // platea risolta (solo regole a platea)
	Covered      map[string]bool // persone coperte (solo regole a platea)
	CoveredCount int             // platea: coperti nella platea; posizioni: posizioni coperte
}

// ruleCoverageStatus calcola i campi derivati riusando il nucleo condiviso
// (D3): tornata in corso, prossima tornata e copertura secondo la natura del
// bisogno. Per le regole a platea il conteggio e l'intersezione tra platea
// risolta e persone coperte; per le regole a posizioni delega a
// coveredSeatCount.
func (s *SQLStore) ruleCoverageStatus(ctx context.Context, q sqlRunner, facts ruleFacts) (ruleStatus, error) {
	var status ruleStatus
	round, err := s.currentRound(ctx, q, facts.ID)
	if err != nil {
		return status, err
	}
	status.Round = round
	next, ok, err := s.nextRoundDeadline(ctx, q, facts)
	if err != nil {
		return status, err
	}
	if ok {
		value := next
		status.NextDeadline = &value
	}
	if facts.Target == nil {
		covered, err := s.coveredSeatCount(ctx, q, facts)
		if err != nil {
			return status, err
		}
		status.CoveredCount = covered
		return status, nil
	}
	population, err := s.resolvePopulation(ctx, q, facts.ID, facts.Target)
	if err != nil {
		return status, err
	}
	status.Population = population
	// Stessa scelta di scadenza di coveredSeatCount: la finestra della
	// tornata in corso vale solo per il bisogno di frequenza con ricorrenza
	// ad ancora di calendario.
	var roundDeadline *time.Time
	if facts.Need == needAttendance && facts.RecurrenceMonths != nil && facts.RecurrenceAnchor == anchorCalendar && round != nil {
		value := round.RuleDeadline
		roundDeadline = &value
	}
	covered, err := s.coveredEmployees(ctx, q, facts, roundDeadline)
	if err != nil {
		return status, err
	}
	status.Covered = covered
	for _, employeeID := range population {
		if covered[employeeID] {
			status.CoveredCount++
		}
	}
	return status, nil
}

// ListRules elenca le regole con i campi calcolati (§4): natura del bisogno,
// platea o posizioni, dimensione della platea risolta, coperti, prossima
// tornata e tornata in corso. Scala piccola (D10): i derivati si calcolano
// per regola riusando il nucleo condiviso.
func (s *SQLStore) ListRules(ctx context.Context) ([]RuleListRow, error) {
	const q = `
SELECT
  r.id::text,
  r.name,
  r.course_id::text,
  c.title,
  r.is_mandatory,
  r.is_active,
  r.created_at::text,
  r.updated_at::text
FROM training.training_rule r
JOIN training.course c ON c.id = r.course_id
ORDER BY r.is_active DESC, r.deadline, r.name, r.id
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training rules: %w", err)
	}
	defer rows.Close()

	result := make([]RuleListRow, 0)
	for rows.Next() {
		var row RuleListRow
		if err := rows.Scan(
			&row.ID,
			&row.Name,
			&row.CourseID,
			&row.CourseTitle,
			&row.IsMandatory,
			&row.IsActive,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan training rule: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range result {
		facts, err := s.loadRuleFacts(ctx, s.db, result[i].ID)
		if err != nil {
			return nil, err
		}
		result[i].Need = facts.Need
		result[i].SeatCount = facts.SeatCount
		result[i].Deadline = facts.Deadline.Format("2006-01-02")
		result[i].RecurrenceMonths = facts.RecurrenceMonths
		result[i].RecurrenceAnchor = facts.RecurrenceAnchor
		if facts.Target != nil {
			result[i].PopulationKind = facts.Target.Kind
		}
		status, err := s.ruleCoverageStatus(ctx, s.db, facts)
		if err != nil {
			if _, ok := asAppError(err); ok {
				// Platea non piu risolvibile (es. area rimasta senza gruppo
				// collegato dopo la creazione della regola): la riga resta
				// visibile senza campi calcolati; il dettaglio espone
				// l'errore esplicito.
				continue
			}
			return nil, err
		}
		result[i].CoveredCount = status.CoveredCount
		if facts.Target != nil {
			size := len(status.Population)
			result[i].PopulationSize = &size
		}
		if status.NextDeadline != nil {
			result[i].NextRoundDeadline = status.NextDeadline.Format("2006-01-02")
		}
		if status.Round != nil {
			result[i].CurrentRoundEventID = status.Round.EventID
			result[i].CurrentRoundDeadline = status.Round.RuleDeadline.Format("2006-01-02")
		}
	}
	return result, nil
}

// ruleRounds elenca le tornate della regola (eventi origin='rule').
func (s *SQLStore) ruleRounds(ctx context.Context, q sqlRunner, ruleID string) ([]RuleRoundRow, error) {
	const query = `
SELECT
  ev.id::text,
  COALESCE(ev.rule_deadline::text, ''),
  ev.cancelled_at IS NOT NULL,
  COALESCE(ev.cancelled_at::text, ''),
  (
    SELECT COUNT(*)
    FROM training.enrollment en
    WHERE en.event_id = ev.id
      AND en.delivery_status <> 'cancelled'
  ),
  ev.created_at::text
FROM training.training_event ev
WHERE ev.origin = 'rule'
  AND ev.source_rule_id = $1::uuid
ORDER BY ev.rule_deadline DESC NULLS LAST, ev.created_at DESC, ev.id
LIMIT 200`
	rows, err := q.QueryContext(ctx, query, ruleID)
	if err != nil {
		return nil, fmt.Errorf("list training rule rounds: %w", err)
	}
	defer rows.Close()

	result := make([]RuleRoundRow, 0)
	for rows.Next() {
		var row RuleRoundRow
		if err := rows.Scan(
			&row.EventID,
			&row.RuleDeadline,
			&row.Cancelled,
			&row.CancelledAt,
			&row.EnrollmentsCount,
			&row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan training rule round: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetRuleDetail restituisce la regola con la platea risolta e la copertura
// per persona (o le posizioni richieste/coperte) e le tornate (§4).
func (s *SQLStore) GetRuleDetail(ctx context.Context, id string) (RuleDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return RuleDetail{}, validationError("missing_id", "id regola obbligatorio")
	}

	const headerQuery = `
SELECT
  r.id::text,
  r.name,
  c.title,
  r.is_mandatory,
  COALESCE(r.notes, ''),
  r.created_at::text,
  r.updated_at::text
FROM training.training_rule r
JOIN training.course c ON c.id = r.course_id
WHERE r.id = $1::uuid`
	var detail RuleDetail
	err := s.db.QueryRowContext(ctx, headerQuery, id).Scan(
		&detail.ID,
		&detail.Name,
		&detail.CourseTitle,
		&detail.IsMandatory,
		&detail.Notes,
		&detail.CreatedAt,
		&detail.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RuleDetail{}, notFoundError("rule_not_found", "regola non trovata")
	}
	if err != nil {
		return RuleDetail{}, fmt.Errorf("load training rule detail: %w", err)
	}

	facts, err := s.loadRuleFacts(ctx, s.db, id)
	if err != nil {
		return RuleDetail{}, err
	}
	detail.CourseID = facts.CourseID
	detail.Need = facts.Need
	detail.CertificationID = facts.CertificationID
	detail.Deadline = facts.Deadline.Format("2006-01-02")
	detail.RecurrenceMonths = facts.RecurrenceMonths
	detail.RecurrenceAnchor = facts.RecurrenceAnchor
	detail.IsActive = facts.IsActive

	status, err := s.ruleCoverageStatus(ctx, s.db, facts)
	if err != nil {
		return RuleDetail{}, err
	}
	if status.NextDeadline != nil {
		detail.NextRoundDeadline = status.NextDeadline.Format("2006-01-02")
	}
	if status.Round != nil {
		detail.CurrentRoundEventID = status.Round.EventID
		detail.CurrentRoundDeadline = status.Round.RuleDeadline.Format("2006-01-02")
	}

	if facts.Target != nil {
		population := &RulePopulationDetail{
			Kind:         facts.Target.Kind,
			TargetID:     facts.Target.ID,
			Size:         len(status.Population),
			CoveredCount: status.CoveredCount,
			Members:      make([]RulePopulationMember, 0, len(status.Population)),
		}
		if facts.Target.Kind == populationKindPeople {
			personIDs, err := s.rulePersonIDs(ctx, s.db, id)
			if err != nil {
				return RuleDetail{}, err
			}
			population.PersonIDs = personIDs
		}
		for _, employeeID := range status.Population {
			name, email, err := s.employeeRef(ctx, s.db, employeeID)
			if err != nil {
				return RuleDetail{}, err
			}
			population.Members = append(population.Members, RulePopulationMember{
				EmployeeID: employeeID,
				Name:       name,
				Email:      email,
				Covered:    status.Covered[employeeID],
			})
		}
		sort.Slice(population.Members, func(i, j int) bool {
			if population.Members[i].Name != population.Members[j].Name {
				return population.Members[i].Name < population.Members[j].Name
			}
			return population.Members[i].EmployeeID < population.Members[j].EmployeeID
		})
		detail.Population = population
	} else {
		requested := 0
		if facts.SeatCount != nil {
			requested = *facts.SeatCount
		}
		detail.Seats = &RuleSeatsDetail{Requested: requested, Covered: status.CoveredCount}
	}

	rounds, err := s.ruleRounds(ctx, s.db, id)
	if err != nil {
		return RuleDetail{}, err
	}
	detail.Rounds = rounds
	return detail, nil
}
