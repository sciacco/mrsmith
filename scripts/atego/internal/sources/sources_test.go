package sources

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestSourceTextHash(t *testing.T) {
	// sha256("abc") = ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	sum := sha256.Sum256([]byte("abc"))
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("SourceTextHash mismatch: got %s want %s", got, want)
	}
	if got := SourceTextHash("abc"); got != want {
		t.Fatalf("SourceTextHash mismatch: got %s want %s", got, want)
	}
}

func TestValidateEmbeddingTextEmpty(t *testing.T) {
	for _, v := range []any{nil, "", "   "} {
		if _, err := validateEmbeddingText(v, "label[0]"); err == nil {
			t.Fatalf("expected error for %v", v)
		}
	}
}
