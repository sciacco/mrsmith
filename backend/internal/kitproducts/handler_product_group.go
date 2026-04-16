package kitproducts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const productGroupVocabularySection = "kit_product_group"

type ProductGroup struct {
	Name            string        `json:"name"`
	TranslationUUID string        `json:"translation_uuid"`
	UsageCount      int           `json:"usage_count"`
	Translations    []Translation `json:"translations"`
}

type ProductGroupWriteRequest struct {
	Name               string        `json:"name"`
	Translations       []Translation `json:"translations"`
	ConfirmPropagation bool          `json:"confirm_propagation"`
}

func (h *Handler) handleListProductGroups(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT
  v.name,
  v.translation_uuid,
  (
    SELECT COUNT(DISTINCT kit_id)
    FROM products.kit_product kp
    WHERE kp.group_name = v.name
  ) AS usage_count,
  COALESCE(common.get_translations(v.translation_uuid), '[]'::json)
FROM common.vocabulary v
WHERE v.section = $1
ORDER BY lower(v.name), v.name
`, productGroupVocabularySection)
	if err != nil {
		h.dbFailure(w, r, "list_product_groups", err)
		return
	}
	defer rows.Close()

	groups := make([]ProductGroup, 0)
	for rows.Next() {
		group, err := scanProductGroup(rows)
		if err != nil {
			h.dbFailure(w, r, "list_product_groups", err)
			return
		}
		groups = append(groups, group)
	}
	if !h.rowsDone(w, r, rows, "list_product_groups") {
		return
	}

	httputil.JSON(w, http.StatusOK, groups)
}

func (h *Handler) handleCreateProductGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	var req ProductGroupWriteRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httputil.Error(w, http.StatusBadRequest, "name is required")
		return
	}

	languages, err := h.listLanguageOptions(r.Context())
	if err != nil {
		h.dbFailure(w, r, "create_product_group_languages", err)
		return
	}
	translations, err := normalizeProductGroupTranslations(req.Translations, req.Name, languages)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	exists, err := h.productGroupNameExists(r.Context(), req.Name, nil)
	if err != nil {
		h.dbFailure(w, r, "create_product_group_duplicate_check", err, "name", req.Name)
		return
	}
	if exists {
		httputil.Error(w, http.StatusConflict, "duplicate_name")
		return
	}

	tx, err := h.mistraDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "create_product_group_begin", err, "name", req.Name)
		return
	}
	defer h.rollbackTx(r, tx, "create_product_group", "name", req.Name)

	var translationUUID uuid.UUID
	err = tx.QueryRowContext(r.Context(), `
INSERT INTO common.vocabulary (section, name)
VALUES ($1, $2)
RETURNING translation_uuid
`, productGroupVocabularySection, req.Name).Scan(&translationUUID)
	if err != nil {
		h.dbFailure(w, r, "create_product_group_insert", err, "name", req.Name)
		return
	}

	if err := upsertProductGroupTranslations(r.Context(), tx, translationUUID, translations); err != nil {
		h.dbFailure(w, r, "create_product_group_translations", err, "name", req.Name)
		return
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "create_product_group_commit", err, "name", req.Name)
		return
	}

	group, err := h.getProductGroupByTranslationUUID(r.Context(), translationUUID)
	if err != nil {
		h.dbFailure(w, r, "create_product_group_fetch", err, "translation_uuid", translationUUID.String())
		return
	}

	httputil.JSON(w, http.StatusCreated, group)
}

func (h *Handler) handleUpdateProductGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	translationUUID, err := uuid.Parse(strings.TrimSpace(r.PathValue("translationUUID")))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid translation_uuid")
		return
	}

	var req ProductGroupWriteRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httputil.Error(w, http.StatusBadRequest, "name is required")
		return
	}

	languages, err := h.listLanguageOptions(r.Context())
	if err != nil {
		h.dbFailure(w, r, "update_product_group_languages", err, "translation_uuid", translationUUID.String())
		return
	}
	translations, err := normalizeProductGroupTranslations(req.Translations, req.Name, languages)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var currentName string
	err = h.mistraDB.QueryRowContext(r.Context(), `
SELECT name
FROM common.vocabulary
WHERE section = $1 AND translation_uuid = $2
`, productGroupVocabularySection, translationUUID).Scan(&currentName)
	if h.rowError(w, r, "update_product_group_lookup", err, "translation_uuid", translationUUID.String()) {
		return
	}

	exists, err := h.productGroupNameExists(r.Context(), req.Name, &translationUUID)
	if err != nil {
		h.dbFailure(w, r, "update_product_group_duplicate_check", err, "translation_uuid", translationUUID.String(), "name", req.Name)
		return
	}
	if exists {
		httputil.Error(w, http.StatusConflict, "duplicate_name")
		return
	}

	nameChanged := currentName != req.Name
	if nameChanged && !req.ConfirmPropagation {
		usageCount, err := h.countProductGroupUsageByName(r.Context(), currentName)
		if err != nil {
			h.dbFailure(w, r, "update_product_group_usage_count", err, "translation_uuid", translationUUID.String(), "name", currentName)
			return
		}
		httputil.JSON(w, http.StatusConflict, map[string]any{
			"error":                 "rename_confirmation_required",
			"impacted_kit_products": usageCount,
			"quotes_unchanged":      true,
		})
		return
	}

	tx, err := h.mistraDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "update_product_group_begin", err, "translation_uuid", translationUUID.String())
		return
	}
	defer h.rollbackTx(r, tx, "update_product_group", "translation_uuid", translationUUID.String())

	result, err := tx.ExecContext(r.Context(), `
UPDATE common.vocabulary
SET name = $1
WHERE section = $2 AND translation_uuid = $3
`, req.Name, productGroupVocabularySection, translationUUID)
	if err != nil {
		h.dbFailure(w, r, "update_product_group_name", err, "translation_uuid", translationUUID.String())
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		h.dbFailure(w, r, "update_product_group_rows_affected", err, "translation_uuid", translationUUID.String())
		return
	}
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	if err := upsertProductGroupTranslations(r.Context(), tx, translationUUID, translations); err != nil {
		h.dbFailure(w, r, "update_product_group_translations", err, "translation_uuid", translationUUID.String())
		return
	}

	if nameChanged {
		if _, err := tx.ExecContext(r.Context(), `
UPDATE products.kit_product
SET group_name = $1
WHERE group_name = $2
`, req.Name, currentName); err != nil {
			h.dbFailure(w, r, "update_product_group_propagate", err, "translation_uuid", translationUUID.String())
			return
		}
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "update_product_group_commit", err, "translation_uuid", translationUUID.String())
		return
	}

	group, err := h.getProductGroupByTranslationUUID(r.Context(), translationUUID)
	if err != nil {
		h.dbFailure(w, r, "update_product_group_fetch", err, "translation_uuid", translationUUID.String())
		return
	}

	httputil.JSON(w, http.StatusOK, group)
}

func (h *Handler) listLanguageOptions(ctx context.Context) ([]LanguageOption, error) {
	rows, err := h.mistraDB.QueryContext(ctx, `
SELECT iso, name
FROM common.language
ORDER BY iso
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	languages := make([]LanguageOption, 0)
	for rows.Next() {
		var language LanguageOption
		if err := rows.Scan(&language.ISO, &language.Name); err != nil {
			return nil, err
		}
		languages = append(languages, language)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(languages) == 0 {
		return nil, errors.New("no languages configured")
	}

	return languages, nil
}

func (h *Handler) productGroupNameExists(ctx context.Context, name string, excludeUUID *uuid.UUID) (bool, error) {
	var (
		exists bool
		err    error
	)
	if excludeUUID == nil {
		err = h.mistraDB.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM common.vocabulary
  WHERE section = $1 AND lower(name) = lower($2)
)
`, productGroupVocabularySection, name).Scan(&exists)
		return exists, err
	}

	err = h.mistraDB.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM common.vocabulary
  WHERE section = $1
    AND lower(name) = lower($2)
    AND translation_uuid <> $3
)
`, productGroupVocabularySection, name, *excludeUUID).Scan(&exists)
	return exists, err
}

