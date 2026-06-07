package aenad

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const (
	defaultTipoDoc         = "Q"
	defaultDocumentDays    = 100
	maxDocumentDays        = 380
	defaultDocumentsPage   = 1
	defaultDocumentsSize   = 50
	maxDocumentsPageSize   = 100
	dateLayout             = "2006-01-02"
	defaultTipoDocSortRank = 999999
)

var (
	errTipoDocRequired  = errors.New("tipo_doc_required")
	errInvalidDate      = errors.New("invalid_date")
	errInvalidDateRange = errors.New("invalid_date_range")
	errInvalidPage      = errors.New("invalid_page")
	errInvalidPageSize  = errors.New("invalid_page_size")
)

type documentTypeOption struct {
	TipoDoc string `json:"tipoDoc"`
	Label   string `json:"label"`
}

type documentListFilters struct {
	TipoDoc  string
	DateFrom string
	DateTo   string
	Page     int
	PageSize int
	Offset   int
}

type documentListResponse struct {
	Items    []documentRow `json:"items"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

type documentRow struct {
	IDDoc             int     `json:"IDDoc"`
	TipoDoc           *string `json:"TipoDoc"`
	IDAnagr           *int    `json:"IDAnagr"`
	CodDestIDAnagr    *int    `json:"CodDest_IDAnagr"`
	CodDest           *string `json:"CodDest"`
	Data              *string `json:"Data"`
	Num               *int    `json:"Num"`
	DataDoc           *string `json:"DataDoc"`
	NumDoc            *string `json:"NumDoc"`
	DescDoc           *string `json:"DescDoc"`
	TotNetto          *int64  `json:"TotNetto"`
	TotDoc            *int64  `json:"TotDoc"`
	TotPrezzoAcquisto *int64  `json:"TotPrezzoAcquisto"`
	TotGuadagno       *int64  `json:"TotGuadagno"`
}

func (h *Handler) handleDocumentTypes(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.mistra.QueryContext(r.Context(), `
		SELECT
			"TipoDoc",
			COALESCE(NULLIF(TRIM("NomeBreve"), ''), NULLIF(TRIM("Nome"), ''), NULLIF(TRIM("NomePlurale"), ''), "TipoDoc") AS label
		FROM aenad."TTipiDoc"
		WHERE COALESCE("Visibile", 0) = 1
		ORDER BY COALESCE("Ordinam", $1), label, "TipoDoc"`, defaultTipoDocSortRank)
	if err != nil {
		h.dbFailure(w, r, "document_types", err)
		return
	}
	defer rows.Close()

	items := make([]documentTypeOption, 0)
	for rows.Next() {
		var item documentTypeOption
		if err := rows.Scan(&item.TipoDoc, &item.Label); err != nil {
			h.dbFailure(w, r, "document_types_scan", err)
			return
		}
		item.TipoDoc = strings.TrimSpace(item.TipoDoc)
		item.Label = strings.TrimSpace(item.Label)
		if item.Label == "" {
			item.Label = item.TipoDoc
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "document_types_rows", err)
		return
	}

	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleDocuments(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	filters, err := parseDocumentListFilters(r, time.Now())
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var total int
	if err := h.mistra.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM aenad."TDocTestate" d
		WHERE d."TipoDoc" = $1
		  AND d."Data" >= $2::date
		  AND d."Data" <= $3::date`,
		filters.TipoDoc, filters.DateFrom, filters.DateTo,
	).Scan(&total); err != nil {
		h.dbFailure(w, r, "documents_count", err, "tipo_doc", filters.TipoDoc)
		return
	}

	rows, err := h.mistra.QueryContext(r.Context(), `
		SELECT
			d."IDDoc",
			d."TipoDoc",
			d."IDAnagr",
			d."CodDest_IDAnagr",
			d."CodDest",
			d."Data",
			d."Num",
			d."DataDoc",
			d."NumDoc",
			d."DescDoc",
			d."TotNetto",
			d."TotDoc",
			d."TotPrezzoAcquisto",
			d."TotGuadagno"
		FROM aenad."TDocTestate" d
		WHERE d."TipoDoc" = $1
		  AND d."Data" >= $2::date
		  AND d."Data" <= $3::date
		ORDER BY d."Data" DESC, d."IDDoc" DESC
		LIMIT $4 OFFSET $5`,
		filters.TipoDoc, filters.DateFrom, filters.DateTo, filters.PageSize, filters.Offset,
	)
	if err != nil {
		h.dbFailure(w, r, "documents", err, "tipo_doc", filters.TipoDoc)
		return
	}
	defer rows.Close()

	items := make([]documentRow, 0)
	for rows.Next() {
		item, err := scanDocumentRow(rows)
		if err != nil {
			h.dbFailure(w, r, "documents_scan", err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "documents_rows", err)
		return
	}

	httputil.JSON(w, http.StatusOK, documentListResponse{
		Items:    items,
		Total:    total,
		Page:     filters.Page,
		PageSize: filters.PageSize,
	})
}

