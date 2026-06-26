package llm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// globalApp is the reserved sentinel app-id that holds portal-wide fallback
// bindings (see migration 047). Real catalog app-ids never start with "_".
const globalApp = "_global"

// ErrConfigNotFound is returned when no model/prompt binding resolves for an
// (app, scope). It wraps the resolution context for clearer errors.
var ErrConfigNotFound = errors.New("llm: config not found")

// ErrNotConfigured is returned when the Service has no DB handle (Anisetta not
// configured).
var ErrNotConfigured = errors.New("llm: service not configured")

// Model is a resolved model->scope binding from mrsmith.llm_model.
type Model struct {
	ID               string
	App              string
	Scope            string
	ProviderID       string
	Name             string
	Model            string
	Params           json.RawMessage
	SupportsTools    bool
	SupportsJSONMode bool
	IsDefault        bool
}

// Prompt is a resolved prompt binding from mrsmith.llm_prompt.
type Prompt struct {
	ID        string
	App       string
	Scope     string
	Name      string
	Prompt    string
	IsDefault bool
}

// Params are the per-call request parameters stored on llm_model.params.
// Pointers distinguish "set" from "absent" so callers can fall back to their
// own defaults when a field is not configured.
type Params struct {
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
}

// DecodedParams parses Model.Params; an empty/invalid value yields zero Params.
func (m Model) DecodedParams() Params {
	var p Params
	if len(m.Params) == 0 {
		return p
	}
	_ = json.Unmarshal(m.Params, &p)
	return p
}

const modelColumns = `id::text, app, scope, provider_id::text, name, model, params, supports_tools, supports_json_mode, is_default`
const promptColumns = `id::text, app, scope, name, prompt, is_default`

// ResolveModel resolves a model for (app, scope). If explicitID is non-empty it
// loads that specific model (scoped to the app or the global tier); otherwise
// it applies the cascade (app,scope) -> (app,'default') -> ('_global','default').
func (s *Service) ResolveModel(ctx context.Context, app, scope, explicitID string) (Model, error) {
	if s == nil || s.db == nil {
		return Model{}, ErrNotConfigured
	}
	app, scope, explicitID = strings.TrimSpace(app), strings.TrimSpace(scope), strings.TrimSpace(explicitID)
	if explicitID != "" {
		m, err := scanModelInto(s.db.QueryRowContext(ctx, `SELECT `+modelColumns+`
FROM mrsmith.llm_model WHERE id = $1::uuid AND app IN ($2, $3)`, explicitID, app, globalApp))
		if errors.Is(err, sql.ErrNoRows) {
			return Model{}, fmt.Errorf("%w: model id=%s app=%s", ErrConfigNotFound, explicitID, app)
		}
		return m, err
	}
	m, err := scanModelInto(s.db.QueryRowContext(ctx, `SELECT `+modelColumns+`
FROM mrsmith.llm_model
WHERE is_default AND (
  (app = $1 AND scope = $2) OR (app = $1 AND scope = 'default') OR (app = $3 AND scope = 'default')
)
ORDER BY CASE
  WHEN app = $1 AND scope = $2 THEN 0
  WHEN app = $1 AND scope = 'default' THEN 1
  ELSE 2 END
LIMIT 1`, app, scope, globalApp))
	if errors.Is(err, sql.ErrNoRows) {
		return Model{}, fmt.Errorf("%w: model app=%s scope=%s", ErrConfigNotFound, app, scope)
	}
	return m, err
}

// ResolvePrompt resolves a prompt for (app, scope). If explicitID is non-empty
// it loads that specific prompt (scoped to the app); otherwise it applies the
// cascade (app,scope) -> (app,'default'). No global tier (prompts are not
// portable across apps).
func (s *Service) ResolvePrompt(ctx context.Context, app, scope, explicitID string) (Prompt, error) {
	if s == nil || s.db == nil {
		return Prompt{}, ErrNotConfigured
	}
	app, scope, explicitID = strings.TrimSpace(app), strings.TrimSpace(scope), strings.TrimSpace(explicitID)
	if explicitID != "" {
		p, err := scanPromptInto(s.db.QueryRowContext(ctx, `SELECT `+promptColumns+`
FROM mrsmith.llm_prompt WHERE id = $1::uuid AND app = $2`, explicitID, app))
		if errors.Is(err, sql.ErrNoRows) {
			return Prompt{}, fmt.Errorf("%w: prompt id=%s app=%s", ErrConfigNotFound, explicitID, app)
		}
		return p, err
	}
	p, err := scanPromptInto(s.db.QueryRowContext(ctx, `SELECT `+promptColumns+`
FROM mrsmith.llm_prompt
WHERE is_default AND app = $1 AND scope IN ($2, 'default')
ORDER BY CASE WHEN scope = $2 THEN 0 ELSE 1 END
LIMIT 1`, app, scope))
	if errors.Is(err, sql.ErrNoRows) {
		return Prompt{}, fmt.Errorf("%w: prompt app=%s scope=%s", ErrConfigNotFound, app, scope)
	}
	return p, err
}

// ListModels returns all model bindings visible to an app (its own + global),
// for selection UIs.
func (s *Service) ListModels(ctx context.Context, app string) ([]Model, error) {
	if s == nil || s.db == nil {
		return nil, ErrNotConfigured
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+modelColumns+`
FROM mrsmith.llm_model WHERE app IN ($1, $2) ORDER BY scope, name`, strings.TrimSpace(app), globalApp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Model
	for rows.Next() {
		m, err := scanModelInto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListPrompts returns all prompt bindings for an app, for selection UIs.
func (s *Service) ListPrompts(ctx context.Context, app string) ([]Prompt, error) {
	if s == nil || s.db == nil {
		return nil, ErrNotConfigured
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+promptColumns+`
FROM mrsmith.llm_prompt WHERE app = $1 ORDER BY scope, name`, strings.TrimSpace(app))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Prompt
	for rows.Next() {
		p, err := scanPromptInto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanModelInto(sc rowScanner) (Model, error) {
	var m Model
	var params []byte
	if err := sc.Scan(&m.ID, &m.App, &m.Scope, &m.ProviderID, &m.Name, &m.Model, &params, &m.SupportsTools, &m.SupportsJSONMode, &m.IsDefault); err != nil {
		return Model{}, err
	}
	if len(params) > 0 {
		m.Params = json.RawMessage(params)
	}
	return m, nil
}

func scanPromptInto(sc rowScanner) (Prompt, error) {
	var p Prompt
	if err := sc.Scan(&p.ID, &p.App, &p.Scope, &p.Name, &p.Prompt, &p.IsDefault); err != nil {
		return Prompt{}, err
	}
	return p, nil
}
