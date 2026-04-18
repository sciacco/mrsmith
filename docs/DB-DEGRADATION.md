# Database Degradation and Background Reconnect — Feature Spec

Sources: `backend/cmd/server/main.go`, `backend/internal/platform/database/database.go`, `backend/internal/platform/health/health.go`, `docs/IMPLEMENTATION-PLANNING.md`
Date: 2026-04-18
Status: Draft — proposed backend runtime change
Execution target: This document is written for direct implementation in the current repo.

---

## Problem

Today the backend treats every configured database connection failure during startup as fatal.

Current behavior:

- `database.New(...)` performs `sql.Open(...)` followed by an immediate `db.Ping()`.
- `backend/cmd/server/main.go` calls `os.Exit(1)` if any configured DB fails that startup connect.
- Many modules already support `nil` database handles and return `503` per route, but that path is only reachable when the DSN is unset, not when the DSN is configured but temporarily unreachable.
- Handlers capture `*sql.DB` values at route-registration time, so a startup failure cannot recover later without restarting the process.

Operational consequence:

- a scheduled restart can turn a transient DB outage into a full backend outage
- one app-local dependency such as `WHMCS_DSN` or `VODKA_DSN` can take down unrelated routes
- `/readyz` currently always returns `200` and does not describe dependency state

This feature changes the DB startup model from `connect-or-exit` to `serve-and-degrade`, with background reconnect for unavailable DBs.

---

## Goals

- No database connection failure should terminate the whole backend process after auth initialization succeeds.
- Treat `DSN unset` and `startup connection failure` as the same runtime availability state: the dependency is unavailable.
- Retry unavailable DB connections in the background with exponential backoff and jitter.
- Allow a dependency that was unavailable at startup to become available later without restarting the process.
- Preserve existing per-route graceful-degradation contracts wherever they already exist.
- Keep health probes stable during transient DB outages so restarts do not create avoidable downtime.
- Add explicit observability for dependency state without exposing DSNs or secrets.

## Non-Goals

- Dynamic portal tile hiding based on transient DB outages.
- Automatic demotion of an already-connected DB handle when a later query fails at runtime.
- Reworking non-DB dependencies such as HubSpot, Carbone, Arak, or OpenRouter in this change.
- Changing app-specific frontend UX beyond the backend contracts they already consume.
- Changing deployment-time secret resolution. If Kubernetes cannot inject a required secret key, this feature does not help because the container will not start.

---

## Current Repo Constraints

### Startup and Connection Rules

- Shared DB construction already goes through one common helper: `backend/internal/platform/database/database.go` `database.New(...)`.
- The helper should remain the only place that performs DB open + initial ping.
- The current problem is not DB creation duplication. It is fatal startup policy in `main.go`.

### Handler Wiring

- Most modules store `*sql.DB` directly in a handler struct.
- Examples:
  - `backend/internal/compliance/handler.go`
  - `backend/internal/panoramica/handler.go`
  - `backend/internal/quotes/handler.go`
- This means “retry in background” is impossible without an indirection layer. A `nil` captured at startup stays `nil` forever.

### Existing Graceful-Degradation Contracts

- Many handlers already return `503` when a dependency handle is `nil`.
- Examples:
  - `mistra_database_not_configured`
  - `grappa_database_not_configured`
  - `anisetta_database_not_configured`
  - `whmcs_database_not_configured`
  - `vodka_database_not_configured`
- Some modules intentionally use documented fallbacks instead of `503` for specific optional dependencies. The clearest current example is Quotes on some Alyante-backed reference paths. Those endpoint-specific fallbacks must be preserved.

### Health and Deployment

- `deploy/k8s/deployment.yaml` uses `/healthz` for liveness and `/readyz` for readiness.
- `backend/internal/platform/health/health.go` currently returns `200 {"status":"ok"}` for both endpoints and has a TODO for dependency checks.
- The deployment currently runs `replicas: 2`, so keeping readiness stable during transient outages is valuable.

### Portal Catalog

- `backend/cmd/server/main.go` filters app visibility at startup based on DSN presence, not live connectivity.
- This feature will not make launcher visibility depend on transient DB health.

---

