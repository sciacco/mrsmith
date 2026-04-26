package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/afctools"
	"github.com/sciacco/mrsmith/internal/simulatorivendita"
)

type Config struct {
	Port              string
	LogLevel          string
	KeycloakIssuerURL string
	CORSOrigins       string
	StaticDir         string

	// Optional launcher override for split-server local development.
	BudgetAppURL               string
	FornitoriAppURL            string
	RDAAppURL                  string
	ComplianceAppURL           string
	CopertureAppURL            string
	CPBackofficeAppURL         string
	EnergiaDCAppURL            string
	KitProductsAppURL          string
	ListiniAppURL              string
	ManutenzioniAppURL         string
	PanoramicaAppURL           string
	QuotesAppURL               string
	RichiesteFattibilitaAppURL string
	RDFBackendAppURL           string
	ReportsAppURL              string
	SimulatoriVenditaAppURL    string
	AFCToolsAppURL             string

	// Anisetta PostgreSQL (compliance module)
	AnisettaDSN string

	// Mistra / Kit Products PostgreSQL
	MistraDSN string

	// Arak PostgreSQL (provider qualifications and articles)
	ArakDSN string

	// Manutenzioni PostgreSQL
	ManutenzioniDSN string

	// Alyante ERP MSSQL
	AlyanteDSN string

	// Grappa MySQL (listini module)
	GrappaDSN string

	// Vodka/daiquiri MySQL (AFC Tools — Sales/CRM orders DB)
	VodkaDSN string

	// WHMCS (Prometeus) MySQL (AFC Tools — billing transactions and invoice feed)
	WhmcsDSN string

	// Energia in DC exclusions for the "Senza variabile" flow.
	EnergiaDCExcludedCustomerIDs []int

	// Coperture PostgreSQL
	DBCopertureDSN string

	// HubSpot integration (optional — listini module)
	HubSpotAPIKey string

	// Carbone PDF generation (optional — listini and simulatori modules)
	CarboneAPIKey string
	// Simulatori di Vendita Carbone template override.
	SimulatoriVenditaIaaSTemplateID string
	// AFC Tools — Transazioni WHMCS Carbone template id.
	CarboneAFCToolsTransazioniTemplateID string

	// OpenRouter AI integration (optional)
	OpenRouterAPIKey string

	// RDF Teams notifications (optional)
	RDFTeamsWebhookURL           string
	RDFTeamsNotificationsEnabled bool

	// Frontend Keycloak (public client, no secret — served to browser via GET /config)
	KeycloakFrontendURL      string
	KeycloakFrontendRealm    string
	KeycloakFrontendClientId string

	// Arak / MISTRA-NG API proxy (service account client credentials)
	ArakBaseURL         string
	ArakServiceClientID string
	ArakServiceSecret   string
	ArakServiceTokenURL string
}

