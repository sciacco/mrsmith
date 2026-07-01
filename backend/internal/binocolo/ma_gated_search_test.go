package binocolo

import "testing"

// gatedTestTarget builds an address-stage target with a gate verdict. An empty action
// leaves WebValidation nil (the "gate did not decide" case).
func gatedTestTarget(id, finalAction, enrichment string) MATarget {
	t := MATarget{ID: id, VATCode: "VAT" + id, CompanyName: "Co " + id, EnrichmentLevel: enrichment}
	if finalAction != "" {
		t.WebValidation = &MAWebValidation{FinalAction: finalAction}
	}
	return t
}

// gatedTestTargetKeyed is gatedTestTarget with an explicit company_key, for the
// association remedy (which selects the target to enrich by company_key).
func gatedTestTargetKeyed(id, companyKey, finalAction, enrichment string) MATarget {
	t := gatedTestTarget(id, finalAction, enrichment)
	t.CompanyKey = companyKey
	return t
}

func containsTargetID(targets []MATarget, id string) bool {
	for _, t := range targets {
		if t.ID == id {
			return true
		}
	}
	return false
}

// The gated funnel's whole cost premise rests on WHO pays the €0.10 Advanced. This pins
// the pure partition: reject never pays, a domain-unresolved company is HELD for manual
// review (never auto-charged), an already-advanced survivor is never re-charged
// (idempotent re-run), and a missing gate verdict is a forse survivor (recall-safe).
func TestPlanGatedEnrichmentSpendPartition(t *testing.T) {
	targets := []MATarget{
		gatedTestTarget("keep", "confirm", maEnrichmentAddress),                      // survivor → enrich
		gatedTestTarget("forse-depri", "deprioritize", maEnrichmentAddress),          // survivor → enrich
		gatedTestTarget("manual-domain", "needs_domain_review", maEnrichmentAddress), // domain unresolved → HELD, never charged
		gatedTestTarget("forse-nil", "", maEnrichmentAddress),                        // no verdict → forse → enrich
		gatedTestTarget("reject", "reject", maEnrichmentAddress),                     // salta → never charged
		gatedTestTarget("keep-done", "confirm", maEnrichmentAdvanced),                // already advanced → reuse
		gatedTestTarget("reject-done", "reject", maEnrichmentAdvanced),               // reject wins even if advanced
	}

	plan := planGatedEnrichment(targets)

	// Exactly the three fresh survivors pay Advanced this pass (manual-domain does NOT).
	if len(plan.ToEnrich) != 3 {
		t.Fatalf("ToEnrich = %d, want 3 (%v)", len(plan.ToEnrich), plan.ToEnrich)
	}
	for _, id := range []string{"keep", "forse-depri", "forse-nil"} {
		if !containsTargetID(plan.ToEnrich, id) {
			t.Errorf("survivor %q missing from ToEnrich", id)
		}
	}

	// INVARIANT 1: a reject is NEVER enriched (never pays €0.10).
	if containsTargetID(plan.ToEnrich, "reject") || containsTargetID(plan.ToEnrich, "reject-done") {
		t.Errorf("reject target leaked into ToEnrich — would pay Advanced")
	}
	if !containsTargetID(plan.Carry, "reject") || !containsTargetID(plan.Carry, "reject-done") {
		t.Errorf("reject targets must be carried identity-only, got Carry=%v", plan.Carry)
	}

	// INVARIANT 2: an already-advanced survivor is reused, never re-charged.
	if containsTargetID(plan.ToEnrich, "keep-done") {
		t.Errorf("already-advanced survivor leaked into ToEnrich — would re-charge on re-run")
	}
	if len(plan.Reuse) != 1 || !containsTargetID(plan.Reuse, "keep-done") {
		t.Errorf("Reuse = %v, want exactly [keep-done]", plan.Reuse)
	}

	// INVARIANT 3: recall-safety — a missing gate verdict is a survivor, not dropped.
	if !containsTargetID(plan.ToEnrich, "forse-nil") {
		t.Errorf("target with no gate verdict must be a forse survivor, not carried")
	}

	// INVARIANT 4: a domain-unresolved company is HELD for manual review — never enriched
	// (never pays Advanced), kept identity-only, distinct from a rejected company.
	if containsTargetID(plan.ToEnrich, "manual-domain") {
		t.Errorf("domain-unresolved target leaked into ToEnrich — would pay Advance on an unverified company")
	}
	if len(plan.ManualReview) != 1 || !containsTargetID(plan.ManualReview, "manual-domain") {
		t.Errorf("ManualReview = %v, want exactly [manual-domain]", plan.ManualReview)
	}
}

