# Fornitori — Phase B: UX Pattern Map

> Phase B output del workflow `appsmith-migration-spec`.
> Per ciascuna pagina visibile dell'app Appsmith: pattern di interazione, intent utente, sezioni logiche, eventuali ambiguità.
> Dashboard Copy nascosta esclusa per decisione di Phase A (Q-A8).

## Convenzioni

- **Pattern**: classificazione del tipo di vista (lista+detail, master-detail con tab, tabella inline-edit, lookup admin, dashboard di sintesi).
- **Intent**: cosa l'operatore vuole ottenere quando atterra sulla pagina.
- **Sezioni**: blocchi di UI raggruppati per scopo, non widget-by-widget.
- ❓ = decisione UX da chiedere all'esperto.

---

## Pagine in scope

| # | Pagina (Appsmith) | Pattern | Intent primario |
| --- | --- | --- | --- |
| 1 | Dashboard | Operational landing (3 tile + 3 tabelle scorrelate) | "Cosa devo fare oggi?" — vedere drafts, doc in scadenza, categorie da rinnovare |
| 2 | Fornitori | Master-detail con tab condizionali (5 tab + 5 modali) | Anagrafica, qualifica e documenti del singolo fornitore |
| 3 | Impostazioni Qualifica | Doppio pannello CRUD admin (categorie + tipi documento) | Manutenzione del catalogo di qualifica |
| 4 | Modalità Pagamenti RDA | Tabella inline-edit | Toggle `rda_available` per ciascun metodo di pagamento |
| 5 | Articoli - Categorie | Master-detail "snello" (tabella + form contestuale) | Associare articoli ERP a categorie di qualifica |

5 pagine, 5 pattern diversi. Niente componente comune oltre alla testata di portale.

---

## 1. Dashboard

**Pattern**: operational landing. Tre KPI tiles + tre tabelle non correlate fra loro.

**Intent**: l'operatore qualifica fornitori atterra qui per vedere "cosa va gestito ora": chi è in DRAFT da promuovere/cestinare, quali documenti scadranno entro 30 giorni, quali categorie hanno status `NEW` o `NOT_QUALIFIED`.

**Sezioni**:

| Sezione | Contenuto | Sorgente dati (target) |
| --- | --- | --- |
| KPI tiles | 3 contatori: documenti scaduti/in scadenza, fornitori da qualificare, fornitori con categoria scaduta | `.length` delle tre tabelle sottostanti (no API separata) |
| Fornitori da qualificare | Tabella: ragione sociale, P.IVA, CF, indirizzo. Click → Fornitori con `?id_provider=X` e tab `Dati` | `GET /api/fornitori/v1/dashboard/drafts` (nuovo) |
| Documenti scaduti / in scadenza | Tabella: ragione sociale, tipo doc, data scadenza, giorni rimanenti, link file | `GET /api/fornitori/v1/dashboard/expiring-documents` (nuovo, threshold 30gg fisso) |
| Categorie da gestire | Tabella: ragione sociale, categoria, status. Click → Fornitori con `?id_provider=X` e tab `Qualifica` | `GET /api/fornitori/v1/dashboard/categories-to-review` (nuovo) |

**Azioni utente**:
- **Click riga "Fornitori da qualificare"** → naviga a Fornitori, deep-link `?id_provider=X`, tab `Dati`.
- **Click colonna File su "Documenti in scadenza"** → download PDF (`GET /document/{id}/download` di Arak).
- **Click riga "Categorie da gestire"** → naviga a Fornitori, deep-link `?id_provider=X`, tab `Qualifica`.

**Note di porting**:
- I 3 SQL si fondono in 3 endpoint REST distinti (non un unico aggregato — più semplice paginazione/cache lato browser). I conteggi delle tile vengono dalla `.length` del rispettivo array, niente endpoint counter dedicato.
- Calcolo `days_remaining` ed eventuali `state in ('DRAFT','ACTIVE')` (audit) **si spostano nel backend Go**.
- Il bug `Dashboard.GetDocumentByIDfile` che referenzia `TBL_Document.triggeredRow.ID` (widget di Dashboard Copy) sparisce: il download usa la riga della tabella corrente.

