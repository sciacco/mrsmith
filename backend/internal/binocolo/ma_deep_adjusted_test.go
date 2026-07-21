package binocolo

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// Approved test #3 (issue #78): the on-read adjusted valuation. It pins the three invariants the
// spec makes binding — (1) ΔEBITDA enters BOTH band extremes and ΔPFN moves equity by its signed
// amount while TFR/tax fund stay put; (2) the exercise-alignment filter and the canonical-filing
// resolution (no filing / not aligned / ambiguous re-deposit); (3) an ev_sales extreme never
// takes ΔEBITDA, and the method is inherited verbatim (never re-selected on the add-back). Plus
// the pure brief-staleness predicate. No DB, no LLM: baselines are built with the real engine and
// the canonical-filing path is driven through a fake store (the F4 pattern).

// adjBaselinePricing mirrors the engine tests: flat 30% haircut (factor 0.7), 5% EBITDA-margin gate.
func adjBaselinePricing() maPricing { return maPricing{SMEHaircutPct: 30, EBITDAFallbackPct: 5} }

// ---------------------------------------------------------------------------
// (1) Signs: ΔEBITDA on both extremes, ΔPFN on equity, TFR/tax fund invariant.
// ---------------------------------------------------------------------------

func TestBuildMAAdjustedValuationSigns(t *testing.T) {
	pricing := adjBaselinePricing()
	// ev_ebitda, prudential low extreme (both margins ≥ 5%): high on reported EBITDA, low on
	// prudential EBITDA. A wide sample (NFirms=290) keeps the caveat quiet.
	turnover, ebitda, prud, pfn := 5_000_000.0, 750_000.0, 600_000.0, 1_000_000.0
	sc := &MADeepScorecard{Turnover: &turnover, Ebitda: &ebitda, PFN: &pfn}
	reading := &maCEEReading{
		EBITDAPrudential: &prud,
		TFR:              niF(100_000),
		TaxFund:          niF(50_000),
		PFN:              &maCEEAmount{Value: pfn, Provenance: maCEEProvDetail},
	}
	evEbitda, evSales := 10.0, 1.5
	multiple := &sectorMultiple{Industry: "Software", EVEbitda: &evEbitda, EVSales: &evSales, NFirms: 290, Source: "Damodaran"}
	base := buildMADeepValuation(sc, reading, multiple, pricing)
	if base == nil || base.Method != "ev_ebitda" || base.LowMethod != "" {
		t.Fatalf("baseline setup: %+v", base)
	}
	// Baseline band (see engine math): high 750k×10×0.7×1.15, low 600k×10×0.7×0.85.
	if base.EVHigh != 6_037_500 || base.EVLow != 3_570_000 {
		t.Fatalf("baseline EV band: low=%v high=%v", base.EVLow, base.EVHigh)
	}
	if base.EquityHigh == nil || *base.EquityHigh != 4_887_500 || base.EquityLow == nil || *base.EquityLow != 2_420_000 {
		t.Fatalf("baseline equity: low=%v high=%v", base.EquityLow, base.EquityHigh)
	}

	d2024 := niDate(2024, 12, 31)
	addBack := maNIEffectiveAdjustment{ProposalID: "p1", Treatment: maNITrattamentoEbitda, ExerciseDate: &d2024, Amount: 50_000}

	// (a) EBITDA add-back only: EBITDA_adj = base + 50k enters BOTH extremes; EV up, equity up.
	adj, flag := buildMAAdjustedValuation(base, sc, []maNIEffectiveAdjustment{addBack}, pricing)
	if adj == nil {
		t.Fatal("nil adjusted valuation")
	}
	if flag {
		t.Errorf("evSalesDeltaEbitdaNotApplied must be false on a pure EBITDA path")
	}
	// high 800k×10×0.7×1.15 = 6,440,000; low (prudential) 650k×10×0.7×0.85 = 3,867,500.
	if adj.EVHigh != 6_440_000 || adj.EVLow != 3_867_500 {
		t.Fatalf("adjusted EV band (both extremes +Δ): low=%v high=%v", adj.EVLow, adj.EVHigh)
	}
	if adj.EVHigh <= base.EVHigh || adj.EVLow <= base.EVLow {
		t.Errorf("EV must rise on both extremes: base(%v,%v) adj(%v,%v)", base.EVLow, base.EVHigh, adj.EVLow, adj.EVHigh)
	}
	if adj.PrudentialEbitda == nil || *adj.PrudentialEbitda != 650_000 {
		t.Errorf("prudential EBITDA must be adjusted by the same Δ: %v", adj.PrudentialEbitda)
	}
	// deductions unchanged (PFN 1,000,000 + TFR 100,000 + tax fund 50,000): equity up by the EV rise.
	if adj.EquityHigh == nil || *adj.EquityHigh != 5_290_000 || adj.EquityLow == nil || *adj.EquityLow != 2_717_500 {
		t.Fatalf("adjusted equity (EBITDA only): low=%v high=%v", adj.EquityLow, adj.EquityHigh)
	}
	if adj.Bridge == nil || adj.Bridge.TFR == nil || adj.Bridge.TFR.Value != 100_000 || adj.Bridge.TaxFund == nil || adj.Bridge.TaxFund.Value != 50_000 {
		t.Fatalf("TFR/tax fund must be invariant in the bridge: %+v", adj.Bridge)
	}

	// (b) ΔPFN +200k (more debt): equity DOWN by exactly 200k vs the EBITDA-only case.
	morePfn := maNIEffectiveAdjustment{ProposalID: "p2", Treatment: maNITrattamentoPFN, ExerciseDate: &d2024, Amount: 200_000}
	adjDebt, _ := buildMAAdjustedValuation(base, sc, []maNIEffectiveAdjustment{addBack, morePfn}, pricing)
	if adjDebt.EquityHigh == nil || *adjDebt.EquityHigh != *adj.EquityHigh-200_000 {
		t.Fatalf("ΔPFN>0 must cut equity by the amount: %v vs %v", adjDebt.EquityHigh, adj.EquityHigh)
	}
	if adjDebt.Bridge.PFN == nil || adjDebt.Bridge.PFN.Value != 1_200_000 {
		t.Errorf("PFN row must reflect PFN_adj: %+v", adjDebt.Bridge.PFN)
	}
	// TFR / tax fund still untouched under a PFN move.
	if adjDebt.Bridge.TFR.Value != 100_000 || adjDebt.Bridge.TaxFund.Value != 50_000 {
		t.Errorf("TFR/tax fund must stay invariant under ΔPFN: %+v", adjDebt.Bridge)
	}

	// (c) ΔPFN −300k (ring-fenced cash-like item): equity UP by exactly 300k vs EBITDA-only.
	lessPfn := maNIEffectiveAdjustment{ProposalID: "p3", Treatment: maNITrattamentoPFN, ExerciseDate: &d2024, Amount: -300_000}
	adjCash, _ := buildMAAdjustedValuation(base, sc, []maNIEffectiveAdjustment{addBack, lessPfn}, pricing)
	if adjCash.EquityHigh == nil || *adjCash.EquityHigh != *adj.EquityHigh+300_000 {
		t.Fatalf("ΔPFN<0 must lift equity by the amount: %v vs %v", adjCash.EquityHigh, adj.EquityHigh)
	}
}

