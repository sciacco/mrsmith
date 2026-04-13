package reports

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type aovRequest struct {
	DateFrom string   `json:"dateFrom"`
	DateTo   string   `json:"dateTo"`
	Statuses []string `json:"statuses"`
}

// ── AOV response types ──

type aovByType struct {
	Anno       *string  `json:"anno"`
	Mese       *string  `json:"mese"`
	TipoOrdine *string  `json:"tipo_ordine"`
	TotaleMRC  float64  `json:"totale_mrc"`
	TotaleNRC  float64  `json:"totale_nrc"`
	ValoreAOV  float64  `json:"valore_aov"`
}

type aovByCategory struct {
	Anno      *string  `json:"anno"`
	Mese      *string  `json:"mese"`
	Categoria *string  `json:"categoria"`
	TotaleMRC float64  `json:"totale_mrc"`
	TotaleNRC float64  `json:"totale_nrc"`
	ValoreAOV float64  `json:"valore_aov"`
}

type aovBySales struct {
	Anno        *string  `json:"anno"`
	Commerciale *string  `json:"commerciale"`
	TipoOrdine  *string  `json:"tipo_ordine"`
	TotaleMRC   float64  `json:"totale_mrc"`
	TotaleNRC   float64  `json:"totale_nrc"`
	ValoreAOV   float64  `json:"valore_aov"`
}

type aovDetail struct {
	TipoDocumento    *string  `json:"tipo_documento"`
	Anno             *string  `json:"anno"`
	Mese             *string  `json:"mese"`
	NomeTestataOrdine *string `json:"nome_testata_ordine"`
	TipoOrdine       *string  `json:"tipo_ordine"`
	SostOrd          *string  `json:"sost_ord"`
	Commerciale      *string  `json:"commerciale"`
	TotaleMRC        *float64 `json:"totale_mrc"`
	TotaleNRC        *float64 `json:"totale_nrc"`
	TotaleMRCOdvSost *float64 `json:"totale_mrc_odv_sost"`
	TotaleMRCNew     *float64 `json:"totale_mrc_new"`
	ValoreAOV        *float64 `json:"valore_aov"`
}

type aovPreviewResponse struct {
	ByType     []aovByType     `json:"byType"`
	ByCategory []aovByCategory `json:"byCategory"`
	BySales    []aovBySales    `json:"bySales"`
	Detail     []aovDetail     `json:"detail"`
}

// ── Query builders ──

func (h *Handler) buildAovArgs(req aovRequest) []any {
	args := make([]any, 0, len(req.Statuses)+2)
	for _, s := range req.Statuses {
		args = append(args, s)
	}
	args = append(args, req.DateFrom, req.DateTo)
	return args
}

func aovWhereClause(statusPlaceholders string, dateFromIdx, dateToIdx int) string {
	return fmt.Sprintf(`
where stato_ordine in (%s)
and (
CASE
	WHEN o.data_conferma ='0001-01-01 00:00:00' THEN data_ordine BETWEEN $%d AND $%d
	ELSE o.data_conferma BETWEEN $%d AND $%d
END
)`, statusPlaceholders, dateFromIdx, dateToIdx, dateFromIdx, dateToIdx)
}

// ── get_report_data_tipo_ord ──

