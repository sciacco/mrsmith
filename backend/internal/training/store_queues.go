package training

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Code operative derivate (#140, §4-Code). Sola lettura: ogni coda e una
// query sui fatti — nessuna tabella di to-do, nessuna scadenza automatica,
// nessuna scrittura.
//
// Conseguenza di D10: si caricano prima le regole attive, i limiti di
// calendario si calcolano in Go con le funzioni pure di store_coverage.go e
// le query ricevono i limiti gia calcolati come parametri. La semantica di
// platea, natura del bisogno, copertura e tornate resta nel nucleo condiviso
// (D3): le code la riusano senza duplicarla.

// queuePersonRowCap e il tetto delle righe per persona composte in Go (le
// code a query diretta hanno il loro LIMIT in SQL, stile pacchetto): le piu
// urgenti — ordinate per scadenza — restano, le altre vengono tagliate.
const queuePersonRowCap = 1000

// parseWithinDays normalizza il parametro query withinDays (D4): assente =
// default 60; ammessi i valori da 1 al massimo 365; altrimenti errore.
func parseWithinDays(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultHorizonDays, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil {
		return 0, validationError("invalid_within_days", "orizzonte in giorni non valido")
	}
	if days < 1 || days > maxHorizonDays {
		return 0, validationError("invalid_within_days", "orizzonte in giorni non valido: ammessi i valori da 1 a 365")
	}
	return days, nil
}

// daysBetween conta i giorni interi tra due date pure (negativo quando to
// precede from). Entrambe vengono normalizzate a mezzanotte UTC.
func daysBetween(from, to time.Time) int {
	return int(dateOnly(to).Sub(dateOnly(from)).Hours() / 24)
}

// queueRule e una regola attiva con i campi di presentazione delle code.
type queueRule struct {
	ruleFacts
	Name        string
	CourseTitle string
}

// activeQueueRules carica le regole attive con i fatti di copertura: le
// regole disattivate sono escluse da tutte le code. Ordine per scadenza
// (indice parziale idx_training_rule_active_deadline).
func (s *SQLStore) activeQueueRules(ctx context.Context) ([]queueRule, error) {
	const query = `
SELECT
  r.id::text,
  r.name,
  r.course_id::text,
  c.title,
  r.population_target,
  r.seat_count,
  r.deadline,
  r.recurrence_months,
  COALESCE(r.recurrence_anchor, ''),
  COALESCE(c.leads_to_cert_id::text, '')
FROM training.training_rule r
JOIN training.course c ON c.id = r.course_id
WHERE r.is_active
ORDER BY r.deadline, r.name, r.id
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list active training rules for queues: %w", err)
	}
	defer rows.Close()

	rules := make([]queueRule, 0)
	for rows.Next() {
		var (
			rule      queueRule
			rawTarget []byte
			seatCount sql.NullInt64
			months    sql.NullInt64
		)
		if err := rows.Scan(
			&rule.ID,
			&rule.Name,
			&rule.CourseID,
			&rule.CourseTitle,
			&rawTarget,
			&seatCount,
			&rule.Deadline,
			&months,
			&rule.RecurrenceAnchor,
			&rule.CertificationID,
		); err != nil {
			return nil, fmt.Errorf("scan active training rule for queues: %w", err)
		}
		rule.IsActive = true
		rule.Deadline = dateOnly(rule.Deadline)
		rule.SeatCount = nullInt(seatCount)
		rule.RecurrenceMonths = nullInt(months)
		rule.Target, err = parsePopulationTarget(rawTarget)
		if err != nil {
			return nil, err
		}
		rule.Need = needAttendance
		if rule.CertificationID != "" {
			rule.Need = needCertification
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// employeeNamesByID carica i nomi visualizzati di tutte le persone in una
// sola query: le code compongono le righe in Go e risolvono i nomi da qui.
func (s *SQLStore) employeeNamesByID(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, concat(last_name, ' ', first_name)
FROM training.employee
LIMIT 1000`)
	if err != nil {
		return nil, fmt.Errorf("load training employee names: %w", err)
	}
	defer rows.Close()

	names := make(map[string]string)
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan training employee name: %w", err)
		}
		names[id] = name
	}
	return names, rows.Err()
}

