package statsrda

import "encoding/json"

// PeriodPreset identifies an accepted riepilogo period preset.
type PeriodPreset string

// RiepilogoPeriod is the resolved reporting period, with from inclusive and to exclusive.
type RiepilogoPeriod struct {
	Preset string `json:"preset"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// RiepilogoTotals contains aggregate totals for the riepilogo response.
type RiepilogoTotals struct {
	OrderCount  int     `json:"order_count"`
	BudgetCount int     `json:"budget_count"`
	Amount      float64 `json:"amount"`
}

// RiepilogoBudget is one budget aggregate row.
type RiepilogoBudget struct {
	Budget     string  `json:"budget"`
	OrderCount int     `json:"order_count"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
}

// RiepilogoDetail is one PA purchase order detail row for the selected period.
type RiepilogoDetail struct {
	IssueKey             string  `json:"issue_key"`
	NumeroOrdine         *string `json:"numero_ordine"`
	Summary              string  `json:"summary"`
	BudgetDiRiferimento  string  `json:"budget_di_riferimento"`
	ImportoTotale        float64 `json:"importo_totale"`
	Valuta               *string `json:"valuta"`
	ReporterName         *string `json:"reporter_name"`
	FornitoreSelezionato *string `json:"fornitore_selezionato"`
	Status               *string `json:"status"`
	Resolution           *string `json:"resolution"`
	Created              *string `json:"created"`
}

// RiepilogoResponse is the JSON envelope for /pa/riepilogo.
type RiepilogoResponse struct {
	Period  RiepilogoPeriod   `json:"period"`
	Totals  RiepilogoTotals   `json:"totals"`
	Budgets []RiepilogoBudget `json:"budgets"`
	Details []RiepilogoDetail `json:"details"`
}

// IssueSummary is one row of the list endpoint (/pa/issues).
type IssueSummary struct {
	IssueKey             string   `json:"issue_key"`
	Summary              string   `json:"summary"`
	NumeroOrdine         *string  `json:"numero_ordine"`
	Status               *string  `json:"status"`
	IssueType            *string  `json:"issue_type"`
	ImportoTotale        *float64 `json:"importo_totale"`
	Valuta               *string  `json:"valuta"`
	BudgetDiRiferimento  *string  `json:"budget_di_riferimento"`
	ReporterName         *string  `json:"reporter_name"`
	FornitoreSelezionato *string  `json:"fornitore_selezionato"`
	Created              *string  `json:"created"` // ISO timestamp
}

// IssueListResponse is the paginated envelope for /pa/issues.
type IssueListResponse struct {
	Items []IssueSummary `json:"items"`
	Total int            `json:"total"`
	Page  int            `json:"page"`
	Limit int            `json:"limit"`
}

// FilterOption is a {value, count} pair used by /pa/filters and autocomplete.
type FilterOption struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// FiltersResponse is the shape of /pa/filters.
type FiltersResponse struct {
	Budget    []FilterOption  `json:"budget"`
	Stati     []FilterOption  `json:"stati"`
	Tipi      []FilterOption  `json:"tipi"`
	Valute    []FilterOption  `json:"valute"`
	RangeDate FilterDateRange `json:"range_date"`
}

type FilterDateRange struct {
	Min *string `json:"min"` // ISO date
	Max *string `json:"max"` // ISO date
}

// AutocompleteResponse is the shape of /pa/fornitori and /pa/richiedenti.
type AutocompleteResponse struct {
	Items []FilterOption `json:"items"`
}

// IssueDetail is the full card envelope for /pa/issues/:issueKey.
type IssueDetail struct {
	Issue           IssueHeader           `json:"issue"`
	Purchase        PurchaseSection       `json:"purchase"`
	Description     *string               `json:"description"`
	LineItemsByGrid map[string][]LineItem `json:"line_items_by_grid"`
	Comments        []Comment             `json:"comments"`
	Attachments     []Attachment          `json:"attachments"`
	Links           []IssueLink           `json:"links"`
	History         []HistoryEntry        `json:"history"`
	HistoryTotal    int                   `json:"history_total"`
}

