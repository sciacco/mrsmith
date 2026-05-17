export const QUALIFICATION_REF = 'QUALIFICATION_REF';
export const PROVIDER_REFERENCE_PHONE_PATTERN = String.raw`\+[1-9][0-9]{4,19}`;
export const PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE = 'Usa il formato +391234567890 oppure lascia il campo vuoto.';

const PROVIDER_REFERENCE_PHONE_RE = /^\+[1-9][0-9]{4,19}$/;

export const providerReferenceTypes = [
  { value: 'ADMINISTRATIVE_REF', label: 'Amministrativo' },
  { value: 'TECHNICAL_REF', label: 'Tecnico' },
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

export function canManageProviderContacts(
  po: { id?: number; type?: string | null } | null | undefined,
  provider: { id?: number } | null | undefined,
) {
  return Boolean(po?.id && provider?.id && po.type !== 'ECOMMERCE');
}
