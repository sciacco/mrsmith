# `@mrsmith/auth-client`

Shared Keycloak / OIDC auth provider for all mini-apps in the monorepo.

## Normal usage

```tsx
import { AuthProvider, useAuth } from '@mrsmith/auth-client';

<AuthProvider keycloakUrl={...} realm={...} clientId={...}>
  <App />
</AuthProvider>
```

`useAuth()` returns `{ status, authenticated, token, user, login, logout, getAccessToken, forceRefreshToken, loading }`.

## Dev bypass (skip Keycloak locally)

When you need to iterate on a UI without going through the Keycloak login, set `VITE_DEV_AUTH_BYPASS=true` in your app's `.env.local`:

```sh
# apps/<your-app>/.env.local
VITE_DEV_AUTH_BYPASS=true
```

With that flag active:

- `AuthProvider` does **not** instantiate `keycloak-js` — the Keycloak SDK still ships in the dev bundle (tree-shaken in `vite build`) but no network call is made.
- A fake user is injected immediately: `status='authenticated'`, `token='dev-token'`, `user={ name, email, roles }`.
- `login()` / `logout()` become no-ops (and log a warning).
- `getAccessToken()` / `forceRefreshToken()` return `'dev-token'` synchronously.

### Default fake roles

Without further configuration the fake user gets `[app_devadmin]`. Because `hasAnyRole()` short-circuits to `true` for that role, every route guard / `hasAnyRole` check passes. This is what you want 99% of the time when you're iterating on the UI.

### Scoped fake roles (simulate a limited user)

To verify that a page is correctly hidden / disabled for a user without a specific role, override the roles list via `VITE_DEV_FAKE_ROLES`:

```sh
# simulate a user that only has read access to manutenzioni
VITE_DEV_FAKE_ROLES=app_manutenzioni_access

# simulate a manager (without approver)
VITE_DEV_FAKE_ROLES=app_manutenzioni_access,app_manutenzioni_manager
```

You can also override identity fields: `VITE_DEV_FAKE_NAME`, `VITE_DEV_FAKE_EMAIL`.

### Backend side

The frontend bypass is **not enough on its own** — the Go backend still verifies the bearer token. Start the backend with `SKIP_KEYCLOAK=true` so its `auth` middleware skips OIDC verification and injects a matching fake user (see `backend/internal/auth/middleware.go`). The fake backend user defaults to every known `app_*` role from the catalog; override with `DEV_FAKE_ROLES=...`.

A typical local setup:

```sh
# apps/manutenzioni/.env.local
VITE_DEV_AUTH_BYPASS=true

# shell that runs the backend
SKIP_KEYCLOAK=true make dev
```

### Production safety

`VITE_DEV_AUTH_BYPASS` is a Vite environment variable: it gets inlined at build time. If the env var isn't set during `vite build` (the case in CI / production deploys), the runtime check `import.meta.env.VITE_DEV_AUTH_BYPASS === 'true'` evaluates against `undefined` and the bypass path is never taken. `.env.local` files are git-ignored so they cannot leak into a committed build.
