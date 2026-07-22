package binocolo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// Store for the deposited-filing pipeline (issue #78). database/sql on the shared
// Anisetta DB, same conventions as ma_store.go: methods on *SQLStore, $n params,
// ::uuid/::jsonb/::date casts, nullString/nullInt for NULLs (maDateArg for date columns),
// jsonb columns carried as []byte, best-effort logging via logging.FromContext. No ORM.
// Column names must track migrations 113/114 exactly.

// ---------------------------------------------------------------------------
// Fiscal-identity normalization.
//
// The FINDER SQL in ma_store.go (the source_clean/source_rows CTEs, ~lines
// 3174-3191) is the source of truth for how a fiscal identity is normalized; these
// helpers replicate it in Go so the filing pipeline keys filings and looks up deep
// analyses with byte-identical values. Any change to the finder normalization MUST be
// mirrored here (and vice versa) — the identity-aware deep enqueue depends on the two
// agreeing.
//
//   *_clean  = regexp_replace(upper(btrim(x)), '[[:space:].]', '', 'g')
//   stable   = strip a leading "IT" when the clean value is IT followed by 11 digits
//   fiscal_key = stable(vat) when non-empty, else clean(tax); empty when both blank
// ---------------------------------------------------------------------------

var (
	// maFiscalNormalizeRe strips every whitespace character and every dot, matching the
	// Postgres character class [[:space:].] used by the finder's regexp_replace.
	maFiscalNormalizeRe = regexp.MustCompile(`[[:space:].]`)
	// maStableVATRe matches a cleaned VAT that is the literal "IT" plus exactly 11 digits;
	// such values have their IT prefix stripped to the bare 11-digit stable key.
	maStableVATRe = regexp.MustCompile(`^IT[0-9]{11}$`)
)

// normalizeMAFiscalValue mirrors the finder's *_clean expression: uppercase, trim, and
// remove all whitespace and dots.
func normalizeMAFiscalValue(s string) string {
	return maFiscalNormalizeRe.ReplaceAllString(strings.ToUpper(strings.TrimSpace(s)), "")
}

// maStableVAT mirrors the finder's stable_key for a VAT: the cleaned value with a
// leading "IT" removed when the value is IT + 11 digits (an Italian VAT written with the
// country prefix). Idempotent; safe on an already-clean input.
func maStableVAT(vat string) string {
	clean := normalizeMAFiscalValue(vat)
	if maStableVATRe.MatchString(clean) {
		return clean[2:]
	}
	return clean
}

// buildMAFiscalKey is the canonical filing identity: the stable VAT when present, else
// the cleaned tax code. Empty when both inputs are blank (the caller must reject that —
// a filing/analysis without a fiscal identity has no key).
func buildMAFiscalKey(vat, tax string) string {
	if v := maStableVAT(vat); v != "" {
		return v
	}
	return normalizeMAFiscalValue(tax)
}

// maDateArg formats a nullable date-column value as a timezone-neutral 'YYYY-MM-DD'
// string (NULL for nil), for use with a `$n::date` cast. Passing a string rather than a
// time.Time (which pgx would encode as timestamptz) keeps the stored date independent of
// the session timezone — the same string+::date pattern used by InsertMADeepVintage.
func maDateArg(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format("2006-01-02")
}

// maDateValue is the non-nullable counterpart of maDateArg for lookup/comparison params.
func maDateValue(value time.Time) string {
	return value.Format("2006-01-02")
}

// jsonbArg carries a jsonb column value: nil (SQL NULL) for an empty slice, else the
// bytes verbatim. An empty non-nil []byte would otherwise be sent as ” and rejected by
// the jsonb type, so callers signal "leave unset" by passing nil/empty.
func jsonbArg(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return []byte(b)
}

// ---------------------------------------------------------------------------
// ma_filing_blob — PDF deduplicated by digest (md5 hex lowercase canonical).
// ---------------------------------------------------------------------------

// InsertMAFilingBlob stores a PDF keyed by its canonical (hex lowercase) md5. size_bytes
// is derived from the payload. ON CONFLICT (md5) DO NOTHING makes re-storing the same
// bytes a no-op, so the blob is written exactly once regardless of how many filings share
// it. sha256 is a redundant cross-check digest, stored when provided.
func (s *SQLStore) InsertMAFilingBlob(ctx context.Context, md5Hex, sha256Hex string, pdf []byte) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_filing_blob (md5, sha256, size_bytes, pdf)
VALUES ($1, NULLIF($2, ''), $3, $4)
ON CONFLICT (md5) DO NOTHING
`, md5Hex, sha256Hex, int64(len(pdf)), pdf); err != nil {
		return fmt.Errorf("insert ma filing blob: %w", err)
	}
	return nil
}

// GetMAFilingBlob returns the stored PDF, its sha256, and size for a canonical md5.
// Returns (nil, "", 0, nil) when absent.
func (s *SQLStore) GetMAFilingBlob(ctx context.Context, md5Hex string) ([]byte, string, int64, error) {
	if s == nil || s.db == nil {
		return nil, "", 0, errors.New("binocolo ma store not configured")
	}
	var pdf []byte
	var sha256 string
	var size int64
	err := s.db.QueryRowContext(ctx, `
SELECT pdf, COALESCE(sha256, ''), COALESCE(size_bytes, 0)
FROM binocolo.ma_filing_blob
WHERE md5 = $1
`, md5Hex).Scan(&pdf, &sha256, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", 0, nil
	}
	if err != nil {
		return nil, "", 0, fmt.Errorf("get ma filing blob: %w", err)
	}
	return pdf, sha256, size, nil
}

// ExistsMAFilingBlob reports whether a blob with this canonical md5 is already stored,
// so a channel can skip a re-download when the bytes are on hand.
func (s *SQLStore) ExistsMAFilingBlob(ctx context.Context, md5Hex string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
SELECT EXISTS (SELECT 1 FROM binocolo.ma_filing_blob WHERE md5 = $1)
`, md5Hex).Scan(&exists); err != nil {
		return false, fmt.Errorf("check ma filing blob exists: %w", err)
	}
	return exists, nil
}

// ---------------------------------------------------------------------------
// ma_filing — the fascicolo, identified by fiscal identity (never company_key).
// ---------------------------------------------------------------------------

// maFiling mirrors every column of binocolo.ma_filing. Nullable date columns surface as
// *time.Time (nil when NULL); page_count as *int.
type maFiling struct {
	ID                        string
	FiscalKey                 string
	VATClean                  string
	TaxClean                  string
	ClosingDate               *time.Time
	BalanceSheetID            string
	BalanceSheetType          string
	TaxonomyVersion           string
	IdentityStatus            string
	IdentityOverrideBySubject string
	IdentityOverrideByEmail   string
	IdentityOverrideReason    string
	IdentityOverrideAt        *time.Time
	Status                    string
	BlobMD5                   string
	ActiveProcessingRunID     string
	PageCount                 *int
	Error                     string
	CreatedBySubject          string
	CreatedByEmail            string
	CreatedAt                 time.Time
}

// maFilingCreate is the input to CreateOrMergeMAFiling. VAT/Tax are raw (normalized
// inside); the DocuEngine channel supplies ClosingDate + BalanceSheetID at creation
// while the upload channel leaves them zero and discovers ClosingDate at parse.
type maFilingCreate struct {
	VAT              string
	Tax              string
	ClosingDate      *time.Time
	BalanceSheetID   string
	BalanceSheetType string
	TaxonomyVersion  string
	BlobMD5          string
	PageCount        *int
	CreatedBySubject string
	CreatedByEmail   string
}

const maFilingColumns = `id::text, fiscal_key, COALESCE(vat_clean, ''), COALESCE(tax_clean, ''), closing_date,
	COALESCE(balance_sheet_id, ''), COALESCE(balance_sheet_type, ''), COALESCE(taxonomy_version, ''),
	identity_status, COALESCE(identity_override_by_subject, ''), COALESCE(identity_override_by_email, ''),
	COALESCE(identity_override_reason, ''), identity_override_at, status, COALESCE(blob_md5, ''),
	COALESCE(active_processing_run_id::text, ''), page_count, COALESCE(error, ''),
	COALESCE(created_by_subject, ''), COALESCE(created_by_email, ''), created_at`

