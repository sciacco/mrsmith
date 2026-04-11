package quotes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/hubspot"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const paymentMethodLabelQuery = `SELECT desc_pagamento FROM loader.erp_metodi_pagamento WHERE cod_pagamento = $1`

type publishStep struct {
	Step   int    `json:"step"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (h *Handler) handlePublish(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	if !h.requireHS(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}

	ctx := r.Context()
	steps := []publishStep{}

	addStep := func(step int, name, status string) {
		steps = append(steps, publishStep{Step: step, Name: name, Status: status})
	}
	failStep := func(step int, name, errMsg string) {
		steps = append(steps, publishStep{Step: step, Name: name, Status: "error", Error: errMsg})
		httputil.JSON(w, http.StatusOK, map[string]any{"success": false, "steps": steps})
	}

	// Step 1: Save
	if err := h.persistQuoteForPublish(ctx, quoteID); err != nil {
		failStep(1, "save", err.Error())
		return
	}
	addStep(1, "save", "completed")

	// Step 2: Validate required products
	invalidCount, err := h.countInvalidRequiredGroups(ctx, quoteID)
	if err != nil {
		failStep(2, "validate", fmt.Sprintf("validation query failed: %v", err))
		return
	}
	if invalidCount > 0 {
		failStep(2, "validate", fmt.Sprintf("%d required product groups are not configured", invalidCount))
		return
	}
	addStep(2, "validate", "completed")

	// Load quote data for HS
	var q struct {
		QuoteNumber       string
		CustomerID        *int64
		HSDealID          *int64
		HSQuoteID         *int64
		Template          *string
		Owner             *string
		DocumentDate      *string
		Notes             *string
		Status            string
		Description       string
		BillMonths        int
		InitialTermMonths int
		NextTermMonths    int
		DeliveredInDays   int
		NrcChargeTime     int
		PaymentMethod     *string
	}
	err = h.db.QueryRowContext(ctx, `
		SELECT quote_number, customer_id, hs_deal_id, hs_quote_id, template, owner,
		       document_date, notes, status, COALESCE(description, ''),
		       bill_months, initial_term_months, next_term_months, delivered_in_days,
		       nrc_charge_time, payment_method
		FROM quotes.quote WHERE id = $1`, quoteID).Scan(
		&q.QuoteNumber, &q.CustomerID, &q.HSDealID, &q.HSQuoteID, &q.Template, &q.Owner,
		&q.DocumentDate, &q.Notes, &q.Status, &q.Description,
		&q.BillMonths, &q.InitialTermMonths, &q.NextTermMonths, &q.DeliveredInDays,
		&q.NrcChargeTime, &q.PaymentMethod)
	if err != nil {
		failStep(3, "hubspot_quote", fmt.Sprintf("load quote: %v", err))
		return
	}

	// Lookup template metadata for T&C
	var templateType string
	var isColo bool
	var lang string
	if q.Template != nil {
		_ = h.db.QueryRowContext(ctx,
			`SELECT COALESCE(template_type, 'standard'), COALESCE(is_colo, false), lang
			 FROM quotes.template WHERE template_id = $1`, *q.Template).Scan(&templateType, &isColo, &lang)
	}

	// Lookup payment method label
	paymentLabel := "402"
	if q.PaymentMethod != nil {
		_ = h.db.QueryRowContext(ctx,
			paymentMethodLabelQuery,
			*q.PaymentMethod).Scan(&paymentLabel)
	}

	// Generate T&C
	tec := GenerateTermsAndConditions(templateType, isColo, lang, paymentLabel,
		q.InitialTermMonths, q.NextTermMonths, q.DeliveredInDays, q.NrcChargeTime, q.BillMonths,
		ptrStr(q.Notes))

	// Build HS properties
	expiryDate := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	if q.DocumentDate != nil {
		if t, err := time.Parse("2006-01-02", *q.DocumentDate); err == nil {
			expiryDate = t.AddDate(0, 0, 30).Format("2006-01-02")
		}
	}

	hsStatus := "APPROVED"
	if ptrStr(q.Notes) != "" {
		hsStatus = "PENDING_APPROVAL"
	}

	hsProps := map[string]any{
		"hs_title":           q.QuoteNumber,
		"hs_expiration_date": expiryDate,
		"hs_status":          hsStatus,
		"hs_terms":           tec,
	}

	// Lookup owner email
	if q.Owner != nil {
		var email string
		_ = h.db.QueryRowContext(ctx,
			`SELECT email FROM loader.hubs_owner WHERE id = $1`, *q.Owner).Scan(&email)
		if email != "" {
			hsProps["hs_sender_email"] = email
		}
	}

	// Step 3: Create or update HS quote
	var hsQuoteID int64
	finalHSProps := cloneMap(hsProps)
	if q.HSQuoteID != nil {
		hsQuoteID = *q.HSQuoteID
		remote, err := h.hs.GetQuoteStatus(ctx, hsQuoteID)
		if err != nil {
			failStep(3, "hubspot_quote", fmt.Sprintf("load current HubSpot quote status: %v", err))
			return
		}

		if remoteQuoteLocked(remote) {
			currentStatus := remoteQuoteState(remote)
			if err := h.hs.UpdateQuote(ctx, hsQuoteID, map[string]any{"hs_status": "DRAFT"}); err != nil {
				logging.FromContext(ctx).Warn("failed to unlock hubspot quote before republish",
					"component", "quotes",
					"operation", "publish_unlock_quote",
					"quote_id", quoteID,
					"hs_quote_id", hsQuoteID,
					"remote_hs_status", currentStatus,
					"remote_hs_locked", true,
					"error", err,
				)
				failStep(3, "hubspot_quote", fmt.Sprintf("unlock HubSpot quote from %s: %v", currentStatus, err))
				return
			}
		}
	} else {
		createProps := cloneMap(hsProps)
		createProps["hs_status"] = "DRAFT"

		// Build associations
		associations := []hubspot.Association{}
		if q.Template != nil {
			tid, _ := strconv.ParseInt(*q.Template, 10, 64)
			associations = append(associations, hubspot.NewAssociation(tid, hubspot.AssocTypeQuoteToTemplate))
		}
		if q.HSDealID != nil {
			associations = append(associations, hubspot.NewAssociation(*q.HSDealID, hubspot.AssocTypeQuoteToDeal))
		}
		if q.CustomerID != nil {
			associations = append(associations, hubspot.NewAssociation(*q.CustomerID, hubspot.AssocTypeQuoteToCompany))
		}

		hsQuoteID, err = h.hs.CreateQuote(ctx, createProps, associations)
		if err != nil {
			failStep(3, "hubspot_quote", err.Error())
			return
		}

		// Store hs_quote_id
		_, _ = h.db.ExecContext(ctx,
			`UPDATE quotes.quote SET hs_quote_id = $1 WHERE id = $2`, hsQuoteID, quoteID)
	}

	// Re-associate template
	if q.Template != nil {
		_ = h.hs.AssociateQuoteToTemplate(ctx, hsQuoteID, *q.Template)
	}
	addStep(3, "hubspot_quote", "completed")

	// Step 4: Sync line items
	rows, err := h.db.QueryContext(ctx,
		`SELECT qr.id, qr.kit_id, k.internal_name, qr.nrc_row, qr.mrc_row,
		        qr.hs_line_item_id, qr.hs_line_item_nrc
		 FROM quotes.quote_rows qr
		 LEFT JOIN products.kit k ON k.id = qr.kit_id
		 WHERE qr.quote_id = $1 ORDER BY qr.position`, quoteID)
	if err != nil {
		failStep(4, "line_items", err.Error())
		return
	}

	type rowData struct {
		ID            int
		KitID         int
		InternalName  string
		NrcRow        float64
		MrcRow        float64
		HsLineItemID  *int64
		HsLineItemNrc *int64
	}
	var rowList []rowData
	for rows.Next() {
		var rd rowData
		if err := rows.Scan(&rd.ID, &rd.KitID, &rd.InternalName, &rd.NrcRow, &rd.MrcRow,
			&rd.HsLineItemID, &rd.HsLineItemNrc); err != nil {
			rows.Close()
			failStep(4, "line_items", err.Error())
			return
		}
		rowList = append(rowList, rd)
	}
	rows.Close()

	lineAssoc := []hubspot.Association{
		hubspot.NewAssociation(hsQuoteID, hubspot.AssocTypeLineItemToQuote),
	}

	for _, rd := range rowList {
		// MRC line item
		mrcProps := map[string]any{
			"name":                      rd.InternalName + " (MRC)",
			"hs_sku":                    fmt.Sprintf("MRC-%d", rd.KitID),
			"recurringbillingfrequency": "monthly",
			"price":                     rd.MrcRow,
			"quantity":                  1,
		}

		if rd.HsLineItemID != nil {
			_ = h.hs.UpdateLineItem(ctx, *rd.HsLineItemID, mrcProps, lineAssoc)
		} else {
			itemID, err := h.hs.CreateLineItem(ctx, mrcProps, lineAssoc)
			if err == nil {
				_, _ = h.db.ExecContext(ctx,
					`UPDATE quotes.quote_rows SET hs_line_item_id = $1 WHERE id = $2`, itemID, rd.ID)
			}
		}

		// NRC line item
		nrcProps := map[string]any{
			"name":     rd.InternalName + " (NRC)",
			"hs_sku":   fmt.Sprintf("NRC-%d", rd.KitID),
			"price":    rd.NrcRow,
			"quantity": 1,
		}

		if rd.HsLineItemNrc != nil {
			_ = h.hs.UpdateLineItem(ctx, *rd.HsLineItemNrc, nrcProps, lineAssoc)
		} else {
			itemID, err := h.hs.CreateLineItem(ctx, nrcProps, lineAssoc)
			if err == nil {
				_, _ = h.db.ExecContext(ctx,
					`UPDATE quotes.quote_rows SET hs_line_item_nrc = $1 WHERE id = $2`, itemID, rd.ID)
			}
		}
	}
	addStep(4, "line_items", "completed")

	// Step 5: Update status
	if err := h.hs.UpdateQuote(ctx, hsQuoteID, finalHSProps); err != nil {
		failStep(5, "update_status", err.Error())
		return
	}

	_, err = h.db.ExecContext(ctx,
		`UPDATE quotes.quote SET status = $1, date_sent = NOW() WHERE id = $2`, hsStatus, quoteID)
	if err != nil {
		failStep(5, "update_status", err.Error())
		return
	}
	addStep(5, "update_status", "completed")

	// Re-fetch quote
	var updatedQuote json.RawMessage
	_ = h.db.QueryRowContext(ctx,
		`SELECT row_to_json(q) FROM quotes.quote q WHERE q.id = $1`, quoteID).Scan(&updatedQuote)

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"steps":   steps,
		"quote":   updatedQuote,
	})
}

func (h *Handler) handlePublishPrecheck(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}

	invalidCount, err := h.countInvalidRequiredGroups(r.Context(), quoteID)
	if err != nil {
		h.dbFailure(w, r, "publish_precheck", err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"invalid_required_groups":       invalidCount,
		"has_missing_required_products": invalidCount > 0,
	})
}

func (h *Handler) countInvalidRequiredGroups(ctx context.Context, quoteID int) (int, error) {
	var invalidCount int
	err := h.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM quotes.v_quote_rows_products vqrp
		WHERE vqrp.quote_row_id IN (
			SELECT id FROM quotes.quote_rows WHERE quote_id = $1
		)
		AND vqrp.required = 1
		AND NOT EXISTS (
			SELECT 1
			FROM jsonb_array_elements(vqrp.riga::jsonb) AS j(obj)
			WHERE obj->>'included' = 'true'
		)`, quoteID).Scan(&invalidCount)
	return invalidCount, err
}

