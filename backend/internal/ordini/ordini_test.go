package ordini

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/hubspot"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

func TestNullDateScan(t *testing.T) {
	tests := []struct {
		name  string
		value any
		valid bool
		want  string
	}{
		{name: "empty", value: "", valid: false},
		{name: "zero date", value: "0000-00-00", valid: false},
		{name: "zero datetime", value: "0000-00-00 00:00:00", valid: false},
		{name: "date", value: "2026-05-23", valid: true, want: "2026-05-23"},
		{name: "rfc3339", value: "2026-05-23T10:20:30Z", valid: true, want: "2026-05-23"},
		{name: "legacy datetime", value: "2026-05-23 10:20:30", valid: true, want: "2026-05-23"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got NullDate
			if err := got.Scan(tc.value); err != nil {
				t.Fatalf("Scan() error = %v", err)
			}
			if got.Valid != tc.valid {
				t.Fatalf("Valid = %v, want %v", got.Valid, tc.valid)
			}
			if got.String() != tc.want {
				t.Fatalf("String() = %q, want %q", got.String(), tc.want)
			}
		})
	}
	var invalid NullDate
	if err := invalid.Scan("23/05/2026"); err == nil {
		t.Fatal("expected invalid locale date to fail")
	}
}

func TestDBFailureReturnsPlannedCodeAndLogsCorrelation(t *testing.T) {
	var log bytes.Buffer
	h := &Handler{logger: logging.NewWithWriter(&log, "debug")}
	req := requestWithRoles(http.MethodGet, "/ordini/v1/orders", nil, false)
	req = req.WithContext(logging.WithRequestID(req.Context(), "req-db-failure"))
	rec := httptest.NewRecorder()

	h.dbFailure(rec, req, "list_orders", errors.New("db down"), "order_id", int64(7))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if errorCode(t, rec.Body.Bytes()) != "db_failed" {
		t.Fatalf("body = %s", rec.Body.String())
	}
	entry := decodeLogEntry(t, log.String())
	if entry["component"] != component || entry["operation"] != "list_orders" || entry["request_id"] != "req-db-failure" {
		t.Fatalf("missing log correlation fields: %#v", entry)
	}
	if _, ok := entry["duration_ms"]; !ok {
		t.Fatalf("missing duration_ms in log: %#v", entry)
	}
}

func TestConfirmationDateValidation(t *testing.T) {
	if value, ok := confirmationDateOrNil(""); !ok || value != nil {
		t.Fatalf("blank date = (%#v, %v), want nil true", value, ok)
	}
	if value, ok := confirmationDateOrNil("2026-05-23"); !ok || value != "2026-05-23" {
		t.Fatalf("valid date = (%#v, %v), want normalized true", value, ok)
	}
	if _, ok := confirmationDateOrNil("2026-5-23"); ok {
		t.Fatal("non-normalized date was accepted")
	}
	if _, ok := confirmationDateOrNil("2026-05-23T00:00:00Z"); ok {
		t.Fatal("datetime was accepted")
	}
}

func TestPatchOrderHeaderDualWritesCustomerAndValidatesDate(t *testing.T) {
	state := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			switch {
			case strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?"):
				return nil, [][]driver.Value{orderDetailValues("BOZZA", "2026-05-22")}, nil
			case strings.Contains(query, "FROM Tsmi_Anagrafiche_clienti"):
				return []string{"NUMERO_AZIENDA", "RAGIONE_SOCIALE"}, [][]driver.Value{{int64(42), "ACME Spa"}}, nil
			default:
				return nil, nil, errors.New("unexpected query: " + query)
			}
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			if !strings.Contains(query, "cdlan_cliente_id = ?") || !strings.Contains(query, "cdlan_cliente = ?") {
				t.Fatalf("header update missing customer dual write: %s", query)
			}
			if got := args[1].Value; got != "2026-05-23" {
				t.Fatalf("confirmation arg = %#v, want 2026-05-23", got)
			}
			if got := args[2].Value; got != int64(42) {
				t.Fatalf("customer id arg = %#v, want 42", got)
			}
			if got := args[3].Value; got != "ACME Spa" {
				t.Fatalf("customer name arg = %#v, want ACME Spa", got)
			}
			return 1, nil
		},
	}
	db := openOrdiniTestDB(t, state)
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: db, Alyante: db, Logger: logging.NewWithWriter(io.Discard, "debug")})
	req := requestWithRoles(http.MethodPatch, "/ordini/v1/orders/1", strings.NewReader(`{"customer_po":"PO-1","confirmation_date":"2026-05-23","customer_id":42}`), true)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(state.execs) != 1 {
		t.Fatalf("exec count = %d, want 1", len(state.execs))
	}

	req = requestWithRoles(http.MethodPatch, "/ordini/v1/orders/1", strings.NewReader(`{"customer_po":"PO-1","confirmation_date":"2026-05-23T00:00:00Z","customer_id":42}`), true)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec.Body.Bytes()) != "invalid_confirmation_date" {
		t.Fatalf("invalid date status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestPatchOrderHeaderCustomerLookupFailureDoesNotWrite(t *testing.T) {
	state := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			switch {
			case strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?"):
				return nil, [][]driver.Value{orderDetailValues("BOZZA", "2026-05-22")}, nil
			case strings.Contains(query, "FROM Tsmi_Anagrafiche_clienti"):
				return nil, nil, sql.ErrNoRows
			default:
				return nil, nil, errors.New("unexpected query: " + query)
			}
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			t.Fatalf("unexpected header exec after customer miss: %s", query)
			return 0, nil
		},
	}
	db := openOrdiniTestDB(t, state)
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: db, Alyante: db, Logger: logging.NewWithWriter(io.Discard, "debug")})

	req := requestWithRoles(http.MethodPatch, "/ordini/v1/orders/1", strings.NewReader(`{"customer_po":"PO-1","confirmation_date":"2026-05-23","customer_id":42}`), true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.Bytes()) != "customer_not_found" {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
	if len(state.execs) != 0 {
		t.Fatalf("execs = %d, want 0", len(state.execs))
	}
}

