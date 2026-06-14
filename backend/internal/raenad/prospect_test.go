package raenad

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/hubspot"
)

func TestQuoteProspectsExistingContactWithAssociatedCompany(t *testing.T) {
	hs := newFakeProspectHubSpot()
	hs.contactsByEmail["jane@example.com"] = &hubspot.CRMObject{
		ID:         "contact-1",
		Properties: map[string]string{"email": "jane@example.com", "firstname": "Jane", "lastname": "Doe", "jobtitle": "CTO"},
	}
	hs.associationsByContactID["contact-1"] = []string{"company-1"}
	hs.companiesByID["company-1"] = &hubspot.CRMObject{
		ID:         "company-1",
		Properties: map[string]string{"name": "Example Spa", "domain": "example.com"},
	}

	rec := postProspect(t, hs, `{"email":"Jane@Example.com"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}
	var got prospectResponse
	decodeProspectResponse(t, rec, &got)
	if got.HubSpotCompanyID != "company-1" || deref(got.HubSpotContactID) != "contact-1" {
		t.Fatalf("unexpected ids: %#v", got)
	}
	if deref(got.Customer.Name) != "Example Spa" || deref(got.Contact.FirstName) != "Jane" || deref(got.Contact.Role) != "CTO" {
		t.Fatalf("unexpected snapshots: %#v", got)
	}
	if len(hs.searchCompanyDomains) != 0 || len(hs.createdCompanies) != 0 || len(hs.associatedContactCompany) != 0 {
		t.Fatalf("associated company path should not search/create/associate: %#v", hs)
	}
}

func TestQuoteProspectsNewContactExistingCompanyByDomain(t *testing.T) {
	hs := newFakeProspectHubSpot()
	hs.companiesByDomain["acme.test"] = []hubspot.CRMObject{{
		ID:         "company-2",
		Properties: map[string]string{"name": "ACME", "domain": "acme.test"},
	}}

	rec := postProspect(t, hs, `{"email":"lead@acme.test","first_name":"Lead","role":"Buyer"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}
	var got prospectResponse
	decodeProspectResponse(t, rec, &got)
	if got.HubSpotCompanyID != "company-2" || deref(got.HubSpotContactID) != "contact-new-1" {
		t.Fatalf("unexpected ids: %#v", got)
	}
	if len(hs.getContactEmails) != 1 || hs.getContactEmails[0] != "lead@acme.test" {
		t.Fatalf("contact lookup did not use direct email path: %#v", hs.getContactEmails)
	}
	if len(hs.createdContacts) != 1 || hs.createdContacts[0]["email"] != "lead@acme.test" || hs.createdContacts[0]["firstname"] != "Lead" {
		t.Fatalf("unexpected contact create properties: %#v", hs.createdContacts)
	}
	if len(hs.searchCompanyDomains) != 1 || hs.searchCompanyDomains[0] != "acme.test" {
		t.Fatalf("expected domain company search, got %#v", hs.searchCompanyDomains)
	}
	assertAssociated(t, hs, "contact-new-1", "company-2")
}

func TestQuoteProspectsNewContactNewCompanyByDomain(t *testing.T) {
	hs := newFakeProspectHubSpot()

	rec := postProspect(t, hs, `{"email":"owner@newco.test","full_name":"Owner Person","company_name":"NewCo"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}
	var got prospectResponse
	decodeProspectResponse(t, rec, &got)
	if got.HubSpotCompanyID != "company-new-1" || deref(got.HubSpotContactID) != "contact-new-1" {
		t.Fatalf("unexpected ids: %#v", got)
	}
	if len(hs.createdCompanies) != 1 || hs.createdCompanies[0]["domain"] != "newco.test" || hs.createdCompanies[0]["name"] != "NewCo" {
		t.Fatalf("unexpected company create properties: %#v", hs.createdCompanies)
	}
	if len(hs.createdContacts) != 1 || hs.createdContacts[0]["firstname"] != "Owner" || hs.createdContacts[0]["lastname"] != "Person" {
		t.Fatalf("full_name was not split for contact creation: %#v", hs.createdContacts)
	}
	assertAssociated(t, hs, "contact-new-1", "company-new-1")
}

func TestQuoteProspectsValidationFailures(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "empty body", body: ``, want: "invalid_payload"},
		{name: "invalid json", body: `{`, want: "invalid_payload"},
		{name: "unknown field", body: `{"email":"lead@example.com","unexpected":true}`, want: "invalid_payload"},
		{name: "missing email", body: `{"company_name":"Example"}`, want: "validation_failed"},
		{name: "invalid email", body: `{"email":"not an email"}`, want: "validation_failed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hs := newFakeProspectHubSpot()
			rec := postProspect(t, hs, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%q", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("expected %q in body, got %q", tc.want, rec.Body.String())
			}
			if len(hs.getContactEmails) != 0 || len(hs.createdContacts) != 0 || len(hs.createdCompanies) != 0 || len(hs.associatedContactCompany) != 0 {
				t.Fatalf("validation failure should not call HubSpot, got %#v", hs)
			}
		})
	}
}

func TestQuoteProspectsMissingCompanyResolution(t *testing.T) {
	hs := newFakeProspectHubSpot()

	rec := postProspect(t, hs, `{"email":"lead@gmail.com"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "validation_failed") {
		t.Fatalf("expected validation_failed, got %q", rec.Body.String())
	}
	if len(hs.createdCompanies) != 0 || len(hs.searchCompanyDomains) != 0 {
		t.Fatalf("public mailbox fallback should not search/create by domain: %#v", hs)
	}
}

func TestQuoteProspectsSanitizesHubSpotFailure(t *testing.T) {
	hs := newFakeProspectHubSpot()
	hs.getContactErr = errors.New("remote rejected raw customer payload secret@example.com")

	rec := postProspect(t, hs, `{"email":"lead@example.com","company_name":"Example"}`)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hubspot_request_failed") {
		t.Fatalf("expected stable hubspot error, got %q", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret@example.com") || strings.Contains(rec.Body.String(), "remote rejected") {
		t.Fatalf("response leaked hubspot error detail: %q", rec.Body.String())
	}
}

func TestQuoteProspectsMissingHubSpotDoesNotRequireDatabases(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{})

	req := httptest.NewRequest(http.MethodPost, "/aenad/v1/quotes/prospects", strings.NewReader(`{"email":"lead@example.com","company_name":"Example"}`))
	req = withRoles(req, "app_aenad_access")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hubspot_not_configured") {
		t.Fatalf("expected hubspot_not_configured, got %q", rec.Body.String())
	}
}

func TestSanitizeProspectErrorRedactsEmailsAndBoundsLength(t *testing.T) {
	msg := sanitizeProspectError(errors.New("remote error for jane@example.com: " + strings.Repeat("x", 700)))
	if strings.Contains(msg, "jane@example.com") {
		t.Fatalf("email was not redacted: %q", msg)
	}
	if !strings.Contains(msg, "[redacted-email]") {
		t.Fatalf("expected redaction marker, got %q", msg)
	}
	if len(msg) > maxProspectPersistedErrBytes {
		t.Fatalf("message length = %d, want <= %d", len(msg), maxProspectPersistedErrBytes)
	}
}

func postProspect(t *testing.T, hs HubSpotProspectClient, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{HubSpot: hs})
	req := httptest.NewRequest(http.MethodPost, "/aenad/v1/quotes/prospects", strings.NewReader(body))
	req = withRoles(req, "app_aenad_access")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decodeProspectResponse(t *testing.T, rec *httptest.ResponseRecorder, out *prospectResponse) {
	t.Helper()
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(out); err != nil {
		t.Fatalf("decode response: %v body=%q", err, rec.Body.String())
	}
}

func assertAssociated(t *testing.T, hs *fakeProspectHubSpot, contactID, companyID string) {
	t.Helper()
	for _, assoc := range hs.associatedContactCompany {
		if assoc.contactID == contactID && assoc.companyID == companyID {
			return
		}
	}
	t.Fatalf("expected association %s -> %s, got %#v", contactID, companyID, hs.associatedContactCompany)
}

type fakeProspectHubSpot struct {
	contactsByEmail         map[string]*hubspot.CRMObject
	associationsByContactID map[string][]string
	companiesByID           map[string]*hubspot.CRMObject
	companiesByDomain       map[string][]hubspot.CRMObject

	getContactErr    error
	createContactErr error
	associationErr   error
	getCompanyErr    error
	searchCompanyErr error
	createCompanyErr error
	associateErr     error

	getContactEmails         []string
	createdContacts          []map[string]any
	associationContactIDs    []string
	getCompanyIDs            []string
	searchCompanyDomains     []string
	createdCompanies         []map[string]any
	associatedContactCompany []struct {
		contactID string
		companyID string
	}
}

func newFakeProspectHubSpot() *fakeProspectHubSpot {
	return &fakeProspectHubSpot{
		contactsByEmail:         map[string]*hubspot.CRMObject{},
		associationsByContactID: map[string][]string{},
		companiesByID:           map[string]*hubspot.CRMObject{},
		companiesByDomain:       map[string][]hubspot.CRMObject{},
	}
}

func (f *fakeProspectHubSpot) GetContactByEmail(_ context.Context, email string, _ []string) (*hubspot.CRMObject, error) {
	f.getContactEmails = append(f.getContactEmails, email)
	if f.getContactErr != nil {
		return nil, f.getContactErr
	}
	if contact, ok := f.contactsByEmail[strings.ToLower(email)]; ok {
		return cloneCRMObject(contact), nil
	}
	return nil, &hubspot.APIError{StatusCode: http.StatusNotFound, Body: "not found"}
}

func (f *fakeProspectHubSpot) CreateContact(_ context.Context, properties map[string]any) (*hubspot.CRMObject, error) {
	f.createdContacts = append(f.createdContacts, cloneProspectAnyMap(properties))
	if f.createContactErr != nil {
		return nil, f.createContactErr
	}
	id := fmt.Sprintf("contact-new-%d", len(f.createdContacts))
	return &hubspot.CRMObject{ID: id, Properties: stringProperties(properties)}, nil
}

func (f *fakeProspectHubSpot) GetContactAssociations(_ context.Context, contactID, toObjectType string) ([]string, error) {
	f.associationContactIDs = append(f.associationContactIDs, contactID+":"+toObjectType)
	if f.associationErr != nil {
		return nil, f.associationErr
	}
	return append([]string(nil), f.associationsByContactID[contactID]...), nil
}

func (f *fakeProspectHubSpot) GetCompany(_ context.Context, companyID string, _ []string) (*hubspot.CRMObject, error) {
	f.getCompanyIDs = append(f.getCompanyIDs, companyID)
	if f.getCompanyErr != nil {
		return nil, f.getCompanyErr
	}
	company, ok := f.companiesByID[companyID]
	if !ok {
		return nil, &hubspot.APIError{StatusCode: http.StatusNotFound, Body: "not found"}
	}
	return cloneCRMObject(company), nil
}

func (f *fakeProspectHubSpot) SearchCompaniesByDomain(_ context.Context, domain string, _ []string) ([]hubspot.CRMObject, error) {
	f.searchCompanyDomains = append(f.searchCompanyDomains, domain)
	if f.searchCompanyErr != nil {
		return nil, f.searchCompanyErr
	}
	return append([]hubspot.CRMObject(nil), f.companiesByDomain[domain]...), nil
}

func (f *fakeProspectHubSpot) CreateCompany(_ context.Context, properties map[string]any) (*hubspot.CRMObject, error) {
	f.createdCompanies = append(f.createdCompanies, cloneProspectAnyMap(properties))
	if f.createCompanyErr != nil {
		return nil, f.createCompanyErr
	}
	id := fmt.Sprintf("company-new-%d", len(f.createdCompanies))
	return &hubspot.CRMObject{ID: id, Properties: stringProperties(properties)}, nil
}

func (f *fakeProspectHubSpot) AssociateContactToCompany(_ context.Context, contactID, companyID string) error {
	f.associatedContactCompany = append(f.associatedContactCompany, struct {
		contactID string
		companyID string
	}{contactID: contactID, companyID: companyID})
	return f.associateErr
}

func cloneCRMObject(obj *hubspot.CRMObject) *hubspot.CRMObject {
	if obj == nil {
		return nil
	}
	return &hubspot.CRMObject{ID: obj.ID, Properties: cloneProspectStringMap(obj.Properties)}
}

func cloneProspectStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneProspectAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func stringProperties(in map[string]any) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = fmt.Sprint(value)
	}
	return out
}
