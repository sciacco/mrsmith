# Auth and Transport Behavior

Keycloak, role gating, and shared SPA client behavior common to all mini-apps.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Keycloak Role User Lookups Must Include Group-Derived Membership

- Context: backend services that need the current enabled users for a Keycloak realm role.
- Discovery: `GET /admin/realms/{realm}/roles/{role-name}/users` returns users assigned directly to the role, but does not cover users who inherit the role through Keycloak groups.
- Practical rule: use `backend/internal/platform/keycloak.Client.UsersByRealmRole` for backend role-user resolution. It combines direct role users with groups that carry the role, recursively visits subgroup children, pages group members, excludes disabled users and blank emails, and deduplicates by Keycloak user ID. V1 intentionally does not expand composite roles.
- Evidence: Keycloak Admin REST role users/groups and group members endpoints; Keycloak issue `keycloak/keycloak#19391` documents the direct lookup limitation for group-inherited users.
- Used by: future backend-only role recipient or operator resolution flows.
- Open questions: none for realm roles; client-role and composite-role expansion would need separate explicit design.

### Devadmin Must Be Centralized as a Superuser Override

- Context: Keycloak-role authorization across launcher visibility, backend ACL middleware, and app-specific elevated permissions.
- Discovery: role checks implemented independently (`acl.RequireRole`, launcher catalog filtering, and direct role checks like quotes delete) drift unless they share a single superuser rule.
- Practical rule: implement `app_devadmin` as a centralized override in shared authz helpers and consume those helpers everywhere role checks are performed (backend ACL, portal catalog filters, app-specific elevated checks, and frontend role-gated controls). Avoid raw `includes`/`slices.Contains` role checks in feature code.
- Evidence: `backend/internal/authz/authz.go`, `backend/internal/acl/acl.go`, `backend/internal/platform/applaunch/catalog.go`, `backend/internal/quotes/handler_quotes.go`, `packages/auth-client/src/roles.ts`, `apps/quotes/src/components/QuoteTable.tsx`.
- Used by: portal app visibility, all ACL-protected backend app routes, quotes delete authorization.
- Open questions: none.

### Shared SPA Clients Must Not Hit Protected APIs Before a Bearer Token Exists

- Context: frontend mini-apps using `@mrsmith/auth-client` plus `@mrsmith/api-client` for Keycloak-protected `/api/*` requests.
- Discovery: if the shared API client sends a request when `getAccessToken()` returns `undefined`, the backend logs a noisy `401 missing_bearer`, then a forced refresh-and-retry can immediately succeed with `200`. When app wrappers also call `login()` from request-level 401 handlers, that pattern can escalate into visible remount/refetch loops.
- Practical rule: shared API clients must acquire a bearer token before the first network request, treat "no token available" as a local unauthorized error, and reserve backend retries for true stale-token 401s. Reauthentication should be driven centrally by `AuthProvider` refresh failure handling, not per-app query error callbacks.
- Evidence: `packages/api-client/src/client.ts`, `packages/auth-client/src/AuthProvider.tsx`, and the 2026-04-17 `apps/richieste-fattibilita` loop on `GET /api/rdf/v1/richieste/summary` alternating `401 missing_bearer` and `200`.
- Used by: portal and all mini-apps using the shared API/auth client stack.
- Open questions: none.

### Keycloak Initialization Must Be Idempotent In AuthProvider

- Context: frontend apps wrapped in React `StrictMode` while using `@mrsmith/auth-client`.
- Discovery: React 18 development mode can rerun effects, and calling `keycloak.init()` more than once on the same Keycloak instance can produce a transient failed bootstrap that surfaces as `unauthenticated` and shows the mini-app "Accesso richiesto" page.
- Practical rule: `AuthProvider` must start Keycloak initialization once per provider instance and let repeated effect executions subscribe to the same init promise. Stale effect results must not overwrite a newer/authenticated state.
- Evidence: `packages/auth-client/src/AuthProvider.tsx`; observed mini-app entry flicker after adding root role gates.
- Used by: all portal mini-apps and the portal launcher in development and production builds.
- Open questions: none.

### Mini-App Auth Fallbacks Must Fail Closed and Retry Local Preflight Unauthorized Errors

- Context: Vite mini-app bootstraps using `useOptionalAuth()`, app-shell auth gates, and React Query startup fetches.
- Discovery: if an app-local auth fallback reports `authenticated: true` without a token, routed pages mount before Keycloak state is usable. Once the shared API client correctly refuses to send bearerless requests, those startup fetches fail locally with no backend logs; if React Query also disables retries for every `401`, the page can freeze in a false "not authorized" state.
- Practical rule: optional-auth fallbacks must default to `unauthenticated`, mini-app shells must gate route rendering on `authenticated` rather than `loading` alone, and query retry policies must keep retry disabled for real backend ACL failures while allowing retries for local auth-preflight `401`s.
- Evidence: `apps/*/src/hooks/useOptionalAuth.ts`, `apps/*/src/App.tsx`, `apps/*/src/main.tsx`, `apps/richieste-fattibilita/src/lib/format.ts`, and the 2026-04-17 first-load `richieste-fattibilita` empty-state error with no matching backend request.
- Used by: all mini-apps consuming `@mrsmith/auth-client` and `@mrsmith/api-client`.
- Open questions: none.

### Mini-App Shells Must Gate Routes With Launcher Roles

- Context: direct browser entry to `/apps/<app>/` while authenticated in Keycloak but missing that app's launcher role.
- Discovery: launcher visibility and backend ACL were already role-gated, but mini-app roots rendered their nav and route trees after checking only authentication. This let unassigned users see app workspaces, empty tables, or data-error states before backend ACL stopped data access.
- Practical rule: every mini-app root must use `getAppAccessState()` with `APP_ACCESS_ROLES` before rendering app nav or calling `useRoutes`. Non-allowed states should render only the shared shell header and `AccessNotice`; `app_devadmin` remains the central bypass through `hasAnyRole()`.
- Evidence: `packages/auth-client/src/roles.ts`, `packages/ui/src/components/AccessNotice/AccessNotice.tsx`, and `apps/*/src/App.tsx`.
- Used by: all Vite mini-apps mounted under `/apps/<app>/`.
- Open questions: none.

### Route-Scoped Roles Share the App Launcher Role Set

- Context: a user should open an existing mini-app but only operate one route/tab inside it.
- Discovery: a single `APP_ACCESS_ROLES[app]` value is both the mini-app shell gate and the frontend mirror of launcher visibility, while backend ACL is the real authorization boundary for each API route.
- Practical rule: put every role that may enter the app in the launcher/app access union, then enforce narrower route/API permissions separately. The frontend should hide or redirect unavailable tabs for UX, but backend handlers must use the precise role helper for each operation. Do not treat a broad app role as implicitly allowed on a route-scoped workflow when product has split that workflow into a dedicated role; users who need both surfaces should receive both roles.
- Evidence: CP Backoffice full role `app_cpbackoffice_access`, biometric-only role `app_cpbackoffice_biometric_access`, `backend/internal/cpbackoffice/handler.go`, `backend/internal/platform/applaunch/catalog.go`, `packages/auth-client/src/roles.ts`, and `apps/cp-backoffice/src/App.tsx`.
- Used by: CP Backoffice biometric-only access.
- Open questions: none.
