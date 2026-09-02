package smartpassive

import (
	"database/sql"
	"encoding/xml"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Electronic invoices received from SDI are kept whole in
// smartpassive.sdi_invoice (Anisetta). They carry what Alyante does not import:
// the order reference the supplier wrote, per line; the competence period per
// line; the supplier's own article codes. Here they are linked to the Alyante
// registration by supplier VAT (or fiscal code), document number and date, and
// the declared order references are resolved against Alyante orders and RDAs.

// Header keys of the documents whose number matches an invoice in scope.
// Alyante keeps only the last 15 characters of the supplier's document
// number ("A89020261000054401" becomes "020261000054401"), so a document also
// matches when its number ends with the registered reference ($2 carries the
// references long enough to be a truncation).
const funnelSDIHeadersQuery = `SELECT
    source_id,
    supplier_vat,
    supplier_name,
    document_type,
    document_number,
    document_date
FROM smartpassive.sdi_invoice
WHERE document_number = ANY($1)
   OR EXISTS (
        SELECT 1
        FROM unnest($2::text[]) AS s(n)
        WHERE right(upper(document_number), length(s.n)) = upper(s.n)
   )`

// sdiSuffixMinLength is the shortest registered reference tried as a suffix
// of the SDI document number: shorter ones are too likely to end another
// supplier's number by chance.
const sdiSuffixMinLength = 10

// XML of the documents not yet parsed in this process. Attachments (base64
// PDFs) are stripped in SQL: they weigh most of the XML and carry nothing for
// the matching.
const funnelSDIXMLQuery = `SELECT
    source_id,
    regexp_replace(xml, '<Allegati>.*?</Allegati>', '', 'g')
FROM smartpassive.sdi_invoice
WHERE source_id = ANY($1)`

type sdiOrderRef struct {
	ID    string
	Date  string
	Lines []string
}

type sdiLine struct {
	Number      string
	Articles    []string
	Description string
	Qty         *float64
	UnitPrice   *float64
	Total       *float64
	PeriodStart string
	PeriodEnd   string
}

type sdiDocument struct {
	sourceID     int64
	supplierVAT  string
	supplierName string
	docType      string
	number       string
	date         *time.Time
	orderRefs    []sdiOrderRef
	contracts    []string
	lines        []sdiLine
}

// fatturaPA mirrors the parts of the FatturaPA schema used here. Tags without
// namespace match every schema version (FPR12, FPA12, FSM10).
type fatturaPA struct {
	Body []struct {
		Generali struct {
			Ordini []struct {
				Lines []string `xml:"RiferimentoNumeroLinea"`
				ID    string   `xml:"IdDocumento"`
				Date  string   `xml:"Data"`
			} `xml:"DatiOrdineAcquisto"`
			Contratti []struct {
				ID string `xml:"IdDocumento"`
			} `xml:"DatiContratto"`
		} `xml:"DatiGenerali"`
		Beni struct {
			Linee []struct {
				Number   string `xml:"NumeroLinea"`
				Articoli []struct {
					Value string `xml:"CodiceValore"`
				} `xml:"CodiceArticolo"`
				Description string `xml:"Descrizione"`
				Qty         string `xml:"Quantita"`
				UnitPrice   string `xml:"PrezzoUnitario"`
				Total       string `xml:"PrezzoTotale"`
				PeriodStart string `xml:"DataInizioPeriodo"`
				PeriodEnd   string `xml:"DataFinePeriodo"`
			} `xml:"DettaglioLinee"`
		} `xml:"DatiBeniServizi"`
	} `xml:"FatturaElettronicaBody"`
}

// sdiParseCache keeps parsed documents per process: the stored XML never
// changes, and parsing thousands of files per request would be wasteful.
var sdiParseCache sync.Map // source_id -> *sdiDocument

func parseSDIDocument(raw string) (orderRefs []sdiOrderRef, contracts []string, lines []sdiLine, err error) {
	var doc fatturaPA
	if err := xml.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, nil, nil, err
	}
	for _, body := range doc.Body {
		for _, o := range body.Generali.Ordini {
			orderRefs = append(orderRefs, sdiOrderRef{ID: strings.TrimSpace(o.ID), Date: strings.TrimSpace(o.Date), Lines: trimAll(o.Lines)})
		}
		for _, c := range body.Generali.Contratti {
			if id := strings.TrimSpace(c.ID); id != "" {
				contracts = append(contracts, id)
			}
		}
		for _, l := range body.Beni.Linee {
			line := sdiLine{
				Number:      strings.TrimSpace(l.Number),
				Description: strings.TrimSpace(l.Description),
				Qty:         parseFloatPtr(l.Qty),
				UnitPrice:   parseFloatPtr(l.UnitPrice),
				Total:       parseFloatPtr(l.Total),
				PeriodStart: strings.TrimSpace(l.PeriodStart),
				PeriodEnd:   strings.TrimSpace(l.PeriodEnd),
			}
			for _, a := range l.Articoli {
				if v := strings.TrimSpace(a.Value); v != "" {
					line.Articles = append(line.Articles, v)
				}
			}
			lines = append(lines, line)
		}
	}
	return orderRefs, contracts, lines, nil
}

