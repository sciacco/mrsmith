package rda

import (
	"bytes"
	"encoding/json"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploadAttachmentForwardsSelectedType(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

	rec := httptest.NewRecorder()
	req := multipartAttachmentRequest(t, "other")
	h.handleUploadAttachment(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := arakState.lastMultipartField(t, http.MethodPost, "/arak/rda/v1/po/42/attachment", "attachment_type"); got != "other" {
		t.Fatalf("expected forwarded attachment_type other, got %q", got)
	}
}

func TestUploadAttachmentRejectsInvalidType(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

	rec := httptest.NewRecorder()
	req := multipartAttachmentRequest(t, "invoice")
	h.handleUploadAttachment(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := arakState.count(http.MethodPost, "/arak/rda/v1/po/42/attachment"); got != 0 {
		t.Fatalf("expected no upstream upload, got %d", got)
	}
}

func TestUploadAttachmentKeepsLegacyDefaultWhenTypeIsMissing(t *testing.T) {
	h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{})

	rec := httptest.NewRecorder()
	req := multipartAttachmentRequest(t, "")
	h.handleUploadAttachment(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := arakState.lastMultipartField(t, http.MethodPost, "/arak/rda/v1/po/42/attachment", "attachment_type"); got != "quote" {
		t.Fatalf("expected draft fallback attachment_type quote, got %q", got)
	}
}

func TestSubmitPORequiresTwoQuoteAttachmentsAboveThreshold(t *testing.T) {
	t.Run("rejects when second attachment is not quote", func(t *testing.T) {
		h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
			poDetail: poDetailForSubmit("3000.00", []string{"quote", "other"}),
		})

		rec := httptest.NewRecorder()
		req := authedRDARequest(http.MethodPost, "/rda/v1/pos/42/submit", nil)
		req.SetPathValue("id", "42")
		h.handleSubmitPO(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := arakState.count(http.MethodPost, "/arak/rda/v1/po/42/submit"); got != 0 {
			t.Fatalf("expected no upstream submit, got %d", got)
		}
	})

	t.Run("allows two quote attachments", func(t *testing.T) {
		h, arakState := newPaymentValidationHandler(t, paymentValidationFixture{
			poDetail: poDetailForSubmit("3000.00", []string{"quote", "quote"}),
		})

		rec := httptest.NewRecorder()
		req := authedRDARequest(http.MethodPost, "/rda/v1/pos/42/submit", nil)
		req.SetPathValue("id", "42")
		h.handleSubmitPO(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := arakState.count(http.MethodPost, "/arak/rda/v1/po/42/submit"); got != 1 {
			t.Fatalf("expected one upstream submit, got %d", got)
		}
	})
}

func multipartAttachmentRequest(t *testing.T, attachmentType string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if attachmentType != "" {
		if err := writer.WriteField("attachment_type", attachmentType); err != nil {
			t.Fatalf("failed to write attachment_type field: %v", err)
		}
	}
	part, err := writer.CreateFormFile("file", "document.pdf")
	if err != nil {
		t.Fatalf("failed to create file part: %v", err)
	}
	if _, err := part.Write([]byte("%PDF-1.4")); err != nil {
		t.Fatalf("failed to write file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := authedRDARequest(http.MethodPost, "/rda/v1/pos/42/attachments", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetPathValue("id", "42")
	return req
}

func poDetailForSubmit(total string, attachmentTypes []string) string {
	attachments := make([]map[string]any, 0, len(attachmentTypes))
	for index, attachmentType := range attachmentTypes {
		attachments = append(attachments, map[string]any{
			"id":              index + 1,
			"attachment_type": attachmentType,
			"file_id":         index + 100,
			"file_name":       "document.pdf",
		})
	}
	body := map[string]any{
		"id":          42,
		"state":       "DRAFT",
		"total_price": total,
		"requester":   map[string]any{"email": "user@example.com"},
		"rows":        []map[string]any{{"id": 10}},
		"attachments": attachments,
	}
	encoded, _ := json.Marshal(body)
	return string(encoded)
}

func (s *paymentValidationArakState) lastMultipartField(t *testing.T, method string, path string, field string) string {
	t.Helper()
	request := s.lastRequest(t, method, path)
	mediaType, params, err := mime.ParseMediaType(request.header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("failed to parse content type: %v", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("expected multipart content type, got %q", mediaType)
	}
	reader := multipart.NewReader(bytes.NewReader(request.body), params["boundary"])
	form, err := reader.ReadForm(1 << 20)
	if err != nil {
		t.Fatalf("failed to parse multipart body: %v", err)
	}
	defer func() {
		_ = form.RemoveAll()
	}()
	values := form.Value[field]
	if len(values) == 0 {
		t.Fatalf("missing multipart field %q", field)
	}
	return values[0]
}

func (s *paymentValidationArakState) lastRequest(t *testing.T, method string, path string) capturedArakRequest {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := len(s.requests) - 1; index >= 0; index-- {
		request := s.requests[index]
		if request.method == method && request.path == path {
			return request
		}
	}
	t.Fatalf("missing forwarded request %s %s", method, path)
	return capturedArakRequest{}
}
