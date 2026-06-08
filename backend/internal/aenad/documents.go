package aenad

import (
	"database/sql"
	"encoding/json"
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
	IDDoc               int     `json:"IDDoc"`
	TipoDoc             *string `json:"TipoDoc"`
	IDAnagr             *int    `json:"IDAnagr"`
	AnagrNome           *string `json:"Anagr_Nome"`
	CodDestIDAnagr      *int    `json:"CodDest_IDAnagr"`
	CodDest             *string `json:"CodDest"`
	Data                *string `json:"Data"`
	Num                 *int    `json:"Num"`
	DataDoc             *string `json:"DataDoc"`
	NumDoc              *string `json:"NumDoc"`
	DescDoc             *string `json:"DescDoc"`
	TotNetto            *int64  `json:"TotNetto"`
	TotDoc              *int64  `json:"TotDoc"`
	TotPrezzoAcquisto   *int64  `json:"TotPrezzoAcquisto"`
	TotGuadagno         *int64  `json:"TotGuadagno"`
	Pagamento           *string `json:"Pagamento"`
	PagamCoordBancarie  *string `json:"Pagam_CoordBancarie"`
	NoteInterne         *string `json:"NoteInterne"`
	AnagrIndirizzo      *string `json:"Anagr_Indirizzo"`
	AnagrCap            *string `json:"Anagr_Cap"`
	AnagrCitta          *string `json:"Anagr_Citta"`
	AnagrProv           *string `json:"Anagr_Prov"`
	AnagrNazione        *string `json:"Anagr_Nazione"`
	AnagrCodiceFiscale  *string `json:"Anagr_CodiceFiscale"`
	AnagrPartitaIva     *string `json:"Anagr_PartitaIva"`
	AnagrDestNome       *string `json:"Anagr_DestNome"`
	AnagrDestIndirizzo  *string `json:"Anagr_DestIndirizzo"`
	AnagrDestCap        *string `json:"Anagr_DestCap"`
	AnagrDestCitta      *string `json:"Anagr_DestCitta"`
	AnagrDestProv       *string `json:"Anagr_DestProv"`
	AnagrDestNazione    *string `json:"Anagr_DestNazione"`
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
			d."Anagr_Nome",
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
			d."TotGuadagno",
			d."Pagamento",
			d."Pagam_CoordBancarie",
			d."NoteInterne",
			d."Anagr_Indirizzo",
			d."Anagr_Cap",
			d."Anagr_Citta",
			d."Anagr_Prov",
			d."Anagr_Nazione",
			d."Anagr_CodiceFiscale",
			d."Anagr_PartitaIva",
			d."Anagr_DestNome",
			d."Anagr_DestIndirizzo",
			d."Anagr_DestCap",
			d."Anagr_DestCitta",
			d."Anagr_DestProv",
			d."Anagr_DestNazione"
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
	var tipoDoc, codDest, numDoc, descDoc, anagrNome sql.NullString
	var pagamento, pagamCoordBancarie, noteInterne, anagrIndirizzo, anagrCap, anagrCitta, anagrProv, anagrNazione, anagrCodiceFiscale, anagrPartitaIva, anagrDestNome, anagrDestIndirizzo, anagrDestCap, anagrDestCitta, anagrDestProv, anagrDestNazione sql.NullString
	var idAnagr, codDestIDAnagr, num sql.NullInt64
	var data, dataDoc sql.NullTime
	var totNetto, totDoc, totPrezzoAcquisto, totGuadagno sql.NullInt64

	if err := rows.Scan(
		&item.IDDoc,
		&tipoDoc,
		&idAnagr,
		&anagrNome,
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
		&pagamento,
		&pagamCoordBancarie,
		&noteInterne,
		&anagrIndirizzo,
		&anagrCap,
		&anagrCitta,
		&anagrProv,
		&anagrNazione,
		&anagrCodiceFiscale,
		&anagrPartitaIva,
		&anagrDestNome,
		&anagrDestIndirizzo,
		&anagrDestCap,
		&anagrDestCitta,
		&anagrDestProv,
		&anagrDestNazione,
	); err != nil {
		return documentRow{}, err
	}

	item.TipoDoc = nullableString(tipoDoc)
	item.IDAnagr = nullableInt(idAnagr)
	item.AnagrNome = nullableString(anagrNome)
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
	item.Pagamento = nullableString(pagamento)
	item.PagamCoordBancarie = nullableString(pagamCoordBancarie)
	item.NoteInterne = nullableString(noteInterne)
	item.AnagrIndirizzo = nullableString(anagrIndirizzo)
	item.AnagrCap = nullableString(anagrCap)
	item.AnagrCitta = nullableString(anagrCitta)
	item.AnagrProv = nullableString(anagrProv)
	item.AnagrNazione = nullableString(anagrNazione)
	item.AnagrCodiceFiscale = nullableString(anagrCodiceFiscale)
	item.AnagrPartitaIva = nullableString(anagrPartitaIva)
	item.AnagrDestNome = nullableString(anagrDestNome)
	item.AnagrDestIndirizzo = nullableString(anagrDestIndirizzo)
	item.AnagrDestCap = nullableString(anagrDestCap)
	item.AnagrDestCitta = nullableString(anagrDestCitta)
	item.AnagrDestProv = nullableString(anagrDestProv)
	item.AnagrDestNazione = nullableString(anagrDestNazione)

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

func (h *Handler) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	idStr := r.PathValue("id")
	idDoc, err := strconv.Atoi(idStr)
	if err != nil || idDoc <= 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_document_id")
		return
	}

	var updatedRow documentRow
	rowQuery := h.mistra.QueryRowContext(r.Context(), `
		SELECT
			d."IDDoc", d."TipoDoc", d."IDAnagr", d."Anagr_Nome", d."CodDest_IDAnagr", d."CodDest",
			d."Data", d."Num", d."DataDoc", d."NumDoc", d."DescDoc", d."TotNetto", d."TotDoc",
			d."TotPrezzoAcquisto", d."TotGuadagno", d."Pagamento", d."Pagam_CoordBancarie",
			d."NoteInterne", d."Anagr_Indirizzo", d."Anagr_Cap", d."Anagr_Citta", d."Anagr_Prov",
			d."Anagr_Nazione", d."Anagr_CodiceFiscale", d."Anagr_PartitaIva", d."Anagr_DestNome",
			d."Anagr_DestIndirizzo", d."Anagr_DestCap", d."Anagr_DestCitta", d."Anagr_DestProv",
			d."Anagr_DestNazione"
		FROM aenad."TDocTestate" d
		WHERE d."IDDoc" = $1`, idDoc)

	var tipoDoc, codDest, numDoc, descDoc, anagrNome sql.NullString
	var pagamento, pagamCoordBancarie, noteInterne, anagrIndirizzo, anagrCap, anagrCitta, anagrProv, anagrNazione, anagrCodiceFiscale, anagrPartitaIva, anagrDestNome, anagrDestIndirizzo, anagrDestCap, anagrDestCitta, anagrDestProv, anagrDestNazione sql.NullString
	var idAnagr, codDestIDAnagr, num sql.NullInt64
	var data, dataDoc sql.NullTime
	var totNetto, totDoc, totPrezzoAcquisto, totGuadagno sql.NullInt64

	err = rowQuery.Scan(
		&updatedRow.IDDoc, &tipoDoc, &idAnagr, &anagrNome, &codDestIDAnagr, &codDest,
		&data, &num, &dataDoc, &numDoc, &descDoc, &totNetto, &totDoc,
		&totPrezzoAcquisto, &totGuadagno, &pagamento, &pagamCoordBancarie,
		&noteInterne, &anagrIndirizzo, &anagrCap, &anagrCitta, &anagrProv,
		&anagrNazione, &anagrCodiceFiscale, &anagrPartitaIva, &anagrDestNome,
		&anagrDestIndirizzo, &anagrDestCap, &anagrDestCitta, &anagrDestProv,
		&anagrDestNazione,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "document_not_found")
			return
		}
		h.dbFailure(w, r, "query_document", err, "id_doc", idDoc)
		return
	}

	updatedRow.TipoDoc = nullableString(tipoDoc)
	updatedRow.IDAnagr = nullableInt(idAnagr)
	updatedRow.AnagrNome = nullableString(anagrNome)
	updatedRow.CodDestIDAnagr = nullableInt(codDestIDAnagr)
	updatedRow.CodDest = nullableString(codDest)
	updatedRow.Data = nullableDate(data)
	updatedRow.Num = nullableInt(num)
	updatedRow.DataDoc = nullableDate(dataDoc)
	updatedRow.NumDoc = nullableString(numDoc)
	updatedRow.DescDoc = nullableString(descDoc)
	updatedRow.TotNetto = nullableInt64(totNetto)
	updatedRow.TotDoc = nullableInt64(totDoc)
	updatedRow.TotPrezzoAcquisto = nullableInt64(totPrezzoAcquisto)
	updatedRow.TotGuadagno = nullableInt64(totGuadagno)
	updatedRow.Pagamento = nullableString(pagamento)
	updatedRow.PagamCoordBancarie = nullableString(pagamCoordBancarie)
	updatedRow.NoteInterne = nullableString(noteInterne)
	updatedRow.AnagrIndirizzo = nullableString(anagrIndirizzo)
	updatedRow.AnagrCap = nullableString(anagrCap)
	updatedRow.AnagrCitta = nullableString(anagrCitta)
	updatedRow.AnagrProv = nullableString(anagrProv)
	updatedRow.AnagrNazione = nullableString(anagrNazione)
	updatedRow.AnagrCodiceFiscale = nullableString(anagrCodiceFiscale)
	updatedRow.AnagrPartitaIva = nullableString(anagrPartitaIva)
	updatedRow.AnagrDestNome = nullableString(anagrDestNome)
	updatedRow.AnagrDestIndirizzo = nullableString(anagrDestIndirizzo)
	updatedRow.AnagrDestCap = nullableString(anagrDestCap)
	updatedRow.AnagrDestCitta = nullableString(anagrDestCitta)
	updatedRow.AnagrDestProv = nullableString(anagrDestProv)
	updatedRow.AnagrDestNazione = nullableString(anagrDestNazione)

	httputil.JSON(w, http.StatusOK, updatedRow)
}

