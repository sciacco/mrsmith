package rda

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildPOActionModelStates(t *testing.T) {
	tests := []struct {
		name       string
		po         poDetail
		permission rdaPermissionFlag
		actionID   string
		modeID     string
	}{
		{
			name:       "level approver",
			po:         poForActionModel("PENDING_APPROVAL", "requester@example.com", approverEmail("user@example.com")),
			permission: permissionApprover,
			actionID:   "approve",
			modeID:     "approver_l1_l2",
		},
		{
			name:       "payment method AFC",
			po:         poForActionModel("PENDING_APPROVAL_PAYMENT_METHOD", "requester@example.com"),
			permission: permissionAFC,
			actionID:   "payment-method/approve",
			modeID:     "afc_payment",
		},
		{
			name:       "leasing AFC",
			po:         poForActionModel("PENDING_LEASING", "requester@example.com"),
			permission: permissionAFC,
			actionID:   "leasing/approve",
			modeID:     "afc_leasing",
		},
		{
			name:       "leasing created AFC",
			po:         poForActionModel("PENDING_LEASING_ORDER_CREATION", "requester@example.com"),
			permission: permissionAFC,
			actionID:   "leasing/created",
			modeID:     "afc_leasing_created",
		},
		{
			name:       "no leasing approver",
			po:         poForActionModel("PENDING_APPROVAL_NO_LEASING", "requester@example.com"),
			permission: permissionApproverNoLeasing,
			actionID:   "no-leasing/approve",
			modeID:     "no_leasing",
		},
		{
			name:       "extra budget approver",
			po:         poForActionModel("PENDING_BUDGET_INCREMENT", "requester@example.com"),
			permission: permissionExtraBudget,
			actionID:   "budget-increment/approve",
			modeID:     "extra_budget",
		},
		{
			name:     "send provider",
			po:       poForActionModel("PENDING_SEND", "requester@example.com"),
			actionID: "send-to-provider",
			modeID:   "send_provider",
		},
		{
			name:     "conformity",
			po:       poForActionModel("PENDING_VERIFICATION", "requester@example.com"),
			actionID: "conformity/confirm",
			modeID:   "conformity",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := buildPOActionModel(tc.po, poActionPermissions{rdaPermissions: permissionsWith(tc.permission), Status: poPermissionAvailable}, "user@example.com", 3000)
			if !hasMode(model, tc.modeID) {
				t.Fatalf("expected mode %q in %#v", tc.modeID, model.Modes)
			}
			action, ok := findAction(model, tc.actionID, tc.modeID)
			if !ok {
				t.Fatalf("expected action %q for mode %q in %#v", tc.actionID, tc.modeID, model.Actions)
			}
			if action.Disabled {
				t.Fatalf("expected action %q to be enabled, got reason %q", tc.actionID, action.DisabledReason)
			}
		})
	}
}

func TestBuildPOActionModelPaymentRequesterAndAFCModes(t *testing.T) {
	model := buildPOActionModel(
		poForActionModel("PENDING_APPROVAL_PAYMENT_METHOD", "user@example.com"),
		poActionPermissions{rdaPermissions: rdaPermissions{IsAFC: true}, Status: poPermissionAvailable},
		"user@example.com",
		3000,
	)

	if model.PrimaryModeID != "requester_payment_update" {
		t.Fatalf("expected requester payment mode as primary, got %q", model.PrimaryModeID)
	}
	if !hasMode(model, "requester_payment_update") || !hasMode(model, "afc_payment") {
		t.Fatalf("expected requester and AFC modes, got %#v", model.Modes)
	}
}

func TestBuildPOActionModelApproverNotAssignedDoesNotSeeApprove(t *testing.T) {
	model := buildPOActionModel(
		poForActionModel("PENDING_APPROVAL", "requester@example.com", approverEmail("other@example.com")),
		poActionPermissions{rdaPermissions: rdaPermissions{IsApprover: true}, Status: poPermissionAvailable},
		"user@example.com",
		3000,
	)

	if _, ok := findAction(model, "approve", "approver_l1_l2"); ok {
		t.Fatalf("unassigned approver should not see approve action: %#v", model.Actions)
	}
}

