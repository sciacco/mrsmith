package binocolo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/openapiit"
)

func TestCompanySearchCacheKeyNormalizesQueryOrderAndExcludesForceRefresh(t *testing.T) {
	first := mustParseCompanySearchRequest(t, "province=ag&companyName=ACME&limit=10&skip=0&dry_run=true&dataEnrichment=start")
	second := mustParseCompanySearchRequest(t, "force_refresh=true&skip=0&dataEnrichment=start&dry_run=true&limit=10&companyName=ACME&province=AG")

	if first.cacheKey != second.cacheKey {
		t.Fatalf("cache keys differ for equivalent searches: %q != %q", first.cacheKey, second.cacheKey)
	}
	if !second.forceRefresh {
		t.Fatal("expected second request to parse force_refresh=true")
	}
}

func TestCompanySearchCacheKeyExcludesEmptyFields(t *testing.T) {
	withEmpty := mustParseCompanySearchRequest(t, "province=AG&dry_run=false&companyName=&atecoCode=%20%20&limit=10")
	withoutEmpty := mustParseCompanySearchRequest(t, "province=AG&dry_run=false&limit=10")

	if withEmpty.cacheKey != withoutEmpty.cacheKey {
		t.Fatalf("cache keys differ when only empty fields were added: %q != %q", withEmpty.cacheKey, withoutEmpty.cacheKey)
	}

	var params map[string]string
	if err := json.Unmarshal(withEmpty.paramsJSON, &params); err != nil {
		t.Fatalf("unmarshal params JSON: %v", err)
	}
	if _, ok := params["companyName"]; ok {
		t.Fatalf("companyName should be excluded from params JSON: %#v", params)
	}
	if _, ok := params["atecoCode"]; ok {
		t.Fatalf("atecoCode should be excluded from params JSON: %#v", params)
	}
	if got := params["dryRun"]; got != "0" {
		t.Fatalf("dryRun param = %q, want 0", got)
	}
}

func TestCompanySearchValidCacheAvoidsOpenAPIIT(t *testing.T) {
	searchReq := mustParseCompanySearchRequest(t, "province=AG&dry_run=true&dataEnrichment=start&limit=10")
	store := newMemoryCompanySearchCache()
	store.entries[searchReq.cacheKey] = memoryCompanySearchCacheEntry{
		response:  json.RawMessage(`{"data":{"source":"cache"},"success":true,"message":"cached","error":null}`),
		expiresAt: time.Now().Add(time.Hour),
	}
	h := &Handler{companySearchCache: store}

	rec := performCompanySearch(t, h, "province=AG&dry_run=true&dataEnrichment=start&limit=10")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := responseSource(t, rec); got != "cache" {
		t.Fatalf("response source = %q, want cache", got)
	}
	if store.upsertCalls != 0 || store.lockCalls != 0 {
		t.Fatalf("cache hit should not lock or upsert: locks=%d upserts=%d", store.lockCalls, store.upsertCalls)
	}
}

func TestCompanySearchExpiredCacheRefreshesAndUpdatesTTL(t *testing.T) {
	searchReq := mustParseCompanySearchRequest(t, "province=AG&dry_run=true&dataEnrichment=start&limit=10")
	store := newMemoryCompanySearchCache()
	store.entries[searchReq.cacheKey] = memoryCompanySearchCacheEntry{
		response:  json.RawMessage(`{"data":{"source":"stale"},"success":true,"message":"stale","error":null}`),
		expiresAt: time.Now().Add(-time.Minute),
	}
	var calls int
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"source":"fresh"},"success":true,"message":"fresh","error":null}`))
	})
	h := &Handler{openapiit: client, companySearchCache: store}

	rec := performCompanySearch(t, h, "province=AG&dry_run=true&dataEnrichment=start&limit=10")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls)
	}
	if got := responseSource(t, rec); got != "fresh" {
		t.Fatalf("response source = %q, want fresh", got)
	}
	entry := store.entry(searchReq.cacheKey)
	if entry.expiresAt.Sub(entry.fetchedAt) != companySearchCacheTTL {
		t.Fatalf("cache TTL = %s, want %s", entry.expiresAt.Sub(entry.fetchedAt), companySearchCacheTTL)
	}
	if got := sourceFromRaw(t, entry.response); got != "fresh" {
		t.Fatalf("stored source = %q, want fresh", got)
	}
}

func TestCompanySearchForceRefreshBypassesValidCache(t *testing.T) {
	searchReq := mustParseCompanySearchRequest(t, "province=AG&dry_run=true&dataEnrichment=start&limit=10")
	store := newMemoryCompanySearchCache()
	store.entries[searchReq.cacheKey] = memoryCompanySearchCacheEntry{
		response:  json.RawMessage(`{"data":{"source":"cache"},"success":true,"message":"cached","error":null}`),
		expiresAt: time.Now().Add(time.Hour),
	}
	var calls int
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"source":"forced"},"success":true,"message":"forced","error":null}`))
	})
	h := &Handler{openapiit: client, companySearchCache: store}

	rec := performCompanySearch(t, h, "province=AG&dry_run=true&dataEnrichment=start&limit=10&force_refresh=true")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls)
	}
	if got := responseSource(t, rec); got != "forced" {
		t.Fatalf("response source = %q, want forced", got)
	}
	if got := sourceFromRaw(t, store.entry(searchReq.cacheKey).response); got != "forced" {
		t.Fatalf("stored source = %q, want forced", got)
	}
}

