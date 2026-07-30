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

type billingRow struct{ label, value string }

func billingRows(templateType string, isColo, en bool, nrc, bill string) []billingRow {
	switch {
	case isColo && en:
		return []billingRow{
			{"Setup", nrc},
			{"Colocation", "Three months in advance"},
			{"Used Electric Current and Excess Amperes", "Monthly deferred"},
		}
	case isColo:
		return []billingRow{
			{"Attivazione", nrc},
			{"Colocation", "Trimestrale anticipata"},
			{"Corrente Utilizzata e Ampere Eccedenti", "Mensile posticipata"},
		}
	case en:
		return []billingRow{{"Setup", nrc}, {"Fee", bill}}
	case templateType == "iaas":
		return []billingRow{{"Corrispettivi Una Tantum", nrc}, {"Canone", bill}}
	default:
		return []billingRow{{"Corrispettivi Una Tantum", nrc}, {"Canone", bill + " anticipata"}}
	}
}

func renderBillingBlock(title string, rows []billingRow) string {
	var b strings.Builder
	b.WriteString("<ul>\n<li><b>" + title + "</b>\n<ul>\n")
	for _, r := range rows {
		b.WriteString("<li>" + r.label + ": " + r.value + "</li>\n")
	}
	b.WriteString("</ul>\n</li>\n</ul>\n")
	return b.String()
}

func GenerateTermsAndConditions(
	templateType string, isColo bool, lang string,
	paymentMethodLabel string,
	initialTermMonths, nextTermMonths, deliveredInDays, nrcChargeTime, billMonths int,
	legalNotes string,
) string {
	en := lang == "en"

	nrcLabel := nrcChargeTimeLabel(nrcChargeTime)
	billingLabel := billingPeriodLabel(billMonths)
	if en {
		if t := billingTranslations[nrcLabel]; t != "" {
			nrcLabel = t
		}
		if t := billingTranslations[billingLabel]; t != "" {
			billingLabel = t
		}
	}

	title, common := "Modalit&agrave; di fatturazione", fmt.Sprintf(`<ul>
<li><b>Condizioni di pagamento</b>: %s</li>
<li><b>Durata Soluzione (mesi)</b>: %d</li>
<li><b>Durata Rinnovo (mesi)</b>: %d</li>
<li><b>Tempi di rilascio (giorni lavorativi)</b>: %d dalla ricezione di tutta la documentazione contrattuale firmata</li>
<li><b>Esclusioni</b>: IVA e quant&rsquo;altro non indicato</li>
<li><b>Valuta</b>: Euro (se non diversamente specificato)</li>
</ul>`, paymentMethodLabel, initialTermMonths, nextTermMonths, deliveredInDays)
	if en {
		title, common = "Billing Methods", fmt.Sprintf(`<ul>
<li><b>Payment Conditions</b>: %s</li>
<li><b>Period of Service (months)</b>: %d</li>
<li><b>Renewal Period (months)</b>: %d</li>
<li><b>Delivery Time (working days)</b>: %d upon receipt of all duly signed contractual documentation</li>
<li><b>Not included</b>: VAT and what is not specified</li>
<li><b>Currency</b>: Euro (unless otherwise specified)</li>
</ul>`, paymentMethodLabel, initialTermMonths, nextTermMonths, deliveredInDays)
	}

	tec := renderBillingBlock(title, billingRows(templateType, isColo, en, nrcLabel, billingLabel)) + common

	if strings.TrimSpace(legalNotes) != "" {
		tec += "<p>" + legalNotes + "</p>"
	}

	return tec
}
