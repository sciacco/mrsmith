# Quotes — Phase C: Logic Placement

**Source**: `apps/quotes/APPSMITH-AUDIT.md` (JSObjects, hidden logic, business rules)
**Date**: 2026-04-09
**Scope**: Quotes/proposals only. E-signature e order conversion esclusi.

---

## Classificazione

Ogni item è classificato come:
- **Domain**: regola di business che determina il comportamento del sistema
- **Orchestration**: coordinamento multi-step tra sistemi o entità
- **Presentation**: logica puramente visiva (colori, visibilità, formattazione)

Placement raccomandato:
- **Backend**: Go handler/service — enforcement obbligatorio, sicurezza, accesso DB/API
- **Frontend**: React component — feedback visivo, UX interattiva
- **Shared**: contratto condiviso (enum, costanti, tipi) usato da entrambi

---

## 1. Domain Logic → Backend

Queste regole DEVONO vivere nel backend. Oggi sono tutte nel frontend Appsmith senza enforcement server-side.

### 1.1 RBAC — Delete authorization

| Current | `Elenco.utils.eliminaOfferta()` — client-side role check |
|---|---|
| **Rule** | Delete richiede ruolo `"Administrator - Sambuca"` o `"Kit and Products manager"` |
| **Placement** | **Backend** — Keycloak role check nel handler Go. Il frontend nasconde il bottone, ma il backend è l'enforcement point. |
| **Mapping ruoli** | `app_quotes_delete` (nuovo ruolo Keycloak) oppure mappatura dei ruoli Appsmith esistenti |
| **Bug fix** | `==` vs `===` inconsistency eliminata — controllo server-side tipizzato |

### 1.2 Quote delete — HS + DB atomicity

| Current | `eliminaOfferta()`: HS delete → DB delete, non-atomico |
|---|---|
| **Rule** | Se la quote ha un `hs_quote_id`, prima cancella su HS, poi nel DB |
| **Placement** | **Backend** — singolo endpoint `DELETE /api/quotes/v1/quotes/:id`. Il backend orchestra: HS delete (se esiste) → DB delete. Se HS delete fallisce, non cancella il DB. |
| **Improvement** | Atomicità: se HS fallisce, errore all'utente, DB intatto |

### 1.3 Quote number generation

| Current | `common.new_document_number('SP-')` — DB function |
|---|---|
| **Rule** | Formato `SP-{seq}/{YYYY}`, sequenza `quote_number_seq` (start 1101) |
| **Placement** | **Backend** — chiamata alla DB function nella transazione di creazione quote. Il frontend non deve mai generare numeri. |
| **Note** | Già nel DB, resta lì. Il backend la invoca dentro `ins_quote_head`. |

### 1.4 Status determination at publish

| Current | `Dettaglio.mainForm.mandaSuHubspot()` |
|---|---|
| **Rule** | Se `notes` (pattuizioni speciali) non vuoto → `PENDING_APPROVAL`; altrimenti → `APPROVED` |
| **Placement** | **Backend** — il publish endpoint determina lo status. Il frontend non deve decidere lo status. |
| **Note** | Lo status `APPROVAL_NOT_NEEDED` è impostato da HubSpot, non dalla nostra app |

### 1.5 Required products validation

| Current | `Dettaglio.check_quote_rows` SQL query |
|---|---|
| **Rule** | Pubblicazione bloccata se esistono prodotti con `required = true` e nessuna variante `included = true` nel gruppo |
| **Placement** | **Backend** — validazione nel publish endpoint. Ritorna lista dei prodotti mancanti. |
| **Frontend companion** | Warning visivo in tempo reale (badge "2/3 obbligatori" per kit row) — ma non è l'enforcement point |

### 1.6 MRC forced to 0 for spot orders

| Current | `Dettaglio.detailForm.aggiornaRiga()` e `updateDetails()` |
|---|---|
| **Rule** | Se `document_type = 'TSC-ORDINE'` (spot) → `mrc = 0` su tutti i prodotti |
| **Placement** | **Backend** — enforcement nell'endpoint di update prodotto. Se `document_type` del parent quote è `TSC-ORDINE`, forza `mrc = 0`. |
| **Frontend companion** | Campo MRC disabilitato/grigio per spot orders, con helper text |

### 1.7 Quantity floor

| Current | `Dettaglio.detailForm.aggiornaRiga()` |
|---|---|
| **Rule** | Se `included = true` e `quantity = 0` → forza `quantity = 1` |
| **Placement** | **Backend** — enforcement nell'endpoint di update prodotto |

