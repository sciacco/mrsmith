export const PROVIDER_DETAIL_SECTIONS = ['dati', 'qualifica', 'documenti', 'contatti'] as const;

export type ProviderDetailSection = (typeof PROVIDER_DETAIL_SECTIONS)[number];

const DEFAULT_PROVIDER_SECTION: ProviderDetailSection = 'dati';

export function normalizeProviderSection(section?: string | null): ProviderDetailSection {
  if (!section) return DEFAULT_PROVIDER_SECTION;
  const normalized = section.trim().toLocaleLowerCase('it');
  return PROVIDER_DETAIL_SECTIONS.includes(normalized as ProviderDetailSection)
    ? (normalized as ProviderDetailSection)
    : DEFAULT_PROVIDER_SECTION;
}

export function sectionFromLegacyTab(tab?: string | null): ProviderDetailSection {
  const normalized = tab?.trim().toLocaleLowerCase('it') ?? '';
  if (normalized === 'qualifica') return 'qualifica';
  return 'dati';
}

export function providerDetailPath(providerId: number | string, section?: ProviderDetailSection, focus?: string | null): string {
  const params = new URLSearchParams();
  if (section) params.set('section', section);
  if (focus) params.set('focus', focus);
  const suffix = params.size > 0 ? `?${params.toString()}` : '';
  return `/fornitori/${encodeURIComponent(String(providerId))}${suffix}`;
}

export function legacyProviderDetailPath(idProvider?: string | null, tab?: string | null): string | null {
  const value = idProvider?.trim();
  if (!value) return null;
  const id = Number(value);
  if (!Number.isInteger(id) || id <= 0) return null;
  return providerDetailPath(id, sectionFromLegacyTab(tab));
}
