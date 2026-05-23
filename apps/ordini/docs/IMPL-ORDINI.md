# Ordini — Piano finale di implementazione

> **Target:** `apps/ordini/` + `backend/internal/ordini/`  
> **Spec sorgente:** `apps/ordini/audit/ordini-migspec-phaseE-spec.md`  
> **Data:** 2026-05-23  
> **Stato:** piano finale pronto per implementation planning / pre-gate UI review

---

## 0. Sintesi decisionale

Implementare la mini-app **Ordini** come porting 1:1 delle sole superfici Appsmith approvate:

- `Home` / Lista ordini
- `Dettaglio ordine`

Tutte le feature morte, incomplete o post-v1 restano escluse.

### Decisioni già chiuse

| Tema | Decisione finale |
|---|---|
| API standard | Browser: `/api/ordini/v1/...`; Go mux: `/ordini/v1/...` perché `main.go` strip-prefixa `/api`. |
| GW internal CDLAN | Usare il client condiviso `backend/internal/platform/arak.Client` con env `ARAK_*`. Nessun `GW_INT_*` o client auth dedicato. |
| Route frontend | App mount `/apps/ordini/`; route interne `/ordini` e `/ordini/:id`. |
| Port dev | Vite `5192`. |
| Vite base | `base: command === 'build' ? '/apps/ordini/' : '/'`. |
| Ruolo base | `app_ordini_access`. |
| Ruolo elevato | `app_customer_relations`. |
| Assegnazione ruoli | Ogni utente `app_customer_relations` riceve anche `app_ordini_access`; l'app gate resta su `app_ordini_access`. |
| Cancel order | Non in v1; rimandato a `docs/TODO.md`. |
| Ordine perso | Non in v1. |
| Retry righe ERP fallite | Non in v1; rimandato a `docs/TODO.md`. |
| Order creation | Fuori scope; vive in Quotes / Customer Portal / ERP. |
| DB migrations | Nessuna. |
| Catalog status iniziale | `test`; flip a `ready` solo dopo UI post-gate e verifica manuale. |

---

## 1. Scope v1

### In scope

- Lista ordini da Vodka MySQL.
- Dettaglio ordine con 5 tab:
  - Info
  - Azienda
  - Referenti
  - Righe
  - Informazioni dai tecnici
- BOZZA header save:
  - `cdlan_rif_ordcli`
  - `cdlan_dataconferma`
  - `cdlan_cliente`
  - `cdlan_cliente_id`
- Referenti save in `BOZZA` o `INVIATO`.
- `INVIA in ERP`:
  - loop riga-per-riga verso GW `/orders/v1/erp`;
  - state flip Vodka solo su full success;
  - upload PDF ad Arxivar solo su full success;
  - outcome strutturato per riga.
- Per-row activation:
  - update Vodka row;
  - call GW `/orders/v1/set-order-activation`;
  - auto-promozione ordine ad `ATTIVO` quando tutte le righe risultano confermate/cancellate/a quantità zero.
- Inline edit:
  - `cdlan_serialnumber` solo in `BOZZA`;
  - `note_tecnici` in ogni stato.
- PDF backend-proxied:
  - kickoff;
  - activation form;
  - order PDF pre-Arxivar;
  - signed PDF da Arxivar.
- Dropdown clienti da Alyante.
- Optional quote-origin pointer via Mistra `orders.legacy_orders`.

### Out of scope v1

- `RICHIEDI ANNULLAMENTO`.
- `ORDINE PERSO`.
- Retry solo righe ERP fallite.
- Server-side pagination Home.
- Order creation.
- Migrazione dati `cdlan_int_fatturazione = 5 -> 4`.

---

## 2. Comparable Apps Audit

### Reference 1 — RDA

File/pattern da riusare:

- `apps/rda/src/pages/RdaListPage.tsx`
- `apps/rda/src/pages/PoDetailPage.tsx`
- `apps/rda/src/components/PoTabs.tsx`
- `apps/rda/src/components/POCommandBar.tsx`
- `apps/rda/src/components/PoListTable.tsx`
- `apps/rda/vite.config.ts`

Pattern utili:

- lista con toolbar, filtri, search e tabella;
- detail con tab e action bar;
- azioni gated da ruolo/stato;
- toast di successo/errore;
- download protetti via blob/fetch e Bearer token;
- CSS clean mini-app, niente hero/KPI decorativi.

Pattern da non riusare:

- inbox multiple;
- comments panel;
- wizard di creazione;
- provider side panels.

### Reference 2 — AFC Tools / Ordini Sales

File/pattern da riusare:

- `apps/afc-tools/src/pages/OrdiniSalesPage.tsx`
- `apps/afc-tools/src/pages/OrdiniSalesDetailPage.tsx`
- `backend/internal/afctools/handler.go`
- `backend/internal/afctools/vodka.go`
- `backend/internal/afctools/gateway.go`

Pattern utili:

- letture da Vodka MySQL;
- scanner/null handling per dati legacy;
- layout data-table + detail;
- proxy backend per PDF GW;
- error mapping lato backend.

Pattern da non riusare:

- filtering AFC a soli ordini `ATTIVO/INVIATO`;
- SQL-side label mapping;
- modello read-only: Ordini v1 è write/lifecycle-heavy.

### Reference 3 — Quotes

File/pattern da riusare:

- `backend/internal/quotes/handler.go`
- `backend/internal/quotes/order_conversion.go`
- `apps/quotes/vite.config.ts`

Pattern utili:

