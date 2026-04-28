package fornitori

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/authz"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	component = "fornitori"
	arakRoot  = "/arak/provider-qualification/v1"

	codeDependencyUnavailable = "DEPENDENCY_UNAVAILABLE"
	codeReadonlyDenied        = "READONLY_DENIED"
	codeSkipRoleRequired      = "SKIP_QUALIFICATION_ROLE_REQUIRED"

	maxUploadBytes = 25 << 20

	qualificationReferenceType = "QUALIFICATION_REF"
)

const alyanteSuppliersQuery = `SELECT TOP 100
       LTRIM(RTRIM(CG16_RAGSOANAG)) AS company_name,
       LTRIM(RTRIM(CONVERT(varchar(30), CG44_CLIFOR))) AS code
FROM CG44_CLIFOR
JOIN CG16_ANAGGEN
  ON CG16_ANAGGEN.CG16_CODICE = CG44_CLIFOR.CG44_CODICE_CG16
WHERE CG44_DITTA_CG18 = 1
  AND CG44_TIPOCF = 1
  AND (CG16_DATAVALID IS NULL OR CG16_DATAVALID >= GETDATE())
  AND (
    CG16_RAGSOANAG LIKE @p1
    OR CONVERT(varchar(30), CG44_CLIFOR) LIKE @p1
  )
ORDER BY CG16_RAGSOANAG`

var allowedUploadTypes = map[string]struct{}{
	"application/pdf":    {},
	"image/jpeg":         {},
	"image/png":          {},
	"image/webp":         {},
	"application/msword": {},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
	"application/vnd.ms-excel": {},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         {},
	"application/vnd.ms-powerpoint":                                             {},
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": {},
}

// allowedProviderReferenceTypes lists the reference types that can be created
// or edited through the generic /provider/{id}/reference proxy. QUALIFICATION_REF
// is intentionally excluded: Mistra is the owner of that contact and it is
// persisted via PUT /provider/{id} (provider-edit `ref` field) instead.
var allowedProviderReferenceTypes = map[string]struct{}{
	"OTHER_REF":          {},
	"ADMINISTRATIVE_REF": {},
	"TECHNICAL_REF":      {},
}

type Handler struct {
	arak      *arak.Client
	db        *sql.DB
	alyanteDB *sql.DB
	logger    *slog.Logger
}

