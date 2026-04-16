package rdf

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHandleListRichiesteSummaryAllowsNullPreferredSuppliers(t *testing.T) {
	h := &Handler{
		anisettaDB: openRDFTestDB(t, "summary-null-preferred-suppliers"),
		mistraDB:   openRDFTestDB(t, "unexpected-query"),
	}

	req := httptest.NewRequest(http.MethodGet, "/rdf/v1/richieste/summary", nil)
	rec := httptest.NewRecorder()

	h.handleListRichiesteSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"fornitori_preferiti":[]`) {
		t.Fatalf("expected empty preferred suppliers array in response, got %s", rec.Body.String())
	}

	var resp pagedResponse[RichiestaSummary]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("unexpected pagination payload: %#v", resp)
	}
	if resp.Items[0].FornitoriPreferiti == nil {
		t.Fatalf("expected preferred suppliers slice to be empty, got nil")
	}
	if len(resp.Items[0].FornitoriPreferiti) != 0 {
		t.Fatalf("expected no preferred suppliers, got %#v", resp.Items[0].FornitoriPreferiti)
	}
}

func TestHandleGetRichiestaFullAllowsNullPreferredSuppliers(t *testing.T) {
	h := &Handler{
		anisettaDB: openRDFTestDB(t, "full-null-preferred-suppliers"),
		mistraDB:   openRDFTestDB(t, "unexpected-query"),
	}

	req := httptest.NewRequest(http.MethodGet, "/rdf/v1/richieste/42/full", nil)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	h.handleGetRichiestaFull(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"fornitori_preferiti":[]`) {
		t.Fatalf("expected empty preferred suppliers array in response, got %s", rec.Body.String())
	}

	var resp RichiestaFull
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.FornitoriPreferiti == nil {
		t.Fatalf("expected preferred suppliers slice to be empty, got nil")
	}
	if len(resp.FornitoriPreferiti) != 0 {
		t.Fatalf("expected no preferred suppliers, got %#v", resp.FornitoriPreferiti)
	}
	if len(resp.Fattibilita) != 0 {
		t.Fatalf("expected no fattibilita rows, got %#v", resp.Fattibilita)
	}
}

func openRDFTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()

	registerRDFTestDriver()

	db, err := sql.Open(rdfTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const rdfTestDriverName = "rdf_test_driver"

var registerRDFTestDriverOnce sync.Once

func registerRDFTestDriver() {
	registerRDFTestDriverOnce.Do(func() {
		sql.Register(rdfTestDriverName, rdfTestDriver{})
	})
}

type rdfTestDriver struct{}

func (rdfTestDriver) Open(name string) (driver.Conn, error) {
	return &rdfTestConn{mode: name}, nil
}

type rdfTestConn struct {
	mode string
}

func (c *rdfTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *rdfTestConn) Close() error { return nil }

func (c *rdfTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *rdfTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch c.mode {
	case "summary-null-preferred-suppliers":
		if strings.Contains(query, "FROM public.rdf_richieste r") && strings.Contains(query, "LEFT JOIN counts c") {
			return &rdfTestRows{
				columns: []string{
					"id", "deal_id", "data_richiesta", "descrizione", "indirizzo", "stato",
					"annotazioni_richiedente", "annotazioni_carrier", "created_by", "created_at", "updated_at",
					"fornitori_preferiti", "codice_deal", "bozza", "inviata", "sollecitata", "completata", "annullata", "totale",
				},
				values: [][]driver.Value{{
					int64(42),
					nil,
					time.Date(2026, time.April, 16, 0, 0, 0, 0, time.UTC),
					"Nuova richiesta",
					"Via Roma 1",
					"nuova",
					nil,
					nil,
					"tester@example.com",
					time.Date(2026, time.April, 16, 10, 21, 0, 0, time.UTC),
					nil,
					nil,
					"DL-42",
					int64(0),
					int64(0),
					int64(0),
					int64(0),
					int64(0),
					int64(0),
				}},
			}, nil
		}
	case "full-null-preferred-suppliers":
		if strings.Contains(query, "FROM public.rdf_richieste") && strings.Contains(query, "WHERE id = $1") {
			return &rdfTestRows{
				columns: []string{
					"id", "deal_id", "data_richiesta", "descrizione", "indirizzo", "stato",
					"annotazioni_richiedente", "annotazioni_carrier", "created_by", "created_at", "updated_at",
					"fornitori_preferiti", "codice_deal",
				},
				values: [][]driver.Value{{
					int64(42),
					nil,
					time.Date(2026, time.April, 16, 0, 0, 0, 0, time.UTC),
					"Nuova richiesta",
					"Via Roma 1",
					"nuova",
					nil,
					nil,
					"tester@example.com",
					time.Date(2026, time.April, 16, 10, 21, 0, 0, time.UTC),
					nil,
					nil,
					"DL-42",
				}},
			}, nil
		}
		if strings.Contains(query, "FROM public.rdf_fattibilita_fornitori ff") && strings.Contains(query, "WHERE ff.richiesta_id = $1") {
			return &rdfTestRows{
				columns: []string{
					"id", "richiesta_id", "fornitore_id", "nome", "data_richiesta", "tecnologia_id", "tecnologia_nome",
					"descrizione", "contatto_fornitore", "riferimento_fornitore", "stato", "annotazioni",
					"esito_ricevuto_il", "da_ordinare", "profilo_fornitore", "nrc", "mrc", "durata_mesi",
					"aderenza_budget", "copertura", "giorni_rilascio",
				},
			}, nil
		}
	case "unexpected-query":
		return nil, fmt.Errorf("unexpected query: %s", query)
	}

	return nil, fmt.Errorf("unexpected query in mode %q: %s", c.mode, query)
}

var _ driver.QueryerContext = (*rdfTestConn)(nil)

type rdfTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *rdfTestRows) Columns() []string {
	return r.columns
}

func (r *rdfTestRows) Close() error { return nil }

func (r *rdfTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
