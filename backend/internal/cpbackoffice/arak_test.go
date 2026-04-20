package cpbackoffice

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/arak"
)

// capturedRequest is the minimum set of outgoing-request fields the tests
// need to assert on. We record them via a test-controlled upstream fake.
type capturedRequest struct {
	Method string
	Path   string
	Query  string
	Body   string
}

// fakeUpstream is an httptest.Server whose /token endpoint emits a stub
// access token and whose business endpoints return configurable
// status+body. Every non-token request is recorded so tests can assert on
// path, query string, and body verbatim.
type fakeUpstream struct {
	server       *httptest.Server
	requests     []capturedRequest
	hits         atomic.Int32
	responseCode int
	responseBody string
}

// newFakeUpstream wires an httptest.Server with JSON responses keyed by
// path prefix. Tests reconfigure responseCode/responseBody as needed.
func newFakeUpstream(t *testing.T) *fakeUpstream {
	t.Helper()

	fu := &fakeUpstream{
		responseCode: http.StatusOK,
		responseBody: `{"total_number":0,"current_page":1,"total_pages":1,"items":[]}`,
	}
	fu.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":300}`))
			return
		}

		bodyBytes, _ := io.ReadAll(r.Body)
		fu.requests = append(fu.requests, capturedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
			Body:   string(bodyBytes),
		})
		fu.hits.Add(1)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(fu.responseCode)
		_, _ = w.Write([]byte(fu.responseBody))
	}))
	t.Cleanup(fu.server.Close)
	return fu
}

func (fu *fakeUpstream) client() *arak.Client {
	return arak.New(arak.Config{
		BaseURL:      fu.server.URL,
		TokenURL:     fu.server.URL + "/token",
		ClientID:     "cp-backoffice-test",
		ClientSecret: "cp-backoffice-secret",
	})
}

// lastRequest returns the most recent non-token request the upstream saw.
// Tests should assert fu.hits.Load() > 0 first or call mustLastRequest.
func (fu *fakeUpstream) lastRequest(t *testing.T) capturedRequest {
	t.Helper()
	if len(fu.requests) == 0 {
		t.Fatalf("no requests captured")
	}
	return fu.requests[len(fu.requests)-1]
}

// serveWithRole wires the S3 routes on a fresh mux and plays the request
// through auth gating using an operator role claim.
func serveWithRole(t *testing.T, deps Deps, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)

	req := httptest.NewRequest(method, path, body)
	req = withRoleClaims(req, "app_cpbackoffice_access")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// ═══ Customers list ═══

func TestListCustomersPassesThroughItems(t *testing.T) {
	fu := newFakeUpstream(t)
	fu.responseBody = `{"total_number":2,"current_page":1,"total_pages":1,"items":[{"id":1,"name":"Alpha"},{"id":2,"name":"Beta"}]}`

	rec := serveWithRole(t, Deps{Arak: fu.client()}, http.MethodGet, "/cp-backoffice/v1/customers", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if fu.hits.Load() != 1 {
		t.Fatalf("expected 1 upstream hit, got %d", fu.hits.Load())
	}
	got := fu.lastRequest(t)
	if got.Method != http.MethodGet {
		t.Errorf("expected GET, got %s", got.Method)
	}
	if got.Path != "/customers/v2/customer" {
		t.Errorf("expected path /customers/v2/customer, got %s", got.Path)
	}
	if !strings.Contains(got.Query, "page_number=1") {
		t.Errorf("expected query to contain page_number=1, got %q", got.Query)
	}
	if !strings.Contains(got.Query, "disable_pagination=true") {
		t.Errorf("expected query to contain disable_pagination=true, got %q", got.Query)
	}

	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("response is not a JSON array: %v (%s)", err, rec.Body.String())
	}
	if len(items) != 2 || items[0]["name"] != "Alpha" {
		t.Fatalf("unexpected items payload: %s", rec.Body.String())
	}
}

// ═══ Customer states list ═══