**Ambiguità**:
- ❓ **Q-B1**: La tile "Fornitori con categoria scaduta" oggi mostra `category_expired.data.length`. Se un provider ha 3 categorie da gestire, conta 3 o 1? Lo SQL legacy ritorna una riga per associazione → conta 3. Manteniamo questa semantica? Default: **sì** (1:1 con SQL).

---

## 2. Fornitori (master-detail con tab)

**Pattern**: lista master + tabs condizionali con detail. Cinque tab, di cui una hidden (Storico Modifiche).

**Intent**: gestire l'intero ciclo di vita del fornitore — dall'inserimento bozza, alla raccolta dati anagrafici, all'assegnazione di categorie di qualifica, al caricamento documenti.

**Entry points**:
1. Click su una riga di `TBL_supply` (lista in tab 0).
2. Deep-link da Dashboard: `?id_provider=X` + `selectedTab=Dati|Qualifica`.

L'espressione `TBL_supply.selectedRow.id || appsmith.URL.queryParams.id_provider` ricorre 17 volte nel codice attuale.

**Sezioni / tab**:

### Tab 0 — Dati e Qualifica (lista master)

**Pattern**: tabella con filtro implicito + bottone "Nuovo".

**Intent**: trovare un fornitore esistente o creare un nuovo provider.

| Elemento | Comportamento |
| --- | --- |
| `BTN_new_fornitore` | Apre `Modal_new_fornitore` (form completo + categorie iniziali) |
| `TBL_supply` | Lista tutti i provider. Colonne: ragione sociale, P.IVA, CF, indirizzo, stato, codice Alyante, ERP, lingua. Click riga → tab Dati |

**Modal_new_fornitore**:
- Form anagrafica completa + 1 reference (qualification ref) + multi-select categorie + checkbox `critical` per ognuna.
- Validazioni `main.ProviderAdd`: ragione sociale, indirizzo, città, CAP, paese, lingua, payment method, email ref obbligatori; CF-or-VAT se `country=='IT'`; CAP≥5 + provincia se IT.
- Flusso save: `POST /provider` → per ciascuna categoria selezionata `POST /provider/{id}/category/{cat_id}?critical=...`
- ⚠ Race condition oggi (loop POST non awaited). Da fixare nel porting (sequenziale o `Promise.all`).

### Tab 1 — Dati (anagrafica + edit)

**Pattern**: form di dettaglio con campi disabilitati condizionalmente.

**Intent**: leggere/modificare i dati di un fornitore selezionato.

**Sezioni**:
- **Anagrafica fiscale**: ragione sociale, P.IVA, CF, codice ERP (Alyante), lingua.
- **Sede**: paese, provincia, città, indirizzo, CAP.
- **Contatto qualifica**: nome, cognome, email, telefono. (Singolo `QUALIFICATION_REF` — è un sotto-set di `refs[]`).
- **Operative**: stato (DDL DRAFT/ACTIVE/INACTIVE), payment method default, switch `skip_qualification_validate` (privilegiato).
- **Bottoni**: `BTN_edit_provider` (salva), `BTN_delete_provider` (apre modal conferma).

**Regola di lock**: `state == 'ACTIVE'` → tutto disabilitato tranne `ref` (contatto qualifica), `default_payment_method`, `skip_qualification_validate`. Più la possibilità di cambiare `state` (DDL_State sempre attivo).

**Validazioni edit**:
- Se cambia `state` da DRAFT → ACTIVE/INACTIVE: ERP code deve essere valorizzato.
- Stesse regole CF-or-VAT, CAP, provincia di add (in branch DRAFT).

