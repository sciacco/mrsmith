# Fornitori — Appsmith audit

> Reverse-engineering audit of the Appsmith export `apps/fornitori/Fornitori.json.gz`.
> Phase 1 (structural). Hand off to `appsmith-migration-spec` for the product specification.
> Source: app **Fornitori**, slug `fornitori`, schema/server v1.

## 1. Application inventory

- **Application name**: Fornitori
- **Source type**: Appsmith application export (single-file JSON, version `clientSchemaVersion=1`, `serverSchemaVersion=11`)
- **Pages (6)** — *Dashboard Copy is hidden*
  1. **Dashboard** — landing screen with KPI tiles + three operational tables
  2. **Fornitori** — provider master with tabbed detail (data/qualification/contacts/documents)
  3. **Impostazioni Qualifica** — admin: qualification categories + document types
  4. **Modalità Pagamenti RDA** — admin: which payment methods are allowed for RDA (purchase requests)
  5. **Articoli - Categorie** — admin: associate ERP articles to qualification categories
  6. **Dashboard Copy** *(hidden)* — earlier dashboard variant driven by the Arak `notification` API
- **Datasources (2)**
  - `arak_db (nuovo)` — direct PostgreSQL to `10.129.32.20:5432`, schemas `provider_qualifications` and `articles` (READ_WRITE)
  - `Arak (mistra-ng-int)` — REST API at `https://gw-int.cdlan.net`, prefix `/arak/provider-qualification/v1`
- **JSObjects (10)**
  - Dashboard: `Utils`, `providerManager`
  - Fornitori: `main`, `category`, `reference`, `JSObject1` (empty)
  - Impostazioni Qualifica: `Category`, `TypeDocument`
  - Modalità Pagamenti RDA: `ModalitaPagamentiJS`
  - Dashboard Copy: `Utils` (duplicate)
- **92 actions total** (REST + raw SQL + JS triggers).

### Global findings

- **Two parallel data layers** are mixed without a clear boundary. The same domain entities are read via REST (`/arak/provider-qualification/v1/...`) on some pages and via direct SQL on others. The Dashboard, Modalità Pagamenti, Articoli-Categorie pages access PostgreSQL directly, including a **raw `UPDATE`** on `articles.article_category` and on `payment_method.rda_available`.
- **Authorization is enforced from the UI** by inspecting `appsmith.user.groups.includes('Acquisti RDA AFC')`. Used in 6 places (Impostazioni Qualifica, Articoli-Categorie). When this group is present the user is read-only on those screens.
- The app coexists with — and depends on — the Arak Mistra NG Internal API (`docs/mistra-dist.yaml`). It is the same `gw-int.cdlan.net` gateway already proxied through `backend/internal/platform/arak`.
- **Italian-only copy** throughout (alerts, labels, validation).
- **Dashboard Copy** is a hidden, half-finished port of the Dashboard to the Arak `notification` endpoint. The visible Dashboard still uses raw SQL. This divergence is a migration risk — see §5.
- Provincia options are **inlined** as a literal JSON array (107 Italian provinces) inside the widgets `DDL_province` and `DDL_province_edt`. Country options come from `provider_qualifications.country` via SQL.

### Migration risks (high level)

| Risk | Detail |
| --- | --- |
| Direct DB writes from UI | `UpdateAvailability`, `UpdateAssociationArticleCat` execute `UPDATE` against `provider_qualifications` / `articles` from the browser. No backend API. |
| Direct DB reads bypassing API | `provider_draft`, `category_expired`, `provider_document_expiredCopy`, `PaymentList`, `Country`, `PaymentMethod`, `provider_state_draft`, `GetArticlesWithCategory`, `GetActiveCategoryQualification` all SELECT directly. |
| Group-based authorization in UI | `Acquisti RDA AFC` group check is client-side only. Migration must move enforcement to backend RBAC. |
| Skip-qualification-validation switch | `skip_qualification_validate` posts `skip_qualification_validation: <bool>` into PUT `/provider/{id}` — privileged operation visible to everyone today. |
| File download path mismatch | `Dashboard.GetDocumentByIDfile` references `TBL_Document.triggeredRow.ID` (a Dashboard Copy table). The Dashboard's actual file links wire through `Utils.ViewDocument` but the underlying query points at a widget that does not exist on the visible Dashboard. Likely broken. |
| Duplicate Dashboard | Active Dashboard + hidden Dashboard Copy contain near-identical queries with subtle SQL differences. Need to pick one source of truth. |
| Anonymous datasources | Both datasources have `authentication: null`/`authenticationType: null` in the export. Real auth is configured on the live workspace, not in this artifact. |

---

## 2. Page audits

### 2.1 Dashboard

**Purpose** — operational landing page summarizing items requiring action: providers in DRAFT, expired/expiring documents, categories needing requalification.

