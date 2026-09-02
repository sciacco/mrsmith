package smartpassive

import (
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Alyante purchase orders (document type 22) are created by AFC when an RDA is
// approved, before the supplier invoice arrives, so they are genuine matching
// candidates. TSF-ORDINE orders carry the RDA code in the original document
// number ("PO/50349" or "PA/8093"); BGF-ORD orders use the OF numbering only.
// Orders are read for every year: recurring contracts are invoiced for years
// after the order date, and AFC links 2026 invoices to 2020 orders.

const funnelOrdersQuery = `SELECT
    t.DO11_DITTA_CG18,
    t.DO11_NUMREG_CO99,
    t.DO11_DOCUM_MG36,
    t.DO11_SEZDOC,
    t.DO11_NUMDOC,
    t.DO11_DATADOC,
    t.DO11_CLIFOR_CG44,
    t.DO11_NUMDOCORIG,
    tot.DO13_TOTDOCUMENTO,
    r.DO30_PROGRIGA,
    r.DO30_INDTIPORIGA,
    r.DO30_CODART_MG66,
    r.DO30_DESCART,
    r.DO30_QTA1,
    r.DO30_PREZZO1,
    r.DO30_IMPNETSCP
FROM dbo.DO11_DOCTESTATA AS t
INNER JOIN dbo.MG36_DOCUMENTI AS d
    ON d.MG36_CODDOCUM = t.DO11_DOCUM_MG36
LEFT JOIN dbo.DO13_DOCTOTALI AS tot
    ON tot.DO13_DITTA_CG18 = t.DO11_DITTA_CG18
   AND tot.DO13_NUMREG_CO99 = t.DO11_NUMREG_CO99
LEFT JOIN dbo.DO30_DOCCORPO AS r
    ON r.DO30_DITTA_CG18 = t.DO11_DITTA_CG18
   AND r.DO30_NUMREG_CO99 = t.DO11_NUMREG_CO99
WHERE d.MG36_TIPODOC = 22
  AND d.MG36_INDCLIFOR = 2
  AND t.DO11_DITTA_CG18 = 1
ORDER BY t.DO11_NUMREG_CO99, r.DO30_PROGRIGA;`

// funnelInvoiceLinesQuery returns the body rows of the invoices in scope.
func funnelInvoiceLinesQuery(scope funnelScope) string {
	q := `SELECT
    t.DO11_DITTA_CG18,
    t.DO11_NUMREG_CO99,
    r.DO30_PROGRIGA,
    r.DO30_INDTIPORIGA,
    r.DO30_CODART_MG66,
    r.DO30_DESCART,
    r.DO30_QTA1,
    r.DO30_PREZZO1,
    r.DO30_IMPNETSCP
FROM dbo.DO11_DOCTESTATA AS t
INNER JOIN dbo.MG36_DOCUMENTI AS d
    ON d.MG36_CODDOCUM = t.DO11_DOCUM_MG36
INNER JOIN dbo.DO30_DOCCORPO AS r
    ON r.DO30_DITTA_CG18 = t.DO11_DITTA_CG18
   AND r.DO30_NUMREG_CO99 = t.DO11_NUMREG_CO99
`
	if scope == funnelScopeAll {
		return q + funnelInvoicesWhere + ";"
	}
	return q + funnelOpenBalanceJoin + funnelInvoicesWhere + funnelOpenBalanceWhere + ";"
}

// funnelOrderLineLinksQuery returns every body reference from a document row
// to a purchase order row (type 22), with the quantity it consumed. Read for
// every year: the consumed quantities decide which order rows still have a
// residual, and AFC links 2026 invoices to rows of 2020 orders.
const funnelOrderLineLinksQuery = `SELECT
    cr.DO33_DITTA_CG18,
    cr.DO33_NUMREG_CO99,
    cr.DO33_PROGRIGA,
    cr.DO33_NUMREGRIF_CO99,
    cr.DO33_PROGRIGARIF_DO30,
    cr.DO33_QTA1MOV
FROM dbo.DO33_DOCCORPORIF AS cr
WHERE cr.DO33_DITTA_CG18 = 1
  AND cr.DO33_INDTIPODOC = 22
  AND cr.DO33_NUMREGRIF_CO99 IS NOT NULL;`

// orderLineKey identifies one order row: order key plus row progressive.
type orderLineKey struct {
	order string
	row   int64
}

// orderLineLink is one invoice row consuming quantity from an order row.
type orderLineLink struct {
	invoiceRow int64
	line       orderLineKey
	qty        float64
}

// funnelOrderLinks holds the consumed quantity per order row and, per invoice,
// the rows it consumed. Both come from the links AFC made in Alyante.
type funnelOrderLinks struct {
	consumed  map[orderLineKey]float64
	byInvoice map[string][]orderLineLink
}

func (l funnelOrderLinks) ordersOf(invoiceKey string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, link := range l.byInvoice[invoiceKey] {
		if _, ok := seen[link.line.order]; ok {
			continue
		}
		seen[link.line.order] = struct{}{}
		out = append(out, link.line.order)
	}
	sort.Strings(out)
	return out
}

// own returns the quantity this invoice itself consumed per order row, so a
// reconciled invoice can be re-evaluated as if it were not yet linked.
func (l funnelOrderLinks) own(invoiceKey string) map[orderLineKey]float64 {
	out := map[orderLineKey]float64{}
	for _, link := range l.byInvoice[invoiceKey] {
		out[link.line] += link.qty
	}
	return out
}

type funnelDocLine struct {
	progressive int64
	rowType     *int64
	article     string
	description string
	qty         *float64
	price       *float64
	net         *float64
}

// isAmountLine tells apart real lines from descriptive rows (type 2, no
// article, zero amount) such as the "Ordine Fo. num." rows Alyante writes.
func (l funnelDocLine) isAmountLine() bool {
	return l.net != nil && cents(*l.net) != 0
}

type funnelOrder struct {
	key        string
	reg        int64
	docCode    string
	series     string
	number     string
	date       *time.Time
	supplierID *int64
	origRef    string
	total      *float64
	lines      []funnelDocLine
	// rdaCode is the PO or PA code read from origRef, when present.
	rdaCode   *noteCode
	rdaLegacy bool
}

func (o funnelOrder) label() string {
	if o.origRef != "" {
		return o.origRef
	}
	return strings.TrimSpace(o.series + "/" + o.number)
}

func (o funnelOrder) taxable() int64 {
	var sum int64
	for _, l := range o.lines {
		if l.net != nil {
			sum += cents(*l.net)
		}
	}
	return sum
}

func (h *Handler) loadFunnelOrders(r *http.Request) ([]funnelOrder, error) {
	rows, err := h.alyanteDB.QueryContext(r.Context(), funnelOrdersQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byKey := make(map[string]*funnelOrder)
	order := make([]string, 0)
	for rows.Next() {
		var ditta, reg int64
		var docCode, series, number, origRef, article, description sql.NullString
		var date sql.NullTime
		var supplier sql.NullInt64
		var total sql.NullFloat64
		var progressive, rowType sql.NullInt64
		var qty, price, net sql.NullFloat64
		if err := rows.Scan(&ditta, &reg, &docCode, &series, &number, &date, &supplier, &origRef, &total,
			&progressive, &rowType, &article, &description, &qty, &price, &net); err != nil {
			return nil, err
		}
		key := strconv.FormatInt(ditta, 10) + ":" + strconv.FormatInt(reg, 10)
		item := byKey[key]
		if item == nil {
			item = &funnelOrder{
				key:        key,
				reg:        reg,
				docCode:    strings.TrimSpace(docCode.String),
				series:     strings.TrimSpace(series.String),
				number:     strings.TrimSpace(number.String),
				date:       timePtr(date),
				supplierID: int64Ptr(supplier),
				origRef:    strings.TrimSpace(origRef.String),
				total:      float64Ptr(total),
			}
			item.rdaCode, item.rdaLegacy = orderRDACode(item.origRef)
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
	out := make([]funnelOrder, 0, len(order))
	for _, key := range order {
		out = append(out, *byKey[key])
	}
	return out, nil
}

// orderRDACode reads the RDA code AFC wrote in the order's original number.
// Returns the code and whether it is a legacy PA code.
func orderRDACode(origRef string) (*noteCode, bool) {
	note := parseAFCNote(origRef)
	if len(note.arakCodes) > 0 {
		c := note.arakCodes[0]
		return &c, false
	}
	if len(note.legacyCodes) > 0 {
		c := note.legacyCodes[0]
		return &c, true
	}
	return nil, false
}

func (h *Handler) loadFunnelInvoiceLines(r *http.Request, scope funnelScope) (map[string][]funnelDocLine, error) {
	rows, err := h.alyanteDB.QueryContext(r.Context(), funnelInvoiceLinesQuery(scope))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]funnelDocLine)
	for rows.Next() {
		var ditta, reg int64
		var progressive, rowType sql.NullInt64
		var article, description sql.NullString
		var qty, price, net sql.NullFloat64
		if err := rows.Scan(&ditta, &reg, &progressive, &rowType, &article, &description, &qty, &price, &net); err != nil {
			return nil, err
		}
		key := strconv.FormatInt(ditta, 10) + ":" + strconv.FormatInt(reg, 10)
		out[key] = append(out[key], funnelDocLine{
			progressive: progressive.Int64,
			rowType:     int64Ptr(rowType),
			article:     strings.TrimSpace(article.String),
			description: strings.TrimSpace(description.String),
			qty:         float64Ptr(qty),
			price:       float64Ptr(price),
			net:         float64Ptr(net),
		})
	}
	for key := range out {
		lines := out[key]
		sort.Slice(lines, func(i, j int) bool { return lines[i].progressive < lines[j].progressive })
	}
	return out, rows.Err()
}

func (h *Handler) loadFunnelOrderLinks(r *http.Request) (funnelOrderLinks, error) {
	links := funnelOrderLinks{consumed: map[orderLineKey]float64{}, byInvoice: map[string][]orderLineLink{}}
	rows, err := h.alyanteDB.QueryContext(r.Context(), funnelOrderLineLinksQuery)
	if err != nil {
		return links, err
	}
	defer rows.Close()

	for rows.Next() {
		var ditta int64
		var invoiceReg, orderReg sql.NullString
		var invoiceRow, orderRow sql.NullInt64
		var qty sql.NullFloat64
		if err := rows.Scan(&ditta, &invoiceReg, &invoiceRow, &orderReg, &orderRow, &qty); err != nil {
			return links, err
		}
		orderRef := strings.TrimSpace(orderReg.String)
		if orderRef == "" {
			continue
		}
		prefix := strconv.FormatInt(ditta, 10) + ":"
		key := orderLineKey{order: prefix + orderRef, row: orderRow.Int64}
		link := orderLineLink{invoiceRow: invoiceRow.Int64, line: key, qty: qty.Float64}
		links.consumed[key] += link.qty
		invoiceKey := prefix + strings.TrimSpace(invoiceReg.String)
		links.byInvoice[invoiceKey] = append(links.byInvoice[invoiceKey], link)
	}
	return links, rows.Err()
}

// --- analysis -------------------------------------------------------------

type MatchingFunnelOrdersSummary struct {
	OrderCount        int            `json:"order_count"`
	ByDocCode         map[string]int `json:"by_doc_code"`
	WithRDACode       int            `json:"with_rda_code"`
	RDAResolved       int            `json:"rda_resolved"`
	RDAUnresolved     int            `json:"rda_unresolved"`
	WithLegacyCode    int            `json:"with_legacy_code"`
	WithoutCode       int            `json:"without_code"`
	SupplierMismatch  int            `json:"supplier_mismatch"`
	OrdersWithoutERP  int            `json:"orders_without_supplier"`
	OpenOrders        int            `json:"open_orders"`
	ClosedOrders      int            `json:"closed_orders"`
	OverConsumed      int            `json:"over_consumed_orders"`
	OrdersNoLines     int            `json:"orders_without_lines"`
	InvoicesNoOrder   int            `json:"invoices_no_candidates"`
	InvoicesOneOrder  int            `json:"invoices_one_candidate"`
	InvoicesManyOrder int            `json:"invoices_multiple_candidates"`
	InvoicesNoOpen    int            `json:"invoices_no_open_candidates"`
	InvoicesOneOpen   int            `json:"invoices_one_open_candidate"`
	InvoicesManyOpen  int            `json:"invoices_multiple_open_candidates"`
	InvoicesLinked    int            `json:"invoices_with_afc_link"`
}

type MatchingFunnelOrder struct {
	Registration int64      `json:"registration"`
	DocCode      string     `json:"doc_code"`
	Label        string     `json:"label"`
	Date         *time.Time `json:"date"`
	RDACode      string     `json:"rda_code"`
	RDAID        *int64     `json:"rda_id"`
	Total        *float64   `json:"total"`
	Taxable      float64    `json:"taxable"`
	LineCount    int        `json:"line_count"`
	// Residual is the ordered quantity not yet consumed by linked invoices,
	// summed over the amount lines; Open is true when any line has residual.
	Residual float64 `json:"residual_qty"`
	Open     bool    `json:"open"`
}

type MatchingFunnelOrderRuleResult struct {
	// Rule: "" none, header, lines.
	Rule      string     `json:"rule"`
	Proposals [][]string `json:"proposals"`
	// Verdict vs the AFC link in Alyante: match, ambiguous, wrong, none,
	// unlinked_proposal, unlinked_silent.
	Verdict      string   `json:"verdict"`
	AFCLinks     []string `json:"afc_links"`
	LinesTotal   int      `json:"lines_total"`
	LinesMatched int      `json:"lines_matched"`
	// AmbiguousLines counts invoice lines that fit more than one order.
	AmbiguousLines int `json:"ambiguous_lines"`
	// OpenCandidates is the number of supplier orders with residual when the
	// invoice is evaluated.
	OpenCandidates int `json:"open_candidates"`
}

type MatchingFunnelOrderRulesSummary struct {
	LinkedInvoices   int `json:"linked_invoices"`
	Match            int `json:"match"`
	Ambiguous        int `json:"ambiguous"`
	Wrong            int `json:"wrong"`
	None             int `json:"none"`
	MatchByHeader    int `json:"match_by_header"`
	MatchByLines     int `json:"match_by_lines"`
	UnlinkedInvoices int `json:"unlinked_invoices"`
	UnlinkedProposal int `json:"unlinked_proposal"`
}

type funnelOrderIndex struct {
	bySupplier map[int64][]funnelOrder
	byKey      map[string]funnelOrder
	rdaByOrder map[string]*int64
	// byArakNumber and byLegacyNumber index orders by the RDA or PA number
	// written in their original document number.
	byArakNumber   map[string][]funnelOrder
	byLegacyNumber map[string][]funnelOrder
	links          funnelOrderLinks
	summary        MatchingFunnelOrdersSummary
}

// byCodeNumber returns the orders whose original number carries the given RDA
// (or legacy PA) number.
func (idx funnelOrderIndex) byCodeNumber(number string, legacy bool) []funnelOrder {
	if legacy {
		return idx.byLegacyNumber[number]
	}
	return idx.byArakNumber[number]
}

const qtyEpsilon = 0.0005

// residual returns the quantity of an order row still to be invoiced, adding
// back what the invoice under evaluation already consumed (own).
func (idx funnelOrderIndex) residual(o funnelOrder, l funnelDocLine, own map[orderLineKey]float64) *float64 {
	if l.qty == nil {
		return nil
	}
	key := orderLineKey{order: o.key, row: l.progressive}
	r := *l.qty - idx.links.consumed[key] + own[key]
	return &r
}

// isOpen tells whether any amount line of the order still has residual.
// Orders without amount lines cannot be judged and count as open.
func (idx funnelOrderIndex) isOpen(o funnelOrder, own map[orderLineKey]float64) bool {
	amountLines := 0
	for _, l := range o.lines {
		if !l.isAmountLine() {
			continue
		}
		amountLines++
		if r := idx.residual(o, l, own); r == nil || *r > qtyEpsilon {
			return true
		}
	}
	return amountLines == 0
}

func newFunnelOrderIndex(orders []funnelOrder, refs funnelReferenceIndex, links funnelOrderLinks) funnelOrderIndex {
	idx := funnelOrderIndex{
		bySupplier:     make(map[int64][]funnelOrder),
		byKey:          make(map[string]funnelOrder, len(orders)),
		rdaByOrder:     make(map[string]*int64, len(orders)),
		byArakNumber:   make(map[string][]funnelOrder),
		byLegacyNumber: make(map[string][]funnelOrder),
		links:          links,
		summary:        MatchingFunnelOrdersSummary{OrderCount: len(orders), ByDocCode: make(map[string]int)},
	}
	for _, o := range orders {
		idx.byKey[o.key] = o
		if o.rdaCode != nil {
			if o.rdaLegacy {
				idx.byLegacyNumber[o.rdaCode.number] = append(idx.byLegacyNumber[o.rdaCode.number], o)
			} else {
				idx.byArakNumber[o.rdaCode.number] = append(idx.byArakNumber[o.rdaCode.number], o)
			}
		}
		idx.summary.ByDocCode[o.docCode]++
		amountLines := 0
		for _, l := range o.lines {
			if l.isAmountLine() {
				amountLines++
			}
		}
		switch {
		case amountLines == 0:
			idx.summary.OrdersNoLines++
		case idx.isOpen(o, nil):
			idx.summary.OpenOrders++
		default:
			idx.summary.ClosedOrders++
		}
		for _, l := range o.lines {
			if r := idx.residual(o, l, nil); l.isAmountLine() && r != nil && *r < -qtyEpsilon {
				idx.summary.OverConsumed++
				break
			}
		}
		if o.supplierID == nil {
			idx.summary.OrdersWithoutERP++
		} else {
			idx.bySupplier[*o.supplierID] = append(idx.bySupplier[*o.supplierID], o)
		}
		switch {
		case o.rdaCode == nil:
			idx.summary.WithoutCode++
		case o.rdaLegacy:
			idx.summary.WithLegacyCode++
		default:
			idx.summary.WithRDACode++
			matches := refs.lookup(*o.rdaCode)
			if len(matches) == 1 {
				idx.summary.RDAResolved++
				id := matches[0].id
				idx.rdaByOrder[o.key] = &id
				if matches[0].supplierID == nil || o.supplierID == nil || *matches[0].supplierID != *o.supplierID {
					idx.summary.SupplierMismatch++
				}
			} else {
				idx.summary.RDAUnresolved++
			}
		}
	}
	return idx
}

// hasOrderFor reports whether an Alyante order carries the RDA code in its
// original document number.
func (idx funnelOrderIndex) hasOrderFor(rda funnelRDA) bool {
	m := rdaCodeShape.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(rda.code)))
	if m == nil {
		return false
	}
	return len(idx.byArakNumber[m[1]]) > 0
}

