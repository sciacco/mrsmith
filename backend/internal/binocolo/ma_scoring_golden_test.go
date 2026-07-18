package binocolo

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

// Golden test dello scoring v3 (bande assolute + viability assente≠distressed +
// routing). Inchiodano i comportamenti decisi nella review del modello
// (apps/binocolo/docs/SCORING-MODEL-CRITIQUE.md):
//   1. l'archetipo SRL padronale (bilanci sottili, titolare anziano) NON è
//      punito per il dato assente — entra in lista, non tra i fuori_criterio;
//   2. il "fantasma" ad alta media su pochi segnali resta visibile ma va nella
//      coda da_verificare (contenuto dal routing, non nascosto né travestito);
//   3. i sotto-score economici sono pool-indipendenti: stesso input → stesso
//      score, qualunque sia il resto della ricerca;
//   4. il trend è robusto a un anno sballato (mediana YoY, non estremi);
//   5. il gate settore non ri-boccia un sopravvissuto confermato dal gate
//      semantico (riscatto → cassetto azionabile);
//   6. il distress è thesis-aware nel routing (consolidamento → azionabile).

var goldenNow = time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

func goldenSuccessionStrategy(t *testing.T) MAStrategySpec {
	t.Helper()
	strategy, err := validateMAStrategy(MAStrategySpec{
		SectorDescription: "servizi informatici e software",
		Provinces:         []string{"MI"},
		ActivityStatus:    "ATTIVA",
		Keywords:          []string{"informatica", "software"},
		AtecoCandidates:   []MAAtecoCandidate{{Code: "6209", Description: "Servizi informatici"}, {Code: "6201", Description: "Produzione software"}},
		Thesis:            maThesisSuccession,
	})
	if err != nil {
		t.Fatalf("validate strategy: %v", err)
	}
	return strategy
}

// archetypeTarget è la SRL padronale: titolare unico del 1958 (68 anni al
// goldenNow), azienda del 1995, forma di capitali, NESSUN bilancio depositato
// nel payload (micro-impresa) — il target di successione per eccellenza.
func archetypeTarget() MATarget {
	return MATarget{
		CompanyName:      "ARCHETIPO SOFTWARE SRL",
		AtecoCode:        "620920",
		AtecoDescription: "Servizi informatici e software gestionale",
		ActivityStatus:   "ATTIVA",
		VendorPayload: json.RawMessage(`{
			"companyName": "ARCHETIPO SOFTWARE SRL",
			"vatCode": "01111111111",
			"activityStatus": "ATTIVA",
			"taxCodeCeased": false,
			"startDate": "1995-03-01",
			"detailedLegalForm": {"code": "SR", "description": "SOCIETA' A RESPONSABILITA' LIMITATA"},
			"shareHolders": [
				{"name": "GIOVANNI", "surname": "BIANCHI", "taxCode": "BNCGNN58A01F205X", "percentShare": 100}
			]
		}`),
	}
}

func TestGoldenArchetypeMissingFinancialsIsNeutral(t *testing.T) {
	strategy := goldenSuccessionStrategy(t)
	scored := scoreMATargetsV2([]MATarget{archetypeTarget()}, strategy, maScoringParams{ThesisFitHoldingFactor: 0.4}, goldenNow)
	target := scored[0]

	if target.MatchState == maMatchStateOutside {
		t.Fatalf("l'archetipo non deve finire fuori_criterio: %+v", target.Flags)
	}
	// Nessuna penalità viability per il bilancio assente: l'unica traccia deve
	// essere informativa (flag bilancio_assente + coverage ridotta).
	for _, adj := range target.Adjustments {
		if adj.Code == "viability" {
			t.Errorf("bilancio assente non è distress: adjustment viability %v inatteso", adj.Factor)
		}
	}
	if !maHasFlag(target, "bilancio_assente") {
		t.Errorf("manca il flag informativo bilancio_assente")
	}
	if target.Score < 70 {
		t.Errorf("score archetipo = %d, atteso alto (>=70): il target di successione deve emergere", target.Score)
	}
	if target.Confidence == "alta" {
		t.Errorf("confidence = alta, attesa ridotta: la scarsità la dice la coverage")
	}
	if target.ScoreVersion == nil || *target.ScoreVersion != maScoreVersion {
		t.Errorf("scoreVersion = %v, atteso %d", target.ScoreVersion, maScoreVersion)
	}
}

