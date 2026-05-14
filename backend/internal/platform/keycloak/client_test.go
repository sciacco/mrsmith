package keycloak

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTokenCachingExpiryRefreshAndUnauthorizedRetry(t *testing.T) {
	var mu sync.Mutex
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	tokenCalls := 0
	userAuthHeaders := []string{}
	groupAuthHeaders := []string{}
	unauthorizedOnce := true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse token form: %v", err)
			}
			if r.Form.Get("grant_type") != "client_credentials" {
				t.Fatalf("unexpected grant_type %q", r.Form.Get("grant_type"))
			}
			if r.Form.Get("client_id") != "client" {
				t.Fatalf("unexpected client_id %q", r.Form.Get("client_id"))
			}
			if r.Form.Get("client_secret") != "secret" {
				t.Fatalf("unexpected client_secret %q", r.Form.Get("client_secret"))
			}

			mu.Lock()
			tokenCalls++
			token := fmt.Sprintf("token-%d", tokenCalls)
			mu.Unlock()
			writeJSON(t, w, map[string]any{"access_token": token, "expires_in": 90})
		case "/admin/realms/test/roles/app_access/users":
			mu.Lock()
			userAuthHeaders = append(userAuthHeaders, r.Header.Get("Authorization"))
			shouldUnauthorized := unauthorizedOnce
			if unauthorizedOnce {
				unauthorizedOnce = false
			}
			mu.Unlock()

			if shouldUnauthorized {
				http.Error(w, "stale token", http.StatusUnauthorized)
				return
			}
			writeJSON(t, w, []map[string]any{})
		case "/admin/realms/test/roles/app_access/groups":
			mu.Lock()
			groupAuthHeaders = append(groupAuthHeaders, r.Header.Get("Authorization"))
			mu.Unlock()
			writeJSON(t, w, []map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	client.now = func() time.Time { return now }

	if _, err := client.UsersByRealmRole(context.Background(), "app_access", UsersByRealmRoleOptions{}); err != nil {
		t.Fatalf("first lookup failed: %v", err)
	}
	if _, err := client.UsersByRealmRole(context.Background(), "app_access", UsersByRealmRoleOptions{}); err != nil {
		t.Fatalf("cached lookup failed: %v", err)
	}
	now = now.Add(61 * time.Second)
	if _, err := client.UsersByRealmRole(context.Background(), "app_access", UsersByRealmRoleOptions{}); err != nil {
		t.Fatalf("expired lookup failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if tokenCalls != 3 {
		t.Fatalf("expected 3 token calls, got %d", tokenCalls)
	}
	if len(userAuthHeaders) != 4 {
		t.Fatalf("expected 4 role-user calls including retry, got %d", len(userAuthHeaders))
	}
	if userAuthHeaders[0] != "Bearer token-1" || userAuthHeaders[1] != "Bearer token-2" {
		t.Fatalf("expected unauthorized retry to refresh token, got headers %v", userAuthHeaders)
	}
	if groupAuthHeaders[0] != "Bearer token-2" || groupAuthHeaders[1] != "Bearer token-2" || groupAuthHeaders[2] != "Bearer token-3" {
		t.Fatalf("expected cached then refreshed group tokens, got %v", groupAuthHeaders)
	}
}

func TestUsersByRealmRolePagesDirectUsers(t *testing.T) {
	var mu sync.Mutex
	roleUserFirsts := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			writeJSON(t, w, map[string]any{"access_token": "token", "expires_in": 300})
		case "/admin/realms/test/roles/app_access/users":
			mu.Lock()
			roleUserFirsts = append(roleUserFirsts, r.URL.Query().Get("first"))
			mu.Unlock()

			switch r.URL.Query().Get("first") {
			case "0":
				writeJSON(t, w, []map[string]any{
					userJSON("u3", "carol", "Carol", "", "carol@example.com", true),
					userJSON("u1", "alice", "Alice", "", "alice@example.com", true),
				})
			case "2":
				writeJSON(t, w, []map[string]any{
					userJSON("u2", "bob", "Bob", "", "bob@example.com", true),
				})
			default:
				t.Fatalf("unexpected direct-user page first=%q", r.URL.Query().Get("first"))
			}
		case "/admin/realms/test/roles/app_access/groups":
			writeJSON(t, w, []map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	users, err := newTestClient(server.URL).UsersByRealmRole(context.Background(), "app_access", UsersByRealmRoleOptions{PageSize: 2})
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}

	gotEmails := emails(users)
	wantEmails := []string{"alice@example.com", "bob@example.com", "carol@example.com"}
	if !reflect.DeepEqual(gotEmails, wantEmails) {
		t.Fatalf("expected sorted direct users %v, got %v", wantEmails, gotEmails)
	}

	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(roleUserFirsts, []string{"0", "2"}) {
		t.Fatalf("expected direct-user pagination first offsets [0 2], got %v", roleUserFirsts)
	}
}

func TestUsersByRealmRoleCollectsRoleGroupsChildrenAndMembers(t *testing.T) {
	var mu sync.Mutex
	calls := make(map[string][]url.Values)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls[r.URL.Path] = append(calls[r.URL.Path], r.URL.Query())
		mu.Unlock()

		switch r.URL.Path {
		case "/token":
			writeJSON(t, w, map[string]any{"access_token": "token", "expires_in": 300})
		case "/admin/realms/test/roles/app_access/users":
			switch r.URL.Query().Get("first") {
			case "0":
				writeJSON(t, w, []map[string]any{
					userJSON("u1", "alice", "Alice", "Able", "ALICE@example.com", true),
					userJSON("u-disabled-direct", "disabled", "", "", "disabled-direct@example.com", false),
				})
			case "2":
				writeJSON(t, w, []map[string]any{
					userJSON("u-blank-direct", "blank", "", "", " ", true),
				})
			default:
				t.Fatalf("unexpected direct-user page first=%q", r.URL.Query().Get("first"))
			}
		case "/admin/realms/test/roles/app_access/groups":
			switch r.URL.Query().Get("first") {
			case "0":
				writeJSON(t, w, []map[string]any{{"id": "g-root"}, {"id": "g-sibling"}})
			case "2":
				writeJSON(t, w, []map[string]any{{"id": "g-last"}})
			default:
				t.Fatalf("unexpected role-group page first=%q", r.URL.Query().Get("first"))
			}
		case "/admin/realms/test/groups/g-root/members":
			switch r.URL.Query().Get("first") {
			case "0":
				writeJSON(t, w, []map[string]any{
					userJSON("u1", "alice-duplicate", "Alice", "Able", "alice-duplicate@example.com", true),
					userJSON("u2", "bob", "Bob", "Baker", "bob@example.com", true),
				})
			case "2":
				writeJSON(t, w, []map[string]any{})
			default:
				t.Fatalf("unexpected root-member page first=%q", r.URL.Query().Get("first"))
			}
		case "/admin/realms/test/groups/g-root/children":
			switch r.URL.Query().Get("first") {
			case "0":
				writeJSON(t, w, []map[string]any{{"id": "g-child"}, {"id": "g-child-2"}})
			case "2":
				writeJSON(t, w, []map[string]any{})
			default:
				t.Fatalf("unexpected child page first=%q", r.URL.Query().Get("first"))
			}
		case "/admin/realms/test/groups/g-child/members":
			switch r.URL.Query().Get("first") {
			case "0":
				writeJSON(t, w, []map[string]any{
					userJSON("u3", "carol", "Carol", "Clark", "carol@example.com", true),
					userJSON("u-disabled", "disabled", "", "", "disabled@example.com", false),
				})
			case "2":
				writeJSON(t, w, []map[string]any{})
			default:
				t.Fatalf("unexpected child-member page first=%q", r.URL.Query().Get("first"))
			}
		case "/admin/realms/test/groups/g-child/children":
			writeJSON(t, w, []map[string]any{})
		case "/admin/realms/test/groups/g-child-2/members":
			writeJSON(t, w, []map[string]any{
				userJSON("u-blank", "blank", "", "", "", true),
			})
		case "/admin/realms/test/groups/g-child-2/children":
			writeJSON(t, w, []map[string]any{})
		case "/admin/realms/test/groups/g-sibling/members":
			writeJSON(t, w, []map[string]any{})
		case "/admin/realms/test/groups/g-sibling/children":
			writeJSON(t, w, []map[string]any{})
		case "/admin/realms/test/groups/g-last/members":
			writeJSON(t, w, []map[string]any{})
		case "/admin/realms/test/groups/g-last/children":
			writeJSON(t, w, []map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	users, err := newTestClient(server.URL).UsersByRealmRole(context.Background(), "app_access", UsersByRealmRoleOptions{PageSize: 2})
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}

	gotEmails := emails(users)
	wantEmails := []string{"alice@example.com", "bob@example.com", "carol@example.com"}
	if !reflect.DeepEqual(gotEmails, wantEmails) {
		t.Fatalf("expected deduped enabled users with email %v, got %v", wantEmails, gotEmails)
	}
	if users[0].ID != "u1" || users[0].Username != "alice" || users[0].Name != "Alice Able" {
		t.Fatalf("expected direct user to win duplicate and normalize name/email, got %#v", users[0])
	}

	mu.Lock()
	defer mu.Unlock()
	if got := firstOffsets(calls["/admin/realms/test/roles/app_access/groups"]); !reflect.DeepEqual(got, []string{"0", "2"}) {
		t.Fatalf("expected role group pagination [0 2], got %v", got)
	}
	if got := firstOffsets(calls["/admin/realms/test/groups/g-root/children"]); !reflect.DeepEqual(got, []string{"0", "2"}) {
		t.Fatalf("expected child pagination [0 2], got %v", got)
	}
}

func TestUsersByRealmRoleMapsRoleNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			writeJSON(t, w, map[string]any{"access_token": "token", "expires_in": 300})
		case "/admin/realms/test/roles/missing/users":
			http.Error(w, "missing role", http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	_, err := newTestClient(server.URL).UsersByRealmRole(context.Background(), "missing", UsersByRealmRoleOptions{})
	if !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestUsersByRealmRoleReturnsUpstreamErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{
		{name: "forbidden", status: http.StatusForbidden},
		{name: "server error", status: http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/token":
					writeJSON(t, w, map[string]any{"access_token": "token", "expires_in": 300})
				case "/admin/realms/test/roles/app_access/users":
					http.Error(w, tc.name, tc.status)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			_, err := newTestClient(server.URL).UsersByRealmRole(context.Background(), "app_access", UsersByRealmRoleOptions{})
			var upstreamErr *UpstreamError
			if !errors.As(err, &upstreamErr) {
				t.Fatalf("expected UpstreamError, got %T %v", err, err)
			}
			if upstreamErr.StatusCode != tc.status {
				t.Fatalf("expected status %d, got %d", tc.status, upstreamErr.StatusCode)
			}
		})
	}
}

func TestTokenEndpointUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			http.Error(w, "token down", http.StatusServiceUnavailable)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, err := newTestClient(server.URL).UsersByRealmRole(context.Background(), "app_access", UsersByRealmRoleOptions{})
	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected token UpstreamError, got %T %v", err, err)
	}
	if upstreamErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected token status 503, got %d", upstreamErr.StatusCode)
	}
}

func newTestClient(baseURL string) *Client {
	return New(Config{
		BaseURL:      baseURL,
		Realm:        "test",
		TokenURL:     baseURL + "/token",
		ClientID:     "client",
		ClientSecret: "secret",
	})
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func userJSON(id, username, firstName, lastName, email string, enabled bool) map[string]any {
	return map[string]any{
		"id":        id,
		"username":  username,
		"firstName": firstName,
		"lastName":  lastName,
		"email":     email,
		"enabled":   enabled,
	}
}

func emails(users []User) []string {
	result := make([]string, 0, len(users))
	for _, user := range users {
		result = append(result, user.Email)
	}
	return result
}

func firstOffsets(values []url.Values) []string {
	result := make([]string, 0, len(values))
	for _, query := range values {
		if strings.HasPrefix(query.Get("first"), "-") {
			continue
		}
		result = append(result, query.Get("first"))
	}
	return result
}
