package aenad

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// Contratto dati del template Carbone (scripts/carbone/aenad-offerta/):
// tutte le stringhe sono valori display già formattati (it-IT), vuote se assenti.
type offertaRenderPayload struct {
	ConvertTo string      `json:"convertTo"`
	Data      offertaData `json:"data"`
}

type offertaData struct {
	Titolo           string              `json:"titolo"`
	Numero           string              `json:"numero"`
	Data             string              `json:"data"`
	Destinatario     offertaDestinatario `json:"destinatario"`
	Righe            []offertaRiga       `json:"righe"`
	Totali           offertaTotali       `json:"totali"`
	Pagamento        string              `json:"pagamento"`
	MostraCondizioni bool                `json:"mostraCondizioni"`
}

type offertaDestinatario struct {
	Nome         string `json:"nome"`
	Indirizzo    string `json:"indirizzo"`
	CapCittaProv string `json:"capCittaProv"`
	RigaFiscale  string `json:"rigaFiscale"`
}

type offertaRiga struct {
	Codice      string `json:"codice"`
	Descrizione string `json:"descrizione"`
	Udm         string `json:"udm"`
	Qta         string `json:"qta"`
	Prezzo      string `json:"prezzo"`
	Sconto      string `json:"sconto"`
	Importo     string `json:"importo"`
	Iva         string `json:"iva"`
}

type offertaTotali struct {
	Imponibile string `json:"imponibile"`
	Iva        string `json:"iva"`
	Documento  string `json:"documento"`
}

func (h *Handler) handleDocumentPDF(w http.ResponseWriter, r *http.Request) {
	if h.carbone == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "aenad_pdf_not_configured")
		return
	}
	if !h.requireMistra(w) {
		return
	}

	idDoc, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || idDoc <= 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_document_id")
		return
	}

	// Equivalente della scelta del report in Easyfatt: "SHELLI CDLAN offerta"
	// (condizioni complete) vs "offerta noleggio-servizi" (solo privacy).
	mostraCondizioni := parseCondizioni(r.URL.Query().Get("condizioni"))

	var (
		numDoc, pagamento                   sql.NullString
		anagrNome, anagrIndirizzo           sql.NullString
		anagrCap, anagrCitta, anagrProv     sql.NullString
		anagrCodiceFiscale, anagrPartitaIva sql.NullString
		totNetto, totIva, totDoc, titolo    sql.NullString
		dataDoc                             sql.NullTime
	)
	err = h.mistra.QueryRowContext(r.Context(), `
		SELECT
			t."NumDoc", t."DataDoc", t."Pagamento",
			t."Anagr_Nome", t."Anagr_Indirizzo", t."Anagr_Cap", t."Anagr_Citta", t."Anagr_Prov",
			t."Anagr_CodiceFiscale", t."Anagr_PartitaIva",
			t."TotNetto"::text, t."TotIva"::text, t."TotDoc"::text,
			COALESCE(tt."TitoloReport", tt."Nome", t."TipoDoc")
		FROM aenad."TDocTestate" t
		LEFT JOIN aenad."TTipiDoc" tt ON tt."TipoDoc" = t."TipoDoc"
		WHERE t."IDDoc" = $1`, idDoc).Scan(
		&numDoc, &dataDoc, &pagamento,
		&anagrNome, &anagrIndirizzo, &anagrCap, &anagrCitta, &anagrProv,
		&anagrCodiceFiscale, &anagrPartitaIva,
		&totNetto, &totIva, &totDoc, &titolo,
	)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "document_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "document_pdf_header", err, "id_doc", idDoc)
		return
	}

	// Tutte le righe, incluse quelle completamente vuote: nel tracciato Easyfatt
	// sono spaziatori intenzionali tra gli articoli (vedi IMPLEMENTATION-KNOWLEDGE).
	rows, err := h.mistra.QueryContext(r.Context(), `
		SELECT "CodArticolo", "Desc", "Udm", "Qta"::text, "PrezzoNetto"::text,
			"Sconti", "ImportoNettoRiga"::text, "CodIva"
		FROM aenad."TDocRighe"
		WHERE "IDDoc" = $1
		ORDER BY "IDDocRiga" ASC`, idDoc)
	if err != nil {
		h.dbFailure(w, r, "document_pdf_rows", err, "id_doc", idDoc)
		return
	}
	defer rows.Close()

	righe := make([]offertaRiga, 0)
	for rows.Next() {
		var codArticolo, desc, udm, qta, prezzo, sconti, importo, codIva sql.NullString
		if err := rows.Scan(&codArticolo, &desc, &udm, &qta, &prezzo, &sconti, &importo, &codIva); err != nil {
			h.dbFailure(w, r, "document_pdf_rows_scan", err, "id_doc", idDoc)
			return
		}
		righe = append(righe, offertaRiga{
			Codice:      codArticolo.String,
			Descrizione: descToHTML(desc.String),
			Udm:         udm.String,
			Qta:         formatQtaIT(qta),
			Prezzo:      formatEuroIT(prezzo),
			Sconto:      sconti.String,
			Importo:     formatEuroIT(importo),
			Iva:         codIva.String,
		})
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "document_pdf_rows", err, "id_doc", idDoc)
		return
	}

	payload := offertaRenderPayload{
		ConvertTo: "pdf",
		Data: offertaData{
			Titolo: titolo.String,
			Numero: numDoc.String,
			Data:   formatDateIT(dataDoc),
			Destinatario: offertaDestinatario{
				Nome:         anagrNome.String,
				Indirizzo:    anagrIndirizzo.String,
				CapCittaProv: capCittaProv(anagrCap.String, anagrCitta.String, anagrProv.String),
				RigaFiscale:  rigaFiscale(anagrCodiceFiscale.String, anagrPartitaIva.String),
			},
			Righe: righe,
			Totali: offertaTotali{
				Imponibile: formatEuroIT(totNetto),
				Iva:        formatEuroIT(totIva),
				Documento:  formatEuroIT(totDoc),
			},
			Pagamento:        pagamento.String,
			MostraCondizioni: mostraCondizioni,
		},
	}

	pdfBytes, err := h.carbone.GeneratePDF(r.Context(), h.resolveOffertaTemplateID(r.Context()), payload)
	if err != nil {
		httputil.InternalError(w, r, err, "aenad offerta pdf generation failed",
			"component", component, "operation", "document_pdf", "id_doc", idDoc)
		return
	}

	filename := offertaFilename(titolo.String, numDoc.String, dataDoc, anagrNome.String)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_, _ = w.Write(pdfBytes)
}

