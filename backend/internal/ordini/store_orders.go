package ordini

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const orderSummarySelect = `
SELECT id,
       cdlan_systemodv,
       cdlan_tipodoc,
       cdlan_ndoc,
       cdlan_anno,
       CONCAT(cdlan_ndoc, '/', cdlan_anno) AS codice_ordine,
       cdlan_sost_ord,
       cdlan_cliente,
       cdlan_cliente_id,
       cdlan_datadoc,
       service_type,
       is_colo,
       cdlan_tipo_ord,
       cdlan_dataconferma,
       cdlan_stato,
       profile_lang,
       cdlan_evaso,
       from_cp,
       arx_doc_number
`

func (h *Handler) listOrders(r *http.Request) ([]OrderSummary, error) {
	rows, err := h.deps.Vodka.QueryContext(r.Context(), orderSummarySelect+`
FROM orders
ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]OrderSummary, 0)
	for rows.Next() {
		var item OrderSummary
		if err := scanOrderSummary(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanOrderSummary(scanner interface{ Scan(dest ...any) error }, item *OrderSummary) error {
	return scanner.Scan(
		&item.ID,
		&item.CdlanSystemODV,
		&item.CdlanTipodoc,
		&item.CdlanNdoc,
		&item.CdlanAnno,
		&item.CodiceOrdine,
		&item.CdlanSostOrd,
		&item.CdlanCliente,
		&item.CdlanClienteID,
		&item.CdlanDatadoc,
		&item.ServiceType,
		&item.IsColo,
		&item.CdlanTipoOrd,
		&item.CdlanDataconferma,
		&item.CdlanStato,
		&item.ProfileLang,
		&item.CdlanEvaso,
		&item.FromCP,
		&item.ArxDocNumber,
	)
}

func (h *Handler) getOrder(r *http.Request, id int64) (*OrderDetail, error) {
	order, err := h.getOrderWithoutOrigin(r, id)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	origin, err := h.loadOrigin(r, id)
	if err != nil {
		h.logFailure(r, slog.LevelWarn, "origin lookup failed", "origin_lookup", start, "order_id", id, "error", err)
	} else {
		order.Origin = origin
	}
	return order, nil
}

func (h *Handler) getOrderWithoutOrigin(r *http.Request, id int64) (*OrderDetail, error) {
	row := h.deps.Vodka.QueryRowContext(r.Context(), orderSummarySelect+`,
       cdlan_commerciale,
       cdlan_cod_termini_pag,
       COALESCE(cdlan_note, '') AS cdlan_note,
       cdlan_dur_rin,
       cdlan_tacito_rin,
       cdlan_tempi_ril,
       cdlan_durata_servizio,
       cdlan_rif_ordcli,
       cdlan_rif_tech_nom,
       cdlan_rif_tech_tel,
       cdlan_rif_tech_email,
       cdlan_rif_altro_tech_nom,
       cdlan_rif_altro_tech_tel,
       cdlan_rif_altro_tech_email,
       cdlan_rif_adm_nom,
       cdlan_rif_adm_tech_tel,
       cdlan_rif_adm_tech_email,
       cdlan_int_fatturazione,
       cdlan_int_fatturazione_att,
       cdlan_chiuso,
       cdlan_valuta,
       written_by,
       profile_iva,
       profile_cf,
       profile_address,
       profile_city,
       profile_cap,
       profile_pv,
       profile_sdi,
       data_decorrenza,
       cdlan_tacito_rin_in_pdf,
       origin_cod_termini_pag,
       is_arxivar
FROM orders
WHERE id = ?
LIMIT 1`, id)

	var order OrderDetail
	if err := row.Scan(
		&order.ID,
		&order.CdlanSystemODV,
		&order.CdlanTipodoc,
		&order.CdlanNdoc,
		&order.CdlanAnno,
		&order.CodiceOrdine,
		&order.CdlanSostOrd,
		&order.CdlanCliente,
		&order.CdlanClienteID,
		&order.CdlanDatadoc,
		&order.ServiceType,
		&order.IsColo,
		&order.CdlanTipoOrd,
		&order.CdlanDataconferma,
		&order.CdlanStato,
		&order.ProfileLang,
		&order.CdlanEvaso,
		&order.FromCP,
		&order.ArxDocNumber,
		&order.CdlanCommerciale,
		&order.CdlanCodTerminiPag,
		&order.CdlanNote,
		&order.CdlanDurRin,
		&order.CdlanTacitoRin,
		&order.CdlanTempiRil,
		&order.CdlanDurataServizio,
		&order.CdlanRifOrdcli,
		&order.CdlanRifTechNom,
		&order.CdlanRifTechTel,
		&order.CdlanRifTechEmail,
		&order.CdlanRifAltroTechNom,
		&order.CdlanRifAltroTechTel,
		&order.CdlanRifAltroTechEmail,
		&order.CdlanRifAdmNom,
		&order.CdlanRifAdmTechTel,
		&order.CdlanRifAdmTechEmail,
		&order.CdlanIntFatturazione,
		&order.CdlanIntFatturazioneAtt,
		&order.CdlanChiuso,
		&order.CdlanValuta,
		&order.WrittenBy,
		&order.ProfileIVA,
		&order.ProfileCF,
		&order.ProfileAddress,
		&order.ProfileCity,
		&order.ProfileCAP,
		&order.ProfilePV,
		&order.ProfileSDI,
		&order.DataDecorrenza,
		&order.CdlanTacitoRinInPDF,
		&order.OriginCodTerminiPag,
		&order.IsArxivar,
	); err != nil {
		return nil, err
	}
	return &order, nil
}

func (h *Handler) handleListOrders(w http.ResponseWriter, r *http.Request) {
	if !h.requireVodka(w) {
		return
	}
	items, err := h.listOrders(r)
	if err != nil {
		h.dbFailure(w, r, "list_orders", err)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	if !h.requireVodka(w) {
		return
	}
	id, ok := h.parseOrderID(w, r)
	if !ok {
		return
	}
	order, err := h.getOrder(r, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "order_not_found")
			return
		}
		h.dbFailure(w, r, "get_order", err, "order_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, order)
}

func (h *Handler) handlePatchOrderHeader(w http.ResponseWriter, r *http.Request) {
	if !h.requireCustomerRelations(w, r) || !h.requireVodka(w) || !h.requireAlyante(w) {
		return
	}
	id, ok := h.parseOrderID(w, r)
	if !ok {
		return
	}
	payload, ok := decodeJSON[UpdateHeaderRequest](w, r)
	if !ok {
		return
	}
	if payload.CustomerID <= 0 {
		httputil.Error(w, http.StatusUnprocessableEntity, "missing_customer")
		return
	}
	confirmationDate, ok := confirmationDateOrNil(payload.ConfirmationDate)
	if !ok {
		httputil.Error(w, http.StatusUnprocessableEntity, "invalid_confirmation_date")
		return
	}
	order, err := h.getOrderWithoutOrigin(r, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "order_not_found")
			return
		}
		h.dbFailure(w, r, "get_order_for_header_patch", err, "order_id", id)
		return
	}
	if !requireState(w, stateOf(order), OrderStateBozza) {
		return
	}
	customer, err := h.getCustomerByID(r.Context(), payload.CustomerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "customer_not_found")
			return
		}
		h.dbFailure(w, r, "get_customer", err, "customer_id", payload.CustomerID)
		return
	}
	res, err := h.deps.Vodka.ExecContext(r.Context(), `
UPDATE orders
SET cdlan_rif_ordcli = ?,
    cdlan_dataconferma = ?,
    cdlan_cliente_id = ?,
    cdlan_cliente = ?
WHERE id = ? AND cdlan_stato = 'BOZZA'`, nullIfBlank(payload.CustomerPO), confirmationDate, customer.ID, customer.Name, id)
	if err != nil {
		h.dbFailure(w, r, "patch_order_header", err, "order_id", id)
		return
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		httputil.Error(w, http.StatusConflict, "wrong_state")
		return
	}
	h.writeOrderOrNotFound(w, r, id)
}

func (h *Handler) handlePatchReferents(w http.ResponseWriter, r *http.Request) {
	if !h.requireCustomerRelations(w, r) || !h.requireVodka(w) {
		return
	}
	id, ok := h.parseOrderID(w, r)
	if !ok {
		return
	}
	payload, ok := decodeJSON[UpdateReferentsRequest](w, r)
	if !ok {
		return
	}
	order, err := h.getOrderWithoutOrigin(r, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "order_not_found")
			return
		}
		h.dbFailure(w, r, "get_order_for_referents_patch", err, "order_id", id)
		return
	}
	if !requireState(w, stateOf(order), OrderStateBozza, OrderStateInviato) {
		return
	}
	res, err := h.deps.Vodka.ExecContext(r.Context(), `
UPDATE orders
SET cdlan_rif_tech_nom = ?,
    cdlan_rif_tech_tel = ?,
    cdlan_rif_tech_email = ?,
    cdlan_rif_altro_tech_nom = ?,
    cdlan_rif_altro_tech_tel = ?,
    cdlan_rif_altro_tech_email = ?,
    cdlan_rif_adm_nom = ?,
    cdlan_rif_adm_tech_tel = ?,
    cdlan_rif_adm_tech_email = ?
WHERE id = ?
  AND cdlan_stato IN ('BOZZA', 'INVIATO')`, nullIfBlank(payload.TechnicalName), nullIfBlank(payload.TechnicalPhone), nullIfBlank(payload.TechnicalEmail), nullIfBlank(payload.OtherTechnicalName), nullIfBlank(payload.OtherTechnicalPhone), nullIfBlank(payload.OtherTechnicalEmail), nullIfBlank(payload.AdminName), nullIfBlank(payload.AdminPhone), nullIfBlank(payload.AdminEmail), id)
	if err != nil {
		h.dbFailure(w, r, "patch_order_referents", err, "order_id", id)
		return
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		httputil.Error(w, http.StatusConflict, "wrong_state")
		return
	}
	h.writeOrderOrNotFound(w, r, id)
}

func confirmationDateOrNil(value string) (any, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, true
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return nil, false
	}
	return parsed.Format("2006-01-02"), true
}
