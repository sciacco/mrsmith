package binocolo

// Google Drive document bindings for the Binocolo domain (#98 executing PRD #85;
// first consumer of the shared Google Drive client #97).
//
// Drive is the source of truth: Binocolo only owns the company→folder and
// card→subfolder bindings (migration 127). Bindings are created lazily on first
// access and are write-once. The shared googledrive.Service (#97) is injected as
// a soft dependency: when absent, the documents surface degrades to a clean
// not_configured state (503) without blocking the rest of the Scheda.
//
// Concurrency: two concurrent first-access requests for the same company must
// never produce duplicate bindings. The orchestration holds a row lock
// (ma_company / ma_initiative_card) across the check→CreateFolder→persist
// sequence. CreateFolder is the single non-idempotent step and is never retried
// (#97 §9): a crash after create leaves an orphan Drive folder (acceptable),
// never a double binding (the UNIQUE/PK is the backstop).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/googledrive"
)

// maDriveContextRef addresses the binocolo "documenti" context. A row with
// app="binocolo", context="documenti" is inserted by the operator into
// mrsmith.googledrive_context; when missing/disabled the client returns
// CodeNotConfigured (mapped here to 503).
var maDriveContextRef = googledrive.ContextRef{App: "binocolo", Context: "documenti"}

// maDriveFolderMimeType is the Drive mime type of a folder, used to sort
// folders before files in the documents listing.
const maDriveFolderMimeType = "application/vnd.google-apps.folder"

const (
	insertMACompanyDriveFolderQuery = `
INSERT INTO binocolo.ma_company_drive_folder (company_key, drive_folder_id, created_by_subject, created_by_email)
VALUES ($1, $2, $3, $4)`

	insertMACardDriveFolderQuery = `
INSERT INTO binocolo.ma_card_drive_folder (initiative_id, company_key, drive_folder_id, created_by_subject, created_by_email)
VALUES ($1::uuid, $2, $3, $4, $5)`
)

// ---------------------------------------------------------------------------
// Response shapes (camelCase). The frontend branches on the error code strings
// surfaced by maHTTPError, so the documents section renders a graceful state.
// ---------------------------------------------------------------------------

// MACompanyDocuments is the documents panel of the Scheda azienda: the folder's
// browse link plus its direct children (folders first, then files, by name).
// CardFolderID is the BOUND subfolder id of the lens initiative's card (empty
// when no lens/binding): the frontend highlights by id, never by name (PRD
// §3.3 — renames must not break the link, so they must not break the pill).
type MACompanyDocuments struct {
	FolderWebViewLink string              `json:"folderWebViewLink"`
	CardFolderID      string              `json:"cardFolderId,omitempty"`
	Items             []MACompanyDocument `json:"items"`
}

// MACompanyDocument is one Drive child projected for the UI. Trashed children
// are filtered out upstream: Drive's own folder view hides them, and the
// listing mirrors what the analyst sees in Drive.
type MACompanyDocument struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	MimeType    string    `json:"mimeType"`
	ModifiedAt  time.Time `json:"modifiedAt"`
	WebViewLink string    `json:"webViewLink"`
}

// errMADriveNotConfigured is the canonical "Drive integration disabled" error
// raised when the googledrive.Service is not wired (or the context row is
// missing/disabled). It is a *googledrive.Error so maHTTPError maps it to 503.
func errMADriveNotConfigured() error {
	return &googledrive.Error{Code: googledrive.CodeNotConfigured, Op: "binocolo"}
}

// errMADriveTrashed is raised when a bound folder is in the trash. GetItem
// returns trashed items without error (spike-verified in #97), so the check is
// explicit on our side: a trashed folder never silently yields a browse link
// (PRD §9). It is a *googledrive.Error so maHTTPError maps it to 410.
func errMADriveTrashed() error {
	return &googledrive.Error{Code: googledrive.CodeTrashed, Op: "binocolo"}
}

// ---------------------------------------------------------------------------
// Store accessors (*SQLStore). The four binding accessors are part of the
// maWorkspaceStore contract; the name/root helpers are called on the concrete
// store by the orchestration (which type-asserts to *SQLStore to begin the
// lock-spanning transaction).
// ---------------------------------------------------------------------------

