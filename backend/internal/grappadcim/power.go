package grappadcim

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListRackSockets(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_rack_id")
		return
	}
	items, err := h.listSocketsForRack(r, id)
	if err != nil {
		h.dbFailure(w, r, "list_rack_sockets", err, "rack_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleCreateRackSocket(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rackID, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_rack_id")
		return
	}
	var body RackSocketInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_socket_payload")
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `
		INSERT INTO rack_sockets
			(rack_id, magnetotermico, snmp_monitoring_device, detector_ip, oid, oid2, oid3, oid4,
			 posizione, posizione2, posizione3, posizione4, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rackID,
		optionalStringValue(body.Magnetotermico),
		optionalStringValue(body.SNMPMonitoringDevice),
		optionalStringValue(body.DetectorIP),
		optionalStringValue(body.OID),
		optionalStringValue(body.OID2),
		optionalStringValue(body.OID3),
		optionalStringValue(body.OID4),
		optionalStringValue(body.Position),
		optionalStringValue(body.Position2),
		optionalStringValue(body.Position3),
		optionalStringValue(body.Position4),
		socketStatusOrDefault(body.Status),
	)
	if err != nil {
		h.dbFailure(w, r, "create_rack_socket", err, "rack_id", rackID)
		return
	}
	id, _ := result.LastInsertId()
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Socket aggiunto."})
}

func (h *Handler) handleUpdateRackSocket(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	socketID, err := parsePathInt(r, "socketId")
	if err != nil {
		invalidRequest(w, "invalid_socket_id")
		return
	}
	var body RackSocketInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_socket_payload")
		return
	}
	sets, args := socketPatch(body)
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: socketID, Message: "Nessuna modifica."})
		return
	}
	args = append(args, socketID)
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE rack_sockets SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		h.dbFailure(w, r, "update_rack_socket", err, "socket_id", socketID)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "socket_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: socketID, Message: "Socket aggiornato."})
}

func (h *Handler) handleDeleteRackSocket(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	socketID, err := parsePathInt(r, "socketId")
	if err != nil {
		invalidRequest(w, "invalid_socket_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	deps, err := h.runDependencyChecks(r, socketID, []dependencyCheck{
		{Key: "readings", Label: "Letture storiche", Query: `SELECT COUNT(*) FROM rack_power_readings WHERE rack_socket_id = ?`},
	})
	if err != nil {
		h.dbFailure(w, r, "rack_socket_delete_dependencies", err, "socket_id", socketID)
		return
	}
	if !deps.Allowed {
		httputil.JSON(w, http.StatusConflict, deps)
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `DELETE FROM rack_sockets WHERE id = ?`, socketID)
	if err != nil {
		h.dbFailure(w, r, "delete_rack_socket", err, "socket_id", socketID)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "socket_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: socketID, Message: "Socket eliminato."})
}

func (h *Handler) handleListRackPowerReadings(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rackID, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_rack_id")
		return
	}
	page := queryPositiveInt(r, "page", 1)
	size := queryPositiveInt(r, "size", 50)
	if size > 500 {
		size = 500
	}
	where := []string{"rs.rack_id = ?"}
	args := []any{rackID}
	if from := strings.TrimSpace(r.URL.Query().Get("from")); from != "" {
		where = append(where, "rpr.date >= ?")
		args = append(args, from)
	}
	if to := strings.TrimSpace(r.URL.Query().Get("to")); to != "" {
		where = append(where, "rpr.date <= ?")
		args = append(args, to)
	}
	var total int
	if err := h.grappa.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM rack_power_readings rpr JOIN rack_sockets rs ON rs.id = rpr.rack_socket_id WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		h.dbFailure(w, r, "count_power_readings", err, "rack_id", rackID)
		return
	}
	offset := (page - 1) * size
	queryArgs := append(append([]any{}, args...), size, offset)
	rows, err := h.grappa.QueryContext(r.Context(), `
		SELECT rpr.id, rpr.oid, rpr.date, rpr.ampere, rpr.rack_socket_id
		FROM rack_power_readings rpr
		JOIN rack_sockets rs ON rs.id = rpr.rack_socket_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY rpr.date DESC, rpr.id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		h.dbFailure(w, r, "list_power_readings", err, "rack_id", rackID)
		return
	}
	defer rows.Close()
	items := []RackPowerReading{}
	for rows.Next() {
		var item RackPowerReading
		var date sql.NullTime
		if err := rows.Scan(&item.ID, &item.OID, &date, &item.Ampere, &item.RackSocketID); err != nil {
			h.dbFailure(w, r, "list_power_readings_scan", err, "rack_id", rackID)
			return
		}
		if value := nullableTime(date); value != nil {
			item.Date = *value
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_power_readings") {
		return
	}
	httputil.JSON(w, http.StatusOK, RackPowerReadingsResponse{Items: items, Total: total, Page: page, Size: size})
}

