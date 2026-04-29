package rda

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/arak"
)

func TestHandleArticlesMergesGoodAndServiceCatalogs(t *testing.T) {
	h, state := newArticleCatalogHandler(t, map[string][]article{
		"good": {
			{Code: "GOOD-1", Description: "Monitor"},
			{Code: "DUP", Description: "Zed duplicate as good"},
		},
		"service": {
			{Code: "DUP", Description: "Alpha duplicate as service"},
			{Code: "SVC-1", Description: "Backup service"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/rda/v1/articles", nil)
	rec := httptest.NewRecorder()
	h.handleArticles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body articleCatalogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.TotalNumber != 3 || len(body.Items) != 3 {
		t.Fatalf("expected 3 deduped articles, got total=%d items=%#v", body.TotalNumber, body.Items)
	}
	if body.Items[0] != (article{Code: "SVC-1", Description: "Backup service", Type: "service"}) {
		t.Fatalf("expected merged catalog to sort services and goods alphabetically, got %#v", body.Items)
	}
	if body.Items[2] != (article{Code: "DUP", Description: "Zed duplicate as good", Type: "good"}) {
		t.Fatalf("expected duplicate code to keep first fetched good item, got %#v", body.Items[2])
	}

	calls := state.articleCalls()
	if len(calls) != 2 {
		t.Fatalf("expected good and service upstream calls, got %#v", calls)
	}
	for _, call := range calls {
		if got := call.query.Get("page_number"); got != "1" {
			t.Fatalf("expected page_number default, got %q", got)
		}
		if got := call.query.Get("disable_pagination"); got != "true" {
			t.Fatalf("expected disable_pagination default, got %q", got)
		}
	}
	if calls[0].query.Get("type") != "good" || calls[1].query.Get("type") != "service" {
		t.Fatalf("expected stable good/service fetch order, got %#v", calls)
	}
}

func TestHandleArticlesSearchIncludesServices(t *testing.T) {
	h, state := newArticleCatalogHandler(t, map[string][]article{
		"good": {
			{Code: "GOOD-1", Description: "Monitor"},
		},
		"service": {
			{Code: "SVC-1", Description: "Cloud backup service"},
			{Code: "SVC-2", Description: "Network support"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/rda/v1/articles?search=cloud", nil)
	rec := httptest.NewRecorder()
	h.handleArticles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body articleCatalogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0] != (article{Code: "SVC-1", Description: "Cloud backup service", Type: "service"}) {
		t.Fatalf("expected search to include service matches, got %#v", body.Items)
	}

	calls := state.articleCalls()
	if len(calls) != 2 {
		t.Fatalf("expected good and service upstream searches, got %#v", calls)
	}
	for _, call := range calls {
		if got := call.query.Get("search_string"); got != "cloud" {
			t.Fatalf("expected search_string to be propagated, got %q in %#v", got, call.query)
		}
	}
}

func TestHandleArticlesKeepsTypeFilter(t *testing.T) {
	h, state := newArticleCatalogHandler(t, map[string][]article{
		"good": {
			{Code: "GOOD-1", Description: "Monitor"},
		},
		"service": {
			{Code: "SVC-1", Description: "Managed service"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/rda/v1/articles?type=service&search_string=managed", nil)
	rec := httptest.NewRecorder()
	h.handleArticles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body articleCatalogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0] != (article{Code: "SVC-1", Description: "Managed service", Type: "service"}) {
		t.Fatalf("unexpected response: %#v", body.Items)
	}

	calls := state.articleCalls()
	if len(calls) != 1 {
		t.Fatalf("expected one typed upstream call, got %#v", calls)
	}
	if calls[0].query.Get("type") != "service" || calls[0].query.Get("search_string") != "managed" {
		t.Fatalf("unexpected upstream query: %#v", calls[0].query)
	}
}

func TestDecodeArticleItemsAcceptsNestedEnvelope(t *testing.T) {
	items, err := decodeArticleItems([]byte(`{
		"data": {
			"items": [
				{"code": "SVC-1", "description": "Cloud backup"}
			]
		}
	}`), "service")

	if err != nil {
		t.Fatalf("expected nested envelope to decode, got %v", err)
	}
	if len(items) != 1 || items[0] != (article{Code: "SVC-1", Description: "Cloud backup", Type: "service"}) {
		t.Fatalf("unexpected decoded articles: %#v", items)
	}
}

func TestHandleArticlesRejectsUnknownType(t *testing.T) {
	h, state := newArticleCatalogHandler(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/rda/v1/articles?type=thing", nil)
	rec := httptest.NewRecorder()
	h.handleArticles(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if calls := state.articleCalls(); len(calls) != 0 {
		t.Fatalf("expected invalid request not to call upstream, got %#v", calls)
	}
}

type articleCatalogCall struct {
	query url.Values
}

type articleCatalogState struct {
	mu       sync.Mutex
	fixtures map[string][]article
	calls    []articleCatalogCall
}

func newArticleCatalogHandler(t *testing.T, fixtures map[string][]article) (*Handler, *articleCatalogState) {
	t.Helper()
	state := &articleCatalogState{fixtures: fixtures}
	server := httptest.NewServer(state)
	t.Cleanup(server.Close)

	client := arak.New(arak.Config{
		BaseURL:      server.URL,
		TokenURL:     server.URL + "/token",
		ClientID:     "client",
		ClientSecret: "secret",
	})
	return &Handler{arak: client}, state
}

func (s *articleCatalogState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"arak-token","expires_in":3600}`))
		return
	}

	if r.Method != http.MethodGet || r.URL.Path != "/arak/rda/v1/article" {
		http.NotFound(w, r)
		return
	}
	query := r.URL.Query()
	s.mu.Lock()
	s.calls = append(s.calls, articleCatalogCall{query: cloneValues(query)})
	s.mu.Unlock()

	articleType := strings.TrimSpace(query.Get("type"))
	items := filterArticleFixtures(s.fixtures[articleType], query.Get("search_string"))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(articleCatalogResponse{
		TotalNumber: len(items),
		CurrentPage: 1,
		TotalPages:  1,
		Items:       items,
	})
}

func filterArticleFixtures(items []article, search string) []article {
	search = strings.ToLower(strings.TrimSpace(search))
	if search == "" {
		return items
	}
	filtered := make([]article, 0, len(items))
	for _, item := range items {
		haystack := strings.ToLower(strings.Join([]string{item.Code, item.Description}, " "))
		if strings.Contains(haystack, search) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (s *articleCatalogState) articleCalls() []articleCatalogCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]articleCatalogCall, len(s.calls))
	copy(out, s.calls)
	return out
}

func cloneValues(values url.Values) url.Values {
	out := make(url.Values, len(values))
	for key, vals := range values {
		out[key] = append([]string(nil), vals...)
	}
	return out
}

var _ http.Handler = (*articleCatalogState)(nil)