// ---------------------------------------------------------------------------
// (2) EV/Sales invariance + method inheritance (pure).
// ---------------------------------------------------------------------------

func TestBuildMAAdjustedValuationEVSalesInvariant(t *testing.T) {
	pricing := adjBaselinePricing()
	d2024 := niDate(2024, 12, 31)

	// (a) method == ev_sales: EBITDA add-back must NOT move EV/equity; ΔPFN still does.
	turnover, ebitda, prud, pfn := 2_000_000.0, 20_000.0, 15_000.0, 500_000.0 // margin 1% < 5% ⇒ ev_sales
	sc := &MADeepScorecard{Turnover: &turnover, Ebitda: &ebitda, PFN: &pfn}
	reading := &maCEEReading{EBITDAPrudential: &prud, PFN: &maCEEAmount{Value: pfn, Provenance: maCEEProvTotal}}
	evEbitda, evSales := 10.0, 1.5
	multiple := &sectorMultiple{Industry: "X", EVEbitda: &evEbitda, EVSales: &evSales, NFirms: 50}
	base := buildMADeepValuation(sc, reading, multiple, pricing)
	if base == nil || base.Method != "ev_sales" {
		t.Fatalf("expected ev_sales baseline: %+v", base)
	}

	addBack := maNIEffectiveAdjustment{ProposalID: "e", Treatment: maNITrattamentoEbitda, ExerciseDate: &d2024, Amount: 100_000}
	adj, flag := buildMAAdjustedValuation(base, sc, []maNIEffectiveAdjustment{addBack}, pricing)
	if !flag {
		t.Errorf("evSalesDeltaEbitdaNotApplied must be true when an ev_sales extreme drops a ΔEBITDA")
	}
	if adj.EVLow != base.EVLow || adj.EVHigh != base.EVHigh {
		t.Fatalf("ev_sales EV must be invariant under ΔEBITDA: base(%v,%v) adj(%v,%v)", base.EVLow, base.EVHigh, adj.EVLow, adj.EVHigh)
	}
	if adj.EquityLow == nil || *adj.EquityLow != *base.EquityLow || adj.EquityHigh == nil || *adj.EquityHigh != *base.EquityHigh {
		t.Fatalf("ev_sales equity must be invariant under ΔEBITDA: %v/%v", adj.EquityLow, adj.EquityHigh)
	}
	// ΔPFN still flows through the bridge even on ev_sales.
	morePfn := maNIEffectiveAdjustment{ProposalID: "d", Treatment: maNITrattamentoPFN, ExerciseDate: &d2024, Amount: 50_000}
	adjPfn, _ := buildMAAdjustedValuation(base, sc, []maNIEffectiveAdjustment{addBack, morePfn}, pricing)
	if adjPfn.EquityHigh == nil || *adjPfn.EquityHigh != *base.EquityHigh-50_000 {
		t.Fatalf("ev_sales ΔPFN must still cut equity: %v vs %v", adjPfn.EquityHigh, base.EquityHigh)
	}

	// (b) method == ev_ebitda with a LOW extreme on ev_sales: Δ applies ONLY to the high extreme.
	turn2, eb2, prud2, pfn2 := 5_000_000.0, 750_000.0, 50_000.0, 300_000.0 // prud margin 1% ⇒ low falls to ev_sales
	sc2 := &MADeepScorecard{Turnover: &turn2, Ebitda: &eb2, PFN: &pfn2}
	reading2 := &maCEEReading{EBITDAPrudential: &prud2, PFN: &maCEEAmount{Value: pfn2, Provenance: maCEEProvTotal}}
	evEb2, evSl2 := 10.0, 0.8
	mult2 := &sectorMultiple{Industry: "Y", EVEbitda: &evEb2, EVSales: &evSl2, NFirms: 40}
	base2 := buildMADeepValuation(sc2, reading2, mult2, pricing)
	if base2 == nil || base2.Method != "ev_ebitda" || base2.LowMethod != "ev_sales" {
		t.Fatalf("expected ev_ebitda high + ev_sales low: %+v", base2)
	}
	adj2, flag2 := buildMAAdjustedValuation(base2, sc2, []maNIEffectiveAdjustment{{ProposalID: "e2", Treatment: maNITrattamentoEbitda, ExerciseDate: &d2024, Amount: 100_000}}, pricing)
	if !flag2 {
		t.Errorf("evSalesDeltaEbitdaNotApplied must be true (low extreme is ev_sales)")
	}
	if adj2.EVLow != base2.EVLow {
		t.Errorf("ev_sales low extreme must NOT take ΔEBITDA: base=%v adj=%v", base2.EVLow, adj2.EVLow)
	}
	if adj2.EVHigh <= base2.EVHigh {
		t.Errorf("ev_ebitda high extreme must rise with the add-back: base=%v adj=%v", base2.EVHigh, adj2.EVHigh)
	}

	// (c) Method inheritance: a huge add-back that WOULD flip the selection (ev_sales → ev_ebitda)
	// must leave method/multiple/lowMethod/haircut identical — the selector is never re-run.
	huge := maNIEffectiveAdjustment{ProposalID: "big", Treatment: maNITrattamentoEbitda, ExerciseDate: &d2024, Amount: 5_000_000}
	adjHuge, _ := buildMAAdjustedValuation(base, sc, []maNIEffectiveAdjustment{huge}, pricing)
	if adjHuge.Method != base.Method || adjHuge.LowMethod != base.LowMethod || adjHuge.Multiple != base.Multiple || adjHuge.HaircutPct != base.HaircutPct {
		t.Fatalf("method inheritance violated: base{%s/%s/%v/%v} adj{%s/%s/%v/%v}",
			base.Method, base.LowMethod, base.Multiple, base.HaircutPct,
			adjHuge.Method, adjHuge.LowMethod, adjHuge.Multiple, adjHuge.HaircutPct)
	}
	if adjHuge.EVHigh != base.EVHigh || adjHuge.EVLow != base.EVLow {
		t.Errorf("inherited ev_sales must keep EV invariant even under a flip-worthy add-back")
	}
}

