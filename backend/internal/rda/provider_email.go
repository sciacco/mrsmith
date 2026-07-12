package rda

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	platformemail "github.com/sciacco/mrsmith/internal/platform/email"
	"github.com/sciacco/mrsmith/internal/platform/emailledger"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const (
	providerEmailPurpose = "provider-email"

	codeInvalidRequest      = "INVALID_REQUEST"
	codePONotFound          = "PO_NOT_FOUND"
	codePOSendForbidden     = "PO_SEND_FORBIDDEN"
	codePOStateChanged      = "PO_STATE_CHANGED"
	codeContactInvalid      = "CONTACT_INVALID"
	codeDocumentInvalid     = "DOCUMENT_INVALID"
	codeAttachmentsTooLarge = "ATTACHMENTS_TOO_LARGE"
	codePOClosedEmailFailed = "PO_CLOSED_EMAIL_FAILED"
	codeEmailFailed         = "EMAIL_FAILED"
)

type ProviderEmailPreparation struct {
	POID           int64                     `json:"po_id"`
	POCode         string                    `json:"po_code"`
	State          string                    `json:"state"`
	Language       string                    `json:"language"`
	Provider       ProviderEmailProvider     `json:"provider"`
	Contacts       []ProviderEmailContact    `json:"contacts"`
	InitialToIDs   []int64                   `json:"initial_to_ids"`
	InitialCCIDs   []int64                   `json:"initial_cc_ids"`
	Subject        string                    `json:"subject"`
	Introduction   string                    `json:"introduction"`
	Conclusion     string                    `json:"conclusion"`
	Templates      ProviderEmailTemplates    `json:"templates"`
	OrderSummary   ProviderEmailOrderSummary `json:"order_summary"`
	RequiredPDF    ProviderEmailRequiredPDF  `json:"required_pdf"`
	Documents      []ProviderEmailDocument   `json:"documents"`
	AcceptedCount  int                       `json:"accepted_count"`
	LastAcceptedAt *time.Time                `json:"last_accepted_at"`
}

type providerEmailReference struct {
	ID            int64  `json:"id"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Email         string `json:"email"`
	ReferenceType string `json:"reference_type"`
}

type providerEmailAttachment struct {
	ID             int64  `json:"id"`
	FileName       string `json:"file_name"`
	AttachmentType string `json:"attachment_type"`
}

type ProviderEmailProvider struct {
	ID          int64  `json:"id"`
	CompanyName string `json:"company_name"`
}
type ProviderEmailContact struct {
	ID            int64  `json:"id"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Email         string `json:"email"`
	ReferenceType string `json:"reference_type"`
}
type ProviderEmailOrderSummary struct {
	Number    string `json:"number"`
	Date      string `json:"date"`
	Requester string `json:"requester"`
}
type ProviderEmailRequiredPDF struct {
	Filename string `json:"filename"`
}
type ProviderEmailDocument struct {
	ID             int64  `json:"id"`
	Filename       string `json:"filename"`
	AttachmentType string `json:"attachment_type"`
}
type ProviderEmailInitialTemplate struct {
	Subject      string `json:"subject"`
	Introduction string `json:"introduction"`
	Conclusion   string `json:"conclusion"`
}
type ProviderEmailTemplates struct {
	IT ProviderEmailInitialTemplate `json:"it"`
	EN ProviderEmailInitialTemplate `json:"en"`
}

type ProviderEmailSendRequest struct {
	Language     string  `json:"language"`
	ToContactIDs []int64 `json:"to_contact_ids"`
	CCContactIDs []int64 `json:"cc_contact_ids"`
	Subject      string  `json:"subject"`
	Introduction string  `json:"introduction"`
	Conclusion   string  `json:"conclusion"`
	DocumentIDs  []int64 `json:"document_ids"`
}

type ProviderEmailSendResponse struct {
	Status         string    `json:"status"`
	POState        string    `json:"po_state"`
	AcceptedCount  int       `json:"accepted_count"`
	LastAcceptedAt time.Time `json:"last_accepted_at"`
}

