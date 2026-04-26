package manutenzioni

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHandleMaintenanceRadarReturnsOperationalSixMonthScope(t *testing.T) {
	db, state := openMaintenanceRadarTestDB(t, "scope")
	h := &Handler{maintenance: db}
	req := httptest.NewRequest(http.MethodGet, "/manutenzioni/v1/maintenances/radar", nil)
	rec := httptest.NewRecorder()

	h.handleMaintenanceRadar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp maintenanceRadarResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	codes := make([]string, 0, len(resp.Items))
	for _, item := range resp.Items {
		codes = append(codes, item.Code)
	}
	want := []string{"scheduled-today", "scheduled-day-7", "scheduled-day-8", "scheduled-day-52", "scheduled-day-53", "scheduled-six-months", "unscheduled"}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("codes = %#v, want %#v", codes, want)
	}
	if !state.sawRadarPredicate {
		t.Fatalf("radar query did not include six-month/unscheduled predicate")
	}
	if !state.sawTerminalStatusExclusion {
		t.Fatalf("radar query did not exclude terminal statuses")
	}
}

func TestHandleMaintenanceRadarAppliesNonDateFiltersAndIgnoresDateAndPagination(t *testing.T) {
	db, state := openMaintenanceRadarTestDB(t, "filters")
	h := &Handler{maintenance: db}
	req := httptest.NewRequest(http.MethodGet, "/manutenzioni/v1/maintenances/radar?q=edge&status=scheduled&technical_domain_id=2&maintenance_kind_id=3&customer_scope_id=4&site_id=5&scheduled_from=2099-01-01&scheduled_to=2099-02-01&page=9&page_size=99", nil)
	rec := httptest.NewRecorder()

	h.handleMaintenanceRadar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	state.mu.Lock()
	query := state.lastQuery
	args := append([]driver.NamedValue{}, state.lastArgs...)
	state.mu.Unlock()

	for _, fragment := range []string{
		"ILIKE",
		"m.status IN",
		"m.status NOT IN",
		"m.technical_domain_id =",
		"m.maintenance_kind_id =",
		"m.customer_scope_id =",
		"m.site_id =",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("radar query missing %q in %s", fragment, query)
		}
	}
	if strings.Contains(query, "vcw.scheduled_start_at::date >=") || strings.Contains(query, "vcw.scheduled_start_at::date <=") {
		t.Fatalf("radar query should ignore scheduled_from/scheduled_to filters: %s", query)
	}
	for _, arg := range args {
		switch arg.Value {
		case "2099-01-01", "2099-02-01", int64(9), int64(99), 9, 99:
			t.Fatalf("radar query passed ignored filter/pagination arg: %#v", arg.Value)
		}
	}
	for _, expected := range []driver.Value{"%edge%", "scheduled", int64(2), int64(3), int64(4), int64(5)} {
		if !radarArgsContain(args, expected) {
			t.Fatalf("radar query args missing applied filter %#v in %#v", expected, args)
		}
	}
	for _, excluded := range []driver.Value{StatusCancelled, StatusSuperseded} {
		if !radarArgsContain(args, excluded) {
			t.Fatalf("radar query args missing terminal status exclusion %#v in %#v", excluded, args)
		}
	}
}

func TestHandleMaintenanceRadarAlwaysExcludesTerminalStatuses(t *testing.T) {
	db, state := openMaintenanceRadarTestDB(t, "terminal-statuses")
	h := &Handler{maintenance: db}
	req := httptest.NewRequest(http.MethodGet, "/manutenzioni/v1/maintenances/radar?status=cancelled,superseded", nil)
	rec := httptest.NewRecorder()

	h.handleMaintenanceRadar(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	state.mu.Lock()
	query := state.lastQuery
	args := append([]driver.NamedValue{}, state.lastArgs...)
	state.mu.Unlock()

	if !strings.Contains(query, "m.status IN") {
		t.Fatalf("radar query should preserve caller status filter: %s", query)
	}
	if !strings.Contains(query, "m.status NOT IN") {
		t.Fatalf("radar query should exclude terminal statuses: %s", query)
	}
	for _, expected := range []driver.Value{StatusCancelled, StatusSuperseded} {
		if !radarArgsContain(args, expected) {
			t.Fatalf("radar query args missing expected status %#v in %#v", expected, args)
		}
	}
}

const maintenanceRadarTestDriverName = "manutenzioni_radar_test_driver"

var (
	registerMaintenanceRadarDriverOnce sync.Once
	maintenanceRadarTestStates         sync.Map
)

type maintenanceRadarTestState struct {
	mu                         sync.Mutex
	lastQuery                  string
	lastArgs                   []driver.NamedValue
	sawRadarPredicate          bool
	sawTerminalStatusExclusion bool
}

func openMaintenanceRadarTestDB(t *testing.T, mode string) (*sql.DB, *maintenanceRadarTestState) {
	t.Helper()
	registerMaintenanceRadarDriverOnce.Do(func() {
		sql.Register(maintenanceRadarTestDriverName, maintenanceRadarTestDriver{})
	})
	state := &maintenanceRadarTestState{}
	maintenanceRadarTestStates.Store(mode, state)
	db, err := sql.Open(maintenanceRadarTestDriverName, mode)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		maintenanceRadarTestStates.Delete(mode)
	})
	return db, state
}

