package kitproducts

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type Kit struct {
	ID                     int64         `json:"id"`
	InternalName           string        `json:"internal_name"`
	MainProductCode        string        `json:"main_product_code"`
	InitialSubscription    int           `json:"initial_subscription_months"`
	NextSubscriptionMonths int           `json:"next_subscription_months"`
	ActivationTimeDays     int           `json:"activation_time_days"`
	NRC                    float64       `json:"nrc"`
	MRC                    float64       `json:"mrc"`
	TranslationUUID        string        `json:"translation_uuid"`
	BundlePrefix           string        `json:"bundle_prefix"`
	Ecommerce              bool          `json:"ecommerce"`
	CategoryID             int           `json:"category_id"`
	CategoryName           string        `json:"category_name"`
	CategoryColor          string        `json:"category_color"`
	IsMainPrdSellable      bool          `json:"is_main_prd_sellable"`
	IsActive               bool          `json:"is_active"`
	Quotable               bool          `json:"-"`
	BillingPeriod          int           `json:"billing_period"`
	ScontoMassimo          float64       `json:"sconto_massimo"`
	VariableBilling        bool          `json:"variable_billing"`
	H24Assurance           bool          `json:"h24_assurance"`
	SLAResolutionHours     int           `json:"sla_resolution_hours"`
	Notes                  string        `json:"notes"`
	Translations           []Translation `json:"translations,omitempty"`
	SellableGroupIDs       []int64       `json:"sellable_group_ids,omitempty"`
	HelpURL                string        `json:"help_url,omitempty"`
}

type KitWriteRequest struct {
	InternalName           string  `json:"internal_name"`
	MainProductCode        string  `json:"main_product_code"`
	InitialSubscription    int     `json:"initial_subscription_months"`
	NextSubscriptionMonths int     `json:"next_subscription_months"`
	ActivationTimeDays     int     `json:"activation_time_days"`
	NRC                    float64 `json:"nrc"`
	MRC                    float64 `json:"mrc"`
	BundlePrefix           string  `json:"bundle_prefix"`
	Ecommerce              bool    `json:"ecommerce"`
	CategoryID             int     `json:"category_id"`
	IsMainPrdSellable      bool    `json:"is_main_prd_sellable"`
	IsActive               bool    `json:"is_active"`
	BillingPeriod          int     `json:"billing_period"`
	ScontoMassimo          float64 `json:"sconto_massimo"`
	VariableBilling        bool    `json:"variable_billing"`
	H24Assurance           bool    `json:"h24_assurance"`
	SLAResolutionHours     int     `json:"sla_resolution_hours"`
	Notes                  string  `json:"notes"`
	SellableGroupIDs       []int64 `json:"sellable_group_ids"`
}

type KitCloneRequest struct {
	Name string `json:"name"`
}

type KitHelpRequest struct {
	HelpURL *string `json:"help_url"`
}

type KitTranslationUpdateRequest struct {
	Translations []Translation `json:"translations"`
}

