package notifications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestSQLStoreCreateNotificationIsIdempotent(t *testing.T) {
	db := openNotificationTestDB(t)
	typeKey := seedNotificationType(t, db)
	store := NewSQLStore(db)
	ctx := context.Background()
	dedupeKey := typeKey + ":dedupe"
	input := notificationCreateInput(typeKey, dedupeKey, "po-42", time.Now().UTC().Add(-time.Hour))

	const workers = 8
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.CreateNotification(ctx, input)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("CreateNotification failed under duplicate concurrency: %v", err)
		}
	}

	var notificationCount, recipientCount, deliveryCount int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM mrsmith.notification
		WHERE dedupe_key = $1
	`, dedupeKey).Scan(&notificationCount); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM mrsmith.notification_recipient r
		JOIN mrsmith.notification n ON n.id = r.notification_id
		WHERE n.dedupe_key = $1
	`, dedupeKey).Scan(&recipientCount); err != nil {
		t.Fatalf("count recipients: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM mrsmith.notification_delivery d
		JOIN mrsmith.notification_recipient r ON r.id = d.recipient_id
		JOIN mrsmith.notification n ON n.id = r.notification_id
		WHERE n.dedupe_key = $1
	`, dedupeKey).Scan(&deliveryCount); err != nil {
		t.Fatalf("count deliveries: %v", err)
	}
	if notificationCount != 1 || recipientCount != 2 || deliveryCount != 4 {
		t.Fatalf("unexpected idempotent counts: notifications=%d recipients=%d deliveries=%d", notificationCount, recipientCount, deliveryCount)
	}
}

func TestSQLStoreRecipientScopingPaginationAndCancellation(t *testing.T) {
	db := openNotificationTestDB(t)
	typeKey := seedNotificationType(t, db)
	store := NewSQLStore(db)
	ctx := context.Background()

	first, err := store.CreateNotification(ctx, notificationCreateInput(typeKey, typeKey+":first", "po-1", time.Now().UTC().Add(2*time.Hour)))
	if err != nil {
		t.Fatalf("create first notification: %v", err)
	}
	second, err := store.CreateNotification(ctx, notificationCreateInput(typeKey, typeKey+":second", "po-2", time.Now().UTC().Add(2*time.Hour)))
	if err != nil {
		t.Fatalf("create second notification: %v", err)
	}
	if first.NotificationID == second.NotificationID {
		t.Fatalf("expected distinct notifications")
	}

	summary, err := store.Summary(ctx, aliceEmail(typeKey))
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.TotalUnread != 2 || summary.UnreadByApp["rda"] != 2 {
		t.Fatalf("unexpected alice summary: %#v", summary)
	}
	bobSummary, err := store.Summary(ctx, bobEmail(typeKey))
	if err != nil {
		t.Fatalf("bob summary: %v", err)
	}
	if bobSummary.TotalUnread != 2 {
		t.Fatalf("unexpected bob summary: %#v", bobSummary)
	}

	page1, err := store.List(ctx, ListInput{Email: aliceEmail(typeKey), Status: ListStatusAll, Limit: 1})
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1.Items) != 1 || !page1.HasNext {
		t.Fatalf("unexpected first page: %#v", page1)
	}
	page2, err := store.List(ctx, ListInput{
		Email:           aliceEmail(typeKey),
		Status:          ListStatusAll,
		Limit:           1,
		CursorCreatedAt: page1.NextCreatedAt,
		CursorID:        page1.NextID,
	})
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2.Items) != 1 || page2.Items[0].ID == page1.Items[0].ID {
		t.Fatalf("unexpected second page: %#v", page2)
	}

	if ok, err := store.MarkRead(ctx, aliceEmail(typeKey), page1.Items[0].ID); err != nil || !ok {
		t.Fatalf("mark read ok=%v err=%v", ok, err)
	}
	if ok, err := store.MarkRead(ctx, "mallory@example.com", page2.Items[0].ID); err != nil || ok {
		t.Fatalf("cross-recipient mark read should not update ok=%v err=%v", ok, err)
	}
	assertRecipientDeliveriesCancelled(t, db, page1.Items[0].ID)

	updated, err := store.MarkAllRead(ctx, aliceEmail(typeKey))
	if err != nil {
		t.Fatalf("mark all read: %v", err)
	}
	if updated != 1 {
		t.Fatalf("expected one remaining alice notification marked read, got %d", updated)
	}
	assertRecipientDeliveriesCancelled(t, db, page2.Items[0].ID)

	bobPage, err := store.List(ctx, ListInput{Email: bobEmail(typeKey), Status: ListStatusAll, Limit: 10})
	if err != nil {
		t.Fatalf("bob list: %v", err)
	}
	if len(bobPage.Items) != 2 {
		t.Fatalf("expected bob still scoped to own two items, got %#v", bobPage.Items)
	}
	if ok, err := store.Archive(ctx, bobEmail(typeKey), bobPage.Items[0].ID); err != nil || !ok {
		t.Fatalf("archive ok=%v err=%v", ok, err)
	}
	assertRecipientDeliveriesCancelled(t, db, bobPage.Items[0].ID)
}

func TestSQLStoreResolveClaimAndCompleteDelivery(t *testing.T) {
	db := openNotificationTestDB(t)
	typeKey := seedNotificationType(t, db)
	store := NewSQLStore(db)
	ctx := context.Background()

	claimResult, err := store.CreateNotification(ctx, notificationCreateInput(typeKey, typeKey+":claim", "po-claim", ancientDueAt()))
	if err != nil {
		t.Fatalf("create claim notification: %v", err)
	}
	claimed, err := store.ClaimDueDeliveries(ctx, ClaimInput{WorkerID: "test-worker", Limit: 50, Now: time.Now().UTC()})
	if err != nil {
		t.Fatalf("claim due deliveries: %v", err)
	}
	claimed = filterClaimedDeliveries(claimed, claimResult.NotificationID)
	if len(claimed) != 2 {
		t.Fatalf("expected one email delivery per recipient, got %#v", claimed)
	}
	retryAt := time.Now().UTC().Add(-time.Minute)
	if err := store.CompleteDelivery(ctx, DeliveryCompletion{
		DeliveryID: claimed[0].ID,
		Status:     deliveryStatusFailed,
		Error:      "temporary",
		RetryAt:    &retryAt,
	}); err != nil {
		t.Fatalf("complete retry: %v", err)
	}
	if err := store.CompleteDelivery(ctx, DeliveryCompletion{
		DeliveryID: claimed[1].ID,
		Status:     deliveryStatusSent,
	}); err != nil {
		t.Fatalf("complete sent: %v", err)
	}

	reclaimed, err := store.ClaimDueDeliveries(ctx, ClaimInput{WorkerID: "test-worker", Limit: 50, Now: time.Now().UTC()})
	if err != nil {
		t.Fatalf("reclaim retry: %v", err)
	}
	reclaimed = filterClaimedDeliveries(reclaimed, claimResult.NotificationID)
	if len(reclaimed) != 1 || reclaimed[0].ID != claimed[0].ID || reclaimed[0].PolicyStep != claimed[0].PolicyStep {
		t.Fatalf("expected same failed step to be retried once, got %#v", reclaimed)
	}

	resolveResult, err := store.CreateNotification(ctx, notificationCreateInput(typeKey, typeKey+":resolve", "po-resolve", time.Now().UTC().Add(2*time.Hour)))
	if err != nil {
		t.Fatalf("create resolve notification: %v", err)
	}
	resolved, err := store.Resolve(ctx, ResolveInput{TypeKey: typeKey, EntityType: "rda_po", EntityID: "po-resolve"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved != 2 {
		t.Fatalf("expected two recipients resolved, got %d", resolved)
	}
	assertEntityDeliveriesCancelled(t, db, typeKey, "po-resolve")

	if _, err := store.CreateNotification(ctx, notificationCreateInput(typeKey, typeKey+":resolve", "po-resolve", ancientDueAt())); err != nil {
		t.Fatalf("reactivate resolved notification: %v", err)
	}
	reactivatedSummary, err := store.Summary(ctx, aliceEmail(typeKey))
	if err != nil {
		t.Fatalf("reactivated summary: %v", err)
	}
	if reactivatedSummary.TotalUnread == 0 {
		t.Fatalf("expected resolved dedupe reuse to reactivate recipients, got %#v", reactivatedSummary)
	}
	reactivated, err := store.ClaimDueDeliveries(ctx, ClaimInput{WorkerID: "test-worker", Limit: 50, Now: time.Now().UTC()})
	if err != nil {
		t.Fatalf("claim reactivated deliveries: %v", err)
	}
	reactivated = filterClaimedDeliveries(reactivated, resolveResult.NotificationID)
	if len(reactivated) == 0 {
		t.Fatalf("expected reactivated cancelled email step to be claimable, got %#v", reactivated)
	}
}

func TestSQLStoreCancelledLockedDeliveryCannotBeOverwrittenByCompletion(t *testing.T) {
	db := openNotificationTestDB(t)
	typeKey := seedNotificationType(t, db)
	store := NewSQLStore(db)
	ctx := context.Background()

	inflightResult, err := store.CreateNotification(ctx, notificationCreateInput(typeKey, typeKey+":inflight", "po-inflight", ancientDueAt()))
	if err != nil {
		t.Fatalf("create notification: %v", err)
	}
	claimed, err := store.ClaimDueDeliveries(ctx, ClaimInput{WorkerID: "test-worker", Limit: 50, Now: time.Now().UTC()})
	if err != nil {
		t.Fatalf("claim delivery: %v", err)
	}
	claimed = filterClaimedDeliveries(claimed, inflightResult.NotificationID)
	if len(claimed) == 0 {
		t.Fatalf("expected at least one claimed delivery, got %#v", claimed)
	}
	if ok, err := store.MarkRead(ctx, claimed[0].RecipientEmail, claimed[0].RecipientID); err != nil || !ok {
		t.Fatalf("mark read claimed recipient ok=%v err=%v", ok, err)
	}
	if err := store.CompleteDelivery(ctx, DeliveryCompletion{
		DeliveryID: claimed[0].ID,
		Status:     deliveryStatusSent,
	}); err != nil {
		t.Fatalf("complete cancelled delivery: %v", err)
	}

	var status string
	if err := db.QueryRowContext(ctx, `
		SELECT status
		FROM mrsmith.notification_delivery
		WHERE id = $1
	`, claimed[0].ID).Scan(&status); err != nil {
		t.Fatalf("read delivery status: %v", err)
	}
	if status != deliveryStatusCancelled {
		t.Fatalf("expected cancelled delivery to stay cancelled, got %q", status)
	}
	var attempts int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM mrsmith.notification_delivery_attempt
		WHERE delivery_id = $1
		  AND status = 'cancelled'
	`, claimed[0].ID).Scan(&attempts); err != nil {
		t.Fatalf("count cancelled attempts: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected one cancelled attempt and no completion overwrite, got %d", attempts)
	}
}

