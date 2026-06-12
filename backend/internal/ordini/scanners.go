package ordini

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type NullDate struct {
	Time  time.Time
	Valid bool
}

func (d *NullDate) Scan(value any) error {
	if value == nil {
		d.Valid = false
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		if v.IsZero() {
			d.Valid = false
			return nil
		}
		d.Time = v
		d.Valid = true
		return nil
	case []byte:
		return d.scanString(string(v))
	case string:
		return d.scanString(v)
	default:
		return fmt.Errorf("ordini: unsupported date scan type %T", value)
	}
}

func (d *NullDate) scanString(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || value == "0000-00-00" || value == "0000-00-00 00:00:00" {
		d.Valid = false
		return nil
	}
	layouts := []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			d.Time = t
			d.Valid = true
			return nil
		}
	}
	return fmt.Errorf("ordini: invalid date %q", value)
}

func (d NullDate) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(d.Time.Format("2006-01-02"))
}

func (d NullDate) Value() (driver.Value, error) {
	if !d.Valid {
		return nil, nil
	}
	return d.Time.Format("2006-01-02"), nil
}

func (d NullDate) String() string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

// NullFloat parses vodka varchar money/quantity columns into a number for the
// app API and comparisons, while Raw preserves the column content verbatim
// (production data mixes "10,00" and "350.00" in the same record) for paths
// that must forward it untouched, such as the ERP gateway payload.
type NullFloat struct {
	Float64 float64
	Valid   bool
	Raw     *string
}

func (f *NullFloat) Scan(value any) error {
	if value == nil {
		f.Valid = false
		f.Raw = nil
		return nil
	}
	switch v := value.(type) {
	case float64:
		f.setFloat(v)
		return nil
	case float32:
		f.setFloat(float64(v))
		return nil
	case int:
		f.setFloat(float64(v))
		return nil
	case int64:
		f.setFloat(float64(v))
		return nil
	case []byte:
		return f.scanString(string(v))
	case string:
		return f.scanString(v)
	default:
		return fmt.Errorf("ordini: unsupported float scan type %T", value)
	}
}

func (f *NullFloat) setFloat(value float64) {
	f.Float64 = value
	f.Valid = true
	raw := strconv.FormatFloat(value, 'f', -1, 64)
	f.Raw = &raw
}

func (f *NullFloat) scanString(value string) error {
	f.Raw = &value
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		f.Valid = false
		return nil
	}
	parsed, err := strconv.ParseFloat(strings.ReplaceAll(trimmed, ",", "."), 64)
	if err != nil {
		return err
	}
	f.Float64 = parsed
	f.Valid = true
	return nil
}

func (f NullFloat) MarshalJSON() ([]byte, error) {
	if !f.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(f.Float64)
}

func ptrStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func ptrIntValue(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullIfBlank(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func parseRequiredInt(raw string) (int64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	return value, err == nil
}
