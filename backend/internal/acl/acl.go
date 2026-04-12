package acl

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/authz"
)

// RequireRole returns middleware that checks if the user has one of the required roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.GetClaims(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if authz.HasAnyRole(claims.Roles, roles...) {
				next.ServeHTTP(w, r)
				return
			}

			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}
