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

type sqlRunner interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type enrollmentGuard struct {
	ID               string
	EmployeeID       string
	EmployeeEmail    string
	CourseTitle      string
	Status           string
	PlanStatus       string
	HasCertification bool
}

type requestGuard struct {
	ID            string
	EmployeeID    string
	EmployeeEmail string
	CourseID      string
	FreeTextTitle string
	Status        string
}

type awardGuard struct {
	ID            string
	EmployeeID    string
	EmployeeEmail string
	Outcome       string
}

type documentGuard struct {
	DocumentMetadata
	StorageKey    string
	EmployeeEmail string
}

type upsertField struct {
	column string
	value  any
	cast   string
}

func field(column string, value any) upsertField {
	return upsertField{column: column, value: value}
}

func typedField(column string, value any, cast string) upsertField {
	return upsertField{column: column, value: value, cast: cast}
}

func (s *SQLStore) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	if s == nil || s.db == nil {
		return errors.New("training database not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin training transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			return fmt.Errorf("%w; rollback training transaction: %v", err, rbErr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit training transaction: %w", err)
	}
	return nil
}

func (s *SQLStore) employeeIDByEmail(ctx context.Context, q sqlRunner, email string) (string, error) {
	var id string
	err := q.QueryRowContext(ctx, `SELECT id::text FROM training.employee WHERE email = $1 LIMIT 1`, normalizeEmail(email)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", notFoundError("employee_not_found", "profilo HR non trovato")
	}
	if err != nil {
		return "", fmt.Errorf("load training employee id: %w", err)
	}
	return id, nil
}

func (s *SQLStore) actorEmployeeID(ctx context.Context, q sqlRunner, principal Principal) *string {
	id, err := s.employeeIDByEmail(ctx, q, principal.Email)
	if err != nil {
		return nil
	}
	return &id
}

func (s *SQLStore) audit(ctx context.Context, q sqlRunner, principal Principal, entityType, entityID, action string, before, after json.RawMessage) error {
	actorID := s.actorEmployeeID(ctx, q, principal)
	const stmt = `
INSERT INTO training.audit_log (
  actor_id,
  entity_type,
  entity_id,
  action,
  before_state,
  after_state,
  correlation_id
) VALUES ($1::uuid, $2, $3::uuid, $4, $5::jsonb, $6::jsonb, gen_random_uuid())`
	_, err := q.ExecContext(ctx, stmt, nullableUUIDPtr(actorID), entityType, entityID, action, jsonOrNull(before), jsonOrNull(after))
	if err != nil {
		return fmt.Errorf("insert training audit log: %w", err)
	}
	return nil
}

func entitySnapshot(ctx context.Context, q sqlRunner, table string, id string) (json.RawMessage, error) {
	query := fmt.Sprintf(`SELECT to_jsonb(row) FROM (SELECT * FROM training.%s WHERE id = $1::uuid) row`, table)
	var raw []byte
	err := q.QueryRowContext(ctx, query, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFoundError("entity_not_found", "elemento non trovato")
	}
	if err != nil {
		return nil, fmt.Errorf("load training %s snapshot: %w", table, err)
	}
	return json.RawMessage(raw), nil
}

func jsonOrNull(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}

