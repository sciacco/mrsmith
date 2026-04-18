package energiadc

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListNoVariableCustomers(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	query := `
		SELECT DISTINCT c.id, c.intestazione
		FROM racks r
		JOIN cli_fatturazione c ON c.id = r.id_anagrafica
		JOIN datacenter d ON d.id_datacenter = r.id_datacenter
		JOIN dc_build db ON db.id = d.dc_build_id
		WHERE r.stato = 'attivo'
		  AND (r.variable_billing IS NULL OR r.variable_billing = 0)`
	args := make([]any, 0, len(h.config.ExcludedCustomerIDs))
	if len(h.config.ExcludedCustomerIDs) > 0 {
		query += ` AND r.id_anagrafica NOT IN (` + placeholders(len(h.config.ExcludedCustomerIDs)) + `)`
		for _, customerID := range h.config.ExcludedCustomerIDs {
			args = append(args, customerID)
		}
	}
	query += ` ORDER BY c.intestazione ASC, c.id ASC`

	rows, err := h.grappaDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_no_variable_customers", err)
		return
	}
	defer rows.Close()

	result := make([]lookupItem, 0)
	for rows.Next() {
		var item lookupItem
		var name sql.NullString
		if err := rows.Scan(&item.ID, &name); err != nil {
			h.dbFailure(w, r, "list_no_variable_customers_scan", err)
			return
		}
		item.Name = cleanString(name)
		result = append(result, item)
	}
	if !h.rowsDone(w, r, rows, "list_no_variable_customers") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

func (h *Handler) handleListNoVariableRacks(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	customerID, ok := h.parsePathInt(w, r, "customerId", "invalid_customer_id")
	if !ok {
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(), `
		SELECT
			r.id_rack,
			r.name,
			db.name,
			d.name,
			r.floor,
			r.island,
			r.type,
			r.pos,
			r.codice_ordine,
			r.serialnumber,
			r.variable_billing
		FROM racks r
		JOIN datacenter d ON d.id_datacenter = r.id_datacenter
		JOIN dc_build db ON db.id = d.dc_build_id
		WHERE r.id_anagrafica = ?
		  AND r.stato = 'attivo'
		  AND (r.variable_billing IS NULL OR r.variable_billing = 0)
		ORDER BY db.name ASC, d.name ASC, r.name ASC, r.id_rack ASC`, customerID)
	if err != nil {
		h.dbFailure(w, r, "list_no_variable_racks", err, "customer_id", customerID)
		return
	}
	defer rows.Close()

	result := make([]noVariableRackResponse, 0)
	for rows.Next() {
		var item noVariableRackResponse
		var buildingName sql.NullString
		var roomName sql.NullString
		var floor sql.NullInt64
		var island sql.NullInt64
		var rackType sql.NullString
		var position sql.NullString
		var orderCode sql.NullString
		var serialNumber sql.NullString
		var variableBilling sql.NullInt64
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&buildingName,
			&roomName,
			&floor,
			&island,
			&rackType,
			&position,
			&orderCode,
			&serialNumber,
			&variableBilling,
		); err != nil {
			h.dbFailure(w, r, "list_no_variable_racks_scan", err, "customer_id", customerID)
			return
		}

		item.BuildingName = cleanString(buildingName)
		item.RoomName = cleanString(roomName)
		item.Floor = nullableInt(floor)
		item.Island = nullableInt(island)
		item.RackType = cleanString(rackType)
		item.Position = cleanString(position)
		item.OrderCode = cleanString(orderCode)
		item.SerialNumber = cleanString(serialNumber)
		item.VariableBilling = variableBilling.Valid && variableBilling.Int64 > 0
		result = append(result, item)
	}
	if !h.rowsDone(w, r, rows, "list_no_variable_racks") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

func (h *Handler) handleListLowConsumption(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	minAmpere, ok := h.parseRequiredQueryFloat64(w, r, "min", "invalid_min_parameter")
	if !ok {
		return
	}
	customerID, ok := h.parseOptionalQueryInt(w, r, "customerId", "invalid_customer_id")
	if !ok {
		return
	}

	since := time.Now().In(h.config.Location).Add(-48 * time.Hour).Format(sqlDateTimeLayout)
	query := `
		SELECT
			cf.id,
			cf.intestazione,
			db.name,
			d.name,
			r.name,
			rs.id,
			COALESCE(ROUND(AVG(rpr.ampere), 2), 0) AS avg_ampere,
			rs.snmp_monitoring_device,
			rs.magnetotermico,
			rs.posizione,
			rs.posizione2,
			rs.posizione3,
			rs.posizione4
		FROM rack_sockets rs
		JOIN racks r ON r.id_rack = rs.rack_id
		JOIN cli_fatturazione cf ON cf.id = r.id_anagrafica
		JOIN datacenter d ON d.id_datacenter = r.id_datacenter
		JOIN dc_build db ON db.id = d.dc_build_id
		LEFT JOIN rack_power_readings rpr
			ON rpr.rack_socket_id = rs.id
		   AND rpr.date >= ?
		WHERE r.stato = 'attivo'`
	args := []any{since}
	if customerID != nil {
		query += ` AND r.id_anagrafica = ?`
		args = append(args, *customerID)
	}
	query += `
		GROUP BY
			cf.id,
			cf.intestazione,
			db.name,
			d.name,
			r.name,
			rs.id,
			rs.snmp_monitoring_device,
			rs.magnetotermico,
			rs.posizione,
			rs.posizione2,
			rs.posizione3,
			rs.posizione4
		HAVING COALESCE(AVG(rpr.ampere), 0) <= ?
		ORDER BY cf.intestazione ASC, db.name ASC, d.name ASC, r.name ASC, rs.id ASC`
	args = append(args, minAmpere)

	rows, err := h.grappaDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_low_consumption", err)
		return
	}
	defer rows.Close()

	result := make([]lowConsumptionRowResponse, 0)
	for rows.Next() {
		var item lowConsumptionRowResponse
		var powerMeter sql.NullString
		var breaker sql.NullString
		var posizione sql.NullString
		var posizione2 sql.NullString
		var posizione3 sql.NullString
		var posizione4 sql.NullString
		if err := rows.Scan(
			&item.CustomerID,
			&item.CustomerName,
			&item.BuildingName,
			&item.RoomName,
			&item.RackName,
			&item.SocketID,
			&item.Ampere,
			&powerMeter,
			&breaker,
			&posizione,
			&posizione2,
			&posizione3,
			&posizione4,
		); err != nil {
			h.dbFailure(w, r, "list_low_consumption_scan", err)
			return
		}

		item.PowerMeter = cleanString(powerMeter)
		item.Breaker = cleanString(breaker)
		item.Positions = composePositions(
			cleanString(posizione),
			cleanString(posizione2),
			cleanString(posizione3),
			cleanString(posizione4),
		)
		item.SocketLabel = socketLabel(item.SocketID, item.Positions)
		result = append(result, item)
	}
	if !h.rowsDone(w, r, rows, "list_low_consumption") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}
