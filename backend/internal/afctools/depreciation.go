package afctools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/xuri/excelize/v2"
)

// depreciationUpstreamPath is the Mistra NG Internal API report endpoint.
const depreciationUpstreamPath = "/erp/v1/report/depreciation-of-fixed-assets"

// depreciationCompany pins the report to ditta 1. The UI never exposes it.
const depreciationCompany = 1

// DepreciationRow is one fixed-asset depreciation register line for a year.
// Percentages and amounts are decoded from the string form documented by the
// upstream spec; date-times are reduced to their civil day (YYYY-MM-DD).
type DepreciationRow struct {
	Company                     int64    `json:"company"`
	Group                       int64    `json:"group"`
	Category                    *string  `json:"category"`
	Subcategory                 int64    `json:"subcategory"`
	SubcategoryDescription      *string  `json:"subcategory_description"`
	Progress                    int64    `json:"progress"`
	AssetCode                   int64    `json:"asset_code"`
	FixedAssetCode              int64    `json:"fixed_asset_code"`
	FixedAssetDescription       *string  `json:"fixed_asset_description"`
	FixedAssetAccount           *string  `json:"fixed_asset_account"`
	DepreciationFundAccount     *string  `json:"depreciation_fund_account"`
	DepreciationExpenseAccount  *string  `json:"depreciation_expense_account"`
	SerialNumber                *string  `json:"serial_number"`
	Description                 *string  `json:"description"`
	StatutoryDepreciationRate   *float64 `json:"statutory_depreciation_rate"`
	StatutoryDepreciationAmount *float64 `json:"statutory_depreciation_amount"`
	PurchaseYear                *int64   `json:"purchase_year"`
	SaleYear                    *int64   `json:"sale_year"`
	HistoricalCost              *float64 `json:"historical_cost"`
	PriorDepreciation           *float64 `json:"prior_depreciation"`
	OrdinaryDepreciation        *float64 `json:"ordinary_depreciation"`
	InitialResidualValue        *float64 `json:"initial_residual_value"`
	CurrentYearDepreciation     *float64 `json:"current_year_depreciation"`
	ResidualToDepreciate        *float64 `json:"residual_to_depreciate"`
	SaleAmount                  *float64 `json:"sale_amount"`
	ActivationDate              *string  `json:"activation_date"`
	DeactivationDate            *string  `json:"deactivation_date"`
	Notes                       *string  `json:"notes"`
}

// depreciationEnvelope mirrors the upstream paginated response. A single
// call with disable_pagination=true fills Items with every row.
type depreciationEnvelope struct {
	Items []depreciationWireItem `json:"items"`
}

// depreciationWireItem is the raw upstream shape: percentages and amounts are
// documented as strings with "." as decimal separator, and integers may also
// arrive as strings. Custom decoders accept both forms.
type depreciationWireItem struct {
	Company                     depreciationFlexInt   `json:"company"`
	Group                       depreciationFlexInt   `json:"group"`
	Category                    *string               `json:"category"`
	Subcategory                 depreciationFlexInt   `json:"subcategory"`
	SubcategoryDescription      *string               `json:"subcategory_description"`
	Progress                    depreciationFlexInt   `json:"progress"`
	AssetCode                   depreciationFlexInt   `json:"asset_code"`
	FixedAssetCode              depreciationFlexInt   `json:"fixed_asset_code"`
	FixedAssetDescription       *string               `json:"fixed_asset_description"`
	FixedAssetAccount           *string               `json:"fixed_asset_account"`
	DepreciationFundAccount     *string               `json:"depreciation_fund_account"`
	DepreciationExpenseAccount  *string               `json:"depreciation_expense_account"`
	SerialNumber                *string               `json:"serial_number"`
	Description                 *string               `json:"description"`
	StatutoryDepreciationRate   depreciationFlexFloat `json:"statutory_depreciation_rate"`
	StatutoryDepreciationAmount depreciationFlexFloat `json:"statutory_depreciation_amount"`
	PurchaseYear                depreciationFlexInt   `json:"purchase_year"`
	SaleYear                    depreciationFlexInt   `json:"sale_year"`
	HistoricalCost              depreciationFlexFloat `json:"historical_cost"`
	PriorDepreciation           depreciationFlexFloat `json:"prior_depreciation"`
	OrdinaryDepreciation        depreciationFlexFloat `json:"ordinary_depreciation"`
	InitialResidualValue        depreciationFlexFloat `json:"initial_residual_value"`
	CurrentYearDepreciation     depreciationFlexFloat `json:"current_year_depreciation"`
	ResidualToDepreciate        depreciationFlexFloat `json:"residual_to_depreciate"`
	SaleAmount                  depreciationFlexFloat `json:"sale_amount"`
	ActivationDate              *string               `json:"activation_date"`
	DeactivationDate            *string               `json:"deactivation_date"`
	Notes                       *string               `json:"notes"`
}

