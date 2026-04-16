package kitproducts

import (
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type AssetFlow struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

type CustomFieldKey struct {
	KeyName        string `json:"key_name"`
	KeyDescription string `json:"key_description"`
}

type LanguageOption struct {
	ISO  string `json:"iso"`
	Name string `json:"name"`
}

type VocabularyItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

func (h *Handler) handleListAssetFlows(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT name, label
FROM products.asset_flow
ORDER BY name
`)
	if err != nil {
		h.dbFailure(w, r, "list_asset_flows", err)
		return
	}
	defer rows.Close()

	flows := make([]AssetFlow, 0)
	for rows.Next() {
		var flow AssetFlow
		if err := rows.Scan(&flow.Name, &flow.Label); err != nil {
			h.dbFailure(w, r, "list_asset_flows", err)
			return
		}
		flows = append(flows, flow)
	}
	if !h.rowsDone(w, r, rows, "list_asset_flows") {
		return
	}

	httputil.JSON(w, http.StatusOK, flows)
}

func (h *Handler) handleListCustomFieldKeys(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT key_name, key_description
FROM common.custom_field_key
ORDER BY key_description
`)
	if err != nil {
		h.dbFailure(w, r, "list_custom_field_keys", err)
		return
	}
	defer rows.Close()

	keys := make([]CustomFieldKey, 0)
	for rows.Next() {
		var key CustomFieldKey
		if err := rows.Scan(&key.KeyName, &key.KeyDescription); err != nil {
			h.dbFailure(w, r, "list_custom_field_keys", err)
			return
		}
		keys = append(keys, key)
	}
	if !h.rowsDone(w, r, rows, "list_custom_field_keys") {
		return
	}

	httputil.JSON(w, http.StatusOK, keys)
}

func (h *Handler) handleListLanguages(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT iso, name
FROM common.language
ORDER BY iso
`)
	if err != nil {
		h.dbFailure(w, r, "list_languages", err)
		return
	}
	defer rows.Close()

	languages := make([]LanguageOption, 0)
	for rows.Next() {
		var language LanguageOption
		if err := rows.Scan(&language.ISO, &language.Name); err != nil {
			h.dbFailure(w, r, "list_languages", err)
			return
		}
		languages = append(languages, language)
	}
	if !h.rowsDone(w, r, rows, "list_languages") {
		return
	}

	httputil.JSON(w, http.StatusOK, languages)
}

func (h *Handler) handleListVocabulary(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	section := strings.TrimSpace(r.URL.Query().Get("section"))
	if section == "" {
		httputil.Error(w, http.StatusBadRequest, "section is required")
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT name AS label, name AS value
FROM common.vocabulary
WHERE section = $1
ORDER BY label
`, section)
	if err != nil {
		h.dbFailure(w, r, "list_vocabulary", err, "section", section)
		return
	}
	defer rows.Close()

	items := make([]VocabularyItem, 0)
	for rows.Next() {
		var item VocabularyItem
		if err := rows.Scan(&item.Label, &item.Value); err != nil {
			h.dbFailure(w, r, "list_vocabulary", err, "section", section)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_vocabulary", "section", section) {
		return
	}

	httputil.JSON(w, http.StatusOK, items)
}
