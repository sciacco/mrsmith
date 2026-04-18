package afctools

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// SalesOrderSummary mirrors Select_Orders_Table (spec §B.4.9).
type SalesOrderSummary struct {
	ID             int64      `json:"id"`
	CdlanTipodoc   *string    `json:"cdlan_tipodoc"`
	CdlanNdoc      *string    `json:"cdlan_ndoc"`
	CdlanAnno      *int64     `json:"cdlan_anno"`
	CodiceOrdine   *string    `json:"codice_ordine"`
	CdlanSostOrd   *string    `json:"cdlan_sost_ord"`
	CdlanCliente   *string    `json:"cdlan_cliente"`
	CdlanDatadoc   *time.Time `json:"cdlan_datadoc"`
	TipoDiServizi  *string    `json:"tipo_di_servizi"`
	TipoDiOrdine   *string    `json:"tipo_di_ordine"`
	CdlanDataconf  *time.Time `json:"cdlan_dataconferma"`
	CdlanStato     *string    `json:"cdlan_stato"`
	DalCP          *string    `json:"dal_cp"`
}

// OrderHeader mirrors the Dettaglio ordini "Order" query (spec §B.4.10).
type OrderHeader struct {
	ID                          int64      `json:"id"`
	CdlanSystemodv              *string    `json:"cdlan_systemodv"`
	CdlanTipodoc                *string    `json:"cdlan_tipodoc"`
	CdlanNdoc                   *string    `json:"cdlan_ndoc"`
	CdlanDatadoc                *time.Time `json:"cdlan_datadoc"`
	CdlanCliente                *string    `json:"cdlan_cliente"`
	CdlanCommerciale            *string    `json:"cdlan_commerciale"`
	CdlanCodTerminiPag          *int64     `json:"cdlan_cod_termini_pag"`
	CdlanNote                   *string    `json:"cdlan_note"`
	CdlanTipoOrd                *string    `json:"cdlan_tipo_ord"`
	CdlanDurRin                 *int64     `json:"cdlan_dur_rin"`
	CdlanTacitoRin              *int64     `json:"cdlan_tacito_rin"`
	CdlanSostOrd                *string    `json:"cdlan_sost_ord"`
	CdlanTempiRil               *string    `json:"cdlan_tempi_ril"`
	CdlanDurataServizio         *string    `json:"cdlan_durata_servizio"`
	CdlanDataconferma           *time.Time `json:"cdlan_dataconferma"`
	CdlanRifOrdcli              *string    `json:"cdlan_rif_ordcli"`
	CdlanRifTechNom             *string    `json:"cdlan_rif_tech_nom"`
	CdlanRifTechTel             *string    `json:"cdlan_rif_tech_tel"`
	CdlanRifTechEmail           *string    `json:"cdlan_rif_tech_email"`
	CdlanRifAltroTechNom        *string    `json:"cdlan_rif_altro_tech_nom"`
	CdlanRifAltroTechTel        *string    `json:"cdlan_rif_altro_tech_tel"`
	CdlanRifAltroTechEmail      *string    `json:"cdlan_rif_altro_tech_email"`
	CdlanRifAdmNom              *string    `json:"cdlan_rif_adm_nom"`
	CdlanRifAdmTechTel          *string    `json:"cdlan_rif_adm_tech_tel"`
	CdlanRifAdmTechEmail        *string    `json:"cdlan_rif_adm_tech_email"`
	CdlanIntFatturazioneDesc    *string    `json:"cdlan_int_fatturazione_desc"`
	CdlanIntFatturazione        *int64     `json:"cdlan_int_fatturazione"`
	CdlanIntFatturazioneAttDesc *string    `json:"cdlan_int_fatturazione_att_desc"`
	CdlanIntFatturazioneAtt     *int64     `json:"cdlan_int_fatturazione_att"`
	CdlanStato                  *string    `json:"cdlan_stato"`
	CdlanEvaso                  *int64     `json:"cdlan_evaso"`
	CdlanChiuso                 *int64     `json:"cdlan_chiuso"`
	CdlanAnno                   *int64     `json:"cdlan_anno"`
	CdlanValuta                 *string    `json:"cdlan_valuta"`
	WrittenBy                   *string    `json:"written_by"`
	ProfileIVA                  *string    `json:"profile_iva"`
	ProfileCF                   *string    `json:"profile_cf"`
	ProfileAddress              *string    `json:"profile_address"`
	ProfileCity                 *string    `json:"profile_city"`
	ProfileCAP                  *string    `json:"profile_cap"`
	ProfilePV                   *string    `json:"profile_pv"`
	ProfileSDI                  *string    `json:"profile_sdi"`
	ProfileLang                 *string    `json:"profile_lang"`
	CdlanClienteID              *int64     `json:"cdlan_cliente_id"`
	ServiceType                 *string    `json:"service_type"`
	DataDecorrenza              *string    `json:"data_decorrenza"`
	CdlanTacitoRinInPdf         *int64     `json:"cdlan_tacito_rin_in_pdf"`
	IsColo                      *string    `json:"is_colo"`
	OriginCodTerminiPag         *int64     `json:"origin_cod_termini_pag"`
	IsArxivar                   *int64     `json:"is_arxivar"`
	FromCP                      *int64     `json:"from_cp"`
	ArxDocNumber                *string    `json:"arx_doc_number"`
}

