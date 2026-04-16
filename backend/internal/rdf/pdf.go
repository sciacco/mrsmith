package rdf

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

const (
	pdfPageWidth  = 595
	pdfPageHeight = 842
	pdfTopY       = 806
	pdfLineStep   = 15
	pdfLeftX      = 48
	pdfMaxLines   = 48
	pdfWrapWidth  = 92
)

func renderRichiestaPDF(full RichiestaFull, analysis string) ([]byte, error) {
	lines := make([]string, 0, 128)
	lines = append(lines, fmt.Sprintf("Richiesta Fattibilita %s", firstNonEmpty(full.CodiceDeal, fmt.Sprintf("#%d", full.ID))))
	lines = append(lines, "")
	lines = append(lines, "Richiesta Utente")
	lines = append(lines, fmt.Sprintf("Cliente: %s", derefString(full.CompanyName, "Non disponibile")))
	lines = append(lines, fmt.Sprintf("Deal: %s", derefString(full.DealName, "Non disponibile")))
	lines = append(lines, fmt.Sprintf("Stato richiesta: %s", full.Stato))
	lines = append(lines, fmt.Sprintf("Data richiesta: %s", full.DataRichiesta))
	lines = append(lines, fmt.Sprintf("Richiedente: %s", derefString(full.CreatedBy, "Non disponibile")))
	lines = append(lines, fmt.Sprintf("Indirizzo: %s", full.Indirizzo))
	lines = append(lines, fmt.Sprintf("Descrizione: %s", full.Descrizione))
	if len(full.PreferredSupplierNames) > 0 {
		lines = append(lines, fmt.Sprintf("Fornitori preferiti: %s", strings.Join(full.PreferredSupplierNames, ", ")))
	}

	lines = append(lines, "")
	lines = append(lines, "Analisi")
	if strings.TrimSpace(analysis) == "" {
		lines = append(lines, "Analisi non disponibile.")
	} else {
		for _, paragraph := range strings.Split(strings.TrimSpace(analysis), "\n") {
			if strings.TrimSpace(paragraph) == "" {
				lines = append(lines, "")
				continue
			}
			lines = append(lines, wrapText(paragraph, pdfWrapWidth)...)
		}
	}

	lines = append(lines, "")
	lines = append(lines, "Richieste di Fattibilita ai Fornitori")
	items := append([]Fattibilita(nil), full.Fattibilita...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].FornitoreNome == items[j].FornitoreNome {
			return items[i].TecnologiaNome < items[j].TecnologiaNome
		}
		return items[i].FornitoreNome < items[j].FornitoreNome
	})
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s - %s", item.FornitoreNome, item.TecnologiaNome))
		lines = append(lines, fmt.Sprintf("Stato: %s", item.Stato))
		lines = append(lines, fmt.Sprintf("Copertura: %s", yesNo(item.Copertura)))
		if item.DurataMesi != nil {
			lines = append(lines, fmt.Sprintf("Durata: %d mesi", *item.DurataMesi))
		}
		if item.GiorniRilascio != nil && *item.GiorniRilascio > 0 {
			lines = append(lines, fmt.Sprintf("Giorni rilascio: %d", *item.GiorniRilascio))
		}
		if label := budgetLabel(item.AderenzaBudget); label != "" {
			lines = append(lines, fmt.Sprintf("Aderenza budget: %s", label))
		}
		if item.EsitoRicevutoIl != nil {
			lines = append(lines, fmt.Sprintf("Esito ricevuto il: %s", *item.EsitoRicevutoIl))
		}
		if item.ProfiloFornitore != nil && strings.TrimSpace(*item.ProfiloFornitore) != "" {
			lines = append(lines, fmt.Sprintf("Profilo: %s", *item.ProfiloFornitore))
		}
		if item.Annotazioni != nil && strings.TrimSpace(*item.Annotazioni) != "" {
			lines = append(lines, wrapText(fmt.Sprintf("Annotazioni: %s", *item.Annotazioni), pdfWrapWidth)...)
		}
		lines = append(lines, "")
	}

	return buildSimplePDF(lines)
}

func buildSimplePDF(lines []string) ([]byte, error) {
	pages := paginateLines(lines, pdfMaxLines)
	if len(pages) == 0 {
		pages = [][]string{{"Documento vuoto"}}
	}

	fontObjectID := 3 + len(pages)*2
	pageObjectIDs := make([]int, 0, len(pages))
	contentObjectIDs := make([]int, 0, len(pages))

	var objects []string
	objects = append(objects, "<< /Type /Catalog /Pages 2 0 R >>")

	for i := range pages {
		pageObjectIDs = append(pageObjectIDs, 3+i*2)
		contentObjectIDs = append(contentObjectIDs, 4+i*2)
	}

	kids := make([]string, 0, len(pageObjectIDs))
	for _, id := range pageObjectIDs {
		kids = append(kids, fmt.Sprintf("%d 0 R", id))
	}
	objects = append(objects, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pageObjectIDs)))

	for i, pageLines := range pages {
		objects = append(objects, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>", pdfPageWidth, pdfPageHeight, fontObjectID, contentObjectIDs[i]))
		stream := buildPageStream(pageLines)
		objects = append(objects, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}

	objects = append(objects, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for i, object := range objects {
		offsets[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}

	startXRef := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objects); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(objects)+1, startXRef)
	return buf.Bytes(), nil
}

func buildPageStream(lines []string) string {
	var sb strings.Builder
	sb.WriteString("BT\n/F1 11 Tf\n")
	y := pdfTopY
	for _, line := range lines {
		fmt.Fprintf(&sb, "1 0 0 1 %d %d Tm (%s) Tj\n", pdfLeftX, y, escapePDFText(normalizePDFText(line)))
		y -= pdfLineStep
	}
	sb.WriteString("ET")
	return sb.String()
}

func paginateLines(lines []string, maxLines int) [][]string {
	expanded := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			expanded = append(expanded, "")
			continue
		}
		expanded = append(expanded, wrapText(line, pdfWrapWidth)...)
	}

	if len(expanded) == 0 {
		return nil
	}

	pages := make([][]string, 0, len(expanded)/maxLines+1)
	for len(expanded) > 0 {
		chunkSize := maxLines
		if len(expanded) < chunkSize {
			chunkSize = len(expanded)
		}
		pages = append(pages, expanded[:chunkSize])
		expanded = expanded[chunkSize:]
	}
	return pages
}

func wrapText(text string, width int) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []string{""}
	}
	words := strings.Fields(trimmed)
	if len(words) == 0 {
		return []string{""}
	}

	lines := make([]string, 0, len(words)/6+1)
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if len(candidate) <= width {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = word
	}
	lines = append(lines, current)
	return lines
}

func escapePDFText(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"(", "\\(",
		")", "\\)",
	)
	return replacer.Replace(value)
}

func normalizePDFText(value string) string {
	replacer := strings.NewReplacer(
		"à", "a",
		"á", "a",
		"è", "e",
		"é", "e",
		"ì", "i",
		"í", "i",
		"ò", "o",
		"ó", "o",
		"ù", "u",
		"ú", "u",
		"À", "A",
		"È", "E",
		"É", "E",
		"Ì", "I",
		"Ò", "O",
		"Ù", "U",
		"•", "-",
		"’", "'",
		"“", "\"",
		"”", "\"",
		"–", "-",
		"—", "-",
	)
	return replacer.Replace(value)
}