### 1.8 Mutual exclusion within product group

| Current | `quotes.upd_quote_row_product()` stored procedure |
|---|---|
| **Rule** | Un solo prodotto `included = true` per `group_name`. Selezionare un prodotto deseleziona gli altri del gruppo. |
| **Placement** | **Backend** — già nel DB (stored procedure). Resta lì. |

### 1.9 COLOCATION → Trimestrale billing

| Current | `Service.ServiceChange()` su Dettaglio e Nuova Proposta |
|---|---|
| **Rule** | Se i servizi includono COLOCATION → `bill_months = 3` (trimestrale) |
| **Placement** | **Backend** — validazione/enforcement al save della quote. Se COLOCATION nei services, `bill_months` forzato a 3. |
| **Frontend companion** | Campo billing disabilitato con valore pre-impostato e helper text |

### 1.10 Colo template blocked for spot

| Current | `Nuova Proposta.checkValori.spot_template()` |
|---|---|
| **Rule** | Template COLO non selezionabile se `document_type = 'TSC-ORDINE'` (spot) |
| **Placement** | **Backend** — validazione al save/publish. Template con `is_colo = true` incompatibile con `TSC-ORDINE`. |
| **Frontend companion** | Template COLO non mostrato nelle opzioni quando doc type è spot |

### 1.11 IaaS/VCloud field locking

| Current | 10+ widget `isDisabled` expressions con 8 template ID hardcoded |
|---|---|
| **Rule** | Se il template è di tipo IaaS → services, template, billing, term fields non modificabili |
| **Placement** | **Backend** — validazione al save: se `template.template_type = 'iaas'`, rifiuta modifiche a services, bill_months, initial_term_months, next_term_months (valori fissi: 1 mese). |
| **Frontend companion** | Campi disabilitati con helper "Valori fissi per offerte IaaS" |
| **Improvement** | Derivazione da `quotes.template.template_type` (DB) invece di 8 ID hardcoded |

### 1.12 HubSpot quote expiry

| Current | `salvaOfferta()`: `expire_date = today + 30 days` |
|---|---|
| **Rule** | Scadenza quote HS = data documento + 30 giorni |
| **Placement** | **Backend** — calcolata nel publish endpoint quando costruisce il payload HS |
| **Bug fix** | Appsmith usa `new Date(currentDate.setDate(...))` che muta `currentDate` — nel backend è un calcolo pulito |

### 1.13 Pipeline/stage filtering for deals

| Current | `get_potentials` / `get_deals` SQL con pipeline/stage ID hardcoded |
|---|---|
| **Rule** | Solo deals da pipeline `255768766`, `255768768` con stage whitelist specifici |
| **Placement** | **Backend** — costanti Go nel handler/service (decisione A6). Query SQL con filtro server-side. |

### 1.14 Product category exclusion

| Current | `get_product_category` SQL con `WHERE id NOT IN (12,13,14,15)` |
|---|---|
| **Rule** | Flow standard esclude categorie 12,13,14,15 (pay-per-use/IaaS). Decisione A5. |
| **Placement** | **Backend** — costanti Go. Endpoint `GET /api/quotes/v1/categories?type=standard` filtra server-side. |

### 1.15 Kit ecommerce exclusion

| Current | `list_kit` SQL con `WHERE ecommerce = false` |
|---|---|
| **Rule** | Kit con `ecommerce = true` non disponibili per quoting |
| **Placement** | **Backend** — filtro SQL `WHERE ecommerce = false AND is_active = true AND quotable = true` |

### 1.16 Default payment code

| Current | SQL `ISNULL(CAST(CODICE_PAGAMENTO as INT), 402)` + JS fallback |
|---|---|
| **Rule** | Metodo pagamento default dal cliente Alyante, fallback al codice 402 |
| **Placement** | **Backend** — query Alyante con fallback. Costante Go per default 402. |

### 1.17 `replace_orders` serialization

| Current | MultiSelect values joined con `;` in `salvaOfferta()` |
|---|---|
| **Rule** | Ordini sostituiti salvati come stringa separator `;` |
| **Placement** | **Backend** — il frontend invia un array, il backend serializza per coesistenza DB |

### 1.18 `cli_orders` filtered by customer

| Current | Alyante query senza filtro cliente (bug) |
|---|---|
| **Rule** | Ordini Alyante filtrati per `NUMERO_AZIENDA = :customer_erp_id`. Decisione A7. |
| **Placement** | **Backend** — query Alyante parametrizzata. Richiede cross-ref `customer_id` → `erp_id`. |

