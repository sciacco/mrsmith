import type {
  DashboardCategory,
  DashboardDocument,
  DashboardDraft,
  Provider,
  ProviderCategory,
  ProviderDocument,
} from '../api/types';
import { providerDetailPath, type ProviderDetailSection } from './providerRoutes.ts';

export type ProviderAttentionSeverity = 'none' | 'blocking' | 'expired' | 'expiring' | 'completion';

export interface ProviderAttentionCounts {
  drafts: number;
  expiredDocuments: number;
  expiringDocuments: number;
  openCategories: number;
  criticalCategories: number;
}

export interface ProviderAttentionAction {
  id: string;
  label: string;
  detail: string;
  severity: Exclude<ProviderAttentionSeverity, 'none'>;
  section: ProviderDetailSection;
  focus?: string;
  score: number;
}

export interface ProviderAttention {
  providerId: number;
  companyName: string;
  severity: ProviderAttentionSeverity;
  openCount: number;
  counts: ProviderAttentionCounts;
  actions: ProviderAttentionAction[];
  href: string;
  actionLabel: string;
  sortScore: number;
}

export interface PrioritySummary {
  overdue: number;
  expiring: number;
  drafts: number;
  openCategories: number;
}

interface AttentionAccumulator {
  providerId: number;
  companyName: string;
  counts: ProviderAttentionCounts;
  actions: ProviderAttentionAction[];
}

const SEVERITY_RANK: Record<ProviderAttentionSeverity, number> = {
  blocking: 0,
  expired: 1,
  expiring: 2,
  completion: 3,
  none: 4,
};

export const PROVIDER_ATTENTION_LABELS: Record<ProviderAttentionSeverity, string> = {
  blocking: 'Bloccante',
  expired: 'Scaduto',
  expiring: 'In scadenza',
  completion: 'Da completare',
  none: 'In ordine',
};

function displayValue(value?: string | number | boolean | null) {
  if (value === null || value === undefined || value === '') return '-';
  return String(value);
}

function normalizeState(value?: string | null) {
  return (value ?? '').toUpperCase().replace(/[-\s]+/g, '_');
}

function providerName(provider?: Pick<Provider, 'company_name'> | DashboardDraft | DashboardDocument | DashboardCategory | null) {
  return displayValue(provider?.company_name);
}

function actionSeverity(actions: ProviderAttentionAction[]): ProviderAttentionSeverity {
  if (actions.length === 0) return 'none';
  if (actions.some((action) => action.severity === 'blocking')) return 'blocking';
  if (actions.some((action) => action.severity === 'expired')) return 'expired';
  if (actions.some((action) => action.severity === 'expiring')) return 'expiring';
  return 'completion';
}

function sortActions(actions: ProviderAttentionAction[]) {
  return [...actions].sort((a, b) => (
    SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity] ||
    b.score - a.score ||
    a.detail.localeCompare(b.detail, 'it')
  ));
}

function buildAttention(item: AttentionAccumulator): ProviderAttention {
  const actions = sortActions(item.actions);
  const firstAction = actions[0];
  const severity = actionSeverity(actions);
  const openCount = item.counts.drafts + item.counts.expiredDocuments + item.counts.expiringDocuments + item.counts.openCategories;
  const sortScore = actions.reduce((total, action) => total + action.score, 0);
  return {
    providerId: item.providerId,
    companyName: item.companyName,
    severity,
    openCount,
    counts: item.counts,
    actions,
    href: firstAction ? providerDetailPath(item.providerId, firstAction.section, firstAction.focus) : providerDetailPath(item.providerId, 'dati'),
    actionLabel: firstAction?.label ?? 'Apri fornitore',
    sortScore,
  };
}

