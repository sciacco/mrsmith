package quotes

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

	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/hubspot"
)

func TestBuildVodkaOrderHeaderMatchesRecoveredGpUtilsContract(t *testing.T) {
	source := &quoteOrderSource{
		DocumentType:            ns("TSC-ORDINE"),
		ProposalType:            ns("SOSTITUZIONE"),
		PaymentMethod:           ns(" 402 "),
		Notes:                   ns("note operative"),
		Trial:                   ns("trial "),
		NextTermMonths:          24,
		InitialTermMonths:       12,
		BillMonths:              3,
		NrcChargeTime:           2,
		DeliveredInDays:         30,
		CustomerName:            ns("ACME S.p.A."),
		PartitaIVA:              ns("IT123"),
		CodiceFiscale:           ns("CF123"),
		Address:                 ns("Via Roma 1"),
		City:                    ns("Milano"),
		ZIP:                     ns("20100"),
		ProvinciaDiFatturazione: ns("MI - Milano"),
		Lingua:                  ns("ITA"),
		OwnerName:               ns("Sales Owner"),
		Services:                ns("[1,2]"),
		TemplateDescription:     ns("COLO Dedicated"),
		TemplateIsColo:          true,
		ReplaceOrders:           ns("123/2024"),
		RifOrdcli:               ns("PO-7"),
		CdlanNdoc:               "HP-7363",
		CdlanAnno:               "2026",
	}

	header, reqErr := buildVodkaOrderHeader(
		source,
		map[int]string{1: "Colocation", 2: "IaaS"},
		98765,
		time.Date(2026, 5, 18, 10, 30, 0, 0, time.UTC),
	)
	if reqErr != nil {
		t.Fatalf("unexpected request error: %v", reqErr)
	}

	if header.CdlanSystemODV != 98765 {
		t.Fatalf("cdlan_systemodv = %d, want 98765", header.CdlanSystemODV)
	}
	if header.CdlanTipodoc != "TSC-ORDINE" || header.CdlanNdoc != "HP-7363" || header.CdlanAnno != "2026" {
		t.Fatalf("unexpected document mapping: %#v", header)
	}
	if header.CdlanDatadoc != "2026-05-18" {
		t.Fatalf("cdlan_datadoc = %q, want 2026-05-18", header.CdlanDatadoc)
	}
	if header.CdlanTipoOrd != "A" {
		t.Fatalf("cdlan_tipo_ord = %q, want A", header.CdlanTipoOrd)
	}
	if header.CdlanTacitoRin != "0" {
		t.Fatalf("spot order tacito rinnovo = %q, want 0", header.CdlanTacitoRin)
	}
	if header.CdlanCodTerminiPag != "402" {
		t.Fatalf("payment = %q, want 402", header.CdlanCodTerminiPag)
	}
	if got := ptrValue(header.CdlanNote); got != "trial note operative" {
		t.Fatalf("cdlan_note = %q, want trial note operative", got)
	}
	if got := ptrValue(header.ProfilePV); got != "MI" {
		t.Fatalf("profile_pv = %q, want MI", got)
	}
	if header.ProfileLang != "it" {
		t.Fatalf("profile_lang = %q, want it", header.ProfileLang)
	}
	if header.ServiceType != "Colocation, IaaS" {
		t.Fatalf("service_type = %q, want Colocation, IaaS", header.ServiceType)
	}
	if header.IsColo != "Colocation variabile" {
		t.Fatalf("is_colo = %q, want Colocation variabile", header.IsColo)
	}
}

func TestBuildVodkaOrderRowMatchesRecoveredGpUtilsContract(t *testing.T) {
	source := quoteOrderRowSource{
		RowID:               321,
		ProductCode:         "PRD-1",
		NRC:                 10,
		MRC:                 129.5,
		Quantity:            2,
		BundlePrefixRow:     ns("KIT-A"),
		InternalName:        ns("Fallback"),
		ExtendedDescription: ns("Dettaglio esteso"),
		Translations:        json.RawMessage(`[{"language":"it","short":"Prodotto IT"},{"language":"en","short":"Product EN"}]`),
	}

	row := buildVodkaOrderRow(701, "it", source, 9101, 8301)

	if row.OrdersID != 701 || row.CdlanSystemODVRow != 9101 || row.CdlanSerialNumber != 8301 {
		t.Fatalf("unexpected sequence/id mapping: %#v", row)
	}
	if row.CdlanDescart != "Prodotto IT\r\nDettaglio esteso" {
		t.Fatalf("cdlan_descart = %q", row.CdlanDescart)
	}
	if row.CdlanQta != "2" {
		t.Fatalf("cdlan_qta = %q, want 2", row.CdlanQta)
	}
	if row.CdlanPrezzo != "129,5" {
		t.Fatalf("cdlan_prezzo = %q, want 129,5", row.CdlanPrezzo)
	}
	if row.CdlanPrezzoAttivazione != "10" {
		t.Fatalf("cdlan_prezzo_attivazione = %q, want 10", row.CdlanPrezzoAttivazione)
	}
	if got := ptrValue(row.CdlanCodiceKit); got != "KIT-A" {
		t.Fatalf("cdlan_codice_kit = %q, want KIT-A", got)
	}
}

