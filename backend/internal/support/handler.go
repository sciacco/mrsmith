package support

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	maxRequestBodyBytes        = 96 * 1024
	maxMessageLength           = 4000
	maxAttachmentCount         = 5
	maxAttachmentBytes         = 10 << 20
	maxAttachmentTotalBytes    = 25 << 20
	maxMultipartOverheadBytes  = 1 << 20
	maxMultipartBodyBytes      = maxAttachmentTotalBytes + maxMultipartOverheadBytes
	maxAttachmentFilenameRunes = 180
)

var allowedAttachmentContentTypes = map[string]struct{}{
	"application/csv":               {},
	"application/msword":            {},
	"application/pdf":               {},
	"application/vnd.ms-excel":      {},
	"application/vnd.ms-powerpoint": {},
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": {},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         {},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   {},
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
	"text/csv":   {},
	"text/plain": {},
}

var attachmentContentTypesByExtension = map[string]string{
	".csv":  "text/csv",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".pdf":  "application/pdf",
	".png":  "image/png",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".txt":  "text/plain",
	".webp": "image/webp",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
}

type Handler struct {
	store  Store
	mailer Mailer
	logger *slog.Logger
}

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	var store Store
	if deps.DB != nil {
		store = NewSQLStore(deps.DB)
	}
	h := &Handler{
		store:  store,
		mailer: deps.Mailer,
		logger: logger.With("component", component),
	}
	mux.HandleFunc("POST /support/v1/requests", h.handleCreateRequest)
}

func (h *Handler) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "support_database_not_configured")
		return
	}

	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "missing_auth_claims")
		return
	}

	payload, attachments, err := decodeCreateRequest(w, r)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, requestErrorCode(err))
		return
	}

	input, err := buildCreateInput(payload, claims)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	input.Attachments = attachments

	id, err := h.store.CreateRequest(r.Context(), input)
	if err != nil {
		if isSupportStoreNotReady(err) {
			h.requestLogger(r).Warn("support database not ready", "error", err)
			httputil.Error(w, http.StatusServiceUnavailable, "support_database_not_ready")
			return
		}
		h.internalError(w, r, err, "support_request_insert_failed")
		return
	}

	emailStatus := emailNotificationSkipped
	recipients, err := h.store.GetStringListConfig(r.Context(), configNamespaceSupport, configKeyEmailTo)
	if err != nil {
		emailStatus = emailNotificationFailed
		h.requestLogger(r).Warn("support notification config failed", "request_id", id, "error", err)
	} else {
		emailStatus, err = sendSupportNotification(r.Context(), h.mailer, input, id, recipients)
		if err != nil {
			h.requestLogger(r).Warn("support notification email failed", "request_id", id, "error", err)
		}
	}

	if err := h.store.UpdateEmailStatus(r.Context(), id, emailStatus, claims); err != nil {
		h.requestLogger(r).Warn("support email status update failed", "request_id", id, "email_status", emailStatus, "error", err)
	}

	httputil.JSON(w, http.StatusCreated, createRequestResponse{
		ID:                id,
		Status:            "open",
		EmailNotification: emailStatus,
	})
}

type requestValidationError string

func (e requestValidationError) Error() string {
	return string(e)
}

func requestErrorCode(err error) string {
	var validation requestValidationError
	if errors.As(err, &validation) {
		return string(validation)
	}
	return "invalid_request"
}

func decodeCreateRequest(w http.ResponseWriter, r *http.Request) (createRequestPayload, []CreateRequestAttachment, error) {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType == "multipart/form-data" {
		return decodeMultipartCreatePayload(w, r)
	}
	payload, err := decodeCreatePayload(w, r)
	return payload, nil, err
}

func decodeCreatePayload(w http.ResponseWriter, r *http.Request) (createRequestPayload, error) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	return decodeCreatePayloadBytesFromReader(r.Body)
}

func decodeCreatePayloadBytes(raw []byte) (createRequestPayload, error) {
	return decodeCreatePayloadBytesFromReader(bytes.NewReader(raw))
}

func decodeCreatePayloadBytesFromReader(reader io.Reader) (createRequestPayload, error) {
	var payload createRequestPayload
	dec := json.NewDecoder(reader)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		return payload, requestValidationError("invalid_request")
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return payload, requestValidationError("invalid_request")
	}
	return payload, nil
}

func decodeMultipartCreatePayload(w http.ResponseWriter, r *http.Request) (createRequestPayload, []CreateRequestAttachment, error) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBodyBytes)
	if err := r.ParseMultipartForm(maxMultipartBodyBytes); err != nil {
		return createRequestPayload{}, nil, requestValidationError("invalid_request")
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	payloadValues := r.MultipartForm.Value["payload"]
	if len(payloadValues) != 1 || strings.TrimSpace(payloadValues[0]) == "" {
		return createRequestPayload{}, nil, requestValidationError("invalid_request")
	}
	payload, err := decodeCreatePayloadBytes([]byte(payloadValues[0]))
	if err != nil {
		return createRequestPayload{}, nil, err
	}

	attachments, err := decodeMultipartAttachments(r.MultipartForm.File["attachments"])
	if err != nil {
		return createRequestPayload{}, nil, err
	}
	return payload, attachments, nil
}

