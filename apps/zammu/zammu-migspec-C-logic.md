# Zammu migspec — Phase C: logic placement

> Each non-trivial JSObject method and inline binding classified as **domain**, **orchestration**, or **presentation**, then assigned a recommended placement (backend / frontend / shared / dead).

Legend:
- **D** — domain logic (business rule; should live backend unless purely view-layer)
- **O** — orchestration (UI sequencing)
- **P** — presentation (rendering)
- **X** — dead code / drop
- **B** / **F** / **S** — backend / frontend / shared (type package)

---

## coperture

### `utils` (Coperture page)
| Method | Class | Current behavior | Placement | Notes |
|--------|-------|------------------|-----------|-------|
| `formatProfili(p)` | P | Maps `p[].name` into HTML table | **F** | Replace with React component rendering a list. |
| `formatDettagli(p)` | P + D | HTML `<table><ul>`, resolves detail-type name via `getDetailName`, strips `/0000$/` from values | **F** for rendering, **B** for value strip if it's a data rule | The `0000` strip may be compensating for a data-storage convention — if so, backend should return already-stripped values and the frontend renders plainly. ⚠ Business confirmation. |
| `getImageUrl(o)` | D | Hardcoded 4-entry operator→logo map | **B** | Move to backend operator master data or portal config; frontend consumes `logo_url` from the API. |
| `getDetailName(i)` | D | Looks up detail type name from `get_details_types.data` | **B** (via inlined coverage response) or **S** (if we keep a separate types endpoint) | Preferred: backend joins detail-type names into the coverage response so frontend has no lookup step. |
| `updateTestoRicerca()` | O | Builds breadcrumb string from selected labels, writes to `t_ricerca` | **F** (drop entirely) | In React this is derived state — no imperative write needed. |
| `test()` | X | Debug fixture | drop | — |

### Inline bindings (notable)
- `get_states` consumed as `data[0].get_states` (PG function returns JSON-wrapped). In the rewrite the backend should unwrap and return a flat array. Move shaping **backend**.

---

## energia-in-dc

### `utils` (Energia page)
| Method | Class | Current behavior | Placement | Notes |
|--------|-------|------------------|-----------|-------|
| `loadData()` | O | Fires 3 queries fire-and-forget + awaits 2 | **F** | In React/RQ-style: one hook per data need. The fire-and-forget pattern (audit bug #4) goes away because rendering is governed by individual loading states. |
| `myNoVarCli()` | D (trivial) | Extracts distinct `intestazione` values from `racks_no_variable.data` | drop | In the rewrite the no-variable listing is ID-keyed; distinct-by-name is unnecessary. |

### `echart_ampere`
| Field | Class | Placement | Notes |
|-------|-------|-----------|-------|
| `option` | P | **F** | ECharts config for the dual-axis ampere/kW chart. Port as React ECharts component. |
| `Rigenera()` | P | **F** | Data mapping into chart series. |

### `jschart_kw`
| Field/Method | Class | Placement | Notes |
|--------------|-------|-----------|-------|
| `aggiorna()` | O | **F** | Period switch → query → plot. Replace with parameterized hook. |
| `plot(dataset)` | P + D | **F** with backend data | yAxis bounds computation OK on frontend. Title composition OK on frontend. |
| `options` | P | **F** | Bar chart, log-2 y-axis. |
| `myFun2()` | X | drop | Debug dead code. |

### SQL-resident business rules (lift to backend)
| Rule | Current location | Placement |
|------|------------------|-----------|
| `id_anagrafica <> 3` company self-exclusion | Embedded in SQL for customer/no-variable listings | **B** (preferably as a config flag, not a hardcoded constant) |
| `maxampere` CASE on `magnetotermico` | Socket status SQL | **B** (and ideally a lookup table or config) |
| Progress gauge formula `ampere/(maxampere/2)*100` | Widget binding | **F** (pure view) — but confirm the `/2` safety margin is intentional |
| `kW = SUM(ampere) * 225 / 1000` | `stats_last_days` SQL | **B** — and revisit per-phase voltage (audit bug #3) |
| cosfi applied as `value/100` | Day/Month kW queries | **B** — ensure consistent convention |

### API-level decisions
- Server-side pagination for power readings → **B** exposes `page`, `size`, total count on one endpoint (merge `count_power_reading` + `get_power_readings`).
- Replace `LIKE` on numeric FK with `=` → **B** fix on migration.
- Parameterize all currently-unprepared queries → **B** security fix.

---

## simulatori-di-vendita

### `utils` (IaaS page)
| Method/Field | Class | Current behavior | Placement | Notes |
|--------------|-------|------------------|-----------|-------|
| `qta` (reactive) | O | Quantities from form | **F** (form state) | React form state / controlled inputs. |
| `prezzi`, `prezzi_diretta`, `prezzi_indiretta` | D | Tier rates | **B** (preferred) or **config in repo** | Audit recommends moving out of frontend. Confirm Phase A question #1. |
| `templateId` | D | Carbone template hash | **B** config | Never in frontend. |
| `hours = 730` | X | Declared, never used | drop | |
| `days = 30` | D | Monthly multiplier | **B** or **S** (single source of truth) | Confirm 30 vs 30.4167 (Phase A q #2). |
| `loadPrezzi()` | O | Read inputs into `qta` | **F** | Trivial with React controlled inputs. |
| `calcolaTotali()` | D | Line items, subtotals, monthly total | **F** OK (simple arithmetic) or **B** (if we want server-side validation for PDF) | Recommend: frontend computes for display; backend recomputes at PDF render time for trust. |
| `updatePrezzi()` | P + D | Copies tier into active prices + regenerates price-table HTML | **F** (rendering) + data from **B** (tier) | React renders the price table from tier data; HTML string generation goes away. |
| `getURL()` | O | Builds Carbone download URL from render response | drop (**B** proxy returns a direct URL or streams the PDF) | |
| `pippo()` | X | Debug | drop | |

### SQL-resident business rules
- None (no DB).

### API-level decisions
- Backend proxy for PDF render. Endpoint shape: `POST /api/simulatori/iaas/quote` → returns PDF stream or a signed URL. **Confirmed direction.**
- Pricing fetch: `GET /api/simulatori/iaas/pricing?tier=diretta|indiretta`.
- Consider persisting quotes (Phase A q #6) — if yes: `POST /api/simulatori/iaas/quote` writes a quote row and returns an ID + PDF link.

---

## Cross-target logic summary

| Concern | Placement rule |
|---------|----------------|
| Rendering (HTML strings, logos, charts) | Frontend React components. |
| Hardcoded lookups (operator logos, breaker-to-amp mapping, pricing tiers) | Move to backend (config or DB). |
| SQL-side business filters (self-exclusion, hardcoded voltage, date filters) | Either surface as API parameters or move to config in backend. |
| Pagination | Backend returns `{items, total, page, size}`. |
| Parameterization | All queries must use prepared statements in backend. |
