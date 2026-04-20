# Customer Portal — Datasource & Query Catalog

Two datasources, 16 actions.

## Datasource: `mistra` (PostgreSQL, direct)
- **Plugin**: `postgres-plugin`
- **Host**: `10.129.32.20`
- **Mode**: `READ_WRITE`, default SSL
- **Schemas touched**: `customers`, `accounting`
- **Credentials in export**: none (Appsmith holds them server-side).

This is a **direct DB connection from the UI runtime** — the rewrite must replace every use of this datasource with a backend HTTP endpoint.

### Queries on `mistra`

#### `GetTableData` (page: Accessi Biometrico) — READ
```
SELECT
  br.id,
  us.first_name AS nome,
  us.last_name AS cognome,
  us.primary_email AS email,
  c.name AS azienda,
  br.request_type AS tipo_richiesta,
  br.request_completed AS stato_richiesta,
  br.request_approval_date AS data_approvazione,
  br.request_date AS data_richiesta,
  COALESCE(ued.is_biometric, false) AS is_biometric_lenel
FROM customers.biometric_request br
JOIN customers.user_struct us        ON br.user_struct_id = us.id
JOIN customers.customer c            ON br.customer_id = c.id
LEFT JOIN customers.user_entrance_detail ued ON ued.email = us.primary_email
ORDER BY data_richiesta DESC
```
- **Purpose**: list all biometric access requests with user + company context + current Lenel-enrollment flag.
- **Inputs**: none.
- **Outputs**: array of rows; `stato_richiesta` is `boolean` (from `request_completed`).
- **Dependencies**: bound to `BiometricData.tableData` and indirectly by every JSObject1 method that reads `BiometricData.processedTableData`.
- **Rewrite recommendation**: **backend**. New endpoint `GET /biometric-requests` returning a flat DTO that includes `user.first_name`, `user.last_name`, `user.email`, `customer.name`, `request_type`, `request_completed` (bool), `request_date`, `request_approval_date`, `is_biometric_lenel`. Server owns pagination / filtering. Consider a `status` filter (`pending | completed | all`).

#### `UpdateRequestCompleted` (page: Accessi Biometrico) — WRITE
```
SELECT customers.biometric_request_set_completed({{ BiometricData.updatedRow.id }}, {{BiometricData.updatedRow.stato_richiesta}});
```
- **Purpose**: flip the completion state of a request — the **only** mutation path in this app.
- **Inputs**: `id` (bigint), `stato_richiesta` (value-in-row; wired path sends boolean, JSObject1 path would send `"ok"/"pending"` string).
- **Outputs**: result of the stored function (unused by the UI).
- **Dependencies**: triggered from `BiometricData.primaryColumns.EditActions1.onSave`.
- **Rewrite recommendation**: **backend**. Wrap the stored function behind e.g. `POST /biometric-requests/{id}/complete` and `POST /biometric-requests/{id}/reopen` (or a single PATCH with `{ completed: bool }`). **Before reimplementing**, inspect the function body in Postgres — it almost certainly performs side effects (Lenel sync, audit log, notifications) that must not be lost.

#### `Query1` (page: Area documentale) — READ, **unused**
```
SELECT * FROM accounting."acct_log" LIMIT 10;
```
- **Purpose**: leftover scratch query; not referenced by any widget.
- **Rewrite recommendation**: **do not port**. Delete.

---

## Datasource: `GW interno CDLAN - S` (REST API)
- **Plugin**: `restapi-plugin`
- **Base URL**: `https://gw-int.cdlan.net`
- **Auth / headers**: none in the export (injected by Appsmith at runtime — session or gateway-level).
- **SSL**: default

This is the **internal customer/user gateway**. Paths match the Mistra NG Internal API (cross-reference `docs/mistra-dist.yaml` before finalizing the rewrite).

### Queries on `GW interno CDLAN - S`

#### `GetAllCustomer` (pages: Stato Aziende, Gestione Utenti) — READ
- **Method / path**: `GET /customers/v2/customer`
- **Query params**: `page_number=1`, `disable_pagination=true`
- **Purpose**: list all customers; same query exists on two pages (duplication).
- **Rewrite recommendation**: **backend proxy or direct call**. Consolidate into one shared frontend client. Consider enabling real pagination once the dataset grows.

