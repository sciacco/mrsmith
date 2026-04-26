# Fornitori — Phase D: Integration & Data Flow

> Phase D output del workflow `appsmith-migration-spec`.
> Sistemi esterni, journey cross-pagina, processi triggered/automatici, boundary di ownership dei dati.

## Convenzioni

- **Outbound BE**: chiamata che il backend `mrsmith-fornitori` fa verso un sistema esterno.
- **Inbound BE**: chiamata che riceve dal FE (sotto `/api/fornitori/v1/...`) o eventualmente da altre app.
- ❓ = decisione/flusso da chiarire.

---

## 1. Sistemi esterni e datasource

| # | Sistema | Tipo | Scope | Auth | Reuse |
| --- | --- | --- | --- | --- | --- |
| 1 | **Mistra NG / Arak gateway** (`gw-int.cdlan.net`) | REST API | Provider, category, document-type, document, provider×category, reference (live, non documentato), notification (non usato) | OAuth2 client_credentials (`ARAK_SERVICE_*`) — già in `.env`, già in client Go | `backend/internal/platform/arak` (già usato da `budget`, `cpbackoffice`, `quotes`, etc.) |
| 2 | **Arak Postgres** (host `10.129.32.20:5432`, schemi `provider_qualifications` + `articles`) | DB diretto | Dashboard SQL (drafts, expiring docs, categories-to-review), payment-method (list + toggle), article-category (list + update) | DSN `ARAK_DSN` (nuovo — Phase A §1.10) | Stessa istanza già usata da `MISTRA_DSN` ma DB diverso |
| 3 | **Keycloak** (`nemo.cub-otto.it/realms/cdlan`) | OIDC | Token utente per validazione + role check (`app_fornitori_*`) | Client `mrsmith-portal` (FE) + JWT validation (BE) | Già wired in `backend/internal/auth` |

Nessun altro sistema. Niente integrazioni esterne (Slack, email, webhook).

### 1.1 Endpoint Mistra utilizzati

Già elencati in SPEC-A §1 e SPEC-C §4. Riassunto:

- `/arak/provider-qualification/v1/provider` (CRUD + list)
- `/arak/provider-qualification/v1/provider/{id}/category[/{cat_id}]` (list + create)
- `/arak/provider-qualification/v1/provider/{id}/reference[/{ref_id}]` (POST + PUT — endpoint **non documentati nello spec ma in produzione**)
- `/arak/provider-qualification/v1/category` (CRUD + list)
- `/arak/provider-qualification/v1/document-type` (CRUD + list)
- `/arak/provider-qualification/v1/document` (CRUD + list + download multipart)

### 1.2 Tabelle Postgres utilizzate (via `ARAK_DSN`)

Schema `provider_qualifications`:
- `provider` (drafts dashboard)
- `provider_category`, `service_category` (categories-to-review dashboard)
- `document`, `document_type` (expiring-documents dashboard)
- `payment_method` (list + toggle `rda_available`)
- `country` (non più letta dal DB — ora statica FE per Q-A9)

Schema `articles`:
- `article` (lista articoli)
- `article_category` (mapping article × service_category, list + update)

### 1.3 Endpoint Arak NON usati ma disponibili

Per memoria — disponibili nello spec, scelta di non usarli nel porting 1:1:

| Endpoint | Motivo non usato |
| --- | --- |
| `POST /provider/draft` | Q-A2: la UI usa solo `POST /provider`. |
| `GET /provider?usable=...` | Filtro non sfruttato dall'audit. |
| `DELETE /provider/{id}?mode=hard` | UI usa solo soft. |
| `PUT /provider/{provider_id}/category/{category_id}` | Q-A6: nessun toggle critical post-creazione. |
| `GET /document/{id}` (detail) | UI va direttamente in edit. |
| `GET /notification` | Q-A8: Dashboard Copy esclusa. |
| `GET /provider?state=ACTIVE` (`GetActiveProvider`) | Cablato in audit ma non usato a video. |

---

## 2. Cross-view user journeys

