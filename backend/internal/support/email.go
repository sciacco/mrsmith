package support

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/mail"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/email"
)

const maxEmailAPIFailures = 5

type supportEmailSummary struct {
	AppID           string
	AppName         string
	PageURL         string
	PagePath        string
	PageTitle       string
	BrowserLanguage string
	BrowserTimezone string
	BrowserOnline   string
	BrowserViewport string
	UserAgent       string
	CapturedAt      string
	APIHistoryCount int
	APIFailures     []supportEmailAPIFailure
}

type supportEmailAPIFailure struct {
	Method     string
	Path       string
	Status     string
	Duration   string
	RequestID  string
	Error      string
	CapturedAt string
}

func supportRequestEmail(input CreateRequestInput, id int64, to []string) email.Message {
	contextValue := sanitizeEmailContext(input.Context)
	summary := summarizeSupportContext(input, contextValue)
	contextJSON := supportContextAttachment(contextValue)

	attachments := []email.Attachment{{
		Filename:    fmt.Sprintf("support-request-%d-context.json", id),
		ContentType: "application/json",
		Content:     strings.NewReader(contextJSON),
	}}
	for _, attachment := range input.Attachments {
		attachments = append(attachments, email.Attachment{
			Filename:    attachment.Filename,
			ContentType: attachment.ContentType,
			Content:     bytes.NewReader(attachment.Content),
		})
	}

	msg := email.Message{
		To:          to,
		Subject:     supportEmailSubject(input, id, summary),
		Text:        supportEmailText(input, id, summary),
		HTML:        supportEmailHTML(input, id, summary),
		Attachments: attachments,
	}
	if replyTo := validReplyTo(input.Requester.Email); replyTo != "" {
		msg.ReplyTo = []string{replyTo}
	}
	return msg
}

func supportEmailSubject(input CreateRequestInput, id int64, summary supportEmailSummary) string {
	priority := emailPriority(input)
	requester := requesterDisplayName(input)
	appName := compactLine(summary.AppName)
	if appName == "" {
		return fmt.Sprintf("[MrSmith][%s] %s needs help #%d", priority, requester, id)
	}
	return fmt.Sprintf("[MrSmith][%s] %s needs help - %s #%d", priority, requester, appName, id)
}

func supportEmailText(input CreateRequestInput, id int64, summary supportEmailSummary) string {
	lines := []string{
		fmt.Sprintf("Support request #%d", id),
		requesterDisplayName(input) + " needs help",
		"Priority: " + emailPriority(input),
		"App: " + summary.AppName,
		"Path: " + summary.PagePath,
		"URL: " + summary.PageURL,
		"Captured at: " + summary.CapturedAt,
		technicalContextLine(input),
		"",
		"Requester:",
		"Name: " + input.Requester.Name,
		"Email: " + input.Requester.Email,
		"Subject: " + input.Requester.Subject,
		"",
		"Message:",
		input.Message,
		"",
		"Browser:",
		"Language: " + summary.BrowserLanguage,
		"Timezone: " + summary.BrowserTimezone,
		"Viewport: " + summary.BrowserViewport,
		"Online: " + summary.BrowserOnline,
		"User agent: " + summary.UserAgent,
		"",
		fmt.Sprintf("Recent API requests: %d", summary.APIHistoryCount),
		"Recent API failures:",
	}
	if len(summary.APIFailures) == 0 {
		lines = append(lines, "None reported")
	} else {
		for _, failure := range summary.APIFailures {
			lines = append(lines, "- "+formatAPIFailure(failure))
		}
	}
	lines = append(lines, "", "Attachments:")
	if len(input.Attachments) == 0 {
		lines = append(lines, "None provided")
	} else {
		for _, attachment := range input.Attachments {
			lines = append(lines, "- "+formatSupportAttachment(attachment))
		}
	}
	lines = append(lines, "", fmt.Sprintf("Full sanitized context is attached as support-request-%d-context.json.", id))
	return strings.Join(lines, "\n")
}