// ---------------------------------------------------------------------------
// (3) Canonical-filing resolution + exercise-alignment filter (through the store path).
// ---------------------------------------------------------------------------

// adjFakeFiling drives computeMAAdjustedView without a DB: it embeds the F4 fake (for the rest of
// maFilingStore) and overrides only the three methods the on-read path touches.
type adjFakeFiling struct {
	*fakeFilingStore
	filings   []maFiling
	proposals map[string][]maNIProposalWithDecisions
}

func (f *adjFakeFiling) ListMAFilingsByFiscalKey(context.Context, string) ([]maFiling, error) {
	return f.filings, nil
}
func (f *adjFakeFiling) ListMANIProposalsWithDecisions(_ context.Context, filingID string) ([]maNIProposalWithDecisions, error) {
	return f.proposals[filingID], nil
}
func (f *adjFakeFiling) GetMAFilingExtracts(context.Context, string) ([]maFilingExtract, error) {
	return nil, nil // reconciliation not under test here
}

func adjDeepRecord() *maDeepVATRecord {
	turnover, ebitda, prud, pfn := 5_000_000.0, 750_000.0, 600_000.0, 1_000_000.0
	sc := &MADeepScorecard{Turnover: &turnover, Ebitda: &ebitda, PFN: &pfn}
	reading := &maCEEReading{EBITDAPrudential: &prud, TFR: niF(100_000), PFN: &maCEEAmount{Value: pfn, Provenance: maCEEProvDetail}}
	evEbitda := 10.0
	multiple := &sectorMultiple{Industry: "Software", EVEbitda: &evEbitda, NFirms: 290}
	val := buildMADeepValuation(sc, reading, multiple, adjBaselinePricing())
	// balanceSheetDate → deepVintageKey → baseline exercise 2024-12-31.
	return &maDeepVATRecord{
		CompanyKey: "co-1",
		Status:     "ready",
		Scorecard:  sc,
		Valuation:  val,
		Payload:    json.RawMessage(`{"ecofin":{"balanceSheetDate":"2024-12-31","turnoverYear":2024}}`),
	}
}

