package raenad

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const (
	raenadPDFContentType       = "application/pdf"
	raenadPDFTemplateNamespace = "raenad"
	raenadPDFTemplateKey       = "carbone_quote_template"
)

type pdfExportListResponse struct {
	Items []pdfExport `json:"items"`
}

type pdfAttachResponse struct {
	HubSpotRequestID int64  `json:"hubspot_request_id"`
	Status           string `json:"status"`
	Action           string `json:"action"`
}

type raenadPDFTemplateConfig struct {
	TemplateID string `json:"template_id"`
}

type raenadQuotePDFRenderPayload struct {
	ConvertTo string             `json:"convertTo"`
	Data      raenadQuotePDFData `json:"data"`
}

type raenadQuotePDFData struct {
	QuoteNumber  string                `json:"quote_number"`
	DocumentDate *string               `json:"document_date"`
	Description  *string               `json:"description"`
	Customer     customerSnapshotInput `json:"customer"`
	Contact      contactSnapshotInput  `json:"contact"`
	Payment      paymentSnapshot       `json:"payment"`
	Lines        []raenadQuotePDFLine  `json:"lines"`
	Totals       raenadQuotePDFTotals  `json:"totals"`
}

type raenadQuotePDFLine struct {
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
	LineNet            *string `json:"line_net"`
	LineVAT            *string `json:"line_vat"`
	LineGross          *string `json:"line_gross"`
	LinePurchase       *string `json:"line_purchase"`
	LineGain           *string `json:"line_gain"`
}

type raenadQuotePDFTotals struct {
	Net      string `json:"net"`
	VAT      string `json:"vat"`
	Gross    string `json:"gross"`
	Purchase string `json:"purchase"`
	Gain     string `json:"gain"`
}

const pdfExportScanColumns = `
	id,
	quote_id,
	revision,
	filename,
	content_type,
	checksum_sha256,
	render_payload,
	created_at,
	created_by,
	hubspot_attachment_status,
	hubspot_file_id,
	hubspot_note_id,
	hubspot_deal_id,
	hubspot_attached_at,
	hubspot_error`

func (h *Handler) handlePDFExportList(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}
	quoteID, ok := quoteIDFromRequest(w, r)
	if !ok {
		return
	}

	quote, err := h.loadQuoteDetail(r.Context(), h.deps.Mistra, quoteID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "quote_pdf_list_quote", err, "quote_id", quoteID)
		return
	}
	_, currentChecksum, err := canonicalRaenadQuotePDFPayload(quote)
	if err != nil {
		httputil.InternalError(w, r, err, "raenad pdf payload marshal failed",
			"component", component, "operation", "quote_pdf_list_payload", "quote_id", quoteID)
		return
	}

	exports, err := h.loadPDFExports(r.Context(), h.deps.Mistra, quoteID)
	if err != nil {
		h.dbFailure(w, r, "quote_pdf_list", err, "quote_id", quoteID)
		return
	}
	for i := range exports {
		exports[i].IsStale = deref(exports[i].ChecksumSHA256) != currentChecksum
	}
	httputil.JSON(w, http.StatusOK, pdfExportListResponse{Items: exports})
}

