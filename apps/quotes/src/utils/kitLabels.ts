const BILLING_PERIOD_LABELS: Record<number, string> = {
  1: 'Mensile',
  2: 'Bimestrale',
  3: 'Trimestrale',
  4: 'Quadrimestrale',
  6: 'Semestrale',
  12: 'Annuale',
  24: 'Biennale',
};

export function billingPeriodLabel(value: number): string {
  return BILLING_PERIOD_LABELS[value] ?? `${value} mesi`;
}
