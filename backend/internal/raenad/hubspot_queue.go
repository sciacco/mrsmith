package raenad

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	hubSpotOperationCreateDeal = "hubspot.create_deal"
	hubSpotOperationUpdateDeal = "hubspot.update_deal"
	hubSpotOperationAttachPDF  = "hubspot.attach_pdf"

	hubSpotQueueEntityQuote          = "raenad.quote"
	hubSpotQueueEntityQuotePDFExport = "raenad.quote_pdf_export"

	hubSpotRequestStatusPending   = "pending"
	hubSpotRequestStatusLocked    = "locked"
	hubSpotRequestStatusSucceeded = "succeeded"
	hubSpotRequestStatusFailed    = "failed"
	hubSpotRequestStatusDead      = "dead"
	hubSpotRequestStatusCancelled = "cancelled"

	quoteEventHubSpotCreateEnqueued    = "hubspot_create_enqueued"
	quoteEventHubSpotUpdateEnqueued    = "hubspot_update_enqueued"
	quoteEventHubSpotRetryEnqueued     = "hubspot_retry_enqueued"
	quoteEventHubSpotPDFAttachEnqueued = "hubspot_pdf_attach_enqueued"
	quoteEventHubSpotEnqueueFailed     = "hubspot_enqueue_failed"
)

type hubSpotQueueStore struct {
	db *sql.DB
}

type hubSpotQueueResult struct {
	RequestID int64
	Status    string
	Action    string
}

type hubSpotQueuePayload struct {
	Source                string              `json:"source"`
	Operation             string              `json:"operation"`
	QuoteID               int64               `json:"quote_id"`
	QuoteNumber           string              `json:"quote_number"`
	SourceQuoteUpdatedAt  string              `json:"source_quote_updated_at"`
	Customer              customerSnapshot    `json:"customer"`
	Contact               contactSnapshot     `json:"contact"`
	Payment               paymentSnapshot     `json:"payment"`
	HubSpotCompanyID      *string             `json:"hubspot_company_id"`
	HubSpotContactID      *string             `json:"hubspot_contact_id"`
	HubSpotDealID         *string             `json:"hubspot_deal_id,omitempty"`
	HubSpotPipelineID     *string             `json:"hubspot_pipeline_id,omitempty"`
	HubSpotDealstageID    *string             `json:"hubspot_dealstage_id,omitempty"`
	ExportID              int64               `json:"export_id,omitempty"`
	ExportRevision        int                 `json:"export_revision,omitempty"`
	ExportFilename        string              `json:"export_filename,omitempty"`
	ExportContentType     string              `json:"export_content_type,omitempty"`
	ExportChecksumSHA256  *string             `json:"export_checksum_sha256,omitempty"`
	SourceExportCreatedAt string              `json:"source_export_created_at,omitempty"`
	DocumentDate          *string             `json:"document_date"`
	Description           *string             `json:"description"`
	TotalNet              string              `json:"total_net"`
	TotalVAT              string              `json:"total_vat"`
	TotalGross            string              `json:"total_gross"`
	Lines                 []quoteLine         `json:"lines"`
	Actor                 hubSpotPayloadActor `json:"actor"`
}

type hubSpotPayloadActor struct {
	Subject string `json:"subject"`
	Email   string `json:"email,omitempty"`
	Name    string `json:"name,omitempty"`
}

func newHubSpotQueueStore(db *sql.DB) *hubSpotQueueStore {
	if db == nil {
		return nil
	}
	return &hubSpotQueueStore{db: db}
}

func hubSpotCreateDealDedupeKey(quoteID int64) string {
	return fmt.Sprintf("raenad:quote:%d:deal:create", quoteID)
}

func hubSpotUpdateDealDedupeKey(quoteID int64) string {
	return fmt.Sprintf("raenad:quote:%d:deal:update", quoteID)
}

func hubSpotAttachPDFDedupeKey(quoteID, exportID int64) string {
	return fmt.Sprintf("raenad:quote:%d:pdf-export:%d:attach", quoteID, exportID)
}

func (s *hubSpotQueueStore) enqueueCreateDeal(ctx context.Context, quote quoteResponse, actor actor, retry bool) (hubSpotQueueResult, error) {
	return s.enqueue(ctx, hubSpotOperationCreateDeal, hubSpotCreateDealDedupeKey(quote.ID), quote, actor, retry)
}

func (s *hubSpotQueueStore) enqueueUpdateDeal(ctx context.Context, quote quoteResponse, actor actor, retry bool) (hubSpotQueueResult, error) {
	return s.enqueue(ctx, hubSpotOperationUpdateDeal, hubSpotUpdateDealDedupeKey(quote.ID), quote, actor, retry)
}

