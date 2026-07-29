// Package googledrive is the shared, app-neutral Google Shared Drive client
// for MrSmith mini-apps (#97). It is configured entirely from the DB
// (mrsmith.googledrive_credential + mrsmith.googledrive_context, migration
// 126), authenticates with a single direct Service Account (no DWD, no
// impersonation) and enforces each context's root folder as a hard boundary.
//
// The package knows Drive, credentials, contexts and roots; it knows nothing
// about consumer entities, domain naming or bindings, and exposes no HTTP
// endpoints. It is injected into consumer domains.
package googledrive

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

const component = "googledrive"

// itemFields is the minimal metadata surface (#97 §7) plus what the root
// boundary check needs (parents, driveId).
const itemFields = "id, name, mimeType, modifiedTime, webViewLink, trashed, parents, driveId"

// maxAncestryDepth caps the parent walk of the root boundary check. Deeper
// resources are treated as outside the context root.
const maxAncestryDepth = 20

// ContextRef addresses one documentary context of one mini-app, resolved from
// mrsmith.googledrive_context on every call.
type ContextRef struct {
	App     string
	Context string
}

// Item is the neutral Drive resource exposed to consumers. Trashed resources
// are still returned by Google with Trashed=true (spike-verified), so the
// flag is part of the contract; operations that would work *inside* a trashed
// container fail with CodeTrashed instead.
type Item struct {
	ID          string
	Name        string
	MimeType    string
	ModifiedAt  time.Time
	WebViewLink string
	Trashed     bool
}

// Service is the shared Google Drive client. Construct once in main and
// inject into consumer domains.
type Service struct {
	db     *sql.DB
	logger *slog.Logger

	mu     sync.Mutex
	client *cachedClient
}

// New builds the Service over the Anisetta DB handle (mrsmith schema). A nil
// db yields a Service whose operations return not_configured.
func New(db *sql.DB, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{db: db, logger: logger}
}

// GetItem returns one resource by ID after verifying it belongs to the
// context's Shared Drive and root subtree.
func (s *Service) GetItem(ctx context.Context, ref ContextRef, itemID string) (Item, error) {
	const op = "GetItem"
	call := s.begin(ctx, op, ref, slog.String("item_id", itemID))
	rc, err := s.resolveContext(ctx, op, ref)
	if err != nil {
		return Item{}, call.fail(err)
	}
	files, err := call.files(rc)
	if err != nil {
		return Item{}, call.fail(err)
	}
	f, err := s.fetchInRoot(call, files, rc, itemID)
	if err != nil {
		return Item{}, call.fail(err)
	}
	return toItem(f), nil
}

// ListChildren returns the direct children of a folder inside the context
// root, transparently walking Drive pagination. Trashed children are included
// with Trashed=true; a trashed folder itself is an error (CodeTrashed).
func (s *Service) ListChildren(ctx context.Context, ref ContextRef, folderID string) ([]Item, error) {
	const op = "ListChildren"
	call := s.begin(ctx, op, ref, slog.String("folder_id", folderID))
	rc, err := s.resolveContext(ctx, op, ref)
	if err != nil {
		return nil, call.fail(err)
	}
	files, err := call.files(rc)
	if err != nil {
		return nil, call.fail(err)
	}
	folder, err := s.fetchInRoot(call, files, rc, folderID)
	if err != nil {
		return nil, call.fail(err)
	}
	if folder.Trashed {
		return nil, call.fail(newError(op, CodeTrashed, "folder is in the trash", nil))
	}

	query := fmt.Sprintf("'%s' in parents", strings.ReplaceAll(folderID, "'", `\'`))
	var items []Item
	pageToken := ""
	for {
		list, err := retryRead(call, func() (*drive.FileList, error) {
			req := files.List().
				Q(query).
				Corpora("drive").
				DriveId(rc.SharedDriveID).
				IncludeItemsFromAllDrives(true).
				SupportsAllDrives(true).
				PageSize(1000).
				Fields("nextPageToken", googleapi.Field("files("+itemFields+")")).
				Context(call.ctx)
			if pageToken != "" {
				req = req.PageToken(pageToken)
			}
			return req.Do()
		})
		if err != nil {
			return nil, call.fail(mapUpstreamError(op, err))
		}
		for _, f := range list.Files {
			items = append(items, toItem(f))
		}
		if list.NextPageToken == "" {
			return items, nil
		}
		pageToken = list.NextPageToken
	}
}

// CreateFolder creates a folder under a parent that must sit inside the
// context root. The create call itself is never retried after an ambiguous
// failure (#97 §9): retrying could duplicate the folder, and reconciliation
// belongs to the consumer domain.
func (s *Service) CreateFolder(ctx context.Context, ref ContextRef, parentID, name string) (Item, error) {
	const op = "CreateFolder"
	call := s.begin(ctx, op, ref, slog.String("parent_id", parentID), slog.String("name", name))
	if strings.TrimSpace(name) == "" {
		return Item{}, call.fail(newError(op, CodeInvalidUpstreamResponse, "empty folder name", nil))
	}
	rc, err := s.resolveContext(ctx, op, ref)
	if err != nil {
		return Item{}, call.fail(err)
	}
	files, err := call.files(rc)
	if err != nil {
		return Item{}, call.fail(err)
	}
	parent, err := s.fetchInRoot(call, files, rc, parentID)
	if err != nil {
		return Item{}, call.fail(err)
	}
	if parent.Trashed {
		return Item{}, call.fail(newError(op, CodeTrashed, "parent folder is in the trash", nil))
	}

	created, err := files.Create(&drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentID},
	}).SupportsAllDrives(true).Fields(itemFields).Context(call.ctx).Do()
	if err != nil {
		return Item{}, call.fail(mapUpstreamError(op, err))
	}
	return toItem(created), nil
}

