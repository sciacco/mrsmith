package rda

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"
)

type providerEmailTemplateData struct {
	Language, CustomerName, OrderNumber, OrderDate, RequesterFirstName, RequesterLastName string
}
type providerEmailRendered struct{ Text, HTML string }

func providerEmailInitialTemplates(data providerEmailTemplateData) (ProviderEmailTemplates, error) {
	if strings.TrimSpace(data.OrderNumber) == "" {
		return ProviderEmailTemplates{}, fmt.Errorf("order number is required")
	}
	itIntroduction, itConclusion := initialProviderMessage(data, "it")
	enIntroduction, enConclusion := initialProviderMessage(data, "en")
	it := ProviderEmailInitialTemplate{Subject: "Ordine " + data.OrderNumber + subjectDate(data.OrderDate, "it"), Introduction: itIntroduction, Conclusion: itConclusion}
	en := ProviderEmailInitialTemplate{Subject: "Purchase Order " + data.OrderNumber + subjectDate(data.OrderDate, "en"), Introduction: enIntroduction, Conclusion: enConclusion}
	return ProviderEmailTemplates{IT: it, EN: en}, nil
}

func subjectDate(value, language string) string {
	if value = formatProviderEmailDate(value, language); value != "" {
		if language == "it" {
			return " del " + value
		}
		return " dated " + value
	}
	return ""
}
func initialProviderMessage(d providerEmailTemplateData, lang string) (string, string) {
	name := strings.TrimSpace(d.CustomerName)
	if lang == "it" {
		introduction := "inviamo in allegato l’ordine di acquisto emesso dal referente indicato di seguito."
		if name != "" {
			introduction = "Spett.le " + name + ",\n\n" + introduction
		}
		return introduction, "Vi invitiamo a indicare il numero del PO nelle fatture per facilitare il processo di pagamento.\nIl Codice Univoco di CDLAN è M5UXCR1."
	}
	introduction := "Please find attached the purchase order issued by the contact person shown below."
	if name != "" {
		introduction = "Dear " + name + ",\n\n" + introduction
	}
	return introduction, "Please include the PO number on all invoices to help facilitate the payment process."
}

func providerEmailParagraphs(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	paragraphs := make([]string, 0)
	lines := make([]string, 0)
	flush := func() {
		if len(lines) == 0 {
			return
		}
		paragraphs = append(paragraphs, strings.Join(lines, "\n"))
		lines = lines[:0]
	}
	for _, line := range strings.Split(value, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		lines = append(lines, line)
	}
	flush()
	return paragraphs
}

func safeProviderEmailParagraphs(paragraphs []string, emphasizeUniqueCode bool, firstMargin string) template.HTML {
	var out strings.Builder
	for i, paragraph := range paragraphs {
		escaped := template.HTMLEscapeString(paragraph)
		escaped = strings.ReplaceAll(escaped, "\n", "<br>")
		if emphasizeUniqueCode {
			escaped = strings.ReplaceAll(escaped, "M5UXCR1", "<strong>M5UXCR1</strong>")
		}
		margin := "8px"
		if i == 0 {
			margin = firstMargin
		}
		out.WriteString(`<p style="margin:` + margin + ` 0 0;font-size:15px;line-height:1.6;color:#444444;">` + escaped + `</p>`)
	}
	return template.HTML(out.String()) // escaped above; only fixed markup is introduced
}

func renderProviderEmail(data providerEmailTemplateData, introduction, conclusion string) (providerEmailRendered, error) {
	lang := normalizeProviderEmailLanguage(data.Language)
	number := strings.TrimSpace(data.OrderNumber)
	if number == "" {
		return providerEmailRendered{}, fmt.Errorf("order number is required")
	}
	date := formatProviderEmailDate(data.OrderDate, lang)
	requester := strings.TrimSpace(strings.Join([]string{data.RequesterFirstName, data.RequesterLastName}, " "))
	labels := map[string]string{"dept": "Purchasing Department", "eyebrow": "Purchase order", "title": "Purchase Order ", "number": "Order number", "date": "Order date", "requester": "Contact person", "regards": "Best regards,", "footer": "Message sent through the MrSmith corporate portal."}
	if lang == "it" {
		labels = map[string]string{"dept": "Ufficio Acquisti", "eyebrow": "Ordine di acquisto", "title": "Ordine ", "number": "Numero ordine", "date": "Data ordine", "requester": "Referente", "regards": "Cordiali saluti,", "footer": "Messaggio inviato tramite il portale aziendale MrSmith."}
	}
	before := providerEmailParagraphs(introduction)
	after := providerEmailParagraphs(conclusion)
	view := struct {
		Lang, Dept, Eyebrow, Title, NumberLabel, Number, DateLabel, Date, RequesterLabel, Requester, Regards, Footer string
		Before, After                                                                                                template.HTML
	}{lang, labels["dept"], labels["eyebrow"], labels["title"] + number, labels["number"], number, labels["date"], date, labels["requester"], requester, labels["regards"], labels["footer"], safeProviderEmailParagraphs(before, false, "12px"), safeProviderEmailParagraphs(after, lang == "it", "0")}
	var out bytes.Buffer
	if err := providerHTMLTemplate.Execute(&out, view); err != nil {
		return providerEmailRendered{}, err
	}
	text := strings.TrimSpace(introduction) + "\n\n" + labels["number"] + ": " + number
	if date != "" {
		text += "\n" + labels["date"] + ": " + date
	}
	if requester != "" {
		text += "\n" + labels["requester"] + ": " + requester
	}
	text += "\n\n" + strings.TrimSpace(conclusion)
	text += "\n\n" + labels["regards"] + "\nCDLAN S.p.A."
	return providerEmailRendered{Text: text, HTML: out.String()}, nil
}

