package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Richieste formative (#140, §4-Richieste, decisioni D5 e D8).
//
// I dati originali della richiesta sono create-only: nessuna API li modifica
// e nessun UPDATE li include. Parere TL e decisione People sono fatti
// immutabili; sequenzialita e override vivono soltanto in
// requestDecisionPolicy; l'accoglimento e una transazione unica.

const (
	requestActionTLOpinion = "tl_opinion"
	requestActionDecision  = "decision"
	requestActionWithdraw  = "withdraw"

	requestOpinionFavorable   = "favorable"
	requestOpinionUnfavorable = "unfavorable"

	requestDecisionAccepted = "accepted"
	requestDecisionRejected = "rejected"

	requestOutcomeWithdrawn = "withdrawn"
)

// requestPolicyFacts sono i fatti decisionali di una richiesta, letti con
// lock: campi vuoti = fatto assente.
type requestPolicyFacts struct {
	ID             string
	EmployeeID     string
	SelectedTeamID string
	Outcome        string
	TLOpinion      string
	PeopleDecision string
}

// requestDecisionPolicy e l'UNICO punto che codifica la sequenzialita e
// l'override del workflow richieste (D5): cambiare la policy non riscrive i
// fatti storici gia registrati.
//
// Policy corrente:
//   - ogni azione e ammessa solo su richiesta aperta (esito assente);
//   - parere TL una sola volta;
//   - decisione People una sola volta e solo con parere TL presente
//     (sequenzialita iniziale);
//   - accoglimento con parere sfavorevole = override, ammesso: la motivazione
//     obbligatoria della decisione e la motivazione dell'override, nessun
//     campo aggiuntivo;
//   - ritiro ammesso finche la richiesta e aperta, senza toccare i campi del
//     parere e della decisione.
func requestDecisionPolicy(facts requestPolicyFacts, action string) error {
	if facts.Outcome != "" {
		return conflictError("request_closed", "la richiesta e gia chiusa")
	}
	switch action {
	case requestActionTLOpinion:
		if facts.TLOpinion != "" {
			return conflictError("tl_opinion_already_recorded", "parere TL gia registrato")
		}
	case requestActionDecision:
		if facts.PeopleDecision != "" {
			return conflictError("decision_already_recorded", "decisione People gia registrata")
		}
		if facts.TLOpinion == "" {
			return conflictError("tl_opinion_required", "decisione ammessa solo con il parere TL registrato")
		}
	case requestActionWithdraw:
		// Nessun vincolo oltre alla richiesta aperta.
	default:
		return fmt.Errorf("unknown training request action: %s", action)
	}
	return nil
}

// lockRequestFacts carica i fatti decisionali della richiesta con FOR UPDATE:
// serializza parere, decisione e ritiro concorrenti.
func (s *SQLStore) lockRequestFacts(ctx context.Context, tx *sql.Tx, id string) (requestPolicyFacts, error) {
	const q = `
SELECT
  r.id::text,
  r.employee_id::text,
  r.selected_team_id::text,
  COALESCE(r.outcome, ''),
  COALESCE(r.tl_opinion, ''),
  COALESCE(r.people_decision, '')
FROM training.training_request r
WHERE r.id = $1::uuid
FOR UPDATE`
	var facts requestPolicyFacts
	err := tx.QueryRowContext(ctx, q, id).Scan(
		&facts.ID,
		&facts.EmployeeID,
		&facts.SelectedTeamID,
		&facts.Outcome,
		&facts.TLOpinion,
		&facts.PeopleDecision,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return requestPolicyFacts{}, notFoundError("request_not_found", "richiesta non trovata")
	}
	if err != nil {
		return requestPolicyFacts{}, fmt.Errorf("load training request facts: %w", err)
	}
	return facts, nil
}

// ensureSelectedTeamMembership verifica che il team scelto sia tra le
// appartenenze attive della persona (membership attiva = end_date IS NULL).
func (s *SQLStore) ensureSelectedTeamMembership(ctx context.Context, q sqlRunner, employeeID, teamID string) error {
	var ok bool
	err := q.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM training.team_membership tm
  WHERE tm.employee_id = $1::uuid
    AND tm.team_id = $2::uuid
    AND tm.end_date IS NULL
)`, employeeID, teamID).Scan(&ok)
	if err != nil {
		return fmt.Errorf("check training request team membership: %w", err)
	}
	if !ok {
		return validationError("selected_team_invalid", "il team scelto non e tra le appartenenze attive della persona")
	}
	return nil
}

// ensureLeadOfSelectedTeam verifica che chi esprime il parere sia un lead
// attivo del team scelto sulla richiesta (role='lead', end_date IS NULL).
func (s *SQLStore) ensureLeadOfSelectedTeam(ctx context.Context, q sqlRunner, leadEmployeeID, teamID string) error {
	var ok bool
	err := q.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM training.team_membership tm
  WHERE tm.employee_id = $1::uuid
    AND tm.team_id = $2::uuid
    AND tm.role = 'lead'
    AND tm.end_date IS NULL
)`, leadEmployeeID, teamID).Scan(&ok)
	if err != nil {
		return fmt.Errorf("check training request team lead: %w", err)
	}
	if !ok {
		return validationError("lead_not_of_team", "la persona indicata non e un lead attivo del team scelto")
	}
	return nil
}

