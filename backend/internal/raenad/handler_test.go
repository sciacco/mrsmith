package raenad

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/hubspot"
)

func TestRegisterRoutesProtectsFullSurface(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{})

	for _, route := range raenadRoutes() {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected protected route to return 401 without claims, got %d body=%q", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRegisterRoutesRequiresAenadRole(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{})

	req := httptest.NewRequest(http.MethodGet, "/aenad/v1/quotes", nil)
	req = withRoles(req, "viewer")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMistraRoutesReturnRouteScopedUnavailable(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{})

	req := httptest.NewRequest(http.MethodGet, "/aenad/v1/quotes", nil)
	req = withRoles(req, "app_aenad_access")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "raenad_database_not_configured") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestUnrelatedMissingDependenciesDoNotBlockQuoteList(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Mistra: openRaenadTestDB(t)})

	req := httptest.NewRequest(http.MethodGet, "/aenad/v1/quotes", nil)
	req = withRoles(req, "app_aenad_access")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestRouteSpecificDependencyFailures(t *testing.T) {
	db := openRaenadTestDB(t)
	renderer := fakePDFRenderer{}

	cases := []struct {
		name string
		deps Deps
		req  *http.Request
		want string
	}{
		{
			name: "articles require alyante",
			deps: Deps{Mistra: db, ConfigDB: db},
			req:  httptest.NewRequest(http.MethodGet, "/aenad/v1/quotes/articles", nil),
			want: "alyante_database_not_configured",
		},
		{
			name: "prospects require hubspot",
			deps: Deps{Mistra: db, ConfigDB: db},
			req:  httptest.NewRequest(http.MethodPost, "/aenad/v1/quotes/prospects", nil),
			want: "hubspot_not_configured",
		},
		{
			name: "pdf create requires carbone",
			deps: Deps{Mistra: db, ConfigDB: db},
			req:  httptest.NewRequest(http.MethodPost, "/aenad/v1/quotes/123/pdf-exports", nil),
			want: "raenad_pdf_not_configured",
		},
		{
			name: "pdf download requires config before carbone",
			deps: Deps{Mistra: db, Carbone: renderer},
			req:  httptest.NewRequest(http.MethodGet, "/aenad/v1/quotes/123/pdf-exports/456/download", nil),
			want: "raenad_config_not_configured",
		},
		{
			name: "stage transition requires hubspot after databases",
			deps: Deps{Mistra: db, ConfigDB: db},
			req:  httptest.NewRequest(http.MethodPost, "/aenad/v1/quotes/123/hubspot/stage", nil),
			want: "hubspot_not_configured",
		},
		{
			name: "attach requires queue config before later work",
			deps: Deps{Mistra: db, Carbone: renderer},
			req:  httptest.NewRequest(http.MethodPost, "/aenad/v1/quotes/123/pdf-exports/456/attach", nil),
			want: "raenad_config_not_configured",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			RegisterRoutes(mux, tc.deps)

			req := withRoles(tc.req, "app_aenad_access")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d body=%q", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("expected %q in body, got %q", tc.want, rec.Body.String())
			}
		})
	}
}

func TestConfiguredSkeletonRoutesReturnNotImplemented(t *testing.T) {
	db := openRaenadTestDB(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   db,
		ConfigDB: db,
		Alyante:  db,
		HubSpot:  hubspot.New("test-token"),
		Carbone:  fakePDFRenderer{},
	})

	for _, route := range raenadRoutes() {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			if isImplementedReferenceRoute(route.method, route.path) {
				t.Skip("implemented in slice 3")
			}
			if isImplementedQuoteLifecycleRoute(route.method, route.path) {
				t.Skip("implemented in slice 5")
			}
			if isImplementedProspectRoute(route.method, route.path) {
				t.Skip("implemented in slice 9")
			}
			if isImplementedStageTransitionRoute(route.method, route.path) {
				t.Skip("implemented in slice 10")
			}
			if isImplementedPDFExportRoute(route.method, route.path) {
				t.Skip("implemented in slice 11")
			}

			req := httptest.NewRequest(route.method, route.path, nil)
			req = withRoles(req, "app_aenad_access")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("expected 501, got %d body=%q", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "raenad_endpoint_not_implemented") {
				t.Fatalf("unexpected body: %q", rec.Body.String())
			}
		})
	}
}

func withRoles(req *http.Request, roles ...string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
		Subject: "user-1",
		Name:    "Aenad User",
		Email:   "aenad@example.com",
		Roles:   roles,
	}))
}

func raenadRoutes() []struct {
	method string
	path   string
} {
	return []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/aenad/v1/quotes/customers"},
		{http.MethodPost, "/aenad/v1/quotes/prospects"},
		{http.MethodGet, "/aenad/v1/quotes/payment-methods"},
		{http.MethodGet, "/aenad/v1/quotes/stages"},
		{http.MethodGet, "/aenad/v1/quotes/defaults"},
		{http.MethodGet, "/aenad/v1/quotes/articles"},
		{http.MethodGet, "/aenad/v1/quotes"},
		{http.MethodPost, "/aenad/v1/quotes"},
		{http.MethodGet, "/aenad/v1/quotes/123"},
		{http.MethodPut, "/aenad/v1/quotes/123"},
		{http.MethodPost, "/aenad/v1/quotes/123/ready"},
		{http.MethodPost, "/aenad/v1/quotes/123/hubspot/retry"},
		{http.MethodPost, "/aenad/v1/quotes/123/hubspot/stage"},
		{http.MethodGet, "/aenad/v1/quotes/123/pdf-exports"},
		{http.MethodPost, "/aenad/v1/quotes/123/pdf-exports"},
		{http.MethodGet, "/aenad/v1/quotes/123/pdf-exports/456/download"},
		{http.MethodPost, "/aenad/v1/quotes/123/pdf-exports/456/attach"},
	}
}

func isImplementedReferenceRoute(method, path string) bool {
	return method == http.MethodGet && (path == "/aenad/v1/quotes/customers" ||
		path == "/aenad/v1/quotes/payment-methods" ||
		path == "/aenad/v1/quotes/stages" ||
		path == "/aenad/v1/quotes/defaults" ||
		path == "/aenad/v1/quotes/articles")
}

func isImplementedQuoteLifecycleRoute(method, path string) bool {
	return (method == http.MethodGet && path == "/aenad/v1/quotes") ||
		(method == http.MethodPost && path == "/aenad/v1/quotes") ||
		(method == http.MethodGet && path == "/aenad/v1/quotes/123") ||
		(method == http.MethodPut && path == "/aenad/v1/quotes/123") ||
		(method == http.MethodPost && path == "/aenad/v1/quotes/123/ready") ||
		(method == http.MethodPost && path == "/aenad/v1/quotes/123/hubspot/retry")
}

func isImplementedProspectRoute(method, path string) bool {
	return method == http.MethodPost && path == "/aenad/v1/quotes/prospects"
}

func isImplementedStageTransitionRoute(method, path string) bool {
	return method == http.MethodPost && path == "/aenad/v1/quotes/123/hubspot/stage"
}

func isImplementedPDFExportRoute(method, path string) bool {
	return (method == http.MethodGet && path == "/aenad/v1/quotes/123/pdf-exports") ||
		(method == http.MethodPost && path == "/aenad/v1/quotes/123/pdf-exports") ||
		(method == http.MethodGet && path == "/aenad/v1/quotes/123/pdf-exports/456/download") ||
		(method == http.MethodPost && path == "/aenad/v1/quotes/123/pdf-exports/456/attach")
}

type fakePDFRenderer struct{}

func (fakePDFRenderer) GeneratePDF(context.Context, string, any) ([]byte, error) {
	return []byte("%PDF"), nil
}

func openRaenadTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return openRaenadTestDBWithState(t, nil)
}

func openRaenadTestDBWithState(t *testing.T, state *raenadTestState) *sql.DB {
	t.Helper()
	registerRaenadTestDriver()

	raenadTestStateMu.Lock()
	raenadTestDSNSeq++
	dsn := fmt.Sprintf("state-%d", raenadTestDSNSeq)
	raenadTestStates[dsn] = state
	raenadTestStateMu.Unlock()

	db, err := sql.Open(raenadTestDriverName, dsn)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		raenadTestStateMu.Lock()
		delete(raenadTestStates, dsn)
		raenadTestStateMu.Unlock()
	})
	return db
}

const raenadTestDriverName = "raenad_test_driver"

var (
	registerRaenadTestDriverOnce sync.Once
	raenadTestStateMu            sync.Mutex
	raenadTestDSNSeq             int
	raenadTestStates             = map[string]*raenadTestState{}
)

