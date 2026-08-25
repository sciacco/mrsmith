package training

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// registerRuleRoutes registra le route delle regole formative (#140).
func (h *handler) registerRuleRoutes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /training/v1/rules", protect(h.requireStore(http.HandlerFunc(h.handleListRules))))
	mux.Handle("POST /training/v1/rules", protect(h.requireStore(http.HandlerFunc(h.handleCreateRule))))
	mux.Handle("GET /training/v1/rules/{id}", protect(h.requireStore(http.HandlerFunc(h.handleGetRule))))
	mux.Handle("PUT /training/v1/rules/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpdateRule))))
	mux.Handle("POST /training/v1/rules/{id}/activate", protect(h.requireStore(http.HandlerFunc(h.handleActivateRule))))
	mux.Handle("POST /training/v1/rules/{id}/deactivate", protect(h.requireStore(http.HandlerFunc(h.handleDeactivateRule))))
	mux.Handle("POST /training/v1/rules/{id}/events", protect(h.requireStore(http.HandlerFunc(h.handleCreateRuleEvent))))
	mux.Handle("POST /training/v1/events/{id}/feed", protect(h.requireStore(http.HandlerFunc(h.handleFeedRuleEvent))))
}

func (h *handler) handleListRules(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	rules, err := h.store.ListRules(r.Context())
	if err != nil {
		h.writeActionError(w, r, err, "training.list_rules")
		return
	}
	httputil.JSON(w, http.StatusOK, RuleListResponse{Rules: rules})
}

func (h *handler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[RuleInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.CreateRule(r.Context(), principal, input)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_rule")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleGetRule(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	detail, err := h.store.GetRuleDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.get_rule")
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *handler) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[RuleInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.UpdateRule(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.update_rule")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleActivateRule(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.SetRuleActive(r.Context(), principal, r.PathValue("id"), true)
	if err != nil {
		h.writeActionError(w, r, err, "training.activate_rule")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleDeactivateRule(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.SetRuleActive(r.Context(), principal, r.PathValue("id"), false)
	if err != nil {
		h.writeActionError(w, r, err, "training.deactivate_rule")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleCreateRuleEvent(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.CreateRuleEvent(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.create_rule_event")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleFeedRuleEvent(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.FeedRuleEvent(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.feed_rule_event")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}
