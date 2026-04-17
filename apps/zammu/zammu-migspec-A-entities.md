# Zammu migspec — Phase A: entity-operation model

> Extracted from `apps/zammu/ZAMMU-AUDIT.md`. Sections per target. The final per-app specs will reorganize this, but the extractions here are authoritative.

---

## coperture

### Entity: State (Provincia)
- **Operations (inferred):** `list()` — no filtering params observed.
- **Fields (inferred from audit / standard Italian admin geography):**
  - id (int) — used as `network_coverage_state_id` FK on cities
  - name (string) — province name
  - *Note:* Audit shows PostgreSQL stored function `coperture.get_states()` returns nested JSON consumed as `data[0].get_states`. Field list must be confirmed by reading the function body or a sample row. ⚠ **unknown exact shape.**
- **Relationships:** 1 → N with City via `network_coverage_cities.network_coverage_state_id`.
- **Constraints:** None visible.
- **Open questions:**
  - Confirm exact columns returned by `coperture.get_states()` (JSON shape).

### Entity: City (Comune)
- **Operations:** `listByState(state_id)`.
- **Fields:** `id`, `name`, `network_coverage_state_id` (FK).
- **Relationships:** N ← 1 State; 1 → N Address.
- **Constraints:** None visible; ordered by `name`.

### Entity: Address (Indirizzo)
- **Operations:** `listByCity(city_id)`.
- **Fields:** `id`, `name`.
- **Relationships:** N ← 1 City (via `network_coverage_addresses.network_coverage_city_id`); 1 → N HouseNumber.

### Entity: HouseNumber (Numero civico)
- **Operations:** `listByAddress(address_id)`.
- **Fields:** `id`, `name`.
- **Relationships:** N ← 1 Address (via `network_coverage_house_numbers.network_coverage_address_id`); 1 → N CoverageResult.

### Entity: CoverageResult
- **Operations:** `listByHouseNumber(house_number_id)` → `SELECT * FROM coperture.v_get_coverage WHERE house_number_id = ? ORDER BY operator, tech`.
- **Fields (from view):**
  - `coverage_id` (list key)
  - `operator` (name string, used for display)
  - `operator_id` (int, maps to Operator entity)
  - `tech` (technology name, e.g. FTTH/FTTC — ⚠ exact enum not in audit)
  - `profiles` (nested JSON array — structure ⚠ not fully specified; consumed by `formatProfili` which reads profile names)
  - `details` (nested JSON array — each has at minimum a `type` id + a `value`; `formatDettagli` strips trailing `0000` from value strings)
  - `house_number_id` (FK)
- **Relationships:** N ← 1 HouseNumber; each result references 1 Operator and N CoverageDetailType (via details[].type).
- **Constraints / embedded rules:**
  - Values ending in `0000` are stripped by regex — likely a data-formatting convention (⚠ confirm whether this is compensating for bad data or is a real presentation rule).
- **Open questions:**
  - Exact shape of `profiles[]` and `details[]` inside `v_get_coverage` — audit only shows consumer usage.
  - Is `tech` a free-text string or an enum?

### Entity: Operator
- **Operations:** None via query; the entity is currently a hardcoded 4-row map in `utils.getImageUrl()`.
- **Fields:** `id` (1–4), `name` (TIM, Fastweb, OpenFiber, OpenFiber CD), `logo_url` (CDN URL under `static.cdlan.business`).
- **Relationships:** 1 → N CoverageResult.
- **Constraints:** Audit notes this is hardcoded in the JSObject. Should become backend lookup or config.
- **Open questions:**
  - Is the set of operators stable (5th operator possible)?
  - Should logo URLs move to backend config, DB table, or stay as frontend assets?

### Entity: CoverageDetailType
- **Operations:** `list()` — `SELECT coperture.get_coverage_details_types()` (PG stored function).
- **Fields:** `id`, `name`.
- **Relationships:** 1 → N CoverageResult.details[] (lookup target for `getDetailName(i)`).
- **Open questions:**
  - Could be inlined in the `coverage` endpoint response instead of a separate lookup — design decision for Phase D.

