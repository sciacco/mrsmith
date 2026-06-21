package raenad

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQuoteCustomersDefaultExcludesCompaniesWithoutNumeroAzienda(t *testing.T) {
	state := &raenadTestState{
		companies: []raenadTestCompany{
			{
				ID:            1001,
				Name:          ptr("ERP Customer"),
				VAT:           ptr("IT001"),
				TaxCode:       ptr("CF001"),
				PEC:           ptr("erp@examplepec.it"),
				Email:         ptr("billing@example.com"),
				Domain:        ptr("example.com"),
				Address:       ptr("Via Roma 1"),
				ZIP:           ptr("20100"),
				City:          ptr("Milano"),
				Province:      ptr("MI"),
				Country:       ptr("IT"),
				Language:      ptr("it"),
				NumeroAzienda: ptr("12345"),
			},
			{ID: 1002, Name: ptr("Prospect Null"), NumeroAzienda: nil},
			{ID: 1003, Name: ptr("Prospect Blank"), NumeroAzienda: ptr("  ")},
		},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Mistra: openRaenadTestDBWithState(t, state)})

	rec := serveRaenadRequest(mux, http.MethodGet, "/aenad/v1/quotes/customers", t)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}

	var got []customerSelection
	decodeRaenadResponse(t, rec, &got)
	if len(got) != 1 {
		t.Fatalf("expected one ERP-linked customer, got %#v", got)
	}
	if got[0].HubSpotCompanyID != "1001" {
		t.Fatalf("hubspot id should be stringified bigint, got %#v", got[0].HubSpotCompanyID)
	}
	if got[0].Customer.NumeroAziendaSnapshot == nil || *got[0].Customer.NumeroAziendaSnapshot != "12345" {
		t.Fatalf("unexpected numero azienda snapshot: %#v", got[0].Customer.NumeroAziendaSnapshot)
	}
	if got[0].Customer.VAT == nil || *got[0].Customer.VAT != "IT001" || got[0].Customer.TaxCode == nil || *got[0].Customer.TaxCode != "CF001" {
		t.Fatalf("customer snapshot fields not mapped: %#v", got[0].Customer)
	}
}

func TestQuoteCustomersIncludeToggleIncludesProspects(t *testing.T) {
	state := &raenadTestState{
		companies: []raenadTestCompany{
			{ID: 1001, Name: ptr("ERP Customer"), NumeroAzienda: ptr("12345")},
			{ID: 1002, Name: ptr("Prospect Null"), NumeroAzienda: nil},
			{ID: 1003, Name: ptr("Prospect Blank"), NumeroAzienda: ptr("  ")},
		},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Mistra: openRaenadTestDBWithState(t, state)})

	rec := serveRaenadRequest(mux, http.MethodGet, "/aenad/v1/quotes/customers?include_without_numero_azienda=true&limit=10", t)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}

	var got []customerSelection
	decodeRaenadResponse(t, rec, &got)
	if len(got) != 3 {
		t.Fatalf("expected ERP customer plus prospects, got %#v", got)
	}
	if got[1].Customer.NumeroAziendaSnapshot != nil || got[2].Customer.NumeroAziendaSnapshot != nil {
		t.Fatalf("prospects should keep nullable numero_azienda snapshot: %#v", got)
	}
}

func TestQuoteCustomersSearchMatchesRequiredFields(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{name: "name", query: "Acme"},
		{name: "numero azienda", query: "ERP-77"},
		{name: "partita iva", query: "VAT-77"},
		{name: "domain", query: "acme.example"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := &raenadTestState{
				companies: []raenadTestCompany{
					{ID: 77, Name: ptr("Acme SpA"), VAT: ptr("VAT-77"), NumeroAzienda: ptr("ERP-77"), Domain: ptr("acme.example")},
					{ID: 88, Name: ptr("Other SpA"), VAT: ptr("VAT-88"), NumeroAzienda: ptr("ERP-88"), Domain: ptr("other.example")},
				},
			}
			mux := http.NewServeMux()
			RegisterRoutes(mux, Deps{Mistra: openRaenadTestDBWithState(t, state)})

			rec := serveRaenadRequest(mux, http.MethodGet, "/aenad/v1/quotes/customers?q="+tc.query, t)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
			}
			var got []customerSelection
			decodeRaenadResponse(t, rec, &got)
			if len(got) != 1 || got[0].HubSpotCompanyID != "77" {
				t.Fatalf("expected required-field match to return company 77, got %#v", got)
			}
			assertRaenadQueryContains(t, state, "loader.hubs_company", "c.name ILIKE", "c.numero_azienda ILIKE", "c.partita_iva ILIKE", "c.domain ILIKE", "c.email ILIKE")
		})
	}
}

