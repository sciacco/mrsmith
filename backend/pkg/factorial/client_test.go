package factorial

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// --- local test schema -------------------------------------------------------

type widget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type updateWidget struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type staticParams struct{ v url.Values }

func (s staticParams) encode() url.Values { return s.v }

// --- helpers -----------------------------------------------------------------

type captured struct {
	req  *http.Request
	body []byte
}

func captureServer(t *testing.T, status int, resp []byte, cap *captured) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cap != nil {
			b, _ := io.ReadAll(r.Body)
			cap.req = r
			cap.body = b
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(resp)
	}))
}

// doRequest stands up a capture server, builds a client pointed at it, and
// performs a single do() call. It returns the captured request.
func doRequest(t *testing.T, opts ...Option) *http.Request {
	t.Helper()
	var cap captured
	srv := captureServer(t, http.StatusOK, []byte(`{}`), &cap)
	t.Cleanup(srv.Close)
	all := append([]Option{WithBaseURL(srv.URL)}, opts...)
	c := New(all...)
	if err := c.do(context.Background(), http.MethodGet, "/ping", nil, nil, nil); err != nil {
		t.Fatalf("do: %v", err)
	}
	return cap.req
}

// --- auth / options / env precedence -----------------------------------------

func TestClientAuthEnvAndOptions(t *testing.T) {
	t.Run("explicit api key beats env", func(t *testing.T) {
		t.Setenv("FACTORIAL_API_KEY", "envkey")
		req := doRequest(t, WithAPIKey("explicit"))
		if got := req.Header.Get("x-api-key"); got != "explicit" {
			t.Errorf("x-api-key = %q, want %q", got, "explicit")
		}
	})

	t.Run("env api key used when no option", func(t *testing.T) {
		t.Setenv("FACTORIAL_API_KEY", "envkey")
		req := doRequest(t)
		if got := req.Header.Get("x-api-key"); got != "envkey" {
			t.Errorf("x-api-key = %q, want %q", got, "envkey")
		}
	})

	t.Run("env token used as bearer", func(t *testing.T) {
		t.Setenv("FACTORIAL_TOKEN", "envtok")
		req := doRequest(t)
		if got := req.Header.Get("Authorization"); got != "Bearer envtok" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer envtok")
		}
	})

	t.Run("both headers when both set", func(t *testing.T) {
		t.Setenv("FACTORIAL_API_KEY", "envkey")
		t.Setenv("FACTORIAL_TOKEN", "envtok")
		req := doRequest(t)
		if got := req.Header.Get("x-api-key"); got != "envkey" {
			t.Errorf("x-api-key = %q, want envkey", got)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer envtok" {
			t.Errorf("Authorization = %q, want Bearer envtok", got)
		}
	})

	t.Run("explicit token beats env", func(t *testing.T) {
		t.Setenv("FACTORIAL_TOKEN", "envtok")
		req := doRequest(t, WithToken("explicit"))
		if got := req.Header.Get("Authorization"); got != "Bearer explicit" {
			t.Errorf("Authorization = %q, want Bearer explicit", got)
		}
	})

	t.Run("no auth headers when nothing configured", func(t *testing.T) {
		t.Setenv("FACTORIAL_API_KEY", "")
		t.Setenv("FACTORIAL_TOKEN", "")
		req := doRequest(t)
		if got := req.Header.Get("x-api-key"); got != "" {
			t.Errorf("x-api-key should be absent, got %q", got)
		}
		if got := req.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization should be absent, got %q", got)
		}
	})
}

func TestClientBaseURLDefaultFromEnv(t *testing.T) {
	t.Setenv("FACTORIAL_BASE_URL", "https://example.test")
	c := New()
	if c.baseURL != "https://example.test" {
		t.Errorf("baseURL from env = %q", c.baseURL)
	}
}