func RegisterRoutes(mux *http.ServeMux, arakClient *arak.Client, arakDB *sql.DB, alyanteDB *sql.DB) {
	h := &Handler{
		arak:      arakClient,
		db:        arakDB,
		alyanteDB: alyanteDB,
		logger:    slog.Default().With("component", component),
	}

	protect := acl.RequireRole(applaunch.FornitoriAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /fornitori/v1/provider", h.proxyArak("/provider", false))
	handle("GET /fornitori/v1/provider/{id}", h.proxyArakPath("/provider/{id}", true))
	handle("POST /fornitori/v1/provider", h.proxyArak("/provider", true))
	handle("PUT /fornitori/v1/provider/{id}", h.handlePutProvider)
	handle("DELETE /fornitori/v1/provider/{id}", h.proxyArakPath("/provider/{id}", true))

	handle("POST /fornitori/v1/provider/{id}/reference", h.handleCreateProviderReference)
	handle("PUT /fornitori/v1/provider/{id}/reference/{ref_id}", h.handleUpdateProviderReference)

	handle("GET /fornitori/v1/provider/{id}/category", h.proxyArakPath("/provider/{id}/category", false))
	handle("POST /fornitori/v1/provider/{id}/category/{category_id}", h.proxyArakPath("/provider/{id}/category/{category_id}", true))

	handle("GET /fornitori/v1/category", h.proxyArak("/category", false))
	handle("GET /fornitori/v1/category/{id}", h.proxyArakPath("/category/{id}", true))
	handle("POST /fornitori/v1/category", h.requireWritable(h.proxyArak("/category", true)))
	handle("PUT /fornitori/v1/category/{id}", h.requireWritable(h.proxyArakPath("/category/{id}", true)))
	handle("DELETE /fornitori/v1/category/{id}", h.requireWritable(h.proxyArakPath("/category/{id}", true)))

	handle("GET /fornitori/v1/document-type", h.proxyArak("/document-type", false))
	handle("POST /fornitori/v1/document-type", h.requireWritable(h.proxyArak("/document-type", true)))
	handle("PUT /fornitori/v1/document-type/{id}", h.requireWritable(h.proxyArakPath("/document-type/{id}", true)))
	handle("DELETE /fornitori/v1/document-type/{id}", h.requireWritable(h.proxyArakPath("/document-type/{id}", true)))

	handle("GET /fornitori/v1/document", h.proxyArak("/document", false))
	handle("POST /fornitori/v1/document", h.handleUploadDocument)
	handle("PATCH /fornitori/v1/document/{id}", h.handlePatchDocument)
	handle("GET /fornitori/v1/document/{id}/download", h.handleDownloadDocument)

	handle("GET /fornitori/v1/provider-summary", h.handleProviderSummary)
	handle("GET /fornitori/v1/alyante-suppliers", h.handleAlyanteSuppliers)

	handle("GET /fornitori/v1/dashboard/drafts", h.handleDashboardDrafts)
	handle("GET /fornitori/v1/dashboard/expiring-documents", h.handleDashboardExpiringDocuments)
	handle("GET /fornitori/v1/dashboard/categories-to-review", h.handleDashboardCategoriesToReview)

	handle("GET /fornitori/v1/payment-method", h.handlePaymentMethods)
	handle("PUT /fornitori/v1/payment-method/{code}/rda-available", h.requireWritable(h.handlePaymentMethodAvailability))
	handle("GET /fornitori/v1/country", h.handleCountries)

	handle("GET /fornitori/v1/article-category", h.handleArticleCategories)
	handle("PUT /fornitori/v1/article-category/{article_code}", h.requireWritable(h.handleArticleCategoryUpdate))
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
		"error": "Servizio fornitori temporaneamente non disponibile",
		"code":  codeDependencyUnavailable,
	})
	return false
}

func (h *Handler) requireDB(w http.ResponseWriter) bool {
	if h.db != nil {
		return true
	}
	httputil.JSON(w, http.StatusServiceUnavailable, map[string]string{
		"error": "Archivio fornitori temporaneamente non disponibile",
		"code":  codeDependencyUnavailable,
	})
	return false
}

func (h *Handler) requireWritable(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.GetClaims(r.Context())
		if ok && slices.Contains(claims.Roles, "app_fornitori_readonly") && !slices.Contains(claims.Roles, authz.DevAdminRole) {
			httputil.JSON(w, http.StatusForbidden, map[string]string{
				"error": "Operazione non consentita per il tuo profilo",
				"code":  codeReadonlyDenied,
			})
			return
		}
		next(w, r)
	}
}

func (h *Handler) proxyArak(path string, defaults bool) http.HandlerFunc {
	return h.proxyToArak(func(*http.Request) string { return path }, defaults)
}

func (h *Handler) proxyArakPath(template string, defaults bool) http.HandlerFunc {
	return h.proxyToArak(func(r *http.Request) string {
		path := template
		for _, key := range []string{"id", "ref_id", "category_id"} {
			if value := r.PathValue(key); value != "" {
				path = strings.ReplaceAll(path, "{"+key+"}", url.PathEscape(value))
			}
		}
		return path
	}, defaults)
}

func (h *Handler) proxyToArak(pathFn func(*http.Request) string, defaultPagination bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.requireArak(w) {
			return
		}
		h.forwardArak(w, r, arakRoot+pathFn(r), queryWithDefaults(r, defaultPagination), r.Body, nil)
	}
}

