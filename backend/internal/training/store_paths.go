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

// Percorsi formativi, passi, assegnazioni e progresso (#161, slice 2 del
// task 7): greenfield sulle tabelle di 012 (learning_path,
// learning_path_step, employee_learning_path). Il progresso non si
// persiste mai: si ricalcola in lettura dai fatti (decisioni #159 punto 8).

const pathListColumns = `
  lp.id::text, lp.code, lp.name,
  COALESCE(lp.skill_area_id::text, ''), COALESCE(sa.name, ''),
  COALESCE(lp.description, ''), lp.is_active,
  (SELECT COUNT(*) FROM training.learning_path_step st WHERE st.path_id = lp.id),
  (SELECT COUNT(*) FROM training.employee_learning_path elp WHERE elp.path_id = lp.id)`

const pathListFrom = `
FROM training.learning_path lp
LEFT JOIN training.skill_area sa ON sa.id = lp.skill_area_id`

func scanPathListRow(row interface{ Scan(...any) error }, dest *PathListRow) error {
	return row.Scan(
		&dest.ID, &dest.Code, &dest.Name, &dest.SkillAreaID, &dest.SkillAreaName,
		&dest.Description, &dest.Active, &dest.StepsCount, &dest.AssigneesCount,
	)
}

// ListPaths include i percorsi inattivi (flag active); nessuna lookup
// dedicata a soli attivi e richiesta da questa slice.
func (s *SQLStore) ListPaths(ctx context.Context) ([]PathListRow, error) {
	q := "SELECT" + pathListColumns + pathListFrom + "\nORDER BY lp.name\nLIMIT 500"
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training learning paths: %w", err)
	}
	defer rows.Close()

	result := make([]PathListRow, 0)
	for rows.Next() {
		var row PathListRow
		if err := scanPathListRow(rows, &row); err != nil {
			return nil, fmt.Errorf("scan training learning path: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetPathDetail aggrega anagrafica, passi ordinati e assegnazioni con il
// progresso calcolato. Le assegnazioni si caricano a parte (non via
// json_agg): il progresso richiede query per passo lato Go, che il json_agg
// a riga singola non potrebbe fornire.
func (s *SQLStore) GetPathDetail(ctx context.Context, id string) (PathDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return PathDetail{}, validationError("missing_id", "id percorso obbligatorio")
	}
	q := "SELECT" + pathListColumns + pathListFrom + "\nWHERE lp.id = $1::uuid"
	var detail PathDetail
	if err := scanPathListRow(s.db.QueryRowContext(ctx, q, id), &detail.PathListRow); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PathDetail{}, notFoundError("path_not_found", "percorso non trovato")
		}
		return PathDetail{}, fmt.Errorf("load training learning path: %w", err)
	}

	steps, err := s.pathSteps(ctx, s.db, id)
	if err != nil {
		return PathDetail{}, err
	}
	detail.Steps = steps

	rows, err := s.db.QueryContext(ctx, `
SELECT elp.employee_id::text, concat(e.last_name, ' ', e.first_name),
  elp.started_on::text, COALESCE(elp.target_completion::text, ''), COALESCE(elp.completed_on::text, ''), COALESCE(elp.notes, '')
FROM training.employee_learning_path elp
JOIN training.employee e ON e.id = elp.employee_id
WHERE elp.path_id = $1::uuid
ORDER BY e.last_name, e.first_name`, id)
	if err != nil {
		return PathDetail{}, fmt.Errorf("list training learning path assignees: %w", err)
	}
	defer rows.Close()

	assignees := make([]PathAssigneeRef, 0)
	for rows.Next() {
		var a PathAssigneeRef
		if err := rows.Scan(&a.EmployeeID, &a.EmployeeName, &a.StartedOn, &a.TargetCompletion, &a.CompletedOn, &a.Notes); err != nil {
			return PathDetail{}, fmt.Errorf("scan training learning path assignee: %w", err)
		}
		assignees = append(assignees, a)
	}
	if err := rows.Err(); err != nil {
		return PathDetail{}, err
	}

	if err := s.pathAssigneesProgress(ctx, s.db, steps, assignees); err != nil {
		return PathDetail{}, err
	}
	detail.Assignees = assignees
	return detail, nil
}

