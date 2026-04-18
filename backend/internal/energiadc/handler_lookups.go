package energiadc

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListCustomers(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(), `
		SELECT cf.id, cf.intestazione
		FROM cli_fatturazione cf
		WHERE cf.id IN (
			SELECT DISTINCT r.id_anagrafica
			FROM racks r
			JOIN rack_sockets rs ON rs.rack_id = r.id_rack
			WHERE r.stato = 'attivo'
		)
		ORDER BY cf.intestazione ASC`)
	if err != nil {
		h.dbFailure(w, r, "list_customers", err)
		return
	}
	defer rows.Close()

	result := make([]lookupItem, 0)
	for rows.Next() {
		var item lookupItem
		var name sql.NullString
		if err := rows.Scan(&item.ID, &name); err != nil {
			h.dbFailure(w, r, "list_customers_scan", err)
			return
		}
		item.Name = cleanString(name)
		result = append(result, item)
	}
	if !h.rowsDone(w, r, rows, "list_customers") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

func (h *Handler) handleListSites(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	customerID, ok := h.parsePathInt(w, r, "customerId", "invalid_customer_id")
	if !ok {
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(), `
		SELECT DISTINCT db.id, db.name
		FROM dc_build db
		JOIN datacenter d ON d.dc_build_id = db.id
		JOIN racks r ON r.id_datacenter = d.id_datacenter
		JOIN rack_sockets rs ON rs.rack_id = r.id_rack
		WHERE r.id_anagrafica = ?
		  AND r.stato = 'attivo'
		ORDER BY db.name ASC, db.id ASC`, customerID)
	if err != nil {
		h.dbFailure(w, r, "list_sites", err, "customer_id", customerID)
		return
	}
	defer rows.Close()

	result := make([]lookupItem, 0)
	for rows.Next() {
		var item lookupItem
		var name sql.NullString
		if err := rows.Scan(&item.ID, &name); err != nil {
			h.dbFailure(w, r, "list_sites_scan", err, "customer_id", customerID)
			return
		}
		item.Name = cleanString(name)
		result = append(result, item)
	}
	if !h.rowsDone(w, r, rows, "list_sites") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

func (h *Handler) handleListRooms(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	siteID, ok := h.parsePathInt(w, r, "siteId", "invalid_site_id")
	if !ok {
		return
	}
	customerID, ok := h.parseRequiredQueryInt(w, r, "customerId", "invalid_customer_id")
	if !ok {
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(), `
		SELECT DISTINCT d.id_datacenter, d.name
		FROM datacenter d
		JOIN racks r ON r.id_datacenter = d.id_datacenter
		JOIN rack_sockets rs ON rs.rack_id = r.id_rack
		WHERE d.dc_build_id = ?
		  AND r.id_anagrafica = ?
		  AND r.stato = 'attivo'
		ORDER BY d.name ASC, d.id_datacenter ASC`, siteID, customerID)
	if err != nil {
		h.dbFailure(w, r, "list_rooms", err, "site_id", siteID, "customer_id", customerID)
		return
	}
	defer rows.Close()

	result := make([]lookupItem, 0)
	for rows.Next() {
		var item lookupItem
		var name sql.NullString
		if err := rows.Scan(&item.ID, &name); err != nil {
			h.dbFailure(w, r, "list_rooms_scan", err, "site_id", siteID, "customer_id", customerID)
			return
		}
		item.Name = cleanString(name)
		result = append(result, item)
	}
	if !h.rowsDone(w, r, rows, "list_rooms") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

func (h *Handler) handleListRacks(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	roomID, ok := h.parsePathInt(w, r, "roomId", "invalid_room_id")
	if !ok {
		return
	}
	customerID, ok := h.parseRequiredQueryInt(w, r, "customerId", "invalid_customer_id")
	if !ok {
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(), `
		SELECT DISTINCT r.id_rack, r.name
		FROM racks r
		JOIN rack_sockets rs ON rs.rack_id = r.id_rack
		WHERE r.id_datacenter = ?
		  AND r.id_anagrafica = ?
		  AND r.stato = 'attivo'
		ORDER BY r.name ASC, r.id_rack ASC`, roomID, customerID)
	if err != nil {
		h.dbFailure(w, r, "list_racks", err, "room_id", roomID, "customer_id", customerID)
		return
	}
	defer rows.Close()

	result := make([]lookupItem, 0)
	for rows.Next() {
		var item lookupItem
		var name sql.NullString
		if err := rows.Scan(&item.ID, &name); err != nil {
			h.dbFailure(w, r, "list_racks_scan", err, "room_id", roomID, "customer_id", customerID)
			return
		}
		item.Name = cleanString(name)
		result = append(result, item)
	}
	if !h.rowsDone(w, r, rows, "list_racks") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}
