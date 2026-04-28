package applaunch

import (
	"slices"
	"strings"

	"github.com/sciacco/mrsmith/internal/authz"
)

const (
	BudgetAppID   = "budget"
	BudgetAppHref = "/apps/budget/"

	FornitoriAppID   = "fornitori"
	FornitoriAppHref = "/apps/fornitori/"

	RDAAppID   = "rda"
	RDAAppHref = "/apps/rda/"

	ComplianceAppID   = "compliance"
	ComplianceAppHref = "/apps/compliance/"

	CopertureAppID   = "coperture"
	CopertureAppHref = "/apps/coperture/"

	CPBackofficeAppID   = "cp-backoffice"
	CPBackofficeAppHref = "/apps/cp-backoffice/"

	EnergiaDCAppID   = "energia-dc"
	EnergiaDCAppHref = "/apps/energia-dc/"

	KitProductsAppID   = "kit-e-prodotti"
	KitProductsAppHref = "/apps/kit-products/"

	ListiniAppID   = "listini-e-sconti"
	ListiniAppHref = "/apps/listini-e-sconti/"

	ManutenzioniAppID   = "manutenzioni"
	ManutenzioniAppHref = "/apps/manutenzioni/"

	PanoramicaAppID   = "panoramica-cliente"
	PanoramicaAppHref = "/apps/panoramica-cliente/"

	QuotesAppID   = "proposte"
	QuotesAppHref = "/apps/quotes/"

	SimulatoriVenditaAppID   = "simulatori-vendita"
	SimulatoriVenditaAppHref = "/apps/simulatori-vendita/"

	RichiesteFattibilitaAppID   = "richieste-fattibilita"
	RichiesteFattibilitaAppHref = "/apps/richieste-fattibilita/"

	RDFBackendAppID   = "rdf-backend"
	RDFBackendAppHref = "/apps/rdf-backend/"

	ReportsAppID   = "reports"
	ReportsAppHref = "/apps/reports/"

	AFCToolsAppID   = "afc-tools"
	AFCToolsAppHref = "/apps/afc-tools/"
)

