package statsrda

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// ---- helpers for scanning nullable columns -------------------------------- //

type nullString struct {
	sql.NullString
}

func (n nullString) ptr() *string {
	if !n.Valid {
		return nil
	}
	v := strings.TrimSpace(n.String)
	if v == "" {
		return nil
	}
	return &v
}

type nullFloat struct {
	sql.NullFloat64
}

func (n nullFloat) ptr() *float64 {
	if !n.Valid {
		return nil
	}
	return &n.Float64
}

type nullInt struct {
	sql.NullInt64
}

func (n nullInt) ptr() *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

func (n nullInt) intPtr() *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

// ---- /pa/filters --------------------------------------------------------- //

const filtersQuery = `
SELECT COALESCE(budget_di_riferimento, '') AS value, COUNT(*) AS count
FROM pa.issue
WHERE issue_type = 'Acquisto'
  AND budget_di_riferimento IS NOT NULL AND budget_di_riferimento <> ''
GROUP BY 1
ORDER BY count DESC, value ASC`

const statiQuery = `
SELECT COALESCE(status, '') AS value, COUNT(*) AS count
FROM pa.issue
WHERE issue_type = 'Acquisto'
  AND status IS NOT NULL AND status <> ''
GROUP BY 1
ORDER BY count DESC, value ASC`

const tipiQuery = `
SELECT COALESCE(issue_type, '') AS value, COUNT(*) AS count
FROM pa.issue
WHERE issue_type = 'Acquisto'
  AND issue_type IS NOT NULL AND issue_type <> ''
GROUP BY 1
ORDER BY count DESC, value ASC`

const valuteQuery = `
SELECT COALESCE(valuta, '') AS value, COUNT(*) AS count
FROM pa.issue
WHERE issue_type = 'Acquisto'
  AND valuta IS NOT NULL AND valuta <> ''
GROUP BY 1
ORDER BY count DESC, value ASC`

const rangeDateQuery = `
SELECT to_char(min(created), 'YYYY-MM-DD') AS min_date,
       to_char(max(created), 'YYYY-MM-DD') AS max_date
FROM pa.issue
WHERE issue_type = 'Acquisto'`

