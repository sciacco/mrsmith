package grappadcim

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

var errFiberAssignmentConflict = errors.New("fiber assignment conflict")

func (h *Handler) handleAssignFiber(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_fiber_id")
		return
	}
	var body FiberAssignmentInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_fiber_assignment_payload")
		return
	}
	if body.LeftPortID != nil && *body.LeftPortID <= 0 || body.RightPortID != nil && *body.RightPortID <= 0 {
		invalidRequest(w, "invalid_port_id")
		return
	}
	if body.LeftPortID != nil && body.RightPortID != nil && *body.LeftPortID == *body.RightPortID {
		invalidRequest(w, "fiber_ports_must_differ")
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		var oldLeft, oldRight sql.NullInt64
		if err := tx.QueryRowContext(r.Context(), `SELECT left_port_id, right_port_id FROM fibers WHERE id = ? FOR UPDATE`, id).Scan(&oldLeft, &oldRight); err != nil {
			return err
		}
		portIDs := uniquePositiveInts(body.LeftPortID, body.RightPortID, nullableInt(oldLeft), nullableInt(oldRight))
		if len(portIDs) > 0 {
			query := fmt.Sprintf(`SELECT id, cable_fiber_id FROM ports WHERE id IN (%s) FOR UPDATE`, placeholders(len(portIDs)))
			args := make([]any, len(portIDs))
			for i, portID := range portIDs {
				args[i] = portID
			}
			rows, err := tx.QueryContext(r.Context(), query, args...)
			if err != nil {
				return err
			}
			defer rows.Close()
			seen := map[int]bool{}
			for rows.Next() {
				var portID int
				var assigned sql.NullInt64
				if err := rows.Scan(&portID, &assigned); err != nil {
					return err
				}
				seen[portID] = true
				if assigned.Valid && int(assigned.Int64) != id && containsInt(targetPortIDs(body), portID) {
					return errFiberAssignmentConflict
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}
			for _, target := range targetPortIDs(body) {
				if !seen[target] {
					return sql.ErrNoRows
				}
			}
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE ports SET cable_fiber_id = NULL, status = 'Empty' WHERE cable_fiber_id = ?`, id); err != nil {
			return err
		}
		if body.LeftPortID != nil {
			if _, err := tx.ExecContext(r.Context(), `UPDATE ports SET cable_fiber_id = ?, status = 'Linked' WHERE id = ?`, id, *body.LeftPortID); err != nil {
				return err
			}
		}
		if body.RightPortID != nil {
			if _, err := tx.ExecContext(r.Context(), `UPDATE ports SET cable_fiber_id = ?, status = 'Linked' WHERE id = ?`, id, *body.RightPortID); err != nil {
				return err
			}
		}
		status := "Libera"
		if body.LeftPortID != nil || body.RightPortID != nil {
			status = "Occupata"
		}
		result, err := tx.ExecContext(r.Context(), `UPDATE fibers SET left_port_id = ?, right_port_id = ?, status = ? WHERE id = ?`, body.LeftPortID, body.RightPortID, status, id)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return sql.ErrNoRows
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "fiber_or_port_not_found")
			return
		}
		if err == errFiberAssignmentConflict {
			httputil.Error(w, http.StatusConflict, "fiber_assignment_conflict")
			return
		}
		h.dbFailure(w, r, "assign_fiber", err, "fiber_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Fibra assegnata."})
}

func uniquePositiveInts(values ...*int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, value := range values {
		if value == nil || *value <= 0 || seen[*value] {
			continue
		}
		seen[*value] = true
		out = append(out, *value)
	}
	return out
}

func targetPortIDs(body FiberAssignmentInput) []int {
	return uniquePositiveInts(body.LeftPortID, body.RightPortID)
}

func containsInt(values []int, value int) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func (h *Handler) handleListPorts(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	where := []string{"1=1"}
	args := []any{}
	if raw := strings.TrimSpace(r.URL.Query().Get("plenumId")); raw != "" {
		id, err := parsePositiveString(raw)
		if err != nil {
			invalidRequest(w, "invalid_plenum_id")
			return
		}
		where = append(where, "(p.plenum_id = ? OR ps.plenums_id = ?)")
		args = append(args, id, id)
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && status != "all" {
		where = append(where, "p.status = ?")
		args = append(args, status)
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("availableForFiberId")); raw != "" {
		fiberID, err := parsePositiveString(raw)
		if err != nil {
			invalidRequest(w, "invalid_fiber_id")
			return
		}
		where = append(where, "(p.cable_fiber_id IS NULL OR p.cable_fiber_id = ?)")
		args = append(args, fiberID)
	}
	rows, err := h.grappa.QueryContext(r.Context(), `
		SELECT p.id, p.slots_id, p.num, p.status, p.pl_slots_id, p.pl_port_num,
		       p.rack_id, r.name, p.plenum_id, p.device_id, p.name, p.cable_fiber_id
		FROM ports p
		LEFT JOIN racks r ON r.id_rack = p.rack_id
		LEFT JOIN pl_slots ps ON ps.id = p.pl_slots_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY COALESCE(r.name, ''), p.unit ASC, p.num ASC, p.id ASC`, args...)
	if err != nil {
		h.dbFailure(w, r, "list_ports", err)
		return
	}
	defer rows.Close()
	items := []PortItem{}
	for rows.Next() {
		item, err := scanPort(rows)
		if err != nil {
			h.dbFailure(w, r, "list_ports_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_ports") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

type portScanner interface {
	Scan(dest ...any) error
}

func scanPort(scanner portScanner) (PortItem, error) {
	var item PortItem
	var num, plSlotID, plPortNum, rackID, plenumID, deviceID, fiberID sql.NullInt64
	var rackName, name sql.NullString
	if err := scanner.Scan(
		&item.ID,
		&item.SlotID,
		&num,
		&item.Status,
		&plSlotID,
		&plPortNum,
		&rackID,
		&rackName,
		&plenumID,
		&deviceID,
		&name,
		&fiberID,
	); err != nil {
		return item, err
	}
	item.Number = nullableInt(num)
	item.PlSlotID = nullableInt(plSlotID)
	item.PlPortNumber = nullableInt(plPortNum)
	item.RackID = nullableInt(rackID)
	item.RackName = nullableString(rackName)
	item.PlenumID = nullableInt(plenumID)
	item.DeviceID = nullableInt(deviceID)
	item.Name = nullableString(name)
	item.CableFiberID = nullableInt(fiberID)
	item.Label = fiberLabel(item.ID, name, num)
	if item.RackName != nil {
		item.Label = *item.RackName + " - " + item.Label
	}
	return item, nil
}
