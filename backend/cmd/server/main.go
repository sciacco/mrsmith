package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"

	"github.com/sciacco/mrsmith/internal/afctools"
	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/budget"
	"github.com/sciacco/mrsmith/internal/compliance"
	"github.com/sciacco/mrsmith/internal/coperture"
	"github.com/sciacco/mrsmith/internal/cpbackoffice"
	"github.com/sciacco/mrsmith/internal/energiadc"
	"github.com/sciacco/mrsmith/internal/fornitori"
	"github.com/sciacco/mrsmith/internal/kitproducts"
	"github.com/sciacco/mrsmith/internal/listini"
	"github.com/sciacco/mrsmith/internal/manutenzioni"
	"github.com/sciacco/mrsmith/internal/panoramica"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/config"
	"github.com/sciacco/mrsmith/internal/platform/database"
	"github.com/sciacco/mrsmith/internal/platform/health"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/hubspot"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openrouter"
	"github.com/sciacco/mrsmith/internal/platform/staticspa"
	"github.com/sciacco/mrsmith/internal/portal"
	"github.com/sciacco/mrsmith/internal/quotes"
	"github.com/sciacco/mrsmith/internal/rda"
	"github.com/sciacco/mrsmith/internal/rdf"
	"github.com/sciacco/mrsmith/internal/rdfbackend"
	"github.com/sciacco/mrsmith/internal/reports"
	"github.com/sciacco/mrsmith/internal/simulatorivendita"
	"github.com/sciacco/mrsmith/pkg/middleware"
)

