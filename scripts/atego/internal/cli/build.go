package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	qd "github.com/qdrant/go-client/qdrant"
	"github.com/spf13/cobra"

	"github.com/sciacco/atego/internal/config"
	"github.com/sciacco/atego/internal/embeddings"
	"github.com/sciacco/atego/internal/manifest"
	"github.com/sciacco/atego/internal/qdrant"
	src "github.com/sciacco/atego/internal/sources"
)

var (
	buildCollection string
	buildRecreate   bool
	buildDryRun     bool
	buildDataDir    string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Costruisce gli indici vettoriali su Qdrant",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, log := mustConfig()
		ctx, cancel := signalCtx()
		defer cancel()

		embedder := embeddings.New(cfg, log)
		store, err := qdrant.New(cfg, log)
		if err != nil {
			return err
		}
		defer store.Close()

		// 1. Probe dimensione
		dim, err := embedder.ProbeDimension(ctx)
		if err != nil {
			return err
		}

		var sourcesInfo []manifest.SourceInfo
		var collectionsUsed []string

		if buildCollection == "all" || buildCollection == "ateco" {
			if err := runBuildAteco(ctx, cfg, log, embedder, store, dim, buildRecreate, buildDryRun, &sourcesInfo, &collectionsUsed); err != nil {
				return err
			}
		}
		if buildCollection == "all" || buildCollection == "concepts" {
			if err := runBuildConcepts(ctx, cfg, log, embedder, store, dim, buildRecreate, buildDryRun, &sourcesInfo, &collectionsUsed); err != nil {
				return err
			}
		}

		// Scrivi manifest (mai in dry-run)
		if !buildDryRun {
			manifestPath := filepath.Join(resolveDataDir(), "manifest.json")
			m := manifest.New(cfg.EmbeddingModel, dim, cfg.EmbeddingAPIBase, sourcesInfo, collectionsUsed)
			if err := manifest.Write(manifestPath, m); err != nil {
				return err
			}
			log.Info("manifest written", "path", manifestPath)
		} else {
			log.Info("[DRY-RUN] manifest not written")
		}
		return nil
	},
}

func init() {
	buildCmd.Flags().StringVar(&buildCollection, "collection", "all", "Quale collection processare: all|ateco|concepts")
	buildCmd.Flags().BoolVar(&buildRecreate, "recreate", false, "Elimina e ricrea la collection prima di caricare")
	buildCmd.Flags().BoolVar(&buildDryRun, "dry-run", false, "Calcola tutto ma non scrive su Qdrant")
	buildCmd.Flags().StringVar(&buildDataDir, "data-dir", "data", "Directory con i file sorgente e destinazione del manifest")
}

// resolveDataDir restituisce la directory dati (default "data" se vuota).
func resolveDataDir() string {
	if buildDataDir == "" {
		return "data"
	}
	return buildDataDir
}

// runBuildAteco processa la collection ateco_ict.
func runBuildAteco(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
	embedder *embeddings.Client,
	store *qdrant.Store,
	dim int,
	recreate, dryRun bool,
	sourcesInfo *[]manifest.SourceInfo,
	collectionsUsed *[]string,
) error {
	path := filepath.Join(resolveDataDir(), "ateco_ict_kb.json")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("file sorgente ateco non trovato %s: %w", path, err)
	}
	nodes, fileHash, err := src.LoadAtecoNodes(path)
	if err != nil {
		return err
	}
	coll := cfg.CollectionName(CollectionAteco)

	if !dryRun {
		if err := store.EnsureCollection(ctx, coll, dim, recreate); err != nil {
			return err
		}
	}

	ids := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		ids[src.PointIDAteco(n.Codice)] = struct{}{}
	}
	existing := map[string]string{}
	if !dryRun {
		existing, err = store.GetExistingHashes(ctx, coll, ids)
		if err != nil {
			return err
		}
	}

	var textsToEmbed []string
	var idxToEmbed []int
	for i, n := range nodes {
		pid := src.PointIDAteco(n.Codice)
		h := src.SourceTextHash(n.EmbeddingText)
		if eh, ok := existing[pid]; ok && eh == h {
			continue
		}
		textsToEmbed = append(textsToEmbed, n.EmbeddingText)
		idxToEmbed = append(idxToEmbed, i)
	}
	log.Info("embedding ateco",
		"to_embed", len(textsToEmbed), "total", len(nodes),
		"skipped", len(nodes)-len(textsToEmbed))

	var points []*qd.PointStruct
	if len(textsToEmbed) > 0 {
		vectors, err := embedder.EmbedAll(ctx, textsToEmbed)
		if err != nil {
			return err
		}
		for j, orig := range idxToEmbed {
			n := nodes[orig]
			pid := src.PointIDAteco(n.Codice)
			h := src.SourceTextHash(n.EmbeddingText)
			points = append(points, makePoint(pid, vectors[j], atecoPayload(n, h)))
		}
	}

	switch {
	case len(points) == 0:
		log.Info("no new ateco points to upsert")
	case dryRun:
		log.Info("[DRY-RUN] would upsert points", "count", len(points), "collection", coll)
	default:
		if err := store.UpsertPoints(ctx, coll, points); err != nil {
			return err
		}
	}

	count := len(points)
	if !dryRun {
		count, err = store.CountPoints(ctx, coll)
		if err != nil {
			return err
		}
	} else {
		count = len(points) + len(existing)
	}
	log.Info("collection points", "collection", coll, "count", count)

	*sourcesInfo = append(*sourcesInfo, manifest.SourceInfo{
		Filename:     filepath.Base(path),
		Sha256:       fileHash,
		ElementCount: len(nodes),
	})
	*collectionsUsed = append(*collectionsUsed, coll)
	return nil
}

