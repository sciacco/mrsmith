# Fornitori — Application Specification

## Summary

- **Application name**: Fornitori (`mrsmith-fornitori`)
- **Audit source**: `apps/fornitori/AUDIT.md` (Phase 1 reverse-engineering del bundle Appsmith `Fornitori.json.gz`)
- **API reference**: `docs/mistra-dist.yaml` — sezione `arak-provider-qualification` v1
- **Spec status**: ✅ Phase A → E completate, decisioni esperto incorporate. Pronto per `portal-miniapp-generator`.
- **Last updated decisions** (in ordine di risoluzione):
  - Q-A4 — endpoint `/provider/{id}/reference[/{ref_id}]` esistono in live (non documentati nello spec); body include `phone` e `reference_type`.
  - Q-A8 — porting 1:1 della Dashboard **visibile** (3 endpoint nuovi nel backend nostro; Dashboard Copy ignorata).
  - Q-A10 / Q-A12 — nuovo `ARAK_DSN` Postgres per accedere agli schemi `provider_qualifications` e `articles` (DB `arak`).
  - 8 default Phase A confermati (Q-A1, Q-A2, Q-A3, Q-A5, Q-A6, Q-A7, Q-A9, Q-A11).
  - 9 default Phase B confermati (Q-B1 → Q-B9).
  - 4 default Phase C confermati (Q-C1 → Q-C4).
  - 3 default Phase D risolti (Q-D1 TODO, Q-D2 = `arak`, Q-D3 = porta `5189`).

Documenti di lavoro per Phase A/B/C/D: vedi `apps/fornitori/SPEC-A-entities.md`, `SPEC-B-uxmap.md`, `SPEC-C-logic.md`, `SPEC-D-integration.md`. Questo file è la sintesi finale.

---

## Current-State Evidence

### Source pages/views

5 pagine in scope (Dashboard Copy nascosta esclusa):

| Pagina (Appsmith) | Pattern | Ruolo |
| --- | --- | --- |
| Dashboard | Operational landing (3 tile + 3 tabelle scorrelate) | "Cosa devo gestire oggi?" |
| Fornitori | Master-detail con 5 tab condizionali + 5 modali | CRUD provider, qualifica, documenti |
| Impostazioni Qualifica | Doppio CRUD admin (categorie + tipi documento) | Manutenzione catalogo |
| Modalità Pagamenti RDA | Tabella inline-edit single-column | Toggle `rda_available` per metodo di pagamento |
| Articoli - Categorie | Master-detail snello (tabella + form contestuale) | Mapping articolo ERP → categoria di qualifica |

### Source entities and operations

