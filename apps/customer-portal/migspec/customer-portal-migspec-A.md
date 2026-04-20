# Phase A — Entity-Operation Model

Source: `apps/customer-portal/audit/` (APPLICATION_INVENTORY.md, DATASOURCE_CATALOG.md, PAGE_AUDITS.md, FINDINGS.md) cross-checked against `docs/mistra-dist.yaml` and `docs/mistradb/mistra_customers.json`.

Scope rule for this migration: **1:1 port of the wired flows; dead features ignored.**
Ignored (carried in the export, not wired): JSObject1 + JSObject2, every `UNUSED_DATASOURCE` actionList entry (`onSave`, `onToggle`, `onDiscard`, `onToggleCompletion`, `onSaveCompletion`, `Table1primaryColumnsstato_richiestaonCheckChange`), `Api1`, `Query1`, the `Area documentale` page (both modals have no onClick), and the Lenel-enrollment column that is computed but hidden.

## Extracted entities

### Customer
- **Source of truth**: Mistra NG `/customers/v2/customer` (list) + `/customers/v2/customer/{customerId}` (edit). Schema `#/components/schemas/customer`.
- **Fields (from the API contract, not inferred)**:
  - `id: int64`
  - `name: string`
  - `language: enum(it, en)`
  - `group: customer-group` (nested object with `id, name, is_default, is_partner, variables[]`)
  - `state: customer-state` (nested object with `id, name, enabled`)
  - `variables: variable[]` (`resource, access_type`)
- **Operations used by this app**:
  - `list` — `GET /customers/v2/customer?page_number=1&disable_pagination=true` (called from `Stato Aziende` and `Gestione Utenti`; same request).
  - `edit.state` — `PUT /customers/v2/customer/{id}` with `customer-edit` body, in this app always `{ state_id }` (never `group_id`, never `variables`).
- **Relationships**: owns `User/Admin` (via `customer_id`) and `BiometricRequest` (via `customer_id`).
- **Rules preserved**:
  - Only `state_id` is mutated from this app; the server is the authority on allowed state transitions.
- **Open questions**: none for the 1:1 port.

### CustomerState
- **Source of truth**: `/customers/v2/customer-state`. Schema `#/components/schemas/customer-state`.
- **Fields**: `id: int64`, `name: string`, `enabled: bool`.
- **Operations used**: `list` — `GET /customers/v2/customer-state?page_number=1&disable_pagination=true` (options for the edit-state select).
- **Relationships**: referenced by `Customer.state` and by `customer-edit.state_id`.
- **Rules preserved**: list is used verbatim as a lookup (no client-side filtering, no de-duplication).
- **Open questions**: the `enabled` flag exists but is not consulted by the current UI (disabled states are still offered in the select). 1:1 port keeps that behavior.

### User / Admin
- **Source of truth**: `/users/v2/user` (list) + `/users/v2/admin` (create admin). Schemas `#/components/schemas/user-brief` and `#/components/schemas/user-admin-new`.
- **Fields on the list DTO (`user-brief`)**:
  - `id: int64`, `customer_id: int64`, `first_name`, `last_name`, `email`, `enabled: bool`, `role: role-brief{id,name,color}`, `phone?`, `created: string`, `last_login?: string`.
- **Fields on the create DTO (`user-admin-new`)** as composed by the UI:
  - `first_name`, `last_name`, `email`, `customer_id`, `phone`
  - `maintenance_on_primary_email: bool` — derived from checkbox group value `"maintenance"`.
  - `marketing_on_primary_email: bool` — derived from checkbox group value `"marketing"`.
  - `skip_keycloak: bool` — from a switch, default off. Wire contract field.
  - `biometric: bool` — available on the DTO but **not sent by this app**.
- **Operations used**:
  - `list` — `GET /users/v2/user?page_number=1&disable_pagination=true&customer_id={id}` (scoped by customer).
  - `createAdmin` — `POST /users/v2/admin`.
- **Relationships**: belongs to a `Customer`; carries a `role` reference.
- **Rules preserved**:
  - `skip_keycloak=true` suppresses Keycloak provisioning. No client-side role gate in the source app; the server is assumed to enforce (audit SEC concern not addressed by 1:1 port).
  - Notification flags come from a hard-coded checkbox group `["maintenance","marketing"]`; those string keys are part of the wire contract for the form, not of the API.
  - `enabled` is display-only in this app. No edit/delete/disable path.
- **Open questions (not blocking the 1:1 port)**:
  - Are the two checkbox-group values exactly `"maintenance"` and `"marketing"`? (Yes — verbatim from the widget config; 1:1.)
  - Should the `biometric` flag on `user-admin-new` be surfaced? (No — dead in source, skip.)

