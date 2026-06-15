package openapiit

import (
	"bytes"
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
	DefaultCAPBaseURL     = "https://cap.openapi.it"
	DefaultCompanyBaseURL = "https://company.openapi.com"

	capServiceName     = "cap"
	companyServiceName = "company"
	defaultTimeout     = 30 * time.Second
)

type Config struct {
	APIToken       string
	CAPBaseURL     string
	CompanyBaseURL string
	HTTPClient     *http.Client
}

type Client struct {
	apiToken       string
	capBaseURL     string
	companyBaseURL string
	httpClient     *http.Client
}

type Envelope[T any] struct {
	Data    T      `json:"data"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   *int   `json:"error"`
}

type APIError struct {
	Service    string
	StatusCode int
	Path       string
	Body       string
	Message    string
	Code       *int
}

func (e *APIError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = http.StatusText(e.StatusCode)
	}
	if message == "" {
		message = "upstream error"
	}
	return fmt.Sprintf("openapi.it %s: HTTP %d: %s", e.Service, e.StatusCode, message)
}

func New(cfg Config) *Client {
	apiToken := strings.TrimSpace(cfg.APIToken)
	if apiToken == "" {
		return nil
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		apiToken:       apiToken,
		capBaseURL:     defaultBaseURL(cfg.CAPBaseURL, DefaultCAPBaseURL),
		companyBaseURL: defaultBaseURL(cfg.CompanyBaseURL, DefaultCompanyBaseURL),
		httpClient:     httpClient,
	}
}

func NewWithBaseURL(apiToken, capBaseURL string, httpClient *http.Client) *Client {
	return New(Config{
		APIToken:   apiToken,
		CAPBaseURL: capBaseURL,
		HTTPClient: httpClient,
	})
}

func NewWithBaseURLs(apiToken, capBaseURL, companyBaseURL string, httpClient *http.Client) *Client {
	return New(Config{
		APIToken:       apiToken,
		CAPBaseURL:     capBaseURL,
		CompanyBaseURL: companyBaseURL,
		HTTPClient:     httpClient,
	})
}

func (c *Client) CAP() *CAPClient {
	if c == nil {
		return nil
	}
	return &CAPClient{client: c}
}

func (c *Client) Company() *CompanyClient {
	if c == nil {
		return nil
	}
	return &CompanyClient{client: c}
}

func (c *Client) getCAP(ctx context.Context, path string, query url.Values, out any) error {
	return c.doJSON(ctx, capServiceName, c.capBaseURL, http.MethodGet, path, query, nil, out)
}

func (c *Client) getCompany(ctx context.Context, path string, query url.Values, out any) error {
	return c.doJSON(ctx, companyServiceName, c.companyBaseURL, http.MethodGet, path, query, nil, out)
}

func (c *Client) postCompany(ctx context.Context, path string, body any, out any) error {
	return c.doJSON(ctx, companyServiceName, c.companyBaseURL, http.MethodPost, path, nil, body, out)
}

func (c *Client) deleteCompany(ctx context.Context, path string, out any) error {
	return c.doJSON(ctx, companyServiceName, c.companyBaseURL, http.MethodDelete, path, nil, nil, out)
}

func (c *Client) doJSON(
	ctx context.Context,
	service string,
	baseURL string,
	method string,
	path string,
	query url.Values,
	body any,
	out any,
) error {
	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/") + normalizePath(path))
	if err != nil {
		return fmt.Errorf("openapi.it %s: build URL: %w", service, err)
	}
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("openapi.it %s: marshal request: %w", service, err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("openapi.it %s: create request: %w", service, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("openapi.it %s: request %s %s: %w", service, method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("openapi.it %s: read response: %w", service, err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, code := parseErrorPayload(respBody)
		return &APIError{
			Service:    service,
			StatusCode: resp.StatusCode,
			Path:       path,
			Body:       string(respBody),
			Message:    message,
			Code:       code,
		}
	}

	if len(strings.TrimSpace(string(respBody))) == 0 || out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("openapi.it %s: decode response for %s: %w", service, path, err)
	}
	return nil
}

func defaultBaseURL(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	return strings.TrimRight(value, "/")
}

func normalizePath(path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func parseErrorPayload(body []byte) (string, *int) {
	var payload struct {
		Message string          `json:"message"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil
	}

	var code *int
	if len(payload.Error) > 0 && strings.TrimSpace(string(payload.Error)) != "null" {
		if parsed, ok := parseIntRaw(payload.Error); ok {
			code = &parsed
		}
	}
	return payload.Message, code
}

func parseIntRaw(raw json.RawMessage) (int, bool) {
	var numeric int
	if err := json.Unmarshal(raw, &numeric); err == nil {
		return numeric, true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return 0, false
		}
		if parsed, err := strconv.Atoi(text); err == nil {
			return parsed, true
		}
	}
	return 0, false
}
