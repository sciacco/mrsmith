package openapiit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewUsesDefaultsAndDisablesWhenTokenMissing(t *testing.T) {
	if New(Config{}) != nil {
		t.Fatalf("expected nil client without API token")
	}

	client := New(Config{APIToken: " test-token "})
	if client == nil {
		t.Fatalf("expected client with API token")
	}
	if client.capBaseURL != DefaultCAPBaseURL {
		t.Fatalf("CAP base URL = %q, want %q", client.capBaseURL, DefaultCAPBaseURL)
	}
}

func TestLookupCAPSetsAuthAndDecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodGet; got != want {
			t.Fatalf("method = %s, want %s", got, want)
		}
		if got, want := r.URL.Path, "/cap/00136"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer test-token"; got != want {
			t.Fatalf("authorization = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Accept"), "application/json"; got != want {
			t.Fatalf("accept = %q, want %q", got, want)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"comuni": []map[string]any{
					{
						"istat":        "58091",
						"comune":       "Roma",
						"frazione":     nil,
						"comune_istat": "58091",
						"multi_cap":    true,
						"strade": map[string]any{
							"00136": []map[string]any{
								{
									"dug":                      "VIA",
									"dug_complemento":          nil,
									"nome":                     "TRIONFALE",
									"dug_apici":                "VIA",
									"dug_complemento_apici":    nil,
									"nome_apici":               "TRIONFALE",
									"denominazione_abbreviata": "VIA TRIONFALE",
									"archi_stradali": []map[string]any{
										{"dal": "69", "al": "5885", "parita": "D"},
									},
								},
							},
						},
					},
				},
				"provincia":       "Roma",
				"sigla_provincia": "RM",
				"regione":         "Lazio",
			},
			"success": true,
			"message": "",
			"error":   nil,
		})
	}))
	t.Cleanup(server.Close)

	client := NewWithBaseURL("test-token", server.URL, server.Client())
	got, err := client.CAP().LookupCAP(context.Background(), "00136")
	if err != nil {
		t.Fatalf("LookupCAP returned error: %v", err)
	}
	if !got.Success {
		t.Fatalf("expected success envelope")
	}
	if got.Data.SiglaProvincia != "RM" {
		t.Fatalf("sigla provincia = %q, want RM", got.Data.SiglaProvincia)
	}
	if len(got.Data.Comuni) != 1 {
		t.Fatalf("expected 1 municipality, got %d", len(got.Data.Comuni))
	}
	street := got.Data.Comuni[0].Strade["00136"][0]
	if street.DenominazioneAbbreviata != "VIA TRIONFALE" {
		t.Fatalf("street = %q, want VIA TRIONFALE", street.DenominazioneAbbreviata)
	}
	if got.Data.Comuni[0].Frazione != nil {
		t.Fatalf("expected nil frazione")
	}
}

func TestSearchMunicipalitiesEncodesQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/cerca_comuni"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		query := r.URL.Query()
		if got, want := query.Get("comune"), "San Dona"; got != want {
			t.Fatalf("comune query = %q, want %q", got, want)
		}
		if got, want := query.Get("cap"), "30027"; got != want {
			t.Fatalf("cap query = %q, want %q", got, want)
		}
		if got, want := query.Get("codice_catasto"), "H823"; got != want {
			t.Fatalf("codice_catasto query = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"result":[{"istat":"027033","comune":"San Dona di Piave","suppressed":false}],"suppressed":[]},"success":true,"message":"","error":null}`))
	}))
	t.Cleanup(server.Close)

	client := NewWithBaseURL("test-token", server.URL, server.Client())
	got, err := client.CAP().SearchMunicipalities(context.Background(), MunicipalitySearchParams{
		Comune:        " San Dona ",
		CAP:           "30027",
		CodiceCatasto: "H823",
	})
	if err != nil {
		t.Fatalf("SearchMunicipalities returned error: %v", err)
	}
	if got.Data.Result[0].ISTAT != "027033" {
		t.Fatalf("istat = %q, want 027033", got.Data.Result[0].ISTAT)
	}
}