func nullableUUID(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullableUUIDPtr(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return strings.TrimSpace(*value)
}

func nullableText(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func (s *SQLStore) CreateEnrollment(ctx context.Context, principal Principal, input EnrollmentInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if strings.TrimSpace(input.EmployeeID) == "" || strings.TrimSpace(input.CourseID) == "" || strings.TrimSpace(input.TrainingPlanID) == "" {
			return validationError("missing_required_fields", "dipendente, corso e piano sono obbligatori")
		}
		const stmt = `
INSERT INTO training.enrollment (
  employee_id,
  course_id,
  training_plan_id,
  priority,
  level_as_is,
  level_to_be,
  planned_start,
  planned_end,
  hours_planned,
  cost_planned,
  course_title_snapshot,
  vendor_name_snapshot,
  motivation,
  objective,
  notes
)
SELECT
  $1::uuid,
  c.id,
  $3::uuid,
  $4,
  $5,
  $6,
  NULLIF($7, '')::date,
  NULLIF($8, '')::date,
  $9,
  $10,
  c.title,
  v.name,
  NULLIF($11, ''),
  NULLIF($12, ''),
  NULLIF($13, '')
FROM training.course c
LEFT JOIN training.vendor v ON v.id = c.vendor_id
WHERE c.id = $2::uuid
RETURNING id::text, status::text`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			input.EmployeeID,
			input.CourseID,
			input.TrainingPlanID,
			input.Priority,
			input.LevelAsIs,
			input.LevelToBe,
			strings.TrimSpace(input.PlannedStart),
			strings.TrimSpace(input.PlannedEnd),
			input.HoursPlanned,
			input.CostPlanned,
			input.Motivation,
			input.Objective,
			input.Notes,
		).Scan(&response.ID, &response.Status); err != nil {
			return fmt.Errorf("create training enrollment: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "enrollment", response.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "enrollment", response.ID, "create", nil, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) UpdateEnrollment(ctx context.Context, principal Principal, id string, input EnrollmentInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		before, err := entitySnapshot(ctx, tx, "enrollment", id)
		if err != nil {
			return err
		}
		const stmt = `
UPDATE training.enrollment
SET
  priority = $2,
  level_as_is = $3,
  level_to_be = $4,
  planned_start = NULLIF($5, '')::date,
  planned_end = NULLIF($6, '')::date,
  hours_planned = $7,
  cost_planned = $8,
  motivation = NULLIF($9, ''),
  objective = NULLIF($10, ''),
  notes = NULLIF($11, '')
WHERE id = $1::uuid
RETURNING id::text, status::text`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			id,
			input.Priority,
			input.LevelAsIs,
			input.LevelToBe,
			strings.TrimSpace(input.PlannedStart),
			strings.TrimSpace(input.PlannedEnd),
			input.HoursPlanned,
			input.CostPlanned,
			input.Motivation,
			input.Objective,
			input.Notes,
		).Scan(&response.ID, &response.Status); err != nil {
			return fmt.Errorf("update training enrollment: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "enrollment", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "enrollment", id, "update", before, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) enrollmentGuard(ctx context.Context, q sqlRunner, principal Principal, id string, lock bool) (enrollmentGuard, error) {
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE OF en"
	}
	query := `
SELECT
  en.id::text,
  e.id::text,
  e.email::text,
  c.title,
  en.status::text,
  tp.status::text,
  c.leads_to_cert_id IS NOT NULL
FROM training.enrollment en
JOIN training.employee e ON e.id = en.employee_id
JOIN training.course c ON c.id = en.course_id
JOIN training.training_plan tp ON tp.id = en.training_plan_id
WHERE en.id = $1::uuid` + lockClause
	var guard enrollmentGuard
	err := q.QueryRowContext(ctx, query, id).Scan(
		&guard.ID,
		&guard.EmployeeID,
		&guard.EmployeeEmail,
		&guard.CourseTitle,
		&guard.Status,
		&guard.PlanStatus,
		&guard.HasCertification,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return enrollmentGuard{}, notFoundError("enrollment_not_found", "iscrizione non trovata")
	}
	if err != nil {
		return enrollmentGuard{}, fmt.Errorf("load training enrollment guard: %w", err)
	}
	if !principalCanAccessEmployee(principal, guard.EmployeeEmail) {
		return enrollmentGuard{}, forbiddenError("not_owner", "iscrizione non accessibile")
	}
	return guard, nil
}

func (s *SQLStore) TransitionEnrollment(ctx context.Context, principal Principal, id string, input EnrollmentTransitionInput) (ActionResponse, error) {
	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		guard, err := s.enrollmentGuard(ctx, tx, principal, id, true)
		if err != nil {
			return err
		}
		before, err := entitySnapshot(ctx, tx, "enrollment", id)
		if err != nil {
			return err
		}

		actor := actorForPrincipal(principal)
		actualStart, err := parseOptionalDate(input.ActualStart)
		if err != nil {
			return validationError("invalid_actual_start", "data inizio non valida")
		}
		result := AttemptEnrollmentTransition(
			EnrollmentState(guard.Status),
			EnrollmentTransition(strings.TrimSpace(input.Transition)),
			TransitionContext{
				Actor:                  actor,
				Reason:                 input.Reason,
				PlanStatus:             guard.PlanStatus,
				ActualStart:            actualStart,
				HasLinkedCertification: &guard.HasCertification,
			},
		)
		if !result.OK {
			return conflictError(result.Code, result.Message)
		}

		status := result.Target
		if status == "" {
			return validationError("invalid_transition", "transizione non valida")
		}
		if err := s.applyEnrollmentTransition(ctx, tx, id, status, input); err != nil {
			return err
		}
		if status == string(EnrollmentCompleted) || status == string(EnrollmentFailed) {
			if err := s.createAwardFromEnrollmentOutcome(ctx, tx, id, status); err != nil {
				return err
			}
		}
		after, err := entitySnapshot(ctx, tx, "enrollment", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "enrollment", id, "transition:"+input.Transition, before, after); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id, Status: status}
		return nil
	})
	return response, err
}

