export const QUALIFICATION_REFERENCE_TYPE = 'QUALIFICATION_REF';
export const PROVIDER_REFERENCE_PHONE_PATTERN = String.raw`\+?[0-9 ]{6,20}`;
export const PROVIDER_REFERENCE_PHONE_INVALID_MESSAGE = 'Inserisci un numero di telefono valido (+nnnnnn) oppure lascia il campo vuoto.';

const PROVIDER_REFERENCE_PHONE_RE = /^\+?[0-9 ]{6,20}$/;

export const referenceTypes = [
  { value: QUALIFICATION_REFERENCE_TYPE, label: 'Qualifica' },
  { value: 'ADMINISTRATIVE_REF', label: 'Amministrativo' },
  { value: 'TECHNICAL_REF', label: 'Tecnico' },
  { value: 'OTHER_REF', label: 'Altro' },
] as const;

export const addableReferenceTypes = referenceTypes.filter((item) => item.value !== QUALIFICATION_REFERENCE_TYPE);

export function stateLabel(value?: string | null) {
  if (!value) return '-';
  const labels: Record<string, string> = {
    ACTIVE: 'Attivo',
    CEASED: 'Cessato',
    DRAFT: 'Bozza',
    EXPIRED: 'Scaduto',
    INACTIVE: 'Sospeso',
    NEW: 'Nuova',
    NOT_QUALIFIED: 'Da qualificare',
    OK: 'Valido',
    PENDING_VERIFY_ALL: 'In verifica',
    PENDING_VERIFY_DATE: 'In verifica',
    PENDING_VERIFY_DOC: 'In verifica',
    QUALIFIED: 'Qualificata',
    REQUIRED: 'Obbligatorio',
  };
  const key = value.toUpperCase().replace(/[-\s]+/g, '_');
  if (labels[key]) return labels[key];
  return value
    .toLowerCase()
    .split('_')
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');
}

export function referenceTypeLabel(value?: string | null) {
  return referenceTypes.find((item) => item.value === value)?.label ?? 'Altro';
}

export function isValidOptionalProviderRefPhone(value?: string | null) {
  const phone = value?.trim() ?? '';
  return phone === '' || PROVIDER_REFERENCE_PHONE_RE.test(phone);
}