func TestPatchOrderHeaderStateGuardStopsCustomerLookup(t *testing.T) {
	state := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			switch {
			case strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?"):
				return nil, [][]driver.Value{orderDetailValues("ATTIVO", "2026-05-22")}, nil
			case strings.Contains(query, "FROM Tsmi_Anagrafiche_clienti"):
				t.Fatal("customer lookup should not run after wrong order state")
			}
			return nil, nil, errors.New("unexpected query: " + query)
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			t.Fatalf("unexpected header exec in wrong state: %s", query)
			return 0, nil
		},
	}
	db := openOrdiniTestDB(t, state)
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: db, Alyante: db, Logger: logging.NewWithWriter(io.Discard, "debug")})

	req := requestWithRoles(http.MethodPatch, "/ordini/v1/orders/1", strings.NewReader(`{"customer_po":"PO-1","confirmation_date":"2026-05-23","customer_id":42}`), true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict || errorCode(t, rec.Body.Bytes()) != "wrong_state" {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
	if len(state.execs) != 0 {
		t.Fatalf("execs = %d, want 0", len(state.execs))
	}
}

func TestPatchReferentsUsesSQLStateGuard(t *testing.T) {
	state := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?") {
				return nil, [][]driver.Value{orderDetailValues("BOZZA", "2026-05-22")}, nil
			}
			return nil, nil, errors.New("unexpected query: " + query)
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			if !strings.Contains(query, "cdlan_stato IN ('BOZZA', 'INVIATO')") {
				t.Fatalf("referents update missing state guard: %s", query)
			}
			return 0, nil
		},
	}
	db := openOrdiniTestDB(t, state)
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: db, Logger: logging.NewWithWriter(io.Discard, "debug")})

	req := requestWithRoles(http.MethodPatch, "/ordini/v1/orders/1/referents", strings.NewReader(`{"technical_name":"","technical_phone":"","technical_email":"","other_technical_name":"","other_technical_phone":"","other_technical_email":"","admin_name":"","admin_phone":"","admin_email":""}`), true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict || errorCode(t, rec.Body.Bytes()) != "wrong_state" {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestSanitizeGatewayError(t *testing.T) {
	if got := sanitizeGatewayError(errGatewayPreconditionMissing); got != "precondition_missing" {
		t.Fatalf("sentinel = %q", got)
	}
	if got := sanitizeGatewayError(&gatewayHTTPError{Status: 422, Body: `{"error":"precondition_missing"}`}); got != "precondition_missing" {
		t.Fatalf("typed http error = %q", got)
	}
	if got := sanitizeGatewayError(errors.New("gateway returned precondition_missing in text")); got != "gateway_error" {
		t.Fatalf("plain text match should not be trusted, got %q", got)
	}
}

func TestGatewayFailureAttrsDoNotLeakRawUpstreamBodies(t *testing.T) {
	err := &gatewayHTTPError{Status: http.StatusBadGateway, Body: `{"error":"precondition_missing","detail":"dsn=password-secret"}`}
	if strings.Contains(err.Error(), "password-secret") || strings.Contains(err.Error(), "precondition_missing") {
		t.Fatalf("gatewayHTTPError leaked body text: %q", err.Error())
	}
	attrs := gatewayFailureAttrs("/orders/v1/erp", err, "order_id", int64(1))
	for _, attr := range attrs {
		if strings.Contains(toString(attr), "password-secret") {
			t.Fatalf("raw upstream body leaked in attrs: %#v", attrs)
		}
	}
	if !containsAttr(attrs, "upstream_status", http.StatusBadGateway) || !containsAttr(attrs, "upstream_code", "precondition_missing") {
		t.Fatalf("missing sanitized gateway attrs: %#v", attrs)
	}
}

func TestSendToERPPartialFailureDoesNotTransition(t *testing.T) {
	state := ordiniSendDBState(t, "BOZZA", [][]driver.Value{
		orderRowValues(11, 1, 101, 1, "", 0),
		orderRowValues(12, 1, 102, 1, "", 0),
	})
	gw := newOrdiniGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/orders/v1/erp" && strings.Contains(readBody(t, r), `"cdlan_systemodv_row":102`) {
			http.Error(w, `{"error":"precondition_missing"}`, http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: openOrdiniTestDB(t, state), Arak: gw.client, Logger: logging.NewWithWriter(io.Discard, "debug")})

	req := requestWithRoles(http.MethodPost, "/ordini/v1/orders/1/send-to-erp", multipartPDF(t), true)
	req.Header.Set("Content-Type", multipartContentType)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp SendToERPResponse
	decodeJSONBody(t, rec.Body.Bytes(), &resp)
	if resp.StateTransitioned || resp.ArxivarUploaded || len(resp.Rows) != 2 || resp.Rows[1].Error == nil || *resp.Rows[1].Error != "precondition_missing" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if state.execContains("SET cdlan_stato = 'INVIATO'") {
		t.Fatal("state transition exec happened after partial failure")
	}
}

func TestSendToERPFullSuccessTransitionsAndUploadsArxivar(t *testing.T) {
	state := ordiniSendDBState(t, "BOZZA", [][]driver.Value{orderRowValues(11, 1, 101, 1, "", 0)})
	var uploaded atomic.Bool
	gw := newOrdiniGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/orders/v1/send-to-arxivar" {
			uploaded.Store(true)
		}
		w.WriteHeader(http.StatusOK)
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: openOrdiniTestDB(t, state), Arak: gw.client, Logger: logging.NewWithWriter(io.Discard, "debug")})

	req := requestWithRoles(http.MethodPost, "/ordini/v1/orders/1/send-to-erp", multipartPDF(t), true)
	req.Header.Set("Content-Type", multipartContentType)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp SendToERPResponse
	decodeJSONBody(t, rec.Body.Bytes(), &resp)
	if rec.Code != http.StatusOK || !resp.StateTransitioned || !resp.ArxivarUploaded || !uploaded.Load() {
		t.Fatalf("status=%d response=%#v uploaded=%v", rec.Code, resp, uploaded.Load())
	}
	if !state.execContains("SET cdlan_stato = 'INVIATO', cdlan_evaso = 1") {
		t.Fatal("missing local INVIATO transition")
	}
}

