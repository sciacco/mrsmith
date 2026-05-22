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

type mandatoryRuleFilters struct {
	Status         string
	PopulationKind string
	Search         string
}

func normalizePopulationTarget(target PopulationTarget) (PopulationTarget, error) {
	kind := strings.ToLower(strings.TrimSpace(target.Kind))
	if kind == "" {
		kind = "all"
	}
	id := strings.TrimSpace(target.ID)
	normalized := PopulationTarget{Kind: kind, ID: id}
	switch kind {
	case "all":
		normalized.ID = ""
	case "team", "skill_area", "custom_group":
		if id == "" {
			return normalized, validationError("population_target_required", "popolazione obbligatoria")
		}
	default:
		return normalized, validationError("population_target_invalid", "popolazione non valida")
	}
	return normalized, nil
}

func (s *SQLStore) ensurePopulationTarget(ctx context.Context, q sqlRunner, target PopulationTarget) (PopulationTarget, error) {
	normalized, err := normalizePopulationTarget(target)
	if err != nil {
		return normalized, err
	}
	var label string
	switch normalized.Kind {
	case "all":
		normalized.Label = "Tutte le persone"
		return normalized, nil
	case "team":
		err = q.QueryRowContext(ctx, `
SELECT name
FROM training.team
WHERE id = $1::uuid`, normalized.ID).Scan(&label)
	case "skill_area":
		err = q.QueryRowContext(ctx, `
SELECT name
FROM training.skill_area
WHERE id = $1::uuid`, normalized.ID).Scan(&label)
	case "custom_group":
		err = q.QueryRowContext(ctx, `
SELECT name
FROM training.custom_groups
WHERE id = $1::uuid`, normalized.ID).Scan(&label)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return normalized, validationError("population_target_not_found", "popolazione non trovata")
	}
	if err != nil {
		return normalized, fmt.Errorf("check training population target: %w", err)
	}
	normalized.Label = label
	return normalized, nil
}

func populationTargetJSON(target PopulationTarget) ([]byte, error) {
	payload := map[string]string{"kind": target.Kind}
	if target.ID != "" {
		payload["id"] = target.ID
	}
	return json.Marshal(payload)
}

func scanPopulationTarget(raw []byte, label string, count int) (PopulationTarget, error) {
	var payload struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	}
	if len(raw) == 0 {
		payload.Kind = "all"
	} else if err := json.Unmarshal(raw, &payload); err != nil {
		return PopulationTarget{}, fmt.Errorf("decode population target: %w", err)
	}
	target := PopulationTarget{
		Kind:  strings.TrimSpace(payload.Kind),
		ID:    strings.TrimSpace(payload.ID),
		Label: strings.TrimSpace(label),
		Count: count,
	}
	if target.Kind == "" {
		target.Kind = "all"
	}
	if target.Label == "" && target.Kind == "all" {
		target.Label = "Tutte le persone"
	}
	return target, nil
}

