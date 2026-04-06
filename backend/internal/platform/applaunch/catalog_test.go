package applaunch

import "testing"

func TestVisibleCategoriesFiltersByRole(t *testing.T) {
	categories := VisibleCategories(Catalog("http://localhost:5174"), []string{"app_budget_access"})
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

func TestVisibleCategoriesHidesAppsWithoutRole(t *testing.T) {
	categories := VisibleCategories(Catalog("http://localhost:5174"), []string{"viewer"})
	if len(categories) != 0 {
		t.Fatalf("expected 0 categories, got %d", len(categories))
	}
}
