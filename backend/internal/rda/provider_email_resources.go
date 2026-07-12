package rda

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"path/filepath"
	"strconv"
	"strings"

	platformemail "github.com/sciacco/mrsmith/internal/platform/email"
)

const providerEmailAttachmentLimit int64 = 25 << 20

type providerEmailRecipients struct {
	To []string
	CC []string
}

type providerEmailDocumentResource struct {
	ID             int64
	Filename       string
	AttachmentType string
}

type providerEmailDownloadedAttachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

func resourceProviderEmailError(status int, code, message, field string, id int64, err error) error {
	return &providerEmailError{Status: status, Code: code, Message: message, Field: field, ID: id, err: err}
}

// validateProviderEmailContacts resolves addresses from the authoritative provider
// references. Request order is retained; duplicate addresses are compared case-insensitively,
// and an address selected in To is always omitted from CC.
func (h *Handler) validateProviderEmailContacts(ctx context.Context, providerID int64, toIDs, ccIDs []int64) (providerEmailRecipients, error) {
	if h.arakDB == nil {
		return providerEmailRecipients{}, newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio RDA temporaneamente non disponibile", nil)
	}
	resolve := func(id int64, field string) (string, error) {
		var address string
		err := h.arakDB.QueryRowContext(ctx, `SELECT email FROM provider_qualifications.provider_ref WHERE id = $1 AND provider_id = $2`, id, providerID).Scan(&address)
		if errors.Is(err, sql.ErrNoRows) {
			return "", resourceProviderEmailError(http.StatusBadRequest, codeContactInvalid, "Il contatto selezionato non è più disponibile", field, id, nil)
		}
		if err != nil {
			return "", newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio RDA temporaneamente non disponibile", fmt.Errorf("validate provider contact: %w", err))
		}
		address = strings.TrimSpace(address)
		parsed, err := mail.ParseAddress(address)
		if err != nil || !strings.EqualFold(parsed.Address, address) {
			return "", resourceProviderEmailError(http.StatusBadRequest, codeContactInvalid, "Il contatto selezionato non ha un indirizzo email valido", field, id, nil)
		}
		return address, nil
	}

	result := providerEmailRecipients{To: make([]string, 0, len(toIDs)), CC: make([]string, 0, len(ccIDs))}
	toSeen := make(map[string]struct{}, len(toIDs))
	for _, id := range toIDs {
		address, err := resolve(id, "to_contact_ids")
		if err != nil {
			return providerEmailRecipients{}, err
		}
		key := strings.ToLower(address)
		if _, exists := toSeen[key]; !exists {
			toSeen[key] = struct{}{}
			result.To = append(result.To, address)
		}
	}
	ccSeen := make(map[string]struct{}, len(ccIDs))
	for _, id := range ccIDs {
		address, err := resolve(id, "cc_contact_ids")
		if err != nil {
			return providerEmailRecipients{}, err
		}
		key := strings.ToLower(address)
		if _, inTo := toSeen[key]; inTo {
			continue
		}
		if _, exists := ccSeen[key]; !exists {
			ccSeen[key] = struct{}{}
			result.CC = append(result.CC, address)
		}
	}
	if len(result.To) == 0 {
		return providerEmailRecipients{}, resourceProviderEmailError(http.StatusBadRequest, codeContactInvalid, "Seleziona almeno un destinatario A valido", "to_contact_ids", 0, nil)
	}
	return result, nil
}

func (h *Handler) validateProviderEmailDocuments(ctx context.Context, poID int64, ids []int64) ([]providerEmailDocumentResource, error) {
	if h.arakDB == nil {
		return nil, newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio RDA temporaneamente non disponibile", nil)
	}
	result := make([]providerEmailDocumentResource, 0, len(ids))
	for _, id := range ids {
		var item providerEmailDocumentResource
		item.ID = id
		err := h.arakDB.QueryRowContext(ctx, `SELECT f.name, COALESCE(poa.attachment_type, '') FROM rda.purchase_order_attachment poa JOIN files.file f ON f.id = poa.file_id AND f.deleted_at IS NULL WHERE poa.id = $1 AND poa.order_id = $2 AND poa.deleted IS NULL`, id, poID).Scan(&item.Filename, &item.AttachmentType)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, resourceProviderEmailError(http.StatusBadRequest, codeDocumentInvalid, "Il documento selezionato non è più disponibile", "document_ids", id, nil)
		}
		if err != nil {
			return nil, newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio RDA temporaneamente non disponibile", fmt.Errorf("validate PO document: %w", err))
		}
		result = append(result, item)
	}
	return result, nil
}

