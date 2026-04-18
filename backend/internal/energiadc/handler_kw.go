package energiadc

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListCustomerKW(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	customerID, ok := h.parsePathInt(w, r, "customerId", "invalid_customer_id")
	if !ok {
		return
	}

	period := r.URL.Query().Get("period")
	if period != "day" && period != "month" {
		httputil.Error(w, http.StatusBadRequest, "invalid_period_parameter")
		return
	}

	cosfi, ok := h.parseRequiredQueryInt(w, r, "cosfi", "invalid_cosfi_parameter")
	if !ok {
		return
	}
	if cosfi < 70 || cosfi > 100 {
		httputil.Error(w, http.StatusBadRequest, "invalid_cosfi_parameter")
		return
	}

	multiplier := cosfiMultiplier(cosfi)
	var (
		query string
		args  []any
	)
	if period == "day" {
		query = `
			SELECT
				DATE_FORMAT(giorno, '%Y-%m-%d') AS bucket,
				DATE_FORMAT(giorno, '%d/%m/%Y') AS label,
				ROUND(COALESCE(kilowatt, 0) * ?, 2) AS kilowatt
			FROM rack_power_daily_summary
			WHERE id_anagrafica = ?
			ORDER BY giorno ASC`
		args = []any{multiplier, customerID}
	} else {
		query = `
			SELECT
				DATE_FORMAT(giorno, '%Y-%m') AS bucket,
				DATE_FORMAT(MIN(giorno), '%m/%Y') AS label,
				ROUND(AVG(COALESCE(kilowatt, 0)) * ?, 2) AS kilowatt
			FROM rack_power_daily_summary
			WHERE id_anagrafica = ?
			GROUP BY DATE_FORMAT(giorno, '%Y-%m')
			ORDER BY MIN(giorno) ASC`
		args = []any{multiplier, customerID}
	}

	rows, err := h.grappaDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_customer_kw", err, "customer_id", customerID, "period", period)
		return
	}
	defer rows.Close()

	result := make([]kwPointResponse, 0)
	for rows.Next() {
		var item kwPointResponse
		if err := rows.Scan(&item.Bucket, &item.Label, &item.Kilowatt); err != nil {
			h.dbFailure(w, r, "list_customer_kw_scan", err, "customer_id", customerID, "period", period)
			return
		}
		result = append(result, item)
	}
	if !h.rowsDone(w, r, rows, "list_customer_kw") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}