func registerRaenadTestDriver() {
	registerRaenadTestDriverOnce.Do(func() {
		sql.Register(raenadTestDriverName, raenadTestDriver{})
	})
}

type raenadTestDriver struct{}

func (raenadTestDriver) Open(name string) (driver.Conn, error) {
	raenadTestStateMu.Lock()
	state := raenadTestStates[name]
	raenadTestStateMu.Unlock()
	return &raenadTestConn{state: state}, nil
}

type raenadTestConn struct {
	state *raenadTestState
	tx    *raenadTestTx
}

func (raenadTestConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (raenadTestConn) Close() error                        { return nil }
func (c *raenadTestConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *raenadTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	if c.state == nil {
		tx := &raenadTestTx{conn: c}
		c.tx = tx
		return tx, nil
	}
	c.state.mu.Lock()
	work := c.state.clone()
	c.state.begins++
	c.state.mu.Unlock()
	tx := &raenadTestTx{conn: c, parent: c.state, work: work}
	c.tx = tx
	return tx, nil
}

func (c *raenadTestConn) activeState() *raenadTestState {
	if c.tx != nil && c.tx.work != nil {
		return c.tx.work
	}
	return c.state
}

func (c *raenadTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	state := c.state
	if state == nil {
		normalized := strings.ToLower(query)
		if strings.Contains(normalized, "count(*)") && strings.Contains(normalized, "from raenad.quote") {
			return &raenadTestRows{columns: []string{"count"}, values: [][]driver.Value{{int64(0)}}}, nil
		}
		if strings.Contains(normalized, "from raenad.quote") || strings.Contains(normalized, "from raenad.quote_line") || strings.Contains(normalized, "from raenad.quote_event") {
			return &raenadTestRows{columns: []string{"unused"}}, nil
		}
		return &raenadTestRows{columns: []string{"unused"}}, nil
	}
	state.recordQuery(query, args)

	normalized := strings.ToLower(query)
	active := c.activeState()
	switch {
	case strings.Contains(normalized, "pg_try_advisory_lock"):
		return active.advisoryLockRows(), nil
	case strings.Contains(normalized, "pg_advisory_unlock"):
		return active.advisoryUnlockRows(), nil
	case strings.Contains(normalized, "with due") && strings.Contains(normalized, "update mrsmith.hubspot_request"):
		return active.claimHubSpotRequestRows(args), nil
	case strings.Contains(normalized, "insert into raenad.quote_pdf_export"):
		return active.insertPDFExportRows(args), nil
	case strings.Contains(normalized, "insert into raenad.quote"):
		return active.insertQuoteRows(args), nil
	case strings.Contains(normalized, "insert into mrsmith.hubspot_request"):
		return active.upsertHubSpotRequest(query, args)
	case strings.Contains(normalized, "from mrsmith.runtime_config"):
		return state.runtimeConfigRows(args), nil
	case strings.Contains(normalized, "select exists") && strings.Contains(normalized, "from mrsmith.hubspot_request"):
		return active.liveHubSpotRequestRows(args), nil
	case strings.Contains(normalized, "status = 'locked'") && strings.Contains(normalized, "from mrsmith.hubspot_request"):
		return active.staleHubSpotRequestRows(args), nil
	case strings.Contains(normalized, "from mrsmith.hubspot_request"):
		return active.hubSpotRequestRows(args), nil
	case strings.Contains(normalized, "from loader.hubs_owner"):
		return active.ownerRows(args), nil
	case strings.Contains(normalized, "from loader.hubs_company"):
		return active.companyRows(args), nil
	case strings.Contains(normalized, "from loader.erp_metodi_pagamento") && strings.Contains(normalized, "rtrim(cod_pagamento) = $1"):
		return active.paymentMethodValidateRows(args), nil
	case strings.Contains(normalized, "from loader.erp_metodi_pagamento"):
		return active.paymentMethodRows(), nil
	case strings.Contains(normalized, "from loader.hubs_stages") && strings.Contains(normalized, "s.pipeline = $1 and s.id = $2"):
		return active.initialStageRows(args), nil
	case strings.Contains(normalized, "from loader.hubs_stages"):
		return active.stageRows(args), nil
	case strings.Contains(normalized, "from listino l"):
		return active.articleRows(args), nil
	case strings.Contains(normalized, "select raenad.resolve_iva_percent"):
		return active.vatRows(args), nil
	case strings.Contains(normalized, "count(*)") && strings.Contains(normalized, "from raenad.quote"):
		return active.quoteCountRows(), nil
	case strings.Contains(normalized, "from raenad.quote_pdf_export"):
		return active.pdfExportRows(query, args), nil
	case strings.Contains(normalized, "from raenad.quote_event"):
		return active.quoteEventRows(args), nil
	case strings.Contains(normalized, "from raenad.quote_line"):
		return active.quoteLineRows(args), nil
	case strings.Contains(normalized, "from raenad.quote"):
		return active.quoteRows(query, args), nil
	default:
		return &raenadTestRows{columns: []string{"unused"}}, nil
	}
}

func (c *raenadTestConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.state == nil {
		return driver.RowsAffected(0), nil
	}
	c.state.recordQuery(query, args)
	active := c.activeState()
	normalized := strings.ToLower(query)
	switch {
	case strings.Contains(normalized, "delete from raenad.quote_line"):
		return active.deleteQuoteLines(args), nil
	case strings.Contains(normalized, "update raenad.quote_pdf_export"):
		return active.updatePDFExportHubSpot(normalized, args), nil
	case strings.Contains(normalized, "insert into raenad.quote_line"):
		return active.insertQuoteLine(args)
	case strings.Contains(normalized, "insert into raenad.quote_event"):
		return active.insertQuoteEvent(args), nil
	case strings.Contains(normalized, "insert into mrsmith.hubspot_request_attempt"):
		return active.insertHubSpotRequestAttempt(args), nil
	case strings.Contains(normalized, "update mrsmith.hubspot_request"):
		return active.updateHubSpotRequest(query, args), nil
	case strings.Contains(normalized, "update raenad.quote") && strings.Contains(normalized, "hubspot_deal_id"):
		return active.markQuoteHubSpotSucceeded(args), nil
	case strings.Contains(normalized, "update raenad.quote") && strings.Contains(normalized, "hubspot_sync_status = 'pending'"):
		return active.markQuoteHubSpotPending(args), nil
	case strings.Contains(normalized, "update raenad.quote") && strings.Contains(normalized, "hubspot_sync_status = 'failed'"):
		return active.markQuoteHubSpotFailed(args), nil
	case strings.Contains(normalized, "update raenad.quote") && strings.Contains(normalized, "hubspot_pipeline_id = $2") && strings.Contains(normalized, "hubspot_dealstage_id = $4"):
		return active.updateQuoteHubSpotStageSnapshot(args), nil
	case strings.Contains(normalized, "update raenad.quote") && strings.Contains(normalized, "authoring_status = 'ready'"):
		return active.markQuoteReady(args), nil
	case strings.Contains(normalized, "update raenad.quote set"):
		return active.updateQuote(args), nil
	default:
		return nil, fmt.Errorf("unexpected exec: %s", query)
	}
}

var _ driver.QueryerContext = (*raenadTestConn)(nil)
var _ driver.ExecerContext = (*raenadTestConn)(nil)
var _ driver.ConnBeginTx = (*raenadTestConn)(nil)

type raenadTestTx struct {
	conn   *raenadTestConn
	parent *raenadTestState
	work   *raenadTestState
}

func (tx *raenadTestTx) Commit() error {
	if tx.parent != nil && tx.work != nil {
		tx.parent.mu.Lock()
		tx.parent.copyMutableFrom(tx.work)
		tx.parent.commits++
		tx.parent.mu.Unlock()
	}
	tx.conn.tx = nil
	return nil
}

func (tx *raenadTestTx) Rollback() error {
	if tx.parent != nil {
		tx.parent.mu.Lock()
		tx.parent.rollbacks++
		tx.parent.mu.Unlock()
	}
	tx.conn.tx = nil
	return nil
}

type raenadTestState struct {
	companies            []raenadTestCompany
	paymentMethods       []raenadTestPaymentMethod
	stages               []raenadTestStage
	articles             []raenadTestArticle
	config               map[string][]byte
	owners               []raenadTestOwner
	quotes               []raenadTestQuote
	lines                map[int64][]raenadTestQuoteLine
	pdfExports           []raenadTestPDFExport
	events               map[int64][]raenadTestQuoteEvent
	validVATCodes        map[string]string
	hubspotRequests      []raenadTestHubSpotRequest
	hubspotAttempts      []raenadTestHubSpotAttempt
	nextQuoteID          int64
	nextLineID           int64
	nextPDFExportID      int64
	nextEventID          int64
	nextHubSpotRequestID int64
	failLineInsert       bool
	failHubSpotEnqueue   bool

	mu        sync.Mutex
	queries   []raenadTestQuery
	begins    int
	commits   int
	rollbacks int
}

type raenadTestQuery struct {
	Query string
	Args  []driver.NamedValue
}

type raenadTestCompany struct {
	ID                  int64
	Name                *string
	VAT                 *string
	TaxCode             *string
	PEC                 *string
	Email               *string
	Domain              *string
	Address             *string
	ZIP                 *string
	City                *string
	Province            *string
	Country             *string
	Language            *string
	AlyanteIDAnagrafica *string
	NumeroAzienda       *string
}

type raenadTestPaymentMethod struct {
	Code        string
	Description string
	Selectable  bool
}

type raenadTestStage struct {
	ID            string
	Label         *string
	Pipeline      string
	DisplayOrder  *int
	PipelineLabel *string
}

type raenadTestArticle struct {
	Code        string
	DescDefault *string
	DescITA     *string
	DescENG     *string
	ExtITA      *string
	ExtENG      *string
	ExtDefault  *string
	UOM         *string
	Price       *string
}

type raenadTestOwner struct {
	ID       string
	Email    string
	Archived bool
}

type raenadTestQuote struct {
	ID                    int64
	QuoteNumber           string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	CreatedBy             string
	UpdatedBy             string
	AuthoringStatus       string
	HubSpotSyncStatus     string
	HubSpotSyncError      *string
	HubSpotSyncedAt       *time.Time
	HubSpotCompanyID      *string
	HubSpotContactID      *string
	HubSpotDealID         *string
	HubSpotPipelineID     *string
	HubSpotPipelineLabel  *string
	HubSpotDealstageID    *string
	HubSpotDealstageLabel *string
	Customer              customerSnapshotInput
	Contact               contactSnapshotInput
	DocumentDate          *string
	Payment               paymentSnapshot
	Description           *string
	InternalNotes         *string
	TotalNet              string
	TotalVAT              string
	TotalGross            string
	TotalPurchase         string
	TotalGain             string
}

type raenadTestQuoteLine struct {
	ID                 int64
	QuoteID            int64
	Position           int
	LineType           string
	ItemCode           *string
	ItemDescription    *string
	Description        *string
	UnitOfMeasure      *string
	Qta                *string
	UnitPrice          *string
	Discounts          *string
	CodIVA             *string
	IVAPercentSnapshot *string
	PurchaseUnitPrice  *string
	LineNet            *string
	LineVAT            *string
	LineGross          *string
	LinePurchase       *string
	LineGain           *string
}

type raenadTestQuoteEvent struct {
	ID           int64
	QuoteID      int64
	EventType    string
	ActorSubject string
	Payload      string
	CreatedAt    time.Time
}

type raenadTestPDFExport struct {
	ID                      int64
	QuoteID                 int64
	Revision                int
	Filename                string
	ContentType             string
	ChecksumSHA256          *string
	RenderPayload           string
	CreatedAt               time.Time
	CreatedBy               string
	HubSpotAttachmentStatus string
	HubSpotFileID           *string
	HubSpotNoteID           *string
	HubSpotDealID           *string
	HubSpotAttachedAt       *time.Time
	HubSpotError            *string
}

type raenadTestHubSpotRequest struct {
	ID            int64
	Status        string
	Operation     string
	EntityType    string
	EntityID      string
	DedupeKey     string
	Payload       string
	AttemptCount  int
	LastError     *string
	LockedAt      *time.Time
	LockedBy      *string
	Response      *string
	NextAttemptAt *time.Time
}

type raenadTestHubSpotAttempt struct {
	RequestID     int64
	AttemptNumber int
	Status        string
	Response      string
	ErrorDetail   string
}

func (s *raenadTestState) clone() *raenadTestState {
	cp := *s
	cp.mu = sync.Mutex{}
	cp.queries = nil
	cp.quotes = append([]raenadTestQuote(nil), s.quotes...)
	cp.lines = cloneLineMap(s.lines)
	cp.pdfExports = append([]raenadTestPDFExport(nil), s.pdfExports...)
	cp.events = cloneEventMap(s.events)
	cp.config = cloneBytesMap(s.config)
	cp.validVATCodes = cloneStringMap(s.validVATCodes)
	cp.hubspotRequests = append([]raenadTestHubSpotRequest(nil), s.hubspotRequests...)
	cp.hubspotAttempts = append([]raenadTestHubSpotAttempt(nil), s.hubspotAttempts...)
	return &cp
}

func (s *raenadTestState) copyMutableFrom(work *raenadTestState) {
	s.quotes = append([]raenadTestQuote(nil), work.quotes...)
	s.lines = cloneLineMap(work.lines)
	s.pdfExports = append([]raenadTestPDFExport(nil), work.pdfExports...)
	s.events = cloneEventMap(work.events)
	s.nextQuoteID = work.nextQuoteID
	s.nextLineID = work.nextLineID
	s.nextPDFExportID = work.nextPDFExportID
	s.nextEventID = work.nextEventID
	s.hubspotRequests = append([]raenadTestHubSpotRequest(nil), work.hubspotRequests...)
	s.hubspotAttempts = append([]raenadTestHubSpotAttempt(nil), work.hubspotAttempts...)
	s.nextHubSpotRequestID = work.nextHubSpotRequestID
}

func cloneLineMap(in map[int64][]raenadTestQuoteLine) map[int64][]raenadTestQuoteLine {
	out := map[int64][]raenadTestQuoteLine{}
	for id, lines := range in {
		out[id] = append([]raenadTestQuoteLine(nil), lines...)
	}
	return out
}

func cloneEventMap(in map[int64][]raenadTestQuoteEvent) map[int64][]raenadTestQuoteEvent {
	out := map[int64][]raenadTestQuoteEvent{}
	for id, events := range in {
		out[id] = append([]raenadTestQuoteEvent(nil), events...)
	}
	return out
}

func cloneBytesMap(in map[string][]byte) map[string][]byte {
	out := map[string][]byte{}
	for k, v := range in {
		out[k] = append([]byte(nil), v...)
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (s *raenadTestState) recordQuery(query string, args []driver.NamedValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]driver.NamedValue, len(args))
	copy(copied, args)
	s.queries = append(s.queries, raenadTestQuery{Query: query, Args: copied})
}

func (s *raenadTestState) allQueries() []raenadTestQuery {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]raenadTestQuery, len(s.queries))
	copy(copied, s.queries)
	return copied
}

