package raenad

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const (
	defaultReferenceLimit = 25
	maxReferenceLimit     = 100
)

type customerSelection struct {
	HubSpotCompanyID    string                `json:"hubspot_company_id"`
	Customer            customerSnapshotInput `json:"customer"`
	Domain              *string               `json:"domain"`
	AlyanteIDAnagrafica *string               `json:"alyante_id_anagrafica"`
}

type paymentMethodSelection struct {
	CodPagamento  string `json:"cod_pagamento"`
	DescPagamento string `json:"desc_pagamento"`
}

type stagesResponse struct {
	PipelineID         string           `json:"pipeline_id"`
	PipelineLabel      *string          `json:"pipeline_label"`
	InitialDealstageID string           `json:"initial_dealstage_id"`
	Items              []stageSelection `json:"items"`
}

type stageSelection struct {
	ID            string  `json:"id"`
	Label         *string `json:"label"`
	PipelineID    string  `json:"pipeline_id"`
	PipelineLabel *string `json:"pipeline_label"`
	DisplayOrder  *int    `json:"display_order"`
	IsInitial     bool    `json:"is_initial"`
}

type quoteDefaultsResponse struct {
	Line quoteLineDefaults `json:"line"`
}

type articleLineInitializer struct {
	LineType          string  `json:"line_type"`
	ItemCode          string  `json:"item_code"`
	ItemDescription   *string `json:"item_description"`
	Description       *string `json:"description"`
	UnitOfMeasure     *string `json:"unit_of_measure"`
	Qta               *string `json:"qta"`
	UnitPrice         *string `json:"unit_price"`
	Discounts         *string `json:"discounts"`
	CodIVA            string  `json:"cod_iva"`
	PurchaseUnitPrice *string `json:"purchase_unit_price"`
}

type quoteLineDefaults struct {
	CodIVA string `json:"cod_iva"`
}

type hubSpotDealPipelineConfig struct {
	PipelineID         string `json:"pipeline_id"`
	InitialDealstageID string `json:"initial_dealstage_id"`
}

type quoteDefaultsConfig struct {
	CodIVA string `json:"cod_iva"`
}

