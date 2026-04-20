# Feedback on `IMPL.md` (Customer Portal Back-office / `cp-backoffice`)

Reviewer: pre-gate review of `apps/customer-portal/IMPL.md` (plan only — no code yet).

Verified against:
- `apps/customer-portal/SPEC.md` (the plan's source of truth)
- `apps/customer-portal/audit/FINDINGS.md`
- `docs/IMPLEMENTATION-PLANNING.md` (repo-fit checklist)
- `docs/API-CONVENTIONS.md`
- `docs/UI-UX.md`
- Live state of `backend/cmd/server/main.go`, `backend/internal/platform/applaunch/catalog.go`, `backend/internal/platform/config/config.go`, root `package.json`, `Makefile`, `deploy/Dockerfile`, and representative mini-app scaffolds (`afc-tools`, `compliance`, `listini-e-sconti`, `budget`).

This doc lists what is strong, what is wrong or inconsistent, and what is underspecified. Each item cites the claim in the plan and the authoritative source in the repo.

---

## TL;DR

- **Overall quality**: good. The plan correctly reuses the `afctools` shape for dependency injection, reuses the Arak client for Mistra NG auth, and respects the mini-app UI family instead of inventing a bespoke shell.
- **Ready to code? Not quite.** There are three pre-code corrections that should be locked, one real security verification gap, and several minor inconsistencies with the SPEC or with repo conventions that will otherwise cause review churn later.

Blocking items: §1.1.
Non-blocking but recommended: everything else.

---

## 1. Blocking

### 1.1 IMPL surfaces a switch that is hidden in the source

IMPL §User Copy and §Slice 5 read as if the `skip_keycloak` switch is rendered in the new `Nuovo Admin` modal, with a cleaned-up label (`Non creare account su KC` → `Non creare account di accesso`). Static inspection of the Appsmith export (`apps/customer-portal/customer-portal.json.gz`, page *Gestione Utenti*, widget `new_user_skip_kc`) shows:

```json
{ "isVisible": false, "isDisabled": false, "defaultSwitchState": false }
```

with no dynamic binding on `isVisible` or `defaultSwitchState`. The switch is hidden in production and every admin creation today sends `skip_keycloak: false`. It was left in the DSL on purpose so an Appsmith editor could flip `isVisible: true` when the capability was needed — a latent toggle, not a live affordance.

Under the plan's own scope rule ("1:1 port of the wired flows; dead features ignored", SPEC §Scope rule) and the `1:1 = observed behavior` convention, the operator-visible surface today does not include this switch. Rendering it in the React port expands operator capability beyond the source.

Fix before coding:
- Drop the switch from the `Nuovo Admin` modal in IMPL §Slice 5.
- Omit `skip_keycloak` from the `createAdmin` request body and rely on Mistra NG's server default (`false`, per `docs/mistra-dist.yaml:5795`).
- Delete the "copy correction" for `Non creare account su KC` → `Non creare account di accesso` from IMPL §User Copy — there is no label to relabel.
- Re-enablement is tracked in `docs/TODO.md` → `Customer Portal Back-office App` → `Expose skip_keycloak toggle in Nuovo Admin modal (hidden in source, omitted in port)`. That follow-up carries the server-side role verification that SPEC §Security silently assumed.

Note: `audit/FINDINGS.md:22` treats the switch as operator-reachable; static DSL inspection disproves that. The SEC concern is moot under strict 1:1 because the field never carries `true` — but the verification it calls for is still the right work when the toggle is re-enabled, and lives in the TODO entry above.

---

## 2. Incorrect or Inconsistent With the Repo

### 2.1 Env var and Go field naming are underspecified

IMPL §Static hosting names the env var `CP_BACKOFFICE_APP_URL` but never pins the Go struct field in `config.go`. SPEC §Summary calls it `CPBackofficeAppURL`. Both are internally consistent, but the plan should state both to avoid two rounds of rename churn:

- Go field: `CPBackofficeAppURL` (matches `AFCToolsAppURL`, `KitProductsAppURL`, `RDFBackendAppURL`, `RichiesteFattibilitaAppURL` at `config.go:20–32`).
- Env var: `CP_BACKOFFICE_APP_URL` (matches the hyphen-to-underscore pattern used by `ENERGIA_DC_APP_URL`, `KIT_PRODUCTS_APP_URL`, `RDF_BACKEND_APP_URL`, `RICHIESTE_FATTIBILITA_APP_URL` at `config.go:98–104`).
- CORS default in `config.go:92` must be extended to include `http://localhost:5187`.

Worth noting: `AFCToolsAppURL` maps to `AFCTOOLS_APP_URL` (collapsed) — that app is the anomaly, not the pattern. Pick the majority pattern and do not copy `afctools` blindly.

### 2.2 Replace the commented-out legacy catalog placeholder

`catalog.go:243–250` has a commented-out `customer-portal` entry (icon `chat`, href `/apps/smart-apps/customer-portal`). `catalog.go:281–288` has a second one, `customer-portal-settings`. IMPL just says "Add a SMART APPS entry" but does not state whether either placeholder should be removed.

Leaving both placeholders plus a new active entry creates future drift. In the plan, explicitly:
- remove the `customer-portal` placeholder (superseded by `cp-backoffice`),
- leave or remove `customer-portal-settings` depending on intent (it is a distinct unbuilt app).

### 2.3 The icon "chat" is a legacy leftover — reconsider

IMPL §Repo-Fit picks `chat` "because it is already supported by the portal icon registry." It is (`apps/portal/src/components/Icon/icons.tsx:77`), but `chat` was used by the commented-out legacy Customer Portal placeholder for the **end-user** app — not a back-office admin tool. Using the same icon for the back-office companion will semantically confuse operators who remember the old launcher.

Pick one of the existing neutral icons: `shield`, `settings`, `users`, or `folder`. The icon registry (`apps/portal/src/components/Icon/icons.tsx`) already supports all of them.

### 2.4 API path naming is correct, but app-slug-vs-API-prefix convention isn't called out

IMPL picks `/api/cp-backoffice/v1/...` (slug = API prefix). `docs/API-CONVENTIONS.md:173–180` documents that not every app uses its folder slug as the API prefix — e.g. `listini-e-sconti` → `/api/listini/v1`, `panoramica-cliente` → `/api/panoramica/v1`. Using the full slug `cp-backoffice` is consistent with `afc-tools`, `energia-dc`, `kit-products`, `rdf-backend`, `richieste-fattibilita`, so it is fine. Just record that this is a deliberate choice and not an oversight; the reviewer will otherwise ask.

### 2.5 Dockerfile static-copy path is not in IMPL

IMPL §Static hosting correctly states the app must be copied to `/static/apps/cp-backoffice`, but does not show the concrete `deploy/Dockerfile:21–33` lines to add:

```
COPY --from=frontend /app/apps/cp-backoffice/dist /static/apps/cp-backoffice
```

Current Dockerfile follows one COPY per app. Lock the line in the plan — easy to forget under a bulk refactor.

### 2.6 Workspace / package-name consistency

IMPL says the package name follows `mrsmith-cp-backoffice`. That matches most apps' pattern (`mrsmith-budget`, `mrsmith-compliance`, …) and is fine. The one exception, `@mrsmith/afc-tools`, is a scoped anomaly — do not use it as a template.

Note: once the new package exists, root `package.json:6` must add a 16th entry to both `--names` (`cp-backoffice`) and `--prefix-colors`. The color list currently has 15 colors for 15 names; the plan should call out that both lists must grow in lockstep.

---

## 3. Underspecified

### 3.1 Frontend test for "users-query-disabled-until-customer-selected" conflicts with the repo test rule

IMPL §Verification lists a frontend test: "the users query stays disabled until a customer is selected." But:

- `AGENTS.md` Test Rule: tests only for reproduced bugs, business-critical rules, or non-trivial queries. Prefer the smallest useful test surface. Avoid custom fixtures / harnesses unless strictly necessary.
- Only one mini-app currently has frontend tests (`apps/simulatori-vendita/src/features/iaas/*.test.ts`). No shared frontend test harness exists. Adding one just to assert "no fetch on empty customer" means introducing a new harness for a small behavioral claim.
- The equivalent is already covered server-side by the plan's own backend test ("reject missing or empty `customer_id`"). That is the right enforcement point.

Drop the frontend test or justify the new harness. Do not leave both.

### 3.2 Implementation Slice 5 is too large to be a single PR

Slice 5 covers all three routes with distinct complexity:
- `Stato Aziende`: table + modal, simplest.
- `Gestione Utenti`: master-detail + modal form with DTO mapping + copy-gate correction on `skip_keycloak`.
- `Accessi Biometrico`: inline row-edit with Save / Discard, the one UI exception called out in §Exceptions.

Split Slice 5 into 5a / 5b / 5c. Each is reviewable and shippable independently; today's plan buries the biometric row-edit exception inside a mega-slice.

### 3.3 Observability and error surfacing are implicit

`docs/IMPLEMENTATION-PLANNING.md:34` is explicit: "Do not leave observability implicit." IMPL §Slice 2 mentions "sanitized internal-error responses with server logs carrying the real cause." Good, but §Verification does not close the loop. Add one line:
- internal 5xx responses use `httputil.InternalError` (matches `afctools/handler.go:78`); real cause stays in server logs with `component="cpbackoffice"` and `operation=...`.
- access log + request ID + recover middleware apply because the module mounts under the existing `api` sub-mux (`main.go:369–376`).

### 3.4 Pagination / defensive ceiling on `biometric-requests`

IMPL §Slice 4 preserves "no pagination" from source. Source SQL has no `LIMIT`. At current volumes that is fine; under growth, a single page can become a browser / network DoS for the operator. Even a 1:1 port can defend against this silently:
- add a safety `LIMIT` (e.g. 2000) and return an `isTruncated: true` flag;
- or explicitly state the plan accepts unbounded response size as a post-v1 risk, with a TODO entry.

Right now the plan is silent.

### 3.5 Identifier-strategy statement is missing (required by planning doc §5)

`docs/IMPLEMENTATION-PLANNING.md:25` requires identifier strategy to be explicit. For this app, every identifier is owned upstream (Mistra NG for customer/user/admin, Mistra PostgreSQL sequence for biometric_request). One line in §Repo-Fit closes this:
> "All identifiers are upstream-owned. This app creates no primary keys; request bodies are DTO-shaped exactly as Mistra NG expects."

### 3.6 Greeting copy references "Customer Portal" — which is now ambiguous

IMPL §User Copy allows preserving the greeting text from source. Source: *"Ciao X, in questa applicazione vengono visualizzati tutti gli utenti inseriti sul **Customer Portal** per l'azienda selezionata — da indicare tramite la select."* This app is not the Customer Portal — it is the back-office companion. The phrase now means "the other app" from the operator's point of view.

Options:
- Keep verbatim (1:1) with a code comment noting the ambiguity and a TODO.
- Apply a minimal copy patch: *"…tutti gli utenti inseriti **per l'azienda selezionata** — da indicare tramite la select."* (drop "sul Customer Portal").

Either is defensible; the plan should pick one.

### 3.7 Minor — column labels on `Accessi Biometrico` are flagged as a presentation gap in the audit

`audit/FINDINGS.md:41` flags `nome`, `cognome`, `tipo_richiesta`, `stato_richiesta` (lowercase, untidy) as presentation gaps to fix in rewrite. IMPL §User Copy preserves them verbatim under "1:1 observed behavior." That is a defensible decision (operators already see these exact labels; per the memory guideline `1:1 = observed behavior`) but it should be named as a **deliberate acceptance of the gap**, not a passthrough. Add a TODO in `docs/TODO.md` for post-port polish so the audit finding is not orphaned.

### 3.8 Nuovo Admin notification checkbox — map is asymmetric

IMPL §Slice 5 says the checkbox group "maps the notification checkbox values directly to `maintenance_on_primary_email` and `marketing_on_primary_email`." SPEC makes clear the values `"maintenance"` / `"marketing"` are internal UI keys — not wire values. IMPL should make that explicit so the next developer does not send `{ notifications: ["maintenance","marketing"] }` over the wire. Suggested wording: *"UI keys `'maintenance'` / `'marketing'` are **not** part of the DTO; they map locally onto the two booleans."*

### 3.9 Two-folder layout needs a one-line README pointer

IMPL §Repo-Fit says "Implement the SPA in `apps/cp-backoffice/`." The audit, SPEC, IMPL, FB, and the legacy `customer-portal.json.gz` export all live in `apps/customer-portal/`. After implementation the repo has two top-level folders for one app. This is a **recognized pattern** in the repo: `apps/zammu/` already holds the audit + four migspecs + a legacy `zammu-main.zip` for a port that was split into `apps/coperture/`, `apps/energia-dc/`, and `apps/simulatori-vendita/`, and no one has re-unified those folders.

Not a blocker — do not reorganize or delete anything. Just add a short `apps/customer-portal/README.md` during scaffolding, along the lines of:

> "Migration workspace for `apps/cp-backoffice/`. The SPA lives at `apps/cp-backoffice/`. This folder holds the original Appsmith export (`customer-portal.json.gz`), the phased audit (`audit/`), the staged migspecs (`migspec/`), the approved spec (`SPEC.md`), the implementation plan (`IMPL.md`), and this review (`FB.md`). Pattern follows `apps/zammu/`."

That closes the "what is this folder for?" question for a future reader without costing anything.

---

## 4. Positives Worth Preserving

These are things the plan gets right and that any revision should not undo.

- `master_detail_crud` archetype is the right call; the plan is explicit about rejecting `data_workspace` and the reports-style KPI rows.
- Dependency shape (`Arak *arak.Client`, `Mistra *sql.DB`, with `requireArak` / `requireMistra` / `dbFailure` guards) matches `afctools/handler.go:16–81` — good fit, no package-globals.
- Catalog visibility rule (`arakCli != nil && MistraDSN present`) matches the existing filter pattern at `main.go:301–336`.
- Browser-to-backend trust boundary is locked: no direct `gw-int.cdlan.net`, no direct Mistra PostgreSQL. Matches `docs/API-CONVENTIONS.md:63–69`.
- `disable_pagination=true` preserved intentionally for Mistra NG list endpoints, matching the 1:1 scope rule.

---

## 5. Concrete Edits Suggested to `IMPL.md`

1. Remove the `skip_keycloak` switch from IMPL §Slice 5 and delete the `Non creare account su KC` copy exception from §User Copy (see §1.1). Link the TODO entry that tracks re-enablement.
2. Add a "Pre-code verifications" subsection listing:
   - Mistra NG error shape preserved in toast format.
   - Vite port `5187` is not claimed by any developer's local dev override.
3. In §Repo-Fit, pin:
   - `CPBackofficeAppURL` Go field and `CP_BACKOFFICE_APP_URL` env var in one place.
   - CORS default extension.
   - Commented-out `customer-portal` placeholder removal in `catalog.go`.
   - Exact `deploy/Dockerfile` COPY line.
4. Replace the `chat` icon (see §2.3).
5. Drop the frontend test item (see §3.1).
6. Split Slice 5 into 5a/5b/5c.
7. Add an identifier-strategy line (see §3.5).
8. Resolve the Customer Portal greeting (see §3.6).
9. Add a one-line `apps/customer-portal/README.md` pointing at `apps/cp-backoffice/` (see §3.9).

---

## 6. Signoff Readiness

| Layer | Status |
|---|---|
| Product behavior | **Not OK** — IMPL surfaces a switch that is hidden in source (§1.1). |
| Repo / runtime integration | **OK** — host path, ports, dev wiring, static copy all match repo precedent. |
| Data & auth contract | **OK** — trust boundary locked, DTOs pinned, identifiers upstream-owned. |
| Verification strategy | **Mostly OK** — drop the frontend test (§3.1), add observability line (§3.3), define defensive ceiling for biometric list (§3.4). |

After §1.1 is resolved and the smaller items are folded in, this plan is ready for code.
