import { ApiError } from '@mrsmith/api-client';
import type {
  MAEstimate,
  MAGatedProgressResponse,
  MAProvinceCatalogItem,
  MASessionStatus,
  MAStrategySpec,
  MAStrategyType,
  MATarget,
  MATargetRow,
} from '../../api/types';

export const numberFormat = new Intl.NumberFormat('it-IT');
export const dateFormat = new Intl.DateTimeFormat('it-IT', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
});

export const dateTimeFormat = new Intl.DateTimeFormat('it-IT', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
});

export const defaultSearchLimit = 100;
export const maxSearchLimit = 1000;

export interface EstimatePreview {
  type: MAStrategyType;
  count: number;
  blocked: boolean;
  lowerBound: boolean;
  probeCount: number;
  rows: MAEstimate[];
}

export interface SectorBlocker {
  title: string;
  message: string;
}

export function dateLabel(value?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return dateFormat.format(date);
}

export function dateTimeLabel(value?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return dateTimeFormat.format(date);
}

export function relativeDate(value?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMin = Math.round(diffMs / 60000);
  const diffHour = Math.round(diffMs / 3600000);
  const diffDay = Math.round(diffMs / 86400000);
  if (diffMin < 1) return 'adesso';
  if (diffMin < 60) return `${diffMin} min fa`;
  if (diffHour < 24) return `${diffHour} h fa`;
  if (diffDay === 1) return 'ieri';
  if (diffDay < 31) return `${diffDay} g fa`;
  return date.toLocaleDateString('it-IT', { day: '2-digit', month: 'short' });
}

export function shortAuthor(email?: string): string {
  if (!email) return 'Sistema';
  return email.split('@')[0] ?? email;
}

export function sessionStatusLabel(status: MASessionStatus): string {
  switch (status) {
    case 'draft':
      return 'Bozza strategia';
    case 'estimating':
      return 'Stima in corso';
    case 'estimated':
      return 'Stima pronta';
    case 'running':
      return 'Ricerca in corso';
    case 'completed':
      return 'Completata';
    case 'failed':
      return 'Da rivedere';
    default:
      return status;
  }
}

export function errorLabel(error: unknown): string {
  if (error instanceof ApiError) {
    const code = errorCode(error);
    if (code === 'openrouter_not_configured') return 'La preparazione della strategia non e disponibile in questo ambiente.';
    if (code === 'binocolo_llm_config_not_configured') return 'La configurazione IA di Binocolo non e completa.';
    if (code === 'openapiit_not_configured') return 'La ricerca aziende non e disponibile in questo ambiente.';
    if (code === 'openapiit_credit_required') return 'Credito ricerca insufficiente per completare la stima.';
    if (code === 'estimate_too_large') return 'La stima e troppo ampia: restringi territorio, fatturato o settore prima di confermare.';
    if (code === 'invalid_ma_request') return 'Controlla i criteri della strategia e riprova.';
    if (code === 'ma_session_not_found') return 'Ricerca non trovata.';
    if (code === 'ma_session_archived') return 'La ricerca e archiviata: ripristinala prima di modificarla.';
    if (code === 'ma_session_deleted') return 'La ricerca e nel cestino: ripristinala prima di aprirla.';
    if (error.status === 401) return 'Sessione non valida.';
    if (error.status === 403) return 'Non hai accesso a Binocolo.';
    return 'Operazione non riuscita.';
  }
  if (error instanceof Error) return error.message;
  return 'Operazione non riuscita.';
}

export function optionalNumber(value: string): number | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
}

export function normalizeSearchLimit(value?: number): number {
  if (!Number.isFinite(value) || !value || value < 1) return defaultSearchLimit;
  return Math.min(maxSearchLimit, Math.trunc(value));
}

