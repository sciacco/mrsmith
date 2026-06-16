package binocolo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	provinceCacheKey = "list_provinces"
	provinceCacheTTL = 30 * 24 * time.Hour
)

type provinceCacheEntry struct {
	Response json.RawMessage
}

type provinceCacheWrite struct {
	Response  json.RawMessage
	FetchedAt time.Time
	ExpiresAt time.Time
}

type provinceCacheStore interface {
	GetValidProvinceCache(ctx context.Context, now time.Time) (*provinceCacheEntry, error)
	WithProvinceCacheLock(ctx context.Context, fn func(context.Context) error) error
	UpsertProvinceCache(ctx context.Context, input provinceCacheWrite) error
}

func (s *SQLStore) GetValidProvinceCache(ctx context.Context, now time.Time) (*provinceCacheEntry, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo province cache store not configured")
	}

	var response []byte
	err := s.db.QueryRowContext(ctx, `
UPDATE binocolo.province_cache
SET last_served_at = $2,
    served_count = served_count + 1
WHERE cache_key = $1
  AND expires_at > $2
RETURNING response
`, provinceCacheKey, now).Scan(&response)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get valid province cache: %w", err)
	}

	return &provinceCacheEntry{Response: json.RawMessage(append([]byte(nil), response...))}, nil
}

func (s *SQLStore) WithProvinceCacheLock(ctx context.Context, fn func(context.Context) error) (err error) {
	if s == nil || s.db == nil {
		return errors.New("binocolo province cache store not configured")
	}

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open province cache lock connection: %w", err)
	}
	defer conn.Close()

	lockID := companySearchAdvisoryLockID("province:" + provinceCacheKey)
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, lockID); err != nil {
		return fmt.Errorf("acquire province cache lock: %w", err)
	}
	defer func() {
		if _, unlockErr := conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, lockID); unlockErr != nil && err == nil {
			err = fmt.Errorf("release province cache lock: %w", unlockErr)
		}
	}()

	return fn(ctx)
}

func (s *SQLStore) UpsertProvinceCache(ctx context.Context, input provinceCacheWrite) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo province cache store not configured")
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.province_cache (
  cache_key,
  response,
  fetched_at,
  expires_at,
  last_served_at,
  served_count
) VALUES (
  $1,
  $2::jsonb,
  $3,
  $4,
  NULL,
  0
)
ON CONFLICT (cache_key) DO UPDATE
SET response = EXCLUDED.response,
    fetched_at = EXCLUDED.fetched_at,
    expires_at = EXCLUDED.expires_at,
    last_served_at = NULL,
    served_count = 0,
    updated_at = now()
`, provinceCacheKey,
		[]byte(input.Response),
		input.FetchedAt,
		input.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("upsert province cache: %w", err)
	}
	return nil
}
