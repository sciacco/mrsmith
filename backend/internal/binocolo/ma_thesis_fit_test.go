package binocolo

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"
)

var thesisFitTestNow = time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

// personTaxCodeAge builds a syntactically-parseable Italian tax code whose
// birth-year digits decode to `age` at `now` (born Jan 1, so the age is exact
// regardless of the test month). ageFromItalianTaxCode reads only the year/month/
// day positions and not the checksum, so the trailing belfiore+check chars are
// filler — same convention as the distressedPayload fixture.
func personTaxCodeAge(age int, now time.Time) string {
	yy := ((now.Year()-age)%100 + 100) % 100
	return fmt.Sprintf("RSSMRA%02dA01H501X", yy)
}

func hasAdjustment(target *MATarget, code string) bool {
	if target == nil {
		return false
	}
	for _, adj := range target.Adjustments {
		if adj.Code == code {
			return true
		}
	}
	return false
}

// TestComputeThesisFit locks the owner-TYPE multiplier: under succession a clear
// corporate (holding) controller is demoted, a natural-person controller is
// neutral, ambiguity is neutral, and other theses never penalize. A misconfigured
// factor clamps to the default (never a knockout or a bonus).
func TestComputeThesisFit(t *testing.T) {
	const factor = 0.4
	person := maShareholder{Name: "MARIO", Surname: "ROSSI", TaxCode: personTaxCodeAge(65, thesisFitTestNow), PercentShare: 100}
	holding := maShareholder{CompanyName: "ALAB HOLDING SRL", TaxCode: "02530360037", PercentShare: 100}
	holdingMajority := maShareholder{CompanyName: "ALAB HOLDING SRL", TaxCode: "02530360037", PercentShare: 70}
	personMinority := maShareholder{Name: "ANNA", Surname: "BIANCHI", TaxCode: personTaxCodeAge(45, thesisFitTestNow), PercentShare: 30}

	cases := []struct {
		name    string
		thesis  string
		holders []maShareholder
		want    float64
	}{
		{"sole holding demoted", maThesisSuccession, []maShareholder{holding}, factor},
		{"holding majority over minority person demoted", maThesisSuccession, []maShareholder{holdingMajority, personMinority}, factor},
		{"person majority neutral", maThesisSuccession, []maShareholder{person}, 1.0},
		{"no holders neutral", maThesisSuccession, nil, 1.0},
		{"non-succession thesis neutral", maThesisConsolidation, []maShareholder{holding}, 1.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := maSignalContext{thesis: tc.thesis, holders: tc.holders, now: thesisFitTestNow}
			if got := computeThesisFit(c, factor); math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("computeThesisFit = %v, want %v", got, tc.want)
			}
		})
	}

	wantDefault := 1 - maThesisFitHoldingHaircutPctDefault/100
	c := maSignalContext{thesis: maThesisSuccession, holders: []maShareholder{holding}, now: thesisFitTestNow}
	for _, bad := range []float64{0, -0.5, 1.5} {
		if got := computeThesisFit(c, bad); math.Abs(got-wantDefault) > 1e-9 {
			t.Fatalf("computeThesisFit(factor=%v) = %v, want clamped default %v", bad, got, wantDefault)
		}
	}
}

// TestMeasureSuccessionOwner locks the owner-AGE signal: applicable only for a
// natural-person controller, graded by age, peaking above the threshold and
// tapering below it (never to zero), not applicable for a holding, and honoring
// the strategy-supplied threshold over the default.
func TestMeasureSuccessionOwner(t *testing.T) {
	person := func(age int) []maShareholder {
		return []maShareholder{{Name: "MARIO", Surname: "ROSSI", TaxCode: personTaxCodeAge(age, thesisFitTestNow), PercentShare: 100}}
	}
	holding := []maShareholder{{CompanyName: "ALAB HOLDING SRL", TaxCode: "02530360037", PercentShare: 100}}

	// Default threshold (maSuccessionDefaultMinAge = 60).
	if got := measureSuccessionOwner(maSignalContext{holders: person(85), now: thesisFitTestNow}); !got.Applicable || math.Abs(got.Score-1.0) > 1e-9 {
		t.Fatalf("age 85: %+v, want applicable score 1.0", got)
	}
	if got := measureSuccessionOwner(maSignalContext{holders: person(60), now: thesisFitTestNow}); !got.Applicable || math.Abs(got.Score-0.7) > 1e-9 {
		t.Fatalf("age 60 (at threshold): %+v, want applicable score 0.7", got)
	}
	if got := measureSuccessionOwner(maSignalContext{holders: person(50), now: thesisFitTestNow}); !got.Applicable || got.Score <= 0.3 || got.Score >= 0.6 {
		t.Fatalf("age 50 (below threshold): %+v, want applicable score in (0.3, 0.6)", got)
	}
	if got := measureSuccessionOwner(maSignalContext{holders: holding, now: thesisFitTestNow}); got.Applicable {
		t.Fatalf("holding-controlled: %+v, want not applicable", got)
	}

	// Threshold from strategy: a 55yo is below the default (60) but above an
	// explicit 50, so the explicit threshold must score higher.
	low := 50
	withStrategy := measureSuccessionOwner(maSignalContext{holders: person(55), strategy: MAStrategySpec{SuccessionMinOwnerAge: &low}, now: thesisFitTestNow})
	withDefault := measureSuccessionOwner(maSignalContext{holders: person(55), now: thesisFitTestNow})
	if !withStrategy.Applicable || !withDefault.Applicable {
		t.Fatalf("age 55 should be applicable both ways: strategy=%+v default=%+v", withStrategy, withDefault)
	}
	if withStrategy.Score <= withDefault.Score {
		t.Fatalf("explicit threshold 50 (%.3f) should outscore default 60 (%.3f) for a 55yo", withStrategy.Score, withDefault.Score)
	}
}