// pathSteps carica i passi del percorso ordinati; riusato sia dal dettaglio
// percorso sia dal calcolo progresso in scheda persona (store_people_reads.go).
func (s *SQLStore) pathSteps(ctx context.Context, q sqlRunner, pathID string) ([]PathStepRef, error) {
	rows, err := q.QueryContext(ctx, `
SELECT s.id::text, s.step_order,
  COALESCE(s.course_id::text, ''), COALESCE(c.title, ''),
  COALESCE(s.certification_id::text, ''), COALESCE(cert.name, ''),
  s.is_required, COALESCE(s.notes, '')
FROM training.learning_path_step s
LEFT JOIN training.course c ON c.id = s.course_id
LEFT JOIN training.certification cert ON cert.id = s.certification_id
WHERE s.path_id = $1::uuid
ORDER BY s.step_order`, pathID)
	if err != nil {
		return nil, fmt.Errorf("list training learning path steps: %w", err)
	}
	defer rows.Close()

	result := make([]PathStepRef, 0)
	for rows.Next() {
		var step PathStepRef
		if err := rows.Scan(
			&step.StepID, &step.StepOrder, &step.CourseID, &step.CourseTitle,
			&step.CertificationID, &step.CertificationName, &step.IsRequired, &step.Notes,
		); err != nil {
			return nil, fmt.Errorf("scan training learning path step: %w", err)
		}
		result = append(result, step)
	}
	return result, rows.Err()
}

// assignmentProgress calcola in lettura la copertura di una persona sui
// passi di un percorso (#161, decisioni #159 punto 8): nessuna
// persistenza. I facoltativi non contano nel segnale allRequiredCovered.
// completedOn e il completed_on registrato sull'assegnazione: se valorizzato,
// finalizeProgress lo fa prevalere sul ricalcolo (contratto #159 punto g,
// "assegnazione conclusa che non si riapre").
func (s *SQLStore) assignmentProgress(ctx context.Context, q sqlRunner, employeeID, completedOn string, steps []PathStepRef) (PathProgress, error) {
	progress := PathProgress{Steps: make([]PathStepProgressRef, 0, len(steps))}
	for _, step := range steps {
		ref := PathStepProgressRef{
			StepID: step.StepID, StepOrder: step.StepOrder,
			CourseID: step.CourseID, CourseTitle: step.CourseTitle,
			CertificationID: step.CertificationID, CertificationName: step.CertificationName,
			IsRequired: step.IsRequired,
		}
		if step.CourseID != "" {
			matches, err := s.courseStepCoveredEmployees(ctx, q, step.CourseID, employeeID)
			if err != nil {
				return PathProgress{}, err
			}
			if match, ok := matches[employeeID]; ok {
				ref.Covered, ref.EnrollmentID, ref.EventID = true, match.EnrollmentID, match.EventID
			}
		} else {
			awards, err := s.certStepCoveredEmployees(ctx, q, step.CertificationID, employeeID)
			if err != nil {
				return PathProgress{}, err
			}
			if awardID, ok := awards[employeeID]; ok {
				ref.Covered, ref.AwardID = true, awardID
			}
		}
		if step.IsRequired {
			progress.RequiredTotal++
			if ref.Covered {
				progress.RequiredCovered++
			}
		}
		progress.Steps = append(progress.Steps, ref)
	}
	finalizeProgress(&progress, completedOn)
	return progress, nil
}

