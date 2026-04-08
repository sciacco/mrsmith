# Phase C: Logic Placement — Panoramica Cliente

## Overview

This app is **entirely read-only** — no writes to any database. All logic is either query construction, data transformation for charts, or presentation formatting.

---

## JSObject Analysis

### Dashboard JSObjects (EXCLUDED — page deferred)

Not analyzed. Captured in audit for future reference.

---

### IaaS PPU: `utils.aggiornaSerie()`

**Original code:**
```javascript
// Original Appsmith JSObject: IaaS Pay Per Use > utils
export default {
    daySeries: [],
    async aggiornaSerie () {
        await get_charges_by_type.run();
        const r = get_charges_by_type.data;
        const serie = [];
        r.forEach((item) => {
            Object.keys(item).forEach((key) => {
                if (key.startsWith("ut") && item[key] > 0) {
                    serie.push({x: key, y: item[key]});
                }
            });
        });
        this.daySeries = serie;
        return serie;
    }
}
```

**Classification:** Presentation logic
- Transforms a single-row charge breakdown into a chart-ready array
- Filters out zero-value charge types (only shows types with actual consumption)
- The `ut` prefix convention is an artifact of the SQL column aliases

**Placement:** Frontend
- The backend returns charge data as-is (one object with typed charge fields)
- The frontend maps non-zero fields to chart series
- Or: backend could return as `[{type: "RunningVM", amount: 1.23}, ...]` array — eliminating the `ut` prefix convention

**Question for expert:**
18. Should the backend return charges as a flat object (like Appsmith SQL) or as a typed array `[{type, label, amount}]`? The array format is cleaner for charting and allows the backend to own label names.

---

### Timoo: `utils` JSObject

**Original code:**
```javascript
// Original Appsmith JSObject: Timoo tenants > utils
export default {
    tenants: [],
    statistiche: [],
    totalUsers: 0,
    totalSE: 0,
    generaTenantIdList () {
        // DROPPED — was building TIMOO REST API URL
        const tenantIds = getAnisettaTenants.data.map(tenant => tenant.as7_tenant_id);
        const idString = tenantIds.join(',');
        const url = `/orgUnits?where=type.eq('tenant').and(id.in(${idString})).and(name.ne('KlajdiandCo'))`;
        return url;
    },
    async listaTenants () {
        // DROPPED — was calling TIMOO REST API via getPlaceholder proxy
        const URL = this.generaTenantIdList() ;
        await getPlaceholder.run({URL: URL});
        this.tenants = getPlaceholder.data.orgUnits;
        return getPlaceholder.data;
    },
    async pbxStats () {
        await getPbxByTenandId.run();
        const pbxSummaries = [];
        let totalServiceExtensions = 0;
        let totalUsers = 0;
        this.statistiche = {};

        for (const pbx of getPbxByTenandId.data) {
            totalUsers += pbx.users;
            totalServiceExtensions += pbx.service_extensions;

            pbxSummaries.push({
                pbxName: pbx.pbx_name,
                pbxId: pbx.pbx_id,
                users: pbx.users,
                serviceExtensions: pbx.service_extensions,
                ivr: 0,
                totale: pbx.users+pbx.service_extensions
            });
        };
        // Commented-out REST API approach omitted (see Phase A for full original)
        this.statistiche = pbxSummaries;
        this.totalUsers = totalUsers;
        this.totalSE = totalServiceExtensions;
        return {pbxSummaries: pbxSummaries,totalUsers: totalUsers, totalServiceExtensions: totalServiceExtensions } ;
    }
}
```

**Classification by method:**

| Method | Classification | Placement | Notes |
|--------|---------------|-----------|-------|
| `generaTenantIdList()` | Orchestration (REST API) | **DROP** | TIMOO API excluded per user direction |
| `listaTenants()` | Orchestration (REST API) | **DROP** | Replaced by direct DB query `getAnisettaTenants` |
| `pbxStats()` | Data aggregation | **Backend** | Summing totals across PBX instances is backend logic |

**Practical impact:**
- Tenant list → single `GET /api/.../tenants` from Anisetta DB
- PBX stats → `GET /api/.../pbx?tenant=X` returns rows + totals, computed server-side
- No JSObject logic survives in the migration — all moves to backend or becomes trivial frontend data binding

---

## Inline Binding / Expression Analysis

### Dynamic SQL WHERE clauses (Ordini ricorrenti)

**Original Appsmith binding:**
```javascript
// In get_ordini_ricorrenti query body, "Ordini ricorrenti" page
// "All clients" conditional WHERE
{{s_o_cliente.selectedOptionValue === '' || s_o_cliente.selectedOptionValue == -1 ? 'true' : " numero_azienda = '"+s_o_cliente.selectedOptionValue+"'"  }}
```

