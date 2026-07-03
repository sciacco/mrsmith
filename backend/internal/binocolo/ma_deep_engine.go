package binocolo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
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

	// maMetricTierContorno (Fase 3, lente compratore): metriche sulla struttura
	// del capitale del venditore — visibili ma escluse dall'overall RAG.
	maMetricTierContorno = "contorno"
)

// maDeepThresholds sono le soglie RAG strutturalmente dipendenti dal business
// model (Fase 3): solo margine EBITDA, ROS e ciclo finanziario — leva e liquidità
// restano uniformi (e di contorno). Override per famiglia in ma_parameter (mig 095).
type maDeepThresholds struct {
	MarginOK, MarginGood float64
	ROSOK, ROSGood       float64
	CicloGood, CicloOK   float64
}

// defaultMADeepThresholds: le soglie storiche, usate senza famiglia.
func defaultMADeepThresholds() maDeepThresholds {
	return maDeepThresholds{MarginOK: 8, MarginGood: 15, ROSOK: 5, ROSGood: 10, CicloGood: 60, CicloOK: 120}
}

// maFamilyThresholdDefaults: tabella DEEP-DIVE-DECISIONS.md §5.3 (valori da
// prassi; la mig 095 li replica in ma_parameter per la taratura da UI).
var maFamilyThresholdDefaults = map[string]maDeepThresholds{
	maBMFamilyServiziRicorrenti:    {MarginOK: 10, MarginGood: 18, ROSOK: 6, ROSGood: 12, CicloGood: 30, CicloOK: 75},
	maBMFamilyProgettoIntegrazione: {MarginOK: 6, MarginGood: 12, ROSOK: 4, ROSGood: 8, CicloGood: 90, CicloOK: 150},
	maBMFamilyRivenditaVAR:         {MarginOK: 3, MarginGood: 7, ROSOK: 2, ROSGood: 5, CicloGood: 45, CicloOK: 90},
	maBMFamilySoftwareProdotto:     {MarginOK: 15, MarginGood: 25, ROSOK: 10, ROSGood: 18, CicloGood: 15, CicloOK: 60},
}

