package smartpassive

import (
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const funnelInvoicesQuery = `SELECT
    t.DO11_DITTA_CG18,
    t.DO11_NUMREG_CO99,
    t.DO11_CLIFOR_CG44,
    ISNULL(a.CG16_RAGSOANAG, '') AS RAGIONE_SOCIALE,
    t.DO11_NUMDOC,
    t.DO11_DATADOC,
    t.DO11_NUMDOCORIG,
    tot.DO13_TOTDOCUMENTO
FROM dbo.DO11_DOCTESTATA AS t
INNER JOIN dbo.MG36_DOCUMENTI AS d
    ON d.MG36_CODDOCUM = t.DO11_DOCUM_MG36
LEFT JOIN dbo.DO13_DOCTOTALI AS tot
    ON tot.DO13_DITTA_CG18 = t.DO11_DITTA_CG18
   AND tot.DO13_NUMREG_CO99 = t.DO11_NUMREG_CO99
LEFT JOIN dbo.CG44_CLIFOR AS cf
    ON cf.CG44_DITTA_CG18 = t.DO11_DITTACF_CG44
   AND cf.CG44_TIPOCF = t.DO11_TIPOCF_CG44
   AND cf.CG44_CLIFOR = t.DO11_CLIFOR_CG44
LEFT JOIN dbo.CG16_ANAGGEN AS a
    ON a.CG16_CODICE = cf.CG44_CODICE_CG16
LEFT JOIN (
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
WHERE d.MG36_INDCLIFOR = 2
  AND d.MG36_INDLISACQVEN = 1
  AND d.MG36_TIPODOC IN (3, 4, 5)
  AND t.DO11_DITTA_CG18 = 1
  AND t.DO11_ANNODOC > 2025
  AND (
        s.EF01_NUMREG_CO99 IS NULL
     OR ABS(ISNULL(s.RESIDUO, 0)) > 0.02
  );`

const funnelRDAsQuery = `SELECT
    po.id,
    po.code,
    po.state,
    po.object,
    po.total_price,
    po.currency,
    po.created,
    p.erp_id,
    p.company_name,
    por.id AS row_id,
    por."type" AS row_type,
    porp.is_recurrent,
    porp.month_recursion,
    porr.initial_subscription_months,
    porr.automatic_renew
FROM rda.purchase_order po
LEFT JOIN provider_qualifications.provider p ON p.id = po.provider_id
LEFT JOIN rda.purchase_order_row por ON por.order_id = po.id
LEFT JOIN rda.purchase_order_row_payment porp ON porp.purchase_order_row_id = por.id
LEFT JOIN rda.purchase_order_row_renew_rule porr ON porr.purchase_order_row_id = por.id
WHERE po."state" NOT IN ('DRAFT','CANCELED')
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
	total             *float64
}

type funnelRDALine struct {
	rowType        sql.NullString
	isRecurrent    sql.NullBool
	recurrence     sql.NullInt64
	initialMonths  sql.NullInt64
	automaticRenew sql.NullBool
}

type funnelRDA struct {
	id           int64
	code         string
	state        string
	object       string
	total        *float64
	currency     string
	created      *time.Time
	supplierID   *int64
	providerName string
	lines        map[int64]funnelRDALine
}

type MatchingFunnelSummary struct {
	InvoiceCount       int `json:"invoice_count"`
	NoCandidates       int `json:"no_candidates"`
	OneCandidate       int `json:"one_candidate"`
	MultipleCandidates int `json:"multiple_candidates"`
	RDACount           int `json:"rda_count"`
	RDAsWithoutERPID   int `json:"rdas_without_erp_id"`
}

type MatchingFunnelProfileCounts struct {
	OneTime   int `json:"one_time"`
	Recurring int `json:"recurring"`
	Mixed     int `json:"mixed"`
	Unknown   int `json:"unknown"`
}

type MatchingFunnelInvoice struct {
	Registration      int64      `json:"registration"`
	DocumentNumber    string     `json:"document_number"`
	DocumentDate      *time.Time `json:"document_date"`
	SupplierReference string     `json:"supplier_reference"`
	Total             *float64   `json:"total"`
}

type MatchingFunnelRDA struct {
	ID       int64         `json:"id"`
	Code     string        `json:"code"`
	State    string        `json:"state"`
	Object   string        `json:"object"`
	Total    *float64      `json:"total"`
	Currency string        `json:"currency"`
	Created  *time.Time    `json:"created"`
	Profile  funnelProfile `json:"profile"`
}

type MatchingFunnelSupplier struct {
	SupplierERPID       *int64                      `json:"supplier_erp_id"`
	AlyanteSupplierName *string                     `json:"alyante_supplier_name"`
	ProviderName        *string                     `json:"provider_name"`
	InvoiceCount        int                         `json:"invoice_count"`
	CandidateCount      int                         `json:"candidate_count"`
	Outcome             string                      `json:"outcome"`
	Profiles            MatchingFunnelProfileCounts `json:"profiles"`
	Invoices            []MatchingFunnelInvoice     `json:"invoices"`
	Candidates          []MatchingFunnelRDA         `json:"candidates"`
}

type MatchingFunnelResponse struct {
	Summary   MatchingFunnelSummary       `json:"summary"`
	Profiles  MatchingFunnelProfileCounts `json:"profiles"`
	Suppliers []MatchingFunnelSupplier    `json:"suppliers"`
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

	invoices, err := h.loadFunnelInvoices(r)
	if err != nil {
		httputil.InternalError(w, r, err, "matching funnel invoices query failed", "component", component, "operation", "load_funnel_invoices")
		return
	}
	rdas, err := h.loadFunnelRDAs(r)
	if err != nil {
		httputil.InternalError(w, r, err, "matching funnel rdas query failed", "component", component, "operation", "load_funnel_rdas")
		return
	}

	response := buildMatchingFunnel(invoices, rdas)
	httputil.JSON(w, http.StatusOK, response)
}

func (h *Handler) loadFunnelInvoices(r *http.Request) ([]funnelInvoice, error) {
	rows, err := h.alyanteDB.QueryContext(r.Context(), funnelInvoicesQuery)
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
		var total sql.NullFloat64
		if err := rows.Scan(
			&ditta,
			&registration,
			&supplier,
			&supplierName,
			&documentNumber,
			&documentDate,
			&supplierReference,
			&total,
		); err != nil {
			return nil, err
		}
		key := strconv.FormatInt(ditta, 10) + ":" + strconv.FormatInt(registration, 10)
		out = append(out, funnelInvoice{
			key:               key,
			registration:      registration,
			supplierID:        int64Ptr(supplier),
			supplierName:      supplierName.String,
			documentNumber:    documentNumber.String,
			documentDate:      timePtr(documentDate),
			supplierReference: supplierReference.String,
			total:             float64Ptr(total),
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
		var object sql.NullString
		var total sql.NullFloat64
		var currency sql.NullString
		var created sql.NullTime
		var supplier sql.NullInt64
		var providerName sql.NullString
		var rowID sql.NullInt64
		var line funnelRDALine
		if err := rows.Scan(
			&id,
			&code,
			&state,
			&object,
			&total,
			&currency,
			&created,
			&supplier,
			&providerName,
			&rowID,
			&line.rowType,
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
				object:       object.String,
				total:        float64Ptr(total),
				currency:     currency.String,
				created:      timePtr(created),
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

func buildMatchingFunnel(invoices []funnelInvoice, rdas []funnelRDA) MatchingFunnelResponse {
	rdasBySupplier := make(map[int64][]funnelRDA)
	profilesByRDA := make(map[int64]funnelProfile, len(rdas))
	response := MatchingFunnelResponse{
		Summary:   MatchingFunnelSummary{InvoiceCount: len(invoices), RDACount: len(rdas)},
		Suppliers: make([]MatchingFunnelSupplier, 0),
	}

	for _, rda := range rdas {
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

	for _, acc := range supplierInvoices {
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

		row := MatchingFunnelSupplier{
			SupplierERPID:  acc.supplierID,
			InvoiceCount:   len(acc.invoices),
			CandidateCount: len(candidates),
			Outcome:        outcome,
			Invoices:       make([]MatchingFunnelInvoice, 0, len(acc.invoices)),
			Candidates:     make([]MatchingFunnelRDA, 0, len(candidates)),
		}
		if acc.alyanteName != "" {
			name := acc.alyanteName
			row.AlyanteSupplierName = &name
		}
		for _, invoice := range acc.invoices {
			row.Invoices = append(row.Invoices, MatchingFunnelInvoice{
				Registration:      invoice.registration,
				DocumentNumber:    invoice.documentNumber,
				DocumentDate:      invoice.documentDate,
				SupplierReference: invoice.supplierReference,
				Total:             invoice.total,
			})
		}
		for _, candidate := range candidates {
			profile := profilesByRDA[candidate.id]
			addFunnelProfile(&row.Profiles, profile)
			row.Candidates = append(row.Candidates, MatchingFunnelRDA{
				ID:       candidate.id,
				Code:     candidate.code,
				State:    candidate.state,
				Object:   candidate.object,
				Total:    candidate.total,
				Currency: candidate.currency,
				Created:  candidate.created,
				Profile:  profile,
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
