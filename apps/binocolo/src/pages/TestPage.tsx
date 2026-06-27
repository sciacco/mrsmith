import { useEffect, useMemo, useState, type ChangeEvent, type FormEvent } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Button, Icon, Skeleton, ToggleSwitch } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import type {
  CandidateMatchAnalysisResponse,
  CandidateMatchFinalDecision,
  CompanySearchRow,
  DomainResolutionCandidate,
  DomainResolutionResponse,
  MASessionDetail,
  MASessionListResponse,
  MASessionVisibility,
  MAStrategySpec,
  MATarget,
  MAWebValidation,
  OpenAPIITEnvelope,
  PipelineFinalAction,
  PipelineWebValidationState,
  WebSearchResponse,
} from '../api/types';
import styles from './TestPage.module.css';

const numberFormat = new Intl.NumberFormat('it-IT');
const defaultCompanyFilters = {
  province: 'AG',
  dataEnrichment: 'start',
  activityStatus: 'ATTIVA',
  companyName: '',
  atecoCode: '',
  cciaa: '',
  reaCode: '',
  minTurnover: '',
  maxTurnover: '',
  minEmployees: '',
  maxEmployees: '',
  skip: '0',
  limit: '10',
};

const defaultDomainResolutionForm = {
  companyName: '',
  vatCode: '',
  taxCode: '',
  town: '',
  province: '',
  keywords: '',
  count: '10',
};

const defaultKeywordEvidenceForm = {
  domain: '',
  keywords: '',
  count: '10',
  rank: true,
};

type CompanySearchFilters = typeof defaultCompanyFilters;
type CompanySearchFilterField = keyof CompanySearchFilters;
type DomainResolutionForm = typeof defaultDomainResolutionForm;
type DomainResolutionField = keyof DomainResolutionForm;
type KeywordEvidenceForm = typeof defaultKeywordEvidenceForm;
type KeywordEvidenceField = keyof Omit<KeywordEvidenceForm, 'rank'>;
type TestTab = 'company' | 'pipeline' | 'domain' | 'keyword' | 'maintenance';
type EvidenceBucket = 'core' | 'adjacent' | 'negative';

const testTabs = [
  { id: 'company', label: 'Company search', icon: 'database' },
  { id: 'pipeline', label: 'Evidence pipeline', icon: 'route' },
  { id: 'domain', label: 'Domain resolver', icon: 'network' },
  { id: 'keyword', label: 'Keyword evidence', icon: 'search' },
  { id: 'maintenance', label: 'Manutenzione', icon: 'settings' },
] as const;

const sessionVisibilityOptions: Array<{ value: MASessionVisibility; label: string }> = [
  { value: 'active', label: 'Attive' },
  { value: 'archived', label: 'Archiviate' },
  { value: 'deleted', label: 'Cestino' },
];

interface PipelineKeywordSet {
  intentLabel: string;
  coreTerms: string[];
  adjacentTerms: string[];
  negativeTerms: string[];
  sources: string[];
}

interface PipelineTermSpec {
  bucket: EvidenceBucket;
  term: string;
}

interface PipelineEvidenceRun extends PipelineTermSpec {
  response?: WebSearchResponse;
  error?: string;
  resultCount: number;
  bestScore?: number;
  matched: boolean;
}

interface PipelineSummary {
  score: number;
  confidence: 'alta' | 'media' | 'bassa';
  sectorEvidenceScore: number;
  coverageScore: number;
  domainScore: number;
  negativePenalty: number;
  coreMatches: number;
  adjacentMatches: number;
  negativeMatches: number;
  searchedCoreTerms: number;
  searchedAdjacentTerms: number;
  searchedNegativeTerms: number;
  totalCoreTerms: number;
  totalAdjacentTerms: number;
  totalNegativeTerms: number;
}

type PipelineFinalDecision = CandidateMatchFinalDecision;

interface PipelineRunResult {
  target: MATarget;
  keywordSet: PipelineKeywordSet;
  domainResponse: DomainResolutionResponse;
  selectedDomain?: DomainResolutionCandidate;
  evidenceRuns: PipelineEvidenceRun[];
  summary: PipelineSummary;
  candidateMatchAnalysis?: CandidateMatchAnalysisResponse;
  candidateMatchError?: string;
  persistedWebValidation?: MAWebValidation;
  persistError?: string;
  loadedFromCache?: boolean;
  finalDecision: PipelineFinalDecision;
}

type PipelineReconciliationInput = Omit<PipelineRunResult, 'finalDecision'>;

const dataEnrichmentOptions = [
  { value: '', label: 'Non impostato' },
  { value: 'start', label: 'Start' },
  { value: 'advanced', label: 'Advanced' },
  { value: 'pec', label: 'PEC' },
  { value: 'address', label: 'Address' },
  { value: 'shareholders', label: 'Shareholders' },
  { value: 'name', label: 'Name' },
];

const activityStatusOptions = [
  { value: '', label: 'Tutti' },
  { value: 'ATTIVA', label: 'ATTIVA' },
  { value: 'CESSATA', label: 'CESSATA' },
  { value: 'REGISTRATA', label: 'REGISTRATA' },
  { value: 'INATTIVA', label: 'INATTIVA' },
  { value: 'SOSPESA', label: 'SOSPESA' },
  { value: 'IN_ISCRIZIONE', label: 'IN_ISCRIZIONE' },
];

const textFilterFields: Array<{
  name: CompanySearchFilterField;
  label: string;
  maxLength?: number;
}> = [
  { name: 'companyName', label: 'Nome azienda' },
  { name: 'atecoCode', label: 'ATECO' },
  { name: 'cciaa', label: 'CCIAA', maxLength: 2 },
  { name: 'reaCode', label: 'REA' },
];

const numberFilterFields: Array<{
  name: CompanySearchFilterField;
  label: string;
  min?: number;
  max?: number;
}> = [
  { name: 'minTurnover', label: 'Fatturato min', min: 0 },
  { name: 'maxTurnover', label: 'Fatturato max', min: 0 },
  { name: 'minEmployees', label: 'Dipendenti min', min: 0 },
  { name: 'maxEmployees', label: 'Dipendenti max', min: 0 },
  { name: 'skip', label: 'Skip', min: 0 },
  { name: 'limit', label: 'Limit', min: 1, max: 1000 },
];

const countHints = ['count', 'record', 'total', 'found', 'result'];
const priceHints = ['price', 'cost', 'amount', 'prezzo'];

const validationErrorLabels: Record<string, string> = {
  invalid_province: 'Provincia non valida. Inserisci una sigla di due lettere.',
  invalid_dry_run: 'Valore dry_run non valido.',
  invalid_force_refresh: 'Valore Forza nuova ricerca non valido.',
  invalid_data_enrichment: 'Arricchimento dati non valido. Seleziona una voce disponibile.',
  invalid_activity_status: 'Stato attivita non valido. Seleziona una voce disponibile.',
  invalid_min_turnover: 'Fatturato minimo non valido. Inserisci un numero intero.',
  invalid_max_turnover: 'Fatturato massimo non valido. Inserisci un numero intero.',
  invalid_min_employees: 'Dipendenti minimi non validi. Inserisci un numero intero.',
  invalid_max_employees: 'Dipendenti massimi non validi. Inserisci un numero intero.',
  invalid_skip: 'Skip non valido. Inserisci un numero maggiore o uguale a 0.',
  invalid_limit: 'Limit non valido. Inserisci un numero tra 1 e 1000.',
  invalid_ateco_code: 'Codice ATECO non trovato. Inserisci un codice ATECO 2025 valido.',
  invalid_domain: 'Dominio non valido.',
  missing_keywords: 'Inserisci almeno una keyword.',
  missing_company_name: 'Inserisci la ragione sociale.',
  missing_selected_domain: 'Seleziona o risolvi prima un dominio candidato.',
  query_too_long: 'Query troppo lunga per Brave Search.',
};

function apiErrorCode(error: ApiError): string | undefined {
  const body = error.body as { error?: unknown } | undefined;
  return body && typeof body === 'object' && typeof body.error === 'string' ? body.error : undefined;
}

function errorLabel(error: unknown): string {
  if (error instanceof ApiError) {
    const code = apiErrorCode(error);
    if (error.status === 400 && code && validationErrorLabels[code]) return validationErrorLabels[code];
    if (error.status === 400 && code) return `Richiesta non valida: ${code}.`;
    if (error.status === 400) return 'Richiesta non valida. Controlla i parametri inseriti.';
    if (error.status === 503 && code === 'binocolo_cache_not_configured') {
      return 'La cache Anisetta per Binocolo non e configurata in questo ambiente.';
    }
    if (error.status === 503 && code === 'binocolo_ateco_not_configured') {
      return 'Archivio ATECO Binocolo non configurato in questo ambiente.';
    }
    if (error.status === 503 && code === 'openapiit_not_configured') {
      return 'OpenAPI.it non e configurato in questo ambiente.';
    }
    if (error.status === 503 && code === 'brave_not_configured') {
      return 'Brave Search non e configurato in questo ambiente.';
    }
    if (error.status === 503 && code === 'openrouter_not_configured') {
      return 'LLM non configurato in questo ambiente.';
    }
    if (error.status === 503 && code === 'binocolo_llm_config_not_configured') {
      return 'Scope LLM Binocolo non configurato in Anisetta.';
    }
    if (error.status === 503) return 'Il servizio richiesto non e configurato in questo ambiente.';
    if (error.status === 502) return 'Il servizio esterno non ha risposto correttamente.';
    if (error.status === 401) return 'Sessione non valida.';
    if (error.status === 403) return 'Non hai accesso a Binocolo.';
    return `Richiesta non riuscita (${error.status}).`;
  }
  if (error instanceof Error) return error.message;
  return 'Richiesta non riuscita.';
}

