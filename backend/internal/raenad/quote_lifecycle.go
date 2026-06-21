package raenad

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const (
	defaultQuotePageSize = 50
	maxQuotePageSize     = 100
	dateLayout           = "2006-01-02"
)

type paymentSnapshotResolved struct {
	Code  *string
	Label *string
}

type stageSnapshotResolved struct {
	PipelineID     string
	PipelineLabel  *string
	DealstageID    string
	DealstageLabel *string
}

func (h *Handler) handleQuoteList(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	page := parsePositiveInt(r.URL.Query().Get("page"), 1, 1, 100000)
	pageSize := parsePositiveInt(r.URL.Query().Get("page_size"), defaultQuotePageSize, 1, maxQuotePageSize)
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	authoringStatus := strings.TrimSpace(r.URL.Query().Get("authoring_status"))
	syncStatus := strings.TrimSpace(r.URL.Query().Get("hubspot_sync_status"))

	where := make([]string, 0)
	args := make([]any, 0)
	addArg := func(value any) string {
		args = append(args, value)
		return "$" + strconv.Itoa(len(args))
	}
	if search != "" {
		p := addArg("%" + search + "%")
		where = append(where, "(quote_number ILIKE "+p+" OR COALESCE(customer_name, '') ILIKE "+p+" OR COALESCE(description, '') ILIKE "+p+")")
	}
	if authoringStatus != "" {
		where = append(where, "authoring_status = "+addArg(authoringStatus))
	}
	if syncStatus != "" {
		where = append(where, "hubspot_sync_status = "+addArg(syncStatus))
	}
	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := h.deps.Mistra.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM raenad.quote"+whereClause, args...).Scan(&total); err != nil {
		h.dbFailure(w, r, "quote_list_count", err)
		return
	}

	listArgs := append([]any{}, args...)
	limit := "$" + strconv.Itoa(len(listArgs)+1)
	offset := "$" + strconv.Itoa(len(listArgs)+2)
	listArgs = append(listArgs, pageSize, (page-1)*pageSize)

	rows, err := h.deps.Mistra.QueryContext(r.Context(), `
		SELECT `+quoteScanColumns+`
		FROM raenad.quote`+whereClause+`
		ORDER BY updated_at DESC, id DESC
		LIMIT `+limit+` OFFSET `+offset, listArgs...)
	if err != nil {
		h.dbFailure(w, r, "quote_list", err)
		return
	}
	defer rows.Close()

	items := make([]quoteSummary, 0)
	for rows.Next() {
		item, err := scanQuote(rows)
		if err != nil {
			h.dbFailure(w, r, "quote_list_scan", err)
			return
		}
		items = append(items, item.quoteSummary)
	}
	if !h.rowsDone(w, r, rows, "quote_list") {
		return
	}

	httputil.JSON(w, http.StatusOK, quoteListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func (h *Handler) handleQuoteCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) || !h.requireConfigDB(w) {
		return
	}

	req, ok := h.decodeSaveQuoteRequest(w, r)
	if !ok {
		return
	}

	actor := requestActor(r)
	stage, ok := h.initialStageSnapshot(r.Context())
	if !ok {
		httputil.Error(w, http.StatusServiceUnavailable, "raenad_config_not_configured")
		return
	}
	payment, ok := h.resolvePaymentForSave(w, r, req.PaymentMethodCode, false)
	if !ok {
		return
	}
	lines, ok := h.normalizeLineInputs(w, req.Lines)
	if !ok {
		return
	}
	if !h.validateCreatePayload(w, req) {
		return
	}
	if !h.validateWritableLines(r.Context(), w, r, h.deps.Mistra, lines, false) {
		return
	}

	tx, err := h.deps.Mistra.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "quote_create_begin", err)
		return
	}
	defer tx.Rollback()

	var quoteID int64
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO raenad.quote (
			created_by, updated_by, authoring_status, hubspot_sync_status,
			hubspot_company_id, hubspot_contact_id, hubspot_pipeline_id, hubspot_pipeline_label,
			hubspot_dealstage_id, hubspot_dealstage_label,
			customer_name, customer_vat, customer_tax_code, customer_pec, customer_email,
			customer_address, customer_zip, customer_city, customer_province, customer_country,
			customer_language, numero_azienda_snapshot,
			contact_first_name, contact_last_name, contact_full_name, contact_email, contact_role,
			document_date, payment_method_code, payment_method_label, payment_bank_details,
			description, internal_notes
		) VALUES (
			$1, $2, 'draft', 'pending',
			$3, $4, $5, $6,
			$7, $8,
			$9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18,
			$19, $20,
			$21, $22, $23, $24, $25,
			$26::date, $27, $28, $29,
			$30, $31
		)
		RETURNING id`,
		actor.Subject, actor.Subject,
		trimmedOrNil(req.HubSpotCompanyID), trimmedOrNil(req.HubSpotContactID),
		stage.PipelineID, stage.PipelineLabel, stage.DealstageID, stage.DealstageLabel,
		trimmedOrNil(req.Customer.Name), trimmedOrNil(req.Customer.VAT), trimmedOrNil(req.Customer.TaxCode),
		trimmedOrNil(req.Customer.PEC), trimmedOrNil(req.Customer.Email), trimmedOrNil(req.Customer.Address),
		trimmedOrNil(req.Customer.ZIP), trimmedOrNil(req.Customer.City), trimmedOrNil(req.Customer.Province),
		trimmedOrNil(req.Customer.Country), trimmedOrNil(req.Customer.Language), trimmedOrNil(req.Customer.NumeroAziendaSnapshot),
		trimmedOrNil(req.Contact.FirstName), trimmedOrNil(req.Contact.LastName), trimmedOrNil(req.Contact.FullName),
		trimmedOrNil(req.Contact.Email), trimmedOrNil(req.Contact.Role),
		trimmedOrNil(req.DocumentDate), payment.Code, payment.Label, trimmedOrNil(req.PaymentBankDetails),
		trimmedOrNil(req.Description), trimmedOrNil(req.InternalNotes),
	).Scan(&quoteID)
	if err != nil {
		h.dbFailure(w, r, "quote_create_insert", err)
		return
	}

	if err := h.replaceQuoteLines(r.Context(), tx, quoteID, lines); err != nil {
		h.dbFailure(w, r, "quote_create_lines", err, "quote_id", quoteID)
		return
	}
	if err := h.appendQuoteEvent(r.Context(), tx, quoteID, "created", actor.Subject, map[string]any{"authoring_status": authoringStatusDraft}); err != nil {
		h.dbFailure(w, r, "quote_create_event", err, "quote_id", quoteID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "quote_create_commit", err, "quote_id", quoteID)
		return
	}

	h.enqueueCreateDealPostCommit(r, quoteID, actor)
	h.respondQuoteByID(w, r, quoteID, http.StatusCreated)
}

func (h *Handler) handleQuoteGet(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}
	quoteID, ok := quoteIDFromRequest(w, r)
	if !ok {
		return
	}
	h.respondQuoteByID(w, r, quoteID, http.StatusOK)
}

func (h *Handler) handleQuoteUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}
	quoteID, ok := quoteIDFromRequest(w, r)
	if !ok {
		return
	}
	req, ok := h.decodeSaveQuoteRequest(w, r)
	if !ok {
		return
	}
	actor := requestActor(r)
	payment, ok := h.resolvePaymentForSave(w, r, req.PaymentMethodCode, false)
	if !ok {
		return
	}
	lines, ok := h.normalizeLineInputs(w, req.Lines)
	if !ok {
		return
	}
	if !h.validateWritableLines(r.Context(), w, r, h.deps.Mistra, lines, false) {
		return
	}
	if !h.validateUpdatePayload(w, req) {
		return
	}

	tx, err := h.deps.Mistra.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "quote_update_begin", err, "quote_id", quoteID)
		return
	}
	defer tx.Rollback()

	current, err := h.loadQuoteForUpdate(r.Context(), tx, quoteID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "quote_update_load", err, "quote_id", quoteID)
		return
	}
	currentLines, err := h.loadQuoteLines(r.Context(), tx, quoteID)
	if err != nil {
		h.dbFailure(w, r, "quote_update_lines_load", err, "quote_id", quoteID)
		return
	}

	relevantChanged := quoteRelevantChange(current, currentLines, req, payment, lines)
	nextStatus := current.AuthoringStatus
	if nextStatus == authoringStatusReady && relevantChanged {
		nextStatus = authoringStatusDraft
	}

	_, err = tx.ExecContext(r.Context(), `
		UPDATE raenad.quote SET
			updated_by = $1,
			authoring_status = $2,
			hubspot_company_id = $3,
			hubspot_contact_id = $4,
			customer_name = $5,
			customer_vat = $6,
			customer_tax_code = $7,
			customer_pec = $8,
			customer_email = $9,
			customer_address = $10,
			customer_zip = $11,
			customer_city = $12,
			customer_province = $13,
			customer_country = $14,
			customer_language = $15,
			numero_azienda_snapshot = $16,
			contact_first_name = $17,
			contact_last_name = $18,
			contact_full_name = $19,
			contact_email = $20,
			contact_role = $21,
			document_date = $22::date,
			payment_method_code = $23,
			payment_method_label = $24,
			payment_bank_details = $25,
			description = $26,
			internal_notes = $27,
			hubspot_sync_status = CASE WHEN $28 THEN 'pending' ELSE hubspot_sync_status END,
			hubspot_sync_error = CASE WHEN $28 THEN NULL ELSE hubspot_sync_error END,
			hubspot_synced_at = CASE WHEN $28 THEN NULL ELSE hubspot_synced_at END
		WHERE id = $29`,
		actor.Subject, nextStatus,
		trimmedOrNil(req.HubSpotCompanyID), trimmedOrNil(req.HubSpotContactID),
		trimmedOrNil(req.Customer.Name), trimmedOrNil(req.Customer.VAT), trimmedOrNil(req.Customer.TaxCode),
		trimmedOrNil(req.Customer.PEC), trimmedOrNil(req.Customer.Email), trimmedOrNil(req.Customer.Address),
		trimmedOrNil(req.Customer.ZIP), trimmedOrNil(req.Customer.City), trimmedOrNil(req.Customer.Province),
		trimmedOrNil(req.Customer.Country), trimmedOrNil(req.Customer.Language), trimmedOrNil(req.Customer.NumeroAziendaSnapshot),
		trimmedOrNil(req.Contact.FirstName), trimmedOrNil(req.Contact.LastName), trimmedOrNil(req.Contact.FullName),
		trimmedOrNil(req.Contact.Email), trimmedOrNil(req.Contact.Role),
		trimmedOrNil(req.DocumentDate), payment.Code, payment.Label, trimmedOrNil(req.PaymentBankDetails),
		trimmedOrNil(req.Description), trimmedOrNil(req.InternalNotes),
		relevantChanged,
		quoteID,
	)
	if err != nil {
		h.dbFailure(w, r, "quote_update_header", err, "quote_id", quoteID)
		return
	}
	if err := h.replaceQuoteLines(r.Context(), tx, quoteID, lines); err != nil {
		h.dbFailure(w, r, "quote_update_lines", err, "quote_id", quoteID)
		return
	}

	eventPayload := map[string]any{"authoring_status": nextStatus}
	if current.AuthoringStatus == authoringStatusReady && nextStatus == authoringStatusDraft {
		eventPayload["demoted_from_ready"] = true
	}
	if err := h.appendQuoteEvent(r.Context(), tx, quoteID, "updated", actor.Subject, eventPayload); err != nil {
		h.dbFailure(w, r, "quote_update_event", err, "quote_id", quoteID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "quote_update_commit", err, "quote_id", quoteID)
		return
	}

	if relevantChanged {
		h.enqueueUpdateDealPostCommit(r, quoteID, actor)
	}
	h.respondQuoteByID(w, r, quoteID, http.StatusOK)
}

func (h *Handler) handleHubSpotRetry(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) || !h.requireConfigDB(w) {
		return
	}
	quoteID, ok := quoteIDFromRequest(w, r)
	if !ok {
		return
	}
	actor := requestActor(r)

	quote, err := h.loadQuoteDetail(r.Context(), h.deps.Mistra, quoteID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "hubspot_retry_quote_load", err, "quote_id", quoteID)
		return
	}

	var (
		operation string
		dedupeKey string
		result    hubSpotQueueResult
	)
	queue := newHubSpotQueueStore(h.deps.ConfigDB)
	if strings.TrimSpace(deref(quote.HubSpotDealID)) == "" {
		operation = hubSpotOperationCreateDeal
		dedupeKey = hubSpotCreateDealDedupeKey(quoteID)
		result, err = queue.enqueueCreateDeal(r.Context(), quote, actor, true)
	} else {
		operation = hubSpotOperationUpdateDeal
		dedupeKey = hubSpotUpdateDealDedupeKey(quoteID)
		result, err = queue.enqueueUpdateDeal(r.Context(), quote, actor, true)
	}
	if err != nil {
		h.logger.Error(
			"hubspot retry enqueue failed",
			"operation", "hubspot_retry_enqueue",
			"hubspot_operation", operation,
			"quote_id", quoteID,
			"error", err,
		)
		h.recordHubSpotQueueFailure(r.Context(), quoteID, actor, operation, dedupeKey, err)
		h.dbFailure(w, r, "hubspot_retry_enqueue", err, "quote_id", quoteID)
		return
	}
	if err := h.appendQuoteEventCommitted(r.Context(), quoteID, quoteEventHubSpotRetryEnqueued, actor.Subject, hubSpotQueueEventPayload(operation, dedupeKey, result)); err != nil {
		h.dbFailure(w, r, "hubspot_retry_event", err, "quote_id", quoteID)
		return
	}

	h.respondQuoteByID(w, r, quoteID, http.StatusOK)
}

func (h *Handler) handleQuoteReady(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}
	quoteID, ok := quoteIDFromRequest(w, r)
	if !ok {
		return
	}
	actor := requestActor(r)

	tx, err := h.deps.Mistra.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "quote_ready_begin", err, "quote_id", quoteID)
		return
	}
	defer tx.Rollback()

	quote, err := h.loadQuoteForUpdate(r.Context(), tx, quoteID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "quote_ready_load", err, "quote_id", quoteID)
		return
	}
	lines, err := h.loadQuoteLines(r.Context(), tx, quoteID)
	if err != nil {
		h.dbFailure(w, r, "quote_ready_lines_load", err, "quote_id", quoteID)
		return
	}
	payment, ok := h.resolvePaymentForSave(w, r, quote.Payment.MethodCode, true)
	if !ok {
		return
	}
	if !validateReadyQuote(w, quote, lines) {
		return
	}

	_, err = tx.ExecContext(r.Context(), `
		UPDATE raenad.quote
		SET authoring_status = 'ready',
			updated_by = $1,
			payment_method_code = $2,
			payment_method_label = $3
		WHERE id = $4`, actor.Subject, payment.Code, payment.Label, quoteID)
	if err != nil {
		h.dbFailure(w, r, "quote_ready_update", err, "quote_id", quoteID)
		return
	}
	if err := h.appendQuoteEvent(r.Context(), tx, quoteID, "marked_ready", actor.Subject, map[string]any{"authoring_status": authoringStatusReady}); err != nil {
		h.dbFailure(w, r, "quote_ready_event", err, "quote_id", quoteID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "quote_ready_commit", err, "quote_id", quoteID)
		return
	}

	h.respondQuoteByID(w, r, quoteID, http.StatusOK)
}

func (h *Handler) decodeSaveQuoteRequest(w http.ResponseWriter, r *http.Request) (saveQuoteRequest, bool) {
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_payload")
		return saveQuoteRequest{}, false
	}
	if hasClientOwnedTotals(raw) {
		httputil.Error(w, http.StatusBadRequest, "invalid_payload")
		return saveQuoteRequest{}, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var req saveQuoteRequest
	if err := decoder.Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_payload")
		return saveQuoteRequest{}, false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		httputil.Error(w, http.StatusBadRequest, "invalid_payload")
		return saveQuoteRequest{}, false
	}
	return req, true
}

func hasClientOwnedTotals(raw []byte) bool {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return false
	}
	for _, key := range []string{"quote_number", "total_net", "total_vat", "total_gross", "total_purchase", "total_gain"} {
		if _, ok := top[key]; ok {
			return true
		}
	}
	var lines []map[string]json.RawMessage
	if rawLines, ok := top["lines"]; ok && json.Unmarshal(rawLines, &lines) == nil {
		for _, line := range lines {
			for _, key := range []string{"id", "quote_id", "line_net", "line_vat", "line_gross", "line_purchase", "line_gain"} {
				if _, ok := line[key]; ok {
					return true
				}
			}
		}
	}
	return false
}

func (h *Handler) validateCreatePayload(w http.ResponseWriter, req saveQuoteRequest) bool {
	if strings.TrimSpace(deref(req.HubSpotCompanyID)) == "" ||
		strings.TrimSpace(deref(req.Customer.Name)) == "" ||
		strings.TrimSpace(deref(req.DocumentDate)) == "" ||
		strings.TrimSpace(deref(req.Description)) == "" {
		httputil.Error(w, http.StatusBadRequest, "validation_failed")
		return false
	}
	if !validDate(req.DocumentDate) {
		httputil.Error(w, http.StatusBadRequest, "invalid_payload")
		return false
	}
	return true
}

func (h *Handler) validateUpdatePayload(w http.ResponseWriter, req saveQuoteRequest) bool {
	if req.DocumentDate != nil && !validDate(req.DocumentDate) {
		httputil.Error(w, http.StatusBadRequest, "invalid_payload")
		return false
	}
	return true
}

func validateReadyQuote(w http.ResponseWriter, quote quoteResponse, lines []quoteLine) bool {
	if strings.TrimSpace(deref(quote.HubSpotCompanyID)) == "" ||
		strings.TrimSpace(deref(quote.Customer.Name)) == "" ||
		strings.TrimSpace(deref(quote.DocumentDate)) == "" ||
		strings.TrimSpace(deref(quote.Description)) == "" ||
		strings.TrimSpace(deref(quote.Payment.MethodCode)) == "" {
		httputil.Error(w, http.StatusBadRequest, "validation_failed")
		return false
	}
	for _, line := range lines {
		if line.LineType != lineTypeItem {
			continue
		}
		if strings.TrimSpace(deref(line.Qta)) != "" &&
			strings.TrimSpace(deref(line.UnitPrice)) != "" &&
			(strings.TrimSpace(deref(line.CodIVA)) != "" || strings.TrimSpace(deref(line.IVAPercentSnapshot)) != "") {
			return true
		}
	}
	httputil.Error(w, http.StatusBadRequest, "validation_failed")
	return false
}

func (h *Handler) resolvePaymentForSave(w http.ResponseWriter, r *http.Request, code *string, required bool) (paymentSnapshotResolved, bool) {
	trimmed := strings.TrimSpace(deref(code))
	if trimmed == "" {
		if required {
			httputil.Error(w, http.StatusBadRequest, "validation_failed")
			return paymentSnapshotResolved{}, false
		}
		return paymentSnapshotResolved{}, true
	}
	var dbCode, label string
	err := h.deps.Mistra.QueryRowContext(r.Context(), `
		SELECT RTRIM(cod_pagamento) AS cod_pagamento, desc_pagamento
		FROM loader.erp_metodi_pagamento
		WHERE RTRIM(cod_pagamento) = $1`, trimmed).Scan(&dbCode, &label)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusBadRequest, "payment_method_not_found")
		return paymentSnapshotResolved{}, false
	}
	if err != nil {
		h.dbFailure(w, r, "quote_payment_method_validate", err)
		return paymentSnapshotResolved{}, false
	}
	dbCode = strings.TrimSpace(dbCode)
	label = strings.TrimSpace(label)
	return paymentSnapshotResolved{Code: &dbCode, Label: &label}, true
}

func (h *Handler) initialStageSnapshot(ctx context.Context) (stageSnapshotResolved, bool) {
	cfg, ok := h.readHubSpotDealPipelineConfig(ctx)
	if !ok {
		return stageSnapshotResolved{}, false
	}
	stage := stageSnapshotResolved{PipelineID: cfg.PipelineID, DealstageID: cfg.InitialDealstageID}
	var stageLabel, pipelineLabel sql.NullString
	err := h.deps.Mistra.QueryRowContext(ctx, `
		SELECT s.label, p.label
		FROM loader.hubs_stages s
		LEFT JOIN loader.hubs_pipeline p ON p.id = s.pipeline
		WHERE s.pipeline = $1 AND s.id = $2`, cfg.PipelineID, cfg.InitialDealstageID).Scan(&stageLabel, &pipelineLabel)
	if err == nil {
		stage.DealstageLabel = nullTrimmedStringPtr(stageLabel)
		stage.PipelineLabel = nullTrimmedStringPtr(pipelineLabel)
	}
	return stage, true
}

func (h *Handler) normalizeLineInputs(w http.ResponseWriter, inputs []quoteLineInput) ([]quoteLineInput, bool) {
	out := make([]quoteLineInput, 0, len(inputs))
	for i, line := range inputs {
		line.LineType = strings.TrimSpace(line.LineType)
		if line.LineType == "" {
			line.LineType = lineTypeItem
		}
		if line.LineType != lineTypeItem && line.LineType != lineTypeDescription && line.LineType != lineTypeSpacer {
			httputil.Error(w, http.StatusBadRequest, "invalid_payload")
			return nil, false
		}
		line.Position = i + 1
		line.ItemCode = trimmedOrNil(line.ItemCode)
		line.ItemDescription = trimmedOrNil(line.ItemDescription)
		line.Description = trimmedOrNil(line.Description)
		line.UnitOfMeasure = trimmedOrNil(line.UnitOfMeasure)
		line.Discounts = trimmedOrNil(line.Discounts)
		line.CodIVA = trimmedOrNil(line.CodIVA)
		var err error
		if line.Qta, err = normalizeDecimal18_4(line.Qta); err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_payload")
			return nil, false
		}
		if line.UnitPrice, err = normalizeDecimal18_4(line.UnitPrice); err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_payload")
			return nil, false
		}
		if line.IVAPercentSnapshot, err = normalizeDecimal18_4(line.IVAPercentSnapshot); err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_payload")
			return nil, false
		}
		if line.PurchaseUnitPrice, err = normalizeDecimal18_4(line.PurchaseUnitPrice); err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_payload")
			return nil, false
		}
		out = append(out, line)
	}
	return out, true
}

func (h *Handler) validateWritableLines(ctx context.Context, w http.ResponseWriter, r *http.Request, q queryer, lines []quoteLineInput, ready bool) bool {
	for _, line := range lines {
		economic := line.LineType == lineTypeItem && line.Qta != nil && line.UnitPrice != nil
		if !economic {
			continue
		}
		if line.IVAPercentSnapshot != nil {
			continue
		}
		if strings.TrimSpace(deref(line.CodIVA)) == "" {
			httputil.Error(w, http.StatusBadRequest, "validation_failed")
			return false
		}
		var resolved sql.NullString
		if err := q.QueryRowContext(ctx, `SELECT raenad.resolve_iva_percent($1)::text`, *line.CodIVA).Scan(&resolved); err != nil {
			h.dbFailure(w, r, "quote_line_vat_validate", err)
			return false
		}
		if !resolved.Valid || strings.TrimSpace(resolved.String) == "" {
			httputil.Error(w, http.StatusBadRequest, "validation_failed")
			return false
		}
	}
	if ready {
		return true
	}
	return true
}

func (h *Handler) replaceQuoteLines(ctx context.Context, tx *sql.Tx, quoteID int64, lines []quoteLineInput) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM raenad.quote_line WHERE quote_id = $1`, quoteID); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO raenad.quote_line (
				quote_id, position, line_type, item_code, item_description, description,
				unit_of_measure, qta, unit_price, discounts, cod_iva, iva_percent_snapshot,
				purchase_unit_price
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8::numeric(18,4), $9::numeric(18,4), $10, $11, $12::numeric(9,4),
				$13::numeric(18,4)
			)`,
			quoteID, line.Position, line.LineType, line.ItemCode, line.ItemDescription, line.Description,
			line.UnitOfMeasure, line.Qta, line.UnitPrice, line.Discounts, line.CodIVA, line.IVAPercentSnapshot,
			line.PurchaseUnitPrice,
		); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) appendQuoteEvent(ctx context.Context, tx *sql.Tx, quoteID int64, eventType, actorSubject string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO raenad.quote_event (quote_id, event_type, actor_subject, payload)
		VALUES ($1, $2, $3, $4::jsonb)`, quoteID, eventType, actorSubject, string(raw))
	return err
}

