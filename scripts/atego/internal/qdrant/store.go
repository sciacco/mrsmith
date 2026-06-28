// Package qdrant implementa lo store su Qdrant (client gRPC ufficiale).
// È la traduzione di src/ingest/qdrant_store.py.
//
// Nota: il client go-client di Qdrant usa gRPC (porta 6334), mentre il tool
// Python usava REST (porta 6333, come da QDRANT_URL). ParseGrpcAddr converte
// QDRANT_URL in host:port gRPC rimappando automaticamente 6333 → 6334.
package qdrant

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/qdrant/go-client/qdrant"

	"github.com/sciacco/atego/internal/config"
)

// Store wrappa il client gRPC Qdrant.
type Store struct {
	client  *qdrant.Client
	log     *slog.Logger
	restURL string // URL base REST (es. http://localhost:6333) per le operazioni snapshot
	apiKey  string // eventuale api-key per header REST
}

// New costruisce il client Qdrant parsando cfg.QdrantURL in host:port gRPC.
func New(cfg *config.Config, log *slog.Logger) (*Store, error) {
	host, port, useTLS, err := ParseGrpcAddr(cfg.QdrantURL)
	if err != nil {
		return nil, fmt.Errorf("qdrant url non valida: %w", err)
	}
	qcfg := &qdrant.Config{
		Host:                   host,
		Port:                   port,
		APIKey:                 cfg.QdrantAPIKey,
		UseTLS:                 useTLS,
		SkipCompatibilityCheck: true,
	}
	client, err := qdrant.NewClient(qcfg)
	if err != nil {
		return nil, err
	}
	return &Store{client: client, log: log, restURL: strings.TrimRight(cfg.QdrantURL, "/"), apiKey: cfg.QdrantAPIKey}, nil
}

// Close rilascia la connessione.
func (s *Store) Close() error { return s.client.Close() }

// ParseGrpcAddr converte un QDRANT_URL (es. http://localhost:6333) in
// (host, grpcPort, useTLS). Se la porta è 6333 (REST) la rimappa a 6334 (gRPC).
// Se la porta è assente usa 6334. scheme https/grpcs → useTLS=true.
func ParseGrpcAddr(rawURL string) (host string, port int, useTLS bool, err error) {
	if rawURL == "" {
		return "", 0, false, fmt.Errorf("url vuota")
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", 0, false, err
	}
	host = u.Hostname()
	if host == "" {
		host = "localhost"
	}
	scheme := strings.ToLower(u.Scheme)
	useTLS = scheme == "https" || scheme == "grpcs" || scheme == "wss"

	portStr := u.Port()
	switch {
	case portStr == "":
		port = 6334
	case portStr == "6333":
		port = 6334 // REST → gRPC
	default:
		p, perr := strconv.Atoi(portStr)
		if perr != nil {
			return "", 0, false, fmt.Errorf("porta non valida %q: %w", portStr, perr)
		}
		port = p
	}
	return host, port, useTLS, nil
}

// EnsureCollection crea la collection se non esiste, oppure la ricrea con
// recreate=true. Se esiste già con dimensione diversa restituisce errore.
func (s *Store) EnsureCollection(ctx context.Context, name string, dim int, recreate bool) error {
	if recreate {
		exists, err := s.client.CollectionExists(ctx, name)
		if err != nil {
			return fmt.Errorf("check collection exists: %w", err)
		}
		if exists {
			s.log.Info("deleting existing collection (--recreate)", "collection", name)
			if err := s.client.DeleteCollection(ctx, name); err != nil {
				return fmt.Errorf("delete collection: %w", err)
			}
		}
	}

	exists, err := s.client.CollectionExists(ctx, name)
	if err != nil {
		return fmt.Errorf("check collection exists: %w", err)
	}
	if !exists {
		s.log.Info("creating collection", "collection", name, "size", dim, "distance", "Cosine")
		err := s.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: name,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     uint64(dim),
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			return fmt.Errorf("create collection: %w", err)
		}
		return nil
	}

	// collection esiste — verifica dimensione
	existing, err := s.GetCollectionDimension(ctx, name)
	if err != nil {
		return err
	}
	if existing != dim {
		return fmt.Errorf("collection '%s' has dimension %d but model requires %d. Use --recreate to rebuild it",
			name, existing, dim)
	}
	s.log.Info("collection already exists with matching dimension", "collection", name, "dimension", dim)
	return nil
}

