package applaunch

import (
	"slices"
	"strings"

	"github.com/sciacco/mrsmith/internal/authz"
)

const (
	BudgetAppID   = "budget"
	BudgetAppHref = "/apps/budget/"

	ComplianceAppID   = "compliance"
	ComplianceAppHref = "/apps/compliance/"

	KitProductsAppID   = "kit-e-prodotti"
	KitProductsAppHref = "/apps/kit-products/"

	ListiniAppID   = "listini-e-sconti"
	ListiniAppHref = "/apps/listini-e-sconti/"

	PanoramicaAppID   = "panoramica-cliente"
	PanoramicaAppHref = "/apps/panoramica-cliente/"

	QuotesAppID   = "proposte"
	QuotesAppHref = "/apps/quotes/"

	RDFBackendAppID   = "rdf-backend"
	RDFBackendAppHref = "/apps/rdf-backend/"

	ReportsAppID   = "reports"
	ReportsAppHref = "/apps/reports/"
)

var (
	budgetAccessRoles      = []string{"app_budget_access"}
	complianceAccessRoles  = []string{"app_compliance_access"}
	kitProductsAccessRoles = []string{"app_kitproducts_access"}
	listiniAccessRoles     = []string{"app_listini_access"}
	panoramicaAccessRoles  = []string{"app_panoramica_access"}
	quotesAccessRoles      = []string{"app_quotes_access"}
	quotesDeleteRoles      = []string{"app_quotes_delete"}
	rdfBackendAccessRoles  = []string{"app_rdf_backend_access"}
	reportsAccessRoles     = []string{"app_reports_access"}
	defaultAccessRoles     = []string{"no-default-roles-cdlan"}
)

type Definition struct {
	ID            string
	Name          string
	Description   string
	Icon          string
	Href          string
	Status        string
	CategoryID    string
	CategoryTitle string
	AccessRoles   []string
}

type App struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon"`
	Href        string `json:"href"`
	Status      string `json:"status,omitempty"`
}

type Category struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Apps  []App  `json:"apps"`
}

