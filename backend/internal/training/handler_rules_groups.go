package training

import (
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *handler) handleListMandatoryRules(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	rules, err := h.store.ListMandatoryRules(r.Context(), principal, mandatoryRuleFilters{
		Status:         strings.TrimSpace(r.URL.Query().Get("status")),
		PopulationKind: strings.TrimSpace(r.URL.Query().Get("population_kind")),
		Search:         strings.TrimSpace(r.URL.Query().Get("q")),
	})
	if err != nil {
		h.writeActionError(w, r, err, "training.list_mandatory_rules")
		return
	}
	httputil.JSON(w, http.StatusOK, MandatoryRulesResponse{Rules: rules})
}

func (h *handler) handleCreateMandatoryRuleV2(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[MandatoryRuleInputV2](w, r)
	if !ok {
		return
	}
	response, err := h.store.CreateMandatoryRule(r.Context(), principal, input)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_mandatory_rule")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleUpdateMandatoryRuleV2(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[MandatoryRuleInputV2](w, r)
	if !ok {
		return
	}
	response, err := h.store.UpdateMandatoryRule(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.update_mandatory_rule")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleDeleteMandatoryRuleV2(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteMandatoryRule(r.Context(), principal, r.PathValue("id")); err != nil {
		h.writeActionError(w, r, err, "training.delete_mandatory_rule")
		return
	}
	httputil.JSON(w, http.StatusOK, OKResponse{OK: true})
}

func (h *handler) handlePreviewMandatoryRuleImpact(w http.ResponseWriter, r *http.Request) {
	_, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	impact, err := h.store.MandatoryRuleImpact(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.preview_mandatory_rule")
		return
	}
	httputil.JSON(w, http.StatusOK, impact)
}

func (h *handler) handleListCustomGroups(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	groups, err := h.store.ListCustomGroups(
		r.Context(),
		principal,
		strings.TrimSpace(r.URL.Query().Get("status")),
		strings.TrimSpace(r.URL.Query().Get("q")),
	)
	if err != nil {
		h.writeActionError(w, r, err, "training.list_custom_groups")
		return
	}
	httputil.JSON(w, http.StatusOK, CustomGroupsResponse{Groups: groups})
}

func (h *handler) handleGetCustomGroup(w http.ResponseWriter, r *http.Request) {
	_, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	group, err := h.store.CustomGroupByID(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.get_custom_group")
		return
	}
	httputil.JSON(w, http.StatusOK, group)
}

func (h *handler) handleCreateCustomGroup(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[CustomGroupInput](w, r)
	if !ok {
		return
	}
	group, err := h.store.CreateCustomGroup(r.Context(), principal, input)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_custom_group")
		return
	}
	httputil.JSON(w, http.StatusCreated, group)
}

func (h *handler) handleUpdateCustomGroup(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[CustomGroupInput](w, r)
	if !ok {
		return
	}
	group, err := h.store.UpdateCustomGroup(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.update_custom_group")
		return
	}
	httputil.JSON(w, http.StatusOK, group)
}

func (h *handler) handleDeleteCustomGroup(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteCustomGroup(r.Context(), principal, r.PathValue("id")); err != nil {
		h.writeActionError(w, r, err, "training.delete_custom_group")
		return
	}
	httputil.JSON(w, http.StatusOK, OKResponse{OK: true})
}
