package smartpassive

import "sort"

// The suggestion runs the same levels as the cascade with a stricter reading.
// Each level answers in one of three ways: verified (the proposal passed the
// level's own checks: the invoice stops here), hint (the level found
// something it could not verify: the invoice goes on carrying the hint),
// nothing. When no level verifies, the invoice ends with the first hint
// collected, if any, as its best suggestion; otherwise without suggestion.
//
//  1. sdi: verified when every declared reference resolves, none fills the 20
//     characters of the field and the residual of the orders covers the
//     invoice amount. Orders found without these conditions are a hint.
//  2. contracts: verified when one combination of fees sums exactly to the
//     invoice.
//  3. orders: verified when a single combination fits every line, or when a
//     line names one order by its identifying text and the order's residual
//     covers the amount; a hint when the lines fit several orders or the
//     named order has no residual left.
//  4. fixed_fee: the order confirmed on the previous invoice of the series.
//     It rests on AFC's earlier work, not on an RDA, an order or a contract,
//     so it is always a hint.
//
// Whenever a level finds the right order but its residual does not cover the
// amount, the invoice is flagged as billed beyond the order.
//
// Proposals are checked afterwards against the orders AFC linked in Alyante.

// sdiReferenceMaxLen is the size of IdDocumento in the electronic invoice.
const sdiReferenceMaxLen = 20

const (
	suggestionVerified = "verified"
	suggestionHint     = "hint"
	suggestionNone     = "none"
)

// MatchingSuggestionCheck is the answer of one level for one invoice.
type MatchingSuggestionCheck struct {
	Level string `json:"level"`
	// Outcome: verified, hint, none.
	Outcome string `json:"outcome"`
	// Reason tells why the level did not verify. sdi: no_xml, no_ref, no_code,
	// unresolved, truncated, not_covered. contracts: no_contracts, no_match,
	// ambiguous. orders: no_orders, goods_only (only goods orders, loaded at
	// delivery), no_open_orders, no_match, ambiguous, not_covered.
	// fixed_fee: no_series, no_anchor, anchor_only, not_covered.
	Reason    string   `json:"reason"`
	Proposals []string `json:"proposals"`
}

type MatchingSuggestionInvoice struct {
	// Checks are the levels tested, in order, up to the one that verified.
	Checks []MatchingSuggestionCheck `json:"checks"`
	// Level is the level of the best suggestion: the one that verified, or the
	// first that gave a hint; residual when there is no suggestion at all.
	Level string `json:"level"`
	// Verified tells whether the suggestion passed the checks of its level.
	Verified bool `json:"verified"`
	// Proposals are the order labels of the suggestion, or the contract
	// labels for the contracts level.
	Proposals []string `json:"proposals"`
	// Verdict vs the AFC link: match, partial, wrong, no_truth when there is a
	// suggestion; afc_linked or afc_unlinked otherwise.
	Verdict string `json:"verdict"`
	// OverBilled flags an invoice whose order was found by some level but has
	// no residual left for the amount: billed beyond the order.
	OverBilled bool `json:"over_billed"`
	// Family classifies an invoice without suggestion by what exists upstream
	// for its supplier.
	Family string `json:"family"`
}

func (c MatchingSuggestionInvoice) reason(level string) string {
	for _, check := range c.Checks {
		if check.Level == level {
			return check.Reason
		}
	}
	return ""
}

type MatchingSuggestionLevel struct {
	Key      string `json:"key"`
	Tested   int    `json:"tested"`
	Verified int    `json:"verified"`
	Hint     int    `json:"hint"`
	// Verdicts of the verified proposals vs AFC.
	Match   int `json:"match"`
	Partial int `json:"partial"`
	Wrong   int `json:"wrong"`
	NoTruth int `json:"no_truth"`
}

