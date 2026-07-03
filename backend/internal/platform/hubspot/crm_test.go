package hubspot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func TestCreateDealRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if got, want := r.URL.Path, "/crm/objects/2026-03/0-3"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		properties := body["properties"].(map[string]any)
		if got, want := properties["dealname"], "Q-1 - Example"; got != want {
			t.Fatalf("dealname = %v, want %v", got, want)
		}
		associations := body["associations"].([]any)
		first := associations[0].(map[string]any)
		to := first["to"].(map[string]any)
		if got, want := to["id"], "456"; got != want {
			t.Fatalf("association to id = %v, want %v", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":123,"properties":{"dealname":"Q-1 - Example"}}`)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", httputil.NewMockClient(handler))
	obj, err := client.CreateDeal(context.Background(), map[string]any{
		"dealname": "Q-1 - Example",
	}, []ObjectAssociation{NewObjectAssociation("456", AssocTypeDealToCompany)})
	if err != nil {
		t.Fatalf("CreateDeal() error = %v", err)
	}
	if got, want := obj.ID, "123"; got != want {
		t.Fatalf("id = %s, want %s", got, want)
	}
}

func TestUpdateDealRequestExcludesDealstageWhenNotSupplied(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s", r.Method)
		}
		if got, want := r.URL.Path, "/crm/objects/2026-03/0-3/123"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		var body struct {
			Properties map[string]any `json:"properties"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if _, ok := body.Properties["dealstage"]; ok {
			t.Fatal("dealstage was present")
		}
		if got, want := body.Properties["dealname"], "Updated"; got != want {
			t.Fatalf("dealname = %v, want %v", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"123","properties":{"dealname":"Updated"}}`)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", httputil.NewMockClient(handler))
	if _, err := client.UpdateDeal(context.Background(), "123", map[string]any{"dealname": "Updated"}); err != nil {
		t.Fatalf("UpdateDeal() error = %v", err)
	}
}

func TestGetDealStageRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if got, want := r.URL.Path, "/crm/objects/2026-03/0-3/123"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		if got, want := r.URL.Query().Get("properties"), "pipeline,dealstage"; got != want {
			t.Fatalf("properties = %s, want %s", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"123","properties":{"pipeline":"pipe-1","dealstage":"stage-1"}}`)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", httputil.NewMockClient(handler))
	stage, err := client.GetDealStage(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetDealStage() error = %v", err)
	}
	if got, want := stage.Pipeline, "pipe-1"; got != want {
		t.Fatalf("pipeline = %s, want %s", got, want)
	}
	if got, want := stage.Dealstage, "stage-1"; got != want {
		t.Fatalf("dealstage = %s, want %s", got, want)
	}
}

func TestSearchCompaniesByDomainRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if got, want := r.URL.Path, "/crm/objects/2026-03/0-2/search"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		var body SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		filter := body.FilterGroups[0].Filters[0]
		if filter.PropertyName != "domain" || filter.Operator != "EQ" || filter.Value != "example.com" {
			t.Fatalf("filter = %+v", filter)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"results":[{"id":"456","properties":{"domain":"example.com"}}]}`)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", httputil.NewMockClient(handler))
	results, err := client.SearchCompaniesByDomain(context.Background(), "example.com", []string{"domain"})
	if err != nil {
		t.Fatalf("SearchCompaniesByDomain() error = %v", err)
	}
	if got, want := results[0].ID, "456"; got != want {
		t.Fatalf("id = %s, want %s", got, want)
	}
}

