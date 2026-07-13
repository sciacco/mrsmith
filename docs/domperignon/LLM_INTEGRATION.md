# DomPerignon — External Integration Guide (machine/LLM-oriented)

> Audience: external applications and LLM agents consuming the DomPerignon API
> over backend-to-backend API-key authentication. This document is optimized
> for machine consumption: explicit rules, exact endpoints, no implied context.
>
> Canonical contract: [`docs/openapi.json`](./openapi.json) (OpenAPI 3).
> Where this document and the OpenAPI spec disagree, the spec wins.

## 1. What DomPerignon is

DomPerignon is a domain-portfolio management system: it manages domain
registrations, renewals, transfers, DNS, and registrant contacts across
multiple registrars (NIC.it, EURid, NameSilo, ClouDNS, manual registrars),
with a job queue for registrar operations, audit logging, and reconciliation
against registrar state.

## 2. Base URL and protocol

- All endpoints are under `https://<host>/api/v1`. Paths in this document omit that prefix.
- Requests and responses are JSON: send `Content-Type: application/json`.
- All IDs are UUIDs unless stated otherwise. Timestamps are RFC 3339 / ISO 8601 UTC.

## 3. Authentication (API key)

External applications MUST authenticate with an API key, not the login flow.

```http
GET /api/v1/domains HTTP/1.1
X-API-Key: dpak_<key_id>_<secret>
```

Rules:

1. Send the key in the `X-API-Key` header on every request. There is no session.
2. NEVER send both `X-API-Key` and `Authorization` — the request is rejected with `401`.
3. NEVER call `/auth/login`, `/auth/refresh`, or `/auth/logout`, and ignore refresh cookies. Those exist for the human web UI only.
4. Any invalid key condition (malformed, unknown, revoked, expired, wrong secret, disabled account) returns `401` with no distinguishing detail. Do not retry a `401` — the credential is bad; alert your operator.
5. Keys are issued by a DomPerignon admin, are shown exactly once at creation, and are bound to a service account whose role (see §4) is your permission ceiling.
6. `GET /auth/me` works with an API key and returns your identity and role — useful as a startup self-check.

## 4. Authorization (RBAC)

Your API key inherits the role of its service account. There are no per-key scopes.

| Role | Can do |
|------|--------|
| `viewer` | All read (GET) endpoints |
| `operator` | viewer + domain/contact/DNS/transfer mutations, batch operations |
| `admin` | everything, including settings, registrars, users, API keys |

The general rule is: **read routes accept viewer; mutation routes require
operator or admin; configuration routes require admin.** A request with an
insufficient role returns `403` with code `FORBIDDEN`. Do not retry `403`s.

## 5. Response envelopes

Success (single object or list):

```json
{ "data": { ... } }
{ "data": [ ... ], "meta": { "page": 1, "per_page": 50, "total": 1234 } }
```

Error (always this shape, any non-2xx):

```json
{ "error": { "code": "DOMAIN_NOT_FOUND", "message": "Domain example.it not found" } }
```

Always branch on `error.code` (stable, machine-readable), not on `error.message`.

## 6. Pagination, sorting, filtering

- Pagination: `?page=1&per_page=50` (list endpoints). Read `meta.total` to know when to stop.
- Sorting where supported: `?sort=expires_at&order=asc`.
- Filtering where supported: e.g. `?status=active&tld=it&q=searchterm&customer_ref=12345`.
- Consult the OpenAPI spec for the exact parameters of each endpoint; do not invent parameters.

## 7. Correlation and idempotency

- Send an `X-Correlation-ID` header (any unique string) on every request. It is
  echoed in the response and recorded in logs; include it when reporting problems.
- Some mutation endpoints REQUIRE an `Idempotency-Key` header and return
  `400 IDEMPOTENCY_KEY_REQUIRED` without it (currently: transfer tracking
  start endpoints and destructive transfer actions such as
  `/domains/{id}/transfer-out/start`, `/domains/{id}/transfer-in/start`,
  `/transfers/{id}/actions/archive-domain`; some provider mutations such as
  ClouDNS SSL orders also accept it).
