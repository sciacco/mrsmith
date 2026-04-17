package coperture

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleListCoverageNormalizesResponse(t *testing.T) {
	h := &Handler{db: openCopertureTestDB(t, "coverage")}

	req := httptest.NewRequest(http.MethodGet, "/coperture/v1/house-numbers/44/coverage", nil)
	req.SetPathValue("houseNumberId", "44")
	rec := httptest.NewRecorder()

	h.handleListCoverage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var results []coverageResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.CoverageID != "71" {
		t.Fatalf("expected coverage_id 71, got %q", result.CoverageID)
	}
	if result.OperatorName != "TIM" {
		t.Fatalf("expected operator_name TIM, got %q", result.OperatorName)
	}
	if result.LogoURL != "https://static.cdlan.business/x/logo_tim.png" {
		t.Fatalf("unexpected logo_url %q", result.LogoURL)
	}
	if len(result.Profiles) != 2 || result.Profiles[0].Name != "GEA 1000/1000" {
		t.Fatalf("unexpected profiles %#v", result.Profiles)
	}
	if len(result.Details) != 2 {
		t.Fatalf("expected 2 details, got %d", len(result.Details))
	}
	if result.Details[0].TypeName != "Download" {
		t.Fatalf("expected first detail type_name Download, got %q", result.Details[0].TypeName)
	}
	if result.Details[0].Value != "100" {
		t.Fatalf("expected first detail value 100 after normalization, got %q", result.Details[0].Value)
	}
	if result.Details[1].Value != "5000" {
		t.Fatalf("expected second detail value 5000 after normalization, got %q", result.Details[1].Value)
	}
}