**Page-load batch** — `category_expired`, `provider_document_expiredCopy`, `provider_draft` (all SQL).

**Widgets**

| Widget | Type | Role |
| --- | --- | --- |
| `Statbox1` ("Documenti scaduti / in scadenza") | STATBOX | Counter, value = `provider_document_expiredCopy.data.length` |
| `Statbox1Copy` ("Fornitori da qualificare") | STATBOX | Counter, value = `provider_draft.data.length` |
| `Statbox1CopyCopy` ("Fornitori con categoria scaduta") | STATBOX | Counter, value = `category_expired.data.length` |
| `Table1` "Fornitori da qualificare" | TABLE | bound to `provider_draft.data`; `Dettagli` icon column → `storeValue('selectedTab','Dati')` then `navigateTo('Fornitori', { id_provider })` |
| `Table2` "Documenti scaduti/in scadenza" | TABLE | bound to `provider_document_expiredCopy.data`; columns include `days_remaining`, `file_expire`, `company_name`; `File` column triggers `Utils.ViewDocument(currentRow.file_id)` |
| `Table3` "Categorie di qualifica da gestire" | TABLE | bound to `category_expired.data`; columns `category_name`, `stato`; navigate column opens Fornitori with `id_provider`, expecting `Qualifica` tab via `selectedTab` |

**Event flow**
- Page load → 3 SQL queries hydrate the 3 tables.
- Stat boxes are pure derivations of `.data.length`.
- Row click on Table1/Table3 navigates to `Fornitori` page with `id_provider` query param and stores a `selectedTab` value (consumed by the `wrapper` Tabs widget on Fornitori).
- Row click on Table2 calls `Utils.ViewDocument(file_id)` which calls `GetDocumentByIDfile` (REST `/document/{id}/download`) and downloads PDF in the browser.

**Hidden logic**
- `GetDocumentByIDfile` uses `{{TBL_Document.triggeredRow.ID}}` — `TBL_Document` lives on **Dashboard Copy**, not Dashboard. On Dashboard the file_id comes from `Table2.customColumn1.onClick` invoking `Utils.ViewDocument(currentRow.file_id)`, but the URL builder still references the wrong widget. Likely a porting bug masked by Appsmith's lazy evaluation (the URL token resolves to undefined or the cross-page reference resolves to the last-rendered Dashboard Copy state).
- `Utils.ViewDocument` does base64 vs raw-binary auto-detection of the PDF payload, builds a Blob, calls `download()`. This is non-trivial business behavior the migration must reproduce.
- Three `STATBOX.IconButton1*` are present but their onClick is empty → cosmetic only.
- `Utils.mapTable` and `providerManager.myFun1/myFun2` are stubs (dead code).

**Domain entities** — `Provider`, `Provider Document`, `Provider Category Qualification`, `Service Category`, `Document Type`.

**Migration notes**
- The 3 SQL queries should become a single backend endpoint `GET /api/fornitori/v1/dashboard` returning `{drafts[], expiringDocs[], categoriesToReview[]}`. Days-remaining computation belongs server-side.
- The PDF download flow is already centralized in JS — port `Utils.ViewDocument` to a shared frontend utility.

---

### 2.2 Fornitori (provider master)

**Purpose** — full CRUD on providers with five conditional tabs.

**Page-load batches (in order)**
1. `Country`, `PaymentMethod` (lookup data, both SQL)
2. `GetCategory`, `GetDocumentType`, `GetProvider` (REST list endpoints)
3. `category.setDocFromCatOBB`, `category.setDocFromCatOPT` (JS, derive lists of mandatory/optional doc types for the selected category)
4. `GetCategoryByIDCopy`, `GetCategoryByProviderID`, `GetDocumentByProviderID`, `GetProviderByID` (provider-detail hydration)
5. `GetDocumentByCategoryProvider`

The provider can be selected either by clicking `TBL_supply` or by deep-linking with `?id_provider=<id>`. The expression `TBL_supply.selectedRow.id || appsmith.URL.queryParams.id_provider` appears in **17 places**.

**Top-level layout** — single `Container1` with a TABS widget named `wrapper`, plus 5 modals: `Modal_new_fornitore`, `mdl_new_category`, `mdl_detailDocument`, `mdl_editDocument`, `mdl_delete_provider`.

**Tabs (visibility expressions in italics)**

| Index | Label | Visible when |
| --- | --- | --- |
| 0 | Dati e Qualifica | always (provider list + new) |
| 1 | Dati | *a row is selected or `id_provider` is in URL* |
| 2 | Contatti | same condition |
| 3 | Qualifica | same condition |
| 4 | Documenti Qualifica | same condition |
| 5 | Storico Modifiche | hard-coded `false` (placeholder, never shown — Table5 lives behind it) |