// teamLeads carica i lead attivi di un team (role='lead', end_date IS NULL):
// stessa definizione della validazione del parere TL (D5).
func (s *SQLStore) teamLeads(ctx context.Context, teamID string) ([]QueueLeadRef, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT tm.employee_id::text, concat(e.last_name, ' ', e.first_name), e.email::text
FROM training.team_membership tm
JOIN training.employee e ON e.id = tm.employee_id
WHERE tm.team_id = $1::uuid
  AND tm.role = 'lead'
  AND tm.end_date IS NULL
ORDER BY 2, 1
LIMIT 20`, teamID)
	if err != nil {
		return nil, fmt.Errorf("list training team leads: %w", err)
	}
	defer rows.Close()

	leads := make([]QueueLeadRef, 0)
	for rows.Next() {
		var lead QueueLeadRef
		if err := rows.Scan(&lead.EmployeeID, &lead.Name, &lead.Email); err != nil {
			return nil, fmt.Errorf("scan training team lead: %w", err)
		}
		leads = append(leads, lead)
	}
	return leads, rows.Err()
}

// ── Coda 1: richieste aperte senza parere TL ──

// QueueRequestsWithoutTLOpinion elenca le richieste aperte (outcome IS NULL,
// indice parziale idx_training_request_open) senza parere TL, con i lead del
// team scelto come contatto. Le piu vecchie prima.
func (s *SQLStore) QueueRequestsWithoutTLOpinion(ctx context.Context) ([]RequestWithoutTLOpinionRow, error) {
	const query = `
SELECT
  r.id::text,
  r.employee_id::text,
  concat(e.last_name, ' ', e.first_name),
  r.selected_team_id::text,
  t.name,
  COALESCE(r.course_id::text, ''),
  COALESCE(c.title, ''),
  r.created_at::date,
  r.created_at::text
FROM training.training_request r
JOIN training.employee e ON e.id = r.employee_id
JOIN training.team t ON t.id = r.selected_team_id
LEFT JOIN training.course c ON c.id = r.course_id
WHERE r.outcome IS NULL
  AND r.suspended_at IS NULL
  AND r.tl_opinion IS NULL
ORDER BY r.created_at, r.id
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list training requests without tl opinion: %w", err)
	}
	defer rows.Close()

	today := dateOnly(time.Now())
	result := make([]RequestWithoutTLOpinionRow, 0)
	for rows.Next() {
		var (
			row       RequestWithoutTLOpinionRow
			createdOn time.Time
		)
		if err := rows.Scan(
			&row.RequestID,
			&row.EmployeeID,
			&row.EmployeeName,
			&row.SelectedTeamID,
			&row.SelectedTeamName,
			&row.CourseID,
			&row.CourseTitle,
			&createdOn,
			&row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan training request without tl opinion: %w", err)
		}
		row.AgeDays = daysBetween(createdOn, today)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// I lead si risolvono per team, una volta sola per team.
	leadsByTeam := make(map[string][]QueueLeadRef)
	for i := range result {
		teamID := result[i].SelectedTeamID
		leads, ok := leadsByTeam[teamID]
		if !ok {
			leads, err = s.teamLeads(ctx, teamID)
			if err != nil {
				return nil, err
			}
			leadsByTeam[teamID] = leads
		}
		result[i].TeamLeads = leads
	}
	return result, nil
}

// ── Coda 2: richieste con parere TL e senza decisione People ──