func trimAll(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func parseFloatPtr(raw string) *float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return nil
	}
	return &v
}

// loadFunnelSDI reads the SDI documents whose number matches one of the
// invoices in scope. Matching on VAT and date happens in Go.
func (h *Handler) loadFunnelSDI(r *http.Request, invoices []funnelInvoice) (map[string][]*sdiDocument, error) {
	out := make(map[string][]*sdiDocument)
	if h.anisettaDB == nil {
		return out, nil
	}
	numbers := make([]string, 0, len(invoices))
	suffixes := make([]string, 0, len(invoices))
	requested := map[string]struct{}{}
	for _, inv := range invoices {
		n := normalizeDocNumber(inv.supplierReference)
		if n == "" {
			continue
		}
		if _, ok := requested[n]; ok {
			continue
		}
		requested[n] = struct{}{}
		numbers = append(numbers, strings.TrimSpace(inv.supplierReference))
		if len(n) >= sdiSuffixMinLength {
			suffixes = append(suffixes, n)
		}
	}
	if len(numbers) == 0 {
		return out, nil
	}

	rows, err := h.anisettaDB.QueryContext(r.Context(), funnelSDIHeadersQuery, numbers, suffixes)
	if err != nil {
		return nil, err
	}
	headers := make([]*sdiDocument, 0)
	missing := make([]int64, 0)
	for rows.Next() {
		var sourceID int64
		var vat, name, docType, number sql.NullString
		var date sql.NullTime
		if err := rows.Scan(&sourceID, &vat, &name, &docType, &number, &date); err != nil {
			rows.Close()
			return nil, err
		}
		if cached, ok := sdiParseCache.Load(sourceID); ok {
			headers = append(headers, cached.(*sdiDocument))
			continue
		}
		headers = append(headers, &sdiDocument{
			sourceID:     sourceID,
			supplierVAT:  strings.ToUpper(strings.TrimSpace(vat.String)),
			supplierName: strings.TrimSpace(name.String),
			docType:      strings.TrimSpace(docType.String),
			number:       strings.TrimSpace(number.String),
			date:         timePtr(date),
		})
		missing = append(missing, sourceID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	if len(missing) > 0 {
		parsed, err := h.parseSDIBatch(r, missing)
		if err != nil {
			return nil, err
		}
		for _, doc := range headers {
			if body, ok := parsed[doc.sourceID]; ok {
				doc.orderRefs, doc.contracts, doc.lines = body.orderRefs, body.contracts, body.lines
				sdiParseCache.Store(doc.sourceID, doc)
			}
		}
	}
	for _, doc := range headers {
		key := normalizeDocNumber(doc.number)
		if _, ok := requested[key]; ok {
			out[key] = append(out[key], doc)
		}
		// The registered reference may be a suffix of the full number.
		for l := len(key) - 1; l >= sdiSuffixMinLength; l-- {
			suffix := key[len(key)-l:]
			if _, ok := requested[suffix]; ok {
				out[suffix] = append(out[suffix], doc)
			}
		}
	}
	return out, nil
}

// parseSDIBatch reads and parses the XML of the given documents.
func (h *Handler) parseSDIBatch(r *http.Request, ids []int64) (map[int64]sdiDocument, error) {
	out := make(map[int64]sdiDocument, len(ids))
	rows, err := h.anisettaDB.QueryContext(r.Context(), funnelSDIXMLQuery, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sourceID int64
		var raw string
		if err := rows.Scan(&sourceID, &raw); err != nil {
			return nil, err
		}
		refs, contracts, lines, err := parseSDIDocument(raw)
		if err != nil {
			continue
		}
		out[sourceID] = sdiDocument{orderRefs: refs, contracts: contracts, lines: lines}
	}
	return out, rows.Err()
}

func normalizeDocNumber(n string) string {
	return strings.ToUpper(strings.TrimSpace(n))
}

// fiscalKeys returns the comparable forms of the Alyante supplier's VAT and
// fiscal code: digits and letters only, without the two-letter country prefix.
func fiscalKeys(values ...string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, v := range values {
		k := fiscalKey(v)
		if k != "" {
			out[k] = struct{}{}
		}
	}
	return out
}

func fiscalKey(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	v = strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') {
			return r
		}
		return -1
	}, v)
	if len(v) > 2 && v[0] >= 'A' && v[0] <= 'Z' && v[1] >= 'A' && v[1] <= 'Z' {
		v = v[2:]
	}
	return v
}

