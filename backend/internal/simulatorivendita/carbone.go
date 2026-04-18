package simulatorivendita

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultIaaSTemplateID is the audited Carbone template for the source simulator.
const DefaultIaaSTemplateID = "7229f811c77569a9ab09c7f71cd923a942e3d5d5aac1d26b98950a19beb2e920"

// CarboneService renders Simulatori di Vendita PDFs via Carbone Cloud API.
type CarboneService struct {
	apiKey     string
	httpCli    *http.Client
	templateID string
}

// NewCarboneService creates a renderer. Returns nil when config is incomplete.
func NewCarboneService(apiKey, templateID string) *CarboneService {
	if apiKey == "" || templateID == "" {
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

// GeneratePDF submits a prepared Carbone render payload and returns PDF bytes.
func (s *CarboneService) GeneratePDF(ctx context.Context, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("https://api.carbone.io/render/%s", s.templateID),
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
