import react from '@vitejs/plugin-react';
import { defineConfig, mergeConfig, type UserConfig } from 'vite';

export interface MrSmithAppConfigOptions {
  /**
   * App slug for the production base path `/apps/<slug>/`.
   * Omit for apps served at the root (portal).
   */
  appSlug?: string;
  /** Unique dev-server port (see the New App Checklist). Omit to use Vite's default. */
  port?: number;
  /** Overrides the `/api` + `/config` proxy target; defaults to VITE_DEV_BACKEND_URL or localhost:8080. */
  backendTarget?: string;
  /** App-specific Vite options, deep-merged on top of the shared defaults. */
  overrides?: UserConfig;
}

/**
 * Shared Vite config for MrSmith React mini-apps.
 *
 * React (and its JSX runtimes) is deduped because workspace packages such as
 * @mrsmith/auth-client and @mrsmith/ui are served from source in dev: without
 * dedupe, Vite can load a second React instance for them and hooks crash with
 * "Invalid hook call" at startup (issue #49).
 */
export function defineMrSmithAppConfig(options: MrSmithAppConfigOptions = {}) {
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
      } satisfies UserConfig,
      options.overrides ?? {},
    ),
  );
}
