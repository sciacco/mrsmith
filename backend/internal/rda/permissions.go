package rda

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

var errRDAPermissionsUnavailable = errors.New("rda permission store unavailable")

type rdaPermissionFlag string

const (
	permissionApprover          rdaPermissionFlag = "is_approver"
	permissionAFC               rdaPermissionFlag = "is_afc"
	permissionApproverNoLeasing rdaPermissionFlag = "is_approver_no_leasing"
	permissionExtraBudget       rdaPermissionFlag = "is_approver_extra_budget"
)

func (h *Handler) handlePermissions(w http.ResponseWriter, r *http.Request) {
	email, ok := currentEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return
	}
	permissions, ok := h.loadPermissionsForRequest(w, r, email)
	if !ok {
		return
	}
	httputil.JSON(w, http.StatusOK, permissions)
}

func (h *Handler) permissionsForEmail(ctx context.Context, email string) (rdaPermissions, error) {
	if h.arakDB == nil {
		return rdaPermissions{}, errRDAPermissionsUnavailable
	}
	email = strings.TrimSpace(email)
	var permissions rdaPermissions
	err := h.arakDB.QueryRowContext(ctx, `
		SELECT
			COALESCE(r.is_approver, false),
			COALESCE(r.is_afc, false),
			COALESCE(r.is_approver_no_leasing, false),
			COALESCE(r.is_approver_extra_budget, false)
		FROM users_int."user" u
		JOIN users_int.role r ON u.role = r.name
		WHERE u.email = $1
	`, email).Scan(
		&permissions.IsApprover,
		&permissions.IsAFC,
		&permissions.IsApproverNoLeasing,
		&permissions.IsApproverExtraBudget,
	)
	if err == nil {
		return permissions, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		logger := h.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("rda permissions not found for user", "email", strings.ToLower(email))
		return rdaPermissions{}, nil
	}
	return rdaPermissions{}, err
}

func (h *Handler) loadPermissionsForRequest(w http.ResponseWriter, r *http.Request, email string) (rdaPermissions, bool) {
	permissions, err := h.permissionsForEmail(r.Context(), email)
	if err == nil {
		return permissions, true
	}
	if errors.Is(err, errRDAPermissionsUnavailable) {
		httputil.JSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "Le approvazioni non sono disponibili in questo momento",
			"code":  codeDependencyUnavailable,
		})
		return rdaPermissions{}, false
	}
	h.requestLogger(r, "rda_permissions").Error("permission load failed", "error", err)
	httputil.JSON(w, http.StatusInternalServerError, map[string]string{
		"error": "Le approvazioni non sono disponibili in questo momento",
	})
	return rdaPermissions{}, false
}

func (p rdaPermissions) has(flag rdaPermissionFlag) bool {
	switch flag {
	case permissionApprover:
		return p.IsApprover
	case permissionAFC:
		return p.IsAFC
	case permissionApproverNoLeasing:
		return p.IsApproverNoLeasing
	case permissionExtraBudget:
		return p.IsApproverExtraBudget
	default:
		return false
	}
}
