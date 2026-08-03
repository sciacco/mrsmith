package afctools

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const rdaDDTDateLayout = "2006-01-02"

// RDADDTAttachmentRow is an eligible RDA transport-document attachment.
// It is intentionally flat so both the list contract and the phase-2 download
// selection can use the same database result.
type RDADDTAttachmentRow struct {
	PurchaseOrderID    int64     `json:"po_id"`
	PurchaseOrderCode  *string   `json:"po_code"`
	AttachmentID       int64     `json:"attachment_id"`
	FileName           string    `json:"file_name"`
	Created            time.Time `json:"created"`
	RequesterFirstName *string   `json:"requester_first_name"`
	RequesterLastName  *string   `json:"requester_last_name"`
	RequesterEmail     *string   `json:"requester_email"`
	Project            string    `json:"project"`
	Subject            *string   `json:"subject"`
	CostCenter         *string   `json:"cost_center"`
}

type rdaDDTDateRange struct {
	start time.Time
	end   time.Time
}

// parseRDADDTDateRange resolves ISO civil dates to a half-open timestamp range
// in Europe/Rome. Passing instants to PostgreSQL avoids dependence on its session
// time zone and preserves the full final civil day across DST transitions.
func parseRDADDTDateRange(from, to string) (rdaDDTDateRange, error) {
	location, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		return rdaDDTDateRange{}, err
	}

	start, err := time.ParseInLocation(rdaDDTDateLayout, from, location)
	if err != nil {
		return rdaDDTDateRange{}, err
	}
	endDay, err := time.ParseInLocation(rdaDDTDateLayout, to, location)
	if err != nil {
		return rdaDDTDateRange{}, err
	}
	if start.After(endDay) {
		return rdaDDTDateRange{}, errors.New("from date is after to date")
	}

	return rdaDDTDateRange{start: start, end: endDay.AddDate(0, 0, 1)}, nil
}

const rdaDDTAttachmentQueryPrefix = `
SELECT
	po.id,
	po.code,
	pa.id,
	f.name,
	pa.created,
	u.first_name,
	u.last_name,
	u.email,
	po.project,
	po.object,
	po.cost_center
FROM rda.purchase_order_attachment pa
JOIN files.file f ON f.id = pa.file_id
JOIN rda.purchase_order po ON po.id = pa.order_id
LEFT JOIN users_int."user" u ON u.id = po.requester_id
WHERE pa.attachment_type = 'transport_document'
  AND pa.deleted IS NULL
  AND f.deleted_at IS NULL
  AND pa.created >= $1
  AND pa.created < $2
`

const rdaDDTAttachmentQueryOrder = `ORDER BY pa.created DESC, po.id ASC, pa.id ASC`

const rdaDDTAttachmentQuery = rdaDDTAttachmentQueryPrefix + rdaDDTAttachmentQueryOrder

// listRDADDTAttachments is shared query foundation for the phase-2 ZIP endpoint.
func (h *Handler) listRDADDTAttachments(ctx context.Context, dateRange rdaDDTDateRange) ([]RDADDTAttachmentRow, error) {
	return h.queryRDADDTAttachments(ctx, rdaDDTAttachmentQuery, dateRange.start, dateRange.end)
}

// listRDADDTAttachmentsForPOIDs rechecks the current database state for the
// requested orders. The dynamically generated placeholders contain only their
// ordinal number; every PO ID remains a parameterized database value.
func (h *Handler) listRDADDTAttachmentsForPOIDs(ctx context.Context, dateRange rdaDDTDateRange, poIDs []int64) ([]RDADDTAttachmentRow, error) {
	placeholders := make([]string, len(poIDs))
	args := make([]any, 0, len(poIDs)+2)
	args = append(args, dateRange.start, dateRange.end)
	for i, poID := range poIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+3)
		args = append(args, poID)
	}
	query := rdaDDTAttachmentQueryPrefix + "  AND po.id IN (" + strings.Join(placeholders, ", ") + ")\n" + rdaDDTAttachmentQueryOrder
	return h.queryRDADDTAttachments(ctx, query, args...)
}

