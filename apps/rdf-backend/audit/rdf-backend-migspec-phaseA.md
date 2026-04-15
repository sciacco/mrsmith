# RDF Backend — Phase A: Entity–Operation Model

Policy: **porting 1:1**. Nessuna modifica di comportamento, nessun campo nuovo, nessuna domanda al dominio.

## Entità: `Fornitore`

- **Tabella:** `public.rdf_fornitori` su datasource `anisetta` (Postgres `10.129.32.20:5432`).
- **Campi:**
  | Campo | Tipo | Vincoli |
  |---|---|---|
  | `id` | integer PK, DB-assigned | immutabile, non editabile da UI |
  | `nome` | string | required (non vuoto) |
- **Relazioni:** nessuna.

## Operazioni

| Op | Input | Semantica (identica all'Appsmith) |
|---|---|---|
| List | `search`, `sortColumn`, `sortOrder`, `page`, `pageSize` | `WHERE nome ILIKE '%search%'`, `ORDER BY <col> <order>` (default `id ASC`), `LIMIT/OFFSET` |
| Create | `{ nome }` | INSERT singola riga, `id` DB-assegnato |
| Update | `id`, `nome` | UPDATE singola riga per `id` |
| Delete | `id` | DELETE singola riga per `id` |

## Fix minimi (non sono modifiche di comportamento, sono bug della baseline)
1. `totalRecordsCount` deve essere restituito dal backend (Appsmith lo lasciava a 0 rompendo la paginazione).
2. Errore di `DeleteQuery` non più silenzioso: toast come per insert/update.
3. Colonne sortabili whitelisted (`id`, `nome`) per evitare SQL injection — il contratto esterno resta identico.