func scanMAFiling(scanner interface{ Scan(...any) error }) (maFiling, error) {
	var f maFiling
	var closingDate, overrideAt sql.NullTime
	var pageCount sql.NullInt64
	if err := scanner.Scan(
		&f.ID, &f.FiscalKey, &f.VATClean, &f.TaxClean, &closingDate,
		&f.BalanceSheetID, &f.BalanceSheetType, &f.TaxonomyVersion,
		&f.IdentityStatus, &f.IdentityOverrideBySubject, &f.IdentityOverrideByEmail,
		&f.IdentityOverrideReason, &overrideAt, &f.Status, &f.BlobMD5,
		&f.ActiveProcessingRunID, &pageCount, &f.Error,
		&f.CreatedBySubject, &f.CreatedByEmail, &f.CreatedAt,
	); err != nil {
		return maFiling{}, err
	}
	if closingDate.Valid {
		f.ClosingDate = &closingDate.Time
	}
	if overrideAt.Valid {
		f.IdentityOverrideAt = &overrideAt.Time
	}
	if pageCount.Valid {
		v := int(pageCount.Int64)
		f.PageCount = &v
	}
	return f, nil
}

// CreateOrMergeMAFiling implements the issue's cross-channel merge in one transaction:
//
//	(a) a filing already exists for (fiscal_key, blob_md5) → merged; if the incoming
//	    balance_sheet_id is set and the row lacks one, enrich it (id, type, and
//	    closing_date only when still NULL — never clobber DocuEngine values);
//	(b) else, if the incoming balance_sheet_id + closing_date match an existing
//	    (fiscal_key, closing_date, balance_sheet_id) → merged; a different blob for the
//	    same deposit does NOT overwrite blob_md5, the existing filing is returned as-is;
//	(c) else INSERT a new filing (status queued, identity pending_validation).
//
// A concurrent insert that wins one of the two unique indexes surfaces as ON CONFLICT
// DO NOTHING returning no row; the method then re-reads and falls back into the merge.
// Returns the filing id and whether it merged into an existing row.
func (s *SQLStore) CreateOrMergeMAFiling(ctx context.Context, in maFilingCreate) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, errors.New("binocolo ma store not configured")
	}
	vatClean := normalizeMAFiscalValue(in.VAT)
	taxClean := normalizeMAFiscalValue(in.Tax)
	fiscalKey := buildMAFiscalKey(in.VAT, in.Tax)
	if fiscalKey == "" {
		return "", false, errors.New("create ma filing: empty fiscal identity (vat and tax both blank)")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("begin create ma filing: %w", err)
	}
	defer tx.Rollback()

	id, merged, err := mergeOrInsertMAFiling(ctx, tx, in, vatClean, taxClean, fiscalKey)
	if err != nil {
		return "", false, err
	}
	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("commit create ma filing: %w", err)
	}
	return id, merged, nil
}

func mergeOrInsertMAFiling(ctx context.Context, tx *sql.Tx, in maFilingCreate, vatClean, taxClean, fiscalKey string) (string, bool, error) {
	// (a) same identity + same blob.
	if in.BlobMD5 != "" {
		if id, found, err := findMAFilingByBlobTx(ctx, tx, fiscalKey, in.BlobMD5); err != nil {
			return "", false, err
		} else if found {
			if err := enrichMAFilingBalanceSheet(ctx, tx, id, in); err != nil {
				return "", false, err
			}
			return id, true, nil
		}
	}
	// (b) same identity + same deposit (different blob): keep the existing filing.
	if in.BalanceSheetID != "" && in.ClosingDate != nil {
		if id, found, err := findMAFilingByBalanceSheetTx(ctx, tx, fiscalKey, *in.ClosingDate, in.BalanceSheetID); err != nil {
			return "", false, err
		} else if found {
			return id, true, nil
		}
	}
	// (c) new filing.
	var id string
	err := tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_filing
  (fiscal_key, vat_clean, tax_clean, closing_date, balance_sheet_id, balance_sheet_type, taxonomy_version, blob_md5, page_count, created_by_subject, created_by_email)
VALUES
  ($1, NULLIF($2, ''), NULLIF($3, ''), $4::date, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9, NULLIF($10, ''), NULLIF($11, ''))
ON CONFLICT DO NOTHING
RETURNING id::text
`, fiscalKey, vatClean, taxClean, maDateArg(in.ClosingDate), in.BalanceSheetID, in.BalanceSheetType, in.TaxonomyVersion, in.BlobMD5, nullInt(in.PageCount), in.CreatedBySubject, in.CreatedByEmail).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, fmt.Errorf("insert ma filing: %w", err)
	}
	// A concurrent insert won a unique index: re-read and merge into it.
	if in.BlobMD5 != "" {
		if id, found, err := findMAFilingByBlobTx(ctx, tx, fiscalKey, in.BlobMD5); err != nil {
			return "", false, err
		} else if found {
			if err := enrichMAFilingBalanceSheet(ctx, tx, id, in); err != nil {
				return "", false, err
			}
			return id, true, nil
		}
	}
	if in.BalanceSheetID != "" && in.ClosingDate != nil {
		if id, found, err := findMAFilingByBalanceSheetTx(ctx, tx, fiscalKey, *in.ClosingDate, in.BalanceSheetID); err != nil {
			return "", false, err
		} else if found {
			return id, true, nil
		}
	}
	return "", false, fmt.Errorf("insert ma filing: unique conflict without a matching existing row (fiscal_key=%s)", fiscalKey)
}

func findMAFilingByBlobTx(ctx context.Context, tx *sql.Tx, fiscalKey, blobMD5 string) (string, bool, error) {
	var id string
	err := tx.QueryRowContext(ctx, `
SELECT id::text FROM binocolo.ma_filing
WHERE fiscal_key = $1 AND blob_md5 = $2
LIMIT 1
`, fiscalKey, blobMD5).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("lookup ma filing by blob: %w", err)
	}
	return id, true, nil
}

func findMAFilingByBalanceSheetTx(ctx context.Context, tx *sql.Tx, fiscalKey string, closing time.Time, balanceSheetID string) (string, bool, error) {
	var id string
	err := tx.QueryRowContext(ctx, `
SELECT id::text FROM binocolo.ma_filing
WHERE fiscal_key = $1 AND closing_date = $2::date AND balance_sheet_id = $3
LIMIT 1
`, fiscalKey, maDateValue(closing), balanceSheetID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("lookup ma filing by balance sheet: %w", err)
	}
	return id, true, nil
}

// enrichMAFilingBalanceSheet fills the balance-sheet identity onto a filing that matched
// by blob but had none (typically an upload row later matched by a DocuEngine acquire of
// the same bytes). It only writes when balance_sheet_id is still NULL, sets closing_date
// only when still NULL, and is skipped entirely when the target values would collide with
// the partial UNIQUE (fiscal_key, closing_date, balance_sheet_id) on another row.
func enrichMAFilingBalanceSheet(ctx context.Context, tx *sql.Tx, id string, in maFilingCreate) error {
	if in.BalanceSheetID == "" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_filing f
SET balance_sheet_id = $2,
    balance_sheet_type = COALESCE(f.balance_sheet_type, NULLIF($3, '')),
    closing_date = COALESCE(f.closing_date, $4::date)
WHERE f.id = $1::uuid
  AND f.balance_sheet_id IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM binocolo.ma_filing g
    WHERE g.id <> f.id
      AND g.fiscal_key = f.fiscal_key
      AND g.balance_sheet_id = $2
      AND g.closing_date IS NOT DISTINCT FROM COALESCE(f.closing_date, $4::date)
  )
`, id, in.BalanceSheetID, in.BalanceSheetType, maDateArg(in.ClosingDate)); err != nil {
		return fmt.Errorf("enrich ma filing balance sheet: %w", err)
	}
	return nil
}

