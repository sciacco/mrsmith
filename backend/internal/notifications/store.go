package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) GetType(ctx context.Context, typeKey string) (NotificationType, error) {
	if s == nil || s.db == nil {
		return NotificationType{}, errors.New("notifications database not configured")
	}
	var item NotificationType
	var defaultPolicy []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT type_key, app_id, title_template, body_template, severity, default_policy, enabled
		FROM mrsmith.notification_type
		WHERE type_key = $1
	`, typeKey).Scan(
		&item.TypeKey,
		&item.AppID,
		&item.TitleTemplate,
		&item.BodyTemplate,
		&item.Severity,
		&defaultPolicy,
		&item.Enabled,
	)
	if err != nil {
		return NotificationType{}, fmt.Errorf("read notification type %q: %w", typeKey, err)
	}
	item.DefaultPolicy = append(json.RawMessage(nil), defaultPolicy...)
	return item, nil
}

func (s *SQLStore) CreateNotification(ctx context.Context, input CreateNotificationInput) (NotifyResult, error) {
	if s == nil || s.db == nil {
		return NotifyResult{}, errors.New("notifications database not configured")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return NotifyResult{}, fmt.Errorf("begin notification transaction: %w", err)
	}
	defer rollbackTx(tx)

	var result NotifyResult
	err = tx.QueryRowContext(ctx, `
		INSERT INTO mrsmith.notification (
			type_key,
			app_id,
			severity,
			title,
			body,
			entity_type,
			entity_id,
			dedupe_key,
			deep_link,
			metadata,
			policy_override,
			created_by_subject,
			created_by_email
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10::jsonb, $11::jsonb, $12, $13
		)
		ON CONFLICT (dedupe_key) DO UPDATE
		SET dedupe_key = EXCLUDED.dedupe_key
		RETURNING id, (xmax = 0)
	`,
		input.TypeKey,
		input.AppID,
		input.Severity,
		input.Title,
		input.Body,
		input.EntityType,
		input.EntityID,
		input.DedupeKey,
		input.DeepLink,
		string(input.MetadataJSON),
		string(input.PolicyOverrideJSON),
		input.CreatedBySubject,
		input.CreatedByEmail,
	).Scan(&result.NotificationID, &result.Created)
	if err != nil {
		return NotifyResult{}, fmt.Errorf("insert notification: %w", err)
	}

	recipientIDs := make([]int64, 0, len(input.Recipients))
	reactivatedRecipients := map[int64]struct{}{}
	for _, recipient := range input.Recipients {
		var recipientID int64
		var reactivated bool
		err := tx.QueryRowContext(ctx, `
			SELECT id, resolved_at IS NOT NULL
			FROM mrsmith.notification_recipient
			WHERE notification_id = $1
			  AND lower(recipient_email) = lower($2)
			FOR UPDATE
		`, result.NotificationID, recipient.Email).Scan(&recipientID, &reactivated)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `
				INSERT INTO mrsmith.notification_recipient (
					notification_id,
					recipient_subject,
					recipient_email,
					recipient_name
				)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (notification_id, (lower(recipient_email))) DO UPDATE
				SET recipient_subject = CASE
						WHEN EXCLUDED.recipient_subject <> '' THEN EXCLUDED.recipient_subject
						ELSE mrsmith.notification_recipient.recipient_subject
					END,
					recipient_name = CASE
						WHEN EXCLUDED.recipient_name <> '' THEN EXCLUDED.recipient_name
						ELSE mrsmith.notification_recipient.recipient_name
					END
				RETURNING id
			`, result.NotificationID, recipient.Subject, recipient.Email, recipient.Name).Scan(&recipientID)
			reactivated = false
		}
		if err != nil {
			return NotifyResult{}, fmt.Errorf("upsert notification recipient: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE mrsmith.notification_recipient
			SET recipient_subject = CASE
					WHEN $2 <> '' THEN $2
					ELSE recipient_subject
				END,
				recipient_name = CASE
					WHEN $3 <> '' THEN $3
					ELSE recipient_name
				END,
				read_at = CASE WHEN $4 THEN NULL ELSE read_at END,
				archived_at = CASE WHEN $4 THEN NULL ELSE archived_at END,
				resolved_at = CASE WHEN $4 THEN NULL ELSE resolved_at END
			WHERE id = $1
		`, recipientID, recipient.Subject, recipient.Name, reactivated); err != nil {
			return NotifyResult{}, fmt.Errorf("update notification recipient: %w", err)
		}
		if reactivated {
			reactivatedRecipients[recipientID] = struct{}{}
		}
		recipientIDs = append(recipientIDs, recipientID)
	}

	for _, recipientID := range recipientIDs {
		for _, delivery := range input.Deliveries {
			var sentAt any
			if delivery.Status == deliveryStatusSent {
				sentAt = delivery.DueAt
			}
			_, reactivated := reactivatedRecipients[recipientID]
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO mrsmith.notification_delivery (
					recipient_id,
					channel,
					policy_step,
					status,
					due_at,
					sent_at
				)
				VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (recipient_id, channel, policy_step) DO UPDATE
				SET status = CASE
						WHEN $7 AND mrsmith.notification_delivery.status IN ('sent', 'skipped', 'failed', 'cancelled') THEN EXCLUDED.status
						ELSE mrsmith.notification_delivery.status
					END,
					due_at = CASE
						WHEN $7 AND mrsmith.notification_delivery.status IN ('sent', 'skipped', 'failed', 'cancelled') THEN EXCLUDED.due_at
						ELSE mrsmith.notification_delivery.due_at
					END,
					attempt_count = CASE
						WHEN $7 AND mrsmith.notification_delivery.status IN ('sent', 'skipped', 'failed', 'cancelled') THEN 0
						ELSE mrsmith.notification_delivery.attempt_count
					END,
					locked_at = CASE
						WHEN $7 AND mrsmith.notification_delivery.status IN ('sent', 'skipped', 'failed', 'cancelled') THEN NULL
						ELSE mrsmith.notification_delivery.locked_at
					END,
					locked_by = CASE
						WHEN $7 AND mrsmith.notification_delivery.status IN ('sent', 'skipped', 'failed', 'cancelled') THEN ''
						ELSE mrsmith.notification_delivery.locked_by
					END,
					sent_at = CASE
						WHEN $7 AND mrsmith.notification_delivery.status IN ('sent', 'skipped', 'failed', 'cancelled') THEN EXCLUDED.sent_at
						ELSE mrsmith.notification_delivery.sent_at
					END,
					skipped_at = CASE
						WHEN $7 AND mrsmith.notification_delivery.status IN ('sent', 'skipped', 'failed', 'cancelled') THEN NULL
						ELSE mrsmith.notification_delivery.skipped_at
					END,
					failed_at = CASE
						WHEN $7 AND mrsmith.notification_delivery.status IN ('sent', 'skipped', 'failed', 'cancelled') THEN NULL
						ELSE mrsmith.notification_delivery.failed_at
					END,
					last_error = CASE
						WHEN $7 AND mrsmith.notification_delivery.status IN ('sent', 'skipped', 'failed', 'cancelled') THEN ''
						ELSE mrsmith.notification_delivery.last_error
					END
			`, recipientID, delivery.Channel, delivery.PolicyStep, delivery.Status, delivery.DueAt, sentAt, reactivated); err != nil {
				return NotifyResult{}, fmt.Errorf("insert notification delivery: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return NotifyResult{}, fmt.Errorf("commit notification transaction: %w", err)
	}
	result.RecipientCount = len(recipientIDs)
	return result, nil
}

func (s *SQLStore) Summary(ctx context.Context, email string) (Summary, error) {
	if s == nil || s.db == nil {
		return Summary{}, errors.New("notifications database not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT n.app_id, count(*)
		FROM mrsmith.notification_recipient r
		JOIN mrsmith.notification n ON n.id = r.notification_id
		WHERE lower(r.recipient_email) = lower($1)
		  AND r.read_at IS NULL
		  AND r.archived_at IS NULL
		  AND r.resolved_at IS NULL
		GROUP BY n.app_id
	`, email)
	if err != nil {
		return Summary{}, fmt.Errorf("read notification summary: %w", err)
	}
	defer rows.Close()

	summary := Summary{UnreadByApp: map[string]int64{}}
	for rows.Next() {
		var appID string
		var count int64
		if err := rows.Scan(&appID, &count); err != nil {
			return Summary{}, fmt.Errorf("scan notification summary: %w", err)
		}
		summary.UnreadByApp[appID] = count
		summary.TotalUnread += count
	}
	if err := rows.Err(); err != nil {
		return Summary{}, fmt.Errorf("iterate notification summary: %w", err)
	}
	return summary, nil
}

