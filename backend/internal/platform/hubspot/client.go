package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a shared HubSpot REST API client.
type Client struct {
	apiKey  string
	httpCli *http.Client
}

// New creates a new HubSpot client. Returns nil if apiKey is empty.
func New(apiKey string) *Client {
	if apiKey == "" {
		return nil
	}
	return &Client{
		apiKey: apiKey,
		httpCli: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// APIError represents a non-2xx response from HubSpot.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("hubspot: HTTP %d: %s", e.StatusCode, e.Body)
}

// Get performs a GET request to the HubSpot API.
func (c *Client) Get(ctx context.Context, path string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

// Post performs a POST request to the HubSpot API.
func (c *Client) Post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, path, body)
}

// Patch performs a PATCH request to the HubSpot API.
func (c *Client) Patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPatch, path, body)
}

// Put performs a PUT request to the HubSpot API.
func (c *Client) Put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPut, path, body)
}

// Delete performs a DELETE request to the HubSpot API.
func (c *Client) Delete(ctx context.Context, path string) error {
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *Client) do(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("hubspot: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	url := "https://api.hubapi.com" + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("hubspot: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hubspot: request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hubspot: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	if len(respBody) == 0 {
		return nil, nil
	}
	return json.RawMessage(respBody), nil
}
