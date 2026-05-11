# Notifications V1 - LLM Agent Execution Plan

## 1. Purpose

This document is the execution contract for implementing MrSmith Notifications V1.

The assigned LLM must act as an **Orchestrator**. The Orchestrator coordinates the work, delegates implementation slices to specialized subagents, integrates their results, runs QA gates, and iterates until every control passes. The Orchestrator must not produce a final completion report while any gate is still `FAIL` or `BLOCKED`.

## 2. Required Repo Reading

Before planning code changes or spawning implementation subagents, the Orchestrator must read:

- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `docs/UI-UX.md`
- `deploy/migrations/004_anisetta_mrsmith_support.sql`
- `deploy/migrations/005_anisetta_mrsmith_support_attachments.sql`
- `deploy/migrations/006_anisetta_mrsmith_notifications.sql`
- `backend/internal/support/`
- `backend/internal/platform/email/`
- `backend/internal/rda/`
- `apps/portal/`
- `packages/ui/`

Targeted automated tests are approved for this feature. They must cover idempotency, policy merging, delivery worker behavior, user-facing APIs, security boundaries, and the RDA pilot integration.

## 3. V1 Goal

Implement a central `notifications` module in the Go backend, persisted in the `mrsmith` schema of the Anisetta PostgreSQL database, with:

- persistent portal inbox;
- unread badge in the Matrix launcher;
- Keycloak-protected REST APIs;
- lightweight browser polling;
- deferred email delivery with multiple reminders;
- default policies per notification type;
- per-notification policy overrides at creation time;
- RDA approvals as the first producer.

V1 explicitly excludes external brokers, WebSocket, SSE, browser push, admin policy UI, email digest, user-managed preferences, and chat-style internal messaging.

## 4. Locked Decisions

- Database: use `ANISETTA_DSN`, PostgreSQL schema `mrsmith`.
- Schema state: the target Anisetta database is already updated with `deploy/migrations/006_anisetta_mrsmith_notifications.sql`. Implementation agents must treat that file as the schema contract, not as pending migration work.
- Email: reuse `backend/internal/platform/email` and existing SMTP configuration.
- Runtime: run the delivery worker inside the backend process with graceful shutdown.
- First producer: RDA approvals.
- RDA V1 reminder policy: email reminders after `4h`, `24h`, and `72h` when the recipient has not read or resolved the notification.
- Recipient identity: normalized email is the operational identity key; save Keycloak `subject` when available.
- Deep links: RDA PO links must work from the portal and from email.
- New required env var for absolute email links: `MRSMITH_PUBLIC_BASE_URL`.
- Optional worker env vars:
  - `NOTIFICATIONS_WORKER_ENABLED=true`
  - `NOTIFICATIONS_WORKER_INTERVAL=60s`

If SMTP is disabled or `MRSMITH_PUBLIC_BASE_URL` is missing, portal notifications must still work. Email deliveries must be marked `skipped` with an auditable reason.

## 5. Target Architecture

### 5.1 Backend Package

Create `backend/internal/notifications` with these responsibilities:

- REST route registration;
- SQL store;
- internal `Notifier` service;
- policy and delivery scheduling;
- asynchronous worker;
- email rendering;
- targeted tests.

Application modules must not write notification tables directly. They must depend on an internal interface:

```go
type Notifier interface {
    Notify(ctx context.Context, input NotifyInput) (NotifyResult, error)
    Resolve(ctx context.Context, input ResolveInput) error
}
```

### 5.2 Existing Schema Contract

The target database already contains:

- `mrsmith.notification_type`
- `mrsmith.notification`
- `mrsmith.notification_recipient`
- `mrsmith.notification_delivery`
- `mrsmith.notification_delivery_attempt`

It also seeds `rda_approval_requested` with portal delivery enabled and email reminder steps:

- `unread_after_4h`
- `unread_after_24h`
- `unread_after_72h`

Implementation code must match that schema instead of inventing a new table shape.

### 5.3 User APIs

Expose routes under `/api/notifications/v1`, after the existing `/api` prefix is stripped by the server:

- `GET /notifications/v1/summary`
  - returns total unread count and unread counts per app.

