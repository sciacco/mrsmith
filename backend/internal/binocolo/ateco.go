package binocolo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	atecoSearchDefaultLimit = 50
	atecoSearchHardLimit    = 100
)

var (
	errAtecoStoreUnavailable = errors.New("binocolo ateco store unavailable")
	errAtecoCodeNotFound     = errors.New("ateco code not found")
	atecoTokenPattern        = regexp.MustCompile(`[[:alnum:]]+`)
)

type atecoStore interface {
	ResolveAtecoCode(ctx context.Context, code string) (AtecoCode, error)
	SearchAteco(ctx context.Context, query string, limit int) ([]AtecoCode, error)
	SubtreeAtecoCodes(ctx context.Context, code string) ([]AtecoCode, error)
}

type AtecoCode struct {
	Ordine               int    `json:"-"`
	Codice               string `json:"codice"`
	CodiceSearch         string `json:"codiceSearch"`
	Titolo               string `json:"titolo"`
	Gerarchia            *int   `json:"gerarchia,omitempty"`
	NumeroCorrispondenze int    `json:"-"`
}

func (s *SQLStore) ResolveAtecoCode(ctx context.Context, code string) (AtecoCode, error) {
	if s == nil || s.db == nil {
		return AtecoCode{}, errAtecoStoreUnavailable
	}
	code = normalizeAtecoCode(code)
	if code == "" {
		return AtecoCode{}, errAtecoCodeNotFound
	}
	searchCode := atecoSearchCode(code)
	row := s.db.QueryRowContext(ctx, `
SELECT ordine, codice, codice_search, titolo, gerarchia, numero_corrispondenze
FROM binocolo.codici_ateco_2025
WHERE codice_search ~ '^[0-9]+$'
  AND (upper(codice) = upper($1) OR upper(codice_search) = upper($2))
ORDER BY CASE WHEN upper(codice) = upper($1) THEN 0 ELSE 1 END, ordine
LIMIT 1
`, code, searchCode)
	item, err := scanAtecoRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return AtecoCode{}, errAtecoCodeNotFound
	}
	if err != nil {
		return AtecoCode{}, fmt.Errorf("resolve ateco code: %w", err)
	}
	return item, nil
}

