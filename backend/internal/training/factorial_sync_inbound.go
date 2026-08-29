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

// Diff inbound puro e applicazione in transazioni brevi (#141, slice 4/8 =
// #146): dal grafo Factorial perimetrato (slice 3) al dominio locale. La
// parte diff (fino ad applyInboundSync) e' pura: zero IO, solo il
// sotto-grafo di un Training e lo stato locale gia' letto
// (factorial_sync_local.go), produce liste piatte di mutazioni, conflitti e
// warning. L'applicazione e' l'unica parte con IO: una transazione breve
// per Training, ordine cablato corsi -> eventi -> sessioni -> iscrizioni ->
// presenze. Nessuna chiamata Factorial: il grafo arriva gia' fetchato.

// actionFactorialImport e' l'action di audit_log per le scritture generate
// da questo sync; e' anche il marcatore di provenienza che decide le
// sessioni collegate senza checkpoint (vedi resolveSession).
const actionFactorialImport = "factorial_import"

// Esiti condivisi dei resolver (sessione, presenza).
const (
	resolveAdopt    = "adopt"
	resolveNoop     = "noop"
	resolveConflict = "conflict"
	resolveUnknown  = "unknown"
)

type courseSeedItem struct {
	Create              bool
	CourseID            string // valorizzato quando !Create
	FactorialTrainingID string
	Title               string
	Description         string
	TitleMissing        bool
}

type eventSeedItem struct {
	FactorialClassID string
}

type sessionChangeItem struct {
	New                bool
	FactorialClassID   string
	FactorialSessionID string
	SessionID          string // valorizzato quando !New
	Remote             sessionSyncState
}

type enrollmentSeedItem struct {
	FactorialClassID   string
	EmployeeExternalID string
}

type accessAssignItem struct {
	FactorialClassID            string
	FactorialSessionID          string
	EmployeeExternalID          string
	FactorialAccessMembershipID string
	FactorialAttendanceID       string
	// NeedsWrite: false quando la riga locale ha gia' questi stessi id
	// (idempotenza); l'item resta comunque nella lista per far risolvere
	// alla fase presenze la coppia (enrollment, sessione) dell'access.
	NeedsWrite bool
}

type attendanceChangeItem struct {
	FactorialAccessMembershipID string
	NewStatus                   string
}

// inboundIssue e' una voce di conflitto o warning: Kind un codice stabile,
// Ref il miglior identificatore disponibile (id Factorial o locale).
// LocalEntity/LocalID/EmployeeID: riferimento locale gia' disponibile nel
// punto di generazione (nessuna lookup aggiuntiva), per la persistenza dei
// finding (#154); vuoti quando non risolvibile li'.
type inboundIssue struct {
	Kind        string
	Ref         string
	LocalEntity string
	LocalID     string
	EmployeeID  string
}

type trainingDiff struct {
	Course      *courseSeedItem
	Events      []eventSeedItem
	Sessions    []sessionChangeItem
	Enrollments []enrollmentSeedItem
	Assigns     []accessAssignItem
	Attendances []attendanceChangeItem
	Memberships []factorial.TrainingsTrainingMembership
	Conflicts   []inboundIssue
	Warnings    []inboundIssue
}

// trainingSubgraph isola, dal grafo gia' perimetrato, il sotto-grafo di un
// singolo Training: le sue classi, le sessioni delle classi, e le relazioni
// (membership per Training, access per sessione, attendance per access). Il
// grafo in ingresso e' gia' perimetrato (slice 3): qui si raggruppa solo per
// Training, senza rifiltrare gli attivi.
func trainingSubgraph(trainingID string, graph factorialTrainingGraph) trainingClassPerimeter {
	var sub trainingClassPerimeter
	classIDs := map[string]struct{}{}
	for _, c := range graph.TrainingClasses {
		if c.TrainingID == nil || *c.TrainingID != trainingID {
			continue
		}
		sub.Classes = append(sub.Classes, c)
		if c.ID != nil {
			classIDs[*c.ID] = struct{}{}
		}
	}
	sessionIDs := map[string]struct{}{}
	for _, sess := range graph.Sessions {
		if sess.TrainingClassID == nil {
			continue
		}
		if _, ok := classIDs[*sess.TrainingClassID]; !ok {
			continue
		}
		sub.Sessions = append(sub.Sessions, sess)
		if sess.ID != nil {
			sessionIDs[*sess.ID] = struct{}{}
		}
	}
	for _, m := range graph.TrainingMemberships {
		if m.TrainingID != nil && *m.TrainingID == trainingID {
			sub.TrainingMemberships = append(sub.TrainingMemberships, m)
		}
	}
	accessIDs := map[string]struct{}{}
	for _, a := range graph.SessionAccessMemberships {
		if a.SessionID == nil {
			continue
		}
		if _, ok := sessionIDs[*a.SessionID]; !ok {
			continue
		}
		sub.Access = append(sub.Access, a)
		if a.ID != nil {
			accessIDs[*a.ID] = struct{}{}
		}
	}
	for _, att := range graph.SessionAttendances {
		if att.SessionAccessMembershipID == nil {
			continue
		}
		if _, ok := accessIDs[*att.SessionAccessMembershipID]; ok {
			sub.Attendances = append(sub.Attendances, att)
		}
	}
	return sub
}

