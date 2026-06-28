// Package manifest scrive il file di tracciabilità data/manifest.json.
// È la traduzione di src/ingest/manifest.py.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"time"
)

// SourceInfo descrive un file sorgente.
type SourceInfo struct {
	Filename     string `json:"filename"`
	Sha256       string `json:"sha256"`
	ElementCount int    `json:"element_count"`
}

// Manifest è il documento di tracciabilità.
type Manifest struct {
	EmbeddingModel      string       `json:"embedding_model"`
	EmbeddingDimension  int          `json:"embedding_dimension"`
	Distance            string       `json:"distance"`
	ProviderBaseURL     string       `json:"provider_base_url"`
	CreatedAt           string       `json:"created_at"`
	Sources             []SourceInfo `json:"sources"`
	Collections         []string     `json:"collections"`
	QdrantClientVersion string       `json:"qdrant_client_version"`
	OpenAIClientVersion string       `json:"openai_client_version"`
}

// New costruisce un Manifest con valori di default (distance=Cosine,
// created_at=UTC RFC3339, versioni delle librerie).
func New(model string, dim int, baseURL string, sources []SourceInfo, collections []string) *Manifest {
	return &Manifest{
		EmbeddingModel:      model,
		EmbeddingDimension:  dim,
		Distance:            "Cosine",
		ProviderBaseURL:     baseURL,
		CreatedAt:           time.Now().UTC().Format(time.RFC3339),
		Sources:             sources,
		Collections:         collections,
		QdrantClientVersion: qdrantGoClientVersion(),
		OpenAIClientVersion: "net/http (no SDK)",
	}
}

// Write serializza il manifest in JSON indentato con chiavi ordinate
// (parità con sort_keys=True del Python) e UTF-8 raw, e lo scrive su path.
func Write(path string, m *Manifest) error {
	// Marshalla su map[string]any per forzare chiavi ordinate (encoding/json
	// ordina le chiavi di map). Poi ri-marshalla indentato.
	raw, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	var ordered map[string]any
	if err := json.Unmarshal(raw, &ordered); err != nil {
		return fmt.Errorf("reorder manifest: %w", err)
	}
	out, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return fmt.Errorf("indent manifest: %w", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// qdrantGoClientVersion estrae la versione del modulo github.com/qdrant/go-client
// dalle build info, con fallback a stringa vuota.
func qdrantGoClientVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, dep := range bi.Deps {
		if dep.Path == "github.com/qdrant/go-client" {
			return dep.Version
		}
	}
	return ""
}