func TestSendToERPArxivarFailureReturnsWarningAfterStateFlip(t *testing.T) {
	state := ordiniSendDBState(t, "BOZZA", [][]driver.Value{orderRowValues(11, 1, 101, 1, "", 0)})
	gw := newOrdiniGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/orders/v1/send-to-arxivar" {
			http.Error(w, "upload failed", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: openOrdiniTestDB(t, state), Arak: gw.client, Logger: logging.NewWithWriter(io.Discard, "debug")})

	req := requestWithRoles(http.MethodPost, "/ordini/v1/orders/1/send-to-erp", multipartPDF(t), true)
	req.Header.Set("Content-Type", multipartContentType)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp SendToERPResponse
	decodeJSONBody(t, rec.Body.Bytes(), &resp)
	if rec.Code != http.StatusOK || !resp.StateTransitioned || resp.ArxivarUploaded || resp.Warning != "arxivar_upload_failed" {
		t.Fatalf("status=%d response=%#v", rec.Code, resp)
	}
	if !state.execContains("SET cdlan_stato = 'INVIATO', cdlan_evaso = 1") {
		t.Fatal("state transition was not persisted before Arxivar warning")
	}
}

func TestRevertConversionBlocksNonQuoteOrigin(t *testing.T) {
	vodka := openOrdiniTestDB(t, &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?") {
				return nil, [][]driver.Value{orderDetailValues("BOZZA", "2026-05-22")}, nil
			}
			return nil, nil, errors.New("unexpected vodka query: " + query)
		},
	})
	mistra := openOrdiniTestDB(t, &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders.legacy_orders") {
				return nil, nil, sql.ErrNoRows
			}
			return nil, nil, errors.New("unexpected mistra query: " + query)
		},
	})
	alyante := openOrdiniTestDB(t, &ordiniTestDBState{})
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: vodka, Mistra: mistra, Alyante: alyante, Logger: logging.NewWithWriter(io.Discard, "debug")})

	req := requestWithRoles(http.MethodPost, "/ordini/v1/orders/1/revert-conversion", nil, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict || errorCode(t, rec.Body.Bytes()) != "not_converted_from_quote" {
		t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
	}
}

func TestRevertConversionBlocksWrongStateBeforeExternalChecks(t *testing.T) {
	vodkaState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?") {
				return nil, [][]driver.Value{orderDetailValues("INVIATO", "2026-05-22")}, nil
			}
			return nil, nil, errors.New("unexpected vodka query: " + query)
		},
	}
	mistraState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			t.Fatalf("mistra should not be queried after wrong state: %s", query)
			return nil, nil, nil
		},
	}
	alyanteState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			t.Fatalf("alyante should not be queried after wrong state: %s", query)
			return nil, nil, nil
		},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Vodka:   openOrdiniTestDB(t, vodkaState),
		Mistra:  openOrdiniTestDB(t, mistraState),
		Alyante: openOrdiniTestDB(t, alyanteState),
		Logger:  logging.NewWithWriter(io.Discard, "debug"),
	})

	req := requestWithRoles(http.MethodPost, "/ordini/v1/orders/1/revert-conversion", nil, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict || errorCode(t, rec.Body.Bytes()) != "wrong_state" {
		t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
	}
}

func TestRevertConversionBlocksWhenAlyanteRowsExist(t *testing.T) {
	vodkaState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?") {
				return nil, [][]driver.Value{orderDetailValues("BOZZA", "2026-05-22")}, nil
			}
			return nil, nil, errors.New("unexpected vodka query: " + query)
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			t.Fatalf("vodka delete should not run when Alyante rows exist: %s", query)
			return 0, nil
		},
	}
	mistraState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders.legacy_orders") {
				return []string{"quote_id", "jdata"}, [][]driver.Value{{int64(77), nil}}, nil
			}
			return nil, nil, errors.New("unexpected mistra query: " + query)
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			t.Fatalf("bridge delete should not run when Alyante rows exist: %s", query)
			return 0, nil
		},
	}
	alyanteState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if !strings.Contains(query, "FROM Tsmi_Ordini_Esteso") || !strings.Contains(query, "NUM_DOC_GAMMA") || !strings.Contains(query, "NUM_DOCUMENTO") {
				t.Fatalf("unexpected Alyante query: %s", query)
			}
			if args[0].Value != int64(12) || args[1].Value != int64(2026) || args[2].Value != "12" {
				t.Fatalf("unexpected Alyante args: %#v", args)
			}
			return []string{"count"}, [][]driver.Value{{int64(2)}}, nil
		},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Vodka:   openOrdiniTestDB(t, vodkaState),
		Mistra:  openOrdiniTestDB(t, mistraState),
		Alyante: openOrdiniTestDB(t, alyanteState),
		Logger:  logging.NewWithWriter(io.Discard, "debug"),
	})

	req := requestWithRoles(http.MethodPost, "/ordini/v1/orders/1/revert-conversion", nil, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict || errorCode(t, rec.Body.Bytes()) != "order_has_erp_rows" {
		t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
	}
	if len(vodkaState.execs) != 0 || len(mistraState.execs) != 0 {
		t.Fatalf("unexpected deletes vodka=%v mistra=%v", vodkaState.execs, mistraState.execs)
	}
}

func TestRevertConversionDeletesVodkaOrderAndMistraBridge(t *testing.T) {
	vodkaState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?") {
				return nil, [][]driver.Value{orderDetailValues("BOZZA", "2026-05-22")}, nil
			}
			return nil, nil, errors.New("unexpected vodka query: " + query)
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			switch {
			case strings.Contains(query, "DELETE FROM orders_rows"):
				if args[0].Value != int64(1) {
					t.Fatalf("row delete order arg = %#v", args[0].Value)
				}
				return 3, nil
			case strings.Contains(query, "DELETE FROM orders WHERE id = ? AND cdlan_stato = 'BOZZA'"):
				if args[0].Value != int64(1) {
					t.Fatalf("order delete arg = %#v", args[0].Value)
				}
				return 1, nil
			default:
				t.Fatalf("unexpected vodka exec: %s", query)
				return 0, nil
			}
		},
	}
	mistraState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders.legacy_orders") {
				return []string{"quote_id", "jdata"}, [][]driver.Value{{int64(77), nil}}, nil
			}
			return nil, nil, errors.New("unexpected mistra query: " + query)
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			if !strings.Contains(query, "DELETE FROM orders.legacy_orders") {
				t.Fatalf("unexpected mistra exec: %s", query)
			}
			if args[0].Value != int64(77) || args[1].Value != int64(1) {
				t.Fatalf("bridge delete args = %#v", args)
			}
			return 1, nil
		},
	}
	alyanteState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM Tsmi_Ordini_Esteso") {
				return []string{"count"}, [][]driver.Value{{int64(0)}}, nil
			}
			return nil, nil, errors.New("unexpected alyante query: " + query)
		},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Vodka:   openOrdiniTestDB(t, vodkaState),
		Mistra:  openOrdiniTestDB(t, mistraState),
		Alyante: openOrdiniTestDB(t, alyanteState),
		Logger:  logging.NewWithWriter(io.Discard, "debug"),
	})

	req := requestWithRoles(http.MethodPost, "/ordini/v1/orders/1/revert-conversion", nil, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
	}
	var resp RevertConversionResponse
	decodeJSONBody(t, rec.Body.Bytes(), &resp)
	if !resp.Reverted || resp.OrderID != 1 || resp.QuoteID != 77 || resp.DeletedRows != 3 || !resp.BridgeDeleted || resp.Warning != "" {
		t.Fatalf("response = %#v", resp)
	}
	if vodkaState.commits != 1 {
		t.Fatalf("vodka commits = %d, want 1", vodkaState.commits)
	}
	if !vodkaState.execContains("DELETE FROM orders_rows") || !vodkaState.execContains("DELETE FROM orders WHERE id = ? AND cdlan_stato = 'BOZZA'") || !mistraState.execContains("DELETE FROM orders.legacy_orders") {
		t.Fatalf("missing deletes vodka=%v mistra=%v", vodkaState.execs, mistraState.execs)
	}
}

