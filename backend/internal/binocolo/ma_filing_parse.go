package binocolo

// Deterministic CEE parser for deposited-filing fascicoli (issue #78, Fase 5). Reads
// the per-page markdown produced by Mistral OCR (table_format=markdown) of a CCIAA
// XBRL-generated fascicolo (tassonomia itcc-ci, bilancio abbreviato or ordinario) and
// derives, PER EXERCISE COLUMN, the Stato Patrimoniale / Conto Economico headline
// figures plus arithmetic quadrature checks. NO LLM here: the numbers are read straight
// from the markdown tables. The NI (nota integrativa) is only SECTIONED (for F6); its
// invisible facts are the LLM's job, not this parser's.
//
// Everything here is pure (pages in, structs out) so it is unit-tested on frozen
// fixtures with zero external calls. The ingest job (ma_filing_ingest_job.go) wires it
// to the store.

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Output shapes (jsonb-ready — json tags are the stored column shape).
// ---------------------------------------------------------------------------

// maFilingSP is the Stato Patrimoniale headline of one exercise column. Pointers = voce
// not recognized in the fascicolo (abbreviato omits many sub-totals); "totaleDebiti" is
// the single D) Debiti figure the abbreviato reports (no entro/oltre split in the
// prospetto). The json tag totaleAttivo is read back by GetMAFilingExtractsByFiscalKey.
type maFilingSP struct {
	TotaleAttivo           *float64 `json:"totaleAttivo,omitempty"`
	TotalePassivo          *float64 `json:"totalePassivo,omitempty"`
	TotaleImmobilizzazioni *float64 `json:"totaleImmobilizzazioni,omitempty"`
	TotaleAttivoCircolante *float64 `json:"totaleAttivoCircolante,omitempty"`
	TotalePatrimonioNetto  *float64 `json:"totalePatrimonioNetto,omitempty"`
	TotaleDebiti           *float64 `json:"totaleDebiti,omitempty"`
	TFR                    *float64 `json:"tfr,omitempty"`
}

// maFilingCE is the Conto Economico headline of one exercise column.
type maFilingCE struct {
	ValoreProduzione *float64 `json:"valoreProduzione,omitempty"`
	CostiProduzione  *float64 `json:"costiProduzione,omitempty"`
	DifferenzaAB     *float64 `json:"differenzaAB,omitempty"` // A-B as DECLARED in the prospetto
	UtileEsercizio   *float64 `json:"utileEsercizio,omitempty"`
}

// maFilingCheck is one quadrature/consistency check for an exercise (jsonb array).
type maFilingCheck struct {
	Name     string   `json:"name"`
	Passed   bool     `json:"passed"`
	Expected *float64 `json:"expected,omitempty"`
	Actual   *float64 `json:"actual,omitempty"`
}

// maFilingExerciseExtract is the parse of one exercise column.
type maFilingExerciseExtract struct {
	ExerciseDate time.Time
	SP           maFilingSP
	CE           maFilingCE
	Checks       []maFilingCheck
}

// maFilingParseResult is the whole fascicolo parse: one extract per exercise column
// (1..2 — the comparative is mandatory by art. 2423-ter c.5 but a neocostituita reports
// a single column), plus document-level metadata.
type maFilingParseResult struct {
	Exercises        []maFilingExerciseExtract
	TaxonomyVersion  string
	BalanceSheetType string
}

// ClosingDate is the most-recent exercise date across the fascicolo (nil when empty).
func (r maFilingParseResult) ClosingDate() *time.Time {
	var latest *time.Time
	for i := range r.Exercises {
		d := r.Exercises[i].ExerciseDate
		if latest == nil || d.After(*latest) {
			dd := d
			latest = &dd
		}
	}
	return latest
}

// maFilingPeerExtract is the SP total-attivo of an exercise from ANOTHER filing sharing
// the fiscal identity (from GetMAFilingExtractsByFiscalKey), consumed by
// detectAdaptedComparative.
type maFilingPeerExtract struct {
	ExerciseDate time.Time
	ClosingDate  *time.Time
	TotaleAttivo *float64
}

