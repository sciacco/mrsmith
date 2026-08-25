package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Nucleo condiviso di copertura per le regole formative (#140, D3/D4/D10):
// risoluzione della platea, natura del bisogno, copertura e tornate vivono
// solo qui. Regole, gesti evento, accoglimento richieste e code derivate
// riusano queste funzioni senza duplicarne la semantica.
//
// I calcoli di calendario (prossima tornata, finestra di tornata, scadenza
// personale, orizzonte) sono funzioni pure senza ctx/db: le query ricevono i
// limiti gia calcolati come parametri e non contengono aritmetica di date.

// Natura del bisogno espresso dalla regola, derivata dal corso: certificazione
// collegata al corso -> bisogno di certificazione; altrimenti frequenza.
const (
	needAttendance    = "attendance"
	needCertification = "certification"
)

// Ancora della ricorrenza (valori del CHECK chk_rule_recurrence_anchor).
const (
	anchorCalendar   = "calendar"
	anchorCompletion = "completion"
)

// Kind ammessi per population_target (CHECK chk_rule_population_kind).
const (
	populationKindAll         = "all"
	populationKindTeam        = "team"
	populationKindSkillArea   = "skill_area"
	populationKindCustomGroup = "custom_group"
	populationKindPeople      = "people"
)

// Orizzonte "in scadenza" delle code: default e massimo del parametro query
// withinDays (D4).
const (
	defaultHorizonDays = 60
	maxHorizonDays     = 365
)

// populationTarget e la forma Go del jsonb training_rule.population_target.
type populationTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}

// parsePopulationTarget legge il jsonb population_target; raw vuoto o NULL
// restituisce nil (regola a posizioni).
func parsePopulationTarget(raw []byte) (*populationTarget, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var target populationTarget
	if err := json.Unmarshal(raw, &target); err != nil {
		return nil, fmt.Errorf("parse training rule population target: %w", err)
	}
	target.Kind = strings.TrimSpace(target.Kind)
	target.ID = strings.TrimSpace(target.ID)
	return &target, nil
}

// validate controlla forma e coerenza del target: stesse regole dei CHECK
// chk_rule_population_kind / chk_rule_population_id, con messaggi espliciti.
func (t *populationTarget) validate() error {
	if t == nil {
		return nil
	}
	switch t.Kind {
	case populationKindAll, populationKindPeople:
		if t.ID != "" {
			return validationError("population_id_forbidden", "id della platea non ammesso per organizzazione e singole persone")
		}
	case populationKindTeam, populationKindSkillArea, populationKindCustomGroup:
		if t.ID == "" {
			return validationError("population_id_required", "id della platea obbligatorio per team, area di competenza e gruppo locale")
		}
	default:
		return validationError("population_kind_invalid", "tipo di platea non supportato")
	}
	return nil
}

// ruleFacts riunisce i fatti della regola necessari a copertura e tornate.
type ruleFacts struct {
	ID               string
	CourseID         string
	Target           *populationTarget // nil per le regole a posizioni
	SeatCount        *int
	Deadline         time.Time
	RecurrenceMonths *int
	RecurrenceAnchor string // "" senza ricorrenza
	IsActive         bool
	Need             string // needAttendance | needCertification
	CertificationID  string // valorizzato solo con bisogno di certificazione
}

// loadRuleFacts carica la regola e ne deriva la natura del bisogno dal corso
// (course.leads_to_cert_id).
func (s *SQLStore) loadRuleFacts(ctx context.Context, q sqlRunner, ruleID string) (ruleFacts, error) {
	const query = `
SELECT
  r.id::text,
  r.course_id::text,
  r.population_target,
  r.seat_count,
  r.deadline,
  r.recurrence_months,
  COALESCE(r.recurrence_anchor, ''),
  r.is_active,
  COALESCE(c.leads_to_cert_id::text, '')
FROM training.training_rule r
JOIN training.course c ON c.id = r.course_id
WHERE r.id = $1::uuid`
	var (
		facts     ruleFacts
		rawTarget []byte
		seatCount sql.NullInt64
		months    sql.NullInt64
	)
	err := q.QueryRowContext(ctx, query, strings.TrimSpace(ruleID)).Scan(
		&facts.ID,
		&facts.CourseID,
		&rawTarget,
		&seatCount,
		&facts.Deadline,
		&months,
		&facts.RecurrenceAnchor,
		&facts.IsActive,
		&facts.CertificationID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ruleFacts{}, notFoundError("rule_not_found", "regola non trovata")
	}
	if err != nil {
		return ruleFacts{}, fmt.Errorf("load training rule facts: %w", err)
	}
	facts.SeatCount = nullInt(seatCount)
	facts.RecurrenceMonths = nullInt(months)
	facts.Target, err = parsePopulationTarget(rawTarget)
	if err != nil {
		return ruleFacts{}, err
	}
	facts.Need = needAttendance
	if facts.CertificationID != "" {
		facts.Need = needCertification
	}
	return facts, nil
}

