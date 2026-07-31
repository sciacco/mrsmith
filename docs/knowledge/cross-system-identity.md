# Cross-System Identity and Keys

Identity mappings, cross-system keys, eligibility exclusions, and cross-database join rules shared by every app.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

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
| Mistra | PostgreSQL | `loader.hubs_company` | `numero_azienda` = Alyante ERP ID (varchar) |
| Grappa | MySQL | `cli_fatturazione` | `id` = internal Grappa ID |

#### Key Mapping

```text
Alyante ERP ID
    |
    ├── Mistra PG:  customers.customer.id
    |
    ├── Mistra PG:  loader.hubs_company.numero_azienda     (HubSpot mirror; varchar)
    |
    └── Grappa MySQL: cli_fatturazione.codice_aggancio_gest
                      cli_fatturazione.id                    (internal Grappa ID)
```

`loader.hubs_company.numero_azienda` carries the Alyante ERP ID on the HubSpot mirror side. When a flow starts from a HubSpot deal or company (e.g. quote→order conversion in `backend/internal/quotes/order_conversion.go`, or the `HubSpot Company Lookup from Grappa` recipe below), reading `numero_azienda` avoids a round-trip to Alyante MSSQL for the same value. Treat it as the same identifier — stored as `varchar` rather than `int` — and cast to the integer form when joining against `customers.customer.id`. Evidence: Appsmith package `Ordini gestione portale` (`gpUtils` module) which reads `loader.hubs_company.numero_azienda as customer_number` in `get_quote_by_id`, mirrored by the Go port `loadQuoteOrderSource`.

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

### Cross-Database Mini-App Summaries Must Merge In Code, Not In One SQL Join

- Context: mini-apps that read business records from one DB and enrich them with replica/loader data from another DB in the same request path.
- Discovery: the MrSmith backend wires `ANISETTA_DSN`, `MISTRA_DSN`, `GRAPPA_DSN`, and other stores as separate `*sql.DB` handles. A handler cannot issue a single SQL statement that joins tables across those DSN boundaries. `apps/richieste-fattibilita` hit this when `rdf_richieste` (Anisetta) needed HubSpot deal/company enrichment from `loader.hubs_*` (Mistra).
- Practical rule: when a screen needs cross-DB enrichment, fetch the base rows from the owning DB, batch-load enrichment rows from the secondary DB, merge in Go, and only then apply filters that depend on enriched fields (for example customer/company name filters). Do not plan a “server-side join” as a single SQL query unless the data is confirmed to live behind the same connection.
- Evidence: separate DB wiring in `backend/cmd/server/main.go`; merge implementation in `backend/internal/rdf/handler.go` for `GET /rdf/v1/richieste/summary`.
- Used by: `apps/richieste-fattibilita`.
- Open questions: none.

### HubSpot Deal Codes Match ERP Orders After Separator Normalization

- Context: Reports AOV detail and any flow matching ERP order numbers to HubSpot deals.
- Discovery: ERP order codes conventionally use `XXXXXXX-YYYY`, while `loader.hubs_deal.codice` can store the corresponding deal as `XXXXXXX/YYYY`.
- Practical rule: match order codes to deal codes with whitespace trimming and `/` -> `-` normalization on both sides. When displaying the matched deal, keep the original deal code from `loader.hubs_deal.codice` and pair it with `loader.hubs_deal.name`.
- Evidence: `loader.v_ordini_ric_spot.nome_testata_ordine`, `loader.hubs_deal.codice`, `loader.hubs_deal.name`, Reports AOV detail implementation.
- Used by: `apps/reports` AOV detail.
- Open questions: none.
