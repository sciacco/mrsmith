export type ResourceKey =
  | 'sites'
  | 'technical-domains'
  | 'maintenance-kinds'
  | 'customer-scopes'
  | 'service-taxonomy'
  | 'reason-classes'
  | 'impact-effects'
  | 'quality-flags'
  | 'target-types'
  | 'notice-channels';

export type ResourceGroup = 'classification' | 'impact' | 'communications';

export type FieldRequirement = 'required' | 'optional' | 'hidden';

export interface ResourceMeta {
  key: ResourceKey;
  title: string;
  singular: string;
  singularArticle: 'il' | 'la' | "l'";
  plural: string;
  subtitle: string;
  shortDescription: string;
  group: ResourceGroup;
  fields: {
    name_en: FieldRequirement;
    description: FieldRequirement;
    sort_order: FieldRequirement;
    technical_domain_id?: FieldRequirement;
    city?: FieldRequirement;
    country_code?: FieldRequirement;
  };
}

export const RESOURCE_META: Record<ResourceKey, ResourceMeta> = {
  sites: {
    key: 'sites',
    title: 'Siti',
    singular: 'sito',
    singularArticle: 'il',
    plural: 'siti',
    subtitle: 'Sedi e data center disponibili per le manutenzioni.',
    shortDescription: 'Sedi e data center usati nelle manutenzioni.',
    group: 'impact',
    fields: {
      name_en: 'optional',
      description: 'optional',
      sort_order: 'required',
      city: 'required',
      country_code: 'required',
    },
  },
  'technical-domains': {
    key: 'technical-domains',
    title: 'Domini tecnici',
    singular: 'dominio tecnico',
    singularArticle: 'il',
    plural: 'domini tecnici',
    subtitle: 'Aree tecniche usate per classificare le manutenzioni.',
    shortDescription: 'Aree tecniche e operative.',
    group: 'classification',
    fields: { name_en: 'optional', description: 'optional', sort_order: 'required' },
  },
  'maintenance-kinds': {
    key: 'maintenance-kinds',
    title: 'Tipi manutenzione',
    singular: 'tipo manutenzione',
    singularArticle: 'il',
    plural: 'tipi manutenzione',
    subtitle: 'Classificazione principale delle manutenzioni.',
    shortDescription: 'Classificazione principale della manutenzione.',
    group: 'classification',
    fields: { name_en: 'optional', description: 'optional', sort_order: 'required' },
  },
  'customer-scopes': {
    key: 'customer-scopes',
    title: 'Ambiti clienti',
    singular: 'ambito clienti',
    singularArticle: "l'",
    plural: 'ambiti clienti',
    subtitle: 'Perimetro clienti coinvolto dalla manutenzione.',
    shortDescription: 'Perimetro clienti coinvolto.',
    group: 'classification',
    fields: { name_en: 'optional', description: 'optional', sort_order: 'required' },
  },
  'service-taxonomy': {
    key: 'service-taxonomy',
    title: 'Servizi',
    singular: 'servizio',
    singularArticle: 'il',
    plural: 'servizi',
    subtitle: 'Tassonomia servizi collegata ai domini tecnici.',
    shortDescription: 'Tassonomia servizi collegata ai domini.',
    group: 'impact',
    fields: {
      name_en: 'optional',
      description: 'optional',
      sort_order: 'required',
      technical_domain_id: 'required',
    },
  },
  'reason-classes': {
    key: 'reason-classes',
    title: 'Motivi',
    singular: 'motivo',
    singularArticle: 'il',
    plural: 'motivi',
    subtitle: 'Motivazioni operative ricorrenti delle manutenzioni.',
    shortDescription: 'Motivazioni operative ricorrenti.',
    group: 'classification',
    fields: { name_en: 'optional', description: 'optional', sort_order: 'required' },
  },
  'impact-effects': {
    key: 'impact-effects',
    title: 'Effetti impatto',
    singular: 'effetto impatto',
    singularArticle: "l'",
    plural: 'effetti impatto',
    subtitle: 'Effetti attesi sui servizi durante la manutenzione.',
    shortDescription: 'Effetti attesi sui servizi.',
    group: 'impact',
    fields: { name_en: 'optional', description: 'optional', sort_order: 'required' },
  },
  'quality-flags': {
    key: 'quality-flags',
    title: 'Segnali qualità',
    singular: 'segnale qualità',
    singularArticle: 'il',
    plural: 'segnali qualità',
    subtitle: 'Controlli editoriali applicati alle comunicazioni.',
    shortDescription: 'Controlli editoriali applicati alle comunicazioni.',
    group: 'communications',
    fields: { name_en: 'optional', description: 'optional', sort_order: 'required' },
  },
  'target-types': {
    key: 'target-types',
    title: 'Tipi target',
    singular: 'tipo target',
    singularArticle: 'il',
    plural: 'tipi target',
    subtitle: 'Categorie di elementi impattati dalle manutenzioni.',
    shortDescription: 'Categorie di elementi impattati dalle manutenzioni.',
    group: 'impact',
    fields: { name_en: 'optional', description: 'optional', sort_order: 'required' },
  },
  'notice-channels': {
    key: 'notice-channels',
    title: 'Canali comunicazione',
    singular: 'canale comunicazione',
    singularArticle: 'il',
    plural: 'canali comunicazione',
    subtitle: 'Canali disponibili per le comunicazioni ai clienti.',
    shortDescription: 'Canali disponibili per le comunicazioni.',
    group: 'communications',
    fields: { name_en: 'optional', description: 'optional', sort_order: 'required' },
  },
};

export const RESOURCE_GROUPS: Array<{ id: ResourceGroup; label: string }> = [
  { id: 'classification', label: 'Classificazione' },
  { id: 'impact', label: 'Impatto e servizi' },
  { id: 'communications', label: 'Comunicazioni' },
];

export const RESOURCE_KEYS: ResourceKey[] = [
  'sites',
  'technical-domains',
  'maintenance-kinds',
  'customer-scopes',
  'service-taxonomy',
  'reason-classes',
  'impact-effects',
  'quality-flags',
  'target-types',
  'notice-channels',
];

export function getResourceMeta(resource: string): ResourceMeta | undefined {
  return RESOURCE_META[resource as ResourceKey];
}

export function capitalize(value: string): string {
  if (!value) return value;
  return value.charAt(0).toUpperCase() + value.slice(1);
}
