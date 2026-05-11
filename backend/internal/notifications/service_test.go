package notifications

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestServiceNotifyNormalizesRecipientsAndBuildsIdempotentInput(t *testing.T) {
	store := &fakeServiceStore{
		notificationType: NotificationType{
			TypeKey:       "rda_approval_requested",
			AppID:         "rda",
			TitleTemplate: "Default title",
			BodyTemplate:  "Default body",
			Severity:      "warning",
			DefaultPolicy: json.RawMessage(`{
				"portal": {"enabled": true},
				"email": {
					"enabled": true,
					"steps": [{"step": "unread_after_4h", "delay": "4h"}]
				}
			}`),
			Enabled: true,
		},
	}
	service := NewService(store, nil)
	service.now = func() time.Time {
		return time.Date(2026, 5, 10, 8, 0, 0, 0, time.UTC)
	}

	result, err := service.Notify(context.Background(), NotifyInput{
		TypeKey:   " rda_approval_requested ",
		DedupeKey: " dedupe-1 ",
		DeepLink:  "/apps/rda/rda/po/42",
		Metadata:  map[string]any{"po_id": "42"},
		Recipients: []Recipient{
			{Email: "Approver@example.com", Name: "Approver"},
			{Email: "approver@example.com", Name: "Duplicate"},
			{Email: "not-an-email"},
		},
	})
	if err != nil {
		t.Fatalf("Notify failed: %v", err)
	}
	if result.NotificationID != 101 || result.RecipientCount != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	input := store.created
	if input.DedupeKey != "dedupe-1" || input.TypeKey != "rda_approval_requested" {
		t.Fatalf("unexpected create input identity: %#v", input)
	}
	if len(input.Recipients) != 1 || input.Recipients[0].Email != "approver@example.com" {
		t.Fatalf("recipients were not normalized/deduped: %#v", input.Recipients)
	}
	if len(input.Deliveries) != 2 {
		t.Fatalf("expected portal + email delivery specs, got %#v", input.Deliveries)
	}
	if input.Deliveries[0].PolicyStep != portalCreatedStep || input.Deliveries[1].PolicyStep != "unread_after_4h" {
		t.Fatalf("unexpected delivery specs: %#v", input.Deliveries)
	}
}

type fakeServiceStore struct {
	Store
	notificationType NotificationType
	created          CreateNotificationInput
}

func (s *fakeServiceStore) GetType(context.Context, string) (NotificationType, error) {
	return s.notificationType, nil
}

func (s *fakeServiceStore) CreateNotification(_ context.Context, input CreateNotificationInput) (NotifyResult, error) {
	s.created = input
	return NotifyResult{NotificationID: 101, RecipientCount: len(input.Recipients), Created: true}, nil
}