## Resolved Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| DBD1 | All DB-backed dependencies use a managed availability wrapper around `database.New(...)`. | Reuses the existing common DB constructor and moves the change to runtime policy, not driver logic. |
| DBD2 | Database connection failure is never process-fatal after auth middleware initializes successfully. | Prevents transient DB issues during restart from becoming full backend outage. |
| DBD3 | `DSN unset` and `startup connect failure` are the same request-time availability state. | Route handlers should not care why the dependency is unavailable. |
| DBD4 | Unavailable DBs retry in the background with exponential backoff and jitter. | Allows automatic recovery after transient startup/network/database outages. |
| DBD5 | Backoff policy is `1s, 2s, 4s, ...` capped at `60s`, with `+-20%` jitter per attempt. | Fast initial recovery without thundering-herd retry loops. |
| DBD6 | Once the HTTP server is up, `/readyz` stays `200` even if one or more DBs are unavailable. | This feature is explicitly meant to avoid restart-triggered downtime from dependency outages. |
| DBD7 | Detailed dependency state is exposed by a new `/dependenciesz` endpoint, not by failing readiness. | Keeps probes stable while preserving observability. |
| DBD8 | Portal tile visibility remains configuration-based, not connectivity-based. | Avoids app list flapping during transient outages; recovery should happen in place. |
| DBD9 | Handlers must fetch DB handles at request time from a provider, not store only a startup-time `*sql.DB`. | Required so background reconnect can actually restore functionality without restart. |
| DBD10 | Runtime demotion of already-connected handles is out of scope for v1. | Solves the restart problem first without building a full DB health-orchestration system. |

---

## Scope of Dependencies

This feature applies to all DB dependencies currently initialized in `backend/cmd/server/main.go`:

| Env Var | Driver | Primary Consumers |
|---------|--------|-------------------|
| `ANISETTA_DSN` | postgres | compliance, rdf, rdfbackend, panoramica, reports |
| `MISTRA_DSN` | postgres | kitproducts, listini, panoramica, quotes, reports, rdf, afctools |
| `DBCOPERTURE_DSN` | postgres | coperture |
| `ALYANTE_DSN` | mssql | quotes, kitproducts adapter, afctools |
| `GRAPPA_DSN` | mysql | listini, panoramica, reports, energiadc, afctools |
| `VODKA_DSN` | mysql | afctools |
| `WHMCS_DSN` | mysql | afctools |

This feature does not change the fatal startup behavior for:

- auth middleware initialization
- HTTP server bind failures
- deployment-time missing required secrets or invalid Kubernetes env injection

---

## Runtime Design

### 1. Managed DB Provider

Create a new platform package:

- `backend/internal/platform/dbdeps/`

Core types:

```go
type Provider interface {
	Current() *sql.DB
	Snapshot() Status
}

type Status struct {
	Name          string
	Driver        string
	Configured    bool
	State         string // disabled | connecting | retrying | ready
	Attempt       int
	LastSuccessAt *time.Time
	LastErrorAt   *time.Time
	NextRetryAt   *time.Time
}

type Config struct {
	Name   string
	Driver string
	DSN    string
	Open   func(database.Config) (*sql.DB, error) // default: database.New
}
```

Concrete implementation requirements:

- Store current `*sql.DB` behind synchronization safe for concurrent reads.
- Keep status metadata separate from the DB handle.
- If `DSN == ""`, mark the dependency as `disabled`, never retry, and always return `nil`.
- If initial connect fails:
  - set state to `retrying`
  - log the failure as non-fatal
  - schedule the next retry
- If a retry succeeds:
  - publish the new `*sql.DB`
  - set state to `ready`
  - reset attempt count
  - log recovery with outage duration and attempt count
- Retry loop ends only on process shutdown.

### 2. Startup Policy

`backend/cmd/server/main.go` must stop calling `os.Exit(1)` for DB connect failures.

Instead:

- build one managed provider per configured DB dependency
- start all providers before route registration
- pass providers into modules
- let the HTTP server start even when some or all DB providers are in `retrying`

Fatal startup stays limited to:

- auth middleware setup failure
- HTTP server startup failure
- explicit shutdown failure handling already present in `main.go`

### 3. Request-Time Behavior

Handlers must resolve the active DB handle per request.

Pattern:

```go
func (h *Handler) requireMistra(w http.ResponseWriter) (*sql.DB, bool) {
	db := h.mistra.Current()
	if db == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "mistra_database_not_configured")
		return nil, false
	}
	return db, true
}
```

