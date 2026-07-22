package openapiit

import (
	"context"
	"encoding/json"
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

// Il vendor è incoerente tra esempi e produzione sui tipi scalari (fileSize stringa
// nella spec, numero in produzione; documents come stringhe): il decode dei Download
// e della Request deve tollerare entrambe le forme.
func TestDocuEngineFlexibleDecoding(t *testing.T) {
	t.Run("download with numeric fileSize and urlExpire", func(t *testing.T) {
		var d DocuDownload
		payload := `{"fileName":"req_0.pdf","mimeType":"application/pdf","fileSize":34144,"md5":"IPJJgdrAt4xrvGR4ve7oTg==","urlExpire":1767000000,"downloadUrl":"https://example"}`
		if err := json.Unmarshal([]byte(payload), &d); err != nil {
			t.Fatalf("decode numeric forms: %v", err)
		}
		if d.FileSize != "34144" || d.URLExpire != 1767000000 {
			t.Fatalf("decoded %+v, want fileSize 34144 urlExpire 1767000000", d)
		}
	})
	t.Run("download with string fileSize and urlExpire", func(t *testing.T) {
		var d DocuDownload
		payload := `{"fileSize":"34144","urlExpire":"1767000000"}`
		if err := json.Unmarshal([]byte(payload), &d); err != nil {
			t.Fatalf("decode string forms: %v", err)
		}
		if d.FileSize != "34144" || d.URLExpire != 1767000000 {
			t.Fatalf("decoded %+v, want fileSize 34144 urlExpire 1767000000", d)
		}
	})
	t.Run("request documents are file-name strings", func(t *testing.T) {
		var r DocuRequest
		payload := `{"id":"req-1","state":"DONE","documents":["req-1_0.pdf"],"resultId":"abc"}`
		if err := json.Unmarshal([]byte(payload), &r); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(r.Documents) != 1 || r.Documents[0] != "req-1_0.pdf" {
			t.Fatalf("decoded %+v, want one document name", r.Documents)
		}
	})
}
