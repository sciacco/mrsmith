package training

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestReadStorageBodyComputesHashAndSize(t *testing.T) {
	content, sha, size, err := readStorageBody(strings.NewReader("pdf-bytes"), 64)
	if err != nil {
		t.Fatalf("readStorageBody: %v", err)
	}
	hash := sha256.Sum256([]byte("pdf-bytes"))
	if sha != hex.EncodeToString(hash[:]) {
		t.Fatalf("SHA256 = %q, want %q", sha, hex.EncodeToString(hash[:]))
	}
	if size != int64(len("pdf-bytes")) {
		t.Fatalf("size = %d, want %d", size, len("pdf-bytes"))
	}
	if string(content) != "pdf-bytes" {
		t.Fatalf("content = %q", string(content))
	}
}

func TestReadStorageBodyRejectsOversize(t *testing.T) {
	if _, _, _, err := readStorageBody(strings.NewReader("12345"), 4); err == nil {
		t.Fatal("expected oversize upload to fail")
	}
}

func TestStorageKeyRejectsEmpty(t *testing.T) {
	if _, err := storageKey("  / "); err == nil {
		t.Fatal("expected empty key to fail")
	}
	key, err := storageKey("/awards/award-1/attestato.pdf")
	if err != nil {
		t.Fatalf("storageKey: %v", err)
	}
	if key != "awards/award-1/attestato.pdf" {
		t.Fatalf("key = %q", key)
	}
}
