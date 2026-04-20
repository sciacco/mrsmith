# Phase C — Logic Placement

Classification tags (as used in the audit): **[B]** domain / business, **[O]** orchestration, **[P]** presentation.
Placement target: backend (Go `backend/`), frontend (React `apps/customer-portal-admin/` — name TBD in Phase E), or shared contract.

Dead JS (JSObject1 / JSObject2 / `UNUSED_DATASOURCE` action list) is not analyzed — excluded by the 1:1-minus-dead scope.

## Stato Aziende

| Source expression | Tag | Placement | Notes |
|---|---|---|---|
| `edit_customer.text = "Aggiorna " + t_customer_list.selectedRow.name` | **[P]** | frontend | Simple template; native React state. |
| `m_edit_customer` visibility via `showModal(m_edit_customer.name)` | **[O]** | frontend | Local UI state (modal open/closed). |
| `edit_customer_state.sourceData = {{GetAllCustomerState.data.items}}` + `optionLabel=name, optionValue=id` | **[P]** | frontend | Options populated from the prefetched customer-state list; no logic beyond mapping. |
| `EditCustomer` body `{ state_id: edit_customer_state.selectedOptionValue }` | **[O]** | frontend | Request assembly. The allowed-transition rule is server-owned. |
| `EditCustomer.run().then(...).catch(...)` — success refetch + modal close, error toast with HTTP status + `data.message` | **[O]** | frontend | Standard optimistic-free refetch flow. |
| PUT `/customers/v2/customer/{id}` with `{state_id}` → validate + persist | **[B]** | backend (Mistra NG, already live) | Not reimplemented; consumed via API client. |
| `customColumn1 = currentRow["group"].name`, `customColumn2 = currentRow["state"].name` | **[P]** | frontend | Column cell rendering. Keep nested DTO; unwrap at render time. |

## Gestione Utenti

| Source expression | Tag | Placement | Notes |
|---|---|---|---|
| `select_customer.sourceData = {{GetAllCustomer.data.items}}`, `optionLabel=name, optionValue=id` | **[P]** | frontend | Same customer list reused from page 2. |
| `GetAllUsersByCustomer` re-runs on `select_customer` change (Appsmith implicit binding) | **[O]** | frontend | New app: explicit `useEffect`/`queryKey` on `customer_id`. **Gate the fetch on a non-empty customer id.** |
| `t_user_list.primaryColumns.customColumn1 = currentRow["role"].name` | **[P]** | frontend | Column render; no domain logic. |
| `NewAdmin` body assembly from form widgets | **[O]** | frontend | Direct mapping to `user-admin-new`. |
| `new_user_notifications.selectedValues.includes("maintenance")` → `maintenance_on_primary_email` | **[P]** | frontend | Hard-coded value keys `"maintenance"`, `"marketing"` are internal to the form. |
| `new_user_skip_kc.isSwitchedOn` → `skip_keycloak` | **[B]** | **backend (server-side enforcement, not this app)** | Field is sent as-is; the permission check to allow `skip_keycloak=true` belongs to Mistra NG. No new gating is added in this 1:1 port. |
| `NewAdmin.run().then(refetch; closeModal).catch(errorToast)` | **[O]** | frontend | Same shape as EditCustomer. |
| `new_user_button.isDisabled` → `select_customer.selectedOptionValue` is empty | **[O]** | frontend | Confirmed by expert: button disabled until a customer is selected. |
| POST `/users/v2/admin` → create admin + (optionally) Keycloak provision | **[B]** | backend (Mistra NG, already live) | Not reimplemented. |
| `Text2 = "Ciao " + (appsmith.user.name || appsmith.user.email) + ", ..."` | **[P]** | frontend | Greeting uses operator identity; source it from Keycloak in the new app. |

## Accessi Biometrico

