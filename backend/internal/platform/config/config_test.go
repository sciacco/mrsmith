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
