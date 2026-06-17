package binocolo

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestComputeViability(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	ip := func(v int) *int { return &v }

	ctx := func(status string, ceased bool, lastYear int, turnover, netWorth, prevNetWorth *int) maSignalContext {
		return maSignalContext{
			target: MATarget{ActivityStatus: status},
			object: map[string]any{"taxCodeCeased": ceased},
			fin: maFinancials{
				LastYear:     lastYear,
				Turnover:     turnover,
				NetWorth:     netWorth,
				PrevNetWorth: prevNetWorth,
			},
			now: now,
		}
	}

	cases := []struct {
		name         string
		ctx          maSignalContext
		wantFactor   float64
		wantKnockout bool
	}{
		{
			// attiva, bilancio 2024 (gap 2 fisiologico), PN positivo e stabile
			name:         "sana",
			ctx:          ctx("ATTIVA", false, 2024, ip(1_200_000), ip(500_000), ip(450_000)),
			wantFactor:   1.0,
			wantKnockout: false,
		},
		{
			// nel 2026 un bilancio 2024 non deve penalizzare (resta solo come flag)
			name:         "finestra fisiologica gap 2",
			ctx:          ctx("ATTIVA", false, 2024, ip(800_000), ip(100_000), nil),
			wantFactor:   1.0,
			wantKnockout: false,
		},
		{
			name:         "bilancio gap 3 lieve penalita",
			ctx:          ctx("ATTIVA", false, 2023, ip(800_000), ip(100_000), nil),
			wantFactor:   0.8,
			wantKnockout: false,
		},
		{
			// FOOD RACERS: bilancio 2024 fisiologico ma PN negativo -> score basso, non escluso
			name:         "food racers: PN negativo, bilancio recente",
			ctx:          ctx("ATTIVA", false, 2024, ip(1_169_588), ip(-50_000), nil),
			wantFactor:   0.3,
			wantKnockout: false,
		},
		{
			// MARCUZZO: bilancio 2018 (gap 8) + PN eroso -> knockout
			name:         "marcuzzo: bilancio datato + PN eroso",
			ctx:          ctx("ATTIVA", false, 2018, ip(0), ip(10_000), ip(100_000)),
			wantFactor:   0.15, // 1.0 * 0.25 * 0.6
			wantKnockout: true,
		},
		{
			name:         "cessata fiscalmente: knockout immediato",
			ctx:          ctx("ATTIVA", true, 2024, ip(900_000), ip(200_000), nil),
			wantFactor:   0.0,
			wantKnockout: true,
		},
		{
			name:         "non attiva: sotto soglia -> knockout",
			ctx:          ctx("INATTIVA", false, 2024, ip(900_000), ip(200_000), nil),
			wantFactor:   0.2,
			wantKnockout: true,
		},
		{
			name:         "bilancio assente (turnover nil)",
			ctx:          ctx("ATTIVA", false, 0, nil, nil, nil),
			wantFactor:   0.3,
			wantKnockout: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			factor, knockout := computeViability(tc.ctx)
			if math.Abs(factor-tc.wantFactor) > 1e-9 {
				t.Errorf("factor = %.4f, want %.4f", factor, tc.wantFactor)
			}
			if knockout != tc.wantKnockout {
				t.Errorf("knockout = %v, want %v", knockout, tc.wantKnockout)
			}
		})
	}
}

// TestScoreMATargetsViabilityKnockout verifies the wiring: a distressed company
// (stale balance + negative equity) is collapsed below a healthy one and demoted
// to fuori_criterio, even when its sector fit is identical.
func TestScoreMATargetsViabilityKnockout(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	around := 1_000_000
	strategy, err := validateMAStrategy(MAStrategySpec{
		SectorDescription: "servizi informatici",
		Provinces:         []string{"TV"},
		ActivityStatus:    "ATTIVA",
		TurnoverAround:    &around,
		AtecoCandidates:   []MAAtecoCandidate{{Code: "6290", Description: "Servizi informatici"}},
		Thesis:            maThesisConsolidation,
	})
	if err != nil {
		t.Fatalf("validate strategy: %v", err)
	}

	targets := scoreMATargetsV2([]MATarget{bfInformaticaTarget(), distressedTarget()}, strategy, now)

	healthy := targetByName(targets, "BFINFORMATICA")
	distressed := targetByName(targets, "DISTRESSED")
	if healthy == nil || distressed == nil {
		t.Fatalf("missing target in result: %+v", targets)
	}
	if distressed.MatchState != maMatchStateOutside {
		t.Errorf("distressed matchState = %q, want %q", distressed.MatchState, maMatchStateOutside)
	}
	if distressed.Score >= healthy.Score {
		t.Errorf("distressed score %d should sink below healthy %d", distressed.Score, healthy.Score)
	}
}

func targetByName(targets []MATarget, name string) *MATarget {
	for i := range targets {
		if strings.Contains(targets[i].CompanyName, name) {
			return &targets[i]
		}
	}
	return nil
}

func distressedTarget() MATarget {
	return MATarget{
		CompanyName:      "DISTRESSED SRL",
		AtecoCode:        "629009",
		AtecoDescription: "Altre attivita' dei servizi connessi alle tecnologie dell'informatica n.c.a.",
		VendorPayload:    json.RawMessage(distressedPayload),
	}
}

const distressedPayload = `{
  "companyName": "DISTRESSED SRL",
  "vatCode": "01234567890",
  "taxCode": "01234567890",
  "activityStatus": "ATTIVA",
  "taxCodeCeased": false,
  "startDate": "2000-01-01",
  "registrationDate": "2000-01-01",
  "detailedLegalForm": {"code": "SR", "description": "SOCIETA' A RESPONSABILITA' LIMITATA"},
  "atecoClassification": {"ateco": {"code": "629009"}},
  "shareHolders": [
    {"name": "MARIO", "surname": "ROSSI", "taxCode": "RSSMRA60A01H501U", "percentShare": 100}
  ],
  "balanceSheets": {
    "all": [
      {"year": 2018, "turnover": 0, "netWorth": -50000}
    ],
    "last": {"year": 2018, "turnover": 0, "netWorth": -50000}
  }
}`
