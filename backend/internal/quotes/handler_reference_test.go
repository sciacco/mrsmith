package quotes

import (
	"reflect"
	"strings"
	"testing"
)

func TestCustomerOrdersQueryMatchesAppsmithContract(t *testing.T) {
	mustContain := []string{
		"NOME_TESTATA_ORDINE",
		"NUMERO_AZIENDA = @p1",
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
}

func TestParseCategoryExclusions(t *testing.T) {
	got := parseCategoryExclusions("12, 13,foo,,15")
	want := []int{12, 13, 15}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCategoryExclusions mismatch: got %v, want %v", got, want)
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
