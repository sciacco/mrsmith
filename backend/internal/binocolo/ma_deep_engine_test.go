package binocolo

import (
	"encoding/json"
	"testing"
)

func findDeepMetric(sc *MADeepScorecard, key string) *MADeepMetric {
	for i := range sc.Metrics {
		if sc.Metrics[i].Key == key {
			return &sc.Metrics[i]
		}
	}
	return nil
}

func TestBuildMADeepScorecardHealthy(t *testing.T) {
	payload := json.RawMessage(`{
		"atecoClassification": {"ateco": {"code": "62.01"}},
		"ecofin": {"turnover": 5000000, "turnoverYear": 2024, "turnoverTrend": 12.0, "netWorth": 2000000},
		"operatingResults": {"ebitda": 750000, "ebit": 500000},
		"profitability": {"ros": 10.0, "roe": 15.0, "roi": 12.0},
		"financialStability": {"currentRatio": 1.6, "acidTest": 1.1},
		"indebtedness": {"leverage": 2.5, "debtRatio": 55.0, "capitalizationDegree": 0.45},
		"leverageRatios": {"pfnEbitda": 2.0},
		"financialCycle": {"financialCycleDuration": 40.0},
		"development": {"ebitVariation": 0.10}
	}`)
	sc := buildMADeepScorecard(payload, defaultMADeepThresholds())
	if sc == nil {
		t.Fatal("nil scorecard")
	}
	if sc.Ebitda == nil || *sc.Ebitda != 750000 {
		t.Fatalf("ebitda: %v", sc.Ebitda)
	}
	if sc.Turnover == nil || *sc.Turnover != 5000000 {
		t.Fatalf("turnover: %v", sc.Turnover)
	}
	if sc.TurnoverYear == nil || *sc.TurnoverYear != 2024 {
		t.Fatalf("turnoverYear: %v", sc.TurnoverYear)
	}
	if sc.AtecoCode != "62.01" {
		t.Fatalf("ateco: %s", sc.AtecoCode)
	}
	// PFN = pfnEbitda * ebitda = 2.0 * 750000.
	if sc.PFN == nil || *sc.PFN != 1500000 {
		t.Fatalf("pfn expected 1500000 got %v", sc.PFN)
	}
	// EBITDA margin = 750000/5000000*100 = 15 -> green.
	margin := findDeepMetric(sc, "ebitda_margin")
	if margin == nil || margin.Value == nil || *margin.Value != 15 || margin.RAG != maRAGGreen {
		t.Fatalf("ebitda_margin: %+v", margin)
	}
	if m := findDeepMetric(sc, "pfn_ebitda"); m == nil || m.RAG != maRAGGreen {
		t.Fatalf("pfn_ebitda rag: %+v", m)
	}
	if m := findDeepMetric(sc, "current_ratio"); m == nil || m.RAG != maRAGGreen {
		t.Fatalf("current_ratio rag: %+v", m)
	}
	if m := findDeepMetric(sc, "ros"); m == nil || m.RAG != maRAGGreen {
		t.Fatalf("ros rag: %+v", m)
	}
	if sc.OverallRAG != maRAGGreen {
		t.Fatalf("overall rag: %s", sc.OverallRAG)
	}
}

func TestBuildMADeepScorecardStressedAndMissing(t *testing.T) {
	payload := json.RawMessage(`{
		"ecofin": {"turnover": 1000000, "netWorth": 50000, "turnoverTrend": -5.0},
		"operatingResults": {"ebitda": 30000},
		"financialStability": {"currentRatio": 0.8, "acidTest": 0.5},
		"indebtedness": {"leverage": 8.0, "debtRatio": 90.0, "capitalizationDegree": 0.10},
		"leverageRatios": {"pfnEbitda": 7.0}
	}`)
	sc := buildMADeepScorecard(payload, defaultMADeepThresholds())
	if sc == nil {
		t.Fatal("nil scorecard")
	}
	// profitability block absent -> ros metric present but RAG na with no value.
	if m := findDeepMetric(sc, "ros"); m == nil || m.RAG != maRAGNone || m.Value != nil {
		t.Fatalf("ros should be na: %+v", m)
	}
	if m := findDeepMetric(sc, "pfn_ebitda"); m == nil || m.RAG != maRAGRed {
		t.Fatalf("pfn_ebitda should be red: %+v", m)
	}
	if m := findDeepMetric(sc, "current_ratio"); m == nil || m.RAG != maRAGRed {
		t.Fatalf("current_ratio should be red: %+v", m)
	}
	// PFN = 7.0 * 30000 = 210000.
	if sc.PFN == nil || *sc.PFN != 210000 {
		t.Fatalf("pfn: %v", sc.PFN)
	}
	if sc.OverallRAG != maRAGRed {
		t.Fatalf("overall rag should be red: %s", sc.OverallRAG)
	}
}

