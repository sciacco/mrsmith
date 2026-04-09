package quotes

import (
	"fmt"
	"strings"
)

var billingTranslations = map[string]string{
	"All'ordine":                               "After order confirmation",
	"All'attivazione della Soluzione/Consegna": "Payment on delivery",
	"Mensile":              "Monthly",
	"Bimestrale":           "Bimonthly",
	"Trimestrale":          "Quarterly",
	"Quadrimestrale":       "Every 4 months",
	"Semestrale":           "Every 6 months",
	"Annuale":              "Annually",
	"Biennale":             "Every 2 years",
	"Senza tacito rinnovo": "No automatic renewal",
}

func nrcChargeTimeLabel(code int) string {
	switch code {
	case 1:
		return "All'ordine"
	case 2:
		return "All'attivazione della Soluzione/Consegna"
	default:
		return "All'attivazione della Soluzione/Consegna"
	}
}

func billingPeriodLabel(months int) string {
	switch months {
	case 1:
		return "Mensile"
	case 2:
		return "Bimestrale"
	case 3:
		return "Trimestrale"
	case 4:
		return "Quadrimestrale"
	case 6:
		return "Semestrale"
	case 12:
		return "Annuale"
	case 24:
		return "Biennale"
	default:
		return fmt.Sprintf("Ogni %d mesi", months)
	}
}

func GenerateTermsAndConditions(
	templateType string, isColo bool, lang string,
	paymentMethodLabel string,
	initialTermMonths, nextTermMonths, deliveredInDays, nrcChargeTime, billMonths int,
	legalNotes string,
) string {
	nrcLabel := nrcChargeTimeLabel(nrcChargeTime)
	billingLabel := billingPeriodLabel(billMonths)

	nrcLabelEN := billingTranslations[nrcLabel]
	if nrcLabelEN == "" {
		nrcLabelEN = nrcLabel
	}
	billingLabelEN := billingTranslations[billingLabel]
	if billingLabelEN == "" {
		billingLabelEN = billingLabel
	}

	commonIT := fmt.Sprintf(`<ul>
<li><b>Condizioni di pagamento</b>: %s</li>
<li><b>Durata Soluzione (mesi)</b>: %d</li>
<li><b>Durata Rinnovo (mesi)</b>: %d</li>
<li><b>Tempi di rilascio (giorni lavorativi)</b>: %d dalla ricezione di tutta la documentazione contrattuale firmata</li>
<li><b>Esclusioni</b>: IVA e quant&rsquo;altro non indicato</li>
<li><b>Valuta</b>: Euro (se non diversamente specificato)</li>
</ul>`, paymentMethodLabel, initialTermMonths, nextTermMonths, deliveredInDays)

	commonEN := fmt.Sprintf(`<ul>
<li><b>Payment Conditions</b>: %s</li>
<li><b>Period of Service (months)</b>: %d</li>
<li><b>Renewal Period (months)</b>: %d</li>
<li><b>Delivery Time (working days)</b>: %d upon receipt of all duly signed contractual documentation</li>
<li><b>Not included</b>: VAT and what is not specified</li>
<li><b>Currency</b>: Euro (unless otherwise specified)</li>
</ul>`, paymentMethodLabel, initialTermMonths, nextTermMonths, deliveredInDays)

	var tec string

	if isColo && lang == "it" {
		tec = fmt.Sprintf(`<ul>
<li><b>Modalit&agrave; di fatturazione</b>
<ul>
<li>Attivazione: %s</li>
<li>Colocation: Trimestrale anticipata</li>
<li>Corrente Utilizzata e Ampere Eccedenti: Mensile posticipata</li>
</ul>
</li>
</ul>
`, nrcLabel) + commonIT
	} else if isColo && lang == "en" {
		tec = fmt.Sprintf(`<ul>
<li><b>Billing Methods</b>
<ul>
<li>Setup: %s</li>
<li>Colocation: Three months in advance</li>
<li>Used Electric Current and Excess Amperes: Monthly deferred</li>
</ul>
</li>
</ul>
`, nrcLabelEN) + commonEN
	} else if !isColo && templateType != "iaas" && lang == "en" {
		tec = fmt.Sprintf(`<ul>
<li><b>Billing Methods</b>
<ul>
<li>Setup: %s</li>
<li>Fee: %s</li>
</ul>
</li>
</ul>
`, nrcLabelEN, billingLabelEN) + commonEN
	} else if templateType == "iaas" && lang == "it" {
		tec = fmt.Sprintf(`<ul>
<li><b>Modalit&agrave; di fatturazione</b>
<ul>
<li>Corrispettivi Una Tantum: %s</li>
<li>Canone: %s anticipata</li>
</ul>
</li>
</ul>
`, nrcLabel, billingLabel) + commonIT
	} else if templateType == "iaas" && lang == "en" {
		tec = fmt.Sprintf(`<ul>
<li><b>Billing Methods</b>
<ul>
<li>Setup: %s</li>
<li>Fee: %s</li>
</ul>
</li>
</ul>
`, nrcLabelEN, billingLabelEN) + commonEN
	} else {
		// Default: standard non-colo IT
		tec = fmt.Sprintf(`<ul>
<li><b>Modalit&agrave; di fatturazione</b>
<ul>
<li>Corrispettivi Una Tantum: %s</li>
<li>Canone: %s anticipata</li>
</ul>
</li>
</ul>
`, nrcLabel, billingLabel) + commonIT
	}

	if strings.TrimSpace(legalNotes) != "" {
		tec += "<p>" + legalNotes + "</p>"
	}

	return tec
}
