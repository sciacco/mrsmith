package binocolo

import (
	"fmt"
	"sort"
	"testing"
)

// TestAllocateExecutionLimit locks the floor+proportional budget allocation:
// when the combos hold more companies than the fetch limit allows, every named
// combo keeps a guaranteed minimum and the remainder is split proportionally to
// the real population — never exhausting the budget on the first province.
func TestAllocateExecutionLimit(t *testing.T) {
	cases := []struct {
		name   string
		counts []int
		limit  int
		want   []int
	}{
		{name: "no budget", counts: []int{50, 50}, limit: 0, want: []int{0, 0}},
		{name: "fits under limit fetches all", counts: []int{3, 2}, limit: 10, want: []int{3, 2}},
		{name: "proportional split", counts: []int{90, 10}, limit: 10, want: []int{8, 2}},
		{name: "equal split", counts: []int{100, 100}, limit: 10, want: []int{5, 5}},
		{name: "floor protects small combos", counts: []int{3, 3, 3}, limit: 5, want: []int{2, 2, 1}},
		{name: "limit below combo count", counts: []int{1, 1, 1, 1}, limit: 2, want: []int{1, 1, 0, 0}},
		{name: "empty combos ignored", counts: []int{0, 50, 0}, limit: 10, want: []int{0, 10, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := allocateExecutionLimit(tc.counts, tc.limit)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("allocate(%v, %d) = %v, want %v", tc.counts, tc.limit, got, tc.want)
			}
			// Invariants that must hold for any allocation.
			total, sum := 0, 0
			for index, count := range tc.counts {
				if got[index] < 0 || got[index] > count {
					t.Fatalf("slot %d = %d out of [0,%d]", index, got[index], count)
				}
				total += count
				sum += got[index]
			}
			wantSum := total
			if tc.limit < wantSum {
				wantSum = tc.limit
			}
			if tc.limit <= 0 {
				wantSum = 0
			}
			if sum != wantSum {
				t.Fatalf("allocated sum = %d, want min(limit,total) = %d", sum, wantSum)
			}
		})
	}
}

// TestBuildMAEstimateQueriesCartesian guards the retrieval completeness promise:
// province, atecoCode and legalFormCode are single-valued at OpenAPI.it, so the
// estimate must probe the full (province × ateco code × legal form) cartesian.
// A missing combination is silent under-retrieval, hence this is locked.
func TestBuildMAEstimateQueriesCartesian(t *testing.T) {
	strategy := MAStrategySpec{
		SectorDescription: "consulenza informatica",
		ActivityStatus:    "ATTIVA",
		Provinces:         []string{"MI", "BG"},
		LegalForms:        []string{"SR", "SP"},
		AtecoCandidates: []MAAtecoCandidate{
			{Code: "62.20.1", SearchCode: "62201"},
			{Code: "62.90", SearchCode: "6290"},
		},
	}

	queries := buildMAEstimateQueries(strategy)

	ateco := map[string]int{}
	expanded := map[string]int{}
	for _, query := range queries {
		switch query.strategyType {
		case maStrategyTypeATECO:
			if query.params.AtecoCode != query.atecoSearchCode {
				t.Fatalf("ateco query params.AtecoCode = %q, want %q", query.params.AtecoCode, query.atecoSearchCode)
			}
			if query.params.LegalFormCode != query.legalForm {
				t.Fatalf("ateco query params.LegalFormCode = %q, want %q", query.params.LegalFormCode, query.legalForm)
			}
			if query.params.Province != query.province {
				t.Fatalf("ateco query params.Province = %q, want %q", query.params.Province, query.province)
			}
			ateco[fmt.Sprintf("%s|%s|%s", query.province, query.atecoSearchCode, query.legalForm)]++
		case maStrategyTypeExpanded:
			if query.params.AtecoCode != "" {
				t.Fatalf("expanded query should not pin an ateco code, got %q", query.params.AtecoCode)
			}
			expanded[fmt.Sprintf("%s|%s", query.province, query.legalForm)]++
		default:
			t.Fatalf("unexpected strategy type %q", query.strategyType)
		}
	}

	wantAteco := []string{
		"MI|62201|SR", "MI|6290|SR", "MI|62201|SP", "MI|6290|SP",
		"BG|62201|SR", "BG|6290|SR", "BG|62201|SP", "BG|6290|SP",
	}
	assertCoverExactlyOnce(t, "ateco", ateco, wantAteco)

	wantExpanded := []string{"MI|SR", "MI|SP", "BG|SR", "BG|SP"}
	assertCoverExactlyOnce(t, "expanded", expanded, wantExpanded)
}

// TestBuildMAEstimateQueriesAnyFormWhenNoConstraint ensures that without a legal
// form perimeter the cartesian collapses on that axis to a single "any form"
// pass (no server-side legal-form filter).
func TestBuildMAEstimateQueriesAnyFormWhenNoConstraint(t *testing.T) {
	strategy := MAStrategySpec{
		SectorDescription: "consulenza informatica",
		ActivityStatus:    "ATTIVA",
		Provinces:         []string{"MI"},
		AtecoCandidates:   []MAAtecoCandidate{{Code: "62.20.1", SearchCode: "62201"}},
	}
	queries := buildMAEstimateQueries(strategy)
	for _, query := range queries {
		if query.legalForm != "" || query.params.LegalFormCode != "" {
			t.Fatalf("expected empty legal form, got query %#v", query)
		}
	}
	// 1 province × 1 form × (1 ateco + 1 expanded) = 2 queries.
	if len(queries) != 2 {
		t.Fatalf("queries = %d, want 2", len(queries))
	}
}

func assertCoverExactlyOnce(t *testing.T, label string, got map[string]int, want []string) {
	t.Helper()
	if len(got) != len(want) {
		keys := make([]string, 0, len(got))
		for key := range got {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		t.Fatalf("%s combos = %v, want %v", label, keys, want)
	}
	for _, key := range want {
		if got[key] != 1 {
			t.Fatalf("%s combo %q covered %d times, want exactly once", label, key, got[key])
		}
	}
}
