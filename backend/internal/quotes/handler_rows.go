package quotes

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// ── List kit rows ──

func (h *Handler) handleListRows(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}

	// Verify quote exists
	var exists bool
	if err := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM quotes.quote WHERE id = $1)`, quoteID).Scan(&exists); err != nil {
		h.dbFailure(w, r, "list_rows_check", err)
		return
	}
	if !exists {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT qr.id, qr.quote_id, qr.kit_id, k.internal_name, qr.nrc_row, qr.mrc_row,
		        COALESCE(qr.bundle_prefix_row, ''), qr.hs_line_item_id, qr.hs_line_item_nrc, qr.position
		 FROM quotes.quote_rows qr
		 LEFT JOIN products.kit k ON k.id = qr.kit_id
		 WHERE qr.quote_id = $1
		 ORDER BY qr.position`, quoteID)
	if err != nil {
		h.dbFailure(w, r, "list_rows", err)
		return
	}
	defer rows.Close()

	type kitRow struct {
		ID              int     `json:"id"`
		QuoteID         int     `json:"quote_id"`
		KitID           int     `json:"kit_id"`
		InternalName    string  `json:"internal_name"`
		NrcRow          float64 `json:"nrc_row"`
		MrcRow          float64 `json:"mrc_row"`
		BundlePrefixRow string  `json:"bundle_prefix_row"`
		HsLineItemID    *int64  `json:"hs_line_item_id"`
		HsLineItemNrc   *int64  `json:"hs_line_item_nrc"`
		Position        int     `json:"position"`
	}

	result := []kitRow{}
	for rows.Next() {
		var kr kitRow
		if err := rows.Scan(&kr.ID, &kr.QuoteID, &kr.KitID, &kr.InternalName, &kr.NrcRow, &kr.MrcRow,
			&kr.BundlePrefixRow, &kr.HsLineItemID, &kr.HsLineItemNrc, &kr.Position); err != nil {
			h.dbFailure(w, r, "list_rows_scan", err)
			return
		}
		result = append(result, kr)
	}
	if !h.rowsDone(w, r, rows, "list_rows") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// ── Add kit row ──

func (h *Handler) handleAddRow(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}

	var body struct {
		KitID int `json:"kit_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.KitID == 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_kit_id")
		return
	}

	// Get next position
	var maxPos int
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(MAX(position), 0) FROM quotes.quote_rows WHERE quote_id = $1`, quoteID).Scan(&maxPos)

	var rowID int
	err = h.db.QueryRowContext(r.Context(),
		`INSERT INTO quotes.quote_rows (quote_id, kit_id, position) VALUES ($1, $2, $3) RETURNING id`,
		quoteID, body.KitID, maxPos+1).Scan(&rowID)
	if err != nil {
		h.dbFailure(w, r, "add_row", err)
		return
	}

	// Re-query to get trigger-populated fields
	var kr struct {
		ID              int     `json:"id"`
		QuoteID         int     `json:"quote_id"`
		KitID           int     `json:"kit_id"`
		InternalName    string  `json:"internal_name"`
		NrcRow          float64 `json:"nrc_row"`
		MrcRow          float64 `json:"mrc_row"`
		BundlePrefixRow string  `json:"bundle_prefix_row"`
		Position        int     `json:"position"`
	}
	err = h.db.QueryRowContext(r.Context(),
		`SELECT qr.id, qr.quote_id, qr.kit_id, k.internal_name, qr.nrc_row, qr.mrc_row,
		        COALESCE(qr.bundle_prefix_row, ''), qr.position
		 FROM quotes.quote_rows qr
		 LEFT JOIN products.kit k ON k.id = qr.kit_id
		 WHERE qr.id = $1`, rowID).Scan(
		&kr.ID, &kr.QuoteID, &kr.KitID, &kr.InternalName, &kr.NrcRow, &kr.MrcRow, &kr.BundlePrefixRow, &kr.Position)
	if err != nil {
		h.dbFailure(w, r, "add_row_refetch", err)
		return
	}

	httputil.JSON(w, http.StatusCreated, kr)
}

// ── Delete kit row ──

