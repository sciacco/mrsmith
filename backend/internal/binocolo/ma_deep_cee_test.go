package binocolo

// Golden test dello strato CEE (Fase 1, approvati): pinnano le riconciliazioni
// verificate a mano sulle due fixture reali (DEEP-DIVE-DECISIONS.md §1) e la catena
// di fallback con provenienza sui casi limite.

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

func loadFixturePayload(t *testing.T, path string) (json.RawMessage, map[string]any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture %s: %v", path, err)
	}
	payload := json.RawMessage(raw)
	object, err := decodeVendorObject(payload)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return payload, deepFullRoot(object)
}

func assertPtr(t *testing.T, label string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: nil, want %v", label, want)
	}
	if math.Abs(*got-want) > 0.5 {
		t.Fatalf("%s: %v, want %v", label, *got, want)
	}
}

// TestMACEEReadingMFT pinna la lettura sulla fixture MFT ITALIA SRL (micro,
// impiantistica, bilancio 2024): dettaglio pieno, derivati assenti, A.4/contributi
// zero (prudenziale = reported), magazzino in forte crescita (B.11 negativo).
func TestMACEEReadingMFT(t *testing.T) {
	_, root := loadFixturePayload(t, "testdata/itfull_mft_2024.json")
	r := maCEEReadingFromRoot(root)
	if r == nil {
		t.Fatal("nil reading")
	}
	if r.GrossFinancialDebt == nil || r.GrossFinancialDebt.Value != 34492 || r.GrossFinancialDebt.Provenance != maCEEProvDetail {
		t.Fatalf("gross: %+v", r.GrossFinancialDebt)
	}
	if r.PFN == nil || r.PFN.Value != 14682 || r.PFN.Provenance != maCEEProvDetail {
		t.Fatalf("pfn: %+v", r.PFN)
	}
	assertPtr(t, "beyond", r.FinancialDebtBeyond, 0)
	assertPtr(t, "cash", r.Cash, 19810)
	assertPtr(t, "tfr", r.TFR, 100573)
	assertPtr(t, "fondi", r.RiskProvisionsTotal, 0)
	if r.ShareholderLoans == nil || r.ShareholderLoans.Value != 10000 || r.ShareholderLoans.Provenance != maCEEProvDetail {
		t.Fatalf("soci: %+v", r.ShareholderLoans)
	}
	assertPtr(t, "ebitda_cee", r.EBITDACEE, 94934)
	assertPtr(t, "ebitda_prudenziale", r.EBITDAPrudential, 94934) // A.4 e contributi zero
	if r.Capitalizations != 0 || r.OperatingGrants != 0 {
		t.Fatalf("A.4/contributi attesi zero: %v %v", r.Capitalizations, r.OperatingGrants)
	}
	if r.OtherRevenuesA5 != 31669 {
		t.Fatalf("A.5: %v", r.OtherRevenuesA5)
	}
	if r.LeaseCosts != 14981 {
		t.Fatalf("B.8: %v", r.LeaseCosts)
	}
	if r.InventoryVariation != -115701 {
		t.Fatalf("B.11: %v", r.InventoryVariation)
	}
	assertPtr(t, "ante_imposte", r.PreTaxResult, 12177)
	assertPtr(t, "imposte", r.Taxes, 11691)
	assertPtr(t, "utile", r.NetProfit, 486) // 178 = imposte, NON utile (semantica verificata)
	assertPtr(t, "pn", r.NetWorth, 80993)
	assertPtr(t, "attivo", r.TotalAssets, 708807)
}

