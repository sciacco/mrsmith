package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultWorkerInterval = time.Minute
	defaultClaimLimit     = 10
	maxEmailAttempts      = 3
)

var errMissingPublicBaseURL = errors.New("mrsmith public base url is not configured")

type Worker struct {
	store         Store
	mailer        Mailer
	publicBaseURL string
	interval      time.Duration
	workerID      string
	logger        *slog.Logger
	now           func() time.Time
}

type WorkerConfig struct {
	Mailer        Mailer
	PublicBaseURL string
	Interval      time.Duration
	WorkerID      string
	Logger        *slog.Logger
}

func NewWorker(store Store, cfg WorkerConfig) *Worker {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	interval := cfg.Interval
	if interval <= 0 {
		interval = defaultWorkerInterval
	}
	workerID := strings.TrimSpace(cfg.WorkerID)
	if workerID == "" {
		workerID = defaultWorkerID()
	}
	return &Worker{
		store:         store,
		mailer:        cfg.Mailer,
		publicBaseURL: strings.TrimSpace(cfg.PublicBaseURL),
		interval:      interval,
		workerID:      workerID,
		logger:        logger.With("component", component, "worker_id", workerID),
		now:           time.Now,
	}
}

func (w *Worker) Run(ctx context.Context) {
	if w == nil || w.store == nil {
		return
	}
	w.logger.Info("notifications worker started", "interval", w.interval.String())
	defer w.logger.Info("notifications worker stopped")

	w.processOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processOnce(ctx)
		}
	}
}

func (w *Worker) ProcessOnce(ctx context.Context) {
	w.processOnce(ctx)
}

func (w *Worker) processOnce(ctx context.Context) {
	deliveries, err := w.store.ClaimDueDeliveries(ctx, ClaimInput{
		WorkerID: w.workerID,
		Limit:    defaultClaimLimit,
		Now:      w.now().UTC(),
	})
	if err != nil {
		if ctx.Err() == nil {
			w.logger.Warn("claim due deliveries failed", "error", err)
		}
		return
	}
	for _, delivery := range deliveries {
		if err := w.processDelivery(ctx, delivery); err != nil && ctx.Err() == nil {
			w.logger.Warn("notification delivery processing failed", "delivery_id", delivery.ID, "error", err)
		}
	}
}

func (w *Worker) processDelivery(ctx context.Context, delivery ClaimedDelivery) error {
	if delivery.ReadAt != nil || delivery.ArchivedAt != nil || delivery.ResolvedAt != nil {
		return w.store.CompleteDelivery(ctx, DeliveryCompletion{
			DeliveryID: delivery.ID,
			Status:     deliveryStatusCancelled,
			Error:      "recipient_not_active",
		})
	}
	if delivery.Channel != channelEmail {
		return w.store.CompleteDelivery(ctx, DeliveryCompletion{
			DeliveryID: delivery.ID,
			Status:     deliveryStatusSent,
		})
	}
	if w.mailer == nil || !w.mailer.Enabled() {
		return w.store.CompleteDelivery(ctx, DeliveryCompletion{
			DeliveryID: delivery.ID,
			Status:     deliveryStatusSkipped,
			Error:      "smtp_disabled",
		})
	}
	message, err := notificationEmail(delivery, w.publicBaseURL)
	if err != nil {
		reason := "email_render_failed"
		if errors.Is(err, errMissingPublicBaseURL) {
			reason = "mrsmith_public_base_url_missing"
		}
		return w.store.CompleteDelivery(ctx, DeliveryCompletion{
			DeliveryID: delivery.ID,
			Status:     deliveryStatusSkipped,
			Error:      reason,
		})
	}
	if err := w.mailer.Send(ctx, message); err != nil {
		completion := DeliveryCompletion{
			DeliveryID: delivery.ID,
			Status:     deliveryStatusFailed,
			Error:      truncateError(err),
		}
		if delivery.AttemptCount < maxEmailAttempts {
			retryAt := w.now().UTC().Add(retryDelay(delivery.AttemptCount))
			completion.RetryAt = &retryAt
		}
		return w.store.CompleteDelivery(ctx, completion)
	}
	return w.store.CompleteDelivery(ctx, DeliveryCompletion{
		DeliveryID: delivery.ID,
		Status:     deliveryStatusSent,
	})
}

func retryDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return 5 * time.Minute
	}
	if attempt == 2 {
		return 15 * time.Minute
	}
	return 30 * time.Minute
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 900 {
		message = message[:900]
	}
	return message
}

func defaultWorkerID() string {
	host, _ := os.Hostname()
	host = strings.TrimSpace(host)
	if host == "" {
		host = "mrsmith"
	}
	return fmt.Sprintf("%s-%s", host, uuid.NewString())
}
