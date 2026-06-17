package binocolo

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

// TestInSectorPerimeter locks the sector gate: a target is in-perimeter when its
// ATECO division is declared AND it is not under an excluded subtree. The killer
// case is the shared group 63.10 — excluding elaborazione dati (63.10.21) must not
// also drop hosting (63.10.10). A sector-less strategy or a target without an ATECO
// code is never gated on sector.
func TestInSectorPerimeter(t *testing.T) {
	cases := []struct {
		name      string
		divisions []string
		excluded  []string
		ateco     string
		want      bool
	}{
		{"in declared division", []string{"62", "63"}, nil, "62.09.01", true},
		{"other division out", []string{"62", "63"}, nil, "68.31", false},
		{"excluded leaf out", []string{"62", "63"}, []string{"63.10.21"}, "63.10.21", false},
		{"sibling of excluded stays in", []string{"62", "63"}, []string{"63.10.21"}, "63.10.10", true},
		{"excluded subtree by prefix", []string{"62", "63"}, []string{"63.10.2"}, "63.10.21", false},
		{"no divisions disables the gate", nil, []string{"63.10.21"}, "68.31", true},
		{"missing ateco code passes", []string{"62"}, nil, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			strategy := MAStrategySpec{SectorDivisions: tc.divisions, ExcludedAteco: tc.excluded}
			if got := inSectorPerimeter(strategy, tc.ateco); got != tc.want {
				t.Fatalf("inSectorPerimeter(ateco=%q, div=%v, excl=%v) = %v, want %v", tc.ateco, tc.divisions, tc.excluded, got, tc.want)
			}
		})
	}
}

// TestAtecoDivisions locks the 2-digit division derivation used to widen the
// expanded net and to define the gate perimeter: distinct, sorted, dot-insensitive,
// dropping codes with fewer than two leading digits.
func TestAtecoDivisions(t *testing.T) {
	got := atecoDivisions([]MAAtecoCandidate{
		{Code: "62.20.10"}, {Code: "62.09"}, {Code: "63.10.10"}, {Code: "63.11"},
	})
	if want := []string{"62", "63"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("atecoDivisions = %v, want %v", got, want)
	}
	if got := atecoDivisions([]MAAtecoCandidate{{Code: "6"}, {Code: ""}}); len(got) != 0 {
		t.Fatalf("atecoDivisions(short/empty) = %v, want empty", got)
	}
}

// TestMeasureKeywordMatch locks the D2 hygiene: a single shared filler word
// ("servizi" is a stopword) no longer claims sector adherence — a real-estate firm
// scores the floor; one significant token is a partial match; two or more is full.
func TestMeasureKeywordMatch(t *testing.T) {
	// SectorDescription deliberately carries the exclusion clause inline, as the LLM
	// sometimes leaves it: positiveSectorText must strip it so the excluded activity
	// cannot match the sector on its own words.
	strategy := MAStrategySpec{
		SectorDescription: "servizi IT gestiti, assistenza sistemistica, hosting; esclusi servizi di elaborazione dati contabili",
		Keywords:          []string{"managed services", "sistemistica", "hosting"},
	}
	mk := func(desc, name string) maSignalContext {
		return maSignalContext{target: MATarget{AtecoDescription: desc, CompanyName: name}, strategy: strategy}
	}

	// Shares only the filler word "servizi" → floor, not a match.
	if got := measureKeywordMatch(mk("Attività di servizi di intermediazione immobiliare", "ARTEKASA NOVARA SRL")); got.Score != 0.2 {
		t.Fatalf("immobiliare keyword score = %v, want 0.2 (filler 'servizi' must not count)", got.Score)
	}
	// The excluded activity itself ("Elaborazione dati contabili") must NOT match the
	// sector — its words live only in the stripped exclusion clause.
	if got := measureKeywordMatch(mk("Elaborazione dati contabili", "SEA SERVIZI SRL")); got.Score != 0.2 {
		t.Fatalf("excluded-activity keyword score = %v, want 0.2 (exclusion words must be stripped)", got.Score)
	}
	// Exactly one significant token ("hosting") → partial.
	if got := measureKeywordMatch(mk("Gestione hosting e datacenter", "X SRL")); got.Score != 0.6 {
		t.Fatalf("single-token keyword score = %v, want 0.6", got.Score)
	}
	// Two+ significant tokens (assistenza, sistemistica, hosting) → full match.
	if got := measureKeywordMatch(mk("Assistenza sistemistica e gestione infrastrutture, hosting", "ACME SRL")); got.Score != 1.0 {
		t.Fatalf("multi-token keyword score = %v, want 1.0", got.Score)
	}
}