// TestMACEEReadingCDLAN pinna la lettura sulla fixture CDLAN SPA (media, TLC/DC,
// bilancio 2025): split scadenze non degenere, derivati passivi inclusi nel lordo,
// contributi non nulli (prudenziale < reported), partecipazioni al 49% dell'attivo.
func TestMACEEReadingCDLAN(t *testing.T) {
	_, root := loadFixturePayload(t, "testdata/itfull_cdlan_2025.json")
	r := maCEEReadingFromRoot(root)
	if r == nil {
		t.Fatal("nil reading")
	}
	// Lordo = 4.915.376 (D.1-D.5) + 7.241 derivati passivi (definizione vendor).
	if r.GrossFinancialDebt == nil || r.GrossFinancialDebt.Value != 4922617 || r.GrossFinancialDebt.Provenance != maCEEProvDetail {
		t.Fatalf("gross: %+v", r.GrossFinancialDebt)
	}
	if r.PassiveDerivatives != 7241 {
		t.Fatalf("derivati: %v", r.PassiveDerivatives)
	}
	if r.PFN == nil || r.PFN.Value != 1666053 {
		t.Fatalf("pfn: %+v", r.PFN)
	}
	assertPtr(t, "beyond", r.FinancialDebtBeyond, 3152407) // solo banche oltre esercizio
	assertPtr(t, "tfr", r.TFR, 294528)
	assertPtr(t, "fondi", r.RiskProvisionsTotal, 42241)
	assertPtr(t, "partecipazioni_tot", r.ParticipationsTotal, 7698063)
	assertPtr(t, "partecipazioni_controllate", r.ParticipationsSubs, 7693063)
	if r.ParticipationIncome != 1100000 {
		t.Fatalf("C.15: %v", r.ParticipationIncome)
	}
	assertPtr(t, "ebitda_cee", r.EBITDACEE, 2528884)
	assertPtr(t, "ebitda_prudenziale", r.EBITDAPrudential, 2499509) // − contributi 29.375
	if r.OperatingGrants != 29375 || r.OtherRevenuesA5 != 376557 {
		t.Fatalf("contributi/A.5: %v %v", r.OperatingGrants, r.OtherRevenuesA5)
	}
	if r.LeaseCosts != 1517608 {
		t.Fatalf("B.8: %v", r.LeaseCosts)
	}
	assertPtr(t, "utile", r.NetProfit, 2171236)
	assertPtr(t, "imposte", r.Taxes, 471598)
}

// TestMACEEReadingFallbacks esercita la catena di provenienza: soli totali per voce
// (cee_total, niente split oltre), famiglia IPL normalizzata, fallback vendor_ratio
// senza dati patrimoniali, payload senza codici CEE.
func TestMACEEReadingFallbacks(t *testing.T) {
	// Solo totali (deposito abbreviato sintetico) — provenienza cee_total.
	totalsOnly, _ := decodeVendorObject(json.RawMessage(`{
		"debts": [{"code": "IIC332", "value": 100000}, {"code": "IIC331", "value": 20000}],
		"cashEquivalents": [{"code": "IIC070", "value": 30000}]
	}`))
	r := maCEEReadingFromRoot(deepFullRoot(totalsOnly))
	if r == nil || r.GrossFinancialDebt == nil || r.GrossFinancialDebt.Value != 120000 || r.GrossFinancialDebt.Provenance != maCEEProvTotal {
		t.Fatalf("totals-only gross: %+v", r.GrossFinancialDebt)
	}
	if r.PFN == nil || r.PFN.Value != 90000 || r.PFN.Provenance != maCEEProvTotal {
		t.Fatalf("totals-only pfn: %+v", r.PFN)
	}
	if r.FinancialDebtBeyond != nil {
		t.Fatalf("beyond deve mancare senza split: %v", *r.FinancialDebtBeyond)
	}
	if r.ShareholderLoans == nil || r.ShareholderLoans.Value != 20000 || r.ShareholderLoans.Provenance != maCEEProvTotal {
		t.Fatalf("totals-only soci: %+v", r.ShareholderLoans)
	}

	// Famiglia IPL: stessa numerica, stessa lettura.
	iplFamily, _ := decodeVendorObject(json.RawMessage(`{
		"debts": [{"code": "IPL094", "value": 50}, {"code": "IPL095", "value": 30}],
		"cashEquivalents": [{"code": "IPL070", "value": 10}]
	}`))
	r = maCEEReadingFromRoot(deepFullRoot(iplFamily))
	if r == nil || r.PFN == nil || r.PFN.Value != 70 || r.PFN.Provenance != maCEEProvDetail {
		t.Fatalf("ipl family pfn: %+v", r)
	}

	// Nessun dato patrimoniale ma ratio vendor presente — provenienza vendor_ratio.
	ratioOnly, _ := decodeVendorObject(json.RawMessage(`{
		"productionValue": [{"code": "IIC124", "value": 500}],
		"leverageRatios": {"pfnEbitda": 2.0},
		"operatingResults": {"ebitda": 100}
	}`))
	r = maCEEReadingFromRoot(deepFullRoot(ratioOnly))
	if r == nil || r.PFN == nil || r.PFN.Value != 200 || r.PFN.Provenance != maCEEProvVendorRatio {
		t.Fatalf("vendor-ratio pfn: %+v", r)
	}
	if r.GrossFinancialDebt != nil {
		t.Fatalf("gross deve mancare: %+v", r.GrossFinancialDebt)
	}

	// Nessun codice CEE — lettura assente (2 payload su 10 nella cache reale).
	empty, _ := decodeVendorObject(json.RawMessage(`{"ecofin": {"turnover": 1}}`))
	if r := maCEEReadingFromRoot(deepFullRoot(empty)); r != nil {
		t.Fatalf("reading atteso nil: %+v", r)
	}
}

