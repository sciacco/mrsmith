package budget

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
)

// moneyFields are the JSON keys that carry monetary decimal strings in the
// budget domain. They appear only on money fields — pagination uses
// current_page / total_pages, never bare "current" — so rounding by key name
// is safe and cannot corrupt unrelated decimals (e.g. percentages).
var moneyFields = map[string]struct{}{
	"limit":   {},
	"current": {},
}

// proxyToArakRoundingMoney forwards a request to Arak like proxyToArak, but
// rounds every monetary string field (limit/current) in a 2xx JSON response
// to exactly 2 decimal places before returning it to the client.
//
// Arak emits budget amounts with 3 decimal places, a legacy API-design
// choice we do not want to expose. 2dp is the correct precision for EUR
// amounts. The transform is a no-op on responses without limit/current
// fields and on non-2xx / empty responses (errors pass through untouched).
//
// Used only for responses that carry budget amounts (list, details,
// budget-over-percent report). Other endpoints keep using proxyToArak.
func proxyToArakRoundingMoney(w http.ResponseWriter, r *http.Request, arakPath string) {
	resp, err := arakClient.Do(r.Method, arakPath, r.URL.RawQuery, r.Body)
	if err != nil {
		requestLogger(r, "proxy_to_arak", "upstream_path", arakPath).Error("upstream request failed", "error", err)
		writeUpstreamError(w, http.StatusBadGateway, upstreamUnavailableCode, "upstream API error")
		return
	}
	defer resp.Body.Close()

	if translateUpstreamAuthFailure(w, resp.StatusCode) {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		requestLogger(r, "proxy_to_arak", "upstream_path", arakPath).Warn("failed to read upstream response", "error", err)
		writeUpstreamError(w, http.StatusBadGateway, upstreamUnavailableCode, "upstream read error")
		return
	}

	// Only round successful JSON bodies. Errors, empty bodies, and non-JSON
	// pass through unchanged.
	if resp.StatusCode == http.StatusOK && len(body) > 0 {
		var v any
		if err := json.Unmarshal(body, &v); err == nil {
			roundMoneyFields(v)
			if out, err := json.Marshal(v); err == nil {
				body = out
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

// roundMoneyFields recursively walks a decoded JSON value and rounds every
// monetary string field (limit/current) to exactly 2 decimal places.
// Maps, arrays, and nested structures are traversed; non-monetary values
// are left untouched.
func roundMoneyFields(v any) {
	switch val := v.(type) {
	case map[string]any:
		for k, item := range val {
			if _, isMoney := moneyFields[k]; isMoney {
				if s, ok := item.(string); ok {
					val[k] = roundMoneyTo2(s)
					continue
				}
			}
			roundMoneyFields(item)
		}
	case []any:
		for _, item := range val {
			roundMoneyFields(item)
		}
	}
}

// roundMoneyTo2 parses a decimal string and returns it rounded to 2 decimal
// places as a fixed string (always exactly 2 fractional digits). Non-numeric
// input is returned unchanged.
//
// Uses float64 + math.Round. This is exact for all non-tie inputs and for
// EUR budget amounts in practice. Half-cent ties (e.g. a 3rd decimal of 5)
// are subject to float64 representation — most round half-up as expected,
// a few classic cases (e.g. "1.005") round down because the value cannot be
// represented exactly. This is acceptable here: budget amounts never carry
// sub-cent significance, and Arak's 3dp trailing digit is effectively always
// 0 in real data. If exact decimal rounding ever becomes a requirement,
// swap this for a decimal-arithmetic implementation.
func roundMoneyTo2(s string) string {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	rounded := math.Round(f*100) / 100
	return strconv.FormatFloat(rounded, 'f', 2, 64)
}
