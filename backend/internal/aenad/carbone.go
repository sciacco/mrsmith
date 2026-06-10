package aenad

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultOffertaTemplateID is the fallback Carbone template id used when the
// mrsmith.runtime_config entry (namespace "aenad", key "carbone_offerta") is
// missing or unreadable. Carbone assigns a new id on every template upload, so
// the live id is kept in runtime config to avoid application restarts. Source
// and regeneration workflow live in scripts/carbone/aenad-offerta/.
const DefaultOffertaTemplateID = "efaaf49c181bd74b732de025c9004656b72530f8c73dc89a6d511645c80236d0"

// CarboneService renders aenad offer PDFs via Carbone Cloud API.
type CarboneService struct {
	apiKey  string
	httpCli *http.Client
}

// NewCarboneService creates a renderer. Returns nil when config is incomplete.
func NewCarboneService(apiKey string) *CarboneService {
	if apiKey == "" {
		return nil
	}
	return &CarboneService{
		apiKey: apiKey,
		httpCli: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GeneratePDF renders the payload with the given template and returns PDF bytes.
func (s *CarboneService) GeneratePDF(ctx context.Context, templateID string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("https://api.carbone.io/render/%s", templateID),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("new render request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("carbone-version", "4")

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("render http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("carbone render returned %d", resp.StatusCode)
	}

	var renderResp struct {
		Success bool `json:"success"`
		Data    struct {
			RenderID string `json:"renderId"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&renderResp); err != nil {
		return nil, fmt.Errorf("decode render response: %w", err)
	}
	if !renderResp.Success || renderResp.Data.RenderID == "" {
		return nil, fmt.Errorf("carbone render failed")
	}

	dlReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://api.carbone.io/render/%s", renderResp.Data.RenderID),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("new download request: %w", err)
	}
	dlReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	dlReq.Header.Set("carbone-version", "4")

	dlResp, err := s.httpCli.Do(dlReq)
	if err != nil {
		return nil, fmt.Errorf("download http: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode >= 400 {
		return nil, fmt.Errorf("carbone download returned %d", dlResp.StatusCode)
	}

	pdfBytes, err := io.ReadAll(dlResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read pdf: %w", err)
	}

	return pdfBytes, nil
}
