package rda

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type poApproverBrief struct {
	Email string `json:"email"`
	Level int64  `json:"level"`
}

func (h *Handler) enrichPOListApprovers(ctx context.Context, body []byte) ([]byte, error) {
	if h.arakDB == nil {
		return body, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return body, err
	}

	items, ok := poListItems(payload)
	if !ok || len(items) == 0 {
		return body, nil
	}

	ids := poListIDs(items)
	if len(ids) == 0 {
		return body, nil
	}

	approversByPO, err := h.fetchPOApproverBriefs(ctx, ids)
	if err != nil {
		return body, err
	}

	for _, item := range items {
		id, ok := poListItemID(item)
		if !ok {
			continue
		}
		approvers := approversByPO[id]
		if approvers == nil {
			approvers = []poApproverBrief{}
		}
		item["approvers"] = approvers
	}

	return json.Marshal(payload)
}

func poListItems(payload any) ([]map[string]any, bool) {
	if items, ok := payload.([]any); ok {
		return poListItemMaps(items), true
	}

	if envelope, ok := payload.(map[string]any); ok {
		if items, ok := envelope["items"].([]any); ok {
			return poListItemMaps(items), true
		}
	}

	return nil, false
}

func poListItemMaps(items []any) []map[string]any {
	mapped := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if row, ok := item.(map[string]any); ok {
			mapped = append(mapped, row)
		}
	}
	return mapped
}

func poListIDs(items []map[string]any) []int64 {
	ids := make([]int64, 0, len(items))
	seen := make(map[int64]struct{}, len(items))
	for _, item := range items {
		id, ok := poListItemID(item)
		if !ok {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func poListItemID(item map[string]any) (int64, bool) {
	switch value := item["id"].(type) {
	case json.Number:
		id, err := value.Int64()
		return id, err == nil
	case float64:
		id := int64(value)
		return id, float64(id) == value
	case string:
		id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return id, err == nil
	default:
		return 0, false
	}
}

func (h *Handler) fetchPOApproverBriefs(ctx context.Context, ids []int64) (map[int64][]poApproverBrief, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	rows, err := h.arakDB.QueryContext(ctx, `
		SELECT a.order_id, a.level, u.email
		FROM rda.approval a
		JOIN users_int."user" u ON u.id = a.approver_id
		WHERE a.order_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY a.order_id, a.level, u.email`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]poApproverBrief, len(ids))
	for rows.Next() {
		var orderID int64
		var level sql.NullInt64
		var email sql.NullString
		if err := rows.Scan(&orderID, &level, &email); err != nil {
			return nil, err
		}
		if !level.Valid || !email.Valid || strings.TrimSpace(email.String) == "" {
			continue
		}
		result[orderID] = append(result[orderID], poApproverBrief{
			Email: strings.TrimSpace(email.String),
			Level: level.Int64,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
