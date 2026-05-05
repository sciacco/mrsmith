import type { AudienceOverride, ReferenceItem, SeverityValue } from '../api/types';

export interface ImpactSelectionView {
  reference: ReferenceItem;
  source: string;
  confidence?: number | null;
  isPrimary: boolean;
  role: 'operated' | 'dependent';
  expectedSeverity: SeverityValue | null;
  expectedAudience: AudienceOverride | null;
}

export function impactSourceLabel(source: string): string {
  const labels: Record<string, string> = {
    manual: 'Manuale',
    import: 'Importazione',
    rule: 'Regola',
    ai_extracted: 'AI',
    ai: 'AI',
    catalog_mapping: 'Catalogo',
    dependency_graph: 'Dependency graph',
    hybrid: 'Ibrido',
  };
  return labels[source] ?? source;
}