func TestParseReferenceLimitDefaultsAndCaps(t *testing.T) {
	if got := parseReferenceLimit(""); got != 25 {
		t.Fatalf("empty limit should default to 25, got %d", got)
	}
	if got := parseReferenceLimit("-1"); got != 25 {
		t.Fatalf("invalid limit should default to 25, got %d", got)
	}
	if got := parseReferenceLimit("7"); got != 7 {
		t.Fatalf("explicit limit should be used, got %d", got)
	}
	if got := parseReferenceLimit("250"); got != 100 {
		t.Fatalf("limit should cap at 100, got %d", got)
	}
}

func TestQuotePaymentMethodsReturnsAllAndTrimsCode(t *testing.T) {
	state := &raenadTestState{
		paymentMethods: []raenadTestPaymentMethod{
			{Code: "999   ", Description: "Hidden", Selectable: false},
			{Code: "030   ", Description: "Carta", Selectable: true},
			{Code: "010   ", Description: "Bonifico", Selectable: true},
		},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Mistra: openRaenadTestDBWithState(t, state)})

	rec := serveRaenadRequest(mux, http.MethodGet, "/aenad/v1/quotes/payment-methods", t)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}

	var got []paymentMethodSelection
	decodeRaenadResponse(t, rec, &got)
	if len(got) != 3 {
		t.Fatalf("expected all three methods, got %#v", got)
	}
	if got[0].CodPagamento != "010" || got[0].DescPagamento != "Bonifico" {
		t.Fatalf("expected methods ordered by description and code trimmed, got %#v", got)
	}
	assertRaenadQueryContains(t, state, "loader.erp_metodi_pagamento", "ORDER BY desc_pagamento")
}

func TestQuoteStagesUsesConfiguredPipelineAndOrdering(t *testing.T) {
	mistraState := &raenadTestState{
		stages: []raenadTestStage{
			{ID: "s2", Label: ptr("Second"), Pipeline: "p1", DisplayOrder: intPtr(2), PipelineLabel: ptr("Aenad Pipeline")},
			{ID: "other", Label: ptr("Other"), Pipeline: "p2", DisplayOrder: intPtr(1), PipelineLabel: ptr("Other Pipeline")},
			{ID: "s1", Label: ptr("First"), Pipeline: "p1", DisplayOrder: intPtr(1), PipelineLabel: ptr("Aenad Pipeline")},
		},
	}
	configState := &raenadTestState{
		config: map[string][]byte{
			"raenad.hubspot_deal_pipeline": []byte(`{"pipeline_id":"p1","initial_dealstage_id":"s2"}`),
		},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Mistra:   openRaenadTestDBWithState(t, mistraState),
		ConfigDB: openRaenadTestDBWithState(t, configState),
	})

	rec := serveRaenadRequest(mux, http.MethodGet, "/aenad/v1/quotes/stages", t)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}

	var got stagesResponse
	decodeRaenadResponse(t, rec, &got)
	if got.PipelineID != "p1" || got.InitialDealstageID != "s2" || got.PipelineLabel == nil || *got.PipelineLabel != "Aenad Pipeline" {
		t.Fatalf("unexpected stage response header: %#v", got)
	}
	if len(got.Items) != 2 || got.Items[0].ID != "s1" || got.Items[1].ID != "s2" {
		t.Fatalf("stages not filtered/ordered by configured pipeline: %#v", got.Items)
	}
	if !got.Items[1].IsInitial {
		t.Fatalf("initial stage not marked: %#v", got.Items)
	}
	assertRaenadQueryContains(t, mistraState, "loader.hubs_stages", "WHERE s.pipeline = $1", "ORDER BY s.display_order")
}

func TestQuoteDefaultsReturnsConfiguredLineDefaults(t *testing.T) {
	configState := &raenadTestState{
		config: map[string][]byte{
			"aenad.quote_defaults": []byte(`{"cod_iva":"22"}`),
		},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{ConfigDB: openRaenadTestDBWithState(t, configState)})

	rec := serveRaenadRequest(mux, http.MethodGet, "/aenad/v1/quotes/defaults", t)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}

	var got quoteDefaultsResponse
	decodeRaenadResponse(t, rec, &got)
	if got.Line.CodIVA != "22" {
		t.Fatalf("unexpected defaults: %#v", got)
	}
}

