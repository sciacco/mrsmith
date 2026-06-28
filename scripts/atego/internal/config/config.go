// Package config legge e valida le variabili d'ambiente.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config contiene la configurazione del tool atego.
type Config struct {
	EmbeddingAPIBase string
	EmbeddingAPIKey  string
	EmbeddingModel   string
	DatabaseURL      string // DSN Postgres (Anisetta: schemi mrsmith + binocolo)
	BatchSize        int
	MaxRetries       int
	RequestTimeout   time.Duration
}

// Load carica la configurazione da variabili d'ambiente (e da .env se presente).
// envFile == "" → usa ".env" di default; errori di lettura del file sono ignorati.
func Load(envFile string) (*Config, error) {
	if envFile == "" {
		_ = godotenv.Load(".env") // ignore error se manca
	} else {
		_ = godotenv.Load(envFile)
	}

	apiBase := strings.TrimRight(os.Getenv("EMBEDDING_API_BASE"), "/")
	apiKey := os.Getenv("EMBEDDING_API_KEY")
	model := os.Getenv("EMBEDDING_MODEL")
	dsn := os.Getenv("DATABASE_URL")

	var errs []string
	if apiBase == "" {
		errs = append(errs, "EMBEDDING_API_BASE è obbligatorio")
	}
	if apiKey == "" {
		errs = append(errs, "EMBEDDING_API_KEY è obbligatoria")
	}
	if model == "" {
		errs = append(errs, "EMBEDDING_MODEL è obbligatorio")
	}
	if dsn == "" {
		errs = append(errs, "DATABASE_URL è obbligatorio (DSN Postgres Anisetta)")
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("configurazione incompleta:\n%s", strings.Join(errs, "\n"))
	}

	return &Config{
		EmbeddingAPIBase: apiBase,
		EmbeddingAPIKey:  apiKey,
		EmbeddingModel:   model,
		DatabaseURL:      dsn,
		BatchSize:        32,
		MaxRetries:       5,
		RequestTimeout:   60 * time.Second,
	}, nil
}
