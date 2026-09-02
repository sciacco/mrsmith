package smartpassive

import (
	"database/sql"
	"net/http"
	"sync"
)

// The billing profile of a supplier is read from every electronic invoice it
// sent, not only the ones in scope: how many invoices a month, how many lines
// each, how often the same taxable amount comes back. Three numbers that tell a
// contract-by-contract biller from a wholesale aggregate and from a fixed
// monthly fee. Credit and debit notes are left out.
const funnelBillingQuery = `SELECT
    supplier_vat,
    count(*) AS docs,
    count(DISTINCT date_trunc('month', document_date)) AS months,
    percentile_cont(0.5) WITHIN GROUP (ORDER BY lines) AS median_lines,
    sum(CASE WHEN dup > 1 THEN 1 ELSE 0 END)::float / count(*) AS repeat_share,
    avg(CASE WHEN has_period THEN 1 ELSE 0 END) AS period_share,
    avg(CASE WHEN has_code THEN 1 ELSE 0 END) AS code_share
FROM (
    SELECT d.*, count(*) OVER (PARTITION BY supplier_vat, taxable) AS dup
    FROM (
        SELECT
            supplier_vat,
            document_date,
            (length(xml) - length(replace(xml, '<DettaglioLinee>', ''))) / length('<DettaglioLinee>') AS lines,
            position('<DataInizioPeriodo>' IN xml) > 0 AS has_period,
            xml ~* '<IdDocumento>\s*P[OA][-/ ]?\d' AS has_code,
            (SELECT round(sum(m[1]::numeric), 2)
             FROM regexp_matches(xml, '<ImponibileImporto>([^<]+)</ImponibileImporto>', 'g') AS m) AS taxable
        FROM smartpassive.sdi_invoice
        WHERE supplier_vat IS NOT NULL
          AND coalesce(document_type, '') NOT IN ('TD04', 'TD05', 'TD08')
    ) d
) x
GROUP BY supplier_vat`

// MatchingSupplierBilling is the billing profile of one supplier.
type MatchingSupplierBilling struct {
	Documents   int     `json:"documents"`
	Months      int     `json:"months"`
	PerMonth    float64 `json:"per_month"`
	MedianLines float64 `json:"median_lines"`
	// RepeatShare is the share of invoices whose taxable amount equals that
	// of another invoice of the same supplier.
	RepeatShare float64 `json:"repeat_share"`
	PeriodShare float64 `json:"period_share"`
	CodeShare   float64 `json:"code_share"`
}

// billingCache holds the profiles per process, keyed by the document count:
// the table only grows, so a changed count means new invoices to include.
var billingCache struct {
	sync.Mutex
	count    int64
	profiles map[string]MatchingSupplierBilling
}

func (h *Handler) loadFunnelBilling(r *http.Request) (map[string]MatchingSupplierBilling, error) {
	if h.anisettaDB == nil {
		return map[string]MatchingSupplierBilling{}, nil
	}
	var count int64
	if err := h.anisettaDB.QueryRowContext(r.Context(), `SELECT count(*) FROM smartpassive.sdi_invoice`).Scan(&count); err != nil {
		return nil, err
	}
	billingCache.Lock()
	defer billingCache.Unlock()
	if billingCache.profiles != nil && billingCache.count == count {
		return billingCache.profiles, nil
	}
	rows, err := h.anisettaDB.QueryContext(r.Context(), funnelBillingQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := make(map[string]MatchingSupplierBilling)
	for rows.Next() {
		var vat sql.NullString
		var b MatchingSupplierBilling
		var median, repeat, period, code sql.NullFloat64
		if err := rows.Scan(&vat, &b.Documents, &b.Months, &median, &repeat, &period, &code); err != nil {
			return nil, err
		}
		b.MedianLines, b.RepeatShare, b.PeriodShare, b.CodeShare = median.Float64, repeat.Float64, period.Float64, code.Float64
		if b.Months > 0 {
			b.PerMonth = float64(b.Documents) / float64(b.Months)
		}
		if k := fiscalKey(vat.String); k != "" {
			profiles[k] = b
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	billingCache.count, billingCache.profiles = count, profiles
	return profiles, nil
}

// supplierBilling picks the profile of an invoice's supplier by any of its
// fiscal identifiers.
func supplierBilling(profiles map[string]MatchingSupplierBilling, invoice funnelInvoice) *MatchingSupplierBilling {
	for k := range fiscalKeys(invoice.supplierVAT, invoice.supplierVATForeign, invoice.fiscalCode) {
		if b, ok := profiles[k]; ok {
			return &b
		}
	}
	return nil
}
