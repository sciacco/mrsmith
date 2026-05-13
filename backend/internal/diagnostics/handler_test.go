package diagnostics

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type recordingSink struct {
	events []Event
}

func (r *recordingSink) Enqueue(event Event) {
	r.events = append(r.events, event)
}

func TestSlogHandlerCapturesWarnAndErrorOnly(t *testing.T) {
	recorder := &recordingSink{}
	logger := slog.New(NewSlogHandler(slog.NewJSONHandler(io.Discard, nil), recorder)).
		With("component", "diagnostics-test", "request_id", "req-123", "auth_subject", "user-1")

	logger.Info("skip me", "operation", "info")
	logger.Warn("keep me", "operation", "warn_op", "token", "secret-token")
	logger.Error("boom", "operation", "error_op", "status", 500, "error", errors.New("database exploded"))

	if len(recorder.events) != 2 {
		t.Fatalf("expected 2 captured events, got %d", len(recorder.events))
	}

	warn := recorder.events[0]
	if warn.Level != "WARN" {
		t.Fatalf("expected WARN, got %q", warn.Level)
	}
	if warn.Component != "diagnostics-test" || warn.Operation != "warn_op" || warn.RequestID != "req-123" {
		t.Fatalf("unexpected warn metadata: %#v", warn)
	}
	if warn.Attrs["token"] != "[redacted]" {
		t.Fatalf("expected sensitive token to be redacted, got %#v", warn.Attrs["token"])
	}

	errEvent := recorder.events[1]
	if errEvent.Level != "ERROR" {
		t.Fatalf("expected ERROR, got %q", errEvent.Level)
	}
	if errEvent.Error != "database exploded" {
		t.Fatalf("expected error string, got %q", errEvent.Error)
	}
	if errEvent.Status == nil || *errEvent.Status != 500 {
		t.Fatalf("expected status 500, got %#v", errEvent.Status)
	}
}

func TestSlogHandlerSanitizesNestedSensitiveFields(t *testing.T) {
	recorder := &recordingSink{}
	logger := slog.New(NewSlogHandler(slog.NewJSONHandler(io.Discard, nil), recorder))

	logger.Warn("nested", "context", map[string]any{
		"safe": "visible",
		"headers": map[string]any{
			"authorization": "Bearer secret",
		},
	})

	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(recorder.events))
	}
	contextAttr, ok := recorder.events[0].Attrs["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context map, got %#v", recorder.events[0].Attrs["context"])
	}
	headers, ok := contextAttr["headers"].(map[string]any)
	if !ok {
		t.Fatalf("expected headers map, got %#v", contextAttr["headers"])
	}
	if headers["authorization"] != "[redacted]" {
		t.Fatalf("expected authorization to be redacted, got %#v", headers["authorization"])
	}
}

func TestSinkDropsWhenQueueFullWithoutBlocking(t *testing.T) {
	store := &fakeStore{}
	sink := NewSink(store, SinkConfig{Enabled: true, QueueSize: 1})

	sink.Enqueue(Event{Message: "first"})
	sink.Enqueue(Event{Message: "second"})

	status := sink.Status()
	if status.QueueDepth != 1 {
		t.Fatalf("expected queue depth 1, got %d", status.QueueDepth)
	}
	if status.DroppedCount != 1 {
		t.Fatalf("expected dropped count 1, got %d", status.DroppedCount)
	}
}

func TestSinkRunFlushesQueuedEvents(t *testing.T) {
	store := &fakeStore{inserted: make(chan struct{}, 1)}
	sink := NewSink(store, SinkConfig{
		Enabled:       true,
		QueueSize:     4,
		BatchSize:     1,
		BatchInterval: 10 * time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sink.Run(ctx)

	sink.Enqueue(Event{Message: "write me"})

	select {
	case <-store.inserted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for diagnostic event insert")
	}
}
