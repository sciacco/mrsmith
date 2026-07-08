import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Modal, Skeleton, Tooltip, useToast } from '@mrsmith/ui';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import { DeepAnalysisContent } from '../../components/deep/DeepComponents';
import { RatingStars } from '../../components/RatingStars';
import { writeCohort } from '../../components/scheda/cohort';
import { ThesisReadingPanel } from '../../components/ThesisReadingPanel/ThesisReadingPanel';
import type {
  MAGatedProgressResponse,
  MASessionDetail,
  MAManualAddTargetRequest,
  MADeepAnalysis,
  MATarget,
  MATargetListResponse,
  MATargetRow,
  MASessionThesisReading,
  MAVerificationQueueItem,
  MAVerificationQueueResponse,
  MAThesis,
} from '../../api/types';
import {
  bucketChipDescription,
  bucketChipLabel,
  bucketLabel,
  dateLabel,
  downloadBlob,
  errorLabel,
  gatedBucketCounts,
  isGateReject,
  isOperationalGateReject,
  normalizeSearchLimit,
  numberFormat,
  safeFilename,
  sessionStatusLabel,
  targetKey,
  targetsToCSV,
} from './helpers';
import styles from './Ricerche.module.css';

const registryFactLabels: Record<string, { label: string; tone: 'warn' | 'info' }> = {
  non_vende: { label: 'Non vende', tone: 'warn' },
  in_trattativa_altrui: { label: 'In trattativa con altri', tone: 'warn' },
  da_evitare: { label: 'Da evitare', tone: 'warn' },
  gia_cliente: { label: 'Già cliente', tone: 'info' },
  partner: { label: 'Partner', tone: 'info' },
};

const thesisOptions: { value: MAThesis; label: string }[] = [
  { value: 'generico', label: 'Generico' },
  { value: 'successione', label: 'Successione' },
  { value: 'crescita', label: 'Crescita' },
  { value: 'consolidamento', label: 'Consolidamento' },
  { value: 'tuck_in', label: 'Competenze' },
];

const moneyFormat = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  maximumFractionDigits: 0,
});

type ActiveTab = 'results' | 'queue' | 'outside';

type ManualAddErrors = Partial<Record<'vatCode' | 'domain' | 'form', string>>;
type ManualAddLocalState = { vatCode: string; domain?: string; submittedAt: string };

const manualAddActiveStatuses = new Set(['queued', 'pending', 'running', 'processing']);

function isManualAddActive(job?: MAGatedProgressResponse['manualAdd'] | null): boolean {
  return Boolean(job?.status && manualAddActiveStatuses.has(job.status));
}

function manualAddJobErrorCopy(errorCode?: string): { title: string; detail: string } {
  switch (errorCode) {
    case 'vat_not_found':
      return { title: 'P.IVA non trovata nel registro', detail: 'Verifica l’identificativo e riprova con una P.IVA o un codice fiscale valido.' };
    case 'openapiit_unavailable':
      return { title: 'Registro aziende temporaneamente non disponibile', detail: 'Riprova tra poco: l’inserimento non è stato completato.' };
    case 'brave_unavailable':
      return { title: 'Verifica dominio temporaneamente non disponibile', detail: 'Riprova tra poco oppure indica il dominio ufficiale se lo conosci.' };
    case 'manual_add_failed':
      return { title: 'Inserimento non completato', detail: 'Verifica i dati inseriti e riprova.' };
    default:
      return { title: 'Inserimento non completato', detail: 'Controlla P.IVA, codice fiscale o dominio e riprova.' };
  }
}

function normalizeManualVat(value: string): string {
  return value.trim().toUpperCase();
}

function validateManualVat(value: string): string | null {
  const normalized = normalizeManualVat(value);
  if (!normalized) return 'Inserisci una P.IVA o un codice fiscale.';
  if (/^\d{11}$/.test(normalized) || /^[A-Z0-9]{16}$/.test(normalized)) return null;
  return 'Usa 11 cifre oppure 16 caratteri alfanumerici.';
}

function schedaAziendaHref(companyKey: string, lens: 'ricerca' | 'iniziativa', lensId: string): string {
  return `/aziende/${encodeURIComponent(companyKey)}?${lens}=${encodeURIComponent(lensId)}`;
}

