package binocolo

// Test Fase 3 (approvati): classificatore famiglia (ATECO univoci + euristica
// struttura costi per il blob 62/63), ordine di risoluzione del multiplo, soglie
// per famiglia, lente compratore (contorno fuori dall'overall), haircut graduato.

import (
	"context"
	"testing"
)

func TestSuggestBMFamily(t *testing.T) {
	// ATECO univoci dalle fixture reali.
	mft, _ := loadFixturePayload(t, "testdata/itfull_mft_2024.json")
	s := suggestBMFamily("432102", maCEEReadingFromPayload(mft))
	if s == nil || s.Family != maBMFamilyProgettoIntegrazione || s.Source != "ateco" {
		t.Fatalf("mft: %+v", s)
	}
	cdlan, _ := loadFixturePayload(t, "testdata/itfull_cdlan_2025.json")
	s = suggestBMFamily("61901", maCEEReadingFromPayload(cdlan))
	if s == nil || s.Family != maBMFamilyServiziRicorrenti || s.Source != "ateco" {
		t.Fatalf("cdlan: %+v", s)
	}

	// Blob 62: euristica dalla struttura dei costi CE.
	production, assets := 1000.0, 1000.0
	ebitda := 200.0
	inventory := 200.0
	cases := []struct {
		name    string
		reading maCEEReading
		want    string
	}{
		{"merci dominanti -> rivendita", maCEEReading{ProductionValue: &production, MaterialCosts: 500}, maBMFamilyRivenditaVAR},
		{"capitalizzazioni -> software", maCEEReading{ProductionValue: &production, MaterialCosts: 100, EBITDACEE: &ebitda, Capitalizations: 40}, maBMFamilySoftwareProdotto},
		{"magazzino -> progetto", maCEEReading{ProductionValue: &production, MaterialCosts: 100, Inventory: &inventory, TotalAssets: &assets}, maBMFamilyProgettoIntegrazione},
		{"personale su VA -> servizi", maCEEReading{ProductionValue: &production, MaterialCosts: 50, ServiceCosts: 200, StaffCosts: 400}, maBMFamilyServiziRicorrenti},
	}
	for _, tc := range cases {
		got := suggestBMFamily("6202", &tc.reading)
		if got == nil || got.Family != tc.want || got.Source != "cost_structure" || got.Evidence == "" {
			t.Fatalf("%s: %+v", tc.name, got)
		}
	}

	// Nessuna base: niente suggerimento.
	if s := suggestBMFamily("6202", nil); s != nil {
		t.Fatalf("senza reading: %+v", s)
	}
	if s := suggestBMFamily("7022", maCEEReadingFromPayload(mft)); s != nil {
		t.Fatalf("ateco fuori perimetro: %+v", s)
	}
}

func TestSectorMultipleLookupKeys(t *testing.T) {
	assertKeys := func(name string, got, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: %v, want %v", name, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s[%d] = %q, want %q", name, i, got[i], want[i])
			}
		}
	}
	// Prefisso fuorviante (blob 62): la famiglia scavalca.
	assertKeys("62 dispersa", sectorMultipleLookupKeys(maBMFamilyServiziRicorrenti, "62.02"),
		[]string{"FAMILY:servizi_ricorrenti", "6202", "620", "62", "TOTAL"})
	// Prefisso 43 (Construction Supplies fuorviante per gli installatori): idem.
	assertKeys("43 installatori", sectorMultipleLookupKeys(maBMFamilyProgettoIntegrazione, "432102"),
		[]string{"FAMILY:progetto_integrazione", "4321", "432", "43", "TOTAL"})
	// Prefisso specifico CORRETTO (61 → Telecom): la famiglia resta fallback —
	// un ISP non deve prendere il multiplo Computer Services.
	assertKeys("61 telecom", sectorMultipleLookupKeys(maBMFamilyServiziRicorrenti, "61901"),
		[]string{"6190", "619", "61", "FAMILY:servizi_ricorrenti", "TOTAL"})
	// Senza famiglia: sola catena prefissi.
	assertKeys("senza famiglia", sectorMultipleLookupKeys("", "61901"),
		[]string{"6190", "619", "61", "TOTAL"})
	assertKeys("bogus", sectorMultipleLookupKeys("famiglia_bogus", "6"), []string{"TOTAL"})
}

// TestFamilyThresholdsChangeRAG: MFT con soglie progetto/integrazione — il margine
// 7,77% passa da rosso (default ok=8) ad ambra (ok=6), e leva/capitalizzazione
// diventano contorno (visibili, fuori dall'overall).
func TestFamilyThresholdsChangeRAG(t *testing.T) {
	mft, _ := loadFixturePayload(t, "testdata/itfull_mft_2024.json")

	def := buildMADeepScorecard(mft, defaultMADeepThresholds())
	if m := findDeepMetric(def, "ebitda_margin"); m == nil || m.RAG != maRAGRed {
		t.Fatalf("default margin: %+v", m)
	}

	sc := buildMADeepScorecard(mft, maFamilyThresholdDefaults[maBMFamilyProgettoIntegrazione])
	if m := findDeepMetric(sc, "ebitda_margin"); m == nil || m.RAG != maRAGAmber {
		t.Fatalf("progetto margin: %+v", m)
	}
	if m := findDeepMetric(sc, "financial_cycle"); m == nil || m.RAG != maRAGAmber {
		t.Fatalf("progetto ciclo: %+v", m)
	}
	for _, key := range []string{"leverage", "capitalizzazione", "current_ratio", "acid_test", "roe"} {
		if m := findDeepMetric(sc, key); m == nil || m.Tier != maMetricTierContorno {
			t.Fatalf("%s deve essere contorno: %+v", key, m)
		}
	}
	// MFT resta rossa anche con la lente compratore, ma per ragioni CORE (ROS
	// 1,56%% e ricavi in calo), non per il rumore patrimoniale di contorno.
	if sc.OverallRAG != maRAGRed {
		t.Fatalf("overall: %s", sc.OverallRAG)
	}
}