// computeTrainingDiff produce il diff inbound puro di un Training: nessun
// accesso IO, solo il sotto-grafo (trainingSubgraph) e lo stato locale
// (loadLocalTrainingState).
func computeTrainingDiff(training factorial.TrainingsTraining, sub trainingClassPerimeter, local localTrainingState) trainingDiff {
	var diff trainingDiff
	trainingID := deref(training.ID)
	var course *localCourse
	if c, ok := local.Courses[trainingID]; ok {
		course = &c
	}
	diff.Course = courseSeed(trainingID, training, course)
	if diff.Course != nil && diff.Course.TitleMissing {
		localCourseID := ""
		if course != nil {
			localCourseID = course.ID
		}
		diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "course_title_missing", Ref: trainingID, LocalEntity: "course", LocalID: localCourseID})
	}

	keptClasses := map[string]struct{}{}
	for _, class := range sub.Classes {
		if class.ID == nil {
			continue
		}
		event, found := local.Events[*class.ID]
		if found && event.Cancelled {
			diff.Conflicts = append(diff.Conflicts, inboundIssue{Kind: "event_reopen_blocked", Ref: *class.ID, LocalEntity: "training_event", LocalID: event.ID})
			continue
		}
		keptClasses[*class.ID] = struct{}{}
		if !found {
			diff.Events = append(diff.Events, eventSeedItem{FactorialClassID: *class.ID})
		}
	}

	sessionClass := map[string]string{}
	for _, sess := range sub.Sessions {
		if sess.ID == nil || sess.TrainingClassID == nil {
			continue
		}
		if _, kept := keptClasses[*sess.TrainingClassID]; !kept {
			continue // classe cancellata localmente: gia' in conflitto sopra
		}
		sessionClass[*sess.ID] = *sess.TrainingClassID
		existing, found := local.Sessions[*sess.ID]
		remote, endsCleared := sanitizeRemoteSession(sess)
		if endsCleared {
			issue := inboundIssue{Kind: "session_ends_before_starts", Ref: *sess.ID}
			if found {
				issue.LocalEntity, issue.LocalID = "training_session", existing.ID
			}
			diff.Warnings = append(diff.Warnings, issue)
		}
		if !found {
			diff.Sessions = append(diff.Sessions, sessionChangeItem{New: true, FactorialClassID: *sess.TrainingClassID, FactorialSessionID: *sess.ID, Remote: remote})
			continue
		}
		switch resolveSession(existing, remote) {
		case resolveAdopt:
			diff.Sessions = append(diff.Sessions, sessionChangeItem{FactorialClassID: *sess.TrainingClassID, FactorialSessionID: *sess.ID, SessionID: existing.ID, Remote: remote})
		case resolveConflict:
			diff.Conflicts = append(diff.Conflicts, inboundIssue{Kind: "session_conflict", Ref: *sess.ID, LocalEntity: "training_session", LocalID: existing.ID})
		case resolveUnknown:
			diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "session_provenance_unknown", Ref: *sess.ID, LocalEntity: "training_session", LocalID: existing.ID})
		}
	}

	attendanceByAccess := map[string]factorial.TrainingsSessionAttendance{}
	for _, att := range sub.Attendances {
		if att.SessionAccessMembershipID != nil {
			attendanceByAccess[*att.SessionAccessMembershipID] = att
		}
	}
	seeded := map[[2]string]struct{}{}
	for _, acc := range sub.Access {
		if acc.ID == nil || acc.SessionID == nil || acc.EmployeeID == nil {
			continue
		}
		classID, ok := sessionClass[*acc.SessionID]
		if !ok {
			continue // sessione fuori perimetro (classe cancellata): gia' in conflitto
		}
		employeeExternalID := *acc.EmployeeID
		employeeLocalID, resolved := local.Employees[employeeExternalID]
		if !resolved {
			diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "employee_unresolved", Ref: employeeExternalID})
			continue
		}
		eventID := local.Events[classID].ID // "" se l'evento e' nuovo in questo giro
		enrollment, hasEnrollment := local.Enrollments[[2]string{employeeLocalID, eventID}]
		if !hasEnrollment {
			seedKey := [2]string{classID, employeeExternalID}
			if _, already := seeded[seedKey]; !already {
				seeded[seedKey] = struct{}{}
				diff.Enrollments = append(diff.Enrollments, enrollmentSeedItem{FactorialClassID: classID, EmployeeExternalID: employeeExternalID})
			}
		} else if enrollment.Cancelled {
			diff.Conflicts = append(diff.Conflicts, inboundIssue{Kind: "enrollment_reopen_blocked", Ref: enrollment.ID, LocalEntity: "enrollment", LocalID: enrollment.ID, EmployeeID: employeeLocalID})
			continue
		}
		attendanceID := ""
		remoteAttendance, hasAttendance := attendanceByAccess[*acc.ID]
		if hasAttendance && remoteAttendance.ID != nil {
			attendanceID = *remoteAttendance.ID
		}
		session := local.Sessions[*acc.SessionID] // ID == "" se la sessione e' nuova in questo giro
		existingAssign, hasAssign := local.Assigns[[2]string{enrollment.ID, session.ID}]
		needsWrite := !hasAssign || existingAssign.FactorialAccessMembershipID != *acc.ID || existingAssign.FactorialAttendanceID != attendanceID
		diff.Assigns = append(diff.Assigns, accessAssignItem{
			FactorialClassID: classID, FactorialSessionID: *acc.SessionID,
			EmployeeExternalID: employeeExternalID, FactorialAccessMembershipID: *acc.ID,
			FactorialAttendanceID: attendanceID, NeedsWrite: needsWrite,
		})
		if attendanceID == "" || remoteAttendance.Status == nil {
			continue
		}
		remoteStatus, err := attendanceLocalFromRemote(*remoteAttendance.Status)
		if err != nil {
			diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "attendance_status_unknown", Ref: attendanceID, LocalEntity: "enrollment", LocalID: enrollment.ID, EmployeeID: employeeLocalID})
			continue
		}
		localStatus, checkpoint := participationAssigned, (*string)(nil)
		if hasAssign {
			localStatus, checkpoint = existingAssign.Status, existingAssign.Checkpoint
		}
		switch resolveAttendance(localStatus, remoteStatus, checkpoint) {
		case resolveAdopt:
			diff.Attendances = append(diff.Attendances, attendanceChangeItem{FactorialAccessMembershipID: *acc.ID, NewStatus: remoteStatus})
		case resolveConflict:
			diff.Conflicts = append(diff.Conflicts, inboundIssue{Kind: "attendance_conflict", Ref: attendanceID, LocalEntity: "enrollment", LocalID: enrollment.ID, EmployeeID: employeeLocalID})
		}
	}
	// Iscrizione dalla sola membership: una persona iscritta al corso su
	// Factorial ma senza alcuna sessione assegnata non deve sparire. Con un
	// solo evento reale l'aggancio e' univoco (iscritta al corso = iscritta
	// alla sua unica iniziativa) e nasce senza assegnazioni di sessione,
	// stato locale gia' valido. Con zero o piu' eventi la collocazione non
	// e' deducibile dai dati Factorial: warning, mai un aggancio indovinato.
	// Un'iscrizione locale gia' esistente (anche annullata) vince: nessuna
	// ricreazione e nessuna riapertura.
	onlyClass := ""
	if len(keptClasses) == 1 {
		for classID := range keptClasses {
			onlyClass = classID
		}
	}
	for _, m := range sub.TrainingMemberships {
		if m.ID == nil || m.EmployeeID == nil {
			continue
		}
		employeeLocalID, resolved := local.Employees[*m.EmployeeID]
		if !resolved {
			diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "employee_unresolved", Ref: *m.EmployeeID})
			continue
		}
		enrolled := false
		for classID := range keptClasses {
			if _, alreadySeeded := seeded[[2]string{classID, *m.EmployeeID}]; alreadySeeded {
				enrolled = true
				break
			}
			if ev, ok := local.Events[classID]; ok && ev.ID != "" {
				if _, exists := local.Enrollments[[2]string{employeeLocalID, ev.ID}]; exists {
					enrolled = true
					break
				}
			}
		}
		if enrolled {
			continue
		}
		if onlyClass == "" {
			diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "membership_without_event", Ref: deref(m.ID), EmployeeID: employeeLocalID})
			continue
		}
		seeded[[2]string{onlyClass, *m.EmployeeID}] = struct{}{}
		diff.Enrollments = append(diff.Enrollments, enrollmentSeedItem{FactorialClassID: onlyClass, EmployeeExternalID: *m.EmployeeID})
	}

	diff.Memberships = sub.TrainingMemberships
	return diff
}

