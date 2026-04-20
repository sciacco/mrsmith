# Phase D — Integration and Data Flow

## External systems used by the live app

### 1. Mistra NG Internal API (REST)
- **Audit name**: `GW interno CDLAN - S`
- **Base URL (source)**: `https://gw-int.cdlan.net`
- **Spec of record**: `docs/mistra-dist.yaml`
- **Endpoints consumed**:
  - `GET /customers/v2/customer` (Stato Aziende, Gestione Utenti)
  - `GET /customers/v2/customer-state` (Stato Aziende)
  - `PUT /customers/v2/customer/{customerId}` (Stato Aziende)
  - `GET /users/v2/user` (Gestione Utenti)
  - `POST /users/v2/admin` (Gestione Utenti)
- **Auth in source**: none captured in the export; Appsmith injected credentials at runtime.
- **Auth in the new app**: use the existing `backend/internal/platform/arak` client. It performs an OAuth2 client-credentials grant against Keycloak (`ARAK_SERVICE_TOKEN_URL`), caches the bearer token (with a 30s safety margin before expiry), refreshes on 401 (up to 2 retries), and attaches `Authorization: Bearer …` to every request. Config env vars: `ARAK_BASE_URL`, `ARAK_SERVICE_CLIENT_ID`, `ARAK_SERVICE_CLIENT_SECRET`, `ARAK_SERVICE_TOKEN_URL`. Reference consumer: `backend/internal/afctools/gateway.go` (`proxyGatewayPDF`). Public API: `arak.Client.Do(method, path, queryString, body)`.
- **Migration placement**: in the rewrite, the **browser does not call `gw-int.cdlan.net` directly**. The mini-app's Go handlers receive an `*arak.Client` in their dependencies and delegate every REST call to it.

### 2. Mistra PostgreSQL (direct SQL, source only)
- **Audit name**: `mistra`
- **Host (source)**: `10.129.32.20`, schemas `customers` and `accounting` touched.
- **Statements actually used**:
  - `GetTableData`: SELECT over `customers.biometric_request` join `user_struct` join `customer` left join `user_entrance_detail` (see Phase A for full projection).
  - `UpdateRequestCompleted`: `SELECT customers.biometric_request_set_completed($1::bigint, $2::boolean)`.
- **Migration placement**: **gone from the new frontend**. The Go backend owns these queries behind REST endpoints (see API Contract section below). The database itself is unchanged — no schema migration, no new stored functions.

### 3. Out of scope
- `accounting."acct_log"` (via dead `Query1`) — not used. No integration.
- Lenel (physical access control system) — **not integrated**. The UI only reads the `is_biometric` flag from `user_entrance_detail` (a column populated elsewhere). The stored function verified in Phase A (`biometric_request_set_completed`) does not push to Lenel.

## End-to-end user journeys

### Journey 1 — Change a customer's state
1. Operator opens **Stato Aziende** from the sidebar.
2. Frontend dispatches two reads in parallel: `list customers`, `list customer-states`. Both flow through the Go backend to Mistra NG.
3. Operator selects a row → selected customer id + name become available in local UI state.
4. Operator clicks `Aggiorna {name}` → modal opens with the state picker.
5. Operator picks a new state → clicks `Conferma`.
6. Frontend calls `editCustomer(id, { state_id })` → backend → Mistra NG `PUT /customers/v2/customer/{id}`.
7. On 2xx: frontend refetches the customer list, closes the modal. On non-2xx: error toast with HTTP status + API `data.message`.

### Journey 2 — List users for a customer and create an admin
1. Operator opens **Gestione Utenti**.
2. Frontend fetches the customer list. **No user fetch yet** (1:1-plus-correctness: we wait for a customer).
3. Operator picks a customer in `select_customer`.
4. Frontend fetches `/users/v2/user?customer_id={id}&disable_pagination=true` via the backend.
5. Operator optionally clicks `Nuovo Admin` → modal opens.
6. Operator fills: first/last name, email, phone, two notification checkboxes, `Non creare account su KC` switch.
7. On `Crea`: frontend → backend → `POST /users/v2/admin` with the composed body.
8. On 2xx: refetch the user list for the same customer, close the modal. On non-2xx: error toast.

### Journey 3 — Mark biometric requests completed
1. Operator opens **Accessi Biometrico**.
2. Frontend fetches the biometric-request list via the backend (new endpoint — see Contract).
3. Operator toggles `stato_richiesta` on a row → the row enters edit mode (`EditActions` Save/Discard become visible).
4. Operator clicks **Save** → frontend → backend `setCompleted(id, completed)` → backend invokes `customers.biometric_request_set_completed($1, $2)` on Mistra PostgreSQL.
5. On 2xx: refetch the biometric list; single success toast `"Perfetto, stato biometrico cambiato"`. On error: `"Qualcosa e' andato storto"`.