func (s *SQLStore) List(ctx context.Context, input ListInput) (ListResult, error) {
	if s == nil || s.db == nil {
		return ListResult{}, errors.New("notifications database not configured")
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 30
	}

	args := []any{input.Email, limit + 1}
	filters := []string{
		"lower(r.recipient_email) = lower($1)",
		"r.archived_at IS NULL",
		"r.resolved_at IS NULL",
	}
	if input.Status == ListStatusUnread {
		filters = append(filters, "r.read_at IS NULL")
	}
	if input.AppID != "" {
		args = append(args, input.AppID)
		filters = append(filters, fmt.Sprintf("n.app_id = $%d", len(args)))
	}
	if !input.CursorCreatedAt.IsZero() && input.CursorID > 0 {
		args = append(args, input.CursorCreatedAt, input.CursorID)
		cursorAtPos := len(args) - 1
		cursorIDPos := len(args)
		filters = append(filters, fmt.Sprintf("(r.created_at < $%d OR (r.created_at = $%d AND r.id < $%d))", cursorAtPos, cursorAtPos, cursorIDPos))
	}

	query := fmt.Sprintf(`
		SELECT
			r.id,
			r.notification_id,
			n.type_key,
			n.app_id,
			n.severity,
			n.title,
			n.body,
			n.entity_type,
			n.entity_id,
			n.deep_link,
			n.metadata,
			r.created_at,
			r.read_at,
			r.archived_at,
			r.resolved_at
		FROM mrsmith.notification_recipient r
		JOIN mrsmith.notification n ON n.id = r.notification_id
		WHERE %s
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $2
	`, strings.Join(filters, "\n\t\t  AND "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	items := make([]NotificationItem, 0, limit)
	for rows.Next() {
		var item NotificationItem
		var metadata []byte
		if err := rows.Scan(
			&item.ID,
			&item.NotificationID,
			&item.TypeKey,
			&item.AppID,
			&item.Severity,
			&item.Title,
			&item.Body,
			&item.EntityType,
			&item.EntityID,
			&item.DeepLink,
			&metadata,
			&item.CreatedAt,
			&item.ReadAt,
			&item.ArchivedAt,
			&item.ResolvedAt,
		); err != nil {
			return ListResult{}, fmt.Errorf("scan notification item: %w", err)
		}
		item.Metadata = append(json.RawMessage(nil), metadata...)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, fmt.Errorf("iterate notification items: %w", err)
	}

	result := ListResult{}
	if len(items) > limit {
		result.HasNext = true
		next := items[limit-1]
		result.NextCreatedAt = next.CreatedAt
		result.NextID = next.ID
		items = items[:limit]
	}
	result.Items = items
	return result, nil
}

func (s *SQLStore) MarkRead(ctx context.Context, email string, recipientID int64) (bool, error) {
	return s.updateRecipientTimestamp(ctx, `
		UPDATE mrsmith.notification_recipient
		SET read_at = COALESCE(read_at, now())
		WHERE id = $1
		  AND lower(recipient_email) = lower($2)
		  AND archived_at IS NULL
		  AND resolved_at IS NULL
		RETURNING id
	`, recipientID, email)
}

func (s *SQLStore) MarkAllRead(ctx context.Context, email string) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("notifications database not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin mark all notifications read transaction: %w", err)
	}
	defer rollbackTx(tx)

	rows, err := tx.QueryContext(ctx, `
		UPDATE mrsmith.notification_recipient
		SET read_at = COALESCE(read_at, now())
		WHERE lower(recipient_email) = lower($1)
		  AND read_at IS NULL
		  AND archived_at IS NULL
		  AND resolved_at IS NULL
		RETURNING id
	`, email)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", err)
	}
	recipientIDs, err := scanInt64Rows(rows, "mark all notifications read")
	if err != nil {
		return 0, err
	}
	if err := cancelPendingDeliveriesForRecipients(ctx, tx, recipientIDs, "recipient_read"); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit mark all notifications read transaction: %w", err)
	}
	return int64(len(recipientIDs)), nil
}