- `GET /notifications/v1/items?status=unread|all&app_id=&limit=&cursor=`
  - returns only the current user's recipient rows;
  - default `status=all`;
  - default `limit=30`;
  - stable order by `created_at desc, id desc`;
  - cursor must be stable across pages.

- `POST /notifications/v1/items/{id}/read`
  - marks the current user's recipient as read.

- `POST /notifications/v1/items/read-all`
  - marks all current-user, non-archived, non-resolved notifications as read.

- `POST /notifications/v1/items/{id}/archive`
  - archives only the current user's recipient.

Do not expose admin policy endpoints in V1.

### 5.4 Delivery Worker

The worker must:

- start only when `NOTIFICATIONS_WORKER_ENABLED` is not `false`;
- use `NOTIFICATIONS_WORKER_INTERVAL`, default `60s`;
- select due `pending` deliveries with `due_at <= now()`;
- use transactional locking, preferably `FOR UPDATE SKIP LOCKED`;
- skip delivery if the recipient has `read_at`, `archived_at`, or `resolved_at`;
- send email only when SMTP and `MRSMITH_PUBLIC_BASE_URL` are configured;
- append every delivery attempt to `notification_delivery_attempt`;
- never resend a step already marked `sent`, `skipped`, `failed`, or `cancelled`;
- use bounded retry for temporary SMTP errors without duplicating a policy step.

### 5.5 RDA Pilot

Integrate `backend/internal/rda` without coupling RDA to notification tables:

- extend `rda.Deps` with an optional `notifications.Notifier`;
- pass the notifier from `backend/cmd/server/main.go`;
- after successful upstream `POST /rda/v1/pos/{id}/submit`, fetch the updated PO and notify current-level approvers;
- never create notifications when the upstream RDA call fails or returns non-2xx;
- resolve obsolete notifications after successful approval/rejection and other workflow transitions.

RDA notification contract:

- `type_key = 'rda_approval_requested'`
- `entity_type = 'rda_po'`
- `entity_id = '{poID}'`
- dedupe key: `rda:po:{poID}:approval:{approvalLevel}:requested`
- deep link: RDA PO detail path, with absolute URL in email via `MRSMITH_PUBLIC_BASE_URL`.

For transitions such as approve/reject, payment method approval, leasing approval, no-leasing approval, and budget increment approval/rejection:

- resolve notifications tied to the PO and stale workflow step;
- if the PO remains in an approval state and the active level changes, create the next-level notification.

## 6. Portal UI

Update `apps/portal` and only use `packages/ui` when a component is genuinely reusable.

Required UI behavior:

- unread badge in the Matrix launcher header;
- compact notification dropdown;
- unread/read visual state;
- click notification: mark as read and navigate to the deep link;
- "mark all as read" action;
- lightweight polling only while the document is visible;
- immediate refresh after read/archive actions;
- graceful empty and error states.

The portal UI must remain Matrix-themed. Do not drift into the clean mini-app visual language.

## 7. Execution Slices

### S0 - Preflight and Repo Fit

Executor subagent: `Repo Fit Engineer`

Objective:
- confirm repo state, implementation anchors, and schema-contract fit.

Owned scope:
- no code changes except documenting verified blockers if necessary.

Tasks:
- read all required documents and code areas;
- confirm `docs/NOTIFICHE-V1.md` is treated as the execution contract;
- inspect git status and report pre-existing changes;
- verify implementation will target the schema defined by `deploy/migrations/006_anisetta_mrsmith_notifications.sql`;
- identify likely file ownership conflicts for later slices.

QA subagent: `Repo Fit QA`

QA checklist:
- required sources were read;
- schema references match `deploy/migrations/006_anisetta_mrsmith_notifications.sql`;
- no unrelated file scope is planned;
- env/config strategy fits existing repo conventions.

Gate PASS:
- repo-fit and schema-contract fit are confirmed.

### S1 - Store and SQL Contract Implementation

Executor subagent: `Data Backend Engineer`

Owned files/modules:
- `backend/internal/notifications/store.go`
- `backend/internal/notifications/types.go`
- store tests.

