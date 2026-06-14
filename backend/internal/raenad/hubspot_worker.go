package raenad

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/hubspot"
)

const (
	defaultHubSpotQueueWorkerInterval      = time.Minute
	defaultHubSpotQueueWorkerBatchSize     = 10
	defaultHubSpotQueueWorkerMaxAttempts   = 5
	defaultHubSpotQueueWorkerBackoffBase   = time.Minute
	defaultHubSpotQueueWorkerBackoffMax    = time.Hour
	defaultHubSpotQueueWorkerLockTimeout   = 15 * time.Minute
	maxHubSpotQueueWorkerPersistedErrBytes = 512
	hubSpotQueueWorkerLockKey              = int64(44000062)

	hubSpotQueueWorkerConfigNamespace = "raenad"
	hubSpotQueueWorkerConfigKey       = "hubspot_queue_worker"
	hubSpotDealOwnerConfigKey         = "hubspot_deal_owner"
	hubSpotPDFExportFolderPath        = "/mrsmith/raenad/quote-pdfs"

	quoteEventHubSpotSyncSucceeded = "hubspot_sync_succeeded"
	quoteEventHubSpotSyncFailed    = "hubspot_sync_failed"
	quoteEventHubSpotUpdateRearmed = "hubspot_update_rearmed"
)

// HubSpotDealClient is the HubSpot subset needed by Raenad queue processing.
type HubSpotDealClient interface {
	CreateDeal(ctx context.Context, properties map[string]any, associations []hubspot.ObjectAssociation) (*hubspot.CRMObject, error)
	UpdateDeal(ctx context.Context, dealID string, properties map[string]any) (*hubspot.CRMObject, error)
	AssociateDealToCompany(ctx context.Context, dealID, companyID string) error
	AssociateDealToContact(ctx context.Context, dealID, contactID string) error
	UploadFile(ctx context.Context, filename string, content []byte, folderPath string, options map[string]any) (*hubspot.UploadedFile, error)
	CreateGenericNoteWithAttachment(ctx context.Context, req hubspot.NoteWithAttachmentRequest) (string, error)
}

type HubSpotQueueWorkerDeps struct {
	Mistra   *sql.DB
	Anisetta *sql.DB
	HubSpot  HubSpotDealClient
	Carbone  PDFRenderer
	Logger   *slog.Logger
}

type HubSpotQueueWorker struct {
	mistra    *sql.DB
	anisetta  *sql.DB
	hs        HubSpotDealClient
	carbone   PDFRenderer
	logger    *slog.Logger
	now       func() time.Time
	workerID  string
	lockKey   int64
	lockLabel string
}

type HubSpotQueueWorkerStats struct {
	Claimed        int
	Succeeded      int
	Retried        int
	Dead           int
	Recovered      int
	Reconciled     int
	Errors         int
	LockSkipped    bool
	ConfigDisabled bool
	Duration       time.Duration
}

type hubSpotQueueWorkerConfig struct {
	Enabled     bool
	Interval    time.Duration
	BatchSize   int
	MaxAttempts int
	BackoffBase time.Duration
	BackoffMax  time.Duration
	LockTimeout time.Duration
}

type hubSpotQueueWorkerConfigPayload struct {
	Enabled            *bool `json:"enabled"`
	IntervalSeconds    int   `json:"interval_seconds"`
	BatchSize          int   `json:"batch_size"`
	MaxAttempts        int   `json:"max_attempts"`
	BackoffBaseSeconds int   `json:"backoff_base_seconds"`
	BackoffMaxSeconds  int   `json:"backoff_max_seconds"`
	LockTimeoutSeconds int   `json:"lock_timeout_seconds"`
}

type hubSpotDealOwnerConfigPayload struct {
	FallbackOwnerEmail string `json:"fallback_owner_email"`
}

type claimedHubSpotRequest struct {
	ID           int64
	Operation    string
	EntityID     string
	DedupeKey    string
	Payload      []byte
	AttemptCount int
}

type staleHubSpotRequest struct {
	ID           int64
	EntityID     string
	Operation    string
	Payload      []byte
	AttemptCount int
}

func NewHubSpotQueueWorker(deps HubSpotQueueWorkerDeps) *HubSpotQueueWorker {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown-host"
	}
	return &HubSpotQueueWorker{
		mistra:    deps.Mistra,
		anisetta:  deps.Anisetta,
		hs:        deps.HubSpot,
		carbone:   deps.Carbone,
		logger:    logger.With("component", component, "worker", "hubspot_queue"),
		now:       time.Now,
		workerID:  fmt.Sprintf("%s:%d", host, os.Getpid()),
		lockKey:   hubSpotQueueWorkerLockKey,
		lockLabel: "raenad_hubspot_queue",
	}
}

