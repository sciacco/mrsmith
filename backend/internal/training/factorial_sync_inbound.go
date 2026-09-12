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
	// Adopt (#172): true quando il seed non crea un corso nuovo ma adotta
	// un corso locale senza correlatore con titolo identico (embrione nato
	// da una richiesta, migrazione 142): la UPDATE scrive anche il
	// correlatore factorial_training_id. Primo contatto col sync: i campi
	// si seminano come alla nascita di un gemello; dal run successivo il
	// corso e' correlato a tutti gli effetti (e, una volta attivo, il sync
	// non lo tocca piu').
	Adopt bool
	// VendorName: fornitore abituale dal testo Factorial ("" = nessuno,
	// scartata la spazzatura "null"); ProviderKind: erogazione dal flag
	// external; Tags: categorie Factorial come tag liberi (esclusa
	// «Formazione interna», ridondante con ProviderKind).
	VendorName   string
	ProviderKind string
	Tags         []string
	// SkillAreas: competenze del Training risolte in aree (codice+nome,
	// ordinate per codice) via factorialCompetencyNames.
	SkillAreas []skillAreaSeed
}

// skillAreaSeed e' un'area di competenza da agganciare al corso seed:
// Code e' la chiave del get-or-create in anagrafica, Name il nome alla
// prima creazione.
type skillAreaSeed struct {
	Code string
	Name string
}

type eventSeedItem struct {
	FactorialClassID string
	// Notes: nome (ed eventuale descrizione) della classe Factorial, unico
	// posto dove l'edizione resta leggibile nel nostro modello senza nome-evento.
	Notes string
}

// eventNoteItem riempie le note di un evento gia' esistente, SOLO se vuote:
// mai sopra testo scritto da una persona.
type eventNoteItem struct {
	EventID string
	Notes   string
}

type sessionChangeItem struct {
	New                bool
	FactorialClassID   string
	FactorialSessionID string
	SessionID          string // valorizzato quando !New
	Remote             sessionSyncState
	Op                 sessionOpState
}

