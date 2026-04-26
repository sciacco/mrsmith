package rda

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/authz"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handlePermissions(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentClaims(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return
	}
	httputil.JSON(w, http.StatusOK, rdaPermissions{
		IsApprover:            authz.HasAnyRole(claims.Roles, applaunch.RDAApproverL1L2Roles()...),
		IsAFC:                 authz.HasAnyRole(claims.Roles, applaunch.RDAApproverAFCRoles()...),
		IsApproverNoLeasing:   authz.HasAnyRole(claims.Roles, applaunch.RDAApproverNoLeasingRoles()...),
		IsApproverExtraBudget: authz.HasAnyRole(claims.Roles, applaunch.RDAApproverExtraBudgetRoles()...),
	})
}

func applaunchRDAApproverL1L2Roles() []string {
	return applaunch.RDAApproverL1L2Roles()
}

func applaunchRDAApproverAFCRoles() []string {
	return applaunch.RDAApproverAFCRoles()
}

func applaunchRDAApproverNoLeasingRoles() []string {
	return applaunch.RDAApproverNoLeasingRoles()
}

func applaunchRDAApproverExtraBudgetRoles() []string {
	return applaunch.RDAApproverExtraBudgetRoles()
}
