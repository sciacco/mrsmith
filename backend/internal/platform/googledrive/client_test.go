package googledrive

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// fakeFile is what the fake Drive handler serves for GET /drive/v3/files/{id}.
type fakeFile struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Mime    string   `json:"mimeType"`
	Trashed bool     `json:"trashed"`
	Parents []string `json:"parents,omitempty"`
	DriveID string   `json:"driveId"`
}

func driveHandler(files map[string]fakeFile) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/drive/v3/files/")
		f, ok := files[id]
		if !ok {
			writeDriveError(w, http.StatusNotFound, "notFound", "File not found")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(f)
	}
}

func writeDriveError(w http.ResponseWriter, status int, reason, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"code":%d,"message":%q,"errors":[{"reason":%q,"message":%q}]}}`, status, message, reason, message)
}

// newTestCall builds a FilesService over the fake handler (behind the real
// capture transport) and a callState like the ops construct, bypassing the DB
// resolution steps.
func newTestCall(t *testing.T, handler http.Handler) (*drive.FilesService, *callState) {
	t.Helper()
	mock := httputil.NewMockClient(handler)
	client := &http.Client{Transport: &captureTransport{next: mock.Transport}}
	svc, err := drive.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		t.Fatalf("drive.NewService: %v", err)
	}
	ctx, capture := withCapture(context.Background())
	call := &callState{
		svc:     New(nil, slog.New(slog.DiscardHandler)),
		ctx:     ctx,
		op:      "GetItem",
		ref:     ContextRef{App: "testapp", Context: "docs"},
		capture: capture,
		started: time.Now(),
	}
	return svc.Files, call
}

var testRC = resolvedContext{SharedDriveID: "drive1", RootFolderID: "root1"}

func TestFetchInRootAcceptsRootAndDescendants(t *testing.T) {
	files, call := newTestCall(t, driveHandler(map[string]fakeFile{
		"root1": {ID: "root1", Name: "Root", DriveID: "drive1"},
		"f1":    {ID: "f1", Name: "Level1", Parents: []string{"root1"}, DriveID: "drive1"},
		"x":     {ID: "x", Name: "Doc", Parents: []string{"f1"}, DriveID: "drive1", Mime: "application/pdf"},
	}))
	for _, id := range []string{"root1", "f1", "x"} {
		if _, err := call.svc.fetchInRoot(call, files, testRC, id); err != nil {
			t.Fatalf("fetchInRoot(%s): unexpected error %v", id, err)
		}
	}
}

func TestFetchInRootRejectsResourceMovedOutsideRoot(t *testing.T) {
	// "elsewhere" hangs directly from the drive top, not from root1: the move
	// preserved its ID (spike-verified), only ancestry can catch it.
	files, call := newTestCall(t, driveHandler(map[string]fakeFile{
		"elsewhere": {ID: "elsewhere", Name: "Moved", Parents: []string{"drive1"}, DriveID: "drive1"},
		"y":         {ID: "y", Name: "Doc", Parents: []string{"elsewhere"}, DriveID: "drive1"},
	}))
	_, err := call.svc.fetchInRoot(call, files, testRC, "y")
	if !IsCode(err, CodeOutsideContextRoot) {
		t.Fatalf("expected outside_context_root, got %v", err)
	}
}

func TestFetchInRootRejectsOtherDrive(t *testing.T) {
	files, call := newTestCall(t, driveHandler(map[string]fakeFile{
		"z": {ID: "z", Name: "Foreign", Parents: []string{"root1"}, DriveID: "drive2"},
	}))
	_, err := call.svc.fetchInRoot(call, files, testRC, "z")
	if !IsCode(err, CodeOutsideContextRoot) {
		t.Fatalf("expected outside_context_root, got %v", err)
	}
}

func TestFetchInRootStopsAtAncestryDepthCap(t *testing.T) {
	deep := map[string]fakeFile{}
	for i := 0; i <= maxAncestryDepth+2; i++ {
		deep[fmt.Sprintf("deep%d", i)] = fakeFile{
			ID:      fmt.Sprintf("deep%d", i),
			Parents: []string{fmt.Sprintf("deep%d", i+1)},
			DriveID: "drive1",
		}
	}
	files, call := newTestCall(t, driveHandler(deep))
	_, err := call.svc.fetchInRoot(call, files, testRC, "deep0")
	if !IsCode(err, CodeOutsideContextRoot) {
		t.Fatalf("expected outside_context_root at depth cap, got %v", err)
	}
}

func TestFetchInRootKeepsTrashedFlag(t *testing.T) {
	// Spike-verified: a trashed resource is still served with trashed=true.
	// GetItem surfaces the flag; container ops turn it into CodeTrashed.
	files, call := newTestCall(t, driveHandler(map[string]fakeFile{
		"t1": {ID: "t1", Name: "Trashed", Parents: []string{"root1"}, DriveID: "drive1", Trashed: true},
	}))
	f, err := call.svc.fetchInRoot(call, files, testRC, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.Trashed {
		t.Fatal("expected trashed=true to be preserved")
	}
}

func Test404MeansNotFoundOrNotAccessible(t *testing.T) {
	// Spike-verified: revoking the Service Account from the Shared Drive
	// yields the same 404 as a wrong or deleted ID.
	files, call := newTestCall(t, driveHandler(nil))
	_, err := call.svc.fetchInRoot(call, files, testRC, "revoked-or-missing")
	if !IsCode(err, CodeNotFound) {
		t.Fatalf("expected not_found, got %v", err)
	}
	if call.attempts != 1 {
		t.Fatalf("404 must not be retried, got %d attempts", call.attempts)
	}
}

func TestRetryReadHonorsRetryAfterAndSucceeds(t *testing.T) {
	calls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 2 {
			w.Header().Set("Retry-After", "0")
			writeDriveError(w, http.StatusTooManyRequests, "rateLimitExceeded", "Rate limit exceeded")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok1","name":"Doc","driveId":"drive1","parents":["root1"]}`))
	})
	files, call := newTestCall(t, handler)
	f, err := call.svc.fetchInRoot(call, files, testRC, "ok1")
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if f.Id != "ok1" || call.attempts != 3 {
		t.Fatalf("expected 3 attempts and item ok1, got %d attempts, id %q", call.attempts, f.Id)
	}
}