func TestCompanySearchRefreshFailureDoesNotOverwriteExistingCache(t *testing.T) {
	searchReq := mustParseCompanySearchRequest(t, "province=AG&dry_run=true&dataEnrichment=start&limit=10")
	store := newMemoryCompanySearchCache()
	store.entries[searchReq.cacheKey] = memoryCompanySearchCacheEntry{
		response:  json.RawMessage(`{"data":{"source":"cache"},"success":true,"message":"cached","error":null}`),
		expiresAt: time.Now().Add(time.Hour),
	}
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"success":false,"message":"upstream failed","error":502}`))
	})
	h := &Handler{openapiit: client, companySearchCache: store}

	rec := performCompanySearch(t, h, "province=AG&dry_run=true&dataEnrichment=start&limit=10&force_refresh=true")

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.upsertCalls != 0 {
		t.Fatalf("failed refresh should not upsert, got %d upserts", store.upsertCalls)
	}
	if got := sourceFromRaw(t, store.entry(searchReq.cacheKey).response); got != "cache" {
		t.Fatalf("stored source = %q, want cache", got)
	}
}

func TestCompanySearchDryRunAndRealSearchDoNotCollide(t *testing.T) {
	store := newMemoryCompanySearchCache()
	var calls int
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		source := "real"
		if r.URL.Query().Get("dryRun") == "1" {
			source = "dry"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data":    map[string]string{"source": source},
			"success": true,
			"message": source,
			"error":   nil,
		})
	})
	h := &Handler{openapiit: client, companySearchCache: store}

	dry := performCompanySearch(t, h, "province=AG&dry_run=true&dataEnrichment=start&limit=10")
	real := performCompanySearch(t, h, "province=AG&dry_run=false&dataEnrichment=start&limit=10")
	dryAgain := performCompanySearch(t, h, "province=AG&dry_run=true&dataEnrichment=start&limit=10")

	if dry.Code != http.StatusOK || real.Code != http.StatusOK || dryAgain.Code != http.StatusOK {
		t.Fatalf("unexpected statuses: dry=%d real=%d dryAgain=%d", dry.Code, real.Code, dryAgain.Code)
	}
	if responseSource(t, dry) != "dry" || responseSource(t, dryAgain) != "dry" {
		t.Fatalf("dry-run responses should use dry cache: first=%q again=%q", responseSource(t, dry), responseSource(t, dryAgain))
	}
	if got := responseSource(t, real); got != "real" {
		t.Fatalf("real response source = %q, want real", got)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d, want 2", calls)
	}
	if got := store.len(); got != 2 {
		t.Fatalf("cache entry count = %d, want 2", got)
	}
}

func TestCachedCompanySearchTraceDistinguishesCacheAndUpstream(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	dryRun := 1
	params := openapiit.CompanyITSearchParams{
		DryRun:         &dryRun,
		DataEnrichment: "advanced",
		Province:       "MI",
		ActivityStatus: "ATTIVA",
	}
	cacheKey, _, err := companySearchCacheKey(params)
	if err != nil {
		t.Fatalf("cache key: %v", err)
	}

	cacheStore := newMemoryCompanySearchCache()
	cacheStore.entries[cacheKey] = memoryCompanySearchCacheEntry{
		response:  json.RawMessage(`{"data":{},"success":true,"message":"cached","error":null,"count":3,"cost":0.01}`),
		expiresAt: now.Add(time.Hour),
	}
	traceStore := &fakeMAWorkspaceStore{}
	service := &maService{
		store:       traceStore,
		searchCache: cacheStore,
		now:         func() time.Time { return now },
	}
	trace, err := service.startTrace(context.Background(), maTraceStart{Operation: "ma_session_estimate"})
	if err != nil {
		t.Fatalf("start cache trace: %v", err)
	}
	_, _, err = service.cachedCompanySearch(withMATrace(context.Background(), trace), params, "", "")
	if err != nil {
		t.Fatalf("cached search: %v", err)
	}
	if event := traceEvent(traceStore.events, "company_search_cache_hit"); event == nil || event.ExternalSystem != "binocolo_cache" {
		t.Fatalf("cache hit event = %#v, want binocolo_cache", event)
	}

	upstreamStore := &fakeMAWorkspaceStore{}
	upstreamCache := newMemoryCompanySearchCache()
	client := newCompanySearchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{},"success":true,"message":"fresh","error":null,"count":4,"cost":0.02}`))
	})
	upstreamService := &maService{
		store:       upstreamStore,
		searchCache: upstreamCache,
		openapiit:   client,
		now:         func() time.Time { return now },
	}
	upstreamTrace, err := upstreamService.startTrace(context.Background(), maTraceStart{Operation: "ma_session_estimate"})
	if err != nil {
		t.Fatalf("start upstream trace: %v", err)
	}
	_, _, err = upstreamService.cachedCompanySearch(withMATrace(context.Background(), upstreamTrace), params, "", "")
	if err != nil {
		t.Fatalf("upstream search: %v", err)
	}
	if event := traceEvent(upstreamStore.events, "company_search_cache_miss"); event == nil || event.ExternalSystem != "binocolo_cache" {
		t.Fatalf("cache miss event = %#v, want binocolo_cache", event)
	}
	if event := traceEvent(upstreamStore.events, "company_search_upstream"); event == nil || event.ExternalSystem != "openapiit" {
		t.Fatalf("upstream event = %#v, want openapiit", event)
	}
	if event := traceEvent(upstreamStore.events, "company_search_cache_write"); event == nil || event.ExternalSystem != "binocolo_cache" {
		t.Fatalf("cache write event = %#v, want binocolo_cache", event)
	}
}

