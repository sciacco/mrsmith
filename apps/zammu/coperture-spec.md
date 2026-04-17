# Coperture — Application Specification

## Summary
- **Application name:** Coperture
- **Portal category:** Smart Apps
- **Audit source:** `apps/zammu/ZAMMU-AUDIT.md` §2.2 (source page: `Coperture` inside Zammu Appsmith app)
- **Spec status:** Ready for hand-off to `portal-miniapp-generator`
- **Last updated decisions (2026-04-17):**
  - Operator master data: backend DB table (decided).
  - Page `Home` from Zammu is dropped.
  - `Transazioni whmcs` is a separate future target, not in this app.

## Current-State Evidence
- **Source pages/views:** Single Appsmith page `Coperture` within the `zammu-main` application.
- **Source entities and operations:** State, City, Address, HouseNumber, CoverageResult, Operator (hardcoded 4-entry map in frontend), CoverageDetailType — all read-only.
- **Source integrations and datasources:** `dbcoperture` PostgreSQL schema `coperture`: tables `network_coverage_cities`, `network_coverage_addresses`, `network_coverage_house_numbers`; view `v_get_coverage`; stored functions `get_states()`, `get_coverage_details_types()`. Operator logos served from `static.cdlan.business` CDN.
- **Known audit gaps or ambiguities:**
  - Exact JSON shape of `get_states()` and `v_get_coverage.profiles[]` / `details[]` not visible in the audit (only the consumer usage).
  - `tech` field is consumed as a free-text string; its domain (enum or open) is not audit-visible.
  - The `/0000$/` strip on detail values is a presentation workaround in the current app; its business meaning is unconfirmed.

## Entity Catalog

### Entity: State (Provincia)
- **Purpose:** Italian province, top of the coverage address hierarchy.
- **Operations:** `list()`.
- **Fields and inferred types:** `id` (int), `name` (string).
- **Relationships:** 1 → N City.
- **Constraints and business rules:** None visible.
- **Open questions:** Confirm exact column list returned by `coperture.get_states()`.

### Entity: City (Comune)
- **Purpose:** Municipality within a province.
- **Operations:** `listByState(state_id)`.
- **Fields:** `id` (int), `name` (string), `state_id` (int FK).
- **Relationships:** N ← 1 State; 1 → N Address.
- **Constraints:** Results ordered by `name`.

### Entity: Address (Indirizzo)
- **Purpose:** Street within a city.
- **Operations:** `listByCity(city_id)`.
- **Fields:** `id` (int), `name` (string), `city_id` (int FK).
- **Relationships:** N ← 1 City; 1 → N HouseNumber.
- **Constraints:** Ordered by `name`.

### Entity: HouseNumber (Numero civico)
- **Purpose:** Street number within an address.
- **Operations:** `listByAddress(address_id)`.
- **Fields:** `id` (int), `name` (string), `address_id` (int FK).
- **Relationships:** N ← 1 Address; 1 → N CoverageResult.

### Entity: CoverageResult
- **Purpose:** A commercial coverage offering at a specific house number, belonging to a single operator and technology.
- **Operations:** `listByHouseNumber(house_number_id)`.
- **Fields:**
  - `coverage_id` (int/string — list key)
  - `operator_id` (int FK)
  - `operator_name` (string — may be denormalized for convenience)
  - `tech` (string — technology label, e.g. FTTH/FTTC)
  - `profiles` (array of profile objects — at minimum each has a `name`; other fields unknown in audit)
  - `details` (array of detail items — each is a `{ type_id, value }` pair; `value` trailing-`0000` may be stripped before display)
  - `house_number_id` (int FK)
- **Relationships:** N ← 1 HouseNumber; each result references 1 Operator and N CoverageDetailType via `details[].type_id`.
- **Constraints and business rules:**
  - Default ordering: `operator, tech`.
  - The trailing-`0000` strip on `value` is currently done client-side; see open questions.
- **Open questions:**
  - Exact shape of `profiles[]` inside `v_get_coverage` (beyond `name`).
  - Is the `0000` strip a genuine data presentation rule or a workaround? If the former, the backend should return already-stripped values.

### Entity: Operator
- **Purpose:** Master data for the network operators whose coverage is surfaced.
- **Operations:** `list()` (consumed internally; no admin UI in v1).
- **Fields:** `id` (int), `name` (string), `logo_url` (string absolute URL).
- **Relationships:** 1 → N CoverageResult.
- **Constraints and business rules:** Operator master data lives in a new backend table seeded with the 4 current operators (TIM, Fastweb, OpenFiber, OpenFiber CD). Logos continue to be served from the existing `static.cdlan.business` CDN.
- **Open questions:**
  - Final table location (new schema in `dbcoperture` vs. Mistra — implementation decision).
  - Is an admin UI desired later (out of scope for v1)?

### Entity: CoverageDetailType
- **Purpose:** Label lookup for coverage detail items (one row per detail kind).
- **Operations:** `list()` — alternatively inlined into CoverageResult responses.
- **Fields:** `id` (int), `name` (string).
- **Relationships:** Referenced by `CoverageResult.details[].type_id`.
- **Open questions:** Backend may prefer to inline the `type_name` into the `details[]` payload rather than expose a separate types endpoint. Decide at API design time.

## View Specifications

### View: "Ricerca copertura" (Coverage Lookup)
- **User intent:** Given a physical address, tell me which commercial network coverage profiles are available and from which operator.
- **Interaction pattern:** Cascading filter → on-demand search → results list.
- **Main data shown or edited:** Read-only; no edits.
- **Key actions:**
  - Provincia → Comune → Indirizzo → Numero civico (cascading selects).
  - "Cerca copertura" button fetches coverage and updates the search breadcrumb.
  - "Reset" clears the form.
