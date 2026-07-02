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

// maRouteTarget assegna la destinazione di presentazione di un target.
//
//   - principale: scorato, confidence media/alta — la lista di lavoro.
//   - da_verificare: scorato ma a evidenza sottile (confidence bassa), oppure
//     sopravvissuto del gate mai arricchito. Un punteggio alto qui è un invito a
//     verificare, non un rank di qualità: è la coda di lavoro che impedisce al
//     fantasma a 3 segnali di travestirsi da 100 documentato.
//   - azionabile: un click lo rimette in gioco — ATECO fuori perimetro ma
//     confermato dal gate semantico (override/rivedi), dominio irrisolto
//     (associa dominio), distress sotto tesi consolidamento (il distress È
//     l'opportunità).
//   - soppresso: nessuna azione possibile — cessate/dormienti, escluse dalle
//     regole dure che l'analista stesso ha impostato, fuori settore senza
//     conferma semantica. Solo conteggio.
func maRouteTarget(target MATarget, thesis string) string {
	// Righe identity-only (score/matchState NULL): instradate dal verdetto gate.
	if target.MatchState == "" {
		switch gatedTargetBucket(target) {
		case maGatedBucketManualReview:
			return maBucketAzionabile // azione: associa dominio → processa
		case maGatedBucketReject:
			return maBucketSoppresso
		default:
			// keep/forse sopravvissuto ma mai arricchito (fetch fallito): visibile
			// tra i da-verificare, mai perso in silenzio.
			return maBucketDaVerificare
		}
	}

	if target.MatchState == maMatchStateOutside {
		// Precedenza: regola dell'analista > morte legale > distress > settore.
		if maHasOutsidePostFilter(target) {
			return maBucketSoppresso // esclusione corretta: l'ha chiesta l'analista
		}
		if maHasFlag(target, "cessata_fiscalmente") || maHasFlagPrefix(target, maFlagKnockoutVitalita+"_"+maViabilityReasonCeased) ||
			maHasFlagPrefix(target, maFlagKnockoutVitalita+"_"+maViabilityReasonInactive) {
			return maBucketSoppresso // entità morte/dormienti: nessuna azione
		}
		if maHasFlagPrefix(target, maFlagKnockoutVitalita+"_"+maViabilityReasonDistress) {
			if normalizeMAThesis(thesis) == maThesisConsolidation {
				return maBucketAzionabile // per un consolidatore il distress è il deal
			}
			return maBucketSoppresso
		}
		return maBucketSoppresso // fuori settore non riscattato dal gate, o legacy
	}

	if maHasFlag(target, maFlagAtecoFuoriPerimetro) {
		return maBucketAzionabile // probabile mis-codifica: rivedi/override
	}
	if target.Confidence == "bassa" {
		return maBucketDaVerificare
	}
	return maBucketPrincipale
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

// maHasOutsidePostFilter reports whether the target was gated out by one of the
// analyst's own hard rules (post-filter evidence rows with status outside).
func maHasOutsidePostFilter(target MATarget) bool {
	for _, item := range target.Evidence {
		if item.Status != maEvidenceOutside {
			continue
		}
		if item.Criterion == maPostFilterRevenuePerEmployeeMin || item.Criterion == maPostFilterMaxShareholders {
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