func (s *raenadTestState) runtimeConfigRows(args []driver.NamedValue) driver.Rows {
	namespace, _ := args[0].Value.(string)
	key, _ := args[1].Value.(string)
	raw, ok := s.config[namespace+"."+key]
	if !ok {
		return &raenadTestRows{columns: []string{"value"}}
	}
	return &raenadTestRows{
		columns: []string{"value"},
		values:  [][]driver.Value{{raw}},
	}
}

func (s *raenadTestState) advisoryLockRows() driver.Rows {
	return &raenadTestRows{
		columns: []string{"pg_try_advisory_lock"},
		values:  [][]driver.Value{{true}},
	}
}

func (s *raenadTestState) advisoryUnlockRows() driver.Rows {
	return &raenadTestRows{
		columns: []string{"pg_advisory_unlock"},
		values:  [][]driver.Value{{true}},
	}
}

func (s *raenadTestState) ownerRows(args []driver.NamedValue) driver.Rows {
	email := strings.ToLower(strings.TrimSpace(fmt.Sprint(args[0].Value)))
	for _, owner := range s.owners {
		if owner.Archived {
			continue
		}
		if strings.ToLower(strings.TrimSpace(owner.Email)) == email {
			return &raenadTestRows{
				columns: []string{"id"},
				values:  [][]driver.Value{{owner.ID}},
			}
		}
	}
	return &raenadTestRows{columns: []string{"id"}}
}