func formatTimeUTC(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func (s *SQLStore) ListMandatoryRules(ctx context.Context, principal Principal, filters mandatoryRuleFilters) ([]MandatoryRule, error) {
	if !principal.IsPeopleAdmin {
		return nil, forbiddenError("people_role_required", "azione riservata a People")
	}
	status := strings.ToLower(strings.TrimSpace(filters.Status))
	kind := strings.ToLower(strings.TrimSpace(filters.PopulationKind))
	search := strings.TrimSpace(filters.Search)

	conds := []string{"1=1"}
	args := []any{}
	idx := 1
	switch status {
	case "attiva", "active":
		conds = append(conds, "rule.is_active")
	case "disattivata", "inactive":
		conds = append(conds, "NOT rule.is_active")
	case "":
	default:
		return nil, validationError("invalid_status", "stato regola non valido")
	}
	if kind != "" {
		switch kind {
		case "all", "team", "skill_area", "custom_group":
			conds = append(conds, fmt.Sprintf("rule.population_target->>'kind' = $%d", idx))
			args = append(args, kind)
			idx++
		default:
			return nil, validationError("population_target_invalid", "popolazione non valida")
		}
	}
	if search != "" {
		conds = append(conds, fmt.Sprintf("(rule.name ILIKE $%d OR course.title ILIKE $%d)", idx, idx))
		args = append(args, "%"+search+"%")
		idx++
	}

	query := fmt.Sprintf(ruleSelectQuery()+`
WHERE %s
ORDER BY rule.is_active DESC, rule.name, course.title
LIMIT 500`, strings.Join(conds, " AND "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list mandatory rules: %w", err)
	}
	defer rows.Close()
	return s.scanMandatoryRuleRows(ctx, rows, true)
}

func ruleSelectQuery() string {
	return `
SELECT
  rule.id::text,
  rule.name,
  rule.course_id::text,
  course.title,
  COALESCE(course.compliance_framework, ''),
  COALESCE(course.recurrence_interval::text, ''),
  rule.population_target,
  COALESCE(
    CASE rule.population_target->>'kind'
      WHEN 'all' THEN 'Tutte le persone'
      WHEN 'team' THEN team.name
      WHEN 'skill_area' THEN skill.name
      WHEN 'custom_group' THEN groups.name
      ELSE ''
    END,
    ''
  ) AS population_label,
  COALESCE(population.population_count, 0) AS population_count,
  COALESCE(rule.notes, ''),
  rule.is_active,
  rule.created_at,
  rule.updated_at
FROM training.mandatory_rules rule
JOIN training.course course ON course.id = rule.course_id
LEFT JOIN training.team team
  ON rule.population_target->>'kind' = 'team'
 AND team.id = (rule.population_target->>'id')::uuid
LEFT JOIN training.skill_area skill
  ON rule.population_target->>'kind' = 'skill_area'
 AND skill.id = (rule.population_target->>'id')::uuid
LEFT JOIN training.custom_groups groups
  ON rule.population_target->>'kind' = 'custom_group'
 AND groups.id = (rule.population_target->>'id')::uuid
LEFT JOIN (
  SELECT rule_id, COUNT(*) AS population_count
  FROM training.v_mandatory_rule_population
  GROUP BY rule_id
) population ON population.rule_id = rule.id
`
}

func (s *SQLStore) scanMandatoryRuleRows(ctx context.Context, rows *sql.Rows, includeUsage bool) ([]MandatoryRule, error) {
	out := make([]MandatoryRule, 0)
	for rows.Next() {
		rule, err := scanMandatoryRuleRow(rows)
		if err != nil {
			return nil, err
		}
		impact, err := s.MandatoryRuleImpact(ctx, rule.ID)
		if err != nil {
			return nil, err
		}
		applyImpactToRule(&rule, impact)
		if includeUsage {
			usage, err := s.mandatoryRuleUsedBy(ctx, rule.ID)
			if err != nil {
				return nil, err
			}
			rule.UsedBy = usage
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

type mandatoryRuleScanner interface {
	Scan(dest ...any) error
}

func scanMandatoryRuleRow(row mandatoryRuleScanner) (MandatoryRule, error) {
	var rule MandatoryRule
	var rawTarget []byte
	var populationLabel string
	var populationCount int
	var createdAt, updatedAt time.Time
	var cadence string
	if err := row.Scan(
		&rule.ID,
		&rule.Name,
		&rule.CourseID,
		&rule.CourseTitle,
		&rule.ComplianceFramework,
		&cadence,
		&rawTarget,
		&populationLabel,
		&populationCount,
		&rule.Notes,
		&rule.Active,
		&createdAt,
		&updatedAt,
	); err != nil {
		return MandatoryRule{}, fmt.Errorf("scan mandatory rule: %w", err)
	}
	target, err := scanPopulationTarget(rawTarget, populationLabel, populationCount)
	if err != nil {
		return MandatoryRule{}, err
	}
	rule.PopulationTarget = target
	rule.CadenceLabel = cadenceLabel(cadence)
	rule.CreatedAt = formatTimeUTC(createdAt)
	rule.UpdatedAt = formatTimeUTC(updatedAt)
	return rule, nil
}

func applyImpactToRule(rule *MandatoryRule, impact RuleImpact) {
	rule.TargetCount = impact.TargetCount
	rule.CoveredCount = impact.CoveredCount
	rule.GapCount = impact.GapCount
	rule.CoveragePct = impact.CoveragePct
	rule.Gaps = impact.Gaps
	rule.Severity = impact.Severity
}

func (s *SQLStore) MandatoryRuleByID(ctx context.Context, id string) (MandatoryRule, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return MandatoryRule{}, validationError("missing_rule_id", "id regola obbligatorio")
	}
	query := ruleSelectQuery() + `WHERE rule.id = $1::uuid`
	var rule MandatoryRule
	row := s.db.QueryRowContext(ctx, query, id)
	scanned, err := scanMandatoryRuleRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return MandatoryRule{}, notFoundError("rule_not_found", "regola non trovata")
	}
	if err != nil {
		return MandatoryRule{}, err
	}
	rule = scanned
	impact, err := s.MandatoryRuleImpact(ctx, rule.ID)
	if err != nil {
		return MandatoryRule{}, err
	}
	applyImpactToRule(&rule, impact)
	usage, err := s.mandatoryRuleUsedBy(ctx, rule.ID)
	if err != nil {
		return MandatoryRule{}, err
	}
	rule.UsedBy = usage
	return rule, nil
}

func (s *SQLStore) MandatoryRuleImpact(ctx context.Context, ruleID string) (RuleImpact, error) {
	var impact RuleImpact
	if strings.TrimSpace(ruleID) == "" {
		return impact, validationError("missing_rule_id", "id regola obbligatorio")
	}
	err := s.db.QueryRowContext(ctx, `
SELECT
  COUNT(*)::int AS target_count,
  COUNT(*) FILTER (WHERE compliance_status = 'compliant')::int AS covered_count,
  COUNT(*) FILTER (WHERE compliance_status <> 'compliant')::int AS gap_count
FROM training.v_mandatory_compliance_gap
WHERE rule_id = $1::uuid`, ruleID).Scan(&impact.TargetCount, &impact.CoveredCount, &impact.GapCount)
	if err != nil {
		return impact, fmt.Errorf("mandatory rule impact: %w", err)
	}
	if impact.TargetCount > 0 {
		impact.CoveragePct = float64(impact.CoveredCount) / float64(impact.TargetCount) * 100
	} else {
		impact.CoveragePct = 100
	}
	impact.Severity = complianceSeverity(impact.CoveragePct, impact.GapCount)

	rows, err := s.db.QueryContext(ctx, `
SELECT
  employee_id::text,
  last_name || ' ' || first_name AS employee_name,
  CASE
    WHEN COALESCE(last_valid_awarded_on::text, last_valid_completed_on::text, '') = '' THEN 'never_covered'
    ELSE 'expired'
  END AS gap_status,
  COALESCE(last_valid_awarded_on::text, last_valid_completed_on::text, '') AS detail
FROM training.v_mandatory_compliance_gap
WHERE rule_id = $1::uuid
  AND compliance_status <> 'compliant'
ORDER BY last_name, first_name
LIMIT 10`, ruleID)
	if err != nil {
		return impact, fmt.Errorf("mandatory rule gaps: %w", err)
	}
	defer rows.Close()
	impact.Gaps = []ComplianceRuleGap{}
	for rows.Next() {
		var gap ComplianceRuleGap
		if err := rows.Scan(&gap.EmployeeID, &gap.EmployeeName, &gap.Status, &gap.Detail); err != nil {
			return impact, fmt.Errorf("scan mandatory rule gap: %w", err)
		}
		impact.Gaps = append(impact.Gaps, gap)
	}
	return impact, rows.Err()
}

func (s *SQLStore) mandatoryRuleUsedBy(ctx context.Context, ruleID string) ([]RuleUsage, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)::int
FROM training.enrollment
WHERE mandatory_rule_id = $1::uuid`, ruleID).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("mandatory rule usage: %w", err)
	}
	if count == 0 {
		return []RuleUsage{}, nil
	}
	return []RuleUsage{{
		Kind:  "enrollment",
		Label: "Iscrizioni collegate",
		Count: count,
	}}, nil
}

func (s *SQLStore) CreateMandatoryRule(ctx context.Context, principal Principal, input MandatoryRuleInputV2) (MandatoryRuleMutationResponse, error) {
	if !principal.IsPeopleAdmin {
		return MandatoryRuleMutationResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return MandatoryRuleMutationResponse{}, validationError("name_required", "nome regola obbligatorio")
	}
	if strings.TrimSpace(input.CourseID) == "" {
		return MandatoryRuleMutationResponse{}, validationError("course_required", "corso obbligatorio")
	}
	active := boolValue(input.Active, true)
	var ruleID string
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		target, err := s.ensurePopulationTarget(ctx, tx, input.PopulationTarget)
		if err != nil {
			return err
		}
		targetJSON, err := populationTargetJSON(target)
		if err != nil {
			return fmt.Errorf("marshal population target: %w", err)
		}
		if err := s.ensureComplianceRuleCourse(ctx, tx, input.CourseID); err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `
INSERT INTO training.mandatory_rules (name, course_id, population_target, is_active, notes)
VALUES ($1, $2::uuid, $3::jsonb, $4, NULLIF($5, ''))
RETURNING id::text`, name, input.CourseID, targetJSON, active, strings.TrimSpace(input.Notes)).Scan(&ruleID)
		if err != nil {
			return fmt.Errorf("create mandatory rule: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "mandatory_rules", ruleID)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, principal, "mandatory_rules", ruleID, "create", nil, after)
	})
	if err != nil {
		return MandatoryRuleMutationResponse{}, err
	}
	return s.ruleMutationResponse(ctx, ruleID)
}

func (s *SQLStore) UpdateMandatoryRule(ctx context.Context, principal Principal, id string, input MandatoryRuleInputV2) (MandatoryRuleMutationResponse, error) {
	if !principal.IsPeopleAdmin {
		return MandatoryRuleMutationResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return MandatoryRuleMutationResponse{}, validationError("missing_rule_id", "id regola obbligatorio")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return MandatoryRuleMutationResponse{}, validationError("name_required", "nome regola obbligatorio")
	}
	if strings.TrimSpace(input.CourseID) == "" {
		return MandatoryRuleMutationResponse{}, validationError("course_required", "corso obbligatorio")
	}
	active := boolValue(input.Active, true)
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		before, err := entitySnapshot(ctx, tx, "mandatory_rules", id)
		if appErr, ok := asAppError(err); ok && appErr.code == "entity_not_found" {
			return notFoundError("rule_not_found", "regola non trovata")
		}
		if err != nil {
			return err
		}
		target, err := s.ensurePopulationTarget(ctx, tx, input.PopulationTarget)
		if err != nil {
			return err
		}
		targetJSON, err := populationTargetJSON(target)
		if err != nil {
			return fmt.Errorf("marshal population target: %w", err)
		}
		if err := s.ensureComplianceRuleCourse(ctx, tx, input.CourseID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
UPDATE training.mandatory_rules
SET name = $2,
    course_id = $3::uuid,
    population_target = $4::jsonb,
    is_active = $5,
    notes = NULLIF($6, ''),
    updated_at = now()
WHERE id = $1::uuid`, id, name, input.CourseID, targetJSON, active, strings.TrimSpace(input.Notes))
		if err != nil {
			return fmt.Errorf("update mandatory rule: %w", err)
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			return notFoundError("rule_not_found", "regola non trovata")
		}
		after, err := entitySnapshot(ctx, tx, "mandatory_rules", id)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, principal, "mandatory_rules", id, "update", before, after)
	})
	if err != nil {
		return MandatoryRuleMutationResponse{}, err
	}
	return s.ruleMutationResponse(ctx, id)
}

