package simulatorivendita

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
)

type stubRenderer struct {
	payload any
	pdf     []byte
	err     error
}

func (s *stubRenderer) GeneratePDF(_ context.Context, payload any) ([]byte, error) {
	s.payload = payload
	if s.err != nil {
		return nil, s.err
	}
	return s.pdf, nil
}

func TestRegisterRoutesEnforcesACLAndNilRendererFallback(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, nil)

	t.Run("missing claims", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/simulatori-vendita/v1/iaas/quote", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("missing role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/simulatori-vendita/v1/iaas/quote", strings.NewReader(`{}`))
		req = withClaims(req, []string{"viewer"})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("valid role without renderer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/simulatori-vendita/v1/iaas/quote", strings.NewReader(`{}`))
		req = withClaims(req, []string{"app_simulatorivendita_access"})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "simulatori_vendita_pdf_not_configured") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
}

func TestHandleGenerateQuoteRejectsInvalidPayload(t *testing.T) {
	renderer := &stubRenderer{pdf: []byte("pdf")}
	mux := http.NewServeMux()
	RegisterRoutes(mux, renderer)

	req := httptest.NewRequest(
		http.MethodPost,
		"/simulatori-vendita/v1/iaas/quote",
		strings.NewReader(`{"qta":{"vcpu":-1},"prezzi":{},"totale_giornaliero":{}}`),
	)
	req = withClaims(req, []string{"app_simulatorivendita_access"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid_quote_payload") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleGenerateQuoteProxiesPayloadToRenderer(t *testing.T) {
	renderer := &stubRenderer{pdf: []byte("%PDF")}
	mux := http.NewServeMux()
	RegisterRoutes(mux, renderer)

	req := httptest.NewRequest(
		http.MethodPost,
		"/simulatori-vendita/v1/iaas/quote",
		strings.NewReader(validQuoteRequestJSON()),
	)
	req = withClaims(req, []string{"app_simulatorivendita_access"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("expected application/pdf, got %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="calcolatore-iaas.pdf"` {
		t.Fatalf("unexpected content disposition: %q", got)
	}
	if body := rec.Body.String(); body != "%PDF" {
		t.Fatalf("unexpected pdf body: %q", body)
	}

	captured, ok := renderer.payload.(renderPayload)
	if !ok {
		t.Fatalf("expected renderPayload, got %#v", renderer.payload)
	}
	if captured.ConvertTo != "pdf" {
		t.Fatalf("expected convertTo pdf, got %q", captured.ConvertTo)
	}
	if captured.Data.Quantities.VCPU != 1 || captured.Data.Quantities.StoragePri != 100 {
		t.Fatalf("unexpected qta payload: %#v", captured.Data.Quantities)
	}
	if captured.Data.Prices.RAMVMware != 0.3 || captured.Data.Prices.MSSQLStd != 6.33 {
		t.Fatalf("unexpected prezzi payload: %#v", captured.Data.Prices)
	}
	if captured.Data.DailyTotal.Totale != 11.43 || captured.Data.DailyTotal.AddOn != 6.33 {
		t.Fatalf("unexpected totals payload: %#v", captured.Data.DailyTotal)
	}
}

func TestHandleGenerateQuoteReturnsInternalErrorOnRendererFailure(t *testing.T) {
	renderer := &stubRenderer{err: errors.New("upstream failed")}
	mux := http.NewServeMux()
	RegisterRoutes(mux, renderer)

	req := httptest.NewRequest(
		http.MethodPost,
		"/simulatori-vendita/v1/iaas/quote",
		strings.NewReader(validQuoteRequestJSON()),
	)
	req = withClaims(req, []string{"app_simulatorivendita_access"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body["error"] != "internal_server_error" {
		t.Fatalf("expected internal_server_error, got %#v", body["error"])
	}
}

func withClaims(req *http.Request, roles []string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
		Name:  "Simulatori User",
		Email: "simulatori@example.com",
		Roles: roles,
	}))
}

func validQuoteRequestJSON() string {
	return `{
		"qta": {
			"vcpu": 1,
			"ram_vmware": 10,
			"ram_os": 0,
			"storage_pri": 100,
			"storage_sec": 100,
			"fw_std": 0,
			"fw_adv": 1,
			"priv_net": 0,
			"os_windows": 0,
			"ms_sql_std": 1
		},
		"prezzi": {
			"vcpu": 0.1,
			"ram_vmware": 0.3,
			"ram_os": 0.1,
			"storage_pri": 0.001,
			"storage_sec": 0.001,
			"fw_std": 0,
			"fw_adv": 1.8,
			"priv_net": 0,
			"os_windows": 1,
			"ms_sql_std": 6.33
		},
		"totale_giornaliero": {
			"computing": 3.1,
			"storage": 0.2,
			"sicurezza": 1.8,
			"addon": 6.33,
			"totale": 11.43
		}
	}`
}
