package smartpassive

import (
	"net/url"
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// funnelScope selects the invoice base: "open" is the real Smart Passive
// scope (invoices not yet settled or not yet in the payment schedule), "all"
// widens it to every purchase invoice of the period and exists only for this
// analysis work.
type funnelScope string

const (
	funnelScopeOpen funnelScope = "open"
	funnelScopeAll  funnelScope = "all"
)

func parseFunnelScope(raw string) (funnelScope, bool) {
	switch raw {
	case "", string(funnelScopeOpen):
		return funnelScopeOpen, true
	case string(funnelScopeAll):
		return funnelScopeAll, true
	}
	return "", false
}

// funnelFilter selects the invoices to analyse: the scope and an optional
// window of document dates, both ends included.
type funnelFilter struct {
	scope funnelScope
	from  *time.Time
	to    *time.Time
}

// parseFunnelFilter reads scope, from and to (YYYY-MM-DD) from the query.
func parseFunnelFilter(q url.Values) (funnelFilter, string) {
	scope, ok := parseFunnelScope(q.Get("scope"))
	if !ok {
		return funnelFilter{}, "Parametro scope non valido: usare open oppure all"
	}
	f := funnelFilter{scope: scope}
	for _, name := range []string{"from", "to"} {
		raw := q.Get(name)
		if raw == "" {
			continue
		}
		day, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return funnelFilter{}, "Parametro " + name + " non valido: usare AAAA-MM-GG"
		}
		if name == "from" {
			f.from = &day
		} else {
			f.to = &day
		}
	}
	if f.from != nil && f.to != nil && f.to.Before(*f.from) {
		return funnelFilter{}, "Intervallo di date non valido: la fine precede l'inizio"
	}
	return f, ""
}

// where returns the invoice conditions for the filter. Dates are validated
// by parsing, so they can be written into the statement.
func (f funnelFilter) where() string {
	w := funnelInvoicesWhere
	if f.from != nil {
		w += "\n  AND t.DO11_DATADOC >= '" + f.from.Format("2006-01-02") + "'"
	}
	if f.to != nil {
		w += "\n  AND t.DO11_DATADOC < '" + f.to.AddDate(0, 0, 1).Format("2006-01-02") + "'"
	}
	if f.scope == funnelScopeAll {
		return w
	}
	return w + funnelOpenBalanceWhere
}

// funnelInvoicesQuery assembles the invoice query for the requested filter.
func funnelInvoicesQuery(f funnelFilter) string {
	if f.scope == funnelScopeAll {
		return funnelInvoicesSelect + f.where() + ";"
	}
	return funnelInvoicesSelect + funnelOpenBalanceJoin + f.where() + ";"
}

const funnelInvoicesSelect = `SELECT
    t.DO11_DITTA_CG18,
    t.DO11_NUMREG_CO99,
    t.DO11_CLIFOR_CG44,
    ISNULL(a.CG16_RAGSOANAG, '') AS RAGIONE_SOCIALE,
    t.DO11_NUMDOC,
    t.DO11_DATADOC,
    t.DO11_NUMDOCORIG,
    t.DO11_NOTEDOCUM,
    line_totals.IMPONIBILE,
    a.CG16_PARTIVA,
    a.CG16_PARTIVA_EST,
    a.CG16_CODFISCALE
FROM dbo.DO11_DOCTESTATA AS t
INNER JOIN dbo.MG36_DOCUMENTI AS d
    ON d.MG36_CODDOCUM = t.DO11_DOCUM_MG36
LEFT JOIN (
    SELECT
        DO30_DITTA_CG18,
        DO30_NUMREG_CO99,
        SUM(ISNULL(DO30_IMPNETSCP, 0)) AS IMPONIBILE
    FROM dbo.DO30_DOCCORPO
    GROUP BY DO30_DITTA_CG18, DO30_NUMREG_CO99
) AS line_totals
    ON line_totals.DO30_DITTA_CG18 = t.DO11_DITTA_CG18
   AND line_totals.DO30_NUMREG_CO99 = t.DO11_NUMREG_CO99
LEFT JOIN dbo.CG44_CLIFOR AS cf
    ON cf.CG44_DITTA_CG18 = t.DO11_DITTACF_CG44
   AND cf.CG44_TIPOCF = t.DO11_TIPOCF_CG44
   AND cf.CG44_CLIFOR = t.DO11_CLIFOR_CG44
LEFT JOIN dbo.CG16_ANAGGEN AS a
    ON a.CG16_CODICE = cf.CG44_CODICE_CG16
`

