package energiadc

import (
	"database/sql"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// Match the daily-summary source calculation, not the instantaneous 225 V helper:
// positive socket/hour maxima -> rack/hour sums -> rack/day mean, at 230 V.
// Raw observations never leave MySQL. Civil datetime bounds use Europe/Rome.
const kwReportDailySQL = `
 SELECT hourly.rack_id, LEFT(hourly.hour_bucket, 10), AVG(hourly.ampere) * 230 / 1000
 FROM (
   SELECT sockets.rack_id, sockets.hour_bucket, SUM(sockets.ampere) AS ampere
   FROM (
     SELECT rs.rack_id, rs.id AS socket_id,
            DATE_FORMAT(p.date, '%Y-%m-%d %H') AS hour_bucket, MAX(p.ampere) AS ampere
     FROM racks r
     JOIN rack_sockets rs ON rs.rack_id = r.id_rack
     JOIN rack_power_readings p ON p.rack_socket_id = rs.id
     WHERE r.id_anagrafica = ? AND r.stato = 'attivo'
       AND p.date >= ? AND p.date < ? AND p.ampere > 0
     GROUP BY rs.rack_id, rs.id, DATE_FORMAT(p.date, '%Y-%m-%d %H')
   ) sockets
   GROUP BY sockets.rack_id, sockets.hour_bucket
 ) hourly
 GROUP BY hourly.rack_id, LEFT(hourly.hour_bucket, 10)
 ORDER BY hourly.rack_id, LEFT(hourly.hour_bucket, 10)`

// Same aggregation as kwReportDailySQL but without the 230 V conversion: the
// reported value is the ampere average. The Cos φ multiplier is not applied in
// this mode (it only scales active power, not current).
const ampereReportDailySQL = `
 SELECT hourly.rack_id, LEFT(hourly.hour_bucket, 10), AVG(hourly.ampere)
 FROM (
   SELECT sockets.rack_id, sockets.hour_bucket, SUM(sockets.ampere) AS ampere
   FROM (
     SELECT rs.rack_id, rs.id AS socket_id,
            DATE_FORMAT(p.date, '%Y-%m-%d %H') AS hour_bucket, MAX(p.ampere) AS ampere
     FROM racks r
     JOIN rack_sockets rs ON rs.rack_id = r.id_rack
     JOIN rack_power_readings p ON p.rack_socket_id = rs.id
     WHERE r.id_anagrafica = ? AND r.stato = 'attivo'
       AND p.date >= ? AND p.date < ? AND p.ampere > 0
     GROUP BY rs.rack_id, rs.id, DATE_FORMAT(p.date, '%Y-%m-%d %H')
   ) sockets
   GROUP BY sockets.rack_id, sockets.hour_bucket
 ) hourly
 GROUP BY hourly.rack_id, LEFT(hourly.hour_bucket, 10)
 ORDER BY hourly.rack_id, LEFT(hourly.hour_bucket, 10)`

func (h *Handler) handleCustomerKWReport(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	customerID, ok := h.parsePathInt(w, r, "customerId", "invalid_customer_id")
	if !ok {
		return
	}
	year, ok := h.parseRequiredQueryInt(w, r, "year", "invalid_year_parameter")
	if !ok {
		return
	}
	if year < 1000 || year > 9998 {
		httputil.Error(w, 400, "invalid_year_parameter")
		return
	}
	month, ok := h.parseOptionalQueryInt(w, r, "month", "invalid_month_parameter")
	if !ok {
		return
	}
	if month != nil && *month > 12 {
		httputil.Error(w, 400, "invalid_month_parameter")
		return
	}
	cosfi, ok := h.parseRequiredQueryInt(w, r, "cosfi", "invalid_cosfi_parameter")
	if !ok {
		return
	}
	if cosfi < 70 || cosfi > 100 {
		httputil.Error(w, 400, "invalid_cosfi_parameter")
		return
	}
	unit, ok := parseReportUnit(w, r)
	if !ok {
		return
	}
	start := time.Date(year, 1, 1, 0, 0, 0, 0, h.config.Location)
	end := start.AddDate(1, 0, 0)
	if month != nil {
		start = time.Date(year, time.Month(*month), 1, 0, 0, 0, 0, h.config.Location)
		end = start.AddDate(0, 1, 0)
	}
	result := kwReportResponse{Customer: lookupItem{ID: customerID}, Year: year, Month: month, Unit: unit, Rooms: []kwReportRoom{}}
	var customerName sql.NullString
	if h.rowError(w, r, "kw_report_customer", h.grappaDB.QueryRowContext(r.Context(),
		`SELECT intestazione FROM cli_fatturazione WHERE id = ?`, customerID).Scan(&customerName)) {
		return
	}
	result.Customer.Name = cleanString(customerName)

	// Inventory is independent of readings: racks with no observations retain null series.
	rows, err := h.grappaDB.QueryContext(r.Context(), `
   SELECT d.id_datacenter, d.name, r.id_rack, r.name
   FROM racks r JOIN datacenter d ON d.id_datacenter = r.id_datacenter
   WHERE r.id_anagrafica = ? AND r.stato = 'attivo'
     AND EXISTS (SELECT 1 FROM rack_sockets rs WHERE rs.rack_id = r.id_rack)
   ORDER BY d.name, d.id_datacenter, r.name, r.id_rack`, customerID)
	if err != nil {
		h.dbFailure(w, r, "kw_report_inventory", err)
		return
	}
	roomIndexes := map[int]int{}
	rackDays := map[int]map[string]float64{}
	for rows.Next() {
		var roomID, rackID int
		var roomName, rackName sql.NullString
		if err := rows.Scan(&roomID, &roomName, &rackID, &rackName); err != nil {
			rows.Close()
			h.dbFailure(w, r, "kw_report_inventory_scan", err)
			return
		}
		index, found := roomIndexes[roomID]
		if !found {
			index = len(result.Rooms)
			roomIndexes[roomID] = index
			result.Rooms = append(result.Rooms, kwReportRoom{ID: roomID, Name: cleanString(roomName), Racks: []kwReportRack{}})
		}
		result.Rooms[index].Racks = append(result.Rooms[index].Racks, kwReportRack{ID: rackID, Name: cleanString(rackName)})
		rackDays[rackID] = map[string]float64{}
	}
	good := h.rowsDone(w, r, rows, "kw_report_inventory")
	rows.Close()
	if !good {
		return
	}
	rows, err = h.grappaDB.QueryContext(r.Context(), reportDailySQL(unit), customerID, start.Format(sqlDateTimeLayout), end.Format(sqlDateTimeLayout))
	if err != nil {
		h.dbFailure(w, r, "kw_report_daily", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var rackID int
		var day string
		var kw float64
		if err := rows.Scan(&rackID, &day, &kw); err != nil {
			h.dbFailure(w, r, "kw_report_daily_scan", err)
			return
		}
		if days, found := rackDays[rackID]; found {
			if unit == "A" {
				days[day] = kw
			} else {
				days[day] = kw * cosfiMultiplier(cosfi)
			}
		}
	}
	if !h.rowsDone(w, r, rows, "kw_report_daily") {
		return
	}
	customerDays := map[string]float64{}
	for i := range result.Rooms {
		room := &result.Rooms[i]
		roomDays := map[string]float64{}
		for j := range room.Racks {
			rack := &room.Racks[j]
			days := rackDays[rack.ID]
			rack.Series = kwReportSeries(days, start, end, month == nil)
			for day, value := range days {
				roomDays[day] += value
			}
		}
		room.Series = kwReportSeries(roomDays, start, end, month == nil)
		for day, value := range roomDays {
			customerDays[day] += value
		}
	}
	result.Series = kwReportSeries(customerDays, start, end, month == nil)
	httputil.JSON(w, http.StatusOK, result)
}

// Average available daily values independently at each level. Missing days are
// not zeros, and monthly customer totals are not sums of monthly rack averages.
func kwReportSeries(days map[string]float64, start, end time.Time, annual bool) []kwReportPoint {
	type average struct {
		sum   float64
		count int
	}
	buckets := map[string]average{}
	keys := make([]string, 0, len(days))
	for day := range days {
		keys = append(keys, day)
	}
	sort.Strings(keys)
	for _, day := range keys {
		bucket := day
		if annual {
			bucket = day[:7]
		}
		value := buckets[bucket]
		value.sum += days[day]
		value.count++
		buckets[bucket] = value
	}
	result := make([]kwReportPoint, 0, 31)
	for date := start; date.Before(end); {
		bucket := date.Format(dateLayout)
		if annual {
			bucket = date.Format("2006-01")
		}
		point := kwReportPoint{Bucket: bucket}
		if value, exists := buckets[bucket]; exists {
			rounded := roundFloat(value.sum/float64(value.count), 2)
			point.Kilowatt = &rounded
		}
		result = append(result, point)
		if annual {
			date = date.AddDate(0, 1, 0)
		} else {
			date = date.AddDate(0, 0, 1)
		}
	}
	return result
}

// parseReportUnit normalizes the optional `unit` query parameter to the
// canonical "kW" / "A" labels. Empty defaults to kW for backward
// compatibility; unknown values reject with invalid_unit_parameter.
func parseReportUnit(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("unit")))
	if raw == "" {
		return "kW", true
	}
	switch raw {
	case "kw":
		return "kW", true
	case "a", "ampere", "amp":
		return "A", true
	}
	httputil.Error(w, http.StatusBadRequest, "invalid_unit_parameter")
	return "", false
}

// reportDailySQL selects the aggregation query for the requested unit. kW
// applies the 230 V conversion; A returns the raw ampere average.
func reportDailySQL(unit string) string {
	if unit == "A" {
		return ampereReportDailySQL
	}
	return kwReportDailySQL
}