func TestRevertConversionDeletesTrackedHubSpotArtifacts(t *testing.T) {
	vodkaState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?") {
				return nil, [][]driver.Value{orderDetailValues("BOZZA", "2026-05-22")}, nil
			}
			return nil, nil, errors.New("unexpected vodka query: " + query)
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			switch {
			case strings.Contains(query, "DELETE FROM orders_rows"):
				return 2, nil
			case strings.Contains(query, "DELETE FROM orders WHERE id = ? AND cdlan_stato = 'BOZZA'"):
				return 1, nil
			default:
				t.Fatalf("unexpected vodka exec: %s", query)
				return 0, nil
			}
		},
	}
	mistraState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders.legacy_orders") {
				return []string{"quote_id", "jdata"}, [][]driver.Value{{int64(77), `{"hubspot":{"file_id":"file-123","note_id":456}}`}}, nil
			}
			return nil, nil, errors.New("unexpected mistra query: " + query)
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			if !strings.Contains(query, "DELETE FROM orders.legacy_orders") {
				t.Fatalf("unexpected mistra exec: %s", query)
			}
			return 1, nil
		},
	}
	alyanteState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM Tsmi_Ordini_Esteso") {
				return []string{"count"}, [][]driver.Value{{int64(0)}}, nil
			}
			return nil, nil, errors.New("unexpected alyante query: " + query)
		},
	}
	hs, hsState := newOrdiniHubSpotDeleteServer(t, false)
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Vodka:   openOrdiniTestDB(t, vodkaState),
		Mistra:  openOrdiniTestDB(t, mistraState),
		Alyante: openOrdiniTestDB(t, alyanteState),
		HubSpot: hs,
		Logger:  logging.NewWithWriter(io.Discard, "debug"),
	})

	req := requestWithRoles(http.MethodPost, "/ordini/v1/orders/1/revert-conversion", nil, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
	}
	var resp RevertConversionResponse
	decodeJSONBody(t, rec.Body.Bytes(), &resp)
	if resp.Warning != "" || !resp.HubSpot.Attempted || !resp.HubSpot.NoteDeleted || !resp.HubSpot.FileDeleted {
		t.Fatalf("response = %#v", resp)
	}
	if got, want := strings.Join(hsState.paths, ","), "/crm/v3/objects/notes/456,/files/v3/files/file-123"; got != want {
		t.Fatalf("hubspot deletes = %s, want %s", got, want)
	}
}

func TestRevertConversionWarnsWhenTrackedHubSpotCleanupFails(t *testing.T) {
	vodkaState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?") {
				return nil, [][]driver.Value{orderDetailValues("BOZZA", "2026-05-22")}, nil
			}
			return nil, nil, errors.New("unexpected vodka query: " + query)
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			switch {
			case strings.Contains(query, "DELETE FROM orders_rows"):
				return 2, nil
			case strings.Contains(query, "DELETE FROM orders WHERE id = ? AND cdlan_stato = 'BOZZA'"):
				return 1, nil
			default:
				t.Fatalf("unexpected vodka exec: %s", query)
				return 0, nil
			}
		},
	}
	mistraState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders.legacy_orders") {
				return []string{"quote_id", "jdata"}, [][]driver.Value{{int64(77), `{"hubspot":{"file_id":"file-123","note_id":456}}`}}, nil
			}
			return nil, nil, errors.New("unexpected mistra query: " + query)
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			if !strings.Contains(query, "DELETE FROM orders.legacy_orders") {
				t.Fatalf("unexpected mistra exec: %s", query)
			}
			return 1, nil
		},
	}
	alyanteState := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM Tsmi_Ordini_Esteso") {
				return []string{"count"}, [][]driver.Value{{int64(0)}}, nil
			}
			return nil, nil, errors.New("unexpected alyante query: " + query)
		},
	}
	hs, _ := newOrdiniHubSpotDeleteServer(t, true)
	var log bytes.Buffer
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Vodka:   openOrdiniTestDB(t, vodkaState),
		Mistra:  openOrdiniTestDB(t, mistraState),
		Alyante: openOrdiniTestDB(t, alyanteState),
		HubSpot: hs,
		Logger:  logging.NewWithWriter(&log, "debug"),
	})

	req := requestWithRoles(http.MethodPost, "/ordini/v1/orders/1/revert-conversion", nil, true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
	}
	var resp RevertConversionResponse
	decodeJSONBody(t, rec.Body.Bytes(), &resp)
	if resp.Warning != "hubspot_cleanup_failed" || len(resp.Warnings) != 1 || resp.Warnings[0] != "hubspot_cleanup_failed" || !resp.BridgeDeleted {
		t.Fatalf("response = %#v", resp)
	}
	if !vodkaState.execContains("DELETE FROM orders_rows") || !mistraState.execContains("DELETE FROM orders.legacy_orders") {
		t.Fatalf("local cleanup did not complete vodka=%v mistra=%v", vodkaState.execs, mistraState.execs)
	}
	firstLogLine := strings.Split(strings.TrimSpace(log.String()), "\n")[0]
	entry := decodeLogEntry(t, firstLogLine)
	if entry["operation"] != "revert_conversion_hubspot_note" || entry["hubspot_note_id"] != float64(456) {
		t.Fatalf("missing hubspot cleanup log fields: %#v", entry)
	}
}

