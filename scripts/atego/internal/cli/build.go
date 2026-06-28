package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sciacco/atego/internal/embeddings"
	"github.com/sciacco/atego/internal/sources"
	"github.com/sciacco/atego/internal/store"
)

var (
	buildCollection string
	buildForce      bool
	buildDryRun     bool
	buildDataDir    string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Sincronizza la KB su Postgres (curatela + embeddings)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, log := mustConfig()
		ctx, cancel := signalCtx()
		defer cancel()

		embedder := embeddings.New(cfg, log)
		st, err := store.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer st.Close()

		// Risolvi il modello di embedding dal registry e verifica la dimensione.
		modelID, dim, err := st.ResolveEmbeddingModel(ctx, cfg.EmbeddingModel)
		if err != nil {
			return err
		}
		probed, err := embedder.ProbeDimension(ctx)
		if err != nil {
			return err
		}
		if probed != dim {
			return fmt.Errorf("il modello %q produce vettori di dimensione %d ma mrsmith.embedding_model dichiara %d", cfg.EmbeddingModel, probed, dim)
		}
		log.Info("embedding model resolved", "model", cfg.EmbeddingModel, "dimension", dim, "model_id", modelID)

		if buildCollection == "all" || buildCollection == "concepts" {
			if err := runBuildConcepts(ctx, log, embedder, st, modelID); err != nil {
				return err
			}
		}
		if buildCollection == "all" || buildCollection == "ateco" {
			if err := runBuildAteco(ctx, log, embedder, st, modelID); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	buildCmd.Flags().StringVar(&buildCollection, "collection", "all", "Quale set processare: all|ateco|concepts")
	buildCmd.Flags().BoolVar(&buildForce, "force", false, "Ri-embedda tutto, ignorando source_text_hash (usare al cambio di modello)")
	buildCmd.Flags().BoolVar(&buildDryRun, "dry-run", false, "Calcola tutto ma non scrive su Postgres")
	buildCmd.Flags().StringVar(&buildDataDir, "data-dir", "data", "Directory con i file sorgente JSON")
}

func resolveDataDir() string {
	if buildDataDir == "" {
		return "data"
	}
	return buildDataDir
}

// runBuildConcepts sincronizza binocolo.kb_concept (+ mapping fit).
func runBuildConcepts(ctx context.Context, log *slog.Logger, embedder *embeddings.Client, st *store.Store, modelID string) error {
	path := filepath.Join(resolveDataDir(), "concept_index_source.json")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("file sorgente concepts non trovato %s: %w", path, err)
	}
	concepts, _, err := sources.LoadBusinessConcepts(path)
	if err != nil {
		return err
	}

	existing := map[string]string{}
	if !buildForce {
		existing, err = st.ConceptHashes(ctx)
		if err != nil {
			return err
		}
	}

	var textsToEmbed []string
	var idsToEmbed []string
	for _, c := range concepts {
		h := sources.SourceTextHash(c.EmbeddingText)
		if eh, ok := existing[c.ID]; ok && eh == h {
			continue
		}
		textsToEmbed = append(textsToEmbed, c.EmbeddingText)
		idsToEmbed = append(idsToEmbed, c.ID)
	}
	log.Info("embedding concepts", "to_embed", len(textsToEmbed), "total", len(concepts), "skipped", len(concepts)-len(textsToEmbed))

	vectors := map[string][]float32{}
	if len(textsToEmbed) > 0 {
		vecs, err := embedder.EmbedAll(ctx, textsToEmbed)
		if err != nil {
			return err
		}
		for i, id := range idsToEmbed {
			vectors[id] = vecs[i]
		}
	}

	if buildDryRun {
		log.Info("[DRY-RUN] concepts non scritti", "would_upsert", len(concepts), "would_embed", len(vectors))
		return nil
	}
	if err := st.SyncConcepts(ctx, concepts, vectors, modelID); err != nil {
		return err
	}
	log.Info("concepts synced", "upserted", len(concepts), "embedded", len(vectors))
	return nil
}

// runBuildAteco sincronizza binocolo.kb_ateco_node.
func runBuildAteco(ctx context.Context, log *slog.Logger, embedder *embeddings.Client, st *store.Store, modelID string) error {
	path := filepath.Join(resolveDataDir(), "ateco_ict_kb.json")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("file sorgente ateco non trovato %s: %w", path, err)
	}
	nodes, _, err := sources.LoadAtecoNodes(path)
	if err != nil {
		return err
	}

	existing := map[string]string{}
	if !buildForce {
		existing, err = st.AtecoHashes(ctx)
		if err != nil {
			return err
		}
	}

	var textsToEmbed []string
	var idsToEmbed []string
	for _, n := range nodes {
		h := sources.SourceTextHash(n.EmbeddingText)
		if eh, ok := existing[n.Codice]; ok && eh == h {
			continue
		}
		textsToEmbed = append(textsToEmbed, n.EmbeddingText)
		idsToEmbed = append(idsToEmbed, n.Codice)
	}
	log.Info("embedding ateco", "to_embed", len(textsToEmbed), "total", len(nodes), "skipped", len(nodes)-len(textsToEmbed))

	vectors := map[string][]float32{}
	if len(textsToEmbed) > 0 {
		vecs, err := embedder.EmbedAll(ctx, textsToEmbed)
		if err != nil {
			return err
		}
		for i, id := range idsToEmbed {
			vectors[id] = vecs[i]
		}
	}

	if buildDryRun {
		log.Info("[DRY-RUN] ateco non scritti", "would_upsert", len(nodes), "would_embed", len(vectors))
		return nil
	}
	if err := st.SyncAteco(ctx, nodes, vectors, modelID); err != nil {
		return err
	}
	log.Info("ateco synced", "upserted", len(nodes), "embedded", len(vectors))
	return nil
}