// sessionOpItem riallinea i soli dati operativi (argomento, modalita',
// durata, luogo) di una sessione il cui stato schedule/date e' gia' in sync:
// campi di proprieta' del remoto, nessun conflitto.
type sessionOpItem struct {
	SessionID string
	Op        sessionOpState
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

// attendanceHoursItem riallinea le ore completate di una partecipazione
// (consuntivo remoto, fuori dal checkpoint 3-way: mai un conflitto).
type attendanceHoursItem struct {
	FactorialAccessMembershipID string
	Hours                       string // forma canonica, "" = NULL
}

// inboundIssue e' una voce di conflitto o warning: Kind un codice stabile,
// Ref il miglior identificatore disponibile (id Factorial o locale).
// LocalEntity/LocalID/EmployeeID: riferimento locale gia' disponibile nel
// punto di generazione (nessuna lookup aggiuntiva), per la persistenza dei
// finding (#154); vuoti quando non risolvibile li'. Detail (#172): payload
// opzionale del finding (es. candidati di un'adozione ambigua), persistito
// nel detail jsonb.
type inboundIssue struct {
	Kind        string
	Ref         string
	LocalEntity string
	LocalID     string
	EmployeeID  string
	Detail      map[string]any
}

type trainingDiff struct {
	Course      *courseSeedItem
	Events      []eventSeedItem
	EventNotes  []eventNoteItem
	Sessions    []sessionChangeItem
	SessionOps  []sessionOpItem
	Enrollments []enrollmentSeedItem
	Assigns     []accessAssignItem
	Attendances []attendanceChangeItem
	Hours       []attendanceHoursItem
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
func computeTrainingDiff(training factorial.TrainingsTraining, sub trainingClassPerimeter, local localTrainingState, categories map[string]string) trainingDiff {
	var diff trainingDiff
	trainingID := deref(training.ID)
	var course *localCourse
	if c, ok := local.Courses[trainingID]; ok {
		course = &c
	}
	var unresolvedCategories, unresolvedCompetencies []string
	diff.Course, unresolvedCategories, unresolvedCompetencies = courseSeed(trainingID, training, course, categories)
	for _, categoryID := range unresolvedCategories {
		diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "course_category_unresolved", Ref: categoryID})
	}
	for _, competencyID := range unresolvedCompetencies {
		diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "course_competency_unresolved", Ref: competencyID})
	}
	if diff.Course != nil && diff.Course.TitleMissing {
		localCourseID := ""
		if course != nil {
			localCourseID = course.ID
		}
		diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "course_title_missing", Ref: trainingID, LocalEntity: "course", LocalID: localCourseID})
	}
	// Adozione embrione (#172): senza corso correlato, prima di creare un
	// gemello si cerca tra i corsi locali senza correlatore una corrispondenza
	// esatta di titolo, case-insensitive (stessa convenzione del riuso in
	// creazione richiesta). Un solo candidato -> adozione (correlatore + seed
	// dei campi, con audit e finding dedicato); piu' candidati -> si crea il
	// gemello come oggi, con warning che li elenca. Mai col titolo di
	// fallback: un TitleMissing non adotta.
	if course == nil && diff.Course != nil && diff.Course.Create && !diff.Course.TitleMissing {
		if candidates := local.UnlinkedCourses[strings.ToLower(diff.Course.Title)]; len(candidates) == 1 {
			diff.Course.Create = false
			diff.Course.CourseID = candidates[0]
			diff.Course.Adopt = true
			diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "course_adopted_by_title", Ref: trainingID, LocalEntity: "course", LocalID: candidates[0]})
		} else if len(candidates) > 1 {
			diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "course_adopt_ambiguous", Ref: trainingID, LocalEntity: "course", Detail: map[string]any{"candidates": candidates}})
		}
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
		notes := classNotes(class)
		if !found {
			diff.Events = append(diff.Events, eventSeedItem{FactorialClassID: *class.ID, Notes: notes})
			continue
		}
		if event.NotesEmpty && notes != "" {
			diff.EventNotes = append(diff.EventNotes, eventNoteItem{EventID: event.ID, Notes: notes})
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
		remoteOp, durationInvalid := newSessionOpStateFromRemote(sess)
		if durationInvalid {
			diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "session_duration_invalid", Ref: *sess.ID})
		}
		if !found {
			diff.Sessions = append(diff.Sessions, sessionChangeItem{New: true, FactorialClassID: *sess.TrainingClassID, FactorialSessionID: *sess.ID, Remote: remote, Op: remoteOp})
			continue
		}
		switch resolveSession(existing, remote) {
		case resolveAdopt:
			diff.Sessions = append(diff.Sessions, sessionChangeItem{FactorialClassID: *sess.TrainingClassID, FactorialSessionID: *sess.ID, SessionID: existing.ID, Remote: remote, Op: remoteOp})
		case resolveNoop:
			// Schedule/date gia' allineati: i dati operativi restano di
			// proprieta' del remoto e si riallineano da soli, senza conflitti.
			// Solo per sessioni gia' sincronizzate (checkpoint presente): su
			// una nata locale e mai checkpointata non si scrive nulla.
			if existing.Checkpoint != nil && existing.Op != remoteOp {
				diff.SessionOps = append(diff.SessionOps, sessionOpItem{SessionID: existing.ID, Op: remoteOp})
			}
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
		if attendanceID == "" {
			continue
		}
		// Ore completate: consuntivo di proprieta' del remoto, fuori dal
		// checkpoint 3-way. "0" equivale ad assente (nessun consuntivo).
		remoteHours, hoursOK := canonicalHours(deref(remoteAttendance.CompletedDuration))
		if !hoursOK {
			diff.Warnings = append(diff.Warnings, inboundIssue{Kind: "attendance_duration_invalid", Ref: attendanceID, LocalEntity: "enrollment", LocalID: enrollment.ID, EmployeeID: employeeLocalID})
		}
		if remoteHours == "0" {
			remoteHours = ""
		}
		localHours := ""
		if hasAssign {
			localHours = existingAssign.CompletedHours
		}
		if hoursOK && remoteHours != localHours {
			diff.Hours = append(diff.Hours, attendanceHoursItem{FactorialAccessMembershipID: *acc.ID, Hours: remoteHours})
		}
		if remoteAttendance.Status == nil {
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

// courseSeed decide il seed del corso: nil quando il locale e' gia' attivo
// (l'attivazione governa il sync: da li' in poi non lo tocca mai piu'), o
// quando tutti i campi seminati
// sono gia' allineati col remoto (idempotenza, anche a corso inattivo).
// Campi seminati: titolo, descrizione, fornitore abituale (dal testo
// external_provider, spazzatura "null" scartata), erogazione (dal flag
// external) e tag (dalle categorie, esclusa «Formazione interna»). Titolo
// assente -> fallback "Training Factorial <id>" e warning (TitleMissing).
// Il secondo valore elenca i category id senza nome (warning del chiamante).
func courseSeed(trainingID string, training factorial.TrainingsTraining, course *localCourse, categories map[string]string) (*courseSeedItem, []string, []string) {
	providerKind := "internal"
	if training.External != nil && *training.External {
		providerKind = "external"
	}
	vendorName := strings.TrimSpace(deref(training.ExternalProvider))
	if strings.EqualFold(vendorName, "null") {
		vendorName = ""
	}
	var tags []string
	var unresolved []string
	for _, categoryID := range training.CategoryIDs {
		name := categories[categoryID]
		if name == "" {
			unresolved = append(unresolved, categoryID)
			continue
		}
		if strings.EqualFold(name, "Formazione interna") {
			continue // erogazione: ha il suo campo dedicato, il flag comanda
		}
		tags = append(tags, name)
	}
	var areas []skillAreaSeed
	var unresolvedCompetencies []string
	for _, competencyID := range training.CompetencyIDs {
		name := factorialCompetencyNames[competencyID]
		if name == "" {
			unresolvedCompetencies = append(unresolvedCompetencies, competencyID)
			continue
		}
		areas = append(areas, skillAreaSeed{Code: skillAreaCode(name), Name: name})
	}
	sort.Slice(areas, func(i, j int) bool { return areas[i].Code < areas[j].Code })
	if course != nil && course.IsActive {
		return nil, unresolved, unresolvedCompetencies
	}
	title := strings.TrimSpace(deref(training.Name))
	missing := title == ""
	if missing {
		title = "Training Factorial " + trainingID
	}
	description := deref(training.Description)
	areaCodes := make([]string, len(areas))
	for i, area := range areas {
		areaCodes[i] = area.Code
	}
	if course != nil && course.Title == title && course.Description == description &&
		strings.EqualFold(course.VendorName, vendorName) && course.ProviderKind == providerKind &&
		strings.Join(course.Tags, "\x1f") == strings.Join(tags, "\x1f") &&
		strings.Join(course.SkillAreaCodes, "\x1f") == strings.Join(areaCodes, "\x1f") {
		return nil, unresolved, unresolvedCompetencies // gia' allineato: nessuna mutazione (idempotenza)
	}
	item := courseSeedItem{
		FactorialTrainingID: trainingID, Title: title, Description: description, TitleMissing: missing,
		VendorName: vendorName, ProviderKind: providerKind, Tags: tags, SkillAreas: areas,
	}
	if course == nil {
		item.Create = true
	} else {
		item.CourseID = course.ID
	}
	return &item, unresolved, unresolvedCompetencies
}

// classNotes compone il testo per le note dell'evento dal nome (sempre
// presente sui dati reali) e dall'eventuale descrizione della classe
// Factorial: e' l'unico posto dove l'edizione resta leggibile.
func classNotes(class factorial.TrainingsTrainingClass) string {
	name := strings.TrimSpace(deref(class.Name))
	description := strings.TrimSpace(deref(class.Description))
	switch {
	case name == "":
		return description
	case description == "":
		return name
	default:
		return name + "\n\n" + description
	}
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
		diff := computeTrainingDiff(training, sub, local, graph.Categories)
		result.Conflicts = append(result.Conflicts, diff.Conflicts...)
		result.Warnings = append(result.Warnings, diff.Warnings...)
		// Bookkeeping in-run (#172): l'embrione adottato non e' piu' candidato
		// per un altro Training omonimo dello stesso run (bulk letto una volta
		// sola). Vale anche in dry-run: il piano deve restare coerente con
		// quello che farebbe la run vera.
		if diff.Course != nil && diff.Course.Adopt {
			key := strings.ToLower(diff.Course.Title)
			ids := local.UnlinkedCourses[key]
			for i, id := range ids {
				if id == diff.Course.CourseID {
					local.UnlinkedCourses[key] = append(ids[:i:i], ids[i+1:]...)
					break
				}
			}
		}
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
INSERT INTO training.training_event (course_id, title, origin, factorial_class_id, notes)
VALUES ($1::uuid, (SELECT c.title FROM training.course c WHERE c.id = $1::uuid), 'factorial_import', $2, NULLIF($3, ''))
RETURNING id::text`, courseID, item.FactorialClassID, item.Notes).Scan(&id); err != nil {
			return nil, fmt.Errorf("create training event from factorial sync: %w", err)
		}
		if err := copyCourseTrainers(ctx, tx, id, courseID); err != nil {
			return nil, err
		}
		if err := s.auditFields(ctx, tx, principal, "training_event", id, actionFactorialImport, []string{"course_id", "title", "factorial_class_id", "notes"}); err != nil {
			return nil, err
		}
		eventID[item.FactorialClassID] = id
	}
	// Note di eventi gia' esistenti: solo se ancora vuote (guardia ripetuta
	// anche nella UPDATE: mai sopra testo scritto da una persona).
	for _, item := range diff.EventNotes {
		res, err := tx.ExecContext(ctx, `
UPDATE training.training_event SET notes = $2, updated_at = now()
WHERE id = $1::uuid AND COALESCE(notes, '') = ''`, item.EventID, item.Notes)
		if err != nil {
			return nil, fmt.Errorf("fill training event notes from factorial sync: %w", err)
		}
		if changed, _ := res.RowsAffected(); changed == 0 {
			continue
		}
		if err := s.auditFields(ctx, tx, principal, "training_event", item.EventID, actionFactorialImport, []string{"notes"}); err != nil {
			return nil, err
		}
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
INSERT INTO training.training_session (event_id, schedule_type, starts_at, ends_at, due_at, factorial_session_id, factorial_synced_state,
  topic, modality, duration_hours, location)
VALUES ($1::uuid, NULLIF($2,''), NULLIF($3,'')::timestamptz, NULLIF($4,'')::timestamptz, NULLIF($5,'')::timestamptz, $6, $7::jsonb,
  NULLIF($8,''), NULLIF($9,''), NULLIF($10,'')::numeric, NULLIF($11,''))
RETURNING id::text`, ev, item.Remote.ScheduleType, item.Remote.StartsAt, item.Remote.EndsAt, dueAt, item.FactorialSessionID, checkpoint,
				item.Op.Topic, item.Op.Modality, item.Op.DurationHours, item.Op.Location).Scan(&id); err != nil {
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
    due_at = NULLIF($5,'')::timestamptz, factorial_synced_state = $6::jsonb,
    topic = NULLIF($7,''), modality = NULLIF($8,''), duration_hours = NULLIF($9,'')::numeric, location = NULLIF($10,''), updated_at = now()
WHERE id = $1::uuid`, item.SessionID, item.Remote.ScheduleType, item.Remote.StartsAt, item.Remote.EndsAt, dueAt, checkpoint,
				item.Op.Topic, item.Op.Modality, item.Op.DurationHours, item.Op.Location); err != nil {
				return nil, fmt.Errorf("update training session from factorial sync: %w", err)
			}
			if err := s.auditFields(ctx, tx, principal, "training_session", item.SessionID, actionFactorialImport, []string{"schedule_type", "starts_at", "ends_at", "due_at", "topic", "modality", "duration_hours", "location"}); err != nil {
				return nil, err
			}
			sessionID[item.FactorialSessionID] = item.SessionID
		}
	}
	// Riallineamento dei soli dati operativi: sessioni gia' in sync su
	// schedule/date il cui argomento/modalita'/durata/luogo remoto e' cambiato.
	for _, item := range diff.SessionOps {
		if _, err := tx.ExecContext(ctx, `
UPDATE training.training_session
SET topic = NULLIF($2,''), modality = NULLIF($3,''), duration_hours = NULLIF($4,'')::numeric, location = NULLIF($5,''), updated_at = now()
WHERE id = $1::uuid`, item.SessionID, item.Op.Topic, item.Op.Modality, item.Op.DurationHours, item.Op.Location); err != nil {
			return nil, fmt.Errorf("update training session operational fields from factorial sync: %w", err)
		}
		if err := s.auditFields(ctx, tx, principal, "training_session", item.SessionID, actionFactorialImport, []string{"topic", "modality", "duration_hours", "location"}); err != nil {
			return nil, err
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
	for _, item := range diff.Hours {
		key, ok := assignKey[item.FactorialAccessMembershipID]
		if !ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE training.enrollment_session SET completed_hours = NULLIF($3,'')::numeric, updated_at = now()
WHERE enrollment_id = $1::uuid AND session_id = $2::uuid`, key[0], key[1], item.Hours); err != nil {
			return nil, fmt.Errorf("update training attendance completed hours from factorial sync: %w", err)
		}
		if err := s.auditFields(ctx, tx, principal, "enrollment_session", key[0], actionFactorialImport, []string{"completed_hours"}); err != nil {
			return nil, err
		}
	}
	if err := s.applyCompletionDates(ctx, tx, principal, diff.Memberships, eventID); err != nil {
		return nil, err
	}
	return rdaIssues, nil
}

