package rda

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/authz"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	component        = "rda"
	arakRDARoot      = "/arak/rda/v1"
	arakBudgetRoot   = "/arak/budget/v1"
	arakUsersRoot    = "/arak/users-int/v1"
	arakProviderRoot = "/arak/provider-qualification/v1"
	maxUploadBytes   = 25 << 20

	codeDependencyUnavailable = "DEPENDENCY_UNAVAILABLE"
	codeUpstreamUnavailable   = "UPSTREAM_UNAVAILABLE"
	codeUpstreamAuthFailed    = "UPSTREAM_AUTH_FAILED"
)

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	h := &Handler{
		arak:   deps.Arak,
		arakDB: deps.ArakDB,
		logger: logger.With("component", component),
	}

	protect := acl.RequireRole(applaunch.RDAAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /rda/v1/me/permissions", h.handlePermissions)
	handle("GET /rda/v1/budgets", h.handleBudgets)
	handle("GET /rda/v1/payment-methods", h.handlePaymentMethods)
	handle("GET /rda/v1/payment-methods/default", h.handleDefaultPaymentMethod)
	handle("GET /rda/v1/articles", h.handleArticles)
	handle("GET /rda/v1/users", h.handleUsers)

	handle("GET /rda/v1/pos", h.handlePOList)
	handle("GET /rda/v1/pos/inbox/level1-2", h.handlePOInbox)
	handle("GET /rda/v1/pos/inbox/leasing", h.handlePOInbox)
	handle("GET /rda/v1/pos/inbox/no-leasing", h.handlePOInbox)
	handle("GET /rda/v1/pos/inbox/payment-method", h.handlePOInbox)
	handle("GET /rda/v1/pos/inbox/budget-increment", h.handlePOInbox)
	handle("POST /rda/v1/pos", h.handleCreatePO)
	handle("GET /rda/v1/pos/{id}", h.handleGetPO)
	handle("PATCH /rda/v1/pos/{id}", h.handlePatchPO)
	handle("DELETE /rda/v1/pos/{id}", h.handleDeletePO)

	handle("POST /rda/v1/pos/{id}/submit", h.handleSubmitPO)
	handle("POST /rda/v1/pos/{id}/approve", h.handleApprovePO)
	handle("POST /rda/v1/pos/{id}/reject", h.handleRejectPO)
	handle("POST /rda/v1/pos/{id}/leasing/approve", h.handleRoleTransition(applaunch.RDAApproverAFCRoles(), "/leasing/approve", "approva leasing"))
	handle("POST /rda/v1/pos/{id}/leasing/reject", h.handleRoleTransition(applaunch.RDAApproverAFCRoles(), "/leasing/reject", "rifiuta leasing"))
	handle("POST /rda/v1/pos/{id}/leasing/created", h.handleRoleTransition(applaunch.RDAApproverAFCRoles(), "/leasing/created", "leasing creato"))
	handle("POST /rda/v1/pos/{id}/no-leasing/approve", h.handleRoleTransition(applaunch.RDAApproverNoLeasingRoles(), "/no-leasing/approve", "approva no leasing"))
	handle("POST /rda/v1/pos/{id}/payment-method/approve", h.handleRoleTransition(applaunch.RDAApproverAFCRoles(), "/payment-method/approve", "approva metodo pagamento"))
	handle("PATCH /rda/v1/pos/{id}/payment-method", h.handlePatchPaymentMethod)
	handle("POST /rda/v1/pos/{id}/budget-increment/approve", h.handleBudgetIncrement(true))
	handle("POST /rda/v1/pos/{id}/budget-increment/reject", h.handleBudgetIncrement(false))
	handle("POST /rda/v1/pos/{id}/conformity/confirm", h.handleStateTransition("PENDING_VERIFICATION", "/confirm-conformity", "conferma conformita"))
	handle("POST /rda/v1/pos/{id}/conformity/reject", h.handleStateTransition("PENDING_VERIFICATION", "/reject-conformity", "rifiuta conformita"))
	handle("POST /rda/v1/pos/{id}/send-to-provider", h.handleStateTransition("PENDING_SEND", "/send-to-provider", "invia al fornitore"))
	handle("GET /rda/v1/pos/{id}/pdf", h.handlePDF)

	handle("POST /rda/v1/pos/{id}/rows", h.handleCreateRow)
	handle("DELETE /rda/v1/pos/{id}/rows/{rowId}", h.handleDeleteRow)

	handle("POST /rda/v1/pos/{id}/attachments", h.handleUploadAttachment)
	handle("GET /rda/v1/pos/{id}/attachments/{aid}", h.handleDownloadAttachment)
	handle("DELETE /rda/v1/pos/{id}/attachments/{aid}", h.handleDeleteAttachment)

	handle("GET /rda/v1/pos/{id}/comments", h.handleComments)
	handle("POST /rda/v1/pos/{id}/comments", h.handlePostComment)
}

