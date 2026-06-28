package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// restRecoverSnapshot ripristina uno snapshot nella collection via REST
// (endpoint PUT /collections/{name}/snapshots/recover) con timeout di default.
func (s *Store) restRecoverSnapshot(ctx context.Context, name, snapURL string) error {
	return s.restRecoverSnapshotTimeout(ctx, name, snapURL, 60*time.Second)
}

// restRecoverSnapshotTimeout è l'implementazione concreta del recover via REST.
func (s *Store) restRecoverSnapshotTimeout(ctx context.Context, name, snapURL string, timeout time.Duration) error {
	body, err := json.Marshal(map[string]string{"location": snapURL})
	if err != nil {
		return fmt.Errorf("marshal recover body: %w", err)
	}

	endpoint := fmt.Sprintf("%s/collections/%s/snapshots/recover", s.restURL, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build recover request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("recover snapshot request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("recover snapshot HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