// applyCompletionDates valorizza enrollment.actual_end (fine effettiva) per
// le iscrizioni completate che ne sono prive, in cascata:
//  1. data di completamento della membership Factorial (solo se anche lo
//     stato remoto dice completed: sulle incoerenze data-senza-completamento
//     lo stato comanda);
//  2. altrimenti la data dell'ultima sessione completata assegnata alla
//     persona.
//
// Sempre e solo se actual_end e' NULL (una data messa a mano non si tocca) e
// se i fatti locali dicono completato. Idempotente per costruzione: dopo il
// primo riempimento le righe non matchano piu'.
func (s *SQLStore) applyCompletionDates(ctx context.Context, tx *sql.Tx, principal Principal, memberships []factorial.TrainingsTrainingMembership, eventID map[string]string) error {
	type memberDate struct {
		MembershipID string `json:"mid"`
		Date         string `json:"d"`
	}
	var explicit []memberDate
	for _, m := range memberships {
		if m.ID == nil || m.Status == nil || *m.Status != factorial.TrainingsTrainingMembershipStatusCompleted || m.TrainingCompletedAt == nil {
			continue
		}
		if d := formatDateUTC(&m.TrainingCompletedAt.Time); d != "" {
			explicit = append(explicit, memberDate{MembershipID: *m.ID, Date: d})
		}
	}
	audit := func(rows *sql.Rows) error {
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("scan training completion date enrollment: %w", err)
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range ids {
			if err := s.auditFields(ctx, tx, principal, "enrollment", id, actionFactorialImport, []string{"actual_end"}); err != nil {
				return err
			}
		}
		return nil
	}
	if len(explicit) > 0 {
		payload, err := json.Marshal(explicit)
		if err != nil {
			return fmt.Errorf("encode training completion dates: %w", err)
		}
		rows, err := tx.QueryContext(ctx, `
UPDATE training.enrollment e SET actual_end = v.d, updated_at = now()
FROM jsonb_to_recordset($1::jsonb) AS v(mid text, d date)
WHERE e.factorial_training_membership_id = v.mid
  AND e.actual_end IS NULL AND e.delivery_status = 'completed'
RETURNING e.id::text`, string(payload))
		if err != nil {
			return fmt.Errorf("apply training membership completion dates: %w", err)
		}
		if err := audit(rows); err != nil {
			return err
		}
	}
	if len(eventID) == 0 {
		return nil
	}
	events := make([]string, 0, len(eventID))
	for _, id := range eventID {
		events = append(events, id)
	}
	eventsJSON, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("encode training completion events: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `
UPDATE training.enrollment e SET actual_end = d.max_date, updated_at = now()
FROM (
  SELECT es.enrollment_id, (MAX(COALESCE(s.ends_at, s.starts_at)) AT TIME ZONE 'UTC')::date AS max_date
  FROM training.enrollment_session es
  JOIN training.training_session s ON s.id = es.session_id
  WHERE es.participation_status = 'completed'
  GROUP BY es.enrollment_id
) d
WHERE e.id = d.enrollment_id AND d.max_date IS NOT NULL
  AND e.actual_end IS NULL AND e.delivery_status = 'completed'
  AND e.event_id IN (SELECT jsonb_array_elements_text($1::jsonb)::uuid)
RETURNING e.id::text`, string(eventsJSON))
	if err != nil {
		return fmt.Errorf("apply training session completion dates: %w", err)
	}
	return audit(rows)
}

// applyCourseSeed crea o riaggiorna (solo se ancora inattivo) il corso
// seed di un Training, e ritorna il suo id. Il fornitore abituale viene
// risolto in anagrafica per nome (get-or-create, confronto senza distinzione
// di maiuscole via citext); i tag arrivano come array JSON.
func (s *SQLStore) applyCourseSeed(ctx context.Context, tx *sql.Tx, principal Principal, item courseSeedItem) (string, error) {
	vendorID := ""
	if item.VendorName != "" {
		var created bool
		if err := tx.QueryRowContext(ctx, `
WITH ins AS (
  INSERT INTO training.vendor (name, name_normalized) VALUES ($1::text, $1::text::citext)
  ON CONFLICT (name_normalized) DO NOTHING
  RETURNING id::text
)
SELECT COALESCE((SELECT id FROM ins), (SELECT id::text FROM training.vendor WHERE name_normalized = $1::text::citext)),
       EXISTS (SELECT 1 FROM ins)`, item.VendorName).Scan(&vendorID, &created); err != nil {
			return "", fmt.Errorf("resolve training vendor from factorial sync: %w", err)
		}
		if created {
			if err := s.auditFields(ctx, tx, principal, "vendor", vendorID, actionFactorialImport, []string{"name"}); err != nil {
				return "", err
			}
		}
	}
	tags := item.Tags
	if tags == nil {
		tags = []string{} // serializza [] e mai null: la UPDATE estrae elementi jsonb
	}
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return "", fmt.Errorf("encode training course tags: %w", err)
	}
	courseID := item.CourseID
	if item.Create {
		if err := tx.QueryRowContext(ctx, `
INSERT INTO training.course (title, description, is_active, factorial_training_id, vendor_id, provider_kind, tags)
VALUES ($1, NULLIF($2, ''), false, $3, NULLIF($4, '')::uuid, $5::training.course_provider_kind,
  COALESCE((SELECT array_agg(t.x) FROM jsonb_array_elements_text($6::jsonb) AS t(x)), '{}'))
RETURNING id::text`, item.Title, item.Description, item.FactorialTrainingID, vendorID, item.ProviderKind, string(tagsJSON)).Scan(&courseID); err != nil {
			return "", fmt.Errorf("create training course from factorial sync: %w", err)
		}
		if err := s.auditFields(ctx, tx, principal, "course", courseID, actionFactorialImport, []string{"title", "description", "factorial_training_id", "vendor_id", "provider_kind", "tags"}); err != nil {
			return "", err
		}
	} else {
		// Ramo adozione (#172): primo aggancio di un corso senza correlatore,
		// la UPDATE porta anche il correlatore (audit come nel ramo create).
		// Nessun factorial_training_id precedentemente diverso puo' esistere:
		// i candidati sono senza correlatore per costruzione (bulk snapshot +
		// advisory lock + unique index a rete di sicurezza).
		auditFieldsList := []string{"title", "description", "vendor_id", "provider_kind", "tags"}
		if item.Adopt {
			auditFieldsList = append([]string{"factorial_training_id"}, auditFieldsList...)
			_, err = tx.ExecContext(ctx, `
UPDATE training.course SET factorial_training_id = $7, title = $2, description = NULLIF($3, ''), vendor_id = NULLIF($4, '')::uuid,
  provider_kind = $5::training.course_provider_kind,
  tags = COALESCE((SELECT array_agg(t.x) FROM jsonb_array_elements_text($6::jsonb) AS t(x)), '{}'), updated_at = now()
WHERE id = $1::uuid`,
				item.CourseID, item.Title, item.Description, vendorID, item.ProviderKind, string(tagsJSON), item.FactorialTrainingID)
		} else {
			_, err = tx.ExecContext(ctx, `
UPDATE training.course SET title = $2, description = NULLIF($3, ''), vendor_id = NULLIF($4, '')::uuid,
  provider_kind = $5::training.course_provider_kind,
  tags = COALESCE((SELECT array_agg(t.x) FROM jsonb_array_elements_text($6::jsonb) AS t(x)), '{}'), updated_at = now()
WHERE id = $1::uuid`,
				item.CourseID, item.Title, item.Description, vendorID, item.ProviderKind, string(tagsJSON))
		}
		if err != nil {
			return "", fmt.Errorf("reseed training course from factorial sync: %w", err)
		}
		if err := s.auditFields(ctx, tx, principal, "course", item.CourseID, actionFactorialImport, auditFieldsList); err != nil {
			return "", err
		}
	}
	areaIDs := make([]string, 0, len(item.SkillAreas))
	for _, area := range item.SkillAreas {
		var areaID string
		var created bool
		if err := tx.QueryRowContext(ctx, `
WITH ins AS (
  INSERT INTO training.skill_area (code, name) VALUES ($1::text, $2::text)
  ON CONFLICT (code) DO NOTHING
  RETURNING id::text
)
SELECT COALESCE((SELECT id FROM ins), (SELECT id::text FROM training.skill_area WHERE code = $1::text)),
       EXISTS (SELECT 1 FROM ins)`, area.Code, area.Name).Scan(&areaID, &created); err != nil {
			return "", fmt.Errorf("resolve training skill area from factorial sync: %w", err)
		}
		if created {
			if err := s.auditFields(ctx, tx, principal, "skill_area", areaID, actionFactorialImport, []string{"code", "name"}); err != nil {
				return "", err
			}
		}
		areaIDs = append(areaIDs, areaID)
	}
	if err := replaceSkillAreaLinks(ctx, tx, "course_skill_area", "course_id", courseID, areaIDs); err != nil {
		return "", fmt.Errorf("relink training course skill areas from factorial sync: %w", err)
	}
	return courseID, nil
}