func (h *Handler) handleFilters(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	ctx := r.Context()

	resp := FiltersResponse{
		Budget: []FilterOption{}, Stati: []FilterOption{},
		Tipi: []FilterOption{}, Valute: []FilterOption{},
	}

	readOptions := func(query string) ([]FilterOption, error) {
		rows, err := h.db.QueryContext(ctx, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []FilterOption{}
		for rows.Next() {
			var opt FilterOption
			if err := rows.Scan(&opt.Value, &opt.Count); err != nil {
				return nil, err
			}
			out = append(out, opt)
		}
		return out, rows.Err()
	}

	var err error
	if resp.Budget, err = readOptions(filtersQuery); err != nil {
		httputil.InternalError(w, r, err, "statsrda filters budget query failed")
		return
	}
	if resp.Stati, err = readOptions(statiQuery); err != nil {
		httputil.InternalError(w, r, err, "statsrda filters stati query failed")
		return
	}
	if resp.Tipi, err = readOptions(tipiQuery); err != nil {
		httputil.InternalError(w, r, err, "statsrda filters tipi query failed")
		return
	}
	if resp.Valute, err = readOptions(valuteQuery); err != nil {
		httputil.InternalError(w, r, err, "statsrda filters valute query failed")
		return
	}

	var minDate, maxDate sql.NullString
	if err := h.db.QueryRowContext(ctx, rangeDateQuery).Scan(&minDate, &maxDate); err != nil {
		httputil.InternalError(w, r, err, "statsrda filters range_date query failed")
		return
	}
	if minDate.Valid && minDate.String != "" {
		v := minDate.String
		resp.RangeDate.Min = &v
	}
	if maxDate.Valid && maxDate.String != "" {
		v := maxDate.String
		resp.RangeDate.Max = &v
	}

	httputil.JSON(w, http.StatusOK, resp)
}

// ---- autocomplete (fornitori / richiedenti) ------------------------------ //
//
// column is a hardcoded literal chosen by the caller (never user input), so
// the interpolated SQL is safe. The ILIKE pattern and LIMIT use $1/$2.

func (h *Handler) handleAutocompleteDimension(w http.ResponseWriter, r *http.Request, column string) {
	if !h.requireDB(w) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, ok := parseLimit(w, r.URL.Query().Get("limit"), defaultAutoLimit, maxAutoLimit)
	if !ok {
		return
	}

	var query string
	args := []any{}
	if q != "" {
		query = fmt.Sprintf(`
SELECT COALESCE(%[1]s, '') AS value, COUNT(*) AS count
FROM pa.issue
WHERE issue_type = 'Acquisto'
  AND %[1]s IS NOT NULL AND %[1]s <> ''
  AND %[1]s ILIKE $1
GROUP BY 1
ORDER BY count DESC, value ASC
LIMIT $2`, column)
		args = append(args, "%"+sanitizeLike(q)+"%", limit)
	} else {
		query = fmt.Sprintf(`
SELECT COALESCE(%[1]s, '') AS value, COUNT(*) AS count
FROM pa.issue
WHERE issue_type = 'Acquisto'
  AND %[1]s IS NOT NULL AND %[1]s <> ''
GROUP BY 1
ORDER BY count DESC, value ASC
LIMIT $1`, column)
		args = append(args, limit)
	}

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda autocomplete query failed")
		return
	}
	defer rows.Close()

	items := []FilterOption{}
	for rows.Next() {
		var opt FilterOption
		if err := rows.Scan(&opt.Value, &opt.Count); err != nil {
			httputil.InternalError(w, r, err, "statsrda autocomplete scan failed")
			return
		}
		items = append(items, opt)
	}
	if err := rows.Err(); err != nil {
		httputil.InternalError(w, r, err, "statsrda autocomplete rows failed")
		return
	}
	httputil.JSON(w, http.StatusOK, AutocompleteResponse{Items: items})
}

func (h *Handler) handleFornitoriAutocomplete(w http.ResponseWriter, r *http.Request) {
	h.handleAutocompleteDimension(w, r, "fornitore_selezionato")
}

func (h *Handler) handleRichiedentiAutocomplete(w http.ResponseWriter, r *http.Request) {
	h.handleAutocompleteDimension(w, r, "reporter_name")
}

// ---- /pa/issues (list) --------------------------------------------------- //

const issueListFields = `
	i.issue_key, i.summary, i.numero_ordine, i.status, i.issue_type,
	i.importo_totale, i.valuta, i.budget_di_riferimento, i.reporter_name,
	i.fornitore_selezionato, i.created`

