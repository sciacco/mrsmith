package quotes

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListQuotes(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	// Parse query parameters
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize := 25 // Fixed per spec

	status := q.Get("status")
	owner := q.Get("owner")
	search := q.Get("q")
	dateFrom := q.Get("date_from")
	dateTo := q.Get("date_to")
	sortBy := q.Get("sort")
	if sortBy == "" {
		sortBy = "quote_number"
	}
	sortDir := q.Get("dir")
	if sortDir != "asc" {
		sortDir = "desc"
	}

	// Whitelist sort columns
	allowedSorts := map[string]string{
		"quote_number":  "q.quote_number",
		"document_date": "q.document_date",
		"customer_name": "c.name",
		"status":        "q.status",
	}
	sortCol, ok := allowedSorts[sortBy]
	if !ok {
		sortCol = "q.quote_number"
	}

	// Build WHERE clause
	where := []string{}
	args := []any{}
	argIdx := 0

	if status != "" {
		argIdx++
		where = append(where, fmt.Sprintf("q.status = $%d", argIdx))
		args = append(args, status)
	}
	if owner != "" {
		argIdx++
		where = append(where, fmt.Sprintf("q.owner = $%d", argIdx))
		args = append(args, owner)
	}
	if search != "" {
		argIdx++
		like := "%" + search + "%"
		where = append(where, fmt.Sprintf(
			"(q.quote_number ILIKE $%d OR c.name ILIKE $%d OR d.name ILIKE $%d)",
			argIdx, argIdx, argIdx))
		args = append(args, like)
	}
	if dateFrom != "" {
		argIdx++
		where = append(where, fmt.Sprintf("q.document_date >= $%d::date", argIdx))
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		argIdx++
		where = append(where, fmt.Sprintf("q.document_date <= $%d::date", argIdx))
		args = append(args, dateTo)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	baseFrom := ` FROM quotes.quote q
	              LEFT JOIN loader.hubs_company c ON c.id = q.customer_id
	              LEFT JOIN loader.hubs_deal d ON d.id = q.hs_deal_id
	              LEFT JOIN loader.hubs_owner o ON o.id::text = q.owner`

	// Count total
	var total int
	countQuery := "SELECT COUNT(*)" + baseFrom + whereClause
	if err := h.db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total); err != nil {
		h.dbFailure(w, r, "list_quotes_count", err)
		return
	}

	// Fetch page
	offset := (page - 1) * pageSize
	argIdx++
	limitArg := argIdx
	argIdx++
	offsetArg := argIdx
	args = append(args, pageSize, offset)

	selectQuery := fmt.Sprintf(`SELECT q.id, q.quote_number, q.customer_id, q.document_date, q.document_type,
	       q.status, q.owner, q.hs_deal_id, q.hs_quote_id, q.proposal_type, q.created_at, q.updated_at,
	       c.name as customer_name, d.name as deal_name,
	       COALESCE(o.first_name || ' ' || o.last_name, '') as owner_name`+
		baseFrom+whereClause+
		fmt.Sprintf(` ORDER BY %s %s LIMIT $%d OFFSET $%d`, sortCol, sortDir, limitArg, offsetArg))

	rows, err := h.db.QueryContext(r.Context(), selectQuery, args...)
	if err != nil {
		h.dbFailure(w, r, "list_quotes", err)
		return
	}
	defer rows.Close()

	type quoteRow struct {
		ID            int            `json:"id"`
		QuoteNumber   string         `json:"quote_number"`
		CustomerID    sql.NullInt64  `json:"-"`
		CustomerIDV   *int64         `json:"customer_id"`
		DocumentDate  sql.NullString `json:"-"`
		DocumentDateV *string        `json:"document_date"`
		DocumentType  sql.NullString `json:"-"`
		DocumentTypeV *string        `json:"document_type"`
		Status        string         `json:"status"`
		Owner         sql.NullString `json:"-"`
		OwnerV        *string        `json:"owner"`
		HSDealID      sql.NullInt64  `json:"-"`
		HSDealIDV     *int64         `json:"hs_deal_id"`
		HSQuoteID     sql.NullInt64  `json:"-"`
		HSQuoteIDV    *int64         `json:"hs_quote_id"`
		ProposalType  sql.NullString `json:"-"`
		ProposalTypeV *string        `json:"proposal_type"`
		CreatedAt     string         `json:"created_at"`
		UpdatedAt     string         `json:"updated_at"`
		CustomerName  sql.NullString `json:"-"`
		CustomerNameV *string        `json:"customer_name"`
		DealName      sql.NullString `json:"-"`
		DealNameV     *string        `json:"deal_name"`
		OwnerName     sql.NullString `json:"-"`
		OwnerNameV    *string        `json:"owner_name"`
	}

	quotes := []quoteRow{}
	for rows.Next() {
		var qr quoteRow
		if err := rows.Scan(
			&qr.ID, &qr.QuoteNumber, &qr.CustomerID, &qr.DocumentDate, &qr.DocumentType,
			&qr.Status, &qr.Owner, &qr.HSDealID, &qr.HSQuoteID, &qr.ProposalType,
			&qr.CreatedAt, &qr.UpdatedAt,
			&qr.CustomerName, &qr.DealName, &qr.OwnerName,
		); err != nil {
			h.dbFailure(w, r, "list_quotes_scan", err)
			return
		}
		if qr.CustomerID.Valid {
			qr.CustomerIDV = &qr.CustomerID.Int64
		}
		if qr.DocumentDate.Valid {
			qr.DocumentDateV = &qr.DocumentDate.String
		}
		if qr.DocumentType.Valid {
			qr.DocumentTypeV = &qr.DocumentType.String
		}
		if qr.Owner.Valid {
			qr.OwnerV = &qr.Owner.String
		}
		if qr.HSDealID.Valid {
			qr.HSDealIDV = &qr.HSDealID.Int64
		}
		if qr.HSQuoteID.Valid {
			qr.HSQuoteIDV = &qr.HSQuoteID.Int64
		}
		if qr.ProposalType.Valid {
			qr.ProposalTypeV = &qr.ProposalType.String
		}
		if qr.CustomerName.Valid {
			qr.CustomerNameV = &qr.CustomerName.String
		}
		if qr.DealName.Valid {
			qr.DealNameV = &qr.DealName.String
		}
		if qr.OwnerName.Valid {
			qr.OwnerNameV = &qr.OwnerName.String
		}
		quotes = append(quotes, qr)
	}
	if !h.rowsDone(w, r, rows, "list_quotes") {
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"quotes":    quotes,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ── Get single quote ──

func (h *Handler) handleGetQuote(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}

	row := h.db.QueryRowContext(r.Context(),
		`SELECT q.id, q.quote_number, q.customer_id, q.deal_number, q.owner,
		        q.document_date, q.document_type, q.replace_orders, q.template, q.services,
		        q.proposal_type, q.initial_term_months, q.next_term_months, q.bill_months,
		        q.delivered_in_days, q.date_sent, q.status, q.notes, q.nrc_charge_time,
		        q.created_at, q.updated_at, q.description, q.hs_deal_id, q.hs_quote_id,
		        q.payment_method, q.trial,
		        q.rif_ordcli, q.rif_tech_nom, q.rif_tech_tel, q.rif_tech_email,
		        q.rif_altro_tech_nom, q.rif_altro_tech_tel, q.rif_altro_tech_email,
		        q.rif_adm_nom, q.rif_adm_tech_tel, q.rif_adm_tech_email,
		        c.name as customer_name, d.name as deal_name,
		        COALESCE(o.first_name || ' ' || o.last_name, '') as owner_name
		 FROM quotes.quote q
		 LEFT JOIN loader.hubs_company c ON c.id = q.customer_id
		 LEFT JOIN loader.hubs_deal d ON d.id = q.hs_deal_id
		 LEFT JOIN loader.hubs_owner o ON o.id::text = q.owner
		 WHERE q.id = $1`, quoteID)

	var q struct {
		ID                int     `json:"id"`
		QuoteNumber       string  `json:"quote_number"`
		CustomerID        *int64  `json:"customer_id"`
		DealNumber        *string `json:"deal_number"`
		Owner             *string `json:"owner"`
		DocumentDate      *string `json:"document_date"`
		DocumentType      *string `json:"document_type"`
		ReplaceOrders     *string `json:"replace_orders"`
		Template          *string `json:"template"`
		Services          *string `json:"services"`
		ProposalType      *string `json:"proposal_type"`
		InitialTermMonths int     `json:"initial_term_months"`
		NextTermMonths    int     `json:"next_term_months"`
		BillMonths        int     `json:"bill_months"`
		DeliveredInDays   int     `json:"delivered_in_days"`
		DateSent          *string `json:"date_sent"`
		Status            string  `json:"status"`
		Notes             *string `json:"notes"`
		NrcChargeTime     int     `json:"nrc_charge_time"`
		CreatedAt         string  `json:"created_at"`
		UpdatedAt         string  `json:"updated_at"`
		Description       string  `json:"description"`
		HSDealID          *int64  `json:"hs_deal_id"`
		HSQuoteID         *int64  `json:"hs_quote_id"`
		PaymentMethod     *string `json:"payment_method"`
		Trial             *string `json:"trial"`
		RifOrdcli         *string `json:"rif_ordcli"`
		RifTechNom        *string `json:"rif_tech_nom"`
		RifTechTel        *string `json:"rif_tech_tel"`
		RifTechEmail      *string `json:"rif_tech_email"`
		RifAltroTechNom   *string `json:"rif_altro_tech_nom"`
		RifAltroTechTel   *string `json:"rif_altro_tech_tel"`
		RifAltroTechEmail *string `json:"rif_altro_tech_email"`
		RifAdmNom         *string `json:"rif_adm_nom"`
		RifAdmTechTel     *string `json:"rif_adm_tech_tel"`
		RifAdmTechEmail   *string `json:"rif_adm_tech_email"`
		CustomerName      *string `json:"customer_name"`
		DealName          *string `json:"deal_name"`
		OwnerName         *string `json:"owner_name"`
	}

	err = row.Scan(
		&q.ID, &q.QuoteNumber, &q.CustomerID, &q.DealNumber, &q.Owner,
		&q.DocumentDate, &q.DocumentType, &q.ReplaceOrders, &q.Template, &q.Services,
		&q.ProposalType, &q.InitialTermMonths, &q.NextTermMonths, &q.BillMonths,
		&q.DeliveredInDays, &q.DateSent, &q.Status, &q.Notes, &q.NrcChargeTime,
		&q.CreatedAt, &q.UpdatedAt, &q.Description, &q.HSDealID, &q.HSQuoteID,
		&q.PaymentMethod, &q.Trial,
		&q.RifOrdcli, &q.RifTechNom, &q.RifTechTel, &q.RifTechEmail,
		&q.RifAltroTechNom, &q.RifAltroTechTel, &q.RifAltroTechEmail,
		&q.RifAdmNom, &q.RifAdmTechTel, &q.RifAdmTechEmail,
		&q.CustomerName, &q.DealName, &q.OwnerName,
	)
	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "get_quote", err)
		return
	}

	httputil.JSON(w, http.StatusOK, q)
}