// runBuildConcepts processa la collection business_concepts.
func runBuildConcepts(
	ctx context.Context,
	cfg *config.Config,
	log *slog.Logger,
	embedder *embeddings.Client,
	store *qdrant.Store,
	dim int,
	recreate, dryRun bool,
	sourcesInfo *[]manifest.SourceInfo,
	collectionsUsed *[]string,
) error {
	path := filepath.Join(resolveDataDir(), "concept_index_source.json")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("file sorgente concepts non trovato %s: %w", path, err)
	}
	concepts, fileHash, err := src.LoadBusinessConcepts(path)
	if err != nil {
		return err
	}
	coll := cfg.CollectionName(CollectionConcepts)

	if !dryRun {
		if err := store.EnsureCollection(ctx, coll, dim, recreate); err != nil {
			return err
		}
	}

	ids := make(map[string]struct{}, len(concepts))
	for _, c := range concepts {
		ids[src.PointIDConcept(c.ID)] = struct{}{}
	}
	existing := map[string]string{}
	if !dryRun {
		existing, err = store.GetExistingHashes(ctx, coll, ids)
		if err != nil {
			return err
		}
	}

	var textsToEmbed []string
	var idxToEmbed []int
	for i, c := range concepts {
		pid := src.PointIDConcept(c.ID)
		h := src.SourceTextHash(c.EmbeddingText)
		if eh, ok := existing[pid]; ok && eh == h {
			continue
		}
		textsToEmbed = append(textsToEmbed, c.EmbeddingText)
		idxToEmbed = append(idxToEmbed, i)
	}
	log.Info("embedding concepts",
		"to_embed", len(textsToEmbed), "total", len(concepts),
		"skipped", len(concepts)-len(textsToEmbed))

	var points []*qd.PointStruct
	if len(textsToEmbed) > 0 {
		vectors, err := embedder.EmbedAll(ctx, textsToEmbed)
		if err != nil {
			return err
		}
		for j, orig := range idxToEmbed {
			c := concepts[orig]
			pid := src.PointIDConcept(c.ID)
			h := src.SourceTextHash(c.EmbeddingText)
			points = append(points, makePoint(pid, vectors[j], conceptPayload(c, h)))
		}
	}

	switch {
	case len(points) == 0:
		log.Info("no new concept points to upsert")
	case dryRun:
		log.Info("[DRY-RUN] would upsert points", "count", len(points), "collection", coll)
	default:
		if err := store.UpsertPoints(ctx, coll, points); err != nil {
			return err
		}
	}

	count := len(points)
	if !dryRun {
		count, err = store.CountPoints(ctx, coll)
		if err != nil {
			return err
		}
	} else {
		count = len(points) + len(existing)
	}
	log.Info("collection points", "collection", coll, "count", count)

	*sourcesInfo = append(*sourcesInfo, manifest.SourceInfo{
		Filename:     filepath.Base(path),
		Sha256:       fileHash,
		ElementCount: len(concepts),
	})
	*collectionsUsed = append(*collectionsUsed, coll)
	return nil
}

// makePoint costruisce un qd.PointStruct da id, vettore e payload.
func makePoint(id string, vector []float32, payload map[string]any) *qd.PointStruct {
	m, err := qd.TryValueMap(payload)
	if err != nil {
		// I payload sono costruiti da noi con tipi noti; un errore qui è un bug.
		panic(fmt.Sprintf("impossibile convertire payload per punto %s: %v", id, err))
	}
	return &qd.PointStruct{
		Id:      qd.NewIDUUID(id),
		Vectors: qd.NewVectors(vector...),
		Payload: m,
	}
}

// atecoPayload costruisce il payload Qdrant per un nodo ATECO.
func atecoPayload(n src.AtecoNode, textHash string) map[string]any {
	return map[string]any{
		"codice":           n.Codice,
		"titolo":           n.Titolo,
		"gerarchia":        n.Gerarchia,
		"relevance_tier":   n.RelevanceTier,
		"classe_4cifre":    n.Classe4Cifre,
		"keywords":         toAnySlice(n.Keywords),
		"seed_concepts":    toAnySlice(n.SeedConcepts),
		"path_text":        n.PathText,
		"source_text_hash": textHash,
	}
}

// conceptPayload costruisce il payload Qdrant per un concetto.
func conceptPayload(c src.BusinessConcept, textHash string) map[string]any {
	return map[string]any{
		"id":                        c.ID,
		"name":                      c.Name,
		"domain":                    c.Domain,
		"kind":                      c.Kind,
		"aliases":                   toAnySlice(c.Aliases),
		"ateco_candidates_in_kb":    toAnySlice(c.AtecoCandidatesInKB),
		"ateco_candidates_excluded": toAnySlice(c.AtecoCandidatesExcluded),
		"source_text_hash":          textHash,
	}
}

// toAnySlice converte []string in []any (richiesto da qd.TryValueMap).
func toAnySlice(s []string) []any {
	if s == nil {
		return []any{}
	}
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}