func (h *Handler) queryRDADDTAttachments(ctx context.Context, query string, args ...any) ([]RDADDTAttachmentRow, error) {
	rows, err := h.deps.ArakDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attachments := make([]RDADDTAttachmentRow, 0)
	for rows.Next() {
		var attachment RDADDTAttachmentRow
		if err := rows.Scan(
			&attachment.PurchaseOrderID,
			&attachment.PurchaseOrderCode,
			&attachment.AttachmentID,
			&attachment.FileName,
			&attachment.Created,
			&attachment.RequesterFirstName,
			&attachment.RequesterLastName,
			&attachment.RequesterEmail,
			&attachment.Project,
			&attachment.Subject,
			&attachment.CostCenter,
		); err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attachments, nil
}

func (h *Handler) handleRDADDT(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w, h.deps.ArakDB, "arak") {
		return
	}

	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_from_or_to")
		return
	}
	dateRange, err := parseRDADDTDateRange(from, to)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_from_or_to")
		return
	}

	attachments, err := h.listRDADDTAttachments(r.Context(), dateRange)
	if err != nil {
		h.dbFailure(w, r, "list_rda_ddt", err)
		return
	}

	httputil.JSON(w, http.StatusOK, attachments)
}

type rdaDDTDownloadRequest struct {
	From  string  `json:"from"`
	To    string  `json:"to"`
	POIDs []int64 `json:"poIds"`
}

