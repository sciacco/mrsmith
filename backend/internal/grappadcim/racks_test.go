package grappadcim

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
)

func TestHandleListRacksUsesCustomerNameAndDeterministicFilters(t *testing.T) {
	h := &Handler{grappa: openGrappaDCIMRackTestDB(t, "list-racks")}

	req := httptest.NewRequest(http.MethodGet, "/grappa-dcim/v1/racks?q=Acme&customerId=22&datacenterId=9&status=Cessato", nil)
	rec := httptest.NewRecorder()
	h.handleListRacks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var rows []RackListItem
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	if rows[0].CustomerName == nil || *rows[0].CustomerName != "Acme Spa" {
		t.Fatalf("expected customer name from cli_fatturazione, got %#v", rows[0].CustomerName)
	}
}

func TestHandleRackFilterOptionsReturnsCustomerRoomAndStatusOptions(t *testing.T) {
	h := &Handler{grappa: openGrappaDCIMRackTestDB(t, "filter-options")}

	req := httptest.NewRequest(http.MethodGet, "/grappa-dcim/v1/racks/filter-options", nil)
	rec := httptest.NewRecorder()
	h.handleRackFilterOptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body RackFilterOptions
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.Customers) != 1 || body.Customers[0].Label != "Acme Spa" {
		t.Fatalf("unexpected customers: %#v", body.Customers)
	}
	if len(body.Datacenters) != 1 || !body.Datacenters[0].IsMMR || body.Datacenters[0].Label != "MMR - DC Milano / MMR 1" {
		t.Fatalf("unexpected datacenters: %#v", body.Datacenters)
	}
	if len(body.Statuses) != 3 || body.Statuses[0].Label != "Solo attivi" || body.Statuses[1].Label != "Tutti" || body.Statuses[2].Label != "Cessato" {
		t.Fatalf("unexpected statuses: %#v", body.Statuses)
	}
}

func openGrappaDCIMRackTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()
	registerGrappaDCIMRackTestDriver()

	db, err := sql.Open(grappaDCIMRackTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const grappaDCIMRackTestDriverName = "grappadcim_rack_test_driver"

var registerGrappaDCIMRackTestDriverOnce sync.Once

func registerGrappaDCIMRackTestDriver() {
	registerGrappaDCIMRackTestDriverOnce.Do(func() {
		sql.Register(grappaDCIMRackTestDriverName, grappaDCIMRackTestDriver{})
	})
}

type grappaDCIMRackTestDriver struct{}

func (grappaDCIMRackTestDriver) Open(name string) (driver.Conn, error) {
	return &grappaDCIMRackTestConn{mode: name}, nil
}

type grappaDCIMRackTestConn struct {
	mode string
}

func (c *grappaDCIMRackTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *grappaDCIMRackTestConn) Close() error { return nil }

func (c *grappaDCIMRackTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *grappaDCIMRackTestConn) Ping(context.Context) error { return nil }

func (c *grappaDCIMRackTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch c.mode {
	case "list-racks":
		if !strings.Contains(query, "LEFT JOIN cli_fatturazione cf ON cf.id = r.id_anagrafica") {
			return nil, errors.New("missing cli_fatturazione customer join")
		}
		if !strings.Contains(query, "cf.intestazione LIKE ?") {
			return nil, errors.New("free search does not include customer name")
		}
			if !strings.Contains(query, "r.id_anagrafica = ?") || !strings.Contains(query, "r.id_datacenter = ?") || !strings.Contains(query, "r.stato = ?") {
				return nil, errors.New("missing deterministic rack filters")
			}
			if !strings.Contains(query, "ORDER BY CASE WHEN TRIM(COALESCE(db.name, '')) = '' THEN 1 ELSE 0 END ASC, db.name ASC, d.name ASC, r.name ASC, r.stato ASC, r.id_rack ASC") {
				return nil, errors.New("unexpected default rack ordering")
			}
			if len(args) != 7 {
				return nil, errors.New("unexpected list-racks args length")
			}
		if args[0].Value != "Cessato" || args[1].Value != int64(9) || args[2].Value != int64(22) {
			return nil, errors.New("unexpected deterministic filter args")
		}
		for i := 3; i < 7; i++ {
			if args[i].Value != "%Acme%" {
				return nil, errors.New("unexpected free-search arg")
			}
		}
		return &grappaDCIMRackTestRows{
			columns: rackTestSelectColumns(),
			values: [][]driver.Value{
				{
					int64(44), "Rack A01", int64(42), int64(22), "Acme Spa", int64(9), "MMR 1", "DC Milano",
					"Cessato", "MT-1", int64(16), int64(1), int64(2), "Full", "F", int64(1),
					int64(101), int64(201), "No", "No", "Note", nil, nil, "ORD-1", float64(4.5), "SN-1", float64(3.2), int64(1), int64(2),
				},
			},
		}, nil
	case "filter-options":
		switch {
		case strings.Contains(query, "FROM cli_fatturazione cf"):
			return &grappaDCIMRackTestRows{
				columns: []string{"id", "intestazione"},
				values:  [][]driver.Value{{int64(22), "Acme Spa"}},
			}, nil
		case strings.Contains(query, "FROM datacenter d"):
			return &grappaDCIMRackTestRows{
				columns: []string{"id_datacenter", "name", "building_name", "ismmr"},
				values:  [][]driver.Value{{int64(9), "MMR 1", "DC Milano", int64(1)}},
			}, nil
		case strings.Contains(query, "SELECT DISTINCT TRIM(r.stato)"):
			return &grappaDCIMRackTestRows{
				columns: []string{"stato"},
				values:  [][]driver.Value{{"Attivo"}, {"Cessato"}},
			}, nil
		}
	}

	return nil, errors.New("unexpected query for mode: " + c.mode)
}

func rackTestSelectColumns() []string {
	return []string{
		"id_rack", "name", "unit", "id_anagrafica", "intestazione", "id_datacenter", "datacenter_name", "building_name",
		"stato", "magnetotermico", "ampere", "floor", "island", "type", "pos", "racknum",
		"positions_id", "islet_id", "shared", "reserved", "note", "data_attivazione", "data_cessazione",
		"codice_ordine", "sold_power", "serialnumber", "committed_power", "variable_billing", "socket_count",
	}
}

type grappaDCIMRackTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *grappaDCIMRackTestRows) Columns() []string { return r.columns }

func (r *grappaDCIMRackTestRows) Close() error { return nil }

func (r *grappaDCIMRackTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

var _ driver.QueryerContext = (*grappaDCIMRackTestConn)(nil)
var _ driver.Pinger = (*grappaDCIMRackTestConn)(nil)
