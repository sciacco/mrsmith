package binocolo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// kbConcept is one curated business concept with its embedding vector and the
// concept->ATECO fit mapping, loaded from binocolo.kb_concept (+ kb_concept_ateco)
// for the embedding-based ATECO retrieval (use case 1). Only rows whose vector has
// been populated by the `atego` loader are returned.
type kbConcept struct {
	ID   string
	Name string
	Kind string // "target" | "distractor"
	// EmbeddingModelID names the mrsmith.embedding_model that produced Vector. The
	// runtime resolves it to embed the query with the SAME model (drift guard).
	EmbeddingModelID string
	Vector           []float32
	// InKB / Excluded are the relation='in_kb' / 'excluded' ATECO codes from
	// kb_concept_ateco, in source order. The core/weak split is decided at query
	// time from the matched concept's cosine, not stored on the edge.
	InKB     []string
	Excluded []string
}

// kbStore reads the curated KB vectors for ATECO retrieval. Soft dependency: when
// nil or empty the retrieval path falls back to the LLM hierarchy resolver.
type kbStore interface {
	LoadKBConcepts(ctx context.Context) ([]kbConcept, error)
}

// LoadKBConcepts loads every concept whose embedding has been populated, plus its
// concept->ATECO fit mapping. The vector is read as text (real[]::text -> "{...}")
// and parsed in Go so it stays driver-agnostic (no pgx/lib-pq array codec). The
// corpus is tiny (a few dozen vectors), so this runs per draft without caching.
func (s *SQLStore) LoadKBConcepts(ctx context.Context) ([]kbConcept, error) {
	if s == nil || s.db == nil {
		return nil, errMAStoreUnavailable
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, kind, COALESCE(embedding_model_id::text, ''), COALESCE(embedding::text, '')
FROM binocolo.kb_concept
WHERE embedding IS NOT NULL
ORDER BY id
`)
	if err != nil {
		return nil, fmt.Errorf("load kb concepts: %w", err)
	}
	defer rows.Close()

	byID := map[string]*kbConcept{}
	out := []kbConcept{}
	for rows.Next() {
		var (
			id, name, kind, modelID, vecText string
		)
		if err := rows.Scan(&id, &name, &kind, &modelID, &vecText); err != nil {
			return nil, fmt.Errorf("scan kb concept: %w", err)
		}
		vec, err := parsePGFloatArray(vecText)
		if err != nil {
			return nil, fmt.Errorf("parse kb concept %q vector: %w", id, err)
		}
		if len(vec) == 0 {
			continue
		}
		out = append(out, kbConcept{ID: id, Name: name, Kind: normalizeKBKind(kind), EmbeddingModelID: modelID, Vector: vec})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate kb concepts: %w", err)
	}
	if len(out) == 0 {
		return out, nil
	}
	for i := range out {
		byID[out[i].ID] = &out[i]
	}

	mapRows, err := s.db.QueryContext(ctx, `
SELECT concept_id, ateco_code, relation
FROM binocolo.kb_concept_ateco
ORDER BY concept_id, position NULLS LAST, ateco_code
`)
	if err != nil {
		return nil, fmt.Errorf("load kb concept ateco: %w", err)
	}
	defer mapRows.Close()
	for mapRows.Next() {
		var conceptID, atecoCode, relation string
		if err := mapRows.Scan(&conceptID, &atecoCode, &relation); err != nil {
			return nil, fmt.Errorf("scan kb concept ateco: %w", err)
		}
		concept := byID[conceptID]
		if concept == nil {
			continue // mapping for an un-embedded concept; ignore
		}
		switch relation {
		case "in_kb":
			concept.InKB = append(concept.InKB, atecoCode)
		case "excluded":
			concept.Excluded = append(concept.Excluded, atecoCode)
		}
	}
	if err := mapRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate kb concept ateco: %w", err)
	}
	return out, nil
}

func normalizeKBKind(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "distractor") {
		return "distractor"
	}
	return "target"
}

// parsePGFloatArray parses a Postgres array text literal ("{1.5,2,-0.3}") into a
// []float32. An empty array ("{}") or empty string yields a nil slice.
func parsePGFloatArray(text string) ([]float32, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	text = strings.TrimPrefix(text, "{")
	text = strings.TrimSuffix(text, "}")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	parts := strings.Split(text, ",")
	out := make([]float32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseFloat(p, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid float %q: %w", p, err)
		}
		out = append(out, float32(v))
	}
	return out, nil
}
