package afctools

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// WhmcsTransaction mirrors the row shape produced by v_transazioni (WHMCS MySQL).
// `date` is pre-formatted by the query as "YYYY-MM-DD".
type WhmcsTransaction struct {
	Cliente       *string  `json:"cliente"`
	Fattura       *string  `json:"fattura"`
	InvoiceID     *int64   `json:"invoiceid"`
	UserID        *int64   `json:"userid"`
	PaymentMethod *string  `json:"payment_method"`
	Date          *string  `json:"date"`
	Description   *string  `json:"description"`
	AmountIn      *float64 `json:"amountin"`
	Fees          *float64 `json:"fees"`
	AmountOut     *float64 `json:"amountout"`
	Rate          *float64 `json:"rate"`
	TransID       *string  `json:"transid"`
	RefundID      *int64   `json:"refundid"`
	AccountsID    *int64   `json:"accountsid"`
}

// WhmcsInvoiceLine mirrors the row shape produced by the rigaaliante table.
// All 30 columns are kept as projected; JSON tags match Appsmith field names.
type WhmcsInvoiceLine struct {
	Raggruppamento        *string  `json:"raggruppamento"`
	RagioneSocialeCliente *string  `json:"ragionesocialecliente"`
	NomeCliente           *string  `json:"nomecliente"`
	CognomeCliente        *string  `json:"cognomecliente"`
	PartitaIVA            *string  `json:"partitaiva"`
	CodiceFiscale         *string  `json:"codicefiscale"`
	CodiceISO             *string  `json:"codiceiso"`
	FlagPersonaFisica     *string  `json:"flagpersonafisica"`
	Indirizzo             *string  `json:"indirizzo"`
	NumeroCivico          *string  `json:"numerocivico"`
	CAP                   *string  `json:"cap"`
	Comune                *string  `json:"comune"`
	Provincia             *string  `json:"provincia"`
	Nazione               *string  `json:"nazione"`
	NumeroDocumento       *string  `json:"numerodocumento"`
	DataDocumento         *string  `json:"datadocumento"`
	Causale               *string  `json:"causale"`
	NumeroLinea           *int64   `json:"numerolinea"`
	Quantita              *float64 `json:"quantita"`
	DescrizioneRiga       *string  `json:"descrizioneriga"`
	Prezzo                *float64 `json:"prezzo"`
	DataInizioPeriodo     *string  `json:"datainizioperiodo"`
	DataFinePeriodo       *string  `json:"datafineperiodo"`
	ModalitaPagamento     *string  `json:"modalitapagamento"`
	IVARiga               *float64 `json:"ivariga"`
	Bollo                 *float64 `json:"bollo"`
	CodiceClienteERP      *string  `json:"codiceclienteerp"`
	Tipo                  *string  `json:"tipo"`
	InvoiceID             *int64   `json:"invoiceid"`
	ID                    int64    `json:"id"`
}

// dateToYYYYMMDDInt converts a "YYYY-MM-DD" string into the integer representation
// stored by v_transazioni.date (e.g. 2024-03-08 → 20240308). Returns 0 on empty.
func dateToYYYYMMDDInt(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0, fmt.Errorf("invalid date %q: %w", s, err)
	}
	return t.Year()*10000 + int(t.Month())*100 + t.Day(), nil
}

// listTransactions runs the verbatim getTransactions query (spec §B.4.1) with
// typed date parameters. Preserves the `date > 20230120` floor and the
// invoice/refund filter.
func (h *Handler) listTransactions(r *http.Request, from, to string) ([]WhmcsTransaction, error) {
	fromInt, err := dateToYYYYMMDDInt(from)
	if err != nil {
		return nil, err
	}
	toInt, err := dateToYYYYMMDDInt(to)
	if err != nil {
		return nil, err
	}

	const query = `SELECT CONVERT(CAST(cliente AS BINARY) USING utf8mb4) AS cliente,
	                      fattura, invoiceid, userid, payment_method,
                          date_format(date, '%Y-%m-%d') AS date,
                          description, amountin, fees, amountout, rate,
                          transid, refundid, accountsid
                   FROM v_transazioni
                   WHERE ((fattura <> '' AND invoiceid > 0) OR refundid > 0)
                     AND date > 20230120
                     AND date BETWEEN ? AND ?
                   ORDER BY date DESC, fattura ASC`

	rows, err := h.deps.Whmcs.QueryContext(r.Context(), query, fromInt, toInt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]WhmcsTransaction, 0)
	for rows.Next() {
		var t WhmcsTransaction
		if err := rows.Scan(
			&t.Cliente, &t.Fattura, &t.InvoiceID, &t.UserID, &t.PaymentMethod,
			&t.Date, &t.Description, &t.AmountIn, &t.Fees, &t.AmountOut, &t.Rate,
			&t.TransID, &t.RefundID, &t.AccountsID,
		); err != nil {
			return nil, err
		}
		normalizeWHMCSTextPtrs(
			t.Cliente,
			t.Fattura,
			t.PaymentMethod,
			t.Date,
			t.Description,
			t.TransID,
		)
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Handler) handleTransactions(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w, h.deps.Whmcs, "whmcs") {
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_from_or_to")
		return
	}
	rowsOut, err := h.listTransactions(r, from, to)
	if err != nil {
		h.dbFailure(w, r, "list_transactions", err)
		return
	}
	httputil.JSON(w, http.StatusOK, rowsOut)
}