- **Entry and exit points:** Entry from portal (Smart Apps category). No in-app navigation targets; no external hand-offs.
- **Notes on current vs intended behavior:**
  - Current: HTML strings for profile/detail rendering, hardcoded operator logo URLs, a separate TEXT widget for the breadcrumb.
  - Intended: React components for profile and detail lists, logos driven by the Operator entity returned by the API, breadcrumb as derived state.
  - An empty state ("nessuna copertura disponibile") is recommended but not present in the source.

## Logic Allocation

### Backend responsibilities
- Own all DB access to `dbcoperture` (read-only).
- Unwrap `get_states()` JSON payload and expose a clean `{id, name}` array.
- Own Operator master data (new table, seed with current 4 operators). Serve `logo_url` per result.
- Decide whether `CoverageDetailType` names are inlined into `details[]` or returned via a separate endpoint.
- Authenticate every request with a Keycloak access token; require role `app_coperture_access`.

### Frontend responsibilities
- Cascading select UX (shared pattern candidate for `@mrsmith/ui`).
- Empty state, loading state, error state per step.
- Render profile and detail lists as React components (no HTML-string widgets).
- Render operator logo from the URL supplied by the backend.

### Shared validation or formatting
- Type definitions for State / City / Address / HouseNumber / CoverageResult / Operator / CoverageDetailType in `@mrsmith/api-client` (or the app's own types module if not reused).

### Rules being revised rather than ported
- Operator master data moves from frontend-hardcoded constants to a backend table.
- Breadcrumb becomes derived from form state (no imperative `t_ricerca` write).

## Integrations and Data Flow

### External systems and purpose
- `dbcoperture` PostgreSQL — primary read store.
- `static.cdlan.business` CDN — logo assets (unchanged origin; referenced by the Operator entity).

### End-to-end user journeys
1. User opens Coperture from the portal sidebar.
2. Selects Provincia → cities load.
3. Selects Comune → addresses load.
4. Selects Indirizzo → house numbers load.
5. Selects Numero civico → clicks "Cerca copertura".
6. Results list renders per-row with operator logo, technology, profiles, details.

### Background or triggered processes
- None. Fully user-initiated, request/response.

### Data ownership boundaries
- This app is a **pure reader** of `dbcoperture`. No writes.
- The new Operator table is owned by this app's backend module; its content does not cross into other mini-apps in this migration.

## API Contract Summary

### Required capabilities
- Cascading geographic lookup.
- Coverage lookup by house number.
- Operator master-data serving.

### Read endpoints or queries (proposed shape)
- `GET /api/coperture/states`
- `GET /api/coperture/states/{state_id}/cities`
- `GET /api/coperture/cities/{city_id}/addresses`
- `GET /api/coperture/addresses/{address_id}/house-numbers`
- `GET /api/coperture/house-numbers/{house_number_id}/coverage`
- `GET /api/coperture/operators` *(consumed only internally if backend denormalizes logos into the coverage payload; otherwise exposed)*
- `GET /api/coperture/detail-types` *(optional; may be inlined in `/coverage`)*

### Write commands or mutations
- None in v1.

### Derived or workflow-specific operations
- None.

## Constraints and Non-Functional Requirements

### Security or compliance
- All routes behind Keycloak OIDC; require role `app_coperture_access`.
- Replace any unprepared SQL from the source with parameterized queries.
- Logos served from existing CDN (no new trust boundary).

### Performance or scale
- All payloads are small (cascading lookups return tens to low hundreds of rows). Typical deployment caching TTLs (~minutes) are adequate.
- `v_get_coverage` response size is bounded by per-address operator/tech combinations — expected small.

### Operational constraints
- Requires backend DB connectivity to `dbcoperture` from the Go backend pod.

### UX or accessibility expectations
- Use portal/UI-UX conventions (sidebar, Matrix theme) per `docs/UI-UX.md`. Cascading-select component is a candidate to live in `@mrsmith/ui`.
- Provide empty states when a level returns no rows.

## Open Questions and Deferred Decisions

- **Q1.** Exact JSON shape of `coperture.get_states()`.
  - *Needed input:* read the function definition or a sample row; confirm field list.
  - *Decision owner:* Backend implementer.
- **Q2.** Structure of `profiles[]` and `details[]` in `v_get_coverage`.
  - *Needed input:* view definition or sample rows.
  - *Decision owner:* Backend implementer.
- **Q3.** Keep the `/0000$/` value strip?
  - *Needed input:* whether the trailing zeros reflect data-storage convention (strip server-side) or a real display rule.
  - *Decision owner:* Domain expert.
- **Q4.** Inline detail-type names in the coverage response, or separate endpoint?
  - *Needed input:* payload-size vs. cache-friendliness trade-off.
  - *Decision owner:* Backend implementer.
- **Q5.** Is `tech` an enum? If so, the values should be defined.
  - *Needed input:* source of truth for technology labels.
  - *Decision owner:* Domain expert.
- **Q6.** Upstream cadence for `coperture` tables (who ingests provinces/cities/addresses, how often) — affects caching policy.
  - *Decision owner:* Data/ingest team.

## Acceptance Notes

- **What the audit proved directly:** Page widgets, query SQL, JSObject method bodies, datasource configuration, embedded regexes, hardcoded operator-logo map.
- **What the expert confirmed (2026-04-17):** Operator master data moves to a backend DB table (not config, not frontend constant).
- **What still needs validation:** PG function/view shapes (Q1, Q2), presentation-vs-data meaning of the `0000` strip (Q3), `tech` enum (Q5).