func (w *HubSpotQueueWorker) Run(ctx context.Context) {
	if w == nil || w.mistra == nil || w.anisetta == nil || w.hs == nil {
		return
	}
	w.logger.Info("raenad hubspot queue worker started")
	defer w.logger.Info("raenad hubspot queue worker stopped")

	for {
		cfg := w.loadRuntimeConfig(ctx)
		if cfg.Enabled {
			if _, err := w.processOnceWithConfig(ctx, cfg); err != nil && ctx.Err() == nil {
				w.logger.Warn("raenad hubspot queue worker run failed", "operation", "raenad_hubspot_queue_run", "error", err)
			}
		} else {
			w.logger.Info("raenad hubspot queue worker disabled by runtime config")
		}

		timer := time.NewTimer(cfg.Interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
		}
	}
}

func (w *HubSpotQueueWorker) ProcessOnce(ctx context.Context) (HubSpotQueueWorkerStats, error) {
	if w == nil || w.mistra == nil || w.anisetta == nil || w.hs == nil {
		return HubSpotQueueWorkerStats{}, nil
	}
	cfg := w.loadRuntimeConfig(ctx)
	if !cfg.Enabled {
		w.logger.Info("raenad hubspot queue worker disabled by runtime config")
		return HubSpotQueueWorkerStats{ConfigDisabled: true}, nil
	}
	return w.processOnceWithConfig(ctx, cfg)
}

func (w *HubSpotQueueWorker) processOnceWithConfig(ctx context.Context, cfg hubSpotQueueWorkerConfig) (HubSpotQueueWorkerStats, error) {
	started := w.now()
	stats := HubSpotQueueWorkerStats{}

	conn, err := w.anisetta.Conn(ctx)
	if err != nil {
		return stats, fmt.Errorf("open anisetta connection: %w", err)
	}
	defer conn.Close()

	locked, err := w.tryAdvisoryLock(ctx, conn)
	if err != nil {
		return stats, err
	}
	if !locked {
		stats.LockSkipped = true
		stats.Duration = w.now().Sub(started)
		w.logger.Info("raenad hubspot queue skipped because another worker owns the advisory lock")
		return stats, nil
	}
	defer w.releaseAdvisoryLock(conn)

	recovered, err := w.recoverStaleLocked(ctx, conn, cfg)
	if err != nil {
		return stats, err
	}
	stats.Recovered = recovered

	reconciled, err := w.reconcilePendingQuotes(ctx)
	if err != nil {
		return stats, err
	}
	stats.Reconciled = reconciled

	requests, err := claimDueHubSpotRequests(ctx, conn, cfg.BatchSize, w.workerID)
	if err != nil {
		return stats, err
	}
	stats.Claimed = len(requests)
	for _, req := range requests {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		result := w.processRequest(ctx, req, cfg)
		switch result {
		case hubSpotRequestStatusSucceeded:
			stats.Succeeded++
		case hubSpotRequestStatusPending:
			stats.Retried++
		case hubSpotRequestStatusDead:
			stats.Dead++
		default:
			stats.Errors++
		}
	}

	stats.Duration = w.now().Sub(started)
	w.logger.Info(
		"raenad hubspot queue completed",
		"claimed", stats.Claimed,
		"succeeded", stats.Succeeded,
		"retried", stats.Retried,
		"dead", stats.Dead,
		"recovered", stats.Recovered,
		"reconciled", stats.Reconciled,
		"errors", stats.Errors,
		"duration_ms", stats.Duration.Milliseconds(),
	)
	return stats, nil
}

func (w *HubSpotQueueWorker) processRequest(ctx context.Context, req claimedHubSpotRequest, cfg hubSpotQueueWorkerConfig) string {
	var payload hubSpotQueuePayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		msg := sanitizeHubSpotWorkerError(err)
		w.finishDead(ctx, req, hubSpotQueuePayload{}, "invalid_payload", msg)
		return hubSpotRequestStatusDead
	}

	var (
		response map[string]any
		err      error
	)
	switch req.Operation {
	case hubSpotOperationCreateDeal:
		response, err = w.processCreateDeal(ctx, req, payload)
	case hubSpotOperationUpdateDeal:
		response, err = w.processUpdateDeal(ctx, req, payload)
	case hubSpotOperationAttachPDF:
		response, err = w.processAttachPDF(ctx, payload)
	default:
		err = fmt.Errorf("unsupported hubspot request operation %q", req.Operation)
	}
	if err == nil {
		if finished, _ := response["request_finished"].(bool); finished {
			return hubSpotRequestStatusSucceeded
		}
		if finishErr := markHubSpotRequestSucceeded(ctx, w.anisetta, req, response); finishErr != nil {
			w.logger.Warn("mark hubspot request succeeded failed", "operation", "raenad_hubspot_queue_finish", "request_id", req.ID, "error", finishErr)
			return ""
		}
		return hubSpotRequestStatusSucceeded
	}

	msg := sanitizeHubSpotWorkerError(err)
	if req.AttemptCount >= cfg.MaxAttempts {
		w.finishDead(ctx, req, payload, "max_attempts_exhausted", msg)
		return hubSpotRequestStatusDead
	}
	if finishErr := rearmHubSpotRequest(ctx, w.anisetta, req, w.nextAttemptDelay(req.AttemptCount, cfg), msg); finishErr != nil {
		w.logger.Warn("rearm hubspot request failed", "operation", "raenad_hubspot_queue_rearm", "request_id", req.ID, "error", finishErr)
		return ""
	}
	return hubSpotRequestStatusPending
}

