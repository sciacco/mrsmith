package binocolo

import (
	"strings"
	"testing"
)

func TestNormalizeMACompanySearch(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		wantKind  string
		wantValue string
		wantErr   bool
	}{
		{name: "empty is recent", query: "", wantKind: maCompanySearchRecent, wantValue: ""},
		{name: "blank is recent", query: "   ", wantKind: maCompanySearchRecent, wantValue: ""},
		{name: "vat plain", query: "01234567890", wantKind: maCompanySearchVAT, wantValue: "01234567890"},
		{name: "vat strips IT prefix and spaces", query: "it 01234567890", wantKind: maCompanySearchVAT, wantValue: "01234567890"},
		{name: "vat strips dots", query: "012.345 678.90", wantKind: maCompanySearchVAT, wantValue: "01234567890"},
		{name: "tax code uppercased", query: "rssmra80a01h501u", wantKind: maCompanySearchTax, wantValue: "RSSMRA80A01H501U"},
		{name: "name preserves case", query: "Adam Srl", wantKind: maCompanySearchName, wantValue: "Adam Srl"},
		{name: "name escapes ILIKE wildcards", query: `50%_di\`, wantKind: maCompanySearchName, wantValue: `50\%\_di\\`},
		{name: "IT prefix with letters stays name", query: "IT1234567890A", wantKind: maCompanySearchName, wantValue: "IT1234567890A"},
		{name: "twelve digits are a name", query: "123456789012", wantKind: maCompanySearchName, wantValue: "123456789012"},
		{name: "single char rejected", query: "a", wantErr: true},
		{name: "over 200 chars rejected", query: strings.Repeat("a", 201), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, value, err := normalizeMACompanySearch(tc.query)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got kind=%q value=%q", kind, value)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if kind != tc.wantKind || value != tc.wantValue {
				t.Fatalf("got kind=%q value=%q, want kind=%q value=%q", kind, value, tc.wantKind, tc.wantValue)
			}
		})
	}
}

func TestResolveMACompanySearchStatus(t *testing.T) {
	excluded := maRatingExcluded
	preferred := 2
	inThesis := &MATargetRow{MatchState: "in_criterio", Confidence: "alta"}

	t.Run("active card wins over everything", func(t *testing.T) {
		status := resolveMACompanySearchStatus(maCompanySearchHydration{
			ActiveCardState:      "in_dialogo",
			ActiveCardInitiative: "Roll-up MSP",
			ClosedCardEsito:      "no_go",
			LatestRating:         &excluded,
			LatestTarget:         inThesis,
		})
		if status.Kind != "working" || status.Value != "in_dialogo" || status.ContextTitle != "Roll-up MSP" {
			t.Fatalf("unexpected status: %+v", status)
		}
	})

	t.Run("closed card beats rating and appearance", func(t *testing.T) {
		status := resolveMACompanySearchStatus(maCompanySearchHydration{
			ClosedCardEsito:      "no_go",
			ClosedCardInitiative: "Roll-up MSP",
			LatestRating:         &excluded,
			LatestTarget:         inThesis,
		})
		if status.Kind != "closed" || status.Value != "no_go" || status.ContextTitle != "Roll-up MSP" {
			t.Fatalf("unexpected status: %+v", status)
		}
	})

	t.Run("excluded rating carries reason", func(t *testing.T) {
		status := resolveMACompanySearchStatus(maCompanySearchHydration{
			LatestRating:       &excluded,
			LatestRatingReason: "fuori perimetro",
			LatestTarget:       inThesis,
		})
		if status.Kind != "excluded" || status.Reason != "fuori perimetro" {
			t.Fatalf("unexpected status: %+v", status)
		}
	})

	t.Run("positive rating is preferred", func(t *testing.T) {
		status := resolveMACompanySearchStatus(maCompanySearchHydration{
			LatestRating: &preferred,
			LatestTarget: inThesis,
		})
		if status.Kind != "preferred" || status.Value != "2" {
			t.Fatalf("unexpected status: %+v", status)
		}
	})

	t.Run("appearance routes through maRouteTarget", func(t *testing.T) {
		status := resolveMACompanySearchStatus(maCompanySearchHydration{LatestTarget: inThesis})
		if status.Kind != "thesis" || status.Value != maBucketPrincipale {
			t.Fatalf("unexpected status: %+v", status)
		}
	})

	t.Run("low confidence appearance needs review", func(t *testing.T) {
		status := resolveMACompanySearchStatus(maCompanySearchHydration{
			LatestTarget: &MATargetRow{MatchState: "in_criterio", Confidence: "bassa"},
		})
		if status.Kind != "review" || status.Value != maBucketDaVerificare {
			t.Fatalf("unexpected status: %+v", status)
		}
	})

	t.Run("outside appearance is suppressed with reason", func(t *testing.T) {
		status := resolveMACompanySearchStatus(maCompanySearchHydration{
			LatestTarget: &MATargetRow{MatchState: maMatchStateOutside},
		})
		if status.Kind != "suppressed" || status.Value != maBucketSoppresso || status.Reason == "" {
			t.Fatalf("unexpected status: %+v", status)
		}
	})

	t.Run("empty hydration is registry-only", func(t *testing.T) {
		status := resolveMACompanySearchStatus(maCompanySearchHydration{})
		if status.Kind != "registry" {
			t.Fatalf("unexpected status: %+v", status)
		}
	})
}