func (h *Handler) handleIssueList(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	f, ok := parseIssueFilters(w, r.URL.Query())
	if !ok {
		return
	}
	ctx := r.Context()

	// Build WHERE clause with positional $N placeholders.
	var where strings.Builder
	args := []any{}
	paramIdx := 1
	addClause := func(clause string, arg any) {
		where.WriteString(" AND ")
		where.WriteString(clause)
		args = append(args, arg)
		paramIdx++
	}
	// helper to emit $N
	dollar := func() string { return fmt.Sprintf("$%d", paramIdx) }

	// Always filter to Acquisto issues only
	where.WriteString(" AND i.issue_type = 'Acquisto'")

	if f.q != "" {
		like := "%" + sanitizeLike(f.q) + "%"
		where.WriteString(fmt.Sprintf(
			" AND (i.issue_key ILIKE $%d OR i.summary ILIKE $%d OR COALESCE(i.numero_ordine, '') ILIKE $%d)",
			paramIdx, paramIdx+1, paramIdx+2,
		))
		args = append(args, like, like, like)
		paramIdx += 3
	}
	if f.budget != "" {
		addClause(fmt.Sprintf("i.budget_di_riferimento = %s", dollar()), f.budget)
	}
	if f.stato != "" {
		addClause(fmt.Sprintf("i.status = %s", dollar()), f.stato)
	}
	if f.tipo != "" {
		addClause(fmt.Sprintf("i.issue_type = %s", dollar()), f.tipo)
	}
	if f.fornitore != "" {
		addClause(fmt.Sprintf("i.fornitore_selezionato = %s", dollar()), f.fornitore)
	}
	if f.richiedente != "" {
		addClause(fmt.Sprintf("i.reporter_name = %s", dollar()), f.richiedente)
	}
	if f.valuta != "" {
		addClause(fmt.Sprintf("i.valuta = %s", dollar()), f.valuta)
	}
	if f.from != "" {
		addClause(fmt.Sprintf("i.created >= %s", dollar()), f.from)
	}
	if f.to != "" {
		addClause(fmt.Sprintf("i.created < %s::date + INTERVAL '1 day'", dollar()), f.to)
	}

	whereClause := ""
	if where.Len() > 0 {
		whereClause = " WHERE " + strings.TrimPrefix(where.String(), " AND ")
	}

	// total count
	var total int
	countSQL := "SELECT COUNT(*) FROM pa.issue i" + whereClause
	if err := h.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		httputil.InternalError(w, r, err, "statsrda issue list count failed")
		return
	}

	offset := (f.page - 1) * f.limit
	limitParam := paramIdx
	offsetParam := paramIdx + 1
	listSQL := "SELECT " + issueListFields + " FROM pa.issue i" + whereClause +
		fmt.Sprintf(" ORDER BY %s %s, i.id DESC LIMIT $%d OFFSET $%d", f.sortCol, f.sortDir, limitParam, offsetParam)
	listArgs := append(args, f.limit, offset)

	rows, err := h.db.QueryContext(ctx, listSQL, listArgs...)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda issue list query failed")
		return
	}
	defer rows.Close()

	items := []IssueSummary{}
	for rows.Next() {
		var s IssueSummary
		var numero, status, issueType, valuta, budget, reporter, fornitore nullString
		var importo nullFloat
		var created nullString
		if err := rows.Scan(
			&s.IssueKey, &s.Summary, &numero, &status, &issueType,
			&importo, &valuta, &budget, &reporter, &fornitore, &created,
		); err != nil {
			httputil.InternalError(w, r, err, "statsrda issue list scan failed")
			return
		}
		s.NumeroOrdine = numero.ptr()
		s.Status = status.ptr()
		s.IssueType = issueType.ptr()
		s.ImportoTotale = importo.ptr()
		s.Valuta = valuta.ptr()
		s.BudgetDiRiferimento = budget.ptr()
		s.ReporterName = reporter.ptr()
		s.FornitoreSelezionato = fornitore.ptr()
		s.Created = created.ptr()
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		httputil.InternalError(w, r, err, "statsrda issue list rows failed")
		return
	}

	httputil.JSON(w, http.StatusOK, IssueListResponse{
		Items: items, Total: total, Page: f.page, Limit: f.limit,
	})
}

// ---- /pa/issues/{issueKey} (detail) -------------------------------------- //

const issueDetailQuery = `
SELECT issue_key, summary, numero_ordine, issue_type, status, stato, priority,
       resolution, valuta, to_char(created, 'YYYY-MM-DD"T"HH24:MI:SSOF'),
       to_char(updated, 'YYYY-MM-DD"T"HH24:MI:SSOF'),
       to_char(resolution_date, 'YYYY-MM-DD"T"HH24:MI:SSOF'),
       to_char(due_date, 'YYYY-MM-DD'),
       reporter_name, reporter_email, assignee_name, creator_name
FROM pa.issue
WHERE issue_type = 'Acquisto'
  AND issue_key = $1`