// QueueRequestsAwaitingDecision elenca le richieste aperte con parere TL
// registrato e senza decisione People, con esito e motivazione del parere.
func (s *SQLStore) QueueRequestsAwaitingDecision(ctx context.Context) ([]RequestAwaitingDecisionRow, error) {
	const query = `
SELECT
  r.id::text,
  r.employee_id::text,
  concat(e.last_name, ' ', e.first_name),
  r.selected_team_id::text,
  t.name,
  COALESCE(r.course_id::text, ''),
  COALESCE(c.title, ''),
  r.tl_opinion,
  COALESCE(r.tl_opinion_by::text, ''),
  COALESCE(tl.last_name || ' ' || tl.first_name, ''),
  COALESCE(r.tl_opinion_at::text, ''),
  COALESCE(r.tl_opinion_reason, ''),
  r.created_at::date,
  r.created_at::text
FROM training.training_request r
JOIN training.employee e ON e.id = r.employee_id
JOIN training.team t ON t.id = r.selected_team_id
LEFT JOIN training.course c ON c.id = r.course_id
LEFT JOIN training.employee tl ON tl.id = r.tl_opinion_by
WHERE r.outcome IS NULL
  AND r.suspended_at IS NULL
  AND r.tl_opinion IS NOT NULL
  AND r.people_decision IS NULL
ORDER BY r.created_at, r.id
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list training requests awaiting decision: %w", err)
	}
	defer rows.Close()

	today := dateOnly(time.Now())
	result := make([]RequestAwaitingDecisionRow, 0)
	for rows.Next() {
		var (
			row       RequestAwaitingDecisionRow
			createdOn time.Time
		)
		if err := rows.Scan(
			&row.RequestID,
			&row.EmployeeID,
			&row.EmployeeName,
			&row.SelectedTeamID,
			&row.SelectedTeamName,
			&row.CourseID,
			&row.CourseTitle,
			&row.TLOpinion,
			&row.TLOpinionByID,
			&row.TLOpinionByName,
			&row.TLOpinionAt,
			&row.TLOpinionReason,
			&createdOn,
			&row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan training request awaiting decision: %w", err)
		}
		row.AgeDays = daysBetween(createdOn, today)
		result = append(result, row)
	}
	return result, rows.Err()
}

// ── Coda 3: regole attive a posizioni ──

// seatRuleInTraining elenca le iscrizioni collegate alla regola e non
// concluse (planned/in_progress): chi si sta formando sulle posizioni.
// Il collegamento e' diretto (source_rule_id sull'iscrizione) oppure per
// tornata: iscriversi a un evento-tornata della regola e' il gesto con cui
// People collega le posizioni, senza un'azione dedicata.
func (s *SQLStore) seatRuleInTraining(ctx context.Context, ruleID string) ([]SeatRuleInTrainingRow, error) {
	const query = `
SELECT
  en.id::text,
  en.employee_id::text,
  concat(e.last_name, ' ', e.first_name),
  en.event_id::text,
  en.delivery_status
FROM training.enrollment en
JOIN training.employee e ON e.id = en.employee_id
LEFT JOIN training.training_event ev ON ev.id = en.event_id
WHERE (en.source_rule_id = $1::uuid
   OR (ev.source_rule_id = $1::uuid AND ev.cancelled_at IS NULL))
  AND en.delivery_status IN ('planned', 'in_progress')
ORDER BY 3, en.id
LIMIT 300`
	rows, err := s.db.QueryContext(ctx, query, ruleID)
	if err != nil {
		return nil, fmt.Errorf("list training seat rule enrollments: %w", err)
	}
	defer rows.Close()

	result := make([]SeatRuleInTrainingRow, 0)
	for rows.Next() {
		var row SeatRuleInTrainingRow
		if err := rows.Scan(
			&row.EnrollmentID,
			&row.EmployeeID,
			&row.EmployeeName,
			&row.EventID,
			&row.DeliveryStatus,
		); err != nil {
			return nil, fmt.Errorf("scan training seat rule enrollment: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// QueueSeatRuleCoverage elenca le regole attive a posizioni con posizioni
// richieste, coperte secondo la natura del bisogno (nucleo condiviso D3/D4),
// mancanti e iscrizioni collegate in corso.
func (s *SQLStore) QueueSeatRuleCoverage(ctx context.Context) ([]SeatRuleCoverageRow, error) {
	rules, err := s.activeQueueRules(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]SeatRuleCoverageRow, 0)
	for _, rule := range rules {
		if rule.Target != nil {
			continue
		}
		seatCount := 0
		if rule.SeatCount != nil {
			seatCount = *rule.SeatCount
		}
		covered, err := s.coveredSeatCount(ctx, s.db, rule.ruleFacts)
		if err != nil {
			return nil, err
		}
		missing := seatCount - covered
		if missing < 0 {
			missing = 0
		}
		inTraining, err := s.seatRuleInTraining(ctx, rule.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, SeatRuleCoverageRow{
			RuleID:          rule.ID,
			RuleName:        rule.Name,
			CourseID:        rule.CourseID,
			CourseTitle:     rule.CourseTitle,
			Need:            rule.Need,
			CertificationID: rule.CertificationID,
			Deadline:        rule.Deadline.Format("2006-01-02"),
			SeatCount:       seatCount,
			Covered:         covered,
			Missing:         missing,
			InTraining:      inTraining,
		})
	}
	return result, nil
}

// ── Coda 4: coperture in scadenza ──

func expiringPersonRow(rule queueRule, employeeID string, names map[string]string, deadline, today time.Time, reason string) ExpiringPersonRow {
	return ExpiringPersonRow{
		RuleID:       rule.ID,
		RuleName:     rule.Name,
		CourseID:     rule.CourseID,
		CourseTitle:  rule.CourseTitle,
		Need:         rule.Need,
		EmployeeID:   employeeID,
		EmployeeName: names[employeeID],
		Deadline:     deadline.Format("2006-01-02"),
		DaysUntil:    daysBetween(today, deadline),
		Reason:       reason,
	}
}

// awardExpiry riassume i conseguimenti passed_exam di una persona per una
// certificazione: un award senza scadenza copre per sempre; altrimenti conta
// la scadenza massima.
type awardExpiry struct {
	NeverExpires bool
	MaxExpiresOn time.Time
}

// certificationAwardExpiries carica per persona in forza (status <>
// 'terminated') le scadenze dei conseguimenti passed_exam della
// certificazione, qualunque validation_source (le fonti esterne valgono).
// attendance_only non e un conseguimento e le prove fallite non compaiono
// nel registro (D4). Usa l'indice parziale idx_cert_award_passed_by_employee.
func (s *SQLStore) certificationAwardExpiries(ctx context.Context, certificationID string) (map[string]awardExpiry, error) {
	const query = `
SELECT
  ca.employee_id::text,
  bool_or(ca.expires_on IS NULL),
  MAX(ca.expires_on)
FROM training.certification_award ca
JOIN training.employee e ON e.id = ca.employee_id
WHERE ca.certification_id = $1::uuid
  AND ca.outcome = 'passed_exam'
  AND e.status <> 'terminated'
GROUP BY ca.employee_id`
	rows, err := s.db.QueryContext(ctx, query, certificationID)
	if err != nil {
		return nil, fmt.Errorf("load training certification award expiries: %w", err)
	}
	defer rows.Close()

	expiries := make(map[string]awardExpiry)
	for rows.Next() {
		var (
			employeeID   string
			neverExpires bool
			maxExpiresOn sql.NullTime
		)
		if err := rows.Scan(&employeeID, &neverExpires, &maxExpiresOn); err != nil {
			return nil, fmt.Errorf("scan training certification award expiry: %w", err)
		}
		entry := awardExpiry{NeverExpires: neverExpires}
		if !neverExpires && maxExpiresOn.Valid {
			entry.MaxExpiresOn = dateOnly(maxExpiresOn.Time)
		}
		expiries[employeeID] = entry
	}
	return expiries, rows.Err()
}

// expiringAttendancePeople calcola le persone della platea scoperte rispetto
// alla scadenza rilevante entro l'orizzonte, per una regola con bisogno di
// frequenza (D4): scadenza della regola senza ricorrenza, tornata in corso
// (o prima tornata) con ancora di calendario, scadenza personale da ultimo
// completamento con ancora completion.
func (s *SQLStore) expiringAttendancePeople(ctx context.Context, rule queueRule, names map[string]string, today time.Time, days int) ([]ExpiringPersonRow, error) {
	population, err := s.resolvePopulation(ctx, s.db, rule.ID, rule.Target)
	if err != nil {
		return nil, err
	}
	if len(population) == 0 {
		return nil, nil
	}

	if rule.RecurrenceMonths != nil && rule.RecurrenceAnchor == anchorCompletion {
		// Scadenze personali: ultimo completamento + mesi; mai completato ->
		// scadenza della regola. Il limite si calcola in Go (D10).
		completions, err := s.lastCompletionByEmployee(ctx, s.db, rule.CourseID)
		if err != nil {
			return nil, err
		}
		result := make([]ExpiringPersonRow, 0)
		for _, employeeID := range population {
			var last *time.Time
			reason := "never_completed"
			if value, ok := completions[employeeID]; ok {
				completedOn := value
				last = &completedOn
				reason = "personal_deadline"
			}
			deadline := personalDeadline(last, *rule.RecurrenceMonths, rule.Deadline)
			if !withinHorizon(deadline, today, days) {
				continue
			}
			result = append(result, expiringPersonRow(rule, employeeID, names, deadline, today, reason))
		}
		return result, nil
	}

	// Senza ricorrenza la scadenza rilevante e quella della regola; con
	// ancora di calendario e la tornata in corso, altrimenti la prima
	// tornata (= scadenza della regola, D4).
	deadline := rule.Deadline
	if rule.RecurrenceMonths != nil && rule.RecurrenceAnchor == anchorCalendar {
		round, err := s.currentRound(ctx, s.db, rule.ID)
		if err != nil {
			return nil, err
		}
		if round != nil {
			deadline = round.RuleDeadline
		}
	}
	if !withinHorizon(deadline, today, days) {
		return nil, nil
	}
	covered, err := s.coveredEmployees(ctx, s.db, rule.ruleFacts, &deadline)
	if err != nil {
		return nil, err
	}
	result := make([]ExpiringPersonRow, 0)
	for _, employeeID := range population {
		if covered[employeeID] {
			continue
		}
		result = append(result, expiringPersonRow(rule, employeeID, names, deadline, today, "uncovered"))
	}
	return result, nil
}

// expiringCertificationPeople calcola le persone della platea da rinnovare
// per una regola con bisogno di certificazione: award con scadenza entro
// l'orizzonte o gia scaduto, oppure mai conseguito rispetto alla scadenza
// della regola. Il rinnovo non usa la ricorrenza in mesi: le date sono le
// scadenze dei singoli conseguimenti (contratto #136).
func (s *SQLStore) expiringCertificationPeople(ctx context.Context, rule queueRule, names map[string]string, today time.Time, days int) ([]ExpiringPersonRow, error) {
	population, err := s.resolvePopulation(ctx, s.db, rule.ID, rule.Target)
	if err != nil {
		return nil, err
	}
	if len(population) == 0 {
		return nil, nil
	}
	expiries, err := s.certificationAwardExpiries(ctx, rule.CertificationID)
	if err != nil {
		return nil, err
	}
	result := make([]ExpiringPersonRow, 0)
	for _, employeeID := range population {
		info, ok := expiries[employeeID]
		if !ok {
			if withinHorizon(rule.Deadline, today, days) {
				result = append(result, expiringPersonRow(rule, employeeID, names, rule.Deadline, today, "never_awarded"))
			}
			continue
		}
		if info.NeverExpires {
			continue
		}
		if !withinHorizon(info.MaxExpiresOn, today, days) {
			continue
		}
		reason := "award_expiring"
		if !info.MaxExpiresOn.After(today) {
			reason = "award_expired"
		}
		result = append(result, expiringPersonRow(rule, employeeID, names, info.MaxExpiresOn, today, reason))
	}
	return result, nil
}

// QueueExpiringCoverage e la coda delle coperture in scadenza (D4, orizzonte
// withinDays). Frequenza (regole a platea): persone scoperte rispetto a
// scadenza o tornata, comprese le scadenze personali da ultimo completamento.
// Certificazione: a platea le persone da rinnovare; a posizioni le regole il
// cui conteggio valido scenderebbe sotto le posizioni entro l'orizzonte. Le
// regole a posizioni con bisogno di frequenza vivono nella coda
// seat-rule-coverage e non compaiono qui.
func (s *SQLStore) QueueExpiringCoverage(ctx context.Context, withinDaysRaw string) (ExpiringCoverageResponse, error) {
	days, err := parseWithinDays(withinDaysRaw)
	if err != nil {
		return ExpiringCoverageResponse{}, err
	}
	today := dateOnly(time.Now())
	horizon := today.AddDate(0, 0, days)
	response := ExpiringCoverageResponse{
		WithinDays: days,
		People:     make([]ExpiringPersonRow, 0),
		SeatRules:  make([]ExpiringSeatRuleRow, 0),
	}

	rules, err := s.activeQueueRules(ctx)
	if err != nil {
		return ExpiringCoverageResponse{}, err
	}
	if len(rules) == 0 {
		return response, nil
	}
	names, err := s.employeeNamesByID(ctx)
	if err != nil {
		return ExpiringCoverageResponse{}, err
	}

	for _, rule := range rules {
		switch {
		case rule.Need == needCertification && rule.Target != nil:
			people, err := s.expiringCertificationPeople(ctx, rule, names, today, days)
			if err != nil {
				if _, ok := asAppError(err); ok {
					// Platea non piu risolvibile (es. area rimasta senza
					// gruppo collegato): la coda resta consultabile, il
					// dettaglio regola espone l'errore esplicito.
					continue
				}
				return ExpiringCoverageResponse{}, err
			}
			response.People = append(response.People, people...)
		case rule.Need == needCertification:
			seatCount := 0
			if rule.SeatCount != nil {
				seatCount = *rule.SeatCount
			}
			validToday, err := s.certificationCoveredEmployees(ctx, s.db, rule.CertificationID, today)
			if err != nil {
				return ExpiringCoverageResponse{}, err
			}
			validAtHorizon, err := s.certificationCoveredEmployees(ctx, s.db, rule.CertificationID, horizon)
			if err != nil {
				return ExpiringCoverageResponse{}, err
			}
			if len(validAtHorizon) >= seatCount {
				continue
			}
			response.SeatRules = append(response.SeatRules, ExpiringSeatRuleRow{
				RuleID:          rule.ID,
				RuleName:        rule.Name,
				CourseID:        rule.CourseID,
				CourseTitle:     rule.CourseTitle,
				CertificationID: rule.CertificationID,
				SeatCount:       seatCount,
				ValidToday:      len(validToday),
				ValidAtHorizon:  len(validAtHorizon),
			})
		case rule.Target != nil:
			people, err := s.expiringAttendancePeople(ctx, rule, names, today, days)
			if err != nil {
				if _, ok := asAppError(err); ok {
					continue
				}
				return ExpiringCoverageResponse{}, err
			}
			response.People = append(response.People, people...)
		}
	}

	// Le righe piu urgenti prima; oltre il tetto restano solo quelle.
	sort.Slice(response.People, func(i, j int) bool {
		if response.People[i].Deadline != response.People[j].Deadline {
			return response.People[i].Deadline < response.People[j].Deadline
		}
		if response.People[i].RuleName != response.People[j].RuleName {
			return response.People[i].RuleName < response.People[j].RuleName
		}
		return response.People[i].EmployeeName < response.People[j].EmployeeName
	})
	if len(response.People) > queuePersonRowCap {
		response.People = response.People[:queuePersonRowCap]
	}
	return response, nil
}

// ── Coda 5: platea non alimentata ──

// QueueUnfedPopulation elenca, per le regole attive a platea con una tornata
// in corso, i membri attuali della platea non coperti e senza alcuna
// iscrizione sull'evento della tornata — anche annullata: il gesto non la
// scavalca. Sono gli stessi che il gesto di alimentazione aggiungerebbe (D7),
// calcolati con il nucleo condiviso e con lo stesso filtro del gesto.
func (s *SQLStore) QueueUnfedPopulation(ctx context.Context) ([]UnfedPopulationRow, error) {
	rules, err := s.activeQueueRules(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]UnfedPopulationRow, 0)
	if len(rules) == 0 {
		return result, nil
	}
	names, err := s.employeeNamesByID(ctx)
	if err != nil {
		return nil, err
	}

	for _, rule := range rules {
		if rule.Target == nil {
			continue
		}
		round, err := s.currentRound(ctx, s.db, rule.ID)
		if err != nil {
			return nil, err
		}
		if round == nil {
			continue
		}
		population, err := s.resolvePopulation(ctx, s.db, rule.ID, rule.Target)
		if err != nil {
			if _, ok := asAppError(err); ok {
				// Platea non piu risolvibile: vedi QueueExpiringCoverage.
				continue
			}
			return nil, err
		}
		covered, err := s.coveredEmployees(ctx, s.db, rule.ruleFacts, &round.RuleDeadline)
		if err != nil {
			return nil, err
		}
		enrolled, err := s.eventEnrolledEmployees(ctx, s.db, round.EventID)
		if err != nil {
			return nil, err
		}
		members := make([]QueueMemberRef, 0)
		for _, employeeID := range population {
			if covered[employeeID] || enrolled[employeeID] {
				continue
			}
			members = append(members, QueueMemberRef{EmployeeID: employeeID, Name: names[employeeID]})
		}
		if len(members) == 0 {
			continue
		}
		sort.Slice(members, func(i, j int) bool {
			if members[i].Name != members[j].Name {
				return members[i].Name < members[j].Name
			}
			return members[i].EmployeeID < members[j].EmployeeID
		})
		result = append(result, UnfedPopulationRow{
			RuleID:        rule.ID,
			RuleName:      rule.Name,
			CourseID:      rule.CourseID,
			CourseTitle:   rule.CourseTitle,
			EventID:       round.EventID,
			RoundDeadline: round.RuleDeadline.Format("2006-01-02"),
			Members:       members,
		})
	}
	return result, nil
}

// ── Coda 6: tornata in arrivo senza evento ──

// QueueRoundsWithoutEvent elenca le regole attive con ancora di calendario o
// senza ricorrenza la cui prossima (o prima) tornata cade entro l'orizzonte
// e non ha una tornata non annullata con quella rule_deadline. Le ancora
// completion sono escluse: non hanno una tornata condivisa (D4).
func (s *SQLStore) QueueRoundsWithoutEvent(ctx context.Context, withinDaysRaw string) (RoundsWithoutEventResponse, error) {
	days, err := parseWithinDays(withinDaysRaw)
	if err != nil {
		return RoundsWithoutEventResponse{}, err
	}
	today := dateOnly(time.Now())
	response := RoundsWithoutEventResponse{
		WithinDays: days,
		Rules:      make([]RoundWithoutEventRow, 0),
	}

	rules, err := s.activeQueueRules(ctx)
	if err != nil {
		return RoundsWithoutEventResponse{}, err
	}
	for _, rule := range rules {
		if rule.RecurrenceAnchor == anchorCompletion {
			continue
		}
		next, ok, err := s.nextRoundDeadline(ctx, s.db, rule.ruleFacts)
		if err != nil {
			return RoundsWithoutEventResponse{}, err
		}
		if !ok {
			// Regola senza ricorrenza con la prima tornata gia registrata:
			// non esiste una tornata successiva.
			continue
		}
		if !withinHorizon(next, today, days) {
			continue
		}
		round, err := s.currentRound(ctx, s.db, rule.ID)
		if err != nil {
			return RoundsWithoutEventResponse{}, err
		}
		// Guardia della definizione ratificata: nessuna tornata non annullata
		// con quella rule_deadline. Per costruzione la prossima scadenza
		// supera sempre la massima registrata (o non esistono tornate), ma il
		// confronto resta esplicito; le tornate annullate non sopprimono la
		// riga perche currentRound le ignora.
		if round != nil && round.RuleDeadline.Equal(next) {
			continue
		}
		response.Rules = append(response.Rules, RoundWithoutEventRow{
			RuleID:            rule.ID,
			RuleName:          rule.Name,
			CourseID:          rule.CourseID,
			CourseTitle:       rule.CourseTitle,
			Need:              rule.Need,
			RecurrenceMonths:  rule.RecurrenceMonths,
			RecurrenceAnchor:  rule.RecurrenceAnchor,
			NextRoundDeadline: next.Format("2006-01-02"),
			DaysUntil:         daysBetween(today, next),
			FirstRound:        round == nil,
		})
	}
	sort.Slice(response.Rules, func(i, j int) bool {
		if response.Rules[i].NextRoundDeadline != response.Rules[j].NextRoundDeadline {
			return response.Rules[i].NextRoundDeadline < response.Rules[j].NextRoundDeadline
		}
		if response.Rules[i].RuleName != response.Rules[j].RuleName {
			return response.Rules[i].RuleName < response.Rules[j].RuleName
		}
		return response.Rules[i].RuleID < response.Rules[j].RuleID
	})
	return response, nil
}

// QueueUnapprovedEventExpenses lists every local event expense candidate.
// Callers must hydrate and classify its PO live before exposing the queue.
func (s *SQLStore) QueueUnapprovedEventExpenses(ctx context.Context) ([]unapprovedEventExpenseLocal, error) {
	const query = `
SELECT
  ee.id::text,
  ee.event_id::text,
  ee.rda_id,
  ee.created_at::text,
  ee.updated_at::text,
  ev.course_id::text,
  c.title,
  COUNT(eee.enrollment_id)
FROM training.event_expense ee
JOIN training.training_event ev ON ev.id = ee.event_id
JOIN training.course c ON c.id = ev.course_id
LEFT JOIN training.event_expense_enrollment eee ON eee.expense_id = ee.id
GROUP BY ee.id, ev.id, c.id
ORDER BY ee.created_at, ee.id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list training unapproved event expense candidates: %w", err)
	}
	defer rows.Close()

	result := make([]unapprovedEventExpenseLocal, 0)
	for rows.Next() {
		var row unapprovedEventExpenseLocal
		if err := rows.Scan(
			&row.Expense.ID,
			&row.Expense.EventID,
			&row.Expense.POID,
			&row.Expense.CreatedAt,
			&row.Expense.UpdatedAt,
			&row.CourseID,
			&row.CourseTitle,
			&row.EnrollmentCount,
		); err != nil {
			return nil, fmt.Errorf("scan training unapproved event expense candidate: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// ── Coda 7: iscrizioni "planned" ferme (#152, slice 1 del task 6) ──

// parseOlderThanDays normalizza il parametro query olderThanDays: assente =
// default 30; ammessi i valori da 1 al massimo 365 (stesso tetto di D4).
func parseOlderThanDays(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 30, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil {
		return 0, validationError("invalid_older_than_days", "soglia in giorni non valida")
	}
	if days < 1 || days > maxHorizonDays {
		return 0, validationError("invalid_older_than_days", "soglia in giorni non valida: ammessi i valori da 1 a 365")
	}
	return days, nil
}

// QueueStaleEnrollments elenca le iscrizioni pianificate (delivery_status
// 'planned') su un evento non annullato, create prima della soglia e senza
// un'assegnazione a una sessione con data futura (starts_at o due_at): le
// piu vecchie prima.
func (s *SQLStore) QueueStaleEnrollments(ctx context.Context, olderThanDaysRaw string) (StaleEnrollmentsResponse, error) {
	days, err := parseOlderThanDays(olderThanDaysRaw)
	if err != nil {
		return StaleEnrollmentsResponse{}, err
	}
	const query = `
SELECT
  en.id::text,
  e.id::text,
  concat(e.last_name, ' ', e.first_name),
  en.event_id::text,
  c.title,
  en.created_at::date,
  en.created_at::text,
  (
    SELECT MAX(COALESCE(s.starts_at, s.due_at))::text
    FROM training.enrollment_session es
    JOIN training.training_session s ON s.id = es.session_id
    WHERE es.enrollment_id = en.id
      AND COALESCE(s.starts_at, s.due_at) <= now()
  )
FROM training.enrollment en
JOIN training.employee e ON e.id = en.employee_id
JOIN training.training_event ev ON ev.id = en.event_id
JOIN training.course c ON c.id = ev.course_id
WHERE en.delivery_status = 'planned'
  AND ev.cancelled_at IS NULL
  AND en.created_at < now() - ($1::int * interval '1 day')
  AND NOT EXISTS (
    SELECT 1
    FROM training.enrollment_session es2
    JOIN training.training_session s2 ON s2.id = es2.session_id
    WHERE es2.enrollment_id = en.id
      AND COALESCE(s2.starts_at, s2.due_at) > now()
  )
ORDER BY en.created_at ASC, en.id
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, query, days)
	if err != nil {
		return StaleEnrollmentsResponse{}, fmt.Errorf("list training stale enrollments: %w", err)
	}
	defer rows.Close()

	today := dateOnly(time.Now())
	response := StaleEnrollmentsResponse{
		OlderThanDays: days,
		Enrollments:   make([]StaleEnrollmentRow, 0),
	}
	for rows.Next() {
		var (
			row         StaleEnrollmentRow
			createdOn   time.Time
			lastSession sql.NullString
		)
		if err := rows.Scan(
			&row.EnrollmentID, &row.EmployeeID, &row.EmployeeName, &row.EventID, &row.CourseTitle,
			&createdOn, &row.CreatedAt, &lastSession,
		); err != nil {
			return StaleEnrollmentsResponse{}, fmt.Errorf("scan training stale enrollment: %w", err)
		}
		row.AgeDays = daysBetween(createdOn, today)
		if lastSession.Valid {
			value := lastSession.String
			row.LastSessionDate = &value
		}
		response.Enrollments = append(response.Enrollments, row)
	}
	return response, rows.Err()
}

// ── Coda 8: certificazioni in scadenza ──

// defaultExpiringCertificationsWithinDays e l'orizzonte di default della
// coda 8 (#160, slice 1 del task 7): diverso dal default di D4
// (defaultHorizonDays, coda 4), il tetto massimo resta lo stesso (maxHorizonDays).
const defaultExpiringCertificationsWithinDays = 90

// parseExpiringCertificationsWithinDays normalizza il parametro query
// withinDays della coda 8: assente = default 90; ammessi i valori da 1 al
// massimo 365.
func parseExpiringCertificationsWithinDays(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultExpiringCertificationsWithinDays, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil {
		return 0, validationError("invalid_within_days", "orizzonte in giorni non valido")
	}
	if days < 1 || days > maxHorizonDays {
		return 0, validationError("invalid_within_days", "orizzonte in giorni non valido: ammessi i valori da 1 a 365")
	}
	return days, nil
}

// QueueExpiringCertifications elenca i conseguimenti passed_exam di persone
// attive con scadenza entro l'orizzonte (coda 8, #160): wrapper sulla vista
// viva v_expiring_certifications, che gia' applica outcome, scadenza e
// stato persona. award_id non e' nella vista, si recupera unendo
// certification_award sulla stessa combinazione (persona, certificazione,
// scadenza) che ha prodotto la riga.
func (s *SQLStore) QueueExpiringCertifications(ctx context.Context, withinDaysRaw string) (ExpiringCertificationsResponse, error) {
	days, err := parseExpiringCertificationsWithinDays(withinDaysRaw)
	if err != nil {
		return ExpiringCertificationsResponse{}, err
	}
	const q = `
SELECT DISTINCT
  ca.id::text,
  v.employee_id::text,
  concat(v.last_name, ' ', v.first_name),
  v.email::text,
  v.cert_code,
  v.cert_name,
  v.expires_on::text,
  v.days_to_expiry::int
FROM training.v_expiring_certifications v
JOIN training.certification c ON c.code = v.cert_code
JOIN training.certification_award ca
  ON ca.employee_id = v.employee_id
 AND ca.certification_id = c.id
 AND ca.expires_on = v.expires_on
 AND ca.outcome = 'passed_exam'
WHERE v.days_to_expiry <= $1
ORDER BY 8, 3
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q, days)
	if err != nil {
		return ExpiringCertificationsResponse{}, fmt.Errorf("list expiring certifications queue: %w", err)
	}
	defer rows.Close()

	response := ExpiringCertificationsResponse{WithinDays: days, Certifications: make([]ExpiringCertificationQueueRow, 0)}
	for rows.Next() {
		var row ExpiringCertificationQueueRow
		if err := rows.Scan(
			&row.AwardID, &row.EmployeeID, &row.EmployeeName, &row.EmployeeEmail,
			&row.CertificationCode, &row.CertificationName, &row.ExpiresOn, &row.DaysToExpiry,
		); err != nil {
			return ExpiringCertificationsResponse{}, fmt.Errorf("scan expiring certifications queue: %w", err)
		}
		response.Certifications = append(response.Certifications, row)
	}
	return response, rows.Err()
}
