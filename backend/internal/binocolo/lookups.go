package binocolo

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListProvinces(w http.ResponseWriter, r *http.Request) {
	if !h.requireOpenAPIIT(w) {
		return
	}

	_, raw, err := listProvincesWithCache(r.Context(), h.provinceCache, h.openapiit, timeNowUTC)
	if err != nil {
		if errors.Is(err, errProvinceCacheFailure) {
			h.binocoloCacheFailure(w, r, err)
			return
		}
		h.openAPIITFailure(w, r, "list_provinces", err)
		return
	}
	httputil.JSON(w, http.StatusOK, json.RawMessage(raw))
}

func (h *Handler) handleListMAProvinceCatalog(w http.ResponseWriter, r *http.Request) {
	if !h.requireOpenAPIIT(w) {
		return
	}

	envelope, _, err := listProvincesWithCache(r.Context(), h.provinceCache, h.openapiit, timeNowUTC)
	if err != nil {
		if errors.Is(err, errProvinceCacheFailure) {
			h.binocoloCacheFailure(w, r, err)
			return
		}
		h.openAPIITFailure(w, r, "ma_catalog_provinces", err)
		return
	}
	items := make([]MAProvinceCatalogItem, 0, len(envelope.Data))
	for _, item := range envelope.Data {
		code := strings.ToUpper(strings.TrimSpace(item.Sigla))
		name := strings.TrimSpace(item.Provincia)
		region := strings.TrimSpace(item.Regione)
		if code == "" || name == "" || region == "" {
			continue
		}
		items = append(items, MAProvinceCatalogItem{Code: code, Name: name, Region: region})
	}
	httputil.JSON(w, http.StatusOK, MAProvinceCatalogResponse{Items: items})
}

// maAtecoSearchItem is one ATECO typeahead hit for the strategy editor.
type maAtecoSearchItem struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Hierarchy   *int   `json:"hierarchy,omitempty"`
}

type maAtecoSearchResponse struct {
	Items []maAtecoSearchItem `json:"items"`
}

// handleSearchAteco exposes the ATECO catalog search (SearchAteco) to the frontend
// so the strategy editor can offer a code/description typeahead — codes are picked
// from the real catalog (descriptions attached, format normalized) rather than typed.
func (h *Handler) handleSearchAteco(w http.ResponseWriter, r *http.Request) {
	if h.ateco == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "binocolo_ateco_not_configured")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		httputil.JSON(w, http.StatusOK, maAtecoSearchResponse{Items: []maAtecoSearchItem{}})
		return
	}
	limit := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	results, err := h.ateco.SearchAteco(r.Context(), query, limit)
	if err != nil {
		h.maFailure(w, r, "ma_ateco_search", err)
		return
	}
	items := make([]maAtecoSearchItem, 0, len(results))
	for _, item := range results {
		items = append(items, maAtecoSearchItem{Code: item.Codice, Description: item.Titolo, Hierarchy: item.Gerarchia})
	}
	httputil.JSON(w, http.StatusOK, maAtecoSearchResponse{Items: items})
}