func (s *SQLStore) ruleMutationResponse(ctx context.Context, id string) (MandatoryRuleMutationResponse, error) {
	rule, err := s.MandatoryRuleByID(ctx, id)
	if err != nil {
		return MandatoryRuleMutationResponse{}, err
	}
	impact := RuleImpact{
		TargetCount:  rule.TargetCount,
		CoveredCount: rule.CoveredCount,
		GapCount:     rule.GapCount,
		CoveragePct:  rule.CoveragePct,
		Severity:     rule.Severity,
		Gaps:         rule.Gaps,
	}
	return MandatoryRuleMutationResponse{
		Rule:     rule,
		Warnings: mandatoryRuleWarnings(rule),
		Impact:   impact,
	}, nil
}

func mandatoryRuleWarnings(rule MandatoryRule) []string {
	warnings := []string{}
	if !rule.Active {
		return warnings
	}
	if rule.TargetCount == 0 {
		warnings = append(warnings, "empty_population")
	}
	if rule.GapCount > 0 {
		warnings = append(warnings, "coverage_gap")
	}
	return warnings
}

func (s *SQLStore) DeleteMandatoryRule(ctx context.Context, principal Principal, id string) error {
	if !principal.IsPeopleAdmin {
		return forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return validationError("missing_rule_id", "id regola obbligatorio")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var referenced int
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)::int
FROM training.enrollment
WHERE mandatory_rule_id = $1::uuid`, id).Scan(&referenced); err != nil {
			return fmt.Errorf("count rule enrollments: %w", err)
		}
		if referenced > 0 {
			return conflictError("rule_in_use", "regola usata in iscrizioni esistenti")
		}
		before, err := entitySnapshot(ctx, tx, "mandatory_rules", id)
		if appErr, ok := asAppError(err); ok && appErr.code == "entity_not_found" {
			return notFoundError("rule_not_found", "regola non trovata")
		}
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "mandatory_rules", id, "delete", before, nil); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM training.mandatory_rules WHERE id = $1::uuid`, id)
		if err != nil {
			return fmt.Errorf("delete mandatory rule: %w", err)
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			return notFoundError("rule_not_found", "regola non trovata")
		}
		return nil
	})
}

