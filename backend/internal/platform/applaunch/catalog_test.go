package applaunch

import (
	"reflect"
	"sort"
	"testing"
)

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

func TestCatalogAppliesEnergiaDCHrefOverride(t *testing.T) {
	catalog := Catalog(map[string]string{EnergiaDCAppID: "http://localhost:5184"})

	for _, definition := range catalog {
		if definition.ID != EnergiaDCAppID {
			continue
		}
		if definition.Href != "http://localhost:5184" {
			t.Fatalf("expected dev override href, got %q", definition.Href)
		}
		return
	}

	t.Fatal("expected energia-dc definition in catalog")
}

func TestCatalogAppliesSimulatoriVenditaHrefOverride(t *testing.T) {
	catalog := Catalog(map[string]string{SimulatoriVenditaAppID: "http://localhost:5185"})

	for _, definition := range catalog {
		if definition.ID != SimulatoriVenditaAppID {
			continue
		}
		if definition.Href != "http://localhost:5185" {
			t.Fatalf("expected dev override href, got %q", definition.Href)
		}
		return
	}

	t.Fatal("expected simulatori-vendita definition in catalog")
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
