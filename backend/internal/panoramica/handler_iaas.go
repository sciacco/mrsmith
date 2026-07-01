package panoramica

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleListIaaSAccounts returns active IaaS billing accounts from Grappa.
// GET /panoramica/v1/iaas/accounts
func (h *Handler) handleListIaaSAccounts(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(),
		`SELECT c.intestazione, a.credito, domainuuid AS cloudstack_domain, id_cli_fatturazione,
       abbreviazione, codice_ordine, serialnumber, data_attivazione
FROM cdl_accounts a
JOIN cli_fatturazione c ON a.id_cli_fatturazione = c.id
WHERE id_cli_fatturazione > 0 AND attivo = 1 AND fatturazione = 1
  AND c.codice_aggancio_gest NOT IN (385, 485)
ORDER BY intestazione`)
	if err != nil {
		h.dbFailure(w, r, "list_iaas_accounts", err)
		return
	}
	defer rows.Close()

	type account struct {
		Intestazione      string  `json:"intestazione"`
		Credito           float64 `json:"credito"`
		CloudstackDomain  string  `json:"cloudstack_domain"`
		IDCliFatturazione int     `json:"id_cli_fatturazione"`
		Abbreviazione     *string `json:"abbreviazione"`
		CodiceOrdine      *string `json:"codice_ordine"`
		Serialnumber      *string `json:"serialnumber"`
		DataAttivazione   *string `json:"data_attivazione"`
	}

	var result []account
	for rows.Next() {
		var a account
		var abbreviazione, codiceOrdine, serialnumber, dataAtt sql.NullString

		if err := rows.Scan(
			&a.Intestazione, &a.Credito, &a.CloudstackDomain, &a.IDCliFatturazione,
			&abbreviazione, &codiceOrdine, &serialnumber, &dataAtt,
		); err != nil {
			h.dbFailure(w, r, "list_iaas_accounts_scan", err)
			return
		}

		a.Abbreviazione = nullStringPtr(abbreviazione)
		a.CodiceOrdine = nullStringPtr(codiceOrdine)
		a.Serialnumber = nullStringPtr(serialnumber)
		a.DataAttivazione = nullStringPtr(dataAtt)

		result = append(result, a)
	}
	if !h.rowsDone(w, r, rows, "list_iaas_accounts") {
		return
	}
	if result == nil {
		result = []account{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// bucketExpression maps an aggregation group to its SQL bucketing expression.
// Every expression is wrapped in DATE_FORMAT(..., '%Y-%m-%d') so the bucket is
// always a plain YYYY-MM-DD string. Without this, DATE columns are returned by
// the MySQL driver (parseTime) as time.Time and serialized as RFC3339
// (e.g. 2025-12-22T00:00:00Z), which breaks date validation and formatting.
func bucketExpression(group string) (string, bool) {
	switch group {
	case "daily":
		return "DATE_FORMAT(charge_day, '%Y-%m-%d')", true
	case "weekly":
		// Monday of the week
		return "DATE_FORMAT(DATE_SUB(charge_day, INTERVAL WEEKDAY(charge_day) DAY), '%Y-%m-%d')", true
	case "monthly":
		return "DATE_FORMAT(charge_day, '%Y-%m-01')", true
	case "quarterly":
		return "DATE_FORMAT(DATE(CONCAT(YEAR(charge_day), '-', LPAD((QUARTER(charge_day)-1)*3+1, 2, '0'), '-01')), '%Y-%m-%d')", true
	case "yearly":
		return "DATE_FORMAT(charge_day, '%Y-01-01')", true
	default:
		return "", false
	}
}

// validChargeDate validates a YYYY-MM-DD parameter.
func validChargeDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// handleListCharges returns an aggregated charge time series for an IaaS domain.
// GET /panoramica/v1/iaas/charges?domain=uuid&from=YYYY-MM-DD&to=YYYY-MM-DD&group=daily|weekly|monthly|quarterly|yearly
//
// usage_type 9999 (Credit) is excluded from totals.
func (h *Handler) handleListCharges(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	q := r.URL.Query()
	domain := q.Get("domain")
	if domain == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_domain_parameter")
		return
	}
	from := q.Get("from")
	if !validChargeDate(from) {
		httputil.Error(w, http.StatusBadRequest, "invalid_from_parameter")
		return
	}
	to := q.Get("to")
	if !validChargeDate(to) {
		httputil.Error(w, http.StatusBadRequest, "invalid_to_parameter")
		return
	}
	group := strings.TrimSpace(q.Get("group"))
	if group == "" {
		group = "daily"
	}
	bucketExpr, ok := bucketExpression(group)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_group_parameter")
		return
	}

	query := "SELECT " + bucketExpr + " AS bucket, " +
		"CAST(SUM(CASE WHEN usage_type = 9999 THEN 0 ELSE usage_charge END) AS DECIMAL(12,2)) AS total_importo " +
		"FROM cdl_charges " +
		"WHERE domainid = ? AND charge_day BETWEEN ? AND ? " +
		"GROUP BY bucket ORDER BY bucket ASC"

	rows, err := h.grappaDB.QueryContext(r.Context(), query, domain, from, to)
	if err != nil {
		h.dbFailure(w, r, "list_charges", err)
		return
	}
	defer rows.Close()

	type seriesPoint struct {
		Bucket        string  `json:"bucket"`
		TotalImporto  float64 `json:"total_importo"`
	}

	var result []seriesPoint
	for rows.Next() {
		var p seriesPoint
		if err := rows.Scan(&p.Bucket, &p.TotalImporto); err != nil {
			h.dbFailure(w, r, "list_charges_scan", err)
			return
		}
		result = append(result, p)
	}
	if !h.rowsDone(w, r, rows, "list_charges") {
		return
	}
	if result == nil {
		result = []seriesPoint{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// categoryFromUsageType maps a raw usage_type to a billing macro-category.
// 9999 (Credit) must be filtered out by the caller and never reaches here.
func categoryFromUsageType(t sql.NullInt64) string {
	if !t.Valid {
		return "Altro"
	}
	switch t.Int64 {
	case 2:
		return "VM"
	case 6, 7, 8, 9:
		return "Storage"
	case 9998:
		return "Licenze Windows"
	default:
		return "Altro"
	}
}

// handleChargesByCategory returns the charge composition by macro-category over a period.
// GET /panoramica/v1/iaas/charges-by-category?domain=uuid&from=YYYY-MM-DD&to=YYYY-MM-DD
//
// Categories: VM (2), Storage (6,7,8,9), Licenze Windows (9998), Altro (1,3,26,27,NULL,unknown).
// usage_type 9999 (Credit) is excluded. Total equals the sum of the returned categories.
func (h *Handler) handleChargesByCategory(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	q := r.URL.Query()
	domain := q.Get("domain")
	if domain == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_domain_parameter")
		return
	}
	from := q.Get("from")
	if !validChargeDate(from) {
		httputil.Error(w, http.StatusBadRequest, "invalid_from_parameter")
		return
	}
	to := q.Get("to")
	if !validChargeDate(to) {
		httputil.Error(w, http.StatusBadRequest, "invalid_to_parameter")
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(),
		`SELECT
  CASE
    WHEN usage_type = 2 THEN 'VM'
    WHEN usage_type IN (6,7,8,9) THEN 'Storage'
    WHEN usage_type = 9998 THEN 'Licenze Windows'
    ELSE 'Altro'
  END AS category,
  CAST(SUM(usage_charge) AS DECIMAL(12,2)) AS amount
FROM cdl_charges
WHERE domainid = ? AND charge_day BETWEEN ? AND ?
  AND (usage_type IS NULL OR usage_type != 9999)
GROUP BY category
ORDER BY FIELD(category, 'VM', 'Storage', 'Licenze Windows', 'Altro')`, domain, from, to)
	if err != nil {
		h.dbFailure(w, r, "charges_by_category", err)
		return
	}
	defer rows.Close()

	type categoryAmount struct {
		Category string  `json:"category"`
		Amount   float64 `json:"amount"`
	}

	var categories []categoryAmount
	var total float64
	for rows.Next() {
		var ca categoryAmount
		if err := rows.Scan(&ca.Category, &ca.Amount); err != nil {
			h.dbFailure(w, r, "charges_by_category_scan", err)
			return
		}
		total += ca.Amount
		categories = append(categories, ca)
	}
	if !h.rowsDone(w, r, rows, "charges_by_category") {
		return
	}
	if categories == nil {
		categories = []categoryAmount{}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"categories": categories,
		"total":      total,
	})
}

// handleListWindowsLicenses returns daily Windows license counts (last 14 days).
// GET /panoramica/v1/iaas/windows-licenses
func (h *Handler) handleListWindowsLicenses(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(),
		`SELECT charge_day AS x, COUNT(0) AS y
FROM cdl_charges
WHERE charge_day >= CURDATE() - INTERVAL 14 DAY AND usage_type = 9998
GROUP BY charge_day ORDER BY charge_day DESC`)
	if err != nil {
		h.dbFailure(w, r, "list_windows_licenses", err)
		return
	}
	defer rows.Close()

	type licenseCount struct {
		X string `json:"x"`
		Y int    `json:"y"`
	}

	var result []licenseCount
	for rows.Next() {
		var l licenseCount
		if err := rows.Scan(&l.X, &l.Y); err != nil {
			h.dbFailure(w, r, "list_windows_licenses_scan", err)
			return
		}
		result = append(result, l)
	}
	if !h.rowsDone(w, r, rows, "list_windows_licenses") {
		return
	}
	if result == nil {
		result = []licenseCount{}
	}

	httputil.JSON(w, http.StatusOK, result)
}