func notificationCreateInput(typeKey, dedupeKey, entityID string, emailDueAt time.Time) CreateNotificationInput {
	return CreateNotificationInput{
		TypeKey:            typeKey,
		AppID:              "rda",
		Severity:           "warning",
		Title:              "Approval requested",
		Body:               "A purchase request is waiting.",
		EntityType:         "rda_po",
		EntityID:           entityID,
		DedupeKey:          dedupeKey,
		DeepLink:           "/apps/rda/rda/po/" + entityID,
		MetadataJSON:       []byte(`{"source":"test"}`),
		PolicyOverrideJSON: []byte(`{}`),
		Recipients: []Recipient{
			{Email: aliceEmail(typeKey), Name: "Alice"},
			{Email: bobEmail(typeKey), Name: "Bob"},
		},
		Deliveries: []DeliverySpec{
			{Channel: channelPortal, PolicyStep: portalCreatedStep, Status: deliveryStatusSent, DueAt: time.Now().UTC()},
			{Channel: channelEmail, PolicyStep: "unread_after_4h", Status: deliveryStatusPending, DueAt: emailDueAt},
		},
	}
}

func aliceEmail(typeKey string) string {
	return testEmail("alice", typeKey)
}

func bobEmail(typeKey string) string {
	return testEmail("bob", typeKey)
}

