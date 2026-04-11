package quotes

import (
	"net/url"
	"strings"
	"testing"
)

func TestIsAppsmithDefaultQuoteListRequest(t *testing.T) {
	tests := []struct {
		name  string
		query url.Values
		want  bool
	}{
		{
			name:  "empty query uses appsmith defaults",
			query: url.Values{},
			want:  true,
		},
		{
			name: "explicit first page keeps paginated mode",
			query: url.Values{
				"page": {"1"},
			},
			want: false,
		},
		{
			name: "filters keep paginated mode",
			query: url.Values{
				"status": {"DRAFT"},
			},
			want: false,
		},
		{
			name: "explicit sort keeps paginated mode",
			query: url.Values{
				"sort": {"quote_number"},
				"dir":  {"desc"},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAppsmithDefaultQuoteListRequest(tc.query); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

// TestListDealsQueryMatchesAppsmithEligibility pins the `/quotes/v1/deals`
// SQL to the Appsmith `get_potentials` eligibility rules. Source:
// apps/quotes/quotes-main.tar.gz → quotes-main/pages/Nuova Proposta/queries/get_potentials/get_potentials.txt
// Any drift here must be reviewed against that Appsmith query.
func TestListDealsQueryMatchesAppsmithEligibility(t *testing.T) {
	// Pipeline/stage constants must match Appsmith source verbatim.
	if standardPipeline != "255768766" {
		t.Fatalf("standardPipeline drift: got %q, want %q", standardPipeline, "255768766")
	}
	if iaasPipeline != "255768768" {
		t.Fatalf("iaasPipeline drift: got %q, want %q", iaasPipeline, "255768768")
	}
	wantStandardStages := []string{"424443344", "424502259", "424502261", "424502262"}
	if len(standardStages) != len(wantStandardStages) {
		t.Fatalf("standardStages length: got %d, want %d", len(standardStages), len(wantStandardStages))
	}
	for i, s := range wantStandardStages {
		if standardStages[i] != s {
			t.Fatalf("standardStages[%d] drift: got %q, want %q", i, standardStages[i], s)
		}
	}
	wantIaasStages := []string{"424443381", "424443586", "424443588", "424443587", "424443589"}
	if len(iaasStages) != len(wantIaasStages) {
		t.Fatalf("iaasStages length: got %d, want %d", len(iaasStages), len(wantIaasStages))
	}
	for i, s := range wantIaasStages {
		if iaasStages[i] != s {
			t.Fatalf("iaasStages[%d] drift: got %q, want %q", i, iaasStages[i], s)
		}
	}

	// The assembled query must honor the three Appsmith eligibility rules:
	// pipeline filter, stage whitelist, and non-empty codice.
	q := listDealsQuery
	mustContain := []string{
		"d.pipeline = '255768766'",
		"d.dealstage IN ('424443344','424502259','424502261','424502262')",
		"d.pipeline = '255768768'",
		"d.dealstage IN ('424443381','424443586','424443588','424443587','424443589')",
		"d.codice <> ''",
	}
	for _, frag := range mustContain {
		if !strings.Contains(q, frag) {
			t.Fatalf("listDealsQuery missing Appsmith eligibility fragment %q; query was:\n%s", frag, q)
		}
	}
}
