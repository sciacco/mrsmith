package reports

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type activeLinesRequest struct {
	ConnectionTypes []string `json:"connectionTypes"`
	Statuses        []string `json:"statuses"`
}

type activeLineRow struct {
	RagioneSociale     string   `json:"ragione_sociale"`
	TipoConn           *string  `json:"tipo_conn"`
	Fornitore          *string  `json:"fornitore"`
	Provincia          *string  `json:"provincia"`
	Comune             *string  `json:"comune"`
	Tipo               *string  `json:"tipo"`
	ProfiloCommerciale *string  `json:"profilo_commerciale"`
	Macro              *string  `json:"macro"`
	Intestatario       *string  `json:"intestatario"`
	Ordine             *string  `json:"ordine"`
	FatturatoFinoAl    *string  `json:"fatturato_fino_al"`
	StatoRiga          *string  `json:"stato_riga"`
	StatoOrdine        *string  `json:"stato_ordine"`
	Stato              *string  `json:"stato"`
	ID                 int      `json:"id"`
	CodiceOrdine       *string  `json:"codice_ordine"`
	Serialnumber       *string  `json:"serialnumber"`
	IDAnagrafica       *string  `json:"id_anagrafica"`
	Quantita           *float64 `json:"quantita"`
	Canone             float64  `json:"canone"`
}

func (h *Handler) queryActiveLines(r *http.Request, req activeLinesRequest) ([]activeLineRow, error) {
	statusPlaceholders, nextIdx := buildInClause(1, len(req.Statuses))
	connTypePlaceholders, _ := buildInClause(nextIdx, len(req.ConnectionTypes))

	query := fmt.Sprintf(`SELECT cf.intestazione as ragione_sociale,
    tipo_conn, fl.fornitore, provincia, comune, p.tipo, p.profilo_commerciale,
    case when p.banda_up <> p.banda_down then 'CONDIVISA' else 'DEDICATA' end as macro,
    cogn_rsoc_intest_linea AS intestatario, r.nome_testata_ordine ordine, r.data_ultima_fatt fatturato_fino_al,
    r.stato_riga, r.stato_ordine,
    fl.stato, fl.id, codice_ordine, fl.serialnumber,  cf.codice_aggancio_gest AS id_anagrafica,
    r.quantita, r.canone
FROM
    loader.grappa_foglio_linee fl
        JOIN loader.grappa_cli_fatturazione cf ON fl.id_anagrafica = cf.id
        LEFT JOIN loader.grappa_profili p ON fl.id_profilo = p.id
        LEFT JOIN (
        SELECT *,
               ROW_NUMBER() OVER(PARTITION BY serialnumber ORDER BY data_documento DESC , progressivo_riga) AS rn
        FROM loader.v_ordini_ricorrenti
    ) r ON fl.serialnumber = r.serialnumber AND r.numero_azienda = cf.codice_aggancio_gest AND r.rn = 1
WHERE fl.stato IN (%s)
    AND fl.tipo_conn IN (%s)
ORDER BY cf.intestazione, tipo_conn, fl.fornitore, provincia, comune, p.tipo, p.profilo_commerciale`,
		statusPlaceholders, connTypePlaceholders)

	args := make([]any, 0, len(req.Statuses)+len(req.ConnectionTypes))
	for _, s := range req.Statuses {
		args = append(args, s)
	}
	for _, c := range req.ConnectionTypes {
		args = append(args, c)
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []activeLineRow
	for rows.Next() {
		var row activeLineRow
		var (
			tipoConn, fornitore, provincia, comune          sql.NullString
			tipo, profiloComm, macro, intestatario          sql.NullString
			ordine, fattFinoAl, statoRiga, statoOrdine      sql.NullString
			stato, codiceOrdine, serialnumber, idAnagrafica sql.NullString
			quantita                                        sql.NullFloat64
			canone                                          sql.NullFloat64
		)

		if err := rows.Scan(
			&row.RagioneSociale,
			&tipoConn, &fornitore, &provincia, &comune,
			&tipo, &profiloComm, &macro,
			&intestatario, &ordine, &fattFinoAl,
			&statoRiga, &statoOrdine,
			&stato, &row.ID, &codiceOrdine, &serialnumber, &idAnagrafica,
			&quantita, &canone,
		); err != nil {
			return nil, err
		}

		row.TipoConn = nullStringPtr(tipoConn)
		row.Fornitore = nullStringPtr(fornitore)
		row.Provincia = nullStringPtr(provincia)
		row.Comune = nullStringPtr(comune)
		row.Tipo = nullStringPtr(tipo)
		row.ProfiloCommerciale = nullStringPtr(profiloComm)
		row.Macro = nullStringPtr(macro)
		row.Intestatario = nullStringPtr(intestatario)
		row.Ordine = nullStringPtr(ordine)
		row.FatturatoFinoAl = nullStringPtr(fattFinoAl)
		row.StatoRiga = nullStringPtr(statoRiga)
		row.StatoOrdine = nullStringPtr(statoOrdine)
		row.Stato = nullStringPtr(stato)
		row.CodiceOrdine = nullStringPtr(codiceOrdine)
		row.Serialnumber = nullStringPtr(serialnumber)
		row.IDAnagrafica = nullStringPtr(idAnagrafica)
		row.Quantita = nullFloat64Ptr(quantita)
		if canone.Valid {
			row.Canone = canone.Float64
		}

		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []activeLineRow{}
	}
	return result, nil
}

// handleActiveLinesPreview returns active-lines report rows as JSON.
// POST /reports/v1/active-lines/preview
func (h *Handler) handleActiveLinesPreview(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	var req activeLinesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request_body")
		return
	}
	if len(req.Statuses) == 0 || len(req.ConnectionTypes) == 0 {
		httputil.Error(w, http.StatusBadRequest, "missing_required_fields")
		return
	}

	result, err := h.queryActiveLines(r, req)
	if err != nil {
		h.dbFailure(w, r, "active_lines_preview", err)
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleActiveLinesExport generates an XLSX export of active-lines report rows via Carbone.
// POST /reports/v1/active-lines/export
func (h *Handler) handleActiveLinesExport(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}
	if h.carbone == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "carbone_not_configured")
		return
	}

	var req activeLinesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request_body")
		return
	}
	if len(req.Statuses) == 0 || len(req.ConnectionTypes) == 0 {
		httputil.Error(w, http.StatusBadRequest, "missing_required_fields")
		return
	}

	result, err := h.queryActiveLines(r, req)
	if err != nil {
		h.dbFailure(w, r, "active_lines_export", err)
		return
	}

	xlsxBytes, err := h.carbone.GenerateXLSX(r.Context(), AccessiTemplateID, result)
	if err != nil {
		h.dbFailure(w, r, "active_lines_export_carbone", err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="report_accessi_attivi.xlsx"`)
	w.Write(xlsxBytes)
}