func (s *SQLStore) courseTitle(ctx context.Context, q sqlRunner, courseID string) (string, error) {
	var title string
	err := q.QueryRowContext(ctx, `
SELECT title
FROM training.course
WHERE id = $1::uuid`, strings.TrimSpace(courseID)).Scan(&title)
	if errors.Is(err, sql.ErrNoRows) {
		return "", validationError("course_not_found", "corso non trovato")
	}
	if err != nil {
		return "", fmt.Errorf("load training course: %w", err)
	}
	return title, nil
}

func (s *SQLStore) ensureComplianceRuleCourse(ctx context.Context, q sqlRunner, courseID string) error {
	var framework string
	var active, mandatory bool
	err := q.QueryRowContext(ctx, `
SELECT is_active, is_mandatory, COALESCE(compliance_framework, '')
FROM training.course
WHERE id = $1::uuid`, strings.TrimSpace(courseID)).Scan(&active, &mandatory, &framework)
	if errors.Is(err, sql.ErrNoRows) {
		return validationError("course_not_found", "corso non trovato")
	}
	if err != nil {
		return fmt.Errorf("load training course: %w", err)
	}
	if !active || !mandatory || strings.TrimSpace(framework) == "" {
		return validationError("course_not_compliance", "seleziona un corso compliance attivo")
	}
	return nil
}