function isValidManualDomainInput(value: string): boolean {
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

function validateManualDomain(value: string): string | null {
  if (isValidManualDomainInput(value)) return null;
  return 'Inserisci un dominio valido, es. azienda.it.';
}

function apiBodyMessage(error: ApiError): string | undefined {
  const body = error.body;
  if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') return body.error;
  if (body && typeof body === 'object' && 'message' in body && typeof body.message === 'string') return body.message;
  return undefined;
}

function manualAddErrorLabel(error: unknown): string {
  if (error instanceof ApiError) {
    const message = apiBodyMessage(error);
    if (error.status === 409 && message?.includes('già presente')) return 'Azienda già presente nella ricerca';
    if (message?.includes('P.IVA non trovata')) return 'P.IVA non trovata nel registro';
    if (message?.includes('Inserimento manuale già in corso')) return 'Inserimento manuale già in corso per questa ricerca.';
    if (message === 'invalid_domain' || message?.includes('domain')) return 'Dominio non valido.';
    if (message === 'invalid_vat' || message === 'invalid_ma_request' || message?.includes('vat')) return 'P.IVA o codice fiscale non valido.';
  }
  return errorLabel(error);
}

function describedBy(...ids: Array<string | false | null | undefined>): string | undefined {
  const value = ids.filter(Boolean).join(' ');
  return value || undefined;
}

export function RicercaDetailPage() {
  const { id } = useParams<{ id: string }>();
  const api = useApiClient();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [detail, setDetail] = useState<MASessionDetail | null>(null);
  const [rows, setRows] = useState<MATargetRow[]>([]);
  const [progress, setProgress] = useState<MAGatedProgressResponse | null>(null);
  const [queue, setQueue] = useState<MAVerificationQueueItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [rowsLoading, setRowsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<ActiveTab>('results');
  const [selectedRow, setSelectedRow] = useState<MATargetRow | null>(null);
  // Coorte visibile della tabella funnel (post filtri/ordinamento) catturata
  // all'apertura del dettaglio: alimenta il link scheda del modal (Staffetta).
  const [selectedCohortKeys, setSelectedCohortKeys] = useState<string[]>([]);
  const [targetCache, setTargetCache] = useState<Record<string, MATarget>>({});
  const [targetLoading, setTargetLoading] = useState(false);
  const [targetError, setTargetError] = useState<string | null>(null);
  const [deepLaunchFor, setDeepLaunchFor] = useState<string | null>(null);
  const [deepLaunchError, setDeepLaunchError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [thesis, setThesis] = useState<MAThesis>('generico');
  const [associateFor, setAssociateFor] = useState<string | null>(null);
  const [associateDomain, setAssociateDomain] = useState('');
  const [noWebsiteItem, setNoWebsiteItem] = useState<MAVerificationQueueItem | null>(null);
  const [reprocessingKeys, setReprocessingKeys] = useState<Set<string>>(() => new Set());
  const [forceRerun, setForceRerun] = useState(false);
  const [excludeCandidate, setExcludeCandidate] = useState<MATargetRow | null>(null);
  const [excludeReason, setExcludeReason] = useState('');
  const [manualAddOpen, setManualAddOpen] = useState(false);
  const [manualAddVat, setManualAddVat] = useState('');
  const [manualAddDomain, setManualAddDomain] = useState('');
  const [manualAddErrors, setManualAddErrors] = useState<ManualAddErrors>({});
  const [manualAddSubmitting, setManualAddSubmitting] = useState(false);
  const [manualAddLocal, setManualAddLocal] = useState<ManualAddLocalState | null>(null);
  const rowsTerminalKey = useRef('');
  const manualAddTerminalKey = useRef('');

  const loadStatus = useCallback(async () => {
    if (!id) return;
    const [detailData, progressData, queueData] = await Promise.all([
      api.get<MASessionDetail>(`/binocolo/v1/ma/sessions/${id}?targets=none`),
      api.get<MAGatedProgressResponse>(`/binocolo/v1/ma/sessions/${id}/gated-progress`),
      api.get<MAVerificationQueueResponse>(`/binocolo/v1/ma/sessions/${id}/verification-queue`),
    ]);
    setDetail(detailData);
    setProgress(progressData);
    setQueue(queueData.items);
    setThesis(detailData.strategy?.strategy.thesis ?? 'generico');
    if (detailData.session.status !== 'running') {
      setReprocessingKeys(new Set());
    }
  }, [api, id]);

  const loadRows = useCallback(async () => {
    if (!id) return;
    setRowsLoading(true);
    try {
      const data = await api.get<MATargetListResponse>(`/binocolo/v1/ma/sessions/${id}/targets`);
      setRows(data.items);
      setTargetCache({});
    } finally {
      setRowsLoading(false);
    }
  }, [api, id]);

  const loadAll = useCallback(
    async (includeRows = true) => {
      if (!id) return;
      setError(null);
      try {
        await Promise.all([loadStatus(), includeRows ? loadRows() : Promise.resolve()]);
      } catch (err) {
        setError(errorLabel(err));
      } finally {
        setLoading(false);
      }
    },
    [id, loadRows, loadStatus],
  );

  useEffect(() => {
    void loadAll();
  }, [loadAll]);

  useEffect(() => {
    if (!detail || !progress) return;
    const shouldPoll =
      detail.session.status === 'running' ||
      progress.stage === 'address' ||
      progress.stage === 'gate' ||
      progress.stage === 'enrich' ||
      isManualAddActive(progress.manualAdd) ||
      manualAddSubmitting;
    if (!shouldPoll) return;
    const refreshRows = detail.session.status === 'running' && (progress.stage === 'ready' || progress.stage === 'failed');
    const handle = setInterval(() => {
      void loadAll(refreshRows);
    }, 4000);
    return () => clearInterval(handle);
  }, [detail, loadAll, manualAddSubmitting, progress]);

  useEffect(() => {
    if (!progress) return;
    const terminal = progress.stage === 'ready' || progress.stage === 'failed';
    if (!terminal) {
      rowsTerminalKey.current = '';
      return;
    }
    const key = `${progress.stage}:${progress.run.completedAt ?? ''}:${progress.run.errorCode ?? ''}`;
    if (rowsTerminalKey.current === key) return;
    rowsTerminalKey.current = key;
    void loadRows().catch((err) => setError(errorLabel(err)));
  }, [loadRows, progress]);

  useEffect(() => {
    const job = progress?.manualAdd;
    if (!job) return;
    if (isManualAddActive(job)) {
      manualAddTerminalKey.current = '';
      return;
    }
    if (job.status !== 'ready' && job.status !== 'failed') return;
    const key = `${job.status}:${job.updatedAt}:${job.errorCode ?? ''}`;
    if (manualAddTerminalKey.current === key) return;
    manualAddTerminalKey.current = key;
    if (job.status === 'ready') {
      setManualAddLocal(null);
      setManualAddErrors({});
    }
    void loadRows().catch((err) => setError(errorLabel(err)));
  }, [loadRows, progress?.manualAdd]);

  const selectedTarget = selectedRow ? targetCache[selectedRow.id] ?? null : null;

  const refetchTargetDetail = useCallback(
    async (row: MATargetRow) => {
      if (!id) return null;
      const target = await api.get<MATarget>(`/binocolo/v1/ma/sessions/${id}/targets/${row.id}`);
      setTargetCache((current) => ({ ...current, [row.id]: target }));
      return target;
    },
    [api, id],
  );

  useEffect(() => {
    if (!id || !selectedRow || targetCache[selectedRow.id]) return;
    let active = true;
    setTargetLoading(true);
    setTargetError(null);
    api
      .get<MATarget>(`/binocolo/v1/ma/sessions/${id}/targets/${selectedRow.id}`)
      .then((target) => {
        if (!active) return;
        setTargetCache((current) => ({ ...current, [selectedRow.id]: target }));
      })
      .catch((err) => {
        if (active) setTargetError(errorLabel(err));
      })
      .finally(() => {
        if (active) setTargetLoading(false);
      });
    return () => {
      active = false;
    };
  }, [api, id, selectedRow, targetCache]);

  const selectedDeepStatus = selectedTarget?.deep?.status;
  const selectedDeepRunning = selectedDeepStatus === 'queued' || selectedDeepStatus === 'running';
  useEffect(() => {
    if (!selectedRow || !selectedDeepRunning) return;
    const handle = setInterval(() => {
      void refetchTargetDetail(selectedRow).catch(() => undefined);
    }, 5000);
    return () => clearInterval(handle);
  }, [refetchTargetDetail, selectedDeepRunning, selectedRow]);

  const outsideTargets = useMemo(() => rows.filter(isOperationalGateReject), [rows]);
  const resultTargets = useMemo(
    () =>
      rows
        .filter((target) => !isOperationalGateReject(target))
        .sort((a, b) => {
          // Le soppresse non hanno rango tra le vive: sempre in coda alla lista.
          const suppressedDelta = Number(a.bucket === 'soppresso') - Number(b.bucket === 'soppresso');
          if (suppressedDelta !== 0) return suppressedDelta;
          const ratingDelta = (b.rating ?? 0) - (a.rating ?? 0);
          if (ratingDelta !== 0) return ratingDelta;
          if (a.score !== b.score) return b.score - a.score;
          return a.companyName.localeCompare(b.companyName);
        }),
    [rows],
  );
  const buckets = gatedBucketCounts(progress);
  const routingCounts = useMemo(
    () => {
      const c = { principale: 0, daVerificare: 0, azionabile: 0, soppresso: 0 };
      for (const r of rows) {
        if (r.bucket === 'principale') c.principale++;
        else if (r.bucket === 'da_verificare') c.daVerificare++;
        else if (r.bucket === 'azionabile') c.azionabile++;
        else if (r.bucket === 'soppresso') c.soppresso++;
      }
      return c;
    },
    [rows],
  );
  const isRunning = detail?.session.status === 'running' || progress?.stage === 'address' || progress?.stage === 'gate' || progress?.stage === 'enrich';
  const isFailed = detail?.session.status === 'failed' || progress?.stage === 'failed';
  const manualAddJob = progress?.manualAdd ?? null;
  const manualAddInFlight = manualAddSubmitting || isManualAddActive(manualAddJob);
  const manualAddFailed = manualAddJob?.status === 'failed';
  const showManualAddNotice = manualAddInFlight || manualAddFailed || Boolean(manualAddLocal && !manualAddJob);

  function openManualAddRetry() {
    if (manualAddInFlight) return;
    if (manualAddLocal) {
      setManualAddVat(manualAddLocal.vatCode);
      setManualAddDomain(manualAddLocal.domain ?? '');
    }
    setManualAddErrors({});
    setManualAddOpen(true);
  }

  async function resumeSearch(force = false) {
    if (!detail?.session.id) return;
    setBusy('resume');
    setError(null);
    try {
      await api.post<MASessionDetail>(`/binocolo/v1/ma/sessions/${detail.session.id}/gated-search?targets=none`, {
        strategyType: 'expanded',
        limit: normalizeSearchLimit(detail.strategy?.strategy.searchLimit),
        force,
      });
      await loadAll();
      toast(force ? 'Ricerca rilanciata con rivalutazione forzata.' : 'Ricerca rilanciata.', 'success');
    } catch (err) {
      setError(errorLabel(err));
      toast(errorLabel(err), 'error');
    } finally {
      setBusy(null);
    }
  }

  async function rescore() {
    if (!detail?.session.id) return;
    setBusy('rescore');
    setError(null);
    try {
      const data = await api.post<MASessionDetail>(`/binocolo/v1/ma/sessions/${detail.session.id}/rescore?targets=none`, { thesis });
      setDetail(data);
      await loadAll();
      toast('Shortlist ricalcolata sotto la nuova tesi.', 'success');
    } catch (err) {
      setError(errorLabel(err));
      toast(errorLabel(err), 'error');
    } finally {
      setBusy(null);
    }
  }

  async function exportXLSX() {
    if (!detail?.session.id) return;
    setBusy('export');
    try {
      const blob = await api.postBlob(`/binocolo/v1/ma/sessions/${detail.session.id}/export`, { format: 'xlsx' });
      downloadBlob(blob, `ricerca-${safeFilename(detail.session.title)}.xlsx`);
    } catch (err) {
      setError(errorLabel(err));
      toast(errorLabel(err), 'error');
    } finally {
      setBusy(null);
    }
  }

  function exportCSV() {
    if (!detail) return;
    const blob = new Blob([targetsToCSV(resultTargets)], { type: 'text/csv;charset=utf-8' });
    downloadBlob(blob, `ricerca-${safeFilename(detail.session.title)}.csv`);
  }

  function closeManualAddModal() {
    if (manualAddSubmitting) return;
    setManualAddOpen(false);
    setManualAddVat('');
    setManualAddDomain('');
    setManualAddErrors({});
  }

  async function submitManualAdd(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const vatCode = normalizeManualVat(manualAddVat);
    const domain = manualAddDomain.trim();
    const nextErrors: ManualAddErrors = {};
    const vatError = validateManualVat(manualAddVat);
    const domainError = validateManualDomain(manualAddDomain);
    if (vatError) nextErrors.vatCode = vatError;
    if (domainError) nextErrors.domain = domainError;
    if (nextErrors.vatCode || nextErrors.domain) {
      setManualAddErrors(nextErrors);
      return;
    }
    const sessionId = detail?.session.id ?? id;
    if (!sessionId) return;
    if (manualAddInFlight) {
      setManualAddErrors({ form: 'Inserimento manuale già in corso per questa ricerca.' });
      return;
    }
    const payload: MAManualAddTargetRequest = domain ? { vatCode, domain } : { vatCode };
    setManualAddSubmitting(true);
    setManualAddErrors({});
    try {
      const data = await api.post<MASessionDetail>(`/binocolo/v1/ma/sessions/${sessionId}/targets/manual?targets=none`, payload);
      setDetail(data);
      setManualAddLocal({ vatCode, ...(domain ? { domain } : {}), submittedAt: new Date().toISOString() });
      setManualAddOpen(false);
      setManualAddVat('');
      setManualAddDomain('');
      toast('Azienda in inserimento.', 'success');
      await loadAll();
    } catch (err) {
      const message = manualAddErrorLabel(err);
      if (message.includes('Dominio')) setManualAddErrors({ domain: message });
      else if (message.includes('P.IVA') || message.includes('codice fiscale')) setManualAddErrors({ vatCode: message });
      else setManualAddErrors({ form: message });
    } finally {
      setManualAddSubmitting(false);
    }
  }

  async function rateTarget(target: MATargetRow, rating: number) {
    if (!detail?.session.id) return;
    if (rating === -1) {
      // L'esclusione chiede il motivo (ground truth per la calibrazione futura):
      // la chiamata parte dal modal di conferma.
      setExcludeReason('');
      setExcludeCandidate(target);
      return;
    }
    const nextRating = target.rating === rating ? 0 : rating;
    await submitRating(target, nextRating, '');
  }

  async function submitRating(target: MATargetRow, rating: number, reason: string) {
    if (!detail?.session.id) return;
    const sessionId = detail.session.id;
    const previousRows = rows;
    setRows((current) =>
      current.map((item) => (targetKey(item) === targetKey(target) ? { ...item, rating: rating === 0 ? undefined : rating } : item)),
    );
    setTargetCache((current) => {
      const cached = current[target.id];
      if (!cached) return current;
      return { ...current, [target.id]: { ...cached, rating: rating === 0 ? undefined : rating } };
    });
    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${sessionId}/rating`, {
        companyKey: targetKey(target),
        rating,
        ...(rating > 0 ? { scoreAtRating: target.score, confidenceAtRating: target.confidence } : {}),
        ...(reason ? { reason } : {}),
      });
    } catch (err) {
      toast(errorLabel(err), 'error');
      setRows(previousRows);
      await loadAll();
    }
  }

  async function submitAssociate(companyKey: string) {
    if (!detail?.session.id || !associateDomain.trim()) return;
    setBusy(`associate:${companyKey}`);
    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${detail.session.id}/associate-domain?targets=none`, {
        companyKey,
        domain: associateDomain.trim(),
      });
      setReprocessingKeys((current) => new Set(current).add(companyKey));
      setAssociateFor(null);
      setAssociateDomain('');
      await loadAll();
    } catch (err) {
      setError(errorLabel(err));
      toast(errorLabel(err), 'error');
    } finally {
      setBusy(null);
    }
  }

  async function confirmGroupSite(item: MAVerificationQueueItem) {
    if (!detail?.session.id) return;
    setBusy(`group:${item.companyKey}`);
    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${detail.session.id}/confirm-group-site?targets=none`, {
        companyKey: item.companyKey,
      });
      setReprocessingKeys((current) => new Set(current).add(item.companyKey));
      await loadAll();
    } catch (err) {
      setError(errorLabel(err));
      toast(errorLabel(err), 'error');
    } finally {
      setBusy(null);
    }
  }

  async function declareNoWebsite() {
    if (!detail?.session.id || !noWebsiteItem) return;
    const item = noWebsiteItem;
    setBusy(`no-website:${item.companyKey}`);
    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${detail.session.id}/no-website?targets=none`, {
        companyKey: item.companyKey,
      });
      setReprocessingKeys((current) => new Set(current).add(item.companyKey));
      setNoWebsiteItem(null);
      await loadAll();
    } catch (err) {
      setError(errorLabel(err));
      toast(errorLabel(err), 'error');
    } finally {
      setBusy(null);
    }
  }

  async function startTargetDeepAnalysis(row: MATargetRow, target: MATarget) {
    const companyKey = target.companyKey || row.companyKey || target.vatCode || row.vatCode || target.taxCode || target.id;
    if (!companyKey) {
      setDeepLaunchError('Chiave azienda non disponibile.');
      return;
    }
    setDeepLaunchFor(companyKey);
    setDeepLaunchError(null);
    try {
      const response = await api.post<{ status: MADeepAnalysis['status'] }>(
        `/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/deep-dive`,
        {},
      );
      setTargetCache((current) => {
        const cached = current[row.id];
        if (!cached) return current;
        const nextDeep: MADeepAnalysis = {
          ...(cached.deep ?? { companyKey }),
          companyKey,
          status: response.status,
        };
        return { ...current, [row.id]: { ...cached, deep: nextDeep } };
      });
      void refetchTargetDetail(row).catch(() => undefined);
      toast(response.status === 'ready' ? 'Analisi già disponibile.' : 'Analisi avviata.', 'success');
    } catch (err) {
      const message = errorLabel(err);
      setDeepLaunchError(message);
      toast(message, 'error');
    } finally {
      setDeepLaunchFor(null);
    }
  }

  if (!id) {
    return <main className={styles.page}><div className={styles.danger}>Ricerca non indicata.</div></main>;
  }

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <span className={styles.eyebrow}>Ricerca</span>
          <h1>{detail?.session.title || 'Ricerca'}</h1>
          <p className={styles.subtitle}>
            {detail?.session.prompt}
            {detail?.session.updatedAt ? ` · aggiornata ${dateLabel(detail.session.updatedAt)}` : ''}
          </p>
        </div>
        <Button variant="secondary" onClick={() => navigate('/ricerche')} leftIcon={<Icon name="arrow-left" />}>
          Ricerche
        </Button>
      </header>

      {error ? (
        <div className={styles.danger} role="alert">
          <Icon name="triangle-alert" size={18} />
          <span>{error}</span>
        </div>
      ) : null}

      {loading && !detail ? (
        <section className={styles.panel}><div className={styles.panelBody}><Skeleton rows={10} /></div></section>
      ) : detail ? (
        <>
          {progress ? (
            <ProgressPanel
              progress={progress}
              status={detail.session.status}
              queueCount={queue.length}
              routingCounts={routingCounts}
              running={isRunning}
              failed={isFailed}
              onResume={(force) => void resumeSearch(force)}
              resumeBusy={busy === 'resume'}
              forceRerun={forceRerun}
              onForceRerunChange={setForceRerun}
            />
          ) : null}

          {showManualAddNotice ? (
            <ManualAddJobNotice
              job={manualAddJob}
              local={manualAddLocal}
              inFlight={manualAddInFlight}
              onRetry={openManualAddRetry}
            />
          ) : null}

          {isRunning ? (
            <section className={styles.panel} aria-labelledby="queue-ghost-title">
              <div className={styles.panelHeader}>
                <div>
                  <h2 id="queue-ghost-title">Coda di verifica · {numberFormat.format(queue.length)}</h2>
                  <p className={styles.hint}>Le righe si accumulano durante la valutazione.</p>
                </div>
              </div>
              <div className={styles.queueRows}>
                {queue.length === 0 ? (
                  <div className={styles.panelBody}><p className={styles.hint}>Nessuna verifica manuale finora.</p></div>
                ) : (
                  queue.map((item) => (
                    <div key={item.targetId || item.companyKey} className={styles.queueRow}>
                      <div>
                        <span className={styles.queueName}>{item.companyName}</span>
                        <p className={styles.queueReason}>{item.reason.detail}</p>
                      </div>
                      {reprocessingKeys.has(item.companyKey) ? (
                        <span className={`${styles.statusPill} ${styles.statusRunning}`}>
                          <span className={styles.pulse} />
                          Nuova valutazione in corso
                        </span>
                      ) : (
                        <span className={styles.hint}>disponibile a ricerca completata</span>
                      )}
                    </div>
                  ))
                )}
              </div>
            </section>
          ) : (
            <section className={styles.panel} aria-labelledby="results-title">
              <div className={styles.panelHeader}>
                <div>
                  <h2 id="results-title">Lavorazione</h2>
                  <p className={styles.hint}>Risultati, rimedi e audit del gate.</p>
                </div>
              </div>
              <div className={styles.panelBody}>
                <div className={styles.summaryStrip}>
                  <span>Superficie <b>{numberFormat.format(progress?.surface.fetched || progress?.surface.expected || rows.length)}</b></span>
                  <span>→</span>
                  <span>valutate <b>{numberFormat.format(progress?.enrich.survivors ?? buckets.keep + buckets.forse + buckets.review)}</b></span>
                  <span>→</span>
                  <span>analizzate <b>{numberFormat.format(progress?.enrich.enriched ?? 0)}</b></span>
                  <span>·</span>
                  <span>coda <b>{numberFormat.format(queue.length)}</b></span>
                  <span>·</span>
                  <span>fuori tesi <b>{numberFormat.format(buckets.reject || outsideTargets.length)}</b></span>
                </div>
              </div>
              <div className={styles.panelBody}>
                <div className={styles.toolbar}>
                  <span className={styles.thesis}>Tesi</span>
                  <select className={styles.select} value={thesis} onChange={(event) => setThesis(event.target.value as MAThesis)}>
                    {thesisOptions.map((item) => (
                      <option key={item.value} value={item.value}>{item.label}</option>
                    ))}
                  </select>
                  <Button onClick={() => setManualAddOpen(true)} leftIcon={<Icon name="plus" />} disabled={manualAddInFlight}>
                    Aggiungi azienda
                  </Button>
                  <Button variant="secondary" onClick={() => void rescore()} loading={busy === 'rescore'} leftIcon={<Icon name="refresh-cw" />}>
                    Cambia tesi e ricalcola
                  </Button>
                  <Button variant="secondary" onClick={() => void exportXLSX()} loading={busy === 'export'} leftIcon={<Icon name="download" />}>
                    Export XLSX
                  </Button>
                  <Button variant="secondary" onClick={exportCSV}>CSV</Button>
                </div>
              </div>
              <div className={styles.tabs} role="tablist" aria-label="Viste risultati">
                <button type="button" role="tab" aria-selected={activeTab === 'results'} className={`${styles.tab} ${activeTab === 'results' ? styles.tabActive : ''}`} onClick={() => setActiveTab('results')}>
                  Risultati
                </button>
                <button type="button" role="tab" aria-selected={activeTab === 'queue'} className={`${styles.tab} ${activeTab === 'queue' ? styles.tabActive : ''}`} onClick={() => setActiveTab('queue')}>
                  Coda di verifica <span className={styles.tabBadge}>{numberFormat.format(queue.length)}</span>
                </button>
                <button type="button" role="tab" aria-selected={activeTab === 'outside'} className={`${styles.tab} ${activeTab === 'outside' ? styles.tabActive : ''}`} onClick={() => setActiveTab('outside')}>
                  Fuori tesi <span className={`${styles.tabBadge} ${styles.tabBadgeGray}`}>{numberFormat.format(outsideTargets.length)}</span>
                </button>
              </div>
              {activeTab === 'results' ? (
                <ResultsTable
                  rows={resultTargets}
                  loading={rowsLoading}
                  sessionId={id}
                  onOpen={(target, cohortKeys) => {
                    setSelectedRow(target);
                    setSelectedCohortKeys(cohortKeys);
                  }}
                  onRate={(target, rating) => void rateTarget(target, rating)}
                  onAdd={() => setManualAddOpen(true)}
                  manualAddDisabled={manualAddInFlight}
                />
              ) : null}
              {activeTab === 'queue' ? (
                <QueueTab
                  items={queue}
                  disabled={detail.session.status === 'running'}
                  busy={busy}
                  associateFor={associateFor}
                  associateDomain={associateDomain}
                  onAssociateFor={(companyKey) => {
                    setAssociateFor(companyKey);
                    setAssociateDomain('');
                  }}
                  onAssociateDomain={setAssociateDomain}
                  onAssociate={(companyKey) => void submitAssociate(companyKey)}
                  onConfirmGroup={(item) => void confirmGroupSite(item)}
                  onNoWebsite={setNoWebsiteItem}
                  onRetry={() => void resumeSearch()}
                />
              ) : null}
              {activeTab === 'outside' ? (
                <OutsideTab
                  rows={outsideTargets}
                  busy={busy}
                  associateFor={associateFor}
                  associateDomain={associateDomain}
                  onAssociateFor={(companyKey) => {
                    setAssociateFor(companyKey);
                    setAssociateDomain('');
                  }}
                  onAssociateDomain={setAssociateDomain}
                  onAssociate={(companyKey) => void submitAssociate(companyKey)}
                />
              ) : null}
            </section>
          )}
        </>
      ) : null}

      <TargetDetailModal
        row={selectedRow}
        target={selectedTarget}
        loading={targetLoading}
        error={targetError}
        sessionId={id}
        cohortKeys={selectedCohortKeys}
        deepLaunchBusy={deepLaunchFor !== null}
        deepLaunchError={deepLaunchError}
        onStartDeepAnalysis={(row, target) => void startTargetDeepAnalysis(row, target)}
        onClose={() => {
          setSelectedRow(null);
          setTargetError(null);
          setDeepLaunchError(null);
        }}
      />

      <ManualAddCompanyModal
        open={manualAddOpen}
        vatCode={manualAddVat}
        domain={manualAddDomain}
        errors={manualAddErrors}
        submitting={manualAddSubmitting}
        onVatCodeChange={(value) => {
          const nextValue = value.toUpperCase();
          setManualAddVat(nextValue);
          if (manualAddErrors.vatCode || manualAddErrors.form) {
            setManualAddErrors((current) => ({
              ...current,
              vatCode: current.vatCode ? validateManualVat(nextValue) ?? undefined : current.vatCode,
              form: undefined,
            }));
          }
        }}
        onDomainChange={(value) => {
          setManualAddDomain(value);
          if (manualAddErrors.domain || manualAddErrors.form) {
            setManualAddErrors((current) => ({
              ...current,
              domain: current.domain ? validateManualDomain(value) ?? undefined : current.domain,
              form: undefined,
            }));
          }
        }}
        onSubmit={(event) => void submitManualAdd(event)}
        onClose={closeManualAddModal}
      />

      <Modal
        open={noWebsiteItem !== null}
        onClose={() => setNoWebsiteItem(null)}
        title="Nessun sito ufficiale"
        size="sm"
        dismissible={!busy?.startsWith('no-website:')}
      >
        <div className={styles.stack}>
          <p className={styles.hint}>
            {noWebsiteItem?.companyName ?? 'L azienda'} viene dichiarata senza sito ufficiale. La dichiarazione è permanente e vale anche per le ricerche future.
          </p>
          <p className={styles.hint}>
            L'azienda prosegue sui soli dati strutturati, è inclusa nell'analisi completa e resta marcata "nessuna evidenza web".
          </p>
          <div className={styles.modalActions}>
            <Button onClick={() => void declareNoWebsite()} loading={Boolean(busy?.startsWith('no-website:'))}>Conferma</Button>
            <Button variant="secondary" onClick={() => setNoWebsiteItem(null)} disabled={Boolean(busy?.startsWith('no-website:'))}>Annulla</Button>
          </div>
        </div>
      </Modal>

      <Modal
        open={excludeCandidate !== null}
        onClose={() => setExcludeCandidate(null)}
        title="Escludi target"
        size="sm"
      >
        <div className={styles.excludeModalBody}>
          <p>
            &ldquo;{excludeCandidate?.companyName ?? ''}&rdquo; esce dalla shortlist. Il motivo alimenta la
            calibrazione futura del punteggio.
          </p>
          <div className={styles.excludeReasons}>
            {['Fuori settore', 'Troppo piccola', 'Distress', 'Non in vendita'].map((preset) => (
              <button
                key={preset}
                type="button"
                className={`${styles.excludeChip} ${excludeReason === preset ? styles.excludeChipActive : ''}`}
                onClick={() => setExcludeReason(preset)}
              >
                {preset}
              </button>
            ))}
          </div>
          <input
            type="text"
            className={styles.excludeReasonInput}
            placeholder="Motivo libero (opzionale)"
            value={excludeReason}
            onChange={(event) => setExcludeReason(event.target.value)}
            maxLength={300}
          />
          <div className={styles.modalActions} style={{ justifyContent: 'flex-end' }}>
            <Button variant="secondary" onClick={() => setExcludeCandidate(null)}>
              Annulla
            </Button>
            <Button
              variant="danger"
              onClick={() => {
                const target = excludeCandidate;
                setExcludeCandidate(null);
                if (target) void submitRating(target, -1, excludeReason.trim());
              }}
              leftIcon={<Icon name="x-circle" size={16} />}
            >
              Escludi
            </Button>
          </div>
        </div>
      </Modal>
    </main>
  );
}

