package training

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const (
	defaultAuditLimit = 50
	maxAuditLimit     = 200
)

// registerAuditRoutes registra il pannello storia (#164, slice 5 del task 7,
// #159): un'unica route di sola lettura su training.audit_log, in tre
// modalita' di selezione mutuamente esclusive — entita' singola (regola,
// richiesta...), persona aggregata (RATIFICATO) o evento aggregato (forma
// scelta per l'evento in questa slice, figli vivi soltanto).
func (h *handler) registerAuditRoutes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /training/v1/audit", protect(h.requireStore(http.HandlerFunc(h.handleAuditHistory))))
}

func (h *handler) handleAuditHistory(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	limit, err := parseAuditLimit(r.URL.Query().Get("limit"))
	if err != nil {
		h.writeActionError(w, r, err, "training.audit_history")
		return
	}

	query := r.URL.Query()
	employeeID := strings.TrimSpace(query.Get("employeeId"))
	eventID := strings.TrimSpace(query.Get("eventId"))
	entityType := strings.TrimSpace(query.Get("entityType"))
	entityID := strings.TrimSpace(query.Get("entityId"))

	var entries []AuditEntry
	switch {
	case employeeID != "":
		entries, err = h.store.AuditHistoryByEmployee(r.Context(), employeeID, limit)
	case eventID != "":
		entries, err = h.store.AuditHistoryByEvent(r.Context(), eventID, limit)
	case entityType != "" && entityID != "":
		if !auditEntityTypes[entityType] {
			h.writeActionError(w, r, validationError("invalid_entity_type", "tipo entità non tracciato in cronologia"), "training.audit_history")
			return
		}
		entries, err = h.store.AuditHistoryByEntity(r.Context(), entityType, entityID, limit)
	default:
		h.writeActionError(w, r, validationError("missing_selector", "specificare entityType ed entityId, employeeId o eventId"), "training.audit_history")
		return
	}
	if err != nil {
		h.writeActionError(w, r, err, "training.audit_history")
		return
	}
	httputil.JSON(w, http.StatusOK, AuditHistoryResponse{Entries: entries})
}

func parseAuditLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultAuditLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 {
		return 0, validationError("invalid_limit", "limite non valido")
	}
	if limit > maxAuditLimit {
		limit = maxAuditLimit
	}
	return limit, nil
}