Tasks:
- implement SQL access matching `006_anisetta_mrsmith_notifications.sql`;
- implement idempotent notification creation using `dedupe_key`;
- insert recipients idempotently by normalized email;
- create delivery rows from policy steps idempotently;
- implement summary query;
- implement paged list query;
- implement mark-read, read-all, archive;
- implement resolve by entity and/or dedupe scope;
- implement delivery claiming with transactional locking.

Acceptance criteria:
- no runtime SQL references a table/column absent from the schema contract;
- duplicate `Notify` calls do not create duplicate notification, recipient, or delivery step rows;
- all recipient-facing reads are scoped by normalized caller email.

QA subagent: `Data Integrity QA`

QA checklist:
- schema/code names match the existing schema contract exactly;
- transactions cannot leave partial notifications;
- query filters enforce recipient ownership;
- indexes support unread summary, list, and pending delivery lookup;
- idempotency is tested.

Gate PASS:
- data layer is correct, idempotent, and repo-fit.

### S2 - Notification Service, Policy, Email, and Worker

Executor subagent: `Notifications Backend Engineer`

Owned files/modules:
- `backend/internal/notifications/service.go`
- `backend/internal/notifications/policy.go`
- `backend/internal/notifications/email.go`
- `backend/internal/notifications/worker.go`

Tasks:
- define `NotifyInput`, `NotifyResult`, `Recipient`, `PolicyOverride`, and `ResolveInput`;
- merge `notification_type.default_policy` with per-notification `policy_override`;
- schedule `portal` and `email` delivery steps;
- render plain text and HTML email;
- include absolute links based on `MRSMITH_PUBLIC_BASE_URL`;
- implement delivery worker lifecycle with context cancellation;
- record skipped delivery when SMTP/base URL is unavailable;
- record every attempt in `notification_delivery_attempt`;
- log with `component=notifications`.

Acceptance criteria:
- `4h`, `24h`, and `72h` reminder steps are scheduled for the RDA default policy;
- read/archive/resolve prevents future email sends;
- SMTP disabled does not break portal notifications;
- worker shutdown is graceful.

QA subagent: `Delivery Reliability QA`

QA checklist:
- multiple reminders do not duplicate sends;
- stop conditions are enforced before sending;
- retries are bounded and auditable;
- missing SMTP/base URL produces `skipped`, not crashes;
- logs are sufficient to diagnose delivery failures.

Gate PASS:
- delivery is reliable, auditable, and non-spamming.

### S3 - Notifications API and Runtime Wiring

Executor subagent: `API Backend Engineer`

Owned files/modules:
- `backend/internal/notifications/handler.go`
- `backend/cmd/server/main.go`
- `backend/internal/platform/config/config.go`
- `backend/.env.example`
- `.env.preprod.example`
- deployment config if required by existing conventions.

Tasks:
- register `/notifications/v1/...` routes;
- use existing auth claims from context;
- normalize caller email and reject empty/invalid email where needed;
- sanitize 5xx responses while logging details;
- wire `ANISETTA_DSN` store dependency;
- wire SMTP mailer dependency;
- start the worker according to config;
- add `MRSMITH_PUBLIC_BASE_URL`;
- add `NOTIFICATIONS_WORKER_ENABLED`;
- add `NOTIFICATIONS_WORKER_INTERVAL`.

Acceptance criteria:
- backend starts when Anisetta is absent and returns clear 503 for notification APIs;
- backend starts when SMTP is disabled;
- no sensitive config is exposed through `/config`;
- API paths match the contract.

QA subagent: `API Security QA`

QA checklist:
- users cannot read or mutate other recipients;
- unauthenticated requests are rejected by existing middleware;
- internal errors are logged but sanitized;
- no SMTP credentials, tokens, or secrets are returned to the browser;
- config/env examples are complete.

Gate PASS:
- API is secure, wired, and operationally safe.

### S4 - RDA Producer Integration

Executor subagent: `RDA Domain Engineer`

Owned files/modules:
- `backend/internal/rda/types.go`
- `backend/internal/rda/handler.go`
- `backend/internal/rda/validation.go`
- RDA notification helper file if useful.

