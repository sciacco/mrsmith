# Energia in DC — Application Specification

## Summary
- **Application name:** Energia in DC
- **Portal category:** Smart Apps
- **Audit source:** `apps/zammu/ZAMMU-AUDIT.md` §2.3 (source page: `Energia variabile` inside Zammu Appsmith app)
- **Spec status:** Ready for hand-off to `portal-miniapp-generator`
- **Last updated decisions (2026-04-17):**
  - Keep the existing `kW = SUM(ampere) * 225 / 1000` formula as an accepted approximation (do not introduce per-phase computation in this rewrite).
  - Drop the unreachable "Settimanale" period branch (dead code + cosfi scale bug).
  - Replace the display-name-keyed lookup in the "Racks no variable" flow with an ID-keyed endpoint.

## Current-State Evidence
- **Source pages/views:** One Appsmith page with 5 tabs: "Situazione per rack", "Consumi in kW", "Addebiti", "Racks no variable", "Consumi < 1A".
- **Source entities and operations:** Customer, Site, Room, Rack, RackSocket, PowerReading, DailySummary (kW), BillingCharge; plus derived views "no-variable-billing customers" and "low-consumption sockets".
- **Source integrations and datasources:** `grappa` MySQL only. Tables: `cli_fatturazione`, `racks`, `rack_sockets`, `rack_power_readings`, `rack_power_daily_summary`, `datacenter`, `dc_build`, `importi_corrente_colocation`.
- **Known audit gaps or ambiguities:**
  - Exact SQL for `get_customers` "active customers with rack sockets" criterion not visible.
  - Exact columns of `anagrafiche_no_variable` view not detailed.
  - The `maxampere / 2` factor in the socket gauge could be a safety-margin policy or a bug — not confirmed.
  - DB time zone for timestamp fields not audit-visible.

## Entity Catalog

### Entity: Customer
- **Purpose:** Billing customer for colocation services.
- **Operations:**
  - `listActive()` — customers with at least one rack socket.
  - `listWithoutVariableBilling()` — customers whose racks all have `variable_billing = false`, excluding the company self-row (`id_anagrafica = 3`).
- **Fields and inferred types:** `id` (int — aliased `id_anagrafica` in joins), `intestazione` (string), `codice_aggancio_gest` (string — Alyante ERP id per `docs/IMPLEMENTATION-KNOWLEDGE.md`; cross-DB mapping constant).
- **Relationships:** 1 → N Rack; 1 → N BillingCharge; 1 → N DailySummary.
- **Constraints and business rules:**
  - `id_anagrafica <> 3` self-exclusion is to be surfaced as a backend config flag, not hardcoded.
- **Open questions:** Confirm precise "active with rack sockets" predicate before porting.

### Entity: Site (Building)
- **Purpose:** Physical building hosting datacenter rooms for a customer.
- **Operations:** `listByCustomer(customer_id)`.
- **Fields:** `dc_build_id` (int PK), `name` (string).
- **Relationships:** 1 → N Room.
- **Constraints and business rules:**
  - Original SQL uses `LIKE` on numeric FK — replace with `=` in the rewrite.
  - All queries parameterized.

### Entity: Room (Datacenter)
- **Purpose:** A datacenter room within a building for a specific customer.
- **Operations:** `listBySiteAndCustomer(site_id, customer_id)`.
- **Fields:** `id_datacenter` (int PK), `room_name` (string), `dc_build_id` (int FK).
- **Relationships:** N ← 1 Site; 1 → N Rack.

### Entity: Rack
- **Purpose:** Colocation rack.
- **Operations:**
  - `listByRoomAndCustomer(room_id, customer_id)`.
  - `get(rack_id)` — metadata.
  - `listWithoutVariableBillingByCustomer(customer_id)` — ID-keyed (rewrite replaces the original `intestazione`-keyed query).
- **Fields:** `id_rack` (int PK), `name` (string), `id_datacenter` (int FK), `id_anagrafica` (int FK to Customer), `stato`, `variable_billing` (bool), `floor`, `island`, `type`, `pos`, `codice_ordine`, `serialnumber`, `committed_power`, `billing_start_date`.
- **Relationships:** N ← 1 Room; N ← 1 Customer; 1 → N RackSocket.

### Entity: RackSocket
- **Purpose:** Individual power socket on a rack, monitored via SNMP.
- **Operations:**
  - `statusByRack(rack_id)` — returns per-socket avg ampere (last 2 days) + derived `maxampere`.
  - `lowConsumption(min_ampere, customer_id?)` — sockets absorbing ≤ threshold, joined with rack/room/building.