func (h *Handler) persistQuoteForPublish(ctx context.Context, quoteID int) error {
	var currentJSON json.RawMessage
	if err := h.db.QueryRowContext(ctx,
		`SELECT row_to_json(q) FROM quotes.quote q WHERE q.id = $1`, quoteID).Scan(&currentJSON); err != nil {
		return fmt.Errorf("load quote before publish: %w", err)
	}

	var result json.RawMessage
	if err := h.db.QueryRowContext(ctx,
		`SELECT quotes.upd_quote_head($1::json)`, string(currentJSON)).Scan(&result); err != nil {
		return fmt.Errorf("persist quote before publish: %w", err)
	}

	var procResult struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(result, &procResult); err != nil {
		return fmt.Errorf("parse publish save response: %w", err)
	}
	if procResult.Status == "ERROR" {
		return fmt.Errorf("%s", procResult.Message)
	}
	return nil
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func remoteQuoteState(remote *hubspot.QuoteStatus) string {
	if remote == nil {
		return ""
	}
	return strings.TrimSpace(remote.Properties["hs_status"])
}

func remoteQuoteLocked(remote *hubspot.QuoteStatus) bool {
	if remote == nil {
		return false
	}
	if locked, ok := parseHubSpotBool(remote.Properties["hs_locked"]); ok {
		return locked
	}
	switch remoteQuoteState(remote) {
	case "APPROVED", "APPROVAL_NOT_NEEDED":
		return true
	default:
		return false
	}
}

func parseHubSpotBool(raw string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}
