# API Conventions

This document captures the frontend/backend communication conventions currently used in the MrSmith repo and the recommended default for new mini-apps.

It is intentionally based on the running codebase, not on a generic REST style guide. Where the repo has legacy or app-specific exceptions, those are called out explicitly.

Related references:

- [docs/IMPLEMENTATION-PLANNING.md](IMPLEMENTATION-PLANNING.md)
- [docs/IMPLEMENTATION-KNOWLEDGE.md](IMPLEMENTATION-KNOWLEDGE.md)
- [docs/UI-UX.md](UI-UX.md)

## Summary

The stable repo convention is:

1. Frontend bootstraps auth from `GET /config`.
2. Frontend talks to same-origin `/api`, never directly to Mistra, Grappa, HubSpot, Arak, or other upstream systems.
3. Each mini-app owns a backend namespace, usually `/api/<app-prefix>/v1/...`.
4. The Go backend acts as the BFF/proxy/orchestration layer.
5. JSON is the default transport; file downloads use blob responses.

What is not standardized repo-wide is a single universal response envelope. Some endpoints return arrays, some `{ items: [...] }`, some paged objects, and some mirror upstream contracts on purpose.

## Runtime Contract

### Production Hosting

- Each SPA is built under `/apps/<app-slug>/`.
- API traffic remains same-origin under `/api/...`.
- Auth bootstrap remains same-origin under `/config`.

Practical rule: a mini-app can be deployed under its own static app path without changing its API base URL.

### Local Development

- Each Vite app runs on its own port.
- Each Vite app proxies both `/api` and `/config` to the Go backend, usually `http://localhost:8080`.
- Some Vite configs allow overriding the backend target with `VITE_DEV_BACKEND_URL`.

Practical rule: if a new mini-app uses the shared auth/client setup, it should proxy both `/api` and `/config`, not only `/api`.

## Frontend Conventions

### Auth Bootstrap

- Frontend startup fetches `GET /config`.
- The response supplies Keycloak frontend config such as `keycloakUrl`, `realm`, and `clientId`.
- Apps pass that config into `@mrsmith/auth-client`'s `AuthProvider`.

Practical rule: auth bootstrap is part of the frontend/backend contract for mini-apps, even though it is not under `/api`.

### Shared API Client

- Frontends use `@mrsmith/api-client`.
- The shared client is created with `baseUrl: '/api'`.
- The client attaches `Authorization: Bearer <token>`.
- On `401`, it refreshes the token once and retries once.
- For file endpoints, use `getBlob()` or `postBlob()` instead of plain links when auth is required.

Practical rule: new mini-apps should not hand-roll fetch wrappers unless they have a concrete need that the shared client cannot cover.

### Browser-to-Backend Boundary

- Browsers should call only repo-owned endpoints under `/api/...`.
- External systems are integrated server-side.
- If an endpoint exists only to proxy an upstream API, that proxy still belongs in the Go backend.

Practical rule: avoid direct browser calls to internal APIs, DB gateways, or vendor APIs, even if the original legacy app did that implicitly.

## Backend Conventions

### Global Mount Shape

- Backend modules register app routes on a shared API mux.
- The server mounts that mux under `/api/`.
- Shared middleware applies authentication, request logging, recovery, CORS, and request IDs at the `/api` boundary.

Practical rule: mini-app routes should be defined without the `/api` prefix in module code, then mounted centrally by the server.

### Namespacing

Preferred default for new mini-apps:

- `/api/<app-prefix>/v1/...`

Examples:

- `/api/quotes/v1/quotes`
- `/api/reports/v1/orders/preview`
- `/api/kit-products/v1/kit`

Practical rule: the app namespace is the stability boundary. Do not put a new mini-app's routes directly at `/api/<resource>` with no app prefix.

### Role Protection

- All `/api/*` routes are behind Bearer-auth middleware.
- Individual modules usually apply app-specific Keycloak role checks on top of that.
- Some apps also define elevated manager-only endpoints inside the same namespace.

Practical rule: global auth is not enough. Each mini-app should still protect its own routes with app-specific role middleware.

## Data and Response Conventions

### JSON by Default

- Standard read/write endpoints return JSON.
- `Content-Type: application/json` is the default for handler responses.
- The shared client sends JSON for `POST`, `PUT`, and `PATCH`.
- If no body is provided for those methods, the shared client sends `{}` so handlers still receive valid JSON.

### File/Export Endpoints

- File endpoints are allowed when needed.
- Common cases include PDF, CSV, and XLSX exports.
- These endpoints typically return blobs with `Content-Disposition` headers.

Practical rule: when an authenticated flow downloads a file, expose a backend endpoint and consume it with `getBlob()` or `postBlob()`.

### Response Shapes

There is no single mandatory repo-wide envelope.

Current patterns include:

- raw arrays
- `{ items: [...] }`
- paged payloads such as `{ items, page, pageSize, total }`
- mutation acknowledgements such as `{ ok: true }`
- upstream-shaped payloads preserved intentionally

