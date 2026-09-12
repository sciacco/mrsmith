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

// loadPlanningSnapshot reads the complete local planning graph under one
// repeatable-read, read-only transaction. It intentionally has no LIMIT and
// performs no remote calls. courseID is an internal detail scope only.
func (s *SQLStore) loadPlanningSnapshot(ctx context.Context, courseID string) (PlanningSnapshot, error) {
	if s == nil || s.db == nil {
		return PlanningSnapshot{}, errors.New("training database not configured")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return PlanningSnapshot{}, fmt.Errorf("begin planning snapshot: %w", err)
	}
	defer tx.Rollback()
	snap := PlanningSnapshot{Courses: []planningCourse{}, Requests: []planningRequest{}, Events: []planningEvent{}, Enrollments: []planningEnrollment{}, Expenses: []planningExpense{}, ExpenseEnrollments: []planningExpenseEnrollment{}, CurrentTeams: map[string][]PlanningRef{}}
	scope := ""
	args := []any{}
	if courseID != "" {
		scope = " WHERE c.id = $1::uuid"
		args = []any{courseID}
	}
	rows, err := tx.QueryContext(ctx, `SELECT c.id::text,c.title,COALESCE(to_json(c.tags)::text,'[]'),c.suspended_at IS NOT NULL,NULLIF(c.suspension_reason,''),NULLIF(c.reminder_text,''),c.reminder_at::text FROM training.course c`+scope, args...)
	if err != nil {
		return snap, fmt.Errorf("load planning courses: %w", err)
	}
	for rows.Next() {
		var c planningCourse
		var tagsRaw string
		var reason, text, date sql.NullString
		if err := rows.Scan(&c.ID, &c.Title, &tagsRaw, &c.Suspended, &reason, &text, &date); err != nil {
			rows.Close()
			return snap, err
		}
		if err := json.Unmarshal([]byte(tagsRaw), &c.Tags); err != nil {
			rows.Close()
			return snap, fmt.Errorf("decode planning course tags: %w", err)
		}
		if reason.Valid {
			c.SuspensionReason = &reason.String
		}
		if text.Valid {
			c.ReminderText = text.String
		}
		if date.Valid {
			c.ReminderDate = &date.String
		}
		snap.Courses = append(snap.Courses, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snap, err
	}
	rows.Close()
	if len(snap.Courses) == 0 && courseID != "" {
		return snap, notFoundError("course_not_found", "corso non trovato")
	}
	// Subsequent reads are all restricted through the selected course set, so a
	// detail never leaks sibling facts while still retaining explicit requests
	// linked to its enrollments from another course.
	in := ` IN (SELECT id FROM training.course)`
	var scopeArgs []any
	if courseID != "" {
		in = ` IN (SELECT id FROM training.course WHERE id=$1::uuid)`
		scopeArgs = []any{courseID}
	}
	requestScope := `((CASE WHEN r.outcome = 'accepted' THEN r.accepted_course_id ELSE r.course_id END)` + in + ` OR r.id IN (SELECT en.source_request_id FROM training.enrollment en JOIN training.training_event ev ON ev.id=en.event_id WHERE ev.course_id` + in + ` AND en.source_request_id IS NOT NULL) OR r.resulting_enrollment_id IN (SELECT en.id FROM training.enrollment en JOIN training.training_event ev ON ev.id=en.event_id WHERE ev.course_id` + in + `))`
	requestArgs := scopeArgs
	rows, err = tx.QueryContext(ctx, `SELECT DISTINCT r.id::text,COALESCE(r.course_id::text,''),COALESCE(oc.title,''),r.description,e.id::text,concat(e.last_name,' ',e.first_name),t.id::text,t.name,r.priority,r.created_at,NULLIF(r.tl_opinion,''),NULLIF(r.people_decision,''),NULLIF(r.outcome,''),r.suspended_at IS NOT NULL,NULLIF(r.reminder_text,''),r.reminder_at::text,ac.id::text,ac.title,r.accepted_event_id::text,r.resulting_enrollment_id::text FROM training.training_request r JOIN training.employee e ON e.id=r.employee_id LEFT JOIN training.team t ON t.id=r.selected_team_id LEFT JOIN training.course ac ON ac.id=r.accepted_course_id LEFT JOIN training.course oc ON oc.id=r.course_id WHERE `+requestScope, requestArgs...)
	if err != nil {
		return snap, fmt.Errorf("load planning requests: %w", err)
	}
	for rows.Next() {
		var x planningRequest
		var tl, pd, out, text, date, acid, acname, aeid, reid sql.NullString
		var teamID, teamName sql.NullString
		var pri sql.NullInt64
		var created time.Time
		if err := rows.Scan(&x.ID, &x.CourseID, &x.CourseTitle, &x.Description, &x.Employee.ID, &x.Employee.Name, &teamID, &teamName, &pri, &created, &tl, &pd, &out, &x.Suspended, &text, &date, &acid, &acname, &aeid, &reid); err != nil {
			rows.Close()
			return snap, err
		}
		x.CreatedAt = created.UTC().Format(time.RFC3339)
		if teamID.Valid {
			x.Team = &PlanningRef{ID: teamID.String, Name: teamName.String}
		}
		if pri.Valid {
			v := int(pri.Int64)
			x.Priority = &v
		}
		if tl.Valid {
			x.TLOpinion = &tl.String
		}
		if pd.Valid {
			x.PeopleDecision = &pd.String
		}
		if out.Valid {
			x.Outcome = &out.String
		}
		if text.Valid {
			x.ReminderText = text.String
		}
		if date.Valid {
			x.ReminderDate = &date.String
		}
		if acid.Valid {
			x.AcceptedCourse = &PlanningRef{ID: acid.String, Name: acname.String}
		}
		if aeid.Valid {
			x.AcceptedEventID = &aeid.String
		}
		if reid.Valid {
			x.ResultingEnrollmentID = &reid.String
		}
		snap.Requests = append(snap.Requests, x)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snap, err
	}
	rows.Close()
	rows, err = tx.QueryContext(ctx, `SELECT ev.id::text,ev.course_id::text,ev.title,ev.origin,ev.cancelled_at IS NOT NULL,cond.without_sessions,cond.unassigned_enrollments,cond.needs_reconciliation,NULLIF(ev.reminder_text,''),ev.reminder_at::text,(SELECT count(*) FROM training.training_session s WHERE s.event_id=ev.id),(SELECT min(starts_at) FROM training.training_session s WHERE s.event_id=ev.id),(SELECT max(ends_at) FROM training.training_session s WHERE s.event_id=ev.id),(SELECT max(due_at) FROM training.training_session s WHERE s.event_id=ev.id AND s.schedule_type='self_paced') FROM training.training_event ev JOIN training.v_event_operational_condition cond ON cond.event_id=ev.id WHERE ev.course_id`+in, scopeArgs...)
	if err != nil {
		return snap, fmt.Errorf("load planning events: %w", err)
	}
	for rows.Next() {
		var x planningEvent
		var text, date sql.NullString
		var start, end, due sql.NullTime
		if err := rows.Scan(&x.ID, &x.CourseID, &x.Title, &x.Origin, &x.Cancelled, &x.WithoutSessions, &x.UnassignedEnrollments, &x.NeedsReconciliation, &text, &date, &x.SessionsCount, &start, &end, &due); err != nil {
			rows.Close()
			return snap, err
		}
		if text.Valid {
			x.ReminderText = text.String
		}
		if date.Valid {
			x.ReminderDate = &date.String
		}
		if start.Valid {
			v := start.Time.UTC().Format(time.RFC3339)
			x.StartsAt = &v
		}
		if end.Valid {
			v := end.Time.UTC().Format(time.RFC3339)
			x.EndsAt = &v
		}
		if due.Valid {
			v := due.Time.UTC().Format("2006-01-02")
			x.DueOn = &v
		}
		snap.Events = append(snap.Events, x)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snap, err
	}
	rows.Close()
	rows, err = tx.QueryContext(ctx, `SELECT en.id::text,en.employee_id::text,concat(e.last_name,' ',e.first_name),en.event_id::text,en.delivery_status,NULLIF(en.source_request_id::text,'') FROM training.enrollment en JOIN training.training_event ev ON ev.id=en.event_id JOIN training.employee e ON e.id=en.employee_id WHERE ev.course_id`+in, scopeArgs...)
	if err != nil {
		return snap, fmt.Errorf("load planning enrollments: %w", err)
	}
	for rows.Next() {
		var x planningEnrollment
		var src sql.NullString
		if err := rows.Scan(&x.ID, &x.EmployeeID, &x.Employee.Name, &x.EventID, &x.DeliveryStatus, &src); err != nil {
			rows.Close()
			return snap, err
		}
		x.Employee.ID = x.EmployeeID
		if src.Valid {
			x.SourceRequestID = &src.String
		}
		snap.Enrollments = append(snap.Enrollments, x)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snap, err
	}
	rows.Close()
	rows, err = tx.QueryContext(ctx, `SELECT ee.id::text,ee.event_id::text,ee.rda_id FROM training.event_expense ee JOIN training.training_event ev ON ev.id=ee.event_id WHERE ev.course_id`+in, scopeArgs...)
	if err != nil {
		return snap, fmt.Errorf("load planning expenses: %w", err)
	}
	for rows.Next() {
		var x planningExpense
		if err := rows.Scan(&x.ID, &x.EventID, &x.POID); err != nil {
			rows.Close()
			return snap, err
		}
		snap.Expenses = append(snap.Expenses, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return snap, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT eee.expense_id::text,eee.enrollment_id::text FROM training.event_expense_enrollment eee JOIN training.event_expense ee ON ee.id=eee.expense_id JOIN training.training_event ev ON ev.id=ee.event_id WHERE ev.course_id`+in, scopeArgs...)
	if err != nil {
		return snap, fmt.Errorf("load planning expense coverage: %w", err)
	}
	for rows.Next() {
		var x planningExpenseEnrollment
		if err := rows.Scan(&x.ExpenseID, &x.EnrollmentID); err != nil {
			rows.Close()
			return snap, err
		}
		snap.ExpenseEnrollments = append(snap.ExpenseEnrollments, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return snap, err
	}
	// Set-based dimensions: no per-course/person queries and no inherited caps.
	rows, err = tx.QueryContext(ctx, `SELECT csa.course_id::text,sa.id::text,sa.name FROM training.course_skill_area csa JOIN training.skill_area sa ON sa.id=csa.skill_area_id JOIN training.course c ON c.id=csa.course_id`+scope, args...)
	if err != nil {
		return snap, fmt.Errorf("load planning course areas: %w", err)
	}
	courseIndex := map[string]int{}
	for i := range snap.Courses {
		courseIndex[snap.Courses[i].ID] = i
	}
	for rows.Next() {
		var cid string
		var a PlanningRef
		if err := rows.Scan(&cid, &a.ID, &a.Name); err != nil {
			rows.Close()
			return snap, err
		}
		if i, ok := courseIndex[cid]; ok {
			snap.Courses[i].Areas = append(snap.Courses[i].Areas, a)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snap, err
	}
	rows.Close()
	rows, err = tx.QueryContext(ctx, `SELECT rsa.request_id::text,sa.id::text,sa.name,rsa.level_current,rsa.level_target FROM training.training_request_skill_area rsa JOIN training.skill_area sa ON sa.id=rsa.skill_area_id JOIN training.training_request r ON r.id=rsa.request_id WHERE `+requestScope, requestArgs...)
	if err != nil {
		return snap, fmt.Errorf("load planning request areas: %w", err)
	}
	requestIndex := map[string]int{}
	for i := range snap.Requests {
		requestIndex[snap.Requests[i].ID] = i
	}
	for rows.Next() {
		var rid string
		var a PlanningArea
		var current, target sql.NullInt64
		if err := rows.Scan(&rid, &a.ID, &a.Name, &current, &target); err != nil {
			rows.Close()
			return snap, err
		}
		if current.Valid {
			v := int(current.Int64)
			a.LevelCurrent = &v
		}
		if target.Valid {
			v := int(target.Int64)
			a.LevelTarget = &v
		}
		if i, ok := requestIndex[rid]; ok {
			snap.Requests[i].Areas = append(snap.Requests[i].Areas, a)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snap, err
	}
	rows.Close()
	rows, err = tx.QueryContext(ctx, `SELECT tm.employee_id::text,t.id::text,t.name FROM training.team_membership tm JOIN training.team t ON t.id=tm.team_id WHERE tm.end_date IS NULL AND tm.employee_id IN (SELECT r.employee_id FROM training.training_request r WHERE `+requestScope+` UNION SELECT en.employee_id FROM training.enrollment en JOIN training.training_event ev ON ev.id=en.event_id WHERE ev.course_id`+in+`)`, scopeArgs...)
	if err != nil {
		return snap, fmt.Errorf("load planning current teams: %w", err)
	}
	for rows.Next() {
		var employeeID string
		var team PlanningRef
		if err := rows.Scan(&employeeID, &team.ID, &team.Name); err != nil {
			rows.Close()
			return snap, err
		}
		snap.CurrentTeams[employeeID] = append(snap.CurrentTeams[employeeID], team)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return snap, err
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		return snap, fmt.Errorf("commit planning snapshot: %w", err)
	}
	return snap, nil
}

// UpdatePlanningReminder is deliberately narrower than the legacy annotation
// editor: it remains available for closed requests and cancelled events and
// changes no domain fact other than the two reminder columns.
func (s *SQLStore) UpdatePlanningReminder(ctx context.Context, principal Principal, kind, id string, input ReminderUpdateInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if input.Text == nil {
		return ActionResponse{}, planningBadRequest("invalid_reminder", "testo obbligatorio")
	}
	id = strings.TrimSpace(id)
	text := strings.TrimSpace(*input.Text)
	var date any
	if text != "" && input.Date != nil {
		date = *input.Date
	}

	// The mapping is closed over constants rather than interpolating client input.
	table, entityType := "", ""
	switch kind {
	case "course":
		table, entityType = "course", "course"
	case "request":
		table, entityType = "training_request", "training_request"
	case "event":
		table, entityType = "training_event", "training_event"
	default:
		return ActionResponse{}, planningBadRequest("invalid_kind", "tipo promemoria non valido")
	}

	response := ActionResponse{ID: id}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		// This lock establishes last-save-wins serialization before snapshotting.
		var lockedID string
		if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT id::text FROM training.%s WHERE id = $1::uuid FOR UPDATE", table), id).Scan(&lockedID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return notFoundError(kind+"_not_found", "elemento non trovato")
			}
			return fmt.Errorf("lock training reminder owner: %w", err)
		}
		before, err := entitySnapshot(ctx, tx, table, lockedID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`UPDATE training.%s
SET reminder_text = NULLIF($2, ''), reminder_at = $3::date, updated_at = now()
WHERE id = $1::uuid`, table), lockedID, text, date); err != nil {
			return fmt.Errorf("update training reminder: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, table, lockedID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, entityType, lockedID, "reminder_update", before, after); err != nil {
			return err
		}
		response.ID = lockedID
		response.OK = true
		return nil
	})
	return response, err
}
