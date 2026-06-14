package hubspot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const (
	ObjectTypeContact = "0-1"
	ObjectTypeCompany = "0-2"
	ObjectTypeDeal    = "0-3"
	ObjectTypeNote    = "0-46"

	AssocTypeContactToCompany = 279
	AssocTypeContactToDeal    = 4
	AssocTypeDealToContact    = 3
	AssocTypeDealToCompany    = 341
	AssocTypeCompanyToDeal    = 342
)

type CRMObject struct {
	ID         string            `json:"id"`
	Properties map[string]string `json:"properties"`
}

type ObjectAssociation struct {
	To    ObjectAssociationTo `json:"to"`
	Types []AssociationType   `json:"types"`
}

type ObjectAssociationTo struct {
	ID string `json:"id"`
}

func NewObjectAssociation(toID string, typeID int) ObjectAssociation {
	return ObjectAssociation{
		To: ObjectAssociationTo{ID: toID},
		Types: []AssociationType{{
			Category: "HUBSPOT_DEFINED",
			TypeID:   typeID,
		}},
	}
}

type SearchFilterGroup struct {
	Filters []SearchFilter `json:"filters"`
}

type SearchFilter struct {
	PropertyName string `json:"propertyName"`
	Operator     string `json:"operator"`
	Value        string `json:"value"`
}

type SearchRequest struct {
	FilterGroups []SearchFilterGroup `json:"filterGroups"`
	Properties   []string            `json:"properties,omitempty"`
	Limit        int                 `json:"limit,omitempty"`
	After        string              `json:"after,omitempty"`
}

type DealStage struct {
	ID        string
	Pipeline  string
	Dealstage string
}

func (c *Client) CreateDeal(ctx context.Context, properties map[string]any, associations []ObjectAssociation) (*CRMObject, error) {
	body := map[string]any{"properties": properties}
	if len(associations) > 0 {
		body["associations"] = associations
	}
	return c.createCRMObject(ctx, ObjectTypeDeal, body, "create deal")
}

func (c *Client) UpdateDeal(ctx context.Context, dealID string, properties map[string]any) (*CRMObject, error) {
	path := crmObjectPath(ObjectTypeDeal, dealID)
	resp, err := c.Patch(ctx, path, map[string]any{"properties": properties})
	if err != nil {
		return nil, fmt.Errorf("update deal: %w", err)
	}
	return parseCRMObject(resp, "parse update deal response")
}

func (c *Client) GetDealStage(ctx context.Context, dealID string) (*DealStage, error) {
	path := crmObjectPath(ObjectTypeDeal, dealID) + crmQuery(url.Values{
		"properties": {strings.Join([]string{"pipeline", "dealstage"}, ",")},
	})
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get deal stage: %w", err)
	}
	obj, err := parseCRMObject(resp, "parse deal stage response")
	if err != nil {
		return nil, err
	}
	return &DealStage{
		ID:        obj.ID,
		Pipeline:  obj.Properties["pipeline"],
		Dealstage: obj.Properties["dealstage"],
	}, nil
}

func (c *Client) CreateCompany(ctx context.Context, properties map[string]any) (*CRMObject, error) {
	return c.createCRMObject(ctx, ObjectTypeCompany, map[string]any{"properties": properties}, "create company")
}

func (c *Client) GetCompany(ctx context.Context, companyID string, properties []string) (*CRMObject, error) {
	path := crmObjectPath(ObjectTypeCompany, companyID) + propertiesQuery(properties)
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get company: %w", err)
	}
	return parseCRMObject(resp, "parse company response")
}

func (c *Client) SearchCompaniesByDomain(ctx context.Context, domain string, properties []string) ([]CRMObject, error) {
	return c.searchCRMObjects(ctx, ObjectTypeCompany, SearchRequest{
		FilterGroups: []SearchFilterGroup{{
			Filters: []SearchFilter{{
				PropertyName: "domain",
				Operator:     "EQ",
				Value:        domain,
			}},
		}},
		Properties: properties,
		Limit:      200,
	}, "search companies by domain")
}

func (c *Client) CreateContact(ctx context.Context, properties map[string]any) (*CRMObject, error) {
	return c.createCRMObject(ctx, ObjectTypeContact, map[string]any{"properties": properties}, "create contact")
}

func (c *Client) GetContactByEmail(ctx context.Context, email string, properties []string) (*CRMObject, error) {
	values := url.Values{"idProperty": {"email"}}
	if len(properties) > 0 {
		values.Set("properties", strings.Join(properties, ","))
	}
	path := crmObjectPath(ObjectTypeContact, email) + crmQuery(values)
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get contact by email: %w", err)
	}
	return parseCRMObject(resp, "parse contact response")
}

func (c *Client) SearchContactsByEmail(ctx context.Context, email string, properties []string) ([]CRMObject, error) {
	return c.searchCRMObjects(ctx, ObjectTypeContact, SearchRequest{
		FilterGroups: []SearchFilterGroup{{
			Filters: []SearchFilter{{
				PropertyName: "email",
				Operator:     "EQ",
				Value:        email,
			}},
		}},
		Properties: properties,
		Limit:      200,
	}, "search contacts by email")
}