func queryWithDefaults(r *http.Request, defaultPagination bool) string {
	values := r.URL.Query()
	if defaultPagination {
		return values.Encode()
	}
	if values.Get("page_number") == "" {
		values.Set("page_number", "1")
	}
	if values.Get("disable_pagination") == "" {
		values.Set("disable_pagination", "true")
	}
	return values.Encode()
}

func (h *Handler) forwardArak(w http.ResponseWriter, r *http.Request, path, rawQuery string, body io.Reader, headers http.Header) {
	resp, err := h.arak.DoWithHeaders(r.Method, path, rawQuery, body, headers)
	if err != nil {
		h.requestLogger(r, "proxy_to_arak", "upstream_path", path).Error("upstream request failed", "error", err)
		httputil.JSON(w, http.StatusBadGateway, map[string]string{
			"error": "Servizio fornitori temporaneamente non disponibile",
			"code":  "UPSTREAM_UNAVAILABLE",
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		httputil.JSON(w, http.StatusBadGateway, map[string]string{
			"error": "Autorizzazione verso il servizio fornitori non riuscita",
			"code":  "UPSTREAM_AUTH_FAILED",
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

func (h *Handler) handlePutProvider(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
		return
	}
	var raw map[string]any
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &raw); err != nil {
			httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
			return
		}
		if value, ok := raw["skip_qualification_validation"].(bool); ok && value {
			claims, ok := auth.GetClaims(r.Context())
			if !ok || !authz.HasAnyRole(claims.Roles, applaunch.FornitoriSkipQualificationRoles()...) {
				httputil.JSON(w, http.StatusForbidden, map[string]string{
					"error": "Operazione riservata agli utenti abilitati",
					"code":  codeSkipRoleRequired,
				})
				return
			}
		}
	}
	path := arakRoot + "/provider/" + url.PathEscape(r.PathValue("id"))
	h.forwardArak(w, r, path, r.URL.RawQuery, bytes.NewReader(body), nil)
}

type providerReferencePayload struct {
	FirstName     *string
	LastName      *string
	Email         *string
	Phone         string
	ReferenceType string
}

type providerReferenceForwardOptions struct {
	includeReferenceType bool
	includeEmptyPhone    bool
}

func (h *Handler) handleCreateProviderReference(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeProviderReferencePayload(w, r)
	if !ok {
		return
	}
	if payload.ReferenceType == "" {
		httputil.Error(w, http.StatusBadRequest, "Seleziona il tipo contatto")
		return
	}
	if payload.ReferenceType == qualificationReferenceType {
		httputil.Error(w, http.StatusBadRequest, "QUALIFICATION_REF non puo' essere gestito da /reference: usa PUT /provider/{id}")
		return
	}
	if !isAllowedProviderReferenceType(payload.ReferenceType) {
		httputil.Error(w, http.StatusBadRequest, "Tipo contatto non valido")
		return
	}
	h.forwardProviderReference(
		w,
		r,
		"/provider/"+url.PathEscape(r.PathValue("id"))+"/reference",
		payload,
		providerReferenceForwardOptions{includeReferenceType: true},
	)
}

func (h *Handler) handleUpdateProviderReference(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeProviderReferencePayload(w, r)
	if !ok {
		return
	}
	if payload.ReferenceType == qualificationReferenceType {
		httputil.Error(w, http.StatusBadRequest, "QUALIFICATION_REF non puo' essere gestito da /reference: usa PUT /provider/{id}")
		return
	}
	if payload.ReferenceType != "" && !isAllowedProviderReferenceType(payload.ReferenceType) {
		httputil.Error(w, http.StatusBadRequest, "Tipo contatto non valido")
		return
	}
	h.forwardProviderReference(
		w,
		r,
		"/provider/"+url.PathEscape(r.PathValue("id"))+"/reference/"+url.PathEscape(r.PathValue("ref_id")),
		payload,
		providerReferenceForwardOptions{includeEmptyPhone: true},
	)
}

func decodeProviderReferencePayload(w http.ResponseWriter, r *http.Request) (providerReferencePayload, bool) {
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
		return providerReferencePayload{}, false
	}
	var payload providerReferencePayload
	for _, field := range []struct {
		key string
		dst **string
	}{
		{key: "first_name", dst: &payload.FirstName},
		{key: "last_name", dst: &payload.LastName},
		{key: "email", dst: &payload.Email},
	} {
		value, ok := referencePayloadString(raw, field.key)
		if !ok {
			httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
			return providerReferencePayload{}, false
		}
		if value != "" {
			copied := value
			*field.dst = &copied
		}
	}
	phone, ok := referencePayloadString(raw, "phone")
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
		return providerReferencePayload{}, false
	}
	payload.Phone = phone
	refType, ok := referencePayloadString(raw, "reference_type")
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
		return providerReferencePayload{}, false
	}
	payload.ReferenceType = strings.ToUpper(refType)
	return payload, true
}

