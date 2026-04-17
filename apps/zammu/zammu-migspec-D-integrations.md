# Zammu migspec — Phase D: integrations & data flow

> External systems per target, end-to-end user journeys, hidden triggers / timers, and anything the Appsmith export can't reveal.

---

## coperture

### External systems
| System | Type | Purpose | Proposed backend relationship |
|--------|------|---------|-------------------------------|
| `dbcoperture` | PostgreSQL | Primary read source. Contains tables `network_coverage_cities`, `network_coverage_addresses`, `network_coverage_house_numbers`; view `v_get_coverage`; functions `get_states()`, `get_coverage_details_types()` | Backend owns the connection (pool + Keycloak-authenticated routes). Frontend never touches DB. |
| `static.cdlan.business` CDN | Static asset | Operator logos (4 URLs currently hardcoded in frontend) | If operator master data moves backend, the `logo_url` travels with it; frontend just renders the URL. Asset origin unchanged. |

### End-to-end user journey
```
User → Portal → Coperture app
  → opens "Ricerca copertura"
  → selects Provincia (API call 1)
  → selects Comune (API call 2)
  → selects Indirizzo (API call 3)
  → selects Numero civico (API call 4)
  → clicks "Cerca copertura"
  → results with operator logos & profile/detail breakdowns (API call 5)
```
All calls are request/response. No async jobs, no webhooks, no background processing.

### Hidden triggers / timers
- None. Zero cron, zero push, zero polling. All user-initiated.

### Data ownership boundaries
- **Reader only.** Backend performs read-only queries. No writes to `dbcoperture` from this app.
- **Master data seed:** the Operator entity will be owned wherever the backend decides to host it (operator-management is an open question; see Phase A q #3). Not owned by this mini-app.

### Things the export can't reveal (to confirm with expert)
- Update cadence of `coperture` tables — who ingests provinces/cities/addresses, on what schedule? Relevant for caching headers on the cascading endpoints.
- Whether `v_get_coverage` is a simple view or materialized; affects query-time expectations and potential stale data.

---

## energia-in-dc

### External systems
| System | Type | Purpose | Proposed backend relationship |
|--------|------|---------|-------------------------------|
| `grappa` | MySQL | Primary read source: customers (`cli_fatturazione`), buildings (`dc_build`), datacenters (`datacenter`), racks (`racks`), sockets (`rack_sockets`), raw readings (`rack_power_readings`), daily summaries (`rack_power_daily_summary`), billing (`importi_corrente_colocation`) | Backend owns connection. All queries parameterized. |

### End-to-end user journeys

#### View 1 — Situazione per rack
```
user picks Cliente → API: GET /customers/{id}/sites
  → Site → GET /sites/{id}/rooms
    → Room → GET /rooms/{id}/racks
      → Rack + date range → click "Aggiorna"
        → parallel: GET /racks/{id}, /racks/{id}/socket-status, /racks/{id}/stats-last-days
        →         : GET /racks/{id}/power-readings?from=&to=&page=&size=  (with total count)
```

#### View 2 — Consumi in kW
```
user picks customer + period (day/month) + cosfi
  → click "Aggiorna"
  → GET /customers/{id}/kw?period=&cosfi=
  → chart renders
```

#### View 3 — Addebiti
```
user picks customer
  → GET /customers/{id}/addebiti
  → table renders
```

#### View 4 — Racks no variable
```
page load → GET /no-variable-billing/customers
  → user selects row
  → GET /no-variable-billing/customers/{id}/racks   (ID-keyed, not name-keyed — rewrite change)
```

#### View 5 — Consumi < 1A
```
user sets threshold + optional customer
  → click "Cerca"
  → GET /low-consumption?min=&customer=
```

### Hidden triggers / timers
- None at the page level. However, audit-noted embedded rules (`kW = ampere × 225V / 1000`, `maxampere` CASE, company self-exclusion) are implicit business "behaviors" — they look like triggers but are static transforms.
- Audit flags potential race conditions from `utils.loadData()` fire-and-forget — in the rewrite, this is eliminated by per-hook loading state.

### Data ownership boundaries
- **Reader only** from this app. All tables are owned upstream (provisioning / billing systems populate them).
- Do not write back to `grappa` from this app.

### Things the export can't reveal (to confirm)
- Whether `rack_power_readings` grows fast enough that the paginated endpoint needs read replicas or materialized aggregates.
- Where the 225V assumption originally came from (operational assumption, vendor default, calibration).
- Whether there's a preferred time zone for `date` fields (audit shows `YYYY-MM-DD HH:mm` formatting but not the DB tz).
- Whether the `anagrafiche_no_variable` query joins against a contract/billing-mode table or uses the rack-level `variable_billing` flag exclusively.

---

## simulatori-di-vendita

### External systems
| System | Type | Purpose | Proposed backend relationship |
|--------|------|---------|-------------------------------|
| Carbone.io (`/render/{templateId}`) | REST API | PDF template rendering for IaaS quote output | **Backend proxy.** API key + template ID live in backend config. Frontend calls `POST /api/simulatori/iaas/quote`; backend renders via Carbone and returns the PDF stream (or a short-lived URL). |
| (Future) Pricing master | Config or DB | Hold Diretta/Indiretta tier prices | Placement is Phase A open question #1. |

### End-to-end user journey
```
user opens calculator
  → backend serves active pricing (for both tiers) on page load: GET /pricing
  → user fills form + picks tier
  → click "Calcola" → frontend computes totals from pricing + quantities
  → (optional) click "Genera PDF"
    → POST /quote (quantities, tier, totals)
    → backend validates totals, calls Carbone.io, streams/returns PDF
    → (optional) persist quote row if that decision lands
```

### Hidden triggers / timers
- None. Fully synchronous and user-initiated.

### Data ownership boundaries
- Pricing master data: owned backend (once migrated out of the frontend JSObject).
- Carbone template: template ID + API key owned by backend config. **Never** in the frontend bundle.
- If quotes get persisted, the new quote-store is owned by this mini-app's backend module.

### Things the export can't reveal (to confirm)
- Whether Carbone.io is the only rendering vendor under consideration (or whether to abstract behind a renderer interface so we can swap later).
- SLA requirements for PDF generation (latency, concurrency).
- Who currently has the Carbone.io admin credentials and where the template is versioned.

---

## Cross-target (shared boundary preview)

| Concern | Scope |
|---------|-------|
| Auth (Keycloak OIDC, per-app access role) | Shared across all 3 apps; each app enforces its own `app_{appname}_access` role. |
| Portal shell (navigation, logout, user menu) | Provided by the portal host; each mini-app is mounted independently. |
| `@mrsmith/api-client` + `@mrsmith/auth-client` | Reused across all 3 frontends. |
| Shared UI (`@mrsmith/ui`) | Reused — cascading-select pattern in Coperture and Energia is a candidate to live here. |
| Cross-app data flows | **None identified** in the audit. |
| Shared entities | **None identified.** Customer in Energia is scoped to grappa; Coperture has no customer concept; Simulatori doesn't query customers in the current design. |

---

## Summary of Phase D findings

- All three target apps are **read-heavy** (one exception: Simulatori di vendita's PDF render, which is a fire-and-return write to Carbone). No background workers, no webhooks, no scheduled jobs surfaced by the audit.
- The three apps are **domain-disjoint** — no cross-app user journeys, no shared entities. The shared boundary doc captures only portal-level concerns (auth, shell, shared libs).
- The audit cannot reveal: upstream data cadence, DB time zones, SLAs, credential ownership — these become open questions carried into the final specs.
