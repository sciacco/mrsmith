package binocolo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Approved test #4 (issue #78): the deterministic CEE parser + identity/sectioning/
// adapted-comparative helpers, driven ENTIRELY by frozen fixtures under testdata/filing/.
// Zero external calls — no llm.Service, no DocuEngine, no DB, no network: everything here
// operates on the pure parse functions.

// ---------------------------------------------------------------------------
// Fixture loading.
// ---------------------------------------------------------------------------

type fixturePage struct {
	PageNo int      `json:"pageNo"`
	Lines  []string `json:"lines"`
}

type fixtureDoc struct {
	Comment string        `json:"comment"`
	Source  string        `json:"source"`
	Pages   []fixturePage `json:"pages"`
}

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
		pages = append(pages, maFilingPage{PageNo: p.PageNo, Markdown: strings.Join(p.Lines, "\n")})
	}
	return pages
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

func firstPage(pages []maFilingPage, pageNo int) string {
	for _, p := range pages {
		if p.PageNo == pageNo {
			return p.Markdown
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// (a) Full 2024 fascicolo: two columns, quadrature checks all pass.
// ---------------------------------------------------------------------------

func TestParseMAFiling2024(t *testing.T) {
	pages := loadFilingFixture(t, "digital_system_2024.json")
	result, err := parseMAFilingProspetti(pages)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Exercises) != 2 {
		t.Fatalf("exercises = %d, want 2 (2024 corrente + 2023 comparativa)", len(result.Exercises))
	}
	if result.TaxonomyVersion != "itcc-ci-2018-11-04" {
		t.Errorf("taxonomy = %q, want itcc-ci-2018-11-04", result.TaxonomyVersion)
	}
	if result.BalanceSheetType != "bilancio abbreviato" {
		t.Errorf("balance sheet type = %q, want bilancio abbreviato", result.BalanceSheetType)
	}
	if cd := result.ClosingDate(); cd == nil || !cd.Equal(time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("closing date = %v, want 2024-12-31", cd)
	}

	ex := exerciseByYear(t, result.Exercises, 2024)
	wantFloat(t, ex.SP.TotaleAttivo, 1039105, "2024 totale attivo")
	wantFloat(t, ex.SP.TotalePassivo, 1039105, "2024 totale passivo")
	wantFloat(t, ex.SP.TotalePatrimonioNetto, 830875, "2024 patrimonio netto")
	wantFloat(t, ex.SP.TotaleDebiti, 154605, "2024 totale debiti")
	wantFloat(t, ex.SP.TFR, 53389, "2024 tfr")
	wantFloat(t, ex.CE.ValoreProduzione, 1148536, "2024 valore produzione")
	wantFloat(t, ex.CE.CostiProduzione, 888006, "2024 costi produzione")
	wantFloat(t, ex.CE.DifferenzaAB, 260530, "2024 differenza A-B")
	wantFloat(t, ex.CE.UtileEsercizio, 191760, "2024 utile")

	// Every check on the primary exercise passes, and the four expected checks are present.
	seen := map[string]bool{}
	for _, c := range ex.Checks {
		seen[c.Name] = true
		if !c.Passed {
			t.Errorf("2024 check %q NOT passed: expected=%v actual=%v", c.Name, deref(c.Expected), deref(c.Actual))
		}
	}
	for _, name := range []string{"attivo_uguale_passivo", "differenza_a_meno_b", "cross_valore_produzione", "cross_costi_produzione"} {
		if !seen[name] {
			t.Errorf("missing check %q on 2024 exercise (have %v)", name, seen)
		}
	}

	// The comparative column (2023) is a full extract too.
	ex23 := exerciseByYear(t, result.Exercises, 2023)
	wantFloat(t, ex23.SP.TotaleAttivo, 920586, "2023 totale attivo")
	wantFloat(t, ex23.CE.DifferenzaAB, 314876, "2023 differenza A-B")
	for _, c := range ex23.Checks {
		if !c.Passed {
			t.Errorf("2023 check %q NOT passed: expected=%v actual=%v", c.Name, deref(c.Expected), deref(c.Actual))
		}
	}
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// ---------------------------------------------------------------------------
// (b) Page-1 identity validation: match / mismatch / unreadable + closing date.
// ---------------------------------------------------------------------------

func TestValidateMAFilingIdentityPage1(t *testing.T) {
	pages := loadFilingFixture(t, "digital_system_2024.json")
	page1 := firstPage(pages, 1)
	if page1 == "" {
		t.Fatal("fixture page 1 empty")
	}

	// Match: the fixture's CF/P.IVA is 00888440252.
	match := validateMAFilingIdentityPage1(page1, "00888440252", "00888440252")
	if match.Outcome != maIdentityMatch {
		t.Errorf("expected match, got outcome %d (found %v)", match.Outcome, match.FoundCodes)
	}
	if match.ClosingDate == nil || !match.ClosingDate.Equal(time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("closing date = %v, want 2024-12-31", match.ClosingDate)
	}

	// Match also when the expected VAT carries the IT prefix (stable-key equivalence).
	if r := validateMAFilingIdentityPage1(page1, "IT00888440252", ""); r.Outcome != maIdentityMatch {
		t.Errorf("expected match with IT-prefixed vat, got %d", r.Outcome)
	}

	// Mismatch: a readable but different identity is a hard block.
	mismatch := validateMAFilingIdentityPage1(page1, "12345678901", "12345678901")
	if mismatch.Outcome != maIdentityMismatch {
		t.Errorf("expected mismatch, got outcome %d (found %v)", mismatch.Outcome, mismatch.FoundCodes)
	}

	// Unreadable: a truncated page 1 with no fiscal code (override possible).
	truncated := "# ACME SRL\n## Bilancio di esercizio al 31-12-2024\nDati anagrafici non leggibili."
	un := validateMAFilingIdentityPage1(truncated, "00888440252", "00888440252")
	if un.Outcome != maIdentityUnreadable {
		t.Errorf("expected unreadable, got outcome %d (found %v)", un.Outcome, un.FoundCodes)
	}
	if un.ClosingDate == nil || !un.ClosingDate.Equal(time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("unreadable closing date = %v, want 2024-12-31 (frontespizio still parsed)", un.ClosingDate)
	}
}

// ---------------------------------------------------------------------------
// (c) Single-column fascicolo (neocostituita): parse ok, no comparative.
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
// (d) NI sectioning: sections carry page ranges; pages 10 and 17 hold the quotes.
// ---------------------------------------------------------------------------

func TestSectionMAFilingNI(t *testing.T) {
	pages := loadFilingFixture(t, "digital_system_2024.json")
	sections := sectionMAFilingNI(pages)
	if len(sections) == 0 {
		t.Fatal("no NI sections detected")
	}
	for _, s := range sections {
		if s.StartPage < 4 {
			t.Errorf("section %q starts at page %d, before the NI (page 4)", s.Label, s.StartPage)
		}
		if s.EndPage < s.StartPage {
			t.Errorf("section %q has EndPage %d < StartPage %d", s.Label, s.EndPage, s.StartPage)
		}
	}

	assertPageQuote := func(page int, quote string) {
		for _, s := range sections {
			if s.StartPage <= page && page <= s.EndPage && strings.Contains(s.Markdown, quote) {
				return
			}
		}
		t.Errorf("no NI section covering page %d contains %q", page, quote)
	}
	// p.10: conto corrente vincolato 250.000; p.17: compensi 31.750 + canoni verso soci 18.000.
	assertPageQuote(10, "250.000")
	assertPageQuote(17, "31.750")
	assertPageQuote(17, "18.000")
}

// ---------------------------------------------------------------------------
// (e) Italian number parsing (thousands dots, negatives, parentheses, absent).
// ---------------------------------------------------------------------------

func TestParseItalianNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"1.039.105", 1039105, true},
		{"260.530", 260530, true},
		{"(4.506)", -4506, true},
		{"-6.709", -6709, true},
		{"250.000,00", 250000, true},
		{"4.915,00", 4915, true},
		{"1.131", 1131, true},
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
// (f) detectAdaptedComparative: divergence flags; parity ⇒ no auto-selection.
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