func TestQuoteArticlesQueriesAlyanteDirectlyAndMapsLineDefaults(t *testing.T) {
	alyanteState := &raenadTestState{
		articles: []raenadTestArticle{
			{
				Code:        " ROUTER-1 ",
				DescDefault: ptr("Router default"),
				DescITA:     ptr("Router ITA"),
				DescENG:     ptr("Router ENG"),
				ExtITA:      ptr("Descrizione estesa ITA"),
				ExtENG:      ptr("Extended description ENG"),
				ExtDefault:  ptr("Extended description default"),
				UOM:         ptr(" PZ "),
				Price:       ptr("123.4500"),
			},
			{Code: "OTHER-1", DescITA: ptr("Other"), UOM: ptr("NR"), Price: ptr("9.0000")},
		},
	}
	configState := quoteDefaultsTestState("22")
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Alyante:  openRaenadTestDBWithState(t, alyanteState),
		ConfigDB: openRaenadTestDBWithState(t, configState),
	})

	rec := serveRaenadRequest(mux, http.MethodGet, "/aenad/v1/quotes/articles?q=router&limit=1", t)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}

	var got []articleLineInitializer
	decodeRaenadResponse(t, rec, &got)
	if len(got) != 1 {
		t.Fatalf("expected one article, got %#v", got)
	}
	item := got[0]
	if item.LineType != lineTypeItem || item.ItemCode != "ROUTER-1" || item.CodIVA != "22" {
		t.Fatalf("article did not initialize item identity/defaults: %#v", item)
	}
	if item.ItemDescription == nil || *item.ItemDescription != "Router ITA" {
		t.Fatalf("expected Italian short description, got %#v", item.ItemDescription)
	}
	if item.Description == nil || *item.Description != "Descrizione estesa ITA" {
		t.Fatalf("expected Italian extended description, got %#v", item.Description)
	}
	if item.UnitOfMeasure == nil || *item.UnitOfMeasure != "PZ" || item.UnitPrice == nil || *item.UnitPrice != "123.4500" {
		t.Fatalf("article economics were not mapped as editable strings: %#v", item)
	}
	if item.Qta != nil || item.Discounts != nil || item.PurchaseUnitPrice != nil {
		t.Fatalf("manual fields should remain empty: %#v", item)
	}
	body := rec.Body.String()
	for _, forbidden := range []string{"iva_percent_snapshot", "line_net", "hubspot", "pdf"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed non-initializer field %q: %s", forbidden, body)
		}
	}

	query := onlyRaenadQuery(t, alyanteState)
	for _, fragment := range []string{
		"WITH listino AS",
		"ROW_NUMBER() OVER",
		"l.LI10_DITTA_CG18 = 1",
		"l.LI10_FLGVENDACQ = 0",
		"GETDATE() BETWEEN l.LI10_DATAINIZIOVAL AND l.LI10_DATAFINEVAL",
		"WHERE l.rn = 1",
		"SELECT TOP (@p3)",
		"@p1 = ''",
		"LIKE @p2",
	} {
		if !strings.Contains(query.Query, fragment) {
			t.Fatalf("article query missing %q: %s", fragment, query.Query)
		}
	}
	forbiddenMirror := "loader.erp_anagrafica_" + "articoli_vendita"
	if strings.Contains(query.Query, forbiddenMirror) {
		t.Fatalf("article search must not use loader mirror: %s", query.Query)
	}
	assertNamedArg(t, query, 0, "router")
	assertNamedArg(t, query, 1, "%router%")
	assertNamedArg(t, query, 2, int64(1))
}

func TestQuoteArticlesLimitDefaultsAndCaps(t *testing.T) {
	cases := []struct {
		name      string
		path      string
		wantLimit int64
	}{
		{name: "default", path: "/aenad/v1/quotes/articles", wantLimit: 25},
		{name: "cap", path: "/aenad/v1/quotes/articles?limit=250", wantLimit: 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			alyanteState := &raenadTestState{}
			mux := http.NewServeMux()
			RegisterRoutes(mux, Deps{
				Alyante:  openRaenadTestDBWithState(t, alyanteState),
				ConfigDB: openRaenadTestDBWithState(t, quoteDefaultsTestState("22")),
			})

			rec := serveRaenadRequest(mux, http.MethodGet, tc.path, t)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
			}
			assertNamedArg(t, onlyRaenadQuery(t, alyanteState), 2, tc.wantLimit)
		})
	}
}

