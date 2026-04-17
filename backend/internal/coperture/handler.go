package coperture

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type Handler struct {
	db *sql.DB
}

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	h := &Handler{db: db}
	protect := acl.RequireRole(applaunch.CopertureAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /coperture/v1/states", h.handleListStates)
	handle("GET /coperture/v1/states/{stateId}/cities", h.handleListCities)
	handle("GET /coperture/v1/cities/{cityId}/addresses", h.handleListAddresses)
	handle("GET /coperture/v1/addresses/{addressId}/house-numbers", h.handleListHouseNumbers)
	handle("GET /coperture/v1/house-numbers/{houseNumberId}/coverage", h.handleListCoverage)
}

func (h *Handler) requireDB(w http.ResponseWriter) bool {
	if h.db == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "coperture_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", "coperture", "operation", operation}
	args = append(args, attrs...)
	httputil.InternalError(w, r, err, "database operation failed", args...)
}

func (h *Handler) rowsDone(w http.ResponseWriter, r *http.Request, rows *sql.Rows, operation string) bool {
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, operation+"_rows", err)
		return false
	}
	return true
}
