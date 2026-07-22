package binocolo

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// Adjusted valuation, computed ON READ (issue #78, Fase 7). The baseline deep-dive analysis
// (ma_deep_analysis) is the vendor-derived, GLOBAL artifact and is NEVER mutated here — the
// analyst's nota-integrativa decisions are folded onto it only at read time, per company, and
// nothing is cached. The adjusted view is a bag of CONTESTABLE FACTS (numbers + provenance);
// the frontend composes the sentences.
//
// The effective set is deliberately narrow: only the effective decisions of the ACTIVE reading
// run of the ACTIVE processing run of the CANONICAL filing (the filing whose fiscal identity is
// validated|override and whose closing_date equals the current vendor baseline exercise). If more
// than one validated filing exists for that exercise, they are the SAME deposit acquired through
// two channels (analyst upload + camerale acquisition) when their key CEE figures are identical —
// then one is elected canonical (vendor-anchored, then most recent) with no ambiguity; a genuine
// re-deposit (figures diverge, or an extract is missing) leaves the canonical UNDEFINED ⇒ zero
// effect + declared ambiguity.
//
// Formula (conservative, screening): EBITDA_adj = EBITDA_baseline + Σ signed ΔEBITDA; the SAME Δ
// enters BOTH band extremes (reported×haircut high, prudential low). PFN_adj = PFN + Σ signed
// ΔPFN (ΔPFN>0 = more debt ⇒ equity down; a ring-fenced cash-like item ⇒ ΔPFN<0 ⇒ equity up).
// TFR and the tax fund are NEVER touched. The valuation METHOD is inherited verbatim from the
// baseline — the EV/EBITDA-vs-EV/Sales selector is never re-run on the adjusted scorecard.

const (
	maAdjustedStatusActive     = "active"      // canonical filing + at least one aligned effect
	maAdjustedStatusNotAligned = "not_aligned" // filings exist but none is validated+aligned
	maAdjustedStatusAmbiguous  = "ambiguous"   // >1 validated filing for the baseline exercise, not a same-deposit pair (real re-deposit)
	maAdjustedStatusNoFiling   = "no_filing"   // no filing at all for the fiscal identity
	maAdjustedStatusNoEffects  = "no_effects"  // canonical filing but no aligned effective adjustment
)

const (
	// maAdjustedReconTolerancePct is the relative divergence (in %) above which a doc-vs-vendor
	// reconciliation field is flagged divergent, mirroring the vendor-vs-CEE tolerance convention
	// (buildMADeepReconciliation): |filing − vendor| / max(|vendor|, floor) × 100 > tolerance.
	maAdjustedReconTolerancePct = 1.0
	// maAdjustedReconFloorEUR floors the denominator so a near-zero vendor figure cannot make a
	// tiny absolute gap read as a huge relative divergence (same 1000€ floor as the PFN reconc.).
	maAdjustedReconFloorEUR = 1000.0
)

// MAAdjustedView is the on-read adjusted valuation of one company: the baseline exercise, the
// canonical filing (when resolved), the effective adjustments, the derived adjusted valuation,
// the doc-vs-vendor reconciliation, and the count of open DD points. Facts only — no copy.
type MAAdjustedView struct {
	BaselineExercise string `json:"baselineExercise,omitempty"` // YYYY-MM-DD (vendor baseline closing date)
	FilingID         string `json:"filingId,omitempty"`
	Status           string `json:"status"`
	AmbiguousCount   int    `json:"ambiguousCount,omitempty"`
	// AdjustedValuation is present only for status=="active": the baseline valuation re-run with
	// EBITDA_adj/PFN_adj under the inherited method. Absent when nothing is effective.
	AdjustedValuation            *MADeepValuation          `json:"adjustedValuation,omitempty"`
	Effects                      []MAAdjustedEffect        `json:"effects"`
	DeltaEbitda                  float64                   `json:"deltaEbitda"`
	DeltaPfn                     float64                   `json:"deltaPfn"`
	EVSalesDeltaEbitdaNotApplied bool                      `json:"evSalesDeltaEbitdaNotApplied"`
	Reconciliation               *MAAdjustedReconciliation `json:"reconciliation,omitempty"`
	// PendingCount is the number of open DD points: proposals of the active run aligned to the
	// baseline exercise that are NOT effective (dd_only or awaiting/without a ratify decision).
	PendingCount int `json:"pendingCount"`
}

// MAAdjustedEffect is one effective nota-integrativa adjustment applied to the baseline: the
// analyst ratified it (directly or by promotion) with a signed amount, aligned to the baseline
// exercise. Carries its provenance (page) so the frontend can cite it.
type MAAdjustedEffect struct {
	ProposalID   string  `json:"proposalId"`
	Label        string  `json:"label,omitempty"`
	Treatment    string  `json:"treatment"` // ebitda | pfn
	Amount       float64 `json:"amount"`    // signed
	ExerciseDate string  `json:"exerciseDate,omitempty"`
	PageNo       *int    `json:"pageNo,omitempty"`
}

