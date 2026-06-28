// Package sources esegue il parsing dei file JSON sorgente e fornisce
// utility per ID deterministici (UUIDv5) e hash. È la traduzione di
// src/ingest/sources.py.
package sources

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
)

// ModuleNamespace è l'UUIDv5 namespace fisso per i point id.
// Deve coincidere con Python: uuid.uuid5(uuid.NAMESPACE_DNS, "embed-mrsmith.local").
// uuid.NameSpaceDNS in google/uuid == Python uuid.NAMESPACE_DNS.
var ModuleNamespace = uuid.NewSHA1(uuid.NameSpaceDNS, []byte("embed-mrsmith.local"))

// AtecoNode rappresenta un nodo ATECO.
type AtecoNode struct {
	Codice        string
	EmbeddingText string
	Titolo        string
	Gerarchia     int // nel JSON è un intero
	RelevanceTier string
	Classe4Cifre  string
	Keywords      []string
	SeedConcepts  []string
	PathText      string
}

// BusinessConcept rappresenta un concetto business.
type BusinessConcept struct {
	ID                      string
	EmbeddingText           string
	Name                    string
	Domain                  string
	Kind                    string // "target" (default) | "distractor"
	Aliases                 []string
	AtecoCandidatesInKB     []string
	AtecoCandidatesExcluded []string
}

// SourceTextHash calcola SHA-256 (hex) del testo da embeddare.
func SourceTextHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// PointIDAteco restituisce l'UUID deterministico per un nodo ATECO.
func PointIDAteco(codice string) string {
	return uuid.NewSHA1(ModuleNamespace, []byte("ateco:"+codice)).String()
}

// PointIDConcept restituisce l'UUID deterministico per un concetto.
func PointIDConcept(id string) string {
	return uuid.NewSHA1(ModuleNamespace, []byte("concept:"+id)).String()
}

// LoadAtecoNodes legge il file ATECO e restituisce (nodi, sha256 del file).
func LoadAtecoNodes(path string) ([]AtecoNode, string, error) {
	fileHash, raw, err := readAndHash(path)
	if err != nil {
		return nil, "", err
	}
	var doc struct {
		Nodes []map[string]any `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, "", fmt.Errorf("%s: JSON non valido: %w", path, err)
	}
	if doc.Nodes == nil {
		return nil, "", fmt.Errorf("%s: 'nodes' deve essere una lista", path)
	}

	nodes := make([]AtecoNode, 0, len(doc.Nodes))
	for i, item := range doc.Nodes {
		label := fmt.Sprintf("%s[%d]", path, i)
		codice, ok := item["codice"].(string)
		if !ok || codice == "" {
			return nil, "", fmt.Errorf("%s: codice mancante", label)
		}
		embedText, err := validateEmbeddingText(item["embedding_text"], label)
		if err != nil {
			return nil, "", err
		}
		nodes = append(nodes, AtecoNode{
			Codice:        codice,
			EmbeddingText: embedText,
			Titolo:        stringOr(item["titolo"]),
			Gerarchia:     intOr(item["gerarchia"]),
			RelevanceTier: stringOr(item["relevance_tier"]),
			Classe4Cifre:  stringOr(item["classe_4cifre"]),
			Keywords:      stringSliceOr(item["keywords"]),
			SeedConcepts:  stringSliceOr(item["seed_concepts"]),
			PathText:      stringOr(item["path_text"]),
		})
	}
	return nodes, fileHash, nil
}

// LoadBusinessConcepts legge il file concepts e restituisce (concetti, sha256 del file).
func LoadBusinessConcepts(path string) ([]BusinessConcept, string, error) {
	fileHash, raw, err := readAndHash(path)
	if err != nil {
		return nil, "", err
	}
	var doc struct {
		Concepts []map[string]any `json:"concepts"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, "", fmt.Errorf("%s: JSON non valido: %w", path, err)
	}
	if doc.Concepts == nil {
		return nil, "", fmt.Errorf("%s: 'concepts' deve essere una lista", path)
	}

	concepts := make([]BusinessConcept, 0, len(doc.Concepts))
	for i, item := range doc.Concepts {
		label := fmt.Sprintf("%s[%d]", path, i)
		id, ok := item["id"].(string)
		if !ok || id == "" {
			return nil, "", fmt.Errorf("%s: id mancante", label)
		}
		embedText, err := validateEmbeddingText(item["embedding_text"], label)
		if err != nil {
			return nil, "", err
		}
		concepts = append(concepts, BusinessConcept{
			ID:                      id,
			EmbeddingText:           embedText,
			Name:                    stringOr(item["name"]),
			Domain:                  stringOr(item["domain"]),
			Kind:                    normalizeKind(stringOr(item["kind"])),
			Aliases:                 stringSliceOr(item["aliases"]),
			AtecoCandidatesInKB:     stringSliceOr(item["ateco_candidates_in_kb"]),
			AtecoCandidatesExcluded: stringSliceOr(item["ateco_candidates_excluded"]),
		})
	}
	return concepts, fileHash, nil
}

// --- helpers ---

// readAndHash legge i byte del file e restituisce (sha256 hex, raw bytes).
func readAndHash(path string) (string, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), raw, nil
}

// normalizeKind restituisce "distractor" se esplicitamente marcato tale,
// altrimenti "target" (default: assenza di kind = concetto-perimetro).
func normalizeKind(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "distractor") {
		return "distractor"
	}
	return "target"
}

func validateEmbeddingText(v any, label string) (string, error) {
	s, _ := v.(string)
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("%s: embedding_text è vuoto o assente", label)
	}
	return s, nil
}

func stringOr(v any) string {
	s, _ := v.(string)
	return s
}

func intOr(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

// stringSliceOr converte un []any (JSON array di stringhe) in []string.
// Restituisce nil se l'elemento non è un array.
func stringSliceOr(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
