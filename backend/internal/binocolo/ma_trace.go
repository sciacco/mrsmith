package binocolo

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	maTraceStatusRunning   = "running"
	maTraceStatusSucceeded = "succeeded"
	maTraceStatusFailed    = "failed"

	maTraceEventInfo      = "info"
	maTraceEventStarted   = "started"
	maTraceEventSucceeded = "succeeded"
	maTraceEventFailed    = "failed"
)

type maTraceContextKey struct{}

type maTraceContext struct {
	id        string
	startedAt time.Time
	mu        sync.Mutex
	nextOrder int
}

type maTraceStart struct {
	ID               string
	RequestID        string
	Operation        string
	Method           string
	Path             string
	SessionID        string
	CreatedBySubject string
	CreatedByEmail   string
	Request          json.RawMessage
	StartedAt        time.Time
}

type maTraceLink struct {
	ID                string
	SessionID         string
	StrategyVersionID string
	ExecutionRunID    string
}

type maTraceComplete struct {
	ID           string
	Status       string
	HTTPStatus   int
	ErrorCode    string
	ErrorMessage string
	CompletedAt  time.Time
}

type maTraceEventWrite struct {
	TraceID        string
	EventOrder     int
	EventType      string
	Round          *int
	ToolName       string
	ExternalSystem string
	Status         string
	DurationMS     *int
	Request        json.RawMessage
	Response       json.RawMessage
	Metadata       json.RawMessage
	Error          string
}

func (s *maService) startTrace(ctx context.Context, input maTraceStart) (*maTraceContext, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	if input.ID == "" {
		input.ID = uuid.NewString()
	}
	if input.StartedAt.IsZero() {
		input.StartedAt = s.now()
	}
	if len(input.Request) == 0 {
		input.Request = json.RawMessage(`{}`)
	}
	id, err := s.store.StartMATrace(ctx, input)
	if err != nil {
		return nil, err
	}
	return &maTraceContext{id: id, startedAt: input.StartedAt}, nil
}

func withMATrace(ctx context.Context, trace *maTraceContext) context.Context {
	if trace == nil || strings.TrimSpace(trace.id) == "" {
		return ctx
	}
	return context.WithValue(ctx, maTraceContextKey{}, trace)
}

func maTraceFromContext(ctx context.Context) *maTraceContext {
	trace, _ := ctx.Value(maTraceContextKey{}).(*maTraceContext)
	return trace
}

func maTraceID(ctx context.Context) string {
	trace := maTraceFromContext(ctx)
	if trace == nil {
		return ""
	}
	return trace.id
}

func (s *maService) linkTrace(ctx context.Context, input maTraceLink) error {
	traceID := strings.TrimSpace(input.ID)
	if traceID == "" {
		traceID = maTraceID(ctx)
	}
	if traceID == "" || s.store == nil {
		return nil
	}
	input.ID = traceID
	return s.store.LinkMATrace(ctx, input)
}

func (s *maService) completeTrace(ctx context.Context, input maTraceComplete) error {
	traceID := strings.TrimSpace(input.ID)
	if traceID == "" {
		traceID = maTraceID(ctx)
	}
	if traceID == "" || s.store == nil {
		return nil
	}
	input.ID = traceID
	if input.CompletedAt.IsZero() {
		input.CompletedAt = s.now()
	}
	if input.Status == "" {
		input.Status = maTraceStatusSucceeded
	}
	return s.store.CompleteMATrace(ctx, input)
}

func (s *maService) traceEvent(ctx context.Context, event maTraceEventWrite) error {
	trace := maTraceFromContext(ctx)
	if trace == nil || strings.TrimSpace(trace.id) == "" || s.store == nil {
		return nil
	}
	trace.mu.Lock()
	trace.nextOrder++
	event.TraceID = trace.id
	event.EventOrder = trace.nextOrder
	trace.mu.Unlock()
	if event.Status == "" {
		event.Status = maTraceEventInfo
	}
	if event.EventType == "" {
		event.EventType = "event"
	}
	if len(event.Request) == 0 {
		event.Request = json.RawMessage(`{}`)
	}
	if len(event.Response) == 0 {
		event.Response = json.RawMessage(`{}`)
	}
	if len(event.Metadata) == 0 {
		event.Metadata = json.RawMessage(`{}`)
	}
	if err := s.store.RecordMATraceEvent(ctx, event); err != nil {
		logging.FromContext(ctx).Error("binocolo ma trace event failed", "component", "binocolo", "operation", "ma_trace_event", "trace_id", trace.id, "error", err)
		return err
	}
	return nil
}

func maTraceDuration(start time.Time) *int {
	if start.IsZero() {
		return nil
	}
	ms := int(time.Since(start).Milliseconds())
	return &ms
}

func maTraceJSON(value any) json.RawMessage {
	if value == nil {
		return json.RawMessage(`{}`)
	}
	if raw, ok := value.(json.RawMessage); ok {
		return maTraceRawJSON(raw)
	}
	if raw, ok := value.([]byte); ok {
		return maTraceRawJSON(raw)
	}
	data, err := json.Marshal(value)
	if err != nil {
		fallback, _ := json.Marshal(map[string]string{"marshal_error": err.Error()})
		return fallback
	}
	return maTraceRawJSON(data)
}

