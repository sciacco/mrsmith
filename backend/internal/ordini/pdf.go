package ordini

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleKickoffPDF(w http.ResponseWriter, r *http.Request) {
	h.handleGatedPDF(w, r, pdfGate{
		Operation:     "kickoff_pdf",
		GatewayPath:   func(id int64) string { return gatewayPathWithID("/orders/v1/kick-off/", id, "") },
		Filename:      func(o *OrderDetail) string { return "kick off_" + orderCodeForFilename(o) + ".pdf" },
		RequiresCR:    true,
		AllowedStates: []OrderState{OrderStateInviato},
	})
}

func (h *Handler) handleActivationFormPDF(w http.ResponseWriter, r *http.Request) {
	h.handleGatedPDF(w, r, pdfGate{
		Operation:   "activation_form_pdf",
		GatewayPath: func(id int64) string { return gatewayPathWithID("/orders/v1/activation-form/", id, "") },
		Filename: func(o *OrderDetail) string {
			if strings.EqualFold(ptrStringValue(o.ProfileLang), "en") {
				return "Activation Form_" + orderCodeForFilename(o) + ".pdf"
			}
			return "Modulo di Attivazione_" + orderCodeForFilename(o) + ".pdf"
		},
		RequiresCR:    true,
		AllowedStates: []OrderState{OrderStateInviato, OrderStateAttivo},
	})
}

func (h *Handler) handleOrderPDF(w http.ResponseWriter, r *http.Request) {
	h.handleGatedPDF(w, r, pdfGate{
		Operation:   "order_pdf",
		GatewayPath: func(id int64) string { return gatewayPathWithID("/orders/v1/order/pdf/", id, "/generate") },
		Filename:    func(o *OrderDetail) string { return orderCodeForFilename(o) + ".pdf" },
		Check: func(o *OrderDetail) (int, string, bool) {
			if o.ArxDocNumber != nil && strings.TrimSpace(*o.ArxDocNumber) != "" {
				return http.StatusConflict, "wrong_state", false
			}
			return 0, "", true
		},
	})
}

func (h *Handler) handleSignedPDF(w http.ResponseWriter, r *http.Request) {
	h.handleGatedPDF(w, r, pdfGate{
		Operation:   "signed_pdf",
		GatewayPath: func(id int64) string { return gatewayPathWithID("/orders/v1/order/pdf/", id, "") },
		Query:       "from=vodka",
		Filename:    func(o *OrderDetail) string { return orderCodeForFilename(o) + "_firmato.pdf" },
		Check: func(o *OrderDetail) (int, string, bool) {
			if o.ArxDocNumber == nil || strings.TrimSpace(*o.ArxDocNumber) == "" {
				return http.StatusConflict, "wrong_state", false
			}
			return 0, "", true
		},
	})
}

type pdfGate struct {
	Operation     string
	GatewayPath   func(id int64) string
	Query         string
	Filename      func(order *OrderDetail) string
	RequiresCR    bool
	AllowedStates []OrderState
	Check         func(order *OrderDetail) (status int, code string, ok bool)
}

func (h *Handler) handleGatedPDF(w http.ResponseWriter, r *http.Request, gate pdfGate) {
	if gate.RequiresCR && !h.requireCustomerRelations(w, r) {
		return
	}
	if !h.requireVodka(w) || !h.requireGateway(w) {
		return
	}
	id, ok := h.parseOrderID(w, r)
	if !ok {
		return
	}
	order, err := h.getOrderWithoutOrigin(r, id)
	if err != nil {
		h.writeOrderLoadError(w, r, gate.Operation, id, err)
		return
	}
	if len(gate.AllowedStates) > 0 && !requireState(w, stateOf(order), gate.AllowedStates...) {
		return
	}
	if gate.Check != nil {
		if status, code, ok := gate.Check(order); !ok {
			httputil.Error(w, status, code)
			return
		}
	}
	if err := h.ensureGatewayPDFNullableTextFields(r.Context(), id); err != nil {
		h.dbFailure(w, r, gate.Operation+"_normalize_order", err, "order_id", id)
		return
	}
	h.proxyNormalizedPDF(w, r, gate.GatewayPath(id), gate.Query, gate.Filename(order), gate.Operation)
}

