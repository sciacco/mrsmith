// Package arak provides an HTTP client for the MISTRA-NG / Arak API,
// authenticated via Keycloak client credentials grant.
package arak

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Config holds the settings needed to create a Client.
type Config struct {
	BaseURL      string // e.g. "https://gw-int.cdlan.net"
	TokenURL     string // Keycloak token endpoint
	ClientID     string
	ClientSecret string
}

// Client calls the Arak API using a cached service-account token.
type Client struct {
	cfg        Config
	httpClient *http.Client

	mu          sync.Mutex
	accessToken string
	expiry      time.Time
}

// New creates a ready-to-use Arak client.
func New(cfg Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// token returns a valid access token, refreshing it if expired or missing.
func (c *Client) token() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Reuse token if it has at least 30s of life left.
	if c.accessToken != "" && time.Now().Before(c.expiry.Add(-30*time.Second)) {
		return c.accessToken, nil
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
	}

	resp, err := c.httpClient.PostForm(c.cfg.TokenURL, data)
	if err != nil {
		return "", fmt.Errorf("arak: token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("arak: token endpoint returned %d: %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("arak: failed to decode token response: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	c.expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return c.accessToken, nil
}

// Do performs an authenticated request to the Arak API.
// method is the HTTP method (GET, POST, PUT, DELETE).
// path should start with "/" (e.g. "/arak/budget/v1/budget").
// queryString is appended as-is (already URL-encoded).
// body may be nil for requests without a body.
func (c *Client) Do(method, path, queryString string, body io.Reader) (*http.Response, error) {
	fullURL := strings.TrimRight(c.cfg.BaseURL, "/") + path
	if queryString != "" {
		fullURL += "?" + queryString
	}

	tok, err := c.token()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("arak: failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}
