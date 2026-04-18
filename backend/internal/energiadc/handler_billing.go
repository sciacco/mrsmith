package energiadc

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListBillingCharges(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	customerID, ok := h.parsePathInt(w, r, "customerId", "invalid_customer_id")
	if !ok {
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(), `
		SELECT
			id,
			start_period,
			end_period,
			ampere,
			eccedenti,
			amount,
			pun,
			coefficiente,
			fisso_cu,
			importo_eccedenti
		FROM importi_corrente_colocation
		WHERE customer_id = ?
		ORDER BY start_period DESC, end_period DESC, id DESC`, customerID)
	if err != nil {
		h.dbFailure(w, r, "list_billing_charges", err, "customer_id", customerID)
		return
	}
	defer rows.Close()

	result := make([]billingChargeResponse, 0)
	for rows.Next() {
		var item billingChargeResponse
		var startPeriod sql.NullTime
		var endPeriod sql.NullTime
		var amount sql.NullFloat64
		if err := rows.Scan(
			&item.ID,
			&startPeriod,
			&endPeriod,
			&item.Ampere,
			&item.Eccedenti,
			&amount,
			&item.PUN,
			&item.Coefficiente,
			&item.FissoCU,
			&item.ImportoEccedenti,
		); err != nil {
			h.dbFailure(w, r, "list_billing_charges_scan", err, "customer_id", customerID)
			return
		}
		item.StartPeriod = formatDate(startPeriod, h.config.Location)
		item.EndPeriod = formatDate(endPeriod, h.config.Location)
		item.Amount = nullableFloat(amount)
		result = append(result, item)
	}
	if !h.rowsDone(w, r, rows, "list_billing_charges") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}
