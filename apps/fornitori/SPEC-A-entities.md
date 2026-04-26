# Fornitori — Phase A: Entity-Operation Model

> Phase A output del workflow `appsmith-migration-spec`.
> Estratto da `apps/fornitori/AUDIT.md` + verifica contro `docs/mistra-dist.yaml` (sezione `arak-provider-qualification`).
> Scopo: catalogare le entità di dominio, mappare le operazioni effettive sull'API Mistra NG, e isolare i gap che richiedono decisione dell'esperto prima di passare a Phase B.

## Convenzioni

- "Audit" = quello che l'app Appsmith oggi fa (osservato).
- "API spec" = `docs/mistra-dist.yaml` (`arak-provider-qualification` v1).
- ⚠ = divergenza rilevante audit vs API spec.
- ❓ = decisione richiesta all'esperto.

---

## 1. Entità

### 1.1 Provider (`provider`)

**Scopo** — fornitore qualificabile: anagrafica fiscale, sede, riferimento di contatto principale, lingua, pagamento di default, stato del workflow di qualifica.

**Stati** (`provider.state`):
- API spec enum: `DRAFT`, `ACTIVE`, `INACTIVE`, `CEASED`.
- UI `DDL_State` (Tab Dati) hard-coded a 3 valori: `DRAFT`, `ACTIVE`, `INACTIVE`. Manca `CEASED`.
- ❓ **Q-A1**: La UI deve esporre anche `CEASED`? Oggi non è selezionabile dal DDL ma lo stato esiste lato API e i provider in `CEASED` finirebbero comunque nella lista (a meno che il filtro `state` venga sempre applicato).

**Operazioni esposte oggi**:
| Op | Endpoint | Note |
| --- | --- | --- |
| List | `GET /provider?disable_pagination=true` | Tab 0 (lista) — l'audit ignora paginazione e filtri `state`/`usable`. |
| Detail | `GET /provider/{id}` | Tab 1-4 |
| Create | `POST /provider` | Modal_new_fornitore (provider completo) |
| Edit | `PUT /provider/{id}` | Tab Dati (`EditProvider`) |
| Delete | `DELETE /provider/{id}` | mdl_delete_provider (default `mode=soft`) |
| List DRAFT | (oggi SQL: `SELECT … WHERE state='DRAFT'`) | Dashboard widget `Statbox1Copy` + tabella `Table1` |
| Aggregate "active" | (oggi SQL diretto, anche `GET /provider?state=ACTIVE` cablato ma non usato a video) | Dashboard counters |

**Operazioni esposte dall'API ma non sfruttate dalla UI**:
- `POST /provider/draft` (`NewDraftProvider`): crea provider in stato DRAFT con campi minimi (no `default_payment_method`, no fiscal data obbligatoria). Oggi l'app usa `POST /provider` anche per le creazioni in DRAFT.
- `GET /provider?usable=true|false`: filtro non usato.
- `DELETE /provider/{id}?mode=hard`: la UI usa solo soft delete.
- ❓ **Q-A2**: Per il porting 1:1 manteniamo l'unico path di creazione via `POST /provider` (no draft create) o introduciamo il bottone "Crea bozza" come parte del 1:1? **Default proposto: 1:1 → solo `POST /provider`.**

**Campi (da schema `provider`)**:

| Campo | Tipo | Vincoli | UI mappata |
| --- | --- | --- | --- |
| `id` | int | required | `TBL_supply` colonna ID, query string `id_provider` |
| `company_name` | string | 1-100 | `TXT_company_name(_edt)` |
| `state` | enum DRAFT/ACTIVE/INACTIVE/CEASED | required | `DDL_State` (3 valori in UI) |
| `default_payment_method` | object {code,description} | required | `DDL_payment_method(_edt)` (default code 320 hard-coded ⚠) |
| `vat_number` | string | 2-20, opzionale | `TXT_vat_number(_edt)` |
| `cf` | string | 2-20, opzionale | `TXT_cf(_edt)` |
| `address` | string | 1-200 | `TXT_address(_edt)` |
| `city` | string | 1-100 | `TXT_city(_edt)` |
| `postal_code` | string | 1-20 | `TXT_postal_code(_edt)` |
| `province` | string | 1-10 | `DDL_province(_edt)` (107 province italiane inline ⚠) |
| `erp_id` | int64 | opzionale | `TXT_erp_id(_edt)` (alias "Cod. Alyante") |
| `language` | enum it/en | required, default `it` | `DDL_lingua(_edt)` |
| `country` | enum ISO 3166-1 alpha-2 | required | `DDL_country(_edt)` (oggi popolato da SQL `provider_qualifications.country`) |
| `ref` | object `provider-ref` | required | `TXT_first_name_edt`, `TXT_last_name_edt`, `TXT_email_edt` |