func (h *Handler) requestLogger(r *http.Request, operation string, attrs ...any) *slog.Logger {
	args := []any{"component", component, "operation", operation}
	args = append(args, attrs...)
	return logging.FromContext(r.Context()).With(args...)
}

func (h *Handler) requireArak(w http.ResponseWriter) bool {
	if h.arak != nil {
		return true
	}
	httputil.JSON(w, http.StatusServiceUnavailable, map[string]string{
		"error": "Servizio RDA temporaneamente non disponibile",
		"code":  codeDependencyUnavailable,
	})
	return false
}

func (h *Handler) requireArakDB(w http.ResponseWriter) bool {
	if h.arakDB != nil {
		return true
	}
	httputil.JSON(w, http.StatusServiceUnavailable, map[string]string{
		"error": "Catalogo RDA temporaneamente non disponibile",
		"code":  codeDependencyUnavailable,
	})
	return false
}

func currentClaims(r *http.Request) (auth.Claims, bool) {
	return auth.GetClaims(r.Context())
}

func currentEmail(r *http.Request) (string, bool) {
	claims, ok := currentClaims(r)
	if !ok {
		return "", false
	}
	email := strings.TrimSpace(claims.Email)
	if email == "" || !strings.Contains(email, "@") {
		return "", false
	}
	return email, true
}

func requesterHeaders(email string) http.Header {
	return http.Header{"Requester-Email": []string{email}}
}

func budgetHeaders(email string) http.Header {
	return http.Header{"user_email": []string{email}}
}

func jsonHeaders() http.Header {
	return http.Header{"Content-Type": []string{"application/json"}}
}

func queryWithDefaults(r *http.Request) string {
	values := r.URL.Query()
	if values.Get("page_number") == "" {
		values.Set("page_number", "1")
	}
	if values.Get("disable_pagination") == "" {
		values.Set("disable_pagination", "true")
	}
	return values.Encode()
}

func (h *Handler) forwardArak(w http.ResponseWriter, r *http.Request, method, path, rawQuery string, body io.Reader, headers http.Header) {
	resp, err := h.arak.DoWithHeaders(method, path, rawQuery, body, headers)
	if err != nil {
		h.requestLogger(r, "proxy_to_arak", "upstream_path", path).Error("upstream request failed", "error", err)
		httputil.JSON(w, http.StatusBadGateway, map[string]string{
			"error": "Servizio RDA temporaneamente non disponibile",
			"code":  codeUpstreamUnavailable,
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		httputil.JSON(w, http.StatusBadGateway, map[string]string{
			"error": "Autorizzazione verso il servizio RDA non riuscita",
			"code":  codeUpstreamAuthFailed,
		})
		return
	}

	copyResponseHeaders(w.Header(), resp.Header)
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		h.requestLogger(r, "proxy_to_arak", "upstream_path", path).Warn("failed to stream upstream response", "error", err)
	}
}

func copyResponseHeaders(dst, src http.Header) {
	for _, key := range []string{"Content-Type", "Content-Disposition", "Content-Length"} {
		if value := src.Get(key); value != "" {
			dst.Set(key, value)
		}
	}
}

func encodeJSONBody(value any) (*bytes.Reader, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(value); err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}

func (h *Handler) handleBudgets(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, ok := currentEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return
	}
	h.forwardArak(w, r, http.MethodGet, arakBudgetRoot+"/budget-for-user", queryWithDefaults(r), nil, budgetHeaders(email))
}

func (h *Handler) handleArticles(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	values := r.URL.Query()
	if search := strings.TrimSpace(values.Get("search")); search != "" && values.Get("search_string") == "" {
		values.Set("search_string", search)
	}
	if values.Get("page_number") == "" {
		values.Set("page_number", "1")
	}
	if values.Get("disable_pagination") == "" {
		values.Set("disable_pagination", "true")
	}
	h.forwardArak(w, r, http.MethodGet, arakRDARoot+"/article", values.Encode(), nil, nil)
}

func (h *Handler) handleUsers(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	values := r.URL.Query()
	if search := strings.TrimSpace(values.Get("search")); search != "" {
		values.Set("search_string", search)
		values.Del("search")
	}
	if values.Get("enabled") == "" {
		values.Set("enabled", "true")
	}
	if values.Get("page_number") == "" {
		values.Set("page_number", "1")
	}
	if values.Get("disable_pagination") == "" {
		values.Set("disable_pagination", "true")
	}
	h.forwardArak(w, r, http.MethodGet, arakUsersRoot+"/user", values.Encode(), nil, nil)
}

