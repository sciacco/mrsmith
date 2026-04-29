package rda

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

var (
	errPaymentMethodNotAllowed = errors.New("payment method not allowed for RDA")
	errPaymentProviderRequired = errors.New("payment provider required")
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

func (h *Handler) resolvePaymentMethod(ctx context.Context, requested string, provider providerDetail) (string, string, error) {
	providerDefault := providerDefaultPaymentMethod(provider)
	paymentMethod := strings.TrimSpace(requested)
	if paymentMethod == "" {
		paymentMethod = providerDefault
	}
	if paymentMethod == "" {
		code, err := h.defaultPaymentMethodCode(ctx)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", providerDefault, nil
			}
			return "", providerDefault, err
		}
		paymentMethod = strings.TrimSpace(code)
	}
	return paymentMethod, providerDefault, nil
}

func (h *Handler) validateEffectivePaymentMethod(ctx context.Context, code string, providerDefault string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return errPaymentMethodNotAllowed
	}
	method, err := h.paymentMethodByCode(ctx, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errPaymentMethodNotAllowed
		}
		return err
	}
	if method.RDAAvailable || code == strings.TrimSpace(providerDefault) {
		return nil
	}
	return errPaymentMethodNotAllowed
}

func (h *Handler) paymentMethodByCode(ctx context.Context, code string) (paymentMethod, error) {
	var method paymentMethod
	err := h.arakDB.QueryRowContext(ctx, `
		SELECT code, description, COALESCE(rda_available, false)
		FROM provider_qualifications.payment_method
		WHERE code = $1`, strings.TrimSpace(code)).Scan(&method.Code, &method.Description, &method.RDAAvailable)
	return method, err
}

func (m defaultPaymentMethod) MarshalJSON() ([]byte, error) {
	if m.Code == "" {
		return []byte(`{"code":""}`), nil
	}
	type alias defaultPaymentMethod
	return json.Marshal(alias(m))
}
