package applaunch

import (
	"reflect"
	"sort"
	"testing"
)

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

func TestVisibleCategoriesDefaultRoleSeesNoActiveApps(t *testing.T) {
	categories := VisibleCategories(Catalog(nil), []string{"no-default-roles-cdlan"})
	if len(categories) != 0 {
		t.Fatalf("expected 0 categories, got %d", len(categories))
	}
}

func TestVisibleCategoriesAllCatalogRolesSeeAllActiveApps(t *testing.T) {
	catalog := Catalog(nil)
	categories := VisibleCategories(catalog, allCatalogRoles(catalog))

	if got, want := visibleAppIDs(categories), catalogAppIDs(catalog); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected all active app IDs %v, got %v", want, got)
	}
}

func TestVisibleCategoriesDevAdminSeesEverything(t *testing.T) {
	catalog := Catalog(nil)
	categories := VisibleCategories(catalog, []string{"app_devadmin"})

	if got, want := visibleAppIDs(categories), catalogAppIDs(catalog); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected all active app IDs %v for app_devadmin, got %v", want, got)
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

func TestVisibleCategoriesFiltersByCopertureRole(t *testing.T) {
	categories := VisibleCategories(Catalog(nil), []string{"app_coperture_access"})
	if len(categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(categories))
	}
	if categories[0].ID != "smart-apps" {
		t.Fatalf("expected smart-apps category, got %q", categories[0].ID)
	}
	if len(categories[0].Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(categories[0].Apps))
	}
	if categories[0].Apps[0].ID != CopertureAppID {
		t.Fatalf("expected coperture app, got %q", categories[0].Apps[0].ID)
	}
	if categories[0].Apps[0].Href != CopertureAppHref {
		t.Fatalf("expected coperture href %q, got %q", CopertureAppHref, categories[0].Apps[0].Href)
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

func TestCatalogAppliesCopertureHrefOverride(t *testing.T) {
	catalog := Catalog(map[string]string{CopertureAppID: "http://localhost:5183"})

	for _, definition := range catalog {
		if definition.ID != CopertureAppID {
			continue
		}
		if definition.Href != "http://localhost:5183" {
			t.Fatalf("expected dev override href, got %q", definition.Href)
		}
		return
	}

	t.Fatal("expected coperture definition in catalog")
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

func TestVisibleCategoriesFiltersByPanoramicaRole(t *testing.T) {
	categories := VisibleCategories(Catalog(nil), []string{"app_panoramica_access"})
	if len(categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(categories))
	}
	if categories[0].ID != "smart-apps" {
		t.Fatalf("expected smart-apps category, got %q", categories[0].ID)
	}
	if len(categories[0].Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(categories[0].Apps))
	}
	if categories[0].Apps[0].ID != PanoramicaAppID {
		t.Fatalf("expected panoramica app, got %q", categories[0].Apps[0].ID)
	}
}

func TestVisibleCategoriesFiltersByQuotesRole(t *testing.T) {
	categories := VisibleCategories(Catalog(nil), []string{"app_quotes_access"})
	if len(categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(categories))
	}
	if categories[0].ID != "mkt-sales" {
		t.Fatalf("expected mkt-sales category, got %q", categories[0].ID)
	}
	if len(categories[0].Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(categories[0].Apps))
	}
	if categories[0].Apps[0].ID != QuotesAppID {
		t.Fatalf("expected quotes app, got %q", categories[0].Apps[0].ID)
	}
	if categories[0].Apps[0].Href != QuotesAppHref {
		t.Fatalf("expected quotes href %q, got %q", QuotesAppHref, categories[0].Apps[0].Href)
	}
}

func TestVisibleCategoriesFiltersByReportsRole(t *testing.T) {
	categories := VisibleCategories(Catalog(nil), []string{"app_reports_access"})
	if len(categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(categories))
	}
	if categories[0].ID != "smart-apps" {
		t.Fatalf("expected smart-apps category, got %q", categories[0].ID)
	}
	if len(categories[0].Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(categories[0].Apps))
	}
	if categories[0].Apps[0].ID != ReportsAppID {
		t.Fatalf("expected reports app, got %q", categories[0].Apps[0].ID)
	}
	if categories[0].Apps[0].Href != ReportsAppHref {
		t.Fatalf("expected reports href %q, got %q", ReportsAppHref, categories[0].Apps[0].Href)
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

func visibleAppIDs(categories []Category) []string {
	ids := make([]string, 0)
	for _, category := range categories {
		for _, app := range category.Apps {
			ids = append(ids, app.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func catalogAppIDs(definitions []Definition) []string {
	ids := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		ids = append(ids, definition.ID)
	}
	sort.Strings(ids)
	return ids
}

func allCatalogRoles(definitions []Definition) []string {
	seen := make(map[string]struct{})
	roles := make([]string, 0)
	for _, definition := range definitions {
		for _, role := range definition.AccessRoles {
			if role == "" {
				continue
			}
			if _, ok := seen[role]; ok {
				continue
			}
			seen[role] = struct{}{}
			roles = append(roles, role)
		}
	}
	sort.Strings(roles)
	return roles
}