func maTraceRawJSON(raw []byte) json.RawMessage {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		fallback, _ := json.Marshal(map[string]string{"raw": string(raw)})
		return fallback
	}
	out, err := json.Marshal(redactMATraceTokens(value))
	if err != nil {
		fallback, _ := json.Marshal(map[string]string{"marshal_error": err.Error()})
		return fallback
	}
	return json.RawMessage(out)
}

// maTraceRedacted is the placeholder substituted for a redacted secret.
const maTraceRedacted = "[redacted]"

// maTraceSecretKeys is the exact set of object keys whose value is a credential
// and must be redacted from traces. Lookup is on the normalized key (lowercased,
// separators stripped — so access_token / accessToken / access-token all match)
// and EXACT, never substring. This is the deliberate fix for the old
// strings.Contains(key, "token") rule, which also nuked usage/cost fields that
// merely contain the word: prompt_tokens, completion_tokens, total_tokens (the
// response usage) and max_tokens (the request budget) are NOT secrets and are
// preserved. password is intentionally absent (kept by policy; see
// TestMATraceJSONRedactsOnlyTokenFields) — a credential-shaped password value is
// still caught by looksLikeMATraceSecretValue.
var maTraceSecretKeys = map[string]struct{}{
	"authorization": {},
	"apikey":        {},
	"xapikey":       {},
	"accesstoken":   {},
	"refreshtoken":  {},
	"idtoken":       {},
	"bearertoken":   {},
}

var (
	// A Bearer authorization header is the one credential format that contains a
	// space, so it is matched before the whitespace gate below.
	maTraceBearerRE = regexp.MustCompile(`(?i)^bearer\s+[a-z0-9._\-+/=]{12,}$`)
	// OpenAI/OpenRouter-style API keys (sk-..., sk-or-v1-...).
	maTraceAPIKeyRE = regexp.MustCompile(`^sk-[A-Za-z0-9_-]{12,}$`)
	// Compact JWT (header.payload.signature).
	maTraceJWTRE = regexp.MustCompile(`^eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{4,}\.[A-Za-z0-9_-]{4,}$`)
	// Standard UUID — high-entropy but never a secret (response/session ids).
	maTraceUUIDRE = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	// Opaque single-token charset (base64/hex/api-key shape). The entropy fallback
	// only fires on strings made ENTIRELY of these chars, so JSON ("{", "\"", ":")
	// and prose never qualify.
	maTraceOpaqueRE = regexp.MustCompile(`^[A-Za-z0-9._+/=-]+$`)
)

func redactMATraceTokens(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, val := range typed {
			if isMATraceSecretKey(key) {
				out[key] = maTraceRedacted
				continue
			}
			out[key] = redactMATraceTokens(val)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, redactMATraceTokens(item))
		}
		return out
	case string:
		if looksLikeMATraceSecretValue(typed) {
			return maTraceRedacted
		}
		return typed
	default:
		return value
	}
}

func isMATraceSecretKey(key string) bool {
	_, ok := maTraceSecretKeys[normalizeMATraceKey(key)]
	return ok
}

// normalizeMATraceKey lowercases and strips every non-alphanumeric rune, so a
// secret key matches regardless of its naming style (snake_case, camelCase,
// kebab-case, header form).
func normalizeMATraceKey(key string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(key)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// looksLikeMATraceSecretValue redacts a string VALUE shaped like a credential,
// whatever its key — defense in depth against a secret stored under an
// unexpected name. Deliberately conservative so audit data survives: it matches
// only specific credential formats (Bearer headers, sk- API keys, JWTs) plus
// long, whitespace-free, opaque, high-entropy blobs. Natural-language content
// (prompts/messages contain spaces), JSON payloads (contain "{"/"\""/":"), short
// values (ateco codes, model ids) and UUIDs are never touched.
func looksLikeMATraceSecretValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if maTraceBearerRE.MatchString(trimmed) {
		return true
	}
	if strings.ContainsAny(trimmed, " \t\r\n") {
		return false
	}
	if maTraceAPIKeyRE.MatchString(trimmed) || maTraceJWTRE.MatchString(trimmed) {
		return true
	}
	if maTraceUUIDRE.MatchString(trimmed) {
		return false
	}
	return len(trimmed) >= 48 && maTraceOpaqueRE.MatchString(trimmed) && shannonEntropyBits(trimmed) >= 4.0
}

// shannonEntropyBits returns the Shannon entropy (bits per character) of value.
func shannonEntropyBits(value string) float64 {
	runes := []rune(value)
	if len(runes) == 0 {
		return 0
	}
	counts := make(map[rune]int, len(runes))
	for _, r := range runes {
		counts[r]++
	}
	total := float64(len(runes))
	entropy := 0.0
	for _, c := range counts {
		p := float64(c) / total
		entropy -= p * math.Log2(p)
	}
	return entropy
}

func maTraceRound(round int) *int {
	value := round
	return &value
}

func maToolResultStatus(content string) string {
	var body map[string]any
	if err := json.Unmarshal([]byte(content), &body); err != nil {
		return maTraceEventSucceeded
	}
	if errValue, ok := body["error"]; ok && strings.TrimSpace(toTraceString(errValue)) != "" {
		return maTraceEventFailed
	}
	return maTraceEventSucceeded
}

func toTraceString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(raw)
	}
}

func nullableTraceString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
