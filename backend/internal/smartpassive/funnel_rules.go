package smartpassive

import (
	"math"
	"sort"
	"strings"
)

// Fixed matching rules under test. They compare the invoice taxable amount with
// the RDAs of the same supplier, to the cent, and are scored against the RDA
// chosen by AFC in the invoice note. Nothing here is persisted or proposed to
// an operator yet.
const (
	ruleFull        = "full"
	ruleInstallment = "installment"
	ruleSum         = "sum"

	// Combinatorial bounds for the sum rule.
	sumPairMaxCandidates   = 60
	sumTripleMaxCandidates = 30

	// Relative gap under which a non-exact candidate is reported as a near miss.
	nearMissRelative = 0.02
)

type MatchingFunnelProposal struct {
	Codes []string `json:"codes"`
	IDs   []int64  `json:"ids"`
}

type MatchingFunnelRuleResult struct {
	Rule      string                   `json:"rule"`
	Proposals []MatchingFunnelProposal `json:"proposals"`
	// Verdict against the AFC note: match, ambiguous, wrong, none,
	// false_positive, silent_ok, no_truth.
	Verdict  string   `json:"verdict"`
	NearMiss *float64 `json:"near_miss"`
}

type MatchingFunnelRulesSummary struct {
	// Invoices whose note names one or more Arak RDAs.
	TruthInvoices int `json:"truth_invoices"`
	Match         int `json:"match"`
	Ambiguous     int `json:"ambiguous"`
	Wrong         int `json:"wrong"`
	None          int `json:"none"`
	// Invoices whose note declares that no RDA exists.
	NoRDAInvoices int `json:"no_rda_invoices"`
	FalsePositive int `json:"false_positive"`
	SilentOK      int `json:"silent_ok"`
	// Invoices without a usable note: proposals are informative only.
	NoTruthInvoices  int `json:"no_truth_invoices"`
	NoTruthProposals int `json:"no_truth_proposals"`
	// Which rule produced the correct answer.
	MatchByFull        int `json:"match_by_full"`
	MatchByInstallment int `json:"match_by_installment"`
	MatchBySum         int `json:"match_by_sum"`
	// Invoices with no exact proposal but a candidate within nearMissRelative.
	NearMisses int `json:"near_misses"`
}

func cents(value float64) int64 {
	return int64(math.Round(value * 100))
}

func applyFunnelRules(invoice funnelInvoice, candidates []funnelRDA, reference MatchingFunnelReference) MatchingFunnelRuleResult {
	result := MatchingFunnelRuleResult{Proposals: []MatchingFunnelProposal{}}
	if invoice.taxableAmount != nil && len(candidates) > 0 {
		amount := cents(*invoice.taxableAmount)
		if amount != 0 {
			result.Rule, result.Proposals = proposeByRules(amount, candidates)
			if len(result.Proposals) == 0 {
				result.NearMiss = nearestRelativeGap(amount, candidates)
			}
		}
	}
	result.Verdict = ruleVerdict(result, reference)
	return result
}

func proposeByRules(amount int64, candidates []funnelRDA) (string, []MatchingFunnelProposal) {
	if proposals := fullAmountProposals(amount, candidates); len(proposals) > 0 {
		return ruleFull, proposals
	}
	if proposals := installmentProposals(amount, candidates); len(proposals) > 0 {
		return ruleInstallment, proposals
	}
	if proposals := sumProposals(amount, candidates); len(proposals) > 0 {
		return ruleSum, proposals
	}
	return "", []MatchingFunnelProposal{}
}

// fullAmountProposals: the invoice equals the whole RDA.
func fullAmountProposals(amount int64, candidates []funnelRDA) []MatchingFunnelProposal {
	out := []MatchingFunnelProposal{}
	for _, rda := range candidates {
		if rda.total != nil && cents(*rda.total) == amount {
			out = append(out, singleProposal(rda))
		}
	}
	return out
}

// installmentProposals: the invoice equals one period of a recurring row
// (monthly fee × quantity × months per period) or the sum of one period of
// every recurring row of the RDA.
func installmentProposals(amount int64, candidates []funnelRDA) []MatchingFunnelProposal {
	out := []MatchingFunnelProposal{}
	for _, rda := range candidates {
		installments := recurringInstallments(rda)
		if len(installments) == 0 {
			continue
		}
		var sum int64
		hit := false
		for _, installment := range installments {
			sum += installment
			if installment == amount {
				hit = true
			}
		}
		if hit || (len(installments) > 1 && sum == amount) {
			out = append(out, singleProposal(rda))
		}
	}
	return out
}

func recurringInstallments(rda funnelRDA) []int64 {
	out := make([]int64, 0)
	for _, line := range rda.lines {
		if classifyFunnelLine(line) != funnelProfileRecurring || !line.mrc.Valid {
			continue
		}
		qty := 1.0
		if line.qty.Valid && line.qty.Float64 > 0 {
			qty = line.qty.Float64
		}
		months := 1.0
		if line.recurrence.Valid && line.recurrence.Int64 > 0 {
			months = float64(line.recurrence.Int64)
		}
		installment := cents(line.mrc.Float64 * qty * months)
		if installment > 0 {
			out = append(out, installment)
		}
	}
	return out
}