Tasks:
- extend `rda.Deps` with an optional notifier;
- preserve existing RDA behavior and permissions;
- create notification only after successful submit;
- fetch updated PO before resolving recipients;
- identify approvers using existing `approvers[]` and `current_approval_level` behavior;
- resolve obsolete notifications after successful workflow transitions;
- avoid hard failures when notifier is unavailable; log warning and preserve RDA success response.

Acceptance criteria:
- submit failure creates no notification;
- successful submit notifies only current-level approvers;
- approval/rejection resolves stale notifications;
- duplicate browser retries do not duplicate notifications;
- existing RDA tests still pass.

QA subagent: `RDA Workflow QA`

QA checklist:
- no RDA permission regression;
- no upstream failure causes notification side effects;
- dedupe key matches the locked contract;
- current-level approver filtering is preserved;
- notifier failure does not break the main RDA transaction after upstream success.

Gate PASS:
- RDA pilot is correct and non-regressive.

### S5 - Portal UI

Executor subagent: `Portal Frontend Engineer`

Owned files/modules:
- `apps/portal/src/`
- `packages/ui` only for genuinely reusable UI primitives.

Tasks:
- add a notifications API client in the portal;
- fetch summary/list after authentication bootstrap;
- poll summary/list only when the document is visible;
- add unread badge;
- add dropdown list;
- implement mark-read and read-all actions;
- navigate to deep links after successful mark-read;
- handle empty, loading, and error states without tutorial copy;
- preserve Matrix theme and responsive layout.

Acceptance criteria:
- badge count updates correctly;
- dropdown is usable on desktop and mobile;
- no layout overlap;
- deep links work;
- polling stops when the tab is not visible.

QA subagent: `Portal UI QA`

QA checklist:
- visual fit with `docs/UI-UX.md` Matrix portal guidance;
- desktop screenshot reviewed;
- mobile screenshot reviewed;
- no one-note palette drift;
- no text overflow or overlapping controls;
- frontend build/typecheck passes.

Gate PASS:
- portal UI is polished, responsive, and theme-consistent.

### S6 - Integration, Documentation, and Hardening

Executor subagent: `Integration Engineer`

Owned scope:
- cross-slice integration;
- documentation updates;
- final verification commands.

Tasks:
- update `docs/IMPLEMENTATION-KNOWLEDGE.md` if reusable rules are discovered;
- verify env examples and deployment config;
- run backend tests;
- run frontend build/typecheck;
- collect QA evidence and residual risks.

QA subagent: `End-to-End QA`

QA checklist:
- RDA submit creates portal notification;
- unread badge updates;
- due email reminder is sent or skipped audibly according to config;
- read/archive/resolve stops future reminders;
- backend behaves safely with Anisetta absent;
- logging is sufficient for delivery diagnosis.

Gate PASS:
- all slices integrate into a working V1.

## 8. Sequencing and Parallelism

Critical path:

1. S0 - Preflight and Repo Fit
2. S1 - Store and SQL Contract Implementation
3. S2 - Notification Service, Policy, Email, and Worker
4. S3 - Notifications API and Runtime Wiring
5. S4 - RDA Producer Integration
6. S5 - Portal UI
7. S6 - Integration, Documentation, and Hardening

Parallelism rules:

- S2 and S3 may proceed in parallel after S1 defines the stable service/store contract.
- S5 may begin against a mocked API contract after S3 route shapes are fixed, but its QA cannot pass before the real API exists.
- S4 must wait for the `Notifier` interface.
- Do not assign two agents to edit the same files concurrently.

## 9. Mandatory QA Protocol

Every QA subagent must report in this format:

```text
QA RESULT: PASS | FAIL | BLOCKED
Scope reviewed:
Checks executed:
Findings:
Required fixes:
Residual risks:
Evidence:
```

Meaning:

- `PASS`: all required checks passed.
- `FAIL`: fixable issues exist; the Orchestrator must assign fixes and rerun the same QA gate.
- `BLOCKED`: an environment, secret, dependency, or operator action is missing. The Orchestrator must find a safe fallback or keep the gate blocked.

The Orchestrator must maintain this gate matrix:

