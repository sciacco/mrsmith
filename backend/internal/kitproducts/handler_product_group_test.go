package kitproducts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleListLanguagesReturnsConfiguredLanguages(t *testing.T) {
	h := &Handler{mistraDB: openKitProductsTestDB(t, "product-group-success", nil)}

	req := httptest.NewRequest(http.MethodGet, "/kit-products/v1/lookup/language", nil)
	rec := httptest.NewRecorder()

	h.handleListLanguages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body []LanguageOption
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(body))
	}
	if body[0].ISO != "en" || body[1].ISO != "it" {
		t.Fatalf("unexpected language payload: %#v", body)
	}
}

func TestHandleCreateProductGroupDefaultsMissingShortsToNameAndCommits(t *testing.T) {
	tracker := &kitProductsTxTracker{}
	h := &Handler{mistraDB: openKitProductsTestDB(t, "product-group-create", tracker)}

	req := httptest.NewRequest(http.MethodPost, "/kit-products/v1/product-group", strings.NewReader(`{"name":"Router","translations":[{"language":"it","short":"","long":"Descrizione IT"}]}`))
	rec := httptest.NewRecorder()

	h.handleCreateProductGroup(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if tracker.commitCount != 1 {
		t.Fatalf("expected 1 commit, got %d", tracker.commitCount)
	}

	translationCalls := collectExecsContaining(tracker.execs, "INSERT INTO common.translation")
	if len(translationCalls) != 2 {
		t.Fatalf("expected 2 translation upserts, got %d", len(translationCalls))
	}

	seen := map[string]struct {
		short string
		long  string
	}{}
	for _, call := range translationCalls {
		language := call.Args[1].(string)
		seen[language] = struct {
			short string
			long  string
		}{
			short: call.Args[2].(string),
			long:  call.Args[3].(string),
		}
	}

	if seen["en"].short != "Router" || seen["en"].long != "" {
		t.Fatalf("expected english fallback short, got %#v", seen["en"])
	}
	if seen["it"].short != "Router" || seen["it"].long != "Descrizione IT" {
		t.Fatalf("expected italian fallback short, got %#v", seen["it"])
	}
}

func TestHandleCreateProductGroupRejectsDuplicateNameCaseInsensitive(t *testing.T) {
	h := &Handler{mistraDB: openKitProductsTestDB(t, "product-group-duplicate", nil)}

	req := httptest.NewRequest(http.MethodPost, "/kit-products/v1/product-group", strings.NewReader(`{"name":"circuito","translations":[{"language":"it","short":"Circuito"},{"language":"en","short":"Circuit"}]}`))
	rec := httptest.NewRecorder()

	h.handleCreateProductGroup(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"duplicate_name"}` {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestHandleUpdateProductGroupRequiresRenameConfirmation(t *testing.T) {
	h := &Handler{mistraDB: openKitProductsTestDB(t, "product-group-rename-confirm", nil)}

	req := httptest.NewRequest(http.MethodPut, "/kit-products/v1/product-group/44444444-4444-4444-4444-444444444444", strings.NewReader(`{"name":"Circuito primario","translations":[{"language":"it","short":"Circuito primario"},{"language":"en","short":"Primary Circuit"}]}`))
	req.SetPathValue("translationUUID", "44444444-4444-4444-4444-444444444444")
	rec := httptest.NewRecorder()

	h.handleUpdateProductGroup(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	var body struct {
		Error               string `json:"error"`
		ImpactedKitProducts int    `json:"impacted_kit_products"`
		QuotesUnchanged     bool   `json:"quotes_unchanged"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Error != "rename_confirmation_required" {
		t.Fatalf("unexpected error payload: %#v", body)
	}
	if body.ImpactedKitProducts != 3 || !body.QuotesUnchanged {
		t.Fatalf("unexpected confirmation payload: %#v", body)
	}
}

func TestHandleUpdateProductGroupPropagatesRenameAfterConfirmation(t *testing.T) {
	tracker := &kitProductsTxTracker{}
	h := &Handler{mistraDB: openKitProductsTestDB(t, "product-group-update", tracker)}

	req := httptest.NewRequest(http.MethodPut, "/kit-products/v1/product-group/44444444-4444-4444-4444-444444444444", strings.NewReader(`{"name":"Circuito primario","confirm_propagation":true,"translations":[{"language":"it","short":"Circuito primario","long":"Descrizione"},{"language":"en","short":"Primary Circuit","long":""}]}`))
	req.SetPathValue("translationUUID", "44444444-4444-4444-4444-444444444444")
	rec := httptest.NewRecorder()

	h.handleUpdateProductGroup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if tracker.commitCount != 1 {
		t.Fatalf("expected 1 commit, got %d", tracker.commitCount)
	}

	propagationCalls := collectExecsContaining(tracker.execs, "UPDATE products.kit_product")
	if len(propagationCalls) != 1 {
		t.Fatalf("expected 1 propagation query, got %d", len(propagationCalls))
	}
	if propagationCalls[0].Args[0] != "Circuito primario" || propagationCalls[0].Args[1] != "Circuito" {
		t.Fatalf("unexpected propagation args: %#v", propagationCalls[0].Args)
	}
}

func TestHandleUpdateProductGroupReturnsNotFoundWhenMissing(t *testing.T) {
	h := &Handler{mistraDB: openKitProductsTestDB(t, "product-group-missing", nil)}

	req := httptest.NewRequest(http.MethodPut, "/kit-products/v1/product-group/44444444-4444-4444-4444-444444444444", strings.NewReader(`{"name":"Circuito","translations":[{"language":"it","short":"Circuito"},{"language":"en","short":"Circuit"}]}`))
	req.SetPathValue("translationUUID", "44444444-4444-4444-4444-444444444444")
	rec := httptest.NewRecorder()

	h.handleUpdateProductGroup(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"not_found"}` {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func collectExecsContaining(calls []kitProductsDBCall, fragment string) []kitProductsDBCall {
	matches := make([]kitProductsDBCall, 0)
	for _, call := range calls {
		if strings.Contains(call.Query, fragment) {
			matches = append(matches, call)
		}
	}
	return matches
}
