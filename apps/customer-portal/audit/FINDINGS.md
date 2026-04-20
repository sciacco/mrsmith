# Customer Portal — Findings Summary

Classification tags: **[B]** business rule, **[O]** orchestration, **[P]** presentation, **[SEC]** security / trust boundary, **[BUG]** actual defect in the Appsmith app.

## Embedded business rules to preserve
1. **[B]** Creating an admin user can skip Keycloak provisioning (`skip_keycloak: true`). The DTO field `skip_keycloak` is a real business primitive. Rewrite must preserve it and gate it behind an operator-role check server-side (current UI has no role gate).
2. **[B]** Admin-user creation persists two notification flags derived from a hard-coded set: `maintenance_on_primary_email` and `marketing_on_primary_email`. Hard-coded values `"maintenance"` and `"marketing"` are the wire contract.
3. **[B]** A customer's "state" can only be changed via `PUT /customers/v2/customer/{id}` with `{ state_id }`. The set of allowed transitions is enforced by the server, not the UI.
4. **[B]** Biometric-request completion is performed by a Postgres stored function `customers.biometric_request_set_completed(id, ?)`. The stored function almost certainly performs side effects (Lenel sync, audit, notifications) that are not visible in this export. **Do not rewrite the mutation until that function is read and its contract documented** — see `docs/mistradb/MISTRA.md` and the schema dumps it indexes.
5. **[B]** `is_biometric_lenel` is a real signal (user already enrolled in Lenel) but is hidden from operators today. Surface it in the rewrite unless product says otherwise.

## Duplication
- `GetAllCustomer` exists twice — one per page (Stato Aziende, Gestione Utenti) — identical definition. Consolidate into one shared client.
- Two parallel implementations of the biometric row-edit flow:
  - **Wired**: `BiometricData.primaryColumns.EditActions1.onSave` → `UpdateRequestCompleted.run()` → `GetTableData.run()`.
  - **Dead**: `JSObject1.onToggle/onSave/onDiscard/onToggleCompletion/onSaveCompletion` operating on `appsmith.store.tableData` + `appsmith.store.updatedRowIndices`. Intended a batch-edit workflow; never connected. Delete on migration; do not port.
- `Area documentale` has `areaSelect` and `Select1` both using the same hard-coded Blue/Green/Red list. Both are placeholders.

## Security / trust-boundary concerns
- **[SEC]** The UI holds a direct PostgreSQL connection (`mistra`, `10.129.32.20`) with `READ_WRITE` mode. SELECTs and a stored-function call go straight from the Appsmith runtime to the database. This is only safe because Appsmith proxies everything server-side; any rewrite that tries to replicate "UI → DB" semantics in a real browser is dangerous. **Every `mistra` call must become a backend endpoint** owned by the `backend/` Go service.
- **[SEC]** `GW interno CDLAN - S` has no auth headers in the export. Authentication is handled outside the exported config (gateway cookie, network-level allowlist, or Appsmith-injected header). The rewrite must reproduce whatever real mechanism is in use — do not assume "no auth".
- **[SEC]** `skip_keycloak` has no visible client-side gate. Anyone who can open the modal can toggle it. Server-side role check is mandatory in the rewrite.
- **[SEC]** Hard-coded IP `10.129.32.20` inside the export — fine for an Appsmith config blob, but any extracted env/secret handling in the rewrite must not land in client bundles.

## Fragile bindings / orphan logic
- **[O]** `BiometricData.primaryColumns.stato_richiesta.onCheckChange` does `storeValue('flag', BiometricData.updatedRow.stato_richiesta)` — writes a key nothing reads. In its `catch` it references `UpdateRequestCompleted.responseMeta.statusCode`, which is unrelated to a `storeValue` failure. **Dead/defensive noise.**
- **[O]** `Gestione Utenti` loads `GetAllUsersByCustomer` on page-load even though `select_customer.selectedOptionValue` is empty at that moment. Behavior depends on Appsmith binding evaluator subtleties; the rewrite must explicitly defer the fetch until a customer is selected.
- **[O]** `new_user_button.isDisabled` has a dynamic binding declared but the expression was not captured cleanly in this export inspection. Validate before rewrite — likely gates on `select_customer.selectedOptionValue`.
- **[O]** Several onClick handlers on `Area documentale` modals (`Button2`, `Button4`) have no code. The page is a wireframe.

