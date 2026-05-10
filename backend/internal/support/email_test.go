package support

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/email"
)

func TestSupportRequestEmailIncludesTriageContextAndAttachment(t *testing.T) {
	msg := supportRequestEmail(CreateRequestInput{
		Priority:                 "urgent",
		AppID:                    "quotes",
		AppName:                  "Proposte",
		PageURL:                  "https://mrsmith.test/apps/quotes?draft=1",
		PagePath:                 "/apps/quotes",
		Message:                  "La pagina non carica i dati",
		Requester:                auth.Claims{Subject: "sub-1", Name: "Mario Rossi", Email: "mario.rossi@example.com"},
		TechnicalContextIncluded: true,
		Context: map[string]any{
			"app":        map[string]any{"id": "quotes", "name": "Proposte"},
			"page":       map[string]any{"url": "https://mrsmith.test/apps/quotes?draft=1", "path": "/apps/quotes", "title": "Proposte"},
			"capturedAt": "2026-05-10T15:00:00Z",
			"browser": map[string]any{
				"userAgent": "Mozilla/5.0 Chrome/124",
				"language":  "it-IT",
				"timezone":  "Europe/Rome",
				"online":    true,
				"viewport":  map[string]any{"width": 1440, "height": 900, "devicePixelRatio": 2},
			},
			"api": map[string]any{
				"recentRequests": []any{
					map[string]any{"timestamp": "2026-05-10T15:00:00Z", "method": "GET", "path": "/quotes/v1/quotes", "status": 500, "ok": false, "durationMs": 73, "requestId": "req-123"},
					map[string]any{"timestamp": "2026-05-10T15:00:01Z", "method": "GET", "path": "/quotes/v1/templates", "status": 200, "ok": true, "durationMs": 20, "requestId": "req-124"},
				},
			},
		},
	}, 42, []string{"support@example.com"})

	if msg.Subject != "[MrSmith][URGENT] Mario Rossi needs help - Proposte #42" {
		t.Fatalf("unexpected subject %q", msg.Subject)
	}
	if len(msg.ReplyTo) != 1 || msg.ReplyTo[0] != "mario.rossi@example.com" {
		t.Fatalf("unexpected Reply-To %#v", msg.ReplyTo)
	}
	if msg.HTML == "" || msg.Text == "" {
		t.Fatal("expected html and text email bodies")
	}
	for _, body := range []string{msg.Text, msg.HTML} {
		for _, want := range []string{
			"Proposte",
			"Mario Rossi needs help",
			"/apps/quotes",
			"URGENT",
			"Mario Rossi",
			"La pagina non carica i dati",
			"/quotes/v1/quotes",
			"req-123",
			"Mozilla/5.0 Chrome/124",
			"1440x900 @ 2x",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("email body missing %q:\n%s", want, body)
			}
		}
	}
	if strings.Contains(msg.Subject, "Proposte support request") || strings.Contains(msg.HTML, "Proposte needs help") {
		t.Fatalf("app name used as requester in email:\nsubject=%s\nhtml=%s", msg.Subject, msg.HTML)
	}

	context := readSupportContextAttachment(t, msg.Attachments, "support-request-42-context.json")
	page := context["page"].(map[string]any)
	if page["path"] != "/apps/quotes" {
		t.Fatalf("unexpected attachment page path: %#v", page["path"])
	}
}

func TestSupportRequestEmailSanitizesContextBeforeEmailing(t *testing.T) {
	msg := supportRequestEmail(CreateRequestInput{
		Priority:  "normal",
		AppID:     "rda",
		AppName:   "RDA",
		PagePath:  "/rda",
		Message:   "Serve aiuto",
		Requester: auth.Claims{Subject: "sub-1", Email: "requester@example.com"},
		Context: map[string]any{
			"app":           map[string]any{"id": "rda", "name": "RDA"},
			"authorization": "Bearer very-secret",
			"headers":       map[string]any{"cookie": "sid=secret", "x-request-id": "req-safe"},
			"nested":        map[string]any{"refreshToken": "refresh-secret", "safe": "kept"},
			"payload":       map[string]any{"card": "hidden"},
		},
	}, 7, []string{"support@example.com"})

	attachment := readAttachmentText(t, msg.Attachments, "support-request-7-context.json")
	combined := msg.Text + "\n" + msg.HTML + "\n" + attachment
	for _, forbidden := range []string{"Bearer very-secret", "sid=secret", "refresh-secret", `"payload"`, `"authorization"`, `"cookie"`} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("sensitive value %q leaked in email:\n%s", forbidden, combined)
		}
	}
	if !strings.Contains(attachment, "req-safe") || !strings.Contains(attachment, "kept") {
		t.Fatalf("safe context was not preserved in attachment:\n%s", attachment)
	}
}

func TestSupportRequestEmailSkipsInvalidReplyTo(t *testing.T) {
	msg := supportRequestEmail(CreateRequestInput{
		Priority:  "normal",
		AppID:     "rda",
		AppName:   "RDA",
		Message:   "Serve aiuto",
		Requester: auth.Claims{Email: "bad\naddress@example.com"},
		Context:   map[string]any{"app": map[string]any{"id": "rda"}},
	}, 8, []string{"support@example.com"})

	if len(msg.ReplyTo) != 0 {
		t.Fatalf("expected no Reply-To, got %#v", msg.ReplyTo)
	}
}

func readSupportContextAttachment(t *testing.T, attachments []email.Attachment, expectedFilename string) map[string]any {
	t.Helper()
	raw := readAttachmentText(t, attachments, expectedFilename)
	var context map[string]any
	if err := json.Unmarshal([]byte(raw), &context); err != nil {
		t.Fatalf("unmarshal context attachment: %v\n%s", err, raw)
	}
	return context
}

func readAttachmentText(t *testing.T, attachments []email.Attachment, expectedFilename string) string {
	t.Helper()
	if len(attachments) != 1 {
		t.Fatalf("expected one attachment, got %d", len(attachments))
	}
	if attachments[0].Filename != expectedFilename {
		t.Fatalf("unexpected attachment filename %q", attachments[0].Filename)
	}
	if attachments[0].ContentType != "application/json" {
		t.Fatalf("unexpected attachment content type %q", attachments[0].ContentType)
	}
	raw, err := io.ReadAll(attachments[0].Content)
	if err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	return string(raw)
}
