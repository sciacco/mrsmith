package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/pkg/factorial"
)

// Ramo outbound del reconciler Factorial (#141, slice 5/8 = #147):
// search-before-create sui correlatori, placeholder tecnici alla create,
// update che preservano i campi remoti non mappati, membership/access con
// rilettura attendance, propagazione 3-way. Chiamate Factorial fuori dalle
// transazioni DB; ogni create/adozione persiste l'id subito in una tx breve dedicata.

// actionFactorialExport e' l'action di audit per le scritture di questo ramo (simmetrica a actionFactorialImport, inbound).
const actionFactorialExport = "factorial_export"

// Esiti di matchChild (Code per i Training, token [MS:<uuid>] per classi/sessioni).
const (
	matchCreate   = "create"
	matchAdopt    = "adopt"
	matchConflict = "conflict"
)

// technicalEmployee e' l'identita' Factorial dell'employee tecnico, letta
// una volta per run: AccessID per author_id, CompanyID per company_id/guard mismatch.
type technicalEmployee struct {
	AccessID  string
	CompanyID string
}

// outboundIssue e' una voce di warning/conflitto/piano (dry-run). Err
// propaga l'errore Factorial originale (mai perso dietro la classificazione); nil altrimenti.
type outboundIssue struct {
	Kind  string
	Ref   string
	Err   error `json:"-"`
	Count int
}

// outboundSyncResult: conflitti/warning accumulati; Planned solo in dry-run, al posto di ogni scrittura.
type outboundSyncResult struct {
	Conflicts []outboundIssue
	Warnings  []outboundIssue
	Planned   []outboundIssue
}

// dateEnvelope e' l'intervallo (YYYY-MM-DD) da classDateEnvelope; OK=false se nessuna sessione contribuisce.
type dateEnvelope struct {
	Start, End string
	OK         bool
}

// correlatorChild: candidato per la ricerca correlator-based (Code per i Training, Name per classi/sessioni).
type correlatorChild struct {
	ID   string
	Name string
}

// graphIndex organizza il grafo fetchato (spazio di ricerca del search-before-create, #145) per lookup per id e per figli.
type graphIndex struct {
	trainingByID      map[string]factorial.TrainingsTraining
	classByID         map[string]factorial.TrainingsTrainingClass
	sessionByID       map[string]factorial.TrainingsSession
	attendanceByID    map[string]factorial.TrainingsSessionAttendance
	allTrainings      []correlatorChild
	classesByTraining map[string][]correlatorChild
	sessionsByClass   map[string][]correlatorChild
}

func buildGraphIndex(graph factorialTrainingGraph) graphIndex {
	idx := graphIndex{
		trainingByID:      map[string]factorial.TrainingsTraining{},
		classByID:         map[string]factorial.TrainingsTrainingClass{},
		sessionByID:       map[string]factorial.TrainingsSession{},
		attendanceByID:    map[string]factorial.TrainingsSessionAttendance{},
		classesByTraining: map[string][]correlatorChild{},
		sessionsByClass:   map[string][]correlatorChild{},
	}
	for _, t := range graph.Trainings {
		if t.ID == nil {
			continue
		}
		idx.trainingByID[*t.ID] = t
		idx.allTrainings = append(idx.allTrainings, correlatorChild{ID: *t.ID, Name: deref(t.Code)})
	}
	for _, c := range graph.TrainingClasses {
		if c.ID == nil || c.TrainingID == nil {
			continue
		}
		idx.classByID[*c.ID] = c
		idx.classesByTraining[*c.TrainingID] = append(idx.classesByTraining[*c.TrainingID], correlatorChild{ID: *c.ID, Name: deref(c.Name)})
	}
	for _, sess := range graph.Sessions {
		if sess.ID == nil {
			continue
		}
		idx.sessionByID[*sess.ID] = sess
		if sess.TrainingClassID != nil {
			idx.sessionsByClass[*sess.TrainingClassID] = append(idx.sessionsByClass[*sess.TrainingClassID], correlatorChild{ID: *sess.ID, Name: deref(sess.Name)})
		}
	}
	for _, a := range graph.SessionAttendances {
		if a.ID != nil {
			idx.attendanceByID[*a.ID] = a
		}
	}
	return idx
}

// sessionExportReason: "" se esportabile; tipo NULL attende People; scheduled multigiorno urta il vincolo same-day dell'API.
func sessionExportReason(s sessionSyncState) string {
	switch s.ScheduleType {
	case "":
		return "session_type_missing"
	case "scheduled":
		if s.StartsAt != "" && s.EndsAt != "" && s.StartsAt[:10] != s.EndsAt[:10] {
			return "session_multiday_scheduled"
		}
	}
	return ""
}

// anySessionExportable dice se almeno una sessione e' individualmente
// esportabile: gate per la create della classe, separato dall'envelope (che
// include anche date di sessioni non esportabili, vedi classDateEnvelope).
func anySessionExportable(sessions []sessionSyncState) bool {
	for _, s := range sessions {
		if sessionExportReason(s) == "" {
			return true
		}
	}
	return false
}

// classDateEnvelope calcola min/max (YYYY-MM-DD) tra le sessioni con una
// data utile: scheduled -> StartsAt/EndsAt, self-paced -> sola DueDate come
// entrambi gli estremi (incluso un tipo locale NULL con date residue: qui
// non si giudica la rappresentabilita'). Contribuiscono ANCHE le sessioni
// non individualmente esportabili (es. multigiorno): la classe esiste
// fisicamente su quei giorni. Il gate "serve almeno una sessione esportabile
// per creare la classe" e' del chiamante (resolveClass). OK=false se nessuna sessione contribuisce.
func classDateEnvelope(sessions []sessionSyncState) dateEnvelope {
	var env dateEnvelope
	for _, s := range sessions {
		var lo, hi string
		switch {
		case s.StartsAt != "":
			lo = s.StartsAt[:10]
			if s.EndsAt != "" {
				hi = s.EndsAt[:10]
			} else {
				hi = lo
			}
		case s.DueDate != "":
			lo, hi = s.DueDate, s.DueDate
		default:
			continue
		}
		if !env.OK || lo < env.Start {
			env.Start = lo
		}
		if !env.OK || hi > env.End {
			env.End = hi
		}
		env.OK = true
	}
	return env
}

// firstExportableEnvelope sceglie l'envelope con inizio piu' antico tra gli
// eventi di un corso: la "prima erogazione esportabile" che autorizza la create del Training (#141). ok=false se nessuno e' valido.
func firstExportableEnvelope(envelopes []dateEnvelope) (dateEnvelope, bool) {
	var best dateEnvelope
	found := false
	for _, e := range envelopes {
		if !e.OK {
			continue
		}
		if !found || e.Start < best.Start {
			best = e
			found = true
		}
	}
	return best, found
}

