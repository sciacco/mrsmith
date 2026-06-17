package binocolo

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMARescoreReport is a re-scoring verification harness — NO API calls.
// It runs scoreMATargetsV2 over archived vendor payloads and prints the full
// breakdown (score, confidence, flags, per-signal points/weight).
//
// Defaults to the two embedded real payloads (BFInformatica, Prometeo). To score
// your own archived data and tune the strategy:
//
//	BINOCOLO_PAYLOADS=/path/payloads.json \
//	BINOCOLO_THESIS=successione \
//	BINOCOLO_ATECO=6290 BINOCOLO_TURNOVER=1200000 BINOCOLO_KEYWORDS=informatica \
//	go test ./internal/binocolo/ -run TestMARescoreReport -v -count=1
//
// payloads.json is a JSON array of vendor company objects (the shape stored in
// binocolo.ma_target.vendor_payload), exportable read-only with:
//
//	psql "$ANISETTA_DSN" -At -c \
//	  "select coalesce(jsonb_agg(vendor_payload), '[]') from binocolo.ma_target where session_id = '<uuid>'" \
//	  > payloads.json
func TestMARescoreReport(t *testing.T) {
	var raws []json.RawMessage
	if path := os.Getenv("BINOCOLO_PAYLOADS"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read payloads: %v", err)
		}
		if err := json.Unmarshal(data, &raws); err != nil {
			t.Fatalf("parse payloads (expected a JSON array): %v", err)
		}
	} else {
		raws = []json.RawMessage{json.RawMessage(bfInformaticaPayload), json.RawMessage(prometeoPayload)}
	}

	targets := make([]MATarget, 0, len(raws))
	for _, raw := range raws {
		target, err := normalizeMATarget(raw)
		if err != nil {
			t.Logf("skip payload: %v", err)
			continue
		}
		targets = append(targets, target)
	}
	if len(targets) == 0 {
		t.Skip("no payloads to score")
	}

	around := 1_200_000
	strategy := MAStrategySpec{
		SectorDescription: "verifica ri-scoring",
		ActivityStatus:    "ATTIVA",
		Thesis:            os.Getenv("BINOCOLO_THESIS"),
		AtecoCandidates:   []MAAtecoCandidate{{Code: "6290"}},
		Keywords:          []string{"informatica"},
		TurnoverAround:    &around,
	}
	if v := os.Getenv("BINOCOLO_ATECO"); v != "" {
		strategy.AtecoCandidates = nil
		for _, code := range strings.Split(v, ",") {
			if code = strings.TrimSpace(code); code != "" {
				strategy.AtecoCandidates = append(strategy.AtecoCandidates, MAAtecoCandidate{Code: code})
			}
		}
	}
	if v := os.Getenv("BINOCOLO_TURNOVER"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			strategy.TurnoverAround = &n
		}
	}
	if v := os.Getenv("BINOCOLO_KEYWORDS"); v != "" {
		strategy.Keywords = strings.Split(v, ",")
	}

	strategy, err := validateMAStrategy(strategy)
	if err != nil {
		t.Fatalf("validate strategy: %v", err)
	}

	scored := scoreMATargetsV2(targets, strategy, maScoringParams{ThesisFitHoldingFactor: 1 - maThesisFitHoldingHaircutPctDefault/100}, time.Now())

	t.Logf("Tesi: %s — %d target", maThesisLabel(strategy.Thesis), len(scored))
	for i, target := range scored {
		t.Logf("#%-2d %-34s score=%3d  conf=%-5s  %s",
			i+1, rescoreTruncate(target.CompanyName, 34), target.Score, target.Confidence, rescoreFlagLabels(target.Flags))
		for _, ev := range target.Evidence {
			t.Logf("      %-24s %6.1f / %-5.0f  %s", ev.Label, ev.Points, ev.Weight, ev.Value)
		}
	}
}

func rescoreFlagLabels(flags []MATargetFlag) string {
	if len(flags) == 0 {
		return ""
	}
	labels := make([]string, 0, len(flags))
	for _, flag := range flags {
		labels = append(labels, flag.Label)
	}
	return "[" + strings.Join(labels, ", ") + "]"
}

func rescoreTruncate(value string, max int) string {
	if len([]rune(value)) <= max {
		return value
	}
	return string([]rune(value)[:max])
}
