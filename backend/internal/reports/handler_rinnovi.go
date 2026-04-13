package reports

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleUpcomingRenewals returns aggregated upcoming renewal data.
// GET /reports/v1/upcoming-renewals?months=4&minMrc=11
func (h *Handler) handleUpcomingRenewals(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	months := r.URL.Query().Get("months")
	if months == "" {
		months = "4"
	}
	minMrc := r.URL.Query().Get("minMrc")
	if minMrc == "" {
		minMrc = "11"
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
select ragione_sociale, min(prossimo_rinnovo) as rinnovi_dal, max(prossimo_rinnovo) as rinnovi_al,
       count(distinct nome_testata_ordine) as numero_ordini, count(0) as servizi_attivi,
       count(distinct nome_testata_ordine) || ' / ' || count(0) as ordini_servizi,
       sum(tacito_rinnovo) < count(0) as senza_tacito_rinnovo, sum(mrc) as canoni, numero_azienda
from loader.v_ordini_ricorrenti_conrinnovo os
where (durata_rinnovo > 3 or tacito_rinnovo=0)
  and stato_ordine in ('Evaso') and (data_cessazione is null) and stato_riga in ('Attiva')
  and prossimo_rinnovo BETWEEN current_date - INTERVAL '15 days' and current_date + ($1 || ' months')::interval
  and mrc >= $2
group by ragione_sociale, numero_azienda
order by 2`, months, minMrc)
	if err != nil {
		h.dbFailure(w, r, "upcoming_renewals", err)
		return
	}
	defer rows.Close()

	type renewal struct {
		RagioneSociale      string   `json:"ragione_sociale"`
		RinnoviDal          *string  `json:"rinnovi_dal"`
		RinnoviAl           *string  `json:"rinnovi_al"`
		NumeroOrdini        int      `json:"numero_ordini"`
		ServiziAttivi       int      `json:"servizi_attivi"`
		OrdiniServizi       *string  `json:"ordini_servizi"`
		SenzaTacitoRinnovo  *bool    `json:"senza_tacito_rinnovo"`
		Canoni              *float64 `json:"canoni"`
		NumeroAzienda       int      `json:"numero_azienda"`
	}

	var result []renewal
	for rows.Next() {
		var ren renewal
		var (
			ragioneSociale             sql.NullString
			rinnoviDal, rinnoviAl      sql.NullString
			ordiniServizi              sql.NullString
			senzaTacitoRinnovo         sql.NullBool
			canoni                     sql.NullFloat64
		)

		if err := rows.Scan(
			&ragioneSociale, &rinnoviDal, &rinnoviAl,
			&ren.NumeroOrdini, &ren.ServiziAttivi,
			&ordiniServizi, &senzaTacitoRinnovo, &canoni, &ren.NumeroAzienda,
		); err != nil {
			h.dbFailure(w, r, "upcoming_renewals_scan", err)
			return
		}

		ren.RagioneSociale = nullStr(ragioneSociale)
		ren.RinnoviDal = nullStrPtr(rinnoviDal)
		ren.RinnoviAl = nullStrPtr(rinnoviAl)
		ren.OrdiniServizi = nullStrPtr(ordiniServizi)
		ren.SenzaTacitoRinnovo = nullBoolPtr(senzaTacitoRinnovo)
		ren.Canoni = nullFloat64Ptr(canoni)

		result = append(result, ren)
	}
	if !h.rowsDone(w, r, rows, "upcoming_renewals") {
		return
	}
	if result == nil {
		result = []renewal{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleUpcomingRenewalRows returns line-level detail for a customer's upcoming renewals.
// GET /reports/v1/upcoming-renewals/{customerId}/rows?months=4&minMrc=11
func (h *Handler) handleUpcomingRenewalRows(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	customerId := r.PathValue("customerId")

	months := r.URL.Query().Get("months")
	if months == "" {
		months = "4"
	}
	minMrc := r.URL.Query().Get("minMrc")
	if minMrc == "" {
		minMrc = "11"
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT nome_testata_ordine, stato_ordine, descrizione_long, quantita,
       setup as nrc, canone as mrc, stato_riga, serialnumber, note_legali,
       data_attivazione, durata_servizio, durata_rinnovo,
       durata_servizio || ' / ' || durata_rinnovo as durata,
       prossimo_rinnovo, sost_ord, sostituito_da, tacito_rinnovo
from loader.v_ordini_ricorrenti_conrinnovo
where (durata_rinnovo > 3 or tacito_rinnovo=0)
  and stato_ordine in ('Evaso') and (data_cessazione is null) and stato_riga in ('Attiva')
  and prossimo_rinnovo BETWEEN current_date - INTERVAL '15 days' and current_date + ($1 || ' months')::interval
  and mrc >= $2
  and numero_azienda = $3
order by prossimo_rinnovo, nome_testata_ordine`, months, minMrc, customerId)
	if err != nil {
		h.dbFailure(w, r, "upcoming_renewal_rows", err)
		return
	}
	defer rows.Close()

	type renewalRow struct {
		NomeTestataOrdine string   `json:"nome_testata_ordine"`
		StatoOrdine       string   `json:"stato_ordine"`
		DescrizioneLong   *string  `json:"descrizione_long"`
		Quantita          int      `json:"quantita"`
		NRC               float64  `json:"nrc"`
		MRC               float64  `json:"mrc"`
		StatoRiga         string   `json:"stato_riga"`
		Serialnumber      *string  `json:"serialnumber"`
		NoteLegali        *string  `json:"note_legali"`
		DataAttivazione   *string  `json:"data_attivazione"`
		DurataServizio    *string  `json:"durata_servizio"`
		DurataRinnovo     *string  `json:"durata_rinnovo"`
		Durata            *string  `json:"durata"`
		ProssimoRinnovo   *string  `json:"prossimo_rinnovo"`
		SostOrd           *string  `json:"sost_ord"`
		SostituitoDa      *string  `json:"sostituito_da"`
		TacitoRinnovo     int      `json:"tacito_rinnovo"`
	}

	var result []renewalRow
	for rows.Next() {
		var row renewalRow
		var (
			nomeTestataOrdine, statoOrdine, descrizioneLong sql.NullString
			statoRiga, serialnumber, noteLegali             sql.NullString
			dataAttivazione, durataServizio, durataRinnovo   sql.NullString
			durata, prossimoRinnovo                          sql.NullString
			sostOrd, sostituitoDa                            sql.NullString
		)

		if err := rows.Scan(
			&nomeTestataOrdine, &statoOrdine, &descrizioneLong, &row.Quantita,
			&row.NRC, &row.MRC, &statoRiga, &serialnumber, &noteLegali,
			&dataAttivazione, &durataServizio, &durataRinnovo,
			&durata, &prossimoRinnovo, &sostOrd, &sostituitoDa, &row.TacitoRinnovo,
		); err != nil {
			h.dbFailure(w, r, "upcoming_renewal_rows_scan", err)
			return
		}

		row.NomeTestataOrdine = nullStr(nomeTestataOrdine)
		row.StatoOrdine = nullStr(statoOrdine)
		row.DescrizioneLong = nullStrPtr(descrizioneLong)
		row.StatoRiga = nullStr(statoRiga)
		row.Serialnumber = nullStrPtr(serialnumber)
		row.NoteLegali = nullStrPtr(noteLegali)
		row.DataAttivazione = nullStrPtr(dataAttivazione)
		row.DurataServizio = nullStrPtr(durataServizio)
		row.DurataRinnovo = nullStrPtr(durataRinnovo)
		row.Durata = nullStrPtr(durata)
		row.ProssimoRinnovo = nullStrPtr(prossimoRinnovo)
		row.SostOrd = nullStrPtr(sostOrd)
		row.SostituitoDa = nullStrPtr(sostituitoDa)

		result = append(result, row)
	}
	if !h.rowsDone(w, r, rows, "upcoming_renewal_rows") {
		return
	}
	if result == nil {
		result = []renewalRow{}
	}

	httputil.JSON(w, http.StatusOK, result)
}