**Campo privilegiato**:
- `skip_qualification_validate` → switch oggi visibile a tutti, payload `skip_qualification_validation: <bool>` solo quando attivato. Migrazione: gating lato backend con Keycloak role privilegiato (vedi Phase D).
- ❓ **Q-B2**: Visibilità del switch — visibile a tutti ma il save fallisce se non hai il ruolo? Oppure nascosto se non hai il ruolo? **Default proposto: nascosto se non hai il ruolo** (1:1 stretto = visibile a tutti, ma il porting attuale risolve un'altra anomalia di sicurezza, simmetrica a `Acquisti RDA AFC`).

**Bug noto da NON portare** (audit step 159):
- `main.ShowDetailProvider` ha un loop spezzato; oggi la qualification ref nei `TXT_*_edt` è popolata da `refs[0]` (primo elemento qualunque sia il tipo). Nel porting filtriamo esplicitamente `refs.find(r => r.reference_type === 'QUALIFICATION_REF')`.

### Tab 2 — Contatti

**Pattern**: tabella editabile inline con add-row.

**Intent**: gestire i contatti non-qualifica del fornitore (amministrativo, tecnico, altro).

**Sezioni**:
- `TBL_reference`: lista filtrata di `refs[]` esclusi quelli `QUALIFICATION_REF`.
- Colonne: tipo (DDL: amministrativo/tecnico/altro), nome, cognome, email, telefono.
- `EditActions` per riga: save / discard.
- Add new row inline.

**Operazioni**:
- Save row esistente → `PUT /provider/{id}/reference/{ref_id}` con body `{first_name?, last_name?, email?, phone}` (phone sempre inviato anche vuoto, audit conferma).
- Save row nuova → `POST /provider/{id}/reference` con body completo + `reference_type` obbligatorio.
- ❓ **Q-B3**: Oggi non c'è una delete per reference. Aggiungiamo (UX naturale per inline-edit) o restiamo 1:1? **Default proposto: NO delete** (1:1 stretto, l'audit non la cita, niente regressione di sicurezza).

**Note di filtro**:
- `reference.allCategory` lista 4 valori (per la rappresentazione/edit).
- `reference.addCategory` ne ha 3 (esclude QUALIFICATION_REF, perché si gestisce in Tab Dati).

### Tab 3 — Qualifica

**Pattern**: tabella primaria + tabella secondaria filtrata da selezione.

**Intent**: gestire le categorie di qualifica associate a un fornitore e vedere lo stato dei documenti per ciascuna.

**Sezioni**:
- `BTN_add_category` → apre `mdl_new_category` (multi-select categorie + flag `critical`).
- `TBL_categoryProvider`: lista `provider×category` per il fornitore. Colonne: nome categoria, status (NEW/QUALIFIED/NOT_QUALIFIED), critical.
- `TBL_documentByCategory`: visibile solo se è selezionata una riga in `TBL_categoryProvider`. Mostra documenti del fornitore filtrati per `category_id`.
- 2 icon-button refresh.

**Operazioni**:
- Add → loop POST `/provider/{id}/category/{cat_id}?critical=...` (fix race condition rispetto a oggi).
- No delete provider×category (Q-A6 → no).
- No toggle critical post-creazione (Q-A6 → no).

**Ambiguità**:
- ❓ **Q-B4**: La sotto-tabella `TBL_documentByCategory` mostra i documenti **del fornitore filtrati per categoria** ma i documenti in Mistra non sono associati direttamente alla categoria — sono associati al `document_type`. La query `GET /document?provider_id=X&category_id=Y` come filtra? Per `category_id` Mistra fa probabilmente: documenti il cui `document_type` appartiene alla `category.document_types[]`. Va verificato runtime. **Default operativo: usiamo l'endpoint as-is, l'API decide la semantica.**

### Tab 4 — Documenti Qualifica

**Pattern**: tabella con upload modal.

**Intent**: caricare/aggiornare documenti del fornitore (PDF).

**Sezioni**:
- `BTN_new_document` → apre `mdl_detailDocument`: tipo documento (DDL), data scadenza (date picker), file (file picker). Save abilitato solo se tutti e 3 sono presenti.
- `TBL_documents`: documenti del fornitore. Colonne: tipo, scadenza, stato (grezzo, Q-A7), source (INTERNAL/EXTERNAL), File (download), Edit.
- Click Edit → `mdl_editDocument`: file (re-upload obbligatorio) + nuova data scadenza. Save abilitato se entrambi presenti.
- `TBL_categoryProvider_memo` + label dinamiche: "Doc obbligatori: ..." / "Doc facoltativi: ..." derivate da `category.document_types` quando si seleziona una categoria nel memo (vista informativa, nessuna azione).

**Operazioni**:
- Upload → `POST /document` multipart (file, expire_date, provider_id, document_type_id). Cambio rispetto a oggi: vero `multipart/form-data` con `Blob`, niente trick base64.
- Edit → `PATCH /document/{id}` multipart (file, expire_date).
- Download → `GET /document/{id}/download`. Il backend Mistra ritorna `Content-Type: application/pdf` (Phase A §1.6) → useremo `Blob` direttamente, niente più base64-vs-binary heuristic di `Utils.ViewDocument`.

