// Type augmentation for Vite's `import.meta.env`. Declared locally to avoid
// pulling Vite as a dev dependency of this package — the host application is
// always a Vite app, which provides the runtime implementation.
interface ImportMetaEnv {
  readonly VITE_DEV_AUTH_BYPASS?: string;
  readonly VITE_DEV_FAKE_ROLES?: string;
  readonly VITE_DEV_FAKE_NAME?: string;
  readonly VITE_DEV_FAKE_EMAIL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
