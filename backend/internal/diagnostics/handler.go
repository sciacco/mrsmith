package diagnostics

import (
	"context"
	"log/slog"
	"runtime"
)

type EventRecorder interface {
	Enqueue(Event)
}

type SlogHandler struct {
	next     slog.Handler
	recorder EventRecorder
	attrs    []slog.Attr
	groups   []string
}

func NewSlogHandler(next slog.Handler, recorder EventRecorder) *SlogHandler {
	return &SlogHandler{next: next, recorder: recorder}
}

func (h *SlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *SlogHandler) Handle(ctx context.Context, record slog.Record) error {
	nextErr := h.next.Handle(ctx, record)
	if h.recorder != nil && record.Level >= slog.LevelWarn {
		h.recorder.Enqueue(eventFromRecord(record, h.attrs, h.groups))
	}
	return nextErr
}

func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nextAttrs := make([]slog.Attr, len(attrs))
	copy(nextAttrs, attrs)
	return &SlogHandler{
		next:     h.next.WithAttrs(attrs),
		recorder: h.recorder,
		attrs:    append(cloneAttrs(h.attrs), nextAttrs...),
		groups:   append([]string(nil), h.groups...),
	}
}

func (h *SlogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &SlogHandler{
		next:     h.next.WithGroup(name),
		recorder: h.recorder,
		attrs:    cloneAttrs(h.attrs),
		groups:   append(append([]string(nil), h.groups...), name),
	}
}

func eventFromRecord(record slog.Record, handlerAttrs []slog.Attr, groups []string) Event {
	attrs := make(map[string]any)
	for _, attr := range handlerAttrs {
		addAttr(attrs, attr, nil)
	}
	record.Attrs(func(attr slog.Attr) bool {
		addAttr(attrs, attr, groups)
		return true
	})
	attrs = sanitizeAttrs(attrs)

	sourceFile, sourceLine, sourceFunction := sourceFromPC(record.PC)
	stack := stringField(attrs, "stack")
	if stack != "" {
		stack = truncateString(stack, maxStackBytes)
	}

	return Event{
		ObservedAt:     record.Time,
		Level:          record.Level.String(),
		Message:        truncateString(record.Message, maxMessageBytes),
		Component:      truncateString(stringField(attrs, "component"), maxStringBytes),
		Operation:      truncateString(stringField(attrs, "operation"), maxStringBytes),
		RequestID:      truncateString(stringField(attrs, "request_id"), maxStringBytes),
		Method:         truncateString(stringField(attrs, "method"), maxStringBytes),
		Path:           truncateString(stringField(attrs, "path"), maxStringBytes),
		Status:         intField(attrs, "status"),
		AuthSubject:    truncateString(stringField(attrs, "auth_subject"), maxStringBytes),
		Error:          truncateString(stringField(attrs, "error"), maxErrorBytes),
		SourceFile:     sourceFile,
		SourceLine:     sourceLine,
		SourceFunction: sourceFunction,
		Attrs:          attrs,
		Stack:          stack,
	}
}

func addAttr(attrs map[string]any, attr slog.Attr, groups []string) {
	if attr.Key == "" {
		return
	}
	value := slogValueToAny(attr.Value)
	if len(groups) == 0 {
		attrs[attr.Key] = value
		return
	}
	current := attrs
	for _, group := range groups {
		raw, ok := current[group]
		if !ok {
			next := make(map[string]any)
			current[group] = next
			current = next
			continue
		}
		next, ok := raw.(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[group] = next
		}
		current = next
	}
	current[attr.Key] = value
}

func sourceFromPC(pc uintptr) (string, int, string) {
	if pc == 0 {
		return "", 0, ""
	}
	frames := runtime.CallersFrames([]uintptr{pc})
	frame, _ := frames.Next()
	return frame.File, frame.Line, frame.Function
}

func cloneAttrs(attrs []slog.Attr) []slog.Attr {
	if len(attrs) == 0 {
		return nil
	}
	cloned := make([]slog.Attr, len(attrs))
	copy(cloned, attrs)
	return cloned
}