// depreciationFlexFloat decodes a JSON number or numeric string. Empty and null
// values leave set=false so they map to nil instead of a misleading zero.
type depreciationFlexFloat struct {
	value float64
	set   bool
}

func (f *depreciationFlexFloat) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		return nil
	}
	raw = strings.Trim(raw, `"`)
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fmt.Errorf("invalid numeric value %q: %w", raw, err)
	}
	f.value = value
	f.set = true
	return nil
}

// depreciationFlexInt decodes a JSON integer or integer string. Empty and null
// values leave set=false so they map to nil instead of a misleading zero.
type depreciationFlexInt struct {
	value int64
	set   bool
}

func (i *depreciationFlexInt) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		return nil
	}
	raw = strings.Trim(raw, `"`)
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid integer value %q: %w", raw, err)
	}
	i.value = value
	i.set = true
	return nil
}

// depreciationYear resolves the ?year= parameter. Absent means the current
// year; anything else must be a four-digit year.
func depreciationYear(r *http.Request) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("year"))
	if raw == "" {
		return time.Now().Year(), true
	}
	return parseYear(raw)
}

func (h *Handler) handleDepreciation(w http.ResponseWriter, r *http.Request) {
	year, ok := depreciationYear(r)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_year")
		return
	}

	rows, ok := h.loadDepreciationRows(w, r, year)
	if !ok {
		return
	}

	httputil.JSON(w, http.StatusOK, rows)
}

func (h *Handler) handleDepreciationExport(w http.ResponseWriter, r *http.Request) {
	year, ok := depreciationYear(r)
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_year")
		return
	}

	rows, ok := h.loadDepreciationRows(w, r, year)
	if !ok {
		return
	}

	buf, err := buildDepreciationExcel(rows)
	if err != nil {
		httputil.InternalError(w, r, err, "depreciation export build failed",
			"component", "afctools", "operation", "depreciation_export", "year", year)
		return
	}

	filename := fmt.Sprintf("ammortamenti-cespiti_%d.xlsx", year)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf)
}

// loadDepreciationRows performs the single upstream call and, on failure,
// writes the error response itself and reports ok=false.
func (h *Handler) loadDepreciationRows(w http.ResponseWriter, r *http.Request, year int) ([]DepreciationRow, bool) {
	if h.deps.Arak == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "arak_gateway_not_configured")
		return nil, false
	}

	query := url.Values{}
	query.Set("page_number", "1")
	query.Set("disable_pagination", "true")
	query.Set("year", strconv.Itoa(year))
	query.Set("company", strconv.Itoa(depreciationCompany))

	resp, err := h.deps.Arak.Do(http.MethodGet, depreciationUpstreamPath, query.Encode(), nil)
	if err != nil {
		httputil.InternalError(w, r, err, "depreciation upstream request failed",
			"component", "afctools", "operation", "depreciation", "year", year)
		return nil, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		httputil.InternalError(w, r, err, "depreciation upstream response read failed",
			"component", "afctools", "operation", "depreciation", "year", year)
		return nil, false
	}

	if resp.StatusCode >= http.StatusBadRequest {
		logging.FromContext(r.Context()).Warn("depreciation upstream returned error",
			"component", "afctools", "operation", "depreciation", "year", year,
			"upstream_status", resp.StatusCode, "upstream_body", compactGatewayPDFBody(body))
		httputil.Error(w, resp.StatusCode, "gateway_error")
		return nil, false
	}

	var envelope depreciationEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		httputil.InternalError(w, r, fmt.Errorf("decode depreciation upstream response: %w", err),
			"depreciation upstream decode failed",
			"component", "afctools", "operation", "depreciation", "year", year)
		return nil, false
	}

	rows := make([]DepreciationRow, 0, len(envelope.Items))
	for _, item := range envelope.Items {
		rows = append(rows, depreciationRowFromWire(item))
	}
	return rows, true
}

func depreciationRowFromWire(item depreciationWireItem) DepreciationRow {
	return DepreciationRow{
		Company:                     depreciationIntValue(item.Company),
		Group:                       depreciationIntValue(item.Group),
		Category:                    item.Category,
		Subcategory:                 depreciationIntValue(item.Subcategory),
		SubcategoryDescription:      item.SubcategoryDescription,
		Progress:                    depreciationIntValue(item.Progress),
		AssetCode:                   depreciationIntValue(item.AssetCode),
		FixedAssetCode:              depreciationIntValue(item.FixedAssetCode),
		FixedAssetDescription:       item.FixedAssetDescription,
		FixedAssetAccount:           item.FixedAssetAccount,
		DepreciationFundAccount:     item.DepreciationFundAccount,
		DepreciationExpenseAccount:  item.DepreciationExpenseAccount,
		SerialNumber:                item.SerialNumber,
		Description:                 item.Description,
		StatutoryDepreciationRate:   depreciationFloatPtr(item.StatutoryDepreciationRate),
		StatutoryDepreciationAmount: depreciationFloatPtr(item.StatutoryDepreciationAmount),
		PurchaseYear:                depreciationIntPtr(item.PurchaseYear),
		SaleYear:                    depreciationIntPtr(item.SaleYear),
		HistoricalCost:              depreciationFloatPtr(item.HistoricalCost),
		PriorDepreciation:           depreciationFloatPtr(item.PriorDepreciation),
		OrdinaryDepreciation:        depreciationFloatPtr(item.OrdinaryDepreciation),
		InitialResidualValue:        depreciationFloatPtr(item.InitialResidualValue),
		CurrentYearDepreciation:     depreciationFloatPtr(item.CurrentYearDepreciation),
		ResidualToDepreciate:        depreciationFloatPtr(item.ResidualToDepreciate),
		SaleAmount:                  depreciationFloatPtr(item.SaleAmount),
		ActivationDate:              depreciationDateOnly(item.ActivationDate),
		DeactivationDate:            depreciationDateOnly(item.DeactivationDate),
		Notes:                       item.Notes,
	}
}

