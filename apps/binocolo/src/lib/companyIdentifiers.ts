export function normalizeManualVat(value: string): string {
  return value.trim().toUpperCase();
}

export function validateManualVat(value: string): string | null {
  const normalized = normalizeManualVat(value);
  if (!normalized) return 'Inserisci una P.IVA o un codice fiscale.';
  if (/^\d{11}$/.test(normalized) || /^[A-Z0-9]{16}$/.test(normalized)) return null;
  return 'Usa 11 cifre oppure 16 caratteri alfanumerici.';
}

export function isValidManualDomainInput(value: string): boolean {
  const trimmed = value.trim();
  if (!trimmed) return true;
  if (/\s/.test(trimmed)) return false;
  const withoutProtocol = trimmed.replace(/^[a-z][a-z0-9+.-]*:\/\//i, '');
  const hostPart = withoutProtocol.split(/[/?#]/)[0] ?? '';
  const host = (hostPart.split('@').pop() ?? '').split(':')[0]?.replace(/^www\./i, '') ?? '';
  if (!host || host.startsWith('.') || host.endsWith('.') || host.includes('..') || !/^[a-z0-9.-]+$/i.test(host)) return false;
  const labels = host.split('.');
  if (labels.length < 2) return false;
  return labels.every((label) => label.length > 0 && !label.startsWith('-') && !label.endsWith('-'));
}

export function validateManualDomain(value: string): string | null {
  return isValidManualDomainInput(value) ? null : 'Inserisci un dominio valido, es. azienda.it.';
}
