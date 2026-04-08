package kitproducts

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type KitProduct struct {
	ID          int64   `json:"id"`
	KitID       int64   `json:"kit_id"`
	ProductCode string  `json:"product_code"`
	Name        string  `json:"name"`
	Minimum     int     `json:"minimum"`
	Maximum     int     `json:"maximum"`
	Required    bool    `json:"required"`
	NRC         float64 `json:"nrc"`
	MRC         float64 `json:"mrc"`
	Position    int     `json:"position"`
	GroupName   string  `json:"group_name"`
	Notes       string  `json:"notes"`
	ImageURL    string  `json:"image_url,omitempty"`
}

type KitProductRequest struct {
	ProductCode string  `json:"product_code"`
	Minimum     int     `json:"minimum"`
	Maximum     int     `json:"maximum"`
	Required    bool    `json:"required"`
	NRC         float64 `json:"nrc"`
	MRC         float64 `json:"mrc"`
	Position    int     `json:"position"`
	GroupName   string  `json:"group_name"`
	Notes       string  `json:"notes"`
}

type KitProductUpdateItem struct {
	ID int64 `json:"id"`
	KitProductRequest
}

type KitProductBatchUpdateRequest struct {
	Items []KitProductUpdateItem `json:"items"`
}

func (h *Handler) handleListKitProducts(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	kitID, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}
	if ok, err := h.kitExists(r, kitID); err != nil {
		h.dbFailure(w, r, "list_kit_products_lookup", err, "kit_id", kitID)
		return
	} else if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT
  kp.id,
  kp.kit_id,
  kp.product_code,
  p.internal_name,
  kp.minimum,
  kp.maximum,
  kp.required,
  kp.nrc,
  kp.mrc,
  kp.position,
  COALESCE(NULLIF(kp.group_name, ''), p.internal_name),
  COALESCE(kp.notes, ''),
  COALESCE(p.img_url, '')
FROM products.kit_product kp
JOIN products.product p ON p.code = kp.product_code
WHERE kp.kit_id = $1
ORDER BY kp.position, COALESCE(NULLIF(kp.group_name, ''), p.internal_name), p.internal_name
`, kitID)
	if err != nil {
		h.dbFailure(w, r, "list_kit_products", err, "kit_id", kitID)
		return
	}
	defer rows.Close()

	products := make([]KitProduct, 0)
	for rows.Next() {
		product, err := scanKitProduct(rows)
		if err != nil {
			h.dbFailure(w, r, "list_kit_products", err, "kit_id", kitID)
			return
		}
		products = append(products, product)
	}
	if !h.rowsDone(w, r, rows, "list_kit_products", "kit_id", kitID) {
		return
	}

	httputil.JSON(w, http.StatusOK, products)
}

func (h *Handler) handleCreateKitProduct(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	kitID, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}
	if ok, err := h.kitExists(r, kitID); err != nil {
		h.dbFailure(w, r, "create_kit_product_lookup", err, "kit_id", kitID)
		return
	} else if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	var req KitProductRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ProductCode = strings.TrimSpace(req.ProductCode)
	req.GroupName = strings.TrimSpace(req.GroupName)
	req.Notes = strings.TrimSpace(req.Notes)
	if req.ProductCode == "" {
		httputil.Error(w, http.StatusBadRequest, "product_code is required")
		return
	}

	payload := kitProductPayload(kitID, req)
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		h.dbFailure(w, r, "create_kit_product_marshal", err, "kit_id", kitID)
		return
	}

	var createdID int64
	err = h.mistraDB.QueryRowContext(r.Context(), `
SELECT products.new_kit_product($1::json)
`, string(rawPayload)).Scan(&createdID)
	if err != nil {
		h.dbFailure(w, r, "create_kit_product", err, "kit_id", kitID)
		return
	}
	if createdID <= 0 {
		h.dbFailure(w, r, "create_kit_product_result", errors.New("kit product creation returned invalid id"), "kit_id", kitID)
		return
	}

	product, err := h.getKitProductByID(r, kitID, createdID)
	if h.rowError(w, r, "create_kit_product_fetch", err, "kit_id", kitID, "kit_product_id", createdID) {
		return
	}
	httputil.JSON(w, http.StatusCreated, product)
}

func (h *Handler) handleUpdateKitProduct(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	kitID, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}
	productID, err := pathID64(r, "pid")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit product id")
		return
	}
	if ok, err := h.kitExists(r, kitID); err != nil {
		h.dbFailure(w, r, "update_kit_product_lookup", err, "kit_id", kitID, "kit_product_id", productID)
		return
	} else if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	var req KitProductRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ProductCode = strings.TrimSpace(req.ProductCode)
	req.GroupName = strings.TrimSpace(req.GroupName)
	req.Notes = strings.TrimSpace(req.Notes)
	if req.ProductCode == "" {
		httputil.Error(w, http.StatusBadRequest, "product_code is required")
		return
	}

	tx, err := h.mistraDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "update_kit_product_begin", err, "kit_id", kitID, "kit_product_id", productID)
		return
	}
	defer h.rollbackTx(r, tx, "update_kit_product", "kit_id", kitID, "kit_product_id", productID)

	var owningKitID int64
	err = tx.QueryRowContext(r.Context(), `
SELECT kit_id
FROM products.kit_product
WHERE id = $1
`, productID).Scan(&owningKitID)
	if h.rowError(w, r, "update_kit_product_ownership", err, "kit_id", kitID, "kit_product_id", productID) {
		return
	}
	if owningKitID != kitID {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	payload := kitProductPayload(kitID, req)
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		h.dbFailure(w, r, "update_kit_product_marshal", err, "kit_id", kitID, "kit_product_id", productID)
		return
	}

	var updated bool
	err = tx.QueryRowContext(r.Context(), `
