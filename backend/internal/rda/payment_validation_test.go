package rda

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/arak"
)

func TestCreatePOValidatesRDAAvailablePaymentMethod(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
		methods: map[string]paymentMethod{
			"RDA": {Code: "RDA", Description: "RDA", RDAAvailable: true},
			"SUP": {Code: "SUP", Description: "Supplier", RDAAvailable: false},
		},
		defaultCode:     "RDA",
		providerDefault: "SUP",
	})

	rec := httptest.NewRecorder()
	h.handleCreatePO(rec, authedRDARequest(http.MethodPost, "/rda/v1/pos", strings.NewReader(createPOBody("RDA"))))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := arakState.lastPaymentMethod(t, http.MethodPost, "/arak/rda/v1/po"); got != "RDA" {
		t.Fatalf("expected forwarded payment RDA, got %q", got)
	}
}

func TestCreatePOAcceptsNonRDASupplierDefault(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
		methods: map[string]paymentMethod{
			"SUP": {Code: "SUP", Description: "Supplier", RDAAvailable: false},
		},
		defaultCode:     "CDLAN",
		providerDefault: "SUP",
	})

	rec := httptest.NewRecorder()
	h.handleCreatePO(rec, authedRDARequest(http.MethodPost, "/rda/v1/pos", strings.NewReader(createPOBody("SUP"))))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := arakState.lastPaymentMethod(t, http.MethodPost, "/arak/rda/v1/po"); got != "SUP" {
		t.Fatalf("expected forwarded supplier default, got %q", got)
	}
}

func TestCreatePORejectsUnknownOrNonRDANonDefaultPaymentMethod(t *testing.T) {
	tests := []struct {
		name    string
		methods map[string]paymentMethod
		code    string
	}{
		{
			name: "unknown",
			methods: map[string]paymentMethod{
				"SUP": {Code: "SUP", Description: "Supplier", RDAAvailable: false},
			},
			code: "OLD",
		},
		{
			name: "non-rda non-default",
			methods: map[string]paymentMethod{
				"SUP": {Code: "SUP", Description: "Supplier", RDAAvailable: false},
				"OLD": {Code: "OLD", Description: "Old", RDAAvailable: false},
			},
			code: "OLD",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
				methods:         tc.methods,
				defaultCode:     "SUP",
				providerDefault: "SUP",
			})

			rec := httptest.NewRecorder()
			h.handleCreatePO(rec, authedRDARequest(http.MethodPost, "/rda/v1/pos", strings.NewReader(createPOBody(tc.code))))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
			}
			if arakState.count(http.MethodPost, "/arak/rda/v1/po") != 0 {
				t.Fatalf("expected create not to be forwarded")
			}
		})
	}
}

func TestCreatePOPaymentFallbackOrder(t *testing.T) {
	t.Run("provider default before CDLAN", func(t *testing.T) {
		h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
			methods: map[string]paymentMethod{
				"SUP":   {Code: "SUP", Description: "Supplier", RDAAvailable: false},
				"CDLAN": {Code: "CDLAN", Description: "CDLAN", RDAAvailable: true},
			},
			defaultCode:     "CDLAN",
			providerDefault: "SUP",
		})

		rec := httptest.NewRecorder()
		h.handleCreatePO(rec, authedRDARequest(http.MethodPost, "/rda/v1/pos", strings.NewReader(createPOBody(""))))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := arakState.lastPaymentMethod(t, http.MethodPost, "/arak/rda/v1/po"); got != "SUP" {
			t.Fatalf("expected provider default fallback, got %q", got)
		}
	})

	t.Run("CDLAN when provider has no default", func(t *testing.T) {
		h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
			methods: map[string]paymentMethod{
				"CDLAN": {Code: "CDLAN", Description: "CDLAN", RDAAvailable: true},
			},
			defaultCode: "CDLAN",
		})

		rec := httptest.NewRecorder()
		h.handleCreatePO(rec, authedRDARequest(http.MethodPost, "/rda/v1/pos", strings.NewReader(createPOBody(""))))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := arakState.lastPaymentMethod(t, http.MethodPost, "/arak/rda/v1/po"); got != "CDLAN" {
			t.Fatalf("expected CDLAN fallback, got %q", got)
		}
	})
}