function ensureAccumulator(groups: Map<number, AttentionAccumulator>, providerId: number, companyName?: string | null) {
  const existing = groups.get(providerId);
  if (existing) {
    if (existing.companyName === '-' && companyName) existing.companyName = displayValue(companyName);
    return existing;
  }
  const next: AttentionAccumulator = {
    providerId,
    companyName: displayValue(companyName),
    counts: {
      drafts: 0,
      expiredDocuments: 0,
      expiringDocuments: 0,
      openCategories: 0,
      criticalCategories: 0,
    },
    actions: [],
  };
  groups.set(providerId, next);
  return next;
}

export function buildPrioritySummary(
  documents: DashboardDocument[],
  categories: DashboardCategory[],
  drafts: DashboardDraft[],
): PrioritySummary {
  return {
    overdue: documents.filter((row) => row.days_remaining < 0).length,
    expiring: documents.filter((row) => row.days_remaining >= 0).length,
    drafts: drafts.length,
    openCategories: categories.length,
  };
}

function expiryCopy(documentType?: string | null, days?: number | null) {
  const label = documentType || 'Documento';
  if (days == null) return `${label} in scadenza`;
  if (days < 0) return `${label} scaduto da ${Math.abs(days)}gg`;
  if (days === 0) return `${label} scade oggi`;
  return `${label} scade tra ${days}gg`;
}

function documentAction(documentId: number, documentType: string | null | undefined, days: number): ProviderAttentionAction {
  const expired = days < 0;
  return {
    id: `document-${documentId}`,
    label: 'Sostituisci documento',
    detail: expiryCopy(documentType, days),
    severity: expired ? 'expired' : 'expiring',
    section: 'documenti',
    focus: `document-${documentId}`,
    score: expired ? Math.min(45, 25 + Math.abs(days)) : Math.max(4, 18 - days),
  };
}

function namedCategoryIssue(name: string, critical: boolean) {
  if (!name || name === '-') return critical ? 'Categoria critica da qualificare' : 'Categoria da qualificare';
  return critical ? `Categoria ${name} critica da qualificare` : `Categoria ${name} da qualificare`;
}

function categoryAction(categoryId: number, name: string | null | undefined, critical: boolean): ProviderAttentionAction {
  return {
    id: `category-${categoryId}`,
    label: 'Apri qualifica',
    detail: namedCategoryIssue(displayValue(name), critical),
    severity: critical ? 'blocking' : 'completion',
    section: 'qualifica',
    focus: `category-${categoryId}`,
    score: critical ? 56 : 16,
  };
}

export function buildDashboardProviderAttention({
  documents,
  categories,
  drafts,
}: {
  documents: DashboardDocument[];
  categories: DashboardCategory[];
  drafts: DashboardDraft[];
}): ProviderAttention[] {
  const groups = new Map<number, AttentionAccumulator>();

  for (const row of drafts) {
    const item = ensureAccumulator(groups, row.id, row.company_name);
    item.counts.drafts += 1;
    item.actions.push({
      id: `draft-${row.id}`,
      label: 'Completa dati',
      detail: 'Qualifica da completare',
      severity: 'blocking',
      section: 'dati',
      score: 55,
    });
  }

  for (const row of documents) {
    const item = ensureAccumulator(groups, row.provider_id, row.company_name);
    if (row.days_remaining < 0) item.counts.expiredDocuments += 1;
    else item.counts.expiringDocuments += 1;
    item.actions.push(documentAction(row.id, row.document_type, row.days_remaining));
  }

  for (const row of categories) {
    const item = ensureAccumulator(groups, row.provider_id, row.company_name);
    item.counts.openCategories += 1;
    if (row.critical) item.counts.criticalCategories += 1;
    item.actions.push(categoryAction(row.category_id, row.category_name, row.critical));
  }

  return Array.from(groups.values())
    .map(buildAttention)
    .filter((attention) => attention.openCount > 0)
    .sort((a, b) => (
      SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity] ||
      b.sortScore - a.sortScore ||
      a.companyName.localeCompare(b.companyName, 'it')
    ));
}

