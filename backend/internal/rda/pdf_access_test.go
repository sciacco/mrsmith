package rda

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanDownloadPOPDFStates(t *testing.T) {
	allowed := []string{
		"APPROVED",
		"PENDING_SEND",
		"SENT",
		"PENDING_VERIFICATION",
		"PENDING_DISPUTE",
		"DELIVERED_AND_COMPLIANT",
		"CLOSED",
	}
	for _, state := range allowed {
		t.Run(state, func(t *testing.T) {
			if !canDownloadPOPDF(state) {
				t.Fatalf("expected %s to allow PO PDF download", state)
			}
		})
	}

	blocked := []string{
		"",
		"DRAFT",
		"PENDING_APPROVAL",
		"PENDING_APPROVAL_PAYMENT_METHOD",
		"PENDING_LEASING",
		"PENDING_LEASING_ORDER_CREATION",
		"PENDING_APPROVAL_NO_LEASING",
		"PENDING_BUDGET_INCREMENT",
		"PENDING_PDF_GENERATION",
		"PENDING_ERP_SAVE",
	}
	for _, state := range blocked {
		t.Run(state, func(t *testing.T) {
			if canDownloadPOPDF(state) {
				t.Fatalf("expected %s to block PO PDF download", state)
			}
		})
	}
}

func TestHandlePDFAllowsOnlyConfiguredStates(t *testing.T) {
	allowed := []string{
		"APPROVED",
		"PENDING_SEND",
		"SENT",
		"PENDING_VERIFICATION",
		"PENDING_DISPUTE",
		"DELIVERED_AND_COMPLIANT",
		"CLOSED",
	}
	for _, state := range allowed {
		t.Run(state, func(t *testing.T) {
			h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
				poDetail: poActionModelDetailJSON(state, "requester@example.com", "approver@example.com"),
			})

			rec := httptest.NewRecorder()
			req := authedRDARequest(http.MethodGet, "/rda/v1/pos/42/pdf", nil)
			req.SetPathValue("id", "42")
			h.handlePDF(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			if arakState.count(http.MethodGet, "/arak/rda/v1/po/42/download") != 1 {
				t.Fatalf("expected PDF download to be forwarded for %s", state)
			}
		})
	}

	blocked := []string{
		"DRAFT",
		"PENDING_APPROVAL",
		"PENDING_APPROVAL_PAYMENT_METHOD",
		"PENDING_LEASING_ORDER_CREATION",
		"PENDING_PDF_GENERATION",
		"PENDING_ERP_SAVE",
	}
	for _, state := range blocked {
		t.Run(state, func(t *testing.T) {
			h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
				poDetail: poActionModelDetailJSON(state, "requester@example.com", "approver@example.com"),
			})

			rec := httptest.NewRecorder()
			req := authedRDARequest(http.MethodGet, "/rda/v1/pos/42/pdf", nil)
			req.SetPathValue("id", "42")
			h.handlePDF(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
			}
			if arakState.count(http.MethodGet, "/arak/rda/v1/po/42/download") != 0 {
				t.Fatalf("expected PDF download not to be forwarded for %s", state)
			}
		})
	}
}