// GetMACompanyDriveFolder returns the bound company folder id, or
// ("", sql.ErrNoRows) when no binding exists yet.
func (s *SQLStore) GetMACompanyDriveFolder(ctx context.Context, companyKey string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	var folderID string
	err := s.db.QueryRowContext(ctx, `
SELECT drive_folder_id FROM binocolo.ma_company_drive_folder WHERE company_key = $1`, companyKey).Scan(&folderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", sql.ErrNoRows
		}
		return "", fmt.Errorf("get ma company drive folder: %w", err)
	}
	return folderID, nil
}

// InsertMACompanyDriveFolder records a company binding. Plain INSERT: the
// caller holds the row lock and the PK is the safety net against duplicates.
func (s *SQLStore) InsertMACompanyDriveFolder(ctx context.Context, companyKey, driveFolderID, subject, email string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, insertMACompanyDriveFolderQuery, companyKey, driveFolderID, nullString(subject), nullString(email)); err != nil {
		return fmt.Errorf("insert ma company drive folder: %w", translateMACompanyConstraintError(err, false))
	}
	return nil
}

// GetMACardDriveFolder returns the bound card subfolder id, or
// ("", sql.ErrNoRows) when no binding exists yet.
func (s *SQLStore) GetMACardDriveFolder(ctx context.Context, initiativeID, companyKey string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	var folderID string
	err := s.db.QueryRowContext(ctx, `
SELECT drive_folder_id FROM binocolo.ma_card_drive_folder WHERE initiative_id = $1::uuid AND company_key = $2`, initiativeID, companyKey).Scan(&folderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", sql.ErrNoRows
		}
		return "", fmt.Errorf("get ma card drive folder: %w", err)
	}
	return folderID, nil
}

// InsertMACardDriveFolder records a card subfolder binding. Plain INSERT.
func (s *SQLStore) InsertMACardDriveFolder(ctx context.Context, initiativeID, companyKey, driveFolderID, subject, email string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if _, err := s.db.ExecContext(ctx, insertMACardDriveFolderQuery, initiativeID, companyKey, driveFolderID, nullString(subject), nullString(email)); err != nil {
		return fmt.Errorf("insert ma card drive folder: %w", translateMACompanyConstraintError(err, false))
	}
	return nil
}

// GetMACompanyDriveFolderName reads the company legal name to use as the folder
// name, falling back to the company key when the name is null/empty so folder
// creation never fails on an empty name (the client rejects empty names).
func (s *SQLStore) GetMACompanyDriveFolderName(ctx context.Context, companyKey string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	var name sql.NullString
	err := s.db.QueryRowContext(ctx, `
SELECT company_name FROM binocolo.ma_company WHERE company_key = $1`, companyKey).Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errMACompanyKeyUnknown
		}
		return "", fmt.Errorf("get ma company drive folder name: %w", err)
	}
	if name.Valid && strings.TrimSpace(name.String) != "" {
		return name.String, nil
	}
	return companyKey, nil
}

// maDriveRootFolderID reads the context root folder id from
// mrsmith.googledrive_context. The company folder is created directly under
// this root, and Binocolo needs the id itself (the shared client resolves it
// internally for read/list ops but CreateFolder takes a concrete parent id).
// Missing/disabled context → not_configured (mapped to 503).
func (s *SQLStore) maDriveRootFolderID(ctx context.Context) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	var rootFolderID string
	var enabled bool
	err := s.db.QueryRowContext(ctx, `
SELECT root_folder_id, enabled
FROM mrsmith.googledrive_context
WHERE app = $1 AND context = $2`, maDriveContextRef.App, maDriveContextRef.Context).Scan(&rootFolderID, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errMADriveNotConfigured()
	}
	if err != nil {
		return "", fmt.Errorf("get ma drive root folder id: %w", err)
	}
	if !enabled {
		return "", errMADriveNotConfigured()
	}
	return rootFolderID, nil
}

// ---------------------------------------------------------------------------
// Orchestration (maService).
// ---------------------------------------------------------------------------

// maDriveStore returns the SQL backing store so the drive orchestration can
// hold a transaction + row lock across the single non-idempotent CreateFolder
// call. The production path always wires *SQLStore; a nil/non-SQL store yields
// errMAStoreUnavailable.
func (s *maService) maDriveStore() (*SQLStore, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	store, ok := s.store.(*SQLStore)
	if !ok || store == nil {
		return nil, errMAStoreUnavailable
	}
	return store, nil
}