// MAAdjustedReconciliation is the numeric cross-check between the canonical filing's parsed CEE
// extract and the vendor CEE view for the SAME exercise. It is a structured flag only — the
// filing never overrides the vendor baseline (issue #78).
type MAAdjustedReconciliation struct {
	Exercise  string                 `json:"exercise,omitempty"`
	Fields    []MAAdjustedReconField `json:"fields"`
	Divergent bool                   `json:"divergent"`
}

// MAAdjustedReconField is one comparable CEE figure on both sides plus their signed delta.
type MAAdjustedReconField struct {
	Name   string   `json:"name"`
	Filing *float64 `json:"filing,omitempty"`
	Vendor *float64 `json:"vendor,omitempty"`
	Delta  *float64 `json:"delta,omitempty"` // filing − vendor
}

// sameMADay reports whether two dates fall on the same calendar day (both are stored as
// timezone-neutral ::date values, so a UTC Y/M/D comparison is exact).
func sameMADay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// maBaselineExerciseDate resolves the baseline exercise date for the ADJUSTED-VIEW path from the
// vendor payload's ecofin.balanceSheetDate, correcting a vendor timezone-serialization quirk. The
// vendor renders the LOCAL Europe/Rome midnight of the closing date WITHOUT a zone suffix, so a
// 31-Dec close appears as "2025-12-30T23:00:00" (UTC+1 winter) and a 30-Jun close as
// "2025-06-30T22:00:00" (UTC+2 summer) — a bare instant that reads as the DAY BEFORE the real
// closing date. We parse the full value (RFC3339, then no-zone datetime, then bare date) and, when
// the clock component is ≥ 12:00 — a late-evening instant that is really the next local midnight —
// round UP to the following day; otherwise we truncate to the day. Returns ok=false when the field
// is absent or unparseable. The result is a UTC midnight so it compares exactly (sameMADay) with a
// filing's ::date closing_date and with an effective adjustment's ::date exercise date.
//
// NOTE — deliberately DIFFERENT from deepVintageKey (ma_deep_engine.go): that function takes the
// literal raw[:10] prefix and its output is PERSISTED as-is in the DB (vendor vintage keying /
// historical continuity), so it must NOT be changed. This function is read-only and used ONLY by
// the adjusted view / brief-context path, where the exercise must line up with a deposited filing's
// closing_date — hence the off-by-one correction lives here alone.
func maBaselineExerciseDate(payload json.RawMessage) (time.Time, bool) {
	object, err := decodeVendorObject(payload)
	if err != nil || object == nil {
		return time.Time{}, false
	}
	root := deepFullRoot(object)
	raw := firstVendorString(root, "ecofin.balanceSheetDate")
	if raw == "" {
		return time.Time{}, false
	}
	var parsed time.Time
	ok := false
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			parsed = t
			ok = true
			break
		}
	}
	if !ok {
		return time.Time{}, false
	}
	y, m, d := parsed.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	if parsed.Hour() >= 12 {
		day = day.AddDate(0, 0, 1) // late-evening instant = next local midnight = the true closing date
	}
	return day, true
}

// selectMACanonicalMatches returns the aligned validated filings for an exercise (pure): those
// whose identity is validated|override AND whose closing_date matches baselineExercise. Zero ⇒ no
// canonical; one ⇒ the unique canonical; more than one ⇒ candidates for the duplicate resolution
// (decideMADuplicateCanonical) — the blob bytes differ across channels so a merge never collapsed
// them, and only comparing the parsed figures can tell a same-deposit pair from a real re-deposit.
func selectMACanonicalMatches(filings []maFiling, baselineExercise time.Time) []*maFiling {
	var matches []*maFiling
	for i := range filings {
		f := &filings[i]
		if f.IdentityStatus != maFilingIdentityValidated && f.IdentityStatus != maFilingIdentityOverride {
			continue
		}
		if f.ClosingDate == nil || !sameMADay(*f.ClosingDate, baselineExercise) {
			continue
		}
		matches = append(matches, f)
	}
	return matches
}

// maFilingDigest is the set of key CEE figures used to decide whether two aligned validated filings
// are the SAME deposit acquired through two channels (analyst upload + camerale acquisition) rather
// than a genuine re-deposit. Read straight from the parsed exercise extract (no arithmetic here),
// so equality is EXACT (zero tolerance). present=false means the exercise extract was missing;
// readable means it was present AND yielded at least one comparable figure.
type maFilingDigest struct {
	totaleAttivo  *float64
	totalePassivo *float64
	differenzaAB  *float64 // conto economico A−B as declared in the prospetto
	utile         *float64 // utile d'esercizio — compared only when present on both sides
	present       bool
}

