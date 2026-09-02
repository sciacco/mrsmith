package smartpassive

import (
	"sort"
	"strings"
)

// The cascade runs the matching as a sequence of levels: each level tries to
// close the invoice and hands the rest to the next one.
//
//  1. sdi: the order reference the supplier wrote in the electronic invoice,
//     resolved to Alyante orders through the RDA or PA code.
//  2. orders: line by line against the open orders of the same supplier.
//  3. residual: whatever is left, split by reason.
//
// Closed invoices are checked against the orders AFC linked in Alyante.

const (
	cascadeLevelSDI      = "sdi"
	cascadeLevelOrders   = "orders"
	cascadeLevelResidual = "residual"
)

type MatchingCascadeInvoice struct {
	// Level where the invoice stopped: sdi, orders, residual.
	Level string `json:"level"`
	// Proposals are the order labels proposed by the closing level.
	Proposals []string `json:"proposals"`
	// Verdict vs the AFC link: match, partial, wrong, no_truth for closed
	// invoices; afc_linked or afc_unlinked for the residual.
	Verdict string `json:"verdict"`
	// SDIReason tells why level 1 did not close: no_xml, no_ref (no order
	// reference in the XML), no_code (reference without a PO or PA code),
	// unresolved (code not found).
	SDIReason string `json:"sdi_reason"`
	// OrdersReason tells why level 2 did not close: no_orders, no_open_orders,
	// no_match, ambiguous.
	OrdersReason string `json:"orders_reason"`
}

type MatchingCascadeLevel struct {
	Key     string `json:"key"`
	Entered int    `json:"entered"`
	Closed  int    `json:"closed"`
	Passed  int    `json:"passed"`
	Match   int    `json:"match"`
	Partial int    `json:"partial"`
	Wrong   int    `json:"wrong"`
	NoTruth int    `json:"no_truth"`
}

type MatchingCascadeReason struct {
	Reason      string `json:"reason"`
	Count       int    `json:"count"`
	WithAFCLink int    `json:"with_afc_link"`
}

type MatchingCascadeSummary struct {
	Invoices            int                     `json:"invoices"`
	Levels              []MatchingCascadeLevel  `json:"levels"`
	Residual            int                     `json:"residual"`
	ResidualWithAFCLink int                     `json:"residual_with_afc_link"`
	ResidualBySDI       []MatchingCascadeReason `json:"residual_by_sdi"`
	ResidualByOrders    []MatchingCascadeReason `json:"residual_by_orders"`
	// ResidualByPair crosses the two reasons ("no_xml / ambiguous").
	ResidualByPair []MatchingCascadeReason `json:"residual_by_pair"`
}

func newCascadeSummary() MatchingCascadeSummary {
	return MatchingCascadeSummary{
		Levels: []MatchingCascadeLevel{{Key: cascadeLevelSDI}, {Key: cascadeLevelOrders}},
	}
}

// applyCascade derives the cascade outcome of one invoice from the level
// results already computed, using the AFC links as the check.
func applyCascade(sdi MatchingFunnelSDIResult, orderRules MatchingFunnelOrderRuleResult, supplierOrders int) MatchingCascadeInvoice {
	out := MatchingCascadeInvoice{Proposals: []string{}}

	// Level 1: order reference in the XML.
	switch {
	case !sdi.Linked:
		out.SDIReason = "no_xml"
	case len(sdi.OrderRefs) == 0:
		out.SDIReason = "no_ref"
	default:
		set := map[string]struct{}{}
		withCode := false
		for _, ref := range sdi.OrderRefs {
			if ref.Code != "" {
				withCode = true
			}
			for _, o := range ref.Orders {
				set[o] = struct{}{}
			}
		}
		switch {
		case len(set) > 0:
		case !withCode:
			out.SDIReason = "no_code"
		default:
			out.SDIReason = "unresolved"
		}
		if len(set) > 0 {
			out.Level = cascadeLevelSDI
			out.Proposals = sortedKeys(set)
		}
	}

	// Level 2: line by line on the open orders of the supplier.
	if out.Level == "" {
		unique := len(orderRules.Proposals) == 1 && orderRules.AmbiguousLines == 0
		switch {
		case supplierOrders == 0:
			out.OrdersReason = "no_orders"
		case orderRules.OpenCandidates == 0:
			out.OrdersReason = "no_open_orders"
		case len(orderRules.Proposals) == 0:
			out.OrdersReason = "no_match"
		case !unique:
			out.OrdersReason = "ambiguous"
		default:
			out.Level = cascadeLevelOrders
			out.Proposals = append(out.Proposals, orderRules.Proposals[0]...)
			sort.Strings(out.Proposals)
		}
	}

	truth := append([]string(nil), orderRules.AFCLinks...)
	sort.Strings(truth)
	if out.Level == "" {
		out.Level = cascadeLevelResidual
		if len(truth) > 0 {
			out.Verdict = "afc_linked"
		} else {
			out.Verdict = "afc_unlinked"
		}
		return out
	}
	out.Verdict = cascadeVerdict(out.Proposals, truth)
	return out
}

// cascadeVerdict compares a proposal with the orders AFC linked: match when the
// sets are equal, partial when the proposal is part of the AFC set, wrong
// otherwise, no_truth when AFC linked nothing.
func cascadeVerdict(proposals, truth []string) string {
	if len(truth) == 0 {
		return "no_truth"
	}
	if strings.Join(proposals, "|") == strings.Join(truth, "|") {
		return "match"
	}
	truthSet := map[string]struct{}{}
	for _, t := range truth {
		truthSet[t] = struct{}{}
	}
	for _, p := range proposals {
		if _, ok := truthSet[p]; !ok {
			return "wrong"
		}
	}
	return "partial"
}

func addCascade(summary *MatchingCascadeSummary, result MatchingCascadeInvoice) {
	summary.Invoices++
	summary.Levels[0].Entered++
	if result.Level != cascadeLevelSDI {
		summary.Levels[0].Passed++
		summary.Levels[1].Entered++
	}
	switch result.Level {
	case cascadeLevelSDI:
		closeLevel(&summary.Levels[0], result.Verdict)
	case cascadeLevelOrders:
		closeLevel(&summary.Levels[1], result.Verdict)
	default:
		summary.Levels[1].Passed++
		summary.Residual++
		linked := result.Verdict == "afc_linked"
		if linked {
			summary.ResidualWithAFCLink++
		}
		summary.ResidualBySDI = addReason(summary.ResidualBySDI, result.SDIReason, linked)
		summary.ResidualByOrders = addReason(summary.ResidualByOrders, result.OrdersReason, linked)
		summary.ResidualByPair = addReason(summary.ResidualByPair, result.SDIReason+" / "+result.OrdersReason, linked)
	}
}

func closeLevel(level *MatchingCascadeLevel, verdict string) {
	level.Closed++
	switch verdict {
	case "match":
		level.Match++
	case "partial":
		level.Partial++
	case "wrong":
		level.Wrong++
	default:
		level.NoTruth++
	}
}

func addReason(list []MatchingCascadeReason, reason string, linked bool) []MatchingCascadeReason {
	for i := range list {
		if list[i].Reason == reason {
			list[i].Count++
			if linked {
				list[i].WithAFCLink++
			}
			return list
		}
	}
	item := MatchingCascadeReason{Reason: reason, Count: 1}
	if linked {
		item.WithAFCLink = 1
	}
	return append(list, item)
}

func sortReasons(list []MatchingCascadeReason) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].Reason < list[j].Reason
	})
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