// ensureGatewayPDFNullableTextFields normalizes legacy nullable text before
// proxying gw-int PDFs, which expect selected text fields to scan as strings.
func (h *Handler) ensureGatewayPDFNullableTextFields(ctx context.Context, orderID int64) error {
	if _, err := h.deps.Vodka.ExecContext(ctx, `
UPDATE orders
SET cdlan_commerciale = COALESCE(cdlan_commerciale, ''),
    cdlan_cod_termini_pag = COALESCE(cdlan_cod_termini_pag, ''),
    cdlan_note = COALESCE(cdlan_note, ''),
    cdlan_dur_rin = COALESCE(cdlan_dur_rin, ''),
    cdlan_tacito_rin = COALESCE(cdlan_tacito_rin, ''),
    cdlan_sost_ord = COALESCE(cdlan_sost_ord, ''),
    cdlan_tempi_ril = COALESCE(cdlan_tempi_ril, ''),
    cdlan_durata_servizio = COALESCE(cdlan_durata_servizio, ''),
    cdlan_rif_ordcli = COALESCE(cdlan_rif_ordcli, ''),
    cdlan_rif_tech_nom = COALESCE(cdlan_rif_tech_nom, ''),
    cdlan_rif_tech_tel = COALESCE(cdlan_rif_tech_tel, ''),
    cdlan_rif_tech_email = COALESCE(cdlan_rif_tech_email, ''),
    cdlan_rif_altro_tech_nom = COALESCE(cdlan_rif_altro_tech_nom, ''),
    cdlan_rif_altro_tech_tel = COALESCE(cdlan_rif_altro_tech_tel, ''),
    cdlan_rif_altro_tech_email = COALESCE(cdlan_rif_altro_tech_email, ''),
    cdlan_rif_adm_nom = COALESCE(cdlan_rif_adm_nom, ''),
    cdlan_rif_adm_tech_tel = COALESCE(cdlan_rif_adm_tech_tel, ''),
    cdlan_rif_adm_tech_email = COALESCE(cdlan_rif_adm_tech_email, ''),
    cdlan_valuta = COALESCE(cdlan_valuta, 'EURO'),
    written_by = COALESCE(written_by, ''),
    profile_iva = COALESCE(profile_iva, ''),
    profile_cf = COALESCE(profile_cf, ''),
    profile_address = COALESCE(profile_address, ''),
    profile_city = COALESCE(profile_city, ''),
    profile_cap = COALESCE(profile_cap, ''),
    profile_pv = COALESCE(profile_pv, ''),
    profile_sdi = COALESCE(profile_sdi, ''),
    profile_lang = COALESCE(profile_lang, 'it'),
    service_type = COALESCE(service_type, ''),
    data_decorrenza = COALESCE(data_decorrenza, ''),
    cdlan_tacito_rin_in_pdf = COALESCE(cdlan_tacito_rin_in_pdf, '1'),
    is_colo = COALESCE(is_colo, '0'),
    origin_cod_termini_pag = COALESCE(origin_cod_termini_pag, '')
WHERE id = ?
  AND (cdlan_commerciale IS NULL OR cdlan_cod_termini_pag IS NULL OR cdlan_note IS NULL OR cdlan_dur_rin IS NULL OR cdlan_tacito_rin IS NULL OR cdlan_sost_ord IS NULL OR cdlan_tempi_ril IS NULL OR cdlan_durata_servizio IS NULL OR cdlan_rif_ordcli IS NULL OR cdlan_rif_tech_nom IS NULL OR cdlan_rif_tech_tel IS NULL OR cdlan_rif_tech_email IS NULL OR cdlan_rif_altro_tech_nom IS NULL OR cdlan_rif_altro_tech_tel IS NULL OR cdlan_rif_altro_tech_email IS NULL OR cdlan_rif_adm_nom IS NULL OR cdlan_rif_adm_tech_tel IS NULL OR cdlan_rif_adm_tech_email IS NULL OR cdlan_valuta IS NULL OR written_by IS NULL OR profile_iva IS NULL OR profile_cf IS NULL OR profile_address IS NULL OR profile_city IS NULL OR profile_cap IS NULL OR profile_pv IS NULL OR profile_sdi IS NULL OR profile_lang IS NULL OR service_type IS NULL OR data_decorrenza IS NULL OR cdlan_tacito_rin_in_pdf IS NULL OR is_colo IS NULL OR origin_cod_termini_pag IS NULL)`, orderID); err != nil {
		return err
	}
	_, err := h.deps.Vodka.ExecContext(ctx, `
UPDATE orders_rows
SET cdlan_codart = COALESCE(cdlan_codart, ''),
    cdlan_descart = COALESCE(cdlan_descart, ''),
    cdlan_qta = COALESCE(cdlan_qta, '1'),
    cdlan_serialnumber = COALESCE(cdlan_serialnumber, ''),
    cdlan_prezzo = COALESCE(cdlan_prezzo, '0'),
    cdlan_prezzo_attivazione = COALESCE(cdlan_prezzo_attivazione, '0'),
    cdlan_prezzo_cessazione = COALESCE(cdlan_prezzo_cessazione, '0'),
    cdlan_ragg_fatturazione = COALESCE(cdlan_ragg_fatturazione, 'A'),
    cdlan_codice_kit = COALESCE(cdlan_codice_kit, '')
WHERE orders_id = ?
  AND (cdlan_codart IS NULL OR cdlan_descart IS NULL OR cdlan_qta IS NULL OR cdlan_serialnumber IS NULL OR cdlan_prezzo IS NULL OR cdlan_prezzo_attivazione IS NULL OR cdlan_prezzo_cessazione IS NULL OR cdlan_ragg_fatturazione IS NULL OR cdlan_codice_kit IS NULL)`, orderID)
	return err
}

