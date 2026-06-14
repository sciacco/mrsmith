package raenad

import "encoding/json"

const (
	authoringStatusDraft    = "draft"
	authoringStatusReady    = "ready"
	authoringStatusArchived = "archived"

	hubSpotSyncStatusPending   = "pending"
	hubSpotSyncStatusSucceeded = "succeeded"
	hubSpotSyncStatusFailed    = "failed"

	lineTypeSpacer      = "spacer"
	lineTypeDescription = "description"
	lineTypeItem        = "item"
)

type quoteListResponse struct {
	Items    []quoteSummary `json:"items"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

type quoteSummary struct {
	ID                    int64   `json:"id"`
	QuoteNumber           string  `json:"quote_number"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
	CreatedBy             string  `json:"created_by"`
	UpdatedBy             string  `json:"updated_by"`
	AuthoringStatus       string  `json:"authoring_status"`
	HubSpotSyncStatus     string  `json:"hubspot_sync_status"`
	HubSpotSyncError      *string `json:"hubspot_sync_error"`
	HubSpotSyncedAt       *string `json:"hubspot_synced_at"`
	HubSpotCompanyID      *string `json:"hubspot_company_id"`
	HubSpotContactID      *string `json:"hubspot_contact_id"`
	HubSpotDealID         *string `json:"hubspot_deal_id"`
	HubSpotPipelineID     *string `json:"hubspot_pipeline_id"`
	HubSpotPipelineLabel  *string `json:"hubspot_pipeline_label"`
	HubSpotDealstageID    *string `json:"hubspot_dealstage_id"`
	HubSpotDealstageLabel *string `json:"hubspot_dealstage_label"`
	CustomerName          *string `json:"customer_name"`
	DocumentDate          *string `json:"document_date"`
	Description           *string `json:"description"`
	TotalNet              string  `json:"total_net"`
	TotalVAT              string  `json:"total_vat"`
	TotalGross            string  `json:"total_gross"`
	TotalPurchase         string  `json:"total_purchase"`
	TotalGain             string  `json:"total_gain"`
}

type quoteResponse struct {
	quoteSummary
	Customer      customerSnapshot `json:"customer"`
	Contact       contactSnapshot  `json:"contact"`
	Payment       paymentSnapshot  `json:"payment"`
	InternalNotes *string          `json:"internal_notes"`
	Lines         []quoteLine      `json:"lines"`
	Events        []quoteEvent     `json:"events"`
}

type saveQuoteRequest struct {
	HubSpotCompanyID      *string               `json:"hubspot_company_id"`
	HubSpotContactID      *string               `json:"hubspot_contact_id"`
	HubSpotPipelineID     *string               `json:"hubspot_pipeline_id"`
	HubSpotPipelineLabel  *string               `json:"hubspot_pipeline_label"`
	HubSpotDealstageID    *string               `json:"hubspot_dealstage_id"`
	HubSpotDealstageLabel *string               `json:"hubspot_dealstage_label"`
	Customer              customerSnapshotInput `json:"customer"`
	Contact               contactSnapshotInput  `json:"contact"`
	DocumentDate          *string               `json:"document_date"`
	PaymentMethodCode     *string               `json:"payment_method_code"`
	PaymentMethodLabel    *string               `json:"payment_method_label"`
	PaymentBankDetails    *string               `json:"payment_bank_details"`
	Description           *string               `json:"description"`
	InternalNotes         *string               `json:"internal_notes"`
	Lines                 []quoteLineInput      `json:"lines"`
}

type customerSnapshotInput struct {
	Name                  *string `json:"name"`
	VAT                   *string `json:"vat"`
	TaxCode               *string `json:"tax_code"`
	PEC                   *string `json:"pec"`
	Email                 *string `json:"email"`
	Address               *string `json:"address"`
	ZIP                   *string `json:"zip"`
	City                  *string `json:"city"`
	Province              *string `json:"province"`
	Country               *string `json:"country"`
	Language              *string `json:"language"`
	NumeroAziendaSnapshot *string `json:"numero_azienda_snapshot"`
}

type customerSnapshot struct {
	customerSnapshotInput
}

type contactSnapshotInput struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	FullName  *string `json:"full_name"`
	Email     *string `json:"email"`
	Role      *string `json:"role"`
}

type contactSnapshot struct {
	contactSnapshotInput
}

type paymentSnapshot struct {
	MethodCode  *string `json:"method_code"`
	MethodLabel *string `json:"method_label"`
	BankDetails *string `json:"bank_details"`
}

type quoteLineInput struct {
	Position           int     `json:"position"`
	LineType           string  `json:"line_type"`
	ItemCode           *string `json:"item_code"`
	ItemDescription    *string `json:"item_description"`
	Description        *string `json:"description"`
	UnitOfMeasure      *string `json:"unit_of_measure"`
	Qta                *string `json:"qta"`
	UnitPrice          *string `json:"unit_price"`
	Discounts          *string `json:"discounts"`
	CodIVA             *string `json:"cod_iva"`
	IVAPercentSnapshot *string `json:"iva_percent_snapshot"`
	PurchaseUnitPrice  *string `json:"purchase_unit_price"`
}

type quoteLine struct {
	quoteLineInput
	ID           int64   `json:"id"`
	QuoteID      int64   `json:"quote_id"`
	LineNet      *string `json:"line_net"`
	LineVAT      *string `json:"line_vat"`
	LineGross    *string `json:"line_gross"`
	LinePurchase *string `json:"line_purchase"`
	LineGain     *string `json:"line_gain"`
}

type quoteEvent struct {
	ID           int64           `json:"id"`
	QuoteID      int64           `json:"quote_id"`
	EventType    string          `json:"event_type"`
	ActorSubject string          `json:"actor_subject"`
	Payload      json.RawMessage `json:"payload"`
	CreatedAt    string          `json:"created_at"`
}

type pdfExport struct {
	ID                      int64           `json:"id"`
	QuoteID                 int64           `json:"quote_id"`
	Revision                int             `json:"revision"`
	Filename                string          `json:"filename"`
	ContentType             string          `json:"content_type"`
	ChecksumSHA256          *string         `json:"checksum_sha256"`
	RenderPayload           json.RawMessage `json:"render_payload"`
	CreatedAt               string          `json:"created_at"`
	CreatedBy               string          `json:"created_by"`
	HubSpotAttachmentStatus string          `json:"hubspot_attachment_status"`
	HubSpotFileID           *string         `json:"hubspot_file_id"`
	HubSpotNoteID           *string         `json:"hubspot_note_id"`
	HubSpotDealID           *string         `json:"hubspot_deal_id"`
	HubSpotAttachedAt       *string         `json:"hubspot_attached_at"`
	HubSpotError            *string         `json:"hubspot_error"`
	IsStale                 bool            `json:"is_stale"`
}

type stageTransitionRequest struct {
	ExpectedDealstageID string `json:"expected_dealstage_id"`
	TargetDealstageID   string `json:"target_dealstage_id"`
}
