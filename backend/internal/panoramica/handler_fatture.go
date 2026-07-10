package panoramica

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/xuri/excelize/v2"
)

const (
	defaultInvoicePageSize = 250
	maxInvoicePageSize     = 250
)

type invoiceListParams struct {
	Cliente  int
	Mesi     *int
	Query    string
	Sort     string
	Dir      string
	Page     int
	PageSize int
}

type invoiceLine struct {
	ProgressivoRiga          int      `json:"progressivo_riga"`
	DescrizioneRiga          *string  `json:"descrizione_riga"`
	Qta                      *float64 `json:"qta"`
	PrezzoUnitario           *float64 `json:"prezzo_unitario"`
	PrezzoTotaleNetto        *float64 `json:"prezzo_totale_netto"`
	CodiceArticolo           *string  `json:"codice_articolo"`
	Serialnumber             *string  `json:"serialnumber"`
	RiferimentoOrdineCliente *string  `json:"riferimento_ordine_cliente"`
	CondizionePagamento      *string  `json:"condizione_pagamento"`
	Scadenza                 *string  `json:"scadenza"`
	DescContoRicavo          *string  `json:"desc_conto_ricavo"`
	Gruppo                   *string  `json:"gruppo"`
	Sottogruppo              *string  `json:"sottogruppo"`
}

type invoiceDocument struct {
	ID            string        `json:"id"`
	AnnoDocumento int           `json:"anno_documento"`
	MeseDocumento int           `json:"mese_documento"`
	TipoDocumento string        `json:"tipo_documento"`
	NumDocumento  string        `json:"num_documento"`
	Doc           string        `json:"doc"`
	DataDocumento *string       `json:"data_documento"`
	IDCliente     int           `json:"id_cliente"`
	Segno         int           `json:"segno"`
	LineCount     int           `json:"line_count"`
	TotaleNetto   *float64      `json:"totale_netto"`
	Lines         []invoiceLine `json:"lines"`
}

type invoiceDocumentsResponse struct {
	Items          []invoiceDocument `json:"items"`
	TotalDocuments int               `json:"total_documents"`
	Page           int               `json:"page"`
	PageSize       int               `json:"page_size"`
}

func parseInvoiceParams(r *http.Request, paginated bool) (invoiceListParams, error) {
	q := r.URL.Query()
	cliente, err := strconv.Atoi(q.Get("cliente"))
	if err != nil || cliente <= 0 {
		return invoiceListParams{}, fmt.Errorf("invalid_cliente_parameter")
	}
	p := invoiceListParams{Cliente: cliente, Query: strings.TrimSpace(q.Get("q")), Sort: q.Get("sort"), Dir: q.Get("dir"), Page: 1, PageSize: defaultInvoicePageSize}
	if len(p.Query) > 200 {
		return p, fmt.Errorf("invalid_q_parameter")
	}
	if raw := q.Get("mesi"); raw != "" {
		mesi, err := strconv.Atoi(raw)
		if err != nil || mesi <= 0 {
			return p, fmt.Errorf("invalid_mesi_parameter")
		}
		p.Mesi = &mesi
	}
	if p.Sort == "" {
		p.Sort = "data_documento"
	}
	if p.Sort != "data_documento" && p.Sort != "documento" && p.Sort != "totale_netto" {
		return p, fmt.Errorf("invalid_sort_parameter")
	}
	if p.Dir == "" {
		p.Dir = "desc"
	}
	if p.Dir != "asc" && p.Dir != "desc" {
		return p, fmt.Errorf("invalid_dir_parameter")
	}
	if paginated {
		if raw := q.Get("page"); raw != "" {
			p.Page, err = strconv.Atoi(raw)
			if err != nil || p.Page <= 0 {
				return p, fmt.Errorf("invalid_page_parameter")
			}
		}
		if raw := q.Get("page_size"); raw != "" {
			p.PageSize, err = strconv.Atoi(raw)
			if err != nil || p.PageSize <= 0 || p.PageSize > maxInvoicePageSize {
				return p, fmt.Errorf("invalid_page_size_parameter")
			}
		}
	}
	return p, nil
}

func invoiceOrderClause(p invoiceListParams) string {
	column := map[string]string{
		"data_documento": "data_documento",
		"documento":      "CASE WHEN num_documento ~ '^[0-9]+$' THEN num_documento::numeric END",
		"totale_netto":   "totale_netto",
	}[p.Sort]
	direction := strings.ToUpper(p.Dir)
	return fmt.Sprintf("%s %s NULLS LAST, anno_documento %s, mese_documento %s, tipo_documento %s, num_documento %s", column, direction, direction, direction, direction, direction)
}

