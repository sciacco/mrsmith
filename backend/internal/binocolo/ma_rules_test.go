package binocolo

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestMAStrategyValidationNormalizesBoundaries(t *testing.T) {
	around := 5_000_000
	strategy, err := validateMAStrategy(MAStrategySpec{
		Title:             "Software Lombardia",
		SectorDescription: "aziende software gestionali",
		Provinces:         []string{"mi", "MI", " bg "},
		ActivityStatus:    "",
		TurnoverAround:    &around,
		AtecoCandidates: []MAAtecoCandidate{
			{Code: "62.01", Description: "Produzione software"},
			{Code: "6201", Description: "duplicato"},
		},
	})
	if err != nil {
		t.Fatalf("validate strategy: %v", err)
	}
	if strategy.ActivityStatus != "ATTIVA" {
		t.Fatalf("activity status = %q, want ATTIVA", strategy.ActivityStatus)
	}
	if got, want := strategy.Provinces, []string{"BG", "MI"}; !slices.Equal(got, want) {
		t.Fatalf("provinces = %#v, want %#v", got, want)
	}
	if strategy.TurnoverMin == nil || *strategy.TurnoverMin != 3_500_000 {
		t.Fatalf("turnover min = %v, want 3500000", strategy.TurnoverMin)
	}
	if strategy.TurnoverMax == nil || *strategy.TurnoverMax != 6_500_000 {
		t.Fatalf("turnover max = %v, want 6500000", strategy.TurnoverMax)
	}
	if len(strategy.AtecoCandidates) != 1 || strategy.AtecoCandidates[0].Code != "62.01" {
		t.Fatalf("ateco candidates = %#v, want one normalized candidate", strategy.AtecoCandidates)
	}
	if strategy.SearchLimit != maDefaultSearchLimit {
		t.Fatalf("search limit = %d, want %d", strategy.SearchLimit, maDefaultSearchLimit)
	}
}

func TestMAStrategyValidationRejectsInvalidRanges(t *testing.T) {
	min := 10
	max := 5
	_, err := validateMAStrategy(MAStrategySpec{
		SectorDescription: "impianti",
		ActivityStatus:    "ATTIVA",
		TurnoverMin:       &min,
		TurnoverMax:       &max,
	})
	if err == nil {
		t.Fatal("expected invalid turnover range")
	}
}

func TestMAEstimateStrategyFallbackThreshold(t *testing.T) {
	if got := chooseSelectedStrategy(9, true); got != maStrategyTypeExpanded {
		t.Fatalf("strategy for weak ATECO estimate = %q, want expanded", got)
	}
	if got := chooseSelectedStrategy(10, true); got != maStrategyTypeATECO {
		t.Fatalf("strategy for viable ATECO estimate = %q, want ateco", got)
	}
	if got := chooseSelectedStrategy(25, false); got != maStrategyTypeExpanded {
		t.Fatalf("strategy without ATECO candidates = %q, want expanded", got)
	}
}

func TestMAEstimateStrategySelectionSkipsTooBroadSurfaces(t *testing.T) {
	estimates := []MAEstimate{
		{StrategyType: maStrategyTypeATECO, EstimatedCount: 2000, SurfaceStatus: maEstimateSurfaceTooBroad},
		{StrategyType: maStrategyTypeExpanded, EstimatedCount: 80, SurfaceStatus: maEstimateSurfaceExact},
	}
	if got := chooseSelectedStrategyFromEstimates(estimates, true); got != maStrategyTypeExpanded {
		t.Fatalf("strategy with broad ATECO = %q, want expanded", got)
	}

	estimates = []MAEstimate{
		{StrategyType: maStrategyTypeATECO, EstimatedCount: 9, SurfaceStatus: maEstimateSurfaceExact},
		{StrategyType: maStrategyTypeExpanded, EstimatedCount: 2000, SurfaceStatus: maEstimateSurfaceTooBroad},
	}
	if got := chooseSelectedStrategyFromEstimates(estimates, true); got != maStrategyTypeATECO {
		t.Fatalf("strategy with broad expanded fallback = %q, want ateco", got)
	}

	estimates = []MAEstimate{
		{StrategyType: maStrategyTypeATECO, EstimatedCount: 2000, SurfaceStatus: maEstimateSurfaceTooBroad},
		{StrategyType: maStrategyTypeExpanded, EstimatedCount: 2000, SurfaceStatus: maEstimateSurfaceTooBroad},
	}
	if got := chooseSelectedStrategyFromEstimates(estimates, true); got != "" {
		t.Fatalf("strategy with only broad options = %q, want empty selection", got)
	}
}