// ghostDuplicateClasses individua le classi fantasma sull'INTERO grafo
// fetchato, prima del perimetro attivi: copie esatte (training + nome +
// data inizio + data fine) senza sessioni di una gemella con sessioni.
// Impronta della duplicazione di massa osservata sui dati reali Factorial
// (160 classi su 161 vuote nel perimetro importato, nessuna copia
// successiva a luglio 2024). Va calcolata sul grafo completo: la gemella
// piena puo' avere solo partecipanti cessati e quindi sparire dal
// perimetro, ma il fantasma resta un fantasma. Una classe vuota senza
// gemella piena resta legittima: fase organizzativa, non fantasma.
// Ritorna anche l'ordine di apparizione (con il training di appartenenza,
// per filtrare i warning al solo perimetro) per warning deterministici.
func ghostDuplicateClasses(graph factorialTrainingGraph) (map[string]struct{}, []ghostClassRef) {
	sessionCount := map[string]int{}
	for _, sess := range graph.Sessions {
		if sess.TrainingClassID != nil {
			sessionCount[*sess.TrainingClassID]++
		}
	}
	type classKey struct{ training, name, start, end string }
	dateStr := func(d *factorial.Date) string {
		if d == nil {
			return ""
		}
		return d.String()
	}
	groups := map[classKey][]string{}
	full := map[classKey]bool{}
	for _, class := range graph.TrainingClasses {
		if class.ID == nil || class.TrainingID == nil {
			continue
		}
		key := classKey{*class.TrainingID, deref(class.Name), dateStr(class.StartDate), dateStr(class.EndDate)}
		groups[key] = append(groups[key], *class.ID)
		if sessionCount[*class.ID] > 0 {
			full[key] = true
		}
	}
	ghosts := map[string]struct{}{}
	for key, ids := range groups {
		if len(ids) < 2 || !full[key] {
			continue
		}
		for _, id := range ids {
			if sessionCount[id] == 0 {
				ghosts[id] = struct{}{}
			}
		}
	}
	ordered := make([]ghostClassRef, 0, len(ghosts))
	for _, class := range graph.TrainingClasses {
		if class.ID == nil || class.TrainingID == nil {
			continue
		}
		if _, ok := ghosts[*class.ID]; ok {
			ordered = append(ordered, ghostClassRef{ClassID: *class.ID, TrainingID: *class.TrainingID})
		}
	}
	return ghosts, ordered
}

