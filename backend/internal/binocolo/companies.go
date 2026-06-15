package binocolo

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
)

const companySearchEnrichment = "start"

func (h *Handler) handleSearchCompanies(w http.ResponseWriter, r *http.Request) {
	if !h.requireOpenAPIIT(w) {
		return
	}

	province, ok := normalizeProvince(r.URL.Query().Get("province"))
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_province")
		return
	}

	dryRun, err := parseBoolQuery(r.URL.Query().Get("dry_run"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_dry_run")
		return
	}
	dryRunFlag := 0
	if dryRun {
		dryRunFlag = 1
	}

	result, err := h.openapiit.Company().SearchITRaw(r.Context(), openapiit.CompanyITSearchParams{
		Province:       province,
		DryRun:         &dryRunFlag,
		DataEnrichment: companySearchEnrichment,
	})
	if err != nil {
		h.openAPIITFailure(w, r, "search_companies", err)
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

func normalizeProvince(raw string) (string, bool) {
	province := strings.ToUpper(strings.TrimSpace(raw))
	if province == "" {
		return "", true
	}
	if len(province) != 2 {
		return "", false
	}
	for _, value := range province {
		if value < 'A' || value > 'Z' {
			return "", false
		}
	}
	return province, true
}

func parseBoolQuery(raw string) (bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false, nil
	}
	return strconv.ParseBool(value)
}