function ProgressPanel({
  progress,
  status,
  queueCount,
  running,
  failed,
  onResume,
  resumeBusy,
  forceRerun,
  onForceRerunChange,
  routingCounts,
}: {
  progress: MAGatedProgressResponse;
  status: MASessionDetail['session']['status'];
  queueCount: number;
  routingCounts: { principale: number; daVerificare: number; azionabile: number; soppresso: number };
  running: boolean;
  failed: boolean;
  onResume: (force?: boolean) => void;
  resumeBusy: boolean;
  forceRerun: boolean;
  onForceRerunChange: (force: boolean) => void;
}) {
  const buckets = gatedBucketCounts(progress);
  const total = Math.max(1, buckets.total);
  const done = progress.stage === 'ready' || progress.stage === 'failed';
  const [collapsed, setCollapsed] = useState(done);

  useEffect(() => {
    if (done) setCollapsed(true);
  }, [done]);

  return (
    <section className={styles.panel} aria-labelledby="progress-title">
      <div
        className={styles.panelHeader}
        onClick={() => { if (done) setCollapsed(!collapsed); }}
        style={done ? { cursor: 'pointer' } : undefined}
      >
        <div>
          {done && !failed && !collapsed ? (
            <div className={styles.rerunControls} onClick={(event) => event.stopPropagation()}>
              <span id="progress-title" className={styles.srOnly}>Esecuzione completata</span>
              <Button onClick={() => onResume(forceRerun)} loading={resumeBusy} leftIcon={<Icon name="refresh-cw" />}>
                Riesegui
              </Button>
              <label className={styles.forceCheck}>
                <input
                  type="checkbox"
                  checked={forceRerun}
                  onChange={(event) => onForceRerunChange(event.target.checked)}
                />
                <span>Forza</span>
              </label>
            </div>
          ) : (
            <>
              <h2 id="progress-title">{collapsed ? 'Esecuzione completata' : 'Avanzamento'}</h2>
              {collapsed ? (
                <p className={styles.hint}>
                  In tesi <b>{numberFormat.format(routingCounts.principale)}</b>
                  {' '}(di cui analizzate <b>{numberFormat.format(progress.enrich.enriched)}</b>)
                  {' · '}Da verificare <b>{numberFormat.format(routingCounts.daVerificare + routingCounts.azionabile)}</b>
                  {' · '}Soppresso <b>{numberFormat.format(routingCounts.soppresso)}</b>
                </p>
              ) : (
                <p className={styles.hint}>La valutazione richiede tempo. La pagina si aggiorna da sola.</p>
              )}
            </>
          )}
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {collapsed && done ? <Icon name="chevron-down" size={14} className={styles.hint} /> : null}
          <span className={statusPillClass(status, running, failed)}>
            {running ? <span className={styles.pulse} /> : null}
            {running ? 'Ricerca in corso' : failed ? 'Ricerca interrotta' : sessionStatusLabel(status)}
          </span>
        </div>
      </div>
      {!collapsed ? (
        <div className={styles.panelBody}>
        {failed ? (
          <div className={styles.stack}>
            <div className={styles.danger}>
              <Icon name="triangle-alert" size={18} />
              <span>
                <b>Ricerca interrotta</b> — errore del fornitore dati durante l'arricchimento. Le aziende già elaborate sono conservate: la ripresa riparte da dove si era fermata.
              </span>
            </div>
            <Button onClick={() => onResume(false)} loading={resumeBusy} leftIcon={<Icon name="refresh-cw" />}>Riprendi la ricerca</Button>
          </div>
        ) : (
          <>
            <div className={styles.stages}>
              <StageCard
                name="Superficie"
                state={progress.stage === 'address' ? 'In corso' : 'Completato'}
                tone={progress.stage === 'address' ? 'running' : 'done'}
                value={progress.surface.fetched || progress.surface.expected}
                detail="aziende individuate"
              />
              <StageCard
                name="Valutazione attività"
                state={progress.stage === 'gate' ? 'In corso' : progress.stage === 'ready' || progress.gate.processed >= progress.gate.total ? 'Completato' : 'In attesa'}
                tone={progress.stage === 'gate' ? 'running' : progress.stage === 'ready' || progress.gate.processed >= progress.gate.total ? 'done' : 'waiting'}
                value={progress.gate.processed}
                detail={`/ ${numberFormat.format(progress.gate.total)} valutate`}
              />
              <StageCard
                name="Analisi e punteggio"
                state={progress.stage === 'enrich' ? 'In corso' : progress.stage === 'ready' ? 'Completato' : 'In attesa'}
                tone={progress.stage === 'enrich' ? 'running' : progress.stage === 'ready' ? 'done' : 'waiting'}
                value={progress.enrich.enriched}
                detail={progress.stage === 'ready' ? 'aziende analizzate' : `/ ${numberFormat.format(progress.enrich.survivors)} analizzate`}
              />
            </div>
            <div className={styles.funnelBar} aria-hidden="true">
              <span className={styles.funnelKeep} style={{ width: `${(buckets.keep / total) * 100}%` }} />
              <span className={styles.funnelForse} style={{ width: `${(buckets.forse / total) * 100}%` }} />
              <span className={styles.funnelReview} style={{ width: `${(buckets.review / total) * 100}%` }} />
              <span className={styles.funnelReject} style={{ width: `${(buckets.reject / total) * 100}%` }} />
            </div>
            <div className={styles.buckets}>
              <BucketLegend
                className={styles.funnelKeep ?? ''}
                label="In tesi"
                count={buckets.keep}
                hint="Attività giudicata aderente agli ambiti descritti nella richiesta: prosegue nell’analisi completa."
              />
              <BucketLegend
                className={styles.funnelForse ?? ''}
                label="Da approfondire"
                count={buckets.forse}
                hint="Aderenza incerta dalle evidenze web: inclusa comunque nell’analisi completa."
              />
              <BucketLegend
                className={styles.funnelReview ?? ''}
                label="Da verificare"
                count={buckets.review || queueCount}
                hint="Identità o sito web non confermati: richiede un’azione nella coda di verifica."
              />
              <BucketLegend
                className={styles.funnelReject ?? ''}
                label="Soppresso"
                count={buckets.reject}
                hint="Società cessata, dormiente o fuori dagli ambiti descritti: esclusa dall’analisi."
              />
            </div>
            <p className={styles.hint} style={{ marginTop: 6 }}>
              Verdetto del gate. A run completato, la riga di riepilogo usa il vocabolario della tabella: «In tesi» e «Da approfondire» diventano la lista di lavoro; «Da verificare» va nella coda di verifica; «Soppresso» è escluso.
            </p>
          </>
        )}
        </div>
      ) : null}
    </section>
  );
}

