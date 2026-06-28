// Package cli implementa la CLI cobra di atego.
// È la traduzione di src/ingest/__main__.py.
package cli

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sciacco/atego/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "atego",
	Short:         "Build-time ingestion della KB su Postgres (curatela + embeddings)",
	Long:          "atego — sincronizza la KB binocolo (concetti + nodi ATECO ICT) su Postgres: upsert curatela dalla JSON sorgente + embeddings nelle colonne real[]. Coseno calcolato a runtime in Go (niente Qdrant/pgvector).",
	SilenceUsage:  true,
	SilenceErrors: true,
}

var verbose bool

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Logging verbose (DEBUG)")
}

// Execute avvia la CLI.
func Execute() error {
	rootCmd.AddCommand(probeCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(smokeCmd)
	return rootCmd.Execute()
}

// logger restituisce uno slog.Logger con livello in base a --verbose.
func logger() *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(h)
}

// mustConfig carica la configurazione; esce con errore chiaro se invalida.
func mustConfig() (*config.Config, *slog.Logger) {
	log := logger()
	cfg, err := config.Load("")
	if err != nil {
		slog.New(slog.NewTextHandler(os.Stderr, nil)).Error(err.Error())
		os.Exit(1)
	}
	return cfg, log
}

// signalCtx restituisce un context cancellato su SIGINT/SIGTERM.
func signalCtx() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, cancel
}