func (d maFilingDigest) readable() bool {
	return d.present && (d.totaleAttivo != nil || d.totalePassivo != nil || d.differenzaAB != nil || d.utile != nil)
}

// sameMAFloatPtr reports exact equality of two optional figures: both absent, or both present with
// equal value. A presence mismatch (one nil, one set) is a divergence.
func sameMAFloatPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameMAFilingDigest(a, b maFilingDigest) bool {
	return sameMAFloatPtr(a.totaleAttivo, b.totaleAttivo) &&
		sameMAFloatPtr(a.totalePassivo, b.totalePassivo) &&
		sameMAFloatPtr(a.differenzaAB, b.differenzaAB) &&
		sameMAFloatPtr(a.utile, b.utile)
}

// decideMADuplicateCanonical is the pure "vendor merge" of the plan for >1 aligned validated
// candidates, given each candidate's already-loaded digest (aligned by index; the loading happens
// in the caller so this stays testable). The upload and the camerale acquisition of the SAME
// deposit produce different blob bytes — the DocuEngine PDF carries a per-request cover header ⇒
// distinct md5 ⇒ never blob-merged — so they legitimately co-exist as two filings for one exercise.
// When every candidate's key CEE figures are byte-for-byte identical it is ONE deposit ⇒ canonical
// = the vendor-anchored filing (balance_sheet_id set), tie-broken by the most-recent created_at,
// with NO ambiguity. Any divergence — different figures, or a missing/unreadable extract on any
// candidate — is a genuine re-deposit and stays conservatively ambiguous (resolved=false ⇒ zero
// effect). Returns (canonical, resolved).
func decideMADuplicateCanonical(candidates []*maFiling, digests []maFilingDigest) (*maFiling, bool) {
	if len(candidates) < 2 || len(candidates) != len(digests) {
		return nil, false
	}
	for _, d := range digests {
		if !d.readable() {
			return nil, false // missing or unreadable extract ⇒ cannot confirm same deposit
		}
	}
	for i := 1; i < len(digests); i++ {
		if !sameMAFilingDigest(digests[0], digests[i]) {
			return nil, false // figures diverge ⇒ real re-deposit ⇒ ambiguous
		}
	}
	var canonical *maFiling
	for _, c := range candidates {
		if canonical == nil || betterMACanonicalDuplicate(c, canonical) {
			canonical = c
		}
	}
	return canonical, true
}

// betterMACanonicalDuplicate ranks two same-deposit candidates: the vendor-anchored filing
// (balance_sheet_id set) wins over an upload; on a tie the most-recent created_at wins.
func betterMACanonicalDuplicate(a, b *maFiling) bool {
	aVendor := a.BalanceSheetID != ""
	bVendor := b.BalanceSheetID != ""
	if aVendor != bVendor {
		return aVendor
	}
	return a.CreatedAt.After(b.CreatedAt)
}

// resolveMACanonicalFilingDetail is the shared list+select path: it lists the fiscal identity's
// filings and resolves the canonical one, also returning the total filing count so the caller can
// tell "no filing at all" from "filings present but none aligned". Zero aligned ⇒ (nil,0); exactly
// one ⇒ that filing; more than one ⇒ the duplicate resolution — the same deposit acquired through
// two channels (upload + camerale) collapses to one canonical filing, while a real re-deposit stays
// ambiguous. Only the >1 branch reads the extracts (the common single-filing path stays one SELECT).
func (s *maService) resolveMACanonicalFilingDetail(ctx context.Context, vat, tax string, baselineExercise time.Time) (canonical *maFiling, ambiguous, total int, err error) {
	fiscalKey := buildMAFiscalKey(vat, tax)
	if fiscalKey == "" || s.filing == nil {
		return nil, 0, 0, nil
	}
	filings, err := s.filing.ListMAFilingsByFiscalKey(ctx, fiscalKey)
	if err != nil {
		return nil, 0, 0, err
	}
	matches := selectMACanonicalMatches(filings, baselineExercise)
	switch len(matches) {
	case 0:
		return nil, 0, len(filings), nil
	case 1:
		return matches[0], 0, len(filings), nil
	default:
		// >1 aligned validated filings: load each candidate's key CEE figures (from its ACTIVE
		// processing run) and let the pure decision tell a same-deposit pair from a real re-deposit.
		digests := make([]maFilingDigest, len(matches))
		for i, m := range matches {
			digests[i] = s.maFilingDigestForExercise(ctx, m, baselineExercise)
		}
		if resolved, ok := decideMADuplicateCanonical(matches, digests); ok {
			return resolved, 0, len(filings), nil
		}
		return nil, len(matches), len(filings), nil
	}
}