func Load() Config {
	return Config{
		Port:                         envOr("PORT", "8080"),
		LogLevel:                     envOr("LOG_LEVEL", "info"),
		KeycloakIssuerURL:            envOr("KEYCLOAK_ISSUER_URL", ""),
		CORSOrigins:                  envOr("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174,http://localhost:5175,http://localhost:5176,http://localhost:5177,http://localhost:5178,http://localhost:5179,http://localhost:5180,http://localhost:5181,http://localhost:5182,http://localhost:5183,http://localhost:5184,http://localhost:5185,http://localhost:5186,http://localhost:5187,http://localhost:5188,http://localhost:5189,http://localhost:5190"),
		StaticDir:                    envOr("STATIC_DIR", ""),
		BudgetAppURL:                 envOr("BUDGET_APP_URL", ""),
		FornitoriAppURL:              envOr("FORNITORI_APP_URL", ""),
		RDAAppURL:                    envOr("RDA_APP_URL", ""),
		ComplianceAppURL:             envOr("COMPLIANCE_APP_URL", ""),
		CopertureAppURL:              envOr("COPERTURE_APP_URL", ""),
		CPBackofficeAppURL:           envOr("CP_BACKOFFICE_APP_URL", ""),
		EnergiaDCAppURL:              envOr("ENERGIA_DC_APP_URL", ""),
		KitProductsAppURL:            envOr("KIT_PRODUCTS_APP_URL", ""),
		ListiniAppURL:                envOr("LISTINI_APP_URL", ""),
		ManutenzioniAppURL:           envOr("MANUTENZIONI_APP_URL", ""),
		PanoramicaAppURL:             envOr("PANORAMICA_APP_URL", ""),
		QuotesAppURL:                 envOr("QUOTES_APP_URL", ""),
		RichiesteFattibilitaAppURL:   envOr("RICHIESTE_FATTIBILITA_APP_URL", ""),
		RDFBackendAppURL:             envOr("RDF_BACKEND_APP_URL", ""),
		ReportsAppURL:                envOr("REPORTS_APP_URL", ""),
		SimulatoriVenditaAppURL:      envOr("SIMULATORI_VENDITA_APP_URL", ""),
		AFCToolsAppURL:               envOr("AFCTOOLS_APP_URL", ""),
		AnisettaDSN:                  envOr("ANISETTA_DSN", ""),
		MistraDSN:                    envOr("MISTRA_DSN", ""),
		ArakDSN:                      envOr("ARAK_DSN", ""),
		ManutenzioniDSN:              envOr("MANUTENZIONI_DSN", ""),
		AlyanteDSN:                   envOr("ALYANTE_DSN", ""),
		GrappaDSN:                    envOr("GRAPPA_DSN", ""),
		VodkaDSN:                     envOr("VODKA_DSN", ""),
		WhmcsDSN:                     envOr("WHMCS_DSN", ""),
		EnergiaDCExcludedCustomerIDs: intListEnvOr("ENERGIA_DC_EXCLUDED_CUSTOMER_IDS", []int{3}),
		DBCopertureDSN:               envOr("DBCOPERTURE_DSN", ""),
		HubSpotAPIKey:                envOr("HUBSPOT_API_KEY", ""),
		CarboneAPIKey:                envOr("CARBONE_API_KEY", ""),
		SimulatoriVenditaIaaSTemplateID: envOr(
			"SIMULATORI_VENDITA_IAAS_TEMPLATE_ID",
			simulatorivendita.DefaultIaaSTemplateID,
		),
		CarboneAFCToolsTransazioniTemplateID: envOr(
			"CARBONE_AFCTOOLS_TRANSAZIONI_TEMPLATE_ID",
			afctools.DefaultTransazioniTemplateID,
		),
		OpenRouterAPIKey:             envOr("OPENROUTER_API_KEY", ""),
		RDFTeamsWebhookURL:           envOr("RDF_TEAMS_WEBHOOK_URL", ""),
		RDFTeamsNotificationsEnabled: boolEnvOr("RDF_TEAMS_NOTIFICATIONS_ENABLED", false),

		KeycloakFrontendURL:      envOr("KEYCLOAK_FRONTEND_URL", ""),
		KeycloakFrontendRealm:    envOr("KEYCLOAK_FRONTEND_REALM", ""),
		KeycloakFrontendClientId: envOr("KEYCLOAK_FRONTEND_CLIENT_ID", ""),

		ArakBaseURL:         envOr("ARAK_BASE_URL", ""),
		ArakServiceClientID: envOr("ARAK_SERVICE_CLIENT_ID", ""),
		ArakServiceSecret:   envOr("ARAK_SERVICE_CLIENT_SECRET", ""),
		ArakServiceTokenURL: envOr("ARAK_SERVICE_TOKEN_URL", ""),
	}
}

func boolEnvOr(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func intListEnvOr(key string, fallback []int) []int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return append([]int(nil), fallback...)
	}

	result := make([]int, 0)
	for _, raw := range strings.Split(value, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return append([]int(nil), fallback...)
		}
		result = append(result, parsed)
	}
	if len(result) == 0 {
		return append([]int(nil), fallback...)
	}
	return result
}
