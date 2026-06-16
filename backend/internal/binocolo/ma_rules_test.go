package binocolo

import (
	"encoding/json"
	"slices"
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

func TestMATargetScoringMatchPartialAndOutsideRules(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	strategy := validScoringStrategy()

	match := scoreMATarget(validScoringTarget(t), strategy, now)
	if match.MatchState != maMatchStateMatch {
		t.Fatalf("match state = %q, want match; evidence=%#v", match.MatchState, match.Evidence)
	}
	if match.Score != 100 {
		t.Fatalf("score = %d, want 100", match.Score)
	}

	partial := validScoringTarget(t)
	partial.Turnover = nil
	partial = scoreMATarget(partial, strategy, now)
	if partial.MatchState != maMatchStatePartial {
		t.Fatalf("missing turnover state = %q, want partial", partial.MatchState)
	}
	if !slices.Contains(partial.MissingCriteria, "fatturato") {
		t.Fatalf("missing criteria = %#v, want fatturato", partial.MissingCriteria)
	}

	outside := validScoringTarget(t)
	lowTurnover := 400_000
	outside.Turnover = &lowTurnover
	outside = scoreMATarget(outside, strategy, now)
	if outside.MatchState != maMatchStateOutside {
		t.Fatalf("outside turnover state = %q, want outside", outside.MatchState)
	}
}

func TestMADynamicCriteriaUseVendorPaths(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	strategy := validScoringStrategy()
	target := validScoringTarget(t)
	target.VendorPayload = mustVendorPayload(t, []map[string]any{
		{"taxCode": "RSSMRA70A01H501U", "percentShare": 65},
	})
	scored := scoreMATarget(target, strategy, now)
	if statusForEvidence(scored.Evidence, "dynamic_quota_minima") != maEvidenceMatch {
		t.Fatalf("dynamic evidence = %#v, want match", scored.Evidence)
	}
	target.VendorPayload = mustVendorPayload(t, []map[string]any{
		{"taxCode": "RSSMRA70A01H501U", "percentShare": 20},
	})
	scored = scoreMATarget(target, strategy, now)
	if statusForEvidence(scored.Evidence, "dynamic_quota_minima") != maEvidenceOutside {
		t.Fatalf("dynamic evidence = %#v, want outside", scored.Evidence)
	}
}

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
	if rows[1][11] != "eta soci" {
		t.Fatalf("missing criteria = %#v, want eta soci", rows[1][11])
	}
}

func validScoringStrategy() MAStrategySpec {
	minTurnover := 1_000_000
	maxTurnover := 2_000_000
	strategy, err := validateMAStrategy(MAStrategySpec{
		SectorDescription: "software gestionale",
		Provinces:         []string{"MI"},
		ActivityStatus:    "ATTIVA",
		TurnoverMin:       &minTurnover,
		TurnoverMax:       &maxTurnover,
		AtecoCandidates: []MAAtecoCandidate{
			{Code: "6201", Description: "Produzione software"},
		},
		Keywords: []string{"software"},
		ScoringCriteria: []MAScoringCriterion{
			{
				ID:     "quota_minima",
				Label:  "Quota minima verificabile",
				Weight: 30,
				Evaluation: MAScoringEvaluation{
					SourcePath: "shareHolders.percentShare",
					Operator:   "gte",
					Value:      60,
					Match:      "any",
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	return strategy
}

func validScoringTarget(t *testing.T) MATarget {
	t.Helper()
	turnover := 1_500_000
	return MATarget{
		CompanyName:      "ACME S.r.l.",
		VATCode:          "12345678901",
		Province:         "MI",
		ActivityStatus:   "ATTIVA",
		Turnover:         &turnover,
		AtecoCode:        "620100",
		AtecoDescription: "Produzione di software non connesso all'edizione",
		VendorPayload: mustVendorPayload(t, []map[string]any{
			{"taxCode": "RSSMRA70A01H501U", "percentShare": 65},
			{"taxCode": "VRDLGI65M01H501Q", "percentShare": 35},
		}),
	}
}

func mustVendorPayload(t *testing.T, shareholders []map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"shareHolders": shareholders})
	if err != nil {
		t.Fatalf("marshal vendor payload: %v", err)
	}
	return raw
}

func statusForEvidence(evidence []MATargetEvidence, criterion string) string {
	for _, item := range evidence {
		if item.Criterion == criterion {
			return item.Status
		}
	}
	return ""
}