// GetMAFiling returns a filing by id, or (nil, nil) when absent.
func (s *SQLStore) GetMAFiling(ctx context.Context, id string) (*maFiling, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	f, err := scanMAFiling(s.db.QueryRowContext(ctx, `SELECT `+maFilingColumns+` FROM binocolo.ma_filing WHERE id = $1::uuid`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ma filing: %w", err)
	}
	return &f, nil
}

// ListMAFilingsByFiscalKey returns every filing for a fiscal identity, most recent
// exercise first (NULL closing dates — upload rows not yet parsed — sort last).
func (s *SQLStore) ListMAFilingsByFiscalKey(ctx context.Context, fiscalKey string) ([]maFiling, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+maFilingColumns+`
FROM binocolo.ma_filing
WHERE fiscal_key = $1
ORDER BY closing_date DESC NULLS LAST, created_at DESC`, fiscalKey)
	if err != nil {
		return nil, fmt.Errorf("list ma filings by fiscal key: %w", err)
	}
	defer rows.Close()
	out := []maFiling{}
	for rows.Next() {
		f, err := scanMAFiling(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ma filing: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma filings: %w", err)
	}
	return out, nil
}

// UpdateMAFilingStatus moves the ingest pipeline status and sets (or clears, when errMsg
// is empty) the error column. The status machine itself is owned by the caller (F4/F5).
func (s *SQLStore) UpdateMAFilingStatus(ctx context.Context, id, status, errMsg string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing
SET status = $2, error = NULLIF($3, '')
WHERE id = $1::uuid
`, id, status, errMsg); err != nil {
		return fmt.Errorf("update ma filing status: %w", err)
	}
	return nil
}

// SetMAFilingParsed writes the fields discovered at parse — closing_date, balance sheet
// type, taxonomy version, page count — but only when the column is still NULL/empty, so
// values the DocuEngine channel already wrote at creation are never overwritten.
func (s *SQLStore) SetMAFilingParsed(ctx context.Context, id string, closingDate *time.Time, balanceSheetType, taxonomyVersion string, pageCount int) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing
SET closing_date = COALESCE(closing_date, $2::date),
    balance_sheet_type = COALESCE(NULLIF(balance_sheet_type, ''), NULLIF($3, '')),
    taxonomy_version = COALESCE(NULLIF(taxonomy_version, ''), NULLIF($4, '')),
    page_count = COALESCE(page_count, $5)
WHERE id = $1::uuid
`, id, maDateArg(closingDate), balanceSheetType, taxonomyVersion, nullIntValue(pageCount)); err != nil {
		return fmt.Errorf("set ma filing parsed: %w", err)
	}
	return nil
}

// SetMAFilingIdentityStatus sets the identity_status column (separate from the pipeline
// status). The DB CHECK enforces the allowed values.
func (s *SQLStore) SetMAFilingIdentityStatus(ctx context.Context, id, identityStatus string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing
SET identity_status = $2
WHERE id = $1::uuid
`, id, identityStatus); err != nil {
		return fmt.Errorf("set ma filing identity status: %w", err)
	}
	return nil
}

// ApplyMAFilingIdentityOverride records a one-shot, non-revocable manual acceptance of a
// filing whose fiscal identity could NOT be READ from page 1. Set-once: it succeeds (rows
// affected == 1) only while identity_override_at IS NULL.
//
// Contract (orchestrator, correcting the earlier F3 decision): the override resolves ONLY the
// unreadable soft-block, i.e. a filing that the ingest already parked at status
// 'identity_blocked' with identity_status 'pending_validation' (page 1 could not be read). Both
// guards are required:
//   - identity_status = 'pending_validation' is ALSO the transient pre-gate state of a freshly
//     created upload; without the status guard an override fired before the first ingest tick
//     would silently skip the page-1 validation entirely.
//   - status = 'identity_blocked' proves the gate ran and blocked on an unreadable identity.
//
// A 'mismatch' — a readable page-1 identity that DIFFERS from the expected one — is a HARD BLOCK
// and is NEVER overridable ("Mismatch leggibile = blocco duro; illeggibile = override esplicito
// auditabile"): accepting a document that self-declares a different company would defeat the
// identity guarantee. 'validated' needs no override and 'override' is already terminal.
func (s *SQLStore) ApplyMAFilingIdentityOverride(ctx context.Context, id, subject, email, reason string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing
SET identity_status = 'override',
    identity_override_by_subject = NULLIF($2, ''),
    identity_override_by_email = NULLIF($3, ''),
    identity_override_reason = NULLIF($4, ''),
    identity_override_at = now()
WHERE id = $1::uuid
  AND identity_override_at IS NULL
  AND identity_status = 'pending_validation'
  AND status = 'identity_blocked'
`, id, subject, email, reason)
	if err != nil {
		return false, fmt.Errorf("apply ma filing identity override: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("apply ma filing identity override rows: %w", err)
	}
	return affected == 1, nil
}

// SetMAFilingActiveProcessingRun points the filing at its current processing run.
// CreateMAFilingProcessingRun already does this in its transaction; this is the standalone
// setter for callers that adopt an existing run.
func (s *SQLStore) SetMAFilingActiveProcessingRun(ctx context.Context, id, runID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing
SET active_processing_run_id = $2::uuid
WHERE id = $1::uuid
`, id, runID); err != nil {
		return fmt.Errorf("set ma filing active processing run: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ma_filing_search — durable DocuEngine search state machine.
// ---------------------------------------------------------------------------

// maFilingSearch mirrors binocolo.ma_filing_search. Results carries the opaque result_id
// handles + balanceSheetId + type + closing date as jsonb.
type maFilingSearch struct {
	ID                      string
	FiscalKey               string
	Status                  string
	ConsumedByAcquisitionID string
	DocuEngineRequestID     string
	Results                 json.RawMessage
	SearchedBySubject       string
	SearchedByEmail         string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

const maFilingSearchColumns = `id::text, fiscal_key, status, COALESCE(consumed_by_acquisition_id::text, ''),
	COALESCE(docuengine_request_id, ''), results, COALESCE(searched_by_subject, ''),
	COALESCE(searched_by_email, ''), created_at, updated_at`

func scanMAFilingSearch(scanner interface{ Scan(...any) error }) (maFilingSearch, error) {
	var s maFilingSearch
	var results []byte
	if err := scanner.Scan(
		&s.ID, &s.FiscalKey, &s.Status, &s.ConsumedByAcquisitionID,
		&s.DocuEngineRequestID, &results, &s.SearchedBySubject,
		&s.SearchedByEmail, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return maFilingSearch{}, err
	}
	if len(results) > 0 {
		s.Results = json.RawMessage(results)
	}
	return s, nil
}

// CreateMAFilingSearchIntent opens a search state machine at 'intent' — the durable row
// written BEFORE the DocuEngine POST, so a crash after the vendor call is reconcilable.
func (s *SQLStore) CreateMAFilingSearchIntent(ctx context.Context, fiscalKey, subject, email string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	var id string
	if err := s.db.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_filing_search (fiscal_key, status, searched_by_subject, searched_by_email)
VALUES ($1, 'intent', NULLIF($2, ''), NULLIF($3, ''))
RETURNING id::text
`, fiscalKey, subject, email).Scan(&id); err != nil {
		return "", fmt.Errorf("create ma filing search intent: %w", err)
	}
	return id, nil
}

// SetMAFilingSearchRequested records the DocuEngine request id once the search has been
// submitted (intent → requested). Gated on the source status so a stale caller can't move
// a search that has already advanced.
func (s *SQLStore) SetMAFilingSearchRequested(ctx context.Context, id, docuRequestID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_search
SET status = 'requested', docuengine_request_id = NULLIF($2, ''), updated_at = now()
WHERE id = $1::uuid AND status = 'intent'
`, id, docuRequestID); err != nil {
		return fmt.Errorf("set ma filing search requested: %w", err)
	}
	return nil
}

// SetMAFilingSearchResults stores the vendor results (already serialized jsonb) and moves
// requested → results.
func (s *SQLStore) SetMAFilingSearchResults(ctx context.Context, id string, results []byte) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_search
SET status = 'results', results = $2::jsonb, updated_at = now()
WHERE id = $1::uuid AND status = 'requested'
`, id, jsonbArg(results)); err != nil {
		return fmt.Errorf("set ma filing search results: %w", err)
	}
	return nil
}