func TestPatchPOValidatesPaymentMethod(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
		methods: map[string]paymentMethod{
			"SUP": {Code: "SUP", Description: "Supplier", RDAAvailable: false},
			"OLD": {Code: "OLD", Description: "Old", RDAAvailable: false},
		},
		defaultCode:     "SUP",
		providerDefault: "SUP",
		poDetail:        poDetailJSON("SUP", "SUP"),
	})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPatch, "/rda/v1/pos/42", strings.NewReader(`{"payment_method":"SUP"}`))
	req.SetPathValue("id", "42")
	h.handlePatchPO(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := arakState.lastPaymentMethod(t, http.MethodPatch, "/arak/rda/v1/po/42"); got != "SUP" {
		t.Fatalf("expected forwarded supplier default, got %q", got)
	}

	rec = httptest.NewRecorder()
	req = authedRDARequest(http.MethodPatch, "/rda/v1/pos/42", strings.NewReader(`{"payment_method":"OLD"}`))
	req.SetPathValue("id", "42")
	h.handlePatchPO(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if arakState.count(http.MethodPatch, "/arak/rda/v1/po/42") != 1 {
		t.Fatalf("expected rejected patch not to be forwarded again")
	}
}

func TestPatchPOProviderChangeFallsBackToNewProviderDefault(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
		methods: map[string]paymentMethod{
			"NEWSUP": {Code: "NEWSUP", Description: "New supplier", RDAAvailable: false},
		},
		defaultCode:      "CDLAN",
		providerDefault:  "SUP",
		providerDefaults: map[int64]string{9: "NEWSUP"},
		poDetail:         poDetailJSON("SUP", "SUP"),
	})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPatch, "/rda/v1/pos/42", strings.NewReader(`{"provider_id":9}`))
	req.SetPathValue("id", "42")
	h.handlePatchPO(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := arakState.lastPaymentMethod(t, http.MethodPatch, "/arak/rda/v1/po/42"); got != "NEWSUP" {
		t.Fatalf("expected new provider default fallback, got %q", got)
	}
}

func createPOBody(paymentMethod string) string {
	body := map[string]any{
		"type":           "STANDARD",
		"budget_id":      11,
		"budget_user_id": 22,
		"provider_id":    7,
		"project":        "PRJ",
		"object":         "Oggetto",
	}
	if paymentMethod != "" {
		body["payment_method"] = paymentMethod
	}
	encoded, _ := json.Marshal(body)
	return string(encoded)
}

func poDetailJSON(providerDefault string, paymentMethod string) string {
	provider := map[string]any{"id": 7}
	if providerDefault != "" {
		provider["default_payment_method"] = map[string]any{"code": providerDefault, "description": providerDefault}
	}
	body := map[string]any{
		"id":             42,
		"state":          "DRAFT",
		"requester":      map[string]any{"email": "user@example.com"},
		"provider":       provider,
		"payment_method": map[string]any{"code": paymentMethod, "description": paymentMethod},
	}
	encoded, _ := json.Marshal(body)
	return string(encoded)
}

func authedRDARequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Content-Type", "application/json")
	claims := auth.Claims{Subject: "u1", Email: "user@example.com", RawToken: "token"}
	return req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, claims))
}

type paymentValidationFixture struct {
	methods          map[string]paymentMethod
	defaultCode      string
	providerDefault  string
	providerDefaults map[int64]string
	poDetail         string
	rowCreateStatus  int
	rowCreateBody    string
	rowDeleteStatus  int
	rowDeleteBody    string
}

func newPaymentValidationHandler(t *testing.T, fixture paymentValidationFixture) (*Handler, *paymentValidationArakState) {
	t.Helper()
	state := &paymentValidationArakState{fixture: fixture}
	server := httptest.NewServer(state)
	t.Cleanup(server.Close)

	client := arak.New(arak.Config{
		BaseURL:      server.URL,
		TokenURL:     server.URL + "/token",
		ClientID:     "client",
		ClientSecret: "secret",
	})

	return &Handler{arak: client, arakDB: openPaymentValidationDB(t, fixture)}, state
}

type capturedArakRequest struct {
	method string
	path   string
	header http.Header
	body   []byte
}

type paymentValidationArakState struct {
	mu       sync.Mutex
	fixture  paymentValidationFixture
	requests []capturedArakRequest
}

