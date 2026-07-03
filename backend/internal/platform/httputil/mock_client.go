package httputil

import (
	"net/http"
	"net/http/httptest"
)

// RoundTripperFunc is a type that implements http.RoundTripper.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// NewMockClient returns an http.Client that passes all requests to the given handler
// using httptest.NewRecorder, avoiding the need for a real network server.
func NewMockClient(h http.Handler) *http.Client {
	return &http.Client{
		Transport: RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			return rec.Result(), nil
		}),
	}
}
