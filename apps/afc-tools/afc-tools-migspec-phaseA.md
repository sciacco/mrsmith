# AFC Tools — Migration Spec Phase A: Entity-Operation Model

**Directive for this spec**: 1:1 porting. Every current behavior — including oddities, hard-coded floors, filters, and mapping bugs — is preserved verbatim. Fixes and redesigns are out of scope; they are logged as deferred items for a later phase.

**Audit source**: `apps/afc-tools/audit.md`

---

## A.1 Extracted entities

Nine candidate entities surface across the 8 pages. Fields are aggregated from query columns, widget bindings, and SQL projections.

### E1. `WhmcsTransaction` (payment/refund)
- **Source**: `whmcs_prom` MySQL, via `getTransactions`.
- **Operations**:
  - `list(from, to)` — returns transactions in a date range.
  - `exportXlsx(from, to)` — same dataset, delivered as XLSX via carbone.io template.
- **Fields (inferred from `tbl_transactions` columns)**:
  - `cliente` (string, customer label)
  - `fattura` (string, invoice number)
  - `invoiceid` (int)
  - `userid` (int)
  - `payment_method` (string)
  - `date` (int, `YYYYMMDD`)
  - `description` (string)
  - `amountin` (decimal)
  - `fees` (decimal)
  - `amountout` (decimal)
  - `rate` (decimal)
  - `transid` (string)
  - `refundid` (int)
  - `accountsid` (int)
- **Constraints / business rules (preserve verbatim)**:
  - Hard floor: `date > 20230120`.
  - Filter: `(fattura <> '' AND invoiceid > 0) OR refundid > 0`.
  - Integer-date comparison against `selectedDate` string (works because `date` is `YYYYMMDD`).
- **Open questions**: none for 1:1 port (all behavior is preserved).

### E2. `WhmcsInvoiceLine` (feed WHMCS → Alyante)
- **Source**: `whmcs_prom` MySQL, via `righealiante`.
- **Operations**: `listLatest()` — last 2000 rows by `id DESC`.
- **Fields**: 31 columns from `tbl_fatture_prometeus` (exact names need to come from the query projection — audit lists count but not names; see A.4 open item).
- **Constraints (preserve)**: `LIMIT 2000`, `ORDER BY id DESC`, no user filter.
- **Open questions**:
  - Q-A1: The exact 31 columns are not enumerated in the audit. We need the SQL of `righealiante` to lock the field list. *Fallback for 1:1: copy whatever the query returns; match the table column order.*

### E3. `MistraProductMissingInAlyante`
- **Source**: `mistra` PostgreSQL, via `articoli_non_in_alyante`. Joins `loader.erp_anagrafica_articoli_vendita`, `products.product`, `common.translation`.
- **Operations**: `list()` — no inputs.
- **Fields (from `tbl_lista_articoli`)**:
  - `code` (string, product code)
  - `categoria` (string)
  - `descrizione_it` (string, IT translation)
  - `descrizione_en` (string, EN translation)
  - `nrc` (decimal, non-recurring charge)
  - `mrc` (decimal, monthly recurring charge)
- **Constraints (preserve)**:
  - `erp_sync = true`.
  - Anti-join: `RIGHT JOIN erp_anagrafica_articoli_vendita ON … WHERE a.cod_articolo IS NULL`.
  - `MAX(CASE WHEN language='it' THEN description END)` pivot for IT/EN.
- **Open questions**: none for 1:1.

### E4. `RemoteHandsTicket` (download-only)
- **Source**: Internal API gateway `gw-int.cdlan.net`, `GET /tickets/v1/pdf/{ticketId}?ticket_type=RemoteHands&lang={it|en}`.
- **Operations**: `downloadPdf(ticketId, lang)`.
- **Fields**: none retained in UI state — payload is a PDF blob.
- **Constraints (preserve)**:
  - `ticket_type=RemoteHands` is hard-pinned.
  - `lang ∈ {it, en}` (static select options).
  - Client-side base64 vs raw-bytes detection (starts with `%PDF` or regex match) — the 1:1 port replaces this with a native `Blob` fetch; behavior-equivalent, not a feature change.
- **Open questions**: none for 1:1.

