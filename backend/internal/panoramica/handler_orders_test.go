package panoramica

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

func TestHandleListOrdersSummaryAllowsNullTextFields(t *testing.T) {
	h := &Handler{mistraDB: openPanoramicaTestDB(t, "summary-null-text-fields")}

	req := httptest.NewRequest(http.MethodGet, "/panoramica/v1/orders/summary?cliente=123&stati=Evaso", nil)
	rec := httptest.NewRecorder()

	h.handleListOrdersSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var rows []struct {
		Stato             string   `json:"stato"`
		NumeroOrdine      string   `json:"numero_ordine"`
		DescrizioneLong   string   `json:"descrizione_long"`
		Quantita          *float64 `json:"quantita"`
		StatoOrdine       string   `json:"stato_ordine"`
		NomeTestataOrdine string   `json:"nome_testata_ordine"`
		StatoRiga         string   `json:"stato_riga"`
		Storico           *string  `json:"storico"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Stato != "" {
		t.Fatalf("expected empty stato for NULL source, got %q", rows[0].Stato)
	}
	if rows[0].NumeroOrdine != "" {
		t.Fatalf("expected empty numero_ordine for NULL source, got %q", rows[0].NumeroOrdine)
	}
	if rows[0].DescrizioneLong != "" {
		t.Fatalf("expected empty descrizione_long for NULL source, got %q", rows[0].DescrizioneLong)
	}
	if rows[0].Quantita == nil || *rows[0].Quantita != 2.5 {
		t.Fatalf("expected fractional quantita 2.5, got %#v", rows[0].Quantita)
	}
	if rows[0].StatoOrdine != "" {
		t.Fatalf("expected empty stato_ordine for NULL source, got %q", rows[0].StatoOrdine)
	}
	if rows[0].NomeTestataOrdine != "" {
		t.Fatalf("expected empty nome_testata_ordine for NULL source, got %q", rows[0].NomeTestataOrdine)
	}
	if rows[0].StatoRiga != "" {
		t.Fatalf("expected empty stato_riga for NULL source, got %q", rows[0].StatoRiga)
	}
	if rows[0].Storico == nil || *rows[0].Storico != "history/path" {
		t.Fatalf("unexpected row payload: %#v", rows[0])
	}
}

func openPanoramicaTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()

	registerPanoramicaTestDriver()

	db, err := sql.Open(panoramicaTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const panoramicaTestDriverName = "panoramica_test_driver"

var registerPanoramicaDriverOnce sync.Once

func registerPanoramicaTestDriver() {
	registerPanoramicaDriverOnce.Do(func() {
		sql.Register(panoramicaTestDriverName, panoramicaTestDriver{})
	})
}

type panoramicaTestDriver struct{}

func (panoramicaTestDriver) Open(name string) (driver.Conn, error) {
	return &panoramicaTestConn{mode: name}, nil
}

type panoramicaTestConn struct {
	mode string
}

func (c *panoramicaTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *panoramicaTestConn) Close() error { return nil }

func (c *panoramicaTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *panoramicaTestConn) Ping(context.Context) error { return nil }

func (c *panoramicaTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.mode == "summary-null-text-fields" && strings.Contains(query, "FROM loader.v_ordini_sintesi") {
		return &panoramicaTestRows{
			columns: []string{
				"stato", "numero_ordine", "descrizione_long", "quantita", "nrc", "mrc", "totale_mrc",
				"stato_ordine", "nome_testata_ordine", "rn", "numero_azienda", "data_documento",
				"stato_riga", "data_ultima_fatt", "serialnumber", "metodo_pagamento", "durata_servizio",
				"durata_rinnovo", "data_cessazione", "data_attivazione", "note_legali", "sost_ord",
				"sostituito_da", "storico",
			},
			values: [][]driver.Value{{
				nil,
				nil,
				nil,
				float64(2.5),
				float64(10),
				float64(20),
				float64(30),
				nil,
				nil,
				int64(1),
				int64(123),
				"2026-04-09",
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				"history/path",
			}},
		}, nil
	}

	return &panoramicaTestRows{columns: []string{"stub"}}, nil
}

var _ driver.QueryerContext = (*panoramicaTestConn)(nil)
var _ driver.Pinger = (*panoramicaTestConn)(nil)

type panoramicaTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *panoramicaTestRows) Columns() []string {
	return r.columns
}

func (r *panoramicaTestRows) Close() error { return nil }

func (r *panoramicaTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