func (s *raenadTestState) upsertHubSpotRequest(query string, args []driver.NamedValue) (driver.Rows, error) {
	if s.failHubSpotEnqueue {
		return nil, fmt.Errorf("forced hubspot enqueue failure")
	}
	normalized := strings.ToLower(query)
	operation := stringFromNamed(args[0])
	entityType := stringFromNamed(args[1])
	entityID := stringFromNamed(args[2])
	dedupeKey := stringFromNamed(args[3])
	payload := stringFromNamed(args[4])

	matchesStatus := func(status string) bool {
		switch {
		case strings.Contains(normalized, "do nothing"):
			return false
		case strings.Contains(normalized, "status <> 'locked'"):
			return status != hubSpotRequestStatusLocked
		case strings.Contains(normalized, "status in ('pending', 'failed', 'succeeded')"):
			return status == hubSpotRequestStatusPending || status == hubSpotRequestStatusFailed || status == hubSpotRequestStatusSucceeded
		default:
			return false
		}
	}

	for i := range s.hubspotRequests {
		req := &s.hubspotRequests[i]
		if req.DedupeKey != dedupeKey {
			continue
		}
		if !matchesStatus(req.Status) {
			return &raenadTestRows{columns: []string{"id", "status"}}, nil
		}
		req.Operation = operation
		req.EntityType = entityType
		req.EntityID = entityID
		req.Payload = payload
		req.Status = hubSpotRequestStatusPending
		req.AttemptCount = 0
		req.LastError = nil
		req.LockedAt = nil
		req.LockedBy = nil
		req.Response = nil
		now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
		req.NextAttemptAt = &now
		return &raenadTestRows{
			columns: []string{"id", "status"},
			values:  [][]driver.Value{{req.ID, req.Status}},
		}, nil
	}

	s.nextHubSpotRequestID++
	if s.nextHubSpotRequestID == 1 {
		s.nextHubSpotRequestID = 1001
	}
	req := raenadTestHubSpotRequest{
		ID:           s.nextHubSpotRequestID,
		Status:       hubSpotRequestStatusPending,
		Operation:    operation,
		EntityType:   entityType,
		EntityID:     entityID,
		DedupeKey:    dedupeKey,
		Payload:      payload,
		AttemptCount: 0,
	}
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	req.NextAttemptAt = &now
	s.hubspotRequests = append(s.hubspotRequests, req)
	return &raenadTestRows{
		columns: []string{"id", "status"},
		values:  [][]driver.Value{{req.ID, req.Status}},
	}, nil
}

func (s *raenadTestState) claimHubSpotRequestRows(args []driver.NamedValue) driver.Rows {
	limit := intFromNamed(args[0])
	if limit <= 0 {
		limit = 10
	}
	workerID := stringFromNamed(args[1])
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	values := make([][]driver.Value, 0)
	for i := range s.hubspotRequests {
		if len(values) >= limit {
			break
		}
		req := &s.hubspotRequests[i]
		if !isSupportedHubSpotRequest(req.EntityType, req.Operation) {
			continue
		}
		if req.Status != hubSpotRequestStatusPending {
			continue
		}
		if req.NextAttemptAt != nil && req.NextAttemptAt.After(now) {
			continue
		}
		req.Status = hubSpotRequestStatusLocked
		req.LockedAt = &now
		req.LockedBy = &workerID
		req.AttemptCount++
		values = append(values, []driver.Value{
			req.ID,
			req.Operation,
			req.EntityID,
			req.DedupeKey,
			[]byte(req.Payload),
			int64(req.AttemptCount),
		})
	}
	return &raenadTestRows{
		columns: []string{"id", "operation", "entity_id", "dedupe_key", "payload", "attempt_count"},
		values:  values,
	}
}

func (s *raenadTestState) staleHubSpotRequestRows(args []driver.NamedValue) driver.Rows {
	timeoutSeconds := intFromNamed(args[0])
	cutoff := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC).Add(-time.Duration(timeoutSeconds) * time.Second)
	values := make([][]driver.Value, 0)
	for _, req := range s.hubspotRequests {
		if !isSupportedHubSpotRequest(req.EntityType, req.Operation) {
			continue
		}
		if req.Status != hubSpotRequestStatusLocked || req.LockedAt == nil || !req.LockedAt.Before(cutoff) {
			continue
		}
		values = append(values, []driver.Value{req.ID, req.EntityID, req.Operation, []byte(req.Payload), int64(req.AttemptCount)})
	}
	return &raenadTestRows{
		columns: []string{"id", "entity_id", "operation", "payload", "attempt_count"},
		values:  values,
	}
}

func (s *raenadTestState) liveHubSpotRequestRows(args []driver.NamedValue) driver.Rows {
	entityID := stringFromNamed(args[0])
	exists := false
	for _, req := range s.hubspotRequests {
		if req.EntityType == hubSpotQueueEntityQuote &&
			req.EntityID == entityID &&
			(req.Operation == hubSpotOperationCreateDeal || req.Operation == hubSpotOperationUpdateDeal) &&
			(req.Status == hubSpotRequestStatusPending || req.Status == hubSpotRequestStatusLocked) {
			exists = true
			break
		}
	}
	return &raenadTestRows{
		columns: []string{"exists"},
		values:  [][]driver.Value{{exists}},
	}
}

func isSupportedHubSpotRequest(entityType, operation string) bool {
	return (entityType == hubSpotQueueEntityQuote &&
		(operation == hubSpotOperationCreateDeal || operation == hubSpotOperationUpdateDeal)) ||
		(entityType == hubSpotQueueEntityQuotePDFExport && operation == hubSpotOperationAttachPDF)
}

func (s *raenadTestState) hubSpotRequestRows(args []driver.NamedValue) driver.Rows {
	dedupeKey := stringFromNamed(args[0])
	for _, req := range s.hubspotRequests {
		if req.DedupeKey == dedupeKey {
			return &raenadTestRows{
				columns: []string{"id", "status"},
				values:  [][]driver.Value{{req.ID, req.Status}},
			}
		}
	}
	return &raenadTestRows{columns: []string{"id", "status"}}
}

func (s *raenadTestState) companyRows(args []driver.NamedValue) driver.Rows {
	includeWithoutNumeroAzienda, _ := args[0].Value.(bool)
	search, _ := args[1].Value.(string)
	limit, _ := args[2].Value.(int64)
	if limit == 0 {
		limit, _ = int64FromAny(args[2].Value)
	}

	filtered := make([]raenadTestCompany, 0)
	for _, company := range s.companies {
		if !includeWithoutNumeroAzienda && strings.TrimSpace(stringValue(company.NumeroAzienda)) == "" {
			continue
		}
		if search != "" && !companyMatchesSearch(company, search) {
			continue
		}
		filtered = append(filtered, company)
	}
	sort.Slice(filtered, func(i, j int) bool {
		leftHasNumero := strings.TrimSpace(stringValue(filtered[i].NumeroAzienda)) != ""
		rightHasNumero := strings.TrimSpace(stringValue(filtered[j].NumeroAzienda)) != ""
		if leftHasNumero != rightHasNumero {
			return leftHasNumero
		}
		leftName := strings.TrimSpace(stringValue(filtered[i].Name))
		rightName := strings.TrimSpace(stringValue(filtered[j].Name))
		if leftName != rightName {
			return leftName < rightName
		}
		return filtered[i].ID < filtered[j].ID
	})
	if limit > 0 && int64(len(filtered)) > limit {
		filtered = filtered[:limit]
	}

	values := make([][]driver.Value, 0, len(filtered))
	for _, company := range filtered {
		values = append(values, []driver.Value{
			company.ID,
			driverValue(company.Name),
			driverValue(company.VAT),
			driverValue(company.TaxCode),
			driverValue(company.PEC),
			driverValue(company.Email),
			driverValue(company.Domain),
			driverValue(company.Address),
			driverValue(company.ZIP),
			driverValue(company.City),
			driverValue(company.Province),
			driverValue(company.Country),
			driverValue(company.Language),
			driverValue(company.AlyanteIDAnagrafica),
			driverValue(company.NumeroAzienda),
		})
	}
	return &raenadTestRows{
		columns: []string{
			"id",
			"name",
			"partita_iva",
			"codice_fiscale",
			"pec",
			"email",
			"domain",
			"address",
			"zip",
			"city",
			"provincia_di_fatturazione",
			"country",
			"lingua",
			"alyante_id_anagrafica",
			"numero_azienda",
		},
		values: values,
	}
}

func (s *raenadTestState) paymentMethodRows() driver.Rows {
	filtered := make([]raenadTestPaymentMethod, 0)
	for _, method := range s.paymentMethods {
		filtered = append(filtered, method)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Description != filtered[j].Description {
			return filtered[i].Description < filtered[j].Description
		}
		return strings.TrimSpace(filtered[i].Code) < strings.TrimSpace(filtered[j].Code)
	})

	values := make([][]driver.Value, 0, len(filtered))
	for _, method := range filtered {
		values = append(values, []driver.Value{method.Code, method.Description})
	}
	return &raenadTestRows{
		columns: []string{"cod_pagamento", "desc_pagamento"},
		values:  values,
	}
}