func (s *SQLStore) applyEnrollmentTransition(ctx context.Context, tx *sql.Tx, id string, status string, input EnrollmentTransitionInput) error {
	switch EnrollmentState(status) {
	case EnrollmentInProgress:
		start := strings.TrimSpace(input.ActualStart)
		if start == "" {
			start = time.Now().Format("2006-01-02")
		}
		_, err := tx.ExecContext(ctx, `
UPDATE training.enrollment
SET status = $2::training.enrollment_status,
    actual_start = COALESCE(actual_start, $3::date)
WHERE id = $1::uuid`, id, status, start)
		if err != nil {
			return fmt.Errorf("start training enrollment: %w", err)
		}
	case EnrollmentCompleted, EnrollmentFailed:
		end := strings.TrimSpace(input.ActualEnd)
		if end == "" {
			end = time.Now().Format("2006-01-02")
		}
		_, err := tx.ExecContext(ctx, `
UPDATE training.enrollment
SET status = $2::training.enrollment_status,
    actual_end = COALESCE(actual_end, $3::date)
WHERE id = $1::uuid`, id, status, end)
		if err != nil {
			return fmt.Errorf("finish training enrollment: %w", err)
		}
	default:
		_, err := tx.ExecContext(ctx, `
UPDATE training.enrollment
SET status = $2::training.enrollment_status
WHERE id = $1::uuid`, id, status)
		if err != nil {
			return fmt.Errorf("transition training enrollment: %w", err)
		}
	}
	return nil
}

func (s *SQLStore) createAwardFromEnrollmentOutcome(ctx context.Context, tx *sql.Tx, enrollmentID string, status string) error {
	outcome := "passed_exam"
	if status == string(EnrollmentFailed) {
		outcome = "failed_exam"
	}
	const stmt = `
INSERT INTO training.certification_award (
  employee_id,
  certification_id,
  enrollment_id,
  outcome,
  awarded_on,
  expires_on,
  validation_source,
  notes
)
SELECT
  en.employee_id,
  c.leads_to_cert_id,
  en.id,
  $2::training.award_outcome,
  COALESCE(en.actual_end, CURRENT_DATE),
  CASE
    WHEN cert.typical_validity IS NULL THEN NULL
    ELSE COALESCE(en.actual_end, CURRENT_DATE) + cert.typical_validity
  END,
  'document_verified'::training.validation_source,
  'Generata dalla chiusura iscrizione'
FROM training.enrollment en
JOIN training.course c ON c.id = en.course_id
JOIN training.certification cert ON cert.id = c.leads_to_cert_id
WHERE en.id = $1::uuid
  AND c.leads_to_cert_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM training.certification_award ca WHERE ca.enrollment_id = en.id
  )`
	if _, err := tx.ExecContext(ctx, stmt, enrollmentID, outcome); err != nil {
		return fmt.Errorf("create training award from enrollment: %w", err)
	}
	return nil
}