export function expandedEstimatePreview(estimates: MAEstimate[]): EstimatePreview | null {
  const rows = estimates.filter((estimate) => estimate.strategyType === 'expanded');
  if (rows.length === 0) return null;
  return rows.reduce<EstimatePreview>(
    (acc, estimate) => ({
      ...acc,
      count: acc.count + estimate.estimatedCount,
      blocked: acc.blocked || estimate.surfaceStatus === 'too_broad',
      lowerBound: acc.lowerBound || estimateUsesLowerBound(estimate),
      probeCount: acc.probeCount + (estimate.probeCount ?? 1),
      rows: [...acc.rows, estimate],
    }),
    { type: 'expanded', count: 0, blocked: false, lowerBound: false, probeCount: 0, rows: [] },
  );
}

export function estimateUsesLowerBound(estimate: MAEstimate): boolean {
  return estimate.surfaceStatus === 'too_broad' && (estimate.probeCount ?? 1) > 1;
}

export function estimateCountLabel(count: number, lowerBound: boolean): string {
  const formatted = numberFormat.format(count);
  return lowerBound ? `piu di ${formatted}` : formatted;
}

export function provinceCatalogMap(catalog: MAProvinceCatalogItem[]): Map<string, MAProvinceCatalogItem> {
  return new Map(catalog.map((item) => [item.code.toUpperCase(), item]));
}

export function provinceLabel(code: string | undefined, catalog: MAProvinceCatalogItem[]): string {
  if (!code) return '-';
  const item = provinceCatalogMap(catalog).get(code.toUpperCase());
  return item ? item.name : code;
}

export function summarizeTerritory(provinceCodes: string[], catalog: MAProvinceCatalogItem[]): string {
  if (provinceCodes.length === 0) return 'Italia';
  const byCode = provinceCatalogMap(catalog);
  const selected = new Set(provinceCodes.map((code) => code.toUpperCase()));
  const byRegion = new Map<string, MAProvinceCatalogItem[]>();
  for (const item of catalog) {
    const list = byRegion.get(item.region) ?? [];
    list.push(item);
    byRegion.set(item.region, list);
  }

  const labels: string[] = [];
  const consumed = new Set<string>();
  for (const [region, items] of byRegion) {
    if (items.length > 0 && items.every((item) => selected.has(item.code.toUpperCase()))) {
      labels.push(region);
      for (const item of items) consumed.add(item.code.toUpperCase());
    }
  }
  for (const code of provinceCodes) {
    const normalized = code.toUpperCase();
    if (!consumed.has(normalized)) labels.push(byCode.get(normalized)?.name ?? normalized);
  }
  return labels.join(', ') || 'Italia';
}

export function territoryIsWide(provinceCodes: string[], catalog: MAProvinceCatalogItem[]): boolean {
  if (provinceCodes.length === 0) return true;
  const byCode = provinceCatalogMap(catalog);
  const regions = new Set(
    provinceCodes
      .map((code) => byCode.get(code.toUpperCase())?.region)
      .filter((region): region is string => Boolean(region)),
  );
  return regions.size > 1;
}

export function sectorBlocker(strategy: MAStrategySpec): SectorBlocker | null {
  const missing = (strategy.missingCriteria ?? []).join(' ').toLowerCase();
  const concepts = strategy.sectorConcepts ?? [];
  const positiveConcepts = concepts.filter((concept) => concept.fit !== 'excluded');
  const positiveCodes = (strategy.atecoCandidates ?? []).filter((candidate) => (candidate.fit ?? 'core') !== 'excluded');

  if (concepts.length === 0 && missing.includes('settore non riconosciuto dalla base di conoscenza ateco')) {
    return {
      title: 'Ambiti non riconosciuti.',
      message: 'Il sistema e attualmente configurato per settori ICT e adiacenti. Riformulare la richiesta descrivendo in concreto le attivita svolte.',
    };
  }

  if (missing.includes('solo') && missing.includes('esclud')) {
    return {
      title: 'La richiesta indica solo cosa escludere.',
      message: 'Descrivere anche gli ambiti cercati.',
    };
  }

  if (
    positiveConcepts.length === 0 &&
    positiveCodes.length === 0 &&
    strategy.sectorRetrievalMode !== 'fallback_llm'
  ) {
    return {
      title: 'La richiesta non descrive cosa fanno le aziende cercate.',
      message: 'Indicare gli ambiti di business, anche in modo ampio: il giudizio di aderenza si basa su questa descrizione.',
    };
  }

  return null;
}

