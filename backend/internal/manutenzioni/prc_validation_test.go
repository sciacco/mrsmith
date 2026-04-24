package manutenzioni

import "testing"

func TestDependencyGraphSourceOnlyAllowedForServiceTaxonomy(t *testing.T) {
	if !validClassificationSource(serviceTaxonomyClass, "dependency_graph") {
		t.Fatalf("dependency_graph should be accepted for service taxonomy")
	}
	if validClassificationSource(reasonClassClass, "dependency_graph") {
		t.Fatalf("dependency_graph should not be accepted for reason classes")
	}
}

func TestResolveServiceTaxonomyIDPreservesLegacyReferenceID(t *testing.T) {
	withServiceID := classificationInput{ReferenceID: 10, ServiceTaxonomyID: 20}
	if got := resolveClassificationReferenceID(serviceTaxonomyClass, withServiceID); got != 20 {
		t.Fatalf("service taxonomy id = %d, want 20", got)
	}
	legacy := classificationInput{ReferenceID: 10}
	if got := resolveClassificationReferenceID(serviceTaxonomyClass, legacy); got != 10 {
		t.Fatalf("legacy reference id = %d, want 10", got)
	}
}

func TestValidateServiceDependencyRequest(t *testing.T) {
	valid := serviceDependencyRequest{
		UpstreamServiceID:   1,
		DownstreamServiceID: 2,
		DependencyType:      "runs_on",
		DefaultSeverity:     "degraded",
	}
	if err := validateServiceDependencyRequest(valid); err != nil {
		t.Fatalf("valid dependency returned error: %v", err)
	}
	sameService := valid
	sameService.DownstreamServiceID = sameService.UpstreamServiceID
	if err := validateServiceDependencyRequest(sameService); err == nil {
		t.Fatalf("same upstream/downstream should be rejected")
	}
	badSeverity := valid
	badSeverity.DefaultSeverity = "partial"
	if err := validateServiceDependencyRequest(badSeverity); err == nil {
		t.Fatalf("invalid severity should be rejected")
	}
}

func TestDraftCancelTransitionRemainsAllowed(t *testing.T) {
	next, _, _, ok := nextStatus(StatusDraft, "cancel")
	if !ok || next != StatusCancelled {
		t.Fatalf("draft cancel = (%q, %v), want cancelled true", next, ok)
	}
}