func (s *raenadTestState) stageRows(args []driver.NamedValue) driver.Rows {
	pipelineID, _ := args[0].Value.(string)
	filtered := make([]raenadTestStage, 0)
	for _, stage := range s.stages {
		if stage.Pipeline == pipelineID {
			filtered = append(filtered, stage)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		leftOrder := orderValue(filtered[i].DisplayOrder)
		rightOrder := orderValue(filtered[j].DisplayOrder)
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		leftLabel := strings.TrimSpace(stringValue(filtered[i].Label))
		rightLabel := strings.TrimSpace(stringValue(filtered[j].Label))
		if leftLabel != rightLabel {
			return leftLabel < rightLabel
		}
		return filtered[i].ID < filtered[j].ID
	})

	values := make([][]driver.Value, 0, len(filtered))
	for _, stage := range filtered {
		values = append(values, []driver.Value{
			stage.ID,
			driverValue(stage.Label),
			stage.Pipeline,
			driverIntValue(stage.DisplayOrder),
			driverValue(stage.PipelineLabel),
		})
	}
	return &raenadTestRows{
		columns: []string{"id", "label", "pipeline", "display_order", "pipeline_label"},
		values:  values,
	}
}

func (s *raenadTestState) articleRows(args []driver.NamedValue) driver.Rows {
	search, _ := args[0].Value.(string)
	limit, _ := int64FromAny(args[2].Value)

	filtered := make([]raenadTestArticle, 0)
	for _, article := range s.articles {
		if search != "" && !articleMatchesSearch(article, search) {
			continue
		}
		filtered = append(filtered, article)
	}
	if limit > 0 && int64(len(filtered)) > limit {
		filtered = filtered[:limit]
	}

	values := make([][]driver.Value, 0, len(filtered))
	for _, article := range filtered {
		values = append(values, []driver.Value{
			article.Code,
			driverValue(article.DescDefault),
			driverValue(article.DescITA),
			driverValue(article.DescENG),
			driverValue(article.ExtITA),
			driverValue(article.ExtENG),
			driverValue(article.ExtDefault),
			driverValue(article.UOM),
			driverValue(article.Price),
		})
	}
	return &raenadTestRows{
		columns: []string{
			"COD_ART",
			"DESCART_DEF",
			"DESCART_ITA",
			"DESCART_ENG",
			"DESCARTEST_ITA",
			"DESCARTEST_ENG",
			"DESCARTEST_DEF",
			"MG66_UM1",
			"LI10_PREZZO",
		},
		values: values,
	}
}

func (s *raenadTestState) paymentMethodValidateRows(args []driver.NamedValue) driver.Rows {
	code := strings.TrimSpace(fmt.Sprint(args[0].Value))
	for _, method := range s.paymentMethods {
		if strings.TrimSpace(method.Code) == code {
			return &raenadTestRows{
				columns: []string{"cod_pagamento", "desc_pagamento"},
				values:  [][]driver.Value{{method.Code, method.Description}},
			}
		}
	}
	return &raenadTestRows{columns: []string{"cod_pagamento", "desc_pagamento"}}
}

func (s *raenadTestState) initialStageRows(args []driver.NamedValue) driver.Rows {
	pipelineID := strings.TrimSpace(fmt.Sprint(args[0].Value))
	stageID := strings.TrimSpace(fmt.Sprint(args[1].Value))
	for _, stage := range s.stages {
		if stage.Pipeline == pipelineID && stage.ID == stageID {
			return &raenadTestRows{
				columns: []string{"stage_label", "pipeline_label"},
				values:  [][]driver.Value{{driverValue(stage.Label), driverValue(stage.PipelineLabel)}},
			}
		}
	}
	return &raenadTestRows{columns: []string{"stage_label", "pipeline_label"}}
}

func (s *raenadTestState) vatRows(args []driver.NamedValue) driver.Rows {
	code := strings.TrimSpace(fmt.Sprint(args[0].Value))
	valid := s.validVATCodes
	if valid == nil {
		valid = map[string]string{"22": "22.0000"}
	}
	value, ok := valid[code]
	if !ok {
		return &raenadTestRows{columns: []string{"resolve_iva_percent"}}
	}
	return &raenadTestRows{
		columns: []string{"resolve_iva_percent"},
		values:  [][]driver.Value{{value}},
	}
}

func (s *raenadTestState) quoteCountRows() driver.Rows {
	return &raenadTestRows{
		columns: []string{"count"},
		values:  [][]driver.Value{{int64(len(s.quotes))}},
	}
}

func (s *raenadTestState) quoteRows(query string, args []driver.NamedValue) driver.Rows {
	normalized := strings.ToLower(query)
	values := make([][]driver.Value, 0)
	if strings.Contains(normalized, "where id = $1") {
		id, _ := int64FromAny(args[0].Value)
		for _, quote := range s.quotes {
			if quote.ID == id {
				values = append(values, quoteScanValues(quote))
				break
			}
		}
	} else {
		quotes := append([]raenadTestQuote(nil), s.quotes...)
		sort.Slice(quotes, func(i, j int) bool {
			if !quotes[i].UpdatedAt.Equal(quotes[j].UpdatedAt) {
				return quotes[i].UpdatedAt.After(quotes[j].UpdatedAt)
			}
			return quotes[i].ID > quotes[j].ID
		})
		for _, quote := range quotes {
			if strings.Contains(normalized, "hubspot_sync_status = 'pending'") && quote.HubSpotSyncStatus != hubSpotSyncStatusPending {
				continue
			}
			values = append(values, quoteScanValues(quote))
		}
	}
	return &raenadTestRows{columns: quoteTestColumns(), values: values}
}

func (s *raenadTestState) quoteLineRows(args []driver.NamedValue) driver.Rows {
	quoteID, _ := int64FromAny(args[0].Value)
	lines := append([]raenadTestQuoteLine(nil), s.lines[quoteID]...)
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].Position != lines[j].Position {
			return lines[i].Position < lines[j].Position
		}
		return lines[i].ID < lines[j].ID
	})
	values := make([][]driver.Value, 0, len(lines))
	for _, line := range lines {
		values = append(values, quoteLineScanValues(line))
	}
	return &raenadTestRows{columns: quoteLineTestColumns(), values: values}
}

func (s *raenadTestState) quoteEventRows(args []driver.NamedValue) driver.Rows {
	quoteID, _ := int64FromAny(args[0].Value)
	events := append([]raenadTestQuoteEvent(nil), s.events[quoteID]...)
	sort.Slice(events, func(i, j int) bool {
		if !events[i].CreatedAt.Equal(events[j].CreatedAt) {
			return events[i].CreatedAt.Before(events[j].CreatedAt)
		}
		return events[i].ID < events[j].ID
	})
	values := make([][]driver.Value, 0, len(events))
	for _, event := range events {
		values = append(values, []driver.Value{
			event.ID,
			event.QuoteID,
			event.EventType,
			event.ActorSubject,
			[]byte(event.Payload),
			event.CreatedAt,
		})
	}
	return &raenadTestRows{
		columns: []string{"id", "quote_id", "event_type", "actor_subject", "payload", "created_at"},
		values:  values,
	}
}

func (s *raenadTestState) pdfExportRows(query string, args []driver.NamedValue) driver.Rows {
	normalized := strings.ToLower(query)
	quoteID, _ := int64FromAny(args[0].Value)
	values := make([][]driver.Value, 0)
	if strings.Contains(normalized, "and id = $2") {
		exportID, _ := int64FromAny(args[1].Value)
		for _, export := range s.pdfExports {
			if export.QuoteID == quoteID && export.ID == exportID {
				values = append(values, pdfExportScanValues(export))
				break
			}
		}
	} else {
		exports := make([]raenadTestPDFExport, 0)
		for _, export := range s.pdfExports {
			if export.QuoteID == quoteID {
				exports = append(exports, export)
			}
		}
		sort.Slice(exports, func(i, j int) bool {
			if exports[i].Revision != exports[j].Revision {
				return exports[i].Revision > exports[j].Revision
			}
			return exports[i].ID > exports[j].ID
		})
		if strings.Contains(normalized, "limit 1") && len(exports) > 1 {
			exports = exports[:1]
		}
		for _, export := range exports {
			values = append(values, pdfExportScanValues(export))
		}
	}
	return &raenadTestRows{columns: pdfExportTestColumns(), values: values}
}