type documentRowDetail struct {
	IDDocRiga        int     `json:"IDDocRiga"`
	IDDoc            int     `json:"IDDoc"`
	CodArticolo      *string `json:"CodArticolo"`
	Desc             *string `json:"Desc"`
	Qta              *int64  `json:"Qta"`
	Udm              *string `json:"Udm"`
	PrezzoNetto      *int64  `json:"PrezzoNetto"`
	Sconti           *string `json:"Sconti"`
	ImportoNettoRiga *int64  `json:"ImportoNettoRiga"`
}

func (h *Handler) handleDocumentRows(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	idStr := r.PathValue("id")
	idDoc, err := strconv.Atoi(idStr)
	if err != nil || idDoc <= 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_document_id")
		return
	}

	rows, err := h.mistra.QueryContext(r.Context(), `
		SELECT
			"IDDocRiga",
			"IDDoc",
			"CodArticolo",
			"Desc",
			"Qta",
			"Udm",
			"PrezzoNetto",
			"Sconti",
			"ImportoNettoRiga"
		FROM aenad."TDocRighe"
		WHERE "IDDoc" = $1
		  AND ("CodArticolo" IS NOT NULL OR "Desc" IS NOT NULL OR "Qta" IS NOT NULL OR "ImportoNettoRiga" IS NOT NULL)
		ORDER BY "IDDocRiga" ASC`, idDoc)
	if err != nil {
		h.dbFailure(w, r, "document_rows", err, "id_doc", idDoc)
		return
	}
	defer rows.Close()

	items := make([]documentRowDetail, 0)
	for rows.Next() {
		var item documentRowDetail
		var codArticolo, desc, udm, sconti sql.NullString
		var qta, prezzoNetto, importoNettoRiga sql.NullInt64

		if err := rows.Scan(
			&item.IDDocRiga,
			&item.IDDoc,
			&codArticolo,
			&desc,
			&qta,
			&udm,
			&prezzoNetto,
			&sconti,
			&importoNettoRiga,
		); err != nil {
			h.dbFailure(w, r, "document_rows_scan", err)
			return
		}

		item.CodArticolo = nullableString(codArticolo)
		item.Desc = nullableString(desc)
		item.Qta = nullableInt64(qta)
		item.Udm = nullableString(udm)
		item.PrezzoNetto = nullableInt64(prezzoNetto)
		item.Sconti = nullableString(sconti)
		item.ImportoNettoRiga = nullableInt64(importoNettoRiga)

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "document_rows_loop", err)
		return
	}

	httputil.JSON(w, http.StatusOK, items)
}

