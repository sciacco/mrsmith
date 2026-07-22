package binocolo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Approved test #4 (issue #78): the deterministic CEE parser + reassembly / identity /
// sectioning / adapted-comparative helpers, driven ENTIRELY by frozen fixtures under
// testdata/filing/. The fixtures reproduce the REAL Mistral OCR contract discovered in the
// production smoke: tables are NOT inline in the page markdown — each is a [tbl-N.md] link and
// its content lives in extras.tables[{id,format,content}]. Two layouts are covered: the CCIAA
// fascicolo with a copertina (2023) and the naked XBRL PDF whose fiscal code lives only inside
// the frontespizio table (2024). Zero external calls — no llm.Service, no DocuEngine, no DB,
// no network: everything here operates on the pure parse functions.

// ---------------------------------------------------------------------------
// Fixture loading (real OCR shape: page markdown + externalized tables in extras).
// ---------------------------------------------------------------------------

type fixtureTable struct {
	ID      string `json:"id"`
	Format  string `json:"format"`
	Content string `json:"content"`
}

type fixturePage struct {
	Page     int            `json:"page"`
	Markdown string         `json:"markdown"`
	Tables   []fixtureTable `json:"tables"`
}

type fixtureDoc struct {
	Comment string        `json:"comment"`
	Source  string        `json:"source"`
	Pages   []fixturePage `json:"pages"`
}

// loadFilingFixture reproduces the store's maFilingPage shape: the page markdown verbatim and
// its externalized tables re-encoded into the extras jsonb ({"tables":[{id,format,content}]}),
// exactly as InsertMAFilingPages persists the vendor payload.
func loadFilingFixture(t *testing.T, name string) []maFilingPage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "filing", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var doc fixtureDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	if len(doc.Pages) == 0 {
		t.Fatalf("fixture %s has no pages", name)
	}
	pages := make([]maFilingPage, 0, len(doc.Pages))
	for _, p := range doc.Pages {
		var extras json.RawMessage
		if len(p.Tables) > 0 {
			b, merr := json.Marshal(map[string]any{"tables": p.Tables})
			if merr != nil {
				t.Fatalf("marshal fixture extras (%s page %d): %v", name, p.Page, merr)
			}
			extras = b
		}
		pages = append(pages, maFilingPage{PageNo: p.Page, Markdown: p.Markdown, Extras: extras})
	}
	return pages
}

