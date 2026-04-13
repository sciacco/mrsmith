package reports

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type ordersRequest struct {
	DateFrom string   `json:"dateFrom"`
	DateTo   string   `json:"dateTo"`
	Statuses []string `json:"statuses"`
}

type orderReportRow struct {
	RagioneSociale   string  `json:"ragione_sociale"`
	StatoOrdine      string  `json:"stato_ordine"`
	NumeroOrdine     *string `json:"numero_ordine"`
	DescrizioneLong  *string `json:"descrizione_long"`
	Quantita         int     `json:"quantita"`
	NRC              float64 `json:"nrc"`
	MRC              float64 `json:"mrc"`
	TotaleMRC        float64 `json:"totale_mrc"`
	NumeroAzienda    int     `json:"numero_azienda"`
	DataDocumento    *string `json:"data_documento"`
	StatoRiga        *string `json:"stato_riga"`
	DataUltimaFatt   *string `json:"data_ultima_fatt"`
	Serialnumber     *string `json:"serialnumber"`
	MetodoPagamento  *string `json:"metodo_pagamento"`
	DurataServizio   *string `json:"durata_servizio"`
	DurataRinnovo    *string `json:"durata_rinnovo"`
	DataCessazione   *string `json:"data_cessazione"`
	DataAttivazione  *string `json:"data_attivazione"`
	NoteLegali       *string `json:"note_legali"`
	SostOrd          *string `json:"sost_ord"`
	SostituitoDa     *string `json:"sostituito_da"`
	ProgressivoRiga  int     `json:"progressivo_riga"`
}

// buildInClause generates a parameterized IN clause like "$1, $2, $3" starting
// at startIdx and returns the clause string and the next available index.
func buildInClause(startIdx int, count int) (string, int) {
	parts := make([]string, count)
	for i := 0; i < count; i++ {
		parts[i] = fmt.Sprintf("$%d", startIdx+i)
	}
	return strings.Join(parts, ", "), startIdx + count
}

func (h *Handler) queryOrders(r *http.Request, req ordersRequest) ([]orderReportRow, error) {
	statusPlaceholders, nextIdx := buildInClause(1, len(req.Statuses))

	query := fmt.Sprintf(`SELECT eac.ragione_sociale, o.stato_ordine,
       o.nome_testata_ordine as numero_ordine,
       o.descrizione_long,
       o.quantita,
       o.setup as nrc,
       o.canone as mrc,
       round(o.quantita::decimal * o.canone::decimal,2) as totale_mrc,
       o.numero_azienda,
       o.data_ordine as data_documento,
       o.stato_riga,
       o.data_ultima_fatt,
       o.serialnumber,
       o.metodo_pagamento,o.durata_servizio, o.durata_rinnovo, o.data_cessazione, o.data_attivazione, o.note_legali,
       o.sost_ord, o.sostituito_da, o.progressivo_riga
FROM loader.v_ordini_ric_spot AS o
JOIN loader.erp_anagrafiche_clienti eac ON o.numero_azienda = eac.numero_azienda
WHERE stato_ordine IN (%s)
  AND data_ordine BETWEEN $%d AND $%d
ORDER BY eac.ragione_sociale, data_documento, nome_testata_ordine, progressivo_riga`,
		statusPlaceholders, nextIdx, nextIdx+1)

	args := make([]any, 0, len(req.Statuses)+2)
	for _, s := range req.Statuses {
		args = append(args, s)
	}
	args = append(args, req.DateFrom, req.DateTo)

	rows, err := h.mistraDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []orderReportRow
	for rows.Next() {
		var o orderReportRow
		var (
			numeroOrdine, descrizioneLong, statoRiga         sql.NullString
			dataDocumento, dataUltimaFatt, serialnumber      sql.NullString
			metodoPagamento, durataServizio, durataRinnovo    sql.NullString
			dataCessazione, dataAttivazione, noteLegali       sql.NullString
			sostOrd, sostituitoDa                             sql.NullString
		)

		if err := rows.Scan(
			&o.RagioneSociale, &o.StatoOrdine,
			&numeroOrdine, &descrizioneLong,
			&o.Quantita, &o.NRC, &o.MRC, &o.TotaleMRC,
			&o.NumeroAzienda, &dataDocumento, &statoRiga,
			&dataUltimaFatt, &serialnumber,
			&metodoPagamento, &durataServizio, &durataRinnovo,
			&dataCessazione, &dataAttivazione, &noteLegali,
			&sostOrd, &sostituitoDa, &o.ProgressivoRiga,
		); err != nil {
			return nil, err
		}

		o.NumeroOrdine = nullStringPtr(numeroOrdine)
		o.DescrizioneLong = nullStringPtr(descrizioneLong)
		o.DataDocumento = nullStringPtr(dataDocumento)
		o.StatoRiga = nullStringPtr(statoRiga)
		o.DataUltimaFatt = nullStringPtr(dataUltimaFatt)
		o.Serialnumber = nullStringPtr(serialnumber)
		o.MetodoPagamento = nullStringPtr(metodoPagamento)
		o.DurataServizio = nullStringPtr(durataServizio)
		o.DurataRinnovo = nullStringPtr(durataRinnovo)
		o.DataCessazione = nullStringPtr(dataCessazione)
		o.DataAttivazione = nullStringPtr(dataAttivazione)
		o.NoteLegali = nullStringPtr(noteLegali)
		o.SostOrd = nullStringPtr(sostOrd)
		o.SostituitoDa = nullStringPtr(sostituitoDa)

		result = append(result, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []orderReportRow{}
	}
	return result, nil
}

// handleOrdersPreview returns order report rows as JSON.
// POST /reports/v1/orders/preview
func (h *Handler) handleOrdersPreview(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	var req ordersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request_body")
		return
	}
	if len(req.Statuses) == 0 || req.DateFrom == "" || req.DateTo == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_required_fields")
		return
	}

	result, err := h.queryOrders(r, req)
	if err != nil {
		h.dbFailure(w, r, "orders_preview", err)
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleOrdersExport generates an XLSX export of order report rows via Carbone.
// POST /reports/v1/orders/export
func (h *Handler) handleOrdersExport(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}
	if h.carbone == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "carbone_not_configured")
		return
	}

	var req ordersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request_body")
		return
	}
	if len(req.Statuses) == 0 || req.DateFrom == "" || req.DateTo == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_required_fields")
		return
	}

	result, err := h.queryOrders(r, req)
	if err != nil {
		h.dbFailure(w, r, "orders_export", err)
		return
	}

	xlsxBytes, err := h.carbone.GenerateXLSX(r.Context(), OrdiniTemplateID, result)
	if err != nil {
		h.dbFailure(w, r, "orders_export_carbone", err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="report_ordini.xlsx"`)
	w.Write(xlsxBytes)
}

func nullStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}
