package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Annotazioni di pianificazione e sospensioni (decisioni di agosto 2026):
// nota libera, promemoria con data di richiamo facoltativa e sospensione
// motivata reversibile su richiesta e corso; promemoria anche sull'evento
// (gestito nell'update evento). La sospesa esce dalle viste operative di
// default; l'audit conserva la storia.

// replaceEmployeeLinks sostituisce l'elenco di persone di una tabella ponte
// (course_trainer, event_trainer, course_visibility_person).
func replaceEmployeeLinks(ctx context.Context, tx *sql.Tx, table, ownerColumn, ownerID string, employeeIDs []string) error {
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM training.%s WHERE %s = $1::uuid", table, ownerColumn), ownerID); err != nil {
		return fmt.Errorf("clear %s: %w", table, err)
	}
	if len(employeeIDs) == 0 {
		return nil
	}
	payload, err := json.Marshal(employeeIDs)
	if err != nil {
		return fmt.Errorf("encode employee ids: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO training.%s (%s, employee_id)
SELECT $1::uuid, t.x::uuid FROM jsonb_array_elements_text($2::jsonb) AS t(x)`, table, ownerColumn),
		ownerID, string(payload)); err != nil {
		return fmt.Errorf("insert %s: %w", table, err)
	}
	return nil
}

// normalizeActiveEmployeeIDs ripulisce gli id persona (spazi, vuoti, doppioni)
// e verifica che siano tutti in anagrafica attiva.
func (s *SQLStore) normalizeActiveEmployeeIDs(ctx context.Context, ids []string) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return out, nil
	}
	payload, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encode employee ids: %w", err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM training.employee
WHERE status = 'active'
  AND id IN (SELECT t.x::uuid FROM jsonb_array_elements_text($1::jsonb) AS t(x))`, string(payload)).Scan(&count); err != nil {
		return nil, fmt.Errorf("check training employees: %w", err)
	}
	if count != len(out) {
		return nil, validationError("employee_not_active", "persona non trovata o non attiva")
	}
	return out, nil
}

// normalizeCourseVisibility valida la platea di visibilita e restituisce il
// JSON per visibility_target piu l'elenco persone del ponte (solo kind
// «people»). Visibility nil = NULL = riservato a People.
func (s *SQLStore) normalizeCourseVisibility(ctx context.Context, v *CourseVisibilityInput) (any, []string, error) {
	if v == nil {
		return nil, nil, nil
	}
	kind := strings.TrimSpace(v.Kind)
	switch kind {
	case "team", "skill_area", "custom_group":
		id := strings.TrimSpace(v.ID)
		if id == "" {
			return nil, nil, validationError("visibility_id_required", "id della cerchia di visibilita obbligatorio")
		}
		table := map[string]string{"team": "team", "skill_area": "skill_area", "custom_group": "custom_groups"}[kind]
		var exists bool
		if err := s.db.QueryRowContext(ctx,
			fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM training.%s WHERE id = $1::uuid)", table), id).Scan(&exists); err != nil {
			return nil, nil, fmt.Errorf("check course visibility target: %w", err)
		}
		if !exists {
			return nil, nil, validationError("visibility_target_not_found", "cerchia di visibilita non trovata")
		}
		payload, err := json.Marshal(map[string]string{"kind": kind, "id": id})
		if err != nil {
			return nil, nil, fmt.Errorf("encode course visibility: %w", err)
		}
		return string(payload), nil, nil
	case "people":
		people, err := s.normalizeActiveEmployeeIDs(ctx, v.EmployeeIDs)
		if err != nil {
			return nil, nil, err
		}
		payload, err := json.Marshal(map[string]string{"kind": kind})
		if err != nil {
			return nil, nil, fmt.Errorf("encode course visibility: %w", err)
		}
		return string(payload), people, nil
	case "all":
		payload, err := json.Marshal(map[string]string{"kind": kind})
		if err != nil {
			return nil, nil, fmt.Errorf("encode course visibility: %w", err)
		}
		return string(payload), nil, nil
	default:
		return nil, nil, validationError("invalid_visibility_kind", "platea di visibilita non valida")
	}
}

// validateLevel valida un livello facoltativo sulla scala 0-5.
func validateLevel(value *int, code string) error {
	if value != nil && (*value < 0 || *value > 5) {
		return validationError(code, "livello fuori scala (0-5)")
	}
	return nil
}

// normalizeRequestAreas ripulisce le aree dell'esigenza (doppioni per id) e
// valida id e livelli (scala 0-5); riusa la verifica di esistenza delle aree.
func (s *SQLStore) normalizeRequestAreas(ctx context.Context, areas []RequestSkillAreaInput) ([]RequestSkillAreaInput, error) {
	seen := map[string]struct{}{}
	out := make([]RequestSkillAreaInput, 0, len(areas))
	ids := make([]string, 0, len(areas))
	for _, a := range areas {
		a.ID = strings.TrimSpace(a.ID)
		if a.ID == "" {
			continue
		}
		if _, ok := seen[a.ID]; ok {
			continue
		}
		if err := validateLevel(a.LevelCurrent, "invalid_level_current"); err != nil {
			return nil, err
		}
		if err := validateLevel(a.LevelTarget, "invalid_level_target"); err != nil {
			return nil, err
		}
		seen[a.ID] = struct{}{}
		out = append(out, a)
		ids = append(ids, a.ID)
	}
	if _, err := s.normalizeSkillAreaIDs(ctx, ids); err != nil {
		return nil, err
	}
	return out, nil
}

