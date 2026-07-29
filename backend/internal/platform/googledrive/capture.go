package googledrive

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
)

// exchange records one full HTTP round trip against the Drive API for the
// diagnostic event (#97 §11: URL, query, method, status, headers, whole body,
// no internal truncation). Request headers are never recorded — the capture
// transport sits below the oauth2 transport, so the outgoing request carries
// the bearer token; response headers are safe.
type exchange struct {
	Method     string      `json:"method"`
	URL        string      `json:"url"`
	Status     int         `json:"status"`
	RespHeader http.Header `json:"responseHeaders"`
	RespBody   string      `json:"responseBody"`
	NetErr     string      `json:"networkError,omitempty"`
}

// captureLog accumulates the exchanges of a single application-level
// operation (including retries and ancestry walks). It travels via context so
// the cached, shared drive client stays goroutine-safe.
type captureLog struct {
	mu        sync.Mutex
	exchanges []exchange
}

func (l *captureLog) add(e exchange) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.exchanges = append(l.exchanges, e)
}

func (l *captureLog) all() []exchange {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]exchange(nil), l.exchanges...)
}

type captureKey struct{}

func withCapture(ctx context.Context) (context.Context, *captureLog) {
	log := &captureLog{}
	return context.WithValue(ctx, captureKey{}, log), log
}

func captureFrom(ctx context.Context) *captureLog {
	log, _ := ctx.Value(captureKey{}).(*captureLog)
	return log
}

// captureTransport records every Drive API round trip into the captureLog
// found in the request context, buffering the response body so the caller
// still reads it untouched.
type captureTransport struct {
	next http.RoundTripper
}

func (t *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	next := t.next
	if next == nil {
		next = http.DefaultTransport
	}
	log := captureFrom(req.Context())
	resp, err := next.RoundTrip(req)
	if log == nil {
		return resp, err
	}
	e := exchange{Method: req.Method, URL: req.URL.String()}
	if err != nil {
		e.NetErr = err.Error()
		log.add(e)
		return resp, err
	}
	e.Status = resp.StatusCode
	e.RespHeader = resp.Header.Clone()
	if resp.Body != nil {
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			e.NetErr = readErr.Error()
		}
		e.RespBody = string(body)
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}
	log.add(e)
	return resp, err
}
