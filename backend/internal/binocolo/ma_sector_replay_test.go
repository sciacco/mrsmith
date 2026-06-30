package binocolo

import "testing"

// Synthetic fixture with a hand-computable oracle, used to verify the replay math
// (precision/recall, distractor-by-label, baseline vs best relative rule) without any
// DB or live calls. Mirrors the shape of the real UC2 review:
//   - the absolute (baseline) rule leaves true-scarta rows in `forse` (the dump);
//   - a relative margin rule recovers them with zero keep leakage.
func replayFixture() []sectorReplayInput {
	d := func(p float64) maConceptScore { return maConceptScore{Kind: "distractor", RerankProb: p} }
	t := func(p float64) maConceptScore { return maConceptScore{Kind: "target", InStrategy: true, RerankProb: p} }
	return []sectorReplayInput{
		{label: "keep", deterministicVerdict: "confirm", concepts: []maConceptScore{t(0.30), d(0.10)}},
		{label: "keep", deterministicVerdict: "ambiguous", concepts: []maConceptScore{t(0.20), d(0.12)}},
		{label: "scarta", deterministicVerdict: "reject", concepts: []maConceptScore{d(0.30), t(0.10)}},
		{label: "scarta", deterministicVerdict: "ambiguous", concepts: []maConceptScore{d(0.18), t(0.12)}},
		{label: "scarta", deterministicVerdict: "ambiguous", concepts: []maConceptScore{d(0.16), t(0.10)}},
		{label: "forse", deterministicVerdict: "ambiguous", concepts: []maConceptScore{d(0.14), t(0.13)}},
	}
}

func TestSectorReplayVerdictPR(t *testing.T) {
	in := replayFixture()

	confirm := sectorReplayVerdictPR(in, "confirm", "keep")
	if confirm.Fired != 1 || confirm.Correct != 1 || confirm.Precision != 1.0 {
		t.Fatalf("confirm PR: got fired=%d correct=%d precision=%.4f, want 1/1/1.0", confirm.Fired, confirm.Correct, confirm.Precision)
	}
	if confirm.TrueTotal != 2 || confirm.Recall != 0.5 {
		t.Fatalf("confirm recall: got trueTotal=%d recall=%.4f, want 2/0.5", confirm.TrueTotal, confirm.Recall)
	}

	reject := sectorReplayVerdictPR(in, "reject", "scarta")
	if reject.Fired != 1 || reject.Correct != 1 || reject.Precision != 1.0 {
		t.Fatalf("reject PR: got fired=%d correct=%d precision=%.4f, want 1/1/1.0", reject.Fired, reject.Correct, reject.Precision)
	}
	if reject.TrueTotal != 3 || reject.Recall != 0.3333 {
		t.Fatalf("reject recall: got trueTotal=%d recall=%.4f, want 3/0.3333", reject.TrueTotal, reject.Recall)
	}
}

func TestSectorReplayDistractorByLabel(t *testing.T) {
	got := sectorReplayDistractorByLabel(replayFixture())
	cases := map[string]SectorReplayDistractorSlice{
		"keep":   {Total: 2, DistractorWins: 0},
		"scarta": {Total: 3, DistractorWins: 3},
		"forse":  {Total: 1, DistractorWins: 1},
	}
	for label, want := range cases {
		if got[label] != want {
			t.Fatalf("distractor[%s]: got %+v, want %+v", label, got[label], want)
		}
	}
}

func TestSectorReplayBaselineVsBest(t *testing.T) {
	rep := computeSectorReplay(replayFixture())
	if rep == nil {
		t.Fatal("computeSectorReplay returned nil for non-empty input")
	}
	// Baseline (absolute thresholds) leaves 2 of 3 true-scarta rows in forse.
	if rep.Baseline.ScartaRecall != 0.3333 {
		t.Fatalf("baseline scarta-recall: got %.4f, want 0.3333", rep.Baseline.ScartaRecall)
	}
	if rep.Baseline.KeepLeak != 0 {
		t.Fatalf("baseline keepLeak: got %d, want 0", rep.Baseline.KeepLeak)
	}
	// A relative rule recovers all true-scarta rows with zero keep leakage.
	if rep.Best.KeepLeak != 0 {
		t.Fatalf("best keepLeak: got %d, want 0 (must never auto-reject a true keep)", rep.Best.KeepLeak)
	}
	if rep.Best.ScartaRecall != 1.0 {
		t.Fatalf("best scarta-recall: got %.4f, want 1.0", rep.Best.ScartaRecall)
	}
	if rep.Best.ScartaRecall <= rep.Baseline.ScartaRecall {
		t.Fatalf("best (%.4f) should beat baseline (%.4f) scarta-recall", rep.Best.ScartaRecall, rep.Baseline.ScartaRecall)
	}
}

func TestComputeSectorReplayEmpty(t *testing.T) {
	if rep := computeSectorReplay(nil); rep != nil {
		t.Fatalf("empty input should yield nil report, got %+v", rep)
	}
}
