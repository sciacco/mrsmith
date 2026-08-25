package training

import (
	"testing"
	"time"
)

func testDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func testDatePtr(year int, month time.Month, day int) *time.Time {
	value := testDate(year, month, day)
	return &value
}

func intPtr(value int) *int {
	return &value
}

func TestComputeNextRoundDeadline(t *testing.T) {
	ruleDeadline := testDate(2026, time.October, 31)
	cases := []struct {
		name      string
		months    *int
		anchor    string
		lastRound *time.Time
		want      time.Time
		wantOK    bool
	}{
		{
			name:   "prima tornata senza ricorrenza",
			want:   ruleDeadline,
			wantOK: true,
		},
		{
			name:   "prima tornata con ricorrenza a calendario",
			months: intPtr(12),
			anchor: anchorCalendar,
			want:   ruleDeadline,
			wantOK: true,
		},
		{
			name:   "prima tornata con ancora completion",
			months: intPtr(6),
			anchor: anchorCompletion,
			want:   ruleDeadline,
			wantOK: true,
		},
		{
			name:      "senza ricorrenza esiste solo la prima tornata",
			lastRound: testDatePtr(2026, time.October, 31),
			wantOK:    false,
		},
		{
			name:      "ancora completion senza tornate condivise successive",
			months:    intPtr(6),
			anchor:    anchorCompletion,
			lastRound: testDatePtr(2026, time.October, 31),
			wantOK:    false,
		},
		{
			name:      "calendario: ultima scadenza + mesi",
			months:    intPtr(12),
			anchor:    anchorCalendar,
			lastRound: testDatePtr(2026, time.May, 10),
			want:      testDate(2027, time.May, 10),
			wantOK:    true,
		},
		{
			name:      "calendario a cavallo d'anno",
			months:    intPtr(3),
			anchor:    anchorCalendar,
			lastRound: testDatePtr(2026, time.November, 15),
			want:      testDate(2027, time.February, 15),
			wantOK:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := computeNextRoundDeadline(ruleDeadline, tc.months, tc.anchor, tc.lastRound)
			if ok != tc.wantOK {
				t.Fatalf("computeNextRoundDeadline ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && !got.Equal(tc.want) {
				t.Fatalf("computeNextRoundDeadline = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestRoundWindow(t *testing.T) {
	cases := []struct {
		name     string
		deadline time.Time
		months   int
		wantFrom time.Time
		wantTo   time.Time
	}{
		{
			name:     "finestra a cavallo d'anno",
			deadline: testDate(2027, time.February, 15),
			months:   6,
			wantFrom: testDate(2026, time.August, 15),
			wantTo:   testDate(2027, time.February, 15),
		},
		{
			name:     "finestra annuale da fine anno",
			deadline: testDate(2026, time.December, 31),
			months:   12,
			wantFrom: testDate(2025, time.December, 31),
			wantTo:   testDate(2026, time.December, 31),
		},
		{
			name:     "finestra dentro lo stesso anno",
			deadline: testDate(2026, time.September, 30),
			months:   3,
			wantFrom: testDate(2026, time.June, 30),
			wantTo:   testDate(2026, time.September, 30),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from, to := roundWindow(tc.deadline, tc.months)
			if !from.Equal(tc.wantFrom) {
				t.Fatalf("roundWindow from = %s, want %s", from, tc.wantFrom)
			}
			if !to.Equal(tc.wantTo) {
				t.Fatalf("roundWindow to = %s, want %s", to, tc.wantTo)
			}
		})
	}
}

func TestPersonalDeadline(t *testing.T) {
	ruleDeadline := testDate(2026, time.December, 31)
	cases := []struct {
		name           string
		lastCompletion *time.Time
		months         int
		want           time.Time
	}{
		{
			name:   "mai completato: vale la scadenza della regola",
			months: 6,
			want:   ruleDeadline,
		},
		{
			name:           "ultimo completamento + mesi a cavallo d'anno",
			lastCompletion: testDatePtr(2026, time.October, 20),
			months:         6,
			want:           testDate(2027, time.April, 20),
		},
		{
			name:           "ultimo completamento + dodici mesi",
			lastCompletion: testDatePtr(2026, time.March, 1),
			months:         12,
			want:           testDate(2027, time.March, 1),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := personalDeadline(tc.lastCompletion, tc.months, ruleDeadline)
			if !got.Equal(tc.want) {
				t.Fatalf("personalDeadline = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestWithinHorizon(t *testing.T) {
	asOf := testDate(2026, time.August, 25)
	cases := []struct {
		name     string
		deadline time.Time
		days     int
		want     bool
	}{
		{
			name:     "orizzonte zero ricade sul default: entro 60 giorni",
			deadline: testDate(2026, time.October, 24),
			days:     0,
			want:     true,
		},
		{
			name:     "orizzonte zero ricade sul default: oltre 60 giorni",
			deadline: testDate(2026, time.October, 25),
			days:     0,
			want:     false,
		},
		{
			name:     "orizzonte negativo ricade sul default: entro 60 giorni",
			deadline: testDate(2026, time.October, 24),
			days:     -5,
			want:     true,
		},
		{
			name:     "orizzonte negativo ricade sul default: oltre 60 giorni",
			deadline: testDate(2026, time.October, 25),
			days:     -5,
			want:     false,
		},
		{
			name:     "orizzonte oltre il massimo ridotto a 365: entro",
			deadline: testDate(2027, time.August, 25),
			days:     400,
			want:     true,
		},
		{
			name:     "orizzonte oltre il massimo ridotto a 365: oltre",
			deadline: testDate(2027, time.August, 26),
			days:     400,
			want:     false,
		},
		{
			name:     "scadenza gia superata sempre nell'orizzonte",
			deadline: testDate(2026, time.January, 1),
			days:     30,
			want:     true,
		},
		{
			name:     "giorno limite incluso",
			deadline: testDate(2026, time.September, 24),
			days:     30,
			want:     true,
		},
		{
			name:     "giorno oltre il limite escluso",
			deadline: testDate(2026, time.September, 25),
			days:     30,
			want:     false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := withinHorizon(tc.deadline, asOf, tc.days); got != tc.want {
				t.Fatalf("withinHorizon(%s, %s, %d) = %v, want %v", tc.deadline, asOf, tc.days, got, tc.want)
			}
		})
	}
}