// skillAreaGroupID risolve il gruppo locale collegato all'area: un'area senza
// gruppo collegato non puo essere usata come platea (D6).
func (s *SQLStore) skillAreaGroupID(ctx context.Context, q sqlRunner, skillAreaID string) (string, error) {
	var groupID sql.NullString
	err := q.QueryRowContext(ctx, `
SELECT custom_group_id::text
FROM training.skill_area
WHERE id = $1::uuid`, skillAreaID).Scan(&groupID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", validationError("skill_area_not_found", "area di competenza non trovata")
	}
	if err != nil {
		return "", fmt.Errorf("load training skill area group: %w", err)
	}
	if !groupID.Valid || strings.TrimSpace(groupID.String) == "" {
		return "", validationError("skill_area_without_group", "l'area di competenza non ha un gruppo locale collegato")
	}
	return groupID.String, nil
}

// validatePopulation verifica le esistenze del target (D6): team e gruppo
// locale esistenti, area di competenza esistente e con gruppo locale
// collegato, persone presenti e tutte attive per kind 'people'. Le righe
// training_rule_person esistono solo per kind 'people' (fatto di schema),
// quindi un elenco persone con altri kind viene rifiutato.
func (s *SQLStore) validatePopulation(ctx context.Context, q sqlRunner, target *populationTarget, personIDs []string) error {
	if target == nil {
		return nil
	}
	if err := target.validate(); err != nil {
		return err
	}
	if target.Kind != populationKindPeople && len(personIDs) > 0 {
		return validationError("population_people_forbidden", "elenco persone ammesso solo per platea a singole persone")
	}
	switch target.Kind {
	case populationKindTeam:
		var exists bool
		if err := q.QueryRowContext(ctx, `
SELECT EXISTS (SELECT 1 FROM training.team WHERE id = $1::uuid)`, target.ID).Scan(&exists); err != nil {
			return fmt.Errorf("check training rule team: %w", err)
		}
		if !exists {
			return validationError("team_not_found", "team non trovato")
		}
	case populationKindCustomGroup:
		var exists bool
		if err := q.QueryRowContext(ctx, `
SELECT EXISTS (SELECT 1 FROM training.custom_groups WHERE id = $1::uuid)`, target.ID).Scan(&exists); err != nil {
			return fmt.Errorf("check training rule custom group: %w", err)
		}
		if !exists {
			return validationError("custom_group_not_found", "gruppo locale non trovato")
		}
	case populationKindSkillArea:
		if _, err := s.skillAreaGroupID(ctx, q, target.ID); err != nil {
			return err
		}
	case populationKindPeople:
		if len(personIDs) == 0 {
			return validationError("population_people_required", "indicare almeno una persona nella platea")
		}
		for _, personID := range personIDs {
			personID = strings.TrimSpace(personID)
			if personID == "" {
				return validationError("missing_id", "id persona obbligatorio")
			}
			if err := s.ensureEmployeeActive(ctx, q, personID); err != nil {
				return err
			}
		}
	}
	return nil
}

const groupMembersQuery = `
SELECT DISTINCT e.id::text
FROM training.custom_group_members m
JOIN training.employee e ON e.id = m.employee_id
WHERE m.group_id = $1::uuid
  AND e.status = 'active'
ORDER BY 1`