- Recommended policy for an agent: generate a fresh UUID `Idempotency-Key` for
  every risky mutation and reuse the SAME key when retrying that same logical
  action after a network failure. Replaying with the same key returns the
  original result instead of duplicating the action; a same-key replay with a
  different body returns `409 IDEMPOTENCY_CONFLICT`.

## 8. The async operation model (critical)

Most registrar-affecting mutations do NOT complete synchronously. They enqueue
an operation and return `202` with:

```json
{ "data": { "operation_id": "<uuid>" } }
```

Contract for consumers:

1. A `202` means "accepted", not "done". The domain state changes only when the operation completes.
2. Poll `GET /operations/{id}` to track it. Status lifecycle: `queued` → `processing` → `completed` | `failed` | `pending_manual` (waits for human action) | `cancelled`.
3. Poll with backoff (e.g. every 5–15 s); operations normally complete within seconds to minutes. Do not poll faster than once per second.
4. On `failed`, read `error_message`. `POST /operations/{id}/retry` re-queues a failed operation; `POST /operations/{id}/cancel` cancels a queued one.
5. Batch endpoints return a `batch_id`; track with `GET /batches/{batch_id}` (completed/failed/pending counts).
6. Synchronous exceptions that do NOT queue an operation: `POST /domains/check` (availability) and `POST /domains/{id}/transfer-out` (auth-code retrieval) return their result directly.

## 9. Hard rules and invariants (do not violate)

1. **EURid (.eu) registrations**: `POST /domains/register` for `.eu` REQUIRES `nsgroups` and REJECTS direct `nameservers`. Manage NSGroups via `/settings/registrars/{registrar_id}/nsgroups` (admin).
2. **Manual inbound transfer tracking** (`POST /domains/{id}/transfer-in/start`) is tracking-only: it records state and never triggers a registrar transfer. To actually transfer a domain in, use `POST /domains/transfer-in` or `POST /domains/{id}/transfer`.
3. **Contact-to-customer scoping**: domain contact assignment is customer-scoped. When strict scope validation is enabled, assigning a contact not linked to the domain's customer returns `409 CONTACT_NOT_ASSIGNED_TO_CUSTOMER`. Fix the association (`PUT /contacts/{id}/customers`) instead of retrying.
4. **NameSilo prices**: `GET /providers/namesilo/prices` exposes `registration_domains` only. Do not expect retail price fields.
5. **Confirmation-gated actions**: some destructive/expensive actions return `400 CONFIRMATION_REQUIRED` unless the request body carries the documented confirmation flag. Never set a confirmation flag unless your own caller explicitly confirmed the action.
6. **Domain locks**: mutations on a locked domain return `409/400 DOMAIN_LOCKED`. Unlock first (`POST /domains/{id}/unlock`, queued) only if that is genuinely intended.
7. Respect `pending_manual`: it means a human must act. Do not retry or work around it.

## 10. Error code reference (most common)

| HTTP | `error.code` | Meaning / agent action |
|------|--------------|------------------------|
| 400 | `INVALID_REQUEST`, `INVALID_PARAM`, `INVALID_ID` | Malformed input — fix the request, do not retry as-is |
| 400 | `IDEMPOTENCY_KEY_REQUIRED` | Add an `Idempotency-Key` header |
| 400 | `CONFIRMATION_REQUIRED` | Action needs explicit confirmation flag (see §9.5) |
| 401 | `UNAUTHORIZED` | Bad/missing credential — do not retry, alert operator |
| 403 | `FORBIDDEN` | Role insufficient — do not retry |
| 404 | `NOT_FOUND` | Entity does not exist |
| 409 | `CONFLICT`, `DUPLICATE` | Already exists / concurrent change |
| 409 | `CONTACT_NOT_ASSIGNED_TO_CUSTOMER` | See §9.3 |
| 409 | `IDEMPOTENCY_CONFLICT` | Same key, different body — bug in caller |
| 409 | `TRANSFER_ALREADY_ACTIVE` | A transfer run already exists for the domain |
| 422/400 | `DOMAIN_LOCKED`, `INVALID_STATE`, `INVALID_STATUS_TRANSITION` | Entity state does not allow the action |
| 429 | `RATE_LIMITED` | Back off and retry later |
| 502/503 | `REGISTRY_ERROR`, `REGISTRY_UNAVAILABLE`, `NAMESILO_UNAVAILABLE`, `HOSTING_SOLUTIONS_UNAVAILABLE` | Upstream registrar issue — retry with backoff |
| 500 | `INTERNAL_ERROR` | Server-side failure — retry once with backoff, then report with correlation ID |

