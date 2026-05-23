package ordini

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/authz"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) hasCustomerRelations(r *http.Request) bool {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		return false
	}
	return authz.HasAnyRole(claims.Roles, applaunch.CustomerRelationsRoles()...)
}

func (h *Handler) requireCustomerRelations(w http.ResponseWriter, r *http.Request) bool {
	if h.hasCustomerRelations(r) {
		return true
	}
	httputil.Error(w, http.StatusForbidden, "role_insufficient")
	return false
}

func requireState(w http.ResponseWriter, actual OrderState, allowed ...OrderState) bool {
	for _, state := range allowed {
		if actual == state {
			return true
		}
	}
	httputil.Error(w, http.StatusConflict, "wrong_state")
	return false
}

func canShowArxivarFilePicker(order *OrderDetail) bool {
	state := stateOf(order)
	return state != OrderStateAnnullato && state != OrderStatePerso && state != OrderStateAttivo
}
