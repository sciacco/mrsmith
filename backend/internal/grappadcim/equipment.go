package grappadcim

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListEquipment(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	where := []string{"1=1"}
	args := []any{}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status == "active" || status == "" {
		where = append(where, activeStateSQL("a.stato"), "a.data_cessazione IS NULL")
	} else if status != "all" {
		where = append(where, "a.stato = ?")
		args = append(args, status)
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("rackId")); raw != "" {
		id, err := parsePositiveString(raw)
		if err != nil {
			invalidRequest(w, "invalid_rack_id")
			return
		}
		where = append(where, "a.id_rack = ?")
		args = append(args, id)
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		where = append(where, "(a.name LIKE ? OR a.serial LIKE ? OR a.serialnumber LIKE ? OR a.codice_ordine LIKE ? OR a.ip_management LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like, like, like)
	}
	rows, err := h.grappa.QueryContext(r.Context(), equipmentSelectSQL()+` WHERE `+strings.Join(where, " AND ")+` GROUP BY `+equipmentGroupSQL()+` ORDER BY a.name ASC, a.id_apparato ASC`, args...)
	if err != nil {
		h.dbFailure(w, r, "list_equipment", err)
		return
	}
	defer rows.Close()
	items := []EquipmentItem{}
	for rows.Next() {
		item, err := scanEquipment(rows)
		if err != nil {
			h.dbFailure(w, r, "list_equipment_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_equipment") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetEquipment(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_equipment_id")
		return
	}
	item, found, err := h.getEquipment(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_equipment", err, "equipment_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "equipment_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleCreateEquipment(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body EquipmentInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_equipment_payload")
		return
	}
	if err := validateEquipmentInput(body.Name, body.Type, body.PortCount); err != nil {
		invalidRequest(w, err.Error())
		return
	}
	var id int64
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(r.Context(), `
			INSERT INTO apparato
				(name, id_rack, unit_position, unit, ip_management, note, type, serial, os, model, id_anagrafica,
				 stato, banda, numero_porte, nome_porte, tipo_porte, layer_porte, data_attivazione,
				 indirizzo_installazione, indirizzo_spedizione, proprieta_cdlan, cluster_name, cliente_finale,
				 tipo_configurazione, spedizione, installazione_onsite, monitoraggio_attivo, tipologia_firewall,
				 serialnumber, codice_ordine)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			strings.TrimSpace(body.Name), body.RackID, body.UnitPosition, body.Unit, optionalTrimmed(body.ManagementIP),
			optionalTrimmed(body.Note), strings.TrimSpace(body.Type), optionalTrimmed(body.Serial), optionalTrimmed(body.OS),
			optionalTrimmed(body.Model), body.CustomerID, statusOrActive(body.Status), body.Bandwidth, body.PortCount,
			optionalTrimmed(body.PortName), optionalTrimmed(body.PortType), optionalTrimmed(body.PortLayer),
			optionalTrimmed(body.ActivatedAt), optionalTrimmed(body.InstallAddress), optionalTrimmed(body.ShippingAddress),
			optionalTrimmed(body.CdlanOwned), optionalTrimmed(body.ClusterName), optionalTrimmed(body.EndCustomer),
			optionalTrimmed(body.ConfigurationType), optionalTrimmed(body.Shipping), optionalTrimmed(body.OnsiteInstallation),
			optionalTrimmed(body.MonitoringActive), optionalTrimmed(body.FirewallType), optionalTrimmed(body.SerialNumber),
			optionalTrimmed(body.OrderCode),
		)
		if err != nil {
			return err
		}
		id, _ = result.LastInsertId()
		return createEquipmentNICTx(r, tx, int(id), body)
	}); err != nil {
		h.dbFailure(w, r, "create_equipment", err)
		return
	}
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Apparato creato."})
}

func (h *Handler) handleUpdateEquipment(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_equipment_id")
		return
	}
	var body EquipmentPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_equipment_payload")
		return
	}
	sets, args, err := equipmentPatch(body)
	if err != nil {
		invalidRequest(w, err.Error())
		return
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Nessuna modifica."})
		return
	}
	args = append(args, id)
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE apparato SET `+strings.Join(sets, ", ")+` WHERE id_apparato = ?`, args...)
	if err != nil {
		h.dbFailure(w, r, "update_equipment", err, "equipment_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "equipment_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Apparato aggiornato."})
}

func (h *Handler) handleCeaseEquipment(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_equipment_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(r.Context(), `UPDATE apparato SET stato = 'Cessato', data_cessazione = COALESCE(data_cessazione, NOW()) WHERE id_apparato = ?`, id)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return sql.ErrNoRows
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE nic SET stato = 'Cessato' WHERE id_apparato = ?`, id)
		return err
	}); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "equipment_not_found")
			return
		}
		h.dbFailure(w, r, "cease_equipment", err, "equipment_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Apparato cessato."})
}

func (h *Handler) handleListEquipmentNICs(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_equipment_id")
		return
	}
	rows, err := h.grappa.QueryContext(r.Context(), `
		SELECT id_nic, id_apparato, identificativo, name, id_anagrafica, note, type, link_id_apparato,
		       link_id_nic, layer, link_id_server, stato
		FROM nic
		WHERE id_apparato = ?
		ORDER BY id_nic ASC`, id)
	if err != nil {
		h.dbFailure(w, r, "list_equipment_nics", err, "equipment_id", id)
		return
	}
	defer rows.Close()
	items := []NICItem{}
	for rows.Next() {
		var item NICItem
		var equipmentID, customerID, linkedEquipmentID, linkedNICID, linkedServerID sql.NullInt64
		var note, nicType, layer, status sql.NullString
		if err := rows.Scan(&item.ID, &equipmentID, &item.Identifier, &item.Name, &customerID, &note, &nicType, &linkedEquipmentID, &linkedNICID, &layer, &linkedServerID, &status); err != nil {
			h.dbFailure(w, r, "list_equipment_nics_scan", err, "equipment_id", id)
			return
		}
		item.EquipmentID = nullableInt(equipmentID)
		item.CustomerID = nullableInt(customerID)
		item.Note = nullableString(note)
		item.Type = nullableString(nicType)
		item.LinkedEquipmentID = nullableInt(linkedEquipmentID)
		item.LinkedNICID = nullableInt(linkedNICID)
		item.Layer = nullableString(layer)
		item.LinkedServerID = nullableInt(linkedServerID)
		item.Status = nullableString(status)
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_equipment_nics") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleEquipmentTypeOptions(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rows, err := h.grappa.QueryContext(r.Context(), `SELECT DISTINCT TRIM(type) FROM apparato WHERE TRIM(type) <> '' ORDER BY TRIM(type) ASC`)
	if err != nil {
		h.dbFailure(w, r, "equipment_type_options", err)
		return
	}
	defer rows.Close()
	items := []LookupItem{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			h.dbFailure(w, r, "equipment_type_options_scan", err)
			return
		}
		items = append(items, LookupItem{ID: value, Label: value})
	}
	if !h.rowsDone(w, r, rows, "equipment_type_options") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) getEquipment(r *http.Request, id int) (EquipmentItem, bool, error) {
	row := h.grappa.QueryRowContext(r.Context(), equipmentSelectSQL()+` WHERE a.id_apparato = ? GROUP BY `+equipmentGroupSQL(), id)
	item, err := scanEquipment(row)
	if err == sql.ErrNoRows {
		return EquipmentItem{}, false, nil
	}
	return item, err == nil, err
}

func equipmentSelectSQL() string {
	return `
		SELECT a.id_apparato, a.name, a.id_rack, r.name, d.name, a.unit_position, a.unit, a.ip_management,
		       a.note, a.type, a.serial, a.os, a.model, a.id_anagrafica, a.stato, a.banda, a.numero_porte,
		       a.nome_porte, a.tipo_porte, a.layer_porte, a.data_attivazione, a.data_cessazione,
		       a.indirizzo_installazione, a.indirizzo_spedizione, a.proprieta_cdlan, a.cluster_name,
		       a.cliente_finale, a.tipo_configurazione, a.spedizione, a.installazione_onsite,
		       a.monitoraggio_attivo, a.tipologia_firewall, a.serialnumber, a.codice_ordine,
		       a.ultima_notifica, COUNT(DISTINCT n.id_nic)
		FROM apparato a
		LEFT JOIN racks r ON r.id_rack = a.id_rack
		LEFT JOIN datacenter d ON d.id_datacenter = r.id_datacenter
		LEFT JOIN nic n ON n.id_apparato = a.id_apparato`
}

func equipmentGroupSQL() string {
	return `a.id_apparato, a.name, a.id_rack, r.name, d.name, a.unit_position, a.unit, a.ip_management,
		a.note, a.type, a.serial, a.os, a.model, a.id_anagrafica, a.stato, a.banda, a.numero_porte,
		a.nome_porte, a.tipo_porte, a.layer_porte, a.data_attivazione, a.data_cessazione,
		a.indirizzo_installazione, a.indirizzo_spedizione, a.proprieta_cdlan, a.cluster_name,
		a.cliente_finale, a.tipo_configurazione, a.spedizione, a.installazione_onsite,
		a.monitoraggio_attivo, a.tipologia_firewall, a.serialnumber, a.codice_ordine, a.ultima_notifica`
}

type equipmentScanner interface {
	Scan(dest ...any) error
}

func scanEquipment(scanner equipmentScanner) (EquipmentItem, error) {
	var item EquipmentItem
	var rackID, unitPosition, unit, customerID, bandwidth, portCount sql.NullInt64
	var rackName, datacenterName, managementIP, note, serial, os, model, status, portName, portType, portLayer sql.NullString
	var installAddress, shippingAddress, cdlanOwned, clusterName, endCustomer, configurationType, shipping, onsiteInstallation sql.NullString
	var monitoringActive, firewallType, serialNumber, orderCode sql.NullString
	var activatedAt, ceasedAt, lastNotificationAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.Name, &rackID, &rackName, &datacenterName, &unitPosition, &unit, &managementIP,
		&note, &item.Type, &serial, &os, &model, &customerID, &status, &bandwidth, &portCount, &portName,
		&portType, &portLayer, &activatedAt, &ceasedAt, &installAddress, &shippingAddress, &cdlanOwned,
		&clusterName, &endCustomer, &configurationType, &shipping, &onsiteInstallation, &monitoringActive,
		&firewallType, &serialNumber, &orderCode, &lastNotificationAt, &item.NICCount,
	); err != nil {
		return item, err
	}
	item.RackID = nullableInt(rackID)
	item.RackName = nullableString(rackName)
	item.DatacenterName = nullableString(datacenterName)
	item.UnitPosition = nullableInt(unitPosition)
	item.Unit = nullableInt(unit)
	item.ManagementIP = nullableString(managementIP)
	item.Note = nullableString(note)
	item.Serial = nullableString(serial)
	item.OS = nullableString(os)
	item.Model = nullableString(model)
	item.CustomerID = nullableInt(customerID)
	item.Status = nullableString(status)
	item.Bandwidth = nullableInt(bandwidth)
	item.PortCount = nullableInt(portCount)
	item.PortName = nullableString(portName)
	item.PortType = nullableString(portType)
	item.PortLayer = nullableString(portLayer)
	item.ActivatedAt = nullableDate(activatedAt)
	item.CeasedAt = nullableDate(ceasedAt)
	item.InstallAddress = nullableString(installAddress)
	item.ShippingAddress = nullableString(shippingAddress)
	item.CdlanOwned = nullableString(cdlanOwned)
	item.ClusterName = nullableString(clusterName)
	item.EndCustomer = nullableString(endCustomer)
	item.ConfigurationType = nullableString(configurationType)
	item.Shipping = nullableString(shipping)
	item.OnsiteInstallation = nullableString(onsiteInstallation)
	item.MonitoringActive = nullableString(monitoringActive)
	item.FirewallType = nullableString(firewallType)
	item.SerialNumber = nullableString(serialNumber)
	item.OrderCode = nullableString(orderCode)
	item.LastNotificationAt = nullableTime(lastNotificationAt)
	return item, nil
}

func validateEquipmentInput(name string, equipmentType string, portCount *int) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("equipment_name_required")
	}
	if strings.TrimSpace(equipmentType) == "" {
		return fmt.Errorf("equipment_type_required")
	}
	if portCount != nil && (*portCount < 0 || *portCount > 512) {
		return fmt.Errorf("invalid_equipment_port_count")
	}
	return nil
}

func createEquipmentNICTx(r *http.Request, tx *sql.Tx, equipmentID int, body EquipmentInput) error {
	if body.PortCount == nil || *body.PortCount <= 0 {
		return nil
	}
	portBase := strings.TrimSpace(optionalStringValue(body.PortName))
	if portBase == "" {
		portBase = "Porta"
	}
	for i := 1; i <= *body.PortCount; i++ {
		label := fmt.Sprintf("%s %d", portBase, i)
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO nic (id_apparato, identificativo, name, id_anagrafica, type, layer, stato)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			equipmentID, label, label, body.CustomerID, optionalTrimmed(body.PortType), optionalTrimmed(body.PortLayer), statusOrActive(body.Status),
		); err != nil {
			return err
		}
	}
	return nil
}

func equipmentPatch(body EquipmentPatch) ([]string, []any, error) {
	sets := []string{}
	args := []any{}
	if body.Name != nil {
		if strings.TrimSpace(*body.Name) == "" {
			return nil, nil, fmt.Errorf("equipment_name_required")
		}
		sets = append(sets, "name = ?")
		args = append(args, strings.TrimSpace(*body.Name))
	}
	if body.Type != nil {
		if strings.TrimSpace(*body.Type) == "" {
			return nil, nil, fmt.Errorf("equipment_type_required")
		}
		sets = append(sets, "type = ?")
		args = append(args, strings.TrimSpace(*body.Type))
	}
	if body.Status != nil {
		sets = append(sets, "stato = ?")
		args = append(args, strings.TrimSpace(*body.Status))
	}
	intFields := []struct {
		column string
		value  *int
	}{
		{"id_rack", body.RackID},
		{"unit_position", body.UnitPosition},
		{"unit", body.Unit},
		{"id_anagrafica", body.CustomerID},
		{"banda", body.Bandwidth},
		{"numero_porte", body.PortCount},
	}
	for _, field := range intFields {
		if field.value != nil {
			sets = append(sets, field.column+" = ?")
			args = append(args, *field.value)
		}
	}
	stringFields := []struct {
		column string
		value  *string
	}{
		{"ip_management", body.ManagementIP},
		{"note", body.Note},
		{"serial", body.Serial},
		{"os", body.OS},
		{"model", body.Model},
		{"nome_porte", body.PortName},
		{"tipo_porte", body.PortType},
		{"layer_porte", body.PortLayer},
		{"data_attivazione", body.ActivatedAt},
		{"indirizzo_installazione", body.InstallAddress},
		{"indirizzo_spedizione", body.ShippingAddress},
		{"proprieta_cdlan", body.CdlanOwned},
		{"cluster_name", body.ClusterName},
		{"cliente_finale", body.EndCustomer},
		{"tipo_configurazione", body.ConfigurationType},
		{"spedizione", body.Shipping},
		{"installazione_onsite", body.OnsiteInstallation},
		{"monitoraggio_attivo", body.MonitoringActive},
		{"tipologia_firewall", body.FirewallType},
		{"serialnumber", body.SerialNumber},
		{"codice_ordine", body.OrderCode},
	}
	for _, field := range stringFields {
		if field.value != nil {
			sets = append(sets, field.column+" = ?")
			args = append(args, optionalTrimmed(field.value))
		}
	}
	return sets, args, nil
}
