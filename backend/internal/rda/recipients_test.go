package rda

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdatePORecipientsForwardsDedicatedEndpoint(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPatch, "/rda/v1/pos/42/recipients", strings.NewReader(`{"recipient_ids":[11,22]}`))
	req.SetPathValue("id", "42")
	h.handleUpdatePORecipients(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	request := arakState.lastRequest(t, http.MethodPatch, "/arak/rda/v1/po/42/recipients")
	if got := request.header.Get("Requester-Email"); got != "user@example.com" {
		t.Fatalf("expected Requester-Email header from claims, got %q", got)
	}
	var body map[string][]int64
	if err := json.Unmarshal(request.body, &body); err != nil {
		t.Fatalf("failed to decode forwarded body: %v body=%s", err, string(request.body))
	}
	if got := body["recipient_ids"]; len(got) != 2 || got[0] != 11 || got[1] != 22 {
		t.Fatalf("expected forwarded recipient IDs [11 22], got %#v", got)
	}
}

func TestUpdatePORecipientsAcceptsEmptySelection(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPatch, "/rda/v1/pos/42/recipients", strings.NewReader(`{"recipient_ids":[]}`))
	req.SetPathValue("id", "42")
	h.handleUpdatePORecipients(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := arakState.lastJSONBody(t, http.MethodPatch, "/arak/rda/v1/po/42/recipients")
	recipients, ok := body["recipient_ids"].([]any)
	if !ok || len(recipients) != 0 {
		t.Fatalf("expected empty recipient_ids array, got %#v", body["recipient_ids"])
	}
}

func TestUpdatePORecipientsDoesNotRequireDraftOrRequester(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
		poDetail: poDetailWithRowsJSON("PENDING_APPROVAL", "other@example.com"),
	})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPatch, "/rda/v1/pos/42/recipients", strings.NewReader(`{"recipient_ids":[7]}`))
	req.SetPathValue("id", "42")
	h.handleUpdatePORecipients(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := arakState.count(http.MethodGet, "/arak/rda/v1/po/42"); got != 0 {
		t.Fatalf("expected recipients update not to fetch PO detail, got %d fetches", got)
	}
	if got := arakState.count(http.MethodPatch, "/arak/rda/v1/po/42/recipients"); got != 1 {
		t.Fatalf("expected one recipients patch, got %d", got)
	}
}

func TestUpdatePORecipientsRejectsInvalidPayload(t *testing.T) {
	tests := []struct {
		name string
		id   string
		body string
	}{
		{name: "invalid path id", id: "0", body: `{"recipient_ids":[]}`},
		{name: "missing recipient ids", id: "42", body: `{}`},
		{name: "null recipient ids", id: "42", body: `{"recipient_ids":null}`},
		{name: "zero id", id: "42", body: `{"recipient_ids":[0]}`},
		{name: "negative id", id: "42", body: `{"recipient_ids":[-1]}`},
		{name: "non integer id", id: "42", body: `{"recipient_ids":["1"]}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

			rec := httptest.NewRecorder()
			req := authedRDARequest(http.MethodPatch, "/rda/v1/pos/"+tc.id+"/recipients", strings.NewReader(tc.body))
			req.SetPathValue("id", tc.id)
			h.handleUpdatePORecipients(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
			}
			if got := arakState.count(http.MethodPatch, "/arak/rda/v1/po/42/recipients"); got != 0 {
				t.Fatalf("expected invalid request not to be forwarded, got %d forwards", got)
			}
		})
	}
}

func TestUpdatePORecipientsPreservesUpstreamForbidden(t *testing.T) {
	h, _ := newPaymentValidationHandler(t, paymentValidationFixture{
		recipientStatus: http.StatusForbidden,
		recipientBody:   `{"error":"forbidden"}`,
	})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPatch, "/rda/v1/pos/42/recipients", strings.NewReader(`{"recipient_ids":[11]}`))
	req.SetPathValue("id", "42")
	h.handleUpdatePORecipients(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected upstream 403 to be preserved, got %d body=%s", rec.Code, rec.Body.String())
	}
}