const alyanteArticleSearchQuery = `
WITH listino AS (
	SELECT
		l.*,
		ROW_NUMBER() OVER (
			PARTITION BY
				l.LI10_CODART_MG66,
				l.LI10_NUMLIST,
				l.LI10_OPZIONE_MG5E,
				l.LI10_DEPOS_MG58
			ORDER BY
				l.LI10_DATAINIZIOVAL DESC,
				l.LI10_DATAFINEVAL DESC,
				l.LI10_LASTCHANGE DESC
		) AS rn
	FROM [dbo].[LI10_LISTARTIC] l
	WHERE l.LI10_DITTA_CG18 = 1
	  AND l.LI10_FLGVENDACQ = 0
	  AND GETDATE() BETWEEN l.LI10_DATAINIZIOVAL AND l.LI10_DATAFINEVAL
)
SELECT TOP (@p3)
	LTRIM(RTRIM(l.LI10_CODART_MG66)) AS COD_ART,
	ISNULL(dDEF.MG87_DESCART, '') AS DESCART_DEF,
	ISNULL(dITA.MG87_DESCART, '') AS DESCART_ITA,
	ISNULL(dENG.MG87_DESCART, '') AS DESCART_ENG,
	ISNULL(dITA.MG87_DESCARTEST, '') AS DESCARTEST_ITA,
	ISNULL(dENG.MG87_DESCARTEST, '') AS DESCARTEST_ENG,
	ISNULL(dDEF.MG87_DESCARTEST, '') AS DESCARTEST_DEF,
	LTRIM(RTRIM(ISNULL(a.MG66_UM1, ''))) AS MG66_UM1,
	CONVERT(varchar(50), CONVERT(decimal(18,4), l.LI10_PREZZO)) AS LI10_PREZZO
FROM listino l
JOIN [dbo].[MG66_ANAGRART] a
	ON a.MG66_DITTA_CG18 = l.LI10_DITTA_CG18
   AND a.MG66_CODART = l.LI10_CODART_MG66
LEFT JOIN [dbo].[MG53_FAMIGLIE] fam
	ON fam.MG53_DITTA_CG18 = a.MG66_DITTA_CG18
   AND fam.MG53_CODFAM = a.MG66_FAM_MG53
LEFT JOIN [dbo].[MG54_SOTTOFAM] sfam
	ON sfam.MG54_DITTA_CG18 = a.MG66_DITTA_CG18
   AND sfam.MG54_CODFAM_MG53 = a.MG66_FAM_MG53
   AND sfam.MG54_CODSFAM = a.MG66_SFAM_MG54
LEFT JOIN [dbo].[MG55_GRUPPI] grp
	ON grp.MG55_DITTA_CG18 = a.MG66_DITTA_CG18
   AND grp.MG55_CODFAM_MG53 = a.MG66_FAM_MG53
   AND grp.MG55_CODSFAM_MG54 = a.MG66_SFAM_MG54
   AND grp.MG55_CODGRUPPO = a.MG66_GRUPPO_MG55
LEFT JOIN [dbo].[MG87_ARTDESC] dITA
	ON dITA.MG87_DITTA_CG18 = l.LI10_DITTA_CG18
   AND dITA.MG87_CODART_MG66 = l.LI10_CODART_MG66
   AND dITA.MG87_OPZIONE_MG5E = l.LI10_OPZIONE_MG5E
   AND dITA.MG87_LINGUA_MG52 = 'ITA'
LEFT JOIN [dbo].[MG87_ARTDESC] dENG
	ON dENG.MG87_DITTA_CG18 = l.LI10_DITTA_CG18
   AND dENG.MG87_CODART_MG66 = l.LI10_CODART_MG66
   AND dENG.MG87_OPZIONE_MG5E = l.LI10_OPZIONE_MG5E
   AND dENG.MG87_LINGUA_MG52 = 'ENG'
LEFT JOIN [dbo].[MG87_ARTDESC] dDEF
	ON dDEF.MG87_DITTA_CG18 = l.LI10_DITTA_CG18
   AND dDEF.MG87_CODART_MG66 = l.LI10_CODART_MG66
   AND dDEF.MG87_OPZIONE_MG5E = l.LI10_OPZIONE_MG5E
   AND dDEF.MG87_LINGUA_MG52 = ''
WHERE l.rn = 1
  AND (
	@p1 = ''
	OR l.LI10_CODART_MG66 LIKE @p2
	OR dITA.MG87_DESCART LIKE @p2
	OR dENG.MG87_DESCART LIKE @p2
	OR dDEF.MG87_DESCART LIKE @p2
	OR dITA.MG87_DESCARTEST LIKE @p2
	OR dENG.MG87_DESCARTEST LIKE @p2
	OR dDEF.MG87_DESCARTEST LIKE @p2
	OR a.MG66_MARCA_MG64 LIKE @p2
	OR fam.MG53_DESCRFAM LIKE @p2
	OR sfam.MG54_DESCRSFAM LIKE @p2
	OR grp.MG55_DESCRGRUPPO LIKE @p2
  )
ORDER BY
	CASE WHEN @p1 <> '' AND l.LI10_CODART_MG66 LIKE @p1 + '%' THEN 0 ELSE 1 END,
	CASE WHEN @p1 <> '' AND COALESCE(NULLIF(dITA.MG87_DESCART, ''), NULLIF(dENG.MG87_DESCART, ''), dDEF.MG87_DESCART, '') LIKE @p1 + '%' THEN 0 ELSE 1 END,
	LTRIM(RTRIM(l.LI10_CODART_MG66))`

