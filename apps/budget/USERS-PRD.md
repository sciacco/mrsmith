# Internal Users Management — PRD

App: `apps/budget` (Budget Management)
Owner: Budget squad
Status: Draft for implementation
Last updated: 2026-05-25

## 1. Summary

Add a new page to the Budget Management mini-app that lets administrators manage the catalog of internal Arak users (the same users that are later assigned to Groups, Cost Centers, and Budget allocations).

Today the Budget app only *consumes* the internal user list (via the read-only `useUsers()` hook in `src/api/shared-queries.ts:11`). Creation, edit and deactivation of an internal user are not available in any portal mini-app, so the operator has to fall back to the legacy Appsmith dashboard. This page closes that gap.

The page is a single full-width table with search and column sorting. Create and edit happen in a modal; deactivation is a destructive action and therefore requires a double confirmation.

## 2. Goals & Non-Goals

### Goals

- Provide a portal-native UI to list, search and sort all internal Arak users.
- Allow creating a new internal user with email, first name, last name and role.
- Allow editing email, first name, last name and role of an existing user.
- Allow deactivating (soft-deleting) an existing user with an explicit double confirmation.
- Reuse the existing Budget UI conventions (toolbar, table card, modal, toast, skeleton, empty state, danger button) already established in `views/gruppi` and `views/centri-di-costo`.
- Reuse the Arak proxy pattern already used by the Budget backend (`backend/internal/budget/handler.go`) — no direct calls from the SPA to `gw-int.cdlan.net`.

### Non-Goals

- Re-enabling a previously deactivated user. The Mistra Internal API exposes only a soft-delete; if a real re-enable endpoint surfaces later, it will be added in a follow-up.
- Bulk operations (multi-select disable, CSV import/export).
- Password / SSO management. Authentication remains Keycloak.
- Role CRUD. Roles are read-only here; they are managed elsewhere.
- Assignment of users to Groups, Cost Centers or Budgets. Those flows already live in the respective views and are unchanged.
- Pagination UI. The list is fetched with `disable_pagination=true`, matching the convention used by `useUsers()`, `useGroups()`, `useCostCenters()`. Client-side search/sort is sufficient at the current scale (tens to low hundreds of internal users).

## 3. Users & Access

- Persona: Budget administrator / HR-finance operator.
- Access role: same role gating as the rest of the Budget app — `app_budget_access` (see `backend/internal/platform/applaunch/catalog.go:70`).
- The new route is wrapped by the existing `AppShell` access check in `apps/budget/src/App.tsx:22`; no new role is introduced.

## 4. Navigation & Route