func main() {
	cfg := config.Load()
	logger := logging.New(cfg.LogLevel)
	slog.SetDefault(logger)

	// Auth middleware
	var authMiddleware *auth.Middleware
	if os.Getenv("SKIP_KEYCLOAK") == "true" {
		logger.Warn("auth disabled via SKIP_KEYCLOAK", "component", "auth")
		authMiddleware = auth.NewNoopMiddleware()
	} else {
		var err error
		authMiddleware, err = auth.NewMiddleware(cfg.KeycloakIssuerURL)
		if err != nil {
			logger.Error("failed to initialize auth", "component", "auth", "error", err)
			os.Exit(1)
		}
	}

	mux := http.NewServeMux()
	var arakCli *arak.Client

	// Health probes (no auth)
	health.Register(mux)

	// Frontend config (no auth — needed before auth initializes)
	mux.HandleFunc("GET /config", func(w http.ResponseWriter, _ *http.Request) {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"keycloakUrl": cfg.KeycloakFrontendURL,
			"realm":       cfg.KeycloakFrontendRealm,
			"clientId":    cfg.KeycloakFrontendClientId,
			"arakEnabled": arakCli != nil,
		})
	})

	// Arak API client (optional — when configured, report handlers proxy to real API)
	if cfg.ArakBaseURL != "" && cfg.ArakServiceTokenURL != "" {
		arakCli = arak.New(arak.Config{
			BaseURL:      cfg.ArakBaseURL,
			TokenURL:     cfg.ArakServiceTokenURL,
			ClientID:     cfg.ArakServiceClientID,
			ClientSecret: cfg.ArakServiceSecret,
		})
		logger.Info("arak client configured", "component", "budget", "base_url", cfg.ArakBaseURL)
	}

	// Anisetta DB (compliance module)
	var anisettaDB *sql.DB
	if cfg.AnisettaDSN != "" {
		var err error
		anisettaDB, err = database.New(database.Config{Driver: "postgres", DSN: cfg.AnisettaDSN})
		if err != nil {
			logger.Error("failed to connect to anisetta", "component", "compliance", "error", err)
			os.Exit(1)
		}
		logger.Info("anisetta database connected", "component", "compliance")
	}

	// Mistra DB (kit products)
	var mistraDB *sql.DB
	if cfg.MistraDSN != "" {
		var err error
		mistraDB, err = database.New(database.Config{Driver: "postgres", DSN: cfg.MistraDSN})
		if err != nil {
			logger.Error("failed to connect to mistra", "component", "kitproducts", "error", err)
			os.Exit(1)
		}
		logger.Info("mistra database connected", "component", "kitproducts")
	}

	// Arak DB (fornitori module)
	var arakDB *sql.DB
	if cfg.ArakDSN != "" {
		var err error
		arakDB, err = database.New(database.Config{Driver: "postgres", DSN: cfg.ArakDSN})
		if err != nil {
			logger.Error("failed to connect to arak", "component", "fornitori", "error", err)
			os.Exit(1)
		}
		logger.Info("arak database connected", "component", "fornitori")
	}

	// Coperture DB
	var dbCoperture *sql.DB
	if cfg.DBCopertureDSN != "" {
		var err error
		dbCoperture, err = database.New(database.Config{Driver: "postgres", DSN: cfg.DBCopertureDSN})
		if err != nil {
			logger.Error("failed to connect to dbcoperture", "component", "coperture", "error", err)
			os.Exit(1)
		}
		logger.Info("dbcoperture database connected", "component", "coperture")
	}

	// Alyante ERP (optional)
	var alyanteDB *sql.DB
	if cfg.AlyanteDSN != "" {
		var err error
		alyanteDB, err = database.New(database.Config{Driver: "mssql", DSN: cfg.AlyanteDSN})
		if err != nil {
			logger.Error("failed to connect to alyante", "component", "kitproducts", "error", err)
			os.Exit(1)
		}
		logger.Info("alyante database connected", "component", "kitproducts")
	} else {
		logger.Info("Alyante ERP adapter not configured", "component", "kitproducts")
	}

	// Grappa DB (listini module — MySQL)
	var grappaDB *sql.DB
	if cfg.GrappaDSN != "" {
		var err error
		grappaDB, err = database.New(database.Config{Driver: "mysql", DSN: cfg.GrappaDSN})
		if err != nil {
			logger.Error("failed to connect to grappa", "component", "listini", "error", err)
			os.Exit(1)
		}
		logger.Info("grappa database connected", "component", "listini")
	}

	// Manutenzioni DB
	var manutenzioniDB *sql.DB
	if cfg.ManutenzioniDSN != "" {
		var err error
		manutenzioniDB, err = database.New(database.Config{Driver: "postgres", DSN: cfg.ManutenzioniDSN})
		if err != nil {
			logger.Error("failed to connect to manutenzioni", "component", "manutenzioni", "error", err)
			os.Exit(1)
		}
		logger.Info("manutenzioni database connected", "component", "manutenzioni")
	}

	// Vodka/daiquiri DB (afc-tools — MySQL, Sales/CRM orders)
	var vodkaDB *sql.DB
	if cfg.VodkaDSN != "" {
		var err error
		vodkaDB, err = database.New(database.Config{Driver: "mysql", DSN: cfg.VodkaDSN})
		if err != nil {
			logger.Error("failed to connect to vodka", "component", "afctools", "error", err)
			os.Exit(1)
		}
		logger.Info("vodka database connected", "component", "afctools")
	}

	// WHMCS (Prometeus) DB (afc-tools — MySQL, billing transactions + invoice feed)
	var whmcsDB *sql.DB
	if cfg.WhmcsDSN != "" {
		var err error
		whmcsDB, err = database.New(database.Config{Driver: "mysql", DSN: cfg.WhmcsDSN})
		if err != nil {
			logger.Error("failed to connect to whmcs", "component", "afctools", "error", err)
			os.Exit(1)
		}
		logger.Info("whmcs database connected", "component", "afctools")
	}

	// HubSpot service (optional — listini module)
	var hubspotSvc *listini.HubSpotService
	if cfg.HubSpotAPIKey != "" && grappaDB != nil && mistraDB != nil {
		hubspotSvc = listini.NewHubSpotService(grappaDB, mistraDB, cfg.HubSpotAPIKey)
		logger.Info("hubspot service configured", "component", "listini")
	} else {
		logger.Info("hubspot service not configured", "component", "listini")
	}

	// Shared HubSpot client (quotes module and future consumers)
	var hubspotCli *hubspot.Client
	if cfg.HubSpotAPIKey != "" {
		hubspotCli = hubspot.New(cfg.HubSpotAPIKey)
		logger.Info("shared hubspot client configured", "component", "hubspot")
	}

	var openrouterCli *openrouter.Client
	if cfg.OpenRouterAPIKey != "" {
		openrouterCli = openrouter.New(cfg.OpenRouterAPIKey)
		logger.Info("shared openrouter client configured", "component", "openrouter")
	}

	// Carbone service (optional — listini module)
	var carboneSvc *listini.CarboneService
	if cfg.CarboneAPIKey != "" {
		carboneSvc = listini.NewCarboneService(cfg.CarboneAPIKey, listini.DefaultKitTemplateID)
		logger.Info("carbone service configured", "component", "listini")
	}

	// Carbone service (optional — reports module)
	var reportsCarboneSvc *reports.CarboneService
	if cfg.CarboneAPIKey != "" {
		reportsCarboneSvc = reports.NewCarboneService(cfg.CarboneAPIKey)
		logger.Info("carbone service configured", "component", "reports")
	}

	var simulatoriVenditaCarboneSvc *simulatorivendita.CarboneService
	if cfg.CarboneAPIKey != "" {
		simulatoriVenditaCarboneSvc = simulatorivendita.NewCarboneService(
			cfg.CarboneAPIKey,
			cfg.SimulatoriVenditaIaaSTemplateID,
		)
		logger.Info("carbone service configured", "component", "simulatori-vendita")
	}

	var afcToolsCarboneSvc *afctools.CarboneService
	if cfg.CarboneAPIKey != "" {
		afcToolsCarboneSvc = afctools.NewCarboneService(
			cfg.CarboneAPIKey,
			cfg.CarboneAFCToolsTransazioniTemplateID,
		)
		logger.Info("carbone service configured", "component", "afctools")
	}

	// API routes (with auth)
	api := http.NewServeMux()
	hrefOverrides := map[string]string{}
	if cfg.BudgetAppURL != "" {
		hrefOverrides[applaunch.BudgetAppID] = cfg.BudgetAppURL
	} else if cfg.StaticDir == "" {
		// Local split-server development still launches Budget on its own Vite server.
		hrefOverrides[applaunch.BudgetAppID] = "http://localhost:5174"
	}
	if cfg.FornitoriAppURL != "" {
		hrefOverrides[applaunch.FornitoriAppID] = cfg.FornitoriAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.FornitoriAppID] = "http://localhost:5189"
	}
	if cfg.RDAAppURL != "" {
		hrefOverrides[applaunch.RDAAppID] = cfg.RDAAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.RDAAppID] = "http://localhost:5190"
	}
	if cfg.ComplianceAppURL != "" {
		hrefOverrides[applaunch.ComplianceAppID] = cfg.ComplianceAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.ComplianceAppID] = "http://localhost:5175"
	}
	if cfg.CopertureAppURL != "" {
		hrefOverrides[applaunch.CopertureAppID] = cfg.CopertureAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.CopertureAppID] = "http://localhost:5183"
	}
	if cfg.CPBackofficeAppURL != "" {
		hrefOverrides[applaunch.CPBackofficeAppID] = cfg.CPBackofficeAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.CPBackofficeAppID] = "http://localhost:5187"
	}
	if cfg.EnergiaDCAppURL != "" {
		hrefOverrides[applaunch.EnergiaDCAppID] = cfg.EnergiaDCAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.EnergiaDCAppID] = "http://localhost:5184"
	}
	if cfg.KitProductsAppURL != "" {
		hrefOverrides[applaunch.KitProductsAppID] = cfg.KitProductsAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.KitProductsAppID] = "http://localhost:5176"
	}
	if cfg.ListiniAppURL != "" {
		hrefOverrides[applaunch.ListiniAppID] = cfg.ListiniAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.ListiniAppID] = "http://localhost:5177"
	}
	if cfg.ManutenzioniAppURL != "" {
		hrefOverrides[applaunch.ManutenzioniAppID] = cfg.ManutenzioniAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.ManutenzioniAppID] = "http://localhost:5188"
	}
	if cfg.PanoramicaAppURL != "" {
		hrefOverrides[applaunch.PanoramicaAppID] = cfg.PanoramicaAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.PanoramicaAppID] = "http://localhost:5178"
	}
	if cfg.QuotesAppURL != "" {
		hrefOverrides[applaunch.QuotesAppID] = cfg.QuotesAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.QuotesAppID] = "http://localhost:5179"
	}
	if cfg.RichiesteFattibilitaAppURL != "" {
		hrefOverrides[applaunch.RichiesteFattibilitaAppID] = cfg.RichiesteFattibilitaAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.RichiesteFattibilitaAppID] = "http://localhost:5182"
	}
	if cfg.RDFBackendAppURL != "" {
		hrefOverrides[applaunch.RDFBackendAppID] = cfg.RDFBackendAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.RDFBackendAppID] = "http://localhost:5181"
	}
	if cfg.ReportsAppURL != "" {
		hrefOverrides[applaunch.ReportsAppID] = cfg.ReportsAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.ReportsAppID] = "http://localhost:5180"
	}
	if cfg.SimulatoriVenditaAppURL != "" {
		hrefOverrides[applaunch.SimulatoriVenditaAppID] = cfg.SimulatoriVenditaAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.SimulatoriVenditaAppID] = "http://localhost:5185"
	}
	if cfg.AFCToolsAppURL != "" {
		hrefOverrides[applaunch.AFCToolsAppID] = cfg.AFCToolsAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.AFCToolsAppID] = "http://localhost:5186"
	}
	appCatalog := applaunch.Catalog(hrefOverrides)
	{
		filtered := make([]applaunch.Definition, 0, len(appCatalog))
		for _, definition := range appCatalog {
			if definition.Status == "dev" && !cfg.IncludeDevApps {
				continue
			}
			if definition.ID == applaunch.CopertureAppID && cfg.DBCopertureDSN == "" {
				continue
			}
			if definition.ID == applaunch.CPBackofficeAppID && (arakCli == nil || cfg.MistraDSN == "") {
				continue
			}
			if definition.ID == applaunch.FornitoriAppID && (arakCli == nil || cfg.ArakDSN == "") {
				continue
			}
			if definition.ID == applaunch.RDAAppID && (arakCli == nil || arakDB == nil) {
				continue
			}
			if definition.ID == applaunch.EnergiaDCAppID && cfg.GrappaDSN == "" {
				continue
			}
			if definition.ID == applaunch.KitProductsAppID && cfg.MistraDSN == "" {
				continue
			}
			if definition.ID == applaunch.ListiniAppID && (cfg.MistraDSN == "" || cfg.GrappaDSN == "") {
				continue
			}
			if definition.ID == applaunch.ManutenzioniAppID && cfg.ManutenzioniDSN == "" {
				continue
			}
			if definition.ID == applaunch.PanoramicaAppID && cfg.MistraDSN == "" && cfg.GrappaDSN == "" && cfg.AnisettaDSN == "" {
				continue
			}
			if definition.ID == applaunch.QuotesAppID && cfg.MistraDSN == "" {
				continue
			}
			if definition.ID == applaunch.RichiesteFattibilitaAppID && (cfg.AnisettaDSN == "" || cfg.MistraDSN == "") {
				continue
			}
			if definition.ID == applaunch.RDFBackendAppID && cfg.AnisettaDSN == "" {
				continue
			}
			if definition.ID == applaunch.ReportsAppID && cfg.MistraDSN == "" && cfg.GrappaDSN == "" && cfg.AnisettaDSN == "" {
				continue
			}
			if definition.ID == applaunch.AFCToolsAppID &&
				cfg.VodkaDSN == "" && cfg.WhmcsDSN == "" &&
				cfg.MistraDSN == "" && cfg.GrappaDSN == "" && cfg.AlyanteDSN == "" {
				continue
			}
			filtered = append(filtered, definition)
		}
		appCatalog = filtered
	}
	portal.RegisterRoutes(api, appCatalog)
	budget.RegisterRoutes(api, arakCli)
	fornitori.RegisterRoutes(api, arakCli, arakDB, alyanteDB)
	rda.RegisterRoutes(api, rda.Deps{Arak: arakCli, ArakDB: arakDB, Logger: logger})
	compliance.RegisterRoutes(api, anisettaDB)
	coperture.RegisterRoutes(api, dbCoperture)
	cpbackoffice.RegisterRoutes(api, cpbackoffice.Deps{
		Arak:   arakCli,
		Mistra: mistraDB,
		Logger: logger,
	})
	energiadc.RegisterRoutes(api, grappaDB, energiadc.ModuleConfig{
		ExcludedCustomerIDs: cfg.EnergiaDCExcludedCustomerIDs,
	})
	var alyanteAdapter *kitproducts.AlyanteAdapter
	if alyanteDB != nil {
		alyanteAdapter = kitproducts.NewAlyanteAdapter(alyanteDB)
	}
	kitproducts.RegisterRoutes(api, mistraDB, alyanteAdapter, arakCli)
	listini.RegisterRoutes(api, mistraDB, grappaDB, hubspotSvc, carboneSvc)
	manutenzioni.RegisterRoutes(api, manutenzioni.Deps{
		Maintenance: manutenzioniDB,
		Mistra:      mistraDB,
		AI:          openrouterCli,
		Logger:      logger,
	})
	panoramica.RegisterRoutes(api, mistraDB, grappaDB, anisettaDB)
	quotes.RegisterRoutes(api, mistraDB, alyanteDB, hubspotCli)
	rdf.RegisterRoutes(api, anisettaDB, mistraDB, openrouterCli, cfg.RDFTeamsWebhookURL, cfg.RDFTeamsNotificationsEnabled)
	rdfbackend.RegisterRoutes(api, anisettaDB)
	reports.RegisterRoutes(api, mistraDB, grappaDB, anisettaDB, reportsCarboneSvc)
	simulatorivendita.RegisterRoutes(api, simulatoriVenditaCarboneSvc)
	afctools.RegisterRoutes(api, afctools.Deps{
		Vodka:   vodkaDB,
		Whmcs:   whmcsDB,
		Mistra:  mistraDB,
		Grappa:  grappaDB,
		Alyante: alyanteDB,
		Carbone: afcToolsCarboneSvc,
		Arak:    arakCli,
	})

	mux.Handle("/api/", middleware.Chain(
		http.StripPrefix("/api", api),
		middleware.Recover(logger),
		middleware.RequestID,
		middleware.CORS(cfg.CORSOrigins),
		middleware.AccessLog(logger),
		authMiddleware.Handler,
	))

	// Static files (production)
	if cfg.StaticDir != "" {
		mux.Handle("/", staticspa.New(cfg.StaticDir))
	}

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		logger.Info("server listening", "component", "http", "addr", ":"+cfg.Port)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("server exited unexpectedly", "component", "http", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "component", "http", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped", "component", "http")
}