const funnelOpenBalanceJoin = `LEFT JOIN (
    SELECT
        EF01_DITTA_CG18,
        EF01_NUMREG_CO99,
        SUM(CASE
                WHEN EF01_INDSTATO_S = 0
                 AND ABS(ISNULL(EF01_IMPEFF_S, 0)) > 0.02
                THEN ISNULL(EF01_IMPEFF_S, 0)
                ELSE CAST(0 AS decimal(18,2))
            END) AS RESIDUO
    FROM dbo.EF01_SCADENZE
    WHERE EF01_DITTA_CG18 = 1
    GROUP BY EF01_DITTA_CG18, EF01_NUMREG_CO99
) AS s
    ON s.EF01_DITTA_CG18 = t.DO11_DITTA_CG18
   AND s.EF01_NUMREG_CO99 = t.DO11_NUMREG_CO99
`

const funnelInvoicesWhere = `WHERE d.MG36_INDCLIFOR = 2
  AND d.MG36_INDLISACQVEN = 1
  AND d.MG36_TIPODOC IN (3, 4, 5)
  AND t.DO11_DITTA_CG18 = 1
  AND t.DO11_ANNODOC > 2025`

const funnelOpenBalanceWhere = `
  AND (
        s.EF01_NUMREG_CO99 IS NULL
     OR ABS(ISNULL(s.RESIDUO, 0)) > 0.02
  )`

const funnelRDAsQuery = `SELECT
    po.id,
    po.code,
    po.state,
    po."type",
    po.object,
    po.total_price,
    po.currency,
    po.created,
    poh.delivered,
    p.erp_id,
    p.company_name,
    por.id AS row_id,
    por."type" AS row_type,
    por.qty,
    por.nrc,
    por.mrc,
    por.price,
    por.total,
    porp.is_recurrent,
    porp.month_recursion,
    porr.initial_subscription_months,
    porr.automatic_renew
FROM rda.purchase_order po
LEFT JOIN provider_qualifications.provider p ON p.id = po.provider_id
LEFT JOIN rda.purchase_order_row por ON por.order_id = po.id
LEFT JOIN rda.purchase_order_row_payment porp ON porp.purchase_order_row_id = por.id
LEFT JOIN rda.purchase_order_row_renew_rule porr ON porr.purchase_order_row_id = por.id
LEFT JOIN LATERAL (
    SELECT min(h."timestamp") AS delivered
    FROM rda.purchase_order_history h
    WHERE h.order_id = po.id
      AND h.next_state = 'DELIVERED_AND_COMPLIANT'
) poh ON TRUE
WHERE po."state" NOT IN ('DRAFT','CANCELED','REJECTED')
  AND po.deleted IS NULL
ORDER BY po.id, por.id;`

type funnelProfile string

const (
	funnelProfileOneTime   funnelProfile = "one_time"
	funnelProfileRecurring funnelProfile = "recurring"
	funnelProfileMixed     funnelProfile = "mixed"
	funnelProfileUnknown   funnelProfile = "unknown"
)

type funnelInvoice struct {
	key               string
	registration      int64
	supplierID        *int64
	supplierName      string
	documentNumber    string
	documentDate      *time.Time
	supplierReference string
	note              string
	taxableAmount     *float64
	// Alyante supplier identifiers, used to link the SDI document.
	supplierVAT        string
	supplierVATForeign string
	fiscalCode         string
}

type funnelRDALine struct {
	rowType        sql.NullString
	qty            sql.NullFloat64
	nrc            sql.NullFloat64
	mrc            sql.NullFloat64
	price          sql.NullFloat64
	total          sql.NullFloat64
	isRecurrent    sql.NullBool
	recurrence     sql.NullInt64
	initialMonths  sql.NullInt64
	automaticRenew sql.NullBool
}

type funnelRDA struct {
	id       int64
	code     string
	state    string
	rdaType  string
	object   string
	total    *float64
	currency string
	created  *time.Time
	// delivered is when the RDA first became "Erogata conforme" in Arak: the
	// service starts there, not at creation.
	delivered    *time.Time
	supplierID   *int64
	providerName string
	lines        map[int64]funnelRDALine
}

