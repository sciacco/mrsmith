package portal

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /portal/apps", handleListApps)
	mux.HandleFunc("GET /portal/me", handleMe)
}

func handleListApps(w http.ResponseWriter, _ *http.Request) {
	// TODO: return list of apps the user has access to
	httputil.JSON(w, http.StatusOK, map[string]any{
		"apps": []any{},
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
