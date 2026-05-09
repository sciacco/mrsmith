package grappadcim

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleGetDatacenterMap(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_datacenter_id")
		return
	}
	dc, found, err := h.getDatacenter(r, id)
	if err != nil {
		h.dbFailure(w, r, "datacenter_map_detail", err, "datacenter_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "datacenter_not_found")
		return
	}
	islets, err := h.listIsletsForDatacenter(r, id)
	if err != nil {
		h.dbFailure(w, r, "datacenter_map_islets", err, "datacenter_id", id)
		return
	}
	positions, err := h.listPositionsForDatacenter(r, id)
	if err != nil {
		h.dbFailure(w, r, "datacenter_map_positions", err, "datacenter_id", id)
		return
	}
	racks, err := h.listRacksForDatacenter(r, id)
	if err != nil {
		h.dbFailure(w, r, "datacenter_map_racks", err, "datacenter_id", id)
		return
	}
	incomplete := len(islets) > 0 && len(positions) == 0
	httputil.JSON(w, http.StatusOK, DatacenterMap{Datacenter: dc, Islets: islets, Positions: positions, Racks: racks, Incomplete: incomplete})
}

func (h *Handler) handleListDatacenterPorts(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_datacenter_id")
		return
	}
	rows, err := h.grappa.QueryContext(r.Context(), `
		SELECT p.id, p.rack_id, r.name, p.slots_id, p.num, p.status, p.name
		FROM ports p
		LEFT JOIN racks r ON r.id_rack = p.rack_id
		WHERE r.id_datacenter = ?
		ORDER BY r.name ASC, p.unit ASC, p.num ASC, p.id ASC`, id)
	if err != nil {
		h.dbFailure(w, r, "list_datacenter_ports", err, "datacenter_id", id)
		return
	}
	defer rows.Close()
	items := []DatacenterPort{}
	for rows.Next() {
		var item DatacenterPort
		var rackID, slotID, portNumber sql.NullInt64
		var rackName, status, label sql.NullString
		if err := rows.Scan(&item.ID, &rackID, &rackName, &slotID, &portNumber, &status, &label); err != nil {
			h.dbFailure(w, r, "list_datacenter_ports_scan", err, "datacenter_id", id)
			return
		}
		item.RackID = nullableInt(rackID)
		item.RackName = nullableString(rackName)
		item.SlotID = nullableInt(slotID)
		item.PortNumber = nullableInt(portNumber)
		item.Status = nullableString(status)
		if label.Valid && strings.TrimSpace(label.String) != "" {
			item.Label = strings.TrimSpace(label.String)
		} else if item.PortNumber != nil {
			item.Label = "Porta"
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_datacenter_ports") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleCreateDatacenterPort(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	datacenterID, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_datacenter_id")
		return
	}
	var body DatacenterPortInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_port_payload")
		return
	}
	if body.RackID == nil || body.SlotID == nil {
		invalidRequest(w, "port_rack_slot_required")
		return
	}
	var exists int
	if err := h.grappa.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM racks WHERE id_rack = ? AND id_datacenter = ?`, *body.RackID, datacenterID).Scan(&exists); err != nil {
		h.dbFailure(w, r, "create_datacenter_port_check", err, "datacenter_id", datacenterID)
		return
	}
	if exists == 0 {
		invalidRequest(w, "rack_not_in_datacenter")
		return
	}
	status := optionalStringValue(body.Status)
	if status == "" {
		status = "Empty"
	}
	result, err := h.grappa.ExecContext(r.Context(), `
		INSERT INTO ports (slots_id, num, status, rack_id, name)
		VALUES (?, ?, ?, ?, ?)`,
		*body.SlotID,
		body.PortNumber,
		status,
		*body.RackID,
		optionalTrimmed(body.Label),
	)
	if err != nil {
		h.dbFailure(w, r, "create_datacenter_port", err, "datacenter_id", datacenterID)
		return
	}
	id, _ := result.LastInsertId()
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Porta aggiunta."})
}