// maFilingAdaptedComparative flags, for an overlapping exercise, whether a peer filing's
// restated figure diverges from this fascicolo's and which filing prevails in the view.
type maFilingAdaptedComparative struct {
	ExerciseDate time.Time
	Diverged     bool
	Expected     *float64 // this fascicolo's totale attivo
	Actual       *float64 // the peer's totale attivo
	Prevails     string   // "current" | "peer" | "" (parity ⇒ no auto-selection)
}

const (
	// maFilingCheckTolerance is the half-euro equality slack for arithmetic checks (the
	// prospetti are integer euros; rounding can still leave a ±1 in a declared subtotal).
	maFilingCheckTolerance = 0.5
	// maFilingAdaptedTolerance is the >1 EUR divergence threshold for a restated
	// comparative to be flagged (below it is rounding, not an adaptation).
	maFilingAdaptedTolerance = 1.0
)

// ---------------------------------------------------------------------------
// Number parsing (Italian conventions).
// ---------------------------------------------------------------------------

var maNumberParenNegRe = regexp.MustCompile(`^\((.*)\)$`)

// parseItalianNumber reads an Italian-formatted accounting number: "." = thousands, ","
// = decimals, parentheses (or a leading "-") = negative, "-"/""/dash = absent (ok=false).
// Currency/euro markers are tolerated. Returns (value, true) only for a real number.
func parseItalianNumber(raw string) (float64, bool) {
	s := strings.TrimSpace(raw)
	if s == "" || s == "-" || s == "–" || s == "—" || s == "n.a." || s == "n/a" {
		return 0, false
	}
	for _, marker := range []string{"€", "EUR", "Euro", "euro"} {
		s = strings.ReplaceAll(s, marker, "")
	}
	s = strings.TrimSpace(s)
	negative := false
	if m := maNumberParenNegRe.FindStringSubmatch(s); m != nil {
		negative = true
		s = strings.TrimSpace(m[1])
	}
	if strings.HasPrefix(s, "-") {
		negative = true
		s = strings.TrimSpace(strings.TrimPrefix(s, "-"))
	} else if strings.HasPrefix(s, "+") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "+"))
	}
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ".", "")  // Italian thousands separator
	s = strings.ReplaceAll(s, ",", ".") // Italian decimal separator → dot
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	if negative {
		v = -v
	}
	return v, true
}

// ---------------------------------------------------------------------------
// Markdown line/table primitives.
// ---------------------------------------------------------------------------

// maDocLine is one markdown line tagged with its 1-based page number.
type maDocLine struct {
	Page int
	Text string
}

// maFlattenPages returns every markdown line across all pages in page order.
func maFlattenPages(pages []maFilingPage) []maDocLine {
	sorted := append([]maFilingPage(nil), pages...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].PageNo < sorted[j].PageNo })
	var out []maDocLine
	for _, p := range sorted {
		for _, line := range strings.Split(p.Markdown, "\n") {
			out = append(out, maDocLine{Page: p.PageNo, Text: line})
		}
	}
	return out
}

// maMarkdownRow splits a "| a | b | c |" row into trimmed cells; ok=false when the line
// is not a table row.
func maMarkdownRow(line string) ([]string, bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "|") {
		return nil, false
	}
	t = strings.Trim(t, "|")
	parts := strings.Split(t, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells, true
}

// maIsSeparatorRow detects a |---|:--:| markdown separator line.
func maIsSeparatorRow(cells []string) bool {
	sawDash := false
	for _, c := range cells {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		for _, r := range c {
			if r != '-' && r != ':' {
				return false
			}
		}
		sawDash = true
	}
	return sawDash
}

var maExerciseDateRe = regexp.MustCompile(`(\d{1,2})[-/.](\d{1,2})[-/.](\d{4})`)

// maParseDMY parses a dd-mm-yyyy / dd/mm/yyyy date anywhere in the cell.
func maParseDMY(cell string) (time.Time, bool) {
	m := maExerciseDateRe.FindStringSubmatch(cell)
	if m == nil {
		return time.Time{}, false
	}
	day, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	year, _ := strconv.Atoi(m[3])
	if month < 1 || month > 12 || day < 1 || day > 31 || year < 1900 || year > 2100 {
		return time.Time{}, false
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), true
}

