package smartpassive

import (
	"sort"
	"strings"
	"time"
)

// The cascade runs the matching as a sequence of levels: each level tries to
// close the invoice and hands the rest to the next one.
//
//  1. sdi: the order reference the supplier wrote in the electronic invoice,
//     resolved to Alyante orders through the RDA or PA code.
//  2. contracts: the purchase contracts AFC keeps in Alyante for recurring
//     commitments, by amount (one contract or a combination of them).
//  3. orders: line by line against the open orders of the same supplier.
//  4. fixed_fee: fixed monthly fee recognised by repetition, taking the order
//     confirmed on the previous invoice of the series.
//  5. residual: whatever is left, split by reason.
//
// Contracts exist since July 2026 and not for every supplier: the levels
// after them are the fallback.
//
// Closed invoices are checked against the orders AFC linked in Alyante.

const (
	cascadeLevelSDI       = "sdi"
	cascadeLevelContracts = "contracts"
	cascadeLevelOrders    = "orders"
	cascadeLevelFixedFee  = "fixed_fee"
	cascadeLevelResidual  = "residual"
)

// cascadeLevels lists the closing levels in order.
var cascadeLevels = []string{cascadeLevelSDI, cascadeLevelContracts, cascadeLevelOrders, cascadeLevelFixedFee}

type MatchingCascadeInvoice struct {
	// Level where the invoice stopped: sdi, contracts, orders, fixed_fee,
	// residual.
	Level string `json:"level"`
	// Proposals are the order labels proposed by the closing level, or the
	// contract labels for the contracts level.
	Proposals []string `json:"proposals"`
	// Verdict vs the AFC link: match, partial, wrong, no_truth for closed
	// invoices; afc_linked or afc_unlinked for the residual. The contracts
	// level is checked against the contracts AFC linked, the others against
	// the orders.
	Verdict string `json:"verdict"`
	// ContractsReason tells why the contracts level did not close:
	// no_contracts (none for the supplier at the invoice date), no_match (no
	// combination of fees sums to the invoice), ambiguous (more than one).
	ContractsReason string `json:"contracts_reason"`
	// ContractCandidates is the number of contracts of the supplier at the
	// invoice date.
	ContractCandidates int `json:"contract_candidates"`
	// SDIReason tells why level 1 did not close: no_xml, no_ref (no order
	// reference in the XML), no_code (reference without a PO or PA code),
	// unresolved (a code not found), truncated (reference fills the 20
	// characters of the field), not_covered (the orders found do not have
	// enough residual for the invoice amount).
	SDIReason string `json:"sdi_reason"`
	// OrdersReason tells why level 2 did not close: no_orders, no_open_orders,
	// no_match, ambiguous.
	OrdersReason string `json:"orders_reason"`
	// FixedFeeReason tells why level 3 did not close: no_series (the invoice
	// is not part of a monthly series with the same taxable amount), no_anchor
	// (series without any member AFC linked).
	FixedFeeReason string `json:"fixed_fee_reason"`
	// SeriesSize is the number of invoices in the fixed-fee series, 0 outside.
	SeriesSize int `json:"series_size"`
	// Family classifies a residual invoice by what exists upstream for its
	// supplier: recurring_in_course, goods_orders, service_orders_expired,
	// rda_only, unknown.
	Family string `json:"family"`
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
	ResidualByPair      []MatchingCascadeReason `json:"residual_by_pair"`
	ResidualByContracts []MatchingCascadeReason `json:"residual_by_contracts"`
	ResidualByFixedFee  []MatchingCascadeReason `json:"residual_by_fixed_fee"`
	ResidualByFamily    []MatchingCascadeReason `json:"residual_by_family"`
}

func newCascadeSummary() MatchingCascadeSummary {
	levels := make([]MatchingCascadeLevel, 0, len(cascadeLevels))
	for _, key := range cascadeLevels {
		levels = append(levels, MatchingCascadeLevel{Key: key})
	}
	return MatchingCascadeSummary{Levels: levels}
}

