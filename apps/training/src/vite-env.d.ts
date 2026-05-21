/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_TRAINING_USE_MOCKS?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
