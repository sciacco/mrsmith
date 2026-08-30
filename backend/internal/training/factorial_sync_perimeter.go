package training

import (
	"github.com/sciacco/mrsmith/internal/platform/directory"
	"github.com/sciacco/mrsmith/pkg/factorial"
)

// Regole pure di perimetro sugli attivi Factorial (#141, slice 3/8 = #145):
// l'insieme degli employee attivi dallo snapshot anagrafico, la selezione
// dei Training da importare/aggiornare, la conservazione delle classi vuote
// e il filtro delle relazioni ai soli employee attivi. Zero IO: operano solo
// sul grafo gia' letto da fetchFactorialTrainingGraph e sullo snapshot
// directory.

// activeEmployeeIDs estrae le Person.ExternalID attive dallo snapshot
// anagrafico. Mai derivare gli attivi da training.employee.status:
// directory_exempt puo' mantenere localmente attiva una persona esterna
// cessata.
func activeEmployeeIDs(snapshot directory.Snapshot) map[string]struct{} {
	active := make(map[string]struct{}, len(snapshot.People))
	for _, p := range snapshot.People {
		if p.Active {
			active[p.ExternalID] = struct{}{}
		}
	}
	return active
}

// selectTrainingIDs individua i Training Factorial che entrano nel
// perimetro. Un Training gia' collegato localmente (linkedTrainingIDs)
// resta sempre nel perimetro; un Training nuovo entra solo se ha almeno una
// training membership o un access (session access membership) di employee
// attivo.
func selectTrainingIDs(graph factorialTrainingGraph, activeIDs, linkedTrainingIDs map[string]struct{}) map[string]struct{} {
	sessionTrainingID := make(map[string]string, len(graph.Sessions))
	for _, s := range graph.Sessions {
		if s.ID != nil && s.TrainingID != nil {
			sessionTrainingID[*s.ID] = *s.TrainingID
		}
	}
	hasActiveEngagement := make(map[string]bool)
	for _, m := range graph.TrainingMemberships {
		if m.TrainingID == nil || m.EmployeeID == nil {
			continue
		}
		if _, ok := activeIDs[*m.EmployeeID]; ok {
			hasActiveEngagement[*m.TrainingID] = true
		}
	}
	for _, a := range graph.SessionAccessMemberships {
		if a.SessionID == nil || a.EmployeeID == nil {
			continue
		}
		if _, ok := activeIDs[*a.EmployeeID]; !ok {
			continue
		}
		if trainingID, ok := sessionTrainingID[*a.SessionID]; ok {
			hasActiveEngagement[trainingID] = true
		}
	}
	selected := make(map[string]struct{})
	for _, t := range graph.Trainings {
		if t.ID == nil {
			continue
		}
		_, linked := linkedTrainingIDs[*t.ID]
		if linked || hasActiveEngagement[*t.ID] {
			selected[*t.ID] = struct{}{}
		}
	}
	return selected
}

// trainingClassPerimeter e' il sotto-grafo perimetrato delle classi di un
// singolo Training selezionato: le classi tenute (con tutte le loro
// sessioni) e le relazioni (training membership, access, attendance)
// filtrate ai soli employee attivi.
type trainingClassPerimeter struct {
	Classes             []factorial.TrainingsTrainingClass
	Sessions            []factorial.TrainingsSession
	TrainingMemberships []factorial.TrainingsTrainingMembership
	Access              []factorial.TrainingsSessionAccessMembership
	Attendances         []factorial.TrainingsSessionAttendance
}

// selectTrainingClasses calcola, per un Training selezionato, le classi
// tenute e le relazioni filtrate ai soli employee attivi. Le training
// membership del Training sono filtrate ai soli employee attivi
// indipendentemente dalla classe: non hanno granularita' di classe.
func selectTrainingClasses(trainingID string, graph factorialTrainingGraph, activeIDs map[string]struct{}) trainingClassPerimeter {
	var result trainingClassPerimeter
	for _, m := range graph.TrainingMemberships {
		if m.TrainingID == nil || *m.TrainingID != trainingID || m.EmployeeID == nil {
			continue
		}
		if _, ok := activeIDs[*m.EmployeeID]; ok {
			result.TrainingMemberships = append(result.TrainingMemberships, m)
		}
	}
	for _, c := range graph.TrainingClasses {
		if c.TrainingID == nil || *c.TrainingID != trainingID {
			continue
		}
		kept, sessions, access, attendances := selectClassSessions(c, graph, activeIDs)
		if !kept {
			continue
		}
		result.Classes = append(result.Classes, c)
		result.Sessions = append(result.Sessions, sessions...)
		result.Access = append(result.Access, access...)
		result.Attendances = append(result.Attendances, attendances...)
	}
	return result
}

