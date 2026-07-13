# @mrsmith/vite-config

Shared Vite configuration for MrSmith React mini-apps. Owns the React plugin,
the React dedupe list, the `/apps/<slug>/` production base-path convention, and
the standard `/api` + `/config` dev proxies.

## Usage

```ts
// apps/<app>/vite.config.ts
import { defineMrSmithAppConfig } from '@mrsmith/vite-config';

export default defineMrSmithAppConfig({ appSlug: 'my-app', port: 5197 });
```

Add `"@mrsmith/vite-config": "workspace:*"` to the app's `devDependencies`.
Pick a dev port that is not already taken (`grep -r "port:" apps/*/vite.config.ts`)
and wire it into `backend/cmd/server/main.go` hrefOverrides per the New App
Checklist in `CLAUDE.md`.

App-specific options go through `overrides`, which is deep-merged on top of the
shared defaults — never bypass the helper by going back to a raw `defineConfig`:

```ts
export default defineMrSmithAppConfig({
  appSlug: 'my-app',
  port: 5197,
  overrides: { server: { open: true } },
});
```

## Why React is deduped

Workspace packages (`@mrsmith/auth-client`, `@mrsmith/ui`, `@mrsmith/features`)
are served from TypeScript source in dev. Without `resolve.dedupe`, Vite can
resolve a second `react` module instance for them under pnpm's isolated
`node_modules` layout, and the app crashes at startup with React's
"Invalid hook call" / `Cannot read properties of null (reading 'useState')`
error (issue #49). Hooks require a single React *module instance*, not merely a
single React version. The shared packages also declare `react`/`react-dom` as
`peerDependencies` for the same reason; keep it that way for new shared React
packages.