// ensureCompanyFolder lazily creates the company's root-level Drive folder and
// records the binding. Idempotent: an existing binding is returned without
// touching Drive. The row lock on ma_company spans check→CreateFolder→persist
// so two concurrent first-access requests never produce a duplicate; the PK is
// the backstop. CreateFolder is never retried.
func (s *maService) ensureCompanyFolder(ctx context.Context, companyKey, subject, email string) (folderID, webViewLink string, err error) {
	if s.drive == nil {
		return "", "", errMADriveNotConfigured()
	}
	store, err := s.maDriveStore()
	if err != nil {
		return "", "", err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", fmt.Errorf("begin ensure ma company drive folder: %w", err)
	}
	defer tx.Rollback()

	// Serialize concurrent first-access for the same company.
	if err := lockMAContactCompany(ctx, tx, companyKey); err != nil {
		return "", "", err
	}

	// Existing binding: release the lock immediately, then resolve the browse
	// link. The read on a separate connection is safe — the lock already
	// serializes writes, and a committed binding is visible to any connection.
	existing, err := store.GetMACompanyDriveFolder(ctx, companyKey)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return "", "", fmt.Errorf("commit ensure ma company drive folder (existing): %w", err)
		}
		item, gerr := s.drive.GetItem(ctx, maDriveContextRef, existing)
		if gerr != nil {
			return "", "", gerr
		}
		if item.Trashed {
			return "", "", errMADriveTrashed()
		}
		return existing, item.WebViewLink, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", "", err
	}

	// Absent: keep the transaction + lock open across the Google call (ratified
	// design — worst case is an orphan Drive folder on a crash, never a double
	// binding). The folder sits directly under the context root.
	name, err := store.GetMACompanyDriveFolderName(ctx, companyKey)
	if err != nil {
		return "", "", err
	}
	rootID, err := store.maDriveRootFolderID(ctx)
	if err != nil {
		return "", "", err
	}
	created, err := s.drive.CreateFolder(ctx, maDriveContextRef, rootID, name)
	if err != nil {
		return "", "", err
	}
	// Persist inside the SAME transaction/connection as the lock: a child-table
	// INSERT on a different connection would need a KEY SHARE on the locked
	// ma_company row (FK) and deadlock against the FOR UPDATE held here.
	if _, err := tx.ExecContext(ctx, insertMACompanyDriveFolderQuery, companyKey, created.ID, nullString(subject), nullString(email)); err != nil {
		return "", "", fmt.Errorf("insert ma company drive folder: %w", translateMACompanyConstraintError(err, false))
	}
	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("commit ensure ma company drive folder: %w", err)
	}
	return created.ID, created.WebViewLink, nil
}

// ensureCardFolder lazily creates the card's subfolder (under the company
// folder) and records the binding. The parent company folder is ensured first;
// then the ma_initiative_card row lock spans check→CreateFolder→persist.
func (s *maService) ensureCardFolder(ctx context.Context, initiativeID, companyKey, subject, email string) (folderID, webViewLink string, err error) {
	if s.drive == nil {
		return "", "", errMADriveNotConfigured()
	}
	// Ensure the parent (company) folder first. ensureCompanyFolder commits and
	// releases the company lock before we acquire the card lock, so lock
	// ordering is always company→card and there is no overlap (no deadlock).
	companyFolderID, _, err := s.ensureCompanyFolder(ctx, companyKey, subject, email)
	if err != nil {
		return "", "", err
	}
	store, err := s.maDriveStore()
	if err != nil {
		return "", "", err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", fmt.Errorf("begin ensure ma card drive folder: %w", err)
	}
	defer tx.Rollback()

	// Serialize concurrent first-access for the same card. The handler guards
	// with requireOperationalInitiativeCard first; this is defense-in-depth.
	if err := lockMACardDriveFolder(ctx, tx, initiativeID, companyKey); err != nil {
		return "", "", err
	}

	existing, err := store.GetMACardDriveFolder(ctx, initiativeID, companyKey)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return "", "", fmt.Errorf("commit ensure ma card drive folder (existing): %w", err)
		}
		item, gerr := s.drive.GetItem(ctx, maDriveContextRef, existing)
		if gerr != nil {
			return "", "", gerr
		}
		if item.Trashed {
			return "", "", errMADriveTrashed()
		}
		return existing, item.WebViewLink, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", "", err
	}

	// Absent: the subfolder name is the initiative title (fallback company key).
	name, err := s.cardDriveFolderName(ctx, initiativeID, companyKey)
	if err != nil {
		return "", "", err
	}
	// The company folder sits inside the context root, so the client's boundary
	// check passes when creating under it.
	created, err := s.drive.CreateFolder(ctx, maDriveContextRef, companyFolderID, name)
	if err != nil {
		return "", "", err
	}
	if _, err := tx.ExecContext(ctx, insertMACardDriveFolderQuery, initiativeID, companyKey, created.ID, nullString(subject), nullString(email)); err != nil {
		return "", "", fmt.Errorf("insert ma card drive folder: %w", translateMACompanyConstraintError(err, false))
	}
	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("commit ensure ma card drive folder: %w", err)
	}
	return created.ID, created.WebViewLink, nil
}