// OrderRow mirrors RigheOrdine (spec §B.4.11).
type OrderRow struct {
	IDRiga                   int64      `json:"id_riga"`
	SystemODVRiga            *string    `json:"system_odv_riga"`
	CodiceArticoloBundle     *string    `json:"codice_articolo_bundle"`
	CodiceArticolo           *string    `json:"codice_articolo"`
	DescrizioneArticolo      *string    `json:"descrizione_articolo"`
	Canone                   *float64   `json:"canone"`
	Attivazione              *float64   `json:"attivazione"`
	Quantita                 *float64   `json:"quantita"`
	PrezzoCessazione         *float64   `json:"prezzo_cessazione"`
	CodRaggFatt              *string    `json:"codice_raggruppamento_fatturazione"`
	DataAttivazione          *time.Time `json:"data_attivazione"`
	NumeroSeriale            *string    `json:"numero_seriale"`
	ConfirmDataAttivazione   *time.Time `json:"confirm_data_attivazione"`
	DataAnnullamento         *time.Time `json:"data_annullamento"`
}

func (h *Handler) listOrders(r *http.Request) ([]SalesOrderSummary, error) {
	const query = `
SELECT id,
       cdlan_tipodoc,
       cdlan_ndoc,
       cdlan_anno,
       CONCAT(cdlan_ndoc, '/', cdlan_anno) AS codice_ordine,
       cdlan_sost_ord,
       cdlan_cliente,
       cdlan_datadoc,
       IF(is_colo != 0, is_colo, service_type) AS tipo_di_servizi,
       CASE cdlan_tipo_ord
           WHEN 'A' THEN 'Sostituzione'
           WHEN 'N' THEN 'Nuovo'
           WHEN 'R' THEN 'Rinnovo'
           ELSE NULL
       END AS tipo_di_ordine,
       cdlan_dataconferma,
       cdlan_stato,
       IF(from_cp != 0, 'Sì', 'No') AS dal_cp
FROM orders
WHERE cdlan_stato IN ('ATTIVO', 'INVIATO')
ORDER BY cdlan_datadoc DESC
`
	rows, err := h.deps.Vodka.QueryContext(r.Context(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SalesOrderSummary, 0)
	for rows.Next() {
		var s SalesOrderSummary
		if err := rows.Scan(
			&s.ID, &s.CdlanTipodoc, &s.CdlanNdoc, &s.CdlanAnno,
			&s.CodiceOrdine, &s.CdlanSostOrd, &s.CdlanCliente,
			&s.CdlanDatadoc, &s.TipoDiServizi, &s.TipoDiOrdine,
			&s.CdlanDataconf, &s.CdlanStato, &s.DalCP,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Handler) handleOrders(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w, h.deps.Vodka, "vodka") {
		return
	}
	rowsOut, err := h.listOrders(r)
	if err != nil {
		h.dbFailure(w, r, "list_orders", err)
		return
	}
	httputil.JSON(w, http.StatusOK, rowsOut)
}

func (h *Handler) getOrder(r *http.Request, id int64) (*OrderHeader, error) {
	const query = `
SELECT id,
       cdlan_systemodv, cdlan_tipodoc, cdlan_ndoc, cdlan_datadoc,
       cdlan_cliente, cdlan_commerciale, cdlan_cod_termini_pag,
       cdlan_note, cdlan_tipo_ord, cdlan_dur_rin, cdlan_tacito_rin,
       cdlan_sost_ord, cdlan_tempi_ril, cdlan_durata_servizio, cdlan_dataconferma,
       cdlan_rif_ordcli, cdlan_rif_tech_nom, cdlan_rif_tech_tel, cdlan_rif_tech_email,
       cdlan_rif_altro_tech_nom, cdlan_rif_altro_tech_tel, cdlan_rif_altro_tech_email,
       cdlan_rif_adm_nom, cdlan_rif_adm_tech_tel, cdlan_rif_adm_tech_email,
       CASE cdlan_int_fatturazione
           WHEN 1 THEN 'Mensile'
           WHEN 2 THEN 'Bimestrale'
           WHEN 3 THEN 'Trimestrale'
           WHEN 5 THEN 'Quadrimestrale'
           WHEN 6 THEN 'Semestrale'
           ELSE 'Annuale'
       END AS cdlan_int_fatturazione_desc,
       cdlan_int_fatturazione,
       CASE cdlan_int_fatturazione_att
           WHEN 1 THEN 'All''ordine'
           ELSE 'All''attivazione della Soluzione/Consegna'
       END AS cdlan_int_fatturazione_att_desc,
       cdlan_int_fatturazione_att,
       cdlan_stato, cdlan_evaso, cdlan_chiuso, cdlan_anno, cdlan_valuta,
       written_by, profile_iva, profile_cf, profile_address, profile_city,
       profile_cap, profile_pv, profile_sdi, profile_lang,
       cdlan_cliente_id, service_type, data_decorrenza,
       cdlan_tacito_rin_in_pdf, is_colo, origin_cod_termini_pag,
       is_arxivar, from_cp, arx_doc_number
FROM orders
WHERE id = ?
LIMIT 1
`
	row := h.deps.Vodka.QueryRowContext(r.Context(), query, id)

	var o OrderHeader
	if err := row.Scan(
		&o.ID, &o.CdlanSystemodv, &o.CdlanTipodoc, &o.CdlanNdoc, &o.CdlanDatadoc,
		&o.CdlanCliente, &o.CdlanCommerciale, &o.CdlanCodTerminiPag,
		&o.CdlanNote, &o.CdlanTipoOrd, &o.CdlanDurRin, &o.CdlanTacitoRin,
		&o.CdlanSostOrd, &o.CdlanTempiRil, &o.CdlanDurataServizio, &o.CdlanDataconferma,
		&o.CdlanRifOrdcli, &o.CdlanRifTechNom, &o.CdlanRifTechTel, &o.CdlanRifTechEmail,
		&o.CdlanRifAltroTechNom, &o.CdlanRifAltroTechTel, &o.CdlanRifAltroTechEmail,
		&o.CdlanRifAdmNom, &o.CdlanRifAdmTechTel, &o.CdlanRifAdmTechEmail,
		&o.CdlanIntFatturazioneDesc, &o.CdlanIntFatturazione,
		&o.CdlanIntFatturazioneAttDesc, &o.CdlanIntFatturazioneAtt,
		&o.CdlanStato, &o.CdlanEvaso, &o.CdlanChiuso, &o.CdlanAnno, &o.CdlanValuta,
		&o.WrittenBy, &o.ProfileIVA, &o.ProfileCF, &o.ProfileAddress, &o.ProfileCity,
		&o.ProfileCAP, &o.ProfilePV, &o.ProfileSDI, &o.ProfileLang,
		&o.CdlanClienteID, &o.ServiceType, &o.DataDecorrenza,
		&o.CdlanTacitoRinInPdf, &o.IsColo, &o.OriginCodTerminiPag,
		&o.IsArxivar, &o.FromCP, &o.ArxDocNumber,
	); err != nil {
		return nil, err
	}
	return &o, nil
}

func (h *Handler) handleOrderHeader(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w, h.deps.Vodka, "vodka") {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_order_id")
		return
	}
	order, err := h.getOrder(r, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "order_not_found")
			return
		}
		h.dbFailure(w, r, "get_order", err)
		return
	}
	httputil.JSON(w, http.StatusOK, order)
}