func (a providerEmailDownloadedAttachment) emailAttachment() platformemail.Attachment {
	return platformemail.Attachment{Filename: a.Filename, ContentType: a.ContentType, Content: bytes.NewReader(a.Content)}
}

// downloadProviderEmailAttachments downloads the mandatory PDF followed by only
// the validated selected documents. The byte budget is cumulative and based on
// bytes actually read.
func (h *Handler) downloadProviderEmailAttachments(ctx context.Context, email string, poID int64, poCode string, documents []providerEmailDocumentResource) ([]providerEmailDownloadedAttachment, int64, error) {
	if h.arak == nil {
		return nil, 0, newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio RDA temporaneamente non disponibile", nil)
	}
	result := make([]providerEmailDownloadedAttachment, 0, len(documents)+1)
	var total int64
	download := func(path, fallbackName, fallbackMIME, field string, id int64) error {
		resp, err := h.arak.DoWithHeadersContext(ctx, http.MethodGet, path, "", nil, requesterHeaders(email))
		if err != nil {
			return resourceProviderEmailError(http.StatusBadGateway, codeDependencyUnavailable, "Servizio documenti temporaneamente non disponibile", field, id, fmt.Errorf("download provider email attachment: %w", err))
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			return resourceProviderEmailError(http.StatusBadGateway, codeDependencyUnavailable, "Servizio documenti temporaneamente non disponibile", field, id, fmt.Errorf("attachment upstream status %d", resp.StatusCode))
		}
		remaining := providerEmailAttachmentLimit - total
		body, err := io.ReadAll(io.LimitReader(resp.Body, remaining+1))
		if err != nil {
			return resourceProviderEmailError(http.StatusBadGateway, codeDependencyUnavailable, "Servizio documenti temporaneamente non disponibile", field, id, fmt.Errorf("read provider email attachment: %w", err))
		}
		if len(body) == 0 {
			return resourceProviderEmailError(http.StatusBadRequest, codeDocumentInvalid, "Il documento selezionato non è più disponibile", field, id, nil)
		}
		if int64(len(body)) > remaining {
			return resourceProviderEmailError(http.StatusBadRequest, codeAttachmentsTooLarge, "Gli allegati superano il limite complessivo di 25 MB", field, id, nil)
		}
		total += int64(len(body))
		result = append(result, providerEmailDownloadedAttachment{Filename: safeProviderEmailFilename(resp.Header.Get("Content-Disposition"), fallbackName), ContentType: safeProviderEmailMIME(resp.Header.Get("Content-Type"), fallbackMIME), Content: body})
		return nil
	}
	base := arakRDARoot + "/po/" + strconv.FormatInt(poID, 10)
	if err := download(base+"/download", safeFilenameFallback(poCode+".pdf", "ordine.pdf"), "application/pdf", "required_pdf", 0); err != nil {
		return nil, total, err
	}
	for _, document := range documents {
		path := base + "/attachment/" + strconv.FormatInt(document.ID, 10) + "/download"
		if err := download(path, safeFilenameFallback(document.Filename, "documento"), "application/octet-stream", "document_ids", document.ID); err != nil {
			return nil, total, err
		}
	}
	return result, total, nil
}

func safeProviderEmailFilename(disposition, fallback string) string {
	name := ""
	if _, params, err := mime.ParseMediaType(disposition); err == nil {
		name = params["filename"]
	}
	return safeFilenameFallback(name, fallback)
}

func safeFilenameFallback(name, fallback string) string {
	clean := func(value string) string {
		value = filepath.Base(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
		value = strings.Map(func(r rune) rune {
			if r < 32 || r == 127 {
				return -1
			}
			return r
		}, value)
		if value == "." || value == "" {
			return ""
		}
		return value
	}
	if value := clean(name); value != "" {
		return value
	}
	if value := clean(fallback); value != "" {
		return value
	}
	return "documento"
}

func safeProviderEmailMIME(value, fallback string) string {
	if mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value)); err == nil && strings.Contains(mediaType, "/") {
		return strings.ToLower(mediaType)
	}
	return fallback
}
