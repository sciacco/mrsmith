package reports

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handlePendingActivations returns confirmed orders with rows pending activation.
// GET /reports/v1/pending-activations
func (h *Handler) handlePendingActivations(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT distinct
       eac.ragione_sociale,
       nome_testata_ordine as numero_ordine,
       data_documento,
       durata_servizio, durata_rinnovo, sost_ord, sostituito_da,
       loader.get_reverse_order_history_path(nome_testata_ordine) as storico,
       os.numero_azienda
from loader.v_ordini_sintesi os join loader.erp_anagrafiche_clienti eac on os.numero_azienda = eac.numero_azienda
where os.stato_ordine in ('Confermato') and stato_riga in ('Da attivare')
order by eac.ragione_sociale, data_documento, nome_testata_ordine`)
	if err != nil {
		h.dbFailure(w, r, "pending_activations", err)
		return
	}
	defer rows.Close()

	type activation struct {
		RagioneSociale string  `json:"ragione_sociale"`
		NumeroOrdine   string  `json:"numero_ordine"`
		DataDocumento  *string `json:"data_documento"`
		DurataServizio *string `json:"durata_servizio"`
		DurataRinnovo  *string `json:"durata_rinnovo"`
		SostOrd        *string `json:"sost_ord"`
		SostituitoDa   *string `json:"sostituito_da"`
		Storico        *string `json:"storico"`
		NumeroAzienda  int     `json:"numero_azienda"`
	}

	var result []activation
	for rows.Next() {
		var a activation
		var (
			ragioneSociale, numeroOrdine             sql.NullString
			dataDocumento, durataServizio             sql.NullString
			durataRinnovo, sostOrd, sostituitoDa     sql.NullString
			storico                                  sql.NullString
		)

		if err := rows.Scan(
			&ragioneSociale, &numeroOrdine, &dataDocumento,
			&durataServizio, &durataRinnovo, &sostOrd, &sostituitoDa,
			&storico, &a.NumeroAzienda,
		); err != nil {
			h.dbFailure(w, r, "pending_activations_scan", err)
			return
		}

		a.RagioneSociale = nullStr(ragioneSociale)
		a.NumeroOrdine = nullStr(numeroOrdine)
		a.DataDocumento = nullStrPtr(dataDocumento)
		a.DurataServizio = nullStrPtr(durataServizio)
		a.DurataRinnovo = nullStrPtr(durataRinnovo)
		a.SostOrd = nullStrPtr(sostOrd)
		a.SostituitoDa = nullStrPtr(sostituitoDa)
		a.Storico = nullStrPtr(storico)

		result = append(result, a)
	}
	if !h.rowsDone(w, r, rows, "pending_activations") {
		return
	}
	if result == nil {
		result = []activation{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handlePendingActivationRows returns line-level detail for a pending activation order.
// GET /reports/v1/pending-activations/{orderNumber}/rows
func (h *Handler) handlePendingActivationRows(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	orderNumber := r.PathValue("orderNumber")

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT descrizione_long, quantita, nrc, mrc, totale_mrc,
       stato_riga, serialnumber, note_legali
from loader.v_ordini_sintesi os join loader.erp_anagrafiche_clienti eac on os.numero_azienda = eac.numero_azienda
where os.nome_testata_ordine = $1
  and stato_riga in ('Da attivare')
order by eac.ragione_sociale, data_documento, nome_testata_ordine`, orderNumber)
	if err != nil {
		h.dbFailure(w, r, "pending_activation_rows", err)
		return
	}
	defer rows.Close()

	type activationRow struct {
		DescrizioneLong *string  `json:"descrizione_long"`
		Quantita        int      `json:"quantita"`
		NRC             float64  `json:"nrc"`
		MRC             float64  `json:"mrc"`
		TotaleMRC       float64  `json:"totale_mrc"`
		StatoRiga       string   `json:"stato_riga"`
		Serialnumber    *string  `json:"serialnumber"`
		NoteLegali      *string  `json:"note_legali"`
	}

	var result []activationRow
	for rows.Next() {
		var row activationRow
		var (
			descrizioneLong, statoRiga sql.NullString
			serialnumber, noteLegali  sql.NullString
		)

		if err := rows.Scan(
			&descrizioneLong, &row.Quantita, &row.NRC, &row.MRC, &row.TotaleMRC,
			&statoRiga, &serialnumber, &noteLegali,
		); err != nil {
			h.dbFailure(w, r, "pending_activation_rows_scan", err)
			return
		}

		row.DescrizioneLong = nullStrPtr(descrizioneLong)
		row.StatoRiga = nullStr(statoRiga)
		row.Serialnumber = nullStrPtr(serialnumber)
		row.NoteLegali = nullStrPtr(noteLegali)

		result = append(result, row)
	}
	if !h.rowsDone(w, r, rows, "pending_activation_rows") {
		return
	}
	if result == nil {
		result = []activationRow{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// -- Null helpers --

func nullStrPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullFloat64Ptr(nf sql.NullFloat64) *float64 {
	if nf.Valid {
		return &nf.Float64
	}
	return nil
}

func nullBoolPtr(nb sql.NullBool) *bool {
	if nb.Valid {
		return &nb.Bool
	}
	return nil
}