func TestGetContactByEmailDirectReadAndNotFound(t *testing.T) {
	var requestedRawQuery string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/crm/objects/2026-03/0-1/"+url.PathEscape("user+sales@example.com"); got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		requestedRawQuery = r.URL.RawQuery
		if r.URL.Query().Get("idProperty") != "email" {
			t.Fatalf("idProperty = %s", r.URL.Query().Get("idProperty"))
		}
		if r.URL.Query().Get("properties") != "email,firstname" {
			t.Fatalf("properties = %s", r.URL.Query().Get("properties"))
		}
		http.Error(w, "missing", http.StatusNotFound)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", httputil.NewMockClient(handler))
	_, err := client.GetContactByEmail(context.Background(), "user+sales@example.com", []string{"email", "firstname"})
	if err == nil {
		t.Fatal("GetContactByEmail() error = nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want APIError wrapper", err)
	}
	if !IsNotFound(err) {
		t.Fatal("IsNotFound() = false")
	}
	values, err := url.ParseQuery(requestedRawQuery)
	if err != nil {
		t.Fatalf("parse raw query: %v", err)
	}
	if got, want := values.Get("properties"), "email,firstname"; got != want {
		t.Fatalf("raw query properties = %s, want %s", got, want)
	}
}

func TestSearchContactsByEmailRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/crm/objects/2026-03/0-1/search"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		var body SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		filter := body.FilterGroups[0].Filters[0]
		if filter.PropertyName != "email" || filter.Operator != "EQ" || filter.Value != "person@example.com" {
			t.Fatalf("filter = %+v", filter)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"results":[{"id":789,"properties":{"email":"person@example.com"}}]}`)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", httputil.NewMockClient(handler))
	results, err := client.SearchContactsByEmail(context.Background(), "person@example.com", []string{"email"})
	if err != nil {
		t.Fatalf("SearchContactsByEmail() error = %v", err)
	}
	if got, want := results[0].ID, "789"; got != want {
		t.Fatalf("id = %s, want %s", got, want)
	}
}

func TestGetContactAssociationsParsesStringAndNumericIDs(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/crm/objects/2026-03/0-1/123"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		if got, want := r.URL.Query().Get("associations"), ObjectTypeCompany; got != want {
			t.Fatalf("associations = %s, want %s", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"123","associations":{"companies":{"results":[{"id":"456"},{"id":789}]}}}`)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", httputil.NewMockClient(handler))
	ids, err := client.GetContactAssociations(context.Background(), "123", ObjectTypeCompany)
	if err != nil {
		t.Fatalf("GetContactAssociations() error = %v", err)
	}
	if got, want := strings.Join(ids, ","), "456,789"; got != want {
		t.Fatalf("ids = %s, want %s", got, want)
	}
}

func TestDefaultAssociationPUTPaths(t *testing.T) {
	var paths []string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s", r.Method)
		}
		paths = append(paths, r.URL.EscapedPath())
		w.WriteHeader(http.StatusNoContent)
	})

	client := NewWithBaseURL("test-token", "http://hubspot.local", httputil.NewMockClient(handler))
	if err := client.AssociateContactToCompany(context.Background(), "contact/id", "company id"); err != nil {
		t.Fatalf("AssociateContactToCompany() error = %v", err)
	}
	if err := client.AssociateDealToCompany(context.Background(), "deal-id", "company-id"); err != nil {
		t.Fatalf("AssociateDealToCompany() error = %v", err)
	}
	if err := client.AssociateDealToContact(context.Background(), "deal-id", "contact-id"); err != nil {
		t.Fatalf("AssociateDealToContact() error = %v", err)
	}

	got := strings.Join(paths, ",")
	want := "/crm/v4/objects/0-1/contact%2Fid/associations/default/0-2/company%20id," +
		"/crm/v4/objects/0-3/deal-id/associations/default/0-2/company-id," +
		"/crm/v4/objects/0-3/deal-id/associations/default/0-1/contact-id"
	if got != want {
		t.Fatalf("paths = %s, want %s", got, want)
	}
}

func TestParseCRMObjectRequiresID(t *testing.T) {
	if _, err := parseCRMObject(json.RawMessage(`{"properties":{}}`), "parse test"); err == nil {
		t.Fatal("parseCRMObject() error = nil")
	}
}