// applyCascade derives the cascade outcome of one invoice from the level
// results already computed, using the AFC links as the check.
func applyCascade(invoice funnelInvoice, sdi MatchingFunnelSDIResult, contracts contractResult, contractTruth []string, orderRules MatchingFunnelOrderRuleResult, fixedFee fixedFeeResult, supplierOrders []funnelOrder, supplierRDAs []funnelRDA, idx funnelOrderIndex, rdaByID map[int64]funnelRDA) MatchingCascadeInvoice {
	out := MatchingCascadeInvoice{Proposals: []string{}}

	// Level 1: order reference in the XML.
	switch {
	case !sdi.Linked:
		out.SDIReason = "no_xml"
	case len(sdi.OrderRefs) == 0:
		out.SDIReason = "no_ref"
	default:
		declared := map[string]struct{}{}
		withCode, unresolved, truncated := false, false, false
		for _, ref := range sdi.OrderRefs {
			if ref.Code != "" {
				withCode = true
			}
			// References without a PO or PA code are the supplier's own
			// numbers (order, offer, charges): they do not count.
			if ref.Code != "" && len(ref.orderKeys) == 0 {
				unresolved = true
			}
			// IdDocumento holds 20 characters: a reference that fills them
			// may have lost the tail of the list of codes.
			if len(ref.Declared) >= sdiReferenceMaxLen {
				truncated = true
			}
			for _, k := range ref.orderKeys {
				declared[k] = struct{}{}
			}
		}
		switch {
		case len(declared) == 0 && !withCode:
			out.SDIReason = "no_code"
		case len(declared) == 0 || unresolved:
			// Every declared code must resolve: one missing means the
			// proposal is incomplete.
			out.SDIReason = "unresolved"
		case truncated:
			out.SDIReason = "truncated"
		default:
			// The declared orders are only candidates: the invoice closes
			// here when their residual covers the invoice amount.
			if covered, ok := orderCoverage(invoice, sortedKeys(declared), idx); ok {
				out.Level = cascadeLevelSDI
				out.Proposals = covered
			} else {
				out.SDIReason = "not_covered"
			}
		}
	}

	// Level 2: the supplier's contracts, by amount. Checked against the
	// contracts AFC linked to the invoice.
	if out.Level == "" {
		out.ContractCandidates = contracts.Candidates
		if len(contracts.Proposals) > 0 {
			out.Level = cascadeLevelContracts
			out.Proposals = append(out.Proposals, contracts.Proposals...)
			out.Verdict = cascadeVerdict(out.Proposals, contractTruth)
			return out
		}
		out.ContractsReason = contracts.Reason
	}

	// Level 3: line by line on the open orders of the supplier.
	if out.Level == "" {
		unique := len(orderRules.Proposals) == 1 && orderRules.AmbiguousLines == 0
		switch {
		case len(supplierOrders) == 0:
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

	// Level 4: fixed monthly fee, order taken from the previous invoice of
	// the series.
	if out.Level == "" {
		out.SeriesSize = fixedFee.SeriesSize
		switch {
		case !fixedFee.InSeries:
			out.FixedFeeReason = "no_series"
		case len(fixedFee.Proposals) == 0:
			out.FixedFeeReason = "no_anchor"
		default:
			out.Level = cascadeLevelFixedFee
			out.Proposals = append(out.Proposals, fixedFee.Proposals...)
		}
	}

	truth := append([]string(nil), orderRules.AFCLinks...)
	sort.Strings(truth)
	if out.Level == "" {
		out.Level = cascadeLevelResidual
		out.Family = classifyResidual(invoice, supplierOrders, supplierRDAs, idx.rdaByOrder, rdaByID)
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
	for i := range summary.Levels {
		summary.Levels[i].Entered++
		if summary.Levels[i].Key == result.Level {
			closeLevel(&summary.Levels[i], result.Verdict)
			return
		}
		summary.Levels[i].Passed++
	}
	summary.Residual++
	linked := result.Verdict == "afc_linked"
	if linked {
		summary.ResidualWithAFCLink++
	}
	summary.ResidualBySDI = addReason(summary.ResidualBySDI, result.SDIReason, linked)
	summary.ResidualByOrders = addReason(summary.ResidualByOrders, result.OrdersReason, linked)
	summary.ResidualByPair = addReason(summary.ResidualByPair, result.SDIReason+" / "+result.OrdersReason, linked)
	summary.ResidualByContracts = addReason(summary.ResidualByContracts, result.ContractsReason, linked)
	summary.ResidualByFixedFee = addReason(summary.ResidualByFixedFee, result.FixedFeeReason, linked)
	summary.ResidualByFamily = addReason(summary.ResidualByFamily, result.Family, linked)
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

// Residual families, in order of precedence.
const (
	familyRecurringInCourse    = "recurring_in_course"
	familyGoodsOrders          = "goods_orders"
	familyServiceOrdersExpired = "service_orders_expired"
	familyRDAOnly              = "rda_only"
	familyUnknown              = "unknown"
)

// classifyResidual tells what exists upstream for the supplier of a residual
// invoice. A service order is "in course" when the invoice date falls within
// its duration: for orders with a known RDA the recurrence rules of the RDA
// decide, otherwise the order lines, whose quantity is the number of months
// for recurring services. A recurring RDA is in course from its creation for
// the initial months, or indefinitely when it renews automatically.
func classifyResidual(invoice funnelInvoice, orders []funnelOrder, rdas []funnelRDA, rdaByOrder map[string]*int64, rdaByID map[int64]funnelRDA) string {
	hasGoods, hasService := false, false
	for _, o := range orders {
		switch {
		case strings.HasPrefix(o.docCode, "BGF"):
			hasGoods = true
		default:
			hasService = true
			if invoice.documentDate == nil {
				continue
			}
			// With the RDA known, its recurrence rules decide; otherwise
			// the order lines: a quantity of 12 or more reads as months.
			if id := rdaByOrder[o.key]; id != nil {
				if rda, ok := rdaByID[*id]; ok && recurringRDACovers(rda, *invoice.documentDate) {
					return familyRecurringInCourse
				}
				continue
			}
			if serviceOrderCovers(o, *invoice.documentDate) {
				return familyRecurringInCourse
			}
		}
	}
	for _, rda := range rdas {
		if invoice.documentDate != nil && recurringRDACovers(rda, *invoice.documentDate) {
			return familyRecurringInCourse
		}
	}
	switch {
	case hasGoods:
		return familyGoodsOrders
	case hasService:
		return familyServiceOrdersExpired
	case len(rdas) > 0:
		return familyRDAOnly
	default:
		return familyUnknown
	}
}

func serviceOrderCovers(o funnelOrder, day time.Time) bool {
	if o.date == nil || o.date.After(day) {
		return false
	}
	months := 0.0
	for _, l := range o.lines {
		if l.isAmountLine() && l.qty != nil && *l.qty > months {
			months = *l.qty
		}
	}
	if months < 12 {
		return false
	}
	end := o.date.AddDate(0, int(months), 0)
	return !day.After(end)
}

func recurringRDACovers(rda funnelRDA, day time.Time) bool {
	start := rda.startDate()
	if start == nil || start.After(day) {
		return false
	}
	for _, l := range rda.lines {
		if classifyFunnelLine(l) != funnelProfileRecurring {
			continue
		}
		if l.automaticRenew.Valid && l.automaticRenew.Bool {
			return true
		}
		if l.initialMonths.Valid && !day.After(start.AddDate(0, int(l.initialMonths.Int64), 0)) {
			return true
		}
	}
	return false
}
