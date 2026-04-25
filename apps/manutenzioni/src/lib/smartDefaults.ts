import type { ReferenceItem, SeverityValue } from '../api/types';

export type SuggestedSeverity = SeverityValue | null;

const KIND_CODE_TO_SEVERITY: Record<string, SuggestedSeverity> = {
  preventive: 'none',
  planned: 'degraded',
  corrective: 'degraded',
  extraordinary: 'unavailable',
  emergency: 'unavailable',
  other: null,
};

export function suggestedSeverityForKind(kind: ReferenceItem | undefined | null): SuggestedSeverity {
  if (!kind) return null;
  const code = kind.code?.toLowerCase();
  if (!code) return null;
  return KIND_CODE_TO_SEVERITY[code] ?? null;
}

export function suggestedSeverityForKindId(
  kindId: number | null,
  kinds: ReferenceItem[],
): SuggestedSeverity {
  if (!kindId) return null;
  return suggestedSeverityForKind(kinds.find((kind) => kind.id === kindId));
}

export function slugifyName(name: string): string {
  return name
    .normalize('NFD')
    .replace(/\p{M}/gu, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, 60);
}