// sessionStates proietta una lista di sessioni locali grezze nella
// proiezione canonica (riusa newSessionSyncStateFromLocal, slice 2).
func sessionStates(rows []outboundSessionRow) []sessionSyncState {
	states := make([]sessionSyncState, len(rows))
	for i, r := range rows {
		states[i] = newSessionSyncStateFromLocal(r.ScheduleType, r.StartsAt, r.EndsAt, r.DueAt)
	}
	return states
}

// factorialDateFromState/factorialTimeFromState riportano la proiezione
// canonica (stringhe UTC ratificate) in *factorial.Date/Time senza
// ulteriori conversioni di fuso: la proiezione e' la fonte, mai il time.Time grezzo.
func factorialDateFromState(s string) (*factorial.Date, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("parse session sync state date %q: %w", s, err)
	}
	return &factorial.Date{Time: t}, nil
}

func factorialTimeFromState(s string) (*factorial.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, fmt.Errorf("parse session sync state instant %q: %w", s, err)
	}
	return &factorial.Time{Time: t}, nil
}

// remoteScheduleFromLocal e' l'inverso di normalizeRemoteSchedule: locale
// "self_paced" -> remoto "selfpaced". "" (NULL) non arriva mai qui (i chiamanti filtrano prima con sessionExportReason).
func remoteScheduleFromLocal(local string) string {
	if local == "self_paced" {
		return "selfpaced"
	}
	return local
}

// withFallback: value, o fallback se vuoto/spazi.
func withFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// matchChild applica matchCorrelator ai figli di un parent (o alla lista
// piatta dei Training per Code): 0=create, 1=adopt (con l'id), >1=conflict (#141, slice 2).
func matchChild(children []correlatorChild, token string) (outcome, adoptedID string) {
	names := make([]string, len(children))
	for i, c := range children {
		names[i] = c.Name
	}
	switch matches := matchCorrelator(names, token); len(matches) {
	case 0:
		return matchCreate, ""
	case 1:
		return matchAdopt, children[matches[0]].ID
	default:
		return matchConflict, ""
	}
}

// trainingCreateBody costruisce il body di create di un Training nuovo
// (#141): Code = uuid del corso, Company/Author dall'employee tecnico.
func trainingCreateBody(courseID, title, description, providerKind string, year int64, tech technicalEmployee) *factorial.TrainingsTrainingsCreateBody {
	return &factorial.TrainingsTrainingsCreateBody{
		Name:        title,
		Code:        &courseID,
		Description: withFallback(description, title),
		External:    providerKind == "external",
		Year:        year,
		CompanyID:   &tech.CompanyID,
		AuthorID:    &tech.AccessID,
		Attachments: []any{},
	}
}

// trainingUpdateBody: Year e ogni altro campo remoto non mappato restano invariati (Year preservato dal chiamante, obbligatorio nel body).
func trainingUpdateBody(title, description, providerKind string, remoteYear int64) *factorial.TrainingsTrainingsUpdateBody {
	return &factorial.TrainingsTrainingsUpdateBody{
		Name:        title,
		Description: withFallback(description, title),
		External:    providerKind == "external",
		Year:        remoteYear,
	}
}

// classCreateBody: date dall'envelope, placeholder tecnici (costi "0", payment_status pending).
func classCreateBody(trainingID, name string, start, end factorial.Date, tech technicalEmployee) *factorial.TrainingsTrainingClassesCreateBody {
	return &factorial.TrainingsTrainingClassesCreateBody{
		Name:           &name,
		StartDate:      start,
		EndDate:        end,
		TrainingID:     trainingID,
		CompanyID:      tech.CompanyID,
		AuthorID:       tech.AccessID,
		Cost:           "0",
		SubsidizedCost: "0",
		IndirectCost:   "0",
		SalaryCost:     "0",
		PaymentStatus:  factorial.TrainingsTrainingClassesCreateBodyPaymentStatusPending,
	}
}

// classUpdateBody costruisce l'update di una classe: date dall'envelope,
// costi/payment_status preservati dal remoto; ok=false se un obbligatorio manca (update saltato, #141).
func classUpdateBody(remote factorial.TrainingsTrainingClass, start, end factorial.Date) (*factorial.TrainingsTrainingClassesUpdateBody, bool) {
	if remote.Cost == nil || remote.SubsidizedCost == nil || remote.IndirectCost == nil || remote.SalaryCost == nil || remote.PaymentStatus == nil {
		return nil, false
	}
	return &factorial.TrainingsTrainingClassesUpdateBody{
		StartDate:      &start,
		EndDate:        &end,
		Cost:           *remote.Cost,
		SubsidizedCost: *remote.SubsidizedCost,
		SalaryCost:     *remote.SalaryCost,
		IndirectCost:   *remote.IndirectCost,
		PaymentStatus:  factorial.TrainingsTrainingClassesUpdateBodyPaymentStatus(*remote.PaymentStatus),
	}, true
}

// sessionScheduleFields: i tre campi condivisi da create/update sessione (StartsAt/EndsAt/DueDate) dalla proiezione canonica.
func sessionScheduleFields(local sessionSyncState) (starts *factorial.Time, ends *factorial.Time, due *factorial.Date, err error) {
	if starts, err = factorialTimeFromState(local.StartsAt); err != nil {
		return nil, nil, nil, err
	}
	if ends, err = factorialTimeFromState(local.EndsAt); err != nil {
		return nil, nil, nil, err
	}
	if due, err = factorialDateFromState(local.DueDate); err != nil {
		return nil, nil, nil, err
	}
	return starts, ends, due, nil
}

// sessionCreateBody: nome = titolo corso + correlatore, schedule/date dalla proiezione locale, reminder/inviti disabilitati.
func sessionCreateBody(trainingID, classID, name string, local sessionSyncState) (*factorial.TrainingsSessionsCreateBody, error) {
	starts, ends, due, err := sessionScheduleFields(local)
	if err != nil {
		return nil, err
	}
	schedule := factorial.TrainingsSessionsCreateBodySchedule(remoteScheduleFromLocal(local.ScheduleType))
	sendInvites := false
	return &factorial.TrainingsSessionsCreateBody{
		Name: name, TrainingID: trainingID, TrainingClassID: &classID,
		StartsAt: starts, EndsAt: ends, DueDate: due, Schedule: &schedule,
		Reminders: []any{}, SendCalendarInvites: &sendInvites,
	}, nil
}

// sessionUpdateBody: schedule/date dal locale corrente; Name preserva il remoto (obbligatorio, mai rigenerato in update, #141).
func sessionUpdateBody(remoteName string, local sessionSyncState) (*factorial.TrainingsSessionsUpdateBody, error) {
	starts, ends, due, err := sessionScheduleFields(local)
	if err != nil {
		return nil, err
	}
	schedule := factorial.TrainingsSessionsUpdateBodySchedule(remoteScheduleFromLocal(local.ScheduleType))
	return &factorial.TrainingsSessionsUpdateBody{
		Name: remoteName, StartsAt: starts, EndsAt: ends, DueDate: due, Schedule: &schedule,
	}, nil
}

