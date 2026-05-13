package diagnostics

import (
	"context"
	"time"
)

const component = "diagnostics"

type Event struct {
	ID             int64          `json:"id"`
	ObservedAt     time.Time      `json:"observedAt"`
	Level          string         `json:"level"`
	Message        string         `json:"message"`
	Component      string         `json:"component"`
	Operation      string         `json:"operation"`
	RequestID      string         `json:"requestId"`
	Method         string         `json:"method"`
	Path           string         `json:"path"`
	Status         *int           `json:"status,omitempty"`
	AuthSubject    string         `json:"authSubject"`
	Error          string         `json:"error"`
	SourceFile     string         `json:"sourceFile"`
	SourceLine     int            `json:"sourceLine"`
	SourceFunction string         `json:"sourceFunction"`
	Attrs          map[string]any `json:"attrs"`
	Stack          string         `json:"stack,omitempty"`
	DroppedBefore  int64          `json:"droppedBefore"`
	CreatedAt      time.Time      `json:"createdAt"`
}

type Store interface {
	InsertEvents(ctx context.Context, events []Event) error
	ListEvents(ctx context.Context, filter ListFilter) ([]Event, error)
	GetEvent(ctx context.Context, id int64) (Event, bool, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

type ListFilter struct {
	Level     string
	Component string
	Operation string
	RequestID string
	Path      string
	Since     time.Time
	Before    time.Time
	Limit     int
}

type SinkStatus struct {
	Enabled       bool       `json:"enabled"`
	QueueDepth    int        `json:"queueDepth"`
	QueueCapacity int        `json:"queueCapacity"`
	DroppedCount  int64      `json:"droppedCount"`
	LastWriteErr  string     `json:"lastWriteError"`
	LastWriteAt   *time.Time `json:"lastWriteAt,omitempty"`
	RetentionDays int        `json:"retentionDays"`
}
