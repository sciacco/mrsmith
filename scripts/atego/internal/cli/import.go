package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sciacco/atego/internal/qdrant"
)

var (
	importCollection string
	importSnapshots  string
	importRecreate   bool
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Importa collection da snapshot Qdrant",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, log := mustConfig()
		ctx, cancel := signalCtx()
		defer cancel()

		store, err := qdrant.New(cfg, log)
		if err != nil {
			return err
		}
		defer store.Close()

		var colls []string
		if importCollection == "all" || importCollection == "ateco" {
			colls = append(colls, cfg.CollectionName(CollectionAteco))
		}
		if importCollection == "all" || importCollection == "concepts" {
			colls = append(colls, cfg.CollectionName(CollectionConcepts))
		}

		for _, coll := range colls {
			if err := importOne(ctx, store, log, coll); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	importCmd.Flags().StringVar(&importCollection, "collection", "all", "Quale collection importare: all|ateco|concepts")
	importCmd.Flags().StringVar(&importSnapshots, "snapshots-dir", "data/snapshots", "Directory con gli snapshot")
	importCmd.Flags().BoolVar(&importRecreate, "recreate", false, "Elimina la collection esistente prima di ripristinare")
}

func importOne(ctx context.Context, store *qdrant.Store, log *slog.Logger, coll string) error {
	snapshotFile := filepath.Join(importSnapshots, coll+".snapshot")
	if _, err := os.Stat(snapshotFile); err != nil {
		log.Warn("snapshot file not found", "path", snapshotFile)
		fmt.Printf("⚠️  %s: file snapshot non trovato in %s/\n", coll, importSnapshots)
		fmt.Printf("   Atteso: %s\n", snapshotFile)
		return nil
	}

	exists, err := store.CollectionExists(ctx, coll)
	if err != nil {
		return err
	}
	if exists {
		if importRecreate {
			log.Info("deleting existing collection", "collection", coll)
			if err := store.DeleteCollection(ctx, coll); err != nil {
				return err
			}
		} else {
			log.Error("collection already exists; use --recreate", "collection", coll)
			fmt.Printf("⚠️  %s: esiste già (usa --recreate per sovrascrivere)\n", coll)
			return nil
		}
	}

	if err := store.RestoreFromLocalFile(ctx, coll, snapshotFile); err != nil {
		return err
	}

	count, err := store.CountPoints(ctx, coll)
	if err != nil {
		return err
	}
	fmt.Printf("✅ %s: ripristinata da %s (%d punti)\n", coll, filepath.Base(snapshotFile), count)
	return nil
}
