package applaunch

import "testing"

func TestVisibleCategoriesFiltersByBudgetRole(t *testing.T) {
	categories := VisibleCategories(Catalog(map[string]string{BudgetAppID: "http://localhost:5174"}), []string{"app_budget_access"})
	// Only budget-specific app visible (no default-roles-cdlan)
	if len(categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(categories))
	}
	if categories[0].ID != "acquisti" {
		t.Fatalf("expected acquisti category, got %q", categories[0].ID)
	}
	if len(categories[0].Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(categories[0].Apps))
	}
	if categories[0].Apps[0].ID != BudgetAppID {
		t.Fatalf("expected budget app, got %q", categories[0].Apps[0].ID)
	}
}

func TestVisibleCategoriesDefaultRoleSeesAllPlaceholders(t *testing.T) {
	catalog := Catalog(map[string]string{BudgetAppID: "http://localhost:5174"})
	categories := VisibleCategories(catalog, []string{"default-roles-cdlan"})

	// Should see all 4 categories (acquisti placeholders, mkt-sales, smart-apps, provisioning)
	if len(categories) != 4 {
		t.Fatalf("expected 4 categories, got %d", len(categories))
	}

	// Count total apps across all categories
	total := 0
	for _, cat := range categories {
		total += len(cat.Apps)
	}
	// All placeholder apps (excludes budget which requires app_budget_access)
	if total != 23 {
		t.Fatalf("expected 23 placeholder apps, got %d", total)
	}
}

func TestVisibleCategoriesBothRolesSeesEverything(t *testing.T) {
	catalog := Catalog(map[string]string{BudgetAppID: "http://localhost:5174"})
	categories := VisibleCategories(catalog, []string{"default-roles-cdlan", "app_budget_access"})

	total := 0
	for _, cat := range categories {
		total += len(cat.Apps)
	}
	// All 24 apps (23 placeholders + 1 budget)
	if total != 24 {
		t.Fatalf("expected 24 total apps, got %d", total)
	}
}

func TestVisibleCategoriesHidesAppsWithoutRole(t *testing.T) {
	categories := VisibleCategories(Catalog(map[string]string{BudgetAppID: "http://localhost:5174"}), []string{"viewer"})
	if len(categories) != 0 {
		t.Fatalf("expected 0 categories, got %d", len(categories))
	}
}