func TestQuoteArticlesSearchParamIsBound(t *testing.T) {
	alyanteState := &raenadTestState{}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Alyante:  openRaenadTestDBWithState(t, alyanteState),
		ConfigDB: openRaenadTestDBWithState(t, quoteDefaultsTestState("22")),
	})

	rec := serveRaenadRequest(mux, http.MethodGet, "/aenad/v1/quotes/articles?q=%27%3Bdrop%20table%20x--", t)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}
	query := onlyRaenadQuery(t, alyanteState)
	assertNamedArg(t, query, 0, "';drop table x--")
	assertNamedArg(t, query, 1, "%';drop table x--%")
	if strings.Contains(query.Query, "drop table") {
		t.Fatalf("search input was interpolated into SQL: %s", query.Query)
	}
}

func TestQuoteArticlesLanguageSelectionAndFallback(t *testing.T) {
	cases := []struct {
		name            string
		path            string
		article         raenadTestArticle
		wantShort       string
		wantDescription string
	}{
		{
			name: "default italian",
			path: "/aenad/v1/quotes/articles",
			article: raenadTestArticle{
				Code:       "A1",
				DescITA:    ptr("Nome italiano"),
				DescENG:    ptr("English name"),
				ExtITA:     ptr("Testo italiano"),
				ExtENG:     ptr("English text"),
				ExtDefault: ptr("Default text"),
				Price:      ptr("1.0000"),
			},
			wantShort:       "Nome italiano",
			wantDescription: "Testo italiano",
		},
		{
			name: "english selected",
			path: "/aenad/v1/quotes/articles?lang=en",
			article: raenadTestArticle{
				Code:    "A1",
				DescITA: ptr("Nome italiano"),
				DescENG: ptr("English name"),
				ExtITA:  ptr("Testo italiano"),
				ExtENG:  ptr("English text"),
				Price:   ptr("1.0000"),
			},
			wantShort:       "English name",
			wantDescription: "English text",
		},
		{
			name: "english falls back to default",
			path: "/aenad/v1/quotes/articles?lang=eng",
			article: raenadTestArticle{
				Code:        "A1",
				DescITA:     ptr("Nome italiano"),
				DescDefault: ptr("Default name"),
				ExtITA:      ptr("Testo italiano"),
				ExtDefault:  ptr("Default text"),
				Price:       ptr("1.0000"),
			},
			wantShort:       "Default name",
			wantDescription: "Default text",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			alyanteState := &raenadTestState{articles: []raenadTestArticle{tc.article}}
			mux := http.NewServeMux()
			RegisterRoutes(mux, Deps{
				Alyante:  openRaenadTestDBWithState(t, alyanteState),
				ConfigDB: openRaenadTestDBWithState(t, quoteDefaultsTestState("22")),
			})

			rec := serveRaenadRequest(mux, http.MethodGet, tc.path, t)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
			}
			var got []articleLineInitializer
			decodeRaenadResponse(t, rec, &got)
			if len(got) != 1 {
				t.Fatalf("expected one article, got %#v", got)
			}
			if got[0].ItemDescription == nil || *got[0].ItemDescription != tc.wantShort {
				t.Fatalf("unexpected localized short description: %#v", got[0].ItemDescription)
			}
			if got[0].Description == nil || *got[0].Description != tc.wantDescription {
				t.Fatalf("unexpected localized long description: %#v", got[0].Description)
			}
		})
	}
}

func TestQuoteArticlesConfigAndAlyanteErrorsAreStable(t *testing.T) {
	cases := []struct {
		name string
		deps Deps
		want string
	}{
		{
			name: "missing alyante",
			deps: Deps{ConfigDB: openRaenadTestDBWithState(t, quoteDefaultsTestState("22"))},
			want: "alyante_database_not_configured",
		},
		{
			name: "missing config database",
			deps: Deps{Alyante: openRaenadTestDBWithState(t, &raenadTestState{})},
			want: "raenad_config_not_configured",
		},
		{
			name: "missing quote defaults",
			deps: Deps{
				Alyante:  openRaenadTestDBWithState(t, &raenadTestState{}),
				ConfigDB: openRaenadTestDBWithState(t, &raenadTestState{}),
			},
			want: "raenad_config_not_configured",
		},
		{
			name: "invalid quote defaults",
			deps: Deps{
				Alyante: openRaenadTestDBWithState(t, &raenadTestState{}),
				ConfigDB: openRaenadTestDBWithState(t, &raenadTestState{
					config: map[string][]byte{"aenad.quote_defaults": []byte(`{"cod_iva":" "}`)},
				}),
			},
			want: "raenad_config_not_configured",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			RegisterRoutes(mux, tc.deps)

			rec := serveRaenadRequest(mux, http.MethodGet, "/aenad/v1/quotes/articles", t)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d body=%q", rec.Code, rec.Body.String())
			}
			if strings.TrimSpace(rec.Body.String()) != `{"error":"`+tc.want+`"}` {
				t.Fatalf("unexpected stable error body: %q", rec.Body.String())
			}
		})
	}
}