func (h *Handler) countProductGroupUsageByName(ctx context.Context, name string) (int, error) {
	var count int
	err := h.mistraDB.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM products.kit_product
WHERE group_name = $1
`, name).Scan(&count)
	return count, err
}

func (h *Handler) getProductGroupByTranslationUUID(ctx context.Context, translationUUID uuid.UUID) (ProductGroup, error) {
	row := h.mistraDB.QueryRowContext(ctx, `
SELECT
  v.name,
  v.translation_uuid,
  (
    SELECT COUNT(*)
    FROM products.kit_product kp
    WHERE kp.group_name = v.name
  ) AS usage_count,
  COALESCE(common.get_translations(v.translation_uuid), '[]'::json)
FROM common.vocabulary v
WHERE v.section = $1 AND v.translation_uuid = $2
`, productGroupVocabularySection, translationUUID)
	return scanProductGroup(row)
}

func scanProductGroup(scanner interface{ Scan(dest ...any) error }) (ProductGroup, error) {
	var (
		group           ProductGroup
		translationUUID uuid.UUID
		usageCount      int64
		rawTranslations []byte
	)

	if err := scanner.Scan(
		&group.Name,
		&translationUUID,
		&usageCount,
		&rawTranslations,
	); err != nil {
		return ProductGroup{}, err
	}

	group.TranslationUUID = translationUUID.String()
	group.UsageCount = int(usageCount)
	if len(rawTranslations) > 0 {
		if err := json.Unmarshal(rawTranslations, &group.Translations); err != nil {
			return ProductGroup{}, err
		}
	}

	return group, nil
}

func normalizeProductGroupTranslations(input []Translation, name string, languages []LanguageOption) ([]Translation, error) {
	knownLanguages := make(map[string]string, len(languages))
	for _, language := range languages {
		knownLanguages[strings.ToLower(strings.TrimSpace(language.ISO))] = language.ISO
	}

	normalizedInput := make(map[string]Translation, len(input))
	for _, translation := range input {
		code := strings.ToLower(strings.TrimSpace(translation.Language))
		canonical, ok := knownLanguages[code]
		if !ok {
			return nil, errors.New("invalid language")
		}
		normalizedInput[canonical] = Translation{
			Language: canonical,
			Short:    strings.TrimSpace(translation.Short),
			Long:     strings.TrimSpace(translation.Long),
		}
	}

	normalized := make([]Translation, 0, len(languages))
	for _, language := range languages {
		translation := normalizedInput[language.ISO]
		short := strings.TrimSpace(translation.Short)
		if short == "" {
			short = name
		}
		if short == "" {
			return nil, errors.New("short is required for every language")
		}
		normalized = append(normalized, Translation{
			Language: language.ISO,
			Short:    short,
			Long:     strings.TrimSpace(translation.Long),
		})
	}

	return normalized, nil
}

func upsertProductGroupTranslations(ctx context.Context, tx *sql.Tx, translationUUID uuid.UUID, translations []Translation) error {
	for _, translation := range translations {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO common.translation (translation_uuid, language, short, long)
VALUES ($1, $2, $3, $4)
ON CONFLICT (translation_uuid, language)
DO UPDATE SET
  short = EXCLUDED.short,
  long = EXCLUDED.long
`, translationUUID, translation.Language, translation.Short, translation.Long); err != nil {
			return err
		}
	}
	return nil
}