### E5. `XConnectOrder` (EVASO list + PDF)
- **Source**:
  - `All_orders_xcon` on `mistra` PG — joins `loader.hubs_deal`, `loader.cp_ordini`, `orders.order`, `customers.customer`, `orders.order_state`.
  - `DownloadOrderPDF` on gateway — `GET /orders/v1/order/pdf/{orderId}`.
- **Operations**:
  - `listEvaso()` — no inputs.
  - `downloadPdf(orderId)`.
- **Fields (from `Table1` columns)**:
  - `id_ordine` (int/string, PK)
  - `codice_ordine` (string)
  - `cliente` (string)
  - `data_creazione` (date/datetime)
- **Constraints (preserve)**:
  - Filter: `kit_category='XCONNECT' AND state='EVASO'`.
  - 404 on PDF → show "Il PDF non è ancora pronto."
- **Open questions**: none for 1:1.

### E6. `EnergiaColoConsumption`
- **Source**: `grappa` MySQL, via `Q_select_consumi_colo_filter` (monthly pivot) and `Q_select_consumi_colo` (detail rows). Tables: `importi_corrente_colocation` (alias `i`), `cli_fatturazione`.
- **Operations**:
  - `listMonthlyPivot(year)` — 12-column (`Gennaio…Dicembre`) sum by customer.
  - `listDetail(year)` — raw per-period rows.
- **Fields — pivot (inferred)**:
  - customer identifier(s) (the exact grouping columns need the SQL — see Q-A2)
  - `Gennaio`, `Febbraio`, …, `Dicembre` (decimal sums)
- **Fields — detail (from `TBL_ConsumiColoDetail`)**:
  - `tipo_variabile` (string, semantics unknown)
  - `ampere` (decimal)
  - `Kw` (decimal)
  - `coefficiente` (decimal, semantics unknown)
  - `fisso_cu` (decimal, semantics unknown)
  - `eccedenti` (decimal, semantics unknown)
  - `importo_eccedenti` (decimal, semantics unknown)
  - `start_period`, `end_period` (date)
  - customer link via `cli_fatturazione`
- **Constraints**:
  - Consumption metric: `IF(i.ampere > 0, i.ampere, i.Kw)` — preserve.
  - Year interpolated as string into `year('{{TXT_anno.text}}')` — preserve (but see deviation below).
  - **Deviation from 1:1 (expert-approved, decision A.5.1d)**: the anno input defaults to the current year (`new Date().getFullYear()`) instead of being empty. On-load the two queries therefore return current-year data instead of empty tables. All other behavior (Cerca button, pivot shape, detail fields) unchanged.
- **Open questions**:
  - Q-A2: audit does not quote the full SQL of `Q_select_consumi_colo_filter` / `Q_select_consumi_colo`, so the grouping columns (customer? ragione sociale? codice_aggancio?) and the detail column list are not locked. We need to read the queries from `AFC-Tools.json.gz` to freeze the field list.
  - Q-A3 (semantic, deferred): `tipo_variabile`, `coefficiente`, `fisso_cu`, `eccedenti`, `importo_eccedenti` meanings — not required for a 1:1 port (columns are displayed verbatim) but flagged for later.

### E7. `SalesOrderSummary` (list view)
- **Source**: Vodka/daiquiri MySQL, via `Select_Orders_Table`.
- **Operations**: `listActive()` — no inputs.
- **Fields (from `Lista_ordini`)**:
  - `id` (int, PK used for drill-down)
  - customer / cliente
  - `cdlan_stato` (string, `ATTIVO` or `INVIATO`)
  - `cdlan_tipo_ord` (char, A/N/R → Sostituzione/Nuovo/Rinnovo) — displayed as label
  - service_type (string) and/or `is_colo` (string) — coalesced via `IF(is_colo != 0, is_colo, service_type)` → "Tipo di servizi"
  - `from_cp` (int → "Sì"/"No")
  - *(further columns need confirmation from the live SQL — see Q-A4)*
- **Constraints (preserve)**:
  - `cdlan_stato IN ('ATTIVO','INVIATO')`.
  - `IF(is_colo != 0, is_colo, service_type)` coalesce.
  - `CASE cdlan_tipo_ord WHEN 'A' THEN 'Sostituzione' WHEN 'N' THEN 'Nuovo' WHEN 'R' THEN 'Rinnovo' END`.
  - `IF(from_cp != 0, 'Sì', 'No')`.
