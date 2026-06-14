package raenad

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/hubspot"
)

const maxProspectPersistedErrBytes = 512

var prospectEmailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)

// HubSpotProspectClient is the HubSpot subset needed by the live prospect
// selection flow. Keeping it narrow lets handler tests use a fake client.
type HubSpotProspectClient interface {
	GetContactByEmail(ctx context.Context, email string, properties []string) (*hubspot.CRMObject, error)
	CreateContact(ctx context.Context, properties map[string]any) (*hubspot.CRMObject, error)
	GetContactAssociations(ctx context.Context, contactID, toObjectType string) ([]string, error)
	GetCompany(ctx context.Context, companyID string, properties []string) (*hubspot.CRMObject, error)
	SearchCompaniesByDomain(ctx context.Context, domain string, properties []string) ([]hubspot.CRMObject, error)
	CreateCompany(ctx context.Context, properties map[string]any) (*hubspot.CRMObject, error)
	AssociateContactToCompany(ctx context.Context, contactID, companyID string) error
}

var prospectContactProperties = []string{"email", "firstname", "lastname", "jobtitle"}
var prospectCompanyProperties = []string{"name", "domain"}

type prospectRequest struct {
	Email       *string               `json:"email"`
	FirstName   *string               `json:"first_name"`
	LastName    *string               `json:"last_name"`
	FullName    *string               `json:"full_name"`
	Role        *string               `json:"role"`
	CompanyName *string               `json:"company_name"`
	Domain      *string               `json:"domain"`
	Customer    customerSnapshotInput `json:"customer"`
}

type prospectResponse struct {
	HubSpotCompanyID string           `json:"hubspot_company_id"`
	HubSpotContactID *string          `json:"hubspot_contact_id,omitempty"`
	Customer         customerSnapshot `json:"customer"`
	Contact          contactSnapshot  `json:"contact"`
}

func (h *Handler) handleQuoteProspects(w http.ResponseWriter, r *http.Request) {
	if !h.requireHubSpot(w) {
		return
	}

	req, ok := h.decodeProspectRequest(w, r)
	if !ok {
		return
	}
	email, ok := normalizeProspectEmail(req.Email)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "validation_failed")
		return
	}

	req = normalizeProspectRequest(req)
	contact, err := h.resolveHubSpotProspectContact(r.Context(), email, req)
	if err != nil {
		h.hubSpotProspectFailure(w, "resolve_contact", err)
		return
	}
	company, associated, err := h.resolveHubSpotProspectCompany(r.Context(), contact.ID, email, req)
	if err != nil {
		if errorsIsValidation(err) {
			httputil.Error(w, http.StatusBadRequest, "validation_failed")
			return
		}
		h.hubSpotProspectFailure(w, "resolve_company", err)
		return
	}
	if !associated {
		if err := h.deps.HubSpot.AssociateContactToCompany(r.Context(), contact.ID, company.ID); err != nil {
			h.hubSpotProspectFailure(w, "associate_contact_company", err)
			return
		}
	}

	contactID := strings.TrimSpace(contact.ID)
	httputil.JSON(w, http.StatusOK, prospectResponse{
		HubSpotCompanyID: strings.TrimSpace(company.ID),
		HubSpotContactID: &contactID,
		Customer:         prospectCustomerSnapshot(req, company),
		Contact:          prospectContactSnapshot(req, contact, email),
	})
}

func (h *Handler) decodeProspectRequest(w http.ResponseWriter, r *http.Request) (prospectRequest, bool) {
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_payload")
		return prospectRequest{}, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var req prospectRequest
	if err := decoder.Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_payload")
		return prospectRequest{}, false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		httputil.Error(w, http.StatusBadRequest, "invalid_payload")
		return prospectRequest{}, false
	}
	return req, true
}

func (h *Handler) resolveHubSpotProspectContact(ctx context.Context, email string, req prospectRequest) (*hubspot.CRMObject, error) {
	contact, err := h.deps.HubSpot.GetContactByEmail(ctx, email, prospectContactProperties)
	if err == nil {
		return contact, nil
	}
	if !hubspot.IsNotFound(err) {
		return nil, err
	}
	return h.deps.HubSpot.CreateContact(ctx, prospectContactCreateProperties(email, req))
}

