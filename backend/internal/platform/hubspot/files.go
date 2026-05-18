package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"
)

const AssocTypeNoteToDeal = 214

type UploadedFile struct {
	ID string `json:"id"`
}

func (c *Client) UploadFile(ctx context.Context, filename string, content []byte, folderPath string, options map[string]any) (*UploadedFile, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create file part: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return nil, fmt.Errorf("write file part: %w", err)
	}
	if err := writer.WriteField("folderPath", folderPath); err != nil {
		return nil, fmt.Errorf("write folderPath: %w", err)
	}
	if options == nil {
		options = map[string]any{"access": "PRIVATE"}
	}
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("marshal options: %w", err)
	}
	if err := writer.WriteField("options", string(optionsJSON)); err != nil {
		return nil, fmt.Errorf("write options: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/files/v3/files", &body)
	if err != nil {
		return nil, fmt.Errorf("hubspot: create file upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hubspot: upload file: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hubspot: read upload response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var result struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse upload response: %w", err)
	}
	id := rawJSONID(result.ID)
	if id == "" {
		return nil, fmt.Errorf("parse upload response: missing id")
	}
	return &UploadedFile{ID: id}, nil
}

func (c *Client) CreateNoteWithAttachment(ctx context.Context, dealID, fileID string, orderID int64) (int64, error) {
	dealIDInt, err := strconv.ParseInt(dealID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid deal id %q: %w", dealID, err)
	}
	body := map[string]any{
		"properties": map[string]any{
			"hs_timestamp":      time.Now().UTC().Format(time.RFC3339),
			"hs_note_body":      fmt.Sprintf("Order PDF %d", orderID),
			"hs_attachment_ids": fileID,
		},
		"associations": []Association{
			{
				To: AssociationTo{ID: dealIDInt},
				Types: []AssociationType{{
					Category: "HUBSPOT_DEFINED",
					TypeID:   AssocTypeNoteToDeal,
				}},
			},
		},
	}
	resp, err := c.Post(ctx, "/crm/v3/objects/notes", body)
	if err != nil {
		return 0, fmt.Errorf("create note: %w", err)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, fmt.Errorf("parse note response: %w", err)
	}
	id, _ := strconv.ParseInt(result.ID, 10, 64)
	return id, nil
}

func rawJSONID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	return ""
}
