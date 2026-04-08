# Implementation Knowledge Handbook

This document is the canonical handbook for reusable implementation knowledge discovered while building apps in this repo.

Use it to capture facts that are easy to rediscover badly and expensive to relearn later: identifier mappings, cross-system joins, hidden business rules, exclusions, legacy quirks, API/DB mismatches, and operational conventions that affect future implementations.

## How to Use This Document

- Read the relevant sections before planning a new app, integration, or cross-system feature.
- Update this document in the same change set when implementation work uncovers reusable knowledge that other apps are likely to need.
- Prefer adding curated entries under a stable domain section instead of app-specific notes or a chronological dump.
- Keep each entry actionable: describe the fact, the practical rule it implies, the evidence, and where it matters.

## Entry Format

Use this format for new knowledge entries:

### Entry Title

- Context: where this knowledge applies
- Discovery: the fact that was verified
- Practical rule: how future implementations should use it
- Evidence: source tables, specs, code paths, or repo docs
- Used by: apps or domains already depending on it
- Open questions: only if unresolved details remain

## Domains

Add new discoveries under the most relevant domain:

- Cross-system identity and keys
- Customer eligibility and exclusion rules
- API and backend contract quirks
- Legacy data model constraints
- Auth and transport behavior
- Deployment and runtime integration rules

## Cross-System Identity and Keys

### Customer Identity Across Systems

- Context: customer lookup, filtering, and joins across Alyante, Mistra, and Grappa.
- Discovery: the same customer is represented with different keys across systems. Alyante ERP ID is the shared business identifier. In Mistra, `customers.customer.id` stores that ERP ID directly. In Grappa, `cli_fatturazione.codice_aggancio_gest` stores the ERP ID, while `cli_fatturazione.id` is a separate internal Grappa identifier.
- Practical rule: when moving from Grappa data to Mistra data, use `cli_fatturazione.codice_aggancio_gest -> customers.customer.id`. Do not assume `cli_fatturazione.id` matches Mistra customer IDs.
- Evidence: `customers.customer`, `loader.erp_clienti_provenienza`, `cli_fatturazione`; prior analysis captured in the legacy cross-db identity note.
- Used by: customer selectors and pricing/credit flows in `apps/listini-e-sconti`.
- Open questions: none on the identifier mapping itself.

#### Systems Involved

| System | Database | Main table | Primary key meaning |
| --- | --- | --- | --- |
| Alyante | — | — | ERP company ID |
| Mistra | PostgreSQL | `customers.customer` | `id` = Alyante ERP ID |
| Grappa | MySQL | `cli_fatturazione` | `id` = internal Grappa ID |

#### Key Mapping

```text
Alyante ERP ID
    |
    ├── Mistra PG:  customers.customer.id
    |
    └── Grappa MySQL: cli_fatturazione.codice_aggancio_gest
                      cli_fatturazione.id                    (internal Grappa ID)
```

#### ERP Bridge in Mistra

- Context: filtering customers eligible for billing-related flows.
- Discovery: `loader.erp_clienti_provenienza.numero_azienda` links back to `customers.customer.id`, and `fatgamma > 0` marks a customer as active for billing.
- Practical rule: when a flow needs ERP-linked or billing-eligible customers in Mistra, join through `loader.erp_clienti_provenienza` and treat `fatgamma > 0` as the current eligibility signal unless product requirements say otherwise.
- Evidence: `loader.erp_clienti_provenienza.numero_azienda`, `loader.erp_clienti_provenienza.fatgamma`.
- Used by: customer list variants described in `apps/listini-e-sconti/listini-e-sconti-migspec-phaseA.md`.
- Open questions: confirm with the domain team whether `fatgamma > 0` is the durable business rule or a current operational shortcut.

### HubSpot Company Lookup from Grappa

- Context: audit trail flows that create HubSpot notes/tasks after pricing, credit, or discount changes.
- Discovery: the Grappa customer ID must be resolved to a HubSpot company ID via a two-step cross-database lookup:
  1. Grappa → ERP ID: `SELECT codice_aggancio_gest FROM cli_fatturazione WHERE id = :grappa_id` (Grappa MySQL)
  2. ERP ID → HubSpot ID: `SELECT id FROM loader.hubs_company WHERE numero_azienda = :erp_id::varchar` (Mistra PG)
- Practical rule: backend services that need to write to HubSpot from a Grappa context must query both databases sequentially. Cache the mapping if performance is a concern — the mapping changes infrequently.
- Evidence: Appsmith `HS_utils` module method `CompanyByGrappaId`, queries `get_erp_id` and `get_hubspot_id_by_erp_code`.
- Used by: IaaS Prezzi risorse, IaaS Credito omaggio, Sconti variabile energia (all in `apps/listini-e-sconti`).
- Open questions: none.

## Customer Eligibility and Exclusion Rules

### Known Grappa Customer Exclusions

- Context: customer selectors used by IaaS pricing and credit pages.
- Discovery: some flows explicitly exclude specific `cli_fatturazione.codice_aggancio_gest` values.
- Practical rule: do not silently generalize active-billing customer selectors across pages; verify whether exclusion codes must be preserved for that use case.
- Evidence: current documented exclusions from existing migration analysis.
- Used by: IaaS Prezzi risorse, IaaS Credito omaggio.
- Open questions: whether these exclusions are permanent business rules or should become configurable.

| Code | Excluded in |
| --- | --- |
| `385` | IaaS Prezzi risorse, IaaS Credito omaggio |
| `485` | IaaS Credito omaggio |

## Legacy Data Model Constraints

### Alyante Product Translation Write Contract

- Context: server-side sync of product short descriptions from kit-products into Alyante ERP table `MG87_ARTDESC`.
- Discovery: the live Appsmith datasource query updates `MG87_DESCART` and filters with suffixed legacy column names: `MG87_DITTA_CG18`, `MG87_OPZIONE_MG5E`, `MG87_LINGUA_MG52`, `MG87_CODART_MG66`. Earlier backend assumptions using `MG87_DESCRIZIONE`, `MG87_DITTA`, `MG87_OPZIONE`, `MG87_LINGUA`, `MG87_CODART` do not match this environment.
- Practical rule: when writing product short descriptions to Alyante, use `UPDATE MG87_ARTDESC SET MG87_DESCART = ?` with `MG87_DITTA_CG18 = 1`, `MG87_OPZIONE_MG5E = '                    '`, `MG87_LINGUA_MG52 = 'ITA'/'ING'`, and `MG87_CODART_MG66 = code.padEnd(25, ' ')`.
- Evidence: verified Appsmith query `update MG87_ARTDESC set MG87_DESCART = {{this.params.descr}} where MG87_DITTA_CG18 = 1 and MG87_OPZIONE_MG5E = '                    ' and MG87_LINGUA_MG52 = {{this.params.lang}} AND MG87_CODART_MG66 = {{this.params.code}}`; backend adapter in `backend/internal/kitproducts/alyante.go`.
- Used by: `apps/kit-products` product translation sync.
- Open questions: none for this environment; if another Alyante tenant exposes different column names, verify its datasource query before generalizing.