- **Open questions**:
  - Q-A4: audit summarizes the mappings but does not enumerate every column on `Select_Orders_Table`. Need to read the SQL for the exact column list.

### E8. `SalesOrder` (header + rows detail)
- **Source**: Vodka/daiquiri MySQL, via `Order` (header) and `RigheOrdine` (rows). Keyed by URL `?id=`.
- **Operations**:
  - `get(id)` — header.
  - `listRows(id)` — order rows.
- **Fields — header (from the 20+ text widgets)**:
  - `cdlan_tipodoc` (string → "Ordine ricorrente" if `TSC-ORDINE-RIC`, else "Ordine Spot")
  - `cdlan_tipo_ord` (A/N/R mapping)
  - `cdlan_dur_rin` (int, 1/2/3/4/6/12 → Mensile/Bimestrale/Trimestrale/Quadrimestrale/Semestrale/Annuale)
  - `cdlan_int_fatturazione` (int, 1/2/3/5/6/else → same labels; Quadrimestrale uses code **5** here — divergence with `cdlan_dur_rin`)
  - `cdlan_int_fatturazione_att` (int, 1 → "All'ordine", else → "All'attivazione della Soluzione/Consegna")
  - `cdlan_cod_termini_pag` (int, extensive mapping 301/303/304/311–318/400–409; **bug preserved**: code 400 unreachable due to typo in ternary)
  - `cdlan_tacito_rin` (int, 1 → "Sì" else "No")
  - `cdlan_note` (string, null/empty → "Nessun valore")
  - `data_decorrenza` (date/string, null/empty → "Nessun valore")
  - data_creazione, customer fields, plus remaining header fields (exact full list requires the `Order` SQL — see Q-A5)
- **Fields — rows (from `TBL_OrderRows`)**:
  - order line fields incl. `cdlan_codice_kit`, `index_kit` (composed via `IF(cdlan_codice_kit != '', CONCAT(cdlan_codice_kit, '-', index_kit), '')` as the displayed kit code)
  - *(full column list requires `RigheOrdine` SQL — see Q-A5)*
- **Constraints**:
  - All mappings exactly as in the audit §2.7 "Hidden logic" block.
  - **Bug fix applied (expert-approved, decision A.5.1a)**: `cdlan_cod_termini_pag == 400` typo corrected → value 400 now maps to label "SDD FM".
  - **Discrepancy preserved (decision A.5.1b)**: Quadrimestrale = code 4 on `cdlan_dur_rin`, code 5 on `cdlan_int_fatturazione`. TODO logged in `docs/TODO.md` under "AFC Tools App".
  - **Null-check behavior fixed (decision A.5.1c)**: both `null` and `''` on `cdlan_note` / `data_decorrenza` render "Nessun valore".
  - HTML `<b>…</b>` in bindings: replaced by equivalent bold typography in the new UI (behavior-equivalent presentation, not a design change).
- **Open questions**:
  - Q-A5: need the exact SQL of `Order` and `RigheOrdine` to lock header/rows field lists.

### E9. `DdtVerificaCespiti`
- **Source**: Alyante MSSQL view `Tsmi_DDT_Verifica_Cespiti`, via `ListaDdtVerificaCespiti` (`SELECT *`).
- **Operations**: `list()` — no inputs, no filter, no limit.
- **Fields**: 12 columns incl. `Seriali`, `Importo_unitario`, … *(full list requires the view projection — see Q-A6)*.
- **Constraints (preserve)**: no pagination, no filter. **Explicitly preserve the full-table load** per the 1:1 directive, despite performance risk flagged in the audit (decision A.5.1e). TODO logged in `docs/TODO.md` under "AFC Tools App".
- **Open questions**:
  - Q-A6: lock the 12 column names from the view.

---

## A.2 Auxiliary non-entity concerns (for completeness)

- **Carbone.io render** is not a domain entity — it is an outbound integration for XLSX production. Treated in Phase D.
- **`Dettaglio_ordine` modal** on Ordini Sales is declared but never opened. 1:1 directive says "port what exists"; since nothing opens it, the modal is out of scope (it produces no observable behavior). *Confirmed defer.*