L'app è state-light: niente flow multi-step persistente. I journey osservabili sono:

### Journey J1 — "Cosa devo gestire oggi?"

Operatore qualifica → Dashboard → vede 3 tile + 3 tabelle.

```
Dashboard (landing)
├─ Click riga "Fornitori da qualificare" (drafts)
│   └─ → Fornitori?id_provider=X&tab=Dati
│       └─ Edit/promuovi a ACTIVE/elimina
│
├─ Click File su "Documenti in scadenza"
│   └─ Stream PDF download (no nav)
│
└─ Click riga "Categorie da gestire"
    └─ → Fornitori?id_provider=X&tab=Qualifica
        └─ Visualizza/aggiungi categorie + carica documenti
```

Stato di passaggio Dashboard → Fornitori: solo i due query param (`id_provider`, `tab`). Niente `storeValue` Appsmith — `useSearchParams` React.

### Journey J2 — "Onboarding nuovo fornitore"

```
Fornitori (Tab 0 - lista)
├─ BTN_new_fornitore → Modal_new_fornitore
│   ├─ Form anagrafica + reference qualifica + multi-select categorie + critical flag
│   └─ Save: POST /provider → loop POST /provider/{id}/category/{cat_id}?critical=...
│       └─ Reload lista, chiudi modal
│
└─ Click riga creata → Tab Dati con tutti i campi disabilitati tranne quelli editabili in DRAFT
    └─ Eventualmente Edit con state→ACTIVE (richiede ERP code valorizzato)
```

⚠ Race condition in `AddCategoryProvider` (audit) → fix con `Promise.all` (Phase C §1.3).

### Journey J3 — "Caricamento documento di qualifica"

```
Fornitori?id_provider=X
├─ Tab Documenti Qualifica
│   ├─ TBL_categoryProvider_memo: seleziona una categoria → label "Doc obbligatori: ..." mostra cosa va caricato
│   ├─ BTN_new_document → mdl_detailDocument
│   │   ├─ DDL tipo + DT scadenza + file picker (tutti required per save)
│   │   └─ POST /document multipart → reload lista → close modal
│   └─ Click Edit su riga → mdl_editDocument (file + scadenza required)
│       └─ PATCH /document/{id} multipart → reload
```

### Journey J4 — "Manutenzione catalogo qualifica" (admin)

```
Impostazioni Qualifica
├─ TBL_category: lista categorie
│   ├─ BTN_new_category → Modal_new_category (nome + 2 multi-select required/optional)
│   │   └─ Validazione no-overlap (Q-A5 fix) → POST /category → reload
│   └─ Click riga → detail panel con form pre-filled
│       ├─ Save → PUT /category/{id} (name solo se cambiato) → reload
│       └─ Delete → DELETE /category/{id} (soft) → reload + hide detail
│
└─ TBL_document_type: stesso pattern per document type
```

Read-only se ruolo `app_fornitori_readonly` (button disabled FE + 403 BE).

### Journey J5 — "Toggle disponibilità RDA"

```
Modalità Pagamenti RDA
└─ Table inline-edit sul booleano rda_available
    └─ onSave → PUT /payment-method/{code}/rda-available { rda_available }
        └─ Reload lista
```

### Journey J6 — "Mappatura articolo → categoria"

```
Articoli - Categorie
├─ TBL_article_category: lista articoli con mapping corrente (orfani nascosti — Q-A11)
└─ Click riga → form contestuale (article disabled + DDL category)
    └─ Save → PUT /article-category/{code} { category_id } → reload + close form
```

### Journey J7 — "Add reference (non-qualifica)"

```
Fornitori?id_provider=X&tab=Contatti
└─ TBL_reference (filtra fuori QUALIFICATION_REF)
    ├─ Inline-edit row: PUT /provider/{id}/reference/{ref_id} (phone sempre nel body)
    └─ Add new row: POST /provider/{id}/reference (reference_type required, esclude QUALIFICATION_REF)
```

---

## 3. Processi triggered / automatici / temporali

