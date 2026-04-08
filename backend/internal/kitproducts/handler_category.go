package kitproducts

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type Category struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CategoryWriteRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

var hexColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (h *Handler) handleListCategories(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT id, name, color
FROM products.product_category
ORDER BY name
`)
	if err != nil {
		h.dbFailure(w, r, "list_categories", err)
		return
	}
	defer rows.Close()

	categories := make([]Category, 0)
	for rows.Next() {
		var category Category
		if err := rows.Scan(&category.ID, &category.Name, &category.Color); err != nil {
			h.dbFailure(w, r, "list_categories", err)
			return
		}
		categories = append(categories, category)
	}
	if !h.rowsDone(w, r, rows, "list_categories") {
		return
	}

	httputil.JSON(w, http.StatusOK, categories)
}

func (h *Handler) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	var req CategoryWriteRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Color = normalizeColor(req.Color)
	if req.Name == "" {
		httputil.Error(w, http.StatusBadRequest, "name is required")
		return
	}
	if !isHexColor(req.Color) {
		httputil.Error(w, http.StatusBadRequest, "invalid color")
		return
	}

	var category Category
	err := h.mistraDB.QueryRowContext(r.Context(), `
INSERT INTO products.product_category (name, color)
VALUES ($1, $2)
RETURNING id, name, color
`, req.Name, req.Color).Scan(&category.ID, &category.Name, &category.Color)
	if h.rowError(w, r, "create_category", err) {
		return
	}

	httputil.JSON(w, http.StatusCreated, category)
}

func (h *Handler) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	id, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid category id")
		return
	}

	var req CategoryWriteRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Color = normalizeColor(req.Color)
	if req.Name == "" {
		httputil.Error(w, http.StatusBadRequest, "name is required")
		return
	}
	if !isHexColor(req.Color) {
		httputil.Error(w, http.StatusBadRequest, "invalid color")
		return
	}

	var category Category
	err = h.mistraDB.QueryRowContext(r.Context(), `
UPDATE products.product_category
SET name = $1, color = $2
WHERE id = $3
RETURNING id, name, color
`, req.Name, req.Color, id).Scan(&category.ID, &category.Name, &category.Color)
	if h.rowError(w, r, "update_category", err, "category_id", id) {
		return
	}

	httputil.JSON(w, http.StatusOK, category)
}

func normalizeColor(color string) string {
	trimmed := strings.TrimSpace(color)
	if trimmed == "" {
		return "#231F20"
	}
	return trimmed
}

func isHexColor(color string) bool {
	return hexColorPattern.MatchString(color)
}
