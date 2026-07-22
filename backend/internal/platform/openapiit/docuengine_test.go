package openapiit

import (
	"context"
	"net/http"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// DocuEngine serves the EMPTY request list of a virgin account as an ERROR — HTTP 404
// with envelope error=221 "no requests related to your account", not an empty array.
// ListRequests must map that to an empty slice (no error), otherwise the filing_search
// pre-POST reconciliation fails every tick and the POST never fires (the incident).
func TestDocuEngineListRequestsMapsNoRequests404ToEmpty(t *testing.T) {
	docuClient := func(handler http.HandlerFunc) *DocuEngineClient {
		client := New(Config{APIToken: "test-token", HTTPClient: httputil.NewMockClient(handler)})
		return client.DocuEngine()
	}

	t.Run("http 404 with business code 221", func(t *testing.T) {
		dc := docuClient(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"message":"no requests related to your account","error":221,"data":null}`))
		})
		got, err := dc.ListRequests(context.Background())
		if err != nil {
			t.Fatalf("ListRequests returned error, want empty list: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("ListRequests returned %d summaries, want 0", len(got))
		}
	})

	t.Run("business code 221 on http 200 envelope", func(t *testing.T) {
		dc := docuClient(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":false,"message":"no requests related to your account","error":221,"data":null}`))
		})
		got, err := dc.ListRequests(context.Background())
		if err != nil {
			t.Fatalf("ListRequests returned error, want empty list: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("ListRequests returned %d summaries, want 0", len(got))
		}
	})

	t.Run("a real upstream error still surfaces", func(t *testing.T) {
		dc := docuClient(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"success":false,"message":"boom","error":500,"data":null}`))
		})
		if _, err := dc.ListRequests(context.Background()); err == nil {
			t.Fatalf("ListRequests swallowed a real 500 error, want it surfaced")
		}
	})

	t.Run("a populated list decodes normally", func(t *testing.T) {
		dc := docuClient(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"message":"","error":null,"data":[{"id":"req-1","name":"Bilancio Ottico","state":"SEARCH"}]}`))
		})
		got, err := dc.ListRequests(context.Background())
		if err != nil {
			t.Fatalf("ListRequests returned error: %v", err)
		}
		if len(got) != 1 || got[0].ID != "req-1" {
			t.Fatalf("ListRequests = %+v, want one summary req-1", got)
		}
	})
}