`onTabSelected` stores the current tab name in `selectedTab` (the Dashboard depends on this when it deep-links with `Dati` / `Qualifica` etc.).

#### Tab 0 — Dati e Qualifica (provider list)
- `BTN_new_fornitore` opens `Modal_new_fornitore` and resets the form via `main.ProviderAddResetField`.
- `TBL_supply` bound to `GetProvider.data.items`; columns: Indirizzo, Codice fiscale, Ragione sociale, Partita IVA, Stato, Cod. Alyante, ERP, Lingua, etc. Row select runs `main.ShowDetailProvider()` and stores `selectedTab='Dati'`.

#### Tab 1 — Dati (provider details / edit)
- 14 input widgets (`*_edt`), bound bidirectionally to `GetProviderByID.data`. Many are `isDisabled` based on `GetProviderByID.data.state == "ACTIVE"` — the **state machine rule**: once a provider is ACTIVE, only contact fields, payment method and the qualification-skip switch can be edited. The `state` itself is editable via `DDL_State`.
- `DDL_State` is hard-coded with three options: ACTIVE/INACTIVE/DRAFT.
- `DDL_country_edt`/`DDL_lingua_edt`/`DDL_payment_method_edit`: lookups; default selected from `GetProviderByID.data` via the standard `((options, ssf) => ...)` Appsmith pattern.
- `BTN_edit_provider` → `main.ProviderEdit()` (validates 12+ rules then calls `EditProvider`).
- `BTN_delete_provider` opens `mdl_delete_provider`; confirm runs `main.ProviderDelete()` which calls `DeleteProvider`, closes modal, navigates to Fornitori (clearing query param), reloads `GetProvider`.
- `skip_qualification_validate` SWITCH controls whether the PUT body includes `skip_qualification_validation`.

#### Tab 2 — Contatti
- `TBL_reference` bound to `GetProviderByID.data.refs`. Reference categories come from JSObject `reference.allCategory` (`OTHER_REF`, `ADMINISTRATIVE_REF`, `TECHNICAL_REF`, `QUALIFICATION_REF`).
- `EditActions1` column wires `onSave` and `onDiscard` (handled inline in the widget config — not visible in the JSObject).
- `TBL_reference.onAddNewRowSave` → `JSObject1.TBL_referenceonAddNewRowSave` (empty stub) — the inline-add path is wired but the JSObject is empty, so the actual save is presumably bound to the table's `onSaveRow`/`onAddNewRow` directly (verify in the live app).
- `reference.addContact()` and `reference.updateReference()` build the body and call `NewReference` / `EditReference`. Categories shown for **add** exclude `QUALIFICATION_REF` (ref.addCategory has only 3 entries) — so the qualification contact can never be added/changed from this tab; it is captured via the Dati tab inputs.

#### Tab 3 — Qualifica (categories assigned to the provider)
- `BTN_add_category` opens `mdl_new_category`. Inside: `MS_category` (multi-select of categories), `CKB_critical` (bool). `BTN_save_addcategory` → `main.AddCategoryProvider()` which loops over selected categories and calls `NewCategoryByProviderID` for each.
- `TBL_categoryProvider` bound to `GetCategoryByProviderID.data.items`. On row select: `main.GetDocumentByCategoryProviderID()` triggers `GetDocumentByCategoryProvider` filtered by provider+category.
- `TBL_documentByCategory` is shown only when `TBL_categoryProvider.selectedRow.ID > 0`.
- Two refresh icon buttons (`IconButton6`/`IconButton7`) re-run the list queries.

#### Tab 4 — Documenti Qualifica
- `TBL_documents` bound to `GetDocumentByProviderID.data.items`. Columns include `expire_date`, `state`, `source`, custom **File** column → `main.ViewDocument(TBL_documents.selectedRow.id)` and an Edit column → opens `mdl_editDocument`.
- `BTN_new_document` opens `mdl_detailDocument` with `DDL_type_document`, `DT_expireDate`, `fpkr_document` (file picker). Save button is **disabled** until file + type + date are present.
- `mdl_editDocument.BTN_save_edit_document` is disabled until file+date present.
- `main.UploadDocument()` POSTs `multipart/form-data` to `/document` (file, expire_date, provider_id, document_type_id). `main.UploadEditDocument()` PATCHes `/document/{id}` with file + expire_date.
- `TBL_categoryProvider_memo` (separate table on this tab) drives derived text labels:
  - `categoryname` = selected row name.
  - `doc_mandatory` = "Doc obbligatori: ..." (computed by `category.setDocFromCatOBB`).
  - `doc_notmandatory` = "Doc facoltativi: ..." (computed by `category.setDocFromCatOPT`).
  - Both helpers call `GetCategoryByIDCopy` (a duplicate of `GetCategoryByID` keyed off the memo table). When no row is selected the JSObject returns the literal `"nessun documento"`.