func TestOrderConversionParsingAndFormattingHelpers(t *testing.T) {
	ndoc, anno, err := parseDealOrderCode(" HP-7363 / 2026 ")
	if err != nil {
		t.Fatalf("parseDealOrderCode returned error: %v", err)
	}
	if ndoc != "HP-7363" || anno != "2026" {
		t.Fatalf("deal code = %q/%q, want HP-7363/2026", ndoc, anno)
	}
	if _, _, err := parseDealOrderCode("missing-year"); err == nil {
		t.Fatalf("expected malformed deal number to fail")
	}

	ids := parseServiceCategoryIDs(`[1,"2",3]`)
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Fatalf("parseServiceCategoryIDs JSON = %#v", ids)
	}
	if got := normalizeLegacyQuoteLanguage("ING"); got != "en" {
		t.Fatalf("ING language = %q, want en", got)
	}
	if got := orderPDFFilename("HP-7363/2026", time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)); got != "order_HP-7363_2026_2026-05-18.pdf" {
		t.Fatalf("filename = %q", got)
	}
}

func TestCanConvertQuoteToOrderOnlyAllowsApproved(t *testing.T) {
	for _, tt := range []struct {
		status string
		want   bool
	}{
		{status: "APPROVED", want: true},
		{status: " approved ", want: true},
		{status: "DRAFT", want: false},
		{status: "PENDING_APPROVAL", want: false},
		{status: "APPROVAL_NOT_NEEDED", want: false},
		{status: "ESIGN_COMPLETED", want: false},
		{status: "", want: false},
	} {
		if got := canConvertQuoteToOrder(tt.status); got != tt.want {
			t.Fatalf("canConvertQuoteToOrder(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestConvertQuoteToOrderRejectsNonApprovedBeforeSideEffects(t *testing.T) {
	h := &Handler{db: openOrderConversionStatusTestDB(t, "DRAFT")}

	result, reqErr, err := h.convertQuoteToOrder(context.Background(), 42)
	if err != nil {
		t.Fatalf("convertQuoteToOrder returned unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
	if reqErr == nil {
		t.Fatalf("expected request error")
	}
	if reqErr.status != http.StatusConflict || reqErr.code != "quote_status_not_approved" {
		t.Fatalf("request error = %#v, want 409 quote_status_not_approved", reqErr)
	}
}

func TestFindLegacyOrderSkipsStaleBridgeRows(t *testing.T) {
	mistra := openOrderBridgeTestDB(t, &orderBridgeTestState{
		bridges: []int64{200, 100},
	})
	vodka := openOrderBridgeTestDB(t, &orderBridgeTestState{
		existingVodka: map[int64]bool{100: true},
	})
	h := &Handler{db: mistra, vodkaDB: vodka}

	bridge, err := h.findLegacyOrder(context.Background(), 42)
	if err != nil {
		t.Fatalf("findLegacyOrder() error = %v", err)
	}
	if bridge == nil || bridge.VodkaID != 100 {
		t.Fatalf("bridge = %#v, want vodka 100", bridge)
	}
}

func TestFindLegacyOrderReturnsNilWhenAllBridgeRowsAreStale(t *testing.T) {
	mistra := openOrderBridgeTestDB(t, &orderBridgeTestState{
		bridges: []int64{200},
	})
	vodka := openOrderBridgeTestDB(t, &orderBridgeTestState{
		existingVodka: map[int64]bool{},
	})
	h := &Handler{db: mistra, vodkaDB: vodka}

	bridge, err := h.findLegacyOrder(context.Background(), 42)
	if err != nil {
		t.Fatalf("findLegacyOrder() error = %v", err)
	}
	if bridge != nil {
		t.Fatalf("bridge = %#v, want nil", bridge)
	}
}

func TestConvertQuoteToOrderPersistsHubSpotMetadata(t *testing.T) {
	mistraState := &orderConversionFlowTestState{
		status:        "APPROVED",
		bridgeOrderID: 100,
		bridgeJData:   `{}`,
	}
	arakClient, pdfCalls := newOrderConversionArakClient(t)
	hsClient, hsState := newOrderConversionHubSpotServer(t, "file-123", "456")
	h := &Handler{
		db:   openOrderConversionFlowTestDB(t, mistraState),
		arak: arakClient,
		hs:   hsClient,
	}

	result, reqErr, err := h.convertQuoteToOrder(context.Background(), 42)
	if err != nil || reqErr != nil {
		t.Fatalf("convertQuoteToOrder() err=%v reqErr=%v", err, reqErr)
	}
	if result == nil || !result.Success || result.FileID == nil || *result.FileID != "file-123" || result.NoteID == nil || *result.NoteID != 456 {
		t.Fatalf("result = %#v", result)
	}
	if *pdfCalls != 1 || hsState.uploads != 1 || hsState.notes != 1 {
		t.Fatalf("calls pdf=%d uploads=%d notes=%d", *pdfCalls, hsState.uploads, hsState.notes)
	}
	if len(mistraState.metadataUpdates) != 2 {
		t.Fatalf("metadata updates = %#v", mistraState.metadataUpdates)
	}
	if mistraState.metadataUpdates[0].FileID != "file-123" || mistraState.metadataUpdates[0].NoteID != 0 {
		t.Fatalf("file metadata update = %#v", mistraState.metadataUpdates[0])
	}
	if mistraState.metadataUpdates[1].FileID != "file-123" || mistraState.metadataUpdates[1].NoteID != 456 {
		t.Fatalf("note metadata update = %#v", mistraState.metadataUpdates[1])
	}
}

func TestConvertQuoteToOrderReusesPersistedHubSpotFile(t *testing.T) {
	mistraState := &orderConversionFlowTestState{
		status:        "APPROVED",
		bridgeOrderID: 100,
		bridgeJData:   `{"hubspot":{"deal_id":"240882923764","file_id":"file-123"}}`,
	}
	hsClient, hsState := newOrderConversionHubSpotServer(t, "", "456")
	h := &Handler{
		db: openOrderConversionFlowTestDB(t, mistraState),
		hs: hsClient,
	}

	result, reqErr, err := h.convertQuoteToOrder(context.Background(), 42)
	if err != nil || reqErr != nil {
		t.Fatalf("convertQuoteToOrder() err=%v reqErr=%v", err, reqErr)
	}
	if result == nil || !result.Success || result.FileID == nil || *result.FileID != "file-123" || result.NoteID == nil || *result.NoteID != 456 {
		t.Fatalf("result = %#v", result)
	}
	if hsState.uploads != 0 || hsState.notes != 1 {
		t.Fatalf("hubspot calls uploads=%d notes=%d", hsState.uploads, hsState.notes)
	}
	if len(mistraState.metadataUpdates) != 1 || mistraState.metadataUpdates[0].FileID != "file-123" || mistraState.metadataUpdates[0].NoteID != 456 {
		t.Fatalf("metadata updates = %#v", mistraState.metadataUpdates)
	}
}

func TestConvertQuoteToOrderReusesPersistedHubSpotNote(t *testing.T) {
	mistraState := &orderConversionFlowTestState{
		status:        "APPROVED",
		bridgeOrderID: 100,
		bridgeJData:   `{"hubspot":{"deal_id":"240882923764","file_id":"file-123","note_id":456}}`,
	}
	hsClient, hsState := newOrderConversionHubSpotServer(t, "", "")
	h := &Handler{
		db: openOrderConversionFlowTestDB(t, mistraState),
		hs: hsClient,
	}

	result, reqErr, err := h.convertQuoteToOrder(context.Background(), 42)
	if err != nil || reqErr != nil {
		t.Fatalf("convertQuoteToOrder() err=%v reqErr=%v", err, reqErr)
	}
	if result == nil || !result.Success || result.FileID == nil || *result.FileID != "file-123" || result.NoteID == nil || *result.NoteID != 456 {
		t.Fatalf("result = %#v", result)
	}
	if hsState.uploads != 0 || hsState.notes != 0 || len(mistraState.metadataUpdates) != 0 {
		t.Fatalf("unexpected work uploads=%d notes=%d updates=%#v", hsState.uploads, hsState.notes, mistraState.metadataUpdates)
	}
}

type orderConversionHubSpotServerState struct {
	uploads int
	notes   int
}

func newOrderConversionHubSpotServer(t *testing.T, uploadID, noteID string) (*hubspot.Client, *orderConversionHubSpotServerState) {
	t.Helper()
	state := &orderConversionHubSpotServerState{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/files/v3/files":
			state.uploads++
			if uploadID == "" {
				t.Fatalf("unexpected HubSpot file upload")
			}
			httputilJSON(t, w, map[string]any{"id": uploadID})
		case r.Method == http.MethodPost && r.URL.Path == "/crm/v3/objects/notes":
			state.notes++
			if noteID == "" {
				t.Fatalf("unexpected HubSpot note create")
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode HubSpot note body: %v", err)
			}
			properties, _ := body["properties"].(map[string]any)
			if properties["hs_attachment_ids"] != "file-123" {
				t.Fatalf("note attachment = %#v", properties["hs_attachment_ids"])
			}
			httputilJSON(t, w, map[string]any{"id": noteID})
		default:
			t.Fatalf("unexpected HubSpot request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	return hubspot.NewWithBaseURL("test-token", server.URL, server.Client()), state
}

func newOrderConversionArakClient(t *testing.T) (*arak.Client, *int) {
	t.Helper()
	pdfCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/token":
			httputilJSON(t, w, map[string]any{"access_token": "test-token", "expires_in": 3600})
		case r.Method == http.MethodGet && r.URL.Path == "/orders/v1/order/pdf/100/generate":
			pdfCalls++
			_, _ = io.WriteString(w, "%PDF-test")
		default:
			t.Fatalf("unexpected Arak request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	return arak.New(arak.Config{
		BaseURL:      server.URL,
		TokenURL:     server.URL + "/token",
		ClientID:     "client",
		ClientSecret: "secret",
	}), &pdfCalls
}

func httputilJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func ns(value string) sql.NullString {
	return sql.NullString{String: value, Valid: true}
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func openOrderConversionStatusTestDB(t *testing.T, status string) *sql.DB {
	t.Helper()

	registerOrderConversionStatusTestDriver()

	db, err := sql.Open(orderConversionStatusTestDriverName, status)
	if err != nil {
		t.Fatalf("failed to open order conversion status test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

const orderConversionStatusTestDriverName = "quotes_order_conversion_status_test_driver"

var registerOrderConversionStatusDriverOnce sync.Once

func registerOrderConversionStatusTestDriver() {
	registerOrderConversionStatusDriverOnce.Do(func() {
		sql.Register(orderConversionStatusTestDriverName, orderConversionStatusTestDriver{})
	})
}

type orderConversionStatusTestDriver struct{}

func (orderConversionStatusTestDriver) Open(name string) (driver.Conn, error) {
	return &orderConversionStatusTestConn{status: name}, nil
}

type orderConversionStatusTestConn struct {
	status string
}

func (c *orderConversionStatusTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *orderConversionStatusTestConn) Close() error { return nil }

func (c *orderConversionStatusTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *orderConversionStatusTestConn) Ping(context.Context) error { return nil }

func (c *orderConversionStatusTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if strings.Contains(query, `FROM quotes.quote q`) && strings.Contains(query, `LEFT JOIN loader.hubs_company hc`) {
		return &publishHandlerTestRows{
			columns: []string{
				"id", "quote_number", "customer_id", "deal_number", "owner",
				"document_type", "replace_orders", "template", "services",
				"proposal_type", "initial_term_months", "next_term_months", "bill_months",
				"delivered_in_days", "status", "notes", "trial", "nrc_charge_time",
				"hs_deal_id", "description", "payment_method",
				"customer_name", "customer_number", "partita_iva", "owner_name",
				"city", "zip", "country", "provincia_di_fatturazione", "codice_fiscale",
				"address", "lingua", "template_description", "template_is_colo", "rif_ordcli", "rif_tech_nom",
				"rif_tech_tel", "rif_tech_email", "rif_altro_tech_nom", "rif_altro_tech_tel",
				"rif_altro_tech_email", "rif_adm_nom", "rif_adm_tech_tel", "rif_adm_tech_email",
			},
			values: [][]driver.Value{{
				int64(42), "SP-42/2026", int64(1001), "HP-7363/2026", "owner-1",
				"TSC-ORDINE", nil, "template-1", "[1]",
				"NUOVO", int64(12), int64(12), int64(1),
				int64(30), c.status, nil, nil, int64(1),
				int64(240882923764), "Descrizione", "402",
				"ACME S.p.A.", "C-1001", "IT123", "Sales Owner",
				"Milano", "20100", "IT", "MI", "CF123",
				"Via Roma 1", "ITA", "Standard IT", false, nil, nil,
				nil, nil, nil, nil,
				nil, nil, nil, nil,
			}},
		}, nil
	}
	return nil, errors.New("unexpected query before APPROVED guard: " + query)
}

var _ driver.QueryerContext = (*orderConversionStatusTestConn)(nil)
var _ driver.Pinger = (*orderConversionStatusTestConn)(nil)

const orderBridgeTestDriverName = "quotes_order_bridge_test_driver"

var (
	registerOrderBridgeTestDriverOnce sync.Once
	orderBridgeStatesMu               sync.Mutex
	orderBridgeStates                 = map[string]*orderBridgeTestState{}
)

type orderBridgeTestState struct {
	bridges       []int64
	existingVodka map[int64]bool
}

func openOrderBridgeTestDB(t *testing.T, state *orderBridgeTestState) *sql.DB {
	t.Helper()
	registerOrderBridgeTestDriverOnce.Do(func() {
		sql.Register(orderBridgeTestDriverName, orderBridgeTestDriver{})
	})
	dsn := t.Name() + time.Now().Format(time.RFC3339Nano)
	orderBridgeStatesMu.Lock()
	orderBridgeStates[dsn] = state
	orderBridgeStatesMu.Unlock()
	db, err := sql.Open(orderBridgeTestDriverName, dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		orderBridgeStatesMu.Lock()
		delete(orderBridgeStates, dsn)
		orderBridgeStatesMu.Unlock()
	})
	return db
}

type orderBridgeTestDriver struct{}

func (orderBridgeTestDriver) Open(name string) (driver.Conn, error) {
	orderBridgeStatesMu.Lock()
	state := orderBridgeStates[name]
	orderBridgeStatesMu.Unlock()
	if state == nil {
		return nil, errors.New("missing order bridge test state")
	}
	return &orderBridgeTestConn{state: state}, nil
}

type orderBridgeTestConn struct {
	state *orderBridgeTestState
}

func (c *orderBridgeTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *orderBridgeTestConn) Close() error { return nil }

func (c *orderBridgeTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *orderBridgeTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "FROM orders.legacy_orders"):
		values := make([][]driver.Value, 0, len(c.state.bridges))
		for _, id := range c.state.bridges {
			values = append(values, []driver.Value{id, nil})
		}
		return &publishHandlerTestRows{columns: []string{"vodka_id", "jdata"}, values: values}, nil
	case strings.Contains(query, "FROM orders") && strings.Contains(query, "WHERE id = ?"):
		orderID, _ := args[0].Value.(int64)
		if c.state.existingVodka[orderID] {
			return &publishHandlerTestRows{columns: []string{"id"}, values: [][]driver.Value{{orderID}}}, nil
		}
		return &publishHandlerTestRows{columns: []string{"id"}}, nil
	default:
		return nil, errors.New("unexpected order bridge query: " + query)
	}
}

var _ driver.QueryerContext = (*orderBridgeTestConn)(nil)

const orderConversionFlowTestDriverName = "quotes_order_conversion_flow_test_driver"

var (
	registerOrderConversionFlowTestDriverOnce sync.Once
	orderConversionFlowStatesMu               sync.Mutex
	orderConversionFlowStates                 = map[string]*orderConversionFlowTestState{}
)

type orderConversionFlowTestState struct {
	status          string
	bridgeOrderID   int64
	bridgeJData     string
	metadataUpdates []orderConversionHubSpotMetadata
}

func openOrderConversionFlowTestDB(t *testing.T, state *orderConversionFlowTestState) *sql.DB {
	t.Helper()
	registerOrderConversionFlowTestDriverOnce.Do(func() {
		sql.Register(orderConversionFlowTestDriverName, orderConversionFlowTestDriver{})
	})
	dsn := t.Name() + time.Now().Format(time.RFC3339Nano)
	orderConversionFlowStatesMu.Lock()
	orderConversionFlowStates[dsn] = state
	orderConversionFlowStatesMu.Unlock()
	db, err := sql.Open(orderConversionFlowTestDriverName, dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		orderConversionFlowStatesMu.Lock()
		delete(orderConversionFlowStates, dsn)
		orderConversionFlowStatesMu.Unlock()
	})
	return db
}

type orderConversionFlowTestDriver struct{}

func (orderConversionFlowTestDriver) Open(name string) (driver.Conn, error) {
	orderConversionFlowStatesMu.Lock()
	state := orderConversionFlowStates[name]
	orderConversionFlowStatesMu.Unlock()
	if state == nil {
		return nil, errors.New("missing order conversion flow test state")
	}
	return &orderConversionFlowTestConn{state: state}, nil
}

type orderConversionFlowTestConn struct {
	state *orderConversionFlowTestState
}

func (c *orderConversionFlowTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *orderConversionFlowTestConn) Close() error { return nil }

func (c *orderConversionFlowTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *orderConversionFlowTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, `FROM quotes.quote q`) && strings.Contains(query, `LEFT JOIN loader.hubs_company hc`):
		return &publishHandlerTestRows{
			columns: orderConversionSourceColumns(),
			values:  [][]driver.Value{orderConversionSourceValues(c.state.status)},
		}, nil
	case strings.Contains(query, "FROM orders.legacy_orders"):
		jdata := c.state.bridgeJData
		if jdata == "" {
			jdata = "{}"
		}
		return &publishHandlerTestRows{
			columns: []string{"vodka_id", "jdata"},
			values:  [][]driver.Value{{c.state.bridgeOrderID, jdata}},
		}, nil
	default:
		return nil, errors.New("unexpected order conversion query: " + query)
	}
}

func (c *orderConversionFlowTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if !strings.Contains(query, "UPDATE orders.legacy_orders") {
		return nil, errors.New("unexpected order conversion exec: " + query)
	}
	if len(args) != 3 || namedInt64(args[0]) != 42 || namedInt64(args[1]) != 100 {
		return nil, errors.New("unexpected metadata update args")
	}
	raw, _ := args[2].Value.(string)
	var metadata orderConversionHubSpotMetadata
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil, err
	}
	c.state.metadataUpdates = append(c.state.metadataUpdates, metadata)
	return driver.RowsAffected(1), nil
}

func orderConversionSourceColumns() []string {
	return []string{
		"id", "quote_number", "customer_id", "deal_number", "owner",
		"document_type", "replace_orders", "template", "services",
		"proposal_type", "initial_term_months", "next_term_months", "bill_months",
		"delivered_in_days", "status", "notes", "trial", "nrc_charge_time",
		"hs_deal_id", "description", "payment_method",
		"customer_name", "customer_number", "partita_iva", "owner_name",
		"city", "zip", "country", "provincia_di_fatturazione", "codice_fiscale",
		"address", "lingua", "template_description", "template_is_colo", "rif_ordcli", "rif_tech_nom",
		"rif_tech_tel", "rif_tech_email", "rif_altro_tech_nom", "rif_altro_tech_tel",
		"rif_altro_tech_email", "rif_adm_nom", "rif_adm_tech_tel", "rif_adm_tech_email",
	}
}

func orderConversionSourceValues(status string) []driver.Value {
	return []driver.Value{
		int64(42), "SP-42/2026", int64(1001), "HP-7363/2026", "owner-1",
		"TSC-ORDINE", nil, "template-1", "[1]",
		"NUOVO", int64(12), int64(12), int64(1),
		int64(30), status, nil, nil, int64(1),
		int64(240882923764), "Descrizione", "402",
		"ACME S.p.A.", "C-1001", "IT123", "Sales Owner",
		"Milano", "20100", "IT", "MI", "CF123",
		"Via Roma 1", "ITA", "Standard IT", false, nil, nil,
		nil, nil, nil, nil,
		nil, nil, nil, nil,
	}
}

var _ driver.QueryerContext = (*orderConversionFlowTestConn)(nil)
var _ driver.ExecerContext = (*orderConversionFlowTestConn)(nil)

func namedInt64(value driver.NamedValue) int64 {
	switch v := value.Value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}