SELECT products.upd_kit_product($1, $2::json)
`, productID, string(rawPayload)).Scan(&updated)
	if err != nil {
		h.dbFailure(w, r, "update_kit_product", err, "kit_id", kitID, "kit_product_id", productID)
		return
	}
	if !updated {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "update_kit_product_commit", err, "kit_id", kitID, "kit_product_id", productID)
		return
	}

	product, err := h.getKitProductByID(r, kitID, productID)
	if h.rowError(w, r, "update_kit_product_fetch", err, "kit_id", kitID, "kit_product_id", productID) {
		return
	}
	httputil.JSON(w, http.StatusOK, product)
}

func (h *Handler) handleBatchUpdateKitProducts(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	kitID, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}
	if ok, err := h.kitExists(r, kitID); err != nil {
		h.dbFailure(w, r, "batch_update_kit_products_lookup", err, "kit_id", kitID)
		return
	} else if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	var req KitProductBatchUpdateRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Items) == 0 {
		httputil.Error(w, http.StatusBadRequest, "items are required")
		return
	}

	tx, err := h.mistraDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "batch_update_kit_products_begin", err, "kit_id", kitID)
		return
	}
	defer h.rollbackTx(r, tx, "batch_update_kit_products", "kit_id", kitID)

	for _, item := range req.Items {
		item.ProductCode = strings.TrimSpace(item.ProductCode)
		item.GroupName = strings.TrimSpace(item.GroupName)
		item.Notes = strings.TrimSpace(item.Notes)
		if item.ProductCode == "" {
			httputil.Error(w, http.StatusBadRequest, "product_code is required")
			return
		}

		var owningKitID int64
		err := tx.QueryRowContext(r.Context(), `
SELECT kit_id
FROM products.kit_product
WHERE id = $1
`, item.ID).Scan(&owningKitID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				httputil.Error(w, http.StatusNotFound, "not_found")
				return
			}
			h.dbFailure(w, r, "batch_update_kit_products_lookup", err, "kit_id", kitID, "kit_product_id", item.ID)
			return
		}
		if owningKitID != kitID {
			httputil.Error(w, http.StatusNotFound, "not_found")
			return
		}

		payload := kitProductPayload(kitID, item.KitProductRequest)
		rawPayload, err := json.Marshal(payload)
		if err != nil {
			h.dbFailure(w, r, "batch_update_kit_products_marshal", err, "kit_id", kitID, "kit_product_id", item.ID)
			return
		}

		var updated bool
		err = tx.QueryRowContext(r.Context(), `
SELECT products.upd_kit_product($1, $2::json)
`, item.ID, string(rawPayload)).Scan(&updated)
		if err != nil {
			h.dbFailure(w, r, "batch_update_kit_products_exec", err, "kit_id", kitID, "kit_product_id", item.ID)
			return
		}
		if !updated {
			httputil.Error(w, http.StatusNotFound, "not_found")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "batch_update_kit_products_commit", err, "kit_id", kitID)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]int{"updated": len(req.Items)})
}

func (h *Handler) handleDeleteKitProduct(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	kitID, err := pathID64(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit id")
		return
	}
	productID, err := pathID64(r, "pid")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid kit product id")
		return
	}
	if ok, err := h.kitExists(r, kitID); err != nil {
		h.dbFailure(w, r, "delete_kit_product_lookup", err, "kit_id", kitID, "kit_product_id", productID)
		return
	} else if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	result, err := h.mistraDB.ExecContext(r.Context(), `
DELETE FROM products.kit_product
WHERE id = $1 AND kit_id = $2
`, productID, kitID)
	if err != nil {
		h.dbFailure(w, r, "delete_kit_product", err, "kit_id", kitID, "kit_product_id", productID)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		h.dbFailure(w, r, "delete_kit_product_rows_affected", err, "kit_id", kitID, "kit_product_id", productID)
		return
	}
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getKitProductByID(r *http.Request, kitID, productID int64) (KitProduct, error) {
	row := h.mistraDB.QueryRowContext(r.Context(), `
SELECT
  kp.id,
  kp.kit_id,
  kp.product_code,
  p.internal_name,
  kp.minimum,
  kp.maximum,
  kp.required,
  kp.nrc,
  kp.mrc,
  kp.position,
  COALESCE(NULLIF(kp.group_name, ''), p.internal_name),
  COALESCE(kp.notes, ''),
  COALESCE(p.img_url, '')
FROM products.kit_product kp
JOIN products.product p ON p.code = kp.product_code
WHERE kp.id = $1 AND kp.kit_id = $2
`, productID, kitID)
	return scanKitProduct(row)
}

func scanKitProduct(scanner interface{ Scan(dest ...any) error }) (KitProduct, error) {
	var product KitProduct
	if err := scanner.Scan(
		&product.ID,
		&product.KitID,
		&product.ProductCode,
		&product.Name,
		&product.Minimum,
		&product.Maximum,
		&product.Required,
		&product.NRC,
		&product.MRC,
		&product.Position,
		&product.GroupName,
		&product.Notes,
		&product.ImageURL,
	); err != nil {
		return KitProduct{}, err
	}
	return product, nil
}

func kitProductPayload(kitID int64, req KitProductRequest) map[string]any {
	return map[string]any{
		"kit_id":       kitID,
		"product_code": req.ProductCode,
		"minimum":      req.Minimum,
		"maximum":      req.Maximum,
		"required":     req.Required,
		"nrc":          req.NRC,
		"mrc":          req.MRC,
		"position":     req.Position,
		"group_name":   req.GroupName,
		"notes":        req.Notes,
	}
}