// SetMAFilingSearchUnknown marks an indeterminate vendor outcome (intent/requested →
// unknown). ma_filing_search has no error column (migration 113), so errMsg is logged for
// diagnostics rather than persisted.
func (s *SQLStore) SetMAFilingSearchUnknown(ctx context.Context, id, errMsg string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if errMsg != "" {
		logging.FromContext(ctx).Warn("binocolo ma filing search unknown", "component", "binocolo", "operation", "ma_filing_search", "search_id", id, "error", errMsg)
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_search
SET status = 'unknown', updated_at = now()
WHERE id = $1::uuid AND status IN ('intent', 'requested')
`, id); err != nil {
		return fmt.Errorf("set ma filing search unknown: %w", err)
	}
	return nil
}

// FailMAFilingSearch marks a non-consumed search failed. As with the unknown transition,
// errMsg is logged (no error column on ma_filing_search).
func (s *SQLStore) FailMAFilingSearch(ctx context.Context, id, errMsg string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if errMsg != "" {
		logging.FromContext(ctx).Warn("binocolo ma filing search failed", "component", "binocolo", "operation", "ma_filing_search", "search_id", id, "error", errMsg)
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_search
SET status = 'failed', updated_at = now()
WHERE id = $1::uuid AND status <> 'consumed'
`, id); err != nil {
		return fmt.Errorf("fail ma filing search: %w", err)
	}
	return nil
}

// AdoptMAFilingSearchRequest reconciles an 'unknown' search back to 'requested' once the
// vendor request id has been matched client-side (name + timestamp window + readableSearch
// taxCode). unknown → requested.
func (s *SQLStore) AdoptMAFilingSearchRequest(ctx context.Context, id, docuRequestID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_search
SET status = 'requested', docuengine_request_id = NULLIF($2, ''), updated_at = now()
WHERE id = $1::uuid AND status = 'unknown'
`, id, docuRequestID); err != nil {
		return fmt.Errorf("adopt ma filing search request: %w", err)
	}
	return nil
}

// ClaimMAFilingSearchConsumed is a CAS that binds a search's results to the acquisition
// that consumes them: results → consumed, exactly once. Returns true only for the caller
// that won (rows affected == 1), so two acquisitions can't both consume one search.
func (s *SQLStore) ClaimMAFilingSearchConsumed(ctx context.Context, searchID, acquisitionID string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_search
SET status = 'consumed', consumed_by_acquisition_id = $2::uuid, updated_at = now()
WHERE id = $1::uuid AND status = 'results' AND consumed_by_acquisition_id IS NULL
`, searchID, acquisitionID)
	if err != nil {
		return false, fmt.Errorf("claim ma filing search consumed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim ma filing search consumed rows: %w", err)
	}
	return affected == 1, nil
}

// GetMAFilingSearch returns a search by id, or (nil, nil) when absent.
func (s *SQLStore) GetMAFilingSearch(ctx context.Context, id string) (*maFilingSearch, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rec, err := scanMAFilingSearch(s.db.QueryRowContext(ctx, `SELECT `+maFilingSearchColumns+` FROM binocolo.ma_filing_search WHERE id = $1::uuid`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ma filing search: %w", err)
	}
	return &rec, nil
}

// ListMAFilingReferencedRequestIDs returns the set of DocuEngine request ids already
// referenced by a search or acquisition row (excluding the given search/acquisition id,
// passed empty when N/A). The filing jobs use it as the exclusion set for pre-POST
// reconciliation: a request already owned by another row must never be adopted, so only a
// genuinely orphaned request (e.g. a prior attempt whose persist failed after a paid POST)
// can be reused instead of paying for a duplicate.
func (s *SQLStore) ListMAFilingReferencedRequestIDs(ctx context.Context, excludeSearchID, excludeAcquisitionID string) (map[string]bool, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT docuengine_request_id FROM binocolo.ma_filing_search
WHERE docuengine_request_id IS NOT NULL AND ($1 = '' OR id <> $1::uuid)
UNION
SELECT docuengine_request_id FROM binocolo.ma_filing_acquisition
WHERE docuengine_request_id IS NOT NULL AND ($2 = '' OR id <> $2::uuid)
`, excludeSearchID, excludeAcquisitionID)
	if err != nil {
		return nil, fmt.Errorf("list ma filing referenced request ids: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan ma filing referenced request id: %w", err)
		}
		if id != "" {
			out[id] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma filing referenced request ids: %w", err)
	}
	return out, nil
}

// GetLatestMAFilingSearchWithResults returns the most recent search for a fiscal identity
// that has results ready (status='results') — what the UI offers for acquisition. Returns
// (nil, nil) when none.
func (s *SQLStore) GetLatestMAFilingSearchWithResults(ctx context.Context, fiscalKey string) (*maFilingSearch, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rec, err := scanMAFilingSearch(s.db.QueryRowContext(ctx, `SELECT `+maFilingSearchColumns+`
FROM binocolo.ma_filing_search
WHERE fiscal_key = $1 AND status = 'results'
ORDER BY updated_at DESC, created_at DESC
LIMIT 1`, fiscalKey))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest ma filing search with results: %w", err)
	}
	return &rec, nil
}

// ---------------------------------------------------------------------------
// ma_filing_acquisition — one attempt to procure a filing (upload or DocuEngine).
// ---------------------------------------------------------------------------

// maFilingAcquisition mirrors binocolo.ma_filing_acquisition.
type maFilingAcquisition struct {
	ID                  string
	FilingID            string
	Origin              string
	Status              string
	DocuEngineRequestID string
	ResultID            string
	BalanceSheetID      string
	ContextCompanyKey   string
	ActorSubject        string
	ActorEmail          string
	Error               string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// maFilingAcquisitionCreate is the input to CreateMAFilingAcquisitionIntent. The row is
// written BEFORE any vendor call; context_company_key is a non-identifying reference to the
// company the analyst was looking at.
type maFilingAcquisitionCreate struct {
	Origin            string
	BalanceSheetID    string
	ResultID          string
	ContextCompanyKey string
	ActorSubject      string
	ActorEmail        string
}

const maFilingAcquisitionColumns = `id::text, COALESCE(filing_id::text, ''), origin, status,
	COALESCE(docuengine_request_id, ''), COALESCE(result_id, ''), COALESCE(balance_sheet_id, ''),
	COALESCE(context_company_key, ''), COALESCE(actor_subject, ''), COALESCE(actor_email, ''),
	COALESCE(error, ''), created_at, updated_at`

func scanMAFilingAcquisition(scanner interface{ Scan(...any) error }) (maFilingAcquisition, error) {
	var a maFilingAcquisition
	if err := scanner.Scan(
		&a.ID, &a.FilingID, &a.Origin, &a.Status,
		&a.DocuEngineRequestID, &a.ResultID, &a.BalanceSheetID,
		&a.ContextCompanyKey, &a.ActorSubject, &a.ActorEmail,
		&a.Error, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return maFilingAcquisition{}, err
	}
	return a, nil
}

// CreateMAFilingAcquisitionIntent opens an acquisition at 'intent' — the durable row that
// precedes every vendor call, so a crash mid-acquire leaves an auditable trace.
func (s *SQLStore) CreateMAFilingAcquisitionIntent(ctx context.Context, in maFilingAcquisitionCreate) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	var id string
	if err := s.db.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_filing_acquisition
  (origin, status, result_id, balance_sheet_id, context_company_key, actor_subject, actor_email)
VALUES
  ($1, 'intent', NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''))
RETURNING id::text
`, in.Origin, in.ResultID, in.BalanceSheetID, in.ContextCompanyKey, in.ActorSubject, in.ActorEmail).Scan(&id); err != nil {
		return "", fmt.Errorf("create ma filing acquisition intent: %w", err)
	}
	return id, nil
}

