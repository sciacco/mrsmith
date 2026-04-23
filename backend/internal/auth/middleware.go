package auth

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

type contextKey string

const ClaimsKey contextKey = "claims"

type Claims struct {
	Subject  string
	Email    string
	Name     string
	Roles    []string
	RawToken string
}

type Middleware struct {
	verifier       *oidc.IDTokenVerifier
	noopFakeClaims *Claims
}

func NewMiddleware(issuerURL string) (*Middleware, error) {
	provider, err := oidc.NewProvider(context.Background(), issuerURL)
	if err != nil {
		return nil, err
	}
	verifier := provider.Verifier(&oidc.Config{SkipClientIDCheck: true})
	return &Middleware{verifier: verifier}, nil
}

// NewNoopMiddleware returns a middleware that skips token validation
// and injects fake claims. Only for local development.
//
// Roles injected into the fake claims are resolved in this order:
//  1. DEV_FAKE_ROLES env var (comma-separated), if set.
//  2. DEV_FAKE_FULL_ACCESS=true (default) → applaunch.AllRoles() so that
//     every mini-app endpoint is reachable.
//  3. Fallback: {"admin", "manager"} for backward compatibility with
//     callers that never set the env var.
//
// DEV_FAKE_SUBJECT, DEV_FAKE_EMAIL, DEV_FAKE_NAME override the identity
// fields when set.
func NewNoopMiddleware() *Middleware {
	claims := Claims{
		Subject:  envOr("DEV_FAKE_SUBJECT", "dev-user-001"),
		Email:    envOr("DEV_FAKE_EMAIL", "john.doe@acme.com"),
		Name:     envOr("DEV_FAKE_NAME", "John Doe"),
		Roles:    resolveNoopRoles(),
		RawToken: "dev-token",
	}
	return &Middleware{verifier: nil, noopFakeClaims: &claims}
}

func envOr(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func resolveNoopRoles() []string {
	if raw := strings.TrimSpace(os.Getenv("DEV_FAKE_ROLES")); raw != "" {
		parts := strings.Split(raw, ",")
		roles := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				roles = append(roles, trimmed)
			}
		}
		return roles
	}
	// Default: inject every app_* role so the dev user can exercise any
	// mini-app endpoint. Keep legacy "admin"/"manager" for anything that
	// still checks for them.
	roles := applaunch.AllRoles()
	return append(roles, "admin", "manager")
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := logging.FromContext(r.Context()).With("component", "auth")
		// Dev mode: skip token validation, inject fake claims
		if m.verifier == nil {
			claims := *m.noopFakeClaims
			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			ctx = logging.WithAttrs(ctx, "auth_subject", claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			logger.Warn("authentication failed", "reason", "missing_bearer")
			http.Error(w, "missing or invalid authorization header", http.StatusUnauthorized)
			return
		}
		rawToken := strings.TrimPrefix(authHeader, "Bearer ")

		idToken, err := m.verifier.Verify(r.Context(), rawToken)
		if err != nil {
			logger.Warn("authentication failed", "reason", "token_verify_failed", "error", err)
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		var tokenClaims struct {
			Email             string `json:"email"`
			PreferredUsername string `json:"preferred_username"`
			RealmAccess       struct {
				Roles []string `json:"roles"`
			} `json:"realm_access"`
		}
		if err := idToken.Claims(&tokenClaims); err != nil {
			logger.Warn("authentication failed", "reason", "claims_parse_failed", "error", err)
			http.Error(w, "failed to parse claims", http.StatusInternalServerError)
			return
		}

		claims := Claims{
			Subject:  idToken.Subject,
			Email:    tokenClaims.Email,
			Name:     tokenClaims.PreferredUsername,
			Roles:    tokenClaims.RealmAccess.Roles,
			RawToken: rawToken,
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		ctx = logging.WithAttrs(ctx, "auth_subject", claims.Subject)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetClaims(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(ClaimsKey).(Claims)
	return claims, ok
}
