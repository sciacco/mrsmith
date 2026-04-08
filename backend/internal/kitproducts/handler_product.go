package kitproducts

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

type Translation struct {
	Language string `json:"language"`
	Short    string `json:"short"`
	Long     string `json:"long"`
}

type Product struct {
	Code            string        `json:"code"`
	InternalName    string        `json:"internal_name"`
	CategoryID      int           `json:"category_id"`
	CategoryName    string        `json:"category_name"`
	CategoryColor   string        `json:"category_color"`
	TranslationUUID string        `json:"translation_uuid"`
	NRC             float64       `json:"nrc"`
	MRC             float64       `json:"mrc"`
	ImgURL          *string       `json:"img_url"`
	ERPSync         bool          `json:"erp_sync"`
	AssetFlow       *string       `json:"asset_flow"`
	Translations    []Translation `json:"translations"`
}

type ProductCreateRequest struct {
	Code         string        `json:"code"`
	InternalName string        `json:"internal_name"`
	CategoryID   int           `json:"category_id"`
	NRC          float64       `json:"nrc"`
	MRC          float64       `json:"mrc"`
	ImgURL       *string       `json:"img_url"`
	ERPSync      *bool         `json:"erp_sync"`
	AssetFlow    *string       `json:"asset_flow"`
	Translations []Translation `json:"translations"`
}

type ProductUpdateRequest struct {
	InternalName string  `json:"internal_name"`
	CategoryID   int     `json:"category_id"`
	NRC          float64 `json:"nrc"`
	MRC          float64 `json:"mrc"`
	ImgURL       *string `json:"img_url"`
	ERPSync      *bool   `json:"erp_sync"`
	AssetFlow    *string `json:"asset_flow"`
}

type TranslationUpdateRequest struct {
	Translations []Translation `json:"translations"`
}

