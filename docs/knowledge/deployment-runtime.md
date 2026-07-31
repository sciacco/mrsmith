# Deployment and Runtime Integration Rules

Backend runtime, shared infrastructure, and deploy-process rules.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Slow Read Endpoints Must Fit Server Write Timeout

- Context: slow report-style endpoints behind the shared Go HTTP server, including `GET /api/panoramica/v1/iaas/monthly-charges`.
- Discovery: a handler can finish its SQL work and still surface as a client-side transport failure if response delivery exceeds the server write budget or the downstream connection closes first. In that case, naive access logs can still misleadingly report a clean `200` unless the response writer captures downstream write errors.
- Practical rule: when a read endpoint is expected to run for tens of seconds, align `http.Server.WriteTimeout` with that runtime budget and make access logs record downstream write failures and request-context cancellation separately from normal completions.
- Practical rule: any synchronous endpoint that fans out batches of LLM or vendor calls will overrun the shared 60s write budget and lose the entire result (observed: a 138-call model-comparison run died at 175s with HTTP 000; a 10-call brief regeneration was truncated). Either move the work to an async job (`ma_job` pattern), or explicitly clear the deadlines via `http.NewResponseController` — which requires the wrapping response recorder in `backend/pkg/middleware` to implement `Unwrap()` — and bound each inner call with its own timeout so one slow call becomes a counted failure instead of sinking the request.
- Evidence: Panoramica local-dev failure on 2026-04-09 where `monthly-charges` took ~44s, Vite logged `socket hang up`, and backend access logging needed downstream write-error tracking to distinguish true delivery from handler completion.
- Used by: `apps/panoramica-cliente` IaaS PPU monthly charges view; shared backend middleware in `backend/pkg/middleware`.
- Open questions: whether future report endpoints should adopt per-handler query deadlines or asynchronous export flows instead of relying on a larger shared write timeout.

### Database Diagnostics Are A Low-Volume Error Inbox, Not Access Logging

- Context: support/debugging when stdout/container logs are not easy to inspect.
- Discovery: MrSmith can persist warning/error diagnostics to Anisetta through the async `internal/diagnostics` sink while keeping normal access logs on stdout.
- Practical rule: do not store routine 2xx/3xx request logs in SQL. Use `mrsmith.diagnostic_event` for low-volume `WARN`/`ERROR` events, 4xx/5xx API access summaries, panic recovery details, and support correlation by `request_id`. The sink must stay bounded, async, sanitized, and fail-open.
- Evidence: diagnostics migration `deploy/migrations/008_anisetta_mrsmith_diagnostics.sql` and `backend/internal/diagnostics`.
- Used by: backend-wide warning/error visibility and support request correlation.
- Open questions: whether frontend browser exceptions should feed the same table in a later phase.

### MrSmith-Owned Tables In Anisetta

- Context: cross-app MrSmith platform features that need durable storage or runtime configuration.
- Discovery: Anisetta is already wired through `ANISETTA_DSN` and contains several legacy/application domains in `public`; MrSmith-owned platform data should not be added to `public`.
- Practical rule: create MrSmith platform tables in the dedicated `mrsmith` schema on Anisetta. Runtime-editable configuration should live in `mrsmith.runtime_config` as namespaced JSONB key/value rows so operational values, such as support notification recipients, can change without restarting the backend.
- Evidence: support request migration `deploy/migrations/004_anisetta_mrsmith_support.sql`.
- Used by: contextual support requests.
- Open questions: none.

### GW Internal CDLAN Calls Use The Shared Arak Client

- Context: backend handlers that call `https://gw-int.cdlan.net` for ERP, PDF, Arxivar, or Mistra-NG style bridge operations.
- Discovery: MrSmith already wires a service-account HTTP client for this host through `backend/internal/platform/arak.Client` and the `ARAK_BASE_URL`, `ARAK_SERVICE_TOKEN_URL`, `ARAK_SERVICE_CLIENT_ID`, and `ARAK_SERVICE_CLIENT_SECRET` env vars.
- Practical rule: new mini-app BFF modules should inject and use the shared `*arak.Client` for `gw-int` calls. Do not add app-specific gateway credentials such as `GW_INT_*` unless the shared client is proven insufficient for a different upstream.
- Evidence: existing `arak.Client` wiring in `backend/cmd/server/main.go`; Ordini migration decision for ERP/PDF/Arxivar calls.
- Used by: `apps/ordini` implementation planning; `apps/rda`, `apps/fornitori`, and `apps/afc-tools` gateway/API proxy patterns.
- Open questions: none.

### New DSN-Backed Mini-Apps Must Update Both Dev and Preprod Env Templates

- Context: introducing a new launcher-backed mini-app that needs backend DSNs and optional split-server frontend URL overrides.
- Discovery: contributor defaults and deployment defaults are documented in two different places: local/backend-facing samples live in `backend/.env.example`, while the repo's pre-production sample lives at the root `.env.preprod.example`. Updating only the backend-local example leaves the real deploy template stale.
- Practical rule: when a new mini-app adds config such as `<APP>_APP_URL` or `<DB>_DSN`, update `backend/internal/platform/config/config.go`, `backend/.env.example`, and the root `.env.preprod.example` in the same change set. Treat both env examples as part of repo-fit wiring, not optional documentation.
- Evidence: Coperture rollout on 2026-04-17 added `COPERTURE_APP_URL` / `DBCOPERTURE_DSN` in `backend/internal/platform/config/config.go`, `backend/.env.example`, and `.env.preprod.example`.
- Used by: `apps/coperture`; future DSN-backed mini-apps.
- Open questions: none.

### Production Deploy Builds Run On The Target Host From A Git Archive

- Context: `make deploy-prod` production releases.
- Discovery: developer workstations may not have Docker available, or may use incompatible local container tooling on Apple Silicon. Production deploys therefore stream a committed Git archive over SSH and run `docker buildx build --load -` directly on the production host.
- Practical rule: production deploys require only `git` and `ssh` locally, but require Docker Engine, buildx, compose v2, and outbound build-network access on the target host. Deploys use committed source only and default to a clean-worktree guard; rollback retags immutable release image tags like `mrsmith:prod-YYYYmmddHHMMSS` instead of loading uploaded tarballs. If `deploy/Dockerfile` starts copying new root paths, update the `git archive` allowlist in `scripts/deploy/prod.sh` in the same change.
- Evidence: `scripts/deploy/prod.sh`, `Makefile`, `.env.deploy.prod.example`, and `docs/PROD-DEPLOY.md`.
- Used by: production deploy and rollback workflow.
- Open questions: none.