#### Tab 5 — Storico Modifiche (hidden)
- `Table5` with **hard-coded empty data** `[{Data:"", Categoria:"", Documento:"", log:"", "Effettuato da":""}]`. Placeholder for an audit log that was never wired up.

**Hidden logic / fragile bindings**
- The `TBL_supply.selectedRow.id || appsmith.URL.queryParams.id_provider` pattern is duplicated everywhere; behavior depends on whichever resolves truthy. After delete, `ShowDetailProvider` clears `appsmith.URL.queryParams.id_provider`.
- `ShowDetailProvider` contains **dead code**: the inner loop iterates over `for(const line in dettaglio) ... for(const ref in dettaglio[line])` and reads `ref['reference_type']`, but `dettaglio` is a single object (not array of providers), so `line` enumerates keys of the object and `ref['reference_type']` is `undefined`. Result: the `TXT_*_edt` for the qualification reference are populated by **the widget defaultText bindings to `GetProviderByID.data.refs[0]`** — which always picks the *first* ref regardless of whether it is the qualification one.
- `EditProvider`'s body branch when `state == 'ACTIVE'` truncates the payload to `{ ref, default_payment_method, skip_qualification_validation? }`. Validation in `main.ProviderEdit` is a different, narrower set of rules in the ACTIVE branch.
- `AddCategoryProvider` fires N **parallel** `NewCategoryByProviderID.run()` calls (no `await`) inside the loop, then awaits `GetCategoryByProviderID`. Race condition: the GET may resolve before all POSTs complete.
- `DT_expireDate.defaultDate` is hard-coded `2025-10-17T07:22:00.753Z` — frozen in the export. Same for `DT_expireDateEdit`.
- `DDL_payment_method.defaultOptionValue: 320` — hard-coded code; presumably "Riba 60gg" or similar. Not derived from data.
- `DDL_lingua` defaults to `it`; only two languages defined (`it`, `en`).

**Domain entities** — `Provider`, `Reference` (4 types), `Category` (with mandatory/optional document_types), `Document` (file + expire_date), `Document type`, `Country`, `Payment method`.

**Migration notes**
- Validation in `ProviderAdd` / `ProviderEdit` (~16 rules each) must be moved to the backend; the UI should only echo the API's per-field error messages.
- ACTIVE-state field locking is a real business rule, not a presentation rule. Surface it as part of the provider DTO (e.g. `editable_fields` list) rather than client-side state checks.
- The `id_provider` URL parameter is used as a deep link from the Dashboard. Preserve in the new app.
- The "qualification reference" prefill bug must be replaced with an explicit selection (`refs.find(r => r.reference_type == 'QUALIFICATION_REF')`).
- File picker today base64-encodes file content into `fpkr_document.files[0].data`; the multipart POST sends that string as a `Text` field. The backend must accept the existing format or the migration must change the client to true `multipart/form-data` with a `Blob`.

---

### 2.3 Impostazioni Qualifica

**Purpose** — admin maintenance of the catalog: qualification categories and document types.

**Page-load batch** — `GetCategory`, `GetDocumentType`.

**Widgets**

- Two side-by-side tables: `TBL_category` and `TBL_document_type`.
- `BTN_new_category` opens `Modal_new_category` (input + two MultiSelects: required vs optional document types).
- `BTN_new_type_document` opens `Modal_new_document_type` (single input).
- Selecting a row in `TBL_category` reveals `Container_detail_category` (default hidden via `setVisibility(false)` at load). The detail form is pre-filled by `Category.setDocumentFromCategory`.
- Selecting a row in `TBL_document_type` reveals `Container_detail_typedocument`.
- Save / delete buttons in the detail panels are disabled when `appsmith.user.groups.includes('Acquisti RDA AFC') == true`. Read-only group.

**Event flow**
- Add category: `Category.checkSelectDocumentAddCategory()` validates that no document_type appears in both required and optional lists, then `NewCategory.run({name, document_types[]})`, then `GetCategory.run()`, then close modal.
- Edit category: `Category.checkSelectDocumentEditCategory()` — the request body conditionally includes `name` only if it changed (compared against `TXT_category_selected`); `document_types` always sent. Then `GetCategory.run()` (note: it does **not** hide the detail panel after edit, only after delete).
- Delete category: `Category.deleteCategory()` → `DeleteCategory` then alert + `hideDetailCategory` + reload.
- Document type CRUD mirrors the same pattern via the `TypeDocument` JSObject.