---

## 2. Orchestration Logic → Backend

Flussi multi-step che coordinano più sistemi. Tutti nel backend.

### 2.1 Quote creation (unified workflow)

| Current | Split: Nuova Proposta wizard crea HS quote vuota → Dettaglio configura prodotti |
|---|---|
| **New flow** | Wizard unico: header + kit + prodotti → save completo nel DB (DRAFT). Nessuna quote HS creata. |
| **Placement** | **Backend** — singolo endpoint `POST /api/quotes/v1/quotes` che in transazione: genera numero → inserisce quote → inserisce kit rows (trigger espande prodotti) → ritorna quote completa |
| **Note** | La quote HS viene creata solo alla pubblicazione esplicita |

### 2.2 HubSpot publish (idempotent)

| Current | `Dettaglio.mainForm.mandaSuHubspot()` — 16 step, nessun feedback, nessun retry |
|---|---|
| **New flow** | Backend endpoint `POST /api/quotes/v1/quotes/:id/publish` con step idempotenti: |
| **Steps** | 1. Salva quote nel DB (se dirty) |
| | 2. Valida prodotti obbligatori |
| | 3. Crea o aggiorna quote HS (idempotente via `hs_quote_id`) |
| | 4. Sync line items: delete orfani, update esistenti, create nuovi (idempotente via `hs_line_item_id`) |
| | 5. Write-back HS IDs nel DB |
| | 6. Aggiorna quote HS (status, T&C, associazioni) |
| | 7. Salva status finale nel DB |
| **Placement** | **Backend** — interamente Go. Frontend chiama e riceve progress via SSE o polling. |
| **Error handling** | Ogni step controlla stato attuale. Su errore: ritorna step fallito + messaggio. Retry riesegue tutto ma salta step già completati. |

### 2.3 HS line item sync (bidirectional)

| Current | `Dettaglio.hs_utils.hs_save_all_line_items()` |
|---|---|
| **Rule** | Per ogni kit row: confronta `hs_line_item_id` locale vs HS. Cancella orfani, aggiorna esistenti, crea nuovi. Due items per row: MRC e NRC. Position/group counters per label `A)`, `B)`. |
| **Placement** | **Backend** — sotto-step del publish flow. Usa view `v_quote_rows_for_hs` per descrizioni bilingui. |

### 2.4 Quote delete (HS + DB)

| Current | `Elenco.utils.eliminaOfferta()` |
|---|---|
| **Placement** | **Backend** — vedi 1.2. Endpoint `DELETE /api/quotes/v1/quotes/:id` |

### 2.5 T&C generation

| Current | `templates.terms_and_conditions()` — 6 varianti HTML hardcoded in JS |
|---|---|
| **Rule** | Genera HTML Terms & Conditions basato su `(template_type, is_colo, lang)` |
| **Placement** | **Backend** — Go templates (text/template o string builder). Contenuto business significativo, non deve vivere nel frontend. |
| **Varianti** | Non Colo IT, Non Colo EN, Colo IT, Colo EN, IaaS IT, IaaS EN |
| **Note** | Il contenuto va migrato verbatim dall'Appsmith JS. È contenuto contrattuale. |

---

## 3. Presentation Logic → Frontend

Logica puramente visiva. Vive nei React components.

### 3.1 Status color mapping

| Current | `Elenco.utils.bgStatus()` |
|---|---|
| **Rule** | DRAFT→grigio, PENDING_APPROVAL→amber, APPROVED→verde, APPROVAL_NOT_NEEDED→verde chiaro, ESIGN_COMPLETED→grigio "Firmata", unknown→rosso |
| **Placement** | **Frontend** — componente `StatusBadge`. Mapping colori da shared enum/costanti. |

### 3.2 Field enable/disable cascade

| Current | `TypeDocument.changeTypeDocument()`, widget `isDisabled` expressions |
|---|---|
| **Rule** | Tipo documento + tipo template → determinano quali campi sono modificabili |
| **Placement** | **Frontend** — React state derivato. Regole codificate come config object, non sparse in widget props. |
| **Source of truth** | `template.template_type` dal backend (non 8 ID hardcoded) |
| **Note** | Il backend valida comunque (vedi 1.11) — il frontend è solo UX |

### 3.3 Conditional visibility

| Current | Widget `isVisible` expressions sparse |
|---|---|
| **Items** | `replace_orders` visibile solo per SOSTITUZIONE, `trial` visibile solo per IaaS, billing fields nascosti per spot |
| **Placement** | **Frontend** — derivato dal tipo proposta/documento nel React state |