func (s *hubSpotQueueStore) enqueueAttachPDF(ctx context.Context, quote quoteResponse, export pdfExport, actor actor) (hubSpotQueueResult, error) {
	if s == nil || s.db == nil {
		return hubSpotQueueResult{}, errors.New("raenad hubspot queue database not configured")
	}
	rawPayload, err := json.Marshal(hubSpotPDFAttachPayload(quote, export, actor))
	if err != nil {
		return hubSpotQueueResult{}, fmt.Errorf("marshal hubspot pdf attach payload: %w", err)
	}
	return s.enqueueAttach(ctx, hubSpotAttachPDFDedupeKey(quote.ID, export.ID), export.ID, rawPayload)
}

func (s *hubSpotQueueStore) enqueue(ctx context.Context, operation, dedupeKey string, quote quoteResponse, actor actor, retry bool) (hubSpotQueueResult, error) {
	if s == nil || s.db == nil {
		return hubSpotQueueResult{}, errors.New("raenad hubspot queue database not configured")
	}

	rawPayload, err := json.Marshal(hubSpotPayload(operation, quote, actor))
	if err != nil {
		return hubSpotQueueResult{}, fmt.Errorf("marshal hubspot queue payload: %w", err)
	}

	if operation == hubSpotOperationCreateDeal && !retry {
		return s.enqueueCreate(ctx, operation, dedupeKey, quote.ID, rawPayload)
	}
	if retry {
		return s.enqueueRetry(ctx, operation, dedupeKey, quote.ID, rawPayload)
	}
	return s.enqueueLatest(ctx, operation, dedupeKey, quote.ID, rawPayload)
}

func (s *hubSpotQueueStore) enqueueAttach(ctx context.Context, dedupeKey string, exportID int64, payload []byte) (hubSpotQueueResult, error) {
	var result hubSpotQueueResult
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO mrsmith.hubspot_request (
			status, operation, entity_type, entity_id, dedupe_key, payload, next_attempt_at
		)
		VALUES ('pending', $1, $2, $3, $4, $5::jsonb, now())
		ON CONFLICT (dedupe_key) DO NOTHING
		RETURNING id, status
	`, hubSpotOperationAttachPDF, hubSpotQueueEntityQuotePDFExport, strconv.FormatInt(exportID, 10), dedupeKey, string(payload)).Scan(&result.RequestID, &result.Status)
	if err == nil {
		result.Action = "inserted"
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return hubSpotQueueResult{}, fmt.Errorf("enqueue hubspot pdf attach: %w", err)
	}
	return s.loadExisting(ctx, dedupeKey, "exists")
}

func (s *hubSpotQueueStore) enqueueCreate(ctx context.Context, operation, dedupeKey string, quoteID int64, payload []byte) (hubSpotQueueResult, error) {
	var result hubSpotQueueResult
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO mrsmith.hubspot_request (
			status, operation, entity_type, entity_id, dedupe_key, payload, next_attempt_at
		)
		VALUES ('pending', $1, $2, $3, $4, $5::jsonb, now())
		ON CONFLICT (dedupe_key) DO NOTHING
		RETURNING id, status
	`, operation, hubSpotQueueEntityQuote, strconv.FormatInt(quoteID, 10), dedupeKey, string(payload)).Scan(&result.RequestID, &result.Status)
	if err == nil {
		result.Action = "inserted"
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return hubSpotQueueResult{}, fmt.Errorf("enqueue hubspot create deal: %w", err)
	}
	return s.loadExisting(ctx, dedupeKey, "exists")
}

func (s *hubSpotQueueStore) enqueueLatest(ctx context.Context, operation, dedupeKey string, quoteID int64, payload []byte) (hubSpotQueueResult, error) {
	var result hubSpotQueueResult
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO mrsmith.hubspot_request (
			status, operation, entity_type, entity_id, dedupe_key, payload, attempt_count,
			next_attempt_at, locked_at, locked_by, last_error, response
		)
		VALUES ('pending', $1, $2, $3, $4, $5::jsonb, 0, now(), NULL, NULL, NULL, NULL)
		ON CONFLICT (dedupe_key) DO UPDATE
		SET payload = EXCLUDED.payload,
		    status = 'pending',
		    attempt_count = 0,
		    next_attempt_at = now(),
		    locked_at = NULL,
		    locked_by = NULL,
		    last_error = NULL,
		    response = NULL
		WHERE mrsmith.hubspot_request.status IN ('pending', 'failed', 'succeeded')
		RETURNING id, status
	`, operation, hubSpotQueueEntityQuote, strconv.FormatInt(quoteID, 10), dedupeKey, string(payload)).Scan(&result.RequestID, &result.Status)
	if err == nil {
		result.Action = "latest_wins"
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return hubSpotQueueResult{}, fmt.Errorf("enqueue hubspot update deal: %w", err)
	}
	return s.loadExisting(ctx, dedupeKey, "unchanged")
}

func (s *hubSpotQueueStore) enqueueRetry(ctx context.Context, operation, dedupeKey string, quoteID int64, payload []byte) (hubSpotQueueResult, error) {
	var result hubSpotQueueResult
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO mrsmith.hubspot_request (
			status, operation, entity_type, entity_id, dedupe_key, payload, attempt_count,
			next_attempt_at, locked_at, locked_by, last_error, response
		)
		VALUES ('pending', $1, $2, $3, $4, $5::jsonb, 0, now(), NULL, NULL, NULL, NULL)
		ON CONFLICT (dedupe_key) DO UPDATE
		SET payload = EXCLUDED.payload,
		    status = 'pending',
		    attempt_count = 0,
		    next_attempt_at = now(),
		    locked_at = NULL,
		    locked_by = NULL,
		    last_error = NULL,
		    response = NULL
		WHERE mrsmith.hubspot_request.status <> 'locked'
		RETURNING id, status
	`, operation, hubSpotQueueEntityQuote, strconv.FormatInt(quoteID, 10), dedupeKey, string(payload)).Scan(&result.RequestID, &result.Status)
	if err == nil {
		result.Action = "retry_rearmed"
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return hubSpotQueueResult{}, fmt.Errorf("retry hubspot queue request: %w", err)
	}
	return s.loadExisting(ctx, dedupeKey, "locked")
}