// maNormalizeLabel lowercases and collapses whitespace of a table label for matching.
func maNormalizeLabel(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

// maExerciseColumn maps a table column index to its exercise closing date.
type maExerciseColumn struct {
	Index int
	Date  time.Time
}

// maHeaderColumns reads the exercise columns from a table header row (the cells that
// carry a date). Header column 0 is the label column and never an exercise.
func maHeaderColumns(header []string) []maExerciseColumn {
	var cols []maExerciseColumn
	for i, cell := range header {
		if i == 0 {
			continue
		}
		if d, ok := maParseDMY(cell); ok {
			cols = append(cols, maExerciseColumn{Index: i, Date: d})
		}
	}
	return cols
}

// ---------------------------------------------------------------------------
// Prospetti extraction.
// ---------------------------------------------------------------------------

var maMarkdownHeadingRe = regexp.MustCompile(`^\s*#{1,6}\s+\S`)

// maNIStartIndex returns the flat-line index where the nota integrativa begins (the
// first line mentioning it), or len(lines) when there is no NI region.
func maNIStartIndex(lines []maDocLine) int {
	for i, l := range lines {
		if strings.Contains(strings.ToLower(l.Text), "nota integrativa") {
			return i
		}
	}
	return len(lines)
}

// maProspettiTables collects the SP and CE tables from the prospetti region (before the
// NI): the date header row and the data rows of each. Section is driven by the
// "Stato patrimoniale" / "Conto economico" HEADINGS (non-table lines), so the NI's later
// restatements — which live after the cut — never leak in.
func maProspettiTables(lines []maDocLine) (spHeader []string, spRows [][]string, ceHeader []string, ceRows [][]string) {
	section := ""
	for _, l := range lines {
		cells, isRow := maMarkdownRow(l.Text)
		if !isRow {
			norm := maNormalizeLabel(l.Text)
			switch {
			case strings.Contains(norm, "stato patrimoniale"):
				section = "sp"
			case strings.Contains(norm, "conto economico"):
				section = "ce"
			}
			continue
		}
		if maIsSeparatorRow(cells) {
			continue
		}
		if cols := maHeaderColumns(cells); len(cols) > 0 {
			switch section {
			case "sp":
				if spHeader == nil {
					spHeader = cells
				}
			case "ce":
				if ceHeader == nil {
					ceHeader = cells
				}
			}
			continue
		}
		switch section {
		case "sp":
			spRows = append(spRows, cells)
		case "ce":
			ceRows = append(ceRows, cells)
		}
	}
	return spHeader, spRows, ceHeader, ceRows
}

// maCellValue reads the number at column idx of a data row (ok=false when absent).
func maCellValue(row []string, idx int) (float64, bool) {
	if idx <= 0 || idx >= len(row) {
		return 0, false
	}
	return parseItalianNumber(row[idx])
}

func maExtractSP(rows [][]string, colIdx int) maFilingSP {
	var sp maFilingSP
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		label := maNormalizeLabel(row[0])
		val, ok := maCellValue(row, colIdx)
		if !ok {
			continue
		}
		v := val
		switch {
		case strings.Contains(label, "totale attivo circolante"):
			sp.TotaleAttivoCircolante = &v
		case strings.Contains(label, "totale attivo"):
			sp.TotaleAttivo = &v
		case strings.Contains(label, "totale passivo"):
			sp.TotalePassivo = &v
		case strings.Contains(label, "totale patrimonio netto"):
			sp.TotalePatrimonioNetto = &v
		case strings.Contains(label, "totale immobilizzazioni"):
			sp.TotaleImmobilizzazioni = &v
		case strings.Contains(label, "totale debiti"):
			sp.TotaleDebiti = &v
		case strings.Contains(label, "trattamento di fine rapporto"):
			sp.TFR = &v
		}
	}
	return sp
}

func maExtractCE(rows [][]string, colIdx int) maFilingCE {
	var ce maFilingCE
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		label := maNormalizeLabel(row[0])
		val, ok := maCellValue(row, colIdx)
		if !ok {
			continue
		}
		v := val
		switch {
		case strings.Contains(label, "totale valore della produzione"):
			ce.ValoreProduzione = &v
		case strings.Contains(label, "totale costi della produzione"):
			ce.CostiProduzione = &v
		case strings.Contains(label, "differenza tra valore e costi della produzione"):
			ce.DifferenzaAB = &v
		case strings.Contains(label, "utile (perdita) dell'esercizio"):
			ce.UtileEsercizio = &v
		}
	}
	return ce
}