**Classification:** Query orchestration (with SQL injection risk)
**Placement:** Backend — query parameter handling with proper parameterization
**Change from current:** Backend receives `cliente` as parameter; builds WHERE clause safely. `-1` or empty = no client filter.

---

### Multi-select IN clause construction (Accessi, Ordini)

**Original Appsmith binding:**
```javascript
// Used in get_accessi_cliente, get_ordini_ricorrenti, GET_ordini_Ric_Spot
// Example from Accessi page:
{{ms_clienti.selectedOptionValues.map(i => "'" + i + "'").join()||-1}}
// Example from Ordini pages:
{{ms_o_stati.selectedOptionValues.map(i => "'" + i + "'").join()}}
{{ms_stati.selectedOptionValues.map(i => "'" + i + "'").join()}}
```

**Classification:** Query orchestration (SQL injection risk)
**Placement:** Backend — receives array parameter, builds parameterized IN clause
**Change from current:** Array passed as JSON body or query parameter; backend uses parameterized queries.

---

### Document grouping (Fatture)

**Original SQL (in get_fatture query):**
```sql
CASE WHEN rn = 1
    THEN doc || ' ' || num_documento || CHR(13) || CHR(10) || to_char(data_documento, '(YYYY-MM-DD)')
    ELSE NULL
END AS documento
```

**Classification:** Presentation logic
**Placement:** Could be either:
- **Backend:** Compute in SQL (as now) — keeps presentation logic server-side but it's tightly coupled to table display
- **Frontend:** Backend returns `rn`, `doc`, `num_documento`, `data_documento` as separate fields; frontend groups visually

**Recommendation:** Keep as-is in backend SQL for as-is migration. The query already computes it — no reason to change.

**Question for expert:**
19. For invoice line grouping: should the backend compute the grouped `documento` display string (as Appsmith does now), or return flat rows and let the frontend handle visual grouping?

---

### Order visual grouping (Ordini R&S)

**Original SQL (in GET_ordini_Ric_Spot query):**
```sql
CASE
   WHEN r.progressivo_riga = 1 THEN o.nome_testata_ordine
   ELSE NULL::character varying
END AS ORDINE
```

**Classification:** Presentation logic
**Placement:** Keep in backend SQL for as-is migration — same rationale as Fatture grouping.

---

### `intestazione_ordine` formatting (Ordini R&S)

**Original SQL (in GET_ordini_Ric_Spot query):**
```sql
((((o.nome_testata_ordine::text || ' del '::text) || to_char(o.data_documento::timestamp with time zone, 'YYYY-MM-DD'::text)) || ' ('::text) || o.stato_ordine::text) || ')'::text AS intestazione_ordine
```

**Classification:** Presentation logic
**Placement:** Keep in backend SQL for as-is migration.

---

### `descrizione_long` concatenation (Ordini R&S)

**Original SQL (in GET_ordini_Ric_Spot query):**
```sql
CASE
    WHEN r.descrizione_prodotto::text = r.descrizione_estesa::text OR r.descrizione_estesa IS NULL OR r.descrizione_estesa::text = ''::text THEN r.descrizione_prodotto::text
    ELSE ((r.descrizione_prodotto::text || chr(13)) || chr(10)) || r.descrizione_estesa::text
END AS descrizione_long
```

**Classification:** Presentation logic
**Placement:** Keep in backend SQL for as-is migration.

---

### `stato_riga` computation (Ordini R&S — CRITICAL)

**Original SQL (in GET_ordini_Ric_Spot query):**
```sql
CASE
    WHEN o.stato_ordine::text = 'Cessato'::text THEN 'Cessata'::text
    WHEN o.stato_ordine::text = 'Bloccato'::text THEN 'Bloccata'::text
    WHEN o.stato_ordine::text = 'Confermato'::text AND date_part('year'::text, r.data_attivazione) = 1::double precision THEN 'Da attivare'::text
    WHEN o.stato_ordine::text = 'Confermato'::text AND date_part('year'::text, r.data_attivazione) > 1::double precision THEN 'Attiva'::text
    WHEN r.annullato = 1 THEN 'Annullata'::text
    WHEN date_part('year'::text, r.data_cessazione) = 1::double precision THEN 'Attiva'::text
    WHEN r.data_cessazione >= '0001-01-01 00:00:00'::timestamp without time zone AND r.data_cessazione <= now() THEN 'Cessata'::text
    WHEN r.data_cessazione > now() THEN 'Cessazione richiesta'::text
    ELSE 'Unknown'::text
END AS stato_riga
```

