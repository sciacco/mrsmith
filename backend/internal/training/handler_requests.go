package training

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// registerRequestRoutes registra le route delle richieste formative (#140;
// #171 per la modifica dei dati originali).
func (h *handler) registerRequestRoutes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /training/v1/requests", protect(h.requireStore(http.HandlerFunc(h.handleListRequests))))
	mux.Handle("POST /training/v1/requests", protect(h.requireStore(http.HandlerFunc(h.handleCreateRequest))))
	mux.Handle("GET /training/v1/requests/{id}", protect(h.requireStore(http.HandlerFunc(h.handleGetRequest))))
	mux.Handle("POST /training/v1/requests/{id}/tl-opinion", protect(h.requireStore(http.HandlerFunc(h.handleRecordTLOpinion))))
	mux.Handle("POST /training/v1/requests/{id}/decision", protect(h.requireStore(http.HandlerFunc(h.handleRecordPeopleDecision))))
	mux.Handle("POST /training/v1/requests/{id}/withdraw", protect(h.requireStore(http.HandlerFunc(h.handleWithdrawRequest))))
	mux.Handle("PUT /training/v1/requests/{id}/annotations", protect(h.requireStore(http.HandlerFunc(h.handleUpdateRequestAnnotations))))
	mux.Handle("PUT /training/v1/requests/{id}/original", protect(h.requireStore(http.HandlerFunc(h.handleUpdateRequestOriginal))))
	mux.Handle("POST /training/v1/requests/{id}/suspend", protect(h.requireStore(http.HandlerFunc(h.handleSuspendRequest))))
	mux.Handle("POST /training/v1/requests/{id}/resume", protect(h.requireStore(http.HandlerFunc(h.handleResumeRequest))))
}

func (h *handler) handleUpdateRequestOriginal(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[RequestOriginalDataInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.UpdateRequestOriginalData(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.request_original")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleUpdateRequestAnnotations(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[RequestAnnotationsInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.UpdateRequestAnnotations(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.request_annotations")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleSuspendRequest(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[ReasonInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.SuspendRequest(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.suspend_request")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleResumeRequest(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.ResumeRequest(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.resume_request")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleListRequests(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	requests, err := h.store.ListRequests(r.Context(), r.URL.Query().Get("state"))
	if err != nil {
		h.writeActionError(w, r, err, "training.list_requests")
		return
	}
	httputil.JSON(w, http.StatusOK, RequestListResponse{Requests: requests})
}

func (h *handler) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[RequestInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.CreateRequest(r.Context(), principal, input)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_request")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	detail, err := h.store.GetRequestDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.get_request")
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *handler) handleRecordTLOpinion(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[TLOpinionInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.RecordTLOpinion(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.record_tl_opinion")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleRecordPeopleDecision(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[RequestDecisionInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.RecordPeopleDecision(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.record_people_decision")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleWithdrawRequest(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	// Il ritiro non ha un corpo: l'esito e la chiusura sono impliciti.
	response, err := h.store.WithdrawRequest(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.withdraw_request")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}
