package aenad

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
	RegisterRoutes(mux, Deps{})

	t.Run("missing claims", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/aenad/v1/documents?tipoDoc=Q", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("missing role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/aenad/v1/documents?tipoDoc=Q", nil)
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

	t.Run("valid role without db", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/aenad/v1/documents?tipoDoc=Q", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
			Name:  "Aenad User",
			Email: "aenad@example.com",
			Roles: []string{"app_aenad_access"},
		}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "aenad_database_not_configured") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
}

func TestParseDocumentListFiltersDefaultsAndLimits(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/aenad/v1/documents", nil)
	filters, err := parseDocumentListFilters(req, time.Date(2026, 6, 7, 12, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected default filters to parse, got %v", err)
	}
	if filters.TipoDoc != "Q" {
		t.Fatalf("expected default TipoDoc Q, got %q", filters.TipoDoc)
	}
	if filters.DateFrom != "2026-02-28" || filters.DateTo != "2026-06-07" {
		t.Fatalf("unexpected default dates: %s - %s", filters.DateFrom, filters.DateTo)
	}
	if filters.Page != 1 || filters.PageSize != 50 || filters.Offset != 0 {
		t.Fatalf("unexpected default pagination: %#v", filters)
	}

	req = httptest.NewRequest(http.MethodGet, "/aenad/v1/documents?tipoDoc=Q&dateFrom=2025-01-01&dateTo=2026-02-15", nil)
	if _, err := parseDocumentListFilters(req, time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)); !errors.Is(err, errInvalidDateRange) {
		t.Fatalf("expected max range validation, got %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/aenad/v1/documents?tipoDoc=Q&dateFrom=2026-03-01&dateTo=2026-01-01", nil)
	if _, err := parseDocumentListFilters(req, time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)); !errors.Is(err, errInvalidDateRange) {
		t.Fatalf("expected inverted range validation, got %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/aenad/v1/documents?tipoDoc=Q&page=0", nil)
	if _, err := parseDocumentListFilters(req, time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)); !errors.Is(err, errInvalidPage) {
		t.Fatalf("expected page validation, got %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/aenad/v1/documents?tipoDoc=Q&pageSize=101", nil)
	if _, err := parseDocumentListFilters(req, time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)); !errors.Is(err, errInvalidPageSize) {
		t.Fatalf("expected page size validation, got %v", err)
	}
}

func TestHandleDocumentTypesUsesVisibleOnly(t *testing.T) {
	h := &Handler{mistra: openAenadTestDB(t, "document-types")}

	req := httptest.NewRequest(http.MethodGet, "/aenad/v1/document-types", nil)
	rec := httptest.NewRecorder()
	h.handleDocumentTypes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body []documentTypeOption
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("expected two document types, got %d", len(body))
	}
	if body[0].TipoDoc != "Q" || body[0].Label != "Preventivi" {
		t.Fatalf("unexpected first option: %#v", body[0])
	}
}

