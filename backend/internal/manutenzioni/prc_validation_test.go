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

func TestValidateOperatedServiceDomains(t *testing.T) {
	serviceDomains := map[int64]int64{
		10: 1,
		20: 2,
	}
	validSameDomain := []classificationInput{
		{ReferenceID: 10, Role: "operated"},
	}
	if err := validateOperatedServiceDomains(1, serviceDomains, validSameDomain); err != nil {
		t.Fatalf("operated same-domain returned error: %v", err)
	}
	validDependentCrossDomain := []classificationInput{
		{ReferenceID: 20, Role: "dependent"},
	}
	if err := validateOperatedServiceDomains(1, serviceDomains, validDependentCrossDomain); err != nil {
		t.Fatalf("dependent cross-domain returned error: %v", err)
	}
	emptyRoleDefaultsToOperated := []classificationInput{
		{ReferenceID: 20},
	}
	if err := validateOperatedServiceDomains(1, serviceDomains, emptyRoleDefaultsToOperated); err == nil {
		t.Fatalf("empty role cross-domain should be rejected as operated")
	}
	invalidCrossDomain := []classificationInput{
		{ReferenceID: 20, Role: "operated"},
	}
	if err := validateOperatedServiceDomains(1, serviceDomains, invalidCrossDomain); err == nil {
		t.Fatalf("operated cross-domain should be rejected")
	}
	missingService := []classificationInput{
		{ReferenceID: 30, Role: "dependent"},
	}
	if err := validateOperatedServiceDomains(1, serviceDomains, missingService); err == nil {
		t.Fatalf("unknown service should be rejected")
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
