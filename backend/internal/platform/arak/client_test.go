package arak

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestDoRetriesUnauthorizedTwiceAndSucceeds(t *testing.T) {
	t.Helper()

	var mu sync.Mutex
	tokenCalls := 0
	apiCalls := 0
	authHeaders := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			mu.Lock()
			tokenCalls++
			token := map[string]any{
				"access_token": "token-" + string(rune('0'+tokenCalls)),
				"expires_in":   300,
			}
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(token)
		case "/arak/budget/v1/group":
			mu.Lock()
			apiCalls++
			authHeaders = append(authHeaders, r.Header.Get("Authorization"))
			currentCall := apiCalls
			mu.Unlock()

			if currentCall <= 2 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(Config{
		BaseURL:      server.URL,
		TokenURL:     server.URL + "/token",
		ClientID:     "budget-client",
		ClientSecret: "budget-secret",
	})

	resp, err := client.Do(http.MethodGet, "/arak/budget/v1/group", "", nil)
	if err != nil {
		t.Fatalf("expected retry flow to succeed, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after retries, got %d", resp.StatusCode)
	}

	mu.Lock()
	defer mu.Unlock()

	if tokenCalls != 3 {
		t.Fatalf("expected 3 token fetches, got %d", tokenCalls)
	}
	if apiCalls != 3 {
		t.Fatalf("expected 3 upstream calls, got %d", apiCalls)
	}
	if len(authHeaders) != 3 {
		t.Fatalf("expected 3 auth headers, got %d", len(authHeaders))
	}
	if authHeaders[0] == authHeaders[1] || authHeaders[1] == authHeaders[2] {
		t.Fatalf("expected token to be refreshed between retries, got headers %v", authHeaders)
	}
}

func TestDoStopsAfterTwoUnauthorizedRetries(t *testing.T) {
	t.Helper()

	var mu sync.Mutex
	tokenCalls := 0
	apiCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			mu.Lock()
			tokenCalls++
			token := map[string]any{
				"access_token": "fixed-token",
				"expires_in":   300,
			}
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(token)
		case "/arak/budget/v1/report/unassigned-users":
			mu.Lock()
			apiCalls++
			mu.Unlock()

			http.Error(w, "unauthorized", http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(Config{
		BaseURL:      server.URL,
		TokenURL:     server.URL + "/token",
		ClientID:     "budget-client",
		ClientSecret: "budget-secret",
	})

	resp, err := client.Do(http.MethodPost, "/arak/budget/v1/report/unassigned-users", "", strings.NewReader(`{"enabled":true}`))
	if err != nil {
		t.Fatalf("expected final unauthorized response, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected final status 401, got %d", resp.StatusCode)
	}

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		t.Fatalf("expected readable final response body, got error: %v", readErr)
	}
	if !strings.Contains(string(body), "unauthorized") {
		t.Fatalf("expected upstream unauthorized body, got %q", string(body))
	}

	mu.Lock()
	defer mu.Unlock()

	if tokenCalls != 3 {
		t.Fatalf("expected 3 token fetches, got %d", tokenCalls)
	}
	if apiCalls != 3 {
		t.Fatalf("expected 3 upstream calls, got %d", apiCalls)
	}
}
