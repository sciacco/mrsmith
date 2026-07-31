package factorial

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
)

const (
	defaultBaseURL   = "https://api.factorialhr.com"
	defaultUserAgent = "factorial-api-go"
	// maxErrorBody caps the raw response body captured into an APIError so a
	// pathological server response cannot blow up memory.
	maxErrorBody = 64 << 10 // 64 KiB
)

// Client is the entry point for the Factorial API. It owns authentication,
// the HTTP transport, and the dated API version prefix. Generated namespace
// methods are bound to a *Client at construction time.
type Client struct {
	baseURL    string
	apiKey     string
	token      string
	httpClient *http.Client
	userAgent  string

	// clientNamespaces is generated in namespaces.gen.go; embedding it promotes
	// the namespace accessors (Teams, ATS, APIPublic, ...) onto Client.
	clientNamespaces
}

// Option configures a Client. Explicit options always override environment
// defaults; environment defaults always override built-in defaults.
type Option func(*Client)

// WithAPIKey authenticates requests via the x-api-key header.
func WithAPIKey(key string) Option { return func(c *Client) { c.apiKey = key } }

// WithToken authenticates requests via Authorization: Bearer <token>.
func WithToken(token string) Option { return func(c *Client) { c.token = token } }

// WithBaseURL overrides the API base URL (any trailing "/" is trimmed).
func WithBaseURL(baseURL string) Option { return func(c *Client) { c.baseURL = baseURL } }

// WithHTTPClient supplies a custom *http.Client for outbound requests.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithUserAgent overrides the default User-Agent header.
func WithUserAgent(ua string) Option { return func(c *Client) { c.userAgent = ua } }

// New builds a Client from environment defaults overlaid with the given
// options. FACTORIAL_API_KEY / FACTORIAL_TOKEN / FACTORIAL_BASE_URL seed the
// corresponding fields; any matching With* option wins.
func New(opts ...Option) *Client {
	c := &Client{
		baseURL:    firstNonEmpty(os.Getenv("FACTORIAL_BASE_URL"), defaultBaseURL),
		apiKey:     os.Getenv("FACTORIAL_API_KEY"),
		token:      os.Getenv("FACTORIAL_TOKEN"),
		httpClient: http.DefaultClient,
		userAgent:  defaultUserAgent,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	// Wire the generated namespace accessors now that baseURL/auth are settled.
	c.initNamespaces()
	return c
}

// Ptr returns a pointer to v. Optional request-body fields and nullable
// response fields are pointers throughout the generated types; Ptr lets
// callers set them inline without an intermediate variable:
//
//	Description: factorial.Ptr("Team infrastruttura")
func Ptr[T any](v T) *T { return &v }

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// do is the single transport entry point used by the generated helpers. It
// builds a versioned request, applies auth/content headers, performs the
// round trip, and decodes 2xx bodies into out. path is VERSION-LESS; the
// Client owns apiVersionPrefix.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	requestURL := c.baseURL + apiVersionPrefix + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil && !isNilAny(body) {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("factorial: marshaling request body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return fmt.Errorf("factorial: building request: %w", err)
	}
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.setCommonHeaders(req)
	return c.doRequest(req, method, requestURL, out)
}

// setCommonHeaders sets Accept, User-Agent, x-api-key, and Authorization on
// req. Content-Type is intentionally left to the caller: do() applies
// application/json only when a body is present, and the multipart helpers
// apply multipart/form-data; boundary.
func (c *Client) setCommonHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// doRequest performs the round trip for req, reads the full response body,
// maps any status >= 300 into an *APIError (with the body capped at
// maxErrorBody), and on 2xx JSON-decodes the body into out when out != nil.
// method and requestURL are recorded on any returned *APIError so the error
// is self-describing without retaining the request object.
func (c *Client) doRequest(req *http.Request, method, requestURL string, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Transport errors are returned unwrapped; they are not API errors.
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("factorial: reading response body: %w", err)
	}

	if resp.StatusCode >= 300 {
		captured := respBody
		if len(captured) > maxErrorBody {
			captured = captured[:maxErrorBody]
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       captured,
			Method:     method,
			URL:        requestURL,
		}
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("factorial: decoding response: %w", err)
		}
	}
	return nil
}

