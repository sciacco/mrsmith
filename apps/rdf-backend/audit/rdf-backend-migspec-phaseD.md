# RDF Backend — Phase D: Integration & Data Flow

## Sistemi esterni
- **Postgres `anisetta`** (`10.129.32.20:5432`). Unico backing store. DSN e credenziali da `backend/internal/platform/config/config.go` (nuovo campo `AnisettaDSN` + env var).
- **Keycloak** (già integrato nel portal): ruolo `app_rdf_access` per l'accesso.

## User journeys
1. Login portal → sidebar → "RDF Backend" → `/fornitori` → `GET /api/rdf/fornitori` on mount.
2. Digita in search → debounce → `GET` con `search=…`.
3. Click header colonna → `GET` con `sort=…&order=…`.
4. Paginazione → `GET` con `page=…&pageSize=…`.
5. Click `+` → modal → submit → `POST` → re-fetch → chiudi modal.
6. Click riga → form inline visibile → edit `nome` → submit → `PATCH /:id` → re-fetch.
7. Click "Delete" sulla riga → modal di conferma → `DELETE /:id` → re-fetch → chiudi modal.

## Automazioni / timer / trigger nascosti
- Nessuno. L'export non contiene job, cron, webhook, o azioni schedulate.

## Confini di ownership
- Mini-app è single-owner: scrive solo su `public.rdf_fornitori`. Nessun altro servizio o app condivide la tabella secondo l'export.