// startDate is the day the RDA's service is taken to start: delivery when
// known, creation otherwise.
func (r funnelRDA) startDate() *time.Time {
	if r.delivered != nil {
		return r.delivered
	}
	return r.created
}

type MatchingFunnelSummary struct {
	InvoiceCount       int `json:"invoice_count"`
	NoCandidates       int `json:"no_candidates"`
	OneCandidate       int `json:"one_candidate"`
	MultipleCandidates int `json:"multiple_candidates"`
	RDACount           int `json:"rda_count"`
	RDAsWithoutERPID   int `json:"rdas_without_erp_id"`
	DuplicateRDACodes  int `json:"duplicate_rda_codes"`
	RDAsWithLegacy     int `json:"rdas_with_legacy_predecessor"`
}

type MatchingFunnelProfileCounts struct {
	OneTime   int `json:"one_time"`
	Recurring int `json:"recurring"`
	Mixed     int `json:"mixed"`
	Unknown   int `json:"unknown"`
}

type MatchingFunnelInvoice struct {
	Registration      int64                         `json:"registration"`
	DocumentNumber    string                        `json:"document_number"`
	DocumentDate      *time.Time                    `json:"document_date"`
	SupplierReference string                        `json:"supplier_reference"`
	TaxableAmount     *float64                      `json:"taxable_amount"`
	Reference         MatchingFunnelReference       `json:"reference"`
	Rules             MatchingFunnelRuleResult      `json:"rules"`
	OrderRules        MatchingFunnelOrderRuleResult `json:"order_rules"`
	SDI               MatchingFunnelSDIResult       `json:"sdi"`
	Cascade           MatchingCascadeInvoice        `json:"cascade"`
	Suggestion        MatchingSuggestionInvoice     `json:"suggestion"`
}

type MatchingFunnelRDA struct {
	ID    int64  `json:"id"`
	Code  string `json:"code"`
	State string `json:"state"`
	Type  string `json:"type"`
	// HasOrder is true when an Alyante order carries this RDA code in its
	// original document number.
	HasOrder bool       `json:"has_order"`
	Object   string     `json:"object"`
	Total    *float64   `json:"total"`
	Currency string     `json:"currency"`
	Created  *time.Time `json:"created"`
	// Delivered is the first "Erogata conforme" transition in the Arak history.
	Delivered *time.Time    `json:"delivered"`
	Profile   funnelProfile `json:"profile"`
}

type MatchingFunnelSupplier struct {
	SupplierERPID       *int64  `json:"supplier_erp_id"`
	AlyanteSupplierName *string `json:"alyante_supplier_name"`
	ProviderName        *string `json:"provider_name"`
	// Billing is the supplier's billing profile read from all its electronic
	// invoices; nil when the supplier never sent one.
	Billing             *MatchingSupplierBilling       `json:"billing"`
	InvoiceCount        int                            `json:"invoice_count"`
	CandidateCount      int                            `json:"candidate_count"`
	Outcome             string                         `json:"outcome"`
	Profiles            MatchingFunnelProfileCounts    `json:"profiles"`
	Reference           MatchingFunnelReferenceSummary `json:"reference"`
	Invoices            []MatchingFunnelInvoice        `json:"invoices"`
	Candidates          []MatchingFunnelRDA            `json:"candidates"`
	OrderCandidateCount int                            `json:"order_candidate_count"`
	OrderOutcome        string                         `json:"order_outcome"`
	Orders              []MatchingFunnelOrder          `json:"orders"`
	Contracts           []MatchingFunnelContract       `json:"contracts"`
}

// MatchingFunnelRDAOrderCounts counts, for one RDA type, the RDAs of the
// universe that have an Alyante order carrying their code and those that do not.
type MatchingFunnelRDAOrderCounts struct {
	WithOrder    int `json:"with_order"`
	WithoutOrder int `json:"without_order"`
	// WithoutOrderByState splits the RDAs without an order by Arak state, to
	// tell the ones not yet approved from the ones AFC should have loaded.
	WithoutOrderByState map[string]int `json:"without_order_by_state"`
}

// MatchingFunnelOrderYearCounts counts the Alyante orders dated in one year, by
// document code, by what their original document number carries.
type MatchingFunnelOrderYearCounts struct {
	WithRDACode    int `json:"with_rda_code"`
	WithLegacyCode int `json:"with_legacy_code"`
	WithoutCode    int `json:"without_code"`
}

