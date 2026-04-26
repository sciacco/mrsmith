# Fornitori — Phase C: Logic Placement

> Phase C output del workflow `appsmith-migration-spec`.
> Per ogni JSObject method e regola di business non-triviale: classificazione (domain / orchestration / presentation) + placement consigliato (backend / frontend / shared).
> Decisioni di Phase A e B presupposte (porting 1:1 con i caveat documentati).

## Convenzioni

- **Domain**: regola che esprime il dominio del business (es. "ACTIVE blocca tutti i campi tranne X").
- **Orchestration**: sequenza di chiamate API + transizioni UI (es. "salva → reload lista → chiudi modal").
- **Presentation**: derivazioni visive (es. concatenare label "Doc obbligatori: ...").
- **Placement**:
  - `BE`: backend Go (`mrsmith-fornitori`) — sempre se è authz, scrittura DB, o nuovo endpoint dashboard.
  - `FE`: frontend React — orchestration UI, mostrare/nascondere, validazione client-side replicata.
  - `SHARED`: stessa logica replicata su entrambi i lati (validazioni di sicurezza che vivono lato BE + UX echo lato FE).

---

## 1. JSObject methods (mappatura)

### 1.1 Dashboard / `Utils`

| Metodo | Classe | Placement | Note |
| --- | --- | --- | --- |
| `setFilterPeriod()` | Orchestration | **— (omettere)** | Era usato solo da Dashboard Copy nascosta. |
| `ViewDocument(idFile)` | Orchestration + Presentation | **FE** | Nel porting usiamo `<a href={blobUrl} download>` o `window.open(URL.createObjectURL(blob))`. Niente più heuristic base64-vs-binary: l'API Mistra ritorna `application/pdf`, useremo `Blob` direttamente. Helper riusabile in `apps/fornitori/src/lib/document-download.ts`. |
| `mapTable()` | dead code | **— (omettere)** | Audit conferma stub vuoto. |

### 1.2 Dashboard / `providerManager`

| Metodo | Classe | Placement | Note |
| --- | --- | --- | --- |
| `openProviderTab(providerId, selectedTab)` | Orchestration | **FE** | Diventa `useNavigate('/fornitori?id_provider=X&tab=Dati')`. |
| `myFun1`, `myFun2` | dead code | **— (omettere)** | Stub vuoti. |

### 1.3 Fornitori / `main` (~12 methods, ~250 lines)

L'oggetto JS più carico dell'app. Tabella di mappatura per ognuno:

| Metodo | Classe | Placement | Note |
| --- | --- | --- | --- |
| `ProviderAddResetField()` | Presentation | **FE** | Reset dei campi del Modal_new_fornitore. Diventa `form.reset()` di react-hook-form. |
| `ProviderAdd()` | Orchestration + Domain (validazioni) | **FE** (validazioni client-side) | ~16 rules: required CF or VAT (IT), CAP≥5 (IT), provincia (IT), email format ref, ecc. **Re-implementate identiche client-side** (Q-A3). Eventuali errori 4xx dell'API Arak già presenti — visualizzati lato FE. |
| `ProviderEdit()` | Orchestration + Domain | **FE** | Stesse ~16 rules + branch ACTIVE (sottoinsieme stretto: solo ref + payment + skip flag). Lock di campi sull'UI è presentation pura. **Regola "ERP required per ACTIVE/INACTIVE" replicata client-side**, ma se Mistra accetta il PUT senza errori il dato salta — la regola è solo lato nostro. |
| `ProviderDelete()` | Orchestration | **FE** | DELETE → close modal → navigate via `?id_provider` cleared → reload lista. |
| `ShowDetailProvider()` | Orchestration + Presentation | **FE** | Hydrata i campi dei tab Dati/Contatti. **Bug di `refs[0]` da NON portare**: usiamo `refs.find(r => r.reference_type === 'QUALIFICATION_REF')`. |
| `AddCategoryProvider()` | Orchestration | **FE** | Loop POST `/provider/{id}/category/{cat_id}?critical=...`. **Fix race condition**: `await Promise.all(...)` invece dei `.run()` non awaited. |
| `GetDocumentByCategoryProviderID()` | Orchestration | **FE** | Trigger della GET filtrata. Banale, vive nel componente. |
| `UploadDocument()` | Orchestration | **FE** | POST `/document` multipart. **Cambio di transport**: vero `multipart/form-data` con `Blob`, niente più stringa base64. Comportamento utente identico. |
| `UploadEditDocument()` | Orchestration | **FE** | PATCH `/document/{id}` multipart. Stesso pattern. |
| `ViewDocument(idDoc)` | Orchestration + Presentation | **FE** | Versione di `Utils.ViewDocument` per la pagina Fornitori. Stesso helper riusato. |

