package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/budget"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/config"
	"github.com/sciacco/mrsmith/internal/platform/health"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/portal"
	"github.com/sciacco/mrsmith/pkg/middleware"
)

func main() {
	cfg := config.Load()

	// Auth middleware
	var authMiddleware *auth.Middleware
	if os.Getenv("SKIP_KEYCLOAK") == "true" {
		log.Println("WARNING: SKIP_KEYCLOAK=true — auth disabled, using fake John Doe user. DO NOT use in production.")
		authMiddleware = auth.NewNoopMiddleware()
	} else {
		var err error
		authMiddleware, err = auth.NewMiddleware(cfg.KeycloakIssuerURL)
		if err != nil {
			log.Fatalf("failed to init auth: %v", err)
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
		log.Println("Arak API client configured — report handlers will proxy to", cfg.ArakBaseURL)
	}

	// API routes (with auth)
	api := http.NewServeMux()
	appCatalog := applaunch.Catalog(map[string]string{
		applaunch.BudgetAppID: cfg.BudgetAppURL,
	})
	portal.RegisterRoutes(api, appCatalog)
	budget.RegisterRoutes(api, arakCli)

	mux.Handle("/api/", middleware.Chain(
		http.StripPrefix("/api", api),
		middleware.RequestID,
		middleware.CORS(cfg.CORSOrigins),
		authMiddleware.Handler,
	))

	// Static files (production)
	if cfg.StaticDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(cfg.StaticDir)))
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
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("server stopped")
}