// TestMAScoringSuccessionHoldingDemotion is the end-to-end wiring check: two
// targets identical on every fit signal (ATECO, size, legal form, financials),
// differing only in ownership — a 100% holding vs a 70yo individual. Under
// succession the holding is demoted below the person and carries the thesis-fit
// malus; under consolidation the thesis gate removes the malus entirely.
func TestMAScoringSuccessionHoldingDemotion(t *testing.T) {
	mkTargets := func() []MATarget {
		return []MATarget{
			{CompanyName: "ALAB SRL", AtecoCode: "629009", VendorPayload: json.RawMessage(holdingControlledPayload)},
			{CompanyName: "FONTANA SRL", AtecoCode: "629009", VendorPayload: json.RawMessage(personOwnedPayload)},
		}
	}
	params := maScoringParams{ThesisFitHoldingFactor: 0.4}

	succ := scoreMATargetsV2(mkTargets(), thesisScoringStrategy(t, maThesisSuccession), params, thesisFitTestNow)
	holding := targetByName(succ, "ALAB")
	person := targetByName(succ, "FONTANA")
	if holding == nil || person == nil {
		t.Fatalf("missing target in result: %+v", succ)
	}
	if person.Score <= holding.Score {
		t.Fatalf("successione: atteso FONTANA (%d) > ALAB holding (%d)", person.Score, holding.Score)
	}
	if !hasAdjustment(holding, "controllo_holding") {
		t.Fatalf("ALAB holding should carry a controllo_holding malus, got %+v", holding.Adjustments)
	}
	if hasAdjustment(person, "controllo_holding") {
		t.Fatalf("FONTANA person-owned should have no holding malus, got %+v", person.Adjustments)
	}

	cons := scoreMATargetsV2(mkTargets(), thesisScoringStrategy(t, maThesisConsolidation), params, thesisFitTestNow)
	h := targetByName(cons, "ALAB")
	if h == nil {
		t.Fatalf("consolidamento: ALAB missing from result")
	}
	if hasAdjustment(h, "controllo_holding") {
		t.Fatalf("consolidamento: ALAB should have no holding malus (thesis gate), got %+v", h.Adjustments)
	}
}

// Two healthy targets identical on every fit signal; only the controlling owner
// differs (100% holding vs 70yo individual — taxCode RSSMRA56A01H501X decodes to
// born 1956 = age 70 at the fixed test date).
const holdingControlledPayload = `{
  "companyName": "ALAB SRL",
  "vatCode": "02530360037",
  "taxCode": "02530360037",
  "activityStatus": "ATTIVA",
  "taxCodeCeased": false,
  "startDate": "2001-03-10",
  "registrationDate": "2001-03-10",
  "detailedLegalForm": {"code": "SR", "description": "SOCIETA' A RESPONSABILITA' LIMITATA"},
  "atecoClassification": {"ateco": {"code": "629009"}},
  "shareHolders": [
    {"companyName": "ALAB HOLDING SRL", "taxCode": "02530360037", "percentShare": 100}
  ],
  "balanceSheets": {
    "all": [
      {"year": 2023, "turnover": 1180000, "netWorth": 420000, "employees": 12, "totalAssets": 1500000},
      {"year": 2024, "turnover": 1210000, "netWorth": 450000, "employees": 12, "totalAssets": 1560000}
    ],
    "last": {"year": 2024, "turnover": 1210000, "netWorth": 450000, "employees": 12, "totalAssets": 1560000}
  }
}`

const personOwnedPayload = `{
  "companyName": "FONTANA SRL",
  "vatCode": "02062450299",
  "taxCode": "02062450299",
  "activityStatus": "ATTIVA",
  "taxCodeCeased": false,
  "startDate": "2001-03-10",
  "registrationDate": "2001-03-10",
  "detailedLegalForm": {"code": "SR", "description": "SOCIETA' A RESPONSABILITA' LIMITATA"},
  "atecoClassification": {"ateco": {"code": "629009"}},
  "shareHolders": [
    {"name": "GIULIO", "surname": "FONTANA", "taxCode": "RSSMRA56A01H501X", "percentShare": 100}
  ],
  "balanceSheets": {
    "all": [
      {"year": 2023, "turnover": 1180000, "netWorth": 420000, "employees": 12, "totalAssets": 1500000},
      {"year": 2024, "turnover": 1210000, "netWorth": 450000, "employees": 12, "totalAssets": 1560000}
    ],
    "last": {"year": 2024, "turnover": 1210000, "netWorth": 450000, "employees": 12, "totalAssets": 1560000}
  }
}`
