// Command atego è il binario di caricamento dati vettoriali (build-time) su Qdrant.
// Port Go del tool Python embed-mrsmith.
package main

import (
	"fmt"
	"os"

	"github.com/sciacco/atego/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
