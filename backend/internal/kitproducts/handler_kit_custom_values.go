package kitproducts

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type KitCustomValue struct {
	ID      int64  `json:"id"`
	KitID   int64  `json:"kit_id"`
	KeyName string `json:"key_name"`
	Value   string `json:"value"`
}

type KitCustomValueRequest struct {
	KeyName string          `json:"key_name"`
	Value   json.RawMessage `json:"value"`
}

func (h *Handler) handleListKitCustomValues(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	kitID, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}
	if ok, err := h.kitExists(r, kitID); err != nil {
		h.dbFailure(w, r, "list_kit_custom_values_lookup", err, "kit_id", kitID)
		return
	} else if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT
  id,
  kit_id,
  key_name,
  COALESCE(jsonb_pretty(value), '')
FROM products.kit_custom_value
WHERE kit_id = $1
ORDER BY id
`, kitID)
	if err != nil {
		h.dbFailure(w, r, "list_kit_custom_values", err, "kit_id", kitID)
		return
	}
	defer rows.Close()

	values := make([]KitCustomValue, 0)
	for rows.Next() {
		value, err := scanKitCustomValue(rows)
		if err != nil {
			h.dbFailure(w, r, "list_kit_custom_values", err, "kit_id", kitID)
			return
		}
		values = append(values, value)
	}
	if !h.rowsDone(w, r, rows, "list_kit_custom_values", "kit_id", kitID) {
		return
	}

	httputil.JSON(w, http.StatusOK, values)
}

func (h *Handler) handleCreateKitCustomValue(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	kitID, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}
	if ok, err := h.kitExists(r, kitID); err != nil {
		h.dbFailure(w, r, "create_kit_custom_value_lookup", err, "kit_id", kitID)
		return
	} else if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	var req KitCustomValueRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.KeyName = strings.TrimSpace(req.KeyName)
	if req.KeyName == "" {
		httputil.Error(w, http.StatusBadRequest, "key_name is required")
		return
	}
	if len(req.Value) > 0 && !json.Valid(req.Value) {
		httputil.Error(w, http.StatusBadRequest, "invalid value")
		return
	}

	rawValue := kitCustomValueJSON(req.Value)
	var createdID int64
	err = h.mistraDB.QueryRowContext(r.Context(), `
WITH inserted AS (
  INSERT INTO products.kit_custom_value (kit_id, key_name, value)
  VALUES ($1, $2, $3::jsonb)
  RETURNING id, kit_id, key_name, COALESCE(jsonb_pretty(value), '')
)
SELECT id FROM inserted
`, kitID, req.KeyName, rawValue).Scan(&createdID)
	if err != nil {
		h.dbFailure(w, r, "create_kit_custom_value", err, "kit_id", kitID)
		return
	}
	if createdID <= 0 {
		h.dbFailure(w, r, "create_kit_custom_value_result", errors.New("custom value creation returned invalid id"), "kit_id", kitID)
		return
	}

	value, err := h.getKitCustomValueByID(r, kitID, createdID)
	if h.rowError(w, r, "create_kit_custom_value_fetch", err, "kit_id", kitID, "kit_custom_value_id", createdID) {
		return
	}
	httputil.JSON(w, http.StatusCreated, value)
}

func (h *Handler) handleUpdateKitCustomValue(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	kitID, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}
	valueID, err := pathID64(r, "cvid")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid custom value id")
		return
	}
	if ok, err := h.kitExists(r, kitID); err != nil {
		h.dbFailure(w, r, "update_kit_custom_value_lookup", err, "kit_id", kitID, "kit_custom_value_id", valueID)
		return
	} else if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	var req KitCustomValueRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.KeyName = strings.TrimSpace(req.KeyName)
	if req.KeyName == "" {
		httputil.Error(w, http.StatusBadRequest, "key_name is required")
		return
	}
	if len(req.Value) > 0 && !json.Valid(req.Value) {
		httputil.Error(w, http.StatusBadRequest, "invalid value")
		return
	}

	rawValue := kitCustomValueJSON(req.Value)
	result, err := h.mistraDB.ExecContext(r.Context(), `
UPDATE products.kit_custom_value
SET key_name = $1,
    value = $2::jsonb
WHERE id = $3 AND kit_id = $4
`, req.KeyName, rawValue, valueID, kitID)
	if err != nil {
		h.dbFailure(w, r, "update_kit_custom_value", err, "kit_id", kitID, "kit_custom_value_id", valueID)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		h.dbFailure(w, r, "update_kit_custom_value_rows_affected", err, "kit_id", kitID, "kit_custom_value_id", valueID)
		return
	}
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	value, err := h.getKitCustomValueByID(r, kitID, valueID)
	if h.rowError(w, r, "update_kit_custom_value_fetch", err, "kit_id", kitID, "kit_custom_value_id", valueID) {
		return
	}
	httputil.JSON(w, http.StatusOK, value)
}

func (h *Handler) handleDeleteKitCustomValue(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	kitID, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}
	valueID, err := pathID64(r, "cvid")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid custom value id")
		return
	}
	if ok, err := h.kitExists(r, kitID); err != nil {
		h.dbFailure(w, r, "delete_kit_custom_value_lookup", err, "kit_id", kitID, "kit_custom_value_id", valueID)
		return
	} else if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	result, err := h.mistraDB.ExecContext(r.Context(), `
DELETE FROM products.kit_custom_value
WHERE id = $1 AND kit_id = $2
`, valueID, kitID)
	if err != nil {
		h.dbFailure(w, r, "delete_kit_custom_value", err, "kit_id", kitID, "kit_custom_value_id", valueID)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		h.dbFailure(w, r, "delete_kit_custom_value_rows_affected", err, "kit_id", kitID, "kit_custom_value_id", valueID)
		return
	}
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getKitCustomValueByID(r *http.Request, kitID, valueID int64) (KitCustomValue, error) {
	row := h.mistraDB.QueryRowContext(r.Context(), `
SELECT
  id,
  kit_id,
  key_name,
  COALESCE(jsonb_pretty(value), '')
FROM products.kit_custom_value
WHERE id = $1 AND kit_id = $2
`, valueID, kitID)
	return scanKitCustomValue(row)
}

func scanKitCustomValue(scanner interface{ Scan(dest ...any) error }) (KitCustomValue, error) {
	var value KitCustomValue
	if err := scanner.Scan(&value.ID, &value.KitID, &value.KeyName, &value.Value); err != nil {
		return KitCustomValue{}, err
	}
	return value, nil
}

func kitCustomValueJSON(value json.RawMessage) string {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" {
		return "null"
	}
	return trimmed
}
