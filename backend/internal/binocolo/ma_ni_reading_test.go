package binocolo

import (
	"testing"
	"time"
)

// Approved test #2 (issue #78): the nota-integrativa decision reducer, proposal fingerprint,
// citation validation, and cross-run auto-reconfirm — all driven by pure logic + the frozen F5
// fixture. Zero external calls (no llm.Service, no DocuEngine, no DB). It proves the central
// invariant: the same fact ratified once and auto-reconfirmed across a re-read produces exactly
// ONE effective adjustment (no double effect).

func niF(v float64) *float64 { return &v }
func niInt(v int) *int       { return &v }

func niDate(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func niTS(sec int) time.Time {
	return time.Date(2026, 7, 22, 10, 0, sec, 0, time.UTC)
}

// ---------------------------------------------------------------------------
// (1) Reducer: ratify / reject / revoke, promotion, uncertain, ordering.
// ---------------------------------------------------------------------------

func TestReduceMANIEffectiveAdjustments(t *testing.T) {
	d2024 := niDate(2024, 12, 31)

	prop := func(id, trattamento, direction string, rett *float64) maNIProposal {
		return maNIProposal{ID: id, TrattamentoCandidato: trattamento, Direction: direction, ImportoRettifica: rett, ExerciseDate: &d2024}
	}
	dec := func(id, propID, action string, amount *float64, treatment string, at time.Time) maNIDecision {
		return maNIDecision{ID: id, ProposalID: propID, Action: action, RatifiedAmount: amount, RatifiedTreatment: treatment, CreatedAt: at}
	}

	proposals := []maNIProposal{
		prop("p-ratify", maNITrattamentoEbitda, maNIDirectionIncrease, niF(31750)), // (a) simple ratify → uses importo_rettifica
		prop("p-revoke", maNITrattamentoPFN, maNIDirectionDecrease, niF(-250000)),  // (b) ratify then revoke → not effective
		prop("p-reject", maNITrattamentoEbitda, maNIDirectionIncrease, niF(18000)), // (c) reject → not effective
		prop("p-promote", maNITrattamentoDDOnly, maNIDirectionIncrease, nil),       // (d) dd_only promoted to ebitda by ratified_treatment
		prop("p-unc-noamt", maNITrattamentoPFN, maNIDirectionUncertain, nil),       // (e) uncertain w/o ratified amount → never effective
		prop("p-unc-amt", maNITrattamentoPFN, maNIDirectionUncertain, nil),         // (f) uncertain WITH ratified amount+treatment → effective
		prop("p-order", maNITrattamentoEbitda, maNIDirectionIncrease, niF(10000)),  // (g) out-of-order decisions → latest created_at wins
		prop("p-tie", maNITrattamentoEbitda, maNIDirectionIncrease, niF(5000)),     // (h) same created_at → higher id wins
		prop("p-none", maNITrattamentoEbitda, maNIDirectionIncrease, niF(9999)),    // no decision → not effective
	}

	decisions := []maNIDecision{
		dec("d1", "p-ratify", maNIActionRatify, nil, "", niTS(1)),
		dec("d2", "p-revoke", maNIActionRatify, nil, "", niTS(1)),
		dec("d3", "p-revoke", maNIActionRevoke, nil, "", niTS(2)),
		dec("d4", "p-reject", maNIActionReject, nil, "", niTS(1)),
		dec("d5", "p-promote", maNIActionRatify, niF(25000), maNITrattamentoEbitda, niTS(1)),
		dec("d6", "p-unc-noamt", maNIActionRatify, nil, "", niTS(1)),
		dec("d7", "p-unc-amt", maNIActionRatify, niF(-250000), maNITrattamentoPFN, niTS(1)),
		// out of order in the slice; latest by created_at is the ratify at niTS(3).
		dec("d8", "p-order", maNIActionRatify, nil, "", niTS(3)),
		dec("d9", "p-order", maNIActionReject, nil, "", niTS(1)),
		dec("d10", "p-order", maNIActionRevoke, nil, "", niTS(2)),
		// same created_at → higher id ("d12") wins (revoke), so not effective.
		dec("d11", "p-tie", maNIActionRatify, nil, "", niTS(5)),
		dec("d12", "p-tie", maNIActionRevoke, nil, "", niTS(5)),
	}

	got := reduceMANIEffectiveAdjustments(proposals, decisions)
	byID := map[string]maNIEffectiveAdjustment{}
	for _, adj := range got {
		byID[adj.ProposalID] = adj
	}

	// (a) simple ratify → effective, signed amount from importo_rettifica.
	if adj, ok := byID["p-ratify"]; !ok {
		t.Errorf("p-ratify: expected effective, got none")
	} else if adj.Treatment != maNITrattamentoEbitda || adj.Amount != 31750 {
		t.Errorf("p-ratify: got treatment=%q amount=%v, want ebitda +31750", adj.Treatment, adj.Amount)
	}
	// (b) ratify then revoke → not effective.
	if _, ok := byID["p-revoke"]; ok {
		t.Errorf("p-revoke: ratify+revoke must NOT be effective")
	}
	// (c) reject → not effective.
	if _, ok := byID["p-reject"]; ok {
		t.Errorf("p-reject: reject must NOT be effective")
	}
	// (d) dd_only promoted to ebitda with ratified amount.
	if adj, ok := byID["p-promote"]; !ok {
		t.Errorf("p-promote: expected effective after dd_only→ebitda promotion")
	} else if adj.Treatment != maNITrattamentoEbitda || adj.Amount != 25000 {
		t.Errorf("p-promote: got treatment=%q amount=%v, want ebitda +25000", adj.Treatment, adj.Amount)
	}
	// (e) uncertain without ratified amount → never effective.
	if _, ok := byID["p-unc-noamt"]; ok {
		t.Errorf("p-unc-noamt: uncertain w/o ratified amount must NOT be effective")
	}
	// (f) uncertain WITH ratified amount+treatment → effective.
	if adj, ok := byID["p-unc-amt"]; !ok {
		t.Errorf("p-unc-amt: expected effective with ratified amount+treatment")
	} else if adj.Treatment != maNITrattamentoPFN || adj.Amount != -250000 {
		t.Errorf("p-unc-amt: got treatment=%q amount=%v, want pfn -250000", adj.Treatment, adj.Amount)
	}
	// (g) out-of-order decisions → latest created_at (ratify) wins.
	if adj, ok := byID["p-order"]; !ok {
		t.Errorf("p-order: expected effective (latest decision is ratify)")
	} else if adj.Amount != 10000 {
		t.Errorf("p-order: got amount=%v, want +10000", adj.Amount)
	}
	// (h) tie on created_at → higher id (revoke) wins → not effective.
	if _, ok := byID["p-tie"]; ok {
		t.Errorf("p-tie: at created_at tie the higher-id decision (revoke) must win")
	}
	// no decision → not effective.
	if _, ok := byID["p-none"]; ok {
		t.Errorf("p-none: a proposal with no decision must NOT be effective")
	}

	if len(got) != 4 {
		t.Errorf("effective adjustments = %d, want 4 (p-ratify, p-promote, p-unc-amt, p-order)", len(got))
	}
}

// ---------------------------------------------------------------------------
// (2) Fingerprint: stable on quote whitespace/case; amount-sensitive; dedup.
// ---------------------------------------------------------------------------

func TestMANIProposalFingerprint(t *testing.T) {
	d2024 := niDate(2024, 12, 31)
	quote := "per Euro 250.000,00 dalla liquidita' presente in un conto corrente vincolato"
	messy := "  PER   EURO  250.000,00  dalla LIQUIDITA' presente  in un CONTO corrente vincolato "

	base := maNIProposalFingerprint(maNITrattamentoPFN, &d2024, niF(250000), niF(-250000), quote)
	same := maNIProposalFingerprint(maNITrattamentoPFN, &d2024, niF(250000), niF(-250000), messy)
	if base != same {
		t.Errorf("fingerprint must be stable across quote whitespace/case:\n base=%s\n messy=%s", base, same)
	}
	if len(base) != 32 {
		t.Errorf("fingerprint length = %d, want 32 (hex prefix)", len(base))
	}

	// A different amount ⇒ a different fingerprint.
	if diff := maNIProposalFingerprint(maNITrattamentoPFN, &d2024, niF(250001), niF(-250000), quote); diff == base {
		t.Errorf("fingerprint must change when importo_lordo changes")
	}
	// A nil importo_rettifica ("null") ⇒ a different fingerprint than a valued one.
	if diff := maNIProposalFingerprint(maNITrattamentoPFN, &d2024, niF(250000), nil, quote); diff == base {
		t.Errorf("fingerprint must distinguish importo_rettifica null from a value")
	}

	// Intra-run dedup: two proposals with the same fingerprint collapse to one.
	fp := base
	deduped := dedupMANIProposalsByFingerprint([]maNIProposal{
		{ID: "a", Fingerprint: fp},
		{ID: "b", Fingerprint: fp},
		{ID: "c", Fingerprint: "other"},
	})
	if len(deduped) != 2 {
		t.Errorf("dedup: got %d, want 2 (one per distinct fingerprint)", len(deduped))
	}
}

// ---------------------------------------------------------------------------
// (3) Auto-reconfirm matcher: ratify/reject carried, revoke not; most-recent wins.
// ---------------------------------------------------------------------------

func TestMatchMANIAutoReconfirm(t *testing.T) {
	newProps := []maNINewProposal{{ProposalID: "np1", Fingerprint: "fpA"}}

	ratify := &maNIDecision{ID: "hist-d1", Action: maNIActionRatify, RatifiedAmount: niF(31750), RatifiedTreatment: maNITrattamentoEbitda, ActorEmail: "analyst@x", Reason: "compensi eccedenza"}
	prior := []maNIPriorProposal{{ProposalID: "pp1", Fingerprint: "fpA", CreatedAt: niTS(1), Decision: ratify}}

	got := matchMANIAutoReconfirm(newProps, prior)
	if len(got) != 1 {
		t.Fatalf("ratify prior: got %d clones, want 1", len(got))
	}
	if got[0].NewProposalID != "np1" || got[0].HistoricalDecisionID != "hist-d1" || got[0].Action != maNIActionRatify {
		t.Errorf("clone mismatch: %+v", got[0])
	}
	if got[0].RatifiedAmount == nil || *got[0].RatifiedAmount != 31750 || got[0].RatifiedTreatment != maNITrattamentoEbitda || got[0].ActorEmail != "analyst@x" {
		t.Errorf("clone must carry the historical decision verbatim: %+v", got[0])
	}

	// A reject standing decision is carried too.
	reject := &maNIDecision{ID: "hist-d2", Action: maNIActionReject, ActorEmail: "analyst@x"}
	if r := matchMANIAutoReconfirm(newProps, []maNIPriorProposal{{ProposalID: "pp2", Fingerprint: "fpA", CreatedAt: niTS(1), Decision: reject}}); len(r) != 1 || r[0].Action != maNIActionReject {
		t.Errorf("reject prior must be carried, got %+v", r)
	}

	// A revoke standing decision is NOT carried (the fact is un-decided again).
	revoke := &maNIDecision{ID: "hist-d3", Action: maNIActionRevoke, ActorEmail: "analyst@x"}
	if r := matchMANIAutoReconfirm(newProps, []maNIPriorProposal{{ProposalID: "pp3", Fingerprint: "fpA", CreatedAt: niTS(1), Decision: revoke}}); len(r) != 0 {
		t.Errorf("revoke prior must NOT be carried, got %+v", r)
	}

	// Most-recent prior wins: an older ratify then a newer revoke on the same fingerprint ⇒
	// the standing state is revoked ⇒ nothing carried.
	older := maNIPriorProposal{ProposalID: "pp-old", Fingerprint: "fpA", CreatedAt: niTS(1), Decision: &maNIDecision{ID: "old", Action: maNIActionRatify, ActorEmail: "a@x"}}
	newer := maNIPriorProposal{ProposalID: "pp-new", Fingerprint: "fpA", CreatedAt: niTS(9), Decision: &maNIDecision{ID: "new", Action: maNIActionRevoke, ActorEmail: "a@x"}}
	if r := matchMANIAutoReconfirm(newProps, []maNIPriorProposal{older, newer}); len(r) != 0 {
		t.Errorf("most-recent prior is a revoke ⇒ nothing carried, got %+v", r)
	}
	// And the reverse: older revoke, newer ratify ⇒ carried.
	older.Decision = &maNIDecision{ID: "old2", Action: maNIActionRevoke, ActorEmail: "a@x"}
	newer.Decision = &maNIDecision{ID: "new2", Action: maNIActionRatify, RatifiedAmount: niF(18000), RatifiedTreatment: maNITrattamentoPFN, ActorEmail: "a@x"}
	if r := matchMANIAutoReconfirm(newProps, []maNIPriorProposal{older, newer}); len(r) != 1 || r[0].HistoricalDecisionID != "new2" {
		t.Errorf("most-recent prior is a ratify ⇒ carried from newest, got %+v", r)
	}
}

// ---------------------------------------------------------------------------
// (4) Citation validation on the F5 fixture: grounded / invented / amount-missing.
// ---------------------------------------------------------------------------

func TestValidateMANICitation(t *testing.T) {
	pages := loadFilingFixture(t, "digital_system_2024.json")
	idx := buildMANIPageIndex(pages)

	// The real p.10 conto-corrente-vincolato quote (verbatim substring of the fixture).
	quote10 := "per Euro 250.000,00 dalla liquidita' presente in un conto corrente vincolato"

	// (a) Grounded quote + amount present ⇒ valid.
	if v := validateMANICitation(maNIProposalLLM{Quote: quote10, PageNo: niInt(10), ImportoLordo: niF(250000)}, idx); v.Discard || v.AmountMissing {
		t.Errorf("real p.10 quote must be valid, got %+v", v)
	}

	// (b) Invented quote ⇒ discard.
	if v := validateMANICitation(maNIProposalLLM{Quote: "un conto vincolato di 999 milioni che non esiste", PageNo: niInt(10), ImportoLordo: niF(250000)}, idx); !v.Discard {
		t.Errorf("invented quote must be discarded, got %+v", v)
	}

	// (c) Real quote but the declared amount is NOT in it ⇒ amount missing (demote), not discard.
	if v := validateMANICitation(maNIProposalLLM{Quote: quote10, PageNo: niInt(10), ImportoLordo: niF(123456)}, idx); v.Discard || !v.AmountMissing {
		t.Errorf("real quote with an amount absent from it must demote (AmountMissing), got %+v", v)
	}

	// (d) Same quote, different whitespace/case ⇒ still valid (normalization).
	messy := "   PER   EURO  250.000,00   dalla LIQUIDITA' presente in un  CONTO corrente vincolato  "
	if v := validateMANICitation(maNIProposalLLM{Quote: messy, PageNo: niInt(10), ImportoLordo: niF(250000)}, idx); v.Discard || v.AmountMissing {
		t.Errorf("whitespace/case-different quote must be valid, got %+v", v)
	}

	// (e) Off-by-one page: quote is on p.10 but cited as p.11 ⇒ corrected to 10, not discarded.
	if v := validateMANICitation(maNIProposalLLM{Quote: quote10, PageNo: niInt(11), ImportoLordo: niF(250000)}, idx); v.Discard || v.CorrectedPage == nil || *v.CorrectedPage != 10 {
		t.Errorf("off-by-one page must be corrected to 10, got %+v", v)
	}

	// (f) The p.17 compensi quote grounds its 31.750 amount.
	quote17 := "L'ammontare dei compensi lordi annui corrisposti agli amministratori ammonta ad Euro 31.750,00"
	if v := validateMANICitation(maNIProposalLLM{Quote: quote17, PageNo: niInt(17), ImportoLordo: niF(31750)}, idx); v.Discard || v.AmountMissing {
		t.Errorf("real p.17 compensi quote must be valid, got %+v", v)
	}
}

// ---------------------------------------------------------------------------
// (5) No double effect: a fact ratified in run1 and auto-reconfirmed in run2 yields ONE
//     effective adjustment when the reducer runs over the ACTIVE (run2) data only.
// ---------------------------------------------------------------------------

func TestMANINoDoubleEffect(t *testing.T) {
	d2024 := niDate(2024, 12, 31)
	const fp = "fp-shared-fact"

	// run1: the analyst ratified the fact.
	run1Prop := maNIProposal{ID: "r1-p1", Fingerprint: fp, TrattamentoCandidato: maNITrattamentoEbitda, Direction: maNIDirectionIncrease, ImportoRettifica: niF(31750), ExerciseDate: &d2024}
	run1Dec := maNIDecision{ID: "r1-d1", ProposalID: "r1-p1", Action: maNIActionRatify, CreatedAt: niTS(1), ActorEmail: "analyst@x"}

	// run2: a re-read produced an identical-fingerprint proposal; auto-reconfirm cloned the
	// decision onto it (auto_reconfirmed_from = the run1 decision).
	run2Prop := maNIProposal{ID: "r2-p1", Fingerprint: fp, TrattamentoCandidato: maNITrattamentoEbitda, Direction: maNIDirectionIncrease, ImportoRettifica: niF(31750), ExerciseDate: &d2024}
	clones := matchMANIAutoReconfirm(
		[]maNINewProposal{{ProposalID: run2Prop.ID, Fingerprint: fp}},
		[]maNIPriorProposal{{ProposalID: run1Prop.ID, Fingerprint: fp, CreatedAt: niTS(1), Decision: &run1Dec}},
	)
	if len(clones) != 1 || clones[0].NewProposalID != "r2-p1" || clones[0].HistoricalDecisionID != "r1-d1" {
		t.Fatalf("expected one clone onto r2-p1 from r1-d1, got %+v", clones)
	}
	run2Dec := maNIDecision{ID: "r2-d1", ProposalID: clones[0].NewProposalID, Action: clones[0].Action, RatifiedAmount: clones[0].RatifiedAmount, RatifiedTreatment: clones[0].RatifiedTreatment, CreatedAt: niTS(2), ActorEmail: clones[0].ActorEmail, AutoReconfirmedFrom: clones[0].HistoricalDecisionID}
	if run2Dec.AutoReconfirmedFrom != "r1-d1" {
		t.Errorf("cloned decision must record auto_reconfirmed_from=r1-d1, got %q", run2Dec.AutoReconfirmedFrom)
	}

	// The scheda/F7 reduces over the ACTIVE run (run2) only ⇒ exactly one effective adjustment.
	active := reduceMANIEffectiveAdjustments([]maNIProposal{run2Prop}, []maNIDecision{run2Dec})
	if len(active) != 1 {
		t.Fatalf("active-run reduce: got %d effective adjustments, want 1 (no double effect)", len(active))
	}
	if active[0].Amount != 31750 || active[0].Treatment != maNITrattamentoEbitda {
		t.Errorf("active-run adjustment = %+v, want ebitda +31750", active[0])
	}

	// Guard: mixing BOTH runs' data (the bug the scoping prevents) would double-count — shown
	// here so the single-effect result above is meaningful, not accidental.
	both := reduceMANIEffectiveAdjustments([]maNIProposal{run1Prop, run2Prop}, []maNIDecision{run1Dec, run2Dec})
	if len(both) != 2 {
		t.Errorf("mixing both runs must double-count (%d), proving the active-run scoping matters", len(both))
	}
}
