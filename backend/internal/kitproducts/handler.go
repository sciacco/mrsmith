package kitproducts

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

type Handler struct {
	mistraDB *sql.DB
	alyante  *AlyanteAdapter
	arak     arakDoer
}

type arakDoer interface {
	Do(method, path, queryString string, body io.Reader) (*http.Response, error)
}

func RegisterRoutes(mux *http.ServeMux, mistraDB *sql.DB, alyante *AlyanteAdapter, arakCli *arak.Client) {
	h := &Handler{
		mistraDB: mistraDB,
		alyante:  alyante,
		arak:     arakCli,
	}
	protect := acl.RequireRole(applaunch.KitProductsAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /kit-products/v1/lookup/asset-flow", h.handleListAssetFlows)
	handle("GET /kit-products/v1/lookup/custom-field-key", h.handleListCustomFieldKeys)
	handle("GET /kit-products/v1/lookup/vocabulary", h.handleListVocabulary)

	handle("GET /kit-products/v1/category", h.handleListCategories)
	handle("POST /kit-products/v1/category", h.handleCreateCategory)
	handle("PUT /kit-products/v1/category/{id}", h.handleUpdateCategory)

	handle("GET /kit-products/v1/customer-group", h.handleListCustomerGroups)
	handle("POST /kit-products/v1/customer-group", h.handleCreateCustomerGroup)
	handle("PATCH /kit-products/v1/customer-group", h.handleBatchUpdateCustomerGroups)

	handle("GET /kit-products/v1/product", h.handleListProducts)
	handle("POST /kit-products/v1/product", h.handleCreateProduct)
	handle("PUT /kit-products/v1/product/{code}", h.handleUpdateProduct)
	handle("PUT /kit-products/v1/product/{code}/translations", h.handleUpdateProductTranslations)

	handle("GET /kit-products/v1/kit", h.handleListKits)
	handle("POST /kit-products/v1/kit", h.handleCreateKit)
	handle("DELETE /kit-products/v1/kit/{id}", h.handleDeleteKit)
	handle("POST /kit-products/v1/kit/{id}/clone", h.handleCloneKit)
	handle("GET /kit-products/v1/kit/{id}", h.handleGetKit)
	handle("PUT /kit-products/v1/kit/{id}", h.handleUpdateKit)
	handle("PUT /kit-products/v1/kit/{id}/help", h.handleUpdateKitHelp)
	handle("PUT /kit-products/v1/kit/{id}/translations", h.handleUpdateKitTranslations)

	handle("GET /kit-products/v1/kit/{id}/products", h.handleListKitProducts)
	handle("POST /kit-products/v1/kit/{id}/products", h.handleCreateKitProduct)
	handle("PATCH /kit-products/v1/kit/{id}/products", h.handleBatchUpdateKitProducts)
	handle("PUT /kit-products/v1/kit/{id}/products/{pid}", h.handleUpdateKitProduct)
	handle("DELETE /kit-products/v1/kit/{id}/products/{pid}", h.handleDeleteKitProduct)

	handle("GET /kit-products/v1/kit/{id}/custom-values", h.handleListKitCustomValues)
	handle("POST /kit-products/v1/kit/{id}/custom-values", h.handleCreateKitCustomValue)
	handle("PUT /kit-products/v1/kit/{id}/custom-values/{cvid}", h.handleUpdateKitCustomValue)
	handle("DELETE /kit-products/v1/kit/{id}/custom-values/{cvid}", h.handleDeleteKitCustomValue)

	handle("GET /kit-products/v1/mistra/kit", h.handleProxyMistraKit)
	handle("GET /kit-products/v1/mistra/kit-discount", h.handleProxyMistraKitDiscount)
	handle("POST /kit-products/v1/mistra/kit-discount", h.handleProxyMistraKitDiscount)
	handle("GET /kit-products/v1/mistra/discounted-kit", h.handleProxyMistraDiscountedKit)
	handle("GET /kit-products/v1/mistra/discounted-kit/{id}", h.handleProxyMistraDiscountedKitByID)
	handle("GET /kit-products/v1/mistra/customer", h.handleProxyMistraCustomer)
}

func (h *Handler) requireDB(w http.ResponseWriter) bool {
	if h.mistraDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "database not configured")
		return false
	}
	return true
}

func pathID(r *http.Request, name string) (int, error) {
	return strconv.Atoi(r.PathValue(name))
}

func decodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", "kitproducts", "operation", operation}
	args = append(args, attrs...)
	httputil.InternalError(w, r, err, "database operation failed", args...)
}

func (h *Handler) rowError(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return true
	}
	h.dbFailure(w, r, operation, err, attrs...)
	return true
}

func (h *Handler) rowsDone(w http.ResponseWriter, r *http.Request, rows *sql.Rows, operation string, attrs ...any) bool {
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, operation, err, attrs...)
		return false
	}
	return true
}

func (h *Handler) rollbackTx(r *http.Request, tx *sql.Tx, operation string, attrs ...any) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		args := []any{"component", "kitproducts", "operation", operation, "error", err}
		args = append(args, attrs...)
		logging.FromContext(r.Context()).Warn("transaction rollback failed", args...)
	}
}
