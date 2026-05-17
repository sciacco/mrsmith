package rda

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateProviderReferenceRequiresEmail(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPost, "/rda/v1/providers/7/references", strings.NewReader(`{
		"reference_type": "ADMINISTRATIVE_REF",
		"phone": "+391234567890"
	}`))
	req.SetPathValue("id", "7")
	h.handleCreateProviderReference(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := arakState.count(http.MethodPost, "/arak/provider-qualification/v1/provider/7/reference"); got != 0 {
		t.Fatalf("expected invalid reference not to be forwarded, got %d forwards", got)
	}
}

func TestProviderReferenceRejectsInvalidPhone(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		refID  string
		body   string
		path   string
	}{
		{
			name:   "create",
			method: http.MethodPost,
			target: "/rda/v1/providers/7/references",
			body:   `{"reference_type":"ADMINISTRATIVE_REF","email":"admin@example.com","phone":"+39 1234567890"}`,
			path:   "/arak/provider-qualification/v1/provider/7/reference",
		},
		{
			name:   "update",
			method: http.MethodPut,
			target: "/rda/v1/providers/7/references/9",
			refID:  "9",
			body:   `{"email":"admin@example.com","phone":"123456"}`,
			path:   "/arak/provider-qualification/v1/provider/7/reference/9",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

			rec := httptest.NewRecorder()
			req := authedRDARequest(tc.method, tc.target, strings.NewReader(tc.body))
			req.SetPathValue("id", "7")
			if tc.refID != "" {
				req.SetPathValue("refId", tc.refID)
			}
			if tc.method == http.MethodPost {
				h.handleCreateProviderReference(rec, req)
			} else {
				h.handleUpdateProviderReference(rec, req)
			}

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
			}
			if got := arakState.count(tc.method, tc.path); got != 0 {
				t.Fatalf("expected invalid reference not to be forwarded, got %d forwards", got)
			}
		})
	}
}

func TestProviderReferenceAcceptsArakPhoneFormat(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPost, "/rda/v1/providers/7/references", strings.NewReader(`{
		"reference_type": "TECHNICAL_REF",
		"email": "tech@example.com",
		"phone": "+391234567890"
	}`))
	req.SetPathValue("id", "7")
	h.handleCreateProviderReference(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := arakState.count(http.MethodPost, "/arak/provider-qualification/v1/provider/7/reference"); got != 1 {
		t.Fatalf("expected valid reference to be forwarded once, got %d", got)
	}
}
