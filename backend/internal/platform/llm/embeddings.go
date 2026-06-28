package llm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// EmbeddingModel is a resolved row from mrsmith.embedding_model. Unlike chat
// models (resolver.go) there is deliberately NO scope cascade or fallback: an
// embedding model is bound to a stored vector set by id — a vector set names the
// exact model that produced it — and falling back to a different model yields
// vectors from an incompatible space (silent cosine garbage). Resolution is
// therefore always by explicit id, never by (app, scope).
type EmbeddingModel struct {
	ID         string
	ProviderID string
	Name       string
	Model      string
	Dimension  int
	Distance   string
}

const embeddingModelColumns = `id::text, provider_id::text, name, model, dimension, distance`

// ResolveEmbeddingModel loads an embedding model by id (row-driven, no cascade).
// Callers pass the embedding_model_id carried by the stored vectors they intend
// to query against, guaranteeing query and documents share one model.
func (s *Service) ResolveEmbeddingModel(ctx context.Context, id string) (EmbeddingModel, error) {
	if s == nil || s.db == nil {
		return EmbeddingModel{}, ErrNotConfigured
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return EmbeddingModel{}, fmt.Errorf("%w: embedding model id empty", ErrConfigNotFound)
	}
	var m EmbeddingModel
	err := s.db.QueryRowContext(ctx, `SELECT `+embeddingModelColumns+`
FROM mrsmith.embedding_model WHERE id = $1::uuid`, id).
		Scan(&m.ID, &m.ProviderID, &m.Name, &m.Model, &m.Dimension, &m.Distance)
	if errors.Is(err, sql.ErrNoRows) {
		return EmbeddingModel{}, fmt.Errorf("%w: embedding model id=%s", ErrConfigNotFound, id)
	}
	if err != nil {
		return EmbeddingModel{}, err
	}
	return m, nil
}

// Embed produces an embedding vector per input using the resolved embedding
// model. Provider/key are resolved from mrsmith.llm_provider (shared with chat).
// Vectors come back in input order; token usage is returned for tracing.
func (s *Service) Embed(ctx context.Context, m EmbeddingModel, inputs []string) ([][]float32, Usage, error) {
	if s == nil || s.db == nil {
		return nil, Usage{}, ErrNotConfigured
	}
	if len(inputs) == 0 {
		return nil, Usage{}, nil
	}
	client, _, err := s.clientForProvider(ctx, m.ProviderID)
	if err != nil {
		return nil, Usage{}, err
	}
	return client.Embed(ctx, m.Model, inputs)
}
