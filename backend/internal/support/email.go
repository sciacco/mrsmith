package support

import (
	"context"
	"fmt"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/email"
)

func supportRequestEmail(input CreateRequestInput, id int64, to []string) email.Message {
	subject := fmt.Sprintf("[MrSmith] Support request #%d - %s", id, input.AppName)
	if strings.TrimSpace(input.AppName) == "" {
		subject = fmt.Sprintf("[MrSmith] Support request #%d", id)
	}

	text := strings.Join([]string{
		fmt.Sprintf("Support request #%d", id),
		"",
		"App: " + input.AppName,
		"Path: " + input.PagePath,
		"URL: " + input.PageURL,
		"Priority: " + input.Priority,
		"",
		"Requester:",
		"Name: " + input.Requester.Name,
		"Email: " + input.Requester.Email,
		"Subject: " + input.Requester.Subject,
		"",
		"Message:",
		input.Message,
	}, "\n")

	return email.Message{
		To:      to,
		Subject: subject,
		Text:    text,
	}
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