**Hidden logic**
- The "duplicate document_type in required and optional" check has a dead `check` flag — it's set to `false` and never flipped to `true`, so the loop only shows the alert (via `break`) but the save still proceeds. **Bug**: showing the warning does not stop the save.
- Container visibility is managed imperatively via `setVisibility(true|false)`; a row deselection does not auto-hide.
- `TXT_category_selected` is an invisible TEXT widget used as a hidden field to remember the original category name (so that the PUT body can decide whether to include `name`).

**Migration notes**
- Move the "no overlap" rule server-side; surface as a validation error.
- Read-only group check must move to the backend authorization layer (Keycloak role).
- The hidden TEXT widget pattern is a common Appsmith anti-pattern; in the rewrite use component state.

---

### 2.4 Modalità Pagamenti RDA

**Purpose** — admin: toggle each payment method's `rda_available` flag (whether it can be selected in RDA / purchase requests).

**Page-load batch** — `PaymentList` (`SELECT * FROM provider_qualifications.payment_method`).

**Widgets**
- Single `Table1` bound to `PaymentList.data`, with an inline `EditActions1` column wiring:
  - `onSave` → `UpdateAvailability.run({id: Table1.selectedRow.code, newValue: Table1.updatedRow.rda_available})` then `PaymentList.run()`.
  - `onDiscard` → `resetWidget("Table1", true)` then `PaymentList.run()`.

**Hidden logic**
- `UpdateAvailability` is the **only direct UPDATE** in the export, executing `UPDATE provider_qualifications.payment_method SET rda_available = {{this.params.newValue}} WHERE code = {{this.params.id}}`.
- The `ModalitaPagamentiJS` JSObject (`pendingRow`, `setPendingRow`, `savePending`) is wired but **not bound** to any widget event in this export — defines an unused alternative save path.

**Migration notes**
- Wrap the update in a backend endpoint that enforces the admin role.
- The inline-edit save lives in the column's `onSave` (table v2 EditActions). The new app should use the same in-place edit pattern.

---

### 2.5 Articoli - Categorie

**Purpose** — admin: associate ERP articles to qualification categories.

**Page-load batch** — `GetActiveCategoryQualification`, `GetArticlesWithCategory` (both SQL, joining `articles.article_category` with `articles.article` and `provider_qualifications.service_category`).

**Widgets**
- `TBL_article_category` (article code, description, current category name).
- Below it `CTN_article_category` (initially hidden) reveals on row click:
  - `DDL_articles` (disabled, prefilled with the selected article code) — chosen for visual confirmation.
  - `DDL_category` (active category list).
  - `BTN_save` runs `UpdateAssociationArticleCat` then alert + reload.
  - `BTN_reset` hides the container.
- Save and reset are `isDisabled` for the `Acquisti RDA AFC` group.

**Hidden logic**
- `UpdateAssociationArticleCat` runs `UPDATE articles.article_category SET category_id={{DDL_category.selectedOptionValue}}, updated_at=now() WHERE article_code={{DDL_articles.selectedOptionValue}}` — direct SQL.
- The article list is the result of an inner JOIN, so articles **without** an existing category mapping never appear → can't fix orphans from this UI.

**Migration notes**
- Replace the SQL UPDATE with a backend `PUT /articles/{code}/category`.
- Decide if orphan / unassociated articles must be exposed (likely yes).

---

### 2.6 Dashboard Copy *(hidden)*

**Purpose** — alternative landing page driven entirely by the Arak `notification` REST API instead of raw SQL.

**Page-load batch** — `GetNotificationCatStatusChange`, `GetNotificationDocumentExpire`, `GetNotificationDraftProvider`.

**Widgets**
- `DDL_filterperiod` (7/30/60/90 giorni; default 30) → `onOptionChange` runs `Utils.setFilterPeriod()` which re-runs the three notification queries.
- Three tables: `TBL_Document` (DOCUMENT_EXPIRATION), `TBL_provider` (PROVIDER_DRAFT), `TBL_category` (PROVIDER_CATEGORY_STATE_CHANGE).

**Hidden logic / divergences from active Dashboard**
- The active Dashboard reads SQL with no period filter. Dashboard Copy adds a `last_days` filter via the Arak API.
- `category_expired` here lacks the `state in('DRAFT','ACTIVE')` filter present in the active Dashboard's version.
- Counters and "fornitori attivi" stats are absent.

**Migration notes**
- Treat as a *requirements signal*: the team intends to migrate the Dashboard from raw SQL to the Arak notification endpoint, with a period filter. Plan for that as the target design.

---

## 3. Datasource & query catalog

### 3.1 REST endpoints (Arak Mistra NG, base `https://gw-int.cdlan.net/arak/provider-qualification/v1`)