func (h *Handler) respondQuoteByID(w http.ResponseWriter, r *http.Request, quoteID int64, status int) {
	quote, err := h.loadQuoteDetail(r.Context(), h.deps.Mistra, quoteID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "quote_get", err, "quote_id", quoteID)
		return
	}
	httputil.JSON(w, status, quote)
}

func (h *Handler) enqueueCreateDealPostCommit(r *http.Request, quoteID int64, actor actor) {
	h.enqueueDealPostCommit(r, quoteID, actor, hubSpotOperationCreateDeal, hubSpotCreateDealDedupeKey(quoteID), quoteEventHubSpotCreateEnqueued)
}

func (h *Handler) enqueueUpdateDealPostCommit(r *http.Request, quoteID int64, actor actor) {
	h.enqueueDealPostCommit(r, quoteID, actor, hubSpotOperationUpdateDeal, hubSpotUpdateDealDedupeKey(quoteID), quoteEventHubSpotUpdateEnqueued)
}

func (h *Handler) enqueueDealPostCommit(r *http.Request, quoteID int64, actor actor, operation, dedupeKey, eventType string) {
	if h.deps.ConfigDB == nil {
		err := errors.New("raenad hubspot queue database not configured")
		h.logger.Error(
			"hubspot queue database missing after quote commit",
			"operation", "hubspot_enqueue_post_commit",
			"hubspot_operation", operation,
			"quote_id", quoteID,
			"error", err,
		)
		h.recordHubSpotQueueFailure(r.Context(), quoteID, actor, operation, dedupeKey, err)
		return
	}

	quote, err := h.loadQuoteDetail(r.Context(), h.deps.Mistra, quoteID)
	if err != nil {
		h.logger.Error(
			"quote committed but could not be loaded for hubspot enqueue",
			"operation", "hubspot_enqueue_quote_load",
			"hubspot_operation", operation,
			"quote_id", quoteID,
			"error", err,
		)
		h.recordHubSpotQueueFailure(r.Context(), quoteID, actor, operation, dedupeKey, err)
		return
	}

	queue := newHubSpotQueueStore(h.deps.ConfigDB)
	var result hubSpotQueueResult
	if operation == hubSpotOperationCreateDeal {
		result, err = queue.enqueueCreateDeal(r.Context(), quote, actor, false)
	} else {
		result, err = queue.enqueueUpdateDeal(r.Context(), quote, actor, false)
	}
	if err != nil {
		h.logger.Error(
			"hubspot enqueue failed after quote commit",
			"operation", "hubspot_enqueue_post_commit",
			"hubspot_operation", operation,
			"quote_id", quoteID,
			"error", err,
		)
		h.recordHubSpotQueueFailure(r.Context(), quoteID, actor, operation, dedupeKey, err)
		return
	}
	if err := h.appendQuoteEventCommitted(r.Context(), quoteID, eventType, actor.Subject, hubSpotQueueEventPayload(operation, dedupeKey, result)); err != nil {
		h.logger.Error(
			"hubspot enqueue event append failed after quote commit",
			"operation", "hubspot_enqueue_event",
			"hubspot_operation", operation,
			"quote_id", quoteID,
			"hubspot_request_id", result.RequestID,
			"error", err,
		)
	}
}

