package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func (s *SQLStore) ListNeeds(ctx context.Context, id string) ([]Need, error) {
	// Id vuoto = elenco completo; un id malformato non è un errore del server:
	// vale come esigenza inesistente, come per un uuid valido ma assente.
	id = strings.TrimSpace(id)
	if id != "" {
		parsed, err := uuid.Parse(id)
		if err != nil {
			return nil, notFoundError("need_not_found", "esigenza non trovata")
		}
		id = parsed.String()
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT n.id::text, n.description, n.status, COALESCE(n.final_course_id::text, ''),
  COALESCE(c.title, ''), COALESCE(n.notes, ''), COALESCE(n.reminder_text, ''),
  COALESCE(n.reminder_at::text, ''), n.created_at::text, n.updated_at::text,
  (SELECT count(*) FROM training.training_need_course nc WHERE nc.need_id = n.id),
  (SELECT count(*) FROM training.training_need_request nr WHERE nr.need_id = n.id),
  COALESCE((SELECT json_agg(json_build_object('id', sa.id::text, 'name', sa.name) ORDER BY sa.name)
    FROM training.training_need_skill_area na JOIN training.skill_area sa ON sa.id = na.skill_area_id
    WHERE na.need_id = n.id), '[]')::text
FROM training.training_need n
LEFT JOIN training.course c ON c.id = n.final_course_id
WHERE n.deleted_at IS NULL AND ($1 = '' OR n.id = NULLIF($1, '')::uuid)
ORDER BY n.updated_at DESC, n.id`, id)
	if err != nil {
		return nil, fmt.Errorf("list training needs: %w", err)
	}
	defer rows.Close()
	result := make([]Need, 0)
	for rows.Next() {
		var n Need
		var areas string
		if err := rows.Scan(&n.ID, &n.Description, &n.Status, &n.FinalCourseID, &n.FinalCourseTitle,
			&n.Notes, &n.ReminderText, &n.ReminderAt, &n.CreatedAt, &n.UpdatedAt,
			&n.CandidatesCount, &n.RequestsCount, &areas); err != nil {
			return nil, fmt.Errorf("scan training need: %w", err)
		}
		if n.SkillAreas, err = decodeJSONSlice[SkillAreaRef](areas); err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

func (s *SQLStore) GetNeedDetail(ctx context.Context, id string) (NeedDetail, error) {
	needs, err := s.ListNeeds(ctx, id)
	if err != nil {
		return NeedDetail{}, err
	}
	if len(needs) != 1 {
		return NeedDetail{}, notFoundError("need_not_found", "esigenza non trovata")
	}
	detail := NeedDetail{Need: needs[0], Candidates: []NeedCandidate{}, Coverage: []NeedCoverage{}}
	rows, err := s.db.QueryContext(ctx, `
SELECT c.id::text, c.title, c.is_active, COALESCE(nc.notes, ''), nc.rank
FROM training.training_need_course nc JOIN training.course c ON c.id = nc.course_id
WHERE nc.need_id = $1::uuid ORDER BY nc.rank NULLS LAST, c.title, c.id`, id)
	if err != nil {
		return detail, fmt.Errorf("load need candidates: %w", err)
	}
	for rows.Next() {
		var c NeedCandidate
		if err := rows.Scan(&c.CourseID, &c.CourseTitle, &c.IsActive, &c.Notes, &c.Rank); err != nil {
			rows.Close()
			return detail, err
		}
		detail.Candidates = append(detail.Candidates, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return detail, err
	}
	detail.Requests, err = needRequests(ctx, s.db, id, false)
	if err != nil {
		return detail, err
	}
	seen := map[string]bool{}
	for _, r := range detail.Requests {
		if seen[r.EmployeeID] {
			continue
		}
		seen[r.EmployeeID] = true
		for _, c := range detail.Candidates {
			coverage, err := s.requestExistingCoverage(ctx, r.EmployeeID, c.CourseID, c.CourseTitle)
			if err != nil {
				return detail, err
			}
			detail.Coverage = append(detail.Coverage, NeedCoverage{
				EmployeeID: r.EmployeeID, EmployeeName: r.EmployeeName, RequestCoverage: coverage,
			})
		}
	}
	return detail, nil
}

// Il lock delle richieste precede quello dell'evento, come nella decisione individuale.
// ORDER BY r.id serializza nello stesso ordine anche esigenze con richieste condivise.
func needRequests(ctx context.Context, q sqlRunner, id string, lock bool) ([]NeedRequest, error) {
	query := `
SELECT r.id::text, r.description, r.employee_id::text, concat(e.last_name, ' ', e.first_name),
  COALESCE(r.outcome, ''), COALESCE(r.suspended_at::text, ''),
  COALESCE(r.accepted_course_id::text, ''), COALESCE(c.title, ''), COALESCE(r.accepted_event_id::text, '')
FROM training.training_need_request nr
JOIN training.training_request r ON r.id = nr.request_id
JOIN training.employee e ON e.id = r.employee_id
LEFT JOIN training.course c ON c.id = r.accepted_course_id
WHERE nr.need_id = $1::uuid ORDER BY r.id`
	if lock {
		query += ` FOR UPDATE OF r`
	}
	rows, err := q.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("load need requests: %w", err)
	}
	defer rows.Close()
	result := make([]NeedRequest, 0)
	for rows.Next() {
		var r NeedRequest
		if err := rows.Scan(&r.ID, &r.Description, &r.EmployeeID, &r.EmployeeName, &r.Outcome,
			&r.SuspendedAt, &r.AcceptedCourseID, &r.AcceptedCourseTitle, &r.AcceptedEventID); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func lockNeed(ctx context.Context, tx *sql.Tx, id string) (Need, error) {
	var n Need
	err := tx.QueryRowContext(ctx, `
SELECT id::text, status, COALESCE(final_course_id::text, '')
FROM training.training_need WHERE id = $1::uuid AND deleted_at IS NULL FOR UPDATE`, id).
		Scan(&n.ID, &n.Status, &n.FinalCourseID)
	if errors.Is(err, sql.ErrNoRows) {
		return n, notFoundError("need_not_found", "esigenza non trovata")
	}
	return n, err
}

// Lo snapshot comprende i ponti: l'audit conserva anche modifiche alla rosa e ai collegamenti.
func needSnapshot(ctx context.Context, tx *sql.Tx, id string) (json.RawMessage, error) {
	var raw []byte
	err := tx.QueryRowContext(ctx, `
SELECT to_jsonb(n) || jsonb_build_object(
  'candidates', COALESCE((SELECT jsonb_agg(to_jsonb(nc) ORDER BY nc.course_id) FROM training.training_need_course nc WHERE nc.need_id = n.id), '[]'),
  'requestIds', COALESCE((SELECT jsonb_agg(nr.request_id ORDER BY nr.request_id) FROM training.training_need_request nr WHERE nr.need_id = n.id), '[]'),
  'skillAreaIds', COALESCE((SELECT jsonb_agg(na.skill_area_id ORDER BY na.skill_area_id) FROM training.training_need_skill_area na WHERE na.need_id = n.id), '[]'))
FROM training.training_need n WHERE n.id = $1::uuid`, id).Scan(&raw)
	return json.RawMessage(raw), err
}

// Tutte le modifiche ai figli bloccano prima l'esigenza: rosa, definitivo,
// collegamenti e accoglimento vedono un insieme stabile nella propria transazione.
func (s *SQLStore) changeNeed(ctx context.Context, principal Principal, id, action string, change func(*sql.Tx, Need) error) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		n, err := lockNeed(ctx, tx, id)
		if err != nil {
			return err
		}
		before, err := needSnapshot(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := change(tx, n); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE training.training_need SET updated_at = now() WHERE id = $1::uuid`, id); err != nil {
			return err
		}
		after, err := needSnapshot(ctx, tx, id)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, principal, "training_need", id, action, before, after)
	})
	return ActionResponse{OK: err == nil, ID: id}, err
}