func (idx funnelOrderIndex) candidates(supplierID *int64) []funnelOrder {
	if supplierID == nil {
		return nil
	}
	return idx.bySupplier[*supplierID]
}

// openCandidates keeps the orders of the supplier that still have residual,
// judged as if the invoice under evaluation were not linked yet.
func (idx funnelOrderIndex) openCandidates(supplierID *int64, own map[orderLineKey]float64) []funnelOrder {
	all := idx.candidates(supplierID)
	out := make([]funnelOrder, 0, len(all))
	for _, o := range all {
		if idx.isOpen(o, own) {
			out = append(out, o)
		}
	}
	return out
}

func (idx funnelOrderIndex) export(o funnelOrder) MatchingFunnelOrder {
	code := ""
	if o.rdaCode != nil {
		if o.rdaLegacy {
			code = o.rdaCode.legacyCode()
		} else {
			code = o.rdaCode.arakCode()
		}
	}
	amountLines := 0
	residual := 0.0
	for _, l := range o.lines {
		if !l.isAmountLine() {
			continue
		}
		amountLines++
		if r := idx.residual(o, l, nil); r != nil && *r > 0 {
			residual += *r
		}
	}
	return MatchingFunnelOrder{
		Registration: o.reg,
		DocCode:      o.docCode,
		Label:        o.label(),
		Date:         o.date,
		RDACode:      code,
		RDAID:        idx.rdaByOrder[o.key],
		Total:        o.total,
		Taxable:      float64(o.taxable()) / 100,
		LineCount:    amountLines,
		Residual:     residual,
		Open:         idx.isOpen(o, nil),
	}
}