// Catalog returns the full app catalog. hrefOverrides maps app IDs to
// custom hrefs, primarily for split-server local development.
func Catalog(hrefOverrides map[string]string) []Definition {
	_ = DefaultAccessRoles // used by commented-out entries below

	defs := []Definition{
		// ── Acquisti ──
		{
			ID:            BudgetAppID,
			Name:          "Budget Management",
			Icon:          "coins",
			Href:          BudgetAppHref,
			Status:        "ready",
			CategoryID:    "acquisti",
			CategoryTitle: "Acquisti",
			AccessRoles:   BudgetAccessRoles(),
		},
		// {
		// 	ID:            "gestione-utenti",
		// 	Name:          "Gestione Utenti",
		// 	Icon:          "users",
		// 	Href:          "/apps/acquisti/gestione-utenti",
		// 	CategoryID:    "acquisti",
		// 	CategoryTitle: "Acquisti",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "rda-richieste-di-acquisto",
		// 	Name:          "RDA Richieste di Acquisto",
		// 	Icon:          "cart",
		// 	Href:          "/apps/acquisti/rda-richieste-di-acquisto",
		// 	CategoryID:    "acquisti",
		// 	CategoryTitle: "Acquisti",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "fornitori",
		// 	Name:          "Fornitori",
		// 	Icon:          "handshake",
		// 	Href:          "/apps/acquisti/fornitori",
		// 	CategoryID:    "acquisti",
		// 	CategoryTitle: "Acquisti",
		// 	AccessRoles:   defaultRoles,
		// },
		// ── MKT&Sales ──
		{
			ID:            KitProductsAppID,
			Name:          "Kit e Prodotti",
			Icon:          "package",
			Href:          KitProductsAppHref,
			Status:        "ready",
			CategoryID:    "mkt-sales",
			CategoryTitle: "MKT&Sales",
			AccessRoles:   KitProductsAccessRoles(),
		},
		{
			ID:            QuotesAppID,
			Name:          "Proposte",
			Icon:          "mail",
			Href:          QuotesAppHref,
			Status:        "ready",
			CategoryID:    "mkt-sales",
			CategoryTitle: "MKT&Sales",
			AccessRoles:   QuotesAccessRoles(),
		},
		// {
		// 	ID:            "richieste-fattibilita",
		// 	Name:          "Richieste Fattibilità",
		// 	Icon:          "clipboard",
		// 	Href:          "/apps/mkt-sales/richieste-fattibilita",
		// 	CategoryID:    "mkt-sales",
		// 	CategoryTitle: "MKT&Sales",
		// 	AccessRoles:   defaultRoles,
		// },
		{
			ID:            ListiniAppID,
			Name:          "Listini e Sconti",
			Icon:          "tag",
			Href:          ListiniAppHref,
			Status:        "ready",
			CategoryID:    "mkt-sales",
			CategoryTitle: "MKT&Sales",
			AccessRoles:   ListiniAccessRoles(),
		},
		// {
		// 	ID:            "ordini",
		// 	Name:          "Ordini",
		// 	Icon:          "document",
		// 	Href:          "/apps/mkt-sales/ordini",
		// 	CategoryID:    "mkt-sales",
		// 	CategoryTitle: "MKT&Sales",
		// 	AccessRoles:   defaultRoles,
		// },
		// ── SMART APPS ──
		{
			ID:            ReportsAppID,
			Name:          "Reports",
			Icon:          "chart",
			Href:          ReportsAppHref,
			Status:        "ready",
			CategoryID:    "smart-apps",
			CategoryTitle: "SMART APPS",
			AccessRoles:   ReportsAccessRoles(),
		},
		{
			ID:            PanoramicaAppID,
			Name:          "Panoramica cliente",
			Icon:          "folder",
			Href:          PanoramicaAppHref,
			Status:        "ready",
			CategoryID:    "smart-apps",
			CategoryTitle: "SMART APPS",
			AccessRoles:   PanoramicaAccessRoles(),
		},
		// {
		// 	ID:            "customer-portal",
		// 	Name:          "Customer Portal",
		// 	Icon:          "chat",
		// 	Href:          "/apps/smart-apps/customer-portal",
		// 	CategoryID:    "smart-apps",
		// 	CategoryTitle: "SMART APPS",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "zammu",
		// 	Name:          "Zammù",
		// 	Icon:          "spark",
		// 	Href:          "/apps/smart-apps/zammu",
		// 	CategoryID:    "smart-apps",
		// 	CategoryTitle: "SMART APPS",
		// 	AccessRoles:   defaultRoles,
		// },
		{
			ID:            ComplianceAppID,
			Name:          "Compliance",
			Icon:          "shield",
			Href:          ComplianceAppHref,
			Status:        "ready",
			CategoryID:    "smart-apps",
			CategoryTitle: "SMART APPS",
			AccessRoles:   ComplianceAccessRoles(),
		},
		{
			ID:            RDFBackendAppID,
			Name:          "RDF Backend",
			Icon:          "database",
			Href:          RDFBackendAppHref,
			Status:        "ready",
			CategoryID:    "provisioning",
			CategoryTitle: "Provisioning",
			AccessRoles:   RDFBackendAccessRoles(),
		},
		// {
		// 	ID:            "customer-portal-settings",
		// 	Name:          "Customer Portal settings - APPLICATIONS...",
		// 	Icon:          "settings",
		// 	Href:          "/apps/smart-apps/customer-portal-settings",
		// 	CategoryID:    "smart-apps",
		// 	CategoryTitle: "SMART APPS",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "nardini",
		// 	Name:          "Nardini",
		// 	Icon:          "briefcase",
		// 	Href:          "/apps/smart-apps/nardini",
		// 	CategoryID:    "smart-apps",
		// 	CategoryTitle: "SMART APPS",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "non-usare-app-di-test",
		// 	Name:          "NON usare - App di TEST",
		// 	Icon:          "spark",
		// 	Status:        "test",
		// 	Href:          "/apps/smart-apps/non-usare-app-di-test",
		// 	CategoryID:    "smart-apps",
		// 	CategoryTitle: "SMART APPS",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "afc-tools",
		// 	Name:          "AFC Tools",
		// 	Icon:          "settings",
		// 	Href:          "/apps/smart-apps/afc-tools",
		// 	CategoryID:    "smart-apps",
		// 	CategoryTitle: "SMART APPS",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "coperture",
		// 	Name:          "Coperture",
		// 	Icon:          "shield",
		// 	Href:          "/apps/smart-apps/coperture",
		// 	CategoryID:    "smart-apps",
		// 	CategoryTitle: "SMART APPS",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "fdc-playground",
		// 	Name:          "FDC_playground",
		// 	Icon:          "spark",
		// 	Status:        "test",
		// 	Href:          "/apps/smart-apps/fdc-playground",
		// 	CategoryID:    "smart-apps",
		// 	CategoryTitle: "SMART APPS",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "manutenzioni",
		// 	Name:          "Manutenzioni",
		// 	Icon:          "wrench",
		// 	Href:          "/apps/smart-apps/manutenzioni",
		// 	CategoryID:    "smart-apps",
		// 	CategoryTitle: "SMART APPS",
		// 	AccessRoles:   defaultRoles,
		// },
		// // ── Provisioning ──
		// {
		// 	ID:            "rdf-backend-strafatti",
		// 	Name:          "RDF Backend StraFatti",
		// 	Icon:          "database",
		// 	Href:          "/apps/provisioning/rdf-backend-strafatti",
		// 	CategoryID:    "provisioning",
		// 	CategoryTitle: "Provisioning",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "la-vendetta-di-timoo",
		// 	Name:          "La vendetta di Timoo",
		// 	Icon:          "briefcase",
		// 	Href:          "/apps/provisioning/la-vendetta-di-timoo",
		// 	CategoryID:    "provisioning",
		// 	CategoryTitle: "Provisioning",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "s3cchiate-di-storage",
		// 	Name:          "S3cchiate di storage",
		// 	Icon:          "launch",
		// 	Href:          "/apps/provisioning/s3cchiate-di-storage",
		// 	CategoryID:    "provisioning",
		// 	CategoryTitle: "Provisioning",
		// 	AccessRoles:   defaultRoles,
		// },
	}

	for i, d := range defs {
		if override, ok := hrefOverrides[d.ID]; ok {
			if v := strings.TrimSpace(override); v != "" {
				defs[i].Href = v
			}
		}
	}

	return defs
}