// IssueHeader — section 1 (identity, state, dates, people).
type IssueHeader struct {
	IssueKey       string  `json:"issue_key"`
	Summary        string  `json:"summary"`
	NumeroOrdine   *string `json:"numero_ordine"`
	IssueType      *string `json:"issue_type"`
	Status         *string `json:"status"`
	Stato          *string `json:"stato"` // domain field, != status
	Priority       *string `json:"priority"`
	Resolution     *string `json:"resolution"`
	Valuta         *string `json:"valuta"`
	Created        *string `json:"created"`         // ISO
	Updated        *string `json:"updated"`         // ISO
	ResolutionDate *string `json:"resolution_date"` // ISO
	DueDate        *string `json:"due_date"`        // ISO date
	ReporterName   *string `json:"reporter_name"`
	ReporterEmail  *string `json:"reporter_email"`
	AssigneeName   *string `json:"assignee_name"`
	CreatorName    *string `json:"creator_name"`
}

// PurchaseSection — section 2 (amounts, supplier, budget, approval).
type PurchaseSection struct {
	ImportoTotale           *float64 `json:"importo_totale"`
	ImportoTotaleMerci      *float64 `json:"importo_totale_merci"`
	ImportoTotaleServizi    *float64 `json:"importo_totale_servizi"`
	ImportoTotaleLeasing    *float64 `json:"importo_totale_leasing"`
	FornitoreSelezionato    *string  `json:"fornitore_selezionato"`
	TipoDiOrdine            *string  `json:"tipo_di_ordine"`
	TipoDocumento           *string  `json:"tipo_documento"`
	BudgetDiRiferimento     *string  `json:"budget_di_riferimento"`
	BudgetCorrente          *float64 `json:"budget_corrente"`
	BudgetTotale            *float64 `json:"budget_totale"`
	LimiteApprovazione      *float64 `json:"limite_approvazione"`
	PercentualeApprovazione *float64 `json:"percentuale_approvazione"`
	InviatoInApprovazione   *string  `json:"inviato_in_approvazione"` // ISO
	Approvato               *string  `json:"approvato"`               // ISO
	Ricorrente              *string  `json:"ricorrente"`
}

// LineItem — section 4 row.
type LineItem struct {
	Grid          string   `json:"grid"`
	RowNo         int      `json:"row_no"`
	ArticoloName  *string  `json:"articolo_name"`
	Vendor        *string  `json:"vendor"`
	PartNumber    *string  `json:"part_number"`
	Descrizione   *string  `json:"descrizione"`
	Quantita      *float64 `json:"quantita"`
	Importo       *float64 `json:"importo"`
	PrezzoTotale  *float64 `json:"prezzo_totale"` // null for Articoli -> FE computes quantita*importo
	PagamentoName *string  `json:"pagamento_name"`
	DurataName    *string  `json:"durata_name"`
	DataPagamento *string  `json:"data_pagamento"` // ISO date
	Vendita       *string  `json:"vendita"`
	Rinnovo       *string  `json:"rinnovo"`
}

// Comment — section 5.
type Comment struct {
	Created    *string `json:"created"` // ISO
	AuthorName *string `json:"author_name"`
	Body       *string `json:"body"`
}

// Attachment — section 6 (metadata only).
type Attachment struct {
	Filename   string  `json:"filename"`
	Mimetype   *string `json:"mimetype"`
	Filesize   *int64  `json:"filesize"`
	Created    *string `json:"created"` // ISO
	AuthorName *string `json:"author_name"`
}

// IssueLink — section 7.
type IssueLink struct {
	SourceKey      string  `json:"source_key"`
	DestinationKey string  `json:"destination_key"`
	LinkName       *string `json:"link_name"`
	Inward         *string `json:"inward"`
	Outward        *string `json:"outward"`
}

// HistoryEntry — section 8.
type HistoryEntry struct {
	Created    *string `json:"created"` // ISO
	AuthorName *string `json:"author_name"`
	Field      *string `json:"field"`
	OldString  *string `json:"old_string"`
	NewString  *string `json:"new_string"`
}

// ensure json is referenced even if future helpers marshal manually
var _ = json.Marshal