func (h *Handler) handlePOList(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, ok := currentEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return
	}
	h.forwardArak(w, r, http.MethodGet, arakRDARoot+"/po", queryWithDefaults(r), nil, requesterHeaders(email))
}

func (h *Handler) handlePOInbox(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, ok := currentEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return
	}
	cfg, ok := inboxConfig(inboxKindFromRequest(r))
	if !ok {
		httputil.Error(w, http.StatusNotFound, "Pagina non disponibile")
		return
	}
	if !h.hasAnyRole(r, cfg.roles...) {
		httputil.Error(w, http.StatusForbidden, "Operazione riservata agli utenti abilitati")
		return
	}
	h.forwardArak(w, r, http.MethodGet, arakRDARoot+cfg.upstreamPath, queryWithDefaults(r), nil, requesterHeaders(email))
}

func (h *Handler) handleGetPO(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, ok := currentEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return
	}
	h.forwardArak(w, r, http.MethodGet, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id")), r.URL.RawQuery, nil, requesterHeaders(email))
}

func (h *Handler) handleCreatePO(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, ok := currentEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return
	}
	var req createPORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
		return
	}
	if err := validateCreatePO(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	provider := h.fetchProviderForCreate(r, req.ProviderID)
	paymentMethod := strings.TrimSpace(req.PaymentMethod)
	if paymentMethod == "" {
		paymentMethod = providerDefaultPaymentMethod(provider)
	}
	if paymentMethod == "" {
		if !h.requireArakDB(w) {
			return
		}
		code, err := h.defaultPaymentMethodCode(r.Context())
		if err != nil {
			httputil.InternalError(w, r, err, "rda default payment method query failed")
			return
		}
		paymentMethod = code
	}
	if paymentMethod == "" {
		httputil.Error(w, http.StatusBadRequest, "Seleziona un metodo di pagamento")
		return
	}

	body := map[string]any{
		"type":                req.Type,
		"budget_id":           req.BudgetID,
		"provider_id":         req.ProviderID,
		"project":             req.Project,
		"object":              req.Object,
		"reference_warehouse": "MILANO",
		"currency":            "EUR",
		"language":            providerLanguage(provider),
		"payment_method":      paymentMethod,
		"recipient_ids":       []int64{},
	}
	if strings.TrimSpace(req.CostCenter) != "" {
		body["cost_center"] = strings.TrimSpace(req.CostCenter)
	} else {
		body["budget_user_id"] = req.BudgetUserID
	}
	if req.Description != "" {
		body["description"] = req.Description
	}
	if req.Note != "" {
		body["note"] = req.Note
	}
	if req.ProviderOfferCode != "" {
		body["provider_offer_code"] = req.ProviderOfferCode
	}
	if req.ProviderOfferDate != "" {
		body["provider_offer_date"] = req.ProviderOfferDate
	}
	if cap := providerCAP(provider); cap != "" {
		body["cap"] = cap
	}
	if vat := strings.TrimSpace(provider.VATNumber); vat != "" {
		body["vat"] = vat
	}
	encoded, err := encodeJSONBody(body)
	if err != nil {
		httputil.InternalError(w, r, err, "rda create body encode failed")
		return
	}
	h.forwardArak(w, r, http.MethodPost, arakRDARoot+"/po", "", encoded, mergeHeaders(requesterHeaders(email), jsonHeaders()))
}

func (h *Handler) handlePatchPO(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, po, ok := h.loadPOForWrite(w, r)
	if !ok {
		return
	}
	if !isRequester(po, email) || po.State != "DRAFT" {
		httputil.Error(w, http.StatusForbidden, "La bozza puo essere modificata solo dal richiedente")
		return
	}
	body, err := decodeAllowedPatch(r.Body)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	encoded, err := encodeJSONBody(body)
	if err != nil {
		httputil.InternalError(w, r, err, "rda patch body encode failed")
		return
	}
	h.forwardArak(w, r, http.MethodPatch, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id")), "", encoded, mergeHeaders(requesterHeaders(email), jsonHeaders()))
}

func (h *Handler) handleDeletePO(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, po, ok := h.loadPOForWrite(w, r)
	if !ok {
		return
	}
	if !isRequester(po, email) || po.State != "DRAFT" {
		httputil.Error(w, http.StatusForbidden, "La bozza puo essere eliminata solo dal richiedente")
		return
	}
	h.forwardArak(w, r, http.MethodDelete, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id")), "", nil, requesterHeaders(email))
}

func (h *Handler) handlePDF(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, ok := currentEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return
	}
	h.forwardArak(w, r, http.MethodGet, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+"/download", "", nil, requesterHeaders(email))
}

func mergeHeaders(headers ...http.Header) http.Header {
	out := http.Header{}
	for _, header := range headers {
		for key, values := range header {
			for _, value := range values {
				out.Add(key, value)
			}
		}
	}
	return out
}

func (h *Handler) loadPOForWrite(w http.ResponseWriter, r *http.Request) (string, poDetail, bool) {
	email, ok := currentEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return "", poDetail{}, false
	}
	po, err := h.fetchPODetail(r, email, r.PathValue("id"))
	if err != nil {
		h.handleFetchPOError(w, r, err)
		return "", poDetail{}, false
	}
	return email, po, true
}

