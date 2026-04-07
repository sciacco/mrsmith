package compliance

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// Handler holds the database connection for compliance endpoints.
type Handler struct {
	db *sql.DB
}

// RegisterRoutes registers all compliance API routes on the given mux.
func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	h := &Handler{db: db}
	protect := acl.RequireRole(applaunch.ComplianceAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}
	// Block requests
	handle("GET /compliance/blocks", h.handleListBlocks)
	handle("GET /compliance/blocks/{id}", h.handleGetBlock)
	handle("POST /compliance/blocks", h.handleCreateBlock)
	handle("PUT /compliance/blocks/{id}", h.handleUpdateBlock)
	handle("GET /compliance/blocks/{id}/domains", h.handleListBlockDomains)
	handle("POST /compliance/blocks/{id}/domains", h.handleAddBlockDomains)
	handle("PUT /compliance/blocks/{id}/domains/{domainId}", h.handleUpdateBlockDomain)
	// Release requests
	handle("GET /compliance/releases", h.handleListReleases)
	handle("GET /compliance/releases/{id}", h.handleGetRelease)
	handle("POST /compliance/releases", h.handleCreateRelease)
	handle("PUT /compliance/releases/{id}", h.handleUpdateRelease)
	handle("GET /compliance/releases/{id}/domains", h.handleListReleaseDomains)
	handle("POST /compliance/releases/{id}/domains", h.handleAddReleaseDomains)
	handle("PUT /compliance/releases/{id}/domains/{domainId}", h.handleUpdateReleaseDomain)
	// Domain status & history
	handle("GET /compliance/domains", h.handleListDomainStatus)
	handle("GET /compliance/domains/history", h.handleListHistory)
	// Origins
	handle("GET /compliance/origins", h.handleListOrigins)
	handle("POST /compliance/origins", h.handleCreateOrigin)
	handle("PUT /compliance/origins/{id}", h.handleUpdateOrigin)
	handle("DELETE /compliance/origins/{id}", h.handleDeleteOrigin)
}

// requireDB returns false and writes a 503 if the database is not configured.
func (h *Handler) requireDB(w http.ResponseWriter) bool {
	if h.db == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "database not configured")
		return false
	}
	return true
}

// pathID extracts an integer path parameter.
func pathID(r *http.Request, name string) (int, error) {
	return strconv.Atoi(r.PathValue(name))
}

// decodeBody reads and decodes the JSON request body into v.
func decodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", "compliance", "operation", operation}
	args = append(args, attrs...)
	httputil.InternalError(w, r, err, "database operation failed", args...)
}

func (h *Handler) rowError(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return true
	}
	h.dbFailure(w, r, operation, err, attrs...)
	return true
}

func (h *Handler) rowsDone(w http.ResponseWriter, r *http.Request, rows *sql.Rows, operation string, attrs ...any) bool {
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, operation, err, attrs...)
		return false
	}
	return true
}

func (h *Handler) rollbackTx(r *http.Request, tx *sql.Tx, operation string, attrs ...any) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		args := []any{"component", "compliance", "operation", operation, "error", err}
		args = append(args, attrs...)
		logging.FromContext(r.Context()).Warn("transaction rollback failed", args...)
	}
}
