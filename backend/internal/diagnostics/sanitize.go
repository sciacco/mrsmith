package diagnostics

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	maxMessageBytes   = 2000
	maxErrorBytes     = 4000
	maxStackBytes     = 20000
	maxStringBytes    = 2000
	maxAttrsJSONBytes = 64 * 1024
	maxObjectFields   = 120
	maxArrayItems     = 50
	maxDepth          = 8
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
	clean := sanitizeValue(attrs, 0)
	if typed, ok := clean.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func sanitizeValue(value any, depth int) any {
	if depth > maxDepth {
		return "[truncated]"
	}

	switch typed := value.(type) {
	case nil:
		return nil
	case error:
		return truncateString(typed.Error(), maxErrorBytes)
	case fmt.Stringer:
		return truncateString(typed.String(), maxStringBytes)
	case map[string]any:
		clean := make(map[string]any, minInt(len(typed), maxObjectFields))
		count := 0
		for key, val := range typed {
			if count >= maxObjectFields {
				clean["_truncated"] = true
				break
			}
			if isSensitiveKey(key) {
				clean[key] = "[redacted]"
			} else {
				clean[key] = sanitizeValue(val, depth+1)
			}
			count++
		}
		return clean
	case []any:
		limit := minInt(len(typed), maxArrayItems)
		clean := make([]any, 0, limit)
		for i := 0; i < limit; i++ {
			clean = append(clean, sanitizeValue(typed[i], depth+1))
		}
		if len(typed) > limit {
			clean = append(clean, "[truncated]")
		}
		return clean
	case []string:
		limit := minInt(len(typed), maxArrayItems)
		clean := make([]any, 0, limit)
		for i := 0; i < limit; i++ {
			clean = append(clean, sanitizeValue(typed[i], depth+1))
		}
		if len(typed) > limit {
			clean = append(clean, "[truncated]")
		}
		return clean
	case string:
		return truncateString(typed, maxStringBytes)
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
	if len(raw) <= maxAttrsJSONBytes {
		return raw, nil
	}
	raw, err = json.Marshal(map[string]any{
		"_truncated": true,
		"reason":     "attrs_too_large",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal truncated diagnostic attrs: %w", err)
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
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, part := range sensitiveKeyParts {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}

func truncateString(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	if maxBytes <= len("[truncated]") {
		return value[:maxBytes]
	}
	return value[:maxBytes-len("[truncated]")] + "[truncated]"
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
