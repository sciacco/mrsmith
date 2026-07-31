package factorial

import (
	"errors"
	"strings"
	"testing"
)

func TestAPIErrorMessageExact(t *testing.T) {
	e := &APIError{
		StatusCode: 404,
		Body:       []byte(`{"error":"missing"}`),
		Method:     "POST",
		URL:        "https://api.factorialhr.com/api/2026-07-01/resources/x/y",
	}
	want := `factorial: POST https://api.factorialhr.com/api/2026-07-01/resources/x/y: 404 Not Found: {"error":"missing"}`
	if got := e.Error(); got != want {
		t.Errorf("Error():\n got: %q\nwant: %q", got, want)
	}
}

func TestAPIErrorMessageTrimming(t *testing.T) {
	t.Run("newlines become spaces", func(t *testing.T) {
		e := &APIError{
			StatusCode: 400,
			Body:       []byte("line1\nline2\nline3"),
			Method:     "GET",
			URL:        "https://example.test/x",
		}
		got := e.Error()
		if strings.Contains(got, "\n") {
			t.Errorf("Error() must not contain newlines: %q", got)
		}
		if !strings.Contains(got, "line1 line2 line3") {
			t.Errorf("Error() must join lines with spaces: %q", got)
		}
	})

	t.Run("overlong body truncated with ellipsis", func(t *testing.T) {
		body := strings.Repeat("a", 300)
		e := &APIError{
			StatusCode: 500,
			Body:       []byte(body),
			Method:     "GET",
			URL:        "https://example.test/x",
		}
		got := e.Error()
		wantSuffix := strings.Repeat("a", maxErrorStringBody) + "…"
		if !strings.HasSuffix(got, wantSuffix) {
			t.Errorf("Error() must end with 256-byte body + ellipsis; tail = %q", tail(got, len(wantSuffix)))
		}
	})

	t.Run("exactly 256 bytes is not truncated", func(t *testing.T) {
		body := strings.Repeat("b", maxErrorStringBody)
		e := &APIError{StatusCode: 500, Body: []byte(body), Method: "GET", URL: "u"}
		got := e.Error()
		if strings.HasSuffix(got, "…") {
			t.Errorf("256-byte body must not be truncated: %q", tail(got, 16))
		}
		if !strings.HasSuffix(got, body) {
			t.Errorf("body tail mismatch: %q", tail(got, 16))
		}
	})
}

func TestAPIErrorIsMethods(t *testing.T) {
	tests := []struct {
		code                       int
		notFound, unauth, forb, rl bool
	}{
		{404, true, false, false, false},
		{401, false, true, false, false},
		{403, false, false, true, false},
		{429, false, false, false, true},
		{500, false, false, false, false},
		{200, false, false, false, false},
	}
	for _, tc := range tests {
		e := &APIError{StatusCode: tc.code}
		if e.IsNotFound() != tc.notFound {
			t.Errorf("code %d IsNotFound = %v, want %v", tc.code, e.IsNotFound(), tc.notFound)
		}
		if e.IsUnauthorized() != tc.unauth {
			t.Errorf("code %d IsUnauthorized = %v, want %v", tc.code, e.IsUnauthorized(), tc.unauth)
		}
		if e.IsForbidden() != tc.forb {
			t.Errorf("code %d IsForbidden = %v, want %v", tc.code, e.IsForbidden(), tc.forb)
		}
		if e.IsRateLimited() != tc.rl {
			t.Errorf("code %d IsRateLimited = %v, want %v", tc.code, e.IsRateLimited(), tc.rl)
		}
	}
}

func TestAPIErrorAsTarget(t *testing.T) {
	// *APIError must satisfy the error interface and be matchable via errors.As.
	var err error = &APIError{StatusCode: 418, Method: "GET", URL: "u"}
	var target *APIError
	if !errors.As(err, &target) {
		t.Fatal("errors.As must match *APIError")
	}
	if target.StatusCode != 418 {
		t.Errorf("target StatusCode = %d, want 418", target.StatusCode)
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
