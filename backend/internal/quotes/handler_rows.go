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
		        COALESCE(qr.bundle_prefix_row, ''), qr.hs_line_item_id, qr.hs_line_item_nrc, qr.position,
		        COALESCE(qr.hs_line_item_id::varchar, ''), COALESCE(qr.hs_line_item_nrc::varchar, '')
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
		HsMRC           string  `json:"hs_mrc"`
		HsNRC           string  `json:"hs_nrc"`
	}

	result := []kitRow{}
	for rows.Next() {
		var kr kitRow
		if err := rows.Scan(&kr.ID, &kr.QuoteID, &kr.KitID, &kr.InternalName, &kr.NrcRow, &kr.MrcRow,
			&kr.BundlePrefixRow, &kr.HsLineItemID, &kr.HsLineItemNrc, &kr.Position, &kr.HsMRC, &kr.HsNRC); err != nil {
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

	var quoteExists bool
	if err := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM quotes.quote WHERE id = $1)`, quoteID).Scan(&quoteExists); err != nil {
		h.dbFailure(w, r, "add_row_quote_check", err)
		return
	}
	if !quoteExists {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}

	var kitEligible bool
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(
			SELECT 1
			FROM products.kit
			WHERE id = $1 AND is_active = true AND ecommerce = false AND quotable = true
		)`, body.KitID).Scan(&kitEligible); err != nil {
		h.dbFailure(w, r, "add_row_kit_check", err)
		return
	}
	if !kitEligible {
		httputil.Error(w, http.StatusBadRequest, "kit_not_selectable")
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
		HsLineItemID    *int64  `json:"hs_line_item_id"`
		HsLineItemNrc   *int64  `json:"hs_line_item_nrc"`
		Position        int     `json:"position"`
		HsMRC           string  `json:"hs_mrc"`
		HsNRC           string  `json:"hs_nrc"`
	}
	err = h.db.QueryRowContext(r.Context(),
		`SELECT qr.id, qr.quote_id, qr.kit_id, k.internal_name, qr.nrc_row, qr.mrc_row,
		        COALESCE(qr.bundle_prefix_row, ''), qr.hs_line_item_id, qr.hs_line_item_nrc, qr.position,
		        COALESCE(qr.hs_line_item_id::varchar, ''), COALESCE(qr.hs_line_item_nrc::varchar, '')
		 FROM quotes.quote_rows qr
		 LEFT JOIN products.kit k ON k.id = qr.kit_id
		 WHERE qr.id = $1`, rowID).Scan(
		&kr.ID, &kr.QuoteID, &kr.KitID, &kr.InternalName, &kr.NrcRow, &kr.MrcRow, &kr.BundlePrefixRow,
		&kr.HsLineItemID, &kr.HsLineItemNrc, &kr.Position, &kr.HsMRC, &kr.HsNRC)
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

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "update_position_begin", err)
		return
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(r.Context(),
		`SELECT id FROM quotes.quote_rows WHERE quote_id = $1 ORDER BY position, id`, quoteID)
	if err != nil {
		h.dbFailure(w, r, "update_position_list", err)
		return
	}
	defer rows.Close()

	order := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			h.dbFailure(w, r, "update_position_scan", err)
			return
		}
		order = append(order, id)
	}
	if !h.rowsDone(w, r, rows, "update_position") {
		return
	}

	currentIndex := -1
	for i, id := range order {
		if id == rowID {
			currentIndex = i
			break
		}
	}
	if currentIndex == -1 {
		httputil.Error(w, http.StatusNotFound, "row_not_found")
		return
	}

	targetIndex := body.Position - 1
	if targetIndex < 0 {
		targetIndex = 0
	}
	if targetIndex >= len(order) {
		targetIndex = len(order) - 1
	}
	if targetIndex != currentIndex {
		moved := order[currentIndex]
		order = append(order[:currentIndex], order[currentIndex+1:]...)
		if targetIndex >= len(order) {
			order = append(order, moved)
		} else {
			order = append(order[:targetIndex], append([]int{moved}, order[targetIndex:]...)...)
		}
	}

	for i, id := range order {
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE quotes.quote_rows SET position = $1 WHERE id = $2`, i+1, id); err != nil {
			h.dbFailure(w, r, "update_position_write", err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "update_position_commit", err)
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
		`SELECT group_name, quote_row_id, riga, conta, required, main_product, position
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

	type productGroup struct {
		GroupName       string    `json:"group_name"`
		QuoteRowID      int       `json:"quote_row_id"`
		Products        []product `json:"products"`
		Count           int       `json:"count"`
		Required        bool      `json:"required"`
		MainProduct     bool      `json:"main_product"`
		Position        int       `json:"position"`
		IncludedProduct *product  `json:"included_product"`
	}

	result := []productGroup{}
	for rows.Next() {
		var (
			group        productGroup
			productsJSON json.RawMessage
			count        int
			required     int
			mainProduct  int
		)
		if err := rows.Scan(&group.GroupName, &group.QuoteRowID, &productsJSON, &count, &required, &mainProduct, &group.Position); err != nil {
			h.dbFailure(w, r, "list_products_scan", err)
			return
		}
		if err := json.Unmarshal(productsJSON, &group.Products); err != nil {
			h.dbFailure(w, r, "list_products_unmarshal", err)
			return
		}
		group.Count = count
		group.Required = required > 0
		group.MainProduct = mainProduct > 0
		for i := range group.Products {
			if group.Products[i].Included {
				group.IncludedProduct = &group.Products[i]
				break
			}
		}
		result = append(result, group)
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

	var updated bool
	err = h.db.QueryRowContext(r.Context(),
		`SELECT quotes.upd_quote_row_product($1::json)`, string(payload)).Scan(&updated)
	if err != nil {
		h.dbFailure(w, r, "update_product_proc", err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]bool{"ok": updated})
}
