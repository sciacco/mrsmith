export type RackNumberFormatter = (value: number) => string;

export function formatRackPowerLabel(value: number | undefined, formatNumber: RackNumberFormatter): string {
  if (value === undefined || value === 0) return '-';
  return `${formatNumber(value)} kW`;
}
