export const providerTabs = ['Dati', 'Contatti', 'Qualifica', 'Documenti'] as const;
export type ProviderTab = (typeof providerTabs)[number];

export const referenceTypes = [
  { value: 'ADMINISTRATIVE_REF', label: 'Amministrativo' },
  { value: 'TECHNICAL_REF', label: 'Tecnico' },
  { value: 'OTHER_REF', label: 'Altro' },
] as const;

export function stateLabel(value?: string | null) {
  if (!value) return '-';
  return value;
}

export function referenceTypeLabel(value?: string | null) {
  return referenceTypes.find((item) => item.value === value)?.label ?? 'Altro';
}
