# Authentication Enablement For Portal And Exposed Apps

## Summary
- Current auth foundation exists: the shared browser client is in [packages/auth-client/src/AuthProvider.tsx](../packages/auth-client/src/AuthProvider.tsx), and the backend already validates bearer tokens on `/api/*` plus exposes `/config` for frontend bootstrap in [backend/cmd/server/main.go](../backend/cmd/server/main.go) and [backend/internal/auth/middleware.go](../backend/internal/auth/middleware.go).
- One app already uses that pattern: budget bootstraps auth from `/config` and attaches the bearer token to API calls. The portal does not: [apps/portal/src/main.tsx](../apps/portal/src/main.tsx) is still static, and [backend/internal/portal/handler.go](../backend/internal/portal/handler.go) still returns an empty app list.
- Authorization is not implemented yet. `acl.RequireRole` exists but is unused, so the system currently authenticates requests but does not decide which apps a user may see or use.
- Deployment config is incomplete for browser login. [deploy/k8s/configmap.yaml](../deploy/k8s/configmap.yaml) sets `KEYCLOAK_ISSUER_URL` only; it does not set `KEYCLOAK_FRONTEND_URL`, `KEYCLOAK_FRONTEND_REALM`, or `KEYCLOAK_FRONTEND_CLIENT_ID`.
- Validation status today: `go test ./...` passed, `pnpm --filter mrsmith-portal lint` passed, and `pnpm --filter mrsmith-budget lint` currently fails on unrelated TypeScript issues in `apps/budget/src/utils/format.ts` and `apps/budget/src/views/voci-di-costo/BudgetListPage.tsx`.

## Implementation Changes
- Portal login gate:
  Add the same `/config` bootstrap flow used by budget to the portal, wrap the portal app in `AuthProvider`, and add `/config` proxying in `apps/portal/vite.config.ts`. The portal should not render its launcher UI until auth bootstrap completes.
- Portal user and app data:
  Replace the hardcoded seed-driven portal home with backend-driven data. Keep `/api/portal/me` as the identity endpoint, and change `/api/portal/apps` to return filtered launcher data in the frontend’s category/app shape.
- Entitlement model:
  Use Keycloak roles as the source of truth. Introduce a backend app catalog with stable app IDs, display metadata, launch URL, category, and required roles. Default role convention: `app_<app_id>_access` (first enforced app: `app_budget_access`).
- Access behavior:
  Portal access requires successful authentication only. App visibility is role-filtered. If an authenticated user has no app roles, the portal still loads and shows an empty-state launcher.
- Backend enforcement:
  Reuse the same role catalog to protect exposed app APIs with `acl.RequireRole(...)`. Start with budget routes so hidden apps are also blocked server-side, not just omitted from the portal UI.
- Config and deployment:
  Add frontend Keycloak env vars to local and Kubernetes config, and register redirect URIs/web origins in Keycloak for each deployed frontend. Keep one public client per deployed frontend bundle.
- Hosting constraint to account for:
  The production image now serves portal at `/` and budget at `/apps/budget/` from the same backend/static root. Future mini-apps should follow the same `/apps/<app_id>/` hosting pattern rather than introducing separate deployment origins.

## Test Plan
- Backend tests for `/api/portal/apps` filtering by role and for guarded app routes returning `401` without a token and `403` with the wrong role.
- Portal acceptance checks:
  unauthenticated visit triggers Keycloak login, authenticated visit loads the current user name from `/api/portal/me`, and a user with only `app_budget_access` sees only the Budget card.
- App acceptance checks:
  visible apps load normally with bearer-authenticated API calls, and direct calls to hidden app APIs are rejected.
- Deployment checks:
  `/config` returns non-empty frontend Keycloak values in each environment, and the configured Keycloak client redirect URIs match the actual portal/app origins.

## Assumptions
- This milestone is `login + app filtering`, not full feature-level RBAC inside every app.
- Keycloak realm roles are the entitlement source; no external entitlement service is introduced in this step.
- Portal does not get its own dedicated access role by default; any authenticated user may enter, but only role-entitled apps are shown.
- Each frontend remains deployment-scoped for auth bootstrap; a shared multi-app static gateway is out of scope for this step.
