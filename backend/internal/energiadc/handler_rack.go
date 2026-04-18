package energiadc

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const (
	defaultPowerReadingsPage = 1
	defaultPowerReadingsSize = 20
	maxPowerReadingsSize     = 200
)

func (h *Handler) handleGetRackDetail(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rackID, ok := h.parsePathInt(w, r, "rackId", "invalid_rack_id")
	if !ok {
		return
	}

	row := h.grappaDB.QueryRowContext(r.Context(), `
		SELECT
			r.id_rack,
			r.name,
			COALESCE(r.id_anagrafica, 0),
			cf.intestazione,
			db.name,
			d.name,
			r.floor,
			r.island,
			r.type,
			r.pos,
			r.codice_ordine,
			r.serialnumber,
			r.committed_power,
			r.variable_billing,
			r.billing_start_date
		FROM racks r
		LEFT JOIN cli_fatturazione cf ON cf.id = r.id_anagrafica
		LEFT JOIN datacenter d ON d.id_datacenter = r.id_datacenter
		LEFT JOIN dc_build db ON db.id = d.dc_build_id
		WHERE r.id_rack = ?`, rackID)

	var response rackDetailResponse
	var customerName sql.NullString
	var buildingName sql.NullString
	var roomName sql.NullString
	var floor sql.NullInt64
	var island sql.NullInt64
	var rackType sql.NullString
	var position sql.NullString
	var orderCode sql.NullString
	var serialNumber sql.NullString
	var committedPower sql.NullFloat64
	var variableBilling sql.NullInt64
	var billingStart sql.NullTime
	if err := row.Scan(
		&response.ID,
		&response.Name,
		&response.CustomerID,
		&customerName,
		&buildingName,
		&roomName,
		&floor,
		&island,
		&rackType,
		&position,
		&orderCode,
		&serialNumber,
		&committedPower,
		&variableBilling,
		&billingStart,
	); h.rowError(w, r, "get_rack_detail", err, "rack_id", rackID) {
		return
	}

	response.CustomerName = cleanString(customerName)
	response.BuildingName = cleanString(buildingName)
	response.RoomName = cleanString(roomName)
	response.Floor = nullableInt(floor)
	response.Island = nullableInt(island)
	response.RackType = cleanString(rackType)
	response.Position = cleanString(position)
	response.OrderCode = cleanString(orderCode)
	response.SerialNumber = cleanString(serialNumber)
	response.CommittedPower = nullableFloat(committedPower)
	response.VariableBilling = variableBilling.Valid && variableBilling.Int64 > 0
	response.BillingStartDate = formatDate(billingStart, h.config.Location)

	httputil.JSON(w, http.StatusOK, response)
}

func (h *Handler) handleListRackSocketStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rackID, ok := h.parsePathInt(w, r, "rackId", "invalid_rack_id")
	if !ok {
		return
	}

	since := time.Now().In(h.config.Location).Add(-48 * time.Hour).Format(sqlDateTimeLayout)
	rows, err := h.grappaDB.QueryContext(r.Context(), `
		SELECT
			rs.id,
			rs.magnetotermico,
			rs.snmp_monitoring_device,
			rs.detector_ip,
			rs.posizione,
			rs.posizione2,
			rs.posizione3,
			rs.posizione4,
			COALESCE(ROUND(AVG(rpr.ampere), 2), 0) AS avg_ampere
		FROM rack_sockets rs
		LEFT JOIN rack_power_readings rpr
			ON rpr.rack_socket_id = rs.id
		   AND rpr.date >= ?
		WHERE rs.rack_id = ?
		GROUP BY
			rs.id,
			rs.magnetotermico,
			rs.snmp_monitoring_device,
			rs.detector_ip,
			rs.posizione,
			rs.posizione2,
			rs.posizione3,
			rs.posizione4
		ORDER BY rs.posizione ASC, rs.posizione2 ASC, rs.posizione3 ASC, rs.posizione4 ASC, rs.id ASC`, since, rackID)
	if err != nil {
		h.dbFailure(w, r, "list_socket_status", err, "rack_id", rackID)
		return
	}
	defer rows.Close()

	result := make([]rackSocketStatusResponse, 0)
	for rows.Next() {
		var item rackSocketStatusResponse
		var breaker sql.NullString
		var powerMeter sql.NullString
		var detectorIP sql.NullString
		var posizione sql.NullString
		var posizione2 sql.NullString
		var posizione3 sql.NullString
		var posizione4 sql.NullString
		if err := rows.Scan(
			&item.SocketID,
			&breaker,
			&powerMeter,
			&detectorIP,
			&posizione,
			&posizione2,
			&posizione3,
			&posizione4,
			&item.Ampere,
		); err != nil {
			h.dbFailure(w, r, "list_socket_status_scan", err, "rack_id", rackID)
			return
		}

		item.Breaker = cleanString(breaker)
		item.PowerMeter = cleanString(powerMeter)
		item.DetectorIP = cleanString(detectorIP)
		item.Position1 = cleanString(posizione)
		item.Position2 = cleanString(posizione2)
		item.Position3 = cleanString(posizione3)
		item.Position4 = cleanString(posizione4)
		item.Positions = composePositions(
			item.Position1,
			item.Position2,
			item.Position3,
			item.Position4,
		)
		item.Label = socketLabel(item.SocketID, item.Positions)
		item.MaxAmpere = breakerCapacity(item.Breaker)
		item.UsagePercent = gaugePercent(item.Ampere, item.MaxAmpere)
		result = append(result, item)
	}
	if !h.rowsDone(w, r, rows, "list_socket_status") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