// TestMAScoringThesisInversion locks the core promise of scoring v2 on two real
// OpenAPI.it payloads: BFInformatica (strong growth, productive, solid) and
// Prometeo (flat, single 60yo owner, mature, eroded equity). Same ATECO/size/
// province, so the ranking inverts purely with the acquisition thesis.
func TestMAScoringThesisInversion(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	run := func(thesis string) (int, int) {
		strategy := thesisScoringStrategy(t, thesis)
		targets := scoreMATargetsV2([]MATarget{bfInformaticaTarget(), prometeoTarget()}, strategy, now)
		return scoreByCompany(targets, "BFINFORMATICA"), scoreByCompany(targets, "PROMETEO")
	}

	bfiGrowth, proGrowth := run(maThesisGrowth)
	if bfiGrowth <= proGrowth {
		t.Fatalf("crescita: atteso BFInformatica (%d) > Prometeo (%d)", bfiGrowth, proGrowth)
	}

	bfiSucc, proSucc := run(maThesisSuccession)
	if proSucc <= bfiSucc {
		t.Fatalf("successione: atteso Prometeo (%d) > BFInformatica (%d)", proSucc, bfiSucc)
	}
}

func thesisScoringStrategy(t *testing.T, thesis string) MAStrategySpec {
	t.Helper()
	around := 1_200_000
	strategy, err := validateMAStrategy(MAStrategySpec{
		SectorDescription: "servizi informatici",
		Provinces:         []string{"TV"},
		ActivityStatus:    "ATTIVA",
		TurnoverAround:    &around,
		AtecoCandidates:   []MAAtecoCandidate{{Code: "6290", Description: "Servizi informatici"}},
		Keywords:          []string{"informatica"},
		Thesis:            thesis,
	})
	if err != nil {
		t.Fatalf("validate strategy: %v", err)
	}
	return strategy
}

func scoreByCompany(targets []MATarget, name string) int {
	for _, target := range targets {
		if strings.Contains(target.CompanyName, name) {
			return target.Score
		}
	}
	return -1
}

func bfInformaticaTarget() MATarget {
	return MATarget{
		CompanyName:      "BFINFORMATICA SRL",
		AtecoCode:        "629009",
		AtecoDescription: "Altre attivita' dei servizi connessi alle tecnologie dell'informatica n.c.a.",
		VendorPayload:    json.RawMessage(bfInformaticaPayload),
	}
}

func prometeoTarget() MATarget {
	return MATarget{
		CompanyName:      "PROMETEO S.R.L.",
		AtecoCode:        "629009",
		AtecoDescription: "Altre attivita' dei servizi connessi alle tecnologie dell'informatica n.c.a.",
		VendorPayload:    json.RawMessage(prometeoPayload),
	}
}

