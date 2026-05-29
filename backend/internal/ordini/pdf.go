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
	_, err := h.deps.Vodka.ExecContext(ctx, `
UPDATE orders
SET cdlan_note = COALESCE(cdlan_note, ''),
    profile_iva = COALESCE(profile_iva, ''),
    profile_cf = COALESCE(profile_cf, ''),
    profile_address = COALESCE(profile_address, ''),
    profile_city = COALESCE(profile_city, ''),
    profile_cap = COALESCE(profile_cap, ''),
    profile_pv = COALESCE(profile_pv, ''),
    profile_sdi = COALESCE(profile_sdi, '')
WHERE id = ?
  AND (cdlan_note IS NULL OR profile_iva IS NULL OR profile_cf IS NULL OR profile_address IS NULL OR profile_city IS NULL OR profile_cap IS NULL OR profile_pv IS NULL OR profile_sdi IS NULL)`, orderID)
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
		if code := gatewayBodyCode(compactGatewayBody(body)); code != "" {
			args = append(args, "upstream_code", code)
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