func TestReferenceConfigErrorsAreStable(t *testing.T) {
	cases := []struct {
		name string
		path string
		deps Deps
	}{
		{
			name: "stages missing runtime config",
			path: "/aenad/v1/quotes/stages",
			deps: Deps{
				Mistra:   openRaenadTestDBWithState(t, &raenadTestState{}),
				ConfigDB: openRaenadTestDBWithState(t, &raenadTestState{}),
			},
		},
		{
			name: "stages invalid runtime config",
			path: "/aenad/v1/quotes/stages",
			deps: Deps{
				Mistra: openRaenadTestDBWithState(t, &raenadTestState{}),
				ConfigDB: openRaenadTestDBWithState(t, &raenadTestState{
					config: map[string][]byte{"raenad.hubspot_deal_pipeline": []byte(`{"pipeline_id":""}`)},
				}),
			},
		},
		{
			name: "defaults missing runtime config",
			path: "/aenad/v1/quotes/defaults",
			deps: Deps{ConfigDB: openRaenadTestDBWithState(t, &raenadTestState{})},
		},
		{
			name: "defaults invalid runtime config",
			path: "/aenad/v1/quotes/defaults",
			deps: Deps{ConfigDB: openRaenadTestDBWithState(t, &raenadTestState{
				config: map[string][]byte{"aenad.quote_defaults": []byte(`{"cod_iva":" "}`)},
			})},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			RegisterRoutes(mux, tc.deps)

			rec := serveRaenadRequest(mux, http.MethodGet, tc.path, t)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d body=%q", rec.Code, rec.Body.String())
			}
			if strings.TrimSpace(rec.Body.String()) != `{"error":"raenad_config_not_configured"}` {
				t.Fatalf("unexpected stable config error: %q", rec.Body.String())
			}
		})
	}
}

func serveRaenadRequest(mux *http.ServeMux, method, path string, t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req = withRoles(req, "app_aenad_access")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decodeRaenadResponse(t *testing.T, rec *httptest.ResponseRecorder, dest any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), dest); err != nil {
		t.Fatalf("failed to decode response %q: %v", rec.Body.String(), err)
	}
}

func assertRaenadQueryContains(t *testing.T, state *raenadTestState, table string, fragments ...string) {
	t.Helper()
	for _, query := range state.allQueries() {
		if !strings.Contains(query.Query, table) {
			continue
		}
		for _, fragment := range fragments {
			if !strings.Contains(query.Query, fragment) {
				t.Fatalf("query for %s missing %q: %s", table, fragment, query.Query)
			}
		}
		return
	}
	t.Fatalf("query for %s was not executed; queries=%#v", table, state.allQueries())
}

func onlyRaenadQuery(t *testing.T, state *raenadTestState) raenadTestQuery {
	t.Helper()
	queries := state.allQueries()
	if len(queries) != 1 {
		t.Fatalf("expected one query, got %#v", queries)
	}
	return queries[0]
}

func assertNamedArg(t *testing.T, query raenadTestQuery, index int, want any) {
	t.Helper()
	if index >= len(query.Args) {
		t.Fatalf("query args missing index %d: %#v", index, query.Args)
	}
	got := query.Args[index].Value
	if wantInt, ok := want.(int64); ok {
		gotInt, ok := int64FromAny(got)
		if !ok || gotInt != wantInt {
			t.Fatalf("arg %d = %#v, want %d", index, got, wantInt)
		}
		return
	}
	if got != want {
		t.Fatalf("arg %d = %#v, want %#v", index, got, want)
	}
}

func quoteDefaultsTestState(codIVA string) *raenadTestState {
	return &raenadTestState{
		config: map[string][]byte{
			"aenad.quote_defaults": []byte(`{"cod_iva":"` + codIVA + `"}`),
		},
	}
}

func intPtr(value int) *int {
	return &value
}
