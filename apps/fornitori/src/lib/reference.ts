export const providerTabs = ['Dati', 'Contatti', 'Qualifica', 'Documenti'] as const;
export type ProviderTab = (typeof providerTabs)[number];

export const referenceTypes = [
  { value: 'ADMINISTRATIVE_REF', label: 'Amministrativo' },
  { value: 'TECHNICAL_REF', label: 'Tecnico' },
  { value: 'OTHER_REF', label: 'Altro' },
] as const;

export function stateLabel(value?: string | null) {
  if (!value) return '-';
  const labels: Record<string, string> = {
    ACTIVE: 'Attivo',
    APPROVED: 'Approvato',
    DRAFT: 'Bozza',
    EXPIRED: 'Scaduto',
    INACTIVE: 'Non attivo',
    MISSING: 'Mancante',
    NOT_QUALIFIED: 'Non qualificato',
    PENDING: 'In attesa',
    QUALIFIED: 'Qualificato',
    REJECTED: 'Respinto',
    REQUIRED: 'Obbligatorio',
    TO_REVIEW: 'Da verificare',
    VALID: 'Valido',
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
