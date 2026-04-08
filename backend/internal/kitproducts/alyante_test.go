package kitproducts

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

func TestAlyanteSyncTranslationUsesVerifiedAppsmithContract(t *testing.T) {
	var (
		gotQuery string
		gotArgs  []any
	)
	adapter := &AlyanteAdapter{
		execFn: func(_ context.Context, query string, args ...any) (sql.Result, error) {
			gotQuery = query
			gotArgs = args
			return testSQLResult(1), nil
		},
	}

	err := adapter.SyncTranslation(context.Background(), "CDL-ADSL-640", "it", "Descrizione IT")
	if err != nil {
		t.Fatalf("SyncTranslation returned error: %v", err)
	}

	query := normalizeSQL(gotQuery)
	for _, fragment := range []string{
		"UPDATE MG87_ARTDESC",
		"SET MG87_DESCART = @p1",
		"WHERE MG87_DITTA_CG18 = 1",
		"AND MG87_LINGUA_MG52 = @p3",
		"AND MG87_CODART_MG66 = @p2",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("expected query to contain %q, got %q", fragment, query)
		}
	}
	if !strings.Contains(gotQuery, "MG87_OPZIONE_MG5E = '                    '") {
		t.Fatalf("expected query to contain 20-space option filter, got %q", gotQuery)
	}

	if len(gotArgs) != 3 {
		t.Fatalf("expected 3 args, got %d", len(gotArgs))
	}
	if gotArgs[0] != "Descrizione IT" {
		t.Fatalf("expected short description as first arg, got %#v", gotArgs[0])
	}
	if gotArgs[1] != fmt.Sprintf("%-25s", "CDL-ADSL-640") {
		t.Fatalf("expected padded code as second arg, got %#v", gotArgs[1])
	}
	if gotArgs[2] != "ITA" {
		t.Fatalf("expected mapped ERP language as third arg, got %#v", gotArgs[2])
	}
}

type testSQLResult int64

func (r testSQLResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r testSQLResult) RowsAffected() (int64, error) {
	return int64(r), nil
}

func normalizeSQL(query string) string {
	return strings.Join(strings.Fields(query), " ")
}
