// Package scrape is a thin client for the self-hosted scrape service
// (Firecrawl-compatible POST /v1/scrape). It fetches a URL's rendered content as
// markdown — no crawling logic lives here, the service does the work. Binocolo
// uses it to verify a resolved domain belongs to the target company (on-page
// P.IVA / name) and to reuse the real page content as classification evidence
// instead of thin search snippets.
package scrape

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	scrapePath     = "/v1/scrape"
	defaultTimeout = 30 * time.Second
)

type Config struct {
	BaseURL    string
	HTTPClient *http.Client
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New returns a client, or nil when no base URL is configured — mirroring
// brave.New so callers can keep the integration optional (nil => disabled).
func New(cfg Config) *Client {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

type APIError struct {
	StatusCode int
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
	return fmt.Sprintf("scrape: HTTP %d: %s", e.StatusCode, message)
}

// Result is the decoupled view of the scrape response we use.
type Result struct {
	Markdown   string
	Title      string
	SourceURL  string
	StatusCode int // HTTP status the scraper observed fetching the target page
}

// Scrape fetches target and returns its content as markdown. The request format
// is fixed to ["markdown"]. Returns an error on transport failure, a non-2xx
// scraper response, or success:false.
func (c *Client) Scrape(ctx context.Context, target string) (Result, error) {
	payload, err := json.Marshal(map[string]any{
		"url":     target,
		"formats": []string{"markdown"},
	})
	if err != nil {
		return Result{}, fmt.Errorf("scrape: encode request: %w", err)
	}

	endpoint := c.baseURL + scrapePath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Result{}, fmt.Errorf("scrape: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("scrape: request %s: %w", scrapePath, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("scrape: read response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Result{}, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
			Message:    parseErrorMessage(body),
		}
	}

	var parsed scrapeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Result{}, fmt.Errorf("scrape: decode response: %w", err)
	}
	if !parsed.Success {
		return Result{}, &APIError{StatusCode: resp.StatusCode, Body: string(body), Message: firstNonEmpty(parsed.Error, "success=false")}
	}

	return Result{
		Markdown:   parsed.Data.Markdown,
		Title:      parsed.Data.Metadata.Title,
		SourceURL:  parsed.Data.Metadata.SourceURL,
		StatusCode: parsed.Data.Metadata.StatusCode,
	}, nil
}

// scrapeResponse mirrors the relevant slice of the wire JSON:
//
//	{ "success": true,
//	  "data": { "markdown": "...", "metadata": { "title", "sourceURL", "statusCode" } },
//	  "error": "..." }
type scrapeResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Markdown string `json:"markdown"`
		Metadata struct {
			Title      string `json:"title"`
			SourceURL  string `json:"sourceURL"`
			StatusCode int    `json:"statusCode"`
		} `json:"metadata"`
	} `json:"data"`
	Error string `json:"error"`
}

func parseErrorMessage(body []byte) string {
	var payload struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return firstNonEmpty(payload.Error, payload.Message)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
