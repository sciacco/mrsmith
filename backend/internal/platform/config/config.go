package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/afctools"
	"github.com/sciacco/mrsmith/internal/simulatorivendita"
)

const DefaultRDAQuoteThreshold float64 = 3000

type Config struct {
	Port              string
	LogLevel          string
	KeycloakIssuerURL string
	CORSOrigins       string
	StaticDir         string

	// Include catalog entries marked Status: "dev" in the portal launcher.
	// Default false so production hides them automatically; dev environments
	// must opt in via INCLUDE_DEV_APPS=true.
	IncludeDevApps bool

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

	// SMTP email delivery (optional, disabled by default)
	SMTPEnabled       bool
	SMTPHost          string
	SMTPPort          string
	SMTPUsername      string
	SMTPPassword      string
	SMTPFrom          string
	SMTPTLSMode       string
	SMTPTLSSkipVerify bool
	SMTPTLSServerName string
	SMTPAuthMode      string

	// Notifications runtime (portal works without SMTP; email links need public base URL)
	MrSmithPublicBaseURL        string
	NotificationsWorkerEnabled  bool
	NotificationsWorkerInterval time.Duration
	NotifySelfMentions          bool

	// Diagnostic event sink (optional, backed by ANISETTA_DSN)
	DiagnosticEventsEnabled       bool
	DiagnosticEventsRetentionDays int
	DiagnosticEventsQueueSize     int

	// Frontend Keycloak (public client, no secret — served to browser via GET /config)
	KeycloakFrontendURL      string
	KeycloakFrontendRealm    string
	KeycloakFrontendClientId string

	// Backend-only Keycloak Admin API client credentials and endpoints. These
	// values are not exposed through GET /config.
	KeycloakAdminBaseURL      string
	KeycloakAdminRealm        string
	KeycloakAdminClientID     string
	KeycloakAdminClientSecret string
	KeycloakAdminTokenURL     string

	// Public RDA runtime settings served to browser via GET /config.
	RDAQuoteThreshold float64

	// Arak / MISTRA-NG API proxy (service account client credentials)
	ArakBaseURL         string
	ArakServiceClientID string
	ArakServiceSecret   string
	ArakServiceTokenURL string
}

