package energiadc

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/auth"
)

func TestRegisterRoutesEnforcesACLAndNilDBFallback(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, nil, ModuleConfig{})

	t.Run("missing claims", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/energia-dc/v1/customers", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("missing role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/energia-dc/v1/customers", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
			Name:  "Viewer",
			Email: "viewer@example.com",
			Roles: []string{"viewer"},
		}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("valid role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/energia-dc/v1/customers", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
			Name:  "Energia User",
			Email: "energia@example.com",
			Roles: []string{"app_energiadc_access"},
		}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "energia_dc_database_not_configured") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
}

func TestFormulaHelpers(t *testing.T) {
	if got := breakerCapacity("trifase 32A"); got != 63 {
		t.Fatalf("expected trifase 32A breaker capacity 63, got %v", got)
	}
	if got := breakerCapacity("monofase 16A"); got != 16 {
		t.Fatalf("expected monofase 16A breaker capacity 16, got %v", got)
	}
	if got := breakerCapacity("qualcosaltro"); got != 32 {
		t.Fatalf("expected fallback breaker capacity 32, got %v", got)
	}
	if got := gaugePercent(30, 63); got != 95.2 {
		t.Fatalf("expected gauge percent 95.2, got %v", got)
	}
	if got := cosfiMultiplier(95); got != 0.95 {
		t.Fatalf("expected cosfi multiplier 0.95, got %v", got)
	}
	if got := kilowattFromAmpere(20); got != 4.5 {
		t.Fatalf("expected kilowatt 4.5, got %v", got)
	}
}

