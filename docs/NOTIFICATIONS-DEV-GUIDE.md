# Notifications Developer Guide

This guide explains how to make a mini-app activity create a MrSmith user notification.

## Runtime Requirements

- `ANISETTA_DSN` must be configured.
- `deploy/migrations/006_anisetta_mrsmith_notifications.sql` must be applied.
- Portal notifications work without SMTP.
- Self-mentions are skipped by default. Set `NOTIFY_SELF_MENTIONS=true` only in local/development environments if you want to test mention notifications without notifying other users.
- Email reminders also require:
  - `NOTIFICATIONS_WORKER_ENABLED=true` (default)
  - SMTP configuration
  - `MRSMITH_PUBLIC_BASE_URL`

## 1. Register a Notification Type

Add or update a row in `mrsmith.notification_type`.

```sql
INSERT INTO mrsmith.notification_type (
  type_key,
  app_id,
  title_template,
  body_template,
  severity,
  default_policy
)
VALUES (
  'my_app_activity_assigned',
  'my_app',
  'Activity assigned',
  'An activity requires your attention.',
  'warning',
  '{
    "portal": { "enabled": true },
    "email": {
      "enabled": false,
      "steps": []
    }
  }'::jsonb
)
ON CONFLICT (type_key) DO UPDATE
SET app_id = EXCLUDED.app_id,
    title_template = EXCLUDED.title_template,
    body_template = EXCLUDED.body_template,
    severity = EXCLUDED.severity,
    default_policy = EXCLUDED.default_policy,
    enabled = true;
```

Use a stable `type_key`. The notification service refuses disabled or unknown types.

## 2. Wire the Notifier Into the Mini-App Backend

Mini-app modules must not write notification tables directly.

Add `notifications.Notifier` to the module dependencies and handler, then pass the shared notifier from `backend/cmd/server/main.go`.

RDA is the reference implementation:

- `backend/internal/rda/types.go`
- `backend/internal/rda/notifications.go`
- `backend/cmd/server/main.go`

The notifier is optional. Handlers should safely no-op when it is `nil`.

## 3. Create the Notification From the Activity

Call `Notify` only after the activity state change has succeeded.

```go
claims, _ := auth.GetClaims(ctx)

_, err := h.notifier.Notify(ctx, notifications.NotifyInput{
    TypeKey:    "my_app_activity_assigned",
    Title:      "Activity assigned",
    Body:       "An activity requires your attention.",
    EntityType: "my_app_activity",
    EntityID:   activityID,
    DedupeKey:  fmt.Sprintf("my-app:activity:%s:assigned:%s", activityID, assigneeEmail),
    DeepLink:   "/apps/my-app/activities/" + url.PathEscape(activityID),
    Metadata: map[string]any{
        "activity_id": activityID,
    },
    Recipients: []notifications.Recipient{
        {Email: assigneeEmail},
    },
    CreatedBySubject: claims.Subject,
    CreatedByEmail:   claims.Email,
})
```

Required fields:

- `TypeKey`
- `DedupeKey`
- at least one valid recipient email

Recommended fields:

- `EntityType` and `EntityID` for later resolution
- `DeepLink` so users can open the activity from the bell or email
- `Metadata` for compact context, not full business payloads

Use deterministic `DedupeKey` values. Duplicate calls with the same key are idempotent.

## 4. Resolve Obsolete Notifications

When the activity is completed, cancelled, approved, rejected, or reassigned, resolve stale notifications.

```go
err := h.notifier.Resolve(ctx, notifications.ResolveInput{
    TypeKey:    "my_app_activity_assigned",
    EntityType: "my_app_activity",
    EntityID:   activityID,
})
```

Resolve before creating the next notification when workflow ownership moves to another user or level.

## 5. Frontend Behavior

Mini-apps using `AppShell` with `support={auth}` get the clean notification bell automatically.

```tsx
<AppShell appName="My App" userName={user?.name} onLogout={logout} support={auth}>
  <AppShell.Content>
    <AppRoutes />
  </AppShell.Content>
</AppShell>
```

Disable it only when needed:

```tsx
<AppShell support={auth} notifications={false}>
```

The bell reads `/api/notifications/v1/summary` and `/api/notifications/v1/items`, marks items read, archives them, and follows `deepLink`.

## Quick Checklist

- Notification type exists and is enabled.
- `app_id` matches the mini-app id.
- Backend module receives `notifications.Notifier`.
- `Notify` is called after the business action succeeds.
- Recipients are real user email addresses.
- `DedupeKey` is stable and unique for the business event.
- `DeepLink` opens the relevant activity.
- Stale notifications are resolved on terminal or superseding state changes.