func mustParseCompanySearchRequest(t *testing.T, rawQuery string) companySearchRequest {
	t.Helper()
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	searchReq, code, err := parseCompanySearchRequest(context.Background(), values, nil)
	if code != "" {
		t.Fatalf("unexpected bad request code %q", code)
	}
	if err != nil {
		t.Fatalf("parse company search request: %v", err)
	}
	return searchReq
}

func performCompanySearch(t *testing.T, h *Handler, rawQuery string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/binocolo/v1/companies/search?"+rawQuery, nil)
	rec := httptest.NewRecorder()
	h.handleSearchCompanies(rec, req)
	return rec
}

func newCompanySearchTestClient(t *testing.T, handler http.HandlerFunc) *openapiit.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/IT-search" {
			t.Errorf("unexpected upstream path %q", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return openapiit.NewWithBaseURLs("test-token", server.URL, server.URL, server.Client())
}

func responseSource(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	return sourceFromRaw(t, rec.Body.Bytes())
}

func sourceFromRaw(t *testing.T, raw []byte) string {
	t.Helper()
	var payload struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal response: %v; raw=%s", err, string(raw))
	}
	return payload.Data["source"]
}

type memoryCompanySearchCache struct {
	mu          sync.Mutex
	entries     map[string]memoryCompanySearchCacheEntry
	lockCalls   int
	upsertCalls int
}

type memoryCompanySearchCacheEntry struct {
	response       json.RawMessage
	params         json.RawMessage
	dryRun         bool
	dataEnrichment string
	fetchedAt      time.Time
	expiresAt      time.Time
	servedCount    int
	lastServedAt   time.Time
}

func newMemoryCompanySearchCache() *memoryCompanySearchCache {
	return &memoryCompanySearchCache{entries: make(map[string]memoryCompanySearchCacheEntry)}
}

func (s *memoryCompanySearchCache) GetValidCompanySearch(_ context.Context, cacheKey string, now time.Time) (*companySearchCacheEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[cacheKey]
	if !ok || !entry.expiresAt.After(now) {
		return nil, nil
	}
	entry.servedCount++
	entry.lastServedAt = now
	s.entries[cacheKey] = entry
	return &companySearchCacheEntry{Response: json.RawMessage(append([]byte(nil), entry.response...))}, nil
}

func (s *memoryCompanySearchCache) WithCompanySearchCacheLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	s.mu.Lock()
	s.lockCalls++
	s.mu.Unlock()
	return fn(ctx)
}

func (s *memoryCompanySearchCache) UpsertCompanySearch(_ context.Context, input companySearchCacheWrite) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upsertCalls++
	s.entries[input.CacheKey] = memoryCompanySearchCacheEntry{
		response:       json.RawMessage(append([]byte(nil), input.Response...)),
		params:         json.RawMessage(append([]byte(nil), input.Params...)),
		dryRun:         input.DryRun,
		dataEnrichment: input.DataEnrichment,
		fetchedAt:      input.FetchedAt,
		expiresAt:      input.ExpiresAt,
	}
	return nil
}

func (s *memoryCompanySearchCache) entry(cacheKey string) memoryCompanySearchCacheEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.entries[cacheKey]
}

func (s *memoryCompanySearchCache) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}