export function gatedBucketCounts(progress: MAGatedProgressResponse | null): { keep: number; forse: number; review: number; reject: number; total: number } {
  const buckets = progress?.gate.buckets;
  const keep = buckets?.keep ?? 0;
  const forse = buckets?.forse ?? 0;
  const review = buckets?.manualReview ?? 0;
  const reject = buckets?.scarta ?? 0;
  return { keep, forse, review, reject, total: keep + forse + review + reject };
}

export type MATargetListItem = MATarget | MATargetRow;

export function targetKey(target: MATargetListItem): string {
  const taxCode = 'taxCode' in target ? target.taxCode : undefined;
  return target.companyKey || target.vatCode || taxCode || target.id;
}

export function isGateReject(target: MATargetListItem): boolean {
  const validation = target.webValidation;
  const finalDecisionAction =
    validation?.finalDecision && 'finalAction' in validation.finalDecision ? validation.finalDecision.finalAction : undefined;
  return (
    validation?.finalAction === 'reject' ||
    finalDecisionAction === 'reject' ||
    validation?.webValidationState === 'rejected'
  );
}

export function isOperationalGateReject(target: MATargetListItem): boolean {
  return target.origin !== 'manual' && isGateReject(target);
}

export function bucketLabel(bucket?: string): string {
  switch (bucket) {
    case 'azionabile':
      return 'Azionabile';
    case 'da_verificare':
      return 'Da verificare';
    case 'soppresso':
      return 'Soppresso';
    case 'principale':
      return 'In tesi';
    default:
      return 'In tesi';
  }
}

// Chip Esito in tabella: segnala solo le eccezioni del routing v3.
// Le righe principale (e i bucket sconosciuti) non portano badge.
export function bucketChipLabel(bucket?: string): string | null {
  switch (bucket) {
    case 'azionabile':
      return 'Azionabile';
    case 'da_verificare':
      return 'Da verificare';
    case 'soppresso':
      return 'Soppresso';
    default:
      return null;
  }
}

export function bucketChipDescription(bucket?: string): string | null {
  switch (bucket) {
    case 'azionabile':
      return 'Esclusa da un criterio formale, ma un’azione la rimette in valutazione: settore da rivedere o dominio da associare.';
    case 'da_verificare':
      return 'Valutata su evidenza limitata: il punteggio indica da dove iniziare la verifica, non un rango consolidato.';
    case 'soppresso':
      return 'Nessuna azione disponibile: società cessata o dormiente, esclusa dalle regole della ricerca o fuori dagli ambiti descritti.';
    default:
      return null;
  }
}

export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export function targetsToCSV(targets: MATargetListItem[]): string {
  const rows = [
    ['Azienda', 'Partita IVA', 'Codice fiscale', 'Provincia', 'Comune', 'Punteggio', 'Esito', 'Dominio', 'Motivo'],
    ...targets.map((target) => {
      const taxCode = 'taxCode' in target ? target.taxCode : undefined;
      const rationale = 'rationale' in target ? target.rationale : undefined;
      return [
        target.companyName,
        target.vatCode ?? '',
        taxCode ?? '',
        target.province ?? '',
        target.town ?? '',
        String(target.score ?? ''),
        bucketLabel(target.bucket),
        target.webValidation?.selectedDomain ?? '',
        target.webValidation?.finalDecision?.reason ?? rationale ?? '',
      ];
    }),
  ];
  return rows.map((row) => row.map(csvCell).join(',')).join('\n');
}

export function safeFilename(value: string): string {
  const normalized = value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48);
  return normalized || 'ricerca';
}

function errorCode(error: ApiError): string | undefined {
  const body = error.body;
  if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') {
    return body.error;
  }
  return undefined;
}

function csvCell(value: string) {
  return `"${value.replace(/"/g, '""')}"`;
}