func TestActivationFinalSatisfiedRowSetsOrderAttivo(t *testing.T) {
	state := ordiniActivationDBState(t, 3, 3, orderRowValues(11, 1, 101, 1, "", 1))
	gw := newOrdiniGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orders/v1/set-order-activation" {
			t.Fatalf("unexpected gateway path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: openOrdiniTestDB(t, state), Arak: gw.client, Logger: logging.NewWithWriter(io.Discard, "debug")})

	req := requestWithRoles(http.MethodPatch, "/ordini/v1/orders/1/rows/11/activate", strings.NewReader(`{"activation_date":"2026-05-23"}`), true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		OrderState string `json:"order_state"`
	}
	decodeJSONBody(t, rec.Body.Bytes(), &resp)
	if resp.OrderState != string(OrderStateAttivo) {
		t.Fatalf("order state = %q, want ATTIVO", resp.OrderState)
	}
	if !state.queryContains("data_annullamento IS NOT NULL") || !state.queryContains("cdlan_qta = 0") || !state.execContains("SET cdlan_stato = 'ATTIVO'") {
		t.Fatalf("activation count or ATTIVO transition missing; queries=%v execs=%v", state.queries, state.execs)
	}
}

func TestActivationAttivoStateRaceReturnsConflict(t *testing.T) {
	state := ordiniActivationDBState(t, 1, 1, orderRowValues(11, 1, 101, 1, "", 1))
	state.exec = func(query string, args []driver.NamedValue) (int64, error) {
		if strings.Contains(query, "SET cdlan_stato = 'ATTIVO'") {
			return 0, nil
		}
		return 1, nil
	}
	gw := newOrdiniGateway(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: openOrdiniTestDB(t, state), Arak: gw.client, Logger: logging.NewWithWriter(io.Discard, "debug")})

	req := requestWithRoles(http.MethodPatch, "/ordini/v1/orders/1/rows/11/activate", strings.NewReader(`{"activation_date":"2026-05-23"}`), true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict || errorCode(t, rec.Body.Bytes()) != "wrong_state" {
		t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
	}
	if state.commits != 0 {
		t.Fatalf("expected no commit after ATTIVO race, got %d", state.commits)
	}
}

func TestActivationGatewayFailureRollsBackLocalUpdate(t *testing.T) {
	state := ordiniActivationDBState(t, 1, 1, orderRowValues(11, 1, 101, 1, "", 1))
	gw := newOrdiniGateway(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: openOrdiniTestDB(t, state), Arak: gw.client, Logger: logging.NewWithWriter(io.Discard, "debug")})

	req := requestWithRoles(http.MethodPatch, "/ordini/v1/orders/1/rows/11/activate", strings.NewReader(`{"activation_date":"2026-05-23"}`), true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway || errorCode(t, rec.Body.Bytes()) != "gateway_error" {
		t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
	}
	if state.commits != 0 {
		t.Fatalf("expected no commit after gateway failure, got %d", state.commits)
	}
}

func TestActivationRejectsCanceledOrZeroQuantityRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  []driver.Value
	}{
		{name: "canceled", row: orderRowValues(11, 1, 101, 1, "2026-05-01", 0)},
		{name: "zero quantity", row: orderRowValues(11, 1, 101, 0, "", 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := ordiniActivationDBState(t, 1, 0, tc.row)
			var gatewayHit atomic.Bool
			gw := newOrdiniGateway(t, func(w http.ResponseWriter, r *http.Request) {
				gatewayHit.Store(true)
				w.WriteHeader(http.StatusOK)
			})
			mux := http.NewServeMux()
			RegisterRoutes(mux, Deps{Vodka: openOrdiniTestDB(t, state), Arak: gw.client, Logger: logging.NewWithWriter(io.Discard, "debug")})

			req := requestWithRoles(http.MethodPatch, "/ordini/v1/orders/1/rows/11/activate", strings.NewReader(`{"activation_date":"2026-05-23"}`), true)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec.Body.Bytes()) != "precondition_missing" {
				t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
			}
			if gatewayHit.Load() || state.begins != 0 {
				t.Fatalf("gatewayHit=%v begins=%d", gatewayHit.Load(), state.begins)
			}
		})
	}
}

func TestRowOwnershipCheckRejectsForeignSerialRow(t *testing.T) {
	state := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			switch {
			case strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?"):
				return nil, [][]driver.Value{orderDetailValues("BOZZA", "2026-05-22")}, nil
			case strings.Contains(query, "FROM orders_rows") && strings.Contains(query, "WHERE orders_id = ? AND id = ?"):
				return nil, nil, sql.ErrNoRows
			default:
				return nil, nil, errors.New("unexpected query: " + query)
			}
		},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: openOrdiniTestDB(t, state), Logger: logging.NewWithWriter(io.Discard, "debug")})
	req := requestWithRoles(http.MethodPatch, "/ordini/v1/orders/1/rows/999/serial-number", strings.NewReader(`{"serial_number":"S1"}`), false)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.Bytes()) != "row_not_found" {
		t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
	}
}

func TestRowOwnershipCheckRejectsForeignTechnicalNotesRow(t *testing.T) {
	state := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders_rows") && strings.Contains(query, "WHERE orders_id = ? AND id = ?") {
				return nil, nil, sql.ErrNoRows
			}
			return nil, nil, errors.New("unexpected query: " + query)
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			t.Fatalf("unexpected technical-notes exec for foreign row: %s", query)
			return 0, nil
		},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: openOrdiniTestDB(t, state), Logger: logging.NewWithWriter(io.Discard, "debug")})
	req := requestWithRoles(http.MethodPatch, "/ordini/v1/orders/1/rows/999/technical-notes", strings.NewReader(`{"technical_notes":"note"}`), false)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.Bytes()) != "row_not_found" {
		t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
	}
	if len(state.execs) != 0 {
		t.Fatalf("execs = %d, want 0", len(state.execs))
	}
}

