package training

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

func (s *SQLStore) ComplianceOverview(ctx context.Context, principal Principal, year int, team string, deadlineDays int) (ComplianceOverviewResponse, error) {
	if !principal.IsPeopleAdmin {
		return ComplianceOverviewResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if deadlineDays <= 0 {
		deadlineDays = 30
	}
	resp := ComplianceOverviewResponse{
		Year:              year,
		TeamScope:         team,
		DeadlineDays:      deadlineDays,
		ExpiringDeadlines: []ComplianceExpiringRow{},
		Rules:             []ComplianceRule{},
	}

	expiring, err := s.complianceExpiringWithin(ctx, team, deadlineDays)
	if err != nil {
		return resp, err
	}
	resp.ExpiringDeadlines = expiring

	rules, err := s.complianceRulesAggregated(ctx, team)
	if err != nil {
		return resp, err
	}
	resp.Rules = rules
	return resp, nil
}

func (s *SQLStore) complianceExpiringWithin(ctx context.Context, team string, days int) ([]ComplianceExpiringRow, error) {
	const q = `
SELECT
  vec.employee_id::text,
  concat(vec.last_name, ' ', vec.first_name) AS employee_name,
  c.id::text,
  vec.cert_name,
  vec.days_to_expiry::int
FROM training.v_expiring_certifications vec
JOIN training.certification c ON c.code = vec.cert_code
LEFT JOIN training.team_membership tm
  ON tm.employee_id = vec.employee_id
  AND tm.start_date <= now()
  AND (tm.end_date IS NULL OR tm.end_date >= now())
LEFT JOIN training.team t ON t.id = tm.team_id
WHERE vec.days_to_expiry <= $1
  AND ($2 = '' OR t.code = $2)
ORDER BY vec.days_to_expiry ASC, vec.last_name, vec.first_name
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q, days, team)
	if err != nil {
		return nil, fmt.Errorf("compliance expiring: %w", err)
	}
	defer rows.Close()
	out := make([]ComplianceExpiringRow, 0)
	for rows.Next() {
		var row ComplianceExpiringRow
		if err := rows.Scan(&row.EmployeeID, &row.EmployeeName, &row.RuleID, &row.RuleTitle, &row.ExpiresInDays); err != nil {
			return nil, fmt.Errorf("scan compliance expiring: %w", err)
		}
		switch {
		case row.ExpiresInDays <= 7:
			row.Severity = "critical"
		case row.ExpiresInDays <= 30:
			row.Severity = "warning"
		default:
			row.Severity = "info"
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// complianceRulesAggregated builds one ComplianceRule per active Training rule.
func (s *SQLStore) complianceRulesAggregated(ctx context.Context, team string) ([]ComplianceRule, error) {
	const ruleQ = `
SELECT
  r.id::text                                              AS rule_id,
  r.name                                                  AS rule_name,
  c.id::text                                              AS course_id,
  c.title                                                 AS course_title,
  COALESCE(c.compliance_framework, '')                    AS framework,
  COALESCE(c.recurrence_interval::text, '')               AS cadence,
  COALESCE(
    CASE r.population_target->>'kind'
      WHEN 'all' THEN 'Tutte le persone'
      WHEN 'team' THEN team.code || ' - ' || team.name
      WHEN 'skill_area' THEN skill.code || ' - ' || skill.name
      WHEN 'custom_group' THEN groups.name
      ELSE ''
    END,
    ''
  )                                                       AS population_label
FROM training.mandatory_rules r
JOIN training.course c ON c.id = r.course_id
LEFT JOIN training.team team
  ON r.population_target->>'kind' = 'team'
 AND team.id = (r.population_target->>'id')::uuid
LEFT JOIN training.skill_area skill
  ON r.population_target->>'kind' = 'skill_area'
 AND skill.id = (r.population_target->>'id')::uuid
LEFT JOIN training.custom_groups groups
  ON r.population_target->>'kind' = 'custom_group'
 AND groups.id = (r.population_target->>'id')::uuid
WHERE r.is_active
ORDER BY r.name, c.title`
	rows, err := s.db.QueryContext(ctx, ruleQ)
	if err != nil {
		return nil, fmt.Errorf("compliance rules: %w", err)
	}
	defer rows.Close()

	type ruleAcc struct {
		rule     ComplianceRule
		ruleID   string
		courseID string
	}
	rules := make([]ruleAcc, 0)
	for rows.Next() {
		var (
			ruleID, ruleName, courseID, courseTitle, framework, cadence, population string
		)
		if err := rows.Scan(&ruleID, &ruleName, &courseID, &courseTitle, &framework, &cadence, &population); err != nil {
			return nil, fmt.Errorf("scan compliance rule: %w", err)
		}
		title := strings.TrimSpace(ruleName)
		if title == "" {
			title = courseTitle
		}
		if framework != "" && !strings.Contains(title, framework) {
			title = fmt.Sprintf("%s · %s", framework, title)
		}
		rules = append(rules, ruleAcc{
			rule: ComplianceRule{
				ID:                 ruleID,
				Title:              title,
				CadenceLabel:       cadenceLabel(cadence),
				PopulationTarget:   population,
				Gaps:               []ComplianceRuleGap{},
				SuggestedCourseIDs: []string{courseID},
			},
			ruleID:   ruleID,
			courseID: courseID,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range rules {
		acc := &rules[i]
		covered, target, gaps, err := s.complianceRuleCoverage(ctx, acc.ruleID, team)
		if err != nil {
			return nil, err
		}
		acc.rule.CoveredCount = covered
		acc.rule.TargetCount = target
		acc.rule.Gaps = gaps
		if target > 0 {
			acc.rule.CoveragePct = float64(covered) / float64(target) * 100
		} else {
			acc.rule.CoveragePct = 100
		}
		acc.rule.Severity = complianceSeverity(acc.rule.CoveragePct, len(gaps))
	}

	out := make([]ComplianceRule, 0, len(rules))
	for _, r := range rules {
		if team != "" && r.rule.TargetCount == 0 {
			continue
		}
		out = append(out, r.rule)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return severityRank(out[i].Severity) < severityRank(out[j].Severity)
	})
	return out, nil
}

func cadenceLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "una tantum"
	}
	// Postgres interval text comes as "1 year", "2 mons", etc.
	return raw
}

func (s *SQLStore) complianceRuleCoverage(ctx context.Context, ruleID, filterTeam string) (covered int, target int, gaps []ComplianceRuleGap, err error) {
	gaps = []ComplianceRuleGap{}
	const q = `
SELECT
  g.employee_id::text,
  concat(g.last_name, ' ', g.first_name)            AS employee_name,
  g.compliance_status,
  COALESCE(g.last_valid_awarded_on::text, g.last_valid_completed_on::text, '') AS detail
FROM training.v_mandatory_compliance_gap g
LEFT JOIN training.team_membership tm
  ON tm.employee_id = g.employee_id
  AND tm.start_date <= now()
  AND (tm.end_date IS NULL OR tm.end_date >= now())
LEFT JOIN training.team t ON t.id = tm.team_id
WHERE g.rule_id = $1::uuid
  AND ($2 = '' OR t.code = $2)
ORDER BY g.compliance_status DESC, g.last_name, g.first_name`
	rows, queryErr := s.db.QueryContext(ctx, q, ruleID, filterTeam)
	if queryErr != nil {
		return 0, 0, nil, fmt.Errorf("compliance rule coverage: %w", queryErr)
	}
	defer rows.Close()

	for rows.Next() {
		var empID, empName, status, detail string
		if scanErr := rows.Scan(&empID, &empName, &status, &detail); scanErr != nil {
			return 0, 0, nil, fmt.Errorf("scan rule coverage: %w", scanErr)
		}
		target++
		if status == "compliant" {
			covered++
			continue
		}
		gapStatus := "never_covered"
		if detail != "" {
			gapStatus = "expired"
		}
		gaps = append(gaps, ComplianceRuleGap{
			EmployeeID:   empID,
			EmployeeName: empName,
			Status:       gapStatus,
			Detail:       detail,
		})
	}
	if err = rows.Err(); err != nil {
		return 0, 0, nil, err
	}
	if len(gaps) > 10 {
		gaps = gaps[:10]
	}
	return covered, target, gaps, nil
}

func complianceSeverity(coveragePct float64, gapsCount int) string {
	if gapsCount == 0 || coveragePct >= 100 {
		return "ok"
	}
	gapPct := 100 - coveragePct
	if gapPct > 10 {
		return "critical"
	}
	return "warning"
}