func (h *Handler) resolveHubSpotProspectCompany(ctx context.Context, contactID, email string, req prospectRequest) (*hubspot.CRMObject, bool, error) {
	associationIDs, err := h.deps.HubSpot.GetContactAssociations(ctx, contactID, hubspot.ObjectTypeCompany)
	if err != nil {
		return nil, false, err
	}
	for _, companyID := range associationIDs {
		companyID = strings.TrimSpace(companyID)
		if companyID == "" {
			continue
		}
		company, err := h.deps.HubSpot.GetCompany(ctx, companyID, prospectCompanyProperties)
		if err != nil {
			return nil, false, err
		}
		return company, true, nil
	}

	domain := prospectDomain(req.Domain, email)
	if domain != "" {
		companies, err := h.deps.HubSpot.SearchCompaniesByDomain(ctx, domain, prospectCompanyProperties)
		if err != nil {
			return nil, false, err
		}
		for i := range companies {
			if strings.TrimSpace(companies[i].ID) != "" {
				return &companies[i], false, nil
			}
		}
	}

	properties := prospectCompanyCreateProperties(req, domain)
	if len(properties) == 0 {
		return nil, false, prospectValidationError("company resolution requires an associated company, non-public email domain, supplied domain, or company name")
	}
	company, err := h.deps.HubSpot.CreateCompany(ctx, properties)
	if err != nil {
		return nil, false, err
	}
	return company, false, nil
}

func (h *Handler) hubSpotProspectFailure(w http.ResponseWriter, operation string, err error) {
	h.logger.Warn(
		"hubspot prospect request failed",
		"operation", "raenad_hubspot_prospect_"+operation,
		"error", sanitizeProspectError(err),
	)
	httputil.Error(w, http.StatusBadGateway, "hubspot_request_failed")
}

func normalizeProspectRequest(req prospectRequest) prospectRequest {
	req.Email = trimmedOrNil(req.Email)
	req.FirstName = trimmedOrNil(req.FirstName)
	req.LastName = trimmedOrNil(req.LastName)
	req.FullName = trimmedOrNil(req.FullName)
	req.Role = trimmedOrNil(req.Role)
	req.CompanyName = trimmedOrNil(req.CompanyName)
	req.Domain = trimmedOrNil(req.Domain)
	req.Customer = normalizeCustomerInput(req.Customer)
	return req
}

func normalizeProspectEmail(value *string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(deref(value)))
	if email == "" || strings.ContainsAny(email, " \t\r\n") {
		return "", false
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") {
		return "", false
	}
	return email, true
}

func prospectContactCreateProperties(email string, req prospectRequest) map[string]any {
	firstName := deref(req.FirstName)
	lastName := deref(req.LastName)
	if firstName == "" && lastName == "" {
		firstName, lastName = splitFullName(deref(req.FullName))
	}
	properties := map[string]any{"email": email}
	if firstName != "" {
		properties["firstname"] = firstName
	}
	if lastName != "" {
		properties["lastname"] = lastName
	}
	if role := deref(req.Role); role != "" {
		properties["jobtitle"] = role
	}
	return properties
}

func prospectCompanyCreateProperties(req prospectRequest, domain string) map[string]any {
	name := firstProspectValue(req.CompanyName, req.Customer.Name)
	if name == "" {
		name = domain
	}
	properties := make(map[string]any)
	if name != "" {
		properties["name"] = name
	}
	if domain != "" {
		properties["domain"] = domain
	}
	return properties
}

func prospectCustomerSnapshot(req prospectRequest, company *hubspot.CRMObject) customerSnapshot {
	customer := normalizeCustomerInput(req.Customer)
	if customer.Name == nil {
		customer.Name = firstProspectStringPtr(companyProperty(company, "name"), deref(req.CompanyName), companyProperty(company, "domain"))
	}
	return customerSnapshot{customerSnapshotInput: customer}
}