#### `GetAllCustomerState` (page: Stato Aziende) — READ
- **Method / path**: `GET /customers/v2/customer-state`
- **Query params**: `page_number=1`, `disable_pagination=true`
- **Purpose**: populate the "Stato" select when editing a customer.
- **Rewrite recommendation**: cache on the client; this is a small lookup list.

#### `EditCustomer` (page: Stato Aziende) — WRITE
- **Method / path**: `PUT /customers/v2/customer/{{t_customer_list.selectedRow.id}}`
- **Headers**: `content-type: application/json`
- **Body**:
  ```
  { state_id: edit_customer_state.selectedOptionValue }
  ```
- **Purpose**: change a customer's `state_id`.
- **Rewrite recommendation**: preserve the contract (same endpoint, same verb, same body). Add explicit confirmation dialog before sending.

#### `GetAllUsersByCustomer` (page: Gestione Utenti) — READ
- **Method / path**: `GET /users/v2/user`
- **Query params**: `page_number=1`, `disable_pagination=true`, `customer_id={{select_customer.selectedOptionValue}}`
- **Purpose**: list users scoped to a customer.
- **Dependencies**: re-runs on `select_customer` change (implicit via Appsmith binding evaluator).
- **Rewrite recommendation**: backend-filtered by `customer_id`; in the rewrite, gate execution until a customer id is selected (don't hit the endpoint with an empty value).

#### `NewAdmin` (page: Gestione Utenti) — WRITE
- **Method / path**: `POST /users/v2/admin`
- **Headers**: `content-type: application/json`
- **Body**:
  ```
  {
    first_name: new_user_first_name.text,
    last_name: new_user_last_name.text,
    email: new_user_email.text,
    customer_id: select_customer.selectedOptionValue,
    phone: new_user_phone.text,
    maintenance_on_primary_email: new_user_notifications.selectedValues.includes("maintenance"),
    marketing_on_primary_email: new_user_notifications.selectedValues.includes("marketing"),
    skip_keycloak: new_user_skip_kc.isSwitchedOn
  }
  ```
- **Purpose**: create an admin user for a customer; `skip_keycloak=true` means "do not provision in Keycloak".
- **Rewrite recommendation**: keep the same DTO contract. Gate `skip_keycloak` behind an explicit operator role check server-side (the current UI does not gate it).

#### `Api1` (page: Accessi Biometrico) — DEAD
- **Method / path**: GET, empty path.
- **Purpose**: none — never completed.
- **Rewrite recommendation**: **do not port**.

---

## JS / in-UI actions (datasource `UNUSED_DATASOURCE`)
All on page **Accessi Biometrico**. These are the `actionList` entries that mirror JSObject1 methods:

| Name | Purpose | Wired to a widget? |
|------|---------|--------------------|
| `onSave` | Mark row as `"ok"` in `appsmith.store.tableData` and clear dirty flag for that row | **No** |
| `onToggle(currentRow, currentIndex)` | Toggle row between `"ok"` and `"pending"` in `appsmith.store.tableData`, mark row dirty | **No** |
| `onDiscard` | Reset row to server baseline | **No** |
| `onToggleCompletion(currentRow, currentIndex)` | Toggle `request_completed` on row in staging buffer | **No** |
| `onSaveCompletion` | Run `UpdateRequestCompleted.run()` then clear dirty flag | **No** |
| `Table1primaryColumnsstato_richiestaonCheckChange` | Empty stub | **No** |

All six are orphaned. See Findings.

---

## Cross-datasource dependencies
- `Gestione Utenti` depends on a customer id from `/customers/v2/customer`, then fans out to `/users/v2/user?customer_id=…` (REST → REST).
- `Stato Aziende` depends on both `/customers/v2/customer` and `/customers/v2/customer-state`, combined only at the modal save.
- `Accessi Biometrico` **does not** use `GW interno CDLAN - S` for either read or write — it talks to Postgres directly. This is the page most at risk in the rewrite because its mutation flow is a DB stored function, not an HTTP API.