type updateDocumentRequest struct {
	AnagrNome           *string                 `json:"Anagr_Nome"`
	AnagrIndirizzo      *string                 `json:"Anagr_Indirizzo"`
	AnagrCap            *string                 `json:"Anagr_Cap"`
	AnagrCitta          *string                 `json:"Anagr_Citta"`
	AnagrProv           *string                 `json:"Anagr_Prov"`
	AnagrNazione        *string                 `json:"Anagr_Nazione"`
	AnagrCodiceFiscale  *string                 `json:"Anagr_CodiceFiscale"`
	AnagrPartitaIva     *string                 `json:"Anagr_PartitaIva"`
	AnagrDestNome       *string                 `json:"Anagr_DestNome"`
	AnagrDestIndirizzo  *string                 `json:"Anagr_DestIndirizzo"`
	AnagrDestCap        *string                 `json:"Anagr_DestCap"`
	AnagrDestCitta      *string                 `json:"Anagr_DestCitta"`
	AnagrDestProv       *string                 `json:"Anagr_DestProv"`
	AnagrDestNazione    *string                 `json:"Anagr_DestNazione"`
	Pagamento           *string                 `json:"Pagamento"`
	PagamCoordBancarie  *string                 `json:"Pagam_CoordBancarie"`
	NoteInterne         *string                 `json:"NoteInterne"`
	DescDoc             *string                 `json:"DescDoc"`
	DataDoc             *string                 `json:"DataDoc"`
	NumDoc              *string                 `json:"NumDoc"`
	Rows                []updateDocumentRowItem `json:"Rows"`
}

