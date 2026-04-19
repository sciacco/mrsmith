package afctools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

func TestHandleOrderPDFNormalizesArxivarMissingDocument(t *testing.T) {
	h := &Handler{
		deps: Deps{
			Arak: newGatewayTestArakClient(t, http.StatusInternalServerError, `{"message":"ARX_DOC_NUMBER_NOT_FOUND"}`),
		},
	}

	req := newGatewayOrderPDFRequest(t, "301")
	rec := httptest.NewRecorder()

	h.handleOrderPDF(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != orderPDFNotReadyErrorCode {
		t.Fatalf("expected %q, got %#v", orderPDFNotReadyErrorCode, body["error"])
	}
	if body["message"] != orderPDFNotReadyErrorMessage {
		t.Fatalf("expected %q, got %#v", orderPDFNotReadyErrorMessage, body["message"])
	}
}

func TestHandleOrderPDFMapsUpstream404ToNotReadyPayload(t *testing.T) {
	h := &Handler{
		deps: Deps{
			Arak: newGatewayTestArakClient(t, http.StatusNotFound, `{"message":"missing"}`),
		},
	}

	req := newGatewayOrderPDFRequest(t, "301")
	rec := httptest.NewRecorder()

	h.handleOrderPDF(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != orderPDFNotReadyErrorCode {
		t.Fatalf("expected %q, got %#v", orderPDFNotReadyErrorCode, body["error"])
	}
	if body["message"] != orderPDFNotReadyErrorMessage {
		t.Fatalf("expected %q, got %#v", orderPDFNotReadyErrorMessage, body["message"])
	}
}

func TestHandleOrderPDFPreservesGenericUpstreamFailures(t *testing.T) {
	h := &Handler{
		deps: Deps{
			Arak: newGatewayTestArakClient(t, http.StatusInternalServerError, `{"message":"UNEXPECTED_FAILURE"}`),
		},
	}

	req := newGatewayOrderPDFRequest(t, "301")
	rec := httptest.NewRecorder()

	h.handleOrderPDF(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["error"] != "gateway_error" {
		t.Fatalf("expected gateway_error, got %#v", body["error"])
	}
}

func newGatewayOrderPDFRequest(t *testing.T, orderID string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/afc-tools/v1/orders/"+orderID+"/pdf", nil)
	req.SetPathValue("orderId", orderID)
	ctx := logging.IntoContext(req.Context(), logging.NewWithWriter(io.Discard, "info"))
	return req.WithContext(ctx)
}

func newGatewayTestArakClient(t *testing.T, orderStatus int, orderBody string) *arak.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":300}`))
		case strings.HasPrefix(r.URL.Path, "/orders/v1/order/pdf/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(orderStatus)
			_, _ = w.Write([]byte(orderBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	return arak.New(arak.Config{
		BaseURL:      server.URL,
		TokenURL:     server.URL + "/token",
		ClientID:     "afctools-client",
		ClientSecret: "afctools-secret",
	})
}
