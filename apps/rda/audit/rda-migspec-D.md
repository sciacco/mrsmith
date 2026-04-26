# RDA migration spec — Phase D: Integration & Data Flow

**Reads from:** Phase A (entities), Phase B (views), Phase C (logic placement). Confirms the cross-system wiring for the new RDA module against the actual repo state.

This phase locks **data ownership boundaries**, **cross-module reuse**, and the **end-to-end journeys** that span more than one external system. The audit's `05_datasource_catalog.md` is the starting point; the goal here is to verify each datasource against the existing repo and decide reuse vs duplication.

---

## D.1 External systems and purpose

| System | Connection | Used for | Already wired? |
|--------|-----------|----------|----------------|
| **Mistra NG (Arak gateway)** at `gw-int.cdlan.net` | `backend/internal/platform/arak.Client` (Keycloak client-credentials, cached service token; `DoWithHeaders` for `Requester-Email` injection) | Every PO / row / attachment / comment / approval / provider / budget / article / user-search call | **Yes.** Re-used for RDA; no client-side calls to Mistra ever from the browser. |
| **Keycloak (`Mistra` realm)** | OIDC code flow at the portal layer; resolved by `backend/internal/auth.Middleware` into `auth.UserClaims{Email, Roles…}` | Authenticate the user and derive permissions (replaces legacy `users_int.role` SQL) | **Yes.** RDA registers new roles; does not invent a new auth path. |
| **Arak Postgres (`arak_db (nuovo)`)** | `*sql.DB` injected into module handlers (used today by `fornitori`) | Catalogs that don't have a Mistra REST endpoint: `provider_qualifications.payment_method`, `provider_qualifications.payment_method_default_cdlan` | **Yes** as a connection; **No** as RDA-specific reads — to be added in `backend/internal/rda/`. |
| **S3 bucket `arak`** | `s3cloudlan` plugin in legacy | Listing / downloading raw files | **Not used** — the legacy `listArak` query is dead. PDF/attachment download go via Mistra (`/po/{id}/download`, `/po/{id}/attachment/{aid}/download`). The S3 bucket may live behind those Mistra endpoints, but our module never talks to S3 directly. |
| **Mailer / notification system** | n/a | n/a | Not wired in v1 (Q-A6 → @-mentions cosmetic; no notification path is created in the new module). |

---

## D.2 Cross-module reuse (within `backend/internal/`)

The portal already hosts a **`fornitori`** module that proxies the entire `/arak/provider-qualification/v1/*` surface (`backend/internal/fornitori/handler.go` registers `GET/PUT/POST/DELETE /fornitori/v1/provider[/...]`, `POST/PUT /fornitori/v1/provider/{id}/reference[/{ref_id}]`, `GET /fornitori/v1/provider/{id}/category`, etc.). RDA needs **exactly the same calls** the legacy app made under `ListaFornitori`, `nuovoFornitore`, `GetProviderDetail`, `CreateProviderRef`, `EditProviderRef`.

