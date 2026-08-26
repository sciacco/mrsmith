package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Letture pure del dominio locale per il ramo inbound (#141, slice 4/8 =
// #146): solo SELECT. Bulk a livello di run (non N+1 per classe/sessione/
// employee, come directory_sync.go): loadLocalSyncState legge in poche
// query cio' che il sync ha gia' collegato (factorial_*_id valorizzato) e
// l'intero roster employee, raggruppati in mappe per tutta la run.

type localCourse struct {
	ID          string
	IsActive    bool
	Title       string
	Description string
}

type localEvent struct {
	ID        string
	Cancelled bool
}

// localSession e' la proiezione locale di una sessione gia' collegata
// (trovata per factorial_session_id): il valore corrente confrontabile
// (Local), l'ultimo checkpoint noto (Checkpoint, nil se mai sincronizzata) e
// la provenienza (prima action di audit per l'entita': "factorial_import" =
// nata dal sync, altrimenti gesto locale) — significativa solo quando il
// checkpoint manca.
type localSession struct {
	ID         string
	Local      sessionSyncState
	Checkpoint *sessionSyncState
	Provenance string
}

type localEnrollment struct {
	ID        string
	Cancelled bool
}

// localAssign e' la proiezione locale di una riga enrollment_session gia'
// presente, indicizzata in localTrainingState.Assigns dalla coppia
// (enrollment_id, session_id).
type localAssign struct {
	Status                      string
	Checkpoint                  *string
	FactorialAccessMembershipID string
	FactorialAttendanceID       string
}

// localTrainingState e' lo stato locale gia' collegato, per l'intera run:
// tutte le entita' con un factorial_*_id valorizzato, piu' l'intero roster
// employee. Le mappe usano le chiavi naturali del matching (id Factorial, o
// coppie di id locali): il diff puro di un singolo Training legge solo le
// chiavi del proprio sotto-grafo, quindi contenere anche entita' di altri
// Training e' innocuo.
type localTrainingState struct {
	Courses     map[string]localCourse        // factorial_training_id -> corso
	Events      map[string]localEvent         // factorial_class_id -> evento
	Sessions    map[string]localSession       // factorial_session_id -> sessione
	Employees   map[string]string             // id Factorial employee -> id locale
	Enrollments map[[2]string]localEnrollment // [employeeID, eventID] -> iscrizione
	Assigns     map[[2]string]localAssign     // [enrollmentID, sessionID] -> riga
}

// loadLocalSyncState legge in blocco, per l'intera run, tutto lo stato
// locale gia' collegato a Factorial: corsi, eventi (con le loro iscrizioni),
// sessioni (con provenienza e assegnazioni) e il roster employee. Query
// complessive indipendenti dal numero di Training/classi/sessioni nel
// grafo: nessun N+1 per classe/sessione/employee.
func (s *SQLStore) loadLocalSyncState(ctx context.Context, q sqlRunner) (localTrainingState, error) {
	courses, err := s.bulkCourses(ctx, q)
	if err != nil {
		return localTrainingState{}, err
	}
	events, enrollments, err := s.bulkEventsAndEnrollments(ctx, q)
	if err != nil {
		return localTrainingState{}, err
	}
	sessions, assigns, err := s.bulkSessionsAndAssigns(ctx, q)
	if err != nil {
		return localTrainingState{}, err
	}
	employees, err := s.bulkEmployees(ctx, q)
	if err != nil {
		return localTrainingState{}, err
	}
	return localTrainingState{
		Courses: courses, Events: events, Sessions: sessions,
		Employees: employees, Enrollments: enrollments, Assigns: assigns,
	}, nil
}

func (s *SQLStore) bulkCourses(ctx context.Context, q sqlRunner) (map[string]localCourse, error) {
	rows, err := q.QueryContext(ctx, `
SELECT factorial_training_id, id::text, is_active, title, COALESCE(description, '')
FROM training.course WHERE factorial_training_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("list local training courses: %w", err)
	}
	defer rows.Close()
	result := map[string]localCourse{}
	for rows.Next() {
		var trainingID string
		var c localCourse
		if err := rows.Scan(&trainingID, &c.ID, &c.IsActive, &c.Title, &c.Description); err != nil {
			return nil, fmt.Errorf("scan local training course: %w", err)
		}
		result[trainingID] = c
	}
	return result, rows.Err()
}

// enrollmentJSON e' la forma della riga di iscrizione aggregata in JSON
// dalla sotto-query di bulkEventsAndEnrollments (stesso pattern di
// directory_sync.go per le team_membership).
type enrollmentJSON struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employeeId"`
	Cancelled  bool   `json:"cancelled"`
}

func (s *SQLStore) bulkEventsAndEnrollments(ctx context.Context, q sqlRunner) (map[string]localEvent, map[[2]string]localEnrollment, error) {
	rows, err := q.QueryContext(ctx, `
SELECT ev.factorial_class_id, ev.id::text, ev.cancelled_at IS NOT NULL,
  COALESCE((SELECT json_agg(json_build_object('id', en.id::text, 'employeeId', en.employee_id::text, 'cancelled', en.delivery_status = 'cancelled'))
    FROM training.enrollment en WHERE en.event_id = ev.id), '[]')::text
FROM training.training_event ev WHERE ev.factorial_class_id IS NOT NULL`)
	if err != nil {
		return nil, nil, fmt.Errorf("list local training events: %w", err)
	}
	defer rows.Close()
	events := map[string]localEvent{}
	enrollments := map[[2]string]localEnrollment{}
	for rows.Next() {
		var classID, enrollmentsRaw string
		var e localEvent
		if err := rows.Scan(&classID, &e.ID, &e.Cancelled, &enrollmentsRaw); err != nil {
			return nil, nil, fmt.Errorf("scan local training event: %w", err)
		}
		events[classID] = e
		var parsed []enrollmentJSON
		if err := json.Unmarshal([]byte(enrollmentsRaw), &parsed); err != nil {
			return nil, nil, fmt.Errorf("decode local training enrollments: %w", err)
		}
		for _, en := range parsed {
			enrollments[[2]string{en.EmployeeID, e.ID}] = localEnrollment{ID: en.ID, Cancelled: en.Cancelled}
		}
	}
	return events, enrollments, rows.Err()
}

// assignJSON e' la forma della riga enrollment_session aggregata in JSON
// dalla sotto-query di bulkSessionsAndAssigns. Checkpoint resta *string
// (non COALESCE) per non confondere "mai sincronizzata" con "".
type assignJSON struct {
	EnrollmentID string  `json:"enrollmentId"`
	Status       string  `json:"status"`
	Checkpoint   *string `json:"checkpoint"`
	AccessID     string  `json:"accessId"`
	AttendanceID string  `json:"attendanceId"`
}

func (s *SQLStore) bulkSessionsAndAssigns(ctx context.Context, q sqlRunner) (map[string]localSession, map[[2]string]localAssign, error) {
	rows, err := q.QueryContext(ctx, `
SELECT s.factorial_session_id, s.id::text, s.schedule_type, s.starts_at, s.ends_at, s.due_at, s.factorial_synced_state, COALESCE(fa.action, ''),
  COALESCE((SELECT json_agg(json_build_object('enrollmentId', es.enrollment_id::text, 'status', es.participation_status,
    'checkpoint', es.factorial_synced_status, 'accessId', COALESCE(es.factorial_access_membership_id, ''), 'attendanceId', COALESCE(es.factorial_attendance_id, '')))
    FROM training.enrollment_session es WHERE es.session_id = s.id), '[]')::text
FROM training.training_session s
LEFT JOIN LATERAL (SELECT action FROM training.audit_log WHERE entity_type = 'training_session' AND entity_id = s.id ORDER BY occurred_at ASC, id ASC LIMIT 1) fa ON true
WHERE s.factorial_session_id IS NOT NULL`)
	if err != nil {
		return nil, nil, fmt.Errorf("list local training sessions: %w", err)
	}
	defer rows.Close()
	sessions := map[string]localSession{}
	assigns := map[[2]string]localAssign{}
	for rows.Next() {
		var (
			factorialSessionID, assignsRaw string
			session                        localSession
			scheduleType                   sql.NullString
			startsAt, endsAt, dueAt        sql.NullTime
			checkpointRaw                  []byte
		)
		if err := rows.Scan(&factorialSessionID, &session.ID, &scheduleType, &startsAt, &endsAt, &dueAt, &checkpointRaw, &session.Provenance, &assignsRaw); err != nil {
			return nil, nil, fmt.Errorf("scan local training session: %w", err)
		}
		session.Local = newSessionSyncStateFromLocal(nullStringPtr(scheduleType), nullTimePtr(startsAt), nullTimePtr(endsAt), nullTimePtr(dueAt))
		if len(checkpointRaw) > 0 {
			var checkpoint sessionSyncState
			if err := json.Unmarshal(checkpointRaw, &checkpoint); err != nil {
				return nil, nil, fmt.Errorf("decode local training session checkpoint: %w", err)
			}
			session.Checkpoint = &checkpoint
		}
		sessions[factorialSessionID] = session
		var parsed []assignJSON
		if err := json.Unmarshal([]byte(assignsRaw), &parsed); err != nil {
			return nil, nil, fmt.Errorf("decode local training enrollment sessions: %w", err)
		}
		for _, a := range parsed {
			assigns[[2]string{a.EnrollmentID, session.ID}] = localAssign{
				Status: a.Status, Checkpoint: a.Checkpoint,
				FactorialAccessMembershipID: a.AccessID, FactorialAttendanceID: a.AttendanceID,
			}
		}
	}
	return sessions, assigns, rows.Err()
}

func (s *SQLStore) bulkEmployees(ctx context.Context, q sqlRunner) (map[string]string, error) {
	rows, err := q.QueryContext(ctx, `
SELECT external_id, id::text FROM training.employee WHERE external_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("list local training employees: %w", err)
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var externalID, id string
		if err := rows.Scan(&externalID, &id); err != nil {
			return nil, fmt.Errorf("scan local training employee: %w", err)
		}
		result[externalID] = id
	}
	return result, rows.Err()
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func nullTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}
