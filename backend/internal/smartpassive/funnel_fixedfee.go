package smartpassive

import (
	"math"
	"sort"
	"strings"
)

// Level 3: fixed monthly fee recognised by repetition. A supplier billing a
// fixed fee sends, every month, one invoice with the same taxable amount
// (Sparkle, Cogent, Arelion, leasing instalments). The series is a property of
// the invoices themselves: same supplier, same taxable amount, at least three
// distinct months, never two documents in the same month. A new invoice of a
// known series takes the order confirmed on the previous one: the anchor is
// the nearest earlier member AFC linked in Alyante, or the nearest later one
// when the invoice is the oldest of the series.
//
// The series is read from the invoices loaded for the scope: in the open
// scope, where settled invoices are missing, it is seen only in part.

const fixedFeeMinMonths = 3

type fixedFeeResult struct {
	InSeries   bool
	SeriesSize int
	Proposals  []string
	// Reason tells why the level did not close: no_series, no_anchor.
	Reason string
}

// buildFixedFeeSeries groups the invoices of one supplier into fixed-fee
// series and indexes them by invoice key. Members are sorted by date.
func buildFixedFeeSeries(invoices []funnelInvoice) map[string][]funnelInvoice {
	byAmount := map[int64][]funnelInvoice{}
	for _, inv := range invoices {
		if inv.taxableAmount == nil || *inv.taxableAmount <= 0 || inv.documentDate == nil {
			continue
		}
		cents := int64(math.Round(*inv.taxableAmount * 100))
		byAmount[cents] = append(byAmount[cents], inv)
	}
	out := map[string][]funnelInvoice{}
	for _, group := range byAmount {
		months := map[string]int{}
		for _, inv := range group {
			months[inv.documentDate.Format("2006-01")]++
		}
		if len(months) < fixedFeeMinMonths {
			continue
		}
		monthly := true
		for _, n := range months {
			if n > 1 {
				monthly = false
				break
			}
		}
		if !monthly {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].documentDate.Before(*group[j].documentDate) })
		for _, inv := range group {
			out[inv.key] = group
		}
	}
	return out
}

// applyFixedFee proposes for an invoice the orders AFC linked on the nearest
// other member of its series.
func applyFixedFee(invoice funnelInvoice, series map[string][]funnelInvoice, idx funnelOrderIndex) fixedFeeResult {
	group, ok := series[invoice.key]
	if !ok {
		return fixedFeeResult{Reason: "no_series"}
	}
	result := fixedFeeResult{InSeries: true, SeriesSize: len(group)}
	var before, after *funnelInvoice
	for i := range group {
		member := &group[i]
		if member.key == invoice.key || len(idx.links.ordersOf(member.key)) == 0 {
			continue
		}
		if !member.documentDate.After(*invoice.documentDate) {
			before = member
		} else if after == nil {
			after = member
		}
	}
	anchor := before
	if anchor == nil {
		anchor = after
	}
	if anchor == nil {
		result.Reason = "no_anchor"
		return result
	}
	for _, key := range idx.links.ordersOf(anchor.key) {
		if o, ok := idx.byKey[key]; ok {
			result.Proposals = append(result.Proposals, o.label())
		} else {
			result.Proposals = append(result.Proposals, "reg. "+strings.TrimPrefix(key, "1:"))
		}
	}
	sort.Strings(result.Proposals)
	return result
}