// Body delle azioni bulk (#141): niente flag notify sulla membership (il tipo non lo espone), disabilitato sull'access.
func membershipBulkCreateBody(trainingID string, employeeIDs []string) *factorial.TrainingsTrainingMembershipsBulkCreateBody {
	return &factorial.TrainingsTrainingMembershipsBulkCreateBody{EmployeeIDs: employeeIDs, TrainingID: trainingID}
}

func accessBulkCreateBody(sessionID string, employeeIDs []string) *factorial.TrainingsSessionAccessMembershipsBulkCreateBody {
	return &factorial.TrainingsSessionAccessMembershipsBulkCreateBody{EmployeeIDs: employeeIDs, SessionID: sessionID, Notify: false}
}

func attendanceBulkUpdateBody(attendanceID string, remoteStatus factorial.TrainingsSessionAttendanceStatus) *factorial.TrainingsSessionAttendancesBulkUpdateBody {
	status := string(remoteStatus)
	return &factorial.TrainingsSessionAttendancesBulkUpdateBody{IDs: []string{attendanceID}, Status: &status}
}

// --- Lettura locale bulk (candidati outbound, linkati e non) ---------------

type outboundCourseRow struct {
	ID, FactorialTrainingID, Title, Description, ProviderKind string
	IsActive                                                  bool
}

type outboundEventRow struct {
	ID, FactorialClassID string
}

type outboundSessionRow struct {
	ID, FactorialSessionID  string
	ScheduleType            *string
	StartsAt, EndsAt, DueAt *time.Time
	Checkpoint              *sessionSyncState
}

type outboundEnrollmentRow struct {
	ID, EventID, EmployeeExternalID, MembershipID string
	Cancelled                                     bool
}

type outboundParticipantRow struct {
	EnrollmentID, SessionID, Status, AccessID, AttendanceID string
	Checkpoint                                              *string
}

// outboundState e' lo stato locale grezzo per l'intera run, raggruppato per
// chiave del parent: sempre lookup per chiave sugli insiemi scoped al
// corso/evento/sessione corrente, mai iterazioni sulle mappe globali (#141 slice 4).
type outboundState struct {
	Courses               []outboundCourseRow
	EventsByCourse        map[string][]outboundEventRow
	SessionsByEvent       map[string][]outboundSessionRow
	EnrollmentsByEvent    map[string][]outboundEnrollmentRow
	EnrollmentByID        map[string]outboundEnrollmentRow
	ParticipantsBySession map[string][]outboundParticipantRow
}

func (s *SQLStore) loadOutboundState(ctx context.Context, q sqlRunner) (outboundState, error) {
	state := outboundState{
		EventsByCourse: map[string][]outboundEventRow{}, SessionsByEvent: map[string][]outboundSessionRow{},
		EnrollmentsByEvent: map[string][]outboundEnrollmentRow{}, EnrollmentByID: map[string]outboundEnrollmentRow{},
		ParticipantsBySession: map[string][]outboundParticipantRow{},
	}
	courseRows, err := q.QueryContext(ctx, `
SELECT id::text, COALESCE(factorial_training_id, ''), title, COALESCE(description, ''), provider_kind::text, is_active
FROM training.course`)
	if err != nil {
		return outboundState{}, fmt.Errorf("list outbound courses: %w", err)
	}
	defer courseRows.Close()
	for courseRows.Next() {
		var c outboundCourseRow
		if err := courseRows.Scan(&c.ID, &c.FactorialTrainingID, &c.Title, &c.Description, &c.ProviderKind, &c.IsActive); err != nil {
			return outboundState{}, fmt.Errorf("scan outbound course: %w", err)
		}
		state.Courses = append(state.Courses, c)
	}
	if err := courseRows.Err(); err != nil {
		return outboundState{}, err
	}

	eventRows, err := q.QueryContext(ctx, `
SELECT course_id::text, id::text, COALESCE(factorial_class_id, '')
FROM training.training_event WHERE cancelled_at IS NULL`)
	if err != nil {
		return outboundState{}, fmt.Errorf("list outbound events: %w", err)
	}
	defer eventRows.Close()
	for eventRows.Next() {
		var courseID string
		var e outboundEventRow
		if err := eventRows.Scan(&courseID, &e.ID, &e.FactorialClassID); err != nil {
			return outboundState{}, fmt.Errorf("scan outbound event: %w", err)
		}
		state.EventsByCourse[courseID] = append(state.EventsByCourse[courseID], e)
	}
	if err := eventRows.Err(); err != nil {
		return outboundState{}, err
	}

	sessionRows, err := q.QueryContext(ctx, `
SELECT s.event_id::text, s.id::text, s.schedule_type, s.starts_at, s.ends_at, s.due_at,
  COALESCE(s.factorial_session_id, ''), s.factorial_synced_state
FROM training.training_session s
JOIN training.training_event ev ON ev.id = s.event_id
WHERE ev.cancelled_at IS NULL`)
	if err != nil {
		return outboundState{}, fmt.Errorf("list outbound sessions: %w", err)
	}
	defer sessionRows.Close()
	for sessionRows.Next() {
		var (
			eventID                 string
			sess                    outboundSessionRow
			scheduleType            sql.NullString
			startsAt, endsAt, dueAt sql.NullTime
			checkpointRaw           []byte
		)
		if err := sessionRows.Scan(&eventID, &sess.ID, &scheduleType, &startsAt, &endsAt, &dueAt, &sess.FactorialSessionID, &checkpointRaw); err != nil {
			return outboundState{}, fmt.Errorf("scan outbound session: %w", err)
		}
		sess.ScheduleType = nullStringPtr(scheduleType)
		sess.StartsAt, sess.EndsAt, sess.DueAt = nullTimePtr(startsAt), nullTimePtr(endsAt), nullTimePtr(dueAt)
		if len(checkpointRaw) > 0 {
			var checkpoint sessionSyncState
			if err := json.Unmarshal(checkpointRaw, &checkpoint); err != nil {
				return outboundState{}, fmt.Errorf("decode outbound session checkpoint: %w", err)
			}
			sess.Checkpoint = &checkpoint
		}
		state.SessionsByEvent[eventID] = append(state.SessionsByEvent[eventID], sess)
	}
	if err := sessionRows.Err(); err != nil {
		return outboundState{}, err
	}

	enrollmentRows, err := q.QueryContext(ctx, `
SELECT en.event_id::text, en.id::text, emp.external_id, COALESCE(en.factorial_training_membership_id, ''), en.delivery_status = 'cancelled'
FROM training.enrollment en
JOIN training.training_event ev ON ev.id = en.event_id
JOIN training.employee emp ON emp.id = en.employee_id
WHERE ev.cancelled_at IS NULL AND emp.external_id IS NOT NULL`)
	if err != nil {
		return outboundState{}, fmt.Errorf("list outbound enrollments: %w", err)
	}
	defer enrollmentRows.Close()
	for enrollmentRows.Next() {
		var en outboundEnrollmentRow
		if err := enrollmentRows.Scan(&en.EventID, &en.ID, &en.EmployeeExternalID, &en.MembershipID, &en.Cancelled); err != nil {
			return outboundState{}, fmt.Errorf("scan outbound enrollment: %w", err)
		}
		state.EnrollmentsByEvent[en.EventID] = append(state.EnrollmentsByEvent[en.EventID], en)
		state.EnrollmentByID[en.ID] = en
	}
	if err := enrollmentRows.Err(); err != nil {
		return outboundState{}, err
	}

	participantRows, err := q.QueryContext(ctx, `
SELECT es.session_id::text, es.enrollment_id::text, es.participation_status, es.factorial_synced_status,
  COALESCE(es.factorial_access_membership_id, ''), COALESCE(es.factorial_attendance_id, '')
FROM training.enrollment_session es
JOIN training.enrollment en ON en.id = es.enrollment_id
WHERE en.delivery_status <> 'cancelled'`)
	if err != nil {
		return outboundState{}, fmt.Errorf("list outbound participants: %w", err)
	}
	defer participantRows.Close()
	for participantRows.Next() {
		var p outboundParticipantRow
		var checkpoint sql.NullString
		if err := participantRows.Scan(&p.SessionID, &p.EnrollmentID, &p.Status, &checkpoint, &p.AccessID, &p.AttendanceID); err != nil {
			return outboundState{}, fmt.Errorf("scan outbound participant: %w", err)
		}
		p.Checkpoint = nullStringPtr(checkpoint)
		state.ParticipantsBySession[p.SessionID] = append(state.ParticipantsBySession[p.SessionID], p)
	}
	return state, participantRows.Err()
}

