package grappadcim

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListIslets(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	raw := strings.TrimSpace(r.URL.Query().Get("datacenterId"))
	if raw == "" {
		invalidRequest(w, "datacenter_id_required")
		return
	}
	datacenterID, err := parsePositiveString(raw)
	if err != nil {
		invalidRequest(w, "invalid_datacenter_id")
		return
	}
	items, err := h.listIsletsForDatacenter(r, datacenterID)
	if err != nil {
		h.dbFailure(w, r, "list_islets", err, "datacenter_id", datacenterID)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleCreateIslet(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body IsletInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_islet_payload")
		return
	}
	if err := validateIsletInput(body); err != nil {
		invalidRequest(w, err.Error())
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `
		INSERT INTO islets (datacenter_id, name, rack_num, type, floor, serial, `+"`order`"+`, clifat_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		body.DatacenterID,
		strings.TrimSpace(body.Name),
		body.RackNum,
		strings.TrimSpace(body.Type),
		body.Floor,
		optionalTrimmed(body.Serial),
		optionalTrimmed(body.Order),
		body.CustomerID,
	)
	if err != nil {
		h.dbFailure(w, r, "create_islet", err)
		return
	}
	id, _ := result.LastInsertId()
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Isola creata."})
}

func (h *Handler) handleUpdateIslet(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_islet_id")
		return
	}
	var body IsletPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_islet_payload")
		return
	}
	sets, args, err := isletPatch(body)
	if err != nil {
		invalidRequest(w, err.Error())
		return
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Nessuna modifica."})
		return
	}
	args = append(args, id)
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE islets SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		h.dbFailure(w, r, "update_islet", err, "islet_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "islet_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Isola aggiornata."})
}

// handleUpdateIsletCanvas upserts the representative room-canvas placement of an
// islet (x, y, rotation). Coordinates are operator-authored, non-metric.
func (h *Handler) handleUpdateIsletCanvas(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_islet_id")
		return
	}
	var body IsletCanvasInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_islet_canvas_payload")
		return
	}
	if body.DatacenterID <= 0 {
		invalidRequest(w, "datacenter_id_required")
		return
	}
	rotation := ((body.Rotation % 360) + 360) % 360
	if _, err := h.grappa.ExecContext(r.Context(), `
		INSERT INTO dcim_room_islet_layout (datacenter_id, islet_id, x, y, rotation)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE x = VALUES(x), y = VALUES(y), rotation = VALUES(rotation)`,
		body.DatacenterID, id, body.X, body.Y, rotation); err != nil {
		h.dbFailure(w, r, "update_islet_canvas", err, "islet_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Posizione isola aggiornata."})
}

func (h *Handler) handleDeleteIslet(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_islet_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	deps, err := h.runDependencyChecks(r, id, []dependencyCheck{
		{Key: "occupied_positions", Label: "Posizioni occupate", Query: `SELECT COUNT(*) FROM positions WHERE islets_id = ? AND LOWER(status) = 'occupied'`},
		{Key: "racks", Label: "Rack collegati", Query: `SELECT COUNT(*) FROM racks WHERE islet_id = ? AND ` + activeStateSQL("stato")},
	})
	if err != nil {
		h.dbFailure(w, r, "islet_delete_dependencies", err, "islet_id", id)
		return
	}
	if !deps.Allowed {
		httputil.JSON(w, http.StatusConflict, deps)
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `DELETE FROM positions WHERE islets_id = ?`, id); err != nil {
			return err
		}
		result, err := tx.ExecContext(r.Context(), `DELETE FROM islets WHERE id = ?`, id)
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
			httputil.Error(w, http.StatusNotFound, "islet_not_found")
			return
		}
		h.dbFailure(w, r, "delete_islet", err, "islet_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Isola eliminata."})
}

func (h *Handler) handleListPositions(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_islet_id")
		return
	}
	items, err := h.listPositionsForIslet(r, id)
	if err != nil {
		h.dbFailure(w, r, "list_positions", err, "islet_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleCreatePositionBatch(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	isletID, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_islet_id")
		return
	}
	var body PositionBatchInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_position_batch_payload")
		return
	}
	if body.Count <= 0 || body.Count > 300 {
		invalidRequest(w, "invalid_position_count")
		return
	}
	if strings.TrimSpace(body.Type) == "" {
		invalidRequest(w, "position_type_required")
		return
	}
	var existing int
	if err := h.grappa.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM positions WHERE islets_id = ?`, isletID).Scan(&existing); err != nil {
		h.dbFailure(w, r, "position_batch_existing", err, "islet_id", isletID)
		return
	}
	if existing > 0 {
		httputil.JSON(w, http.StatusConflict, DependencySummary{
			Allowed: false,
			Counts:  map[string]int{"positions": existing},
			Details: []DependencyDetail{{Label: "Posizioni gia presenti", Count: existing}},
			Message: "La creazione massiva e bloccata perche l'isola ha gia posizioni configurate.",
		})
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		for i := 1; i <= body.Count; i++ {
			if _, err := tx.ExecContext(r.Context(), `INSERT INTO positions (status, type, num, islets_id) VALUES ('free', ?, ?, ?)`, strings.TrimSpace(body.Type), i, isletID); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		h.dbFailure(w, r, "position_batch_create", err, "islet_id", isletID)
		return
	}
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: isletID, Message: "Posizioni create."})
}

