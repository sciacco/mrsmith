package panoramica

import (
	"net/http"
	"strconv"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleListTimooTenants returns Anisetta tenants (excluding KlajdiandCo).
// GET /panoramica/v1/timoo/tenants
func (h *Handler) handleListTimooTenants(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) {
		return
	}

	rows, err := h.anisettaDB.QueryContext(r.Context(),
		`SELECT as7_tenant_id, name FROM public."as7_tenants" WHERE name != 'KlajdiandCo'`)
	if err != nil {
		h.dbFailure(w, r, "list_timoo_tenants", err)
		return
	}
	defer rows.Close()

	type tenant struct {
		AS7TenantID int    `json:"as7_tenant_id"`
		Name        string `json:"name"`
	}

	var result []tenant
	for rows.Next() {
		var t tenant
		if err := rows.Scan(&t.AS7TenantID, &t.Name); err != nil {
			h.dbFailure(w, r, "list_timoo_tenants_scan", err)
			return
		}
		result = append(result, t)
	}
	if !h.rowsDone(w, r, rows, "list_timoo_tenants") {
		return
	}
	if result == nil {
		result = []tenant{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleGetPbxStats returns PBX stats for a tenant with computed totals.
// GET /panoramica/v1/timoo/pbx-stats?tenant=123
func (h *Handler) handleGetPbxStats(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) {
		return
	}

	tenantStr := r.URL.Query().Get("tenant")
	if tenantStr == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_tenant_parameter")
		return
	}
	tenantID, err := strconv.Atoi(tenantStr)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_tenant_parameter")
		return
	}

	rows, err := h.anisettaDB.QueryContext(r.Context(),
		`SELECT as7_tenant_id, pbx_id, pbx_name, MAX(users) AS users, MAX(service_extensions) AS service_extensions
FROM public.as7_pbx_accounting apb
WHERE as7_tenant_id = $1
  AND to_char(data, 'YYYY-MM-DD') = (SELECT to_char(data, 'YYYY-MM-DD') FROM public.as7_pbx_accounting ORDER BY id DESC LIMIT 1)
GROUP BY as7_tenant_id, pbx_id, pbx_name
ORDER BY pbx_name`, tenantID)
	if err != nil {
		h.dbFailure(w, r, "get_pbx_stats", err)
		return
	}
	defer rows.Close()

	type pbxRow struct {
		PbxName           string `json:"pbx_name"`
		PbxID             int    `json:"pbx_id"`
		Users             int    `json:"users"`
		ServiceExtensions int    `json:"service_extensions"`
		Totale            int    `json:"totale"`
	}

	var pbxRows []pbxRow
	var totalUsers, totalSE int

	for rows.Next() {
		var p pbxRow
		var tenantIDScan int
		if err := rows.Scan(&tenantIDScan, &p.PbxID, &p.PbxName, &p.Users, &p.ServiceExtensions); err != nil {
			h.dbFailure(w, r, "get_pbx_stats_scan", err)
			return
		}
		p.Totale = p.Users + p.ServiceExtensions
		totalUsers += p.Users
		totalSE += p.ServiceExtensions
		pbxRows = append(pbxRows, p)
	}
	if !h.rowsDone(w, r, rows, "get_pbx_stats") {
		return
	}
	if pbxRows == nil {
		pbxRows = []pbxRow{}
	}

	type response struct {
		Rows       []pbxRow `json:"rows"`
		TotalUsers int      `json:"totalUsers"`
		TotalSE    int      `json:"totalSE"`
	}

	httputil.JSON(w, http.StatusOK, response{
		Rows:       pbxRows,
		TotalUsers: totalUsers,
		TotalSE:    totalSE,
	})
}
