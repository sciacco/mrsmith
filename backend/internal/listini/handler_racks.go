package listini

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// handleListCustomerRacks returns racks for a Grappa customer.
func (h *Handler) handleListCustomerRacks(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	customerID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_customer_id")
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(), `
		SELECT r.id_rack, r.name, r.floor, r.island, r.type, r.sconto,
		       dc.name AS room,
		       db.name AS building
		FROM racks r
		JOIN datacenter dc ON dc.id_datacenter = r.id_datacenter
		JOIN dc_build db ON db.id = dc.dc_build_id
		WHERE r.id_anagrafica = (
		    SELECT id FROM cli_fatturazione WHERE id = ? AND stato = 'attivo'
		)
		AND r.stato = 'attivo'
		ORDER BY db.name, dc.name, r.name`, customerID)
	if err != nil {
		h.dbFailure(w, r, "list_customer_racks", err)
		return
	}
	defer rows.Close()

	type rack struct {
		IDRack   int      `json:"id_rack"`
		Name     string   `json:"name"`
		Building string   `json:"building"`
		Room     string   `json:"room"`
		Floor    *int     `json:"floor"`
		Island   *int     `json:"island"`
		Type     *string  `json:"type"`
		Sconto   float64  `json:"sconto"`
	}

	var result []rack
	for rows.Next() {
		var rk rack
		if err := rows.Scan(
			&rk.IDRack, &rk.Name, &rk.Floor, &rk.Island, &rk.Type, &rk.Sconto,
			&rk.Room, &rk.Building,
		); err != nil {
			h.dbFailure(w, r, "list_customer_racks_scan", err)
			return
		}
		result = append(result, rk)
	}
	if !h.rowsDone(w, r, rows, "list_customer_racks") {
		return
	}
	if result == nil {
		result = []rack{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleBatchUpdateRackDiscounts updates discounts for multiple racks in a transaction.
func (h *Handler) handleBatchUpdateRackDiscounts(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	var req BatchRackDiscountRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body")
		return
	}

	if len(req.Items) == 0 {
		httputil.Error(w, http.StatusBadRequest, "empty_items")
		return
	}

	// Validate discount range
	for _, item := range req.Items {
		if item.Sconto < 0 || item.Sconto > 20 {
			httputil.JSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error":   "discount_out_of_range",
				"id_rack": item.IDRack,
				"message": "discount must be between 0 and 20",
			})
			return
		}
	}

	logger := logging.FromContext(r.Context())

	// Fetch old values for diff detection and HubSpot audit
	type oldRack struct {
		IDRack            int
		Name              string
		OldSconto         float64
		IDCliFatturazione int
	}
	oldMap := make(map[int]oldRack)

	for _, item := range req.Items {
		var name string
		var oldSconto float64
		var idCli int
		err := h.grappaDB.QueryRowContext(r.Context(), `
			SELECT r.name, r.sconto, r.id_anagrafica
			FROM racks r WHERE r.id_rack = ?`, item.IDRack).Scan(&name, &oldSconto, &idCli)
		if err == nil {
			oldMap[item.IDRack] = oldRack{
				IDRack:            item.IDRack,
				Name:              name,
				OldSconto:         oldSconto,
				IDCliFatturazione: idCli,
			}
		}
	}

	tx, err := h.grappaDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "batch_update_rack_discounts_begin", err)
		return
	}
	defer h.rollbackTx(r, tx, "batch_update_rack_discounts")

	for _, item := range req.Items {
		_, err := tx.ExecContext(r.Context(),
			`UPDATE racks SET sconto = ? WHERE id_rack = ?`,
			item.Sconto, item.IDRack)
		if err != nil {
			h.dbFailure(w, r, "batch_update_rack_discounts_exec", err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "batch_update_rack_discounts_commit", err)
		return
	}

	// HubSpot audit (note + task, per customer with changes)
	if h.hubspot != nil {
		// Group changes by customer
		customerChanges := make(map[int][]oldRack)
		for _, item := range req.Items {
			old, ok := oldMap[item.IDRack]
			if !ok || old.OldSconto == item.Sconto {
				continue
			}
			// Store new sconto in a copy
			entry := old
			entry.OldSconto = old.OldSconto
			customerChanges[old.IDCliFatturazione] = append(customerChanges[old.IDCliFatturazione], entry)
		}

		for grappaCustomerID, changes := range customerChanges {
			companyID, lookupErr := h.hubspot.LookupCompanyID(r.Context(), grappaCustomerID)
			if lookupErr != nil {
				logger.Warn("hubspot company lookup failed",
					"component", "listini", "customer_id", grappaCustomerID, "error", lookupErr)
				continue
			}

			var rows []string
			for _, changed := range changes {
				// Find new sconto from request
				for _, item := range req.Items {
					if item.IDRack == changed.IDRack {
						rows = append(rows, fmt.Sprintf("<tr><td>%s</td><td>%g%%</td><td>%g%%</td></tr>",
							changed.Name, changed.OldSconto, item.Sconto))
						break
					}
				}
			}
			noteBody := fmt.Sprintf("<table><tr><th>Rack</th><th>Vecchio</th><th>Nuovo</th></tr>%s</table>",
				strings.Join(rows, ""))

			h.hubspot.CreateNoteAndTaskAsync(r.Context(), companyID,
				noteBody,
				"Aggiornamento sconto energia",
				noteBody,
				"eva.grimaldi@cdlan.it",
			)
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