func (h *Handler) handlePDFExportCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) || !h.requireConfigDB(w) || !h.requireCarbone(w) {
		return
	}
	quoteID, ok := quoteIDFromRequest(w, r)
	if !ok {
		return
	}

	tx, err := h.deps.Mistra.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "quote_pdf_create_begin", err, "quote_id", quoteID)
		return
	}
	defer tx.Rollback()

	quote, err := h.loadQuoteForUpdate(r.Context(), tx, quoteID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "quote_pdf_create_quote", err, "quote_id", quoteID)
		return
	}
	if quote.AuthoringStatus != authoringStatusReady {
		httputil.Error(w, http.StatusBadRequest, "quote_not_ready")
		return
	}

	lines, err := h.loadQuoteLines(r.Context(), tx, quoteID)
	if err != nil {
		h.dbFailure(w, r, "quote_pdf_create_lines", err, "quote_id", quoteID)
		return
	}
	quote.Lines = lines

	if _, ok, err := h.resolveRaenadPDFTemplateID(r.Context()); err != nil {
		h.dbFailure(w, r, "quote_pdf_template_config", err, "quote_id", quoteID)
		return
	} else if !ok {
		httputil.Error(w, http.StatusServiceUnavailable, "raenad_pdf_not_configured")
		return
	}

	payload, checksum, err := canonicalRaenadQuotePDFPayload(quote)
	if err != nil {
		httputil.InternalError(w, r, err, "raenad pdf payload marshal failed",
			"component", component, "operation", "quote_pdf_create_payload", "quote_id", quoteID)
		return
	}

	latest, err := h.loadLatestPDFExport(r.Context(), tx, quoteID)
	if errors.Is(err, sql.ErrNoRows) {
		latest = pdfExport{}
	} else if err != nil {
		h.dbFailure(w, r, "quote_pdf_create_latest", err, "quote_id", quoteID)
		return
	}
	if latest.ID != 0 && deref(latest.ChecksumSHA256) == checksum {
		latest.IsStale = false
		if err := tx.Commit(); err != nil {
			h.dbFailure(w, r, "quote_pdf_create_reuse_commit", err, "quote_id", quoteID)
			return
		}
		httputil.JSON(w, http.StatusOK, latest)
		return
	}

	revision := latest.Revision + 1
	if revision <= 0 {
		revision = 1
	}
	filename := raenadQuotePDFFilename(quote, revision)
	createdBy := requestActor(r).Subject
	export, err := h.insertPDFExport(r.Context(), tx, quoteID, revision, filename, checksum, payload, createdBy)
	if err != nil {
		h.dbFailure(w, r, "quote_pdf_create_insert", err, "quote_id", quoteID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "quote_pdf_create_commit", err, "quote_id", quoteID)
		return
	}
	httputil.JSON(w, http.StatusCreated, export)
}

func (h *Handler) handlePDFExportDownload(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) || !h.requireConfigDB(w) || !h.requireCarbone(w) {
		return
	}
	quoteID, ok := quoteIDFromRequest(w, r)
	if !ok {
		return
	}
	exportID, ok := exportIDFromRequest(w, r)
	if !ok {
		return
	}

	export, err := h.loadPDFExport(r.Context(), h.deps.Mistra, quoteID, exportID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "pdf_export_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "quote_pdf_download_export", err, "quote_id", quoteID, "export_id", exportID)
		return
	}

	templateID, ok, err := h.resolveRaenadPDFTemplateID(r.Context())
	if err != nil {
		h.dbFailure(w, r, "quote_pdf_template_config", err, "quote_id", quoteID, "export_id", exportID)
		return
	}
	if !ok {
		httputil.Error(w, http.StatusServiceUnavailable, "raenad_pdf_not_configured")
		return
	}

	payload := export.RenderPayload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	pdfBytes, err := h.deps.Carbone.GeneratePDF(r.Context(), templateID, payload)
	if err != nil {
		h.logger.Warn("raenad pdf generation failed",
			"operation", "quote_pdf_download_generate", "quote_id", quoteID, "export_id", exportID, "error", err)
		httputil.Error(w, http.StatusBadGateway, "raenad_pdf_generation_failed")
		return
	}

	contentType := strings.TrimSpace(export.ContentType)
	if contentType == "" {
		contentType = raenadPDFContentType
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", export.Filename))
	_, _ = w.Write(pdfBytes)
}