| Query | Method | Path | Inputs | Used by | Migration target |
| --- | --- | --- | --- | --- | --- |
| `GetProvider` | GET | `/provider` | `disable_pagination=true` | Fornitori list | Backend list endpoint |
| `GetProviderByID` | GET | `/provider/{id}` | id from row or URL | Fornitori detail | Backend detail endpoint |
| `NewProvider` | POST | `/provider` | full provider body incl. nested `ref` | Modal_new_fornitore | Backend create |
| `EditProvider` | PUT | `/provider/{id}` | conditional payload by state | Tab Dati | Backend update |
| `DeleteProvider` | DELETE | `/provider/{id}` | – | mdl_delete_provider | Backend delete |
| `GetActiveProvider` | GET | `/provider?state=ACTIVE` | – | Dashboard counters (also Dashboard Copy) | Likely fold into a dashboard endpoint |
| `GetCategory` | GET | `/category` | – | Multiple pages | Backend list |
| `GetCategoryByID` / `GetCategoryByIDCopy` | GET | `/category/{id}` | id from selected row | Tab Qualifica + Documenti Qualifica | Backend detail |
| `NewCategory` | POST | `/category` | `{name, document_types[]}` | Modal_new_category (Imp.) | Backend create |
| `EditCategory` | PUT | `/category/{id}` | `{name?, document_types}` | Imp. detail panel | Backend update |
| `DeleteCategory` | DELETE | `/category/{id}` | – | Imp. detail panel | Backend delete |
| `GetCategoryByProviderID` | GET | `/provider/{id}/category` | – | Tab Qualifica | Backend list |
| `NewCategoryByProviderID` | POST | `/provider/{provider_id}/category/{category_id}?critical=...` | params | mdl_new_category | Backend create |
| `GetDocumentType` | GET | `/document-type` | – | Multiple | Backend list |
| `NewDocumentType`/`EditDocumentType`/`DeleteDocumentType` | POST/PUT/DELETE | `/document-type[/{id}]` | – | Imp. Qualifica | Backend CRUD |
| `GetDocumentByProviderID` | GET | `/document?provider_id=...` | provider id | Tab Documenti | Backend list |
| `GetDocumentByCategoryProvider` | GET | `/document?provider_id=...&category_id=...` | provider+category | Tab Qualifica | Backend list |
| `GetDocumentByIDfile` | GET | `/document/{id}/download` | file id | Dashboard, Tab Documenti | Backend file download (proxy bytes) |
| `NewDocQualification` | POST | `/document` | multipart: file, expire_date, provider_id, document_type_id | mdl_detailDocument | Backend upload |
| `EditDocument` | PATCH | `/document/{id}` | multipart: file, expire_date | mdl_editDocument | Backend update |
| `NewReference` | POST | `/provider/{id}/reference` | body | Tab Contatti | Backend create |
| `EditReference` | PUT | `/provider/{id}/reference/{ref_id}` | body | Tab Contatti | Backend update |
| `GetNotificationDraftProvider` | GET | `/notification?notification_type=PROVIDER_DRAFT` | optional `last_days` | Dashboard Copy | Future Dashboard |
| `GetNotificationDocumentExpire` | GET | `/notification?notification_type=DOCUMENT_EXPIRATION` | optional `last_days` | Dashboard Copy | Future Dashboard |
| `GetNotificationCatStatusChange` | GET | `/notification?notification_type=PROVIDER_CATEGORY_STATE_CHANGE&disable_pagination=true` | optional `last_days` | Dashboard Copy | Future Dashboard |

All REST queries can be migrated to call the existing Arak client (`backend/internal/platform/arak`) and re-exposed under `/api/fornitori/v1/...`.

### 3.2 Direct SQL queries (datasource `arak_db (nuovo)`)

