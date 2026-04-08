package listini

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// HubSpotService handles cross-database company lookup and HubSpot API calls.
type HubSpotService struct {
	grappaDB *sql.DB
	mistraDB *sql.DB
	apiKey   string
	httpCli  *http.Client
}

// NewHubSpotService creates a new HubSpotService. Returns nil if apiKey is empty.
func NewHubSpotService(grappaDB, mistraDB *sql.DB, apiKey string) *HubSpotService {
	if apiKey == "" {
		return nil
	}
	return &HubSpotService{
		grappaDB: grappaDB,
		mistraDB: mistraDB,
		apiKey:   apiKey,
		httpCli: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// LookupCompanyID resolves a Grappa customer ID to a HubSpot company ID.
// Path: Grappa cli_fatturazione.id → codice_aggancio_gest (ERP ID) → loader.hubs_company.id
func (s *HubSpotService) LookupCompanyID(ctx context.Context, grappaCustomerID int) (int64, error) {
	// Step 1: Grappa → ERP ID
	var erpID int
	err := s.grappaDB.QueryRowContext(ctx,
		`SELECT codice_aggancio_gest FROM cli_fatturazione WHERE id = ?`,
		grappaCustomerID).Scan(&erpID)
	if err != nil {
		return 0, fmt.Errorf("grappa lookup: %w", err)
	}

	// Step 2: ERP ID → HubSpot company ID
	var companyID int64
	err = s.mistraDB.QueryRowContext(ctx,
		`SELECT id FROM loader.hubs_company WHERE numero_azienda = $1::varchar`,
		fmt.Sprintf("%d", erpID)).Scan(&companyID)
	if err != nil {
		return 0, fmt.Errorf("hubspot company lookup: %w", err)
	}

	return companyID, nil
}

// CreateNote creates a note on a HubSpot company.
func (s *HubSpotService) CreateNote(ctx context.Context, companyID int64, body string) error {
	payload := map[string]any{
		"properties": map[string]any{
			"hs_note_body":      body,
			"hs_timestamp":      time.Now().UnixMilli(),
			"hs_attachment_ids": "",
		},
		"associations": []map[string]any{
			{
				"to":   map[string]any{"id": companyID},
				"types": []map[string]any{{"associationCategory": "HUBSPOT_DEFINED", "associationTypeId": 190}},
			},
		},
	}

	return s.apiCall(ctx, "POST", "https://api.hubapi.com/crm/v3/objects/notes", payload)
}

// CreateTask creates a task on a HubSpot company.
func (s *HubSpotService) CreateTask(ctx context.Context, companyID int64, subject, body, assigneeEmail string) error {
	payload := map[string]any{
		"properties": map[string]any{
			"hs_task_subject": subject,
			"hs_task_body":    body,
			"hs_task_status":  "NOT_STARTED",
			"hs_timestamp":    time.Now().UnixMilli(),
		},
		"associations": []map[string]any{
			{
				"to":   map[string]any{"id": companyID},
				"types": []map[string]any{{"associationCategory": "HUBSPOT_DEFINED", "associationTypeId": 192}},
			},
		},
	}

	// If assignee email is set, add it as an owner lookup
	if assigneeEmail != "" {
		payload["properties"].(map[string]any)["hs_task_reminders"] = assigneeEmail
	}

	return s.apiCall(ctx, "POST", "https://api.hubapi.com/crm/v3/objects/tasks", payload)
}

// CreateNoteAsync creates a note asynchronously (fire-and-forget).
func (s *HubSpotService) CreateNoteAsync(ctx context.Context, companyID int64, body string) {
	if s == nil || s.apiKey == "" {
		return
	}
	logger := logging.FromContext(ctx)
	go func() {
		if err := s.CreateNote(context.Background(), companyID, body); err != nil {
			logger.Error("hubspot note failed", "component", "listini", "company_id", companyID, "error", err)
		}
	}()
}

// CreateNoteAndTaskAsync creates a note and task asynchronously (fire-and-forget).
func (s *HubSpotService) CreateNoteAndTaskAsync(ctx context.Context, companyID int64, noteBody, taskSubject, taskBody, assigneeEmail string) {
	if s == nil || s.apiKey == "" {
		return
	}
	logger := logging.FromContext(ctx)
	go func() {
		if err := s.CreateNote(context.Background(), companyID, noteBody); err != nil {
			logger.Error("hubspot note failed", "component", "listini", "company_id", companyID, "error", err)
		}
		if err := s.CreateTask(context.Background(), companyID, taskSubject, taskBody, assigneeEmail); err != nil {
			logger.Error("hubspot task failed", "component", "listini", "company_id", companyID, "error", err)
		}
	}()
}

func (s *HubSpotService) apiCall(ctx context.Context, method, url string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("hubspot api returned %d", resp.StatusCode)
	}

	return nil
}
