package factorial

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDateRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		input      string // raw JSON fed to UnmarshalJSON
		wantZero   bool
		wantYear   int
		wantMonth  time.Month
		wantDay    int
		wantJSON   string // expected MarshalJSON output
		wantString string // expected String() output ("" to skip)
	}{
		{
			name:     "json null",
			input:    `null`,
			wantZero: true,
			wantJSON: `null`,
		},
		{
			name:     "empty string",
			input:    `""`,
			wantZero: true,
			wantJSON: `null`,
		},
		{
			name:       "bare date",
			input:      `"2023-05-14"`,
			wantYear:   2023,
			wantMonth:  time.May,
			wantDay:    14,
			wantJSON:   `"2023-05-14"`,
			wantString: "2023-05-14",
		},
		{
			name:      "rfc3339 truncated to day",
			input:     `"2023-05-14T08:30:00Z"`,
			wantYear:  2023,
			wantMonth: time.May,
			wantDay:   14,
			wantJSON:  `"2023-05-14"`,
		},
		{
			name:      "millis-z truncated to day",
			input:     `"2023-05-14T08:30:00.000Z"`,
			wantYear:  2023,
			wantMonth: time.May,
			wantDay:   14,
			wantJSON:  `"2023-05-14"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var d Date
			if err := json.Unmarshal([]byte(tc.input), &d); err != nil {
				t.Fatalf("UnmarshalJSON(%s): unexpected error: %v", tc.input, err)
			}
			if tc.wantZero {
				if !d.Time.IsZero() {
					t.Fatalf("expected zero Time, got %v", d.Time)
				}
			} else {
				if d.Year() != tc.wantYear || d.Month() != tc.wantMonth || d.Day() != tc.wantDay {
					t.Fatalf("date fields: got %v, want %d-%d-%d", d.Time, tc.wantYear, tc.wantMonth, tc.wantDay)
				}
				// Must be truncated to midnight.
				if h, m, s := d.Clock(); h != 0 || m != 0 || s != 0 {
					t.Fatalf("expected truncated to midnight, got %02d:%02d:%02d", h, m, s)
				}
			}
			out, err := d.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON: %v", err)
			}
			if string(out) != tc.wantJSON {
				t.Errorf("MarshalJSON: got %s, want %s", out, tc.wantJSON)
			}
			if tc.wantString != "" && d.String() != tc.wantString {
				t.Errorf("String: got %q, want %q", d.String(), tc.wantString)
			}
		})
	}
}

func TestDateMarshalStandalone(t *testing.T) {
	// Zero value -> null, regardless of value/pointer.
	zero := Date{}
	got, err := zero.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "null" {
		t.Errorf("zero Date marshal: got %s, want null", got)
	}

	d := Date{Time: time.Date(1999, time.December, 31, 23, 59, 59, 0, time.UTC)}
	got, err = (&d).MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `"1999-12-31"` {
		t.Errorf("pointer marshal: got %s", got)
	}
}

func TestDateInvalid(t *testing.T) {
	var d Date
	err := json.Unmarshal([]byte(`"not-a-date"`), &d)
	if err == nil {
		t.Fatal("expected error for invalid date, got nil")
	}
	if !strings.Contains(err.Error(), "factorial:") {
		t.Errorf("error %q must contain 'factorial:'", err.Error())
	}
}

func TestDateRoundTripPreservesValue(t *testing.T) {
	original := Date{Time: time.Date(2021, time.January, 2, 0, 0, 0, 0, time.UTC)}
	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var round Date
	if err := json.Unmarshal(raw, &round); err != nil {
		t.Fatal(err)
	}
	if !round.Time.Equal(original.Time) {
		t.Errorf("round-trip mismatch: got %v, want %v", round.Time, original.Time)
	}
}

func TestTimeRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantEqual  time.Time // parsed expectation
		wantZero   bool
		wantString string // expected String() output (RFC3339)
	}{
		{name: "json null", input: `null`, wantZero: true},
		{name: "empty string", input: `""`, wantZero: true},
		{
			name:       "rfc3339 utc",
			input:      `"2023-05-14T08:30:00Z"`,
			wantEqual:  time.Date(2023, time.May, 14, 8, 30, 0, 0, time.UTC),
			wantString: "2023-05-14T08:30:00Z",
		},
		{
			name:      "millis z",
			input:     `"2023-05-14T08:30:00.000Z"`,
			wantEqual: time.Date(2023, time.May, 14, 8, 30, 0, 0, time.UTC),
		},
		{
			name:      "fractional seconds",
			input:     `"2023-05-14T08:30:00.123456789Z"`,
			wantEqual: time.Date(2023, time.May, 14, 8, 30, 0, 123456789, time.UTC),
		},
		{
			name:      "bare date parsed at midnight",
			input:     `"2023-05-14"`,
			wantEqual: time.Date(2023, time.May, 14, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var tm Time
			if err := json.Unmarshal([]byte(tc.input), &tm); err != nil {
				t.Fatalf("UnmarshalJSON(%s): %v", tc.input, err)
			}
			if tc.wantZero {
				if !tm.Time.IsZero() {
					t.Fatalf("expected zero Time, got %v", tm.Time)
				}
				out, _ := tm.MarshalJSON()
				if string(out) != "null" {
					t.Errorf("zero marshal: got %s, want null", out)
				}
				return
			}
			if !tm.Time.Equal(tc.wantEqual) {
				t.Errorf("parsed value: got %v, want %v", tm.Time, tc.wantEqual)
			}
			if tc.wantString != "" && tm.String() != tc.wantString {
				t.Errorf("String: got %q, want %q", tm.String(), tc.wantString)
			}
		})
	}
}

func TestTimeMarshalNano(t *testing.T) {
	// Non-zero nanoseconds must survive a marshal/unmarshal round trip.
	original := Time{Time: time.Date(2023, time.May, 14, 8, 30, 0, 123456789, time.UTC)}
	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "123456789") {
		t.Errorf("expected fractional seconds in %s", raw)
	}
	var round Time
	if err := json.Unmarshal(raw, &round); err != nil {
		t.Fatal(err)
	}
	if !round.Time.Equal(original.Time) {
		t.Errorf("round-trip: got %v, want %v", round.Time, original.Time)
	}
}

func TestTimeInvalid(t *testing.T) {
	var tm Time
	err := json.Unmarshal([]byte(`"totally-not-a-time"`), &tm)
	if err == nil {
		t.Fatal("expected error for invalid time, got nil")
	}
	if !strings.Contains(err.Error(), "factorial:") {
		t.Errorf("error %q must contain 'factorial:'", err.Error())
	}
}

func TestDateStringZeroFormat(t *testing.T) {
	// A zero Date still formats without panicking; it yields the epoch date.
	if got := (Date{}).String(); len(got) != len("2006-01-02") {
		t.Errorf("zero Date String len: got %q", got)
	}
}

func TestTimeStringFormat(t *testing.T) {
	tm := Time{Time: time.Date(2023, time.May, 14, 8, 30, 0, 0, time.UTC)}
	if got, want := tm.String(), "2023-05-14T08:30:00Z"; got != want {
		t.Errorf("String: got %q, want %q", got, want)
	}
}