func TestGatedTargetBucket(t *testing.T) {
	cases := []struct {
		action string
		want   string
	}{
		{"confirm", maGatedBucketKeep},
		{"reject", maGatedBucketReject},
		{"deprioritize", maGatedBucketForse},
		{"needs_business_validation", maGatedBucketForse},
		{"needs_domain_review", maGatedBucketManualReview},
	}
	for _, c := range cases {
		got := gatedTargetBucket(gatedTestTarget("x", c.action, maEnrichmentAddress))
		if got != c.want {
			t.Errorf("gatedTargetBucket(%q) = %q, want %q", c.action, got, c.want)
		}
	}
	// A domain_unresolved state (even without the paired action) is manual_review.
	stateOnly := MATarget{ID: "s", WebValidation: &MAWebValidation{WebValidationState: "domain_unresolved"}}
	if got := gatedTargetBucket(stateOnly); got != maGatedBucketManualReview {
		t.Errorf("gatedTargetBucket(domain_unresolved state) = %q, want %q", got, maGatedBucketManualReview)
	}
	// No web validation → forse (recall-safe), never scarta or manual_review.
	if got := gatedTargetBucket(MATarget{ID: "n"}); got != maGatedBucketForse {
		t.Errorf("gatedTargetBucket(nil validation) = %q, want %q", got, maGatedBucketForse)
	}
}

// The manual-review remedy must charge Advanced for AT MOST the just-associated company:
// existing advanced survivors are re-scored for free and every other target (including a
// non-associated un-enriched forse) is carried untouched, so a per-company action never
// fans spend across the set.
func TestPlanDomainAssociationEnrichSpend(t *testing.T) {
	key := normalizeMACompanyKey("ASSOC")
	targets := []MATarget{
		gatedTestTargetKeyed("assoc-keep", "ASSOC", "confirm", maEnrichmentAddress),                // associated, now keep, not advanced → pays
		gatedTestTargetKeyed("other-adv", "OTHER1", "confirm", maEnrichmentAdvanced),               // existing advanced survivor → reuse free
		gatedTestTargetKeyed("other-forse", "OTHER2", "deprioritize", maEnrichmentAddress),         // NON-associated un-enriched forse → carry, no charge
		gatedTestTargetKeyed("other-reject", "OTHER3", "reject", maEnrichmentAddress),              // reject → carry
		gatedTestTargetKeyed("other-manual", "OTHER4", "needs_domain_review", maEnrichmentAddress), // still manual_review → carry
	}

	plan := planDomainAssociationEnrich(targets, key)

	// INVARIANT: exactly the associated survivor pays Advanced.
	if plan.Enrich == nil || plan.Enrich.ID != "assoc-keep" {
		t.Fatalf("Enrich = %v, want the associated survivor assoc-keep", plan.Enrich)
	}
	// The already-advanced survivor is reused, never re-charged.
	if len(plan.Reuse) != 1 || plan.Reuse[0].ID != "other-adv" {
		t.Errorf("Reuse = %v, want exactly [other-adv]", plan.Reuse)
	}
	// Everything else is carried untouched — no lingering-survivor spend fan-out.
	if len(plan.Carry) != 3 {
		t.Errorf("Carry = %d, want 3", len(plan.Carry))
	}
	for _, want := range []string{"other-forse", "other-reject", "other-manual"} {
		if !containsTargetID(plan.Carry, want) {
			t.Errorf("%q must be carried (never charged by a domain-association action)", want)
		}
	}
}

// The associated company only pays when it actually SURVIVES the re-gate, and never twice.
func TestPlanDomainAssociationEnrichAssociatedEdges(t *testing.T) {
	key := normalizeMACompanyKey("ASSOC")

	// Associated company classified reject after forcing a domain → carried, never pays.
	rejected := planDomainAssociationEnrich(
		[]MATarget{gatedTestTargetKeyed("assoc-reject", "ASSOC", "reject", maEnrichmentAddress)}, key)
	if rejected.Enrich != nil {
		t.Errorf("a rejected associated company must not pay Advanced, got Enrich=%v", rejected.Enrich)
	}
	if !containsTargetID(rejected.Carry, "assoc-reject") {
		t.Errorf("rejected associated company must be carried")
	}

	// Associated company already advanced (re-association) → reused, never re-charged.
	already := planDomainAssociationEnrich(
		[]MATarget{gatedTestTargetKeyed("assoc-adv", "ASSOC", "confirm", maEnrichmentAdvanced)}, key)
	if already.Enrich != nil {
		t.Errorf("an already-advanced associated company must not re-pay Advanced, got Enrich=%v", already.Enrich)
	}
	if len(already.Reuse) != 1 || already.Reuse[0].ID != "assoc-adv" {
		t.Errorf("already-advanced associated company must be reused, got Reuse=%v", already.Reuse)
	}
}
