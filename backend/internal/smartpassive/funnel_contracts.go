package smartpassive

import (
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Since July 2026 AFC keeps the recurring commitments as purchase contracts
// in Alyante (document code TSF-CONTRATTO): one document per contract, one
// line with the category article, the description and the fee. Invoices are
// linked to the contract they settle. The contract level of the cascade
// matches an invoice against the contracts of its supplier by amount; the
// links AFC made are the check.

const funnelContractDocCode = "TSF-CONTRATTO"

// funnelContractsQuery returns the purchase contracts with their lines.
const funnelContractsQuery = `SELECT
    t.DO11_DITTA_CG18,
    t.DO11_NUMREG_CO99,
    t.DO11_SEZDOC,
    t.DO11_NUMDOC,
    t.DO11_DATADOC,
    t.DO11_CLIFOR_CG44,
    tot.DO13_TOTDOCUMENTO,
    r.DO30_PROGRIGA,
    r.DO30_INDTIPORIGA,
    r.DO30_CODART_MG66,
    r.DO30_DESCART,
    r.DO30_QTA1,
    r.DO30_PREZZO1,
    r.DO30_IMPNETSCP
FROM dbo.DO11_DOCTESTATA AS t
LEFT JOIN dbo.DO13_DOCTOTALI AS tot
    ON tot.DO13_DITTA_CG18 = t.DO11_DITTA_CG18
   AND tot.DO13_NUMREG_CO99 = t.DO11_NUMREG_CO99
LEFT JOIN dbo.DO30_DOCCORPO AS r
    ON r.DO30_DITTA_CG18 = t.DO11_DITTA_CG18
   AND r.DO30_NUMREG_CO99 = t.DO11_NUMREG_CO99
WHERE t.DO11_DOCUM_MG36 = '` + funnelContractDocCode + `'
  AND t.DO11_DITTA_CG18 = 1
ORDER BY t.DO11_NUMREG_CO99, r.DO30_PROGRIGA;`

// funnelContractLinksQuery returns the body references from a document row to
// a contract row. Contracts share document type 1 with delivery notes, so the
// referenced document is filtered by code.
const funnelContractLinksQuery = `SELECT DISTINCT
    cr.DO33_DITTA_CG18,
    cr.DO33_NUMREG_CO99,
    cr.DO33_NUMREGRIF_CO99
FROM dbo.DO33_DOCCORPORIF AS cr
INNER JOIN dbo.DO11_DOCTESTATA AS c
    ON c.DO11_DITTA_CG18 = cr.DO33_DITTA_CG18
   AND c.DO11_NUMREG_CO99 = cr.DO33_NUMREGRIF_CO99
WHERE cr.DO33_DITTA_CG18 = 1
  AND cr.DO33_INDTIPODOC = 1
  AND c.DO11_DOCUM_MG36 = '` + funnelContractDocCode + `';`

type funnelContract struct {
	key        string
	reg        int64
	series     string
	number     string
	date       *time.Time
	supplierID *int64
	total      *float64
	lines      []funnelDocLine
}

func (c funnelContract) label() string {
	return strings.TrimSpace(c.series + "/" + c.number)
}

// taxable is the fee of the contract: the sum of its amount lines.
func (c funnelContract) taxable() int64 {
	var sum int64
	for _, l := range c.lines {
		if l.net != nil {
			sum += cents(*l.net)
		}
	}
	return sum
}

// category is the article of the first amount line (CROSS-CONNECT,
// CANONE-LEASING-HW...).
func (c funnelContract) category() string {
	for _, l := range c.lines {
		if l.isAmountLine() && l.article != "" {
			return l.article
		}
	}
	return ""
}

func (c funnelContract) description() string {
	for _, l := range c.lines {
		if l.isAmountLine() && l.description != "" {
			return l.description
		}
	}
	return ""
}

// funnelContractIndex holds the contracts per supplier and the invoices AFC
// linked to each contract.
type funnelContractIndex struct {
	bySupplier map[int64][]funnelContract
	byKey      map[string]funnelContract
	// linksByInvoice lists the contract keys an invoice references.
	linksByInvoice map[string][]string
	count          int
}

func (h *Handler) loadFunnelContracts(r *http.Request) ([]funnelContract, error) {
	rows, err := h.alyanteDB.QueryContext(r.Context(), funnelContractsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byKey := map[string]*funnelContract{}
	order := []string{}
	for rows.Next() {
		var ditta, reg int64
		var series, number, article, description sql.NullString
		var date sql.NullTime
		var supplier sql.NullInt64
		var total sql.NullFloat64
		var progressive, rowType sql.NullInt64
		var qty, price, net sql.NullFloat64
		if err := rows.Scan(&ditta, &reg, &series, &number, &date, &supplier, &total,
			&progressive, &rowType, &article, &description, &qty, &price, &net); err != nil {
			return nil, err
		}
		key := strconv.FormatInt(ditta, 10) + ":" + strconv.FormatInt(reg, 10)
		item := byKey[key]
		if item == nil {
			item = &funnelContract{
				key:        key,
				reg:        reg,
				series:     strings.TrimSpace(series.String),
				number:     strings.TrimSpace(number.String),
				date:       timePtr(date),
				supplierID: int64Ptr(supplier),
				total:      float64Ptr(total),
			}
			byKey[key] = item
			order = append(order, key)
		}
		if progressive.Valid {
			item.lines = append(item.lines, funnelDocLine{
				progressive: progressive.Int64,
				rowType:     int64Ptr(rowType),
				article:     strings.TrimSpace(article.String),
				description: strings.TrimSpace(description.String),
				qty:         float64Ptr(qty),
				price:       float64Ptr(price),
				net:         float64Ptr(net),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]funnelContract, 0, len(order))
	for _, key := range order {
		out = append(out, *byKey[key])
	}
	return out, nil
}

func (h *Handler) loadFunnelContractLinks(r *http.Request) (map[string][]string, error) {
	rows, err := h.alyanteDB.QueryContext(r.Context(), funnelContractLinksQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var ditta int64
		var invoiceReg, contractReg sql.NullString
		if err := rows.Scan(&ditta, &invoiceReg, &contractReg); err != nil {
			return nil, err
		}
		prefix := strconv.FormatInt(ditta, 10) + ":"
		invoiceKey := prefix + strings.TrimSpace(invoiceReg.String)
		out[invoiceKey] = append(out[invoiceKey], prefix+strings.TrimSpace(contractReg.String))
	}
	return out, rows.Err()
}

func newFunnelContractIndex(contracts []funnelContract, links map[string][]string) funnelContractIndex {
	idx := funnelContractIndex{
		bySupplier:     map[int64][]funnelContract{},
		byKey:          map[string]funnelContract{},
		linksByInvoice: links,
		count:          len(contracts),
	}
	for _, c := range contracts {
		idx.byKey[c.key] = c
		if c.supplierID != nil {
			idx.bySupplier[*c.supplierID] = append(idx.bySupplier[*c.supplierID], c)
		}
	}
	for id := range idx.bySupplier {
		list := idx.bySupplier[id]
		sort.Slice(list, func(i, j int) bool { return list[i].number < list[j].number })
	}
	return idx
}

func (idx funnelContractIndex) candidates(supplierID *int64) []funnelContract {
	if supplierID == nil {
		return nil
	}
	return idx.bySupplier[*supplierID]
}

// truth returns the labels of the contracts AFC linked to the invoice.
func (idx funnelContractIndex) truth(invoiceKey string) []string {
	out := []string{}
	for _, key := range idx.linksByInvoice[invoiceKey] {
		if c, ok := idx.byKey[key]; ok {
			out = append(out, c.label())
		}
	}
	sort.Strings(out)
	return out
}

// --- matching -------------------------------------------------------------

// contractMaxCandidates bounds the subset search: beyond it, the supplier's
// contracts are too many to combine and the level gives up as ambiguous.
const contractMaxCandidates = 24

type contractResult struct {
	// Candidates is the number of contracts of the supplier at the invoice
	// date.
	Candidates int
	// Proposals holds the labels of the contracts whose fees sum to the
	// invoice, when exactly one combination does.
	Proposals []string
	// Reason tells why the level did not close: no_contracts, no_match
	// (no combination sums to the invoice), ambiguous (more than one).
	Reason string
}

// applyContracts matches the invoice taxable amount against the fees of the
// supplier's contracts dated on or before the invoice: one contract, or a
// combination of them (aggregated invoices), summing exactly to the amount.
func applyContracts(invoice funnelInvoice, contracts []funnelContract) contractResult {
	result := contractResult{Proposals: []string{}}
	if invoice.taxableAmount == nil || invoice.documentDate == nil {
		result.Reason = "no_contracts"
		return result
	}
	active := make([]funnelContract, 0, len(contracts))
	for _, c := range contracts {
		if c.date != nil && c.date.After(*invoice.documentDate) {
			continue
		}
		if c.taxable() <= 0 {
			continue
		}
		active = append(active, c)
	}
	result.Candidates = len(active)
	if len(active) == 0 {
		result.Reason = "no_contracts"
		return result
	}
	target := cents(*invoice.taxableAmount)
	if len(active) > contractMaxCandidates {
		// Too many to combine: accept only a single contract with the exact
		// fee, otherwise ambiguous.
		var hits [][]string
		for _, c := range active {
			if c.taxable() == target {
				hits = append(hits, []string{c.label()})
			}
		}
		return closeContracts(result, hits, "ambiguous")
	}
	hits := contractSubsets(active, target)
	return closeContracts(result, hits, "no_match")
}

func closeContracts(result contractResult, hits [][]string, empty string) contractResult {
	switch len(hits) {
	case 0:
		result.Reason = empty
	case 1:
		result.Proposals = hits[0]
		sort.Strings(result.Proposals)
	default:
		result.Reason = "ambiguous"
	}
	return result
}

// contractSubsets lists the combinations of contracts whose fees sum to
// target, stopping after two (enough to know it is ambiguous).
func contractSubsets(contracts []funnelContract, target int64) [][]string {
	fees := make([]int64, len(contracts))
	for i, c := range contracts {
		fees[i] = c.taxable()
	}
	var hits [][]string
	var walk func(start int, remaining int64, chosen []int)
	walk = func(start int, remaining int64, chosen []int) {
		if len(hits) > 1 {
			return
		}
		if remaining == 0 && len(chosen) > 0 {
			labels := make([]string, 0, len(chosen))
			for _, i := range chosen {
				labels = append(labels, contracts[i].label())
			}
			hits = append(hits, labels)
			return
		}
		for i := start; i < len(contracts); i++ {
			if fees[i] > remaining {
				continue
			}
			walk(i+1, remaining-fees[i], append(chosen, i))
		}
	}
	walk(0, target, nil)
	return hits
}

// --- export ---------------------------------------------------------------

type MatchingFunnelContract struct {
	Registration int64      `json:"registration"`
	Label        string     `json:"label"`
	Date         *time.Time `json:"date"`
	Category     string     `json:"category"`
	Description  string     `json:"description"`
	Fee          float64    `json:"fee"`
	// LinkedInvoices counts the invoices AFC linked to this contract.
	LinkedInvoices int `json:"linked_invoices"`
}

func (idx funnelContractIndex) export(c funnelContract) MatchingFunnelContract {
	linked := 0
	for _, keys := range idx.linksByInvoice {
		for _, k := range keys {
			if k == c.key {
				linked++
			}
		}
	}
	return MatchingFunnelContract{
		Registration:   c.reg,
		Label:          c.label(),
		Date:           c.date,
		Category:       c.category(),
		Description:    c.description(),
		Fee:            float64(c.taxable()) / 100,
		LinkedInvoices: linked,
	}
}

type MatchingFunnelContractsSummary struct {
	ContractCount int `json:"contract_count"`
	// SuppliersWithContracts counts the suppliers in scope having contracts.
	SuppliersWithContracts int `json:"suppliers_with_contracts"`
	// InvoicesLinked counts the invoices in scope AFC linked to a contract.
	InvoicesLinked int `json:"invoices_linked"`
}
