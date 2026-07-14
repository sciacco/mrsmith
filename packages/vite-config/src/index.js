// Plain JavaScript on purpose: Vite loads vite.config.ts by bundling it with
// esbuild but EXTERNALIZES bare imports like this package, which Node then
// loads at runtime. The Docker build runs node:20-slim, which cannot load .ts
// modules (ERR_UNKNOWN_FILE_EXTENSION), so this package must ship runnable JS.
import react from '@vitejs/plugin-react';
import { defineConfig, mergeConfig } from 'vite';

/**
 * @typedef {Object} MrSmithAppConfigOptions
 * @property {string} [appSlug] App slug for the production base path `/apps/<slug>/`. Omit for apps served at the root (portal).
 * @property {number} [port] Unique dev-server port (see the New App Checklist). Omit to use Vite's default.
 * @property {string} [backendTarget] Overrides the `/api` + `/config` proxy target; defaults to VITE_DEV_BACKEND_URL or localhost:8080.
 * @property {import('vite').UserConfig} [overrides] App-specific Vite options, deep-merged on top of the shared defaults.
 */

/**
 * Shared Vite config for MrSmith React mini-apps.
 *
 * React (and its JSX runtimes) is deduped because workspace packages such as
 * @mrsmith/auth-client and @mrsmith/ui are served from source in dev: without
 * dedupe, Vite can load a second React instance for them and hooks crash with
 * "Invalid hook call" at startup (issue #49).
 *
 * @param {MrSmithAppConfigOptions} [options]
 */
export function defineMrSmithAppConfig(options = {}) {
  const backendTarget =
    options.backendTarget ?? process.env.VITE_DEV_BACKEND_URL ?? 'http://localhost:8080';
  return defineConfig(({ command }) =>
    mergeConfig(
      {
        base: command === 'build' && options.appSlug ? `/apps/${options.appSlug}/` : '/',
        plugins: [react()],
        resolve: {
          dedupe: ['react', 'react-dom', 'react/jsx-runtime', 'react/jsx-dev-runtime'],
        },
        server: {
          ...(options.port !== undefined ? { port: options.port } : {}),
          proxy: {
            '/api': backendTarget,
            '/config': backendTarget,
          },
        },
      },
      options.overrides ?? {},
    ),
  );
}