func referencePayloadString(raw map[string]any, key string) (string, bool) {
	value, exists := raw[key]
	if !exists || value == nil {
		return "", true
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(text), true
}

func isAllowedProviderReferenceType(value string) bool {
	_, ok := allowedProviderReferenceTypes[value]
	return ok
}

func (p providerReferencePayload) arakBody(opts providerReferenceForwardOptions) ([]byte, error) {
	body := map[string]any{}
	if p.Phone != "" || opts.includeEmptyPhone {
		body["phone"] = p.Phone
	}
	if p.FirstName != nil {
		body["first_name"] = *p.FirstName
	}
	if p.LastName != nil {
		body["last_name"] = *p.LastName
	}
	if p.Email != nil {
		body["email"] = *p.Email
	}
	if opts.includeReferenceType && p.ReferenceType != "" {
		body["reference_type"] = p.ReferenceType
	}
	return json.Marshal(body)
}

func (h *Handler) forwardProviderReference(w http.ResponseWriter, r *http.Request, path string, payload providerReferencePayload, opts providerReferenceForwardOptions) {
	if !h.requireArak(w) {
		return
	}
	body, err := payload.arakBody(opts)
	if err != nil {
		httputil.InternalError(w, r, err, "provider reference body encode failed")
		return
	}
	h.forwardArak(w, r, arakRoot+path, r.URL.RawQuery, bytes.NewReader(body), nil)
}

func (h *Handler) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	h.handleMultipartProxy(w, r, arakRoot+"/document")
}

func (h *Handler) handlePatchDocument(w http.ResponseWriter, r *http.Request) {
	h.handleMultipartProxy(w, r, arakRoot+"/document/"+url.PathEscape(r.PathValue("id")))
}

func (h *Handler) handleMultipartProxy(w http.ResponseWriter, r *http.Request, upstreamPath string) {
	if !h.requireArak(w) {
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxUploadBytes))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Il file supera la dimensione massima o non puo essere letto")
		return
	}
	checkReq := new(http.Request)
	checkReq.Body = io.NopCloser(bytes.NewReader(body))
	checkReq.Header = r.Header.Clone()
	if err := checkReq.ParseMultipartForm(maxUploadBytes); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Il file supera la dimensione massima o non puo essere letto")
		return
	}
	fileHeaders := checkReq.MultipartForm.File["file"]
	if len(fileHeaders) == 0 || fileHeaders[0].Size == 0 {
		httputil.Error(w, http.StatusBadRequest, "Seleziona un file da caricare")
		return
	}
	if !isAllowedUploadType(fileHeaders[0]) {
		httputil.Error(w, http.StatusBadRequest, "Formato file non supportato")
		return
	}
	h.forwardArak(w, r, upstreamPath, r.URL.RawQuery, bytes.NewReader(body), http.Header{"Content-Type": []string{r.Header.Get("Content-Type")}})
}