// SubtreeAtecoCodes returns the node itself plus every descendant in the ATECO
// hierarchy. Because OpenAPI.it matches the ATECO code exactly (no prefix, no
// descent) and companies are tagged at heterogeneous levels per branch, callers
// probe the whole subtree to discover which exact codes are actually populated.
func (s *SQLStore) SubtreeAtecoCodes(ctx context.Context, code string) ([]AtecoCode, error) {
	if s == nil || s.db == nil {
		return nil, errAtecoStoreUnavailable
	}
	code = normalizeAtecoCode(code)
	if code == "" {
		return nil, errAtecoCodeNotFound
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT ordine, codice, codice_search, titolo, gerarchia, numero_corrispondenze
FROM binocolo.codici_ateco_2025
WHERE codice_search ~ '^[0-9]+$'
  AND (upper(codice) = upper($1) OR upper(codice) LIKE upper($1) || '.%')
ORDER BY ordine
`, code)
	if err != nil {
		return nil, fmt.Errorf("subtree ateco codes: %w", err)
	}
	defer rows.Close()
	out := []AtecoCode{}
	for rows.Next() {
		item, err := scanAtecoScanner(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subtree ateco codes: %w", err)
	}
	return out, nil
}

func (s *SQLStore) SearchAteco(ctx context.Context, query string, limit int) ([]AtecoCode, error) {
	if s == nil || s.db == nil {
		return nil, errAtecoStoreUnavailable
	}
	query = cleanText(query, 240)
	if query == "" {
		return []AtecoCode{}, nil
	}
	limit = normalizeAtecoSearchLimit(limit)
	tokens := atecoSearchTokens(query)
	tokensRaw, err := json.Marshal(tokens)
	if err != nil {
		return nil, fmt.Errorf("marshal ateco search tokens: %w", err)
	}
	tsQuery := atecoTSQuery(tokens)
	rows, err := s.db.QueryContext(ctx, `
WITH input AS (
  SELECT
    $1::text AS q,
    upper(replace($1::text, '.', '')) AS q_code,
    $2::jsonb AS tokens,
    to_tsquery('italian', NULLIF($4::text, '')) AS tsq
),
tokens AS (
  SELECT value AS token
  FROM input, jsonb_array_elements_text(input.tokens)
  WHERE length(value) >= 2
),
ranked AS (
  SELECT
    ateco.ordine,
    ateco.codice,
    ateco.codice_search,
    ateco.titolo,
    ateco.gerarchia,
    ateco.numero_corrispondenze,
    (
      CASE
        WHEN upper(ateco.codice) = upper(input.q) THEN 1000
        WHEN upper(ateco.codice_search) = input.q_code THEN 950
        ELSE 0
      END
      + CASE WHEN lower(ateco.titolo) LIKE '%' || lower(input.q) || '%' THEN 120 ELSE 0 END
      + CASE
          WHEN input.tsq IS NOT NULL AND to_tsvector('italian', ateco.titolo) @@ input.tsq
            THEN 80 + (ts_rank_cd(to_tsvector('italian', ateco.titolo), input.tsq) * 100)
          ELSE 0
        END
      + COALESCE((
          SELECT count(*) * 20
          FROM tokens
          WHERE lower(ateco.titolo) LIKE '%' || tokens.token || '%'
        ), 0)
      + (similarity(lower(ateco.titolo), lower(input.q)) * 30)
    ) AS rank_score
  FROM binocolo.codici_ateco_2025 ateco
  CROSS JOIN input
  WHERE ateco.codice_search ~ '^[0-9]+$'
)
SELECT ordine, codice, codice_search, titolo, gerarchia, numero_corrispondenze
FROM ranked
WHERE rank_score > 0
ORDER BY rank_score DESC, COALESCE(gerarchia, 0) DESC, ordine ASC
LIMIT $3
`, query, []byte(tokensRaw), limit, tsQuery)
	if err != nil {
		return nil, fmt.Errorf("search ateco: %w", err)
	}
	defer rows.Close()
	out := []AtecoCode{}
	for rows.Next() {
		item, err := scanAtecoScanner(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ateco search: %w", err)
	}
	return out, nil
}

type atecoScanner interface {
	Scan(...any) error
}

func scanAtecoRow(row *sql.Row) (AtecoCode, error) {
	return scanAtecoScanner(row)
}

func scanAtecoScanner(scanner atecoScanner) (AtecoCode, error) {
	var item AtecoCode
	var gerarchia sql.NullInt64
	if err := scanner.Scan(
		&item.Ordine,
		&item.Codice,
		&item.CodiceSearch,
		&item.Titolo,
		&gerarchia,
		&item.NumeroCorrispondenze,
	); err != nil {
		return AtecoCode{}, err
	}
	item.Codice = strings.TrimSpace(item.Codice)
	item.CodiceSearch = strings.TrimSpace(item.CodiceSearch)
	item.Titolo = strings.TrimSpace(item.Titolo)
	if gerarchia.Valid {
		value := int(gerarchia.Int64)
		item.Gerarchia = &value
	}
	return item, nil
}

func normalizeAtecoSearchLimit(limit int) int {
	if limit <= 0 {
		return atecoSearchDefaultLimit
	}
	if limit > atecoSearchHardLimit {
		return atecoSearchHardLimit
	}
	return limit
}

func atecoSearchCode(code string) string {
	return strings.ReplaceAll(normalizeAtecoCode(code), ".", "")
}

func atecoSearchTokens(query string) []string {
	rawTokens := atecoTokenPattern.FindAllString(strings.ToLower(query), -1)
	seen := map[string]struct{}{}
	out := make([]string, 0, len(rawTokens)+8)
	add := func(token string) {
		token = strings.ToLower(strings.TrimSpace(token))
		if token == "" || atecoSearchStopwords[token] {
			return
		}
		if _, exists := seen[token]; exists {
			return
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	for _, token := range rawTokens {
		add(token)
		for _, expanded := range atecoSearchSynonyms[token] {
			add(expanded)
		}
	}
	return out
}

func atecoTSQuery(tokens []string) string {
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(strings.ToLower(token))
		if token == "" {
			continue
		}
		parts = append(parts, token+":*")
	}
	return strings.Join(parts, " | ")
}

var atecoSearchStopwords = map[string]bool{
	"attivita": true,
	"attività": true,
	"azienda":  true,
	"aziende":  true,
	"dei":      true,
	"dell":     true,
	"della":    true,
	"delle":    true,
	"servizi":  true,
	"servizio": true,
}

var atecoSearchSynonyms = map[string][]string{
	"ict": {"informatica", "informatiche", "informazione", "software", "programmazione", "consulenza", "hosting", "elaborazione"},
	"it":  {"informatica", "informatiche", "informazione", "software", "programmazione", "consulenza", "hosting", "elaborazione"},
	"cloud": {
		"hosting",
		"infrastrutture",
		"informatiche",
	},
}
