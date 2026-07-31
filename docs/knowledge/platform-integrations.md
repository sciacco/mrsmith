# Platform Integrations

Contracts and quirks of shared vendors and platform services: OpenAPI.it, HubSpot write queue, Google Drive, email ledger, OCR.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Google Shared Drive Service Accounts Need Explicit Trash and Access Handling

- Context: shared Google Drive integration used by MrSmith mini-app document contexts.
- Discovery: a Service Account added directly to a Shared Drive with the Google **Contributor** role can read metadata, list children, create folders, and read files uploaded manually by an analyst. Drive API calls for Shared Drives require the Shared Drive flags (`supportsAllDrives`, plus `includeItemsFromAllDrives`/`corpora=drive`/`driveId` when listing). Renaming preserves the folder ID and web link. Moving a folder elsewhere in the same Shared Drive also preserves its ID, so the backend must enforce the configured context root by walking parent ancestry. A trashed folder is still returned successfully with `trashed=true`; its descendants are also returned with `trashed=true`, so trash must be detected explicitly. Removing the Service Account from the Shared Drive makes an existing folder lookup return Google `404 notFound`, indistinguishable from a missing or invalid ID rather than a `403 permissionDenied`.
- Practical rule: use one DB-configured Service Account directly, bind each `(app, context)` to a Shared Drive and hard root, and validate both `driveId` and ancestry on every operation. Treat `trashed=true` as a dedicated domain error. Treat Google 404 as “not found or not accessible”; do not claim that revoked membership can be distinguished from deletion. Contributor is the verified minimum role for the initial `GetItem`/`ListChildren`/`CreateFolder` surface.
- Evidence: live spike against `MrSmith — Spike Google Drive` on 2026-07-29, recorded in GitHub issue #85.
- Used by: `backend/internal/platform/googledrive` (migration 126 for its `mrsmith.googledrive_credential` + `mrsmith.googledrive_context` tables); first consumer `apps/binocolo`.
- Open questions: none for the verified initial surface.

### Direct User-Triggered Emails Go Through the Email Ledger

- Context: any mini-app that sends an email as a user action tied to a domain object (e.g. RDA sending a PO email to a supplier) and needs "sent N times, last on … by … to …" before offering a resend.
- Discovery: the shared SMTP client (`backend/internal/platform/email`) does not track sends; the notifications worker tracks only its own deliveries. A global ledger was introduced: `emailledger.Send(ctx, msg, Correlation, Actor)` sends synchronously and always records the attempt in `mrsmith.email_send`, keyed by `app + entity_type + entity_id + purpose`. `status = 'accepted'` means the SMTP relay (Postal) accepted the message, not that it was delivered. The subject is stored, the body is not. Failures (SMTP or insert) are logged with `component=email` and flow into `mrsmith.diagnostic_event` automatically.
- Practical rule: user-triggered direct emails must go through `backend/internal/platform/emailledger`, never through the raw `email.Client`. Read counts/history via the Go API (`Count`, `List`) inside the owning app's endpoints, which keep their own authz; there is no shared HTTP read surface. Policy-driven notification emails stay on `internal/notifications` and are outside the ledger (future convergence possible by making the worker a ledger producer).
- Evidence: `backend/internal/platform/emailledger/emailledger.go`, migration `deploy/migrations/108_anisetta_mrsmith_email_ledger.sql`, wiring in `backend/cmd/server/main.go`.
- Used by: `backend/internal/rda` (injected as `Deps.EmailLedger` for the in-progress PO email feature).
- Open questions: none.

### OpenAPI.it Wrappers Stay Backend-Side

- Context: any MrSmith app that needs OpenAPI.it services such as CAP or Company.
- Discovery: OpenAPI.it specs use bearer authentication with service-specific hosts. CAP uses `https://cap.openapi.it` in production and `https://test.cap.openapi.it` for sandbox; Company uses `https://company.openapi.com` in production and `https://test.company.openapi.com` for sandbox. The token is shared backend secret material and must not be exposed through frontend config or browser clients.
- Practical rule: call OpenAPI.it through `backend/internal/platform/openapiit`. Configure `OPENAPI_IT_API_TOKEN` as a backend-only secret, `OPENAPI_IT_CAP_BASE_URL` only when overriding the default CAP production host, and `OPENAPI_IT_COMPANY_BASE_URL` only when overriding the default Company production host. Apps should inject the shared backend client into app-specific handlers rather than adding a generic public proxy.
- Evidence: `apps/binocolo/docs/cap.openapi.json`, `apps/binocolo/docs/company.openapi.json`, `backend/internal/platform/openapiit`, and backend config env wiring.
- Used by: `apps/binocolo` endpoint test workspace, future CAP/address validation and enrichment flows.
- Open questions: none.