function StageCard({ name, state, tone, value, detail }: { name: string; state: string; tone: 'done' | 'running' | 'waiting'; value: number; detail: string }) {
  const stateClass = tone === 'done' ? styles.statusDone : tone === 'running' ? styles.statusRunning : styles.statusPill;
  return (
    <div className={styles.stageCard}>
      <span className={styles.stageName}>{name}</span>
      <span className={`${styles.statusPill} ${stateClass}`}>{state}</span>
      <span className={styles.stageCount}>{numberFormat.format(value)}</span>
      <span className={styles.hint}>{detail}</span>
    </div>
  );
}

function BucketLegend({ className, label, count, hint }: { className: string; label: string; count: number; hint: string }) {
  return (
    <Tooltip content={hint}>
      <span className={styles.inlineActions}>
        <span className={`${styles.bucketDot} ${className}`} />
        {label} <b>{numberFormat.format(count)}</b>
      </span>
    </Tooltip>
  );
}

function ManualAddJobNotice({
  job,
  local,
  inFlight,
  onRetry,
}: {
  job?: MAGatedProgressResponse['manualAdd'] | null;
  local: ManualAddLocalState | null;
  inFlight: boolean;
  onRetry: () => void;
}) {
  const failed = job?.status === 'failed';
  const copy = failed ? manualAddJobErrorCopy(job?.errorCode) : null;
  const subject = local?.vatCode ? `P.IVA / CF ${local.vatCode}` : 'azienda richiesta';

  return (
    <section className={`${styles.manualAddNotice} ${failed ? styles.manualAddNoticeError : styles.manualAddNoticeActive}`} role={failed ? 'alert' : 'status'}>
      <div className={styles.manualAddNoticeIcon} aria-hidden="true">
        <Icon name={failed ? 'triangle-alert' : 'clock'} size={18} />
      </div>
      <div className={styles.manualAddNoticeBody}>
        <strong>{failed ? copy?.title : 'Azienda in inserimento'}</strong>
        <p>
          {failed
            ? copy?.detail
            : `${subject}: il sistema sta completando registro, verifica dominio e ricalcolo. La riga comparirà al termine.`}
        </p>
      </div>
      {failed ? (
        <Button variant="secondary" onClick={onRetry} disabled={inFlight} leftIcon={<Icon name="refresh-cw" />}>
          Riprova
        </Button>
      ) : (
        <span className={`${styles.statusPill} ${styles.statusRunning}`}>
          <span className={styles.pulse} />
          In elaborazione
        </span>
      )}
    </section>
  );
}