// MatchingFunnelChainSummary measures the RDA → Alyante order link in both
// directions: which RDAs have an order, and which orders carry a code.
type MatchingFunnelChainSummary struct {
	RDAsByType   map[string]MatchingFunnelRDAOrderCounts  `json:"rdas_by_type"`
	OrdersByYear map[string]MatchingFunnelOrderYearCounts `json:"orders_by_year"`
}

type MatchingFunnelResponse struct {
	Scope      funnelScope                     `json:"scope"`
	From       *time.Time                      `json:"from"`
	To         *time.Time                      `json:"to"`
	Summary    MatchingFunnelSummary           `json:"summary"`
	Chain      MatchingFunnelChainSummary      `json:"chain"`
	Profiles   MatchingFunnelProfileCounts     `json:"profiles"`
	Reference  MatchingFunnelReferenceSummary  `json:"reference"`
	Rules      MatchingFunnelRulesSummary      `json:"rules"`
	Orders     MatchingFunnelOrdersSummary     `json:"orders"`
	OrderRules MatchingFunnelOrderRulesSummary `json:"order_rules"`
	Contracts  MatchingFunnelContractsSummary  `json:"contracts"`
	SDI        MatchingFunnelSDISummary        `json:"sdi"`
	Cascade    MatchingCascadeSummary          `json:"cascade"`
	Suggestions MatchingSuggestionSummary      `json:"suggestions"`
	Suppliers  []MatchingFunnelSupplier        `json:"suppliers"`
}

func (h *Handler) handleMatchingFunnel(w http.ResponseWriter, r *http.Request) {
	if h.alyanteDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Connessione Alyante non configurata")
		return
	}
	if h.arakDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Connessione Arak non configurata")
		return
	}

	filter, problem := parseFunnelFilter(r.URL.Query())
	if problem != "" {
		httputil.Error(w, http.StatusBadRequest, problem)
		return
	}
	scope := filter.scope

	invoices, err := h.loadFunnelInvoices(r, filter)
	if err != nil {
		httputil.InternalError(w, r, err, "matching funnel invoices query failed", "component", component, "operation", "load_funnel_invoices")
		return
	}
	// The fixed-fee series is read from every invoice of the supplier, settled
	// ones included: in the open scope the earlier members are already paid.
	seriesBase := invoices
	if scope != funnelScopeAll || filter.from != nil || filter.to != nil {
		if seriesBase, err = h.loadFunnelInvoices(r, funnelFilter{scope: funnelScopeAll}); err != nil {
			httputil.InternalError(w, r, err, "matching funnel series base query failed", "component", component, "operation", "load_funnel_series_base")
			return
		}
	}
	rdas, err := h.loadFunnelRDAs(r)
	if err != nil {
		httputil.InternalError(w, r, err, "matching funnel rdas query failed", "component", component, "operation", "load_funnel_rdas")
		return
	}
	orders, err := h.loadFunnelOrders(r)
	if err != nil {
		httputil.InternalError(w, r, err, "matching funnel orders query failed", "component", component, "operation", "load_funnel_orders")
		return
	}
	invoiceLines, err := h.loadFunnelInvoiceLines(r, filter)
	if err != nil {
		httputil.InternalError(w, r, err, "matching funnel invoice lines query failed", "component", component, "operation", "load_funnel_invoice_lines")
		return
	}
	orderLinks, err := h.loadFunnelOrderLinks(r)
	if err != nil {
		httputil.InternalError(w, r, err, "matching funnel order links query failed", "component", component, "operation", "load_funnel_order_links")
		return
	}
	contracts, err := h.loadFunnelContracts(r)
	if err != nil {
		httputil.InternalError(w, r, err, "matching funnel contracts query failed", "component", component, "operation", "load_funnel_contracts")
		return
	}
	contractLinks, err := h.loadFunnelContractLinks(r)
	if err != nil {
		httputil.InternalError(w, r, err, "matching funnel contract links query failed", "component", component, "operation", "load_funnel_contract_links")
		return
	}

	sdiDocs, err := h.loadFunnelSDI(r, invoices)
	if err != nil {
		httputil.InternalError(w, r, err, "matching funnel sdi query failed", "component", component, "operation", "load_funnel_sdi")
		return
	}

	billing, err := h.loadFunnelBilling(r)
	if err != nil {
		httputil.InternalError(w, r, err, "matching funnel billing query failed", "component", component, "operation", "load_funnel_billing")
		return
	}

	response := buildMatchingFunnel(invoices, seriesBase, rdas, orders, invoiceLines, orderLinks, newFunnelContractIndex(contracts, contractLinks), sdiDocs, billing)
	response.From, response.To = filter.from, filter.to
	response.Scope = scope
	httputil.JSON(w, http.StatusOK, response)
}