### OpenAPI.it CAP `cod_fisco` Can Be Alphanumeric

- Context: CAP suppressed-municipality responses.
- Discovery: the CAP spec models `comuni_soppressi.cod_fisco` as a number, but its example contains alphanumeric Belfiore/cadastral codes such as `A627`.
- Practical rule: treat CAP fiscal/cadastral code fields as strings in MrSmith DTOs, even when the vendor schema says number.
- Evidence: `apps/binocolo/docs/cap.openapi.json`, endpoint `/comuni_soppressi`.
- Used by: `backend/internal/platform/openapiit` CAP DTOs.
- Open questions: none.

### OpenAPI.it DocuEngine Payloads Need Defensive Decoding And Vendor-Paced Polling

- Context: Binocolo filing acquisition (`backend/internal/platform/openapiit` DocuEngine client, `ma_filing` jobs) and any future OpenAPI.it document-service integration.
- Discovery: DocuEngine serves an empty `GET /requests` collection as HTTP `404` with error code `221` ("no requests related to your account"), so on a fresh account a pre-POST reconciliation loop fails forever and the POST never fires. The free tier allows **1440 GET/day per endpoint**, which a 2s worker tick burns in hours. Production scalars diverge from the spec: `GET /requests/{id}` returns `documents` as an array of filename strings (Download objects only come from `/requests/{id}/documents`), and `fileSize` is a number in production but a string in the spec examples — a rigid struct field makes the whole decode fail and the job spins in a silent "transient" poll loop (`attempts=0`, business row frozen, logs show `cannot unmarshal`). `GET /requests` has no server-side filters, so reconciliation works by name + time window + per-id GET. Separately, a manual upload and a camerale purchase of the *same* deposited filing always produce different md5s, because DocuEngine prepends a cover page containing a unique request header — two validated, aligned filings for one deposit is the systemic case, not a re-deposit.
- Practical rule: map empty-collection HTTP errors inside the client, never at call sites. Decouple vendor poll cadence from the worker tick (per-job interval, e.g. 30s). Decode OpenAPI.it scalars with flexible types (`DocuFlexString`/`DocuFlexInt64`-style). Resolve duplicate filings by extract identity (identical key figures → canonical is the one with `balance_sheet_id`; any divergence or missing extract → conservative ambiguous), never by md5.
- Evidence: production incident 2026-07-22 (fix `b9aed1d`) and smoke findings the same day; `backend/internal/binocolo/ma_filing_*.go`.
- Used by: Binocolo bilanci/NI acquisition (`filing_search`/`filing_acquire`/`filing_ingest` jobs).
- Open questions: none.

### Mistral OCR Page Markdown Excludes Tables When `include_blocks` Is On

- Context: Binocolo filing ingest OCR (`mistral-ocr-4-0` through the platform LLM client) and any future OCR consumer.
- Discovery: with `include_blocks=true` the per-page markdown does **not** contain the tables — only links like `[tbl-N.md](tbl-N.md)`; the actual content lives in `extras.tables[{id, format:"markdown", content}]` (duplicated in `blocks` of type `table`, with typed header/footer, bbox, per-page confidence). Italian filings come in two layouts: CCIAA fascicles with a cover page (CF in clear on p.1, repeated headers, page offsets, verbale at the end) versus "naked" XBRL-derived PDFs where the CF appears **only inside tbl-0** — so identity validation must look across multiple pages and inside tables. Batch mode can take hours and must never be used for interactive flows.
- Practical rule: reassemble page text **at read time** with a single shared helper for all consumers; persist the vendor markdown untouched. Never rely on page-1 markdown alone for identity checks.
- Evidence: live smoke 2026-07-22 on real fascicles (real 2023 figures: attivo=passivo 920.586; negatives rendered as `(6.709)`; header row `|   | 31-12-2023 | 31-12-2022  |`).
- Used by: Binocolo filing ingest and NI reading.
- Open questions: none.

### OpenAPI.it IT-full Closing Dates Are Local Midnight Serialized In UTC

- Context: any consumer of IT-full balance-sheet payloads that derives a date key.
- Discovery: the vendor serializes a closing date as local midnight expressed in UTC evening — FY2025 arrives as `2025-12-30T23:00:00` — so `raw[:10]` lands on the wrong day.
- Practical rule: when deriving a calendar date, round hour ≥ 12 up to the next day (`maBaselineExerciseDate`). Apply the correction only in new read paths: persisted vintage keys (`deepVintageKey`) were written from the raw prefix and must not be re-derived.
- Evidence: fix `318f271` in Binocolo bilanci.
- Used by: Binocolo adjusted-valuation path; any new IT-full consumer.
- Open questions: none.