func (h *Handler) listOrderRows(r *http.Request, id int64) ([]OrderRow, error) {
	const query = `
SELECT id                                AS id_riga,
       cdlan_systemodv_row               AS system_odv_riga,
       IF(cdlan_codice_kit != '', CONCAT(cdlan_codice_kit, '-', index_kit), '') AS codice_articolo_bundle,
       cdlan_codart                      AS codice_articolo,
       cdlan_descart                     AS descrizione_articolo,
       cdlan_prezzo                      AS canone,
       cdlan_prezzo_attivazione          AS attivazione,
       cdlan_qta                         AS quantita,
       cdlan_prezzo_cessazione           AS prezzo_cessazione,
       cdlan_ragg_fatturazione           AS codice_raggruppamento_fatturazione,
       cdlan_data_attivazione            AS data_attivazione,
       cdlan_serialnumber                AS numero_seriale,
       confirm_data_attivazione,
       data_annullamento
FROM orders_rows
WHERE orders_id = ?
`
	rows, err := h.deps.Vodka.QueryContext(r.Context(), query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]OrderRow, 0)
	for rows.Next() {
		var or OrderRow
		if err := rows.Scan(
			&or.IDRiga, &or.SystemODVRiga, &or.CodiceArticoloBundle,
			&or.CodiceArticolo, &or.DescrizioneArticolo,
			&or.Canone, &or.Attivazione, &or.Quantita, &or.PrezzoCessazione,
			&or.CodRaggFatt, &or.DataAttivazione, &or.NumeroSeriale,
			&or.ConfirmDataAttivazione, &or.DataAnnullamento,
		); err != nil {
			return nil, err
		}
		out = append(out, or)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Handler) handleOrderRows(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w, h.deps.Vodka, "vodka") {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_order_id")
		return
	}
	rowsOut, err := h.listOrderRows(r, id)
	if err != nil {
		h.dbFailure(w, r, "list_order_rows", err)
		return
	}
	httputil.JSON(w, http.StatusOK, rowsOut)
}
