package googledrive

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

// Code is the shared error taxonomy exposed to consumer domains (#97 §8).
// Consumers branch on the code only; upstream details (Google payload,
// headers, retries) live exclusively in the diagnostic event.
type Code string

const (
	// CodeNotConfigured: the (app, context) row is missing or disabled, or no
	// credential is stored. Disables only the affected context.
	CodeNotConfigured Code = "not_configured"
	// CodeInvalidCredentials: the stored Service Account JSON cannot be parsed
	// or Google rejects it at token exchange.
	CodeInvalidCredentials Code = "invalid_credentials"
	// CodePermissionDenied: Google actually returned 403 (not the revoked-
	// membership case, which surfaces as 404 → CodeNotFound).
	CodePermissionDenied Code = "permission_denied"
	// CodeNotFound means "not found OR not accessible": revoking the Service
	// Account from a Shared Drive yields the same Google 404 as a wrong or
	// deleted ID, and the two cannot be told apart (verified in the spike).
	CodeNotFound Code = "not_found"
	// CodeTrashed: the addressed container is in the trash. Trashed resources
	// are still returned successfully by Google with trashed=true, so this
	// check is explicit on our side.
	CodeTrashed Code = "trashed"
	// CodeOutsideContextRoot: the resource exists but does not belong to the
	// configured Shared Drive or is not a descendant of the configured root.
	CodeOutsideContextRoot Code = "outside_context_root"
	// CodeRateLimited: Google throttled the call and retries were exhausted.
	CodeRateLimited Code = "rate_limited"
	// CodeUnavailable: transient upstream or network failure.
	CodeUnavailable Code = "unavailable"
	// CodeInvalidUpstreamResponse: Google answered something we cannot interpret.
	CodeInvalidUpstreamResponse Code = "invalid_upstream_response"
)

// Error is the only error type returned to consumers. The message stays
// synthetic; the full upstream detail is emitted as a diagnostic event.
type Error struct {
	Code Code
	Op   string // GetItem | ListChildren | CreateFolder
	msg  string
	err  error
}

func (e *Error) Error() string {
	if e.msg != "" {
		return fmt.Sprintf("googledrive: %s: %s: %s", e.Op, e.Code, e.msg)
	}
	return fmt.Sprintf("googledrive: %s: %s", e.Op, e.Code)
}

func (e *Error) Unwrap() error { return e.err }

func newError(op string, code Code, msg string, cause error) *Error {
	return &Error{Code: code, Op: op, msg: msg, err: cause}
}

// CodeOf extracts the taxonomy code from any error returned by this package;
// it returns "" for foreign errors.
func CodeOf(err error) Code {
	if e, ok := errors.AsType[*Error](err); ok {
		return e.Code
	}
	return ""
}

// IsCode reports whether err carries the given taxonomy code.
func IsCode(err error, code Code) bool { return CodeOf(err) == code }

// mapUpstreamError converts a raw Google/network error into the taxonomy.
// The spike-verified quirks are encoded here: revoked Shared Drive membership
// produces 404 notFound (never 403), so 404 always means "not found or not
// accessible"; 403 with a rate-limit reason is throttling, not a permission
// problem.
func mapUpstreamError(op string, err error) *Error {
	if gerr, ok := errors.AsType[*googleapi.Error](err); ok {
		switch {
		case gerr.Code == http.StatusNotFound:
			return newError(op, CodeNotFound, "not found or not accessible", err)
		case gerr.Code == http.StatusForbidden && isRateLimitReason(gerr):
			return newError(op, CodeRateLimited, "", err)
		case gerr.Code == http.StatusForbidden:
			return newError(op, CodePermissionDenied, "", err)
		case gerr.Code == http.StatusUnauthorized:
			return newError(op, CodeInvalidCredentials, "", err)
		case gerr.Code == http.StatusTooManyRequests:
			return newError(op, CodeRateLimited, "", err)
		case gerr.Code >= 500:
			return newError(op, CodeUnavailable, "", err)
		default:
			return newError(op, CodeInvalidUpstreamResponse, fmt.Sprintf("unexpected status %d", gerr.Code), err)
		}
	}
	if rerr, ok := errors.AsType[*oauth2.RetrieveError](err); ok {
		if rerr.Response != nil && rerr.Response.StatusCode >= 500 {
			return newError(op, CodeUnavailable, "token endpoint unavailable", err)
		}
		return newError(op, CodeInvalidCredentials, "token exchange rejected", err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return newError(op, CodeUnavailable, "request cancelled or timed out", err)
	}
	return newError(op, CodeUnavailable, "", err)
}

func isRateLimitReason(gerr *googleapi.Error) bool {
	for _, item := range gerr.Errors {
		reason := strings.ToLower(item.Reason)
		if strings.Contains(reason, "ratelimit") {
			return true
		}
	}
	return false
}