func (h *Handler) handleQuoteCustomers(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	limit := parseReferenceLimit(r.URL.Query().Get("limit"))
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	includeWithoutNumeroAzienda := parseBoolQuery(r.URL.Query().Get("include_without_numero_azienda"))

	rows, err := h.deps.Mistra.QueryContext(r.Context(), `
		SELECT
			c.id,
			c.name,
			c.partita_iva,
			c.codice_fiscale,
			c.pec,
			c.email,
			c.domain,
			c.address,
			c.zip,
			c.city,
			c.provincia_di_fatturazione,
			c.country,
			c.lingua,
			c.alyante_id_anagrafica,
			c.numero_azienda
		FROM loader.hubs_company c
		WHERE ($1::boolean OR NULLIF(BTRIM(COALESCE(c.numero_azienda, '')), '') IS NOT NULL)
		  AND (
			$2 = ''
			OR c.name ILIKE '%' || $2 || '%'
			OR c.numero_azienda ILIKE '%' || $2 || '%'
			OR c.partita_iva ILIKE '%' || $2 || '%'
			OR c.domain ILIKE '%' || $2 || '%'
			OR c.email ILIKE '%' || $2 || '%'
		  )
		ORDER BY
			CASE WHEN NULLIF(BTRIM(COALESCE(c.numero_azienda, '')), '') IS NULL THEN 1 ELSE 0 END,
			CASE WHEN $2 <> '' AND BTRIM(COALESCE(c.name, '')) ILIKE $2 || '%' THEN 0 ELSE 1 END,
			BTRIM(COALESCE(c.name, '')),
			c.id
		LIMIT $3`, includeWithoutNumeroAzienda, search, limit)
	if err != nil {
		h.dbFailure(w, r, "quote_customers", err)
		return
	}
	defer rows.Close()

	items := make([]customerSelection, 0)
	for rows.Next() {
		var (
			item                customerSelection
			companyID           int64
			name                sql.NullString
			vat                 sql.NullString
			taxCode             sql.NullString
			pec                 sql.NullString
			email               sql.NullString
			domain              sql.NullString
			address             sql.NullString
			zip                 sql.NullString
			city                sql.NullString
			province            sql.NullString
			country             sql.NullString
			language            sql.NullString
			alyanteIDAnagrafica sql.NullString
			numeroAzienda       sql.NullString
		)
		if err := rows.Scan(
			&companyID,
			&name,
			&vat,
			&taxCode,
			&pec,
			&email,
			&domain,
			&address,
			&zip,
			&city,
			&province,
			&country,
			&language,
			&alyanteIDAnagrafica,
			&numeroAzienda,
		); err != nil {
			h.dbFailure(w, r, "quote_customers_scan", err)
			return
		}

		item.HubSpotCompanyID = strconv.FormatInt(companyID, 10)
		item.Customer = customerSnapshotInput{
			Name:                  nullTrimmedStringPtr(name),
			VAT:                   nullTrimmedStringPtr(vat),
			TaxCode:               nullTrimmedStringPtr(taxCode),
			PEC:                   nullTrimmedStringPtr(pec),
			Email:                 nullTrimmedStringPtr(email),
			Address:               nullTrimmedStringPtr(address),
			ZIP:                   nullTrimmedStringPtr(zip),
			City:                  nullTrimmedStringPtr(city),
			Province:              nullTrimmedStringPtr(province),
			Country:               nullTrimmedStringPtr(country),
			Language:              nullTrimmedStringPtr(language),
			NumeroAziendaSnapshot: nullTrimmedStringPtr(numeroAzienda),
		}
		item.Domain = nullTrimmedStringPtr(domain)
		item.AlyanteIDAnagrafica = nullTrimmedStringPtr(alyanteIDAnagrafica)
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "quote_customers") {
		return
	}

	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleQuotePaymentMethods(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.deps.Mistra.QueryContext(r.Context(), `
		SELECT RTRIM(cod_pagamento) AS cod_pagamento, desc_pagamento
		FROM loader.erp_metodi_pagamento
		WHERE selezionabile IS TRUE
		ORDER BY desc_pagamento, RTRIM(cod_pagamento)`)
	if err != nil {
		h.dbFailure(w, r, "quote_payment_methods", err)
		return
	}
	defer rows.Close()

	items := make([]paymentMethodSelection, 0)
	for rows.Next() {
		var item paymentMethodSelection
		if err := rows.Scan(&item.CodPagamento, &item.DescPagamento); err != nil {
			h.dbFailure(w, r, "quote_payment_methods_scan", err)
			return
		}
		item.CodPagamento = strings.TrimSpace(item.CodPagamento)
		item.DescPagamento = strings.TrimSpace(item.DescPagamento)
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "quote_payment_methods") {
		return
	}

	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleQuoteStages(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) || !h.requireConfigDB(w) {
		return
	}

	cfg, ok := h.readHubSpotDealPipelineConfig(r.Context())
	if !ok {
		httputil.Error(w, http.StatusServiceUnavailable, "raenad_config_not_configured")
		return
	}

	rows, err := h.deps.Mistra.QueryContext(r.Context(), `
		SELECT s.id, s.label, s.pipeline, s.display_order, p.label
		FROM loader.hubs_stages s
		LEFT JOIN loader.hubs_pipeline p ON p.id = s.pipeline
		WHERE s.pipeline = $1
		ORDER BY s.display_order NULLS LAST, s.label NULLS LAST, s.id`, cfg.PipelineID)
	if err != nil {
		h.dbFailure(w, r, "quote_stages", err)
		return
	}
	defer rows.Close()

	items := make([]stageSelection, 0)
	var pipelineLabel *string
	for rows.Next() {
		var (
			item         stageSelection
			label        sql.NullString
			pipelineID   string
			displayOrder sql.NullInt64
			joinedLabel  sql.NullString
		)
		if err := rows.Scan(&item.ID, &label, &pipelineID, &displayOrder, &joinedLabel); err != nil {
			h.dbFailure(w, r, "quote_stages_scan", err)
			return
		}
		item.Label = nullTrimmedStringPtr(label)
		item.PipelineID = pipelineID
		item.PipelineLabel = nullTrimmedStringPtr(joinedLabel)
		item.DisplayOrder = nullIntPtr(displayOrder)
		item.IsInitial = item.ID == cfg.InitialDealstageID
		if pipelineLabel == nil {
			pipelineLabel = item.PipelineLabel
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "quote_stages") {
		return
	}

	httputil.JSON(w, http.StatusOK, stagesResponse{
		PipelineID:         cfg.PipelineID,
		PipelineLabel:      pipelineLabel,
		InitialDealstageID: cfg.InitialDealstageID,
		Items:              items,
	})
}

func (h *Handler) handleQuoteDefaults(w http.ResponseWriter, r *http.Request) {
	if !h.requireConfigDB(w) {
		return
	}

	cfg, ok := h.readQuoteDefaultsConfig(r.Context())
	if !ok {
		httputil.Error(w, http.StatusServiceUnavailable, "raenad_config_not_configured")
		return
	}

	httputil.JSON(w, http.StatusOK, quoteDefaultsResponse{
		Line: quoteLineDefaults{
			CodIVA: cfg.CodIVA,
		},
	})
}

func (h *Handler) handleQuoteArticles(w http.ResponseWriter, r *http.Request) {
	if !h.requireAlyante(w) || !h.requireConfigDB(w) {
		return
	}

	cfg, ok := h.readQuoteDefaultsConfig(r.Context())
	if !ok {
		httputil.Error(w, http.StatusServiceUnavailable, "raenad_config_not_configured")
		return
	}

	search := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := parseReferenceLimit(r.URL.Query().Get("limit"))
	language := articleLanguage(r.URL.Query().Get("lang"))

	rows, err := h.deps.Alyante.QueryContext(r.Context(), alyanteArticleSearchQuery, search, "%"+search+"%", limit)
	if err != nil {
		h.dbFailure(w, r, "quote_articles", err)
		return
	}
	defer rows.Close()

	items := make([]articleLineInitializer, 0)
	for rows.Next() {
		var (
			code          sql.NullString
			descDefault   sql.NullString
			descItalian   sql.NullString
			descEnglish   sql.NullString
			extItalian    sql.NullString
			extEnglish    sql.NullString
			extDefault    sql.NullString
			unitOfMeasure sql.NullString
			unitPrice     sql.NullString
		)
		if err := rows.Scan(
			&code,
			&descDefault,
			&descItalian,
			&descEnglish,
			&extItalian,
			&extEnglish,
			&extDefault,
			&unitOfMeasure,
			&unitPrice,
		); err != nil {
			h.dbFailure(w, r, "quote_articles_scan", err)
			return
		}

		itemCode := strings.TrimSpace(code.String)
		if itemCode == "" {
			continue
		}
		shortDescription := localizedArticleText(language, descItalian, descEnglish, descDefault)
		longDescription := localizedArticleText(language, extItalian, extEnglish, extDefault)
		if longDescription == nil {
			longDescription = shortDescription
		}
		items = append(items, articleLineInitializer{
			LineType:          lineTypeItem,
			ItemCode:          itemCode,
			ItemDescription:   shortDescription,
			Description:       longDescription,
			UnitOfMeasure:     nullTrimmedStringPtr(unitOfMeasure),
			Qta:               nil,
			UnitPrice:         nullTrimmedStringPtr(unitPrice),
			Discounts:         nil,
			CodIVA:            cfg.CodIVA,
			PurchaseUnitPrice: nil,
		})
	}
	if !h.rowsDone(w, r, rows, "quote_articles") {
		return
	}

	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) readHubSpotDealPipelineConfig(ctx context.Context) (hubSpotDealPipelineConfig, bool) {
	var raw []byte
	err := h.deps.ConfigDB.QueryRowContext(ctx, `
		SELECT value
		FROM mrsmith.runtime_config
		WHERE namespace = $1 AND key = $2`, "raenad", "hubspot_deal_pipeline").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return hubSpotDealPipelineConfig{}, false
	}
	if err != nil {
		return hubSpotDealPipelineConfig{}, false
	}

	var cfg hubSpotDealPipelineConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return hubSpotDealPipelineConfig{}, false
	}
	cfg.PipelineID = strings.TrimSpace(cfg.PipelineID)
	cfg.InitialDealstageID = strings.TrimSpace(cfg.InitialDealstageID)
	if cfg.PipelineID == "" || cfg.InitialDealstageID == "" {
		return hubSpotDealPipelineConfig{}, false
	}
	return cfg, true
}