func (w *HubSpotQueueWorker) processCreateDeal(ctx context.Context, req claimedHubSpotRequest, payload hubSpotQueuePayload) (map[string]any, error) {
	quote, err := w.loadQuote(ctx, payload.QuoteID)
	if err != nil {
		return nil, err
	}
	if dealID := strings.TrimSpace(deref(quote.HubSpotDealID)); dealID != "" {
		if err := w.markQuoteHubSpotSucceeded(ctx, quote.ID, dealID, "reconciled_existing_deal"); err != nil {
			return nil, err
		}
		return map[string]any{"action": "reconciled_existing_deal", "hubspot_deal_id": dealID}, nil
	}

	ownerID, err := w.resolveHubSpotOwnerID(ctx, payload.Actor.Email)
	if err != nil {
		return nil, err
	}
	properties, err := hubSpotDealProperties(quote, ownerID, true)
	if err != nil {
		return nil, err
	}
	companyID := strings.TrimSpace(deref(quote.HubSpotCompanyID))
	if companyID == "" {
		return nil, errors.New("missing hubspot company id")
	}
	associations := []hubspot.ObjectAssociation{
		hubspot.NewObjectAssociation(companyID, hubspot.AssocTypeDealToCompany),
	}
	if contactID := strings.TrimSpace(deref(quote.HubSpotContactID)); contactID != "" {
		associations = append(associations, hubspot.NewObjectAssociation(contactID, hubspot.AssocTypeDealToContact))
	}

	deal, err := w.hs.CreateDeal(ctx, properties, associations)
	if err != nil {
		return nil, err
	}
	dealID := strings.TrimSpace(deal.ID)
	if dealID == "" {
		return nil, errors.New("hubspot create deal returned empty id")
	}
	if err := w.markQuoteHubSpotSucceeded(ctx, quote.ID, dealID, "created_deal"); err != nil {
		return nil, err
	}
	return map[string]any{"action": "created_deal", "hubspot_deal_id": dealID}, nil
}

func (w *HubSpotQueueWorker) processUpdateDeal(ctx context.Context, req claimedHubSpotRequest, payload hubSpotQueuePayload) (map[string]any, error) {
	quote, err := w.loadQuote(ctx, payload.QuoteID)
	if err != nil {
		return nil, err
	}
	dealID := strings.TrimSpace(deref(quote.HubSpotDealID))
	if dealID == "" {
		return nil, errors.New("hubspot deal id missing; waiting for create request")
	}

	ownerID, err := w.resolveHubSpotOwnerID(ctx, payload.Actor.Email)
	if err != nil {
		return nil, err
	}
	properties, err := hubSpotDealProperties(quote, ownerID, false)
	if err != nil {
		return nil, err
	}
	if _, err := w.hs.UpdateDeal(ctx, dealID, properties); err != nil {
		return nil, err
	}
	if companyID := strings.TrimSpace(deref(quote.HubSpotCompanyID)); companyID != "" {
		if err := w.hs.AssociateDealToCompany(ctx, dealID, companyID); err != nil {
			return nil, err
		}
	}
	if contactID := strings.TrimSpace(deref(quote.HubSpotContactID)); contactID != "" {
		if err := w.hs.AssociateDealToContact(ctx, dealID, contactID); err != nil {
			return nil, err
		}
	}
	latestQuote, err := w.loadQuote(ctx, payload.QuoteID)
	if err != nil {
		return nil, err
	}
	if err := w.markQuoteHubSpotSucceeded(ctx, latestQuote.ID, dealID, "updated_deal"); err != nil {
		return nil, err
	}

	if sourceQuoteStale(payload.SourceQuoteUpdatedAt, latestQuote.UpdatedAt) {
		if err := markHubSpotRequestSucceeded(ctx, w.anisetta, req, map[string]any{
			"action":          "updated_deal_stale_payload",
			"hubspot_deal_id": dealID,
		}); err != nil {
			return nil, err
		}
		latest, err := w.loadQuoteDetail(ctx, latestQuote.ID)
		if err != nil {
			return nil, err
		}
		result, err := newHubSpotQueueStore(w.anisetta).enqueueUpdateDeal(ctx, latest, actor{
			Subject: nonEmptyString(payload.Actor.Subject, "raenad_hubspot_queue_worker"),
			Email:   payload.Actor.Email,
			Name:    payload.Actor.Name,
		}, true)
		if err != nil {
			return nil, err
		}
		if err := w.markQuoteHubSpotPending(ctx, latestQuote.ID, "latest_update_rearmed"); err != nil {
			return nil, err
		}
		return map[string]any{
			"request_finished":   true,
			"action":             "updated_deal_rearmed_latest",
			"hubspot_deal_id":    dealID,
			"hubspot_request_id": result.RequestID,
			"queue_action":       result.Action,
		}, nil
	}

	return map[string]any{"action": "updated_deal", "hubspot_deal_id": dealID}, nil
}