func testEmail(prefix string, typeKey string) string {
	return prefix + "+" + strings.ReplaceAll(typeKey, "_", "-") + "@example.com"
}

func ancientDueAt() time.Time {
	return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
}

func filterClaimedDeliveries(deliveries []ClaimedDelivery, notificationID int64) []ClaimedDelivery {
	filtered := make([]ClaimedDelivery, 0, len(deliveries))
	for _, delivery := range deliveries {
		if delivery.NotificationID == notificationID {
			filtered = append(filtered, delivery)
		}
	}
	return filtered
}

func openNotificationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := notificationTestDSN(t)
	if dsn == "" {
		t.Skip("set NOTIFICATIONS_TEST_DSN or backend/.env ANISETTA_DSN to run SQLStore integration tests")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	return db
}

func notificationTestDSN(t *testing.T) string {
	t.Helper()
	if value := strings.TrimSpace(os.Getenv("NOTIFICATIONS_TEST_DSN")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("ANISETTA_DSN")); value != "" {
		return value
	}
	for _, path := range []string{".env", "../../.env", "../.env"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			t.Fatalf("read %s: %v", path, err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ANISETTA_DSN=") {
				return strings.TrimSpace(strings.TrimPrefix(line, "ANISETTA_DSN="))
			}
		}
	}
	return ""
}

