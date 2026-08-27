package training

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// registerBulkRoutes registra i tre endpoint massivi del workspace (#153,
// slice 2 del task 6): assegnazioni, presenze e iscrizioni. Ogni chiamata e
// una singola transazione sul nucleo gia esistente della singola operazione.
func (h *handler) registerBulkRoutes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("POST /training/v1/events/{id}/bulk-assignments", protect(h.requireStore(http.HandlerFunc(h.handleBulkAssignments))))
	mux.Handle("POST /training/v1/sessions/{id}/bulk-participation", protect(h.requireStore(http.HandlerFunc(h.handleBulkParticipation))))
	mux.Handle("POST /training/v1/events/{id}/enrollments/bulk", protect(h.requireStore(http.HandlerFunc(h.handleBulkEnrollments))))
}

func (h *handler) handleBulkAssignments(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[BulkAssignmentsInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.BulkAssignSessions(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.bulk_assignments")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleBulkParticipation(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[BulkParticipationInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.BulkUpdateParticipation(r.Context(), principal, r.PathValue("id"), input, h.enrollmentStartGate(principal.Email))
	if err != nil {
		h.writeActionError(w, r, err, "training.bulk_participation")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleBulkEnrollments(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[BulkEnrollInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.BulkCreateEventEnrollments(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.bulk_enrollments")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}