// resolvePopulation risolve la platea in un insieme di persone attive
// distinte (D4: platea "persone attive" = employee.status = 'active';
// membership attiva = end_date IS NULL). Kind: all = tutti gli attivi;
// team = membership attive del team; skill_area = membri attivi del gruppo
// locale collegato (assente -> errore D6); custom_group = membri attivi del
// gruppo; people = righe training_rule_person filtrate sugli attivi.
func (s *SQLStore) resolvePopulation(ctx context.Context, q sqlRunner, ruleID string, target *populationTarget) ([]string, error) {
	if target == nil {
		return nil, nil
	}
	if err := target.validate(); err != nil {
		return nil, err
	}
	var (
		query string
		args  []any
	)
	switch target.Kind {
	case populationKindAll:
		query = `
SELECT e.id::text
FROM training.employee e
WHERE e.status = 'active'
ORDER BY 1`
	case populationKindTeam:
		query = `
SELECT DISTINCT e.id::text
FROM training.team_membership tm
JOIN training.employee e ON e.id = tm.employee_id
WHERE tm.team_id = $1::uuid
  AND tm.end_date IS NULL
  AND e.status = 'active'
ORDER BY 1`
		args = append(args, target.ID)
	case populationKindSkillArea:
		groupID, err := s.skillAreaGroupID(ctx, q, target.ID)
		if err != nil {
			return nil, err
		}
		query = groupMembersQuery
		args = append(args, groupID)
	case populationKindCustomGroup:
		query = groupMembersQuery
		args = append(args, target.ID)
	case populationKindPeople:
		query = `
SELECT DISTINCT e.id::text
FROM training.training_rule_person rp
JOIN training.employee e ON e.id = rp.employee_id
WHERE rp.rule_id = $1::uuid
  AND e.status = 'active'
ORDER BY 1`
		args = append(args, strings.TrimSpace(ruleID))
	}
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("resolve training rule population: %w", err)
	}
	defer rows.Close()

	employees := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan training rule population: %w", err)
		}
		employees = append(employees, id)
	}
	return employees, rows.Err()
}

// coveredEmployees calcola le persone coperte per la regola secondo la natura
// del bisogno (D4).
//
// Certificazione: award 'passed_exam' della certificazione collegata al
// corso, qualunque validation_source (le fonti esterne valgono), intestato a
// persona in forza (status <> 'terminated') e valido alla data di
// riferimento: la scadenza di tornata quando indicata, altrimenti oggi. La
// finestra di tornata non si applica al bisogno di certificazione.
//
// Frequenza: iscrizione 'completed' su un evento dello stesso corso, con
// qualunque origine e anche su evento annullato (il completamento e un
// fatto); data di completamento = COALESCE(actual_end, updated_at::date).
//   - roundDeadline valorizzata con ricorrenza ad ancora di calendario:
//     conta solo un completamento nella finestra (scadenza-mesi, scadenza];
//   - ancora completion: coperta la persona il cui ultimo completamento ha
//     scadenza personale (ultimo completamento + mesi) ancora futura;
//     roundDeadline viene ignorata (nessuna tornata condivisa);
//   - senza ricorrenza, o senza tornata indicata: basta un completamento.
//
// La mappa puo contenere persone esterne alla platea: e il chiamante a
// intersecarla con la platea risolta.
func (s *SQLStore) coveredEmployees(ctx context.Context, q sqlRunner, rule ruleFacts, roundDeadline *time.Time) (map[string]bool, error) {
	today := dateOnly(time.Now())

	if rule.Need == needCertification {
		reference := today
		if roundDeadline != nil {
			reference = dateOnly(*roundDeadline)
		}
		return s.certificationCoveredEmployees(ctx, q, rule.CertificationID, reference)
	}

	if rule.RecurrenceMonths != nil && rule.RecurrenceAnchor == anchorCompletion {
		completions, err := s.lastCompletionByEmployee(ctx, q, rule.CourseID)
		if err != nil {
			return nil, err
		}
		covered := make(map[string]bool, len(completions))
		for employeeID, last := range completions {
			lastCompletion := last
			if personalDeadline(&lastCompletion, *rule.RecurrenceMonths, rule.Deadline).After(today) {
				covered[employeeID] = true
			}
		}
		return covered, nil
	}

	var windowFrom, windowTo *time.Time
	if roundDeadline != nil && rule.RecurrenceMonths != nil && rule.RecurrenceAnchor == anchorCalendar {
		from, to := roundWindow(dateOnly(*roundDeadline), *rule.RecurrenceMonths)
		windowFrom, windowTo = &from, &to
	}
	return s.attendanceCoveredEmployees(ctx, q, rule.CourseID, windowFrom, windowTo)
}

func (s *SQLStore) certificationCoveredEmployees(ctx context.Context, q sqlRunner, certificationID string, reference time.Time) (map[string]bool, error) {
	const query = `
SELECT DISTINCT ca.employee_id::text
FROM training.certification_award ca
JOIN training.employee e ON e.id = ca.employee_id
WHERE ca.certification_id = $1::uuid
  AND ca.outcome = 'passed_exam'
  AND (ca.expires_on IS NULL OR ca.expires_on > $2::date)
  AND e.status <> 'terminated'`
	rows, err := q.QueryContext(ctx, query, certificationID, reference)
	if err != nil {
		return nil, fmt.Errorf("load training certification coverage: %w", err)
	}
	defer rows.Close()

	covered := make(map[string]bool)
	for rows.Next() {
		var employeeID string
		if err := rows.Scan(&employeeID); err != nil {
			return nil, fmt.Errorf("scan training certification coverage: %w", err)
		}
		covered[employeeID] = true
	}
	return covered, rows.Err()
}