func (w *HubSpotQueueWorker) processAttachPDF(ctx context.Context, payload hubSpotQueuePayload) (map[string]any, error) {
	if payload.QuoteID <= 0 {
		return nil, errors.New("pdf attach payload missing quote id")
	}
	if payload.ExportID <= 0 {
		return nil, errors.New("pdf attach payload missing export id")
	}
	quote, err := w.loadQuote(ctx, payload.QuoteID)
	if err != nil {
		return nil, err
	}
	dealID := strings.TrimSpace(deref(quote.HubSpotDealID))
	if dealID == "" {
		return nil, errors.New("hubspot deal id missing for pdf attach")
	}

	h := Handler{deps: Deps{Mistra: w.mistra, ConfigDB: w.anisetta}, logger: w.logger}
	export, err := h.loadPDFExport(ctx, w.mistra, payload.QuoteID, payload.ExportID)
	if err != nil {
		return nil, err
	}
	templateID, ok, err := h.resolveRaenadPDFTemplateID(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("raenad pdf template not configured")
	}
	if w.carbone == nil {
		return nil, errors.New("raenad pdf renderer not configured")
	}

	renderPayload := export.RenderPayload
	if len(renderPayload) == 0 {
		renderPayload = json.RawMessage(`{}`)
	}
	pdfBytes, err := w.carbone.GeneratePDF(ctx, templateID, renderPayload)
	if err != nil {
		return nil, err
	}
	upload, err := w.hs.UploadFile(ctx, export.Filename, pdfBytes, hubSpotPDFExportFolderPath, map[string]any{"access": "PRIVATE"})
	if err != nil {
		return nil, err
	}
	fileID := strings.TrimSpace(upload.ID)
	if fileID == "" {
		return nil, errors.New("hubspot upload file returned empty id")
	}

	noteID, err := w.hs.CreateGenericNoteWithAttachment(ctx, hubspot.NoteWithAttachmentRequest{
		TargetObjectType:  hubspot.ObjectTypeDeal,
		TargetObjectID:    dealID,
		AssociationTypeID: hubspot.AssocTypeNoteToDeal,
		AttachmentIDs:     []string{fileID},
		Body:              hubSpotPDFExportNoteBody(quote, export),
		Timestamp:         w.now(),
	})
	if err != nil {
		return nil, err
	}
	noteID = strings.TrimSpace(noteID)
	if noteID == "" {
		return nil, errors.New("hubspot create note returned empty id")
	}
	if err := w.markPDFExportAttached(ctx, payload.QuoteID, payload.ExportID, fileID, noteID, dealID); err != nil {
		return nil, err
	}
	return map[string]any{
		"action":          "attached_pdf",
		"hubspot_file_id": fileID,
		"hubspot_note_id": noteID,
		"hubspot_deal_id": dealID,
	}, nil
}

func (w *HubSpotQueueWorker) loadQuote(ctx context.Context, quoteID int64) (quoteResponse, error) {
	return scanQuote(w.mistra.QueryRowContext(ctx, `SELECT `+quoteScanColumns+` FROM raenad.quote WHERE id = $1`, quoteID))
}

func (w *HubSpotQueueWorker) loadQuoteDetail(ctx context.Context, quoteID int64) (quoteResponse, error) {
	h := Handler{deps: Deps{Mistra: w.mistra}, logger: w.logger}
	return h.loadQuoteDetail(ctx, w.mistra, quoteID)
}

func (w *HubSpotQueueWorker) resolveHubSpotOwnerID(ctx context.Context, actorEmail string) (string, error) {
	if ownerID, err := w.lookupHubSpotOwnerByEmail(ctx, actorEmail); err == nil && ownerID != "" {
		return ownerID, nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	fallbackEmail, err := w.loadFallbackOwnerEmail(ctx)
	if err != nil {
		return "", err
	}
	if fallbackEmail == "" {
		return "", errors.New("hubspot owner unresolved and no fallback owner email configured")
	}
	ownerID, err := w.lookupHubSpotOwnerByEmail(ctx, fallbackEmail)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("fallback hubspot owner email %q not found", fallbackEmail)
	}
	return ownerID, err
}

func (w *HubSpotQueueWorker) lookupHubSpotOwnerByEmail(ctx context.Context, email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", sql.ErrNoRows
	}
	var ownerID string
	err := w.mistra.QueryRowContext(ctx, `
		SELECT id::text
		FROM loader.hubs_owner
		WHERE lower(email) = lower($1) AND archived = false
		ORDER BY id
		LIMIT 1`, email).Scan(&ownerID)
	if err != nil {
		return "", err
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return "", sql.ErrNoRows
	}
	return ownerID, nil
}