func (s *SQLStore) ListCustomGroups(ctx context.Context, principal Principal, status, search string) ([]CustomGroup, error) {
	if !principal.IsPeopleAdmin {
		return nil, forbiddenError("people_role_required", "azione riservata a People")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	search = strings.TrimSpace(search)
	conds := []string{"1=1"}
	args := []any{}
	idx := 1
	switch status {
	case "attivo", "active":
		conds = append(conds, "groups.is_active")
	case "disattivato", "inactive":
		conds = append(conds, "NOT groups.is_active")
	case "":
	default:
		return nil, validationError("invalid_status", "stato gruppo non valido")
	}
	if search != "" {
		conds = append(conds, fmt.Sprintf("(groups.name ILIKE $%d OR COALESCE(groups.description, '') ILIKE $%d)", idx, idx))
		args = append(args, "%"+search+"%")
		idx++
	}
	query := fmt.Sprintf(groupSelectQuery()+`
WHERE %s
ORDER BY groups.is_active DESC, groups.name
LIMIT 500`, strings.Join(conds, " AND "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list custom groups: %w", err)
	}
	defer rows.Close()
	out := make([]CustomGroup, 0)
	for rows.Next() {
		group, err := scanCustomGroupRow(rows)
		if err != nil {
			return nil, err
		}
		members, err := s.customGroupMembers(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		group.Members = members
		usage, err := s.customGroupUsedBy(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		group.UsedBy = usage
		out = append(out, group)
	}
	return out, rows.Err()
}

func groupSelectQuery() string {
	return `
SELECT
  groups.id::text,
  groups.name,
  COALESCE(groups.description, ''),
  groups.is_active,
  COALESCE(member_counts.member_count, 0)::int,
  groups.created_at,
  groups.updated_at
FROM training.custom_groups groups
LEFT JOIN (
  SELECT group_id, COUNT(*) AS member_count
  FROM training.custom_group_members
  GROUP BY group_id
) member_counts ON member_counts.group_id = groups.id
`
}

type customGroupScanner interface {
	Scan(dest ...any) error
}

func scanCustomGroupRow(row customGroupScanner) (CustomGroup, error) {
	var group CustomGroup
	var createdAt, updatedAt time.Time
	if err := row.Scan(
		&group.ID,
		&group.Name,
		&group.Description,
		&group.Active,
		&group.MemberCount,
		&createdAt,
		&updatedAt,
	); err != nil {
		return CustomGroup{}, fmt.Errorf("scan custom group: %w", err)
	}
	group.CreatedAt = formatTimeUTC(createdAt)
	group.UpdatedAt = formatTimeUTC(updatedAt)
	return group, nil
}

func (s *SQLStore) CustomGroupByID(ctx context.Context, id string) (CustomGroup, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return CustomGroup{}, validationError("missing_group_id", "id gruppo obbligatorio")
	}
	group, err := scanCustomGroupRow(s.db.QueryRowContext(ctx, groupSelectQuery()+`WHERE groups.id = $1::uuid`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return CustomGroup{}, notFoundError("group_not_found", "gruppo non trovato")
	}
	if err != nil {
		return CustomGroup{}, err
	}
	members, err := s.customGroupMembers(ctx, group.ID)
	if err != nil {
		return CustomGroup{}, err
	}
	group.Members = members
	usage, err := s.customGroupUsedBy(ctx, group.ID)
	if err != nil {
		return CustomGroup{}, err
	}
	group.UsedBy = usage
	return group, nil
}

func (s *SQLStore) customGroupMembers(ctx context.Context, groupID string) ([]GroupMember, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT
  employee.id::text,
  employee.last_name || ' ' || employee.first_name AS name,
  employee.email::text,
  COALESCE(team.code, '') AS team_code,
  COALESCE(team.name, '') AS team_name
FROM training.custom_group_members member
JOIN training.employee employee ON employee.id = member.employee_id
LEFT JOIN training.team_membership tm
  ON tm.employee_id = employee.id
 AND tm.start_date <= now()
 AND (tm.end_date IS NULL OR tm.end_date >= now())
LEFT JOIN training.team team ON team.id = tm.team_id
WHERE member.group_id = $1::uuid
ORDER BY employee.last_name, employee.first_name`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list custom group members: %w", err)
	}
	defer rows.Close()
	members := []GroupMember{}
	for rows.Next() {
		var member GroupMember
		if err := rows.Scan(&member.ID, &member.Name, &member.Email, &member.TeamCode, &member.TeamName); err != nil {
			return nil, fmt.Errorf("scan custom group member: %w", err)
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *SQLStore) customGroupUsedBy(ctx context.Context, groupID string) ([]CustomGroupUsage, error) {
	usage := []CustomGroupUsage{}
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, name
FROM training.mandatory_rules
WHERE is_active
  AND population_target->>'kind' = 'custom_group'
  AND population_target->>'id' = $1
ORDER BY name`, groupID)
	if err != nil {
		return nil, fmt.Errorf("custom group rule usage: %w", err)
	}
	for rows.Next() {
		var row CustomGroupUsage
		row.Kind = "rule"
		if err := rows.Scan(&row.ID, &row.Label); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan custom group rule usage: %w", err)
		}
		usage = append(usage, row)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var enrollmentCount int
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)::int
FROM training.enrollment
WHERE source_custom_group_id = $1::uuid`, groupID).Scan(&enrollmentCount); err != nil {
		return nil, fmt.Errorf("custom group enrollment usage: %w", err)
	}
	if enrollmentCount > 0 {
		usage = append(usage, CustomGroupUsage{
			Kind:  "enrollment",
			Label: "Iscrizioni collegate",
			Count: enrollmentCount,
		})
	}
	return usage, nil
}

func normalizeMemberIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
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
	return out
}

func (s *SQLStore) ensureEmployeesSelectable(ctx context.Context, q sqlRunner, ids []string) error {
	for _, id := range ids {
		var exists bool
		err := q.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM training.employee
  WHERE id = $1::uuid
    AND status = 'active'
)`, id).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check custom group employee: %w", err)
		}
		if !exists {
			return validationError("group_members_invalid", "alcune persone non sono selezionabili")
		}
	}
	return nil
}