func (h *Handler) handleRackPowerSummary(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rackID, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_rack_id")
		return
	}
	rows, err := h.grappa.QueryContext(r.Context(), `
		SELECT s.giorno, s.kilowatt
		FROM rack_power_daily_summary s
		JOIN racks r ON r.id_anagrafica = s.id_anagrafica
		WHERE r.id_rack = ?
		ORDER BY s.giorno DESC
		LIMIT 90`, rackID)
	if err != nil {
		h.dbFailure(w, r, "rack_power_summary", err, "rack_id", rackID)
		return
	}
	defer rows.Close()
	items := []RackPowerSummaryPoint{}
	for rows.Next() {
		var item RackPowerSummaryPoint
		var day sql.NullTime
		var kw sql.NullFloat64
		if err := rows.Scan(&day, &kw); err != nil {
			h.dbFailure(w, r, "rack_power_summary_scan", err, "rack_id", rackID)
			return
		}
		item.Day = nullableDate(day)
		item.Kilowatt = nullableFloat(kw)
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "rack_power_summary") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) listSocketsForRack(r *http.Request, rackID int) ([]RackSocket, error) {
	rows, err := h.grappa.QueryContext(r.Context(), `
		SELECT rs.id, rs.rack_id, rs.magnetotermico, rs.snmp_monitoring_device, rs.detector_ip,
		       rs.oid, rs.oid2, rs.oid3, rs.oid4, rs.posizione, rs.posizione2, rs.posizione3, rs.posizione4, rs.status,
		       latest.ampere, latest.date
		FROM rack_sockets rs
		LEFT JOIN (
			SELECT r1.rack_socket_id, r1.ampere, r1.date
			FROM rack_power_readings r1
			JOIN (
				SELECT rack_socket_id, MAX(date) AS max_date
				FROM rack_power_readings
				GROUP BY rack_socket_id
			) mx ON mx.rack_socket_id = r1.rack_socket_id AND mx.max_date = r1.date
		) latest ON latest.rack_socket_id = rs.id
		WHERE rs.rack_id = ?
		ORDER BY rs.posizione ASC, rs.posizione2 ASC, rs.id ASC`, rackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RackSocket{}
	for rows.Next() {
		var item RackSocket
		var rack sql.NullInt64
		var latestAmp sql.NullFloat64
		var latestAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &rack, &item.Magnetotermico, &item.SNMPMonitoringDevice, &item.DetectorIP,
			&item.OID, &item.OID2, &item.OID3, &item.OID4, &item.Position, &item.Position2,
			&item.Position3, &item.Position4, &item.Status, &latestAmp, &latestAt,
		); err != nil {
			return nil, err
		}
		item.RackID = nullableInt(rack)
		item.LatestAmpere = nullableFloat(latestAmp)
		item.LatestReadingAt = nullableTime(latestAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func socketPatch(body RackSocketInput) ([]string, []any) {
	sets := []string{}
	args := []any{}
	fields := []struct {
		column string
		value  *string
	}{
		{"magnetotermico", body.Magnetotermico},
		{"snmp_monitoring_device", body.SNMPMonitoringDevice},
		{"detector_ip", body.DetectorIP},
		{"oid", body.OID},
		{"oid2", body.OID2},
		{"oid3", body.OID3},
		{"oid4", body.OID4},
		{"posizione", body.Position},
		{"posizione2", body.Position2},
		{"posizione3", body.Position3},
		{"posizione4", body.Position4},
		{"status", body.Status},
	}
	for _, field := range fields {
		if field.value != nil {
			sets = append(sets, field.column+" = ?")
			args = append(args, strings.TrimSpace(*field.value))
		}
	}
	return sets, args
}

func socketStatusOrDefault(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "Acceso"
	}
	return strings.TrimSpace(*value)
}
