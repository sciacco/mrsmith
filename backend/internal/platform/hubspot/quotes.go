package hubspot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Verified association type IDs from Appsmith source code:
const (
	AssocTypeQuoteToTemplate = 286
	AssocTypeQuoteToDeal     = 64
	AssocTypeQuoteToCompany  = 71
	AssocTypeLineItemToQuote = 68
)

type Association struct {
	To    AssociationTo     `json:"to"`
	Types []AssociationType `json:"types"`
}

type AssociationTo struct {
	ID int64 `json:"id"`
}

type AssociationType struct {
	Category string `json:"associationCategory"`
	TypeID   int    `json:"associationTypeId"`
}

func NewAssociation(toID int64, typeID int) Association {
	return Association{
		To: AssociationTo{ID: toID},
		Types: []AssociationType{{
			Category: "HUBSPOT_DEFINED",
			TypeID:   typeID,
		}},
	}
}

func (c *Client) CreateQuote(ctx context.Context, properties map[string]any, associations []Association) (int64, error) {
	body := map[string]any{
		"properties":   properties,
		"associations": associations,
	}
	resp, err := c.Post(ctx, "/crm/v3/objects/quote", body)
	if err != nil {
		return 0, fmt.Errorf("create quote: %w", err)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, fmt.Errorf("parse create quote response: %w", err)
	}
	var id int64
	fmt.Sscanf(result.ID, "%d", &id)
	return id, nil
}

func (c *Client) UpdateQuote(ctx context.Context, quoteID int64, properties map[string]any) error {
	body := map[string]any{"properties": properties}
	_, err := c.Patch(ctx, fmt.Sprintf("/crm/v3/objects/quotes/%d", quoteID), body)
	return err
}

func (c *Client) DeleteQuote(ctx context.Context, quoteID int64) error {
	return c.Delete(ctx, fmt.Sprintf("/crm/v3/objects/quotes/%d", quoteID))
}

type QuoteStatus struct {
	Properties map[string]string `json:"properties"`
}

func (c *Client) GetQuoteStatus(ctx context.Context, quoteID int64) (*QuoteStatus, error) {
	properties := []string{
		"hs_status",
		"hs_language",
		"hs_pdf_download_link",
		"hs_quote_link",
		"hs_sign_status",
		"hs_esign_enabled",
		"hs_esign_num_signers_completed",
		"hs_esign_num_signers_required",
	}
	path := fmt.Sprintf(
		"/crm/v3/objects/quotes/%d?properties=%s",
		quoteID,
		url.QueryEscape(strings.Join(properties, ",")),
	)
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get quote status: %w", err)
	}
	var result QuoteStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse quote status response: %w", err)
	}
	return &result, nil
}

func (c *Client) GetQuoteLineItemIDs(ctx context.Context, quoteID int64) ([]int64, error) {
	resp, err := c.Get(ctx, fmt.Sprintf("/crm/v3/objects/quotes/%d?associations=line_items", quoteID))
	if err != nil {
		return nil, fmt.Errorf("get quote associations: %w", err)
	}
	var result struct {
		Associations map[string]struct {
			Results []struct {
				ID json.Number `json:"id"`
			} `json:"results"`
		} `json:"associations"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse associations: %w", err)
	}
	lineItems, ok := result.Associations["line items"]
	if !ok {
		return nil, nil
	}
	ids := make([]int64, 0, len(lineItems.Results))
	for _, r := range lineItems.Results {
		id, _ := r.ID.Int64()
		ids = append(ids, id)
	}
	return ids, nil
}

func (c *Client) CreateLineItem(ctx context.Context, properties map[string]any, associations []Association) (int64, error) {
	body := map[string]any{
		"properties":   properties,
		"associations": associations,
	}
	resp, err := c.Post(ctx, "/crm/v3/objects/line_item", body)
	if err != nil {
		return 0, fmt.Errorf("create line item: %w", err)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, fmt.Errorf("parse line item response: %w", err)
	}
	var id int64
	fmt.Sscanf(result.ID, "%d", &id)
	return id, nil
}

func (c *Client) UpdateLineItem(ctx context.Context, itemID int64, properties map[string]any, associations []Association) error {
	body := map[string]any{
		"properties":   properties,
		"associations": associations,
	}
	_, err := c.Patch(ctx, fmt.Sprintf("/crm/v3/objects/line_item/%d", itemID), body)
	return err
}

func (c *Client) DeleteLineItem(ctx context.Context, itemID int64) error {
	return c.Delete(ctx, fmt.Sprintf("/crm/v3/objects/line_item/%d", itemID))
}

func (c *Client) AssociateQuoteToTemplate(ctx context.Context, quoteID int64, templateID string) error {
	path := fmt.Sprintf("/crm/v4/objects/quotes/%d/associations/default/quote_template/%s", quoteID, templateID)
	_, err := c.Put(ctx, path, nil)
	return err
}
