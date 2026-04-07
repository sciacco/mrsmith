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

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/budget"
	"github.com/sciacco/mrsmith/internal/compliance"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/config"
	"github.com/sciacco/mrsmith/internal/platform/database"
	"github.com/sciacco/mrsmith/internal/platform/health"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/staticspa"
	"github.com/sciacco/mrsmith/internal/portal"
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

	// Health probes (no auth)
	health.Register(mux)

	// Frontend config (no auth — needed before auth initializes)
	mux.HandleFunc("GET /config", func(w http.ResponseWriter, _ *http.Request) {
		httputil.JSON(w, http.StatusOK, map[string]string{
			"keycloakUrl": cfg.KeycloakFrontendURL,
			"realm":       cfg.KeycloakFrontendRealm,
			"clientId":    cfg.KeycloakFrontendClientId,
		})
	})

	// Arak API client (optional — when configured, report handlers proxy to real API)
	var arakCli *arak.Client
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

	// API routes (with auth)
	api := http.NewServeMux()
	hrefOverrides := map[string]string{}
	if cfg.BudgetAppURL != "" {
		hrefOverrides[applaunch.BudgetAppID] = cfg.BudgetAppURL
	} else if cfg.StaticDir == "" {
		// Local split-server development still launches Budget on its own Vite server.
		hrefOverrides[applaunch.BudgetAppID] = "http://localhost:5174"
	}
	if cfg.ComplianceAppURL != "" {
		hrefOverrides[applaunch.ComplianceAppID] = cfg.ComplianceAppURL
	} else if cfg.StaticDir == "" {
		hrefOverrides[applaunch.ComplianceAppID] = "http://localhost:5175"
	}
	appCatalog := applaunch.Catalog(hrefOverrides)
	portal.RegisterRoutes(api, appCatalog)
	budget.RegisterRoutes(api, arakCli)
	compliance.RegisterRoutes(api, anisettaDB)

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
		WriteTimeout: 15 * time.Second,
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
