package training

import (
	"fmt"
	"net/http"
	"strconv"
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

func filterPlanRows(rows []PlanEnrollment, query map[string]string) []PlanEnrollment {
	out := make([]PlanEnrollment, 0, len(rows))
	q := strings.ToLower(strings.TrimSpace(query["q"]))
	for _, row := range rows {
		if query["team"] != "" && row.TeamCode != query["team"] {
			continue
		}
		if query["status"] != "" && row.Status != query["status"] {
			continue
		}
		if query["year"] != "" && strconv.Itoa(row.Year) != query["year"] {
			continue
		}
		if q != "" && !containsAny(q, row.EmployeeName, row.EmployeeEmail, row.CourseTitle, row.VendorName, row.SkillAreaName) {
			continue
		}
		out = append(out, row)
	}
	return out
}

func filterRequestRows(rows []TrainingRequest, query map[string]string) []TrainingRequest {
	out := make([]TrainingRequest, 0, len(rows))
	q := strings.ToLower(strings.TrimSpace(query["q"]))
	for _, row := range rows {
		if query["status"] != "" && row.Status != query["status"] {
			continue
		}
		if q != "" && !containsAny(q, row.EmployeeName, row.EmployeeEmail, row.CourseTitle, row.FreeTextTitle, row.SkillAreaName, row.Motivation) {
			continue
		}
		out = append(out, row)
	}
	return out
}

func filterCatalogRows(rows []CatalogCourse, query map[string]string) []CatalogCourse {
	out := make([]CatalogCourse, 0, len(rows))
	q := strings.ToLower(strings.TrimSpace(query["q"]))
	for _, row := range rows {
		if query["status"] == "mandatory" && !row.ComplianceRelated {
			continue
		}
		if query["status"] == "active" && !row.Active {
			continue
		}
		if q != "" && !containsAny(q, row.Title, row.VendorName, row.SkillAreaName, row.CertificationName, row.ComplianceFramework) {
			continue
		}
		out = append(out, row)
	}
	return out
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

func intString(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func floatString(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', 2, 64)
}