func (h *Handler) recordHubSpotQueueFailure(ctx context.Context, quoteID int64, actor actor, operation, dedupeKey string, err error) {
	if h.deps.Mistra == nil {
		return
	}
	if eventErr := h.appendQuoteEventCommitted(ctx, quoteID, quoteEventHubSpotEnqueueFailed, actor.Subject, hubSpotQueueFailurePayload(operation, dedupeKey, err)); eventErr != nil {
		h.logger.Error(
			"hubspot enqueue failure event append failed",
			"operation", "hubspot_enqueue_failure_event",
			"hubspot_operation", operation,
			"quote_id", quoteID,
			"error", eventErr,
		)
	}
}

func (h *Handler) loadQuoteDetail(ctx context.Context, q queryer, quoteID int64) (quoteResponse, error) {
	quote, err := h.loadQuote(ctx, q, quoteID)
	if err != nil {
		return quoteResponse{}, err
	}
	lines, err := h.loadQuoteLines(ctx, q, quoteID)
	if err != nil {
		return quoteResponse{}, err
	}
	events, err := h.loadQuoteEvents(ctx, q, quoteID)
	if err != nil {
		return quoteResponse{}, err
	}
	quote.Lines = lines
	quote.Events = events
	return quote, nil
}

func (h *Handler) loadQuote(ctx context.Context, q queryer, quoteID int64) (quoteResponse, error) {
	return scanQuote(q.QueryRowContext(ctx, `SELECT `+quoteScanColumns+` FROM raenad.quote WHERE id = $1`, quoteID))
}