// finalizeProgress applica la garanzia del contratto (#159 punto g):
// un'assegnazione con completed_on registrato non si riapre, anche se il
// percorso guadagna un passo con ReplacePathSteps o una certificazione
// coperta scade dopo la conclusione. RequiredCovered/RequiredTotal restano
// il conteggio effettivo (informativo); solo il segnale aggregato
// allRequiredCovered viene forzato a true dalla conclusione registrata.
func finalizeProgress(progress *PathProgress, completedOn string) {
	progress.AllRequiredCovered = progress.RequiredCovered == progress.RequiredTotal || strings.TrimSpace(completedOn) != ""
}

// pathAssigneesProgress calcola il progresso di tutti gli assegnatari di un
// percorso con una query per passo, non una per coppia (assegnatario,
// passo): stesso pattern batch di store_coverage.go:369-410
// (certificationCoveredEmployees/attendanceCoveredEmployees), necessario
// perche lo scenario centrale (percorso obbligatorio aziendale con
// centinaia di assegnatari) renderebbe assignmentProgress per-assegnatario
// assegnatari*passi round-trip sequenziali.
func (s *SQLStore) pathAssigneesProgress(ctx context.Context, q sqlRunner, steps []PathStepRef, assignees []PathAssigneeRef) error {
	courseCoverage := make(map[string]map[string]courseStepMatch, len(steps))
	certCoverage := make(map[string]map[string]string, len(steps))
	for _, step := range steps {
		if step.CourseID != "" {
			matches, err := s.courseStepCoveredEmployees(ctx, q, step.CourseID, "")
			if err != nil {
				return err
			}
			courseCoverage[step.StepID] = matches
		} else {
			awards, err := s.certStepCoveredEmployees(ctx, q, step.CertificationID, "")
			if err != nil {
				return err
			}
			certCoverage[step.StepID] = awards
		}
	}

	for i := range assignees {
		employeeID := assignees[i].EmployeeID
		progress := PathProgress{Steps: make([]PathStepProgressRef, 0, len(steps))}
		for _, step := range steps {
			ref := PathStepProgressRef{
				StepID: step.StepID, StepOrder: step.StepOrder,
				CourseID: step.CourseID, CourseTitle: step.CourseTitle,
				CertificationID: step.CertificationID, CertificationName: step.CertificationName,
				IsRequired: step.IsRequired,
			}
			if step.CourseID != "" {
				if match, ok := courseCoverage[step.StepID][employeeID]; ok {
					ref.Covered, ref.EnrollmentID, ref.EventID = true, match.EnrollmentID, match.EventID
				}
			} else if awardID, ok := certCoverage[step.StepID][employeeID]; ok {
				ref.Covered, ref.AwardID = true, awardID
			}
			if step.IsRequired {
				progress.RequiredTotal++
				if ref.Covered {
					progress.RequiredCovered++
				}
			}
			progress.Steps = append(progress.Steps, ref)
		}
		finalizeProgress(&progress, assignees[i].CompletedOn)
		assignees[i].PathProgress = progress
	}
	return nil
}

// courseStepMatch e l'iscrizione/evento piu recente che copre un passo-corso
// per una persona (usato da pathAssigneesProgress).
type courseStepMatch struct {
	EnrollmentID string
	EventID      string
}