L'audit non documenta cron, scheduled jobs, webhook receiver. **L'app è 100% richiesta-utente.**

Effetti collaterali del backend Mistra che NON gestiamo noi:
- Calcolo automatico `document.state` (PENDING_VERIFY_*, EXPIRED, OK) — è derivato lato Mistra. Quando la data scade, lo `state` cambia. Niente trigger nostro.
- Calcolo `provider_category.status` (NEW/QUALIFIED/NOT_QUALIFIED) — idem, lato Mistra.

Quindi: nessun cron job nel backend `mrsmith-fornitori`, nessun message queue, nessun webhook. Architettura request/response pura.

---

## 4. Boundary di ownership dei dati

| Schema/tabella | Owner originale | Accesso `mrsmith-fornitori` | Pattern |
| --- | --- | --- | --- |
| `provider_qualifications.*` (provider, category, document, ecc.) | Team Arak/Mistra | RW via API REST (default) **+** RO via DB diretto (3 dashboard endpoint) | Doppio path: API per CRUD utente, SQL per aggregazioni dashboard |
| `provider_qualifications.payment_method` | Team Arak/Mistra | RW via DB diretto (no API) | SQL diretto, identico a Appsmith oggi |
| `articles.article` + `articles.article_category` | Team Arak/Mistra | RW via DB diretto (no API) | SQL diretto, identico a Appsmith oggi |

Implicazione operativa:
- Se il team Arak fa migration di queste tabelle, gli endpoint dashboard / payment-method / article-category vanno sistemati in parallelo — **rischio noto**. Possibile mitigazione: contract test sui SELECT (non in scope 1:1).
- ❓ **Q-D1**: il monorepo ha un pattern noto per "schema-coupling alert" (es. integration test che fallisce se lo schema cambia)? Se non c'è, va aperto un TODO. **Default operativo**: aggiungere voce in `docs/TODO.md` per il rischio di drift schema.

---

## 5. Hidden triggers / Appsmith specifics da non portare

Cose che oggi accadono "per magia" Appsmith e vanno esplicitate nel porting:

| Comportamento Appsmith | Nel porting React |
| --- | --- |
| `appsmith.user.groups.includes(...)` | Hook custom `useHasRole('app_fornitori_readonly')` che legge dal token Keycloak |
| `appsmith.URL.queryParams.id_provider` | `useSearchParams()` |
| `storeValue('selectedTab', 'Dati')` + `appsmith.store.selectedTab` | `searchParams.set('tab', 'Dati')` (URL come unica source of truth, niente storage globale) |
| `setVisibility(true|false)` imperative | Layout naturale (Q-B7, Q-B9) o conditional rendering React |
| `resetWidget('Table1', true)` | `queryClient.invalidateQueries(['payment-method'])` + form reset |
| `showAlert('msg', 'success')` | Hook toast (Q-C4: shared se esiste) |
| File picker → `fpkr_document.files[0].data` (base64 string) | `<input type="file" />` + `FormData` + `Blob` reale |
| Default value Appsmith con `((options, ssf) => ...)` | `useForm` + `defaultValues` |
| `TBL_supply.selectedRow.id || queryParams.id_provider` (17 occorrenze) | Una funzione utilitaria `useSelectedProviderId()` con fallback all'URL |
| `DT_expireDate.defaultDate = "2025-10-17T07:22:00.753Z"` (frozen) | Default `today` o `today + 365gg`, nessuna data hard-coded |

---

## 6. Configurazione (env + portale)

### File da modificare per la nuova app (riassunto operativo dal "New App Checklist" CLAUDE.md)

