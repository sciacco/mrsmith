package sources

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
)

// Valori attesi calcolati con Python per garantire parità point-id:
//
//	import uuid
//	ns = uuid.uuid5(uuid.NAMESPACE_DNS, "embed-mrsmith.local")  # 0b89cdaf-...
//	uuid.uuid5(ns, "ateco:62.20.20")                            # c7bb0066-...
//	uuid.uuid5(ns, "concept:cloud_infrastructure")
//	uuid.uuid5(ns, "ateco:62.20")
const (
	wantNamespace      = "0b89cdaf-93a7-5854-97cc-53d0dc68eb31"
	wantPointAtecoFull = "c7bb0066-48da-5836-b9f5-ef46c5a2d35e"
	wantPointAtecoMid  = "f2699617-50dd-5130-ac23-7f482691111d" // "ateco:62.20"
	wantPointConcept   = "1e1aa02f-31d9-59e1-90fa-f329a0654fa1" // "concept:msp"
)

func TestModuleNamespaceParity(t *testing.T) {
	if got := ModuleNamespace.String(); got != wantNamespace {
		t.Fatalf("namespace mismatch: got %s want %s", got, wantNamespace)
	}
}

func TestPointIDAtecoParity(t *testing.T) {
	cases := map[string]string{
		"62.20.20": wantPointAtecoFull,
		"62.20":    wantPointAtecoMid,
	}
	for codice, want := range cases {
		if got := PointIDAteco(codice); got != want {
			t.Fatalf("PointIDAteco(%q) = %s, want %s", codice, got, want)
		}
	}
}

func TestPointIDConceptParity(t *testing.T) {
	if got := PointIDConcept("msp"); got != wantPointConcept {
		t.Fatalf("PointIDConcept(\"msp\") = %s, want %s", got, wantPointConcept)
	}
}

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

func TestLoadAtecoNodes(t *testing.T) {
	path := filepath.Join("..", "..", "data", "ateco_ict_kb.json")
	nodes, fileHash, err := LoadAtecoNodes(path)
	if err != nil {
		t.Fatalf("LoadAtecoNodes: %v", err)
	}
	if len(nodes) != 37 {
		t.Fatalf("expected 37 ATECO nodes, got %d", len(nodes))
	}
	if fileHash == "" {
		t.Fatal("empty file hash")
	}
	first := nodes[0]
	if first.Codice != "26.11.00" {
		t.Fatalf("first codice = %q, want 26.11.00", first.Codice)
	}
	if first.EmbeddingText == "" {
		t.Fatal("empty embedding_text")
	}
	if first.Gerarchia == 0 {
		t.Fatal("gerarchia not parsed")
	}
	if len(first.Keywords) == 0 {
		t.Fatal("keywords not parsed")
	}
}

func TestLoadBusinessConcepts(t *testing.T) {
	path := filepath.Join("..", "..", "data", "concept_index_source.json")
	concepts, fileHash, err := LoadBusinessConcepts(path)
	if err != nil {
		t.Fatalf("LoadBusinessConcepts: %v", err)
	}
	if len(concepts) != 29 {
		t.Fatalf("expected 29 concepts, got %d", len(concepts))
	}
	if fileHash == "" {
		t.Fatal("empty file hash")
	}
	first := concepts[0]
	if first.ID != "cloud_infrastructure" {
		t.Fatalf("first id = %q, want cloud_infrastructure", first.ID)
	}
}

func TestValidateEmbeddingTextEmpty(t *testing.T) {
	for _, v := range []any{nil, "", "   "} {
		if _, err := validateEmbeddingText(v, "label[0]"); err == nil {
			t.Fatalf("expected error for %v", v)
		}
	}
}
