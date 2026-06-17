package binocolo

import (
	"strconv"
	"strings"
)

// Scoring v2 flags — non-scoring annotations for manual triage. Neutral flags
// describe the deal profile; warning flags surface data/risk caveats and are the
// human-readable reasons behind a low confidence score. All derived from the
// vendor payload (no cross-database lookups; the CDLAN synergy match is phase 2).

const (
	maFlagNeutral = "neutral"
	maFlagWarning = "warning"

	maBilancioStaleYears      = 2
	maPatrimonioErosionFactor = 0.3
)

func computeMAFlags(c maSignalContext) []MATargetFlag {
	flags := make([]MATargetFlag, 0, 4)
	add := func(code, label, severity string) {
		flags = append(flags, MATargetFlag{Code: code, Label: label, Severity: severity})
	}

	legalForm := strings.ToUpper(firstVendorString(c.object, "detailedLegalForm.code"))
	holders := c.holders

	// --- Deal profile (neutral) ---
	maxPercent := 0.0
	personCount := 0
	companyHolder := false
	surnames := map[string]int{}
	for _, holder := range holders {
		if holder.PercentShare > maxPercent {
			maxPercent = holder.PercentShare
		}
		if isPersonShareholder(holder) {
			personCount++
			if surname := strings.ToUpper(strings.TrimSpace(holder.Surname)); surname != "" {
				surnames[surname]++
			}
		} else if strings.TrimSpace(holder.CompanyName) != "" {
			companyHolder = true
		}
	}

	if legalForm == "AU" || legalForm == "SU" || (len(holders) == 1 && personCount == 1) || maxPercent >= 90 {
		add("socio_unico", "Socio unico", maFlagNeutral)
	}
	familyBySurname := false
	for _, count := range surnames {
		if count >= 2 {
			familyBySurname = true
			break
		}
	}
	if legalForm == "IF" || familyBySurname {
		add("impresa_familiare", "Impresa familiare", maFlagNeutral)
	}
	if companyHolder {
		add("controllo_holding", "Controllo holding", maFlagNeutral)
	}
	if age, ok := dominantOwnerAge(c); ok && age >= successionMinAge(c.strategy) {
		add("ricambio_generazionale", "Ricambio generazionale", maFlagNeutral)
	}

	// --- Caveats (warning) ---
	if c.fin.Turnover == nil {
		add("bilancio_assente", "Bilancio assente", maFlagWarning)
	} else if c.fin.LastYear > 0 && c.fin.LastYear <= c.now.Year()-maBilancioStaleYears {
		add("bilancio_datato", "Ultimo bilancio "+strconv.Itoa(c.fin.LastYear), maFlagWarning)
	}
	if c.fin.NetWorth != nil && *c.fin.NetWorth < 0 {
		add("patrimonio_netto_negativo", "Patrimonio netto negativo", maFlagWarning)
	} else if c.fin.NetWorth != nil && c.fin.PrevNetWorth != nil &&
		*c.fin.PrevNetWorth > 0 && float64(*c.fin.NetWorth) < maPatrimonioErosionFactor*float64(*c.fin.PrevNetWorth) {
		add("patrimonio_netto_eroso", "Patrimonio netto eroso", maFlagWarning)
	}
	if vendorBool(c.object, "taxCodeCeased") {
		add("cessata_fiscalmente", "Cessata fiscalmente", maFlagWarning)
	} else if status := strings.ToUpper(strings.TrimSpace(c.target.ActivityStatus)); status != "" && status != "ATTIVA" {
		add("non_attiva", "Stato: "+strings.ToLower(status), maFlagWarning)
	}

	return flags
}

func isPersonShareholder(holder maShareholder) bool {
	if strings.TrimSpace(holder.Name) != "" || strings.TrimSpace(holder.Surname) != "" {
		return true
	}
	return len(strings.TrimSpace(holder.TaxCode)) == 16
}

// dominantOwnerAge decodes the age of the majority (>=50%) natural-person owner.
func dominantOwnerAge(c maSignalContext) (int, bool) {
	for _, holder := range c.holders {
		if holder.PercentShare < 50 {
			continue
		}
		if !isPersonShareholder(holder) {
			continue
		}
		if age, ok := ageFromItalianTaxCode(holder.TaxCode, c.now); ok {
			return age, true
		}
	}
	return 0, false
}

func vendorBool(object map[string]any, path string) bool {
	value, ok := vendorPath(object, path)
	if !ok {
		return false
	}
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}
