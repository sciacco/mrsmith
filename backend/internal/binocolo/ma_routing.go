package binocolo

import (
	"fmt"
	"strings"
)

// Routing scarti — le tre destinazioni di presentazione (più la lista principale).
//
// Il test che decide la destinazione: c'è un'azione specifica che rimetterebbe
// questo elemento in gioco? Se sì → cassetto azionabile (poche righe, segnale
// alto). Se no → soppresso, con onestà: la UI mostra il conteggio, non le righe.
// I bucket sono DERIVATI A LETTURA dai campi persistiti (matchState, flags,
// evidence, web-validation, confidence) — mai salvati: cambiare la regola di
// routing non richiede re-score.
const (
	maBucketPrincipale   = "principale"
	maBucketDaVerificare = "da_verificare"
	maBucketAzionabile   = "azionabile"
	maBucketSoppresso    = "soppresso"
)

type MABucketReason struct {
	Code  string `json:"code"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type MARouteDecision struct {
	Bucket           string
	Reason           *MABucketReason
	SuppressedReason *MABucketReason
}

// maRouteTarget assegna la destinazione di presentazione di un target. Il
// verdetto resta derivato a lettura: una valutazione positiva dell'analista
// rimette una soppressa tra i target da verificare, conservando il motivo
// controfattuale che la UI deve rendere visibile.
func maRouteTarget(target MATarget, thesis string) MARouteDecision {
	decision := maRouteTargetBase(target, thesis)
	if decision.Bucket == maBucketSoppresso && target.MatchState != "" && target.Rating != nil && *target.Rating >= 1 {
		return MARouteDecision{Bucket: maBucketDaVerificare, SuppressedReason: decision.Reason}
	}
	return decision
}

func maRouteTargetBase(target MATarget, thesis string) MARouteDecision {
	if maManualGateDissent(target) {
		return MARouteDecision{Bucket: maBucketDaVerificare}
	}

	// Righe identity-only (score/matchState NULL): instradate dal verdetto gate.
	if target.MatchState == "" {
		switch gatedTargetBucket(target) {
		case maGatedBucketManualReview:
			return MARouteDecision{Bucket: maBucketAzionabile}
		case maGatedBucketReject:
			return suppressedMADecision(maGateRejectReason(target))
		default:
			return MARouteDecision{Bucket: maBucketDaVerificare}
		}
	}

	if target.MatchState == maMatchStateOutside {
		// Precedenza: regola dell'analista > morte legale > inattività > distress > settore.
		if reason := maOutsidePostFilterReason(target); reason != nil {
			return suppressedMADecision(*reason)
		}
		if reason, found := maReasonFromFlag(target, "ceased", "Cessata fiscalmente", "cessata_fiscalmente", maFlagKnockoutVitalita+"_"+maViabilityReasonCeased); found {
			return suppressedMADecision(reason)
		}
		if reason, found := maReasonFromFlag(target, "inactive", "Società inattiva", maFlagKnockoutVitalita+"_"+maViabilityReasonInactive); found {
			return suppressedMADecision(reason)
		}
		if reason, found := maReasonFromFlag(target, "distress", "Distress conclamato nei bilanci; opportunità solo per una tesi di consolidamento", maFlagKnockoutVitalita+"_"+maViabilityReasonDistress); found {
			if normalizeMAThesis(thesis) == maThesisConsolidation {
				return MARouteDecision{Bucket: maBucketAzionabile}
			}
			return suppressedMADecision(reason)
		}
		return suppressedMADecision(maOffSectorReason(target))
	}

	if maHasFlag(target, maFlagAtecoFuoriPerimetro) {
		return MARouteDecision{Bucket: maBucketAzionabile}
	}
	if target.Confidence == "bassa" {
		return MARouteDecision{Bucket: maBucketDaVerificare}
	}
	return MARouteDecision{Bucket: maBucketPrincipale}
}

func suppressedMADecision(reason MABucketReason) MARouteDecision {
	return MARouteDecision{Bucket: maBucketSoppresso, Reason: &reason}
}

func maOutsidePostFilterReason(target MATarget) *MABucketReason {
	for _, item := range target.Evidence {
		if item.Status != maEvidenceOutside {
			continue
		}
		var fallback string
		switch item.Criterion {
		case maPostFilterRevenuePerEmployeeMin:
			fallback = "Ricavo per dipendente sotto la soglia impostata nella ricerca"
		case maPostFilterMaxShareholders:
			fallback = "Numero di soci oltre il massimo impostato nella ricerca"
		default:
			continue
		}
		label := fallback
		if strings.TrimSpace(item.Value) != "" {
			label = fmt.Sprintf("%s: %s", fallback, strings.TrimSpace(item.Value))
		}
		return &MABucketReason{Code: "post_filter", Kind: "analyst_rule", Label: label}
	}
	return nil
}

func maReasonFromFlag(target MATarget, code, fallback string, prefixes ...string) (MABucketReason, bool) {
	for _, flag := range target.Flags {
		for _, prefix := range prefixes {
			if !strings.HasPrefix(flag.Code, prefix) {
				continue
			}
			label := strings.TrimSpace(flag.Label)
			if label == "" {
				label = fallback
			}
			return MABucketReason{Code: code, Kind: "system", Label: label}, true
		}
	}
	return MABucketReason{}, false
}

func maOffSectorReason(target MATarget) MABucketReason {
	ateco := strings.TrimSpace(strings.Join([]string{target.AtecoCode, target.AtecoDescription}, " — "))
	ateco = strings.Trim(ateco, " —")
	if ateco != "" {
		return MABucketReason{Code: "off_sector", Kind: "system", Label: "ATECO " + ateco + ", fuori dal perimetro della ricerca"}
	}
	return MABucketReason{Code: "off_sector", Kind: "system", Label: "Attività fuori dagli ambiti descritti: ATECO fuori perimetro, nessun riscontro web nel settore"}
}

func maGateRejectReason(target MATarget) MABucketReason {
	fallback := "Le pagine web lette indicano un’attività fuori tesi"
	if target.WebValidation == nil {
		return MABucketReason{Code: "gate_reject", Kind: "system", Label: fallback}
	}
	reason := strings.TrimSpace(target.WebValidation.FinalDecision.Reason)
	domain := strings.TrimSpace(target.WebValidation.SelectedDomain)
	if reason == "" {
		return MABucketReason{Code: "gate_reject", Kind: "system", Label: fallback}
	}
	if domain != "" {
		reason += " — giudicata su " + domain
	}
	return MABucketReason{Code: "gate_reject", Kind: "system", Label: reason}
}

func maManualGateDissent(target MATarget) bool {
	if target.Origin != maTargetOriginManual || target.WebValidation == nil {
		return false
	}
	return maGateDissentAction(target.WebValidation.FinalAction) ||
		maGateDissentAction(target.WebValidation.FinalDecision.FinalAction) ||
		maGateDissentState(target.WebValidation.WebValidationState) ||
		maGateDissentState(target.WebValidation.FinalDecision.WebValidationState)
}

func maGateDissentAction(action string) bool {
	switch strings.TrimSpace(action) {
	case "reject", "deprioritize":
		return true
	default:
		return false
	}
}

func maGateDissentState(state string) bool {
	switch strings.TrimSpace(state) {
	case "rejected", "deprioritized":
		return true
	default:
		return false
	}
}

func maHasFlag(target MATarget, code string) bool {
	for _, flag := range target.Flags {
		if flag.Code == code {
			return true
		}
	}
	return false
}

func maHasFlagPrefix(target MATarget, prefix string) bool {
	for _, flag := range target.Flags {
		if strings.HasPrefix(flag.Code, prefix) {
			return true
		}
	}
	return false
}

// buildMAScoringPlan costruisce il registro "valutato / filtrato / ignorato"
// della sessione: cosa lo score gradua (i segnali intended con i pesi nominali
// della tesi), cosa è stato applicato come filtro duro a monte (ogni
// sopravvissuto lo rispetta già, quindi lo score non lo ri-valuta) e cosa è
// stato lasciato cadere (vincoli free-form non supportati — MissingCriteria).
func buildMAScoringPlan(strategy MAStrategySpec) MAScoringPlan {
	plan := MAScoringPlan{
		Thesis:       normalizeMAThesis(strategy.Thesis),
		ScoreVersion: maScoreVersion,
		Evaluated:    []MAScoringPlanSignal{},
		Filtered:     []string{},
		Ignored:      append([]string{}, strategy.MissingCriteria...),
	}

	nominal := maSignalNominalWeights(strategy)
	for _, signal := range maSignalCatalog() {
		weight, ok := nominal[signal.ID]
		if !ok {
			continue
		}
		plan.Evaluated = append(plan.Evaluated, MAScoringPlanSignal{
			ID:     signal.ID,
			Label:  signal.Label,
			Family: signal.Family,
			Weight: weight,
		})
	}

	addFilter := func(text string) { plan.Filtered = append(plan.Filtered, text) }
	if len(strategy.Provinces) > 0 {
		addFilter("Province: " + strings.Join(strategy.Provinces, ", "))
	}
	if strategy.ActivityStatus != "" {
		addFilter("Stato attività: " + strategy.ActivityStatus)
	}
	switch {
	case strategy.TurnoverAround != nil && *strategy.TurnoverAround > 0:
		addFilter(fmt.Sprintf("Fatturato intorno a €%s", formatMAPlanAmount(*strategy.TurnoverAround)))
	case strategy.TurnoverMin != nil && strategy.TurnoverMax != nil:
		addFilter(fmt.Sprintf("Fatturato €%s – €%s", formatMAPlanAmount(*strategy.TurnoverMin), formatMAPlanAmount(*strategy.TurnoverMax)))
	case strategy.TurnoverMin != nil:
		// Un solo estremo taglia la superficie ma NON attiva il segnale di taglia
		// (turnover_proximity resta spento): dichiararlo qui è ciò che evita che
		// il vincolo sembri "valutato" quando è solo filtrato.
		addFilter(fmt.Sprintf("Fatturato minimo €%s (solo filtro, non graduato)", formatMAPlanAmount(*strategy.TurnoverMin)))
	case strategy.TurnoverMax != nil:
		addFilter(fmt.Sprintf("Fatturato massimo €%s (solo filtro, non graduato)", formatMAPlanAmount(*strategy.TurnoverMax)))
	}
	if strategy.EmployeeMin != nil || strategy.EmployeeMax != nil {
		addFilter("Dipendenti: " + formatMAPlanRange(strategy.EmployeeMin, strategy.EmployeeMax))
	}
	if len(strategy.LegalForms) > 0 {
		addFilter("Forme giuridiche: " + strings.Join(strategy.LegalForms, ", "))
	}
	if len(strategy.AtecoCandidates) > 0 {
		addFilter(fmt.Sprintf("Perimetro ATECO: %d codici curati", len(strategy.AtecoCandidates)))
	}
	if strategy.RevenuePerEmployeeMin != nil {
		addFilter(fmt.Sprintf("Ricavo/dipendente minimo €%s (knockout)", formatMAPlanAmount(*strategy.RevenuePerEmployeeMin)))
	}
	if strategy.MaxShareholders != nil {
		addFilter(fmt.Sprintf("Massimo %d soci (knockout)", *strategy.MaxShareholders))
	}

	return plan
}

func formatMAPlanAmount(v int) string {
	switch {
	case v >= 1_000_000 && v%100_000 == 0:
		return fmt.Sprintf("%.1fM", float64(v)/1_000_000)
	case v >= 1_000 && v%1_000 == 0:
		return fmt.Sprintf("%dk", v/1_000)
	default:
		return fmt.Sprintf("%d", v)
	}
}

func formatMAPlanRange(min, max *int) string {
	switch {
	case min != nil && max != nil:
		return fmt.Sprintf("%d – %d", *min, *max)
	case min != nil:
		return fmt.Sprintf("almeno %d", *min)
	case max != nil:
		return fmt.Sprintf("fino a %d", *max)
	default:
		return ""
	}
}