func BudgetAccessRoles() []string {
	return slices.Clone(budgetAccessRoles)
}

func ComplianceAccessRoles() []string {
	return slices.Clone(complianceAccessRoles)
}

func KitProductsAccessRoles() []string {
	return slices.Clone(kitProductsAccessRoles)
}

func ListiniAccessRoles() []string {
	return slices.Clone(listiniAccessRoles)
}

func PanoramicaAccessRoles() []string {
	return slices.Clone(panoramicaAccessRoles)
}

func QuotesAccessRoles() []string {
	return slices.Clone(quotesAccessRoles)
}

func QuotesDeleteRoles() []string {
	return slices.Clone(quotesDeleteRoles)
}

func RDFBackendAccessRoles() []string {
	return slices.Clone(rdfBackendAccessRoles)
}

func ReportsAccessRoles() []string {
	return slices.Clone(reportsAccessRoles)
}

func DefaultAccessRoles() []string {
	return slices.Clone(defaultAccessRoles)
}

func VisibleCategories(definitions []Definition, roles []string) []Category {
	categories := make([]Category, 0)
	categoryIdx := make(map[string]int)

	for _, definition := range definitions {
		if !hasAnyRole(roles, definition.AccessRoles) {
			continue
		}

		idx, ok := categoryIdx[definition.CategoryID]
		if !ok {
			categories = append(categories, Category{
				ID:    definition.CategoryID,
				Title: definition.CategoryTitle,
				Apps:  []App{},
			})
			idx = len(categories) - 1
			categoryIdx[definition.CategoryID] = idx
		}

		categories[idx].Apps = append(categories[idx].Apps, App{
			ID:          definition.ID,
			Name:        definition.Name,
			Description: definition.Description,
			Icon:        definition.Icon,
			Href:        definition.Href,
			Status:      definition.Status,
		})
	}

	return categories
}

func hasAnyRole(userRoles []string, requiredRoles []string) bool {
	return authz.HasAnyRole(userRoles, requiredRoles...)
}
