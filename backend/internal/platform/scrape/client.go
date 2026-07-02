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
	searchPath     = "/v1/search"
	crawlPath      = "/v1/crawl"
	mapPath        = "/v1/map"
	defaultTimeout = 30 * time.Second
)

type Config struct {
	BaseURL string
	// APIKey, when set, is sent as an "Authorization: Bearer <key>" header.
	// Self-hosted Firecrawl needs no auth (leave empty); managed services
	// (e.g. fastcrw.com) require it.
	APIKey     string
	HTTPClient *http.Client
}

type Client struct {
	baseURL    string
	apiKey     string
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
		apiKey:     strings.TrimSpace(cfg.APIKey),
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
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

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

// SearchResult is one web-search hit from the /v1/search endpoint.
type SearchResult struct {
	URL         string
	Title       string
	Description string
	Snippet     string
}

// Search runs a web search via /v1/search (fastcrw-compatible). Binocolo uses it
// as a domain-resolution retrieval fallback when the primary search engine fails
// to surface a company's official site. Requires an API key. Returns an error on
// transport failure, a non-2xx response, or success:false.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 8
	}
	payload, err := json.Marshal(map[string]any{"query": query, "limit": limit})
	if err != nil {
		return nil, fmt.Errorf("scrape: encode search request: %w", err)
	}

	endpoint := c.baseURL + searchPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("scrape: create search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scrape: request %s: %w", searchPath, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("scrape: read search response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body), Message: parseErrorMessage(body)}
	}

	var parsed searchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("scrape: decode search response: %w", err)
	}
	if !parsed.Success {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body), Message: firstNonEmpty(parsed.Error, "success=false")}
	}

	out := make([]SearchResult, 0, len(parsed.Data))
	for _, d := range parsed.Data {
		out = append(out, SearchResult{URL: d.URL, Title: d.Title, Description: d.Description, Snippet: d.Snippet})
	}
	return out, nil
}

// searchResponse mirrors the relevant slice of the /v1/search wire JSON:
//
//	{ "success": true,
//	  "data": [ { "url", "title", "description", "snippet", ... } ],
//	  "error": "..." }
type searchResponse struct {
	Success bool `json:"success"`
	Data    []struct {
		URL         string `json:"url"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Snippet     string `json:"snippet"`
	} `json:"data"`
	Error string `json:"error"`
}

// Map discovers a site's URLs via /v1/map (sitemap-first with BFS fallback,
// synchronous) WITHOUT fetching page content. Use it as the cheap first pass to
// decide which pages are worth scraping — binocolo maps a candidate domain and
// then scrapes only the few identity-bearing pages (/contatti, /note-legali)
// instead of running a blind multi-page crawl. Requires an API key on managed
// services.
func (c *Client) Map(ctx context.Context, target string, maxDepth int) ([]string, error) {
	reqBody := map[string]any{"url": target}
	if maxDepth > 0 {
		reqBody["maxDepth"] = maxDepth
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("scrape: encode map request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+mapPath, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("scrape: create map request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scrape: request %s: %w", mapPath, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("scrape: read map response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body), Message: parseErrorMessage(body)}
	}

	var parsed mapResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("scrape: decode map response: %w", err)
	}
	if !parsed.Success {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body), Message: firstNonEmpty(parsed.Error, "success=false")}
	}
	return parsed.Data.Links, nil
}

// mapResponse mirrors the /v1/map wire JSON:
//
//	{ "success": true, "data": { "links": ["https://example.com", ...] } }
type mapResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Links []string `json:"links"`
	} `json:"data"`
	Error string `json:"error"`
}

// CrawlPage is one page returned by a crawl job.
type CrawlPage struct {
	URL      string
	Markdown string
}

// Crawl runs a bounded, same-site crawl via /v1/crawl. The endpoint is async:
// this starts the job and polls until it completes, fails, or ctx is cancelled,
// returning the per-page markdown collected. Binocolo uses it post-resolution to
// gather multi-page self-description evidence and to confirm the target's P.IVA
// on legal/contact pages the homepage omits. Bound the scope with maxDepth /
// maxPages. Requires an API key.
func (c *Client) Crawl(ctx context.Context, target string, maxDepth, maxPages int) ([]CrawlPage, error) {
	reqBody := map[string]any{"url": target, "formats": []string{"markdown"}, "onlyMainContent": true}
	if maxDepth > 0 {
		reqBody["maxDepth"] = maxDepth
	}
	if maxPages > 0 {
		reqBody["maxPages"] = maxPages
	}
	id, err := c.startCrawl(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	const pollEvery = 2 * time.Second
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollEvery):
		}
		pages, status, err := c.pollCrawl(ctx, id)
		if err != nil {
			return nil, err
		}
		switch status {
		case "completed":
			return pages, nil
		case "failed":
			return pages, fmt.Errorf("scrape: crawl %s failed", id)
		}
	}
}

func (c *Client) startCrawl(ctx context.Context, reqBody map[string]any) (string, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("scrape: encode crawl request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+crawlPath, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("scrape: create crawl request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("scrape: request %s: %w", crawlPath, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("scrape: read crawl response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", &APIError{StatusCode: resp.StatusCode, Body: string(body), Message: parseErrorMessage(body)}
	}
	var parsed struct {
		Success bool   `json:"success"`
		ID      string `json:"id"`
		JobID   string `json:"jobId"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("scrape: decode crawl response: %w", err)
	}
	id := firstNonEmpty(parsed.ID, parsed.JobID)
	if !parsed.Success || id == "" {
		return "", &APIError{StatusCode: resp.StatusCode, Body: string(body), Message: firstNonEmpty(parsed.Error, "no crawl id")}
	}
	return id, nil
}

func (c *Client) pollCrawl(ctx context.Context, id string) ([]CrawlPage, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+crawlPath+"/"+id, nil)
	if err != nil {
		return nil, "", fmt.Errorf("scrape: create crawl poll: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("scrape: poll %s: %w", crawlPath, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("scrape: read crawl poll: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", &APIError{StatusCode: resp.StatusCode, Body: string(body), Message: parseErrorMessage(body)}
	}
	var parsed crawlStatusResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, "", fmt.Errorf("scrape: decode crawl poll: %w", err)
	}
	pages := make([]CrawlPage, 0, len(parsed.Data))
	for _, d := range parsed.Data {
		pages = append(pages, CrawlPage{URL: d.Metadata.SourceURL, Markdown: d.Markdown})
	}
	return pages, parsed.Status, nil
}

// crawlStatusResponse mirrors the /v1/crawl/{id} poll JSON:
//
//	{ "success": true, "status": "scraping|completed|failed",
//	  "total": 12, "completed": 12,
//	  "data": [ { "markdown": "...", "metadata": { "sourceURL": "..." } } ] }
type crawlStatusResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
	Data    []struct {
		Markdown string `json:"markdown"`
		Metadata struct {
			SourceURL string `json:"sourceURL"`
		} `json:"metadata"`
	} `json:"data"`
	Error string `json:"error"`
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
