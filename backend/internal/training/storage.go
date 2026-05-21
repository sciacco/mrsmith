package training

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

type LocalStorage struct {
	root string
}

func NewLocalStorage(root string) (*LocalStorage, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create training storage root: %w", err)
	}
	return &LocalStorage{root: root}, nil
}

func (s *LocalStorage) Put(ctx context.Context, key string, body io.Reader, maxBytes int64) (StoredObject, error) {
	if s == nil {
		return StoredObject{}, errors.New("training storage not configured")
	}
	if maxBytes <= 0 {
		maxBytes = defaultStorageMaxBytes
	}
	path, err := s.safePath(key)
	if err != nil {
		return StoredObject{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return StoredObject{}, fmt.Errorf("create training storage directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return StoredObject{}, fmt.Errorf("create training storage temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	hasher := sha256.New()
	limited := io.LimitReader(body, maxBytes+1)
	written, err := io.Copy(tmp, io.TeeReader(limited, hasher))
	if err != nil {
		tmp.Close()
		return StoredObject{}, fmt.Errorf("write training document: %w", err)
	}
	if err := ctx.Err(); err != nil {
		tmp.Close()
		return StoredObject{}, err
	}
	if written > maxBytes {
		tmp.Close()
		return StoredObject{}, errors.New("training document too large")
	}
	if err := tmp.Close(); err != nil {
		return StoredObject{}, fmt.Errorf("close training document: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return StoredObject{}, fmt.Errorf("store training document: %w", err)
	}

	return StoredObject{
		Key:       key,
		SHA256:    hex.EncodeToString(hasher.Sum(nil)),
		SizeBytes: written,
	}, nil
}

func (s *LocalStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if s == nil {
		return nil, errors.New("training storage not configured")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := s.safePath(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	if s == nil {
		return errors.New("training storage not configured")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.safePath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *LocalStorage) safePath(key string) (string, error) {
	key = strings.Trim(strings.TrimSpace(key), "/")
	if key == "" || strings.Contains(key, "..") {
		return "", errors.New("invalid training storage key")
	}
	path := filepath.Join(s.root, filepath.FromSlash(key))
	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", errors.New("invalid training storage key")
	}
	return abs, nil
}