var (
	budgetAccessRoles                = []string{"app_budget_access"}
	fornitoriAccessRoles             = []string{"app_fornitori_access"}
	fornitoriSkipQualificationRoles  = []string{"app_fornitori_skip_qualification"}
	rdaAccessRoles                   = []string{"app_rda_access"}
	rdaApproverL1L2Roles             = []string{"app_rda_approver_l1l2"}
	rdaApproverAFCRoles              = []string{"app_rda_approver_afc"}
	rdaApproverNoLeasingRoles        = []string{"app_rda_approver_no_leasing"}
	rdaApproverExtraBudgetRoles      = []string{"app_rda_approver_extra_budget"}
	complianceAccessRoles            = []string{"app_compliance_access"}
	copertureAccessRoles             = []string{"app_coperture_access"}
	cpBackofficeAccessRoles          = []string{"app_cpbackoffice_access"}
	energiaDCAccessRoles             = []string{"app_energiadc_access"}
	kitProductsAccessRoles           = []string{"app_kitproducts_access"}
	listiniAccessRoles               = []string{"app_listini_access"}
	manutenzioniAccessRoles          = []string{"app_manutenzioni_access"}
	manutenzioniManagerRoles         = []string{"app_manutenzioni_manager"}
	manutenzioniOperatorRoles        = []string{"app_manutenzioni_operator"}
	manutenzioniApproverRoles        = []string{"app_manutenzioni_approver"}
	panoramicaAccessRoles            = []string{"app_panoramica_access"}
	quotesAccessRoles                = []string{"app_quotes_access"}
	quotesDeleteRoles                = []string{"app_quotes_delete"}
	simulatoriVenditaAccessRoles     = []string{"app_simulatorivendita_access"}
	richiesteFattibilitaAccessRoles  = []string{"app_rdf_access", "app_rdf_manager"}
	richiesteFattibilitaManagerRoles = []string{"app_rdf_manager"}
	rdfBackendAccessRoles            = []string{"app_rdf_backend_access"}
	reportsAccessRoles               = []string{"app_reports_access"}
	afcToolsAccessRoles              = []string{"app_afctools_access"}
	defaultAccessRoles               = []string{"no-default-roles-cdlan"}
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
		{
			ID:            FornitoriAppID,
			Name:          "Fornitori",
			Icon:          "handshake",
			Href:          FornitoriAppHref,
			Status:        "ready",
			CategoryID:    "acquisti",
			CategoryTitle: "Acquisti",
			AccessRoles:   FornitoriAccessRoles(),
		},
		{
			ID:            RDAAppID,
			Name:          "RDA",
			Description:   "Richieste di acquisto e approvazioni.",
			Icon:          "cart",
			Href:          RDAAppHref,
			Status:        "ready",
			CategoryID:    "acquisti",
			CategoryTitle: "Acquisti",
			AccessRoles:   RDAAccessRoles(),
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
		{
			ID:            SimulatoriVenditaAppID,
			Name:          "Simulatori di Vendita",
			Icon:          "briefcase",
			Href:          SimulatoriVenditaAppHref,
			Status:        "ready",
			CategoryID:    "mkt-sales",
			CategoryTitle: "MKT&Sales",
			AccessRoles:   SimulatoriVenditaAccessRoles(),
		},
		{
			ID:            RichiesteFattibilitaAppID,
			Name:          "Richieste Fattibilita",
			Icon:          "file-text",
			Href:          RichiesteFattibilitaAppHref,
			Status:        "ready",
			CategoryID:    "mkt-sales",
			CategoryTitle: "MKT&Sales",
			AccessRoles:   RichiesteFattibilitaAccessRoles(),
		},
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
			ID:            CopertureAppID,
			Name:          "Coperture",
			Icon:          "shield",
			Href:          CopertureAppHref,
			Status:        "ready",
			CategoryID:    "smart-apps",
			CategoryTitle: "SMART APPS",
			AccessRoles:   CopertureAccessRoles(),
		},
		{
			ID:            EnergiaDCAppID,
			Name:          "Energia in DC",
			Icon:          "chart",
			Href:          EnergiaDCAppHref,
			Status:        "ready",
			CategoryID:    "tech",
			CategoryTitle: "TECH",
			AccessRoles:   EnergiaDCAccessRoles(),
		},
		{
			ID:            ManutenzioniAppID,
			Name:          "Manutenzioni",
			Icon:          "wrench",
			Href:          ManutenzioniAppHref,
			Status:        "dev",
			CategoryID:    "tech",
			CategoryTitle: "TECH",
			AccessRoles:   ManutenzioniAccessRoles(),
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
		// ── Backoffice ──
		{
			ID:            CPBackofficeAppID,
			Name:          "CP Backoffice",
			Description:   "Gestione aziende, utenti e accessi biometrico per il back-office clienti.",
			Icon:          "users",
			Href:          CPBackofficeAppHref,
			Status:        "ready",
			CategoryID:    "backoffice",
			CategoryTitle: "Backoffice",
			AccessRoles:   CPBackofficeAccessRoles(),
		},
		{
			ID:            RDFBackendAppID,
			Name:          "RDF Backend",
			Icon:          "database",
			Href:          RDFBackendAppHref,
			Status:        "ready",
			CategoryID:    "backoffice",
			CategoryTitle: "Backoffice",
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
		{
			ID:     AFCToolsAppID,
			Name:   "AFC Tools",
			Icon:   "settings",
			Href:   AFCToolsAppHref,
			Status: "ready",
			// CategoryID:    "afc",
			// CategoryTitle: "AFC",
			CategoryID:    "smart-apps",
			CategoryTitle: "SMART APPS",
			AccessRoles:   AFCToolsAccessRoles(),
		},
		// {
		// 	ID:            "manutenzioni",
		// 	Name:          "Manutenzioni",
		// 	Icon:          "wrench",
		// 	Href:          "/apps/smart-apps/manutenzioni",
		// 	CategoryID:    "smart-apps",
		// 	CategoryTitle: "SMART APPS",
		// 	AccessRoles:   defaultRoles,
		// },
		// // ── Backoffice ──
		// {
		// 	ID:            "rdf-backend-strafatti",
		// 	Name:          "RDF Backend StraFatti",
		// 	Icon:          "database",
		// 	Href:          "/apps/backoffice/rdf-backend-strafatti",
		// 	CategoryID:    "backoffice",
		// 	CategoryTitle: "Backoffice",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "la-vendetta-di-timoo",
		// 	Name:          "La vendetta di Timoo",
		// 	Icon:          "briefcase",
		// 	Href:          "/apps/backoffice/la-vendetta-di-timoo",
		// 	CategoryID:    "backoffice",
		// 	CategoryTitle: "Backoffice",
		// 	AccessRoles:   defaultRoles,
		// },
		// {
		// 	ID:            "s3cchiate-di-storage",
		// 	Name:          "S3cchiate di storage",
		// 	Icon:          "launch",
		// 	Href:          "/apps/backoffice/s3cchiate-di-storage",
		// 	CategoryID:    "backoffice",
		// 	CategoryTitle: "Backoffice",
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

func FornitoriAccessRoles() []string {
	return slices.Clone(fornitoriAccessRoles)
}

func FornitoriSkipQualificationRoles() []string {
	return slices.Clone(fornitoriSkipQualificationRoles)
}

func RDAAccessRoles() []string {
	return slices.Clone(rdaAccessRoles)
}

func RDAApproverL1L2Roles() []string {
	return slices.Clone(rdaApproverL1L2Roles)
}

func RDAApproverAFCRoles() []string {
	return slices.Clone(rdaApproverAFCRoles)
}

func RDAApproverNoLeasingRoles() []string {
	return slices.Clone(rdaApproverNoLeasingRoles)
}

func RDAApproverExtraBudgetRoles() []string {
	return slices.Clone(rdaApproverExtraBudgetRoles)
}

func ComplianceAccessRoles() []string {
	return slices.Clone(complianceAccessRoles)
}

func CopertureAccessRoles() []string {
	return slices.Clone(copertureAccessRoles)
}

func CPBackofficeAccessRoles() []string {
	return slices.Clone(cpBackofficeAccessRoles)
}

func EnergiaDCAccessRoles() []string {
	return slices.Clone(energiaDCAccessRoles)
}

func KitProductsAccessRoles() []string {
	return slices.Clone(kitProductsAccessRoles)
}

func ListiniAccessRoles() []string {
	return slices.Clone(listiniAccessRoles)
}

func ManutenzioniAccessRoles() []string {
	return slices.Clone(manutenzioniAccessRoles)
}

func ManutenzioniManagerRoles() []string {
	return slices.Clone(manutenzioniManagerRoles)
}

func ManutenzioniOperatorRoles() []string {
	return slices.Clone(manutenzioniOperatorRoles)
}

func ManutenzioniApproverRoles() []string {
	return slices.Clone(manutenzioniApproverRoles)
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

func SimulatoriVenditaAccessRoles() []string {
	return slices.Clone(simulatoriVenditaAccessRoles)
}

func RDFBackendAccessRoles() []string {
	return slices.Clone(rdfBackendAccessRoles)
}

func RichiesteFattibilitaAccessRoles() []string {
	return slices.Clone(richiesteFattibilitaAccessRoles)
}

func RichiesteFattibilitaManagerRoles() []string {
	return slices.Clone(richiesteFattibilitaManagerRoles)
}

func ReportsAccessRoles() []string {
	return slices.Clone(reportsAccessRoles)
}

func AFCToolsAccessRoles() []string {
	return slices.Clone(afcToolsAccessRoles)
}

func DefaultAccessRoles() []string {
	return slices.Clone(defaultAccessRoles)
}

// AllRoles returns the concatenation of every known app_* role declared
// in the catalog, deduplicated. Intended for dev-only scenarios (noop auth
// middleware, dev auth bypass on the frontend) where the caller needs to
// simulate an omnipotent user.
func AllRoles() []string {
	groups := [][]string{
		budgetAccessRoles,
		fornitoriAccessRoles,
		fornitoriSkipQualificationRoles,
		rdaAccessRoles,
		rdaApproverL1L2Roles,
		rdaApproverAFCRoles,
		rdaApproverNoLeasingRoles,
		rdaApproverExtraBudgetRoles,
		complianceAccessRoles,
		copertureAccessRoles,
		cpBackofficeAccessRoles,
		energiaDCAccessRoles,
		kitProductsAccessRoles,
		listiniAccessRoles,
		manutenzioniAccessRoles,
		manutenzioniManagerRoles,
		manutenzioniOperatorRoles,
		manutenzioniApproverRoles,
		panoramicaAccessRoles,
		quotesAccessRoles,
		quotesDeleteRoles,
		simulatoriVenditaAccessRoles,
		richiesteFattibilitaAccessRoles,
		richiesteFattibilitaManagerRoles,
		rdfBackendAccessRoles,
		reportsAccessRoles,
		afcToolsAccessRoles,
	}
	seen := map[string]struct{}{}
	result := make([]string, 0)
	for _, group := range groups {
		for _, role := range group {
			if _, ok := seen[role]; ok {
				continue
			}
			seen[role] = struct{}{}
			result = append(result, role)
		}
	}
	return result
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
