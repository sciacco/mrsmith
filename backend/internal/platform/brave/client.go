// Package brave is a thin client for the Brave Search API. Today it exposes only
// the LLM-context endpoint (pre-extracted, query-relevant text snippets), used by
// binocolo's ad-hoc site-restricted keyword search.
package brave

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the public Brave Search API host.
	DefaultBaseURL = "https://api.search.brave.com"

	llmContextPath = "/res/v1/llm/context"
	defaultTimeout = 30 * time.Second
)

type Config struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// New returns a client, or nil when no API key is configured — mirroring
// openapiit.New so callers can keep the integration optional.
func New(cfg Config) *Client {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

type APIError struct {
	StatusCode int
	Path       string
	Body       string
	Message    string
}

func (e *APIError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = http.StatusText(e.StatusCode)
	}
	if message == "" {
		message = "upstream error"
	}
	return fmt.Sprintf("brave: HTTP %d: %s", e.StatusCode, message)
}

// LLMContextParams are the (subset of) query parameters we send to the
// LLM-context endpoint.
type LLMContextParams struct {
	Query      string // q — keywords; site restriction via the "site:" operator
	Count      int    // count — 1..50
	Country    string // country — 2-char code
	SearchLang string // search_lang
}

// GenericResult is one grounding entry: a source page plus the snippets the API
// extracted from it for the query.
type GenericResult struct {
	URL      string   `json:"url"`
	Title    string   `json:"title"`
	Snippets []string `json:"snippets"`
}

// SourceMeta is the per-URL metadata block from the response "sources" map.
type SourceMeta struct {
	Title    string
	Hostname string
	Age      []string
}

// LLMContextResult is the parsed, decoupled view of the response we care about.
type LLMContextResult struct {
	Generic []GenericResult
	Sources map[string]SourceMeta
}

// LLMContext calls GET /res/v1/llm/context and returns the grounding snippets plus
// per-source metadata. The endpoint also accepts POST; GET keeps it simple since
// our query is short.
func (c *Client) LLMContext(ctx context.Context, params LLMContextParams) (LLMContextResult, error) {
	query := url.Values{}
	query.Set("q", params.Query)
	if params.Count > 0 {
		query.Set("count", strconv.Itoa(params.Count))
	}
	if params.Country != "" {
		query.Set("country", params.Country)
	}
	if params.SearchLang != "" {
		query.Set("search_lang", params.SearchLang)
	}

	endpoint := c.baseURL + llmContextPath + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return LLMContextResult{}, fmt.Errorf("brave: create request: %w", err)
	}
	req.Header.Set("X-Subscription-Token", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return LLMContextResult{}, fmt.Errorf("brave: request %s: %w", llmContextPath, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LLMContextResult{}, fmt.Errorf("brave: read response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return LLMContextResult{}, &APIError{
			StatusCode: resp.StatusCode,
			Path:       llmContextPath,
			Body:       string(body),
			Message:    parseErrorMessage(body),
		}
	}

	if len(strings.TrimSpace(string(body))) == 0 {
		return LLMContextResult{}, nil
	}

	var parsed llmContextResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return LLMContextResult{}, fmt.Errorf("brave: decode response: %w", err)
	}

	result := LLMContextResult{
		Generic: parsed.Grounding.Generic,
		Sources: make(map[string]SourceMeta, len(parsed.Sources)),
	}
	for u, s := range parsed.Sources {
		result.Sources[u] = SourceMeta{Title: s.Title, Hostname: s.Hostname, Age: s.Age}
	}
	return result, nil
}

// llmContextResponse mirrors the relevant slice of the wire JSON:
//
//	{ "grounding": { "generic": [ {url,title,snippets[]} ] },
//	  "sources":   { "<url>": {title,hostname,age} } }
type llmContextResponse struct {
	Grounding struct {
		Generic []GenericResult `json:"generic"`
	} `json:"grounding"`
	Sources map[string]struct {
		Title    string   `json:"title"`
		Hostname string   `json:"hostname"`
		Age      ageField `json:"age"`
	} `json:"sources"`
}

// ageField decodes the "age" field, which the API returns as a string array, a
// bare string, or null depending on availability.
type ageField []string

func (a *ageField) UnmarshalJSON(b []byte) error {
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" || trimmed == "null" {
		*a = nil
		return nil
	}
	var arr []string
	if err := json.Unmarshal(b, &arr); err == nil {
		*a = arr
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*a = []string{s}
		return nil
	}
	*a = nil
	return nil
}

func parseErrorMessage(body []byte) string {
	var payload struct {
		Message string `json:"message"`
		Error   struct {
			Detail string `json:"detail"`
			Code   string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if d := strings.TrimSpace(payload.Error.Detail); d != "" {
		return d
	}
	if m := strings.TrimSpace(payload.Message); m != "" {
		return m
	}
	return strings.TrimSpace(payload.Error.Code)
}