func (h *Handler) handleDeleteRow(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}
	rowID, err := strconv.Atoi(r.PathValue("rowId"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_row_id")
		return
	}

	// Ownership check
	var ownerQuoteID int
	err = h.db.QueryRowContext(r.Context(),
		`SELECT quote_id FROM quotes.quote_rows WHERE id = $1`, rowID).Scan(&ownerQuoteID)
	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "row_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "delete_row_check", err)
		return
	}
	if ownerQuoteID != quoteID {
		httputil.Error(w, http.StatusNotFound, "row_not_found")
		return
	}

	_, err = h.db.ExecContext(r.Context(), `DELETE FROM quotes.quote_rows WHERE id = $1`, rowID)
	if err != nil {
		h.dbFailure(w, r, "delete_row", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── Update row position ──

func (h *Handler) handleUpdateRowPosition(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}
	rowID, err := strconv.Atoi(r.PathValue("rowId"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_row_id")
		return
	}

	var body struct {
		Position int `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}

	// Ownership check
	var ownerQuoteID int
	err = h.db.QueryRowContext(r.Context(),
		`SELECT quote_id FROM quotes.quote_rows WHERE id = $1`, rowID).Scan(&ownerQuoteID)
	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "row_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "update_position_check", err)
		return
	}
	if ownerQuoteID != quoteID {
		httputil.Error(w, http.StatusNotFound, "row_not_found")
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE quotes.quote_rows SET position = $1 WHERE id = $2`, body.Position, rowID)
	if err != nil {
		h.dbFailure(w, r, "update_position", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── List products for a kit row ──

func (h *Handler) handleListProducts(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}
	rowID, err := strconv.Atoi(r.PathValue("rowId"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_row_id")
		return
	}

	// Ownership check
	var ownerQuoteID int
	err = h.db.QueryRowContext(r.Context(),
		`SELECT quote_id FROM quotes.quote_rows WHERE id = $1`, rowID).Scan(&ownerQuoteID)
	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "row_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "list_products_check", err)
		return
	}
	if ownerQuoteID != quoteID {
		httputil.Error(w, http.StatusNotFound, "row_not_found")
		return
	}

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, product_code, product_name, minimum, maximum, required,
		        nrc, mrc, position, group_name, included, main_product, quantity,
		        extended_description
		 FROM quotes.v_quote_rows_products
		 WHERE quote_row_id = $1
		 ORDER BY position`, rowID)
	if err != nil {
		h.dbFailure(w, r, "list_products", err)
		return
	}
	defer rows.Close()

	type product struct {
		ID                  int     `json:"id"`
		ProductCode         string  `json:"product_code"`
		ProductName         string  `json:"product_name"`
		Minimum             int     `json:"minimum"`
		Maximum             int     `json:"maximum"`
		Required            bool    `json:"required"`
		NRC                 float64 `json:"nrc"`
		MRC                 float64 `json:"mrc"`
		Position            int     `json:"position"`
		GroupName           string  `json:"group_name"`
		Included            bool    `json:"included"`
		MainProduct         bool    `json:"main_product"`
		Quantity            int     `json:"quantity"`
		ExtendedDescription *string `json:"extended_description"`
	}

	result := []product{}
	for rows.Next() {
		var p product
		if err := rows.Scan(&p.ID, &p.ProductCode, &p.ProductName, &p.Minimum, &p.Maximum, &p.Required,
			&p.NRC, &p.MRC, &p.Position, &p.GroupName, &p.Included, &p.MainProduct, &p.Quantity,
			&p.ExtendedDescription); err != nil {
			h.dbFailure(w, r, "list_products_scan", err)
			return
		}
		result = append(result, p)
	}
	if !h.rowsDone(w, r, rows, "list_products") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// ── Update product ──

func (h *Handler) handleUpdateProduct(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}
	rowID, err := strconv.Atoi(r.PathValue("rowId"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_row_id")
		return
	}
	productID, err := strconv.Atoi(r.PathValue("productId"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_product_id")
		return
	}

	// Ownership check: product → row → quote
	var ownerQuoteID int
	err = h.db.QueryRowContext(r.Context(),
		`SELECT qr.quote_id FROM quotes.quote_rows_products qrp
		 JOIN quotes.quote_rows qr ON qr.id = qrp.quote_row_id
		 WHERE qrp.id = $1 AND qrp.quote_row_id = $2`, productID, rowID).Scan(&ownerQuoteID)
	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "product_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "update_product_check", err)
		return
	}
	if ownerQuoteID != quoteID {
		httputil.Error(w, http.StatusNotFound, "product_not_found")
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}

	// Business rules
	// Check if spot quote → force MRC to 0
	var docType string
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(q.document_type, '') FROM quotes.quote q WHERE q.id = $1`, quoteID).Scan(&docType)
	if docType == "TSC-ORDINE" {
		body["mrc"] = 0
	}

	// Quantity floor: if included and quantity == 0, force quantity = 1
	if inc, ok := body["included"].(bool); ok && inc {
		if qty, ok := body["quantity"].(float64); ok && qty == 0 {
			body["quantity"] = 1
		}
	}

	body["id"] = productID

	payload, err := json.Marshal(body)
	if err != nil {
		h.dbFailure(w, r, "update_product_marshal", err)
		return
	}

	var result json.RawMessage
	err = h.db.QueryRowContext(r.Context(),
		`SELECT quotes.upd_quote_row_product($1::json)`, string(payload)).Scan(&result)
	if err != nil {
		h.dbFailure(w, r, "update_product_proc", err)
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}
