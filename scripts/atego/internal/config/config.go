// Package config legge e valida le variabili d'ambiente.
// È la traduzione di src/ingest/config.py.
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
	QdrantURL        string
	QdrantAPIKey     string // "" se assente
	CollectionPrefix string
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
	qdrantURL := os.Getenv("QDRANT_URL")
	qdrantAPIKey := os.Getenv("QDRANT_API_KEY")
	prefix := os.Getenv("QDRANT_COLLECTION_PREFIX")

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
	if qdrantURL == "" {
		errs = append(errs, "QDRANT_URL è obbligatorio")
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("configurazione incompleta:\n%s", strings.Join(errs, "\n"))
	}

	return &Config{
		EmbeddingAPIBase: apiBase,
		EmbeddingAPIKey:  apiKey,
		EmbeddingModel:   model,
		QdrantURL:        qdrantURL,
		QdrantAPIKey:     qdrantAPIKey,
		CollectionPrefix: prefix,
		BatchSize:        32,
		MaxRetries:       5,
		RequestTimeout:   60 * time.Second,
	}, nil
}

// CollectionName applica il prefisso come config.py::collection_name.
func (c *Config) CollectionName(base string) string {
	if c.CollectionPrefix != "" {
		return c.CollectionPrefix + base
	}
	return base
}