// TestMADeepScorecardReconciliation verifica che buildMADeepScorecard trasporti gli
// scarti vendor-vs-CEE: ~zero sulle fixture reali (definizioni riconciliate), assente
// sui payload senza CEE, e ROE in punti percentuali.
func TestMADeepScorecardReconciliation(t *testing.T) {
	mft, _ := loadFixturePayload(t, "testdata/itfull_mft_2024.json")
	sc := buildMADeepScorecard(mft)
	if sc == nil || sc.Reconciliation == nil {
		t.Fatal("reconciliation mancante su MFT")
	}
	rec := sc.Reconciliation
	if rec.EBITDAPct == nil || *rec.EBITDAPct != 0 {
		t.Fatalf("ebitda pct: %v", rec.EBITDAPct)
	}
	// PFN vendor 14.686 vs CEE 14.682 → −0.03% (rounding del ratio a 4 decimali).
	if rec.PFNPct == nil || math.Abs(*rec.PFNPct) > 0.05 || rec.PFNProvenance != maCEEProvDetail {
		t.Fatalf("pfn pct/prov: %v %q", rec.PFNPct, rec.PFNProvenance)
	}
	// ROE vendor 0.6 vs CEE 486/80.993 = 0.60% → 0 punti (il falso quirk non esiste).
	if rec.ROEPointsDiff == nil || *rec.ROEPointsDiff != 0 {
		t.Fatalf("roe points: %v", rec.ROEPointsDiff)
	}

	cdlan, _ := loadFixturePayload(t, "testdata/itfull_cdlan_2025.json")
	sc = buildMADeepScorecard(cdlan)
	if sc == nil || sc.Reconciliation == nil {
		t.Fatal("reconciliation mancante su CDLAN")
	}
	rec = sc.Reconciliation
	if rec.EBITDAPct == nil || *rec.EBITDAPct != 0 {
		t.Fatalf("cdlan ebitda pct: %v", rec.EBITDAPct)
	}
	if rec.PFNPct == nil || math.Abs(*rec.PFNPct) > 0.05 {
		t.Fatalf("cdlan pfn pct: %v", rec.PFNPct)
	}
	if rec.ROEPointsDiff == nil || math.Abs(*rec.ROEPointsDiff) > 0.05 {
		t.Fatalf("cdlan roe points: %v", rec.ROEPointsDiff)
	}

	// Payload senza voci CEE: la riconciliazione non deve comparire.
	sc = buildMADeepScorecard(json.RawMessage(`{
		"ecofin": {"turnover": 1000000, "netWorth": 50000},
		"operatingResults": {"ebitda": 30000},
		"leverageRatios": {"pfnEbitda": 7.0}
	}`))
	if sc == nil || sc.Reconciliation != nil {
		t.Fatalf("reconciliation attesa assente: %+v", sc.Reconciliation)
	}
}

func fase2Pricing() maPricing {
	return maPricing{
		SMEHaircutPct: 30, EBITDAFallbackPct: 5, TFRBridgePct: 100,
		A5EBITDAFlagPct: 20, B8RevenueFlagPct: 8,
		ParticipationAssetsFlagPct: 25, ParticipationIncomeFlagPct: 20,
		VendorCEETolerancePct: 1,
	}
}

