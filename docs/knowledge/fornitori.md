# Fornitori

Knowledge entries specific to `apps/fornitori` (provider contacts are also consumed by RDA).
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Fornitori Provider Contacts Follow Appsmith Payload Semantics

- Context: `apps/fornitori` provider detail contacts, `apps/rda` PO recipient contacts, and backend `POST/PUT /fornitori/v1/provider/{id}/reference` plus `POST/PUT /rda/v1/providers/{id}/references`.
- Discovery: Appsmith does not send empty `first_name`, `last_name`, or `email` fields for provider contacts. On reference create, empty `phone` must be omitted because Mistra `provider_ref_new` inserts it directly and the `provider_ref` phone check accepts `NULL` but not `''`; on reference edit, empty `phone` must be sent as `''` because `provider_ref_edit` converts it with `NULLIF` and uses key presence to clear the value. `QUALIFICATION_REF` is also a special reference type: Mistra's standard provider-reference functions reject creating, editing, or deleting it.
- Practical rule: Fornitori and RDA contact forms should omit empty name/email fields. The backend provider-reference proxies must omit empty `phone` on `POST`, include `phone` on `PUT` even when empty, be used **only** for non-qualification reference types (`ADMINISTRATIVE_REF`, `TECHNICAL_REF`, `OTHER_REF`), and reject `QUALIFICATION_REF` with a clear error. The QUALIFICATION_REF contact is owned by Mistra and is created/edited via `PUT /provider/{id}` (the `ref` field of `provider-edit`); never write to `provider_qualifications.provider_ref` directly from the portal backend.
- Evidence: Appsmith contact-save snippet provided during the Fornitori migration; `docs/mistra-dist.yaml` provider-ref schemas; `docs/arak_schema.json` functions `provider_ref_new` and `provider_ref_edit`.
- Used by: `apps/fornitori` detail page contacts and `apps/rda` PO recipient contacts.
- Open questions: none.