// buildMADeepScorecard reads the IT-full payload into a financial scorecard.
// The ATECO code (for the Fase 4 sector-multiple lookup) is read from the payload.
// Le soglie di margine/ROS/ciclo arrivano dal chiamante (per famiglia, Fase 3).
func buildMADeepScorecard(payload json.RawMessage, th maDeepThresholds) *MADeepScorecard {
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
	add("redditivita", "ros", "ROS (margine operativo)", "profitability.ros", "%", 1, ragHigher(th.ROSOK, th.ROSGood))
	add("redditivita", "roe", "ROE", "profitability.roe", "%", 1, ragHigher(5, 12))
	add("redditivita", "roi", "ROI", "profitability.roi", "%", 1, ragHigher(5, 10))
	if sc.Ebitda != nil && sc.Turnover != nil && *sc.Turnover > 0 {
		margin := *sc.Ebitda / *sc.Turnover * 100
		sc.Metrics = append(sc.Metrics, MADeepMetric{
			Group: "redditivita", Key: "ebitda_margin", Label: "Margine EBITDA", Unit: "%",
			Value: &margin, RAG: ragHigher(th.MarginOK, th.MarginGood)(margin),
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
	add("efficienza", "financial_cycle", "Ciclo finanziario", "financialCycle.financialCycleDuration", "gg", 1, ragLower(th.CicloGood, th.CicloOK))

	// Crescita (turnoverTrend is already a percentage; ebitVariation is a fraction)
	add("crescita", "turnover_trend", "Trend fatturato", "ecofin.turnoverTrend", "%", 1, ragHigher(0, 8))
	add("crescita", "ebit_variation", "Variazione EBIT", "development.ebitVariation", "%", 100, ragHigher(0, 8))

	// Qualità del margine (Fase 4): quanto è "vero" l'EBITDA contabile.
	// ΔEBITDA in euro dagli assoluti prior-year del vendor (informativo, RAG na:
	// il semaforo di trend vive già nel gruppo crescita); assorbimento magazzino
	// = quota di EBITDA mangiata dalla variazione rimanenze B.11 (metrica CORE:
	// margine che non diventa cassa — soglie euristiche 25/75, come le altre).
	reading := maCEEReadingFromRoot(root)
	if ebitdaL2Y, ok := maInspectNumber(root, "operatingResults.ebitdaL2Y"); ok && sc.Ebitda != nil {
		delta := *sc.Ebitda - ebitdaL2Y
		sc.Metrics = append(sc.Metrics, MADeepMetric{
			Group: "qualita_margine", Key: "delta_ebitda", Label: "Δ EBITDA vs anno precedente", Unit: "€",
			Value: &delta, RAG: maRAGNone,
		})
	}
	if reading != nil && sc.Ebitda != nil && *sc.Ebitda > 0 {
		absorption := 0.0
		if reading.InventoryVariation < 0 {
			absorption = -reading.InventoryVariation / *sc.Ebitda * 100
		}
		sc.Metrics = append(sc.Metrics, MADeepMetric{
			Group: "qualita_margine", Key: "assorbimento_magazzino", Label: "EBITDA assorbito da magazzino", Unit: "%",
			Value: &absorption, RAG: ragLower(25, 75)(absorption),
		})
	}

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

	// Lente compratore (Fase 3): leva, capitalizzazione, liquidità e ROE
	// fotografano la struttura del capitale del VENDITORE — che il compratore
	// sostituisce al closing. Restano visibili (segnale negoziale: dicono quanta
	// fretta ha il venditore) ma non guidano l'overall RAG.
	for i := range sc.Metrics {
		switch sc.Metrics[i].Key {
		case "leverage", "capitalizzazione", "current_ratio", "acid_test", "roe":
			sc.Metrics[i].Tier = maMetricTierContorno
		}
	}

	// Strato CEE (Fase 1/2): la PFN canonica viene dalla rilettura delle voci
	// depositate (catena dettaglio→totale→ratio vendor, con provenienza) — la
	// derivazione pfnEbitda×EBITDA resta solo come fallback per i payload senza
	// CEE. Gli scarti di riconciliazione viaggiano nello scorecard; il semaforo
	// continua a usare i ratio vendor (riconciliati su n=10).
	if reading != nil && reading.PFN != nil {
		v := reading.PFN.Value
		sc.PFN = &v
	}
	sc.Reconciliation = buildMADeepReconciliation(root, reading)

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
// makes the company red; scattered ambers make it amber; otherwise green. Dal
// redesign (Fase 3) le metriche di contorno sono ESCLUSE: il semaforo risponde ad
// "attrattività per un acquirente", non al merito di credito del venditore.
func deepOverallRAG(metrics []MADeepMetric) string {
	red, amber, scored := 0, 0, 0
	for _, metric := range metrics {
		if metric.Tier == maMetricTierContorno {
			continue
		}
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

// deepVintageKey extracts the payload's vintage key: the balance-sheet closing date
// (as YYYY-MM-DD) plus the turnover year. The date is taken LITERALLY from the vendor
// string — no timezone parsing, because "2025-12-31T00:00:00+01:00" read as an instant
// and rendered in UTC would shift the closing date to Dec 30.
func deepVintageKey(payload json.RawMessage) (string, *int, bool) {
	object, err := decodeVendorObject(payload)
	if err != nil || object == nil {
		return "", nil, false
	}
	root := deepFullRoot(object)
	raw := firstVendorString(root, "ecofin.balanceSheetDate")
	if len(raw) < 10 {
		return "", nil, false
	}
	date := raw[:10]
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", nil, false
	}
	return date, deepIntPtr(root, "ecofin.turnoverYear"), true
}

// buildMADeepValuation derives an EV/equity range from the scorecard, the CEE
// reading and a sector multiple (v2, Fase 2 del redesign).
//
// Banda ASIMMETRICA sul percorso EV/EBITDA: l'estremo alto usa l'EBITDA reported,
// l'estremo basso l'EBITDA prudenziale (reported − A.4 capitalizzazioni − contributi)
// — la banda si allarga esattamente in proporzione alla distorsione misurata e
// degenera nel ±15% classico quando A.4 e contributi sono zero. Prudenziale non
// positivo o sotto soglia → l'estremo basso ricade su EV/Sales (con clamp: mai sopra
// l'estremo basso reported). Sul percorso EV/Sales la banda resta simmetrica (la
// distorsione prudenziale non tocca i ricavi).
//
// Equity dal BRIDGE esplicito: EV − PFN (provenienza dichiarata) − TFR×tfr_bridge_pct
// − fondo imposte; i finanziamenti soci restano dentro la PFN come riga informativa
// (tema negoziale). Senza lettura CEE il bridge degrada alla sola PFN vendor.
func buildMADeepValuation(sc *MADeepScorecard, reading *maCEEReading, multiple *sectorMultiple, pricing maPricing) *MADeepValuation {
	if sc == nil || multiple == nil {
		return nil
	}
	haircutPct := pricing.haircutForTurnover(sc.Turnover)
	haircut := haircutPct / 100.0
	if haircut < 0 {
		haircut = 0
	}
	if haircut > 0.95 {
		haircut = 0.95 // cap an over-100% misconfig rather than silently dropping the discount
	}
	factor := 1 - haircut

	marginPct := func(ebitda float64) float64 {
		if sc.Turnover == nil || *sc.Turnover <= 0 {
			return math.Inf(1) // senza fatturato la soglia di margine non può bocciare
		}
		return ebitda / *sc.Turnover * 100
	}
	useEbitda := sc.Ebitda != nil && *sc.Ebitda > 0 && multiple.EVEbitda != nil &&
		marginPct(*sc.Ebitda) >= pricing.EBITDAFallbackPct

	const spread = 0.15
	var method, lowMethod string
	var mult, evLowBase, evHighBase float64
	var prudential *float64
	if reading != nil && reading.EBITDAPrudential != nil {
		prudential = reading.EBITDAPrudential
	}

	switch {
	case useEbitda:
		method = "ev_ebitda"
		mult = *multiple.EVEbitda
		evHighBase = *sc.Ebitda * mult * factor
		prud := *sc.Ebitda // senza lettura CEE il prudenziale coincide col reported
		if prudential != nil {
			prud = *prudential
		}
		switch {
		case prud > 0 && marginPct(prud) >= pricing.EBITDAFallbackPct:
			evLowBase = prud * mult * factor
		case sc.Turnover != nil && *sc.Turnover > 0 && multiple.EVSales != nil:
			// Prudenziale non utilizzabile: l'estremo basso ricade su EV/Sales,
			// senza mai superare l'estremo basso che darebbe il reported.
			lowMethod = "ev_sales"
			evLowBase = math.Min(*sc.Turnover**multiple.EVSales*factor, evHighBase)
		default:
			// Né prudenziale né EV/Sales: banda simmetrica sul reported, con caveat.
			evLowBase = evHighBase
		}
	case sc.Turnover != nil && *sc.Turnover > 0 && multiple.EVSales != nil:
		method = "ev_sales"
		mult = *multiple.EVSales
		evHighBase = *sc.Turnover * mult * factor
		evLowBase = evHighBase
	default:
		return nil
	}

	val := &MADeepValuation{
		Method:           method,
		LowMethod:        lowMethod,
		Multiple:         math.Round(mult*100) / 100,
		HaircutPct:       haircutPct,
		EVLow:            math.Round(evLowBase * (1 - spread)),
		EVHigh:           math.Round(evHighBase * (1 + spread)),
		PrudentialEbitda: prudential,
		Sector:           multiple.Industry,
		NFirms:           multiple.NFirms,
		Source:           multiple.Source,
		SourceDate:       multiple.SourceDate,
	}
	if method == "ev_ebitda" && evLowBase == evHighBase && lowMethod == "" && prudential != nil && *prudential != *sc.Ebitda {
		val.Caveat = "EBITDA prudenziale non utilizzabile e multiplo EV/Sales assente: estremo basso non prudenziale."
	}
	if multiple.NFirms > 0 && multiple.NFirms < 10 {
		val.Caveat = strings.TrimSpace(val.Caveat + " Multiplo di settore su campione ridotto: stima indicativa.")
	}
	val.Bridge = buildMADeepBridge(val, sc, reading, pricing)
	if val.Bridge != nil {
		val.PFN = sc.PFN
		val.EquityLow = val.Bridge.EquityLow
		val.EquityHigh = val.Bridge.EquityHigh
	}
	return val
}

// buildMADeepBridge costruisce il ponte EV→equity: righe autoportanti con
// provenienza. Ritorna nil quando la PFN non è nota (senza di lei l'equity non ha
// senso). TFR pesato con tfr_bridge_pct (default 100: screening prudente); fondo
// imposte solo se valorizzato; finanziamenti soci come riga informativa (già dentro
// la PFN).
func buildMADeepBridge(val *MADeepValuation, sc *MADeepScorecard, reading *maCEEReading, pricing maPricing) *MADeepBridge {
	if sc.PFN == nil {
		return nil
	}
	bridge := &MADeepBridge{}
	pfnProv := maCEEProvVendorRatio
	if reading != nil && reading.PFN != nil {
		pfnProv = reading.PFN.Provenance
	}
	bridge.PFN = &MADeepBridgeRow{Value: math.Round(*sc.PFN), Provenance: pfnProv}
	deductions := *sc.PFN
	if reading != nil {
		if reading.TFR != nil {
			pct := pricing.TFRBridgePct
			if pct <= 0 || pct > 100 {
				pct = maTFRBridgePctDefault
			}
			tfr := math.Round(*reading.TFR * pct / 100)
			note := ""
			if pct < 100 {
				note = fmt.Sprintf("al %.0f%% del fondo", pct)
			}
			bridge.TFR = &MADeepBridgeRow{Value: tfr, Provenance: maCEEProvDetail, Note: note}
			deductions += tfr
		}
		if reading.TaxFund != nil && *reading.TaxFund > 0 {
			bridge.TaxFund = &MADeepBridgeRow{Value: math.Round(*reading.TaxFund), Provenance: maCEEProvDetail}
			deductions += *reading.TaxFund
		}
		if reading.ShareholderLoans != nil && reading.ShareholderLoans.Value > 0 {
			bridge.ShareholderLoans = &MADeepBridgeRow{
				Value:      math.Round(reading.ShareholderLoans.Value),
				Provenance: reading.ShareholderLoans.Provenance,
				Note:       "inclusi nella PFN — riga negoziale: al closing spesso rinunciati o convertiti",
			}
		}
	}
	low := math.Round(val.EVLow - deductions)
	high := math.Round(val.EVHigh - deductions)
	bridge.EquityLow = &low
	bridge.EquityHigh = &high
	return bridge
}
