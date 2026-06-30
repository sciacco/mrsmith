import { useEffect, useMemo, useState, type ChangeEvent, type FormEvent } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Button, Icon, Skeleton, ToggleSwitch } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import type {
  CompanySearchRow,
  DomainResolutionResponse,
  MASessionDetail,
  MASessionListResponse,
  MASessionVisibility,
  MATarget,
  MAWebValidationEnrichRequest,
  OpenAPIITEnvelope,
  PipelineFinalAction,
  PipelineWebValidationState,
  SectorClassificationTestResponse,
  SectorEvalItem,
  SectorEvalLabel,
  SectorEvalReport,
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
type TestTab = 'company' | 'pipeline' | 'classification' | 'sector-eval' | 'domain' | 'keyword' | 'maintenance';

const testTabs = [
  { id: 'company', label: 'Company search', icon: 'database' },
  { id: 'pipeline', label: 'Evidence pipeline', icon: 'route' },
  { id: 'classification', label: 'Classificazione settore', icon: 'sparkles' },
  { id: 'sector-eval', label: 'Sector eval', icon: 'clipboard-check' },
  { id: 'domain', label: 'Domain resolver', icon: 'network' },
  { id: 'keyword', label: 'Keyword evidence', icon: 'search' },
  { id: 'maintenance', label: 'Manutenzione', icon: 'settings' },
] as const;

const sectorEvalBucketLabels: Record<SectorEvalLabel, string> = {
  keep: 'Keep',
  forse: 'Forse',
  scarta: 'Scarta',
};

// Colour a prediction relative to the human label: green when it matches, amber when it
// disagrees, neutral when there is no label yet or the predictor produced no call.
const evalMarkClass = (
  bucket: SectorEvalLabel | undefined,
  label: SectorEvalLabel | undefined,
): string => {
  if (!label || !bucket) return styles.muted ?? '';
  return (bucket === label ? styles.ok : styles.warn) ?? '';
};

const renderEvalPrediction = (
  bucket: SectorEvalLabel | undefined,
  sub: string | undefined,
  label: SectorEvalLabel | undefined,
) =>
  bucket ? (
    <>
      <span className={evalMarkClass(bucket, label)}>{sectorEvalBucketLabels[bucket]}</span>
      {sub ? <div className={styles.muted}>{sub}</div> : null}
    </>
  ) : (
    <span className={styles.muted}>—</span>
  );

// Domain-resolution outcome cell: resolved (green) vs the two failure modes (amber).
// acceptance_fail also shows the best candidate that the gate rejected, so the operator
// sees how close it was.
const renderDomainOutcome = (item: SectorEvalItem) => {
  if (!item.validated) return <span className={styles.muted}>—</span>;
  if (item.domainOutcome === 'resolved') {
    return (
      <>
        <span className={styles.ok}>risolto</span>
        <div className={styles.muted}>
          {item.domainConfidence}
          {item.domainScore ? ` · ${item.domainScore}` : ''}
        </div>
      </>
    );
  }
  if (item.domainOutcome === 'retrieval_fail') {
    return <span className={styles.warn}>retrieval · 0 candidati</span>;
  }
  return (
    <>
      <span className={styles.warn}>acceptance · scartato</span>
      {item.bestCandidateDomain ? (
        <div className={styles.muted}>
          {item.bestCandidateDomain} ({item.bestCandidateScore}/{item.bestCandidateConfidence})
        </div>
      ) : null}
    </>
  );
};

const sessionVisibilityOptions: Array<{ value: MASessionVisibility; label: string }> = [
  { value: 'active', label: 'Attive' },
  { value: 'archived', label: 'Archiviate' },
  { value: 'deleted', label: 'Cestino' },
];

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
  const [pipelineAnalyzeWithLLM, setPipelineAnalyzeWithLLM] = useState(true);
  const [evalSessionId, setEvalSessionId] = useState('');
  const [sectorForm, setSectorForm] = useState({
    sectorDescription: '',
    companyDescription: '',
    domain: '',
    snippets: '',
    analyze: true,
  });

  const pipelineSessions = useQuery({
    queryKey: ['binocolo-test-ma-sessions', pipelineVisibility],
    queryFn: () =>
      api.get<MASessionListResponse>(`/binocolo/v1/ma/sessions?visibility=${pipelineVisibility}`),
    enabled: activeTab === 'pipeline' || activeTab === 'sector-eval',
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
  const sectorClassification = useMutation({
    mutationFn: () =>
      api.post<SectorClassificationTestResponse>('/binocolo/v1/test/sector-classification', {
        sectorDescription: sectorForm.sectorDescription.trim(),
        companyDescription: sectorForm.companyDescription.trim() || undefined,
        domain: sectorForm.domain.trim() || undefined,
        snippets: sectorForm.snippets.trim()
          ? sectorForm.snippets
              .split('\n')
              .map((line) => line.trim())
              .filter(Boolean)
          : undefined,
        analyze: sectorForm.analyze,
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
    mutationFn: () =>
      api.post<SectorClassificationTestResponse>('/binocolo/v1/test/sector-classification', {
        sessionId: pipelineSessionId,
        targetId: pipelineTargetId,
        analyze: pipelineAnalyzeWithLLM,
      }),
  });

  const sectorEval = useQuery({
    queryKey: ['binocolo-sector-eval', evalSessionId],
    queryFn: () => api.get<SectorEvalReport>(`/binocolo/v1/ma/sessions/${evalSessionId}/sector-eval`),
    enabled: activeTab === 'sector-eval' && Boolean(evalSessionId),
  });
  const setEvalLabel = useMutation({
    mutationFn: (body: { companyKey: string; label: SectorEvalLabel | '' }) =>
      api.put<void>(`/binocolo/v1/ma/sessions/${evalSessionId}/sector-eval/label`, body),
    onSuccess: () => {
      void sectorEval.refetch();
    },
  });
  const runEvalValidation = useMutation({
    mutationFn: () =>
      api.post<unknown>(`/binocolo/v1/ma/sessions/${evalSessionId}/web-validation/enrich`, {
        force: true,
        analyzeWithLLM: true,
        llmOnAll: true,
        inline: true,
      } satisfies MAWebValidationEnrichRequest),
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
            </div>
            <div className={styles.formActions}>
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
            </div>

            {evidencePipeline.isIdle ? (
              <div className={`${styles.statePanel} ${styles.companyState}`}>
                <div className={styles.stateIcon}>
                  <Icon name="route" size={22} />
                </div>
                <p className={styles.stateTitle}>Pipeline pronta</p>
                <p className={styles.stateText}>
                  Esegue la pipeline UC2 di classificazione settore sul target selezionato: risoluzione dominio,
                  evidenza neutra, embedding + reranker e tie-breaker LLM opzionale.
                </p>
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
              <div className={styles.companyResult}>
                <div className={styles.responseBar}>
                  <span>
                    Verdetto: <strong>{evidencePipeline.data.classification.verdict}</strong> · top{' '}
                    {evidencePipeline.data.classification.topProb.toFixed(2)} ·{' '}
                    {evidencePipeline.data.classification.rerankApplied ? 'rerank attivo' : 'solo embedding'}
                  </span>
                  <span className={styles.path}>{evidencePipeline.data.classification.confidence}</span>
                </div>
                <div className={styles.responseBar}>
                  <span>{evidencePipeline.data.companyName || selectedPipelineTarget.companyName}</span>
                  <span className={styles.path}>
                    {evidencePipeline.data.selectedDomain || 'nessun dominio risolto'}
                  </span>
                </div>

                {evidencePipeline.data.finalDecision ? (
                  <article className={`${styles.resultCard} ${styles.finalDecisionCard}`}>
                    <div className={styles.resultCardHead}>
                      <div>
                        <h3>Decisione finale</h3>
                        <p>{evidencePipeline.data.finalDecision.reason}</p>
                      </div>
                      <div className={styles.cardActions}>
                        <span className={`${styles.scoreBadge} ${styles[`finalAction_${evidencePipeline.data.finalDecision.finalAction}`] ?? ''}`}>
                          {finalActionLabel(evidencePipeline.data.finalDecision.finalAction)}
                        </span>
                        <span className={`${styles.scoreBadge} ${styles[`finalAction_${evidencePipeline.data.finalDecision.finalAction}`] ?? ''}`}>
                          {webValidationStateLabel(evidencePipeline.data.finalDecision.webValidationState)}
                        </span>
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
                ) : null}

                <div className={`${styles.rawBlock} ${styles.rawBlockSeparated}`}>
                  <span>Descrizione azienda usata</span>
                  <pre>{evidencePipeline.data.classification.companyDescription || '(nessuna)'}</pre>
                </div>
                {evidencePipeline.data.classification.reason ? (
                  <div className={styles.responseBar}>
                    <span>{evidencePipeline.data.classification.reason}</span>
                  </div>
                ) : null}
                <div className={styles.cardList}>
                  {evidencePipeline.data.classification.concepts.map((concept) => (
                    <article key={concept.conceptId} className={styles.resultCard}>
                      <div className={styles.resultCardHead}>
                        <div>
                          <h3>
                            {concept.name}
                            {concept.inStrategy ? ' · in perimetro' : ''}
                          </h3>
                          <p>
                            {concept.conceptId} · coseno {concept.cosine.toFixed(3)}
                          </p>
                        </div>
                        <div className={styles.cardActions}>
                          <span className={styles.bucketBadge}>{concept.kind}</span>
                          <span className={styles.scoreBadge}>rerank {concept.rerankProb.toFixed(2)}</span>
                        </div>
                      </div>
                    </article>
                  ))}
                </div>
                {evidencePipeline.data.classification.strategyConcepts.length > 0 ? (
                  <div className={styles.responseBar}>
                    <span>Perimetro strategia: {evidencePipeline.data.classification.strategyConcepts.join(', ')}</span>
                  </div>
                ) : null}
                {evidencePipeline.data.analysis ? (
                  <article className={`${styles.resultCard} ${styles.analysisCard}`}>
                    <div className={styles.resultCardHead}>
                      <div>
                        <h3>Analyst LLM (tie-breaker)</h3>
                        <p>{evidencePipeline.data.analysis.rationale}</p>
                      </div>
                      <div className={styles.cardActions}>
                        <span className={styles.scoreBadge}>{evidencePipeline.data.analysis.verdict}</span>
                        <span className={styles.bucketBadge}>{evidencePipeline.data.analysis.recommendedAction}</span>
                      </div>
                    </div>
                  </article>
                ) : null}
                {evidencePipeline.data.evidence.length > 0 ? (
                  <div className={`${styles.rawBlock} ${styles.rawBlockSeparated}`}>
                    <span>Evidenza neutra raccolta</span>
                    <pre>{evidencePipeline.data.evidence.join('\n')}</pre>
                  </div>
                ) : null}
              </div>
            )}
          </div>
        )}
      </section>
      ) : null}

      {activeTab === 'classification' ? (
      <section className={`${styles.panel} ${styles.companyPanel}`} aria-labelledby="classification-title">
        <div className={styles.panelHeader}>
          <div>
            <div className={styles.endpointLine}>
              <span className={styles.method}>POST</span>
              <span className={styles.path}>/binocolo/v1/test/sector-classification</span>
            </div>
            <h2 id="classification-title" className={styles.sectionTitle}>Classificazione settore (UC2)</h2>
          </div>
          <form
            className={styles.companyForm}
            onSubmit={(event) => {
              event.preventDefault();
              sectorClassification.mutate();
            }}
          >
            <div className={styles.filterGrid}>
              <label className={`${styles.filterField} ${styles.fieldWide}`}>
                <span>Settore strategia (intento)</span>
                <input
                  type="text"
                  value={sectorForm.sectorDescription}
                  onChange={(event) => setSectorForm((prev) => ({ ...prev, sectorDescription: event.target.value }))}
                  placeholder="servizi IT gestiti; sicurezza informatica"
                  autoComplete="off"
                  required
                />
              </label>
              <label className={`${styles.filterField} ${styles.fieldWide}`}>
                <span>Autodescrizione azienda (incolla qui per il test più rapido)</span>
                <textarea
                  value={sectorForm.companyDescription}
                  onChange={(event) => setSectorForm((prev) => ({ ...prev, companyDescription: event.target.value }))}
                  placeholder="Es: Azienda che offre servizi di gestione infrastrutture IT, cloud e assistenza sistemistica..."
                  rows={3}
                  autoComplete="off"
                />
              </label>
              <label className={styles.filterField}>
                <span>Oppure dominio (via Brave)</span>
                <input
                  type="text"
                  value={sectorForm.domain}
                  onChange={(event) => setSectorForm((prev) => ({ ...prev, domain: event.target.value }))}
                  placeholder="azienda.it"
                  autoComplete="off"
                  spellCheck={false}
                />
              </label>
              <label className={`${styles.filterField} ${styles.fieldWide}`}>
                <span>Oppure snippet (uno per riga)</span>
                <textarea
                  value={sectorForm.snippets}
                  onChange={(event) => setSectorForm((prev) => ({ ...prev, snippets: event.target.value }))}
                  placeholder={'Chi siamo - gestione IT\nServizi cloud e backup'}
                  rows={3}
                  autoComplete="off"
                />
              </label>
            </div>
            <div className={styles.formActions}>
              <ToggleSwitch
                id="sector-analyze-toggle"
                checked={sectorForm.analyze}
                onChange={(checked) => setSectorForm((prev) => ({ ...prev, analyze: checked }))}
                label="Escalation LLM se ambiguo"
              />
              <Button type="submit" loading={sectorClassification.isPending} leftIcon={<Icon name="sparkles" />}>
                Classifica
              </Button>
            </div>
          </form>
        </div>

        {sectorClassification.isIdle ? (
          <div className={`${styles.statePanel} ${styles.companyState}`}>
            <div className={styles.stateIcon}>
              <Icon name="sparkles" size={22} />
            </div>
            <p className={styles.stateTitle}>Probe pronto</p>
            <p className={styles.stateText}>
              Indica il settore della strategia e l&apos;azienda (descrizione, dominio o snippet). Embedding + reranker
              sui concetti KB, senza toccare il funnel.
            </p>
          </div>
        ) : sectorClassification.isPending ? (
          <div className={styles.skeletonWrap}>
            <Skeleton rows={5} />
          </div>
        ) : sectorClassification.isError ? (
          <div className={`${styles.statePanel} ${styles.companyState}`} role="alert">
            <div className={styles.stateIcon}>
              <Icon name="triangle-alert" size={22} />
            </div>
            <p className={styles.stateTitle}>Classificazione non disponibile</p>
            <p className={styles.stateText}>{errorLabel(sectorClassification.error)}</p>
          </div>
        ) : (
          <div className={styles.companyResult}>
            <div className={styles.responseBar}>
              <span>
                Verdetto: <strong>{sectorClassification.data.classification.verdict}</strong> · top{' '}
                {sectorClassification.data.classification.topProb.toFixed(2)} ·{' '}
                {sectorClassification.data.classification.rerankApplied ? 'rerank attivo' : 'solo embedding'}
              </span>
              <span className={styles.path}>{sectorClassification.data.classification.confidence}</span>
            </div>
            <div className={`${styles.rawBlock} ${styles.rawBlockSeparated}`}>
              <span>Descrizione azienda usata</span>
              <pre>{sectorClassification.data.classification.companyDescription || '(nessuna)'}</pre>
            </div>
            {sectorClassification.data.classification.reason ? (
              <div className={styles.responseBar}>
                <span>{sectorClassification.data.classification.reason}</span>
              </div>
            ) : null}
            <div className={styles.cardList}>
              {sectorClassification.data.classification.concepts.map((concept) => (
                <article key={concept.conceptId} className={styles.resultCard}>
                  <div className={styles.resultCardHead}>
                    <div>
                      <h3>
                        {concept.name}
                        {concept.inStrategy ? ' · in perimetro' : ''}
                      </h3>
                      <p>
                        {concept.conceptId} · coseno {concept.cosine.toFixed(3)}
                      </p>
                    </div>
                    <div className={styles.cardActions}>
                      <span className={styles.bucketBadge}>{concept.kind}</span>
                      <span className={styles.scoreBadge}>rerank {concept.rerankProb.toFixed(2)}</span>
                    </div>
                  </div>
                </article>
              ))}
            </div>
            {sectorClassification.data.classification.strategyConcepts.length > 0 ? (
              <div className={styles.responseBar}>
                <span>Perimetro strategia: {sectorClassification.data.classification.strategyConcepts.join(', ')}</span>
              </div>
            ) : null}
            {sectorClassification.data.analysis ? (
              <article className={`${styles.resultCard} ${styles.analysisCard}`}>
                <div className={styles.resultCardHead}>
                  <div>
                    <h3>Analyst LLM (tie-breaker)</h3>
                    <p>{sectorClassification.data.analysis.rationale}</p>
                  </div>
                  <div className={styles.cardActions}>
                    <span className={styles.scoreBadge}>{sectorClassification.data.analysis.verdict}</span>
                    <span className={styles.bucketBadge}>{sectorClassification.data.analysis.recommendedAction}</span>
                  </div>
                </div>
              </article>
            ) : null}
            {sectorClassification.data.evidence.length > 0 ? (
              <div className={`${styles.rawBlock} ${styles.rawBlockSeparated}`}>
                <span>Evidenza neutra raccolta</span>
                <pre>{sectorClassification.data.evidence.join('\n')}</pre>
              </div>
            ) : null}
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

      {activeTab === 'sector-eval' ? (
      <section className={`${styles.panel} ${styles.companyPanel}`} aria-labelledby="sector-eval-title">
        <div className={styles.panelHeader}>
          <div>
            <div className={styles.endpointLine}>
              <span className={styles.method}>EVAL</span>
              <span className={styles.path}>/binocolo/v1/ma/sessions/&#123;id&#125;/sector-eval</span>
            </div>
            <h2 id="sector-eval-title" className={styles.sectionTitle}>Sector eval</h2>
            <p className={styles.sectionHint}>
              Esegui la validazione (l&apos;LLM gira su <em>ogni</em> azienda, non solo sugli ambigui:
              +1 chiamata LLM per azienda), poi conferma o riclassifica ogni riga. La label è
              ground-truth (persistita, separata dalla predizione). Confronto su tre predittori: A
              embed+rerank senza LLM, B solo LLM, C ibrido in produzione. Verde = concorda con la
              label, ambra = diverge.
            </p>
          </div>
          <div className={styles.companyForm}>
            <div className={styles.filterGrid}>
              <label className={`${styles.filterField} ${styles.fieldWide}`}>
                <span>Sessione</span>
                <select
                  value={evalSessionId}
                  onChange={(event) => setEvalSessionId(event.target.value)}
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
            </div>
            <div className={styles.formActions}>
              <Button
                variant="secondary"
                loading={runEvalValidation.isPending}
                disabled={!evalSessionId}
                onClick={() => runEvalValidation.mutate()}
                leftIcon={<Icon name="route" />}
              >
                Esegui validazione sessione
              </Button>
              <Button
                variant="secondary"
                loading={sectorEval.isFetching}
                disabled={!evalSessionId}
                onClick={() => void sectorEval.refetch()}
                leftIcon={<Icon name="refresh-cw" />}
              >
                Carica report
              </Button>
              {sectorEval.data ? (
                <Button
                  variant="ghost"
                  onClick={() => void navigator.clipboard?.writeText(JSON.stringify(sectorEval.data, null, 2))}
                >
                  Copia JSON
                </Button>
              ) : null}
            </div>
          </div>
        </div>

        {runEvalValidation.isSuccess ? (
          <div className={styles.responseBar}>
            <span>Validazione avviata (async). Attendi il completamento, poi premi “Carica report”.</span>
          </div>
        ) : null}
        {runEvalValidation.isError ? (
          <div className={styles.responseBar}><span>{errorLabel(runEvalValidation.error)}</span></div>
        ) : null}
        {sectorEval.isError ? (
          <div className={styles.responseBar}><span>{errorLabel(sectorEval.error)}</span></div>
        ) : null}

        {sectorEval.data ? (
          <>
            <div className={styles.responseBar}>
              {sectorEval.data.perimeter.sectorDescription || sectorEval.data.perimeter.title ? (
                <span><strong>Settore:</strong> {sectorEval.data.perimeter.sectorDescription || sectorEval.data.perimeter.title}</span>
              ) : null}
              {sectorEval.data.perimeter.territoryLabel || (sectorEval.data.perimeter.provinces?.length ?? 0) > 0 ? (
                <span><strong>Territorio:</strong> {sectorEval.data.perimeter.territoryLabel || sectorEval.data.perimeter.provinces?.join(', ')}</span>
              ) : null}
              {(sectorEval.data.perimeter.atecoCandidates?.length ?? 0) > 0 ? (
                <span><strong>ATECO:</strong> {sectorEval.data.perimeter.atecoCandidates?.map((a) => a.code).join(', ')}</span>
              ) : null}
              {(sectorEval.data.perimeter.legalForms?.length ?? 0) > 0 ? (
                <span><strong>Forme:</strong> {sectorEval.data.perimeter.legalForms?.join(', ')}</span>
              ) : null}
              {sectorEval.data.perimeter.thesis ? (
                <span><strong>Tesi:</strong> {sectorEval.data.perimeter.thesis}</span>
              ) : null}
            </div>
            <div className={styles.responseBar}>
              <span>
                <strong>A · embed+rerank</strong> {(sectorEval.data.metrics.deterministic.accuracy * 100).toFixed(0)}% (
                {sectorEval.data.metrics.deterministic.correct}/{sectorEval.data.metrics.deterministic.evaluable})
              </span>
              <span>
                <strong>B · LLM sempre</strong> {(sectorEval.data.metrics.llm.accuracy * 100).toFixed(0)}% (
                {sectorEval.data.metrics.llm.correct}/{sectorEval.data.metrics.llm.evaluable})
              </span>
              <span>
                <strong>C · ibrido (prod)</strong> {(sectorEval.data.metrics.final.accuracy * 100).toFixed(0)}% (
                {sectorEval.data.metrics.final.correct}/{sectorEval.data.metrics.final.evaluable})
              </span>
            </div>
            <div className={styles.responseBar}>
              <span>
                Escalation LLM {(sectorEval.data.metrics.escalationRate * 100).toFixed(0)}% (
                {sectorEval.data.metrics.escalations}/{sectorEval.data.metrics.validated})
              </span>
              <span>
                LLM vs det — entrambi OK {sectorEval.data.metrics.llmVsDeterministic.bothCorrect} · solo det{' '}
                {sectorEval.data.metrics.llmVsDeterministic.detOnly} (LLM peggiora) · solo LLM{' '}
                {sectorEval.data.metrics.llmVsDeterministic.llmOnly} (LLM salva) · entrambi KO{' '}
                {sectorEval.data.metrics.llmVsDeterministic.bothWrong}
              </span>
              <span>Distrattore &gt; target: {sectorEval.data.metrics.distractorBeatsTarget}</span>
              <span>
                Etichettate {sectorEval.data.metrics.labeled}/{sectorEval.data.metrics.targets} · validate{' '}
                {sectorEval.data.metrics.validated}
              </span>
            </div>
            <div className={styles.responseBar}>
              <span>
                <strong>Dominio risolto</strong> {(sectorEval.data.metrics.domain.resolutionRate * 100).toFixed(0)}% (
                {sectorEval.data.metrics.domain.resolved}/{sectorEval.data.metrics.validated})
              </span>
              <span>retrieval-fail (0 candidati): {sectorEval.data.metrics.domain.retrievalFail}</span>
              <span>acceptance-fail (scartati dal cancello): {sectorEval.data.metrics.domain.acceptanceFail}</span>
            </div>
            <div className={styles.tableScroll}>
              <table className={styles.resultTable}>
                <thead>
                  <tr>
                    <th>Azienda</th>
                    <th>Dominio</th>
                    <th>Self-description</th>
                    <th>A · embed+rerank</th>
                    <th>B · LLM sempre</th>
                    <th>C · ibrido (prod)</th>
                    <th>Top concetti</th>
                    <th>Label (tu)</th>
                  </tr>
                </thead>
                <tbody>
                  {sectorEval.data.items.map((item: SectorEvalItem) => {
                    const finalMismatch = Boolean(item.label && item.finalBucket && item.label !== item.finalBucket);
                    return (
                    <tr key={item.companyKey} className={finalMismatch ? styles.rowMismatch : ''}>
                      <td>
                        <strong>{item.companyName}</strong>
                        {item.domain ? <div className={styles.path}>{item.domain}</div> : null}
                        {item.atecoDescription ? <div className={styles.muted}>{item.atecoDescription}</div> : null}
                      </td>
                      <td>{renderDomainOutcome(item)}</td>
                      <td className={styles.muted}>
                        {item.selfDescription || (item.validated ? '—' : 'non validata')}
                      </td>
                      <td>
                        {item.validated ? (
                          <>
                            {renderEvalPrediction(item.deterministicBucket, item.deterministicVerdict, item.label)}
                            {item.escalated ? <div className={styles.muted}>→ escala LLM</div> : null}
                            {item.distractorBeatsTarget ? <div className={styles.warn}>⚠ distrattore &gt; target</div> : null}
                          </>
                        ) : (
                          <span className={styles.muted}>—</span>
                        )}
                      </td>
                      <td>
                        {renderEvalPrediction(
                          item.llmBucket,
                          [item.llmVerdict, item.llmAction].filter(Boolean).join(' / ') || undefined,
                          item.label,
                        )}
                      </td>
                      <td>
                        {item.validated
                          ? renderEvalPrediction(item.finalBucket, item.finalState, item.label)
                          : <span className={styles.muted}>—</span>}
                      </td>
                      <td className={styles.muted}>
                        {(item.topConcepts ?? []).map((concept) => (
                          <div key={concept.conceptId}>
                            {concept.kind === 'distractor' ? '·' : '✓'} {concept.name} {concept.rerankProb.toFixed(2)}
                            {concept.inStrategy ? ' (perim)' : ''}
                          </div>
                        ))}
                      </td>
                      <td>
                        <select
                          value={item.label ?? ''}
                          onChange={(event) =>
                            setEvalLabel.mutate({
                              companyKey: item.companyKey,
                              label: event.target.value as SectorEvalLabel | '',
                            })
                          }
                        >
                          <option value="">—</option>
                          <option value="keep">Keep</option>
                          <option value="forse">Forse</option>
                          <option value="scarta">Scarta</option>
                        </select>
                      </td>
                    </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </>
        ) : null}
      </section>
      ) : null}
    </div>
  );
}