### 1.4 Fornitori / `category`

| Metodo | Classe | Placement | Note |
| --- | --- | --- | --- |
| `setDocFromCatOBB()` | Presentation | **FE** | Concatena label "Doc obbligatori: …" dalla `category.document_types[].filter(d => d.required).map(...)`. Pura derivazione. Quando non c'è selezione: testo `"nessun documento"` come oggi. |
| `setDocFromCatOPT()` | Presentation | **FE** | Idem per `required === false`. |

### 1.5 Fornitori / `reference`

| Metodo | Classe | Placement | Note |
| --- | --- | --- | --- |
| `allCategory[]` | Constant | **FE** | I 4 valori `OTHER_REF`/`ADMINISTRATIVE_REF`/`TECHNICAL_REF`/`QUALIFICATION_REF` con label IT. Tipo TS condiviso in `apps/fornitori/src/lib/reference.ts`. |
| `addCategory[]` | Constant | **FE** | Sottoinsieme dei 3 (esclude QUALIFICATION_REF). |
| `addContact(...)` | Orchestration | **FE** | POST `/provider/{id}/reference`. Body: campi vuoti omessi tranne per `phone` su create (omesso solo se vuoto). Mantenere comportamento Appsmith verbatim. |
| `updateReference(...)` | Orchestration | **FE** | PUT `/provider/{id}/reference/{ref_id}`. Body: `phone` **sempre inviato** anche se `''`; altri opzionali. Mantenere verbatim. |

### 1.6 Fornitori / `JSObject1`

Stub vuoto. **Omettere** dal porting (pulizia).

### 1.7 Imp. Qualifica / `Category`

| Metodo | Classe | Placement | Note |
| --- | --- | --- | --- |
| `setDocumentFromCategory()` | Presentation | **FE** | Pre-fill del detail panel con `category.document_types`. |
| `checkSelectDocumentAddCategory()` | Domain | **FE** | "Stesso doc_type non può comparire required + optional". **Fix bug** (Q-A5): il check deve effettivamente bloccare il save. |
| `checkSelectDocumentEditCategory()` | Domain | **FE** | Stesso check, branch edit. **Conditional name in body**: `name` incluso solo se cambiato (vs `TXT_category_selected`). Replicato — usiamo lo stato React iniziale per il diff invece di un widget hidden. |
| `addCategory(...)` | Orchestration | **FE** | POST `/category` → reload `GetCategory` → close modal. |
| `editCategory(...)` | Orchestration | **FE** | PUT `/category/{id}` → reload (no auto-hide del detail dopo edit, 1:1). |
| `deleteCategory(...)` | Orchestration | **FE** | DELETE `/category/{id}` → reload + hide del detail. |
| `hideDetailCategory()` | Presentation | **— (omesso)** | Pattern imperative show/hide rimpiazzato dal layout naturale (Q-B7). |

### 1.8 Imp. Qualifica / `TypeDocument`

Stesso pattern di `Category`, applicato ai document type. Tutti i metodi → **FE**.

### 1.9 Modalità Pagamenti / `ModalitaPagamentiJS`

Audit: "alternative pending-row save path **non bound to any widget**". **Omettere** dal porting (dead code). Save-row per inline-edit lo implementiamo dal componente tabella direttamente come oggi fa l'`onSave` di EditActions.

### 1.10 Dashboard Copy / `Utils`

Esclusa: Dashboard Copy non viene portata (Q-A8).

---

## 2. Business rules (mappatura)

Dalla §4 dell'audit + le sezioni di Phase A:

