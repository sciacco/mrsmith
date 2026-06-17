package binocolo

import (
	"bytes"
	"encoding/json"
	"math"
)

// Deterministic financial engine for the veryshort deep-dive (Fase 3).
//
// IT-full pre-computes the analyst KPIs (profitability, leverage, liquidity,
// growth), so the engine READS them verbatim and assigns RAG semaphores; it does
// not recompute from the raw CEE codes. The only derivations are the EBITDA
// margin (ebitda/turnover) and the absolute net financial position
// (PFN = leverageRatios.pfnEbitda * operatingResults.ebitda), both needed by the
// valuation (Fase 4) and the brief (Fase 5). No LLM, no network.
//
// RAG thresholds are heuristic and tunable; they were calibrated against the first
// real IT-full payloads. Mind OpenAPI.it's mixed scale: ros/roe/roi and the trend
// fields arrive as percentages, but capitalizationDegree and ebitVariation arrive as
// fractions and are scaled x100 below. Missing values yield RAG "na" (never wrong).

const (
	maRAGGreen = "green"
	maRAGAmber = "amber"
	maRAGRed   = "red"
	maRAGNone  = "na"
)

// buildMADeepScorecard reads the IT-full payload into a financial scorecard.
// The ATECO code (for the Fase 4 sector-multiple lookup) is read from the payload.
func buildMADeepScorecard(payload json.RawMessage) *MADeepScorecard {
	object, err := decodeVendorObject(payload)
	if err != nil || object == nil {
		return nil
	}
	root := deepFullRoot(object)

	sc := &MADeepScorecard{AtecoCode: normalizeAtecoCode(firstVendorString(root,
		"atecoClassification.ateco.code", "atecoClassification.ateco2022.code", "ateco.code"))}
	sc.Turnover = deepFloatPtr(root, "ecofin.turnover")
	sc.TurnoverYear = deepIntPtr(root, "ecofin.turnoverYear")
	sc.Ebitda = deepFloatPtr(root, "operatingResults.ebitda")
	sc.NetWorth = deepFloatPtr(root, "ecofin.netWorth")
	if pfnEbitda := deepFloatPtr(root, "leverageRatios.pfnEbitda"); pfnEbitda != nil && sc.Ebitda != nil {
		v := *pfnEbitda * *sc.Ebitda
		sc.PFN = &v
	}

	// scale converts the vendor field to the unit the RAG thresholds and the UI
	// expect (1 for already-percent or ratio fields, 100 for fraction fields).
	// Getting this wrong made "capitalizzazione" red for every company and pinned
	// "ebit_variation" to amber.
	add := func(group, key, label, path, unit string, scale float64, rag func(float64) string) {
		metric := MADeepMetric{Group: group, Key: key, Label: label, Unit: unit, RAG: maRAGNone}
		if v := deepFloatPtr(root, path); v != nil {
			scaled := *v * scale
			metric.Value = &scaled
			metric.RAG = rag(scaled)
		}
		sc.Metrics = append(sc.Metrics, metric)
	}

	// Redditività
	add("redditivita", "ros", "ROS (margine operativo)", "profitability.ros", "%", 1, ragHigher(5, 10))
	add("redditivita", "roe", "ROE", "profitability.roe", "%", 1, ragHigher(5, 12))
	add("redditivita", "roi", "ROI", "profitability.roi", "%", 1, ragHigher(5, 10))
	if sc.Ebitda != nil && sc.Turnover != nil && *sc.Turnover > 0 {
		margin := *sc.Ebitda / *sc.Turnover * 100
		sc.Metrics = append(sc.Metrics, MADeepMetric{
			Group: "redditivita", Key: "ebitda_margin", Label: "Margine EBITDA", Unit: "%",
			Value: &margin, RAG: ragHigher(8, 15)(margin),
		})
	}

	// Leva e struttura finanziaria. debt_ratio is intentionally omitted: IT-full's
	// indebtedness.debtRatio duplicates leverage (attivo/PN), its scale is undefined
	// across simple vs complex balance sheets, and it read green for every company.
	add("leva", "pfn_ebitda", "PFN / EBITDA", "leverageRatios.pfnEbitda", "x", 1, ragLower(3, 5))
	add("leva", "leverage", "Leverage (attivo / PN)", "indebtedness.leverage", "x", 1, ragLower(3, 6))
	add("leva", "capitalizzazione", "Grado di capitalizzazione", "indebtedness.capitalizationDegree", "%", 100, ragHigher(20, 40))

	// Liquidità
	add("liquidita", "current_ratio", "Current ratio", "financialStability.currentRatio", "x", 1, ragHigher(1, 1.5))
	add("liquidita", "acid_test", "Acid test (quick ratio)", "financialStability.acidTest", "x", 1, ragHigher(0.8, 1))

	// Efficienza / ciclo
	add("efficienza", "financial_cycle", "Ciclo finanziario", "financialCycle.financialCycleDuration", "gg", 1, ragLower(60, 120))

	// Crescita (turnoverTrend is already a percentage; ebitVariation is a fraction)
	add("crescita", "turnover_trend", "Trend fatturato", "ecofin.turnoverTrend", "%", 1, ragHigher(0, 8))
	add("crescita", "ebit_variation", "Variazione EBIT", "development.ebitVariation", "%", 100, ragHigher(0, 8))

	// Equity-denominated ratios invert sign when equity is eroded: leverage
	// (attivo/PN) goes negative with PN<=0 and ragLower would read it as healthy.
	// Force red so an insolvent capital structure never shows green.
	if sc.NetWorth != nil && *sc.NetWorth <= 0 {
		for i := range sc.Metrics {
			if sc.Metrics[i].Key == "leverage" && sc.Metrics[i].Value != nil {
				sc.Metrics[i].RAG = maRAGRed
			}
		}
	}

	sc.OverallRAG = deepOverallRAG(sc.Metrics)
	return sc
}