// SetMAFilingAcquisitionRequested records the DocuEngine request id (intent → requested).
func (s *SQLStore) SetMAFilingAcquisitionRequested(ctx context.Context, id, docuRequestID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_acquisition
SET status = 'requested', docuengine_request_id = NULLIF($2, ''), updated_at = now()
WHERE id = $1::uuid AND status = 'intent'
`, id, docuRequestID); err != nil {
		return fmt.Errorf("set ma filing acquisition requested: %w", err)
	}
	return nil
}

// SetMAFilingAcquisitionDownloaded marks the PDF as fetched (requested → downloaded).
func (s *SQLStore) SetMAFilingAcquisitionDownloaded(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_acquisition
SET status = 'downloaded', updated_at = now()
WHERE id = $1::uuid AND status = 'requested'
`, id); err != nil {
		return fmt.Errorf("set ma filing acquisition downloaded: %w", err)
	}
	return nil
}

// CompleteMAFilingAcquisition binds the acquisition to its filing and marks it done. It
// completes from any non-terminal state (upload skips requested/downloaded), never from an
// already-failed row.
func (s *SQLStore) CompleteMAFilingAcquisition(ctx context.Context, id, filingID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_acquisition
SET status = 'done', filing_id = $2::uuid, updated_at = now()
WHERE id = $1::uuid AND status IN ('intent', 'requested', 'downloaded', 'unknown')
`, id, filingID); err != nil {
		return fmt.Errorf("complete ma filing acquisition: %w", err)
	}
	return nil
}

// FailMAFilingAcquisition marks a non-done acquisition failed with an error message
// (ma_filing_acquisition has an error column, unlike ma_filing_search).
func (s *SQLStore) FailMAFilingAcquisition(ctx context.Context, id, errMsg string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_acquisition
SET status = 'failed', error = NULLIF($2, ''), updated_at = now()
WHERE id = $1::uuid AND status <> 'done'
`, id, errMsg); err != nil {
		return fmt.Errorf("fail ma filing acquisition: %w", err)
	}
	return nil
}

// SetMAFilingAcquisitionUnknown marks an indeterminate vendor outcome (intent/requested →
// unknown), never resolved heuristically — only via AdoptMAFilingAcquisitionRequest.
func (s *SQLStore) SetMAFilingAcquisitionUnknown(ctx context.Context, id, errMsg string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_acquisition
SET status = 'unknown', error = NULLIF($2, ''), updated_at = now()
WHERE id = $1::uuid AND status IN ('intent', 'requested')
`, id, errMsg); err != nil {
		return fmt.Errorf("set ma filing acquisition unknown: %w", err)
	}
	return nil
}

// AdoptMAFilingAcquisitionRequest reconciles an 'unknown' acquisition to 'requested' once
// its vendor request id has been matched (unknown → requested).
func (s *SQLStore) AdoptMAFilingAcquisitionRequest(ctx context.Context, id, docuRequestID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_acquisition
SET status = 'requested', docuengine_request_id = NULLIF($2, ''), updated_at = now()
WHERE id = $1::uuid AND status = 'unknown'
`, id, docuRequestID); err != nil {
		return fmt.Errorf("adopt ma filing acquisition request: %w", err)
	}
	return nil
}

// GetMAFilingAcquisition returns an acquisition by id, or (nil, nil) when absent.
func (s *SQLStore) GetMAFilingAcquisition(ctx context.Context, id string) (*maFilingAcquisition, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rec, err := scanMAFilingAcquisition(s.db.QueryRowContext(ctx, `SELECT `+maFilingAcquisitionColumns+` FROM binocolo.ma_filing_acquisition WHERE id = $1::uuid`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ma filing acquisition: %w", err)
	}
	return &rec, nil
}

// LinkMAFilingAcquisitionFiling attaches a filing to an acquisition without changing its
// status — the upload channel's immediate link once the filing row exists.
func (s *SQLStore) LinkMAFilingAcquisitionFiling(ctx context.Context, id, filingID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_acquisition
SET filing_id = $2::uuid, updated_at = now()
WHERE id = $1::uuid
`, id, filingID); err != nil {
		return fmt.Errorf("link ma filing acquisition filing: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ma_filing_processing_run + ma_filing_page + ma_filing_extract — immutable OCR/parse.
// ---------------------------------------------------------------------------

// maFilingPage is one OCR page of a processing run (markdown + optional structured extras).
type maFilingPage struct {
	ProcessingRunID string
	PageNo          int
	Markdown        string
	Extras          json.RawMessage
}

// maFilingExtract is the per-exercise CEE extract of a processing run: stato patrimoniale
// (sp), conto economico (ce), quadrature (checks), Document AI reading (docai) and the
// diff of docai vs the deterministic parse (docai_diff).
type maFilingExtract struct {
	ProcessingRunID string
	ExerciseDate    time.Time
	SP              json.RawMessage
	CE              json.RawMessage
	Checks          json.RawMessage
	DocAI           json.RawMessage
	DocAIDiff       json.RawMessage
}

// CreateMAFilingProcessingRun opens a new immutable OCR/parse run for a filing: in one
// transaction it supersedes the filing's prior active run(s), inserts the new run as
// active, and points ma_filing.active_processing_run_id at it. Returns the new run id.
func (s *SQLStore) CreateMAFilingProcessingRun(ctx context.Context, filingID, ocrModel, ocrParamsVersion, parseVersion, requestID string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin create ma filing processing run: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_filing_processing_run
SET status = 'superseded'
WHERE filing_id = $1::uuid AND status = 'active'
`, filingID); err != nil {
		return "", fmt.Errorf("supersede ma filing processing runs: %w", err)
	}

	var id string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_filing_processing_run
  (filing_id, ocr_model, ocr_params_version, parse_version, status, request_id)
VALUES
  ($1::uuid, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), 'active', NULLIF($5, ''))
RETURNING id::text
`, filingID, ocrModel, ocrParamsVersion, parseVersion, requestID).Scan(&id); err != nil {
		return "", fmt.Errorf("insert ma filing processing run: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_filing
SET active_processing_run_id = $2::uuid
WHERE id = $1::uuid
`, filingID, id); err != nil {
		return "", fmt.Errorf("point ma filing at active processing run: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit create ma filing processing run: %w", err)
	}
	return id, nil
}

// InsertMAFilingPages batch-inserts the OCR pages of a run. A run is immutable, so
// ON CONFLICT (processing_run_id, page_no) DO NOTHING makes a re-run of the same insert a
// no-op. A single multi-row INSERT keeps it atomic.
func (s *SQLStore) InsertMAFilingPages(ctx context.Context, runID string, pages []maFilingPage) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if len(pages) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString(`INSERT INTO binocolo.ma_filing_page (processing_run_id, page_no, markdown, extras) VALUES `)
	args := []any{runID}
	for i, p := range pages {
		if i > 0 {
			b.WriteString(", ")
		}
		base := len(args)
		fmt.Fprintf(&b, "($1::uuid, $%d, $%d, $%d::jsonb)", base+1, base+2, base+3)
		args = append(args, p.PageNo, p.Markdown, jsonbArg(p.Extras))
	}
	b.WriteString(" ON CONFLICT (processing_run_id, page_no) DO NOTHING")
	if _, err := s.db.ExecContext(ctx, b.String(), args...); err != nil {
		return fmt.Errorf("insert ma filing pages: %w", err)
	}
	return nil
}

