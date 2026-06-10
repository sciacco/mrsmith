package aenad

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleDocumentPDFGuards(t *testing.T) {
	t.Run("503 when renderer not configured", func(t *testing.T) {
		h := &Handler{}
		req := httptest.NewRequest(http.MethodGet, "/aenad/v1/documents/1/pdf", nil)
		rec := httptest.NewRecorder()
		h.handleDocumentPDF(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})
}

func TestParseCondizioni(t *testing.T) {
	cases := map[string]bool{
		"":      true,
		"1":     true,
		"true":  true,
		"si":    true,
		"0":     false,
		"false": false,
		"no":    false,
		"FALSE": false,
	}
	for raw, want := range cases {
		if got := parseCondizioni(raw); got != want {
			t.Errorf("parseCondizioni(%q) = %v, want %v", raw, got, want)
		}
	}
}

func ns(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }

func TestFormatEuroIT(t *testing.T) {
	cases := []struct {
		in   sql.NullString
		want string
	}{
		{sql.NullString{}, ""},
		{ns(""), ""},
		{ns("240.0000"), "€ 240,00"},
		{ns("24276.1300"), "€ 24.276,13"},
		{ns("13689.6400"), "€ 13.689,64"},
		{ns("1109.8200"), "€ 1.109,82"},
		{ns("0.5000"), "€ 0,50"},
		{ns("2.005"), "€ 2,01"},  // half-up
		{ns("9.999"), "€ 10,00"}, // riporto su intero
		{ns("999.999"), "€ 1.000,00"},
		{ns("-52.8000"), "€ -52,80"},
		{ns("1234567.8900"), "€ 1.234.567,89"},
	}
	for _, c := range cases {
		if got := formatEuroIT(c.in); got != c.want {
			t.Errorf("formatEuroIT(%q) = %q, want %q", c.in.String, got, c.want)
		}
	}
}

func TestFormatQtaIT(t *testing.T) {
	cases := []struct {
		in   sql.NullString
		want string
	}{
		{sql.NullString{}, ""},
		{ns("1.0000"), "1"},
		{ns("2.0000"), "2"},
		{ns("1.5000"), "1,5"},
		{ns("0.2500"), "0,25"},
		{ns("1200.0000"), "1.200"},
	}
	for _, c := range cases {
		if got := formatQtaIT(c.in); got != c.want {
			t.Errorf("formatQtaIT(%q) = %q, want %q", c.in.String, got, c.want)
		}
	}
}

func TestRigaFiscale(t *testing.T) {
	cases := []struct {
		cf, piva, want string
	}{
		{"", "01598660056", "P.Iva 01598660056"},
		{"11361441006", "11361441006", "C.F./P.Iva 11361441006"},
		{"MSCRMN50A01A794L", "01872880164", "C.F. MSCRMN50A01A794L - P.Iva 01872880164"},
		{"MSCRMN50A01A794L", "", "C.F. MSCRMN50A01A794L"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := rigaFiscale(c.cf, c.piva); got != c.want {
			t.Errorf("rigaFiscale(%q, %q) = %q, want %q", c.cf, c.piva, got, c.want)
		}
	}
}

func TestCapCittaProv(t *testing.T) {
	if got := capCittaProv("20122", "Milano", "MI"); got != "20122  Milano  (MI)" {
		t.Errorf("capCittaProv = %q", got)
	}
	if got := capCittaProv("", "Milano", ""); got != "Milano" {
		t.Errorf("capCittaProv senza cap/prov = %q", got)
	}
}

func TestDescToHTML(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"testo semplice", "testo semplice"},
		// Prefisso non chiuso: stila il resto della riga (convenzione Easyfatt).
		{"**SERVIZI UNA TANTUM", "<b>SERVIZI UNA TANTUM</b>"},
		{"//OPZIONALE", "<i>OPZIONALE</i>"},
		// Coppie chiuse inline.
		{"**RICONDIZIONATO** Docking Station", "<b>RICONDIZIONATO</b> Docking Station"},
		// Multiriga con stato per riga.
		{"//PRO SUPPORT PLUS :\r\n//Supporto in loco", "<i>PRO SUPPORT PLUS :</i><br><i>Supporto in loco</i>"},
		// "://" non è un marker corsivo.
		{"vedi https://example.com/x", "vedi https://example.com/x"},
		// Escape HTML prima dei marker.
		{"taglio <40mm & **resa**", "taglio &lt;40mm &amp; <b>resa</b>"},
		// Tripla stella: la coppia ** racchiude un asterisco letterale.
		{"***Alimentatore non incluso***", "<b>*Alimentatore non incluso*</b>"},
	}
	for _, c := range cases {
		if got := descToHTML(c.in); got != c.want {
			t.Errorf("descToHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestOffertaFilename(t *testing.T) {
	data := sql.NullTime{Time: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC), Valid: true}
	got := offertaFilename("Offerta", "1038", data, "RIMOND S.R.L.")
	want := "Offerta 1038 del 03-06-2026 RIMOND S R L.pdf"
	if got != want {
		t.Errorf("offertaFilename = %q, want %q", got, want)
	}
	got = offertaFilename("", "", sql.NullTime{}, `cli/ente "strano"`)
	want = "Documento cli ente strano.pdf"
	if got != want {
		t.Errorf("offertaFilename fallback = %q, want %q", got, want)
	}
}