**Vincoli di business raccolti dall'audit (`main.ProviderAdd` / `main.ProviderEdit`)**:
- Italian providers (`country == 'IT'`) devono avere CF **o** P.IVA. Province obbligatoria solo se `country == 'IT'`. CAP ≥ 5 char solo se `country == 'IT'`.
- Stato non può passare ad ACTIVE/INACTIVE senza `erp_id` valorizzato ("codice alyante").
- Una volta `state == 'ACTIVE'`, in edit sono modificabili solo: `ref`, `default_payment_method`, e (privilegiato) `skip_qualification_validation`. Tutto il resto è disabilitato.
- ❓ **Q-A3**: Queste regole oggi sono client-side. Per il 1:1 le rifacciamo identiche client-side, oppure ci aspettiamo che il backend Mistra le applichi (mostriamo errore API)? L'API spec non documenta vincoli condizionali (CF-or-VAT, ERP-required-on-state, ecc.); si limita a `minLength`/`maxLength`. **Default proposto: re-implementare le regole client-side con stessi messaggi italiani; eventuale duplicazione lato backend è un nice-to-have post-1:1.**

⚠ **Field divergenza — `provider.ref` singolo vs `refs[]`**
- L'API spec definisce `provider.ref` come **un singolo** `provider-ref` (first_name, last_name, email, email required) — non un array.
- L'audit (Tab Contatti, §2.2 step 132-136) parla di `GetProviderByID.data.refs` (plurale) con 4 categorie (`OTHER_REF`, `ADMINISTRATIVE_REF`, `TECHNICAL_REF`, `QUALIFICATION_REF`) e usa endpoint `POST /provider/{id}/reference` e `PUT /provider/{id}/reference/{ref_id}` — **assenti dall'API spec**.
- Possibilità:
  1. L'API deployata espone `/provider/{id}/reference` ma non è documentata in mistra-dist.yaml (vecchia versione del file).
  2. La feature Tab Contatti è su un endpoint deprecato/dead — la UI fallirebbe in chiamata.
  3. Il `provider.ref` nel response è oggi popolato dal "qualification ref" e gli altri 3 ref sono persistiti su un altro path.
- ❓ **Q-A4 (CRITICA)**: Come tratta oggi Mistra i contatti del fornitore? Hai accesso alla versione corrente dell'API per verificare se `/provider/{id}/reference` (POST/PUT) esiste? Se non esiste, **il porting 1:1 della Tab Contatti è impossibile** e va deciso uno tra: (a) eliminare la tab dal porting (la qualification ref resta, gestita dalla Tab Dati come oggi); (b) attendere che il backend Mistra la implementi; (c) gestirla nel nostro monolith Go come dato proprietario di `mrsmith-fornitori`.

---

### 1.2 Provider Reference / Contact