func (s *raenadTestState) insertQuoteRows(args []driver.NamedValue) driver.Rows {
	s.nextQuoteID++
	if s.nextQuoteID == 1 {
		s.nextQuoteID = 101
	}
	now := time.Date(2026, 6, 14, 12, 0, int(s.nextQuoteID%60), 0, time.UTC)
	quote := raenadTestQuote{
		ID:                    s.nextQuoteID,
		QuoteNumber:           fmt.Sprintf("AE-%d/2026", s.nextQuoteID),
		CreatedAt:             now,
		UpdatedAt:             now,
		CreatedBy:             stringFromNamed(args[0]),
		UpdatedBy:             stringFromNamed(args[1]),
		AuthoringStatus:       authoringStatusDraft,
		HubSpotSyncStatus:     hubSpotSyncStatusPending,
		HubSpotCompanyID:      ptrFromNamed(args[2]),
		HubSpotContactID:      ptrFromNamed(args[3]),
		HubSpotPipelineID:     ptrFromNamed(args[4]),
		HubSpotPipelineLabel:  ptrFromNamed(args[5]),
		HubSpotDealstageID:    ptrFromNamed(args[6]),
		HubSpotDealstageLabel: ptrFromNamed(args[7]),
		Customer: customerSnapshotInput{
			Name:                  ptrFromNamed(args[8]),
			VAT:                   ptrFromNamed(args[9]),
			TaxCode:               ptrFromNamed(args[10]),
			PEC:                   ptrFromNamed(args[11]),
			Email:                 ptrFromNamed(args[12]),
			Address:               ptrFromNamed(args[13]),
			ZIP:                   ptrFromNamed(args[14]),
			City:                  ptrFromNamed(args[15]),
			Province:              ptrFromNamed(args[16]),
			Country:               ptrFromNamed(args[17]),
			Language:              ptrFromNamed(args[18]),
			NumeroAziendaSnapshot: ptrFromNamed(args[19]),
		},
		Contact: contactSnapshotInput{
			FirstName: ptrFromNamed(args[20]),
			LastName:  ptrFromNamed(args[21]),
			FullName:  ptrFromNamed(args[22]),
			Email:     ptrFromNamed(args[23]),
			Role:      ptrFromNamed(args[24]),
		},
		DocumentDate: ptrFromNamed(args[25]),
		Payment: paymentSnapshot{
			MethodCode:  ptrFromNamed(args[26]),
			MethodLabel: ptrFromNamed(args[27]),
			BankDetails: ptrFromNamed(args[28]),
		},
		Description:   ptrFromNamed(args[29]),
		InternalNotes: ptrFromNamed(args[30]),
		TotalNet:      "0.0000",
		TotalVAT:      "0.0000",
		TotalGross:    "0.0000",
		TotalPurchase: "0.0000",
		TotalGain:     "0.0000",
	}
	s.quotes = append(s.quotes, quote)
	return &raenadTestRows{
		columns: []string{"id"},
		values:  [][]driver.Value{{quote.ID}},
	}
}

func (s *raenadTestState) deleteQuoteLines(args []driver.NamedValue) driver.Result {
	quoteID, _ := int64FromAny(args[0].Value)
	deleted := len(s.lines[quoteID])
	delete(s.lines, quoteID)
	s.recalculateQuoteTotals(quoteID)
	return driver.RowsAffected(deleted)
}

func (s *raenadTestState) insertQuoteLine(args []driver.NamedValue) (driver.Result, error) {
	if s.failLineInsert {
		return nil, fmt.Errorf("forced line insert failure")
	}
	if s.lines == nil {
		s.lines = map[int64][]raenadTestQuoteLine{}
	}
	s.nextLineID++
	quoteID, _ := int64FromAny(args[0].Value)
	line := raenadTestQuoteLine{
		ID:                 s.nextLineID,
		QuoteID:            quoteID,
		Position:           intFromNamed(args[1]),
		LineType:           stringFromNamed(args[2]),
		ItemCode:           ptrFromNamed(args[3]),
		ItemDescription:    ptrFromNamed(args[4]),
		Description:        ptrFromNamed(args[5]),
		UnitOfMeasure:      ptrFromNamed(args[6]),
		Qta:                ptrFromNamed(args[7]),
		UnitPrice:          ptrFromNamed(args[8]),
		Discounts:          ptrFromNamed(args[9]),
		CodIVA:             ptrFromNamed(args[10]),
		IVAPercentSnapshot: ptrFromNamed(args[11]),
		PurchaseUnitPrice:  ptrFromNamed(args[12]),
	}
	line.applyTotals()
	s.lines[quoteID] = append(s.lines[quoteID], line)
	s.recalculateQuoteTotals(quoteID)
	return driver.RowsAffected(1), nil
}

func (s *raenadTestState) insertQuoteEvent(args []driver.NamedValue) driver.Result {
	if s.events == nil {
		s.events = map[int64][]raenadTestQuoteEvent{}
	}
	s.nextEventID++
	quoteID, _ := int64FromAny(args[0].Value)
	event := raenadTestQuoteEvent{
		ID:           s.nextEventID,
		QuoteID:      quoteID,
		EventType:    stringFromNamed(args[1]),
		ActorSubject: stringFromNamed(args[2]),
		Payload:      stringFromNamed(args[3]),
		CreatedAt:    time.Date(2026, 6, 14, 13, 0, int(s.nextEventID%60), 0, time.UTC),
	}
	s.events[quoteID] = append(s.events[quoteID], event)
	return driver.RowsAffected(1)
}

func (s *raenadTestState) insertPDFExportRows(args []driver.NamedValue) driver.Rows {
	s.nextPDFExportID++
	if s.nextPDFExportID == 1 {
		s.nextPDFExportID = 501
	}
	quoteID, _ := int64FromAny(args[0].Value)
	export := raenadTestPDFExport{
		ID:                      s.nextPDFExportID,
		QuoteID:                 quoteID,
		Revision:                intFromNamed(args[1]),
		Filename:                stringFromNamed(args[2]),
		ContentType:             stringFromNamed(args[3]),
		ChecksumSHA256:          ptrFromNamed(args[4]),
		RenderPayload:           stringFromNamed(args[5]),
		CreatedBy:               stringFromNamed(args[6]),
		CreatedAt:               time.Date(2026, 6, 14, 15, 0, int(s.nextPDFExportID%60), 0, time.UTC),
		HubSpotAttachmentStatus: "pending",
	}
	s.pdfExports = append(s.pdfExports, export)
	return &raenadTestRows{
		columns: pdfExportTestColumns(),
		values:  [][]driver.Value{pdfExportScanValues(export)},
	}
}

func (s *raenadTestState) insertHubSpotRequestAttempt(args []driver.NamedValue) driver.Result {
	requestID, _ := int64FromAny(args[0].Value)
	s.hubspotAttempts = append(s.hubspotAttempts, raenadTestHubSpotAttempt{
		RequestID:     requestID,
		AttemptNumber: intFromNamed(args[1]),
		Status:        stringFromNamed(args[2]),
		Response:      stringFromNamed(args[3]),
		ErrorDetail:   stringFromNamed(args[4]),
	})
	return driver.RowsAffected(1)
}

func (s *raenadTestState) updateHubSpotRequest(query string, args []driver.NamedValue) driver.Result {
	normalized := strings.ToLower(query)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	if strings.Contains(normalized, "status = 'pending'") {
		msg := stringFromNamed(args[0])
		id, _ := int64FromAny(args[1].Value)
		if strings.Contains(normalized, "next_attempt_at = now() +") {
			id, _ = int64FromAny(args[2].Value)
			delay := intFromNamed(args[1])
			next := now.Add(time.Duration(delay) * time.Second)
			for i := range s.hubspotRequests {
				if s.hubspotRequests[i].ID == id {
					s.hubspotRequests[i].NextAttemptAt = &next
				}
			}
		}
		for i := range s.hubspotRequests {
			req := &s.hubspotRequests[i]
			if req.ID != id {
				continue
			}
			req.Status = hubSpotRequestStatusPending
			req.LockedAt = nil
			req.LockedBy = nil
			req.LastError = &msg
			if !strings.Contains(normalized, "next_attempt_at = now() +") {
				req.NextAttemptAt = &now
			}
			return driver.RowsAffected(1)
		}
		return driver.RowsAffected(0)
	}
	idArg := 1
	if strings.Contains(normalized, "status = 'succeeded'") {
		response := stringFromNamed(args[0])
		id, _ := int64FromAny(args[idArg].Value)
		for i := range s.hubspotRequests {
			req := &s.hubspotRequests[i]
			if req.ID != id {
				continue
			}
			req.Status = hubSpotRequestStatusSucceeded
			req.LockedAt = nil
			req.LockedBy = nil
			req.LastError = nil
			req.Response = &response
			return driver.RowsAffected(1)
		}
		return driver.RowsAffected(0)
	}
	if strings.Contains(normalized, "status = 'dead'") {
		msg := stringFromNamed(args[0])
		id, _ := int64FromAny(args[1].Value)
		for i := range s.hubspotRequests {
			req := &s.hubspotRequests[i]
			if req.ID != id {
				continue
			}
			req.Status = hubSpotRequestStatusDead
			req.LockedAt = nil
			req.LockedBy = nil
			req.LastError = &msg
			return driver.RowsAffected(1)
		}
		return driver.RowsAffected(0)
	}
	return driver.RowsAffected(0)
}

