package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sciacco/atego/internal/embeddings"
	"github.com/sciacco/atego/internal/store"
)

var (
	smokeIncludeDistractors bool
	smokeInstruct           string
	smokeRaw                bool
)

// Istruzione di default UC1 (lato query, formato Qwen). Il runtime la terrà in
// mrsmith.llm_prompt; qui è il banco di prova per calibrarne il testo.
const smokeDefaultInstruct = "Data una descrizione di settore aziendale per una ricerca M&A, recupera il concetto di business corrispondente."

var smokeCmd = &cobra.Command{
	Use:   "smoke [QUERY]",
	Short: "Query di prova: coseno in-memory sui concetti (anteprima del percorso runtime)",
	Args:  cobra.ExactArgs(1),
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

		query := args[0]
		// Documenti embeddati GREZZI; la query porta l'istruzione (asimmetria Qwen),
		// salvo --raw per confronto.
		embedInput := query
		mode := "raw"
		if !smokeRaw {
			embedInput = fmt.Sprintf("Instruct: %s\nQuery: %s", smokeInstruct, query)
			mode = "instruct"
		}
		log.Info("embedding query", "query", query, "mode", mode)
		vecs, err := embedder.EmbedBatch(ctx, []string{embedInput})
		if err != nil {
			return err
		}
		q := vecs[0]

		concepts, err := st.LoadConceptVectors(ctx)
		if err != nil {
			return err
		}
		if len(concepts) == 0 {
			return fmt.Errorf("nessun concetto embeddato: lancia prima `atego build`")
		}

		type scored struct {
			id, name, kind string
			score          float64
		}
		var results []scored
		for _, c := range concepts {
			if !smokeIncludeDistractors && c.Kind == "distractor" {
				continue
			}
			results = append(results, scored{c.ID, c.Name, c.Kind, cosine(q, c.Vec)})
		}
		sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

		fmt.Printf("\nQuery: %s\n", query)
		if mode == "instruct" {
			fmt.Printf("Mode: instruct | Instruct: %s\n", smokeInstruct)
		} else {
			fmt.Printf("Mode: raw (nessuna istruzione)\n")
		}
		fmt.Printf("Model: %s | Dimension: %d | Concetti: %d\n", cfg.EmbeddingModel, len(q), len(results))
		fmt.Printf("\nTop-10 concetti per coseno:\n")
		fmt.Printf("%8s  %-12s %-26s %s\n", "Score", "Kind", "ID", "Name")
		fmt.Println(strings.Repeat("-", 72))
		for i, r := range results {
			if i >= 10 {
				break
			}
			fmt.Printf("%8.4f  %-12s %-26s %s\n", r.score, r.kind, r.id, r.name)
		}
		return nil
	},
}

func init() {
	smokeCmd.Flags().BoolVar(&smokeIncludeDistractors, "include-distractors", false, "Includi i concetti distractor (default solo target, come use case 1)")
	smokeCmd.Flags().StringVar(&smokeInstruct, "instruct", smokeDefaultInstruct, "Istruzione Qwen lato query (task description)")
	smokeCmd.Flags().BoolVar(&smokeRaw, "raw", false, "Embedda la query grezza, senza istruzione (per confronto)")
}

// cosine calcola la similarità coseno tra due vettori (gestisce vettori non
// normalizzati). Restituisce 0 se una norma è nulla o le lunghezze differiscono.
func cosine(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