func isAllowedUploadType(header *multipart.FileHeader) bool {
	if header == nil {
		return false
	}
	contentType := header.Header.Get("Content-Type")
	if _, ok := allowedUploadTypes[contentType]; ok {
		return true
	}
	name := strings.ToLower(header.Filename)
	for _, suffix := range []string{".pdf", ".jpg", ".jpeg", ".png", ".webp", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func (h *Handler) handleDownloadDocument(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	path := arakRoot + "/document/" + url.PathEscape(r.PathValue("id")) + "/download"
	h.forwardArak(w, r, path, r.URL.RawQuery, nil, nil)
}

func (h *Handler) handleDashboardDrafts(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, company_name, state, vat_number, cf, erp_id, updated_at
		FROM provider_qualifications.provider
		WHERE state = 'DRAFT'
		ORDER BY updated_at DESC NULLS LAST, id DESC`)
	if err != nil {
		httputil.InternalError(w, r, err, "dashboard drafts query failed")
		return
	}
	defer rows.Close()

	items := make([]dashboardDraft, 0)
	for rows.Next() {
		var item dashboardDraft
		if err := rows.Scan(&item.ID, &item.CompanyName, &item.State, &item.VATNumber, &item.CF, &item.ERPID, &item.UpdatedAt); err != nil {
			httputil.InternalError(w, r, err, "dashboard drafts scan failed")
			return
		}
		items = append(items, item)
	}
	writeRows(w, r, items, rows.Err())
}

func (h *Handler) handleDashboardExpiringDocuments(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT d.id, d.provider_id, p.company_name, d.file_id, d.expire_date, d.state,
		       dt.name, (d.expire_date::date - CURRENT_DATE)::int AS days_remaining
		FROM provider_qualifications.document d
		INNER JOIN provider_qualifications.provider p ON p.id = d.provider_id
		LEFT JOIN provider_qualifications.document_type dt ON dt.id = d.document_type_id
		WHERE d.expire_date::date <= CURRENT_DATE + INTERVAL '30 days'
		  AND p.state IN ('DRAFT', 'ACTIVE')
		  AND d.deleted_at IS NULL
		  AND p.deleted_at IS NULL
		ORDER BY days_remaining ASC, p.company_name ASC`)
	if err != nil {
		httputil.InternalError(w, r, err, "dashboard expiring documents query failed")
		return
	}
	defer rows.Close()

	items := make([]dashboardDocument, 0)
	for rows.Next() {
		var item dashboardDocument
		if err := rows.Scan(&item.ID, &item.ProviderID, &item.CompanyName, &item.FileID, &item.ExpireDate, &item.State, &item.DocumentType, &item.DaysRemaining); err != nil {
			httputil.InternalError(w, r, err, "dashboard expiring documents scan failed")
			return
		}
		items = append(items, item)
	}
	writeRows(w, r, items, rows.Err())
}

func (h *Handler) handleDashboardCategoriesToReview(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT p.id, p.company_name, sc.id, sc.name, pc.state, pc.critical
		FROM provider_qualifications.provider_category pc
		INNER JOIN provider_qualifications.provider p ON p.id = pc.provider_id
		INNER JOIN provider_qualifications.service_category sc ON sc.id = pc.category_id
		WHERE pc.state IN ('NOT_QUALIFIED', 'NEW') AND p.state IN ('DRAFT', 'ACTIVE')
		ORDER BY pc.critical DESC, p.company_name ASC, sc.name ASC`)
	if err != nil {
		httputil.InternalError(w, r, err, "dashboard categories query failed")
		return
	}
	defer rows.Close()

	items := make([]dashboardCategory, 0)
	for rows.Next() {
		var item dashboardCategory
		if err := rows.Scan(&item.ProviderID, &item.CompanyName, &item.CategoryID, &item.CategoryName, &item.State, &item.Critical); err != nil {
			httputil.InternalError(w, r, err, "dashboard categories scan failed")
			return
		}
		items = append(items, item)
	}
	writeRows(w, r, items, rows.Err())
}

type arakProviderItem struct {
	ID          int64   `json:"id"`
	CompanyName *string `json:"company_name"`
	State       *string `json:"state"`
	VATNumber   *string `json:"vat_number"`
	CF          *string `json:"cf"`
	ERPID       *int64  `json:"erp_id"`
}

type providerSummary struct {
	ID              int64   `json:"id"`
	CompanyName     *string `json:"company_name"`
	State           *string `json:"state"`
	VATNumber       *string `json:"vat_number"`
	CF              *string `json:"cf"`
	ERPID           *int64  `json:"erp_id"`
	QualifiedCount  int     `json:"qualified_count"`
	TotalCount      int     `json:"total_count"`
	HasExpiringDocs bool    `json:"has_expiring_docs"`
}

type alyanteSupplier struct {
	Code        string `json:"code"`
	CompanyName string `json:"company_name"`
}

// handleProviderSummary returns the providers list (anagrafica from Mistra) enriched with
// qualification counts and expiring-document flag computed from the local DB.
// One Mistra call + one SQL query, merged in-process.
func (h *Handler) handleProviderSummary(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) || !h.requireDB(w) {
		return
	}
	upstreamQuery := url.Values{}
	upstreamQuery.Set("page_number", "1")
	upstreamQuery.Set("disable_pagination", "true")
	if q := r.URL.Query().Get("search_string"); q != "" {
		upstreamQuery.Set("search_string", q)
	}
	resp, err := h.arak.Do(http.MethodGet, arakRoot+"/provider", upstreamQuery.Encode(), nil)
	if err != nil {
		h.requestLogger(r, "provider_summary").Error("upstream provider list failed", "error", err)
		httputil.JSON(w, http.StatusBadGateway, map[string]string{
			"error": "Servizio fornitori temporaneamente non disponibile",
			"code":  "UPSTREAM_UNAVAILABLE",
		})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		h.requestLogger(r, "provider_summary").Warn("upstream provider list non-2xx", "status", resp.StatusCode)
		httputil.JSON(w, http.StatusBadGateway, map[string]string{
			"error": "Servizio fornitori temporaneamente non disponibile",
			"code":  "UPSTREAM_UNAVAILABLE",
		})
		return
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		httputil.InternalError(w, r, err, "provider summary upstream read failed")
		return
	}
	var providers []arakProviderItem
	var envelope struct {
		Items []arakProviderItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Items != nil {
		providers = envelope.Items
	} else if err := json.Unmarshal(raw, &providers); err != nil {
		httputil.InternalError(w, r, err, "provider summary decode failed")
		return
	}
	if len(providers) == 0 {
		httputil.JSON(w, http.StatusOK, []providerSummary{})
		return
	}

	type enrichment struct {
		Qualified   int
		Total       int
		HasExpiring bool
	}
	enrichmentByID := make(map[int64]enrichment)
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT
			p.id,
			COALESCE(SUM(CASE WHEN pc.state = 'QUALIFIED' THEN 1 ELSE 0 END), 0)::int AS qualified_count,
			COALESCE(COUNT(pc.id), 0)::int AS total_count,
			EXISTS (
				SELECT 1
				FROM provider_qualifications.document d
				WHERE d.provider_id = p.id
				  AND d.expire_date::date <= CURRENT_DATE + INTERVAL '30 days'
				  AND d.deleted_at IS NULL
			) AS has_expiring_docs
		FROM provider_qualifications.provider p
		LEFT JOIN provider_qualifications.provider_category pc
			ON pc.provider_id = p.id
			AND pc.deleted_at IS NULL
		WHERE p.deleted_at IS NULL
		GROUP BY p.id`)
	if err != nil {
		httputil.InternalError(w, r, err, "provider summary enrichment query failed")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var e enrichment
		if err := rows.Scan(&id, &e.Qualified, &e.Total, &e.HasExpiring); err != nil {
			httputil.InternalError(w, r, err, "provider summary enrichment scan failed")
			return
		}
		enrichmentByID[id] = e
	}
	if err := rows.Err(); err != nil {
		httputil.InternalError(w, r, err, "provider summary enrichment iteration failed")
		return
	}

	items := make([]providerSummary, 0, len(providers))
	for _, p := range providers {
		e := enrichmentByID[p.ID]
		items = append(items, providerSummary{
			ID:              p.ID,
			CompanyName:     p.CompanyName,
			State:           p.State,
			VATNumber:       p.VATNumber,
			CF:              p.CF,
			ERPID:           p.ERPID,
			QualifiedCount:  e.Qualified,
			TotalCount:      e.Total,
			HasExpiringDocs: e.HasExpiring,
		})
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleAlyanteSuppliers(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len([]rune(search)) < 3 || h.alyanteDB == nil {
		httputil.JSON(w, http.StatusOK, []alyanteSupplier{})
		return
	}

	rows, err := h.alyanteDB.QueryContext(r.Context(), alyanteSuppliersQuery, "%"+search+"%")
	if err != nil {
		httputil.InternalError(w, r, err, "alyante suppliers query failed", "component", component, "operation", "alyante_suppliers")
		return
	}
	defer rows.Close()

	items := make([]alyanteSupplier, 0)
	for rows.Next() {
		var companyName sql.NullString
		var code sql.NullString
		if err := rows.Scan(&companyName, &code); err != nil {
			httputil.InternalError(w, r, err, "alyante suppliers scan failed", "component", component, "operation", "alyante_suppliers_scan")
			return
		}
		item := alyanteSupplier{
			Code:        strings.TrimSpace(code.String),
			CompanyName: strings.TrimSpace(companyName.String),
		}
		if item.Code == "" {
			continue
		}
		items = append(items, item)
	}
	writeRows(w, r, items, rows.Err())
}

func (h *Handler) handlePaymentMethods(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT code, description, COALESCE(rda_available, false)
		FROM provider_qualifications.payment_method
		ORDER BY description ASC`)
	if err != nil {
		httputil.InternalError(w, r, err, "payment methods query failed")
		return
	}
	defer rows.Close()

	items := make([]paymentMethod, 0)
	for rows.Next() {
		var item paymentMethod
		if err := rows.Scan(&item.Code, &item.Description, &item.RDAAvailable); err != nil {
			httputil.InternalError(w, r, err, "payment methods scan failed")
			return
		}
		items = append(items, item)
	}
	writeRows(w, r, items, rows.Err())
}

func (h *Handler) handlePaymentMethodAvailability(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body struct {
		RDAAvailable bool `json:"rda_available"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
		return
	}
	result, err := h.db.ExecContext(r.Context(), `
		UPDATE provider_qualifications.payment_method
		SET rda_available = $1
		WHERE code = $2`, body.RDAAvailable, r.PathValue("code"))
	if err != nil {
		httputil.InternalError(w, r, err, "payment method update failed")
		return
	}
	if !rowChanged(result) {
		httputil.Error(w, http.StatusNotFound, "Metodo di pagamento non trovato")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "Aggiornamento completato"})
}