// maArithmeticChecks computes the deterministic quadrature checks of one exercise:
// totale attivo == totale passivo, and A−B == the declared differenza.
func maArithmeticChecks(sp maFilingSP, ce maFilingCE) []maFilingCheck {
	var checks []maFilingCheck
	if sp.TotaleAttivo != nil && sp.TotalePassivo != nil {
		expected := *sp.TotalePassivo
		actual := *sp.TotaleAttivo
		checks = append(checks, maFilingCheck{
			Name:     "attivo_uguale_passivo",
			Passed:   math.Abs(actual-expected) <= maFilingCheckTolerance,
			Expected: &expected,
			Actual:   &actual,
		})
	}
	if ce.ValoreProduzione != nil && ce.CostiProduzione != nil && ce.DifferenzaAB != nil {
		expected := *ce.ValoreProduzione - *ce.CostiProduzione
		actual := *ce.DifferenzaAB
		checks = append(checks, maFilingCheck{
			Name:     "differenza_a_meno_b",
			Passed:   math.Abs(actual-expected) <= maFilingCheckTolerance,
			Expected: &expected,
			Actual:   &actual,
		})
	}
	return checks
}

// maNICrossChecks compares the NI's restated valore/costi della produzione for the
// primary exercise (the "Saldo al <date>" tables of the abbreviato NI) against the CE
// prospetto — the prosa↔tabella cross check. Only emits a check when the NI declares a
// figure already extracted from the prospetto.
func maNICrossChecks(niLines []maDocLine, primaryDate time.Time, ce maFilingCE) []maFilingCheck {
	var checks []maFilingCheck
	if v, ok := maNISaldoUnderHeading(niLines, "valore della produzione", primaryDate); ok && ce.ValoreProduzione != nil {
		expected := *ce.ValoreProduzione
		actual := v
		checks = append(checks, maFilingCheck{
			Name:     "cross_valore_produzione",
			Passed:   math.Abs(actual-expected) <= maFilingCheckTolerance,
			Expected: &expected,
			Actual:   &actual,
		})
	}
	if v, ok := maNISaldoUnderHeading(niLines, "costi della produzione", primaryDate); ok && ce.CostiProduzione != nil {
		expected := *ce.CostiProduzione
		actual := v
		checks = append(checks, maFilingCheck{
			Name:     "cross_costi_produzione",
			Passed:   math.Abs(actual-expected) <= maFilingCheckTolerance,
			Expected: &expected,
			Actual:   &actual,
		})
	}
	return checks
}

// maNISaldoUnderHeading finds, after an NI heading matching `heading`, the first
// "Saldo al <date>" table and returns the value under the column for primaryDate.
func maNISaldoUnderHeading(niLines []maDocLine, heading string, primaryDate time.Time) (float64, bool) {
	inSection := false
	saldoCol := -1
	for _, l := range niLines {
		cells, isRow := maMarkdownRow(l.Text)
		if !isRow {
			norm := maNormalizeLabel(l.Text)
			if maMarkdownHeadingRe.MatchString(l.Text) || maIsCanonicalNITitle(norm) {
				// A heading resets the search scope: we only trust a Saldo table INSIDE the
				// target section.
				inSection = strings.Contains(norm, heading)
				saldoCol = -1
			}
			continue
		}
		if !inSection || maIsSeparatorRow(cells) {
			continue
		}
		if saldoCol < 0 {
			// Look for the "Saldo al <primaryDate>" header column.
			for i, c := range cells {
				lc := maNormalizeLabel(c)
				if strings.Contains(lc, "saldo al") {
					if d, ok := maParseDMY(c); ok && d.Equal(primaryDate) {
						saldoCol = i
					}
				}
			}
			continue
		}
		// First value row after the Saldo header: read the primary column. The Saldo table
		// has no leading label column, so column 0 IS a value (maCellValue guards idx>0 for
		// the labelled prospetti and is not used here).
		if saldoCol >= 0 && saldoCol < len(cells) {
			if v, ok := parseItalianNumber(cells[saldoCol]); ok {
				return v, true
			}
		}
	}
	return 0, false
}