Implementation rule:

- replace boolean-only `require*` helpers with `(*sql.DB, bool)` helpers
- use the returned local `db` for the query in that request
- do not read `h.db` or `h.mistraDB` fields directly in handlers after this migration

### 4. Existing Error Contracts

This feature changes availability plumbing, not API semantics.

Rules:

- If a route currently returns a dependency-specific `503`, keep the same body.
- If a route currently uses a documented fallback for an optional dependency, keep the fallback.
- Do not replace app-specific error codes with one new generic DB-unavailable code.

Important example:

- Quotes has Alyante-backed paths where nil Alyante currently falls back to documented defaults instead of `503`.
- That behavior must remain as-is.

### 5. What Background Retry Covers in v1

Background retry is defined only for the `no active DB handle exists` state.

Included:

- DSN configured but DB unavailable during startup
- DB becomes reachable later and should attach without restart

Explicitly out of scope in v1:

- detecting that a previously ready `*sql.DB` should be demoted back to unavailable
- tearing down a live handle because a later query or ping fails

Rationale:

- `sql.DB` already manages its own internal connection pool and reconnect behavior for many transient failures
- the immediate outage risk raised in this discussion is startup-time fatality during restart

---

## Health and Observability

### `/healthz`

Unchanged:

- still returns process liveness only
- still returns `200`

### `/readyz`

New semantics:

- returns `200` once the HTTP server is running and the dependency manager registry is initialized
- does not fail because a DB is unavailable
- returns a JSON summary payload, for example:

```json
{
  "status": "degraded",
  "configured_dependencies": 7,
  "ready_dependencies": 5,
  "retrying_dependencies": 2
}
```

Probe contract:

- Kubernetes continues probing `/readyz`
- readiness remains stable during transient DB outages
- degraded routes surface unavailability in-band as `503` or documented endpoint fallback

### `/dependenciesz`

Add a new unauthenticated internal diagnostics endpoint:

- `GET /dependenciesz`

Response shape:

```json
{
  "dependencies": [
    {
      "name": "whmcs",
      "driver": "mysql",
      "configured": true,
      "state": "retrying",
      "attempt": 4,
      "last_error_at": "2026-04-18T21:35:52Z",
      "next_retry_at": "2026-04-18T21:36:24Z"
    }
  ]
}
```

Security rule:

- do not expose DSNs
- do not expose raw driver errors that may contain usernames, database names, or network detail
- full error detail remains in structured server logs only

### Logging

Managed provider logs must be structured and consistent.

Failure log:

- level: `WARN`
- message: `database unavailable; retry scheduled`
- fields:
  - `component=dbdeps`
  - `dependency`
  - `driver`
  - `attempt`
  - `next_retry_in`
  - `error`

Recovery log:

- level: `INFO`
- message: `database connected`
- fields:
  - `component=dbdeps`
  - `dependency`
  - `driver`
  - `attempt`
  - `recovered_after`

---

## Portal and App Visibility

Portal behavior stays configuration-based in v1.

Rules:

- Keep the existing startup-time app catalog filtering that hides apps only when their required DSNs are absent from config.
- Do not dynamically remove tiles because a configured DB is temporarily in `retrying`.
- Do not make app visibility depend on `/dependenciesz`.

Rationale:

- tile flapping during transient outages is worse than a stable launcher
- background reconnect should restore service without requiring a portal refresh
- this feature targets backend availability first, not portal runtime feature-state UX

Known consequence:

- a tile may remain visible while some or all of its pages return `503` during an outage

This is acceptable in v1.

---

## Module Migration Rules

Every DB-backed module moved to this system must follow these rules.

### Single-DB Modules

Applies to:

- compliance
- coperture
- energiadc
- rdfbackend

Migration rule:

- replace handler `*sql.DB` field with a provider
- replace `requireDB(w) bool` with `requireDB(w) (*sql.DB, bool)`

### Multi-DB Modules

Applies to:

- panoramica
- reports
- rdf
- afctools
- quotes
- listini
- kitproducts

Migration rule:

- one provider per dependency
- one `require*` helper per dependency
- helpers return the DB handle for that request
- existing per-dependency nil behavior must be preserved exactly

### Modules Without DB Dependencies

No change required for:

- portal
- budget
- simulatori-vendita

---

