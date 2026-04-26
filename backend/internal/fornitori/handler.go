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
)

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

type Handler struct {
	arak   *arak.Client
	db     *sql.DB
	logger *slog.Logger
}

func RegisterRoutes(mux *http.ServeMux, arakClient *arak.Client, arakDB *sql.DB) {
	h := &Handler{
		arak:   arakClient,
		db:     arakDB,
		logger: slog.Default().With("component", component),
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

	handle("POST /fornitori/v1/provider/{id}/reference", h.proxyArakPath("/provider/{id}/reference", true))
	handle("PUT /fornitori/v1/provider/{id}/reference/{ref_id}", h.proxyArakPath("/provider/{id}/reference/{ref_id}", true))

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

	handle("GET /fornitori/v1/dashboard/drafts", h.handleDashboardDrafts)
	handle("GET /fornitori/v1/dashboard/expiring-documents", h.handleDashboardExpiringDocuments)
	handle("GET /fornitori/v1/dashboard/categories-to-review", h.handleDashboardCategoriesToReview)

	handle("GET /fornitori/v1/payment-method", h.handlePaymentMethods)
	handle("PUT /fornitori/v1/payment-method/{code}/rda-available", h.requireWritable(h.handlePaymentMethodAvailability))

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
		       dt.name, GREATEST(0, (d.expire_date::date - CURRENT_DATE))::int AS days_remaining
		FROM provider_qualifications.document d
		INNER JOIN provider_qualifications.provider p ON p.id = d.provider_id
		LEFT JOIN provider_qualifications.document_type dt ON dt.id = d.document_type_id
		WHERE d.expire_date::date BETWEEN CURRENT_DATE AND CURRENT_DATE + INTERVAL '30 days'
		ORDER BY d.expire_date ASC, p.company_name ASC`)
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

type articleCategory struct {
	ArticleCode  string         `json:"article_code"`
	Description  nullableString `json:"description"`
	CategoryID   int64          `json:"category_id"`
	CategoryName nullableString `json:"category_name"`
}