func supportEmailHTML(input CreateRequestInput, id int64, summary supportEmailSummary) string {
	failureRows := `<tr><td colspan="6" style="padding:10px 12px;color:#64748b;">None reported</td></tr>`
	if len(summary.APIFailures) > 0 {
		var rows strings.Builder
		for _, failure := range summary.APIFailures {
			rows.WriteString("<tr>")
			rows.WriteString(tableCell(failure.Method))
			rows.WriteString(tableCell(failure.Path))
			rows.WriteString(tableCell(failure.Status))
			rows.WriteString(tableCell(failure.Duration))
			rows.WriteString(tableCell(failure.RequestID))
			rows.WriteString(tableCell(failure.Error))
			rows.WriteString("</tr>")
		}
		failureRows = rows.String()
	}

	attachmentRows := `<tr><td colspan="3" style="padding:10px 12px;color:#64748b;">None provided</td></tr>`
	if len(input.Attachments) > 0 {
		var rows strings.Builder
		for _, attachment := range input.Attachments {
			rows.WriteString("<tr>")
			rows.WriteString(tableCell(attachment.Filename))
			rows.WriteString(tableCell(attachment.ContentType))
			rows.WriteString(tableCell(formatBytes(attachment.SizeBytes)))
			rows.WriteString("</tr>")
		}
		attachmentRows = rows.String()
	}

	return fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f8fafc;font-family:Arial,sans-serif;color:#0f172a;">
  <div style="max-width:760px;margin:0 auto;padding:24px;">
    <div style="background:#ffffff;border:1px solid #e2e8f0;border-radius:12px;padding:24px;">
      <p style="margin:0 0 8px;color:#64748b;font-size:13px;">MrSmith support request #%d</p>
      <h1 style="margin:0 0 20px;font-size:22px;line-height:1.3;color:#0f172a;">%s</h1>
      <table style="width:100%%;border-collapse:collapse;margin-bottom:20px;font-size:14px;">
        %s
        %s
        %s
        %s
        %s
      </table>
      <h2 style="margin:24px 0 8px;font-size:15px;color:#0f172a;">Message</h2>
      <div style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:8px;padding:14px;font-size:14px;line-height:1.5;">%s</div>
      <h2 style="margin:24px 0 8px;font-size:15px;color:#0f172a;">Requester</h2>
      <table style="width:100%%;border-collapse:collapse;font-size:14px;">
        %s
        %s
        %s
      </table>
      <h2 style="margin:24px 0 8px;font-size:15px;color:#0f172a;">Browser</h2>
      <table style="width:100%%;border-collapse:collapse;font-size:14px;">
        %s
        %s
        %s
        %s
        %s
      </table>
      <h2 style="margin:24px 0 8px;font-size:15px;color:#0f172a;">Recent API failures</h2>
      <table style="width:100%%;border-collapse:collapse;font-size:13px;border:1px solid #e2e8f0;">
        <thead>
          <tr style="background:#f1f5f9;">
            <th style="text-align:left;padding:10px 12px;border-bottom:1px solid #e2e8f0;">Method</th>
            <th style="text-align:left;padding:10px 12px;border-bottom:1px solid #e2e8f0;">Path</th>
            <th style="text-align:left;padding:10px 12px;border-bottom:1px solid #e2e8f0;">Status</th>
            <th style="text-align:left;padding:10px 12px;border-bottom:1px solid #e2e8f0;">Duration</th>
            <th style="text-align:left;padding:10px 12px;border-bottom:1px solid #e2e8f0;">Request ID</th>
            <th style="text-align:left;padding:10px 12px;border-bottom:1px solid #e2e8f0;">Error</th>
          </tr>
        </thead>
        <tbody>%s</tbody>
      </table>
      <h2 style="margin:24px 0 8px;font-size:15px;color:#0f172a;">Attachments</h2>
      <table style="width:100%%;border-collapse:collapse;font-size:13px;border:1px solid #e2e8f0;">
        <thead>
          <tr style="background:#f1f5f9;">
            <th style="text-align:left;padding:10px 12px;border-bottom:1px solid #e2e8f0;">Filename</th>
            <th style="text-align:left;padding:10px 12px;border-bottom:1px solid #e2e8f0;">Content type</th>
            <th style="text-align:left;padding:10px 12px;border-bottom:1px solid #e2e8f0;">Size</th>
          </tr>
        </thead>
        <tbody>%s</tbody>
      </table>
      <p style="margin:20px 0 0;color:#64748b;font-size:13px;">Full sanitized context is attached as <strong>support-request-%d-context.json</strong>.</p>
    </div>
  </div>
</body>
</html>`,
		id,
		escapeHTML(emailTitle(input)),
		infoRow("Priority", emailPriority(input)),
		infoRow("App", summary.AppName),
		infoRow("Path", summary.PagePath),
		infoRow("URL", summary.PageURL),
		infoRow("Captured at", summary.CapturedAt),
		multilineHTML(input.Message),
		infoRow("Name", input.Requester.Name),
		infoRow("Email", input.Requester.Email),
		infoRow("Subject", input.Requester.Subject),
		infoRow("Language", summary.BrowserLanguage),
		infoRow("Timezone", summary.BrowserTimezone),
		infoRow("Viewport", summary.BrowserViewport),
		infoRow("Online", summary.BrowserOnline),
		infoRow("User agent", summary.UserAgent),
		failureRows,
		attachmentRows,
		id,
	)
}

func sanitizeEmailContext(value any) any {
	if value == nil {
		return map[string]any{}
	}
	return sanitizeContext(value, 0)
}

func supportContextAttachment(value any) string {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		raw, _ = json.MarshalIndent(map[string]string{"error": "context_unavailable"}, "", "  ")
	}
	return string(raw) + "\n"
}

func summarizeSupportContext(input CreateRequestInput, contextValue any) supportEmailSummary {
	root := mapFromAny(contextValue)
	app := mapFromAny(root["app"])
	page := mapFromAny(root["page"])
	browser := mapFromAny(root["browser"])
	viewport := mapFromAny(browser["viewport"])

	summary := supportEmailSummary{
		AppID:           firstNonEmpty(input.AppID, stringFromAny(app["id"])),
		AppName:         firstNonEmpty(input.AppName, stringFromAny(app["name"])),
		PageURL:         firstNonEmpty(input.PageURL, stringFromAny(page["url"])),
		PagePath:        firstNonEmpty(input.PagePath, stringFromAny(page["path"])),
		PageTitle:       stringFromAny(page["title"]),
		BrowserLanguage: firstNonEmpty(stringFromAny(browser["language"]), strings.Join(stringsFromAny(browser["languages"]), ", ")),
		BrowserTimezone: stringFromAny(browser["timezone"]),
		BrowserOnline:   boolLabel(browser["online"]),
		BrowserViewport: viewportLabel(viewport),
		UserAgent:       stringFromAny(browser["userAgent"]),
		CapturedAt:      stringFromAny(root["capturedAt"]),
		APIHistoryCount: 0,
		APIFailures:     nil,
	}
	summary.APIHistoryCount, summary.APIFailures = summarizeAPIFailures(root)
	return summary
}

func summarizeAPIFailures(root map[string]any) (int, []supportEmailAPIFailure) {
	api := mapFromAny(root["api"])
	requests := sliceFromAny(api["recentRequests"])
	failures := make([]supportEmailAPIFailure, 0, maxEmailAPIFailures)
	for _, request := range requests {
		entry := mapFromAny(request)
		if len(entry) == 0 {
			continue
		}
		if !apiEntryFailed(entry) {
			continue
		}
		failures = append(failures, supportEmailAPIFailure{
			Method:     firstNonEmpty(stringFromAny(entry["method"]), "GET"),
			Path:       stringFromAny(entry["path"]),
			Status:     stringFromAny(entry["status"]),
			Duration:   durationLabel(entry["durationMs"]),
			RequestID:  stringFromAny(entry["requestId"]),
			Error:      stringFromAny(entry["error"]),
			CapturedAt: stringFromAny(entry["timestamp"]),
		})
		if len(failures) >= maxEmailAPIFailures {
			break
		}
	}
	return len(requests), failures
}

func apiEntryFailed(entry map[string]any) bool {
	if err := stringFromAny(entry["error"]); err != "" {
		return true
	}
	if ok, exists := boolFromAny(entry["ok"]); exists {
		return !ok
	}
	status, exists := intFromAny(entry["status"])
	return exists && (status == 0 || status >= 400)
}

func emailTitle(input CreateRequestInput) string {
	return fmt.Sprintf("%s needs help", requesterDisplayName(input))
}

func requesterDisplayName(input CreateRequestInput) string {
	return firstNonEmpty(
		compactLine(input.Requester.Name),
		compactLine(input.Requester.Email),
		compactLine(input.Requester.Subject),
		"User",
	)
}

func technicalContextLine(input CreateRequestInput) string {
	if input.TechnicalContextIncluded {
		return "Technical context: included"
	}
	return "Technical context: limited by requester"
}

func emailPriority(input CreateRequestInput) string {
	priority := strings.ToUpper(compactLine(input.Priority))
	if priority == "" {
		return "NORMAL"
	}
	return priority
}

func formatAPIFailure(failure supportEmailAPIFailure) string {
	parts := []string{
		strings.TrimSpace(failure.Method),
		strings.TrimSpace(failure.Path),
	}
	if status := strings.TrimSpace(failure.Status); status != "" {
		parts = append(parts, "status "+status)
	}
	if duration := strings.TrimSpace(failure.Duration); duration != "" {
		parts = append(parts, duration)
	}
	if requestID := strings.TrimSpace(failure.RequestID); requestID != "" {
		parts = append(parts, "request "+requestID)
	}
	if err := strings.TrimSpace(failure.Error); err != "" {
		parts = append(parts, "error "+err)
	}
	return strings.Join(nonEmpty(parts), " | ")
}

func formatSupportAttachment(attachment CreateRequestAttachment) string {
	parts := []string{
		compactLine(attachment.Filename),
		compactLine(attachment.ContentType),
		formatBytes(attachment.SizeBytes),
	}
	return strings.Join(nonEmpty(parts), " | ")
}

func formatBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MiB", float64(size)/(1024*1024))
}

func validReplyTo(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	address, err := mail.ParseAddress(trimmed)
	if err != nil {
		return ""
	}
	return address.Address
}

func infoRow(label string, value string) string {
	return fmt.Sprintf(`<tr><td style="padding:6px 0;color:#64748b;width:130px;vertical-align:top;">%s</td><td style="padding:6px 0;color:#0f172a;vertical-align:top;">%s</td></tr>`, escapeHTML(label), escapeHTML(emptyDash(value)))
}

func tableCell(value string) string {
	return fmt.Sprintf(`<td style="padding:10px 12px;border-top:1px solid #e2e8f0;vertical-align:top;">%s</td>`, escapeHTML(emptyDash(value)))
}

func escapeHTML(value string) string {
	return html.EscapeString(value)
}

func multilineHTML(value string) string {
	escaped := escapeHTML(strings.ReplaceAll(value, "\r\n", "\n"))
	escaped = strings.ReplaceAll(escaped, "\r", "\n")
	return strings.ReplaceAll(escaped, "\n", "<br>")
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func compactLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}

func mapFromAny(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return nil
}

func sliceFromAny(value any) []any {
	if typed, ok := value.([]any); ok {
		return typed
	}
	return nil
}

func stringsFromAny(value any) []string {
	items := sliceFromAny(value)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text := stringFromAny(item); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return fmt.Sprintf("%.0f", typed)
	case float32:
		return fmt.Sprintf("%.0f", typed)
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed), true
		}
	case float64:
		return int(typed), true
	case float32:
		return int(typed), true
	case int:
		return typed, true
	case int64:
		return int(typed), true
	}
	return 0, false
}

func boolFromAny(value any) (bool, bool) {
	if typed, ok := value.(bool); ok {
		return typed, true
	}
	return false, false
}

func boolLabel(value any) string {
	if typed, exists := boolFromAny(value); exists {
		if typed {
			return "yes"
		}
		return "no"
	}
	return ""
}

func viewportLabel(viewport map[string]any) string {
	width := stringFromAny(viewport["width"])
	height := stringFromAny(viewport["height"])
	if width == "" || height == "" {
		return ""
	}
	ratio := stringFromAny(viewport["devicePixelRatio"])
	if ratio == "" {
		return width + "x" + height
	}
	return width + "x" + height + " @ " + ratio + "x"
}

func durationLabel(value any) string {
	duration := stringFromAny(value)
	if duration == "" {
		return ""
	}
	return duration + " ms"
}

func sendSupportNotification(ctx context.Context, mailer Mailer, input CreateRequestInput, id int64, recipients []string) (string, error) {
	if len(recipients) == 0 {
		return emailNotificationSkipped, nil
	}
	if mailer == nil || !mailer.Enabled() {
		return emailNotificationSkipped, nil
	}
	if err := mailer.Send(ctx, supportRequestEmail(input, id, recipients)); err != nil {
		return emailNotificationFailed, err
	}
	return emailNotificationSent, nil
}