| # | Regola | Placement | Note |
| --- | --- | --- | --- |
| 1 | **Provider state machine**: una volta `ACTIVE`, modificabili solo `ref`, `default_payment_method`, `skip_qualification_validation`. Tutto il resto disabilitato. | **FE** (lock dei campi) — Mistra non documenta enforcement | Replicato 1:1 nei `disabled` di react-hook-form. |
| 2 | **Document expiry threshold = 30gg** (Dashboard) | **BE** (calcolo `days_remaining`) | Hard-coded nel nuovo endpoint dashboard expiring-documents. |
| 3 | **No-overlap doc_types** (required ∩ optional = ∅) | **FE** (Q-A5 → fixato) | Eventuale check parallelo BE non in scope 1:1. |
| 4 | **CF or VAT obbligatorio** se `country=='IT'` | **FE** | Validazione client-side (Q-A3). |
| 5 | **CAP ≥ 5 char** se `country=='IT'` | **FE** | Idem. |
| 6 | **Provincia required** se `country=='IT'` | **FE** | Idem. |
| 7 | **ERP code required** per impostare state ACTIVE/INACTIVE (audit: "non è possibile impostare lo stato del fornitore attivo o disattivo se non viene censito il codice alyante") | **FE** | Replicato. |
| 8 | **`skip_qualification_validation`** è un payload privilegiato | **BE** (gating) + **FE** (visibilità switch) | BE: 403 se non hai il ruolo (vedi §3 authz). FE: switch nascosto se ruolo assente (Q-B2). |
| 9 | **Reference categories** = enum 4 valori; add esclude QUALIFICATION_REF | **FE** | Costanti in `lib/reference.ts`. Mistra non documenta enum, conferma runtime. |
| 10 | **provider×category.critical** boolean, settato a creazione | **FE** | Replicato. |
| 11 | **`Acquisti RDA AFC` → read-only** su Imp.Qualifica + Articoli-Categorie + Modalità Pagamenti | **BE** + **FE** | BE: 403 sulle write per il ruolo `app_fornitori_readonly`. FE: button disabled. |

---

## 3. Authz — placement

**Decisione**: spostiamo l'enforcement da client-side (oggi `appsmith.user.groups.includes('Acquisti RDA AFC')`, bypassabile triviale) a **backend** primario + **frontend** echo per UX.

### Ruoli Keycloak (proposti)