func (s *SQLStore) Archive(ctx context.Context, email string, recipientID int64) (bool, error) {
	return s.updateRecipientTimestamp(ctx, `
		UPDATE mrsmith.notification_recipient
		SET archived_at = COALESCE(archived_at, now())
		WHERE id = $1
		  AND lower(recipient_email) = lower($2)
		  AND resolved_at IS NULL
		RETURNING id
	`, recipientID, email)
}

func (s *SQLStore) updateRecipientTimestamp(ctx context.Context, query string, recipientID int64, email string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("notifications database not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin notification recipient update transaction: %w", err)
	}
	defer rollbackTx(tx)

	rows, err := tx.QueryContext(ctx, query, recipientID, email)
	if err != nil {
		return false, fmt.Errorf("update notification recipient %d: %w", recipientID, err)
	}
	recipientIDs, err := scanInt64Rows(rows, "update notification recipient")
	if err != nil {
		return false, err
	}
	if err := cancelPendingDeliveriesForRecipients(ctx, tx, recipientIDs, "recipient_inactive"); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit notification recipient update transaction: %w", err)
	}
	return len(recipientIDs) > 0, nil
}

func (s *SQLStore) Resolve(ctx context.Context, input ResolveInput) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("notifications database not configured")
	}
	args, filters, err := resolveFilters(input)
	if err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin notification resolve transaction: %w", err)
	}
	defer rollbackTx(tx)

	resolveQuery := fmt.Sprintf(`
		UPDATE mrsmith.notification_recipient r
		SET resolved_at = COALESCE(resolved_at, now())
		FROM mrsmith.notification n
		WHERE r.notification_id = n.id
		  AND r.resolved_at IS NULL
		  AND %s
	`, strings.Join(filters, "\n\t\t  AND "))
	result, err := tx.ExecContext(ctx, resolveQuery, args...)
	if err != nil {
		return 0, fmt.Errorf("resolve notification recipients: %w", err)
	}
	count, _ := result.RowsAffected()

	cancelQuery := fmt.Sprintf(`
		WITH cancelled AS (
			UPDATE mrsmith.notification_delivery d
			SET status = 'cancelled',
			    locked_at = NULL,
			    locked_by = '',
			    last_error = 'notification_resolved'
			FROM mrsmith.notification_recipient r
			JOIN mrsmith.notification n ON n.id = r.notification_id
			WHERE d.recipient_id = r.id
			  AND d.status IN ('pending', 'locked')
			  AND %s
			RETURNING d.id
		)
		INSERT INTO mrsmith.notification_delivery_attempt (delivery_id, status, error)
		SELECT id, 'cancelled', 'notification_resolved'
		FROM cancelled
	`, strings.Join(filters, "\n\t\t  AND "))
	if _, err := tx.ExecContext(ctx, cancelQuery, args...); err != nil {
		return 0, fmt.Errorf("cancel resolved notification deliveries: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit notification resolve transaction: %w", err)
	}
	return count, nil
}