func TestRowOwnershipCheckRejectsForeignActivationRow(t *testing.T) {
	state := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			switch {
			case strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?"):
				return nil, [][]driver.Value{orderDetailValues("INVIATO", "2026-05-22")}, nil
			case strings.Contains(query, "FROM orders_rows") && strings.Contains(query, "WHERE orders_id = ? AND id = ?"):
				return nil, nil, sql.ErrNoRows
			default:
				return nil, nil, errors.New("unexpected query: " + query)
			}
		},
	}
	var gatewayHit atomic.Bool
	gw := newOrdiniGateway(t, func(w http.ResponseWriter, r *http.Request) {
		gatewayHit.Store(true)
		w.WriteHeader(http.StatusOK)
	})
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: openOrdiniTestDB(t, state), Arak: gw.client, Logger: logging.NewWithWriter(io.Discard, "debug")})
	req := requestWithRoles(http.MethodPatch, "/ordini/v1/orders/1/rows/999/activate", strings.NewReader(`{"activation_date":"2026-05-23"}`), true)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.Bytes()) != "row_not_found" {
		t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
	}
	if gatewayHit.Load() || state.begins != 0 {
		t.Fatalf("gatewayHit=%v begins=%d", gatewayHit.Load(), state.begins)
	}
}

func TestPDFNormalization(t *testing.T) {
	raw := []byte("%PDF-1.7\nbody")
	if got, err := normalizePDFBody(raw); err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("raw pdf = %q %v", got, err)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	if got, err := normalizePDFBody([]byte(`{"pdf":"` + encoded + `"}`)); err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("base64 pdf = %q %v", got, err)
	}
	if _, err := normalizePDFBody([]byte("not a pdf")); err == nil {
		t.Fatal("malformed pdf accepted")
	}
}

func TestPermissionGatesBackend(t *testing.T) {
	mux := http.NewServeMux()
	db := openOrdiniTestDB(t, &ordiniTestDBState{})
	gw := newOrdiniGateway(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	RegisterRoutes(mux, Deps{Vodka: db, Arak: gw.client, Logger: logging.NewWithWriter(io.Discard, "debug")})

	req := httptest.NewRequest(http.MethodGet, "/ordini/v1/orders", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing claims status = %d, want 401", rec.Code)
	}

	req = requestWithRoles(http.MethodPost, "/ordini/v1/orders/1/send-to-erp", multipartPDF(t), false)
	req.Header.Set("Content-Type", multipartContentType)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || errorCode(t, rec.Body.Bytes()) != "role_insufficient" {
		t.Fatalf("non-CR status/body=%d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	requireState(rec, OrderStateAttivo, OrderStateBozza)
	if rec.Code != http.StatusConflict || errorCode(t, rec.Body.Bytes()) != "wrong_state" {
		t.Fatalf("state gate status/body=%d %s", rec.Code, rec.Body.String())
	}
}

func TestBaseRoleCanReadOrders(t *testing.T) {
	state := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders") && strings.Contains(query, "ORDER BY id DESC") {
				return nil, [][]driver.Value{orderDetailValues("BOZZA", "2026-05-22")[:19]}, nil
			}
			return nil, nil, errors.New("unexpected query: " + query)
		},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: openOrdiniTestDB(t, state), Logger: logging.NewWithWriter(io.Discard, "debug")})
	req := requestWithRoles(http.MethodGet, "/ordini/v1/orders", nil, false)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	decodeJSONBody(t, rec.Body.Bytes(), &rows)
	if len(rows) != 1 || rows[0]["id"] != float64(1) {
		t.Fatalf("rows=%#v", rows)
	}
}

func TestElevatedHandlersRequireCustomerRelations(t *testing.T) {
	db := openOrdiniTestDB(t, &ordiniTestDBState{})
	gw := newOrdiniGateway(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Vodka: db, Alyante: db, Arak: gw.client, Logger: logging.NewWithWriter(io.Discard, "debug")})

	tests := []struct {
		name   string
		method string
		path   string
		body   io.Reader
	}{
		{name: "header", method: http.MethodPatch, path: "/ordini/v1/orders/1", body: strings.NewReader(`{"customer_po":"","confirmation_date":"","customer_id":1}`)},
		{name: "referents", method: http.MethodPatch, path: "/ordini/v1/orders/1/referents", body: strings.NewReader(`{"technical_name":"","technical_phone":"","technical_email":"","other_technical_name":"","other_technical_phone":"","other_technical_email":"","admin_name":"","admin_phone":"","admin_email":""}`)},
		{name: "send", method: http.MethodPost, path: "/ordini/v1/orders/1/send-to-erp"},
		{name: "revert", method: http.MethodPost, path: "/ordini/v1/orders/1/revert-conversion"},
		{name: "activate", method: http.MethodPatch, path: "/ordini/v1/orders/1/rows/1/activate", body: strings.NewReader(`{"activation_date":"2026-05-23"}`)},
		{name: "kickoff", method: http.MethodGet, path: "/ordini/v1/orders/1/kickoff.pdf"},
		{name: "activation form", method: http.MethodGet, path: "/ordini/v1/orders/1/activation-form.pdf"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := requestWithRoles(tc.method, tc.path, tc.body, false)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden || errorCode(t, rec.Body.Bytes()) != "role_insufficient" {
				t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestElevatedHandlersCheckRoleBeforeMissingDependencies(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Logger: logging.NewWithWriter(io.Discard, "debug")})

	tests := []struct {
		name   string
		method string
		path   string
		body   io.Reader
	}{
		{name: "header", method: http.MethodPatch, path: "/ordini/v1/orders/1", body: strings.NewReader(`{"customer_po":"","confirmation_date":"","customer_id":1}`)},
		{name: "send", method: http.MethodPost, path: "/ordini/v1/orders/1/send-to-erp"},
		{name: "revert", method: http.MethodPost, path: "/ordini/v1/orders/1/revert-conversion"},
		{name: "activate", method: http.MethodPatch, path: "/ordini/v1/orders/1/rows/1/activate", body: strings.NewReader(`{"activation_date":"2026-05-23"}`)},
		{name: "kickoff", method: http.MethodGet, path: "/ordini/v1/orders/1/kickoff.pdf"},
		{name: "activation form", method: http.MethodGet, path: "/ordini/v1/orders/1/activation-form.pdf"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := requestWithRoles(tc.method, tc.path, tc.body, false)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden || errorCode(t, rec.Body.Bytes()) != "role_insufficient" {
				t.Fatalf("status/body=%d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestOriginResolver(t *testing.T) {
	state := &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			switch {
			case strings.Contains(query, "FROM orders.legacy_orders"):
				return []string{"quote_id"}, [][]driver.Value{{int64(77)}}, nil
			case strings.Contains(query, "FROM quotes.quote"):
				return []string{"quote_number"}, [][]driver.Value{{"Q-77"}}, nil
			default:
				return nil, nil, errors.New("unexpected query: " + query)
			}
		},
	}
	h := &Handler{deps: Deps{Mistra: openOrdiniTestDB(t, state)}, logger: logging.NewWithWriter(io.Discard, "debug")}
	origin, err := h.loadOrigin(requestWithRoles(http.MethodGet, "/ordini/v1/orders/1", nil, false), 1)
	if err != nil {
		t.Fatalf("loadOrigin() error = %v", err)
	}
	if origin == nil || origin.QuoteID != 77 || origin.QuoteCode == nil || *origin.QuoteCode != "Q-77" || origin.QuoteURL != "/apps/quotes/quotes/77" {
		t.Fatalf("origin = %#v", origin)
	}

	state.query = func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(query, "FROM orders.legacy_orders") {
			return nil, nil, sql.ErrNoRows
		}
		return nil, nil, errors.New("unexpected query")
	}
	origin, err = h.loadOrigin(requestWithRoles(http.MethodGet, "/ordini/v1/orders/1", nil, false), 1)
	if err != nil || origin != nil {
		t.Fatalf("missing origin = %#v, %v", origin, err)
	}

	state.query = func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(query, "FROM orders.legacy_orders"):
			return []string{"quote_id"}, [][]driver.Value{{int64(77)}}, nil
		case strings.Contains(query, "FROM quotes.quote"):
			return nil, nil, sql.ErrNoRows
		default:
			return nil, nil, errors.New("unexpected query")
		}
	}
	var log bytes.Buffer
	h.logger = logging.NewWithWriter(&log, "debug")
	origin, err = h.loadOrigin(requestWithRoles(http.MethodGet, "/ordini/v1/orders/1", nil, false), 1)
	if err != nil || origin == nil || origin.QuoteID != 77 || origin.QuoteCode != nil {
		t.Fatalf("quote-missing origin = %#v, %v", origin, err)
	}
	entry := decodeLogEntry(t, log.String())
	if entry["operation"] != "origin_quote_lookup" || entry["quote_id"] != float64(77) {
		t.Fatalf("missing quote lookup warning fields: %#v", entry)
	}
}