func decodeMultipartAttachments(fileHeaders []*multipart.FileHeader) ([]CreateRequestAttachment, error) {
	if len(fileHeaders) == 0 {
		return nil, nil
	}
	if len(fileHeaders) > maxAttachmentCount {
		return nil, requestValidationError("too_many_attachments")
	}

	attachments := make([]CreateRequestAttachment, 0, len(fileHeaders))
	var totalBytes int64
	for _, header := range fileHeaders {
		if header == nil || header.Size == 0 {
			return nil, requestValidationError("empty_attachment")
		}
		if header.Size > maxAttachmentBytes {
			return nil, requestValidationError("attachment_too_large")
		}

		file, err := header.Open()
		if err != nil {
			return nil, requestValidationError("invalid_attachment")
		}
		content, readErr := io.ReadAll(io.LimitReader(file, maxAttachmentBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return nil, requestValidationError("invalid_attachment")
		}
		if len(content) == 0 {
			return nil, requestValidationError("empty_attachment")
		}
		if len(content) > maxAttachmentBytes {
			return nil, requestValidationError("attachment_too_large")
		}

		totalBytes += int64(len(content))
		if totalBytes > maxAttachmentTotalBytes {
			return nil, requestValidationError("attachments_too_large")
		}

		filename, err := sanitizeAttachmentFilename(header.Filename)
		if err != nil {
			return nil, err
		}
		contentType, err := resolveAttachmentContentType(header, content)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(content)
		attachments = append(attachments, CreateRequestAttachment{
			Filename:      filename,
			ContentType:   contentType,
			SizeBytes:     int64(len(content)),
			ContentSHA256: hex.EncodeToString(sum[:]),
			Content:       content,
		})
	}
	return attachments, nil
}

func sanitizeAttachmentFilename(raw string) (string, error) {
	filename := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	filename = path.Base(filename)
	filename = strings.ReplaceAll(filename, "\x00", "")
	filename = strings.TrimSpace(filename)
	if filename == "" || filename == "." || filename == "/" {
		filename = "attachment"
	}
	if strings.ContainsAny(filename, "\r\n") {
		return "", requestValidationError("invalid_attachment_filename")
	}
	runes := []rune(filename)
	if len(runes) > maxAttachmentFilenameRunes {
		filename = string(runes[:maxAttachmentFilenameRunes])
	}
	return filename, nil
}

func resolveAttachmentContentType(header *multipart.FileHeader, content []byte) (string, error) {
	headerContentType := normalizeMediaType(header.Header.Get("Content-Type"))
	if isAllowedAttachmentContentType(headerContentType) {
		return headerContentType, nil
	}

	sniffBytes := content
	if len(sniffBytes) > 512 {
		sniffBytes = sniffBytes[:512]
	}
	detectedContentType := normalizeMediaType(http.DetectContentType(sniffBytes))
	if isAllowedAttachmentContentType(detectedContentType) {
		return detectedContentType, nil
	}

	if fallback, ok := attachmentContentTypesByExtension[strings.ToLower(path.Ext(header.Filename))]; ok {
		switch headerContentType {
		case "", "application/octet-stream", "application/zip", "application/x-zip-compressed":
			return fallback, nil
		}
	}

	return "", requestValidationError("unsupported_attachment_type")
}

func normalizeMediaType(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(mediaType))
}

func isAllowedAttachmentContentType(contentType string) bool {
	if contentType == "" {
		return false
	}
	_, ok := allowedAttachmentContentTypes[contentType]
	return ok
}

func buildCreateInput(payload createRequestPayload, claims auth.Claims) (CreateRequestInput, error) {
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		return CreateRequestInput{}, errors.New("message_required")
	}
	if len(message) > maxMessageLength {
		return CreateRequestInput{}, errors.New("message_too_long")
	}

	priority := normalizePriority(payload.Priority)
	if priority == "" {
		return CreateRequestInput{}, errors.New("invalid_priority")
	}

	technicalContextIncluded := true
	if payload.TechnicalContextIncluded != nil {
		technicalContextIncluded = *payload.TechnicalContextIncluded
	}

	contextValue, err := decodeAndSanitizeContext(payload.Context)
	if err != nil {
		return CreateRequestInput{}, errors.New("invalid_context")
	}

	contextMap, _ := contextValue.(map[string]any)
	appID := stringFromContext(contextMap, "app", "id")
	appName := stringFromContext(contextMap, "app", "name")
	pageURL := stringFromContext(contextMap, "page", "url")
	pagePath := stringFromContext(contextMap, "page", "path")
	if appID == "" {
		appID = "unknown"
	}
	if !technicalContextIncluded {
		contextValue = minimalContext(contextMap)
	}

	return CreateRequestInput{
		Priority:                 priority,
		AppID:                    appID,
		AppName:                  appName,
		PageURL:                  pageURL,
		PagePath:                 pagePath,
		Message:                  message,
		Requester:                claims,
		TechnicalContextIncluded: technicalContextIncluded,
		Context:                  contextValue,
	}, nil
}

func minimalContext(context map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{"app", "page"} {
		if value, ok := context[key]; ok {
			result[key] = value
		}
	}
	return result
}

func normalizePriority(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "normal":
		return "normal"
	case "low":
		return "low"
	case "high":
		return "high"
	case "urgent":
		return "urgent"
	default:
		return ""
	}
}

func stringFromContext(context map[string]any, group string, key string) string {
	if context == nil {
		return ""
	}
	rawGroup, ok := context[group].(map[string]any)
	if !ok {
		return ""
	}
	rawValue, ok := rawGroup[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(rawValue)
}

func (h *Handler) internalError(w http.ResponseWriter, r *http.Request, err error, operation string) {
	httputil.InternalError(w, r, err, "support request failed", "component", component, "operation", operation)
}

func (h *Handler) requestLogger(r *http.Request) *slog.Logger {
	if h.logger != nil {
		return h.logger.With("request_id", logging.RequestID(r.Context()))
	}
	return logging.FromContext(r.Context()).With("component", component)
}

func isSupportStoreNotReady(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "3F000", // invalid_schema_name
		"42501", // insufficient_privilege
		"42P01": // undefined_table
		return true
	default:
		return false
	}
}
