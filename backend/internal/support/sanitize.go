package support

import (
	"encoding/json"
	"strings"
)

const (
	maxSanitizeDepth = 8
	maxStringLength  = 2000
	maxArrayItems    = 50
	maxObjectFields  = 120
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

func decodeAndSanitizeContext(raw json.RawMessage) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}, nil
	}

	var value any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	return sanitizeContext(value, 0), nil
}

func sanitizeContext(value any, depth int) any {
	if depth > maxSanitizeDepth {
		return "[truncated]"
	}

	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, minInt(len(typed), maxObjectFields))
		count := 0
		for key, val := range typed {
			if count >= maxObjectFields {
				clean["_truncated"] = true
				break
			}
			if isSensitiveContextKey(key) {
				continue
			}
			clean[key] = sanitizeContext(val, depth+1)
			count++
		}
		return clean
	case []any:
		limit := minInt(len(typed), maxArrayItems)
		clean := make([]any, 0, limit)
		for i := 0; i < limit; i++ {
			clean = append(clean, sanitizeContext(typed[i], depth+1))
		}
		if len(typed) > limit {
			clean = append(clean, "[truncated]")
		}
		return clean
	case string:
		if len(typed) <= maxStringLength {
			return typed
		}
		return typed[:maxStringLength] + "[truncated]"
	default:
		return typed
	}
}

func isSensitiveContextKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, part := range sensitiveKeyParts {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