func (h *Handler) handleFetchPOError(w http.ResponseWriter, r *http.Request, err error) {
	var upstream *upstreamStatusError
	if errors.As(err, &upstream) {
		if upstream.status == http.StatusNotFound {
			httputil.Error(w, http.StatusNotFound, "Richiesta non disponibile")
			return
		}
		if upstream.status == http.StatusUnauthorized || upstream.status == http.StatusForbidden {
			httputil.JSON(w, http.StatusBadGateway, map[string]string{
				"error": "Autorizzazione verso il servizio RDA non riuscita",
				"code":  codeUpstreamAuthFailed,
			})
			return
		}
	}
	h.requestLogger(r, "fetch_po_detail").Error("failed to fetch PO detail", "error", err)
	httputil.JSON(w, http.StatusBadGateway, map[string]string{
		"error": "Richiesta non disponibile in questo momento",
		"code":  codeUpstreamUnavailable,
	})
}

func (h *Handler) hasAnyRole(r *http.Request, roles ...string) bool {
	claims, ok := currentClaims(r)
	return ok && authz.HasAnyRole(claims.Roles, roles...)
}

func (h *Handler) fetchProviderForCreate(r *http.Request, providerID int64) providerDetail {
	if h.arak == nil || providerID <= 0 {
		return providerDetail{}
	}
	path := fmt.Sprintf("%s/provider/%s", arakProviderRoot, url.PathEscape(fmt.Sprint(providerID)))
	resp, err := h.arak.DoWithHeaders(http.MethodGet, path, "", nil, nil)
	if err != nil {
		h.requestLogger(r, "fetch_provider_for_create", "upstream_path", path).Warn("provider detail unavailable", "error", err)
		return providerDetail{}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerDetail{}
	}
	var provider providerDetail
	if err := json.NewDecoder(resp.Body).Decode(&provider); err != nil {
		h.requestLogger(r, "fetch_provider_for_create").Warn("provider detail decode failed", "error", err)
		return providerDetail{}
	}
	return provider
}

func providerLanguage(provider providerDetail) string {
	language := strings.TrimSpace(provider.Language)
	if language == "" {
		return "it"
	}
	return language
}

func providerCAP(provider providerDetail) string {
	if cap := strings.TrimSpace(provider.CAP); cap != "" {
		return cap
	}
	return strings.TrimSpace(provider.PostalCode)
}

func providerDefaultPaymentMethod(provider providerDetail) string {
	raw := bytes.TrimSpace(provider.DefaultPaymentMethod)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var object struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(raw, &object) == nil && strings.TrimSpace(object.Code) != "" {
		return strings.TrimSpace(object.Code)
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&number) == nil {
		return strings.TrimSpace(number.String())
	}
	return ""
}

func decodeAllowedPatch(reader io.Reader) (map[string]any, error) {
	var raw map[string]any
	if err := json.NewDecoder(reader).Decode(&raw); err != nil {
		return nil, errors.New("Richiesta non valida")
	}
	allowed := map[string]struct{}{
		"type": {}, "budget_id": {}, "budget_user_id": {}, "cost_center": {}, "description": {},
		"object": {}, "note": {}, "payment_method": {}, "reference_warehouse": {}, "provider_id": {},
		"project": {}, "provider_offer_code": {}, "provider_offer_date": {}, "recipient_ids": {},
	}
	out := make(map[string]any)
	for key, value := range raw {
		if _, ok := allowed[key]; ok {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil, errors.New("Nessun campo da aggiornare")
	}
	return out, nil
}

func multipartContentType(header *multipart.FileHeader) string {
	if header == nil {
		return ""
	}
	return header.Header.Get("Content-Type")
}