### Company Domain Search Queries Must Never Contain Fiscal Identifiers

- Context: resolving a company's official website from anagraphic data (Binocolo domain resolver; any future company-web lookup).
- Discovery: putting the P.IVA/CF in a web-search query (Brave or fastcrw) returns **only registries and aggregators** (paginegialle, registroimprese, cerved…) and never the official site — company sites do not expose fiscal ids in indexable titles/snippets, registries do. Registries also never mention the official domain, so the two-phase "query VAT → registry page → extract domain" idea is a dead end. The strong identity signal is instead the **on-page** P.IVA (a legal obligation, art. 35 DPR 633/72) — which lives in the footer, exactly what Firecrawl-style `onlyMainContent` scraping strips; a foreign 11-digit id on a legal page is an equally strong wrong-entity/group-site signal. Name matching must use whole-word sets, never substrings ("safe" must not match "creditsafe"), and "sito ufficiale" in the primary query biases results toward directories. In practice the dominant residual bottleneck is scraper fetchability (renderer failures, SSRF guards, anti-bot), not acceptance logic.
- Practical rule: keep fiscal identifiers out of search queries but match them on-page with full-page scrapes (`ScrapeFull`, www-first fallback); treat a reject without verified identity as recall-unsafe (route to review, never to discard).
- Evidence: eval splits and live probes 2026-06-30 → 2026-07-02 on real sessions (Liguria, Nord-Est); `backend/internal/binocolo/ma_web_validation_job.go`, `backend/internal/platform/scrape`.
- Used by: Binocolo UC2 gate, associate-domain remedies, direct-card verification.
- Open questions: none.

### HubSpot Writes Go Through A Shared Async Queue On Anisetta

- Context: backend-driven HubSpot integrations that must not block the user-facing save, starting with raenad quote → deal creation (parent #56).
- Discovery: HubSpot calls are persisted as rows in `mrsmith.hubspot_request` on Anisetta and processed asynchronously with retry/backoff, instead of fire-and-forget goroutines or a synchronous call. A unique `dedupe_key` makes enqueue idempotent (`raenad:quote:{id}:deal:create` for the first deal; `raenad:quote:{id}:deal:update` for quote-owned updates), `attempt_count`/`max_attempts`/`next_attempt_at` drive backoff, statuses are `pending|locked|succeeded|failed|dead|cancelled`, and per-try detail is appended to `mrsmith.hubspot_request_attempt`. The local entity reference (e.g. `raenad.quote.hubspot_deal_id`) stays NULL until the worker completes, so a reconciler can re-enqueue `hubspot_sync_status='pending'` rows that have no matching live request. V1 also queues manual PDF attachment to HubSpot instead of doing it synchronously.
- Practical rule: enqueue HubSpot writes via `mrsmith.hubspot_request` with a stable `dedupe_key`; never make the user-facing save or manual PDF attach request depend on a synchronous HubSpot response. Use latest-wins coalescing for `hubspot.update_deal`: each relevant quote save upserts the single update request `raenad:quote:{id}:deal:update` and overwrites a non-locked retryable payload (`pending`, `failed`, or previously `succeeded`) with the newest quote-owned state. The payload must include the source `quote.updated_at` (or an equivalent sync version). If an update request is already `locked`, do not blindly overwrite the in-flight payload; the worker must compare the payload source version with the current quote after the attempt and re-arm a latest update if the quote changed during processing. If the update worker runs before `hubspot_deal_id` exists, it must defer/retry until `hubspot.create_deal` succeeds; update payloads must not modify `dealstage`. Gate the worker through `mrsmith.runtime_config` (namespace `raenad`, key `hubspot_queue_worker`) like other workers. Claim due rows with the `hubspot_request_due_idx` partial index under an advisory lock, and reconcile via the `(entity_type, entity_id)` index.
- Evidence: `deploy/migrations/024_mrsmith_hubspot_queue.sql`, `deploy/migrations/004_anisetta_mrsmith_support.sql` (mrsmith schema + runtime_config), and the worker-switch precedent in `deploy/migrations/011_quotes_hubspot_status_sync_config.sql`.
- Used by: raenad quote → HubSpot deal creation; reusable for any future backend-owned HubSpot write.
- Open questions: backend worker, reconciler, and backoff curve land in later #56 sub-phases.