// applyOrderRules proposes Alyante orders for an invoice: first the order whose
// taxable total equals the invoice's, then line by line (same article and same
// net amount, or same article and same unit price).
func applyOrderRules(invoice funnelInvoice, lines []funnelDocLine, idx funnelOrderIndex) MatchingFunnelOrderRuleResult {
	result := MatchingFunnelOrderRuleResult{Proposals: [][]string{}, AFCLinks: []string{}}
	own := idx.links.own(invoice.key)
	links := idx.links.ordersOf(invoice.key)
	candidates := idx.openCandidates(invoice.supplierID, own)
	result.OpenCandidates = len(candidates)
	for _, link := range links {
		if o, ok := idx.byKey[link]; ok {
			result.AFCLinks = append(result.AFCLinks, o.label())
		} else {
			result.AFCLinks = append(result.AFCLinks, "reg. "+strings.TrimPrefix(link, "1:"))
		}
	}
	sort.Strings(result.AFCLinks)

	unique := true
	proposalKeys := [][]string{}
	for _, l := range lines {
		if l.isAmountLine() {
			result.LinesTotal++
		}
	}
	if len(lines) > 0 && len(candidates) > 0 {
		union, matched, ambiguous := lineProposal(lines, candidates, own, idx)
		result.LinesMatched = matched
		result.AmbiguousLines = ambiguous
		if len(union) > 0 {
			result.Rule = "lines"
			unique = ambiguous == 0
			proposalKeys = [][]string{union}
		}
	}
	if result.Rule == "" && invoice.taxableAmount != nil && len(candidates) > 0 {
		amount := cents(*invoice.taxableAmount)
		if amount != 0 {
			for _, o := range candidates {
				if o.taxable() == amount {
					proposalKeys = append(proposalKeys, []string{o.key})
				}
			}
			if len(proposalKeys) > 0 {
				result.Rule = "header"
				unique = len(proposalKeys) == 1
			}
		}
	}

	for _, keys := range proposalKeys {
		labels := make([]string, 0, len(keys))
		for _, k := range keys {
			labels = append(labels, idx.byKey[k].label())
		}
		sort.Strings(labels)
		result.Proposals = append(result.Proposals, labels)
	}
	result.Verdict = orderVerdict(proposalKeys, unique, links)
	return result
}