- **Fields:** `id` (int PK), `rack_id` (int FK), `magnetotermico` (string, e.g. `trifase 32A` / `monofase 16A`), `snmp_monitoring_device`, `detector_ip`, `posizione`, `posizione2`, `posizione3`, `posizione4`.
- **Constraints and business rules:**
  - Breaker capacity derivation: `trifase 32A` → 63, `monofase 16A` → 16, else → 32. Keep this mapping in the backend; consider a lookup table so new breaker types can be added without code change.
  - Gauge formula `ampere / (maxampere / 2) * 100` ported as-is (see open question Q2).

### Entity: PowerReading
- **Purpose:** Raw per-socket power reading timeseries.
- **Operations:** `list(rack_id, from, to, page, pageSize)` — returns `{items, total, page, pageSize}`.
- **Fields:** `id`, `oid`, `rack_socket_id` (FK), `date` (timestamp), `ampere` (numeric).
- **Constraints:** Server-side pagination required. All inputs parameterized. `count_power_reading` merged into the list endpoint response.

### Entity: DailySummary (kW)
- **Purpose:** Per-day aggregated kW per customer.
- **Operations:** `kwByCustomer(customer_id, period = day|month, cosfi)`.
- **Fields:** `id`, `giorno` (date), `kilowatt` (numeric), `id_anagrafica` (FK).
- **Constraints and business rules:**
  - `cosfi` is an integer percent value (range 70–100); the SQL must apply `cosfi / 100` as a multiplier.
  - Only `day` and `month` periods are supported (weekly dropped).

### Entity: BillingCharge (Addebito)
- **Purpose:** Billing line item for variable-power charges.
- **Operations:** `listByCustomer(customer_id)`.
- **Fields:** `id`, `customer_id` (FK), `start_period` (date), `end_period` (date), `ampere`, `eccedenti`, `amount` (EUR), `pun`, `coefficiente`, `fisso_cu`, `importo_eccedenti`.

## View Specifications

### View 1: "Situazione per rack"
- **User intent:** Inspect a single rack's live power status and a window of historical readings.
- **Interaction pattern:** Cascading filter (Customer → Site → Room → Rack) + date range → composite detail view.
- **Main data shown:** Rack metadata; per-socket gauges (with red >90%); paginated power readings table; dual-axis ampere/kW trend chart.
- **Key actions:** Cascade filters; "Aggiorna"; paginate readings.
- **Entry/exit:** Top-level app view.
- **Current vs intended:** Current fires some data fetches as fire-and-forget; rewrite uses per-hook loading states so widgets cannot render stale.

### View 2: "Consumi in kW"
- **User intent:** Chart a customer's kW over time at a given cos φ.
- **Interaction pattern:** Parameterized analytic chart.
- **Main data shown:** Bar chart (log-2 y-axis) of kW per day or month, titled with customer + cos φ.
- **Key actions:** Select customer, period (day/month), cos φ (70–100 slider, default 95), "Aggiorna".
- **Current vs intended:** Remove the unreachable "Settimanale" option and its buggy branch.

### View 3: "Addebiti"
- **User intent:** View billing records for a customer.
- **Interaction pattern:** Filter-select → table.
- **Main data shown:** Billing rows with period, ampere, eccedenti, amount, PUN, coefficiente, fisso CU, importo eccedenti.
- **Key actions:** Select customer.
- **Current vs intended:** No change in capability; rewrite as a proper table component.

### View 4: "Racks no variable"
- **User intent:** Audit customers and racks not on variable billing.
- **Interaction pattern:** Master-detail table.
- **Main data shown:** Master list of customers without variable billing; on row click, the detail table shows that customer's non-variable racks.
- **Key actions:** Select customer row.
- **Current vs intended:** Detail query is ID-keyed (customer_id) instead of name-keyed in the rewrite.

### View 5: "Consumi < 1A" (Low-consumption sockets)
- **User intent:** Find sockets absorbing below a threshold.
- **Interaction pattern:** Form filter → results table.
- **Main data shown:** Rows with customer, building, room, socket name, ampere, power meter, magnetotermico, posizioni.
- **Key actions:** Set threshold (default 1A), optional customer, "Cerca".

## Logic Allocation

### Backend responsibilities
- All SQL against `grappa`, parameterized. Replace every `LIKE` on numeric FKs with `=`.
- Apply the `id_anagrafica <> 3` self-exclusion via config, not hardcoded in SQL.
- Compute `maxampere` from `magnetotermico` (backend-owned mapping — lookup table preferred over inline CASE).
- Compute live kW in the ampere/kW trend query using the accepted 225V formula.
- Enforce Keycloak access role `app_energiadc_access` on every route.
- Merge `count_power_reading` into the `list power readings` endpoint response.