// CollectionExists dice se la collection esiste.
func (s *Store) CollectionExists(ctx context.Context, name string) (bool, error) {
	return s.client.CollectionExists(ctx, name)
}

// DeleteCollection elimina una collection (usato da import --recreate).
func (s *Store) DeleteCollection(ctx context.Context, name string) error {
	if err := s.client.DeleteCollection(ctx, name); err != nil {
		return fmt.Errorf("delete collection: %w", err)
	}
	return nil
}

// GetCollectionDimension restituisce la dimensione del vettore della collection
// (0 + error se la collection non ha una config params singola).
func (s *Store) GetCollectionDimension(ctx context.Context, name string) (int, error) {
	info, err := s.client.GetCollectionInfo(ctx, name)
	if err != nil {
		return 0, fmt.Errorf("get collection info: %w", err)
	}
	vc := info.GetConfig().GetParams().GetVectorsConfig()
	if vc == nil {
		return 0, fmt.Errorf("collection '%s' has no vectors config", name)
	}
	if p, ok := vc.GetConfig().(*qdrant.VectorsConfig_Params); ok {
		return int(p.Params.GetSize()), nil
	}
	return 0, fmt.Errorf("collection '%s' has named vectors (unsupported)", name)
}

// UpsertPoints inserisce o aggiorna punti (Wait=true). No-op se slice vuota.
func (s *Store) UpsertPoints(ctx context.Context, name string, points []*qdrant.PointStruct) error {
	if len(points) == 0 {
		return nil
	}
	wait := true
	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: name,
		Wait:           &wait,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("upsert points: %w", err)
	}
	s.log.Info("upserted points", "count", len(points), "collection", name)
	return nil
}

// CountPoints conta i punti (exact=true).
func (s *Store) CountPoints(ctx context.Context, name string) (int, error) {
	exact := true
	n, err := s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: name,
		Exact:          &exact,
	})
	if err != nil {
		return 0, fmt.Errorf("count points: %w", err)
	}
	return int(n), nil
}

// Search restituisce i topK punti più vicini al vettore (con payload).
func (s *Store) Search(ctx context.Context, name string, vector []float32, topK int) ([]*qdrant.ScoredPoint, error) {
	limit := uint64(topK)
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: name,
		Query:          qdrant.NewQueryNearest(qdrant.NewVectorInputDense(vector)),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("query points: %w", err)
	}
	return res, nil
}

// GetExistingHashes recupera source_text_hash per i point id richiesti
// via scroll paginato (limit 100). Restituisce map[pointID]hash.
func (s *Store) GetExistingHashes(ctx context.Context, name string, ids map[string]struct{}) (map[string]string, error) {
	results := make(map[string]string)
	if len(ids) == 0 {
		return results, nil
	}
	limit := uint32(100)
	var offset *qdrant.PointId
	for {
		page, next, err := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: name,
			Limit:          &limit,
			WithPayload:    qdrant.NewWithPayload(true),
			Offset:         offset,
		})
		if err != nil {
			return nil, fmt.Errorf("scroll points: %w", err)
		}
		for _, p := range page {
			pid := p.GetId().GetUuid()
			if _, want := ids[pid]; !want {
				continue
			}
			if payload := p.GetPayload(); payload != nil {
				if v, ok := payload["source_text_hash"]; ok {
					if h := v.GetStringValue(); h != "" {
						results[pid] = h
					}
				}
			}
		}
		if next == nil {
			break
		}
		offset = next
		// safety: se Qdrant non restituisce mai next==nil, evita loop infinito
		if len(page) == 0 {
			break
		}
	}
	return results, nil
}

