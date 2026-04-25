import type { SeverityValue } from '../api/types';

export type SeveritySelectionValue = SeverityValue | null;
export type SeverityTone = SeverityValue | 'undefined';

export interface SeverityOption {
  value: SeverityValue;
  label: string;
  tone: SeverityValue;
}

export interface SeveritySelectionOption {
  value: SeveritySelectionValue;
  label: string;
  tone: SeverityTone;
}

export const SEVERITY_OPTIONS: readonly SeverityOption[] = [
  { value: 'none', label: 'Nessun impatto', tone: 'none' },
  { value: 'degraded', label: 'Degradato', tone: 'degraded' },
  { value: 'unavailable', label: 'Down/Indisponibile', tone: 'unavailable' },
] as const;

export const UNDEFINED_SEVERITY_OPTION: SeveritySelectionOption = {
  value: null,
  label: 'Da definire',
  tone: 'undefined',
};

export const NULLABLE_SEVERITY_OPTIONS: readonly SeveritySelectionOption[] = [
  ...SEVERITY_OPTIONS,
  UNDEFINED_SEVERITY_OPTION,
];

export function severityLabel(value?: string | null): string {
  if (!value) return '-';
  return SEVERITY_OPTIONS.find((option) => option.value === value)?.label ?? value;
}

export function severityOption(value: SeveritySelectionValue): SeveritySelectionOption {
  if (value === null) return UNDEFINED_SEVERITY_OPTION;
  return SEVERITY_OPTIONS.find((option) => option.value === value) ?? UNDEFINED_SEVERITY_OPTION;
}
