package budget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/arak"
)

func TestRoundMoneyTo2(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		// The reported regression: 3dp wire values normalized to 2dp.
		{"56554.000", "56554.00"},
		{"100000.000", "100000.00"},
		{"0.000", "0.00"},
		// Pad to exactly 2 fractional digits.
		{"0", "0.00"},
		{"1500", "1500.00"},
		{"1500.5", "1500.50"},
		// 3rd digit < 5 rounds down.
		{"1.234", "1.23"},
		{"10963.881", "10963.88"},
		// 3rd digit > 5 rounds up.
		{"1.236", "1.24"},
		{"10963.889", "10963.89"},
		{"1234.567", "1234.57"},
		// Large value with 3rd digit.
		{"1234567.891", "1234567.89"},
		// Non-numeric input passes through unchanged.
		{"", ""},
		{"abc", "abc"},
		{"not-a-number", "not-a-number"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := roundMoneyTo2(tc.in); got != tc.want {
				t.Errorf("roundMoneyTo2(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRoundMoneyFields(t *testing.T) {
	t.Run("budget-details shape with nested allocations", func(t *testing.T) {
		// Mirrors the real budget-details response: top-level limit/current
		// plus arrays of user and cost-center allocations, each with their own.
		v := map[string]any{
			"id":      float64(17),
			"name":    "CEO",
			"limit":   "100000.000",
			"current": "0.000",
			"cost_center_budgets": []any{
				map[string]any{"limit": "50000.000", "current": "1234.567", "cost_center": "Board"},
				map[string]any{"limit": "20000.000", "current": "0.000", "cost_center": "People"},
			},
			"user_budgets": []any{
				map[string]any{"limit": "10000.000", "current": "0.000", "user_id": float64(1)},
			},
		}
		roundMoneyFields(v)

		want := map[string]string{
			"limit":                       "100000.00",
			"current":                     "0.00",
			"cc[0].limit":                 "50000.00",
			"cc[0].current":               "1234.57",
			"cc[1].limit":                 "20000.00",
			"cc[1].current":               "0.00",
			"ub[0].limit":                 "10000.00",
			"ub[0].current":               "0.00",
		}
		ccbs := v["cost_center_budgets"].([]any)
		ubs := v["user_budgets"].([]any)
		got := map[string]string{
			"limit":         v["limit"].(string),
			"current":       v["current"].(string),
			"cc[0].limit":   ccbs[0].(map[string]any)["limit"].(string),
			"cc[0].current": ccbs[0].(map[string]any)["current"].(string),
			"cc[1].limit":   ccbs[1].(map[string]any)["limit"].(string),
			"cc[1].current": ccbs[1].(map[string]any)["current"].(string),
			"ub[0].limit":   ubs[0].(map[string]any)["limit"].(string),
			"ub[0].current": ubs[0].(map[string]any)["current"].(string),
		}
		for k, w := range want {
			if got[k] != w {
				t.Errorf("%s = %q, want %q", k, got[k], w)
			}
		}
	})

	t.Run("paginated list items", func(t *testing.T) {
		v := map[string]any{
			"total_number":  float64(2),
			"current_page":  float64(1),
			"total_pages":   float64(1),
			"items": []any{
				map[string]any{"id": float64(1), "limit": "1000.000", "current": "0.000"},
				map[string]any{"id": float64(2), "limit": "2000.000", "current": "500.250"},
			},
		}
		roundMoneyFields(v)
		items := v["items"].([]any)
		if items[0].(map[string]any)["limit"] != "1000.00" {
			t.Errorf("items[0].limit = %v", items[0].(map[string]any)["limit"])
		}
		if items[1].(map[string]any)["current"] != "500.25" {
			t.Errorf("items[1].current = %v", items[1].(map[string]any)["current"])
		}
	})

	t.Run("non-money decimal fields are left untouched", func(t *testing.T) {
		// "current_page" shares the "current" prefix as a substring but is a
		// distinct key, and percentage-like fields must not be touched.
		// Guards against naive prefix/substring matching.
		v := map[string]any{
			"percentage":      "23.734",
			"current_page":    float64(1),
			"total_pages":     float64(3),
			"some_limit_max":  "99.999",
			"limit":           "100.000",
			"current":         "50.000",
		}
		roundMoneyFields(v)
		// Untouched
		if v["percentage"] != "23.734" {
			t.Errorf("percentage = %v, want 23.734 (non-money field)", v["percentage"])
		}
		if v["current_page"] != float64(1) {
			t.Errorf("current_page = %v, want 1 (pagination, not money)", v["current_page"])
		}
		if v["total_pages"] != float64(3) {
			t.Errorf("total_pages = %v, want 3", v["total_pages"])
		}
		if v["some_limit_max"] != "99.999" {
			t.Errorf("some_limit_max = %v, want 99.999 (key match is exact)", v["some_limit_max"])
		}
		// Rounded
		if v["limit"] != "100.00" {
			t.Errorf("limit = %v, want 100.00", v["limit"])
		}
		if v["current"] != "50.00" {
			t.Errorf("current = %v, want 50.00", v["current"])
		}
	})

	t.Run("numeric (non-string) money field is left alone", func(t *testing.T) {
		// Arak always emits strings, but if a number ever slips through we do
		// not coerce or round it — leave it for the caller to notice.
		v := map[string]any{"limit": float64(100)}
		roundMoneyFields(v)
		if v["limit"] != float64(100) {
			t.Errorf("limit = %v, want float64(100) unchanged", v["limit"])
		}
	})

	t.Run("empty and nil are no-ops", func(t *testing.T) {
		roundMoneyFields(nil)
		roundMoneyFields(map[string]any{})
		roundMoneyFields([]any{})
		roundMoneyFields("not a container")
		roundMoneyFields(float64(42))
	})
}

func TestProxyToArakRoundingMoney(t *testing.T) {
	t.Cleanup(func() {
		arakClient = nil
	})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "service-token",
				"expires_in":   300,
			})
		case "/arak/budget/v1/budget/17":
			// 3dp values as Arak emits them.
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      17,
				"name":    "CEO",
				"year":    2026,
				"limit":   "100000.000",
				"current": "0.000",
				"cost_center_budgets": []map[string]any{
					{"limit": "50000.000", "current": "1234.567", "cost_center": "Board", "enabled": true},
				},
			})
		case "/arak/budget/v1/budget/empty":
			// Empty 200 body — must not crash the transform.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
		case "/arak/budget/v1/budget/500":
			// Non-2xx error body must pass through untouched.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"upstream boom"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	mux := http.NewServeMux()
	RegisterRoutes(mux, arak.New(arak.Config{
		BaseURL:      upstream.URL,
		TokenURL:     upstream.URL + "/token",
		ClientID:     "budget-client",
		ClientSecret: "budget-secret",
	}))

	authReq := func(target string) *http.Request {
		return httptest.NewRequest(http.MethodGet, target, nil).WithContext(
			context.WithValue(context.Background(), auth.ClaimsKey, auth.Claims{
				Name: "John Doe", Email: "john@example.com", Roles: []string{"app_budget_access"},
			}),
		)
	}

	t.Run("rounds nested money to 2dp", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, authReq("/budget/v1/budget/17"))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if body["limit"] != "100000.00" {
			t.Errorf("limit = %v, want 100000.00", body["limit"])
		}
		if body["current"] != "0.00" {
			t.Errorf("current = %v, want 0.00", body["current"])
		}
		ccbs := body["cost_center_budgets"].([]any)
		cc0 := ccbs[0].(map[string]any)
		if cc0["limit"] != "50000.00" {
			t.Errorf("cc[0].limit = %v, want 50000.00", cc0["limit"])
		}
		if cc0["current"] != "1234.57" {
			t.Errorf("cc[0].current = %v, want 1234.57", cc0["current"])
		}
	})

	t.Run("empty 200 body is a no-op", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, authReq("/budget/v1/budget/empty"))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("expected empty body, got %q", rec.Body.String())
		}
	})

	t.Run("non-2xx passes through untouched", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, authReq("/budget/v1/budget/500"))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 passthrough, got %d", rec.Code)
		}
		if rec.Body.String() != `{"error":"upstream boom"}` {
			t.Errorf("expected untouched error body, got %q", rec.Body.String())
		}
	})
}