// courseStepCoveredEmployees batches la copertura di un passo-corso: DISTINCT
// ON seleziona per ciascuna persona l'iscrizione/evento piu recente con
// delivery_status='completed' su un evento del corso, qualunque origine e
// data (anche precedente a startedOn, #161). employeeID facoltativo:
// valorizzato per il progresso di una singola persona (assignmentProgress),
// vuoto per il batch di tutti gli assegnatari del percorso
// (pathAssigneesProgress) — stessa query, stessa regola di copertura.
func (s *SQLStore) courseStepCoveredEmployees(ctx context.Context, q sqlRunner, courseID, employeeID string) (map[string]courseStepMatch, error) {
	const query = `
SELECT DISTINCT ON (en.employee_id) en.employee_id::text, en.id::text, en.event_id::text
FROM training.enrollment en
JOIN training.training_event ev ON ev.id = en.event_id
WHERE ev.course_id = $1::uuid
  AND en.delivery_status = 'completed'
  AND ($2::uuid IS NULL OR en.employee_id = $2::uuid)
ORDER BY en.employee_id, COALESCE(en.actual_end, en.updated_at::date) DESC, en.updated_at DESC`
	rows, err := q.QueryContext(ctx, query, courseID, nullableUUID(employeeID))
	if err != nil {
		return nil, fmt.Errorf("load training path course coverage: %w", err)
	}
	defer rows.Close()

	result := make(map[string]courseStepMatch)
	for rows.Next() {
		var rowEmployeeID string
		var match courseStepMatch
		if err := rows.Scan(&rowEmployeeID, &match.EnrollmentID, &match.EventID); err != nil {
			return nil, fmt.Errorf("scan training path course coverage: %w", err)
		}
		result[rowEmployeeID] = match
	}
	return result, rows.Err()
}

// certStepCoveredEmployees batches la copertura di un passo-certificazione,
// stessa semantica di certificationCoveredEmployees (store_coverage.go:369):
// un conseguimento scaduto lascia il passo scoperto. employeeID facoltativo,
// stesso ruolo di courseStepCoveredEmployees.
func (s *SQLStore) certStepCoveredEmployees(ctx context.Context, q sqlRunner, certificationID, employeeID string) (map[string]string, error) {
	const query = `
SELECT DISTINCT ON (ca.employee_id) ca.employee_id::text, ca.id::text
FROM training.certification_award ca
WHERE ca.certification_id = $1::uuid
  AND ca.outcome = 'passed_exam'
  AND (ca.expires_on IS NULL OR ca.expires_on > CURRENT_DATE)
  AND ($2::uuid IS NULL OR ca.employee_id = $2::uuid)
ORDER BY ca.employee_id, ca.awarded_on DESC`
	rows, err := q.QueryContext(ctx, query, certificationID, nullableUUID(employeeID))
	if err != nil {
		return nil, fmt.Errorf("load training path certification coverage: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var rowEmployeeID, awardID string
		if err := rows.Scan(&rowEmployeeID, &awardID); err != nil {
			return nil, fmt.Errorf("scan training path certification coverage: %w", err)
		}
		result[rowEmployeeID] = awardID
	}
	return result, rows.Err()
}

// UpsertPath crea o aggiorna l'anagrafica del percorso; il codice e unico
// (409 esplicito su duplicato).
func (s *SQLStore) UpsertPath(ctx context.Context, principal Principal, id string, input PathInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	code := strings.TrimSpace(input.Code)
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" {
		return ActionResponse{}, validationError("code_name_required", "codice e nome obbligatori")
	}
	id = strings.TrimSpace(id)
	skillAreaID := strings.TrimSpace(input.SkillAreaID)
	active := boolValue(input.Active, true)

	if err := s.ensureSkillAreaExists(ctx, s.db, skillAreaID); err != nil {
		return ActionResponse{}, err
	}
	response, err := s.upsertSimple(ctx, principal, "learning_path", id, []upsertField{
		field("code", code),
		field("name", name),
		typedField("skill_area_id", nullableUUID(skillAreaID), "::uuid"),
		field("description", nullableText(input.Description)),
		field("is_active", active),
	})
	if err != nil {
		if appErr, ok := asAppError(err); ok && appErr.code == "entity_not_found" {
			return ActionResponse{}, notFoundError("path_not_found", "percorso non trovato")
		}
		if isUniqueViolation(err, "") {
			return ActionResponse{}, conflictError("path_code_duplicate", "codice gia usato da un altro percorso")
		}
		return ActionResponse{}, err
	}
	return response, nil
}