**Classification:** **Business logic** — determines the semantic status of an order line
**Placement:** **Backend** — this is a core business rule. Must be computed server-side and returned as a field. Kept in SQL for as-is migration.

---

### `data_ordine` computation (Ordini R&S)

**Original SQL (in GET_ordini_Ric_Spot query):**
```sql
CASE
    WHEN o.data_conferma > o.data_documento THEN o.data_conferma
    ELSE o.data_documento
END AS data_ordine
```

**Classification:** Business logic — determines the effective order date
**Placement:** Backend (SQL). Kept as-is.

---

### Sentinel date handling (Ordini R&S)

**Original SQL (in GET_ordini_Ric_Spot query):**
```sql
NULLIF(r.data_attivazione, '0001-01-01 00:00:00'::timestamp without time zone) AS data_attivazione
NULLIF(r.data_disdetta, '0001-01-01 00:00:00'::timestamp without time zone) AS data_disdetta
NULLIF(r.data_cessazione, '0001-01-01 00:00:00'::timestamp without time zone) AS data_cessazione
NULLIF(r.data_ultima_fatt, '0001-01-01 00:00:00'::timestamp without time zone) AS data_ultima_fatt
NULLIF(r.data_fine_fatt, '0001-01-01 00:00:00'::timestamp without time zone) AS data_fine_fatt
NULLIF(r.data_scadenza_ordine, '0001-01-01 00:00:00'::timestamp without time zone) AS data_scadenza_ordine
```

**Classification:** Data normalization
**Placement:** Backend — all `0001-01-01` sentinel dates converted to NULL before reaching frontend. Kept in SQL.

---

### Chart data transformation (IaaS PPU)

**Original JSObject (`utils.aggiornaSerie`):** See full code at top of this document.

**Classification:** Presentation logic
**Placement:** Frontend — chart library data binding. Backend returns raw charge fields; frontend builds chart series from non-zero values.

---

### PBX totals aggregation (Timoo)

**Original JSObject (`utils.pbxStats`):** See full code at top of this document.

**Classification:** Data aggregation
**Placement:** Backend — return both individual rows and totals in a single response.

---

## Logic Placement Summary

### Backend responsibilities

| Responsibility | Details |
|---------------|---------|
| All database queries | Parameterized SQL against Mistra, Grappa, Anisetta — using original queries as-is with parameterization |
| `stato_riga` computation | 8-way CASE logic in SQL, preserved exactly |
| `data_ordine` computation | CASE in SQL, preserved exactly |
| Sentinel date normalization | NULLIF in SQL, preserved exactly |
| Document/order grouping | CASE expressions for `documento`, `ORDINE`, `intestazione_ordine`, `descrizione_long` — preserved in SQL |
| Customer filter safety | Parameterized IN clauses; customer always required on Ordini pages |
| PBX totals aggregation | Sum users/SE across PBX instances |
| Period filter | Accept period value; treat 0/null as "no limit" (replaces 2000-month hack) |
| Order history chain | Call `get_reverse_order_history_path()` (if kept — Q3) |
| Account/tenant exclusions | codice_aggancio_gest NOT IN (385,485); KlajdiandCo filter (Q9) |

### Frontend responsibilities

| Responsibility | Details |
|---------------|---------|
| Chart data binding | Map backend charge data to chart library series; filter zero-value types (IaaS PPU pie chart) |
| Auto-refresh triggers | Fatture: re-query on customer/period change |
| Currency formatting | EUR with 2 decimals |
| Date formatting | DD-MM-YYYY |
| Connection type "select all" default | All types selected by default on Accessi page load |
| Tab navigation | Account → daily/monthly → day breakdown (IaaS PPU) |
| Table features | Client-side search, sort, pagination, optional export |

### Rules being revised (not ported as-is)

| Current behavior | Change | Reason |
|-----------------|--------|--------|
| SQL injection via string concatenation | Parameterized queries | Security |
| Direct DB access from browser | Go backend API layer | Security |
| 2000-month hack for "all" | Backend accepts null/0 as "no limit" | Clean API |
| `getPlaceholder` REST API proxy | Dropped | TIMOO API excluded |
| `generaTenantIdList()` URL builder | Dropped | Direct DB query |
| `listaTenants()` REST orchestration | Dropped | Direct DB query |
| `Hello {{appsmith.user.name}}` greeting | Dropped | Not needed |
| Hardcoded `d_al` default date (2024-02-26) | N/A | Dashboard deferred |