// --- analysis -------------------------------------------------------------

type MatchingFunnelSDISummary struct {
	InvoicesLinked       int `json:"invoices_linked"`
	InvoicesNotLinked    int `json:"invoices_not_linked"`
	LinkedByNumberOnly   int `json:"linked_number_only"`
	LinkedBySuffix       int `json:"linked_suffix"`
	WithOrderRef         int `json:"with_order_ref"`
	WithUsableCode       int `json:"with_usable_code"`
	WithContractRef      int `json:"with_contract_ref"`
	WithPeriod           int `json:"with_period"`
	WithArticleCodes     int `json:"with_article_codes"`
	ResolvedToOrder      int `json:"resolved_to_order"`
	ResolvedToRDA        int `json:"resolved_to_rda"`
	UnresolvedCode       int `json:"unresolved_code"`
	LinkedInvoices       int `json:"afc_linked_invoices"`
	Match                int `json:"match"`
	Partial              int `json:"partial"`
	Wrong                int `json:"wrong"`
	None                 int `json:"none"`
	AmbiguousResolvedNow int `json:"order_rule_ambiguous_resolved"`
}

type MatchingFunnelSDIRef struct {
	Declared string   `json:"declared"`
	Code     string   `json:"code"`
	Lines    []string `json:"lines"`
	Orders   []string `json:"orders"`
	RDAID    *int64   `json:"rda_id"`
	RDACode  string   `json:"rda_code"`
	// orderKeys are the Alyante keys of Orders, for the cascade checks.
	orderKeys []string
}

type MatchingFunnelSDIResult struct {
	Linked      bool                   `json:"linked"`
	LinkKind    string                 `json:"link_kind"`
	SourceID    *int64                 `json:"source_id"`
	OrderRefs   []MatchingFunnelSDIRef `json:"order_refs"`
	Contracts   []string               `json:"contracts"`
	PeriodStart string                 `json:"period_start"`
	PeriodEnd   string                 `json:"period_end"`
	Articles    []string               `json:"articles"`
	// Verdict vs the AFC order links: match, partial, wrong, none, no_truth.
	Verdict string `json:"verdict"`
}