// ReplacePathSteps sostituisce atomicamente l'elenco dei passi (delete +
// reinsert nella stessa transazione, stesso pattern di
// replaceExpenseEnrollments in store_expenses.go). Il progresso e calcolato
// in lettura: le assegnazioni esistenti si rivalutano da sole, la
// sostituzione non le corrompe.
func (s *SQLStore) ReplacePathSteps(ctx context.Context, principal Principal, pathID string, input PathStepsInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	pathID = strings.TrimSpace(pathID)
	if pathID == "" {
		return ActionResponse{}, validationError("missing_id", "id percorso obbligatorio")
	}
	seenOrder := make(map[int]struct{}, len(input.Steps))
	for _, step := range input.Steps {
		if _, exists := seenOrder[step.StepOrder]; exists {
			return ActionResponse{}, validationError("step_order_duplicate", "ordine dei passi duplicato")
		}
		seenOrder[step.StepOrder] = struct{}{}
		courseID := strings.TrimSpace(step.CourseID)
		certificationID := strings.TrimSpace(step.CertificationID)
		if (courseID == "") == (certificationID == "") {
			return ActionResponse{}, validationError("step_course_xor_certification", "ogni passo richiede esattamente un corso o una certificazione")
		}
	}

	response := ActionResponse{OK: true, ID: pathID}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.ensurePathExists(ctx, tx, pathID); err != nil {
			return err
		}
		before, err := s.pathStepsSnapshot(ctx, tx, pathID)
		if err != nil {
			return err
		}
		for _, step := range input.Steps {
			if courseID := strings.TrimSpace(step.CourseID); courseID != "" {
				if err := s.ensureCourseExists(ctx, tx, courseID); err != nil {
					return err
				}
			} else if err := s.ensureCertificationExists(ctx, tx, step.CertificationID); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM training.learning_path_step WHERE path_id = $1::uuid`, pathID); err != nil {
			return fmt.Errorf("delete training learning path steps: %w", err)
		}
		for _, step := range input.Steps {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO training.learning_path_step (path_id, step_order, course_id, certification_id, is_required, notes)
VALUES ($1::uuid, $2, $3::uuid, $4::uuid, $5, $6)`,
				pathID, step.StepOrder, nullableUUID(step.CourseID), nullableUUID(step.CertificationID),
				boolValue(step.IsRequired, true), nullableText(step.Notes),
			); err != nil {
				if isUniqueViolation(err, "") {
					return validationError("step_order_duplicate", "ordine dei passi duplicato")
				}
				return fmt.Errorf("insert training learning path step: %w", err)
			}
		}
		after, err := s.pathStepsSnapshot(ctx, tx, pathID)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, principal, "learning_path", pathID, "replace_steps", before, after)
	})
	return response, err
}

func (s *SQLStore) pathStepsSnapshot(ctx context.Context, q sqlRunner, pathID string) (json.RawMessage, error) {
	var raw string
	err := q.QueryRowContext(ctx, `
SELECT COALESCE((
  SELECT json_agg(json_build_object(
    'stepOrder', step_order, 'courseId', course_id, 'certificationId', certification_id,
    'isRequired', is_required, 'notes', notes
  ) ORDER BY step_order)
  FROM training.learning_path_step
  WHERE path_id = $1::uuid
), '[]')::text`, pathID).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("snapshot training learning path steps: %w", err)
	}
	return json.RawMessage(raw), nil
}

func (s *SQLStore) ensurePathExists(ctx context.Context, q sqlRunner, pathID string) error {
	var exists bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM training.learning_path WHERE id = $1::uuid)`, pathID).Scan(&exists); err != nil {
		return fmt.Errorf("check training learning path: %w", err)
	}
	if !exists {
		return notFoundError("path_not_found", "percorso non trovato")
	}
	return nil
}

func (s *SQLStore) pathActive(ctx context.Context, q sqlRunner, pathID string) (bool, error) {
	var active bool
	err := q.QueryRowContext(ctx, `SELECT is_active FROM training.learning_path WHERE id = $1::uuid`, pathID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return false, validationError("path_not_found", "percorso non trovato")
	}
	if err != nil {
		return false, fmt.Errorf("load training learning path: %w", err)
	}
	return active, nil
}

func (s *SQLStore) ensureCertificationExists(ctx context.Context, q sqlRunner, certificationID string) error {
	certificationID = strings.TrimSpace(certificationID)
	var exists bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM training.certification WHERE id = $1::uuid)`, certificationID).Scan(&exists); err != nil {
		return fmt.Errorf("check training certification: %w", err)
	}
	if !exists {
		return validationError("certification_not_found", "certificazione non trovata")
	}
	return nil
}