// maFilingDigestForExercise loads one candidate's parsed key CEE figures for an exercise (from its
// active processing run) into a maFilingDigest. present=false when the exercise extract is absent
// or unreadable; the figures are read straight from the parse (no arithmetic), so a later exact
// comparison across candidates is a byte-for-byte deposit-identity check.
func (s *maService) maFilingDigestForExercise(ctx context.Context, canonical *maFiling, exercise time.Time) maFilingDigest {
	sp, ce, found := s.maFilingExtractForExercise(ctx, canonical, exercise)
	if !found {
		return maFilingDigest{}
	}
	return maFilingDigest{
		totaleAttivo:  sp.TotaleAttivo,
		totalePassivo: sp.TotalePassivo,
		differenzaAB:  ce.DifferenzaAB,
		utile:         ce.UtileEsercizio,
		present:       true,
	}
}

// maAdjustedLowUsesPrudential reconstructs whether the BASELINE ev_ebitda band derived its low
// extreme from the prudential EBITDA (vs a symmetric reported band). It replays the engine's own
// low-branch predicate on BASELINE inputs — it does NOT re-select on the adjusted scorecard. Only
// meaningful when Method=="ev_ebitda" && LowMethod=="" (the ev_sales fallback was not taken).
func maAdjustedLowUsesPrudential(baselineVal *MADeepValuation, sc *MADeepScorecard, pricing maPricing) bool {
	if sc.Ebitda == nil {
		return false
	}
	prud := *sc.Ebitda
	if baselineVal.PrudentialEbitda != nil {
		prud = *baselineVal.PrudentialEbitda
	}
	if prud <= 0 {
		return false
	}
	if sc.Turnover == nil || *sc.Turnover <= 0 {
		return true // senza fatturato la soglia di margine non può bocciare (Inf ≥ threshold)
	}
	return prud/(*sc.Turnover)*100 >= pricing.EBITDAFallbackPct
}

// buildMAAdjustedValuation folds the effective adjustments onto the baseline valuation, ON READ.
// The METHOD is inherited verbatim from baselineVal (never re-selected): Method/LowMethod/
// Multiple/HaircutPct/Sector/Source. It re-runs the SAME EV/bridge math as the engine but with
// EBITDA_adj/PFN_adj. An ev_sales extreme keeps its baseline EV (ΔEBITDA is NOT applied to it);
// ΔPFN always moves equity through the bridge. TFR and the tax fund are inherited unchanged.
// Returns (nil, false) when the baseline lacks the valuation/scorecard needed to adjust.
// The second result is evSalesDeltaEbitdaNotApplied: a nonzero ΔEBITDA was withheld from an
// ev_sales extreme.
func buildMAAdjustedValuation(baselineVal *MADeepValuation, sc *MADeepScorecard, adjustments []maNIEffectiveAdjustment, pricing maPricing) (*MADeepValuation, bool) {
	if baselineVal == nil || sc == nil {
		return nil, false
	}
	var sumEbitda, sumPfn float64
	for _, a := range adjustments {
		switch a.Treatment {
		case maNITrattamentoEbitda:
			sumEbitda += a.Amount
		case maNITrattamentoPFN:
			sumPfn += a.Amount
		}
	}

	haircut := baselineVal.HaircutPct / 100.0
	if haircut < 0 {
		haircut = 0
	}
	if haircut > 0.95 {
		haircut = 0.95
	}
	factor := 1 - haircut
	const spread = 0.15

	adj := *baselineVal // inherit scalars (method, multiple, sector, band, caveat) verbatim
	adj.Bridge = nil
	adj.PFN = nil
	adj.EquityLow = nil
	adj.EquityHigh = nil

	evSalesExtreme := baselineVal.Method == "ev_sales" || baselineVal.LowMethod == "ev_sales"
	evSalesDeltaNotApplied := evSalesExtreme && sumEbitda != 0

	// PrudentialEbitda is informational; adjust it by the same Δ when present.
	if baselineVal.PrudentialEbitda != nil {
		p := *baselineVal.PrudentialEbitda + sumEbitda
		adj.PrudentialEbitda = &p
	}

	// EV band: recompute only the EBITDA-driven extremes, only when there is an add-back.
	//
	// (a) The recompute uses baselineVal.Multiple, which the engine stored ROUNDED to 2 decimals.
	// That is the best reconstruction available on read (the un-rounded engine multiple is not
	// persisted) and it is the very multiple shown to the analyst, so it is internally consistent;
	// the drift vs a hypothetical un-rounded run is ≤~0.05% and only with a fractional multiple.
	if baselineVal.Method == "ev_ebitda" && sumEbitda != 0 {
		if sc.Ebitda == nil {
			return nil, false // baseline says ev_ebitda but the scorecard has no EBITDA: inconsistent
		}
		evHighBase := (*sc.Ebitda + sumEbitda) * baselineVal.Multiple * factor
		adj.EVHigh = math.Round(evHighBase * (1 + spread))
		switch baselineVal.LowMethod {
		case "ev_sales":
			// low extreme is sales-based ⇒ ΔEBITDA not applied; keep the baseline EVLow.
		default:
			if maAdjustedLowUsesPrudential(baselineVal, sc, pricing) {
				prudBase := *sc.Ebitda
				if baselineVal.PrudentialEbitda != nil {
					prudBase = *baselineVal.PrudentialEbitda
				}
				evLowBase := (prudBase + sumEbitda) * baselineVal.Multiple * factor
				adj.EVLow = math.Round(evLowBase * (1 - spread))
			} else {
				adj.EVLow = math.Round(evHighBase * (1 - spread)) // reported-symmetric
			}
		}
	}
	// Method=="ev_sales": ΔEBITDA never moves EV (both extremes stay baseline).

	// Bridge EV→equity: PFN_adj deducted; TFR/tax fund inherited verbatim; shareholder loans
	// informational (already inside the PFN). Rebuilt whenever a baseline bridge + PFN exist.
	//
	// (b) The deductions use the inherited bridge rows, whose TFR/TaxFund values the engine already
	// ROUNDED, whereas the engine's own deductions summed the raw TaxFund. The divergence vs a fresh
	// engine run is 0–1€ and only with a fractional TaxFund — never on CEE figures, which are whole
	// euros. Using the displayed rows keeps the adjusted bridge internally consistent.
	if baselineVal.Bridge != nil && sc.PFN != nil {
		pfnAdj := *sc.PFN + sumPfn
		bridge := &MADeepBridge{}
		prov := maCEEProvVendorRatio
		if baselineVal.Bridge.PFN != nil {
			prov = baselineVal.Bridge.PFN.Provenance
		}
		bridge.PFN = &MADeepBridgeRow{Value: math.Round(pfnAdj), Provenance: prov}
		deductions := pfnAdj
		if baselineVal.Bridge.TFR != nil {
			row := *baselineVal.Bridge.TFR
			bridge.TFR = &row
			deductions += row.Value
		}
		if baselineVal.Bridge.TaxFund != nil {
			row := *baselineVal.Bridge.TaxFund
			bridge.TaxFund = &row
			deductions += row.Value
		}
		if baselineVal.Bridge.ShareholderLoans != nil {
			row := *baselineVal.Bridge.ShareholderLoans
			bridge.ShareholderLoans = &row // informational, never deducted
		}
		low := math.Round(adj.EVLow - deductions)
		high := math.Round(adj.EVHigh - deductions)
		bridge.EquityLow = &low
		bridge.EquityHigh = &high
		adj.Bridge = bridge
		adj.PFN = &pfnAdj
		adj.EquityLow = &low
		adj.EquityHigh = &high
	}
	return &adj, evSalesDeltaNotApplied
}