func (h *Handler) readQuoteDefaultsConfig(ctx context.Context) (quoteDefaultsConfig, bool) {
	var raw []byte
	err := h.deps.ConfigDB.QueryRowContext(ctx, `
		SELECT value
		FROM mrsmith.runtime_config
		WHERE namespace = $1 AND key = $2`, "aenad", "quote_defaults").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return quoteDefaultsConfig{}, false
	}
	if err != nil {
		return quoteDefaultsConfig{}, false
	}

	var cfg quoteDefaultsConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return quoteDefaultsConfig{}, false
	}
	cfg.CodIVA = strings.TrimSpace(cfg.CodIVA)
	if cfg.CodIVA == "" {
		return quoteDefaultsConfig{}, false
	}
	return cfg, true
}

func parseReferenceLimit(value string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || limit <= 0 {
		return defaultReferenceLimit
	}
	if limit > maxReferenceLimit {
		return maxReferenceLimit
	}
	return limit
}

func parseBoolQuery(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}

func articleLanguage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "en", "eng":
		return "eng"
	default:
		return "ita"
	}
}

func localizedArticleText(language string, italian, english, fallback sql.NullString) *string {
	if language == "eng" {
		return firstTrimmedStringPtr(english, fallback, italian)
	}
	return firstTrimmedStringPtr(italian, fallback, english)
}

func firstTrimmedStringPtr(values ...sql.NullString) *string {
	for _, value := range values {
		if out := nullTrimmedStringPtr(value); out != nil {
			return out
		}
	}
	return nil
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", component, "operation", operation}
	args = append(args, attrs...)
	httputil.InternalError(w, r, err, "database operation failed", args...)
}

func (h *Handler) rowsDone(w http.ResponseWriter, r *http.Request, rows *sql.Rows, operation string) bool {
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, operation+"_rows", err)
		return false
	}
	return true
}

func nullTrimmedStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullIntPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	converted := int(value.Int64)
	return &converted
}