// ensureSkillAreaExists e riusato dall'anagrafica percorso e dalle
// valutazioni di competenza (store_assessments.go).
func (s *SQLStore) ensureSkillAreaExists(ctx context.Context, q sqlRunner, skillAreaID string) error {
	skillAreaID = strings.TrimSpace(skillAreaID)
	if skillAreaID == "" {
		return nil
	}
	var exists bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM training.skill_area WHERE id = $1::uuid)`, skillAreaID).Scan(&exists); err != nil {
		return fmt.Errorf("check training skill area: %w", err)
	}
	if !exists {
		return validationError("skill_area_not_found", "area di competenza non trovata")
	}
	return nil
}

// AssignPersonPath crea l'assegnazione persona-percorso; la chiave primaria
// (employee_id, path_id) della 012 rende il doppio assegnamento un 409
// esplicito.
func (s *SQLStore) AssignPersonPath(ctx context.Context, principal Principal, employeeID string, input PathAssignmentInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	employeeID = strings.TrimSpace(employeeID)
	pathID := strings.TrimSpace(input.PathID)
	if employeeID == "" || pathID == "" {
		return ActionResponse{}, validationError("missing_id", "id persona e id percorso obbligatori")
	}
	startedOnRaw := strings.TrimSpace(input.StartedOn)
	if startedOnRaw == "" {
		startedOnRaw = time.Now().Format("2006-01-02")
	}
	startedOn, err := parseOptionalDate(startedOnRaw)
	if err != nil || startedOn == nil {
		return ActionResponse{}, validationError("invalid_started_on", "data inizio non valida")
	}
	targetCompletion, err := parseOptionalDate(input.TargetCompletion)
	if err != nil {
		return ActionResponse{}, validationError("invalid_target_completion", "data obiettivo non valida")
	}
	if targetCompletion != nil && targetCompletion.Before(*startedOn) {
		return ActionResponse{}, validationError("target_completion_before_start", "l'obiettivo non puo precedere l'inizio")
	}

	response := ActionResponse{OK: true, ID: pathID}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.ensureEmployeeActive(ctx, tx, employeeID); err != nil {
			return err
		}
		active, err := s.pathActive(ctx, tx, pathID)
		if err != nil {
			return err
		}
		if !active {
			return validationError("path_inactive", "il percorso non e attivo")
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO training.employee_learning_path (employee_id, path_id, started_on, target_completion, notes)
VALUES ($1::uuid, $2::uuid, $3::date, NULLIF($4, '')::date, NULLIF($5, ''))`,
			employeeID, pathID, startedOnRaw, strings.TrimSpace(input.TargetCompletion), input.Notes,
		); err != nil {
			if isUniqueViolation(err, "") {
				return conflictError("path_already_assigned", "la persona e gia assegnata al percorso")
			}
			return fmt.Errorf("assign training learning path: %w", err)
		}
		after, err := s.assignmentSnapshot(ctx, tx, employeeID, pathID)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, principal, "employee_learning_path", employeeID, "assign", nil, after)
	})
	return response, err
}