// MatchingSuggestionFinal counts the invoices whose best suggestion comes from
// a level, split by verification and by verdict vs AFC.
type MatchingSuggestionFinal struct {
	Level    string `json:"level"`
	Verified int    `json:"verified"`
	Hint     int    `json:"hint"`
	Match    int    `json:"match"`
	Partial  int    `json:"partial"`
	Wrong    int    `json:"wrong"`
	NoTruth  int    `json:"no_truth"`
}

type MatchingSuggestionSummary struct {
	Invoices int                       `json:"invoices"`
	Levels   []MatchingSuggestionLevel `json:"levels"`
	// Verified, HintOnly and Residual partition the invoices by their final
	// outcome: a verified suggestion, a hint only, no suggestion.
	Verified            int                       `json:"verified"`
	HintOnly            int                       `json:"hint_only"`
	Residual            int                       `json:"residual"`
	ResidualWithAFCLink int                       `json:"residual_with_afc_link"`
	OverBilled          int                       `json:"over_billed"`
	Final               []MatchingSuggestionFinal `json:"final"`
	ResidualBySDI       []MatchingCascadeReason   `json:"residual_by_sdi"`
	ResidualByContracts []MatchingCascadeReason   `json:"residual_by_contracts"`
	ResidualByOrders    []MatchingCascadeReason   `json:"residual_by_orders"`
	ResidualByFixedFee  []MatchingCascadeReason   `json:"residual_by_fixed_fee"`
	ResidualByFamily    []MatchingCascadeReason   `json:"residual_by_family"`
}

func newSuggestionSummary() MatchingSuggestionSummary {
	levels := make([]MatchingSuggestionLevel, 0, len(cascadeLevels))
	final := make([]MatchingSuggestionFinal, 0, len(cascadeLevels))
	for _, key := range cascadeLevels {
		levels = append(levels, MatchingSuggestionLevel{Key: key})
		final = append(final, MatchingSuggestionFinal{Level: key})
	}
	return MatchingSuggestionSummary{Levels: levels, Final: final}
}

func applySuggestion(invoice funnelInvoice, sdi MatchingFunnelSDIResult, contracts contractResult, contractTruth []string, orderRules MatchingFunnelOrderRuleResult, fixedFee fixedFeeResult, supplierOrders []funnelOrder, supplierRDAs []funnelRDA, idx funnelOrderIndex, rdaByID map[int64]funnelRDA) MatchingSuggestionInvoice {
	out := MatchingSuggestionInvoice{Checks: []MatchingSuggestionCheck{}, Proposals: []string{}}
	orderTruth := append([]string(nil), orderRules.AFCLinks...)
	sort.Strings(orderTruth)
	truthFor := func(level string) []string {
		if level == cascadeLevelContracts {
			return contractTruth
		}
		return orderTruth
	}

	checks := []MatchingSuggestionCheck{
		suggestSDI(invoice, sdi, idx),
		suggestContracts(contracts),
		suggestOrders(invoice, orderRules, supplierOrders, idx),
		suggestFixedFee(invoice, fixedFee, idx),
	}
	hint := -1
	for i, check := range checks {
		out.Checks = append(out.Checks, check)
		if check.Reason == "not_covered" {
			out.OverBilled = true
		}
		if check.Outcome == suggestionHint && hint < 0 {
			hint = i
		}
		if check.Outcome == suggestionVerified {
			out.Level, out.Verified, out.Proposals = check.Level, true, check.Proposals
			out.Verdict = cascadeVerdict(out.Proposals, truthFor(check.Level))
			return out
		}
	}
	if hint >= 0 {
		check := checks[hint]
		out.Level, out.Proposals = check.Level, check.Proposals
		out.Verdict = cascadeVerdict(out.Proposals, truthFor(check.Level))
		return out
	}
	out.Level = cascadeLevelResidual
	out.Family = classifyResidual(invoice, supplierOrders, supplierRDAs, idx.rdaByOrder, rdaByID)
	if len(orderTruth) > 0 {
		out.Verdict = "afc_linked"
	} else {
		out.Verdict = "afc_unlinked"
	}
	return out
}

