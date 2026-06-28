package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sciacco/atego/internal/qdrant"
)

var (
	exportCollection string
	exportOutputDir  string
	exportKeepRemote bool
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Esporta collection come snapshot Qdrant",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, log := mustConfig()
		ctx, cancel := signalCtx()
		defer cancel()

		store, err := qdrant.New(cfg, log)
		if err != nil {
			return err
		}
		defer store.Close()

		if err := os.MkdirAll(exportOutputDir, 0o755); err != nil {
			return fmt.Errorf("creazione output dir: %w", err)
		}

		var colls []string
		if exportCollection == "all" || exportCollection == "ateco" {
			colls = append(colls, cfg.CollectionName(CollectionAteco))
		}
		if exportCollection == "all" || exportCollection == "concepts" {
			colls = append(colls, cfg.CollectionName(CollectionConcepts))
		}

		baseURL := strings.TrimRight(cfg.QdrantURL, "/")
		for _, coll := range colls {
			if err := exportOne(ctx, cfg.QdrantAPIKey, baseURL, exportOutputDir, exportKeepRemote, store, log, coll); err != nil {
				return err
			}
		}

		fmt.Printf("\nSnapshots salvati in: %s\n", exportOutputDir)
		fmt.Println("Per importare su un'altra istanza Qdrant:")
		fmt.Printf("  atego import --collection %s\n", exportCollection)
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVar(&exportCollection, "collection", "all", "Quale collection esportare: all|ateco|concepts")
	exportCmd.Flags().StringVar(&exportOutputDir, "output-dir", "data/snapshots", "Directory di output per gli snapshot")
	exportCmd.Flags().BoolVar(&exportKeepRemote, "keep-remote", false, "Non eliminare gli snapshot remoti dopo il download")
}

func exportOne(
	ctx context.Context,
	apiKey, baseURL, outputDir string,
	keepRemote bool,
	store *qdrant.Store,
	log *slog.Logger,
	coll string,
) error {
	exists, err := store.CollectionExists(ctx, coll)
	if err != nil {
		return err
	}
	if !exists {
		log.Error("collection does not exist", "collection", coll)
		return nil
	}

	count, err := store.CountPoints(ctx, coll)
	if err != nil {
		return err
	}
	log.Info("collection points", "collection", coll, "count", count)

	snapshotName, err := store.CreateSnapshot(ctx, coll)
	if err != nil {
		return err
	}

	snapshotURL := fmt.Sprintf("%s/collections/%s/snapshots/%s", baseURL, coll, snapshotName)
	dest := filepath.Join(outputDir, coll+".snapshot")
	if err := downloadFile(ctx, snapshotURL, dest, apiKey, log); err != nil {
		return err
	}

	if !keepRemote {
		if err := store.DeleteSnapshot(ctx, coll, snapshotName); err != nil {
			log.Warn("cannot delete remote snapshot", "err", err)
		}
	}

	fmt.Printf("✅ %s: %d punti → %s\n", coll, count, dest)
	return nil
}

// downloadFile scarica un URL su file locale via streaming, con header api-key.
func downloadFile(ctx context.Context, url, dest, apiKey string, log *slog.Logger) error {
	log.Info("downloading", "url", url, "dest", filepath.Base(dest))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if apiKey != "" {
		req.Header.Set("api-key", apiKey)
	}
	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return err
	}
	log.Info("downloaded", "mb", fmt.Sprintf("%.1f", float64(n)/(1024*1024)))
	return nil
}