// ghostTarget ha SOLO fatturato recente e identità: media altissima su pochi
// segnali, coverage bassa.
func ghostTarget() MATarget {
	return MATarget{
		CompanyName:      "FANTASMA SRL",
		AtecoCode:        "620100",
		AtecoDescription: "Produzione di software",
		ActivityStatus:   "ATTIVA",
		VendorPayload: json.RawMessage(`{
			"companyName": "FANTASMA SRL",
			"vatCode": "02222222222",
			"activityStatus": "ATTIVA",
			"taxCodeCeased": false,
			"balanceSheets": {
				"all": [{"year": 2024, "turnover": 1000000}],
				"last": {"year": 2024, "turnover": 1000000}
			}
		}`),
	}
}

func TestGoldenGhostRoutedToVerifyNotHidden(t *testing.T) {
	strategy := goldenSuccessionStrategy(t)
	scored := scoreMATargetsV2([]MATarget{ghostTarget()}, strategy, maScoringParams{ThesisFitHoldingFactor: 0.4}, goldenNow)
	target := scored[0]

	if target.Confidence != "bassa" {
		t.Fatalf("confidence = %q, attesa bassa (coverage sui soli segnali fit)", target.Confidence)
	}
	if target.MatchState != maMatchStatePartial {
		t.Errorf("matchState = %q, atteso %q", target.MatchState, maMatchStatePartial)
	}
	if bucket := maRouteTarget(target, maThesisSuccession).Bucket; bucket != maBucketDaVerificare {
		t.Errorf("bucket = %q, atteso %q: il fantasma va in coda di verifica, non in cima alla lista", bucket, maBucketDaVerificare)
	}
}

// TestGoldenScoreIsPoolIndependent: i sotto-score economici sono assoluti — lo
// stesso payload deve produrre lo stesso score da solo o in mezzo a un pool di
// crescitori (con i percentili lo swing era di decine di punti).
func TestGoldenScoreIsPoolIndependent(t *testing.T) {
	strategy := goldenSuccessionStrategy(t)
	params := maScoringParams{ThesisFitHoldingFactor: 0.4}

	steady := func() MATarget {
		return MATarget{
			CompanyName:      "COSTANTE SRL",
			AtecoCode:        "620920",
			AtecoDescription: "Servizi informatici",
			ActivityStatus:   "ATTIVA",
			VendorPayload: json.RawMessage(`{
				"companyName": "COSTANTE SRL",
				"vatCode": "03333333333",
				"activityStatus": "ATTIVA",
				"taxCodeCeased": false,
				"startDate": "2000-01-01",
				"detailedLegalForm": {"code": "SR", "description": "SRL"},
				"shareHolders": [{"name": "ANNA", "surname": "VERDI", "taxCode": "VRDNNA60A41F205Y", "percentShare": 100}],
				"balanceSheets": {
					"all": [
						{"year": 2022, "turnover": 1000000, "netWorth": 300000, "totalAssets": 900000, "employees": 8},
						{"year": 2023, "turnover": 1100000, "netWorth": 330000, "totalAssets": 950000, "employees": 8},
						{"year": 2024, "turnover": 1210000, "netWorth": 360000, "totalAssets": 1000000, "employees": 8}
					],
					"last": {"year": 2024, "turnover": 1210000, "netWorth": 360000, "totalAssets": 1000000, "employees": 8}
				}
			}`),
		}
	}
	grower := func(i int) MATarget {
		return MATarget{
			CompanyName:      fmt.Sprintf("RAZZO %d SRL", i),
			AtecoCode:        "620100",
			AtecoDescription: "Produzione di software",
			ActivityStatus:   "ATTIVA",
			VendorPayload: json.RawMessage(fmt.Sprintf(`{
				"companyName": "RAZZO %d SRL",
				"vatCode": "0444444444%d",
				"activityStatus": "ATTIVA",
				"taxCodeCeased": false,
				"balanceSheets": {
					"all": [
						{"year": 2022, "turnover": 1000000, "employees": 5},
						{"year": 2023, "turnover": 1500000, "employees": 6},
						{"year": 2024, "turnover": 2250000, "employees": 7}
					],
					"last": {"year": 2024, "turnover": 2250000, "employees": 7}
				}
			}`, i, i)),
		}
	}

	alone := scoreMATargetsV2([]MATarget{steady()}, strategy, params, goldenNow)
	scoreAlone := alone[0].Score

	pool := []MATarget{steady(), grower(1), grower(2), grower(3), grower(4)}
	scoredPool := scoreMATargetsV2(pool, strategy, params, goldenNow)
	scoreInPool := 0
	for _, target := range scoredPool {
		if target.CompanyName == "COSTANTE SRL" {
			scoreInPool = target.Score
		}
	}

	if scoreAlone != scoreInPool {
		t.Errorf("score pool-dipendente: da sola %d, nel pool di crescitori %d — devono coincidere", scoreAlone, scoreInPool)
	}
}