type providerEmailError struct {
	Status        int
	Code, Message string
	Field         string
	ID            int64
	err           error
}

func (e *providerEmailError) Error() string { return e.Message }
func (e *providerEmailError) Unwrap() error { return e.err }
func newProviderEmailError(status int, code, message string, err error) error {
	return &providerEmailError{Status: status, Code: code, Message: message, err: err}
}

func (r *ProviderEmailSendRequest) validate() error {
	r.Language = strings.ToLower(strings.TrimSpace(r.Language))
	r.Subject = strings.TrimSpace(r.Subject)
	r.Introduction = strings.TrimSpace(r.Introduction)
	r.Conclusion = strings.TrimSpace(r.Conclusion)
	if hasNonPositiveProviderEmailID(r.ToContactIDs) {
		return newProviderEmailError(400, codeInvalidRequest, "I destinatari A selezionati non sono validi", nil)
	}
	if hasNonPositiveProviderEmailID(r.CCContactIDs) {
		return newProviderEmailError(400, codeInvalidRequest, "I destinatari CC selezionati non sono validi", nil)
	}
	if hasNonPositiveProviderEmailID(r.DocumentIDs) {
		return newProviderEmailError(400, codeInvalidRequest, "I documenti selezionati non sono validi", nil)
	}
	r.ToContactIDs = deduplicateProviderEmailIDs(r.ToContactIDs)
	r.CCContactIDs = deduplicateProviderEmailIDs(r.CCContactIDs)
	r.DocumentIDs = deduplicateProviderEmailIDs(r.DocumentIDs)
	toIDs := make(map[int64]struct{}, len(r.ToContactIDs))
	for _, id := range r.ToContactIDs {
		toIDs[id] = struct{}{}
	}
	for _, id := range r.CCContactIDs {
		if _, exists := toIDs[id]; exists {
			return newProviderEmailError(400, codeInvalidRequest, "Lo stesso contatto non può essere selezionato in A e CC", nil)
		}
	}
	if r.Language != "it" && r.Language != "en" {
		return newProviderEmailError(400, codeInvalidRequest, "Seleziona una lingua valida", nil)
	}
	if len(r.ToContactIDs) == 0 {
		return newProviderEmailError(400, codeInvalidRequest, "Seleziona almeno un destinatario A", nil)
	}
	if r.Subject == "" || len([]rune(r.Subject)) > 200 || strings.ContainsAny(r.Subject, "\r\n") {
		return newProviderEmailError(400, codeInvalidRequest, "Oggetto non valido", nil)
	}
	if r.Introduction == "" || r.Conclusion == "" || len([]rune(r.Introduction))+len([]rune(r.Conclusion)) > 10000 {
		return newProviderEmailError(400, codeInvalidRequest, "Testo email non valido", nil)
	}
	return nil
}

func hasNonPositiveProviderEmailID(ids []int64) bool {
	for _, id := range ids {
		if id <= 0 {
			return true
		}
	}
	return false
}

func deduplicateProviderEmailIDs(ids []int64) []int64 {
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

func normalizeProviderEmailLanguage(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "it") {
		return "it"
	}
	return "en"
}
func validProviderEmailAddress(value string) bool {
	a, err := mail.ParseAddress(strings.TrimSpace(value))
	return err == nil && strings.EqualFold(a.Address, strings.TrimSpace(value))
}

func (h *Handler) resolveRequester(ctx context.Context, requester userRef) (userRef, error) {
	if strings.TrimSpace(requester.FirstName) != "" || strings.TrimSpace(requester.LastName) != "" || strings.TrimSpace(requester.Email) == "" {
		return requester, nil
	}
	var raw json.RawMessage
	err := h.arakDB.QueryRowContext(ctx, `SELECT users_int.user_get_by_email($1, false)`, strings.TrimSpace(requester.Email)).Scan(&raw)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "T0JZ0" {
			return requester, nil
		}
		return userRef{}, newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio RDA temporaneamente non disponibile", fmt.Errorf("resolve requester: %w", err))
	}
	var resolved userRef
	if err := json.Unmarshal(raw, &resolved); err != nil {
		return userRef{}, newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio RDA temporaneamente non disponibile", fmt.Errorf("decode requester: %w", err))
	}
	if strings.TrimSpace(resolved.Email) == "" {
		resolved.Email = requester.Email
	}
	return resolved, nil
}