const bfInformaticaPayload = `{
  "companyName": "BFINFORMATICA SRL",
  "vatCode": "04682240264",
  "taxCode": "04682240264",
  "activityStatus": "ATTIVA",
  "taxCodeCeased": false,
  "startDate": "2014-07-29",
  "registrationDate": "2014-07-28",
  "detailedLegalForm": {"code": "SR", "description": "SOCIETA' A RESPONSABILITA' LIMITATA"},
  "atecoClassification": {"ateco": {"code": "629009"}},
  "shareHolders": [
    {"name": "BARBARA", "surname": "FRANCESCHINI", "taxCode": "FRNBBR76C55L378C", "percentShare": 80},
    {"name": "ALESSIA", "surname": "MASE'", "taxCode": "MSALSS02M51L407L", "percentShare": 20}
  ],
  "balanceSheets": {
    "all": [
      {"year": 2022, "turnover": 831095, "netWorth": 52133, "employees": 9},
      {"year": 2023, "turnover": 927803, "netWorth": 43559, "employees": 8, "totalAssets": 617237},
      {"year": 2024, "turnover": 1211015, "netWorth": 73049, "employees": 9, "totalAssets": 722368}
    ],
    "last": {"year": 2024, "turnover": 1211015, "netWorth": 73049, "employees": 9, "totalAssets": 722368}
  }
}`

const prometeoPayload = `{
  "companyName": "PROMETEO S.R.L.",
  "vatCode": "02062450263",
  "taxCode": "02062450263",
  "activityStatus": "ATTIVA",
  "taxCodeCeased": false,
  "startDate": "1988-06-30",
  "registrationDate": "1988-04-21",
  "detailedLegalForm": {"code": "SR", "description": "SOCIETA' A RESPONSABILITA' LIMITATA"},
  "atecoClassification": {"ateco": {"code": "629009"}},
  "shareHolders": [
    {"name": "GIAMPAOLO", "surname": "LIONELLO", "taxCode": "LNLGPL65T16D157M", "percentShare": 100}
  ],
  "balanceSheets": {
    "all": [
      {"year": 2023, "turnover": 1226650, "netWorth": 44696, "employees": 19, "totalAssets": 2272968},
      {"year": 2024, "turnover": 1217690, "netWorth": 40432, "employees": 15, "totalAssets": 2245815},
      {"year": 2025, "turnover": 1234085, "netWorth": 42000, "employees": 16, "totalAssets": 2236197}
    ],
    "last": {"year": 2025, "turnover": 1234085, "netWorth": 42000, "employees": 16, "totalAssets": 2236197}
  }
}`

func TestMADedupeTargetsAcrossAtecoSearches(t *testing.T) {
	first := MATarget{CompanyName: "ACME S.r.l.", VATCode: "12345678901", AtecoCode: "6201"}
	second := MATarget{CompanyName: "ACME S.r.l.", VATCode: "12345678901", AtecoCode: "6311"}
	third := MATarget{CompanyName: "Beta S.r.l.", VATCode: "10987654321", AtecoCode: "6201"}

	got := dedupeMATargets([]MATarget{first, second, third})
	if len(got) != 2 {
		t.Fatalf("deduped targets = %d, want 2: %#v", len(got), got)
	}
	if got[0].AtecoCode != "6201" {
		t.Fatalf("first target should be kept, got %#v", got[0])
	}
}

func TestMAExportRows(t *testing.T) {
	turnover := 1_500_000
	rows := maExportRows([]MATarget{{
		CompanyName:     "ACME S.r.l.",
		VATCode:         "12345678901",
		Province:        "MI",
		Turnover:        &turnover,
		Score:           82,
		MatchState:      maMatchStatePartial,
		MissingCriteria: []string{"eta soci"},
		Rationale:       "criteri mancanti o parziali: eta soci",
	}})
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want header + one row", len(rows))
	}
	if rows[0][0] != "Azienda" {
		t.Fatalf("header = %#v", rows[0])
	}
	if rows[1][10] != "match parziale" {
		t.Fatalf("match label = %#v, want match parziale", rows[1][10])
	}
	if rows[1][11] != "" {
		t.Fatalf("preferito = %#v, want empty (no rating)", rows[1][11])
	}
	// Deep-analysis columns (12..18) are blank without a deep record.
	if rows[1][12] != "" {
		t.Fatalf("analisi = %#v, want empty (no deep)", rows[1][12])
	}
	if rows[1][19] != "eta soci" {
		t.Fatalf("missing criteria = %#v, want eta soci", rows[1][19])
	}
}

