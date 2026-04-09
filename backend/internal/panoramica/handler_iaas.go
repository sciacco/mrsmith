package panoramica

import (
	"database/sql"
	"net/http"

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

// handleListDailyCharges returns daily charge totals for an IaaS domain (last 120 days).
// GET /panoramica/v1/iaas/daily-charges?domain=uuid-string
func (h *Handler) handleListDailyCharges(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	domain := r.URL.Query().Get("domain")
	if domain == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_domain_parameter")
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(),
		`SELECT c.charge_day AS giorno, c.domainid,
    CAST(SUM(CASE WHEN c.usage_type = 9999 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utCredit,
    CAST(SUM(c.usage_charge) AS DECIMAL(10,2)) AS total_importo
FROM cdl_charges c
WHERE c.domainid = ? AND charge_day >= DATE_SUB(NOW(), INTERVAL 120 DAY)
GROUP BY c.charge_day, c.domainid
ORDER BY c.charge_day DESC`, domain)
	if err != nil {
		h.dbFailure(w, r, "list_daily_charges", err)
		return
	}
	defer rows.Close()

	type dailyCharge struct {
		Giorno       string  `json:"giorno"`
		DomainID     string  `json:"domainid"`
		UtCredit     float64 `json:"utCredit"`
		TotalImporto float64 `json:"total_importo"`
	}

	var result []dailyCharge
	for rows.Next() {
		var d dailyCharge
		if err := rows.Scan(&d.Giorno, &d.DomainID, &d.UtCredit, &d.TotalImporto); err != nil {
			h.dbFailure(w, r, "list_daily_charges_scan", err)
			return
		}
		result = append(result, d)
	}
	if !h.rowsDone(w, r, rows, "list_daily_charges") {
		return
	}
	if result == nil {
		result = []dailyCharge{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleListMonthlyCharges returns monthly charge totals for an IaaS domain (last 12 months).
// GET /panoramica/v1/iaas/monthly-charges?domain=uuid-string
func (h *Handler) handleListMonthlyCharges(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	domain := r.URL.Query().Get("domain")
	if domain == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_domain_parameter")
		return
	}

	rows, err := h.grappaDB.QueryContext(r.Context(),
		`SELECT DATE_FORMAT(charge_day, '%Y-%m') AS mese, CAST(SUM(usage_charge) AS DECIMAL(7,2)) AS importo
FROM cdl_charges
WHERE domainid = ? AND charge_day >= DATE_SUB(NOW(), INTERVAL 365 DAY)
GROUP BY 1 ORDER BY 1 DESC LIMIT 12`, domain)
	if err != nil {
		h.dbFailure(w, r, "list_monthly_charges", err)
		return
	}
	defer rows.Close()

	type monthlyCharge struct {
		Mese    string  `json:"mese"`
		Importo float64 `json:"importo"`
	}

	var result []monthlyCharge
	for rows.Next() {
		var m monthlyCharge
		if err := rows.Scan(&m.Mese, &m.Importo); err != nil {
			h.dbFailure(w, r, "list_monthly_charges_scan", err)
			return
		}
		result = append(result, m)
	}
	if !h.rowsDone(w, r, rows, "list_monthly_charges") {
		return
	}
	if result == nil {
		result = []monthlyCharge{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleChargeBreakdown returns a typed breakdown of charges for a domain on a specific day.
// GET /panoramica/v1/iaas/charge-breakdown?domain=uuid-string&day=2026-04-01
func (h *Handler) handleChargeBreakdown(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}

	domain := r.URL.Query().Get("domain")
	if domain == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_domain_parameter")
		return
	}
	day := r.URL.Query().Get("day")
	if day == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_day_parameter")
		return
	}

	row := h.grappaDB.QueryRowContext(r.Context(),
		`SELECT c.charge_day, c.domainid,
    CAST(SUM(CASE WHEN c.usage_type = 1 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utRunningVM,
    CAST(SUM(CASE WHEN c.usage_type = 2 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utAllocatedVM,
    CAST(SUM(CASE WHEN c.usage_type = 3 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utIpCharge,
    CAST(SUM(CASE WHEN c.usage_type = 6 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utVolume,
    CAST(SUM(CASE WHEN c.usage_type = 7 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utTemplate,
    CAST(SUM(CASE WHEN c.usage_type = 8 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utISO,
    CAST(SUM(CASE WHEN c.usage_type = 9 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utSnapshot,
    CAST(SUM(CASE WHEN c.usage_type = 26 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utVolumeSecondary,
    CAST(SUM(CASE WHEN c.usage_type = 27 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utVmSnapshotOnPrimary,
    CAST(SUM(CASE WHEN c.usage_type = 9999 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utCredit,
    CAST(SUM(c.usage_charge) AS DECIMAL(10,2)) AS total_importo
FROM cdl_charges c
WHERE c.domainid = ? AND charge_day = ?
GROUP BY c.charge_day, c.domainid
ORDER BY c.charge_day DESC`, domain, day)

	var chargeDay, domainID string
	var utRunningVM, utAllocatedVM, utIpCharge, utVolume, utTemplate float64
	var utISO, utSnapshot, utVolumeSecondary, utVmSnapshotOnPrimary float64
	var utCredit, totalImporto float64

	err := row.Scan(&chargeDay, &domainID,
		&utRunningVM, &utAllocatedVM, &utIpCharge, &utVolume, &utTemplate,
		&utISO, &utSnapshot, &utVolumeSecondary, &utVmSnapshotOnPrimary,
		&utCredit, &totalImporto)

	if err == sql.ErrNoRows {
		httputil.JSON(w, http.StatusOK, chargeBreakdownResponse{
			Charges: []chargeItem{},
			Total:   0,
		})
		return
	}
	if err != nil {
		h.dbFailure(w, r, "charge_breakdown", err)
		return
	}

	type typeMapping struct {
		typ    string
		label  string
		amount float64
	}

	mappings := []typeMapping{
		{"RunningVM", "utRunningVM", utRunningVM},
		{"AllocatedVM", "utAllocatedVM", utAllocatedVM},
		{"IpCharge", "utIpCharge", utIpCharge},
		{"Volume", "utVolume", utVolume},
		{"Template", "utTemplate", utTemplate},
		{"ISO", "utISO", utISO},
		{"Snapshot", "utSnapshot", utSnapshot},
		{"VolumeSecondary", "utVolumeSecondary", utVolumeSecondary},
		{"VmSnapshotOnPrimary", "utVmSnapshotOnPrimary", utVmSnapshotOnPrimary},
		{"Credit", "utCredit", utCredit},
	}

	var charges []chargeItem
	for _, m := range mappings {
		if m.amount != 0 {
			charges = append(charges, chargeItem{Type: m.typ, Label: m.label, Amount: m.amount})
		}
	}
	if charges == nil {
		charges = []chargeItem{}
	}

	httputil.JSON(w, http.StatusOK, chargeBreakdownResponse{
		Charges: charges,
		Total:   totalImporto,
	})
}

type chargeItem struct {
	Type   string  `json:"type"`
	Label  string  `json:"label"`
	Amount float64 `json:"amount"`
}

type chargeBreakdownResponse struct {
	Charges []chargeItem `json:"charges"`
	Total   float64      `json:"total"`
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
