package binocolo

// Famiglia di business model (Fase 3, filoni C+D del redesign deep-dive):
// UNA classificazione, DUE consumatori — le soglie RAG del motore (metriche
// strutturalmente dipendenti dal modello: margine, ROS, ciclo) e la riga
// Damodaran (chiave sintetica FAMILY:*, mig 094). Il suggerimento è
// deterministico (ATECO univoco, oppure struttura dei costi CE per il blob
// 62/63 dove il codice camerale non separa MSP, software house e rivendita)
// e porta la sua evidenza; la ratifica è dell'analista. Nessun LLM.

import (
	"context"
	"fmt"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	maBMFamilyServiziRicorrenti    = "servizi_ricorrenti"
	maBMFamilyProgettoIntegrazione = "progetto_integrazione"
	maBMFamilyRivenditaVAR         = "rivendita_var"
	maBMFamilySoftwareProdotto     = "software_prodotto"
)

var maBMFamilies = map[string]bool{
	maBMFamilyServiziRicorrenti:    true,
	maBMFamilyProgettoIntegrazione: true,
	maBMFamilyRivenditaVAR:         true,
	maBMFamilySoftwareProdotto:     true,
}

// maDispersedMultiplePrefixes: prefissi ATECO le cui righe Damodaran coprono
// business model a multipli molto dispersi — senza una famiglia ratificata la
// valuation porta il caveat di eterogeneità.
var maDispersedMultiplePrefixes = map[string]bool{
	"46": true, "58": true, "62": true, "620": true, "6201": true, "63": true,
}

// maFamilyFirstPrefixes: prefissi la cui riga Damodaran è FUORVIANTE per identità
// (il blob disperso 62/63/46/58, più il 43 che mappa "Construction Supplies" —
// fornitori di materiali — anche sugli installatori). Solo qui la riga famiglia
// scavalca il prefisso; altrove un prefisso specifico corretto (es. 61 → Telecom)
// batte il proxy di famiglia, e la famiglia resta fallback prima del TOTAL.
var maFamilyFirstPrefixes = map[string]bool{
	"43": true, "46": true, "58": true, "62": true, "620": true, "6201": true, "63": true,
}

type maBMFamilySuggestion struct {
	Family   string
	Source   string // ateco | cost_structure
	Evidence string
}

// suggestBMFamily propone la famiglia: deterministico da ATECO dove il codice è
// univoco, dalla struttura dei costi CE per il blob 62/63. Ritorna nil quando la
// base non è sufficiente (nessuna famiglia = soglie di default, multiplo da
// prefisso).
func suggestBMFamily(atecoCode string, reading *maCEEReading) *maBMFamilySuggestion {
	code := strings.ReplaceAll(normalizeAtecoCode(atecoCode), ".", "")
	switch {
	case strings.HasPrefix(code, "4651") || strings.HasPrefix(code, "4741"):
		return &maBMFamilySuggestion{maBMFamilyRivenditaVAR, "ateco", "ATECO " + code + ": commercio di computer e apparecchiature"}
	case strings.HasPrefix(code, "432"):
		return &maBMFamilySuggestion{maBMFamilyProgettoIntegrazione, "ateco", "ATECO " + code + ": installazione di impianti"}
	case strings.HasPrefix(code, "61"):
		return &maBMFamilySuggestion{maBMFamilyServiziRicorrenti, "ateco", "ATECO " + code + ": telecomunicazioni"}
	case strings.HasPrefix(code, "62") || strings.HasPrefix(code, "63"):
		return suggestBMFamilyFromCosts(reading)
	}
	return nil
}

// suggestBMFamilyFromCosts: euristica da struttura CE (il trucco da analista).
// Ordine: segnali forti prima; nessun segnale → nil (decide l'analista).
func suggestBMFamilyFromCosts(reading *maCEEReading) *maBMFamilySuggestion {
	if reading == nil || reading.ProductionValue == nil || *reading.ProductionValue <= 0 {
		return nil
	}
	production := *reading.ProductionValue
	if share := reading.MaterialCosts / production * 100; share > 40 {
		return &maBMFamilySuggestion{maBMFamilyRivenditaVAR, "cost_structure",
			fmt.Sprintf("B.6 materie/merci = %.0f%% del valore della produzione", share)}
	}
	if reading.EBITDACEE != nil && *reading.EBITDACEE > 0 {
		if share := reading.Capitalizations / *reading.EBITDACEE * 100; share > 15 {
			return &maBMFamilySuggestion{maBMFamilySoftwareProdotto, "cost_structure",
				fmt.Sprintf("A.4 capitalizzazioni = %.0f%% dell'EBITDA (sviluppo interno)", share)}
		}
	}
	if reading.Inventory != nil && reading.TotalAssets != nil && *reading.TotalAssets > 0 {
		if share := *reading.Inventory / *reading.TotalAssets * 100; share > 15 {
			return &maBMFamilySuggestion{maBMFamilyProgettoIntegrazione, "cost_structure",
				fmt.Sprintf("magazzino = %.0f%% dell'attivo (commesse/WIP)", share)}
		}
	}
	addedValue := production - reading.MaterialCosts - reading.ServiceCosts - reading.LeaseCosts - reading.OtherOperatingCosts
	inventoryLight := reading.Inventory == nil ||
		(reading.TotalAssets != nil && *reading.TotalAssets > 0 && *reading.Inventory / *reading.TotalAssets * 100 < 5)
	if addedValue > 0 && inventoryLight {
		if share := reading.StaffCosts / addedValue * 100; share > 45 {
			return &maBMFamilySuggestion{maBMFamilyServiziRicorrenti, "cost_structure",
				fmt.Sprintf("costo del personale = %.0f%% del valore aggiunto, magazzino trascurabile", share)}
		}
	}
	return nil
}