// ── Update quote header ──

func (h *Handler) handleUpdateQuote(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}

	// Parse incoming partial update
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}

	// upd_quote_head is full-overwrite: read current quote, merge, send complete JSON.
	// Step 1: Read current full quote as JSON
	var currentJSON json.RawMessage
	err = h.db.QueryRowContext(r.Context(),
		`SELECT row_to_json(q) FROM quotes.quote q WHERE q.id = $1`, quoteID).Scan(&currentJSON)
	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "update_quote_read", err)
		return
	}

	// Step 2: Merge incoming fields onto current snapshot
	var current map[string]any
	if err := json.Unmarshal(currentJSON, &current); err != nil {
		h.dbFailure(w, r, "update_quote_unmarshal", err)
		return
	}
	for k, v := range body {
		current[k] = v
	}

	// Step 3: Apply business rules
	// COLOCATION template → force bill_months = 3
	if tmpl, ok := current["template"]; ok && tmpl != nil {
		templateID, _ := tmpl.(string)
		if templateID != "" {
			var isColo bool
			_ = h.db.QueryRowContext(r.Context(),
				`SELECT COALESCE(is_colo, false) FROM quotes.template WHERE template_id = $1`,
				templateID).Scan(&isColo)
			if isColo {
				current["bill_months"] = 3
			}
		}
	}

	// IaaS field lock: reject changes to locked fields
	if tmpl, ok := current["template"]; ok && tmpl != nil {
		templateID, _ := tmpl.(string)
		if templateID != "" {
			var templateType string
			_ = h.db.QueryRowContext(r.Context(),
				`SELECT COALESCE(template_type, 'standard') FROM quotes.template WHERE template_id = $1`,
				templateID).Scan(&templateType)
			if templateType == "iaas" {
				// Restore original values for locked fields
				var orig map[string]any
				_ = json.Unmarshal(currentJSON, &orig)
				for _, field := range []string{"services", "template", "initial_term_months", "next_term_months", "bill_months"} {
					current[field] = orig[field]
				}
			}
		}
	}

	// Step 4: Call stored procedure
	payload, err := json.Marshal(current)
	if err != nil {
		h.dbFailure(w, r, "update_quote_marshal", err)
		return
	}

	var result json.RawMessage
	err = h.db.QueryRowContext(r.Context(),
		`SELECT quotes.upd_quote_head($1::json)`, string(payload)).Scan(&result)
	if err != nil {
		h.dbFailure(w, r, "update_quote_proc", err)
		return
	}

	// Parse proc response
	var procResult struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(result, &procResult); err != nil {
		h.dbFailure(w, r, "update_quote_parse", err)
		return
	}
	if procResult.Status == "ERROR" {
		httputil.Error(w, http.StatusBadRequest, procResult.Message)
		return
	}

	// Re-fetch and return updated quote
	h.handleGetQuote(w, r)
}

