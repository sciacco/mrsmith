package applaunch

import "testing"

func TestVisibleCategoriesFiltersByBudgetRole(t *testing.T) {
	categories := VisibleCategories(Catalog(nil), []string{"app_budget_access"})
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
	if categories[0].Apps[0].Href != BudgetAppHref {
		t.Fatalf("expected budget href %q, got %q", BudgetAppHref, categories[0].Apps[0].Href)
	}
}

func TestVisibleCategoriesDefaultRoleSeesAllPlaceholders(t *testing.T) {
	catalog := Catalog(nil)
	categories := VisibleCategories(catalog, []string{"no-default-roles-cdlan"})

	// Should see all 4 categories (acquisti placeholders, mkt-sales, smart-apps, provisioning)
	if len(categories) != 4 {
		t.Fatalf("expected 4 categories, got %d", len(categories))
	}

	// Count total apps across all categories
	total := 0
	for _, cat := range categories {
		total += len(cat.Apps)
	}
	// All placeholder apps (excludes budget, compliance, kit-products, and listini which require specific roles)
	if total != 16 {
		t.Fatalf("expected 16 placeholder apps, got %d", total)
	}
}

func TestVisibleCategoriesBothRolesSeesEverything(t *testing.T) {
	catalog := Catalog(nil)
	categories := VisibleCategories(catalog, []string{"no-default-roles-cdlan", "app_budget_access", "app_compliance_access", "app_kitproducts_access", "app_listini_access"})

	total := 0
	for _, cat := range categories {
		total += len(cat.Apps)
	}
	// All 20 apps (16 placeholders + 1 budget + 1 compliance + 1 kit-products + 1 listini)
	if total != 20 {
		t.Fatalf("expected 20 total apps, got %d", total)
	}
}

func TestVisibleCategoriesFiltersByComplianceRole(t *testing.T) {
	categories := VisibleCategories(Catalog(nil), []string{"app_compliance_access"})
	if len(categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(categories))
	}
	if categories[0].ID != "smart-apps" {
		t.Fatalf("expected smart-apps category, got %q", categories[0].ID)
	}
	if len(categories[0].Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(categories[0].Apps))
	}
	if categories[0].Apps[0].ID != ComplianceAppID {
		t.Fatalf("expected compliance app, got %q", categories[0].Apps[0].ID)
	}
	if categories[0].Apps[0].Href != ComplianceAppHref {
		t.Fatalf("expected compliance href %q, got %q", ComplianceAppHref, categories[0].Apps[0].Href)
	}
}

func TestCatalogAppliesComplianceHrefOverride(t *testing.T) {
	catalog := Catalog(map[string]string{ComplianceAppID: "http://localhost:5175"})

	for _, definition := range catalog {
		if definition.ID != ComplianceAppID {
			continue
		}
		if definition.Href != "http://localhost:5175" {
			t.Fatalf("expected dev override href, got %q", definition.Href)
		}
		return
	}

	t.Fatal("expected compliance definition in catalog")
}

func TestVisibleCategoriesHidesAppsWithoutRole(t *testing.T) {
	categories := VisibleCategories(Catalog(nil), []string{"viewer"})
	if len(categories) != 0 {
		t.Fatalf("expected 0 categories, got %d", len(categories))
	}
}

func TestCatalogAppliesHrefOverrides(t *testing.T) {
	catalog := Catalog(map[string]string{BudgetAppID: "http://localhost:5174"})

	for _, definition := range catalog {
		if definition.ID != BudgetAppID {
			continue
		}
		if definition.Href != "http://localhost:5174" {
			t.Fatalf("expected dev override href, got %q", definition.Href)
		}
		return
	}

	t.Fatal("expected budget definition in catalog")
}

func TestVisibleCategoriesFiltersByListiniRole(t *testing.T) {
	categories := VisibleCategories(Catalog(nil), []string{"app_listini_access"})
	if len(categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(categories))
	}
	if categories[0].ID != "mkt-sales" {
		t.Fatalf("expected mkt-sales category, got %q", categories[0].ID)
	}
	if len(categories[0].Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(categories[0].Apps))
	}
	if categories[0].Apps[0].ID != ListiniAppID {
		t.Fatalf("expected listini app, got %q", categories[0].Apps[0].ID)
	}
	if categories[0].Apps[0].Href != ListiniAppHref {
		t.Fatalf("expected listini href %q, got %q", ListiniAppHref, categories[0].Apps[0].Href)
	}
}

func TestVisibleCategoriesFiltersByKitProductsRole(t *testing.T) {
	categories := VisibleCategories(Catalog(nil), []string{"app_kitproducts_access"})
	if len(categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(categories))
	}
	if categories[0].ID != "mkt-sales" {
		t.Fatalf("expected mkt-sales category, got %q", categories[0].ID)
	}
	if len(categories[0].Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(categories[0].Apps))
	}
	if categories[0].Apps[0].ID != KitProductsAppID {
		t.Fatalf("expected kit-products app, got %q", categories[0].Apps[0].ID)
	}
	if categories[0].Apps[0].Href != KitProductsAppHref {
		t.Fatalf("expected kit-products href %q, got %q", KitProductsAppHref, categories[0].Apps[0].Href)
	}
}