// TestValuationBandDegeneratesSymmetric: con A.4 e contributi a zero (MFT) il
// prudenziale coincide col reported e la banda resta il ±15% classico; il bridge
// porta PFN CEE, TFR pieno e la riga soci informativa.
func TestValuationBandDegeneratesSymmetric(t *testing.T) {
	payload, _ := loadFixturePayload(t, "testdata/itfull_mft_2024.json")
	sc := buildMADeepScorecard(payload)
	reading := maCEEReadingFromPayload(payload)
	evEbitda := 8.0
	multiple := &sectorMultiple{Industry: "Engineering/Construction", EVEbitda: &evEbitda, NFirms: 100}
	val := buildMADeepValuation(sc, reading, multiple, fase2Pricing())
	if val == nil || val.Method != "ev_ebitda" || val.LowMethod != "" {
		t.Fatalf("method: %+v", val)
	}
	// La PFN canonica dello scorecard è ora quella CEE (14.682, non 14.686 vendor).
	if sc.PFN == nil || *sc.PFN != 14682 {
		t.Fatalf("scorecard PFN: %v", sc.PFN)
	}
	// base = 94.934×8×0.7 = 531.630,4 → banda simmetrica ±15%.
	if val.EVLow != 451886 || val.EVHigh != 611375 {
		t.Fatalf("EV band: %v - %v", val.EVLow, val.EVHigh)
	}
	bridge := val.Bridge
	if bridge == nil || bridge.PFN == nil || bridge.PFN.Value != 14682 || bridge.PFN.Provenance != maCEEProvDetail {
		t.Fatalf("bridge pfn: %+v", bridge)
	}
	if bridge.TFR == nil || bridge.TFR.Value != 100573 {
		t.Fatalf("bridge tfr: %+v", bridge.TFR)
	}
	if bridge.TaxFund != nil {
		t.Fatalf("tax fund atteso assente (0): %+v", bridge.TaxFund)
	}
	if bridge.ShareholderLoans == nil || bridge.ShareholderLoans.Value != 10000 {
		t.Fatalf("soci: %+v", bridge.ShareholderLoans)
	}
	// equity = EV − PFN − TFR (soci informativi, già in PFN).
	if bridge.EquityLow == nil || *bridge.EquityLow != 336631 || *bridge.EquityHigh != 496120 {
		t.Fatalf("equity: %v - %v", bridge.EquityLow, bridge.EquityHigh)
	}
	if val.EquityLow == nil || *val.EquityLow != 336631 {
		t.Fatalf("valuation equityLow non allineato al bridge: %v", val.EquityLow)
	}
}

// TestValuationBandAsymmetricProportional: la banda si allarga esattamente della
// distorsione misurata (A.4+contributi), estremo alto su reported.
func TestValuationBandAsymmetricProportional(t *testing.T) {
	turnover, ebitda := 1000000.0, 200000.0
	sc := &MADeepScorecard{Turnover: &turnover, Ebitda: &ebitda}
	prudential := 120000.0
	reading := &maCEEReading{EBITDAPrudential: &prudential}
	evEbitda := 10.0
	multiple := &sectorMultiple{Industry: "X", EVEbitda: &evEbitda, NFirms: 50}
	pricing := fase2Pricing()
	pricing.SMEHaircutPct = 0
	val := buildMADeepValuation(sc, reading, multiple, pricing)
	if val == nil || val.Method != "ev_ebitda" || val.LowMethod != "" {
		t.Fatalf("method: %+v", val)
	}
	if val.EVHigh != 2300000 { // 200.000×10×1.15
		t.Fatalf("EVHigh: %v", val.EVHigh)
	}
	if val.EVLow != 1020000 { // 120.000×10×0.85
		t.Fatalf("EVLow: %v", val.EVLow)
	}
	if val.PrudentialEbitda == nil || *val.PrudentialEbitda != 120000 {
		t.Fatalf("prudential: %v", val.PrudentialEbitda)
	}
}

// TestValuationLowEndFallsBackToEVSales: prudenziale non positivo → estremo basso
// su EV/Sales (dichiarato in LowMethod), mai sopra l'estremo basso reported.
func TestValuationLowEndFallsBackToEVSales(t *testing.T) {
	turnover, ebitda := 2000000.0, 300000.0
	sc := &MADeepScorecard{Turnover: &turnover, Ebitda: &ebitda}
	prudential := -10000.0
	reading := &maCEEReading{EBITDAPrudential: &prudential}
	evEbitda, evSales := 10.0, 1.0
	multiple := &sectorMultiple{Industry: "X", EVEbitda: &evEbitda, EVSales: &evSales, NFirms: 50}
	pricing := fase2Pricing()
	pricing.SMEHaircutPct = 0
	val := buildMADeepValuation(sc, reading, multiple, pricing)
	if val == nil || val.Method != "ev_ebitda" || val.LowMethod != "ev_sales" {
		t.Fatalf("method/low: %+v", val)
	}
	if val.EVHigh != 3450000 { // reported 300.000×10×1.15
		t.Fatalf("EVHigh: %v", val.EVHigh)
	}
	if val.EVLow != 1700000 { // 2.000.000×1.0×0.85
		t.Fatalf("EVLow: %v", val.EVLow)
	}
}