func providerEmailRequesterName(requester userRef) string {
	return strings.TrimSpace(strings.Join([]string{requester.FirstName, requester.LastName}, " "))
}

func (h *Handler) resolveArakUserID(ctx context.Context, email string) (int64, error) {
	if h.arakDB == nil {
		return 0, newProviderEmailError(503, codeDependencyUnavailable, "Servizio RDA temporaneamente non disponibile", nil)
	}
	email = strings.TrimSpace(email)
	var id int64
	err := h.arakDB.QueryRowContext(ctx, `SELECT id FROM users_int."user" WHERE email = $1`, email).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, newProviderEmailError(403, codePOSendForbidden, "Non sei autorizzato a inviare questo ordine", nil)
	}
	if err != nil {
		return 0, newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio RDA temporaneamente non disponibile", fmt.Errorf("resolve Arak user: %w", err))
	}
	return id, nil
}

func providerEmailCorrelation(poID int64) emailledger.Correlation {
	return emailledger.Correlation{App: "rda", EntityType: "po", EntityID: fmt.Sprint(poID), Purpose: providerEmailPurpose}
}

func (h *Handler) handleProviderEmailPreparation(w http.ResponseWriter, r *http.Request) {
	if h.arak == nil || h.arakDB == nil || h.emailLedger == nil || !h.emailLedger.Enabled() {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio email temporaneamente non disponibile", nil))
		return
	}
	poID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || poID <= 0 {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusBadRequest, codeInvalidRequest, "Richiesta non valida", nil))
		return
	}
	email, ok := currentEmail(r)
	if !ok {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusUnauthorized, codeInvalidRequest, "Accesso richiesto", nil))
		return
	}
	po, err := h.fetchPODetail(r, email, strconv.FormatInt(poID, 10))
	if err != nil {
		var upstream *upstreamStatusError
		if errors.As(err, &upstream) && upstream.status == http.StatusNotFound {
			err = newProviderEmailError(http.StatusNotFound, codePONotFound, "Ordine non disponibile", err)
		} else {
			err = newProviderEmailError(http.StatusBadGateway, codeUpstreamUnavailable, "Ordine temporaneamente non disponibile", err)
		}
		h.writeProviderEmailError(w, r, err)
		return
	}
	if po.State != "PENDING_SEND" && po.State != "CLOSED" {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusConflict, codePOStateChanged, "L'ordine non è disponibile per l'invio", nil))
		return
	}
	userID, err := h.resolveArakUserID(r.Context(), email)
	if err == nil {
		var allowed bool
		err = h.arakDB.QueryRowContext(r.Context(), `SELECT rda.purchase_order_can_send($1,$2)`, poID, userID).Scan(&allowed)
		if err != nil {
			err = newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio RDA temporaneamente non disponibile", fmt.Errorf("check Arak purchase order send permission: %w", err))
		} else if !allowed {
			err = newProviderEmailError(http.StatusForbidden, codePOSendForbidden, "Non sei autorizzato a inviare questo ordine", nil)
		}
	}
	if err != nil {
		h.writeProviderEmailError(w, r, err)
		return
	}
	if strings.TrimSpace(po.Code) == "" {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusConflict, codePOStateChanged, "Numero ordine non disponibile", nil))
		return
	}
	requester, err := h.resolveRequester(r.Context(), po.Requester)
	if err != nil {
		h.writeProviderEmailError(w, r, err)
		return
	}
	data := providerEmailTemplateData{CustomerName: po.Provider.CompanyName, OrderNumber: po.Code, OrderDate: po.Created, RequesterFirstName: requester.FirstName, RequesterLastName: requester.LastName}
	templates, err := providerEmailInitialTemplates(data)
	if err != nil {
		h.writeProviderEmailError(w, r, err)
		return
	}
	language := normalizeProviderEmailLanguage(po.Provider.Language)
	selected := templates.EN
	if language == "it" {
		selected = templates.IT
	}
	contacts := make([]ProviderEmailContact, 0, len(po.Provider.Refs))
	validIDs := map[int64]bool{}
	for _, ref := range po.Provider.Refs {
		if ref.ID <= 0 || !validProviderEmailAddress(ref.Email) {
			continue
		}
		contacts = append(contacts, ProviderEmailContact{ID: ref.ID, FirstName: ref.FirstName, LastName: ref.LastName, Email: strings.TrimSpace(ref.Email), ReferenceType: ref.ReferenceType})
		validIDs[ref.ID] = true
	}
	initialTo := []int64{}
	for _, raw := range po.Recipients {
		var ref providerEmailReference
		if json.Unmarshal(raw, &ref) == nil && validIDs[ref.ID] {
			initialTo = append(initialTo, ref.ID)
		}
	}
	initialTo = deduplicateProviderEmailIDs(initialTo)
	if len(initialTo) == 0 && len(contacts) > 0 {
		initialTo = []int64{contacts[0].ID}
	}
	documents := make([]ProviderEmailDocument, 0, len(po.Attachments))
	for _, raw := range po.Attachments {
		var a providerEmailAttachment
		if json.Unmarshal(raw, &a) == nil && a.ID > 0 {
			documents = append(documents, ProviderEmailDocument{ID: a.ID, Filename: a.FileName, AttachmentType: a.AttachmentType})
		}
	}
	corr := providerEmailCorrelation(poID)
	count, err := h.emailLedger.Count(r.Context(), corr)
	if err != nil {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio email temporaneamente non disponibile", fmt.Errorf("count accepted email sends: %w", err)))
		return
	}
	latest, found, err := h.emailLedger.LatestAccepted(r.Context(), corr)
	if err != nil {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio email temporaneamente non disponibile", fmt.Errorf("find latest accepted email send: %w", err)))
		return
	}
	var last *time.Time
	if found {
		value := latest.SentAt
		last = &value
	}
	response := ProviderEmailPreparation{POID: poID, POCode: po.Code, State: po.State, Language: language, Provider: ProviderEmailProvider{ID: po.Provider.ID, CompanyName: po.Provider.CompanyName}, Contacts: contacts, InitialToIDs: initialTo, InitialCCIDs: []int64{}, Subject: selected.Subject, Introduction: selected.Introduction, Conclusion: selected.Conclusion, Templates: templates, OrderSummary: ProviderEmailOrderSummary{Number: po.Code, Date: po.Created, Requester: providerEmailRequesterName(requester)}, RequiredPDF: ProviderEmailRequiredPDF{Filename: po.Code + ".pdf"}, Documents: documents, AcceptedCount: count, LastAcceptedAt: last}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *Handler) handleProviderEmailSend(w http.ResponseWriter, r *http.Request) {
	if h.arak == nil || h.arakDB == nil || h.emailLedger == nil || !h.emailLedger.Enabled() {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio email temporaneamente non disponibile", nil))
		return
	}
	poID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || poID <= 0 {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusBadRequest, codeInvalidRequest, "Richiesta non valida", nil))
		return
	}
	claims, ok := currentClaims(r)
	email := strings.TrimSpace(claims.Email)
	if !ok || !validProviderEmailAddress(email) || strings.TrimSpace(claims.Subject) == "" {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusUnauthorized, codeInvalidRequest, "Accesso richiesto", nil))
		return
	}
	var req ProviderEmailSendRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusBadRequest, codeInvalidRequest, "Richiesta non valida", err))
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusBadRequest, codeInvalidRequest, "Richiesta non valida", err))
		return
	}
	if err := req.validate(); err != nil {
		h.writeProviderEmailError(w, r, err)
		return
	}
	po, err := h.fetchPODetail(r, email, strconv.FormatInt(poID, 10))
	if err != nil {
		var upstream *upstreamStatusError
		if errors.As(err, &upstream) && upstream.status == http.StatusNotFound {
			err = newProviderEmailError(http.StatusNotFound, codePONotFound, "Ordine non disponibile", err)
		} else {
			err = newProviderEmailError(http.StatusBadGateway, codeUpstreamUnavailable, "Ordine temporaneamente non disponibile", err)
		}
		h.writeProviderEmailError(w, r, err)
		return
	}
	initialState := strings.TrimSpace(po.State)
	if initialState != "PENDING_SEND" && initialState != "CLOSED" {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusConflict, codePOStateChanged, "L'ordine non è disponibile per l'invio", nil))
		return
	}
	if strings.TrimSpace(po.Code) == "" {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusConflict, codePOStateChanged, "Numero ordine non disponibile", nil))
		return
	}
	userID, err := h.resolveArakUserID(r.Context(), email)
	if err != nil {
		h.writeProviderEmailError(w, r, err)
		return
	}
	recipients, err := h.validateProviderEmailContacts(r.Context(), po.Provider.ID, req.ToContactIDs, req.CCContactIDs)
	if err != nil {
		h.writeProviderEmailError(w, r, err)
		return
	}
	documents, err := h.validateProviderEmailDocuments(r.Context(), poID, req.DocumentIDs)
	if err != nil {
		h.writeProviderEmailError(w, r, err)
		return
	}
	attachments, totalBytes, err := h.downloadProviderEmailAttachments(r.Context(), email, poID, po.Code, documents)
	if err != nil {
		h.writeProviderEmailError(w, r, err)
		return
	}
	requester, err := h.resolveRequester(r.Context(), po.Requester)
	if err != nil {
		h.writeProviderEmailError(w, r, err)
		return
	}
	rendered, err := renderProviderEmail(providerEmailTemplateData{Language: req.Language, CustomerName: po.Provider.CompanyName, OrderNumber: po.Code, OrderDate: po.Created, RequesterFirstName: requester.FirstName, RequesterLastName: requester.LastName}, req.Introduction, req.Conclusion)
	if err != nil {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusBadRequest, codeInvalidRequest, "Messaggio non valido", err))
		return
	}

	// Lock state and authorize CLOSED sends at one decision boundary. The lock is
	// released before SMTP: holding a database transaction over an external side
	// effect would be worse, though state can unavoidably change after commit.
	tx, txErr := h.arakDB.BeginTx(r.Context(), nil)
	if txErr != nil {
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusServiceUnavailable, codeDependencyUnavailable, "Servizio RDA temporaneamente non disponibile", txErr))
		return
	}
	var currentState string
	err = tx.QueryRowContext(r.Context(), `SELECT state FROM rda.purchase_order WHERE id = $1 AND deleted IS NULL FOR UPDATE`, poID).Scan(&currentState)
	didClose := false
	if err == nil {
		switch currentState {
		case "PENDING_SEND":
			err = tx.QueryRowContext(r.Context(), `SELECT rda.purchase_order_close($1,$2)`, poID, userID).Scan(new(any))
			didClose = err == nil
		case "CLOSED":
			var allowed bool
			err = tx.QueryRowContext(r.Context(), `SELECT rda.purchase_order_can_send($1,$2)`, poID, userID).Scan(&allowed)
			if err == nil && !allowed {
				err = newProviderEmailError(http.StatusForbidden, codePOSendForbidden, "Non sei autorizzato a inviare questo ordine", nil)
			}
		default:
			err = newProviderEmailError(http.StatusConflict, codePOStateChanged, "Lo stato dell'ordine è cambiato", nil)
		}
	}
	if err == nil {
		err = tx.Commit()
	} else {
		_ = tx.Rollback()
	}
	if errors.Is(err, sql.ErrNoRows) {
		err = newProviderEmailError(http.StatusNotFound, codePONotFound, "Ordine non disponibile", err)
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "T0WZ0":
				err = newProviderEmailError(404, codePONotFound, "Ordine non disponibile", err)
			case "T0WZ2":
				err = newProviderEmailError(403, codePOSendForbidden, "Non sei autorizzato a inviare questo ordine", err)
			case "T0WZ3":
				err = newProviderEmailError(409, codePOStateChanged, "Lo stato dell'ordine è cambiato", err)
			}
		}
		h.writeProviderEmailError(w, r, err)
		return
	}
	mailAttachments := make([]platformemail.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		mailAttachments = append(mailAttachments, attachment.emailAttachment())
	}
	msg := platformemail.Message{To: recipients.To, Cc: recipients.CC, Subject: req.Subject, Text: rendered.Text, HTML: rendered.HTML, Attachments: mailAttachments}
	// Once this request has closed the PO (or authorized an already CLOSED PO),
	// finish the bounded send even if the caller disconnects.
	sendCtx, cancelSend := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Second)
	defer cancelSend()
	sent, err := h.emailLedger.Send(sendCtx, msg, providerEmailCorrelation(poID), emailledger.Actor{Subject: claims.Subject, Email: email})
	if err != nil {
		code := codeEmailFailed
		if currentState == "PENDING_SEND" && didClose {
			code = codePOClosedEmailFailed
		}
		h.requestLogger(r, "provider_email_send", "po_id", poID, "initial_state", initialState, "recipient_count", len(recipients.To)+len(recipients.CC), "attachment_count", len(attachments), "attachment_bytes", totalBytes, "code", code).Warn("provider email send failed", "error", err)
		h.writeProviderEmailError(w, r, newProviderEmailError(http.StatusBadGateway, code, "L'email non è stata inviata", nil))
		return
	}
	corr := providerEmailCorrelation(poID)
	count := 1 // The Send result proves at least this accepted send exists.
	latestAt := sent.SentAt
	if refreshedCount, countErr := h.emailLedger.Count(sendCtx, corr); countErr != nil {
		h.requestLogger(r, "provider_email_send", "po_id", poID, "ledger_read", "count").Warn("accepted email ledger refresh failed", "error", countErr)
	} else {
		count = refreshedCount
	}
	if latest, found, latestErr := h.emailLedger.LatestAccepted(sendCtx, corr); latestErr != nil || !found {
		h.requestLogger(r, "provider_email_send", "po_id", poID, "ledger_read", "latest", "found", found).Warn("accepted email ledger refresh failed", "error", latestErr)
	} else {
		latestAt = latest.SentAt
	}
	h.requestLogger(r, "provider_email_send", "po_id", poID, "initial_state", currentState, "recipient_count", len(recipients.To)+len(recipients.CC), "attachment_count", len(attachments), "attachment_bytes", totalBytes).Info("provider email accepted")
	httputil.JSON(w, http.StatusOK, ProviderEmailSendResponse{Status: emailledger.StatusAccepted, POState: "CLOSED", AcceptedCount: count, LastAcceptedAt: latestAt})
}

