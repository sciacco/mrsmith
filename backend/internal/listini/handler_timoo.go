package listini

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleGetTimooPricing returns Timoo pricing for a customer, with fallback to defaults.
func (h *Handler) handleGetTimooPricing(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	customerID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_customer_id")
		return
	}

	type pricing struct {
		UserMonth float64 `json:"user_month"`
		SeMonth   float64 `json:"se_month"`
		IsDefault bool    `json:"is_default"`
	}

	// Try customer-specific first
	var pricesJSON []byte
	err = h.mistraDB.QueryRowContext(r.Context(), `
		SELECT prices FROM products.custom_items
		WHERE key_label = 'timoo_indiretta' AND customer_id = $1`, customerID).Scan(&pricesJSON)
	if err == nil {
		var p pricing
		if err := json.Unmarshal(pricesJSON, &p); err != nil {
			h.dbFailure(w, r, "get_timoo_pricing_unmarshal", err)
			return
		}
		p.IsDefault = false
		httputil.JSON(w, http.StatusOK, p)
		return
	}
	if err != sql.ErrNoRows {
		h.dbFailure(w, r, "get_timoo_pricing", err)
		return
	}

	// Fall back to default (customer_id = -1)
	err = h.mistraDB.QueryRowContext(r.Context(), `
		SELECT prices FROM products.custom_items
		WHERE key_label = 'timoo_indiretta' AND customer_id = -1`).Scan(&pricesJSON)
	if h.rowError(w, r, "get_timoo_pricing_default", err) {
		return
	}

	var p pricing
	if err := json.Unmarshal(pricesJSON, &p); err != nil {
		h.dbFailure(w, r, "get_timoo_pricing_default_unmarshal", err)
		return
	}
	p.IsDefault = true
	httputil.JSON(w, http.StatusOK, p)
}

// handleUpsertTimooPricing creates or updates Timoo pricing for a customer.
func (h *Handler) handleUpsertTimooPricing(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	customerID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_customer_id")
		return
	}

	var req TimooPricingRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body")
		return
	}

	pricesJSON, err := json.Marshal(map[string]float64{
		"user_month": req.UserMonth,
		"se_month":   req.SeMonth,
	})
	if err != nil {
		h.dbFailure(w, r, "upsert_timoo_pricing_marshal", err)
		return
	}

	_, err = h.mistraDB.ExecContext(r.Context(), `
		INSERT INTO products.custom_items (key_label, customer_id, prices)
		VALUES ('timoo_indiretta', $1, $2)
		ON CONFLICT (key_label, customer_id) DO UPDATE
		SET prices = EXCLUDED.prices`,
		customerID, pricesJSON)
	if err != nil {
		h.dbFailure(w, r, "upsert_timoo_pricing", err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
