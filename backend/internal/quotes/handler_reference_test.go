package quotes

import (
	"reflect"
	"strings"
	"testing"
)

func TestCustomerOrdersQueryMatchesAppsmithContract(t *testing.T) {
	mustContain := []string{
		"NOME_TESTATA_ORDINE",
		"ID_CLIENTE = @p1",
		"STATO_ORDINE IN ('Evaso', 'Confermato')",
		"GROUP BY NOME_TESTATA_ORDINE",
		"ORDER BY NOME_TESTATA_ORDINE DESC",
	}
	for _, frag := range mustContain {
		if !strings.Contains(customerOrdersQuery, frag) {
			t.Fatalf("customerOrdersQuery missing %q; query was:\n%s", frag, customerOrdersQuery)
		}
	}
	if strings.Contains(customerOrdersQuery, "LTRIM(RTRIM(NOME))") {
		t.Fatalf("customerOrdersQuery drifted back to NOME; query was:\n%s", customerOrdersQuery)
	}
	if strings.Contains(customerOrdersQuery, "NUMERO_AZIENDA") {
		t.Fatalf("customerOrdersQuery regressed to stale Alyante column; query was:\n%s", customerOrdersQuery)
	}
}

func TestCustomerPaymentQueryMatchesAppsmithContract(t *testing.T) {
	mustContain := []string{
		"CODICE_PAGAMENTO",
		"ISNULL",
		"CAST(CODICE_PAGAMENTO as INT)",
		"NUMERO_AZIENDA = @p1",
	}
	for _, frag := range mustContain {
		if !strings.Contains(customerPaymentQuery, frag) {
			t.Fatalf("customerPaymentQuery missing %q; query was:\n%s", frag, customerPaymentQuery)
		}
	}
}

func TestParseCategoryExclusions(t *testing.T) {
	got := parseCategoryExclusions("12, 13,foo,,15")
	want := []int{12, 13, 15}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCategoryExclusions mismatch: got %v, want %v", got, want)
	}
}

func TestParseKitInclusions(t *testing.T) {
	got := parseKitInclusions("62, 116,foo,0,-1,62,,119")
	want := []int{62, 116, 119}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseKitInclusions mismatch: got %v, want %v", got, want)
	}
}

func TestListKitsQueryMatchesAppsmithEligibility(t *testing.T) {
	mustContain := []string{
		"k.is_active = true",
		"k.ecommerce = false",
		"k.quotable = true",
		"ORDER BY pc.name, k.internal_name",
	}
	for _, frag := range mustContain {
		if !strings.Contains(listKitsQuery, frag) {
			t.Fatalf("listKitsQuery missing %q; query was:\n%s", frag, listKitsQuery)
		}
	}
}
