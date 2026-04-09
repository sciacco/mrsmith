package panoramica

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleListInvoices returns invoice lines for a customer, optionally filtered by period.
// GET /panoramica/v1/invoices?cliente=123&mesi=6
// mesi is optional: null/0 = no date filter
func (h *Handler) handleListInvoices(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	clienteStr := r.URL.Query().Get("cliente")
	if clienteStr == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_cliente_parameter")
		return
	}
	cliente, err := strconv.Atoi(clienteStr)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_cliente_parameter")
		return
	}

	query := `SELECT CASE WHEN rn = 1 THEN doc || ' ' || num_documento || CHR(13) || CHR(10) || to_char(data_documento, '(YYYY-MM-DD)') ELSE NULL END AS documento,
       descrizione_riga, qta, prezzo_unitario, prezzo_totale_netto, codice_articolo,
       data_documento, num_documento, id_cliente, progressivo_riga, serialnumber,
       riferimento_ordine_cliente, condizione_pagamento, scadenza, desc_conto_ricavo,
       gruppo, sottogruppo, rn
FROM loader.v_erp_fatture_nc
WHERE id_cliente = $1`

	args := []any{cliente}

	mesiStr := r.URL.Query().Get("mesi")
	if mesiStr != "" {
		mesiInt, err := strconv.Atoi(mesiStr)
		if err != nil || mesiInt <= 0 {
			httputil.Error(w, http.StatusBadRequest, "invalid_mesi_parameter")
			return
		}
		query += fmt.Sprintf(" AND data_documento >= current_date - interval '%d months'", mesiInt)
	}

	query += ` ORDER BY anno_documento DESC, mese_documento DESC, tipo_documento, num_documento, rn`

	rows, err := h.mistraDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_invoices", err)
		return
	}
	defer rows.Close()

	type invoiceLine struct {
		Documento               *string  `json:"documento"`
		DescrizioneRiga         string   `json:"descrizione_riga"`
		Qta                     float64  `json:"qta"`
		PrezzoUnitario          float64  `json:"prezzo_unitario"`
		PrezzoTotaleNetto       float64  `json:"prezzo_totale_netto"`
		CodiceArticolo          *string  `json:"codice_articolo"`
		DataDocumento           *string  `json:"data_documento"`
		NumDocumento            *string  `json:"num_documento"`
		IDCliente               int      `json:"id_cliente"`
		ProgressivoRiga         int      `json:"progressivo_riga"`
		Serialnumber            *string  `json:"serialnumber"`
		RiferimentoOrdineCliente *string `json:"riferimento_ordine_cliente"`
		CondizionePagamento     *string  `json:"condizione_pagamento"`
		Scadenza                *string  `json:"scadenza"`
		DescContoRicavo         *string  `json:"desc_conto_ricavo"`
		Gruppo                  *string  `json:"gruppo"`
		Sottogruppo             *string  `json:"sottogruppo"`
		RN                      int      `json:"rn"`
	}

	var result []invoiceLine
	for rows.Next() {
		var row invoiceLine
		var (
			documento, codiceArticolo, dataDocumento, numDocumento sql.NullString
			serialnumber, rifOrdCliente, condPagamento, scadenza   sql.NullString
			descContoRicavo, gruppo, sottogruppo                    sql.NullString
		)

		if err := rows.Scan(
			&documento, &row.DescrizioneRiga, &row.Qta, &row.PrezzoUnitario,
			&row.PrezzoTotaleNetto, &codiceArticolo, &dataDocumento, &numDocumento,
			&row.IDCliente, &row.ProgressivoRiga, &serialnumber, &rifOrdCliente,
			&condPagamento, &scadenza, &descContoRicavo, &gruppo, &sottogruppo, &row.RN,
		); err != nil {
			h.dbFailure(w, r, "list_invoices_scan", err)
			return
		}

		row.Documento = nullStringPtr(documento)
		row.CodiceArticolo = nullStringPtr(codiceArticolo)
		row.DataDocumento = nullStringPtr(dataDocumento)
		row.NumDocumento = nullStringPtr(numDocumento)
		row.Serialnumber = nullStringPtr(serialnumber)
		row.RiferimentoOrdineCliente = nullStringPtr(rifOrdCliente)
		row.CondizionePagamento = nullStringPtr(condPagamento)
		row.Scadenza = nullStringPtr(scadenza)
		row.DescContoRicavo = nullStringPtr(descContoRicavo)
		row.Gruppo = nullStringPtr(gruppo)
		row.Sottogruppo = nullStringPtr(sottogruppo)

		result = append(result, row)
	}
	if !h.rowsDone(w, r, rows, "list_invoices") {
		return
	}
	if result == nil {
		result = []invoiceLine{}
	}

	httputil.JSON(w, http.StatusOK, result)
}