## 11. Common recipes

### Check availability, then register (synchronous check + async register)

```
POST /domains/check           {"domains": ["example.it", "example.com"]}   → per-TLD results
POST /domains/register        {domain, registrar?, contacts, nameservers|nsgroups, years, customer_ref}
                              → 202 {operation_id}
GET  /operations/{id}         → poll until completed/failed
GET  /domains?q=example.it    → confirm the domain appears with expected status
```

### Renew

```
GET  /domains?q=<name>                     → find domain id, check expires_at
POST /domains/{id}/renew {"years": 1}      → 202 {operation_id} → poll
```

### Read-only portfolio sync (viewer key is enough)

```
GET /domains?page=N&per_page=100           → iterate until page*per_page >= meta.total
GET /dashboard/expiring                    → 30/60/90-day expiry buckets
GET /operations?status=failed              → surface failures to your side
```

### Transfer a domain in

```
GET  /providers/namesilo/transfers/availability?domains=<name>  → eligibility (NameSilo)
POST /domains/transfer-in {domain, auth_code, contacts, ...}    → 202 → poll operation
GET  /transfers                                                → transfer runs and their statuses
```

## 12. Operational etiquette for agents

- Cache stable reference data (registrars, extensions, capabilities) for minutes, not seconds.
- `GET /capabilities` tells you which optional features this deployment enables — check it at startup instead of probing endpoints.
- Use list filters instead of downloading whole collections repeatedly.
- Treat every mutation as audited: your service-account identity and API key name are recorded with each action.
- On repeated `5xx`/`REGISTRY_*` errors, stop and surface the problem; do not hammer the queue with retries of registrar operations.

## 13. Endpoint catalog

Grouped by area. `(admin)` in the description marks admin-only endpoints;
otherwise the §4 read/mutation rule applies. Exact request/response schemas,
query parameters, and status codes are in [`docs/openapi.json`](./openapi.json).

<!-- BEGIN GENERATED CATALOG — do not edit below; regenerate with `make llm-doc` -->

