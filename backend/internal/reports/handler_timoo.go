package reports

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleTimooDailyStats returns daily user and service extension accounting
// per TIMOO tenant for the last 3 months.
// GET /reports/v1/timoo/daily-stats
func (h *Handler) handleTimooDailyStats(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) {
		return
	}

	rows, err := h.anisettaDB.QueryContext(r.Context(), `
SELECT t.as7_tenant_id as tenant_id, t.name as tenant_name,
       DATE(a.giorno) AS day,
       SUM(a.max_users) as users, SUM(a.max_se) as service_extensions
FROM (
    SELECT as7_tenant_id, pbx_id,
           DATE(data) AS giorno,
           MAX(users) AS max_users,
           MAX(service_extensions) AS max_se
    FROM as7_pbx_accounting
    WHERE data >= DATE_TRUNC('month', CURRENT_DATE - INTERVAL '3 month')
      AND data < CURRENT_DATE
    GROUP BY as7_tenant_id, pbx_id, DATE(data)
) a
JOIN as7_tenants t ON a.as7_tenant_id = t.as7_tenant_id
WHERE t.name IS NOT NULL AND t.name != 'KlajdiandCo'
GROUP BY t.as7_tenant_id, t.name, DATE(a.giorno)
ORDER BY day DESC, tenant_id`)
	if err != nil {
		h.dbFailure(w, r, "timoo_daily_stats", err)
		return
	}
	defer rows.Close()

	type dailyStat struct {
		TenantID          int     `json:"tenant_id"`
		TenantName        *string `json:"tenant_name"`
		Day               string  `json:"day"`
		Users             int     `json:"users"`
		ServiceExtensions int     `json:"service_extensions"`
	}

	var result []dailyStat
	for rows.Next() {
		var s dailyStat
		var tenantName sql.NullString

		if err := rows.Scan(
			&s.TenantID, &tenantName, &s.Day,
			&s.Users, &s.ServiceExtensions,
		); err != nil {
			h.dbFailure(w, r, "timoo_daily_stats_scan", err)
			return
		}

		s.TenantName = nullStringPtr(tenantName)
		result = append(result, s)
	}
	if !h.rowsDone(w, r, rows, "timoo_daily_stats") {
		return
	}
	if result == nil {
		result = []dailyStat{}
	}

	httputil.JSON(w, http.StatusOK, result)
}
