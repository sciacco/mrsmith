package grappadcim

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListRackUnits(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_rack_id")
		return
	}
	items, err := h.listUnitsForRack(r, id)
	if err != nil {
		h.dbFailure(w, r, "list_rack_units", err, "rack_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) listUnitsForRack(r *http.Request, rackID int) ([]RackUnit, error) {
	rows, err := h.grappa.QueryContext(r.Context(), `SELECT id, num, racks_id, device_id FROM units WHERE racks_id = ? ORDER BY num ASC, id ASC`, rackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RackUnit{}
	for rows.Next() {
		var item RackUnit
		var num, rack, device sql.NullInt64
		if err := rows.Scan(&item.ID, &num, &rack, &device); err != nil {
			return nil, err
		}
		item.Num = nullableInt(num)
		item.RackID = nullableInt(rack)
		item.DeviceID = nullableInt(device)
		items = append(items, item)
	}
	return items, rows.Err()
}