### api-keys

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api-keys` | List API keys (admin) |
| POST | `/api-keys` | Create API key for a service account (admin) |
| POST | `/api-keys/{id}/revoke` | Revoke API key (admin, idempotent) |

### Auth

| Method | Path | Description |
|--------|------|-------------|
| GET | `/auth/me` | Get current authenticated user |
| POST | `/auth/login` | Login with identifier and password |
| POST | `/auth/logout` | Logout |
| POST | `/auth/refresh` | Refresh access token |

### Capabilities

| Method | Path | Description |
|--------|------|-------------|
| GET | `/capabilities` | Get deployment feature capabilities |

### ClouDNSProvider

| Method | Path | Description |
|--------|------|-------------|
| DELETE | `/providers/cloudns/zones/{domain_name}/records/{record_id}` | Delete ClouDNS zone record |
| GET | `/providers/cloudns/records/types` | Get available record types for a zone type |
| GET | `/providers/cloudns/ssl` | List ClouDNS SSL certificates |
| GET | `/providers/cloudns/status` | Get ClouDNS provider status |
| GET | `/providers/cloudns/zones` | List ClouDNS zones |
| GET | `/providers/cloudns/zones/{domain_name}/records` | List ClouDNS zone records |
| GET | `/providers/cloudns/zones/{domain_name}/records/export` | Export ClouDNS zone records |
| GET | `/providers/cloudns/zones/{domain_name}/records/ttl` | Get available TTL values for zone |
| POST | `/providers/cloudns/ssl/orders` | Order ClouDNS SSL certificate |
| POST | `/providers/cloudns/zones` | Create ClouDNS zone |
| POST | `/providers/cloudns/zones/{domain_name}/records` | Create ClouDNS zone record |
| POST | `/providers/cloudns/zones/{domain_name}/records/{record_id}/status` | Change ClouDNS record status |
| POST | `/providers/cloudns/zones/{domain_name}/records/copy` | Copy ClouDNS records from another zone |
| POST | `/providers/cloudns/zones/{domain_name}/records/import` | Import ClouDNS zone records |
| POST | `/providers/cloudns/zones/{domain_name}/records/import-transfer` | Import ClouDNS records via transfer |
| PUT | `/providers/cloudns/zones/{domain_name}/records/{record_id}` | Modify ClouDNS zone record |

### Contacts

| Method | Path | Description |
|--------|------|-------------|
| DELETE | `/contacts/{id}` | Queue contact_delete operation |
| GET | `/contacts` | List contacts |
| GET | `/contacts/{id}` | Get contact detail and associated domains |
| POST | `/contacts` | Create contact and queue contact_create operation |
| POST | `/contacts/batch/customers` | Batch update contact customer associations |
| PUT | `/contacts/{id}` | Queue contact update operation |
| PUT | `/contacts/{id}/customers` | Replace contact customer associations |

### Customers

| Method | Path | Description |
|--------|------|-------------|
| GET | `/customers/{id}` | Get CRM/EDP customer by id |
| GET | `/customers/search` | Search CRM/EDP customers by company name |

### Dashboard

| Method | Path | Description |
|--------|------|-------------|
| GET | `/dashboard/expiring` | Domains expiring within 30/60/90 days |
| GET | `/dashboard/operations` | In-progress and recent operations |
| GET | `/dashboard/pending` | Get pending operations summary |
| GET | `/dashboard/stats` | Dashboard domain counters |

### Domains

| Method | Path | Description |
|--------|------|-------------|
| DELETE | `/domains/{id}/asset` | Unlink local asset reference from domain |
| DELETE | `/domains/{id}/dns/records/{record_id}` | Delete DNS record |
| GET | `/domains` | List domains |
| GET | `/domains/{id}` | Get domain detail |
| GET | `/domains/{id}/asset` | Get linked asset detail |
| GET | `/domains/{id}/asset/candidates` | Fetch asset candidates by domain name |
| GET | `/domains/{id}/dns/records` | List DNS records |
| GET | `/domains/manual-imports/{batch_id}` | Get manual import batch detail |
| POST | `/domains/{id}/auth-code` | Retrieve domain auth code |
| POST | `/domains/{id}/auto-renew/disable` | Queue auto-renew disable operation |
| POST | `/domains/{id}/auto-renew/enable` | Queue auto-renew enable operation |
| POST | `/domains/{id}/dns/import` | Queue DNS import operation |
| POST | `/domains/{id}/dns/records` | Create DNS record |
| POST | `/domains/{id}/lock` | Queue lock operation |
| POST | `/domains/{id}/privacy/disable` | Queue privacy disable operation |
| POST | `/domains/{id}/privacy/enable` | Queue privacy enable operation |
| POST | `/domains/{id}/reconduct` | Start reconduction for a relaxed/manual domain |
| POST | `/domains/{id}/renew` | Queue renewal operation |
| POST | `/domains/{id}/sync` | Synchronize domain data from registrar |
| POST | `/domains/{id}/transfer` | Queue transfer-in operation |
| POST | `/domains/{id}/transfer-in/start` | Start inbound transfer tracking manually |
| POST | `/domains/{id}/transfer-out` | Retrieve transfer-out auth code synchronously |
| POST | `/domains/{id}/transfer-out/start` | Start outbound transfer tracking |
| POST | `/domains/{id}/unlock` | Queue unlock operation |
| POST | `/domains/batch/contacts` | Batch update domain contacts |
| POST | `/domains/batch/renew` | Batch renew domains |
| POST | `/domains/check` | Check domain availability synchronously |
| POST | `/domains/import` | Import an existing domain into the portfolio |
| POST | `/domains/manual-imports` | Create a relaxed/manual import batch |
| POST | `/domains/register` | Queue domain registration operation |
| POST | `/domains/transfer-in` | Queue transfer-in operation |
| PUT | `/domains/{id}` | Update domain metadata |
| PUT | `/domains/{id}/asset` | Save asset and link to domain |
| PUT | `/domains/{id}/asset/link` | Link existing asset to domain |
| PUT | `/domains/{id}/contacts` | Queue domain contact update operation |
| PUT | `/domains/{id}/dns/records/{record_id}` | Update DNS record |
| PUT | `/domains/{id}/ns` | Queue nameserver update operation |

### Extensions

| Method | Path | Description |
|--------|------|-------------|
| GET | `/extensions/lookup` | Look up registrars for a domain extension |

### Health

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |

### HostingSolutions

| Method | Path | Description |
|--------|------|-------------|
| GET | `/providers/hosting-solutions/catalog` | Get Hosting Solutions catalog |
| GET | `/providers/hosting-solutions/search` | Search Hosting Solutions services (empty filters allowed) |
| GET | `/providers/hosting-solutions/status` | Get Hosting Solutions cache status |

### NameSiloProvider

| Method | Path | Description |
|--------|------|-------------|
| DELETE | `/providers/namesilo/dnssec/records` | Delete DNSSEC record |
| DELETE | `/providers/namesilo/domain-forward/subdomain` | Delete subdomain forwarding |
| DELETE | `/providers/namesilo/email-forwards` | Delete email forward |
| DELETE | `/providers/namesilo/portfolios/{portfolio}` | Delete portfolio |
| DELETE | `/providers/namesilo/registered-nameservers` | Delete registered nameserver |
| GET | `/providers/namesilo/dnssec/records` | List DNSSEC records |
| GET | `/providers/namesilo/domains/info` | Get NameSilo domain info |
| GET | `/providers/namesilo/email-forwards` | List email forwards |
| GET | `/providers/namesilo/expiring-domains` | List NameSilo expiring domains |
| GET | `/providers/namesilo/expiring-domains/count` | Count NameSilo expiring domains |
| GET | `/providers/namesilo/marketplace/active-sales` | List marketplace active sales |
| GET | `/providers/namesilo/orders` | List NameSilo orders |
| GET | `/providers/namesilo/orders/{order_number}` | Get NameSilo order detail |
| GET | `/providers/namesilo/payment-methods` | List NameSilo payment methods (admin only) |
| GET | `/providers/namesilo/portfolios` | List NameSilo portfolios |
| GET | `/providers/namesilo/prices` | Get NameSilo TLD prices |
| GET | `/providers/namesilo/registered-nameservers` | List registered nameservers |
| GET | `/providers/namesilo/registrant-verification-status` | Get registrant verification status |
| GET | `/providers/namesilo/status` | Get NameSilo provider status |
| GET | `/providers/namesilo/transfers/availability` | Check transfer availability |
| GET | `/providers/namesilo/whois-info` | Get WHOIS info |
| POST | `/providers/namesilo/account/funds` | Add account funds (admin only) |
| POST | `/providers/namesilo/dnssec/records` | Add DNSSEC record |
| POST | `/providers/namesilo/domain-forward` | Configure domain forwarding |
| POST | `/providers/namesilo/domain-forward/subdomain` | Configure subdomain forwarding |
| POST | `/providers/namesilo/domain-push` | Push domain(s) to another account |
| POST | `/providers/namesilo/email-forwards` | Configure email forward |
| POST | `/providers/namesilo/email-verification` | Send registrant email verification |
| POST | `/providers/namesilo/marketplace/landing-page` | Update marketplace landing page |
| POST | `/providers/namesilo/marketplace/sales` | Add or modify marketplace sale |
| POST | `/providers/namesilo/portfolios` | Create portfolio |
| POST | `/providers/namesilo/portfolios/associate-domains` | Associate domains to portfolio |
| POST | `/providers/namesilo/register-domain-drop` | Register dropped domain |
| POST | `/providers/namesilo/registered-nameservers` | Add registered nameserver |
| POST | `/providers/namesilo/transfers/resend-admin-email` | Resend transfer admin email |
| POST | `/providers/namesilo/transfers/resubmit-to-registry` | Resubmit transfer to registry |
| PUT | `/providers/namesilo/payment-methods` | Save NameSilo payment methods (admin only, bulk replace) |
| PUT | `/providers/namesilo/registered-nameservers` | Modify registered nameserver |

### Notifications

| Method | Path | Description |
|--------|------|-------------|
| GET | `/notifications` | List notifications for current user |
| GET | `/notifications/unread-count` | Unread notification count |
| PUT | `/notifications/{id}/read` | Mark a notification as read |
| PUT | `/notifications/read-all` | Mark all current-user notifications as read |

### Operations

| Method | Path | Description |
|--------|------|-------------|
| GET | `/batches/{batch_id}` | Get batch progress |
| GET | `/operations` | List operations |
| GET | `/operations/{id}` | Get operation detail |
| POST | `/operations/{id}/cancel` | Cancel queued operation |
| POST | `/operations/{id}/complete-manual` | Mark a pending_manual operation as completed |
| POST | `/operations/{id}/retry` | Retry failed operation |

### PollEvents

| Method | Path | Description |
|--------|------|-------------|
| GET | `/poll-events` | List poll events |
| GET | `/poll-events/{id}` | Get poll event detail |

### Reconciliation

| Method | Path | Description |
|--------|------|-------------|
| GET | `/reconciliation/discrepancies` | List reconciliation discrepancies |
| GET | `/reconciliation/discrepancies/domain-lines` | List discrepancy lines for a domain |
| GET | `/reconciliation/runs` | List reconciliation runs |
| GET | `/reconciliation/runs/{id}` | Get reconciliation run details |
| GET | `/reconciliation/summary` | Get reconciliation current summary |
| POST | `/reconciliation/discrepancies/{id}/resolve` | Resolve discrepancy |
| POST | `/reconciliation/discrepancies/domain-ignore` | Ignore all open discrepancies for a domain |
| POST | `/reconciliation/discrepancies/domain-sync` | Sync domain from registrar or prefill for import |
| POST | `/reconciliation/runs/trigger` | Trigger reconciliation run |

### Registrars

| Method | Path | Description |
|--------|------|-------------|
| GET | `/registrars/{registrar}/account-balance` | Get registrar account balance |

### Settings

| Method | Path | Description |
|--------|------|-------------|
| DELETE | `/settings/extensions/{id}` | Delete an extension mapping |
| DELETE | `/settings/registrars/{id}` | Delete a registrar |
| DELETE | `/settings/registrars/{registrar_id}/nsgroups/{name}` | Delete an NSGroup |
| GET | `/settings/extensions` | List all extension mappings |
| GET | `/settings/registrars` | List all registrars |
| GET | `/settings/registrars/{registrar_id}/nsgroups` | List NSGroups for a registrar |
| GET | `/settings/registrars/{registrar_id}/nsgroups/{name}` | Get NSGroup live from registrar |
| POST | `/settings/extensions` | Create an extension mapping |
| POST | `/settings/registrars` | Create a registrar |
| POST | `/settings/registrars/{registrar_id}/nsgroups` | Create an NSGroup |
| PUT | `/settings/extensions/{id}` | Update an extension mapping |
| PUT | `/settings/registrars/{id}` | Update a registrar |
| PUT | `/settings/registrars/{registrar_id}/nsgroups/{name}` | Update an NSGroup |

### Transfers

| Method | Path | Description |
|--------|------|-------------|
| GET | `/transfers` | List transfer runs |
| GET | `/transfers/{id}` | Get transfer run detail |
| POST | `/transfers/{id}/actions/archive-domain` | Archive domain from failed transfer |
| POST | `/transfers/{id}/actions/resend-admin-email` | Resend admin approval email |
| POST | `/transfers/{id}/actions/resubmit-registry` | Resubmit transfer to registry |
| POST | `/transfers/{id}/actions/unlock-domain` | Unlock domain for transfer |

### Users

| Method | Path | Description |
|--------|------|-------------|
| GET | `/users` | List users (admin) |
| POST | `/users` | Create user (admin) |
| PUT | `/users/{id}` | Update user (admin) |