func (w *HubSpotQueueWorker) loadFallbackOwnerEmail(ctx context.Context) (string, error) {
	var raw []byte
	err := w.anisetta.QueryRowContext(ctx, `
		SELECT value
		FROM mrsmith.runtime_config
		WHERE namespace = $1 AND key = $2
	`, hubSpotQueueWorkerConfigNamespace, hubSpotDealOwnerConfigKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var payload hubSpotDealOwnerConfigPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("parse hubspot deal owner config: %w", err)
	}
	return strings.TrimSpace(payload.FallbackOwnerEmail), nil
}

func hubSpotDealProperties(quote quoteResponse, ownerID string, includeStage bool) (map[string]any, error) {
	documentDate, err := time.Parse(dateLayout, strings.TrimSpace(deref(quote.DocumentDate)))
	if err != nil {
		return nil, fmt.Errorf("invalid quote document date: %w", err)
	}
	customerName := strings.TrimSpace(deref(quote.Customer.Name))
	if customerName == "" {
		customerName = strings.TrimSpace(deref(quote.CustomerName))
	}
	properties := map[string]any{
		"dealname":         strings.TrimSpace(quote.QuoteNumber + " - " + customerName),
		"amount":           strings.TrimSpace(quote.TotalNet),
		"closedate":        documentDate.AddDate(0, 0, 30).Format(dateLayout),
		"hubspot_owner_id": strings.TrimSpace(ownerID),
	}
	if includeStage {
		if pipelineID := strings.TrimSpace(deref(quote.HubSpotPipelineID)); pipelineID != "" {
			properties["pipeline"] = pipelineID
		}
		if dealstageID := strings.TrimSpace(deref(quote.HubSpotDealstageID)); dealstageID != "" {
			properties["dealstage"] = dealstageID
		}
	}
	return properties, nil
}

func (w *HubSpotQueueWorker) markQuoteHubSpotSucceeded(ctx context.Context, quoteID int64, dealID, action string) error {
	_, err := w.mistra.ExecContext(ctx, `
		UPDATE raenad.quote
		SET hubspot_deal_id = $1,
			hubspot_sync_status = 'succeeded',
			hubspot_sync_error = NULL,
			hubspot_synced_at = now()
		WHERE id = $2`, dealID, quoteID)
	if err != nil {
		return err
	}
	return w.appendWorkerQuoteEvent(ctx, quoteID, quoteEventHubSpotSyncSucceeded, map[string]any{
		"action":          action,
		"hubspot_deal_id": dealID,
	})
}

func (w *HubSpotQueueWorker) markQuoteHubSpotPending(ctx context.Context, quoteID int64, action string) error {
	_, err := w.mistra.ExecContext(ctx, `
		UPDATE raenad.quote
		SET hubspot_sync_status = 'pending',
			hubspot_sync_error = NULL,
			hubspot_synced_at = NULL
		WHERE id = $1`, quoteID)
	if err != nil {
		return err
	}
	return w.appendWorkerQuoteEvent(ctx, quoteID, quoteEventHubSpotUpdateRearmed, map[string]any{"action": action})
}

func (w *HubSpotQueueWorker) markQuoteHubSpotFailed(ctx context.Context, quoteID int64, requestID int64, operation, msg string) error {
	_, err := w.mistra.ExecContext(ctx, `
		UPDATE raenad.quote
		SET hubspot_sync_status = 'failed',
			hubspot_sync_error = $1,
			hubspot_synced_at = NULL
		WHERE id = $2`, msg, quoteID)
	if err != nil {
		return err
	}
	return w.appendWorkerQuoteEvent(ctx, quoteID, quoteEventHubSpotSyncFailed, map[string]any{
		"hubspot_request_id": requestID,
		"operation":          operation,
		"error":              msg,
	})
}

func (w *HubSpotQueueWorker) markPDFExportAttached(ctx context.Context, quoteID, exportID int64, fileID, noteID, dealID string) error {
	_, err := w.mistra.ExecContext(ctx, `
		UPDATE raenad.quote_pdf_export
		SET hubspot_attachment_status = 'attached',
			hubspot_file_id = $1,
			hubspot_note_id = $2,
			hubspot_deal_id = $3,
			hubspot_attached_at = now(),
			hubspot_error = NULL
		WHERE quote_id = $4 AND id = $5`, fileID, noteID, dealID, quoteID, exportID)
	return err
}

func (w *HubSpotQueueWorker) markPDFExportFailed(ctx context.Context, quoteID, exportID int64, msg string) error {
	_, err := w.mistra.ExecContext(ctx, `
		UPDATE raenad.quote_pdf_export
		SET hubspot_attachment_status = 'failed',
			hubspot_attached_at = NULL,
			hubspot_error = $1
		WHERE quote_id = $2 AND id = $3`, msg, quoteID, exportID)
	return err
}

func (w *HubSpotQueueWorker) appendWorkerQuoteEvent(ctx context.Context, quoteID int64, eventType string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = w.mistra.ExecContext(ctx, `
		INSERT INTO raenad.quote_event (quote_id, event_type, actor_subject, payload)
		VALUES ($1, $2, $3, $4::jsonb)`, quoteID, eventType, "raenad_hubspot_queue_worker", string(raw))
	return err
}

