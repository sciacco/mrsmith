//go:build api

package factorial

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// TestSmoke_LiveAPI exercises the real Factorial API. It is compiled only
// under `-tags api` so it never affects the default build or the default test
// run. It gates on credentials: if neither FACTORIAL_API_KEY nor
// FACTORIAL_TOKEN is set it skips cleanly. Credentials are never hardcoded;
// New() applies the FACTORIAL_* env fallbacks, and the bogus-key step uses an
// obviously-invalid literal string only.
//
// A single context with a generous timeout bounds every request so a hung or
// unreachable API turns into a failure rather than an indefinite hang.
func TestSmoke_LiveAPI(t *testing.T) {
	apiKey := os.Getenv("FACTORIAL_API_KEY")
	token := os.Getenv("FACTORIAL_TOKEN")
	if apiKey == "" && token == "" {
		t.Skip("skipping live smoke test: set FACTORIAL_API_KEY or FACTORIAL_TOKEN to run")
	}

	client := New()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: List employees. This is setup; a hard failure here (network,
	// auth, 5xx) aborts the test because nothing downstream can run. An empty
	// tenant is NOT a failure: it just means we skip the id-dependent steps.
	page, err := client.Employees.Employees.List(ctx, &EmployeesEmployeesListParams{
		OnlyActive:   true,
		OnlyManagers: false,
	})
	if err != nil {
		t.Fatalf("List employees failed: %v", err)
	}
	if page == nil {
		t.Fatalf("List employees returned nil page")
	}
	t.Logf("List returned %d employees (meta.total=%d)", len(page.Data), page.Meta.Total)

	emptyTenant := len(page.Data) == 0
	if emptyTenant {
		t.Logf("warning: tenant has 0 employees; skipping id-dependent sub-steps")
	}

	// Step 2: Paginate two pages. Only meaningful when there are at least two
	// employees; otherwise there is no second page to observe.
	t.Run("PaginatePages", func(t *testing.T) {
		if page.Meta.Total < 2 {
			t.Skipf("skipping pagination: meta.total=%d (< 2, cannot yield 2 pages)", page.Meta.Total)
		}
		pages := 0
		for pg, err := range client.Employees.Employees.PaginatePages(ctx,
			&EmployeesEmployeesListParams{OnlyActive: true, OnlyManagers: false},
			WithPageSize(1),
		) {
			if err != nil {
				t.Fatalf("PaginatePages failed: %v", err)
			}
			pages++
			t.Logf("page %d: %d items, meta.total=%d", pages, len(pg.Data), pg.Meta.Total)
			if pages >= 2 {
				break
			}
		}
		if pages < 1 {
			t.Fatalf("PaginatePages observed %d pages, want >= 1", pages)
		}
		t.Logf("PaginatePages observed %d page(s)", pages)
	})

	// Step 3: Get the first employee returned by the list above.
	t.Run("Get", func(t *testing.T) {
		if emptyTenant {
			t.Skip("skipping Get: no employees in tenant")
		}
		emp := page.Data[0]
		if emp.ID == nil {
			t.Fatalf("first employee has nil ID")
		}
		got, err := client.Employees.Employees.Get(ctx, *emp.ID)
		if err != nil {
			t.Fatalf("Get employee %q failed: %v", *emp.ID, err)
		}
		if got == nil {
			t.Fatalf("Get employee %q returned nil", *emp.ID)
		}
		if got.ID == nil {
			t.Fatalf("Get employee %q returned nil ID", *emp.ID)
		}
		if *got.ID != *emp.ID {
			t.Fatalf("Get returned id %q, list returned id %q", *got.ID, *emp.ID)
		}
		t.Logf("Get employee %q OK", *got.ID)
	})

	// Step 4: Query-parameter encoding decisions taken on indirect evidence
	// (PRD §12 decisions 7-8), validated against the live API:
	//   - the `ids[]` repeated-key array encoding actually filters;
	//   - the form-explode flattening of the performance object params
	//     (manager_employee_id=...&only_direct_reports=... with no
	//     managed_by_filter[...] wrapper) is accepted by the server.
	t.Run("QueryEncoding", func(t *testing.T) {
		t.Run("IDsArray", func(t *testing.T) {
			if emptyTenant {
				t.Skip("skipping ids[] check: no employees in tenant")
			}
			want := *page.Data[0].ID
			got, err := client.Employees.Employees.List(ctx, &EmployeesEmployeesListParams{
				IDs:          []string{want},
				OnlyActive:   true,
				OnlyManagers: false,
			})
			if err != nil {
				t.Fatalf("List employees with ids[] filter failed: %v", err)
			}
			if len(got.Data) != 1 {
				t.Fatalf("ids[]=%q returned %d employees, want exactly 1 (encoding not honored as a filter?)", want, len(got.Data))
			}
			if got.Data[0].ID == nil || *got.Data[0].ID != want {
				t.Fatalf("ids[]=%q returned employee id %v, want %q", want, got.Data[0].ID, want)
			}
			t.Logf("ids[] filter honored (1 employee, id=%q)", want)
		})

		t.Run("FormExplodeObjectParam", func(t *testing.T) {
			if emptyTenant {
				t.Skip("skipping form-explode check: no employee id to use as manager filter")
			}
			// An empty result is fine (the tenant may have no review
			// processes); what must NOT happen is a 4xx caused by the
			// flattened encoding being rejected. A 403 means the performance
			// module is not enabled for this tenant — skip, not fail.
			res, err := client.Performance.ReviewProcessTargets.List(ctx, &PerformanceReviewProcessTargetsListParams{
				ManagedByFilter: &PerformanceReviewProcessTargetsListParamsManagedByFilter{
					ManagerEmployeeID: *page.Data[0].ID,
					OnlyDirectReports: true,
				},
			})
			var apiErr *APIError
			if errors.As(err, &apiErr) && apiErr.IsForbidden() {
				t.Skipf("skipping form-explode check: performance module not available (status %d)", apiErr.StatusCode)
			}
			if err != nil {
				t.Fatalf("List review process targets with flattened object param failed: %v", err)
			}
			t.Logf("form-explode object param accepted (%d targets returned)", len(res.Data))
		})
	})

	// Step 5: Bogus credentials must surface as an *APIError. We do not assert
	// a specific status code (live API may return 401/403/etc.); we only
	// assert the error type. Note: a network/transport failure (e.g. the API
	// is unreachable) is NOT an *APIError and will fail this step — that is
	// intentional for a live smoke test.
	t.Run("BogusKey", func(t *testing.T) {
		// WithToken("") clears any FACTORIAL_TOKEN seeded from the env so the
		// negative-auth case is unconditional (only the bogus x-api-key is sent).
		bad := New(WithAPIKey("factorial-smoke-definitely-not-a-real-key"), WithToken(""))
		_, err := bad.Employees.Employees.List(ctx, &EmployeesEmployeesListParams{
			OnlyActive:   true,
			OnlyManagers: false,
		})
		if err == nil {
			t.Fatalf("expected error from bogus API key, got nil")
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		t.Logf("bogus key returned status %d (IsUnauthorized=%v)", apiErr.StatusCode, apiErr.IsUnauthorized())
	})
}