**Decision (locks audit's "Phase D ownership boundary" question and B.Q-1):**

- **Reuse `fornitori` for provider read paths.** The RDA frontend imports the typed `fornitori` API client and calls `/api/fornitori/v1/provider[/...]` directly. No `/api/rda/v1/providers` proxy.
- **Reuse `fornitori` for the inline new-supplier flow.** "Aggiungi nuovo fornitore" inside `ModalNewPO` calls `/api/fornitori/v1/provider` with the same body the legacy `nuovoFornitore` produced. The role gate is `app_fornitori_access` (already required for any RDA user — see D.3 for the access-role bundle).
- **Reuse `fornitori` for the provider-refs editor in `Tab "Contatti Fornitore"`.** Same — call `/api/fornitori/v1/provider/{id}/reference[/{ref_id}]`.

This wipes out 6 endpoints from the proposed C.3 surface (everything under `/api/rda/v1/providers/...`). Track the dependency in `apps/rda/package.json` (or rather, in the `@mrsmith/api-client` package, where the fornitori client should live).

> **Implication for users:** any user with `app_rda_access` must also have `app_fornitori_access` (read at minimum). Decide in Phase D.3 whether to bundle this at the role level or at the launcher level.

---

## D.3 Keycloak role layout

Per `CLAUDE.md` / "New App Checklist" and the existing `backend/internal/platform/applaunch/catalog.go` patterns (e.g. `manutenzioniAccessRoles = []string{"app_manutenzioni_access"}`, `manutenzioniApproverRoles = []string{"app_manutenzioni_approver"}`), the RDA module introduces:

```go
// backend/internal/platform/applaunch/catalog.go
RDAAppID = "rda"   // app id used in URLs / launcher
rdaAccessRoles                = []string{"app_rda_access"}
rdaApproverL1L2Roles          = []string{"app_rda_approver_l1l2"}
rdaApproverAFCRoles           = []string{"app_rda_approver_afc"}
rdaApproverNoLeasingRoles     = []string{"app_rda_approver_no_leasing"}
rdaApproverExtraBudgetRoles   = []string{"app_rda_approver_extra_budget"}
```

Mapping to the legacy flags:

| Legacy `users_int.role` flag | New Keycloak role | Used to gate |
|------------------------------|--------------------|--------------|
| `is_approver` | `app_rda_approver_l1l2` | L1/L2 approve+reject buttons; `/rda/inbox/level1-2` |
| `is_afc` | `app_rda_approver_afc` | leasing approve/reject; payment-method approve/reject; "Leasing Creato"; `/rda/inbox/leasing`, `/rda/inbox/payment-method` |
| `is_approver_no_leasing` | `app_rda_approver_no_leasing` | no-leasing approve/reject; `/rda/inbox/no-leasing` |
| `is_approver_extra_budget` | `app_rda_approver_extra_budget` | budget-increment approve/reject; `/rda/inbox/budget-increment` |

**Access bundle.** Any user opening any `/rda/...` route needs `app_rda_access` (the launcher tile gate). The 4 approver roles are independent and *do not* imply `app_rda_access` — they only matter when the user already has access. The launcher hides the tile when the user lacks `app_rda_access`; each `/rda/inbox/:kind` route additionally enforces its specific approver role server-side.

**`fornitori` dependency:** any user with `app_rda_access` must also have `app_fornitori_access` (read scope) so the provider listing / new-supplier flow works. Two implementation options, decide in Phase E:

- **(D-R1) Bundle at the Keycloak group level** — the "RDA users" group includes both `app_rda_access` and `app_fornitori_access`. Cleaner for ops; requires a Keycloak group-mapping change.
- **(D-R2) Soft fallback at the UI level** — RDA frontend calls `/fornitori/v1/...`; if a 403 comes back the UI surfaces "Servizio fornitori non disponibile, contattare l'amministratore". No backend coupling.

The legacy app effectively used D-R1 (everyone with RDA access already had broader provider-qualification access). Default for v1: **D-R1**. Track in `docs/TODO.md` if the Keycloak group needs to be created.

**`/me/permissions` endpoint.** Stays even though all the data is reachable from the JWT — the RDA frontend uses it as the *single source of truth* for the action-bar gating logic, so it never has to inspect raw token roles. Trivial in Go: read `auth.UserClaims.Roles` from `auth.Middleware`, return four booleans.

---

## D.4 Datasource ownership decisions

The new RDA module **owns** four reads that the legacy app did via `arak_db (nuovo)` direct PG. Per `CLAUDE.md` no client-side DB; per the user's "1:1 + reuse Mistra" stance no new Mistra endpoints. Resolution per query:

| Legacy query | New owner | New endpoint | Notes |
|--------------|-----------|--------------|-------|
| `PaymentMethonds` (`SELECT * FROM provider_qualifications.payment_method WHERE rda_available IS TRUE`) | **`backend/internal/rda`** reads `arakDB *sql.DB` | `GET /api/rda/v1/payment-methods` | Same SQL, server-side. The `arakDB` pool is already passed into `fornitori`; `main.go` extends it to RDA. |
| `GetDefaultPaymentMethod` (`SELECT payment_method_code FROM provider_qualifications.payment_method_default_cdlan`) | same | `GET /api/rda/v1/payment-methods/default` | Same SQL, server-side. |
| `userID` (`SELECT id FROM users_int.user WHERE email = '<input>'`) | derived from token | merged into `GET /api/rda/v1/me` (or `/me/permissions`) | The legacy `idUser` is used only by the comments feature; we surface a numeric `user_id` in `/me` if Mistra requires it for posting comments. (Most likely it does **not** — the Mistra comment endpoint already infers the user from `Requester-Email`.) Confirm in implementation. |
| `user_permissions` (4-flag JOIN on `users_int.role`) | derived from token | `GET /api/rda/v1/me/permissions` | Already covered in D.3. **Closes S-1.** |
| `Suppliers` (legacy SQL, unused) | — | — | DROP. |
| `get_item_types` (legacy SQL, alternate to `/article`) | — | — | DROP — Mistra's `GET /arak/rda/v1/article` is sufficient. |
| `GetArticles` (legacy debug SQL) | — | — | DROP. |

Why direct-DB reads survive for payment methods: there is **no** Mistra REST endpoint exposing `provider_qualifications.payment_method` (the audit notes "BE: expose as REST endpoint or read in backend service layer"; we choose the latter for v1 to avoid a Mistra change). The data is read-only and low-cardinality; the pattern matches what `fornitori` already does for its catalog reads.

---

## D.5 End-to-end user journeys

Five journeys cover the entire app. Each lists the actors, the surfaces touched, and any cross-system hops.

### Journey J-1 — Requester creates and submits a draft RDA

1. User opens `/rda` (must have `app_rda_access`).
2. UI calls (in parallel): `GET /api/rda/v1/pos`, `GET /api/rda/v1/budgets`, `GET /api/fornitori/v1/provider?usable=true`, `GET /api/rda/v1/payment-methods`, `GET /api/rda/v1/payment-methods/default`.
3. User clicks "Nuova richiesta", fills the form, optionally creates a new supplier (calls `POST /api/fornitori/v1/provider/draft` → `fornitori` module → Mistra; refresh `GET /api/fornitori/v1/provider`).
4. User clicks "Crea Bozza" → `POST /api/rda/v1/pos` → RDA module shapes the body and forwards `POST /arak/rda/v1/po` with `Requester-Email: <token email>` → returns `{id}` → frontend navigates to `/rda/po/:newId`.
5. PO Details `/rda/po/:newId` loads `GET /api/rda/v1/pos/{id}` plus dependent fetches (budgets, providers, payment methods, comments, articles, mention users, provider refs).
6. User adds rows via the item modal: each save → `POST /api/rda/v1/pos/{id}/rows`.
7. User uploads quote PDF(s) via the Allegati tab → `POST /api/rda/v1/pos/{id}/attachments` (multipart). RDA module reads current PO state (`GET /arak/rda/v1/po/{id}` once) → picks `attachment_type=quote` → forwards `POST /arak/rda/v1/po/{id}/attachment`.
8. User clicks "Manda PO in Approvazione" → confirm dialog → `PATCH /api/rda/v1/pos/{id}` (header) then `POST /api/rda/v1/pos/{id}/submit` → Mistra routes the PO (PENDING_APPROVAL_PROVIDER → … server-side).

### Journey J-2 — L1/L2 Approver acts on a PO

1. User opens `/rda/inbox/level1-2` (requires `app_rda_access` AND `app_rda_approver_l1l2`).
2. UI calls `GET /api/rda/v1/pos/inbox/level1-2`. RDA module forwards to Mistra `GET /arak/rda/v1/po/pending-approval` with `Requester-Email: <token>`.
3. User clicks "Gestisci" on a row → `/rda/po/:id`.
4. UI loads detail (same as J-1.5). Action bar shows "Approva (Liv N)" / "Rifiuta (Liv N)" because (a) state == PENDING_APPROVAL, (b) user has the role, (c) email ∈ `approvers[]`.
5. User clicks Approve → `POST /api/rda/v1/pos/{id}/approve`. RDA module enforces (b) and (c) before forwarding `POST /arak/rda/v1/po/{id}/approve` (defence in depth, even though Mistra also enforces).
6. Toast + navigate back to `/rda/inbox/level1-2`.

### Journey J-3 — AFC handles leasing

Same as J-2 but `/rda/inbox/leasing`, role `app_rda_approver_afc`, endpoints `/leasing/approve` and `/leasing/reject`. Followed by the leasing-creation step: when state is `PENDING_LEASING_ORDER_CREATION`, the same AFC user clicks "Leasing Creato" on `/rda/po/:id` → `POST /api/rda/v1/pos/{id}/leasing/created` → state → `PENDING_SEND`.

### Journey J-4 — Send order to provider

1. User opens `/rda/po/:id` while state == `PENDING_SEND` (per Q-A2 there is no role check; any reader can do this).
2. Clicks "Invia ordine al fornitore" (`btn_sendOrder`, located in `cnt_fornitore` of the header form).
3. `POST /api/rda/v1/pos/{id}/send-to-provider` → Mistra → server-side dispatches the order.
4. Toast "RDA Inviato con successo" → back to `/rda`.

### Journey J-5 — Conformity (post-delivery)

1. State has reached `PENDING_VERIFICATION` (server-side, after the order is sent and goods/services are delivered).
2. User uploads a DDT via Allegati → tagged `transport_document` automatically.
3. User clicks "Erogato e conforme" → `POST /api/rda/v1/pos/{id}/conformity/confirm`. If no DDT was uploaded, Mistra returns an error; RDA module surfaces it as the legacy toast "verifica inserimento DDT" (B-4).
4. Or: user clicks "In contestazione" → `POST /api/rda/v1/pos/{id}/conformity/reject`.

---

## D.6 Hidden / triggered processes

The legacy app has **no client-side schedulers, timers, or background jobs**. Everything that looks like a "trigger" is in fact a state machine on Mistra:

| Apparent trigger | Reality |
|------------------|---------|
| "PO automatically routes to leasing approval after L1+L2" | Server-side. Mistra changes `state` to `PENDING_LEASING` after both approvals; the inbox endpoints surface it. |
| "Attachments upload after submit auto-tag as `transport_document`" | Pure derivation from the current `state`. No async worker. |
| "Email notifications" | Outside our scope; assumed to live on Mistra (or not at all, given that today's @-mentions don't notify anyone — S-3). |

The **RDA Go module has no goroutines, no cron, no queue** in v1.

---

## D.7 Data ownership boundaries

| Concern | Owner | Rationale |
|---------|-------|-----------|
| PO aggregate (header, rows, attachments, comments, recipients, approvers, state machine) | Mistra | Source of truth; we never persist PO data in our DB. |
| Provider qualification (suppliers + refs + categories) | Mistra (via `fornitori` module) | Already owned upstream; RDA reuses `fornitori`. |
| Budgets | Mistra | Read-only consumer. |
| Payment-method catalog | `arak_db (nuovo)` Postgres, read by `backend/internal/rda` | No Mistra REST equivalent; pattern matches `fornitori` catalog reads. |
| Article catalog | Mistra | Read-only consumer. |
| User identity (id, email, name) | Keycloak (token) + Mistra `users-int` (search only for @-mentions) | We never persist users locally. |
| Permissions (4 booleans) | Keycloak roles (token claims) | Replaces legacy `users_int.role` SQL. |
| File storage (attachments, generated PDFs) | Mistra → S3 (transparent to us) | We only call Mistra `/download` endpoints; we never sign S3 URLs ourselves. |

> **No new tables**, **no new schemas**, **no migrations** are introduced in v1. The only pool addition is whatever connection pool config `cmd/server/main.go` already has for `arakDB`.

---

## D.8 Configuration & dev wiring (preview for Phase E)

These flow into the Phase E new-app checklist:

| Where | Add |
|-------|-----|
| `package.json` (root) | `dev:rda` script + `concurrently` entry (color, name, filter `--filter mrsmith-rda`) |
| `Makefile` | `dev-rda` target + add to `.PHONY` |
| `backend/internal/platform/applaunch/catalog.go` | `RDAAppID` const + 5 role slices + catalog entry in the `Apps()` function + `RDAAccessRoles()`/`RDAApproverL1L2Roles()`/etc. accessors |
| `backend/cmd/server/main.go` | import `internal/rda`; add `hrefOverrides` for dev port; catalog filter; `rda.RegisterRoutes(mux, arakClient, arakDB)` |
| `backend/internal/platform/config/config.go` | `RDAAppURL` field + `RDA_APP_URL` env var (e.g. `http://localhost:5184` in dev) |
| `apps/rda/` (new) | `package.json` (`name: "mrsmith-rda"`), `vite.config.ts` (proxy `/api` → `:8080`), `tsconfig.json`, `index.html`, `src/main.tsx`, `src/App.tsx`, etc. |
| `packages/api-client` | RDA-specific functions (one per `/api/rda/v1/...` endpoint listed in C.3) |

The dev port has to be picked. `apps/manutenzioni` uses `5183`; recent `apps/fornitori` already exists; pick **`5184`** or the next free port — verify in Phase E by reading the existing `vite.config.ts` files.

---

## D.9 Open questions surfaced by Phase D

| # | Question | Default if no answer |
|---|----------|----------------------|
| **D.Q-1** | Provider role bundling: D-R1 (Keycloak group includes both `app_rda_access` and `app_fornitori_access`) vs D-R2 (UI fallback on 403). Which one does ops prefer? | **D-R1.** Track Keycloak group change in `docs/TODO.md`. |
| **D.Q-2** | Does Mistra's `POST /comment` endpoint require a `user_id` in the body, or does it derive the author from `Requester-Email`? Determines whether the Go module needs to call `users_int.user` (DB) or `users-int/v1/user?email=...` (REST) on every comment post. | **Assume Requester-Email is sufficient** until implementation proves otherwise. If it's not, fetch the numeric id once at login and cache. |
| **D.Q-3** | Inbox role gate on `/api/rda/v1/pos/inbox/:kind`: should the Go module *also* enforce the role (so a user without `app_rda_approver_afc` cannot scrape the leasing inbox by hitting the API directly), or is the launcher gate enough? | **Enforce in the Go module too.** Front-end gate ≠ back-end gate; never trust the client. Trivial via `acl.RequireRole(...)`. |
| **D.Q-4** | The legacy `Container2` header had a placeholder for `Acquisti RDA approvers I-II` (a Keycloak group string in `disabledWhenInvalid` of `groupButtono66fk8kt2a`). Is that group actually still used anywhere outside this file? | **Treat as dead.** The new app uses the four `app_rda_approver_*` roles defined in D.3. If ops still has a group by that name, leave it alone; we don't bind to it. |

These do not block Phase E; defaults shipped unless someone objects.
