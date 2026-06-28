package cli

import (
	"fmt"

	"github.com/sciacco/atego/internal/embeddings"
	"github.com/spf13/cobra"
)

var probeCmd = &cobra.Command{
	Use:   "probe",
	Short: "Prova il modello e stampa la dimensione del vettore",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, log := mustConfig()
		ctx, cancel := signalCtx()
		defer cancel()

		c := embeddings.New(cfg, log)
		dim, err := c.ProbeDimension(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("Model: %s\n", cfg.EmbeddingModel)
		fmt.Printf("Dimension: %d\n", dim)
		return nil
	},
}