| Query | Type | Body (verbatim) | Why it's a problem |
| --- | --- | --- | --- |
| `provider_draft` (Dashboard) | SELECT | `SELECT p.company_name, p.id as id_provider, p.vat_number, p.cf FROM provider_qualifications.provider as p WHERE p.state='DRAFT' ORDER BY p.company_name` | Already exposed by `GET /provider?state=DRAFT` (REST) |
| `category_expired` (Dashboard) | SELECT | join `provider_category` × `provider` × `service_category` where `pc.state in('NOT_QUALIFIED','NEW') AND p.state in('DRAFT','ACTIVE')` | No REST equivalent today; needs new backend aggregate |
| `provider_document_expiredCopy` (Dashboard) | SELECT | join `document` × `provider` × `document_type`, `expire_date <= CURRENT_DATE + INTERVAL '30 day'`, computes `days_remaining` | No REST equivalent; backend should expose with same threshold |
| `provider_state_draft` (Fornitori) | SELECT | `SELECT company_name, id, vat_number, cf FROM provider_qualifications.provider WHERE state='DRAFT'` | Dead code (no widget binds to it) — verify before removing |
| `Country` (Fornitori) | SELECT | `SELECT * FROM provider_qualifications.country` | Replace with REST or static config (~250 countries) |
| `PaymentMethod` (Fornitori) | SELECT | `SELECT code, description FROM provider_qualifications.payment_method` | Replace with REST list |
| `PaymentList` (Modalità Pagamenti) | SELECT | `SELECT * FROM provider_qualifications.payment_method` | Same — also exposes `rda_available` |
| `UpdateAvailability` (Modalità Pagamenti) | UPDATE | `UPDATE provider_qualifications.payment_method SET rda_available = ?? WHERE code = ??` | **Direct write from UI**; needs backend |
| `GetArticlesWithCategory` (Articoli) | SELECT | join `article_category` × `article` × `service_category` | Needs backend list |
| `GetActiveCategoryQualification` (Articoli) | SELECT | `... WHERE deleted_at IS null ORDER BY name` | Same as `GetCategory` REST minus the soft-deleted filter |
| `UpdateAssociationArticleCat` (Articoli) | UPDATE | `UPDATE articles.article_category SET category_id=??, updated_at=now() WHERE article_code=??` | **Direct write from UI**; needs backend |
| Dashboard Copy `provider_draft` | SELECT | (variant) | Dead — Copy is hidden |
| Dashboard Copy `provider_document_expiredCopy` | SELECT | variant without `state in(...)` filter and missing `days_remaining` | Dead |
| Dashboard Copy `category_expired` | SELECT | variant without `p.state in(...)` filter | Dead |

Cross-database note: the `articles.article_category` table sits in a separate schema from `provider_qualifications` but they share a Postgres instance. The migration should keep that boundary in mind when designing the backend module layout.

### 3.3 JSObject inventory (orchestration)

- **Dashboard / Utils** — `setFilterPeriod` (no-op given Dashboard has no period selector), `ViewDocument(idFile)` (PDF download), `mapTable` (dead code).
- **Dashboard / providerManager** — `openProviderTab(providerId, selectedTab)` (navigate + storeValue); `myFun1`/`myFun2` empty.
- **Fornitori / main** — provider/document/category CRUD orchestration (~12 methods, ~250 lines).
- **Fornitori / category** — `setDocFromCatOBB` / `setDocFromCatOPT` derive mandatory/optional document type lists.
- **Fornitori / reference** — `allCategory` (4 ref types) + `addCategory` (3 ref types, *excludes* qualification); `addContact`, `updateReference`.
- **Fornitori / JSObject1** — empty (binding stub for `TBL_reference.onAddNewRowSave`).
- **Impostazioni Qualifica / Category** — category CRUD, validation, detail panel toggling.
- **Impostazioni Qualifica / TypeDocument** — document-type CRUD.
- **Modalità Pagamenti / ModalitaPagamentiJS** — alternative pending-row save path (unused in this export).
- **Dashboard Copy / Utils** — duplicate of Dashboard Utils (without `mapTable`).

---

## 4. Cross-cutting findings

### Embedded business rules (must move to backend)

1. **State machine of provider** — once `state == 'ACTIVE'`, only contact fields, payment method and `skip_qualification_validation` are editable.
2. **Document expiry threshold** — Dashboard hard-codes 30 days for "documenti in scadenza"; Dashboard Copy lets the user pick 7/30/60/90.
3. **Mandatory/optional document types per category** — encoded as `(document_type_id, required: bool)` rows. Validation today: a single document type cannot appear in both lists for the same category (UI-only rule, with a bug — see §2.3).
4. **Italian providers must declare CF or VAT** — enforced by `ProviderAdd` / `ProviderEdit` only in the UI. CAP must be ≥ 5 chars only when country is `IT`. Province required only when country is `IT`.
5. **Cannot set state ACTIVE/INACTIVE without ERP code** — UI rule in `ProviderEdit`: "Non è possibile impostare lo stato del fornitore attivo o disattivo se non viene censito il codice alyante". Strong indication that ERP linkage is required for accounting.
6. **Reference categories** — `ADMINISTRATIVE_REF`, `TECHNICAL_REF`, `OTHER_REF`, `QUALIFICATION_REF`. Qualification ref is set via the dedicated form fields on Tab Dati (single ref); the others are managed through Tab Contatti.
7. **Categories assignment** — every assignment carries a `critical` boolean.
8. **Skip qualification validation** — privileged switch on the Dati tab; the migration must gate it behind a role.
9. **Authorization** — `Acquisti RDA AFC` Keycloak group → read-only on `Impostazioni Qualifica` and `Articoli - Categorie`.

### Duplication