// TestValuationNegativeEBITDAStaysSymmetricSales: EBITDA reported ≤ 0 → percorso
// EV/Sales con banda simmetrica (la distorsione prudenziale non tocca i ricavi).
func TestValuationNegativeEBITDAStaysSymmetricSales(t *testing.T) {
	turnover, ebitda := 1000000.0, -50000.0
	sc := &MADeepScorecard{Turnover: &turnover, Ebitda: &ebitda}
	evSales := 1.5
	multiple := &sectorMultiple{Industry: "X", EVSales: &evSales, NFirms: 50}
	val := buildMADeepValuation(sc, nil, multiple, fase2Pricing())
	if val == nil || val.Method != "ev_sales" || val.LowMethod != "" {
		t.Fatalf("method: %+v", val)
	}
	if val.EVLow != 892500 || val.EVHigh != 1207500 { // 1.050.000 ±15%
		t.Fatalf("band: %v - %v", val.EVLow, val.EVHigh)
	}
}

// TestValuationBridgeCDLAN: banda asimmetrica reale (contributi 29.375) + bridge
// completo sulla fixture CDLAN.
func TestValuationBridgeCDLAN(t *testing.T) {
	payload, _ := loadFixturePayload(t, "testdata/itfull_cdlan_2025.json")
	sc := buildMADeepScorecard(payload)
	reading := maCEEReadingFromPayload(payload)
	if sc.PFN == nil || *sc.PFN != 1666053 { // PFN CEE, non più ratio vendor
		t.Fatalf("scorecard PFN: %v", sc.PFN)
	}
	evEbitda := 8.0
	multiple := &sectorMultiple{Industry: "Telecom", EVEbitda: &evEbitda, NFirms: 60}
	val := buildMADeepValuation(sc, reading, multiple, fase2Pricing())
	if val == nil || val.Method != "ev_ebitda" || val.LowMethod != "" {
		t.Fatalf("method: %+v", val)
	}
	if val.EVHigh != 16286013 || val.EVLow != 11897663 { // alto su 2.528.884, basso su 2.499.509
		t.Fatalf("band: %v - %v", val.EVLow, val.EVHigh)
	}
	bridge := val.Bridge
	if bridge == nil || bridge.PFN.Value != 1666053 || bridge.TFR.Value != 294528 {
		t.Fatalf("bridge: %+v", bridge)
	}
	if bridge.EquityLow == nil || *bridge.EquityLow != 9937082 || *bridge.EquityHigh != 14325432 {
		t.Fatalf("equity: %v - %v", bridge.EquityLow, bridge.EquityHigh)
	}
}

// TestQualityFlagsRealPayloads: MFT → solo a5_altri_ricavi (33% dell'EBITDA);
// CDLAN → perimetro_standalone (warning, prima) + b8_beni_terzi (info, dopo).
func TestQualityFlagsRealPayloads(t *testing.T) {
	mft, _ := loadFixturePayload(t, "testdata/itfull_mft_2024.json")
	sc := buildMADeepScorecard(mft)
	flags := buildMADeepQualityFlags(sc, maCEEReadingFromPayload(mft), fase2Pricing())
	if len(flags) != 1 || flags[0].Code != "a5_altri_ricavi" || flags[0].Severity != "warning" {
		t.Fatalf("mft flags: %+v", flags)
	}

	cdlan, _ := loadFixturePayload(t, "testdata/itfull_cdlan_2025.json")
	sc = buildMADeepScorecard(cdlan)
	flags = buildMADeepQualityFlags(sc, maCEEReadingFromPayload(cdlan), fase2Pricing())
	if len(flags) != 2 {
		t.Fatalf("cdlan flags: %+v", flags)
	}
	if flags[0].Code != "perimetro_standalone" || flags[0].Severity != "warning" {
		t.Fatalf("primo flag: %+v", flags[0])
	}
	if flags[1].Code != "b8_beni_terzi" || flags[1].Severity != "info" {
		t.Fatalf("secondo flag: %+v", flags[1])
	}
}

// TestMACEELabels verifica che la mappa generata dalla legend copra i codici usati
// dal motore e i tre refusi normalizzati.
func TestMACEELabels(t *testing.T) {
	for _, num := range []string{"089", "331", "184", "070", "219", "130", "149", "144", "127", "129", "133", "145", "177", "178", "179", "231", "232", "351"} {
		if maCEELabels[num] == "" {
			t.Fatalf("etichetta mancante per %s", num)
		}
	}
	if maCEELabel("IIC178") != maCEELabels["178"] || maCEELabel("IICC351") != maCEELabels["351"] {
		t.Fatal("maCEELabel non normalizza")
	}
}