- `Deps` esplicite con `Vodka`, `Alyante`, `Mistra`, `Arak`;
- route `/quotes/v1/...` come standard API versionato;
- `orders.legacy_orders` writer: Ordini sarà solo reader del back-pointer;
- build base condizionale Vite.

Pattern da non riusare:

- HubSpot publish/conversion logic;
- create wizard;
- direct ownership del dominio quotes.

---

## 3. Archetype e UI rules

### Archetype primario

`master_detail_crud`

Motivo:

- una registry/lista di ordini;
- un detail per singolo ordine;
- mutazioni scoped all'ordine o alle sue righe;
- niente report explorer, dashboard o wizard.

Il dettaglio tabbed è una eccezione di layout interna al detail, non un secondo archetipo.

### Copy policy

UI copy in italiano, business-user-only.

Ammesso:

- `Ordini`
- `Lista ordini`
- `Dettaglio ordine`
- `Ragione sociale`
- `Data conferma`
- `Invia in ERP`
- `Numero seriale`
- `Note tecniche`
- `Modulo di attivazione`
- `Torna agli ordini`

Vietato in UI:

- `payload`
- `datasource`
- `widget`
- `record`
- `gw-int`
- `vodka`
- `alyante`
- nomi handler/backend;
- raw enum dove esiste label business.

### Metrics policy

Nessuna KPI card o summary decorativa.

Ammessi solo:

- badge stato reale ordine;
- eventuale badge con numero righe nel tab `Righe`;
- conteggio outcome nel pannello post `INVIA in ERP`.

### Exceptions

- **Detail tabbed dentro `master_detail_crud`**: ammesso come eccezione interna al detail perché separa campi Info/Azienda/Referenti/Righe/Note tecniche senza introdurre un secondo archetipo.
- **Pannello outcome `INVIA in ERP`**: ammesso solo dopo l'azione, perché rende visibili esiti per-riga e partial failure; non è una KPI card o riepilogo decorativo.
- Nessuna eccezione alla copy policy: UI sempre in italiano business-user-only, senza termini tecnici di implementazione.

---

## 4. Repo-fit finale

### Runtime fit

| Elemento | Decisione |
|---|---|
| Portal href | `/apps/ordini/` |
| Vite build base | `/apps/ordini/` |
| Vite dev base | `/` |
| Route interne | `/ordini`, `/ordini/:id` |
| URL prod detail | `/apps/ordini/ordini/:id` |
| URL dev detail | `http://localhost:5192/ordini/:id` |
| SPA fallback | `staticspa.New(cfg.StaticDir)` già supporta `/apps/<slug>/**`. |

### API fit

Browser-facing:

```text
/api/ordini/v1/...
```

Go mux patterns:

```text
/ordini/v1/...
```

perché `backend/cmd/server/main.go` monta:

```go
http.StripPrefix("/api", api)
```

### Dev fit

- Vite port: `5192`.
- Proxy Vite:
  - `/api` -> backend `localhost:8080`
  - `/config` -> backend `localhost:8080`
- Root script da aggiungere:
  - `dev:ordini`: `pnpm --filter mrsmith-ordini dev`
- Il root `dev` esistente con `pnpm --filter './apps/*' --parallel --if-present dev` includerà automaticamente `apps/ordini` quando il package esiste.
- Make target da aggiungere:
  - `dev-ordini`.
- `CORS_ORIGINS` default in `backend/internal/platform/config/config.go` deve includere `http://localhost:5192`.

### Deployment fit

`deploy/Dockerfile` va aggiornato esplicitamente:

```dockerfile
COPY --from=frontend /app/apps/ordini/dist /static/apps/ordini
```

Non assumere copy automatica `apps/*/dist`: il Dockerfile attuale copia ogni app esplicitamente.

### Env/config fit

Ordini è una SPA interna servita dal backend come Quotes/RDA in produzione, con override env opzionale allineato alla convenzione delle altre 19 mini-app:

- catalog href prod/static: `/apps/ordini/` (default da `applaunch.OrdiniAppHref`);
- override esplicito: `ORDINI_APP_URL` (vuoto in produzione, valorizzabile per scenari custom);
- split-server dev (`cfg.StaticDir == ""`) e `ORDINI_APP_URL` vuoto: `backend/cmd/server/main.go` imposta href launcher a `http://localhost:5192`.

Il pattern segue lo stesso `if/else if` usato per Budget/Fornitori/RDA/.../AFCTools in `main.go:339-429`. Verificato in `backend/internal/platform/config/config.go:29-46` + `168-185` che tutte e 19 le mini-app interne dichiarano il proprio `*AppURL` field + env loader; Ordini si allinea per coerenza ops/devops.

Nuovo env:

```bash
ORDINI_APP_URL=
```

Env riusati:

```bash
VODKA_DSN=
ALYANTE_DSN=
MISTRA_DSN=
ARAK_BASE_URL=
ARAK_SERVICE_TOKEN_URL=
ARAK_SERVICE_CLIENT_ID=
ARAK_SERVICE_CLIENT_SECRET=
```

Non aggiungere:

```bash
GW_INT_BASE_URL=
GW_INT_USERNAME=
GW_INT_PASSWORD=
ORDINI_GW_*
```

### Catalog visibility

Decisione finale:

- nascondere Ordini se `VODKA_DSN` manca, perché senza Vodka non esiste il dato primario;
- mantenere visibile con `ALYANTE_DSN`, `MISTRA_DSN` o `ARAK_*` mancanti, ma gli endpoint dipendenti rispondono `503` con errore sanitizzato:
  - `alyante_database_not_configured`;
  - `gateway_not_configured`;
  - origin quote omesso se `MISTRA_DSN` assente.