func (c *Client) GetContactAssociations(ctx context.Context, contactID, toObjectType string) ([]string, error) {
	path := crmObjectPath(ObjectTypeContact, contactID) + crmQuery(url.Values{
		"associations": {toObjectType},
	})
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get contact associations: %w", err)
	}
	ids, err := parseAssociationIDs(resp, toObjectType)
	if err != nil {
		return nil, fmt.Errorf("parse contact associations: %w", err)
	}
	return ids, nil
}

func (c *Client) AssociateContactToCompany(ctx context.Context, contactID, companyID string) error {
	return c.associateDefault(ctx, ObjectTypeContact, contactID, ObjectTypeCompany, companyID, "associate contact to company")
}

func (c *Client) AssociateDealToCompany(ctx context.Context, dealID, companyID string) error {
	return c.associateDefault(ctx, ObjectTypeDeal, dealID, ObjectTypeCompany, companyID, "associate deal to company")
}

func (c *Client) AssociateDealToContact(ctx context.Context, dealID, contactID string) error {
	return c.associateDefault(ctx, ObjectTypeDeal, dealID, ObjectTypeContact, contactID, "associate deal to contact")
}

func (c *Client) createCRMObject(ctx context.Context, objectType string, body map[string]any, operation string) (*CRMObject, error) {
	resp, err := c.Post(ctx, "/crm/objects/2026-03/"+url.PathEscape(objectType), body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	return parseCRMObject(resp, "parse "+operation+" response")
}

func (c *Client) searchCRMObjects(ctx context.Context, objectType string, req SearchRequest, operation string) ([]CRMObject, error) {
	resp, err := c.Post(ctx, "/crm/objects/2026-03/"+url.PathEscape(objectType)+"/search", req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	var result struct {
		Results []crmObjectResponse `json:"results"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse %s response: %w", operation, err)
	}
	objects := make([]CRMObject, 0, len(result.Results))
	for _, raw := range result.Results {
		obj, err := raw.crmObject()
		if err != nil {
			return nil, fmt.Errorf("parse %s result: %w", operation, err)
		}
		objects = append(objects, *obj)
	}
	return objects, nil
}

func (c *Client) associateDefault(ctx context.Context, fromObjectType, fromObjectID, toObjectType, toObjectID, operation string) error {
	path := fmt.Sprintf(
		"/crm/v4/objects/%s/%s/associations/default/%s/%s",
		url.PathEscape(fromObjectType),
		url.PathEscape(fromObjectID),
		url.PathEscape(toObjectType),
		url.PathEscape(toObjectID),
	)
	if _, err := c.Put(ctx, path, nil); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

func crmObjectPath(objectType, objectID string) string {
	return fmt.Sprintf(
		"/crm/objects/2026-03/%s/%s",
		url.PathEscape(objectType),
		url.PathEscape(objectID),
	)
}

func propertiesQuery(properties []string) string {
	if len(properties) == 0 {
		return ""
	}
	return crmQuery(url.Values{"properties": {strings.Join(properties, ",")}})
}

func crmQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	return "?" + values.Encode()
}

type crmObjectResponse struct {
	ID         json.RawMessage            `json:"id"`
	Properties map[string]json.RawMessage `json:"properties"`
}

func parseCRMObject(raw json.RawMessage, errorPrefix string) (*CRMObject, error) {
	var result crmObjectResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("%s: %w", errorPrefix, err)
	}
	obj, err := result.crmObject()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errorPrefix, err)
	}
	return obj, nil
}

func (r crmObjectResponse) crmObject() (*CRMObject, error) {
	id := rawJSONID(r.ID)
	if id == "" {
		return nil, fmt.Errorf("missing id")
	}
	obj := &CRMObject{
		ID:         id,
		Properties: make(map[string]string, len(r.Properties)),
	}
	for key, raw := range r.Properties {
		obj.Properties[key] = rawJSONValue(raw)
	}
	return obj, nil
}

func rawJSONValue(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return fmt.Sprintf("%t", b)
	}
	return string(raw)
}

func parseAssociationIDs(raw json.RawMessage, toObjectType string) ([]string, error) {
	var result struct {
		Associations map[string]struct {
			Results []struct {
				ID json.RawMessage `json:"id"`
			} `json:"results"`
		} `json:"associations"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	if len(result.Associations) == 0 {
		return nil, nil
	}
	candidates := associationKeys(toObjectType)
	for _, key := range candidates {
		if association, ok := result.Associations[key]; ok {
			return associationResultIDs(association.Results)
		}
	}
	if len(result.Associations) == 1 {
		for _, association := range result.Associations {
			return associationResultIDs(association.Results)
		}
	}
	return nil, nil
}

func associationResultIDs(results []struct {
	ID json.RawMessage `json:"id"`
}) ([]string, error) {
	ids := make([]string, 0, len(results))
	for _, result := range results {
		id := rawJSONID(result.ID)
		if id == "" {
			return nil, fmt.Errorf("missing association id")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func associationKeys(objectType string) []string {
	switch objectType {
	case ObjectTypeContact:
		return []string{ObjectTypeContact, "contacts"}
	case ObjectTypeCompany:
		return []string{ObjectTypeCompany, "companies"}
	case ObjectTypeDeal:
		return []string{ObjectTypeDeal, "deals"}
	default:
		return []string{objectType}
	}
}
