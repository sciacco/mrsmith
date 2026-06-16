package binocolo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/openapiit"
)

const (
	provinceCacheKey = "list_provinces"
	provinceCacheTTL = 30 * 24 * time.Hour
)

var (
	errProvinceCacheFailure         = errors.New("province cache failure")
	errProvinceOpenAPIITUnavailable = errors.New("openapiit unavailable")
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

func timeNowUTC() time.Time {
	return time.Now().UTC()
}

func listProvincesWithCache(ctx context.Context, cache provinceCacheStore, client *openapiit.Client, now func() time.Time) (openapiit.Envelope[[]openapiit.Province], json.RawMessage, error) {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if cache != nil {
		entry, err := cache.GetValidProvinceCache(ctx, now())
		if err != nil {
			return openapiit.Envelope[[]openapiit.Province]{}, nil, fmt.Errorf("%w: %v", errProvinceCacheFailure, err)
		}
		if entry != nil {
			envelope, err := decodeProvinceEnvelope(entry.Response)
			if err != nil {
				return openapiit.Envelope[[]openapiit.Province]{}, nil, fmt.Errorf("%w: %v", errProvinceCacheFailure, err)
			}
			return envelope, entry.Response, err
		}
	}
	if client == nil {
		return openapiit.Envelope[[]openapiit.Province]{}, nil, errProvinceOpenAPIITUnavailable
	}

	fetch := func(ctx context.Context) (openapiit.Envelope[[]openapiit.Province], json.RawMessage, error) {
		result, err := client.CAP().ListProvinces(ctx)
		if err != nil {
			return openapiit.Envelope[[]openapiit.Province]{}, nil, err
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return openapiit.Envelope[[]openapiit.Province]{}, nil, err
		}
		return result, raw, nil
	}

	if cache == nil {
		return fetch(ctx)
	}

	var result openapiit.Envelope[[]openapiit.Province]
	var raw json.RawMessage
	var upstreamErr error
	err := cache.WithProvinceCacheLock(ctx, func(ctx context.Context) error {
		entry, err := cache.GetValidProvinceCache(ctx, now())
		if err != nil {
			return err
		}
		if entry != nil {
			result, err = decodeProvinceEnvelope(entry.Response)
			raw = entry.Response
			return err
		}
		result, raw, upstreamErr = fetch(ctx)
		if upstreamErr != nil {
			return nil
		}
		fetchedAt := now()
		if err := cache.UpsertProvinceCache(ctx, provinceCacheWrite{
			Response:  raw,
			FetchedAt: fetchedAt,
			ExpiresAt: fetchedAt.Add(provinceCacheTTL),
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return openapiit.Envelope[[]openapiit.Province]{}, nil, fmt.Errorf("%w: %v", errProvinceCacheFailure, err)
	}
	if upstreamErr != nil {
		return openapiit.Envelope[[]openapiit.Province]{}, nil, upstreamErr
	}
	return result, raw, nil
}

func decodeProvinceEnvelope(raw json.RawMessage) (openapiit.Envelope[[]openapiit.Province], error) {
	var response openapiit.Envelope[[]openapiit.Province]
	if err := json.Unmarshal(raw, &response); err != nil {
		return response, fmt.Errorf("decode cached province list: %w", err)
	}
	return response, nil
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