func (s *raenadTestState) markQuoteHubSpotSucceeded(args []driver.NamedValue) driver.Result {
	dealID := stringFromNamed(args[0])
	quoteID, _ := int64FromAny(args[1].Value)
	now := time.Date(2026, 6, 14, 12, 5, 0, 0, time.UTC)
	for i := range s.quotes {
		if s.quotes[i].ID == quoteID {
			s.quotes[i].HubSpotDealID = &dealID
			s.quotes[i].HubSpotSyncStatus = hubSpotSyncStatusSucceeded
			s.quotes[i].HubSpotSyncError = nil
			s.quotes[i].HubSpotSyncedAt = &now
			return driver.RowsAffected(1)
		}
	}
	return driver.RowsAffected(0)
}

func (s *raenadTestState) markQuoteHubSpotPending(args []driver.NamedValue) driver.Result {
	quoteID, _ := int64FromAny(args[0].Value)
	for i := range s.quotes {
		if s.quotes[i].ID == quoteID {
			s.quotes[i].HubSpotSyncStatus = hubSpotSyncStatusPending
			s.quotes[i].HubSpotSyncError = nil
			s.quotes[i].HubSpotSyncedAt = nil
			return driver.RowsAffected(1)
		}
	}
	return driver.RowsAffected(0)
}

func (s *raenadTestState) markQuoteHubSpotFailed(args []driver.NamedValue) driver.Result {
	msg := stringFromNamed(args[0])
	quoteID, _ := int64FromAny(args[1].Value)
	for i := range s.quotes {
		if s.quotes[i].ID == quoteID {
			s.quotes[i].HubSpotSyncStatus = hubSpotSyncStatusFailed
			s.quotes[i].HubSpotSyncError = &msg
			s.quotes[i].HubSpotSyncedAt = nil
			return driver.RowsAffected(1)
		}
	}
	return driver.RowsAffected(0)
}

func (s *raenadTestState) updatePDFExportHubSpot(query string, args []driver.NamedValue) driver.Result {
	normalized := strings.ToLower(query)
	now := time.Date(2026, 6, 14, 12, 7, 0, 0, time.UTC)
	if strings.Contains(normalized, "hubspot_attachment_status = 'attached'") {
		fileID := stringFromNamed(args[0])
		noteID := stringFromNamed(args[1])
		dealID := stringFromNamed(args[2])
		quoteID, _ := int64FromAny(args[3].Value)
		exportID, _ := int64FromAny(args[4].Value)
		for i := range s.pdfExports {
			if s.pdfExports[i].QuoteID == quoteID && s.pdfExports[i].ID == exportID {
				s.pdfExports[i].HubSpotAttachmentStatus = "attached"
				s.pdfExports[i].HubSpotFileID = &fileID
				s.pdfExports[i].HubSpotNoteID = &noteID
				s.pdfExports[i].HubSpotDealID = &dealID
				s.pdfExports[i].HubSpotAttachedAt = &now
				s.pdfExports[i].HubSpotError = nil
				return driver.RowsAffected(1)
			}
		}
		return driver.RowsAffected(0)
	}
	if strings.Contains(normalized, "hubspot_attachment_status = 'failed'") {
		msg := stringFromNamed(args[0])
		quoteID, _ := int64FromAny(args[1].Value)
		exportID, _ := int64FromAny(args[2].Value)
		for i := range s.pdfExports {
			if s.pdfExports[i].QuoteID == quoteID && s.pdfExports[i].ID == exportID {
				s.pdfExports[i].HubSpotAttachmentStatus = "failed"
				s.pdfExports[i].HubSpotAttachedAt = nil
				s.pdfExports[i].HubSpotError = &msg
				return driver.RowsAffected(1)
			}
		}
		return driver.RowsAffected(0)
	}
	return driver.RowsAffected(0)
}

func (s *raenadTestState) updateQuote(args []driver.NamedValue) driver.Result {
	relevantChanged, _ := args[27].Value.(bool)
	quoteID, _ := int64FromAny(args[28].Value)
	for i := range s.quotes {
		if s.quotes[i].ID != quoteID {
			continue
		}
		q := &s.quotes[i]
		q.UpdatedBy = stringFromNamed(args[0])
		q.AuthoringStatus = stringFromNamed(args[1])
		q.HubSpotCompanyID = ptrFromNamed(args[2])
		q.HubSpotContactID = ptrFromNamed(args[3])
		q.Customer = customerSnapshotInput{
			Name:                  ptrFromNamed(args[4]),
			VAT:                   ptrFromNamed(args[5]),
			TaxCode:               ptrFromNamed(args[6]),
			PEC:                   ptrFromNamed(args[7]),
			Email:                 ptrFromNamed(args[8]),
			Address:               ptrFromNamed(args[9]),
			ZIP:                   ptrFromNamed(args[10]),
			City:                  ptrFromNamed(args[11]),
			Province:              ptrFromNamed(args[12]),
			Country:               ptrFromNamed(args[13]),
			Language:              ptrFromNamed(args[14]),
			NumeroAziendaSnapshot: ptrFromNamed(args[15]),
		}
		q.Contact = contactSnapshotInput{
			FirstName: ptrFromNamed(args[16]),
			LastName:  ptrFromNamed(args[17]),
			FullName:  ptrFromNamed(args[18]),
			Email:     ptrFromNamed(args[19]),
			Role:      ptrFromNamed(args[20]),
		}
		q.DocumentDate = ptrFromNamed(args[21])
		q.Payment.MethodCode = ptrFromNamed(args[22])
		q.Payment.MethodLabel = ptrFromNamed(args[23])
		q.Payment.BankDetails = ptrFromNamed(args[24])
		q.Description = ptrFromNamed(args[25])
		q.InternalNotes = ptrFromNamed(args[26])
		if relevantChanged {
			q.HubSpotSyncStatus = hubSpotSyncStatusPending
			q.HubSpotSyncError = nil
			q.HubSpotSyncedAt = nil
		}
		q.UpdatedAt = q.UpdatedAt.Add(time.Minute)
		return driver.RowsAffected(1)
	}
	return driver.RowsAffected(0)
}

func (s *raenadTestState) updateQuoteHubSpotStageSnapshot(args []driver.NamedValue) driver.Result {
	quoteID, _ := int64FromAny(args[5].Value)
	for i := range s.quotes {
		if s.quotes[i].ID != quoteID {
			continue
		}
		q := &s.quotes[i]
		q.UpdatedBy = stringFromNamed(args[0])
		q.HubSpotPipelineID = ptrFromNamed(args[1])
		q.HubSpotPipelineLabel = ptrFromNamed(args[2])
		q.HubSpotDealstageID = ptrFromNamed(args[3])
		q.HubSpotDealstageLabel = ptrFromNamed(args[4])
		q.UpdatedAt = q.UpdatedAt.Add(time.Minute)
		return driver.RowsAffected(1)
	}
	return driver.RowsAffected(0)
}

func (s *raenadTestState) markQuoteReady(args []driver.NamedValue) driver.Result {
	quoteID, _ := int64FromAny(args[3].Value)
	for i := range s.quotes {
		if s.quotes[i].ID == quoteID {
			s.quotes[i].AuthoringStatus = authoringStatusReady
			s.quotes[i].UpdatedBy = stringFromNamed(args[0])
			s.quotes[i].Payment.MethodCode = ptrFromNamed(args[1])
			s.quotes[i].Payment.MethodLabel = ptrFromNamed(args[2])
			s.quotes[i].UpdatedAt = s.quotes[i].UpdatedAt.Add(time.Minute)
			return driver.RowsAffected(1)
		}
	}
	return driver.RowsAffected(0)
}

func (s *raenadTestState) recalculateQuoteTotals(quoteID int64) {
	var totalNet, totalVAT, totalGross big.Rat
	for _, line := range s.lines[quoteID] {
		addRat(&totalNet, line.LineNet)
		addRat(&totalVAT, line.LineVAT)
		addRat(&totalGross, line.LineGross)
	}
	for i := range s.quotes {
		if s.quotes[i].ID == quoteID {
			s.quotes[i].TotalNet = ratString(&totalNet)
			s.quotes[i].TotalVAT = ratString(&totalVAT)
			s.quotes[i].TotalGross = ratString(&totalGross)
			return
		}
	}
}

