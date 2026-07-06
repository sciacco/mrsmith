package statsrda

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/xuri/excelize/v2"
)

const riepilogoRDAAllowedStateFilter = `po.state IN (
  'CLOSED',
  'DELIVERED_AND_COMPLIANT',
  'PENDING_CHECK_DOCUMENT',
  'PENDING_CONTRACT_VERIFICATION',
  'PENDING_DISPUTE',
  'PENDING_ERP_SAVE',
  'PENDING_LEASING',
  'PENDING_LEASING_ORDER_CREATION',
  'PENDING_PDF_GENERATION',
  'PENDING_PROVIDER_SAVED_IN_ALYANTE',
  'PENDING_SEND',
  'PENDING_VERIFICATION'
)`

const riepilogoRDABudgetQuery = `
SELECT
  COALESCE(po.budget_id::text, 'senza-budget') AS budget_key,
  CASE
    WHEN po.budget_id IS NULL OR b."name" IS NULL OR btrim(b."name") = ''
    THEN 'Senza budget'
    WHEN b."year" IS NULL
    THEN btrim(b."name")
    ELSE btrim(b."name") || ' ' || b."year"::text
  END AS budget_label,
  po.budget_id,
  NULLIF(btrim(b."name"), '') AS budget_name,
  b."year" AS budget_year,
  COUNT(*) AS order_count,
  COALESCE(SUM(COALESCE(po.total_price, 0)), 0) AS amount
FROM rda.purchase_order po
LEFT JOIN budgets.budget b ON po.budget_id = b.id
WHERE po.deleted IS NULL
  AND ` + riepilogoRDAAllowedStateFilter + `
  AND po.created >= $1
  AND po.created < $2
GROUP BY 1, 2, 3, 4, 5
ORDER BY amount DESC, budget_label ASC`

const riepilogoRDADetailQuery = `
SELECT
  COALESCE(po.code, '') AS code,
  po.project,
  po.object,
  u.email AS requester,
  po.budget_id,
  NULLIF(btrim(b."name"), '') AS budget_name,
  b."year" AS budget_year,
  po.cost_center,
  po.currency,
  COALESCE(po.total_price, 0) AS total_price,
  po.state,
  to_char(po.created, 'YYYY-MM-DD"T"HH24:MI:SSOF') AS created,
  f.company_name
FROM rda.purchase_order po
LEFT JOIN provider_qualifications.provider f ON po.provider_id = f.id
LEFT JOIN users_int."user" u ON po.requester_id = u.id
LEFT JOIN budgets.budget b ON po.budget_id = b.id
WHERE po.deleted IS NULL
  AND ` + riepilogoRDAAllowedStateFilter + `
  AND po.created >= $1
  AND po.created < $2
ORDER BY po.created DESC, COALESCE(po.code, '') ASC`

func (h *Handler) handleRiepilogoRDA(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	query := r.URL.Query()
	from, to, preset, err := resolveRiepilogoRequestPeriod(query.Get("period"), query.Get("from"), query.Get("to"), time.Now())
	if err != nil {
		badRequest(w, err.Error())
		return
	}

	resp, err := h.loadRiepilogoRDA(r, from, to, preset)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda riepilogo rda query failed")
		return
	}

	httputil.JSON(w, http.StatusOK, resp)
}

func (h *Handler) handleRiepilogoRDAExport(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	query := r.URL.Query()
	from, to, preset, err := resolveRiepilogoRequestPeriod(query.Get("period"), query.Get("from"), query.Get("to"), time.Now())
	if err != nil {
		badRequest(w, err.Error())
		return
	}

	resp, err := h.loadRiepilogoRDA(r, from, to, preset)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda riepilogo rda export query failed")
		return
	}

	buf, err := buildRiepilogoRDAExcel(resp)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda riepilogo rda export build failed")
		return
	}

	filename := fmt.Sprintf("riepilogo-rda_%s_%s_%s.xlsx", resp.Period.Preset, resp.Period.From, resp.Period.To)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	_, _ = w.Write(buf)
}