// TestDeepOverallRAGIgnoresContorno: i rossi di contorno non affondano il semaforo.
func TestDeepOverallRAGIgnoresContorno(t *testing.T) {
	v := 1.0
	metrics := []MADeepMetric{
		{Key: "leverage", RAG: maRAGRed, Tier: maMetricTierContorno, Value: &v},
		{Key: "capitalizzazione", RAG: maRAGRed, Tier: maMetricTierContorno, Value: &v},
		{Key: "ebitda_margin", RAG: maRAGGreen, Value: &v},
		{Key: "pfn_ebitda", RAG: maRAGGreen, Value: &v},
	}
	if rag := deepOverallRAG(metrics); rag != maRAGGreen {
		t.Fatalf("overall con contorno rossi: %s", rag)
	}
	metrics[2].RAG = maRAGRed
	if rag := deepOverallRAG(metrics); rag != maRAGAmber {
		t.Fatalf("un rosso core: %s", rag)
	}
}

func TestHaircutTiers(t *testing.T) {
	pricing := fase2Pricing()
	pricing.HaircutTier1Pct, pricing.HaircutTier2Pct, pricing.HaircutTier3Pct = 35, 30, 20
	pricing.HaircutTier1MaxEUR, pricing.HaircutTier2MaxEUR = 5_000_000, 20_000_000
	cases := []struct {
		turnover *float64
		want     float64
	}{
		{ptrF(1_200_000), 35},
		{ptrF(12_400_000), 30},
		{ptrF(25_000_000), 20},
		{nil, 30}, // fallback flat
	}
	for _, tc := range cases {
		if got := pricing.haircutForTurnover(tc.turnover); got != tc.want {
			t.Fatalf("turnover %v: haircut %v, want %v", tc.turnover, got, tc.want)
		}
	}
	// Pricing senza configurazione tier (test/legacy) → flat storico.
	if got := fase2Pricing().haircutForTurnover(ptrF(1_000_000)); got != 30 {
		t.Fatalf("senza tier: %v", got)
	}
}

func ptrF(v float64) *float64 { return &v }

// fakeFamilyStore per resolveMADeepValuation: ritorna il multiplo predisposto.
type fakeFamilyStore struct {
	multiple *sectorMultiple
}

func (f *fakeFamilyStore) UpsertMABMFamilySuggestion(context.Context, string, string, string, string, maBMFamilySuggestion) error {
	return nil
}
func (f *fakeFamilyStore) GetMABMFamily(context.Context, string) (*MABMFamily, error) {
	return nil, nil
}
func (f *fakeFamilyStore) ResolveSectorMultipleKeys(context.Context, []string) (*sectorMultiple, error) {
	return f.multiple, nil
}

// TestResolveMADeepValuationCaveats: famiglia usata ma non ratificata → caveat;
// nessuna famiglia su prefisso disperso → caveat di eterogeneità.
func TestResolveMADeepValuationCaveats(t *testing.T) {
	turnover, ebitda, pfn := 1_000_000.0, 200_000.0, 50_000.0
	sc := &MADeepScorecard{Turnover: &turnover, Ebitda: &ebitda, PFN: &pfn, AtecoCode: "62.02"}
	evEbitda := 10.0
	pricing := fase2Pricing()

	famRow := &sectorMultiple{Prefix: "FAMILY:servizi_ricorrenti", Industry: "Computer Services", EVEbitda: &evEbitda, NFirms: 210}
	val := resolveMADeepValuation(context.Background(), &fakeFamilyStore{multiple: famRow}, sc, nil, maBMFamilyServiziRicorrenti, false, pricing)
	if val == nil || !containsStr(val.Caveat, "non ratificata") {
		t.Fatalf("caveat famiglia non ratificata: %+v", val)
	}
	val = resolveMADeepValuation(context.Background(), &fakeFamilyStore{multiple: famRow}, sc, nil, maBMFamilyServiziRicorrenti, true, pricing)
	if val == nil || containsStr(val.Caveat, "non ratificata") {
		t.Fatalf("ratificata non deve avere caveat: %+v", val)
	}

	dispersedRow := &sectorMultiple{Prefix: "62", Industry: "Software (System & Application)", EVEbitda: &evEbitda, NFirms: 290}
	val = resolveMADeepValuation(context.Background(), &fakeFamilyStore{multiple: dispersedRow}, sc, nil, "", false, pricing)
	if val == nil || !containsStr(val.Caveat, "multipli dispersi") {
		t.Fatalf("caveat dispersione: %+v", val)
	}
}

func containsStr(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexStr(haystack, needle) >= 0
}

func indexStr(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
