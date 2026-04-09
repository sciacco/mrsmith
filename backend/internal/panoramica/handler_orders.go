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
		`SELECT DISTINCT stato_ordine FROM loader.v_ordini_ricorrenti ORDER BY stato_ordine`)
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

// handleListOrdersSummary returns order summary rows for a customer and status filter.
// GET /panoramica/v1/orders/summary?cliente=123&stati=Evaso,Confermato
func (h *Handler) handleListOrdersSummary(w http.ResponseWriter, r *http.Request) {
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

	query := fmt.Sprintf(`SELECT stato, numero_ordine, descrizione_long, quantita, nrc, mrc, totale_mrc,
       stato_ordine, nome_testata_ordine, rn, numero_azienda, data_documento,
       stato_riga, data_ultima_fatt, serialnumber,
       metodo_pagamento, durata_servizio, durata_rinnovo, data_cessazione,
       data_attivazione, note_legali, sost_ord, sostituito_da,
       loader.get_reverse_order_history_path(nome_testata_ordine) AS storico
FROM loader.v_ordini_sintesi
WHERE numero_azienda = $1
  AND stato_ordine IN (%s)
ORDER BY data_documento, nome_testata_ordine, rn`, placeholders)

	rows, err := h.mistraDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_orders_summary", err)
		return
	}
	defer rows.Close()

	type orderRow struct {
		Stato             string  `json:"stato"`
		NumeroOrdine      string  `json:"numero_ordine"`
		DescrizioneLong   string  `json:"descrizione_long"`
		Quantita          int     `json:"quantita"`
		NRC               float64 `json:"nrc"`
		MRC               float64 `json:"mrc"`
		TotaleMRC         float64 `json:"totale_mrc"`
		StatoOrdine       string  `json:"stato_ordine"`
		NomeTestataOrdine string  `json:"nome_testata_ordine"`
		RN                int     `json:"rn"`
		NumeroAzienda     int     `json:"numero_azienda"`
		DataDocumento     *string `json:"data_documento"`
		StatoRiga         string  `json:"stato_riga"`
		DataUltimaFatt    *string `json:"data_ultima_fatt"`
		Serialnumber      *string `json:"serialnumber"`
		MetodoPagamento   *string `json:"metodo_pagamento"`
		DurataServizio    *string `json:"durata_servizio"`
		DurataRinnovo     *string `json:"durata_rinnovo"`
		DataCessazione    *string `json:"data_cessazione"`
		DataAttivazione   *string `json:"data_attivazione"`
		NoteLegali        *string `json:"note_legali"`
		SostOrd           *string `json:"sost_ord"`
		SostituitoDa      *string `json:"sostituito_da"`
		Storico           *string `json:"storico"`
	}

	var result []orderRow
	for rows.Next() {
		var o orderRow
		var stato, numeroOrdine, descrizioneLong, statoOrdine, nomeTestataOrdine sql.NullString
		var statoRiga, dataDocumento, dataUltimaFatt, serialnumber sql.NullString
		var metodoPagamento, durataServizio, durataRinnovo sql.NullString
		var dataCessazione, dataAttivazione, noteLegali sql.NullString
		var sostOrd, sostituitoDa, storico sql.NullString

		if err := rows.Scan(
			&stato, &numeroOrdine, &descrizioneLong, &o.Quantita,
			&o.NRC, &o.MRC, &o.TotaleMRC, &statoOrdine, &nomeTestataOrdine,
			&o.RN, &o.NumeroAzienda, &dataDocumento, &statoRiga,
			&dataUltimaFatt, &serialnumber, &metodoPagamento, &durataServizio,
			&durataRinnovo, &dataCessazione, &dataAttivazione, &noteLegali,
			&sostOrd, &sostituitoDa, &storico,
		); err != nil {
			h.dbFailure(w, r, "list_orders_summary_scan", err)
			return
		}

		o.Stato = nullStringValue(stato)
		o.NumeroOrdine = nullStringValue(numeroOrdine)
		o.DescrizioneLong = nullStringValue(descrizioneLong)
		o.StatoOrdine = nullStringValue(statoOrdine)
		o.NomeTestataOrdine = nullStringValue(nomeTestataOrdine)
		o.StatoRiga = nullStringValue(statoRiga)
		o.DataDocumento = nullStringPtr(dataDocumento)
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
		o.Storico = nullStringPtr(storico)

		result = append(result, o)
	}
	if !h.rowsDone(w, r, rows, "list_orders_summary") {
		return
	}
	if result == nil {
		result = []orderRow{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleListOrdersDetail returns full order detail rows for a customer and status filter.
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

	query := fmt.Sprintf(`SELECT c.ragione_sociale,
    CASE WHEN o.data_conferma > o.data_documento THEN o.data_conferma ELSE o.data_documento END AS data_ordine,
    o.nome_testata_ordine, o.cliente, o.numero_azienda, o.id_gamma, o.commerciale,
    o.data_documento, o.data_conferma, o.stato_ordine, o.tipo_ordine, o.tipo_documento,
    o.sost_ord, o.riferimento_odv_cliente, o.durata_servizio, o.tacito_rinnovo,
    o.durata_rinnovo, o.tempi_rilascio, o.metodo_pagamento, o.note_legali,
    o.referente_amm_nome, o.referente_amm_mail, o.referente_amm_tel,
    o.referente_tech_nome, o.referente_tech_mail, o.referente_tech_tel,
    o.referente_altro_nome, o.referente_altro_mail, o.referente_altro_tel,
    o.data_creazione, o.data_variazione, o.sostituito_da,
    r.quantita, r.codice_kit, r.codice_prodotto, r.descrizione_prodotto, r.descrizione_estesa,
    r.serialnumber, r.setup, r.canone, r.valuta, r.costo_cessazione,
    NULLIF(r.data_attivazione, '0001-01-01 00:00:00'::timestamp) AS data_attivazione,
    NULLIF(r.data_disdetta, '0001-01-01 00:00:00'::timestamp) AS data_disdetta,
    NULLIF(r.data_cessazione, '0001-01-01 00:00:00'::timestamp) AS data_cessazione,
    r.raggruppamento_fatturazione, r.intervallo_fatt_attivazione, r.intervallo_fatt_canone,
    NULLIF(r.data_ultima_fatt, '0001-01-01 00:00:00'::timestamp) AS data_ultima_fatt,
    NULLIF(r.data_fine_fatt, '0001-01-01 00:00:00'::timestamp) AS data_fine_fatt,
    r.system_odv_row, r.id_gamma_testata, r.progressivo_riga,
    CASE WHEN r.progressivo_riga = 1 THEN o.nome_testata_ordine ELSE NULL END AS ordine,
    r.annullato,
    NULLIF(r.data_scadenza_ordine, '0001-01-01 00:00:00'::timestamp) AS data_scadenza_ordine,
    r.quantita * r.canone AS mrc,
    p.famiglia, p.sotto_famiglia, p.desc_conto_ricavo AS conto_ricavo,
    CASE
        WHEN o.stato_ordine = 'Cessato' THEN 'Cessata'
        WHEN o.stato_ordine = 'Bloccato' THEN 'Bloccata'
        WHEN o.stato_ordine = 'Confermato' AND date_part('year', r.data_attivazione) = 1 THEN 'Da attivare'
        WHEN o.stato_ordine = 'Confermato' AND date_part('year', r.data_attivazione) > 1 THEN 'Attiva'
        WHEN r.annullato = 1 THEN 'Annullata'
        WHEN date_part('year', r.data_cessazione) = 1 THEN 'Attiva'
        WHEN r.data_cessazione >= '0001-01-01'::timestamp AND r.data_cessazione <= now() THEN 'Cessata'
        WHEN r.data_cessazione > now() THEN 'Cessazione richiesta'
        ELSE 'Unknown'
    END AS stato_riga,
    o.nome_testata_ordine || ' del ' || to_char(o.data_documento, 'YYYY-MM-DD') || ' (' || o.stato_ordine || ')' AS intestazione_ordine,
    CASE
        WHEN r.descrizione_prodotto = r.descrizione_estesa OR r.descrizione_estesa IS NULL OR r.descrizione_estesa = '' THEN r.descrizione_prodotto
        ELSE r.descrizione_prodotto || chr(13) || chr(10) || r.descrizione_estesa
    END AS descrizione_long,
    loader.get_reverse_order_history_path(o.nome_testata_ordine) AS storico
FROM loader.erp_ordini o
  JOIN loader.erp_righe_ordini r ON o.id_gamma = r.id_gamma_testata
  JOIN loader.erp_anagrafiche_clienti c ON o.numero_azienda = c.numero_azienda
  LEFT JOIN loader.erp_anagrafica_articoli_vendita p ON r.codice_prodotto = btrim(p.cod_articolo)
WHERE c.numero_azienda = $1
  AND o.stato_ordine IN (%s)
  AND r.codice_prodotto <> 'CDL-AUTO'
ORDER BY o.nome_testata_ordine, o.data_documento DESC`, placeholders)

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
		Quantita                   int     `json:"quantita"`
		CodiceKit                  *string `json:"codice_kit"`
		CodiceProdotto             *string `json:"codice_prodotto"`
		DescrizioneProdotto        *string `json:"descrizione_prodotto"`
		DescrizioneEstesa          *string `json:"descrizione_estesa"`
		Serialnumber               *string `json:"serialnumber"`
		Setup                      float64 `json:"setup"`
		Canone                     float64 `json:"canone"`
		Valuta                     *string `json:"valuta"`
		CostoCessazione            float64 `json:"costo_cessazione"`
		DataAttivazione            *string `json:"data_attivazione"`
		DataDisdetta               *string `json:"data_disdetta"`
		DataCessazione             *string `json:"data_cessazione"`
		RaggruppamentoFatturazione *string `json:"raggruppamento_fatturazione"`
		IntervalloFattAttivazione  *string `json:"intervallo_fatt_attivazione"`
		IntervalloFattCanone       *string `json:"intervallo_fatt_canone"`
		DataUltimaFatt             *string `json:"data_ultima_fatt"`
		DataFineFatt               *string `json:"data_fine_fatt"`
		SystemOdvRow               *string `json:"system_odv_row"`
		IDGammaTestata             *string `json:"id_gamma_testata"`
		ProgressivoRiga            int     `json:"progressivo_riga"`
		Ordine                     *string `json:"ordine"`
		Annullato                  int     `json:"annullato"`
		DataScadenzaOrdine         *string `json:"data_scadenza_ordine"`
		MRC                        float64 `json:"mrc"`
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
			ordine, dataScadenza                                      sql.NullString
			famiglia, sottoFamiglia, contoRicavo                      sql.NullString
			intOrdine, descLong, storico                              sql.NullString
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
			&d.Quantita, &codiceKit, &codiceProdotto, &descProdotto, &descEstesa,
			&serialnumber, &d.Setup, &d.Canone, &valuta, &d.CostoCessazione,
			&dataAtt, &dataDisdetta, &dataCess,
			&raggFatt, &intFattAtt, &intFattCanone,
			&dataUltFatt, &dataFineFatt, &sysOdvRow, &idGammaTestata, &d.ProgressivoRiga,
			&ordine, &d.Annullato, &dataScadenza, &d.MRC,
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
		d.Ordine = nullStringPtr(ordine)
		d.DataScadenzaOrdine = nullStringPtr(dataScadenza)
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

func nullStringValue(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
