package listini

import (
	"net/http"
	"strconv"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleListCustomers returns all Mistra customers ordered by name.
func (h *Handler) handleListCustomers(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(),
		`SELECT id, name FROM customers.customer ORDER BY name`)
	if err != nil {
		h.dbFailure(w, r, "list_customers", err)
		return
	}
	defer rows.Close()

	type customer struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	var result []customer
	for rows.Next() {
		var c customer
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			h.dbFailure(w, r, "list_customers_scan", err)
			return
		}
		result = append(result, c)
	}
	if !h.rowsDone(w, r, rows, "list_customers") {
		return
	}
	if result == nil {
		result = []customer{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleListERPLinkedCustomers returns Mistra customers with fatgamma > 0.
func (h *Handler) handleListERPLinkedCustomers(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
		SELECT c.id, c.name
		FROM customers.customer c
		JOIN loader.erp_clienti_provenienza ep ON ep.numero_azienda = c.id
		WHERE ep.fatgamma > 0
		ORDER BY c.name`)
	if err != nil {
		h.dbFailure(w, r, "list_erp_linked_customers", err)
		return
	}
	defer rows.Close()

	type customer struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	var result []customer
	for rows.Next() {
		var c customer
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			h.dbFailure(w, r, "list_erp_linked_customers_scan", err)
			return
		}
		result = append(result, c)
	}
	if !h.rowsDone(w, r, rows, "list_erp_linked_customers") {
		return
	}
	if result == nil {
		result = []customer{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleListGrappaCustomers returns active Grappa customers with exclusions.
func (h *Handler) handleListGrappaCustomers(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	// Parse exclusion codes from query string
	excludeStr := r.URL.Query().Get("exclude")
	excludeCodes := parseIntList(excludeStr)
	if len(excludeCodes) == 0 {
		excludeCodes = []int{385} // default exclusion
	}

	query := `SELECT id, intestazione, codice_aggancio_gest
		FROM cli_fatturazione
		WHERE stato = 'attivo'
		  AND codice_aggancio_gest > 0`

	args := make([]any, 0, len(excludeCodes))
	if len(excludeCodes) > 0 {
		query += ` AND codice_aggancio_gest NOT IN (`
		for i, code := range excludeCodes {
			if i > 0 {
				query += `,`
			}
			query += `?`
			args = append(args, code)
		}
		query += `)`
	}
	query += ` ORDER BY intestazione`

	rows, err := h.grappaDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_grappa_customers", err)
		return
	}
	defer rows.Close()

	type grappaCustomer struct {
		ID                 int    `json:"id"`
		Intestazione       string `json:"intestazione"`
		CodiceAggancioGest int    `json:"codice_aggancio_gest"`
	}

	var result []grappaCustomer
	for rows.Next() {
		var c grappaCustomer
		if err := rows.Scan(&c.ID, &c.Intestazione, &c.CodiceAggancioGest); err != nil {
			h.dbFailure(w, r, "list_grappa_customers_scan", err)
			return
		}
		result = append(result, c)
	}
	if !h.rowsDone(w, r, rows, "list_grappa_customers") {
		return
	}
	if result == nil {
		result = []grappaCustomer{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleListRackCustomers returns Grappa customers that have active racks with sockets.
func (h *Handler) handleListRackCustomers(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(), `
		SELECT DISTINCT cf.id, cf.intestazione
		FROM cli_fatturazione cf
		JOIN racks r ON r.id_anagrafica = cf.id
		JOIN rack_sockets rs ON rs.rack_id = r.id_rack
		WHERE r.stato = 'attivo'
		ORDER BY cf.intestazione`)
	if err != nil {
		h.dbFailure(w, r, "list_rack_customers", err)
		return
	}
	defer rows.Close()

	type rackCustomer struct {
		ID           int    `json:"id"`
		Intestazione string `json:"intestazione"`
	}

	var result []rackCustomer
	for rows.Next() {
		var c rackCustomer
		if err := rows.Scan(&c.ID, &c.Intestazione); err != nil {
			h.dbFailure(w, r, "list_rack_customers_scan", err)
			return
		}
		result = append(result, c)
	}
	if !h.rowsDone(w, r, rows, "list_rack_customers") {
		return
	}
	if result == nil {
		result = []rackCustomer{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// parseIntList parses a comma-separated string of integers.
func parseIntList(s string) []int {
	if s == "" {
		return nil
	}
	var result []int
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			// trim spaces
			lo, hi := start, i
			for lo < hi && s[lo] == ' ' {
				lo++
			}
			for hi > lo && s[hi-1] == ' ' {
				hi--
			}
			if lo < hi {
				if v, err := strconv.Atoi(s[lo:hi]); err == nil {
					result = append(result, v)
				}
			}
			start = i + 1
		}
	}
	return result
}
