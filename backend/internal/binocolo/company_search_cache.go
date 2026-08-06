package binocolo

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/openapiit"
)

// Company-search responses are shared for one day: the compact discovery flow
// keeps upstream traffic low without leaving company facts stale for weeks.
const companySearchCacheTTL = 24 * time.Hour

type companySearchCacheEntry struct {
	Response json.RawMessage
	// FetchedAt è l'istante della CHIAMATA AL FORNITORE che ha prodotto questa
	// risposta, non quello in cui la si rilegge. Il registro identità azienda lo
	// usa per la regola sul nome (issue #86): servire una risposta vecchia dalla
	// cache non è una nuova osservazione, e non deve poter sovrascrivere un nome
	// osservato più tardi.
	FetchedAt time.Time
}

type companySearchCacheWrite struct {
	CacheKey           string
	Params             json.RawMessage
	Response           json.RawMessage
	DryRun             bool
	DataEnrichment     string
	FetchedAt          time.Time
	ExpiresAt          time.Time
	RefreshedBySubject string
	RefreshedByEmail   string
}

type companySearchCacheStore interface {
	GetValidCompanySearch(ctx context.Context, cacheKey string, now time.Time) (*companySearchCacheEntry, error)
	WithCompanySearchCacheLock(ctx context.Context, cacheKey string, fn func(context.Context) error) error
	UpsertCompanySearch(ctx context.Context, input companySearchCacheWrite) error
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	if db == nil {
		return nil
	}
	return &SQLStore{db: db}
}

func (s *SQLStore) GetValidCompanySearch(ctx context.Context, cacheKey string, now time.Time) (*companySearchCacheEntry, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo cache store not configured")
	}

	var response []byte
	var fetchedAt time.Time
	err := s.db.QueryRowContext(ctx, `
UPDATE binocolo.company_search_cache
SET last_served_at = $2,
    served_count = served_count + 1
WHERE cache_key = $1
  AND expires_at > $2
RETURNING response, fetched_at
`, cacheKey, now).Scan(&response, &fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get valid company search cache: %w", err)
	}

	return &companySearchCacheEntry{
		Response:  json.RawMessage(append([]byte(nil), response...)),
		FetchedAt: fetchedAt,
	}, nil
}

func (s *SQLStore) WithCompanySearchCacheLock(ctx context.Context, cacheKey string, fn func(context.Context) error) (err error) {
	if s == nil || s.db == nil {
		return errors.New("binocolo cache store not configured")
	}

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open cache lock connection: %w", err)
	}
	defer conn.Close()

	lockID := companySearchAdvisoryLockID(cacheKey)
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, lockID); err != nil {
		return fmt.Errorf("acquire company search cache lock: %w", err)
	}
	defer func() {
		if _, unlockErr := conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, lockID); unlockErr != nil && err == nil {
			err = fmt.Errorf("release company search cache lock: %w", unlockErr)
		}
	}()

	return fn(ctx)
}

func (s *SQLStore) UpsertCompanySearch(ctx context.Context, input companySearchCacheWrite) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo cache store not configured")
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.company_search_cache (
  cache_key,
  params,
  response,
  dry_run,
  data_enrichment,
  fetched_at,
  expires_at,
  last_served_at,
  served_count,
  last_refreshed_by_subject,
  last_refreshed_by_email
) VALUES (
  $1,
  $2::jsonb,
  $3::jsonb,
  $4,
  $5,
  $6,
  $7,
  NULL,
  0,
  $8,
  $9
)
ON CONFLICT (cache_key) DO UPDATE
SET params = EXCLUDED.params,
    response = EXCLUDED.response,
    dry_run = EXCLUDED.dry_run,
    data_enrichment = EXCLUDED.data_enrichment,
    fetched_at = EXCLUDED.fetched_at,
    expires_at = EXCLUDED.expires_at,
    last_served_at = NULL,
    served_count = 0,
    last_refreshed_by_subject = EXCLUDED.last_refreshed_by_subject,
    last_refreshed_by_email = EXCLUDED.last_refreshed_by_email,
    updated_at = now()
`, input.CacheKey,
		[]byte(input.Params),
		[]byte(input.Response),
		input.DryRun,
		strings.TrimSpace(input.DataEnrichment),
		input.FetchedAt,
		input.ExpiresAt,
		nullString(input.RefreshedBySubject),
		nullString(input.RefreshedByEmail),
	)
	if err != nil {
		return fmt.Errorf("upsert company search cache: %w", err)
	}
	return nil
}

func companySearchCacheKey(params openapiit.CompanyITSearchParams) (string, json.RawMessage, error) {
	values := normalizedCompanySearchValues(params)
	encoded := values.Encode()
	sum := sha256.Sum256([]byte(encoded))

	paramsJSON, err := companySearchParamsJSON(values)
	if err != nil {
		return "", nil, err
	}
	return hex.EncodeToString(sum[:]), paramsJSON, nil
}

func normalizedCompanySearchValues(params openapiit.CompanyITSearchParams) url.Values {
	values := params.Values()
	normalized := make(url.Values, len(values))
	for key, items := range values {
		for _, item := range items {
			if strings.TrimSpace(item) != "" {
				normalized.Add(key, item)
			}
		}
	}
	return normalized
}

func companySearchParamsJSON(values url.Values) (json.RawMessage, error) {
	params := make(map[string]string, len(values))
	for key, items := range values {
		if len(items) == 0 {
			continue
		}
		value := strings.TrimSpace(items[0])
		if value == "" {
			continue
		}
		params[key] = value
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal company search cache params: %w", err)
	}
	return json.RawMessage(raw), nil
}

func companySearchAdvisoryLockID(cacheKey string) int64 {
	sum := sha256.Sum256([]byte(cacheKey))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

func nullString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func positiveOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