// maAdjustedInputs is the resolved on-read state shared by the adjusted view and the brief
// filing context: the baseline exercise, the canonical filing (when unique), the active run's
// proposals+decisions, and the effective adjustments aligned to the baseline exercise.
type maAdjustedInputs struct {
	baselineExercise time.Time
	hasExercise      bool
	status           string
	ambiguousCount   int
	canonical        *maFiling
	proposals        []maNIProposalWithDecisions
	aligned          []maNIEffectiveAdjustment
}

// resolveMAAdjustedInputs is the single on-read resolution path (no writes, only SELECTs): read
// the baseline exercise from the deep record's vendor payload, resolve the canonical filing, and
// (when unique) reduce the active reading run's decisions to the effective adjustments aligned to
// that exercise. Status is set to the terminal states that need no further work (no_filing,
// not_aligned, ambiguous); "active"/"no_effects" is decided by the caller from len(aligned).
func (s *maService) resolveMAAdjustedInputs(ctx context.Context, identity maDeepDiveIdentity, deep *maDeepVATRecord) (maAdjustedInputs, error) {
	in := maAdjustedInputs{}
	if deep == nil {
		in.status = maAdjustedStatusNoFiling
		return in, nil
	}
	// Baseline exercise = the vendor baseline's balance-sheet closing date (ecofin.balanceSheetDate
	// in the IT-full payload). This is the exercise the current vendor scorecard/valuation describes,
	// so it is the exercise a deposited filing must match — hence maBaselineExerciseDate (NOT
	// deepVintageKey) is used here: it corrects the vendor's off-by-one TZ serialization so a
	// 31-Dec close does not read as 30-Dec and wrongly flag an aligned filing as not_aligned.
	if d, ok := maBaselineExerciseDate(deep.Payload); ok {
		in.baselineExercise = d
		in.hasExercise = true
	}
	fiscalKey := buildMAFiscalKey(identity.VATCode, identity.TaxCode)
	if !in.hasExercise || fiscalKey == "" || s.filing == nil {
		in.status = maAdjustedStatusNoFiling
		return in, nil
	}
	canonical, ambiguous, total, err := s.resolveMACanonicalFilingDetail(ctx, identity.VATCode, identity.TaxCode, in.baselineExercise)
	if err != nil {
		return in, err
	}
	switch {
	case ambiguous > 0:
		in.status = maAdjustedStatusAmbiguous
		in.ambiguousCount = ambiguous
		return in, nil
	case canonical == nil:
		if total == 0 {
			in.status = maAdjustedStatusNoFiling
		} else {
			in.status = maAdjustedStatusNotAligned
		}
		return in, nil
	}
	in.canonical = canonical

	proposals, err := s.filing.ListMANIProposalsWithDecisions(ctx, canonical.ID)
	if err != nil {
		return in, err
	}
	in.proposals = proposals
	// Reduce over the ACTIVE run's proposals+decisions ONLY (no double count), then keep only the
	// adjustments aligned to the baseline exercise (a Δ applies solely to its own exercise).
	flatProps := make([]maNIProposal, 0, len(proposals))
	var flatDecs []maNIDecision
	for _, p := range proposals {
		flatProps = append(flatProps, p.Proposal)
		flatDecs = append(flatDecs, p.Decisions...)
	}
	for _, eff := range reduceMANIEffectiveAdjustments(flatProps, flatDecs) {
		if eff.ExerciseDate != nil && sameMADay(*eff.ExerciseDate, in.baselineExercise) {
			in.aligned = append(in.aligned, eff)
		}
	}
	return in, nil
}