var maTaxonomyRe = regexp.MustCompile(`itcc-ci-\d{4}-\d{2}-\d{2}`)

// maDetectTaxonomyVersion reads the declared XBRL taxonomy ("...tassonomia
// itcc-ci-2018-11-04") or falls back to "".
func maDetectTaxonomyVersion(lines []maDocLine) string {
	for _, l := range lines {
		if m := maTaxonomyRe.FindString(strings.ToLower(l.Text)); m != "" {
			return m
		}
	}
	return ""
}

// maDetectBalanceSheetType reads whether the fascicolo declares itself abbreviato or
// ordinario (art. 2435-bis for the abbreviato).
func maDetectBalanceSheetType(lines []maDocLine) string {
	for _, l := range lines {
		norm := maNormalizeLabel(l.Text)
		if strings.Contains(norm, "forma abbreviata") || strings.Contains(norm, "bilancio abbreviato") || strings.Contains(norm, "2435-bis") {
			return "bilancio abbreviato"
		}
	}
	for _, l := range lines {
		if strings.Contains(maNormalizeLabel(l.Text), "bilancio ordinario") {
			return "bilancio ordinario"
		}
	}
	return ""
}

// parseMAFilingProspetti is the deterministic CEE parse of a fascicolo's OCR pages: the
// SP/CE prospetti (before the NI), one extract per exercise column, arithmetic +
// prosa↔tabella checks, and document metadata (taxonomy, balance-sheet type). Returns a
// legible error when the prospetti tables cannot be recognized (the ingest then fails
// the filing with that reason).
func parseMAFilingProspetti(pages []maFilingPage) (maFilingParseResult, error) {
	lines := maFlattenPages(pages)
	if len(lines) == 0 {
		return maFilingParseResult{}, fmt.Errorf("prospetti non riconosciuti: documento vuoto")
	}
	niStart := maNIStartIndex(lines)
	spHeader, spRows, ceHeader, ceRows := maProspettiTables(lines[:niStart])

	columns := maHeaderColumns(spHeader)
	if len(columns) == 0 {
		columns = maHeaderColumns(ceHeader)
	}
	if len(columns) == 0 {
		return maFilingParseResult{}, fmt.Errorf("prospetti non riconosciuti: nessuna colonna esercizio individuata")
	}

	exercises := make([]maFilingExerciseExtract, 0, len(columns))
	for _, col := range columns {
		sp := maExtractSP(spRows, col.Index)
		ce := maExtractCE(ceRows, col.Index)
		exercises = append(exercises, maFilingExerciseExtract{
			ExerciseDate: col.Date,
			SP:           sp,
			CE:           ce,
			Checks:       maArithmeticChecks(sp, ce),
		})
	}

	primaryIdx := maMostRecentExerciseIndex(exercises)
	if primaryIdx < 0 || exercises[primaryIdx].SP.TotaleAttivo == nil || exercises[primaryIdx].SP.TotalePassivo == nil {
		return maFilingParseResult{}, fmt.Errorf("prospetti non riconosciuti: totali di stato patrimoniale assenti nell'esercizio corrente")
	}
	// prosa↔tabella cross-check on the primary exercise (NI restatements).
	cross := maNICrossChecks(lines[niStart:], exercises[primaryIdx].ExerciseDate, exercises[primaryIdx].CE)
	exercises[primaryIdx].Checks = append(exercises[primaryIdx].Checks, cross...)

	return maFilingParseResult{
		Exercises:        exercises,
		TaxonomyVersion:  maDetectTaxonomyVersion(lines),
		BalanceSheetType: maDetectBalanceSheetType(lines),
	}, nil
}

func maMostRecentExerciseIndex(exercises []maFilingExerciseExtract) int {
	best := -1
	for i := range exercises {
		if best < 0 || exercises[i].ExerciseDate.After(exercises[best].ExerciseDate) {
			best = i
		}
	}
	return best
}

// ---------------------------------------------------------------------------
// detectAdaptedComparative: peer-filing restated-column divergence.
// ---------------------------------------------------------------------------

