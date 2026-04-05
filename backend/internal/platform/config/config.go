package config

import "os"

type Config struct {
	Port              string
	KeycloakIssuerURL string
	CORSOrigins       string
	StaticDir         string

	// Frontend Keycloak (public client, no secret — served to browser via GET /config)
	KeycloakFrontendURL      string
	KeycloakFrontendRealm    string
	KeycloakFrontendClientId string
}

func Load() Config {
	return Config{
		Port:              envOr("PORT", "8080"),
		KeycloakIssuerURL: envOr("KEYCLOAK_ISSUER_URL", ""),
		CORSOrigins:       envOr("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174"),
		StaticDir:         envOr("STATIC_DIR", ""),

		KeycloakFrontendURL:      envOr("KEYCLOAK_FRONTEND_URL", ""),
		KeycloakFrontendRealm:    envOr("KEYCLOAK_FRONTEND_REALM", ""),
		KeycloakFrontendClientId: envOr("KEYCLOAK_FRONTEND_CLIENT_ID", ""),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
