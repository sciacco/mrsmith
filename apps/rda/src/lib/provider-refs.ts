export const QUALIFICATION_REF = 'QUALIFICATION_REF';

export const providerReferenceTypes = [
  { value: 'ADMINISTRATIVE_REF', label: 'Amministrativo' },
  { value: 'TECHNICAL_REF', label: 'Tecnico' },
  { value: 'COMMERCIAL_REF', label: 'Commerciale' },
  { value: 'OTHER_REF', label: 'Altro' },
] as const;

export function referenceTypeLabel(value?: string): string {
  if (value === QUALIFICATION_REF) return 'Qualifica';
  return providerReferenceTypes.find((item) => item.value === value)?.label ?? value ?? '-';
}

export function availableReferenceTypes() {
  return providerReferenceTypes;
}