// ghostClassRef: classe fantasma con il training di appartenenza.
type ghostClassRef struct {
	ClassID    string
	TrainingID string
}

// removeGhostClasses toglie dal grafo le classi fantasma (che per
// definizione non hanno sessioni ne' relazioni): il perimetro e l'inbound
// non le vedono, nessun evento viene creato. Una gia' importata in passato
// resta locale (l'inbound non cancella mai): si elimina col ripopolamento.
func removeGhostClasses(graph factorialTrainingGraph, ghosts map[string]struct{}) factorialTrainingGraph {
	graph.TrainingClasses = excludeByID(graph.TrainingClasses, ghosts, func(c factorial.TrainingsTrainingClass) *string { return c.ID })
	return graph
}

// courseSeed decide il seed del corso: nil quando il locale e' gia' attivo,
// o quando titolo/descrizione sono gia' allineati col remoto (idempotenza,
// anche a corso inattivo). Altrimenti titolo/descrizione dal remoto; titolo
// assente -> fallback "Training Factorial <id>" e warning (TitleMissing).
func courseSeed(trainingID string, training factorial.TrainingsTraining, course *localCourse) *courseSeedItem {
	if course != nil && course.IsActive {
		return nil
	}
	title := strings.TrimSpace(deref(training.Name))
	missing := title == ""
	if missing {
		title = "Training Factorial " + trainingID
	}
	description := deref(training.Description)
	if course != nil && course.Title == title && course.Description == description {
		return nil // gia' allineato: nessuna mutazione (idempotenza)
	}
	item := courseSeedItem{FactorialTrainingID: trainingID, Title: title, Description: description, TitleMissing: missing}
	if course == nil {
		item.Create = true
	} else {
		item.CourseID = course.ID
	}
	return &item
}