func (s *SQLStore) CreateTrainingRequest(ctx context.Context, principal Principal, input TrainingRequestInput) (ActionResponse, error) {
	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		employeeID, err := s.employeeIDByEmail(ctx, tx, principal.Email)
		if err != nil {
			return err
		}
		if strings.TrimSpace(input.CourseID) == "" && strings.TrimSpace(input.FreeTextTitle) == "" {
			return validationError("course_or_title_required", "scegli un corso o indica un titolo")
		}
		if strings.TrimSpace(input.Motivation) == "" {
			return validationError("motivation_required", "motivazione obbligatoria")
		}
		const stmt = `
INSERT INTO training.training_request (
  employee_id,
  course_id,
  free_text_title,
  skill_area_id,
  motivation,
  desired_year
) VALUES ($1::uuid, $2::uuid, NULLIF($3, ''), $4::uuid, $5, $6)
RETURNING id::text, status`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			employeeID,
			nullableUUID(input.CourseID),
			strings.TrimSpace(input.FreeTextTitle),
			nullableUUID(input.SkillAreaID),
			strings.TrimSpace(input.Motivation),
			input.DesiredYear,
		).Scan(&response.ID, &response.Status); err != nil {
			return fmt.Errorf("create training request: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "training_request", response.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_request", response.ID, "create", nil, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) requestGuard(ctx context.Context, q sqlRunner, principal Principal, id string, lock bool) (requestGuard, error) {
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE OF tr"
	}
	query := `
SELECT
  tr.id::text,
  e.id::text,
  e.email::text,
  COALESCE(tr.course_id::text, ''),
  COALESCE(tr.free_text_title, ''),
  tr.status
FROM training.training_request tr
JOIN training.employee e ON e.id = tr.employee_id
WHERE tr.id = $1::uuid` + lockClause
	var guard requestGuard
	err := q.QueryRowContext(ctx, query, id).Scan(
		&guard.ID,
		&guard.EmployeeID,
		&guard.EmployeeEmail,
		&guard.CourseID,
		&guard.FreeTextTitle,
		&guard.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return requestGuard{}, notFoundError("request_not_found", "richiesta non trovata")
	}
	if err != nil {
		return requestGuard{}, fmt.Errorf("load training request guard: %w", err)
	}
	if !principalCanAccessEmployee(principal, guard.EmployeeEmail) {
		return requestGuard{}, forbiddenError("not_owner", "richiesta non accessibile")
	}
	return guard, nil
}

func (s *SQLStore) TransitionTrainingRequest(ctx context.Context, principal Principal, id string, input TrainingRequestTransitionInput) (ActionResponse, error) {
	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		guard, err := s.requestGuard(ctx, tx, principal, id, true)
		if err != nil {
			return err
		}
		before, err := entitySnapshot(ctx, tx, "training_request", id)
		if err != nil {
			return err
		}
		actor := actorForPrincipal(principal)
		openable := false
		if strings.TrimSpace(input.TrainingPlanID) != "" {
			openable, err = s.planOpenable(ctx, tx, input.TrainingPlanID)
			if err != nil {
				return err
			}
		}
		result := AttemptRequestTransition(
			RequestState(guard.Status),
			RequestTransition(strings.TrimSpace(input.Transition)),
			TransitionContext{
				Actor:                actor,
				Reason:               input.Reason,
				TargetPlanIsOpenable: openable,
			},
		)
		if !result.OK {
			return conflictError(result.Code, result.Message)
		}
		status := result.Target
		var enrollmentID string
		if RequestTransition(input.Transition) == RequestConvert {
			if guard.CourseID == "" {
				return validationError("course_required_for_convert", "serve un corso catalogo per convertire la richiesta")
			}
			const insertEnrollment = `
INSERT INTO training.enrollment (
  employee_id,
  course_id,
  training_plan_id,
  status,
  course_title_snapshot,
  vendor_name_snapshot,
  motivation
)
SELECT
  tr.employee_id,
  c.id,
  $2::uuid,
  'proposed'::training.enrollment_status,
  c.title,
  v.name,
  tr.motivation
FROM training.training_request tr
JOIN training.course c ON c.id = tr.course_id
LEFT JOIN training.vendor v ON v.id = c.vendor_id
WHERE tr.id = $1::uuid
RETURNING id::text`
			if err := tx.QueryRowContext(ctx, insertEnrollment, id, input.TrainingPlanID).Scan(&enrollmentID); err != nil {
				return fmt.Errorf("convert training request to enrollment: %w", err)
			}
		}
		const updateRequest = `
UPDATE training.training_request
SET status = $2,
    converted_to_enrollment_id = COALESCE(NULLIF($3, '')::uuid, converted_to_enrollment_id),
    review_notes = COALESCE(NULLIF($4, ''), review_notes),
    reviewed_by = COALESCE($5::uuid, reviewed_by),
    reviewed_at = CASE WHEN $6 THEN now() ELSE reviewed_at END
WHERE id = $1::uuid
RETURNING id::text, status`
		reviewedBy := s.actorEmployeeID(ctx, tx, principal)
		reviewed := principal.IsPeopleAdmin
		if err := tx.QueryRowContext(ctx, updateRequest, id, status, enrollmentID, input.Reason, nullableUUIDPtr(reviewedBy), reviewed).Scan(&response.ID, &response.Status); err != nil {
			return fmt.Errorf("transition training request: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "training_request", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "training_request", id, "transition:"+input.Transition, before, after); err != nil {
			return err
		}
		if enrollmentID != "" {
			enrollmentAfter, err := entitySnapshot(ctx, tx, "enrollment", enrollmentID)
			if err != nil {
				return err
			}
			if err := s.audit(ctx, tx, principal, "enrollment", enrollmentID, "create_from_request", nil, enrollmentAfter); err != nil {
				return err
			}
		}
		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) planOpenable(ctx context.Context, q sqlRunner, id string) (bool, error) {
	var status string
	err := q.QueryRowContext(ctx, `SELECT status::text FROM training.training_plan WHERE id = $1::uuid`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, notFoundError("plan_not_found", "piano non trovato")
	}
	if err != nil {
		return false, fmt.Errorf("load training plan status: %w", err)
	}
	return status == "draft" || status == "open", nil
}

func (s *SQLStore) UpsertVendor(ctx context.Context, principal Principal, id string, input VendorInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(input.Name) == "" {
		return ActionResponse{}, validationError("name_required", "nome obbligatorio")
	}
	active := boolValue(input.Active, true)
	name := strings.TrimSpace(input.Name)
	return s.upsertSimple(ctx, principal, "vendor", id, []upsertField{
		field("name", name),
		field("name_normalized", strings.ToLower(name)),
		field("website", nullableText(input.Website)),
		field("notes", nullableText(input.Notes)),
		field("is_active", active),
	})
}

func (s *SQLStore) UpsertTeam(ctx context.Context, principal Principal, id string, input TeamInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.Name) == "" {
		return ActionResponse{}, validationError("code_name_required", "codice e nome obbligatori")
	}
	return s.upsertSimple(ctx, principal, "team", id, []upsertField{
		field("code", strings.TrimSpace(input.Code)),
		field("name", strings.TrimSpace(input.Name)),
		field("description", nullableText(input.Description)),
		field("is_active", boolValue(input.Active, true)),
	})
}

