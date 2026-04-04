package config

import "os"

type Config struct {
	Port              string
	KeycloakIssuerURL string
	CORSOrigins       string
	StaticDir         string
}

func Load() Config {
	return Config{
		Port:              envOr("PORT", "8080"),
		KeycloakIssuerURL: envOr("KEYCLOAK_ISSUER_URL", ""),
		CORSOrigins:       envOr("CORS_ORIGINS", "http://localhost:5173"),
		StaticDir:         envOr("STATIC_DIR", ""),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
