package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	PlanAuditPlanCreated       = "plan_created"
	PlanAuditStatusChanged     = "plan_status_changed"
	PlanAuditBudgetChanged     = "plan_budget_changed"
	PlanAuditNotesChanged      = "plan_notes_changed"
	PlanAuditPlanDeleted       = "plan_deleted"
	PlanAuditBulkPlanApplied   = "bulk_plan_applied"
	PlanAuditSuggestionDismiss = "suggestion_dismissed"
	PlanAuditAdhocCreated      = "adhoc_created"
	PlanAuditEnrollmentChanged = "enrollment_modified"
	PlanAuditEnrollmentCancel  = "enrollment_cancelled"
	PlanAuditBulkReviewApplied = "bulk_review_applied"
)

var planAuditEvents = map[string]struct{}{
	PlanAuditPlanCreated:       {},
	PlanAuditStatusChanged:     {},
	PlanAuditBudgetChanged:     {},
	PlanAuditNotesChanged:      {},
	PlanAuditPlanDeleted:       {},
	PlanAuditBulkPlanApplied:   {},
	PlanAuditSuggestionDismiss: {},
	PlanAuditAdhocCreated:      {},
	PlanAuditEnrollmentChanged: {},
	PlanAuditEnrollmentCancel:  {},
	PlanAuditBulkReviewApplied: {},
}

func knownPlanAuditEvent(eventType string) bool {
	_, ok := planAuditEvents[eventType]
	return ok
}

func planAuditEventTypes() []string {
	events := make([]string, 0, len(planAuditEvents))
	for eventType := range planAuditEvents {
		events = append(events, eventType)
	}
	sort.Strings(events)
	return events
}

func normalizeSuggestionID(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func bulkPlanAuditEventType(suggestionID *string) string {
	if normalizeSuggestionID(suggestionID) == "" {
		return PlanAuditAdhocCreated
	}
	return PlanAuditBulkPlanApplied
}

func emitPlanAuditEvent(ctx context.Context, q sqlRunner, principal Principal, planID string, eventType string, payload map[string]any) error {
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return validationError("missing_plan_id", "id piano obbligatorio")
	}
	if !knownPlanAuditEvent(eventType) {
		return fmt.Errorf("unknown training plan audit event: %s", eventType)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal training plan audit payload: %w", err)
	}
	actorID := strings.TrimSpace(principal.Subject)
	_, err = q.ExecContext(ctx, `
INSERT INTO training.plan_audit_log (
  plan_id,
  actor_id,
  event_type,
  payload
) VALUES ($1::uuid, $2, $3, $4::jsonb)`, planID, actorID, eventType, raw)
	if err != nil {
		return fmt.Errorf("insert training plan audit event: %w", err)
	}
	return nil
}

type planAuditPage struct {
	events     []PlanAuditEvent
	nextCursor string
}

func (s *SQLStore) ListPlanAuditEvents(ctx context.Context, principal Principal, planID string, limit int, before string) (planAuditPage, error) {
	if !principal.IsPeopleAdmin {
		return planAuditPage{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return planAuditPage{}, validationError("missing_plan_id", "id piano obbligatorio")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var beforeArg any
	if strings.TrimSpace(before) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(before))
		if err != nil {
			return planAuditPage{}, validationError("invalid_cursor", "cursore storico non valido")
		}
		beforeArg = parsed
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, plan_id::text, actor_id, event_type, payload, created_at
FROM training.plan_audit_log
WHERE plan_id = $1::uuid
  AND ($2::timestamptz IS NULL OR created_at < $2::timestamptz)
ORDER BY created_at DESC, id DESC
LIMIT $3`, planID, beforeArg, limit+1)
	if err != nil {
		return planAuditPage{}, fmt.Errorf("list training plan audit events: %w", err)
	}
	defer rows.Close()

	events := make([]PlanAuditEvent, 0, limit)
	var cursor string
	for rows.Next() {
		var event PlanAuditEvent
		var actorID string
		var payloadRaw []byte
		var createdAt time.Time
		if err := rows.Scan(&event.ID, &event.PlanID, &actorID, &event.EventType, &payloadRaw, &createdAt); err != nil {
			return planAuditPage{}, fmt.Errorf("scan training plan audit event: %w", err)
		}
		payload := map[string]any{}
		if len(payloadRaw) > 0 {
			if err := json.Unmarshal(payloadRaw, &payload); err != nil {
				return planAuditPage{}, fmt.Errorf("decode training plan audit payload: %w", err)
			}
		}
		event.Actor = PlanAuditActor{ID: actorID, DisplayName: actorID}
		event.Payload = payload
		event.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		if len(events) < limit {
			events = append(events, event)
			cursor = event.CreatedAt
		}
	}
	if err := rows.Err(); err != nil {
		return planAuditPage{}, err
	}
	if len(events) == limit {
		var count int
		err := s.db.QueryRowContext(ctx, `
SELECT 1
FROM training.plan_audit_log
WHERE plan_id = $1::uuid
  AND created_at < $2::timestamptz
LIMIT 1`, planID, cursor).Scan(&count)
		if errors.Is(err, sql.ErrNoRows) {
			cursor = ""
		} else if err != nil {
			return planAuditPage{}, fmt.Errorf("check training plan audit next page: %w", err)
		}
	}
	return planAuditPage{events: events, nextCursor: cursor}, nil
}
