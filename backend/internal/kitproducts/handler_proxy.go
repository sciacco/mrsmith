package kitproducts

import (
	"io"
	"net/http"
	"net/url"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	upstreamAuthFailedCode  = "UPSTREAM_AUTH_FAILED"
	upstreamUnavailableCode = "UPSTREAM_UNAVAILABLE"
)

func (h *Handler) handleProxyMistraKit(w http.ResponseWriter, r *http.Request) {
	h.proxyToArak(w, r, "/products/v2/kit")
}

func (h *Handler) handleProxyMistraKitDiscount(w http.ResponseWriter, r *http.Request) {
	h.proxyToArak(w, r, "/products/v2/kit-discount")
}

func (h *Handler) handleProxyMistraDiscountedKit(w http.ResponseWriter, r *http.Request) {
	h.proxyToArak(w, r, "/products/v2/discounted-kit")
}

func (h *Handler) handleProxyMistraDiscountedKitByID(w http.ResponseWriter, r *http.Request) {
	h.proxyToArak(w, r, "/products/v2/discounted-kit/"+url.PathEscape(r.PathValue("id")))
}

func (h *Handler) handleProxyMistraCustomer(w http.ResponseWriter, r *http.Request) {
	h.proxyToArak(w, r, "/customers/v2/customer")
}

func (h *Handler) proxyToArak(w http.ResponseWriter, r *http.Request, upstreamPath string) {
	if h.arak == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "arak not configured")
		return
	}

	resp, err := h.arak.Do(r.Method, upstreamPath, r.URL.RawQuery, r.Body)
	if err != nil {
		httputil.JSON(w, http.StatusBadGateway, map[string]string{
			"error": "upstream API error",
			"code":  upstreamUnavailableCode,
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		httputil.JSON(w, http.StatusBadGateway, map[string]string{
			"error": "upstream authorization failed",
			"code":  upstreamAuthFailedCode,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		logging.FromContext(r.Context()).Warn(
			"failed to stream upstream response",
			"component", "kitproducts",
			"operation", "proxy_to_arak",
			"upstream_path", upstreamPath,
			"error", err,
		)
	}
}