function ManualAddCompanyModal({
  open,
  vatCode,
  domain,
  errors,
  submitting,
  onVatCodeChange,
  onDomainChange,
  onSubmit,
  onClose,
}: {
  open: boolean;
  vatCode: string;
  domain: string;
  errors: ManualAddErrors;
  submitting: boolean;
  onVatCodeChange: (value: string) => void;
  onDomainChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onClose: () => void;
}) {
  const vatHintId = 'manual-add-vat-hint';
  const vatErrorId = 'manual-add-vat-error';
  const domainHintId = 'manual-add-domain-hint';
  const domainErrorId = 'manual-add-domain-error';

  return (
    <Modal open={open} onClose={onClose} title="Aggiungi azienda" size="md" dismissible={!submitting}>
      <form className={styles.manualAddForm} onSubmit={onSubmit} noValidate>
        <p className={styles.modalCopy}>
          Inserisci un identificativo azienda. Il dominio è opzionale: usalo solo quando conosci il sito ufficiale.
        </p>

        <div className={styles.field}>
          <label htmlFor="manual-add-vat" className={styles.requiredLabel}>
            <span className={styles.dotRequired} aria-hidden="true" />
            <span>P.IVA / codice fiscale</span>
            <span className={styles.srOnly}>obbligatorio</span>
          </label>
          <input
            id="manual-add-vat"
            className={`${styles.input} ${errors.vatCode ? styles.inputError : ''}`}
            value={vatCode}
            onChange={(event) => onVatCodeChange(event.target.value)}
            placeholder="01234567890"
            autoComplete="off"
            inputMode="text"
            required
            aria-invalid={Boolean(errors.vatCode) || undefined}
            aria-describedby={describedBy(vatHintId, errors.vatCode && vatErrorId)}
          />
          <p id={vatHintId} className={styles.fieldHint}>11 cifre oppure 16 caratteri alfanumerici.</p>
          {errors.vatCode ? <p id={vatErrorId} className={styles.fieldError}>{errors.vatCode}</p> : null}
        </div>

        <div className={styles.field}>
          <label htmlFor="manual-add-domain">Dominio</label>
          <input
            id="manual-add-domain"
            className={`${styles.input} ${errors.domain ? styles.inputError : ''}`}
            value={domain}
            onChange={(event) => onDomainChange(event.target.value)}
            placeholder="azienda.it"
            autoComplete="off"
            inputMode="url"
            aria-invalid={Boolean(errors.domain) || undefined}
            aria-describedby={describedBy(domainHintId, errors.domain && domainErrorId)}
          />
          <p id={domainHintId} className={styles.fieldHint}>Se lo conosci, l’inserimento è più affidabile.</p>
          {errors.domain ? <p id={domainErrorId} className={styles.fieldError}>{errors.domain}</p> : null}
        </div>

        {errors.form ? (
          <div className={styles.deepInlineError} role="alert">
            <Icon name="triangle-alert" size={16} />
            <span>{errors.form}</span>
          </div>
        ) : null}

        <div className={`${styles.modalActions} ${styles.manualAddActions}`}>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Annulla
          </Button>
          <Button type="submit" loading={submitting} leftIcon={<Icon name="plus" />}>
            Aggiungi azienda
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function ResultsTable({
  rows,
  loading,
  sessionId,
  onOpen,
  onRate,
  onAdd,
  manualAddDisabled,
}: {
  rows: MATargetRow[];
  loading: boolean;
  sessionId?: string;
  onOpen: (target: MATargetRow, cohortKeys: string[]) => void;
  onRate: (target: MATargetRow, rating: number) => void;
  onAdd: () => void;
  manualAddDisabled: boolean;
}) {
  const [onlyFavorites, setOnlyFavorites] = useState(false);
  const [hideExcluded, setHideExcluded] = useState(false);

  if (loading && rows.length === 0) {
    return <div className={styles.panelBody}><Skeleton rows={8} /></div>;
  }

  const filtered = rows.filter((target) => {
    const rating = target.rating ?? 0;
    if (hideExcluded && rating === -1) return false;
    if (onlyFavorites && rating < 1) return false;
    return true;
  });

  return (
    <div>
      {rows.length > 0 ? (
        <div className={styles.filterRow}>
          <button
            type="button"
            className={`${styles.filterChip} ${onlyFavorites ? styles.filterChipActive : ''}`}
            onClick={() => setOnlyFavorites((v) => !v)}
          >
            Solo preferiti
          </button>
          <button
            type="button"
            className={`${styles.filterChip} ${hideExcluded ? styles.filterChipActive : ''}`}
            onClick={() => setHideExcluded((v) => !v)}
          >
            Nascondi esclusi
          </button>
        </div>
      ) : null}
      {filtered.length === 0 ? (
        <div className={styles.emptyState}>
          <span className={styles.emptyIcon}><Icon name="file-text" size={28} /></span>
          <strong>{rows.length === 0 ? 'Nessun risultato — esegui la ricerca o aggiungi aziende' : 'Nessun risultato con i filtri correnti'}</strong>
          <p>{rows.length === 0 ? 'Puoi mantenere la ricerca come contenitore e inserire aziende manualmente.' : 'Puoi aggiungere manualmente un’azienda anche con i filtri attivi.'}</p>
          <div className={styles.emptyActions}>
            <Button onClick={onAdd} leftIcon={<Icon name="plus" />} disabled={manualAddDisabled}>
              Aggiungi azienda
            </Button>
          </div>
        </div>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Azienda</th>
                <th>Prov.</th>
                <th>Esito</th>
                <th>Punteggio</th>
                <th>Preferenza</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((target) => (
                <tr key={target.id} className={`${styles.clickRow} ${(target.rating ?? 0) === -1 ? styles.rowExcluded : ''}`} onClick={() => onOpen(target, filtered.map(targetKey))}>
              <td>
                <span className={styles.cellStack}>
                  <b>{target.companyName}</b>
                  {target.origin === 'manual' ? (
                    <span className={`${styles.badge} ${styles.badgeManual}`}>Inserita manualmente</span>
                  ) : null}
                  {target.webValidation?.webValidationState === 'no_website_declared' ? (
                    <span className={styles.badge}>nessuna evidenza web</span>
                  ) : null}
                  {(target.registryFacts?.length || target.inLavorazione?.length) ? (
                    <span className={styles.cellBadges}>
                      {(target.registryFacts ?? []).map((kind) => {
                        const meta = registryFactLabels[kind];
                        if (!meta) return null;
                        const toneClass = meta.tone === 'warn' ? styles.badgeWarn : styles.badgeInfo;
                        return (
                          <span key={kind} className={`${styles.badge} ${toneClass}`}>{meta.label}</span>
                        );
                      })}
                      {(target.inLavorazione ?? []).map((marker) => (
                        <span key={marker.initiativeId} className={`${styles.badge} ${styles.badgeLav}`}>
                          In lavorazione · {marker.initiativeTitle}
                        </span>
                      ))}
                    </span>
                  ) : null}
                </span>
              </td>
              <td>{target.province ?? '-'}</td>
              <td>
                {bucketChipLabel(target.bucket) ? (
                  <Tooltip content={bucketChipDescription(target.bucket)}>
                    <span className={bucketClassName(target.bucket)}>{bucketChipLabel(target.bucket)}</span>
                  </Tooltip>
                ) : null}
              </td>
              <td>
                {target.bucket === 'soppresso' || !target.matchState ? (
                  <span className={styles.scoreAbsent}>—</span>
                ) : (
                  <span className={styles.score}>{numberFormat.format(target.score)}</span>
                )}
              </td>
              <td>
                <RatingStars
                  rating={target.rating ?? 0}
                  onRate={(rating) => onRate(target, rating)}
                />
                {sessionId ? (
                  <>
                    <a
                      className={styles.inspectLink}
                      href={schedaAziendaHref(targetKey(target), 'ricerca', sessionId)}
                      target="_blank"
                      rel="noopener noreferrer"
                      onClick={(e) => {
                        e.stopPropagation();
                        writeCohort({ lensType: 'ricerca', lensId: sessionId, companyKeys: filtered.map(targetKey) });
                      }}
                      onAuxClick={(e) => {
                        e.stopPropagation();
                        writeCohort({ lensType: 'ricerca', lensId: sessionId, companyKeys: filtered.map(targetKey) });
                      }}
                    >
                      Apri scheda ↗
                    </a>
                    <a
                      className={styles.inspectLink}
                      href={`/ricerche/${sessionId}/target/${target.id}/inspect`}
                      target="_blank"
                      rel="noopener noreferrer"
                      onClick={(e) => e.stopPropagation()}
                      title="Ispezione completa (nuova tab)"
                    >
                      <Icon name="external-link" size={14} />
                    </a>
                  </>
                ) : null}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
      )}
    </div>
  );
}

function QueueTab({
  items,
  disabled,
  busy,
  associateFor,
  associateDomain,
  onAssociateFor,
  onAssociateDomain,
  onAssociate,
  onConfirmGroup,
  onNoWebsite,
  onRetry,
}: {
  items: MAVerificationQueueItem[];
  disabled: boolean;
  busy: string | null;
  associateFor: string | null;
  associateDomain: string;
  onAssociateFor: (companyKey: string) => void;
  onAssociateDomain: (domain: string) => void;
  onAssociate: (companyKey: string) => void;
  onConfirmGroup: (item: MAVerificationQueueItem) => void;
  onNoWebsite: (item: MAVerificationQueueItem) => void;
  onRetry: () => void;
}) {
  if (items.length === 0) {
    return <div className={styles.emptyState}><span className={styles.emptyIcon}><Icon name="clipboard-check" size={28} /></span><strong>Coda vuota</strong></div>;
  }
  return (
    <div className={styles.queueRows}>
      {items.map((item) => (
        <div key={item.targetId || item.companyKey} className={styles.queueRow}>
          <div>
            <span className={styles.queueName}>{item.companyName}</span>
            <p className={styles.queueReason}>{item.reason.detail}</p>
            {associateFor === item.companyKey ? (
              <AssociateForm
                value={associateDomain}
                disabled={disabled || busy === `associate:${item.companyKey}`}
                busy={busy === `associate:${item.companyKey}`}
                onValue={onAssociateDomain}
                onSubmit={() => onAssociate(item.companyKey)}
              />
            ) : null}
          </div>
          <div className={styles.queueActions}>
            {disabled ? <span className={styles.hint}>disponibile a ricerca completata</span> : null}
            {item.remedies.includes('confirm_group_site') ? (
              <Button size="sm" onClick={() => onConfirmGroup(item)} disabled={disabled} loading={busy === `group:${item.companyKey}`}>
                Conferma sito di gruppo
              </Button>
            ) : null}
            {item.remedies.includes('associate_domain') ? (
              <Button variant="secondary" size="sm" onClick={() => onAssociateFor(item.companyKey)} disabled={disabled}>
                Associa dominio
              </Button>
            ) : null}
            {item.remedies.includes('no_website') ? (
              <Button variant="secondary" size="sm" onClick={() => onNoWebsite(item)} disabled={disabled}>
                Nessun sito
              </Button>
            ) : null}
            {item.remedies.includes('retry_search') ? (
              <Button variant="secondary" size="sm" onClick={onRetry} disabled={disabled} loading={busy === 'resume'}>
                Riprova analisi
              </Button>
            ) : null}
          </div>
        </div>
      ))}
    </div>
  );
}

function OutsideTab({
  rows,
  busy,
  associateFor,
  associateDomain,
  onAssociateFor,
  onAssociateDomain,
  onAssociate,
	}: {
	  rows: MATargetRow[];
	  busy: string | null;
  associateFor: string | null;
  associateDomain: string;
  onAssociateFor: (companyKey: string) => void;
  onAssociateDomain: (domain: string) => void;
  onAssociate: (companyKey: string) => void;
}) {
  if (rows.length === 0) {
    return <div className={styles.emptyState}><span className={styles.emptyIcon}><Icon name="eye" size={28} /></span><strong>Nessuna fuori tesi</strong></div>;
  }
  return (
    <div className={styles.queueRows}>
      {rows.map((target) => {
        const key = targetKey(target);
        return (
          <div key={target.id} className={styles.queueRow}>
            <div>
              <span className={styles.queueName}>{target.companyName}</span>
              <p className={styles.queueReason}>
                {target.webValidation?.finalDecision?.reason ?? 'Fuori perimetro'}
                {target.webValidation?.selectedDomain ? ` — giudicata su ${target.webValidation.selectedDomain}` : ''}
              </p>
              {associateFor === key ? (
                <AssociateForm
                  value={associateDomain}
                  disabled={busy === `associate:${key}`}
                  busy={busy === `associate:${key}`}
                  onValue={onAssociateDomain}
                  onSubmit={() => onAssociate(key)}
                />
              ) : null}
            </div>
            <div className={styles.queueActions}>
              <Button variant="secondary" size="sm" onClick={() => onAssociateFor(key)}>
                Associa dominio corretto
              </Button>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function AssociateForm({
  value,
  disabled,
  busy,
  onValue,
  onSubmit,
}: {
  value: string;
  disabled: boolean;
  busy: boolean;
  onValue: (value: string) => void;
  onSubmit: () => void;
}) {
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit();
  }
  return (
    <form className={styles.associateForm} onSubmit={submit}>
      <input
        className={styles.input}
        value={value}
        onChange={(event) => onValue(event.target.value)}
        placeholder="dominio ufficiale, es. azienda.it"
        disabled={disabled}
      />
      <Button type="submit" size="sm" loading={busy} disabled={disabled || !value.trim()}>
        Associa
      </Button>
    </form>
  );
}

function TargetDetailModal({
  row,
  target,
  loading,
  error,
  sessionId,
  cohortKeys,
  deepLaunchBusy,
  deepLaunchError,
  onStartDeepAnalysis,
  onClose,
}: {
  row: MATargetRow | null;
  target: MATarget | null;
  loading: boolean;
  error: string | null;
  sessionId?: string;
  cohortKeys: string[];
  deepLaunchBusy: boolean;
  deepLaunchError: string | null;
  onStartDeepAnalysis: (row: MATargetRow, target: MATarget) => void;
  onClose: () => void;
}) {
  const api = useApiClient();
  const thesisEnabled = Boolean(target?.deep?.status === 'ready' && sessionId && row?.id);
  const thesisQuery = useQuery({
    queryKey: ['ma-session-thesis-reading', sessionId, row?.id],
    enabled: thesisEnabled,
    queryFn: () => {
      if (!sessionId || !row?.id) throw new Error('Target non disponibile.');
      return api.get<MASessionThesisReading>(`/binocolo/v1/ma/sessions/${sessionId}/targets/${row.id}/thesis-reading`);
    },
    retry: (failureCount, queryError) => !(queryError instanceof ApiError && queryError.status === 404) && failureCount < 2,
  });
  const generateThesis = useMutation({
    mutationFn: () => {
      if (!sessionId || !row?.id) throw new Error('Target non disponibile.');
      return api.post<MASessionThesisReading>(`/binocolo/v1/ma/sessions/${sessionId}/targets/${row.id}/thesis-reading`, {});
    },
    onSuccess: () => void thesisQuery.refetch(),
  });
  useEffect(() => {
    generateThesis.reset();
  }, [row?.id]);
  const thesisNotGenerated = thesisQuery.isError && thesisQuery.error instanceof ApiError && thesisQuery.error.status === 404;
  const thesisQueryError = thesisQuery.isError && !thesisNotGenerated ? errorLabel(thesisQuery.error) : null;

  return (
    <Modal open={row !== null} onClose={onClose} title={target?.companyName ?? row?.companyName ?? 'Dettaglio'} size="wide">
      {loading ? (
        <div className={styles.detailModalBody}>
          <Skeleton rows={8} />
        </div>
      ) : error ? (
        <div className={styles.detailModalBody}>
          <div className={styles.danger} role="alert">
            <Icon name="triangle-alert" size={18} />
            <span>{error}</span>
          </div>
        </div>
      ) : target ? (
        <div className={styles.detailModalBody}>
          {sessionId && row ? (
            <div className={styles.deepActions}>
              <a
                className={styles.inspectLink}
                href={schedaAziendaHref(targetKey(row), 'ricerca', sessionId)}
                target="_blank"
                rel="noopener noreferrer"
                onClick={() => writeCohort({ lensType: 'ricerca', lensId: sessionId, companyKeys: cohortKeys })}
                onAuxClick={() => writeCohort({ lensType: 'ricerca', lensId: sessionId, companyKeys: cohortKeys })}
              >
                Apri scheda ↗
              </a>
            </div>
          ) : null}
          <TargetPriorityMarkers row={row} />
          <TargetIdentitySection target={target} />
          <TargetNumbersSection target={target} />
          <TargetAcquisitionAngle row={row} target={target} />
          <TargetKillCriteria row={row} target={target} />

          <TargetDeepAnalysisSection
            row={row}
            target={target}
            sessionId={sessionId}
            busy={deepLaunchBusy}
            error={deepLaunchError}
            onStart={onStartDeepAnalysis}
          />

          {target.deep?.status === 'ready' && sessionId && row?.id ? (
            <ThesisReadingPanel
              compact
              record={thesisQuery.data}
              loading={thesisQuery.isLoading}
              notGenerated={thesisNotGenerated}
              queryError={thesisQueryError}
              deepReady
              onGenerate={() => generateThesis.mutate()}
              generating={generateThesis.isPending}
              generationError={generateThesis.isError ? errorLabel(generateThesis.error) : null}
              generateLabel="Lettura di tesi"
            />
          ) : null}

          {sessionId && row && target.deep?.status !== 'ready' ? (
            <a
              className={styles.inspectLink}
              href={`/ricerche/${sessionId}/target/${row.id}/inspect`}
              target="_blank"
              rel="noopener noreferrer"
            >
              Ispezione completa <Icon name="external-link" size={14} />
            </a>
          ) : null}

          <TargetScoreSection target={target} />
        </div>
      ) : null}
    </Modal>
  );
}

function TargetPriorityMarkers({ row }: { row: MATargetRow | null }) {
  const registryFacts = row?.registryFacts ?? [];
  const workMarkers = row?.inLavorazione ?? [];
  const isManual = row?.origin === 'manual';
  if (!isManual && registryFacts.length === 0 && workMarkers.length === 0) return null;

  return (
    <div className={styles.targetPriorityMarkers} aria-label="Segnali prioritari">
      {isManual ? <span className={`${styles.badge} ${styles.badgeManual}`}>Inserita manualmente</span> : null}
      {registryFacts.map((kind) => {
        const meta = registryFactLabels[kind] ?? { label: humanizeCode(kind), tone: 'info' as const };
        return (
          <span key={kind} className={`${styles.badge} ${meta.tone === 'warn' ? styles.badgeWarn : styles.badgeInfo}`}>
            {meta.label}
          </span>
        );
      })}
      {workMarkers.map((marker) => (
        <span key={marker.initiativeId} className={`${styles.badge} ${styles.badgeLav}`}>
          In lavorazione · {marker.initiativeTitle}
        </span>
      ))}
    </div>
  );
}

function TargetIdentitySection({ target }: { target: MATarget }) {
  const validation = target.webValidation;
  const selectedDomain = validation?.selectedDomain;
  const domainHref = selectedDomain ? domainLink(selectedDomain) : undefined;
  const businessFit = validation?.candidateMatchAnalysis?.businessFit?.trim();
  const evidenceFor = validation?.candidateMatchAnalysis?.evidenceFor ?? [];
  const evidenceAgainst = validation?.candidateMatchAnalysis?.evidenceAgainst ?? [];
  const hasGateDetails = evidenceFor.length > 0 || evidenceAgainst.length > 0;
  const legalForm = evidenceValue(target, 'legal_form');
  const gate = gateVerdict(validation?.webValidationState, validation?.finalAction);

  return (
    <section className={styles.targetSection} aria-labelledby="target-identity-title">
      <div className={styles.targetSectionHeader}>
        <div>
          <p className={styles.eyebrow}>Chi è e cosa fa</p>
          <h3 id="target-identity-title">{target.companyName}</h3>
        </div>
        {gate ? <span className={`${styles.targetChip} ${chipToneClass(gate.tone)}`}>{gate.label}</span> : null}
      </div>

      <div className={styles.identityGrid}>
        <div>
          <span className={styles.targetMetaLabel}>Attività</span>
          <p className={styles.identityText}>
            {target.atecoDescription || 'Descrizione attività non disponibile'}
            {target.atecoCode ? <small>{target.atecoCode}</small> : null}
          </p>
        </div>
        <div>
          <span className={styles.targetMetaLabel}>Sede</span>
          <p className={styles.identityText}>{[target.town, target.province].filter(Boolean).join(' · ') || 'Sede non disponibile'}</p>
        </div>
        {legalForm || target.activityStatus ? (
          <div>
            <span className={styles.targetMetaLabel}>Stato</span>
            <p className={styles.identityText}>{[legalForm, target.activityStatus].filter(Boolean).join(' · ')}</p>
          </div>
        ) : null}
        {selectedDomain ? (
          <div>
            <span className={styles.targetMetaLabel}>Dominio selezionato</span>
            <a className={styles.targetDomainLink} href={domainHref} target="_blank" rel="noopener noreferrer">
              {selectedDomain}
            </a>
          </div>
        ) : null}
      </div>

      {businessFit ? <p className={styles.targetDecisionText}>{businessFit}</p> : null}
      {!businessFit && target.webValidation?.finalDecision?.reason ? (
        <p className={styles.targetDecisionText}>{target.webValidation.finalDecision.reason}</p>
      ) : null}

      {hasGateDetails ? (
        <details className={styles.targetDetails}>
          <summary>Dettagli del verdetto</summary>
          <div className={styles.evidenceColumns}>
            {evidenceFor.length > 0 ? <EvidenceTextList title="A favore" items={evidenceFor} /> : null}
            {evidenceAgainst.length > 0 ? <EvidenceTextList title="Contro" items={evidenceAgainst} /> : null}
          </div>
        </details>
      ) : null}
    </section>
  );
}

function TargetNumbersSection({ target }: { target: MATarget }) {
  const revenuePerEmployee = revenuePerEmployeeValue(target);
  const trend = evidenceValue(target, 'turnover_trend') ?? evidenceValueByPattern(target, /^[+-]\d/);

  return (
    <section className={styles.targetSection} aria-labelledby="target-numbers-title">
      <div className={styles.targetSectionHeader}>
        <div>
          <p className={styles.eyebrow}>I tre numeri</p>
          <h3 id="target-numbers-title">Dimensione operativa</h3>
        </div>
        {trend ? <span className={`${styles.targetChip} ${chipToneClass(trend.startsWith('-') ? 'warning' : 'success')}`}>{trend}</span> : null}
      </div>
      <div className={styles.metricGrid}>
        <MetricCard label="Fatturato" value={target.turnover != null ? moneyFormat.format(target.turnover) : '—'} detail={target.turnoverYear ? String(target.turnoverYear) : undefined} />
        <MetricCard label="Dipendenti" value={target.employees != null ? numberFormat.format(target.employees) : '—'} />
        <MetricCard label="Ricavo/dipendente" value={revenuePerEmployee ?? '—'} detail={revenuePerEmployee?.includes('/dip') ? undefined : 'derivato'} />
      </div>
    </section>
  );
}

function TargetAcquisitionAngle({ row, target }: { row: MATargetRow | null; target: MATarget }) {
  const flags = combinedFlags(row, target);
  const ownerAge = evidenceValue(target, 'succession_owner');
  const companyAge = evidenceValue(target, 'company_age');
  const chips: Array<{ key: string; label: string }> = [];

  for (const code of ['ricambio_generazionale', 'socio_unico', 'impresa_familiare', 'controllo_holding']) {
    const flag = flags.get(code);
    const adjustment = (target.adjustments ?? []).find((item) => item.code === code);
    if (!flag && !adjustment) continue;
    const label = flag?.label ?? adjustment?.label ?? humanizeCode(code);
    const suffix = code === 'ricambio_generazionale' && ownerAge ? ` · ${ownerAge}` : '';
    chips.push({ key: code, label: `${label}${suffix}` });
  }
  if (companyAge) chips.push({ key: 'company_age', label: `Anzianità · ${companyAge}` });

  return (
    <section className={styles.targetSection} aria-labelledby="target-acquisition-title">
      <div className={styles.targetSectionHeader}>
        <div>
          <p className={styles.eyebrow}>Angolo d'acquisto</p>
          <h3 id="target-acquisition-title">Segnali proprietari</h3>
        </div>
      </div>
      {chips.length > 0 ? (
        <div className={styles.targetChipList}>
          {chips.map((chip) => (
            <span key={chip.key} className={`${styles.targetChip} ${chipToneClass('info')}`}>{chip.label}</span>
          ))}
        </div>
      ) : (
        <p className={styles.targetMuted}>Nessun segnale proprietario rilevante nei dati disponibili.</p>
      )}
    </section>
  );
}

function TargetKillCriteria({ row, target }: { row: MATargetRow | null; target: MATarget }) {
  const { kills, caveats } = killCriteria(row, target);
  if (kills.length === 0 && caveats.length === 0) return null;

  return (
    <section className={`${styles.targetSection} ${styles.killSection}`} aria-labelledby="target-kill-title">
      <div className={styles.targetSectionHeader}>
        <div>
          <p className={styles.eyebrow}>Kill criteria</p>
          <h3 id="target-kill-title">Blocchi e caveat</h3>
        </div>
      </div>
      {kills.length > 0 ? (
        <div className={styles.alertList}>
          {kills.map((item) => (
            <div key={item.key} className={item.tone === 'danger' ? styles.targetDanger : styles.targetWarning}>
              <Icon name="triangle-alert" size={16} />
              <span><b>{item.label}</b>{item.detail ? ` · ${item.detail}` : ''}</span>
            </div>
          ))}
        </div>
      ) : null}
      {caveats.length > 0 ? (
        <div className={styles.caveatBlock}>
          <span className={styles.targetMetaLabel}>Caveat informativi</span>
          <div className={styles.targetChipList}>
            {caveats.map((item) => (
              <span key={item.key} className={`${styles.targetChip} ${chipToneClass('warning')}`}>{item.label}</span>
            ))}
          </div>
        </div>
      ) : null}
    </section>
  );
}

function TargetScoreSection({ target }: { target: MATarget }) {
  const adjustments = target.adjustments ?? [];
  const hasDetails = target.evidence.length > 0 || adjustments.length > 0;

  return (
    <section className={styles.targetSection} aria-labelledby="target-score-title">
      <div className={styles.targetSectionHeader}>
        <div>
          <p className={styles.eyebrow}>Punteggio</p>
          <h3 id="target-score-title">{numberFormat.format(target.score)}</h3>
        </div>
        <div className={styles.scorePills}>
          <span className={`${styles.targetChip} ${chipToneClass('neutral')}`}>{confidenceLabel(target.confidence)}</span>
          <span className={`${styles.targetChip} ${chipToneClass(target.bucket === 'soppresso' ? 'warning' : 'info')}`}>{bucketLabel(target.bucket)}</span>
        </div>
      </div>
      {hasDetails ? (
        <details className={styles.targetDetails}>
          <summary>Dettaglio punteggio</summary>
          <div className={styles.scoreDetailList}>
            {target.evidence.map((item, index) => (
              <div key={`${item.criterion}-${index}`} className={styles.scoreDetailRow}>
                <span>{item.label}</span>
                <span>{item.value || '—'}</span>
                <strong>{item.points != null ? `${numberFormat.format(item.points)} pt` : '—'}</strong>
              </div>
            ))}
            {adjustments.map((item) => (
              <div key={`adjustment-${item.code}`} className={styles.scoreDetailRow}>
                <span>{item.label}</span>
                <span>Fattore</span>
                <strong>{formatFactor(item.factor)}</strong>
              </div>
            ))}
          </div>
        </details>
      ) : null}
    </section>
  );
}

function EvidenceTextList({ title, items }: { title: string; items: string[] }) {
  return (
    <div className={styles.evidenceTextList}>
      <span className={styles.targetMetaLabel}>{title}</span>
      <ul>
        {items.map((item) => <li key={item}>{item}</li>)}
      </ul>
    </div>
  );
}

function MetricCard({ label, value, detail }: { label: string; value: string; detail?: string }) {
  return (
    <div className={styles.metricCard}>
      <span className={styles.metricLabel}>{label}</span>
      <strong>{value}</strong>
      {detail ? <span className={styles.metricDetail}>{detail}</span> : null}
    </div>
  );
}

type ChipTone = 'neutral' | 'info' | 'warning' | 'danger' | 'success';

type KillItem = {
  key: string;
  label: string;
  detail?: string;
  tone: 'warning' | 'danger';
};

function combinedFlags(row: MATargetRow | null, target: MATarget): Map<string, { code: string; label: string }> {
  const flags = new Map<string, { code: string; label: string }>();
  for (const flag of [...(row?.flags ?? []), ...(target.flags ?? [])]) flags.set(flag.code, flag);
  return flags;
}

function evidenceValue(target: MATarget, criterion: string): string | undefined {
  return target.evidence.find((item) => item.criterion === criterion && item.value)?.value;
}

function evidenceValueByPattern(target: MATarget, pattern: RegExp): string | undefined {
  return target.evidence.find((item) => item.value && pattern.test(item.value))?.value;
}

function revenuePerEmployeeValue(target: MATarget): string | undefined {
  const direct = evidenceValue(target, 'revenue_per_employee_min') ?? evidenceValue(target, 'productivity');
  if (direct && !direct.toLowerCase().includes('non valutabile')) return direct;
  if (target.turnover != null && target.employees && target.employees > 0) {
    return `${numberFormat.format(Math.round(target.turnover / target.employees / 1000))}k/dip`;
  }
  return undefined;
}

function gateVerdict(state?: string, action?: string): { label: string; tone: ChipTone } | null {
  if (!state && !action) return null;
  const source = action || state;
  const label = action ? finalActionLabel(action) : webValidationStateLabel(state);
  if (source === 'confirm' || source === 'confirmed') return { label, tone: 'success' };
  if (source === 'reject' || source === 'rejected') return { label, tone: 'danger' };
  if (source === 'needs_domain_review' || source === 'needs_business_validation' || source === 'domain_unresolved' || source === 'unclear') {
    return { label, tone: 'warning' };
  }
  return { label, tone: 'info' };
}

function finalActionLabel(action?: string): string {
  switch (action) {
    case 'confirm':
      return 'Confermata';
    case 'deprioritize':
      return 'Da approfondire';
    case 'reject':
      return 'Respinta';
    case 'needs_domain_review':
      return 'Dominio da verificare';
    case 'needs_business_validation':
      return 'Business da verificare';
    case 'no_website_structured':
      return 'Soli dati strutturati';
    default:
      return action ? humanizeCode(action) : 'Non valutata';
  }
}

function webValidationStateLabel(state?: string): string {
  switch (state) {
    case 'confirmed':
      return 'Confermata';
    case 'deprioritized':
      return 'Declassata';
    case 'domain_unresolved':
      return 'Dominio non risolto';
    case 'analysis_unavailable':
      return 'Analisi web non disponibile';
    case 'no_website_declared':
      return 'Nessun sito dichiarato';
    case 'rejected':
      return 'Respinta';
    case 'unclear':
      return 'Incerta';
    default:
      return state ? humanizeCode(state) : 'Non valutata';
  }
}

function killCriteria(row: MATargetRow | null, target: MATarget): { kills: KillItem[]; caveats: KillItem[] } {
  const flags = combinedFlags(row, target);
  const kills = new Map<string, KillItem>();
  const caveats = new Map<string, KillItem>();
  const addKill = (key: string, label: string, tone: 'warning' | 'danger', detail?: string) => kills.set(key, { key, label, tone, detail });
  const addCaveat = (key: string, label: string) => caveats.set(key, { key, label, tone: 'warning' });

  for (const code of ['cessata_fiscalmente', 'non_attiva']) {
    const flag = flags.get(code);
    if (flag) addKill(code, flag.label, 'danger');
  }
  for (const [code, flag] of flags) {
    if (code.startsWith('knockout_vitalita_')) addKill(code, flag.label, 'danger');
  }
  const viability = (target.adjustments ?? []).find((item) => item.code === 'viability' && item.factor < 1);
  if (viability) addKill('viability', viability.label, 'warning', formatFactor(viability.factor));

  for (const code of ['patrimonio_netto_negativo', 'patrimonio_netto_eroso']) {
    const flag = flags.get(code);
    if (flag) addKill(code, flag.label, 'warning');
  }

  for (const code of ['fuori_settore', 'ateco_fuori_perimetro']) {
    const flag = flags.get(code);
    if (flag) addKill(code, flag.label, 'warning');
  }
  if (isGateReject(target) || (row ? isGateReject(row) : false)) {
    addKill('gate_reject', 'Fuori bersaglio', 'danger', target.webValidation?.finalDecision?.reason);
  }

  for (const code of ['bilancio_assente', 'bilancio_datato']) {
    const flag = flags.get(code);
    if (flag) addCaveat(code, flag.label);
  }

  return { kills: [...kills.values()], caveats: [...caveats.values()] };
}

function chipToneClass(tone: ChipTone): string {
  if (tone === 'success') return styles.targetChipSuccess ?? '';
  if (tone === 'warning') return styles.targetChipWarning ?? '';
  if (tone === 'danger') return styles.targetChipDanger ?? '';
  if (tone === 'info') return styles.targetChipInfo ?? '';
  return styles.targetChipNeutral ?? '';
}

function confidenceLabel(value?: string): string {
  if (!value) return 'Confidenza n/d';
  return `Confidenza ${humanizeCode(value).toLowerCase()}`;
}

function humanizeCode(value: string): string {
  const text = value.replace(/_/g, ' ').trim();
  return text ? text.charAt(0).toUpperCase() + text.slice(1) : value;
}

function domainLink(domain: string): string {
  return /^https?:\/\//i.test(domain) ? domain : `https://${domain}`;
}

function formatFactor(value: number): string {
  return `×${value.toLocaleString('it-IT', { maximumFractionDigits: 2 })}`;
}

function TargetDeepAnalysisSection({
  row,
  target,
  sessionId,
  busy,
  error,
  onStart,
}: {
  row: MATargetRow | null;
  target: MATarget;
  sessionId?: string;
  busy: boolean;
  error: string | null;
  onStart: (row: MATargetRow, target: MATarget) => void;
}) {
  const deep = target.deep;
  const status = deep?.status;
  const inspectorHref = sessionId && row ? `/ricerche/${sessionId}/target/${row.id}/inspect` : undefined;
  const canLaunch = Boolean(row && !busy);

  return (
    <section className={styles.deepAnalysisSection} aria-labelledby="target-deep-analysis-title">
      <div className={styles.deepAnalysisHeader}>
        <div>
          <h3 id="target-deep-analysis-title">Analisi approfondita</h3>
          <p>Stato dell'analisi azienda.</p>
        </div>
        <span className={deepStatusClassName(status)}>
          {status === 'queued' || status === 'running' ? <span className={styles.pulse} /> : null}
          {deepStatusLabel(status)}
        </span>
      </div>

      {!status ? (
        <div className={styles.deepAnalysisBody}>
          <p>Nessuna analisi approfondita disponibile.</p>
          <Button size="sm" onClick={() => row && onStart(row, target)} loading={busy} disabled={!canLaunch}>
            Avvia analisi
          </Button>
        </div>
      ) : status === 'queued' || status === 'running' ? (
        <div className={styles.deepAnalysisBody}>
          <p>{status === 'queued' ? 'Analisi in coda.' : 'Analisi in corso.'} Il risultato viene aggiornato automaticamente.</p>
          {deep?.updatedAt ? <span className={styles.deepAnalysisMeta}>Aggiornata {dateLabel(deep.updatedAt)}</span> : null}
        </div>
      ) : status === 'failed' ? (
        <div className={styles.deepAnalysisBody}>
          <p>Analisi non riuscita. Puoi avviarla di nuovo.</p>
          <Button size="sm" onClick={() => row && onStart(row, target)} loading={busy} disabled={!canLaunch}>
            Avvia analisi
          </Button>
        </div>
      ) : (
        <div className={styles.deepAnalysisBody}>
          <DeepAnalysisContent deep={deep} variant="summary" />
          <div className={styles.deepActions}>
            {deep?.updatedAt ? <span className={styles.deepAnalysisMeta}>Aggiornata {dateLabel(deep.updatedAt)}</span> : null}
            {inspectorHref ? (
              <a className={styles.inspectLink} href={inspectorHref} target="_blank" rel="noopener noreferrer">
                Ispezione completa ↗
              </a>
            ) : null}
          </div>
        </div>
      )}

      {error ? (
        <div className={styles.deepInlineError} role="alert">
          <Icon name="triangle-alert" size={16} />
          <span>{error}</span>
        </div>
      ) : null}
    </section>
  );
}

function deepStatusLabel(status?: MADeepAnalysis['status']): string {
  if (status === 'queued') return 'In coda';
  if (status === 'running') return 'In corso';
  if (status === 'ready') return 'Pronta';
  if (status === 'failed') return 'Fallita';
  return 'Assente';
}

function deepStatusClassName(status?: MADeepAnalysis['status']): string {
  if (status === 'queued' || status === 'running') return `${styles.statusPill} ${styles.statusRunning}`;
  if (status === 'ready') return `${styles.statusPill} ${styles.statusDone}`;
  if (status === 'failed') return `${styles.statusPill} ${styles.statusFailed}`;
  return styles.statusPill ?? '';
}

function statusPillClass(status: MASessionDetail['session']['status'], running: boolean, failed: boolean): string {
  if (running) return `${styles.statusPill} ${styles.statusRunning}`;
  if (failed || status === 'failed') return `${styles.statusPill} ${styles.statusFailed}`;
  if (status === 'completed' || status === 'estimated') return `${styles.statusPill} ${styles.statusDone}`;
  return styles.statusPill ?? '';
}

function bucketClassName(bucket?: string): string {
  if (bucket === 'azionabile') return `${styles.bucketChip} ${styles.bucketAction}`;
  if (bucket === 'da_verificare') return `${styles.bucketChip} ${styles.bucketReview}`;
  if (bucket === 'soppresso') return `${styles.bucketChip} ${styles.bucketSuppressed}`;
  return styles.bucketChip ?? '';
}
