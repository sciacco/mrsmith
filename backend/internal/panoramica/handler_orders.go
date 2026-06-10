package panoramica

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleListOrderStatuses returns distinct order statuses.
// GET /panoramica/v1/order-statuses
func (h *Handler) handleListOrderStatuses(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(),
		`SELECT DISTINCT stato_ordine FROM loader.v_ordini_ric_spot ORDER BY stato_ordine`)
	if err != nil {
		h.dbFailure(w, r, "list_order_statuses", err)
		return
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			h.dbFailure(w, r, "list_order_statuses_scan", err)
			return
		}
		result = append(result, s)
	}
	if !h.rowsDone(w, r, rows, "list_order_statuses") {
		return
	}
	if result == nil {
		result = []string{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleListOrdersDetail returns full order detail rows for a customer and status filter.
// Covers recurring (TSC-ORDINE-RIC) and spot (TSC-ORDINE) orders; for spot orders the
// canone is a one-off charge, so it is folded into setup (NRC) and mrc is NULL.
// GET /panoramica/v1/orders/detail?cliente=123&stati=Evaso,Confermato
func (h *Handler) handleListOrdersDetail(w http.ResponseWriter, r *http.Request) {
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

	statiStr := r.URL.Query().Get("stati")
	stati := parseStringList(statiStr)
	if len(stati) == 0 {
		httputil.Error(w, http.StatusBadRequest, "missing_stati_parameter")
		return
	}

	// Build parameterized query with dynamic placeholders for stati array
	args := []any{cliente}
	placeholders := ""
	for i, s := range stati {
		if i > 0 {
			placeholders += ","
		}
		args = append(args, s)
		placeholders += fmt.Sprintf("$%d", i+2)
	}

	// v_ordini_ric_spot is the canonical recurring+spot source (it already trims the
	// fixed-width padded tipo_documento, e.g. 'TSC-ORDINE    ', and excludes CDL-AUTO).
	// On top of it: for spot orders (TSC-ORDINE) the canone is a one-off charge, so it
	// is folded into setup (NRC) and mrc is NULL.
	query := fmt.Sprintf(`SELECT v.ragione_sociale, v.data_ordine,
    v.nome_testata_ordine, v.cliente, v.numero_azienda, v.id_gamma, v.commerciale,
    v.data_documento, v.data_conferma, v.stato_ordine, v.tipo_ordine, v.tipo_documento,
    v.sost_ord, v.riferimento_odv_cliente, v.durata_servizio, v.tacito_rinnovo,
    v.durata_rinnovo, v.tempi_rilascio, v.metodo_pagamento, v.note_legali,
    v.referente_amm_nome, v.referente_amm_mail, v.referente_amm_tel,
    v.referente_tech_nome, v.referente_tech_mail, v.referente_tech_tel,
    v.referente_altro_nome, v.referente_altro_mail, v.referente_altro_tel,
    v.data_creazione, v.data_variazione, v.sostituito_da,
    v.quantita, v.codice_kit, v.codice_prodotto, v.descrizione_prodotto, v.descrizione_estesa,
    v.serialnumber,
    CASE WHEN btrim(v.tipo_documento) = 'TSC-ORDINE'
         THEN v.setup + COALESCE(v.quantita * v.canone, 0)
         ELSE v.setup
    END AS setup,
    v.canone, v.valuta, v.costo_cessazione,
    v.data_attivazione, v.data_disdetta, v.data_cessazione,
    v.raggruppamento_fatturazione, v.intervallo_fatt_attivazione, v.intervallo_fatt_canone,
    v.data_ultima_fatt, v.data_fine_fatt,
    v.system_odv_row, v.id_gamma_testata, v.progressivo_riga,
    v.annullato,
    v.data_scadenza_ordine,
    CASE WHEN btrim(v.tipo_documento) = 'TSC-ORDINE' THEN NULL
         ELSE v.mrc
    END AS mrc,
    v.famiglia, v.sotto_famiglia, v.conto_ricavo,
    v.stato_riga, v.intestazione_ordine, v.descrizione_long,
    loader.get_reverse_order_history_path(v.nome_testata_ordine) AS storico
FROM loader.v_ordini_ric_spot v
WHERE v.numero_azienda = $1
  AND v.stato_ordine IN (%s)
ORDER BY v.data_documento DESC NULLS LAST, v.nome_testata_ordine, v.progressivo_riga`, placeholders)

	rows, err := h.mistraDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_orders_detail", err)
		return
	}
	defer rows.Close()

	type detailRow struct {
		// Anagrafica
		RagioneSociale    string  `json:"ragione_sociale"`
		DataOrdine        *string `json:"data_ordine"`
		NomeTestataOrdine string  `json:"nome_testata_ordine"`
		Cliente           *string `json:"cliente"`
		NumeroAzienda     int     `json:"numero_azienda"`
		IDGamma           *string `json:"id_gamma"`
		Commerciale       *string `json:"commerciale"`
		DataDocumento     *string `json:"data_documento"`
		DataConferma      *string `json:"data_conferma"`
		StatoOrdine       string  `json:"stato_ordine"`
		TipoOrdine        *string `json:"tipo_ordine"`
		TipoDocumento     *string `json:"tipo_documento"`
		SostOrd           *string `json:"sost_ord"`
		RiferimentoODV    *string `json:"riferimento_odv_cliente"`
		DurataServizio    *string `json:"durata_servizio"`
		TacitoRinnovo     *string `json:"tacito_rinnovo"`
		DurataRinnovo     *string `json:"durata_rinnovo"`
		TempiRilascio     *string `json:"tempi_rilascio"`
		MetodoPagamento   *string `json:"metodo_pagamento"`
		NoteLegali        *string `json:"note_legali"`
		// Referenti
		RefAmmNome   *string `json:"referente_amm_nome"`
		RefAmmMail   *string `json:"referente_amm_mail"`
		RefAmmTel    *string `json:"referente_amm_tel"`
		RefTechNome  *string `json:"referente_tech_nome"`
		RefTechMail  *string `json:"referente_tech_mail"`
		RefTechTel   *string `json:"referente_tech_tel"`
		RefAltroNome *string `json:"referente_altro_nome"`
		RefAltroMail *string `json:"referente_altro_mail"`
		RefAltroTel  *string `json:"referente_altro_tel"`
		// Date testata
		DataCreazione  *string `json:"data_creazione"`
		DataVariazione *string `json:"data_variazione"`
		SostituitoDa   *string `json:"sostituito_da"`
		// Riga
		Quantita                   *float64 `json:"quantita"`
		CodiceKit                  *string  `json:"codice_kit"`
		CodiceProdotto             *string  `json:"codice_prodotto"`
		DescrizioneProdotto        *string  `json:"descrizione_prodotto"`
		DescrizioneEstesa          *string  `json:"descrizione_estesa"`
		Serialnumber               *string  `json:"serialnumber"`
		Setup                      float64  `json:"setup"`
		Canone                     float64  `json:"canone"`
		Valuta                     *string  `json:"valuta"`
		CostoCessazione            float64  `json:"costo_cessazione"`
		DataAttivazione            *string  `json:"data_attivazione"`
		DataDisdetta               *string  `json:"data_disdetta"`
		DataCessazione             *string  `json:"data_cessazione"`
		RaggruppamentoFatturazione *string  `json:"raggruppamento_fatturazione"`
		IntervalloFattAttivazione  *string  `json:"intervallo_fatt_attivazione"`
		IntervalloFattCanone       *string  `json:"intervallo_fatt_canone"`
		DataUltimaFatt             *string  `json:"data_ultima_fatt"`
		DataFineFatt               *string  `json:"data_fine_fatt"`
		SystemOdvRow               *string  `json:"system_odv_row"`
		IDGammaTestata             *string  `json:"id_gamma_testata"`
		ProgressivoRiga            int      `json:"progressivo_riga"`
		Annullato                  int      `json:"annullato"`
		DataScadenzaOrdine         *string  `json:"data_scadenza_ordine"`
		MRC                        *float64 `json:"mrc"`
		// Prodotto
		Famiglia        *string `json:"famiglia"`
		SottoFamiglia   *string `json:"sotto_famiglia"`
		ContoRicavo     *string `json:"conto_ricavo"`
		StatoRiga       string  `json:"stato_riga"`
		IntOrdine       *string `json:"intestazione_ordine"`
		DescrizioneLong *string `json:"descrizione_long"`
		Storico         *string `json:"storico"`
	}

	var result []detailRow
	for rows.Next() {
		var d detailRow
		var (
			dataOrdine, cliente, idGamma, commerciale                 sql.NullString
			dataDocumento, dataConferma, tipoOrdine, tipoDocumento    sql.NullString
			sostOrd, rifODV, durataServizio, tacitoRinnovo            sql.NullString
			durataRinnovo, tempiRilascio, metodoPagamento, noteLegali sql.NullString
			refAmmNome, refAmmMail, refAmmTel                         sql.NullString
			refTechNome, refTechMail, refTechTel                      sql.NullString
			refAltroNome, refAltroMail, refAltroTel                   sql.NullString
			dataCreazione, dataVariazione, sostituitoDa               sql.NullString
			codiceKit, codiceProdotto, descProdotto, descEstesa       sql.NullString
			serialnumber, valuta                                      sql.NullString
			dataAtt, dataDisdetta, dataCess                           sql.NullString
			raggFatt, intFattAtt, intFattCanone                       sql.NullString
			dataUltFatt, dataFineFatt, sysOdvRow, idGammaTestata      sql.NullString
			dataScadenza                                              sql.NullString
			famiglia, sottoFamiglia, contoRicavo                      sql.NullString
			intOrdine, descLong, storico                              sql.NullString
			quantita, mrc                                             sql.NullFloat64
		)

		if err := rows.Scan(
			&d.RagioneSociale, &dataOrdine, &d.NomeTestataOrdine, &cliente,
			&d.NumeroAzienda, &idGamma, &commerciale, &dataDocumento,
			&dataConferma, &d.StatoOrdine, &tipoOrdine, &tipoDocumento,
			&sostOrd, &rifODV, &durataServizio, &tacitoRinnovo,
			&durataRinnovo, &tempiRilascio, &metodoPagamento, &noteLegali,
			&refAmmNome, &refAmmMail, &refAmmTel,
			&refTechNome, &refTechMail, &refTechTel,
			&refAltroNome, &refAltroMail, &refAltroTel,
			&dataCreazione, &dataVariazione, &sostituitoDa,
			&quantita, &codiceKit, &codiceProdotto, &descProdotto, &descEstesa,
			&serialnumber, &d.Setup, &d.Canone, &valuta, &d.CostoCessazione,
			&dataAtt, &dataDisdetta, &dataCess,
			&raggFatt, &intFattAtt, &intFattCanone,
			&dataUltFatt, &dataFineFatt, &sysOdvRow, &idGammaTestata, &d.ProgressivoRiga,
			&d.Annullato, &dataScadenza, &mrc,
			&famiglia, &sottoFamiglia, &contoRicavo,
			&d.StatoRiga, &intOrdine, &descLong, &storico,
		); err != nil {
			h.dbFailure(w, r, "list_orders_detail_scan", err)
			return
		}

		d.DataOrdine = nullStringPtr(dataOrdine)
		d.Cliente = nullStringPtr(cliente)
		d.IDGamma = nullStringPtr(idGamma)
		d.Commerciale = nullStringPtr(commerciale)
		d.DataDocumento = nullStringPtr(dataDocumento)
		d.DataConferma = nullStringPtr(dataConferma)
		d.TipoOrdine = nullStringPtr(tipoOrdine)
		d.TipoDocumento = nullStringPtr(tipoDocumento)
		d.SostOrd = nullStringPtr(sostOrd)
		d.RiferimentoODV = nullStringPtr(rifODV)
		d.DurataServizio = nullStringPtr(durataServizio)
		d.TacitoRinnovo = nullStringPtr(tacitoRinnovo)
		d.DurataRinnovo = nullStringPtr(durataRinnovo)
		d.TempiRilascio = nullStringPtr(tempiRilascio)
		d.MetodoPagamento = nullStringPtr(metodoPagamento)
		d.NoteLegali = nullStringPtr(noteLegali)
		d.RefAmmNome = nullStringPtr(refAmmNome)
		d.RefAmmMail = nullStringPtr(refAmmMail)
		d.RefAmmTel = nullStringPtr(refAmmTel)
		d.RefTechNome = nullStringPtr(refTechNome)
		d.RefTechMail = nullStringPtr(refTechMail)
		d.RefTechTel = nullStringPtr(refTechTel)
		d.RefAltroNome = nullStringPtr(refAltroNome)
		d.RefAltroMail = nullStringPtr(refAltroMail)
		d.RefAltroTel = nullStringPtr(refAltroTel)
		d.DataCreazione = nullStringPtr(dataCreazione)
		d.DataVariazione = nullStringPtr(dataVariazione)
		d.SostituitoDa = nullStringPtr(sostituitoDa)
		d.Quantita = nullFloat64Ptr(quantita)
		d.CodiceKit = nullStringPtr(codiceKit)
		d.CodiceProdotto = nullStringPtr(codiceProdotto)
		d.DescrizioneProdotto = nullStringPtr(descProdotto)
		d.DescrizioneEstesa = nullStringPtr(descEstesa)
		d.Serialnumber = nullStringPtr(serialnumber)
		d.Valuta = nullStringPtr(valuta)
		d.DataAttivazione = nullStringPtr(dataAtt)
		d.DataDisdetta = nullStringPtr(dataDisdetta)
		d.DataCessazione = nullStringPtr(dataCess)
		d.RaggruppamentoFatturazione = nullStringPtr(raggFatt)
		d.IntervalloFattAttivazione = nullStringPtr(intFattAtt)
		d.IntervalloFattCanone = nullStringPtr(intFattCanone)
		d.DataUltimaFatt = nullStringPtr(dataUltFatt)
		d.DataFineFatt = nullStringPtr(dataFineFatt)
		d.SystemOdvRow = nullStringPtr(sysOdvRow)
		d.IDGammaTestata = nullStringPtr(idGammaTestata)
		d.DataScadenzaOrdine = nullStringPtr(dataScadenza)
		d.MRC = nullFloat64Ptr(mrc)
		d.Famiglia = nullStringPtr(famiglia)
		d.SottoFamiglia = nullStringPtr(sottoFamiglia)
		d.ContoRicavo = nullStringPtr(contoRicavo)
		d.IntOrdine = nullStringPtr(intOrdine)
		d.DescrizioneLong = nullStringPtr(descLong)
		d.Storico = nullStringPtr(storico)

		result = append(result, d)
	}
	if !h.rowsDone(w, r, rows, "list_orders_detail") {
		return
	}
	if result == nil {
		result = []detailRow{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

func nullStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

func nullFloat64Ptr(nf sql.NullFloat64) *float64 {
	if nf.Valid {
		return &nf.Float64
	}
	return nil
}