## Bugs in the current app
- **[BUG]** `BiometricData` `EditActions1.onSave` calls `howAlert('success')` (typo; should be `showAlert`). The intended success toast after reload never fires. Only the synchronous toast `'Perfetto, stato biometrico cambiato'` survives.
- **[BUG]** Type/value mismatch on `stato_richiesta`:
  - `GetTableData` projects `br.request_completed AS stato_richiesta` — **boolean**.
  - `BiometricData.primaryColumns.stato_richiesta.columnType = checkbox` — **boolean-display**.
  - `UpdateRequestCompleted` forwards `BiometricData.updatedRow.stato_richiesta` — **boolean** under the current wired flow.
  - JSObject1 treats it as `"ok" / "pending"` **string** and would break the stored function call. This inconsistency is latent because JSObject1 is not wired; if anyone re-wires it, the mutation fails or corrupts data.
- **[BUG]** `onCheckChange` catch branch references `Error(UpdateRequestCompleted.responseMeta.statusCode)` (not a real Error constructor call pattern) — would produce unhelpful alert text if it ever triggered.

## Presentation gaps
- **[P]** Column labels on `BiometricData` are raw aliases (`nome`, `cognome`, `tipo_richiesta`, `stato_richiesta`). Rewrite should use proper Italian labels ("Nome", "Cognome", "Tipo richiesta", "Stato").
- **[P]** Dates in `Accessi Biometrico` are formatted with `new Date(x).toLocaleString()` — locale depends on the browser, no timezone handling.
- **[P]** Nested API objects (`customer.group`, `customer.state`, `user.role`) are unwrapped in the client via `customColumnN.computedValue = currentRow[...].name`. Prefer flat DTOs from the server.
- **[P]** Raw `created` on user list is rendered as-is.
- **[P]** `Home` page is a single welcome sentence — drop or replace with a real landing.

## Migration blockers
1. **Stored-function semantics** (`customers.biometric_request_set_completed`) must be understood before the biometric flow can be ported safely. High risk: side effects (Lenel, audit, notifications) are invisible in the UI export.
2. **Auth model for `gw-int.cdlan.net`** must be identified. The export is silent.
3. **`Area documentale`** has no backend contract to port. It requires a full design + backend spec before implementation.
4. **Data types** for `stato_richiesta` (bool vs. string) must be settled in the new DTO before writing the backend endpoint.

## Candidate domain entities (for the rewrite)
- **Customer** — `id`, `name`, `language`, `group`, `state`, `variables`.
- **Customer state** (lookup) — `id`, `name`.
- **User / Admin** — `id`, `customer_id`, `created`, `email`, `first_name`, `last_name`, `enabled`, `phone`, `role:{id,name}`, `maintenance_on_primary_email`, `marketing_on_primary_email`.
- **Biometric request** — `id`, `request_type`, `request_completed`, `request_date`, `request_approval_date`, `user_struct_id`, `customer_id`; joined with `user_entrance_detail` for `is_biometric` (Lenel-enrollment signal).
- **Document category** and **Document** — unverified; no backend referenced.

## Recommended next steps
1. Hand these artifacts to `appsmith-migration-spec` for the expert-led specification phase (as the skill guidance says: do not generate React code from raw bindings).
2. Before spec, gather three external inputs:
   - `docs/mistra-dist.yaml` entries for `/customers/v2/customer`, `/customers/v2/customer-state`, `/users/v2/user`, `/users/v2/admin` — to verify request/response shapes.
   - Definition of `customers.biometric_request_set_completed` (PostgreSQL) — read from the schema dump indexed by `docs/mistradb/MISTRA.md`.
   - Confirmation of the auth model used by the UI to reach `gw-int.cdlan.net`.
3. Decide explicitly whether `Area documentale` is in-scope for the first rewrite cut.
4. Delete the dead JSObject1 flow on migration — do not port.

## What was not verifiable from this export alone
- Exact `isDisabled` expressions on `new_user_button` (and a few other widgets) — only keys were captured.
- `EditActions1` column sub-widgets configuration (labels of Save/Discard, their `isVisible` / `isDisabled` bindings).
- Runtime behavior of `GetAllUsersByCustomer` when `customer_id` is unset.
- The body of the `customers.biometric_request_set_completed` stored function.
- The production auth mechanism for `GW interno CDLAN - S`.

Flagging these here so the migration-spec phase knows where to open source files, not just re-read the export.