Questa scelta consente lettura lista/detail anche in ambienti read-only parziali.

---

## 5. API contract finale

### Read endpoints

| Method | Browser path | Go mux path | Auth |
|---|---|---|---|
| GET | `/api/ordini/v1/orders` | `/ordini/v1/orders` | `app_ordini_access` |
| GET | `/api/ordini/v1/orders/:id` | `/ordini/v1/orders/{id}` | `app_ordini_access` |
| GET | `/api/ordini/v1/orders/:id/rows` | `/ordini/v1/orders/{id}/rows` | `app_ordini_access` |
| GET | `/api/ordini/v1/orders/:id/technical-rows` | `/ordini/v1/orders/{id}/technical-rows` | `app_ordini_access` |
| GET | `/api/ordini/v1/ref/customers` | `/ordini/v1/ref/customers` | `app_ordini_access` |
| GET | `/api/ordini/v1/orders/:id/kickoff.pdf` | `/ordini/v1/orders/{id}/kickoff.pdf` | `app_customer_relations` + `INVIATO` |
| GET | `/api/ordini/v1/orders/:id/activation-form.pdf` | `/ordini/v1/orders/{id}/activation-form.pdf` | `app_customer_relations` + `INVIATO/ATTIVO` |
| GET | `/api/ordini/v1/orders/:id/pdf` | `/ordini/v1/orders/{id}/pdf` | `app_ordini_access` + `arx_doc_number IS NULL` |
| GET | `/api/ordini/v1/orders/:id/signed-pdf` | `/ordini/v1/orders/{id}/signed-pdf` | `app_ordini_access` + `arx_doc_number IS NOT NULL` |

### Write endpoints

| Method | Browser path | Go mux path | Auth + state |
|---|---|---|---|
| PATCH | `/api/ordini/v1/orders/:id` | `/ordini/v1/orders/{id}` | `app_customer_relations` + `BOZZA` |
| PATCH | `/api/ordini/v1/orders/:id/referents` | `/ordini/v1/orders/{id}/referents` | `app_customer_relations` + `BOZZA/INVIATO` |
| POST | `/api/ordini/v1/orders/:id/send-to-erp` | `/ordini/v1/orders/{id}/send-to-erp` | `app_customer_relations` + `BOZZA` + precondizioni |
| PATCH | `/api/ordini/v1/orders/:id/rows/:rowId/serial-number` | `/ordini/v1/orders/{id}/rows/{rowId}/serial-number` | `app_ordini_access` + `BOZZA` |
| PATCH | `/api/ordini/v1/orders/:id/rows/:rowId/technical-notes` | `/ordini/v1/orders/{id}/rows/{rowId}/technical-notes` | `app_ordini_access`, any state |
| PATCH | `/api/ordini/v1/orders/:id/rows/:rowId/activate` | `/ordini/v1/orders/{id}/rows/{rowId}/activate` | `app_customer_relations` + `INVIATO` |

---

## 6. Backend architecture

### Package layout

```text
backend/internal/ordini/
  handler.go              # Deps, Handler, RegisterRoutes, helpers comuni
  types.go                # DTO, request/response, enum aliases
  store_orders.go         # Vodka order reads/writes
  store_rows.go           # Vodka row reads/writes
  store_customers.go      # Alyante customer dropdown
  store_origin.go         # Mistra origin lookup: orders.legacy_orders -> quotes.quote.quote_number
  permissions.go          # role/state/precondition helpers
  workflow_send.go        # send-to-ERP orchestration
  workflow_activate.go    # row activation + auto-ATTIVO
  gateway.go              # typed wrapper sopra *arak.Client
  pdf.go                  # PDF proxy + normalization
  scanners.go             # legacy null/date/decimal helpers
```

### Deps

```go
type Deps struct {
    Vodka   *sql.DB
    Alyante *sql.DB
    Mistra  *sql.DB
    Arak    *arak.Client
    Logger  *slog.Logger
}
```

Nessuno stato package-global.

### Route registration

```go
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
    h := &Handler{...}
    protect := acl.RequireRole(applaunch.OrdiniAccessRoles()...)

    handle := func(pattern string, fn http.HandlerFunc) {
        mux.Handle(pattern, protect(http.HandlerFunc(fn)))
    }

    handle("GET /ordini/v1/orders", h.handleListOrders)
    // ...
}
```

Gli handler elevati controllano `app_customer_relations` internamente per poter restituire errori business più chiari.

---

## 7. Data contracts principali

### Order summary

Usato da `GET /api/ordini/v1/orders`.

Campi minimi:

```ts
interface OrderSummary {
  id: number;
  cdlan_systemodv: string | null;
  cdlan_tipodoc: string | null;
  cdlan_ndoc: number | string | null;
  cdlan_anno: number | null;
  codice_ordine: string;
  cdlan_sost_ord: string | null;
  cdlan_cliente: string | null;
  cdlan_cliente_id: number | null;
  cdlan_datadoc: string | null;
  service_type: string | null;
  is_colo: string | null;
  cdlan_tipo_ord: string | null;
  cdlan_dataconferma: string | null;
  cdlan_stato: 'BOZZA' | 'INVIATO' | 'ATTIVO' | 'PERSO' | 'ANNULLATO';
  profile_lang: 'it' | 'en' | string | null;
  cdlan_evaso: 0 | 1 | null;
  from_cp: 0 | 1 | null;
  arx_doc_number: string | null;
}
```

Backend restituisce raw codes; frontend formatta label.