// --- Snapshot operations ---

// CreateSnapshot crea uno snapshot della collection e restituisce il nome.
func (s *Store) CreateSnapshot(ctx context.Context, name string) (string, error) {
	s.log.Info("creating snapshot", "collection", name)
	snap, err := s.client.CreateSnapshot(ctx, name)
	if err != nil {
		return "", fmt.Errorf("create snapshot: %w", err)
	}
	s.log.Info("snapshot created", "name", snap.GetName())
	return snap.GetName(), nil
}

// ListSnapshots restituisce i nomi degli snapshot della collection.
func (s *Store) ListSnapshots(ctx context.Context, name string) ([]string, error) {
	snaps, err := s.client.ListSnapshots(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	out := make([]string, 0, len(snaps))
	for _, sn := range snaps {
		out = append(out, sn.GetName())
	}
	return out, nil
}

// DeleteSnapshot elimina uno snapshot dalla collection.
func (s *Store) DeleteSnapshot(ctx context.Context, collection, snapshotName string) error {
	if err := s.client.DeleteSnapshot(ctx, collection, snapshotName); err != nil {
		return fmt.Errorf("delete snapshot: %w", err)
	}
	s.log.Info("snapshot deleted", "snapshot", snapshotName, "collection", collection)
	return nil
}

// RecoverFromURL ripristina una collection da un URL di snapshot.
func (s *Store) RecoverFromURL(ctx context.Context, name, snapURL string) error {
	s.log.Info("recovering collection from URL", "collection", name, "url", snapURL)
	return s.restRecoverSnapshot(ctx, name, snapURL)
}

// RestoreFromLocalFile ripristina una collection da un file snapshot locale.
// Tenta prima file://<abs path>, poi fallback docker cp nel container Qdrant.
func (s *Store) RestoreFromLocalFile(ctx context.Context, name string, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	s.log.Info("restoring collection from local snapshot", "collection", name, "snapshot", filepath.Base(path))

	fileURL := "file://" + abs
	s.log.Info("trying file URL", "url", fileURL)
	if err := s.RestoreFromFileURL(ctx, name, fileURL); err == nil {
		return nil
	} else {
		s.log.Debug("file:// URL failed", "err", err)
	}

	// Fallback: docker cp nel container Qdrant
	s.log.Info("trying docker copy approach")
	containerPath := "/qdrant/snapshots/" + filepath.Base(path)
	containerName, derr := findQdrantContainer()
	if derr != nil {
		return fmt.Errorf("could not restore snapshot: %w", derr)
	}
	s.log.Info("found qdrant container", "container", containerName)
	if out, err := exec.Command("docker", "cp", abs, containerName+":"+containerPath).CombinedOutput(); err != nil {
		return fmt.Errorf("docker cp failed: %w: %s", err, string(out))
	}
	containerURL := "file://" + containerPath
	s.log.Info("recovering from container path", "url", containerURL)
	if err := s.RestoreFromFileURL(ctx, name, containerURL); err != nil {
		return fmt.Errorf("recover from container: %w", err)
	}
	// cleanup file temporaneo nel container
	_ = exec.Command("docker", "exec", containerName, "rm", "-f", containerPath).Run()
	return nil
}

// RestoreFromFileURL ripristina via REST PUT /collections/{name}/snapshots/recover
// con body {"location": url}.
func (s *Store) RestoreFromFileURL(ctx context.Context, name, snapURL string) error {
	return s.restRecoverSnapshotTimeout(ctx, name, snapURL, 120*time.Second)
}

// findQdrantContainer trova il nome del container Docker che gira qdrant/qdrant.
func findQdrantContainer() (string, error) {
	out, err := exec.Command("docker", "ps", "--filter", "ancestor=qdrant/qdrant", "--format", "{{.Names}}").Output()
	if err != nil {
		return "", fmt.Errorf("docker not available or no qdrant container: %w", err)
	}
	name := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if name == "" {
		return "", fmt.Errorf("no qdrant docker container found")
	}
	return name, nil
}