10 entità identificate. Ricapitolate in §[Entity Catalog](#entity-catalog).

### Source integrations and datasources

| # | Sistema | Tipo | Auth | Reuse |
| --- | --- | --- | --- | --- |
| 1 | **Mistra NG / Arak gateway** (`gw-int.cdlan.net`) | REST | OAuth2 client_credentials (`ARAK_*` env già esistenti) | Client Go `backend/internal/platform/arak` |
| 2 | **Arak Postgres** (host `10.129.32.20:5432`, DB `arak`) | Postgres | DSN nuovo `ARAK_DSN` | – (nuovo, riusa pattern di altre mini-app) |
| 3 | **Keycloak** (`nemo.cub-otto.it/realms/cdlan`) | OIDC | JWT validation BE + client `mrsmith-portal` FE | `backend/internal/auth` |

### Known audit gaps or ambiguities (residui dopo le decisioni)

| ID | Topic | Stato |
| --- | --- | --- |
| Q-A4-bis | Forma del response di `GET /provider/{id}` (`refs[]` array vs `ref` singolo) | Da verificare runtime; default operativo: array |
| Q-B4 | Semantica del filtro `category_id` su `GET /document?provider_id&category_id` | Da verificare runtime; uso as-is |
| Q-D1 | Drift schema Arak come rischio di lungo periodo | Voce in `docs/TODO.md` |

---

## Entity Catalog

### Entity: Provider

- **Purpose**: anagrafica fiscale, sede, riferimento qualifica primario, lingua, payment method, stato del workflow di qualifica.
- **Operations**:
  - List: `GET /provider?disable_pagination=true` (Arak)
  - Detail: `GET /provider/{id}`
  - Create: `POST /provider` (no `/draft` — Q-A2)
  - Edit: `PUT /provider/{id}` (payload condizionale per stato)
  - Delete: `DELETE /provider/{id}` (soft, `mode=soft` default)
- **Fields**: `id`, `company_name` (1-100), `state` (DRAFT/ACTIVE/INACTIVE/CEASED — UI espone solo i primi 3 ✓ Q-A1), `default_payment_method` (object `{code, description}`), `vat_number` (2-20), `cf` (2-20), `address` (1-200), `city` (1-100), `postal_code` (1-20), `province` (1-10), `erp_id` (int64, alias "Codice Alyante"), `language` (it|en, default `it`), `country` (ISO 3166-1 alpha-2), `ref` o `refs[]` (vedi sotto).
- **Relationships**: 1:N → Reference (4 tipi); N:M → Category (via Provider Category); 1:N → Document.
- **Constraints and business rules** (replicate client-side, ✓ Q-A3):
  1. Italiani: CF **o** P.IVA obbligatori se `country == 'IT'`.
  2. Italiani: CAP ≥ 5 caratteri se `country == 'IT'`.
  3. Italiani: provincia obbligatoria se `country == 'IT'`.
  4. ERP code obbligatorio per impostare `state` ad ACTIVE/INACTIVE.
  5. Quando `state == 'ACTIVE'`, sono modificabili solo: `ref` qualifica, `default_payment_method`, `skip_qualification_validation` (privilegiato — vedi §[Logic Allocation](#logic-allocation)).
- **Open questions**: Q-A4-bis (shape `refs[]`).

### Entity: Provider Reference

- **Purpose**: contatti del fornitore, 4 tipi.
- **Operations** (endpoint live non documentati nello spec, confermati):
  - Create: `POST /provider/{id}/reference` body `{first_name?, last_name?, email?, phone?, reference_type}`.
  - Edit: `PUT /provider/{id}/reference/{ref_id}` body `{first_name?, last_name?, email?, phone}` — nota: `phone` sempre inviato anche se `''`.
  - **Nessuna delete** (✓ Q-B3).
- **Fields**: `id`, `first_name?`, `last_name?`, `email?`, `phone?`, `reference_type` ∈ {`OTHER_REF`, `ADMINISTRATIVE_REF`, `TECHNICAL_REF`, `QUALIFICATION_REF`}.
- **Relationships**: N:1 → Provider.
- **Constraints**:
  - Tab Dati gestisce solo `QUALIFICATION_REF` (singolo).
  - Tab Contatti gestisce gli altri 3 (esclude QUALIFICATION_REF in add).
  - Bug audit: oggi la qualification ref è popolata da `refs[0]` (loop spezzato in `ShowDetailProvider`) → **fixato nel porting** con `refs.find(r => r.reference_type === 'QUALIFICATION_REF')`.
- **Open questions**: nessuna critica; DTO Go da scrivere a mano (spec incompleto).

### Entity: Service Category (Qualification Category)

- **Purpose**: categoria di qualifica con lista document_type (required + optional).
- **Operations**: CRUD via `/category[/{id}]`.
- **Fields**: `id`, `name`, `document_types[]: {document_type:{id,name}, required:bool}`.
- **Relationships**: N:M → Document Type (con flag required); N:M → Provider (via Provider Category); 1:N → Article (via article_category mapping).
- **Constraints**:
  - "No-overlap": uno stesso `document_type` non può essere required e optional per la stessa categoria. **Bug fixato** ✓ Q-A5: oggi il check c'è ma non blocca il save; nel porting blocca davvero.
  - Soft delete (`mode=soft` default).
- **Open questions**: nessuna.

### Entity: Document Type

- **Purpose**: tipo documento di qualifica (es. DURC, Visura).
- **Operations**: CRUD via `/document-type[/{id}]`.
- **Fields**: `id`, `name`.
- **Relationships**: N:M → Service Category; 1:N → Document.
- **Open questions**: nessuna.

### Entity: Provider Category (associazione Provider × Category)

- **Purpose**: associazione N:M con metadati `status` e `critical`.
- **Operations**:
  - List: `GET /provider/{id}/category?disable_pagination=true`
  - Create: `POST /provider/{provider_id}/category/{category_id}?critical=...`
  - **No edit, no delete** (✓ Q-A6) — endpoint API esiste ma non lo usiamo nel 1:1.
- **Fields**: `category` (object `{id, name}`), `status` ∈ {NEW, QUALIFIED, NOT_QUALIFIED}, `critical` (bool).
- **Relationships**: N:1 Provider, N:1 Service Category.
- **Constraints**:
  - `status` derivato lato Mistra (non scrivibile dalla UI).
  - **Race condition fix** (audit): in add multiplo, oggi i POST partono in parallelo non awaited → fix con `await Promise.all(...)`.
- **Open questions**: nessuna.

### Entity: Document

- **Purpose**: file PDF/altro caricato per qualifica con tipo + scadenza + stato di verifica.
- **Operations**:
  - List per provider: `GET /document?provider_id=X&disable_pagination=true`
  - List per provider+categoria: `GET /document?provider_id=X&category_id=Y` (semantica: vedi Q-B4)
  - Upload: `POST /document` multipart (`file, expire_date, provider_id, document_type_id`)
  - Edit: `PATCH /document/{id}` multipart (`file, expire_date` — entrambi required ✓ Q-B5)
  - Download: `GET /document/{id}/download` (stream pass-through dal backend nostro ✓ Q-C3)
- **Fields**: `id`, `file_id`, `expire_date` (date), `provider_id`, `state` ∈ {PENDING_VERIFY_ALL, PENDING_VERIFY_DOC, PENDING_VERIFY_DATE, EXPIRED, OK} (mostrato grezzo ✓ Q-A7), `document_type` (object), `source` ∈ {INTERNAL, EXTERNAL}, `created_at`, `updated_at`.
- **Constraints**:
  - Multipart vero al posto del trick base64 dell'app Appsmith (cambio di transport, comportamento utente identico).
  - Threshold dashboard "in scadenza" = **30 giorni hard-coded** (calcolato BE).
- **Open questions**: Q-B4 (semantica filtro categoria).

### Entity: Notification

- **Operations**: API `GET /notification?notification_type=...&last_days=...` esiste ma **non usata** (✓ Q-A8 — Dashboard Copy esclusa).
- **Note**: lasciamo l'endpoint per future evoluzioni post-1:1.

### Entity: Country (lookup)

- **Operations**: nessuna chiamata.
- **Note**: ✓ Q-A9 — lista statica FE (ISO 3166-1 alpha-2 hard-coded). Nessun `SELECT FROM country`.

### Entity: Payment Method

- **Purpose**: metodo di pagamento (master + flag `rda_available` per app RDA).
- **Operations** (nuovi endpoint nel nostro backend, niente API Arak):
  - List: `GET /api/fornitori/v1/payment-method` → `SELECT code, description, rda_available FROM provider_qualifications.payment_method`
  - Toggle: `PUT /api/fornitori/v1/payment-method/{code}/rda-available` body `{rda_available: bool}` → `UPDATE … SET rda_available=$1 WHERE code=$2`
- **Fields**: `code` (PK), `description`, `rda_available` (bool).
- **Constraints**: nessun add/delete in UI; toggle gated su ruolo `app_fornitori_readonly` (no write) ✓ Q-B8.
- **Open questions**: nessuna.

### Entity: Article × Category mapping

- **Purpose**: associazione articolo ERP → categoria di qualifica.
- **Operations** (nuovi endpoint nel backend nostro):
  - List: `GET /api/fornitori/v1/article-category` → INNER JOIN `articles.article_category × articles.article × provider_qualifications.service_category` (orfani nascosti ✓ Q-A11)
  - Update: `PUT /api/fornitori/v1/article-category/{article_code}` body `{category_id}` → `UPDATE articles.article_category SET category_id=$1, updated_at=now() WHERE article_code=$2`
- **Fields**: `article_code` (PK), `description`, `category_id`, `category_name` (denormalizzato).
- **Constraints**: gated `app_fornitori_readonly`.
- **Open questions**: nessuna.

---

## View Specifications

### View: Dashboard

- **User intent**: "Cosa devo gestire oggi?" — operatore qualifica vede in un colpo d'occhio drafts, doc in scadenza ≤30gg, categorie da rinnovare.
- **Interaction pattern**: operational landing — 3 KPI tile + 3 tabelle indipendenti; nessun filtro temporale (Q-A8: Dashboard Copy esclusa, niente periodo selezionabile).
- **Main data shown or edited**: read-only.
- **Key actions**:
  - Click riga drafts → naviga a Fornitori `?id_provider=X&tab=Dati`.
  - Click File su tabella documenti → download PDF (stream).
  - Click riga categorie → naviga a Fornitori `?id_provider=X&tab=Qualifica`.
- **Entry / exit points**: entry da portale; exit verso Fornitori o download.
- **Notes**: tile counter = `.length` delle 3 tabelle (no endpoint counter dedicato). Counter "categorie" conta righe `provider×category`, non provider distinct ✓ Q-B1.

### View: Fornitori (Tab 0 — lista)

- **User intent**: trovare un fornitore o creare un nuovo provider.
- **Pattern**: tabella + bottone "Nuovo".
- **Key actions**:
  - `BTN_new_fornitore` → modal con form completo + multi-select categorie + flag critical.
  - Click riga → switch al Tab 1 con quel provider selezionato.

### View: Fornitori (Tab 1 — Dati)

- **User intent**: leggere/modificare anagrafica + stato + payment.
- **Pattern**: form di dettaglio con campi disabilitati condizionalmente (state ACTIVE → lock).
- **Sezioni**:
  1. Anagrafica fiscale (ragione sociale, P.IVA, CF, ERP, lingua).
  2. Sede (paese, provincia, città, indirizzo, CAP).
  3. Contatto qualifica (nome, cognome, email, telefono — singolo `QUALIFICATION_REF`).
  4. Operative (DDL stato, payment method default, switch privilegiato `skip_qualification_validate`).
- **Key actions**: Save (`BTN_edit_provider`) → `PUT /provider/{id}` con payload condizionale per stato. Delete → modal conferma → `DELETE /provider/{id}`.
- **Notes**:
  - Lock ACTIVE: solo `ref`, `default_payment_method`, `skip_qualification_validation` editabili (più cambio stato).
  - Switch `skip_qualification_validate` **nascosto** se l'utente non ha il ruolo `app_fornitori_skip_qualification` ✓ Q-B2.

### View: Fornitori (Tab 2 — Contatti)

- **User intent**: gestire i contatti non-qualifica del fornitore.
- **Pattern**: tabella editabile inline + add-row.
- **Key actions**: inline-edit `PUT .../reference/{ref_id}`; add new row `POST .../reference` con `reference_type` required (esclude QUALIFICATION_REF — gestita in Tab Dati). No delete ✓ Q-B3.

### View: Fornitori (Tab 3 — Qualifica)

- **User intent**: gestire associazioni provider × categoria, vedere documenti per categoria.
- **Pattern**: tabella primaria + tabella secondaria filtrata da selezione.
- **Key actions**:
  - Add categorie (multi-select + critical flag) → loop POST con `Promise.all` (fix race).
  - Selezione riga categoria → mostra documenti filtrati `GET /document?provider_id=X&category_id=Y`.
  - No remove, no toggle critical ✓ Q-A6.

### View: Fornitori (Tab 4 — Documenti Qualifica)

- **User intent**: caricare/aggiornare documenti del fornitore.
- **Pattern**: tabella + 2 modali (upload, edit).
- **Key actions**:
  - Upload `mdl_detailDocument`: tipo + scadenza + file (tutti required) → `POST /document` multipart.
  - Edit `mdl_editDocument`: file + scadenza required ✓ Q-B5 → `PATCH /document/{id}` multipart.
  - Download → stream pass-through.
- **Notes**: label dinamiche "Doc obbligatori: ..." / "Doc facoltativi: ..." derivate da `category.document_types`.

### View: Fornitori (Tab 5 — Storico Modifiche)

- **Notes**: ✓ Q-B6 — **omessa** dal porting. Era hidden e dead code in Appsmith.

### View: Impostazioni Qualifica

- **User intent**: admin manutiene catalogo categorie + tipi documento.
- **Pattern**: doppio pannello CRUD (lista + detail layout naturale ✓ Q-B7, niente più show/hide imperativo).
- **Sezioni**:
  - Categoria: lista, add modal, detail con 2 multi-select (required vs optional).
  - Document type: lista, add modal, detail con form semplice.
- **Notes**:
  - Validazione no-overlap fixata ✓ Q-A5.
  - Read-only se utente in `app_fornitori_readonly` ✓ Q-C1.

### View: Modalità Pagamenti RDA

- **User intent**: admin abilita/disabilita `rda_available` per metodo di pagamento.
- **Pattern**: tabella inline-edit single-column.
- **Key actions**: save inline → `PUT /payment-method/{code}/rda-available`.
- **Notes**: read-only se `app_fornitori_readonly` ✓ Q-B8.

### View: Articoli - Categorie

- **User intent**: associare articolo ERP → categoria di qualifica.
- **Pattern**: master-detail snello (tabella + form contestuale layout naturale ✓ Q-B9).
- **Key actions**: selezione riga → form con DDL category attivo + Save → `PUT /article-category/{code}`.
- **Notes**: orfani nascosti ✓ Q-A11; read-only se `app_fornitori_readonly`.

---

## Logic Allocation

### Backend responsibilities

- Proxy verso Mistra NG via `arakCli` (auth + retry already handled by client).
- 7 nuovi endpoint che colmano le lacune dell'API Mistra:
  - 3 dashboard (`drafts`, `expiring-documents`, `categories-to-review`) — SQL diretto su `provider_qualifications` con threshold 30gg + filtri stato.
  - 2 payment-method (list + toggle `rda_available`) — SQL diretto.
  - 2 article-category (list + update) — SQL diretto.
- Authz primaria (Keycloak role check):
  - `app_fornitori_access` — accesso alla mini-app (precondizione su tutti gli endpoint).
  - `app_fornitori_readonly` — 403 sulle write di Imp.Qualifica, Modalità Pagamenti, Articoli-Categorie.
  - `app_fornitori_skip_qualification` — 403 su `PUT /provider/{id}` se body contiene `skip_qualification_validation: true`.
- Stream pass-through del download documento (✓ Q-C3, no redirect 302).
- Calcolo `days_remaining` per dashboard expiring-documents.
- DTO custom per Reference (endpoint live non documentati nello spec).

### Frontend responsibilities

- ~16 validazioni client-side replicate identiche dalla UI Appsmith ✓ Q-A3 (zod schemas + react-hook-form).
- Filtro qualification ref con `refs.find(r => r.reference_type === 'QUALIFICATION_REF')` (fix bug audit).
- Helper download PDF (`Blob` + `URL.createObjectURL` + `<a download>`) — niente più heuristic base64 vs binary.
- Inline-edit + add-row per Tab Contatti e Modalità Pagamenti.
- Layout naturali (no setVisibility imperativo) per Imp.Qualifica e Articoli-Categorie ✓ Q-B7, Q-B9.
- Toast italiano (`useToast` + `ToastProvider` da `@mrsmith/ui` ✓ Q-C4) con messaggi letterali dell'app Appsmith ("Aggiornamento completato", "Inserimento completato", "Errore nel salvataggio del contatto: …").
- URL come unica source of truth per provider selezionato + tab attiva (`useSearchParams`).
- Hook `useHasRole('app_fornitori_readonly' | 'app_fornitori_skip_qualification')` per disable/hide button + switch privilegiato.

### Shared validation or formatting

- Costanti reference (`OTHER_REF`, `ADMINISTRATIVE_REF`, `TECHNICAL_REF`, `QUALIFICATION_REF` con label IT) in `apps/fornitori/src/lib/reference.ts`.
- Lista paesi ISO 3166 statica in `apps/fornitori/src/lib/countries.ts` ✓ Q-A9.
- Lista province italiane statica (107 elementi) in `apps/fornitori/src/lib/provinces.ts` (oggi inlined nei widget Appsmith).

### Rules being revised rather than ported

| Bug attuale | Nel porting |
| --- | --- |
| `Category.checkSelectDocument*` mostra warning ma non blocca save | **Bloccante** ✓ Q-A5 |
| `main.ShowDetailProvider` popola qualification ref con `refs[0]` | **Filtra per `reference_type`** |
| `main.AddCategoryProvider` POST in parallelo non awaited | **Sequenziale o `Promise.all`** |
| `Dashboard.GetDocumentByIDfile` referenzia widget di Dashboard Copy | **Usa `currentRow.file_id` corretto** |
| `DT_expireDate.defaultDate = "2025-10-17T..."` frozen | **`undefined` o `today + 365gg`** |
| `DDL_payment_method.defaultOptionValue: 320` hard-coded | **Default = `null` o derivato dal provider in edit** |
| File picker base64 string come Text field | **Vero `multipart/form-data` con `Blob`** |
| Authz client-side (`appsmith.user.groups.includes(...)`) bypassabile | **Enforcement BE primario** + echo FE |

---

## Integrations and Data Flow

### External systems and purpose

| Sistema | Direzione | Quando | Auth |
| --- | --- | --- | --- |
| Mistra NG (`gw-int.cdlan.net/arak/provider-qualification/v1`) | BE → ext | CRUD provider/category/document-type/document/provider×category/reference | OAuth2 client_credentials (`ARAK_SERVICE_*`) |
| Postgres `arak` su `10.129.32.20:5432` | BE → ext | Dashboard + payment-method + article-category | DSN `ARAK_DSN` (nuovo) |
| Keycloak `nemo.cub-otto.it/realms/cdlan` | BE ←/→ ext | Validazione JWT + role check | Public client `mrsmith-portal` (FE), JWT (BE) |

### End-to-end user journeys

7 journey mappati in dettaglio in `SPEC-D-integration.md` §2:

1. **J1** — "Cosa devo gestire oggi?": Dashboard → Fornitori (deep-link).
2. **J2** — Onboarding nuovo fornitore: Fornitori → modal full-form → loop add categorie.
3. **J3** — Caricamento documento qualifica: Fornitori Tab Documenti → upload modal.
4. **J4** — Manutenzione catalogo (admin): Imp.Qualifica → CRUD categoria + tipo documento.
5. **J5** — Toggle disponibilità RDA: Modalità Pagamenti → inline-edit.
6. **J6** — Mappatura articolo→categoria: Articoli-Categorie → form contestuale.
7. **J7** — Add reference non-qualifica: Fornitori Tab Contatti → inline / add-row.

### Background or triggered processes

**Nessuno**. L'app è 100% richiesta-utente. Niente cron job, niente webhook receiver, niente coda messaggi.

I cambi di stato `document.state` (PENDING_VERIFY_*, EXPIRED, OK) e `provider_category.status` (NEW, QUALIFIED, NOT_QUALIFIED) sono calcolati lato Mistra — non sotto il nostro controllo.

### Data ownership boundaries

| Schema/tabella | Owner | Nostro accesso |
| --- | --- | --- |
| `provider_qualifications.*` (entità CRUD principali) | Team Arak/Mistra | RW via REST + RO via SQL diretto (dashboard) |
| `provider_qualifications.payment_method` | Team Arak/Mistra | RW via SQL diretto (no API esposta) |
| `articles.article` + `articles.article_category` | Team Arak/Mistra | RW via SQL diretto (no API esposta) |

⚠ Rischio noto: drift schema. Mitigazione: voce in `docs/TODO.md` ✓ Q-D1.

---

## API Contract Summary

### Inbound (FE → backend nostro, prefix `/api/fornitori/v1`)

#### Provider
- `GET /provider`
- `GET /provider/{id}`
- `POST /provider`
- `PUT /provider/{id}` — gating `skip_qualification_validation` se nel body
- `DELETE /provider/{id}`

#### Reference (4 tipi)
- `POST /provider/{id}/reference`
- `PUT /provider/{id}/reference/{ref_id}`

#### Category
- `GET /category`
- `GET /category/{id}`
- `POST /category` — write gated `app_fornitori_readonly`
- `PUT /category/{id}` — write gated
- `DELETE /category/{id}` — write gated

#### Document Type
- `GET /document-type`
- `POST /document-type` — write gated
- `PUT /document-type/{id}` — write gated
- `DELETE /document-type/{id}` — write gated

#### Provider Category
- `GET /provider/{id}/category`
- `POST /provider/{id}/category/{cat_id}?critical=...`

#### Document
- `GET /document?provider_id=X[&category_id=Y]`
- `POST /document` (multipart)
- `PATCH /document/{id}` (multipart)
- `GET /document/{id}/download` (stream)

#### Dashboard (nuovi)
- `GET /dashboard/drafts`
- `GET /dashboard/expiring-documents`
- `GET /dashboard/categories-to-review`

#### Payment Method (nuovi)
- `GET /payment-method`
- `PUT /payment-method/{code}/rda-available` — write gated `app_fornitori_readonly`

#### Article Category (nuovi)
- `GET /article-category`
- `PUT /article-category/{code}` — write gated `app_fornitori_readonly`

### Outbound (backend → ext)

- 18 endpoint Arak documentati in `mistra-dist.yaml` (tutti `provider-qualification/v1`).
- 2 endpoint Arak **non documentati** ma in produzione (POST/PUT reference) — DTO custom.
- 5 query SQL custom su Postgres `arak`:
  - `dashboard.drafts` — `SELECT … FROM provider_qualifications.provider WHERE state='DRAFT'`
  - `dashboard.expiring-documents` — join `document × provider × document_type` con threshold 30gg, calcolo `days_remaining`
  - `dashboard.categories-to-review` — join `provider_category × provider × service_category` con `pc.state IN ('NOT_QUALIFIED','NEW') AND p.state IN ('DRAFT','ACTIVE')`
  - `payment-method.list` + `payment-method.update-availability`
  - `article-category.list` (inner-join, orfani nascosti) + `article-category.update-association`

---

## Constraints and Non-Functional Requirements

### Security or compliance

- **Authz primaria server-side**, mai più solo client-side (tre ruoli Keycloak: `app_fornitori_access`, `app_fornitori_readonly`, `app_fornitori_skip_qualification`).
- Mai chiamate dirette FE → Arak: **tutto passa dal backend nostro** ✓ Q-C2.
- Token Mistra **non esposto al browser**: download = stream pass-through ✓ Q-C3.
- Mantenere isolamento sandbox per il file upload (validazione MIME + size).
- ⚠ Tre tabelle Postgres scritte direttamente dal backend nostro (payment_method, article_category): rischio drift — TODO.

### Performance or scale

- **Carico**: app uso interno operatori qualifica (≤10 utenti concorrenti). No requisiti scale particolari.
- Dashboard SQL: 3 query con threshold/filtri, dimensione attesa <200 righe. No paginazione necessaria.
- Lista provider: paginazione disabilitata (`disable_pagination=true` come oggi). Se cresce >1000, switchare a paginazione lato BE.

### Operational constraints

- Nuovo `ARAK_DSN` da configurare nei deploy (dev + prod).
- Coesistenza con app Appsmith Fornitori durante migration (memory `Kit-Products coexistence`): stessi DB, no schema changes.

### UX or accessibility expectations

- Riferimento canonico: `docs/UI-UX.md` (mandatory per UI work — vedi AGENTS.md).
- Italiano-only (alert, label, validazioni) — mantenuto identico.
- Toast pattern già canonico in `@mrsmith/ui`.
- Tema portale Matrix-styled (project vision).

---

## Open Questions and Deferred Decisions

| ID | Question | Needed input | Decision owner |
| --- | --- | --- | --- |
| Q-A4-bis | `GET /provider/{id}` ritorna `refs[]` array o `ref` singolo? | Verifica runtime + adeguamento DTO Go | Implementatore Phase F (sviluppo BE) |
| Q-B4 | Semantica filtro `category_id` su `GET /document` | Verifica runtime sull'API live | Implementatore Phase F (sviluppo FE Tab Qualifica) |
| Q-D1 | Drift schema Arak come voce in `docs/TODO.md` | Aprire issue + nota TODO | Tech lead in onboarding del nuovo monolith vs schema esterno |
| Q-D2-bis | Credenziali user/pwd per `ARAK_DSN` | Coordinamento con DBA team Arak | DevOps al primo deploy |

Tutte non bloccanti per partire con Phase F (implementazione).

---

## Acceptance Notes

### What the audit proved directly

- Inventario delle 6 pagine Appsmith e dei 92 widget/action.
- Mappa dei 22 endpoint Arak utilizzati (REST) + 13 query SQL dirette.
- 11 regole di business client-side documentate (validazioni provider).
- 5 bug noti dell'app attuale (qualification ref `refs[0]`, race condition add categorie, check no-overlap non bloccante, default date frozen, broken cross-page widget reference per il download).

### What the expert confirmed

- **Endpoint reference live non documentati nello spec**: code JS dell'app condiviso direttamente; body include `phone` + `reference_type`.
- **Dashboard visibile è il target del 1:1**, non Dashboard Copy nascosta. 3 endpoint nuovi nel backend nostro.
- **Direct DB access** (opzione A): nuovo `ARAK_DSN` Postgres, DB `arak`, host `10.129.32.20:5432`. Coerente con audit + Phase A.
- 8 default Phase A confermati in blocco.
- 9 default Phase B confermati in blocco.
- 4 default Phase C confermati in blocco.
- Q-D1, Q-D2 (= `arak`), Q-D3 (= porta `5189`) risolti.

### What still needs validation

- Forma del response `GET /provider/{id}` — `refs[]` array vs `ref` singolo (Q-A4-bis).
- Semantica del filtro `GET /document?category_id=...` (Q-B4).
- Credenziali per `ARAK_DSN` (Q-D2-bis).
- Possibile gating BE delle regole di business oggi solo client-side (es. CF-or-VAT, CAP, ERP-required) — fuori scope 1:1, valutabile post-1:1.

---

## Hand-off

Questo SPEC è pronto per `portal-miniapp-generator`:
- Tutti i pattern UX classificati e le decisioni d'esperto incorporate.
- API contract pubblico (FE → BE) e outbound (BE → ext) definiti.
- Logic allocation chiara (BE primario per authz + 7 endpoint nuovi; FE per validazioni + UX + dead code rimosso).
- Integrations isolate: 3 sistemi esterni, niente background jobs.

Riferimenti per la fase d'implementazione:
- `apps/fornitori/AUDIT.md` — Phase 1 audit (testimone storico).
- `apps/fornitori/SPEC-A-entities.md`, `SPEC-B-uxmap.md`, `SPEC-C-logic.md`, `SPEC-D-integration.md` — fonti di lavoro Phase 2.
- `apps/fornitori/SPEC.md` — questo file, sintesi finale.
- `docs/mistra-dist.yaml` (`arak-provider-qualification` v1) — contratto API esterno.
- `backend/internal/platform/arak` — client Go riusabile.
- `backend/internal/platform/applaunch/catalog.go` — registrazione mini-app.
- `docs/UI-UX.md` — design system (mandatory).
- `docs/IMPLEMENTATION-PLANNING.md` — checklist repo-fit pre-approval del piano.
- `docs/IMPLEMENTATION-KNOWLEDGE.md` — discoveries riusabili (cross-system mappings).
