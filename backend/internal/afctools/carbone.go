package afctools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CarboneService renders templates via the Carbone Cloud API and returns
// just the renderId — the frontend opens the resulting URL in a new tab
// (decision A.5.4 = 4a). The template id never reaches the browser.
type CarboneService struct {
	apiKey                string
	transazioniTemplateID string
	httpCli               *http.Client
}

// NewCarboneService returns nil when either parameter is empty so callers
// can treat the service as "not configured" without an extra flag.
func NewCarboneService(apiKey, transazioniTemplateID string) *CarboneService {
	if apiKey == "" || transazioniTemplateID == "" {
		return nil
	}
	return &CarboneService{
		apiKey:                apiKey,
		transazioniTemplateID: transazioniTemplateID,
		httpCli:               &http.Client{Timeout: 30 * time.Second},
	}
}

// RenderTransazioni POSTs the transactions payload to carbone.io and returns
// the renderId. Matches the Appsmith `render_template` body shape exactly:
// {"convertTo": "xlsx", "reportName": "...", "data": {"righe": [...]}}.
func (s *CarboneService) RenderTransazioni(ctx context.Context, reportName string, data any) (string, error) {
	payload := map[string]any{
		"convertTo":  "xlsx",
		"reportName": reportName,
		"data":       map[string]any{"righe": data},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.carbone.io/render/"+s.transazioniTemplateID,
		bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("carbone-version", "4")

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return "", fmt.Errorf("render http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("carbone render returned %d: %s", resp.StatusCode, b)
	}

	var renderResp struct {
		Success bool `json:"success"`
		Data    struct {
			RenderID string `json:"renderId"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&renderResp); err != nil {
		return "", fmt.Errorf("decode render response: %w", err)
	}
	if !renderResp.Success || renderResp.Data.RenderID == "" {
		return "", fmt.Errorf("carbone render returned no renderId")
	}
	return renderResp.Data.RenderID, nil
}