func (h *Handler) handleCountries(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT code, name
		FROM provider_qualifications.country
		ORDER BY name ASC`)
	if err != nil {
		httputil.InternalError(w, r, err, "countries query failed")
		return
	}
	defer rows.Close()

	items := make([]country, 0)
	for rows.Next() {
		var item country
		if err := rows.Scan(&item.Code, &item.Name); err != nil {
			httputil.InternalError(w, r, err, "countries scan failed")
			return
		}
		items = append(items, item)
	}
	writeRows(w, r, items, rows.Err())
}

func (h *Handler) handleArticleCategories(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT ac.article_code, a.description, ac.category_id, sc.name
		FROM articles.article_category ac
		INNER JOIN articles.article a ON a.code = ac.article_code
		INNER JOIN provider_qualifications.service_category sc ON sc.id = ac.category_id
		ORDER BY ac.article_code ASC`)
	if err != nil {
		httputil.InternalError(w, r, err, "article categories query failed")
		return
	}
	defer rows.Close()

	items := make([]articleCategory, 0)
	for rows.Next() {
		var item articleCategory
		if err := rows.Scan(&item.ArticleCode, &item.Description, &item.CategoryID, &item.CategoryName); err != nil {
			httputil.InternalError(w, r, err, "article categories scan failed")
			return
		}
		items = append(items, item)
	}
	writeRows(w, r, items, rows.Err())
}