### Order detail

Estende summary e include:

- campi Info;
- campi Azienda/Profile;
- campi Referenti;
- `cdlan_int_fatturazione` e `cdlan_int_fatturazione_att` raw;
- `cdlan_dur_rin`, `cdlan_tacito_rin`;
- `data_decorrenza`;
- optional `origin`.

Origin:

```json
{
  "origin": {
    "type": "quote",
    "quote_id": 1234,
    "quote_code": "ABC-2026-0042",
    "quote_url": "/apps/quotes/quotes/1234"
  }
}
```

Resolver origin definitivo in `store_origin.go`: due query Mistra sequenziali, senza join cross-DB e senza chiamate HTTP tra package.

```sql
SELECT quote_id
FROM orders.legacy_orders
WHERE vodka_id = $1
LIMIT 1;
```

poi:

```sql
SELECT quote_number
FROM quotes.quote
WHERE id = $1;
```

`quote_number` alimenta il DTO `origin.quote_code`. Se `MISTRA_DSN` è assente o non esiste una riga `legacy_orders`, `origin` viene omesso. Se il bridge esiste ma la quote non è più risolvibile, restituire comunque `quote_id`/`quote_url` e omettere `quote_code`, loggando un warning sanitizzato.

### Order row

```ts
interface OrderRow {
  id: number;
  orders_id: number;
  cdlan_systemodv_row: number | null;
  cdlan_codice_kit: string | null;
  index_kit: number | null;
  bundle_code: string | null;
  cdlan_codart: string | null;
  cdlan_descart: string | null;
  cdlan_qta: number | null;
  canone: number | null;
  activation_price: number | null;
  termination_price: number | null;
  cdlan_ragg_fatturazione: string | null;
  cdlan_data_attivazione: string | null;
  cdlan_serialnumber: string | null;
  confirm_data_attivazione: 0 | 1 | null;
  data_annullamento: string | null;
}
```

`activation_price` è il nome canonico del DTO; label UI: `Prezzo attivazione`.

### Technical row

```ts
interface TechnicalRow {
  id: number;
  cdlan_systemodv_row: number | null;
  bundle_code: string | null;
  cdlan_codart: string | null;
  cdlan_descart: string | null;
  note_tecnici: string | null;
  data_annullamento: string | null;
}
```

Read query deve preservare `CONVERT(note_tecnici USING UTF8)` o equivalente verificato.

### Customer ref

```ts
interface CustomerRef {
  id: number;      // Alyante NUMERO_AZIENDA
  name: string;    // RAGIONE_SOCIALE
}
```

Filtro Alyante:

```sql
DATA_DISMISSIONE IS NULL
AND RAGGRUPPAMENTO_3 <> 'Ecommerce'
AND TIPOLOGIA_AZIENDA <> 'DIPENDENTE'
```

---

## 8. Role/state matrix

| Action | `app_ordini_access` | `app_customer_relations` | Stato |
|---|---:|---:|---|
| Lista ordini | sì | sì | any |
| Dettaglio ordine | sì | sì | any |
| Edit `note_tecnici` | sì | sì | any |
| Edit `cdlan_serialnumber` | sì | sì | `BOZZA` |
| Info SALVA | no | sì | `BOZZA` |
| Referenti SALVA | no | sì | `BOZZA`, `INVIATO` |
| INVIA in ERP | no | sì | `BOZZA` + precondizioni |
| Per-row activation | no | sì | `INVIATO` |
| Kickoff PDF | no | sì | `INVIATO` |
| Activation form PDF | no | sì | `INVIATO`, `ATTIVO` |
| Order PDF pre-Arxivar | sì | sì | `arx_doc_number IS NULL` |
| Signed PDF | sì | sì | `arx_doc_number IS NOT NULL` |
| Arxivar file picker | no | sì | stato non in `{ANNULLATO,PERSO,ATTIVO}` |

Frontend gates sono advisory. Backend è autoritativo.

Provisioning Keycloak: `app_customer_relations` è un ruolo elevato, non un ruolo di ingresso standalone. Ogni utente CR deve avere anche `app_ordini_access`; per questo launcher, AppShell e ACL base restano agganciati a `app_ordini_access`, mentre i singoli handler elevati controllano `app_customer_relations`.

---

## 9. Business rules deliberatamente riviste

| ID | Source Appsmith | Rewrite |
|---|---|---|
| Q2 | `data_annullamento <> null` | `data_annullamento IS NOT NULL` |
| Q3 | drift `4` vs `5` Quadrimestrale | read formatter accetta entrambi; nessuna migration |
| Q4 | alias `Attivazione` / `Prezzo attivazione` | DTO `activation_price`, label UI `Prezzo attivazione` |
| Q5 | role client-side `CustomerRelations` | Keycloak role `app_customer_relations`, enforced backend |
| Q6 | Arxivar picker OR-chain bug | `state NOT IN {ANNULLATO,PERSO,ATTIVO} AND app_customer_relations` |
| Q8/C1 | partial failure poco visibile | response per-riga + panel UI |
| C2 | salva solo `cdlan_cliente` string | salva `cdlan_cliente` + `cdlan_cliente_id` |
| ERP state | manda `cdlan_stato = local state`? | mantenere hard-code `CREATO` verso ERP |
| Arxivar doc number | write path invisibile | Ordini non scrive `arx_doc_number` |

---

## 10. Workflow: INVIA in ERP

Endpoint:

```text
POST /api/ordini/v1/orders/:id/send-to-erp
```

Input:

```text
multipart/form-data
file=<signed PDF>
```