// sanitizeRemoteSession proietta una sessione remota e applica la regola
// "ends_at < starts_at -> conserva start, importa ends_at=NULL" (il secondo
// valore di ritorno segnala che la pulizia e' scattata).
func sanitizeRemoteSession(remote factorial.TrainingsSession) (sessionSyncState, bool) {
	state := newSessionSyncStateFromRemote(remote)
	if state.StartsAt == "" || state.EndsAt == "" {
		return state, false
	}
	starts, err1 := time.Parse(time.RFC3339, state.StartsAt)
	ends, err2 := time.Parse(time.RFC3339, state.EndsAt)
	if err1 != nil || err2 != nil || !ends.Before(starts) {
		return state, false
	}
	state.EndsAt = ""
	return state, true
}

// sessionDueAtParam converte YYYY-MM-DD in mezzanotte UTC esplicita per
// ::timestamptz: un cast diretto da ::date userebbe il fuso di sessione del
// DB e farebbe derivare il checkpoint ad ogni round-trip.
func sessionDueAtParam(dueDate string) string {
	if dueDate == "" {
		return ""
	}
	return dueDate + "T00:00:00Z"
}

// resolveWithCheckpoint applica la decisione 3-way quando il checkpoint
// esiste (adopt su in-sync/propagate-remoto, conflict su conflitto, noop su
// propagate-locale: senza spinta in uscita in questa slice non c'e' nulla da
// scrivere). Quando il checkpoint manca ritorna "" per ogni caso salvo i
// lati gia' allineati (adopt, senza cercare un vincitore): il chiamante
// decide il resto (provenienza per le sessioni, adozione diretta per le
// presenze).
func resolveWithCheckpoint[T comparable](local, remote T, checkpoint *T) string {
	if checkpoint == nil {
		if local == remote {
			return resolveAdopt
		}
		return ""
	}
	switch threeWayDecision(local, remote, checkpoint) {
	case syncInSync:
		if *checkpoint == remote {
			return resolveNoop // gia' tutto allineato, checkpoint compreso: idempotente
		}
		return resolveAdopt // allineati ma il checkpoint e' da aggiornare
	case syncPropagateRemote:
		return resolveAdopt
	case syncConflict:
		return resolveConflict
	default: // syncPropagateLocal
		return resolveNoop
	}
}

func resolveSession(existing localSession, remote sessionSyncState) string {
	if out := resolveWithCheckpoint(existing.Local, remote, existing.Checkpoint); out != "" {
		return out
	}
	switch existing.Provenance {
	case actionFactorialImport:
		return resolveAdopt
	case "create", "assign":
		return resolveNoop
	default:
		return resolveUnknown // provenienza non dimostrabile: nessun vincitore arbitrario
	}
}

func resolveAttendance(local, remote string, checkpoint *string) string {
	if out := resolveWithCheckpoint(local, remote, checkpoint); out != "" {
		return out
	}
	return resolveAdopt // nessun checkpoint, lati diversi: prima sincronizzazione (access appena creato)
}

// inboundSyncResult riassume l'esito di applyInboundSync su tutti i Training
// del grafo perimetrato: conflitti e warning accumulati, e il numero di
// eccezioni RDA (presenze applicate bypassando il gate locale
// planned->in_progress perche' il fatto esterno e' gia' avvenuto).
type inboundSyncResult struct {
	Conflicts     []inboundIssue
	Warnings      []inboundIssue
	RDAExceptions int
}

