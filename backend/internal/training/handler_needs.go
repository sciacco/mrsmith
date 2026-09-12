package training

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *handler) registerNeedRoutes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /training/v1/needs", protect(h.requireStore(http.HandlerFunc(h.handleListNeeds))))
	mux.Handle("POST /training/v1/needs", protect(h.requireStore(http.HandlerFunc(h.handleSaveNeed))))
	mux.Handle("GET /training/v1/needs/{id}", protect(h.requireStore(http.HandlerFunc(h.handleGetNeed))))
	mux.Handle("PUT /training/v1/needs/{id}", protect(h.requireStore(http.HandlerFunc(h.handleSaveNeed))))
	mux.Handle("DELETE /training/v1/needs/{id}", protect(h.requireStore(http.HandlerFunc(h.handleDeleteNeed))))
	mux.Handle("POST /training/v1/needs/{id}/courses", protect(h.requireStore(http.HandlerFunc(h.handleSaveNeedCandidate))))
	mux.Handle("PUT /training/v1/needs/{id}/courses/{courseId}", protect(h.requireStore(http.HandlerFunc(h.handleSaveNeedCandidate))))
	mux.Handle("DELETE /training/v1/needs/{id}/courses/{courseId}", protect(h.requireStore(http.HandlerFunc(h.handleRemoveNeedCandidate))))
	mux.Handle("POST /training/v1/needs/{id}/requests", protect(h.requireStore(http.HandlerFunc(h.handleAddNeedRequests))))
	mux.Handle("DELETE /training/v1/needs/{id}/requests/{requestId}", protect(h.requireStore(http.HandlerFunc(h.handleRemoveNeedRequest))))
	mux.Handle("POST /training/v1/needs/{id}/accept", protect(h.requireStore(http.HandlerFunc(h.handleAcceptNeed))))
}

func (h *handler) handleListNeeds(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	needs, err := h.store.ListNeeds(r.Context(), "")
	if err != nil {
		h.writeActionError(w, r, err, "training.list_needs")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"needs": needs})
}

func (h *handler) handleGetNeed(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	detail, err := h.store.GetNeedDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.get_need")
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *handler) handleSaveNeed(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[NeedInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.SaveNeed(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.save_need")
		return
	}
	status := http.StatusOK
	if r.Method == http.MethodPost {
		status = http.StatusCreated
	}
	httputil.JSON(w, status, response)
}

func (h *handler) handleDeleteNeed(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.DeleteNeed(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.delete_need")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleSaveNeedCandidate(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[NeedCandidateInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.SaveNeedCandidate(r.Context(), principal, r.PathValue("id"), r.PathValue("courseId"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.save_need_candidate")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleRemoveNeedCandidate(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.RemoveNeedCandidate(r.Context(), principal, r.PathValue("id"), r.PathValue("courseId"))
	if err != nil {
		h.writeActionError(w, r, err, "training.remove_need_candidate")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleAddNeedRequests(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[NeedRequestsInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.AddNeedRequests(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.add_need_requests")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleRemoveNeedRequest(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.RemoveNeedRequest(r.Context(), principal, r.PathValue("id"), r.PathValue("requestId"))
	if err != nil {
		h.writeActionError(w, r, err, "training.remove_need_request")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleAcceptNeed(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[NeedAcceptInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.AcceptNeed(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.accept_need")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}