func (h *Handler) queryAovByType(r *http.Request, req aovRequest) ([]aovByType, error) {
	statusPlaceholders, nextIdx := buildInClause(1, len(req.Statuses))
	where := aovWhereClause(statusPlaceholders, nextIdx, nextIdx+1)

	query := fmt.Sprintf(`SELECT anno, mese, tipo_ordine,
SUM(totale_mrc_new) AS totale_mrc,
sum(totale_nrc) AS totale_nrc,
sum(valore_aov) AS valore_aov

FROM (
SELECT
CASE
	WHEN o.data_conferma ='0001-01-01 00:00:00' THEN to_char(o.data_documento,'YYYY')
	ELSE to_char(o.data_conferma,'YYYY')
END
AS anno,
CASE
	WHEN o.data_conferma ='0001-01-01 00:00:00' THEN to_char(o.data_documento,'MM')
	ELSE to_char(o.data_conferma,'MM')
END
AS mese,
o.nome_testata_ordine,
CASE
     WHEN o.tipo_ordine = 'N'  THEN 'NUOVO'
     WHEN o.tipo_ordine = 'A'  THEN 'SOST'
     WHEN o.tipo_ordine = 'R'  THEN 'RINNOVO'
     WHEN o.tipo_ordine = 'C'  THEN 'CESSAZIONE'
     ELSE  ''
END
as tipo_ordine,
 o.sost_ord,
CASE
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN 0::decimal
	ELSE sum(round(o.quantita::decimal * o.canone::decimal,2))
END
as totale_mrc,

CASE
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN sum(round(o.quantita::decimal * o.canone::decimal,2))
	ELSE sum(round(o.quantita::decimal * o.setup::decimal,2))
END
as totale_nrc,
CASE
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN 0::decimal
	ELSE
		(SELECT sum(round(odv.quantita::decimal * odv.canone::decimal,2))
		FROM loader.v_ordini_ric_spot  AS odv WHERE
		( REPLACE(odv.nome_testata_ordine,'/','-') IN(REPLACE(o.sost_ord,'/','-')))
		AND odv.annullato = 0)
END
AS totale_mrc_odv_sost,

CASE
	WHEN o.tipo_ordine = 'A' THEN
		((sum(round(o.quantita::decimal * o.canone::decimal,2)))
		-
		(SELECT sum(round(odv.quantita::decimal * odv.canone::decimal,2))
		FROM loader.v_ordini_ric_spot  AS odv WHERE
		( REPLACE(odv.nome_testata_ordine,'/','-') IN(REPLACE(o.sost_ord,'/','-')))
		AND odv.annullato = 0)
		)
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN 0::decimal
	ELSE sum(round(o.quantita::decimal * o.canone::decimal,2))
END
AS  totale_mrc_new,

CASE
	WHEN o.tipo_ordine = 'A' THEN
		((sum(round(o.quantita::decimal * o.canone::decimal,2)))
		-
		(SELECT sum(round(odv.quantita::decimal * odv.canone::decimal,2))
		FROM loader.v_ordini_ric_spot  AS odv WHERE
		( REPLACE(odv.nome_testata_ordine,'/','-') IN(REPLACE(o.sost_ord,'/','-')))
		AND odv.annullato = 0)
		)*12 + sum(round(o.quantita::decimal * o.setup::decimal,2))
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN sum(round(o.quantita::decimal * o.canone::decimal,2))
	ELSE sum(round(o.quantita::decimal * o.setup::decimal,2)) + (sum(round(o.quantita::decimal * o.canone::decimal,2))*12)
END
AS  valore_aov

from loader.v_ordini_ric_spot as o join loader.erp_anagrafiche_clienti eac on o.numero_azienda = eac.numero_azienda

%s

GROUP BY
o.data_conferma,o.data_documento, o.tipo_ordine, o.nome_testata_ordine, o.sostituito_da, o.sost_ord, o.tipo_documento
ORDER BY o.tipo_ordine ASC
) AS x
GROUP BY anno, mese, tipo_ordine`, where)

	args := h.buildAovArgs(req)
	rows, err := h.mistraDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []aovByType
	for rows.Next() {
		var row aovByType
		var anno, mese, tipoOrdine sql.NullString
		if err := rows.Scan(&anno, &mese, &tipoOrdine, &row.TotaleMRC, &row.TotaleNRC, &row.ValoreAOV); err != nil {
			return nil, err
		}
		row.Anno = nullStringPtr(anno)
		row.Mese = nullStringPtr(mese)
		row.TipoOrdine = nullStringPtr(tipoOrdine)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []aovByType{}
	}
	return result, nil
}

// ── get_report_data_area ──