func TestGetProvinceEscapesPathSegment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.RequestURI, "/province/A%2FB") {
			t.Fatalf("request URI = %q, want escaped path segment", r.RequestURI)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"sigla":"A/B","provincia":"Test","superficie":1,"residenti":2,"num_comuni":3,"istat":"001","regione":"Test"},"success":true,"message":"","error":null}`))
	}))
	t.Cleanup(server.Close)

	client := NewWithBaseURL("test-token", server.URL, server.Client())
	if _, err := client.CAP().GetProvince(context.Background(), "A/B"); err != nil {
		t.Fatalf("GetProvince returned error: %v", err)
	}
}

func TestGetMunicipalityAdvancedDecodesNestedStreetData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/comuni_advance/H501"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"istat": "58091",
				"comune": "Roma",
				"regione": "Lazio",
				"provincia": "Roma",
				"prefisso": "06",
				"cod_fisco": "H501",
				"superficie": 1308,
				"num_residenti": 2770226,
				"nome_abitanti": "Romani",
				"patrono": {"nome":"SS. Pietro e Paolo","data":"29 giugno"},
				"municipio": {"municipio":"Via del Campidoglio"},
				"istat_old": null,
				"sigla_provincia": "RM",
				"email": "roma@example.com",
				"pec": "roma@pec.example.com",
				"tel": "+39 06/67102001",
				"fax": "+39 06/67103590",
				"frazioni": ["Ostia"],
				"cap": ["00118"],
				"sestieri": null,
				"strade": {
					"00118": [{
						"dug": "VIA",
						"dug_complemento": null,
						"nome": "ANAGNINA",
						"dug_apici": "VIA",
						"dug_complemento_apici": null,
						"nome_apici": "ANAGNINA",
						"denominazione_abbreviata": "VIA ANAGNINA",
						"archi_stradali": [
							{"dal":"100","al":"748","parita":"P"},
							{"dal":"147","al":"559","parita":"D","colore":"R"}
						]
					}]
				}
			},
			"success": true,
			"message": "",
			"error": null
		}`))
	}))
	t.Cleanup(server.Close)

	client := NewWithBaseURL("test-token", server.URL, server.Client())
	got, err := client.CAP().GetMunicipalityAdvanced(context.Background(), "H501")
	if err != nil {
		t.Fatalf("GetMunicipalityAdvanced returned error: %v", err)
	}
	if got.Data.ISTATOld != nil {
		t.Fatalf("expected nil old ISTAT")
	}
	if len(got.Data.Sestieri) != 0 {
		t.Fatalf("expected no sestieri, got %d", len(got.Data.Sestieri))
	}
	ranges := got.Data.Strade["00118"][0].ArchiStradali
	if len(ranges) != 2 {
		t.Fatalf("expected 2 street ranges, got %d", len(ranges))
	}
	if ranges[1].Colore == nil || *ranges[1].Colore != "R" {
		t.Fatalf("expected red street-number color, got %#v", ranges[1].Colore)
	}
}

func TestSuppressedMunicipalitiesDecodeFlexibleCodFisco(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("sigla_provincia"), "VI"; got != want {
			t.Fatalf("sigla_provincia = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"istat":"24011","comune":"Barbarano Vicentino","cod_fisco":"A627","sigla_provincia":"VI","regione":"Veneto","provincia":"Vicenza"},
				{"istat":"24012","comune":"Numeric Code","cod_fisco":123,"sigla_provincia":"VI","regione":"Veneto","provincia":"Vicenza"}
			],
			"success": true,
			"message": "",
			"error": null
		}`))
	}))
	t.Cleanup(server.Close)

	client := NewWithBaseURL("test-token", server.URL, server.Client())
	got, err := client.CAP().ListSuppressedMunicipalities(context.Background(), "VI")
	if err != nil {
		t.Fatalf("ListSuppressedMunicipalities returned error: %v", err)
	}
	if got.Data[0].CodFisco.String() != "A627" {
		t.Fatalf("cod_fisco string = %q, want A627", got.Data[0].CodFisco.String())
	}
	if got.Data[1].CodFisco.String() != "123" {
		t.Fatalf("cod_fisco number = %q, want 123", got.Data[1].CodFisco.String())
	}
}

func TestUpstreamErrorPreservesStatusBodyMessageAndCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"success":false,"message":"insufficient credit","error":610,"data":null}`))
	}))
	t.Cleanup(server.Close)

	client := NewWithBaseURL("test-token", server.URL, server.Client())
	_, err := client.CAP().ListRegions(context.Background())
	if err == nil {
		t.Fatalf("expected upstream error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want %d", apiErr.StatusCode, http.StatusPaymentRequired)
	}
	if apiErr.Message != "insufficient credit" {
		t.Fatalf("message = %q, want insufficient credit", apiErr.Message)
	}
	if apiErr.Code == nil || *apiErr.Code != 610 {
		t.Fatalf("code = %#v, want 610", apiErr.Code)
	}
	if !strings.Contains(apiErr.Body, "insufficient credit") {
		t.Fatalf("body = %q, want upstream body", apiErr.Body)
	}
}
