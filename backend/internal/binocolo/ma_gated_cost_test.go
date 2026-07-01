package binocolo

import (
	"math"
	"testing"
)

func approxEUR(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 0.005 {
		t.Errorf("%s = %.4f, want %.4f", label, got, want)
	}
}

// The gated-search estimate is the only cost guardrail under "tutto automatico"
// (plus the surface cap), so its two-part formula is pinned by a test.
func TestProjectGatedSearchCostDefaultPricing(t *testing.T) {
	pricing := maPricingFromParameters(nil) // compiled defaults incl. the migration-074 costs

	// Sanity: the new gated levers are populated from the compiled defaults.
	approxEUR(t, pricing.CostAddress, 0.01, "CostAddress")
	approxEUR(t, pricing.CostScrapePage, 0.001, "CostScrapePage")
	approxEUR(t, pricing.CostSearch, 0.001, "CostSearch")
	approxEUR(t, pricing.SurvivorRate, 0.35, "SurvivorRate")

	p := projectGatedSearchCost(100, pricing)

	// gate per company = 0.01 + 8*0.001 + 2*0.001 = 0.02
	approxEUR(t, p.GatePerCompanyEUR, 0.02, "GatePerCompanyEUR")
	// gate cost = 100 * 0.02 = 2.00 (certain, whole surface)
	approxEUR(t, p.GateCostEUR, 2.00, "GateCostEUR")
	// expected advanced = 100 * 0.35 * 0.10 = 3.50
	approxEUR(t, p.ExpectedAdvancedEUR, 3.50, "ExpectedAdvancedEUR")
	// total mid = 5.50; band = [4.50 @25%, 7.00 @50%]
	approxEUR(t, p.TotalEUR, 5.50, "TotalEUR")
	approxEUR(t, p.TotalLowEUR, 4.50, "TotalLowEUR")
	approxEUR(t, p.TotalHighEUR, 7.00, "TotalHighEUR")

	// The gated total must beat Advanced-on-everyone (100 * 0.10 = 10.00) at the
	// expected survival — the whole premise of the funnel.
	if p.TotalEUR >= 100*pricing.CostAdvanced {
		t.Errorf("gated total %.2f not cheaper than advanced-on-all %.2f", p.TotalEUR, 100*pricing.CostAdvanced)
	}
	if p.SurfaceCount != 100 {
		t.Errorf("SurfaceCount = %d, want 100", p.SurfaceCount)
	}
}

func TestProjectGatedSearchCostZeroSurface(t *testing.T) {
	p := projectGatedSearchCost(0, maPricingFromParameters(nil))
	approxEUR(t, p.GateCostEUR, 0, "GateCostEUR")
	approxEUR(t, p.ExpectedAdvancedEUR, 0, "ExpectedAdvancedEUR")
	approxEUR(t, p.TotalEUR, 0, "TotalEUR")
	approxEUR(t, p.TotalHighEUR, 0, "TotalHighEUR")
}

func TestSurvivorRateParamOverrideAndGuard(t *testing.T) {
	// A valid override is honoured.
	got := maPricingFromParameters([]MAParameter{{Key: "survivor_rate_default", Value: "0.5"}})
	approxEUR(t, got.SurvivorRate, 0.5, "SurvivorRate override")

	// Out-of-range (>1) is rejected: the default holds, never a nonsense projection.
	guarded := maPricingFromParameters([]MAParameter{{Key: "survivor_rate_default", Value: "1.5"}})
	approxEUR(t, guarded.SurvivorRate, maSurvivorRateDefault, "SurvivorRate guard")
}