func (w *HubSpotQueueWorker) finishDead(ctx context.Context, req claimedHubSpotRequest, payload hubSpotQueuePayload, reason, msg string) {
	if err := markHubSpotRequestDead(ctx, w.anisetta, req, reason, msg); err != nil {
		w.logger.Warn("mark hubspot request dead failed", "operation", "raenad_hubspot_queue_dead", "request_id", req.ID, "error", err)
		return
	}
	if req.Operation == hubSpotOperationAttachPDF {
		if payload.QuoteID <= 0 || payload.ExportID <= 0 {
			payload = hubSpotPayloadFromRequest(req)
		}
		if payload.QuoteID <= 0 || payload.ExportID <= 0 {
			return
		}
		if err := w.markPDFExportFailed(ctx, payload.QuoteID, payload.ExportID, msg); err != nil {
			w.logger.Warn("mark pdf export hubspot attach failed failed", "operation", "raenad_hubspot_queue_pdf_failed", "request_id", req.ID, "quote_id", payload.QuoteID, "export_id", payload.ExportID, "error", err)
		}
		return
	}
	quoteID, err := strconv.ParseInt(strings.TrimSpace(req.EntityID), 10, 64)
	if err != nil || quoteID <= 0 {
		return
	}
	if err := w.markQuoteHubSpotFailed(ctx, quoteID, req.ID, req.Operation, msg); err != nil {
		w.logger.Warn("mark quote hubspot sync failed failed", "operation", "raenad_hubspot_queue_quote_failed", "request_id", req.ID, "quote_id", quoteID, "error", err)
	}
}

func hubSpotPayloadFromRequest(req claimedHubSpotRequest) hubSpotQueuePayload {
	var payload hubSpotQueuePayload
	_ = json.Unmarshal(req.Payload, &payload)
	return payload
}

func hubSpotPDFExportNoteBody(quote quoteResponse, export pdfExport) string {
	parts := []string{"Raenad quote PDF"}
	if quote.QuoteNumber != "" {
		parts = append(parts, quote.QuoteNumber)
	}
	if export.Revision > 0 {
		parts = append(parts, fmt.Sprintf("rev%d", export.Revision))
	}
	if strings.TrimSpace(export.Filename) != "" {
		parts = append(parts, export.Filename)
	}
	return strings.Join(parts, " - ")
}

func (w *HubSpotQueueWorker) recoverStaleLocked(ctx context.Context, conn *sql.Conn, cfg hubSpotQueueWorkerConfig) (int, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT id, entity_id, operation, payload, attempt_count
		FROM mrsmith.hubspot_request
		WHERE (
			(entity_type = 'raenad.quote' AND operation IN ('hubspot.create_deal', 'hubspot.update_deal'))
			OR (entity_type = 'raenad.quote_pdf_export' AND operation = 'hubspot.attach_pdf')
		  )
		  AND status = 'locked'
		  AND locked_at < now() - ($1::int * interval '1 second')
		ORDER BY locked_at, id
	`, int(cfg.LockTimeout.Seconds()))
	if err != nil {
		return 0, fmt.Errorf("load stale hubspot requests: %w", err)
	}
	defer rows.Close()

	stale := []staleHubSpotRequest{}
	for rows.Next() {
		var req staleHubSpotRequest
		if err := rows.Scan(&req.ID, &req.EntityID, &req.Operation, &req.Payload, &req.AttemptCount); err != nil {
			return 0, err
		}
		stale = append(stale, req)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, req := range stale {
		msg := "stale hubspot queue lock recovered"
		if req.AttemptCount >= cfg.MaxAttempts {
			claim := claimedHubSpotRequest{ID: req.ID, EntityID: req.EntityID, Operation: req.Operation, Payload: req.Payload, AttemptCount: req.AttemptCount}
			w.finishDead(ctx, claim, hubSpotPayloadFromRequest(claim), "stale_lock_max_attempts", msg)
			continue
		}
		if _, err := conn.ExecContext(ctx, `
			UPDATE mrsmith.hubspot_request
			SET status = 'pending',
				locked_at = NULL,
				locked_by = NULL,
				last_error = $1,
				next_attempt_at = now()
			WHERE id = $2`, msg, req.ID); err != nil {
			return 0, err
		}
	}
	return len(stale), nil
}

func (w *HubSpotQueueWorker) reconcilePendingQuotes(ctx context.Context) (int, error) {
	rows, err := w.mistra.QueryContext(ctx, `
		SELECT `+quoteScanColumns+`
		FROM raenad.quote
		WHERE hubspot_sync_status = 'pending'
		ORDER BY updated_at, id
		LIMIT 100`)
	if err != nil {
		return 0, fmt.Errorf("load pending raenad quotes for hubspot reconcile: %w", err)
	}
	defer rows.Close()

	quotes := []quoteResponse{}
	for rows.Next() {
		quote, err := scanQuote(rows)
		if err != nil {
			return 0, err
		}
		quotes = append(quotes, quote)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	queue := newHubSpotQueueStore(w.anisetta)
	reconciled := 0
	for _, quote := range quotes {
		if err := ctx.Err(); err != nil {
			return reconciled, err
		}
		if live, err := w.liveRequestExists(ctx, quote.ID); err != nil {
			return reconciled, err
		} else if live {
			continue
		}
		var err error
		if strings.TrimSpace(deref(quote.HubSpotDealID)) == "" {
			_, err = queue.enqueueCreateDeal(ctx, quote, actor{Subject: "raenad_hubspot_queue_reconciler"}, true)
		} else {
			_, err = queue.enqueueUpdateDeal(ctx, quote, actor{Subject: "raenad_hubspot_queue_reconciler"}, true)
		}
		if err != nil {
			return reconciled, err
		}
		reconciled++
	}
	return reconciled, nil
}

func (w *HubSpotQueueWorker) liveRequestExists(ctx context.Context, quoteID int64) (bool, error) {
	var exists bool
	err := w.anisetta.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM mrsmith.hubspot_request
			WHERE entity_type = 'raenad.quote'
			  AND entity_id = $1
			  AND operation IN ('hubspot.create_deal', 'hubspot.update_deal')
			  AND status IN ('pending', 'locked')
		)`, strconv.FormatInt(quoteID, 10)).Scan(&exists)
	return exists, err
}

