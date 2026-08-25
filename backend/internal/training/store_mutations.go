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
	DeliveryStatus   string
	HasCertification bool
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

func (s *SQLStore) auditFields(ctx context.Context, q sqlRunner, principal Principal, entityType, entityID, action string, fields []string) error {
	payload, err := json.Marshal(map[string]any{"changed_fields": fields})
	if err != nil {
		return fmt.Errorf("marshal training audit fields: %w", err)
	}
	return s.audit(ctx, q, principal, entityType, entityID, action, nil, payload)
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

type normalizedPersonUpdate struct {
	FirstName string
	LastName  string
	Email     string
	Status    string
	TeamID    string
	Notes     string
}

type activeTeamMembership struct {
	ID     string
	TeamID string
}

func normalizePersonUpdateInput(input PersonUpdateInput) (normalizedPersonUpdate, error) {
	teamID := ""
	if input.TeamID != nil {
		teamID = strings.TrimSpace(*input.TeamID)
	}
	normalized := normalizedPersonUpdate{
		FirstName: strings.Join(strings.Fields(strings.TrimSpace(input.FirstName)), " "),
		LastName:  strings.Join(strings.Fields(strings.TrimSpace(input.LastName)), " "),
		Email:     normalizeEmail(input.Email),
		Status:    strings.TrimSpace(input.Status),
		TeamID:    teamID,
		Notes:     strings.TrimSpace(input.Notes),
	}
	if normalized.FirstName == "" || normalized.LastName == "" || normalized.Email == "" || normalized.Status == "" {
		return normalized, validationError("person_required_fields_missing", "nome, cognome, email e stato sono obbligatori")
	}
	if !validImportEmail(normalized.Email) {
		return normalized, validationError("person_email_invalid", "email persona non valida")
	}
	if !validPersonStatus(normalized.Status) {
		return normalized, validationError("person_status_invalid", "stato persona non supportato")
	}
	return normalized, nil
}

func normalizePersonCreateInput(input PersonCreateInput) (normalizedPersonUpdate, error) {
	return normalizePersonUpdateInput(PersonUpdateInput{
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Email:     input.Email,
		Status:    input.Status,
		TeamID:    input.TeamID,
		Notes:     input.Notes,
	})
}

func validPersonStatus(status string) bool {
	switch status {
	case "active", "on_leave", "terminated":
		return true
	default:
		return false
	}
}

func (s *SQLStore) ensurePersonEmailAvailable(ctx context.Context, q sqlRunner, employeeID string, email string) error {
	var duplicateID string
	err := q.QueryRowContext(ctx, `
SELECT id::text
FROM training.employee
WHERE email = $1
  AND id <> $2::uuid
LIMIT 1`, email, employeeID).Scan(&duplicateID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check training employee email: %w", err)
	}
	return validationError("person_email_duplicate", "email gia assegnata a un'altra persona")
}

func (s *SQLStore) ensureNewPersonEmailAvailable(ctx context.Context, q sqlRunner, email string) error {
	var duplicateID string
	err := q.QueryRowContext(ctx, `
SELECT id::text
FROM training.employee
WHERE email = $1
LIMIT 1`, email).Scan(&duplicateID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check training employee email: %w", err)
	}
	return validationError("person_email_duplicate", "email gia assegnata a un'altra persona")
}

func (s *SQLStore) ensureTeamSelectable(ctx context.Context, q sqlRunner, teamID string) error {
	if teamID == "" {
		return nil
	}
	var active bool
	err := q.QueryRowContext(ctx, `
SELECT is_active
FROM training.team
WHERE id = $1::uuid`, teamID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return validationError("team_not_found", "team non trovato")
	}
	if err != nil {
		return fmt.Errorf("check training team: %w", err)
	}
	if !active {
		return validationError("team_inactive", "team non attivo")
	}
	return nil
}

type directoryManagedPerson struct {
	// Managed: agganciata alla directory E non esente — i campi anagrafici
	// non si modificano a mano.
	Managed       bool
	HasExternalID bool
	Exempt        bool
	FirstName     string
	LastName      string
	Email         string
	Status        string
}

func (s *SQLStore) directoryManagedPersonState(ctx context.Context, q sqlRunner, employeeID string) (directoryManagedPerson, error) {
	var state directoryManagedPerson
	err := q.QueryRowContext(ctx, `
SELECT COALESCE(external_id, '') <> '', directory_exempt, first_name, last_name, email::text, status::text
FROM training.employee
WHERE id = $1::uuid`, employeeID).Scan(&state.HasExternalID, &state.Exempt, &state.FirstName, &state.LastName, &state.Email, &state.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return state, notFoundError("employee_not_found", "persona non trovata")
	}
	if err != nil {
		return state, fmt.Errorf("load training employee directory state: %w", err)
	}
	state.Managed = state.HasExternalID && !state.Exempt
	return state, nil
}

func (s *SQLStore) activeTeamMemberships(ctx context.Context, q sqlRunner, employeeID string) ([]activeTeamMembership, error) {
	rows, err := q.QueryContext(ctx, `
SELECT id::text, team_id::text
FROM training.team_membership
WHERE employee_id = $1::uuid
  AND start_date <= now()
  AND (end_date IS NULL OR end_date >= now())
ORDER BY start_date DESC, created_at DESC, id`, employeeID)
	if err != nil {
		return nil, fmt.Errorf("list active training team memberships: %w", err)
	}
	defer rows.Close()

	memberships := make([]activeTeamMembership, 0)
	for rows.Next() {
		var membership activeTeamMembership
		if err := rows.Scan(&membership.ID, &membership.TeamID); err != nil {
			return nil, fmt.Errorf("scan active training team membership: %w", err)
		}
		memberships = append(memberships, membership)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return memberships, nil
}

func membershipReplacementNeeded(active []activeTeamMembership, selectedTeamID string) bool {
	if selectedTeamID == "" {
		return len(active) > 0
	}
	return len(active) != 1 || active[0].TeamID != selectedTeamID
}

func membershipCoversTeam(active []activeTeamMembership, selectedTeamID string) bool {
	if selectedTeamID == "" {
		return len(active) == 0
	}
	for _, membership := range active {
		if membership.TeamID == selectedTeamID {
			return true
		}
	}
	return false
}

func (s *SQLStore) CreatePerson(ctx context.Context, principal Principal, input PersonCreateInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	normalized, err := normalizePersonCreateInput(input)
	if err != nil {
		return ActionResponse{}, err
	}

	var response ActionResponse
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.ensureNewPersonEmailAvailable(ctx, tx, normalized.Email); err != nil {
			return err
		}
		if err := s.ensureTeamSelectable(ctx, tx, normalized.TeamID); err != nil {
			return err
		}

		const insertEmployee = `
INSERT INTO training.employee (
  first_name,
  last_name,
  email,
  status,
  notes
) VALUES (
  $1,
  $2,
  $3,
  $4::training.employee_status,
  NULLIF($5, '')
)
RETURNING id::text, status::text`
		if err := tx.QueryRowContext(
			ctx,
			insertEmployee,
			normalized.FirstName,
			normalized.LastName,
			normalized.Email,
			normalized.Status,
			normalized.Notes,
		).Scan(&response.ID, &response.Status); err != nil {
			if isUniqueViolation(err, "") {
				return validationError("person_email_duplicate", "email gia assegnata a un'altra persona")
			}
			return fmt.Errorf("create training employee: %w", err)
		}

		afterEmployee, err := entitySnapshot(ctx, tx, "employee", response.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "employee", response.ID, "create", nil, afterEmployee); err != nil {
			return err
		}

		if normalized.TeamID != "" {
			var membershipID string
			if err := tx.QueryRowContext(ctx, `
INSERT INTO training.team_membership (employee_id, team_id, start_date)
VALUES ($1::uuid, $2::uuid, now())
RETURNING id::text`, response.ID, normalized.TeamID).Scan(&membershipID); err != nil {
				return fmt.Errorf("create training team membership: %w", err)
			}
			afterMembership, err := entitySnapshot(ctx, tx, "team_membership", membershipID)
			if err != nil {
				return err
			}
			if err := s.audit(ctx, tx, principal, "team_membership", membershipID, "create", nil, afterMembership); err != nil {
				return err
			}
		}

		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) UpdatePerson(ctx context.Context, principal Principal, employeeID string, input PersonUpdateInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	employeeID = strings.TrimSpace(employeeID)
	if employeeID == "" {
		return ActionResponse{}, validationError("missing_id", "id persona obbligatorio")
	}
	normalized, err := normalizePersonUpdateInput(input)
	if err != nil {
		return ActionResponse{}, err
	}

	var response ActionResponse
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		beforeEmployee, err := entitySnapshot(ctx, tx, "employee", employeeID)
		if appErr, ok := asAppError(err); ok && appErr.code == "entity_not_found" {
			return notFoundError("employee_not_found", "persona non trovata")
		}
		if err != nil {
			return err
		}
		if err := s.ensurePersonEmailAvailable(ctx, tx, employeeID, normalized.Email); err != nil {
			return err
		}
		if err := s.ensureTeamSelectable(ctx, tx, normalized.TeamID); err != nil {
			return err
		}

		activeMemberships, err := s.activeTeamMemberships(ctx, tx, employeeID)
		if err != nil {
			return err
		}
		replaceMembership := membershipReplacementNeeded(activeMemberships, normalized.TeamID)

		managed, err := s.directoryManagedPersonState(ctx, tx, employeeID)
		if err != nil {
			return err
		}
		exempt := managed.Exempt
		if input.DirectoryExempt != nil {
			exempt = *input.DirectoryExempt
		}
		if managed.HasExternalID && !exempt {
			if normalized.FirstName != managed.FirstName ||
				normalized.LastName != managed.LastName ||
				normalized.Email != managed.Email ||
				normalized.Status != managed.Status {
				return validationError(
					"person_managed_by_directory",
					"nome, email e stato provengono dalla directory esterna: qui si modificano solo le note",
				)
			}
			if input.TeamID != nil && !membershipCoversTeam(activeMemberships, normalized.TeamID) {
				return validationError(
					"person_managed_by_directory",
					"i team provengono dalla directory esterna",
				)
			}
			replaceMembership = false
		}

		const updateEmployee = `
UPDATE training.employee
SET first_name = $2,
    last_name = $3,
    email = $4,
    status = $5::training.employee_status,
    notes = NULLIF($6, ''),
    directory_exempt = $7,
    updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, status::text`
		if err := tx.QueryRowContext(
			ctx,
			updateEmployee,
			employeeID,
			normalized.FirstName,
			normalized.LastName,
			normalized.Email,
			normalized.Status,
			normalized.Notes,
			exempt,
		).Scan(&response.ID, &response.Status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return notFoundError("employee_not_found", "persona non trovata")
			}
			if isUniqueViolation(err, "") {
				return validationError("person_email_duplicate", "email gia assegnata a un'altra persona")
			}
			return fmt.Errorf("update training employee: %w", err)
		}
		afterEmployee, err := entitySnapshot(ctx, tx, "employee", employeeID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "employee", employeeID, "update", beforeEmployee, afterEmployee); err != nil {
			return err
		}

		if replaceMembership {
			for _, membership := range activeMemberships {
				beforeMembership, err := entitySnapshot(ctx, tx, "team_membership", membership.ID)
				if err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx, `
UPDATE training.team_membership
SET end_date = now()
WHERE id = $1::uuid`, membership.ID); err != nil {
					return fmt.Errorf("close training team membership: %w", err)
				}
				afterMembership, err := entitySnapshot(ctx, tx, "team_membership", membership.ID)
				if err != nil {
					return err
				}
				if err := s.audit(ctx, tx, principal, "team_membership", membership.ID, "close", beforeMembership, afterMembership); err != nil {
					return err
				}
			}
			if normalized.TeamID != "" {
				var membershipID string
				if err := tx.QueryRowContext(ctx, `
INSERT INTO training.team_membership (employee_id, team_id, start_date)
VALUES ($1::uuid, $2::uuid, now())
RETURNING id::text`, employeeID, normalized.TeamID).Scan(&membershipID); err != nil {
					return fmt.Errorf("create training team membership: %w", err)
				}
				afterMembership, err := entitySnapshot(ctx, tx, "team_membership", membershipID)
				if err != nil {
					return err
				}
				if err := s.audit(ctx, tx, principal, "team_membership", membershipID, "create", nil, afterMembership); err != nil {
					return err
				}
			}
		}
		response.OK = true
		return nil
	})
	return response, err
}