// ── Letture ──

func (s *SQLStore) ListRequests(ctx context.Context, state string) ([]RequestListRow, error) {
	state = strings.TrimSpace(state)
	if state == "" {
		state = "open"
	}
	switch state {
	case "open", "suspended", "closed", "all":
	default:
		return nil, validationError("invalid_state", "filtro stato non valido")
	}
	// La sospesa esce dalla vista operativa di default (open) e ha il suo
	// filtro dedicato; closed e all restano invariati.
	const q = `
SELECT
  r.id::text,
  r.employee_id::text,
  concat(e.last_name, ' ', e.first_name),
  e.email::text,
  COALESCE(r.course_id::text, ''),
  COALESCE(c.title, ''),
  COALESCE((
    SELECT json_agg(json_build_object('id', sa.id::text, 'name', sa.name) ORDER BY sa.name)
    FROM training.training_request_skill_area rsa
    JOIN training.skill_area sa ON sa.id = rsa.skill_area_id
    WHERE rsa.request_id = r.id
  ), '[]')::text,
  r.selected_team_id::text,
  t.name,
  r.priority,
  COALESCE(r.reminder_text, ''),
  COALESCE(r.reminder_at::text, ''),
  COALESCE(r.suspended_at::text, ''),
  COALESCE(r.tl_opinion, ''),
  COALESCE(r.people_decision, ''),
  COALESCE(r.outcome, ''),
  COALESCE(r.closed_at::text, ''),
  r.created_at::text
FROM training.training_request r
JOIN training.employee e ON e.id = r.employee_id
JOIN training.team t ON t.id = r.selected_team_id
LEFT JOIN training.course c ON c.id = r.course_id
WHERE ($1 = 'all')
   OR ($1 = 'open' AND r.outcome IS NULL AND r.suspended_at IS NULL)
   OR ($1 = 'suspended' AND r.outcome IS NULL AND r.suspended_at IS NOT NULL)
   OR ($1 = 'closed' AND r.outcome IS NOT NULL)
ORDER BY r.priority NULLS LAST, r.created_at DESC, r.id
LIMIT 1000`
	rows, err := s.db.QueryContext(ctx, q, state)
	if err != nil {
		return nil, fmt.Errorf("list training requests: %w", err)
	}
	defer rows.Close()

	result := make([]RequestListRow, 0)
	for rows.Next() {
		var row RequestListRow
		var areasRaw string
		var priority sql.NullInt64
		if err := rows.Scan(
			&row.ID,
			&row.EmployeeID,
			&row.EmployeeName,
			&row.EmployeeEmail,
			&row.CourseID,
			&row.CourseTitle,
			&areasRaw,
			&row.SelectedTeamID,
			&row.SelectedTeam,
			&priority,
			&row.ReminderText,
			&row.ReminderAt,
			&row.SuspendedAt,
			&row.TLOpinion,
			&row.PeopleDecision,
			&row.Outcome,
			&row.ClosedAt,
			&row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan training request: %w", err)
		}
		if err := json.Unmarshal([]byte(areasRaw), &row.SkillAreas); err != nil {
			return nil, fmt.Errorf("decode training request skill areas: %w", err)
		}
		row.Priority = nullInt(priority)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) GetRequestDetail(ctx context.Context, id string) (RequestDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return RequestDetail{}, validationError("missing_id", "id richiesta obbligatorio")
	}
	const q = `
SELECT
  r.id::text,
  r.employee_id::text,
  concat(e.last_name, ' ', e.first_name),
  e.email::text,
  COALESCE(r.course_id::text, ''),
  COALESCE(c.title, ''),
  COALESCE((
    SELECT json_agg(json_build_object(
      'id', sa.id::text, 'name', sa.name,
      'levelCurrent', rsa.level_current, 'levelTarget', rsa.level_target
    ) ORDER BY sa.name)
    FROM training.training_request_skill_area rsa
    JOIN training.skill_area sa ON sa.id = rsa.skill_area_id
    WHERE rsa.request_id = r.id
  ), '[]')::text,
  r.motivation,
  r.selected_team_id::text,
  t.name,
  COALESCE(r.desired_start::text, ''),
  COALESCE(r.desired_end::text, ''),
  r.priority,
  COALESCE(r.notes, ''),
  COALESCE(r.reminder_text, ''),
  COALESCE(r.reminder_at::text, ''),
  COALESCE(r.suspended_at::text, ''),
  COALESCE(sb.last_name || ' ' || sb.first_name, ''),
  COALESCE(r.suspension_reason, ''),
  COALESCE(r.tl_opinion, ''),
  COALESCE(r.tl_opinion_by::text, ''),
  COALESCE(tl.last_name || ' ' || tl.first_name, ''),
  COALESCE(r.tl_opinion_at::text, ''),
  COALESCE(r.tl_opinion_reason, ''),
  COALESCE(r.people_decision, ''),
  COALESCE(r.people_decision_by::text, ''),
  COALESCE(pd.last_name || ' ' || pd.first_name, ''),
  COALESCE(r.people_decision_at::text, ''),
  COALESCE(r.people_decision_reason, ''),
  COALESCE(r.outcome, ''),
  COALESCE(r.closed_at::text, ''),
  COALESCE(r.accepted_course_id::text, ''),
  COALESCE(ac.title, ''),
  COALESCE(r.accepted_event_id::text, ''),
  COALESCE(r.accepted_vendor_id::text, ''),
  COALESCE(v.name, ''),
  COALESCE(r.accepted_period_start::text, ''),
  COALESCE(r.accepted_period_end::text, ''),
  COALESCE(r.accepted_notes, ''),
  COALESCE(r.resulting_enrollment_id::text, ''),
  r.created_at::text,
  r.updated_at::text
FROM training.training_request r
JOIN training.employee e ON e.id = r.employee_id
JOIN training.team t ON t.id = r.selected_team_id
LEFT JOIN training.course c ON c.id = r.course_id
LEFT JOIN training.employee sb ON sb.id = r.suspended_by
LEFT JOIN training.employee tl ON tl.id = r.tl_opinion_by
LEFT JOIN training.employee pd ON pd.id = r.people_decision_by
LEFT JOIN training.course ac ON ac.id = r.accepted_course_id
LEFT JOIN training.vendor v ON v.id = r.accepted_vendor_id
WHERE r.id = $1::uuid`
	var (
		detail            RequestDetail
		tlOpinion         RequestTLOpinionFacts
		decision          RequestDecisionFacts
		accepted          RequestAcceptedData
		hasTLOpinion      string
		hasDecision       string
		acceptedCourse    string
		requestedAreasRaw string
		priority          sql.NullInt64
	)
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&detail.ID,
		&detail.Requested.EmployeeID,
		&detail.Requested.EmployeeName,
		&detail.Requested.EmployeeEmail,
		&detail.Requested.CourseID,
		&detail.Requested.CourseTitle,
		&requestedAreasRaw,
		&detail.Requested.Motivation,
		&detail.Requested.SelectedTeamID,
		&detail.Requested.SelectedTeamName,
		&detail.Requested.DesiredStart,
		&detail.Requested.DesiredEnd,
		&priority,
		&detail.Notes,
		&detail.ReminderText,
		&detail.ReminderAt,
		&detail.SuspendedAt,
		&detail.SuspendedByName,
		&detail.SuspensionReason,
		&hasTLOpinion,
		&tlOpinion.ByEmployeeID,
		&tlOpinion.ByName,
		&tlOpinion.At,
		&tlOpinion.Reason,
		&hasDecision,
		&decision.ByEmployeeID,
		&decision.ByName,
		&decision.At,
		&decision.Reason,
		&detail.Outcome,
		&detail.ClosedAt,
		&acceptedCourse,
		&accepted.CourseTitle,
		&accepted.EventID,
		&accepted.VendorID,
		&accepted.VendorName,
		&accepted.PeriodStart,
		&accepted.PeriodEnd,
		&accepted.Notes,
		&detail.ResultingEnrollmentID,
		&detail.CreatedAt,
		&detail.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RequestDetail{}, notFoundError("request_not_found", "richiesta non trovata")
	}
	if err != nil {
		return RequestDetail{}, fmt.Errorf("load training request detail: %w", err)
	}
	if err := json.Unmarshal([]byte(requestedAreasRaw), &detail.Requested.SkillAreas); err != nil {
		return RequestDetail{}, fmt.Errorf("decode training request skill areas: %w", err)
	}
	detail.Priority = nullInt(priority)
	if hasTLOpinion != "" {
		tlOpinion.Opinion = hasTLOpinion
		detail.TLOpinion = &tlOpinion
	}
	if hasDecision != "" {
		decision.Decision = hasDecision
		detail.Decision = &decision
	}
	if acceptedCourse != "" {
		accepted.CourseID = acceptedCourse
		detail.Accepted = &accepted
	}

	// La copertura esistente si calcola sul corso accettato quando presente,
	// altrimenti sul corso richiesto.
	coverageCourseID := detail.Requested.CourseID
	coverageCourseTitle := detail.Requested.CourseTitle
	if acceptedCourse != "" {
		coverageCourseID = acceptedCourse
		coverageCourseTitle = accepted.CourseTitle
	}
	detail.ExistingCoverage, err = s.requestExistingCoverage(ctx, detail.Requested.EmployeeID, coverageCourseID, coverageCourseTitle)
	if err != nil {
		return RequestDetail{}, err
	}
	return detail, nil
}