### 3.4 Product group display

| Current | `Dettaglio.utils.includedField()`, `isIncluded()`, `isRequired()`, `currentRow()` |
|---|---|
| **Rule** | Helpers per navigare il JSON array `riga` dalla view `v_quote_rows_products` |
| **Placement** | **Frontend** — utility functions nel component del product configurator. La struttura dati arriva dal backend già raggruppata. |

### 3.5 Required product warning badges

| Current | Red cell background quando `required && !included` |
|---|---|
| **Rule** | Badge "2/3 obbligatori" su kit row. Decisione B8. |
| **Placement** | **Frontend** — calcolo derivato dai dati prodotto. Il backend fornisce `required` e `included` per ogni prodotto; il frontend conta e mostra. |

### 3.6 Dirty state indicator

| Current | Non esiste in Appsmith |
|---|---|
| **Rule** | Indicatore visivo quando ci sono modifiche non salvate. Decisione B5. |
| **Placement** | **Frontend** — React state: confronto form values vs last-saved values |

### 3.7 IaaS trial text generation

| Current | `IaaS.creazioneProposta.recuperaTrial()` |
|---|---|
| **Rule** | Slider 0–200 → genera testo bilingue trial |
| **Placement** | **Frontend** — testo generato in tempo reale dal valore slider. Salvato come stringa nel campo `trial` alla creazione. |
| **Alternative** | Potrebbe andare nel backend se la formattazione del testo è contrattualmente rilevante. |

---

## 4. Shared Contracts → Costanti/Tipi condivisi

### 4.1 Status enum

```
DRAFT | PENDING_APPROVAL | APPROVAL_NOT_NEEDED | APPROVED | ESIGN_COMPLETED
```

- **Backend**: Go `const` + validazione
- **Frontend**: TypeScript enum/union type + mapping colori
- Contract: il backend ritorna lo status come stringa, il frontend lo mappa

### 4.2 Document type enum

```
TSC-ORDINE-RIC (recurring) | TSC-ORDINE (spot)
```

### 4.3 Proposal type enum

```
NUOVO | SOSTITUZIONE | RINNOVO
```

### 4.4 Template type enum

```
standard | iaas | legacy
```

### 4.5 NRC charge time enum

```
1 = all'ordine | 2 = all'attivazione
```

---

## 5. Dead Code — Non migrare

Logica presente nell'audit ma da NON portare nella nuova app.

| Item | Page | Reason |
|---|---|---|
| `nuovo_numero_offerta` | Elenco | Duplicato di `new_quote_number` |
| `hs_update_quote` | Elenco | Non chiamato da questa pagina |
| `hs_associa_contatto` | Elenco | Non chiamato da questa pagina |
| `Query1` (vodka) | Elenco | Completamente estraneo |
| `contattiPerEsignature: {}` | Elenco | Mai usato |
| `test_hs2()` | Nuova Proposta | Test/debug |
| `newQuoteAssociations()` dead branch | Nuova Proposta | `if (false && ...)` |
| `inserisci_righe = false` block | Nuova Proposta | Logica spostata a Dettaglio (commento inline) |
| 4 CRUD auto-generated queries | IaaS | Scaffolding non usato |
| `render_template` (Carbone.io) | Dettaglio | Esperimento abbandonato (A8) |
| `firmaForm` (intero JSObject) | Dettaglio | E-signature rimossa (B7) |
| `gestisciContattiEsignature()` | Dettaglio | E-signature rimossa |
| `xmlParser` library | Global | Nessun uso attivo |
| Converti in ordine (intera pagina) | — | Deferred to phase 2 |

---

## 6. Bug Fixes — Correzioni da implementare

Bug Appsmith che vengono risolti dalla nuova architettura.

