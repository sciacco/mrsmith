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