// sumProposals: the invoice equals the sum of two or three whole RDAs.
func sumProposals(amount int64, candidates []funnelRDA) []MatchingFunnelProposal {
	out := []MatchingFunnelProposal{}
	if len(candidates) > sumPairMaxCandidates {
		return out
	}
	totals := make([]int64, len(candidates))
	for i, rda := range candidates {
		if rda.total != nil {
			totals[i] = cents(*rda.total)
		}
	}
	for i := 0; i < len(candidates); i++ {
		if totals[i] <= 0 {
			continue
		}
		for j := i + 1; j < len(candidates); j++ {
			if totals[j] <= 0 {
				continue
			}
			if totals[i]+totals[j] == amount {
				out = append(out, groupProposal(candidates[i], candidates[j]))
			}
		}
	}
	if len(out) > 0 || len(candidates) > sumTripleMaxCandidates {
		return out
	}
	for i := 0; i < len(candidates); i++ {
		if totals[i] <= 0 {
			continue
		}
		for j := i + 1; j < len(candidates); j++ {
			if totals[j] <= 0 {
				continue
			}
			for k := j + 1; k < len(candidates); k++ {
				if totals[k] > 0 && totals[i]+totals[j]+totals[k] == amount {
					out = append(out, groupProposal(candidates[i], candidates[j], candidates[k]))
				}
			}
		}
	}
	return out
}

func nearestRelativeGap(amount int64, candidates []funnelRDA) *float64 {
	best := math.MaxFloat64
	for _, rda := range candidates {
		values := []int64{}
		if rda.total != nil && cents(*rda.total) > 0 {
			values = append(values, cents(*rda.total))
		}
		values = append(values, recurringInstallments(rda)...)
		for _, value := range values {
			gap := math.Abs(float64(value-amount)) / float64(amount)
			if gap < best {
				best = gap
			}
		}
	}
	if best > nearMissRelative {
		return nil
	}
	return &best
}

func singleProposal(rda funnelRDA) MatchingFunnelProposal {
	return groupProposal(rda)
}

func groupProposal(rdas ...funnelRDA) MatchingFunnelProposal {
	proposal := MatchingFunnelProposal{Codes: make([]string, 0, len(rdas)), IDs: make([]int64, 0, len(rdas))}
	for _, rda := range rdas {
		proposal.Codes = append(proposal.Codes, strings.ToUpper(rda.code))
		proposal.IDs = append(proposal.IDs, rda.id)
	}
	return proposal
}

// ruleVerdict scores the proposals against the RDAs named by AFC.
func ruleVerdict(result MatchingFunnelRuleResult, reference MatchingFunnelReference) string {
	truth := make([]int64, 0, len(reference.RDAs))
	for _, rda := range reference.RDAs {
		if rda.ID != nil && (rda.Resolution == "resolved" || rda.Resolution == "successor") {
			truth = append(truth, *rda.ID)
		}
	}
	hasProposals := len(result.Proposals) > 0

	switch {
	case reference.Outcome == referenceNoRDADeclared:
		if hasProposals {
			return "false_positive"
		}
		return "silent_ok"
	case len(truth) == 0 || (reference.Outcome != referenceOne && reference.Outcome != referenceMultiple):
		return "no_truth"
	case !hasProposals:
		return "none"
	}

	hits := 0
	for _, proposal := range result.Proposals {
		if sameIDSet(proposal.IDs, truth) {
			hits++
		}
	}
	switch {
	case hits == 1 && len(result.Proposals) == 1:
		return "match"
	case hits >= 1:
		return "ambiguous"
	default:
		return "wrong"
	}
}

func sameIDSet(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	a := append([]int64(nil), left...)
	b := append([]int64(nil), right...)
	sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
	sort.Slice(b, func(i, j int) bool { return b[i] < b[j] })
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func addRuleResult(summary *MatchingFunnelRulesSummary, result MatchingFunnelRuleResult) {
	switch result.Verdict {
	case "match":
		summary.TruthInvoices++
		summary.Match++
		switch result.Rule {
		case ruleFull:
			summary.MatchByFull++
		case ruleInstallment:
			summary.MatchByInstallment++
		case ruleSum:
			summary.MatchBySum++
		}
	case "ambiguous":
		summary.TruthInvoices++
		summary.Ambiguous++
	case "wrong":
		summary.TruthInvoices++
		summary.Wrong++
	case "none":
		summary.TruthInvoices++
		summary.None++
	case "false_positive":
		summary.NoRDAInvoices++
		summary.FalsePositive++
	case "silent_ok":
		summary.NoRDAInvoices++
		summary.SilentOK++
	default:
		summary.NoTruthInvoices++
		if len(result.Proposals) > 0 {
			summary.NoTruthProposals++
		}
	}
	if result.NearMiss != nil {
		summary.NearMisses++
	}
}
