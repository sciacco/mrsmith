declare module '*.module.css' {
  const classes: { readonly [key: string]: string };
  export default classes;
}

interface ImportMetaEnv {
  readonly VITE_DEV_AUTH_BYPASS?: string;
  readonly VITE_DEV_FAKE_ROLES?: string;
  readonly VITE_DEV_FAKE_NAME?: string;
  readonly VITE_DEV_FAKE_EMAIL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
