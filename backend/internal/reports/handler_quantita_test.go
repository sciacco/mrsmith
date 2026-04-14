package reports

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

func TestHandleOrdersPreviewSupportsFractionalAndNullQuantita(t *testing.T) {
	h := &Handler{mistraDB: openReportsTestDB(t, "orders-preview")}

	req := httptest.NewRequest(http.MethodPost, "/reports/v1/orders/preview",
		strings.NewReader(`{"dateFrom":"2026-01-01","dateTo":"2026-12-31","statuses":["Evaso"]}`))
	rec := httptest.NewRecorder()

	h.handleOrdersPreview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var rows []struct {
		Quantita *float64 `json:"quantita"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Quantita == nil || *rows[0].Quantita != 7.5 {
		t.Fatalf("expected first quantita to be 7.5, got %#v", rows[0].Quantita)
	}
	if rows[1].Quantita != nil {
		t.Fatalf("expected second quantita to be null, got %#v", rows[1].Quantita)
	}
}

func TestHandleActiveLinesPreviewSupportsFractionalAndNullQuantita(t *testing.T) {
	h := &Handler{mistraDB: openReportsTestDB(t, "active-lines-preview")}

	req := httptest.NewRequest(http.MethodPost, "/reports/v1/active-lines/preview",
		strings.NewReader(`{"connectionTypes":["FTTH"],"statuses":["ATTIVA"]}`))
	rec := httptest.NewRecorder()

	h.handleActiveLinesPreview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var rows []struct {
		Quantita *float64 `json:"quantita"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Quantita == nil || *rows[0].Quantita != 1.25 {
		t.Fatalf("expected first quantita to be 1.25, got %#v", rows[0].Quantita)
	}
	if rows[1].Quantita != nil {
		t.Fatalf("expected second quantita to be null, got %#v", rows[1].Quantita)
	}
}

func TestHandlePendingActivationRowsSupportsFractionalAndNullQuantita(t *testing.T) {
	h := &Handler{mistraDB: openReportsTestDB(t, "pending-activation-rows")}

	req := httptest.NewRequest(http.MethodGet, "/reports/v1/pending-activations/ORD-1/rows", nil)
	req.SetPathValue("orderNumber", "ORD-1")
	rec := httptest.NewRecorder()

	h.handlePendingActivationRows(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var rows []struct {
		Quantita *float64 `json:"quantita"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Quantita == nil || *rows[0].Quantita != 0.5 {
		t.Fatalf("expected first quantita to be 0.5, got %#v", rows[0].Quantita)
	}
	if rows[1].Quantita != nil {
		t.Fatalf("expected second quantita to be null, got %#v", rows[1].Quantita)
	}
}

func TestHandleUpcomingRenewalRowsSupportsFractionalAndNullQuantita(t *testing.T) {
	h := &Handler{mistraDB: openReportsTestDB(t, "upcoming-renewal-rows")}

	req := httptest.NewRequest(http.MethodGet, "/reports/v1/upcoming-renewals/123/rows?months=4&minMrc=11", nil)
	req.SetPathValue("customerId", "123")
	rec := httptest.NewRecorder()

	h.handleUpcomingRenewalRows(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var rows []struct {
		Quantita *float64 `json:"quantita"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Quantita == nil || *rows[0].Quantita != 3.5 {
		t.Fatalf("expected first quantita to be 3.5, got %#v", rows[0].Quantita)
	}
	if rows[1].Quantita != nil {
		t.Fatalf("expected second quantita to be null, got %#v", rows[1].Quantita)
	}
}

func openReportsTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()
	registerReportsTestDriver()

	db, err := sql.Open(reportsTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const reportsTestDriverName = "reports_test_driver"

var registerReportsDriverOnce sync.Once

func registerReportsTestDriver() {
	registerReportsDriverOnce.Do(func() {
		sql.Register(reportsTestDriverName, reportsTestDriver{})
	})
}

type reportsTestDriver struct{}

func (reportsTestDriver) Open(name string) (driver.Conn, error) {
	return &reportsTestConn{mode: name}, nil
}

type reportsTestConn struct {
	mode string
}

func (c *reportsTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *reportsTestConn) Close() error { return nil }

func (c *reportsTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *reportsTestConn) Ping(context.Context) error { return nil }

func (c *reportsTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch c.mode {
	case "orders-preview":
		if strings.Contains(query, "FROM loader.v_ordini_ric_spot AS o") {
			return &reportsTestRows{
				columns: []string{
					"ragione_sociale", "stato_ordine", "numero_ordine", "descrizione_long",
					"quantita", "nrc", "mrc", "totale_mrc", "numero_azienda",
					"data_documento", "stato_riga", "data_ultima_fatt", "serialnumber",
					"metodo_pagamento", "durata_servizio", "durata_rinnovo",
					"data_cessazione", "data_attivazione", "note_legali", "sost_ord",
					"sostituito_da", "progressivo_riga",
				},
				values: [][]driver.Value{
					{
						"ACME", "Evaso", "ORD-1", "Linea 1",
						float64(7.5), float64(10), float64(20), float64(150), int64(123),
						"2026-04-10", "Attiva", "2026-04-11", "SN-1",
						"RID", "12", "12", "2027-01-01", "2026-01-01", "N1", "S1", "S2", int64(1),
					},
					{
						"ACME", "Evaso", "ORD-2", "Linea 2",
						nil, float64(11), float64(21), float64(0), int64(123),
						"2026-04-10", "Attiva", "2026-04-11", "SN-2",
						"RID", "12", "12", "2027-01-01", "2026-01-01", "N2", "S3", "S4", int64(2),
					},
				},
			}, nil
		}
	case "active-lines-preview":
		if strings.Contains(query, "loader.grappa_foglio_linee fl") {
			return &reportsTestRows{
				columns: []string{
					"ragione_sociale", "tipo_conn", "fornitore", "provincia", "comune",
					"tipo", "profilo_commerciale", "macro", "intestatario", "ordine",
					"fatturato_fino_al", "stato_riga", "stato_ordine", "stato",
					"id", "codice_ordine", "serialnumber", "id_anagrafica", "quantita", "canone",
				},
				values: [][]driver.Value{
					{
						"ACME", "FTTH", "Fornitore", "MI", "Milano",
						"Internet", "Business", "DEDICATA", "Mario Rossi", "ORD-1",
						"2026-04-10", "Attiva", "Evaso", "ATTIVA",
						int64(1), "ORD-1", "SN-1", "123", float64(1.25), float64(49.9),
					},
					{
						"ACME", "FTTH", "Fornitore", "MI", "Milano",
						"Internet", "Business", "DEDICATA", "Mario Rossi", "ORD-2",
						"2026-04-10", "Attiva", "Evaso", "ATTIVA",
						int64(2), "ORD-2", "SN-2", "123", nil, float64(49.9),
					},
				},
			}, nil
		}
	case "pending-activation-rows":
		if strings.Contains(query, "where os.nome_testata_ordine = $1") {
			return &reportsTestRows{
				columns: []string{
					"descrizione_long", "quantita", "nrc", "mrc", "totale_mrc",
					"stato_riga", "serialnumber", "note_legali",
				},
				values: [][]driver.Value{
					{"Riga 1", float64(0.5), float64(5), float64(10), float64(5), "Da attivare", "SN-1", "note"},
					{"Riga 2", nil, float64(5), float64(10), float64(0), "Da attivare", "SN-2", "note"},
				},
			}, nil
		}
	case "upcoming-renewal-rows":
		if strings.Contains(query, "from loader.v_ordini_ricorrenti_conrinnovo") && strings.Contains(query, "and numero_azienda = $3") {
			return &reportsTestRows{
				columns: []string{
					"nome_testata_ordine", "stato_ordine", "descrizione_long", "quantita",
					"nrc", "mrc", "stato_riga", "serialnumber", "note_legali",
					"data_attivazione", "durata_servizio", "durata_rinnovo", "durata",
					"prossimo_rinnovo", "sost_ord", "sostituito_da", "tacito_rinnovo",
				},
				values: [][]driver.Value{
					{"ORD-1", "Evaso", "Rinnovo 1", float64(3.5), float64(7), float64(11), "Attiva", "SN-1", "note", "2026-01-01", "12", "12", "12 / 12", "2026-12-01", "SO1", "SD1", int64(1)},
					{"ORD-2", "Evaso", "Rinnovo 2", nil, float64(7), float64(11), "Attiva", "SN-2", "note", "2026-01-01", "12", "12", "12 / 12", "2026-12-01", "SO2", "SD2", int64(1)},
				},
			}, nil
		}
	}

	return nil, errors.New("unexpected query for mode: " + c.mode)
}

var _ driver.QueryerContext = (*reportsTestConn)(nil)
var _ driver.Pinger = (*reportsTestConn)(nil)

type reportsTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *reportsTestRows) Columns() []string { return r.columns }

func (r *reportsTestRows) Close() error { return nil }

func (r *reportsTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