Precondizioni server-side:

- user ha `app_customer_relations`;
- ordine esiste;
- `cdlan_stato == 'BOZZA'`;
- `cdlan_dataconferma` valorizzata;
- cliente valorizzato;
- PDF presente e valido.

Flusso:

1. Load order da Vodka.
2. Validate role/stato/precondizioni.
3. Parse multipart PDF.
4. Load rows da `orders_rows`.
5. Per ogni riga, sequenzialmente:
   - costruire payload GW con campi header + row;
   - forzare `cdlan_stato = "CREATO"` nel payload GW;
   - chiamare `POST /orders/v1/erp` via `arak.Client`;
   - accumulare outcome `{rowId, cdlan_systemodv_row, status, error?}`.
6. Se almeno una riga fallisce:
   - non aggiornare Vodka;
   - non uploadare Arxivar;
   - restituire response con `stateTransitioned=false`, `arxivarUploaded=false`.
7. Se tutte le righe sono ok:
   - `UPDATE orders SET cdlan_stato='INVIATO', cdlan_evaso=1 WHERE id=? AND cdlan_stato='BOZZA'`;
   - chiamare `POST /orders/v1/send-to-arxivar` via `arak.Client`;
   - se upload Arxivar fallisce dopo state flip, non rollbackare Vodka; restituire `warning="arxivar_upload_failed"` e log warning.

Response consigliata sempre `200 OK` con stato nel payload:

```json
{
  "rows": [
    { "rowId": 1, "cdlan_systemodv_row": 100, "status": "ok" },
    { "rowId": 2, "cdlan_systemodv_row": 101, "status": "error", "error": "..." }
  ],
  "stateTransitioned": false,
  "arxivarUploaded": false,
  "warning": ""
}
```

Motivo per evitare `207`: contract frontend più semplice e coerente con Appsmith source; lo stato reale vive nel payload.

---

## 11. Workflow: activation row + auto-ATTIVO

Endpoint:

```text
PATCH /api/ordini/v1/orders/:id/rows/:rowId/activate
```

Input:

```json
{ "activation_date": "2026-05-23" }
```

Precondizioni:

- user ha `app_customer_relations`;
- ordine `INVIATO`;
- riga appartiene all'ordine (`orders_rows.orders_id = :id`).
- riga non annullata e con quantità diversa da zero. Le righe annullate o a quantità zero sono già considerate soddisfatte dalla regola Q2 e non devono aprire una nuova attivazione.

Decisione v1 su righe già confermate:

- le righe con `confirm_data_attivazione = 1` restano modificabili in v1 quando l'ordine è `INVIATO` e l'utente è `app_customer_relations`;
- questa è una conservazione intenzionale del comportamento Appsmith del pulsante `Modifica`, che consente correggere la data di attivazione;
- non aggiungere un blocco server-side sulle righe già confermate senza una nuova conferma di dominio, perché cambierebbe un flusso operativo esistente.

Flusso consigliato:

1. Load order + row.
2. Validate state/role/ownership.
3. Aprire transazione Vodka per le write locali.
4. Update row:
   ```sql
   UPDATE orders_rows
   SET cdlan_data_attivazione = ?, confirm_data_attivazione = 1
   WHERE id = ? AND orders_id = ?
   ```
5. Chiamare GW `POST /orders/v1/set-order-activation` via `arak.Client`.
6. Se GW fallisce, rollback Vodka e risposta `502 gateway_error`.
7. Count confirmed rows con Q2 fix:
   ```sql
   SELECT COUNT(id)
   FROM orders_rows
   WHERE orders_id = ?
     AND (
       confirm_data_attivazione = 1
       OR data_annullamento IS NOT NULL
       OR cdlan_qta = 0
     )
   ```
8. Se `confirmed == total`, aggiornare ordine:
   ```sql
   UPDATE orders SET cdlan_stato='ATTIVO'
   WHERE id=? AND cdlan_stato='INVIATO'
   ```
9. Commit.
10. Restituire row/order state aggiornati.

Boundary da documentare in codice:

- se GW riesce ma commit DB fallisce, ERP è avanti rispetto a Vodka;
- loggare con request ID e restituire errore sanitizzato;
- retry manuale deve essere idempotente lato business perché data attivazione è per riga.

---

## 12. Workflow: BOZZA header save / C2

Endpoint:

```text
PATCH /api/ordini/v1/orders/:id
```

Body:

```json
{
  "customer_po": "PO-123",
  "confirmation_date": "2026-05-23",
  "customer_id": 12345
}
```

Regola critica:

- il frontend manda `customer_id`;
- il backend rilegge `RAGIONE_SOCIALE` da Alyante usando `NUMERO_AZIENDA = customer_id`;
- il backend non si fida di un eventuale nome cliente inviato dal client.

Write:

```sql
UPDATE orders
SET cdlan_rif_ordcli = ?,
    cdlan_dataconferma = ?,
    cdlan_cliente_id = ?,
    cdlan_cliente = ?
WHERE id = ? AND cdlan_stato = 'BOZZA'
```

Se Alyante è assente o cliente non trovato: non salvare dati parziali.

---

## 13. PDF proxy rules

Tutti i PDF passano dal backend, mai da link diretto GW o anchor `/api` senza Bearer.

