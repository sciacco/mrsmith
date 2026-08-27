package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// Gruppi locali (#152, slice 1 del task 6): CRUD e vincoli di platea. Un
// gruppo non si puo eliminare quando e platea di una regola attiva (kind
// custom_group) o collegato a un'area di competenza (migrazione 131).

// ListGroups aggrega i membri in JSON lato query, stesso pattern di
// store_people_reads.go.
func (s *SQLStore) ListGroups(ctx context.Context) ([]GroupListRow, error) {
	const q = `
SELECT
  g.id::text, g.name, COALESCE(g.description, ''),
  COALESCE((
    SELECT json_agg(json_build_object('employeeId', e.id::text, 'name', concat(e.last_name, ' ', e.first_name), 'email', e.email::text) ORDER BY e.last_name, e.first_name)
    FROM training.custom_group_members m
    JOIN training.employee e ON e.id = m.employee_id
    WHERE m.group_id = g.id
  ), '[]')::text
FROM training.custom_groups g
ORDER BY g.name
LIMIT 500`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training groups: %w", err)
	}
	defer rows.Close()

	result := make([]GroupListRow, 0)
	for rows.Next() {
		var row GroupListRow
		var members string
		if err := rows.Scan(&row.ID, &row.Name, &row.Description, &members); err != nil {
			return nil, fmt.Errorf("scan training group: %w", err)
		}
		if row.Members, err = decodeJSONSlice[GroupMemberRef](members); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *SQLStore) UpsertGroup(ctx context.Context, principal Principal, id string, input GroupInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(input.Name) == "" {
		return ActionResponse{}, validationError("name_required", "nome obbligatorio")
	}
	return s.upsertSimple(ctx, principal, "custom_groups", id, []upsertField{
		field("name", strings.TrimSpace(input.Name)),
		field("description", nullableText(input.Description)),
	})
}

func (s *SQLStore) groupPersonIDs(ctx context.Context, q sqlRunner, groupID string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `
SELECT employee_id::text
FROM training.custom_group_members
WHERE group_id = $1::uuid
ORDER BY 1`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list training group people: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan training group person: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ReplaceGroupMembers sostituisce per intero l'insieme dei membri del
// gruppo: solo persone attive (422 altrimenti). Stesso schema di
// replaceRulePeople (store_rules.go): le righe non hanno un id proprio,
// l'audit e dedicato sull'entita gruppo.
func (s *SQLStore) ReplaceGroupMembers(ctx context.Context, principal Principal, groupID string, input GroupMembersInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return ActionResponse{}, validationError("missing_id", "id gruppo obbligatorio")
	}
	employeeIDs := make([]string, 0, len(input.EmployeeIDs))
	seen := make(map[string]bool, len(input.EmployeeIDs))
	for _, raw := range input.EmployeeIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			return ActionResponse{}, validationError("missing_id", "id persona obbligatorio")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		employeeIDs = append(employeeIDs, id)
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (SELECT 1 FROM training.custom_groups WHERE id = $1::uuid)`, groupID).Scan(&exists); err != nil {
			return fmt.Errorf("check training group: %w", err)
		}
		if !exists {
			return notFoundError("custom_group_not_found", "gruppo locale non trovato")
		}
		for _, employeeID := range employeeIDs {
			if err := s.ensureEmployeeActive(ctx, tx, employeeID); err != nil {
				return err
			}
		}
		current, err := s.groupPersonIDs(ctx, tx, groupID)
		if err != nil {
			return err
		}
		if equalStringSets(current, employeeIDs) {
			response = ActionResponse{OK: true, ID: groupID}
			return nil
		}
		if _, err := tx.ExecContext(ctx, `
DELETE FROM training.custom_group_members WHERE group_id = $1::uuid`, groupID); err != nil {
			return fmt.Errorf("delete training group members: %w", err)
		}
		for _, employeeID := range employeeIDs {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO training.custom_group_members (group_id, employee_id)
VALUES ($1::uuid, $2::uuid)`, groupID, employeeID); err != nil {
				return fmt.Errorf("insert training group member: %w", err)
			}
		}
		before, err := json.Marshal(map[string]any{"employeeIds": current})
		if err != nil {
			return fmt.Errorf("marshal training group members audit: %w", err)
		}
		after, err := json.Marshal(map[string]any{"employeeIds": employeeIDs})
		if err != nil {
			return fmt.Errorf("marshal training group members audit: %w", err)
		}
		if err := s.audit(ctx, tx, principal, "custom_groups", groupID, "set_members", before, after); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: groupID}
		return nil
	})
	return response, err
}

func (s *SQLStore) DeleteGroup(ctx context.Context, principal Principal, id string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ActionResponse{}, validationError("missing_id", "id gruppo obbligatorio")
	}

	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		before, err := entitySnapshot(ctx, tx, "custom_groups", id)
		if appErr, ok := asAppError(err); ok && appErr.code == "entity_not_found" {
			return notFoundError("custom_group_not_found", "gruppo locale non trovato")
		}
		if err != nil {
			return err
		}

		var usedByActiveRule, linkedToSkillArea bool
		if err := tx.QueryRowContext(ctx, `
SELECT
  EXISTS (SELECT 1 FROM training.training_rule r WHERE r.is_active AND r.population_target->>'kind' = 'custom_group' AND (r.population_target->>'id')::uuid = $1::uuid),
  EXISTS (SELECT 1 FROM training.skill_area WHERE custom_group_id = $1::uuid)`, id).Scan(&usedByActiveRule, &linkedToSkillArea); err != nil {
			return fmt.Errorf("check training group deletion guards: %w", err)
		}
		if usedByActiveRule {
			return conflictError("group_required_by_active_rule", "il gruppo non si puo eliminare: e la platea di una regola attiva")
		}
		if linkedToSkillArea {
			return conflictError("group_linked_to_skill_area", "il gruppo non si puo eliminare: e collegato a un'area di competenza")
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM training.custom_groups WHERE id = $1::uuid`, id); err != nil {
			return fmt.Errorf("delete training group: %w", err)
		}
		if err := s.audit(ctx, tx, principal, "custom_groups", id, "delete", before, nil); err != nil {
			return err
		}
		response = ActionResponse{OK: true, ID: id}
		return nil
	})
	return response, err
}
