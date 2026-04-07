package compliance

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
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
