import type { CalculationResult, QuotePayload, ResourceValues } from '../../api/types.ts';

export function buildQuotePayload(
  calculation: CalculationResult,
  rates: ResourceValues,
): QuotePayload {
  return {
    qta: calculation.normalizedQuantities,
    prezzi: rates,
    totale_giornaliero: calculation.dailyTotals,
  };
}
