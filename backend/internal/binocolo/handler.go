package binocolo

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
)

type Deps struct {
	OpenAPIIT  *openapiit.Client
	AnisettaDB *sql.DB
}

type Handler struct {
	openapiit          *openapiit.Client
	companySearchCache companySearchCacheStore
}

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	var cache companySearchCacheStore
	if deps.AnisettaDB != nil {
		cache = NewSQLStore(deps.AnisettaDB)
	}
	h := &Handler{openapiit: deps.OpenAPIIT, companySearchCache: cache}
	protect := acl.RequireRole(applaunch.BinocoloAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /binocolo/v1/provinces", h.handleListProvinces)
	handle("GET /binocolo/v1/companies/search", h.handleSearchCompanies)
}

func (h *Handler) requireOpenAPIIT(w http.ResponseWriter) bool {
	if h.openapiit == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "openapiit_not_configured")
		return false
	}
	return true
}

func (h *Handler) openAPIITFailure(w http.ResponseWriter, r *http.Request, operation string, err error) {
	logging.FromContext(r.Context()).Warn(
		"openapi.it request failed",
		"component", "binocolo",
		"operation", operation,
		"error", err,
	)
	httputil.Error(w, http.StatusBadGateway, "openapiit_upstream_error")
}

func (h *Handler) binocoloCacheFailure(w http.ResponseWriter, r *http.Request, err error) {
	logging.FromContext(r.Context()).Error(
		"binocolo cache request failed",
		"component", "binocolo",
		"error", err,
	)
	httputil.Error(w, http.StatusInternalServerError, "binocolo_cache_error")
}