- `GetCategoryByID` and `GetCategoryByIDCopy` are functionally identical, differing only in which widget's selected row provides the id.
- `Utils.ViewDocument` defined twice (Dashboard + Dashboard Copy).
- Identical `GetActiveProvider` / `GetNotificationDraftProvider` / `GetNotificationCatStatusChange` / `GetNotificationDocumentExpire` defined on both Dashboard and Dashboard Copy.
- Province options inlined twice (`DDL_province`, `DDL_province_edt`).
- Validation logic duplicated between `ProviderAdd` and `ProviderEdit`.

### Security concerns

- **Two raw `UPDATE` statements** executable from the browser (`UpdateAvailability`, `UpdateAssociationArticleCat`). Anyone who can load the page can mutate `rda_available` and `articles.article_category` rows. Server-side authorization must replace the UI gate.
- **Authorization based on `appsmith.user.groups`** — the gate is rendered client-side. Direct datasource access still works regardless of UI state.
- **PDF payload heuristic** — `Utils.ViewDocument` assumes the API returns either base64 or a raw byte string of a PDF. Migrating to a proper `Content-Type: application/pdf` response with `Blob` removes the magic detection.

### Bugs / latent issues

- `Category.checkSelectDocumentAddCategory` and its Edit twin show a warning when a document type appears in both lists but **still proceed** with the save (the `check` flag is never set to true).
- `main.ShowDetailProvider`'s nested `for...in` cannot work as written; the qualification reference always ends up displaying `refs[0]` regardless of type.
- `AddCategoryProvider` fires unawaited POSTs in a loop, then immediately reloads — race condition.
- `Dashboard.GetDocumentByIDfile` references a widget (`TBL_Document.triggeredRow.ID`) that exists only on Dashboard Copy.
- `DT_expireDate.defaultDate` is a frozen timestamp from October 2025 — likely unintentional.

### Backend coverage of today's data layer

Cross-check of the audit's SQL/REST inventory against `docs/mistra-dist.yaml`:

- **Dashboard SQL is fully replaceable by the existing Arak API.** `/arak/provider-qualification/v1/notification` already supports `notification_type ∈ {PROVIDER_DRAFT, DOCUMENT_EXPIRATION, PROVIDER_CATEGORY_STATE_CHANGE}` with a `last_days ∈ {7,30,60,90}` filter (default 30). This is exactly what `Dashboard Copy` consumes today. **The hidden Dashboard Copy is the intended target design**; the active raw-SQL Dashboard is legacy and should not be ported as-is.
- **`payment_method.rda_available` has no Arak endpoint.** `/arak/rda/v1/po/{id}/payment-method` is per-PO and unrelated. Migrating "Modalità Pagamenti RDA" requires either a new endpoint in Mistra/Arak or a new endpoint in this monolith (`backend/internal/...`) that owns the master toggle directly against the Postgres `provider_qualifications.payment_method` table.
- **`articles.article_category` has no Arak endpoint.** `/arak/rda/v1/article` lists articles but exposes no mapping CRUD. Migrating "Articoli - Categorie" requires the same call: new mrsmith endpoint backed by the `articles.article_category` table.
- **`country` lookup has no Arak endpoint.** Either keep reading the `provider_qualifications.country` table from a backend-side lookup, or replace it with a static ISO 3166 source — design choice for the spec phase.
- **`payment_method` listing has no Arak endpoint either.** Today consumed by `DDL_payment_method` (Fornitori) and by Modalità Pagamenti. Both need a backend-side list endpoint.

Implication: the new mini-app cannot be a thin proxy over the Arak client. It must own at least four areas of data access — payment methods (list + rda_available toggle), article-category mappings (list + update), country lookup, and any Dashboard aggregations beyond what `notification` returns.

---

## 5. Recommended next steps

1. Hand this audit to `appsmith-migration-spec` for the product specification (state-machine semantics, RBAC scope, "Storico modifiche" inclusion, notification time-window UX, qualification-reference modeling, payment-method default).
2. Plan the new app per the New App Checklist (`CLAUDE.md`): root `package.json`, Makefile, `applaunch/catalog.go`, `cmd/server/main.go`, `config.go`. Keycloak access role `app_fornitori_access`.
3. Move authorization server-side: replace the client-side `appsmith.user.groups.includes('Acquisti RDA AFC')` gate with a Keycloak role enforced by the backend on the admin endpoints (`Impostazioni Qualifica`, `Articoli - Categorie`, `Modalità Pagamenti RDA`, and the `skip_qualification_validation` switch).
4. Build the new endpoints not covered by Arak: payment methods list + `rda_available` PUT, article-category list + PUT, country list. All directly against Postgres `provider_qualifications` / `articles` schemas.
5. Treat Dashboard Copy as the target Dashboard design (Arak `notification` API + period selector); do not port the raw-SQL Dashboard.

---

*Audit generated 2026-04-25 from `apps/fornitori/Fornitori.json.gz`. Phase 1 only — no React code, no backend implementation. Do not treat the Appsmith JSON as a source format.*