// computeMAAdjustedView is the F8/F9 entry point: the on-read adjusted valuation for one company.
// It NEVER writes (only SELECTs) and never re-selects the valuation method. The deep record is
// the vendor-derived baseline (its scorecard/valuation/payload); adjustments come from the
// canonical filing's ratified nota-integrativa decisions.
func (s *maService) computeMAAdjustedView(ctx context.Context, identity maDeepDiveIdentity, deep *maDeepVATRecord) (*MAAdjustedView, error) {
	in, err := s.resolveMAAdjustedInputs(ctx, identity, deep)
	if err != nil {
		return nil, err
	}
	view := &MAAdjustedView{Status: in.status, Effects: []MAAdjustedEffect{}, AmbiguousCount: in.ambiguousCount}
	if in.hasExercise {
		view.BaselineExercise = maDateValue(in.baselineExercise)
	}
	if in.canonical == nil {
		return view, nil // no_filing | not_aligned | ambiguous: zero effect, no reconciliation
	}
	view.FilingID = in.canonical.ID

	// Provenance index for effects (label/page come from the proposal, not the decision).
	propByID := make(map[string]maNIProposal, len(in.proposals))
	effectiveIDs := make(map[string]bool, len(in.aligned))
	for _, p := range in.proposals {
		propByID[p.Proposal.ID] = p.Proposal
	}
	for _, eff := range in.aligned {
		effectiveIDs[eff.ProposalID] = true
		e := MAAdjustedEffect{ProposalID: eff.ProposalID, Treatment: eff.Treatment, Amount: eff.Amount}
		if eff.ExerciseDate != nil {
			e.ExerciseDate = maDateValue(*eff.ExerciseDate)
		}
		if p, ok := propByID[eff.ProposalID]; ok {
			e.Label = p.Label
			e.PageNo = p.PageNo
		}
		view.Effects = append(view.Effects, e)
		switch eff.Treatment {
		case maNITrattamentoEbitda:
			view.DeltaEbitda += eff.Amount
		case maNITrattamentoPFN:
			view.DeltaPfn += eff.Amount
		}
	}

	// PendingCount: proposals aligned to the baseline exercise that are NOT effective — the open
	// DD points (dd_only, or awaiting/without a ratify decision).
	for _, p := range in.proposals {
		if p.Proposal.ExerciseDate == nil || !sameMADay(*p.Proposal.ExerciseDate, in.baselineExercise) {
			continue
		}
		if !effectiveIDs[p.Proposal.ID] {
			view.PendingCount++
		}
	}

	// Reconciliation is available whenever a canonical filing exists (independent of effects).
	sp, ce, found := s.maFilingExtractForExercise(ctx, in.canonical, in.baselineExercise)
	view.Reconciliation = buildMAAdjustedReconciliation(sp, ce, found, in.baselineExercise, deep)

	if len(in.aligned) == 0 {
		view.Status = maAdjustedStatusNoEffects
		return view, nil
	}
	view.Status = maAdjustedStatusActive
	if deep.Valuation != nil && deep.Scorecard != nil {
		pricing := s.loadPricing(ctx)
		adjusted, flag := buildMAAdjustedValuation(deep.Valuation, deep.Scorecard, in.aligned, pricing)
		view.AdjustedValuation = adjusted
		view.EVSalesDeltaEbitdaNotApplied = flag
	}
	return view, nil
}