// enrollmentGuard carica l'iscrizione con corso ed erogazione via evento:
// serve ad award e documenti di iscrizione.
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
  en.delivery_status,
  c.leads_to_cert_id IS NOT NULL
FROM training.enrollment en
JOIN training.employee e ON e.id = en.employee_id
JOIN training.training_event ev ON ev.id = en.event_id
JOIN training.course c ON c.id = ev.course_id
WHERE en.id = $1::uuid` + lockClause
	var guard enrollmentGuard
	err := q.QueryRowContext(ctx, query, id).Scan(
		&guard.ID,
		&guard.EmployeeID,
		&guard.EmployeeEmail,
		&guard.CourseTitle,
		&guard.DeliveryStatus,
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
	if err := s.ensureTeamNotManaged(ctx, id, input); err != nil {
		return ActionResponse{}, err
	}
	return s.upsertSimple(ctx, principal, "team", id, []upsertField{
		field("code", strings.TrimSpace(input.Code)),
		field("name", strings.TrimSpace(input.Name)),
		field("description", nullableText(input.Description)),
		field("is_active", boolValue(input.Active, true)),
	})
}

func (s *SQLStore) ensureTeamNotManaged(ctx context.Context, id string, input TeamInput) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	var (
		managed bool
		code    string
		name    string
	)
	err := s.db.QueryRowContext(ctx, `