func (h *Handler) queryAovByCategory(r *http.Request, req aovRequest) ([]aovByCategory, error) {
	statusPlaceholders, nextIdx := buildInClause(1, len(req.Statuses))
	where := aovWhereClause(statusPlaceholders, nextIdx, nextIdx+1)

	query := fmt.Sprintf(`SELECT anno, mese, categoria,
SUM(totale_mrc) AS totale_mrc,
sum(totale_nrc) AS totale_nrc,
sum(valore_aov) AS valore_aov
FROM(
SELECT
o.tipo_documento,
CASE
	WHEN o.data_conferma ='0001-01-01 00:00:00' THEN to_char(o.data_documento,'YYYY')
	ELSE to_char(o.data_conferma,'YYYY')
END
AS anno,
CASE
	WHEN o.data_conferma ='0001-01-01 00:00:00' THEN to_char(o.data_documento,'MM')
	ELSE to_char(o.data_conferma,'MM')
END
AS mese,
o.nome_testata_ordine,
CASE
     WHEN o.tipo_ordine = 'N'  THEN 'NUOVO'
     WHEN o.tipo_ordine = 'A'  THEN 'SOST'
     WHEN o.tipo_ordine = 'R'  THEN 'RINNOVO'
     WHEN o.tipo_ordine = 'C'  THEN 'CESSAZIONE'
     ELSE  ''
END
as tipo_ordine,
 o.sost_ord,

CASE
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN 0::decimal
	ELSE round(o.quantita::decimal * o.canone::decimal,2)
END
as totale_mrc,

CASE
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN round(o.quantita::decimal * o.canone::decimal,2)
	ELSE round(o.quantita::decimal * o.setup::decimal,2)
END
as totale_nrc,
CASE
	WHEN o.tipo_ordine = 'A' THEN
		(((round(o.quantita::decimal * o.canone::decimal,2))))*12 + (round(o.quantita::decimal * o.setup::decimal,2))
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN (round(o.quantita::decimal * o.canone::decimal,2))
	ELSE (round(o.quantita::decimal * o.setup::decimal,2)) + ((round(o.quantita::decimal * o.canone::decimal,2))*12)
END
AS  valore_aov,
(select c.name from products.product_category as c
 		join products.product as p on c.id = p.category_id where p.code = o.codice_prodotto) AS categoria


from loader.v_ordini_ric_spot as o join loader.erp_anagrafiche_clienti eac on o.numero_azienda = eac.numero_azienda

%s

) AS x
GROUP BY anno, mese, categoria`, where)

	args := h.buildAovArgs(req)
	rows, err := h.mistraDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []aovByCategory
	for rows.Next() {
		var row aovByCategory
		var anno, mese, categoria sql.NullString
		if err := rows.Scan(&anno, &mese, &categoria, &row.TotaleMRC, &row.TotaleNRC, &row.ValoreAOV); err != nil {
			return nil, err
		}
		row.Anno = nullStringPtr(anno)
		row.Mese = nullStringPtr(mese)
		row.Categoria = nullStringPtr(categoria)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []aovByCategory{}
	}
	return result, nil
}

// ── get_report_data_sales ──

