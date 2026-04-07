package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewWithWriterHonorsDebugLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf, "debug")

	logger.Debug("debug message", "key", "value")

	entry := decodeLogEntry(t, buf.String())
	if entry["level"] != "DEBUG" {
		t.Fatalf("expected DEBUG level, got %#v", entry["level"])
	}
	if entry["msg"] != "debug message" {
		t.Fatalf("expected debug message, got %#v", entry["msg"])
	}
}

func TestNewWithWriterDefaultsInvalidLevelToInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf, "not-a-level")

	logger.Debug("suppressed")
	logger.Info("info message")

	lines := splitLogLines(buf.String())
	if len(lines) != 1 {
		t.Fatalf("expected 1 emitted line, got %d", len(lines))
	}

	entry := decodeLogEntry(t, lines[0])
	if entry["level"] != "INFO" {
		t.Fatalf("expected INFO level, got %#v", entry["level"])
	}
	if entry["msg"] != "info message" {
		t.Fatalf("expected info message, got %#v", entry["msg"])
	}
}

func decodeLogEntry(t *testing.T, raw string) map[string]any {
	t.Helper()

	lines := splitLogLines(raw)
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("failed to decode log line: %v", err)
	}
	return entry
}

func splitLogLines(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.Split(strings.TrimSpace(raw), "\n")
}