func (h *Handler) handlePDFExportAttach(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) || !h.requireConfigDB(w) {
		return
	}
	quoteID, ok := quoteIDFromRequest(w, r)
	if !ok {
		return
	}
	exportID, ok := exportIDFromRequest(w, r)
	if !ok {
		return
	}

	quote, err := h.loadQuote(r.Context(), h.deps.Mistra, quoteID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "quote_pdf_attach_quote", err, "quote_id", quoteID, "export_id", exportID)
		return
	}

	export, err := h.loadPDFExport(r.Context(), h.deps.Mistra, quoteID, exportID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "pdf_export_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "quote_pdf_attach_export", err, "quote_id", quoteID, "export_id", exportID)
		return
	}
	if strings.TrimSpace(deref(quote.HubSpotDealID)) == "" {
		httputil.Error(w, http.StatusConflict, "quote_hubspot_deal_required")
		return
	}

	dedupeKey := hubSpotAttachPDFDedupeKey(quoteID, exportID)
	result, err := newHubSpotQueueStore(h.deps.ConfigDB).enqueueAttachPDF(r.Context(), quote, export, requestActor(r))
	if err != nil {
		h.logger.Error(
			"hubspot pdf attach enqueue failed",
			"operation", "hubspot_pdf_attach_enqueue",
			"quote_id", quoteID,
			"export_id", exportID,
			"error", err,
		)
		h.dbFailure(w, r, "hubspot_pdf_attach_enqueue", err, "quote_id", quoteID, "export_id", exportID)
		return
	}
	if err := h.appendQuoteEventCommitted(r.Context(), quoteID, quoteEventHubSpotPDFAttachEnqueued, requestActor(r).Subject, hubSpotQueueEventPayload(hubSpotOperationAttachPDF, dedupeKey, result)); err != nil {
		h.dbFailure(w, r, "hubspot_pdf_attach_event", err, "quote_id", quoteID, "export_id", exportID)
		return
	}

	httputil.JSON(w, http.StatusAccepted, pdfAttachResponse{
		HubSpotRequestID: result.RequestID,
		Status:           result.Status,
		Action:           result.Action,
	})
}

func (h *Handler) resolveRaenadPDFTemplateID(ctx context.Context) (string, bool, error) {
	var raw []byte
	err := h.deps.ConfigDB.QueryRowContext(ctx, `
		SELECT value
		FROM mrsmith.runtime_config
		WHERE namespace = $1 AND key = $2`,
		raenadPDFTemplateNamespace, raenadPDFTemplateKey,
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}

	var cfg raenadPDFTemplateConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", false, nil
	}
	templateID := strings.TrimSpace(cfg.TemplateID)
	if templateID == "" {
		return "", false, nil
	}
	return templateID, true, nil
}