### Aggregate gaps
- The audit treats profiles as name-only (`p.map(e => e.name).join()`). If a profile has more fields (bandwidth, SLA, etc.) they're not surfaced anywhere visible in this app. ⚠ To confirm with expert or via `v_get_coverage` definition.

---

## energia-in-dc

### Entity: Customer
- **Source table:** `cli_fatturazione`.
- **Operations:** `listActive()` (used as `get_customers` — filter for "active customers with rack sockets"), plus `listWithoutVariableBilling()` (via `anagrafiche_no_variable`, excludes `id_anagrafica = 3`).
- **Fields:**
  - `id` (int) — exposed as `id_anagrafica` in joins
  - `intestazione` (string) — business name shown in UI
  - `codice_aggancio_gest` (string) — Alyante ERP ID per `docs/IMPLEMENTATION-KNOWLEDGE.md` (cross-DB mapping constant; not directly used in this page's queries but load-bearing for any integration)
- **Relationships:** 1 → N Rack (`racks.id_anagrafica`); 1 → N BillingCharge (`importi_corrente_colocation.customer_id`).
- **Constraints / embedded rules:**
  - Hardcoded self-exclusion: `id_anagrafica <> 3` (likely the company's own racks). Audit flags as embedded rule.
- **Open questions:**
  - Confirm `id_anagrafica = 3` meaning with expert; should the exclusion be a config flag or a data-level attribute?

### Entity: Site / Building
- **Source table:** `dc_build`.
- **Operations:** `listByCustomer(customer_id)` (`get_sites`).
- **Fields:** `id` (exposed as `dc_build_id`), `name` (site).
- **Relationships:** 1 → N Room (`datacenter.dc_build_id`).
- **Constraints / embedded rules:**
  - Current SQL uses `LIKE` against numeric `id_anagrafica` — audit flags as fragile. Should become `=` in rewrite.
  - Query is unprepared; SQL-injection risk per audit §4.3. Should use parameterized query.

### Entity: Room / Datacenter
- **Source table:** `datacenter`.
- **Operations:** `listBySiteAndCustomer(site_id, customer_id)` (`get_rooms`).
- **Fields:** `id_datacenter`, `room_name`, `dc_build_id`.
- **Relationships:** N ← 1 Site; 1 → N Rack.

### Entity: Rack
- **Source table:** `racks`.
- **Operations:**
  - `listByRoomAndCustomer(room_id, customer_id)` (`get_racks`)
  - `get(rack_id)` (`get_rack_details`)
  - `listWithoutVariableBilling(customer_name)` (`racks_no_variable`)
- **Fields:** `id_rack`, `name`, `id_datacenter` (FK), `id_anagrafica` (FK), `stato`, `variable_billing` (bool), `floor`, `island`, `type`, `pos`, `codice_ordine`, `serialnumber`, `committed_power` ("Committed Ampere"), `billing_start_date`.
- **Relationships:** N ← 1 Room, N ← 1 Customer; 1 → N RackSocket; N ← 1 PowerReading (via RackSocket).
- **Constraints / embedded rules:**
  - `LIKE` on numeric FK (same pattern as Sites) — fragile.
  - `variable_billing` flag drives presence on "Racks no variable" tab when false.

### Entity: RackSocket
- **Source table:** `rack_sockets`.
- **Operations:**
  - `statusByRack(rack_id)` (`get_socket_status`) — returns each socket with avg ampere over last 2 days plus computed `maxampere`.
  - `lowConsumption(min_ampere, customer?)` (`rack_basso_consumo`) — returns sockets absorbing ≤ threshold with joined rack/building/room metadata.
- **Fields:** `id`, `rack_id` (FK), `magnetotermico` (string, e.g. "trifase 32A", "monofase 16A"), `snmp_monitoring_device`, `detector_ip`, `posizione`, `posizione2`, `posizione3`, `posizione4`.
- **Relationships:** N ← 1 Rack; 1 → N PowerReading.
- **Constraints / embedded rules:**
  - `maxampere` is computed in SQL via `CASE magnetotermico WHEN 'trifase 32A' THEN 63 WHEN 'monofase 16A' THEN 16 ELSE 32 END`. Audit flags that unknown breaker types default to 32. **Domain business rule.**
  - Progress bar uses `ampere / (maxampere/2) * 100`, red when >90 — so the gauge maxes out at 50% of breaker capacity. Audit flags as possibly a safety-margin policy; **confirm with expert.**

### Entity: PowerReading
- **Source table:** `rack_power_readings`.
- **Operations:**
  - `list(rack_id, from, to, page, pageSize)` (`get_power_readings`) — server-side paginated.
  - `count(rack_id, from, to)` (`count_power_reading`).
- **Fields:** `id`, `oid`, `rack_socket_id`, `date` (timestamp), `ampere`.
- **Relationships:** N ← 1 RackSocket.
- **Constraints / embedded rules:**
  - Server-side pagination must be preserved (large table).
  - Audit flags SQL injection: `prepared: false` with widget values interpolated. Must parameterize.

### Entity: DailySummary
- **Source table:** `rack_power_daily_summary`.
- **Operations:**
  - `kwByCustomer(customer_id, period=day|month, cosfi)` — `get_kw_days` / `get_kw_months`.
  - Weekly variant dropped (see partition doc).
- **Fields:** `id`, `giorno` (date), `kilowatt`, `id_anagrafica` (FK).
- **Relationships:** N ← 1 Customer.
- **Constraints / embedded rules:**
  - `cosfi` is applied as `value/100` in day/month queries. Input range 70–100 on a number slider.
  - Audit flags hardcoded 225V assumption in the adjacent `stats_last_days` computation (kW = ampere × 225/1000) — does not apply here (this table already stores kW) but is a sibling bug.
- **Open questions:**
  - Should the kW stats endpoint return `day` and `month` on one call, or stay split?

### Entity: BillingCharge (Addebito)
- **Source table:** `importi_corrente_colocation`.
- **Operations:** `listByCustomer(customer_id)` (`get_addebiti_by_cli`).
- **Fields:** `id`, `customer_id`, `start_period`, `end_period`, `ampere`, `eccedenti`, `amount` (EUR), `pun`, `coefficiente`, `fisso_cu`, `importo_eccedenti`.
- **Relationships:** N ← 1 Customer.

### Derived entity: NoVariableBillingCustomer (view)
- **Operations:** `list()` (`anagrafiche_no_variable`).
- **Fields:** inferred `intestazione` + counts of racks (exact columns ⚠ not shown in audit §3 details).
- **Relationships:** Implicit — keyed on `intestazione` string, which is then used to drive `racks_no_variable`. **Fragile:** joining on a display name. Should become an ID-keyed endpoint in the rewrite.
- **Open questions:** Confirm whether a customer id (not intestazione) should drive the rack list in the rewrite.

### Derived entity: LiveStats (aggregated readings, not a table)
- Produced by `stats_last_days` query: hourly ampere/kW for the last 2 days for a given rack.
- **Embedded rule:** `kW = SUM(ampere) * 225 / 1000` — hardcoded single-phase 225V assumption. Contradicts per-socket `magnetotermico = 'trifase 32A'`. Audit flags as bug.
- **Open questions:** Should the rewrite (a) keep 225V for now, (b) switch to a per-socket phase-aware formula, or (c) compute kW from a different upstream table?

### Aggregate gaps
- Customer "active" criterion for `get_customers` ("active customers with rack sockets") — exact SQL not shown. ⚠ Confirm filter.
- Breaker capacity mapping (§4.1 #4) exists only in SQL — consider moving to config.

---

## simulatori-di-vendita

### Entity: PricingTier
- **Source:** Hardcoded in `utils` JSObject of IaaS calcolatrice. No DB.
- **Operations:** `get(tier)` where tier ∈ {Diretta, Indiretta}.
- **Fields:** 10 per-resource daily rates in EUR:
  - `vcpu`, `ram_vmware`, `ram_os`, `storage_pri`, `storage_sec`, `fw_std`, `fw_adv`, `priv_net`, `os_windows`, `ms_sql_std`
  - Rates table reproduced in audit §2.5.
- **Relationships:** 1 → N CostCalculation.
- **Constraints / embedded rules:**
  - All pricing is hardcoded. Changing prices requires code deploy.
- **Open questions:**
  - Move pricing to DB, backend config file, or Keycloak-protected admin UI?
  - Are there more tiers (Partner, Reseller) on the horizon?

### Entity: ResourceQuantity
- **Source:** Form inputs; not persisted.
- **Operations:** Local only.
- **Fields:**
  - `vcpu` (int, ≥1, required), `ram_vmware` (int GB), `ram_os` (int GB), `storage_pri` (int GB, ≥10), `storage_sec` (int GB, ≥0), `fw_std` (⚠ widget type is TEXT — bug), `fw_adv` (0..1), `priv_net` (int), `os_windows` (⚠ widget type is TEXT — bug), `ms_sql_std` (int).
- **Open questions:**
  - Confirm min/max constraints per resource in the rewrite (e.g. does `fw_adv` truly cap at 1?).
  - Confirm the two TEXT inputs become NUMBER in the rewrite.

### Entity: CostCalculation
- **Source:** Computed in-memory by `utils.calcolaTotali()`.
- **Operations:** `compute(quantities, tier)`.
- **Fields (derived):**
  - Per-line: `lineTotal = qty × unitPrice`.
  - Category subtotals: `computing = vcpu + ram_vmware + ram_os`; `storage = storage_pri + storage_sec`; `sicurezza = fw_std + fw_adv + priv_net`; `addon = os_windows + ms_sql_std`.
  - `totale_giornaliero` = sum of all line items.
  - `totale_mensile` = `totale_giornaliero × 30`.
- **Constraints / embedded rules:**
  - Monthly multiplier is hardcoded 30 days. `hours = 730` declared but unused. **Confirm whether monthly should use 30 or 30.4167 (730/24).**
  - `toFixed(2)` applied inconsistently (subtotals stringified, final totale recomputed from raw) — cleanup opportunity.
  - `updatePrezzi()` generates a price-table HTML with an incomplete `decodifica` map — only 5 of 10 resources show. **Bug / gap:** remaining 5 resource display names must be defined for the rewrite.

### Entity: PDFQuote
- **Source:** Produced by Carbone.io REST integration.
- **Operations:** `render(qta, prezzi, totali)` → POST `/render/{templateId}` returning a render ID; client then constructs a download URL (`utils.getURL()`).
- **Fields:**
  - Request payload: `{ convertTo: "pdf", data: { qta, prezzi, totale_giornaliero } }`.
  - Template ID: `7229f811c77569a9ab09c7f71cd923a942e3d5d5aac1d26b98950a19beb2e920` (hardcoded — should move to backend config).
- **Open questions:**
  - Should the backend proxy the render call (audit recommends yes for API key safety)?
  - Should PDFs be stored/archived, or are they one-shot downloads?

### Aggregate gaps
- Currency is implicit EUR — not encoded in the payload.
- No persistence of quotes → no audit trail of what was quoted to whom. Expert to confirm whether the rewrite should introduce this.

---

## Open questions carried into Phase B/C/D (by target)

### coperture
1. Exact columns of `coperture.get_states()` (PG function JSON shape).
2. Exact structure of `profiles[]` and `details[]` inside `v_get_coverage`.
3. Operator master-data location in rewrite (backend config vs DB table).
4. Should `get_details_types` be a separate endpoint or inlined in `coverage` response?
5. Keep the trailing-`0000` strip on detail values?

### energia-in-dc
1. Confirm `get_customers` "active" criterion and whether `id_anagrafica <> 3` should be a config flag.
2. Confirm the `maxampere/2` safety threshold is intentional, not a bug.
3. Decide fate of the 225V hardcoding in `stats_last_days`: keep, fix per-phase, or re-source.
4. Replace "join-by-intestazione" in the no-variable-billing flow with an ID-keyed lookup?
5. Decide kW endpoint shape: one endpoint with `period=day|month` param, or two endpoints.

### simulatori-di-vendita
1. Where does pricing live going forward (code / config / DB / admin UI)?
2. Monthly multiplier: 30 exact days vs average (730h / 24)?
3. Complete the `decodifica` display-name map for all 10 resources.
4. Fix `i_fw_standard` and `i_os_windows` to numeric inputs?
5. Proxy Carbone.io through the backend (security) — confirmed direction.
6. Persist quotes? If yes, what metadata (user, timestamp, customer, tier)?
