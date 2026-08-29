package training

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const defaultStorageMaxBytes int64 = 20 * 1024 * 1024

type StorageAdapter interface {
	Put(ctx context.Context, key string, body io.Reader, maxBytes int64) (StoredObject, error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type StoredObject struct {
	Key       string
	SHA256    string
	SizeBytes int64
}

// DBStorage salva i contenuti in training.document_blob: nessun filesystem
// locale in esercizio, i file vivono nel database accanto ai metadati di
// training.document.
type DBStorage struct {
	db *sql.DB
}

func NewDBStorage(db *sql.DB) *DBStorage {
	if db == nil {
		return nil
	}
	return &DBStorage{db: db}
}

func (s *DBStorage) Put(ctx context.Context, key string, body io.Reader, maxBytes int64) (StoredObject, error) {
	if s == nil || s.db == nil {
		return StoredObject{}, errors.New("training storage not configured")
	}
	key, err := storageKey(key)
	if err != nil {
		return StoredObject{}, err
	}
	content, digest, size, err := readStorageBody(body, maxBytes)
	if err != nil {
		return StoredObject{}, err
	}
	if err := ctx.Err(); err != nil {
		return StoredObject{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO training.document_blob (storage_key, content, sha256, size_bytes)
VALUES ($1, $2, $3, $4)
ON CONFLICT (storage_key) DO UPDATE
  SET content = EXCLUDED.content,
      sha256 = EXCLUDED.sha256,
      size_bytes = EXCLUDED.size_bytes`,
		key, content, digest, size); err != nil {
		return StoredObject{}, fmt.Errorf("store training document: %w", err)
	}
	return StoredObject{Key: key, SHA256: digest, SizeBytes: size}, nil
}

func (s *DBStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("training storage not configured")
	}
	key, err := storageKey(key)
	if err != nil {
		return nil, err
	}
	var content []byte
	err = s.db.QueryRowContext(ctx,
		`SELECT content FROM training.document_blob WHERE storage_key = $1`, key,
	).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("training document %q not found", key)
	}
	if err != nil {
		return nil, fmt.Errorf("read training document: %w", err)
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

func (s *DBStorage) Delete(ctx context.Context, key string) error {
	if s == nil || s.db == nil {
		return errors.New("training storage not configured")
	}
	key, err := storageKey(key)
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM training.document_blob WHERE storage_key = $1`, key); err != nil {
		return fmt.Errorf("delete training document: %w", err)
	}
	return nil
}

func storageKey(key string) (string, error) {
	key = strings.Trim(strings.TrimSpace(key), "/")
	if key == "" {
		return "", errors.New("invalid training storage key")
	}
	return key, nil
}

// readStorageBody consuma il body applicando il tetto per file e calcolando
// hash e dimensione, indipendentemente dal deposito che li conservera'.
func readStorageBody(body io.Reader, maxBytes int64) (content []byte, sha string, size int64, err error) {
	if maxBytes <= 0 {
		maxBytes = defaultStorageMaxBytes
	}
	var buf bytes.Buffer
	hasher := sha256.New()
	limited := io.LimitReader(body, maxBytes+1)
	written, err := io.Copy(&buf, io.TeeReader(limited, hasher))
	if err != nil {
		return nil, "", 0, fmt.Errorf("read training document: %w", err)
	}
	if written > maxBytes {
		return nil, "", 0, errors.New("training document too large")
	}
	return buf.Bytes(), hex.EncodeToString(hasher.Sum(nil)), written, nil
}