// isNilAny reports whether v is an untyped nil or a typed nil pointer. The
// generated helpers pass request bodies as `any`, so a nil *SomeBody arrives
// as a non-nil interface wrapping a nil pointer; we must detect that to avoid
// marshaling "null" bodies.
func isNilAny(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}

// queryEncoder is implemented by generated list-parameter structs to surface
// themselves as query values.
type queryEncoder interface {
	encode() url.Values
}

// Page is the generic envelope for paginated list responses.
type Page[T any] struct {
	Data []T       `json:"data"`
	Meta PagedMeta `json:"meta"`
}

// PagedMeta is the pagination metadata block shared by every Page.
type PagedMeta struct {
	StartCursor     *string `json:"start_cursor"`
	EndCursor       *string `json:"end_cursor"`
	HasPreviousPage bool    `json:"has_previous_page"`
	HasNextPage     bool    `json:"has_next_page"`
	Limit           int64   `json:"limit"`
	Total           int64   `json:"total"`
}

// doList performs a GET against path using params as the query string and
// decodes the response into a *Page[T].
func doList[T any](ctx context.Context, c *Client, path string, params queryEncoder) (*Page[T], error) {
	var query url.Values
	if params != nil {
		query = params.encode()
	}
	out := new(Page[T])
	if err := c.do(ctx, http.MethodGet, path, query, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// doGet performs a GET against path/id and decodes the response into a *T.
func doGet[T any](ctx context.Context, c *Client, path, id string) (*T, error) {
	out := new(T)
	if err := c.do(ctx, http.MethodGet, path+"/"+id, nil, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// doCreate performs a POST against path with body and decodes into a *T.
func doCreate[T any](ctx context.Context, c *Client, path string, body any) (*T, error) {
	out := new(T)
	if err := c.do(ctx, http.MethodPost, path, nil, body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// doUpdate performs a PUT against path/id with body, injecting the path id
// into the JSON body, and decodes the response into a *T.
func doUpdate[T any](ctx context.Context, c *Client, path, id string, body any) (*T, error) {
	payload, err := injectID(body, id)
	if err != nil {
		return nil, err
	}
	out := new(T)
	if err := c.do(ctx, http.MethodPut, path+"/"+id, nil, payload, out); err != nil {
		return nil, err
	}
	return out, nil
}

// doDelete performs a DELETE against path/id and decodes the (deleted) object
// into a *T.
func doDelete[T any](ctx context.Context, c *Client, path, id string) (*T, error) {
	out := new(T)
	if err := c.do(ctx, http.MethodDelete, path+"/"+id, nil, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// doAction performs a POST against path with body and decodes into a *T.
func doAction[T any](ctx context.Context, c *Client, path string, body any) (*T, error) {
	out := new(T)
	if err := c.do(ctx, http.MethodPost, path, nil, body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// doActionSlice performs a POST against path with body and decodes a
// top-level JSON array into []T.
func doActionSlice[T any](ctx context.Context, c *Client, path string, body any) ([]T, error) {
	var out []T
	if err := c.do(ctx, http.MethodPost, path, nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// doActionPage performs a POST against path with body and decodes the
// response into a *Page[T].
func doActionPage[T any](ctx context.Context, c *Client, path string, body any) (*Page[T], error) {
	out := new(Page[T])
	if err := c.do(ctx, http.MethodPost, path, nil, body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// injectID marshals body, decodes it into a generic object map, forces the
// "id" field to the path id, and re-serializes. Every PUT body in the spec
// duplicates the path id, so the generated UpdateBody structs omit it and the
// runtime injects it here. A nil/typed-nil body yields {"id": id}.
func injectID(body any, id string) (json.RawMessage, error) {
	m := map[string]json.RawMessage{}
	if body != nil && !isNilAny(body) {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("factorial: marshaling update body: %w", err)
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("factorial: decoding update body: %w", err)
		}
	}
	idRaw, err := json.Marshal(id)
	if err != nil {
		return nil, fmt.Errorf("factorial: encoding id: %w", err)
	}
	m["id"] = idRaw
	out, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("factorial: encoding update body: %w", err)
	}
	return out, nil
}