func adjRatifiedProp(id string, exercise time.Time, amount float64) maNIProposalWithDecisions {
	return maNIProposalWithDecisions{
		Proposal: maNIProposal{ID: id, ExerciseDate: &exercise, TrattamentoCandidato: maNITrattamentoEbitda, Direction: maNIDirectionIncrease, ImportoRettifica: niF(amount), PageNo: niInt(17), Label: "add-back"},
		Decisions: []maNIDecision{
			{ID: id + "-d", ProposalID: id, Action: maNIActionRatify, CreatedAt: niTS(1)},
		},
	}
}

func TestComputeMAAdjustedViewResolution(t *testing.T) {
	ctx := context.Background()
	baseline := niDate(2024, 12, 31)
	identity := maDeepDiveIdentity{VATCode: "IT01234567890"}
	deep := adjDeepRecord()

	filing := func(id, identityStatus string, closing time.Time) maFiling {
		c := closing
		return maFiling{ID: id, IdentityStatus: identityStatus, ClosingDate: &c, ActiveProcessingRunID: id + "-run", FiscalKey: "01234567890"}
	}

	// (a) No filing at all ⇒ no_filing, zero effects, no adjusted valuation.
	svc := &maService{filing: &adjFakeFiling{fakeFilingStore: newFakeFilingStore()}}
	view, err := svc.computeMAAdjustedView(ctx, identity, deep)
	if err != nil {
		t.Fatalf("no_filing: %v", err)
	}
	if view.Status != maAdjustedStatusNoFiling || view.AdjustedValuation != nil || len(view.Effects) != 0 {
		t.Fatalf("no_filing view: %+v", view)
	}
	if view.BaselineExercise != "2024-12-31" {
		t.Errorf("baseline exercise must come from the vendor payload: %q", view.BaselineExercise)
	}

	// (b) A validated filing exists but for a DIFFERENT exercise ⇒ not_aligned.
	svc = &maService{filing: &adjFakeFiling{fakeFilingStore: newFakeFilingStore(), filings: []maFiling{filing("f-old", maFilingIdentityValidated, niDate(2023, 12, 31))}}}
	view, _ = svc.computeMAAdjustedView(ctx, identity, deep)
	if view.Status != maAdjustedStatusNotAligned || view.AdjustedValuation != nil {
		t.Fatalf("not_aligned view: %+v", view)
	}

	// (c) An aligned filing but identity only pending_validation ⇒ still not_aligned (identity gate).
	svc = &maService{filing: &adjFakeFiling{fakeFilingStore: newFakeFilingStore(), filings: []maFiling{filing("f-pending", maFilingIdentityPendingValidation, baseline)}}}
	view, _ = svc.computeMAAdjustedView(ctx, identity, deep)
	if view.Status != maAdjustedStatusNotAligned {
		t.Fatalf("pending identity must not be canonical: %+v", view)
	}

	// (d) TWO validated filings for the baseline exercise ⇒ ambiguous, count 2, zero effects.
	svc = &maService{filing: &adjFakeFiling{fakeFilingStore: newFakeFilingStore(), filings: []maFiling{
		filing("f-a", maFilingIdentityValidated, baseline),
		filing("f-b", maFilingIdentityOverride, baseline),
	}}}
	view, _ = svc.computeMAAdjustedView(ctx, identity, deep)
	if view.Status != maAdjustedStatusAmbiguous || view.AmbiguousCount != 2 || view.AdjustedValuation != nil || len(view.Effects) != 0 {
		t.Fatalf("ambiguous view: %+v", view)
	}

	// (e) One canonical filing, a ratified add-back for the WRONG exercise ⇒ filtered ⇒ no_effects.
	fk := &adjFakeFiling{fakeFilingStore: newFakeFilingStore(),
		filings:   []maFiling{filing("f1", maFilingIdentityValidated, baseline)},
		proposals: map[string][]maNIProposalWithDecisions{"f1": {adjRatifiedProp("p-2023", niDate(2023, 12, 31), 40_000)}},
	}
	svc = &maService{filing: fk}
	view, _ = svc.computeMAAdjustedView(ctx, identity, deep)
	if view.Status != maAdjustedStatusNoEffects || view.AdjustedValuation != nil || len(view.Effects) != 0 {
		t.Fatalf("misaligned exercise must be zero effect: %+v", view)
	}
	if view.FilingID != "f1" {
		t.Errorf("canonical filing id must surface: %q", view.FilingID)
	}

	// (f) One canonical filing, a ratified add-back for the BASELINE exercise ⇒ active, one effect,
	//     ΔEBITDA surfaced, adjusted valuation present and above baseline.
	fk = &adjFakeFiling{fakeFilingStore: newFakeFilingStore(),
		filings:   []maFiling{filing("f1", maFilingIdentityValidated, baseline)},
		proposals: map[string][]maNIProposalWithDecisions{"f1": {adjRatifiedProp("p-2024", baseline, 50_000)}},
	}
	svc = &maService{filing: fk}
	view, err = svc.computeMAAdjustedView(ctx, identity, deep)
	if err != nil {
		t.Fatalf("active: %v", err)
	}
	if view.Status != maAdjustedStatusActive || len(view.Effects) != 1 || view.DeltaEbitda != 50_000 || view.DeltaPfn != 0 {
		t.Fatalf("active view: %+v", view)
	}
	if view.AdjustedValuation == nil || view.AdjustedValuation.EVHigh <= deep.Valuation.EVHigh {
		t.Fatalf("adjusted valuation must be present and above baseline: %+v", view.AdjustedValuation)
	}
	if view.Effects[0].ProposalID != "p-2024" || view.Effects[0].Treatment != maNITrattamentoEbitda || view.Effects[0].PageNo == nil || *view.Effects[0].PageNo != 17 {
		t.Errorf("effect provenance mismatch: %+v", view.Effects[0])
	}
}

// ---------------------------------------------------------------------------
// (4) Brief staleness predicate (pure).
// ---------------------------------------------------------------------------

func TestMADeepBriefStale(t *testing.T) {
	t1 := niTS(1)
	t2 := niTS(2)

	if maDeepBriefStale(nil, nil) {
		t.Errorf("no brief + no decision ⇒ not stale")
	}
	if maDeepBriefStale(&t2, &t1) {
		t.Errorf("brief newer than the latest decision ⇒ not stale")
	}
	if !maDeepBriefStale(&t1, &t2) {
		t.Errorf("a decision newer than the brief ⇒ stale")
	}
	if !maDeepBriefStale(nil, &t1) {
		t.Errorf("a decision with a never-generated brief ⇒ stale")
	}
}
