package kitproducts

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubArakClient struct {
	method string
	path   string
	query  string
	body   string
	resp   *http.Response
	err    error
}

func (s *stubArakClient) Do(method, path, queryString string, body io.Reader) (*http.Response, error) {
	s.method = method
	s.path = path
	s.query = queryString
	if body != nil {
		raw, _ := io.ReadAll(body)
		s.body = string(raw)
	}
	return s.resp, s.err
}

func TestHandleProxyMistraKitRequiresArak(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/kit-products/v1/mistra/kit?page_number=1", nil)
	rec := httptest.NewRecorder()

	h.handleProxyMistraKit(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleProxyMistraKitDiscountPassesThrough(t *testing.T) {
	upstream := httptest.NewRecorder()
	upstream.Header().Set("Content-Type", "application/json")
	upstream.WriteHeader(http.StatusOK)
	upstream.WriteString(`{"message":"ok"}`)

	h := &Handler{
		arak: &stubArakClient{
			resp: upstream.Result(),
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/kit-products/v1/mistra/kit-discount?kit_id=9", strings.NewReader(`{"kit_id":9}`))
	rec := httptest.NewRecorder()

	h.handleProxyMistraKitDiscount(rec, req)

	client := h.arak.(*stubArakClient)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if client.method != http.MethodPost || client.path != "/products/v2/kit-discount" || client.query != "kit_id=9" {
		t.Fatalf("unexpected upstream call: method=%s path=%s query=%s", client.method, client.path, client.query)
	}
	if client.body != `{"kit_id":9}` {
		t.Fatalf("unexpected upstream body: %q", client.body)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"message":"ok"}` {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleProxyMistraKitReturnsBadGatewayOnTransportFailure(t *testing.T) {
	h := &Handler{
		arak: &stubArakClient{
			err: errors.New("dial tcp timeout"),
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/kit-products/v1/mistra/kit?page_number=1", nil)
	rec := httptest.NewRecorder()

	h.handleProxyMistraKit(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["code"] != upstreamUnavailableCode {
		t.Fatalf("unexpected response body: %#v", body)
	}
}

func TestHandleProxyMistraKitReturnsBadGatewayOnUpstreamAuthFailure(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := httptest.NewRecorder()
			upstream.Header().Set("Content-Type", "application/json")
			upstream.WriteHeader(status)
			upstream.WriteString(`{"error":"auth failed"}`)

			h := &Handler{
				arak: &stubArakClient{
					resp: upstream.Result(),
				},
			}
			req := httptest.NewRequest(http.MethodGet, "/kit-products/v1/mistra/kit", nil)
			rec := httptest.NewRecorder()

			h.handleProxyMistraKit(rec, req)

			if rec.Code != http.StatusBadGateway {
				t.Fatalf("expected 502, got %d", rec.Code)
			}

			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if body["code"] != upstreamAuthFailedCode {
				t.Fatalf("unexpected response body: %#v", body)
			}
		})
	}
}
