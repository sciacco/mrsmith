package rda

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeletePOAllowsRequesterBeforeProviderSend(t *testing.T) {
	t.Run("DRAFT uses upstream delete", func(t *testing.T) {
		h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
			poDetail: poDetailWithRowsJSON("DRAFT", "user@example.com"),
		})

		rec := httptest.NewRecorder()
		req := authedRDARequest(http.MethodDelete, "/rda/v1/pos/42", nil)
		req.SetPathValue("id", "42")
		h.handleDeletePO(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := arakState.count(http.MethodDelete, "/arak/rda/v1/po/42"); got != 1 {
			t.Fatalf("expected draft delete to be forwarded once, got %d", got)
		}
	})

	states := []string{
		"PENDING_APPROVAL_PROVIDER",
		"PENDING_APPROVAL",
		"PENDING_APPROVAL_PAYMENT_METHOD",
		"PENDING_BUDGET_INCREMENT",
	}

	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
				poDetail: poDetailWithRowsJSON(state, "user@example.com"),
			})

			rec := httptest.NewRecorder()
			req := authedRDARequest(http.MethodDelete, "/rda/v1/pos/42", nil)
			req.SetPathValue("id", "42")
			h.handleDeletePO(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			if got := arakState.count(http.MethodDelete, "/arak/rda/v1/po/42"); got != 0 {
				t.Fatalf("expected non-draft delete not to be forwarded, got %d", got)
			}
		})
	}
}

func TestDeletePORejectsRequesterAfterProviderSendAndUnconfirmedStates(t *testing.T) {
	states := []string{
		"PENDING_VERIFICATION",
		"PENDING_DISPUTE",
		"CLOSED",
		"DELIVERED_AND_COMPLIANT",
		"REJECTED",
		"CANCELED",
		"PENDING_LEASING",
		"PENDING_APPROVAL_NO_LEASING",
		"PENDING_LEASING_ORDER_CREATION",
		"PENDING_SEND",
		"SUBMITTED",
		"PENDING_CHECK_DOCUMENT",
		"PENDING_BUDGET_INCREMENT_CHECK",
		"PENDING_BUDGET_SUBTRACTION",
		"PENDING_PROVIDER_SAVED_IN_ALYANTE",
		"PENDING_PDF_GENERATION",
		"PENDING_ERP_SAVE",
		"PENDING_CONTRACT_VERIFICATION",
	}

	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
				poDetail: poDetailWithRowsJSON(state, "user@example.com"),
			})

			rec := httptest.NewRecorder()
			req := authedRDARequest(http.MethodDelete, "/rda/v1/pos/42", nil)
			req.SetPathValue("id", "42")
			h.handleDeletePO(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
			}
			if got := arakState.count(http.MethodDelete, "/arak/rda/v1/po/42"); got != 0 {
				t.Fatalf("expected delete not to be forwarded, got %d", got)
			}
		})
	}
}

func TestDeletePORejectsNonRequester(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
		poDetail: poDetailWithRowsJSON("PENDING_APPROVAL", "other@example.com"),
	})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodDelete, "/rda/v1/pos/42", nil)
	req.SetPathValue("id", "42")
	h.handleDeletePO(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := arakState.count(http.MethodDelete, "/arak/rda/v1/po/42"); got != 0 {
		t.Fatalf("expected delete not to be forwarded, got %d", got)
	}
}

func TestDeletePOResolvesPendingNotifications(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
		poDetail: poDetailWithRowsJSON("PENDING_APPROVAL", "user@example.com"),
	})
	notifier := &fakeRDANotifier{}
	h.notifier = notifier

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodDelete, "/rda/v1/pos/42", nil)
	req.SetPathValue("id", "42")
	h.handleDeletePO(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := arakState.count(http.MethodDelete, "/arak/rda/v1/po/42"); got != 0 {
		t.Fatalf("expected non-draft delete not to be forwarded, got %d", got)
	}
	if len(notifier.notifies) != 0 {
		t.Fatalf("delete should not create notifications: %#v", notifier.notifies)
	}
	if len(notifier.resolves) != 2 {
		t.Fatalf("expected approval and mention notifications to be resolved, got %#v", notifier.resolves)
	}

	resolved := map[string]bool{}
	for _, input := range notifier.resolves {
		if input.EntityType != rdaNotificationEntityType || input.EntityID != "42" {
			t.Fatalf("unexpected resolve target: %#v", input)
		}
		resolved[input.TypeKey] = true
	}
	if !resolved[rdaApprovalNotificationType] || !resolved[rdaCommentMentionNotificationType] {
		t.Fatalf("missing resolved notification types: %#v", notifier.resolves)
	}
}