func (w *HubSpotQueueWorker) loadRuntimeConfig(ctx context.Context) hubSpotQueueWorkerConfig {
	cfg := defaultHubSpotQueueWorkerConfig()
	if w == nil || w.anisetta == nil {
		return cfg
	}
	var raw []byte
	err := w.anisetta.QueryRowContext(ctx, `
		SELECT value
		FROM mrsmith.runtime_config
		WHERE namespace = $1 AND key = $2
	`, hubSpotQueueWorkerConfigNamespace, hubSpotQueueWorkerConfigKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg
	}
	if err != nil {
		w.logger.Warn("raenad hubspot queue runtime config read failed; using defaults", "operation", "raenad_hubspot_queue_config_read", "error", err)
		return cfg
	}
	var payload hubSpotQueueWorkerConfigPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		w.logger.Warn("raenad hubspot queue runtime config parse failed; using defaults", "operation", "raenad_hubspot_queue_config_parse", "error", err)
		return cfg
	}
	if payload.Enabled != nil {
		cfg.Enabled = *payload.Enabled
	}
	if payload.IntervalSeconds > 0 {
		cfg.Interval = time.Duration(payload.IntervalSeconds) * time.Second
	}
	if payload.BatchSize > 0 {
		cfg.BatchSize = payload.BatchSize
	}
	if payload.MaxAttempts > 0 {
		cfg.MaxAttempts = payload.MaxAttempts
	}
	if payload.BackoffBaseSeconds > 0 {
		cfg.BackoffBase = time.Duration(payload.BackoffBaseSeconds) * time.Second
	}
	if payload.BackoffMaxSeconds > 0 {
		cfg.BackoffMax = time.Duration(payload.BackoffMaxSeconds) * time.Second
	}
	if payload.LockTimeoutSeconds > 0 {
		cfg.LockTimeout = time.Duration(payload.LockTimeoutSeconds) * time.Second
	}
	return cfg
}

func defaultHubSpotQueueWorkerConfig() hubSpotQueueWorkerConfig {
	return hubSpotQueueWorkerConfig{
		Enabled:     false,
		Interval:    defaultHubSpotQueueWorkerInterval,
		BatchSize:   defaultHubSpotQueueWorkerBatchSize,
		MaxAttempts: defaultHubSpotQueueWorkerMaxAttempts,
		BackoffBase: defaultHubSpotQueueWorkerBackoffBase,
		BackoffMax:  defaultHubSpotQueueWorkerBackoffMax,
		LockTimeout: defaultHubSpotQueueWorkerLockTimeout,
	}
}

func (w *HubSpotQueueWorker) nextAttemptDelay(attempt int, cfg hubSpotQueueWorkerConfig) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := cfg.BackoffBase
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= cfg.BackoffMax {
			return cfg.BackoffMax
		}
	}
	if delay > cfg.BackoffMax {
		return cfg.BackoffMax
	}
	return delay
}

func (w *HubSpotQueueWorker) tryAdvisoryLock(ctx context.Context, conn *sql.Conn) (bool, error) {
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, w.lockKey).Scan(&locked); err != nil {
		return false, fmt.Errorf("acquire %s advisory lock: %w", w.lockLabel, err)
	}
	return locked, nil
}