func (s *SQLStore) SaveNeed(ctx context.Context, principal Principal, id string, input NeedInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	input.Description = strings.TrimSpace(input.Description)
	input.Status = strings.TrimSpace(input.Status)
	input.FinalCourseID = strings.TrimSpace(input.FinalCourseID)
	if input.Description == "" {
		return ActionResponse{}, validationError("description_required", "descrizione obbligatoria")
	}
	if input.Status == "" && id == "" {
		input.Status = "new"
	}
	switch input.Status {
	case "new", "scouting", "finalizing", "closed", "cancelled":
	default:
		return ActionResponse{}, validationError("invalid_need_status", "stato esigenza non valido")
	}
	if input.Status == "closed" && input.FinalCourseID == "" {
		return ActionResponse{}, validationError("final_course_required", "per chiudere l'esigenza scegli il corso definitivo")
	}
	reminderAt, err := parseOptionalDate(input.ReminderAt)
	if err != nil {
		return ActionResponse{}, validationError("invalid_reminder_at", "data promemoria non valida")
	}
	areaIDs, err := s.normalizeSkillAreaIDs(ctx, input.SkillAreaIDs)
	if err != nil {
		return ActionResponse{}, err
	}
	var response ActionResponse
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var before json.RawMessage
		action := "create"
		if id != "" {
			if _, err := lockNeed(ctx, tx, id); err != nil {
				return err
			}
			before, err = needSnapshot(ctx, tx, id)
			if err != nil {
				return err
			}
			action = "update"
		}
		if input.FinalCourseID != "" {
			var candidate bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM training.training_need_course WHERE need_id = NULLIF($1, '')::uuid AND course_id = $2::uuid)`, id, input.FinalCourseID).Scan(&candidate); err != nil {
				return err
			}
			if !candidate {
				return validationError("final_course_not_candidate", "il corso definitivo deve appartenere alla rosa")
			}
		}
		if id == "" {
			if err := tx.QueryRowContext(ctx, `INSERT INTO training.training_need (description, status, notes, reminder_text, reminder_at)
VALUES ($1, $2, $3, $4, $5::date) RETURNING id::text`, input.Description, input.Status, nullableText(input.Notes), nullableText(input.ReminderText), reminderAt).Scan(&id); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `UPDATE training.training_need SET description = $2, status = $3,
final_course_id = $4::uuid, notes = $5, reminder_text = $6, reminder_at = $7::date, updated_at = now() WHERE id = $1::uuid`,
				id, input.Description, input.Status, nullableUUID(input.FinalCourseID), nullableText(input.Notes), nullableText(input.ReminderText), reminderAt); err != nil {
				return err
			}
		}
		if err := replaceSkillAreaLinks(ctx, tx, "training_need_skill_area", "need_id", id, areaIDs); err != nil {
			return err
		}
		after, err := needSnapshot(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_need", id, action, before, after); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id, Status: input.Status}
		return nil
	})
	return response, err
}

func (s *SQLStore) DeleteNeed(ctx context.Context, principal Principal, id string) (ActionResponse, error) {
	return s.changeNeed(ctx, principal, id, "delete", func(tx *sql.Tx, _ Need) error {
		_, err := tx.ExecContext(ctx, `UPDATE training.training_need SET deleted_at = now() WHERE id = $1::uuid`, id)
		return err
	})
}

func (s *SQLStore) SaveNeedCandidate(ctx context.Context, principal Principal, id, courseID string, input NeedCandidateInput) (ActionResponse, error) {
	return s.changeNeed(ctx, principal, id, "save_candidate", func(tx *sql.Tx, _ Need) error {
		if courseID != "" {
			result, err := tx.ExecContext(ctx, `UPDATE training.training_need_course SET notes = $3, rank = $4 WHERE need_id = $1::uuid AND course_id = $2::uuid`, id, courseID, nullableText(input.Notes), input.Rank)
			if err != nil {
				return err
			}
			count, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if count == 0 {
				return notFoundError("candidate_not_found", "candidato non trovato nella rosa")
			}
			return nil
		}
		courseID = strings.TrimSpace(input.CourseID)
		title := strings.TrimSpace(input.NewCourseTitle)
		if (courseID == "") == (title == "") {
			return validationError("candidate_course_required", "scegli un corso oppure inserisci il titolo del nuovo candidato")
		}
		if courseID == "" {
			var err error
			courseID, err = s.resolveOrCreateEmbryoCourse(ctx, tx, principal, title)
			if err != nil {
				return err
			}
		} else if err := s.ensureCourseExists(ctx, tx, courseID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO training.training_need_course (need_id, course_id, notes, rank)
VALUES ($1::uuid, $2::uuid, $3, $4) ON CONFLICT (need_id, course_id) DO NOTHING`, id, courseID, nullableText(input.Notes), input.Rank)
		return err
	})
}