type updateDocumentRowItem struct {
	IDDocRiga   int     `json:"IDDocRiga"`
	CodArticolo *string `json:"CodArticolo"`
	Desc        *string `json:"Desc"`
	Qta         *int64  `json:"Qta"`
	Udm         *string `json:"Udm"`
	PrezzoNetto *int64  `json:"PrezzoNetto"`
	Sconti      *string `json:"Sconti"`
}

func (h *Handler) handleUpdateDocument(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	idStr := r.PathValue("id")
	idDoc, err := strconv.Atoi(idStr)
	if err != nil || idDoc <= 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_document_id")
		return
	}

	var req updateDocumentRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json_body")
		return
	}

	tx, err := h.mistra.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "begin_tx", err)
		return
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 1. Update TDocTestate
	_, err = tx.ExecContext(r.Context(), `
		UPDATE aenad."TDocTestate" SET
			"Anagr_Nome" = $1,
			"Anagr_Indirizzo" = $2,
			"Anagr_Cap" = $3,
			"Anagr_Citta" = $4,
			"Anagr_Prov" = $5,
			"Anagr_Nazione" = $6,
			"Anagr_CodiceFiscale" = $7,
			"Anagr_PartitaIva" = $8,
			"Anagr_DestNome" = $9,
			"Anagr_DestIndirizzo" = $10,
			"Anagr_DestCap" = $11,
			"Anagr_DestCitta" = $12,
			"Anagr_DestProv" = $13,
			"Anagr_DestNazione" = $14,
			"Pagamento" = $15,
			"Pagam_CoordBancarie" = $16,
			"NoteInterne" = $17,
			"DescDoc" = $18,
			"DataDoc" = $19::date,
			"NumDoc" = $20
		WHERE "IDDoc" = $21`,
		req.AnagrNome,
		req.AnagrIndirizzo,
		req.AnagrCap,
		req.AnagrCitta,
		req.AnagrProv,
		req.AnagrNazione,
		req.AnagrCodiceFiscale,
		req.AnagrPartitaIva,
		req.AnagrDestNome,
		req.AnagrDestIndirizzo,
		req.AnagrDestCap,
		req.AnagrDestCitta,
		req.AnagrDestProv,
		req.AnagrDestNazione,
		req.Pagamento,
		req.PagamCoordBancarie,
		req.NoteInterne,
		req.DescDoc,
		req.DataDoc,
		req.NumDoc,
		idDoc,
	)
	if err != nil {
		h.dbFailure(w, r, "update_testata", err, "id_doc", idDoc)
		return
	}

	// 2. Identify rows to delete (keep existing ones in request)
	var keepIDs []string
	for _, row := range req.Rows {
		if row.IDDocRiga > 0 {
			keepIDs = append(keepIDs, strconv.Itoa(row.IDDocRiga))
		}
	}

	if len(keepIDs) > 0 {
		query := `DELETE FROM aenad."TDocRighe" WHERE "IDDoc" = $1 AND "IDDocRiga" NOT IN (` + strings.Join(keepIDs, ",") + `)`
		_, err = tx.ExecContext(r.Context(), query, idDoc)
	} else {
		_, err = tx.ExecContext(r.Context(), `DELETE FROM aenad."TDocRighe" WHERE "IDDoc" = $1`, idDoc)
	}
	if err != nil {
		h.dbFailure(w, r, "delete_removed_rows", err, "id_doc", idDoc)
		return
	}

	// 3. Update or Insert rows
	for _, row := range req.Rows {
		if row.IDDocRiga > 0 {
			// Update
			_, err = tx.ExecContext(r.Context(), `
				UPDATE aenad."TDocRighe" SET
					"CodArticolo" = $1,
					"Desc" = $2,
					"Qta" = $3,
					"Udm" = $4,
					"PrezzoNetto" = $5,
					"Sconti" = $6
				WHERE "IDDocRiga" = $7 AND "IDDoc" = $8`,
				row.CodArticolo,
				row.Desc,
				row.Qta,
				row.Udm,
				row.PrezzoNetto,
				row.Sconti,
				row.IDDocRiga,
				idDoc,
			)
		} else {
			// Insert
			_, err = tx.ExecContext(r.Context(), `
				INSERT INTO aenad."TDocRighe" (
					"IDDoc", "CodArticolo", "Desc", "Qta", "Udm", "PrezzoNetto", "Sconti"
				) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				idDoc,
				row.CodArticolo,
				row.Desc,
				row.Qta,
				row.Udm,
				row.PrezzoNetto,
				row.Sconti,
			)
		}
		if err != nil {
			h.dbFailure(w, r, "upsert_row", err, "id_doc", idDoc, "row_id", row.IDDocRiga)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		h.dbFailure(w, r, "commit_tx", err)
		return
	}

	// Query updated document details
	var updatedRow documentRow
	rowQuery := h.mistra.QueryRowContext(r.Context(), `
		SELECT
			d."IDDoc", d."TipoDoc", d."IDAnagr", d."Anagr_Nome", d."CodDest_IDAnagr", d."CodDest",
			d."Data", d."Num", d."DataDoc", d."NumDoc", d."DescDoc", d."TotNetto", d."TotDoc",
			d."TotPrezzoAcquisto", d."TotGuadagno", d."Pagamento", d."Pagam_CoordBancarie",
			d."NoteInterne", d."Anagr_Indirizzo", d."Anagr_Cap", d."Anagr_Citta", d."Anagr_Prov",
			d."Anagr_Nazione", d."Anagr_CodiceFiscale", d."Anagr_PartitaIva", d."Anagr_DestNome",
			d."Anagr_DestIndirizzo", d."Anagr_DestCap", d."Anagr_DestCitta", d."Anagr_DestProv",
			d."Anagr_DestNazione"
		FROM aenad."TDocTestate" d
		WHERE d."IDDoc" = $1`, idDoc)

	var tipoDoc, codDest, numDoc, descDoc, anagrNome sql.NullString
	var pagamento, pagamCoordBancarie, noteInterne, anagrIndirizzo, anagrCap, anagrCitta, anagrProv, anagrNazione, anagrCodiceFiscale, anagrPartitaIva, anagrDestNome, anagrDestIndirizzo, anagrDestCap, anagrDestCitta, anagrDestProv, anagrDestNazione sql.NullString
	var idAnagr, codDestIDAnagr, num sql.NullInt64
	var data, dataDoc sql.NullTime
	var totNetto, totDoc, totPrezzoAcquisto, totGuadagno sql.NullInt64

	err = rowQuery.Scan(
		&updatedRow.IDDoc, &tipoDoc, &idAnagr, &anagrNome, &codDestIDAnagr, &codDest,
		&data, &num, &dataDoc, &numDoc, &descDoc, &totNetto, &totDoc,
		&totPrezzoAcquisto, &totGuadagno, &pagamento, &pagamCoordBancarie,
		&noteInterne, &anagrIndirizzo, &anagrCap, &anagrCitta, &anagrProv,
		&anagrNazione, &anagrCodiceFiscale, &anagrPartitaIva, &anagrDestNome,
		&anagrDestIndirizzo, &anagrDestCap, &anagrDestCitta, &anagrDestProv,
		&anagrDestNazione,
	)
	if err != nil {
		h.dbFailure(w, r, "query_updated_document", err, "id_doc", idDoc)
		return
	}

	updatedRow.TipoDoc = nullableString(tipoDoc)
	updatedRow.IDAnagr = nullableInt(idAnagr)
	updatedRow.AnagrNome = nullableString(anagrNome)
	updatedRow.CodDestIDAnagr = nullableInt(codDestIDAnagr)
	updatedRow.CodDest = nullableString(codDest)
	updatedRow.Data = nullableDate(data)
	updatedRow.Num = nullableInt(num)
	updatedRow.DataDoc = nullableDate(dataDoc)
	updatedRow.NumDoc = nullableString(numDoc)
	updatedRow.DescDoc = nullableString(descDoc)
	updatedRow.TotNetto = nullableInt64(totNetto)
	updatedRow.TotDoc = nullableInt64(totDoc)
	updatedRow.TotPrezzoAcquisto = nullableInt64(totPrezzoAcquisto)
	updatedRow.TotGuadagno = nullableInt64(totGuadagno)
	updatedRow.Pagamento = nullableString(pagamento)
	updatedRow.PagamCoordBancarie = nullableString(pagamCoordBancarie)
	updatedRow.NoteInterne = nullableString(noteInterne)
	updatedRow.AnagrIndirizzo = nullableString(anagrIndirizzo)
	updatedRow.AnagrCap = nullableString(anagrCap)
	updatedRow.AnagrCitta = nullableString(anagrCitta)
	updatedRow.AnagrProv = nullableString(anagrProv)
	updatedRow.AnagrNazione = nullableString(anagrNazione)
	updatedRow.AnagrCodiceFiscale = nullableString(anagrCodiceFiscale)
	updatedRow.AnagrPartitaIva = nullableString(anagrPartitaIva)
	updatedRow.AnagrDestNome = nullableString(anagrDestNome)
	updatedRow.AnagrDestIndirizzo = nullableString(anagrDestIndirizzo)
	updatedRow.AnagrDestCap = nullableString(anagrDestCap)
	updatedRow.AnagrDestCitta = nullableString(anagrDestCitta)
	updatedRow.AnagrDestProv = nullableString(anagrDestProv)
	updatedRow.AnagrDestNazione = nullableString(anagrDestNazione)

	httputil.JSON(w, http.StatusOK, updatedRow)
}