func invoiceQuery(p invoiceListParams, paginated bool) (string, []any) {
	args := []any{p.Cliente, p.Mesi, p.Query}
	pageSQL := ""
	if paginated {
		args = append(args, p.PageSize, (p.Page-1)*p.PageSize)
		pageSQL = "LIMIT $4 OFFSET $5"
	}
	query := fmt.Sprintf(`
WITH candidate_lines AS (
  SELECT anno_documento, mese_documento, btrim(tipo_documento) AS tipo_documento,
         num_documento::text AS num_documento, btrim(COALESCE(doc, '')) AS doc,
         to_char(data_documento, 'YYYY-MM-DD') AS data_documento, id_cliente,
         progressivo_riga, descrizione_riga, qta, prezzo_unitario, prezzo_totale_netto,
         codice_articolo, serialnumber, riferimento_ordine_cliente, condizione_pagamento,
         to_char(scadenza, 'YYYY-MM-DD') AS scadenza, desc_conto_ricavo, gruppo, sottogruppo,
         COALESCE(segno, 1) AS segno
  FROM loader.v_erp_fatture_nc
  WHERE id_cliente = $1
    AND ($2::integer IS NULL OR data_documento >= current_date - ($2::integer * interval '1 month'))
), matching_documents AS (
  SELECT anno_documento, mese_documento, tipo_documento, num_documento,
         MAX(doc) AS doc, MAX(data_documento) AS data_documento, MAX(id_cliente) AS id_cliente,
         MAX(segno) AS segno, COUNT(*)::integer AS line_count,
         SUM(prezzo_totale_netto * segno) AS totale_netto
  FROM candidate_lines
  GROUP BY anno_documento, mese_documento, tipo_documento, num_documento
  HAVING $3 = '' OR BOOL_OR(
    concat_ws(' ', doc, num_documento, descrizione_riga, codice_articolo, serialnumber,
      riferimento_ordine_cliente, condizione_pagamento, desc_conto_ricavo, gruppo, sottogruppo)
      ILIKE '%%' || $3 || '%%'
  )
), paged_documents AS (
  SELECT *, COUNT(*) OVER()::integer AS total_documents,
         ROW_NUMBER() OVER (ORDER BY %s) AS page_ordinal
  FROM matching_documents
  ORDER BY %s
  %s
)
SELECT p.total_documents, p.page_ordinal, p.anno_documento, p.mese_documento,
       p.tipo_documento, p.num_documento, p.doc, p.data_documento, p.id_cliente,
       p.segno, p.line_count, p.totale_netto, l.progressivo_riga, l.descrizione_riga,
       l.qta, l.prezzo_unitario, l.prezzo_totale_netto, l.codice_articolo,
       l.serialnumber, l.riferimento_ordine_cliente, l.condizione_pagamento,
       l.scadenza, l.desc_conto_ricavo, l.gruppo, l.sottogruppo
FROM paged_documents p
JOIN candidate_lines l USING (anno_documento, mese_documento, tipo_documento, num_documento)
ORDER BY p.page_ordinal, l.progressivo_riga`, invoiceOrderClause(p), invoiceOrderClause(p), pageSQL)
	return query, args
}