// suggestSDI is level 1. Every declared reference must resolve, the
// supplier's own numbers included: one missing may be an order we do not see.
func suggestSDI(invoice funnelInvoice, sdi MatchingFunnelSDIResult, idx funnelOrderIndex) MatchingSuggestionCheck {
	check := MatchingSuggestionCheck{Level: cascadeLevelSDI, Outcome: suggestionNone, Proposals: []string{}}
	switch {
	case !sdi.Linked:
		check.Reason = "no_xml"
		return check
	case len(sdi.OrderRefs) == 0:
		check.Reason = "no_ref"
		return check
	}
	declared := map[string]struct{}{}
	withCode, unresolved, truncated := false, false, false
	for _, ref := range sdi.OrderRefs {
		if ref.Code != "" {
			withCode = true
		}
		if len(ref.orderKeys) == 0 {
			unresolved = true
		}
		if len(ref.Declared) >= sdiReferenceMaxLen {
			truncated = true
		}
		for _, k := range ref.orderKeys {
			declared[k] = struct{}{}
		}
	}
	if len(declared) == 0 {
		if withCode {
			check.Reason = "unresolved"
		} else {
			check.Reason = "no_code"
		}
		return check
	}
	labels, covered := orderCoverage(invoice, sortedKeys(declared), idx)
	check.Proposals = labels
	switch {
	case unresolved:
		check.Reason = "unresolved"
	case truncated:
		check.Reason = "truncated"
	case !covered:
		check.Reason = "not_covered"
	default:
		check.Outcome = suggestionVerified
		return check
	}
	check.Outcome = suggestionHint
	return check
}

func suggestContracts(contracts contractResult) MatchingSuggestionCheck {
	check := MatchingSuggestionCheck{Level: cascadeLevelContracts, Outcome: suggestionNone, Reason: contracts.Reason, Proposals: []string{}}
	if len(contracts.Proposals) > 0 {
		check.Outcome, check.Reason = suggestionVerified, ""
		check.Proposals = append(check.Proposals, contracts.Proposals...)
	}
	return check
}

func suggestOrders(invoice funnelInvoice, orderRules MatchingFunnelOrderRuleResult, supplierOrders []funnelOrder, idx funnelOrderIndex) MatchingSuggestionCheck {
	check := MatchingSuggestionCheck{Level: cascadeLevelOrders, Outcome: suggestionNone, Proposals: []string{}}
	switch {
	case len(supplierOrders) == 0:
		check.Reason = "no_orders"
	case len(withoutGoodsOrders(supplierOrders)) == 0:
		check.Reason = "goods_only"
	case len(orderRules.Proposals) == 0 && orderRules.OpenCandidates == 0:
		check.Reason = "no_open_orders"
	case len(orderRules.Proposals) == 0:
		check.Reason = "no_match"
	case orderRules.Rule == "description":
		// The order is named by its text; the amount is verified on the
		// residual of the order.
		check.Proposals = append(check.Proposals, orderRules.Proposals[0]...)
		sort.Strings(check.Proposals)
		if _, covered := orderCoverage(invoice, orderRules.keys[0], idx); covered {
			check.Outcome = suggestionVerified
		} else {
			check.Outcome, check.Reason = suggestionHint, "not_covered"
		}
	case len(orderRules.Proposals) == 1 && orderRules.AmbiguousLines == 0:
		check.Outcome = suggestionVerified
		check.Proposals = append(check.Proposals, orderRules.Proposals[0]...)
		sort.Strings(check.Proposals)
	case orderRules.Rule == "lines":
		check.Outcome, check.Reason = suggestionHint, "ambiguous"
		check.Proposals = append(check.Proposals, orderRules.Proposals[0]...)
		sort.Strings(check.Proposals)
	default:
		check.Reason = "ambiguous"
	}
	return check
}