// selectClassSessions decide se una classe resta nel perimetro e, se
// tenuta, restituisce tutte le sue sessioni (solo quelle con TrainingClassID
// uguale alla classe: le sessioni remote senza classe sono escluse a monte,
// vedi la nota sul filtro qui sotto) e le relazioni (access, attendance)
// filtrate ai soli employee attivi. Si tengono le classi senza sessioni e
// quelle con roster vuoto; si scarta una classe soltanto quando tutte le
// assegnazioni delle sue sessioni appartengono a employee inattivi.
func selectClassSessions(class factorial.TrainingsTrainingClass, graph factorialTrainingGraph, activeIDs map[string]struct{}) (kept bool, sessions []factorial.TrainingsSession, access []factorial.TrainingsSessionAccessMembership, attendances []factorial.TrainingsSessionAttendance) {
	if class.ID == nil {
		return false, nil, nil, nil
	}
	sessionIDs := make(map[string]struct{})
	for _, s := range graph.Sessions {
		// Le sessioni remote con TrainingClassID nil (non legate ad alcuna
		// classe) restano sempre fuori dal grafo perimetrato: gap noto e
		// voluto di questa slice (#145), che discute solo le sessioni di
		// una classe. Asimmetria intenzionale con selectTrainingIDs: un
		// access su una di queste sessioni conta comunque per la selezione
		// del Training (via s.TrainingID, non tramite questo filtro), ma
		// la sessione stessa e le sue relazioni non compaiono in nessun
		// output di questo file.
		if s.TrainingClassID == nil || *s.TrainingClassID != *class.ID {
			continue
		}
		sessions = append(sessions, s)
		if s.ID != nil {
			sessionIDs[*s.ID] = struct{}{}
		}
	}
	var roster []factorial.TrainingsSessionAccessMembership
	for _, a := range graph.SessionAccessMemberships {
		if a.SessionID == nil {
			continue
		}
		if _, ok := sessionIDs[*a.SessionID]; ok {
			roster = append(roster, a)
		}
	}
	if len(roster) > 0 {
		anyActive := false
		for _, a := range roster {
			if a.EmployeeID == nil {
				continue
			}
			if _, ok := activeIDs[*a.EmployeeID]; ok {
				anyActive = true
				break
			}
		}
		if !anyActive {
			return false, nil, nil, nil
		}
	}
	activeAccessIDs := make(map[string]struct{})
	for _, a := range roster {
		if a.EmployeeID == nil {
			continue
		}
		if _, ok := activeIDs[*a.EmployeeID]; !ok {
			continue
		}
		access = append(access, a)
		if a.ID != nil {
			activeAccessIDs[*a.ID] = struct{}{}
		}
	}
	for _, att := range graph.SessionAttendances {
		if att.SessionAccessMembershipID == nil {
			continue
		}
		if _, ok := activeAccessIDs[*att.SessionAccessMembershipID]; ok {
			attendances = append(attendances, att)
		}
	}
	return true, sessions, access, attendances
}

// computeFactorialPerimeter compone il grafo perimetrato consumato dalla
// slice 4: i Training selezionati, le classi tenute con le rispettive
// sessioni (solo quelle legate a una classe: vedi il gap noto e voluto sulle
// sessioni senza classe documentato in selectClassSessions), e training
// membership/access/attendance filtrati ai soli employee attivi.
func computeFactorialPerimeter(graph factorialTrainingGraph, activeIDs, linkedTrainingIDs map[string]struct{}) factorialTrainingGraph {
	selectedTrainingIDs := selectTrainingIDs(graph, activeIDs, linkedTrainingIDs)
	var perimeter factorialTrainingGraph
	perimeter.Categories = graph.Categories // mappa id->nome, non perimetrabile
	for _, t := range graph.Trainings {
		if t.ID == nil {
			continue
		}
		if _, ok := selectedTrainingIDs[*t.ID]; !ok {
			continue
		}
		perimeter.Trainings = append(perimeter.Trainings, t)
		classes := selectTrainingClasses(*t.ID, graph, activeIDs)
		perimeter.TrainingClasses = append(perimeter.TrainingClasses, classes.Classes...)
		perimeter.Sessions = append(perimeter.Sessions, classes.Sessions...)
		perimeter.TrainingMemberships = append(perimeter.TrainingMemberships, classes.TrainingMemberships...)
		perimeter.SessionAccessMemberships = append(perimeter.SessionAccessMemberships, classes.Access...)
		perimeter.SessionAttendances = append(perimeter.SessionAttendances, classes.Attendances...)
	}
	return perimeter
}