// resolveOffertaTemplateID reads the Carbone template id from
// mrsmith.runtime_config (namespace "aenad", key "carbone_offerta"): Carbone
// assigns a new id on every template upload, so reading per request lets the
// id change without an application restart. Falls back to the compiled-in
// default when the row is missing or unreadable.
func (h *Handler) resolveOffertaTemplateID(ctx context.Context) string {
	if h.configDB == nil {
		return DefaultOffertaTemplateID
	}

	var raw []byte
	err := h.configDB.QueryRowContext(ctx, `
		SELECT value
		FROM mrsmith.runtime_config
		WHERE namespace = 'aenad' AND key = 'carbone_offerta'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return DefaultOffertaTemplateID
	}
	if err != nil {
		h.logger.Warn("aenad carbone runtime config read failed; using default template id",
			"operation", "carbone_offerta_config_read", "error", err)
		return DefaultOffertaTemplateID
	}

	var payload struct {
		TemplateID string `json:"template_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		h.logger.Warn("aenad carbone runtime config parse failed; using default template id",
			"operation", "carbone_offerta_config_parse", "error", err)
		return DefaultOffertaTemplateID
	}
	if strings.TrimSpace(payload.TemplateID) == "" {
		return DefaultOffertaTemplateID
	}
	return payload.TemplateID
}

func parseCondizioni(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "0", "false", "no":
		return false
	default:
		// Default alla variante completa, come il report Easyfatt standard.
		return true
	}
}

// formatEuroIT renders a numeric(18,4) decimal string as "€ 1.234,56"
// (rounding half-up to 2 decimals). Empty input renders as empty string.
func formatEuroIT(value sql.NullString) string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return ""
	}
	formatted, ok := decimalToIT(value.String, 2)
	if !ok {
		return value.String
	}
	return "€ " + formatted
}

// formatQtaIT renders quantities without trailing zeros ("2.0000" → "2",
// "1.5000" → "1,5"). Empty input renders as empty string.
func formatQtaIT(value sql.NullString) string {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return ""
	}
	raw := strings.TrimSpace(value.String)
	intPart, fracPart, _ := strings.Cut(raw, ".")
	fracPart = strings.TrimRight(fracPart, "0")
	neg := strings.HasPrefix(intPart, "-")
	intPart = strings.TrimPrefix(intPart, "-")
	if intPart == "" {
		intPart = "0"
	}
	out := groupThousandsIT(intPart)
	if fracPart != "" {
		out += "," + fracPart
	}
	if neg {
		out = "-" + out
	}
	return out
}

