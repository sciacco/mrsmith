package compliance

import (
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/xuri/excelize/v2"
)

func writeCSV(w http.ResponseWriter, filename string, headers []string, rows [][]string) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	cw := csv.NewWriter(w)
	cw.Write(headers)
	for _, row := range rows {
		cw.Write(row)
	}
	cw.Flush()
}

func writeXLSX(w http.ResponseWriter, filename string, headers []string, rows [][]string) {
	f := excelize.NewFile()
	sheet := "Sheet1"

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	for ri, row := range rows {
		for ci, val := range row {
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
			f.SetCellValue(sheet, cell, val)
		}
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	f.Write(w)
}