| Ruolo | Scopo | Mappa al gruppo Appsmith |
| --- | --- | --- |
| `app_fornitori_access` | Accesso alla mini-app (deve averlo chiunque acceda) | (nessuno specifico — è l'access role del portale) |
| `app_fornitori_readonly` | Sola lettura su Imp.Qualifica, Articoli-Categorie, Modalità Pagamenti | `Acquisti RDA AFC` |
| `app_fornitori_skip_qualification` | Può attivare lo switch `skip_qualification_validation` su `PUT /provider/{id}` | (oggi nessun gating, novità del porting) |

### Enforcement BE

| Endpoint | Default | Read-only | Skip-qual |
| --- | --- | --- | --- |
| `GET /api/fornitori/v1/...` | 200 | 200 | 200 |
| `POST/PUT/DELETE /api/fornitori/v1/category[…]` | 200 | **403** | 200 |
| `POST/PUT/DELETE /api/fornitori/v1/document-type[…]` | 200 | **403** | 200 |
| `PUT /api/fornitori/v1/payment-method/{code}/rda-available` | 200 | **403** | 200 |
| `PUT /api/fornitori/v1/article-category/{code}` | 200 | **403** | 200 |
| `PUT /api/fornitori/v1/provider/{id}` con `skip_qualification_validation: true` nel body | **403** | **403** | 200 |
| `PUT /api/fornitori/v1/provider/{id}` senza `skip_qualification_validation` | 200 | 200 | 200 |

❓ **Q-C1**: Il portale espone già gating `app_<name>_access` per accesso alla mini-app (vedi `applaunch/catalog.go`). Confermo nuovi ruoli `app_fornitori_readonly` e `app_fornitori_skip_qualification` con questi nomi? (in linea con la naming convention `app_{appname}_<scope>` di CLAUDE.md). Se preferisci nomi diversi va bene.

❓ **Q-C2**: Il `skip_qualification_validation` oggi è una PUT al `/provider/{id}` di Arak — non sotto il nostro controllo. Per fare gating dobbiamo: (a) il backend nostro intercetta il body, valida il ruolo, poi forwarda; oppure (b) il frontend chiama Arak via il nostro proxy `/api/fornitori/v1/provider/{id}` (che fa la validazione e poi chiama Arak via il client Go). **Default proposto: (b)** — passiamo sempre dal nostro backend per le write, mai chiamate dirette FE → Arak. Coerente con altre mini-app?

---

## 4. Strategia di proxy verso Arak

Le altre mini-app del repo usano il pattern: FE → `/api/<app>/v1/...` → backend Go → client Arak (`backend/internal/platform/arak`) → Mistra.

Per fornitori questo significa:

| Endpoint pubblico (FE) | Backend handler | Outbound (BE → ext) |
| --- | --- | --- |
| `GET /api/fornitori/v1/provider` | `provider.List` | `arakCli.GetAllProvider(...)` |
| `GET /api/fornitori/v1/provider/{id}` | `provider.Get` | `arakCli.GetProvider(...)` |
| `POST /api/fornitori/v1/provider` | `provider.Create` | `arakCli.NewProvider(...)` |
| `PUT /api/fornitori/v1/provider/{id}` | `provider.Update` (con check `skip_qualification`) | `arakCli.EditProvider(...)` |
| `DELETE /api/fornitori/v1/provider/{id}` | `provider.Delete` | `arakCli.DeleteProvider(...)` |
| `POST /api/fornitori/v1/provider/{id}/reference` | `reference.Create` | direct HTTP via `arakCli.Do(...)` (DTO custom — endpoint non documentato) |
| `PUT /api/fornitori/v1/provider/{id}/reference/{ref_id}` | `reference.Update` | direct HTTP via `arakCli.Do(...)` |
| `POST /api/fornitori/v1/provider/{id}/category/{cat_id}` | `provider_category.Create` | `arakCli.NewProviderCategory(...)` |
| `GET /api/fornitori/v1/provider/{id}/category` | `provider_category.List` | `arakCli.GetAllProviderCategory(...)` |
| `GET/POST/PUT/DELETE /api/fornitori/v1/category[/{id}]` | `category.*` | `arakCli.*Category(...)` |
| `GET/POST/PUT/DELETE /api/fornitori/v1/document-type[/{id}]` | `document_type.*` | `arakCli.*DocumentType(...)` |
| `GET /api/fornitori/v1/document` | `document.List` | `arakCli.GetAllDocument(...)` |
| `POST /api/fornitori/v1/document` | `document.Upload` | `arakCli.NewDocument(...)` (multipart) |
| `PATCH /api/fornitori/v1/document/{id}` | `document.Patch` | `arakCli.EditDocument(...)` |
| `GET /api/fornitori/v1/document/{id}/download` | `document.Download` | `arakCli.DownloadDocument(...)` (stream pass-through) |
| `GET /api/fornitori/v1/dashboard/drafts` | `dashboard.Drafts` | DB `provider_qualifications.provider WHERE state='DRAFT'` |
| `GET /api/fornitori/v1/dashboard/expiring-documents` | `dashboard.ExpiringDocs` | DB join `document × provider × document_type`, threshold 30gg |
| `GET /api/fornitori/v1/dashboard/categories-to-review` | `dashboard.CategoriesToReview` | DB join `provider_category × provider × service_category` con filtri legacy |
| `GET /api/fornitori/v1/payment-method` | `payment_method.List` | DB `provider_qualifications.payment_method` |
| `PUT /api/fornitori/v1/payment-method/{code}/rda-available` | `payment_method.UpdateAvailability` | DB UPDATE |
| `GET /api/fornitori/v1/article-category` | `article_category.List` | DB inner-join schema `articles` |
| `PUT /api/fornitori/v1/article-category/{code}` | `article_category.UpdateAssociation` | DB UPDATE |

Convenzione `/api/<app>/v1/...` in linea con `docs/API-CONVENTIONS.md` (memory). Public URLs hanno il prefisso `/api`, il modulo Go lo omette internamente.

❓ **Q-C3**: Per il download documento, due opzioni: (a) il backend nostro fa stream pass-through (chunk a chunk dal client Arak al response del FE), (b) il backend redirect 302 al gateway Arak (più snello ma espone token Mistra al browser — da evitare). **Default proposto: (a) stream**, con `Content-Disposition` ereditato. Coerente con come fanno altre mini-app del monorepo?

---

## 5. Gestione errori (FE)

Pattern dell'app Appsmith oggi: `try/catch` + `showAlert('msg', 'success'|'error')` italiano. Per il porting:

- Replicare i messaggi italiani **letterali** (audit + JS sample del `reference` JSObject). Esempio: "Aggiornamento completato", "Errore nel salvataggio del contatto: …".
- Toast/snackbar UX: stesso ruolo della `showAlert` Appsmith, usando il pattern già presente nel monorepo (component shared in `@mrsmith/ui` se esiste).
- Errori 4xx dall'API → estrarre `error.message` dal response body Mistra (formato standard).
- Errori 5xx → toast generico "Errore del server. Riprova fra qualche istante."
- 401 → redirect login (già gestito dal portale).
- 403 → toast "Operazione non consentita per il tuo ruolo." (nuovo, conseguenza del migrare l'authz al backend).

❓ **Q-C4**: Esiste già un component toast/alert condiviso in `@mrsmith/ui` o `apps/<altra app>/src/components`? Se sì, lo riusiamo. Lo verifichiamo a Phase E quando assemblo lo SPEC. Default operativo: riusiamo se c'è, altrimenti inline `useToast` di shadcn-style.

---

## 6. State management (FE)

Pattern da seguire (in linea con le altre mini-app del monorepo):

- **Server state**: TanStack Query (`useQuery` per le liste, `useMutation` per le write con `invalidateQueries` su success).
- **URL state**: `useSearchParams` per `?id_provider=X&tab=Dati`.
- **Form state**: react-hook-form + zod schemas per la validazione client-side delle 11 regole di business.
- **Modal state**: state locale del componente o dialog primitive (no provider globale).

Niente Redux / Zustand: l'app è state-light (un master + dettagli derivati dall'URL).

---

## 7. Sintesi placement

| Area | Backend | Frontend | Shared |
| --- | --- | --- | --- |
| CRUD entità (Provider, Category, DocType, Document, ProviderCategory) | Proxy verso Arak + authz | Trigger UI + ottimismo TanStack Query | – |
| Reference (4 tipi) | Proxy verso Arak (endpoint non documentati) | Trigger + filtro QUALIFICATION_REF in Tab Dati | – |
| Dashboard | 3 endpoint nuovi (DB diretto + threshold 30gg + filtri stato) | Stat tile + 3 tabelle, deep-link a Fornitori | – |
| Payment Method (lista + toggle) | 2 endpoint nuovi (DB) | Tabella inline-edit | – |
| Article × Category | 2 endpoint nuovi (DB) | Master-detail snello | – |
| Validazioni provider (16 regole) | (eventuale 4xx da Arak) | Replicate client-side identiche | – |
| Authz | Enforcement primario (403 su write privilegiate) | Disable button + nascondi switch privilegiato | Source of truth: token Keycloak |
| Document download | Stream pass-through | `Blob` + download | – |
| File upload | Pass-through multipart al client Arak | `FormData` + `File` reale (no base64 trick) | – |

---

## Domande aperte (Phase C)

| ID | Topic | Quesito | Default |
| --- | --- | --- | --- |
| Q-C1 | Naming Keycloak roles | OK `app_fornitori_access`, `app_fornitori_readonly`, `app_fornitori_skip_qualification`? | Sì |
| Q-C2 | Skip-qual gating | FE → backend nostro → Arak (non chiamate dirette FE → Arak)? | Sì |
| Q-C3 | Download documento | Backend stream pass-through (no redirect 302)? | Sì |
| Q-C4 | Toast/alert UI | Riusare component shared se esiste, altrimenti inline? | Sì (verifico in Phase E) |

Tutte conferme di default. Sblocco Phase D dopo conferma.

## Conferme finali Phase C

| ID | Esito |
| --- | --- |
| Q-C1 | ✅ Ruoli `app_fornitori_access`, `app_fornitori_readonly`, `app_fornitori_skip_qualification`. |
| Q-C2 | ✅ FE chiama sempre il backend nostro; mai FE → Arak diretto. |
| Q-C3 | ✅ Stream pass-through del download. |
| Q-C4 | ✅ Riuso `ToastProvider` + `useToast` da `@mrsmith/ui` (`packages/ui/src/components/Toast/ToastProvider.tsx`). |