// attendanceCoveredEmployees carica le persone con un completamento su un
// evento del corso; la finestra opzionale (from escluso, to incluso) arriva
// gia calcolata dal chiamante (D10).
func (s *SQLStore) attendanceCoveredEmployees(ctx context.Context, q sqlRunner, courseID string, windowFrom, windowTo *time.Time) (map[string]bool, error) {
	const query = `
SELECT DISTINCT en.employee_id::text
FROM training.enrollment en
JOIN training.training_event ev ON ev.id = en.event_id
WHERE ev.course_id = $1::uuid
  AND en.delivery_status = 'completed'
  AND ($2::date IS NULL OR COALESCE(en.actual_end, en.updated_at::date) > $2::date)
  AND ($3::date IS NULL OR COALESCE(en.actual_end, en.updated_at::date) <= $3::date)`
	rows, err := q.QueryContext(ctx, query, courseID, nullableDate(windowFrom), nullableDate(windowTo))
	if err != nil {
		return nil, fmt.Errorf("load training attendance coverage: %w", err)
	}
	defer rows.Close()

	covered := make(map[string]bool)
	for rows.Next() {
		var employeeID string
		if err := rows.Scan(&employeeID); err != nil {
			return nil, fmt.Errorf("scan training attendance coverage: %w", err)
		}
		covered[employeeID] = true
	}
	return covered, rows.Err()
}

// lastCompletionByEmployee carica per persona l'ultimo completamento su un
// evento del corso (data D4: COALESCE(actual_end, updated_at::date)); il
// confronto con la scadenza personale avviene in Go (D10).
func (s *SQLStore) lastCompletionByEmployee(ctx context.Context, q sqlRunner, courseID string) (map[string]time.Time, error) {
	const query = `
SELECT en.employee_id::text, MAX(COALESCE(en.actual_end, en.updated_at::date))
FROM training.enrollment en
JOIN training.training_event ev ON ev.id = en.event_id
WHERE ev.course_id = $1::uuid
  AND en.delivery_status = 'completed'
GROUP BY en.employee_id`
	rows, err := q.QueryContext(ctx, query, courseID)
	if err != nil {
		return nil, fmt.Errorf("load training last completions: %w", err)
	}
	defer rows.Close()

	completions := make(map[string]time.Time)
	for rows.Next() {
		var (
			employeeID string
			last       time.Time
		)
		if err := rows.Scan(&employeeID, &last); err != nil {
			return nil, fmt.Errorf("scan training last completion: %w", err)
		}
		completions[employeeID] = dateOnly(last)
	}
	return completions, rows.Err()
}

// coveredSeatCount conta le posizioni coperte di una regola a posizioni,
// secondo la natura del bisogno: certificazione = persone in forza distinte
// con award valido oggi; frequenza = persone distinte coperte per D4, con la
// finestra della tornata in corso per le ricorrenze ad ancora di calendario.
func (s *SQLStore) coveredSeatCount(ctx context.Context, q sqlRunner, rule ruleFacts) (int, error) {
	var roundDeadline *time.Time
	if rule.Need == needAttendance && rule.RecurrenceMonths != nil && rule.RecurrenceAnchor == anchorCalendar {
		round, err := s.currentRound(ctx, q, rule.ID)
		if err != nil {
			return 0, err
		}
		if round != nil {
			deadline := round.RuleDeadline
			roundDeadline = &deadline
		}
	}
	covered, err := s.coveredEmployees(ctx, q, rule, roundDeadline)
	if err != nil {
		return 0, err
	}
	return len(covered), nil
}

// roundInfo descrive la tornata in corso: l'evento della regola non annullato
// con la rule_deadline massima (D4).
type roundInfo struct {
	EventID      string
	RuleDeadline time.Time
}

func (s *SQLStore) currentRound(ctx context.Context, q sqlRunner, ruleID string) (*roundInfo, error) {
	const query = `
SELECT ev.id::text, ev.rule_deadline
FROM training.training_event ev
WHERE ev.origin = 'rule'
  AND ev.source_rule_id = $1::uuid
  AND ev.cancelled_at IS NULL
  AND ev.rule_deadline IS NOT NULL
ORDER BY ev.rule_deadline DESC, ev.created_at DESC
LIMIT 1`
	var round roundInfo
	err := q.QueryRowContext(ctx, query, strings.TrimSpace(ruleID)).Scan(&round.EventID, &round.RuleDeadline)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load training rule current round: %w", err)
	}
	round.RuleDeadline = dateOnly(round.RuleDeadline)
	return &round, nil
}

