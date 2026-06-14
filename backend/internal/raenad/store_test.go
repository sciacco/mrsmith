package raenad

import (
	"database/sql"
	"reflect"
	"testing"
	"time"
)

func TestNewSQLStore(t *testing.T) {
	if got := NewSQLStore(nil, nil); got != nil {
		t.Fatalf("expected nil store when both handles are nil")
	}

	db := openRaenadTestDB(t)
	got := NewSQLStore(db, nil)
	if got == nil {
		t.Fatalf("expected store")
	}
	if got.mistra != db {
		t.Fatalf("store did not keep mistra handle")
	}
	if got.configDB != nil {
		t.Fatalf("expected nil config handle")
	}
}

func TestScanQuoteMapsNullableSnapshotsAndDecimalStrings(t *testing.T) {
	createdAt := time.Date(2026, 6, 14, 10, 11, 12, 123, time.UTC)
	updatedAt := time.Date(2026, 6, 14, 11, 12, 13, 456, time.UTC)
	syncedAt := time.Date(2026, 6, 14, 12, 13, 14, 789, time.UTC)

	got, err := scanQuote(fakeScanner{values: []any{
		int64(42),
		"AE-42/2026",
		createdAt,
		updatedAt,
		"creator-subject",
		"updater-subject",
		authoringStatusDraft,
		hubSpotSyncStatusSucceeded,
		nil,
		syncedAt,
		"company-1",
		nil,
		"deal-1",
		"pipeline-1",
		"Pipeline",
		nil,
		nil,
		"Customer SpA",
		"IT123",
		nil,
		nil,
		"billing@example.com",
		"Street 1",
		"20100",
		"Milano",
		"MI",
		"IT",
		"it",
		"12345",
		"Maria",
		nil,
		"Maria",
		"maria@example.com",
		nil,
		"2026-06-14",
		"30",
		"Bonifico 30 giorni",
		nil,
		"Quote title",
		nil,
		"1234.5600",
		"271.6032",
		"1506.1632",
		"1000.0000",
		"234.5600",
	}})
	if err != nil {
		t.Fatalf("scanQuote returned error: %v", err)
	}

	if got.ID != 42 || got.QuoteNumber != "AE-42/2026" {
		t.Fatalf("unexpected quote identity: %#v", got.quoteSummary)
	}
	if got.TotalNet != "1234.5600" || got.TotalVAT != "271.6032" || got.TotalGross != "1506.1632" {
		t.Fatalf("totals were not preserved as strings: %#v", got.quoteSummary)
	}
	if got.HubSpotContactID != nil || got.HubSpotDealstageID != nil || got.HubSpotDealstageLabel != nil {
		t.Fatalf("expected nil nullable HubSpot fields: %#v", got.quoteSummary)
	}
	if got.HubSpotCompanyID == nil || *got.HubSpotCompanyID != "company-1" {
		t.Fatalf("unexpected company id: %#v", got.HubSpotCompanyID)
	}
	if got.Customer.TaxCode != nil || got.Customer.Name == nil || *got.Customer.Name != "Customer SpA" {
		t.Fatalf("unexpected customer snapshot: %#v", got.Customer)
	}
	if got.Contact.LastName != nil || got.Contact.Email == nil || *got.Contact.Email != "maria@example.com" {
		t.Fatalf("unexpected contact snapshot: %#v", got.Contact)
	}
	if got.Payment.BankDetails != nil || got.Payment.MethodCode == nil || *got.Payment.MethodCode != "30" {
		t.Fatalf("unexpected payment snapshot: %#v", got.Payment)
	}
	if got.HubSpotSyncedAt == nil || *got.HubSpotSyncedAt != syncedAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("unexpected synced timestamp: %#v", got.HubSpotSyncedAt)
	}
}

func TestScanQuoteLineMapsNullableEconomics(t *testing.T) {
	got, err := scanQuoteLine(fakeScanner{values: []any{
		int64(9),
		int64(42),
		3,
		lineTypeItem,
		"ITEM-1",
		"Imported description",
		"Editable description",
		"pz",
		"2.0000",
		"10.5000",
		"10+5%",
		"22",
		nil,
		nil,
		"17.9550",
		"3.9501",
		"21.9051",
		nil,
		nil,
	}})
	if err != nil {
		t.Fatalf("scanQuoteLine returned error: %v", err)
	}

	if got.ID != 9 || got.QuoteID != 42 || got.Position != 3 || got.LineType != lineTypeItem {
		t.Fatalf("unexpected line identity: %#v", got)
	}
	if got.Qta == nil || *got.Qta != "2.0000" || got.UnitPrice == nil || *got.UnitPrice != "10.5000" {
		t.Fatalf("line economics were not preserved as strings: %#v", got)
	}
	if got.IVAPercentSnapshot != nil || got.PurchaseUnitPrice != nil || got.LinePurchase != nil || got.LineGain != nil {
		t.Fatalf("expected nullable economics to remain nil: %#v", got)
	}
	if got.LineNet == nil || *got.LineNet != "17.9550" {
		t.Fatalf("unexpected line net: %#v", got.LineNet)
	}
}

type fakeScanner struct {
	values []any
}

func (s fakeScanner) Scan(dest ...any) error {
	if len(dest) != len(s.values) {
		return errFakeScanCount
	}
	for i := range dest {
		assignFakeScanValue(dest[i], s.values[i])
	}
	return nil
}

var errFakeScanCount = fakeScanError("fake scan destination count mismatch")

type fakeScanError string

func (e fakeScanError) Error() string {
	return string(e)
}

func assignFakeScanValue(dest any, value any) {
	switch target := dest.(type) {
	case *int:
		*target = value.(int)
	case *int64:
		*target = value.(int64)
	case *string:
		*target = value.(string)
	case *time.Time:
		*target = value.(time.Time)
	case *sql.NullString:
		if value == nil {
			*target = sql.NullString{}
			return
		}
		if typed, ok := value.(sql.NullString); ok {
			*target = typed
			return
		}
		*target = sql.NullString{String: value.(string), Valid: true}
	case *sql.NullTime:
		if value == nil {
			*target = sql.NullTime{}
			return
		}
		if typed, ok := value.(sql.NullTime); ok {
			*target = typed
			return
		}
		*target = sql.NullTime{Time: value.(time.Time), Valid: true}
	default:
		panic("unsupported fake scan destination: " + reflect.TypeOf(dest).String())
	}
}