func TestGoldenTrendMedianRobustToBadYear(t *testing.T) {
	ip := func(v int) *int { return &v }
	series := []maBalanceSheet{
		{Year: 2021, Turnover: ip(1_000_000)},
		{Year: 2022, Turnover: ip(1_100_000)},
		{Year: 2023, Turnover: ip(1_210_000)},
		{Year: 2024, Turnover: ip(50_000)}, // anno sballato (refuso/parziale)
	}
	growth, ok := medianAnnualGrowth(series)
	if !ok {
		t.Fatal("growth non calcolato")
	}
	if math.Abs(growth-0.10) > 0.001 {
		t.Errorf("mediana YoY = %.4f, attesa 0.10: un anno sballato non deve ribaltare il trend", growth)
	}
}

func TestGoldenTrendScoreAnchors(t *testing.T) {
	params := maScoringParams{TrendCagrFloor: -0.10, TrendCagrTop: 0.15}
	cases := []struct {
		growth float64
		want   float64
	}{
		{-0.25, 0.1},
		{-0.10, 0.1},
		{-0.05, 0.3},
		{0.0, 0.5},
		{0.05, 2.0 / 3.0},
		{0.15, 1.0},
		{0.40, 1.0},
	}
	for _, tc := range cases {
		if got := maTrendScore(tc.growth, params); math.Abs(got-tc.want) > 0.001 {
			t.Errorf("maTrendScore(%.2f) = %.4f, want %.4f", tc.growth, got, tc.want)
		}
	}
	// I parametri a zero (test/config assente) devono cadere sui default, non su rampe degeneri.
	if got := maTrendScore(0.15, maScoringParams{}); math.Abs(got-1.0) > 0.001 {
		t.Errorf("default anchors: maTrendScore(0.15) = %.4f, want 1.0", got)
	}
}