| Endpoint | GW path | Filename | Gate |
|---|---|---|---|
| kickoff | `/orders/v1/kick-off/{id}` | `kick off_<ndoc>_<anno>.pdf` | CR + `INVIATO` |
| activation form | `/orders/v1/activation-form/{id}` | IT `Modulo di Attivazione_<ndoc>_<anno>.pdf`; EN `Activation Form_<ndoc>_<anno>.pdf` | CR + `INVIATO/ATTIVO` |
| order pdf | `/orders/v1/order/pdf/{id}/generate` | `<ndoc>_<anno>.pdf` | access + `arx_doc_number IS NULL` |
| signed pdf | `/orders/v1/order/pdf/{id}?from=vodka` | `<ndoc>_<anno>_firmato.pdf` | access + `arx_doc_number IS NOT NULL` |

Backend normalization:

- se body inizia con `%PDF`, stream raw;
- se body è base64, decode server-side;
- se body è wrapper JSON legacy, estrarre e decode;
- client riceve sempre `application/pdf`.

---

## 14. Frontend architecture

### Files

```text
apps/ordini/
  package.json
  vite.config.ts
  tsconfig.json
  tsconfig.build.json
  index.html
  src/
    main.tsx
    App.tsx
    routes.tsx
    vite-env.d.ts
    api/
      client.ts
      queries.ts
      types.ts
      pdf.ts
    hooks/
      useOptionalAuth.ts
    lib/
      formatters.ts
      permissions.ts
      downloads.ts
      errors.ts
    pages/
      OrderListPage.tsx
      OrderListPage.module.css
      OrderDetailPage.tsx
      OrderDetailPage.module.css
    components/
      OrdersTable.tsx
      DetailHeader.tsx
      InfoTab.tsx
      AziendaTab.tsx
      ReferentiTab.tsx
      RigheTab.tsx
      TechnicalNotesTab.tsx
      ActivationModal.tsx
      SendToErpResultPanel.tsx
      CustomerSelect.tsx
      StatusBadge.tsx
    styles/
      global.css
```

### Bootstrap

- `main.tsx` fetch `/config`.
- `AuthProvider`, `QueryClientProvider`, `BrowserRouter`, `ToastProvider`.
- Browser basename derivato da `import.meta.env.BASE_URL`.
- Query retry policy come AFC/RDA:
  - no retry su real backend 401/403;
  - consentire retry su local preflight unauthorized se pattern esistente lo prevede.

### App gate

`App.tsx` deve usare:

```ts
getAppAccessState(auth, APP_ACCESS_ROLES.ordini)
```

Non renderizzare route/nav finché access state non è `allowed`.

Aggiungere in `packages/auth-client/src/roles.ts`:

```ts
ordini: ['app_ordini_access']
```

`app_customer_relations` non viene aggiunto all'app gate perché gli utenti CR sono provisionati anche con `app_ordini_access`; resta invece controllato dalle permission action-specifiche.

### Routes

```tsx
const routes = [
  { path: '/', element: <Navigate to="/ordini" replace /> },
  { path: '/ordini', element: <OrderListPage /> },
  { path: '/ordini/:id', element: <OrderDetailPage /> },
  { path: '*', element: <Navigate to="/ordini" replace /> },
];
```

### Formatters

App-local only:

- `formatStato`
- `formatTipoProposta`
- `formatTipoDoc`
- `formatFatturazione` (`4` e `5` -> Quadrimestrale)
- `formatFatturazioneAtt`
- `formatDurRin`
- `formatSiNo`
- `formatIsColo`
- `formatServiceTypes`
- `formatDate`
- `formatMoney`

### Permissions mirror

Frontend mirror delle regole backend:

- `canEditBozzaHeader`
- `canSendToErp`
- `canEditReferents`
- `canEditSerialNumber`
- `canEditTechnicalNotes`
- `canOpenActivationModal`
- `canShowArxivarFilePicker`
- `canDownloadKickoffPdf`
- `canDownloadActivationFormPdf`

---

## 15. UI composition

### Home / Lista ordini

- Compact header `Ordini` / `Lista ordini`.
- Toolbar:
  - search;
  - stato filter;
  - tipo documento/proposta filter se leggero.
- Tabella con 15 colonne dalla spec.
- Client-side search/sort/pagination.
- Row action `Visualizza` -> `/ordini/:id`.
- Loading skeleton.
- Empty state standard.
- No create button.
- No KPI cards.

### Detail header

- `Torna agli ordini`.
- Titolo `Codice ordine: <ndoc>/<anno>`.
- Stato badge.
- Ragione sociale.
- Optional origin link `Da proposta <quote_code>`.
- PDF actions gated.

### Info tab

- Metadata readonly.
- Editable BOZZA block:
  - Rif. ordine cliente;
  - Data conferma;
  - Ragione sociale;
  - secondary line `ID cliente: ...`.
- Arxivar external link se `arx_doc_number` presente.
- File picker PDF + `INVIA in ERP`.
- `SendToErpResultPanel` su partial failure.
- No cancel/lost buttons.

### Azienda tab

Readonly profile fields:

- P.IVA;
- CF;
- indirizzo;
- città;
- CAP;
- provincia;
- SDI;
- eventuale lingua come label se utile.

No hidden inputs.

### Referenti tab

Tre gruppi:

- Tecnico;
- Altro tecnico;
- Amministrativo.

Campi: nome, telefono, email.

Save gated CR + `BOZZA/INVIATO`.

### Righe tab

- Tabella righe.
- Preserve line breaks in description.
- Serial inline edit solo `BOZZA`.
- `Modifica` activation solo `INVIATO` + CR.
- Activation modal con date required.

### Informazioni dai tecnici tab

- Note tecniche inline editable.
- Data annullamento readonly.
- Disponibile a ogni `app_ordini_access`.

---

