package raenad

import (
	"errors"
	"testing"
)

func TestNormalizeDecimal18_4AcceptsAndCanonicalizes(t *testing.T) {
	cases := []struct {
		name  string
		input *string
		want  *string
	}{
		{name: "nil", input: nil, want: nil},
		{name: "empty", input: ptr("  "), want: nil},
		{name: "integer", input: ptr("12"), want: ptr("12")},
		{name: "max numeric", input: ptr("99999999999999.9999"), want: ptr("99999999999999.9999")},
		{name: "leading zeroes", input: ptr("000001.2300"), want: ptr("1.2300")},
		{name: "leading decimal separator", input: ptr(".5"), want: ptr("0.5")},
		{name: "negative leading decimal separator", input: ptr("-.5"), want: ptr("-0.5")},
		{name: "trailing decimal separator", input: ptr("42."), want: ptr("42")},
		{name: "comma decimal separator", input: ptr("12,3400"), want: ptr("12.3400")},
		{name: "negative zero", input: ptr("-0.0000"), want: ptr("0.0000")},
		{name: "all leading zeroes", input: ptr("000000000000000"), want: ptr("0")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeDecimal18_4(tc.input)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if stringPtrValue(got) != stringPtrValue(tc.want) {
				t.Fatalf("got %v, want %v", stringPtrValue(got), stringPtrValue(tc.want))
			}
		})
	}
}

func TestNormalizeDecimal18_4RejectsInvalidScaleAndPrecision(t *testing.T) {
	cases := []string{
		"100000000000000",
		"99999999999999.99999",
		"1.00000",
		"1.2.3",
		"1e2",
		"+1",
		"--1",
		"abc",
		"1_000",
	}

	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			if _, err := normalizeDecimal18_4(ptr(tc)); !errors.Is(err, errInvalidDecimal) {
				t.Fatalf("expected errInvalidDecimal, got %v", err)
			}
		})
	}
}

func ptr(value string) *string {
	return &value
}

func stringPtrValue(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}
