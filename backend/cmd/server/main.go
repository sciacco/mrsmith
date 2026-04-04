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
	"github.com/sciacco/mrsmith/internal/platform/config"
	"github.com/sciacco/mrsmith/internal/platform/health"
	"github.com/sciacco/mrsmith/internal/portal"
	"github.com/sciacco/mrsmith/pkg/middleware"
)

func main() {
	cfg := config.Load()

	// Auth middleware
	authMiddleware, err := auth.NewMiddleware(cfg.KeycloakIssuerURL)
	if err != nil {
		log.Fatalf("failed to init auth: %v", err)
	}

	mux := http.NewServeMux()

	// Health probes (no auth)
	health.Register(mux)

	// API routes (with auth)
	api := http.NewServeMux()
	portal.RegisterRoutes(api)
	// Future: hr.RegisterRoutes(api), finance.RegisterRoutes(api), etc.

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
