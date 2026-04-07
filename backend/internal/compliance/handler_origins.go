package compliance

import (
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListOrigins(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	includeInactive := r.URL.Query().Get("include_inactive") == "true"

	query := `SELECT method_id, description, is_active FROM dns_bl_method`
	if !includeInactive {
		query += ` WHERE is_active = true`
	}
	query += ` ORDER BY method_id`

	rows, err := h.db.QueryContext(r.Context(), query)
	if err != nil {
		h.dbFailure(w, r, "list_origins", err)
		return
	}
	defer rows.Close()

	origins := make([]Origin, 0)
	for rows.Next() {
		var o Origin
		if err := rows.Scan(&o.MethodID, &o.Description, &o.IsActive); err != nil {
			h.dbFailure(w, r, "list_origins", err)
			return
		}
		origins = append(origins, o)
	}
	if !h.rowsDone(w, r, rows, "list_origins") {
		return
	}

	httputil.JSON(w, http.StatusOK, origins)
}

func (h *Handler) handleCreateOrigin(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	var body CreateOriginRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	body.MethodID = strings.TrimSpace(body.MethodID)
	body.Description = strings.TrimSpace(body.Description)
	if body.MethodID == "" || body.Description == "" {
		httputil.Error(w, http.StatusBadRequest, "method_id and description are required")
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO dns_bl_method (method_id, description, is_active) VALUES ($1, $2, true)`,
		body.MethodID, body.Description)
	if err != nil {
		h.dbFailure(w, r, "create_origin", err, "method_id", body.MethodID)
		return
	}

	httputil.JSON(w, http.StatusCreated, map[string]string{"method_id": body.MethodID})
}

func (h *Handler) handleUpdateOrigin(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	methodID := r.PathValue("id")

	var body struct {
		Description *string `json:"description"`
		IsActive    *bool   `json:"is_active"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Description != nil {
		desc := strings.TrimSpace(*body.Description)
		if desc == "" {
			httputil.Error(w, http.StatusBadRequest, "description is required")
			return
		}
		res, err := h.db.ExecContext(r.Context(),
			`UPDATE dns_bl_method SET description = $1 WHERE method_id = $2`,
			desc, methodID)
		if err != nil {
			h.dbFailure(w, r, "update_origin_description", err, "method_id", methodID)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			httputil.Error(w, http.StatusNotFound, "not_found")
			return
		}
	}

	if body.IsActive != nil {
		res, err := h.db.ExecContext(r.Context(),
			`UPDATE dns_bl_method SET is_active = $1 WHERE method_id = $2`,
			*body.IsActive, methodID)
		if err != nil {
			h.dbFailure(w, r, "update_origin_active", err, "method_id", methodID)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			httputil.Error(w, http.StatusNotFound, "not_found")
			return
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"method_id": methodID})
}

func (h *Handler) handleDeleteOrigin(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	methodID := r.PathValue("id")

	res, err := h.db.ExecContext(r.Context(),
		`UPDATE dns_bl_method SET is_active = false WHERE method_id = $1`,
		methodID)
	if err != nil {
		h.dbFailure(w, r, "delete_origin", err, "method_id", methodID)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	// Return 200 with JSON body (not 204) — required for shared ApiClient which always calls res.json()
	httputil.JSON(w, http.StatusOK, map[string]string{"method_id": methodID})
}