SELECT COALESCE(external_id, '') <> '', code, name
FROM training.team
WHERE id = $1::uuid`, id).Scan(&managed, &code, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundError("team_not_found", "team non trovato")
	}
	if err != nil {
		return fmt.Errorf("load training team directory state: %w", err)
	}
	if !managed {
		return nil
	}
	if strings.TrimSpace(input.Code) != code || strings.TrimSpace(input.Name) != name {
		return validationError(
			"team_managed_by_directory",
			"codice e nome del team provengono dalla directory esterna",
		)
	}
	return nil
}

func (s *SQLStore) UpsertSkillArea(ctx context.Context, principal Principal, id string, input SkillAreaInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.Name) == "" {
		return ActionResponse{}, validationError("code_name_required", "codice e nome obbligatori")
	}
	if err := s.ensureCustomGroupSelectable(ctx, s.db, input.CustomGroupID); err != nil {
		return ActionResponse{}, err
	}
	return s.upsertSimple(ctx, principal, "skill_area", id, []upsertField{
		field("code", strings.TrimSpace(input.Code)),
		field("name", strings.TrimSpace(input.Name)),
		typedField("parent_id", nullableUUID(input.ParentID), "::uuid"),
		typedField("custom_group_id", nullableUUID(input.CustomGroupID), "::uuid"),
		field("description", nullableText(input.Description)),
		field("is_active", boolValue(input.Active, true)),
	})
}

// ensureCustomGroupSelectable: il gruppo locale collegato a un'area di
// competenza deve esistere ed essere attivo quando valorizzato
// (collegamento area -> gruppo della migrazione 131).
func (s *SQLStore) ensureCustomGroupSelectable(ctx context.Context, q sqlRunner, groupID string) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil
	}
	var active bool
	err := q.QueryRowContext(ctx, `
SELECT is_active
FROM training.custom_groups
WHERE id = $1::uuid`, groupID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return validationError("custom_group_not_found", "gruppo locale non trovato")
	}
	if err != nil {
		return fmt.Errorf("check training custom group: %w", err)
	}
	if !active {
		return validationError("custom_group_inactive", "gruppo locale non attivo")
	}
	return nil
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
	if providerKind != "internal" && providerKind != "external" {
		return ActionResponse{}, validationError("provider_kind_invalid", "erogazione non valida")
	}
	if providerKind == "external" && strings.TrimSpace(input.VendorID) == "" {
		return ActionResponse{}, validationError("vendor_required", "fornitore obbligatorio per corsi a erogazione esterna")
	}
	complianceRelated := input.ComplianceRelated || input.Mandatory
	complianceFramework := strings.TrimSpace(input.ComplianceFramework)
	if complianceRelated && complianceFramework == "" {
		return ActionResponse{}, validationError("compliance_framework_required", "framework compliance obbligatorio per corsi compliance")
	}
	if !complianceRelated {
		complianceFramework = ""
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
		field("is_compliance_course", complianceRelated),
		field("compliance_framework", nullableText(complianceFramework)),
		field("is_active", boolValue(input.Active, true)),
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