### BiometricRequest (+ joined context)
- **Source of truth**: direct SQL on `customers.biometric_request` joined with `customers.user_struct`, `customers.customer`, `customers.user_entrance_detail`.
- **Table schema (`customers.biometric_request`, confirmed in `mistra_customers.json`)**:
  - `id: int`, `user_struct_id: int (not null)`, `admin_user_struct_id: int?`
  - `request_type: request_type_enum (not null)`
  - `request_source: request_source_enum (not null)`
  - `request_date: timestamptz default now()`
  - `request_approval_date: timestamptz?`
  - `request_completed: bool default false`
  - `customer_id: int?`
  - plus `notified_dc`, `notified_user` (not used by this page).
- **List shape projected by `GetTableData` (verbatim alias → column)**:
  - `id` ← `br.id`
  - `nome` ← `user_struct.first_name`
  - `cognome` ← `user_struct.last_name`
  - `email` ← `user_struct.primary_email`
  - `azienda` ← `customer.name`
  - `tipo_richiesta` ← `br.request_type` (enum as string)
  - `stato_richiesta` ← `br.request_completed` (boolean; this is the actual type of the wired value)
  - `data_approvazione` ← `br.request_approval_date`
  - `data_richiesta` ← `br.request_date`
  - `is_biometric_lenel` ← `COALESCE(ued.is_biometric, false)` (computed; carried in the row but column is `isVisible=false`). 1:1 port keeps it hidden.
  - Order: `ORDER BY data_richiesta DESC`. No pagination, no filters.
- **Operations used**:
  - `list` — the SELECT above. **Target for migration**: the 1:1 port must replace direct-DB access with a backend endpoint (this is a non-negotiable trust-boundary move, not a design change).
  - `setCompleted(id, completed: bool)` — calls `customers.biometric_request_set_completed(_request_id bigint, _completed boolean)`, confirmed signature from `mistra_customers.json:1732`. Body only updates `request_completed = _completed` and `request_approval_date = now()`. **No hidden side effects** (Lenel sync / notifications live in sibling functions `biometric_request_set_notified_dc` / `biometric_request_set_notified_user`, which this app does not call). The audit's "likely has side effects" concern does not apply.
- **Relationships**: belongs to `Customer` (via `customer_id`) and to `UserStruct` (via `user_struct_id`). Joined with `user_entrance_detail` on `email` for Lenel-enrollment lookup (projected but hidden in the UI).
- **Rules preserved**:
  - `stato_richiesta` is **boolean** end-to-end (SELECT projects a bool, widget type is `checkbox`, stored function second arg is `boolean`). The "ok"/"pending" string form only exists in dead JSObject1 code; **excluded** by the 1:1-minus-dead rule.
  - `is_biometric_lenel` is computed but not displayed. Port keeps the read but keeps the column hidden; it is not exposed in the DTO returned by the new backend unless the expert asks for it.
  - `request_approval_date` is overwritten by the stored function on every call to `setCompleted` — it reflects "last toggled at", not "first approved at".
- **Open questions (flagged; not blocking the port)**:
  - None — the earlier audit concern about stored-function side effects is resolved.

### UserStruct (internal, join-only)
- Not edited or listed by this app; only joined for `first_name`, `last_name`, `primary_email` when building the biometric list.
- No operations. Does not need its own DTO on the new backend; the biometric list DTO carries the flattened fields.

### UserEntranceDetail (internal, join-only)
- Used only to source `is_biometric` for the (currently hidden) `is_biometric_lenel` column. Join key: `email = user_struct.primary_email`.
- No operations.

### Out of scope
- **Category**, **Document**, **Area / Tipologia utenti**: mentioned in the `Area documentale` wireframe. Hard-coded Blue/Green/Red placeholder data, no queries, no handlers. 1:1 port **does not include** this page.
- **Home**: static welcome text; not a domain entity.

## Operation summary (live only)

| Entity | Operation | Source | Kind |
|---|---|---|---|
| Customer | list | `GET /customers/v2/customer?page_number=1&disable_pagination=true` | REST |
| Customer | edit state | `PUT /customers/v2/customer/{id}` body `{state_id}` | REST |
| CustomerState | list | `GET /customers/v2/customer-state?page_number=1&disable_pagination=true` | REST |
| User | list by customer | `GET /users/v2/user?page_number=1&disable_pagination=true&customer_id={id}` | REST |
| Admin | create | `POST /users/v2/admin` body `user-admin-new` | REST |
| BiometricRequest | list | direct SELECT (to be wrapped behind new backend endpoint) | SQL |
| BiometricRequest | setCompleted | `SELECT customers.biometric_request_set_completed($1::bigint, $2::boolean)` (to be wrapped behind new backend endpoint) | SQL function |

## Gaps this phase could not resolve

- None that block entity definition. The remaining open items are UX/placement decisions (Phase B/C) and the operational auth-model question for `gw-int.cdlan.net` (Phase D).