// maFilingExtractForExercise fetches the canonical filing's parsed CEE extract for one exercise
// and unmarshals its SP/CE (best-effort: found=false on absent/error). It is the SINGLE read of
// the extract, so callers that need both the reconciliation and the summary fetch only once.
func (s *maService) maFilingExtractForExercise(ctx context.Context, canonical *maFiling, exercise time.Time) (sp maFilingSP, ce maFilingCE, found bool) {
	if canonical == nil || canonical.ActiveProcessingRunID == "" || s.filing == nil {
		return
	}
	extracts, err := s.filing.GetMAFilingExtracts(ctx, canonical.ActiveProcessingRunID)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo adjusted extract fetch failed",
			"component", "binocolo", "operation", "ma_adjusted_extract", "filing_id", canonical.ID, "error", err)
		return
	}
	for i := range extracts {
		if !sameMADay(extracts[i].ExerciseDate, exercise) {
			continue
		}
		found = true
		if len(extracts[i].SP) > 0 {
			_ = json.Unmarshal(extracts[i].SP, &sp)
		}
		if len(extracts[i].CE) > 0 {
			_ = json.Unmarshal(extracts[i].CE, &ce)
		}
		break
	}
	return
}

// buildMAAdjustedReconciliation compares the canonical filing's parsed CEE extract (already read
// via maFilingExtractForExercise) against the vendor CEE view for the same exercise. Robust,
// always-comparable fields: totale attivo (074), valore della produzione (130), patrimonio netto
// (084). Returns nil when the extract is absent or neither side yields a comparable figure. Flag
// only — never an override.
func buildMAAdjustedReconciliation(sp maFilingSP, ce maFilingCE, found bool, exercise time.Time, deep *maDeepVATRecord) *MAAdjustedReconciliation {
	if !found {
		return nil
	}
	vendor := maCEEReadingFromPayload(deep.Payload)
	var vTotalAssets, vProduction, vNetWorth *float64
	if vendor != nil {
		vTotalAssets = vendor.TotalAssets
		vProduction = vendor.ProductionValue
		vNetWorth = vendor.NetWorth
	}

	rec := &MAAdjustedReconciliation{Exercise: maDateValue(exercise), Fields: []MAAdjustedReconField{}}
	addField := func(name string, filing, vendorVal *float64) {
		field := MAAdjustedReconField{Name: name, Filing: filing, Vendor: vendorVal}
		if filing != nil && vendorVal != nil {
			delta := *filing - *vendorVal
			field.Delta = &delta
			denom := math.Max(math.Abs(*vendorVal), maAdjustedReconFloorEUR)
			if math.Abs(delta)/denom*100 > maAdjustedReconTolerancePct {
				rec.Divergent = true
			}
		}
		rec.Fields = append(rec.Fields, field)
	}
	addField("totale_attivo", sp.TotaleAttivo, vTotalAssets)
	addField("valore_produzione", ce.ValoreProduzione, vProduction)
	addField("patrimonio_netto", sp.TotalePatrimonioNetto, vNetWorth)

	// Nothing comparable at all ⇒ no reconciliation.
	any := false
	for _, f := range rec.Fields {
		if f.Filing != nil || f.Vendor != nil {
			any = true
			break
		}
	}
	if !any {
		return nil
	}
	return rec
}

// ---------------------------------------------------------------------------
// Brief filing context (F7 → F5 brief enrichment). Built from the SAME on-read path; passed to
// buildMADeepBriefLLM only when a canonical filing exists (else the 097-style prompt is unchanged).
// ---------------------------------------------------------------------------

// maDeepBriefFilingContext is the OPTIONAL filing block added to the brief LLM input under the
// "filings" key. Ratified facts carry provenance + the verbatim quote (grounding); pending points
// are DD seeds (never numbers in the computation); the CEE extract and the adjusted summary give
// the model the deposited-filing reading and its effect on the band.
type maDeepBriefFilingContext struct {
	Exercise string                      `json:"exercise,omitempty"`
	Ratified []maDeepBriefRatifiedFact   `json:"ratified,omitempty"`
	Pending  []maDeepBriefPendingPoint   `json:"pending,omitempty"`
	CEE      *maDeepBriefCEEExtract      `json:"cee,omitempty"`
	Adjusted *maDeepBriefAdjustedSummary `json:"adjusted,omitempty"`
}

type maDeepBriefRatifiedFact struct {
	Fact      string  `json:"fatto"`
	Treatment string  `json:"treatment"`
	Amount    float64 `json:"amount"`
	Page      *int    `json:"page,omitempty"`
	Quote     string  `json:"quote,omitempty"`
}

type maDeepBriefPendingPoint struct {
	Fact string `json:"fatto"`
	Page *int   `json:"page,omitempty"`
}

type maDeepBriefCEEExtract struct {
	TotaleAttivo     *float64 `json:"totaleAttivo,omitempty"`
	ValoreProduzione *float64 `json:"valoreProduzione,omitempty"`
	PatrimonioNetto  *float64 `json:"patrimonioNetto,omitempty"`
	UtileEsercizio   *float64 `json:"utileEsercizio,omitempty"`
	ReconDivergent   bool     `json:"recDivergent,omitempty"`
}