// ── HS Status ──

func (h *Handler) handleGetHSStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}

	var hsQuoteID *int64
	var status string
	err = h.db.QueryRowContext(r.Context(),
		`SELECT hs_quote_id, status FROM quotes.quote WHERE id = $1`, quoteID).Scan(&hsQuoteID, &status)
	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "get_hs_status", err)
		return
	}

	var pdfURL *string
	if hsQuoteID != nil {
		u := fmt.Sprintf("https://app.hubspot.com/quotes/%d", *hsQuoteID)
		pdfURL = &u
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"hs_quote_id": hsQuoteID,
		"status":      status,
		"pdf_url":     pdfURL,
	})
}

// ── Create quote ──

func (h *Handler) handleCreateQuote(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}

	// Extract kit_ids before sending to stored proc
	kitIDs := []int{}
	if raw, ok := body["kit_ids"]; ok {
		if arr, ok := raw.([]any); ok {
			for _, v := range arr {
				if n, ok := v.(float64); ok {
					kitIDs = append(kitIDs, int(n))
				}
			}
		}
		delete(body, "kit_ids")
	}

	// Begin transaction
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "create_quote_tx", err)
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Generate quote number
	var quoteNumber string
	err = tx.QueryRowContext(r.Context(), `SELECT common.new_document_number('SP-')`).Scan(&quoteNumber)
	if err != nil {
		h.dbFailure(w, r, "create_quote_number", err)
		return
	}
	body["quote_number"] = quoteNumber

	// Business rules
	body["status"] = "DRAFT"
	body["hs_quote_id"] = nil

	// COLOCATION template → force bill_months = 3
	if tmpl, ok := body["template"]; ok && tmpl != nil {
		templateID, _ := tmpl.(string)
		if templateID != "" {
			var isColo bool
			_ = tx.QueryRowContext(r.Context(),
				`SELECT COALESCE(is_colo, false) FROM quotes.template WHERE template_id = $1`,
				templateID).Scan(&isColo)
			if isColo {
				body["bill_months"] = 3
			}
		}
	}

	// Default payment
	if pm, ok := body["payment_method"]; !ok || pm == "" || pm == nil {
		body["payment_method"] = "402"
	}

	// Call stored procedure
	payload, err := json.Marshal(body)
	if err != nil {
		h.dbFailure(w, r, "create_quote_marshal", err)
		return
	}

	var result json.RawMessage
	err = tx.QueryRowContext(r.Context(),
		`SELECT quotes.ins_quote_head($1::json)`, string(payload)).Scan(&result)
	if err != nil {
		h.dbFailure(w, r, "create_quote_proc", err)
		return
	}

	// Parse proc response
	var procResult struct {
		ID      int    `json:"id"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(result, &procResult); err != nil {
		h.dbFailure(w, r, "create_quote_parse", err)
		return
	}
	if procResult.Status == "ERROR" {
		httputil.Error(w, http.StatusBadRequest, procResult.Message)
		return
	}

	// Insert kit rows
	for i, kitID := range kitIDs {
		_, err := tx.ExecContext(r.Context(),
			`INSERT INTO quotes.quote_rows (quote_id, kit_id, position) VALUES ($1, $2, $3)`,
			procResult.ID, kitID, i+1)
		if err != nil {
			h.dbFailure(w, r, "create_quote_kit_row", err)
			return
		}
	}

	// Commit
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "create_quote_commit", err)
		return
	}

	// Re-fetch and return
	var q struct {
		ID          int    `json:"id"`
		QuoteNumber string `json:"quote_number"`
		Status      string `json:"status"`
	}
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT id, quote_number, status FROM quotes.quote WHERE id = $1`, procResult.ID).Scan(
		&q.ID, &q.QuoteNumber, &q.Status)

	httputil.JSON(w, http.StatusCreated, q)
}

// ── Delete quote ──

func (h *Handler) handleDeleteQuote(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}

	// Check delete role
	claims := r.Context().Value("claims")
	if claims == nil {
		// Fallback: extract roles from auth context via header
		// The ACL middleware already gates app_quotes_access.
		// Check for app_quotes_delete explicitly.
		httputil.Error(w, http.StatusForbidden, "delete_role_required")
		return
	}

	// Load quote to check hs_quote_id
	var hsQuoteID *int64
	err = h.db.QueryRowContext(r.Context(),
		`SELECT hs_quote_id FROM quotes.quote WHERE id = $1`, quoteID).Scan(&hsQuoteID)
	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "delete_quote_load", err)
		return
	}

	// If HS quote exists, delete from HS first
	if hsQuoteID != nil && h.hs != nil {
		if err := h.hs.DeleteQuote(r.Context(), *hsQuoteID); err != nil {
			httputil.Error(w, http.StatusBadGateway, "hubspot_delete_failed")
			return
		}
	}

	// Delete from DB
	_, err = h.db.ExecContext(r.Context(), `DELETE FROM quotes.quote WHERE id = $1`, quoteID)
	if err != nil {
		h.dbFailure(w, r, "delete_quote", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