func (s *SQLStore) RemoveNeedCandidate(ctx context.Context, principal Principal, id, courseID string) (ActionResponse, error) {
	return s.changeNeed(ctx, principal, id, "remove_candidate", func(tx *sql.Tx, n Need) error {
		if n.FinalCourseID == courseID {
			return conflictError("candidate_is_final", "cambia o rimuovi il corso definitivo prima di togliere il candidato")
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM training.training_need_course WHERE need_id = $1::uuid AND course_id = $2::uuid`, id, courseID)
		return err
	})
}

func (s *SQLStore) AddNeedRequests(ctx context.Context, principal Principal, id string, input NeedRequestsInput) (ActionResponse, error) {
	return s.changeNeed(ctx, principal, id, "link_requests", func(tx *sql.Tx, _ Need) error {
		for _, requestID := range input.RequestIDs {
			var exists bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM training.training_request WHERE id = $1::uuid)`, requestID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return notFoundError("request_not_found", "richiesta non trovata")
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO training.training_need_request (need_id, request_id)
VALUES ($1::uuid, $2::uuid) ON CONFLICT (need_id, request_id) DO NOTHING`, id, requestID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SQLStore) RemoveNeedRequest(ctx context.Context, principal Principal, id, requestID string) (ActionResponse, error) {
	return s.changeNeed(ctx, principal, id, "unlink_request", func(tx *sql.Tx, _ Need) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM training.training_need_request WHERE need_id = $1::uuid AND request_id = $2::uuid`, id, requestID)
		return err
	})
}