func resolveFilters(input ResolveInput) ([]any, []string, error) {
	args := []any{}
	filters := []string{}
	if input.DedupeKey != "" {
		args = append(args, input.DedupeKey)
		filters = append(filters, fmt.Sprintf("n.dedupe_key = $%d", len(args)))
	}
	if input.TypeKey != "" {
		args = append(args, input.TypeKey)
		filters = append(filters, fmt.Sprintf("n.type_key = $%d", len(args)))
	}
	if input.EntityType != "" {
		args = append(args, input.EntityType)
		filters = append(filters, fmt.Sprintf("n.entity_type = $%d", len(args)))
	}
	if input.EntityID != "" {
		args = append(args, input.EntityID)
		filters = append(filters, fmt.Sprintf("n.entity_id = $%d", len(args)))
	}
	if len(filters) == 0 {
		return nil, nil, errors.New("notification resolve scope is required")
	}
	return args, filters, nil
}

func (s *SQLStore) ClaimDueDeliveries(ctx context.Context, input ClaimInput) ([]ClaimedDelivery, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("notifications database not configured")
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin delivery claim transaction: %w", err)
	}
	defer rollbackTx(tx)

	rows, err := tx.QueryContext(ctx, `
		WITH due AS (
			SELECT d.id
			FROM mrsmith.notification_delivery d
			JOIN mrsmith.notification_recipient r ON r.id = d.recipient_id
			WHERE d.status = 'pending'
			  AND d.due_at <= $1
			  AND r.read_at IS NULL
			  AND r.archived_at IS NULL
			  AND r.resolved_at IS NULL
			ORDER BY d.due_at ASC, d.id ASC
			LIMIT $2
			FOR UPDATE OF d SKIP LOCKED
		),
		locked AS (
			UPDATE mrsmith.notification_delivery d
			SET status = 'locked',
			    locked_at = $1,
			    locked_by = $3,
			    attempt_count = attempt_count + 1
			FROM due
			WHERE d.id = due.id
			RETURNING d.id, d.recipient_id, d.channel, d.policy_step, d.attempt_count, d.due_at
		)
		SELECT
			locked.id,
			locked.recipient_id,
			locked.channel,
			locked.policy_step,
			locked.attempt_count,
			locked.due_at,
			r.recipient_email,
			r.recipient_name,
			r.read_at,
			r.archived_at,
			r.resolved_at,
			n.id,
			n.type_key,
			n.app_id,
			n.severity,
			n.title,
			n.body,
			n.deep_link,
			n.created_at
		FROM locked
		JOIN mrsmith.notification_recipient r ON r.id = locked.recipient_id
		JOIN mrsmith.notification n ON n.id = r.notification_id
		ORDER BY locked.due_at ASC, locked.id ASC
	`, now, limit, input.WorkerID)
	if err != nil {
		return nil, fmt.Errorf("claim due notification deliveries: %w", err)
	}
	defer rows.Close()

	deliveries := make([]ClaimedDelivery, 0)
	for rows.Next() {
		var item ClaimedDelivery
		if err := rows.Scan(
			&item.ID,
			&item.RecipientID,
			&item.Channel,
			&item.PolicyStep,
			&item.AttemptCount,
			&item.DueAt,
			&item.RecipientEmail,
			&item.RecipientName,
			&item.ReadAt,
			&item.ArchivedAt,
			&item.ResolvedAt,
			&item.NotificationID,
			&item.TypeKey,
			&item.AppID,
			&item.Severity,
			&item.Title,
			&item.Body,
			&item.DeepLink,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan claimed delivery: %w", err)
		}
		deliveries = append(deliveries, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed deliveries: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit delivery claim transaction: %w", err)
	}
	return deliveries, nil
}