func (s *hubSpotQueueStore) loadExisting(ctx context.Context, dedupeKey, action string) (hubSpotQueueResult, error) {
	var result hubSpotQueueResult
	err := s.db.QueryRowContext(ctx, `
		SELECT id, status
		FROM mrsmith.hubspot_request
		WHERE dedupe_key = $1
	`, dedupeKey).Scan(&result.RequestID, &result.Status)
	if err != nil {
		return hubSpotQueueResult{}, fmt.Errorf("load existing hubspot queue request: %w", err)
	}
	result.Action = action
	return result, nil
}

func hubSpotPayload(operation string, quote quoteResponse, actor actor) hubSpotQueuePayload {
	payload := hubSpotQueuePayload{
		Source:               "raenad",
		Operation:            operation,
		QuoteID:              quote.ID,
		QuoteNumber:          quote.QuoteNumber,
		SourceQuoteUpdatedAt: quote.UpdatedAt,
		Customer:             quote.Customer,
		Contact:              quote.Contact,
		Payment:              quote.Payment,
		HubSpotCompanyID:     quote.HubSpotCompanyID,
		HubSpotContactID:     quote.HubSpotContactID,
		HubSpotDealID:        quote.HubSpotDealID,
		HubSpotPipelineID:    quote.HubSpotPipelineID,
		HubSpotDealstageID:   quote.HubSpotDealstageID,
		DocumentDate:         quote.DocumentDate,
		Description:          quote.Description,
		TotalNet:             quote.TotalNet,
		TotalVAT:             quote.TotalVAT,
		TotalGross:           quote.TotalGross,
		Lines:                quote.Lines,
		Actor: hubSpotPayloadActor{
			Subject: strings.TrimSpace(actor.Subject),
			Email:   strings.TrimSpace(actor.Email),
			Name:    strings.TrimSpace(actor.Name),
		},
	}
	if operation == hubSpotOperationUpdateDeal {
		payload.HubSpotPipelineID = nil
		payload.HubSpotDealstageID = nil
	}
	return payload
}

func hubSpotPDFAttachPayload(quote quoteResponse, export pdfExport, actor actor) hubSpotQueuePayload {
	return hubSpotQueuePayload{
		Source:                "raenad",
		Operation:             hubSpotOperationAttachPDF,
		QuoteID:               quote.ID,
		QuoteNumber:           quote.QuoteNumber,
		HubSpotDealID:         quote.HubSpotDealID,
		ExportID:              export.ID,
		ExportRevision:        export.Revision,
		ExportFilename:        export.Filename,
		ExportContentType:     export.ContentType,
		ExportChecksumSHA256:  export.ChecksumSHA256,
		SourceExportCreatedAt: export.CreatedAt,
		Actor: hubSpotPayloadActor{
			Subject: strings.TrimSpace(actor.Subject),
			Email:   strings.TrimSpace(actor.Email),
			Name:    strings.TrimSpace(actor.Name),
		},
	}
}

func (h *Handler) appendQuoteEventCommitted(ctx context.Context, quoteID int64, eventType, actorSubject string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = h.deps.Mistra.ExecContext(ctx, `
		INSERT INTO raenad.quote_event (quote_id, event_type, actor_subject, payload)
		VALUES ($1, $2, $3, $4::jsonb)`, quoteID, eventType, actorSubject, string(raw))
	return err
}

func hubSpotQueueEventPayload(operation, dedupeKey string, result hubSpotQueueResult) map[string]any {
	return map[string]any{
		"operation":          operation,
		"dedupe_key":         dedupeKey,
		"hubspot_request_id": result.RequestID,
		"queue_status":       result.Status,
		"queue_action":       result.Action,
	}
}

func hubSpotQueueFailurePayload(operation, dedupeKey string, err error) map[string]any {
	message := "hubspot queue enqueue failed"
	if err != nil {
		message = err.Error()
	}
	return map[string]any{
		"operation":  operation,
		"dedupe_key": dedupeKey,
		"error":      message,
		"failed_at":  time.Now().UTC().Format(time.RFC3339Nano),
	}
}