type maintenanceRadarTestDriver struct{}

func (maintenanceRadarTestDriver) Open(name string) (driver.Conn, error) {
	return &maintenanceRadarTestConn{mode: name}, nil
}

type maintenanceRadarTestConn struct {
	mode string
}

func (c *maintenanceRadarTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *maintenanceRadarTestConn) Close() error { return nil }

func (c *maintenanceRadarTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *maintenanceRadarTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	value, ok := maintenanceRadarTestStates.Load(c.mode)
	if !ok {
		return nil, errors.New("missing radar test state")
	}
	state := value.(*maintenanceRadarTestState)
	state.mu.Lock()
	state.lastQuery = query
	state.lastArgs = append([]driver.NamedValue{}, args...)
	state.sawRadarPredicate = strings.Contains(query, "vcw.maintenance_window_id IS NULL") &&
		strings.Contains(query, "vcw.scheduled_start_at::date BETWEEN")
	state.sawTerminalStatusExclusion = strings.Contains(query, "m.status NOT IN")
	state.mu.Unlock()

	today, sixMonthsTo, err := maintenanceRadarBoundsFromArgs(args)
	if err != nil {
		return nil, err
	}
	return maintenanceRadarRowsForFixtures(today, sixMonthsTo), nil
}

var _ driver.QueryerContext = (*maintenanceRadarTestConn)(nil)

func maintenanceRadarBoundsFromArgs(args []driver.NamedValue) (time.Time, time.Time, error) {
	if len(args) < 2 {
		return time.Time{}, time.Time{}, errors.New("radar query missing date args")
	}
	todayRaw, ok := args[len(args)-2].Value.(string)
	if !ok {
		return time.Time{}, time.Time{}, errors.New("today arg is not a string")
	}
	sixMonthsRaw, ok := args[len(args)-1].Value.(string)
	if !ok {
		return time.Time{}, time.Time{}, errors.New("six months arg is not a string")
	}
	today, err := time.Parse("2006-01-02", todayRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	sixMonthsTo, err := time.Parse("2006-01-02", sixMonthsRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return today, sixMonthsTo, nil
}

func maintenanceRadarRowsForFixtures(today time.Time, sixMonthsTo time.Time) driver.Rows {
	fixtures := []struct {
		code      string
		scheduled *time.Time
	}{
		{code: "scheduled-today", scheduled: radarTimePtr(today)},
		{code: "scheduled-day-7", scheduled: radarTimePtr(today.AddDate(0, 0, 7))},
		{code: "scheduled-day-8", scheduled: radarTimePtr(today.AddDate(0, 0, 8))},
		{code: "scheduled-day-52", scheduled: radarTimePtr(today.AddDate(0, 0, 52))},
		{code: "scheduled-day-53", scheduled: radarTimePtr(today.AddDate(0, 0, 53))},
		{code: "scheduled-six-months", scheduled: radarTimePtr(sixMonthsTo)},
		{code: "scheduled-after-six-months", scheduled: radarTimePtr(sixMonthsTo.AddDate(0, 0, 1))},
		{code: "unscheduled"},
	}
	values := make([][]driver.Value, 0, len(fixtures))
	for index, fixture := range fixtures {
		if fixture.scheduled != nil {
			if fixture.scheduled.Before(today) || fixture.scheduled.After(sixMonthsTo) {
				continue
			}
		}
		values = append(values, maintenanceRadarRow(int64(index+1), fixture.code, fixture.scheduled))
	}
	return &maintenanceRadarRows{values: values}
}

func radarTimePtr(value time.Time) *time.Time {
	return &value
}

func radarArgsContain(args []driver.NamedValue, expected driver.Value) bool {
	for _, arg := range args {
		if arg.Value == expected {
			return true
		}
	}
	return false
}

func maintenanceRadarRow(id int64, code string, scheduled *time.Time) []driver.Value {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	var windowID, seqNo, expectedDowntime driver.Value
	var windowStatus driver.Value
	var scheduledStart, scheduledEnd driver.Value
	if scheduled != nil {
		windowID = id * 10
		seqNo = int64(1)
		windowStatus = "planned"
		scheduledStart = *scheduled
		scheduledEnd = scheduled.Add(2 * time.Hour)
		expectedDowntime = int64(30)
	}
	return []driver.Value{
		id,
		code,
		"Titolo " + code,
		nil,
		StatusScheduled,
		int64(1), "kind", "Tipo", nil, nil, int64(1), true,
		int64(2), "domain", "Dominio", nil, nil, int64(1), true,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		windowID, seqNo, windowStatus, scheduledStart, scheduledEnd, expectedDowntime,
		"Impatto",
		[]byte(`[]`),
		now,
		now,
	}
}

type maintenanceRadarRows struct {
	values [][]driver.Value
	index  int
}

func (r *maintenanceRadarRows) Columns() []string {
	return make([]string, 43)
}

func (r *maintenanceRadarRows) Close() error { return nil }

func (r *maintenanceRadarRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
