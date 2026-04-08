export interface RuntimeConfig {
  keycloakUrl: string;
  realm: string;
  clientId: string;
  arakEnabled: boolean;
}

let runtimeConfig: RuntimeConfig | null = null;

export function setRuntimeConfig(config: RuntimeConfig) {
  runtimeConfig = config;
}

export function getRuntimeConfig(): RuntimeConfig {
  if (runtimeConfig == null) {
    throw new Error('Runtime config not initialized');
  }
  return runtimeConfig;
}