func (s *SQLStore) CreateCustomGroup(ctx context.Context, principal Principal, input CustomGroupInput) (CustomGroup, error) {
	if !principal.IsPeopleAdmin {
		return CustomGroup{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CustomGroup{}, validationError("name_required", "nome gruppo obbligatorio")
	}
	active := boolValue(input.Active, true)
	memberIDs := normalizeMemberIDs(input.MemberIDs)
	var groupID string
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.ensureEmployeesSelectable(ctx, tx, memberIDs); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `
INSERT INTO training.custom_groups (name, description, is_active)
VALUES ($1, NULLIF($2, ''), $3)
RETURNING id::text`, name, strings.TrimSpace(input.Description), active).Scan(&groupID); err != nil {
			if isUniqueViolation(err, "custom_groups_name_key") {
				return validationError("group_name_duplicate", "nome gruppo gia usato")
			}
			return fmt.Errorf("create custom group: %w", err)
		}
		if err := replaceCustomGroupMembers(ctx, tx, groupID, memberIDs); err != nil {
			return err
		}
		after, err := entitySnapshot(ctx, tx, "custom_groups", groupID)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, principal, "custom_groups", groupID, "create", nil, after)
	})
	if err != nil {
		return CustomGroup{}, err
	}
	return s.CustomGroupByID(ctx, groupID)
}