func formatProviderEmailDate(value, lang string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02", "02/01/2006"} {
		if t, err := time.Parse(layout, value); err == nil {
			if lang == "it" {
				return t.Format("02/01/2006")
			}
			months := []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
			return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()-1], t.Year())
		}
	}
	return ""
}

var providerHTMLTemplate = template.Must(template.New("provider-email").Parse(`<!doctype html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}}</title></head>
<body style="margin:0;padding:0;background:#f4f6f8;color:#444444;"><table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="width:100%;background:#f4f6f8;"><tr><td align="center" style="padding:32px 12px;"><table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="width:100%;max-width:600px;margin:0 auto;background:#ffffff;border:1px solid #E0E0E0;border-radius:10px;overflow:hidden;font-family:'Work Sans',-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
<tr><td style="height:3px;line-height:3px;font-size:0;background:#E1251B;">&nbsp;</td></tr>
<tr><td style="padding:15px 24px;background:#1A3E6E;color:#ffffff;font-size:14px;line-height:1.4;font-weight:700;">CDLAN <span style="color:#AEBFD6;font-size:13px;font-weight:400;">· {{.Dept}}</span></td></tr>
<tr><td style="padding:28px 24px 0;"><table role="presentation" cellpadding="0" cellspacing="0"><tr><td valign="middle" style="padding-right:7px;"><span style="display:block;width:9px;height:9px;border-radius:2px;background:#00AA46;">&nbsp;</span></td><td valign="middle" style="color:#15803d;font-size:11px;line-height:1.2;font-weight:700;letter-spacing:0.14em;text-transform:uppercase;">{{.Eyebrow}}</td></tr></table><h1 style="margin:9px 0 0;font-size:21px;line-height:1.25;color:#1A3E6E;font-weight:700;letter-spacing:-0.01em;">{{.Title}}</h1>{{.Before}}</td></tr>
<tr><td style="padding:18px 24px 4px;"><table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="width:100%;border:1px solid #E0E0E0;border-radius:8px;overflow:hidden;"><tr><td style="padding:10px 14px;border-bottom:1px solid #EEF1F4;color:#767676;font-size:13px;line-height:1.5;">{{.NumberLabel}}</td><td align="right" style="padding:10px 14px;border-bottom:1px solid #EEF1F4;color:#1A3E6E;font-size:13px;line-height:1.5;font-weight:700;">{{.Number}}</td></tr>{{if .Date}}<tr><td style="padding:10px 14px;border-bottom:1px solid #EEF1F4;color:#767676;font-size:13px;line-height:1.5;">{{.DateLabel}}</td><td align="right" style="padding:10px 14px;border-bottom:1px solid #EEF1F4;color:#444444;font-size:13px;line-height:1.5;">{{.Date}}</td></tr>{{end}}{{if .Requester}}<tr><td style="padding:10px 14px;color:#767676;font-size:13px;line-height:1.5;">{{.RequesterLabel}}</td><td align="right" style="padding:10px 14px;color:#444444;font-size:13px;line-height:1.5;">{{.Requester}}</td></tr>{{end}}</table></td></tr>
<tr><td style="padding:18px 24px 28px;">{{.After}}<p style="margin:18px 0 0;font-size:15px;line-height:1.6;color:#444444;">{{.Regards}}<br><strong>CDLAN S.p.A.</strong></p></td></tr>
<tr><td style="padding:20px 24px 22px;border-top:1px solid #EEF1F4;"><p style="margin:0;font-size:11px;line-height:1.5;color:#8A8A8A;">{{.Footer}}</p></td></tr>
</table></td></tr></table></body></html>`))