func (h *Handler) loadRiepilogoRDA(r *http.Request, from, to time.Time, preset string) (RiepilogoRDAResponse, error) {
	ctx := r.Context()
	resp := RiepilogoRDAResponse{
		Period: RiepilogoPeriod{
			Preset: preset,
			From:   from.Format(riepilogoDateLayout),
			To:     to.Format(riepilogoDateLayout),
		},
		Budgets: []RiepilogoRDABudget{},
		Details: []RiepilogoRDADetail{},
	}

	budgetRows, err := h.db.QueryContext(ctx, riepilogoRDABudgetQuery, from, to)
	if err != nil {
		return resp, err
	}
	defer budgetRows.Close()

	for budgetRows.Next() {
		var b RiepilogoRDABudget
		var budgetID, budgetYear nullInt
		var budgetName nullString
		if err := budgetRows.Scan(
			&b.BudgetKey, &b.Budget, &budgetID, &budgetName, &budgetYear,
			&b.OrderCount, &b.Amount,
		); err != nil {
			return resp, err
		}
		b.BudgetID = budgetID.ptr()
		b.BudgetName = budgetName.ptr()
		b.BudgetYear = budgetYear.intPtr()
		resp.Totals.Amount += b.Amount
		resp.Budgets = append(resp.Budgets, b)
	}
	if err := budgetRows.Err(); err != nil {
		return resp, err
	}

	detailRows, err := h.db.QueryContext(ctx, riepilogoRDADetailQuery, from, to)
	if err != nil {
		return resp, err
	}
	defer detailRows.Close()

	for detailRows.Next() {
		var d RiepilogoRDADetail
		var project, object, requester, budgetName, costCenter, currency, state, created, companyName nullString
		var budgetID, budgetYear nullInt
		if err := detailRows.Scan(
			&d.Code, &project, &object, &requester, &budgetID, &budgetName, &budgetYear,
			&costCenter, &currency, &d.TotalPrice, &state, &created, &companyName,
		); err != nil {
			return resp, err
		}
		d.Project = project.ptr()
		d.Object = object.ptr()
		d.Requester = requester.ptr()
		d.BudgetID = budgetID.ptr()
		d.BudgetName = budgetName.ptr()
		d.BudgetYear = budgetYear.intPtr()
		d.CostCenter = costCenter.ptr()
		d.Currency = currency.ptr()
		d.State = state.ptr()
		d.Created = created.ptr()
		d.CompanyName = companyName.ptr()
		resp.Details = append(resp.Details, d)
	}
	if err := detailRows.Err(); err != nil {
		return resp, err
	}

	resp.Totals.OrderCount = len(resp.Details)
	resp.Totals.BudgetCount = len(resp.Budgets)
	if resp.Totals.Amount > 0 {
		for i := range resp.Budgets {
			resp.Budgets[i].Percentage = resp.Budgets[i].Amount / resp.Totals.Amount * 100
		}
	}

	return resp, nil
}

func buildRiepilogoRDAExcel(resp RiepilogoRDAResponse) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	const totalsSheet = "Totali budget"
	if err := f.SetSheetName("Sheet1", totalsSheet); err != nil {
		return nil, err
	}

	if err := writeRiepilogoRDATotalsSheet(f, totalsSheet, resp.Budgets); err != nil {
		return nil, err
	}

	detailsByBudgetKey := make(map[string][]RiepilogoRDADetail, len(resp.Budgets))
	for _, detail := range resp.Details {
		detailsByBudgetKey[riepilogoRDADetailBudgetKey(detail)] = append(detailsByBudgetKey[riepilogoRDADetailBudgetKey(detail)], detail)
	}

	usedSheetNames := map[string]bool{canonicalRiepilogoSheetName(totalsSheet): true}
	for _, budget := range resp.Budgets {
		sheet := uniqueRiepilogoSheetName(riepilogoRDABudgetSheetName(budget), usedSheetNames)
		if _, err := f.NewSheet(sheet); err != nil {
			return nil, err
		}
		if err := writeRiepilogoRDADetailSheet(f, sheet, detailsByBudgetKey[budget.BudgetKey]); err != nil {
			return nil, err
		}
	}

	f.SetActiveSheet(0)
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeRiepilogoRDATotalsSheet(f *excelize.File, sheet string, budgets []RiepilogoRDABudget) error {
	headers := []string{"Budget", "Anno budget", "Numero ordini", "Importo totale", "Percentuale sul totale periodo"}
	if err := writeRiepilogoHeaderRow(f, sheet, headers); err != nil {
		return err
	}

	for i, budget := range budgets {
		row := i + 2
		values := []any{
			riepilogoRDABudgetName(budget),
			intPtrValue(budget.BudgetYear),
			budget.OrderCount,
			budget.Amount,
			budget.Percentage,
		}
		if err := writeRiepilogoRow(f, sheet, row, values); err != nil {
			return err
		}
	}

	return setRiepilogoColumnWidths(f, sheet, []float64{32, 16, 16, 18, 30})
}

