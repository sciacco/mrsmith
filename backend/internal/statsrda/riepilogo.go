package statsrda

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/xuri/excelize/v2"
)

const riepilogoDateLayout = "2006-01-02"

func resolveRiepilogoPeriod(preset string, now time.Time) (from time.Time, to time.Time, normalizedPreset string, err error) {
	normalizedPreset = strings.TrimSpace(preset)
	if normalizedPreset == "" {
		return time.Time{}, time.Time{}, "", errors.New("period obbligatorio")
	}

	loc := now.Location()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	currentQuarterMonth := time.Month(((int(now.Month())-1)/3)*3 + 1)
	startOfQuarter := time.Date(now.Year(), currentQuarterMonth, 1, 0, 0, 0, 0, loc)
	startOfYear := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, loc)

	switch normalizedPreset {
	case "this_month":
		from = startOfMonth
		to = startOfMonth.AddDate(0, 1, 0)
	case "previous_month":
		to = startOfMonth
		from = startOfMonth.AddDate(0, -1, 0)
	case "this_quarter":
		from = startOfQuarter
		to = startOfQuarter.AddDate(0, 3, 0)
	case "previous_quarter":
		to = startOfQuarter
		from = startOfQuarter.AddDate(0, -3, 0)
	case "current_year":
		from = startOfYear
		to = startOfYear.AddDate(1, 0, 0)
	case "previous_year":
		to = startOfYear
		from = startOfYear.AddDate(-1, 0, 0)
	default:
		return time.Time{}, time.Time{}, "", errors.New("period non valido")
	}

	return from, to, normalizedPreset, nil
}

func resolveRiepilogoRequestPeriod(period, fromParam, toParam string, now time.Time) (from time.Time, to time.Time, normalizedPreset string, err error) {
	period = strings.TrimSpace(period)
	fromParam = strings.TrimSpace(fromParam)
	toParam = strings.TrimSpace(toParam)

	if period == "custom" || fromParam != "" || toParam != "" {
		if period != "" && period != "custom" {
			return time.Time{}, time.Time{}, "", errors.New("period non valido")
		}
		if fromParam == "" || toParam == "" {
			return time.Time{}, time.Time{}, "", errors.New("range date obbligatorio")
		}

		loc := now.Location()
		from, err = time.ParseInLocation(riepilogoDateLayout, fromParam, loc)
		if err != nil {
			return time.Time{}, time.Time{}, "", errors.New("data inizio non valida")
		}
		to, err = time.ParseInLocation(riepilogoDateLayout, toParam, loc)
		if err != nil {
			return time.Time{}, time.Time{}, "", errors.New("data fine non valida")
		}
		if !from.Before(to) {
			return time.Time{}, time.Time{}, "", errors.New("la data inizio deve precedere la data fine")
		}

		return from, to, "custom", nil
	}

	return resolveRiepilogoPeriod(period, now)
}

const riepilogoBudgetQuery = `
SELECT
  CASE
    WHEN i.budget_di_riferimento IS NULL OR btrim(i.budget_di_riferimento) = ''
    THEN 'Senza budget'
    ELSE btrim(i.budget_di_riferimento)
  END AS budget,
  COUNT(*) AS order_count,
  COALESCE(SUM(COALESCE(i.importo_totale, 0)), 0) AS amount
FROM pa.issue i
WHERE i.issue_type = 'Acquisto'
  AND i.resolution NOT IN ('Annullato','Rifiutato')
  AND i.created >= $1
  AND i.created < $2
GROUP BY 1
ORDER BY amount DESC, budget ASC`

const riepilogoDetailQuery = `
SELECT
  i.issue_key,
  i.numero_ordine,
  COALESCE(i.summary, '') AS summary,
  CASE
    WHEN i.budget_di_riferimento IS NULL OR btrim(i.budget_di_riferimento) = ''
    THEN 'Senza budget'
    ELSE btrim(i.budget_di_riferimento)
  END AS budget_di_riferimento,
  COALESCE(i.importo_totale, 0) AS importo_totale,
  i.valuta,
  i.reporter_name,
  i.fornitore_selezionato,
  i.status,
  i.resolution,
  to_char(i.created, 'YYYY-MM-DD"T"HH24:MI:SSOF') AS created
FROM pa.issue i
WHERE i.issue_type = 'Acquisto'
  AND i.resolution NOT IN ('Annullato','Rifiutato')
  AND i.created >= $1
  AND i.created < $2
ORDER BY i.created DESC, i.issue_key ASC`

