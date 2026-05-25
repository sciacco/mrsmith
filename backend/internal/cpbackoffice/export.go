package cpbackoffice

import (
	"encoding/json"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/xuri/excelize/v2"
)

// ExportRequest models the headers and string grid received from the frontend
type ExportRequest struct {
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

// handleExportCustomers parses a custom export payload and streams a formatted XLSX spreadsheet
func handleExportCustomers(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "export_customers"

		var req ExportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_request_body")
			return
		}

		if len(req.Headers) == 0 {
			httputil.Error(w, http.StatusBadRequest, "headers_required")
			return
		}

		f := excelize.NewFile()
		defer func() {
			_ = f.Close()
		}()

		sheet := "Sheet1"

		// Write headers in row 1
		for i, h := range req.Headers {
			cell, err := excelize.CoordinatesToCellName(i+1, 1)
			if err == nil {
				_ = f.SetCellValue(sheet, cell, h)
			}
		}

		// Write data starting from row 2
		for ri, row := range req.Rows {
			for ci, val := range row {
				cell, err := excelize.CoordinatesToCellName(ci+1, ri+2)
				if err == nil {
					_ = f.SetCellValue(sheet, cell, val)
				}
			}
		}

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="stato_aziende.xlsx"`)

		if err := f.Write(w); err != nil {
			deps.Logger.Error("failed to write xlsx export response",
				"error", err,
				"component", "cpbackoffice",
				"operation", op,
			)
		}
	}
}