// UpdatePersonPath sostituisce interamente i fatti dell'assegnazione
// (stesso stile PUT di UpdateAward): i CHECK di 012 sulle date valgono come
// guardia applicativa (422 tradotto) prima di toccare il database.
func (s *SQLStore) UpdatePersonPath(ctx context.Context, principal Principal, employeeID, pathID string, input PathAssignmentUpdateInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	employeeID = strings.TrimSpace(employeeID)
	pathID = strings.TrimSpace(pathID)
	if employeeID == "" || pathID == "" {
		return ActionResponse{}, validationError("missing_id", "id persona e id percorso obbligatori")
	}
	startedOn, err := parseOptionalDate(input.StartedOn)
	if err != nil || startedOn == nil {
		return ActionResponse{}, validationError("invalid_started_on", "data inizio non valida")
	}
	targetCompletion, err := parseOptionalDate(input.TargetCompletion)
	if err != nil {
		return ActionResponse{}, validationError("invalid_target_completion", "data obiettivo non valida")
	}
	completedOn, err := parseOptionalDate(input.CompletedOn)
	if err != nil {
		return ActionResponse{}, validationError("invalid_completed_on", "data conclusione non valida")
	}
	if targetCompletion != nil && targetCompletion.Before(*startedOn) {
		return ActionResponse{}, validationError("target_completion_before_start", "l'obiettivo non puo precedere l'inizio")
	}
	if completedOn != nil && completedOn.Before(*startedOn) {
		return ActionResponse{}, validationError("completed_on_before_start", "la conclusione non puo precedere l'inizio")
	}

	response := ActionResponse{OK: true, ID: pathID}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		before, err := s.assignmentSnapshot(ctx, tx, employeeID, pathID)
		if err != nil {
			return err
		}
		if before == nil {
			return notFoundError("assignment_not_found", "assegnazione non trovata")
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE training.employee_learning_path
SET started_on = $3::date, target_completion = NULLIF($4, '')::date, completed_on = NULLIF($5, '')::date, notes = NULLIF($6, '')
WHERE employee_id = $1::uuid AND path_id = $2::uuid`,
			employeeID, pathID, strings.TrimSpace(input.StartedOn), strings.TrimSpace(input.TargetCompletion), strings.TrimSpace(input.CompletedOn), input.Notes,
		); err != nil {
			return fmt.Errorf("update training learning path assignment: %w", err)
		}
		after, err := s.assignmentSnapshot(ctx, tx, employeeID, pathID)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, principal, "employee_learning_path", employeeID, "update", before, after)
	})
	return response, err
}

// RemovePersonPath rimuove l'assegnazione, auditata.
func (s *SQLStore) RemovePersonPath(ctx context.Context, principal Principal, employeeID, pathID string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	employeeID = strings.TrimSpace(employeeID)
	pathID = strings.TrimSpace(pathID)
	if employeeID == "" || pathID == "" {
		return ActionResponse{}, validationError("missing_id", "id persona e id percorso obbligatori")
	}
	response := ActionResponse{OK: true, ID: pathID}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		before, err := s.assignmentSnapshot(ctx, tx, employeeID, pathID)
		if err != nil {
			return err
		}
		if before == nil {
			return notFoundError("assignment_not_found", "assegnazione non trovata")
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM training.employee_learning_path WHERE employee_id = $1::uuid AND path_id = $2::uuid`, employeeID, pathID); err != nil {
			return fmt.Errorf("delete training learning path assignment: %w", err)
		}
		return s.audit(ctx, tx, principal, "employee_learning_path", employeeID, "remove", before, nil)
	})
	return response, err
}

func (s *SQLStore) assignmentSnapshot(ctx context.Context, q sqlRunner, employeeID, pathID string) (json.RawMessage, error) {
	var raw string
	err := q.QueryRowContext(ctx, `
SELECT json_build_object(
  'employeeId', employee_id, 'pathId', path_id, 'startedOn', started_on,
  'targetCompletion', target_completion, 'completedOn', completed_on, 'notes', notes
)::text
FROM training.employee_learning_path
WHERE employee_id = $1::uuid AND path_id = $2::uuid`, employeeID, pathID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("snapshot training learning path assignment: %w", err)
	}
	return json.RawMessage(raw), nil
}
