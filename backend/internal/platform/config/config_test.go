package config

import "testing"

func TestRDAQuoteThresholdConfig(t *testing.T) {
	t.Run("defaults when absent", func(t *testing.T) {
		t.Setenv("RDA_QUOTE_THRESHOLD", "")

		cfg := Load()

		if cfg.RDAQuoteThreshold != DefaultRDAQuoteThreshold {
			t.Fatalf("expected default threshold %.0f, got %.0f", DefaultRDAQuoteThreshold, cfg.RDAQuoteThreshold)
		}
	})

	t.Run("uses positive env value", func(t *testing.T) {
		t.Setenv("RDA_QUOTE_THRESHOLD", "4500")

		cfg := Load()

		if cfg.RDAQuoteThreshold != 4500 {
			t.Fatalf("expected threshold 4500, got %.0f", cfg.RDAQuoteThreshold)
		}
	})

	t.Run("falls back on invalid env value", func(t *testing.T) {
		t.Setenv("RDA_QUOTE_THRESHOLD", "-1")

		cfg := Load()

		if cfg.RDAQuoteThreshold != DefaultRDAQuoteThreshold {
			t.Fatalf("expected default threshold %.0f, got %.0f", DefaultRDAQuoteThreshold, cfg.RDAQuoteThreshold)
		}
	})
}

func TestSMTPConfigDefaults(t *testing.T) {
	clearSMTPEnv(t)

	cfg := Load()

	if cfg.SMTPEnabled {
		t.Fatalf("expected SMTP disabled by default")
	}
	if cfg.SMTPPort != "587" {
		t.Fatalf("expected default SMTP port 587, got %q", cfg.SMTPPort)
	}
	if cfg.SMTPTLSMode != "auto" {
		t.Fatalf("expected default SMTP TLS mode auto, got %q", cfg.SMTPTLSMode)
	}
	if cfg.SMTPTLSSkipVerify {
		t.Fatalf("expected SMTP TLS verification enabled by default")
	}
	if cfg.SMTPAuthMode != "auto" {
		t.Fatalf("expected default SMTP auth mode auto, got %q", cfg.SMTPAuthMode)
	}
}

func TestSMTPConfigFromEnv(t *testing.T) {
	t.Setenv("SMTP_ENABLED", "true")
	t.Setenv("SMTP_HOST", "10.0.0.5")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_USERNAME", "mailer")
	t.Setenv("SMTP_PASSWORD", "secret")
	t.Setenv("SMTP_FROM", "Portal <portal@example.com>")
	t.Setenv("SMTP_TLS_MODE", "starttls")
	t.Setenv("SMTP_TLS_SKIP_VERIFY", "true")
	t.Setenv("SMTP_TLS_SERVER_NAME", "smtp.internal.example.com")
	t.Setenv("SMTP_AUTH_MODE", "login")

	cfg := Load()

	if !cfg.SMTPEnabled {
		t.Fatalf("expected SMTP enabled from env")
	}
	if cfg.SMTPHost != "10.0.0.5" {
		t.Fatalf("expected SMTP host from env, got %q", cfg.SMTPHost)
	}
	if cfg.SMTPPort != "2525" {
		t.Fatalf("expected SMTP port from env, got %q", cfg.SMTPPort)
	}
	if cfg.SMTPUsername != "mailer" {
		t.Fatalf("expected SMTP username from env, got %q", cfg.SMTPUsername)
	}
	if cfg.SMTPPassword != "secret" {
		t.Fatalf("expected SMTP password from env, got %q", cfg.SMTPPassword)
	}
	if cfg.SMTPFrom != "Portal <portal@example.com>" {
		t.Fatalf("expected SMTP from from env, got %q", cfg.SMTPFrom)
	}
	if cfg.SMTPTLSMode != "starttls" {
		t.Fatalf("expected SMTP TLS mode from env, got %q", cfg.SMTPTLSMode)
	}
	if !cfg.SMTPTLSSkipVerify {
		t.Fatalf("expected SMTP TLS skip verify from env")
	}
	if cfg.SMTPTLSServerName != "smtp.internal.example.com" {
		t.Fatalf("expected SMTP TLS server name from env, got %q", cfg.SMTPTLSServerName)
	}
	if cfg.SMTPAuthMode != "login" {
		t.Fatalf("expected SMTP auth mode from env, got %q", cfg.SMTPAuthMode)
	}
}

