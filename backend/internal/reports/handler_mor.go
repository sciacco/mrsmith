package reports

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleMorAnomalies returns enriched telephone billing records cross-referenced
// with ERP voice orders to detect anomalies.
// GET /reports/v1/mor-anomalies
func (h *Handler) handleMorAnomalies(w http.ResponseWriter, r *http.Request) {
	if !h.requireGrappa(w) {
		return
	}
	if !h.requireMistra(w) {
		return
	}

	// ── Step 1: Query Grappa (MySQL) for latest billing period ──

	grappaRows, err := h.grappaDB.QueryContext(r.Context(), `
select it.conto, lastname, firstname, is_da_fatturare, codice_ordine, serialnumber,
       it.periodo_inizio, it.importo, it.stato, it.tipologia, ct.id_cliente, ac.intestazione
from importi_telefonici it
         left join conti_telefonici ct on ct.conto = it.conto
         left join grappa.cli_fatturazione ac on ct.id_cliente = ac.id
where periodo_inizio = (select periodo_inizio from importi_telefonici order by id desc limit 1)`)
	if err != nil {
		h.dbFailure(w, r, "mor_grappa", err)
		return
	}
	defer grappaRows.Close()

	type billingRecord struct {
		Conto                 *string  `json:"conto"`
		Lastname              *string  `json:"lastname"`
		Firstname             *string  `json:"firstname"`
		IsDaFatturare         *bool    `json:"is_da_fatturare"`
		CodiceOrdine          *string  `json:"codice_ordine"`
		Serialnumber          *string  `json:"serialnumber"`
		PeriodoInizio         *string  `json:"periodo_inizio"`
		Importo               *float64 `json:"importo"`
		Stato                 *string  `json:"stato"`
		Tipologia             *string  `json:"tipologia"`
		IDCliente             *int     `json:"id_cliente"`
		Intestazione          *string  `json:"intestazione"`
		OrdinePresente        string   `json:"ordine_presente"`
		NumeroOrdineCorretto  string   `json:"numero_ordine_corretto"`
	}

	var billingRecords []billingRecord
	for grappaRows.Next() {
		var rec billingRecord
		var (
			conto, lastname, firstname    sql.NullString
			codiceOrdine, serialnumber    sql.NullString
			periodoInizio, stato, tipol   sql.NullString
			intestazione                  sql.NullString
			isDaFatturare                 sql.NullBool
			importo                       sql.NullFloat64
			idCliente                     sql.NullInt64
		)

		if err := grappaRows.Scan(
			&conto, &lastname, &firstname, &isDaFatturare,
			&codiceOrdine, &serialnumber,
			&periodoInizio, &importo, &stato, &tipol,
			&idCliente, &intestazione,
		); err != nil {
			h.dbFailure(w, r, "mor_grappa_scan", err)
			return
		}

		rec.Conto = nullStringPtr(conto)
		rec.Lastname = nullStringPtr(lastname)
		rec.Firstname = nullStringPtr(firstname)
		rec.IsDaFatturare = nullBoolPtr(isDaFatturare)
		rec.CodiceOrdine = nullStringPtr(codiceOrdine)
		rec.Serialnumber = nullStringPtr(serialnumber)
		rec.PeriodoInizio = nullStringPtr(periodoInizio)
		rec.Importo = nullFloat64Ptr(importo)
		rec.Stato = nullStringPtr(stato)
		rec.Tipologia = nullStringPtr(tipol)
		if idCliente.Valid {
			v := int(idCliente.Int64)
			rec.IDCliente = &v
		}
		rec.Intestazione = nullStringPtr(intestazione)

		billingRecords = append(billingRecords, rec)
	}
	if !h.rowsDone(w, r, grappaRows, "mor_grappa") {
		return
	}

	// ── Step 2: Query Mistra (PostgreSQL) for active voice orders ──

	mistraRows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT serialnumber, codice_prodotto, nome_testata_ordine, cliente
from loader.erp_righe_ordini ero
where codice_prodotto = 'CDL-TVOCE' and data_cessazione = '0001-01-01 00:00:00.000000'
order by cliente`)
	if err != nil {
		h.dbFailure(w, r, "mor_mistra", err)
		return
	}
	defer mistraRows.Close()

	type voiceOrder struct {
		Serialnumber      string
		CodiceProdotto    string
		NomeTestataOrdine string
		Cliente           string
	}

	voiceOrders := make(map[string]voiceOrder)
	for mistraRows.Next() {
		var vo voiceOrder
		var (
			sn, cp, nto, cl sql.NullString
		)
		if err := mistraRows.Scan(&sn, &cp, &nto, &cl); err != nil {
			h.dbFailure(w, r, "mor_mistra_scan", err)
			return
		}
		vo.Serialnumber = sn.String
		vo.CodiceProdotto = cp.String
		vo.NomeTestataOrdine = nto.String
		vo.Cliente = cl.String

		if sn.Valid && sn.String != "" {
			voiceOrders[sn.String] = vo
		}
	}
	if !h.rowsDone(w, r, mistraRows, "mor_mistra") {
		return
	}

	// ── Step 3: Cross-reference (collega_ordini logic) ──

	for i := range billingRecords {
		rec := &billingRecords[i]
		sn := ""
		if rec.Serialnumber != nil {
			sn = *rec.Serialnumber
		}

		vo, found := voiceOrders[sn]
		if found && vo.CodiceProdotto != "" {
			rec.OrdinePresente = "SI"
		} else {
			rec.OrdinePresente = "NO"
		}

		if found {
			codOrd := ""
			if rec.CodiceOrdine != nil {
				codOrd = *rec.CodiceOrdine
			}
			if vo.NomeTestataOrdine == codOrd {
				rec.NumeroOrdineCorretto = "SI"
			} else {
				rec.NumeroOrdineCorretto = "NO"
			}
		} else {
			rec.NumeroOrdineCorretto = "NO"
		}
	}

	if billingRecords == nil {
		billingRecords = []billingRecord{}
	}

	httputil.JSON(w, http.StatusOK, billingRecords)
}