| Source expression | Tag | Placement | Notes |
|---|---|---|---|
| `GetTableData` SQL projection (flatten + join + order) | **[B]** on data shape, **[P]** on aliases | **backend (new endpoint)** | Direct SQL from UI is a trust-boundary violation; must move. The new backend endpoint owns the join and returns a flat DTO with the exact field names the current UI expects (so the frontend stays a 1:1 port). |
| Column labels `nome, cognome, tipo_richiesta, stato_richiesta, data conferma, data della richiesta` | **[P]** | frontend | Preserved verbatim; no relabeling under 1:1. |
| `data_approvazione` / `data_richiesta` rendered via `new Date(x).toLocaleString()` | **[P]** | frontend | Preserved as-is. Locale/timezone behavior matches source. |
| `stato_richiesta` editable checkbox, binding a `boolean` to `BiometricData.updatedRow.stato_richiesta` | **[B]** on boolean contract, **[P]** on checkbox | frontend | Type is **boolean** end-to-end (DB bool → SELECT projection → widget → mutation arg). |
| `EditActions1.onSave` → `UpdateRequestCompleted.run().then(refetch + showAlert(success)).catch(showAlert(error))` | **[O]** | frontend | Orchestration of mutation + refetch. The typo `howAlert('success')` is dead-on-arrival in the source (never fires) and is not reproduced. |
| `onCheckChange = storeValue('flag', BiometricData.updatedRow.stato_richiesta).catch(...Error(UpdateRequestCompleted.responseMeta.statusCode)...)` | **[O]** | — | Defensive no-op in the source (nothing reads `flag`, the catch references an unrelated query). **Dead under 1:1-minus-dead. Skip.** |
| `UpdateRequestCompleted` = `SELECT customers.biometric_request_set_completed(id, stato_richiesta)` | **[B]** | **backend (new endpoint)** | Same migration of trust boundary as the list. The backend may either invoke the stored function (preferred) or replicate the two-column update it performs (`request_completed`, `request_approval_date = now()`). Function body verified — no hidden side effects. |
| `is_biometric_lenel` computed by LEFT JOIN then dropped by `isVisible=false` | **[B]** on the join, **[P]** on hide | backend returns it; frontend does not render it | 1:1 keeps it invisible. Including it in the DTO is harmless and mirrors the source data shape. |

## Global / cross-view

| Source expression | Tag | Placement | Notes |
|---|---|---|---|
| `appsmith.user.name || appsmith.user.email` | **[O]** | frontend | Operator identity → Keycloak `preferred_username` / `email` in the new app. |
| Error toasts `'Failed to ... [HTTP ' + responseMeta.statusCode + ']: ' + data.message` | **[P]** | frontend | Same copy preserved. |
| Auth to `gw-int.cdlan.net` (cookie/header/gateway-level; not captured in export) | **[B]** on security | **backend** | The new app MUST NOT call `gw-int.cdlan.net` directly from the browser; every REST call goes through `backend/` which attaches whatever the real auth mechanism turns out to be. This is forced by the trust-boundary rule, not by UX. |

## Duplication handling under 1:1

- `GetAllCustomer` exists on two pages with identical parameters. **1:1 port preserves one call per page**. Code-level sharing of an API client helper (e.g., a single `listCustomers()` function) is **implementation detail**, not a behavioral change.
- No other duplication in live code.

## Rules being preserved, not revised

- `skip_keycloak` is sent as-is with no new client-side gate.
- `stato_richiesta` stays boolean (the dead JSObject1 string path does not exist in the 1:1 cut).
- Empty-selection behavior on `t_customer_list` before editing — preserved (button enabled, server rejects).
- First-load fetch on `Gestione Utenti` with empty `customer_id` — **corrected** (fetch deferred until a customer is picked). This is a correctness fix, not a UX revision; calling the endpoint with no filter is already observably broken in the source.

## Rules explicitly not ported

- `onCheckChange` defensive no-op on `stato_richiesta` (dead).
- `howAlert('success')` post-refetch toast (dead-on-arrival typo).
- JSObject1 client-side staging buffer and all related actionList entries (dead).
- `Query1` scratch SELECT on `accounting.acct_log` (dead).
- `Api1` empty-path REST action (dead).
- `Area documentale` modals, tables, and placeholder selects (dead wireframe).

## Gaps this phase could not resolve

- None (the `new_user_button.isDisabled` assumption was confirmed by the expert: disabled until a customer is selected).
