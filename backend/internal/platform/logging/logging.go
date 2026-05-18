package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
)

type contextKey string

const (
	loggerKey         contextKey = "logger"
	requestIDKey      contextKey = "request_id"
	accessLogAttrsKey contextKey = "access_log_attrs"
)

type accessLogAttrs struct {
	mu    sync.Mutex
	attrs map[string]any
}

// New returns a JSON slog logger writing to stdout.
func New(level string) *slog.Logger {
	return NewWithWriter(os.Stdout, level)
}

// NewWithWriter builds a JSON slog logger for tests and process wiring.
func NewWithWriter(w io.Writer, level string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	return slog.New(slog.NewJSONHandler(w, opts))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func IntoContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

func WithAttrs(ctx context.Context, args ...any) context.Context {
	return IntoContext(ctx, FromContext(ctx).With(args...))
}

func WithAccessLogAttrs(ctx context.Context) context.Context {
	if _, ok := ctx.Value(accessLogAttrsKey).(*accessLogAttrs); ok {
		return ctx
	}
	return context.WithValue(ctx, accessLogAttrsKey, &accessLogAttrs{attrs: make(map[string]any)})
}

func AddAccessLogAttrs(ctx context.Context, args ...any) {
	collector, ok := ctx.Value(accessLogAttrsKey).(*accessLogAttrs)
	if !ok || collector == nil {
		return
	}

	collector.mu.Lock()
	defer collector.mu.Unlock()
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok || key == "" {
			continue
		}
		collector.attrs[key] = args[i+1]
	}
}

func AccessLogAttrs(ctx context.Context) []any {
	collector, ok := ctx.Value(accessLogAttrsKey).(*accessLogAttrs)
	if !ok || collector == nil {
		return nil
	}

	collector.mu.Lock()
	defer collector.mu.Unlock()
	keys := make([]string, 0, len(collector.attrs))
	for key := range collector.attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	args := make([]any, 0, len(keys)*2)
	for _, key := range keys {
		args = append(args, key, collector.attrs[key])
	}
	return args
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