// -- Invoice lines (righealiante) --

func (h *Handler) listInvoiceLines(r *http.Request) ([]WhmcsInvoiceLine, error) {
	const query = `SELECT raggruppamento, ragionesocialecliente, nomecliente, cognomecliente,
                          partitaiva, codicefiscale, codiceiso, flagpersonafisica,
                          indirizzo, numerocivico, cap, comune, provincia, nazione,
                          numerodocumento, datadocumento, causale, numerolinea, quantita,
                          descrizioneriga, prezzo, datainizioperiodo, datafineperiodo,
                          modalitapagamento, ivariga, bollo, codiceclienteerp, tipo,
                          invoiceid, id
                   FROM rigaaliante
                   ORDER BY id DESC
                   LIMIT 2000`

	rows, err := h.deps.Whmcs.QueryContext(r.Context(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]WhmcsInvoiceLine, 0, 2000)
	for rows.Next() {
		var l WhmcsInvoiceLine
		if err := rows.Scan(
			&l.Raggruppamento, &l.RagioneSocialeCliente, &l.NomeCliente, &l.CognomeCliente,
			&l.PartitaIVA, &l.CodiceFiscale, &l.CodiceISO, &l.FlagPersonaFisica,
			&l.Indirizzo, &l.NumeroCivico, &l.CAP, &l.Comune, &l.Provincia, &l.Nazione,
			&l.NumeroDocumento, &l.DataDocumento, &l.Causale, &l.NumeroLinea, &l.Quantita,
			&l.DescrizioneRiga, &l.Prezzo, &l.DataInizioPeriodo, &l.DataFinePeriodo,
			&l.ModalitaPagamento, &l.IVARiga, &l.Bollo, &l.CodiceClienteERP, &l.Tipo,
			&l.InvoiceID, &l.ID,
		); err != nil {
			return nil, err
		}
		normalizeWHMCSTextPtrs(
			l.Raggruppamento,
			l.RagioneSocialeCliente,
			l.NomeCliente,
			l.CognomeCliente,
			l.PartitaIVA,
			l.CodiceFiscale,
			l.CodiceISO,
			l.FlagPersonaFisica,
			l.Indirizzo,
			l.NumeroCivico,
			l.CAP,
			l.Comune,
			l.Provincia,
			l.Nazione,
			l.NumeroDocumento,
			l.DataDocumento,
			l.Causale,
			l.DescrizioneRiga,
			l.DataInizioPeriodo,
			l.DataFinePeriodo,
			l.ModalitaPagamento,
			l.CodiceClienteERP,
			l.Tipo,
		)
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Handler) handleInvoiceLines(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w, h.deps.Whmcs, "whmcs") {
		return
	}
	lines, err := h.listInvoiceLines(r)
	if err != nil {
		h.dbFailure(w, r, "list_invoice_lines", err)
		return
	}
	httputil.JSON(w, http.StatusOK, lines)
}

// -- Export (carbone) --

type transactionsExportRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type transactionsExportResponse struct {
	RenderID   string `json:"renderId"`
	RenderURL  string `json:"renderUrl"`
	ReportName string `json:"reportName"`
}

func (h *Handler) handleTransactionsExport(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w, h.deps.Whmcs, "whmcs") {
		return
	}
	if h.deps.Carbone == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "carbone_not_configured")
		return
	}

	var req transactionsExportRequest
	if err := decodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if req.From == "" || req.To == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_from_or_to")
		return
	}

	transactions, err := h.listTransactions(r, req.From, req.To)
	if err != nil {
		h.dbFailure(w, r, "export_transactions_query", err)
		return
	}

	reportName := "transazioni_whmcs_dal_" + req.From + "_al_" + req.To
	renderID, err := h.deps.Carbone.RenderTransazioni(r.Context(), reportName, transactions)
	if err != nil {
		httputil.InternalError(w, r, err, "carbone render failed",
			"component", "afctools", "operation", "export_transactions")
		return
	}

	httputil.JSON(w, http.StatusOK, transactionsExportResponse{
		RenderID:   renderID,
		RenderURL:  "https://render.carbone.io/render/" + renderID,
		ReportName: reportName,
	})
}
