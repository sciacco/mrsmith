package training

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"
)

func TestLocalStoragePutComputesHashAndPersistsContent(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStorage: %v", err)
	}

	stored, err := storage.Put(context.Background(), "awards/award-1/attestato.pdf", strings.NewReader("pdf-bytes"), 64)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	hash := sha256.Sum256([]byte("pdf-bytes"))
	if stored.SHA256 != hex.EncodeToString(hash[:]) {
		t.Fatalf("SHA256 = %q, want %q", stored.SHA256, hex.EncodeToString(hash[:]))
	}
	if stored.SizeBytes != int64(len("pdf-bytes")) {
		t.Fatalf("SizeBytes = %d, want %d", stored.SizeBytes, len("pdf-bytes"))
	}

	body, err := storage.Get(context.Background(), stored.Key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(raw) != "pdf-bytes" {
		t.Fatalf("stored content = %q", string(raw))
	}
}

func TestLocalStorageRejectsTraversalAndOversize(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStorage: %v", err)
	}

	if _, err := storage.Put(context.Background(), "../escape.pdf", strings.NewReader("x"), 64); err == nil {
		t.Fatal("expected traversal key to fail")
	}
	if _, err := storage.Put(context.Background(), "docs/too-large.pdf", strings.NewReader("12345"), 4); err == nil {
		t.Fatal("expected oversize upload to fail")
	}
}