func (w *HubSpotQueueWorker) releaseAdvisoryLock(conn *sql.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var unlocked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_advisory_unlock($1)`, w.lockKey).Scan(&unlocked); err != nil {
		w.logger.Warn("release raenad hubspot queue advisory lock failed", "operation", "raenad_hubspot_queue_unlock", "error", err)
		return
	}
	if !unlocked {
		w.logger.Warn("raenad hubspot queue advisory lock was not held during release", "operation", "raenad_hubspot_queue_unlock")
	}
}

func claimDueHubSpotRequests(ctx context.Context, conn *sql.Conn, batchSize int, workerID string) ([]claimedHubSpotRequest, error) {
	if batchSize <= 0 {
		batchSize = defaultHubSpotQueueWorkerBatchSize
	}
	rows, err := conn.QueryContext(ctx, `
		WITH due AS (
			SELECT id
			FROM mrsmith.hubspot_request
			WHERE (
				(entity_type = 'raenad.quote' AND operation IN ('hubspot.create_deal', 'hubspot.update_deal'))
				OR (entity_type = 'raenad.quote_pdf_export' AND operation = 'hubspot.attach_pdf')
			  )
			  AND status = 'pending'
			  AND next_attempt_at <= now()
			ORDER BY next_attempt_at, id
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE mrsmith.hubspot_request req
		SET status = 'locked',
			locked_at = now(),
			locked_by = $2,
			attempt_count = attempt_count + 1
		FROM due
		WHERE req.id = due.id
		RETURNING req.id, req.operation, req.entity_id, req.dedupe_key, req.payload, req.attempt_count
	`, batchSize, workerID)
	if err != nil {
		return nil, fmt.Errorf("claim hubspot requests: %w", err)
	}
	defer rows.Close()

	requests := []claimedHubSpotRequest{}
	for rows.Next() {
		var req claimedHubSpotRequest
		if err := rows.Scan(&req.ID, &req.Operation, &req.EntityID, &req.DedupeKey, &req.Payload, &req.AttemptCount); err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return requests, nil
}

func markHubSpotRequestSucceeded(ctx context.Context, db *sql.DB, req claimedHubSpotRequest, response map[string]any) error {
	raw, _ := json.Marshal(response)
	if err := insertHubSpotRequestAttempt(ctx, db, req, hubSpotRequestStatusSucceeded, string(raw), ""); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `
		UPDATE mrsmith.hubspot_request
		SET status = 'succeeded',
			locked_at = NULL,
			locked_by = NULL,
			last_error = NULL,
			response = $1::jsonb
		WHERE id = $2`, string(raw), req.ID)
	return err
}

func rearmHubSpotRequest(ctx context.Context, db *sql.DB, req claimedHubSpotRequest, delay time.Duration, msg string) error {
	if err := insertHubSpotRequestAttempt(ctx, db, req, hubSpotRequestStatusFailed, "", msg); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `
		UPDATE mrsmith.hubspot_request
		SET status = 'pending',
			locked_at = NULL,
			locked_by = NULL,
			last_error = $1,
			next_attempt_at = now() + ($2::int * interval '1 second')
		WHERE id = $3`, msg, int(delay.Seconds()), req.ID)
	return err
}

func markHubSpotRequestDead(ctx context.Context, db *sql.DB, req claimedHubSpotRequest, reason, msg string) error {
	raw, _ := json.Marshal(map[string]any{"reason": reason})
	if err := insertHubSpotRequestAttempt(ctx, db, req, hubSpotRequestStatusDead, string(raw), msg); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `
		UPDATE mrsmith.hubspot_request
		SET status = 'dead',
			locked_at = NULL,
			locked_by = NULL,
			last_error = $1
		WHERE id = $2`, msg, req.ID)
	return err
}

func insertHubSpotRequestAttempt(ctx context.Context, db *sql.DB, req claimedHubSpotRequest, status, response, errorDetail string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO mrsmith.hubspot_request_attempt (
			request_id, attempt_number, finished_at, status, response, error
		)
		VALUES ($1, $2, now(), $3, NULLIF($4, '')::jsonb, NULLIF($5, ''))`,
		req.ID, req.AttemptCount, status, response, errorDetail)
	return err
}

func sanitizeHubSpotWorkerError(err error) string {
	msg := strings.TrimSpace(fmt.Sprint(err))
	if msg == "" {
		msg = "hubspot queue request failed"
	}
	msg = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		default:
			if r < 32 {
				return -1
			}
			return r
		}
	}, msg)
	msg = strings.Join(strings.Fields(msg), " ")
	if len(msg) > maxHubSpotQueueWorkerPersistedErrBytes {
		msg = msg[:maxHubSpotQueueWorkerPersistedErrBytes]
	}
	return msg
}

func sourceQuoteStale(source, current string) bool {
	source = strings.TrimSpace(source)
	current = strings.TrimSpace(current)
	if source == "" || current == "" {
		return false
	}
	sourceTime, sourceErr := time.Parse(time.RFC3339Nano, source)
	currentTime, currentErr := time.Parse(time.RFC3339Nano, current)
	if sourceErr == nil && currentErr == nil {
		return currentTime.After(sourceTime)
	}
	return source != current
}

func nonEmptyString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
