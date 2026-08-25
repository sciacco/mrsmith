package training

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// ── Evento ──

func (h *handler) handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[EventInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.CreateEvent(r.Context(), principal, input)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_event")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleUpdateEvent(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[EventInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.UpdateEvent(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.update_event")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleCancelEvent(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[ReasonInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.CancelEvent(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.cancel_event")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleListEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	events, err := h.store.ListEvents(r.Context())
	if err != nil {
		h.writeActionError(w, r, err, "training.list_events")
		return
	}
	httputil.JSON(w, http.StatusOK, EventListResponse{Events: events})
}

func (h *handler) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	detail, err := h.store.GetEventDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.get_event")
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

// ── Sessione ──

func (h *handler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[SessionInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.CreateSession(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_session")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleUpdateSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[SessionInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.UpdateSession(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.update_session")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.DeleteSession(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.delete_session")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

// ── Iscrizione ──

func (h *handler) handleCreateEventEnrollment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[EnrollmentCreateInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.CreateEventEnrollment(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_enrollment")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleUpdateEnrollmentFacts(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[EnrollmentFactsInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.UpdateEnrollmentFacts(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.update_enrollment_facts")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleCancelEnrollment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[ReasonInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.CancelEnrollment(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.cancel_enrollment")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleCompleteEnrollmentHistorical(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.CompleteEnrollmentHistorical(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.complete_enrollment_historical")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleReopenEnrollment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[ReasonInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.ReopenEnrollment(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.reopen_enrollment")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

// ── Partecipazione ──

func (h *handler) handleAssignEnrollmentSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.AssignEnrollmentSession(r.Context(), principal, r.PathValue("id"), r.PathValue("sessionId"))
	if err != nil {
		h.writeActionError(w, r, err, "training.assign_participation")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleRemoveEnrollmentSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.RemoveEnrollmentSession(r.Context(), principal, r.PathValue("id"), r.PathValue("sessionId"))
	if err != nil {
		h.writeActionError(w, r, err, "training.remove_participation")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleUpdateParticipation(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[ParticipationInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.UpdateParticipationStatus(r.Context(), principal, r.PathValue("id"), r.PathValue("sessionId"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.update_participation")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}
