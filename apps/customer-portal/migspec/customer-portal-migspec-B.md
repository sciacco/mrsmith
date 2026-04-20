# Phase B — UX Pattern Map

One entry per **live** view. Dead pages and widgets are listed once at the end for traceability and not re-specced.

## View: Home
- **User intent**: landing screen after authentication; orient the user inside the admin app.
- **Interaction pattern**: static informational page.
- **Main data shown or edited**: hard-coded Italian welcome text ("Welcome!! Ti trovi nella applicazione che ti permette di gestire i contenuti del Customer Portal").
- **Key actions**: none.
- **Entry and exit points**: default landing on app open; user navigates away via the sidebar.
- **Notes on current vs intended behavior**: 1:1 port preserves this as a minimal landing. No new widgets, no data load, no CTA.

## View: Stato Aziende
- **User intent**: pick a customer from a list and change its lifecycle **state** (active / suspended / …).
- **Interaction pattern**: list + modal editor over the selected row.
- **Main data shown or edited**:
  - Table of customers with columns: `id` (number), `Lingua` (text from `language`), `Ragione sociale` (from `name`), `Tipologia` (computed from `group.name`), `Stato` (computed from `state.name`).
  - Hidden columns carried but not displayed: `group` (raw object), `variables` (raw), `state` (raw object).
- **Key actions**:
  - Row selection — updates the "Aggiorna …" button label to include the selected customer's name.
  - Click `Aggiorna {name}` → open modal `m_edit_customer`.
  - In modal: pick a new state from `edit_customer_state` (options = `GetAllCustomerState` items, `optionLabel=name`, `optionValue=id`).
  - Click `Conferma` → `PUT /customers/v2/customer/{id}` with `{state_id}` → on success refetch list + close modal; on error toast with HTTP status + API message.