// decimalToIT converts a dot-decimal string into Italian display format with
// the requested number of decimals (half-up rounding), e.g. "24276.13" → "24.276,13".
func decimalToIT(raw string, decimals int) (string, bool) {
	raw = strings.TrimSpace(raw)
	neg := strings.HasPrefix(raw, "-")
	raw = strings.TrimPrefix(raw, "-")
	intPart, fracPart, _ := strings.Cut(raw, ".")
	if intPart == "" {
		intPart = "0"
	}
	for _, r := range intPart {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	for _, r := range fracPart {
		if r < '0' || r > '9' {
			return "", false
		}
	}

	// Arrotondamento half-up sulla cifra successiva ai decimali richiesti.
	keep := fracPart
	if len(keep) > decimals {
		next := keep[decimals]
		keep = keep[:decimals]
		if next >= '5' {
			carried, overflow := incrementDecimalString(keep)
			keep = carried
			if overflow {
				intCarried, intOverflow := incrementDecimalString(intPart)
				if intOverflow {
					intPart = "1" + intCarried
				} else {
					intPart = intCarried
				}
			}
		}
	}
	for len(keep) < decimals {
		keep += "0"
	}

	out := groupThousandsIT(intPart)
	if decimals > 0 {
		out += "," + keep
	}
	if neg {
		out = "-" + out
	}
	return out, true
}

// incrementDecimalString adds 1 to a fixed-width digit string; overflow is
// true when the carry exceeds the width (result stays at the same width).
func incrementDecimalString(digits string) (string, bool) {
	if digits == "" {
		return "", true
	}
	out := []byte(digits)
	for i := len(out) - 1; i >= 0; i-- {
		if out[i] < '9' {
			out[i]++
			return string(out), false
		}
		out[i] = '0'
	}
	return string(out), true
}

func groupThousandsIT(digits string) string {
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		digits = "0"
	}
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func formatDateIT(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("02/01/2006")
}

func capCittaProv(cap, citta, prov string) string {
	parts := make([]string, 0, 3)
	if cap = strings.TrimSpace(cap); cap != "" {
		parts = append(parts, cap)
	}
	if citta = strings.TrimSpace(citta); citta != "" {
		parts = append(parts, citta)
	}
	if prov = strings.TrimSpace(prov); prov != "" {
		parts = append(parts, "("+prov+")")
	}
	return strings.Join(parts, "  ")
}

// rigaFiscale replica la riga fiscale delle stampe Easyfatt: "C.F./P.Iva" quando
// coincidono, entrambe quando differiscono, una sola quando presente.
func rigaFiscale(codiceFiscale, partitaIva string) string {
	cf := strings.TrimSpace(codiceFiscale)
	piva := strings.TrimSpace(partitaIva)
	switch {
	case cf != "" && piva != "" && strings.EqualFold(cf, piva):
		return "C.F./P.Iva " + piva
	case cf != "" && piva != "":
		return "C.F. " + cf + " - P.Iva " + piva
	case piva != "":
		return "P.Iva " + piva
	case cf != "":
		return "C.F. " + cf
	default:
		return ""
	}
}

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// descToHTML converte i marker inline Easyfatt di TDocRighe.Desc in HTML per il
// formatter Carbone :html. "**" alterna il grassetto, "//" il corsivo; un marker
// non chiuso stila il resto della riga. Le sequenze "://" (URL) non sono marker.
func descToHTML(desc string) string {
	if desc == "" {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(desc, "\r\n", "\n"), "\n")
	for li, line := range lines {
		line = htmlEscaper.Replace(line)
		var b strings.Builder
		bold, italic := false, false
		for i := 0; i < len(line); {
			// "***" = coppia ** con un asterisco letterale che resta dentro lo stile.
			if strings.HasPrefix(line[i:], "***") && !strings.HasPrefix(line[i:], "****") {
				if bold {
					b.WriteString("*</b>")
				} else {
					b.WriteString("<b>*")
				}
				bold = !bold
				i += 3
				continue
			}
			if strings.HasPrefix(line[i:], "**") {
				if bold {
					b.WriteString("</b>")
				} else {
					b.WriteString("<b>")
				}
				bold = !bold
				i += 2
				continue
			}
			if strings.HasPrefix(line[i:], "//") && (i == 0 || line[i-1] != ':') {
				if italic {
					b.WriteString("</i>")
				} else {
					b.WriteString("<i>")
				}
				italic = !italic
				i += 2
				continue
			}
			b.WriteByte(line[i])
			i++
		}
		if bold {
			b.WriteString("</b>")
		}
		if italic {
			b.WriteString("</i>")
		}
		lines[li] = b.String()
	}
	return strings.Join(lines, "<br>")
}

var filenameSanitizer = strings.NewReplacer(
	".", " ", "/", " ", "\\", " ", ":", " ", "*", " ",
	"?", " ", "\"", " ", "<", " ", ">", " ", "|", " ",
)

// offertaFilename replica il nome delle stampe legacy:
// "Offerta 1038 del 03-06-2026 RIMOND S R L.pdf".
func offertaFilename(titolo, numDoc string, dataDoc sql.NullTime, cliente string) string {
	if titolo == "" {
		titolo = "Documento"
	}
	parts := []string{titolo}
	if numDoc != "" {
		parts = append(parts, numDoc)
	}
	if dataDoc.Valid {
		parts = append(parts, "del", dataDoc.Time.Format("02-01-2006"))
	}
	if cliente = strings.Join(strings.Fields(filenameSanitizer.Replace(cliente)), " "); cliente != "" {
		parts = append(parts, cliente)
	}
	return strings.Join(parts, " ") + ".pdf"
}