// cardDriveFolderName resolves the subfolder name for a card: the initiative
// title, falling back to the company key when the title is empty (defense in
// depth — title is required at creation, so this is effectively unreachable).
func (s *maService) cardDriveFolderName(ctx context.Context, initiativeID, companyKey string) (string, error) {
	initiative, err := s.store.GetMAInitiative(ctx, initiativeID)
	if err != nil {
		return "", err
	}
	if title := strings.TrimSpace(initiative.Title); title != "" {
		return title, nil
	}
	return companyKey, nil
}

// listCompanyDocuments ensures the company folder and lists its direct children,
// sorted with folders first then by name (case-insensitive). The client absorbs
// Drive pagination, so all children are returned. Trashed children are filtered
// out (Drive's own folder view hides them). When initiativeID is non-empty (the
// Scheda's lens), the card's bound subfolder id is included so the frontend
// highlights by id; an absent binding is not an error — no pill is shown.
func (s *maService) listCompanyDocuments(ctx context.Context, companyKey, initiativeID, subject, email string) (MACompanyDocuments, error) {
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return MACompanyDocuments{}, fmt.Errorf("%w: companyKey", errMAStrategyInvalid)
	}
	folderID, folderLink, err := s.ensureCompanyFolder(ctx, companyKey, subject, email)
	if err != nil {
		return MACompanyDocuments{}, err
	}
	children, err := s.drive.ListChildren(ctx, maDriveContextRef, folderID)
	if err != nil {
		return MACompanyDocuments{}, err
	}
	items := make([]MACompanyDocument, 0, len(children))
	for _, c := range children {
		if c.Trashed {
			continue
		}
		items = append(items, MACompanyDocument{
			ID:          c.ID,
			Name:        c.Name,
			MimeType:    c.MimeType,
			ModifiedAt:  c.ModifiedAt,
			WebViewLink: c.WebViewLink,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		fi := items[i].MimeType == maDriveFolderMimeType
		fj := items[j].MimeType == maDriveFolderMimeType
		if fi != fj {
			return fi // folders first
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	docs := MACompanyDocuments{FolderWebViewLink: folderLink, Items: items}
	if initiativeID != "" {
		cardFolderID, err := s.store.GetMACardDriveFolder(ctx, initiativeID, companyKey)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return MACompanyDocuments{}, err
		}
		docs.CardFolderID = cardFolderID
	}
	return docs, nil
}

// lockMACardDriveFolder takes an advisory row lock on the card so concurrent
// first-access for the same (initiative, company) serializes. Missing card →
// errMACardNotFound (the handler guards upstream, so this is defense-in-depth).
func lockMACardDriveFolder(ctx context.Context, tx *sql.Tx, initiativeID, companyKey string) error {
	var exists int
	err := tx.QueryRowContext(ctx, `
SELECT 1 FROM binocolo.ma_initiative_card WHERE initiative_id = $1::uuid AND company_key = $2 FOR UPDATE`, initiativeID, companyKey).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return errMACardNotFound
	}
	if err != nil {
		return fmt.Errorf("lock ma card drive folder: %w", err)
	}
	return nil
}
