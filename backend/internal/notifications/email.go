package notifications

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/email"
)

func notificationEmail(delivery ClaimedDelivery, publicBaseURL string) (email.Message, error) {
	link, err := absoluteNotificationLink(delivery.DeepLink, publicBaseURL)
	if err != nil {
		return email.Message{}, err
	}
	subject := notificationEmailSubject(delivery)
	return email.Message{
		To:      []string{delivery.RecipientEmail},
		Subject: subject,
		Text:    notificationEmailText(delivery, link),
		HTML:    notificationEmailHTML(delivery, link),
	}, nil
}

func notificationEmailSubject(delivery ClaimedDelivery) string {
	app := strings.ToUpper(strings.TrimSpace(delivery.AppID))
	if app == "" {
		app = "MRSMITH"
	}
	return fmt.Sprintf("[MrSmith][%s] %s", app, compactLine(delivery.Title))
}

func notificationEmailText(delivery ClaimedDelivery, link string) string {
	lines := []string{
		compactLine(delivery.Title),
		"",
		strings.TrimSpace(delivery.Body),
		"",
		"Open in MrSmith:",
		link,
	}
	return strings.Join(lines, "\n")
}

func notificationEmailHTML(delivery ClaimedDelivery, link string) string {
	body := html.EscapeString(strings.TrimSpace(delivery.Body))
	if body == "" {
		body = "A notification is waiting in MrSmith."
	}
	return fmt.Sprintf(`<!doctype html>
<html>
<body style="margin:0;padding:0;background:#f8fafc;font-family:Arial,sans-serif;color:#0f172a;">
  <div style="max-width:680px;margin:0 auto;padding:24px;">
    <div style="background:#ffffff;border:1px solid #dbe5df;border-radius:10px;padding:24px;">
      <p style="margin:0 0 8px;color:#3f6b4b;font-size:13px;letter-spacing:.04em;text-transform:uppercase;">MrSmith %s</p>
      <h1 style="margin:0 0 16px;font-size:22px;line-height:1.3;color:#0f172a;">%s</h1>
      <p style="margin:0 0 22px;font-size:15px;line-height:1.5;color:#334155;">%s</p>
      <a href="%s" style="display:inline-block;background:#0f2a18;color:#dfffe7;text-decoration:none;padding:10px 14px;border-radius:6px;font-weight:700;">Open notification</a>
    </div>
  </div>
</body>
</html>`,
		html.EscapeString(strings.ToUpper(strings.TrimSpace(delivery.AppID))),
		html.EscapeString(compactLine(delivery.Title)),
		strings.ReplaceAll(body, "\n", "<br>"),
		html.EscapeString(link),
	)
}

func absoluteNotificationLink(deepLink string, publicBaseURL string) (string, error) {
	deepLink = strings.TrimSpace(deepLink)
	if deepLink == "" {
		deepLink = "/"
	}
	if parsed, err := url.Parse(deepLink); err == nil && parsed.IsAbs() {
		return deepLink, nil
	}
	publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if publicBaseURL == "" {
		return "", errMissingPublicBaseURL
	}
	base, err := url.Parse(publicBaseURL + "/")
	if err != nil || !base.IsAbs() {
		return "", errMissingPublicBaseURL
	}
	relative, err := url.Parse(strings.TrimLeft(deepLink, "/"))
	if err != nil {
		return "", err
	}
	return base.ResolveReference(relative).String(), nil
}

func compactLine(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "Notification"
	}
	return value
}