## 16. Platform wiring tasks

### Backend

1. `backend/internal/platform/applaunch/catalog.go`
   - costanti: `OrdiniAppID = "ordini"`, `OrdiniAppHref = "/apps/ordini/"`
   - due slice package-level **separate**:
     ```go
     ordiniAccessRoles      = []string{"app_ordini_access"}
     customerRelationsRoles = []string{"app_customer_relations"}
     ```
   - due getter pubblici seguendo il pattern di `QuotesAccessRoles()` / `QuotesDeleteRoles()`:
     ```go
     func OrdiniAccessRoles() []string      { return ordiniAccessRoles }
     func CustomerRelationsRoles() []string { return customerRelationsRoles }
     ```
   - catalog entry in categoria MKT&Sales, `AccessRoles: OrdiniAccessRoles()` (solo l'access role; CR è capability orthogonal, non un access role)
   - `AllRoles()` `groups`: aggiungere `ordiniAccessRoles` **e** `customerRelationsRoles` come gruppi separati. Motivo: convenzione esistente per ruoli capability (es. `quotesDeleteRoles`, `trainingPeopleAdminRoles`, `manutenzioniManagerRoles`) che vivono fianco a fianco con i propri access roles in `groups`.
   - **Non** accorpare CR dentro `ordiniAccessRoles`: rompe la semantica del campo `AccessRoles` nel catalogo e rende il ruolo non riusabile da future app.

2. `backend/internal/platform/config/config.go`
   - aggiungere campo `OrdiniAppURL string` (allineato alfabeticamente/per-app con gli altri `*AppURL`)
   - aggiungere loader `OrdiniAppURL: envOr("ORDINI_APP_URL", "")`
   - CORS default include `http://localhost:5192`
   - Motivazione: tutte e 19 le mini-app interne attuali hanno il proprio `*_APP_URL` env override (verificato in `config.go:29-46` + `168-185`). Ordini si allinea alla convenzione per coerenza ops/devops.

3. `backend/cmd/server/main.go`
   - import `ordini`
   - aggiungere il blocco standard allineato agli altri 19 app (vedi `main.go:339-429`):
     ```go
     if cfg.OrdiniAppURL != "" {
         hrefOverrides[applaunch.OrdiniAppID] = cfg.OrdiniAppURL
     } else if cfg.StaticDir == "" {
         hrefOverrides[applaunch.OrdiniAppID] = "http://localhost:5192"
     }
     ```
   - catalog filter per Vodka missing (`cfg.VodkaDSN == ""` → skip definition)
   - `ordini.RegisterRoutes(api, ordini.Deps{...})`

4. Env examples
   - `backend/.env.example`: aggiungere `ORDINI_APP_URL=` (stile esistente, commentato o vuoto come gli altri)
   - `.env.preprod.example`: aggiungere `ORDINI_APP_URL=`
   - verificare che gli env riusati (`VODKA_DSN`, `ALYANTE_DSN`, `MISTRA_DSN`, `ARAK_*`) siano documentati dove necessario

5. `deploy/Dockerfile`
   - aggiungere copy esplicita dist.

### Frontend/workspace

1. `apps/ordini/package.json` con `"name": "mrsmith-ordini"` (allineato a `mrsmith-quotes`, `mrsmith-rda`, `mrsmith-training`).
2. `apps/ordini/vite.config.ts`.
3. `packages/auth-client/src/roles.ts`: aggiungere `ordini: ['app_ordini_access']` in `APP_ACCESS_ROLES`. **Non** includere `app_customer_relations`: l'app gate frontend deve restare solo sull'access role, altrimenti un futuro utente solo-CR senza access role passerebbe il gate e vedrebbe un'app vuota perché ogni read base richiede `app_ordini_access`. CR resta enforced dai singoli action handler backend.
4. Root `package.json` script `dev:ordini`.
5. `Makefile` target `dev-ordini`.

### Impatto collaterale di `customerRelationsRoles` in `AllRoles()`

Verificato che `AllRoles()` ha un solo caller in tutto il backend: `backend/internal/auth/middleware.go:87`, dentro `resolveNoopRoles()`, attivo solo in dev (NoopMiddleware).

Effetto osservabile: in `make dev` il fake user `john.doe@acme.com` ottiene automaticamente `app_customer_relations` e quindi esercita anche le UI/endpoint elevati senza setup extra. È il comportamento già usato per ruoli analoghi come `app_training_people_admin`, `app_rdf_manager`, `app_cpbackoffice_biometric_access`.

Nessun impatto in prod (`AllRoles()` non viene chiamato lì; il ruolo `app_customer_relations` va comunque creato lato Keycloak per gli ambienti reali, come per qualunque nuovo `app_*` ruolo). `catalog_test.go` continua a passare perché itera `definition.AccessRoles`, che NON contiene CR.

---

## 17. Observability/error contract

### Backend logging

Ogni failure path include:

```text
component=ordini
operation=<operation>
request_id=<id>
order_id=<id>
row_id=<id, optional>
gw_path=<path, optional>
upstream_status=<status, optional>
duration_ms=<duration>
```

### Client error codes sanitizzati

Usare codici stabili, non errori SQL/GW raw:

```text
invalid_order_id
order_not_found
row_not_found
forbidden
role_insufficient
wrong_state
precondition_missing
missing_confirmation_date
missing_customer
missing_pdf
invalid_pdf
gateway_not_configured
gateway_error
arxivar_upload_failed
db_failed
db_commit_failed
gw_pdf_malformed
alyante_database_not_configured
vodka_database_not_configured
```

Naming rule: usare `gateway_not_configured` per il client-facing error quando `arak.Client` non è configurato. Non esporre `arak_*` nei codici frontend perché Arak è un dettaglio implementativo.

### Frontend UX errors

- 403: `Operazione non consentita.`
- 409 state: messaggio business specifico.
- GW/PDF: messaggio business, non tecnico.
- Partial send-to-ERP: panel persistente, non solo toast.

---

## 18. Implementation phases

### Phase 0 — Plan pre-gate

- Confermare questo piano.
- UI pre-gate con `portal-miniapp-ui-review`.
- Test contrattuali critici approvati dall'utente per le regole sotto (§19).

### Phase 1 — Platform + frontend skeleton

- Creare `apps/ordini`.
- Vite config port `5192`.
- AppShell + auth gate + routes placeholder.
- Catalog/config/main/Docker/auth-client wiring.
- Catalog entry con status iniziale `test`.
- Verifica: app visibile/lanciabile e build frontend passa.

### Phase 2 — Backend read endpoints

- Creare `backend/internal/ordini`.
- Implementare reads:
  - orders list;
  - order detail;
  - rows;
  - technical rows;
  - customer refs;
  - optional origin.
- Frontend Home list e read-only detail shell.

### Phase 3 — Simple mutations

- PATCH header C2.
- PATCH referents.
- PATCH serial.
- PATCH technical notes.
- UI form/inline edit + invalidation queries.

### Phase 4 — PDF proxies

- Gateway wrapper sopra `arak.Client`.
- Four PDF endpoints.
- Blob download helper.
- Filename/state/role gates.

### Phase 5 — Activation workflow

- `PATCH .../activate`.
- Q2 count fix.
- Auto-ATTIVO.
- Activation modal UI.

### Phase 6 — Send-to-ERP workflow

- Payload builder.
- Per-row loop.
- Outcome panel.
- State transition + Arxivar upload.
- Full success navigate back to Home.

### Phase 7 — Polish + post-gate

- Responsive checks.
- Accessibility/focus/modal.
- Empty/error states.
- UI post-gate.
- Manual verification.
- Catalog status `ready` quando approvato.

---

## 19. Verification plan

### Manual verification obbligatoria

- Launcher tile visibile con ruolo corretto.
- Home list carica.
- Search/sort/pagination client-side.
- Detail direct URL refresh funziona.
- Utente base può leggere, editare note tecniche, editare seriale solo in BOZZA.
- Utente non-CR non vede/usa azioni elevate.
- CR può salvare Info in BOZZA.
- CR può salvare Referenti in BOZZA/INVIATO.
- `INVIA in ERP` full success cambia stato a INVIATO.
- Partial failure mostra outcome per riga e lascia BOZZA.
- Activation ultima riga porta ad ATTIVO.
- PDF scaricati con Bearer e filename corretti.
- `cdlan_cliente_id` scritto/verificato dopo header save.
- Row ownership check non consente mutare righe di altri ordini.

### Test contrattuali approvati

Per il Test Rule del progetto, questi test sono approvati perché proteggono regole business/migration critiche:

- `CheckConfirmRows` con `data_annullamento IS NOT NULL`.
- `sendToErp` partial failure non transiziona Vodka.
- `sendToErp` full success transiziona e tenta Arxivar.
- Arxivar failure dopo state flip ritorna warning.
- C2 dual-write cliente.
- Row ownership check.
- PDF normalization base64/raw.
- Permission gates backend.
- Origin resolver `orders.legacy_orders -> quotes.quote.quote_number`.

### Build/checks

Frontend:

```bash
pnpm --filter mrsmith-ordini build
pnpm --filter mrsmith-ordini lint
```

Backend:

```bash
cd backend && go test ./internal/ordini ./internal/platform/applaunch ./internal/platform/config ./internal/platform/staticspa
```

Full, se runtime consente:

```bash
pnpm -r --if-present build
cd backend && go test ./...
```

---

## 20. Coexistence e cutover

Durante cutover Appsmith/MrSmith possono coesistere sugli stessi dati Vodka.

Verifica coexistence:

1. Aprire Appsmith Ordini e MrSmith Ordini sullo stesso ordine.
2. Fare update in uno.
3. Refresh nell'altro.
4. Verificare dati coerenti.

Cutover suggerito:

1. Ship interno con catalog status `test`.
2. Demo con operatori CustomerRelations.
3. Correggere feedback bloccanti.
4. Flip a `ready`.
5. Mantenere Appsmith disponibile come rollback per un periodo limitato.
6. Ritirare Appsmith dopo verifica operativa.

---

## 21. Deferred finali

Restano in `docs/TODO.md`:

- Cancel-order re-enablement:
  - decidere `order_number`;
  - decidere `customer_name`;
  - decidere policy `from_cp`.
- Retry partial-failure ERP.
- Audit/migrazione `cdlan_int_fatturazione = 5`.
- Server-side pagination solo se il volume cresce.

---

## 22. Definition of Done

- `apps/ordini` builda.
- `backend/internal/ordini` espone tutti gli endpoint v1.
- API contract usa `/api/ordini/v1/...`.
- GW usa `arak.Client` + `ARAK_*`.
- Nessun `GW_INT_*` introdotto.
- Portal launcher aggiornato.
- Docker copia `/static/apps/ordini`.
- Deep-link `/apps/ordini/ordini/:id` funziona.
- UI pre-gate/post-gate completati.
- Manual verification completata.
- Test contrattuali approvati passano.
- Post-v1 TODO restano documentati.