func parseDocumentListFilters(r *http.Request, now time.Time) (documentListFilters, error) {
	q := r.URL.Query()

	tipoDoc := strings.TrimSpace(q.Get("tipoDoc"))
	if tipoDoc == "" {
		tipoDoc = defaultTipoDoc
	}
	if tipoDoc == "" {
		return documentListFilters{}, errTipoDocRequired
	}

	today := dateOnly(now)
	defaultTo := today
	defaultFrom := today.AddDate(0, 0, -(defaultDocumentDays - 1))

	dateFrom, err := parseQueryDate(q.Get("dateFrom"), defaultFrom)
	if err != nil {
		return documentListFilters{}, errInvalidDate
	}
	dateTo, err := parseQueryDate(q.Get("dateTo"), defaultTo)
	if err != nil {
		return documentListFilters{}, errInvalidDate
	}
	if dateFrom.After(dateTo) {
		return documentListFilters{}, errInvalidDateRange
	}
	if inclusiveDays(dateFrom, dateTo) > maxDocumentDays {
		return documentListFilters{}, errInvalidDateRange
	}

	page, err := parsePositiveQueryInt(q.Get("page"), defaultDocumentsPage)
	if err != nil {
		return documentListFilters{}, errInvalidPage
	}
	pageSize, err := parsePositiveQueryInt(q.Get("pageSize"), defaultDocumentsSize)
	if err != nil {
		return documentListFilters{}, errInvalidPageSize
	}
	if pageSize > maxDocumentsPageSize {
		return documentListFilters{}, errInvalidPageSize
	}

	return documentListFilters{
		TipoDoc:  tipoDoc,
		DateFrom: dateFrom.Format(dateLayout),
		DateTo:   dateTo.Format(dateLayout),
		Page:     page,
		PageSize: pageSize,
		Offset:   (page - 1) * pageSize,
	}, nil
}

func parseQueryDate(raw string, fallback time.Time) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func parsePositiveQueryInt(raw string, fallback int) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, errInvalidPage
	}
	return parsed, nil
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func inclusiveDays(from time.Time, to time.Time) int {
	return int(to.Sub(from).Hours()/24) + 1
}

func scanDocumentRow(rows *sql.Rows) (documentRow, error) {
	var item documentRow
	var tipoDoc, codDest, numDoc, descDoc sql.NullString
	var idAnagr, codDestIDAnagr, num sql.NullInt64
	var data, dataDoc sql.NullTime
	var totNetto, totDoc, totPrezzoAcquisto, totGuadagno sql.NullInt64

	if err := rows.Scan(
		&item.IDDoc,
		&tipoDoc,
		&idAnagr,
		&codDestIDAnagr,
		&codDest,
		&data,
		&num,
		&dataDoc,
		&numDoc,
		&descDoc,
		&totNetto,
		&totDoc,
		&totPrezzoAcquisto,
		&totGuadagno,
	); err != nil {
		return documentRow{}, err
	}

	item.TipoDoc = nullableString(tipoDoc)
	item.IDAnagr = nullableInt(idAnagr)
	item.CodDestIDAnagr = nullableInt(codDestIDAnagr)
	item.CodDest = nullableString(codDest)
	item.Data = nullableDate(data)
	item.Num = nullableInt(num)
	item.DataDoc = nullableDate(dataDoc)
	item.NumDoc = nullableString(numDoc)
	item.DescDoc = nullableString(descDoc)
	item.TotNetto = nullableInt64(totNetto)
	item.TotDoc = nullableInt64(totDoc)
	item.TotPrezzoAcquisto = nullableInt64(totPrezzoAcquisto)
	item.TotGuadagno = nullableInt64(totGuadagno)

	return item, nil
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullableInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func nullableInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}

func nullableDate(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.Format(dateLayout)
	return &formatted
}
