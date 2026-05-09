package grappadcim

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const (
	plenumMatrixCables = 2
	plenumMatrixSlots  = 12
	plenumMatrixFibers = 12
)

func (h *Handler) handleListPlenums(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	where := []string{"1=1"}
	args := []any{}
	if raw := strings.TrimSpace(r.URL.Query().Get("datacenterId")); raw != "" {
		id, err := parsePositiveString(raw)
		if err != nil {
			invalidRequest(w, "invalid_datacenter_id")
			return
		}
		where = append(where, "p.datacenter_id = ?")
		args = append(args, id)
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		where = append(where, "(p.name LIKE ? OR p.isle LIKE ? OR p.type LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	rows, err := h.grappa.QueryContext(r.Context(), plenumSelectSQL()+" WHERE "+strings.Join(where, " AND ")+" GROUP BY "+plenumGroupSQL()+" ORDER BY d.name ASC, p.name ASC, p.id ASC", args...)
	if err != nil {
		h.dbFailure(w, r, "list_plenums", err)
		return
	}
	defer rows.Close()
	items := []Plenum{}
	for rows.Next() {
		item, err := scanPlenum(rows)
		if err != nil {
			h.dbFailure(w, r, "list_plenums_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_plenums") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetPlenum(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_plenum_id")
		return
	}
	item, found, err := h.getPlenum(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_plenum", err, "plenum_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "plenum_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleCreatePlenum(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body PlenumInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_plenum_payload")
		return
	}
	if body.DatacenterID <= 0 {
		invalidRequest(w, "datacenter_required")
		return
	}
	status := strings.TrimSpace(body.Status)
	if status == "" {
		status = "Attivo"
	}
	result, err := h.grappa.ExecContext(r.Context(), `
		INSERT INTO plenums (name, isle, type, datacenter_id, status)
		VALUES (?, ?, ?, ?, ?)`,
		optionalTrimmed(body.Name), optionalTrimmed(body.Isle), optionalTrimmed(body.Type), body.DatacenterID, status,
	)
	if err != nil {
		h.dbFailure(w, r, "create_plenum", err)
		return
	}
	id, _ := result.LastInsertId()
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Plenum creato."})
}

func (h *Handler) handleUpdatePlenum(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_plenum_id")
		return
	}
	var body PlenumPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_plenum_payload")
		return
	}
	sets := []string{}
	args := []any{}
	if body.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, optionalTrimmed(body.Name))
	}
	if body.Isle != nil {
		sets = append(sets, "isle = ?")
		args = append(args, optionalTrimmed(body.Isle))
	}
	if body.Type != nil {
		sets = append(sets, "type = ?")
		args = append(args, optionalTrimmed(body.Type))
	}
	if body.Status != nil {
		status := strings.TrimSpace(*body.Status)
		if status == "" {
			invalidRequest(w, "plenum_status_required")
			return
		}
		sets = append(sets, "status = ?")
		args = append(args, status)
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Nessuna modifica."})
		return
	}
	args = append(args, id)
	result, err := h.grappa.ExecContext(r.Context(), "UPDATE plenums SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
	if err != nil {
		h.dbFailure(w, r, "update_plenum", err, "plenum_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "plenum_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Plenum aggiornato."})
}

func (h *Handler) handleDeletePlenum(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_plenum_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	deps, err := h.plenumDependencies(r, id)
	if err != nil {
		h.dbFailure(w, r, "plenum_delete_dependencies", err, "plenum_id", id)
		return
	}
	if !deps.Allowed {
		httputil.JSON(w, http.StatusConflict, deps)
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `DELETE FROM pl_slots WHERE plenums_id = ?`, id); err != nil {
			return err
		}
		result, err := tx.ExecContext(r.Context(), `DELETE FROM plenums WHERE id = ?`, id)
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
			httputil.Error(w, http.StatusNotFound, "plenum_not_found")
			return
		}
		h.dbFailure(w, r, "delete_plenum", err, "plenum_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Plenum eliminato."})
}

