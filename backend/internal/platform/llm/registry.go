package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Provider is an OpenAI-compatible endpoint from mrsmith.llm_provider.
type Provider struct {
	ID             string
	Name           string
	BaseURL        string
	APIKeyEnv      string
	APIKey         string
	DefaultHeaders map[string]string
}

// ErrProviderKeyMissing is returned when a provider resolves no usable key
// (neither the env var named by api_key_env, nor the plaintext fallback).
var ErrProviderKeyMissing = errors.New("llm: provider has no resolvable api key")

// resolveKey applies the env-first, plaintext-fallback rule (uniform across
// environments).
func (p Provider) resolveKey() string {
	if v := strings.TrimSpace(os.Getenv(p.APIKeyEnv)); v != "" {
		return v
	}
	return strings.TrimSpace(p.APIKey)
}

func (s *Service) loadProvider(ctx context.Context, id string) (Provider, error) {
	var p Provider
	var headers []byte
	err := s.db.QueryRowContext(ctx, `
SELECT id::text, name, base_url, api_key_env, api_key, default_headers
FROM mrsmith.llm_provider WHERE id = $1::uuid`, id).Scan(&p.ID, &p.Name, &p.BaseURL, &p.APIKeyEnv, &p.APIKey, &headers)
	if err != nil {
		return Provider{}, err
	}
	if len(headers) > 0 {
		_ = json.Unmarshal(headers, &p.DefaultHeaders)
	}
	return p, nil
}

// ClientForModel loads the provider bound to the model and builds an
// OpenAI-compatible client for it. Built per call (no cache, see design #7); the
// Service's shared *http.Client pools connections per host.
func (s *Service) ClientForModel(ctx context.Context, m Model) (*Client, Provider, error) {
	if s == nil || s.db == nil {
		return nil, Provider{}, ErrNotConfigured
	}
	p, err := s.loadProvider(ctx, m.ProviderID)
	if err != nil {
		return nil, Provider{}, fmt.Errorf("llm: load provider %s: %w", m.ProviderID, err)
	}
	key := p.resolveKey()
	if key == "" {
		return nil, p, fmt.Errorf("%w: provider %q (env %s)", ErrProviderKeyMissing, p.Name, p.APIKeyEnv)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	opts := []option.RequestOption{
		option.WithAPIKey(key),
		option.WithBaseURL(baseURL),
		option.WithHTTPClient(s.http),
		option.WithMaxRetries(2),
	}
	for k, v := range p.DefaultHeaders {
		opts = append(opts, option.WithHeader(k, v))
	}
	return &Client{sdk: openai.NewClient(opts...), label: p.Name}, p, nil
}
