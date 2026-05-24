package ordini

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleActivateRow(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !h.requireCustomerRelations(w, r) || !h.requireVodka(w) || !h.requireGateway(w) {
		return
	}
	orderID, ok := h.parseOrderID(w, r)
	if !ok {
		return
	}
	rowID, ok := h.parseRowID(w, r)
	if !ok {
		return
	}
	payload, ok := decodeJSON[ActivateRowRequest](w, r)
	if !ok {
		return
	}
	if _, err := time.Parse("2006-01-02", payload.ActivationDate); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_activation_date")
		return
	}
	order, err := h.getOrderWithoutOrigin(r, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "order_not_found")
			return
		}
		h.dbFailure(w, r, "get_order_for_activation", err, "order_id", orderID, "row_id", rowID)
		return
	}
	if !requireState(w, stateOf(order), OrderStateInviato) {
		return
	}
	row, err := h.getOrderRow(r, orderID, rowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "row_not_found")
			return
		}
		h.dbFailure(w, r, "get_row_for_activation", err, "order_id", orderID, "row_id", rowID)
		return
	}
	if _, ok := parseRequiredInt(ptrStringValue(order.CdlanSystemODV)); !ok || row.CdlanSystemODVRow == nil {
		httputil.Error(w, http.StatusUnprocessableEntity, "precondition_missing")
		return
	}
	if !canActivateOrderRow(*row) {
		httputil.Error(w, http.StatusUnprocessableEntity, "precondition_missing")
		return
	}

	tx, err := h.deps.Vodka.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "activation_begin", err, "order_id", orderID, "row_id", rowID)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(r.Context(), `
UPDATE orders_rows
SET cdlan_data_attivazione = ?, confirm_data_attivazione = 1
WHERE id = ? AND orders_id = ?`, payload.ActivationDate, rowID, orderID); err != nil {
		h.dbFailure(w, r, "activation_update_row", err, "order_id", orderID, "row_id", rowID)
		return
	}
	if err := h.gatewaySetActivationDate(order, *row, payload.ActivationDate); err != nil {
		h.logFailure(r, slog.LevelWarn, "activation gateway failed", "activate_row", start, gatewayFailureAttrs("/orders/v1/set-order-activation", err, "order_id", orderID, "row_id", rowID)...)
		httputil.Error(w, http.StatusBadGateway, "gateway_error")
		return
	}

	var total, confirmed int64
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(id) FROM orders_rows WHERE orders_id = ?`, orderID).Scan(&total); err != nil {
		h.dbFailure(w, r, "activation_count_total", err, "order_id", orderID, "row_id", rowID)
		return
	}
	if err := tx.QueryRowContext(r.Context(), `
SELECT COUNT(id)
FROM orders_rows
WHERE orders_id = ?
  AND (confirm_data_attivazione = 1 OR data_annullamento IS NOT NULL OR cdlan_qta = 0)`, orderID).Scan(&confirmed); err != nil {
		h.dbFailure(w, r, "activation_count_confirmed", err, "order_id", orderID, "row_id", rowID)
		return
	}
	newState := string(OrderStateInviato)
	if total > 0 && confirmed == total {
		res, err := tx.ExecContext(r.Context(), `
	UPDATE orders
	SET cdlan_stato = 'ATTIVO'
	WHERE id = ? AND cdlan_stato = 'INVIATO'`, orderID)
		if err != nil {
			h.dbFailure(w, r, "activation_set_order_attivo", err, "order_id", orderID, "row_id", rowID)
			return
		}
		affected, err := res.RowsAffected()
		if err != nil {
			h.dbFailure(w, r, "activation_set_order_attivo_rows_affected", err, "order_id", orderID, "row_id", rowID)
			return
		}
		if affected == 0 {
			h.logFailure(r, slog.LevelWarn, "activation state transition affected no rows after gateway success", "activation_set_order_attivo_conflict", start, "order_id", orderID, "row_id", rowID)
			httputil.Error(w, http.StatusConflict, "wrong_state")
			return
		}
		newState = string(OrderStateAttivo)
	}
	if err := tx.Commit(); err != nil {
		h.logFailure(r, slog.LevelError, "activation commit failed after gateway success", "activate_row", start, "order_id", orderID, "row_id", rowID, "error", err)
		httputil.Error(w, http.StatusInternalServerError, "db_commit_failed")
		return
	}
	committed = true

	updated, err := h.getOrderRow(r, orderID, rowID)
	if err != nil {
		h.dbFailure(w, r, "activation_get_updated_row", err, "order_id", orderID, "row_id", rowID)
		return
	}
	httputil.JSON(w, http.StatusOK, ActivationResponse{OrderState: newState, Row: *updated})
}

func canActivateOrderRow(row OrderRow) bool {
	if row.DataAnnullamento.Valid {
		return false
	}
	if row.CdlanQta.Valid && row.CdlanQta.Float64 == 0 {
		return false
	}
	return true
}