## Background / triggered processes
- **None** observable in the source. No timers, no polling, no webhooks, no cron-like triggers. The app is strictly request-response driven by operator actions.

## Data ownership boundaries

| Data | Owner | Exposed to this app | Exposed to browser? |
|---|---|---|---|
| `customers.customer` | Mistra NG | via REST | **via backend** |
| `customers.customer-state` | Mistra NG | via REST | **via backend** |
| `users` | Mistra NG | via REST | **via backend** |
| `customers.biometric_request` | Mistra PostgreSQL | source app: direct SQL. new app: via backend endpoint | **via backend** |
| `customers.user_struct`, `customers.user_entrance_detail` | Mistra PostgreSQL | source app: joined inside the list query. new app: same join, server-side | **not directly** |
| Mistra NG credentials / gateway auth | infra | transparent in source (Appsmith-injected) | **never** |
| Direct Postgres credentials | infra | transparent in source (Appsmith-injected) | **never** |

## Required backend endpoints (consolidated)

Living in `backend/internal/cpbackoffice/`. Paths follow `docs/API-CONVENTIONS.md` §"Namespacing": public URL is `/api/cp-backoffice/v1/...`; module code registers the `/cp-backoffice/v1/...` form (the `/api` prefix is stripped at `backend/cmd/server/main.go:370`).

| Method | Full URL | Purpose | Upstream | DTO (in → out) |
|---|---|---|---|---|
| GET | `/api/cp-backoffice/v1/customers` | list customers | Mistra NG `GET /customers/v2/customer?page_number=1&disable_pagination=true` | none → `{items: customer[]}` — `customer` per spec (`id, name, language, group{…}, state{…}, variables[]`). |
| GET | `/api/cp-backoffice/v1/customer-states` | list lifecycle states | Mistra NG `GET /customers/v2/customer-state?page_number=1&disable_pagination=true` | none → `{items: customer-state[]}` (`id, name, enabled`). |
| PUT | `/api/cp-backoffice/v1/customers/{id}/state` | change customer state | Mistra NG `PUT /customers/v2/customer/{id}` with `{state_id}` | `{state_id: int}` → Mistra `message`. |
| GET | `/api/cp-backoffice/v1/users?customer_id=…` | list users for a customer | Mistra NG `GET /users/v2/user?customer_id=…&disable_pagination=true` | `customer_id` query → `{items: user-brief[]}`. |
| POST | `/api/cp-backoffice/v1/admins` | create admin user | Mistra NG `POST /users/v2/admin` | `user-admin-new` body → `{id}`. |
| GET | `/api/cp-backoffice/v1/biometric-requests` | list biometric requests | Mistra PostgreSQL (same SELECT as source) | none → `{items: BiometricRequestRow[]}` where `BiometricRequestRow = {id, nome, cognome, email, azienda, tipo_richiesta, stato_richiesta:bool, data_richiesta:timestamptz, data_approvazione:timestamptz?, is_biometric_lenel:bool}`. Order: `data_richiesta DESC`. |
| POST | `/api/cp-backoffice/v1/biometric-requests/{id}/completion` | set `request_completed` | Mistra PostgreSQL `SELECT customers.biometric_request_set_completed($1, $2)` | `{completed: bool}` → `{ok: true}` (or 4xx/5xx with message). |

What is **not** negotiable under 1:1:
- Response field names in `BiometricRequestRow` (frontend consumes them verbatim).
- Boolean type for `stato_richiesta` + `completed`.
- `disable_pagination=true` behavior (full list, no paging).
- Unchanged wire shape for `user-admin-new` (including unused `biometric` / `state` fields that the UI never sets).

## Open operational questions

| Question | Needed to start implementing? | Owner |
|---|---|---|
| Does Mistra NG enforce operator-level permission on `POST /users/v2/admin` with `skip_keycloak=true`? | No for a strict 1:1 port; relevant for a security review | backend / security |

**Resolved in this session**:
- Mistra NG auth mechanism — use `backend/internal/platform/arak` (OAuth2 client credentials via Keycloak). Already productized and consumed by `afctools`.
- App slug and Keycloak role — `cp-backoffice` / `app_cpbackoffice_access`.
- `new_user_button.isDisabled` — disabled until a customer is selected.

## Gaps this phase could not resolve

- Preferred endpoint naming inside `backend/internal/*` — handed off to Phase E / portal-miniapp-generator.
