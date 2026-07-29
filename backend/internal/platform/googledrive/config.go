package googledrive

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// resolvedContext is one enabled mrsmith.googledrive_context row.
type resolvedContext struct {
	SharedDriveID string
	RootFolderID  string
}

// resolveContext loads the (app, context) binding from the DB on every
// application request (#97 §10: no config TTL cache, no implicit fallback).
func (s *Service) resolveContext(ctx context.Context, op string, ref ContextRef) (resolvedContext, error) {
	if s == nil || s.db == nil {
		return resolvedContext{}, newError(op, CodeNotConfigured, "googledrive service has no database", nil)
	}
	var rc resolvedContext
	var enabled bool
	err := s.db.QueryRowContext(ctx, `
SELECT shared_drive_id, root_folder_id, enabled
FROM mrsmith.googledrive_context
WHERE app = $1 AND context = $2`, ref.App, ref.Context).Scan(&rc.SharedDriveID, &rc.RootFolderID, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return resolvedContext{}, newError(op, CodeNotConfigured, fmt.Sprintf("no context %q for app %q", ref.Context, ref.App), nil)
	}
	if err != nil {
		return resolvedContext{}, newError(op, CodeUnavailable, "context lookup failed", err)
	}
	if !enabled {
		return resolvedContext{}, newError(op, CodeNotConfigured, fmt.Sprintf("context %q for app %q is disabled", ref.Context, ref.App), nil)
	}
	return rc, nil
}

// cachedClient reuses the authenticated Google client (token + HTTP
// connections) across requests. It is keyed on the credential row's
// updated_at: a rotation becomes effective on the next request, no restart.
type cachedClient struct {
	version     time.Time
	fingerprint string
	svc         *drive.Service
}

// driveClient re-reads the credential row on every request and rebuilds the
// authenticated client only when updated_at changed.
func (s *Service) driveClient(ctx context.Context, op string) (*drive.Service, string, error) {
	var credJSON string
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx, `
SELECT credential_json::text, updated_at
FROM mrsmith.googledrive_credential`).Scan(&credJSON, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", newError(op, CodeNotConfigured, "no service account credential stored", nil)
	}
	if err != nil {
		return nil, "", newError(op, CodeUnavailable, "credential lookup failed", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil && s.client.version.Equal(updatedAt) {
		return s.client.svc, s.client.fingerprint, nil
	}

	jwtCfg, err := google.JWTConfigFromJSON([]byte(credJSON), drive.DriveScope)
	if err != nil {
		return nil, "", newError(op, CodeInvalidCredentials, "service account json not parseable", err)
	}
	fingerprint := credentialFingerprint(credJSON)

	// The client outlives the request, so the token source is bound to a
	// long-lived context; token endpoint calls use oauth2's own HTTP client
	// and never pass through the capture transport (no bearer in diagnostics).
	source := oauth2.ReuseTokenSource(nil, jwtCfg.TokenSource(context.Background()))
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &oauth2.Transport{
			Source: source,
			Base:   &captureTransport{next: http.DefaultTransport},
		},
	}
	svc, err := drive.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, "", newError(op, CodeInvalidCredentials, "drive client construction failed", err)
	}
	s.client = &cachedClient{version: updatedAt, fingerprint: fingerprint, svc: svc}
	return svc, fingerprint, nil
}

// credentialFingerprint identifies the credential in diagnostics without
// exposing any secret: a short hash of client_email + private_key_id.
func credentialFingerprint(credJSON string) string {
	var meta struct {
		ClientEmail  string `json:"client_email"`
		PrivateKeyID string `json:"private_key_id"`
	}
	_ = json.Unmarshal([]byte(credJSON), &meta)
	sum := sha256.Sum256([]byte(meta.ClientEmail + "|" + meta.PrivateKeyID))
	return hex.EncodeToString(sum[:6])
}
