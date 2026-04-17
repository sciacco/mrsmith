# Zammu → 3-way split: partition map

> **Source:** `apps/zammu/ZAMMU-AUDIT.md` (audit 2026-04-09)
> **Scope:** Split Zammu Appsmith app into three portal mini-apps.
> **Strategy:** Boundary is by Appsmith page.

---

## Targets

| Target app | Portal category | Assigned page(s) | Primary datasource |
|------------|-----------------|------------------|--------------------|
| **Coperture** | Smart Apps | `Coperture` | `dbcoperture` (PostgreSQL) |
| **Energia in DC** | Smart Apps | `Energia variabile` | `grappa` (MySQL) |
| **Simulatori di vendita** | Mktg & Sales | `IaaS calcolatrice` | `carbone.io` (REST) |

## Dropped

| Item | Reason |
|------|--------|
| Page: `Home` | Trivial greeting + placeholder text; portal already provides landing/dashboard. Confirmed with expert 2026-04-17. |
| JS lib: `ExcelJS` 4.3.0 | Audit: "Not visibly referenced in any page — likely leftover". |
| JS lib: `fast-xml-parser` 3.17.5 | Audit: "Not visibly referenced in any page — likely leftover". |
| `utils.test()` (Coperture) | Debug/test with hardcoded GEA data; dead code. |
| `jschart_kw.myFun2()` (Energia) | Debug; dead code. |
| `utils.pippo()` (IaaS) | Debug; dead code. |
| `get_kw_weeks` query + "Settimanale" switch branch (Energia) | Unreachable: `s_period` dropdown has no "week" option; also has a cosfi scale bug. |
| GraphQL stub `getTransazioni` + `transazioni-whmcs` datasource | Never runs (`executeOnLoad: false`), only fetches `cliente`. |

## Deferred (explicitly out of scope for this 3-way split)

| Item | Notes |
|------|-------|
| Page: `Transazioni whmcs` | Hidden page, WHMCS paid transactions. Expert decision 2026-04-17: treat as a separate 4th target for a future migration session; do not include in any of the three specs. |
| Datasource: `whmcs_prom` (MySQL) | Referenced only by `Transazioni whmcs`. Deferred. |
| Query: `fatture_pagate` | Deferred with the page. |

---

## Attribution table (per audit element)

### Pages
| Appsmith page | Target | Notes |
|---------------|--------|-------|
| Home | — (dropped) | |
| Coperture | coperture | |
| Energia variabile | energia-in-dc | |
| Transazioni whmcs | — (deferred) | |
| IaaS calcolatrice | simulatori-di-vendita | |

### Datasources
| Datasource | Plugin | Target | Notes |
|------------|--------|--------|-------|
| dbcoperture | PostgreSQL | coperture | Exclusive to this target. |
| grappa | MySQL | energia-in-dc | Exclusive to this target. **Per `docs/IMPLEMENTATION-KNOWLEDGE.md`: `cli_fatturazione.codice_aggancio_gest` = Alyante ERP ID = Mistra `customers.customer.id`** — this customer-ID mapping is a portal-wide constant, not app-specific, but Energia in DC is the only target in this split that touches it. |
| whmcs_prom | MySQL | — (deferred) | |
| transazioni-whmcs | GraphQL | — (dropped, stub) | |
| carbone.io | REST | simulatori-di-vendita | Exclusive to this target. Proxy through backend (API key must not live in frontend). |

### JSObjects
| JSObject | Page | Target |
|----------|------|--------|
| `utils` (Coperture) | Coperture | coperture |
| `utils` (Energia) | Energia variabile | energia-in-dc |
| `echart_ampere` | Energia variabile | energia-in-dc |
| `jschart_kw` | Energia variabile | energia-in-dc |
| `utils` (IaaS) | IaaS calcolatrice | simulatori-di-vendita |

### Queries (per audit §3, excluding dropped/deferred)
| Query | Target |
|-------|--------|
| `get_states`, `get_cities`, `get_addresses`, `get_house_numbers`, `get_coverage`, `get_details_types` | coperture |
| `get_customers`, `get_sites`, `get_rooms`, `get_racks`, `get_rack_details`, `get_socket_status`, `get_power_readings`, `count_power_reading`, `stats_last_days`, `get_kw_days`, `get_kw_months`, `get_addebiti_by_cli`, `anagrafiche_no_variable`, `racks_no_variable`, `rack_basso_consumo` | energia-in-dc |
| `render_template` | simulatori-di-vendita |
| `get_kw_weeks` | — (dropped) |
| `fatture_pagate`, `getTransazioni` | — (deferred / dropped) |

---

## Shared / cross-cutting concerns

The three target apps are structurally independent — the audit explicitly notes "No cross-page shared JSObjects" and "Three completely independent datasources serving three distinct domains". Consequently the `shared` boundary doc captures **portal-level concerns**, not overlapping business logic:

- **Authentication:** Keycloak OIDC (portal-wide). Per CLAUDE.md, each app needs an access role `app_{appname}_access`. Proposed role names (to be confirmed during implementation planning):
  - `app_coperture_access`
  - `app_energiadc_access` (Keycloak role names are usually compact — confirm naming)
  - `app_simulatorivendita_access`
- **Navigation:** Each app is a standalone Vite+React mini-app launched from the portal catalog. No cross-app navigation identified in the audit.
- **User identity:** Audit shows `appsmith.user.name || appsmith.user.email` on Home only (dropped). In the rewrite each app receives the Keycloak token via `@mrsmith/auth-client`.
- **Customer ID concept:** Only Energia in DC touches a customer table (`cli_fatturazione` in grappa). Neither Coperture nor Simulatori di vendita queries customers. The cross-database customer mapping rule in `docs/IMPLEMENTATION-KNOWLEDGE.md` is documented there, not duplicated here.
- **No shared entities** across the three targets.
- **No cross-app user journeys** identified in the audit.

---

## Unresolved at partition time

- Keycloak role naming variants for multi-word app IDs (hyphen vs. flat) — implementation-phase concern, not blocking the spec.
- For each target, whether current-state bugs are carried forward, fixed on migration, or deferred — captured as phase-level open questions per target.