const purchaseQuery = `
SELECT importo_totale, importo_totale_merci, importo_totale_servizi,
       importo_totale_leasing, fornitore_selezionato, tipo_di_ordine,
       tipo_documento, budget_di_riferimento, budget_corrente, budget_totale,
       limite_approvazione, percentuale_approvazione,
       to_char(inviato_in_approvazione, 'YYYY-MM-DD"T"HH24:MI:SSOF'),
       to_char(approvato, 'YYYY-MM-DD"T"HH24:MI:SSOF'),
       ricorrente
FROM pa.issue
WHERE issue_type = 'Acquisto'
  AND issue_key = $1`

const descriptionQuery = `SELECT description FROM pa.issue WHERE issue_type = 'Acquisto' AND issue_key = $1`

const lineItemsQuery = `
SELECT grid, COALESCE(row_no,0), articolo_name, vendor, part_number, descrizione,
       quantita, importo, prezzo_totale, pagamento_name, durata_name,
       to_char(data_pagamento::date, 'YYYY-MM-DD'), vendita, rinnovo
FROM pa.line_item
WHERE issue_key = $1
ORDER BY grid, row_no`

const commentsQuery = `
SELECT to_char(created, 'YYYY-MM-DD"T"HH24:MI:SSOF'), author_name, actionbody
FROM pa.comment
WHERE issue_key = $1
ORDER BY created`

const attachmentsQuery = `
SELECT filename, mimetype, filesize, to_char(created, 'YYYY-MM-DD"T"HH24:MI:SSOF'), author_name
FROM pa.attachment
WHERE issue_key = $1
ORDER BY created`

const linksQuery = `
SELECT source_key, destination_key, link_name, inward, outward
FROM pa.issue_link
WHERE source_key = $1 OR destination_key = $1
ORDER BY id`

const historyQuery = `
SELECT to_char(g.created, 'YYYY-MM-DD"T"HH24:MI:SSOF'), g.author_name, i.field,
       i.old_string, i.new_string
FROM pa.change_group g
JOIN pa.change_item i ON i.group_id = g.id
WHERE g.issue_key = $1
ORDER BY g.created DESC, i.id
LIMIT $2`

const historyTotalQuery = `SELECT COUNT(*) FROM pa.change_group WHERE issue_key = $1`

