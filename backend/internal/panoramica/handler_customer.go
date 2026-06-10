package panoramica

import (
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleListCustomersWithInvoices returns customers from loader.erp_clienti_con_fatture.
// GET /panoramica/v1/customers/with-invoices
func (h *Handler) handleListCustomersWithInvoices(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(),
		`SELECT numero_azienda, ragione_sociale FROM loader.erp_clienti_con_fatture ORDER BY ragione_sociale`)
	if err != nil {
		h.dbFailure(w, r, "list_customers_with_invoices", err)
		return
	}
	defer rows.Close()

	type customer struct {
		NumeroAzienda  int    `json:"numero_azienda"`
		RagioneSociale string `json:"ragione_sociale"`
	}

	var result []customer
	for rows.Next() {
		var c customer
		if err := rows.Scan(&c.NumeroAzienda, &c.RagioneSociale); err != nil {
			h.dbFailure(w, r, "list_customers_with_invoices_scan", err)
			return
		}
		result = append(result, c)
	}
	if !h.rowsDone(w, r, rows, "list_customers_with_invoices") {
		return
	}
	if result == nil {
		result = []customer{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleListCustomersWithOrders returns customers having recurring or spot orders.
// GET /panoramica/v1/customers/with-orders
func (h *Handler) handleListCustomersWithOrders(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	query := `SELECT DISTINCT odv.numero_azienda, odv.ragione_sociale
FROM loader.v_ordini_ric_spot AS odv
JOIN loader.erp_anagrafiche_clienti AS cli
  ON cli.numero_azienda = odv.numero_azienda
  AND (cli.data_dismissione >= NOW() OR cli.data_dismissione = '0001-01-01 00:00:00')
ORDER BY ragione_sociale`

	rows, err := h.mistraDB.QueryContext(r.Context(), query)
	if err != nil {
		h.dbFailure(w, r, "list_customers_with_orders", err)
		return
	}
	defer rows.Close()

	type customer struct {
		NumeroAzienda  int    `json:"numero_azienda"`
		RagioneSociale string `json:"ragione_sociale"`
	}

	var result []customer
	for rows.Next() {
		var c customer
		if err := rows.Scan(&c.NumeroAzienda, &c.RagioneSociale); err != nil {
			h.dbFailure(w, r, "list_customers_with_orders_scan", err)
			return
		}
		result = append(result, c)
	}
	if !h.rowsDone(w, r, rows, "list_customers_with_orders") {
		return
	}
	if result == nil {
		result = []customer{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleListCustomersWithAccessLines returns Grappa clients with active access lines.
// GET /panoramica/v1/customers/with-access-lines
// Note: returns Grappa internal IDs (cf.id), not ERP IDs.
func (h *Handler) handleListCustomersWithAccessLines(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(),
		`SELECT DISTINCT cf.id, cf.intestazione
FROM loader.grappa_foglio_linee fl
JOIN loader.grappa_cli_fatturazione cf ON fl.id_anagrafica = cf.id
WHERE cf.codice_aggancio_gest IS NOT NULL AND cf.stato = 'attivo'
ORDER BY cf.intestazione`)
	if err != nil {
		h.dbFailure(w, r, "list_customers_with_access_lines", err)
		return
	}
	defer rows.Close()

	type customer struct {
		ID           int    `json:"id"`
		Intestazione string `json:"intestazione"`
	}

	var result []customer
	for rows.Next() {
		var c customer
		if err := rows.Scan(&c.ID, &c.Intestazione); err != nil {
			h.dbFailure(w, r, "list_customers_with_access_lines_scan", err)
			return
		}
		result = append(result, c)
	}
	if !h.rowsDone(w, r, rows, "list_customers_with_access_lines") {
		return
	}
	if result == nil {
		result = []customer{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// parseStringList splits a comma-separated string into trimmed, non-empty parts.
func parseStringList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
