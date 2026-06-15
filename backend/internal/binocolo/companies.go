package binocolo

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
)

var companySearchDataEnrichments = map[string]struct{}{
	"start":        {},
	"advanced":     {},
	"pec":          {},
	"address":      {},
	"shareholders": {},
	"name":         {},
}

var companySearchActivityStatuses = map[string]struct{}{
	"ATTIVA":        {},
	"CESSATA":       {},
	"REGISTRATA":    {},
	"INATTIVA":      {},
	"SOSPESA":       {},
	"IN_ISCRIZIONE": {},
}

func (h *Handler) handleSearchCompanies(w http.ResponseWriter, r *http.Request) {
	if !h.requireOpenAPIIT(w) {
		return
	}

	query := r.URL.Query()

	province, ok := normalizeProvince(query.Get("province"))
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_province")
		return
	}

	dryRun, err := parseBoolQuery(query.Get("dry_run"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_dry_run")
		return
	}
	dryRunFlag := 0
	if dryRun {
		dryRunFlag = 1
	}

	dataEnrichment, ok := normalizeCompanyDataEnrichment(query.Get("dataEnrichment"))
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_data_enrichment")
		return
	}

	activityStatus, ok := normalizeCompanyActivityStatus(query.Get("activityStatus"))
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid_activity_status")
		return
	}

	minTurnover, err := parseOptionalIntQuery(query.Get("minTurnover"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_min_turnover")
		return
	}
	maxTurnover, err := parseOptionalIntQuery(query.Get("maxTurnover"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_max_turnover")
		return
	}
	minEmployees, err := parseOptionalIntQuery(query.Get("minEmployees"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_min_employees")
		return
	}
	maxEmployees, err := parseOptionalIntQuery(query.Get("maxEmployees"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_max_employees")
		return
	}
	skip, err := parseOptionalIntQuery(query.Get("skip"))
	if err != nil || (skip != nil && *skip < 0) {
		httputil.Error(w, http.StatusBadRequest, "invalid_skip")
		return
	}
	limit, err := parseOptionalIntQuery(query.Get("limit"))
	if err != nil || (limit != nil && (*limit < 1 || *limit > 1000)) {
		httputil.Error(w, http.StatusBadRequest, "invalid_limit")
		return
	}

	result, err := h.openapiit.Company().SearchITRaw(r.Context(), openapiit.CompanyITSearchParams{
		Province:       province,
		DryRun:         &dryRunFlag,
		DataEnrichment: dataEnrichment,
		CompanyName:    strings.TrimSpace(query.Get("companyName")),
		AtecoCode:      strings.TrimSpace(query.Get("atecoCode")),
		CCIAA:          strings.ToUpper(strings.TrimSpace(query.Get("cciaa"))),
		REACode:        strings.TrimSpace(query.Get("reaCode")),
		MinTurnover:    minTurnover,
		MaxTurnover:    maxTurnover,
		MinEmployees:   minEmployees,
		MaxEmployees:   maxEmployees,
		ActivityStatus: activityStatus,
		Skip:           skip,
		Limit:          limit,
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

func normalizeCompanyDataEnrichment(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", true
	}
	_, ok := companySearchDataEnrichments[value]
	return value, ok
}

func normalizeCompanyActivityStatus(raw string) (string, bool) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if value == "" {
		return "", true
	}
	_, ok := companySearchActivityStatuses[value]
	return value, ok
}

func parseBoolQuery(raw string) (bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false, nil
	}
	return strconv.ParseBool(value)
}

func parseOptionalIntQuery(raw string) (*int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