func (h *Handler) handleRDADDTDownload(w http.ResponseWriter, r *http.Request) {
	var request rdaDDTDownloadRequest
	if err := decodeJSON(r, &request); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if request.From == "" || request.To == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_from_or_to")
		return
	}
	dateRange, err := parseRDADDTDateRange(request.From, request.To)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_from_or_to")
		return
	}
	if len(request.POIDs) == 0 {
		httputil.Error(w, http.StatusBadRequest, "po_ids_required")
		return
	}
	if hasNonPositiveRDADDTPOID(request.POIDs) {
		httputil.Error(w, http.StatusBadRequest, "invalid_po_ids")
		return
	}
	poIDs := deduplicateRDADDTPOIDs(request.POIDs)

	if !h.requireDB(w, h.deps.ArakDB, "arak") {
		return
	}
	if h.deps.Arak == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "arak_client_not_configured")
		return
	}

	// Mistra/Arak resolves the acting user through Requester-Email (see
	// backend/internal/rda/handler.go requesterHeaders). The service token is
	// not enough: without the header the upstream rejects every file with 400.
	claims, ok := auth.GetClaims(r.Context())
	email := strings.TrimSpace(claims.Email)
	if !ok || email == "" || !strings.Contains(email, "@") {
		httputil.Error(w, http.StatusUnauthorized, "authentication_required")
		return
	}
	requesterHeaders := http.Header{"Requester-Email": []string{email}}

	attachments, err := h.listRDADDTAttachmentsForPOIDs(r.Context(), dateRange, poIDs)
	if err != nil {
		h.dbFailure(w, r, "download_rda_ddt_query", err)
		return
	}
	if len(attachments) == 0 {
		httputil.Error(w, http.StatusBadRequest, "no_matching_ddt_documents")
		return
	}

	// Only this long-running streaming response bypasses the server's global
	// write deadline. Request cancellation and the Arak client's own timeouts
	// still propagate through r.Context().
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		logging.FromContext(r.Context()).Warn("RDA DDT download: clear write deadline failed",
			"component", "afctools", "operation", "rda_ddt_download", "error", err)
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="ddt-purchase-order_%s_%s.zip"`, request.From, request.To))
	zipWriter := zip.NewWriter(w)
	defer func() {
		if err := zipWriter.Close(); err != nil {
			logging.FromContext(r.Context()).Error("RDA DDT download: close ZIP failed",
				"component", "afctools", "operation", "rda_ddt_download", "reason", "zip_close_failed", "error", err)
		}
	}()

	archivePaths := rdaDDTArchivePaths(attachments)
	failures := make([]string, 0)
	for i, attachment := range attachments {
		if r.Context().Err() != nil {
			return
		}

		path := fmt.Sprintf("/arak/rda/v1/po/%d/attachment/%d/download", attachment.PurchaseOrderID, attachment.AttachmentID)
		resp, err := h.deps.Arak.DoWithHeadersContext(r.Context(), http.MethodGet, path, "", nil, requesterHeaders)
		if err != nil {
			if r.Context().Err() != nil {
				return
			}
			failures = append(failures, rdaDDTFailureLine(attachment, "upstream_request_failed"))
			logRDADDTDownloadFailure(r, attachment, "upstream_request_failed", 0)
			continue
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			_ = resp.Body.Close()
			failures = append(failures, rdaDDTFailureLine(attachment, "upstream_status_"+strconv.Itoa(resp.StatusCode)))
			logRDADDTDownloadFailure(r, attachment, "upstream_status", resp.StatusCode)
			continue
		}

		entry, err := zipWriter.Create(archivePaths[i])
		if err != nil {
			_ = resp.Body.Close()
			failures = append(failures, rdaDDTFailureLine(attachment, "zip_entry_creation_failed"))
			logRDADDTDownloadFailure(r, attachment, "zip_entry_creation_failed", 0)
			continue
		}
		_, copyErr := io.Copy(entry, resp.Body)
		closeErr := resp.Body.Close()
		if copyErr != nil {
			if r.Context().Err() != nil {
				return
			}
			failures = append(failures, rdaDDTFailureLine(attachment, "document_stream_failed"))
			logRDADDTDownloadFailure(r, attachment, "document_stream_failed", 0)
			continue
		}
		if closeErr != nil {
			failures = append(failures, rdaDDTFailureLine(attachment, "upstream_body_close_failed"))
			logRDADDTDownloadFailure(r, attachment, "upstream_body_close_failed", 0)
		}
	}

	if len(failures) == 0 {
		return
	}
	manifest, err := zipWriter.Create("documenti_non_scaricati.txt")
	if err != nil {
		logging.FromContext(r.Context()).Error("RDA DDT download: create failure manifest failed",
			"component", "afctools", "operation", "rda_ddt_download", "reason", "manifest_creation_failed", "error", err)
		return
	}
	if _, err := io.WriteString(manifest, strings.Join(failures, "\n")+"\n"); err != nil {
		logging.FromContext(r.Context()).Error("RDA DDT download: write failure manifest failed",
			"component", "afctools", "operation", "rda_ddt_download", "reason", "manifest_write_failed", "error", err)
	}
}

func hasNonPositiveRDADDTPOID(ids []int64) bool {
	for _, id := range ids {
		if id <= 0 {
			return true
		}
	}
	return false
}

func deduplicateRDADDTPOIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

// rdaDDTArchivePaths builds one ZIP path per attachment. Each file is placed
// under a PO folder named PO-<po-id>: the PO id is the canonical PO number and
// is unique, so folders never collide. The PO code (e.g. "PO-50412/2026") is
// NOT used for folder naming: codes are not unique and a path-style sanitizer
// would collapse them to their last segment, hiding the PO number.
func rdaDDTArchivePaths(attachments []RDADDTAttachmentRow) []string {
	usedPaths := make(map[string]struct{})
	paths := make([]string, len(attachments))
	for i, attachment := range attachments {
		folder := "PO-" + strconv.FormatInt(attachment.PurchaseOrderID, 10)

		filename := safeRDADDTArchiveFilename(attachment.FileName, fmt.Sprintf("documento-%d", attachment.AttachmentID))
		path := folder + "/" + filename
		if _, exists := usedPaths[path]; exists {
			extension := filepath.Ext(filename)
			base := strings.TrimSuffix(filename, extension)
			path = fmt.Sprintf("%s/%s-%d%s", folder, base, attachment.AttachmentID, extension)
			for suffix := 2; ; suffix++ {
				if _, exists := usedPaths[path]; !exists {
					break
				}
				path = fmt.Sprintf("%s/%s-%d-%d%s", folder, base, attachment.AttachmentID, suffix, extension)
			}
		}
		usedPaths[path] = struct{}{}
		paths[i] = path
	}
	return paths
}

// safeRDADDTArchiveFilename mirrors the RDA attachment filename handling: it
// keeps only a basename and removes control characters before constructing a
// ZIP path, so database-provided names cannot escape their PO directory.
func safeRDADDTArchiveFilename(name, fallback string) string {
	if name = rdaDDTArchivePathPart(name); name != "" {
		return name
	}
	if fallback = rdaDDTArchivePathPart(fallback); fallback != "" {
		return fallback
	}
	return "documento"
}

func rdaDDTArchivePathPart(value string) string {
	value = filepath.Base(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
	value = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, value)
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\\`) {
		return ""
	}
	return value
}

func rdaDDTFailureLine(attachment RDADDTAttachmentRow, reason string) string {
	return fmt.Sprintf("PO %d, allegato %d: %s", attachment.PurchaseOrderID, attachment.AttachmentID, reason)
}

func logRDADDTDownloadFailure(r *http.Request, attachment RDADDTAttachmentRow, reason string, upstreamStatus int) {
	attrs := []any{
		"component", "afctools",
		"operation", "rda_ddt_download",
		"po_id", attachment.PurchaseOrderID,
		"attachment_id", attachment.AttachmentID,
		"reason", reason,
	}
	if upstreamStatus != 0 {
		attrs = append(attrs, "upstream_status", upstreamStatus)
	}
	logging.FromContext(r.Context()).Warn("RDA DDT attachment download failed", attrs...)
}
