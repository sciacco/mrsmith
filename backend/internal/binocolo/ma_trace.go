package binocolo

import (
	"context"
	"encoding/json"
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

func redactMATraceTokens(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, val := range typed {
			if isMATraceTokenKey(key) {
				out[key] = "[redacted]"
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
	default:
		return value
	}
}

func isMATraceTokenKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "-", "_")
	return key == "authorization" ||
		key == "api_key" ||
		key == "apikey" ||
		key == "access_token" ||
		key == "refresh_token" ||
		key == "id_token" ||
		strings.Contains(key, "token")
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
