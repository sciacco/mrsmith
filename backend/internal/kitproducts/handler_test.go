package kitproducts

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/logging"
)

func TestRequireDBReturnsServiceUnavailableWhenMistraNotConfigured(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()

	ok := h.requireDB(rec)

	if ok {
		t.Fatal("expected requireDB to return false")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"database not configured"}` {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestHandleListVocabularyRequiresSection(t *testing.T) {
	h := &Handler{mistraDB: openKitProductsTestDB(t, "success", nil)}

	req := httptest.NewRequest(http.MethodGet, "/kit-products/v1/lookup/vocabulary", nil)
	rec := httptest.NewRecorder()

	h.handleListVocabulary(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"section is required"}` {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestHandleCreateCategoryRejectsInvalidColor(t *testing.T) {
	h := &Handler{mistraDB: openKitProductsTestDB(t, "success", nil)}

	req := httptest.NewRequest(http.MethodPost, "/kit-products/v1/category", strings.NewReader(`{"name":"Networking","color":"amber"}`))
	rec := httptest.NewRecorder()

	h.handleCreateCategory(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"invalid color"}` {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestHandleBatchUpdateCustomerGroupsRejectsReadOnlyGroupAndRollsBack(t *testing.T) {
	tracker := &kitProductsTxTracker{}
	h := &Handler{mistraDB: openKitProductsTestDB(t, "readonly", tracker)}

	req := httptest.NewRequest(http.MethodPatch, "/kit-products/v1/customer-group", strings.NewReader(`{"items":[{"id":1,"name":"VIP","is_partner":true}]}`))
	rec := httptest.NewRecorder()

	h.handleBatchUpdateCustomerGroups(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"read_only_group"}` {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
	if tracker.rollbackCount != 1 {
		t.Fatalf("expected 1 rollback, got %d", tracker.rollbackCount)
	}
	if tracker.commitCount != 0 {
		t.Fatalf("expected 0 commits, got %d", tracker.commitCount)
	}
}

func TestHandleBatchUpdateCustomerGroupsReturnsNotFoundAndRollsBack(t *testing.T) {
	tracker := &kitProductsTxTracker{}
	h := &Handler{mistraDB: openKitProductsTestDB(t, "missing", tracker)}

	req := httptest.NewRequest(http.MethodPatch, "/kit-products/v1/customer-group", strings.NewReader(`{"items":[{"id":99,"name":"Missing","is_partner":false}]}`))
	rec := httptest.NewRecorder()

	h.handleBatchUpdateCustomerGroups(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"not_found"}` {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
	if tracker.rollbackCount != 1 {
		t.Fatalf("expected 1 rollback, got %d", tracker.rollbackCount)
	}
}

func TestHandleBatchUpdateCustomerGroupsRollsBackOnExecFailure(t *testing.T) {
	tracker := &kitProductsTxTracker{}
	h := &Handler{mistraDB: openKitProductsTestDB(t, "exec-fail", tracker)}

	var buf bytes.Buffer
	logger := logging.NewWithWriter(&buf, "debug")

	req := httptest.NewRequest(http.MethodPatch, "/kit-products/v1/customer-group", strings.NewReader(`{"items":[{"id":1,"name":"VIP","is_partner":true}]}`))
	req = req.WithContext(logging.IntoContext(req.Context(), logger))
	rec := httptest.NewRecorder()

	h.handleBatchUpdateCustomerGroups(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "write failed") {
		t.Fatalf("expected sanitized response body, got %q", rec.Body.String())
	}
	if tracker.rollbackCount != 1 {
		t.Fatalf("expected 1 rollback, got %d", tracker.rollbackCount)
	}
}

func TestHandleBatchUpdateCustomerGroupsCommitsSuccessfulBatch(t *testing.T) {
	tracker := &kitProductsTxTracker{}
	h := &Handler{mistraDB: openKitProductsTestDB(t, "success", tracker)}

	req := httptest.NewRequest(http.MethodPatch, "/kit-products/v1/customer-group", strings.NewReader(`{"items":[{"id":1,"name":"VIP","is_partner":true},{"id":2,"name":"Wholesale","is_partner":false}]}`))
	rec := httptest.NewRecorder()

	h.handleBatchUpdateCustomerGroups(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["updated"] != 2 {
		t.Fatalf("expected updated=2, got %#v", body["updated"])
	}
	if tracker.commitCount != 1 {
		t.Fatalf("expected 1 commit, got %d", tracker.commitCount)
	}
	if tracker.rollbackCount != 0 {
		t.Fatalf("expected 0 rollbacks, got %d", tracker.rollbackCount)
	}
}

func TestHandleCreateProductRejectsInvalidCategoryID(t *testing.T) {
	h := &Handler{mistraDB: openKitProductsTestDB(t, "success", nil)}

	req := httptest.NewRequest(http.MethodPost, "/kit-products/v1/product", strings.NewReader(`{"code":"PRD001","internal_name":"Router","category_id":0,"nrc":10,"mrc":5}`))
	rec := httptest.NewRecorder()

	h.handleCreateProduct(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"category_id is required"}` {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestHandleCreateProductRejectsUnknownCategoryID(t *testing.T) {
	h := &Handler{mistraDB: openKitProductsTestDB(t, "invalid-category", nil)}

	req := httptest.NewRequest(http.MethodPost, "/kit-products/v1/product", strings.NewReader(`{"code":"PRD001","internal_name":"Router","category_id":999,"nrc":10,"mrc":5}`))
	rec := httptest.NewRecorder()

	h.handleCreateProduct(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"invalid category_id"}` {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestHandleUpdateProductReturnsNotFoundWhenMissing(t *testing.T) {
	h := &Handler{mistraDB: openKitProductsTestDB(t, "update-missing", nil)}

	req := httptest.NewRequest(http.MethodPut, "/kit-products/v1/product/MISSING", strings.NewReader(`{"internal_name":"Router","category_id":1,"nrc":10,"mrc":5,"erp_sync":true}`))
	req.SetPathValue("code", "MISSING")
	rec := httptest.NewRecorder()

	h.handleUpdateProduct(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"not_found"}` {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestHandleUpdateProductTranslationsRejectsMissingRequiredShortDescriptions(t *testing.T) {
	h := &Handler{mistraDB: openKitProductsTestDB(t, "product-success", nil)}

	tests := []struct {
		name string
		body string
	}{
		{
			name: "empty array",
			body: `{"translations":[]}`,
		},
		{
			name: "unsupported language only",
			body: `{"translations":[{"language":"fr","short":"Routeur","long":"Routeur FR"}]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/kit-products/v1/product/PRD001/translations", strings.NewReader(tc.body))
			req.SetPathValue("code", "PRD001")
			rec := httptest.NewRecorder()

			h.handleUpdateProductTranslations(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rec.Code)
			}
			if strings.TrimSpace(rec.Body.String()) != `{"error":"short translation is required for it and en"}` {
				t.Fatalf("unexpected response body: %q", rec.Body.String())
			}
		})
	}
}

func TestHandleUpdateProductTranslationsSkipsWarningWhenAlyanteMissing(t *testing.T) {
	h := &Handler{mistraDB: openKitProductsTestDB(t, "product-success", nil)}

	req := httptest.NewRequest(http.MethodPut, "/kit-products/v1/product/PRD001/translations", strings.NewReader(`{"translations":[{"language":"it","short":"Router","long":"Router IT"},{"language":"en","short":"Router","long":"Router EN"}]}`))
	req.SetPathValue("code", "PRD001")
	rec := httptest.NewRecorder()

	h.handleUpdateProductTranslations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := body["warning"]; ok {
		t.Fatalf("expected no warning payload, got %#v", body["warning"])
	}
}

func TestHandleUpdateProductTranslationsSkipsERPWhenSyncDisabled(t *testing.T) {
	var syncCalls int
	h := &Handler{
		mistraDB: openKitProductsTestDB(t, "product-erp-off", nil),
		alyante: &AlyanteAdapter{
			syncFn: func(context.Context, string, string, string) error {
				syncCalls++
				return nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPut, "/kit-products/v1/product/PRD001/translations", strings.NewReader(`{"translations":[{"language":"it","short":"Router","long":"Router IT"},{"language":"en","short":"Router","long":"Router EN"}]}`))
	req.SetPathValue("code", "PRD001")
	rec := httptest.NewRecorder()

	h.handleUpdateProductTranslations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if syncCalls != 0 {
		t.Fatalf("expected ERP sync to be skipped, got %d calls", syncCalls)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := body["warning"]; ok {
		t.Fatalf("expected no warning payload, got %#v", body["warning"])
	}
}

func TestHandleUpdateProductTranslationsReturnsWarningOnERPFailure(t *testing.T) {
	var syncCalls int
	h := &Handler{
		mistraDB: openKitProductsTestDB(t, "product-success", nil),
		alyante: &AlyanteAdapter{
			syncFn: func(context.Context, string, string, string) error {
				syncCalls++
				return errors.New("erp unavailable")
			},
		},
	}

	req := httptest.NewRequest(http.MethodPut, "/kit-products/v1/product/PRD001/translations", strings.NewReader(`{"translations":[{"language":"it","short":"Router","long":"Router IT"},{"language":"en","short":"Router","long":"Router EN"}]}`))
	req.SetPathValue("code", "PRD001")
	rec := httptest.NewRecorder()

	h.handleUpdateProductTranslations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if syncCalls == 0 {
		t.Fatal("expected at least one ERP sync attempt")
	}
	var body struct {
		Warning map[string]string `json:"warning"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Warning["code"] != "erp_sync_failed" {
		t.Fatalf("expected erp_sync_failed warning, got %#v", body.Warning)
	}
}

func openKitProductsTestDB(t *testing.T, mode string, tracker *kitProductsTxTracker) *sql.DB {
	t.Helper()

	registerKitProductsTestDriver()

	db, err := sql.Open(kitProductsTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if tracker != nil {
		kitProductsTrackers.Store(mode, tracker)
		t.Cleanup(func() {
			kitProductsTrackers.Delete(mode)
		})
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const kitProductsTestDriverName = "kitproducts_test_driver"

var (
	registerKitProductsDriverOnce sync.Once
	kitProductsTrackers           sync.Map
)

func registerKitProductsTestDriver() {
	registerKitProductsDriverOnce.Do(func() {
		sql.Register(kitProductsTestDriverName, kitProductsTestDriver{})
	})
}

type kitProductsTxTracker struct {
	commitCount   int
	rollbackCount int
	queries       []kitProductsDBCall
	execs         []kitProductsDBCall
}

type kitProductsDBCall struct {
	Query string
	Args  []any
}

type kitProductsTestDriver struct{}

func (kitProductsTestDriver) Open(name string) (driver.Conn, error) {
	return &kitProductsTestConn{mode: name}, nil
}

type kitProductsTestConn struct {
	mode string
	tx   *kitProductsTestTx
}

func (c *kitProductsTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *kitProductsTestConn) Close() error { return nil }

func (c *kitProductsTestConn) Begin() (driver.Tx, error) {
	return c.begin()
}

func (c *kitProductsTestConn) BeginTx(ctx context.Context, _ driver.TxOptions) (driver.Tx, error) {
	_ = ctx
	return c.begin()
}

func (c *kitProductsTestConn) begin() (driver.Tx, error) {
	tx := &kitProductsTestTx{conn: c}
	c.tx = tx
	return tx, nil
}

func (c *kitProductsTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	recordQuery(c.mode, query, args)

	if strings.Contains(query, "SELECT read_only") {
		id := namedInt(args[0])
		switch c.mode {
		case "readonly":
			return &kitProductsTestRows{columns: []string{"read_only"}, values: [][]driver.Value{{true}}}, nil
		case "missing":
			return &kitProductsTestRows{columns: []string{"read_only"}}, nil
		default:
			if id <= 0 {
				return &kitProductsTestRows{columns: []string{"read_only"}}, nil
			}
			return &kitProductsTestRows{columns: []string{"read_only"}, values: [][]driver.Value{{false}}}, nil
		}
	}
	if strings.Contains(query, "SELECT EXISTS") && strings.Contains(query, "FROM products.kit") {
		switch c.mode {
		case "kit-missing":
			return &kitProductsTestRows{columns: []string{"exists"}, values: [][]driver.Value{{false}}}, nil
		default:
			return &kitProductsTestRows{columns: []string{"exists"}, values: [][]driver.Value{{true}}}, nil
		}
	}
	if strings.Contains(query, "SELECT EXISTS") && strings.Contains(query, "FROM products.product_category") {
		switch c.mode {
		case "invalid-category":
			return &kitProductsTestRows{columns: []string{"exists"}, values: [][]driver.Value{{false}}}, nil
		default:
			return &kitProductsTestRows{columns: []string{"exists"}, values: [][]driver.Value{{true}}}, nil
		}
	}
	if strings.Contains(query, "SELECT EXISTS") && strings.Contains(query, "FROM products.product") {
		switch c.mode {
		case "invalid-main-product":
			return &kitProductsTestRows{columns: []string{"exists"}, values: [][]driver.Value{{false}}}, nil
		default:
			code := namedString(args[0])
			if code == "" {
				return &kitProductsTestRows{columns: []string{"exists"}, values: [][]driver.Value{{false}}}, nil
			}
			return &kitProductsTestRows{columns: []string{"exists"}, values: [][]driver.Value{{true}}}, nil
		}
	}
	if strings.Contains(query, "FROM common.language") {
		return &kitProductsTestRows{
			columns: []string{"iso", "name"},
			values:  [][]driver.Value{{"en", "English"}, {"it", "Italiano"}},
		}, nil
	}
	if strings.Contains(query, "SELECT EXISTS") && strings.Contains(query, "FROM common.vocabulary") {
		switch c.mode {
		case "product-group-duplicate":
			return &kitProductsTestRows{columns: []string{"exists"}, values: [][]driver.Value{{true}}}, nil
		default:
			return &kitProductsTestRows{columns: []string{"exists"}, values: [][]driver.Value{{false}}}, nil
		}
	}
	if strings.Contains(query, "SELECT COUNT(*)") && strings.Contains(query, "FROM products.kit_product") && strings.Contains(query, "WHERE group_name = $1") {
		switch c.mode {
		case "product-group-rename-confirm":
			return &kitProductsTestRows{columns: []string{"count"}, values: [][]driver.Value{{int64(3)}}}, nil
		default:
			return &kitProductsTestRows{columns: []string{"count"}, values: [][]driver.Value{{int64(1)}}}, nil
		}
	}
	if strings.Contains(query, "SELECT name") && strings.Contains(query, "FROM common.vocabulary") && strings.Contains(query, "translation_uuid = $2") {
		switch c.mode {
		case "product-group-missing":
			return &kitProductsTestRows{columns: []string{"name"}}, nil
		default:
			return &kitProductsTestRows{columns: []string{"name"}, values: [][]driver.Value{{"Circuito"}}}, nil
		}
	}
	if strings.Contains(query, "INSERT INTO common.vocabulary") && strings.Contains(query, "RETURNING translation_uuid") {
		return &kitProductsTestRows{
			columns: []string{"translation_uuid"},
			values:  [][]driver.Value{{"44444444-4444-4444-4444-444444444444"}},
		}, nil
	}
	if strings.Contains(query, "FROM common.vocabulary v") && strings.Contains(query, "COALESCE(common.get_translations(v.translation_uuid), '[]'::json)") {
		translations := []byte(`[{"language":"en","short":"Circuit","long":""},{"language":"it","short":"Circuito","long":"Descrizione"}]`)
		translationUUID := "44444444-4444-4444-4444-444444444444"
		if strings.Contains(query, "translation_uuid = $2") && len(args) > 1 {
			translationUUID = namedString(args[1])
		}
		switch c.mode {
		case "product-group-missing":
			return &kitProductsTestRows{columns: []string{"name", "translation_uuid", "usage_count", "translations"}}, nil
		case "product-group-list-empty":
			return &kitProductsTestRows{columns: []string{"name", "translation_uuid", "usage_count", "translations"}}, nil
		default:
			return &kitProductsTestRows{
				columns: []string{"name", "translation_uuid", "usage_count", "translations"},
				values: [][]driver.Value{{
					"Circuito",
					translationUUID,
					int64(2),
					translations,
				}},
			}, nil
		}
	}
	if strings.Contains(query, "SELECT bundle_prefix") && strings.Contains(query, "FROM products.kit") {
		switch c.mode {
		case "kit-update-empty-prefix":
			return &kitProductsTestRows{columns: []string{"bundle_prefix"}, values: [][]driver.Value{{""}}}, nil
		default:
			return &kitProductsTestRows{columns: []string{"bundle_prefix"}, values: [][]driver.Value{{"KIT"}}}, nil
		}
	}
	if strings.Contains(query, "SELECT translation_uuid") && strings.Contains(query, "FROM products.kit") {
		return &kitProductsTestRows{columns: []string{"translation_uuid"}, values: [][]driver.Value{{"22222222-2222-2222-2222-222222222222"}}}, nil
	}
	if strings.Contains(query, "SELECT products.new_kit($1::json)") {
		return &kitProductsTestRows{columns: []string{"new_kit"}, values: [][]driver.Value{{int64(42)}}}, nil
	}
	if strings.Contains(query, "SELECT products.clone_kit($1, $2)") {
		return &kitProductsTestRows{columns: []string{"clone_kit"}, values: [][]driver.Value{{int64(43)}}}, nil
	}
	if strings.Contains(query, "SELECT products.new_kit_product($1::json)") {
		return &kitProductsTestRows{columns: []string{"new_kit_product"}, values: [][]driver.Value{{int64(88)}}}, nil
	}
	if strings.Contains(query, "SELECT products.upd_kit_product($1, $2::json)") {
		switch c.mode {
		case "kit-product-update-fail":
			return nil, errors.New("write failed")
		default:
			return &kitProductsTestRows{columns: []string{"upd_kit_product"}, values: [][]driver.Value{{true}}}, nil
		}
	}
	if strings.Contains(query, "SELECT products.upd_kit($1, $2::json)") {
		switch c.mode {
		case "kit-update-missing":
			return &kitProductsTestRows{columns: []string{"upd_kit"}, values: [][]driver.Value{{false}}}, nil
		default:
			return &kitProductsTestRows{columns: []string{"upd_kit"}, values: [][]driver.Value{{true}}}, nil
		}
	}
	if strings.Contains(query, "SELECT common.upd_translation($1, $2::json)") {
		return &kitProductsTestRows{columns: []string{"upd_translation"}, values: [][]driver.Value{{int64(2)}}}, nil
	}
	if strings.Contains(query, "FROM products.kit k") && strings.Contains(query, "common.get_translations(k.translation_uuid)") {
		translations := []byte(`[{"language":"it","short":"Kit IT","long":"Long IT"},{"language":"en","short":"Kit EN","long":"Long EN"}]`)
		sellable := []byte(`[1,2]`)
		return &kitProductsTestRows{
			columns: []string{"id", "internal_name", "main_product_code", "initial_subscription_months", "next_subscription_months", "activation_time_days", "nrc", "mrc", "translation_uuid", "bundle_prefix", "ecommerce", "category_id", "name", "color", "is_main_prd_sellable", "is_active", "quotable", "billing_period", "sconto_massimo", "variable_billing", "h24_assurance", "sla_resolution_hours", "notes", "translations", "ms_sellable_to", "help_url"},
			values: [][]driver.Value{{
				int64(9),
				"Kit One",
				"MAIN001",
				int64(12),
				int64(12),
				int64(30),
				float64(99.5),
				float64(19.5),
				"33333333-3333-3333-3333-333333333333",
				"KIT",
				true,
				int64(1),
				"Networking",
				"#231F20",
				true,
				true,
				true,
				int64(3),
				float64(0),
				false,
				false,
				int64(8),
				"notes",
				translations,
				sellable,
				"https://example.test/help",
			}},
		}, nil
	}
	if strings.Contains(query, "FROM products.kit k") {
		return &kitProductsTestRows{
			columns: []string{"id", "internal_name", "main_product_code", "initial_subscription_months", "next_subscription_months", "activation_time_days", "nrc", "mrc", "translation_uuid", "bundle_prefix", "ecommerce", "category_id", "name", "color", "is_main_prd_sellable", "is_active", "quotable", "billing_period", "sconto_massimo", "variable_billing", "h24_assurance", "sla_resolution_hours", "notes"},
			values: [][]driver.Value{{
				int64(7),
				"Kit One",
				"MAIN001",
				int64(12),
				int64(12),
				int64(30),
				float64(99.5),
				float64(19.5),
				"33333333-3333-3333-3333-333333333333",
				"KIT",
				true,
				int64(1),
				"Networking",
				"#231F20",
				true,
				true,
				true,
				int64(3),
				float64(0),
				false,
				false,
				int64(8),
				"notes",
			}},
		}, nil
	}
	if strings.Contains(query, "FROM products.kit_product kp") && strings.Contains(query, "WHERE kp.id = $1 AND kp.kit_id = $2") {
		productID := namedInt(args[0])
		kitID := namedInt(args[1])
		return &kitProductsTestRows{
			columns: []string{"id", "kit_id", "product_code", "name", "product_internal_name", "minimum", "maximum", "required", "nrc", "mrc", "position", "group_name", "notes", "img_url"},
			values: [][]driver.Value{{
				productID,
				kitID,
				"PRD001",
				"Router",
				"Router",
				int64(1),
				int64(3),
				true,
				float64(10),
				float64(5),
				int64(1),
				"Access",
				"notes",
				"https://example.test/router.png",
			}},
		}, nil
	}
	if strings.Contains(query, "SELECT kit_id") && strings.Contains(query, "FROM products.kit_product") && strings.Contains(query, "WHERE id = $1") {
		switch c.mode {
		case "kit-product-wrong-owner":
			return &kitProductsTestRows{columns: []string{"kit_id"}, values: [][]driver.Value{{int64(8)}}}, nil
		case "kit-product-missing-owner":
			return &kitProductsTestRows{columns: []string{"kit_id"}}, nil
		default:
			return &kitProductsTestRows{columns: []string{"kit_id"}, values: [][]driver.Value{{int64(7)}}}, nil
		}
	}
	if strings.Contains(query, "FROM products.kit_product kp") && strings.Contains(query, "WHERE kp.kit_id = $1") {
		return &kitProductsTestRows{
			columns: []string{"id", "kit_id", "product_code", "name", "product_internal_name", "minimum", "maximum", "required", "nrc", "mrc", "position", "group_name", "notes", "img_url"},
			values: [][]driver.Value{{
				int64(21),
				int64(7),
				"PRD001",
				"Router",
				"Router",
				int64(1),
				int64(3),
				true,
				float64(10),
				float64(5),
				int64(1),
				"Access",
				"notes",
				"https://example.test/router.png",
			}, {
				int64(22),
				int64(7),
				"PRD002",
				"Switch",
				"Switch",
				int64(1),
				int64(2),
				false,
				float64(20),
				float64(7),
				int64(2),
				nil,
				"",
				"",
			}},
		}, nil
	}
	if strings.Contains(query, "WITH inserted AS") && strings.Contains(query, "products.kit_custom_value") {
		return &kitProductsTestRows{columns: []string{"id"}, values: [][]driver.Value{{int64(31)}}}, nil
	}
	if strings.Contains(query, "FROM products.kit_custom_value") && strings.Contains(query, "WHERE id = $1 AND kit_id = $2") {
		valueID := namedInt(args[0])
		kitID := namedInt(args[1])
		return &kitProductsTestRows{
			columns: []string{"id", "kit_id", "key_name", "value"},
			values:  [][]driver.Value{{valueID, kitID, "legal_notes", "{\n    \"it\": \"nota\",\n    \"en\": \"note\"\n}"}},
		}, nil
	}
	if strings.Contains(query, "FROM products.kit_custom_value") && strings.Contains(query, "WHERE kit_id = $1") {
		return &kitProductsTestRows{
			columns: []string{"id", "kit_id", "key_name", "value"},
			values:  [][]driver.Value{{int64(31), int64(7), "legal_notes", "{\n    \"it\": \"nota\",\n    \"en\": \"note\"\n}"}, {int64(32), int64(7), "service_level", "{\"value\": \"gold\"}"}},
		}, nil
	}
	if strings.Contains(query, "SELECT translation_uuid, COALESCE(erp_sync, true)") {
		switch c.mode {
		case "product-missing":
			return &kitProductsTestRows{columns: []string{"translation_uuid", "erp_sync"}}, nil
		case "product-erp-off":
			return &kitProductsTestRows{columns: []string{"translation_uuid", "erp_sync"}, values: [][]driver.Value{{"11111111-1111-1111-1111-111111111111", false}}}, nil
		default:
			return &kitProductsTestRows{columns: []string{"translation_uuid", "erp_sync"}, values: [][]driver.Value{{"11111111-1111-1111-1111-111111111111", true}}}, nil
		}
	}
	if strings.Contains(query, "FROM products.product p") {
		translations := []byte(`[{"language":"it","short":"Router","long":"Router IT"},{"language":"en","short":"Router","long":"Router EN"}]`)
		return &kitProductsTestRows{
			columns: []string{"code", "internal_name", "category_id", "name", "color", "translation_uuid", "nrc", "mrc", "img_url", "erp_sync", "asset_flow", "translations"},
			values: [][]driver.Value{{
				"PRD001",
				"Router",
				int64(1),
				"Networking",
				"#231F20",
				"11111111-1111-1111-1111-111111111111",
				float64(10),
				float64(5),
				nil,
				true,
				nil,
				translations,
			}},
		}, nil
	}
	return &kitProductsTestRows{columns: []string{"stub"}, values: [][]driver.Value{}}, nil
}

func (c *kitProductsTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	recordExec(c.mode, query, args)

	if c.mode == "exec-fail" {
		return nil, errors.New("write failed")
	}
	switch {
	case strings.Contains(query, "UPDATE common.vocabulary"):
		if c.mode == "product-group-missing" {
			return driver.RowsAffected(0), nil
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "UPDATE products.kit_product") && strings.Contains(query, "SET group_name = $1"):
		return driver.RowsAffected(3), nil
	case strings.Contains(query, "UPDATE products.kit SET is_active = false"):
		if c.mode == "kit-delete-missing" {
			return driver.RowsAffected(0), nil
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "INSERT INTO products.kit_help"):
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "DELETE FROM products.kit_product"):
		if c.mode == "kit-product-delete-missing" {
			return driver.RowsAffected(0), nil
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "UPDATE products.kit_custom_value"):
		if c.mode == "kit-custom-update-missing" {
			return driver.RowsAffected(0), nil
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "DELETE FROM products.kit_custom_value"):
		if c.mode == "kit-custom-delete-missing" {
			return driver.RowsAffected(0), nil
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "UPDATE products.kit_product"):
		if c.mode == "kit-product-update-fail" {
			return nil, errors.New("write failed")
		}
		return driver.RowsAffected(1), nil
	}
	if strings.Contains(query, "UPDATE products.product") && c.mode == "update-missing" {
		return driver.RowsAffected(0), nil
	}
	return driver.RowsAffected(1), nil
}

func (c *kitProductsTestConn) Ping(context.Context) error { return nil }

type kitProductsTestTx struct {
	conn      *kitProductsTestConn
	committed bool
}

func (tx *kitProductsTestTx) Commit() error {
	tx.committed = true
	if tracker := trackerForMode(tx.conn.mode); tracker != nil {
		tracker.commitCount++
	}
	return nil
}

func (tx *kitProductsTestTx) Rollback() error {
	if tx.committed {
		return sql.ErrTxDone
	}
	if tracker := trackerForMode(tx.conn.mode); tracker != nil {
		tracker.rollbackCount++
	}
	return nil
}

func trackerForMode(mode string) *kitProductsTxTracker {
	if tracker, ok := kitProductsTrackers.Load(mode); ok {
		return tracker.(*kitProductsTxTracker)
	}
	return nil
}

func recordQuery(mode, query string, args []driver.NamedValue) {
	if tracker := trackerForMode(mode); tracker != nil {
		tracker.queries = append(tracker.queries, kitProductsDBCall{
			Query: query,
			Args:  namedValues(args),
		})
	}
}

func recordExec(mode, query string, args []driver.NamedValue) {
	if tracker := trackerForMode(mode); tracker != nil {
		tracker.execs = append(tracker.execs, kitProductsDBCall{
			Query: query,
			Args:  namedValues(args),
		})
	}
}

func namedValues(args []driver.NamedValue) []any {
	values := make([]any, 0, len(args))
	for _, arg := range args {
		values = append(values, arg.Value)
	}
	return values
}

func namedInt(value driver.NamedValue) int64 {
	switch v := value.Value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}

func namedString(value driver.NamedValue) string {
	switch v := value.Value.(type) {
	case string:
		return v
	case interface{ String() string }:
		return v.String()
	}
	return ""
}

var _ driver.QueryerContext = (*kitProductsTestConn)(nil)
var _ driver.ExecerContext = (*kitProductsTestConn)(nil)
var _ driver.Pinger = (*kitProductsTestConn)(nil)
var _ driver.ConnBeginTx = (*kitProductsTestConn)(nil)

type kitProductsTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *kitProductsTestRows) Columns() []string {
	return r.columns
}

func (r *kitProductsTestRows) Close() error { return nil }

func (r *kitProductsTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