func seedNotificationType(t *testing.T, db *sql.DB) string {
	t.Helper()
	typeKey := fmt.Sprintf("test_notifications_%d", time.Now().UnixNano())
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO mrsmith.notification_type (
			type_key,
			app_id,
			title_template,
			body_template,
			severity,
			default_policy,
			enabled
		)
		VALUES ($1, 'rda', 'Test notification', 'Test body', 'warning', '{}'::jsonb, true)
	`, typeKey)
	if err != nil {
		t.Fatalf("seed notification type: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `
			DELETE FROM mrsmith.notification
			WHERE type_key = $1
		`, typeKey)
		_, _ = db.ExecContext(context.Background(), `
			DELETE FROM mrsmith.notification_type
			WHERE type_key = $1
		`, typeKey)
	})
	return typeKey
}

func assertRecipientDeliveriesCancelled(t *testing.T, db *sql.DB, recipientID int64) {
	t.Helper()
	var pending int
	if err := db.QueryRowContext(context.Background(), `
		SELECT count(*)
		FROM mrsmith.notification_delivery
		WHERE recipient_id = $1
		  AND channel = 'email'
		  AND status IN ('pending', 'locked')
	`, recipientID).Scan(&pending); err != nil {
		t.Fatalf("count pending deliveries: %v", err)
	}
	if pending != 0 {
		t.Fatalf("expected no pending email deliveries for recipient %d, got %d", recipientID, pending)
	}
	var attempts int
	if err := db.QueryRowContext(context.Background(), `
		SELECT count(*)
		FROM mrsmith.notification_delivery_attempt a
		JOIN mrsmith.notification_delivery d ON d.id = a.delivery_id
		WHERE d.recipient_id = $1
		  AND a.status = 'cancelled'
	`, recipientID).Scan(&attempts); err != nil {
		t.Fatalf("count cancelled attempts: %v", err)
	}
	if attempts == 0 {
		t.Fatalf("expected cancelled attempt for recipient %d", recipientID)
	}
}

func assertEntityDeliveriesCancelled(t *testing.T, db *sql.DB, typeKey string, entityID string) {
	t.Helper()
	var pending int
	if err := db.QueryRowContext(context.Background(), `
		SELECT count(*)
		FROM mrsmith.notification_delivery d
		JOIN mrsmith.notification_recipient r ON r.id = d.recipient_id
		JOIN mrsmith.notification n ON n.id = r.notification_id
		WHERE n.type_key = $1
		  AND n.entity_id = $2
		  AND d.status IN ('pending', 'locked')
	`, typeKey, entityID).Scan(&pending); err != nil {
		t.Fatalf("count entity pending deliveries: %v", err)
	}
	if pending != 0 {
		t.Fatalf("expected resolved entity deliveries to be cancelled, got %d pending", pending)
	}
}
