package factorial

import (
	"fmt"
	"strings"
	"time"
)

// dateParseLayouts are the accepted input formats for Date.UnmarshalJSON,
// tried in order. The spec sends calendar days as bare YYYY-MM-DD, but
// timestamps occasionally leak through, so we also tolerate the two RFC3339
// variants and truncate to the day after parsing.
var dateParseLayouts = []string{
	"2006-01-02",
	time.RFC3339,
	"2006-01-02T15:04:05.000Z",
}

// timeParseLayouts are the accepted input formats for Time.UnmarshalJSON,
// tried in order. The spec has zero format: annotations on *_at fields, so we
// accept the common variants the API actually emits.
var timeParseLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000Z",
	"2006-01-02",
}

// Date is a calendar day (timezone-free YYYY-MM-DD). It marshals as a bare
// date string and tolerates a few timestamp shapes on decode, truncating any
// time component to the start of the day.
type Date struct {
	time.Time
}

// MarshalJSON implements json.Marshaler. A zero Date marshals as JSON null.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Time.Format("2006-01-02") + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler. It accepts JSON null, the empty
// string, bare YYYY-MM-DD, and the RFC3339 / millisecond-Z timestamp shapes,
// normalizing the result to midnight on the parsed day.
func (d *Date) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		d.Time = time.Time{}
		return nil
	}
	var (
		parsed time.Time
		err    error
	)
	for _, layout := range dateParseLayouts {
		parsed, err = time.Parse(layout, s)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("factorial: invalid date %q", s)
	}
	d.Time = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, parsed.Location())
	return nil
}

// String formats the Date as YYYY-MM-DD. It is used for query encoding.
func (d Date) String() string {
	return d.Time.Format("2006-01-02")
}

// Time is a timestamp with tolerant decoding. It is the type the generated
// code maps every *_at response field onto (as *Time, not *time.Time).
type Time struct {
	time.Time
}

// MarshalJSON implements json.Marshaler. A zero Time marshals as JSON null;
// otherwise RFC3339Nano is used.
func (t Time) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + t.Time.Format(time.RFC3339Nano) + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler. It accepts JSON null, the empty
// string, RFC3339 with or without fractional seconds, the explicit
// .000Z millisecond style, and a bare YYYY-MM-DD.
func (t *Time) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		t.Time = time.Time{}
		return nil
	}
	var (
		parsed time.Time
		err    error
	)
	for _, layout := range timeParseLayouts {
		parsed, err = time.Parse(layout, s)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("factorial: invalid time %q", s)
	}
	t.Time = parsed
	return nil
}

// String formats the Time as RFC3339. It is used for query encoding.
func (t Time) String() string {
	return t.Time.Format(time.RFC3339)
}