func writeRiepilogoRDADetailSheet(f *excelize.File, sheet string, details []RiepilogoRDADetail) error {
	headers := []string{
		"Codice ordine", "Progetto", "Oggetto", "Budget", "Anno budget", "Centro di costo",
		"Importo totale", "Valuta", "Richiedente", "Fornitore", "Stato", "Data creazione",
	}
	if err := writeRiepilogoHeaderRow(f, sheet, headers); err != nil {
		return err
	}

	for i, detail := range details {
		row := i + 2
		values := []any{
			detail.Code,
			stringPtrValue(detail.Project),
			stringPtrValue(detail.Object),
			riepilogoRDADetailBudgetName(detail),
			intPtrValue(detail.BudgetYear),
			stringPtrValue(detail.CostCenter),
			detail.TotalPrice,
			stringPtrValue(detail.Currency),
			stringPtrValue(detail.Requester),
			stringPtrValue(detail.CompanyName),
			riepilogoRDAStateLabel(detail.State),
			stringPtrValue(detail.Created),
		}
		if err := writeRiepilogoRow(f, sheet, row, values); err != nil {
			return err
		}
	}

	return setRiepilogoColumnWidths(f, sheet, []float64{18, 36, 48, 32, 16, 24, 18, 12, 32, 32, 32, 24})
}

func riepilogoRDABudgetSheetName(budget RiepilogoRDABudget) string {
	if budget.Budget != "" {
		return budget.Budget
	}
	return riepilogoRDABudgetName(budget)
}

func riepilogoRDABudgetName(budget RiepilogoRDABudget) string {
	if budget.BudgetName != nil && *budget.BudgetName != "" {
		return *budget.BudgetName
	}
	return "Senza budget"
}

func riepilogoRDADetailBudgetName(detail RiepilogoRDADetail) string {
	if detail.BudgetName != nil && *detail.BudgetName != "" {
		return *detail.BudgetName
	}
	return "Senza budget"
}

func riepilogoRDADetailBudgetKey(detail RiepilogoRDADetail) string {
	if detail.BudgetID == nil {
		return "senza-budget"
	}
	return fmt.Sprint(*detail.BudgetID)
}

func riepilogoRDAStateLabel(state *string) string {
	if state == nil || *state == "" {
		return ""
	}

	labels := map[string]string{
		"CLOSED":                            "Chiuso",
		"DELIVERED_AND_COMPLIANT":           "Consegnato e conforme",
		"PENDING_CHECK_DOCUMENT":            "In attesa verifica documenti",
		"PENDING_CONTRACT_VERIFICATION":     "In verifica contratto",
		"PENDING_DISPUTE":                   "In gestione contestazione",
		"PENDING_ERP_SAVE":                  "In attesa registrazione ERP",
		"PENDING_LEASING":                   "In attesa leasing",
		"PENDING_LEASING_ORDER_CREATION":    "Creazione ordine leasing in corso",
		"PENDING_PDF_GENERATION":            "Generazione PDF in corso",
		"PENDING_PROVIDER_SAVED_IN_ALYANTE": "Fornitore da registrare in Alyante",
		"PENDING_SEND":                      "In attesa invio",
		"PENDING_VERIFICATION":              "In verifica",
	}
	if label, ok := labels[*state]; ok {
		return label
	}
	return *state
}

func intPtrValue(v *int) any {
	if v == nil {
		return ""
	}
	return *v
}
