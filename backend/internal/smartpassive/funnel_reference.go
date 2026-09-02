package smartpassive

import (
	"regexp"
	"sort"
	"strings"
)

// The AFC team records the reconciliation outcome in the invoice header note as
// free text such as "CHIUSO - PO-50321/2026 E PO-50422/2026",
// "CHIUSI: PO-50205-50206-50207-50208" or "PA MAI CREATI". Separators, year
// placement and status wording vary, so the parser scans for code chains
// anywhere in the note and reads the leading words as the AFC status without
// interpreting the structure.
var (
	// A chain is a prefix followed by numbers separated by "-" or "/". Inside a
	// chain, a four-digit number starting with 20 is the year; the others are
	// document numbers sharing that year.
	arakChainPattern   = regexp.MustCompile(`(?i)\bPO\s*[-/]\s*(\d+(?:\s*[-/]\s*\d+)*)`)
	legacyChainPattern = regexp.MustCompile(`(?i)\bPA\s*[-/]?\s*(\d+(?:\s*[-/]\s*\d+)*)`)
	chainSplitPattern  = regexp.MustCompile(`\s*[-/]\s*`)
	anyCodePattern     = regexp.MustCompile(`(?i)\bP[OA]\s*[-/]?\s*\d`)
	noRDAPattern       = regexp.MustCompile(`(?i)\bPA\s+MAI\s+C\p{L}*`)
	afcStatusPattern   = regexp.MustCompile(`^[\p{L}][\p{L} ]*$`)
)

type funnelReferenceOutcome string

const (
	referenceNoNote        funnelReferenceOutcome = "no_note"
	referenceNoCode        funnelReferenceOutcome = "no_code"
	referenceNoRDADeclared funnelReferenceOutcome = "no_rda_declared"
	referenceLegacyOnly    funnelReferenceOutcome = "legacy_only"
	referenceUnresolved    funnelReferenceOutcome = "unresolved"
	referenceAmbiguous     funnelReferenceOutcome = "ambiguous"
	referenceOne           funnelReferenceOutcome = "one"
	referenceMultiple      funnelReferenceOutcome = "multiple"
)

// noteCode is one document number read from a note; year is empty when the
// note omits it.
type noteCode struct {
	number string
	year   string
}

func (c noteCode) arakCode() string {
	if c.year == "" {
		return "PO-" + c.number
	}
	return "PO-" + c.number + "/" + c.year
}

func (c noteCode) legacyCode() string {
	if c.year == "" {
		return "PA/" + c.number
	}
	return "PA/" + c.number + "-" + c.year
}

type parsedNote struct {
	status        string
	noRDADeclared bool
	arakCodes     []noteCode
	legacyCodes   []noteCode
}

func parseAFCNote(note string) parsedNote {
	trimmed := strings.TrimSpace(note)
	out := parsedNote{}
	if trimmed == "" {
		return out
	}
	out.arakCodes = uniqueCodes(collectChains(arakChainPattern, trimmed))
	out.legacyCodes = uniqueCodes(collectChains(legacyChainPattern, trimmed))
	if len(out.arakCodes) == 0 && len(out.legacyCodes) == 0 {
		out.noRDADeclared = noRDAPattern.MatchString(trimmed)
		return out
	}
	out.status = leadingStatus(trimmed)
	return out
}

// leadingStatus returns the words written before the first code, such as
// "CHIUSO" or "IN APPROVAZIONE", once separators are stripped. Notes that start
// with a code carry no status.
func leadingStatus(text string) string {
	loc := anyCodePattern.FindStringIndex(text)
	if loc == nil || loc[0] == 0 {
		return ""
	}
	prefix := strings.TrimRight(text[:loc[0]], " \t-:–")
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || !afcStatusPattern.MatchString(prefix) {
		return ""
	}
	return strings.ToUpper(prefix)
}

func collectChains(pattern *regexp.Regexp, text string) []noteCode {
	out := make([]noteCode, 0)
	for _, m := range pattern.FindAllStringSubmatch(text, -1) {
		parts := chainSplitPattern.Split(strings.TrimSpace(m[1]), -1)
		numbers := make([]string, 0, len(parts))
		year := ""
		for _, part := range parts {
			if isYear(part) {
				year = part
				continue
			}
			if part != "" {
				numbers = append(numbers, part)
			}
		}
		for _, number := range numbers {
			out = append(out, noteCode{number: number, year: year})
		}
	}
	return out
}