type maDeepBriefAdjustedSummary struct {
	DeltaEbitda float64  `json:"deltaEbitda"`
	DeltaPfn    float64  `json:"deltaPfn"`
	EquityLow   *float64 `json:"equityLow,omitempty"`
	EquityHigh  *float64 `json:"equityHigh,omitempty"`
}

// buildMADeepBriefFilingContext builds the optional filing context via the on-read adjusted path.
// Returns nil (no error surface — best-effort by the caller) when no canonical filing/context
// exists, so the base brief input stays 097-compatible. No writes; only SELECTs.
func (s *maService) buildMADeepBriefFilingContext(ctx context.Context, identity maDeepDiveIdentity, deep *maDeepVATRecord) *maDeepBriefFilingContext {
	in, err := s.resolveMAAdjustedInputs(ctx, identity, deep)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo brief filing context failed",
			"component", "binocolo", "operation", "ma_deep_brief_filing_context", "error", err)
		return nil
	}
	if in.canonical == nil {
		return nil
	}
	fctx := &maDeepBriefFilingContext{Exercise: maDateValue(in.baselineExercise)}

	effectiveIDs := make(map[string]bool, len(in.aligned))
	propByID := make(map[string]maNIProposal, len(in.proposals))
	for _, p := range in.proposals {
		propByID[p.Proposal.ID] = p.Proposal
	}
	for _, eff := range in.aligned {
		effectiveIDs[eff.ProposalID] = true
		fact := maDeepBriefRatifiedFact{Treatment: eff.Treatment, Amount: eff.Amount}
		if p, ok := propByID[eff.ProposalID]; ok {
			fact.Fact = p.FattoOsservato
			fact.Page = p.PageNo
			fact.Quote = p.Quote
		}
		fctx.Ratified = append(fctx.Ratified, fact)
	}
	// Pending DD points: aligned proposals that are not effective (dd_only / no ratify).
	for _, p := range in.proposals {
		if p.Proposal.ExerciseDate == nil || !sameMADay(*p.Proposal.ExerciseDate, in.baselineExercise) {
			continue
		}
		if effectiveIDs[p.Proposal.ID] {
			continue
		}
		fctx.Pending = append(fctx.Pending, maDeepBriefPendingPoint{Fact: p.Proposal.FattoOsservato, Page: p.Proposal.PageNo})
	}

	// CEE extract summary + reconciliation flag: read the filing extract ONCE and feed both.
	sp, ce, found := s.maFilingExtractForExercise(ctx, in.canonical, in.baselineExercise)
	if extract := maCanonicalExtractSummary(sp, ce, found); extract != nil {
		if rec := buildMAAdjustedReconciliation(sp, ce, found, in.baselineExercise, deep); rec != nil {
			extract.ReconDivergent = rec.Divergent
		}
		fctx.CEE = extract
	}

	// Adjusted summary (only when there is an effect and a baseline valuation to adjust).
	if len(in.aligned) > 0 && deep.Valuation != nil && deep.Scorecard != nil {
		var deltaEbitda, deltaPfn float64
		for _, eff := range in.aligned {
			switch eff.Treatment {
			case maNITrattamentoEbitda:
				deltaEbitda += eff.Amount
			case maNITrattamentoPFN:
				deltaPfn += eff.Amount
			}
		}
		summary := &maDeepBriefAdjustedSummary{DeltaEbitda: deltaEbitda, DeltaPfn: deltaPfn}
		if adjusted, _ := buildMAAdjustedValuation(deep.Valuation, deep.Scorecard, in.aligned, s.loadPricing(ctx)); adjusted != nil {
			summary.EquityLow = adjusted.EquityLow
			summary.EquityHigh = adjusted.EquityHigh
		}
		fctx.Adjusted = summary
	}
	return fctx
}

// maCanonicalExtractSummary builds a compact CEE summary from an already-read filing extract
// (best-effort; nil when the extract was absent).
func maCanonicalExtractSummary(sp maFilingSP, ce maFilingCE, found bool) *maDeepBriefCEEExtract {
	if !found {
		return nil
	}
	return &maDeepBriefCEEExtract{
		TotaleAttivo:     sp.TotaleAttivo,
		ValoreProduzione: ce.ValoreProduzione,
		PatrimonioNetto:  sp.TotalePatrimonioNetto,
		UtileEsercizio:   ce.UtileEsercizio,
	}
}

// maDeepBriefStale reports whether the cached brief is stale relative to the analyst's decisions:
// true when there is a decision (maxDecision != nil) newer than the brief stamp (or the brief was
// never stamped). Pure — no persisted flag; the staleness is derived on read.
func maDeepBriefStale(briefGeneratedAt, maxDecision *time.Time) bool {
	if maxDecision == nil {
		return false
	}
	return briefGeneratedAt == nil || maxDecision.After(*briefGeneratedAt)
}
