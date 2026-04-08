package listini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultKitTemplateID is the Carbone template ID for kit PDF generation.
// TODO: make configurable via portal admin module.
const DefaultKitTemplateID = "d7c2d6562ed7ca6d1a9f88958eaf4ce1e791d456cd5e0fd5ef457e596b14b657"

// CarboneService handles PDF generation via Carbone Cloud API.
type CarboneService struct {
	apiKey     string
	httpCli    *http.Client
	templateID string
}

// NewCarboneService creates a new CarboneService. Returns nil if apiKey is empty.
func NewCarboneService(apiKey, templateID string) *CarboneService {
	if apiKey == "" {
		return nil
	}
	return &CarboneService{
		apiKey:     apiKey,
		templateID: templateID,
		httpCli: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GeneratePDF sends data to Carbone and returns PDF bytes and a suggested filename.
func (s *CarboneService) GeneratePDF(ctx context.Context, data any) ([]byte, string, error) {
	// Step 1: Render
	renderPayload := map[string]any{
		"data":      data,
		"convertTo": "pdf",
	}

	body, err := json.Marshal(renderPayload)
	if err != nil {
		return nil, "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://api.carbone.io/render/%s", s.templateID), bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("new render request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("carbone-version", "4")

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("render http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("carbone render returned %d", resp.StatusCode)
	}

	var renderResp struct {
		Success bool `json:"success"`
		Data    struct {
			RenderID string `json:"renderId"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&renderResp); err != nil {
		return nil, "", fmt.Errorf("decode render response: %w", err)
	}
	if !renderResp.Success || renderResp.Data.RenderID == "" {
		return nil, "", fmt.Errorf("carbone render failed")
	}

	// Step 2: Download
	dlReq, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://api.carbone.io/render/%s", renderResp.Data.RenderID), nil)
	if err != nil {
		return nil, "", fmt.Errorf("new download request: %w", err)
	}
	dlReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	dlReq.Header.Set("carbone-version", "4")

	dlResp, err := s.httpCli.Do(dlReq)
	if err != nil {
		return nil, "", fmt.Errorf("download http: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("carbone download returned %d", dlResp.StatusCode)
	}

	pdfBytes, err := io.ReadAll(dlResp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read pdf: %w", err)
	}

	filename := "document.pdf"
	if cd := dlResp.Header.Get("Content-Disposition"); cd != "" {
		filename = cd
	}

	return pdfBytes, filename, nil
}