func canonicalRaenadQuotePDFPayload(quote quoteResponse) (json.RawMessage, string, error) {
	lines := make([]raenadQuotePDFLine, 0, len(quote.Lines))
	for _, line := range quote.Lines {
		lines = append(lines, raenadQuotePDFLine{
			Position:           line.Position,
			LineType:           line.LineType,
			ItemCode:           line.ItemCode,
			ItemDescription:    line.ItemDescription,
			Description:        line.Description,
			UnitOfMeasure:      line.UnitOfMeasure,
			Qta:                line.Qta,
			UnitPrice:          line.UnitPrice,
			Discounts:          line.Discounts,
			CodIVA:             line.CodIVA,
			IVAPercentSnapshot: line.IVAPercentSnapshot,
			PurchaseUnitPrice:  line.PurchaseUnitPrice,
			LineNet:            line.LineNet,
			LineVAT:            line.LineVAT,
			LineGross:          line.LineGross,
			LinePurchase:       line.LinePurchase,
			LineGain:           line.LineGain,
		})
	}
	payload := raenadQuotePDFRenderPayload{
		ConvertTo: "pdf",
		Data: raenadQuotePDFData{
			QuoteNumber:  quote.QuoteNumber,
			DocumentDate: quote.DocumentDate,
			Description:  quote.Description,
			Customer:     quote.Customer.customerSnapshotInput,
			Contact:      quote.Contact.contactSnapshotInput,
			Payment:      quote.Payment,
			Lines:        lines,
			Totals: raenadQuotePDFTotals{
				Net:      quote.TotalNet,
				VAT:      quote.TotalVAT,
				Gross:    quote.TotalGross,
				Purchase: quote.TotalPurchase,
				Gain:     quote.TotalGain,
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	return json.RawMessage(raw), hex.EncodeToString(sum[:]), nil
}

func (h *Handler) loadPDFExports(ctx context.Context, q queryer, quoteID int64) ([]pdfExport, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT `+pdfExportScanColumns+`
		FROM raenad.quote_pdf_export
		WHERE quote_id = $1
		ORDER BY revision DESC`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	exports := make([]pdfExport, 0)
	for rows.Next() {
		export, err := scanPDFExport(rows)
		if err != nil {
			return nil, err
		}
		exports = append(exports, export)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return exports, nil
}

func (h *Handler) loadLatestPDFExport(ctx context.Context, q queryer, quoteID int64) (pdfExport, error) {
	return scanPDFExport(q.QueryRowContext(ctx, `
		SELECT `+pdfExportScanColumns+`
		FROM raenad.quote_pdf_export
		WHERE quote_id = $1
		ORDER BY revision DESC
		LIMIT 1`, quoteID))
}

func (h *Handler) loadPDFExport(ctx context.Context, q queryer, quoteID, exportID int64) (pdfExport, error) {
	return scanPDFExport(q.QueryRowContext(ctx, `
		SELECT `+pdfExportScanColumns+`
		FROM raenad.quote_pdf_export
		WHERE quote_id = $1 AND id = $2`, quoteID, exportID))
}

func (h *Handler) insertPDFExport(
	ctx context.Context,
	tx *sql.Tx,
	quoteID int64,
	revision int,
	filename string,
	checksum string,
	payload json.RawMessage,
	createdBy string,
) (pdfExport, error) {
	return scanPDFExport(tx.QueryRowContext(ctx, `
		INSERT INTO raenad.quote_pdf_export (
			quote_id, revision, filename, content_type, checksum_sha256, render_payload, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6::jsonb, $7
		)
		RETURNING `+pdfExportScanColumns,
		quoteID, revision, filename, raenadPDFContentType, checksum, string(payload), createdBy,
	))
}

func scanPDFExport(scanner rowScanner) (pdfExport, error) {
	var (
		item         pdfExport
		checksum     sql.NullString
		payload      []byte
		createdAt    time.Time
		fileID       sql.NullString
		noteID       sql.NullString
		dealID       sql.NullString
		attachedAt   sql.NullTime
		hubSpotError sql.NullString
	)
	if err := scanner.Scan(
		&item.ID,
		&item.QuoteID,
		&item.Revision,
		&item.Filename,
		&item.ContentType,
		&checksum,
		&payload,
		&createdAt,
		&item.CreatedBy,
		&item.HubSpotAttachmentStatus,
		&fileID,
		&noteID,
		&dealID,
		&attachedAt,
		&hubSpotError,
	); err != nil {
		return pdfExport{}, err
	}
	item.ChecksumSHA256 = nullStringPtr(checksum)
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	item.RenderPayload = json.RawMessage(payload)
	item.CreatedAt = formatTimestamp(createdAt)
	item.HubSpotFileID = nullStringPtr(fileID)
	item.HubSpotNoteID = nullStringPtr(noteID)
	item.HubSpotDealID = nullStringPtr(dealID)
	item.HubSpotAttachedAt = nullTimestampPtr(attachedAt)
	item.HubSpotError = nullStringPtr(hubSpotError)
	return item, nil
}

func exportIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	exportID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("exportID")), 10, 64)
	if err != nil || exportID <= 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_pdf_export_id")
		return 0, false
	}
	return exportID, true
}

var raenadFilenameSanitizer = strings.NewReplacer(
	".", " ", "/", " ", "\\", " ", ":", " ", "*", " ",
	"?", " ", "\"", " ", "<", " ", ">", " ", "|", " ",
)

func raenadQuotePDFFilename(quote quoteResponse, revision int) string {
	parts := []string{"Preventivo"}
	if quote.QuoteNumber != "" {
		parts = append(parts, quote.QuoteNumber)
	}
	if date := strings.TrimSpace(deref(quote.DocumentDate)); date != "" {
		if parsed, err := time.Parse(dateLayout, date); err == nil {
			parts = append(parts, "del", parsed.Format("02-01-2006"))
		}
	}
	if customer := strings.Join(strings.Fields(raenadFilenameSanitizer.Replace(deref(quote.Customer.Name))), " "); customer != "" {
		parts = append(parts, customer)
	}
	if revision > 0 {
		parts = append(parts, fmt.Sprintf("rev%d", revision))
	}
	return strings.Join(strings.Fields(raenadFilenameSanitizer.Replace(strings.Join(parts, " "))), " ") + ".pdf"
}
