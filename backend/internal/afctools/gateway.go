package afctools

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// proxyGatewayPDF pipes a GET request to the Mistra NG Internal API
// (gw-int.cdlan.net) through the shared Arak OAuth2 client.
// The backend never exposes the token to the frontend.
func (h *Handler) proxyGatewayPDF(w http.ResponseWriter, r *http.Request, path string, qs string, notFoundMsg string) {
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

	if resp.StatusCode == http.StatusNotFound {
		httputil.Error(w, http.StatusNotFound, notFoundMsg)
		return
	}
	if resp.StatusCode >= 400 {
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
		"ticket_pdf_not_found")
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
		"Il PDF non è ancora pronto.")
}