func (s *SQLStore) UpsertSkillArea(ctx context.Context, principal Principal, id string, input SkillAreaInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.Name) == "" {
		return ActionResponse{}, validationError("code_name_required", "codice e nome obbligatori")
	}
	return s.upsertSimple(ctx, principal, "skill_area", id, []upsertField{
		field("code", strings.TrimSpace(input.Code)),
		field("name", strings.TrimSpace(input.Name)),
		typedField("parent_id", nullableUUID(input.ParentID), "::uuid"),
		field("description", nullableText(input.Description)),
		field("is_active", boolValue(input.Active, true)),
	})
}

func (s *SQLStore) UpsertCertification(ctx context.Context, principal Principal, id string, input CertificationInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.Name) == "" {
		return ActionResponse{}, validationError("code_name_required", "codice e nome obbligatori")
	}
	return s.upsertSimple(ctx, principal, "certification", id, []upsertField{
		field("code", strings.TrimSpace(input.Code)),
		field("name", strings.TrimSpace(input.Name)),
		typedField("issuer_vendor_id", nullableUUID(input.IssuerVendorID), "::uuid"),
		typedField("skill_area_id", nullableUUID(input.SkillAreaID), "::uuid"),
		typedField("typical_validity", monthsInterval(input.TypicalValidityMonths), "::interval"),
		field("description", nullableText(input.Description)),
		field("is_active", boolValue(input.Active, true)),
	})
}

