package quotes

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
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
				"address", "lingua", "template_description", "rif_ordcli", "rif_tech_nom",
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
				"Via Roma 1", "ITA", "Standard IT", nil, nil,
				nil, nil, nil, nil,
				nil, nil, nil, nil,
			}},
		}, nil
	}
	return nil, errors.New("unexpected query before APPROVED guard: " + query)
}

var _ driver.QueryerContext = (*orderConversionStatusTestConn)(nil)
var _ driver.Pinger = (*orderConversionStatusTestConn)(nil)
