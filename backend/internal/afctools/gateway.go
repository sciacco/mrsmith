package afctools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	arxDocNumberNotFoundCode     = "ARX_DOC_NUMBER_NOT_FOUND"
	orderPDFNotReadyErrorCode    = "pdf_not_ready"
	orderPDFNotReadyErrorMessage = "Il PDF non è ancora disponibile."
)

type gatewayPDFErrorMapper func(status int, body []byte) (mappedStatus int, payload any, handled bool)

// proxyGatewayPDF pipes a GET request to the Mistra NG Internal API
// (gw-int.cdlan.net) through the shared Arak OAuth2 client.
// The backend never exposes the token to the frontend.
func (h *Handler) proxyGatewayPDF(
	w http.ResponseWriter,
	r *http.Request,
	path string,
	qs string,
	notFoundPayload any,
	mapUpstreamError gatewayPDFErrorMapper,
) {
	if h.deps.Arak == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "arak_gateway_not_configured")
		return
	}

	resp, err := h.deps.Arak.Do(http.MethodGet, path, qs, nil)
	if err != nil {
		httputil.InternalError(w, r, err, "gateway proxy failed",
			"component", "afctools", "operation", "gateway_pdf", "path", path)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			httputil.InternalError(w, r, readErr, "gateway proxy failed reading upstream error body",
				"component", "afctools", "operation", "gateway_pdf", "path", path, "upstream_status", resp.StatusCode)
			return
		}

		logGatewayPDFUpstreamFailure(r, path, resp.StatusCode, body)

		if resp.StatusCode == http.StatusNotFound {
			httputil.JSON(w, http.StatusNotFound, notFoundPayload)
			return
		}
		if mapUpstreamError != nil {
			if mappedStatus, payload, handled := mapUpstreamError(resp.StatusCode, body); handled {
				httputil.JSON(w, mappedStatus, payload)
				return
			}
		}
		httputil.Error(w, resp.StatusCode, "gateway_error")
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		w.Header().Set("Content-Disposition", cd)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}

func (h *Handler) handleTicketPDF(w http.ResponseWriter, r *http.Request) {
	ticketID := strings.TrimSpace(r.PathValue("ticketId"))
	if ticketID == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_ticket_id")
		return
	}
	lang := r.URL.Query().Get("lang")
	if lang != "it" && lang != "en" {
		httputil.Error(w, http.StatusBadRequest, "invalid_lang")
		return
	}

	q := url.Values{}
	q.Set("ticket_type", "RemoteHands") // hard-pin preserved (spec §B.4.4)
	q.Set("lang", lang)

	h.proxyGatewayPDF(w, r,
		"/tickets/v1/pdf/"+url.PathEscape(ticketID),
		q.Encode(),
		map[string]string{"error": "ticket_pdf_not_found"},
		nil,
	)
}

func (h *Handler) handleOrderPDF(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(r.PathValue("orderId"))
	if orderID == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_order_id")
		return
	}

	h.proxyGatewayPDF(w, r,
		"/orders/v1/order/pdf/"+url.PathEscape(orderID),
		"",
		map[string]string{
			"error":   orderPDFNotReadyErrorCode,
			"message": orderPDFNotReadyErrorMessage,
		},
		mapOrderPDFGatewayError,
	)
}

func mapOrderPDFGatewayError(status int, body []byte) (int, any, bool) {
	if status != http.StatusInternalServerError || gatewayPDFUpstreamMessage(body) != arxDocNumberNotFoundCode {
		return 0, nil, false
	}

	return http.StatusNotFound, map[string]string{
		"error":   orderPDFNotReadyErrorCode,
		"message": orderPDFNotReadyErrorMessage,
	}, true
}

func logGatewayPDFUpstreamFailure(r *http.Request, path string, status int, body []byte) {
	attrs := []any{
		"component", "afctools",
		"operation", "gateway_pdf",
		"path", path,
		"upstream_status", status,
	}
	if message := gatewayPDFUpstreamMessage(body); message != "" {
		attrs = append(attrs, "upstream_message", message)
	}
	if snippet := compactGatewayPDFBody(body); snippet != "" {
		attrs = append(attrs, "upstream_body", snippet)
	}

	logger := logging.FromContext(r.Context())
	if status == http.StatusNotFound {
		logger.Info("gateway pdf upstream reported missing resource", attrs...)
		return
	}
	logger.Warn("gateway pdf upstream returned error", attrs...)
}

func gatewayPDFUpstreamMessage(body []byte) string {
	if len(strings.TrimSpace(string(body))) == 0 {
		return ""
	}

	var payload struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if payload.Message != "" {
		return payload.Message
	}
	return payload.Error
}

func compactGatewayPDFBody(body []byte) string {
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
