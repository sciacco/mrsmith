package ordini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type gatewayHTTPError struct {
	Status int
	Body   string
}

func (e *gatewayHTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("gateway returned HTTP %d", e.Status)
	}
	return fmt.Sprintf("gateway returned HTTP %d: %s", e.Status, e.Body)
}

func (h *Handler) gatewayPostJSON(path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := h.deps.Arak.Do(http.MethodPost, path, "", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return &gatewayHTTPError{Status: resp.StatusCode, Body: compactGatewayBody(bodyBytes)}
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (h *Handler) gatewaySendToERP(order *OrderDetail, row OrderRow) error {
	payload, err := buildSendToERPPayload(order, row)
	if err != nil {
		return err
	}
	return h.gatewayPostJSON("/orders/v1/erp", payload)
}

func (h *Handler) gatewaySetActivationDate(order *OrderDetail, row OrderRow, activationDate string) error {
	systemODV, ok := parseRequiredInt(ptrStringValue(order.CdlanSystemODV))
	if !ok || row.CdlanSystemODVRow == nil {
		return fmt.Errorf("precondition_missing")
	}
	payload := map[string]any{
		"cdlan_systemodv":        systemODV,
		"cdlan_systemodv_row":    *row.CdlanSystemODVRow,
		"cdlan_data_attivazione": activationDate,
	}
	return h.gatewayPostJSON("/orders/v1/set-order-activation", payload)
}

func (h *Handler) gatewayUploadToArxivar(order *OrderDetail, pdf []byte, filename string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(pdf); err != nil {
		return err
	}
	if err := writer.WriteField("orderId", strconv.FormatInt(order.ID, 10)); err != nil {
		return err
	}
	if err := writer.WriteField("filename", filename); err != nil {
		return err
	}
	if err := writer.WriteField("multipart", "application/pdf"); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	headers := http.Header{}
	headers.Set("Content-Type", writer.FormDataContentType())
	resp, err := h.deps.Arak.DoWithHeaders(http.MethodPost, "/orders/v1/send-to-arxivar", "", &body, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return &gatewayHTTPError{Status: resp.StatusCode, Body: compactGatewayBody(bodyBytes)}
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func buildSendToERPPayload(order *OrderDetail, row OrderRow) (map[string]any, error) {
	systemODV, ok := parseRequiredInt(ptrStringValue(order.CdlanSystemODV))
	if !ok || row.CdlanSystemODVRow == nil {
		return nil, fmt.Errorf("precondition_missing")
	}
	payload := map[string]any{
		"order_id":                   order.ID,
		"orders_id":                  row.OrderID,
		"cdlan_systemodv":            systemODV,
		"cdlan_systemodv_row":        *row.CdlanSystemODVRow,
		"cdlan_tipodoc":              ptrStringValue(order.CdlanTipodoc),
		"cdlan_ndoc":                 ptrStringValue(order.CdlanNdoc),
		"cdlan_anno":                 ptrIntValue(order.CdlanAnno),
		"cdlan_datadoc":              order.CdlanDatadoc.String(),
		"cdlan_cliente":              ptrStringValue(order.CdlanCliente),
		"cdlan_cliente_id":           ptrIntValue(order.CdlanClienteID),
		"cdlan_commerciale":          ptrStringValue(order.CdlanCommerciale),
		"cdlan_cod_termini_pag":      ptrStringValue(order.CdlanCodTerminiPag),
		"cdlan_note":                 ptrStringValue(order.CdlanNote),
		"cdlan_tipo_ord":             ptrStringValue(order.CdlanTipoOrd),
		"cdlan_dur_rin":              ptrStringValue(order.CdlanDurRin),
		"cdlan_tacito_rin":           ptrStringValue(order.CdlanTacitoRin),
		"cdlan_sost_ord":             ptrStringValue(order.CdlanSostOrd),
		"cdlan_tempi_ril":            ptrStringValue(order.CdlanTempiRil),
		"cdlan_durata_servizio":      ptrStringValue(order.CdlanDurataServizio),
		"cdlan_dataconferma":         order.CdlanDataconferma.String(),
		"cdlan_rif_ordcli":           ptrStringValue(order.CdlanRifOrdcli),
		"cdlan_rif_tech_nom":         ptrStringValue(order.CdlanRifTechNom),
		"cdlan_rif_tech_tel":         ptrStringValue(order.CdlanRifTechTel),
		"cdlan_rif_tech_email":       ptrStringValue(order.CdlanRifTechEmail),
		"cdlan_rif_altro_tech_nom":   ptrStringValue(order.CdlanRifAltroTechNom),
		"cdlan_rif_altro_tech_tel":   ptrStringValue(order.CdlanRifAltroTechTel),
		"cdlan_rif_altro_tech_email": ptrStringValue(order.CdlanRifAltroTechEmail),
		"cdlan_rif_adm_nom":          ptrStringValue(order.CdlanRifAdmNom),
		"cdlan_rif_adm_tech_tel":     ptrStringValue(order.CdlanRifAdmTechTel),
		"cdlan_rif_adm_tech_email":   ptrStringValue(order.CdlanRifAdmTechEmail),
		"cdlan_int_fatturazione":     ptrStringValue(order.CdlanIntFatturazione),
		"cdlan_int_fatturazione_att": ptrStringValue(order.CdlanIntFatturazioneAtt),
		"cdlan_stato":                "CREATO",
		"cdlan_evaso":                ptrIntValue(order.CdlanEvaso),
		"cdlan_chiuso":               ptrIntValue(order.CdlanChiuso),
		"cdlan_valuta":               ptrStringValue(order.CdlanValuta),
		"written_by":                 ptrStringValue(order.WrittenBy),
		"profile_iva":                ptrStringValue(order.ProfileIVA),
		"profile_cf":                 ptrStringValue(order.ProfileCF),
		"profile_address":            ptrStringValue(order.ProfileAddress),
		"profile_city":               ptrStringValue(order.ProfileCity),
		"profile_cap":                ptrStringValue(order.ProfileCAP),
		"profile_pv":                 ptrStringValue(order.ProfilePV),
		"profile_sdi":                ptrStringValue(order.ProfileSDI),
		"profile_lang":               ptrStringValue(order.ProfileLang),
		"service_type":               ptrStringValue(order.ServiceType),
		"data_decorrenza":            order.DataDecorrenza.String(),
		"cdlan_tacito_rin_in_pdf":    ptrStringValue(order.CdlanTacitoRinInPDF),
		"is_colo":                    ptrStringValue(order.IsColo),
		"origin_cod_termini_pag":     ptrStringValue(order.OriginCodTerminiPag),
		"is_arxivar":                 ptrIntValue(order.IsArxivar),
		"from_cp":                    ptrIntValue(order.FromCP),
		"cdlan_codice_kit":           ptrStringValue(row.CdlanCodiceKit),
		"index_kit":                  ptrIntValue(row.IndexKit),
		"bundle_code":                ptrStringValue(row.BundleCode),
		"cdlan_codart":               ptrStringValue(row.CdlanCodart),
		"cdlan_descart":              ptrStringValue(row.CdlanDescart),
		"cdlan_qta":                  nullFloatValue(row.CdlanQta),
		"cdlan_serialnumber":         ptrStringValue(row.CdlanSerialNumber),
		"cdlan_prezzo":               nullFloatValue(row.Canone),
		"cdlan_prezzo_attivazione":   nullFloatValue(row.ActivationPrice),
		"cdlan_prezzo_cessazione":    nullFloatValue(row.TerminationPrice),
		"cdlan_ragg_fatturazione":    ptrStringValue(row.CdlanRaggFatturazione),
		"cdlan_data_attivazione":     row.CdlanDataAttivazione.String(),
		"confirm_data_attivazione":   ptrIntValue(row.ConfirmDataAttivazione),
		"data_annullamento":          row.DataAnnullamento.String(),
	}
	return payload, nil
}

func nullFloatValue(value NullFloat) any {
	if !value.Valid {
		return nil
	}
	return value.Float64
}

func gatewayPathWithID(prefix string, id int64, suffix string) string {
	return prefix + url.PathEscape(strconv.FormatInt(id, 10)) + suffix
}

func compactGatewayBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 256 {
		return text[:256] + "..."
	}
	return text
}
