package factorial

import (
	"fmt"
	"net/http"
	"strings"
)

// maxErrorStringBody caps the rendered response body inside APIError.Error.
// Bodies longer than this are truncated and marked with an ellipsis so a
// single error stays log-friendly and testable.
const maxErrorStringBody = 256

// APIError is returned by the transport layer for any non-2xx response from
// the Factorial API. It captures the request context and (a trimmed copy of)
// the response body so callers can branch on status or inspect the payload.
type APIError struct {
	StatusCode int    // HTTP status code, e.g. 404.
	Body       []byte // captured response body, capped at 64KiB.
	Method     string // HTTP method, e.g. "POST".
	URL        string // full request URL (base + prefix + path + query).
}

// Error implements the error interface. The format is
//
//	factorial: <METHOD> <URL>: <code> <StatusText>: <trimmed body>
//
// The body has newlines replaced with spaces and is truncated to
// maxErrorStringBody bytes (with a trailing ellipsis) so the message is safe
// to log on a single line.
func (e *APIError) Error() string {
	body := strings.ReplaceAll(string(e.Body), "\n", " ")
	if len(body) > maxErrorStringBody {
		body = body[:maxErrorStringBody] + "…"
	}
	return fmt.Sprintf("factorial: %s %s: %d %s: %s",
		e.Method, e.URL, e.StatusCode, http.StatusText(e.StatusCode), body)
}

// IsNotFound reports whether the error is an HTTP 404.
func (e *APIError) IsNotFound() bool { return e.StatusCode == http.StatusNotFound }

// IsUnauthorized reports whether the error is an HTTP 401.
func (e *APIError) IsUnauthorized() bool { return e.StatusCode == http.StatusUnauthorized }

// IsForbidden reports whether the error is an HTTP 403.
func (e *APIError) IsForbidden() bool { return e.StatusCode == http.StatusForbidden }

// IsRateLimited reports whether the error is an HTTP 429.
func (e *APIError) IsRateLimited() bool { return e.StatusCode == http.StatusTooManyRequests }