// replaceRequestSkillAreas sostituisce il legame richiesta<->aree con la
// coppia facoltativa di livelli per area.
func replaceRequestSkillAreas(ctx context.Context, tx *sql.Tx, requestID string, areas []RequestSkillAreaInput) error {
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM training.training_request_skill_area WHERE request_id = $1::uuid", requestID); err != nil {
		return fmt.Errorf("clear training request skill areas: %w", err)
	}
	for _, a := range areas {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO training.training_request_skill_area (request_id, skill_area_id, level_current, level_target)
VALUES ($1::uuid, $2::uuid, $3, $4)`, requestID, a.ID, a.LevelCurrent, a.LevelTarget); err != nil {
			return fmt.Errorf("insert training request skill area: %w", err)
		}
	}
	return nil
}

// suspendEntity congela richiesta o corso; resumeEntity riattiva.
func (s *SQLStore) suspendEntity(ctx context.Context, principal Principal, table, id, reason string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id obbligatorio")
	}
	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var suspended bool
		err := tx.QueryRowContext(ctx,
			fmt.Sprintf("SELECT suspended_at IS NOT NULL FROM training.%s WHERE id = $1::uuid FOR UPDATE", table),
			id).Scan(&suspended)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("not_found", "elemento non trovato")
		}
		if err != nil {
			return fmt.Errorf("load %s for suspend: %w", table, err)
		}
		if suspended {
			return conflictError("already_suspended", "gia sospeso")
		}
		before, err := entitySnapshot(ctx, tx, table, id)
		if err != nil {
			return err
		}
		actorID := s.actorEmployeeID(ctx, tx, principal)
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
UPDATE training.%s
SET suspended_at = now(),
    suspended_by = $2::uuid,
    suspension_reason = NULLIF($3, ''),
    updated_at = now()
WHERE id = $1::uuid`, table), id, nullableUUIDPtr(actorID), strings.TrimSpace(reason)); err != nil {
			return fmt.Errorf("suspend %s: %w", table, err)
		}
		after, err := entitySnapshot(ctx, tx, table, id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, table, id, "suspend", before, after); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id, Status: "suspended"}
		return nil
	})
	return response, err
}

func (s *SQLStore) resumeEntity(ctx context.Context, principal Principal, table, id string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id obbligatorio")
	}
	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var suspended bool
		err := tx.QueryRowContext(ctx,
			fmt.Sprintf("SELECT suspended_at IS NOT NULL FROM training.%s WHERE id = $1::uuid FOR UPDATE", table),
			id).Scan(&suspended)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("not_found", "elemento non trovato")
		}
		if err != nil {
			return fmt.Errorf("load %s for resume: %w", table, err)
		}
		if !suspended {
			return conflictError("not_suspended", "non e sospeso")
		}
		before, err := entitySnapshot(ctx, tx, table, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
UPDATE training.%s
SET suspended_at = NULL,
    suspended_by = NULL,
    suspension_reason = NULL,
    updated_at = now()
WHERE id = $1::uuid`, table), id); err != nil {
			return fmt.Errorf("resume %s: %w", table, err)
		}
		after, err := entitySnapshot(ctx, tx, table, id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, table, id, "resume", before, after); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id, Status: "resumed"}
		return nil
	})
	return response, err
}

func (s *SQLStore) SuspendCourse(ctx context.Context, principal Principal, id string, input ReasonInput) (ActionResponse, error) {
	return s.suspendEntity(ctx, principal, "course", id, input.Reason)
}

func (s *SQLStore) ResumeCourse(ctx context.Context, principal Principal, id string) (ActionResponse, error) {
	return s.resumeEntity(ctx, principal, "course", id)
}

func (s *SQLStore) SuspendRequest(ctx context.Context, principal Principal, id string, input ReasonInput) (ActionResponse, error) {
	return s.suspendEntity(ctx, principal, "training_request", id, input.Reason)
}

func (s *SQLStore) ResumeRequest(ctx context.Context, principal Principal, id string) (ActionResponse, error) {
	return s.resumeEntity(ctx, principal, "training_request", id)
}

// UpdateRequestAnnotations sostituisce nota, promemoria e priorita della
// richiesta (PUT: il campo vuoto azzera). Solo su richiesta aperta.
func (s *SQLStore) UpdateRequestAnnotations(ctx context.Context, principal Principal, id string, input RequestAnnotationsInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id richiesta obbligatorio")
	}
	reminderAt, err := parseOptionalDate(input.ReminderAt)
	if err != nil {
		return ActionResponse{}, validationError("invalid_reminder_at", "data di richiamo non valida")
	}
	var response ActionResponse
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		facts, err := s.lockRequestFacts(ctx, tx, id)
		if err != nil {
			return err
		}
		if facts.Outcome != "" {
			return conflictError("request_closed", "la richiesta e gia chiusa")
		}
		before, err := entitySnapshot(ctx, tx, "training_request", id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE training.training_request
SET notes = NULLIF($2, ''),
    reminder_text = NULLIF($3, ''),
    reminder_at = $4,
    priority = $5,
    updated_at = now()
WHERE id = $1::uuid`,
			id,
			strings.TrimSpace(input.Notes),
			strings.TrimSpace(input.ReminderText),
			reminderAt,
			input.Priority,
		); err != nil {
			return fmt.Errorf("update training request annotations: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "training_request", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_request", id, "annotations", before, after); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id}
		return nil
	})
	return response, err
}