**Scopo** — contatto associato al fornitore. Un provider ha **N reference** (array `refs[]` nel response — non documentato nello spec, ma confermato dal codice JS attuale e dall'uso UI).

**Tipi** (`reference_type`): `QUALIFICATION_REF`, `ADMINISTRATIVE_REF`, `TECHNICAL_REF`, `OTHER_REF`.
- Tab Dati gestisce solo il `QUALIFICATION_REF` (campi first/last/email accanto agli altri dati anagrafici).
- Tab Contatti gestisce add/edit degli altri 3 (`reference.addCategory` esclude `QUALIFICATION_REF`).

**Bug dell'attuale UI** (audit §2.2 step 159):
- `main.ShowDetailProvider` ha un loop nidificato che non funziona; la qualification ref nei `TXT_*_edt` finisce per essere `refs[0]` (il primo elemento dell'array `refs`, non necessariamente quello con `reference_type='QUALIFICATION_REF'`).

**Operazioni** (verificate dal codice JS dell'app — Q-A4 risolta):
| Op | Endpoint | Body | Note |
| --- | --- | --- | --- |
| Create | `POST /arak/provider-qualification/v1/provider/{provider_id}/reference` | `{first_name?, last_name?, email?, phone?, reference_type}` | `reference_type` obbligatorio; campi vuoti omessi (tranne `phone` su create: omesso solo se vuoto) |
| Edit | `PUT /arak/provider-qualification/v1/provider/{provider_id}/reference/{reference_id}` | `{first_name?, last_name?, email?, phone}` | Su edit `phone` viene **sempre** inviato (anche `''`); altri campi inviati solo se non vuoti |
| Delete | non implementato in UI | – | Nessuna delete-reference esposta oggi |

⚠ **Divergenze tra API live e `mistra-dist.yaml`**:
- Lo schema `provider-ref` nello spec contiene solo `first_name, last_name, email` (con `email` required). L'API live accetta in più: `phone`, `reference_type`. Il response del provider include `refs[]` (array, non `ref` singolo).
- Lo spec **non documenta** gli endpoint `/provider/{id}/reference[/{ref_id}]` ma sono in produzione.
- ⚠ Implicazione per il backend `mrsmith-fornitori`: il client Arak Go (`backend/internal/platform/arak`) andrà esteso con i metodi reference. Tipi DTO da scrivere a mano (lo spec è incompleto su questo punto).

**Campi (schema reale, derivato dal codice JS + uso UI)**:
| Campo | Tipo | Note |
| --- | --- | --- |
| `id` | int | (presunto) chiave naturale del reference |
| `first_name` | string | opzionale |
| `last_name` | string | opzionale |
| `email` | string | opzionale a livello di body invio (la UI però lo richiede in pratica) |
| `phone` | string | opzionale (assente dallo spec, presente in API live) |
| `reference_type` | enum 4 valori | required su create, immutabile su edit (non nel body PUT) |

❓ **Q-A4-bis**: Confermi che il response di `GET /provider/{id}` ritorna **`refs: [...]`** (array) — anziché il `ref` singolo dello spec? Lo desumo dall'audit (`GetProviderByID.data.refs[0]`, "looping `dettaglio[line]`"), va però verificato runtime per non sbagliare il tipo lato Go. **Default operativo: usare `refs[]` array, fallback al singolo `ref` se disponibile.**

---

### 1.3 Service Category / Qualification Category (`category-get`)

**Scopo** — categoria di qualifica (es. "Pulizie industriali", "Manutenzione elettrica"). Ogni categoria pubblica una lista di document_type richiesti / opzionali.

**Operazioni** (tutte coperte da Arak):
| Op | Endpoint | Usata da |
| --- | --- | --- |
| List | `GET /category` | Modal_new_fornitore, mdl_new_category, Tab Qualifica, Imp. Qualifica, Articoli-Categorie (oggi via SQL diretto, sostituibile) |
| Detail | `GET /category/{id}` | Tab Qualifica/Documenti (per ricavare doc_obb / doc_opt) |
| Create | `POST /category` | Modal_new_category (Imp. Qualifica) |
| Edit | `PUT /category/{id}` | Imp. Qualifica detail panel |
| Delete | `DELETE /category/{id}` (soft default) | Imp. Qualifica detail panel |

**Campi**:
| Campo | Tipo | Vincoli |
| --- | --- | --- |
| `id` | int | required |
| `name` | string | required |
| `document_types[]` | array di `{document_type:{id,name}, required:bool}` | required |

**Vincoli di business** (audit §2.3, `Category.checkSelectDocument…`):
- Un `document_type` non può comparire contemporaneamente come required e optional per la stessa categoria. Oggi il check c'è ma è bug-gato (flag `check` mai settato a true → l'alert appare ma il save procede comunque).
- ❓ **Q-A5**: Per il 1:1 manteniamo il bug oppure correggiamo subito? **Default proposto: correggere — fixare un check che oggi è palesemente non funzionante non è un cambio di comportamento intenzionale.** Conferma?

---

### 1.4 Document Type (`document-type-get`)

**Scopo** — catalogo dei tipi documento (es. "DURC", "Visura camerale", "Polizza RC"). Riferito da Category (lista con flag required) e da Document (singolo tipo per documento caricato).

**Operazioni** (coperte da Arak):
| Op | Endpoint | Usata da |
| --- | --- | --- |
| List | `GET /document-type` | Modal_new_category, mdl_detailDocument |
| Create | `POST /document-type` | Imp. Qualifica |
| Edit | `PUT /document-type/{id}` | Imp. Qualifica |
| Delete | `DELETE /document-type/{id}` (soft default) | Imp. Qualifica |

**Campi**: `id`, `name` (required entrambi).

---

### 1.5 Provider Category (associazione Provider × Category, `provider-category-item-get`)

**Scopo** — associazione N:M con metadati: `status` (NEW/QUALIFIED/NOT_QUALIFIED) e `critical` (boolean).

**Operazioni** (coperte da Arak, ma attenzione):
| Op | Endpoint | Usata da |
| --- | --- | --- |
| List | `GET /provider/{id}/category?disable_pagination=true` | Tab Qualifica, Tab Documenti (memo table) |
| Create | `POST /provider/{provider_id}/category/{category_id}?critical=...` | mdl_new_category |
| Edit | `PUT /provider/{provider_id}/category/{category_id}?critical=...` | ⚠ esiste lato API ma **non risulta usato dall'audit** |
| Delete | nessun endpoint nell'API spec; nessun uso nell'audit | – |

**Campi**:
- `category` (oggetto `{id, name}`)
- `status` (NEW/QUALIFIED/NOT_QUALIFIED)
- `critical` (bool)

⚠ Note:
- `status` è derivato lato server. La UI mostra "stato" nella Table3 della Dashboard ("Categorie di qualifica da gestire" = status NEW o NOT_QUALIFIED).
- Non c'è un endpoint per **rimuovere** una categoria associata né per cambiare `critical` post-creazione lato UI (anche se l'API EditProviderCategory lo permetterebbe).
- L'audit segnala race condition in `AddCategoryProvider` (loop di POST non awaited).
- ❓ **Q-A6**: Nel 1:1 vogliamo esporre **rimozione** di una categoria associata e/o **toggle critical** post-creazione (entrambi assenti dalla UI attuale)? **Default proposto: no, 1:1 = nessuna nuova azione utente.**

---

### 1.6 Document (`document-get`)

**Scopo** — file caricato per qualifica (PDF tipico). Ha tipo, scadenza, stato di verifica, file binario.

**Operazioni** (coperte da Arak):
| Op | Endpoint | Usata da |
| --- | --- | --- |
| List per provider | `GET /document?provider_id=X&disable_pagination=true` | Tab Documenti Qualifica |
| List per provider+categoria | `GET /document?provider_id=X&category_id=Y` | Tab Qualifica (sotto-tabella `TBL_documentByCategory`) |
| Detail | `GET /document/{id}` | (non sembra usato a video — si va direttamente all'edit) |
| Upload | `POST /document` (multipart: file, expire_date, provider_id, document_type_id) | mdl_detailDocument |
| Edit | `PATCH /document/{id}` (multipart: file, expire_date) | mdl_editDocument |
| Download | `GET /document/{id}/download` | Dashboard Table2 + Tab Documenti colonna File |
| Delete | nessun endpoint nell'API spec; nessun uso nell'audit | – |

**Campi**:
| Campo | Tipo | Note |
| --- | --- | --- |
| `id` | int | required |
| `file_id` | int | required (id del file binario) |
| `expire_date` | date | required |
| `provider_id` | int | required |
| `state` | enum PENDING_VERIFY_ALL/PENDING_VERIFY_DOC/PENDING_VERIFY_DATE/EXPIRED/OK | required |
| `document_type` | object `{id, name}` | required |
| `source` | enum INTERNAL/EXTERNAL | required |
| `created_at` / `updated_at` | datetime | required |

⚠ Note:
- `state` enum API ha 5 valori; le tabelle UI mostrano la colonna `state` ma l'audit non documenta una mappatura colore/copy. Va verificata in fase B.
- L'audit dice che il client invia il file come `Text` field con stringa base64 (anti-pattern Appsmith). Per il porting 1:1 useremo `multipart/form-data` reale con `Blob` (nessun cambio di comportamento utente, solo correzione del transport).
- ❓ **Q-A7**: Mostrare la `state` del documento nella tabella della Tab Documenti come oggi (testo grezzo) o tradotta in italiano? **Default proposto: testo grezzo come oggi (1:1).** Conferma?

---

### 1.7 Notification (`notification`)

**Scopo** — eventi del dominio qualifica (cambio stato categoria, scadenza documento, nuovo provider in DRAFT). Oggi consumati solo dalla **Dashboard Copy nascosta**.

**API**:
- `GET /notification?notification_type=…&last_days=…&page_number=1&disable_pagination=true`
- Tipi enum: `PROVIDER_CATEGORY_STATE_CHANGE`, `DOCUMENT_EXPIRATION`, `NEW_PROVIDER` (⚠ l'audit dice `PROVIDER_DRAFT`; lo schema `notification` enuncia `NEW_PROVIDER`. Discrepanza tra path enum (`PROVIDER_DRAFT`) e schema enum (`NEW_PROVIDER`).)
- `last_days` enum: 7, 30, 60, 90 (default 30).
- Payload polimorfico: `ProviderCategoryStateChangePayload` o `DocumentExpirationPayload`.

**Stato attuale del porting**:
- L'audit (§5 e §4) raccomanda: **Dashboard Copy = target design**, abbandonare la Dashboard SQL legacy.
- L'utente ha chiesto **porting 1:1**. La domanda è: 1:1 di **cosa**?
  - 1:1 della Dashboard **visibile** oggi (raw SQL, niente filtro periodo).
  - 1:1 della Dashboard Copy **nascosta** (Arak `notification`, filtro 7/30/60/90).
- ✅ **Q-A8 RISOLTA**: portiamo la **Dashboard visibile** (1:1 vero), Dashboard Copy ignorata. Servono 3 endpoint nuovi nel backend `mrsmith-fornitori` (drafts list, expiring docs ≤30gg, categorie da gestire) che replicano gli SQL attuali (vedi §1.10).

---

### 1.8 Country (lookup)

**Scopo** — lista paesi per i DDL Country.

**Stato**:
- API spec: enum statico di ~250 codici ISO direttamente nello schema `country` (e `provider.country`). **Nessun endpoint** che restituisca la lista.
- App attuale: SQL `SELECT * FROM provider_qualifications.country`. Stessa lista, stoccata in DB.

❓ **Q-A9**: Per il 1:1 del DDL: (a) statico nel frontend (lista hard-coded ISO 3166 con label IT/EN), (b) endpoint nuovo `GET /api/fornitori/v1/country` che legge la tabella Postgres come oggi, (c) lookup statico nel backend Go esposto al frontend. **Default proposto: (a) — statico nel frontend.** Aggiunta: l'audit nota che la **provincia** italiana è oggi inlined come array di 107 elementi nei widget DDL_province; manteniamo questo approccio (statico FE).

---

### 1.9 Payment Method (lookup + flag RDA)

**Scopo** — metodo di pagamento (es. "Riba 60gg", "Bonifico 30gg"). Ha attributo `rda_available` che gating la selezionabilità in RDA (un'altra app).

**Stato API**:
- ⚠ **Mistra NG NON espone payment-method come lista master né come endpoint di toggle `rda_available`.** Solo endpoint per-PO (`/arak/rda/v1/po/{id}/payment-method`) che è **non rilevante** per questa app.
- Oggi l'app fa SQL diretto (`SELECT code, description FROM provider_qualifications.payment_method`) e UPDATE diretto (`UPDATE … SET rda_available = … WHERE code = …`).

**Operazioni necessarie**:
| Op | Oggi | Target |
| --- | --- | --- |
| List | SQL `SELECT * FROM payment_method` | **Nuovo endpoint** `GET /api/fornitori/v1/payment-method` (backend Go + Postgres `provider_qualifications.payment_method`) |
| Toggle `rda_available` | SQL `UPDATE … rda_available = ?` | **Nuovo endpoint** `PUT /api/fornitori/v1/payment-method/{code}/rda-available` (backend Go) |

**Campi** (da SQL audit):
- `code` (string, PK)
- `description` (string)
- `rda_available` (bool)

✅ **Q-A10 RISOLTA**: nuovo DSN `ARAK_DSN` (Postgres, host `10.129.32.20:5432`, database del datasource Appsmith `arak_db (nuovo)`, contiene schemi `provider_qualifications` e `articles`). Da aggiungere in:
- `backend/.env` → `ARAK_DSN=postgres://...@10.129.32.20:5432/arak?sslmode=disable` (nome DB da confermare con DBA — probabilmente `arak`).
- `backend/internal/platform/config/config.go` → campo `ArakDSN string` + `envOr("ARAK_DSN", "")`.
- `backend/cmd/server/main.go` → init `arakDB *sql.DB` se `cfg.ArakDSN != ""`, registrato in `RegisterRoutes` di `fornitori`.
- `backend/internal/fornitori/repo/payment_method.go` (TBD) → `SELECT code, description, rda_available FROM provider_qualifications.payment_method` + `UPDATE … SET rda_available=$1 WHERE code=$2`.

---

### 1.10 Article × Category mapping

**Scopo** — associazione M:1 fra articolo ERP e service_category di qualifica. Permette di sapere "se un'RDA contiene l'articolo X, il fornitore deve avere la categoria di qualifica Y".

**Stato API**:
- ⚠ `GET /arak/rda/v1/article` lista gli articoli **senza** mapping di categoria.
- Nessun endpoint per leggere/scrivere `articles.article_category`.
- Oggi l'app fa `SELECT … FROM articles.article_category INNER JOIN articles.article INNER JOIN service_category` e `UPDATE articles.article_category SET category_id=…`.

**Operazioni necessarie**:
| Op | Oggi | Target |
| --- | --- | --- |
| List articoli con mapping | SQL inner-join | **Nuovo endpoint** `GET /api/fornitori/v1/article-category` (backend Go + Postgres `articles` schema) |
| Aggiorna mapping | SQL `UPDATE articles.article_category` | **Nuovo endpoint** `PUT /api/fornitori/v1/article-category/{article_code}` (body: `{category_id}`) |

**Campi** (da SQL audit):
- `article_code` (string, PK lato `article_category`)
- `description` (da `article`)
- `category_id` → join → `category.name`

⚠ **Limite oggi**: gli articoli senza riga in `article_category` non compaiono (inner join). **Se confermi 1:1, manteniamo l'inner join.** ❓ **Q-A11**: confermi il comportamento legacy (orfani nascosti)?

✅ **Q-A12 RISOLTA**: stesso `ARAK_DSN` di Q-A10, schema `articles`. `backend/internal/fornitori/repo/article_category.go` (TBD) farà l'inner-join attuale + `UPDATE articles.article_category SET category_id=$1, updated_at=now() WHERE article_code=$2`.

---

## 2. Riassunto delle entità

| # | Entità | Endpoint Arak coprono? | Note 1:1 |
| --- | --- | --- | --- |
| 1.1 | Provider | Sì (CRUD) | Manca handling esplicito di `CEASED`. Bozze via `POST /provider` come oggi. |
| 1.2 | Provider Reference (4 tipi) | Sì (endpoint live non documentati nello spec) | DTO da scrivere a mano nel client Go; campo `phone` extra |
| 1.3 | Category | Sì (CRUD) | Bug check duplicati doc-type in attesa di decisione |
| 1.4 | Document Type | Sì (CRUD) | OK |
| 1.5 | Provider Category | Sì (list/create/edit) | No delete. Race condition in add da fixare |
| 1.6 | Document | Sì (CRUD + download) | Multipart vero al posto del trick base64 |
| 1.7 | Notification | Sì (sola lettura) | Q-A8 — quale Dashboard è il target |
| 1.8 | Country | NO endpoint, enum statico | Default: statico FE |
| 1.9 | Payment Method (+rda_available) | ⚠ NO | Endpoint nuovi nel monolith |
| 1.10 | Article × Category | ⚠ NO | Endpoint nuovi nel monolith |

## 3. Domande aperte (Phase A)

| ID | Topic | Decisione richiesta |
| --- | --- | --- |
| Q-A1 | Provider state | ✅ NO — DDL stato resta DRAFT/ACTIVE/INACTIVE; `CEASED` non esposto. |
| Q-A2 | Bozza fornitore | ✅ Solo `POST /provider`; `/provider/draft` non usato. |
| Q-A3 | Validazioni | ✅ Re-implementate client-side identiche (CF-or-VAT IT, ERP-required per ACTIVE/INACTIVE, CAP≥5 IT, provincia IT, ecc.). |
| Q-A4 | Reference endpoints | ✅ Esistono in live (non documentati nello spec). Body include `phone` + `reference_type`. |
| Q-A4-bis | Reference response shape | ⏳ Da verificare runtime: `GET /provider/{id}` ritorna `refs[]` array (default operativo: assumiamo array). |
| Q-A5 | Bug dup doc-type | ✅ Fix — il check oggi è palesemente non funzionante, lo correggiamo nel porting. |
| Q-A6 | Provider×Category | ✅ NO remove, NO toggle critical. Solo add come oggi. |
| Q-A7 | Document state | ✅ Mostrato grezzo (PENDING_VERIFY_ALL, OK, EXPIRED, …) come oggi. |
| Q-A8 | Dashboard | ✅ Dashboard visibile (3 endpoint nuovi nel nostro backend, niente API `notification`). |
| Q-A9 | Country | ✅ Lista statica FE (ISO 3166 hard-coded). |
| Q-A10 | Payment Method | ✅ Nuovo `ARAK_DSN` Postgres. |
| Q-A11 | Articoli orfani | ✅ Restano nascosti (inner join 1:1). |
| Q-A12 | Article-Category | ✅ Stesso `ARAK_DSN`. |

Tutte le decisioni di Phase A risolte. Procedo a Phase B.