// applyInboundSync applica il grafo Factorial perimetrato al dominio
// locale: per ogni Training, legge lo stato locale, calcola il diff puro e
// lo applica in una transazione breve. In dry-run il diff resta puro: si
// accumulano conflitti/warning ma non si apre alcuna transazione ne' si
// chiama applyTrainingDiff (#141, slice 7). Un errore DB abortisce l'intera
// run; gli errori sui singoli dati sono gia' isolati dal diff
// (conflitti/warning) e non raggiungono mai una scrittura.
func (s *SQLStore) applyInboundSync(ctx context.Context, graph factorialTrainingGraph, dryRun bool) (inboundSyncResult, error) {
	if s == nil || s.db == nil {
		return inboundSyncResult{}, errors.New("training database not configured")
	}
	principal := Principal{IsPeopleAdmin: true}
	// Bulk unico per l'intera run: nessuna entita' di un Training dipende da
	// scritture di un altro, uno snapshot preso ora resta corretto.
	local, err := s.loadLocalSyncState(ctx, s.db)
	if err != nil {
		return inboundSyncResult{}, err
	}
	var result inboundSyncResult
	for _, training := range graph.Trainings {
		if training.ID == nil {
			continue
		}
		sub := trainingSubgraph(*training.ID, graph)
		diff := computeTrainingDiff(training, sub, local)
		result.Conflicts = append(result.Conflicts, diff.Conflicts...)
		result.Warnings = append(result.Warnings, diff.Warnings...)
		if dryRun {
			continue
		}
		if err := s.withTx(ctx, func(tx *sql.Tx) error {
			rdaIssues, err := s.applyTrainingDiff(ctx, tx, principal, *training.ID, sub, local, diff)
			if err != nil {
				return err
			}
			result.Warnings = append(result.Warnings, rdaIssues...)
			result.RDAExceptions += len(rdaIssues)
			return nil
		}); err != nil {
			return result, err
		}
	}
	return result, nil
}

