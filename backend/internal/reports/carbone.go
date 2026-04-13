package reports

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Template IDs for Carbone XLSX generation.
const (
	OrdiniTemplateID  = "d18b310491b0c8d2518841b4e09cc18d8b91c5a59ae5a55c37924fcb169de166"
	AccessiTemplateID = "a482f92419a0c17bb9bfae00c64d251c6a527f95c67993d86bf2d11d9e2e7a9e"
)

// CarboneService handles XLSX generation via Carbone Cloud API.
type CarboneService struct {
	apiKey  string
	httpCli *http.Client
}

// NewCarboneService creates a new CarboneService. Returns nil if apiKey is empty.
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

// GenerateXLSX sends data to Carbone and returns XLSX bytes.
func (s *CarboneService) GenerateXLSX(ctx context.Context, templateID string, data any) ([]byte, error) {
	// Step 1: Render
	renderPayload := map[string]any{
		"data":      data,
		"convertTo": "xlsx",
	}

	body, err := json.Marshal(renderPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://api.carbone.io/render/%s", templateID), bytes.NewReader(body))
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

	// Step 2: Download
	dlReq, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://api.carbone.io/render/%s", renderResp.Data.RenderID), nil)
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

	xlsxBytes, err := io.ReadAll(dlResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read xlsx: %w", err)
	}

	return xlsxBytes, nil
}