| File | Aggiunte |
| --- | --- |
| `backend/.env` | `ARAK_DSN=postgres://<user>:<pass>@10.129.32.20:5432/<db>?sslmode=disable` |
| `backend/internal/platform/config/config.go` | Campo `ArakDSN string` + `envOr("ARAK_DSN", "")` |
| `backend/cmd/server/main.go` | Init `arakDB *sql.DB` se `cfg.ArakDSN != ""`; import `internal/fornitori`; `fornitori.RegisterRoutes(api, arakCli, arakDB)`; gating su `cfg.ArakDSN`/`arakCli` come fanno altre app |
| `backend/internal/platform/applaunch/catalog.go` | `FornitoriAppID = "fornitori"`, `FornitoriAppHref`, ruoli `app_fornitori_access` + `app_fornitori_readonly` + `app_fornitori_skip_qualification` (?), entry catalog |
| `package.json` (root) | `dev:fornitori` script + concurrently entry (name + color + filter) |
| `Makefile` | `dev-fornitori` target + `.PHONY` |
| `apps/fornitori/` | Vite+React app (workspace `mrsmith-fornitori`, package name) |

❓ **Q-D2**: nome del database Postgres per `ARAK_DSN`. L'audit dice solo "schemi `provider_qualifications` + `articles`" e datasource Appsmith chiamato `arak_db (nuovo)`. Il database stesso sarà `arak`? `arak_db`? Da chiedere al DBA prima del primo deploy. **Default operativo per la spec**: placeholder `<db>`, da risolvere a livello operativo.

❓ **Q-D3**: porta del Vite dev server per la mini-app. Le altre mini-app usano porte 5174, 5175, ... (vedi `CORS_ORIGINS` in `.env`). Quale è libera oggi? Verifica in Phase E quando assemblo.

---

## 7. Sintesi data-flow

### 7.1 Read flow tipico (es. Tab Dati provider)

```
Browser (apps/fornitori)
  GET /api/fornitori/v1/provider/{id}
    ↓ Vite proxy localhost:5173 → localhost:8080
  Backend mrsmith-fornitori
    Authz: token Keycloak ha app_fornitori_access? sì → continua
    arakCli.GetProvider(ctx, id)
      ↓ HTTPS → gw-int.cdlan.net/arak/provider-qualification/v1/provider/{id}
    Mistra NG
      ↓ select da provider_qualifications.provider
    ← provider JSON
  ← provider JSON
← provider JSON, render con react-hook-form
```

### 7.2 Write flow privilegiato (es. PUT con `skip_qualification_validation`)

```
Browser
  PUT /api/fornitori/v1/provider/{id}
  Body: { ..., skip_qualification_validation: true }
    ↓
  Backend
    Authz BE: ha app_fornitori_skip_qualification? no → 403 + toast italiano
                                                  sì →
    arakCli.EditProvider(ctx, id, body)
      ↓
    Mistra NG → DB write
    ← 200
  ← 200
← Toast "Aggiornamento completato"
```

### 7.3 Dashboard flow

```
Browser
  GET /api/fornitori/v1/dashboard/expiring-documents
    ↓
  Backend
    SELECT join document × provider × document_type
      WHERE expire_date <= CURRENT_DATE + INTERVAL '30 day'
    Compute days_remaining
    ← list JSON
  ← list JSON
← .length = tile counter, .map = righe tabella
```

### 7.4 Document download flow (stream)

```
Browser
  GET /api/fornitori/v1/document/{id}/download
    ↓
  Backend
    Authz: ha app_fornitori_access? sì
    arakCli.DownloadDocument(ctx, id)  // ritorna io.ReadCloser + headers
      ↓
    Mistra → application/pdf bytes
    ← stream + Content-Disposition header
  ← stream + Content-Disposition pass-through
← Blob → URL.createObjectURL → <a download>
```

---

## Domande aperte (Phase D)

| ID | Topic | Quesito | Default |
| --- | --- | --- | --- |
| Q-D1 | Schema-coupling alert | ✅ Aggiungere voce in `docs/TODO.md` per drift schema Arak. |
| Q-D2 | Nome DB Arak | ✅ `arak`. DSN: `postgres://<user>:<pass>@10.129.32.20:5432/arak?sslmode=disable`. |
| Q-D3 | Porta Vite | ✅ **5189** (5174-5188 occupate; portal a 5173). |

Tutte risolte. Sblocco Phase E.
