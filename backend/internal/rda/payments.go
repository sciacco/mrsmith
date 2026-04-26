package rda

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handlePaymentMethods(w http.ResponseWriter, r *http.Request) {
	if !h.requireArakDB(w) {
		return
	}
	rows, err := h.arakDB.QueryContext(r.Context(), `
		SELECT code, description, COALESCE(rda_available, false)
		FROM provider_qualifications.payment_method
		WHERE rda_available IS TRUE
		ORDER BY description ASC`)
	if err != nil {
		httputil.InternalError(w, r, err, "rda payment methods query failed")
		return
	}
	defer rows.Close()

	items := make([]paymentMethod, 0)
	for rows.Next() {
		var item paymentMethod
		if err := rows.Scan(&item.Code, &item.Description, &item.RDAAvailable); err != nil {
			httputil.InternalError(w, r, err, "rda payment methods scan failed")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		httputil.InternalError(w, r, err, "rda payment methods rows failed")
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleDefaultPaymentMethod(w http.ResponseWriter, r *http.Request) {
	if !h.requireArakDB(w) {
		return
	}
	code, err := h.defaultPaymentMethodCode(r.Context())
	if err != nil {
		if err == sql.ErrNoRows {
			httputil.JSON(w, http.StatusOK, defaultPaymentMethod{})
			return
		}
		httputil.InternalError(w, r, err, "rda default payment method query failed")
		return
	}
	httputil.JSON(w, http.StatusOK, defaultPaymentMethod{Code: code})
}

func (h *Handler) defaultPaymentMethodCode(ctx context.Context) (string, error) {
	var code string
	err := h.arakDB.QueryRowContext(ctx, `
		SELECT payment_method_code
		FROM provider_qualifications.payment_method_default_cdlan
		LIMIT 1`).Scan(&code)
	return code, err
}

func (m defaultPaymentMethod) MarshalJSON() ([]byte, error) {
	if m.Code == "" {
		return []byte(`{"code":""}`), nil
	}
	type alias defaultPaymentMethod
	return json.Marshal(alias(m))
}