func Load() Config {
	keycloakIssuerURL := envOr("KEYCLOAK_ISSUER_URL", "")
	keycloakFrontendURL := envOr("KEYCLOAK_FRONTEND_URL", "")
	keycloakFrontendRealm := envOr("KEYCLOAK_FRONTEND_REALM", "")

	keycloakAdminBaseURL := envOr(
		"KEYCLOAK_ADMIN_BASE_URL",
		deriveKeycloakAdminBaseURL(keycloakIssuerURL, keycloakFrontendURL),
	)
	keycloakAdminRealm := envOr(
		"KEYCLOAK_ADMIN_REALM",
		deriveKeycloakAdminRealm(keycloakIssuerURL, keycloakFrontendRealm),
	)
	keycloakAdminTokenURL := envOr(
		"KEYCLOAK_ADMIN_TOKEN_URL",
		deriveKeycloakAdminTokenURL(keycloakAdminBaseURL, keycloakAdminRealm, keycloakIssuerURL),
	)

	return Config{
		Port:                         envOr("PORT", "8080"),
		LogLevel:                     envOr("LOG_LEVEL", "info"),
		KeycloakIssuerURL:            keycloakIssuerURL,
		CORSOrigins:                  envOr("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174,http://localhost:5175,http://localhost:5176,http://localhost:5177,http://localhost:5178,http://localhost:5179,http://localhost:5180,http://localhost:5181,http://localhost:5182,http://localhost:5183,http://localhost:5184,http://localhost:5185,http://localhost:5186,http://localhost:5187,http://localhost:5188,http://localhost:5189,http://localhost:5190"),
		StaticDir:                    envOr("STATIC_DIR", ""),
		IncludeDevApps:               boolEnvOr("INCLUDE_DEV_APPS", false),
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
		SMTPEnabled:                  boolEnvOr("SMTP_ENABLED", false),
		SMTPHost:                     envOr("SMTP_HOST", ""),
		SMTPPort:                     envOr("SMTP_PORT", "587"),
		SMTPUsername:                 envOr("SMTP_USERNAME", ""),
		SMTPPassword:                 envOr("SMTP_PASSWORD", ""),
		SMTPFrom:                     envOr("SMTP_FROM", ""),
		SMTPTLSMode:                  envOr("SMTP_TLS_MODE", "auto"),
		SMTPTLSSkipVerify:            boolEnvOr("SMTP_TLS_SKIP_VERIFY", false),
		SMTPTLSServerName:            envOr("SMTP_TLS_SERVER_NAME", ""),
		SMTPAuthMode:                 envOr("SMTP_AUTH_MODE", "auto"),
		MrSmithPublicBaseURL:         envOr("MRSMITH_PUBLIC_BASE_URL", ""),
		NotificationsWorkerEnabled:   boolEnvOr("NOTIFICATIONS_WORKER_ENABLED", true),
		NotificationsWorkerInterval:  durationEnvOr("NOTIFICATIONS_WORKER_INTERVAL", time.Minute),
		NotifySelfMentions:           boolEnvOr("NOTIFY_SELF_MENTIONS", false),
		DiagnosticEventsEnabled:      boolEnvOr("DIAGNOSTIC_EVENTS_ENABLED", envOr("ANISETTA_DSN", "") != ""),
		DiagnosticEventsRetentionDays: positiveIntEnvOr(
			"DIAGNOSTIC_EVENTS_RETENTION_DAYS",
			90,
		),
		DiagnosticEventsQueueSize: positiveIntEnvOr("DIAGNOSTIC_EVENTS_QUEUE_SIZE", 1000),

		KeycloakFrontendURL:      keycloakFrontendURL,
		KeycloakFrontendRealm:    keycloakFrontendRealm,
		KeycloakFrontendClientId: envOr("KEYCLOAK_FRONTEND_CLIENT_ID", ""),
		KeycloakAdminBaseURL:     keycloakAdminBaseURL,
		KeycloakAdminRealm:       keycloakAdminRealm,
		KeycloakAdminClientID: envOr(
			"KEYCLOAK_ADMIN_CLIENT_ID",
			"",
		),
		KeycloakAdminClientSecret: envOr(
			"KEYCLOAK_ADMIN_CLIENT_SECRET",
			"",
		),
		KeycloakAdminTokenURL: keycloakAdminTokenURL,
		RDAQuoteThreshold:     positiveFloatEnvOr("RDA_QUOTE_THRESHOLD", DefaultRDAQuoteThreshold),

		ArakBaseURL:         envOr("ARAK_BASE_URL", ""),
		ArakServiceClientID: envOr("ARAK_SERVICE_CLIENT_ID", ""),
		ArakServiceSecret:   envOr("ARAK_SERVICE_CLIENT_SECRET", ""),
		ArakServiceTokenURL: envOr("ARAK_SERVICE_TOKEN_URL", ""),
	}
}

func durationEnvOr(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
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

func deriveKeycloakAdminBaseURL(issuerURL, frontendURL string) string {
	if strings.TrimSpace(frontendURL) != "" {
		return strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	}
	baseURL, _ := keycloakIssuerParts(issuerURL)
	return baseURL
}

func deriveKeycloakAdminRealm(issuerURL, frontendRealm string) string {
	if strings.TrimSpace(frontendRealm) != "" {
		return strings.TrimSpace(frontendRealm)
	}
	_, realm := keycloakIssuerParts(issuerURL)
	return realm
}

func deriveKeycloakAdminTokenURL(adminBaseURL, adminRealm, issuerURL string) string {
	if strings.TrimSpace(adminBaseURL) != "" && strings.TrimSpace(adminRealm) != "" {
		return joinURLPath(adminBaseURL, "realms", adminRealm, "protocol", "openid-connect", "token")
	}
	if strings.TrimSpace(issuerURL) == "" {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(issuerURL), "/") + "/protocol/openid-connect/token"
}

func keycloakIssuerParts(issuerURL string) (string, string) {
	parsed, err := url.Parse(strings.TrimSpace(issuerURL))
	if err != nil {
		return "", ""
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := len(parts) - 2; i >= 0; i-- {
		if parts[i] != "realms" {
			continue
		}

		realm, err := url.PathUnescape(parts[i+1])
		if err != nil {
			realm = parts[i+1]
		}
		parsed.Path = ""
		if i > 0 {
			parsed.Path = "/" + strings.Join(parts[:i], "/")
		}
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return strings.TrimRight(parsed.String(), "/"), realm
	}
	return "", ""
}

func joinURLPath(baseURL string, parts ...string) string {
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, url.PathEscape(strings.Trim(part, "/")))
	}
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/" + strings.Join(escaped, "/")
}

func positiveFloatEnvOr(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func positiveIntEnvOr(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
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
