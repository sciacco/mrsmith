package rdfbackend

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type Supplier struct {
	ID   int    `json:"id"`
	Nome string `json:"nome"`
}

type listSuppliersResponse struct {
	Items []Supplier `json:"items"`
	Total int        `json:"total"`
}

type createSupplierRequest struct {
	Nome string `json:"nome"`
}

type updateSupplierRequest struct {
	Nome *string `json:"nome"`
}

type Handler struct {
	db *sql.DB
}

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	h := &Handler{db: db}
	protect := acl.RequireRole(applaunch.RDFBackendAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /rdf-backend/v1/fornitori", h.handleListSuppliers)
	handle("POST /rdf-backend/v1/fornitori", h.handleCreateSupplier)
	handle("PATCH /rdf-backend/v1/fornitori/{id}", h.handleUpdateSupplier)
	handle("DELETE /rdf-backend/v1/fornitori/{id}", h.handleDeleteSupplier)
}

func (h *Handler) handleListSuppliers(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	sortColumn := normalizeSort(r.URL.Query().Get("sort"))
	sortOrder := normalizeOrder(r.URL.Query().Get("order"))
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 20)
	offset := (page - 1) * pageSize

	whereClause := ""
	countArgs := []any{}
	listArgs := []any{}
	if search != "" {
		whereClause = " WHERE nome ILIKE $1"
		pattern := "%" + search + "%"
		countArgs = append(countArgs, pattern)
		listArgs = append(listArgs, pattern)
	}

	countQuery := "SELECT COUNT(*) FROM public.rdf_fornitori" + whereClause
	var total int
	if err := h.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		h.dbFailure(w, r, "list_suppliers_count", err)
		return
	}

	listQuery := fmt.Sprintf(
		"SELECT id, nome FROM public.rdf_fornitori%s ORDER BY %s %s LIMIT $%d OFFSET $%d",
		whereClause,
		sortColumn,
		sortOrder,
		len(listArgs)+1,
		len(listArgs)+2,
	)
	listArgs = append(listArgs, pageSize, offset)

	rows, err := h.db.QueryContext(r.Context(), listQuery, listArgs...)
	if err != nil {
		h.dbFailure(w, r, "list_suppliers", err)
		return
	}
	defer rows.Close()

	items := make([]Supplier, 0)
	for rows.Next() {
		var item Supplier
		if err := rows.Scan(&item.ID, &item.Nome); err != nil {
			h.dbFailure(w, r, "scan_suppliers", err)
			return
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "scan_suppliers_rows", err)
		return
	}

	httputil.JSON(w, http.StatusOK, listSuppliersResponse{
		Items: items,
		Total: total,
	})
}

func (h *Handler) handleCreateSupplier(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	var body createSupplierRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}

	name := strings.TrimSpace(body.Nome)
	if name == "" {
		httputil.Error(w, http.StatusBadRequest, "nome_required")
		return
	}

	var item Supplier
	if err := h.db.QueryRowContext(
		r.Context(),
		"INSERT INTO public.rdf_fornitori (nome) VALUES ($1) RETURNING id, nome",
		name,
	).Scan(&item.ID, &item.Nome); err != nil {
		h.dbFailure(w, r, "create_supplier", err)
		return
	}

	httputil.JSON(w, http.StatusCreated, item)
}

func (h *Handler) handleUpdateSupplier(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	id, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_id")
		return
	}

	var body updateSupplierRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}

	if body.Nome == nil {
		httputil.Error(w, http.StatusBadRequest, "no_fields_to_update")
		return
	}

	name := strings.TrimSpace(*body.Nome)
	if name == "" {
		httputil.Error(w, http.StatusBadRequest, "nome_required")
		return
	}

	var item Supplier
	err = h.db.QueryRowContext(
		r.Context(),
		"UPDATE public.rdf_fornitori SET nome = $1 WHERE id = $2 RETURNING id, nome",
		name,
		id,
	).Scan(&item.ID, &item.Nome)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "update_supplier", err, "supplier_id", id)
		return
	}

	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleDeleteSupplier(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	id, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_id")
		return
	}

	result, err := h.db.ExecContext(r.Context(), "DELETE FROM public.rdf_fornitori WHERE id = $1", id)
	if err != nil {
		h.dbFailure(w, r, "delete_supplier", err, "supplier_id", id)
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		h.dbFailure(w, r, "delete_supplier_rows_affected", err, "supplier_id", id)
		return
	}
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) requireDB(w http.ResponseWriter) bool {
	if h.db == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "anisetta_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", "rdfbackend", "operation", operation}
	args = append(args, attrs...)
	httputil.InternalError(w, r, err, "database operation failed", args...)
}

func decodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func pathID(r *http.Request, name string) (int, error) {
	return strconv.Atoi(r.PathValue(name))
}

func parsePositiveInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func normalizeSort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "nome":
		return "nome"
	default:
		return "id"
	}
}

func normalizeOrder(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "desc") {
		return "DESC"
	}
	return "ASC"
}