func TestClientBaseURLOptionBeatsEnv(t *testing.T) {
	// Prove explicit WithBaseURL wins over FACTORIAL_BASE_URL, and that the
	// env value is used when no option is supplied. All requests are served by
	// local httptest servers; no real network is involved.

	t.Run("WithBaseURL overrides FACTORIAL_BASE_URL", func(t *testing.T) {
		var cap captured
		srv := captureServer(t, http.StatusOK, []byte(`{}`), &cap)
		t.Cleanup(srv.Close)

		t.Setenv("FACTORIAL_BASE_URL", "https://env-invalid.example.com")
		c := New(WithBaseURL(srv.URL))
		if err := c.do(context.Background(), http.MethodGet, "/ping", nil, nil, nil); err != nil {
			t.Fatalf("do: %v", err)
		}
		if cap.req == nil {
			t.Fatal("request did not reach the option base URL (handler never invoked)")
		}
		if cap.req.Host == "env-invalid.example.com" {
			t.Errorf("request routed to env URL host %q, want option server", cap.req.Host)
		}
	})

	t.Run("FACTORIAL_BASE_URL used when no option", func(t *testing.T) {
		var cap captured
		srv := captureServer(t, http.StatusOK, []byte(`{}`), &cap)
		t.Cleanup(srv.Close)

		t.Setenv("FACTORIAL_BASE_URL", srv.URL)
		c := New() // no WithBaseURL -> env must win
		if err := c.do(context.Background(), http.MethodGet, "/ping", nil, nil, nil); err != nil {
			t.Fatalf("do: %v", err)
		}
		if cap.req == nil {
			t.Fatal("request did not reach the env base URL (handler never invoked)")
		}
	})
}

func TestClientTrailingSlashTrimmed(t *testing.T) {
	var cap captured
	srv := captureServer(t, http.StatusOK, []byte(`{}`), &cap)
	t.Cleanup(srv.Close)
	c := New(WithBaseURL(srv.URL + "/"))
	if err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if want := "/api/2026-07-01/x"; cap.req.URL.Path != want {
		t.Errorf("URL path = %q, want %q (no double slash)", cap.req.URL.Path, want)
	}
}

func TestClientAcceptAndUserAgentHeaders(t *testing.T) {
	t.Run("default user agent", func(t *testing.T) {
		req := doRequest(t)
		if got := req.Header.Get("User-Agent"); got != defaultUserAgent {
			t.Errorf("User-Agent = %q, want %q", got, defaultUserAgent)
		}
		if got := req.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
	})
	t.Run("custom user agent", func(t *testing.T) {
		req := doRequest(t, WithUserAgent("custom/1.0"))
		if got := req.Header.Get("User-Agent"); got != "custom/1.0" {
			t.Errorf("User-Agent = %q, want custom/1.0", got)
		}
	})
}

// --- error mapping -----------------------------------------------------------

func TestClientErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"404 not found", http.StatusNotFound, `{"error":"missing"}`},
		{"500 server error", http.StatusInternalServerError, `{"error":"boom"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cap captured
			srv := captureServer(t, tc.status, []byte(tc.body), &cap)
			t.Cleanup(srv.Close)
			c := New(WithBaseURL(srv.URL))

			err := c.do(context.Background(), http.MethodPost, "/things", nil, nil, nil)

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError via errors.As, got %T: %v", err, err)
			}
			if apiErr.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tc.status)
			}
			if apiErr.Method != http.MethodPost {
				t.Errorf("Method = %q, want POST", apiErr.Method)
			}
			wantURL := srv.URL + "/api/2026-07-01/things"
			if apiErr.URL != wantURL {
				t.Errorf("URL = %q, want %q", apiErr.URL, wantURL)
			}
			if !bytes.Equal(apiErr.Body, []byte(tc.body)) {
				t.Errorf("Body = %q, want %q", apiErr.Body, tc.body)
			}
			// The version-less path supplied to do() must appear versioned on
			// the wire (the client owns the prefix).
			if cap.req.URL.Path != "/api/2026-07-01/things" {
				t.Errorf("wire path = %q, want /api/2026-07-01/things", cap.req.URL.Path)
			}
		})
	}
}

func TestClientErrorBodyCapped(t *testing.T) {
	// A pathological 4xx body must be capped at maxErrorBody inside APIError.
	huge := bytes.Repeat([]byte("x"), maxErrorBody+2048)
	srv := captureServer(t, http.StatusBadGateway, huge, nil)
	t.Cleanup(srv.Close)
	c := New(WithBaseURL(srv.URL))
	err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %v", err)
	}
	if len(apiErr.Body) != maxErrorBody {
		t.Errorf("captured body len = %d, want %d", len(apiErr.Body), maxErrorBody)
	}
}

// --- transport + decode behavior --------------------------------------------

func TestClientTransportErrorUnwrapped(t *testing.T) {
	// A cancelled context fails the round trip before any response; the error
	// must be the raw transport error, not wrapped in *APIError.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := New() // default base URL; request never reaches the wire
	err := c.do(ctx, http.MethodGet, "/x", nil, nil, nil)
	if err == nil {
		t.Fatal("expected transport error, got nil")
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Errorf("transport error must not be *APIError: %v", err)
	}
}

func TestClientNilOutSkipsDecode(t *testing.T) {
	srv := captureServer(t, http.StatusOK, []byte(`{"whatever":"payload"}`), nil)
	t.Cleanup(srv.Close)
	c := New(WithBaseURL(srv.URL))
	if err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, nil); err != nil {
		t.Fatalf("out==nil on 2xx must not error, got %v", err)
	}
}

func TestClientDecodeErrorWrapped(t *testing.T) {
	srv := captureServer(t, http.StatusOK, []byte(`{not-valid-json`), nil)
	t.Cleanup(srv.Close)
	c := New(WithBaseURL(srv.URL))
	err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, new(widget))
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "factorial: decoding response") {
		t.Errorf("error must be wrapped as decoding response, got %q", err.Error())
	}
}

func TestClientMarshalErrorWrapped(t *testing.T) {
	// A channel cannot be JSON-marshaled; the error must surface before any
	// network call.
	c := New()
	err := c.do(context.Background(), http.MethodPost, "/x", nil, make(chan int), nil)
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "factorial: marshaling request body") {
		t.Errorf("error must be wrapped as marshaling request body, got %q", err.Error())
	}
}

func TestClientTypedNilBodySkipped(t *testing.T) {
	var cap captured
	srv := captureServer(t, http.StatusOK, []byte(`{"id":"1","name":"x"}`), &cap)
	t.Cleanup(srv.Close)
	c := New(WithBaseURL(srv.URL))
	var nilBody *updateWidget
	if _, err := doCreate[widget](context.Background(), c, "/widgets", nilBody); err != nil {
		t.Fatalf("doCreate with typed-nil body: %v", err)
	}
	if got := cap.req.Header.Get("Content-Type"); got != "" {
		t.Errorf("typed-nil body must skip Content-Type, got %q", got)
	}
	if len(cap.body) != 0 {
		t.Errorf("typed-nil body must send no bytes, got %q", cap.body)
	}
}

// --- doUpdate id injection ---------------------------------------------------

func TestDoUpdateInjectsID(t *testing.T) {
	var cap captured
	srv := captureServer(t, http.StatusOK, []byte(`{"id":"42","name":"foo"}`), &cap)
	t.Cleanup(srv.Close)
	c := New(WithBaseURL(srv.URL))

	body := updateWidget{Name: "foo", Tags: []string{"a", "b"}}
	got, err := doUpdate[widget](context.Background(), c, "/widgets", "42", body)
	if err != nil {
		t.Fatalf("doUpdate: %v", err)
	}

	if cap.req.Method != http.MethodPut {
		t.Errorf("method = %q, want PUT", cap.req.Method)
	}
	if cap.req.URL.Path != "/api/2026-07-01/widgets/42" {
		t.Errorf("URL path = %q", cap.req.URL.Path)
	}
	if ct := cap.req.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}

	var decoded map[string]any
	if err := json.Unmarshal(cap.body, &decoded); err != nil {
		t.Fatalf("unmarshal sent body: %v (body=%q)", err, cap.body)
	}
	if decoded["id"] != "42" {
		t.Errorf("injected id = %v, want 42", decoded["id"])
	}
	if decoded["name"] != "foo" {
		t.Errorf("name = %v, want foo", decoded["name"])
	}
	tags, _ := decoded["tags"].([]any)
	if len(tags) != 2 {
		t.Errorf("tags = %v, want 2 entries", tags)
	}

	if got.ID != "42" || got.Name != "foo" {
		t.Errorf("decoded response = %+v", got)
	}
}

func TestDoUpdateOverwritesExistingID(t *testing.T) {
	// A body that already carries an "id" must have it OVERWRITTEN by the path
	// id; the path is authoritative for PUTs.
	var cap captured
	srv := captureServer(t, http.StatusOK, []byte(`{"id":"42","name":"x"}`), &cap)
	t.Cleanup(srv.Close)
	c := New(WithBaseURL(srv.URL))

	body := map[string]any{"id": "wrong", "name": "x"}
	if _, err := doUpdate[widget](context.Background(), c, "/widgets", "42", body); err != nil {
		t.Fatalf("doUpdate: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(cap.body, &decoded); err != nil {
		t.Fatalf("unmarshal sent body: %v (body=%q)", err, cap.body)
	}
	if decoded["id"] != "42" {
		t.Errorf("id = %v, want 42 (must overwrite body's existing id)", decoded["id"])
	}
	if decoded["name"] != "x" {
		t.Errorf("name = %v, want x", decoded["name"])
	}
}

func TestDoUpdateNilBodyStillInjectsID(t *testing.T) {
	var cap captured
	srv := captureServer(t, http.StatusOK, []byte(`{"id":"7"}`), &cap)
	t.Cleanup(srv.Close)
	c := New(WithBaseURL(srv.URL))

	var nilBody *updateWidget
	if _, err := doUpdate[widget](context.Background(), c, "/widgets", "7", nilBody); err != nil {
		t.Fatalf("doUpdate nil body: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(cap.body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["id"] != "7" {
		t.Errorf("id = %v, want 7", decoded["id"])
	}
	if len(decoded) != 1 {
		t.Errorf("expected only {id}, got %v", decoded)
	}
}

// --- generic helper round-trips ---------------------------------------------

func TestGenericHelpersRoundTrip(t *testing.T) {
	t.Run("doList with params", func(t *testing.T) {
		payload := `{"data":[{"id":"1","name":"a"},{"id":"2","name":"b"}],"meta":{"limit":2,"total":2,"has_next_page":false}}`
		var cap captured
		srv := captureServer(t, http.StatusOK, []byte(payload), &cap)
		t.Cleanup(srv.Close)
		c := New(WithBaseURL(srv.URL))

		page, err := doList[widget](context.Background(), c, "/widgets", staticParams{url.Values{"limit": {"10"}}})
		if err != nil {
			t.Fatal(err)
		}
		if cap.req.Method != http.MethodGet {
			t.Errorf("method = %q", cap.req.Method)
		}
		if cap.req.URL.RawQuery != "limit=10" {
			t.Errorf("query = %q, want limit=10", cap.req.URL.RawQuery)
		}
		if len(page.Data) != 2 || page.Data[0].ID != "1" || page.Data[1].Name != "b" {
			t.Errorf("page.Data = %+v", page.Data)
		}
		if page.Meta.Total != 2 || page.Meta.Limit != 2 {
			t.Errorf("page.Meta = %+v", page.Meta)
		}
	})

	t.Run("doList nil params", func(t *testing.T) {
		srv := captureServer(t, http.StatusOK, []byte(`{"data":[],"meta":{}}`), nil)
		t.Cleanup(srv.Close)
		c := New(WithBaseURL(srv.URL))
		page, err := doList[widget](context.Background(), c, "/widgets", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Data) != 0 {
			t.Errorf("expected empty page, got %+v", page.Data)
		}
	})

	t.Run("doGet", func(t *testing.T) {
		var cap captured
		srv := captureServer(t, http.StatusOK, []byte(`{"id":"9","name":"n"}`), &cap)
		t.Cleanup(srv.Close)
		c := New(WithBaseURL(srv.URL))
		got, err := doGet[widget](context.Background(), c, "/widgets", "9")
		if err != nil {
			t.Fatal(err)
		}
		if cap.req.URL.Path != "/api/2026-07-01/widgets/9" {
			t.Errorf("URL path = %q", cap.req.URL.Path)
		}
		if got.ID != "9" || got.Name != "n" {
			t.Errorf("got = %+v", got)
		}
	})

	t.Run("doCreate", func(t *testing.T) {
		var cap captured
		srv := captureServer(t, http.StatusCreated, []byte(`{"id":"1","name":"foo"}`), &cap)
		t.Cleanup(srv.Close)
		c := New(WithBaseURL(srv.URL))
		got, err := doCreate[widget](context.Background(), c, "/widgets", map[string]any{"name": "foo"})
		if err != nil {
			t.Fatal(err)
		}
		if cap.req.Method != http.MethodPost {
			t.Errorf("method = %q", cap.req.Method)
		}
		if cap.req.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", cap.req.Header.Get("Content-Type"))
		}
		var sent map[string]any
		_ = json.Unmarshal(cap.body, &sent)
		if sent["name"] != "foo" {
			t.Errorf("sent body = %s", cap.body)
		}
		if got.ID != "1" {
			t.Errorf("got = %+v", got)
		}
	})

	t.Run("doDelete", func(t *testing.T) {
		var cap captured
		srv := captureServer(t, http.StatusOK, []byte(`{"id":"5","name":"gone"}`), &cap)
		t.Cleanup(srv.Close)
		c := New(WithBaseURL(srv.URL))
		got, err := doDelete[widget](context.Background(), c, "/widgets", "5")
		if err != nil {
			t.Fatal(err)
		}
		if cap.req.Method != http.MethodDelete {
			t.Errorf("method = %q", cap.req.Method)
		}
		if cap.req.URL.Path != "/api/2026-07-01/widgets/5" {
			t.Errorf("URL path = %q", cap.req.URL.Path)
		}
		if got.ID != "5" || got.Name != "gone" {
			t.Errorf("got = %+v", got)
		}
	})

	t.Run("doAction", func(t *testing.T) {
		srv := captureServer(t, http.StatusOK, []byte(`{"id":"1","name":"acted"}`), nil)
		t.Cleanup(srv.Close)
		c := New(WithBaseURL(srv.URL))
		got, err := doAction[widget](context.Background(), c, "/widgets/1/actions", map[string]any{"kind": "ping"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "acted" {
			t.Errorf("got = %+v", got)
		}
	})

	t.Run("doActionSlice", func(t *testing.T) {
		srv := captureServer(t, http.StatusOK, []byte(`[{"id":"1","name":"a"},{"id":"2","name":"b"}]`), nil)
		t.Cleanup(srv.Close)
		c := New(WithBaseURL(srv.URL))
		got, err := doActionSlice[widget](context.Background(), c, "/widgets/search", map[string]any{"q": "x"})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].ID != "1" || got[1].ID != "2" {
			t.Errorf("slice = %+v", got)
		}
	})

	t.Run("doActionPage", func(t *testing.T) {
		srv := captureServer(t, http.StatusOK, []byte(`{"data":[{"id":"1","name":"a"}],"meta":{"limit":1,"total":1}}`), nil)
		t.Cleanup(srv.Close)
		c := New(WithBaseURL(srv.URL))
		page, err := doActionPage[widget](context.Background(), c, "/widgets/search", map[string]any{"q": "x"})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Data) != 1 || page.Meta.Total != 1 {
			t.Errorf("page = %+v", page)
		}
	})
}

func TestWithHTTPClient(t *testing.T) {
	custom := &http.Client{}
	c := New(WithHTTPClient(custom))
	if c.httpClient != custom {
		t.Errorf("WithHTTPClient did not set the client")
	}
}

func TestGenericHelpersErrorPaths(t *testing.T) {
	// Every generic helper must surface a non-2xx as a *APIError rather than
	// swallowing it.
	srv := captureServer(t, http.StatusInternalServerError, []byte(`{"error":"boom"}`), nil)
	t.Cleanup(srv.Close)
	c := New(WithBaseURL(srv.URL))
	ctx := context.Background()

	check := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("expected error")
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusInternalServerError {
			t.Errorf("StatusCode = %d", apiErr.StatusCode)
		}
	}

	t.Run("doList", func(t *testing.T) {
		_, err := doList[widget](ctx, c, "/widgets", nil)
		check(t, err)
	})
	t.Run("doGet", func(t *testing.T) {
		_, err := doGet[widget](ctx, c, "/widgets", "1")
		check(t, err)
	})
	t.Run("doCreate", func(t *testing.T) {
		_, err := doCreate[widget](ctx, c, "/widgets", map[string]any{"name": "x"})
		check(t, err)
	})
	t.Run("doUpdate", func(t *testing.T) {
		_, err := doUpdate[widget](ctx, c, "/widgets", "1", updateWidget{Name: "x"})
		check(t, err)
	})
	t.Run("doDelete", func(t *testing.T) {
		_, err := doDelete[widget](ctx, c, "/widgets", "1")
		check(t, err)
	})
	t.Run("doAction", func(t *testing.T) {
		_, err := doAction[widget](ctx, c, "/widgets/1/action", nil)
		check(t, err)
	})
	t.Run("doActionSlice", func(t *testing.T) {
		_, err := doActionSlice[widget](ctx, c, "/widgets/search", nil)
		check(t, err)
	})
	t.Run("doActionPage", func(t *testing.T) {
		_, err := doActionPage[widget](ctx, c, "/widgets/search", nil)
		check(t, err)
	})
}

func TestPageMetaJSONShape(t *testing.T) {
	// Smoke-check the JSON tags round-trip the spec field names.
	in := `{"data":[{"id":"1","name":"a"}],"meta":{"start_cursor":"a","end_cursor":"b","has_previous_page":true,"has_next_page":false,"limit":50,"total":123}}`
	var p Page[widget]
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}
	if p.Meta.StartCursor == nil || *p.Meta.StartCursor != "a" {
		t.Errorf("start_cursor: %+v", p.Meta)
	}
	if !p.Meta.HasPreviousPage || p.Meta.HasNextPage {
		t.Errorf("page flags: %+v", p.Meta)
	}
	if p.Meta.Limit != 50 || p.Meta.Total != 123 {
		t.Errorf("limit/total: %+v", p.Meta)
	}
}

func TestPtr(t *testing.T) {
	s := Ptr("hello")
	if s == nil || *s != "hello" {
		t.Errorf("Ptr(string) = %v", s)
	}
	n := Ptr(42)
	if n == nil || *n != 42 {
		t.Errorf("Ptr(int) = %v", n)
	}
	b := Ptr(false)
	if b == nil || *b != false {
		t.Errorf("Ptr(bool) = %v", b)
	}
	if a, bb := Ptr(1), Ptr(1); a == bb {
		t.Error("Ptr must return a fresh pointer per call")
	}
}