func TestKeycloakAdminConfigDerivesFromFrontendConfig(t *testing.T) {
	clearKeycloakAdminEnv(t)
	t.Setenv("KEYCLOAK_ISSUER_URL", "")
	t.Setenv("KEYCLOAK_FRONTEND_URL", "https://keycloak.example.com/")
	t.Setenv("KEYCLOAK_FRONTEND_REALM", "mrsmith-dev")

	cfg := Load()

	if cfg.KeycloakAdminBaseURL != "https://keycloak.example.com" {
		t.Fatalf("expected admin base URL from frontend URL, got %q", cfg.KeycloakAdminBaseURL)
	}
	if cfg.KeycloakAdminRealm != "mrsmith-dev" {
		t.Fatalf("expected admin realm from frontend realm, got %q", cfg.KeycloakAdminRealm)
	}
	if cfg.KeycloakAdminTokenURL != "https://keycloak.example.com/realms/mrsmith-dev/protocol/openid-connect/token" {
		t.Fatalf("unexpected admin token URL %q", cfg.KeycloakAdminTokenURL)
	}
}

func TestKeycloakAdminConfigDerivesFromIssuerConfig(t *testing.T) {
	clearKeycloakAdminEnv(t)
	t.Setenv("KEYCLOAK_ISSUER_URL", "https://keycloak.example.com/auth/realms/mrsmith-dev")
	t.Setenv("KEYCLOAK_FRONTEND_URL", "")
	t.Setenv("KEYCLOAK_FRONTEND_REALM", "")

	cfg := Load()

	if cfg.KeycloakAdminBaseURL != "https://keycloak.example.com/auth" {
		t.Fatalf("expected admin base URL from issuer URL, got %q", cfg.KeycloakAdminBaseURL)
	}
	if cfg.KeycloakAdminRealm != "mrsmith-dev" {
		t.Fatalf("expected admin realm from issuer URL, got %q", cfg.KeycloakAdminRealm)
	}
	if cfg.KeycloakAdminTokenURL != "https://keycloak.example.com/auth/realms/mrsmith-dev/protocol/openid-connect/token" {
		t.Fatalf("unexpected admin token URL %q", cfg.KeycloakAdminTokenURL)
	}
}

func TestKeycloakAdminConfigUsesExplicitOverrides(t *testing.T) {
	clearKeycloakAdminEnv(t)
	t.Setenv("KEYCLOAK_ISSUER_URL", "https://issuer.example.com/realms/issuer")
	t.Setenv("KEYCLOAK_FRONTEND_URL", "https://frontend.example.com")
	t.Setenv("KEYCLOAK_FRONTEND_REALM", "frontend")
	t.Setenv("KEYCLOAK_ADMIN_BASE_URL", "https://admin.example.com/keycloak")
	t.Setenv("KEYCLOAK_ADMIN_REALM", "admin")
	t.Setenv("KEYCLOAK_ADMIN_TOKEN_URL", "https://tokens.example.com/custom/token")
	t.Setenv("KEYCLOAK_ADMIN_CLIENT_ID", "resolver")
	t.Setenv("KEYCLOAK_ADMIN_CLIENT_SECRET", "resolver-secret")

	cfg := Load()

	if cfg.KeycloakAdminBaseURL != "https://admin.example.com/keycloak" {
		t.Fatalf("expected explicit admin base URL, got %q", cfg.KeycloakAdminBaseURL)
	}
	if cfg.KeycloakAdminRealm != "admin" {
		t.Fatalf("expected explicit admin realm, got %q", cfg.KeycloakAdminRealm)
	}
	if cfg.KeycloakAdminTokenURL != "https://tokens.example.com/custom/token" {
		t.Fatalf("expected explicit admin token URL, got %q", cfg.KeycloakAdminTokenURL)
	}
	if cfg.KeycloakAdminClientID != "resolver" {
		t.Fatalf("expected admin client id, got %q", cfg.KeycloakAdminClientID)
	}
	if cfg.KeycloakAdminClientSecret != "resolver-secret" {
		t.Fatalf("expected admin client secret, got %q", cfg.KeycloakAdminClientSecret)
	}
}

func clearSMTPEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"SMTP_ENABLED",
		"SMTP_HOST",
		"SMTP_PORT",
		"SMTP_USERNAME",
		"SMTP_PASSWORD",
		"SMTP_FROM",
		"SMTP_TLS_MODE",
		"SMTP_TLS_SKIP_VERIFY",
		"SMTP_TLS_SERVER_NAME",
		"SMTP_AUTH_MODE",
	} {
		t.Setenv(key, "")
	}
}

func clearKeycloakAdminEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"KEYCLOAK_ISSUER_URL",
		"KEYCLOAK_FRONTEND_URL",
		"KEYCLOAK_FRONTEND_REALM",
		"KEYCLOAK_ADMIN_BASE_URL",
		"KEYCLOAK_ADMIN_REALM",
		"KEYCLOAK_ADMIN_TOKEN_URL",
		"KEYCLOAK_ADMIN_CLIENT_ID",
		"KEYCLOAK_ADMIN_CLIENT_SECRET",
	} {
		t.Setenv(key, "")
	}
}