// GetMAFilingPages returns the OCR pages of a run in page order.
func (s *SQLStore) GetMAFilingPages(ctx context.Context, runID string) ([]maFilingPage, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT processing_run_id::text, page_no, COALESCE(markdown, ''), extras
FROM binocolo.ma_filing_page
WHERE processing_run_id = $1::uuid
ORDER BY page_no
`, runID)
	if err != nil {
		return nil, fmt.Errorf("get ma filing pages: %w", err)
	}
	defer rows.Close()
	out := []maFilingPage{}
	for rows.Next() {
		var p maFilingPage
		var extras []byte
		if err := rows.Scan(&p.ProcessingRunID, &p.PageNo, &p.Markdown, &extras); err != nil {
			return nil, fmt.Errorf("scan ma filing page: %w", err)
		}
		if len(extras) > 0 {
			p.Extras = json.RawMessage(extras)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma filing pages: %w", err)
	}
	return out, nil
}

// UpsertMAFilingExtract writes a per-exercise CEE extract for a run. The extract is written
// in stages (parse writes sp/ce/checks, then the docai stage writes docai/docai_diff), so
// ON CONFLICT DO UPDATE COALESCEs each column: a nil/empty argument leaves the stored value
// untouched rather than clearing it.
func (s *SQLStore) UpsertMAFilingExtract(ctx context.Context, runID string, exerciseDate time.Time, sp, ce, checks, docai, docaiDiff []byte) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_filing_extract (processing_run_id, exercise_date, sp, ce, checks, docai, docai_diff)
VALUES ($1::uuid, $2::date, $3::jsonb, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb)
ON CONFLICT (processing_run_id, exercise_date) DO UPDATE SET
  sp = COALESCE(EXCLUDED.sp, binocolo.ma_filing_extract.sp),
  ce = COALESCE(EXCLUDED.ce, binocolo.ma_filing_extract.ce),
  checks = COALESCE(EXCLUDED.checks, binocolo.ma_filing_extract.checks),
  docai = COALESCE(EXCLUDED.docai, binocolo.ma_filing_extract.docai),
  docai_diff = COALESCE(EXCLUDED.docai_diff, binocolo.ma_filing_extract.docai_diff)
`, runID, maDateValue(exerciseDate), jsonbArg(sp), jsonbArg(ce), jsonbArg(checks), jsonbArg(docai), jsonbArg(docaiDiff)); err != nil {
		return fmt.Errorf("upsert ma filing extract: %w", err)
	}
	return nil
}

// GetMAFilingExtracts returns the per-exercise extracts of a run, oldest exercise first.
func (s *SQLStore) GetMAFilingExtracts(ctx context.Context, runID string) ([]maFilingExtract, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT processing_run_id::text, exercise_date, sp, ce, checks, docai, docai_diff
FROM binocolo.ma_filing_extract
WHERE processing_run_id = $1::uuid
ORDER BY exercise_date
`, runID)
	if err != nil {
		return nil, fmt.Errorf("get ma filing extracts: %w", err)
	}
	defer rows.Close()
	out := []maFilingExtract{}
	for rows.Next() {
		var e maFilingExtract
		var sp, ce, checks, docai, docaiDiff []byte
		if err := rows.Scan(&e.ProcessingRunID, &e.ExerciseDate, &sp, &ce, &checks, &docai, &docaiDiff); err != nil {
			return nil, fmt.Errorf("scan ma filing extract: %w", err)
		}
		if len(sp) > 0 {
			e.SP = json.RawMessage(sp)
		}
		if len(ce) > 0 {
			e.CE = json.RawMessage(ce)
		}
		if len(checks) > 0 {
			e.Checks = json.RawMessage(checks)
		}
		if len(docai) > 0 {
			e.DocAI = json.RawMessage(docai)
		}
		if len(docaiDiff) > 0 {
			e.DocAIDiff = json.RawMessage(docaiDiff)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma filing extracts: %w", err)
	}
	return out, nil
}

// SetMAFilingPageCount records the OCR page count on a filing (authoritative from the
// OCR stage, unlike SetMAFilingParsed which only fills it when still NULL).
func (s *SQLStore) SetMAFilingPageCount(ctx context.Context, id string, pageCount int) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing
SET page_count = $2
WHERE id = $1::uuid
`, id, pageCount); err != nil {
		return fmt.Errorf("set ma filing page count: %w", err)
	}
	return nil
}

// HasMAFilingDocuEngineAcquisition reports whether a filing was procured through the
// DocuEngine channel (an acquisition with origin='docuengine' that reached
// downloaded/done). The ingest treats such a filing's fiscal identity as ASSERTED by
// the channel (the search matched on the codice fiscale), so page-1 identity validation
// is skipped — only uploads must prove identity from the document itself.
func (s *SQLStore) HasMAFilingDocuEngineAcquisition(ctx context.Context, filingID string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM binocolo.ma_filing_acquisition
  WHERE filing_id = $1::uuid AND origin = 'docuengine' AND status IN ('downloaded', 'done')
)
`, filingID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check ma filing docuengine acquisition: %w", err)
	}
	return exists, nil
}

// GetMAFilingExtractsByFiscalKey returns the SP total-attivo of the ACTIVE processing
// run of OTHER filings sharing this fiscal identity, restricted to the given exercise
// dates. It feeds detectAdaptedComparative: a prior deposit may restate a comparative
// column that diverges from a later fascicolo. Excludes the caller's own filing.
// exerciseDates empty ⇒ no work (returns nil). The numeric is read straight from the
// stored sp jsonb (json tag totaleAttivo), so the pure helper needs no re-parse.
func (s *SQLStore) GetMAFilingExtractsByFiscalKey(ctx context.Context, fiscalKey, excludeFilingID string, exerciseDates []time.Time) ([]maFilingPeerExtract, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	if len(exerciseDates) == 0 {
		return nil, nil
	}
	dates := make([]string, 0, len(exerciseDates))
	for _, d := range exerciseDates {
		dates = append(dates, maDateValue(d))
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT e.exercise_date, f.closing_date, (e.sp->>'totaleAttivo')::float8
FROM binocolo.ma_filing_extract e
JOIN binocolo.ma_filing_processing_run r
  ON r.id = e.processing_run_id AND r.status = 'active'
JOIN binocolo.ma_filing f
  ON f.id = r.filing_id
WHERE f.fiscal_key = $1
  AND ($2 = '' OR f.id <> $2::uuid)
  AND e.exercise_date = ANY(string_to_array($3, ',')::date[])
`, fiscalKey, excludeFilingID, strings.Join(dates, ","))
	if err != nil {
		return nil, fmt.Errorf("get ma filing extracts by fiscal key: %w", err)
	}
	defer rows.Close()
	var out []maFilingPeerExtract
	for rows.Next() {
		var peer maFilingPeerExtract
		var closing sql.NullTime
		var totale sql.NullFloat64
		if err := rows.Scan(&peer.ExerciseDate, &closing, &totale); err != nil {
			return nil, fmt.Errorf("scan ma filing peer extract: %w", err)
		}
		if closing.Valid {
			peer.ClosingDate = &closing.Time
		}
		if totale.Valid {
			v := totale.Float64
			peer.TotaleAttivo = &v
		}
		out = append(out, peer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma filing peer extracts: %w", err)
	}
	return out, nil
}

// maFilingIngestOrphan is one filing the sweeper found stuck in a non-terminal ingest
// state with no job working it. Carries what enqueueFilingIngest needs to re-drive it.
type maFilingIngestOrphan struct {
	FilingID          string
	FiscalKey         string
	ContextCompanyKey string
	ActorSubject      string
	ActorEmail        string
}