func (h *Handler) writeProviderEmailError(w http.ResponseWriter, r *http.Request, err error) {
	poID, _ := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	var appErr *providerEmailError
	if errors.As(err, &appErr) {
		if appErr.err != nil {
			operation := "provider_email_prepare"
			if r.Method == http.MethodPost {
				operation = "provider_email_send"
			}
			logger := h.requestLogger(r, operation).With("code", appErr.Code)
			if poID > 0 {
				logger = logger.With("po_id", poID)
			}
			logger.Warn("provider email preparation failed", "error", appErr.err)
		}
		body := map[string]any{"error": appErr.Message, "code": appErr.Code}
		if appErr.Field != "" {
			body["field"] = appErr.Field
		}
		if appErr.ID > 0 {
			body["id"] = appErr.ID
		}
		httputil.JSON(w, appErr.Status, body)
		return
	}
	operation := "provider_email_prepare"
	if r.Method == http.MethodPost {
		operation = "provider_email_send"
	}
	logger := h.requestLogger(r, operation).With("code", codeUpstreamUnavailable)
	if poID > 0 {
		logger = logger.With("po_id", poID)
	}
	logger.Error("provider email preparation failed", "error", err)
	httputil.JSON(w, http.StatusInternalServerError, map[string]string{"error": "Operazione temporaneamente non disponibile", "code": codeUpstreamUnavailable})
}
