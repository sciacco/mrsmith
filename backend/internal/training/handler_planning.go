package training

import (
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *handler) handlePlanningSuggestions(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	year := time.Now().Year()
	if y := strings.TrimSpace(r.URL.Query().Get("year")); y != "" {
		if parsed, err := strconvAtoi(y); err == nil {
			year = parsed
		}
	}
	team := strings.TrimSpace(r.URL.Query().Get("team"))
	resp, err := h.store.PlanningOverview(r.Context(), principal, year, team)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning_overview")
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

func (h *handler) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[CreatePlanInput](w, r)
	if !ok {
		return
	}
	row, err := h.store.CreatePlan(r.Context(), principal, input)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_plan")
		return
	}
	httputil.JSON(w, http.StatusCreated, row)
}

func (h *handler) handleTransitionPlan(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[TransitionPlanInput](w, r)
	if !ok {
		return
	}
	resp, err := h.store.TransitionPlan(r.Context(), principal, r.PathValue("id"), input.Target)
	if err != nil {
		h.writeActionError(w, r, err, "training.transition_plan")
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

func (h *handler) handleBulkPlanFromSuggestion(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[BulkPlanFromSuggestionInput](w, r)
	if !ok {
		return
	}
	year := input.PlanParams.Year
	if year == 0 {
		year = time.Now().Year()
	}
	resp, err := h.store.BulkPlanFromSuggestion(r.Context(), principal, year, input)
	if err != nil {
		h.writeActionError(w, r, err, "training.bulk_plan_from_suggestion")
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

func (h *handler) handleBulkReviewEmployeeRequests(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[BulkReviewEmployeeRequestsInput](w, r)
	if !ok {
		return
	}
	year := time.Now().Year()
	if y := strings.TrimSpace(r.URL.Query().Get("year")); y != "" {
		if parsed, err := strconvAtoi(y); err == nil {
			year = parsed
		}
	}
	resp, err := h.store.BulkReviewEmployeeRequests(r.Context(), principal, year, input)
	if err != nil {
		h.writeActionError(w, r, err, "training.bulk_review_employee_requests")
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

func (h *handler) handleDismissSuggestion(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[DismissSuggestionInput](w, r)
	if !ok {
		return
	}
	if err := h.store.DismissSuggestion(r.Context(), principal, input.PlanID, r.PathValue("id")); err != nil {
		h.writeActionError(w, r, err, "training.dismiss_suggestion")
		return
	}
	httputil.JSON(w, http.StatusOK, OKResponse{OK: true})
}