// sectorMultipleLookupKeys è l'ordine di risoluzione del multiplo. La riga
// famiglia scavalca i prefissi SOLO dove il prefisso è fuorviante
// (maFamilyFirstPrefixes): un prefisso specifico corretto — es. 61 → Telecom
// Services — è un comparable migliore del proxy di famiglia. Altrove la famiglia
// resta il fallback prima della riga TOTAL di mercato.
func sectorMultipleLookupKeys(family, ateco string) []string {
	code := strings.ReplaceAll(normalizeAtecoCode(ateco), ".", "")
	prefixes := make([]string, 0, 3)
	familyFirst := false
	for _, n := range []int{4, 3, 2} {
		if len(code) >= n {
			prefixes = append(prefixes, code[:n])
			if maFamilyFirstPrefixes[code[:n]] {
				familyFirst = true
			}
		}
	}
	keys := make([]string, 0, 5)
	if maBMFamilies[family] && familyFirst {
		keys = append(keys, "FAMILY:"+family)
	}
	keys = append(keys, prefixes...)
	if maBMFamilies[family] && !familyFirst {
		keys = append(keys, "FAMILY:"+family)
	}
	return append(keys, "TOTAL")
}

// maDeepFamilyStore è la persistenza che serve al calcolo degli artefatti deep
// con famiglia: soddisfatta da *SQLStore, condivisa da worker e recompute.
type maDeepFamilyStore interface {
	UpsertMABMFamilySuggestion(ctx context.Context, companyKey, vatCode, taxCode, companyName string, suggestion maBMFamilySuggestion) error
	GetMABMFamily(ctx context.Context, companyKey string) (*MABMFamily, error)
	ResolveSectorMultipleKeys(ctx context.Context, keys []string) (*sectorMultiple, error)
}

// computeMADeepScorecard è il percorso condiviso worker/recompute/regenerate:
// lettura CEE → suggerimento famiglia (upsert best-effort, mai sovrascrive la
// ratifica) → famiglia effettiva → soglie → scorecard + flag di qualità.
func computeMADeepScorecard(ctx context.Context, store maDeepFamilyStore, payload []byte, companyKey, vatCode, taxCode string, pricing maPricing) (*MADeepScorecard, *maCEEReading, string, bool) {
	reading := maCEEReadingFromPayload(payload)
	object, err := decodeVendorObject(payload)
	var root map[string]any
	if err == nil && object != nil {
		root = deepFullRoot(object)
	}
	ateco := normalizeAtecoCode(firstVendorString(root,
		"atecoClassification.ateco.code", "atecoClassification.ateco2022.code", "ateco.code"))
	companyName := firstVendorString(root, "companyDetails.companyName")

	if suggestion := suggestBMFamily(ateco, reading); suggestion != nil {
		if err := store.UpsertMABMFamilySuggestion(ctx, companyKey, vatCode, taxCode, companyName, *suggestion); err != nil {
			logging.FromContext(ctx).Warn("binocolo bm family suggestion upsert failed", "component", "binocolo", "company_key", companyKey, "error", err)
		}
	}
	family, ratified := "", false
	if record, err := store.GetMABMFamily(ctx, companyKey); err != nil {
		logging.FromContext(ctx).Warn("binocolo bm family load failed", "component", "binocolo", "company_key", companyKey, "error", err)
	} else if record != nil {
		family, ratified = record.Effective()
	}
	scorecard := buildMADeepScorecard(payload, pricing.thresholdsForFamily(family))
	if scorecard != nil {
		scorecard.QualityFlags = buildMADeepQualityFlags(scorecard, reading, pricing)
	}
	return scorecard, reading, family, ratified
}

// resolveMADeepValuation risolve il multiplo (famiglia → prefisso → TOTAL) e
// costruisce la valuation; famiglia usata ma non ratificata → caveat esplicito,
// nessuna famiglia su prefisso disperso → caveat di eterogeneità.
func resolveMADeepValuation(ctx context.Context, store maDeepFamilyStore, sc *MADeepScorecard, reading *maCEEReading, family string, ratified bool, pricing maPricing) *MADeepValuation {
	if sc == nil {
		return nil
	}
	multiple, err := store.ResolveSectorMultipleKeys(ctx, sectorMultipleLookupKeys(family, sc.AtecoCode))
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo sector multiple resolve failed", "component", "binocolo", "ateco", sc.AtecoCode, "error", err)
		return nil
	}
	valuation := buildMADeepValuation(sc, reading, multiple, pricing)
	if valuation == nil || multiple == nil {
		return valuation
	}
	switch {
	case strings.HasPrefix(multiple.Prefix, "FAMILY:") && !ratified:
		valuation.Caveat = strings.TrimSpace("Famiglia di business model suggerita ma non ratificata: multiplo per famiglia indicativo. " + valuation.Caveat)
	case family == "" && maDispersedMultiplePrefixes[multiple.Prefix]:
		valuation.Caveat = strings.TrimSpace("Settore a multipli dispersi e famiglia non classificata: il posizionamento di business può spostare il multiplo sensibilmente. " + valuation.Caveat)
	}
	return valuation
}
