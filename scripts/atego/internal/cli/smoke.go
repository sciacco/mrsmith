package cli

import (
	"fmt"
	"strings"

	qd "github.com/qdrant/go-client/qdrant"
	"github.com/spf13/cobra"

	"github.com/sciacco/atego/internal/embeddings"
	"github.com/sciacco/atego/internal/qdrant"
)

var smokeCmd = &cobra.Command{
	Use:   "smoke [QUERY]",
	Short: "Query di prova sulla collection ateco",
	Args:  cobra.ExactArgs(1),
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

		query := args[0]
		log.Info("embedding query", "query", query)
		vecs, err := embedder.EmbedBatch(ctx, []string{query})
		if err != nil {
			return err
		}
		queryVector := vecs[0]

		coll := cfg.CollectionName(CollectionAteco)
		results, err := store.Search(ctx, coll, queryVector, 5)
		if err != nil {
			return err
		}

		fmt.Printf("\nQuery: %s\n", query)
		fmt.Printf("Model: %s | Dimension: %d\n", cfg.EmbeddingModel, len(queryVector))
		fmt.Printf("\nTop-5 risultati su '%s':\n", coll)
		fmt.Printf("%8s  %-10s %s\n", "Score", "Codice", "Titolo")
		fmt.Println(strings.Repeat("-", 60))
		for _, r := range results {
			codice := payloadStr(r.GetPayload(), "codice")
			titolo := payloadStr(r.GetPayload(), "titolo")
			fmt.Printf("%8.4f  %-10s %s\n", r.GetScore(), codice, titolo)
		}
		return nil
	},
}

// payloadStr estrae un valore stringa dal payload Qdrant.
func payloadStr(payload map[string]*qd.Value, key string) string {
	if payload == nil {
		return "?"
	}
	if v, ok := payload[key]; ok {
		if s := v.GetStringValue(); s != "" {
			return s
		}
	}
	return "?"
}