func depreciationIntValue(v depreciationFlexInt) int64 {
	if !v.set {
		return 0
	}
	return v.value
}

func depreciationIntPtr(v depreciationFlexInt) *int64 {
	if !v.set {
		return nil
	}
	value := v.value
	return &value
}

func depreciationFloatPtr(v depreciationFlexFloat) *float64 {
	if !v.set {
		return nil
	}
	value := v.value
	return &value
}

// depreciationDateOnly keeps the civil day of a date-time value.
func depreciationDateOnly(raw *string) *string {
	if raw == nil {
		return nil
	}
	value := strings.TrimSpace(*raw)
	if len(value) >= 10 {
		candidate := value[:10]
		if _, err := time.Parse("2006-01-02", candidate); err == nil {
			return &candidate
		}
	}
	if value == "" {
		return nil
	}
	return &value
}

// depreciationExportHeaders are the Italian column labels, in endpoint order.
var depreciationExportHeaders = []string{
	"Ditta", "Gruppo", "Categoria", "Sottocategoria", "Descrizione sottocategoria", "Progressivo",
	"Codice cespite", "Codice immobilizzo", "Descrizione immobilizzo", "Conto immobilizzo",
	"Conto fondo ammortamento", "Conto costo ammortamento", "Matricola", "Descrizione",
	"% ammortamento civilistico", "Quota ammortamento civilistico", "Anno acquisto",
	"Anno vendita", "Costo storico", "Ammortamento precedente", "Ammortamento ordinario",
	"Residuo iniziale", "Quota anno in corso", "Residuo da ammortizzare", "Vendita",
	"Data attivazione", "Data disattivazione", "Note",
}

func buildDepreciationExcel(rows []DepreciationRow) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "Ammortamenti"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}

	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}

	for i, header := range depreciationExportHeaders {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return nil, err
		}
		if err := f.SetCellStyle(sheet, cell, cell, headerStyle); err != nil {
			return nil, err
		}
	}
	if err := f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}

	for r, row := range rows {
		values := depreciationExportRow(row)
		for c, value := range values {
			cell, err := excelize.CoordinatesToCellName(c+1, r+2)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// depreciationExportRow returns the 28 cell values in endpoint column order.
// Numeric fields stay numeric so Excel treats amounts and percentages as
// numbers; missing values become empty cells.
func depreciationExportRow(row DepreciationRow) []any {
	return []any{
		row.Company,
		row.Group,
		depreciationStringValue(row.Category),
		row.Subcategory,
		depreciationStringValue(row.SubcategoryDescription),
		row.Progress,
		row.AssetCode,
		row.FixedAssetCode,
		depreciationStringValue(row.FixedAssetDescription),
		depreciationStringValue(row.FixedAssetAccount),
		depreciationStringValue(row.DepreciationFundAccount),
		depreciationStringValue(row.DepreciationExpenseAccount),
		depreciationStringValue(row.SerialNumber),
		depreciationStringValue(row.Description),
		depreciationFloatValue(row.StatutoryDepreciationRate),
		depreciationFloatValue(row.StatutoryDepreciationAmount),
		depreciationIntCellValue(row.PurchaseYear),
		depreciationIntCellValue(row.SaleYear),
		depreciationFloatValue(row.HistoricalCost),
		depreciationFloatValue(row.PriorDepreciation),
		depreciationFloatValue(row.OrdinaryDepreciation),
		depreciationFloatValue(row.InitialResidualValue),
		depreciationFloatValue(row.CurrentYearDepreciation),
		depreciationFloatValue(row.ResidualToDepreciate),
		depreciationFloatValue(row.SaleAmount),
		depreciationStringValue(row.ActivationDate),
		depreciationStringValue(row.DeactivationDate),
		depreciationStringValue(row.Notes),
	}
}

func depreciationStringValue(v *string) any {
	if v == nil {
		return ""
	}
	return *v
}

func depreciationFloatValue(v *float64) any {
	if v == nil {
		return ""
	}
	return *v
}

func depreciationIntCellValue(v *int64) any {
	if v == nil {
		return ""
	}
	return *v
}