---

## A.3 Entity relationships

- `WhmcsTransaction` ↔ `WhmcsInvoiceLine` — share invoice identifiers (`invoiceid` / `fattura`) but the two pages do not cross-link in the UI; relation is logical only.
- `MistraProductMissingInAlyante` references the Alyante ERP product master implicitly (the entity is defined by *absence* there).
- `XConnectOrder` shares the generic `Order` concept with `SalesOrder` but they come from different DBs (Mistra PG vs Vodka MySQL) and have different schemas — treat as independent entities.
- `SalesOrderSummary` → `SalesOrder` — drill-down by `id` via `?id=` querystring.
- `EnergiaColoConsumption` references customers via `cli_fatturazione` (grappa).
- `RemoteHandsTicket`: no structural relation to other entities in the AFC Tools scope.
- `DdtVerificaCespiti`: self-contained read-only view.

No cross-entity aggregates, no writes, no cascades. This is a pure read app.

---

## A.4 Gaps in the audit that block Phase B

The audit is thorough on *behavior* but light on *exact SQL projections*. For a true 1:1 port we need to freeze column names. These can be resolved by reading `AFC-Tools.json.gz` directly (I can unpack it in Phase B if you confirm):

| # | Gap | Needed for |
|---|---|---|
| Q-A1 | `righealiante` full 31-column list | Fatture Prometeus table schema |
| Q-A2 | `Q_select_consumi_colo_filter` + `Q_select_consumi_colo` full SQL | Energia Colo pivot grouping + detail fields |
| Q-A4 | `Select_Orders_Table` column list | Ordini Sales table |
| Q-A5 | `Order` + `RigheOrdine` full SQL | Dettaglio ordini header + rows |
| Q-A6 | `Tsmi_DDT_Verifica_Cespiti` view 12 columns | DDT cespiti table |

None of these are *business questions* — they are column-list lookups. For a 1:1 port the answer is "whatever the current query returns".

---

## A.5 Decisions (confirmed by expert)

| # | Topic | Decision |
|---|---|---|
| 1a | `cdlan_cod_termini_pag == 400` typo (Dettaglio ordini) | **FIX** — correct the ternary to reference `cdlan_cod_termini_pag` so value 400 → "SDD FM" renders. |
| 1b | Quadrimestrale code divergence (4 on `cdlan_dur_rin`, 5 on `cdlan_int_fatturazione`) | **PRESERVE** — port as-is, add TODO in `docs/TODO.md`. |
| 1c | `== ''` null-checks on `cdlan_note` / `data_decorrenza` (Dettaglio ordini) | **FIX** — treat `null` as equivalent to empty string, i.e. both `null` and `''` render "Nessun valore". |
| 1d | Energia Colo empty-year on-load → empty tables | **FIX** — default `TXT_anno` to the current year so both tables show current-year data on page open. |
| 1e | DDT cespiti full-table `SELECT *` on every load | **PRESERVE** — port as-is, add TODO in `docs/TODO.md`. |
| 2 | Dead UI: `Dettaglio_ordine` modal on Ordini Sales | **DROP** — not ported. |
| 3 | Orphan queries / JSObjects (`transazioni_whmcs`, duplicate actions, `myFun1/2`, `JSObject1`) | **DROP** — not ported. |
| 4 | Carbone.io XLSX export | **4a** — backend proxies carbone.io, returns the `render.carbone.io/render/{renderId}` URL, frontend opens it in a new tab. `templateId` moves to backend config. |
| 5 | Keycloak role | `app_afctools_access`. |

---

## A.6 Phase A done-check

- [x] All 9 audit entities enumerated with operations and fields.
- [x] Business rules listed per entity, marked "preserve verbatim".
- [x] Gaps that are *column-list lookups* separated from gaps that are *business decisions*.
- [x] Minimum expert-decision set surfaced (§A.5).
- [ ] Answers to §A.5 questions 1–5 → unblocks Phase B.
- [ ] Answers to §A.4 SQL lookups (or authorization to unpack `AFC-Tools.json.gz`) → unblocks Phase B field-freeze.