func (h *Handler) loadQuoteForUpdate(ctx context.Context, tx *sql.Tx, quoteID int64) (quoteResponse, error) {
	return scanQuote(tx.QueryRowContext(ctx, `SELECT `+quoteScanColumns+` FROM raenad.quote WHERE id = $1 FOR UPDATE`, quoteID))
}

func (h *Handler) loadQuoteLines(ctx context.Context, q queryer, quoteID int64) ([]quoteLine, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT `+quoteLineScanColumns+`
		FROM raenad.quote_line
		WHERE quote_id = $1
		ORDER BY position, id`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lines := make([]quoteLine, 0)
	for rows.Next() {
		line, err := scanQuoteLine(rows)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func (h *Handler) loadQuoteEvents(ctx context.Context, q queryer, quoteID int64) ([]quoteEvent, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, quote_id, event_type, actor_subject, payload, created_at
		FROM raenad.quote_event
		WHERE quote_id = $1
		ORDER BY created_at, id`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]quoteEvent, 0)
	for rows.Next() {
		var event quoteEvent
		var createdAt time.Time
		var payload []byte
		if err := rows.Scan(&event.ID, &event.QuoteID, &event.EventType, &event.ActorSubject, &payload, &createdAt); err != nil {
			return nil, err
		}
		if len(payload) == 0 {
			payload = []byte(`{}`)
		}
		event.Payload = json.RawMessage(payload)
		event.CreatedAt = formatTimestamp(createdAt)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func quoteIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	quoteID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || quoteID <= 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return 0, false
	}
	return quoteID, true
}

