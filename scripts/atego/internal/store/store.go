// Package store implementa lo store Postgres del loader (pgx).
// Sostituisce il vecchio store Qdrant: i vettori vivono in colonne real[]
// (float4[]) su binocolo.kb_concept / binocolo.kb_ateco_node, e la curatela
// (concetti + mapping fit) viene sincronizzata dalla JSON sorgente.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sciacco/atego/internal/sources"
)

// Store wrappa un pool pgx.
type Store struct {
	pool *pgxpool.Pool
}

// New apre il pool e verifica la connessione.
func New(ctx context.Context, dsn string) (*Store, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(cctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(cctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close rilascia il pool.
func (s *Store) Close() { s.pool.Close() }

// ResolveEmbeddingModel restituisce (id, dimension) del modello di embedding
// registrato in mrsmith.embedding_model, cercato per stringa modello.
func (s *Store) ResolveEmbeddingModel(ctx context.Context, model string) (id string, dimension int, err error) {
	err = s.pool.QueryRow(ctx,
		`SELECT id::text, dimension FROM mrsmith.embedding_model WHERE model = $1`, model).
		Scan(&id, &dimension)
	if err != nil {
		return "", 0, fmt.Errorf("modello embedding %q non trovato in mrsmith.embedding_model (applicata la migration 058?): %w", model, err)
	}
	return id, dimension, nil
}

// ConceptHashes restituisce id -> source_text_hash per i concetti già embeddati.
func (s *Store) ConceptHashes(ctx context.Context) (map[string]string, error) {
	return s.hashes(ctx, `SELECT id, source_text_hash FROM binocolo.kb_concept WHERE source_text_hash IS NOT NULL`)
}

// AtecoHashes restituisce codice -> source_text_hash per i nodi già embeddati.
func (s *Store) AtecoHashes(ctx context.Context) (map[string]string, error) {
	return s.hashes(ctx, `SELECT codice, source_text_hash FROM binocolo.kb_ateco_node WHERE source_text_hash IS NOT NULL`)
}

func (s *Store) hashes(ctx context.Context, sql string) (map[string]string, error) {
	rows, err := s.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, h string
		if err := rows.Scan(&k, &h); err != nil {
			return nil, err
		}
		out[k] = h
	}
	return out, rows.Err()
}

// SyncConcepts sincronizza la curatela dei concetti dalla JSON: upsert dei
// metadati (sempre) + replace del mapping fit (sempre); per i concetti il cui
// vettore è in `vectors`, scrive embedding + source_text_hash + embedding_model_id.
// Tutto in una transazione.
func (s *Store) SyncConcepts(ctx context.Context, concepts []sources.BusinessConcept, vectors map[string][]float32, modelID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, c := range concepts {
		if _, err := tx.Exec(ctx, `
			INSERT INTO binocolo.kb_concept (id, name, domain, kind, aliases, embedding_text, contrast_text)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (id) DO UPDATE SET
			  name=EXCLUDED.name, domain=EXCLUDED.domain, kind=EXCLUDED.kind,
			  aliases=EXCLUDED.aliases, embedding_text=EXCLUDED.embedding_text,
			  contrast_text=EXCLUDED.contrast_text, updated_at=now()`,
			c.ID, c.Name, c.Domain, c.Kind, c.Aliases, c.EmbeddingText, c.ContrastText); err != nil {
			return fmt.Errorf("upsert concept %s: %w", c.ID, err)
		}

		// Replace fit mapping (la JSON è canonica).
		if _, err := tx.Exec(ctx, `DELETE FROM binocolo.kb_concept_ateco WHERE concept_id=$1`, c.ID); err != nil {
			return fmt.Errorf("clear fit %s: %w", c.ID, err)
		}
		for i, code := range c.AtecoCandidatesInKB {
			if _, err := tx.Exec(ctx,
				`INSERT INTO binocolo.kb_concept_ateco (concept_id, ateco_code, relation, position)
				 VALUES ($1,$2,'in_kb',$3) ON CONFLICT DO NOTHING`,
				c.ID, code, i); err != nil {
				return fmt.Errorf("insert in_kb %s/%s: %w", c.ID, code, err)
			}
		}
		for _, code := range c.AtecoCandidatesExcluded {
			if _, err := tx.Exec(ctx,
				`INSERT INTO binocolo.kb_concept_ateco (concept_id, ateco_code, relation, position)
				 VALUES ($1,$2,'excluded',NULL) ON CONFLICT DO NOTHING`,
				c.ID, code); err != nil {
				return fmt.Errorf("insert excluded %s/%s: %w", c.ID, code, err)
			}
		}

		if vec, ok := vectors[c.ID]; ok {
			h := sources.SourceTextHash(c.EmbeddingText)
			if _, err := tx.Exec(ctx,
				`UPDATE binocolo.kb_concept
				 SET embedding=$2, source_text_hash=$3, embedding_model_id=$4::uuid, updated_at=now()
				 WHERE id=$1`,
				c.ID, vec, h, modelID); err != nil {
				return fmt.Errorf("set concept embedding %s: %w", c.ID, err)
			}
		}
	}
	return tx.Commit(ctx)
}

// SyncAteco sincronizza i nodi ATECO ICT (no mapping fit: vive sui concetti).
func (s *Store) SyncAteco(ctx context.Context, nodes []sources.AtecoNode, vectors map[string][]float32, modelID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, n := range nodes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO binocolo.kb_ateco_node (codice, relevance_tier, keywords, seed_concepts, path_text, embedding_text)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (codice) DO UPDATE SET
			  relevance_tier=EXCLUDED.relevance_tier, keywords=EXCLUDED.keywords,
			  seed_concepts=EXCLUDED.seed_concepts, path_text=EXCLUDED.path_text,
			  embedding_text=EXCLUDED.embedding_text, updated_at=now()`,
			n.Codice, n.RelevanceTier, n.Keywords, n.SeedConcepts, n.PathText, n.EmbeddingText); err != nil {
			return fmt.Errorf("upsert ateco %s: %w", n.Codice, err)
		}

		if vec, ok := vectors[n.Codice]; ok {
			h := sources.SourceTextHash(n.EmbeddingText)
			if _, err := tx.Exec(ctx,
				`UPDATE binocolo.kb_ateco_node
				 SET embedding=$2, source_text_hash=$3, embedding_model_id=$4::uuid, updated_at=now()
				 WHERE codice=$1`,
				n.Codice, vec, h, modelID); err != nil {
				return fmt.Errorf("set ateco embedding %s: %w", n.Codice, err)
			}
		}
	}
	return tx.Commit(ctx)
}

// ConceptVec è una riga per la ricerca di prova (smoke).
type ConceptVec struct {
	ID   string
	Name string
	Kind string
	Vec  []float32
}

// LoadConceptVectors carica i concetti embeddati (per la smoke / preview del
// percorso runtime). Salta i concetti senza vettore.
func (s *Store) LoadConceptVectors(ctx context.Context) ([]ConceptVec, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, kind, embedding FROM binocolo.kb_concept WHERE embedding IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConceptVec
	for rows.Next() {
		var cv ConceptVec
		if err := rows.Scan(&cv.ID, &cv.Name, &cv.Kind, &cv.Vec); err != nil {
			return nil, err
		}
		out = append(out, cv)
	}
	return out, rows.Err()
}
