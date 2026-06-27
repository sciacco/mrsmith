import { useState, type ChangeEvent, type FormEvent } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { useMutation } from '@tanstack/react-query';
import { Button, Icon, Skeleton, ToggleSwitch } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import type { CompanySearchRow, DomainResolutionResponse, OpenAPIITEnvelope, WebSearchResponse } from '../api/types';
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
type TestTab = 'company' | 'domain' | 'keyword' | 'maintenance';

const testTabs = [
  { id: 'company', label: 'Company search', icon: 'database' },
  { id: 'domain', label: 'Domain resolver', icon: 'network' },
  { id: 'keyword', label: 'Keyword evidence', icon: 'search' },
  { id: 'maintenance', label: 'Manutenzione', icon: 'settings' },
] as const;

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

  const companyData = companySearch.data?.data;
  const companyRows: CompanySearchRow[] = Array.isArray(companyData)
    ? (companyData as CompanySearchRow[])
    : [];
  const dryRunCount = findMetric(companySearch.data, countHints);
  const dryRunPrice = findMetric(companySearch.data, priceHints);
  const companyScopeLabel = companyFilterSummary(companyFilters);

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
