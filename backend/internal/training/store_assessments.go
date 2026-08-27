package training

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Valutazioni di competenza per area (#161, slice 2 del task 7): CRUD
// diretto sulla tabella skill_assessment della 012, un record per
// osservazione (nessuno storico riscritto).

func validValidationSource(source string) bool {
	switch source {
	case "document_verified", "declared_survey", "declared_verbal", "declared_cv", "imported_legacy":
		return true
	default:
		return false
	}
}

func (s *SQLStore) ensureEmployeeExists(ctx context.Context, q sqlRunner, employeeID string) error {
	var exists bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM training.employee WHERE id = $1::uuid)`, employeeID).Scan(&exists); err != nil {
		return fmt.Errorf("check training employee: %w", err)
	}
	if !exists {
		return notFoundError("employee_not_found", "persona non trovata")
	}
	return nil
}

// CreateAssessment registra una nuova osservazione; il vincolo UNIQUE
// (persona, area, data) della 012 diventa un 409 esplicito.
func (s *SQLStore) CreateAssessment(ctx context.Context, principal Principal, employeeID string, input AssessmentInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	employeeID = strings.TrimSpace(employeeID)
	skillAreaID := strings.TrimSpace(input.SkillAreaID)
	if employeeID == "" || skillAreaID == "" {
		return ActionResponse{}, validationError("missing_id", "id persona e area di competenza obbligatori")
	}
	if input.Level == nil {
		return ActionResponse{}, validationError("level_required", "livello obbligatorio")
	}
	if *input.Level < 0 || *input.Level > 5 {
		return ActionResponse{}, validationError("invalid_level", "livello non valido: 0-5")
	}
	assessedOn := strings.TrimSpace(input.AssessedOn)
	if assessedOn == "" {
		assessedOn = time.Now().Format("2006-01-02")
	}
	if _, err := parseOptionalDate(assessedOn); err != nil {
		return ActionResponse{}, validationError("invalid_assessed_on", "data valutazione non valida")
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "declared_survey"
	}
	if !validValidationSource(source) {
		return ActionResponse{}, validationError("invalid_source", "fonte non valida")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.ensureEmployeeExists(ctx, tx, employeeID); err != nil {
			return err
		}
		if err := s.ensureSkillAreaExists(ctx, tx, skillAreaID); err != nil {
			return err
		}
		const stmt = `
INSERT INTO training.skill_assessment (employee_id, skill_area_id, level, assessed_on, source, notes)
VALUES ($1::uuid, $2::uuid, $3, $4::date, $5::training.validation_source, NULLIF($6, ''))
RETURNING id::text`
		if err := tx.QueryRowContext(ctx, stmt, employeeID, skillAreaID, *input.Level, assessedOn, source, input.Notes).Scan(&response.ID); err != nil {
			if isUniqueViolation(err, "") {
				return conflictError("assessment_duplicate", "esiste gia una valutazione per questa persona, area e data")
			}
			return fmt.Errorf("create training skill assessment: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "skill_assessment", response.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "skill_assessment", response.ID, "create", nil, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

// UpdateAssessment sostituisce interamente il record; il vincolo UNIQUE
// resta un 409 esplicito anche in aggiornamento (es. data spostata su una
// gia occupata).
func (s *SQLStore) UpdateAssessment(ctx context.Context, principal Principal, id string, input AssessmentUpdateInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id valutazione obbligatorio")
	}
	if input.Level == nil {
		return ActionResponse{}, validationError("level_required", "livello obbligatorio")
	}
	if *input.Level < 0 || *input.Level > 5 {
		return ActionResponse{}, validationError("invalid_level", "livello non valido: 0-5")
	}
	assessedOn := strings.TrimSpace(input.AssessedOn)
	if assessedOn == "" {
		return ActionResponse{}, validationError("assessed_on_required", "data valutazione obbligatoria")
	}
	if _, err := parseOptionalDate(assessedOn); err != nil {
		return ActionResponse{}, validationError("invalid_assessed_on", "data valutazione non valida")
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "declared_survey"
	}
	if !validValidationSource(source) {
		return ActionResponse{}, validationError("invalid_source", "fonte non valida")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		before, err := entitySnapshot(ctx, tx, "skill_assessment", id)
		if appErr, ok := asAppError(err); ok && appErr.code == "entity_not_found" {
			return notFoundError("assessment_not_found", "valutazione non trovata")
		}
		if err != nil {
			return err
		}
		const stmt = `
UPDATE training.skill_assessment
SET level = $2, assessed_on = $3::date, source = $4::training.validation_source, notes = NULLIF($5, '')
WHERE id = $1::uuid`
		if _, err := tx.ExecContext(ctx, stmt, id, *input.Level, assessedOn, source, input.Notes); err != nil {
			if isUniqueViolation(err, "") {
				return conflictError("assessment_duplicate", "esiste gia una valutazione per questa persona, area e data")
			}
			return fmt.Errorf("update training skill assessment: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "skill_assessment", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "skill_assessment", id, "update", before, after); err != nil {
			return err
		}
		response.OK, response.ID = true, id
		return nil
	})
	return response, err
}

func (s *SQLStore) DeleteAssessment(ctx context.Context, principal Principal, id string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id valutazione obbligatorio")
	}
	response := ActionResponse{OK: true, ID: id}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		before, err := entitySnapshot(ctx, tx, "skill_assessment", id)
		if appErr, ok := asAppError(err); ok && appErr.code == "entity_not_found" {
			return notFoundError("assessment_not_found", "valutazione non trovata")
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM training.skill_assessment WHERE id = $1::uuid`, id); err != nil {
			return fmt.Errorf("delete training skill assessment: %w", err)
		}
		return s.audit(ctx, tx, principal, "skill_assessment", id, "delete", before, nil)
	})
	return response, err
}