// requestExistingCoverage carica la copertura esistente della persona per il
// corso considerato (D8): iscrizioni completed su eventi del corso, qualunque
// origine, e award validi (passed_exam, non scaduti) della certificazione
// collegata al corso.
func (s *SQLStore) requestExistingCoverage(ctx context.Context, employeeID, courseID, courseTitle string) (RequestCoverage, error) {
	coverage := RequestCoverage{
		CompletedEnrollments: make([]RequestCoverageEnrollment, 0),
		ValidAwards:          make([]RequestCoverageAward, 0),
	}
	if courseID == "" {
		return coverage, nil
	}
	coverage.CourseID = courseID
	coverage.CourseTitle = courseTitle

	const enrollmentsQuery = `
SELECT
  en.id::text,
  en.event_id::text,
  en.origin,
  COALESCE(en.actual_end::text, en.updated_at::date::text)
FROM training.enrollment en
JOIN training.training_event ev ON ev.id = en.event_id
WHERE en.employee_id = $1::uuid
  AND ev.course_id = $2::uuid
  AND en.delivery_status = 'completed'
ORDER BY COALESCE(en.actual_end, en.updated_at::date) DESC, en.id
LIMIT 100`
	rows, err := s.db.QueryContext(ctx, enrollmentsQuery, employeeID, courseID)
	if err != nil {
		return coverage, fmt.Errorf("load training request coverage enrollments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row RequestCoverageEnrollment
		if err := rows.Scan(&row.EnrollmentID, &row.EventID, &row.Origin, &row.CompletedOn); err != nil {
			return coverage, fmt.Errorf("scan training request coverage enrollment: %w", err)
		}
		coverage.CompletedEnrollments = append(coverage.CompletedEnrollments, row)
	}
	if err := rows.Err(); err != nil {
		return coverage, err
	}

	const awardsQuery = `
SELECT
  ca.id::text,
  ca.certification_id::text,
  cert.name,
  ca.awarded_on::text,
  COALESCE(ca.expires_on::text, ''),
  ca.validation_source::text
FROM training.course co
JOIN training.certification_award ca ON ca.certification_id = co.leads_to_cert_id
JOIN training.certification cert ON cert.id = ca.certification_id
WHERE co.id = $2::uuid
  AND ca.employee_id = $1::uuid
  AND ca.outcome = 'passed_exam'
  AND (ca.expires_on IS NULL OR ca.expires_on > CURRENT_DATE)
ORDER BY ca.awarded_on DESC, ca.id
LIMIT 100`
	awardRows, err := s.db.QueryContext(ctx, awardsQuery, employeeID, courseID)
	if err != nil {
		return coverage, fmt.Errorf("load training request coverage awards: %w", err)
	}
	defer awardRows.Close()
	for awardRows.Next() {
		var row RequestCoverageAward
		if err := awardRows.Scan(
			&row.AwardID,
			&row.CertificationID,
			&row.CertificationName,
			&row.AwardedOn,
			&row.ExpiresOn,
			&row.ValidationSource,
		); err != nil {
			return coverage, fmt.Errorf("scan training request coverage award: %w", err)
		}
		coverage.ValidAwards = append(coverage.ValidAwards, row)
	}
	return coverage, awardRows.Err()
}

// ── Mutazioni ──

// resolveOrCreateEmbryoCourse aggancia un titolo a un corso: riusa il corso
// esistente con lo stesso titolo (confronto case-insensitive), altrimenti
// crea l'embrione con il solo nome e lo registra nell'audit.
func (s *SQLStore) resolveOrCreateEmbryoCourse(ctx context.Context, tx *sql.Tx, principal Principal, title string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM training.course
WHERE lower(title) = lower($1)
ORDER BY created_at, id
LIMIT 1`, title).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("resolve course by title: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `
INSERT INTO training.course (title)
VALUES ($1)
RETURNING id::text`, title).Scan(&id); err != nil {
		return "", fmt.Errorf("create embryo course: %w", err)
	}
	after, err := entitySnapshot(ctx, tx, "course", id)
	if err != nil {
		return "", err
	}
	if err := s.audit(ctx, tx, principal, "course", id, "create", nil, after); err != nil {
		return "", err
	}
	return id, nil
}

func (s *SQLStore) CreateRequest(ctx context.Context, principal Principal, input RequestInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	employeeID := strings.TrimSpace(input.EmployeeID)
	if employeeID == "" {
		return ActionResponse{}, validationError("employee_required", "persona obbligatoria")
	}
	courseID := strings.TrimSpace(input.CourseID)
	newCourseTitle := strings.TrimSpace(input.NewCourseTitle)
	if courseID == "" && newCourseTitle == "" {
		return ActionResponse{}, validationError("course_or_title_required", "indicare il corso a catalogo oppure il titolo del corso da creare")
	}
	if courseID != "" && newCourseTitle != "" {
		return ActionResponse{}, validationError("course_xor_title", "corso a catalogo e titolo nuovo sono alternativi")
	}
	motivation := strings.TrimSpace(input.Motivation)
	if motivation == "" {
		return ActionResponse{}, validationError("motivation_required", "motivazione obbligatoria")
	}
	teamID := strings.TrimSpace(input.SelectedTeamID)
	if teamID == "" {
		return ActionResponse{}, validationError("selected_team_required", "team obbligatorio")
	}
	desiredStart, err := parseOptionalDate(input.DesiredStart)
	if err != nil {
		return ActionResponse{}, validationError("invalid_desired_start", "data inizio desiderata non valida")
	}
	desiredEnd, err := parseOptionalDate(input.DesiredEnd)
	if err != nil {
		return ActionResponse{}, validationError("invalid_desired_end", "data fine desiderata non valida")
	}
	if desiredStart != nil && desiredEnd != nil && desiredEnd.Before(*desiredStart) {
		return ActionResponse{}, validationError("desired_end_before_start", "la fine desiderata non puo precedere l'inizio")
	}
	if input.Priority != nil && *input.Priority < 1 {
		return ActionResponse{}, validationError("invalid_priority", "priorita non valida: 1 = piu importante")
	}
	reminderAt, err := parseOptionalDate(input.ReminderAt)
	if err != nil {
		return ActionResponse{}, validationError("invalid_reminder_at", "data di richiamo non valida")
	}
	areas, err := s.normalizeRequestAreas(ctx, input.SkillAreas)
	if err != nil {
		return ActionResponse{}, err
	}

	var response ActionResponse
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.ensureEmployeeActive(ctx, tx, employeeID); err != nil {
			return err
		}
		if courseID != "" {
			if err := s.ensureCourseExists(ctx, tx, courseID); err != nil {
				return err
			}
		} else {
			// Un titolo e l'embrione di un corso: si riusa il corso con lo
			// stesso titolo se esiste, altrimenti nasce l'embrione (solo nome;
			// la completezza della scheda arriva con la maturazione).
			id, err := s.resolveOrCreateEmbryoCourse(ctx, tx, principal, newCourseTitle)
			if err != nil {
				return err
			}
			courseID = id
		}
		if err := s.ensureSelectedTeamMembership(ctx, tx, employeeID, teamID); err != nil {
			return err
		}

		const stmt = `
INSERT INTO training.training_request (
  employee_id,
  course_id,
  motivation,
  selected_team_id,
  desired_start,
  desired_end,
  priority,
  notes,
  reminder_text,
  reminder_at
) VALUES (
  $1::uuid,
  $2::uuid,
  $3,
  $4::uuid,
  NULLIF($5, '')::date,
  NULLIF($6, '')::date,
  $7,
  NULLIF($8, ''),
  NULLIF($9, ''),
  $10::date
)
RETURNING id::text`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			employeeID,
			courseID,
			motivation,
			teamID,
			strings.TrimSpace(input.DesiredStart),
			strings.TrimSpace(input.DesiredEnd),
			input.Priority,
			strings.TrimSpace(input.Notes),
			strings.TrimSpace(input.ReminderText),
			reminderAt,
		).Scan(&response.ID); err != nil {
			return fmt.Errorf("create training request: %w", err)
		}
		if err := replaceRequestSkillAreas(ctx, tx, response.ID, areas); err != nil {
			return err
		}
		after, err := entitySnapshot(ctx, tx, "training_request", response.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_request", response.ID, "create", nil, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) RecordTLOpinion(ctx context.Context, principal Principal, id string, input TLOpinionInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id richiesta obbligatorio")
	}
	leadID := strings.TrimSpace(input.LeadEmployeeID)
	if leadID == "" {
		return ActionResponse{}, validationError("lead_required", "lead obbligatorio")
	}
	opinion := strings.TrimSpace(input.Opinion)
	if opinion != requestOpinionFavorable && opinion != requestOpinionUnfavorable {
		return ActionResponse{}, validationError("invalid_opinion", "esito del parere non valido")
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return ActionResponse{}, validationError("reason_required", "motivazione obbligatoria")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		facts, err := s.lockRequestFacts(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := requestDecisionPolicy(facts, requestActionTLOpinion); err != nil {
			return err
		}
		if err := s.ensureLeadOfSelectedTeam(ctx, tx, leadID, facts.SelectedTeamID); err != nil {
			return err
		}
		before, err := entitySnapshot(ctx, tx, "training_request", id)
		if err != nil {
			return err
		}
		// tl_opinion_by e il lead validato, non l'operatore People: l'attore
		// della registrazione resta nell'audit.
		if _, err := tx.ExecContext(ctx, `
UPDATE training.training_request
SET tl_opinion = $2,
    tl_opinion_by = $3::uuid,
    tl_opinion_at = now(),
    tl_opinion_reason = $4,
    updated_at = now()
WHERE id = $1::uuid`, id, opinion, leadID, reason); err != nil {
			return fmt.Errorf("record training request tl opinion: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "training_request", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_request", id, "tl_opinion", before, after); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id, Status: opinion}
		return nil
	})
	return response, err
}

// normalizedAcceptance sono i dati di accoglimento gia normalizzati e
// validati nella forma (D8).
type normalizedAcceptance struct {
	CourseID             string
	EventID              string
	VendorID             string
	PeriodStart          string
	PeriodEnd            string
	Notes                string
	ExistingEnrollmentID string
}

func normalizeAcceptance(input *RequestAcceptedInput) (normalizedAcceptance, error) {
	accepted := normalizedAcceptance{
		CourseID:             strings.TrimSpace(input.CourseID),
		EventID:              strings.TrimSpace(input.EventID),
		VendorID:             strings.TrimSpace(input.VendorID),
		PeriodStart:          strings.TrimSpace(input.PeriodStart),
		PeriodEnd:            strings.TrimSpace(input.PeriodEnd),
		Notes:                strings.TrimSpace(input.Notes),
		ExistingEnrollmentID: strings.TrimSpace(input.ExistingEnrollmentID),
	}
	if accepted.CourseID == "" {
		return accepted, validationError("course_required", "corso obbligatorio")
	}
	periodStart, err := parseOptionalDate(accepted.PeriodStart)
	if err != nil {
		return accepted, validationError("invalid_accepted_period_start", "data inizio del periodo accolto non valida")
	}
	periodEnd, err := parseOptionalDate(accepted.PeriodEnd)
	if err != nil {
		return accepted, validationError("invalid_accepted_period_end", "data fine del periodo accolto non valida")
	}
	if periodStart != nil && periodEnd != nil && periodEnd.Before(*periodStart) {
		return accepted, validationError("accepted_period_end_before_start", "la fine del periodo accolto non puo precedere l'inizio")
	}
	return accepted, nil
}

func (s *SQLStore) RecordPeopleDecision(ctx context.Context, principal Principal, id string, input RequestDecisionInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id richiesta obbligatorio")
	}
	decision := strings.TrimSpace(input.Decision)
	if decision != requestDecisionAccepted && decision != requestDecisionRejected {
		return ActionResponse{}, validationError("invalid_decision", "decisione non valida")
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return ActionResponse{}, validationError("reason_required", "motivazione obbligatoria")
	}
	if decision == requestDecisionAccepted && input.Accepted == nil {
		return ActionResponse{}, validationError("accepted_payload_required", "i dati di accoglimento sono obbligatori per accogliere la richiesta")
	}
	if decision == requestDecisionRejected && input.Accepted != nil {
		return ActionResponse{}, validationError("accepted_payload_forbidden", "i dati di accoglimento non sono ammessi quando la richiesta viene respinta")
	}
	var accepted normalizedAcceptance
	if input.Accepted != nil {
		var err error
		accepted, err = normalizeAcceptance(input.Accepted)
		if err != nil {
			return ActionResponse{}, err
		}
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		facts, err := s.lockRequestFacts(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := requestDecisionPolicy(facts, requestActionDecision); err != nil {
			return err
		}
		before, err := entitySnapshot(ctx, tx, "training_request", id)
		if err != nil {
			return err
		}
		actorID := s.actorEmployeeID(ctx, tx, principal)

		if decision == requestDecisionRejected {
			if _, err := tx.ExecContext(ctx, `
UPDATE training.training_request
SET people_decision = 'rejected',
    people_decision_by = $2::uuid,
    people_decision_at = now(),
    people_decision_reason = $3,
    outcome = 'rejected',
    closed_at = now(),
    updated_at = now()
WHERE id = $1::uuid`, id, nullableUUIDPtr(actorID), reason); err != nil {
				return fmt.Errorf("reject training request: %w", err)
			}
		} else if err := s.applyRequestAcceptance(ctx, tx, principal, facts, accepted, actorID, reason); err != nil {
			return err
		}

		after, err := entitySnapshot(ctx, tx, "training_request", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_request", id, "decision", before, after); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id, Status: decision}
		return nil
	})
	return response, err
}

// applyRequestAcceptance esegue l'accoglimento (D8) nella stessa transazione
// della decisione: risolve o crea l'evento, collega l'iscrizione esistente
// senza doppioni oppure crea l'iscrizione planned, scrive la faccia accolta e
// chiude la richiesta. I dati originali non compaiono in nessun SET.
func (s *SQLStore) applyRequestAcceptance(ctx context.Context, tx *sql.Tx, principal Principal, facts requestPolicyFacts, accepted normalizedAcceptance, actorID *string, reason string) error {
	// Il corso accettato deve gia esistere: un corso fuori catalogo viene
	// creato prima da People con l'upsert corsi, mai dentro la decisione.
	if err := s.ensureCourseExists(ctx, tx, accepted.CourseID); err != nil {
		return err
	}
	if err := s.ensureVendorExists(ctx, tx, accepted.VendorID); err != nil {
		return err
	}

	// Evento indicato: esistente, non annullato e del corso accettato. Il
	// lock precede quello dell'iscrizione (ordine evento -> iscrizione, come
	// nel resto del pacchetto).
	acceptedEventID := accepted.EventID
	if acceptedEventID != "" {
		var (
			eventCourseID  string
			eventCancelled bool
		)
		err := tx.QueryRowContext(ctx, `
SELECT course_id::text, cancelled_at IS NOT NULL
FROM training.training_event
WHERE id = $1::uuid
FOR UPDATE`, acceptedEventID).Scan(&eventCourseID, &eventCancelled)
		if errors.Is(err, sql.ErrNoRows) {
			return validationError("event_not_found", "evento non trovato")
		}
		if err != nil {
			return fmt.Errorf("load training event for request acceptance: %w", err)
		}
		if eventCancelled {
			return conflictError("event_cancelled", "l'evento scelto e annullato")
		}
		if eventCourseID != accepted.CourseID {
			return validationError("event_course_mismatch", "l'evento scelto non appartiene al corso accettato")
		}
	}

	var resultingEnrollmentID string
	if accepted.ExistingEnrollmentID != "" {
		// Persona gia coperta: si collega l'iscrizione esistente senza
		// doppioni. Deve appartenere alla stessa persona e a un evento del
		// corso accettato; nessuna nuova iscrizione.
		var (
			enrollmentEmployeeID string
			enrollmentEventID    string
			enrollmentStatus     string
			enrollmentCourseID   string
		)
		err := tx.QueryRowContext(ctx, `
SELECT en.employee_id::text, en.event_id::text, en.delivery_status, ev.course_id::text
FROM training.enrollment en
JOIN training.training_event ev ON ev.id = en.event_id
WHERE en.id = $1::uuid
FOR UPDATE OF en`, accepted.ExistingEnrollmentID).Scan(
			&enrollmentEmployeeID,
			&enrollmentEventID,
			&enrollmentStatus,
			&enrollmentCourseID,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return validationError("enrollment_not_found", "iscrizione non trovata")
		}
		if err != nil {
			return fmt.Errorf("load training enrollment for request acceptance: %w", err)
		}
		if enrollmentEmployeeID != facts.EmployeeID {
			return validationError("enrollment_person_mismatch", "l'iscrizione collegata appartiene a un'altra persona")
		}
		if enrollmentCourseID != accepted.CourseID {
			return validationError("enrollment_course_mismatch", "l'iscrizione collegata non riguarda il corso accettato")
		}
		if enrollmentStatus == deliveryCancelled {
			return conflictError("enrollment_cancelled", "l'iscrizione collegata e annullata")
		}
		resultingEnrollmentID = accepted.ExistingEnrollmentID
		if acceptedEventID == "" {
			// La sede della formazione e l'evento dell'iscrizione collegata:
			// niente evento nuovo da organizzare.
			acceptedEventID = enrollmentEventID
		}
	} else {
		if err := s.ensureEmployeeActive(ctx, tx, facts.EmployeeID); err != nil {
			return err
		}
		if acceptedEventID == "" {
			// Nessun evento adatto esistente: viene creato contestualmente.
			if err := tx.QueryRowContext(ctx, `
INSERT INTO training.training_event (course_id, title, origin, source_request_id)
VALUES (
  $1::uuid,
  (SELECT c.title FROM training.course c WHERE c.id = $1::uuid),
  'request', $2::uuid)
RETURNING id::text`, accepted.CourseID, facts.ID).Scan(&acceptedEventID); err != nil {
				return fmt.Errorf("create training event from request: %w", err)
			}
			if err := copyCourseTrainers(ctx, tx, acceptedEventID, accepted.CourseID); err != nil {
				return err
			}
			afterEvent, err := entitySnapshot(ctx, tx, "training_event", acceptedEventID)
			if err != nil {
				return err
			}
			if err := s.audit(ctx, tx, principal, "training_event", acceptedEventID, "create", nil, afterEvent); err != nil {
				return err
			}
		}
		if err := tx.QueryRowContext(ctx, `
INSERT INTO training.enrollment (employee_id, event_id, delivery_status, origin, source_request_id)
VALUES ($1::uuid, $2::uuid, 'planned', 'request', $3::uuid)
RETURNING id::text`, facts.EmployeeID, acceptedEventID, facts.ID).Scan(&resultingEnrollmentID); err != nil {
			if isUniqueViolation(err, "") {
				return conflictError("already_enrolled", "la persona e gia iscritta all'evento")
			}
			return fmt.Errorf("create training enrollment from request: %w", err)
		}
		afterEnrollment, err := entitySnapshot(ctx, tx, "enrollment", resultingEnrollmentID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "enrollment", resultingEnrollmentID, "create", nil, afterEnrollment); err != nil {
			return err
		}
	}

	// Faccia accolta + decisione + chiusura: i dati originali della richiesta
	// non compaiono nel SET.
	if _, err := tx.ExecContext(ctx, `
UPDATE training.training_request
SET people_decision = 'accepted',
    people_decision_by = $2::uuid,
    people_decision_at = now(),
    people_decision_reason = $3,
    outcome = 'accepted',
    closed_at = now(),
    accepted_course_id = $4::uuid,
    accepted_event_id = $5::uuid,
    accepted_vendor_id = $6::uuid,
    accepted_period_start = NULLIF($7, '')::date,
    accepted_period_end = NULLIF($8, '')::date,
    accepted_notes = NULLIF($9, ''),
    resulting_enrollment_id = $10::uuid,
    updated_at = now()
WHERE id = $1::uuid`,
		facts.ID,
		nullableUUIDPtr(actorID),
		reason,
		accepted.CourseID,
		acceptedEventID,
		nullableUUID(accepted.VendorID),
		accepted.PeriodStart,
		accepted.PeriodEnd,
		accepted.Notes,
		resultingEnrollmentID,
	); err != nil {
		return fmt.Errorf("accept training request: %w", err)
	}
	return nil
}

func (s *SQLStore) WithdrawRequest(ctx context.Context, principal Principal, id string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id richiesta obbligatorio")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		facts, err := s.lockRequestFacts(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := requestDecisionPolicy(facts, requestActionWithdraw); err != nil {
			return err
		}
		before, err := entitySnapshot(ctx, tx, "training_request", id)
		if err != nil {
			return err
		}
		// Il ritiro non tocca i campi del parere e della decisione.
		if _, err := tx.ExecContext(ctx, `
UPDATE training.training_request
SET outcome = 'withdrawn',
    closed_at = now(),
    updated_at = now()
WHERE id = $1::uuid`, id); err != nil {
			return fmt.Errorf("withdraw training request: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "training_request", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_request", id, "withdraw", before, after); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id, Status: requestOutcomeWithdrawn}
		return nil
	})
	return response, err
}