// TestGoldenSectorMismatchRescue: un sopravvissuto confermato dal gate
// semantico con ATECO fuori divisione resta VISIBILE (flag + cassetto
// azionabile); senza conferma del gate resta fuori_criterio come oggi.
func TestGoldenSectorMismatchRescue(t *testing.T) {
	strategy := goldenSuccessionStrategy(t)
	strategy.SectorDivisions = []string{"62"} // transiente: in produzione lo mette canonicalize

	misCoded := func() MATarget {
		target := ghostTarget()
		target.CompanyName = "MISCODIFICATA SRL"
		target.AtecoCode = "432109" // divisione 43, fuori perimetro
		return target
	}

	confirmed := misCoded()
	confirmed.WebValidation = &MAWebValidation{FinalAction: "confirm"}
	scored := scoreMATargetsV2([]MATarget{confirmed}, strategy, maScoringParams{ThesisFitHoldingFactor: 0.4}, goldenNow)
	rescued := scored[0]
	if rescued.MatchState == maMatchStateOutside {
		t.Fatalf("il gate semantico ha confermato: il mismatch ATECO non deve nascondere la riga")
	}
	if !maHasFlag(rescued, maFlagAtecoFuoriPerimetro) {
		t.Errorf("manca il flag %s", maFlagAtecoFuoriPerimetro)
	}
	if bucket := maRouteTarget(rescued, maThesisSuccession).Bucket; bucket != maBucketAzionabile {
		t.Errorf("bucket = %q, atteso %q (azione: rivedi/override)", bucket, maBucketAzionabile)
	}

	unconfirmed := misCoded()
	scored = scoreMATargetsV2([]MATarget{unconfirmed}, strategy, maScoringParams{ThesisFitHoldingFactor: 0.4}, goldenNow)
	hidden := scored[0]
	if hidden.MatchState != maMatchStateOutside {
		t.Errorf("senza conferma del gate il fuori-divisione resta fuori_criterio, got %q", hidden.MatchState)
	}
	if bucket := maRouteTarget(hidden, maThesisSuccession).Bucket; bucket != maBucketSoppresso {
		t.Errorf("bucket = %q, atteso %q", bucket, maBucketSoppresso)
	}
}

// TestGoldenDistressRoutingIsThesisAware: il knockout per distress va nel
// cassetto azionabile sotto consolidamento (il distress È l'opportunità),
// soppresso sotto le altre tesi.
func TestGoldenDistressRoutingIsThesisAware(t *testing.T) {
	strategy, err := validateMAStrategy(MAStrategySpec{
		SectorDescription: "servizi informatici",
		Provinces:         []string{"TV"},
		ActivityStatus:    "ATTIVA",
		AtecoCandidates:   []MAAtecoCandidate{{Code: "6290", Description: "Servizi informatici"}},
		Thesis:            maThesisConsolidation,
	})
	if err != nil {
		t.Fatalf("validate strategy: %v", err)
	}
	scored := scoreMATargetsV2([]MATarget{distressedTarget()}, strategy, maScoringParams{ThesisFitHoldingFactor: 1.0}, goldenNow)
	target := scored[0]
	if target.MatchState != maMatchStateOutside {
		t.Fatalf("il distress conclamato resta knockout, got %q", target.MatchState)
	}
	if !maHasFlagPrefix(target, maFlagKnockoutVitalita+"_"+maViabilityReasonDistress) {
		t.Fatalf("manca il flag knockout distress: %+v", target.Flags)
	}
	if bucket := maRouteTarget(target, maThesisConsolidation).Bucket; bucket != maBucketAzionabile {
		t.Errorf("consolidamento: bucket = %q, atteso %q", bucket, maBucketAzionabile)
	}
	if bucket := maRouteTarget(target, maThesisSuccession).Bucket; bucket != maBucketSoppresso {
		t.Errorf("successione: bucket = %q, atteso %q", bucket, maBucketSoppresso)
	}
}