// SweepMAFilingIngestOrphans returns filings in a non-terminal ingest state
// (queued/ocr/parse) that have NO filing_ingest job inflight (pending/processing) and
// were created more than olderThan ago. The worker re-enqueues each (idempotent via the
// mig-115 inflight index) to recover the QA-F4 gap where a post-acquire ingest enqueue
// failed. context_company_key/actor are recovered from the filing's most recent
// acquisition so the re-enqueued job matches the original. identity_blocked/failed/
// ready/degraded are terminal or await manual action and are never swept.
func (s *SQLStore) SweepMAFilingIngestOrphans(ctx context.Context, olderThan time.Duration) ([]maFilingIngestOrphan, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	minutes := int(olderThan.Minutes())
	if minutes < 0 {
		minutes = 0
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT f.id::text, f.fiscal_key,
       COALESCE(a.context_company_key, ''), COALESCE(a.actor_subject, ''), COALESCE(a.actor_email, '')
FROM binocolo.ma_filing f
LEFT JOIN LATERAL (
  SELECT context_company_key, actor_subject, actor_email
  FROM binocolo.ma_filing_acquisition
  WHERE filing_id = f.id
  ORDER BY updated_at DESC
  LIMIT 1
) a ON true
WHERE f.status IN ('queued', 'ocr', 'parse')
  AND f.created_at < now() - make_interval(mins => $1)
  AND NOT EXISTS (
    SELECT 1 FROM binocolo.ma_job j
    WHERE j.job_type = 'filing_ingest'
      AND j.status IN ('pending', 'processing')
      AND j.payload->>'filingId' = f.id::text
  )
`, minutes)
	if err != nil {
		return nil, fmt.Errorf("sweep ma filing ingest orphans: %w", err)
	}
	defer rows.Close()
	var out []maFilingIngestOrphan
	for rows.Next() {
		var o maFilingIngestOrphan
		if err := rows.Scan(&o.FilingID, &o.FiscalKey, &o.ContextCompanyKey, &o.ActorSubject, &o.ActorEmail); err != nil {
			return nil, fmt.Errorf("scan ma filing ingest orphan: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma filing ingest orphans: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// F8 read helpers (HTTP endpoints) — SELECT-only projections for the scheda.
// ---------------------------------------------------------------------------

// GetMAFilingByBlob resolves the filing for a fiscal identity + canonical blob md5 (the
// upload dedup key), or (nil, nil) when none. Read-only sibling of the tx-scoped
// findMAFilingByBlobTx: the upload endpoint uses it to short-circuit a re-ingest on a
// bit-identical re-upload without opening the create/merge transaction.
func (s *SQLStore) GetMAFilingByBlob(ctx context.Context, fiscalKey, blobMD5 string) (*maFiling, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	f, err := scanMAFiling(s.db.QueryRowContext(ctx, `SELECT `+maFilingColumns+`
FROM binocolo.ma_filing
WHERE fiscal_key = $1 AND blob_md5 = $2
LIMIT 1`, fiscalKey, blobMD5))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ma filing by blob: %w", err)
	}
	return &f, nil
}

// ListMAFilingOrigins returns, per filing id, the DISTINCT acquisition origins that produced
// it (upload | docuengine, ordered), so the scheda can badge how a fascicolo entered. Empty
// input ⇒ no query; a filing with no acquisition row (should not happen) simply maps to nil.
func (s *SQLStore) ListMAFilingOrigins(ctx context.Context, filingIDs []string) (map[string][]string, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	out := map[string][]string{}
	if len(filingIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT filing_id::text, origin
FROM binocolo.ma_filing_acquisition
WHERE filing_id = ANY(string_to_array($1, ',')::uuid[])
ORDER BY filing_id::text, origin`, strings.Join(filingIDs, ","))
	if err != nil {
		return nil, fmt.Errorf("list ma filing origins: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var filingID, origin string
		if err := rows.Scan(&filingID, &origin); err != nil {
			return nil, fmt.Errorf("scan ma filing origin: %w", err)
		}
		out[filingID] = append(out[filingID], origin)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma filing origins: %w", err)
	}
	return out, nil
}

// GetLatestMAFilingSearch returns the most recent search for a fiscal identity regardless of
// status (the fallback when no search sits at 'results'), or (nil, nil) when none.
func (s *SQLStore) GetLatestMAFilingSearch(ctx context.Context, fiscalKey string) (*maFilingSearch, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rec, err := scanMAFilingSearch(s.db.QueryRowContext(ctx, `SELECT `+maFilingSearchColumns+`
FROM binocolo.ma_filing_search
WHERE fiscal_key = $1
ORDER BY updated_at DESC, created_at DESC
LIMIT 1`, fiscalKey))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest ma filing search: %w", err)
	}
	return &rec, nil
}

// GetInflightMAFilingSearchID returns the searchId carried by the currently in-flight
// filing_search job for a fiscal identity (pending|processing), or "" when none is in flight.
// The search endpoint uses it to report the LIVE search rather than the just-created intent row
// (which, on an enqueue dedup, is a harmless orphan that no job references — F8 pendenza).
func (s *SQLStore) GetInflightMAFilingSearchID(ctx context.Context, fiscalKey string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	var searchID string
	err := s.db.QueryRowContext(ctx, `
SELECT payload->>'searchId'
FROM binocolo.ma_job
WHERE job_type = $2
  AND status IN ('pending', 'processing')
  AND payload->>'fiscalKey' = $1
ORDER BY created_at DESC
LIMIT 1`, fiscalKey, maJobTypeFilingSearch).Scan(&searchID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get inflight ma filing search id: %w", err)
	}
	return strings.TrimSpace(searchID), nil
}