```text
S0 Repo Fit QA              PASS/FAIL/BLOCKED
S1 Data Integrity QA        PASS/FAIL/BLOCKED
S2 Delivery Reliability QA  PASS/FAIL/BLOCKED
S3 API Security QA          PASS/FAIL/BLOCKED
S4 RDA Workflow QA          PASS/FAIL/BLOCKED
S5 Portal UI QA             PASS/FAIL/BLOCKED
S6 End-to-End QA            PASS/FAIL/BLOCKED
Final QA                    PASS/FAIL/BLOCKED
```

The final user-facing completion response is forbidden until every row is `PASS`.

## 10. Iteration Rules

For each slice:

1. Executor implements only its owned scope.
2. Executor runs local checks.
3. QA subagent reviews code, tests, repo-fit, and this contract.
4. If QA returns `FAIL`, the Orchestrator converts findings into concrete fix tasks.
5. Executor applies fixes.
6. The same QA role reruns the review.
7. Repeat until `PASS`.

For final QA:

1. Spawn a final QA subagent that did not implement any slice.
2. Re-read this document.
3. Compare implementation, tests, UI, env, and docs against every locked decision.
4. Verify or rerun command evidence.
5. Produce the final QA report.

## 11. Minimum Verification Commands and Checks

Backend:

- `go test ./...`, or at minimum all impacted backend packages if the full suite is impractical.
- targeted tests for `backend/internal/notifications`.
- impacted RDA tests.
- backend startup checks with:
  - Anisetta configured;
  - Anisetta absent;
  - SMTP disabled.

Frontend:

- `pnpm --filter mrsmith-portal build`, or the package's current build command.
- `pnpm --filter mrsmith-portal lint` when available.
- manual portal UI check.

Browser/visual:

- before Playwright or browser checks, first verify whether `make dev` or a suitable Vite dev server is already running and reuse it;
- capture desktop and mobile screenshots for:
  - zero notifications;
  - unread notifications;
  - dropdown open;
  - API error state.

Data:

- SQL code matches the already-applied schema contract;
- unread/list queries use appropriate indexes;
- delivery worker cannot process the same step twice.

Security:

- every recipient query filters by normalized caller email;
- no endpoint trusts client-provided recipient email for read/mutate operations;
- no token, SMTP secret, or sensitive config appears in metadata, email, logs, or responses.

## 12. Definition of Done Per Slice

A slice is done only when:

- implementation is complete within assigned scope;
- relevant tests are added and executed;
- no known regression remains in available commands;
- the slice QA subagent reports `PASS`;
- residual risks are explicit and acceptable;
- no unrelated changes were introduced.

## 13. Global Definition of Done

The project is done only when:

- S0-S6 QA gates are all `PASS`;
- Final QA is `PASS`;
- notification APIs work with Keycloak/dev auth;
- RDA pilot creates and resolves notifications correctly;
- multiple email reminders respect policy, overrides, idempotency, and stop conditions;
- portal UI shows badge/dropdown and navigates deep links without visual regressions;
- env examples and deployment notes are complete;
- reusable discoveries are documented in `docs/IMPLEMENTATION-KNOWLEDGE.md` when applicable;
- final report includes commands run, results, QA matrix, visual evidence, and residual risks.

## 14. Risk Register

- Duplicate emails: mitigate with unique `(recipient_id, channel, policy_step)` and idempotency tests.
- Notification created before upstream RDA confirmation: create only after upstream 2xx.
- RDA state changes before a reminder is due: worker must re-check read/archive/resolve state; RDA transitions must resolve stale recipients.
- Bad email base URL: require `MRSMITH_PUBLIC_BASE_URL`; skip email delivery with audit when missing.
- Portal Matrix UI drift: require Portal UI QA with screenshots.
- Anisetta absent locally: APIs return clear 503; backend must not crash.
- SMTP disabled: portal notifications continue; email deliveries are skipped audibly.

## 15. Final Report Requirements

The Orchestrator's final response must include:

- implementation summary;
- main files changed;
- QA matrix with every gate marked `PASS`;
- commands executed and results;
- screenshot or visual QA notes;
- residual risks;
- operational notes for configuring `MRSMITH_PUBLIC_BASE_URL`, SMTP, and worker env vars.

If any control does not pass, do not produce the final report. Continue the fix and QA iteration loop.