func (h *Handler) queryAovBySales(r *http.Request, req aovRequest) ([]aovBySales, error) {
	statusPlaceholders, nextIdx := buildInClause(1, len(req.Statuses))
	where := aovWhereClause(statusPlaceholders, nextIdx, nextIdx+1)

	query := fmt.Sprintf(`SELECT anno, commerciale,tipo_ordine,
SUM(totale_mrc_new) AS totale_mrc,
sum(totale_nrc) AS totale_nrc,
sum(valore_aov) AS valore_aov

FROM (
SELECT
CASE
	WHEN o.data_conferma ='0001-01-01 00:00:00' THEN to_char(o.data_documento,'YYYY')
	ELSE to_char(o.data_conferma,'YYYY')
END
AS anno,

CASE
     WHEN o.tipo_ordine = 'N'  THEN 'NUOVO'
     WHEN o.tipo_ordine = 'A'  THEN 'SOST'
     WHEN o.tipo_ordine = 'R'  THEN 'RINNOVO'
     WHEN o.tipo_ordine = 'C'  THEN 'CESSAZIONE'
     ELSE  ''
END
as tipo_ordine,
o.nome_testata_ordine,
coalesce((SELECT  concat(own.first_name, ' ', own.last_name)
	FROM loader.hubs_deal AS d JOIN loader.hubs_owner AS own ON d.hubspot_owner_id = own.id
	WHERE d.codice=o.nome_testata_ordine OR REPLACE(d.codice,'/','-') = o.nome_testata_ordine
),'CP') AS commerciale,
 o.sost_ord,
CASE
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN 0::decimal
	ELSE sum(round(o.quantita::decimal * o.canone::decimal,2))
END
as totale_mrc,

CASE
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN sum(round(o.quantita::decimal * o.canone::decimal,2))
	ELSE sum(round(o.quantita::decimal * o.setup::decimal,2))
END
as totale_nrc,
CASE
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN 0::decimal
	ELSE
		(SELECT sum(round(odv.quantita::decimal * odv.canone::decimal,2))
		FROM loader.v_ordini_ric_spot  AS odv WHERE
		( REPLACE(odv.nome_testata_ordine,'/','-') IN(REPLACE(o.sost_ord,'/','-')))
		AND odv.annullato = 0)
END
AS totale_mrc_odv_sost,
CASE
	WHEN o.tipo_ordine = 'A' THEN
		((sum(round(o.quantita::decimal * o.canone::decimal,2)))
		-
		(SELECT sum(round(odv.quantita::decimal * odv.canone::decimal,2))
		FROM loader.v_ordini_ric_spot  AS odv WHERE
		( REPLACE(odv.nome_testata_ordine,'/','-') IN(REPLACE(o.sost_ord,'/','-')))
		AND odv.annullato = 0)
		)
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN 0::decimal
	ELSE sum(round(o.quantita::decimal * o.canone::decimal,2))
END
AS  totale_mrc_new,
CASE
	WHEN o.tipo_ordine = 'A' THEN
		((sum(round(o.quantita::decimal * o.canone::decimal,2)))
		-
		(SELECT sum(round(odv.quantita::decimal * odv.canone::decimal,2))
		FROM loader.v_ordini_ric_spot  AS odv WHERE
		( REPLACE(odv.nome_testata_ordine,'/','-') IN(REPLACE(o.sost_ord,'/','-')))
		AND odv.annullato = 0)
		)*12 + sum(round(o.quantita::decimal * o.setup::decimal,2))
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN sum(round(o.quantita::decimal * o.canone::decimal,2))
	ELSE sum(round(o.quantita::decimal * o.setup::decimal,2)) + (sum(round(o.quantita::decimal * o.canone::decimal,2))*12)
END
AS  valore_aov


from loader.v_ordini_ric_spot as o join loader.erp_anagrafiche_clienti eac on o.numero_azienda = eac.numero_azienda

%s

GROUP BY
o.data_conferma,o.data_documento, o.tipo_ordine, o.nome_testata_ordine, o.sostituito_da, o.sost_ord, o.tipo_documento
ORDER BY o.tipo_ordine ASC
) AS x
GROUP BY anno, commerciale, tipo_ordine`, where)

	args := h.buildAovArgs(req)
	rows, err := h.mistraDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []aovBySales
	for rows.Next() {
		var row aovBySales
		var anno, commerciale, tipoOrdine sql.NullString
		if err := rows.Scan(&anno, &commerciale, &tipoOrdine, &row.TotaleMRC, &row.TotaleNRC, &row.ValoreAOV); err != nil {
			return nil, err
		}
		row.Anno = nullStringPtr(anno)
		row.Commerciale = nullStringPtr(commerciale)
		row.TipoOrdine = nullStringPtr(tipoOrdine)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []aovBySales{}
	}
	return result, nil
}

// ── get_report_data (AOV detail) ──