func (h *Handler) handleIssueDetail(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	issueKey := strings.TrimSpace(r.PathValue("issueKey"))
	if issueKey == "" {
		badRequest(w, "issue_key obbligatorio")
		return
	}
	historyLimit, ok := parseHistoryLimit(w, r.URL.Query().Get("history_limit"))
	if !ok {
		return
	}
	ctx := r.Context()

	// section 1 — header (also used to detect 404)
	var ih IssueHeader
	var numero, issueType, status, stato, priority, resolution, valuta, created, updated,
		resolutionDate, dueDate, reporterName, reporterEmail, assigneeName, creatorName nullString
	err := h.db.QueryRowContext(ctx, issueDetailQuery, issueKey).Scan(
		&ih.IssueKey, &ih.Summary, &numero, &issueType, &status, &stato, &priority,
		&resolution, &valuta, &created, &updated, &resolutionDate, &dueDate,
		&reporterName, &reporterEmail, &assigneeName, &creatorName,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "Ordine non trovato")
			return
		}
		httputil.InternalError(w, r, err, "statsrda detail header query failed")
		return
	}
	ih.NumeroOrdine = numero.ptr()
	ih.IssueType = issueType.ptr()
	ih.Status = status.ptr()
	ih.Stato = stato.ptr()
	ih.Priority = priority.ptr()
	ih.Resolution = resolution.ptr()
	ih.Valuta = valuta.ptr()
	ih.Created = created.ptr()
	ih.Updated = updated.ptr()
	ih.ResolutionDate = resolutionDate.ptr()
	ih.DueDate = dueDate.ptr()
	ih.ReporterName = reporterName.ptr()
	ih.ReporterEmail = reporterEmail.ptr()
	ih.AssigneeName = assigneeName.ptr()
	ih.CreatorName = creatorName.ptr()

	// section 2 — purchase
	var p PurchaseSection
	var fornitore, tipoOrdine, tipoDoc, budgetRif, inviato, approvato, ricorrente nullString
	var impTot, impMerci, impServ, impLeasing, budCorr, budTot, limAppr, pctAppr nullFloat
	if err := h.db.QueryRowContext(ctx, purchaseQuery, issueKey).Scan(
		&impTot, &impMerci, &impServ, &impLeasing, &fornitore, &tipoOrdine, &tipoDoc,
		&budgetRif, &budCorr, &budTot, &limAppr, &pctAppr, &inviato, &approvato, &ricorrente,
	); err != nil {
		httputil.InternalError(w, r, err, "statsrda detail purchase query failed")
		return
	}
	p.ImportoTotale = impTot.ptr()
	p.ImportoTotaleMerci = impMerci.ptr()
	p.ImportoTotaleServizi = impServ.ptr()
	p.ImportoTotaleLeasing = impLeasing.ptr()
	p.FornitoreSelezionato = fornitore.ptr()
	p.TipoDiOrdine = tipoOrdine.ptr()
	p.TipoDocumento = tipoDoc.ptr()
	p.BudgetDiRiferimento = budgetRif.ptr()
	p.BudgetCorrente = budCorr.ptr()
	p.BudgetTotale = budTot.ptr()
	p.LimiteApprovazione = limAppr.ptr()
	p.PercentualeApprovazione = pctAppr.ptr()
	p.InviatoInApprovazione = inviato.ptr()
	p.Approvato = approvato.ptr()
	p.Ricorrente = ricorrente.ptr()

	// section 3 — description
	var desc nullString
	if err := h.db.QueryRowContext(ctx, descriptionQuery, issueKey).Scan(&desc); err != nil {
		httputil.InternalError(w, r, err, "statsrda detail description query failed")
		return
	}
	description := desc.ptr()

	// section 4 — line items, grouped by grid
	rows, err := h.db.QueryContext(ctx, lineItemsQuery, issueKey)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda detail line_items query failed")
		return
	}
	lineItemsByGrid := map[string][]LineItem{}
	for rows.Next() {
		var li LineItem
		var articolo, vendor, partNum, descr, pagName, durName, dataPag, vendita, rinnovo nullString
		var quantita, importo, prezzoTotale nullFloat
		if err := rows.Scan(&li.Grid, &li.RowNo, &articolo, &vendor, &partNum, &descr,
			&quantita, &importo, &prezzoTotale, &pagName, &durName, &dataPag, &vendita, &rinnovo); err != nil {
			rows.Close()
			httputil.InternalError(w, r, err, "statsrda detail line_items scan failed")
			return
		}
		li.ArticoloName = articolo.ptr()
		li.Vendor = vendor.ptr()
		li.PartNumber = partNum.ptr()
		li.Descrizione = descr.ptr()
		li.Quantita = quantita.ptr()
		li.Importo = importo.ptr()
		li.PrezzoTotale = prezzoTotale.ptr()
		li.PagamentoName = pagName.ptr()
		li.DurataName = durName.ptr()
		li.DataPagamento = dataPag.ptr()
		li.Vendita = vendita.ptr()
		li.Rinnovo = rinnovo.ptr()
		lineItemsByGrid[li.Grid] = append(lineItemsByGrid[li.Grid], li)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		httputil.InternalError(w, r, err, "statsrda detail line_items rows failed")
		return
	}
	rows.Close()

	// section 5 — comments
	comments, err := h.scanComments(ctx, issueKey)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda detail comments query failed")
		return
	}
	if comments == nil {
		comments = []Comment{}
	}

	// section 6 — attachments
	attachments, err := h.scanAttachments(ctx, issueKey)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda detail attachments query failed")
		return
	}
	if attachments == nil {
		attachments = []Attachment{}
	}

	// section 7 — links
	links, err := h.scanLinks(ctx, issueKey)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda detail links query failed")
		return
	}
	if links == nil {
		links = []IssueLink{}
	}

	// section 8 — history
	history, historyTotal, err := h.scanHistory(ctx, issueKey, historyLimit)
	if err != nil {
		httputil.InternalError(w, r, err, "statsrda detail history query failed")
		return
	}
	if history == nil {
		history = []HistoryEntry{}
	}

	httputil.JSON(w, http.StatusOK, IssueDetail{
		Issue:           ih,
		Purchase:        p,
		Description:     description,
		LineItemsByGrid: lineItemsByGrid,
		Comments:        comments,
		Attachments:     attachments,
		Links:           links,
		History:         history,
		HistoryTotal:    historyTotal,
	})
}