func TestForbiddenRateLimitReasonIsRetriedAndMapped(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		writeDriveError(w, http.StatusForbidden, "userRateLimitExceeded", "User rate limit exceeded")
	})
	files, call := newTestCall(t, handler)
	_, err := call.svc.fetchInRoot(call, files, testRC, "any")
	if !IsCode(err, CodeRateLimited) {
		t.Fatalf("403 with rate-limit reason must map to rate_limited, got %v", err)
	}
	if call.attempts != maxReadAttempts {
		t.Fatalf("expected %d attempts, got %d", maxReadAttempts, call.attempts)
	}
}

func TestForbiddenPermissionIsNotRetried(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeDriveError(w, http.StatusForbidden, "insufficientFilePermissions", "The user does not have permission")
	})
	files, call := newTestCall(t, handler)
	_, err := call.svc.fetchInRoot(call, files, testRC, "any")
	if !IsCode(err, CodePermissionDenied) {
		t.Fatalf("expected permission_denied, got %v", err)
	}
	if call.attempts != 1 {
		t.Fatalf("permission errors must not be retried, got %d attempts", call.attempts)
	}
}

func TestCaptureRecordsFullExchanges(t *testing.T) {
	files, call := newTestCall(t, driveHandler(nil))
	_, _ = call.svc.fetchInRoot(call, files, testRC, "ghost")
	exchanges := call.capture.all()
	if len(exchanges) != 1 {
		t.Fatalf("expected 1 captured exchange, got %d", len(exchanges))
	}
	e := exchanges[0]
	if e.Method != http.MethodGet || e.Status != http.StatusNotFound {
		t.Fatalf("unexpected exchange %+v", e)
	}
	if !strings.Contains(e.URL, "/drive/v3/files/ghost") || !strings.Contains(e.RespBody, "notFound") {
		t.Fatalf("exchange must keep full URL and body, got %+v", e)
	}
}