func TestHandleDocumentsQueriesFilteredPageAndMapsRows(t *testing.T) {
	h := &Handler{mistra: openAenadTestDB(t, "documents")}

	req := httptest.NewRequest(
		http.MethodGet,
		"/aenad/v1/documents?tipoDoc=Q&dateFrom=2026-05-01&dateTo=2026-05-31&page=2&pageSize=2",
		nil,
	)
	rec := httptest.NewRecorder()
	h.handleDocuments(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body documentListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Total != 5 || body.Page != 2 || body.PageSize != 2 {
		t.Fatalf("unexpected page metadata: %#v", body)
	}
	if len(body.Items) != 1 {
		t.Fatalf("expected one row, got %d", len(body.Items))
	}
	row := body.Items[0]
	if row.IDDoc != 215584 || row.TipoDoc == nil || *row.TipoDoc != "Q" {
		t.Fatalf("unexpected document identity: %#v", row)
	}
	if row.Data == nil || *row.Data != "2026-05-26" || row.DataDoc == nil || *row.DataDoc != "2026-05-26" {
		t.Fatalf("unexpected document dates: %#v %#v", row.Data, row.DataDoc)
	}
	if row.TotDoc == nil || *row.TotDoc != 2017 {
		t.Fatalf("expected TotDoc 2017, got %#v", row.TotDoc)
	}
}

func openAenadTestDB(t *testing.T, mode string) *sql.DB {
	t.Helper()
	registerAenadTestDriver()

	db, err := sql.Open(aenadTestDriverName, mode)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const aenadTestDriverName = "aenad_test_driver"

var registerAenadTestDriverOnce sync.Once

func registerAenadTestDriver() {
	registerAenadTestDriverOnce.Do(func() {
		sql.Register(aenadTestDriverName, aenadTestDriver{})
	})
}

type aenadTestDriver struct{}

func (aenadTestDriver) Open(name string) (driver.Conn, error) {
	return &aenadTestConn{mode: name}, nil
}

type aenadTestConn struct {
	mode string
}

func (c *aenadTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *aenadTestConn) Close() error { return nil }

func (c *aenadTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *aenadTestConn) Ping(context.Context) error { return nil }

func (c *aenadTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch c.mode {
	case "document-types":
		if !strings.Contains(query, `FROM aenad."TTipiDoc"`) {
			return nil, errors.New("document types query should use TTipiDoc")
		}
		if !strings.Contains(query, `COALESCE("Visibile", 0) = 1`) {
			return nil, errors.New("document types query should filter visible options")
		}
		if !strings.Contains(query, `ORDER BY COALESCE("Ordinam", $1), label, "TipoDoc"`) {
			return nil, errors.New("document types query should use deterministic ordering")
		}
		if len(args) != 1 || args[0].Value != int64(defaultTipoDocSortRank) {
			return nil, errors.New("unexpected document types args")
		}
		return &aenadTestRows{
			columns: []string{"TipoDoc", "label"},
			values: [][]driver.Value{
				{"Q", "Preventivi"},
				{"O", "Ordini"},
			},
		}, nil
	case "documents":
		if !strings.Contains(query, `FROM aenad."TDocTestate" d`) {
			return nil, errors.New("documents query should use TDocTestate")
		}
		if !strings.Contains(query, `d."TipoDoc" = $1`) || !strings.Contains(query, `d."Data" >= $2::date`) || !strings.Contains(query, `d."Data" <= $3::date`) {
			return nil, errors.New("documents query should require TipoDoc and date range")
		}
		if strings.Contains(query, "SELECT COUNT(*)") {
			if len(args) != 3 {
				return nil, errors.New("unexpected count args length")
			}
			if args[0].Value != "Q" || args[1].Value != "2026-05-01" || args[2].Value != "2026-05-31" {
				return nil, errors.New("unexpected count args")
			}
			return &aenadTestRows{
				columns: []string{"count"},
				values:  [][]driver.Value{{int64(5)}},
			}, nil
		}
		if !strings.Contains(query, `ORDER BY d."Data" DESC, d."IDDoc" DESC`) || !strings.Contains(query, "LIMIT $4 OFFSET $5") {
			return nil, errors.New("documents query should order and paginate")
		}
		if len(args) != 5 {
			return nil, errors.New("unexpected page args length")
		}
		if args[0].Value != "Q" || args[1].Value != "2026-05-01" || args[2].Value != "2026-05-31" || args[3].Value != int64(2) || args[4].Value != int64(2) {
			return nil, errors.New("unexpected page args")
		}
		return &aenadTestRows{
			columns: documentTestColumns(),
			values: [][]driver.Value{
				{
					int64(215584),
					"Q",
					int64(12174),
					"Test Customer Name",
					int64(12174),
					"Via Galvani",
					testDate(2026, 5, 26),
					int64(1013),
					testDate(2026, 5, 26),
					"1013",
					"Prev. 1013 del 26/5/26",
					int64(1653),
					int64(2017),
					int64(1521),
					int64(132),
					"Contanti",             // Pagamento
					"IT1234567890",         // Pagam_CoordBancarie
					"Nota interna di test", // NoteInterne
					"Via Milano 12",        // Anagr_Indirizzo
					"20100",                // Anagr_Cap
					"Milano",               // Anagr_Citta
					"MI",                   // Anagr_Prov
					"Italia",               // Anagr_Nazione
					"CF1234567890",         // Anagr_CodiceFiscale
					"PI1234567890",         // Anagr_PartitaIva
					"Destinatario Test",    // Anagr_DestNome
					"Via Torino 5",         // Anagr_DestIndirizzo
					"10100",                // Anagr_DestCap
					"Torino",               // Anagr_DestCitta
					"TO",                   // Anagr_DestProv
					"Italia",               // Anagr_DestNazione
				},
			},
		}, nil
	}

	return nil, errors.New("unexpected query for mode: " + c.mode)
}

type aenadTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *aenadTestRows) Columns() []string { return r.columns }

func (r *aenadTestRows) Close() error { return nil }

func (r *aenadTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func documentTestColumns() []string {
	return []string{
		"IDDoc",
		"TipoDoc",
		"IDAnagr",
		"Anagr_Nome",
		"CodDest_IDAnagr",
		"CodDest",
		"Data",
		"Num",
		"DataDoc",
		"NumDoc",
		"DescDoc",
		"TotNetto",
		"TotDoc",
		"TotPrezzoAcquisto",
		"TotGuadagno",
		"Pagamento",
		"Pagam_CoordBancarie",
		"NoteInterne",
		"Anagr_Indirizzo",
		"Anagr_Cap",
		"Anagr_Citta",
		"Anagr_Prov",
		"Anagr_Nazione",
		"Anagr_CodiceFiscale",
		"Anagr_PartitaIva",
		"Anagr_DestNome",
		"Anagr_DestIndirizzo",
		"Anagr_DestCap",
		"Anagr_DestCitta",
		"Anagr_DestProv",
		"Anagr_DestNazione",
	}
}

func testDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
