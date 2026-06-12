package ordini

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var errGatewayPreconditionMissing = errors.New("gateway precondition missing")

type gatewayHTTPError struct {
	Status int
	Body   string
}

func (e *gatewayHTTPError) Error() string {
	return fmt.Sprintf("gateway returned HTTP %d", e.Status)
}

func (e *gatewayHTTPError) Is(target error) bool {
	if target != errGatewayPreconditionMissing {
		return false
	}
	return gatewayBodyHasCode(e.Body, "precondition_missing")
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
		return &gatewayHTTPError{Status: resp.StatusCode, Body: string(bodyBytes)}
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
		return errGatewayPreconditionMissing
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
		return &gatewayHTTPError{Status: resp.StatusCode, Body: string(bodyBytes)}
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// buildSendToERPPayload mirrors the legacy Appsmith GW_SendToErp payload
// field-for-field, including JSON value types: the legacy app used smart
// substitution, so the gateway struct expects the JS types the legacy app
// produced — vodka varchar columns stay strings even when numeric-looking
// (cdlan_anno, cdlan_qta, cdlan_prezzo*), parseInt()'d fields stay numbers,
// and date-typed fields the legacy app never sent (cdlan_data_attivazione,
// data_annullamento, data_decorrenza) must stay absent because the gateway
// rejects "" as a date. cdlan_codice_kit carries the kit-index composite
// (bundle_code), not the raw kit code.
func buildSendToERPPayload(order *OrderDetail, row OrderRow) (map[string]any, error) {
	systemODV, ok := parseRequiredInt(ptrStringValue(order.CdlanSystemODV))
	if !ok || row.CdlanSystemODVRow == nil {
		return nil, errGatewayPreconditionMissing
	}
	payload := map[string]any{
		"cdlan_systemodv":            systemODV,
		"cdlan_systemodv_row":        *row.CdlanSystemODVRow,
		"cdlan_tipodoc":              ptrStringValue(order.CdlanTipodoc),
		"cdlan_ndoc":                 ptrStringValue(order.CdlanNdoc),
		"cdlan_datadoc":              order.CdlanDatadoc.String(),
		"cdlan_cliente":              ptrStringValue(order.CdlanCliente),
		"cdlan_commerciale":          stringValueOrSpace(order.CdlanCommerciale),
		"cdlan_cod_termini_pag":      ptrStringValue(order.CdlanCodTerminiPag),
		"cdlan_note":                 ptrStringValue(order.CdlanNote),
		"cdlan_tipo_ord":             ptrStringValue(order.CdlanTipoOrd),
		"cdlan_dur_rin":              intValueOrNull(order.CdlanDurRin),
		"cdlan_tacito_rin":           intValueOrNull(order.CdlanTacitoRin),
		"cdlan_sost_ord":             stringValueOrSpace(order.CdlanSostOrd),
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
		"cdlan_int_fatturazione":     intValueOrNull(order.CdlanIntFatturazione),
		"cdlan_int_fatturazione_att": intValueOrNull(order.CdlanIntFatturazioneAtt),
		"cdlan_codart":               ptrStringValue(row.CdlanCodart),
		"cdlan_descart":              ptrStringValue(row.CdlanDescart),
		"cdlan_qta":                  nullFloatRawValue(row.CdlanQta),
		"cdlan_serialnumber":         ptrStringValue(row.CdlanSerialNumber),
		"cdlan_prezzo":               nullFloatRawValue(row.Canone),
		"cdlan_prezzo_attivazione":   nullFloatRawValue(row.ActivationPrice),
		"cdlan_prezzo_cessazione":    nullFloatRawValue(row.TerminationPrice),
		"cdlan_ragg_fatturazione":    ptrStringValue(row.CdlanRaggFatturazione),
		"cdlan_stato":                "CREATO",
		"cdlan_evaso":                ptrIntValue(order.CdlanEvaso),
		"cdlan_chiuso":               ptrIntValue(order.CdlanChiuso),
		"cdlan_anno":                 ptrIntAsStringValue(order.CdlanAnno),
		"cdlan_codice_kit":           ptrStringValue(row.BundleCode),
		"cdlan_valuta":               ptrStringValue(order.CdlanValuta),
	}
	return payload, nil
}

// stringValueOrSpace mirrors the legacy `?? " "` fallback: the gateway has only
// ever received a single space for these fields when the vodka column is NULL.
func stringValueOrSpace(value *string) string {
	if value == nil {
		return " "
	}
	return *value
}

// intValueOrNull mirrors the legacy parseInt(): the gateway expects a number,
// and JSON null when the value is missing or not numeric.
func intValueOrNull(value *string) any {
	if value == nil {
		return nil
	}
	parsed, ok := parseRequiredInt(*value)
	if !ok {
		return nil
	}
	return parsed
}

// ptrIntAsStringValue renders an int-scanned vodka varchar column back to the
// string the gateway expects (e.g. Orders.cdlan_anno is a Go string upstream).
func ptrIntAsStringValue(value *int64) any {
	if value == nil {
		return nil
	}
	return strconv.FormatInt(*value, 10)
}

// nullFloatRawValue forwards the vodka column content verbatim: the legacy app
// passed cdlan_qta and cdlan_prezzo* through untouched (mixed "10,00" and
// "350.00" formats exist in production), so the gateway declares them as Go
// strings and the ERP must keep receiving the stored format.
func nullFloatRawValue(value NullFloat) any {
	if value.Raw == nil {
		return nil
	}
	return *value.Raw
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

func gatewayBodyHasCode(body, code string) bool {
	text := strings.TrimSpace(body)
	if text == "" {
		return false
	}
	var payload map[string]any
	if json.Unmarshal([]byte(text), &payload) == nil {
		for _, key := range []string{"error", "code", "message"} {
			if value, ok := payload[key].(string); ok && value == code {
				return true
			}
		}
	}
	return text == code
}

func gatewayBodyCode(body string) string {
	text := strings.TrimSpace(body)
	if text == "" {
		return ""
	}
	if isSafeGatewayCode(text) {
		return text
	}
	var payload map[string]any
	if json.Unmarshal([]byte(text), &payload) == nil {
		for _, key := range []string{"error", "code"} {
			if value, ok := payload[key].(string); ok && isSafeGatewayCode(value) {
				return value
			}
		}
	}
	return ""
}

func gatewayBodyMessage(body string) string {
	text := strings.TrimSpace(body)
	if text == "" {
		return ""
	}
	var payload map[string]any
	if json.Unmarshal([]byte(text), &payload) != nil {
		return ""
	}
	for _, key := range []string{"message", "error_description"} {
		if value, ok := payload[key].(string); ok {
			value = strings.Join(strings.Fields(value), " ")
			if len(value) > 256 {
				runes := []rune(value)
				if len(runes) > 256 {
					value = string(runes[:256]) + "..."
				}
			}
			return value
		}
	}
	return ""
}

func isSafeGatewayCode(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.':
		default:
			return false
		}
	}
	return true
}

func gatewayFailureAttrs(path string, err error, attrs ...any) []any {
	args := []any{"gw_path", path}
	var httpErr *gatewayHTTPError
	if errors.As(err, &httpErr) {
		args = append(args, "upstream_status", httpErr.Status)
		if code := gatewayBodyCode(httpErr.Body); code != "" {
			args = append(args, "upstream_code", code)
		}
		if httpErr.Body != "" {
			args = append(args, "upstream_body", httpErr.Body)
		}
	}
	args = append(args, attrs...)
	args = append(args, "error", err)
	return args
}
