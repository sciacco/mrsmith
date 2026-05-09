package grappadcim

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

var (
	errRackUnitDecreaseBlocked = errors.New("rack unit decrease blocked")
	errRackUnitsInconsistent   = errors.New("rack units inconsistent")
)

func (h *Handler) handleListRacks(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	where := []string{"1=1"}
	args := []any{}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status == "active" || status == "" {
		where = append(where, activeStateSQL("r.stato"), "r.data_cessazione IS NULL")
	} else if status != "all" {
		where = append(where, "r.stato = ?")
		args = append(args, status)
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("datacenterId")); raw != "" {
		id, err := parsePositiveString(raw)
		if err != nil {
			invalidRequest(w, "invalid_datacenter_id")
			return
		}
		where = append(where, "r.id_datacenter = ?")
		args = append(args, id)
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("buildingId")); raw != "" {
		id, err := parsePositiveString(raw)
		if err != nil {
			invalidRequest(w, "invalid_building_id")
			return
		}
		where = append(where, "d.dc_build_id = ?")
		args = append(args, id)
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		where = append(where, "(r.name LIKE ? OR r.serialnumber LIKE ? OR r.codice_ordine LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	rows, err := h.grappa.QueryContext(r.Context(), rackSelectSQL()+` WHERE `+strings.Join(where, " AND ")+` GROUP BY `+rackGroupSQL()+` ORDER BY db.name ASC, d.name ASC, r.name ASC, r.id_rack ASC`, args...)
	if err != nil {
		h.dbFailure(w, r, "list_racks", err)
		return
	}
	defer rows.Close()
	items := []RackListItem{}
	for rows.Next() {
		item, err := scanRack(rows)
		if err != nil {
			h.dbFailure(w, r, "list_racks_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_racks") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetRack(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_rack_id")
		return
	}
	item, found, err := h.getRack(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_rack", err, "rack_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "rack_not_found")
		return
	}
	units, err := h.listUnitsForRack(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_rack_units", err, "rack_id", id)
		return
	}
	sockets, err := h.listSocketsForRack(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_rack_sockets", err, "rack_id", id)
		return
	}
	media, err := h.listMediaForRack(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_rack_media", err, "rack_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, RackDetail{RackListItem: item, Units: units, Sockets: sockets, Media: media})
}

func (h *Handler) handleCreateRack(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body RackInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_rack_payload")
		return
	}
	if err := validateRackInput(body); err != nil {
		invalidRequest(w, err.Error())
		return
	}
	var id int64
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		effectiveIsletID := body.IsletID
		if body.PositionID != nil {
			isletID, err := h.ensureRackPositionAvailableTx(r, tx, 0, body.DatacenterID, *body.PositionID, body.IsletID, body.Type, body.Position, false)
			if err != nil {
				return err
			}
			effectiveIsletID = &isletID
		}
		result, err := tx.ExecContext(r.Context(), `
			INSERT INTO racks
				(name, unit, id_anagrafica, id_datacenter, stato, magnetotermico, ampere, floor, island, type, pos,
				 racknum, positions_id, shared, reserved, note, codice_ordine, sold_power, serialnumber,
				 committed_power, variable_billing, islet_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			strings.TrimSpace(body.Name), body.UnitCount, body.CustomerID, body.DatacenterID, statusOrActive(body.Status),
			optionalTrimmed(body.Magnetotermico), body.Ampere, body.Floor, body.Island, strings.TrimSpace(body.Type),
			strings.TrimSpace(body.Position), body.RackNumber, body.PositionID, optionalTrimmed(body.Shared),
			optionalTrimmed(body.Reserved), optionalTrimmed(body.Note), optionalTrimmed(body.OrderCode), body.SoldPower,
			optionalTrimmed(body.SerialNumber), body.CommittedPower, body.VariableBilling, effectiveIsletID,
		)
		if err != nil {
			return err
		}
		id, _ = result.LastInsertId()
		for i := 1; i <= body.UnitCount; i++ {
			if _, err := tx.ExecContext(r.Context(), `INSERT INTO units (num, racks_id) VALUES (?, ?)`, i, id); err != nil {
				return err
			}
		}
		socketCount := 0
		if body.SocketCount != nil {
			socketCount = *body.SocketCount
		}
		for i := 1; i <= socketCount; i++ {
			if _, err := tx.ExecContext(r.Context(), `INSERT INTO rack_sockets (rack_id, status, posizione) VALUES (?, 'Acceso', ?)`, id, fmt.Sprintf("PDU %d", i)); err != nil {
				return err
			}
		}
		if body.PositionID != nil {
			if _, err := tx.ExecContext(r.Context(), `UPDATE positions SET status = 'occupied' WHERE id = ?`, *body.PositionID); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if err == errBadRequest {
			invalidRequest(w, "rack_position_conflict")
			return
		}
		h.dbFailure(w, r, "create_rack", err)
		return
	}
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Rack creato."})
}

func (h *Handler) handleUpdateRack(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_rack_id")
		return
	}
	var body RackPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_rack_payload")
		return
	}
	sets, args, err := rackPatch(body)
	if err != nil {
		invalidRequest(w, err.Error())
		return
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Nessuna modifica."})
		return
	}
	args = append(args, id)
	if body.UnitCount != nil {
		if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
			var currentUnitCount int
			if err := tx.QueryRowContext(r.Context(), `SELECT unit FROM racks WHERE id_rack = ? FOR UPDATE`, id).Scan(&currentUnitCount); err != nil {
				return err
			}
			if *body.UnitCount < currentUnitCount {
				return errRackUnitDecreaseBlocked
			}
			if _, err := tx.ExecContext(r.Context(), `UPDATE racks SET `+strings.Join(sets, ", ")+`, last_update = NOW() WHERE id_rack = ?`, args...); err != nil {
				return err
			}
			return h.reconcileRackUnitsTx(r, tx, id, *body.UnitCount)
		}); err != nil {
			switch err {
			case sql.ErrNoRows:
				httputil.Error(w, http.StatusNotFound, "rack_not_found")
			case errRackUnitDecreaseBlocked:
				invalidRequest(w, "rack_unit_decrease_blocked")
			case errRackUnitsInconsistent:
				invalidRequest(w, "rack_units_inconsistent")
			default:
				h.dbFailure(w, r, "update_rack", err, "rack_id", id)
			}
			return
		}
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Rack aggiornato."})
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE racks SET `+strings.Join(sets, ", ")+`, last_update = NOW() WHERE id_rack = ?`, args...)
	if err != nil {
		h.dbFailure(w, r, "update_rack", err, "rack_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "rack_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Rack aggiornato."})
}

func (h *Handler) handleMoveRack(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_rack_id")
		return
	}
	var body RackMoveInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_rack_move_payload")
		return
	}
	if body.DatacenterID <= 0 || body.PositionID <= 0 || !validRackTypePosition(body.Type, body.Position) {
		invalidRequest(w, "invalid_rack_move_payload")
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		var oldPosition sql.NullInt64
		if err := tx.QueryRowContext(r.Context(), `SELECT positions_id FROM racks WHERE id_rack = ? FOR UPDATE`, id).Scan(&oldPosition); err != nil {
			return err
		}
		samePosition := oldPosition.Valid && oldPosition.Int64 == int64(body.PositionID)
		isletID, err := h.ensureRackPositionAvailableTx(r, tx, id, body.DatacenterID, body.PositionID, body.IsletID, body.Type, body.Position, samePosition)
		if err != nil {
			return err
		}
		if oldPosition.Valid && oldPosition.Int64 != int64(body.PositionID) {
			if _, err := tx.ExecContext(r.Context(), `UPDATE positions SET status = 'free' WHERE id = ?`, oldPosition.Int64); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE positions SET status = 'occupied' WHERE id = ?`, body.PositionID); err != nil {
			return err
		}
		_, err = tx.ExecContext(r.Context(), `
			UPDATE racks SET id_datacenter = ?, positions_id = ?, islet_id = ?, type = ?, pos = ?, last_update = NOW()
			WHERE id_rack = ?`,
			body.DatacenterID, body.PositionID, isletID, strings.TrimSpace(body.Type), strings.TrimSpace(body.Position), id)
		return err
	}); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "rack_not_found")
			return
		}
		if err == errBadRequest {
			httputil.JSON(w, http.StatusConflict, DependencySummary{
				Allowed: false,
				Counts:  map[string]int{"position_conflicts": 1},
				Details: []DependencyDetail{{Label: "Posizione occupata", Count: 1}},
				Message: "La posizione selezionata non e disponibile per questo rack.",
			})
			return
		}
		h.dbFailure(w, r, "rack_move", err, "rack_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Rack spostato."})
}

func (h *Handler) handleCeaseRack(w http.ResponseWriter, r *http.Request) {
	h.ceaseOrDeleteRack(w, r, false)
}

func (h *Handler) handleDeleteRack(w http.ResponseWriter, r *http.Request) {
	h.ceaseOrDeleteRack(w, r, true)
}

func (h *Handler) ceaseOrDeleteRack(w http.ResponseWriter, r *http.Request, delete bool) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_rack_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	deps, err := h.rackDependencies(r, id, delete)
	if err != nil {
		h.dbFailure(w, r, "rack_lifecycle_dependencies", err, "rack_id", id)
		return
	}
	if !deps.Allowed {
		httputil.JSON(w, http.StatusConflict, deps)
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		var positionID sql.NullInt64
		if err := tx.QueryRowContext(r.Context(), `SELECT positions_id FROM racks WHERE id_rack = ? FOR UPDATE`, id).Scan(&positionID); err != nil {
			return err
		}
		if delete {
			if _, err := tx.ExecContext(r.Context(), `DELETE FROM media WHERE unit_id IN (SELECT id FROM units WHERE racks_id = ?)`, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(r.Context(), `DELETE FROM units WHERE racks_id = ?`, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(r.Context(), `DELETE FROM rack_sockets WHERE rack_id = ?`, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(r.Context(), `DELETE FROM racks WHERE id_rack = ?`, id); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(r.Context(), `UPDATE rack_sockets SET status = 'Spento' WHERE rack_id = ?`, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(r.Context(), `UPDATE racks SET stato = 'Cessato', data_cessazione = COALESCE(data_cessazione, NOW()), last_update = NOW() WHERE id_rack = ?`, id); err != nil {
				return err
			}
		}
		if positionID.Valid {
			if _, err := tx.ExecContext(r.Context(), `UPDATE positions SET status = 'free' WHERE id = ?`, positionID.Int64); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "rack_not_found")
			return
		}
		h.dbFailure(w, r, "rack_lifecycle", err, "rack_id", id)
		return
	}
	if delete {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Rack eliminato."})
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Rack cessato."})
}

func (h *Handler) getRack(r *http.Request, id int) (RackListItem, bool, error) {
	row := h.grappa.QueryRowContext(r.Context(), rackSelectSQL()+` WHERE r.id_rack = ? GROUP BY `+rackGroupSQL(), id)
	item, err := scanRack(row)
	if err == sql.ErrNoRows {
		return RackListItem{}, false, nil
	}
	return item, err == nil, err
}

func (h *Handler) listRacksForDatacenter(r *http.Request, datacenterID int) ([]RackListItem, error) {
	rows, err := h.grappa.QueryContext(r.Context(), rackSelectSQL()+` WHERE r.id_datacenter = ? GROUP BY `+rackGroupSQL()+` ORDER BY r.name ASC, r.id_rack ASC`, datacenterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RackListItem{}
	for rows.Next() {
		item, err := scanRack(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func rackSelectSQL() string {
	return `
		SELECT r.id_rack, r.name, r.unit, r.id_anagrafica, r.id_datacenter, d.name, db.name,
		       r.stato, r.magnetotermico, r.ampere, r.floor, r.island, r.type, r.pos, r.racknum,
		       r.positions_id, r.islet_id, r.shared, r.reserved, r.note, r.data_attivazione, r.data_cessazione,
		       r.codice_ordine, r.sold_power, r.serialnumber, r.committed_power, r.variable_billing,
		       COUNT(DISTINCT rs.id)
		FROM racks r
		LEFT JOIN datacenter d ON d.id_datacenter = r.id_datacenter
		LEFT JOIN dc_build db ON db.id = d.dc_build_id
		LEFT JOIN rack_sockets rs ON rs.rack_id = r.id_rack`
}

func rackGroupSQL() string {
	return `r.id_rack, r.name, r.unit, r.id_anagrafica, r.id_datacenter, d.name, db.name,
		r.stato, r.magnetotermico, r.ampere, r.floor, r.island, r.type, r.pos, r.racknum,
		r.positions_id, r.islet_id, r.shared, r.reserved, r.note, r.data_attivazione, r.data_cessazione,
		r.codice_ordine, r.sold_power, r.serialnumber, r.committed_power, r.variable_billing`
}

type rackScanner interface {
	Scan(dest ...any) error
}

func scanRack(scanner rackScanner) (RackListItem, error) {
	var item RackListItem
	var customerID, ampere, floor, island, rackNum, positionID, isletID, variableBilling sql.NullInt64
	var dcName, buildingName, status, magnetotermico, rackType, pos, shared, reserved, note, orderCode, serial sql.NullString
	var activatedAt, ceasedAt sql.NullTime
	var sold, committed sql.NullFloat64
	if err := scanner.Scan(
		&item.ID, &item.Name, &item.UnitCount, &customerID, &item.DatacenterID, &dcName, &buildingName,
		&status, &magnetotermico, &ampere, &floor, &island, &rackType, &pos, &rackNum, &positionID,
		&isletID, &shared, &reserved, &note, &activatedAt, &ceasedAt, &orderCode, &sold, &serial, &committed,
		&variableBilling, &item.SocketCount,
	); err != nil {
		return item, err
	}
	item.CustomerID = nullableInt(customerID)
	item.DatacenterName = nullableString(dcName)
	item.BuildingName = nullableString(buildingName)
	item.Status = nullableString(status)
	item.Magnetotermico = nullableString(magnetotermico)
	item.Ampere = nullableInt(ampere)
	item.Floor = nullableInt(floor)
	item.Island = nullableInt(island)
	item.Type = nullableString(rackType)
	item.Position = nullableString(pos)
	item.RackNumber = nullableInt(rackNum)
	item.PositionID = nullableInt(positionID)
	item.IsletID = nullableInt(isletID)
	item.Shared = nullableString(shared)
	item.Reserved = nullableString(reserved)
	item.Note = nullableString(note)
	item.ActivatedAt = nullableTime(activatedAt)
	item.CeasedAt = nullableTime(ceasedAt)
	item.OrderCode = nullableString(orderCode)
	item.SoldPower = nullableFloat(sold)
	item.SerialNumber = nullableString(serial)
	item.CommittedPower = nullableFloat(committed)
	item.VariableBilling = nullableInt(variableBilling)
	return item, nil
}

func validateRackInput(body RackInput) error {
	if strings.TrimSpace(body.Name) == "" {
		return fmt.Errorf("rack_name_required")
	}
	if body.UnitCount <= 0 || body.UnitCount > 60 {
		return fmt.Errorf("invalid_rack_units")
	}
	if body.DatacenterID <= 0 {
		return fmt.Errorf("datacenter_id_required")
	}
	if !validRackTypePosition(body.Type, body.Position) {
		return fmt.Errorf("invalid_rack_position")
	}
	if body.Status != nil && !allowedLifecycleStatus(*body.Status) {
		return fmt.Errorf("invalid_rack_status")
	}
	if body.SocketCount != nil && (*body.SocketCount < 0 || *body.SocketCount > 16) {
		return fmt.Errorf("invalid_socket_count")
	}
	return nil
}

func validRackTypePosition(rackType string, pos string) bool {
	t := strings.ToLower(strings.TrimSpace(rackType))
	p := strings.ToUpper(strings.TrimSpace(pos))
	if t == "full" {
		return p == "F"
	}
	if t == "half" {
		return p == "A" || p == "B"
	}
	return false
}

func rackPatch(body RackPatch) ([]string, []any, error) {
	sets := []string{}
	args := []any{}
	if body.Name != nil {
		if strings.TrimSpace(*body.Name) == "" {
			return nil, nil, fmt.Errorf("rack_name_required")
		}
		sets = append(sets, "name = ?")
		args = append(args, strings.TrimSpace(*body.Name))
	}
	if body.UnitCount != nil {
		if *body.UnitCount <= 0 || *body.UnitCount > 60 {
			return nil, nil, fmt.Errorf("invalid_rack_units")
		}
		sets = append(sets, "unit = ?")
		args = append(args, *body.UnitCount)
	}
	if body.CustomerID != nil {
		sets = append(sets, "id_anagrafica = ?")
		args = append(args, *body.CustomerID)
	}
	if body.Status != nil {
		if !allowedLifecycleStatus(*body.Status) {
			return nil, nil, fmt.Errorf("invalid_rack_status")
		}
		sets = append(sets, "stato = ?")
		args = append(args, strings.TrimSpace(*body.Status))
	}
	stringFields := []struct {
		column string
		value  *string
	}{
		{"magnetotermico", body.Magnetotermico},
		{"shared", body.Shared},
		{"reserved", body.Reserved},
		{"note", body.Note},
		{"codice_ordine", body.OrderCode},
		{"serialnumber", body.SerialNumber},
	}
	for _, field := range stringFields {
		if field.value != nil {
			sets = append(sets, field.column+" = ?")
			args = append(args, optionalTrimmed(field.value))
		}
	}
	if body.Ampere != nil {
		sets = append(sets, "ampere = ?")
		args = append(args, *body.Ampere)
	}
	if body.SoldPower != nil {
		sets = append(sets, "sold_power = ?")
		args = append(args, *body.SoldPower)
	}
	if body.CommittedPower != nil {
		sets = append(sets, "committed_power = ?")
		args = append(args, *body.CommittedPower)
	}
	if body.VariableBilling != nil {
		sets = append(sets, "variable_billing = ?")
		args = append(args, *body.VariableBilling)
	}
	return sets, args, nil
}

func (h *Handler) ensureRackPositionAvailableTx(r *http.Request, tx *sql.Tx, rackID int, requestedDatacenterID int, positionID int, requestedIsletID *int, rackType string, rackPos string, sameRackPosition bool) (int, error) {
	var lockedPositionID, positionIsletID, positionDatacenterID int
	var positionStatus string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT p.id, p.status, p.islets_id, i.datacenter_id
		FROM positions p
		JOIN islets i ON i.id = p.islets_id
		WHERE p.id = ?
		FOR UPDATE`, positionID).Scan(&lockedPositionID, &positionStatus, &positionIsletID, &positionDatacenterID); err != nil {
		if err == sql.ErrNoRows {
			return 0, errBadRequest
		}
		return 0, err
	}
	if positionDatacenterID != requestedDatacenterID {
		return 0, errBadRequest
	}
	if requestedIsletID != nil && *requestedIsletID != positionIsletID {
		return 0, errBadRequest
	}
	if !rackTargetPositionStatusCompatible(positionStatus, rackType, sameRackPosition) {
		return 0, errBadRequest
	}
	rows, err := tx.QueryContext(r.Context(), `SELECT id_rack, type, pos FROM racks WHERE positions_id = ? AND id_rack <> ? AND `+activeStateSQL("stato")+` FOR UPDATE`, positionID, rackID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	wantedType := strings.ToLower(strings.TrimSpace(rackType))
	wantedPos := strings.ToUpper(strings.TrimSpace(rackPos))
	for rows.Next() {
		var otherID int
		var otherType, otherPos sql.NullString
		if err := rows.Scan(&otherID, &otherType, &otherPos); err != nil {
			return 0, err
		}
		currentType := strings.ToLower(strings.TrimSpace(otherType.String))
		currentPos := strings.ToUpper(strings.TrimSpace(otherPos.String))
		if wantedType == "full" || currentType == "full" || currentPos == wantedPos {
			return 0, errBadRequest
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return positionIsletID, nil
}

func rackTargetPositionStatusCompatible(status string, rackType string, sameRackPosition bool) bool {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	normalizedType := strings.ToLower(strings.TrimSpace(rackType))
	if normalizedStatus == "free" {
		return true
	}
	if normalizedStatus == "occupied" && sameRackPosition {
		return true
	}
	if normalizedStatus == "occupied" && normalizedType == "half" {
		return true
	}
	return false
}

func (h *Handler) rackDependencies(r *http.Request, id int, includePowerHistory bool) (DependencySummary, error) {
	checks := []dependencyCheck{
		{Key: "apparati", Label: "Apparati collegati", Query: `SELECT COUNT(*) FROM apparato WHERE id_rack = ? AND ` + activeStateSQL("stato")},
		{Key: "servers", Label: "Server collegati", Query: `SELECT COUNT(*) FROM server WHERE id_rack = ? AND ` + activeStateSQL("stato")},
		{Key: "optical", Label: "Cassetti ottici collegati", Query: `SELECT COUNT(*) FROM cassetti_ottici WHERE ? IN (id_rack, id_rack_coll) AND ` + activeStateSQL("stato")},
		{Key: "ports", Label: "Porte collegate", Query: `SELECT COUNT(*) FROM ports WHERE rack_id = ? AND LOWER(status) <> 'empty'`},
	}
	if includePowerHistory {
		checks = append(checks, dependencyCheck{
			Key:   "power_readings",
			Label: "Storico potenza socket",
			Query: `SELECT COUNT(*) FROM rack_power_readings rpr JOIN rack_sockets rs ON rs.id = rpr.rack_socket_id WHERE rs.rack_id = ?`,
		})
	}
	return h.runDependencyChecks(r, id, checks)
}

func (h *Handler) reconcileRackUnitsTx(r *http.Request, tx *sql.Tx, rackID int, desired int) error {
	var current int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM units WHERE racks_id = ?`, rackID).Scan(&current); err != nil {
		return err
	}
	if current > desired {
		return errRackUnitDecreaseBlocked
	}
	for i := current + 1; i <= desired; i++ {
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO units (num, racks_id) VALUES (?, ?)`, i, rackID); err != nil {
			return err
		}
	}
	var generatedCount, distinctNums, minNum, maxNum int
	if err := tx.QueryRowContext(r.Context(), `
		SELECT COUNT(*), COUNT(DISTINCT num), COALESCE(MIN(num), 0), COALESCE(MAX(num), 0)
		FROM units
		WHERE racks_id = ?`, rackID).Scan(&generatedCount, &distinctNums, &minNum, &maxNum); err != nil {
		return err
	}
	if generatedCount != desired || distinctNums != desired || minNum != 1 || maxNum != desired {
		return errRackUnitsInconsistent
	}
	return nil
}