func (h *Handler) handleUpdatePosition(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_position_id")
		return
	}
	var body PositionPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_position_payload")
		return
	}
	sets, args, err := positionPatch(body)
	if err != nil {
		invalidRequest(w, err.Error())
		return
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Nessuna modifica."})
		return
	}
	args = append(args, id)
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE positions SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		h.dbFailure(w, r, "update_position", err, "position_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "position_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Posizione aggiornata."})
}

func (h *Handler) handleDeletePosition(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_position_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	deps, err := h.runDependencyChecks(r, id, []dependencyCheck{
		{Key: "racks", Label: "Rack collegati", Query: `SELECT COUNT(*) FROM racks WHERE positions_id = ? AND ` + activeStateSQL("stato")},
		{Key: "occupied", Label: "Posizione occupata", Query: `SELECT COUNT(*) FROM positions WHERE id = ? AND LOWER(status) = 'occupied'`},
	})
	if err != nil {
		h.dbFailure(w, r, "position_delete_dependencies", err, "position_id", id)
		return
	}
	if !deps.Allowed {
		httputil.JSON(w, http.StatusConflict, deps)
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `DELETE FROM positions WHERE id = ?`, id)
	if err != nil {
		h.dbFailure(w, r, "delete_position", err, "position_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "position_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Posizione eliminata."})
}

func (h *Handler) listIsletsForDatacenter(r *http.Request, datacenterID int) ([]Islet, error) {
	rows, err := h.grappa.QueryContext(r.Context(), `
		SELECT i.id, i.datacenter_id, i.name, i.rack_num, i.type, i.floor, i.serial, i.`+"`order`"+`, i.clifat_id,
		       COUNT(p.id), SUM(CASE WHEN LOWER(p.status) = 'occupied' THEN 1 ELSE 0 END),
		       rl.x, rl.y, rl.rotation
		FROM islets i
		LEFT JOIN positions p ON p.islets_id = i.id
		LEFT JOIN dcim_room_islet_layout rl ON rl.islet_id = i.id AND rl.datacenter_id = i.datacenter_id
		WHERE i.datacenter_id = ?
		GROUP BY i.id, i.datacenter_id, i.name, i.rack_num, i.type, i.floor, i.serial, i.`+"`order`"+`, i.clifat_id, rl.x, rl.y, rl.rotation
		ORDER BY i.floor ASC, i.name ASC, i.id ASC`, datacenterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Islet{}
	for rows.Next() {
		item, err := scanIslet(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (h *Handler) listPositionsForIslet(r *http.Request, isletID int) ([]Position, error) {
	rows, err := h.grappa.QueryContext(r.Context(), positionsSelectSQL()+` WHERE p.islets_id = ? ORDER BY p.num ASC, p.id ASC`, isletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPositions(rows)
}

func (h *Handler) listPositionsForDatacenter(r *http.Request, datacenterID int) ([]Position, error) {
	rows, err := h.grappa.QueryContext(r.Context(), positionsSelectSQL()+` JOIN islets i ON i.id = p.islets_id WHERE i.datacenter_id = ? ORDER BY i.floor ASC, i.name ASC, p.num ASC`, datacenterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPositions(rows)
}

func positionsSelectSQL() string {
	return `
		SELECT p.id, p.status, p.type, p.num, p.islets_id,
		       r.id_rack, r.name, r.type, r.pos, r.shared
		FROM positions p
		LEFT JOIN racks r ON r.positions_id = p.id AND ` + activeStateSQL("r.stato")
}

func scanIslet(rows *sql.Rows) (Islet, error) {
	var item Islet
	var serial, order sql.NullString
	var customerID sql.NullInt64
	var occupied sql.NullInt64
	var canvasX, canvasY sql.NullFloat64
	var canvasRotation sql.NullInt64
	if err := rows.Scan(&item.ID, &item.DatacenterID, &item.Name, &item.RackNum, &item.Type, &item.Floor, &serial, &order, &customerID, &item.PositionCount, &occupied, &canvasX, &canvasY, &canvasRotation); err != nil {
		return item, err
	}
	item.Serial = nullableString(serial)
	item.Order = nullableString(order)
	item.CustomerID = nullableInt(customerID)
	if occupied.Valid {
		item.OccupiedCount = int(occupied.Int64)
	}
	if canvasX.Valid {
		v := canvasX.Float64
		item.CanvasX = &v
	}
	if canvasY.Valid {
		v := canvasY.Float64
		item.CanvasY = &v
	}
	if canvasRotation.Valid {
		v := int(canvasRotation.Int64)
		item.CanvasRotation = &v
	}
	return item, nil
}

// scanPositions groups the LEFT JOIN rows by physical position (mattonella).
// A Half position with two active racks (A/B) arrives as two rows sharing the
// same position id; we collapse them into one Position carrying up to two
// racks, so callers count and render one tile per physical position.
func scanPositions(rows *sql.Rows) ([]Position, error) {
	items := []Position{}
	byID := map[int]int{}
	for rows.Next() {
		var id, num, isletID int
		var status, ptype string
		var rackID sql.NullInt64
		var rackName, rackType, rackPos, rackShared sql.NullString
		if err := rows.Scan(&id, &status, &ptype, &num, &isletID, &rackID, &rackName, &rackType, &rackPos, &rackShared); err != nil {
			return nil, err
		}
		idx, ok := byID[id]
		if !ok {
			items = append(items, Position{ID: id, Status: status, Type: ptype, Num: num, IsletID: isletID, Racks: []PositionRack{}})
			idx = len(items) - 1
			byID[id] = idx
		}
		if rackID.Valid {
			items[idx].Racks = append(items[idx].Racks, PositionRack{
				ID:     int(rackID.Int64),
				Name:   strings.TrimSpace(rackName.String),
				Type:   strings.TrimSpace(rackType.String),
				Pos:    strings.TrimSpace(rackPos.String),
				Shared: strings.EqualFold(strings.TrimSpace(rackShared.String), "Si"),
			})
		}
	}
	return items, rows.Err()
}

func validateIsletInput(body IsletInput) error {
	if body.DatacenterID <= 0 {
		return fmt.Errorf("datacenter_id_required")
	}
	if strings.TrimSpace(body.Name) == "" {
		return fmt.Errorf("islet_name_required")
	}
	if body.RackNum < 0 {
		return fmt.Errorf("invalid_rack_count")
	}
	if strings.TrimSpace(body.Type) == "" {
		return fmt.Errorf("islet_type_required")
	}
	return nil
}

func isletPatch(body IsletPatch) ([]string, []any, error) {
	sets := []string{}
	args := []any{}
	if body.Name != nil {
		if strings.TrimSpace(*body.Name) == "" {
			return nil, nil, fmt.Errorf("islet_name_required")
		}
		sets = append(sets, "name = ?")
		args = append(args, strings.TrimSpace(*body.Name))
	}
	if body.RackNum != nil {
		if *body.RackNum < 0 {
			return nil, nil, fmt.Errorf("invalid_rack_count")
		}
		sets = append(sets, "rack_num = ?")
		args = append(args, *body.RackNum)
	}
	if body.Type != nil {
		if strings.TrimSpace(*body.Type) == "" {
			return nil, nil, fmt.Errorf("islet_type_required")
		}
		sets = append(sets, "type = ?")
		args = append(args, strings.TrimSpace(*body.Type))
	}
	if body.Floor != nil {
		sets = append(sets, "floor = ?")
		args = append(args, *body.Floor)
	}
	if body.Serial != nil {
		sets = append(sets, "serial = ?")
		args = append(args, optionalTrimmed(body.Serial))
	}
	if body.Order != nil {
		sets = append(sets, "`order` = ?")
		args = append(args, optionalTrimmed(body.Order))
	}
	if body.CustomerID != nil {
		sets = append(sets, "clifat_id = ?")
		args = append(args, *body.CustomerID)
	}
	return sets, args, nil
}

func positionPatch(body PositionPatch) ([]string, []any, error) {
	sets := []string{}
	args := []any{}
	if body.Status != nil {
		status := strings.TrimSpace(*body.Status)
		if status != "free" && status != "occupied" && status != "reserved" {
			return nil, nil, fmt.Errorf("invalid_position_status")
		}
		sets = append(sets, "status = ?")
		args = append(args, status)
	}
	if body.Type != nil {
		if strings.TrimSpace(*body.Type) == "" {
			return nil, nil, fmt.Errorf("position_type_required")
		}
		sets = append(sets, "type = ?")
		args = append(args, strings.TrimSpace(*body.Type))
	}
	if body.Num != nil {
		if *body.Num <= 0 {
			return nil, nil, fmt.Errorf("invalid_position_num")
		}
		sets = append(sets, "num = ?")
		args = append(args, *body.Num)
	}
	return sets, args, nil
}