func (s *SQLStore) UpdateCustomGroup(ctx context.Context, principal Principal, id string, input CustomGroupInput) (CustomGroup, error) {
	if !principal.IsPeopleAdmin {
		return CustomGroup{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return CustomGroup{}, validationError("missing_group_id", "id gruppo obbligatorio")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CustomGroup{}, validationError("name_required", "nome gruppo obbligatorio")
	}
	active := boolValue(input.Active, true)
	memberIDs := normalizeMemberIDs(input.MemberIDs)
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		before, err := entitySnapshot(ctx, tx, "custom_groups", id)
		if appErr, ok := asAppError(err); ok && appErr.code == "entity_not_found" {
			return notFoundError("group_not_found", "gruppo non trovato")
		}
		if err != nil {
			return err
		}
		if err := s.ensureEmployeesSelectable(ctx, tx, memberIDs); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
UPDATE training.custom_groups
SET name = $2,
    description = NULLIF($3, ''),
    is_active = $4,
    updated_at = now()
WHERE id = $1::uuid`, id, name, strings.TrimSpace(input.Description), active)
		if err != nil {
			if isUniqueViolation(err, "custom_groups_name_key") {
				return validationError("group_name_duplicate", "nome gruppo gia usato")
			}
			return fmt.Errorf("update custom group: %w", err)
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			return notFoundError("group_not_found", "gruppo non trovato")
		}
		if err := replaceCustomGroupMembers(ctx, tx, id, memberIDs); err != nil {
			return err
		}
		after, err := entitySnapshot(ctx, tx, "custom_groups", id)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, principal, "custom_groups", id, "update", before, after)
	})
	if err != nil {
		return CustomGroup{}, err
	}
	return s.CustomGroupByID(ctx, id)
}

func replaceCustomGroupMembers(ctx context.Context, q sqlRunner, groupID string, memberIDs []string) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM training.custom_group_members WHERE group_id = $1::uuid`, groupID); err != nil {
		return fmt.Errorf("clear custom group members: %w", err)
	}
	for _, memberID := range memberIDs {
		if _, err := q.ExecContext(ctx, `
INSERT INTO training.custom_group_members (group_id, employee_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT DO NOTHING`, groupID, memberID); err != nil {
			return fmt.Errorf("insert custom group member: %w", err)
		}
	}
	return nil
}