func (s *SQLStore) UpsertCourse(ctx context.Context, principal Principal, id string, input CourseInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(input.Title) == "" {
		return ActionResponse{}, validationError("title_required", "titolo obbligatorio")
	}
	deliveryMode := strings.TrimSpace(input.DeliveryMode)
	if deliveryMode == "" {
		deliveryMode = "mixed"
	}
	providerKind := strings.TrimSpace(input.ProviderKind)
	if providerKind == "" {
		providerKind = "external"
	}
	return s.upsertSimple(ctx, principal, "course", id, []upsertField{
		field("title", strings.TrimSpace(input.Title)),
		typedField("vendor_id", nullableUUID(input.VendorID), "::uuid"),
		typedField("skill_area_id", nullableUUID(input.SkillAreaID), "::uuid"),
		typedField("leads_to_cert_id", nullableUUID(input.LeadsToCertID), "::uuid"),
		typedField("delivery_mode", deliveryMode, "::training.course_delivery_mode"),
		typedField("provider_kind", providerKind, "::training.course_provider_kind"),
		field("default_hours", input.DefaultHours),
		field("default_cost", input.DefaultCost),
		field("course_url", nullableText(input.CourseURL)),
		field("description", nullableText(input.Description)),
		field("is_mandatory", input.Mandatory),
		typedField("recurrence_interval", monthsInterval(input.RecurrenceMonths), "::interval"),
		field("compliance_framework", nullableText(input.ComplianceFramework)),
		field("is_active", boolValue(input.Active, true)),
	})
}

func (s *SQLStore) UpsertTrainingPlan(ctx context.Context, principal Principal, id string, input TrainingPlanInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if input.Year < 2020 || input.Year > 2100 {
		return ActionResponse{}, validationError("invalid_year", "anno non valido")
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "draft"
	}
	return s.upsertSimple(ctx, principal, "training_plan", id, []upsertField{
		field("year", input.Year),
		typedField("status", status, "::training.plan_status"),
		field("budget_total", input.BudgetTotal),
		field("notes", nullableText(input.Notes)),
	})
}

func (s *SQLStore) UpsertMandatoryRule(ctx context.Context, principal Principal, id string, input MandatoryRuleInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(input.CourseID) == "" {
		return ActionResponse{}, validationError("course_required", "corso obbligatorio")
	}
	return s.upsertSimple(ctx, principal, "mandatory_assignment_rule", id, []upsertField{
		typedField("course_id", nullableUUID(input.CourseID), "::uuid"),
		typedField("team_id", nullableUUID(input.TeamID), "::uuid"),
		field("role_filter", nullableText(input.RoleFilter)),
		field("is_active", boolValue(input.Active, true)),
		field("notes", nullableText(input.Notes)),
	})
}

func monthsInterval(months *int) any {
	if months == nil || *months <= 0 {
		return nil
	}
	return fmt.Sprintf("%d months", *months)
}

func (s *SQLStore) upsertSimple(ctx context.Context, principal Principal, table string, id string, fields []upsertField) (ActionResponse, error) {
	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		action := "create"
		var before json.RawMessage
		if strings.TrimSpace(id) != "" {
			action = "update"
			var err error
			before, err = entitySnapshot(ctx, tx, table, id)
			if err != nil {
				return err
			}
		}
		columns := make([]string, 0, len(fields))
		args := make([]any, 0, len(fields)+1)
		index := 1
		if strings.TrimSpace(id) == "" {
			placeholders := make([]string, 0, len(fields))
			for _, field := range fields {
				columns = append(columns, field.column)
				args = append(args, field.value)
				placeholders = append(placeholders, fmt.Sprintf("$%d%s", index, field.cast))
				index++
			}
			query := fmt.Sprintf(
				"INSERT INTO training.%s (%s) VALUES (%s) RETURNING id::text",
				table,
				strings.Join(columns, ", "),
				strings.Join(placeholders, ", "),
			)
			if err := tx.QueryRowContext(ctx, query, args...).Scan(&response.ID); err != nil {
				return fmt.Errorf("create training %s: %w", table, err)
			}
		} else {
			setters := make([]string, 0, len(fields))
			args = append(args, id)
			index = 2
			for _, field := range fields {
				args = append(args, field.value)
				setters = append(setters, fmt.Sprintf("%s = $%d%s", field.column, index, field.cast))
				index++
			}
			query := fmt.Sprintf(
				"UPDATE training.%s SET %s WHERE id = $1::uuid RETURNING id::text",
				table,
				strings.Join(setters, ", "),
			)
			if err := tx.QueryRowContext(ctx, query, args...).Scan(&response.ID); err != nil {
				return fmt.Errorf("update training %s: %w", table, err)
			}
		}
		after, err := entitySnapshot(ctx, tx, table, response.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, table, response.ID, action, before, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

func parseOptionalDate(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
