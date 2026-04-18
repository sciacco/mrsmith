import type { CalculationResult, QuotePayload, ResourceValues } from '../../api/types.ts';

const MONTHLY_MULTIPLIER = 30;

function round2(value: number): number {
  return Math.round(value * 100) / 100;
}

export function buildQuotePayload(
  calculation: CalculationResult,
  rates: ResourceValues,
): QuotePayload {
  const { dailyTotals } = calculation;
  return {
    qta: calculation.normalizedQuantities,
    prezzi: rates,
    totale_giornaliero: {
      computing: round2(dailyTotals.computing),
      storage: round2(dailyTotals.storage),
      sicurezza: round2(dailyTotals.sicurezza),
      addon: round2(dailyTotals.addon),
      totale: round2(dailyTotals.totale),
      mese: round2(dailyTotals.totale * MONTHLY_MULTIPLIER),
    },
  };
}
