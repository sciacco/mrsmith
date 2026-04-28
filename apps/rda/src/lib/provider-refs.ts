export const QUALIFICATION_REF = 'QUALIFICATION_REF';
export const PROVIDER_REFERENCE_PHONE_PATTERN = String.raw`\+?[0-9 ]{6,20}`;
export const PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE = 'Inserisci un numero di telefono valido oppure lascia il campo vuoto.';

const PROVIDER_REFERENCE_PHONE_RE = /^\+?[0-9 ]{6,20}$/;

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

export function isValidOptionalProviderRefPhone(value?: string | null) {
  const phone = value?.trim() ?? '';
  return phone === '' || PROVIDER_REFERENCE_PHONE_RE.test(phone);
}