- Route path: `/users` (within the Budget app, i.e. served under the Budget app's base path).
- Added to `apps/budget/src/routes.tsx` after the existing routes.
- Added to the top tab navigation in `apps/budget/src/App.tsx` (`navItems`), placed last:

  Home · Gruppi · Centri di costo · Voci di costo · **Utenti**

- The page is reachable only by users that already have `app_budget_access`; the existing `AccessNotice` fallback in `App.tsx` covers the unauthorized state.

## 5. UI Specification

The page follows the same visual language as `CentriDiCostoPage` / `GruppiPage` (toolbar + table card + empty / skeleton / upstream-error states), but it is a single full-width list — there is no master/detail side panel.

UI copy is in Italian, matching the rest of the Budget app. Internal identifiers (file/route/component names, types, query keys) stay in English, per the convention used by the other Budget views.

### 5.1 Page layout

```
┌──────────────────────────────────────────────────────────────┐
│  Utenti                                  [ + Nuovo utente ]  │
│  Gestisci gli utenti interni                                 │
│                                                              │
│  [ 🔍 Cerca per nome, cognome, email, ruolo… ]               │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Nome ↕   Cognome ↕   Email ↕   Ruolo ↕   Stato   ··· │  │
│  │ Mario    Rossi       …@…       Admin     ● Attivo  ⋯ │  │
│  │ Anna     Bianchi     …@…       Viewer    ○ Disab.  ⋯ │  │
│  │ …                                                      │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

Components/styles to reuse:

- Toolbar: same pattern as `CentriDiCostoPage.tsx:46-57` (`pageTitle`, `pageSubtitle`, `btnPrimary` with the `+` SVG).
- Table card: `tableCard`, `tableHeader`, `tableBody`, `row`, `rowAccent`, `rowIcon`, `rowName`, `rowChevron` classes from `CentriDiCostoPage.module.css`. Extend with the new columns described below.
- Skeleton, empty state, upstream-auth-failure state: identical to the patterns in `GruppiPage.tsx:37-58` and `CentriDiCostoPage.tsx:60-81`, with copy tailored to "utenti".
- Modals: `Modal` from `@mrsmith/ui` (same component used by `GroupCreateModal`, `CostCenterCreateModal`, `CostCenterDisableConfirm`).
- Form inputs: native `<input>` styled via the existing `input`, `label`, `formGroup`, `actions`, `btnPrimary`, `btnSecondary`, `btnDanger` classes already present in `CentriDiCostoPage.module.css`. Role picker uses the existing `SingleSelect` component from `@mrsmith/ui` (same as the Manager picker in `CostCenterCreateModal.tsx:81`).
- Status indicator: reuse the `statusBadge` + `statusDot` + `statusEnabled` / `statusDisabled` classes used in `CentriDiCostoPage.tsx:110-113`.

### 5.2 Toolbar

- Left: page title "Utenti", subtitle "Gestisci gli utenti interni".
- Right: primary button **`+ Nuovo utente`** — opens the create modal.

### 5.3 Search

- A single text input above the table, placeholder: `Cerca per nome, cognome, email, ruolo…`.
- Width: full row, capped to the toolbar width.
- Behaviour: case-insensitive substring match, client-side, applied to the concatenation of `first_name`, `last_name`, `email`, `role.name`. Diacritics are normalized (`String.prototype.normalize('NFD')` + strip combining marks), matching the convention already used by other portal apps with client-side filtering.
- The search is debounced via `useDeferredValue` (same pattern as `apps/ordini/src/pages/OrderListPage.tsx:23`).
- Search input is hidden in the loading and empty states (nothing to search).

### 5.4 Table columns

| Column      | Field                       | Sortable | Notes                                                  |
|-------------|-----------------------------|----------|--------------------------------------------------------|
| Avatar      | initials of `first_name` `last_name` | no       | Same circle treatment as `memberAvatar` in `CentriDiCostoPage.module.css`. |
| Nome        | `first_name`                | yes      | Default sort, ascending.                               |
| Cognome     | `last_name`                 | yes      |                                                        |
| Email       | `email`                     | yes      | Monospace, truncated with `title` tooltip on overflow. |
| Ruolo       | `role.name`                 | yes      | Plain text chip.                                       |
| Stato       | `enabled` (`Attivo`/`Disabilitato`) | yes      | Badge + dot. Disabled rows have lower opacity (reuse `rowDisabled`). |
| Aggiornato  | `updated`                   | yes      | Localised date (`it-IT`, day + short month + year).    |
| Azioni      | —                           | no       | Right-aligned. See 5.5.                                |

- Sort indicator: caret icon next to the column header, identical to `apps/ordini/src/components/OrdersTable.tsx:18-41` (`header()` helper).
- Clicking the header toggles `asc` → `desc` → resets to default. Only one column is sorted at a time.
- String sort uses `localeCompare(b, 'it', { sensitivity: 'base' })`.
- Default sort: `last_name` ascending. Secondary tiebreak: `first_name` ascending.
- The sort state is held in component state (`useState`). No URL persistence required for v1.

### 5.5 Row actions

A small action area at the end of each row (no separate detail panel, since the page is list-only):

- **Modifica** — secondary button, opens the edit modal pre-populated with the row's data.
- **Disattiva** — danger button. Disabled (greyed) for rows where `enabled === false`. Opens the deactivation confirm modal.

Both buttons follow the same icon + label style used in `CentriDiCostoPage.tsx:248-274`.

A double-click on the row is a shortcut for **Modifica** (mirrors the ordini list double-click pattern, `OrdersTable.tsx:45`).

### 5.6 Empty / loading / error states

- **Loading**: `Skeleton rows={6}` inside `tableBody`.
- **Empty (no users at all)**: empty-state card with icon, title "Nessun utente trovato", subtitle "Crea il primo utente per iniziare".
- **Empty after search**: small inline message above the (empty) tbody: "Nessun utente corrisponde alla ricerca", with a "Pulisci ricerca" link.
- **Upstream auth failure**: same pattern used by the other Budget views — detect via `isUpstreamAuthFailed` (`apps/budget/src/api/errors.ts`) and render the "Servizio temporaneamente non disponibile" empty state.
- **Generic error**: toast via `useToast`; the table area falls back to the empty state with subtitle "Riprova fra qualche istante".

## 6. Modals

All three modals use `Modal` from `@mrsmith/ui`. They follow the exact structure of `CostCenterCreateModal`, `CostCenterEditModal` and `CostCenterDisableConfirm` (form layout, button row at the bottom with `btnSecondary` on the left and the primary/danger button on the right, success → toast → close, error → toast with the API message).

### 6.1 Create user — `UserCreateModal`

- Title: **"Nuovo utente"**.
- Fields:
  - `Nome` — `first_name`, required, plain text, max 255 chars.
  - `Cognome` — `last_name`, required, plain text, max 255 chars.
  - `Email` — `email`, required, must match a simple `^.+@.+\..+$` regex on the client; server-side validation is the source of truth.
  - `Ruolo` — `role_name`, required, `SingleSelect` populated by `useRoles()` (see 7.4). Options are `role.name` for both label and value.
- Submit button: **"Conferma"** (primary). While the mutation is pending: **"Creazione…"** (disabled).
- On success: toast with the server `id` from the response is not needed; show a success toast with copy `Utente creato`. Invalidate the user-list query so the new row appears. Close the modal and reset all fields.
- On error: surface the API error message via toast, exactly as `CostCenterCreateModal.tsx:55-59`.

### 6.2 Edit user — `UserEditModal`

- Title: **"Modifica utente"**.
- Same fields as create. All four fields are pre-populated with the current values of the selected user.
- The Email field is editable: the upstream `arak-int-user-edit` schema allows it (`mistra-dist.yaml:7039`).
- Only the *changed* fields are sent in the PUT body. Unchanged fields are omitted, since `arak-int-user-edit` makes them all optional.
- Submit button: **"Salva"** (primary). Pending: **"Salvataggio…"**.
- On success: toast `Utente aggiornato`. Invalidate the user list query and close.
- On error: toast with the API message.

### 6.3 Deactivate user — double confirmation

The DELETE endpoint is a soft-delete and is the only available off-switch. Because the action is not reversible from this UI, the operator must confirm it twice.

Two-step modal flow (single component `UserDisableConfirm` with internal `step` state):

**Step 1 — Confirmation**

- Title: **"Disattiva utente"**.
- Body:
  ```
  Disattivare {first_name} {last_name} ({email})?

  L'utente perderà l'accesso e verrà nascosto dai selettori
  (gruppi, centri di costo, allocazioni). L'operazione non
  può essere annullata da questa pagina.
  ```
- Buttons:
  - Left: **"Annulla"** (`btnSecondary`) — closes the modal.
  - Right: **"Continua"** (`btnDanger`) — advances to step 2.

**Step 2 — Type-to-confirm**

- Title stays the same.
- Body:
  ```
  Per confermare, scrivi DISATTIVA qui sotto.
  ```
  followed by a single text input. The input value must equal the string `DISATTIVA` (case-sensitive) for the action button to enable. This is a deliberate friction: a second mouse-click is not enough.
- Buttons:
  - Left: **"Indietro"** (`btnSecondary`) — returns to step 1, resets the typed value.
  - Right: **"Disattiva"** (`btnDanger`) — enabled only when the input matches; while pending shows **"Disattivazione…"**.
- On success: toast `Utente disattivato`, invalidate the user list, close.
- On error: toast with the API message; stay on step 2 with the typed value preserved.

When the modal is closed (by either route, ESC, backdrop click, or success), the internal `step` and the typed value are reset.

## 7. Data Layer

### 7.1 Backend proxy routes (`backend/internal/budget/handler.go`)

The Budget Go module already proxies `GET /users-int/v1/user`. Three new proxies are added with the same `proxyToArak` helper and the same `protectBudget` middleware:

| Frontend path (under `/api`)    | Method | Upstream Arak path                       |
|----------------------------------|--------|------------------------------------------|
| `/users-int/v1/user`            | GET    | `/arak/users-int/v1/user`                |
| `/users-int/v1/user`            | POST   | `/arak/users-int/v1/user`                |
| `/users-int/v1/user/{user_id}`  | PUT    | `/arak/users-int/v1/user/{user_id}`      |
| `/users-int/v1/user/{user_id}`  | DELETE | `/arak/users-int/v1/user/{user_id}`      |
| `/users-int/v1/role`            | GET    | `/arak/users-int/v1/role`                |

Notes:

- Routes are added inside `RegisterRoutes` next to the existing `GET /users-int/v1/user` line (`handler.go:41`), using the same `handle(...)` wrapper so the `app_budget_access` role gate is applied.
- `user_id` is forwarded via `url.PathEscape`, identical to the cost-center route at `handler.go:392`.
- No fixture fallback is required for the new endpoints in v1: if the Arak client is nil the proxy will simply not be reachable (matches the current behaviour for create/edit/delete on Groups). Tests can be added if/when fixtures are wanted.
- `mistra-dist.yaml` is the contract source. Relevant operations:
  - `NewArakInternalUser` — POST, body `arak-int-user-new`, returns `id-object` (`mistra-dist.yaml:1862-1885`).
  - `GetAllArakInternalUser` — GET, returns paginated `arak-int-user` (`mistra-dist.yaml:1886-1943`).
  - `EditArakInternalUser` — PUT `/arak/users-int/v1/user/{user_id}`, body `arak-int-user-edit`, returns `message` (`mistra-dist.yaml:1944-1977`). **This is the edit endpoint requested in the brief.**
  - `DeleteArakInternalUser` — DELETE `/arak/users-int/v1/user/{user_id}`, returns `message` (`mistra-dist.yaml:1978-2002`). Documented upstream as a soft delete.
  - `GetAllArakInternalRoles` — GET `/arak/users-int/v1/role`, paginated `arak-int-role` (`mistra-dist.yaml:2003-2055`).

### 7.2 Frontend types (`apps/budget/src/api/types.ts`)

The relevant types already exist:

- `ArakIntUser` (`types.ts:36`).
- `ArakIntUserState` (`types.ts:23`).
- `ArakIntRole` (`types.ts:29`).
- `MessageResponse` (`types.ts:18`).
- `IdResponse` (`types.ts:112`).
- `PaginatedResponse<T>` (`types.ts:10`).

Two new interfaces are added (named to match the OpenAPI schema names, consistent with the rest of the file):

```ts
/** schema: arak-int-user-new */
export interface ArakIntUserNew {
  email: string;
  first_name: string;
  last_name: string;
  role_name: string;
}

/** schema: arak-int-user-edit */
export interface ArakIntUserEdit {
  email?: string;
  first_name?: string;
  last_name?: string;
  role_name?: string;
}
```

### 7.3 Query layer (`apps/budget/src/views/utenti/queries.ts`)

The existing `useUsers()` in `shared-queries.ts` returns only `enabled=true` users (`shared-queries.ts:17`). The management page needs *all* users (active + disabled). A second hook is added next to it, scoped to this view:

```ts
sharedKeys.allUsers = ['budget', 'users', 'all'] as const;
sharedKeys.roles    = ['budget', 'roles']         as const;
```

Hooks introduced in `views/utenti/queries.ts`:

- `useAllUsers()` — `GET /users-int/v1/user?page_number=1&disable_pagination=true` (no `enabled` filter), returns `items: ArakIntUser[]`.
- `useRoles()` — `GET /users-int/v1/role?page_number=1&disable_pagination=true`, returns `items: ArakIntRole[]`.
- `useCreateUser()` — `POST /users-int/v1/user`, body `ArakIntUserNew`, returns `IdResponse`. Invalidates `sharedKeys.allUsers` and `sharedKeys.users`.
- `useEditUser()` — `PUT /users-int/v1/user/{id}`, body `ArakIntUserEdit`, returns `MessageResponse`. Invalidates `sharedKeys.allUsers` and `sharedKeys.users`.
- `useDeleteUser()` — `DELETE /users-int/v1/user/{id}`, returns `MessageResponse`. Invalidates `sharedKeys.allUsers` and `sharedKeys.users`.

All hooks follow the exact shape of the existing `useCreateGroup`, `useEditGroup`, `useDeleteGroup` in `apps/budget/src/views/gruppi/queries.ts:25-70`.

### 7.4 Roles in the modals

- `SingleSelect` options are derived as `roles.map(r => ({ value: r.name, label: r.name }))`.
- While the roles query is loading, the role field shows the select component in its disabled state with placeholder `Caricamento ruoli…`.
- On roles upstream failure, the select is rendered disabled with placeholder `Ruoli non disponibili`, and the modal's submit button is disabled.

## 8. File / Module Layout

New files (one folder, mirroring the structure of `views/gruppi`):

```
apps/budget/src/views/utenti/
├── UtentiPage.module.css
├── UtentiPage.tsx
├── UserCreateModal.tsx
├── UserEditModal.tsx
├── UserDisableConfirm.tsx
└── queries.ts
```

Files modified:

- `apps/budget/src/routes.tsx` — register `{ path: 'users', element: <UtentiPage /> }`.
- `apps/budget/src/App.tsx` — append `{ label: 'Utenti', path: '/users' }` to `navItems`.
- `apps/budget/src/api/types.ts` — add `ArakIntUserNew` and `ArakIntUserEdit` interfaces (see 7.2).
- `backend/internal/budget/handler.go` — register the four new proxy routes inside `RegisterRoutes` (see 7.1).

No changes are required to:

- The Keycloak role catalog (reuses `app_budget_access`).
- `backend/internal/platform/applaunch/catalog.go` (not a new mini-app, just a new page).
- The root `package.json` / `Makefile` (not a new app).
- The shared UI package `@mrsmith/ui`.

## 9. Error Handling

- Network / 5xx / Arak upstream errors: surface `error.message` (or `body.message` if present) via the existing `useToast` infrastructure (`@mrsmith/ui`), matching `CostCenterCreateModal.tsx:55-59`.
- Upstream auth failures (401/403 from Arak): the proxy already translates them via `translateUpstreamAuthFailure` (`handler.go:114`). The page renders the "Servizio temporaneamente non disponibile" state via `isUpstreamAuthFailed(error)`.
- Validation:
  - Client: required fields trimmed; submit disabled while invalid; basic email regex.
  - Server is the source of truth for uniqueness (email already exists, role name unknown, etc.). The server's `message` field is surfaced verbatim in the toast.

## 10. Accessibility

- All inputs in modals have an explicit `<label>` (already the pattern in the Budget modals).
- Column headers are real `<th>` with `aria-sort="ascending|descending|none"`, matching the `OrdersTable` convention.
- Modals are focus-trapped by the shared `Modal` component; the first focusable element receives focus on open.
- Action buttons have `aria-label` when they only show an icon.
- The double-confirmation typed-string is announced to screen readers via the label of the input ("Per confermare, scrivi DISATTIVA").

## 11. Telemetry & Logging

- The Go proxy already logs via `requestLogger` with `component: "budget"` and the operation name (`handler.go:80`). No additional logging is added in v1.
- No frontend analytics events are introduced.

## 12. Out of Scope / Open Questions

- **Re-enable a deactivated user**: not exposed by the current Internal API. If a real endpoint becomes available, the "Disattiva" button on disabled rows would be replaced by "Riattiva" using the same pattern as the cost-center enable flow at `CentriDiCostoPage.tsx:262-274`.
- **Bulk deactivation**: not in v1.
- **Server-side search / pagination**: deferred until the user count actually requires it.
- **Audit log of who created/edited/deactivated whom**: handled upstream by Arak; not surfaced in this UI.
- **Role assignment policy** (which roles a Budget admin is allowed to grant): the dropdown currently shows the entire role list returned by Arak. A whitelist can be introduced later if security requires it.

## 13. Acceptance Criteria

1. A user with `app_budget_access` sees an **Utenti** tab in the Budget app navigation.
2. The page loads, showing all internal users (active and disabled) in a sortable, searchable table.
3. Each column header acts as a sort toggle (`asc` → `desc` → asc) with a visible indicator and proper `aria-sort`.
4. Search filters on `first_name`, `last_name`, `email`, `role.name` (case- and diacritic-insensitive).
5. **Nuovo utente** opens a modal with the four required fields; submitting calls `POST /users-int/v1/user` and, on 200, the new user appears in the table and a success toast is shown.
6. **Modifica** on a row opens a modal pre-filled with current values; submitting calls `PUT /users-int/v1/user/{id}` with only the changed fields and refreshes the table.
7. **Disattiva** opens the two-step confirmation; only after typing `DISATTIVA` does the destructive button enable; on confirm it calls `DELETE /users-int/v1/user/{id}`, the row updates to `Disabilitato` and the **Disattiva** button on that row becomes inactive.
8. Loading, empty, and upstream-auth-failure states match the visual treatment used by `views/gruppi` and `views/centri-di-costo`.
9. `pnpm --filter mrsmith-budget exec tsc --noEmit` passes.
10. The existing Budget pages (Home, Gruppi, Centri di costo, Voci di costo) keep working unchanged.
