package llm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// RerankModel is a resolved row from mrsmith.rerank_model. A reranker scores a
// (query, document) pair as a yes/no relevance question; on Fireworks it is served
// through the /embeddings endpoint with return_logits over the "yes"/"no" token
// ids (carried per-row, since they are tokenizer constants of the specific model).
type RerankModel struct {
	ID         string
	App        string
	Scope      string
	ProviderID string
	Name       string
	Model      string
	NoTokenID  int
	YesTokenID int
	Params     json.RawMessage
	IsDefault  bool
}

// RerankDefaultInstruction is the model-card default task description, used when
// the caller passes an empty instruction.
const RerankDefaultInstruction = "Given a web search query, retrieve relevant passages that answer the query"

// rerankPromptFormat is the Qwen3-Reranker input template. Fireworks applies the
// chat wrapping (<|im_start|>…<think>…) server-side, so the caller sends ONLY this
// simple instruction/query/document form — not the full chat-templated string.
const rerankPromptFormat = "<Instruct>: %s\n<Query>: %s\n<Document>: %s"

const rerankModelColumns = `id::text, app, scope, provider_id::text, name, model, no_token_id, yes_token_id, params, is_default`

// ResolveRerankModel resolves a reranker for (app, scope). If explicitID is set it
// loads that row (scoped to the app or the global tier); otherwise it applies the
// same cascade as chat models: (app,scope) -> (app,'default') -> ('_global','default').
func (s *Service) ResolveRerankModel(ctx context.Context, app, scope, explicitID string) (RerankModel, error) {
	if s == nil || s.db == nil {
		return RerankModel{}, ErrNotConfigured
	}
	app, scope, explicitID = strings.TrimSpace(app), strings.TrimSpace(scope), strings.TrimSpace(explicitID)
	if explicitID != "" {
		m, err := scanRerankModelInto(s.db.QueryRowContext(ctx, `SELECT `+rerankModelColumns+`
FROM mrsmith.rerank_model WHERE id = $1::uuid AND app IN ($2, $3)`, explicitID, app, globalApp))
		if errors.Is(err, sql.ErrNoRows) {
			return RerankModel{}, fmt.Errorf("%w: rerank model id=%s app=%s", ErrConfigNotFound, explicitID, app)
		}
		return m, err
	}
	m, err := scanRerankModelInto(s.db.QueryRowContext(ctx, `SELECT `+rerankModelColumns+`
FROM mrsmith.rerank_model
WHERE is_default AND (
  (app = $1 AND scope = $2) OR (app = $1 AND scope = 'default') OR (app = $3 AND scope = 'default')
)
ORDER BY CASE
  WHEN app = $1 AND scope = $2 THEN 0
  WHEN app = $1 AND scope = 'default' THEN 1
  ELSE 2 END
LIMIT 1`, app, scope, globalApp))
	if errors.Is(err, sql.ErrNoRows) {
		return RerankModel{}, fmt.Errorf("%w: rerank model app=%s scope=%s", ErrConfigNotFound, app, scope)
	}
	return m, err
}

// Rerank scores each document's relevance to the query, returning one probability
// in [0,1] per document, in input order (the "yes"-token softmax probability). The
// instruction makes the reranker instruction-aware; empty falls back to the model
// default. Provider/key resolve from mrsmith.llm_provider (shared with chat/embed).
func (s *Service) Rerank(ctx context.Context, m RerankModel, instruction, query string, documents []string) ([]float64, Usage, error) {
	if s == nil || s.db == nil {
		return nil, Usage{}, ErrNotConfigured
	}
	if len(documents) == 0 {
		return nil, Usage{}, nil
	}
	if strings.TrimSpace(instruction) == "" {
		instruction = RerankDefaultInstruction
	}
	prompts := make([]string, len(documents))
	for i, doc := range documents {
		prompts[i] = fmt.Sprintf(rerankPromptFormat, instruction, query, doc)
	}
	client, _, err := s.clientForProvider(ctx, m.ProviderID)
	if err != nil {
		return nil, Usage{}, err
	}
	return client.Rerank(ctx, m.Model, prompts, m.NoTokenID, m.YesTokenID)
}

func scanRerankModelInto(sc rowScanner) (RerankModel, error) {
	var m RerankModel
	var params []byte
	if err := sc.Scan(&m.ID, &m.App, &m.Scope, &m.ProviderID, &m.Name, &m.Model, &m.NoTokenID, &m.YesTokenID, &params, &m.IsDefault); err != nil {
		return RerankModel{}, err
	}
	if len(params) > 0 {
		m.Params = json.RawMessage(params)
	}
	return m, nil
}
