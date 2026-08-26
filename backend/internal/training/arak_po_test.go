package training

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func TestResolvePurchaseOrderByID(t *testing.T) {
	upstream := &poResolverUpstream{detail: poJSON(42, "PO-42/2026", "APPROVED")}
	h := newPOResolverHandler(upstream)

	po, err := h.resolvePurchaseOrder(context.Background(), Principal{Email: "people@example.com"}, " 42 ")
	if err != nil {
		t.Fatalf("resolve PO: %v", err)
	}
	if po.ID != 42 || po.Code != "PO-42/2026" || po.EconomicState != EconomicStateApproved || po.TotalPrice != "1234.50" || po.Currency != "EUR" {
		t.Fatalf("unexpected PO: %#v", po)
	}
	if po.Budget.ID != 7 || po.Budget.Name != "Formazione" || po.Budget.Year != 2026 || po.Budget.CostCenter == nil || *po.Budget.CostCenter != "HR" {
		t.Fatalf("unexpected budget: %#v", po.Budget)
	}
	requests := upstream.capturedRequests()
	if len(requests) != 1 || requests[0].path != "/arak/rda/v1/po/42" {
		t.Fatalf("unexpected requests: %#v", requests)
	}
	if got := requests[0].header.Get("Requester-Email"); got != "people@example.com" {
		t.Fatalf("Requester-Email = %q", got)
	}
}

func TestResolvePurchaseOrderByCodeUsesExactMatch(t *testing.T) {
	upstream := &poResolverUpstream{items: []json.RawMessage{
		json.RawMessage(poJSON(1, "PO-50412/2026-OLD", "PENDING_APPROVAL")),
		json.RawMessage(poJSON(42, "PO-50412/2026", "SENT")),
	}}
	h := newPOResolverHandler(upstream)

	po, err := h.resolvePurchaseOrder(context.Background(), Principal{Email: "people@example.com"}, " PO-50412/2026 ")
	if err != nil {
		t.Fatalf("resolve PO: %v", err)
	}
	if po.ID != 42 || po.RawState != "SENT" {
		t.Fatalf("unexpected PO: %#v", po)
	}
	requests := upstream.capturedRequests()
	if len(requests) != 1 || requests[0].path != "/arak/rda/v1/po" {
		t.Fatalf("unexpected requests: %#v", requests)
	}
	query, err := url.ParseQuery(requests[0].query)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if query.Get("search_string") != "PO-50412/2026" || query.Get("disable_pagination") != "true" {
		t.Fatalf("unexpected query: %s", requests[0].query)
	}
	if got := requests[0].header.Get("Requester-Email"); got != "people@example.com" {
		t.Fatalf("Requester-Email = %q", got)
	}
}

func TestResolvePurchaseOrderByCodeNotFound(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		h := newPOResolverHandler(&poResolverUpstream{})

		_, err := h.resolvePurchaseOrder(context.Background(), Principal{Email: "people@example.com"}, "PO-MISSING")
		assertPOResolverAppError(t, err, http.StatusNotFound, "po_not_found")
	})
	t.Run("upstream 404", func(t *testing.T) {
		h := newPOResolverHandler(&poResolverUpstream{listStatus: http.StatusNotFound})

		_, err := h.resolvePurchaseOrder(context.Background(), Principal{Email: "people@example.com"}, "PO-MISSING")
		assertPOResolverAppError(t, err, http.StatusNotFound, "po_not_found")
	})
}

func TestResolvePurchaseOrderByCodeAmbiguous(t *testing.T) {
	h := newPOResolverHandler(&poResolverUpstream{items: []json.RawMessage{
		json.RawMessage(poJSON(41, "PO-50412/2026", "APPROVED")),
		json.RawMessage(poJSON(42, "PO-50412/2026", "APPROVED")),
	}})

	_, err := h.resolvePurchaseOrder(context.Background(), Principal{Email: "people@example.com"}, "PO-50412/2026")
	assertPOResolverAppError(t, err, http.StatusConflict, "po_reference_ambiguous")
}

func TestClassifyEconomicState(t *testing.T) {
	for _, raw := range []string{"APPROVED", "PENDING_SEND", "SENT", "PENDING_VERIFICATION", "PENDING_DISPUTE", "DELIVERED_AND_COMPLIANT", "CLOSED"} {
		if got := classifyEconomicState(raw); got != EconomicStateApproved {
			t.Errorf("classify %q = %q, want approved", raw, got)
		}
	}
	if got := classifyEconomicState("REJECTED"); got != EconomicStateRejected {
		t.Errorf("classify REJECTED = %q, want rejected", got)
	}
	if got := classifyEconomicState("PENDING_APPROVAL"); got != EconomicStatePending {
		t.Errorf("classify known pending = %q, want pending", got)
	}
	if got := classifyEconomicState("FUTURE_STATE"); got != EconomicStatePending {
		t.Errorf("classify unknown state = %q, want pending", got)
	}
}

func assertPOResolverAppError(t *testing.T, err error, status int, code string) {
	t.Helper()
	appErr, ok := asAppError(err)
	if !ok {
		t.Fatalf("expected appError, got %T %v", err, err)
	}
	if appErr.status != status || appErr.code != code {
		t.Fatalf("appError = %#v, want status=%d code=%q", appErr, status, code)
	}
}

func newPOResolverHandler(upstream *poResolverUpstream) *handler {
	return &handler{arak: arak.New(arak.Config{
		BaseURL:      "http://arak.local",
		TokenURL:     "http://arak.local/token",
		ClientID:     "client",
		ClientSecret: "secret",
		HTTPClient:   httputil.NewMockClient(upstream),
	})}
}

type poResolverRequest struct {
	path   string
	query  string
	header http.Header
}

type poResolverUpstream struct {
	mu         sync.Mutex
	detail     string
	details    map[int64]string
	items      []json.RawMessage
	listStatus int
	requests   []poResolverRequest
}

func (s *poResolverUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		_, _ = w.Write([]byte(`{"access_token":"arak-token","expires_in":3600}`))
		return
	}
	if r.Body != nil {
		_, _ = io.Copy(io.Discard, r.Body)
	}
	s.mu.Lock()
	s.requests = append(s.requests, poResolverRequest{path: r.URL.Path, query: r.URL.RawQuery, header: r.Header.Clone()})
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/arak/rda/v1/po":
		if s.listStatus != 0 {
			w.WriteHeader(s.listStatus)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": s.items})
	default:
		detail := s.detail
		if s.details != nil {
			id, err := strconv.ParseInt(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:], 10, 64)
			if err == nil {
				detail = s.details[id]
			}
		}
		if detail == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(detail))
	}
}

func (s *poResolverUpstream) capturedRequests() []poResolverRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]poResolverRequest(nil), s.requests...)
}

func poJSON(id int64, code, state string) string {
	return `{"id":` + jsonNumber(id) + `,"code":"` + code + `","state":"` + state + `","total_price":"1234.50","currency":"EUR","budget":{"id":7,"name":"Formazione","year":2026,"cost_center":"HR"}}`
}

func jsonNumber(value int64) string {
	return strconv.FormatInt(value, 10)
}