func isYear(value string) bool {
	return len(value) == 4 && strings.HasPrefix(value, "20")
}

func uniqueCodes(values []noteCode) []noteCode {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[noteCode]struct{}, len(values))
	out := make([]noteCode, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// MatchingFunnelReferenceSummary counts invoices by how the AFC note resolves
// against the RDA universe. It is the ground truth used to measure the funnel,
// never an input of the matching itself.
type MatchingFunnelReferenceSummary struct {
	NoNote           int `json:"no_note"`
	NoCode           int `json:"no_code"`
	NoRDADeclared    int `json:"no_rda_declared"`
	LegacyOnly       int `json:"legacy_only"`
	Unresolved       int `json:"unresolved"`
	Ambiguous        int `json:"ambiguous"`
	OneRDA           int `json:"one_rda"`
	MultipleRDAs     int `json:"multiple_rdas"`
	ArakAndLegacy    int `json:"arak_and_legacy"`
	LegacyPromoted   int `json:"legacy_promoted"`
	SupplierMismatch int `json:"supplier_mismatch"`
}

type MatchingFunnelReferencedRDA struct {
	Code          string   `json:"code"`
	LegacyCode    string   `json:"legacy_code"`
	ID            *int64   `json:"id"`
	State         string   `json:"state"`
	SupplierERPID *int64   `json:"supplier_erp_id"`
	Resolution    string   `json:"resolution"`
	SupplierMatch *bool    `json:"supplier_match"`
	InCandidates  bool     `json:"in_candidates"`
	Total         *float64 `json:"total"`
}

type MatchingFunnelReference struct {
	Note        string                        `json:"note"`
	AFCStatus   string                        `json:"afc_status"`
	Outcome     funnelReferenceOutcome        `json:"outcome"`
	RDAs        []MatchingFunnelReferencedRDA `json:"rdas"`
	LegacyCodes []string                      `json:"legacy_codes"`
}

type funnelReferenceIndex struct {
	byCode   map[string][]funnelRDA
	byNumber map[string][]funnelRDA
	// byLegacy maps a legacy PA number quoted in an RDA object (e.g.
	// "Rinnovo PA-7514") to the Arak RDAs that replaced it.
	byLegacy map[string][]funnelRDA
}

var rdaCodeShape = regexp.MustCompile(`^PO-(\d+)/(\d{4})$`)

func newFunnelReferenceIndex(rdas []funnelRDA) funnelReferenceIndex {
	idx := funnelReferenceIndex{
		byCode:   make(map[string][]funnelRDA, len(rdas)),
		byNumber: make(map[string][]funnelRDA, len(rdas)),
		byLegacy: make(map[string][]funnelRDA),
	}
	for _, rda := range rdas {
		for _, legacy := range uniqueCodes(collectChains(legacyChainPattern, rda.object)) {
			idx.byLegacy[legacy.number] = append(idx.byLegacy[legacy.number], rda)
		}
		code := strings.ToUpper(strings.TrimSpace(rda.code))
		if code == "" {
			continue
		}
		idx.byCode[code] = append(idx.byCode[code], rda)
		if m := rdaCodeShape.FindStringSubmatch(code); m != nil {
			idx.byNumber[m[1]] = append(idx.byNumber[m[1]], rda)
		}
	}
	return idx
}

// rdasWithLegacyPredecessor counts RDAs whose object quotes a legacy PA code.
func (idx funnelReferenceIndex) rdasWithLegacyPredecessor() int {
	seen := make(map[int64]struct{})
	for _, list := range idx.byLegacy {
		for _, rda := range list {
			seen[rda.id] = struct{}{}
		}
	}
	return len(seen)
}

// lookup resolves a note code: with a year it must match the full RDA code;
// without a year it matches on the number across years.
func (idx funnelReferenceIndex) lookup(code noteCode) []funnelRDA {
	if code.year != "" {
		return idx.byCode[code.arakCode()]
	}
	return idx.byNumber[code.number]
}

// duplicateCodes counts RDA codes shared by more than one RDA in the universe.
func (idx funnelReferenceIndex) duplicateCodes() int {
	count := 0
	for _, list := range idx.byCode {
		if len(list) > 1 {
			count++
		}
	}
	return count
}

func (idx funnelReferenceIndex) resolve(invoice funnelInvoice, note parsedNote) MatchingFunnelReference {
	ref := MatchingFunnelReference{
		Note:        strings.TrimSpace(invoice.note),
		AFCStatus:   note.status,
		RDAs:        make([]MatchingFunnelReferencedRDA, 0, len(note.arakCodes)+len(note.legacyCodes)),
		LegacyCodes: make([]string, 0, len(note.legacyCodes)),
	}

	if ref.Note == "" {
		ref.Outcome = referenceNoNote
		return ref
	}
	if len(note.arakCodes) == 0 && len(note.legacyCodes) == 0 {
		if note.noRDADeclared {
			ref.Outcome = referenceNoRDADeclared
		} else {
			ref.Outcome = referenceNoCode
		}
		return ref
	}

	hasUnresolved := false
	hasAmbiguous := false
	resolved := make(map[int64]struct{})
	for _, code := range note.arakCodes {
		matches := idx.lookup(code)
		switch len(matches) {
		case 0:
			hasUnresolved = true
			ref.RDAs = append(ref.RDAs, MatchingFunnelReferencedRDA{Code: code.arakCode(), Resolution: "unresolved"})
		case 1:
			resolved[matches[0].id] = struct{}{}
			ref.RDAs = append(ref.RDAs, idx.referencedRDA(invoice, matches[0], "resolved", ""))
		default:
			hasAmbiguous = true
			ref.RDAs = append(ref.RDAs, MatchingFunnelReferencedRDA{Code: code.arakCode(), Resolution: "ambiguous"})
		}
	}

	// Legacy PA codes are promoted to the Arak RDAs that quote them in the
	// object; the others stay listed as legacy codes without a successor.
	for _, code := range note.legacyCodes {
		successors := idx.byLegacy[code.number]
		if len(successors) == 0 {
			ref.LegacyCodes = append(ref.LegacyCodes, code.legacyCode())
			continue
		}
		for _, rda := range successors {
			resolved[rda.id] = struct{}{}
			ref.RDAs = append(ref.RDAs, idx.referencedRDA(invoice, rda, "successor", code.legacyCode()))
		}
	}

	switch {
	case len(note.arakCodes) == 0 && len(resolved) == 0:
		ref.Outcome = referenceLegacyOnly
	case hasUnresolved:
		ref.Outcome = referenceUnresolved
	case hasAmbiguous:
		ref.Outcome = referenceAmbiguous
	case len(resolved) == 1:
		ref.Outcome = referenceOne
	default:
		ref.Outcome = referenceMultiple
	}
	sort.SliceStable(ref.RDAs, func(i, j int) bool { return ref.RDAs[i].Code < ref.RDAs[j].Code })
	return ref
}

func (idx funnelReferenceIndex) referencedRDA(invoice funnelInvoice, rda funnelRDA, resolution, legacyCode string) MatchingFunnelReferencedRDA {
	id := rda.id
	match := invoice.supplierID != nil && rda.supplierID != nil && *invoice.supplierID == *rda.supplierID
	return MatchingFunnelReferencedRDA{
		Code:          strings.ToUpper(rda.code),
		LegacyCode:    legacyCode,
		ID:            &id,
		State:         rda.state,
		SupplierERPID: rda.supplierID,
		Resolution:    resolution,
		SupplierMatch: &match,
		InCandidates:  match,
		Total:         rda.total,
	}
}

func addReferenceOutcome(summary *MatchingFunnelReferenceSummary, ref MatchingFunnelReference) {
	switch ref.Outcome {
	case referenceNoNote:
		summary.NoNote++
	case referenceNoCode:
		summary.NoCode++
	case referenceNoRDADeclared:
		summary.NoRDADeclared++
	case referenceLegacyOnly:
		summary.LegacyOnly++
	case referenceUnresolved:
		summary.Unresolved++
	case referenceAmbiguous:
		summary.Ambiguous++
	case referenceOne:
		summary.OneRDA++
	case referenceMultiple:
		summary.MultipleRDAs++
	}
	if len(ref.LegacyCodes) > 0 && ref.Outcome != referenceLegacyOnly {
		summary.ArakAndLegacy++
	}
	mismatch := false
	promoted := false
	for _, rda := range ref.RDAs {
		if rda.SupplierMatch != nil && !*rda.SupplierMatch {
			mismatch = true
		}
		if rda.Resolution == "successor" {
			promoted = true
		}
	}
	if promoted {
		summary.LegacyPromoted++
	}
	if mismatch {
		summary.SupplierMismatch++
	}
}