// lineProposal matches every amount line of the invoice against the lines of
// the candidate orders. It returns the union of the orders that fit some line,
// the number of lines matched and the number of lines that fit more than one
// order. An empty union means at least one line fits no order.
func lineProposal(lines []funnelDocLine, candidates []funnelOrder, own map[orderLineKey]float64, idx funnelOrderIndex) ([]string, int, int) {
	matched, ambiguous := 0, 0
	union := map[string]struct{}{}
	for _, l := range lines {
		if !l.isAmountLine() {
			continue
		}
		found := map[string]struct{}{}
		for _, o := range candidates {
			if orderHasLine(o, l, own, idx) {
				found[o.key] = struct{}{}
			}
		}
		if len(found) == 0 {
			return nil, matched, ambiguous
		}
		matched++
		if len(found) > 1 {
			ambiguous++
		}
		for k := range found {
			union[k] = struct{}{}
		}
	}
	if matched == 0 {
		return nil, 0, 0
	}
	keys := make([]string, 0, len(union))
	for k := range union {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, matched, ambiguous
}

// orderHasLine tells whether an order row can absorb the invoice row: same
// article (and description for free-text rows), same net amount or same unit
// price, and enough residual quantity.
func orderHasLine(o funnelOrder, l funnelDocLine, own map[orderLineKey]float64, idx funnelOrderIndex) bool {
	article := strings.ToUpper(l.article)
	for _, ol := range o.lines {
		if !ol.isAmountLine() || strings.ToUpper(ol.article) != article {
			continue
		}
		if r := idx.residual(o, ol, own); r != nil && l.qty != nil && *l.qty > *r+qtyEpsilon {
			continue
		}
		if article == "" {
			// Free-text lines: compare the description as well.
			if !strings.EqualFold(ol.description, l.description) {
				continue
			}
		}
		if cents(*ol.net) == cents(*l.net) {
			return true
		}
		if ol.price != nil && l.price != nil && cents(*ol.price) == cents(*l.price) && cents(*ol.price) != 0 {
			return true
		}
	}
	return false
}

// orderVerdict scores the proposal against the orders AFC linked in Alyante.
// A unique proposal must equal the linked set; a non-unique one (several
// header hits, or lines fitting several orders) is "ambiguous" when it contains
// every linked order and "wrong" otherwise.
func orderVerdict(proposals [][]string, unique bool, links []string) string {
	if len(links) == 0 {
		if len(proposals) > 0 {
			return "unlinked_proposal"
		}
		return "unlinked_silent"
	}
	if len(proposals) == 0 {
		return "none"
	}
	truth := append([]string(nil), links...)
	sort.Strings(truth)
	union := map[string]struct{}{}
	for _, p := range proposals {
		for _, k := range p {
			union[k] = struct{}{}
		}
	}
	covered := true
	for _, t := range truth {
		if _, ok := union[t]; !ok {
			covered = false
		}
	}
	if unique {
		if len(proposals) == 1 && strings.Join(proposals[0], "|") == strings.Join(truth, "|") {
			return "match"
		}
		return "wrong"
	}
	if covered {
		return "ambiguous"
	}
	return "wrong"
}

func addOrderRuleResult(summary *MatchingFunnelOrderRulesSummary, result MatchingFunnelOrderRuleResult) {
	switch result.Verdict {
	case "match":
		summary.LinkedInvoices++
		summary.Match++
		if result.Rule == "header" {
			summary.MatchByHeader++
		} else {
			summary.MatchByLines++
		}
	case "ambiguous":
		summary.LinkedInvoices++
		summary.Ambiguous++
	case "wrong":
		summary.LinkedInvoices++
		summary.Wrong++
	case "none":
		summary.LinkedInvoices++
		summary.None++
	case "unlinked_proposal":
		summary.UnlinkedInvoices++
		summary.UnlinkedProposal++
	default:
		summary.UnlinkedInvoices++
	}
}
