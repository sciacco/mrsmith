package rda

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var errInvalidArticleType = errors.New("invalid article type")

type article struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type articleCatalogResponse struct {
	TotalNumber int       `json:"total_number"`
	CurrentPage int       `json:"current_page"`
	TotalPages  int       `json:"total_pages"`
	Items       []article `json:"items"`
}

func (h *Handler) fetchNormalizedArticles(r *http.Request) ([]article, error) {
	values, articleType, err := articleQueryValues(r.URL.Query())
	if err != nil {
		return nil, err
	}
	if articleType != "" {
		return h.fetchArticleType(articleType, values)
	}

	merged := make([]article, 0)
	seen := make(map[string]struct{})
	for _, nextType := range []string{"good", "service"} {
		values.Set("type", nextType)
		items, err := h.fetchArticleType(nextType, values)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			key := strings.TrimSpace(item.Code)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, item)
		}
	}
	return merged, nil
}

func articleQueryValues(input url.Values) (url.Values, string, error) {
	values := make(url.Values, len(input)+3)
	for key, vals := range input {
		values[key] = append([]string(nil), vals...)
	}
	if search := strings.TrimSpace(values.Get("search")); search != "" && values.Get("search_string") == "" {
		values.Set("search_string", search)
	}
	values.Del("search")
	if values.Get("page_number") == "" {
		values.Set("page_number", "1")
	}
	if values.Get("disable_pagination") == "" {
		values.Set("disable_pagination", "true")
	}

	articleType := strings.TrimSpace(values.Get("type"))
	if articleType != "" && articleType != "good" && articleType != "service" {
		return nil, "", errInvalidArticleType
	}
	return values, articleType, nil
}

func (h *Handler) fetchArticleType(articleType string, values url.Values) ([]article, error) {
	query := make(url.Values, len(values)+1)
	for key, vals := range values {
		query[key] = append([]string(nil), vals...)
	}
	query.Set("type", articleType)
	resp, err := h.arak.Do(http.MethodGet, arakRDARoot+"/article", query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &upstreamStatusError{status: resp.StatusCode, body: body}
	}
	items, err := decodeArticleItems(body, articleType)
	if err != nil {
		return nil, fmt.Errorf("decode article catalog: %w", err)
	}
	return items, nil
}

func decodeArticleItems(body []byte, articleType string) ([]article, error) {
	var envelope struct {
		Items []article `json:"items"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Items != nil {
		return normalizeArticles(envelope.Items, articleType), nil
	}

	var items []article
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	return normalizeArticles(items, articleType), nil
}

func normalizeArticles(input []article, fallbackType string) []article {
	normalized := make([]article, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		itemType := strings.TrimSpace(item.Type)
		if itemType != "good" && itemType != "service" {
			itemType = fallbackType
		}
		if itemType != "good" && itemType != "service" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		normalized = append(normalized, article{
			Code:        code,
			Description: strings.TrimSpace(item.Description),
			Type:        itemType,
		})
	}
	return normalized
}