func (h *Handler) handleListPowerReadings(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rackID, ok := h.parsePathInt(w, r, "rackId", "invalid_rack_id")
	if !ok {
		return
	}
	fromValue, fromTime, ok := h.parseLocalDateTime(w, r, "from", "invalid_from_parameter")
	if !ok {
		return
	}
	toValue, toTime, ok := h.parseLocalDateTime(w, r, "to", "invalid_to_parameter")
	if !ok {
		return
	}
	if fromTime.After(toTime) {
		httputil.Error(w, http.StatusBadRequest, "invalid_date_range")
		return
	}

	page := defaultPowerReadingsPage
	if rawPage := r.URL.Query().Get("page"); rawPage != "" {
		parsedPage, err := strconv.Atoi(rawPage)
		if err != nil || parsedPage <= 0 {
			httputil.Error(w, http.StatusBadRequest, "invalid_page_parameter")
			return
		}
		page = parsedPage
	}

	size := defaultPowerReadingsSize
	if rawSize := r.URL.Query().Get("size"); rawSize != "" {
		parsedSize, err := strconv.Atoi(rawSize)
		if err != nil || parsedSize <= 0 || parsedSize > maxPowerReadingsSize {
			httputil.Error(w, http.StatusBadRequest, "invalid_size_parameter")
			return
		}
		size = parsedSize
	}

	var total int
	countRow := h.grappaDB.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM rack_power_readings rpr
		JOIN rack_sockets rs ON rs.id = rpr.rack_socket_id
		WHERE rs.rack_id = ?
		  AND rpr.date >= ?
		  AND rpr.date <= ?`, rackID, fromValue, toValue)
	if err := countRow.Scan(&total); err != nil {
		h.dbFailure(w, r, "count_power_readings", err, "rack_id", rackID)
		return
	}

	offset := (page - 1) * size
	rows, err := h.grappaDB.QueryContext(r.Context(), `
		SELECT
			rpr.id,
			rpr.oid,
			rpr.date,
			rpr.ampere,
			rpr.rack_socket_id,
			rs.posizione,
			rs.posizione2,
			rs.posizione3,
			rs.posizione4
		FROM rack_power_readings rpr
		JOIN rack_sockets rs ON rs.id = rpr.rack_socket_id
		WHERE rs.rack_id = ?
		  AND rpr.date >= ?
		  AND rpr.date <= ?
		ORDER BY rpr.date DESC, rpr.id DESC
		LIMIT ?
		OFFSET ?`, rackID, fromValue, toValue, size, offset)
	if err != nil {
		h.dbFailure(w, r, "list_power_readings", err, "rack_id", rackID)
		return
	}
	defer rows.Close()

	items := make([]powerReadingRowResponse, 0)
	for rows.Next() {
		var item powerReadingRowResponse
		var timestamp time.Time
		var posizione sql.NullString
		var posizione2 sql.NullString
		var posizione3 sql.NullString
		var posizione4 sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.OID,
			&timestamp,
			&item.Ampere,
			&item.SocketID,
			&posizione,
			&posizione2,
			&posizione3,
			&posizione4,
		); err != nil {
			h.dbFailure(w, r, "list_power_readings_scan", err, "rack_id", rackID)
			return
		}

		positions := composePositions(
			cleanString(posizione),
			cleanString(posizione2),
			cleanString(posizione3),
			cleanString(posizione4),
		)
		item.SocketLabel = socketLabel(item.SocketID, positions)
		item.Date = formatDateTime(timestamp, h.config.Location)
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_power_readings") {
		return
	}

	httputil.JSON(w, http.StatusOK, powerReadingsPageResponse{
		Items: items,
		Total: total,
		Page:  page,
		Size:  size,
	})
}

func (h *Handler) handleListRackStatsLastDays(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rackID, ok := h.parsePathInt(w, r, "rackId", "invalid_rack_id")
	if !ok {
		return
	}

	since := time.Now().In(h.config.Location).Add(-48 * time.Hour).Format(sqlDateTimeLayout)
	rows, err := h.grappaDB.QueryContext(r.Context(), `
		SELECT
			DATE_FORMAT(rpr.date, '%Y-%m-%d %H:00') AS bucket,
			ROUND(SUM(rpr.ampere), 2) AS total_ampere
		FROM rack_power_readings rpr
		JOIN rack_sockets rs ON rs.id = rpr.rack_socket_id
		WHERE rs.rack_id = ?
		  AND rpr.date >= ?
		GROUP BY DATE_FORMAT(rpr.date, '%Y-%m-%d %H:00')
		ORDER BY bucket ASC`, rackID, since)
	if err != nil {
		h.dbFailure(w, r, "list_stats_last_days", err, "rack_id", rackID)
		return
	}
	defer rows.Close()

	result := make([]rackStatPointResponse, 0)
	for rows.Next() {
		var item rackStatPointResponse
		if err := rows.Scan(&item.Bucket, &item.Ampere); err != nil {
			h.dbFailure(w, r, "list_stats_last_days_scan", err, "rack_id", rackID)
			return
		}
		item.Kilowatt = kilowattFromAmpere(item.Ampere)
		result = append(result, item)
	}
	if !h.rowsDone(w, r, rows, "list_stats_last_days") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}