function isRetryableCompanySearchError(error: unknown): boolean {
  if (error instanceof ApiError) {
    const code = apiErrorCode(error);
    if (error.status === 400 || error.status === 401 || error.status === 403) return false;
    if (
      error.status === 503 &&
      (code === 'binocolo_cache_not_configured' ||
        code === 'binocolo_ateco_not_configured' ||
        code === 'openapiit_not_configured')
    ) {
      return false;
    }
  }
  return true;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function displayValue(value: string | null | undefined): string {
  const normalized = value?.trim();
  return normalized ? normalized : '-';
}

function rawPreview(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function normalizeMetricKey(key: string): string {
  return key.replace(/[_\-\s]/g, '').toLowerCase();
}

function findMetric(value: unknown, hints: string[]): string | number | undefined {
  return findMetricInValue(value, hints, new Set<object>());
}

function findMetricInValue(
  value: unknown,
  hints: string[],
  seen: Set<object>,
): string | number | undefined {
  if (!isRecord(value) && !Array.isArray(value)) return undefined;
  if (seen.has(value)) return undefined;
  seen.add(value);

  const entries = Array.isArray(value)
    ? value.map((item, index) => [String(index), item] as const)
    : Object.entries(value);

  for (const [key, nested] of entries) {
    const normalizedKey = normalizeMetricKey(key);
    if (
      hints.some((hint) => normalizedKey.includes(hint)) &&
      (typeof nested === 'number' || typeof nested === 'string')
    ) {
      return nested;
    }
  }

  for (const [, nested] of entries) {
    const found = findMetricInValue(nested, hints, seen);
    if (found !== undefined) return found;
  }

  return undefined;
}

function formatMetric(value: string | number | undefined): string {
  if (typeof value === 'number') return numberFormat.format(value);
  const normalized = value?.trim();
  return normalized ? normalized : 'N/D';
}

function normalizeProvinceInput(value: string): string {
  return value.trim().toUpperCase();
}

function appendSearchParam(
  params: URLSearchParams,
  key: string,
  value: string,
  normalize: (value: string) => string = (item) => item,
) {
  const normalized = normalize(value.trim());
  if (normalized) params.set(key, normalized);
}

function splitKeywords(raw: string): string[] {
  return raw
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

function positiveInteger(value: string, fallback: number, min: number, max: number): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return fallback;
  return Math.max(min, Math.min(max, Math.trunc(parsed)));
}

function clampScore(value: number): number {
  return Math.max(0, Math.min(100, Math.round(value)));
}

function ratioScore(value: number, total: number): number {
  if (total <= 0) return 0;
  return Math.max(0, Math.min(100, (value / total) * 100));
}

function cleanTerm(value: string): string {
  return value.replace(/\s+/g, ' ').trim();
}

function addTerm(list: string[], value: string, limit = 12) {
  const term = cleanTerm(value);
  if (!term || list.length >= limit) return;
  const key = term.toLowerCase();
  if (list.some((item) => item.toLowerCase() === key)) return;
  list.push(term);
}

function addTerms(list: string[], values: string[], limit = 12) {
  for (const value of values) addTerm(list, value, limit);
}

function normalizeAteco(value: string | undefined): string {
  return (value ?? '').replace(/\D/g, '');
}

function strategyAtecoCodes(strategy?: MAStrategySpec, target?: MATarget): string[] {
  const codes = new Set<string>();
  if (target?.atecoCode) codes.add(normalizeAteco(target.atecoCode));
  for (const candidate of strategy?.atecoCandidates ?? []) {
    const code = normalizeAteco(candidate.code);
    if (code && candidate.fit !== 'excluded') codes.add(code);
  }
  return Array.from(codes).filter(Boolean);
}

function hasAtecoPrefix(codes: string[], prefixes: string[]): boolean {
  return codes.some((code) => prefixes.some((prefix) => code.startsWith(prefix)));
}

function derivedKeywordSet(strategy?: MAStrategySpec, target?: MATarget, extraKeywords = ''): PipelineKeywordSet {
  const coreTerms: string[] = [];
  const adjacentTerms: string[] = [];
  const negativeTerms: string[] = [];
  const sources: string[] = [];
  const atecoCodes = strategyAtecoCodes(strategy, target);
  const sector = cleanTerm(strategy?.sectorDescription ?? '');

  if (sector) sources.push(`settore: ${sector}`);
  if (atecoCodes.length > 0) sources.push(`ATECO: ${atecoCodes.join(', ')}`);
  if (target?.atecoDescription) sources.push(`target ATECO: ${target.atecoDescription}`);

  addTerms(coreTerms, splitKeywords(extraKeywords), 10);
  addTerms(coreTerms, strategy?.keywords ?? [], 10);

  if (hasAtecoPrefix(atecoCodes, ['631010', '6310', '631'])) {
    addTerms(coreTerms, [
      'infrastrutture informatiche',
      'hosting',
      'cloud infrastructure',
      'cloud native',
      'data center',
    ]);
    addTerms(adjacentTerms, ['cloud', 'server', 'storage', 'backup', 'virtualizzazione']);
  }

  if (hasAtecoPrefix(atecoCodes, ['622020', '6220', '622'])) {
    addTerms(coreTerms, [
      'gestione strutture informatiche',
      'gestione infrastrutture IT',
      'IT operations',
      'system management',
      'assistenza sistemistica',
    ]);
    addTerms(adjacentTerms, ['networking', 'monitoraggio', 'help desk', 'SLA']);
  }

  if (hasAtecoPrefix(atecoCodes, ['629009', '6290', '629'])) {
    addTerms(coreTerms, [
      'servizi IT',
      'servizi informatici',
      'tecnologie informatiche',
      'information technology services',
    ]);
    addTerms(adjacentTerms, ['cybersecurity', 'Microsoft 365', 'consulenza informatica']);
  }

  const text = `${sector} ${target?.atecoDescription ?? ''}`.toLowerCase();
  if (text.includes('cloud')) addTerm(coreTerms, 'cloud');
  if (text.includes('hosting')) addTerm(coreTerms, 'hosting');
  if (text.includes('infrastrutt')) addTerm(coreTerms, 'infrastrutture informatiche');
  if (text.includes('gestione')) addTerm(coreTerms, 'gestione infrastrutture IT');

  if (coreTerms.length === 0) {
    addTerms(coreTerms, ['servizi IT', 'tecnologie informatiche', 'consulenza informatica']);
    sources.push('fallback: lente IT ampia');
  }

  addTerms(adjacentTerms, ['cloud', 'backup', 'cybersecurity', 'networking', 'virtualizzazione'], 8);
  addTerms(negativeTerms, [
    'web agency',
    'marketing digitale',
    'rivendita hardware',
    'sviluppo software puro',
    'formazione informatica',
  ], 6);

  return {
    intentLabel: sector || 'Lente derivata da strategia M&A',
    coreTerms: coreTerms.slice(0, 10),
    adjacentTerms: adjacentTerms.slice(0, 8),
    negativeTerms: negativeTerms.slice(0, 6),
    sources,
  };
}

function pipelineTermSpecs(keywordSet: PipelineKeywordSet): PipelineTermSpec[] {
  return [
    ...keywordSet.coreTerms.slice(0, 6).map((term) => ({ bucket: 'core' as const, term })),
    ...keywordSet.adjacentTerms.slice(0, 4).map((term) => ({ bucket: 'adjacent' as const, term })),
    ...keywordSet.negativeTerms.slice(0, 3).map((term) => ({ bucket: 'negative' as const, term })),
  ];
}

function chooseDomainCandidate(candidates: DomainResolutionCandidate[]): DomainResolutionCandidate | undefined {
  const credibleReasons = [
    'host compatibile con ragione sociale',
    'dominio citato come sameAs',
    'dominio citato come sito ufficiale',
    'email aziendale su dominio',
  ];
  return candidates.find((candidate) => {
    if (candidate.confidence === 'bassa' || candidate.score < 45) return false;
    return candidate.reasons.some((reason) => credibleReasons.includes(reason));
  });
}

function bestWebSearchScore(response: WebSearchResponse): number | undefined {
  const scores = response.results
    .map((result) => result.score)
    .filter((score): score is number => typeof score === 'number');
  return scores.length > 0 ? Math.max(...scores) : undefined;
}

function evidenceResponseMatched(response: WebSearchResponse): boolean {
  const bestScore = bestWebSearchScore(response);
  if (typeof bestScore === 'number') return bestScore >= 45;
  return !response.ranked && response.results.length > 0;
}

function bucketEvidenceScore(runs: PipelineEvidenceRun[], bucket: EvidenceBucket): number {
  const bucketRuns = runs.filter((run) => run.bucket === bucket);
  if (bucketRuns.length === 0) return 0;
  const matchedRuns = bucketRuns.filter((run) => run.matched);
  const matchedRate = matchedRuns.length / bucketRuns.length;
  const quality =
    matchedRuns.length > 0
      ? matchedRuns.reduce((sum, run) => sum + (run.bestScore ?? 60), 0) / matchedRuns.length
      : 0;

  return clampScore(matchedRate * 70 + quality * 0.3);
}

function domainEvidenceScore(selectedDomain: DomainResolutionCandidate | undefined): number {
  if (!selectedDomain) return 0;
  const confidenceScore =
    selectedDomain.confidence === 'alta'
      ? 100
      : selectedDomain.confidence === 'media'
        ? 70
        : 30;
  return clampScore(selectedDomain.score * 0.75 + confidenceScore * 0.25);
}

function computePipelineSummary(
  selectedDomain: DomainResolutionCandidate | undefined,
  keywordSet: PipelineKeywordSet,
  runs: PipelineEvidenceRun[],
): PipelineSummary {
  const matched = (bucket: EvidenceBucket) =>
    runs.filter((run) => run.bucket === bucket && run.matched).length;
  const searched = (bucket: EvidenceBucket) =>
    runs.filter((run) => run.bucket === bucket).length;
  const coreMatches = matched('core');
  const adjacentMatches = matched('adjacent');
  const negativeMatches = matched('negative');
  const searchedCoreTerms = searched('core');
  const searchedAdjacentTerms = searched('adjacent');
  const searchedNegativeTerms = searched('negative');
  const totalCoreTerms = keywordSet.coreTerms.length;
  const totalAdjacentTerms = keywordSet.adjacentTerms.length;
  const totalNegativeTerms = keywordSet.negativeTerms.length;
  const coreEvidenceScore = bucketEvidenceScore(runs, 'core');
  const adjacentEvidenceScore = bucketEvidenceScore(runs, 'adjacent');
  const sectorEvidenceScore = clampScore(coreEvidenceScore * 0.72 + adjacentEvidenceScore * 0.28);
  const coverageScore = clampScore(
    ratioScore(coreMatches, Math.max(totalCoreTerms, searchedCoreTerms)) * 0.7 +
      ratioScore(adjacentMatches, Math.max(totalAdjacentTerms, searchedAdjacentTerms)) * 0.3,
  );
  const domainScore = domainEvidenceScore(selectedDomain);
  const negativeRate = ratioScore(negativeMatches, Math.max(1, searchedNegativeTerms));
  const negativePenalty = clampScore(negativeMatches * 18 + negativeRate * 0.35);
  const noNegativeScore = 100 - negativePenalty;
  const rawScore =
    sectorEvidenceScore * 0.45 +
    coverageScore * 0.2 +
    domainScore * 0.25 +
    noNegativeScore * 0.1 -
    negativePenalty * 0.35;
  const score = selectedDomain ? clampScore(rawScore) : 0;
  const confidence: PipelineSummary['confidence'] =
    selectedDomain && score >= 70 && coreMatches >= 2 && negativePenalty < 25
      ? 'alta'
      : selectedDomain && score >= 45 && (coreMatches >= 1 || adjacentMatches >= 2)
        ? 'media'
        : 'bassa';

  return {
    score,
    confidence,
    sectorEvidenceScore,
    coverageScore,
    domainScore,
    negativePenalty,
    coreMatches,
    adjacentMatches,
    negativeMatches,
    searchedCoreTerms,
    searchedAdjacentTerms,
    searchedNegativeTerms,
    totalCoreTerms,
    totalAdjacentTerms,
    totalNegativeTerms,
  };
}

function pipelineBucketLabel(bucket: EvidenceBucket): string {
  switch (bucket) {
    case 'core':
      return 'Core';
    case 'adjacent':
      return 'Adiacente';
    case 'negative':
      return 'Negativo';
  }
}

function candidateVerdictLabel(verdict: string): string {
  switch (verdict) {
    case 'strong_match':
      return 'Strong match';
    case 'match':
      return 'Match';
    case 'weak_match':
      return 'Weak match';
    case 'no_match':
      return 'No match';
    case 'unclear':
      return 'Unclear';
    default:
      return verdict || 'N/D';
  }
}

function candidateActionLabel(action: string): string {
  switch (action) {
    case 'confirm':
      return 'Conferma';
    case 'review':
      return 'Review';
    case 'downgrade':
      return 'Downgrade';
    case 'reject':
      return 'Reject';
    default:
      return action || 'N/D';
  }
}

function finalActionLabel(action: PipelineFinalAction): string {
  switch (action) {
    case 'confirm':
      return 'Conferma';
    case 'deprioritize':
      return 'Deprioritizza';
    case 'reject':
      return 'Reject';
    case 'needs_domain_review':
      return 'Review dominio';
    case 'needs_business_validation':
      return 'Validazione business';
  }
}

function webValidationStateLabel(state: PipelineWebValidationState): string {
  switch (state) {
    case 'confirmed':
      return 'Confermato';
    case 'deprioritized':
      return 'Declassato';
    case 'domain_unresolved':
      return 'Dominio non risolto';
    case 'analysis_unavailable':
      return 'Analyst non disponibile';
    case 'rejected':
      return 'Respinto';
    case 'unclear':
      return 'Incerto';
  }
}

function latestStaffCost(target: MATarget): number | undefined {
  if (!isRecord(target.vendorPayload)) return undefined;
  const balanceSheets = target.vendorPayload.balanceSheets;
  if (!isRecord(balanceSheets)) return undefined;
  const last = balanceSheets.last;
  if (!isRecord(last)) return undefined;
  return typeof last.totalStaffCost === 'number' ? last.totalStaffCost : undefined;
}

function targetBusinessRiskSignals(target: MATarget): string[] {
  const signals: string[] = [];
  if (target.employees === 0) signals.push('dipendenti dichiarati pari a 0');
  const staffCost = latestStaffCost(target);
  if (typeof staffCost === 'number' && staffCost <= 1000) signals.push('costo personale nullo o quasi nullo');
  if (target.missingCriteria.some((criterion) => criterion.toLowerCase().includes('produtt'))) {
    signals.push('produttivita non valutabile');
  }
  if (target.flags?.some((flag) => flag.code === 'bilancio_datato')) {
    signals.push('ultimo bilancio datato');
  }
  return signals;
}

function reconcilePipelineDecision(input: PipelineReconciliationInput): PipelineFinalDecision {
  const { target, selectedDomain, summary, candidateMatchAnalysis, candidateMatchError } = input;
  const reasons: string[] = [];
  const deterministicHigh = target.matchState === 'match' && target.score >= 75;
  const businessRisks = targetBusinessRiskSignals(target);
  if (businessRisks.length > 0) reasons.push(...businessRisks.map((risk) => `Rischio business: ${risk}.`));

  if (!selectedDomain) {
    reasons.unshift('Nessun dominio ufficiale credibile risolto.');
    return {
      initialMatchState: target.matchState,
      deterministicScore: target.score,
      webScore: summary.score,
      webValidationState: 'domain_unresolved',
      finalAction: 'needs_domain_review',
      confidence: deterministicHigh ? 'media' : 'bassa',
      reason: 'Score iniziale non validabile senza dominio ufficiale.',
      reasons,
    };
  }

  const weakWebEvidence = summary.score < 45 || summary.coreMatches === 0;
  const moderateWebEvidence = summary.score < 70 || selectedDomain.confidence !== 'alta';
  const negativeEvidence = summary.negativeMatches > 0 || summary.negativePenalty >= 25;

  if (!candidateMatchAnalysis) {
    if (candidateMatchError) reasons.unshift(`Analyst non disponibile: ${candidateMatchError}`);
    if (weakWebEvidence || negativeEvidence) {
      reasons.unshift('Web evidence insufficiente o rumorosa senza validazione LLM.');
    }
    return {
      initialMatchState: target.matchState,
      deterministicScore: target.score,
      webScore: summary.score,
      webValidationState: 'analysis_unavailable',
      finalAction: deterministicHigh && !weakWebEvidence ? 'needs_business_validation' : 'deprioritize',
      confidence: deterministicHigh && !weakWebEvidence ? 'media' : 'bassa',
      reason: deterministicHigh
        ? 'Il candidato resta interessante sulla carta, ma manca il giudizio LLM finale.'
        : 'Validazione incompleta e segnale web non sufficiente per confermare.',
      reasons,
    };
  }

  const analystReject =
    candidateMatchAnalysis.recommendedAction === 'reject' || candidateMatchAnalysis.verdict === 'no_match';
  const analystDowngrade =
    candidateMatchAnalysis.recommendedAction === 'downgrade' || candidateMatchAnalysis.verdict === 'weak_match';
  const analystReview =
    candidateMatchAnalysis.recommendedAction === 'review' ||
    candidateMatchAnalysis.verdict === 'unclear' ||
    candidateMatchAnalysis.confidence === 'bassa';
  const analystConfirm =
    candidateMatchAnalysis.recommendedAction === 'confirm' &&
    (candidateMatchAnalysis.verdict === 'strong_match' || candidateMatchAnalysis.verdict === 'match');

  if (candidateMatchAnalysis.evidenceAgainst.length > 0) {
    reasons.push(...candidateMatchAnalysis.evidenceAgainst.map((item) => `Contro: ${item}`));
  }
  if (candidateMatchAnalysis.missingEvidence.length > 0) {
    reasons.push(...candidateMatchAnalysis.missingEvidence.map((item) => `Lacuna: ${item}`));
  }
  if (candidateMatchAnalysis.negativeSignals.length > 0) {
    reasons.push(...candidateMatchAnalysis.negativeSignals.map((item) => `Segnale negativo: ${item}`));
  }
  if (negativeEvidence) reasons.unshift('La web evidence contiene segnali negativi o penalty rilevante.');
  if (weakWebEvidence) reasons.unshift('La web evidence non copre i termini core.');

  if (analystConfirm && summary.score >= 70 && !negativeEvidence && businessRisks.length < 2) {
    if (moderateWebEvidence) {
      reasons.unshift('Analyst positivo, ma dominio/copertura non sono abbastanza forti per conferma automatica.');
      return {
        initialMatchState: target.matchState,
        deterministicScore: target.score,
        webScore: summary.score,
        webValidationState: 'unclear',
        finalAction: 'needs_business_validation',
        confidence: 'media',
        reason: 'Match promettente, da validare prima di promuoverlo.',
        reasons,
        analystVerdict: candidateMatchAnalysis.verdict,
        analystAction: candidateMatchAnalysis.recommendedAction,
      };
    }
    reasons.unshift('Score iniziale, web evidence e analyst sono coerenti.');
    return {
      initialMatchState: target.matchState,
      deterministicScore: target.score,
      webScore: summary.score,
      webValidationState: 'confirmed',
      finalAction: 'confirm',
      confidence: candidateMatchAnalysis.confidence === 'alta' ? 'alta' : 'media',
      reason: 'Candidato confermato dalla validazione web/LLM.',
      reasons,
      analystVerdict: candidateMatchAnalysis.verdict,
      analystAction: candidateMatchAnalysis.recommendedAction,
    };
  }

  if (analystReject) {
    const hardReject = !deterministicHigh || summary.score < 30 || negativeEvidence;
    reasons.unshift('Analyst orientato al reject.');
    return {
      initialMatchState: target.matchState,
      deterministicScore: target.score,
      webScore: summary.score,
      webValidationState: hardReject ? 'rejected' : 'deprioritized',
      finalAction: hardReject ? 'reject' : 'deprioritize',
      confidence: hardReject ? 'alta' : 'media',
      reason: hardReject
        ? 'Il candidato non supera la validazione web/LLM.'
        : 'Score camerale alto, ma validazione web/LLM contraria: non lavorarlo in priorita.',
      reasons,
      analystVerdict: candidateMatchAnalysis.verdict,
      analystAction: candidateMatchAnalysis.recommendedAction,
    };
  }

  if (analystDowngrade || weakWebEvidence || businessRisks.length >= 2) {
    reasons.unshift(
      deterministicHigh
        ? 'Score camerale alto, ma web evidence/LLM non confermano abbastanza.'
        : 'Web evidence/LLM non supportano il match iniziale.',
    );
    return {
      initialMatchState: target.matchState,
      deterministicScore: target.score,
      webScore: summary.score,
      webValidationState: 'deprioritized',
      finalAction: deterministicHigh ? 'needs_business_validation' : 'deprioritize',
      confidence: deterministicHigh ? 'media' : 'bassa',
      reason: deterministicHigh
        ? 'Buona candidata sulla carta, non confermata dalla validazione web/LLM.'
        : 'Candidato declassato dalla validazione web/LLM.',
      reasons,
      analystVerdict: candidateMatchAnalysis.verdict,
      analystAction: candidateMatchAnalysis.recommendedAction,
    };
  }

  if (analystReview || moderateWebEvidence) {
    reasons.unshift('Validazione non conclusiva.');
    return {
      initialMatchState: target.matchState,
      deterministicScore: target.score,
      webScore: summary.score,
      webValidationState: 'unclear',
      finalAction: deterministicHigh ? 'needs_business_validation' : 'deprioritize',
      confidence: 'media',
      reason: 'Serve revisione business prima di decidere.',
      reasons,
      analystVerdict: candidateMatchAnalysis.verdict,
      analystAction: candidateMatchAnalysis.recommendedAction,
    };
  }

  reasons.unshift('Nessun blocco forte, ma manca allineamento pieno per conferma.');
  return {
    initialMatchState: target.matchState,
    deterministicScore: target.score,
    webScore: summary.score,
    webValidationState: 'unclear',
    finalAction: 'needs_business_validation',
    confidence: 'media',
    reason: 'Decisione prudente: validazione manuale richiesta.',
    reasons,
    analystVerdict: candidateMatchAnalysis.verdict,
    analystAction: candidateMatchAnalysis.recommendedAction,
  };
}

function stringListsEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((item, index) => item === b[index]);
}

function pipelineKeywordSetsEqual(a: PipelineKeywordSet, b: PipelineKeywordSet): boolean {
  return (
    a.intentLabel === b.intentLabel &&
    stringListsEqual(a.coreTerms, b.coreTerms) &&
    stringListsEqual(a.adjacentTerms, b.adjacentTerms) &&
    stringListsEqual(a.negativeTerms, b.negativeTerms) &&
    stringListsEqual(a.sources, b.sources)
  );
}

function pipelineRunFromWebValidation(target: MATarget, validation: MAWebValidation): PipelineRunResult {
  return {
    target,
    keywordSet: validation.keywordSet as PipelineKeywordSet,
    domainResponse: validation.domainResponse,
    selectedDomain: validation.selectedDomainPayload,
    evidenceRuns: validation.evidenceRuns as PipelineEvidenceRun[],
    summary: validation.summary as PipelineSummary,
    candidateMatchAnalysis: validation.candidateMatchAnalysis,
    candidateMatchError: validation.candidateMatchError || undefined,
    persistedWebValidation: validation,
    loadedFromCache: true,
    finalDecision: validation.finalDecision,
  };
}

function targetOptionLabel(target: MATarget): string {
  const bits = [
    target.companyName,
    target.province,
    target.atecoCode,
    `score ${target.score}`,
  ].filter(Boolean);
  return bits.join(' · ');
}

function companyFilterSummary(filters: CompanySearchFilters): string {
  const parts: string[] = [];
  const province = normalizeProvinceInput(filters.province);
  const status = filters.activityStatus.trim().toUpperCase();
  const name = filters.companyName.trim();

  if (province) parts.push(`provincia ${province}`);
  if (status) parts.push(`stato ${status}`);
  if (name) parts.push(`nome ${name}`);
  if (filters.atecoCode.trim()) parts.push(`ATECO ${filters.atecoCode.trim()}`);
  if (filters.cciaa.trim()) parts.push(`CCIAA ${filters.cciaa.trim().toUpperCase()}`);
  if (filters.reaCode.trim()) parts.push(`REA ${filters.reaCode.trim()}`);

  return parts.length > 0 ? parts.join(', ') : 'tutte le aziende';
}

export function TestPage() {
  const api = useApiClient();
  const [activeTab, setActiveTab] = useState<TestTab>('company');
  const [companyFilters, setCompanyFilters] = useState<CompanySearchFilters>(defaultCompanyFilters);
  const [companyDryRun, setCompanyDryRun] = useState(true);
  const [companyForceRefresh, setCompanyForceRefresh] = useState(false);
  const [domainForm, setDomainForm] = useState<DomainResolutionForm>(defaultDomainResolutionForm);
  const [keywordForm, setKeywordForm] = useState<KeywordEvidenceForm>(defaultKeywordEvidenceForm);
  const [pipelineVisibility, setPipelineVisibility] = useState<MASessionVisibility>('active');
  const [pipelineSessionId, setPipelineSessionId] = useState('');
  const [pipelineTargetId, setPipelineTargetId] = useState('');
  const [pipelineExtraKeywords, setPipelineExtraKeywords] = useState('');
  const [pipelineDomainCount, setPipelineDomainCount] = useState('10');
  const [pipelineKeywordCount, setPipelineKeywordCount] = useState('5');
  const [pipelineIncludeIdentifiers, setPipelineIncludeIdentifiers] = useState(false);
  const [pipelineRank, setPipelineRank] = useState(true);
  const [pipelineAnalyzeWithLLM, setPipelineAnalyzeWithLLM] = useState(true);
  const [pipelineForceRecompute, setPipelineForceRecompute] = useState(false);

  const pipelineSessions = useQuery({
    queryKey: ['binocolo-test-ma-sessions', pipelineVisibility],
    queryFn: () =>
      api.get<MASessionListResponse>(`/binocolo/v1/ma/sessions?visibility=${pipelineVisibility}`),
    enabled: activeTab === 'pipeline',
  });

  const pipelineDetail = useQuery({
    queryKey: ['binocolo-test-ma-session', pipelineSessionId],
    queryFn: () => api.get<MASessionDetail>(`/binocolo/v1/ma/sessions/${pipelineSessionId}`),
    enabled: activeTab === 'pipeline' && Boolean(pipelineSessionId),
  });

  const companySearch = useMutation({
    mutationFn: (forceRefresh: boolean = companyForceRefresh) => {
      const params = new URLSearchParams({ dry_run: String(companyDryRun) });
      if (forceRefresh) params.set('force_refresh', 'true');
      appendSearchParam(params, 'province', companyFilters.province, normalizeProvinceInput);
      appendSearchParam(params, 'dataEnrichment', companyFilters.dataEnrichment);
      appendSearchParam(params, 'activityStatus', companyFilters.activityStatus, (value) => value.toUpperCase());
      appendSearchParam(params, 'companyName', companyFilters.companyName);
      appendSearchParam(params, 'atecoCode', companyFilters.atecoCode);
      appendSearchParam(params, 'cciaa', companyFilters.cciaa, (value) => value.toUpperCase());
      appendSearchParam(params, 'reaCode', companyFilters.reaCode);
      appendSearchParam(params, 'minTurnover', companyFilters.minTurnover);
      appendSearchParam(params, 'maxTurnover', companyFilters.maxTurnover);
      appendSearchParam(params, 'minEmployees', companyFilters.minEmployees);
      appendSearchParam(params, 'maxEmployees', companyFilters.maxEmployees);
      appendSearchParam(params, 'skip', companyFilters.skip);
      appendSearchParam(params, 'limit', companyFilters.limit);
      return api.get<OpenAPIITEnvelope<unknown>>(`/binocolo/v1/companies/search?${params.toString()}`);
    },
    onSettled: (_data, _error, forceRefresh) => {
      if (forceRefresh) setCompanyForceRefresh(false);
    },
  });

  const recompute = useMutation({
    mutationFn: () => api.post<{ recomputed: number }>('/binocolo/v1/ma/deep/recompute', {}),
  });
  const regenerateBriefs = useMutation({
    mutationFn: () => api.post<{ regenerated: number }>('/binocolo/v1/ma/deep/regenerate-briefs', {}),
  });
  const domainResolution = useMutation({
    mutationFn: () =>
      api.post<DomainResolutionResponse>('/binocolo/v1/test/domain-resolution', {
        companyName: domainForm.companyName.trim(),
        vatCode: domainForm.vatCode.trim() || undefined,
        taxCode: domainForm.taxCode.trim() || undefined,
        town: domainForm.town.trim() || undefined,
        province: domainForm.province.trim().toUpperCase() || undefined,
        keywords: splitKeywords(domainForm.keywords),
        count: positiveInteger(domainForm.count, 10, 1, 20),
      }),
  });
  const keywordEvidence = useMutation({
    mutationFn: () =>
      api.post<WebSearchResponse>('/binocolo/v1/web-search', {
        domain: keywordForm.domain.trim(),
        keywords: splitKeywords(keywordForm.keywords),
        count: positiveInteger(keywordForm.count, 10, 1, 50),
        rank: keywordForm.rank,
      }),
  });
  const evidencePipeline = useMutation({
    mutationFn: async (): Promise<PipelineRunResult> => {
      const detail = pipelineDetail.data;
      if (!detail) throw new Error('Seleziona una sessione M&A.');
      const target = detail.targets.find((item) => item.id === pipelineTargetId) ?? detail.targets[0];
      if (!target) throw new Error('La sessione selezionata non contiene target.');

      const keywordSet = derivedKeywordSet(detail.strategy?.strategy, target, pipelineExtraKeywords);
      if (
        target.webValidation &&
        !pipelineForceRecompute &&
        pipelineKeywordSetsEqual(keywordSet, target.webValidation.keywordSet as PipelineKeywordSet)
      ) {
        return pipelineRunFromWebValidation(target, target.webValidation);
      }

      const domainResponse = await api.post<DomainResolutionResponse>('/binocolo/v1/test/domain-resolution', {
        companyName: target.companyName,
        vatCode: pipelineIncludeIdentifiers ? target.vatCode : undefined,
        taxCode: pipelineIncludeIdentifiers ? target.taxCode : undefined,
        town: target.town || undefined,
        province: target.province || undefined,
        keywords: [...keywordSet.coreTerms.slice(0, 3), ...keywordSet.adjacentTerms.slice(0, 2)],
        count: positiveInteger(pipelineDomainCount, 10, 1, 20),
      });
      const selectedDomain = chooseDomainCandidate(domainResponse.candidates);
      const specs = selectedDomain ? pipelineTermSpecs(keywordSet) : [];
      const evidenceRuns = await Promise.all(
        specs.map(async (spec): Promise<PipelineEvidenceRun> => {
          try {
            const response = await api.post<WebSearchResponse>('/binocolo/v1/web-search', {
              domain: selectedDomain?.domain ?? '',
              keywords: [spec.term],
              count: positiveInteger(pipelineKeywordCount, 5, 1, 20),
              rank: pipelineRank,
            });
            const bestScore = bestWebSearchScore(response);
            return {
              ...spec,
              response,
              resultCount: response.results.length,
              bestScore,
              matched: evidenceResponseMatched(response),
            };
          } catch (err) {
            return { ...spec, error: errorLabel(err), resultCount: 0, matched: false };
          }
        }),
      );

      const result: PipelineReconciliationInput = {
        target,
        keywordSet,
        domainResponse,
        selectedDomain,
        evidenceRuns,
        summary: computePipelineSummary(selectedDomain, keywordSet, evidenceRuns),
      };

      if (selectedDomain && pipelineAnalyzeWithLLM) {
        try {
          result.candidateMatchAnalysis = await api.post<CandidateMatchAnalysisResponse>(
            '/binocolo/v1/test/candidate-match-analysis',
            result,
          );
        } catch (err) {
          result.candidateMatchError = errorLabel(err);
        }
      }

      const finalResult: PipelineRunResult = {
        ...result,
        finalDecision: result.candidateMatchAnalysis?.finalDecision ?? reconcilePipelineDecision(result),
      };
      try {
        finalResult.persistedWebValidation = await api.put<MAWebValidation>(
          `/binocolo/v1/ma/sessions/${target.sessionId}/web-validation`,
          finalResult,
        );
        await pipelineDetail.refetch();
      } catch (err) {
        finalResult.persistError = errorLabel(err);
      }
      return finalResult;
    },
  });

  const companyData = companySearch.data?.data;
  const companyRows: CompanySearchRow[] = Array.isArray(companyData)
    ? (companyData as CompanySearchRow[])
    : [];
  const dryRunCount = findMetric(companySearch.data, countHints);
  const dryRunPrice = findMetric(companySearch.data, priceHints);
  const companyScopeLabel = companyFilterSummary(companyFilters);
  const selectableSessions = pipelineSessions.data?.items ?? [];
  const selectedPipelineDetail = pipelineDetail.data;
  const pipelineTargets = useMemo(() => {
    return [...(selectedPipelineDetail?.targets ?? [])].sort((a, b) => {
      if ((b.rating ?? 0) !== (a.rating ?? 0)) return (b.rating ?? 0) - (a.rating ?? 0);
      if (b.score !== a.score) return b.score - a.score;
      return a.companyName.localeCompare(b.companyName);
    });
  }, [selectedPipelineDetail?.targets]);
  const selectedPipelineTarget =
    pipelineTargets.find((target) => target.id === pipelineTargetId) ?? pipelineTargets[0];
  const pipelineKeywordSet = useMemo(
    () => derivedKeywordSet(selectedPipelineDetail?.strategy?.strategy, selectedPipelineTarget, pipelineExtraKeywords),
    [pipelineExtraKeywords, selectedPipelineDetail?.strategy?.strategy, selectedPipelineTarget],
  );

  useEffect(() => {
    if (activeTab !== 'pipeline' || pipelineSessionId || selectableSessions.length === 0) return;
    const withTargets = selectableSessions.find((session) => session.resultCount > 0) ?? selectableSessions.at(0);
    if (!withTargets) return;
    setPipelineSessionId(withTargets.id);
  }, [activeTab, pipelineSessionId, selectableSessions]);

  useEffect(() => {
    if (activeTab !== 'pipeline') return;
    if (pipelineTargets.length === 0) {
      if (pipelineTargetId) setPipelineTargetId('');
      return;
    }
    const firstTarget = pipelineTargets.at(0);
    if (firstTarget && !pipelineTargets.some((target) => target.id === pipelineTargetId)) {
      setPipelineTargetId(firstTarget.id);
    }
  }, [activeTab, pipelineTargetId, pipelineTargets]);

  function handleCompanyDryRunChange(value: boolean) {
    setCompanyDryRun(value);
    companySearch.reset();
  }

  function handleCompanyForceRefreshChange(value: boolean) {
    setCompanyForceRefresh(value);
    companySearch.reset();
  }

  function handleCompanyFilterChange(field: CompanySearchFilterField) {
    return (event: ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
      setCompanyFilters((current) => ({
        ...current,
        [field]: event.target.value,
      }));
      companySearch.reset();
    };
  }

  function handleCompanySubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    companySearch.mutate(companyForceRefresh);
  }

  function handleDomainFieldChange(field: DomainResolutionField) {
    return (event: ChangeEvent<HTMLInputElement>) => {
      setDomainForm((current) => ({ ...current, [field]: event.target.value }));
      domainResolution.reset();
    };
  }

  function handleDomainSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    domainResolution.mutate();
  }

  function useCandidateDomain(domain: string) {
    setKeywordForm((current) => ({ ...current, domain }));
    setActiveTab('keyword');
  }

  function handleKeywordFieldChange(field: KeywordEvidenceField) {
    return (event: ChangeEvent<HTMLInputElement>) => {
      setKeywordForm((current) => ({ ...current, [field]: event.target.value }));
      keywordEvidence.reset();
    };
  }

  function handleKeywordRankChange(value: boolean) {
    setKeywordForm((current) => ({ ...current, rank: value }));
    keywordEvidence.reset();
  }

  function handleKeywordSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    keywordEvidence.mutate();
  }

  function handlePipelineVisibilityChange(event: ChangeEvent<HTMLSelectElement>) {
    setPipelineVisibility(event.target.value as MASessionVisibility);
    setPipelineSessionId('');
    setPipelineTargetId('');
    evidencePipeline.reset();
  }

  function handlePipelineSessionChange(event: ChangeEvent<HTMLSelectElement>) {
    setPipelineSessionId(event.target.value);
    setPipelineTargetId('');
    evidencePipeline.reset();
  }

  function handlePipelineTargetChange(event: ChangeEvent<HTMLSelectElement>) {
    setPipelineTargetId(event.target.value);
    evidencePipeline.reset();
  }

  function handlePipelineSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    evidencePipeline.mutate();
  }

  return (
    <div className={styles.page}>
      <div className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>Test</h1>
          <p className={styles.pageSubtitle}>Laboratorio per API e funzioni sperimentali Binocolo.</p>
        </div>
      </div>

      <div className={styles.tabBar} role="tablist" aria-label="Funzioni test">
        {testTabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={activeTab === tab.id}
            className={`${styles.tabButton} ${activeTab === tab.id ? styles.tabButtonActive : ''}`}
            onClick={() => setActiveTab(tab.id)}
          >
            <Icon name={tab.icon} size={15} />
            <span>{tab.label}</span>
          </button>
        ))}
      </div>

      {activeTab === 'company' ? (
      <section className={`${styles.panel} ${styles.companyPanel}`} aria-labelledby="companies-title">
        <div className={styles.panelHeader}>
          <div>
            <div className={styles.endpointLine}>
              <span className={styles.method}>GET</span>
              <span className={styles.path}>/binocolo/v1/companies/search</span>
            </div>
            <h2 id="companies-title" className={styles.sectionTitle}>Company search</h2>
          </div>
          <form className={styles.companyForm} onSubmit={handleCompanySubmit}>
            <div className={styles.filterGrid}>
              <label className={styles.filterField}>
                <span>Provincia</span>
                <input
                  type="text"
                  value={companyFilters.province}
                  onChange={handleCompanyFilterChange('province')}
                  maxLength={2}
                  autoCapitalize="characters"
                  autoComplete="off"
                  spellCheck={false}
                  aria-label="Provincia"
                  className={styles.codeInput}
                />
              </label>
              <label className={styles.filterField}>
                <span>Data enrichment</span>
                <select
                  value={companyFilters.dataEnrichment}
                  onChange={handleCompanyFilterChange('dataEnrichment')}
                  aria-label="Data enrichment"
                >
                  {dataEnrichmentOptions.map((option) => (
                    <option key={option.value || 'empty'} value={option.value}>{option.label}</option>
                  ))}
                </select>
              </label>
              <label className={styles.filterField}>
                <span>Stato</span>
                <select
                  value={companyFilters.activityStatus}
                  onChange={handleCompanyFilterChange('activityStatus')}
                  aria-label="Stato"
                >
                  {activityStatusOptions.map((option) => (
                    <option key={option.value || 'empty'} value={option.value}>{option.label}</option>
                  ))}
                </select>
              </label>
              {textFilterFields.map((field) => (
                <label key={field.name} className={styles.filterField}>
                  <span>{field.label}</span>
                  <input
                    type="text"
                    value={companyFilters[field.name]}
                    onChange={handleCompanyFilterChange(field.name)}
                    maxLength={field.maxLength}
                    autoComplete="off"
                    spellCheck={false}
                    className={field.name === 'cciaa' ? styles.codeInput : undefined}
                  />
                </label>
              ))}
              {numberFilterFields.map((field) => (
                <label key={field.name} className={styles.filterField}>
                  <span>{field.label}</span>
                  <input
                    type="number"
                    value={companyFilters[field.name]}
                    onChange={handleCompanyFilterChange(field.name)}
                    min={field.min}
                    max={field.max}
                    step="1"
                    inputMode="numeric"
                  />
                </label>
              ))}
            </div>
            <div className={styles.formActions}>
              <label className={styles.dryRunField}>
                <span>dry_run</span>
                <ToggleSwitch
                  id="binocolo-company-dry-run"
                  checked={companyDryRun}
                  onChange={handleCompanyDryRunChange}
                />
              </label>
              <label className={styles.dryRunField}>
                <span>Forza nuova ricerca</span>
                <ToggleSwitch
                  id="binocolo-company-force-refresh"
                  checked={companyForceRefresh}
                  onChange={handleCompanyForceRefreshChange}
                />
              </label>
              <Button
                type="submit"
                loading={companySearch.isPending}
                leftIcon={<Icon name="search" />}
              >
                Esegui
              </Button>
            </div>
          </form>
        </div>

        {companySearch.isIdle ? (
          <div className={`${styles.statePanel} ${styles.companyState}`}>
            <div className={styles.stateIcon}>
              <Icon name="search" size={22} />
            </div>
            <p className={styles.stateTitle}>Ricerca pronta</p>
            <p className={styles.stateText}>Premi Esegui per interrogare le aziende: {companyScopeLabel}.</p>
          </div>
        ) : companySearch.isPending ? (
          <div className={styles.skeletonWrap}>
            <Skeleton rows={5} />
          </div>
        ) : companySearch.isError ? (
          <div className={`${styles.statePanel} ${styles.companyState}`} role="alert">
            <div className={styles.stateIcon}>
              <Icon name="triangle-alert" size={22} />
            </div>
            <p className={styles.stateTitle}>Ricerca non disponibile</p>
            <p className={styles.stateText}>{errorLabel(companySearch.error)}</p>
            {isRetryableCompanySearchError(companySearch.error) ? (
              <Button variant="secondary" size="sm" onClick={() => companySearch.mutate(companyForceRefresh)}>
                Riprova
              </Button>
            ) : null}
          </div>
        ) : companyDryRun ? (
          <div className={styles.companyResult}>
            <div className={styles.responseBar}>
              <span>Simulazione: {companyScopeLabel}</span>
              {companySearch.data?.message ? <span>{companySearch.data.message}</span> : null}
            </div>
            <div className={styles.dryRunGrid}>
              <div className={styles.metricBox}>
                <span>Risultati</span>
                <strong>{formatMetric(dryRunCount)}</strong>
              </div>
              <div className={styles.metricBox}>
                <span>Prezzo</span>
                <strong>{formatMetric(dryRunPrice)}</strong>
              </div>
            </div>
            <div className={styles.rawBlock}>
              <span>Risposta</span>
              <pre>{rawPreview(companySearch.data)}</pre>
            </div>
          </div>
        ) : companyRows.length === 0 ? (
          <div className={`${styles.statePanel} ${styles.companyState}`}>
            <div className={styles.stateIcon}>
              <Icon name="search" size={22} />
            </div>
            <p className={styles.stateTitle}>Nessuna azienda trovata</p>
            <p className={styles.stateText}>La ricerca non ha restituito aziende per {companyScopeLabel}.</p>
          </div>
        ) : (
          <>
            <div className={styles.responseBar}>
              <span>{companyRows.length} aziende visualizzate</span>
              {companySearch.data?.message ? <span>{companySearch.data.message}</span> : null}
            </div>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Azienda</th>
                    <th>Partita IVA</th>
                    <th>Codice fiscale</th>
                    <th>Comune</th>
                    <th>Stato</th>
                    <th>ID</th>
                  </tr>
                </thead>
                <tbody>
                  {companyRows.map((company, index) => (
                    <tr key={company.id || `${company.companyName ?? 'company'}-${index}`}>
                      <td>{displayValue(company.companyName)}</td>
                      <td className={styles.codeCell}>{displayValue(company.vatCode)}</td>
                      <td className={styles.codeCell}>{displayValue(company.taxCode)}</td>
                      <td>{displayValue(company.address?.registeredOffice?.town)}</td>
                      <td>{displayValue(company.activityStatus)}</td>
                      <td className={styles.codeCell}>{displayValue(company.id)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className={`${styles.rawBlock} ${styles.rawBlockSeparated}`}>
              <span>Risposta</span>
              <pre>{rawPreview(companySearch.data)}</pre>
            </div>
          </>
        )}
      </section>
      ) : null}

      {activeTab === 'pipeline' ? (
      <section className={`${styles.panel} ${styles.companyPanel}`} aria-labelledby="pipeline-title">
        <div className={styles.panelHeader}>
          <div>
            <div className={styles.endpointLine}>
              <span className={styles.method}>LAB</span>
              <span className={styles.path}>session target pick · domain resolution · site keyword evidence</span>
            </div>
            <h2 id="pipeline-title" className={styles.sectionTitle}>Evidence pipeline</h2>
          </div>
          <form className={styles.companyForm} onSubmit={handlePipelineSubmit}>
            <div className={styles.filterGrid}>
              <label className={styles.filterField}>
                <span>Sessioni</span>
                <select
                  value={pipelineVisibility}
                  onChange={handlePipelineVisibilityChange}
                  disabled={pipelineSessions.isFetching}
                >
                  {sessionVisibilityOptions.map((option) => (
                    <option key={option.value} value={option.value}>{option.label}</option>
                  ))}
                </select>
              </label>
              <label className={`${styles.filterField} ${styles.fieldWide}`}>
                <span>Sessione</span>
                <select
                  value={pipelineSessionId}
                  onChange={handlePipelineSessionChange}
                  disabled={pipelineSessions.isFetching || selectableSessions.length === 0}
                >
                  <option value="">Seleziona sessione</option>
                  {selectableSessions.map((session) => (
                    <option key={session.id} value={session.id}>
                      {session.title || session.prompt.slice(0, 72)} · {session.resultCount} target · {session.status}
                    </option>
                  ))}
                </select>
              </label>
              <label className={`${styles.filterField} ${styles.fieldWide}`}>
                <span>Target</span>
                <select
                  value={pipelineTargetId}
                  onChange={handlePipelineTargetChange}
                  disabled={pipelineDetail.isFetching || pipelineTargets.length === 0}
                >
                  <option value="">Seleziona target</option>
                  {pipelineTargets.map((target) => (
                    <option key={target.id} value={target.id}>{targetOptionLabel(target)}</option>
                  ))}
                </select>
              </label>
              <label className={`${styles.filterField} ${styles.fieldWide}`}>
                <span>Keyword core extra</span>
                <input
                  type="text"
                  value={pipelineExtraKeywords}
                  onChange={(event) => {
                    setPipelineExtraKeywords(event.target.value);
                    evidencePipeline.reset();
                  }}
                  placeholder="cloud, hosting, infrastrutture informatiche"
                  autoComplete="off"
                  spellCheck={false}
                />
              </label>
              <label className={styles.filterField}>
                <span>Resolver count</span>
                <input
                  type="number"
                  min={1}
                  max={20}
                  step={1}
                  value={pipelineDomainCount}
                  onChange={(event) => {
                    setPipelineDomainCount(event.target.value);
                    evidencePipeline.reset();
                  }}
                />
              </label>
              <label className={styles.filterField}>
                <span>Evidence count</span>
                <input
                  type="number"
                  min={1}
                  max={20}
                  step={1}
                  value={pipelineKeywordCount}
                  onChange={(event) => {
                    setPipelineKeywordCount(event.target.value);
                    evidencePipeline.reset();
                  }}
                />
              </label>
            </div>
            <div className={styles.formActions}>
              <label className={styles.dryRunField}>
                <span>Usa P.IVA/CF</span>
                <ToggleSwitch
                  id="binocolo-pipeline-identifiers"
                  checked={pipelineIncludeIdentifiers}
                  onChange={(value) => {
                    setPipelineIncludeIdentifiers(value);
                    evidencePipeline.reset();
                  }}
                />
              </label>
              <label className={styles.dryRunField}>
                <span>Rank AI</span>
                <ToggleSwitch
                  id="binocolo-pipeline-rank"
                  checked={pipelineRank}
                  onChange={(value) => {
                    setPipelineRank(value);
                    evidencePipeline.reset();
                  }}
                />
              </label>
              <label className={styles.dryRunField}>
                <span>Analisi LLM</span>
                <ToggleSwitch
                  id="binocolo-pipeline-analysis"
                  checked={pipelineAnalyzeWithLLM}
                  onChange={(value) => {
                    setPipelineAnalyzeWithLLM(value);
                    evidencePipeline.reset();
                  }}
                />
              </label>
              <label className={styles.dryRunField}>
                <span>Forza ricalcolo</span>
                <ToggleSwitch
                  id="binocolo-pipeline-force-recompute"
                  checked={pipelineForceRecompute}
                  onChange={(value) => {
                    setPipelineForceRecompute(value);
                    evidencePipeline.reset();
                  }}
                />
              </label>
              <Button
                type="submit"
                loading={evidencePipeline.isPending}
                disabled={!selectedPipelineTarget || pipelineDetail.isFetching}
                leftIcon={<Icon name="route" />}
              >
                Esegui pipeline
              </Button>
            </div>
          </form>
        </div>

        {pipelineSessions.isFetching && !pipelineSessions.data ? (
          <div className={styles.skeletonWrap}>
            <Skeleton rows={5} />
          </div>
        ) : pipelineSessions.isError ? (
          <div className={`${styles.statePanel} ${styles.companyState}`} role="alert">
            <div className={styles.stateIcon}>
              <Icon name="triangle-alert" size={22} />
            </div>
            <p className={styles.stateTitle}>Sessioni non disponibili</p>
            <p className={styles.stateText}>{errorLabel(pipelineSessions.error)}</p>
          </div>
        ) : selectableSessions.length === 0 ? (
          <div className={`${styles.statePanel} ${styles.companyState}`}>
            <div className={styles.stateIcon}>
              <Icon name="database" size={22} />
            </div>
            <p className={styles.stateTitle}>Nessuna sessione</p>
            <p className={styles.stateText}>Non ci sono sessioni M&A nella visibilita selezionata.</p>
          </div>
        ) : pipelineDetail.isFetching && !pipelineDetail.data ? (
          <div className={styles.skeletonWrap}>
            <Skeleton rows={5} />
          </div>
        ) : pipelineDetail.isError ? (
          <div className={`${styles.statePanel} ${styles.companyState}`} role="alert">
            <div className={styles.stateIcon}>
              <Icon name="triangle-alert" size={22} />
            </div>
            <p className={styles.stateTitle}>Sessione non disponibile</p>
            <p className={styles.stateText}>{errorLabel(pipelineDetail.error)}</p>
          </div>
        ) : !selectedPipelineTarget ? (
          <div className={`${styles.statePanel} ${styles.companyState}`}>
            <div className={styles.stateIcon}>
              <Icon name="search" size={22} />
            </div>
            <p className={styles.stateTitle}>Nessun target</p>
            <p className={styles.stateText}>La sessione selezionata non contiene target su cui eseguire la pipeline.</p>
          </div>
        ) : (
          <div className={styles.companyResult}>
            <div className={styles.pipelinePrep}>
              <article className={styles.resultCard}>
                <div className={styles.resultCardHead}>
                  <div>
                    <h3>{selectedPipelineTarget.companyName}</h3>
                    <p>
                      {[
                        selectedPipelineTarget.town,
                        selectedPipelineTarget.province,
                        selectedPipelineTarget.atecoCode,
                        selectedPipelineTarget.atecoDescription,
                      ].filter(Boolean).join(' · ') || 'Target selezionato'}
                    </p>
                  </div>
                  <span className={styles.scoreBadge}>
                    {selectedPipelineTarget.score} · {selectedPipelineTarget.matchState}
                  </span>
                </div>
                <div className={styles.pipelineFacts}>
                  <span>P.IVA {displayValue(selectedPipelineTarget.vatCode)}</span>
                  <span>CF {displayValue(selectedPipelineTarget.taxCode)}</span>
                  <span>{selectedPipelineTarget.confidence ? `confidenza ${selectedPipelineTarget.confidence}` : 'confidenza N/D'}</span>
                </div>
              </article>

              <div className={styles.keywordGroups}>
                <div className={styles.keywordGroup}>
                  <span>Core</span>
                  <div className={styles.termChips}>
                    {pipelineKeywordSet.coreTerms.map((term) => <span key={term} className={styles.termChip}>{term}</span>)}
                  </div>
                </div>
                <div className={styles.keywordGroup}>
                  <span>Adiacenti</span>
                  <div className={styles.termChips}>
                    {pipelineKeywordSet.adjacentTerms.map((term) => <span key={term} className={styles.termChip}>{term}</span>)}
                  </div>
                </div>
                <div className={styles.keywordGroup}>
                  <span>Negativi</span>
                  <div className={styles.termChips}>
                    {pipelineKeywordSet.negativeTerms.map((term) => <span key={term} className={`${styles.termChip} ${styles.termChipNegative}`}>{term}</span>)}
                  </div>
                </div>
              </div>
            </div>

            {evidencePipeline.isIdle ? (
              <div className={`${styles.statePanel} ${styles.companyState}`}>
                <div className={styles.stateIcon}>
                  <Icon name="route" size={22} />
                </div>
                <p className={styles.stateTitle}>Pipeline pronta</p>
                <p className={styles.stateText}>Esegue domain resolution e ricerche Brave site-restricted sul target selezionato.</p>
              </div>
            ) : evidencePipeline.isPending ? (
              <div className={styles.skeletonWrap}>
                <Skeleton rows={6} />
              </div>
            ) : evidencePipeline.isError ? (
              <div className={`${styles.statePanel} ${styles.companyState}`} role="alert">
                <div className={styles.stateIcon}>
                  <Icon name="triangle-alert" size={22} />
                </div>
                <p className={styles.stateTitle}>Pipeline non disponibile</p>
                <p className={styles.stateText}>{errorLabel(evidencePipeline.error)}</p>
              </div>
            ) : (
              <>
                <div className={styles.responseBar}>
                  <span>
                    Web evidence score {evidencePipeline.data.summary.score} · {evidencePipeline.data.summary.confidence}
                  </span>
                  <span>
                    Final action {finalActionLabel(evidencePipeline.data.finalDecision.finalAction)}
                  </span>
                  <span>
                    {evidencePipeline.data.loadedFromCache
                      ? 'Validazione riusata'
                      : evidencePipeline.data.persistedWebValidation
                        ? 'Validazione salvata'
                        : evidencePipeline.data.persistError
                          ? 'Persistenza non riuscita'
                        : 'Persistenza N/D'}
                  </span>
                  <span className={styles.path}>
                    {evidencePipeline.data.selectedDomain?.domain ?? 'nessun dominio candidato'}
                  </span>
                </div>
                <div className={styles.dryRunGrid}>
                  <div className={styles.metricBox}>
                    <span>Core match</span>
                    <strong>
                      {evidencePipeline.data.summary.coreMatches}/{evidencePipeline.data.summary.searchedCoreTerms}
                    </strong>
                    <p>{evidencePipeline.data.summary.totalCoreTerms} termini totali</p>
                  </div>
                  <div className={styles.metricBox}>
                    <span>Adiacenti</span>
                    <strong>
                      {evidencePipeline.data.summary.adjacentMatches}/{evidencePipeline.data.summary.searchedAdjacentTerms}
                    </strong>
                    <p>{evidencePipeline.data.summary.totalAdjacentTerms} termini totali</p>
                  </div>
                  <div className={styles.metricBox}>
                    <span>Negativi</span>
                    <strong>
                      {evidencePipeline.data.summary.negativeMatches}/{evidencePipeline.data.summary.searchedNegativeTerms}
                    </strong>
                    <p>penalty -{evidencePipeline.data.summary.negativePenalty}</p>
                  </div>
                  <div className={styles.metricBox}>
                    <span>Settore</span>
                    <strong>{evidencePipeline.data.summary.sectorEvidenceScore}</strong>
                    <p>forza web evidence</p>
                  </div>
                  <div className={styles.metricBox}>
                    <span>Copertura</span>
                    <strong>{evidencePipeline.data.summary.coverageScore}</strong>
                    <p>match su keyword set</p>
                  </div>
                  <div className={styles.metricBox}>
                    <span>Dominio</span>
                    <strong>{evidencePipeline.data.summary.domainScore}</strong>
                    <p>{evidencePipeline.data.selectedDomain?.confidence ?? 'N/D'}</p>
                  </div>
                </div>

                <article className={`${styles.resultCard} ${styles.finalDecisionCard}`}>
                  <div className={styles.resultCardHead}>
                    <div>
                      <h3>Final reconciliation</h3>
                      <p>{evidencePipeline.data.finalDecision.reason}</p>
                    </div>
                    <div className={styles.cardActions}>
                      <span className={`${styles.scoreBadge} ${styles[`finalAction_${evidencePipeline.data.finalDecision.finalAction}`] ?? ''}`}>
                        {finalActionLabel(evidencePipeline.data.finalDecision.finalAction)}
                      </span>
                      <span className={`${styles.scoreBadge} ${styles[`confidence_${evidencePipeline.data.finalDecision.confidence}`] ?? ''}`}>
                        {webValidationStateLabel(evidencePipeline.data.finalDecision.webValidationState)}
                      </span>
                    </div>
                  </div>
                  <div className={styles.analysisGrid}>
                    <div>
                      <span>Score iniziale</span>
                      <p>
                        {evidencePipeline.data.finalDecision.deterministicScore} ·{' '}
                        {evidencePipeline.data.finalDecision.initialMatchState}
                      </p>
                    </div>
                    <div>
                      <span>Validazione web</span>
                      <p>
                        {evidencePipeline.data.finalDecision.webScore} ·{' '}
                        {webValidationStateLabel(evidencePipeline.data.finalDecision.webValidationState)}
                      </p>
                    </div>
                    <div>
                      <span>Analyst</span>
                      <p>
                        {evidencePipeline.data.finalDecision.analystVerdict
                          ? `${candidateVerdictLabel(evidencePipeline.data.finalDecision.analystVerdict)} · ${candidateActionLabel(
                              evidencePipeline.data.finalDecision.analystAction ?? '',
                            )}`
                          : 'N/D'}
                      </p>
                    </div>
                    <div>
                      <span>Dominio</span>
                      <p>
                        {evidencePipeline.data.selectedDomain
                          ? `${evidencePipeline.data.selectedDomain.domain} · ${evidencePipeline.data.selectedDomain.confidence}`
                          : 'N/D'}
                      </p>
                    </div>
                    <div>
                      <span>Persistenza</span>
                      <p>
                        {evidencePipeline.data.loadedFromCache
                          ? `riusata · ${new Date(evidencePipeline.data.persistedWebValidation?.updatedAt ?? '').toLocaleString('it-IT')}`
                          : evidencePipeline.data.persistedWebValidation
                            ? `salvata · ${new Date(evidencePipeline.data.persistedWebValidation.updatedAt).toLocaleString('it-IT')}`
                            : evidencePipeline.data.persistError ?? 'N/D'}
                      </p>
                    </div>
                  </div>
                  {evidencePipeline.data.finalDecision.reasons.length > 0 ? (
                    <div className={styles.finalReasons}>
                      {evidencePipeline.data.finalDecision.reasons.slice(0, 6).map((reason) => (
                        <span key={reason}>{reason}</span>
                      ))}
                    </div>
                  ) : null}
                </article>

                {evidencePipeline.data.candidateMatchAnalysis ? (
                  <article className={`${styles.resultCard} ${styles.analysisCard}`}>
                    <div className={styles.resultCardHead}>
                      <div>
                        <h3>Candidate match analyst</h3>
                        <p>{evidencePipeline.data.candidateMatchAnalysis.rationale}</p>
                      </div>
                      <div className={styles.cardActions}>
                        <span className={`${styles.scoreBadge} ${styles[`confidence_${evidencePipeline.data.candidateMatchAnalysis.confidence}`] ?? ''}`}>
                          {candidateVerdictLabel(evidencePipeline.data.candidateMatchAnalysis.verdict)}
                        </span>
                        <span className={styles.bucketBadge}>
                          {candidateActionLabel(evidencePipeline.data.candidateMatchAnalysis.recommendedAction)}
                        </span>
                      </div>
                    </div>
                    <div className={styles.analysisGrid}>
                      <div>
                        <span>Sector fit</span>
                        <p>{evidencePipeline.data.candidateMatchAnalysis.sectorFit || 'N/D'}</p>
                      </div>
                      <div>
                        <span>Business fit</span>
                        <p>{evidencePipeline.data.candidateMatchAnalysis.businessFit || 'N/D'}</p>
                      </div>
                    </div>
                    <div className={styles.analysisColumns}>
                      <div>
                        <span>Prove a favore</span>
                        {evidencePipeline.data.candidateMatchAnalysis.evidenceFor.length > 0 ? (
                          <ul>
                            {evidencePipeline.data.candidateMatchAnalysis.evidenceFor.map((item) => <li key={item}>{item}</li>)}
                          </ul>
                        ) : <p>N/D</p>}
                      </div>
                      <div>
                        <span>Contro / lacune</span>
                        {[...evidencePipeline.data.candidateMatchAnalysis.evidenceAgainst, ...evidencePipeline.data.candidateMatchAnalysis.missingEvidence].length > 0 ? (
                          <ul>
                            {[...evidencePipeline.data.candidateMatchAnalysis.evidenceAgainst, ...evidencePipeline.data.candidateMatchAnalysis.missingEvidence].map((item) => <li key={item}>{item}</li>)}
                          </ul>
                        ) : <p>N/D</p>}
                      </div>
                    </div>
                    {evidencePipeline.data.candidateMatchAnalysis.negativeSignals.length > 0 ? (
                      <div className={styles.analysisList}>
                        <span>Segnali negativi</span>
                        <p>{evidencePipeline.data.candidateMatchAnalysis.negativeSignals.join(' · ')}</p>
                      </div>
                    ) : null}
                    {evidencePipeline.data.candidateMatchAnalysis.conceptAliases.length > 0 ? (
                      <div className={styles.aliasList}>
                        <span>Alias concettuali</span>
                        {evidencePipeline.data.candidateMatchAnalysis.conceptAliases.map((alias) => (
                          <p key={`${alias.term}-${alias.matchedConcept}`}>
                            <strong>{alias.term}</strong> -&gt; {alias.matchedConcept}
                            {alias.evidence ? ` · ${alias.evidence}` : ''}
                          </p>
                        ))}
                      </div>
                    ) : null}
                  </article>
                ) : evidencePipeline.data.candidateMatchError ? (
                  <div className={styles.responseBar}>
                    <span>Candidate match analyst non disponibile: {evidencePipeline.data.candidateMatchError}</span>
                  </div>
                ) : null}

                {evidencePipeline.data.selectedDomain ? (
                  <div className={styles.cardList}>
                    <article className={styles.resultCard}>
                      <div className={styles.resultCardHead}>
                        <div>
                          <h3>{evidencePipeline.data.selectedDomain.domain}</h3>
                          <p>{evidencePipeline.data.selectedDomain.reasons.join(', ')}</p>
                        </div>
                        <span className={`${styles.scoreBadge} ${styles[`confidence_${evidencePipeline.data.selectedDomain.confidence}`] ?? ''}`}>
                          {evidencePipeline.data.selectedDomain.score} · {evidencePipeline.data.selectedDomain.confidence}
                        </span>
                      </div>
                    </article>
                    {evidencePipeline.data.evidenceRuns.map((run) => (
                      <article
                        key={`${run.bucket}-${run.term}`}
                        className={`${styles.resultCard} ${run.matched ? styles.pipelineRunMatched : ''}`}
                      >
                        <div className={styles.resultCardHead}>
                          <div>
                            <h3>{run.term}</h3>
                            <p>
                              {pipelineBucketLabel(run.bucket)} · {run.matched ? 'match' : 'rumore'} · {run.resultCount} risultati
                              {run.error ? ` · ${run.error}` : ''}
                            </p>
                          </div>
                          <span className={`${styles.bucketBadge} ${styles[`bucket_${run.bucket}`]}`}>
                            {run.bestScore ?? run.resultCount}
                          </span>
                        </div>
                        {run.response?.results.slice(0, 2).map((result) => (
                          <a key={result.url} href={result.url} target="_blank" rel="noreferrer" className={styles.evidenceItem}>
                            <span>{result.title || result.url}</span>
                            <small>{result.snippets[0] || result.hostname}</small>
                          </a>
                        ))}
                      </article>
                    ))}
                  </div>
                ) : (
                  <div className={`${styles.statePanel} ${styles.companyState}`}>
                    <div className={styles.stateIcon}>
                      <Icon name="network" size={22} />
                    </div>
                    <p className={styles.stateTitle}>Dominio non risolto</p>
                    <p className={styles.stateText}>La pipeline non esegue keyword evidence senza un candidato dominio.</p>
                  </div>
                )}

                <div className={`${styles.rawBlock} ${styles.rawBlockSeparated}`}>
                  <span>Risposta</span>
                  <pre>{rawPreview(evidencePipeline.data)}</pre>
                </div>
              </>
            )}
          </div>
        )}
      </section>
      ) : null}

      {activeTab === 'domain' ? (
      <section className={`${styles.panel} ${styles.companyPanel}`} aria-labelledby="domain-title">
        <div className={styles.panelHeader}>
          <div>
            <div className={styles.endpointLine}>
              <span className={styles.method}>POST</span>
              <span className={styles.path}>/binocolo/v1/test/domain-resolution</span>
            </div>
            <h2 id="domain-title" className={styles.sectionTitle}>Domain resolver</h2>
          </div>
          <form className={styles.companyForm} onSubmit={handleDomainSubmit}>
            <div className={styles.filterGrid}>
              <label className={`${styles.filterField} ${styles.fieldWide}`}>
                <span>Ragione sociale</span>
                <input
                  type="text"
                  value={domainForm.companyName}
                  onChange={handleDomainFieldChange('companyName')}
                  autoComplete="off"
                  spellCheck={false}
                  required
                />
              </label>
              <label className={styles.filterField}>
                <span>Partita IVA</span>
                <input
                  type="text"
                  value={domainForm.vatCode}
                  onChange={handleDomainFieldChange('vatCode')}
                  autoComplete="off"
                  spellCheck={false}
                  className={styles.codeInput}
                />
              </label>
              <label className={styles.filterField}>
                <span>Codice fiscale</span>
                <input
                  type="text"
                  value={domainForm.taxCode}
                  onChange={handleDomainFieldChange('taxCode')}
                  autoComplete="off"
                  spellCheck={false}
                  className={styles.codeInput}
                />
              </label>
              <label className={styles.filterField}>
                <span>Comune</span>
                <input type="text" value={domainForm.town} onChange={handleDomainFieldChange('town')} autoComplete="off" />
              </label>
              <label className={styles.filterField}>
                <span>Provincia</span>
                <input
                  type="text"
                  value={domainForm.province}
                  onChange={handleDomainFieldChange('province')}
                  maxLength={2}
                  autoCapitalize="characters"
                  autoComplete="off"
                  className={styles.codeInput}
                />
              </label>
              <label className={`${styles.filterField} ${styles.fieldWide}`}>
                <span>Keyword contesto</span>
                <input
                  type="text"
                  value={domainForm.keywords}
                  onChange={handleDomainFieldChange('keywords')}
                  placeholder="cloud, hosting, sistemistica"
                  autoComplete="off"
                  spellCheck={false}
                />
              </label>
              <label className={styles.filterField}>
                <span>Risultati Brave</span>
                <input
                  type="number"
                  value={domainForm.count}
                  onChange={handleDomainFieldChange('count')}
                  min={1}
                  max={20}
                  step={1}
                />
              </label>
            </div>
            <div className={styles.formActions}>
              <Button type="submit" loading={domainResolution.isPending} leftIcon={<Icon name="network" />}>
                Risolvi dominio
              </Button>
            </div>
          </form>
        </div>

        {domainResolution.isIdle ? (
          <div className={`${styles.statePanel} ${styles.companyState}`}>
            <div className={styles.stateIcon}>
              <Icon name="network" size={22} />
            </div>
            <p className={styles.stateTitle}>Resolver pronto</p>
            <p className={styles.stateText}>Inserisci ragione sociale e identificativi per cercare domini candidati con Brave.</p>
          </div>
        ) : domainResolution.isPending ? (
          <div className={styles.skeletonWrap}>
            <Skeleton rows={5} />
          </div>
        ) : domainResolution.isError ? (
          <div className={`${styles.statePanel} ${styles.companyState}`} role="alert">
            <div className={styles.stateIcon}>
              <Icon name="triangle-alert" size={22} />
            </div>
            <p className={styles.stateTitle}>Resolver non disponibile</p>
            <p className={styles.stateText}>{errorLabel(domainResolution.error)}</p>
          </div>
        ) : (
          <div className={styles.companyResult}>
            <div className={styles.responseBar}>
              <span>{domainResolution.data.candidates.length} domini candidati</span>
              <span className={styles.path}>{domainResolution.data.query}</span>
            </div>
            {domainResolution.data.candidates.length > 0 ? (
              <div className={styles.cardList}>
                {domainResolution.data.candidates.map((candidate) => (
                  <article key={candidate.domain} className={styles.resultCard}>
                    <div className={styles.resultCardHead}>
                      <div>
                        <h3>{candidate.domain}</h3>
                        <p>{candidate.reasons.join(', ') || 'Nessuna ragione forte'}</p>
                      </div>
                      <div className={styles.cardActions}>
                        <span className={`${styles.scoreBadge} ${styles[`confidence_${candidate.confidence}`] ?? ''}`}>
                          {candidate.score} · {candidate.confidence}
                        </span>
                        <Button variant="secondary" size="sm" onClick={() => useCandidateDomain(candidate.domain)}>
                          Usa
                        </Button>
                      </div>
                    </div>
                    <div className={styles.evidenceList}>
                      {candidate.results.slice(0, 3).map((result) => (
                        <a key={result.url} href={result.url} target="_blank" rel="noreferrer" className={styles.evidenceItem}>
                          <span>{result.title || result.url}</span>
                          <small>{result.hostname}</small>
                        </a>
                      ))}
                    </div>
                  </article>
                ))}
              </div>
            ) : (
              <div className={`${styles.statePanel} ${styles.companyState}`}>
                <div className={styles.stateIcon}>
                  <Icon name="search" size={22} />
                </div>
                <p className={styles.stateTitle}>Nessun dominio candidato</p>
                <p className={styles.stateText}>Prova con partita IVA, comune o keyword piu specifiche.</p>
              </div>
            )}
            <div className={`${styles.rawBlock} ${styles.rawBlockSeparated}`}>
              <span>Risposta</span>
              <pre>{rawPreview(domainResolution.data)}</pre>
            </div>
          </div>
        )}
      </section>
      ) : null}

      {activeTab === 'keyword' ? (
      <section className={`${styles.panel} ${styles.companyPanel}`} aria-labelledby="keyword-title">
        <div className={styles.panelHeader}>
          <div>
            <div className={styles.endpointLine}>
              <span className={styles.method}>POST</span>
              <span className={styles.path}>/binocolo/v1/web-search</span>
            </div>
            <h2 id="keyword-title" className={styles.sectionTitle}>Keyword evidence</h2>
          </div>
          <form className={styles.companyForm} onSubmit={handleKeywordSubmit}>
            <div className={styles.filterGrid}>
              <label className={styles.filterField}>
                <span>Dominio</span>
                <input
                  type="text"
                  value={keywordForm.domain}
                  onChange={handleKeywordFieldChange('domain')}
                  placeholder="azienda.it"
                  autoComplete="off"
                  spellCheck={false}
                  required
                />
              </label>
              <label className={`${styles.filterField} ${styles.fieldWide}`}>
                <span>Keyword</span>
                <input
                  type="text"
                  value={keywordForm.keywords}
                  onChange={handleKeywordFieldChange('keywords')}
                  placeholder="managed services, cloud, cybersecurity"
                  autoComplete="off"
                  spellCheck={false}
                  required
                />
              </label>
              <label className={styles.filterField}>
                <span>Risultati</span>
                <input
                  type="number"
                  value={keywordForm.count}
                  onChange={handleKeywordFieldChange('count')}
                  min={1}
                  max={50}
                  step={1}
                />
              </label>
            </div>
            <div className={styles.formActions}>
              <label className={styles.dryRunField}>
                <span>Rank AI</span>
                <ToggleSwitch
                  id="binocolo-keyword-rank"
                  checked={keywordForm.rank}
                  onChange={handleKeywordRankChange}
                />
              </label>
              <Button type="submit" loading={keywordEvidence.isPending} leftIcon={<Icon name="search" />}>
                Cerca evidenze
              </Button>
            </div>
          </form>
        </div>

        {keywordEvidence.isIdle ? (
          <div className={`${styles.statePanel} ${styles.companyState}`}>
            <div className={styles.stateIcon}>
              <Icon name="search" size={22} />
            </div>
            <p className={styles.stateTitle}>Keyword evidence pronta</p>
            <p className={styles.stateText}>Usa un dominio candidato e keyword settoriali per verificare snippet citabili.</p>
          </div>
        ) : keywordEvidence.isPending ? (
          <div className={styles.skeletonWrap}>
            <Skeleton rows={5} />
          </div>
        ) : keywordEvidence.isError ? (
          <div className={`${styles.statePanel} ${styles.companyState}`} role="alert">
            <div className={styles.stateIcon}>
              <Icon name="triangle-alert" size={22} />
            </div>
            <p className={styles.stateTitle}>Ricerca non disponibile</p>
            <p className={styles.stateText}>{errorLabel(keywordEvidence.error)}</p>
          </div>
        ) : (
          <div className={styles.companyResult}>
            <div className={styles.responseBar}>
              <span>{keywordEvidence.data.results.length} risultati{keywordEvidence.data.ranked ? ' · ordinati per rilevanza' : ''}</span>
              <span className={styles.path}>{keywordEvidence.data.query}</span>
            </div>
            {keywordEvidence.data.rankError ? (
              <div className={styles.responseBar}>
                <span>{keywordEvidence.data.rankError}</span>
              </div>
            ) : null}
            {keywordEvidence.data.results.length > 0 ? (
              <div className={styles.cardList}>
                {keywordEvidence.data.results.map((result) => (
                  <article key={result.url} className={styles.resultCard}>
                    <div className={styles.resultCardHead}>
                      <div>
                        <h3>
                          <a href={result.url} target="_blank" rel="noreferrer">{result.title || result.url}</a>
                        </h3>
                        <p>{result.hostname}{result.age ? ` · ${result.age}` : ''}</p>
                      </div>
                      {typeof result.score === 'number' ? <span className={styles.scoreBadge}>{result.score}</span> : null}
                    </div>
                    {result.snippets.length > 0 ? <p className={styles.snippetText}>{result.snippets.join(' ... ')}</p> : null}
                  </article>
                ))}
              </div>
            ) : (
              <div className={`${styles.statePanel} ${styles.companyState}`}>
                <div className={styles.stateIcon}>
                  <Icon name="search" size={22} />
                </div>
                <p className={styles.stateTitle}>Nessuna evidenza</p>
                <p className={styles.stateText}>Prova keyword diverse o un dominio candidato alternativo.</p>
              </div>
            )}
            <div className={`${styles.rawBlock} ${styles.rawBlockSeparated}`}>
              <span>Risposta</span>
              <pre>{rawPreview(keywordEvidence.data)}</pre>
            </div>
          </div>
        )}
      </section>
      ) : null}

      {activeTab === 'maintenance' ? (
      <section className={styles.panel} aria-labelledby="maint-title">
        <div className={styles.panelHeader}>
          <div>
            <div className={styles.endpointLine}>
              <span className={styles.method}>POST</span>
              <span className={styles.path}>/binocolo/v1/ma/deep/recompute · /regenerate-briefs</span>
            </div>
            <h2 id="maint-title" className={styles.sectionTitle}>Manutenzione deep analysis</h2>
          </div>
          <div className={styles.formActions}>
            <Button
              variant="secondary"
              loading={recompute.isPending}
              onClick={() => recompute.mutate()}
              leftIcon={<Icon name="refresh-cw" />}
            >
              Ricalcola scorecard
            </Button>
            <Button
              variant="secondary"
              loading={regenerateBriefs.isPending}
              onClick={() => regenerateBriefs.mutate()}
              leftIcon={<Icon name="file-text" />}
            >
              Rigenera brief (LLM)
            </Button>
          </div>
        </div>
        {recompute.isSuccess || recompute.isError ? (
          <div className={styles.responseBar}>
            <span>
              {recompute.isError
                ? errorLabel(recompute.error)
                : `Ricalcolate ${recompute.data?.recomputed ?? 0} scorecard dalla cache (nessuna chiamata IT-full, €0).`}
            </span>
          </div>
        ) : null}
        {regenerateBriefs.isSuccess || regenerateBriefs.isError ? (
          <div className={styles.responseBar}>
            <span>
              {regenerateBriefs.isError
                ? errorLabel(regenerateBriefs.error)
                : `Rigenerati ${regenerateBriefs.data?.regenerated ?? 0} brief via LLM (nessuna chiamata IT-full).`}
            </span>
          </div>
        ) : null}
      </section>
      ) : null}
    </div>
  );
}