// detectAdaptedComparative compares this fascicolo's exercises with the extracts of
// OTHER filings sharing the fiscal identity, for overlapping exercise dates. A totale
// attivo divergence beyond maFilingAdaptedTolerance is flagged; the filing with the
// greater closing_date prevails in the view, and at parity NO auto-selection is made
// (both stay visible — only the flag). Returns one entry per overlapping exercise.
func detectAdaptedComparative(current []maFilingExerciseExtract, currentClosing *time.Time, peers []maFilingPeerExtract, tolerance float64) []maFilingAdaptedComparative {
	peerByDate := make(map[string]maFilingPeerExtract, len(peers))
	for _, p := range peers {
		peerByDate[maDateValue(p.ExerciseDate)] = p
	}
	var out []maFilingAdaptedComparative
	for _, ex := range current {
		peer, ok := peerByDate[maDateValue(ex.ExerciseDate)]
		if !ok {
			continue
		}
		res := maFilingAdaptedComparative{
			ExerciseDate: ex.ExerciseDate,
			Expected:     ex.SP.TotaleAttivo,
			Actual:       peer.TotaleAttivo,
			Prevails:     maPrevailingFiling(currentClosing, peer.ClosingDate),
		}
		if ex.SP.TotaleAttivo != nil && peer.TotaleAttivo != nil {
			res.Diverged = math.Abs(*ex.SP.TotaleAttivo-*peer.TotaleAttivo) > tolerance
		}
		out = append(out, res)
	}
	return out
}

func maPrevailingFiling(current, peer *time.Time) string {
	if current == nil || peer == nil {
		return ""
	}
	if current.After(*peer) {
		return "current"
	}
	if current.Before(*peer) {
		return "peer"
	}
	return "" // parity: no auto-selection
}

// ---------------------------------------------------------------------------
// Identity validation (page 1).
// ---------------------------------------------------------------------------

type maIdentityOutcome int

const (
	maIdentityUnreadable maIdentityOutcome = iota // no fiscal code found on page 1
	maIdentityMatch                               // a found code matches the expected identity
	maIdentityMismatch                            // codes found but none match (hard block)
)

// maIdentityResult is the page-1 identity verdict plus the closing date read from the
// frontespizio ("Bilancio ... al 31-12-2024").
type maIdentityResult struct {
	Outcome     maIdentityOutcome
	FoundCodes  []string
	ClosingDate *time.Time
}

var (
	// maFiscalCodeRe matches an 11-digit P.IVA/CF or a 16-char personal codice fiscale in
	// UPPERCASED page text.
	maFiscalCodeRe = regexp.MustCompile(`\b([0-9]{11}|[A-Z]{6}[0-9]{2}[A-Z][0-9]{2}[A-Z][0-9]{3}[A-Z])\b`)
	// maFrontClosingRe reads the closing date after "al" on the frontespizio.
	maFrontClosingRe = regexp.MustCompile(`(?i)\bal\s+(\d{1,2})[-/.](\d{1,2})[-/.](\d{4})`)
)

// validateMAFilingIdentityPage1 checks page-1 fiscal identity against the expected VAT/CF
// (already-normalized clean columns). match ⇒ a found code equals the expected VAT
// (stable form) or tax; mismatch ⇒ readable codes found but none match (hard block, non
// overridable); unreadable ⇒ no code found (override possible). Also returns the
// frontespizio closing date when present.
func validateMAFilingIdentityPage1(page1Markdown, vatClean, taxClean string) maIdentityResult {
	res := maIdentityResult{Outcome: maIdentityUnreadable}
	if m := maFrontClosingRe.FindStringSubmatch(page1Markdown); m != nil {
		day, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		year, _ := strconv.Atoi(m[3])
		if month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			res.ClosingDate = &d
		}
	}

	upper := strings.ToUpper(page1Markdown)
	seen := map[string]bool{}
	var found []string
	for _, m := range maFiscalCodeRe.FindAllStringSubmatch(upper, -1) {
		code := normalizeMAFiscalValue(m[1])
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		found = append(found, code)
	}
	res.FoundCodes = found
	if len(found) == 0 {
		res.Outcome = maIdentityUnreadable
		return res
	}

	expected := map[string]bool{}
	addExpected := func(v string) {
		if v = normalizeMAFiscalValue(v); v != "" {
			expected[v] = true
		}
	}
	addExpected(vatClean)
	addExpected(maStableVAT(vatClean))
	addExpected(taxClean)
	addExpected(maStableVAT(taxClean))

	for _, code := range found {
		if expected[code] || expected[maStableVAT(code)] {
			res.Outcome = maIdentityMatch
			return res
		}
	}
	res.Outcome = maIdentityMismatch
	return res
}

