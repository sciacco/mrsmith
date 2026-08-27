package training

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/xuri/excelize/v2"
)

func writeTrainingXLSX(w http.ResponseWriter, filename string, headers []string, rows [][]string) error {
	f := excelize.NewFile()
	sheet := "Dati"
	defaultSheet := f.GetSheetName(0)
	if defaultSheet != "" && defaultSheet != sheet {
		f.SetSheetName(defaultSheet, sheet)
	}

	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return err
		}
	}
	for ri, row := range rows {
		for ci, value := range row {
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				return err
			}
		}
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return f.Write(w)
}

func filterCertificationRows(rows []CertificationRow, query map[string]string) []CertificationRow {
	out := make([]CertificationRow, 0, len(rows))
	q := strings.ToLower(strings.TrimSpace(query["q"]))
	for _, row := range rows {
		if query["status"] != "" && row.CurrentStatus != query["status"] {
			continue
		}
		if query["year"] != "" && !strings.HasPrefix(row.AwardedOn, query["year"]) {
			continue
		}
		if q != "" && !containsAny(q, row.EmployeeName, row.EmployeeEmail, row.CertificationCode, row.CertificationName, row.Outcome, row.ValidationSource, row.DocumentFilename) {
			continue
		}
		out = append(out, row)
	}
	return out
}

func containsAny(needle string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}

// economicReportUnknownYearKey e il bucket "anno non disponibile" del client
// (ReportPage.tsx, UNKNOWN_YEAR_KEY): le righe senza anno di competenza
// risolvibile (poError o budgetYear assente).
const economicReportUnknownYearKey = "sconosciuto"

// filterEconomicReportRows applica il filtro per anno di competenza scelto
// nella vista (#163 punto 1) anche all'export, con lo stesso criterio del
// client: BudgetYear del PO vivo, bucket economicReportUnknownYearKey per le
// righe con poError o senza anno. year vuoto = nessun filtro (client "Tutti
// gli anni").
func filterEconomicReportRows(rows []EconomicReportRow, year string) []EconomicReportRow {
	year = strings.TrimSpace(year)
	if year == "" {
		return rows
	}
	out := make([]EconomicReportRow, 0, len(rows))
	for _, row := range rows {
		unavailable := row.POError != "" || row.BudgetYear == 0
		if year == economicReportUnknownYearKey {
			if unavailable {
				out = append(out, row)
			}
			continue
		}
		if !unavailable && fmt.Sprint(row.BudgetYear) == year {
			out = append(out, row)
		}
	}
	return out
}

// economicReportXLSXRows converte le righe del consuntivo economico (#163)
// nel formato tabellare dell'export: stesse colonne del prospetto, incluso
// l'errore PO quando presente.
func economicReportXLSXRows(rows []EconomicReportRow) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, []string{
			row.CourseTitle,
			fmt.Sprint(row.EventCancelled),
			row.CreatedAt,
			fmt.Sprint(row.CoveredEnrollments),
			row.POCode,
			row.Amount,
			row.Currency,
			string(row.EconomicState),
			row.BudgetName,
			fmt.Sprint(row.BudgetYear),
			row.POError,
		})
	}
	return out
}

// deliveredReportXLSXRows converte le righe dell'erogato (#163) nel formato
// tabellare dell'export: i team attivi si appiattiscono in un'unica cella.
func deliveredReportXLSXRows(rows []DeliveredReportRow) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		teamNames := make([]string, 0, len(row.Teams))
		for _, team := range row.Teams {
			teamNames = append(teamNames, team.Name)
		}
		hours := ""
		if row.Hours != nil {
			hours = fmt.Sprint(*row.Hours)
		}
		out = append(out, []string{
			row.EmployeeName,
			strings.Join(teamNames, ", "),
			row.CourseTitle,
			row.SkillAreaName,
			row.DeliveryStatus,
			row.LearningOutcome,
			row.ReferenceDate,
			hours,
		})
	}
	return out
}