func TestGoldenSuppressedRoutingReasonsAndRatingOverride(t *testing.T) {
	positive := 1
	zero := 0
	negative := -1
	base := func() MATarget { return MATarget{MatchState: maMatchStateOutside, AtecoCode: "620100"} }

	cases := []struct {
		name   string
		target MATarget
		code   string
	}{
		{name: "post filter", target: func() MATarget {
			target := base()
			target.Evidence = []MATargetEvidence{{Criterion: maPostFilterRevenuePerEmployeeMin, Status: maEvidenceOutside}}
			return target
		}(), code: "post_filter"},
		{name: "ceased", target: func() MATarget {
			target := base()
			target.Flags = []MATargetFlag{{Code: "cessata_fiscalmente", Label: "Cessata fiscalmente"}}
			return target
		}(), code: "ceased"},
		{name: "inactive", target: func() MATarget {
			target := base()
			target.Flags = []MATargetFlag{{Code: maFlagKnockoutVitalita + "_" + maViabilityReasonInactive, Label: "Società dormiente"}}
			return target
		}(), code: "inactive"},
		{name: "distress", target: func() MATarget {
			target := base()
			target.Flags = []MATargetFlag{{Code: maFlagKnockoutVitalita + "_" + maViabilityReasonDistress, Label: "Distress conclamato"}}
			return target
		}(), code: "distress"},
		{name: "off sector", target: base(), code: "off_sector"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, rating := range []*int{nil, &zero, &negative} {
				target := tc.target
				target.Rating = rating
				decision := maRouteTarget(target, maThesisSuccession)
				if decision.Bucket != maBucketSoppresso || decision.Reason == nil || decision.Reason.Code != tc.code {
					t.Fatalf("decisione senza override = %+v, atteso soppresso/%s", decision, tc.code)
				}
			}
			target := tc.target
			target.Rating = &positive
			decision := maRouteTarget(target, maThesisSuccession)
			if decision.Bucket != maBucketDaVerificare || decision.Reason != nil || decision.SuppressedReason == nil || decision.SuppressedReason.Code != tc.code {
				t.Fatalf("decisione con override = %+v, atteso da_verificare/%s", decision, tc.code)
			}
		})
	}

	postFilterFallback := maRouteTarget(cases[0].target, maThesisSuccession)
	if postFilterFallback.Reason == nil || postFilterFallback.Reason.Label != "Ricavo per dipendente sotto la soglia impostata nella ricerca" {
		t.Errorf("fallback post-filter = %+v", postFilterFallback.Reason)
	}
	projectedValue := "42.000 €/dip anno 2024 (min 50.000)"
	projected := rowAsTarget(MATargetRow{
		MatchState:                     maMatchStateOutside,
		OutsideRevenuePerEmployeeValue: &projectedValue,
	})
	projectedDecision := maRouteTarget(projected, maThesisSuccession)
	if projectedDecision.Reason == nil || !strings.Contains(projectedDecision.Reason.Label, projectedValue) {
		t.Errorf("motivo post-filter proiettato senza valore: %+v", projectedDecision.Reason)
	}

	identityOnly := MATarget{
		Rating: &positive,
		WebValidation: &MAWebValidation{
			FinalAction:    "reject",
			SelectedDomain: "example.it",
			FinalDecision:  CandidateMatchFinalDecision{Reason: "Attività non coerente"},
		},
	}
	decision := maRouteTarget(identityOnly, maThesisSuccession)
	if decision.Bucket != maBucketSoppresso || decision.Reason == nil || decision.Reason.Code != "gate_reject" {
		t.Fatalf("gate reject identity-only = %+v, non deve essere ripristinata", decision)
	}
	if !strings.Contains(decision.Reason.Label, "Attività non coerente") || !strings.Contains(decision.Reason.Label, "example.it") {
		t.Errorf("label gate reject non composta dai fatti: %q", decision.Reason.Label)
	}
}

// TestGoldenTieBreakBySubstance: a parità di score l'ordine di lettura premia
// confidence e fatturato, mai l'alfabeto da solo.
func TestGoldenTieBreakBySubstance(t *testing.T) {
	ip := func(v int) *int { return &v }
	targets := []MATarget{
		{CompanyName: "AAA SRL", Score: 80, Confidence: "bassa"},
		{CompanyName: "ZZZ SRL", Score: 80, Confidence: "alta", Turnover: ip(500_000)},
		{CompanyName: "MMM SRL", Score: 80, Confidence: "alta", Turnover: ip(2_000_000)},
	}
	sortMATargetsByRating(targets)
	got := []string{targets[0].CompanyName, targets[1].CompanyName, targets[2].CompanyName}
	want := []string{"MMM SRL", "ZZZ SRL", "AAA SRL"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordine = %v, atteso %v", got, want)
		}
	}
}