// TestPositiveSectorText locks the exclusion-clause stripping: the positive
// perimeter is kept, the trailing "esclusi/tranne …" clause is dropped, and a
// description with no exclusion marker is returned unchanged.
func TestPositiveSectorText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"servizi IT gestiti, hosting; esclusi servizi di elaborazione dati contabili", "servizi IT gestiti, hosting"},
		{"consulenza informatica tranne elaborazione dati", "consulenza informatica"},
		{"servizi IT gestiti e hosting", "servizi IT gestiti e hosting"},
	}
	for _, tc := range cases {
		if got := positiveSectorText(tc.in); got != tc.want {
			t.Fatalf("positiveSectorText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestAggregateExpandedEstimates locks the estimate collapse: the per-code expanded
// probes become one row per province (summed count, recomputed cost, empty code),
// while ATECO rows pass through untouched.
func TestAggregateExpandedEstimates(t *testing.T) {
	strategy := MAStrategySpec{SectorDivisions: []string{"62", "63"}}
	estimates := []MAEstimate{
		{StrategyType: maStrategyTypeATECO, Province: "NO", AtecoCode: "62.20.1", EstimatedCount: 7, ProbeCount: 1, ExecutionLimit: 100},
		{StrategyType: maStrategyTypeExpanded, Province: "NO", AtecoCode: "62.01", EstimatedCount: 12, ProbeCount: 1, ExecutionLimit: 100},
		{StrategyType: maStrategyTypeExpanded, Province: "NO", AtecoCode: "62.09", EstimatedCount: 20, ProbeCount: 1, ExecutionLimit: 100},
		{StrategyType: maStrategyTypeExpanded, Province: "NO", AtecoCode: "63.11", EstimatedCount: 8, ProbeCount: 1, ExecutionLimit: 100},
	}
	out := aggregateExpandedEstimates(estimates, strategy, 0.10)

	var ateco, expanded []MAEstimate
	for _, est := range out {
		if est.StrategyType == maStrategyTypeExpanded {
			expanded = append(expanded, est)
		} else {
			ateco = append(ateco, est)
		}
	}
	if len(ateco) != 1 || ateco[0].EstimatedCount != 7 || ateco[0].AtecoCode != "62.20.1" {
		t.Fatalf("ATECO row altered: %+v", ateco)
	}
	if len(expanded) != 1 {
		t.Fatalf("expected 1 aggregated expanded row, got %d: %+v", len(expanded), expanded)
	}
	if expanded[0].EstimatedCount != 40 {
		t.Fatalf("aggregated expanded count = %d, want 40 (12+20+8)", expanded[0].EstimatedCount)
	}
	if expanded[0].ProbeCount != 3 {
		t.Fatalf("aggregated probe count = %d, want 3", expanded[0].ProbeCount)
	}
	if expanded[0].AtecoCode != "" {
		t.Fatalf("aggregated expanded ateco code = %q, want empty", expanded[0].AtecoCode)
	}
	if math.Abs(expanded[0].EstimatedCost-4.0) > 1e-9 {
		t.Fatalf("aggregated expanded cost = %v, want 4.0 (40*0.10)", expanded[0].EstimatedCost)
	}
}

// TestScoreMATargetsSectorGate is the end-to-end check: two healthy targets that
// differ only in ATECO division — one inside the declared sector (62), one outside
// (68) — get scored, and the off-division one is demoted to fuori_criterio (hidden)
// while the in-sector one is not.
func TestScoreMATargetsSectorGate(t *testing.T) {
	strategy := thesisScoringStrategy(t, maThesisGeneric)
	strategy.SectorDivisions = []string{"62"}
	targets := []MATarget{
		{CompanyName: "IN SECTOR SRL", AtecoCode: "62.09.09", VendorPayload: json.RawMessage(personOwnedPayload)},
		{CompanyName: "OFF SECTOR SRL", AtecoCode: "68.31.00", VendorPayload: json.RawMessage(personOwnedPayload)},
	}
	scored := scoreMATargetsV2(targets, strategy, maScoringParams{ThesisFitHoldingFactor: 1.0}, thesisFitTestNow)

	in := targetByName(scored, "IN SECTOR")
	off := targetByName(scored, "OFF SECTOR")
	if in == nil || off == nil {
		t.Fatalf("missing target in result: %+v", scored)
	}
	if in.MatchState == maMatchStateOutside {
		t.Fatalf("in-sector target (62) should not be fuori_criterio, got %q", in.MatchState)
	}
	if off.MatchState != maMatchStateOutside {
		t.Fatalf("off-sector target (68) should be fuori_criterio, got %q", off.MatchState)
	}
}
