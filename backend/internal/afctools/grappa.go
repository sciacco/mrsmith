package afctools

import (
	"net/http"
	"strconv"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// EnergiaColoPivotRow: one row per customer, twelve monthly sums.
type EnergiaColoPivotRow struct {
	Customer *string  `json:"customer"`
	Gennaio  *float64 `json:"gennaio"`
	Febbraio *float64 `json:"febbraio"`
	Marzo    *float64 `json:"marzo"`
	Aprile   *float64 `json:"aprile"`
	Maggio   *float64 `json:"maggio"`
	Giugno   *float64 `json:"giugno"`
	Luglio   *float64 `json:"luglio"`
	Agosto   *float64 `json:"agosto"`
	Settembre *float64 `json:"settembre"`
	Ottobre  *float64 `json:"ottobre"`
	Novembre *float64 `json:"novembre"`
	Dicembre *float64 `json:"dicembre"`
}

// EnergiaColoDetailRow mirrors Q_select_consumi_colo output verbatim.
type EnergiaColoDetailRow struct {
	Customer        *string    `json:"customer"`
	StartPeriod     *time.Time `json:"start_period"`
	EndPeriod       *time.Time `json:"end_period"`
	Consumo         *float64   `json:"consumo"`
	Amount          *float64   `json:"amount"`
	Pun             *float64   `json:"pun"`
	Coefficiente    *float64   `json:"coefficiente"`
	FissoCU         *float64   `json:"fisso_cu"`
	Eccedenti       *float64   `json:"eccedenti"`
	ImportoEccedenti *float64  `json:"importo_eccedenti"`
	TipoVariabile   *string    `json:"tipo_variabile"`
}

func parseYear(s string) (int, bool) {
	if len(s) != 4 {
		return 0, false
	}
	y, err := strconv.Atoi(s)
	if err != nil || y < 1900 || y > 2100 {
		return 0, false
	}
	return y, true
}

func (h *Handler) listEnergiaColoPivot(r *http.Request, year int) ([]EnergiaColoPivotRow, error) {
	const query = `
SELECT customer,
       SUM(January)   AS Gennaio,
       SUM(February)  AS Febbraio,
       SUM(March)     AS Marzo,
       SUM(April)     AS Aprile,
       SUM(May)       AS Maggio,
       SUM(June)      AS Giugno,
       SUM(July)      AS Luglio,
       SUM(August)    AS Agosto,
       SUM(September) AS Settembre,
       SUM(October)   AS Ottobre,
       SUM(November)  AS Novembre,
       SUM(December)  AS Dicembre
FROM (
    SELECT c.intestazione AS customer,
           CASE WHEN MONTH(i.start_period) = 1  THEN IF(i.ampere > 0, i.ampere, i.Kw) ELSE 0 END AS January,
           CASE WHEN MONTH(i.start_period) = 2  THEN IF(i.ampere > 0, i.ampere, i.Kw) ELSE 0 END AS February,
           CASE WHEN MONTH(i.start_period) = 3  THEN IF(i.ampere > 0, i.ampere, i.Kw) ELSE 0 END AS March,
           CASE WHEN MONTH(i.start_period) = 4  THEN IF(i.ampere > 0, i.ampere, i.Kw) ELSE 0 END AS April,
           CASE WHEN MONTH(i.start_period) = 5  THEN IF(i.ampere > 0, i.ampere, i.Kw) ELSE 0 END AS May,
           CASE WHEN MONTH(i.start_period) = 6  THEN IF(i.ampere > 0, i.ampere, i.Kw) ELSE 0 END AS June,
           CASE WHEN MONTH(i.start_period) = 7  THEN IF(i.ampere > 0, i.ampere, i.Kw) ELSE 0 END AS July,
           CASE WHEN MONTH(i.start_period) = 8  THEN IF(i.ampere > 0, i.ampere, i.Kw) ELSE 0 END AS August,
           CASE WHEN MONTH(i.start_period) = 9  THEN IF(i.ampere > 0, i.ampere, i.Kw) ELSE 0 END AS September,
           CASE WHEN MONTH(i.start_period) = 10 THEN IF(i.ampere > 0, i.ampere, i.Kw) ELSE 0 END AS October,
           CASE WHEN MONTH(i.start_period) = 11 THEN IF(i.ampere > 0, i.ampere, i.Kw) ELSE 0 END AS November,
           CASE WHEN MONTH(i.start_period) = 12 THEN IF(i.ampere > 0, i.ampere, i.Kw) ELSE 0 END AS December
    FROM importi_corrente_colocation AS i
    JOIN cli_fatturazione AS c ON c.id = i.customer_id
    WHERE YEAR(i.start_period) = ?
    GROUP BY c.intestazione, i.start_period
) AS A2
GROUP BY customer
`
	rows, err := h.deps.Grappa.QueryContext(r.Context(), query, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]EnergiaColoPivotRow, 0)
	for rows.Next() {
		var p EnergiaColoPivotRow
		if err := rows.Scan(
			&p.Customer, &p.Gennaio, &p.Febbraio, &p.Marzo, &p.Aprile,
			&p.Maggio, &p.Giugno, &p.Luglio, &p.Agosto,
			&p.Settembre, &p.Ottobre, &p.Novembre, &p.Dicembre,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Handler) listEnergiaColoDetail(r *http.Request, year int) ([]EnergiaColoDetailRow, error) {
	const query = `
SELECT c.intestazione AS customer,
       i.start_period,
       i.end_period,
       IF(i.ampere > 0, i.ampere, i.Kw) AS consumo,
       i.amount,
       i.pun,
       i.coefficiente,
       i.fisso_cu,
       i.eccedenti,
       i.importo_eccedenti,
       i.tipo_variabile
FROM importi_corrente_colocation AS i
JOIN cli_fatturazione AS c ON c.id = i.customer_id
WHERE YEAR(i.start_period) = ?
`
	rows, err := h.deps.Grappa.QueryContext(r.Context(), query, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]EnergiaColoDetailRow, 0)
	for rows.Next() {
		var d EnergiaColoDetailRow
		if err := rows.Scan(
			&d.Customer, &d.StartPeriod, &d.EndPeriod, &d.Consumo,
			&d.Amount, &d.Pun, &d.Coefficiente, &d.FissoCU,
			&d.Eccedenti, &d.ImportoEccedenti, &d.TipoVariabile,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Handler) handleEnergiaColoPivot(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w, h.deps.Grappa, "grappa") {
		return
	}
	year, ok := parseYear(r.URL.Query().Get("year"))
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_year")
		return
	}
	rowsOut, err := h.listEnergiaColoPivot(r, year)
	if err != nil {
		h.dbFailure(w, r, "energia_colo_pivot", err)
		return
	}
	httputil.JSON(w, http.StatusOK, rowsOut)
}

func (h *Handler) handleEnergiaColoDetail(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w, h.deps.Grappa, "grappa") {
		return
	}
	year, ok := parseYear(r.URL.Query().Get("year"))
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_year")
		return
	}
	rowsOut, err := h.listEnergiaColoDetail(r, year)
	if err != nil {
		h.dbFailure(w, r, "energia_colo_detail", err)
		return
	}
	httputil.JSON(w, http.StatusOK, rowsOut)
}