func filingDate(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func pageByNo(pages []maFilingPage, no int) maFilingPage {
	for _, p := range pages {
		if p.PageNo == no {
			return p
		}
	}
	return maFilingPage{}
}

func exerciseByYear(t *testing.T, exs []maFilingExerciseExtract, year int) maFilingExerciseExtract {
	t.Helper()
	for _, e := range exs {
		if e.ExerciseDate.Year() == year {
			return e
		}
	}
	t.Fatalf("no exercise for year %d (have %d exercises)", year, len(exs))
	return maFilingExerciseExtract{}
}

func wantFloat(t *testing.T, p *float64, want float64, label string) {
	t.Helper()
	if p == nil {
		t.Fatalf("%s: nil, want %.2f", label, want)
	}
	if diff := *p - want; diff > 0.5 || diff < -0.5 {
		t.Fatalf("%s: got %.2f, want %.2f", label, *p, want)
	}
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// ---------------------------------------------------------------------------
// (a) Table reassembly: [tbl-N.md] links become inline table rows for READING; the persisted
//     markdown is never rewritten; a link with no matching table is left verbatim.
// ---------------------------------------------------------------------------

func TestMAFilingPageTextReassembly(t *testing.T) {
	pages := loadFilingFixture(t, "digital_system_2023_cciaa.json")

	p3 := pageByNo(pages, 3) // Stato patrimoniale: markdown carries [tbl-1.md], content in extras.
	if !strings.Contains(p3.Markdown, "[tbl-1.md](tbl-1.md)") {
		t.Fatalf("fixture page 3 markdown must carry the table link, got:\n%s", p3.Markdown)
	}
	text := maFilingPageText(p3)
	if strings.Contains(text, "[tbl-1.md](tbl-1.md)") {
		t.Error("reassembled text must not still contain the raw table link")
	}
	if !strings.Contains(text, "Totale attivo | 920.586") {
		t.Errorf("reassembled text must inline the SP table rows, got:\n%s", text)
	}
	// The persisted page markdown is untouched by the (read-only) reassembly.
	if !strings.Contains(p3.Markdown, "[tbl-1.md](tbl-1.md)") {
		t.Error("maFilingPageText must not mutate the persisted page markdown")
	}

	// A link with no matching table (page 10's tbl-4/tbl-5 have no content) is left verbatim.
	p10 := pageByNo(pages, 10)
	if got := maFilingPageText(p10); !strings.Contains(got, "[tbl-4.md](tbl-4.md)") {
		t.Error("a table link with no matching extras.tables entry must be left verbatim")
	}
}

// ---------------------------------------------------------------------------
// (b) Real CCIAA fascicolo 2023: two columns (2023 + 2022 comparativa), quadrature passes.
// ---------------------------------------------------------------------------

func TestParseMAFiling2023CCIAA(t *testing.T) {
	pages := loadFilingFixture(t, "digital_system_2023_cciaa.json")
	result, err := parseMAFilingProspetti(pages)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Exercises) != 2 {
		t.Fatalf("exercises = %d, want 2 (2023 corrente + 2022 comparativa)", len(result.Exercises))
	}
	if result.TaxonomyVersion != "itcc-ci-2018-11-04" {
		t.Errorf("taxonomy = %q, want itcc-ci-2018-11-04", result.TaxonomyVersion)
	}
	if result.BalanceSheetType != "bilancio abbreviato" {
		t.Errorf("balance sheet type = %q, want bilancio abbreviato", result.BalanceSheetType)
	}
	if cd := result.ClosingDate(); cd == nil || !cd.Equal(filingDate(2023, 12, 31)) {
		t.Errorf("closing date = %v, want 2023-12-31", cd)
	}

	ex := exerciseByYear(t, result.Exercises, 2023)
	wantFloat(t, ex.SP.TotaleAttivo, 920586, "2023 totale attivo")
	wantFloat(t, ex.SP.TotalePassivo, 920586, "2023 totale passivo")
	wantFloat(t, ex.SP.TotalePatrimonioNetto, 639115, "2023 patrimonio netto")
	wantFloat(t, ex.SP.TotaleDebiti, 233446, "2023 totale debiti")
	wantFloat(t, ex.SP.TFR, 48018, "2023 tfr")
	wantFloat(t, ex.CE.ValoreProduzione, 1291757, "2023 valore produzione")
	wantFloat(t, ex.CE.CostiProduzione, 976881, "2023 costi produzione")
	wantFloat(t, ex.CE.DifferenzaAB, 314876, "2023 differenza A-B")
	wantFloat(t, ex.CE.UtileEsercizio, 234166, "2023 utile")

	seen := map[string]bool{}
	for _, c := range ex.Checks {
		seen[c.Name] = true
		if !c.Passed {
			t.Errorf("2023 check %q NOT passed: expected=%v actual=%v", c.Name, deref(c.Expected), deref(c.Actual))
		}
	}
	for _, name := range []string{"attivo_uguale_passivo", "differenza_a_meno_b"} {
		if !seen[name] {
			t.Errorf("missing check %q on 2023 exercise (have %v)", name, seen)
		}
	}

	// The comparative column (2022) is a full extract too.
	ex22 := exerciseByYear(t, result.Exercises, 2022)
	wantFloat(t, ex22.SP.TotaleAttivo, 676760, "2022 totale attivo")
	wantFloat(t, ex22.CE.DifferenzaAB, 233755, "2022 differenza A-B")
	for _, c := range ex22.Checks {
		if !c.Passed {
			t.Errorf("2022 check %q NOT passed: expected=%v actual=%v", c.Name, deref(c.Expected), deref(c.Actual))
		}
	}
}

// ---------------------------------------------------------------------------
// (c) Naked XBRL PDF 2024: SP/CE from the (synthetic-on-real-numbers) frontespizio+prospetti
//     tables; totals and A-B match the real PDF; quadrature passes.
// ---------------------------------------------------------------------------

func TestParseMAFiling2024Naked(t *testing.T) {
	pages := loadFilingFixture(t, "digital_system_2024_naked.json")
	result, err := parseMAFilingProspetti(pages)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Exercises) != 2 {
		t.Fatalf("exercises = %d, want 2 (2024 corrente + 2023 comparativa)", len(result.Exercises))
	}
	if cd := result.ClosingDate(); cd == nil || !cd.Equal(filingDate(2024, 12, 31)) {
		t.Errorf("closing date = %v, want 2024-12-31", cd)
	}

	ex := exerciseByYear(t, result.Exercises, 2024)
	wantFloat(t, ex.SP.TotaleAttivo, 1039105, "2024 totale attivo")
	wantFloat(t, ex.SP.TotalePassivo, 1039105, "2024 totale passivo")
	wantFloat(t, ex.CE.DifferenzaAB, 260530, "2024 differenza A-B")
	wantFloat(t, ex.CE.UtileEsercizio, 191760, "2024 utile")
	for _, c := range ex.Checks {
		if !c.Passed {
			t.Errorf("2024 check %q NOT passed: expected=%v actual=%v", c.Name, deref(c.Expected), deref(c.Actual))
		}
	}

	// Comparative column (2023) matches the real 2023 numbers.
	ex23 := exerciseByYear(t, result.Exercises, 2023)
	wantFloat(t, ex23.SP.TotaleAttivo, 920586, "2024 fascicolo, 2023 comparativa totale attivo")
	wantFloat(t, ex23.CE.DifferenzaAB, 314876, "2024 fascicolo, 2023 comparativa differenza A-B")
}

