package grappadcim

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListServers(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	where := []string{"1=1"}
	args := []any{}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status == "active" || status == "" {
		where = append(where, activeStateSQL("s.stato"), "s.data_cessazione IS NULL")
	} else if status != "all" {
		where = append(where, "s.stato = ?")
		args = append(args, status)
	}
	if kind := strings.TrimSpace(r.URL.Query().Get("kind")); kind != "" && kind != "all" {
		where = append(where, "s.tipologia = ?")
		args = append(args, kind)
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		where = append(where, "(s.name LIKE ? OR s.hostname LIKE ? OR s.seriale LIKE ? OR s.serialnumber LIKE ? OR s.codice_ordine LIKE ? OR s.ip_mngt LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like, like, like, like)
	}
	rows, err := h.grappa.QueryContext(r.Context(), serverSelectSQL()+` WHERE `+strings.Join(where, " AND ")+` ORDER BY s.name ASC, s.id_server ASC`, args...)
	if err != nil {
		h.dbFailure(w, r, "list_servers", err)
		return
	}
	defer rows.Close()
	items := []ServerItem{}
	for rows.Next() {
		item, err := scanServer(rows)
		if err != nil {
			h.dbFailure(w, r, "list_servers_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_servers") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetServer(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_server_id")
		return
	}
	item, found, err := h.getServer(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_server", err, "server_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "server_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body ServerInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_server_payload")
		return
	}
	if err := validateServerInput(body.Kind); err != nil {
		invalidRequest(w, err.Error())
		return
	}
	var id int64
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(r.Context(), `
			INSERT INTO server
				(tipologia, name, id_anagrafica, contatto, stato, sistema_operativo, architettura, hostname,
				 id_rack, unit, unit_position, slot, tipologia_virtualizzazione, cluster_virtualizzazione,
				 modello, seriale, n_socket_cpu, n_cpu, n_core, totale_ram, banchi_ram, dischi, livello_raid,
				 hotspare, ilo_idrac, gestione_patching, accesso_root_administrator_cliente, utenza_cliente,
				 utenza_cdlan, server_syslog, servizi_sotto_syslog, hostname_backup, tipo_backup, server_cdp_nas,
				 schedulazione_backup, quota_backup_cdp_gb, data_attivazione, note, ip_mngt, note_backup,
				 note_gestione, apparato_id, codice_ordine, serialnumber, porte)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			strings.TrimSpace(body.Kind), optionalTrimmed(body.Name), body.CustomerID, optionalTrimmed(body.Contact),
			statusOrActive(body.Status), optionalTrimmed(body.OperatingSystem), optionalTrimmed(body.Architecture),
			optionalTrimmed(body.Hostname), body.RackID, body.Unit, body.UnitPosition, optionalTrimmed(body.Slot),
			optionalTrimmed(body.VirtualizationType), optionalTrimmed(body.VirtualizationCluster), optionalTrimmed(body.Model),
			optionalTrimmed(body.Serial), body.CPUSockets, optionalTrimmed(body.CPU), body.CoreCount, body.RAM,
			optionalTrimmed(body.RAMBanks), optionalTrimmed(body.Disks), optionalTrimmed(body.RaidLevel), body.Hotspare,
			optionalTrimmed(body.IloAddress), optionalTrimmed(body.PatchingManagement), optionalTrimmed(body.CustomerRootAccess),
			optionalTrimmed(body.CustomerUsername), optionalTrimmed(body.CdlanUsername), optionalTrimmed(body.SyslogServer),
			optionalTrimmed(body.SyslogServices), optionalTrimmed(body.BackupHostname), optionalTrimmed(body.BackupType),
			optionalTrimmed(body.BackupNasServer), optionalTrimmed(body.BackupSchedule), body.BackupQuotaGB,
			optionalTrimmed(body.ActivatedAt), optionalTrimmed(body.Note), optionalTrimmed(body.ManagementIP),
			optionalTrimmed(body.BackupNote), optionalTrimmed(body.ManagementNote), body.EquipmentID,
			optionalTrimmed(body.OrderCode), optionalTrimmed(body.SerialNumber), body.PortCount,
		)
		if err != nil {
			return err
		}
		id, _ = result.LastInsertId()
		return syncPhysicalServerEquipmentTx(r, tx, &body.Kind, body.EquipmentID, body.CustomerID, body.OrderCode, body.SerialNumber)
	}); err != nil {
		h.dbFailure(w, r, "create_server", err)
		return
	}
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Server creato."})
}

func (h *Handler) handleUpdateServer(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_server_id")
		return
	}
	var body ServerPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_server_payload")
		return
	}
	sets, args, err := serverPatch(body)
	if err != nil {
		invalidRequest(w, err.Error())
		return
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Nessuna modifica."})
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		updateArgs := append([]any{}, args...)
		updateArgs = append(updateArgs, id)
		result, err := tx.ExecContext(r.Context(), `UPDATE server SET `+strings.Join(sets, ", ")+` WHERE id_server = ?`, updateArgs...)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return sql.ErrNoRows
		}
		if serverPatchTouchesEquipmentSync(body) {
			return syncEffectivePhysicalServerEquipmentTx(r, tx, id)
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "server_not_found")
			return
		}
		h.dbFailure(w, r, "update_server", err, "server_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Server aggiornato."})
}

func (h *Handler) handleGetServerChildren(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_server_id")
		return
	}
	children, err := h.getServerChildren(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_server_children", err, "server_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, children)
}

func (h *Handler) getServer(r *http.Request, id int) (ServerItem, bool, error) {
	row := h.grappa.QueryRowContext(r.Context(), serverSelectSQL()+` WHERE s.id_server = ?`, id)
	item, err := scanServer(row)
	if err == sql.ErrNoRows {
		return ServerItem{}, false, nil
	}
	return item, err == nil, err
}

func serverSelectSQL() string {
	return `
		SELECT s.id_server, s.tipologia, s.name, s.id_anagrafica, s.contatto, s.stato, s.sistema_operativo,
		       s.architettura, s.hostname, s.id_rack, r.name, s.unit, s.unit_position, s.slot,
		       s.tipologia_virtualizzazione, s.cluster_virtualizzazione, s.modello, s.seriale,
		       s.n_socket_cpu, s.n_cpu, s.n_core, s.totale_ram, s.banchi_ram, s.dischi, s.livello_raid,
		       s.hotspare, s.ilo_idrac, s.gestione_patching, s.accesso_root_administrator_cliente,
		       s.utenza_cliente, s.utenza_cdlan, s.server_syslog, s.servizi_sotto_syslog,
		       s.hostname_backup, s.tipo_backup, s.server_cdp_nas, s.schedulazione_backup,
		       s.quota_backup_cdp_gb, s.data_attivazione, s.data_cessazione, s.note, s.ip_mngt,
		       s.note_backup, s.note_gestione, s.apparato_id, a.name, s.codice_ordine, s.serialnumber, s.porte
		FROM server s
		LEFT JOIN racks r ON r.id_rack = s.id_rack
		LEFT JOIN apparato a ON a.id_apparato = s.apparato_id`
}

type serverScanner interface {
	Scan(dest ...any) error
}

func scanServer(scanner serverScanner) (ServerItem, error) {
	var item ServerItem
	var name, contact, status, os, architecture, hostname, rackName, slot, virtualizationType, virtualizationCluster sql.NullString
	var model, serial, cpu, ramBanks, disks, raidLevel, iloAddress, patchingManagement, customerRootAccess sql.NullString
	var customerUsername, cdlanUsername, syslogServer, syslogServices, backupHostname, backupType, backupNasServer sql.NullString
	var backupSchedule, note, managementIP, backupNote, managementNote, equipmentName, orderCode, serialNumber sql.NullString
	var customerID, rackID, unit, unitPosition, cpuSockets, coreCount, ram, hotspare, backupQuota, equipmentID, portCount sql.NullInt64
	var activatedAt, ceasedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.Kind, &name, &customerID, &contact, &status, &os, &architecture, &hostname,
		&rackID, &rackName, &unit, &unitPosition, &slot, &virtualizationType, &virtualizationCluster,
		&model, &serial, &cpuSockets, &cpu, &coreCount, &ram, &ramBanks, &disks, &raidLevel, &hotspare,
		&iloAddress, &patchingManagement, &customerRootAccess, &customerUsername, &cdlanUsername,
		&syslogServer, &syslogServices, &backupHostname, &backupType, &backupNasServer, &backupSchedule,
		&backupQuota, &activatedAt, &ceasedAt, &note, &managementIP, &backupNote, &managementNote,
		&equipmentID, &equipmentName, &orderCode, &serialNumber, &portCount,
	); err != nil {
		return item, err
	}
	item.Name = nullableString(name)
	item.CustomerID = nullableInt(customerID)
	item.Contact = nullableString(contact)
	item.Status = nullableString(status)
	item.OperatingSystem = nullableString(os)
	item.Architecture = nullableString(architecture)
	item.Hostname = nullableString(hostname)
	item.RackID = nullableInt(rackID)
	item.RackName = nullableString(rackName)
	item.Unit = nullableInt(unit)
	item.UnitPosition = nullableInt(unitPosition)
	item.Slot = nullableString(slot)
	item.VirtualizationType = nullableString(virtualizationType)
	item.VirtualizationCluster = nullableString(virtualizationCluster)
	item.Model = nullableString(model)
	item.Serial = nullableString(serial)
	item.CPUSockets = nullableInt(cpuSockets)
	item.CPU = nullableString(cpu)
	item.CoreCount = nullableInt(coreCount)
	item.RAM = nullableInt(ram)
	item.RAMBanks = nullableString(ramBanks)
	item.Disks = nullableString(disks)
	item.RaidLevel = nullableString(raidLevel)
	item.Hotspare = nullableInt(hotspare)
	item.IloAddress = nullableString(iloAddress)
	item.PatchingManagement = nullableString(patchingManagement)
	item.CustomerRootAccess = nullableString(customerRootAccess)
	item.CustomerUsername = nullableString(customerUsername)
	item.CdlanUsername = nullableString(cdlanUsername)
	item.SyslogServer = nullableString(syslogServer)
	item.SyslogServices = nullableString(syslogServices)
	item.BackupHostname = nullableString(backupHostname)
	item.BackupType = nullableString(backupType)
	item.BackupNasServer = nullableString(backupNasServer)
	item.BackupSchedule = nullableString(backupSchedule)
	item.BackupQuotaGB = nullableInt(backupQuota)
	item.ActivatedAt = nullableDate(activatedAt)
	item.CeasedAt = nullableDate(ceasedAt)
	item.Note = nullableString(note)
	item.ManagementIP = nullableString(managementIP)
	item.BackupNote = nullableString(backupNote)
	item.ManagementNote = nullableString(managementNote)
	item.EquipmentID = nullableInt(equipmentID)
	item.EquipmentName = nullableString(equipmentName)
	item.OrderCode = nullableString(orderCode)
	item.SerialNumber = nullableString(serialNumber)
	item.PortCount = nullableInt(portCount)
	return item, nil
}

func validateServerInput(kind string) error {
	if strings.TrimSpace(kind) == "" {
		return fmt.Errorf("server_kind_required")
	}
	return nil
}

func serverPatch(body ServerPatch) ([]string, []any, error) {
	sets := []string{}
	args := []any{}
	stringFields := []struct {
		column string
		value  *string
		trim   bool
	}{
		{"tipologia", body.Kind, true},
		{"name", body.Name, false},
		{"contatto", body.Contact, false},
		{"stato", body.Status, false},
		{"sistema_operativo", body.OperatingSystem, false},
		{"architettura", body.Architecture, false},
		{"hostname", body.Hostname, false},
		{"slot", body.Slot, false},
		{"tipologia_virtualizzazione", body.VirtualizationType, false},
		{"cluster_virtualizzazione", body.VirtualizationCluster, false},
		{"modello", body.Model, false},
		{"seriale", body.Serial, false},
		{"n_cpu", body.CPU, false},
		{"banchi_ram", body.RAMBanks, false},
		{"dischi", body.Disks, false},
		{"livello_raid", body.RaidLevel, false},
		{"ilo_idrac", body.IloAddress, false},
		{"gestione_patching", body.PatchingManagement, false},
		{"accesso_root_administrator_cliente", body.CustomerRootAccess, false},
		{"utenza_cliente", body.CustomerUsername, false},
		{"utenza_cdlan", body.CdlanUsername, false},
		{"server_syslog", body.SyslogServer, false},
		{"servizi_sotto_syslog", body.SyslogServices, false},
		{"hostname_backup", body.BackupHostname, false},
		{"tipo_backup", body.BackupType, false},
		{"server_cdp_nas", body.BackupNasServer, false},
		{"schedulazione_backup", body.BackupSchedule, false},
		{"data_attivazione", body.ActivatedAt, false},
		{"note", body.Note, false},
		{"ip_mngt", body.ManagementIP, false},
		{"note_backup", body.BackupNote, false},
		{"note_gestione", body.ManagementNote, false},
		{"codice_ordine", body.OrderCode, false},
		{"serialnumber", body.SerialNumber, false},
	}
	for _, field := range stringFields {
		if field.value != nil {
			if field.trim && strings.TrimSpace(*field.value) == "" {
				return nil, nil, fmt.Errorf("server_kind_required")
			}
			sets = append(sets, field.column+" = ?")
			args = append(args, optionalTrimmed(field.value))
		}
	}
	intFields := []struct {
		column string
		value  *int
	}{
		{"id_anagrafica", body.CustomerID},
		{"id_rack", body.RackID},
		{"unit", body.Unit},
		{"unit_position", body.UnitPosition},
		{"n_socket_cpu", body.CPUSockets},
		{"n_core", body.CoreCount},
		{"totale_ram", body.RAM},
		{"hotspare", body.Hotspare},
		{"quota_backup_cdp_gb", body.BackupQuotaGB},
		{"apparato_id", body.EquipmentID},
		{"porte", body.PortCount},
	}
	for _, field := range intFields {
		if field.value != nil {
			sets = append(sets, field.column+" = ?")
			args = append(args, *field.value)
		}
	}
	return sets, args, nil
}

func syncPhysicalServerEquipmentTx(r *http.Request, tx *sql.Tx, kind *string, equipmentID *int, customerID *int, orderCode *string, serialNumber *string) error {
	if kind == nil || strings.ToLower(strings.TrimSpace(*kind)) != "fisico" || equipmentID == nil || *equipmentID <= 0 {
		return nil
	}
	_, err := tx.ExecContext(r.Context(), `
		UPDATE apparato
		SET id_anagrafica = ?, codice_ordine = ?, serialnumber = ?
		WHERE id_apparato = ?`,
		customerID, optionalTrimmed(orderCode), optionalTrimmed(serialNumber), *equipmentID,
	)
	return err
}

func serverPatchTouchesEquipmentSync(body ServerPatch) bool {
	return body.Kind != nil || body.EquipmentID != nil || body.CustomerID != nil || body.OrderCode != nil || body.SerialNumber != nil
}

func syncEffectivePhysicalServerEquipmentTx(r *http.Request, tx *sql.Tx, serverID int) error {
	var kind, orderCode, serialNumber sql.NullString
	var equipmentID, customerID sql.NullInt64
	if err := tx.QueryRowContext(r.Context(), `
		SELECT tipologia, apparato_id, id_anagrafica, codice_ordine, serialnumber
		FROM server
		WHERE id_server = ?`, serverID).Scan(&kind, &equipmentID, &customerID, &orderCode, &serialNumber); err != nil {
		return err
	}
	return syncPhysicalServerEquipmentTx(
		r,
		tx,
		nullableString(kind),
		nullableInt(equipmentID),
		nullableInt(customerID),
		nullableString(orderCode),
		nullableString(serialNumber),
	)
}

func (h *Handler) getServerChildren(r *http.Request, id int) (ServerChildren, error) {
	children := ServerChildren{
		Cards:        []ServerCard{},
		Applications: []ServerApplication{},
		Services:     []ServerService{},
		Ports:        []ServerPort{},
	}
	cardRows, err := h.grappa.QueryContext(r.Context(), `SELECT id_server_scheda, nome_fisico, nome_os, ip, id_subnetmask, note FROM server_schede WHERE id_server = ? ORDER BY id_server_scheda ASC`, id)
	if err != nil {
		return children, err
	}
	defer cardRows.Close()
	for cardRows.Next() {
		var item ServerCard
		var physical, os, ip, note sql.NullString
		var subnet sql.NullInt64
		if err := cardRows.Scan(&item.ID, &physical, &os, &ip, &subnet, &note); err != nil {
			return children, err
		}
		item.PhysicalName = nullableString(physical)
		item.OSName = nullableString(os)
		item.IP = nullableString(ip)
		item.SubnetmaskID = nullableInt(subnet)
		item.Note = nullableString(note)
		children.Cards = append(children.Cards, item)
	}
	if err := cardRows.Err(); err != nil {
		return children, err
	}
	appRows, err := h.grappa.QueryContext(r.Context(), `SELECT id_server_applicazione, name, gestito_da_cdlan FROM server_applicazioni WHERE id_server = ? ORDER BY id_server_applicazione ASC`, id)
	if err != nil {
		return children, err
	}
	defer appRows.Close()
	for appRows.Next() {
		var item ServerApplication
		var name, managed sql.NullString
		if err := appRows.Scan(&item.ID, &name, &managed); err != nil {
			return children, err
		}
		item.Name = nullableString(name)
		item.ManagedByCdlan = nullableString(managed)
		children.Applications = append(children.Applications, item)
	}
	if err := appRows.Err(); err != nil {
		return children, err
	}
	serviceRows, err := h.grappa.QueryContext(r.Context(), `SELECT id_server_servizio, name FROM server_servizi WHERE id_server = ? ORDER BY id_server_servizio ASC`, id)
	if err != nil {
		return children, err
	}
	defer serviceRows.Close()
	for serviceRows.Next() {
		var item ServerService
		var name sql.NullString
		if err := serviceRows.Scan(&item.ID, &name); err != nil {
			return children, err
		}
		item.Name = nullableString(name)
		children.Services = append(children.Services, item)
	}
	if err := serviceRows.Err(); err != nil {
		return children, err
	}
	portRows, err := h.grappa.QueryContext(r.Context(), `SELECT id, interface_name, destination_interface, port_type FROM server_porte WHERE id_server = ? ORDER BY id ASC`, id)
	if err != nil {
		return children, err
	}
	defer portRows.Close()
	for portRows.Next() {
		var item ServerPort
		var name, destination, portType sql.NullString
		if err := portRows.Scan(&item.ID, &name, &destination, &portType); err != nil {
			return children, err
		}
		item.InterfaceName = nullableString(name)
		item.DestinationInterface = nullableString(destination)
		item.PortType = nullableString(portType)
		children.Ports = append(children.Ports, item)
	}
	return children, portRows.Err()
}
