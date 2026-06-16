package binocolo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/auth"
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

type companySearchRequest struct {
	params         openapiit.CompanyITSearchParams
	cacheKey       string
	paramsJSON     json.RawMessage
	dryRun         bool
	dataEnrichment string
	forceRefresh   bool
}

func (h *Handler) handleSearchCompanies(w http.ResponseWriter, r *http.Request) {
	searchReq, badRequestCode, err := parseCompanySearchRequest(r.Context(), r.URL.Query(), h.ateco)
	if badRequestCode != "" {
		httputil.Error(w, http.StatusBadRequest, badRequestCode)
		return
	}
	if err != nil {
		if errors.Is(err, errAtecoStoreUnavailable) {
			httputil.Error(w, http.StatusServiceUnavailable, "binocolo_ateco_not_configured")
			return
		}
		h.binocoloCacheFailure(w, r, err)
		return
	}
	if h.companySearchCache == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "binocolo_cache_not_configured")
		return
	}

	if !searchReq.forceRefresh {
		entry, err := h.companySearchCache.GetValidCompanySearch(r.Context(), searchReq.cacheKey, time.Now().UTC())
		if err != nil {
			h.binocoloCacheFailure(w, r, err)
			return
		}
		if entry != nil {
			httputil.JSON(w, http.StatusOK, json.RawMessage(entry.Response))
			return
		}
	}

	if !h.requireOpenAPIIT(w) {
		return
	}

	var response json.RawMessage
	var upstreamErr error
	err = h.companySearchCache.WithCompanySearchCacheLock(r.Context(), searchReq.cacheKey, func(ctx context.Context) error {
		if !searchReq.forceRefresh {
			entry, err := h.companySearchCache.GetValidCompanySearch(ctx, searchReq.cacheKey, time.Now().UTC())
			if err != nil {
				return err
			}
			if entry != nil {
				response = entry.Response
				return nil
			}
		}

		result, err := h.openapiit.Company().SearchITRaw(ctx, searchReq.params)
		if err != nil {
			upstreamErr = err
			return nil
		}

		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		fetchedAt := time.Now().UTC()
		subject, email := companySearchRefreshActor(r.Context())
		if err := h.companySearchCache.UpsertCompanySearch(ctx, companySearchCacheWrite{
			CacheKey:           searchReq.cacheKey,
			Params:             searchReq.paramsJSON,
			Response:           raw,
			DryRun:             searchReq.dryRun,
			DataEnrichment:     searchReq.dataEnrichment,
			FetchedAt:          fetchedAt,
			ExpiresAt:          fetchedAt.Add(companySearchCacheTTL),
			RefreshedBySubject: subject,
			RefreshedByEmail:   email,
		}); err != nil {
			return err
		}
		response = raw
		return nil
	})
	if err != nil {
		h.binocoloCacheFailure(w, r, err)
		return
	}
	if upstreamErr != nil {
		h.openAPIITFailure(w, r, "search_companies", upstreamErr)
		return
	}

	httputil.JSON(w, http.StatusOK, json.RawMessage(response))
}

func parseCompanySearchRequest(ctx context.Context, query url.Values, ateco atecoStore) (companySearchRequest, string, error) {
	province, ok := normalizeProvince(query.Get("province"))
	if !ok {
		return companySearchRequest{}, "invalid_province", nil
	}

	dryRun, err := parseBoolQuery(query.Get("dry_run"))
	if err != nil {
		return companySearchRequest{}, "invalid_dry_run", nil
	}
	dryRunFlag := 0
	if dryRun {
		dryRunFlag = 1
	}

	forceRefresh, err := parseBoolQuery(query.Get("force_refresh"))
	if err != nil {
		return companySearchRequest{}, "invalid_force_refresh", nil
	}

	dataEnrichment, ok := normalizeCompanyDataEnrichment(query.Get("dataEnrichment"))
	if !ok {
		return companySearchRequest{}, "invalid_data_enrichment", nil
	}

	activityStatus, ok := normalizeCompanyActivityStatus(query.Get("activityStatus"))
	if !ok {
		return companySearchRequest{}, "invalid_activity_status", nil
	}

	minTurnover, err := parseOptionalIntQuery(query.Get("minTurnover"))
	if err != nil {
		return companySearchRequest{}, "invalid_min_turnover", nil
	}
	maxTurnover, err := parseOptionalIntQuery(query.Get("maxTurnover"))
	if err != nil {
		return companySearchRequest{}, "invalid_max_turnover", nil
	}
	minEmployees, err := parseOptionalIntQuery(query.Get("minEmployees"))
	if err != nil {
		return companySearchRequest{}, "invalid_min_employees", nil
	}
	maxEmployees, err := parseOptionalIntQuery(query.Get("maxEmployees"))
	if err != nil {
		return companySearchRequest{}, "invalid_max_employees", nil
	}
	skip, err := parseOptionalIntQuery(query.Get("skip"))
	if err != nil || (skip != nil && *skip < 0) {
		return companySearchRequest{}, "invalid_skip", nil
	}
	limit, err := parseOptionalIntQuery(query.Get("limit"))
	if err != nil || (limit != nil && (*limit < 1 || *limit > 1000)) {
		return companySearchRequest{}, "invalid_limit", nil
	}

	params := openapiit.CompanyITSearchParams{
		Province:       province,
		DryRun:         &dryRunFlag,
		DataEnrichment: dataEnrichment,
		CompanyName:    strings.TrimSpace(query.Get("companyName")),
		CCIAA:          strings.ToUpper(strings.TrimSpace(query.Get("cciaa"))),
		REACode:        strings.TrimSpace(query.Get("reaCode")),
		MinTurnover:    minTurnover,
		MaxTurnover:    maxTurnover,
		MinEmployees:   minEmployees,
		MaxEmployees:   maxEmployees,
		ActivityStatus: activityStatus,
		Skip:           skip,
		Limit:          limit,
	}
	if atecoCode := strings.TrimSpace(query.Get("atecoCode")); atecoCode != "" {
		if ateco == nil {
			return companySearchRequest{}, "", errAtecoStoreUnavailable
		}
		item, err := ateco.ResolveAtecoCode(ctx, atecoCode)
		if errors.Is(err, errAtecoCodeNotFound) {
			return companySearchRequest{}, "invalid_ateco_code", nil
		}
		if err != nil {
			return companySearchRequest{}, "", err
		}
		params.AtecoCode = item.CodiceSearch
	}
	cacheKey, paramsJSON, err := companySearchCacheKey(params)
	if err != nil {
		return companySearchRequest{}, "", err
	}

	return companySearchRequest{
		params:         params,
		cacheKey:       cacheKey,
		paramsJSON:     paramsJSON,
		dryRun:         dryRun,
		dataEnrichment: dataEnrichment,
		forceRefresh:   forceRefresh,
	}, "", nil
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

func companySearchRefreshActor(ctx context.Context) (string, string) {
	claims, ok := auth.GetClaims(ctx)
	if !ok {
		return "", ""
	}
	return strings.TrimSpace(claims.Subject), strings.TrimSpace(claims.Email)
}