// ---------------------------------------------------------------------------
// (d) Identity validation across layouts: copertina in clear, frontespizio table, naked
//     no-tables (unreadable), and a readable but different code (hard mismatch).
// ---------------------------------------------------------------------------

func TestValidateMAFilingIdentity(t *testing.T) {
	pages2023 := loadFilingFixture(t, "digital_system_2023_cciaa.json")

	// Copertina CCIAA: "Codice fiscale: 00888440252" in clear on page 1.
	match := validateMAFilingIdentity(pages2023, "00888440252", "00888440252")
	if match.Outcome != maIdentityMatch {
		t.Errorf("copertina: expected match, got outcome %d (found %v)", match.Outcome, match.FoundCodes)
	}
	if match.ClosingDate == nil || !match.ClosingDate.Equal(filingDate(2023, 12, 31)) {
		t.Errorf("copertina closing date = %v, want 2023-12-31", match.ClosingDate)
	}

	// IT-prefixed expected VAT still matches (stable-key equivalence).
	if r := validateMAFilingIdentity(pages2023, "IT00888440252", ""); r.Outcome != maIdentityMatch {
		t.Errorf("expected match with IT-prefixed vat, got %d", r.Outcome)
	}

	// Frontespizio tabellare: the CF lives ONLY inside tbl-0 on page 2 — reassembly must
	// surface it. Prove it by scanning page 2 alone (no clear-text copertina).
	page2 := []maFilingPage{pageByNo(pages2023, 2)}
	if r := validateMAFilingIdentity(page2, "00888440252", ""); r.Outcome != maIdentityMatch {
		t.Errorf("frontespizio-table CF must validate via reassembly, got %d (found %v)", r.Outcome, r.FoundCodes)
	}

	// Readable but different identity ⇒ hard mismatch.
	if r := validateMAFilingIdentity(pages2023, "12345678901", "12345678901"); r.Outcome != maIdentityMismatch {
		t.Errorf("expected mismatch, got outcome %d (found %v)", r.Outcome, r.FoundCodes)
	}

	// Naked layout WITHOUT extras: only the [tbl-0.md] link survives, so no fiscal code is
	// readable ⇒ unreadable (exactly the state that blocked the real 2024 run before this fix;
	// re-OCR then persists the table extras and the code becomes readable). The closing date is
	// still parsed from the naked front matter.
	naked := []maFilingPage{
		{PageNo: 1, Markdown: "v.2.14.1\n\nDIGITAL SYSTEM SRL\n\n# DIGITAL SYSTEM SRL\n\n## Bilancio di esercizio al 31-12-2024\n\n[tbl-0.md](tbl-0.md)\n\nPag. 1 di 19"},
		{PageNo: 2, Markdown: "# Stato patrimoniale\n\n[tbl-1.md](tbl-1.md)"},
	}
	un := validateMAFilingIdentity(naked, "00888440252", "00888440252")
	if un.Outcome != maIdentityUnreadable {
		t.Errorf("naked-no-tables: expected unreadable, got outcome %d (found %v)", un.Outcome, un.FoundCodes)
	}
	if un.ClosingDate == nil || !un.ClosingDate.Equal(filingDate(2024, 12, 31)) {
		t.Errorf("naked closing date = %v, want 2024-12-31 (front matter still parsed)", un.ClosingDate)
	}
}