// maFilingAcquisitionOpen is one still-open acquisition (procurement not yet bound to a
// filing) the scheda surfaces so an "in acquisizione" — or a stalled/failed one — never
// disappears from the UI. closingDate/balanceSheetType are resolved at READ time from the
// linked search jsonb (no dedicated columns, no migration); HasInflightJob distinguishes a
// live procurement from a dead one (poll_timeout / attempts exhausted) that needs a manual
// resume. NO price is ever carried.
type maFilingAcquisitionOpen struct {
	ID               string
	Status           string
	BalanceSheetID   string
	ClosingDate      string // resolved at read (YYYY-MM-DD); "" when the deposit isn't in any search
	BalanceSheetType string
	Error            string
	HasInflightJob   bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// maFilingSearchDeposit is the (closingDate, type) of one deposited balance sheet, resolved
// from a search's stored results jsonb for the open-acquisition view.
type maFilingSearchDeposit struct {
	closingDate      string
	balanceSheetType string
}

// ListMAFilingAcquisitionsOpen returns EVERY still-open acquisition for a fiscal identity: any
// row with filing_id IS NULL still in a non-terminal-for-the-filing state
// (intent/requested/downloaded/unknown/failed). Unlike the old inflight view it does NOT hide a
// row whose filing_acquire job has died — the row is selected via ANY filing_acquire job (any
// status) that carries its acquisitionId + this fiscalKey (the only fiscal linkage an
// acquisition has before it binds a filing), while HasInflightJob is a SEPARATE EXISTS over the
// pending|processing jobs. So a job killed by poll_timeout leaves a visible, recoverable row.
// closingDate/balanceSheetType are resolved in-process from the linked search's results jsonb
// (consumed-by-this-acquisition first, else the most recent search whose results carry the
// balance sheet). Ordered oldest-first for a stable list.
func (s *SQLStore) ListMAFilingAcquisitionsOpen(ctx context.Context, fiscalKey string) ([]maFilingAcquisitionOpen, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT a.id::text, a.status, COALESCE(a.balance_sheet_id, ''), COALESCE(a.error, ''),
       a.created_at, a.updated_at,
       EXISTS (
         SELECT 1 FROM binocolo.ma_job ij
         WHERE ij.job_type = $2
           AND ij.status IN ('pending', 'processing')
           AND ij.payload->>'acquisitionId' = a.id::text
       ) AS has_inflight_job
FROM binocolo.ma_filing_acquisition a
WHERE a.filing_id IS NULL
  AND a.status IN ('intent', 'requested', 'downloaded', 'unknown', 'failed')
  AND EXISTS (
    SELECT 1 FROM binocolo.ma_job j
    WHERE j.job_type = $2
      AND j.payload->>'acquisitionId' = a.id::text
      AND j.payload->>'fiscalKey' = $1
  )
ORDER BY a.created_at`, fiscalKey, maJobTypeFilingAcquire)
	if err != nil {
		return nil, fmt.Errorf("list ma filing acquisitions open: %w", err)
	}
	defer rows.Close()
	out := []maFilingAcquisitionOpen{}
	for rows.Next() {
		var a maFilingAcquisitionOpen
		if err := rows.Scan(&a.ID, &a.Status, &a.BalanceSheetID, &a.Error, &a.CreatedAt, &a.UpdatedAt, &a.HasInflightJob); err != nil {
			return nil, fmt.Errorf("scan ma filing acquisition open: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma filing acquisitions open: %w", err)
	}
	if len(out) == 0 {
		return out, nil
	}
	if err := s.resolveMAFilingAcquisitionDeposits(ctx, fiscalKey, out); err != nil {
		return nil, err
	}
	return out, nil
}

// resolveMAFilingAcquisitionDeposits fills closingDate/balanceSheetType on each open acquisition
// from the fiscal key's search results jsonb, at read time (no persisted column). The search
// this acquisition consumed wins; otherwise the most recent search whose results carry the
// balance sheet id. Best-effort per row: an unresolvable deposit simply stays blank.
func (s *SQLStore) resolveMAFilingAcquisitionDeposits(ctx context.Context, fiscalKey string, acqs []maFilingAcquisitionOpen) error {
	searchRows, err := s.db.QueryContext(ctx, `
SELECT COALESCE(consumed_by_acquisition_id::text, ''), COALESCE(results, '[]'::jsonb)
FROM binocolo.ma_filing_search
WHERE fiscal_key = $1 AND status IN ('results', 'consumed')
ORDER BY updated_at DESC, created_at DESC`, fiscalKey)
	if err != nil {
		return fmt.Errorf("list ma filing searches for deposit resolve: %w", err)
	}
	defer searchRows.Close()
	// Parsed searches kept in updated_at DESC order so the fallback picks the most recent match.
	type parsedSearch struct {
		consumedByAcqID string
		byBSID          map[string]maFilingSearchDeposit
	}
	var searches []parsedSearch
	for searchRows.Next() {
		var consumedByAcqID string
		var results []byte
		if err := searchRows.Scan(&consumedByAcqID, &results); err != nil {
			return fmt.Errorf("scan ma filing search for deposit resolve: %w", err)
		}
		byBSID := map[string]maFilingSearchDeposit{}
		var parsed []maFilingSearchResultRow
		if len(results) > 0 {
			_ = json.Unmarshal(results, &parsed) // defensive: a malformed row simply resolves nothing
		}
		for _, r := range parsed {
			if r.BalanceSheetID == "" {
				continue
			}
			deposit := maFilingSearchDeposit{balanceSheetType: r.BalanceSheetType}
			if t, ok := parseMAFilingDate(r.BalanceSheetDate); ok {
				deposit.closingDate = maDateValue(t)
			} else {
				deposit.closingDate = strings.TrimSpace(r.BalanceSheetDate)
			}
			byBSID[r.BalanceSheetID] = deposit
		}
		searches = append(searches, parsedSearch{consumedByAcqID: consumedByAcqID, byBSID: byBSID})
	}
	if err := searchRows.Err(); err != nil {
		return fmt.Errorf("iterate ma filing searches for deposit resolve: %w", err)
	}
	for i := range acqs {
		bsid := acqs[i].BalanceSheetID
		if bsid == "" {
			continue
		}
		// Direct link: the search this acquisition consumed.
		resolved := false
		for _, sr := range searches {
			if sr.consumedByAcqID == acqs[i].ID {
				if d, ok := sr.byBSID[bsid]; ok {
					acqs[i].ClosingDate, acqs[i].BalanceSheetType = d.closingDate, d.balanceSheetType
					resolved = true
				}
				break
			}
		}
		if resolved {
			continue
		}
		// Fallback: the most recent search (searches are DESC) carrying this balance sheet.
		for _, sr := range searches {
			if d, ok := sr.byBSID[bsid]; ok {
				acqs[i].ClosingDate, acqs[i].BalanceSheetType = d.closingDate, d.balanceSheetType
				break
			}
		}
	}
	return nil
}

// HasInflightMAFilingAcquireJob reports whether a pending|processing filing_acquire job carries
// this acquisitionId — i.e. the acquisition is actively being worked. The retry endpoint uses it
// as the "already in flight ⇒ not retryable (409)" guard so an explicit resume never races a
// live job (the mig-115 inflight index would dedup the enqueue anyway, but the 409 is legible).
func (s *SQLStore) HasInflightMAFilingAcquireJob(ctx context.Context, acquisitionID string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM binocolo.ma_job
  WHERE job_type = $2
    AND status IN ('pending', 'processing')
    AND payload->>'acquisitionId' = $1
)`, acquisitionID, maJobTypeFilingAcquire).Scan(&exists); err != nil {
		return false, fmt.Errorf("has inflight ma filing acquire job: %w", err)
	}
	return exists, nil
}

// GetMAFilingSearchIDConsumedByAcquisition returns the id of the search this acquisition
// consumed (its linked search), or "" when it consumed none. The retry rebuilds the acquire
// payload's searchId from it (matching startFilingAcquisitions), so a resume re-uses the same
// vendor request via the consumed-by-me short-circuit instead of opening a new one.
func (s *SQLStore) GetMAFilingSearchIDConsumedByAcquisition(ctx context.Context, acquisitionID string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	var id string
	err := s.db.QueryRowContext(ctx, `
SELECT id::text FROM binocolo.ma_filing_search
WHERE consumed_by_acquisition_id = $1::uuid
LIMIT 1`, acquisitionID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get ma filing search consumed by acquisition: %w", err)
	}
	return strings.TrimSpace(id), nil
}

// ReopenMAFilingAcquisitionUnknown moves a FAILED acquisition that still holds a DocuEngine
// request id back to 'unknown' (a RESUME: the reconcile path GETs the persisted request and
// adopts it — it NEVER re-POSTs, so no re-charge). This — with ReopenMAFilingAcquisitionIntent —
// is the ONLY sanctioned mutation OUT of the terminal 'failed' state, and only ever on the
// analyst's explicit retry gesture. Guarded on status='failed' AND filing_id IS NULL AND a
// non-null request id so it can never disturb a completed or clean row; the error is cleared for
// the fresh attempt.
func (s *SQLStore) ReopenMAFilingAcquisitionUnknown(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_acquisition
SET status = 'unknown', error = NULL, updated_at = now()
WHERE id = $1::uuid AND status = 'failed' AND filing_id IS NULL AND docuengine_request_id IS NOT NULL
`, id); err != nil {
		return fmt.Errorf("reopen ma filing acquisition unknown: %w", err)
	}
	return nil
}

// ReopenMAFilingAcquisitionIntent moves a FAILED acquisition with NO persisted request id back
// to 'intent' (a CLEAN restart: the intent path reconciles free before paying — the analyst's
// explicit gesture authorises a possible fresh spend, like a deep-dive retry). Guarded on
// status='failed' AND filing_id IS NULL AND a null request id (the request-id case belongs to
// ReopenMAFilingAcquisitionUnknown); the error is cleared for the fresh attempt.
func (s *SQLStore) ReopenMAFilingAcquisitionIntent(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_filing_acquisition
SET status = 'intent', error = NULL, updated_at = now()
WHERE id = $1::uuid AND status = 'failed' AND filing_id IS NULL AND docuengine_request_id IS NULL
`, id); err != nil {
		return fmt.Errorf("reopen ma filing acquisition intent: %w", err)
	}
	return nil
}

// GetMAFilingLatestAcquisitionContext returns the context_company_key of the most recent
// acquisition bound to a filing ("" when none / all blank). It backs the identity-override
// re-enqueue: the resumed ingest needs a context company key to trigger the baseline deep-dive
// (an empty one is tolerated — the enqueue guard simply skips the baseline, leaving a degraded
// filing rather than a wrong-identity baseline).
func (s *SQLStore) GetMAFilingLatestAcquisitionContext(ctx context.Context, filingID string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	var ctxKey string
	err := s.db.QueryRowContext(ctx, `
SELECT COALESCE(context_company_key, '')
FROM binocolo.ma_filing_acquisition
WHERE filing_id = $1::uuid
ORDER BY updated_at DESC
LIMIT 1`, filingID).Scan(&ctxKey)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get ma filing latest acquisition context: %w", err)
	}
	return strings.TrimSpace(ctxKey), nil
}
