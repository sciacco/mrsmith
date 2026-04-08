package kitproducts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleKitLifecycle(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-list", nil)}
		req := httptest.NewRequest(http.MethodGet, "/kit-products/v1/kit", nil)
		rec := httptest.NewRecorder()

		h.handleListKits(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var kits []Kit
		if err := json.Unmarshal(rec.Body.Bytes(), &kits); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(kits) != 1 || kits[0].InternalName != "Kit One" || kits[0].CategoryName != "Networking" {
			t.Fatalf("unexpected payload: %#v", kits)
		}
	})

	t.Run("create", func(t *testing.T) {
		h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-create", nil)}
		req := httptest.NewRequest(http.MethodPost, "/kit-products/v1/kit", strings.NewReader(`{
			"internal_name":"Kit One",
			"main_product_code":"MAIN001",
			"initial_subscription_months":12,
			"next_subscription_months":12,
			"activation_time_days":30,
			"nrc":99.5,
			"mrc":19.5,
			"bundle_prefix":"KIT",
			"ecommerce":true,
			"category_id":1
		}`))
		rec := httptest.NewRecorder()

		h.handleCreateKit(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
		var body map[string]int64
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if body["id"] != 42 {
			t.Fatalf("expected id=42, got %#v", body)
		}
	})

	t.Run("clone", func(t *testing.T) {
		h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-clone", nil)}
		req := httptest.NewRequest(http.MethodPost, "/kit-products/v1/kit/7/clone", strings.NewReader(`{"name":"Kit Copy"}`))
		req.SetPathValue("id", "7")
		rec := httptest.NewRecorder()

		h.handleCloneKit(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
		var body map[string]int64
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if body["id"] != 43 {
			t.Fatalf("expected id=43, got %#v", body)
		}
	})

	t.Run("clone blank name", func(t *testing.T) {
		h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-clone", nil)}
		req := httptest.NewRequest(http.MethodPost, "/kit-products/v1/kit/7/clone", strings.NewReader(`{"name":"   "}`))
		req.SetPathValue("id", "7")
		rec := httptest.NewRecorder()

		h.handleCloneKit(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("delete", func(t *testing.T) {
		h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-delete", nil)}
		req := httptest.NewRequest(http.MethodDelete, "/kit-products/v1/kit/7", nil)
		req.SetPathValue("id", "7")
		rec := httptest.NewRecorder()

		h.handleDeleteKit(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
	})
}

func TestHandleKitDetailAndMetadataUpdates(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-detail", nil)}
		req := httptest.NewRequest(http.MethodGet, "/kit-products/v1/kit/7", nil)
		req.SetPathValue("id", "7")
		rec := httptest.NewRecorder()

		h.handleGetKit(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var kit Kit
		if err := json.Unmarshal(rec.Body.Bytes(), &kit); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if kit.HelpURL != "https://example.test/help" || len(kit.SellableGroupIDs) != 2 || len(kit.Translations) != 2 {
			t.Fatalf("unexpected kit detail: %#v", kit)
		}
	})

	t.Run("update", func(t *testing.T) {
		h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-update", nil)}
		req := httptest.NewRequest(http.MethodPut, "/kit-products/v1/kit/7", strings.NewReader(`{
			"internal_name":"Kit Updated",
			"main_product_code":"MAIN001",
			"initial_subscription_months":6,
			"next_subscription_months":12,
			"activation_time_days":15,
			"nrc":55.5,
			"mrc":11.25,
			"bundle_prefix":"KIT",
			"ecommerce":false,
			"category_id":1,
			"is_main_prd_sellable":false,
			"is_active":true,
			"billing_period":6,
			"sconto_massimo":10,
			"variable_billing":true,
			"h24_assurance":true,
			"sla_resolution_hours":4,
			"notes":"updated",
			"sellable_group_ids":[1,2]
		}`))
		req.SetPathValue("id", "7")
		rec := httptest.NewRecorder()

		h.handleUpdateKit(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var kit Kit
		if err := json.Unmarshal(rec.Body.Bytes(), &kit); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if kit.InternalName != "Kit One" || kit.BundlePrefix != "KIT" {
			t.Fatalf("unexpected kit response: %#v", kit)
		}
	})

	t.Run("help", func(t *testing.T) {
		h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-help", nil)}
		req := httptest.NewRequest(http.MethodPut, "/kit-products/v1/kit/7/help", strings.NewReader(`{"help_url":"https://example.test/help"}`))
		req.SetPathValue("id", "7")
		rec := httptest.NewRecorder()

		h.handleUpdateKitHelp(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
	})

	t.Run("translations", func(t *testing.T) {
		h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-translations", nil)}
		req := httptest.NewRequest(http.MethodPut, "/kit-products/v1/kit/7/translations", strings.NewReader(`{"translations":[{"language":"it","short":"Kit","long":"Kit IT"},{"language":"en","short":"Kit","long":"Kit EN"}]}`))
		req.SetPathValue("id", "7")
		rec := httptest.NewRecorder()

		h.handleUpdateKitTranslations(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var body struct {
			Updated int64 `json:"updated"`
			Data    Kit   `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if body.Updated != 2 || body.Data.ID != 9 {
			t.Fatalf("unexpected translation update response: %#v", body)
		}
	})
}

func TestHandleUpdateKitHelpAllowsClear(t *testing.T) {
	h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-help-clear", nil)}
	req := httptest.NewRequest(http.MethodPut, "/kit-products/v1/kit/7/help", strings.NewReader(`{"help_url":null}`))
	req.SetPathValue("id", "7")
	rec := httptest.NewRecorder()

	h.handleUpdateKitHelp(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestHandleKitNestedResources(t *testing.T) {
	t.Run("products", func(t *testing.T) {
		h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-product-list", nil)}
		req := httptest.NewRequest(http.MethodGet, "/kit-products/v1/kit/7/products", nil)
		req.SetPathValue("id", "7")
		rec := httptest.NewRecorder()

		h.handleListKitProducts(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var items []KitProduct
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(items) != 2 || items[0].Name != "Router" {
			t.Fatalf("unexpected kit products payload: %#v", items)
		}
	})

	t.Run("product create update delete", func(t *testing.T) {
		h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-product-create", nil)}
		createReq := httptest.NewRequest(http.MethodPost, "/kit-products/v1/kit/7/products", strings.NewReader(`{"product_code":"PRD001","minimum":1,"maximum":3,"required":true,"nrc":10,"mrc":5,"position":1,"group_name":"Access","notes":"notes"}`))
		createReq.SetPathValue("id", "7")
		createRec := httptest.NewRecorder()

		h.handleCreateKitProduct(createRec, createReq)

		if createRec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", createRec.Code)
		}

		updateReq := httptest.NewRequest(http.MethodPut, "/kit-products/v1/kit/7/products/88", strings.NewReader(`{"product_code":"PRD001","minimum":2,"maximum":4,"required":false,"nrc":12,"mrc":6,"position":2,"group_name":"Access","notes":"updated"}`))
		updateReq.SetPathValue("id", "7")
		updateReq.SetPathValue("pid", "88")
		updateRec := httptest.NewRecorder()

		h.handleUpdateKitProduct(updateRec, updateReq)

		if updateRec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", updateRec.Code)
		}

		deleteReq := httptest.NewRequest(http.MethodDelete, "/kit-products/v1/kit/7/products/88", nil)
		deleteReq.SetPathValue("id", "7")
		deleteReq.SetPathValue("pid", "88")
		deleteRec := httptest.NewRecorder()

		h.handleDeleteKitProduct(deleteRec, deleteReq)

		if deleteRec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", deleteRec.Code)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-custom-value", nil)}
		listReq := httptest.NewRequest(http.MethodGet, "/kit-products/v1/kit/7/custom-values", nil)
		listReq.SetPathValue("id", "7")
		listRec := httptest.NewRecorder()

		h.handleListKitCustomValues(listRec, listReq)

		if listRec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", listRec.Code)
		}

		createReq := httptest.NewRequest(http.MethodPost, "/kit-products/v1/kit/7/custom-values", strings.NewReader(`{"key_name":"legal_notes","value":{"it":"nota","en":"note"}}`))
		createReq.SetPathValue("id", "7")
		createRec := httptest.NewRecorder()

		h.handleCreateKitCustomValue(createRec, createReq)

		if createRec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", createRec.Code)
		}

		updateReq := httptest.NewRequest(http.MethodPut, "/kit-products/v1/kit/7/custom-values/31", strings.NewReader(`{"key_name":"legal_notes","value":{"it":"nota 2","en":"note 2"}}`))
		updateReq.SetPathValue("id", "7")
		updateReq.SetPathValue("cvid", "31")
		updateRec := httptest.NewRecorder()

		h.handleUpdateKitCustomValue(updateRec, updateReq)

		if updateRec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", updateRec.Code)
		}

		deleteReq := httptest.NewRequest(http.MethodDelete, "/kit-products/v1/kit/7/custom-values/31", nil)
		deleteReq.SetPathValue("id", "7")
		deleteReq.SetPathValue("cvid", "31")
		deleteRec := httptest.NewRecorder()

		h.handleDeleteKitCustomValue(deleteRec, deleteReq)

		if deleteRec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", deleteRec.Code)
		}
	})
}

func TestHandleBatchUpdateKitProductsRollsBackOnFailure(t *testing.T) {
	tracker := &kitProductsTxTracker{}
	h := &Handler{mistraDB: openKitProductsTestDB(t, "kit-product-update-fail", tracker)}

	req := httptest.NewRequest(http.MethodPatch, "/kit-products/v1/kit/7/products", strings.NewReader(`{"items":[{"id":21,"product_code":"PRD001","minimum":2,"maximum":4,"required":true,"nrc":12,"mrc":6,"position":1,"group_name":"Access","notes":"updated"}]}`))
	req.SetPathValue("id", "7")
	rec := httptest.NewRecorder()

	h.handleBatchUpdateKitProducts(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if tracker.rollbackCount != 1 {
		t.Fatalf("expected rollback, got %d", tracker.rollbackCount)
	}
	if tracker.commitCount != 0 {
		t.Fatalf("expected no commit, got %d", tracker.commitCount)
	}
}