| Bug | Root cause | Fix |
|---|---|---|
| `salvaOfferta` template condition always true | `if(x != "A" \|\| x != "B")` | Backend: clean if/else o switch |
| Missing closing quote in `isDisabled` | `'853500899556 }}` | Eliminato: template_type dal DB, non ID hardcoded |
| `recuperaLingua()` tautology | `!= '' \|\| != null` | Backend: proper null/empty check |
| `hs_sender_email` undefined | `owner.selectedOptionLabel` su plain object | Backend: owner lookup dal DB |
| `==` vs `===` in role check | Loose equality | Backend: typed comparison in Go |
| `cli_orders` unscoped | Manca filtro cliente | Backend: filtro `NUMERO_AZIENDA` (A7) |
| Month off-by-one in PDF filename | `date.getMonth()` 0-based | Out of scope (order conversion) |
| Duplicate order guard disabled | Commented out check | Out of scope (order conversion) |
| Category exclusion inconsistency | 12,13 vs 12,13,14,15 | Backend: sempre 12,13,14,15 (A5) |
| `i_next_term_months` type mismatch | TEXT vs NUMBER | Backend: typed `smallint`, frontend `number` input |
| Alyante payment query fires on empty ID | `onPageLoad` senza guardia | Backend: endpoint richiede `customer_id` valido |
| HubSpot expiry date mutation | `currentDate.setDate(...)` muta l'oggetto | Backend: calcolo immutabile `document_date + 30 days` |

---

## 7. Summary: Backend API Surface

Consolidamento delle 30+ query Appsmith in endpoint backend:

### Read endpoints

| Endpoint | Replaces | Notes |
|---|---|---|
| `GET /api/quotes/v1/quotes` | `get_quotes` | Paginated, filterable (status, owner, search). Joins company/deal/owner. |
| `GET /api/quotes/v1/quotes/:id` | `get_quote_by_id` | Full quote header |
| `GET /api/quotes/v1/quotes/:id/rows` | `get_quote_rows` | Kit rows for quote |
| `GET /api/quotes/v1/quotes/:id/rows/:rowId/products` | `get_quote_products_grouped` | Grouped products via view |
| `GET /api/quotes/v1/deals` | `get_potentials` / `get_deals` | Filtered by pipeline/stage constants |
| `GET /api/quotes/v1/deals/:id` | `get_potential_by_id` | Deal detail with ERP cross-ref |
| `GET /api/quotes/v1/customers` | `get_customers` | Company list |
| `GET /api/quotes/v1/owners` | `get_hs_owners` | Owner list |
| `GET /api/quotes/v1/templates` | `get_templates` + `template_suServizio()` | Filterable by type (standard/iaas), lang, is_colo |
| `GET /api/quotes/v1/categories` | `get_product_category` | Filtered (excl 12,13,14,15 for standard) |
| `GET /api/quotes/v1/kits` | `get_kit_internal_names` / `list_kit` | Active, non-ecommerce, quotable |
| `GET /api/quotes/v1/payment-methods` | `get_payment_method` | From `loader.erp_metodi_pagamento` |
| `GET /api/quotes/v1/customer-payment/:customerId` | `get_pagamento_anagrCli` | Alyante ERP default payment |
| `GET /api/quotes/v1/customer-orders/:customerId` | `cli_orders` | Alyante orders filtered by customer (A7 fix) |

### Write endpoints

| Endpoint | Replaces | Notes |
|---|---|---|
| `POST /api/quotes/v1/quotes` | `ins_quote` + `ins_quote_rows` (wizard) | Creates complete quote: header + kit rows + product expansion (trigger). Returns full quote. |
| `PUT /api/quotes/v1/quotes/:id` | `upd_quote` (salvaOfferta) | Updates quote header. Validates business rules (COLOCATION→billing, IaaS lock, spot→MRC). |
| `PUT /api/quotes/v1/quotes/:id/rows/:rowId/products/:productId` | `upd_quote_row_product` | Updates single product. Enforces mutual exclusion, quantity floor, MRC=0 for spot. |
| `POST /api/quotes/v1/quotes/:id/rows` | `ins_quote_rows` (add kit) | Adds kit row to existing quote. Trigger expands products. |
| `DELETE /api/quotes/v1/quotes/:id/rows/:rowId` | `del_quote_row` | Deletes kit row (CASCADE to products). |
| `PUT /api/quotes/v1/quotes/:id/rows/:rowId/position` | `upd_quote_row_position` | Updates row ordering. |
| `DELETE /api/quotes/v1/quotes/:id` | `Cancella_Offerta` + `Cancella_HS_Quote` | RBAC-gated. Orchestrates HS delete → DB delete. |

### Action endpoints

| Endpoint | Replaces | Notes |
|---|---|---|
| `POST /api/quotes/v1/quotes/:id/publish` | `mandaSuHubspot()` + `hs_save_all_line_items()` | Full publish orchestration. Idempotent retry. Returns step-by-step progress. |
| `GET /api/quotes/v1/quotes/:id/hs-status` | `hs_get_quote_status` | Fetches current HS quote status, PDF link |

### Estimated total: ~16 endpoints (vs 30+ Appsmith queries + 8 JSObjects)