// suggestFixedFee is level 4: the orders AFC confirmed on the previous invoice
// of the series. It rests on AFC's earlier work, so it is always a hint; when
// the residual no longer covers the amount the reason says so.
func suggestFixedFee(invoice funnelInvoice, fixedFee fixedFeeResult, idx funnelOrderIndex) MatchingSuggestionCheck {
	check := MatchingSuggestionCheck{Level: cascadeLevelFixedFee, Outcome: suggestionNone, Reason: fixedFee.Reason, Proposals: []string{}}
	if len(fixedFee.Proposals) == 0 {
		return check
	}
	check.Proposals = append(check.Proposals, fixedFee.Proposals...)
	check.Outcome, check.Reason = suggestionHint, "anchor_only"
	if _, covered := orderCoverage(invoice, fixedFee.OrderKeys, idx); !covered || len(fixedFee.OrderKeys) != len(fixedFee.Proposals) {
		check.Reason = "not_covered"
	}
	return check
}

// orderCoverage weighs the given orders against the invoice amount: what
// they still have to be invoiced must cover the taxable amount. It returns
// their labels and whether they cover it.
func orderCoverage(invoice funnelInvoice, keys []string, idx funnelOrderIndex) ([]string, bool) {
	own := idx.links.own(invoice.key)
	labels := make([]string, 0, len(keys))
	var residual int64
	for _, k := range keys {
		o, ok := idx.byKey[k]
		if !ok {
			continue
		}
		residual += idx.residualAmount(o, own)
		labels = append(labels, o.label())
	}
	sort.Strings(labels)
	if len(labels) == 0 {
		return labels, false
	}
	if invoice.taxableAmount != nil && residual < cents(*invoice.taxableAmount) {
		return labels, false
	}
	return labels, true
}

func addSuggestion(summary *MatchingSuggestionSummary, result MatchingSuggestionInvoice) {
	summary.Invoices++
	if result.OverBilled {
		summary.OverBilled++
	}
	for _, check := range result.Checks {
		for i := range summary.Levels {
			if summary.Levels[i].Key != check.Level {
				continue
			}
			summary.Levels[i].Tested++
			switch check.Outcome {
			case suggestionVerified:
				summary.Levels[i].Verified++
				countVerdict(&summary.Levels[i].Match, &summary.Levels[i].Partial, &summary.Levels[i].Wrong, &summary.Levels[i].NoTruth, result.Verdict)
			case suggestionHint:
				summary.Levels[i].Hint++
			}
		}
	}
	if result.Level != cascadeLevelResidual {
		if result.Verified {
			summary.Verified++
		} else {
			summary.HintOnly++
		}
		for i := range summary.Final {
			if summary.Final[i].Level != result.Level {
				continue
			}
			if result.Verified {
				summary.Final[i].Verified++
			} else {
				summary.Final[i].Hint++
			}
			countVerdict(&summary.Final[i].Match, &summary.Final[i].Partial, &summary.Final[i].Wrong, &summary.Final[i].NoTruth, result.Verdict)
		}
		return
	}
	summary.Residual++
	linked := result.Verdict == "afc_linked"
	if linked {
		summary.ResidualWithAFCLink++
	}
	summary.ResidualBySDI = addReason(summary.ResidualBySDI, result.reason(cascadeLevelSDI), linked)
	summary.ResidualByContracts = addReason(summary.ResidualByContracts, result.reason(cascadeLevelContracts), linked)
	summary.ResidualByOrders = addReason(summary.ResidualByOrders, result.reason(cascadeLevelOrders), linked)
	summary.ResidualByFixedFee = addReason(summary.ResidualByFixedFee, result.reason(cascadeLevelFixedFee), linked)
	summary.ResidualByFamily = addReason(summary.ResidualByFamily, result.Family, linked)
}

func countVerdict(match, partial, wrong, noTruth *int, verdict string) {
	switch verdict {
	case "match":
		*match++
	case "partial":
		*partial++
	case "wrong":
		*wrong++
	default:
		*noTruth++
	}
}
