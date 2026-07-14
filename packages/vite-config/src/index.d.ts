import type { UserConfig, UserConfigFnObject } from 'vite';

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
 * Shared Vite config for MrSmith React mini-apps: owns the React plugin, the
 * React/JSX-runtime dedupe list (issue #49), the `/apps/<slug>/` base-path
 * convention, and the standard `/api` + `/config` dev proxies.
 */
export declare function defineMrSmithAppConfig(
  options?: MrSmithAppConfigOptions,
): UserConfigFnObject;