func (h *Handler) loadFunnelInvoices(r *http.Request, f funnelFilter) ([]funnelInvoice, error) {
	rows, err := h.alyanteDB.QueryContext(r.Context(), funnelInvoicesQuery(f))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]funnelInvoice, 0)
	for rows.Next() {
		var ditta int64
		var registration int64
		var supplier sql.NullInt64
		var supplierName sql.NullString
		var documentNumber sql.NullString
		var documentDate sql.NullTime
		var supplierReference sql.NullString
		var note sql.NullString
		var taxableAmount sql.NullFloat64
		var vat, vatForeign, fiscalCode sql.NullString
		if err := rows.Scan(
			&ditta,
			&registration,
			&supplier,
			&supplierName,
			&documentNumber,
			&documentDate,
			&supplierReference,
			&note,
			&taxableAmount,
			&vat,
			&vatForeign,
			&fiscalCode,
		); err != nil {
			return nil, err
		}
		key := strconv.FormatInt(ditta, 10) + ":" + strconv.FormatInt(registration, 10)
		out = append(out, funnelInvoice{
			key:                key,
			registration:       registration,
			supplierID:         int64Ptr(supplier),
			supplierName:       supplierName.String,
			documentNumber:     documentNumber.String,
			documentDate:       timePtr(documentDate),
			supplierReference:  supplierReference.String,
			note:               note.String,
			taxableAmount:      float64Ptr(taxableAmount),
			supplierVAT:        vat.String,
			supplierVATForeign: vatForeign.String,
			fiscalCode:         fiscalCode.String,
		})
	}
	return out, rows.Err()
}

