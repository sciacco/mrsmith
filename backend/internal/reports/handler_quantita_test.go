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
		Stato    *string  `json:"stato"`
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
	if rows[0].Stato == nil || *rows[0].Stato != "ATTIVA" {
		t.Fatalf("expected preview stato to stay available, got %#v", rows[0].Stato)
	}
}

func TestActiveLinesExportRowsRenamesStatoForCarbone(t *testing.T) {
	state := "ATTIVA"
	quantita := 1.25

	exportRows, err := activeLinesExportRows([]activeLineRow{
		{
			RagioneSociale: "ACME",
			Stato:          &state,
			Quantita:       &quantita,
			Canone:         49.9,
		},
		{
			RagioneSociale: "BETA",
			Stato:          nil,
		},
	})
	if err != nil {
		t.Fatalf("expected export payload conversion to succeed, got %v", err)
	}
	if len(exportRows) != 2 {
		t.Fatalf("expected 2 export rows, got %d", len(exportRows))
	}

	first := exportRows[0]
	if _, ok := first["stato"]; ok {
		t.Fatalf("expected export payload to omit stato, got %#v", first["stato"])
	}
	if got, ok := first["stato grappa"].(string); !ok || got != "ATTIVA" {
		t.Fatalf("expected stato grappa alias, got %#v", first["stato grappa"])
	}
	if got, ok := first["stato_grappa"].(string); !ok || got != "ATTIVA" {
		t.Fatalf("expected stato_grappa alias, got %#v", first["stato_grappa"])
	}
	if got, ok := first["ragione_sociale"].(string); !ok || got != "ACME" {
		t.Fatalf("expected ragione_sociale to be preserved, got %#v", first["ragione_sociale"])
	}
	if got, ok := first["quantita"].(float64); !ok || got != 1.25 {
		t.Fatalf("expected quantita to be preserved, got %#v", first["quantita"])
	}
	if got, ok := first["canone"].(float64); !ok || got != 49.9 {
		t.Fatalf("expected canone to be preserved, got %#v", first["canone"])
	}

	second := exportRows[1]
	if _, ok := second["stato"]; ok {
		t.Fatalf("expected nil-state export payload to omit stato, got %#v", second["stato"])
	}
	if value, ok := second["stato grappa"]; !ok || value != nil {
		t.Fatalf("expected nil stato grappa alias, got %#v", second["stato grappa"])
	}
	if value, ok := second["stato_grappa"]; !ok || value != nil {
		t.Fatalf("expected nil stato_grappa alias, got %#v", second["stato_grappa"])
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

func TestHandleAovPreviewIncludesOrderCountsAndDetailFields(t *testing.T) {
	h := &Handler{mistraDB: openReportsTestDB(t, "aov-preview")}

	req := httptest.NewRequest(http.MethodPost, "/reports/v1/aov/preview",
		strings.NewReader(`{"dateFrom":"2026-01-01","dateTo":"2026-12-31","statuses":["Evaso"]}`))
	rec := httptest.NewRecorder()

	h.handleAovPreview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp aovPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.ByType) != 1 || len(resp.ByCategory) != 1 || len(resp.BySales) != 1 || len(resp.Detail) != 1 {
		t.Fatalf("unexpected response shape: byType=%d byCategory=%d bySales=%d detail=%d",
			len(resp.ByType), len(resp.ByCategory), len(resp.BySales), len(resp.Detail))
	}

	if resp.ByType[0].NumeroOrdini != 3 {
		t.Fatalf("expected byType numero_ordini=3, got %d", resp.ByType[0].NumeroOrdini)
	}
	if resp.ByCategory[0].NumeroOrdini != 2 {
		t.Fatalf("expected byCategory numero_ordini=2, got %d", resp.ByCategory[0].NumeroOrdini)
	}
	if resp.BySales[0].NumeroOrdini != 4 {
		t.Fatalf("expected bySales numero_ordini=4, got %d", resp.BySales[0].NumeroOrdini)
	}

	detail := resp.Detail[0]
	if detail.TipoDocumento == nil || *detail.TipoDocumento != "TSC-ORDINE-RIC" {
		t.Fatalf("expected detail.tipo_documento to be populated, got %#v", detail.TipoDocumento)
	}
	if detail.Anno == nil || *detail.Anno != "2026" {
		t.Fatalf("expected detail.anno to be populated, got %#v", detail.Anno)
	}
	if detail.Mese == nil || *detail.Mese != "04" {
		t.Fatalf("expected detail.mese to be populated, got %#v", detail.Mese)
	}
	if detail.NomeTestataOrdine == nil || *detail.NomeTestataOrdine != "ORD-001" {
		t.Fatalf("expected detail.nome_testata_ordine to be populated, got %#v", detail.NomeTestataOrdine)
	}
	if detail.TotaleMRCNew == nil || *detail.TotaleMRCNew != 120 {
		t.Fatalf("expected detail.totale_mrc_new to be populated, got %#v", detail.TotaleMRCNew)
	}
	if detail.ValoreAOV == nil || *detail.ValoreAOV != 1445 {
		t.Fatalf("expected detail.valore_aov to be populated, got %#v", detail.ValoreAOV)
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
	case "aov-preview":
		qLower := strings.ToLower(query)
		switch {
		case strings.Contains(query, "GROUP BY anno, mese, tipo_ordine"):
			if !strings.Contains(qLower, "order by\nanno asc,\nmese asc,") || !strings.Contains(qLower, "case tipo_ordine") {
				return nil, errors.New("missing final ORDER BY in byType query")
			}
			return &reportsTestRows{
				columns: []string{"anno", "mese", "tipo_ordine", "numero_ordini", "totale_mrc", "totale_nrc", "valore_aov"},
				values: [][]driver.Value{
					{"2026", "04", "NUOVO", int64(3), float64(1000), float64(200), float64(12200)},
				},
			}, nil
		case strings.Contains(query, "GROUP BY anno, mese, categoria"):
			if !strings.Contains(qLower, "order by anno asc, mese asc, categoria asc") {
				return nil, errors.New("missing final ORDER BY in byCategory query")
			}
			return &reportsTestRows{
				columns: []string{"anno", "mese", "categoria", "numero_ordini", "totale_mrc", "totale_nrc", "valore_aov"},
				values: [][]driver.Value{
					{"2026", "04", "Connectivity", int64(2), float64(800), float64(120), float64(9720)},
				},
			}, nil
		case strings.Contains(query, "GROUP BY anno, commerciale, tipo_ordine"):
			if !strings.Contains(qLower, "order by\nanno asc,\ncommerciale asc,") || !strings.Contains(qLower, "case tipo_ordine") {
				return nil, errors.New("missing final ORDER BY in bySales query")
			}
			return &reportsTestRows{
				columns: []string{"anno", "commerciale", "tipo_ordine", "numero_ordini", "totale_mrc", "totale_nrc", "valore_aov"},
				values: [][]driver.Value{
					{"2026", "Mario Rossi", "NUOVO", int64(4), float64(1300), float64(260), float64(15860)},
				},
			}, nil
		case strings.Contains(query, "SELECT\no.tipo_documento,"):
			if !strings.Contains(qLower, "order by\nanno asc,\nmese asc,\ncommerciale asc,") || !strings.Contains(qLower, "o.nome_testata_ordine asc") {
				return nil, errors.New("missing final ORDER BY in detail query")
			}
			return &reportsTestRows{
				columns: []string{
					"tipo_documento", "anno", "mese", "nome_testata_ordine", "tipo_ordine",
					"sost_ord", "commerciale", "totale_mrc", "totale_nrc", "totale_mrc_odv_sost",
					"totale_mrc_new", "valore_aov",
				},
				values: [][]driver.Value{
					{
						"TSC-ORDINE-RIC", "2026", "04", "ORD-001", "NUOVO",
						"SOST-000", "Mario Rossi", float64(120), float64(5), float64(0),
						float64(120), float64(1445),
					},
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