// fetchInRoot loads a resource (with retries) and enforces the hard root
// boundary: same Shared Drive, root itself or one of its descendants.
func (s *Service) fetchInRoot(call *callState, files *drive.FilesService, rc resolvedContext, itemID string) (*drive.File, error) {
	f, err := retryRead(call, func() (*drive.File, error) {
		return files.Get(itemID).SupportsAllDrives(true).Fields(itemFields).Context(call.ctx).Do()
	})
	if err != nil {
		return nil, mapUpstreamError(call.op, err)
	}
	if err := s.ensureWithinRoot(call, files, rc, f); err != nil {
		return nil, err
	}
	return f, nil
}

// ensureWithinRoot walks the parent chain up to the configured root. A
// resource moved elsewhere (even inside the same Shared Drive, where its ID
// and link survive — spike-verified) becomes outside_context_root: it is not
// recreated, not unlinked, just inaccessible to the context.
func (s *Service) ensureWithinRoot(call *callState, files *drive.FilesService, rc resolvedContext, f *drive.File) error {
	if f.DriveId != rc.SharedDriveID {
		return newError(call.op, CodeOutsideContextRoot, "resource belongs to another drive", nil)
	}
	if f.Id == rc.RootFolderID {
		return nil
	}
	cur := f
	for range maxAncestryDepth {
		if len(cur.Parents) == 0 {
			return newError(call.op, CodeOutsideContextRoot, "reached drive top without meeting the context root", nil)
		}
		parentID := cur.Parents[0]
		if parentID == rc.RootFolderID {
			return nil
		}
		if parentID == rc.SharedDriveID {
			return newError(call.op, CodeOutsideContextRoot, "resource hangs from the drive root, not the context root", nil)
		}
		parent, err := retryRead(call, func() (*drive.File, error) {
			return files.Get(parentID).SupportsAllDrives(true).Fields("id, parents, driveId").Context(call.ctx).Do()
		})
		if err != nil {
			return mapUpstreamError(call.op, err)
		}
		cur = parent
	}
	return newError(call.op, CodeOutsideContextRoot, fmt.Sprintf("ancestry deeper than %d levels", maxAncestryDepth), nil)
}

func toItem(f *drive.File) Item {
	modified, _ := time.Parse(time.RFC3339, f.ModifiedTime)
	return Item{
		ID:          f.Id,
		Name:        f.Name,
		MimeType:    f.MimeType,
		ModifiedAt:  modified,
		WebViewLink: f.WebViewLink,
		Trashed:     f.Trashed,
	}
}

// callState carries one application-level operation: capture of every HTTP
// exchange (retries and ancestry walks included), retry counters and the
// attrs that end up in the diagnostic event on failure.
type callState struct {
	svc      *Service
	ctx      context.Context
	op       string
	ref      ContextRef
	attrs    []slog.Attr
	capture  *captureLog
	started  time.Time
	attempts int
	rc       *resolvedContext
	fp       string
	client   *drive.Service
}

func (s *Service) begin(ctx context.Context, op string, ref ContextRef, attrs ...slog.Attr) *callState {
	cctx, capture := withCapture(ctx)
	return &callState{
		svc:     s,
		ctx:     cctx,
		op:      op,
		ref:     ref,
		attrs:   attrs,
		capture: capture,
		started: time.Now(),
	}
}

// files resolves the authenticated Drive client for this call and remembers
// the resolved configuration for diagnostics.
func (c *callState) files(rc resolvedContext) (*drive.FilesService, error) {
	c.rc = &rc
	svc, fingerprint, err := c.svc.driveClient(c.ctx, c.op)
	if err != nil {
		return nil, err
	}
	c.fp = fingerprint
	c.client = svc
	return svc.Files, nil
}

// fail emits the full diagnostic event (#97 §11) and returns the synthetic
// consumer error. Bearer tokens and the private key never reach the event:
// the capture transport records response data only, and the credential is
// identified by fingerprint.
func (c *callState) fail(err error) error {
	attrs := []slog.Attr{
		slog.String("component", component),
		slog.String("operation", c.op),
		slog.String("app", c.ref.App),
		slog.String("context", c.ref.Context),
		slog.String("code", string(CodeOf(err))),
		slog.Any("error", err),
		slog.String("error_chain", fmt.Sprintf("%+v", err)),
		slog.Int("attempts", c.attempts),
		slog.Int64("elapsed_ms", time.Since(c.started).Milliseconds()),
		slog.String("stack", string(debug.Stack())),
	}
	attrs = append(attrs, c.attrs...)
	if c.rc != nil {
		attrs = append(attrs,
			slog.String("shared_drive_id", c.rc.SharedDriveID),
			slog.String("root_folder_id", c.rc.RootFolderID),
		)
	}
	if c.fp != "" {
		attrs = append(attrs, slog.String("credential_fingerprint", c.fp))
	}
	if exchanges := c.capture.all(); len(exchanges) > 0 {
		attrs = append(attrs, slog.Any("exchanges", exchanges))
		last := exchanges[len(exchanges)-1]
		attrs = append(attrs,
			slog.String("method", last.Method),
			slog.String("url", last.URL),
			slog.Int("status", last.Status),
		)
		if retryAfter := last.RespHeader.Get("Retry-After"); retryAfter != "" {
			attrs = append(attrs, slog.String("retry_after", retryAfter))
		}
	}
	c.svc.logger.LogAttrs(c.ctx, slog.LevelError, "google drive operation failed", attrs...)
	return err
}