func requestActor(r *http.Request) actor {
	a, ok := actorFromContext(r.Context())
	if !ok {
		return actor{}
	}
	if a.Subject == "" {
		a.Subject = a.Email
	}
	if a.Subject == "" {
		a.Subject = a.Name
	}
	return a
}

func parsePositiveInt(raw string, fallback, min, max int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < min {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func validDate(value *string) bool {
	if strings.TrimSpace(deref(value)) == "" {
		return true
	}
	_, err := time.Parse(dateLayout, strings.TrimSpace(*value))
	return err == nil
}

func trimmedOrNil(value *string) *string {
	trimmed := strings.TrimSpace(deref(value))
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func quoteRelevantChange(current quoteResponse, currentLines []quoteLine, req saveQuoteRequest, payment paymentSnapshotResolved, lines []quoteLineInput) bool {
	currentComparable := quoteComparable{
		HubSpotCompanyID:   deref(current.HubSpotCompanyID),
		HubSpotContactID:   deref(current.HubSpotContactID),
		Customer:           current.Customer.customerSnapshotInput,
		Contact:            current.Contact.contactSnapshotInput,
		DocumentDate:       deref(current.DocumentDate),
		PaymentMethodCode:  deref(current.Payment.MethodCode),
		PaymentMethodLabel: deref(current.Payment.MethodLabel),
		PaymentBankDetails: deref(current.Payment.BankDetails),
		Description:        deref(current.Description),
		Lines:              comparableStoredLines(currentLines),
	}
	nextComparable := quoteComparable{
		HubSpotCompanyID:   deref(trimmedOrNil(req.HubSpotCompanyID)),
		HubSpotContactID:   deref(trimmedOrNil(req.HubSpotContactID)),
		Customer:           normalizeCustomerInput(req.Customer),
		Contact:            normalizeContactInput(req.Contact),
		DocumentDate:       deref(trimmedOrNil(req.DocumentDate)),
		PaymentMethodCode:  deref(payment.Code),
		PaymentMethodLabel: deref(payment.Label),
		PaymentBankDetails: deref(trimmedOrNil(req.PaymentBankDetails)),
		Description:        deref(trimmedOrNil(req.Description)),
		Lines:              comparableInputLines(lines),
	}
	return !sameJSONComparable(currentComparable, nextComparable)
}

type quoteComparable struct {
	HubSpotCompanyID   string
	HubSpotContactID   string
	Customer           customerSnapshotInput
	Contact            contactSnapshotInput
	DocumentDate       string
	PaymentMethodCode  string
	PaymentMethodLabel string
	PaymentBankDetails string
	Description        string
	Lines              []quoteLineInput
}

func normalizeCustomerInput(value customerSnapshotInput) customerSnapshotInput {
	return customerSnapshotInput{
		Name:                  trimmedOrNil(value.Name),
		VAT:                   trimmedOrNil(value.VAT),
		TaxCode:               trimmedOrNil(value.TaxCode),
		PEC:                   trimmedOrNil(value.PEC),
		Email:                 trimmedOrNil(value.Email),
		Address:               trimmedOrNil(value.Address),
		ZIP:                   trimmedOrNil(value.ZIP),
		City:                  trimmedOrNil(value.City),
		Province:              trimmedOrNil(value.Province),
		Country:               trimmedOrNil(value.Country),
		Language:              trimmedOrNil(value.Language),
		NumeroAziendaSnapshot: trimmedOrNil(value.NumeroAziendaSnapshot),
	}
}

func normalizeContactInput(value contactSnapshotInput) contactSnapshotInput {
	return contactSnapshotInput{
		FirstName: trimmedOrNil(value.FirstName),
		LastName:  trimmedOrNil(value.LastName),
		FullName:  trimmedOrNil(value.FullName),
		Email:     trimmedOrNil(value.Email),
		Role:      trimmedOrNil(value.Role),
	}
}

func comparableStoredLines(lines []quoteLine) []quoteLineInput {
	out := make([]quoteLineInput, 0, len(lines))
	for i, line := range lines {
		out = append(out, quoteLineInput{
			Position:           i + 1,
			LineType:           line.LineType,
			ItemCode:           trimmedOrNil(line.ItemCode),
			ItemDescription:    trimmedOrNil(line.ItemDescription),
			Description:        trimmedOrNil(line.Description),
			UnitOfMeasure:      trimmedOrNil(line.UnitOfMeasure),
			Qta:                trimmedOrNil(line.Qta),
			UnitPrice:          trimmedOrNil(line.UnitPrice),
			Discounts:          trimmedOrNil(line.Discounts),
			CodIVA:             trimmedOrNil(line.CodIVA),
			IVAPercentSnapshot: trimmedOrNil(line.IVAPercentSnapshot),
			PurchaseUnitPrice:  trimmedOrNil(line.PurchaseUnitPrice),
		})
	}
	return out
}

func comparableInputLines(lines []quoteLineInput) []quoteLineInput {
	out := make([]quoteLineInput, 0, len(lines))
	for _, line := range lines {
		out = append(out, quoteLineInput{
			Position:           line.Position,
			LineType:           line.LineType,
			ItemCode:           trimmedOrNil(line.ItemCode),
			ItemDescription:    trimmedOrNil(line.ItemDescription),
			Description:        trimmedOrNil(line.Description),
			UnitOfMeasure:      trimmedOrNil(line.UnitOfMeasure),
			Qta:                trimmedOrNil(line.Qta),
			UnitPrice:          trimmedOrNil(line.UnitPrice),
			Discounts:          trimmedOrNil(line.Discounts),
			CodIVA:             trimmedOrNil(line.CodIVA),
			IVAPercentSnapshot: trimmedOrNil(line.IVAPercentSnapshot),
			PurchaseUnitPrice:  trimmedOrNil(line.PurchaseUnitPrice),
		})
	}
	return out
}

func sameJSONComparable(left, right any) bool {
	leftRaw, _ := json.Marshal(left)
	rightRaw, _ := json.Marshal(right)
	return bytes.Equal(leftRaw, rightRaw)
}
