package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
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
	verifier *oidc.IDTokenVerifier
}

func NewMiddleware(issuerURL string) (*Middleware, error) {
	provider, err := oidc.NewProvider(context.Background(), issuerURL)
	if err != nil {
		return nil, err
	}
	verifier := provider.Verifier(&oidc.Config{SkipClientIDCheck: true})
	return &Middleware{verifier: verifier}, nil
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "missing or invalid authorization header", http.StatusUnauthorized)
			return
		}
		rawToken := strings.TrimPrefix(authHeader, "Bearer ")

		idToken, err := m.verifier.Verify(r.Context(), rawToken)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		var tokenClaims struct {
			Email             string   `json:"email"`
			PreferredUsername string   `json:"preferred_username"`
			RealmAccess       struct {
				Roles []string `json:"roles"`
			} `json:"realm_access"`
		}
		if err := idToken.Claims(&tokenClaims); err != nil {
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
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetClaims(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(ClaimsKey).(Claims)
	return claims, ok
}