func (h *Handler) queryAovDetail(r *http.Request, req aovRequest) ([]aovDetail, error) {
	statusPlaceholders, nextIdx := buildInClause(1, len(req.Statuses))
	where := aovWhereClause(statusPlaceholders, nextIdx, nextIdx+1)

	query := fmt.Sprintf(`SELECT
o.tipo_documento,
CASE
	WHEN o.data_conferma ='0001-01-01 00:00:00' THEN to_char(o.data_documento,'YYYY')
	ELSE to_char(o.data_conferma,'YYYY')
END
AS anno,
CASE
	WHEN o.data_conferma ='0001-01-01 00:00:00' THEN to_char(o.data_documento,'MM')
	ELSE to_char(o.data_conferma,'MM')
END
AS mese,
o.nome_testata_ordine,
CASE
     WHEN o.tipo_ordine = 'N'  THEN 'NUOVO'
     WHEN o.tipo_ordine = 'A'  THEN 'SOST'
     WHEN o.tipo_ordine = 'R'  THEN 'RINNOVO'
     WHEN o.tipo_ordine = 'C'  THEN 'CESSAZIONE'
     ELSE  ''
END
as tipo_ordine,
 o.sost_ord,
 coalesce((SELECT  concat(own.first_name, ' ', own.last_name)
	FROM loader.hubs_deal AS d JOIN loader.hubs_owner AS own ON d.hubspot_owner_id = own.id
	WHERE d.codice=o.nome_testata_ordine OR REPLACE(d.codice,'/','-') = o.nome_testata_ordine
),'CP') AS commerciale,
CASE
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN 0::decimal
	ELSE sum(round(o.quantita::decimal * o.canone::decimal,2))
END
as totale_mrc,

CASE
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN sum(round(o.quantita::decimal * o.canone::decimal,2))
	ELSE sum(round(o.quantita::decimal * o.setup::decimal,2))
END
as totale_nrc,

CASE
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN 0::decimal
	ELSE
		(SELECT sum(round(odv.quantita::decimal * odv.canone::decimal,2))
		FROM loader.v_ordini_ric_spot  AS odv WHERE
		( REPLACE(odv.nome_testata_ordine,'/','-') IN(REPLACE(o.sost_ord,'/','-')))
		AND odv.annullato = 0)
END
AS totale_mrc_odv_sost,

CASE
	WHEN o.tipo_ordine = 'A' THEN
		((sum(round(o.quantita::decimal * o.canone::decimal,2)))
		-
		(SELECT sum(round(odv.quantita::decimal * odv.canone::decimal,2))
		FROM loader.v_ordini_ric_spot  AS odv WHERE
		( REPLACE(odv.nome_testata_ordine,'/','-') IN(REPLACE(o.sost_ord,'/','-')))
		AND odv.annullato = 0)
		)
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN 0::decimal
	ELSE sum(round(o.quantita::decimal * o.canone::decimal,2))
END
AS  totale_mrc_new,

CASE
	WHEN o.tipo_ordine = 'A' THEN
		((sum(round(o.quantita::decimal * o.canone::decimal,2)))
		-
		(SELECT sum(round(odv.quantita::decimal * odv.canone::decimal,2))
		FROM loader.v_ordini_ric_spot  AS odv WHERE
		( REPLACE(odv.nome_testata_ordine,'/','-') IN(REPLACE(o.sost_ord,'/','-')))
		AND odv.annullato = 0)
		)*12 + sum(round(o.quantita::decimal * o.setup::decimal,2))
	WHEN trim(o.tipo_documento) = 'TSC-ORDINE' THEN sum(round(o.quantita::decimal * o.canone::decimal,2))
	ELSE sum(round(o.quantita::decimal * o.setup::decimal,2)) + (sum(round(o.quantita::decimal * o.canone::decimal,2))*12)
END
AS  valore_aov



from loader.v_ordini_ric_spot as o join loader.erp_anagrafiche_clienti eac on o.numero_azienda = eac.numero_azienda

%s

GROUP BY
o.data_conferma,o.data_documento, o.tipo_ordine, o.nome_testata_ordine, o.sostituito_da, o.sost_ord, o.tipo_documento
ORDER BY o.tipo_ordine ASC`, where)

	args := h.buildAovArgs(req)
	rows, err := h.mistraDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []aovDetail
	for rows.Next() {
		var row aovDetail
		var (
			tipoDocumento, anno, mese    sql.NullString
			nomeTestataOrdine, tipoOrd   sql.NullString
			sostOrd, commerciale         sql.NullString
			totaleMRC, totaleNRC         sql.NullFloat64
			totaleMRCOdvSost             sql.NullFloat64
			totaleMRCNew, valoreAOV      sql.NullFloat64
		)
		if err := rows.Scan(
			&tipoDocumento, &anno, &mese,
			&nomeTestataOrdine, &tipoOrd, &sostOrd, &commerciale,
			&totaleMRC, &totaleNRC, &totaleMRCOdvSost,
			&totaleMRCNew, &valoreAOV,
		); err != nil {
			return nil, err
		}
		row.TipoDocumento = nullStringPtr(tipoDocumento)
		row.Anno = nullStringPtr(anno)
		row.Mese = nullStringPtr(mese)
		row.NomeTestataOrdine = nullStringPtr(nomeTestataOrdine)
		row.TipoOrdine = nullStringPtr(tipoOrd)
		row.SostOrd = nullStringPtr(sostOrd)
		row.Commerciale = nullStringPtr(commerciale)
		row.TotaleMRC = nullFloat64Ptr(totaleMRC)
		row.TotaleNRC = nullFloat64Ptr(totaleNRC)
		row.TotaleMRCOdvSost = nullFloat64Ptr(totaleMRCOdvSost)
		row.TotaleMRCNew = nullFloat64Ptr(totaleMRCNew)
		row.ValoreAOV = nullFloat64Ptr(valoreAOV)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []aovDetail{}
	}
	return result, nil
}

