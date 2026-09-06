package binocolo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestMACompanyAgreementServiceLifecycleAndDiary(t *testing.T) {
	store := &fakeMAWorkspaceStore{companyKnown: map[string]bool{"COMPANY-1": true}}
	service := newMAService(store, nil, nil, nil, nil, nil)

	agreement, err := service.createCompanyAgreement(context.Background(), "company-1", MACompanyAgreementWrite{
		Kind: maAgreementKindNDA, SignedOn: "2026-01-15",
	}, "subject", "agent@example.test")
	if err != nil {
		t.Fatalf("create agreement: %v", err)
	}
	if agreement.CompanyKey != "COMPANY-1" || agreement.ExpiresOn != "" {
		t.Fatalf("unexpected agreement: %#v", agreement)
	}
	if len(store.outcomes) != 1 {
		t.Fatalf("outcomes after create = %d, want 1", len(store.outcomes))
	}
	createdOutcome := store.outcomes[0]
	if createdOutcome.Event != maEventAccordo || createdOutcome.SessionID != "" || createdOutcome.InitiativeID != "" {
		t.Fatalf("unexpected created outcome: %#v", createdOutcome)
	}
	var createdPayload map[string]any
	if err := json.Unmarshal(createdOutcome.Payload, &createdPayload); err != nil {
		t.Fatalf("unmarshal created payload: %v", err)
	}
	if createdPayload["action"] != "created" {
		t.Fatalf("created payload action = %v", createdPayload["action"])
	}
	if _, ok := createdPayload["expiresOn"]; !ok || createdPayload["expiresOn"] != nil {
		t.Fatalf("created payload expiresOn = %v, want nil", createdPayload["expiresOn"])
	}

	expiresOn := "2026-12-31"
	updated, err := service.updateCompanyAgreement(context.Background(), "company-1", agreement.ID, MACompanyAgreementReplaceRequest{
		Kind: &agreement.Kind, SignedOn: &agreement.SignedOn, ExpiresOn: &expiresOn,
	}, "subject", "agent@example.test")
	if err != nil {
		t.Fatalf("update agreement: %v", err)
	}
	if updated.ExpiresOn != expiresOn {
		t.Fatalf("updated expiresOn = %q, want %q", updated.ExpiresOn, expiresOn)
	}
	if len(store.outcomes) != 2 {
		t.Fatalf("outcomes after update = %d, want 2", len(store.outcomes))
	}
	updatedOutcome := store.outcomes[1]
	var updatedPayload struct {
		Action  string `json:"action"`
		Changes []struct {
			Field string `json:"field"`
			From  any    `json:"from"`
			To    any    `json:"to"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(updatedOutcome.Payload, &updatedPayload); err != nil {
		t.Fatalf("unmarshal updated payload: %v", err)
	}
	if updatedPayload.Action != "updated" {
		t.Fatalf("updated payload action = %q", updatedPayload.Action)
	}
	if len(updatedPayload.Changes) != 1 || updatedPayload.Changes[0].Field != "expiresOn" || updatedPayload.Changes[0].From != nil || updatedPayload.Changes[0].To != expiresOn {
		t.Fatalf("unexpected changes: %#v", updatedPayload.Changes)
	}

	// Identical update: no new diary event.
	if _, err := service.updateCompanyAgreement(context.Background(), "company-1", agreement.ID, MACompanyAgreementReplaceRequest{
		Kind: &updated.Kind, SignedOn: &updated.SignedOn, ExpiresOn: &updated.ExpiresOn,
	}, "subject", "agent@example.test"); err != nil {
		t.Fatalf("no-op update: %v", err)
	}
	if len(store.outcomes) != 2 {
		t.Fatalf("outcomes after no-op update = %d, want 2", len(store.outcomes))
	}

	if err := service.deleteCompanyAgreement(context.Background(), "company-1", agreement.ID, "subject", "agent@example.test"); err != nil {
		t.Fatalf("delete agreement: %v", err)
	}
	if len(store.outcomes) != 3 {
		t.Fatalf("outcomes after delete = %d, want 3", len(store.outcomes))
	}
	deletedOutcome := store.outcomes[2]
	var deletedPayload map[string]any
	if err := json.Unmarshal(deletedOutcome.Payload, &deletedPayload); err != nil {
		t.Fatalf("unmarshal deleted payload: %v", err)
	}
	if deletedPayload["action"] != "removed" || deletedPayload["signedOn"] != agreement.SignedOn || deletedPayload["expiresOn"] != expiresOn {
		t.Fatalf("unexpected deleted payload: %#v", deletedPayload)
	}
	list, err := service.listCompanyAgreements(context.Background(), "company-1")
	if err != nil {
		t.Fatalf("list agreements: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("list after delete = %#v, want empty", list)
	}
}

func TestMACompanyAgreementServiceValidationAndDomainErrors(t *testing.T) {
	store := &fakeMAWorkspaceStore{companyKnown: map[string]bool{"COMPANY-1": true}}
	service := newMAService(store, nil, nil, nil, nil, nil)

	if _, err := service.createCompanyAgreement(context.Background(), "company-1", MACompanyAgreementWrite{
		Kind: "other", SignedOn: "2026-01-15",
	}, "", ""); !errors.Is(err, errMAStrategyInvalid) {
		t.Fatalf("invalid kind error = %v", err)
	}
	if _, err := service.createCompanyAgreement(context.Background(), "company-1", MACompanyAgreementWrite{
		Kind: maAgreementKindNDA,
	}, "", ""); !errors.Is(err, errMAStrategyInvalid) {
		t.Fatalf("empty signedOn error = %v", err)
	}
	if _, err := service.createCompanyAgreement(context.Background(), "company-1", MACompanyAgreementWrite{
		Kind: maAgreementKindNDA, SignedOn: "2026-01-15", ExpiresOn: "2026-01-01",
	}, "", ""); !errors.Is(err, errMAStrategyInvalid) {
		t.Fatalf("expiresOn before signedOn error = %v", err)
	}
	if _, err := service.createCompanyAgreement(context.Background(), "company-1", MACompanyAgreementWrite{
		Kind: maAgreementKindNDA, SignedOn: "15-01-2026",
	}, "", ""); !errors.Is(err, errMAStrategyInvalid) {
		t.Fatalf("malformed date error = %v", err)
	}
	if _, err := service.createCompanyAgreement(context.Background(), "unknown", MACompanyAgreementWrite{
		Kind: maAgreementKindNDA, SignedOn: "2026-01-15",
	}, "", ""); !errors.Is(err, errMACompanyKeyUnknown) {
		t.Fatalf("unknown company error = %v", err)
	}
	kind, signedOn := maAgreementKindNDA, "2026-01-15"
	if _, err := service.updateCompanyAgreement(context.Background(), "company-1", "not-a-uuid", MACompanyAgreementReplaceRequest{
		Kind: &kind, SignedOn: &signedOn, ExpiresOn: new(string),
	}, "", ""); !errors.Is(err, errMACompanyAgreementNotFound) {
		t.Fatalf("bad uuid on update error = %v", err)
	}
	if err := service.deleteCompanyAgreement(context.Background(), "company-1", "not-a-uuid", "", ""); !errors.Is(err, errMACompanyAgreementNotFound) {
		t.Fatalf("bad uuid on delete error = %v", err)
	}
	status, code, _ := maHTTPError(errMACompanyAgreementNotFound)
	if status != http.StatusNotFound || code != "ma_company_agreement_not_found" {
		t.Fatalf("HTTP mapping = %d %q", status, code)
	}
}
