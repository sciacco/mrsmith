package quotes

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/sciacco/mrsmith/internal/platform/hubspot"
)

func TestMapHubSpotQuoteStatus(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "approved", raw: "APPROVED", want: "APPROVED", ok: true},
		{name: "approval not needed", raw: " approval_not_needed ", want: "APPROVAL_NOT_NEEDED", ok: true},
		{name: "rejected", raw: "REJECTED", want: "REJECTED", ok: true},
		{name: "pending skipped", raw: "PENDING_APPROVAL", ok: false},
		{name: "draft skipped", raw: "DRAFT", ok: false},
		{name: "empty skipped", raw: "", ok: false},
		{name: "unknown skipped", raw: "SENT", ok: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := mapHubSpotQuoteStatus(tt.raw)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("mapHubSpotQuoteStatus(%q) = %q, %v; want %q, %v", tt.raw, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestHubSpotStatusSyncProcessOnceUpdatesSupportedStatusesAndContinuesAfterErrors(t *testing.T) {
	state := newStatusSyncTestState()
	state.configRaw = []byte(`{"enabled":true,"interval_seconds":60,"batch_size":2}`)
	state.records = []pendingHubSpotQuote{
		{ID: 1, HSQuoteID: 101},
		{ID: 2, HSQuoteID: 102},
		{ID: 3, HSQuoteID: 103},
		{ID: 4, HSQuoteID: 104},
		{ID: 5, HSQuoteID: 105},
		{ID: 6, HSQuoteID: 106},
	}
	db := openStatusSyncTestDB(t, "process", state)

	hs := &fakeHubSpotStatusProvider{
		statuses: map[int64]string{
			101: "APPROVED",
			102: "PENDING_APPROVAL",
			103: "REJECTED",
			105: "DRAFT",
			106: "APPROVAL_NOT_NEEDED",
		},
		errors: map[int64]error{
			104: errors.New("hubspot unavailable"),
		},
	}
	worker := NewHubSpotStatusSyncWorker(HubSpotStatusSyncDeps{
		Mistra:        db,
		RuntimeConfig: db,
		HubSpot:       hs,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	stats, err := worker.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce returned error: %v", err)
	}

	if stats.Checked != 6 || stats.Updated != 3 || stats.Skipped != 2 || stats.Errors != 1 || stats.LockSkipped {
		t.Fatalf("stats = %#v, want checked=6 updated=3 skipped=2 errors=1 lockSkipped=false", stats)
	}
	if len(hs.calls) != 6 {
		t.Fatalf("hubspot calls = %#v, want 6 calls", hs.calls)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.unlocked {
		t.Fatal("expected advisory lock to be released")
	}
	wantUpdates := map[int]string{
		1: "APPROVED",
		3: "REJECTED",
		6: "APPROVAL_NOT_NEEDED",
	}
	if len(state.updates) != len(wantUpdates) {
		t.Fatalf("updates = %#v, want %#v", state.updates, wantUpdates)
	}
	for id, want := range wantUpdates {
		if got := state.updates[id]; got != want {
			t.Fatalf("update[%d] = %q, want %q", id, got, want)
		}
	}
}

func TestHubSpotStatusSyncSkipsWhenAdvisoryLockIsHeld(t *testing.T) {
	state := newStatusSyncTestState()
	state.lockAvailable = false
	state.records = []pendingHubSpotQuote{{ID: 1, HSQuoteID: 101}}
	db := openStatusSyncTestDB(t, "locked", state)
	hs := &fakeHubSpotStatusProvider{statuses: map[int64]string{101: "APPROVED"}}
	worker := NewHubSpotStatusSyncWorker(HubSpotStatusSyncDeps{
		Mistra:  db,
		HubSpot: hs,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	stats, err := worker.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce returned error: %v", err)
	}
	if !stats.LockSkipped || stats.Checked != 0 || stats.Updated != 0 {
		t.Fatalf("stats = %#v, want lock skipped with no work", stats)
	}
	if len(hs.calls) != 0 {
		t.Fatalf("hubspot calls = %#v, want none", hs.calls)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.unlocked {
		t.Fatal("did not expect unlock when advisory lock was not acquired")
	}
}

func TestHubSpotStatusSyncRuntimeConfigCanDisableWorker(t *testing.T) {
	state := newStatusSyncTestState()
	state.configRaw = []byte(`{"enabled":false,"interval_seconds":60,"batch_size":2}`)
	state.records = []pendingHubSpotQuote{{ID: 1, HSQuoteID: 101}}
	db := openStatusSyncTestDB(t, "disabled", state)
	hs := &fakeHubSpotStatusProvider{statuses: map[int64]string{101: "APPROVED"}}
	worker := NewHubSpotStatusSyncWorker(HubSpotStatusSyncDeps{
		Mistra:        db,
		RuntimeConfig: db,
		HubSpot:       hs,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	stats, err := worker.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("ProcessOnce returned error: %v", err)
	}
	if stats != (HubSpotStatusSyncStats{}) {
		t.Fatalf("stats = %#v, want zero value", stats)
	}
	if len(hs.calls) != 0 {
		t.Fatalf("hubspot calls = %#v, want none", hs.calls)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.lockQueries != 0 {
		t.Fatalf("lock queries = %d, want 0", state.lockQueries)
	}
}

type fakeHubSpotStatusProvider struct {
	statuses map[int64]string
	errors   map[int64]error
	calls    []int64
}

func (p *fakeHubSpotStatusProvider) GetQuoteStatus(_ context.Context, quoteID int64) (*hubspot.QuoteStatus, error) {
	p.calls = append(p.calls, quoteID)
	if err := p.errors[quoteID]; err != nil {
		return nil, err
	}
	return &hubspot.QuoteStatus{
		Properties: map[string]string{"hs_status": p.statuses[quoteID]},
	}, nil
}

func openStatusSyncTestDB(t *testing.T, name string, state *statusSyncTestState) *sql.DB {
	t.Helper()
	registerStatusSyncTestDriver()
	statusSyncStates.Store(name, state)

	db, err := sql.Open(statusSyncTestDriverName, name)
	if err != nil {
		t.Fatalf("failed to open status sync test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		statusSyncStates.Delete(name)
	})
	return db
}

const statusSyncTestDriverName = "quotes_status_sync_test_driver"

var (
	registerStatusSyncDriverOnce sync.Once
	statusSyncStates             sync.Map
)

type statusSyncTestState struct {
	mu             sync.Mutex
	lockAvailable  bool
	lockQueries    int
	unlocked       bool
	configRaw      []byte
	records        []pendingHubSpotQuote
	updates        map[int]string
	updateAffected map[int]int64
}

func newStatusSyncTestState() *statusSyncTestState {
	return &statusSyncTestState{
		lockAvailable:  true,
		updates:        map[int]string{},
		updateAffected: map[int]int64{},
	}
}

func registerStatusSyncTestDriver() {
	registerStatusSyncDriverOnce.Do(func() {
		sql.Register(statusSyncTestDriverName, statusSyncTestDriver{})
	})
}

type statusSyncTestDriver struct{}

func (statusSyncTestDriver) Open(name string) (driver.Conn, error) {
	value, ok := statusSyncStates.Load(name)
	if !ok {
		return nil, errors.New("missing status sync test state")
	}
	return &statusSyncTestConn{state: value.(*statusSyncTestState)}, nil
}

type statusSyncTestConn struct {
	state *statusSyncTestState
}

func (c *statusSyncTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *statusSyncTestConn) Close() error { return nil }

func (c *statusSyncTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *statusSyncTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	switch {
	case stringsContains(query, "SELECT pg_try_advisory_lock"):
		c.state.lockQueries++
		return statusSyncBoolRows("pg_try_advisory_lock", c.state.lockAvailable), nil
	case stringsContains(query, "SELECT pg_advisory_unlock"):
		c.state.unlocked = true
		return statusSyncBoolRows("pg_advisory_unlock", true), nil
	case stringsContains(query, "FROM mrsmith.runtime_config"):
		if len(c.state.configRaw) == 0 {
			return &statusSyncRows{columns: []string{"value"}}, nil
		}
		return &statusSyncRows{
			columns: []string{"value"},
			values:  [][]driver.Value{{c.state.configRaw}},
		}, nil
	case stringsContains(query, "FROM quotes.quote") && stringsContains(query, "status = 'PENDING_APPROVAL'"):
		afterID := statusSyncNamedInt(args[0])
		batchSize := statusSyncNamedInt(args[1])
		values := [][]driver.Value{}
		for _, record := range c.state.records {
			if record.ID <= afterID {
				continue
			}
			values = append(values, []driver.Value{int64(record.ID), record.HSQuoteID})
			if len(values) == batchSize {
				break
			}
		}
		return &statusSyncRows{
			columns: []string{"id", "hs_quote_id"},
			values:  values,
		}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

func (c *statusSyncTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	switch {
	case stringsContains(query, "UPDATE quotes.quote"):
		status := statusSyncNamedString(args[0])
		id := statusSyncNamedInt(args[1])
		affected, ok := c.state.updateAffected[id]
		if !ok {
			affected = 1
		}
		if affected > 0 {
			c.state.updates[id] = status
		}
		return driver.RowsAffected(affected), nil
	default:
		return nil, errors.New("unexpected exec: " + query)
	}
}

var _ driver.QueryerContext = (*statusSyncTestConn)(nil)
var _ driver.ExecerContext = (*statusSyncTestConn)(nil)

type statusSyncRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func statusSyncBoolRows(column string, value bool) *statusSyncRows {
	return &statusSyncRows{
		columns: []string{column},
		values:  [][]driver.Value{{value}},
	}
}

func (r *statusSyncRows) Columns() []string { return r.columns }

func (r *statusSyncRows) Close() error { return nil }

func (r *statusSyncRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func statusSyncNamedString(value driver.NamedValue) string {
	if v, ok := value.Value.(string); ok {
		return v
	}
	return ""
}

func statusSyncNamedInt(value driver.NamedValue) int {
	switch v := value.Value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}

func stringsContains(s, substr string) bool {
	return strings.Contains(s, substr)
}
