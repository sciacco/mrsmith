package portal

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

var appCatalog []applaunch.Definition

func RegisterRoutes(mux *http.ServeMux, definitions []applaunch.Definition) {
	appCatalog = append([]applaunch.Definition(nil), definitions...)
	mux.HandleFunc("GET /portal/apps", handleListApps)
	mux.HandleFunc("GET /portal/me", handleMe)
}

func handleListApps(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"categories": applaunch.VisibleCategories(appCatalog, claims.Roles),
	})
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"name":  claims.Name,
		"email": claims.Email,
		"roles": claims.Roles,
	})
}
