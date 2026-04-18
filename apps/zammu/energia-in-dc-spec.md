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
- **Contract clarifications (2026-04-18):**
  - Standardize the repo API namespace to `/api/energia-dc/v1/...`.
  - Keep rack-reading date/time filters in Europe/Rome local time end-to-end. Frontend sends `YYYY-MM-DDTHH:mm` without timezone offset; backend parses the same local values without timezone conversion.
  - For rack readings, both `from` and `to` bounds are inclusive.
- **Open-question resolutions (2026-04-18):**
  - Q1 — Active-customer predicate confirmed (see Entity: Customer).
  - Q2 — `maxampere / 2` is an intentional safety margin; keep as-is.
  - Q3 — "Addebiti" supports CSV export in v1 (PDF deferred).
  - Q4 — Bulk actions on low-consumption results deferred; tracked in `docs/TODO.md`.
  - Q5 — Single endpoint with `period=day|month` param (payloads identical today; revisit if they diverge).
  - Q6 — All timestamps stay in `Europe/Rome` local time end-to-end; no timezone conversion layer is introduced.

## Current-State Evidence
- **Source pages/views:** One Appsmith page with 5 tabs: "Situazione per rack", "Consumi in kW", "Addebiti", "Racks no variable", "Consumi < 1A".
- **Source entities and operations:** Customer, Site, Room, Rack, RackSocket, PowerReading, DailySummary (kW), BillingCharge; plus derived views "no-variable-billing customers" and "low-consumption sockets".
- **Source integrations and datasources:** `grappa` MySQL only. Tables: `cli_fatturazione`, `racks`, `rack_sockets`, `rack_power_readings`, `rack_power_daily_summary`, `datacenter`, `dc_build`, `importi_corrente_colocation`.
- **Known audit gaps or ambiguities:**
  - Exact SQL text for some Appsmith queries (`get_rack_details`, `get_socket_status`, `stats_last_days`, `get_addebiti_by_cli`, `rack_basso_consumo`) is not fully extracted in the audit.
  - The schema for all relevant Grappa tables is already documented under `docs/grappa/`; implementation can proceed from that documentation plus a few narrow regression checks on the drift-prone query behaviors.

## Entity Catalog

### Entity: Customer
- **Purpose:** Billing customer for colocation services.
- **Operations:**
  - `listActive()` — customers having at least one `stato = 'attivo'` rack that has at least one `rack_sockets` row. Confirmed SQL:
    ```sql
    SELECT id, intestazione
    FROM cli_fatturazione
    WHERE id IN (
      SELECT DISTINCT id_anagrafica
      FROM racks
      JOIN grappa.rack_sockets rs ON racks.id_rack = rs.rack_id
      WHERE racks.stato = 'attivo'
    )
    ORDER BY intestazione;
    ```
  - `listWithoutVariableBilling()` — customers whose active racks all have `variable_billing` null or 0, excluding the company self-row (`id_anagrafica = 3`). Confirmed SQL (backs the `anagrafiche_no_variable` view):
    ```sql
    SELECT DISTINCT c.intestazione, r.id_anagrafica
    FROM racks r
    JOIN grappa.datacenter d ON r.id_datacenter = d.id_datacenter
    JOIN cli_fatturazione c ON r.id_anagrafica = c.id
    JOIN grappa.dc_build db ON db.id = d.dc_build_id
    WHERE r.stato = 'attivo'
      AND (variable_billing IS NULL OR variable_billing = 0)
      AND r.id_anagrafica <> 3
    ORDER BY c.intestazione;
    ```
- **Fields and inferred types:** `id` (int — aliased `id_anagrafica` in joins), `intestazione` (string), `codice_aggancio_gest` (string — Alyante ERP id per `docs/IMPLEMENTATION-KNOWLEDGE.md`; cross-DB mapping constant).
- **Relationships:** 1 → N Rack; 1 → N BillingCharge; 1 → N DailySummary.
- **Constraints and business rules:**
  - `id_anagrafica <> 3` self-exclusion is to be surfaced as a backend config flag, not hardcoded.
- **Open questions:** None — "active with rack sockets" predicate confirmed above.

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
  - Gauge formula `ampere / (maxampere / 2) * 100` ported as-is. The `maxampere / 2` denominator is an intentional **safety margin** (confirmed 2026-04-18) — the gauge shows utilization vs. half the breaker rating so that 100% on the UI corresponds to the policy-safe ceiling, not the electrical maximum.

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
- **Current vs intended:** Current fires some data fetches as fire-and-forget; rewrite uses per-hook loading states so widgets cannot render stale. The paginated readings table follows the submitted `from` / `to` range, while the ampere/kW chart remains a fixed "ultimi due giorni" surface.

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
- **Key actions:** Select customer; export current result set to CSV.
- **Current vs intended:** No change in capability other than CSV export (v1 scope); rewrite as a proper table component. PDF export deferred.

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
- Enforce lookup invariants explicitly: `site + customer -> rooms` and `room + customer -> racks` must be verified in SQL, not assumed from frontend cascade state.
- Parse rack-reading `from` / `to` filters as Europe/Rome local datetimes with no timezone conversion.
- Enforce Keycloak access role `app_energiadc_access` on every route.
- Merge `count_power_reading` into the `list power readings` endpoint response.