func (l *raenadTestQuoteLine) applyTotals() {
	if l.LineType != lineTypeItem || l.Qta == nil || l.UnitPrice == nil {
		return
	}
	qta := ratFromString(*l.Qta)
	unit := ratFromString(*l.UnitPrice)
	net := new(big.Rat).Mul(qta, unit)
	netString := ratString(net)
	l.LineNet = &netString
	pctString := "0"
	if l.IVAPercentSnapshot != nil {
		pctString = *l.IVAPercentSnapshot
	} else if l.CodIVA != nil {
		pctString = strings.TrimSpace(*l.CodIVA)
	}
	pct := ratFromString(pctString)
	vat := new(big.Rat).Quo(new(big.Rat).Mul(net, pct), big.NewRat(100, 1))
	vatString := ratString(vat)
	gross := new(big.Rat).Add(net, vat)
	grossString := ratString(gross)
	l.LineVAT = &vatString
	l.LineGross = &grossString
}

func quoteScanValues(q raenadTestQuote) []driver.Value {
	return []driver.Value{
		q.ID,
		q.QuoteNumber,
		q.CreatedAt,
		q.UpdatedAt,
		q.CreatedBy,
		q.UpdatedBy,
		q.AuthoringStatus,
		q.HubSpotSyncStatus,
		driverValue(q.HubSpotSyncError),
		driverTimeValue(q.HubSpotSyncedAt),
		driverValue(q.HubSpotCompanyID),
		driverValue(q.HubSpotContactID),
		driverValue(q.HubSpotDealID),
		driverValue(q.HubSpotPipelineID),
		driverValue(q.HubSpotPipelineLabel),
		driverValue(q.HubSpotDealstageID),
		driverValue(q.HubSpotDealstageLabel),
		driverValue(q.Customer.Name),
		driverValue(q.Customer.VAT),
		driverValue(q.Customer.TaxCode),
		driverValue(q.Customer.PEC),
		driverValue(q.Customer.Email),
		driverValue(q.Customer.Address),
		driverValue(q.Customer.ZIP),
		driverValue(q.Customer.City),
		driverValue(q.Customer.Province),
		driverValue(q.Customer.Country),
		driverValue(q.Customer.Language),
		driverValue(q.Customer.NumeroAziendaSnapshot),
		driverValue(q.Contact.FirstName),
		driverValue(q.Contact.LastName),
		driverValue(q.Contact.FullName),
		driverValue(q.Contact.Email),
		driverValue(q.Contact.Role),
		driverValue(q.DocumentDate),
		driverValue(q.Payment.MethodCode),
		driverValue(q.Payment.MethodLabel),
		driverValue(q.Payment.BankDetails),
		driverValue(q.Description),
		driverValue(q.InternalNotes),
		nonEmpty(q.TotalNet, "0.0000"),
		nonEmpty(q.TotalVAT, "0.0000"),
		nonEmpty(q.TotalGross, "0.0000"),
		nonEmpty(q.TotalPurchase, "0.0000"),
		nonEmpty(q.TotalGain, "0.0000"),
	}
}

func quoteLineScanValues(l raenadTestQuoteLine) []driver.Value {
	return []driver.Value{
		l.ID,
		l.QuoteID,
		l.Position,
		l.LineType,
		driverValue(l.ItemCode),
		driverValue(l.ItemDescription),
		driverValue(l.Description),
		driverValue(l.UnitOfMeasure),
		driverValue(l.Qta),
		driverValue(l.UnitPrice),
		driverValue(l.Discounts),
		driverValue(l.CodIVA),
		driverValue(l.IVAPercentSnapshot),
		driverValue(l.PurchaseUnitPrice),
		driverValue(l.LineNet),
		driverValue(l.LineVAT),
		driverValue(l.LineGross),
		driverValue(l.LinePurchase),
		driverValue(l.LineGain),
	}
}

func pdfExportScanValues(e raenadTestPDFExport) []driver.Value {
	return []driver.Value{
		e.ID,
		e.QuoteID,
		e.Revision,
		e.Filename,
		e.ContentType,
		driverValue(e.ChecksumSHA256),
		[]byte(nonEmpty(e.RenderPayload, "{}")),
		e.CreatedAt,
		e.CreatedBy,
		nonEmpty(e.HubSpotAttachmentStatus, "pending"),
		driverValue(e.HubSpotFileID),
		driverValue(e.HubSpotNoteID),
		driverValue(e.HubSpotDealID),
		driverTimeValue(e.HubSpotAttachedAt),
		driverValue(e.HubSpotError),
	}
}

func quoteTestColumns() []string {
	return []string{"id", "quote_number", "created_at", "updated_at", "created_by", "updated_by", "authoring_status", "hubspot_sync_status", "hubspot_sync_error", "hubspot_synced_at", "hubspot_company_id", "hubspot_contact_id", "hubspot_deal_id", "hubspot_pipeline_id", "hubspot_pipeline_label", "hubspot_dealstage_id", "hubspot_dealstage_label", "customer_name", "customer_vat", "customer_tax_code", "customer_pec", "customer_email", "customer_address", "customer_zip", "customer_city", "customer_province", "customer_country", "customer_language", "numero_azienda_snapshot", "contact_first_name", "contact_last_name", "contact_full_name", "contact_email", "contact_role", "document_date", "payment_method_code", "payment_method_label", "payment_bank_details", "description", "internal_notes", "total_net", "total_vat", "total_gross", "total_purchase", "total_gain"}
}

func quoteLineTestColumns() []string {
	return []string{"id", "quote_id", "position", "line_type", "item_code", "item_description", "description", "unit_of_measure", "qta", "unit_price", "discounts", "cod_iva", "iva_percent_snapshot", "purchase_unit_price", "line_net", "line_vat", "line_gross", "line_purchase", "line_gain"}
}

func pdfExportTestColumns() []string {
	return []string{"id", "quote_id", "revision", "filename", "content_type", "checksum_sha256", "render_payload", "created_at", "created_by", "hubspot_attachment_status", "hubspot_file_id", "hubspot_note_id", "hubspot_deal_id", "hubspot_attached_at", "hubspot_error"}
}

type raenadTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r raenadTestRows) Columns() []string {
	return r.columns
}

func (r raenadTestRows) Close() error {
	return nil
}

func (r *raenadTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func companyMatchesSearch(company raenadTestCompany, search string) bool {
	needle := strings.ToLower(strings.TrimSpace(search))
	for _, value := range []string{
		stringValue(company.Name),
		stringValue(company.NumeroAzienda),
		stringValue(company.VAT),
		stringValue(company.Domain),
		stringValue(company.Email),
	} {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}

func articleMatchesSearch(article raenadTestArticle, search string) bool {
	needle := strings.ToLower(strings.TrimSpace(search))
	for _, value := range []string{
		article.Code,
		stringValue(article.DescDefault),
		stringValue(article.DescITA),
		stringValue(article.DescENG),
		stringValue(article.ExtITA),
		stringValue(article.ExtENG),
		stringValue(article.ExtDefault),
	} {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}

func int64FromAny(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case int32:
		return int64(typed), true
	default:
		return 0, false
	}
}

func stringFromNamed(value driver.NamedValue) string {
	if value.Value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value.Value))
}

func intFromNamed(value driver.NamedValue) int {
	converted, _ := int64FromAny(value.Value)
	return int(converted)
}

func ptrFromNamed(value driver.NamedValue) *string {
	if value.Value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(fmt.Sprint(value.Value))
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func driverValue(value *string) driver.Value {
	if value == nil {
		return nil
	}
	return *value
}

func driverTimeValue(value *time.Time) driver.Value {
	if value == nil {
		return nil
	}
	return *value
}

func driverIntValue(value *int) driver.Value {
	if value == nil {
		return nil
	}
	return int64(*value)
}

func orderValue(value *int) int {
	if value == nil {
		return int(^uint(0) >> 1)
	}
	return *value
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func ratFromString(value string) *big.Rat {
	r := new(big.Rat)
	if _, ok := r.SetString(strings.TrimSpace(value)); ok {
		return r
	}
	return new(big.Rat)
}

func addRat(total *big.Rat, value *string) {
	if value == nil {
		return
	}
	total.Add(total, ratFromString(*value))
}

func ratString(value *big.Rat) string {
	if value == nil {
		return "0.0000"
	}
	scaled := new(big.Rat).Mul(value, big.NewRat(10000, 1))
	num := new(big.Int).Quo(scaled.Num(), scaled.Denom())
	out := new(big.Rat).SetFrac(num, big.NewInt(10000))
	return out.FloatString(4)
}
