package afctools

import (
	"net/http"
	"strconv"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// EnergiaColoPivotRow: one row per customer, twelve monthly sums split by unit (ampere and kw).
type EnergiaColoPivotRow struct {
	Customer     *string  `json:"customer"`
	GennaioA     *float64 `json:"gennaio_a"`
	GennaioKw    *float64 `json:"gennaio_kw"`
	FebbraioA    *float64 `json:"febbraio_a"`
	FebbraioKw   *float64 `json:"febbraio_kw"`
	MarzoA       *float64 `json:"marzo_a"`
	MarzoKw      *float64 `json:"marzo_kw"`
	AprileA      *float64 `json:"aprile_a"`
	AprileKw     *float64 `json:"aprile_kw"`
	MaggioA      *float64 `json:"maggio_a"`
	MaggioKw     *float64 `json:"maggio_kw"`
	GiugnoA      *float64 `json:"giugno_a"`
	GiugnoKw     *float64 `json:"giugno_kw"`
	LuglioA      *float64 `json:"luglio_a"`
	LuglioKw     *float64 `json:"luglio_kw"`
	AgostoA      *float64 `json:"agosto_a"`
	AgostoKw     *float64 `json:"agosto_kw"`
	SettembreA   *float64 `json:"settembre_a"`
	SettembreKw  *float64 `json:"settembre_kw"`
	OttobreA     *float64 `json:"ottobre_a"`
	OttobreKw    *float64 `json:"ottobre_kw"`
	NovembreA    *float64 `json:"novembre_a"`
	NovembreKw   *float64 `json:"novembre_kw"`
	DicembreA    *float64 `json:"dicembre_a"`
	DicembreKw   *float64 `json:"dicembre_kw"`
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
SELECT c.intestazione AS customer,
       SUM(CASE WHEN MONTH(i.start_period) = 1  THEN i.ampere ELSE 0 END) AS gennaio_a,
       SUM(CASE WHEN MONTH(i.start_period) = 1  THEN i.kw     ELSE 0 END) AS gennaio_kw,
       SUM(CASE WHEN MONTH(i.start_period) = 2  THEN i.ampere ELSE 0 END) AS febbraio_a,
       SUM(CASE WHEN MONTH(i.start_period) = 2  THEN i.kw     ELSE 0 END) AS febbraio_kw,
       SUM(CASE WHEN MONTH(i.start_period) = 3  THEN i.ampere ELSE 0 END) AS marzo_a,
       SUM(CASE WHEN MONTH(i.start_period) = 3  THEN i.kw     ELSE 0 END) AS marzo_kw,
       SUM(CASE WHEN MONTH(i.start_period) = 4  THEN i.ampere ELSE 0 END) AS aprile_a,
       SUM(CASE WHEN MONTH(i.start_period) = 4  THEN i.kw     ELSE 0 END) AS aprile_kw,
       SUM(CASE WHEN MONTH(i.start_period) = 5  THEN i.ampere ELSE 0 END) AS maggio_a,
       SUM(CASE WHEN MONTH(i.start_period) = 5  THEN i.kw     ELSE 0 END) AS maggio_kw,
       SUM(CASE WHEN MONTH(i.start_period) = 6  THEN i.ampere ELSE 0 END) AS giugno_a,
       SUM(CASE WHEN MONTH(i.start_period) = 6  THEN i.kw     ELSE 0 END) AS giugno_kw,
       SUM(CASE WHEN MONTH(i.start_period) = 7  THEN i.ampere ELSE 0 END) AS luglio_a,
       SUM(CASE WHEN MONTH(i.start_period) = 7  THEN i.kw     ELSE 0 END) AS luglio_kw,
       SUM(CASE WHEN MONTH(i.start_period) = 8  THEN i.ampere ELSE 0 END) AS agosto_a,
       SUM(CASE WHEN MONTH(i.start_period) = 8  THEN i.kw     ELSE 0 END) AS agosto_kw,
       SUM(CASE WHEN MONTH(i.start_period) = 9  THEN i.ampere ELSE 0 END) AS settembre_a,
       SUM(CASE WHEN MONTH(i.start_period) = 9  THEN i.kw     ELSE 0 END) AS settembre_kw,
       SUM(CASE WHEN MONTH(i.start_period) = 10 THEN i.ampere ELSE 0 END) AS ottobre_a,
       SUM(CASE WHEN MONTH(i.start_period) = 10 THEN i.kw     ELSE 0 END) AS ottobre_kw,
       SUM(CASE WHEN MONTH(i.start_period) = 11 THEN i.ampere ELSE 0 END) AS novembre_a,
       SUM(CASE WHEN MONTH(i.start_period) = 11 THEN i.kw     ELSE 0 END) AS novembre_kw,
       SUM(CASE WHEN MONTH(i.start_period) = 12 THEN i.ampere ELSE 0 END) AS dicembre_a,
       SUM(CASE WHEN MONTH(i.start_period) = 12 THEN i.kw     ELSE 0 END) AS dicembre_kw
FROM importi_corrente_colocation AS i
JOIN cli_fatturazione AS c ON c.id = i.customer_id
WHERE YEAR(i.start_period) = ?
GROUP BY c.intestazione
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
			&p.Customer,
			&p.GennaioA, &p.GennaioKw,
			&p.FebbraioA, &p.FebbraioKw,
			&p.MarzoA, &p.MarzoKw,
			&p.AprileA, &p.AprileKw,
			&p.MaggioA, &p.MaggioKw,
			&p.GiugnoA, &p.GiugnoKw,
			&p.LuglioA, &p.LuglioKw,
			&p.AgostoA, &p.AgostoKw,
			&p.SettembreA, &p.SettembreKw,
			&p.OttobreA, &p.OttobreKw,
			&p.NovembreA, &p.NovembreKw,
			&p.DicembreA, &p.DicembreKw,
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
       IF(i.tipo_variabile = 2, i.kw, i.ampere) AS consumo,
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