func (h *Handler) handleGetPlenumMatrix(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_plenum_id")
		return
	}
	plenum, found, err := h.getPlenum(r, id)
	if err != nil {
		h.dbFailure(w, r, "plenum_matrix_detail", err, "plenum_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "plenum_not_found")
		return
	}
	matrix, err := h.buildPlenumMatrix(r, plenum)
	if err != nil {
		h.dbFailure(w, r, "plenum_matrix", err, "plenum_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, matrix)
}

func (h *Handler) handleInitializePlenumMatrix(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_plenum_id")
		return
	}
	created := 0
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		var lockedID int
		if err := tx.QueryRowContext(r.Context(), `SELECT id FROM plenums WHERE id = ? FOR UPDATE`, id).Scan(&lockedID); err != nil {
			if err == sql.ErrNoRows {
				return sql.ErrNoRows
			}
			return err
		}
		if lockedID == 0 {
			return sql.ErrNoRows
		}
		rows, err := tx.QueryContext(r.Context(), `SELECT cable, num FROM pl_slots WHERE plenums_id = ? FOR UPDATE`, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		present := map[string]bool{}
		for rows.Next() {
			var cable, num sql.NullInt64
			if err := rows.Scan(&cable, &num); err != nil {
				return err
			}
			if cable.Valid && num.Valid {
				present[fmt.Sprintf("%d:%d", cable.Int64, num.Int64)] = true
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for cable := 1; cable <= plenumMatrixCables; cable++ {
			for num := 1; num <= plenumMatrixSlots; num++ {
				key := fmt.Sprintf("%d:%d", cable, num)
				if present[key] {
					continue
				}
				if _, err := tx.ExecContext(r.Context(), `INSERT INTO pl_slots (plenums_id, num, cable, type, status) VALUES (?, ?, ?, ?, ?)`, id, num, cable, "FO", "Empty"); err != nil {
					return err
				}
				created++
			}
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "plenum_not_found")
			return
		}
		h.dbFailure(w, r, "initialize_plenum_matrix", err, "plenum_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: fmt.Sprintf("Matrice inizializzata: %d terminazioni create.", created)})
}

func (h *Handler) getPlenum(r *http.Request, id int) (Plenum, bool, error) {
	row := h.grappa.QueryRowContext(r.Context(), plenumSelectSQL()+" WHERE p.id = ? GROUP BY "+plenumGroupSQL(), id)
	item, err := scanPlenum(row)
	if err == sql.ErrNoRows {
		return Plenum{}, false, nil
	}
	return item, err == nil, err
}

func (h *Handler) buildPlenumMatrix(r *http.Request, plenum Plenum) (PlenumMatrix, error) {
	rows, err := h.grappa.QueryContext(r.Context(), `SELECT id, cable, num, type, status FROM pl_slots WHERE plenums_id = ?`, plenum.ID)
	if err != nil {
		return PlenumMatrix{}, err
	}
	defer rows.Close()
	type slotRecord struct {
		id     int
		cable  int
		num    int
		typ    sql.NullString
		status sql.NullString
	}
	slots := map[string]slotRecord{}
	slotIDs := []int{}
	for rows.Next() {
		var rec slotRecord
		var cable, num sql.NullInt64
		if err := rows.Scan(&rec.id, &cable, &num, &rec.typ, &rec.status); err != nil {
			return PlenumMatrix{}, err
		}
		if cable.Valid && num.Valid && cable.Int64 >= 1 && cable.Int64 <= plenumMatrixCables && num.Int64 >= 1 && num.Int64 <= plenumMatrixSlots {
			rec.cable = int(cable.Int64)
			rec.num = int(num.Int64)
			slots[fmt.Sprintf("%d:%d", rec.cable, rec.num)] = rec
			slotIDs = append(slotIDs, rec.id)
		}
	}
	if err := rows.Err(); err != nil {
		return PlenumMatrix{}, err
	}
	portsByCell := map[string]PortItem{}
	if len(slotIDs) > 0 {
		query := fmt.Sprintf(`
			SELECT p.id, p.slots_id, p.num, p.status, p.pl_slots_id, p.pl_port_num,
			       p.rack_id, r.name, p.plenum_id, p.device_id, p.name, p.cable_fiber_id
			FROM ports p
			LEFT JOIN racks r ON r.id_rack = p.rack_id
			WHERE p.pl_slots_id IN (%s)
			ORDER BY p.id ASC`, placeholders(len(slotIDs)))
		args := make([]any, len(slotIDs))
		for i, id := range slotIDs {
			args[i] = id
		}
		portRows, err := h.grappa.QueryContext(r.Context(), query, args...)
		if err != nil {
			return PlenumMatrix{}, err
		}
		defer portRows.Close()
		for portRows.Next() {
			item, err := scanPort(portRows)
			if err != nil {
				return PlenumMatrix{}, err
			}
			if item.PlSlotID != nil && item.PlPortNumber != nil {
				portsByCell[fmt.Sprintf("%d:%d", *item.PlSlotID, *item.PlPortNumber)] = item
			}
		}
		if err := portRows.Err(); err != nil {
			return PlenumMatrix{}, err
		}
	}
	var mapOnly int
	if err := h.grappa.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM crossconnects WHERE mmr_id = ?`, plenum.DatacenterID).Scan(&mapOnly); err != nil {
		return PlenumMatrix{}, err
	}
	matrix := PlenumMatrix{
		Plenum:         plenum,
		Slots:          []PlenumMatrixSlot{},
		ExpectedSlots:  plenumMatrixCables * plenumMatrixSlots,
		ExpectedCells:  plenumMatrixCables * plenumMatrixSlots * plenumMatrixFibers,
		MapOnlyRecords: mapOnly,
	}
	for cable := 1; cable <= plenumMatrixCables; cable++ {
		for num := 1; num <= plenumMatrixSlots; num++ {
			key := fmt.Sprintf("%d:%d", cable, num)
			rec, ok := slots[key]
			slot := PlenumMatrixSlot{Cable: cable, Number: num, Missing: !ok, Cells: []PlenumMatrixCell{}}
			if ok {
				slot.ID = &rec.id
				slot.Type = nullableString(rec.typ)
				slot.Status = nullableString(rec.status)
			} else {
				matrix.Incomplete = true
			}
			for fiber := 1; fiber <= plenumMatrixFibers; fiber++ {
				cell := PlenumMatrixCell{Cable: cable, SlotNumber: num, Fiber: fiber, Status: "Missing"}
				if !slot.Missing {
					cell.Status = "Empty"
					port := portsByCell[fmt.Sprintf("%d:%d", rec.id, fiber)]
					if port.ID != 0 {
						cell.PortID = &port.ID
						cell.PortLabel = &port.Label
						cell.FiberID = port.CableFiberID
						cell.Status = port.Status
					}
				}
				if cell.Status == "Missing" {
					matrix.MissingCells++
				} else if cell.Status == "Empty" || strings.TrimSpace(cell.Status) == "" {
					matrix.FreeCells++
				} else {
					matrix.AssignedCells++
				}
				slot.Cells = append(slot.Cells, cell)
			}
			matrix.Slots = append(matrix.Slots, slot)
		}
	}
	return matrix, nil
}

func (h *Handler) plenumDependencies(r *http.Request, id int) (DependencySummary, error) {
	return h.runDependencyChecks(r, id, []dependencyCheck{
		{
			Key:   "linkedPorts",
			Label: "Porte collegate",
			Query: `SELECT COUNT(*)
				FROM ports p
				LEFT JOIN pl_slots ps ON ps.id = p.pl_slots_id
				WHERE ps.plenums_id = ?
				  AND (p.cable_fiber_id IS NOT NULL OR p.fo_in_id IS NOT NULL OR p.fo_out_id IS NOT NULL OR LOWER(TRIM(p.status)) <> 'empty')`,
		},
	})
}

func plenumSelectSQL() string {
	return `
		SELECT p.id, p.name, p.isle, p.type, p.datacenter_id, d.name, p.status,
		       COUNT(DISTINCT ps.id) AS slot_count,
		       COUNT(DISTINCT CASE WHEN p2.cable_fiber_id IS NOT NULL OR LOWER(TRIM(p2.status)) <> 'empty' THEN p2.id END) AS linked_port_count
		FROM plenums p
		LEFT JOIN datacenter d ON d.id_datacenter = p.datacenter_id
		LEFT JOIN pl_slots ps ON ps.plenums_id = p.id
		LEFT JOIN ports p2 ON p2.pl_slots_id = ps.id`
}

func plenumGroupSQL() string {
	return `p.id, p.name, p.isle, p.type, p.datacenter_id, d.name, p.status`
}

type plenumScanner interface {
	Scan(dest ...any) error
}

func scanPlenum(scanner plenumScanner) (Plenum, error) {
	var item Plenum
	var name, isle, typ, dcName sql.NullString
	if err := scanner.Scan(&item.ID, &name, &isle, &typ, &item.DatacenterID, &dcName, &item.Status, &item.SlotCount, &item.LinkedPortCount); err != nil {
		return item, err
	}
	item.Name = nullableString(name)
	item.Isle = nullableString(isle)
	item.Type = nullableString(typ)
	item.DatacenterName = nullableString(dcName)
	return item, nil
}