func (h *Handler) handleRiepilogo(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	query := r.URL.Query()
	from, to, preset, err := resolveRiepilogoRequestPeriod(query.Get("period"), query.Get("from"), query.Get("to"), time.Now())
	if err != nil {
		badRequest(w, err.Error())
		return
	}

	resp, err := h.loadRiepilogo(r, from, to, preset)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda riepilogo query failed")
		return
	}

	httputil.JSON(w, http.StatusOK, resp)
}

func (h *Handler) handleRiepilogoExport(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	query := r.URL.Query()
	from, to, preset, err := resolveRiepilogoRequestPeriod(query.Get("period"), query.Get("from"), query.Get("to"), time.Now())
	if err != nil {
		badRequest(w, err.Error())
		return
	}

	resp, err := h.loadRiepilogo(r, from, to, preset)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda riepilogo export query failed")
		return
	}

	buf, err := buildRiepilogoExcel(resp)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda riepilogo export build failed")
		return
	}

	filename := fmt.Sprintf("riepilogo-pa-jira_%s_%s_%s.xlsx", resp.Period.Preset, resp.Period.From, resp.Period.To)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	_, _ = w.Write(buf)
}

func (h *Handler) loadRiepilogo(r *http.Request, from, to time.Time, preset string) (RiepilogoResponse, error) {
	ctx := r.Context()
	resp := RiepilogoResponse{
		Period: RiepilogoPeriod{
			Preset: preset,
			From:   from.Format(riepilogoDateLayout),
			To:     to.Format(riepilogoDateLayout),
		},
		Budgets: []RiepilogoBudget{},
		Details: []RiepilogoDetail{},
	}

	budgetRows, err := h.db.QueryContext(ctx, riepilogoBudgetQuery, from, to)
	if err != nil {
		return resp, err
	}
	defer budgetRows.Close()

	for budgetRows.Next() {
		var b RiepilogoBudget
		if err := budgetRows.Scan(&b.Budget, &b.OrderCount, &b.Amount); err != nil {
			return resp, err
		}
		resp.Totals.Amount += b.Amount
		resp.Budgets = append(resp.Budgets, b)
	}
	if err := budgetRows.Err(); err != nil {
		return resp, err
	}

	detailRows, err := h.db.QueryContext(ctx, riepilogoDetailQuery, from, to)
	if err != nil {
		return resp, err
	}
	defer detailRows.Close()

	for detailRows.Next() {
		var d RiepilogoDetail
		var numero, valuta, reporter, fornitore, status, resolution, created nullString
		if err := detailRows.Scan(
			&d.IssueKey, &numero, &d.Summary, &d.BudgetDiRiferimento, &d.ImportoTotale,
			&valuta, &reporter, &fornitore, &status, &resolution, &created,
		); err != nil {
			return resp, err
		}
		d.NumeroOrdine = numero.ptr()
		d.Valuta = valuta.ptr()
		d.ReporterName = reporter.ptr()
		d.FornitoreSelezionato = fornitore.ptr()
		d.Status = status.ptr()
		d.Resolution = resolution.ptr()
		d.Created = created.ptr()
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

func buildRiepilogoExcel(resp RiepilogoResponse) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	const totalsSheet = "Totali budget"
	if err := f.SetSheetName("Sheet1", totalsSheet); err != nil {
		return nil, err
	}

	if err := writeRiepilogoTotalsSheet(f, totalsSheet, resp.Budgets); err != nil {
		return nil, err
	}

	detailsByBudget := make(map[string][]RiepilogoDetail, len(resp.Budgets))
	for _, detail := range resp.Details {
		detailsByBudget[detail.BudgetDiRiferimento] = append(detailsByBudget[detail.BudgetDiRiferimento], detail)
	}

	usedSheetNames := map[string]bool{canonicalRiepilogoSheetName(totalsSheet): true}
	for _, budget := range resp.Budgets {
		sheet := uniqueRiepilogoSheetName(budget.Budget, usedSheetNames)
		if _, err := f.NewSheet(sheet); err != nil {
			return nil, err
		}
		if err := writeRiepilogoDetailSheet(f, sheet, detailsByBudget[budget.Budget]); err != nil {
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

func writeRiepilogoTotalsSheet(f *excelize.File, sheet string, budgets []RiepilogoBudget) error {
	headers := []string{"Budget", "Numero ordini", "Importo totale", "Valuta", "Percentuale sul totale periodo"}
	if err := writeRiepilogoHeaderRow(f, sheet, headers); err != nil {
		return err
	}

	for i, budget := range budgets {
		row := i + 2
		values := []any{
			budget.Budget,
			budget.OrderCount,
			budget.Amount,
			"Importi aggregati senza conversione valuta",
			budget.Percentage,
		}
		if err := writeRiepilogoRow(f, sheet, row, values); err != nil {
			return err
		}
	}

	return setRiepilogoColumnWidths(f, sheet, []float64{32, 16, 18, 42, 30})
}

func writeRiepilogoDetailSheet(f *excelize.File, sheet string, details []RiepilogoDetail) error {
	headers := []string{
		"Issue key", "Numero ordine", "Summary", "Budget di riferimento", "Importo totale", "Valuta",
		"Richiedente", "Fornitore selezionato", "Status", "Resolution", "Created",
	}
	if err := writeRiepilogoHeaderRow(f, sheet, headers); err != nil {
		return err
	}

	for i, detail := range details {
		row := i + 2
		values := []any{
			detail.IssueKey,
			stringPtrValue(detail.NumeroOrdine),
			detail.Summary,
			detail.BudgetDiRiferimento,
			detail.ImportoTotale,
			stringPtrValue(detail.Valuta),
			stringPtrValue(detail.ReporterName),
			stringPtrValue(detail.FornitoreSelezionato),
			stringPtrValue(detail.Status),
			stringPtrValue(detail.Resolution),
			stringPtrValue(detail.Created),
		}
		if err := writeRiepilogoRow(f, sheet, row, values); err != nil {
			return err
		}
	}

	return setRiepilogoColumnWidths(f, sheet, []float64{16, 18, 48, 28, 18, 12, 24, 32, 18, 18, 24})
}

func writeRiepilogoHeaderRow(f *excelize.File, sheet string, headers []string) error {
	for i, header := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return err
		}
	}
	return nil
}

func writeRiepilogoRow(f *excelize.File, sheet string, row int, values []any) error {
	for i, value := range values {
		cell, err := excelize.CoordinatesToCellName(i+1, row)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, cell, value); err != nil {
			return err
		}
	}
	return nil
}

func setRiepilogoColumnWidths(f *excelize.File, sheet string, widths []float64) error {
	for i, width := range widths {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		if err := f.SetColWidth(sheet, col, col, width); err != nil {
			return err
		}
	}
	return nil
}

func uniqueRiepilogoSheetName(budget string, used map[string]bool) string {
	base := normalizeRiepilogoSheetName(budget)
	key := canonicalRiepilogoSheetName(base)
	if !used[key] {
		used[key] = true
		return base
	}

	for n := 2; ; n++ {
		suffix := fmt.Sprintf(" %d", n)
		name := truncateRunes(base, 31-len([]rune(suffix))) + suffix
		key := canonicalRiepilogoSheetName(name)
		if !used[key] {
			used[key] = true
			return name
		}
	}
}

func normalizeRiepilogoSheetName(name string) string {
	replacer := strings.NewReplacer(":", "", "\\", "", "/", "", "?", "", "*", "", "[", "", "]", "")
	name = strings.TrimSpace(strings.Trim(strings.TrimSpace(replacer.Replace(name)), "'"))
	if name == "" {
		name = "Senza budget"
	}
	name = truncateRunes(name, 31)
	name = strings.TrimSpace(strings.Trim(name, "'"))
	if name == "" {
		name = "Senza budget"
	}
	return name
}

func canonicalRiepilogoSheetName(name string) string {
	return strings.ToLower(name)
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func stringPtrValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
