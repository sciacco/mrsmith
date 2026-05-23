package ordini

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

type Deps struct {
	Vodka   *sql.DB
	Alyante *sql.DB
	Mistra  *sql.DB
	Arak    *arak.Client
	Logger  *slog.Logger
}

type Handler struct {
	deps   Deps
	logger *slog.Logger
}

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	h := &Handler{deps: deps, logger: logger.With("component", component)}
	protect := acl.RequireRole(applaunch.OrdiniAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /ordini/v1/orders", h.handleListOrders)
	handle("GET /ordini/v1/orders/{id}", h.handleGetOrder)
	handle("GET /ordini/v1/orders/{id}/rows", h.handleListRows)
	handle("GET /ordini/v1/orders/{id}/technical-rows", h.handleListTechnicalRows)
	handle("GET /ordini/v1/ref/customers", h.handleListCustomers)
	handle("GET /ordini/v1/orders/{id}/kickoff.pdf", h.handleKickoffPDF)
	handle("GET /ordini/v1/orders/{id}/activation-form.pdf", h.handleActivationFormPDF)
	handle("GET /ordini/v1/orders/{id}/pdf", h.handleOrderPDF)
	handle("GET /ordini/v1/orders/{id}/signed-pdf", h.handleSignedPDF)

	handle("PATCH /ordini/v1/orders/{id}", h.handlePatchOrderHeader)
	handle("PATCH /ordini/v1/orders/{id}/referents", h.handlePatchReferents)
	handle("POST /ordini/v1/orders/{id}/send-to-erp", h.handleSendToERP)
	handle("PATCH /ordini/v1/orders/{id}/rows/{rowId}/serial-number", h.handlePatchSerialNumber)
	handle("PATCH /ordini/v1/orders/{id}/rows/{rowId}/technical-notes", h.handlePatchTechnicalNotes)
	handle("PATCH /ordini/v1/orders/{id}/rows/{rowId}/activate", h.handleActivateRow)
}

func (h *Handler) requireVodka(w http.ResponseWriter) bool {
	if h.deps.Vodka == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "vodka_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireAlyante(w http.ResponseWriter) bool {
	if h.deps.Alyante == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "alyante_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireGateway(w http.ResponseWriter) bool {
	if h.deps.Arak == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "gateway_not_configured")
		return false
	}
	return true
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	h.logFailure(r, slog.LevelError, "ordini database operation failed", operation, time.Now(), append(attrs, "error", err)...)
	httputil.Error(w, http.StatusInternalServerError, "db_failed")
}

func (h *Handler) logFailure(r *http.Request, level slog.Level, message, operation string, start time.Time, attrs ...any) {
	durationMS := int64(0)
	if !start.IsZero() {
		durationMS = time.Since(start).Milliseconds()
	}
	requestID := logging.RequestID(r.Context())
	if requestID == "" {
		requestID = strings.TrimSpace(r.Header.Get("X-Request-ID"))
	}
	args := []any{
		"component", component,
		"operation", operation,
		"request_id", requestID,
		"duration_ms", durationMS,
	}
	args = append(args, attrs...)
	h.logger.Log(r.Context(), level, message, args...)
}

func (h *Handler) parseOrderID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_order_id")
		return 0, false
	}
	return id, true
}

func (h *Handler) parseRowID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("rowId")), 10, 64)
	if err != nil || id <= 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_row_id")
		return 0, false
	}
	return id, true
}

func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	defer r.Body.Close()
	var payload T
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request_body")
		return payload, false
	}
	return payload, true
}

func (h *Handler) rowsDone(w http.ResponseWriter, r *http.Request, rows *sql.Rows, operation string) bool {
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, operation, err)
		return false
	}
	return true
}

func (h *Handler) writeOrderOrNotFound(w http.ResponseWriter, r *http.Request, id int64) bool {
	order, err := h.getOrder(r, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "order_not_found")
			return false
		}
		h.dbFailure(w, r, "get_order", err, "order_id", id)
		return false
	}
	httputil.JSON(w, http.StatusOK, order)
	return true
}

func orderCodeForFilename(order *OrderDetail) string {
	if order == nil {
		return "ordine"
	}
	ndoc := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(ptrStringValue(order.CdlanNdoc))
	anno := ""
	if order.CdlanAnno != nil {
		anno = strconv.FormatInt(*order.CdlanAnno, 10)
	}
	if ndoc == "" && anno == "" {
		return "ordine_" + strconv.FormatInt(order.ID, 10)
	}
	if ndoc == "" {
		return anno
	}
	if anno == "" {
		return ndoc
	}
	return ndoc + "_" + anno
}