func (s *SQLStore) DeleteCustomGroup(ctx context.Context, principal Principal, id string) error {
	if !principal.IsPeopleAdmin {
		return forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return validationError("missing_group_id", "id gruppo obbligatorio")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var activeRules int
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)::int
FROM training.mandatory_rules
WHERE is_active
  AND population_target->>'kind' = 'custom_group'
  AND population_target->>'id' = $1`, id).Scan(&activeRules); err != nil {
			return fmt.Errorf("count custom group active rules: %w", err)
		}
		if activeRules > 0 {
			return conflictError("group_in_use", "gruppo usato in regole attive")
		}
		before, err := entitySnapshot(ctx, tx, "custom_groups", id)
		if appErr, ok := asAppError(err); ok && appErr.code == "entity_not_found" {
			return notFoundError("group_not_found", "gruppo non trovato")
		}
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "custom_groups", id, "delete", before, nil); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM training.custom_groups WHERE id = $1::uuid`, id)
		if err != nil {
			return fmt.Errorf("delete custom group: %w", err)
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			return notFoundError("group_not_found", "gruppo non trovato")
		}
		return nil
	})
}

func (s *SQLStore) CustomGroupMemberIDs(ctx context.Context, q sqlRunner, groupID string) ([]string, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, validationError("missing_group_id", "id gruppo obbligatorio")
	}
	rows, err := q.QueryContext(ctx, `
SELECT employee_id::text
FROM training.custom_group_members
WHERE group_id = $1::uuid
ORDER BY employee_id`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list custom group member ids: %w", err)
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan custom group member id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		var exists bool
		if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM training.custom_groups WHERE id = $1::uuid)`, groupID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("check custom group exists: %w", err)
		}
		if !exists {
			return nil, notFoundError("group_not_found", "gruppo non trovato")
		}
	}
	return ids, nil
}
