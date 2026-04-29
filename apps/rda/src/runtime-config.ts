let rdaQuoteThreshold = 0;

export interface RdaRuntimeConfig {
  rdaQuoteThreshold: number;
}

export function setRuntimeConfig(config: RdaRuntimeConfig) {
  if (!Number.isFinite(config.rdaQuoteThreshold) || config.rdaQuoteThreshold <= 0) {
    throw new Error('RDA quote threshold is missing from runtime configuration.');
  }
  rdaQuoteThreshold = config.rdaQuoteThreshold;
}

export function getRdaQuoteThreshold(): number {
  return rdaQuoteThreshold;
}
