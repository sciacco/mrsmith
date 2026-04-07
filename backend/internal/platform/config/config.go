package config

import "os"

type Config struct {
	Port              string
	KeycloakIssuerURL string
	CORSOrigins       string
	StaticDir         string

	// Optional launcher override for split-server local development.
	BudgetAppURL     string
	ComplianceAppURL string

	// Anisetta PostgreSQL (compliance module)
	AnisettaDSN string

	// Frontend Keycloak (public client, no secret — served to browser via GET /config)
	KeycloakFrontendURL      string
	KeycloakFrontendRealm    string
	KeycloakFrontendClientId string

	// Arak / MISTRA-NG API proxy (service account client credentials)
	ArakBaseURL         string
	ArakServiceClientID string
	ArakServiceSecret   string
	ArakServiceTokenURL string
}

func Load() Config {
	return Config{
		Port:              envOr("PORT", "8080"),
		KeycloakIssuerURL: envOr("KEYCLOAK_ISSUER_URL", ""),
		CORSOrigins:       envOr("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174,http://localhost:5175"),
		StaticDir:         envOr("STATIC_DIR", ""),
		BudgetAppURL:      envOr("BUDGET_APP_URL", ""),
		ComplianceAppURL:  envOr("COMPLIANCE_APP_URL", ""),
		AnisettaDSN:       envOr("ANISETTA_DSN", ""),

		KeycloakFrontendURL:      envOr("KEYCLOAK_FRONTEND_URL", ""),
		KeycloakFrontendRealm:    envOr("KEYCLOAK_FRONTEND_REALM", ""),
		KeycloakFrontendClientId: envOr("KEYCLOAK_FRONTEND_CLIENT_ID", ""),

		ArakBaseURL:         envOr("ARAK_BASE_URL", ""),
		ArakServiceClientID: envOr("ARAK_SERVICE_CLIENT_ID", ""),
		ArakServiceSecret:   envOr("ARAK_SERVICE_CLIENT_SECRET", ""),
		ArakServiceTokenURL: envOr("ARAK_SERVICE_TOKEN_URL", ""),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