func (h *Handler) writeOrderLoadError(w http.ResponseWriter, r *http.Request, operation string, orderID int64, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "order_not_found")
		return
	}
	h.dbFailure(w, r, operation+"_load_order", err, "order_id", orderID)
}

func (h *Handler) proxyNormalizedPDF(w http.ResponseWriter, r *http.Request, path, query, filename, operation string) {
	start := time.Now()
	resp, err := h.deps.Arak.Do(http.MethodGet, path, query, nil)
	if err != nil {
		h.logFailure(r, slog.LevelError, "gateway pdf request failed", operation, start, "gw_path", path, "error", err)
		httputil.Error(w, http.StatusBadGateway, "gateway_error")
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logFailure(r, slog.LevelError, "gateway pdf read failed", operation, start, "gw_path", path, "error", err)
		httputil.Error(w, http.StatusBadGateway, "gateway_error")
		return
	}
	if resp.StatusCode >= http.StatusBadRequest {
		args := []any{"gw_path", path, "upstream_status", resp.StatusCode}
		compactBody := compactGatewayBody(body)
		if code := gatewayBodyCode(compactBody); code != "" {
			args = append(args, "upstream_code", code)
		}
		if message := gatewayBodyMessage(compactBody); message != "" {
			args = append(args, "upstream_message", message)
		}
		h.logFailure(r, slog.LevelWarn, "gateway pdf upstream error", operation, start, args...)
		httputil.Error(w, http.StatusBadGateway, "gateway_error")
		return
	}
	pdf, err := normalizePDFBody(body)
	if err != nil {
		h.logFailure(r, slog.LevelWarn, "gateway pdf malformed", operation, start, "gw_path", path, "error", err)
		httputil.Error(w, http.StatusBadGateway, "gw_pdf_malformed")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

func normalizePDFBody(body []byte) ([]byte, error) {
	return normalizePDFBodyDepth(bytes.TrimSpace(body), 0)
}

func normalizePDFBodyDepth(body []byte, depth int) ([]byte, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty pdf body")
	}
	if bytes.HasPrefix(body, []byte("%PDF")) {
		return body, nil
	}
	if depth > 2 {
		return nil, fmt.Errorf("pdf wrapper too deep")
	}
	var wrapper map[string]any
	if json.Unmarshal(body, &wrapper) == nil {
		for _, key := range []string{"pdf", "PDF", "data", "file", "content", "body", "base64", "result"} {
			if value, ok := wrapper[key]; ok {
				if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
					return normalizePDFString(text, depth+1)
				}
			}
		}
	}
	text := strings.TrimSpace(string(body))
	text = strings.Trim(text, "\"")
	return normalizePDFString(text, depth+1)
}

func normalizePDFString(text string, depth int) ([]byte, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "data:application/pdf;base64,") {
		text = strings.TrimPrefix(text, "data:application/pdf;base64,")
	}
	decoded, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(text)
	}
	if err != nil {
		return nil, err
	}
	return normalizePDFBodyDepth(bytes.TrimSpace(decoded), depth)
}