// ---------------------------------------------------------------------------
// (e) Closing date extraction from the three real front-matter phrasings.
// ---------------------------------------------------------------------------

func TestMAExtractClosingDate(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"Data chiusura esercizio 31/12/2023", filingDate(2023, 12, 31), true}, // copertina, dd/mm/yyyy, no "al"
		{"## Bilancio di esercizio al 31-12-2024", filingDate(2024, 12, 31), true},
		{"Bilancio aggiornato al 31/12/2023", filingDate(2023, 12, 31), true},
		{"Nessuna data di chiusura qui", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := maExtractClosingDate(c.in)
		if ok != c.ok {
			t.Errorf("maExtractClosingDate(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && !got.Equal(c.want) {
			t.Errorf("maExtractClosingDate(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// (f) NI sectioning across both real layouts: the sections cover the pages that carry the NI
//     quotes even though the fascicolo interleaves the note detail after the prospetti.
// ---------------------------------------------------------------------------

func TestSectionMAFilingNIReal(t *testing.T) {
	type pq struct {
		page  int
		quote string
	}
	check := func(name string, wants []pq) {
		pages := loadFilingFixture(t, name)
		sections := sectionMAFilingNI(pages)
		if len(sections) == 0 {
			t.Fatalf("%s: no NI sections detected", name)
		}
		for _, w := range wants {
			found := false
			for _, s := range sections {
				if s.StartPage <= w.page && w.page <= s.EndPage && strings.Contains(s.Markdown, w.quote) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: no NI section covering page %d contains %q", name, w.page, w.quote)
			}
		}
	}
	// 2023: conto vincolato 250.000 (p.10); compensi amministratori 30.000 + canoni soci 18.000 (p.18).
	check("digital_system_2023_cciaa.json", []pq{
		{10, "250.000"},
		{18, "30.000"},
		{18, "18.000"},
	})
	// 2024: conto vincolato 250.000 (p.10); compensi 31.750 + canoni soci 18.000 (p.17).
	check("digital_system_2024_naked.json", []pq{
		{10, "250.000"},
		{17, "31.750"},
		{17, "18.000"},
	})
}

// ---------------------------------------------------------------------------
// (g) Single-column fascicolo (neocostituita): parse ok, no comparative.
// ---------------------------------------------------------------------------

func TestParseMAFilingSingleColumn(t *testing.T) {
	pages := loadFilingFixture(t, "neocostituita_1col.json")
	result, err := parseMAFilingProspetti(pages)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Exercises) != 1 {
		t.Fatalf("exercises = %d, want 1 (no comparative column)", len(result.Exercises))
	}
	ex := result.Exercises[0]
	if ex.ExerciseDate.Year() != 2024 {
		t.Errorf("exercise year = %d, want 2024", ex.ExerciseDate.Year())
	}
	wantFloat(t, ex.SP.TotaleAttivo, 100000, "totale attivo")
	wantFloat(t, ex.SP.TotalePassivo, 100000, "totale passivo")
	wantFloat(t, ex.CE.DifferenzaAB, 20000, "differenza A-B")
	for _, c := range ex.Checks {
		if !c.Passed {
			t.Errorf("check %q NOT passed", c.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// (h) Italian number parsing: thousands dots, negatives (paren + sign), padding, absent.
// ---------------------------------------------------------------------------

func TestParseItalianNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"1.039.105", 1039105, true},
		{"260.530", 260530, true},
		{"(6.709)", -6709, true}, // real CE negative in parentheses
		{"(4.506)", -4506, true},
		{"-6.709", -6709, true},
		{"250.000,00", 250000, true},
		{" 920.586 ", 920586, true}, // padded cell
		{"31-12-2022  ", 0, false},  // a date cell is not a number
		{"920.586  ", 920586, true}, // trailing padding
		{"0", 0, true},
		{"-", 0, false},
		{"", 0, false},
		{"Euro 18.000", 18000, true},
	}
	for _, c := range cases {
		got, ok := parseItalianNumber(c.in)
		if ok != c.ok {
			t.Errorf("parseItalianNumber(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && (got-c.want > 0.001 || got-c.want < -0.001) {
			t.Errorf("parseItalianNumber(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// (i) detectAdaptedComparative: divergence flags; parity ⇒ no auto-selection. (Unchanged.)
// ---------------------------------------------------------------------------

func TestDetectAdaptedComparative(t *testing.T) {
	d2023 := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	d2024 := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	f := func(v float64) *float64 { return &v }

	current := []maFilingExerciseExtract{
		{ExerciseDate: d2023, SP: maFilingSP{TotaleAttivo: f(920586)}},
	}

	// Divergence beyond tolerance (>1 EUR): this fascicolo (closes 2024) restates 2023 with
	// a different total than the 2023 fascicolo ⇒ flag; the later filing prevails.
	peersDiverge := []maFilingPeerExtract{
		{ExerciseDate: d2023, ClosingDate: &d2023, TotaleAttivo: f(925000)},
	}
	res := detectAdaptedComparative(current, &d2024, peersDiverge, maFilingAdaptedTolerance)
	if len(res) != 1 {
		t.Fatalf("results = %d, want 1 overlapping exercise", len(res))
	}
	if !res[0].Diverged {
		t.Errorf("expected divergence flag for |920586-925000| > 1 EUR")
	}
	if res[0].Prevails != "current" {
		t.Errorf("prevails = %q, want current (2024 fascicolo newer than 2023 peer)", res[0].Prevails)
	}

	// Within tolerance: no flag.
	peersSame := []maFilingPeerExtract{
		{ExerciseDate: d2023, ClosingDate: &d2023, TotaleAttivo: f(920586)},
	}
	res = detectAdaptedComparative(current, &d2024, peersSame, maFilingAdaptedTolerance)
	if len(res) != 1 || res[0].Diverged {
		t.Errorf("expected no divergence for identical totals, got %+v", res)
	}

	// Parity of closing dates ⇒ no auto-selection (both stay visible, only the flag).
	res = detectAdaptedComparative(current, &d2023, peersDiverge, maFilingAdaptedTolerance)
	if len(res) != 1 {
		t.Fatalf("results = %d, want 1", len(res))
	}
	if res[0].Prevails != "" {
		t.Errorf("prevails = %q, want empty at closing-date parity", res[0].Prevails)
	}
	if !res[0].Diverged {
		t.Errorf("divergence should still be flagged at parity")
	}

	// No overlap ⇒ no result (different exercise date).
	if r := detectAdaptedComparative(current, &d2024, []maFilingPeerExtract{{ExerciseDate: d2024, ClosingDate: &d2024, TotaleAttivo: f(1)}}, maFilingAdaptedTolerance); len(r) != 0 {
		t.Errorf("expected no overlap results, got %d", len(r))
	}
}
