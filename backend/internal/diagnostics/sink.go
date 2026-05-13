package diagnostics

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

const (
	defaultQueueSize     = 1000
	defaultBatchSize     = 50
	defaultBatchInterval = 2 * time.Second
	defaultInsertTimeout = 3 * time.Second
	defaultRetentionDays = 90
)

type SinkConfig struct {
	Enabled       bool
	QueueSize     int
	RetentionDays int
	BatchSize     int
	BatchInterval time.Duration
	InsertTimeout time.Duration
}

type Sink struct {
	store         Store
	queue         chan Event
	enabled       bool
	retentionDays int
	batchSize     int
	batchInterval time.Duration
	insertTimeout time.Duration
	dropped       atomic.Int64
	lastWriteErr  atomic.Value
	lastWriteAt   atomic.Int64
	lastNoticeAt  atomic.Int64
}

func NewSink(store Store, cfg SinkConfig) *Sink {
	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = defaultQueueSize
	}
	retentionDays := cfg.RetentionDays
	if retentionDays <= 0 {
		retentionDays = defaultRetentionDays
	}
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	batchInterval := cfg.BatchInterval
	if batchInterval <= 0 {
		batchInterval = defaultBatchInterval
	}
	insertTimeout := cfg.InsertTimeout
	if insertTimeout <= 0 {
		insertTimeout = defaultInsertTimeout
	}
	return &Sink{
		store:         store,
		queue:         make(chan Event, queueSize),
		enabled:       cfg.Enabled && store != nil,
		retentionDays: retentionDays,
		batchSize:     batchSize,
		batchInterval: batchInterval,
		insertTimeout: insertTimeout,
	}
}

func (s *Sink) Enabled() bool {
	return s != nil && s.enabled
}

func (s *Sink) Enqueue(event Event) {
	if !s.Enabled() {
		return
	}
	if event.ObservedAt.IsZero() {
		event.ObservedAt = time.Now()
	}
	event.DroppedBefore = s.dropped.Load()
	select {
	case s.queue <- event:
	default:
		s.dropped.Add(1)
	}
}

func (s *Sink) Run(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	flushTicker := time.NewTicker(s.batchInterval)
	defer flushTicker.Stop()
	cleanupTicker := time.NewTicker(time.Hour)
	defer cleanupTicker.Stop()

	batch := make([]Event, 0, s.batchSize)
	s.cleanup()
	for {
		select {
		case event := <-s.queue:
			batch = append(batch, event)
			if len(batch) >= s.batchSize {
				s.write(batch)
				batch = batch[:0]
			}
		case <-flushTicker.C:
			if len(batch) > 0 {
				s.write(batch)
				batch = batch[:0]
			}
		case <-cleanupTicker.C:
			s.cleanup()
		case <-ctx.Done():
			s.drainAndFlush(batch)
			return
		}
	}
}

func (s *Sink) Status() SinkStatus {
	if s == nil {
		return SinkStatus{}
	}
	var lastWriteAt *time.Time
	if raw := s.lastWriteAt.Load(); raw > 0 {
		value := time.Unix(0, raw)
		lastWriteAt = &value
	}
	lastErr, _ := s.lastWriteErr.Load().(string)
	return SinkStatus{
		Enabled:       s.Enabled(),
		QueueDepth:    len(s.queue),
		QueueCapacity: cap(s.queue),
		DroppedCount:  s.dropped.Load(),
		LastWriteErr:  lastErr,
		LastWriteAt:   lastWriteAt,
		RetentionDays: s.retentionDays,
	}
}

func (s *Sink) write(events []Event) {
	if len(events) == 0 || s.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.insertTimeout)
	defer cancel()
	if err := s.store.InsertEvents(ctx, events); err != nil {
		s.recordWriteError(err)
		return
	}
	s.lastWriteErr.Store("")
	s.lastWriteAt.Store(time.Now().UnixNano())
}

func (s *Sink) cleanup() {
	if s.store == nil || s.retentionDays <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.insertTimeout)
	defer cancel()
	if _, err := s.store.DeleteBefore(ctx, time.Now().AddDate(0, 0, -s.retentionDays)); err != nil {
		s.recordWriteError(err)
	}
}

func (s *Sink) drainAndFlush(batch []Event) {
	deadline := time.NewTimer(s.insertTimeout)
	defer deadline.Stop()
	for {
		select {
		case event := <-s.queue:
			batch = append(batch, event)
			if len(batch) >= s.batchSize {
				s.write(batch)
				batch = batch[:0]
			}
		case <-deadline.C:
			if len(batch) > 0 {
				s.write(batch)
			}
			return
		default:
			if len(batch) > 0 {
				s.write(batch)
			}
			return
		}
	}
}

func (s *Sink) recordWriteError(err error) {
	if err == nil {
		return
	}
	s.lastWriteErr.Store(err.Error())
	now := time.Now().UnixNano()
	last := s.lastNoticeAt.Load()
	if now-last < int64(time.Minute) {
		return
	}
	if s.lastNoticeAt.CompareAndSwap(last, now) {
		_, _ = fmt.Fprintf(os.Stderr, "diagnostic event write failed: %v\n", err)
	}
}
