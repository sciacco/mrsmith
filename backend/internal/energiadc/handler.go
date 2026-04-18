package energiadc

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type Handler struct {
	grappaDB *sql.DB
	config   ModuleConfig
}

func RegisterRoutes(mux *http.ServeMux, db *sql.DB, cfg ModuleConfig) {
	h := &Handler{
		grappaDB: db,
		config:   normalizeConfig(cfg),
	}
	protect := acl.RequireRole(applaunch.EnergiaDCAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /energia-dc/v1/customers", h.handleListCustomers)
	handle("GET /energia-dc/v1/customers/{customerId}/sites", h.handleListSites)
	handle("GET /energia-dc/v1/sites/{siteId}/rooms", h.handleListRooms)
	handle("GET /energia-dc/v1/rooms/{roomId}/racks", h.handleListRacks)
	handle("GET /energia-dc/v1/racks/{rackId}", h.handleGetRackDetail)
	handle("GET /energia-dc/v1/racks/{rackId}/socket-status", h.handleListRackSocketStatus)
	handle("GET /energia-dc/v1/racks/{rackId}/power-readings", h.handleListPowerReadings)
	handle("GET /energia-dc/v1/racks/{rackId}/stats-last-days", h.handleListRackStatsLastDays)
	handle("GET /energia-dc/v1/customers/{customerId}/kw", h.handleListCustomerKW)
	handle("GET /energia-dc/v1/customers/{customerId}/addebiti", h.handleListBillingCharges)
	handle("GET /energia-dc/v1/no-variable-billing/customers", h.handleListNoVariableCustomers)
	handle("GET /energia-dc/v1/no-variable-billing/customers/{customerId}/racks", h.handleListNoVariableRacks)
	handle("GET /energia-dc/v1/low-consumption", h.handleListLowConsumption)
}

func (h *Handler) requireDB(w http.ResponseWriter) bool {
	if h.grappaDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "energia_dc_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", "energia-dc", "operation", operation}
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

func (h *Handler) rowsDone(w http.ResponseWriter, r *http.Request, rows *sql.Rows, operation string) bool {
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, operation+"_rows", err)
		return false
	}
	return true
}

func (h *Handler) parsePathInt(w http.ResponseWriter, r *http.Request, name, errorCode string) (int, bool) {
	value, err := strconv.Atoi(r.PathValue(name))
	if err != nil || value <= 0 {
		httputil.Error(w, http.StatusBadRequest, errorCode)
		return 0, false
	}
	return value, true
}

func (h *Handler) parseRequiredQueryInt(w http.ResponseWriter, r *http.Request, name, errorCode string) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		httputil.Error(w, http.StatusBadRequest, errorCode)
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		httputil.Error(w, http.StatusBadRequest, errorCode)
		return 0, false
	}
	return value, true
}

func (h *Handler) parseOptionalQueryInt(w http.ResponseWriter, r *http.Request, name, errorCode string) (*int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		httputil.Error(w, http.StatusBadRequest, errorCode)
		return nil, false
	}
	return &value, true
}

func (h *Handler) parseRequiredQueryFloat64(w http.ResponseWriter, r *http.Request, name, errorCode string) (float64, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		httputil.Error(w, http.StatusBadRequest, errorCode)
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, errorCode)
		return 0, false
	}
	return value, true
}

func (h *Handler) parseLocalDateTime(w http.ResponseWriter, r *http.Request, name, errorCode string) (string, time.Time, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		httputil.Error(w, http.StatusBadRequest, errorCode)
		return "", time.Time{}, false
	}
	parsed, err := time.ParseInLocation(localDateTimeLayout, raw, h.config.Location)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, errorCode)
		return "", time.Time{}, false
	}
	return parsed.Format(sqlDateTimeLayout), parsed, true
}

func formatDate(timeValue sql.NullTime, location *time.Location) string {
	if !timeValue.Valid {
		return ""
	}
	return timeValue.Time.In(location).Format(dateLayout)
}

func formatDateTime(value time.Time, location *time.Location) string {
	return value.In(location).Format(dateTimeLayout)
}

func nullableInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func nullableFloat(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
}

func cleanString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func composePositions(values ...string) []string {
	positions := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			positions = append(positions, value)
		}
	}
	if positions == nil {
		return []string{}
	}
	return positions
}

func socketLabel(socketID int, positions []string) string {
	if len(positions) == 0 {
		return "Presa " + strconv.Itoa(socketID)
	}
	return strings.Join(positions, " / ")
}

func placeholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}