// applySDI links the invoice to its SDI document and resolves the declared
// order references to Alyante orders (by the RDA code in their original
// number) and to Arak RDAs.
func applySDI(invoice funnelInvoice, docs []*sdiDocument, orders funnelOrderIndex, refs funnelReferenceIndex) MatchingFunnelSDIResult {
	result := MatchingFunnelSDIResult{OrderRefs: []MatchingFunnelSDIRef{}, Contracts: []string{}, Articles: []string{}, Verdict: "no_truth"}
	keys := fiscalKeys(invoice.supplierVAT, invoice.supplierVATForeign, invoice.fiscalCode)

	// Same number, supplier and date first; then same number and date; then a
	// number ending with the reference Alyante truncated, which needs the
	// supplier too.
	var doc *sdiDocument
	number := normalizeDocNumber(invoice.supplierReference)
	for _, d := range docs {
		sameDate := d.date != nil && invoice.documentDate != nil && d.date.Format("2006-01-02") == invoice.documentDate.Format("2006-01-02")
		_, sameSupplier := keys[fiscalKey(d.supplierVAT)]
		sameNumber := normalizeDocNumber(d.number) == number
		switch {
		case sameDate && sameSupplier && sameNumber:
			doc = d
			result.LinkKind = "vat_number_date"
		case sameDate && sameNumber && (doc == nil || result.LinkKind == "vat_suffix_date"):
			doc = d
			result.LinkKind = "number_date"
		case sameDate && sameSupplier && doc == nil:
			doc = d
			result.LinkKind = "vat_suffix_date"
		}
		if result.LinkKind == "vat_number_date" {
			break
		}
	}
	if doc == nil {
		return result
	}
	result.Linked = true
	id := doc.sourceID
	result.SourceID = &id
	result.Contracts = append(result.Contracts, doc.contracts...)

	articles := map[string]struct{}{}
	for _, l := range doc.lines {
		for _, a := range l.Articles {
			articles[a] = struct{}{}
		}
		if l.PeriodStart != "" && (result.PeriodStart == "" || l.PeriodStart < result.PeriodStart) {
			result.PeriodStart = l.PeriodStart
		}
		if l.PeriodEnd != "" && l.PeriodEnd > result.PeriodEnd {
			result.PeriodEnd = l.PeriodEnd
		}
	}
	for a := range articles {
		result.Articles = append(result.Articles, a)
	}
	sort.Strings(result.Articles)

	proposed := map[string]struct{}{}
	own := orders.links.own(invoice.key)
	for _, ref := range doc.orderRefs {
		item := MatchingFunnelSDIRef{Declared: ref.ID, Lines: ref.Lines, Orders: []string{}}
		// A declared reference may chain several codes ("PO-50205-50206-50207",
		// "PA/8095-8096"): every code counts.
		note := parseAFCNote(ref.ID)
		codes := []string{}
		for _, code := range note.arakCodes {
			codes = append(codes, code.arakCode())
			if matches := refs.lookup(code); len(matches) == 1 && item.RDAID == nil {
				rdaID := matches[0].id
				item.RDAID = &rdaID
				item.RDACode = strings.ToUpper(matches[0].code)
			}
			for _, o := range orders.byCodeNumber(code.number, false) {
				item.Orders = append(item.Orders, o.label())
				item.orderKeys = append(item.orderKeys, o.key)
				proposed[o.key] = struct{}{}
			}
		}
		for _, code := range note.legacyCodes {
			codes = append(codes, code.legacyCode())
			// The PA order while it still has residual; otherwise the order of
			// the RDA that replaced it (the RDA quotes the PA in its object):
			// suppliers keep quoting the old PA after the renewal.
			openPA := false
			for _, o := range orders.byCodeNumber(code.number, true) {
				if orders.isOpen(o, own) {
					openPA = true
					item.Orders = append(item.Orders, o.label())
					item.orderKeys = append(item.orderKeys, o.key)
					proposed[o.key] = struct{}{}
				}
			}
			if openPA {
				continue
			}
			for _, rda := range refs.byLegacy[code.number] {
				if m := rdaCodeShape.FindStringSubmatch(strings.ToUpper(rda.code)); m != nil {
					for _, o := range orders.byCodeNumber(m[1], false) {
						item.Orders = append(item.Orders, o.label())
						item.orderKeys = append(item.orderKeys, o.key)
						proposed[o.key] = struct{}{}
					}
				}
			}
			if len(item.Orders) == 0 {
				for _, o := range orders.byCodeNumber(code.number, true) {
					item.Orders = append(item.Orders, o.label())
					item.orderKeys = append(item.orderKeys, o.key)
					proposed[o.key] = struct{}{}
				}
			}
		}
		item.Code = strings.Join(codes, " ")
		item.Orders = uniqueStrings(item.Orders)
		sort.Strings(item.Orders)
		result.OrderRefs = append(result.OrderRefs, item)
	}

	truth := orders.links.ordersOf(invoice.key)
	switch {
	case len(truth) == 0:
		result.Verdict = "no_truth"
	case len(proposed) == 0:
		result.Verdict = "none"
	default:
		hits := 0
		for _, t := range truth {
			if _, ok := proposed[t]; ok {
				hits++
			}
		}
		switch {
		case hits == len(truth) && len(proposed) == len(truth):
			result.Verdict = "match"
		case hits > 0:
			result.Verdict = "partial"
		default:
			result.Verdict = "wrong"
		}
	}
	return result
}

func addSDIResult(summary *MatchingFunnelSDISummary, result MatchingFunnelSDIResult, orderRules MatchingFunnelOrderRuleResult) {
	if !result.Linked {
		summary.InvoicesNotLinked++
		return
	}
	summary.InvoicesLinked++
	if result.LinkKind == "number_date" {
		summary.LinkedByNumberOnly++
	}
	if result.LinkKind == "vat_suffix_date" {
		summary.LinkedBySuffix++
	}
	if len(result.OrderRefs) > 0 {
		summary.WithOrderRef++
	}
	if len(result.Contracts) > 0 {
		summary.WithContractRef++
	}
	if result.PeriodStart != "" {
		summary.WithPeriod++
	}
	if len(result.Articles) > 0 {
		summary.WithArticleCodes++
	}
	usable, toOrder, toRDA, unresolved := false, false, false, false
	for _, ref := range result.OrderRefs {
		if ref.Code == "" {
			continue
		}
		usable = true
		if len(ref.Orders) > 0 {
			toOrder = true
		} else {
			unresolved = true
		}
		if ref.RDAID != nil {
			toRDA = true
		}
	}
	if usable {
		summary.WithUsableCode++
	}
	if toOrder {
		summary.ResolvedToOrder++
	}
	if toRDA {
		summary.ResolvedToRDA++
	}
	if unresolved {
		summary.UnresolvedCode++
	}
	switch result.Verdict {
	case "match":
		summary.LinkedInvoices++
		summary.Match++
		if orderRules.Verdict == "ambiguous" {
			summary.AmbiguousResolvedNow++
		}
	case "partial":
		summary.LinkedInvoices++
		summary.Partial++
	case "wrong":
		summary.LinkedInvoices++
		summary.Wrong++
	case "none":
		summary.LinkedInvoices++
		summary.None++
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
