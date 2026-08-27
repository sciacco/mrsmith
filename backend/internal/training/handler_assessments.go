package training

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// registerAssessmentRoutes registra le route delle valutazioni di
// competenza per area (#161, slice 2 del task 7).
func (h *handler) registerAssessmentRoutes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("POST /training/v1/people/{id}/assessments", protect(h.requireStore(http.HandlerFunc(h.handleCreateAssessment))))
	mux.Handle("PUT /training/v1/assessments/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpdateAssessment))))
	mux.Handle("DELETE /training/v1/assessments/{id}", protect(h.requireStore(http.HandlerFunc(h.handleDeleteAssessment))))
}

func (h *handler) handleCreateAssessment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[AssessmentInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.CreateAssessment(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_assessment")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleUpdateAssessment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[AssessmentUpdateInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.UpdateAssessment(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.update_assessment")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleDeleteAssessment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.DeleteAssessment(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.delete_assessment")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}