Practical rule: define the response shape explicitly per endpoint and keep it stable. Do not assume every API in the repo returns `{ data: ... }`.

### Error Shapes

Handler-level errors usually return JSON:

```json
{ "error": "some_error_code" }
```

Common behavior:

- `500` responses are sanitized to `internal_server_error`.
- Backend logs keep the real internal error.
- Auth middleware and ACL middleware may still return plain-text `401` or `403` responses before JSON helpers run.

Practical rule: frontend error handling must tolerate both structured JSON API errors and plain-text auth/permission failures.

## Current Mini-App Namespace Map

This is the current route-prefix map registered in the backend.

| App | Frontend App Slug | API Namespace | Notes |
| --- | --- | --- | --- |
| Portal | `portal` | `/api/portal/...` | Host app APIs, not a mini-app namespace standard. |
| Budget | `budget` | `/api/budget/v1/...` | Also exposes legacy upstream-shaped `/api/users-int/v1/user`. |
| Compliance | `compliance` | `/api/compliance/...` | Current exception: unversioned namespace. |
| Coperture | `coperture` | `/api/coperture/v1/...` | Standard app-scoped versioned namespace. |
| Energia DC | `energia-dc` | `/api/energia-dc/v1/...` | Standard app-scoped versioned namespace. |
| Kit Products | `kit-products` | `/api/kit-products/v1/...` | Standard app-scoped versioned namespace. |
| Listini e Sconti | `listini-e-sconti` | `/api/listini/v1/...` | App slug and API prefix differ intentionally. |
| Panoramica Cliente | `panoramica-cliente` | `/api/panoramica/v1/...` | App slug and API prefix differ intentionally. |
| Quotes | `quotes` | `/api/quotes/v1/...` | Standard app-scoped versioned namespace. |
| Richieste Fattibilita | `richieste-fattibilita` | `/api/rdf/v1/...` | App slug and API prefix differ intentionally. |
| RDF Backend | `rdf-backend` | `/api/rdf-backend/v1/...` | Separate admin/maintenance app for RDF suppliers CRUD. |
| Reports | `reports` | `/api/reports/v1/...` | Standard app-scoped versioned namespace. |
| Simulatori Vendita | `simulatori-vendita` | `/api/simulatori-vendita/v1/...` | Currently used for authenticated PDF generation. |
| AFC Tools | `afc-tools` | `/api/afc-tools/v1/...` | Standard app-scoped versioned namespace. |

## Known Exceptions and Repo Realities

### Not Every App Uses the Folder Slug as the API Prefix

Examples:

- `apps/listini-e-sconti` uses `/api/listini/v1/...`
- `apps/panoramica-cliente` uses `/api/panoramica/v1/...`
- `apps/richieste-fattibilita` uses `/api/rdf/v1/...`

Practical rule: do not infer the API prefix from the frontend folder name without checking backend registration.

### Not Every App Is Versioned Yet

- `compliance` currently uses `/api/compliance/...` with no `/v1`.

Practical rule: for new work, prefer versioned namespaces even if some older modules are still unversioned.

### Some Contracts Preserve Upstream Shapes

- `budget` intentionally keeps parts of the Arak/upstream shape such as `/users-int/v1/user`.
- Some payloads and query parameters remain upstream-flavored where preserving behavior is more important than repo-wide renaming.

Practical rule: do not "normalize" an upstream-shaped contract unless the migration or product decision explicitly requires it.

## Recommended Default for New Mini-Apps

When adding a new mini-app, use this template unless there is a verified reason not to:

1. Host the SPA at `/apps/<app-slug>/`.
2. Fetch auth bootstrap from `GET /config`.
3. Use `@mrsmith/auth-client` for Keycloak session management.
4. Use `@mrsmith/api-client` with `baseUrl: '/api'`.
5. Proxy both `/api` and `/config` in Vite local dev.
6. Register backend routes under `/api/<app-prefix>/v1/...`.
7. Protect the namespace with app-specific Keycloak roles.
8. Keep browser calls inside repo-owned backend endpoints only.
9. Return JSON by default; use blob endpoints for authenticated downloads/exports.
10. Define response shapes explicitly per endpoint instead of inventing a fake universal envelope.

## Verification Checklist

Before approving a new mini-app API contract, verify:

- The frontend uses `/config` for auth bootstrap.
- The frontend uses same-origin `/api`, not direct upstream URLs.
- Vite proxies both `/api` and `/config`.
- The backend namespace is app-scoped.
- Role protection is applied inside the module.
- Export/download flows use auth-capable blob requests.
- Response shapes are explicit and documented.
- Any intentional upstream-compatible exceptions are called out in the spec or plan.

## Code References

The current conventions described above are implemented in:

- `backend/cmd/server/main.go`
- `backend/internal/auth/middleware.go`
- `backend/internal/platform/httputil/respond.go`
- `packages/api-client/src/client.ts`
- `packages/auth-client/src/AuthProvider.tsx`
- `apps/*/vite.config.ts`
- `apps/*/src/api/client.ts`