// ── HTTP handlers ──

func (h *Handler) decodeAovRequest(w http.ResponseWriter, r *http.Request) (aovRequest, bool) {
	var req aovRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request_body")
		return req, false
	}
	if len(req.Statuses) == 0 || req.DateFrom == "" || req.DateTo == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_required_fields")
		return req, false
	}
	return req, true
}

// handleAovPreview returns all 4 AOV datasets as a single JSON response.
// POST /reports/v1/aov/preview
func (h *Handler) handleAovPreview(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	req, ok := h.decodeAovRequest(w, r)
	if !ok {
		return
	}

	byType, err := h.queryAovByType(r, req)
	if err != nil {
		h.dbFailure(w, r, "aov_by_type", err)
		return
	}

	byCategory, err := h.queryAovByCategory(r, req)
	if err != nil {
		h.dbFailure(w, r, "aov_by_category", err)
		return
	}

	bySales, err := h.queryAovBySales(r, req)
	if err != nil {
		h.dbFailure(w, r, "aov_by_sales", err)
		return
	}

	detail, err := h.queryAovDetail(r, req)
	if err != nil {
		h.dbFailure(w, r, "aov_detail", err)
		return
	}

	httputil.JSON(w, http.StatusOK, aovPreviewResponse{
		ByType:     byType,
		ByCategory: byCategory,
		BySales:    bySales,
		Detail:     detail,
	})
}

// handleAovExport generates an XLSX export of AOV data (by-type) via Carbone.
// POST /reports/v1/aov/export
func (h *Handler) handleAovExport(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}
	if h.carbone == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "carbone_not_configured")
		return
	}

	req, ok := h.decodeAovRequest(w, r)
	if !ok {
		return
	}

	result, err := h.queryAovByType(r, req)
	if err != nil {
		h.dbFailure(w, r, "aov_export", err)
		return
	}

	xlsxBytes, err := h.carbone.GenerateXLSX(r.Context(), OrdiniTemplateID, result)
	if err != nil {
		h.dbFailure(w, r, "aov_export_carbone", err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="report_aov.xlsx"`)
	w.Write(xlsxBytes)
}