func TestOriginFailureIsLoggedWithoutBreakingOrderDetail(t *testing.T) {
	vodka := openOrdiniTestDB(t, &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?") {
				return nil, [][]driver.Value{orderDetailValues("BOZZA", "2026-05-22")}, nil
			}
			return nil, nil, errors.New("unexpected vodka query: " + query)
		},
	})
	mistra := openOrdiniTestDB(t, &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			if strings.Contains(query, "FROM orders.legacy_orders") {
				return nil, nil, errors.New("legacy lookup failed")
			}
			return nil, nil, errors.New("unexpected mistra query: " + query)
		},
	})
	var log bytes.Buffer
	h := &Handler{deps: Deps{Vodka: vodka, Mistra: mistra}, logger: logging.NewWithWriter(&log, "debug")}

	order, err := h.getOrder(requestWithRoles(http.MethodGet, "/ordini/v1/orders/1", nil, false), 1)
	if err != nil {
		t.Fatalf("getOrder() error = %v", err)
	}
	if order == nil || order.Origin != nil {
		t.Fatalf("order origin = %#v", order)
	}
	entry := decodeLogEntry(t, log.String())
	if entry["operation"] != "origin_lookup" || entry["order_id"] != float64(1) || entry["request_id"] != "req-test" {
		t.Fatalf("missing origin failure log fields: %#v", entry)
	}
}

type ordiniHubSpotDeleteState struct {
	paths []string
}

func newOrdiniHubSpotDeleteServer(t *testing.T, fail bool) (*hubspot.Client, *ordiniHubSpotDeleteState) {
	t.Helper()
	state := &ordiniHubSpotDeleteState{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected HubSpot method/path: %s %s", r.Method, r.URL.Path)
		}
		state.paths = append(state.paths, r.URL.Path)
		if fail {
			http.Error(w, "delete failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	return hubspot.NewWithBaseURL("test-token", server.URL, server.Client()), state
}

const ordiniTestDriverName = "ordini_test_driver"

var (
	registerOrdiniDriverOnce sync.Once
	ordiniStateCounter       atomic.Uint64
	ordiniStatesMu           sync.Mutex
	ordiniStates             = map[string]*ordiniTestDBState{}
	multipartContentType     string
)

type ordiniTestDBState struct {
	mu      sync.Mutex
	query   func(string, []driver.NamedValue) ([]string, [][]driver.Value, error)
	exec    func(string, []driver.NamedValue) (int64, error)
	queries []string
	execs   []string
	begins  int
	commits int
}

func (s *ordiniTestDBState) queryContains(needle string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, query := range s.queries {
		if strings.Contains(query, needle) {
			return true
		}
	}
	return false
}

func (s *ordiniTestDBState) execContains(needle string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, query := range s.execs {
		if strings.Contains(query, needle) {
			return true
		}
	}
	return false
}

func openOrdiniTestDB(t *testing.T, state *ordiniTestDBState) *sql.DB {
	t.Helper()
	registerOrdiniDriverOnce.Do(func() {
		sql.Register(ordiniTestDriverName, ordiniTestDriver{})
	})
	dsn := t.Name() + "-" + strconv.FormatUint(ordiniStateCounter.Add(1), 10)
	ordiniStatesMu.Lock()
	ordiniStates[dsn] = state
	ordiniStatesMu.Unlock()
	db, err := sql.Open(ordiniTestDriverName, dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		ordiniStatesMu.Lock()
		delete(ordiniStates, dsn)
		ordiniStatesMu.Unlock()
	})
	return db
}

type ordiniTestDriver struct{}

func (ordiniTestDriver) Open(name string) (driver.Conn, error) {
	ordiniStatesMu.Lock()
	state := ordiniStates[name]
	ordiniStatesMu.Unlock()
	if state == nil {
		return nil, errors.New("missing ordini test state")
	}
	return ordiniTestConn{state: state}, nil
}

type ordiniTestConn struct {
	state *ordiniTestDBState
}

func (c ordiniTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c ordiniTestConn) Close() error { return nil }
func (c ordiniTestConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c ordiniTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.state.mu.Lock()
	c.state.begins++
	c.state.mu.Unlock()
	return ordiniTestTx{state: c.state}, nil
}

func (c ordiniTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.mu.Lock()
	c.state.queries = append(c.state.queries, query)
	queryFn := c.state.query
	c.state.mu.Unlock()
	if queryFn == nil {
		return nil, errors.New("unexpected query: " + query)
	}
	columns, values, err := queryFn(query, args)
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 && len(values) > 0 {
		columns = make([]string, len(values[0]))
		for i := range columns {
			columns[i] = "col_" + strconv.Itoa(i)
		}
	}
	return &ordiniTestRows{columns: columns, values: values}, nil
}

func (c ordiniTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.state.mu.Lock()
	c.state.execs = append(c.state.execs, query)
	execFn := c.state.exec
	c.state.mu.Unlock()
	if execFn == nil {
		return ordiniTestResult(0), nil
	}
	affected, err := execFn(query, args)
	if err != nil {
		return nil, err
	}
	return ordiniTestResult(affected), nil
}

var _ driver.QueryerContext = ordiniTestConn{}
var _ driver.ExecerContext = ordiniTestConn{}
var _ driver.ConnBeginTx = ordiniTestConn{}

type ordiniTestTx struct {
	state *ordiniTestDBState
}

func (tx ordiniTestTx) Commit() error {
	tx.state.mu.Lock()
	tx.state.commits++
	tx.state.mu.Unlock()
	return nil
}

func (tx ordiniTestTx) Rollback() error { return nil }

type ordiniTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *ordiniTestRows) Columns() []string { return r.columns }
func (r *ordiniTestRows) Close() error      { return nil }
func (r *ordiniTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

type ordiniTestResult int64

func (r ordiniTestResult) LastInsertId() (int64, error) { return 0, errors.New("not implemented") }
func (r ordiniTestResult) RowsAffected() (int64, error) { return int64(r), nil }

func ordiniSendDBState(t *testing.T, orderState string, rows [][]driver.Value) *ordiniTestDBState {
	t.Helper()
	return &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			switch {
			case strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?"):
				return nil, [][]driver.Value{orderDetailValues(orderState, "2026-05-22")}, nil
			case strings.Contains(query, "FROM orders_rows") && strings.Contains(query, "WHERE orders_id = ?"):
				return nil, rows, nil
			default:
				return nil, nil, errors.New("unexpected query: " + query)
			}
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			return 1, nil
		},
	}
}