func TestHandleListRoomsUsesSiteAndCustomerInvariant(t *testing.T) {
	h := &Handler{grappaDB: openEnergiaDCTestDB(t, "rooms"), config: normalizeConfig(ModuleConfig{})}

	req := httptest.NewRequest(http.MethodGet, "/energia-dc/v1/sites/7/rooms?customerId=22", nil)
	req.SetPathValue("siteId", "7")
	rec := httptest.NewRecorder()

	h.handleListRooms(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleListRacksUsesRoomAndCustomerInvariant(t *testing.T) {
	h := &Handler{grappaDB: openEnergiaDCTestDB(t, "racks"), config: normalizeConfig(ModuleConfig{})}

	req := httptest.NewRequest(http.MethodGet, "/energia-dc/v1/rooms/9/racks?customerId=22", nil)
	req.SetPathValue("roomId", "9")
	rec := httptest.NewRecorder()

	h.handleListRacks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleListPowerReadingsMergesCountAndPageResults(t *testing.T) {
	h := &Handler{grappaDB: openEnergiaDCTestDB(t, "power-readings"), config: normalizeConfig(ModuleConfig{})}

	req := httptest.NewRequest(
		http.MethodGet,
		"/energia-dc/v1/racks/12/power-readings?from=2026-04-15T10:00&to=2026-04-18T12:00&page=2&size=2",
		nil,
	)
	req.SetPathValue("rackId", "12")
	rec := httptest.NewRecorder()

	h.handleListPowerReadings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body powerReadingsPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Total != 5 {
		t.Fatalf("expected total 5, got %d", body.Total)
	}
	if body.Page != 2 || body.Size != 2 {
		t.Fatalf("unexpected pagination: page=%d size=%d", body.Page, body.Size)
	}
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(body.Items))
	}
	if body.Items[0].SocketID != 91 || body.Items[0].SocketLabel != "A1 / PDU-1" {
		t.Fatalf("unexpected first row: %#v", body.Items[0])
	}
}

func TestHandleListPowerReadingsRejectsMalformedLocalDateTime(t *testing.T) {
	h := &Handler{grappaDB: openEnergiaDCTestDB(t, "rooms"), config: normalizeConfig(ModuleConfig{})}
	req := httptest.NewRequest(
		http.MethodGet,
		"/energia-dc/v1/racks/12/power-readings?from=2026-04-15T10:00:00&to=2026-04-18T12:00",
		nil,
	)
	req.SetPathValue("rackId", "12")
	rec := httptest.NewRecorder()

	h.handleListPowerReadings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid_from_parameter") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleListPowerReadingsRejectsInvalidPagination(t *testing.T) {
	h := &Handler{grappaDB: openEnergiaDCTestDB(t, "rooms"), config: normalizeConfig(ModuleConfig{})}
	req := httptest.NewRequest(
		http.MethodGet,
		"/energia-dc/v1/racks/12/power-readings?from=2026-04-15T10:00&to=2026-04-18T12:00&page=0",
		nil,
	)
	req.SetPathValue("rackId", "12")
	rec := httptest.NewRecorder()

	h.handleListPowerReadings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid_page_parameter") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleListCustomerKWRejectsInvalidPeriodAndCosfi(t *testing.T) {
	h := &Handler{grappaDB: openEnergiaDCTestDB(t, "rooms"), config: normalizeConfig(ModuleConfig{})}

	req := httptest.NewRequest(http.MethodGet, "/energia-dc/v1/customers/11/kw?period=week&cosfi=95", nil)
	req.SetPathValue("customerId", "11")
	rec := httptest.NewRecorder()
	h.handleListCustomerKW(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_period_parameter") {
		t.Fatalf("unexpected period response: %d %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/energia-dc/v1/customers/11/kw?period=day&cosfi=60", nil)
	req.SetPathValue("customerId", "11")
	rec = httptest.NewRecorder()
	h.handleListCustomerKW(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_cosfi_parameter") {
		t.Fatalf("unexpected cosfi response: %d %q", rec.Code, rec.Body.String())
	}
}

func TestHandleListNoVariableRacksUsesCustomerIDNotName(t *testing.T) {
	h := &Handler{grappaDB: openEnergiaDCTestDB(t, "no-variable-racks"), config: normalizeConfig(ModuleConfig{})}

	req := httptest.NewRequest(http.MethodGet, "/energia-dc/v1/no-variable-billing/customers/44/racks", nil)
	req.SetPathValue("customerId", "44")
	rec := httptest.NewRecorder()

	h.handleListNoVariableRacks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExcludedCustomerConfigOnlyAffectsNoVariableCustomers(t *testing.T) {
	h := &Handler{
		grappaDB: openEnergiaDCTestDB(t, "no-variable-customers"),
		config: normalizeConfig(ModuleConfig{
			ExcludedCustomerIDs: []int{3, 9},
		}),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/energia-dc/v1/no-variable-billing/customers", nil)
	h.handleListNoVariableCustomers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleListLowConsumptionSupportsOptionalCustomer(t *testing.T) {
	h := &Handler{grappaDB: openEnergiaDCTestDB(t, "low-consumption"), config: normalizeConfig(ModuleConfig{
		ExcludedCustomerIDs: []int{3},
	})}

	req := httptest.NewRequest(http.MethodGet, "/energia-dc/v1/low-consumption?min=1", nil)
	rec := httptest.NewRecorder()
	h.handleListLowConsumption(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/energia-dc/v1/low-consumption?min=1&customerId=77", nil)
	rec = httptest.NewRecorder()
	h.handleListLowConsumption(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func openEnergiaDCTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()
	registerEnergiaDCTestDriver()

	db, err := sql.Open(energiaDCTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const energiaDCTestDriverName = "energiadc_test_driver"

var registerEnergiaDCTestDriverOnce sync.Once

func registerEnergiaDCTestDriver() {
	registerEnergiaDCTestDriverOnce.Do(func() {
		sql.Register(energiaDCTestDriverName, energiaDCTestDriver{})
	})
}

type energiaDCTestDriver struct{}

func (energiaDCTestDriver) Open(name string) (driver.Conn, error) {
	return &energiaDCTestConn{mode: name}, nil
}

type energiaDCTestConn struct {
	mode string
}

func (c *energiaDCTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *energiaDCTestConn) Close() error { return nil }

func (c *energiaDCTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *energiaDCTestConn) Ping(context.Context) error { return nil }

func (c *energiaDCTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch c.mode {
	case "rooms":
		if !strings.Contains(query, "WHERE d.dc_build_id = ?") || !strings.Contains(query, "AND r.id_anagrafica = ?") {
			return nil, errors.New("missing site/customer invariant in rooms query")
		}
		if len(args) != 2 || args[0].Value != int64(7) || args[1].Value != int64(22) {
			return nil, errors.New("unexpected rooms args")
		}
		return &energiaDCTestRows{
			columns: []string{"id_datacenter", "name"},
			values: [][]driver.Value{
				{int64(9), "Sala Blu"},
			},
		}, nil
	case "racks":
		if !strings.Contains(query, "WHERE r.id_datacenter = ?") || !strings.Contains(query, "AND r.id_anagrafica = ?") {
			return nil, errors.New("missing room/customer invariant in racks query")
		}
		if len(args) != 2 || args[0].Value != int64(9) || args[1].Value != int64(22) {
			return nil, errors.New("unexpected racks args")
		}
		return &energiaDCTestRows{
			columns: []string{"id_rack", "name"},
			values: [][]driver.Value{
				{int64(12), "Rack A-12"},
			},
		}, nil
	case "power-readings":
		if strings.Contains(query, "SELECT COUNT(*)") {
			if len(args) != 3 {
				return nil, errors.New("unexpected power-readings count args length")
			}
			if args[0].Value != int64(12) || args[1].Value != "2026-04-15 10:00:00" || args[2].Value != "2026-04-18 12:00:00" {
				return nil, errors.New("unexpected power-readings count args")
			}
			return &energiaDCTestRows{
				columns: []string{"count"},
				values:  [][]driver.Value{{int64(5)}},
			}, nil
		}
		if strings.Contains(query, "ORDER BY rpr.date DESC, rpr.id DESC") {
			if len(args) != 5 {
				return nil, errors.New("unexpected power-readings page args length")
			}
			if args[0].Value != int64(12) || args[3].Value != int64(2) || args[4].Value != int64(2) {
				return nil, errors.New("unexpected power-readings page args")
			}
			return &energiaDCTestRows{
				columns: []string{"id", "oid", "date", "ampere", "rack_socket_id", "posizione", "posizione2", "posizione3", "posizione4"},
				values: [][]driver.Value{
					{int64(101), "1.2.3", testTime(2026, 4, 18, 11, 30), float64(0.8), int64(91), "A1", "PDU-1", "", ""},
					{int64(100), "1.2.4", testTime(2026, 4, 18, 10, 30), float64(0.6), int64(92), "A2", "", "", ""},
				},
			}, nil
		}
	case "no-variable-racks":
		if strings.Contains(strings.ToLower(query), "intestazione") {
			return nil, errors.New("detail query should not be keyed by display name")
		}
		if !strings.Contains(query, "WHERE r.id_anagrafica = ?") {
			return nil, errors.New("detail query should be keyed by customer id")
		}
		if len(args) != 1 || args[0].Value != int64(44) {
			return nil, errors.New("unexpected no-variable args")
		}
		return &energiaDCTestRows{
			columns: []string{"id_rack", "name", "building_name", "room_name", "floor", "island", "type", "pos", "codice_ordine", "serialnumber", "variable_billing"},
			values: [][]driver.Value{
				{int64(11), "Rack 11", "Milano DC", "Sala A", int64(2), int64(4), "42U", "A-01", "ORD-1", "SN-1", nil},
			},
		}, nil
	case "no-variable-customers":
		if !strings.Contains(query, "NOT IN (?,?)") {
			return nil, errors.New("expected excluded-customer config in no-variable customer query")
		}
		if len(args) != 2 || args[0].Value != int64(3) || args[1].Value != int64(9) {
			return nil, errors.New("unexpected no-variable customer args")
		}
		return &energiaDCTestRows{
			columns: []string{"id", "intestazione"},
			values: [][]driver.Value{
				{int64(44), "Cliente Uno"},
			},
		}, nil
	case "low-consumption":
		if strings.Contains(query, "r.id_anagrafica = ?") {
			if len(args) != 3 || args[1].Value != int64(77) || args[2].Value != float64(1) {
				return nil, errors.New("unexpected low-consumption customer args")
			}
		} else {
			if len(args) != 2 || args[1].Value != float64(1) {
				return nil, errors.New("unexpected low-consumption args for all customers")
			}
		}
		if strings.Contains(query, "NOT IN") {
			return nil, errors.New("excluded-customer config must not affect generic low-consumption search")
		}
		return &energiaDCTestRows{
			columns: []string{"customer_id", "customer_name", "building_name", "room_name", "rack_name", "socket_id", "avg_ampere", "snmp_monitoring_device", "magnetotermico", "posizione", "posizione2", "posizione3", "posizione4"},
			values: [][]driver.Value{
				{int64(77), "Cliente Basso", "Milano DC", "Sala B", "Rack B1", int64(81), float64(0.5), "PDU-2", "monofase 16A", "B1", "", "", ""},
			},
		}, nil
	}

	return nil, errors.New("unexpected query for mode: " + c.mode)
}

type energiaDCTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *energiaDCTestRows) Columns() []string { return r.columns }

func (r *energiaDCTestRows) Close() error { return nil }

func (r *energiaDCTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func testTime(year int, month int, day int, hour int, minute int) time.Time {
	location := time.FixedZone("Europe/Rome", 3600)
	return time.Date(year, time.Month(month), day, hour, minute, 0, 0, location)
}

var _ driver.QueryerContext = (*energiaDCTestConn)(nil)
var _ driver.Pinger = (*energiaDCTestConn)(nil)
