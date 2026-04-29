package rda

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateRowNormalizesServicePayload(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPost, "/rda/v1/pos/42/rows", strings.NewReader(`{
		"type": "service",
		"description": "SOFTWARE USO INTERNO",
		"qty": 1,
		"product_code": "SOFTWARE",
		"product_description": "SOFTWARE USO INTERNO - SOFTWARE (UI)",
		"monthly_fee": 1,
		"activation_price": 0,
		"requester_email": "spoofed@example.com",
		"payment_detail": {
			"start_at": "activation_date",
			"month_recursion": 1
		},
		"renew_detail": {
			"initial_subscription_months": 12,
			"automatic_renew": true,
			"cancellation_advice": "1"
		}
	}`))
	req.SetPathValue("id", "42")

	h.handleCreateRow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := arakState.lastJSONBody(t, http.MethodPost, "/arak/rda/v1/po/42/row")
	if got := body["requester_email"]; got != "user@example.com" {
		t.Fatalf("expected requester email from claims, got %#v", got)
	}
	if got := body["price"]; got != "1" {
		t.Fatalf("expected monthly fee to be forwarded as price string, got %#v", got)
	}
	if got := body["activation_price"]; got != "0" {
		t.Fatalf("expected NRC to be forwarded as activation_price string, got %#v", got)
	}
	if got := body["total"]; got != "12" {
		t.Fatalf("expected row total to be forwarded, got %#v", got)
	}
	if _, ok := body["monthly_fee"]; ok {
		t.Fatalf("expected monthly_fee not to be forwarded")
	}
	if _, ok := body["montly_fee"]; ok {
		t.Fatalf("expected montly_fee not to be forwarded")
	}

	paymentDetail, ok := body["payment_detail"].(map[string]any)
	if !ok {
		t.Fatalf("expected payment_detail object, got %#v", body["payment_detail"])
	}
	if got := paymentDetail["is_recurrent"]; got != true {
		t.Fatalf("expected recurrent payment detail, got %#v", got)
	}
	if got := paymentDetail["month_recursion"]; got != float64(1) {
		t.Fatalf("expected month_recursion 1, got %#v", got)
	}

	renewDetail, ok := body["renew_detail"].(map[string]any)
	if !ok {
		t.Fatalf("expected renew_detail object, got %#v", body["renew_detail"])
	}
	if got := renewDetail["initial_subscription_months"]; got != float64(12) {
		t.Fatalf("expected initial_subscription_months 12, got %#v", got)
	}
	if got := renewDetail["cancellation_advice"]; got != float64(1) {
		t.Fatalf("expected integer cancellation_advice, got %#v", got)
	}
}

func TestCreateRowAcceptsLegacyMontlyFeePayload(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPost, "/rda/v1/pos/42/rows", strings.NewReader(`{
		"type": "service",
		"description": "SOFTWARE USO INTERNO",
		"qty": 1,
		"product_code": "SOFTWARE",
		"product_description": "SOFTWARE USO INTERNO - SOFTWARE (UI)",
		"montly_fee": 2.5,
		"activation_price": 0,
		"payment_detail": {
			"start_at": "activation_date",
			"month_recursion": 1
		},
		"renew_detail": {
			"initial_subscription_months": 12,
			"automatic_renew": false
		}
	}`))
	req.SetPathValue("id", "42")

	h.handleCreateRow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := arakState.lastJSONBody(t, http.MethodPost, "/arak/rda/v1/po/42/row")
	if got := body["price"]; got != "2.5" {
		t.Fatalf("expected legacy montly_fee to be forwarded as price string, got %#v", got)
	}
}

func TestCreateRowNormalizesGoodPayload(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPost, "/rda/v1/pos/42/rows", strings.NewReader(`{
		"type": "good",
		"description": "Monitor",
		"qty": 2,
		"product_code": "MON",
		"product_description": "Monitor",
		"price": 12.5,
		"monthly_fee": 99,
		"activation_price": 88,
		"payment_detail": {
			"start_at": "advance_payment"
		}
	}`))
	req.SetPathValue("id", "42")

	h.handleCreateRow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := arakState.lastJSONBody(t, http.MethodPost, "/arak/rda/v1/po/42/row")
	if got := body["price"]; got != "12.5" {
		t.Fatalf("expected unit price string, got %#v", got)
	}
	if got := body["total"]; got != "25" {
		t.Fatalf("expected row total to be forwarded, got %#v", got)
	}
	if _, ok := body["monthly_fee"]; ok {
		t.Fatalf("expected monthly_fee not to be forwarded for goods")
	}
	if _, ok := body["activation_price"]; ok {
		t.Fatalf("expected activation_price not to be forwarded for goods")
	}
	if renewDetail, ok := body["renew_detail"].(map[string]any); !ok || len(renewDetail) != 0 {
		t.Fatalf("expected empty renew_detail for goods to satisfy Mistra schema, got %#v", body["renew_detail"])
	}
}

func TestCreateRowRejectsServiceWithoutMRCOrNRC(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPost, "/rda/v1/pos/42/rows", strings.NewReader(`{
		"type": "service",
		"description": "SOFTWARE USO INTERNO",
		"qty": 1,
		"product_code": "SOFTWARE",
		"product_description": "SOFTWARE USO INTERNO - SOFTWARE (UI)",
		"monthly_fee": 0,
		"activation_price": 0,
		"payment_detail": {
			"start_at": "activation_date",
			"month_recursion": 1
		},
		"renew_detail": {
			"initial_subscription_months": 12,
			"automatic_renew": false
		}
	}`))
	req.SetPathValue("id", "42")

	h.handleCreateRow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if arakState.count(http.MethodPost, "/arak/rda/v1/po/42/row") != 0 {
		t.Fatalf("expected invalid row not to be forwarded")
	}
}

func (s *paymentValidationArakState) lastJSONBody(t *testing.T, method string, path string) map[string]any {
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
		return body
	}
	t.Fatalf("missing forwarded request %s %s", method, path)
	return nil
}