### Frontend responsibilities
- Cascading-select UX (shared pattern with Coperture).
- Chart rendering for the dual-axis line (ampere/kW) and the log-2 kW bar chart. Library choice is implementation-owned.
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
- `GET /api/energia-dc/v1/customers` — active customers.
- `GET /api/energia-dc/v1/customers/{id}/sites`
- `GET /api/energia-dc/v1/sites/{id}/rooms?customerId=`
- `GET /api/energia-dc/v1/rooms/{id}/racks?customerId=`
- `GET /api/energia-dc/v1/racks/{id}`
- `GET /api/energia-dc/v1/racks/{id}/socket-status`
- `GET /api/energia-dc/v1/racks/{id}/power-readings?from=&to=&page=&size=` — returns `{items, total, page, size}`.
- `GET /api/energia-dc/v1/racks/{id}/stats-last-days`
- `GET /api/energia-dc/v1/customers/{id}/kw?period=day|month&cosfi=`
- `GET /api/energia-dc/v1/customers/{id}/addebiti`
- `GET /api/energia-dc/v1/no-variable-billing/customers`
- `GET /api/energia-dc/v1/no-variable-billing/customers/{id}/racks`
- `GET /api/energia-dc/v1/low-consumption?min=&customerId=`
- Rack-reading datetime filters use local Europe/Rome `YYYY-MM-DDTHH:mm` values without timezone offset. Backend performs no timezone conversion; both bounds are inclusive.

### Write commands or mutations
- None in v1.

### Derived or workflow-specific operations
- Server-side pagination on power readings (unify count + page fetch).

## Lightweight Validation Gate

Use the Appsmith audit plus the documented Grappa schema in `docs/grappa/` as the primary source of truth.

Before final implementation signoff, pin only the behaviors most likely to drift:

- `get_power_readings` + `count_power_reading`: pagination, ordering, merged total count, and local `from` / `to` behavior.
- `racks_no_variable`: rewritten detail route must stay keyed by customer ID, not display name.
- `rack_basso_consumo`: empty-customer behavior must mean "all eligible customers", not "no results".
- Local rack-reading datetime parsing must accept Europe/Rome `YYYY-MM-DDTHH:mm` values without timezone conversion.

Execution rule:

- The app does not need a heavy fixture phase before implementation.
- Use `docs/grappa/*.json` to drive schema-safe handler work, then add the narrow regression checks above before signoff.

## Constraints and Non-Functional Requirements

### Security or compliance
- Keycloak OIDC; require role `app_energiadc_access`.
- All queries parameterized (closes audit-noted SQL-injection findings #1, #2).

### Performance or scale
- `rack_power_readings` is large; paginate server-side. Avoid `SELECT *` in the list endpoint.
- Hourly aggregate endpoint (`stats-last-days`) returns ~48 rows — small.

### Operational constraints
- Backend must connect to `grappa` MySQL.
- All timestamp fields (`rack_power_readings.date`, `rack_power_daily_summary.giorno`, `importi_corrente_colocation.start_period` / `end_period`) are stored and interpreted as **Europe/Rome** local time.
- Rack-reading filters use local Europe/Rome datetimes in `YYYY-MM-DDTHH:mm` format with no timezone offset. Backend parses those values as Europe/Rome local time and performs no timezone conversion.

### UX or accessibility expectations
- Follow portal Matrix/Stripe-level design per `docs/UI-UX.md`.
- 5 views rendered as top-level tabs or sub-routes of the app.

## Open Questions and Deferred Decisions

All V1 open questions resolved on 2026-04-18:

- **Q1.** ✅ Active-customer predicate confirmed (see `Customer.listActive()` SQL).
- **Q2.** ✅ `maxampere / 2` is an intentional safety margin; gauge formula ported verbatim.
- **Q3.** ✅ CSV export in v1 for "Addebiti"; PDF deferred.
- **Q4.** ✅ No bulk actions on low-consumption in v1; deferred task recorded in `docs/TODO.md` (Energia in DC App → "Bulk Actions on Low-Consumption Search").
- **Q5.** ✅ Single endpoint `GET /api/energia-dc/v1/customers/{id}/kw?period=day|month&cosfi=` (payloads identical between day/month; revisit if they diverge).
- **Q6.** ✅ All timestamps stay in `Europe/Rome` local time end-to-end; no timezone conversion layer is introduced.

## Acceptance Notes

- **What the audit proved directly:** Query SQL, widget configuration, JSObject methods, embedded rules (self-exclusion, breaker mapping, gauge formula, 225V coefficient), bugs (cosfi scale in weekly branch, fire-and-forget loads, SQL-injection-risk unprepared queries).
- **What the expert confirmed (2026-04-17):** 225V formula stays; weekly period dropped; no-variable-billing detail query switches to ID-keyed.
- **What the expert confirmed (2026-04-18):** Active-customer SQL (Q1); `maxampere/2` is an intentional safety margin (Q2); CSV-only export in v1 (Q3); low-consumption bulk actions deferred (Q4); single `period=` kW endpoint (Q5); Europe/Rome TZ (Q6).
- **What still needs lightweight validation before signoff:** merged power-readings pagination/order, ID-keyed no-variable detail loading, optional-customer behavior in low-consumption search, and the local Europe/Rome datetime parsing contract.
- **What can still be decided during implementation:** self-exclusion config key name, breaker-mapping storage (inline CASE vs lookup table), pagination defaults for `/power-readings`, shared-types package location.