function parseDateOnly(raw: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(raw);
  if (match) {
    const year = Number(match[1]);
    const month = Number(match[2]);
    const day = Number(match[3]);
    const parsed = new Date(year, month - 1, day);
    return Number.isNaN(parsed.getTime()) ? null : parsed;
  }
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return null;
  parsed.setHours(0, 0, 0, 0);
  return parsed;
}

export function daysUntil(expireDate?: string | null, today = new Date()): number | null {
  if (!expireDate) return null;
  const target = parseDateOnly(expireDate);
  if (!target) return null;
  const start = new Date(today);
  start.setHours(0, 0, 0, 0);
  return Math.round((target.getTime() - start.getTime()) / 86_400_000);
}

export function missingProviderActivationFields(provider: Provider): string[] {
  const missing: string[] = [];
  if (!provider.company_name?.trim()) missing.push('ragione sociale');
  if (!provider.address?.trim()) missing.push('indirizzo');
  if (!provider.city?.trim()) missing.push('citta');
  if (!provider.postal_code?.trim()) missing.push('CAP');
  if (!provider.country?.trim()) missing.push('paese');
  if (!provider.erp_id || provider.erp_id <= 0) missing.push('codice ERP');
  const payment = provider.default_payment_method;
  const paymentCode = payment && typeof payment === 'object' ? payment.code : typeof payment === 'string' ? payment : '';
  if (!paymentCode.trim()) missing.push('metodo di pagamento');
  const refs = provider.refs?.length ? provider.refs : provider.ref ? [provider.ref] : [];
  if (refs.length === 0) missing.push('almeno un contatto');
  return missing;
}

export function buildDetailProviderAttention({
  provider,
  providerCategories,
  providerDocuments,
  today,
}: {
  provider: Provider;
  providerCategories: ProviderCategory[];
  providerDocuments: ProviderDocument[];
  today?: Date;
}): ProviderAttention {
  const item: AttentionAccumulator = {
    providerId: provider.id,
    companyName: providerName(provider),
    counts: {
      drafts: 0,
      expiredDocuments: 0,
      expiringDocuments: 0,
      openCategories: 0,
      criticalCategories: 0,
    },
    actions: [],
  };

  if (normalizeState(provider.state) === 'DRAFT') {
    const missing = missingProviderActivationFields(provider);
    item.counts.drafts = 1;
    const contactMissing = missing.includes('almeno un contatto');
    const missingData = missing.filter((field) => field !== 'almeno un contatto');
    if (missingData.length > 0 || missing.length === 0) {
      item.actions.push({
        id: `draft-${provider.id}`,
        label: 'Completa dati',
        detail: missingData.length > 0 ? `Mancano: ${missingData.join(', ')}` : 'Qualifica da completare',
        severity: 'blocking',
        section: 'dati',
        score: 55 + missingData.length * 2,
      });
    }
    if (contactMissing) {
      item.actions.push({
        id: `draft-contact-${provider.id}`,
        label: 'Aggiungi contatto',
        detail: 'Manca almeno un contatto',
        severity: 'blocking',
        section: 'contatti',
        score: 54,
      });
    }
  }

  for (const document of providerDocuments) {
    const days = daysUntil(document.expire_date, today);
    if (days === null || days > 30) continue;
    if (days < 0) item.counts.expiredDocuments += 1;
    else item.counts.expiringDocuments += 1;
    item.actions.push(documentAction(document.id, document.document_type?.name, days));
  }

  for (const providerCategory of providerCategories) {
    const state = normalizeState(providerCategory.status ?? providerCategory.state);
    if (state === 'QUALIFIED') continue;
    const categoryId = providerCategory.category?.id;
    if (categoryId == null) continue;
    item.counts.openCategories += 1;
    if (providerCategory.critical) item.counts.criticalCategories += 1;
    item.actions.push(categoryAction(categoryId, providerCategory.category?.name, Boolean(providerCategory.critical)));
  }

  return buildAttention(item);
}