### Frontend responsibilities
- Cascading-select UX (shared pattern with Coperture).
- Chart rendering (ECharts): dual-axis line (ampere/kW) and log-2 bar chart.
- Per-view loading and error states.
- Progress gauge with red >90% threshold (keep the `maxampere/2` factor).

### Shared validation or formatting
- Types for all entities in a shared module (likely this app's own package).
- Cos φ integer-to-ratio conversion spec documented in the API contract.

### Rules being revised rather than ported
- No weekly kW endpoint (dropped).
- `racks_no_variable` is ID-keyed, not name-keyed.
- Self-exclusion becomes config-driven.
- SQL injection fixes: parameterize previously-unprepared queries.
- 225V formula **not revised** — accepted as approximation for this release.

## Integrations and Data Flow

### External systems and purpose
- `grappa` MySQL — sole read store.

### End-to-end user journeys
See the view specs above; each view is a single-screen workflow. No cross-view navigation.

### Background or triggered processes
- None.

### Data ownership boundaries
- Read-only from `grappa`. No writes.
- Upstream provisioning and billing systems populate the tables; this app does not write back.

## API Contract Summary

### Required capabilities
- Cascading customer/site/room/rack lookups.
- Rack metadata, socket status, paginated power readings, 2-day ampere/kW stats.
- Customer kW summary by period + cos φ.
- Customer billing charges list.
- No-variable-billing audit: customers + racks (ID-keyed).
- Low-consumption sockets search.

### Read endpoints or queries (proposed shape)
- `GET /api/energia-dc/customers` — active customers.
- `GET /api/energia-dc/customers/{id}/sites`
- `GET /api/energia-dc/sites/{id}/rooms?customerId=`
- `GET /api/energia-dc/rooms/{id}/racks?customerId=`
- `GET /api/energia-dc/racks/{id}`
- `GET /api/energia-dc/racks/{id}/socket-status`
- `GET /api/energia-dc/racks/{id}/power-readings?from=&to=&page=&size=` — returns `{items, total, page, size}`.
- `GET /api/energia-dc/racks/{id}/stats-last-days`
- `GET /api/energia-dc/customers/{id}/kw?period=day|month&cosfi=`
- `GET /api/energia-dc/customers/{id}/addebiti`
- `GET /api/energia-dc/no-variable-billing/customers`
- `GET /api/energia-dc/no-variable-billing/customers/{id}/racks`
- `GET /api/energia-dc/low-consumption?min=&customerId=`

### Write commands or mutations
- None in v1.

### Derived or workflow-specific operations
- Server-side pagination on power readings (unify count + page fetch).

## Constraints and Non-Functional Requirements

### Security or compliance
- Keycloak OIDC; require role `app_energiadc_access`.
- All queries parameterized (closes audit-noted SQL-injection findings #1, #2).

### Performance or scale
- `rack_power_readings` is large; paginate server-side. Avoid `SELECT *` in the list endpoint.
- Hourly aggregate endpoint (`stats-last-days`) returns ~48 rows — small.

### Operational constraints
- Backend must connect to `grappa` MySQL.

### UX or accessibility expectations
- Follow portal Matrix/Stripe-level design per `docs/UI-UX.md`.
- 5 views rendered as top-level tabs or sub-routes of the app.

## Open Questions and Deferred Decisions

- **Q1.** Precise predicate for "active customers with rack sockets" in `listActive()`.
  - *Needed input:* review current SQL or ask billing/ops.
  - *Decision owner:* Backend implementer + domain expert.
- **Q2.** Is the gauge denominator `maxampere / 2` an intentional safety-margin policy or a porting error?
  - *Needed input:* domain confirmation.
  - *Decision owner:* Domain expert.
- **Q3.** Should "Addebiti" table support CSV/PDF export in this release?
  - *Decision owner:* Product.
- **Q4.** Any bulk actions on low-consumption search results (ticketing, notify)?
  - *Decision owner:* Product.
- **Q5.** kW endpoint shape — single endpoint with `period=` or two endpoints?
  - *Decision owner:* Backend implementer (cosmetic).
- **Q6.** Time zone convention for timestamps (`date`, `giorno`, `start_period`).
  - *Decision owner:* Backend implementer.

## Acceptance Notes

- **What the audit proved directly:** Query SQL, widget configuration, JSObject methods, embedded rules (self-exclusion, breaker mapping, gauge formula, 225V coefficient), bugs (cosfi scale in weekly branch, fire-and-forget loads, SQL-injection-risk unprepared queries).
- **What the expert confirmed (2026-04-17):** 225V formula stays; weekly period dropped; no-variable-billing detail query switches to ID-keyed.
- **What still needs validation:** Active-customer predicate (Q1), `maxampere/2` intent (Q2), export & bulk-action UX (Q3, Q4), time zone (Q6).
