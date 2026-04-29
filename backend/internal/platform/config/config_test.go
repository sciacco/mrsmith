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
