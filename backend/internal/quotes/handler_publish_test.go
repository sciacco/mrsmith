package quotes

import (
	"strings"
	"testing"
)

func TestPaymentMethodLabelQueryUsesLoaderSchemaColumns(t *testing.T) {
	if !strings.Contains(paymentMethodLabelQuery, "desc_pagamento") {
		t.Fatalf("paymentMethodLabelQuery missing desc_pagamento: %s", paymentMethodLabelQuery)
	}
	if !strings.Contains(paymentMethodLabelQuery, "cod_pagamento") {
		t.Fatalf("paymentMethodLabelQuery missing cod_pagamento: %s", paymentMethodLabelQuery)
	}
	if strings.Contains(paymentMethodLabelQuery, "descrizione") || strings.Contains(paymentMethodLabelQuery, "codice") {
		t.Fatalf("paymentMethodLabelQuery regressed to stale column names: %s", paymentMethodLabelQuery)
	}
}