func (h *Handler) handleListKits(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT
  k.id,
  k.internal_name,
  k.main_product_code,
  k.initial_subscription_months,
  k.next_subscription_months,
  k.activation_time_days,
  k.nrc,
  k.mrc,
  k.translation_uuid,
  COALESCE(k.bundle_prefix, ''),
  COALESCE(k.ecommerce, true),
  k.category_id,
  pc.name,
  pc.color,
  COALESCE(k.is_main_prd_sellable, true),
  k.is_active,
  true,
  k.billing_period,
  k.sconto_massimo,
  k.variable_billing,
  k.h24_assurance,
  k.sla_resolution_hours,
  COALESCE(k.notes, '')
FROM products.kit k
JOIN products.product_category pc ON pc.id = k.category_id
ORDER BY k.is_active::int DESC, k.internal_name
`)
	if err != nil {
		h.dbFailure(w, r, "list_kits", err)
		return
	}
	defer rows.Close()

	kits := make([]Kit, 0)
	for rows.Next() {
		kit, err := scanKit(rows)
		if err != nil {
			h.dbFailure(w, r, "list_kits", err)
			return
		}
		kits = append(kits, kit)
	}
	if !h.rowsDone(w, r, rows, "list_kits") {
		return
	}

	httputil.JSON(w, http.StatusOK, kits)
}

func (h *Handler) handleCreateKit(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	var req KitWriteRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.InternalName = strings.TrimSpace(req.InternalName)
	req.MainProductCode = strings.TrimSpace(req.MainProductCode)
	req.BundlePrefix = strings.TrimSpace(req.BundlePrefix)
	req.Notes = strings.TrimSpace(req.Notes)
	if req.InternalName == "" {
		httputil.Error(w, http.StatusBadRequest, "internal_name is required")
		return
	}
	if req.MainProductCode == "" {
		httputil.Error(w, http.StatusBadRequest, "main_product_code is required")
		return
	}
	if req.BundlePrefix == "" {
		httputil.Error(w, http.StatusBadRequest, "bundle_prefix is required")
		return
	}

	payload := kitCreatePayload(req)
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		h.dbFailure(w, r, "create_kit_marshal", err)
		return
	}

	var kitID int64
	err = h.mistraDB.QueryRowContext(r.Context(), `
SELECT products.new_kit($1::json)
`, string(rawPayload)).Scan(&kitID)
	if err != nil {
		h.dbFailure(w, r, "create_kit", err)
		return
	}
	if kitID <= 0 {
		h.dbFailure(w, r, "create_kit_result", errors.New("kit creation returned invalid id"))
		return
	}

	httputil.JSON(w, http.StatusCreated, map[string]int64{"id": kitID})
}

func (h *Handler) handleDeleteKit(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	id, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}

	result, err := h.mistraDB.ExecContext(r.Context(), `
UPDATE products.kit
SET is_active = false
WHERE id = $1
`, id)
	if err != nil {
		h.dbFailure(w, r, "delete_kit", err, "kit_id", id)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		h.dbFailure(w, r, "delete_kit_rows_affected", err, "kit_id", id)
		return
	}
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleCloneKit(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	id, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}
	if ok, err := h.kitExists(r, id); err != nil {
		h.dbFailure(w, r, "clone_kit_lookup", err, "kit_id", id)
		return
	} else if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	var req KitCloneRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httputil.Error(w, http.StatusBadRequest, "name is required")
		return
	}

	var clonedID int64
	err = h.mistraDB.QueryRowContext(r.Context(), `
SELECT products.clone_kit($1, $2)
`, id, req.Name).Scan(&clonedID)
	if err != nil {
		h.dbFailure(w, r, "clone_kit", err, "kit_id", id)
		return
	}
	if clonedID <= 0 {
		h.dbFailure(w, r, "clone_kit_result", errors.New("kit clone returned invalid id"), "kit_id", id)
		return
	}

	httputil.JSON(w, http.StatusCreated, map[string]int64{"id": clonedID})
}

func (h *Handler) handleGetKit(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	id, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}

	kit, err := h.getKitByID(r, id)
	if h.rowError(w, r, "get_kit", err, "kit_id", id) {
		return
	}
	httputil.JSON(w, http.StatusOK, kit)
}

func (h *Handler) handleUpdateKit(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	id, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}

	var req KitWriteRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.InternalName = strings.TrimSpace(req.InternalName)
	req.MainProductCode = strings.TrimSpace(req.MainProductCode)
	req.Notes = strings.TrimSpace(req.Notes)
	if req.InternalName == "" {
		httputil.Error(w, http.StatusBadRequest, "internal_name is required")
		return
	}
	if req.MainProductCode == "" {
		httputil.Error(w, http.StatusBadRequest, "main_product_code is required")
		return
	}

	currentBundlePrefix, err := h.getKitBundlePrefix(r, id)
	if h.rowError(w, r, "update_kit_lookup", err, "kit_id", id) {
		return
	}
	if req.BundlePrefix != "" && strings.TrimSpace(req.BundlePrefix) != currentBundlePrefix {
		httputil.Error(w, http.StatusBadRequest, "bundle_prefix is immutable")
		return
	}

	payload := kitUpdatePayload(req)
	payload["f_bundle_prefix"] = currentBundlePrefix
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		h.dbFailure(w, r, "update_kit_marshal", err, "kit_id", id)
		return
	}

	tx, err := h.mistraDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "update_kit_begin", err, "kit_id", id)
		return
	}
	defer h.rollbackTx(r, tx, "update_kit", "kit_id", id)

	var updated bool
	err = tx.QueryRowContext(r.Context(), `
SELECT products.upd_kit($1, $2::json)
`, id, string(rawPayload)).Scan(&updated)
	if err != nil {
		h.dbFailure(w, r, "update_kit", err, "kit_id", id)
		return
	}
	if !updated {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "update_kit_commit", err, "kit_id", id)
		return
	}

	kit, err := h.getKitByID(r, id)
	if h.rowError(w, r, "update_kit_fetch", err, "kit_id", id) {
		return
	}
	httputil.JSON(w, http.StatusOK, kit)
}

func (h *Handler) handleUpdateKitHelp(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	id, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}
	if ok, err := h.kitExists(r, id); err != nil {
		h.dbFailure(w, r, "update_kit_help_lookup", err, "kit_id", id)
		return
	} else if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	var req KitHelpRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	helpURL := strings.TrimSpace(nullStringValue(req.HelpURL))
	if helpURL == "" {
		if _, err := h.mistraDB.ExecContext(r.Context(), `
DELETE FROM products.kit_help
WHERE kit_id = $1
`, id); err != nil {
			h.dbFailure(w, r, "delete_kit_help", err, "kit_id", id)
			return
		}

		w.WriteHeader(http.StatusNoContent)
		return
	}

	if _, err := h.mistraDB.ExecContext(r.Context(), `
INSERT INTO products.kit_help (kit_id, help_url)
VALUES ($1, $2)
ON CONFLICT (kit_id) DO UPDATE SET
  help_url = EXCLUDED.help_url,
  updated_at = CURRENT_TIMESTAMP
`, id, helpURL); err != nil {
		h.dbFailure(w, r, "update_kit_help", err, "kit_id", id)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleUpdateKitTranslations(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	id, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}

	var req KitTranslationUpdateRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	translations := normalizeKitTranslations(req.Translations)

	translationUUID, err := h.getKitTranslationUUID(r, id)
	if h.rowError(w, r, "update_kit_translations_lookup", err, "kit_id", id) {
		return
	}

	rawTranslations, err := json.Marshal(translations)
	if err != nil {
		h.dbFailure(w, r, "update_kit_translations_marshal", err, "kit_id", id)
		return
	}

	var updated int64
	err = h.mistraDB.QueryRowContext(r.Context(), `
SELECT common.upd_translation($1, $2::json)
`, translationUUID, string(rawTranslations)).Scan(&updated)
	if err != nil {
		h.dbFailure(w, r, "update_kit_translations", err, "kit_id", id)
		return
	}

	kit, err := h.getKitByID(r, id)
	if h.rowError(w, r, "update_kit_translations_fetch", err, "kit_id", id) {
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"data":    kit,
		"updated": updated,
	})
}

func (h *Handler) kitExists(r *http.Request, id int64) (bool, error) {
	var exists bool
	err := h.mistraDB.QueryRowContext(r.Context(), `
SELECT EXISTS (
  SELECT 1
  FROM products.kit
  WHERE id = $1
)
`, id).Scan(&exists)
	return exists, err
}

func (h *Handler) getKitBundlePrefix(r *http.Request, id int64) (string, error) {
	var bundlePrefix sql.NullString
	err := h.mistraDB.QueryRowContext(r.Context(), `
SELECT bundle_prefix
FROM products.kit
WHERE id = $1
`, id).Scan(&bundlePrefix)
	if err != nil {
		return "", err
	}
	if !bundlePrefix.Valid {
		return "", nil
	}
	return bundlePrefix.String, nil
}

func (h *Handler) getKitTranslationUUID(r *http.Request, id int64) (string, error) {
	var translationUUID string
	err := h.mistraDB.QueryRowContext(r.Context(), `
SELECT translation_uuid
FROM products.kit
WHERE id = $1
`, id).Scan(&translationUUID)
	return translationUUID, err
}

func (h *Handler) getKitByID(r *http.Request, id int64) (Kit, error) {
	row := h.mistraDB.QueryRowContext(r.Context(), `
SELECT
  k.id,
  k.internal_name,
  k.main_product_code,
  k.initial_subscription_months,
  k.next_subscription_months,
  k.activation_time_days,
  k.nrc,
  k.mrc,
  k.translation_uuid,
  COALESCE(k.bundle_prefix, ''),
  COALESCE(k.ecommerce, true),
  k.category_id,
  pc.name,
  pc.color,
  COALESCE(k.is_main_prd_sellable, true),
  k.is_active,
  true,
  k.billing_period,
  k.sconto_massimo,
  k.variable_billing,
  k.h24_assurance,
  k.sla_resolution_hours,
  COALESCE(k.notes, ''),
  COALESCE(common.get_translations(k.translation_uuid), '[]'::json),
  COALESCE((
    SELECT json_agg(kcg.group_id ORDER BY kcg.group_id)
    FROM products.kit_customer_group kcg
    WHERE kcg.kit_id = k.id AND kcg.sellable = true
  ), '[]'::json),
  COALESCE((
    SELECT help_url
    FROM products.kit_help kh
    WHERE kh.kit_id = k.id
  ), '')
FROM products.kit k
JOIN products.product_category pc ON pc.id = k.category_id
WHERE k.id = $1
`, id)
	return scanKitDetail(row)
}

func scanKit(scanner interface{ Scan(dest ...any) error }) (Kit, error) {
	var (
		kit             Kit
		translationUUID string
	)
	if err := scanner.Scan(
		&kit.ID,
		&kit.InternalName,
		&kit.MainProductCode,
		&kit.InitialSubscription,
		&kit.NextSubscriptionMonths,
		&kit.ActivationTimeDays,
		&kit.NRC,
		&kit.MRC,
		&translationUUID,
		&kit.BundlePrefix,
		&kit.Ecommerce,
		&kit.CategoryID,
		&kit.CategoryName,
		&kit.CategoryColor,
		&kit.IsMainPrdSellable,
		&kit.IsActive,
		&kit.Quotable,
		&kit.BillingPeriod,
		&kit.ScontoMassimo,
		&kit.VariableBilling,
		&kit.H24Assurance,
		&kit.SLAResolutionHours,
		&kit.Notes,
	); err != nil {
		return Kit{}, err
	}
	kit.TranslationUUID = translationUUID
	return kit, nil
}

func scanKitDetail(scanner interface{ Scan(dest ...any) error }) (Kit, error) {
	var (
		kit             Kit
		translationUUID string
		rawTranslations []byte
		rawSellableTo   []byte
		helpURL         sql.NullString
	)
	if err := scanner.Scan(
		&kit.ID,
		&kit.InternalName,
		&kit.MainProductCode,
		&kit.InitialSubscription,
		&kit.NextSubscriptionMonths,
		&kit.ActivationTimeDays,
		&kit.NRC,
		&kit.MRC,
		&translationUUID,
		&kit.BundlePrefix,
		&kit.Ecommerce,
		&kit.CategoryID,
		&kit.CategoryName,
		&kit.CategoryColor,
		&kit.IsMainPrdSellable,
		&kit.IsActive,
		&kit.Quotable,
		&kit.BillingPeriod,
		&kit.ScontoMassimo,
		&kit.VariableBilling,
		&kit.H24Assurance,
		&kit.SLAResolutionHours,
		&kit.Notes,
		&rawTranslations,
		&rawSellableTo,
		&helpURL,
	); err != nil {
		return Kit{}, err
	}
	kit.TranslationUUID = translationUUID
	if help := nullString(helpURL); help != nil {
		kit.HelpURL = *help
	}
	if len(rawTranslations) > 0 {
		if err := json.Unmarshal(rawTranslations, &kit.Translations); err != nil {
			return Kit{}, err
		}
	}
	if len(rawSellableTo) > 0 {
		if err := json.Unmarshal(rawSellableTo, &kit.SellableGroupIDs); err != nil {
			return Kit{}, err
		}
	}
	return kit, nil
}

func kitCreatePayload(req KitWriteRequest) map[string]any {
	return map[string]any{
		"f_internal_name":        req.InternalName,
		"s_main_product":         req.MainProductCode,
		"f_initial_subscription": req.InitialSubscription,
		"f_next_period":          req.NextSubscriptionMonths,
		"f_activation_time":      req.ActivationTimeDays,
		"f_nrc":                  req.NRC,
		"f_mrc":                  req.MRC,
		"f_bundle_prefix":        req.BundlePrefix,
		"sw_ecommerce":           req.Ecommerce,
		"s_category":             req.CategoryID,
		"ms_sellable_to":         normalizeKitGroupIDs(req.SellableGroupIDs),
	}
}

func kitUpdatePayload(req KitWriteRequest) map[string]any {
	return map[string]any{
		"f_internal_name":        req.InternalName,
		"s_main_product":         req.MainProductCode,
		"f_initial_subscription": req.InitialSubscription,
		"f_next_period":          req.NextSubscriptionMonths,
		"f_activation_time":      req.ActivationTimeDays,
		"f_nrc":                  req.NRC,
		"f_mrc":                  req.MRC,
		"f_bundle_prefix":        req.BundlePrefix,
		"sw_ecommerce":           req.Ecommerce,
		"s_category":             req.CategoryID,
		"sw_main_sellable":       req.IsMainPrdSellable,
		"sw_active":              req.IsActive,
		"f_sconto_massimo":       req.ScontoMassimo,
		"sl_billing_period":      req.BillingPeriod,
		"sw_variable_billing":    req.VariableBilling,
		"f_sla_resolution_hours": req.SLAResolutionHours,
		"sw_h24_assurance":       req.H24Assurance,
		"f_notes":                req.Notes,
		"ms_sellable_to":         normalizeKitGroupIDs(req.SellableGroupIDs),
	}
}

func normalizeKitGroupIDs(input []int64) []int64 {
	if input == nil {
		return []int64{}
	}
	return input
}

func normalizeKitTranslations(input []Translation) []Translation {
	translations := make([]Translation, 0, len(input))
	for _, translation := range input {
		language := strings.ToLower(strings.TrimSpace(translation.Language))
		if language == "" {
			continue
		}
		translations = append(translations, Translation{
			Language: language,
			Short:    strings.TrimSpace(translation.Short),
			Long:     strings.TrimSpace(translation.Long),
		})
	}
	return translations
}

func pathID64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

func nullStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