// applyTrainingDiff scrive il diff di un Training nell'ordine cablato corsi
// -> eventi -> sessioni -> iscrizioni -> presenze, dentro la transazione
// breve del chiamante. Ritorna le eccezioni RDA della fase presenze, come
// inboundIssue (segnalate, non solo contate).
func (s *SQLStore) applyTrainingDiff(ctx context.Context, tx *sql.Tx, principal Principal, trainingID string, sub trainingClassPerimeter, local localTrainingState, diff trainingDiff) ([]inboundIssue, error) {
	courseID := ""
	if c, ok := local.Courses[trainingID]; ok {
		courseID = c.ID
	}
	if diff.Course != nil {
		id, err := s.applyCourseSeed(ctx, tx, principal, *diff.Course)
		if err != nil {
			return nil, err
		}
		courseID = id
	}

	// Le mappe di lavoro sono seminate dal SOLO sotto-grafo di questo
	// Training (sub), mai da local.Events/Sessions/Enrollments per intero:
	// quelle sono globali alla run (bulk, factorial_sync_local.go) e un
	// giro su tutti i loro valori mescolerebbe entita' di Training diversi
	// (es. la membership di un impiegato iscritto a due Training).
	eventID := map[string]string{}
	trainingEventIDs := map[string]struct{}{} // valori di eventID: filtra local.Enrollments piu' sotto
	for _, class := range sub.Classes {
		if class.ID == nil {
			continue
		}
		if e, ok := local.Events[*class.ID]; ok {
			eventID[*class.ID], trainingEventIDs[e.ID] = e.ID, struct{}{}
		}
	}
	for _, item := range diff.Events {
		var id string
		if err := tx.QueryRowContext(ctx, `
INSERT INTO training.training_event (course_id, origin, factorial_class_id) VALUES ($1::uuid, 'factorial_import', $2)
RETURNING id::text`, courseID, item.FactorialClassID).Scan(&id); err != nil {
			return nil, fmt.Errorf("create training event from factorial sync: %w", err)
		}
		if err := s.auditFields(ctx, tx, principal, "training_event", id, actionFactorialImport, []string{"course_id", "factorial_class_id"}); err != nil {
			return nil, err
		}
		eventID[item.FactorialClassID] = id
	}

	sessionID := map[string]string{}
	for _, sess := range sub.Sessions {
		if sess.ID == nil {
			continue
		}
		if ls, ok := local.Sessions[*sess.ID]; ok {
			sessionID[*sess.ID] = ls.ID
		}
	}
	for _, item := range diff.Sessions {
		ev, ok := eventID[item.FactorialClassID]
		if !ok {
			continue
		}
		checkpoint, err := json.Marshal(item.Remote)
		if err != nil {
			return nil, fmt.Errorf("marshal training session checkpoint: %w", err)
		}
		dueAt := sessionDueAtParam(item.Remote.DueDate)
		if item.New {
			var id string
			if err := tx.QueryRowContext(ctx, `
INSERT INTO training.training_session (event_id, schedule_type, starts_at, ends_at, due_at, factorial_session_id, factorial_synced_state)
VALUES ($1::uuid, NULLIF($2,''), NULLIF($3,'')::timestamptz, NULLIF($4,'')::timestamptz, NULLIF($5,'')::timestamptz, $6, $7::jsonb)
RETURNING id::text`, ev, item.Remote.ScheduleType, item.Remote.StartsAt, item.Remote.EndsAt, dueAt, item.FactorialSessionID, checkpoint).Scan(&id); err != nil {
				return nil, fmt.Errorf("create training session from factorial sync: %w", err)
			}
			if err := s.auditFields(ctx, tx, principal, "training_session", id, actionFactorialImport, []string{"factorial_session_id"}); err != nil {
				return nil, err
			}
			sessionID[item.FactorialSessionID] = id
		} else {
			if _, err := tx.ExecContext(ctx, `
UPDATE training.training_session
SET schedule_type = NULLIF($2,''), starts_at = NULLIF($3,'')::timestamptz, ends_at = NULLIF($4,'')::timestamptz,
    due_at = NULLIF($5,'')::timestamptz, factorial_synced_state = $6::jsonb, updated_at = now()
WHERE id = $1::uuid`, item.SessionID, item.Remote.ScheduleType, item.Remote.StartsAt, item.Remote.EndsAt, dueAt, checkpoint); err != nil {
				return nil, fmt.Errorf("update training session from factorial sync: %w", err)
			}
			if err := s.auditFields(ctx, tx, principal, "training_session", item.SessionID, actionFactorialImport, []string{"schedule_type", "starts_at", "ends_at", "due_at"}); err != nil {
				return nil, err
			}
			sessionID[item.FactorialSessionID] = item.SessionID
		}
	}

	enrollmentID := map[[2]string]string{}
	for k, v := range local.Enrollments {
		if _, ok := trainingEventIDs[k[1]]; ok {
			enrollmentID[k] = v.ID
		}
	}
	for _, item := range diff.Enrollments {
		employee, ok1 := local.Employees[item.EmployeeExternalID]
		ev, ok2 := eventID[item.FactorialClassID]
		if !ok1 || !ok2 {
			continue
		}
		var id string
		if err := tx.QueryRowContext(ctx, `
INSERT INTO training.enrollment (employee_id, event_id, origin) VALUES ($1::uuid, $2::uuid, 'factorial_import')
RETURNING id::text`, employee, ev).Scan(&id); err != nil {
			return nil, fmt.Errorf("create training enrollment from factorial sync: %w", err)
		}
		if err := s.auditFields(ctx, tx, principal, "enrollment", id, actionFactorialImport, []string{"employee_id", "event_id"}); err != nil {
			return nil, err
		}
		enrollmentID[[2]string{employee, ev}] = id
	}

	assignKey := map[string][2]string{} // access id -> [enrollmentID, sessionID]
	for _, item := range diff.Assigns {
		employee, ok1 := local.Employees[item.EmployeeExternalID]
		ev, ok2 := eventID[item.FactorialClassID]
		sess, ok3 := sessionID[item.FactorialSessionID]
		if !ok1 || !ok2 || !ok3 {
			continue
		}
		enr, ok4 := enrollmentID[[2]string{employee, ev}]
		if !ok4 {
			continue
		}
		assignKey[item.FactorialAccessMembershipID] = [2]string{enr, sess}
		if !item.NeedsWrite {
			continue // gia' allineato: nessuna scrittura, nessun audit (idempotenza)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO training.enrollment_session (enrollment_id, session_id, factorial_access_membership_id, factorial_attendance_id)
VALUES ($1::uuid, $2::uuid, $3, NULLIF($4, '')) ON CONFLICT (enrollment_id, session_id) DO UPDATE
SET factorial_access_membership_id = EXCLUDED.factorial_access_membership_id,
    factorial_attendance_id = COALESCE(EXCLUDED.factorial_attendance_id, training.enrollment_session.factorial_attendance_id)`,
			enr, sess, item.FactorialAccessMembershipID, item.FactorialAttendanceID); err != nil {
			return nil, fmt.Errorf("assign training enrollment session from factorial sync: %w", err)
		}
		if err := s.auditFields(ctx, tx, principal, "enrollment_session", enr, actionFactorialImport, []string{"factorial_access_membership_id"}); err != nil {
			return nil, err
		}
	}

	for _, m := range diff.Memberships {
		if m.ID == nil || m.EmployeeID == nil {
			continue
		}
		employee, ok := local.Employees[*m.EmployeeID]
		if !ok {
			continue
		}
		for _, ev := range eventID {
			enr, ok := enrollmentID[[2]string{employee, ev}]
			if !ok {
				continue
			}
			res, err := tx.ExecContext(ctx, `
UPDATE training.enrollment SET factorial_training_membership_id = $2
WHERE id = $1::uuid AND factorial_training_membership_id IS DISTINCT FROM $2`, enr, *m.ID)
			if err != nil {
				return nil, fmt.Errorf("link training membership id from factorial sync: %w", err)
			}
			if changed, _ := res.RowsAffected(); changed == 0 {
				continue // gia' allineato: nessun audit (idempotenza)
			}
			if err := s.auditFields(ctx, tx, principal, "enrollment", enr, actionFactorialImport, []string{"factorial_training_membership_id"}); err != nil {
				return nil, err
			}
		}
	}

	var rdaIssues []inboundIssue
	for _, item := range diff.Attendances {
		key, ok := assignKey[item.FactorialAccessMembershipID]
		if !ok {
			continue
		}
		enr, sess := key[0], key[1]
		if _, err := tx.ExecContext(ctx, `
UPDATE training.enrollment_session
SET participation_status = $3, factorial_synced_status = $3, updated_at = now()
WHERE enrollment_id = $1::uuid AND session_id = $2::uuid`, enr, sess, item.NewStatus); err != nil {
			return nil, fmt.Errorf("update training attendance from factorial sync: %w", err)
		}
		if err := s.auditFields(ctx, tx, principal, "enrollment_session", enr, actionFactorialImport, []string{"participation_status"}); err != nil {
			return nil, err
		}
		var before, employeeID string
		if err := tx.QueryRowContext(ctx, `SELECT delivery_status, employee_id::text FROM training.enrollment WHERE id = $1::uuid FOR UPDATE`, enr).Scan(&before, &employeeID); err != nil {
			return nil, fmt.Errorf("load training enrollment delivery status: %w", err)
		}
		// startGate=nil: la scrittura inbound non passa mai dal gate locale
		// (eccezione RDA, #141); requiresEnrollmentStartGate qui sotto conta
		// solo la discrepanza, non la blocca.
		after, err := s.reconcileDeliveryStatus(ctx, tx, principal, enr, nil)
		if err != nil {
			return nil, err
		}
		if requiresEnrollmentStartGate(before, after) {
			rdaIssues = append(rdaIssues, inboundIssue{Kind: "rda_exception", Ref: enr, LocalEntity: "enrollment", LocalID: enr, EmployeeID: employeeID})
		}
	}
	return rdaIssues, nil
}

// applyCourseSeed crea o riaggiorna (solo se ancora inattivo) il corso
// seed di un Training, e ritorna il suo id.
func (s *SQLStore) applyCourseSeed(ctx context.Context, tx *sql.Tx, principal Principal, item courseSeedItem) (string, error) {
	if item.Create {
		var id string
		if err := tx.QueryRowContext(ctx, `
INSERT INTO training.course (title, description, is_active, factorial_training_id) VALUES ($1, NULLIF($2, ''), false, $3)
RETURNING id::text`, item.Title, item.Description, item.FactorialTrainingID).Scan(&id); err != nil {
			return "", fmt.Errorf("create training course from factorial sync: %w", err)
		}
		if err := s.auditFields(ctx, tx, principal, "course", id, actionFactorialImport, []string{"title", "description", "factorial_training_id"}); err != nil {
			return "", err
		}
		return id, nil
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE training.course SET title = $2, description = NULLIF($3, ''), updated_at = now() WHERE id = $1::uuid`,
		item.CourseID, item.Title, item.Description); err != nil {
		return "", fmt.Errorf("reseed training course from factorial sync: %w", err)
	}
	if err := s.auditFields(ctx, tx, principal, "course", item.CourseID, actionFactorialImport, []string{"title", "description"}); err != nil {
		return "", err
	}
	return item.CourseID, nil
}