// --- Persistenza (transazioni brevi dedicate, subito dopo ogni IO Factorial) ---

// persistFactorialLink scrive l'id Factorial su course/training_event, in una tx breve dedicata (nessun checkpoint per queste due entita').
func (s *SQLStore) persistFactorialLink(ctx context.Context, principal Principal, table, column, localID, factorialID, action string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		q := fmt.Sprintf(`UPDATE training.%s SET %s = $2, updated_at = now() WHERE id = $1::uuid`, table, column)
		if _, err := tx.ExecContext(ctx, q, localID, factorialID); err != nil {
			return fmt.Errorf("persist factorial link on %s: %w", table, err)
		}
		return s.auditFields(ctx, tx, principal, table, localID, action, []string{column})
	})
}

// persistSessionLink scrive, con un solo UPDATE in una tx breve dedicata,
// l'id Factorial e il checkpoint di una sessione (se gia' collegata,
// factorialSessionID e' il valore che aveva gia': scrittura idempotente).
func (s *SQLStore) persistSessionLink(ctx context.Context, principal Principal, sessionID, factorialSessionID string, checkpoint sessionSyncState, action string) error {
	payload, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("marshal outbound session checkpoint: %w", err)
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
UPDATE training.training_session SET factorial_session_id = $2, factorial_synced_state = $3::jsonb, updated_at = now()
WHERE id = $1::uuid`, sessionID, factorialSessionID, payload); err != nil {
			return fmt.Errorf("persist session factorial link: %w", err)
		}
		return s.auditFields(ctx, tx, principal, "training_session", sessionID, action, []string{"factorial_session_id", "factorial_synced_state"})
	})
}

// enrollmentSessionLink: riga da persistere su enrollment_session (access id, attendance id o checkpoint: stessa forma, colonna diversa).
type enrollmentSessionLink struct{ EnrollmentID, SessionID, Value string }

// persistEnrollmentSessionLinks scrive la colonna indicata su ogni riga della lista, in una tx breve dedicata.
func (s *SQLStore) persistEnrollmentSessionLinks(ctx context.Context, principal Principal, column string, links []enrollmentSessionLink) error {
	if len(links) == 0 {
		return nil
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		q := fmt.Sprintf(`UPDATE training.enrollment_session SET %s = $3, updated_at = now() WHERE enrollment_id = $1::uuid AND session_id = $2::uuid`, column)
		for _, l := range links {
			if _, err := tx.ExecContext(ctx, q, l.EnrollmentID, l.SessionID, l.Value); err != nil {
				return fmt.Errorf("persist enrollment_session %s: %w", column, err)
			}
			if err := s.auditFields(ctx, tx, principal, "enrollment_session", l.EnrollmentID, actionFactorialExport, []string{column}); err != nil {
				return err
			}
		}
		return nil
	})
}

// persistMembershipLinks scrive l'id membership su tutte le iscrizioni
// indicate, in una tx breve dedicata (id condivisibile tra piu' iscrizioni dello stesso corso, #141).
func (s *SQLStore) persistMembershipLinks(ctx context.Context, principal Principal, enrollmentIDs []string, membershipID string) error {
	if len(enrollmentIDs) == 0 {
		return nil
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, id := range enrollmentIDs {
			if _, err := tx.ExecContext(ctx, `
UPDATE training.enrollment SET factorial_training_membership_id = $2, updated_at = now() WHERE id = $1::uuid`,
				id, membershipID); err != nil {
				return fmt.Errorf("persist training membership link: %w", err)
			}
			if err := s.auditFields(ctx, tx, principal, "enrollment", id, actionFactorialExport, []string{"factorial_training_membership_id"}); err != nil {
				return err
			}
		}
		return nil
	})
}

// --- Classificazione errori (mai con errore nil; propaga l'originale) ------

// classifyOutbound valuta un errore Factorial non nullo: issue+nil isola il
// singolo oggetto (400/422, errore originale propagato nell'issue);
// issue-vuota+errore fa fallire l'intera run (401/403/429/5xx, trasporto).
func classifyOutbound(kind, ref string, err error) (outboundIssue, error) {
	if classifySyncError(err) == errIsolateBranch {
		return outboundIssue{Kind: kind, Ref: ref, Err: err}, nil
	}
	return outboundIssue{}, err
}

// --- Employee tecnico (una volta per run) -----------------------------------

func fetchTechnicalEmployee(ctx context.Context, cli *factorial.Client, employeeID string) (technicalEmployee, error) {
	emp, err := cli.Employees.Employees.Get(ctx, employeeID)
	if err != nil {
		return technicalEmployee{}, fmt.Errorf("get factorial technical employee: %w", err)
	}
	return technicalEmployee{AccessID: deref(emp.AccessID), CompanyID: deref(emp.CompanyID)}, nil
}

// --- Orchestrazione (training -> classi -> sessioni -> membership -> access) ---

// applyOutboundSync esporta un corso alla volta. graph e' il grafo fetchato
// per intero (non il perimetro attivi): spazio di ricerca del
// search-before-create (#145). activeIDs filtra solo membership/access. dryRun=true non scrive: solo piani/conteggi in Planned.
func (s *SQLStore) applyOutboundSync(ctx context.Context, cli *factorial.Client, technicalEmployeeID string, graph factorialTrainingGraph, activeIDs map[string]struct{}, dryRun bool) (outboundSyncResult, error) {
	if s == nil || s.db == nil {
		return outboundSyncResult{}, errors.New("training database not configured")
	}
	if cli == nil {
		return outboundSyncResult{}, errors.New("factorial client not configured")
	}
	if strings.TrimSpace(technicalEmployeeID) == "" {
		return outboundSyncResult{}, errors.New("factorial technical employee not configured")
	}
	tech, err := fetchTechnicalEmployee(ctx, cli, technicalEmployeeID)
	if err != nil {
		return outboundSyncResult{}, err
	}
	state, err := s.loadOutboundState(ctx, s.db)
	if err != nil {
		return outboundSyncResult{}, err
	}
	idx := buildGraphIndex(graph)
	principal := Principal{IsPeopleAdmin: true}
	var result outboundSyncResult
	for _, course := range state.Courses {
		if err := s.exportCourse(ctx, cli, idx, tech, course, state, activeIDs, dryRun, principal, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

// exportCourse: ordine cablato training -> classi -> sessioni -> membership
// -> access, poi propaga le presenze 3-way. Errore DB abortisce la run; 400/422 Factorial isolano solo l'oggetto.
func (s *SQLStore) exportCourse(ctx context.Context, cli *factorial.Client, idx graphIndex, tech technicalEmployee, course outboundCourseRow, state outboundState, activeIDs map[string]struct{}, dryRun bool, principal Principal, result *outboundSyncResult) error {
	trainingID := course.FactorialTrainingID
	if trainingID == "" {
		var envelopes []dateEnvelope
		for _, ev := range state.EventsByCourse[course.ID] {
			envelopes = append(envelopes, classDateEnvelope(sessionStates(state.SessionsByEvent[ev.ID])))
		}
		best, ok := firstExportableEnvelope(envelopes)
		if !ok {
			return nil // corso senza erogazione esportabile: nessuna create (#141)
		}
		startDate, err := time.Parse("2006-01-02", best.Start)
		if err != nil {
			return fmt.Errorf("export course %s: parse envelope year %q: %w", course.ID, best.Start, err)
		}
		id, err := s.createOrAdoptTraining(ctx, cli, idx, tech, course, int64(startDate.Year()), dryRun, principal, result)
		if err != nil {
			return err
		}
		if id == "" {
			return nil // conflitto sul correlatore, gia' segnalato
		}
		trainingID = id
	} else if err := s.maybeUpdateTraining(ctx, cli, idx, course, trainingID, dryRun, principal, result); err != nil {
		return err
	}

	companyID := tech.CompanyID
	if remote, ok := idx.trainingByID[trainingID]; ok && remote.CompanyID != nil {
		companyID = *remote.CompanyID
	}
	// Grana intero-corso, non "quel singolo oggetto" (D4, #141): scelta conservativa deliberata, piu' restrittiva della lettera.
	if companyID != tech.CompanyID {
		result.Warnings = append(result.Warnings, outboundIssue{Kind: "training_company_mismatch", Ref: trainingID})
		return nil // isola il corso: nessuna configurazione multi-company (#141)
	}

	resolvedSessions := map[string]string{}
	for _, ev := range state.EventsByCourse[course.ID] {
		classID, err := s.resolveClass(ctx, cli, idx, tech, trainingID, ev, course.Title, state, dryRun, principal, result)
		if err != nil {
			return err
		}
		if classID == "" {
			continue
		}
		for _, sess := range state.SessionsByEvent[ev.ID] {
			sessionID, err := s.resolveSessionOutbound(ctx, cli, idx, trainingID, classID, course.Title, sess, dryRun, principal, result)
			if err != nil {
				return err
			}
			if sessionID != "" {
				resolvedSessions[sess.ID] = sessionID
			}
		}
	}

	if err := s.exportMembershipAndAccess(ctx, cli, trainingID, course, state, resolvedSessions, activeIDs, dryRun, principal, result); err != nil {
		return err
	}
	return s.exportAttendancePropagation(ctx, cli, idx, course, state, dryRun, principal, result)
}

// createOrAdoptTraining: search-before-create per Code (uuid del corso) su tutti i Training del grafo fetchato.
func (s *SQLStore) createOrAdoptTraining(ctx context.Context, cli *factorial.Client, idx graphIndex, tech technicalEmployee, course outboundCourseRow, year int64, dryRun bool, principal Principal, result *outboundSyncResult) (string, error) {
	outcome, adoptedID := matchChild(idx.allTrainings, course.ID)
	switch outcome {
	case matchConflict:
		result.Conflicts = append(result.Conflicts, outboundIssue{Kind: "training_correlator_conflict", Ref: course.ID})
		return "", nil
	case matchAdopt:
		if dryRun {
			result.Planned = append(result.Planned, outboundIssue{Kind: "training_adopt", Ref: course.ID})
			return adoptedID, nil
		}
		if err := s.persistFactorialLink(ctx, principal, "course", "factorial_training_id", course.ID, adoptedID, actionFactorialExport); err != nil {
			return "", err
		}
		return adoptedID, nil
	default:
		if dryRun {
			result.Planned = append(result.Planned, outboundIssue{Kind: "training_create", Ref: course.ID})
			return "planned:" + course.ID, nil
		}
		created, err := cli.Trainings.Trainings.Create(ctx, trainingCreateBody(course.ID, course.Title, course.Description, course.ProviderKind, year, tech))
		if err != nil {
			issue, failErr := classifyOutbound("training_create_failed", course.ID, err)
			if failErr != nil {
				return "", failErr
			}
			result.Warnings = append(result.Warnings, issue)
			return "", nil
		}
		if created.ID == nil {
			return "", fmt.Errorf("factorial training create: missing id for course %s", course.ID)
		}
		if err := s.persistFactorialLink(ctx, principal, "course", "factorial_training_id", course.ID, *created.ID, actionFactorialExport); err != nil {
			return "", err
		}
		return *created.ID, nil
	}
}

// maybeUpdateTraining aggiorna un Training gia' collegato se nome/descrizione/external locali divergono dal remoto.
func (s *SQLStore) maybeUpdateTraining(ctx context.Context, cli *factorial.Client, idx graphIndex, course outboundCourseRow, trainingID string, dryRun bool, principal Principal, result *outboundSyncResult) error {
	if !course.IsActive {
		return nil // seed non ancora curato: il locale prevale solo dopo l'attivazione (#141)
	}
	remote, ok := idx.trainingByID[trainingID]
	if !ok {
		return nil // fuori dal grafo fetchato in questo run: nessuna azione
	}
	external := course.ProviderKind == "external"
	description := withFallback(course.Description, course.Title)
	remoteExternal := remote.External != nil && *remote.External
	if deref(remote.Name) == course.Title && deref(remote.Description) == description && remoteExternal == external {
		return nil // gia' allineato
	}
	if remote.Year == nil {
		result.Warnings = append(result.Warnings, outboundIssue{Kind: "training_update_missing_year", Ref: course.ID})
		return nil // Year e' obbligatorio nel body: mai inviare 0, si salta e si segnala
	}
	if dryRun {
		result.Planned = append(result.Planned, outboundIssue{Kind: "training_propagate", Ref: course.ID})
		return nil
	}
	if _, err := cli.Trainings.Trainings.Update(ctx, trainingID, trainingUpdateBody(course.Title, course.Description, course.ProviderKind, *remote.Year)); err != nil {
		issue, failErr := classifyOutbound("training_update_failed", course.ID, err)
		if failErr != nil {
			return failErr
		}
		result.Warnings = append(result.Warnings, issue)
	}
	return nil
}

// resolveClass: evento non collegato -> search-before-create sotto il
// training; gia' collegato -> aggiorna date/costi se l'envelope diverge dal
// remoto (nessun checkpoint per le classi: confronto diretto). Ritorna
// l'id risolto ("" se non esportabile o in conflitto, gia' segnalato).
func (s *SQLStore) resolveClass(ctx context.Context, cli *factorial.Client, idx graphIndex, tech technicalEmployee, trainingID string, ev outboundEventRow, courseTitle string, state outboundState, dryRun bool, principal Principal, result *outboundSyncResult) (string, error) {
	states := sessionStates(state.SessionsByEvent[ev.ID])
	env := classDateEnvelope(states)
	if ev.FactorialClassID == "" {
		if !env.OK {
			result.Warnings = append(result.Warnings, outboundIssue{Kind: "class_no_dates", Ref: ev.ID})
			return "", nil
		}
		if !anySessionExportable(states) {
			result.Warnings = append(result.Warnings, outboundIssue{Kind: "class_no_exportable_sessions", Ref: ev.ID})
			return "", nil // date reali ma nessuna sessione esportabile: niente classe orfana
		}
		name := factorialClassName(courseTitle, ev.ID)
		outcome, adoptedID := matchChild(idx.classesByTraining[trainingID], msToken(ev.ID))
		switch outcome {
		case matchConflict:
			result.Conflicts = append(result.Conflicts, outboundIssue{Kind: "class_correlator_conflict", Ref: ev.ID})
			return "", nil
		case matchAdopt:
			if dryRun {
				result.Planned = append(result.Planned, outboundIssue{Kind: "class_adopt", Ref: ev.ID})
				return adoptedID, nil
			}
			if err := s.persistFactorialLink(ctx, principal, "training_event", "factorial_class_id", ev.ID, adoptedID, actionFactorialExport); err != nil {
				return "", err
			}
			return adoptedID, nil
		default:
			if dryRun {
				result.Planned = append(result.Planned, outboundIssue{Kind: "class_create", Ref: ev.ID})
				return "planned:" + ev.ID, nil
			}
			start, err := factorialDateFromState(env.Start)
			if err != nil {
				return "", fmt.Errorf("build class create body for event %s: %w", ev.ID, err)
			}
			end, err := factorialDateFromState(env.End)
			if err != nil {
				return "", fmt.Errorf("build class create body for event %s: %w", ev.ID, err)
			}
			created, err := cli.Trainings.TrainingClasses.Create(ctx, classCreateBody(trainingID, name, *start, *end, tech))
			if err != nil {
				issue, failErr := classifyOutbound("class_create_failed", ev.ID, err)
				if failErr != nil {
					return "", failErr
				}
				result.Warnings = append(result.Warnings, issue)
				return "", nil
			}
			if created.ID == nil {
				return "", fmt.Errorf("factorial class create: missing id for event %s", ev.ID)
			}
			if err := s.persistFactorialLink(ctx, principal, "training_event", "factorial_class_id", ev.ID, *created.ID, actionFactorialExport); err != nil {
				return "", err
			}
			return *created.ID, nil
		}
	}

	if !env.OK {
		return ev.FactorialClassID, nil // nessuna data locale utile: non si tocca il remoto
	}
	remoteClass, ok := idx.classByID[ev.FactorialClassID]
	if !ok {
		return ev.FactorialClassID, nil // fuori dal grafo fetchato in questo run
	}
	if remoteClass.StartDate != nil && remoteClass.EndDate != nil && remoteClass.StartDate.String() == env.Start && remoteClass.EndDate.String() == env.End {
		return ev.FactorialClassID, nil // gia' allineata
	}
	start, err := factorialDateFromState(env.Start)
	if err != nil {
		return "", fmt.Errorf("build class update body for event %s: %w", ev.ID, err)
	}
	end, err := factorialDateFromState(env.End)
	if err != nil {
		return "", fmt.Errorf("build class update body for event %s: %w", ev.ID, err)
	}
	body, ok := classUpdateBody(remoteClass, *start, *end)
	if !ok {
		result.Warnings = append(result.Warnings, outboundIssue{Kind: "class_update_missing_costs", Ref: ev.ID})
		return ev.FactorialClassID, nil
	}
	if dryRun {
		result.Planned = append(result.Planned, outboundIssue{Kind: "class_propagate", Ref: ev.ID})
		return ev.FactorialClassID, nil
	}
	if _, err := cli.Trainings.TrainingClasses.Update(ctx, ev.FactorialClassID, body); err != nil {
		issue, failErr := classifyOutbound("class_update_failed", ev.ID, err)
		if failErr != nil {
			return "", failErr
		}
		result.Warnings = append(result.Warnings, issue)
	}
	return ev.FactorialClassID, nil
}

// resolveSessionOutbound: la rappresentabilita' si valuta PRIMA di sapere se
// la sessione e' nuova o gia' collegata (V5, evita una Update con body
// invalido su una sessione diventata multigiorno/tipo-NULL). Poi: non
// collegata -> search-before-create sotto la classe; collegata -> propaga
// il locale al remoto se la decisione 3-way e' syncPropagateLocal.
func (s *SQLStore) resolveSessionOutbound(ctx context.Context, cli *factorial.Client, idx graphIndex, trainingID, classID, courseTitle string, sess outboundSessionRow, dryRun bool, principal Principal, result *outboundSyncResult) (string, error) {
	local := newSessionSyncStateFromLocal(sess.ScheduleType, sess.StartsAt, sess.EndsAt, sess.DueAt)
	if reason := sessionExportReason(local); reason != "" {
		result.Warnings = append(result.Warnings, outboundIssue{Kind: reason, Ref: sess.ID})
		return sess.FactorialSessionID, nil // "" se non ancora collegata, altrimenti resta collegata cosi' com'e'
	}
	if sess.FactorialSessionID == "" {
		name := factorialSessionName(courseTitle, sess.ID)
		outcome, adoptedID := matchChild(idx.sessionsByClass[classID], msToken(sess.ID))
		switch outcome {
		case matchConflict:
			result.Conflicts = append(result.Conflicts, outboundIssue{Kind: "session_correlator_conflict", Ref: sess.ID})
			return "", nil
		case matchAdopt:
			if dryRun {
				result.Planned = append(result.Planned, outboundIssue{Kind: "session_adopt", Ref: sess.ID})
				return adoptedID, nil
			}
			remote := newSessionSyncStateFromRemote(idx.sessionByID[adoptedID])
			if err := s.persistSessionLink(ctx, principal, sess.ID, adoptedID, remote, actionFactorialExport); err != nil {
				return "", err
			}
			return adoptedID, nil
		default:
			if dryRun {
				result.Planned = append(result.Planned, outboundIssue{Kind: "session_create", Ref: sess.ID})
				return "planned:" + sess.ID, nil
			}
			body, err := sessionCreateBody(trainingID, classID, name, local)
			if err != nil {
				return "", fmt.Errorf("build session create body for session %s: %w", sess.ID, err)
			}
			created, err := cli.Trainings.Sessions.Create(ctx, body)
			if err != nil {
				issue, failErr := classifyOutbound("session_create_failed", sess.ID, err)
				if failErr != nil {
					return "", failErr
				}
				result.Warnings = append(result.Warnings, issue)
				return "", nil
			}
			if created.ID == nil {
				return "", fmt.Errorf("factorial session create: missing id for session %s", sess.ID)
			}
			if err := s.persistSessionLink(ctx, principal, sess.ID, *created.ID, local, actionFactorialExport); err != nil {
				return "", err
			}
			return *created.ID, nil
		}
	}

	remoteSession, ok := idx.sessionByID[sess.FactorialSessionID]
	if !ok {
		return sess.FactorialSessionID, nil // fuori dal grafo fetchato in questo run
	}
	remote := newSessionSyncStateFromRemote(remoteSession)
	if threeWayDecision(local, remote, sess.Checkpoint) != syncPropagateLocal {
		return sess.FactorialSessionID, nil
	}
	if dryRun {
		result.Planned = append(result.Planned, outboundIssue{Kind: "session_propagate", Ref: sess.ID})
		return sess.FactorialSessionID, nil
	}
	body, err := sessionUpdateBody(deref(remoteSession.Name), local)
	if err != nil {
		return "", fmt.Errorf("build session update body for session %s: %w", sess.ID, err)
	}
	if _, err := cli.Trainings.Sessions.Update(ctx, sess.FactorialSessionID, body); err != nil {
		issue, failErr := classifyOutbound("session_update_failed", sess.ID, err)
		if failErr != nil {
			return "", failErr
		}
		result.Warnings = append(result.Warnings, issue)
		return sess.FactorialSessionID, nil
	}
	if err := s.persistSessionLink(ctx, principal, sess.ID, sess.FactorialSessionID, local, actionFactorialExport); err != nil {
		return "", err
	}
	return sess.FactorialSessionID, nil
}

// exportMembershipAndAccess crea prima le training membership mancanti
// (training/employee), poi gli access di sessione mancanti (sessione/employee,
// Notify=false), rileggendo l'attendance generata. Mai distrugge una
// membership esistente (#141). activeIDs filtra i partecipanti da
// esportare; i non attivi sono saltati e contati in un warning aggregato "employee_not_active" (V1).
func (s *SQLStore) exportMembershipAndAccess(ctx context.Context, cli *factorial.Client, trainingID string, course outboundCourseRow, state outboundState, resolvedSessions map[string]string, activeIDs map[string]struct{}, dryRun bool, principal Principal, result *outboundSyncResult) error {
	type membershipNeed struct {
		MembershipID string
		Missing      []string
	}
	needMembership := map[string]*membershipNeed{}
	type accessNeed struct{ EnrollmentID, EmployeeExternalID, SessionID string }
	needAccessBySession := map[string][]accessNeed{}
	inactiveSkips := 0

	for _, ev := range state.EventsByCourse[course.ID] {
		for _, sess := range state.SessionsByEvent[ev.ID] {
			factorialSessionID, resolved := resolvedSessions[sess.ID]
			if !resolved {
				continue
			}
			for _, p := range state.ParticipantsBySession[sess.ID] {
				en, ok := state.EnrollmentByID[p.EnrollmentID]
				if !ok || en.Cancelled {
					continue
				}
				if _, active := activeIDs[en.EmployeeExternalID]; !active {
					inactiveSkips++
					continue
				}
				need := needMembership[en.EmployeeExternalID]
				if need == nil {
					need = &membershipNeed{}
					needMembership[en.EmployeeExternalID] = need
				}
				if en.MembershipID != "" {
					need.MembershipID = en.MembershipID
				} else {
					need.Missing = append(need.Missing, en.ID)
				}
				if p.AccessID == "" {
					needAccessBySession[factorialSessionID] = append(needAccessBySession[factorialSessionID], accessNeed{EnrollmentID: en.ID, EmployeeExternalID: en.EmployeeExternalID, SessionID: sess.ID})
				}
			}
		}
	}
	if inactiveSkips > 0 {
		result.Warnings = append(result.Warnings, outboundIssue{Kind: "employee_not_active", Ref: course.ID, Count: inactiveSkips})
	}

	var missingEmployeeIDs []string
	for employeeID, need := range needMembership {
		if len(need.Missing) == 0 {
			continue
		}
		if need.MembershipID != "" {
			if dryRun {
				result.Planned = append(result.Planned, outboundIssue{Kind: "membership_backfill", Ref: employeeID})
			} else if err := s.persistMembershipLinks(ctx, principal, need.Missing, need.MembershipID); err != nil {
				return err
			}
			continue
		}
		missingEmployeeIDs = append(missingEmployeeIDs, employeeID)
	}
	if len(missingEmployeeIDs) > 0 {
		if dryRun {
			result.Planned = append(result.Planned, outboundIssue{Kind: "membership_bulk_create", Ref: trainingID})
		} else if created, err := cli.Trainings.TrainingMemberships.BulkCreate(ctx, membershipBulkCreateBody(trainingID, missingEmployeeIDs)); err != nil {
			issue, failErr := classifyOutbound("membership_bulk_create_failed", trainingID, err)
			if failErr != nil {
				return failErr
			}
			result.Warnings = append(result.Warnings, issue)
		} else {
			for _, m := range created {
				if m.ID == nil || m.EmployeeID == nil {
					continue
				}
				if need, ok := needMembership[*m.EmployeeID]; ok && len(need.Missing) > 0 {
					if err := s.persistMembershipLinks(ctx, principal, need.Missing, *m.ID); err != nil {
						return err
					}
				}
			}
		}
	}

	for factorialSessionID, needs := range needAccessBySession {
		employeeIDs := make([]string, len(needs))
		for i, n := range needs {
			employeeIDs[i] = n.EmployeeExternalID
		}
		if dryRun {
			result.Planned = append(result.Planned, outboundIssue{Kind: "access_bulk_create", Ref: factorialSessionID})
			continue
		}
		createdAccess, err := cli.Trainings.SessionAccessMemberships.BulkCreate(ctx, accessBulkCreateBody(factorialSessionID, employeeIDs))
		if err != nil {
			issue, failErr := classifyOutbound("access_bulk_create_failed", factorialSessionID, err)
			if failErr != nil {
				return failErr
			}
			result.Warnings = append(result.Warnings, issue)
			continue
		}
		accessIDByEmployee := map[string]string{}
		var newAccessIDs []string
		for _, a := range createdAccess {
			if a.ID == nil || a.EmployeeID == nil {
				continue
			}
			accessIDByEmployee[*a.EmployeeID] = *a.ID
			newAccessIDs = append(newAccessIDs, *a.ID)
		}
		var accessLinks []enrollmentSessionLink
		for _, n := range needs {
			accessID, ok := accessIDByEmployee[n.EmployeeExternalID]
			if !ok {
				continue
			}
			accessLinks = append(accessLinks, enrollmentSessionLink{EnrollmentID: n.EnrollmentID, SessionID: n.SessionID, Value: accessID})
		}
		if err := s.persistEnrollmentSessionLinks(ctx, principal, "factorial_access_membership_id", accessLinks); err != nil {
			return err
		}
		if len(newAccessIDs) == 0 {
			continue
		}
		if err := s.rereadAndPersistAttendance(ctx, cli, principal, newAccessIDs, accessLinks, result); err != nil {
			return err
		}
	}
	return nil
}

// rereadAndPersistAttendance rilegge le presenze generate da un
// SessionAccessMemberships.BulkCreate riuscito e ne salva l'id insieme al
// checkpoint nello stesso UPDATE: senza, il primo giro inbound leggerebbe un
// default "pending" senza checkpoint e distruggerebbe un participation_status
// locale gia' avanzato (V3). Presenza assente -> mantiene l'assegnazione e segnala (#141).
func (s *SQLStore) rereadAndPersistAttendance(ctx context.Context, cli *factorial.Client, principal Principal, newAccessIDs []string, accessLinks []enrollmentSessionLink, result *outboundSyncResult) error {
	attendances, err := cli.Trainings.SessionAttendances.All(ctx, &factorial.TrainingsSessionAttendancesListParams{SessionAccessMembershipIDs: newAccessIDs})
	if err != nil {
		issue, failErr := classifyOutbound("attendance_reread_failed", strings.Join(newAccessIDs, ","), err)
		if failErr != nil {
			return failErr
		}
		result.Warnings = append(result.Warnings, issue)
		return nil
	}
	foundForAccess := map[string]bool{}
	txErr := s.withTx(ctx, func(tx *sql.Tx) error {
		for _, att := range attendances {
			if att.ID == nil || att.SessionAccessMembershipID == nil {
				continue
			}
			checkpoint := ""
			if att.Status != nil {
				if mapped, err := attendanceLocalFromRemote(*att.Status); err == nil {
					checkpoint = mapped
				}
			}
			for _, l := range accessLinks {
				if l.Value != *att.SessionAccessMembershipID {
					continue
				}
				if _, err := tx.ExecContext(ctx, `
UPDATE training.enrollment_session SET factorial_attendance_id = $3, factorial_synced_status = NULLIF($4, ''), updated_at = now()
WHERE enrollment_id = $1::uuid AND session_id = $2::uuid`, l.EnrollmentID, l.SessionID, *att.ID, checkpoint); err != nil {
					return fmt.Errorf("persist attendance link and checkpoint: %w", err)
				}
				if err := s.auditFields(ctx, tx, principal, "enrollment_session", l.EnrollmentID, actionFactorialExport, []string{"factorial_attendance_id", "factorial_synced_status"}); err != nil {
					return err
				}
				foundForAccess[l.Value] = true
			}
		}
		return nil
	})
	if txErr != nil {
		return txErr
	}
	for _, l := range accessLinks {
		if !foundForAccess[l.Value] {
			result.Warnings = append(result.Warnings, outboundIssue{Kind: "attendance_missing_after_access_create", Ref: l.EnrollmentID})
		}
	}
	return nil
}

// exportAttendancePropagation propaga al remoto le presenze gia' collegate
// con decisione 3-way syncPropagateLocal; quelle appena collegate da
// exportMembershipAndAccess restano al prossimo run (senza checkpoint la 3-way non ha un vincitore, #141).
func (s *SQLStore) exportAttendancePropagation(ctx context.Context, cli *factorial.Client, idx graphIndex, course outboundCourseRow, state outboundState, dryRun bool, principal Principal, result *outboundSyncResult) error {
	for _, ev := range state.EventsByCourse[course.ID] {
		for _, sess := range state.SessionsByEvent[ev.ID] {
			for _, p := range state.ParticipantsBySession[sess.ID] {
				if p.AttendanceID == "" {
					continue
				}
				remote, ok := idx.attendanceByID[p.AttendanceID]
				if !ok || remote.Status == nil {
					continue
				}
				remoteLocalStatus, err := attendanceLocalFromRemote(*remote.Status)
				if err != nil {
					continue // stato remoto sconosciuto: nessuna propagazione (non abortisce la run)
				}
				if threeWayDecision(p.Status, remoteLocalStatus, p.Checkpoint) != syncPropagateLocal {
					continue
				}
				pushStatus, err := attendanceRemoteFromLocal(p.Status)
				if err != nil {
					continue
				}
				if dryRun {
					result.Planned = append(result.Planned, outboundIssue{Kind: "attendance_propagate", Ref: p.AttendanceID})
					continue
				}
				if _, err := cli.Trainings.SessionAttendances.BulkUpdate(ctx, attendanceBulkUpdateBody(p.AttendanceID, pushStatus)); err != nil {
					issue, failErr := classifyOutbound("attendance_update_failed", p.AttendanceID, err)
					if failErr != nil {
						return failErr
					}
					result.Warnings = append(result.Warnings, issue)
					continue
				}
				link := enrollmentSessionLink{EnrollmentID: p.EnrollmentID, SessionID: sess.ID, Value: p.Status}
				if err := s.persistEnrollmentSessionLinks(ctx, principal, "factorial_synced_status", []enrollmentSessionLink{link}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