func TestBuildPOActionModelSubmitReadiness(t *testing.T) {
	po := poForActionModel("DRAFT", "user@example.com")
	po.TotalPrice = "3500.00"
	po.Rows = []json.RawMessage{json.RawMessage(`{"id":10}`)}
	po.Attachments = []json.RawMessage{json.RawMessage(`{"attachment_type":"quote"}`)}

	model := buildPOActionModel(po, poActionPermissions{Status: poPermissionAvailable}, "user@example.com", 3000)
	action, ok := findAction(model, "submit", "requester_draft")
	if !ok {
		t.Fatalf("expected submit action")
	}
	if !action.Disabled {
		t.Fatalf("expected submit to be blocked by quote threshold")
	}

	po.Attachments = append(po.Attachments, json.RawMessage(`{"attachment_type":"quote"}`))
	model = buildPOActionModel(po, poActionPermissions{Status: poPermissionAvailable}, "user@example.com", 3000)
	action, _ = findAction(model, "submit", "requester_draft")
	if action.Disabled {
		t.Fatalf("expected submit to be enabled, got reason %q", action.DisabledReason)
	}
}

func TestHandleGetPOReturnsActionModelWhenPermissionsUnavailable(t *testing.T) {
	h, _ := newPaymentValidationHandler(t, paymentValidationFixture{
		poDetail: poActionModelDetailJSON("PENDING_APPROVAL", "requester@example.com", "user@example.com"),
	})
	h.arakDB = nil

	rec := httptest.NewRecorder()
	req := authedRDARequest(http.MethodGet, "/rda/v1/pos/42", nil)
	req.SetPathValue("id", "42")
	h.handleGetPO(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		ActionModel poActionModel `json:"action_model"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.ActionModel.PermissionStatus != poPermissionUnavailable {
		t.Fatalf("expected permission unavailable, got %q", body.ActionModel.PermissionStatus)
	}
	action, ok := findAction(body.ActionModel, "approve", "approver_l1_l2")
	if !ok || !action.Disabled {
		t.Fatalf("expected disabled approval action, got %#v", body.ActionModel.Actions)
	}
}

func poForActionModel(state string, requester string, approverEmails ...string) poDetail {
	approvers := make([]approverRef, 0, len(approverEmails))
	for _, email := range approverEmails {
		var approver approverRef
		approver.User.Email = email
		approvers = append(approvers, approver)
	}
	return poDetail{
		ID:            42,
		State:         state,
		TotalPrice:    "1250.00",
		Currency:      "EUR",
		Requester:     userRef{Email: requester},
		Rows:          []json.RawMessage{json.RawMessage(`{"id":1}`)},
		Attachments:   []json.RawMessage{},
		Recipients:    []json.RawMessage{json.RawMessage(`{"id":2}`)},
		PaymentMethod: json.RawMessage(`{"code":"SUP","description":"Supplier"}`),
		Approvers:     approvers,
	}
}

func poActionModelDetailJSON(state string, requester string, approver string) string {
	body := map[string]any{
		"id":             42,
		"state":          state,
		"total_price":    "1250.00",
		"currency":       "EUR",
		"requester":      map[string]any{"email": requester},
		"rows":           []map[string]any{{"id": 1}},
		"attachments":    []map[string]any{},
		"recipients":     []map[string]any{{"id": 2}},
		"payment_method": map[string]any{"code": "SUP", "description": "Supplier"},
		"approvers":      []map[string]any{{"user": map[string]any{"email": approver}}},
	}
	encoded, _ := json.Marshal(body)
	return string(encoded)
}

func approverEmail(email string) string {
	return email
}

func permissionsWith(flag rdaPermissionFlag) rdaPermissions {
	switch flag {
	case permissionApprover:
		return rdaPermissions{IsApprover: true}
	case permissionAFC:
		return rdaPermissions{IsAFC: true}
	case permissionApproverNoLeasing:
		return rdaPermissions{IsApproverNoLeasing: true}
	case permissionExtraBudget:
		return rdaPermissions{IsApproverExtraBudget: true}
	default:
		return rdaPermissions{}
	}
}

func hasMode(model poActionModel, modeID string) bool {
	for _, mode := range model.Modes {
		if mode.ID == modeID {
			return true
		}
	}
	return false
}

func findAction(model poActionModel, actionID string, modeID string) (poAction, bool) {
	for _, action := range model.Actions {
		if action.ID == actionID && action.ModeID == modeID {
			return action, true
		}
	}
	return poAction{}, false
}