func (s *SQLStore) CompleteDelivery(ctx context.Context, completion DeliveryCompletion) error {
	if s == nil || s.db == nil {
		return errors.New("notifications database not configured")
	}
	status := strings.TrimSpace(completion.Status)
	if status == "" {
		return errors.New("delivery completion status is required")
	}
	now := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delivery completion transaction: %w", err)
	}
	defer rollbackTx(tx)

	attemptStatus := status
	if completion.RetryAt != nil {
		attemptStatus = deliveryStatusFailed
	}
	var currentStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT status
		FROM mrsmith.notification_delivery
		WHERE id = $1
		FOR UPDATE
	`, completion.DeliveryID).Scan(&currentStatus)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit missing delivery completion transaction: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read delivery completion status: %w", err)
	}
	if currentStatus != "locked" {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit ignored delivery completion transaction: %w", err)
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mrsmith.notification_delivery_attempt (delivery_id, status, error, attempted_at)
		VALUES ($1, $2, $3, $4)
	`, completion.DeliveryID, attemptStatus, completion.Error, now); err != nil {
		return fmt.Errorf("insert delivery attempt: %w", err)
	}

	if completion.RetryAt != nil {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mrsmith.notification_delivery
			SET status = 'pending',
			    due_at = $2,
			    locked_at = NULL,
			    locked_by = '',
			    last_error = $3
			WHERE id = $1
			  AND status = 'locked'
		`, completion.DeliveryID, *completion.RetryAt, completion.Error); err != nil {
			return fmt.Errorf("schedule delivery retry: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mrsmith.notification_delivery
			SET status = $2,
			    locked_at = NULL,
			    locked_by = '',
			    sent_at = CASE WHEN $2 = 'sent' THEN $4 ELSE sent_at END,
			    skipped_at = CASE WHEN $2 = 'skipped' THEN $4 ELSE skipped_at END,
			    failed_at = CASE WHEN $2 = 'failed' THEN $4 ELSE failed_at END,
			    last_error = $3
			WHERE id = $1
			  AND status = 'locked'
		`, completion.DeliveryID, status, completion.Error, now); err != nil {
			return fmt.Errorf("complete delivery: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delivery completion transaction: %w", err)
	}
	return nil
}

func scanInt64Rows(rows *sql.Rows, operation string) ([]int64, error) {
	defer rows.Close()
	values := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan %s row: %w", operation, err)
		}
		values = append(values, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s rows: %w", operation, err)
	}
	return values, nil
}

func cancelPendingDeliveriesForRecipients(ctx context.Context, tx *sql.Tx, recipientIDs []int64, reason string) error {
	if len(recipientIDs) == 0 {
		return nil
	}
	placeholders := make([]string, 0, len(recipientIDs))
	args := make([]any, 0, len(recipientIDs)+1)
	for _, id := range recipientIDs {
		args = append(args, id)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}
	args = append(args, reason)
	reasonPos := len(args)
	query := fmt.Sprintf(`
		WITH cancelled AS (
			UPDATE mrsmith.notification_delivery d
			SET status = 'cancelled',
			    locked_at = NULL,
			    locked_by = '',
			    last_error = $%d
			WHERE d.recipient_id IN (%s)
			  AND d.status IN ('pending', 'locked')
			RETURNING d.id
		)
		INSERT INTO mrsmith.notification_delivery_attempt (delivery_id, status, error)
		SELECT id, 'cancelled', $%d
		FROM cancelled
	`, reasonPos, strings.Join(placeholders, ","), reasonPos)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("cancel pending notification deliveries: %w", err)
	}
	return nil
}

func rollbackTx(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}