// nextRoundDeadline legge le tornate registrate e delega il calcolo alla
// funzione pura computeNextRoundDeadline. ok=false quando la regola non ha
// una prossima tornata condivisa.
func (s *SQLStore) nextRoundDeadline(ctx context.Context, q sqlRunner, rule ruleFacts) (time.Time, bool, error) {
	const query = `
SELECT MAX(ev.rule_deadline)
FROM training.training_event ev
WHERE ev.origin = 'rule'
  AND ev.source_rule_id = $1::uuid
  AND ev.cancelled_at IS NULL
  AND ev.rule_deadline IS NOT NULL`
	var last sql.NullTime
	if err := q.QueryRowContext(ctx, query, rule.ID).Scan(&last); err != nil {
		return time.Time{}, false, fmt.Errorf("load training rule last round: %w", err)
	}
	var lastDeadline *time.Time
	if last.Valid {
		value := dateOnly(last.Time)
		lastDeadline = &value
	}
	deadline, ok := computeNextRoundDeadline(dateOnly(rule.Deadline), rule.RecurrenceMonths, rule.RecurrenceAnchor, lastDeadline)
	return deadline, ok, nil
}

// -----------------------------------------------------------------------
// Funzioni pure di calendario (D10): nessun ctx, nessun db. Testate in
// coverage_calc_test.go.
// -----------------------------------------------------------------------

// computeNextRoundDeadline calcola la scadenza della prossima tornata (D4):
// senza tornate registrate e la scadenza della regola (prima tornata); con
// una tornata registrata la successiva esiste solo per le ricorrenze ad
// ancora di calendario e cade a ultima scadenza + mesi. ok=false quando non
// esiste una prossima tornata condivisa (regola senza ricorrenza gia svolta,
// oppure ancora completion).
func computeNextRoundDeadline(ruleDeadline time.Time, months *int, anchor string, lastRoundDeadline *time.Time) (time.Time, bool) {
	if lastRoundDeadline == nil {
		return ruleDeadline, true
	}
	if months == nil || anchor != anchorCalendar {
		return time.Time{}, false
	}
	return addMonthsClamped(*lastRoundDeadline, *months), true
}

// roundWindow calcola la finestra di copertura di una tornata con ancora di
// calendario: (scadenza - mesi, scadenza], estremo inferiore escluso e
// superiore incluso.
func roundWindow(roundDeadline time.Time, months int) (from, to time.Time) {
	return addMonthsClamped(roundDeadline, -months), roundDeadline
}

// personalDeadline calcola la scadenza personale con ancora completion:
// ultimo completamento + mesi; chi non ha mai completato scade con la
// scadenza della regola.
func personalDeadline(lastCompletion *time.Time, months int, ruleDeadline time.Time) time.Time {
	if lastCompletion == nil {
		return ruleDeadline
	}
	return addMonthsClamped(*lastCompletion, months)
}

// addMonthsClamped aggiunge o sottrae mesi fermandosi all'ultimo giorno del
// mese di destinazione quando il giorno originale non esiste. Ogni calcolo
// parte dalla data ricevuta: 31 gennaio -> 28 febbraio -> 28 marzo.
func addMonthsClamped(value time.Time, months int) time.Time {
	value = dateOnly(value)
	targetMonth := time.Date(value.Year(), value.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	lastDay := time.Date(targetMonth.Year(), targetMonth.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	day := value.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(targetMonth.Year(), targetMonth.Month(), day, 0, 0, 0, 0, time.UTC)
}

// withinHorizon dice se una scadenza merita attenzione entro l'orizzonte,
// comprese le scadenze gia superate. days viene normalizzato come il
// parametro withinDays delle code (D4): valori <= 0 ricadono sul default,
// valori oltre il massimo vengono ridotti al massimo. Il giorno limite
// (asOf + days) e incluso.
func withinHorizon(deadline, asOf time.Time, days int) bool {
	if days <= 0 {
		days = defaultHorizonDays
	}
	if days > maxHorizonDays {
		days = maxHorizonDays
	}
	return !deadline.After(asOf.AddDate(0, 0, days))
}

// dateOnly azzera l'ora: i confronti di copertura lavorano su date pure.
func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// nullableDate passa una data opzionale come parametro ::date.
func nullableDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return dateOnly(*value)
}