## Implementation Sequence

### Phase 1 — Platform Plumbing

- Create `backend/internal/platform/dbdeps/`
- Add provider status model and retry loop
- Inject `database.New` as the default opener, with test override support
- Extend `backend/internal/platform/health/health.go` to expose dependency summaries and `/dependenciesz`

### Phase 2 — Main Wiring

- Replace direct DB startup connects in `backend/cmd/server/main.go` with managed providers
- Remove DB-specific `os.Exit(1)` calls
- Keep auth and HTTP startup fatal behavior unchanged
- Keep launcher filtering based on config presence only

### Phase 3 — Pilot Migration

Pilot on AFC Tools first:

- `VODKA_DSN`
- `WHMCS_DSN`

Rationale:

- the original failure report came from AFC Tools
- AFC Tools already has clean per-datasource `503` guards
- this validates the provider pattern before touching broader shared dependencies

### Phase 4 — Shared Module Migration

Migrate remaining DB-backed modules in this order:

1. coperture, energiadc, compliance, rdfbackend
2. panoramica, reports, afctools full package
3. quotes, listini, kitproducts, rdf

Ordering rule:

- migrate simpler nil-guard modules before modules with documented optional fallbacks

### Phase 5 — Documentation

- Update `docs/IMPLEMENTATION-KNOWLEDGE.md` with the new runtime rule:
  - configured DB outage no longer implies process-fatal startup
  - route-level graceful degradation is the primary availability model
- Update any affected app implementation docs that currently assume “configured but unreachable” is fatal

---

## Verification Plan

### Unit Tests

For `dbdeps`:

- DSN empty -> `disabled`, no retries, `Current()==nil`
- opener fails repeatedly -> state moves to `retrying`, attempt count increments, next retry is scheduled
- opener fails twice then succeeds -> provider eventually returns non-nil without process restart
- status snapshots never expose DSN values

Implementation note:

- use injected opener and clock/sleeper hooks so tests are deterministic and fast

### Handler Tests

Add or update handler tests so request-time contracts remain unchanged:

- existing `503 *_database_not_configured` paths still return the same payloads
- Quotes Alyante fallback paths still return their documented defaults where they already do today

### Health Tests

- `/healthz` stays `200 {"status":"ok"}`
- `/readyz` returns `200` and shows degraded counts when providers are retrying
- `/dependenciesz` returns sanitized per-dependency states

### Manual Verification

Scenario A:

- start backend with invalid `WHMCS_DSN`
- confirm process stays up
- confirm `/readyz` returns `200`
- confirm `/dependenciesz` shows `whmcs=retrying`
- confirm WHMCS AFC Tools routes return `503 whmcs_database_not_configured`

Scenario B:

- keep backend running
- make WHMCS reachable
- confirm provider reconnects without restart
- confirm WHMCS AFC Tools routes start returning `200`

Scenario C:

- repeat once with a shared dependency such as `MISTRA_DSN`
- confirm unaffected routes stay up while Mistra-backed routes degrade per current contract

---

## Risks and Tradeoffs

### 1. Ready But Degraded Pods

With `/readyz` staying green, the service can route traffic to pods where some routes will return `503`.

This is intentional.

Reason:

- the goal is graceful degradation instead of whole-backend outage
- failing readiness for transient DB outages would preserve the current operational downside during restart windows

### 2. Launcher May Show Unavailable Apps

The portal will not hide configured apps during transient outages.

This is acceptable in v1 and preferable to tile flapping.

### 3. Runtime Disconnects After Successful Boot

This v1 does not demote already-connected handles on later failures.

That behavior can be added later if needed, but it is not required to solve the startup-fatality problem.

### 4. Migration Touch Surface

Because handlers currently store raw `*sql.DB`, this feature requires broad but mechanical handler changes across DB-backed modules.

The AFC Tools pilot is intended to validate the pattern before the shared modules are migrated.

---

## Acceptance Criteria

- Restarting the backend while one or more DBs are temporarily unreachable does not terminate the process.
- A DB that is unavailable at startup can become available later without process restart.
- Existing route-level graceful-degradation contracts remain intact.
- `/readyz` no longer turns transient DB outages into deployment-wide unavailability.
- Operators can inspect dependency state through structured logs and `/dependenciesz`.
- No DSN or credential-bearing detail is exposed in health responses.