func TestBuildMADeepValuationEVEbitda(t *testing.T) {
	turnover, ebitda, pfn := 5000000.0, 750000.0, 1000000.0
	sc := &MADeepScorecard{Turnover: &turnover, Ebitda: &ebitda, PFN: &pfn}
	evEbitda := 10.0
	multiple := &sectorMultiple{Industry: "Software", EVEbitda: &evEbitda, NFirms: 290, Source: "Damodaran", SourceDate: "2026-01-05"}
	pricing := maPricing{SMEHaircutPct: 30, EBITDAFallbackPct: 5}
	val := buildMADeepValuation(sc, nil, multiple, pricing)
	if val == nil || val.Method != "ev_ebitda" {
		t.Fatalf("expected ev_ebitda: %+v", val)
	}
	// Senza lettura CEE il prudenziale coincide col reported: banda simmetrica.
	// EV center = 750000 * 10 * 0.7 = 5,250,000; band +/-15%.
	if val.EVLow != 4462500 || val.EVHigh != 6037500 {
		t.Fatalf("EV band: %v - %v", val.EVLow, val.EVHigh)
	}
	// equity = EV - PFN(1,000,000): bridge degradato alla sola PFN (vendor_ratio).
	if val.EquityLow == nil || *val.EquityLow != 3462500 {
		t.Fatalf("equityLow: %v", val.EquityLow)
	}
	if val.Bridge == nil || val.Bridge.PFN == nil || val.Bridge.PFN.Provenance != maCEEProvVendorRatio {
		t.Fatalf("bridge pfn: %+v", val.Bridge)
	}
}

func TestBuildMADeepValuationFallbackEVSales(t *testing.T) {
	turnover, ebitda := 2000000.0, 20000.0 // margin 1% < 5% -> EV/Sales fallback
	sc := &MADeepScorecard{Turnover: &turnover, Ebitda: &ebitda}
	evEbitda, evSales := 10.0, 1.5
	multiple := &sectorMultiple{Industry: "X", EVEbitda: &evEbitda, EVSales: &evSales, NFirms: 50}
	pricing := maPricing{SMEHaircutPct: 30, EBITDAFallbackPct: 5}
	val := buildMADeepValuation(sc, nil, multiple, pricing)
	if val == nil || val.Method != "ev_sales" {
		t.Fatalf("expected ev_sales: %+v", val)
	}
	// EV center = 2,000,000 * 1.5 * 0.7 = 2,100,000; low = 0.85 * 2.1M.
	if val.EVLow != 1785000 {
		t.Fatalf("evLow: %v", val.EVLow)
	}
	if val.EquityLow != nil {
		t.Fatalf("equity should be nil without PFN: %v", val.EquityLow)
	}
}

func TestDeepFullRootUnwrapsDataEnvelope(t *testing.T) {
	payload := json.RawMessage(`{"data": {"ecofin": {"turnover": 100}, "operatingResults": {"ebitda": 10}}}`)
	sc := buildMADeepScorecard(payload, defaultMADeepThresholds())
	if sc == nil || sc.Turnover == nil || *sc.Turnover != 100 {
		t.Fatalf("unwrap failed: %+v", sc)
	}
}