// ---------------------------------------------------------------------------
// NI sectioning (for F6 chunking).
// ---------------------------------------------------------------------------

// maNISection is one nota-integrativa section: its title, the page range it spans, and
// its markdown. F6 chunks the LLM reading per section (with pages for citations).
type maNISection struct {
	Label     string `json:"label"`
	StartPage int    `json:"startPage"`
	EndPage   int    `json:"endPage"`
	Markdown  string `json:"markdown"`
}

// maCanonicalNITitles are the abbreviato/ordinario NI section titles recognized as
// section boundaries even if OCR does not emit a markdown heading marker.
var maCanonicalNITitles = []string{
	"criteri di valutazione",
	"immobilizzazioni",
	"attivo circolante",
	"crediti iscritti nell'attivo circolante",
	"disponibilità liquide",
	"ratei e risconti attivi",
	"patrimonio netto",
	"trattamento di fine rapporto",
	"debiti",
	"ratei e risconti passivi",
	"valore della produzione",
	"costi della produzione",
	"compensi",
	"impegni, garanzie",
	"informazioni sulle operazioni con parti correlate",
	"informazioni sui fatti di rilievo",
	"proposta di destinazione degli utili",
}

func maIsCanonicalNITitle(norm string) bool {
	if norm == "" {
		return false
	}
	// A canonical title line is short (a heading, not prose) and matches a known title.
	if len(strings.Fields(norm)) > 12 {
		return false
	}
	for _, t := range maCanonicalNITitles {
		if norm == t || strings.HasPrefix(norm, t) {
			return true
		}
	}
	return false
}

// maCleanHeading strips markdown heading markers/emphasis from a heading line for the
// section label.
func maCleanHeading(text string) string {
	t := strings.TrimSpace(text)
	t = strings.TrimLeft(t, "#")
	t = strings.Trim(t, " *_")
	return strings.TrimSpace(t)
}

// sectionMAFilingNI splits the nota integrativa into canonical sections by heading,
// each carrying its page range and markdown. Falls back to a single "nota_integrativa"
// section when the NI has no recognizable sub-headings; returns nil when there is no NI.
func sectionMAFilingNI(pages []maFilingPage) []maNISection {
	lines := maFlattenPages(pages)
	niStart := maNIStartIndex(lines)
	if niStart >= len(lines) {
		return nil
	}
	niLines := lines[niStart:]

	var sections []maNISection
	var cur *maNISection
	var buf []string
	flush := func() {
		if cur == nil {
			return
		}
		cur.Markdown = strings.TrimSpace(strings.Join(buf, "\n"))
		sections = append(sections, *cur)
		cur = nil
		buf = nil
	}
	openSection := func(label string, page int) {
		flush()
		cur = &maNISection{Label: label, StartPage: page, EndPage: page}
		buf = nil
	}

	for _, l := range niLines {
		_, isRow := maMarkdownRow(l.Text)
		norm := maNormalizeLabel(l.Text)
		isHeading := !isRow && (maMarkdownHeadingRe.MatchString(l.Text) || maIsCanonicalNITitle(norm))
		if isHeading {
			openSection(maCleanHeading(l.Text), l.Page)
		}
		if cur == nil {
			// Preamble before the first heading: hold it in an implicit NI section.
			cur = &maNISection{Label: "nota_integrativa", StartPage: l.Page, EndPage: l.Page}
		}
		if l.Page > cur.EndPage {
			cur.EndPage = l.Page
		}
		buf = append(buf, l.Text)
	}
	flush()

	if len(sections) == 0 {
		return nil
	}
	return sections
}

// maMarshalJSON is a small helper for the ingest to serialize a parse struct to a jsonb
// column payload (nil on marshal error, which jsonbArg treats as SQL NULL).
func maMarshalJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
