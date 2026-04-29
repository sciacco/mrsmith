package rda

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReplaceRowCreatesThenDeletesOldRow(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
		poDetail: poDetailWithRowsJSON("DRAFT", "user@example.com", 77),
	})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPut, "/rda/v1/pos/42/rows/77", strings.NewReader(validReplaceRowBody()))
	req.SetPathValue("id", "42")
	req.SetPathValue("rowId", "77")

	h.handleReplaceRow(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}
	if arakState.count(http.MethodPost, "/arak/rda/v1/po/42/row") != 1 {
		t.Fatalf("expected replacement row create")
	}
	if arakState.count(http.MethodDelete, "/arak/rda/v1/po/42/row/77") != 1 {
		t.Fatalf("expected old row delete")
	}
	if !arakState.requestBefore(http.MethodPost, "/arak/rda/v1/po/42/row", http.MethodDelete, "/arak/rda/v1/po/42/row/77") {
		t.Fatalf("expected create request before delete request")
	}
	body := arakState.lastJSONBody(t, http.MethodPost, "/arak/rda/v1/po/42/row")
	if got := body["requester_email"]; got != "user@example.com" {
		t.Fatalf("expected requester email from claims, got %#v", got)
	}
}

func TestReplaceRowDoesNotDeleteWhenCreateFails(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
		poDetail:        poDetailWithRowsJSON("DRAFT", "user@example.com", 77),
		rowCreateStatus: http.StatusBadRequest,
		rowCreateBody:   `{"error":"invalid row"}`,
	})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPut, "/rda/v1/pos/42/rows/77", strings.NewReader(validReplaceRowBody()))
	req.SetPathValue("id", "42")
	req.SetPathValue("rowId", "77")

	h.handleReplaceRow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if arakState.count(http.MethodDelete, "/arak/rda/v1/po/42/row/77") != 0 {
		t.Fatalf("expected old row not to be deleted")
	}
}

func TestReplaceRowReportsPartialFailureWhenDeleteFails(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
		poDetail:        poDetailWithRowsJSON("DRAFT", "user@example.com", 77),
		rowDeleteStatus: http.StatusInternalServerError,
		rowDeleteBody:   `{"error":"delete failed"}`,
	})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPut, "/rda/v1/pos/42/rows/77", strings.NewReader(validReplaceRowBody()))
	req.SetPathValue("id", "42")
	req.SetPathValue("rowId", "77")

	h.handleReplaceRow(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["code"] != codeRowReplaceDeleteFailed {
		t.Fatalf("expected %s, got %#v", codeRowReplaceDeleteFailed, body)
	}
	if body["created_row_id"] != "9001" {
		t.Fatalf("expected created row id, got %#v", body)
	}
	if arakState.count(http.MethodPost, "/arak/rda/v1/po/42/row") != 1 {
		t.Fatalf("expected replacement row create")
	}
	if arakState.count(http.MethodDelete, "/arak/rda/v1/po/42/row/77") != 1 {
		t.Fatalf("expected attempted old row delete")
	}
}

func TestReplaceRowRejectsMissingTargetRow(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
		poDetail: poDetailWithRowsJSON("DRAFT", "user@example.com", 88),
	})

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodPut, "/rda/v1/pos/42/rows/77", strings.NewReader(validReplaceRowBody()))
	req.SetPathValue("id", "42")
	req.SetPathValue("rowId", "77")

	h.handleReplaceRow(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	if arakState.count(http.MethodPost, "/arak/rda/v1/po/42/row") != 0 {
		t.Fatalf("expected no replacement row create")
	}
}

func TestReplaceRowRequiresRequesterDraft(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		requester string
	}{
		{name: "not draft", state: "PENDING_APPROVAL", requester: "user@example.com"},
		{name: "not requester", state: "DRAFT", requester: "other@example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
				poDetail: poDetailWithRowsJSON(tc.state, tc.requester, 77),
			})

			rec := httptest.NewRecorder()
			req := authedRDARequest(http.MethodPut, "/rda/v1/pos/42/rows/77", strings.NewReader(validReplaceRowBody()))
			req.SetPathValue("id", "42")
			req.SetPathValue("rowId", "77")

			h.handleReplaceRow(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
			}
			if arakState.count(http.MethodPost, "/arak/rda/v1/po/42/row") != 0 {
				t.Fatalf("expected no replacement row create")
			}
		})
	}
}

func validReplaceRowBody() string {
	return `{
		"type": "service",
		"description": "SOFTWARE USO INTERNO",
		"qty": 1,
		"product_code": "SOFTWARE",
		"product_description": "SOFTWARE USO INTERNO - SOFTWARE (UI)",
		"monthly_fee": 1,
		"activation_price": 0,
		"payment_detail": {
			"start_at": "activation_date",
			"month_recursion": 1
		},
		"renew_detail": {
			"initial_subscription_months": 12,
			"automatic_renew": false
		}
	}`
}

func poDetailWithRowsJSON(state string, requester string, rowIDs ...int64) string {
	rows := make([]map[string]any, 0, len(rowIDs))
	for _, id := range rowIDs {
		rows = append(rows, map[string]any{"id": id, "type": "service"})
	}
	body := map[string]any{
		"id":        42,
		"state":     state,
		"requester": map[string]any{"email": requester},
		"rows":      rows,
	}
	encoded, _ := json.Marshal(body)
	return string(encoded)
}

func (s *paymentValidationArakState) requestBefore(firstMethod string, firstPath string, secondMethod string, secondPath string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	firstIndex := -1
	secondIndex := -1
	for index, request := range s.requests {
		if request.method == firstMethod && request.path == firstPath && firstIndex == -1 {
			firstIndex = index
		}
		if request.method == secondMethod && request.path == secondPath && secondIndex == -1 {
			secondIndex = index
		}
	}
	return firstIndex >= 0 && secondIndex >= 0 && firstIndex < secondIndex
}
