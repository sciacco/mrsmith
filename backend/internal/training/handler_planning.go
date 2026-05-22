package training

import (
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/keycloak"
)

func (h *handler) handleListPlans(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	resp, err := h.store.ListTrainingPlansForPlanning(r.Context(), principal, status)
	if err != nil {
		h.writeActionError(w, r, err, "training.list_plans")
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

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

func (h *handler) handleUpdatePlan(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[UpdatePlanInput](w, r)
	if !ok {
		return
	}
	resp, err := h.store.UpdatePlan(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.update_plan")
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

func (h *handler) handleDeletePlan(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	if err := h.store.DeletePlan(r.Context(), principal, r.PathValue("id")); err != nil {
		h.writeActionError(w, r, err, "training.delete_plan")
		return
	}
	httputil.JSON(w, http.StatusOK, OKResponse{OK: true})
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

func (h *handler) handlePlanAudit(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconvAtoi(raw); err == nil {
			limit = parsed
		}
	}
	page, err := h.store.ListPlanAuditEvents(r.Context(), principal, r.PathValue("id"), limit, r.URL.Query().Get("before"))
	if err != nil {
		h.writeActionError(w, r, err, "training.plan_audit")
		return
	}
	h.resolvePlanAuditActors(r, page.events)
	httputil.JSON(w, http.StatusOK, PlanAuditResponse{
		Events:     page.events,
		NextCursor: page.nextCursor,
	})
}

func (h *handler) resolvePlanAuditActors(r *http.Request, events []PlanAuditEvent) {
	if h.roleResolver == nil || len(events) == 0 {
		return
	}
	wanted := make(map[string]struct{})
	for _, event := range events {
		if strings.TrimSpace(event.Actor.ID) != "" {
			wanted[event.Actor.ID] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return
	}

	names := make(map[string]string, len(wanted))
	for _, role := range applaunch.TrainingPeopleAdminRoles() {
		users, err := h.roleResolver.UsersByRealmRole(r.Context(), role, keycloak.UsersByRealmRoleOptions{PageSize: 100})
		if err != nil {
			return
		}
		for _, user := range users {
			id := strings.TrimSpace(user.ID)
			if _, ok := wanted[id]; !ok {
				continue
			}
			name := strings.TrimSpace(user.Name)
			if name == "" {
				name = strings.TrimSpace(strings.Join([]string{user.FirstName, user.LastName}, " "))
			}
			if name == "" {
				name = strings.TrimSpace(user.Username)
			}
			if name == "" {
				name = id
			}
			names[id] = name
		}
	}
	for i := range events {
		if name := names[events[i].Actor.ID]; name != "" {
			events[i].Actor.DisplayName = name
		}
	}
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
