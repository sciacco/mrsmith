package energiadc

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const maxDailyKWPoints = 40

type kwSourcePoint struct {
	Day      string
	Kilowatt float64
}

func (h *Handler) handleListCustomerKW(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	customerID, ok := h.parsePathInt(w, r, "customerId", "invalid_customer_id")
	if !ok {
		return
	}

	period := r.URL.Query().Get("period")
	if period != "day" && period != "month" {
		httputil.Error(w, http.StatusBadRequest, "invalid_period_parameter")
		return
	}

	cosfi, ok := h.parseRequiredQueryInt(w, r, "cosfi", "invalid_cosfi_parameter")
	if !ok {
		return
	}
	if cosfi < 70 || cosfi > 100 {
		httputil.Error(w, http.StatusBadRequest, "invalid_cosfi_parameter")
		return
	}

	multiplier := cosfiMultiplier(cosfi)
	var (
		query string
		args  []any
	)
	if period == "day" {
		query = fmt.Sprintf(`
			SELECT day, kilowatt
			FROM (
				SELECT
					DATE_FORMAT(giorno, '%%Y-%%m-%%d') AS day,
					ROUND(COALESCE(kilowatt, 0) * ?, 2) AS kilowatt
				FROM rack_power_daily_summary
				WHERE id_anagrafica = ?
				ORDER BY giorno DESC
				LIMIT %d
			) recent_days
			ORDER BY day ASC`, maxDailyKWPoints)
		args = []any{multiplier, customerID}
	} else {
		query = `
			SELECT
				DATE_FORMAT(MIN(giorno), '%Y-%m-01') AS month_start,
				ROUND(AVG(COALESCE(kilowatt, 0)) * ?, 2) AS kilowatt
			FROM rack_power_daily_summary
			WHERE id_anagrafica = ?
			GROUP BY DATE_FORMAT(giorno, '%Y-%m')
			ORDER BY MIN(giorno) ASC`
		args = []any{multiplier, customerID}
	}

	rows, err := h.grappaDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_customer_kw", err, "customer_id", customerID, "period", period)
		return
	}
	defer rows.Close()

	source := make([]kwSourcePoint, 0)
	for rows.Next() {
		var item kwSourcePoint
		if err := rows.Scan(&item.Day, &item.Kilowatt); err != nil {
			h.dbFailure(w, r, "list_customer_kw_scan", err, "customer_id", customerID, "period", period)
			return
		}
		source = append(source, item)
	}
	if !h.rowsDone(w, r, rows, "list_customer_kw") {
		return
	}

	var (
		result   []kwPointResponse
		shapeErr error
	)
	if period == "day" {
		result, shapeErr = buildDailyKWSeries(source)
	} else {
		result, shapeErr = buildMonthlyKWSeries(source)
	}
	if shapeErr != nil {
		h.dbFailure(w, r, "list_customer_kw_shape", shapeErr, "customer_id", customerID, "period", period)
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

func buildDailyKWSeries(source []kwSourcePoint) ([]kwPointResponse, error) {
	result := make([]kwPointResponse, 0, len(source))
	for _, item := range source {
		day, err := parseKWDay(item.Day)
		if err != nil {
			return nil, err
		}
		result = append(result, kwPointResponse{
			Bucket:     item.Day,
			Label:      day.Format("02/01"),
			RangeLabel: day.Format("02/01/2006"),
			Kilowatt:   item.Kilowatt,
		})
	}
	return result, nil
}

func buildMonthlyKWSeries(source []kwSourcePoint) ([]kwPointResponse, error) {
	result := make([]kwPointResponse, 0, len(source))
	for _, item := range source {
		monthStart, err := parseKWDay(item.Day)
		if err != nil {
			return nil, err
		}
		label := monthStart.Format("01/2006")
		result = append(result, kwPointResponse{
			Bucket:     monthStart.Format("2006-01"),
			Label:      label,
			RangeLabel: label,
			Kilowatt:   item.Kilowatt,
		})
	}
	return result, nil
}

func parseKWDay(raw string) (time.Time, error) {
	day, err := time.Parse(dateLayout, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse kw day %q: %w", raw, err)
	}
	return day, nil
}