- **Entry and exit points**: reached from sidebar; exits back to sidebar nav or stays on the updated list.
- **Notes on current vs intended behavior**:
  - 1:1 port: do **not** redesign the flow into inline editing. Keep list → modal → confirm.
  - Button `Aggiorna …` does not gate on row selection in the DSL (audit couldn't capture `isDisabled` expression cleanly). 1:1 port preserves current observable behavior: button always enabled; clicking with no selection results in `selectedRow.id = undefined`, which the server rejects. No new client-side guard is introduced at this stage.
  - Columns `Tipologia` / `Stato` remain computed unwraps of `group.name` / `state.name`; we do not flatten the DTO (that would be a design change beyond 1:1).

## View: Gestione Utenti
- **User intent**: inspect users for a chosen customer and, optionally, create a new admin user for that customer.
- **Interaction pattern**: master select (customer) → dependent list (users) → modal form (create admin).
- **Main data shown or edited**:
  - `select_customer` — customer picker sourced from `GetAllCustomer` items, `optionLabel=name`, `optionValue=id`.
  - `t_user_list` — table sourced from `GetAllUsersByCustomer`. Columns displayed: `Creato il` (raw `created`), `email`, `Accesso CP abilitato` (checkbox on `enabled`), `Nome`, `Cognome`, `nome ruolo` (computed from `role.name`). Hidden: `customer_id`, `id`, `phone`, `role` (raw).
  - Greeting text: `Ciao {appsmith.user.name || appsmith.user.email}, ...`. The greeting identifies the **operator** (Keycloak-authenticated staff), not the customer's user.
- **Key actions**:
  - Change customer in `select_customer` → `GetAllUsersByCustomer` automatically re-runs (Appsmith binding reactivity). In the rewrite, this is an explicit dependency on the selected id.
  - Click `Nuovo Admin` → open modal `m_new_admin`.
  - Fill modal fields: `Nome`, `Cognome`, `Em@il`, `Telefono` (PHONE_INPUT), `Manutenzioni`/`Marketing` checkbox group, `Non creare account su KC` switch.
  - Click `Crea` → `POST /users/v2/admin` with the body described in Phase A → on success refetch user list + close modal; on error toast with HTTP status + API message.
- **Entry and exit points**: reached from sidebar; modal is the only secondary surface.
- **Notes on current vs intended behavior**:
  - 1:1 port: keep the single-customer scope; do not introduce global search or cross-customer views.
  - On first load `select_customer` has no value, so `GetAllUsersByCustomer` is fetched with an empty `customer_id`. The new implementation must **defer the user fetch until a customer is selected** — this is a correctness patch, not a UX change, because the current behavior is already invalid (the page shows either an empty table or a 400 toast, depending on server).
  - `new_user_button` is disabled until a customer is selected (expert-confirmed).
  - Checkbox-group values must stay exactly `"maintenance"` and `"marketing"` — they are how the form composes the two DTO booleans.
  - Greeting copy is preserved verbatim; source of the name/email is the Keycloak operator identity in the new app (was `appsmith.user` in the source).

## View: Accessi Biometrico
- **User intent**: review biometric-access requests and flip each request's completion flag.
- **Interaction pattern**: flat table with per-row inline toggle + Save/Discard (`EditActions`).
- **Main data shown or edited**:
  - `BiometricData` table bound to the biometric-requests list.
  - Displayed columns (Italian labels preserved verbatim from current config): `nome`, `cognome`, `email`, `azienda`, `tipo_richiesta`, `stato_richiesta` (checkbox, editable), `data conferma` (`new Date(data_approvazione).toLocaleString()`), `data della richiesta` (`new Date(data_richiesta).toLocaleString()`), `Save / Discard` (edit actions).
  - Hidden: `id`, `is_biometric_lenel`.
- **Key actions**:
  - Toggle `stato_richiesta` on a row — enters edit mode for that row; `EditActions1` becomes active.
  - Click Save on the row → `setCompleted(id, stato_richiesta)` → on success refetch list and show toast "Perfetto, stato biometrico cambiato"; on error toast "Qualcosa e' andato storto".
  - Click Discard → row reverts locally (Appsmith built-in; no custom handler).
- **Entry and exit points**: reached from sidebar; no secondary surfaces.
- **Notes on current vs intended behavior**:
  - 1:1 port: keep per-row EditActions flow. Do **not** add batch save, filters, date-range controls, or sort UI.
  - The `onCheckChange` handler on `stato_richiesta` (calls `storeValue('flag', …)` and references `UpdateRequestCompleted.responseMeta.statusCode` in its `catch`) is a defensive no-op in the source: nothing reads `flag` and the referenced query hasn't run yet. 1:1-minus-dead treats this as dead orchestration; the rewrite does not need to reproduce it. Simply entering edit mode is enough.
  - The `howAlert('success')` typo in `EditActions1.onSave` is a real bug: the intended "post-refetch success toast" never fires; only the synchronous `showAlert('Perfetto, stato biometrico cambiato', 'success')` fires. The new app emits the single success toast once, matching the **observed** source behavior (1:1 on what actually runs, not on what was typed).
  - `is_biometric_lenel` remains computed-but-hidden. The new backend endpoint may still include the field in the DTO (harmless) but the UI does not render a column for it.
  - Date formatting stays client-side `toLocaleString()`, preserving the source's locale-dependent output. (Flagging as a presentation gap in Findings but not fixing under 1:1.)

## Dead / skipped views (not re-specced)

- **Area documentale**: fully scaffolded UI (category table, category details, upload modal) with no bound queries and no onClick handlers on `Button2` (Confirm) / `Button4` (Carica). The only query on the page (`Query1 = SELECT * FROM accounting."acct_log" LIMIT 10`) is a scratch read not bound to any widget. Under "1:1 port, dead features ignored", this page is **not part of the rewrite**. Decision on whether to implement it later is a separate product scope.
- **JSObject1 / JSObject2 and all `UNUSED_DATASOURCE` actionList entries on Accessi Biometrico**: parallel, never-wired client-side staging-buffer implementation. Dead. Not ported.
- **Api1 on Accessi Biometrico**: REST action with an empty path. Not ported.

## Cross-view consistency

- Both `Stato Aziende` and `Gestione Utenti` call `GetAllCustomer` with the same parameters. 1:1 port keeps two call sites (no consolidation). Code-level sharing of the API client helper is acceptable; behavior stays per-page.
- Nested objects are unwrapped in the UI on three places: `t_customer_list.customColumn1` (`group.name`), `t_customer_list.customColumn2` (`state.name`), `t_user_list.customColumn1` (`role.name`). The rewrite keeps these as view-layer unwraps; it does not change the server DTO.

## Gaps this phase could not resolve

- Whether any view needs a non-1:1 refinement (filter, pagination, role gating). Deferred by the 1:1 scope.