func (s *SQLStore) AcceptNeed(ctx context.Context, principal Principal, id string, input NeedAcceptInput) (NeedAcceptResponse, error) {
	if !principal.IsPeopleAdmin {
		return NeedAcceptResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	response := NeedAcceptResponse{AcceptedRequestIDs: []string{}, ExcludedRequests: []NeedRequest{}}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		n, err := lockNeed(ctx, tx, id)
		if err != nil {
			return err
		}
		if n.FinalCourseID == "" {
			return validationError("final_course_required", "scegli il corso definitivo prima di accogliere")
		}
		accepted, err := normalizeAcceptance(&RequestAcceptedInput{CourseID: n.FinalCourseID, EventID: input.EventID,
			VendorID: input.VendorID, PeriodStart: input.PeriodStart, PeriodEnd: input.PeriodEnd, Notes: input.Notes})
		if err != nil {
			return err
		}
		requests, err := needRequests(ctx, tx, id, true)
		if err != nil {
			return err
		}
		open := make([]NeedRequest, 0, len(requests))
		for _, r := range requests {
			if r.Outcome != "" {
				response.ExcludedRequests = append(response.ExcludedRequests, r)
			} else {
				open = append(open, r)
			}
		}
		// Nessun lavoro: niente evento, iscrizioni o decisioni nuove,
		// quindi nemmeno audit dell'esigenza o bump di updated_at.
		if len(open) == 0 {
			return nil
		}
		needBefore, err := needSnapshot(ctx, tx, id)
		if err != nil {
			return err
		}
		if accepted.EventID == "" {
			accepted.EventID, err = s.createRequestEvent(ctx, tx, principal, accepted.CourseID, open[0].ID)
			if err != nil {
				return err
			}
		}
		// Serializza anche iscrizioni create da altre esigenze sullo stesso evento.
		var eventCourseID string
		var cancelled bool
		if err := tx.QueryRowContext(ctx, `SELECT course_id::text, cancelled_at IS NOT NULL FROM training.training_event WHERE id = $1::uuid FOR UPDATE`, accepted.EventID).Scan(&eventCourseID, &cancelled); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return validationError("event_not_found", "evento non trovato")
			}
			return err
		}
		if cancelled {
			return conflictError("event_cancelled", "l'evento scelto è annullato")
		}
		if eventCourseID != accepted.CourseID {
			return validationError("event_course_mismatch", "l'evento scelto non appartiene al corso definitivo")
		}
		actorID := s.actorEmployeeID(ctx, tx, principal)
		for _, r := range open {
			accepted.ExistingEnrollmentID = ""
			// L'indice unico (employee_id, event_id) vieta una seconda
			// iscrizione: se quella esistente è annullata l'accoglimento si
			// ferma, ma dice di chi è l'iscrizione che lo blocca.
			enrollmentStatus := ""
			err := tx.QueryRowContext(ctx, `SELECT id::text, delivery_status FROM training.enrollment WHERE employee_id = $1::uuid AND event_id = $2::uuid FOR UPDATE`, r.EmployeeID, accepted.EventID).Scan(&accepted.ExistingEnrollmentID, &enrollmentStatus)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if enrollmentStatus == deliveryCancelled {
				return conflictError("enrollment_cancelled", fmt.Sprintf("l'iscrizione di %s all'evento scelto è annullata", r.EmployeeName))
			}
			before, err := entitySnapshot(ctx, tx, "training_request", r.ID)
			if err != nil {
				return err
			}
			facts := requestPolicyFacts{ID: r.ID, EmployeeID: r.EmployeeID}
			if err := s.applyRequestAcceptance(ctx, tx, principal, facts, accepted, actorID, strings.TrimSpace(input.Reason)); err != nil {
				return err
			}
			after, err := entitySnapshot(ctx, tx, "training_request", r.ID)
			if err != nil {
				return err
			}
			if err := s.audit(ctx, tx, principal, "training_request", r.ID, "decision", before, after); err != nil {
				return err
			}
			response.AcceptedRequestIDs = append(response.AcceptedRequestIDs, r.ID)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE training.training_need SET updated_at = now() WHERE id = $1::uuid`, id); err != nil {
			return err
		}
		needAfter, err := needSnapshot(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_need", id, "accept", needBefore, needAfter); err != nil {
			return err
		}
		response.EventID = accepted.EventID
		return nil
	})
	if err != nil {
		return NeedAcceptResponse{}, err
	}
	return response, nil
}