func (h *Handler) handleListProducts(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT
  p.code,
  p.internal_name,
  p.category_id,
  pc.name,
  pc.color,
  p.translation_uuid,
  p.nrc,
  p.mrc,
  p.img_url,
  COALESCE(p.erp_sync, true),
  p.asset_flow,
  COALESCE(common.get_translations(p.translation_uuid), '[]'::json)
FROM products.product p
JOIN products.product_category pc ON pc.id = p.category_id
ORDER BY p.internal_name
`)
	if err != nil {
		h.dbFailure(w, r, "list_products", err)
		return
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			h.dbFailure(w, r, "list_products", err)
			return
		}
		products = append(products, product)
	}
	if !h.rowsDone(w, r, rows, "list_products") {
		return
	}

	httputil.JSON(w, http.StatusOK, products)
}

func (h *Handler) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	var req ProductCreateRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Code = strings.TrimSpace(req.Code)
	req.InternalName = strings.TrimSpace(req.InternalName)
	req.AssetFlow = trimOptionalString(req.AssetFlow)
	req.ImgURL = trimOptionalString(req.ImgURL)
	if req.Code == "" {
		httputil.Error(w, http.StatusBadRequest, "code is required")
		return
	}
	if len(req.Code) > 25 {
		httputil.Error(w, http.StatusBadRequest, "code_too_long")
		return
	}
	if req.InternalName == "" {
		httputil.Error(w, http.StatusBadRequest, "internal_name is required")
		return
	}
	if req.CategoryID <= 0 {
		httputil.Error(w, http.StatusBadRequest, "category_id is required")
		return
	}
	if ok, err := h.categoryExists(r, req.CategoryID); err != nil {
		h.dbFailure(w, r, "create_product_category_lookup", err, "category_id", req.CategoryID)
		return
	} else if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid category_id")
		return
	}

	tx, err := h.mistraDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "create_product_begin", err)
		return
	}
	defer h.rollbackTx(r, tx, "create_product")

	var translationUUID uuid.UUID
	var erpSync bool
	if req.ERPSync == nil {
		erpSync = true
	} else {
		erpSync = *req.ERPSync
	}

	err = tx.QueryRowContext(r.Context(), `
INSERT INTO products.product (
  code,
  internal_name,
  category_id,
  nrc,
  mrc,
  img_url,
  erp_sync,
  asset_flow
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING translation_uuid
`, req.Code, req.InternalName, req.CategoryID, req.NRC, req.MRC, req.ImgURL, erpSync, req.AssetFlow).Scan(&translationUUID)
	if err != nil {
		h.dbFailure(w, r, "create_product_insert", err, "code", req.Code)
		return
	}

	translations := normalizeTranslations(req.Translations)
	for _, translation := range translations {
		if _, err := tx.ExecContext(r.Context(), `
INSERT INTO common.translation (translation_uuid, language, short, long)
VALUES ($1, $2, $3, $4)
`, translationUUID, translation.Language, translation.Short, translation.Long); err != nil {
			h.dbFailure(w, r, "create_product_translation", err, "code", req.Code, "language", translation.Language)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "create_product_commit", err, "code", req.Code)
		return
	}

	product, err := h.getProductByCode(r, req.Code)
	if h.rowError(w, r, "create_product_fetch", err, "code", req.Code) {
		return
	}
	httputil.JSON(w, http.StatusCreated, product)
}

func (h *Handler) handleUpdateProduct(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	code, err := url.PathUnescape(r.PathValue("code"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid product code")
		return
	}

	var req ProductUpdateRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.InternalName = strings.TrimSpace(req.InternalName)
	req.AssetFlow = trimOptionalString(req.AssetFlow)
	req.ImgURL = trimOptionalString(req.ImgURL)
	if req.InternalName == "" {
		httputil.Error(w, http.StatusBadRequest, "internal_name is required")
		return
	}
	if req.CategoryID <= 0 {
		httputil.Error(w, http.StatusBadRequest, "category_id is required")
		return
	}
	if ok, err := h.categoryExists(r, req.CategoryID); err != nil {
		h.dbFailure(w, r, "update_product_category_lookup", err, "category_id", req.CategoryID)
		return
	} else if !ok {
		httputil.Error(w, http.StatusBadRequest, "invalid category_id")
		return
	}

	var erpSync bool
	if req.ERPSync == nil {
		erpSync = true
	} else {
		erpSync = *req.ERPSync
	}

	result, err := h.mistraDB.ExecContext(r.Context(), `
UPDATE products.product
SET internal_name = $1,
    category_id = $2,
    nrc = $3,
    mrc = $4,
    img_url = $5,
    erp_sync = $6,
    asset_flow = $7
WHERE code = $8
`, req.InternalName, req.CategoryID, req.NRC, req.MRC, req.ImgURL, erpSync, req.AssetFlow, code)
	if err != nil {
		h.dbFailure(w, r, "update_product", err, "code", code)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		h.dbFailure(w, r, "update_product_rows_affected", err, "code", code)
		return
	}
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	product, err := h.getProductByCode(r, code)
	if h.rowError(w, r, "update_product_fetch", err, "code", code) {
		return
	}
	httputil.JSON(w, http.StatusOK, product)
}

func (h *Handler) handleUpdateProductTranslations(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	code, err := url.PathUnescape(r.PathValue("code"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid product code")
		return
	}

	var req TranslationUpdateRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	translations, err := normalizeTranslationUpdate(req.Translations)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	tx, err := h.mistraDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "update_product_translations_begin", err, "code", code)
		return
	}
	defer h.rollbackTx(r, tx, "update_product_translations")

	var (
		translationUUID uuid.UUID
		erpSyncEnabled  bool
	)
	err = tx.QueryRowContext(r.Context(), `
SELECT translation_uuid, COALESCE(erp_sync, true)
FROM products.product
WHERE code = $1
`, code).Scan(&translationUUID, &erpSyncEnabled)
	if h.rowError(w, r, "update_product_translations_lookup", err, "code", code) {
		return
	}

	for _, translation := range translations {
		if _, err := tx.ExecContext(r.Context(), `
INSERT INTO common.translation (translation_uuid, language, short, long)
VALUES ($1, $2, $3, $4)
ON CONFLICT (translation_uuid, language)
DO UPDATE SET
  short = EXCLUDED.short,
  long = EXCLUDED.long
`, translationUUID, translation.Language, translation.Short, translation.Long); err != nil {
			h.dbFailure(w, r, "update_product_translations_upsert", err, "code", code, "language", translation.Language)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "update_product_translations_commit", err, "code", code)
		return
	}

	var warning map[string]string
	if h.alyante != nil && erpSyncEnabled {
		for _, translation := range translations {
			if err := h.alyante.SyncTranslation(r.Context(), code, translation.Language, translation.Short); err != nil {
				logging.FromContext(r.Context()).Error(
					"erp translation sync failed",
					"component", "kitproducts",
					"operation", "update_product_translations",
					"code", code,
					"language", translation.Language,
					"error", err,
				)
				warning = map[string]string{
					"code":    "erp_sync_failed",
					"message": "Salvato, ma sincronizzazione ERP fallita",
				}
				break
			}
		}
	}

	product, err := h.getProductByCode(r, code)
	if h.rowError(w, r, "update_product_translations_fetch", err, "code", code) {
		return
	}

	response := map[string]any{"data": product}
	if warning != nil {
		response["warning"] = warning
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *Handler) getProductByCode(r *http.Request, code string) (Product, error) {
	row := h.mistraDB.QueryRowContext(r.Context(), `
SELECT
  p.code,
  p.internal_name,
  p.category_id,
  pc.name,
  pc.color,
  p.translation_uuid,
  p.nrc,
  p.mrc,
  p.img_url,
  COALESCE(p.erp_sync, true),
  p.asset_flow,
  COALESCE(common.get_translations(p.translation_uuid), '[]'::json)
FROM products.product p
JOIN products.product_category pc ON pc.id = p.category_id
WHERE p.code = $1
`, code)
	return scanProduct(row)
}

func scanProduct(scanner interface{ Scan(dest ...any) error }) (Product, error) {
	var (
		product         Product
		translationUUID uuid.UUID
		imgURL          sql.NullString
		assetFlow       sql.NullString
		rawTranslations []byte
	)
	if err := scanner.Scan(
		&product.Code,
		&product.InternalName,
		&product.CategoryID,
		&product.CategoryName,
		&product.CategoryColor,
		&translationUUID,
		&product.NRC,
		&product.MRC,
		&imgURL,
		&product.ERPSync,
		&assetFlow,
		&rawTranslations,
	); err != nil {
		return Product{}, err
	}
	product.TranslationUUID = translationUUID.String()
	product.ImgURL = nullString(imgURL)
	product.AssetFlow = nullString(assetFlow)
	if len(rawTranslations) > 0 {
		if err := json.Unmarshal(rawTranslations, &product.Translations); err != nil {
			return Product{}, err
		}
	}
	return product, nil
}

func normalizeTranslations(input []Translation) []Translation {
	defaults := map[string]Translation{
		"it": {Language: "it", Short: "", Long: ""},
		"en": {Language: "en", Short: "", Long: ""},
	}
	for _, translation := range input {
		language := strings.ToLower(strings.TrimSpace(translation.Language))
		if language != "it" && language != "en" {
			continue
		}
		defaults[language] = Translation{
			Language: language,
			Short:    strings.TrimSpace(translation.Short),
			Long:     strings.TrimSpace(translation.Long),
		}
	}
	return []Translation{defaults["it"], defaults["en"]}
}

func normalizeTranslationUpdate(input []Translation) ([]Translation, error) {
	translations := normalizeTranslations(input)
	for _, translation := range translations {
		if strings.TrimSpace(translation.Short) == "" {
			return nil, errors.New("short translation is required for it and en")
		}
	}
	return translations, nil
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}
