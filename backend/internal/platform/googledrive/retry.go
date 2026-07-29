package googledrive

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/api/googleapi"
)

// Read retry policy (#97 §9): GetItem/ListChildren (and the ancestry walks
// they trigger) retry a limited number of times on rate limiting and
// transient upstream errors, honoring Retry-After. CreateFolder never goes
// through here.
const (
	maxReadAttempts  = 3
	baseRetryBackoff = 500 * time.Millisecond
	maxRetryBackoff  = 5 * time.Second
)

// retryRead runs one idempotent Drive read, retrying transient failures. It
// counts attempts on the callState so diagnostics report the real number.
func retryRead[T any](call *callState, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error
	for attempt := range maxReadAttempts {
		call.attempts++
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err
		delay, retryable := retryDelay(err, attempt)
		if !retryable || attempt == maxReadAttempts-1 {
			return zero, err
		}
		select {
		case <-time.After(delay):
		case <-call.ctx.Done():
			return zero, call.ctx.Err()
		}
	}
	return zero, lastErr
}

// retryDelay decides whether an upstream error is worth retrying and how long
// to wait, preferring the server's Retry-After over exponential backoff.
func retryDelay(err error, attempt int) (time.Duration, bool) {
	backoff := min(baseRetryBackoff<<attempt, maxRetryBackoff)
	if gerr, ok := errors.AsType[*googleapi.Error](err); ok {
		retryable := gerr.Code == http.StatusTooManyRequests ||
			gerr.Code >= 500 ||
			(gerr.Code == http.StatusForbidden && isRateLimitReason(gerr))
		if !retryable {
			return 0, false
		}
		if retryAfter := gerr.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil && seconds >= 0 {
				return time.Duration(seconds) * time.Second, true
			}
		}
		return backoff, true
	}
	// Non-HTTP errors (network, EOF): transient by default for reads.
	return backoff, true
}