func TestListCustomerStatesPassesThroughItems(t *testing.T) {
	fu := newFakeUpstream(t)
	fu.responseBody = `{"total_number":1,"current_page":1,"total_pages":1,"items":[{"id":3,"name":"Attivo","enabled":true}]}`

	rec := serveWithRole(t, Deps{Arak: fu.client()}, http.MethodGet, "/cp-backoffice/v1/customer-states", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got := fu.lastRequest(t)
	if got.Method != http.MethodGet {
		t.Errorf("expected GET, got %s", got.Method)
	}
	if got.Path != "/customers/v2/customer-state" {
		t.Errorf("expected path /customers/v2/customer-state, got %s", got.Path)
	}
	if !strings.Contains(got.Query, "page_number=1") {
		t.Errorf("expected query to contain page_number=1, got %q", got.Query)
	}
	if !strings.Contains(got.Query, "disable_pagination=true") {
		t.Errorf("expected query to contain disable_pagination=true, got %q", got.Query)
	}

	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("response is not a JSON array: %v (%s)", err, rec.Body.String())
	}
	if len(items) != 1 || items[0]["name"] != "Attivo" {
		t.Fatalf("unexpected items payload: %s", rec.Body.String())
	}
}

// ═══ Update customer state ═══

func TestUpdateCustomerStateProxiesWithStateID(t *testing.T) {
	fu := newFakeUpstream(t)
	fu.responseCode = http.StatusOK
	fu.responseBody = `{"message":"ok"}`

	rec := serveWithRole(t, Deps{Arak: fu.client()},
		http.MethodPut, "/cp-backoffice/v1/customers/42/state",
		strings.NewReader(`{"state_id":7}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got := fu.lastRequest(t)
	if got.Method != http.MethodPut {
		t.Errorf("expected PUT, got %s", got.Method)
	}
	if got.Path != "/customers/v2/customer/42" {
		t.Errorf("expected path /customers/v2/customer/42, got %s", got.Path)
	}
	if got.Query != "" {
		t.Errorf("expected empty query, got %q", got.Query)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(got.Body), &sent); err != nil {
		t.Fatalf("outgoing body is not JSON: %v (%q)", err, got.Body)
	}
	if v, ok := sent["state_id"].(float64); !ok || int64(v) != 7 {
		t.Errorf("expected state_id=7, got %#v", sent["state_id"])
	}
}

func TestUpdateCustomerStateRejectsMissingStateID(t *testing.T) {
	fu := newFakeUpstream(t)

	rec := serveWithRole(t, Deps{Arak: fu.client()},
		http.MethodPut, "/cp-backoffice/v1/customers/42/state",
		strings.NewReader(`{}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if fu.hits.Load() != 0 {
		t.Fatalf("expected zero upstream hits, got %d", fu.hits.Load())
	}
}

// ═══ Users guard (hard backend enforcement) ═══

func TestListUsersRejectsMissingCustomerID(t *testing.T) {
	fu := newFakeUpstream(t)

	rec := serveWithRole(t, Deps{Arak: fu.client()}, http.MethodGet, "/cp-backoffice/v1/users", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if fu.hits.Load() != 0 {
		t.Fatalf("expected zero upstream hits when customer_id missing, got %d", fu.hits.Load())
	}
	if !strings.Contains(rec.Body.String(), "customer_id_required") {
		t.Fatalf("expected customer_id_required error, got %q", rec.Body.String())
	}
}

func TestListUsersRejectsEmptyCustomerID(t *testing.T) {
	fu := newFakeUpstream(t)

	rec := serveWithRole(t, Deps{Arak: fu.client()}, http.MethodGet, "/cp-backoffice/v1/users?customer_id=", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if fu.hits.Load() != 0 {
		t.Fatalf("expected zero upstream hits when customer_id empty, got %d", fu.hits.Load())
	}
}

func TestListUsersWithValidCustomerIDForwardsBoth(t *testing.T) {
	fu := newFakeUpstream(t)
	fu.responseBody = `{"total_number":1,"current_page":1,"total_pages":1,"items":[{"id":9,"first_name":"Op"}]}`

	rec := serveWithRole(t, Deps{Arak: fu.client()}, http.MethodGet, "/cp-backoffice/v1/users?customer_id=77", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got := fu.lastRequest(t)
	if got.Path != "/users/v2/user" {
		t.Errorf("expected path /users/v2/user, got %s", got.Path)
	}
	if !strings.Contains(got.Query, "customer_id=77") {
		t.Errorf("expected query to contain customer_id=77, got %q", got.Query)
	}
	if !strings.Contains(got.Query, "page_number=1") {
		t.Errorf("expected query to contain page_number=1, got %q", got.Query)
	}
	if !strings.Contains(got.Query, "disable_pagination=true") {
		t.Errorf("expected query to contain disable_pagination=true, got %q", got.Query)
	}
}

// ═══ Admin creation pins skip_keycloak: false ═══

func TestCreateAdminPinsSkipKeycloakFalseEvenWhenClientSendsTrue(t *testing.T) {
	fu := newFakeUpstream(t)
	fu.responseBody = `{"id":99}`

	// Client explicitly tries to set skip_keycloak: true. The handler MUST
	// rewrite it to false on the outgoing request.
	rec := serveWithRole(t, Deps{Arak: fu.client()},
		http.MethodPost, "/cp-backoffice/v1/admins",
		strings.NewReader(`{
		    "customer_id": 12,
		    "nome": "Jane",
		    "cognome": "Doe",
		    "email": "jane@example.com",
		    "telefono": "0039",
		    "maintenance_on_primary_email": true,
		    "marketing_on_primary_email": false,
		    "skip_keycloak": true
		}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got := fu.lastRequest(t)
	if got.Method != http.MethodPost {
		t.Errorf("expected POST, got %s", got.Method)
	}
	if got.Path != "/users/v2/admin" {
		t.Errorf("expected path /users/v2/admin, got %s", got.Path)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(got.Body), &sent); err != nil {
		t.Fatalf("outgoing body is not JSON: %v (%q)", err, got.Body)
	}

	// skip_keycloak MUST be present and MUST be false. The handler pins it
	// and never forwards the client's value.
	v, ok := sent["skip_keycloak"]
	if !ok {
		t.Fatalf("outgoing body missing skip_keycloak, got %s", got.Body)
	}
	if b, ok := v.(bool); !ok || b != false {
		t.Fatalf("expected skip_keycloak=false, got %#v", v)
	}

	// Upstream DTO key translation: nome→first_name, cognome→last_name,
	// telefono→phone.
	if sent["first_name"] != "Jane" {
		t.Errorf("expected first_name=Jane, got %#v", sent["first_name"])
	}
	if sent["last_name"] != "Doe" {
		t.Errorf("expected last_name=Doe, got %#v", sent["last_name"])
	}
	if sent["phone"] != "0039" {
		t.Errorf("expected phone=0039, got %#v", sent["phone"])
	}
	if sent["email"] != "jane@example.com" {
		t.Errorf("expected email=jane@example.com, got %#v", sent["email"])
	}
	if v, ok := sent["customer_id"].(float64); !ok || int64(v) != 12 {
		t.Errorf("expected customer_id=12, got %#v", sent["customer_id"])
	}
	if b, ok := sent["maintenance_on_primary_email"].(bool); !ok || b != true {
		t.Errorf("expected maintenance_on_primary_email=true, got %#v", sent["maintenance_on_primary_email"])
	}
	if b, ok := sent["marketing_on_primary_email"].(bool); !ok || b != false {
		t.Errorf("expected marketing_on_primary_email=false, got %#v", sent["marketing_on_primary_email"])
	}
}

// ═══ Upstream business error pass-through ═══

func TestUpdateCustomerStateForwardsUpstreamBusinessMessage(t *testing.T) {
	fu := newFakeUpstream(t)
	fu.responseCode = http.StatusBadRequest
	fu.responseBody = `{"message":"azienda non trovata"}`

	rec := serveWithRole(t, Deps{Arak: fu.client()},
		http.MethodPut, "/cp-backoffice/v1/customers/42/state",
		strings.NewReader(`{"state_id":1}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 forwarded, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, rec.Body.String())
	}
	if body["message"] != "azienda non trovata" {
		t.Errorf("expected message passed through, got %#v", body)
	}
	if body["error"] == "" {
		t.Errorf("expected non-empty error code, got %#v", body)
	}
}

func TestCreateAdminForwardsUpstreamBusinessMessage(t *testing.T) {
	fu := newFakeUpstream(t)
	fu.responseCode = http.StatusConflict
	fu.responseBody = `{"message":"email gia' registrata"}`

	rec := serveWithRole(t, Deps{Arak: fu.client()},
		http.MethodPost, "/cp-backoffice/v1/admins",
		strings.NewReader(`{"customer_id":12,"nome":"X","cognome":"Y","email":"x@y.z","telefono":"","maintenance_on_primary_email":false,"marketing_on_primary_email":false}`))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 forwarded, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, rec.Body.String())
	}
	if body["message"] != "email gia' registrata" {
		t.Errorf("expected message passed through, got %#v", body)
	}
}
