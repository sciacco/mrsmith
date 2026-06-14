package raenad

import (
	"errors"
	"strings"
)

var errInvalidDecimal = errors.New("invalid_decimal")

// normalizeDecimal18_4 validates a value for PostgreSQL numeric(18,4).
// It never parses through float types and never rounds. Empty values map to
// nil so optional numeric fields can be bound as SQL NULL by later slices.
func normalizeDecimal18_4(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}

	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil, nil
	}
	normalized = strings.ReplaceAll(normalized, ",", ".")

	if strings.Count(normalized, ".") == 1 && strings.HasSuffix(normalized, ".") {
		normalized = strings.TrimSuffix(normalized, ".")
	}
	if strings.HasPrefix(normalized, ".") {
		normalized = "0" + normalized
	} else if strings.HasPrefix(normalized, "-.") {
		normalized = "-0" + normalized[1:]
	}

	sign := ""
	if strings.HasPrefix(normalized, "-") {
		sign = "-"
		normalized = strings.TrimPrefix(normalized, "-")
	}
	if normalized == "" || strings.Contains(normalized, "-") {
		return nil, errInvalidDecimal
	}

	parts := strings.Split(normalized, ".")
	if len(parts) > 2 {
		return nil, errInvalidDecimal
	}

	intPart := parts[0]
	if intPart == "" || !allASCIIDigits(intPart) {
		return nil, errInvalidDecimal
	}

	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
		if fracPart == "" || len(fracPart) > 4 || !allASCIIDigits(fracPart) {
			return nil, errInvalidDecimal
		}
	}

	intPart = strings.TrimLeft(intPart, "0")
	if intPart == "" {
		intPart = "0"
	}
	if len(intPart) > 14 {
		return nil, errInvalidDecimal
	}

	if sign == "-" && intPart == "0" && allZeroes(fracPart) {
		sign = ""
	}

	out := sign + intPart
	if fracPart != "" {
		out += "." + fracPart
	}
	return &out, nil
}

func allASCIIDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func allZeroes(value string) bool {
	for _, r := range value {
		if r != '0' {
			return false
		}
	}
	return true
}
