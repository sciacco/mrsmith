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

func containsTargetID(targets []MATarget, id string) bool {
	for _, t := range targets {
		if t.ID == id {
			return true
		}
	}
	return false
}

// The gated funnel's whole cost premise rests on WHO pays the €0.10 Advanced. This pins
// the pure partition: reject never pays, an already-advanced survivor is never re-charged
// (idempotent re-run), and a missing gate verdict is a forse survivor (recall-safe).
func TestPlanGatedEnrichmentSpendPartition(t *testing.T) {
	targets := []MATarget{
		gatedTestTarget("keep", "confirm", maEnrichmentAddress),                     // survivor → enrich
		gatedTestTarget("forse-depri", "deprioritize", maEnrichmentAddress),         // survivor → enrich
		gatedTestTarget("forse-domain", "needs_domain_review", maEnrichmentAddress), // survivor → enrich
		gatedTestTarget("forse-nil", "", maEnrichmentAddress),                       // no verdict → forse → enrich
		gatedTestTarget("reject", "reject", maEnrichmentAddress),                    // salta → never charged
		gatedTestTarget("keep-done", "confirm", maEnrichmentAdvanced),               // already advanced → reuse
		gatedTestTarget("reject-done", "reject", maEnrichmentAdvanced),              // reject wins even if advanced
	}

	plan := planGatedEnrichment(targets)

	// Exactly the four fresh survivors pay Advanced this pass.
	if len(plan.ToEnrich) != 4 {
		t.Fatalf("ToEnrich = %d, want 4 (%v)", len(plan.ToEnrich), plan.ToEnrich)
	}
	for _, id := range []string{"keep", "forse-depri", "forse-domain", "forse-nil"} {
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
}

func TestGatedTargetBucket(t *testing.T) {
	cases := []struct {
		action string
		want   string
	}{
		{"confirm", maGatedBucketKeep},
		{"reject", maGatedBucketReject},
		{"deprioritize", maGatedBucketForse},
		{"needs_domain_review", maGatedBucketForse},
		{"needs_business_validation", maGatedBucketForse},
	}
	for _, c := range cases {
		got := gatedTargetBucket(gatedTestTarget("x", c.action, maEnrichmentAddress))
		if got != c.want {
			t.Errorf("gatedTargetBucket(%q) = %q, want %q", c.action, got, c.want)
		}
	}
	// No web validation → forse (recall-safe), never scarta.
	if got := gatedTargetBucket(MATarget{ID: "n"}); got != maGatedBucketForse {
		t.Errorf("gatedTargetBucket(nil validation) = %q, want %q", got, maGatedBucketForse)
	}
}