func prospectContactSnapshot(req prospectRequest, contact *hubspot.CRMObject, email string) contactSnapshot {
	firstName := firstProspectStringPtr(companyProperty(contact, "firstname"), deref(req.FirstName))
	lastName := firstProspectStringPtr(companyProperty(contact, "lastname"), deref(req.LastName))
	fullName := req.FullName
	if fullName == nil {
		fullName = fullNamePtr(firstName, lastName)
	}
	role := firstProspectStringPtr(companyProperty(contact, "jobtitle"), deref(req.Role))
	contactEmail := firstProspectStringPtr(companyProperty(contact, "email"), email)
	return contactSnapshot{contactSnapshotInput: contactSnapshotInput{
		FirstName: firstName,
		LastName:  lastName,
		FullName:  fullName,
		Email:     contactEmail,
		Role:      role,
	}}
}

func prospectDomain(value *string, email string) string {
	if domain := normalizeDomain(deref(value)); domain != "" {
		return domain
	}
	_, domain, ok := strings.Cut(email, "@")
	if !ok {
		return ""
	}
	domain = normalizeDomain(domain)
	if domain == "" || commonPublicMailboxDomain(domain) {
		return ""
	}
	return domain
}

func normalizeDomain(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.TrimPrefix(value, "mailto:")
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		if parsed, err := url.Parse(value); err == nil {
			value = parsed.Hostname()
		}
	}
	if strings.Contains(value, "@") {
		_, value, _ = strings.Cut(value, "@")
	}
	if i := strings.IndexByte(value, '/'); i >= 0 {
		value = value[:i]
	}
	if i := strings.IndexByte(value, ':'); i >= 0 {
		value = value[:i]
	}
	value = strings.Trim(value, ".")
	value = strings.TrimPrefix(value, "www.")
	if value == "" || !strings.Contains(value, ".") {
		return ""
	}
	return value
}

func commonPublicMailboxDomain(domain string) bool {
	switch strings.ToLower(strings.TrimSpace(domain)) {
	case "gmail.com", "googlemail.com", "yahoo.com", "yahoo.it", "outlook.com", "hotmail.com", "live.com", "msn.com",
		"icloud.com", "me.com", "mac.com", "aol.com", "proton.me", "protonmail.com", "pm.me", "mail.com",
		"gmx.com", "gmx.net", "zoho.com", "yandex.com", "libero.it", "alice.it", "tin.it", "virgilio.it",
		"tiscali.it", "fastwebnet.it", "email.it":
		return true
	default:
		return false
	}
}

func companyProperty(obj *hubspot.CRMObject, name string) string {
	if obj == nil {
		return ""
	}
	return strings.TrimSpace(obj.Properties[name])
}

func firstProspectValue(values ...*string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(deref(value)); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstProspectStringPtr(values ...string) *string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return &trimmed
		}
	}
	return nil
}

func splitFullName(fullName string) (string, string) {
	fields := strings.Fields(fullName)
	if len(fields) == 0 {
		return "", ""
	}
	if len(fields) == 1 {
		return fields[0], ""
	}
	return fields[0], strings.Join(fields[1:], " ")
}

func fullNamePtr(firstName, lastName *string) *string {
	fullName := strings.TrimSpace(strings.Join([]string{deref(firstName), deref(lastName)}, " "))
	if fullName == "" {
		return nil
	}
	return &fullName
}

type prospectValidationError string

func (e prospectValidationError) Error() string {
	return string(e)
}

func errorsIsValidation(err error) bool {
	_, ok := err.(prospectValidationError)
	return ok
}

func sanitizeProspectError(err error) string {
	msg := strings.TrimSpace(fmt.Sprint(err))
	if msg == "" {
		return "hubspot request failed"
	}
	msg = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		default:
			if r < 32 {
				return -1
			}
			return r
		}
	}, msg)
	msg = prospectEmailPattern.ReplaceAllString(msg, "[redacted-email]")
	msg = strings.Join(strings.Fields(msg), " ")
	if len(msg) > maxProspectPersistedErrBytes {
		msg = msg[:maxProspectPersistedErrBytes]
	}
	return msg
}