func ordiniActivationDBState(t *testing.T, total, confirmed int64, row []driver.Value) *ordiniTestDBState {
	t.Helper()
	return &ordiniTestDBState{
		query: func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			switch {
			case strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?"):
				return nil, [][]driver.Value{orderDetailValues("INVIATO", "2026-05-22")}, nil
			case strings.Contains(query, "FROM orders_rows") && strings.Contains(query, "WHERE orders_id = ? AND id = ?"):
				return nil, [][]driver.Value{row}, nil
			case strings.Contains(query, "SELECT COUNT(id)") && !strings.Contains(query, "confirm_data_attivazione"):
				return []string{"count"}, [][]driver.Value{{total}}, nil
			case strings.Contains(query, "SELECT COUNT(id)") && strings.Contains(query, "confirm_data_attivazione"):
				return []string{"count"}, [][]driver.Value{{confirmed}}, nil
			default:
				return nil, nil, errors.New("unexpected query: " + query)
			}
		},
		exec: func(query string, args []driver.NamedValue) (int64, error) {
			return 1, nil
		},
	}
}

func orderDetailValues(state, confirmationDate string) []driver.Value {
	values := []driver.Value{
		int64(1), "1001", "TSC-ORDINE", "12", int64(2026), "12/2026", nil, "ACME Spa", int64(42),
		"2026-05-20", "IaaS", "0", "N", confirmationDate, state, "it", int64(0), int64(0), nil,
		"Sales", "30", nil, "4", "1", "10 giorni", "12 mesi", "PO-1",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "1", "2", int64(0), "EUR",
		"writer", "IT123", "CF123", "Via Roma", "Milano", "20100", "MI", "ABC1234",
		"2026-06-01", "1", nil, int64(0),
	}
	if confirmationDate == "" {
		values[13] = nil
	}
	return values
}

func orderRowValues(id, orderID, systemRow int64, qty float64, canceled string, confirmed int64) []driver.Value {
	var canceledValue driver.Value
	if canceled != "" {
		canceledValue = canceled
	}
	return []driver.Value{
		id, orderID, systemRow, "KIT", int64(1), "KIT-1", "ART", "Articolo", qty,
		float64(10), float64(2), float64(1), "M", "2026-05-23", "SN-1", confirmed, canceledValue,
	}
}

type ordiniGateway struct {
	client *arak.Client
}

func newOrdiniGateway(t *testing.T, handler http.HandlerFunc) *ordiniGateway {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":3600}`))
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return &ordiniGateway{client: arak.New(arak.Config{
		BaseURL:      server.URL,
		TokenURL:     server.URL + "/token",
		ClientID:     "client",
		ClientSecret: "secret",
	})}
}

func requestWithRoles(method, target string, body io.Reader, customerRelations bool) *http.Request {
	req := httptest.NewRequest(method, target, body)
	roles := applaunch.OrdiniAccessRoles()
	if customerRelations {
		roles = append(roles, applaunch.CustomerRelationsRoles()...)
	}
	claims := auth.Claims{Subject: "u1", Email: "user@example.test", Name: "User", Roles: roles, RawToken: "token"}
	req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, claims))
	return req.WithContext(logging.WithRequestID(req.Context(), "req-test"))
}

func multipartPDF(t *testing.T) io.Reader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "signed.pdf")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte("%PDF-1.7\nbody")); err != nil {
		t.Fatalf("part.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
	multipartContentType = writer.FormDataContentType()
	return bytes.NewReader(body.Bytes())
}

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return string(body)
}

func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var payload map[string]string
	decodeJSONBody(t, body, &payload)
	return payload["error"]
}

func decodeJSONBody(t *testing.T, body []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", string(body), err)
	}
}

func decodeLogEntry(t *testing.T, raw string) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one log line, got %d: %q", len(lines), raw)
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log unmarshal error = %v", err)
	}
	return entry
}

func containsAttr(attrs []any, key string, value any) bool {
	for i := 0; i+1 < len(attrs); i += 2 {
		if attrs[i] == key && attrs[i+1] == value {
			return true
		}
	}
	return false
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case error:
		return v.Error()
	default:
		return ""
	}
}
