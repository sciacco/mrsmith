package reports

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleOrderStatuses returns distinct order statuses from v_ordini_ric_spot.
// GET /reports/v1/order-statuses
func (h *Handler) handleOrderStatuses(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(),
		`SELECT DISTINCT stato_ordine FROM loader.v_ordini_ric_spot ORDER BY stato_ordine`)
	if err != nil {
		h.dbFailure(w, r, "order_statuses", err)
		return
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			h.dbFailure(w, r, "order_statuses_scan", err)
			return
		}
		result = append(result, s)
	}
	if !h.rowsDone(w, r, rows, "order_statuses") {
		return
	}
	if result == nil {
		result = []string{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleConnectionTypes returns distinct connection types from grappa_foglio_linee.
// GET /reports/v1/connection-types
func (h *Handler) handleConnectionTypes(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(),
		`SELECT DISTINCT tipo_conn FROM loader.grappa_foglio_linee ORDER BY tipo_conn`)
	if err != nil {
		h.dbFailure(w, r, "connection_types", err)
		return
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			h.dbFailure(w, r, "connection_types_scan", err)
			return
		}
		result = append(result, s)
	}
	if !h.rowsDone(w, r, rows, "connection_types") {
		return
	}
	if result == nil {
		result = []string{}
	}

	httputil.JSON(w, http.StatusOK, result)
}
