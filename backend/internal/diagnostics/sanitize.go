package diagnostics

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	// Temporary diagnostic mode: keep sensitive-looking attrs visible while
	// investigating upstream failures. Set to false to restore redaction.
	sensitiveAttrsPassthrough = true
)

var sensitiveKeyParts = []string{
	"authorization",
	"cookie",
	"password",
	"secret",
	"token",
	"body",
	"payload",
}

func sanitizeAttrs(attrs map[string]any) map[string]any {
	clean := sanitizeValue(attrs)
	if typed, ok := clean.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func sanitizeValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case error:
		return typed.Error()
	case fmt.Stringer:
		return typed.String()
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, val := range typed {
			if isSensitiveKey(key) {
				clean[key] = "[redacted]"
			} else {
				clean[key] = sanitizeValue(val)
			}
		}
		return clean
	case []any:
		clean := make([]any, 0, len(typed))
		for _, val := range typed {
			clean = append(clean, sanitizeValue(val))
		}
		return clean
	case []string:
		clean := make([]any, 0, len(typed))
		for _, val := range typed {
			clean = append(clean, sanitizeValue(val))
		}
		return clean
	case string:
		return typed
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	case time.Duration:
		return typed.String()
	default:
		if _, err := json.Marshal(typed); err == nil {
			return typed
		}
		return fmt.Sprint(typed)
	}
}

func attrsForStorage(attrs map[string]any) ([]byte, error) {
	if attrs == nil {
		attrs = map[string]any{}
	}
	raw, err := json.Marshal(attrs)
	if err != nil {
		return nil, fmt.Errorf("marshal diagnostic attrs: %w", err)
	}
	return raw, nil
}

func attrsFromStorage(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var attrs map[string]any
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return map[string]any{"_decode_error": err.Error()}
	}
	return attrs
}

func slogValueToAny(value slog.Value) any {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindAny:
		return value.Any()
	case slog.KindBool:
		return value.Bool()
	case slog.KindDuration:
		return value.Duration()
	case slog.KindFloat64:
		return value.Float64()
	case slog.KindInt64:
		return value.Int64()
	case slog.KindString:
		return value.String()
	case slog.KindTime:
		return value.Time()
	case slog.KindUint64:
		return value.Uint64()
	case slog.KindGroup:
		group := make(map[string]any)
		for _, attr := range value.Group() {
			if attr.Key == "" {
				continue
			}
			group[attr.Key] = slogValueToAny(attr.Value)
		}
		return group
	case slog.KindLogValuer:
		return slogValueToAny(value.Resolve())
	default:
		return value.Any()
	}
}

func stringField(attrs map[string]any, key string) string {
	raw, ok := attrs[key]
	if !ok || raw == nil {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return typed
	case error:
		return typed.Error()
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

func intField(attrs map[string]any, key string) *int {
	raw, ok := attrs[key]
	if !ok || raw == nil {
		return nil
	}
	var value int
	switch typed := raw.(type) {
	case int:
		value = typed
	case int64:
		value = int(typed)
	case int32:
		value = int(typed)
	case uint64:
		value = int(typed)
	case uint:
		value = int(typed)
	case float64:
		value = int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return nil
		}
		value = int(parsed)
	default:
		return nil
	}
	return &value
}

func isSensitiveKey(key string) bool {
	if sensitiveAttrsPassthrough {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, part := range sensitiveKeyParts {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}
