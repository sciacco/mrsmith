package llm

import (
	"context"
	"encoding/json"
	"regexp"
)

// CallAudit is one row for mrsmith.llm_call_audit. Records both successes and
// failures. Domain references go in Context (soft, jsonb) — the central audit
// has no FK to any app's domain tables.
type CallAudit struct {
	App        string
	Scope      string
	ProviderID string // optional; "" -> NULL
	ModelID    string // optional; "" -> NULL
	PromptID   string // optional; "" -> NULL
	Model      string // snapshot of the model string at call time

	Request  json.RawMessage // messages+tools+config
	Response json.RawMessage
	Usage    json.RawMessage // {prompt_tokens, completion_tokens, total_tokens}
	Context  json.RawMessage // soft domain refs, e.g. {"session_id": "..."}

	ActorSubject string
	ActorEmail   string
	RequestID    string

	Status       string // "succeeded" | "failed"; defaults to "succeeded"
	ErrorCode    string
	ErrorMessage string
	DurationMS   *int
}

// secretPatterns redacts provider error bodies that can echo bearer tokens or
// API keys. Mirrors the binocolo trace redactor for the leak-prone field.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-+/=]{12,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9_\-]{12,}`),
	regexp.MustCompile(`eyJ[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{4,}\.[A-Za-z0-9_\-]{4,}`),
}

func redactSecrets(s string) string {
	for _, re := range secretPatterns {
		s = re.ReplaceAllString(s, "[redacted]")
	}
	return s
}

// RecordAudit appends a call to mrsmith.llm_call_audit. error_message is
// redacted; request/response are stored as provided (callers must not place
// secrets in the request body — the API key travels in the HTTP header, not the
// body).
func (s *Service) RecordAudit(ctx context.Context, a CallAudit) error {
	if s == nil || s.db == nil {
		return ErrNotConfigured
	}
	jsonOrEmpty := func(raw json.RawMessage) []byte {
		if len(raw) == 0 {
			return []byte("{}")
		}
		return raw
	}
	status := a.Status
	if status == "" {
		status = "succeeded"
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO mrsmith.llm_call_audit (
  app, scope, provider_id, model_id, prompt_id, model,
  request, response, usage, context,
  actor_subject, actor_email, request_id, status, error_code, error_message, duration_ms
) VALUES (
  $1, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, $6,
  $7::jsonb, $8::jsonb, $9::jsonb, $10::jsonb,
  $11, $12, $13, $14, $15, $16, $17
)`,
		a.App, a.Scope, a.ProviderID, a.ModelID, a.PromptID, a.Model,
		jsonOrEmpty(a.Request), jsonOrEmpty(a.Response), jsonOrEmpty(a.Usage), jsonOrEmpty(a.Context),
		a.ActorSubject, a.ActorEmail, a.RequestID, status, a.ErrorCode, redactSecrets(a.ErrorMessage), a.DurationMS,
	)
	return err
}