func (s *paymentValidationArakState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"arak-token","expires_in":3600}`))
		return
	}

	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.requests = append(s.requests, capturedArakRequest{method: r.Method, path: r.URL.Path, header: r.Header.Clone(), body: body})
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/arak/provider-qualification/v1/provider/"):
		id := int64(7)
		if strings.HasSuffix(r.URL.Path, "/9") {
			id = 9
		}
		defaultCode := s.fixture.providerDefault
		if s.fixture.providerDefaults != nil && s.fixture.providerDefaults[id] != "" {
			defaultCode = s.fixture.providerDefaults[id]
		}
		response := map[string]any{"id": id, "language": "it"}
		if defaultCode != "" {
			response["default_payment_method"] = map[string]any{"code": defaultCode, "description": defaultCode}
		}
		_ = json.NewEncoder(w).Encode(response)
	case r.Method == http.MethodGet && r.URL.Path == "/arak/rda/v1/po/42":
		if s.fixture.poDetail != "" {
			_, _ = w.Write([]byte(s.fixture.poDetail))
			return
		}
		_, _ = w.Write([]byte(poDetailJSON(s.fixture.providerDefault, s.fixture.providerDefault)))
	case r.Method == http.MethodPost && r.URL.Path == "/arak/rda/v1/po/42/row":
		if s.fixture.rowCreateStatus != 0 {
			w.WriteHeader(s.fixture.rowCreateStatus)
		}
		if s.fixture.rowCreateBody != "" {
			_, _ = w.Write([]byte(s.fixture.rowCreateBody))
			return
		}
		_, _ = w.Write([]byte(`{"id":9001}`))
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/arak/rda/v1/po/42/row/"):
		if s.fixture.rowDeleteStatus != 0 {
			w.WriteHeader(s.fixture.rowDeleteStatus)
		}
		if s.fixture.rowDeleteBody != "" {
			_, _ = w.Write([]byte(s.fixture.rowDeleteBody))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	default:
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}

func (s *paymentValidationArakState) count(method string, path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, request := range s.requests {
		if request.method == method && request.path == path {
			count++
		}
	}
	return count
}

func (s *paymentValidationArakState) lastPaymentMethod(t *testing.T, method string, path string) string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.requests) - 1; i >= 0; i-- {
		request := s.requests[i]
		if request.method != method || request.path != path {
			continue
		}
		var body map[string]any
		if err := json.Unmarshal(request.body, &body); err != nil {
			t.Fatalf("failed to decode forwarded body: %v body=%s", err, string(request.body))
		}
		return strings.TrimSpace(stringValue(body["payment_method"]))
	}
	t.Fatalf("missing forwarded request %s %s", method, path)
	return ""
}

func openPaymentValidationDB(t *testing.T, fixture paymentValidationFixture) *sql.DB {
	t.Helper()
	registerPaymentValidationDriver()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	paymentValidationFixtures.Store(name, fixture)
	t.Cleanup(func() {
		paymentValidationFixtures.Delete(name)
	})

	db, err := sql.Open(paymentValidationDriverName, name)
	if err != nil {
		t.Fatalf("failed to open payment validation db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const paymentValidationDriverName = "rda_payment_validation_test_driver"

var (
	paymentValidationDriverOnce sync.Once
	paymentValidationFixtures   sync.Map
)

func registerPaymentValidationDriver() {
	paymentValidationDriverOnce.Do(func() {
		sql.Register(paymentValidationDriverName, paymentValidationDriver{})
	})
}

type paymentValidationDriver struct{}

func (paymentValidationDriver) Open(name string) (driver.Conn, error) {
	value, ok := paymentValidationFixtures.Load(name)
	if !ok {
		return nil, errors.New("missing payment validation fixture")
	}
	fixture, ok := value.(paymentValidationFixture)
	if !ok {
		return nil, errors.New("invalid payment validation fixture")
	}
	return paymentValidationConn{fixture: fixture}, nil
}

type paymentValidationConn struct {
	fixture paymentValidationFixture
}

func (c paymentValidationConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c paymentValidationConn) Close() error { return nil }

func (c paymentValidationConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c paymentValidationConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "payment_method_default_cdlan"):
		if c.fixture.defaultCode == "" {
			return &paymentValidationRows{columns: []string{"payment_method_code"}}, nil
		}
		return &paymentValidationRows{
			columns: []string{"payment_method_code"},
			values:  [][]driver.Value{{c.fixture.defaultCode}},
		}, nil
	case strings.Contains(query, "provider_qualifications.payment_method") && strings.Contains(query, "WHERE code = $1"):
		if len(args) == 0 {
			return nil, errors.New("missing payment method code arg")
		}
		code, _ := args[0].Value.(string)
		method, ok := c.fixture.methods[code]
		if !ok {
			return &paymentValidationRows{columns: []string{"code", "description", "rda_available"}}, nil
		}
		return &paymentValidationRows{
			columns: []string{"code", "description", "rda_available"},
			values:  [][]driver.Value{{method.Code, method.Description, method.RDAAvailable}},
		}, nil
	case strings.Contains(query, "provider_qualifications.payment_method"):
		values := make([][]driver.Value, 0, len(c.fixture.methods))
		for _, method := range c.fixture.methods {
			if method.RDAAvailable {
				values = append(values, []driver.Value{method.Code, method.Description, method.RDAAvailable})
			}
		}
		return &paymentValidationRows{columns: []string{"code", "description", "rda_available"}, values: values}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

type paymentValidationRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *paymentValidationRows) Columns() []string {
	return r.columns
}

func (r *paymentValidationRows) Close() error { return nil }

func (r *paymentValidationRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

var _ driver.QueryerContext = paymentValidationConn{}
var _ http.Handler = (*paymentValidationArakState)(nil)