// TestBuildMADeepScorecardRealPayloads pins the engine against trimmed real
// IT-full payloads (fields the engine actually reads). It is the regression guard
// for the scale/sign calibration: capitalizationDegree and ebitVariation arrive as
// fractions, and leverage inverts sign when equity is eroded.
func TestBuildMADeepScorecardRealPayloads(t *testing.T) {
	cases := []struct {
		name       string
		payload    string
		overall    string
		metricRAGs map[string]string
	}{
		{
			// REDOKUN SRLS: strong company that overallRag wrongly flagged red only
			// because capitalizzazione (82.7%) was scored as 0.83% -> always red.
			name: "redokun_capitalization_scale_fix",
			payload: `{
				"ecofin": {"turnover": 1052328, "turnoverYear": 2024, "turnoverTrend": 29.86, "netWorth": 375851},
				"operatingResults": {"ebitda": 202796},
				"profitability": {"ros": 11.14, "roe": 37.89, "roi": 75.38},
				"financialStability": {"currentRatio": 4.2639, "acidTest": 4.2639},
				"indebtedness": {"leverage": 1.2089, "capitalizationDegree": 0.8272},
				"leverageRatios": {"pfnEbitda": -1.0863},
				"financialCycle": {"financialCycleDuration": -23.7377},
				"development": {"ebitVariation": -0.0323}
			}`,
			overall: maRAGAmber, // was red before the fix
			metricRAGs: map[string]string{
				"capitalizzazione": maRAGGreen, // 0.8272 -> 82.72%
				"leverage":         maRAGGreen,
			},
		},
		{
			// KRAL SRLS: negative equity (-81.980). leverage raw is -0.54, which
			// ragLower would read as healthy green; the sign guard forces red.
			// Dalla lente compratore (Fase 3) le metriche patrimoniali sono di
			// contorno: l'overall scende ad ambra (resta il rosso core sul trend
			// -27,6%), e l'insolvenza urla dal flag patrimonio_eroso + contorno rossi.
			name: "kral_insolvent_sign_guard",
			payload: `{
				"ecofin": {"turnover": 99747, "turnoverYear": 2025, "turnoverTrend": -27.64, "netWorth": -81980},
				"operatingResults": {"ebitda": 50941},
				"profitability": {"ros": 10.91, "roe": -11.55},
				"financialStability": {"currentRatio": 0.148, "acidTest": 0.148},
				"indebtedness": {"leverage": -0.5407, "capitalizationDegree": -1.8495},
				"development": {"ebitVariation": 37.3947}
			}`,
			overall: maRAGAmber, // era red quando il patrimoniale guidava l'overall
			metricRAGs: map[string]string{
				"leverage":         maRAGRed, // sign guard (raw -0.5407 would be green)
				"capitalizzazione": maRAGRed, // -184.95%
			},
		},
		{
			// MFT ITALIA SRL: genuinely weak, stays red. But ebit_variation 0.3351
			// is +33.5% growth and must be green (was pinned amber as "0.34%").
			name: "mft_ebit_variation_scale_fix",
			payload: `{
				"ecofin": {"turnover": 1222493, "turnoverYear": 2024, "turnoverTrend": -0.85, "netWorth": 80993},
				"operatingResults": {"ebitda": 94934},
				"profitability": {"ros": 1.56, "roe": 0.6, "roi": 19.9},
				"financialStability": {"currentRatio": 1.1682, "acidTest": 0.5082},
				"indebtedness": {"leverage": 8.7515, "capitalizationDegree": 0.1143},
				"leverageRatios": {"pfnEbitda": 0.1547},
				"financialCycle": {"financialCycleDuration": 104.8978},
				"development": {"ebitVariation": 0.3351}
			}`,
			overall: maRAGRed,
			metricRAGs: map[string]string{
				"ebit_variation":   maRAGGreen, // 0.3351 -> 33.51%
				"capitalizzazione": maRAGRed,   // 11.43%
				"leverage":         maRAGRed,   // 8.75
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := buildMADeepScorecard(json.RawMessage(tc.payload), defaultMADeepThresholds())
			if sc == nil {
				t.Fatal("nil scorecard")
			}
			if sc.OverallRAG != tc.overall {
				t.Fatalf("overall rag: got %s want %s", sc.OverallRAG, tc.overall)
			}
			for key, want := range tc.metricRAGs {
				m := findDeepMetric(sc, key)
				if m == nil {
					t.Fatalf("metric %q missing", key)
				}
				if m.RAG != want {
					t.Fatalf("metric %q: got rag %s want %s (value %v)", key, m.RAG, want, m.Value)
				}
			}
			// debt_ratio was dropped from the scorecard.
			if m := findDeepMetric(sc, "debt_ratio"); m != nil {
				t.Fatalf("debt_ratio should be dropped, got %+v", m)
			}
		})
	}
}