func (h *Handler) scanComments(ctx context.Context, issueKey string) ([]Comment, error) {
	rows, err := h.db.QueryContext(ctx, commentsQuery, issueKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		var c Comment
		var created, author, body nullString
		if err := rows.Scan(&created, &author, &body); err != nil {
			return nil, err
		}
		c.Created = created.ptr()
		c.AuthorName = author.ptr()
		c.Body = body.ptr()
		out = append(out, c)
	}
	return out, rows.Err()
}

func (h *Handler) scanAttachments(ctx context.Context, issueKey string) ([]Attachment, error) {
	rows, err := h.db.QueryContext(ctx, attachmentsQuery, issueKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Attachment{}
	for rows.Next() {
		var a Attachment
		var mimetype, created, author nullString
		var filename nullString
		var filesize nullInt
		if err := rows.Scan(&filename, &mimetype, &filesize, &created, &author); err != nil {
			return nil, err
		}
		a.Filename = strings.TrimSpace(filename.String)
		if a.Filename == "" {
			continue
		}
		a.Mimetype = mimetype.ptr()
		a.Filesize = filesize.ptr()
		a.Created = created.ptr()
		a.AuthorName = author.ptr()
		out = append(out, a)
	}
	return out, rows.Err()
}

func (h *Handler) scanLinks(ctx context.Context, issueKey string) ([]IssueLink, error) {
	rows, err := h.db.QueryContext(ctx, linksQuery, issueKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IssueLink{}
	for rows.Next() {
		var l IssueLink
		var src, dst nullString
		var linkName, inward, outward nullString
		if err := rows.Scan(&src, &dst, &linkName, &inward, &outward); err != nil {
			return nil, err
		}
		l.SourceKey = strings.TrimSpace(src.String)
		l.DestinationKey = strings.TrimSpace(dst.String)
		l.LinkName = linkName.ptr()
		l.Inward = inward.ptr()
		l.Outward = outward.ptr()
		out = append(out, l)
	}
	return out, rows.Err()
}

func (h *Handler) scanHistory(ctx context.Context, issueKey string, limit int) ([]HistoryEntry, int, error) {
	var total int
	if err := h.db.QueryRowContext(ctx, historyTotalQuery, issueKey).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := h.db.QueryContext(ctx, historyQuery, issueKey, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []HistoryEntry{}
	for rows.Next() {
		var e HistoryEntry
		var created, author, field, oldS, newS nullString
		if err := rows.Scan(&created, &author, &field, &oldS, &newS); err != nil {
			return nil, 0, err
		}
		e.Created = created.ptr()
		e.AuthorName = author.ptr()
		e.Field = field.ptr()
		e.OldString = oldS.ptr()
		e.NewString = newS.ptr()
		out = append(out, e)
	}
	return out, total, rows.Err()
}