func (h *Handler) loadInvoiceDocuments(r *http.Request, p invoiceListParams, paginated bool) (invoiceDocumentsResponse, error) {
	query, args := invoiceQuery(p, paginated)
	rows, err := h.mistraDB.QueryContext(r.Context(), query, args...)
	resp := invoiceDocumentsResponse{Items: []invoiceDocument{}, Page: p.Page, PageSize: p.PageSize}
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	byID := make(map[string]int)
	for rows.Next() {
		var total, ordinal int
		var d invoiceDocument
		var line invoiceLine
		var date, description, article, serial, reference, payment, due, account, group, subgroup sql.NullString
		var totalNet, qty, unitPrice, lineTotal sql.NullFloat64
		if err := rows.Scan(&total, &ordinal, &d.AnnoDocumento, &d.MeseDocumento, &d.TipoDocumento,
			&d.NumDocumento, &d.Doc, &date, &d.IDCliente, &d.Segno, &d.LineCount, &totalNet,
			&line.ProgressivoRiga, &description, &qty, &unitPrice, &lineTotal, &article, &serial,
			&reference, &payment, &due, &account, &group, &subgroup); err != nil {
			return resp, err
		}
		d.ID = fmt.Sprintf("%d-%d-%s-%s", d.AnnoDocumento, d.MeseDocumento, d.TipoDocumento, d.NumDocumento)
		d.DataDocumento = nullStringPtr(date)
		d.TotaleNetto = nullFloatPtr(totalNet)
		line.DescrizioneRiga = nullStringPtr(description)
		line.Qta = nullFloatPtr(qty)
		line.PrezzoUnitario = nullFloatPtr(unitPrice)
		line.PrezzoTotaleNetto = nullFloatPtr(lineTotal)
		line.CodiceArticolo = nullStringPtr(article)
		line.Serialnumber = nullStringPtr(serial)
		line.RiferimentoOrdineCliente = nullStringPtr(reference)
		line.CondizionePagamento = nullStringPtr(payment)
		line.Scadenza = nullStringPtr(due)
		line.DescContoRicavo = nullStringPtr(account)
		line.Gruppo = nullStringPtr(group)
		line.Sottogruppo = nullStringPtr(subgroup)
		resp.TotalDocuments = total
		if index, ok := byID[d.ID]; ok {
			resp.Items[index].Lines = append(resp.Items[index].Lines, line)
		} else {
			d.Lines = []invoiceLine{line}
			byID[d.ID] = len(resp.Items)
			resp.Items = append(resp.Items, d)
		}
	}
	return resp, rows.Err()
}

func nullFloatPtr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func (h *Handler) handleListInvoices(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}
	p, err := parseInvoiceParams(r, true)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.loadInvoiceDocuments(r, p, true)
	if err != nil {
		h.dbFailure(w, r, "list_invoice_documents", err)
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

func (h *Handler) handleExportInvoices(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}
	p, err := parseInvoiceParams(r, false)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.loadInvoiceDocuments(r, p, false)
	if err != nil {
		h.dbFailure(w, r, "export_invoice_documents", err)
		return
	}
	buf, err := buildInvoicesExcel(resp.Items)
	if err != nil {
		h.dbFailure(w, r, "build_invoices_excel", err)
		return
	}
	filename := fmt.Sprintf("fatture_%d_%s.xlsx", p.Cliente, time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	_, _ = w.Write(buf)
}

func buildInvoicesExcel(documents []invoiceDocument) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Fatture"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	headers := []string{"Documento", "Tipo", "Data", "Numero", "Progressivo", "Descrizione", "Quantità", "Prezzo unitario", "Totale riga", "Totale documento", "Codice articolo", "Serial number", "Riferimento ordine", "Condizione pagamento", "Scadenza", "Conto ricavo", "Gruppo", "Sottogruppo"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return nil, err
		}
	}
	row := 2
	for _, d := range documents {
		for _, line := range d.Lines {
			values := []any{strings.TrimSpace(d.Doc + " " + d.NumDocumento), d.Doc, pointerValue(d.DataDocumento), d.NumDocumento, line.ProgressivoRiga, pointerValue(line.DescrizioneRiga), pointerValue(line.Qta), pointerValue(line.PrezzoUnitario), pointerValue(line.PrezzoTotaleNetto), pointerValue(d.TotaleNetto), pointerValue(line.CodiceArticolo), pointerValue(line.Serialnumber), pointerValue(line.RiferimentoOrdineCliente), pointerValue(line.CondizionePagamento), pointerValue(line.Scadenza), pointerValue(line.DescContoRicavo), pointerValue(line.Gruppo), pointerValue(line.Sottogruppo)}
			for i, value := range values {
				cell, _ := excelize.CoordinatesToCellName(i+1, row)
				if err := f.SetCellValue(sheet, cell, value); err != nil {
					return nil, err
				}
			}
			row++
		}
	}
	widths := []float64{20, 10, 13, 14, 12, 48, 12, 18, 18, 20, 18, 22, 24, 24, 13, 26, 18, 18}
	for i, width := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheet, col, col, width)
	}
	_ = f.AutoFilter(sheet, fmt.Sprintf("A1:R%d", max(row-1, 1)), nil)
	_ = f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func pointerValue[T any](value *T) any {
	if value == nil {
		return ""
	}
	return *value
}
