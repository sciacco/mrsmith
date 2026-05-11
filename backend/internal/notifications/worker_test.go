package notifications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/email"
)

func TestWorkerSkipsEmailWhenSMTPDisabled(t *testing.T) {
	store := &fakeWorkerStore{deliveries: []ClaimedDelivery{emailDelivery(1)}}
	worker := NewWorker(store, WorkerConfig{Mailer: disabledMailer{}, PublicBaseURL: "https://mrsmith.example.com"})
	worker.ProcessOnce(context.Background())

	if len(store.completions) != 1 {
		t.Fatalf("expected one completion, got %#v", store.completions)
	}
	got := store.completions[0]
	if got.Status != deliveryStatusSkipped || got.Error != "smtp_disabled" {
		t.Fatalf("expected smtp skip, got %#v", got)
	}
}

func TestWorkerRetriesFailedEmailWithoutDuplicatingStep(t *testing.T) {
	store := &fakeWorkerStore{deliveries: []ClaimedDelivery{emailDelivery(1)}}
	now := time.Date(2026, 5, 10, 8, 0, 0, 0, time.UTC)
	worker := NewWorker(store, WorkerConfig{
		Mailer:        failingMailer{},
		PublicBaseURL: "https://mrsmith.example.com",
	})
	worker.now = func() time.Time { return now }

	worker.ProcessOnce(context.Background())

	if len(store.completions) != 1 {
		t.Fatalf("expected one completion, got %#v", store.completions)
	}
	got := store.completions[0]
	if got.Status != deliveryStatusFailed || got.RetryAt == nil || !got.RetryAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("expected bounded retry, got %#v", got)
	}
}

func TestWorkerSkipsEmailWhenPublicBaseURLMissing(t *testing.T) {
	store := &fakeWorkerStore{deliveries: []ClaimedDelivery{emailDelivery(1)}}
	worker := NewWorker(store, WorkerConfig{Mailer: enabledMailer{}})

	worker.ProcessOnce(context.Background())

	if len(store.completions) != 1 {
		t.Fatalf("expected one completion, got %#v", store.completions)
	}
	got := store.completions[0]
	if got.Status != deliveryStatusSkipped || got.Error != "mrsmith_public_base_url_missing" {
		t.Fatalf("expected missing-base-url skip, got %#v", got)
	}
}

func TestWorkerCancelsReadResolvedDeliveries(t *testing.T) {
	readAt := time.Date(2026, 5, 10, 7, 0, 0, 0, time.UTC)
	delivery := emailDelivery(1)
	delivery.ReadAt = &readAt
	store := &fakeWorkerStore{deliveries: []ClaimedDelivery{delivery}}
	worker := NewWorker(store, WorkerConfig{Mailer: enabledMailer{}, PublicBaseURL: "https://mrsmith.example.com"})

	worker.ProcessOnce(context.Background())

	if len(store.completions) != 1 || store.completions[0].Status != deliveryStatusCancelled {
		t.Fatalf("expected cancelled completion, got %#v", store.completions)
	}
}

func emailDelivery(attempt int) ClaimedDelivery {
	return ClaimedDelivery{
		ID:             10,
		Channel:        channelEmail,
		PolicyStep:     "unread_after_4h",
		AttemptCount:   attempt,
		RecipientEmail: "approver@example.com",
		AppID:          "rda",
		Title:          "Approval requested",
		Body:           "A PO is waiting.",
		DeepLink:       "/apps/rda/rda/po/42",
	}
}

type fakeWorkerStore struct {
	Store
	deliveries  []ClaimedDelivery
	completions []DeliveryCompletion
}

func (s *fakeWorkerStore) ClaimDueDeliveries(context.Context, ClaimInput) ([]ClaimedDelivery, error) {
	return s.deliveries, nil
}

func (s *fakeWorkerStore) CompleteDelivery(_ context.Context, completion DeliveryCompletion) error {
	s.completions = append(s.completions, completion)
	return nil
}

type disabledMailer struct{}

func (disabledMailer) Enabled() bool { return false }
func (disabledMailer) Send(context.Context, email.Message) error {
	return errors.New("disabled")
}

type enabledMailer struct{}

func (enabledMailer) Enabled() bool { return true }
func (enabledMailer) Send(context.Context, email.Message) error {
	return nil
}

type failingMailer struct{}

func (failingMailer) Enabled() bool { return true }
func (failingMailer) Send(context.Context, email.Message) error {
	return errors.New("temporary smtp failure")
}
