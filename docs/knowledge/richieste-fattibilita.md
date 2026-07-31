# Richieste Fattibilità

Knowledge entries specific to `apps/richieste-fattibilita`.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### RDF `fornitori_preferiti` Must Be Treated as Nullable Text

- Context: `GET /api/rdf/v1/richieste/summary`, `GET /api/rdf/v1/richieste/{id}/full`, and any RDF flow that reads `public.rdf_richieste.fornitori_preferiti`.
- Discovery: the authoritative schema snapshot marks `rdf_richieste.fornitori_preferiti` as nullable `text` with default `''`, so production rows can legitimately contain `NULL`. Scanning that column directly into Go `string` fields causes runtime failures (`converting NULL to string is unsupported`).
- Practical rule: scan `fornitori_preferiti` with `sql.NullString` in RDF handlers and normalize `NULL`, `''`, and empty array literals to `[]` before encoding JSON. Keep the API contract as `number[]`; do not surface `null` to the frontend for this field.
- Evidence: `docs/anisetta_schema.json` (`rdf_richieste.fornitori_preferiti` `nullable: true`), backend failure `list_richieste_summary_scan` on 2026-04-16, and fixes in `backend/internal/rdf/handler.go`.
- Used by: `apps/richieste-fattibilita` summary/detail flows and manager actions that reload a richiesta after writes.
- Open questions: none.