func (h *Handler) loadFunnelRDAs(r *http.Request) ([]funnelRDA, error) {
	rows, err := h.arakDB.QueryContext(r.Context(), funnelRDAsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := make(map[int64]*funnelRDA)
	order := make([]int64, 0)
	for rows.Next() {
		var id int64
		var code sql.NullString
		var state sql.NullString
		var rdaType sql.NullString
		var object sql.NullString
		var total sql.NullFloat64
		var currency sql.NullString
		var created sql.NullTime
		var delivered sql.NullTime
		var supplier sql.NullInt64
		var providerName sql.NullString
		var rowID sql.NullInt64
		var line funnelRDALine
		if err := rows.Scan(
			&id,
			&code,
			&state,
			&rdaType,
			&object,
			&total,
			&currency,
			&created,
			&delivered,
			&supplier,
			&providerName,
			&rowID,
			&line.rowType,
			&line.qty,
			&line.nrc,
			&line.mrc,
			&line.price,
			&line.total,
			&line.isRecurrent,
			&line.recurrence,
			&line.initialMonths,
			&line.automaticRenew,
		); err != nil {
			return nil, err
		}

		item := byID[id]
		if item == nil {
			item = &funnelRDA{
				id:           id,
				code:         code.String,
				state:        state.String,
				rdaType:      rdaType.String,
				object:       object.String,
				total:        float64Ptr(total),
				currency:     currency.String,
				created:      timePtr(created),
				delivered:    timePtr(delivered),
				supplierID:   int64Ptr(supplier),
				providerName: providerName.String,
				lines:        make(map[int64]funnelRDALine),
			}
			byID[id] = item
			order = append(order, id)
		}
		if rowID.Valid {
			item.lines[rowID.Int64] = line
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]funnelRDA, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

func buildMatchingFunnel(invoices, seriesBase []funnelInvoice, rdas []funnelRDA, orders []funnelOrder, invoiceLines map[string][]funnelDocLine, orderLinks funnelOrderLinks, contractIndex funnelContractIndex, sdiDocs map[string][]*sdiDocument, billing map[string]MatchingSupplierBilling) MatchingFunnelResponse {
	rdasBySupplier := make(map[int64][]funnelRDA)
	rdaByID := make(map[int64]funnelRDA, len(rdas))
	profilesByRDA := make(map[int64]funnelProfile, len(rdas))
	referenceIndex := newFunnelReferenceIndex(rdas)
	orderIndex := newFunnelOrderIndex(orders, referenceIndex, orderLinks)
	response := MatchingFunnelResponse{
		Orders:    orderIndex.summary,
		Contracts: MatchingFunnelContractsSummary{ContractCount: contractIndex.count},
		Chain:     buildChainSummary(rdas, orders, orderIndex),
		Cascade:   newCascadeSummary(),
		Suggestions: newSuggestionSummary(),
		Summary: MatchingFunnelSummary{
			InvoiceCount:      len(invoices),
			RDACount:          len(rdas),
			DuplicateRDACodes: referenceIndex.duplicateCodes(),
			RDAsWithLegacy:    referenceIndex.rdasWithLegacyPredecessor(),
		},
		Suppliers: make([]MatchingFunnelSupplier, 0),
	}

	for _, rda := range rdas {
		rdaByID[rda.id] = rda
		profile := classifyFunnelRDA(rda)
		profilesByRDA[rda.id] = profile
		addFunnelProfile(&response.Profiles, profile)
		if rda.supplierID == nil {
			response.Summary.RDAsWithoutERPID++
			continue
		}
		rdasBySupplier[*rda.supplierID] = append(rdasBySupplier[*rda.supplierID], rda)
	}

	type supplierAccumulator struct {
		supplierID  *int64
		alyanteName string
		invoices    []funnelInvoice
	}
	supplierInvoices := make(map[string]*supplierAccumulator)
	seenInvoices := make(map[string]struct{}, len(invoices))
	for _, invoice := range invoices {
		if _, seen := seenInvoices[invoice.key]; seen {
			continue
		}
		seenInvoices[invoice.key] = struct{}{}
		key := "missing"
		if invoice.supplierID != nil {
			key = strconv.FormatInt(*invoice.supplierID, 10)
		}
		acc := supplierInvoices[key]
		if acc == nil {
			acc = &supplierAccumulator{supplierID: invoice.supplierID}
			supplierInvoices[key] = acc
		}
		acc.invoices = append(acc.invoices, invoice)
		if acc.alyanteName == "" && invoice.supplierName != "" {
			acc.alyanteName = invoice.supplierName
		}
	}
	response.Summary.InvoiceCount = len(seenInvoices)

	seriesBySupplier := make(map[string][]funnelInvoice)
	for _, invoice := range seriesBase {
		key := "missing"
		if invoice.supplierID != nil {
			key = strconv.FormatInt(*invoice.supplierID, 10)
		}
		seriesBySupplier[key] = append(seriesBySupplier[key], invoice)
	}

	for supplierKey, acc := range supplierInvoices {
		candidates := []funnelRDA(nil)
		if acc.supplierID != nil {
			candidates = rdasBySupplier[*acc.supplierID]
		}
		outcome := funnelOutcome(len(candidates))
		switch outcome {
		case "none":
			response.Summary.NoCandidates += len(acc.invoices)
		case "one":
			response.Summary.OneCandidate += len(acc.invoices)
		case "multiple":
			response.Summary.MultipleCandidates += len(acc.invoices)
		}

		orderCandidates := orderIndex.candidates(acc.supplierID)
		orderOutcome := funnelOutcome(len(orderCandidates))
		switch orderOutcome {
		case "none":
			response.Orders.InvoicesNoOrder += len(acc.invoices)
		case "one":
			response.Orders.InvoicesOneOrder += len(acc.invoices)
		case "multiple":
			response.Orders.InvoicesManyOrder += len(acc.invoices)
		}

		row := MatchingFunnelSupplier{
			SupplierERPID:       acc.supplierID,
			InvoiceCount:        len(acc.invoices),
			CandidateCount:      len(candidates),
			Outcome:             outcome,
			Invoices:            make([]MatchingFunnelInvoice, 0, len(acc.invoices)),
			Candidates:          make([]MatchingFunnelRDA, 0, len(candidates)),
			OrderCandidateCount: len(orderCandidates),
			OrderOutcome:        orderOutcome,
			Orders:              make([]MatchingFunnelOrder, 0, len(orderCandidates)),
		}
		for _, o := range orderCandidates {
			row.Orders = append(row.Orders, orderIndex.export(o))
		}
		supplierContracts := contractIndex.candidates(acc.supplierID)
		row.Contracts = make([]MatchingFunnelContract, 0, len(supplierContracts))
		for _, c := range supplierContracts {
			row.Contracts = append(row.Contracts, contractIndex.export(c))
		}
		if len(supplierContracts) > 0 {
			response.Contracts.SuppliersWithContracts++
		}
		sort.Slice(row.Orders, func(i, j int) bool {
			return timeAfter(row.Orders[i].Date, row.Orders[j].Date)
		})
		if acc.alyanteName != "" {
			name := acc.alyanteName
			row.AlyanteSupplierName = &name
		}
		for _, invoice := range acc.invoices {
			if row.Billing = supplierBilling(billing, invoice); row.Billing != nil {
				break
			}
		}
		fixedFeeSeries := buildFixedFeeSeries(seriesBySupplier[supplierKey])
		for _, invoice := range acc.invoices {
			reference := referenceIndex.resolve(invoice, parseAFCNote(invoice.note))
			addReferenceOutcome(&row.Reference, reference)
			addReferenceOutcome(&response.Reference, reference)
			rules := applyFunnelRules(invoice, candidates, reference)
			addRuleResult(&response.Rules, rules)
			if len(orderLinks.byInvoice[invoice.key]) > 0 {
				response.Orders.InvoicesLinked++
			}
			orderRules := applyOrderRules(invoice, invoiceLines[invoice.key], orderIndex)
			addOrderRuleResult(&response.OrderRules, orderRules)
			sdi := applySDI(invoice, sdiDocs[normalizeDocNumber(invoice.supplierReference)], orderIndex, referenceIndex)
			addSDIResult(&response.SDI, sdi, orderRules)
			fixedFee := applyFixedFee(invoice, fixedFeeSeries, orderIndex)
			contractTruth := contractIndex.truth(invoice.key)
			if len(contractTruth) > 0 {
				response.Contracts.InvoicesLinked++
			}
			contractsResult := applyContracts(invoice, supplierContracts)
			cascade := applyCascade(invoice, sdi, contractsResult, contractTruth, orderRules, fixedFee, orderCandidates, candidates, orderIndex, rdaByID)
			addCascade(&response.Cascade, cascade)
			suggestion := applySuggestion(invoice, sdi, contractsResult, contractTruth, orderRules, fixedFee, orderCandidates, candidates, orderIndex, rdaByID)
			addSuggestion(&response.Suggestions, suggestion)
			switch funnelOutcome(orderRules.OpenCandidates) {
			case "none":
				response.Orders.InvoicesNoOpen++
			case "one":
				response.Orders.InvoicesOneOpen++
			case "multiple":
				response.Orders.InvoicesManyOpen++
			}
			row.Invoices = append(row.Invoices, MatchingFunnelInvoice{
				Registration:      invoice.registration,
				DocumentNumber:    invoice.documentNumber,
				DocumentDate:      invoice.documentDate,
				SupplierReference: invoice.supplierReference,
				TaxableAmount:     invoice.taxableAmount,
				Reference:         reference,
				Rules:             rules,
				OrderRules:        orderRules,
				SDI:               sdi,
				Cascade:           cascade,
				Suggestion:        suggestion,
			})
		}
		for _, candidate := range candidates {
			profile := profilesByRDA[candidate.id]
			addFunnelProfile(&row.Profiles, profile)
			row.Candidates = append(row.Candidates, MatchingFunnelRDA{
				ID:        candidate.id,
				Code:      candidate.code,
				State:     candidate.state,
				Type:      candidate.rdaType,
				HasOrder:  orderIndex.hasOrderFor(candidate),
				Object:    candidate.object,
				Total:     candidate.total,
				Currency:  candidate.currency,
				Created:   candidate.created,
				Delivered: candidate.delivered,
				Profile:   profile,
			})
			if row.ProviderName == nil && candidate.providerName != "" {
				name := candidate.providerName
				row.ProviderName = &name
			}
		}
		sort.Slice(row.Invoices, func(i, j int) bool {
			return timeAfter(row.Invoices[i].DocumentDate, row.Invoices[j].DocumentDate)
		})
		sort.Slice(row.Candidates, func(i, j int) bool {
			return row.Candidates[i].ID < row.Candidates[j].ID
		})
		response.Suppliers = append(response.Suppliers, row)
	}

	sortReasons(response.Cascade.ResidualBySDI)
	sortReasons(response.Cascade.ResidualByOrders)
	sortReasons(response.Cascade.ResidualByPair)
	sortReasons(response.Cascade.ResidualByFixedFee)
	sortReasons(response.Cascade.ResidualByContracts)
	sortReasons(response.Cascade.ResidualByFamily)
	sort.Slice(response.Suppliers, func(i, j int) bool {
		if response.Suppliers[i].InvoiceCount != response.Suppliers[j].InvoiceCount {
			return response.Suppliers[i].InvoiceCount > response.Suppliers[j].InvoiceCount
		}
		left, right := int64(-1), int64(-1)
		if response.Suppliers[i].SupplierERPID != nil {
			left = *response.Suppliers[i].SupplierERPID
		}
		if response.Suppliers[j].SupplierERPID != nil {
			right = *response.Suppliers[j].SupplierERPID
		}
		return left < right
	})

	return response
}

// buildChainSummary measures the RDA → order link in both directions.
func buildChainSummary(rdas []funnelRDA, orders []funnelOrder, idx funnelOrderIndex) MatchingFunnelChainSummary {
	out := MatchingFunnelChainSummary{
		RDAsByType:   make(map[string]MatchingFunnelRDAOrderCounts),
		OrdersByYear: make(map[string]MatchingFunnelOrderYearCounts),
	}
	for _, rda := range rdas {
		kind := rda.rdaType
		if kind == "" {
			kind = "?"
		}
		counts := out.RDAsByType[kind]
		if counts.WithoutOrderByState == nil {
			counts.WithoutOrderByState = make(map[string]int)
		}
		if idx.hasOrderFor(rda) {
			counts.WithOrder++
		} else {
			counts.WithoutOrder++
			counts.WithoutOrderByState[rda.state]++
		}
		out.RDAsByType[kind] = counts
	}
	for _, o := range orders {
		year := "?"
		if o.date != nil {
			year = strconv.Itoa(o.date.Year())
		}
		year += " " + o.docCode
		counts := out.OrdersByYear[year]
		switch {
		case o.rdaCode == nil:
			counts.WithoutCode++
		case o.rdaLegacy:
			counts.WithLegacyCode++
		default:
			counts.WithRDACode++
		}
		out.OrdersByYear[year] = counts
	}
	return out
}

func classifyFunnelRDA(rda funnelRDA) funnelProfile {
	if len(rda.lines) == 0 {
		return funnelProfileUnknown
	}

	hasOneTime := false
	hasRecurring := false
	for _, line := range rda.lines {
		profile := classifyFunnelLine(line)
		if profile == funnelProfileUnknown {
			return funnelProfileUnknown
		}
		if profile == funnelProfileOneTime {
			hasOneTime = true
		}
		if profile == funnelProfileRecurring {
			hasRecurring = true
		}
	}
	if hasOneTime && hasRecurring {
		return funnelProfileMixed
	}
	if hasRecurring {
		return funnelProfileRecurring
	}
	return funnelProfileOneTime
}

func classifyFunnelLine(line funnelRDALine) funnelProfile {
	if line.rowType.Valid && line.rowType.String == "good" {
		return funnelProfileOneTime
	}
	if !line.rowType.Valid || line.rowType.String != "service" {
		return funnelProfileUnknown
	}

	oneMonthNoRenewal := line.recurrence.Valid && line.recurrence.Int64 == 1 &&
		line.initialMonths.Valid && line.initialMonths.Int64 == 1 &&
		line.automaticRenew.Valid && !line.automaticRenew.Bool
	if oneMonthNoRenewal {
		return funnelProfileOneTime
	}
	if (line.automaticRenew.Valid && line.automaticRenew.Bool) ||
		(line.initialMonths.Valid && line.initialMonths.Int64 > 1) ||
		(line.recurrence.Valid && line.recurrence.Int64 > 1) ||
		(line.isRecurrent.Valid && line.isRecurrent.Bool) {
		return funnelProfileRecurring
	}
	return funnelProfileUnknown
}

func addFunnelProfile(counts *MatchingFunnelProfileCounts, profile funnelProfile) {
	switch profile {
	case funnelProfileOneTime:
		counts.OneTime++
	case funnelProfileRecurring:
		counts.Recurring++
	case funnelProfileMixed:
		counts.Mixed++
	default:
		counts.Unknown++
	}
}

func timeAfter(left, right *time.Time) bool {
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	return left.After(*right)
}

func funnelOutcome(candidateCount int) string {
	if candidateCount == 0 {
		return "none"
	}
	if candidateCount == 1 {
		return "one"
	}
	return "multiple"
}