**Ambiguità**:
- ❓ **Q-B5**: Modal Edit obbliga il file: oggi non si può modificare solo la data scadenza senza ri-uploadare. Vero anche perché `EditDocument` (PATCH) richiede `file` e `expire_date` come required (spec). Lo manteniamo 1:1? **Default: sì.** Volendo è migliorabile post-1:1 (file opzionale, tieni il vecchio se assente) — fuori scope.

### Tab 5 — Storico Modifiche (hidden)

**Pattern**: placeholder.

**Intent**: nessuno — è una tabella con dati hard-coded vuoti (`Table5`), nascosta da `false` hard-coded.

**Decisione**:
- ❓ **Q-B6**: La portiamo nel porting come placeholder hidden o la **omettiamo proprio** (1:1 dell'esperienza utente, non del codice morto)? **Default proposto: omettiamo** — è dead code e l'utente non la vede.

### Modali della pagina Fornitori (5)

| Modal | Scopo | Trigger |
| --- | --- | --- |
| `Modal_new_fornitore` | Add provider completo + categorie iniziali | Tab 0 → `BTN_new_fornitore` |
| `mdl_new_category` | Multi-select categorie + critical, per assegnazione a un fornitore esistente | Tab 3 → `BTN_add_category` |
| `mdl_detailDocument` | Upload nuovo documento qualifica | Tab 4 → `BTN_new_document` |
| `mdl_editDocument` | Edit (re-upload + scadenza) di un documento esistente | Tab 4 → click Edit in tabella |
| `mdl_delete_provider` | Conferma delete provider | Tab 1 → `BTN_delete_provider` |

---

## 3. Impostazioni Qualifica

**Pattern**: doppio pannello CRUD admin (categorie + tipi documento).

**Intent**: l'admin manutiene il catalogo di qualifica — quali categorie esistono, quali tipi documento esistono, e per ciascuna categoria quali tipi sono required vs optional.

**Sezioni**:

| Sezione | Pattern interno |
| --- | --- |
| Lista categorie | `TBL_category` + `BTN_new_category` (apre `Modal_new_category`) + click riga → reveal `Container_detail_category` |
| Detail categoria | Form: nome + 2 multi-select (required doc types vs optional doc types) + Save / Delete |
| Lista tipi documento | `TBL_document_type` + `BTN_new_type_document` (apre `Modal_new_document_type`) + click riga → reveal `Container_detail_typedocument` |
| Detail tipo documento | Form: nome + Save / Delete |

**Comportamenti specifici**:
- I container detail sono nascosti di default; appaiono in `setVisibility(true)` al click riga. Nessun auto-hide su deselezione.
- ❓ **Q-B7**: Nel porting React possiamo usare un *layout naturale* (i form detail sono in una colonna fissa che mostra "Nessuna selezione" quando non c'è nulla) invece dell'imperative show/hide attuale. Mantiene il comportamento "selezione → vedi detail" ma rimuove il pattern fragile. **Default proposto: sì, layout naturale.** Il pattern Appsmith era un workaround — non è una semantica di business.

**Validazioni**:
- "No overlap" tra required e optional doc types (oggi bug-gato — Q-A5 ha già detto: fix nel porting).
- Add categoria: nome + lista doc_types richiesti.
- Edit categoria: nome opzionale (solo se cambiato), doc_types obbligatori — replicato 1:1.

**Authz**: chiunque abbia `app_fornitori_access` può creare/modificare/eliminare. Il gate Appsmith `Acquisti RDA AFC → read-only` non è stato portato.

---

## 4. Modalità Pagamenti RDA

**Pattern**: tabella inline-edit single-column.

**Intent**: l'admin abilita/disabilita la possibilità che un metodo di pagamento sia selezionabile in app RDA (purchase requests).

**Sezione unica**:
- `Table1` con colonne `code`, `description`, `rda_available` (boolean editabile inline).
- EditActions per riga: save / discard.
- Save → `UpdateAvailability` (oggi UPDATE SQL diretto, target: `PUT /api/fornitori/v1/payment-method/{code}/rda-available` body `{rda_available}`).

**Authz**: write disponibile a chiunque abbia `app_fornitori_access`.

**Niente add / niente delete**: la lista metodi di pagamento è popolata da Postgres come master data, nessun CRUD esposto in UI.

---

## 5. Articoli - Categorie

**Pattern**: master-detail "snello" (tabella + form contestuale, nessun modal).

**Intent**: associare ciascun articolo ERP a una categoria di qualifica (così quando un'RDA contiene quell'articolo si sa quale qualifica deve avere il fornitore).

**Sezioni**:
- `TBL_article_category`: tabella con `article_code`, `description`, `category_name` corrente.
- `CTN_article_category` (initially hidden): si apre alla selezione di una riga. Contiene:
  - `DDL_articles` (disabled, prefilled con `article_code` selezionato — solo conferma visiva).
  - `DDL_category`: lista categorie attive (`deleted_at IS NULL`).
  - `BTN_save` → `UpdateAssociationArticleCat` (oggi UPDATE SQL, target: `PUT /api/fornitori/v1/article-category/{article_code}` body `{category_id}`).
  - `BTN_reset` → chiude container.

**Note di porting**:
- Stesso discorso di §3 sul container imperativo → ❓ **Q-B9** (gemella di Q-B7): nel porting layout naturale o show/hide? **Default: layout naturale.**
- Articoli orfani: rimangono nascosti (Q-A11 → inner join 1:1).
- Authz: write disponibile a chiunque abbia `app_fornitori_access`.

---

## Cross-cutting

### Pattern di deep-link

La Dashboard naviga a Fornitori passando `?id_provider=X` + chiama `storeValue('selectedTab', 'Dati'|'Qualifica')`. Nel porting React useremo `useSearchParams` (`?id_provider=X&tab=Dati`) come canale unico.

### Pattern di authz

Un solo livello effettivo: chi ha `app_fornitori_access` può eseguire tutte le operazioni dell'app. Il gate Appsmith `Acquisti RDA AFC → read-only` (audit §1, §2.3, §2.5) non è stato portato — capability privilegiata distinta `app_fornitori_skip_qualification` per il switch sul tab Dati.

### Copy

Tutta la UI è in italiano (alert, label, validazioni). Manteniamo italiano nel porting.

### Pattern table v2 + EditActions

Modalità Pagamenti RDA e Tab Contatti usano il pattern Appsmith Table v2 + colonna EditActions con `onSave`/`onDiscard` nel widget. Nel porting React replicheremo l'inline-edit con uno dei pattern già usati nelle altre mini-app (TanStack Table o equivalente).

---

## Sintesi pattern e domande aperte

| Pagina | Pattern | Q aperte |
| --- | --- | --- |
| Dashboard | Operational landing | Q-B1 (semantica counter "categorie scadute") |
| Fornitori | Master-detail con tab + modali | Q-B2 (visibilità switch privilegiato), Q-B3 (delete reference?), Q-B4 (filtro `category_id` su /document), Q-B5 (Edit doc richiede file?), Q-B6 (Storico Modifiche da omettere?) |
| Impostazioni Qualifica | Doppio CRUD admin | Q-B7 (layout naturale vs show/hide imperativo) |
| Modalità Pagamenti RDA | Tabella inline-edit | — |
| Articoli - Categorie | Master-detail snello | Q-B9 (layout naturale, gemella di Q-B7) |

Le 9 Q-B sono tutte conferme di default ragionevoli. Sblocco Phase C dopo conferma.

## Conferme finali Phase B

| ID | Esito |
| --- | --- |
| Q-B1 | ✅ Counter "categorie da gestire" = righe provider×category con status NEW/NOT_QUALIFIED (1:1 con SQL legacy). |
| Q-B2 | ✅ Switch `skip_qualification_validate` nascosto se manca il ruolo privilegiato. |
| Q-B3 | ✅ No delete reference. |
| Q-B4 | ✅ `GET /document?provider_id=X&category_id=Y` usato as-is; semantica decisa lato API Mistra; verificare runtime. |
| Q-B5 | ✅ Edit documento richiede file + scadenza (1:1 con `document-edit` API). |
| Q-B6 | ✅ Tab "Storico Modifiche" omessa nel porting. |
| Q-B7 | ✅ Layout naturale al posto di setVisibility imperative (Imp.Qualifica). |
| Q-B8 | ⛔ Superato — gate readonly rimosso, Modalità Pagamenti scrivibile a chi ha `app_fornitori_access`. |
| Q-B9 | ✅ Layout naturale (Articoli-Categorie). |
