package grappadcim

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListDatacenters(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	where := []string{"1=1"}
	args := []any{}
	switch strings.TrimSpace(r.URL.Query().Get("kind")) {
	case "room":
		where = append(where, "COALESCE(d.ismmr, 0) = 0")
	case "mmr":
		where = append(where, "COALESCE(d.ismmr, 0) = 1")
	case "", "all":
	default:
		invalidRequest(w, "invalid_datacenter_kind")
		return
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status == "active" || status == "" {
		where = append(where, activeStateSQL("d.stato"), "d.data_cessazione IS NULL")
	} else if status != "all" {
		where = append(where, "d.stato = ?")
		args = append(args, status)
	}
	if buildingID := strings.TrimSpace(r.URL.Query().Get("buildingId")); buildingID != "" {
		id, err := parsePositiveString(buildingID)
		if err != nil {
			invalidRequest(w, "invalid_building_id")
			return
		}
		where = append(where, "d.dc_build_id = ?")
		args = append(args, id)
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		where = append(where, "(d.name LIKE ? OR d.address LIKE ? OR d.serialnumber LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}

	query := fmt.Sprintf(datacenterSelectSQL()+` WHERE %s GROUP BY `+datacenterGroupSQL()+` ORDER BY db.name ASC, d.floor ASC, d.name ASC, d.id_datacenter ASC`, strings.Join(where, " AND "))
	rows, err := h.grappa.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_datacenters", err)
		return
	}
	defer rows.Close()
	items := []Datacenter{}
	for rows.Next() {
		item, err := scanDatacenter(rows)
		if err != nil {
			h.dbFailure(w, r, "list_datacenters_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_datacenters") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetDatacenter(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_datacenter_id")
		return
	}
	item, found, err := h.getDatacenter(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_datacenter", err, "datacenter_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "datacenter_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleCreateDatacenter(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body DatacenterInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_datacenter_payload")
		return
	}
	if err := validateDatacenterInput(body); err != nil {
		invalidRequest(w, err.Error())
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `
		INSERT INTO datacenter
			(name, address, note, rack, stato, id_anagrafica, portale_clienti, codice_ordine, dc_build_id, ismmr, set_order, mmr_type, serialnumber, floor)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(body.Name),
		strings.TrimSpace(body.Address),
		optionalTrimmed(body.Note),
		body.RackCapacity,
		statusOrActive(body.Status),
		body.CustomerID,
		portalChar(body.PortalEnabled),
		optionalTrimmed(body.OrderCode),
		body.BuildingID,
		boolInt(body.IsMMR),
		body.SetOrder,
		optionalTrimmed(body.MMRType),
		optionalTrimmed(body.SerialNumber),
		optionalTrimmed(body.Floor),
	)
	if err != nil {
		h.dbFailure(w, r, "create_datacenter", err)
		return
	}
	id, _ := result.LastInsertId()
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Sala aggiornata."})
}

func (h *Handler) handleUpdateDatacenter(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_datacenter_id")
		return
	}
	var body DatacenterPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_datacenter_payload")
		return
	}
	sets, args, err := datacenterPatch(body)
	if err != nil {
		invalidRequest(w, err.Error())
		return
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Nessuna modifica."})
		return
	}
	args = append(args, id)
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE datacenter SET `+strings.Join(sets, ", ")+` WHERE id_datacenter = ?`, args...)
	if err != nil {
		h.dbFailure(w, r, "update_datacenter", err, "datacenter_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "datacenter_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Sala aggiornata."})
}

func (h *Handler) handleCeaseDatacenter(w http.ResponseWriter, r *http.Request) {
	h.ceaseOrDeleteDatacenter(w, r, false)
}

func (h *Handler) handleDeleteDatacenter(w http.ResponseWriter, r *http.Request) {
	h.ceaseOrDeleteDatacenter(w, r, true)
}

func (h *Handler) ceaseOrDeleteDatacenter(w http.ResponseWriter, r *http.Request, delete bool) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_datacenter_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	deps, err := h.datacenterDependencies(r, id)
	if err != nil {
		h.dbFailure(w, r, "datacenter_dependencies", err, "datacenter_id", id)
		return
	}
	if !deps.Allowed {
		httputil.JSON(w, http.StatusConflict, deps)
		return
	}
	var result sql.Result
	if delete {
		result, err = h.grappa.ExecContext(r.Context(), `DELETE FROM datacenter WHERE id_datacenter = ?`, id)
	} else {
		result, err = h.grappa.ExecContext(r.Context(), `UPDATE datacenter SET stato = 'Cessato', data_cessazione = COALESCE(data_cessazione, NOW()) WHERE id_datacenter = ?`, id)
	}
	if err != nil {
		h.dbFailure(w, r, "datacenter_lifecycle", err, "datacenter_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "datacenter_not_found")
		return
	}
	if delete {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Sala eliminata."})
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Sala cessata."})
}

func (h *Handler) getDatacenter(r *http.Request, id int) (Datacenter, bool, error) {
	row := h.grappa.QueryRowContext(r.Context(), datacenterSelectSQL()+` WHERE d.id_datacenter = ? GROUP BY `+datacenterGroupSQL(), id)
	item, err := scanDatacenter(row)
	if err == sql.ErrNoRows {
		return Datacenter{}, false, nil
	}
	return item, err == nil, err
}

func datacenterSelectSQL() string {
	return `
		SELECT d.id_datacenter, d.name, d.address, d.note, d.rack, d.stato, d.id_anagrafica,
		       d.portale_clienti, d.data_attivazione, d.data_cessazione, d.codice_ordine,
		       d.dc_build_id, db.name, d.ismmr, d.set_order, d.mmr_type, d.serialnumber, d.floor,
		       COUNT(DISTINCT i.id), COUNT(DISTINCT r.id_rack)
		FROM datacenter d
		LEFT JOIN dc_build db ON db.id = d.dc_build_id
		LEFT JOIN islets i ON i.datacenter_id = d.id_datacenter
		LEFT JOIN racks r ON r.id_datacenter = d.id_datacenter AND ` + activeStateSQL("r.stato")
}

func datacenterGroupSQL() string {
	return `d.id_datacenter, d.name, d.address, d.note, d.rack, d.stato, d.id_anagrafica,
		d.portale_clienti, d.data_attivazione, d.data_cessazione, d.codice_ordine, d.dc_build_id,
		db.name, d.ismmr, d.set_order, d.mmr_type, d.serialnumber, d.floor`
}

type datacenterScanner interface {
	Scan(dest ...any) error
}

func scanDatacenter(scanner datacenterScanner) (Datacenter, error) {
	var item Datacenter
	var note, status, portal, orderCode, buildingName, mmrType, serial, floor sql.NullString
	var customerID, buildingID, ismmr, setOrder sql.NullInt64
	var activatedAt, ceasedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.Name, &item.Address, &note, &item.RackCapacity, &status, &customerID,
		&portal, &activatedAt, &ceasedAt, &orderCode, &buildingID, &buildingName, &ismmr,
		&setOrder, &mmrType, &serial, &floor, &item.IsletCount, &item.RackCount,
	); err != nil {
		return item, err
	}
	item.Note = nullableString(note)
	item.Status = nullableString(status)
	item.CustomerID = nullableInt(customerID)
	item.PortalEnabled = strings.TrimSpace(portal.String) == "1"
	item.ActivatedAt = nullableTime(activatedAt)
	item.CeasedAt = nullableTime(ceasedAt)
	item.OrderCode = nullableString(orderCode)
	item.BuildingID = nullableInt(buildingID)
	item.BuildingName = nullableString(buildingName)
	item.IsMMR = ismmr.Valid && ismmr.Int64 == 1
	item.SetOrder = nullableInt(setOrder)
	item.MMRType = nullableString(mmrType)
	item.SerialNumber = nullableString(serial)
	item.Floor = nullableString(floor)
	return item, nil
}

func validateDatacenterInput(body DatacenterInput) error {
	if strings.TrimSpace(body.Name) == "" {
		return fmt.Errorf("datacenter_name_required")
	}
	if strings.TrimSpace(body.Address) == "" {
		return fmt.Errorf("datacenter_address_required")
	}
	if body.RackCapacity < 0 {
		return fmt.Errorf("invalid_rack_capacity")
	}
	if body.Status != nil && !allowedLifecycleStatus(*body.Status) {
		return fmt.Errorf("invalid_datacenter_status")
	}
	return nil
}

func datacenterPatch(body DatacenterPatch) ([]string, []any, error) {
	sets := []string{}
	args := []any{}
	addString := func(column string, value *string, required bool) error {
		if value == nil {
			return nil
		}
		trimmed := strings.TrimSpace(*value)
		if required && trimmed == "" {
			return fmt.Errorf("datacenter_value_required")
		}
		sets = append(sets, column+" = ?")
		if trimmed == "" {
			args = append(args, nil)
		} else {
			args = append(args, trimmed)
		}
		return nil
	}
	if err := addString("name", body.Name, true); err != nil {
		return nil, nil, err
	}
	if err := addString("address", body.Address, true); err != nil {
		return nil, nil, err
	}
	if err := addString("note", body.Note, false); err != nil {
		return nil, nil, err
	}
	if body.RackCapacity != nil {
		if *body.RackCapacity < 0 {
			return nil, nil, fmt.Errorf("invalid_rack_capacity")
		}
		sets = append(sets, "rack = ?")
		args = append(args, *body.RackCapacity)
	}
	if body.Status != nil {
		if !allowedLifecycleStatus(*body.Status) {
			return nil, nil, fmt.Errorf("invalid_datacenter_status")
		}
		sets = append(sets, "stato = ?")
		args = append(args, strings.TrimSpace(*body.Status))
	}
	if body.CustomerID != nil {
		sets = append(sets, "id_anagrafica = ?")
		args = append(args, *body.CustomerID)
	}
	if body.PortalEnabled != nil {
		sets = append(sets, "portale_clienti = ?")
		args = append(args, portalChar(*body.PortalEnabled))
	}
	if err := addString("codice_ordine", body.OrderCode, false); err != nil {
		return nil, nil, err
	}
	if body.BuildingID != nil {
		sets = append(sets, "dc_build_id = ?")
		args = append(args, *body.BuildingID)
	}
	if body.IsMMR != nil {
		sets = append(sets, "ismmr = ?")
		args = append(args, boolInt(*body.IsMMR))
	}
	if body.SetOrder != nil {
		sets = append(sets, "set_order = ?")
		args = append(args, *body.SetOrder)
	}
	if err := addString("mmr_type", body.MMRType, false); err != nil {
		return nil, nil, err
	}
	if err := addString("serialnumber", body.SerialNumber, false); err != nil {
		return nil, nil, err
	}
	if err := addString("floor", body.Floor, false); err != nil {
		return nil, nil, err
	}
	return sets, args, nil
}

func (h *Handler) datacenterDependencies(r *http.Request, id int) (DependencySummary, error) {
	checks := []dependencyCheck{
		{Key: "islets", Label: "Isole configurate", Query: `SELECT COUNT(*) FROM islets WHERE datacenter_id = ?`},
		{Key: "racks", Label: "Rack attivi", Query: `SELECT COUNT(*) FROM racks WHERE id_datacenter = ? AND ` + activeStateSQL("stato") + ` AND data_cessazione IS NULL`},
		{Key: "apparati", Label: "Apparati collegati", Query: `SELECT COUNT(*) FROM apparato a JOIN racks r ON r.id_rack = a.id_rack WHERE r.id_datacenter = ? AND ` + activeStateSQL("a.stato")},
		{Key: "servers", Label: "Server collegati", Query: `SELECT COUNT(*) FROM server s JOIN racks r ON r.id_rack = s.id_rack WHERE r.id_datacenter = ? AND ` + activeStateSQL("s.stato")},
		{Key: "optical", Label: "Cassetti ottici collegati", Query: `SELECT COUNT(*) FROM cassetti_ottici WHERE (id_datacenter = ? OR id_datacenter_coll = ?) AND ` + activeStateSQL("stato")},
		{Key: "portal", Label: "Esposizioni sul portale clienti", Query: `SELECT COUNT(*) FROM datacenter WHERE id_datacenter = ? AND portale_clienti = '1'`},
	}
	summary := DependencySummary{Allowed: true, Counts: map[string]int{}, Details: []DependencyDetail{}}
	for _, check := range checks {
		var count int
		var err error
		if check.Key == "optical" {
			err = h.grappa.QueryRowContext(r.Context(), check.Query, id, id).Scan(&count)
		} else {
			err = h.grappa.QueryRowContext(r.Context(), check.Query, id).Scan(&count)
		}
		if err != nil {
			return summary, err
		}
		summary.Counts[check.Key] = count
		if count > 0 {
			summary.Allowed = false
			summary.Details = append(summary.Details, DependencyDetail{Label: check.Label, Count: count})
		}
	}
	if !summary.Allowed {
		summary.Message = "Azione bloccata da dipendenze operative."
	}
	return summary, nil
}

func statusOrActive(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "Attivo"
	}
	return strings.TrimSpace(*value)
}

func allowedLifecycleStatus(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "Attivo" || trimmed == "Cessato"
}

func portalChar(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