func (h *Handler) handleArticleCategoryUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body struct {
		CategoryID int64 `json:"category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CategoryID == 0 {
		httputil.Error(w, http.StatusBadRequest, "Seleziona una categoria")
		return
	}
	result, err := h.db.ExecContext(r.Context(), `
		UPDATE articles.article_category
		SET category_id = $1, updated_at = now()
		WHERE article_code = $2`, body.CategoryID, r.PathValue("article_code"))
	if err != nil {
		httputil.InternalError(w, r, err, "article category update failed")
		return
	}
	if !rowChanged(result) {
		httputil.Error(w, http.StatusNotFound, "Articolo non trovato")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "Aggiornamento completato"})
}

func writeRows[T any](w http.ResponseWriter, r *http.Request, items []T, err error) {
	if err != nil {
		httputil.InternalError(w, r, err, "rows iteration failed")
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func rowChanged(result sql.Result) bool {
	count, err := result.RowsAffected()
	return err != nil || count > 0
}

type nullableString struct {
	sql.NullString
}

func (s nullableString) MarshalJSON() ([]byte, error) {
	if !s.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(s.String)
}

type nullableInt64 struct {
	sql.NullInt64
}

func (n nullableInt64) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return []byte(strconv.FormatInt(n.Int64, 10)), nil
}

type nullableTime struct {
	sql.NullTime
}

func (t nullableTime) MarshalJSON() ([]byte, error) {
	if !t.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(t.Time.Format(time.RFC3339))
}

func (t *nullableTime) Scan(value any) error {
	var raw sql.NullTime
	if err := raw.Scan(value); err == nil {
		t.NullTime = raw
		return nil
	}
	switch v := value.(type) {
	case nil:
		t.Valid = false
		return nil
	case []byte:
		parsed, err := parseDateTime(string(v))
		if err != nil {
			return err
		}
		t.Time, t.Valid = parsed, true
		return nil
	case string:
		parsed, err := parseDateTime(v)
		if err != nil {
			return err
		}
		t.Time, t.Valid = parsed, true
		return nil
	default:
		return fmt.Errorf("unsupported time value %T", value)
	}
}

func parseDateTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("unsupported date format")
}

type dashboardDraft struct {
	ID          int64          `json:"id"`
	CompanyName nullableString `json:"company_name"`
	State       nullableString `json:"state"`
	VATNumber   nullableString `json:"vat_number"`
	CF          nullableString `json:"cf"`
	ERPID       nullableInt64  `json:"erp_id"`
	UpdatedAt   nullableTime   `json:"updated_at"`
}

type dashboardDocument struct {
	ID            int64          `json:"id"`
	ProviderID    int64          `json:"provider_id"`
	CompanyName   nullableString `json:"company_name"`
	FileID        nullableString `json:"file_id"`
	ExpireDate    nullableTime   `json:"expire_date"`
	State         nullableString `json:"state"`
	DocumentType  nullableString `json:"document_type"`
	DaysRemaining int            `json:"days_remaining"`
}

type dashboardCategory struct {
	ProviderID   int64          `json:"provider_id"`
	CompanyName  nullableString `json:"company_name"`
	CategoryID   int64          `json:"category_id"`
	CategoryName nullableString `json:"category_name"`
	State        nullableString `json:"state"`
	Critical     bool           `json:"critical"`
}

type paymentMethod struct {
	Code         string `json:"code"`
	Description  string `json:"description"`
	RDAAvailable bool   `json:"rda_available"`
}

type country struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type articleCategory struct {
	ArticleCode  string         `json:"article_code"`
	Description  nullableString `json:"description"`
	CategoryID   int64          `json:"category_id"`
	CategoryName nullableString `json:"category_name"`
}