// deepFullRoot returns the Full object, tolerating a {data: Full} envelope wrapper.
func deepFullRoot(object map[string]any) map[string]any {
	if object == nil {
		return nil
	}
	if _, ok := object["operatingResults"]; ok {
		return object
	}
	if _, ok := object["profitability"]; ok {
		return object
	}
	if _, ok := object["ecofin"]; ok {
		return object
	}
	if inner, ok := object["data"].(map[string]any); ok {
		return inner
	}
	return object
}

func deepFloatPtr(root map[string]any, path string) *float64 {
	value, ok := vendorPath(root, path)
	if !ok {
		return nil
	}
	n, ok := vendorNumber(value)
	if !ok {
		return nil
	}
	return &n
}

func deepIntPtr(root map[string]any, path string) *int {
	f := deepFloatPtr(root, path)
	if f == nil {
		return nil
	}
	v := int(*f)
	return &v
}

// ragHigher: higher is better. v >= good -> green, v >= ok -> amber, else red.
func ragHigher(ok, good float64) func(float64) string {
	return func(v float64) string {
		switch {
		case v >= good:
			return maRAGGreen
		case v >= ok:
			return maRAGAmber
		default:
			return maRAGRed
		}
	}
}

// ragLower: lower is better. v <= good -> green, v <= ok -> amber, else red.
func ragLower(good, ok float64) func(float64) string {
	return func(v float64) string {
		switch {
		case v <= good:
			return maRAGGreen
		case v <= ok:
			return maRAGAmber
		default:
			return maRAGRed
		}
	}
}

// deepOverallRAG summarizes the scorecard: any two reds (or one red plus ambers)
// makes the company red; scattered ambers make it amber; otherwise green.
func deepOverallRAG(metrics []MADeepMetric) string {
	red, amber, scored := 0, 0, 0
	for _, metric := range metrics {
		switch metric.RAG {
		case maRAGRed:
			red++
			scored++
		case maRAGAmber:
			amber++
			scored++
		case maRAGGreen:
			scored++
		}
	}
	if scored == 0 {
		return maRAGNone
	}
	switch {
	case red >= 2:
		return maRAGRed
	case red == 1 || amber >= 2:
		return maRAGAmber
	default:
		return maRAGGreen
	}
}

// deepPayloadReady reports whether an IT-full async response actually carries the
// company dataset (vs an empty/pending placeholder while the vendor job runs).
func deepPayloadReady(data json.RawMessage) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("{}")) || bytes.Equal(trimmed, []byte("[]")) {
		return false
	}
	object, err := decodeVendorObject(data)
	if err != nil {
		return false
	}
	root := deepFullRoot(object)
	for _, key := range []string{"companyDetails", "ecofin", "operatingResults", "profitability"} {
		if _, ok := root[key]; ok {
			return true
		}
	}
	return false
}

// buildMADeepValuation derives an EV/equity range from the scorecard and a sector
// multiple. EV/EBITDA is used when EBITDA is positive and its margin clears the
// fallback threshold; otherwise EV/Sales. A +/-15% spread forms the band and the
// SME haircut is applied to the multiple. Equity = EV - PFN when PFN is known.
func buildMADeepValuation(sc *MADeepScorecard, multiple *sectorMultiple, pricing maPricing) *MADeepValuation {
	if sc == nil || multiple == nil {
		return nil
	}
	haircut := pricing.SMEHaircutPct / 100.0
	if haircut < 0 {
		haircut = 0
	}
	if haircut > 0.95 {
		haircut = 0.95 // cap an over-100% misconfig rather than silently dropping the discount
	}
	factor := 1 - haircut

	useEbitda := sc.Ebitda != nil && *sc.Ebitda > 0 && multiple.EVEbitda != nil
	if useEbitda && sc.Turnover != nil && *sc.Turnover > 0 {
		margin := *sc.Ebitda / *sc.Turnover * 100
		if margin < pricing.EBITDAFallbackPct {
			useEbitda = false
		}
	}

	var method string
	var mult, ev float64
	switch {
	case useEbitda:
		method = "ev_ebitda"
		mult = *multiple.EVEbitda
		ev = *sc.Ebitda * mult * factor
	case sc.Turnover != nil && *sc.Turnover > 0 && multiple.EVSales != nil:
		method = "ev_sales"
		mult = *multiple.EVSales
		ev = *sc.Turnover * mult * factor
	default:
		return nil
	}

	const spread = 0.15
	val := &MADeepValuation{
		Method:     method,
		Multiple:   math.Round(mult*100) / 100,
		HaircutPct: pricing.SMEHaircutPct,
		EVLow:      math.Round(ev * (1 - spread)),
		EVHigh:     math.Round(ev * (1 + spread)),
		Sector:     multiple.Industry,
		NFirms:     multiple.NFirms,
		Source:     multiple.Source,
		SourceDate: multiple.SourceDate,
	}
	if sc.PFN != nil {
		low := math.Round(val.EVLow - *sc.PFN)
		high := math.Round(val.EVHigh - *sc.PFN)
		val.PFN = sc.PFN
		val.EquityLow = &low
		val.EquityHigh = &high
	}
	if multiple.NFirms > 0 && multiple.NFirms < 10 {
		val.Caveat = "Multiplo di settore su campione ridotto: stima indicativa."
	}
	return val
}
