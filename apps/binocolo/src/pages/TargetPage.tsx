import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Modal, Skeleton, useToast } from '@mrsmith/ui';
import { useCallback, useEffect, useMemo, useState, type ChangeEvent, type FormEvent } from 'react';
import { useApiClient } from '../api/client';
import type {
  MAAtecoCandidate,
  MALLMModelOption,
  MALLMOptionsResponse,
  MAEstimate,
  MASessionDetail,
  MASessionListResponse,
  MASessionStatus,
  MASessionSummary,
  MASessionVisibility,
  MAStrategySpec,
  MAStrategyType,
  MATarget,
  MATargetEvidence,
  MATargetFlag,
  MAThesis,
} from '../api/types';
import styles from './TargetPage.module.css';

const numberFormat = new Intl.NumberFormat('it-IT');
const dateFormat = new Intl.DateTimeFormat('it-IT', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
});
const moneyFormat = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  maximumFractionDigits: 0,
});
const eurFormat = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});
const defaultSearchLimit = 100;
const maxSearchLimit = 1000;

const thesisDescriptions: Record<string, string> = {
  generico: 'Nessuna tesi: pesi bilanciati, età non valutata.',
  successione: 'Proprietario vicino all’uscita: premia proprietà concentrata, azienda matura, forma acquisibile.',
  crescita: 'Scale-up: premia trend di fatturato, produttività e aziende giovani.',
  consolidamento: 'Quota di mercato: premia aderenza al profilo e dimensione vicina all’ideale.',
  tuck_in: 'Competenza mirata: premia precisione di settore, accetta target piccoli.',
};

const emptyPrompt =
  'Es. target software B2B in Lombardia, fatturato intorno a 5 milioni, con condizioni specifiche da valutare sui risultati.';

const emptyStrategy: MAStrategySpec = {
  sectorDescription: '',
  territoryLabel: '',
  provinces: [],
  activityStatus: 'ATTIVA',
  searchLimit: defaultSearchLimit,
  atecoCandidates: [],
  keywords: [],
  scoringCriteria: [],
  rationale: '',
  missingCriteria: [],
};

type BusyState = 'sessions' | 'create' | 'estimate' | 'execute' | 'export' | null;

interface EstimateGroup {
  type: MAStrategyType;
  label: string;
  count: number;
  cost: number;
  rows: MAEstimate[];
  selected: boolean;
  blocked: boolean;
  lowerBound: boolean;
  probeCount: number;
}

export function TargetPage() {
  const api = useApiClient();
  const { toast } = useToast();
  const [sessions, setSessions] = useState<MASessionSummary[]>([]);
  const [sessionVisibility, setSessionVisibility] = useState<MASessionVisibility>('active');
  const [llmOptions, setLLMOptions] = useState<MALLMOptionsResponse>({ models: [], prompts: [] });
  const [detail, setDetail] = useState<MASessionDetail | null>(null);
  const [strategy, setStrategy] = useState<MAStrategySpec>(emptyStrategy);
  const [prompt, setPrompt] = useState('');
  const [selectedTargetId, setSelectedTargetId] = useState<string | null>(null);
  const [chosenStrategy, setChosenStrategy] = useState<MAStrategyType | ''>('');
  const [selectedModelId, setSelectedModelId] = useState('');
  const [selectedPromptId, setSelectedPromptId] = useState('');
  const [busy, setBusy] = useState<BusyState>(null);
  const [error, setError] = useState<string | null>(null);
  const [acknowledgeCost, setAcknowledgeCost] = useState(false);
  const [activeTab, setActiveTab] = useState<'results' | 'config'>('results');
  const [isFullDetailOpen, setIsFullDetailOpen] = useState(false);
  const [modalActiveTab, setModalActiveTab] = useState<'overview' | 'financials' | 'shareholders' | 'registry'>('overview');
  const [lifecycleBusyId, setLifecycleBusyId] = useState<string | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<MASessionSummary | null>(null);
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState<boolean>(() => {
    try {
      const saved = localStorage.getItem('mrsmith_binocolo_sidebar_collapsed');
      return saved === 'true';
    } catch {
      return false;
    }
  });

  const toggleSidebar = useCallback(() => {
    setIsSidebarCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem('mrsmith_binocolo_sidebar_collapsed', String(next));
      } catch {
        // Ignore
      }
      return next;
    });
  }, []);

  useEffect(() => {
    if (detail) {
      if ((detail.targets?.length ?? 0) > 0) {
        setActiveTab('results');
      } else {
        setActiveTab('config');
      }
    }
  }, [detail?.session.id]);

  const clearActiveDetail = useCallback(() => {
    setDetail(null);
    setStrategy(emptyStrategy);
    setChosenStrategy('');
    setSelectedTargetId(null);
    setAcknowledgeCost(false);
    setIsFullDetailOpen(false);
  }, []);

  const startNewSearch = useCallback(() => {
    setSessionVisibility('active');
    clearActiveDetail();
    setPrompt('');
    setError(null);
  }, [clearActiveDetail]);

  const loadSessionsFor = useCallback(async (visibility: MASessionVisibility) => {
    setBusy((current) => current ?? 'sessions');
    setError(null);
    try {
      const data = await api.get<MASessionListResponse>(`/binocolo/v1/ma/sessions?visibility=${visibility}`);
      setSessions(data.items);
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setBusy((current) => (current === 'sessions' ? null : current));
    }
  }, [api]);

  const loadSessions = useCallback(async () => {
    await loadSessionsFor(sessionVisibility);
  }, [loadSessionsFor, sessionVisibility]);

  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

  useEffect(() => {
    let active = true;
    api
      .get<MALLMOptionsResponse>('/binocolo/v1/ma/llm-options')
      .then((data) => {
        if (!active) return;
        setLLMOptions(data);
        const filteredModels = filterStrategyModels(data.models);
        setSelectedModelId(defaultOptionID(filteredModels, 'ma_strategy'));
        setSelectedPromptId(defaultOptionID(data.prompts, 'ma_strategy'));
      })
      .catch((err) => {
        if (active) setError(errorLabel(err));
      });
    return () => {
      active = false;
    };
  }, [api]);

  useEffect(() => {
    if (!detail?.strategy?.strategy) return;
    setStrategy(detail.strategy.strategy);
    setChosenStrategy(detail.session.selectedStrategy ?? detail.strategy.strategy.selectedStrategy ?? '');
    setSelectedTargetId(detail.targets[0]?.id ?? null);
  }, [detail]);

  const selectedTarget = useMemo(() => {
    if (!detail?.targets.length) return null;
    return detail.targets.find((target) => target.id === selectedTargetId) ?? detail.targets[0];
  }, [detail?.targets, selectedTargetId]);

  const estimateGroups = useMemo(() => groupEstimates(detail?.estimates ?? []), [detail?.estimates]);
  const selectedEstimateType = chosenStrategy || estimateGroups.find((group) => group.selected)?.type || '';
  const selectedEstimateGroup = estimateGroups.find((group) => group.type === selectedEstimateType);
  const hasStrategy = Boolean(detail?.strategy);
  const hasEstimate = estimateGroups.length > 0;
  const hasTargets = (detail?.targets.length ?? 0) > 0;
  const sessionIsArchived = Boolean(detail?.session.archivedAt);
  const sessionIsDeleted = Boolean(detail?.session.deletedAt);
  const canOperateOnSession = !sessionIsArchived && !sessionIsDeleted;
  const estimateMatchesStrategy =
    hasEstimate && detail?.strategy?.strategy && selectedEstimateType
      ? strategyKey(strategy) === strategyKey(detail.strategy.strategy) &&
        estimatesMatchSearchLimit(detail.estimates, selectedEstimateType, normalizeSearchLimit(strategy.searchLimit))
      : false;
  const strategyModels = useMemo(() => filterStrategyModels(llmOptions.models), [llmOptions.models]);
  const strategyPrompts = llmOptions.prompts.filter((item) => item.scope === 'ma_strategy' || item.scope === 'default');
  const canEstimate = hasStrategy && canOperateOnSession && busy !== 'create' && busy !== 'estimate';
  const costPerCompanyEur = detail?.costPerCompanyEur ?? 0.1;
  const budgetEur = detail?.budgetEur ?? 0;
  const projectedFetched = selectedEstimateGroup
    ? Math.min(selectedEstimateGroup.count, normalizeSearchLimit(strategy.searchLimit))
    : 0;
  const projectedSpendEur = projectedFetched * costPerCompanyEur;
  const overBudget = budgetEur > 0 && projectedSpendEur > budgetEur;
  const canExecute =
    hasStrategy &&
    hasEstimate &&
    estimateMatchesStrategy &&
    !selectedEstimateGroup?.blocked &&
    canOperateOnSession &&
    (!overBudget || acknowledgeCost) &&
    busy !== 'execute';

  useEffect(() => {
    setAcknowledgeCost(false);
  }, [selectedEstimateType, detail?.strategy?.id, strategy.searchLimit]);

  function switchSessionVisibility(visibility: MASessionVisibility) {
    setSessionVisibility(visibility);
    setError(null);
    if (detail && sessionVisibilityFor(detail.session) !== visibility) {
      clearActiveDetail();
    }
  }

  async function openSession(id: string) {
    setBusy('sessions');
    setError(null);
    try {
      const data = await api.get<MASessionDetail>(`/binocolo/v1/ma/sessions/${id}`);
      setDetail(data);
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setBusy(null);
    }
  }

  async function createSession(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalized = prompt.trim();
    if (!normalized) return;
    setBusy('create');
    setError(null);
    try {
      const data = await api.post<MASessionDetail>('/binocolo/v1/ma/sessions', {
        prompt: normalized,
        modelId: selectedModelId || undefined,
        promptId: selectedPromptId || undefined,
      });
      setSessionVisibility('active');
      setDetail(data);
      setPrompt('');
      await loadSessionsFor('active');
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setBusy(null);
    }
  }

  async function estimateSession() {
    if (!detail?.session.id) return;
    setBusy('estimate');
    setError(null);
    try {
      const data = await api.post<MASessionDetail>(`/binocolo/v1/ma/sessions/${detail.session.id}/estimate`, {
        strategy,
      });
      setDetail(data);
      await loadSessions();
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setBusy(null);
    }
  }

  async function executeSession() {
    if (!detail?.session.id || !selectedEstimateType) return;
    setBusy('execute');
    setError(null);
    try {
      const data = await api.post<MASessionDetail>(`/binocolo/v1/ma/sessions/${detail.session.id}/execute`, {
        strategyType: selectedEstimateType,
        limit: normalizeSearchLimit(strategy.searchLimit),
        acknowledgeCost,
      });
      setDetail(data);
      await loadSessions();
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setBusy(null);
    }
  }

  async function exportXLSX() {
    if (!detail?.session.id || !hasTargets) return;
    setBusy('export');
    setError(null);
    try {
      const blob = await api.postBlob(`/binocolo/v1/ma/sessions/${detail.session.id}/export`, { format: 'xlsx' });
      downloadBlob(blob, `target-ma-${safeFilename(detail.session.title)}.xlsx`);
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setBusy(null);
    }
  }

  function exportCSV() {
    if (!detail?.targets.length) return;
    const blob = new Blob([targetsToCSV(detail.targets)], { type: 'text/csv;charset=utf-8' });
    downloadBlob(blob, `target-ma-${safeFilename(detail.session.title)}.csv`);
  }

  function updateStrategy(patch: Partial<MAStrategySpec>) {
    setStrategy((current) => ({ ...current, ...patch }));
  }

  async function archiveSession(item: MASessionSummary) {
    setLifecycleBusyId(item.id);
    setError(null);
    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${item.id}/archive`);
      toast('Ricerca archiviata.', 'success');
      if (detail?.session.id === item.id) {
        clearActiveDetail();
      }
      setSessionVisibility('archived');
      await loadSessionsFor('archived');
    } catch (err) {
      setError(errorLabel(err));
      toast(errorLabel(err), 'error');
    } finally {
      setLifecycleBusyId(null);
    }
  }

  async function restoreSession(item: MASessionSummary) {
    setLifecycleBusyId(item.id);
    setError(null);
    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${item.id}/restore`);
      toast('Ricerca ripristinata.', 'success');
      setSessionVisibility('active');
      await loadSessionsFor('active');
    } catch (err) {
      setError(errorLabel(err));
      toast(errorLabel(err), 'error');
    } finally {
      setLifecycleBusyId(null);
    }
  }

  async function softDeleteSession() {
    if (!deleteCandidate) return;
    const candidate = deleteCandidate;
    setLifecycleBusyId(candidate.id);
    setError(null);
    try {
      await api.delete<void>(`/binocolo/v1/ma/sessions/${candidate.id}`);
      toast('Ricerca spostata nel cestino.', 'success');
      setDeleteCandidate(null);
      if (detail?.session.id === candidate.id) {
        clearActiveDetail();
      }
      setSessionVisibility('deleted');
      await loadSessionsFor('deleted');
    } catch (err) {
      setError(errorLabel(err));
      toast(errorLabel(err), 'error');
    } finally {
      setLifecycleBusyId(null);
    }
  }

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <span className={styles.eyebrow}>Ricerca target</span>
          <h1>Target M&amp;A</h1>
        </div>
        <div className={styles.headerActions}>
          <Button variant="secondary" onClick={loadSessions} loading={busy === 'sessions'} leftIcon={<Icon name="refresh-cw" />}>
            Aggiorna
          </Button>
        </div>
      </header>

      {error ? (
        <div className={styles.errorPanel} role="alert">
          <Icon name="triangle-alert" size={18} />
          <span>{error}</span>
        </div>
      ) : null}

      <div className={`${styles.workspace} ${isSidebarCollapsed ? styles.workspaceSidebarCollapsed : ''}`}>
        <aside
          className={`${styles.sessionsPanel} ${isSidebarCollapsed ? styles.sessionsPanelCollapsed : ''}`}
          aria-label="Ricerche salvate"
        >
          <button
            type="button"
            className={styles.sidebarToggle}
            onClick={toggleSidebar}
            title={isSidebarCollapsed ? "Mostra ricerche salvate" : "Nascondi ricerche salvate"}
            aria-label={isSidebarCollapsed ? "Mostra ricerche salvate" : "Nascondi ricerche salvate"}
          >
            <Icon name={isSidebarCollapsed ? "chevron-right" : "chevron-left"} size={14} />
          </button>
          <div className={styles.sessionsPanelContent}>
            <div className={styles.panelHeader}>
              <div>
                <h2>Ricerche salvate</h2>
                <p>{sessionPanelSummary(sessionVisibility, sessions.length)}</p>
              </div>
              <Button
                variant="secondary"
                size="sm"
                onClick={startNewSearch}
                disabled={detail === null && sessionVisibility === 'active'}
                leftIcon={<Icon name="plus" size={14} />}
              >
                Nuova
              </Button>
            </div>
            <div className={styles.visibilityTabs} role="tablist" aria-label="Visibilità ricerche salvate">
              {sessionVisibilityOptions.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  role="tab"
                  aria-selected={sessionVisibility === option.value}
                  className={`${styles.visibilityTab} ${sessionVisibility === option.value ? styles.visibilityTabActive : ''}`}
                  onClick={() => switchSessionVisibility(option.value)}
                >
                  {option.label}
                </button>
              ))}
            </div>
            {busy === 'sessions' && sessions.length === 0 ? (
              <div className={styles.skeletonBlock}>
                <Skeleton rows={6} />
              </div>
            ) : sessions.length === 0 ? (
              <EmptyState
                icon="file-text"
                title={sessionEmptyState(sessionVisibility).title}
                text={sessionEmptyState(sessionVisibility).text}
              />
            ) : (
              <div className={styles.sessionList}>
                {sessions.map((item) => (
                  <div
                    key={item.id}
                    className={`${styles.sessionItem} ${detail?.session.id === item.id ? styles.sessionItemActive : ''}`}
                  >
                    <button
                      type="button"
                      className={styles.sessionOpenButton}
                      onClick={() => void openSession(item.id)}
                      disabled={sessionVisibility === 'deleted'}
                      title={sessionVisibility === 'deleted' ? 'Ripristina la ricerca prima di aprirla' : undefined}
                    >
                      <span className={styles.sessionTitle}>{item.title}</span>
                      <span className={styles.sessionMeta}>
                        {sessionStatusLabel(item.status)}
                        {item.resultCount > 0 ? ` · ${numberFormat.format(item.resultCount)} target` : ''}
                        {sessionLifecycleMeta(item, sessionVisibility)}
                      </span>
                      <span className={styles.sessionPrompt}>{item.prompt}</span>
                    </button>
                    <SessionActions
                      item={item}
                      visibility={sessionVisibility}
                      busy={lifecycleBusyId === item.id}
                      onArchive={archiveSession}
                      onRestore={restoreSession}
                      onDelete={setDeleteCandidate}
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
        </aside>

        {detail === null ? (
          <div className={styles.newSearchWorkspace}>
            <section className={styles.requestPanel} aria-labelledby="request-title">
              <div className={styles.panelHeader}>
                <div>
                  <h2 id="request-title">Nuova ricerca Target M&amp;A</h2>
                  <p>Descrivi il target ideale in linguaggio naturale e lascia che Binocolo prepari una strategia.</p>
                </div>
              </div>
              <form className={styles.requestForm} onSubmit={createSession}>
                <textarea
                  value={prompt}
                  onChange={(event) => setPrompt(event.target.value)}
                  placeholder={emptyPrompt}
                  rows={6}
                />
                <div className={styles.formFooter}>
                  <span>{prompt.trim().length > 0 ? `${prompt.trim().length} caratteri` : 'Richiesta libera in linguaggio naturale'}</span>
                  <Button type="submit" loading={busy === 'create'} disabled={!prompt.trim()} leftIcon={<Icon name="sparkles" />}>
                    Genera perimetro
                  </Button>
                </div>
                <div className={styles.aiOptions}>
                  <label>
                    <span>Modello IA</span>
                    <select value={selectedModelId} onChange={(event) => setSelectedModelId(event.target.value)}>
                      {strategyModels.map((item) => (
                        <option key={item.id} value={item.id}>
                          {item.name}{item.isDefault ? ' (default)' : ''}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    <span>Istruzioni</span>
                    <select value={selectedPromptId} onChange={(event) => setSelectedPromptId(event.target.value)}>
                      {strategyPrompts.map((item) => (
                        <option key={item.id} value={item.id}>
                          {item.name}{item.isDefault ? ' (default)' : ''}
                        </option>
                      ))}
                    </select>
                  </label>
                </div>
              </form>
            </section>
          </div>
        ) : (
          <div className={styles.activeSessionWorkspace}>
            <header className={styles.sessionHeader}>
              <div className={styles.sessionInfo}>
                <div className={styles.sessionTitleBlock}>
                  <Icon name="database" size={20} className={styles.sessionIcon} />
                  <h2>{detail.session.title || 'Ricerca Senza Titolo'}</h2>
                  <StatusPill status={detail.session.status} />
                </div>
                <p className={styles.sessionPromptText}>
                  <strong>Prompt originale:</strong> &ldquo;{detail.session.prompt}&rdquo;
                </p>
                {sessionIsArchived ? (
                  <div className={styles.lifecycleNotice}>
                    <Icon name="archive" size={16} />
                    <span>Ricerca archiviata. Ripristinala dalla lista per modificare stime o avviare nuove esecuzioni.</span>
                  </div>
                ) : null}
              </div>

              <div className={styles.tabNav}>
                <button
                  type="button"
                  className={`${styles.tabLink} ${activeTab === 'results' ? styles.tabLinkActive : ''}`}
                  onClick={() => setActiveTab('results')}
                  disabled={!hasTargets}
                  title={!hasTargets ? 'Completa la stima e cerca i target per sbloccare i risultati' : undefined}
                >
                  <Icon name="list" size={16} />
                  <span>Risultati e Analisi ({detail.targets.length})</span>
                </button>
                <button
                  type="button"
                  className={`${styles.tabLink} ${activeTab === 'config' ? styles.tabLinkActive : ''}`}
                  onClick={() => setActiveTab('config')}
                >
                  <Icon name="settings" size={16} />
                  <span>Configurazione e Stima</span>
                </button>
              </div>
            </header>

            {activeTab === 'results' ? (
              <div className={styles.resultsGrid}>
                <section className={styles.resultsPanel} aria-labelledby="results-title">
                  <div className={styles.panelHeader}>
                    <div>
                      <h2 id="results-title">Shortlist</h2>
                      <p>{hasTargets ? `${detail.targets.length} target ordinati per aderenza` : 'I target appariranno dopo la conferma.'}</p>
                    </div>
                    <div className={styles.exportActions}>
                      <Button variant="secondary" size="sm" onClick={exportCSV} disabled={!hasTargets}>
                        CSV
                      </Button>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={exportXLSX}
                        loading={busy === 'export'}
                        disabled={!hasTargets}
                        leftIcon={<Icon name="download" size={14} />}
                      >
                        XLSX
                      </Button>
                    </div>
                  </div>
                  {busy === 'execute' ? (
                    <div className={styles.skeletonBlock}>
                      <Skeleton rows={8} />
                    </div>
                  ) : hasTargets ? (
                    <TargetShortlist rows={detail.targets} selectedId={selectedTarget?.id} onSelect={setSelectedTargetId} />
                  ) : (
                    <EmptyState icon="clipboard-check" title="In attesa di conferma" text="Completa la stima e avvia la ricerca dei target." />
                  )}
                </section>

                <aside className={styles.detailPanel} aria-label="Dettaglio target">
                  <div className={styles.panelHeader}>
                    <div>
                      <h2>Dettaglio target</h2>
                      <p>{selectedTarget ? 'Evidenze e criteri mancanti' : 'Seleziona un target'}</p>
                    </div>
                    {selectedTarget && (
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => setIsFullDetailOpen(true)}
                        leftIcon={<Icon name="external-link" size={14} />}
                      >
                        Espandi
                      </Button>
                    )}
                  </div>
                  {selectedTarget ? (
                    <TargetDetail target={selectedTarget} />
                  ) : (
                    <EmptyState icon="eye" title="Nessun target selezionato" text="Apri una riga della shortlist per visualizzare i dettagli." />
                  )}
                </aside>
              </div>
            ) : (
              <div className={styles.configGrid}>
                <section className={styles.strategyPanel} aria-labelledby="strategy-title">
                  <div className={styles.panelHeader}>
                    <div>
                      <h2 id="strategy-title">Strategia di Ricerca</h2>
                      <p>Affina criteri geografici, finanziari e settoriali per la stima.</p>
                    </div>
                  </div>
                  <StrategyEditor strategy={strategy} onChange={updateStrategy} />
                  <div className={styles.strategyActions}>
                    <Button
                      variant="secondary"
                      onClick={estimateSession}
                      loading={busy === 'estimate'}
                      disabled={!canEstimate}
                      leftIcon={<Icon name="search" />}
                    >
                      Stima ricerca
                    </Button>
                  </div>
                </section>

                <section className={styles.estimatePanel} aria-labelledby="estimate-title">
                  <div className={styles.panelHeader}>
                    <div>
                      <h2 id="estimate-title">Calcolo Stima &amp; Avvio</h2>
                      <p>Confronta le superfici di ricerca e conferma per estrarre i target.</p>
                    </div>
                  </div>
                  {hasEstimate ? (
                    <>
                      <div className={styles.estimateGrid}>
                        {estimateGroups.map((group) => (
                          <button
                            type="button"
                            key={group.type}
                            className={[
                              styles.estimateChoice,
                              selectedEstimateType === group.type ? styles.estimateChoiceActive : '',
                              group.blocked ? styles.estimateChoiceBlocked : '',
                            ].filter(Boolean).join(' ')}
                            onClick={() => setChosenStrategy(group.type)}
                            aria-describedby={group.blocked ? `${group.type}-surface-status` : undefined}
                          >
                            <span>{group.label}</span>
                            <strong>{formatEstimateCount(group.count, group.lowerBound)}</strong>
                            <small id={`${group.type}-surface-status`}>
                              {group.blocked ? 'superficie troppo ampia' : 'superficie stimata'}
                            </small>
                            <small>{estimateCostLabel(group.cost, group.probeCount)}</small>
                            {group.selected ? <em>scelta proposta</em> : null}
                          </button>
                        ))}
                      </div>
                      <div className={styles.estimateDetails}>
                        {(detail.estimates ?? []).map((estimate) => (
                          <span key={estimate.id} className={estimate.surfaceStatus === 'too_broad' ? styles.estimateDetailBlocked : undefined}>
                            {estimateLabel(estimate)} · {formatEstimateCount(estimate.estimatedCount, estimateUsesLowerBound(estimate))}
                            {estimate.surfaceStatus === 'too_broad' ? ' · troppo ampia' : ''}
                          </span>
                        ))}
                      </div>
                      {!estimateMatchesStrategy ? (
                        <div className={styles.estimateWarning}>
                          La strategia è cambiata dopo la stima. Ricalcola prima di confermare la ricerca.
                        </div>
                      ) : null}
                      {selectedEstimateGroup?.blocked ? (
                        <div className={styles.estimateWarning}>
                          Questa strategia supera 1.000 risultati: restringi i criteri prima di confermare.
                        </div>
                      ) : null}
                      {selectedEstimateGroup ? (
                        <div className={overBudget ? styles.costSummaryOver : styles.costSummary}>
                          <span>
                            Costo enrichment stimato: <strong>{eurFormat.format(projectedSpendEur)}</strong>
                            {selectedEstimateGroup.count > projectedFetched
                              ? ` · ${numberFormat.format(projectedFetched)} di ${numberFormat.format(selectedEstimateGroup.count)} aziende`
                              : ` · ${numberFormat.format(projectedFetched)} aziende`}
                          </span>
                          <span>Budget sessione: {eurFormat.format(budgetEur)}</span>
                        </div>
                      ) : null}
                      {overBudget ? (
                        <label className={styles.costAck}>
                          <input
                            type="checkbox"
                            checked={acknowledgeCost}
                            onChange={(event) => setAcknowledgeCost(event.target.checked)}
                          />
                          <span>
                            Il costo stimato supera il budget di sessione. Restringi i criteri, oppure
                            conferma di voler procedere a {eurFormat.format(projectedSpendEur)}.
                          </span>
                        </label>
                      ) : null}
                      <div className={styles.strategyActions}>
                        <Button
                          onClick={executeSession}
                          loading={busy === 'execute'}
                          disabled={!canExecute || !selectedEstimateType}
                          leftIcon={<Icon name="check" />}
                          style={{ width: '100%' }}
                        >
                          Conferma e cerca target
                        </Button>
                      </div>
                    </>
                  ) : (
                    <div className={styles.estimateEmptyContainer}>
                      <EmptyState
                        icon="search"
                        title="Nessuna stima disponibile"
                        text="Clicca su 'Stima ricerca' per calcolare la dimensione del target e i costi stimati."
                      />
                    </div>
                  )}
                </section>
              </div>
            )}
          </div>
        )}
      </div>

      <Modal
        open={deleteCandidate !== null}
        onClose={() => setDeleteCandidate(null)}
        title="Sposta nel cestino"
        size="sm"
        dismissible={lifecycleBusyId === null}
      >
        <div className={styles.confirmBody}>
          <p>
            La ricerca &ldquo;{deleteCandidate?.title ?? ''}&rdquo; sarà nascosta dalle ricerche attive. Potrai
            ripristinarla dalla vista Cestino.
          </p>
          <div className={styles.confirmActions}>
            <Button variant="secondary" onClick={() => setDeleteCandidate(null)} disabled={lifecycleBusyId !== null}>
              Annulla
            </Button>
            <Button
              variant="danger"
              onClick={() => void softDeleteSession()}
              loading={lifecycleBusyId === deleteCandidate?.id}
              leftIcon={<Icon name="trash" size={16} />}
            >
              Sposta nel cestino
            </Button>
          </div>
        </div>
      </Modal>

      {isFullDetailOpen && selectedTarget && (
        <Modal
          open={isFullDetailOpen}
          onClose={() => setIsFullDetailOpen(false)}
          title={`Dettaglio Completo: ${selectedTarget.companyName}`}
          size="fluid"
        >
          <div className={styles.modalGrid}>
            <div className={styles.modalMain}>
              <div className={styles.tabNav}>
                <button
                  type="button"
                  className={`${styles.tabLink} ${modalActiveTab === 'overview' ? styles.tabLinkActive : ''}`}
                  onClick={() => setModalActiveTab('overview')}
                >
                  <Icon name="external-link" size={16} />
                  <span>Strategia &amp; Match</span>
                </button>
                <button
                  type="button"
                  className={`${styles.tabLink} ${modalActiveTab === 'financials' ? styles.tabLinkActive : ''}`}
                  onClick={() => setModalActiveTab('financials')}
                >
                  <Icon name="circle-dollar-sign" size={16} />
                  <span>Storico Finanziario</span>
                </button>
                <button
                  type="button"
                  className={`${styles.tabLink} ${modalActiveTab === 'shareholders' ? styles.tabLinkActive : ''}`}
                  onClick={() => setModalActiveTab('shareholders')}
                >
                  <Icon name="user" size={16} />
                  <span>Soci &amp; Cap Table</span>
                </button>
                <button
                  type="button"
                  className={`${styles.tabLink} ${modalActiveTab === 'registry' ? styles.tabLinkActive : ''}`}
                  onClick={() => setModalActiveTab('registry')}
                >
                  <Icon name="file-text" size={16} />
                  <span>Anagrafica Legale</span>
                </button>
              </div>

              {/* Tab 1: Overview */}
              {modalActiveTab === 'overview' && (
                <div className={styles.evidenceList}>
                  <div className={styles.modalRationaleBlock}>
                    <h4>Razionale aderenza Strategia</h4>
                    <p>{selectedTarget.rationale || 'Nessun razionale disponibile.'}</p>
                  </div>

                  {selectedTarget.missingCriteria.length > 0 ? (
                    <div className={styles.missingBox}>
                      <span>Criteri mancanti o parziali</span>
                      <p>{selectedTarget.missingCriteria.join(', ')}</p>
                    </div>
                  ) : null}

                  <div className={styles.modalEvidenceList}>
                    {evidenceFamilies.map((family) => {
                      const items = selectedTarget.evidence.filter((item) => item.family === family.key);
                      if (items.length === 0) return null;
                      return (
                        <div key={family.key} className={styles.modalEvidenceGroup}>
                          <h5>{family.label}</h5>
                          {items.map((item) => (
                            <EvidenceRow key={`${item.criterion}-${item.label}`} evidence={item} />
                          ))}
                        </div>
                      );
                    })}
                    {selectedTarget.evidence.filter((item) => !item.family).length > 0 && (
                      <div className={styles.modalEvidenceGroup}>
                        <h5>Altre evidenze</h5>
                        {selectedTarget.evidence
                          .filter((item) => !item.family)
                          .map((item) => (
                            <EvidenceRow key={`${item.criterion}-${item.label}`} evidence={item} />
                          ))}
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Tab 2: Financials */}
              {modalActiveTab === 'financials' && (() => {
                const rawPayload = selectedTarget.vendorPayload;
                const sheets = (() => {
                  const allSheets = rawPayload?.balanceSheets?.all;
                  if (Array.isArray(allSheets) && allSheets.length > 0) {
                    return [...allSheets].sort((a, b) => (a.year ?? 0) - (b.year ?? 0));
                  }
                  if (selectedTarget.turnoverYear && (selectedTarget.turnover != null || selectedTarget.employees != null)) {
                    return [{
                      year: selectedTarget.turnoverYear,
                      turnover: selectedTarget.turnover,
                      employees: selectedTarget.employees,
                      netWorth: null,
                      totalAssets: null,
                    }];
                  }
                  return [];
                })();

                const latestTurnoverSheet = [...sheets].reverse().find(s => s.turnover != null);
                const latestNetWorthSheet = [...sheets].reverse().find(s => s.netWorth != null);
                const latestTotalAssetsSheet = [...sheets].reverse().find(s => s.totalAssets != null);
                const latestEmployeesSheet = [...sheets].reverse().find(s => s.employees != null);

                const getTrend = (key: 'turnover' | 'netWorth' | 'employees' | 'totalAssets') => {
                  const validSheets = sheets.filter(s => s[key] != null);
                  if (validSheets.length < 2) return null;
                  const lastIndex = validSheets.length - 1;
                  const lastVal = validSheets[lastIndex][key];
                  const prevVal = validSheets[lastIndex - 1][key];
                  if (lastVal != null && prevVal != null && prevVal > 0) {
                    return ((lastVal - prevVal) / prevVal) * 100;
                  }
                  return null;
                };

                const formatTrend = (pct: number | null) => {
                  if (pct === null) return null;
                  const sign = pct >= 0 ? '+' : '';
                  const className = pct >= 0 ? styles.finTrendPositive : styles.finTrendNegative;
                  return <small className={className}>{sign}{pct.toFixed(1)}% YoY</small>;
                };

                const turnoverSheets = sheets.filter(s => s.turnover != null);
                const maxTurnover = Math.max(...turnoverSheets.map(s => s.turnover ?? 0), 1);

                const renderBarChart = () => {
                  if (turnoverSheets.length === 0) return null;

                  const width = 600;
                  const height = 200;
                  const paddingLeft = 65;
                  const paddingRight = 20;
                  const paddingTop = 25;
                  const paddingBottom = 35;

                  const chartWidth = width - paddingLeft - paddingRight;
                  const chartHeight = height - paddingTop - paddingBottom;

                  const barSpacing = chartWidth / turnoverSheets.length;
                  const barWidth = Math.min(barSpacing * 0.5, 45);

                  return (
                    <div className={styles.chartContainer}>
                      <div className={styles.chartTitle}>Andamento Fatturato (€)</div>
                      <svg viewBox={`0 0 ${width} ${height}`} className={styles.chartSvg}>
                        <defs>
                          <linearGradient id="barGradient" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="0%" stopColor="var(--color-accent)" />
                            <stop offset="100%" stopColor="#7c6cff" />
                          </linearGradient>
                        </defs>

                        {/* Grid lines (0%, 50%, 100%) */}
                        <line
                          x1={paddingLeft}
                          y1={paddingTop}
                          x2={width - paddingRight}
                          y2={paddingTop}
                          stroke="var(--color-border-subtle)"
                          strokeDasharray="4 4"
                        />
                        <text
                          x={paddingLeft - 10}
                          y={paddingTop + 4}
                          textAnchor="end"
                          style={{ fontSize: '10px', fill: 'var(--color-text-muted)', fontFamily: 'var(--font-mono)' }}
                        >
                          {moneyFormat.format(maxTurnover)}
                        </text>

                        <line
                          x1={paddingLeft}
                          y1={paddingTop + chartHeight / 2}
                          x2={width - paddingRight}
                          y2={paddingTop + chartHeight / 2}
                          stroke="var(--color-border-subtle)"
                          strokeDasharray="4 4"
                        />
                        <text
                          x={paddingLeft - 10}
                          y={paddingTop + chartHeight / 2 + 4}
                          textAnchor="end"
                          style={{ fontSize: '10px', fill: 'var(--color-text-muted)', fontFamily: 'var(--font-mono)' }}
                        >
                          {moneyFormat.format(maxTurnover / 2)}
                        </text>

                        <line
                          x1={paddingLeft}
                          y1={paddingTop + chartHeight}
                          x2={width - paddingRight}
                          y2={paddingTop + chartHeight}
                          stroke="var(--color-border)"
                        />
                        <text
                          x={paddingLeft - 10}
                          y={paddingTop + chartHeight + 4}
                          textAnchor="end"
                          style={{ fontSize: '10px', fill: 'var(--color-text-muted)', fontFamily: 'var(--font-mono)' }}
                        >
                          0 €
                        </text>

                        {/* Bars and Labels */}
                        {turnoverSheets.map((sheet, idx) => {
                          const val = sheet.turnover ?? 0;
                          const barHeight = (val / maxTurnover) * chartHeight;
                          const x = paddingLeft + idx * barSpacing + (barSpacing - barWidth) / 2;
                          const y = paddingTop + chartHeight - barHeight;

                          return (
                            <g key={sheet.year}>
                              <rect
                                x={x}
                                y={y}
                                width={barWidth}
                                height={Math.max(barHeight, 2)}
                                rx={4}
                                ry={4}
                                fill="url(#barGradient)"
                              />
                              <text
                                x={x + barWidth / 2}
                                y={y - 6}
                                textAnchor="middle"
                                style={{ fontSize: '11px', fill: 'var(--color-text)', fontWeight: 700, fontFamily: 'var(--font-mono)' }}
                              >
                                {val >= 1000000
                                  ? `${(val / 1000000).toFixed(2)}M`
                                  : `${Math.round(val / 1000).toLocaleString('it-IT')}k`
                                }
                              </text>
                              <text
                                x={x + barWidth / 2}
                                y={paddingTop + chartHeight + 18}
                                textAnchor="middle"
                                style={{ fontSize: '11px', fill: 'var(--color-text-secondary)', fontWeight: 600 }}
                              >
                                {sheet.year}
                              </text>
                            </g>
                          );
                        })}
                      </svg>
                    </div>
                  );
                };

                return (
                  <div>
                    <div className={styles.finCardsGrid}>
                      <div className={styles.finCard}>
                        <span>Fatturato {latestTurnoverSheet ? `(${latestTurnoverSheet.year})` : ''}</span>
                        <strong>{latestTurnoverSheet?.turnover != null ? moneyFormat.format(latestTurnoverSheet.turnover) : '-'}</strong>
                        {formatTrend(getTrend('turnover'))}
                      </div>
                      <div className={styles.finCard}>
                        <span>Patrimonio Netto {latestNetWorthSheet ? `(${latestNetWorthSheet.year})` : ''}</span>
                        <strong>{latestNetWorthSheet?.netWorth != null ? moneyFormat.format(latestNetWorthSheet.netWorth) : '-'}</strong>
                        {formatTrend(getTrend('netWorth'))}
                      </div>
                      <div className={styles.finCard}>
                        <span>Attivo Totale {latestTotalAssetsSheet ? `(${latestTotalAssetsSheet.year})` : ''}</span>
                        <strong>{latestTotalAssetsSheet?.totalAssets != null ? moneyFormat.format(latestTotalAssetsSheet.totalAssets) : '-'}</strong>
                        {formatTrend(getTrend('totalAssets'))}
                      </div>
                      <div className={styles.finCard}>
                        <span>Dipendenti {latestEmployeesSheet ? `(${latestEmployeesSheet.year})` : ''}</span>
                        <strong>{latestEmployeesSheet?.employees != null ? numberFormat.format(latestEmployeesSheet.employees) : '-'}</strong>
                        {formatTrend(getTrend('employees'))}
                      </div>
                    </div>

                    {renderBarChart()}

                    {sheets.length > 0 ? (
                      <div className={styles.tableContainer}>
                        <table className={styles.finTable}>
                          <thead>
                            <tr>
                              <th>Anno</th>
                              <th>Fatturato</th>
                              <th>Patrimonio Netto</th>
                              <th>Attivo Totale</th>
                              <th>Dipendenti</th>
                            </tr>
                          </thead>
                          <tbody>
                            {[...sheets].reverse().map((sheet) => (
                              <tr key={sheet.year}>
                                <td><strong>{sheet.year}</strong></td>
                                <td>{sheet.turnover != null ? moneyFormat.format(sheet.turnover) : '-'}</td>
                                <td>{sheet.netWorth != null ? moneyFormat.format(sheet.netWorth) : '-'}</td>
                                <td>{sheet.totalAssets != null ? moneyFormat.format(sheet.totalAssets) : '-'}</td>
                                <td>{sheet.employees != null ? numberFormat.format(sheet.employees) : '-'}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    ) : (
                      <EmptyState icon="file-text" title="Dati finanziari non disponibili" text="Nessun dato di bilancio storico presente per questo target." />
                    )}
                  </div>
                );
              })()}

              {/* Tab 3: Shareholders */}
              {modalActiveTab === 'shareholders' && (() => {
                const rawPayload = selectedTarget.vendorPayload;
                const shareholders = (() => {
                  if (!rawPayload) return [];
                  if (Array.isArray(rawPayload.shareHolders)) return rawPayload.shareHolders;
                  if (Array.isArray(rawPayload.shareholders)) {
                    const list: any[] = [];
                    for (const item of rawPayload.shareholders) {
                      const percent = item.percentShare ?? 0;
                      const info = item.shareholdersInformation;
                      if (Array.isArray(info) && info.length > 0) {
                        for (const sub of info) {
                          list.push({
                            name: sub.name,
                            surname: sub.surname,
                            companyName: sub.companyName,
                            percentShare: sub.percentShare ?? percent,
                            taxCode: sub.taxCode,
                          });
                        }
                      } else {
                        list.push({
                          name: item.name,
                          surname: item.surname,
                          companyName: item.companyName,
                          percentShare: percent,
                          taxCode: item.taxCode,
                        });
                      }
                    }
                    return list;
                  }
                  return [];
                })();

                return (
                  <div>
                    {shareholders.length > 0 ? (
                      <div className={styles.shGrid}>
                        {shareholders.map((sh: any, index: number) => {
                          const displayName = [sh.name, sh.surname].filter(Boolean).join(' ') || sh.companyName || 'Socio Sconosciuto';
                          const percent = sh.percentShare ?? 0;
                          return (
                            <div key={index} className={styles.shCard}>
                              <div className={styles.shName}>{displayName}</div>
                              {sh.taxCode && <div className={styles.shTaxCode}>{sh.taxCode}</div>}
                              <div className={styles.shShare}>
                                <span>Quota societaria:</span>
                                <strong>{percent > 0 ? `${percent}%` : 'n.d.'}</strong>
                              </div>
                              {percent > 0 && (
                                <div className={styles.shProgressBarBg}>
                                  <div className={styles.shProgressBar} style={{ width: `${percent}%` }} />
                                </div>
                              )}
                            </div>
                          );
                        })}
                      </div>
                    ) : (
                      <EmptyState icon="file-text" title="Cap Table non disponibile" text="Nessun dato relativo ai soci presente per questo target." />
                    )}
                  </div>
                );
              })()}

              {/* Tab 4: Registry */}
              {modalActiveTab === 'registry' && (() => {
                const rawPayload = selectedTarget.vendorPayload;
                const detailedForm = rawPayload?.detailedLegalForm?.description || '-';
                const startDate = rawPayload?.startDate || '-';
                const regDate = rawPayload?.registrationDate || '-';
                const pec = rawPayload?.pec || rawPayload?.PEC || '-';

                return (
                  <dl className={styles.registryList}>
                    <div>
                      <dt>Ragione Sociale</dt>
                      <dd>{selectedTarget.companyName}</dd>
                    </div>
                    <div>
                      <dt>Forma Giuridica</dt>
                      <dd>{detailedForm}</dd>
                    </div>
                    <div>
                      <dt>Partita IVA</dt>
                      <dd>{selectedTarget.vatCode || '-'}</dd>
                    </div>
                    <div>
                      <dt>Codice Fiscale</dt>
                      <dd>{selectedTarget.taxCode || '-'}</dd>
                    </div>
                    <div>
                      <dt>Inizio Attività</dt>
                      <dd>{startDate}</dd>
                    </div>
                    <div>
                      <dt>Data Registrazione</dt>
                      <dd>{regDate}</dd>
                    </div>
                    <div>
                      <dt>Stato Attività</dt>
                      <dd>{selectedTarget.activityStatus || '-'}</dd>
                    </div>
                    <div>
                      <dt>PEC</dt>
                      <dd>{pec}</dd>
                    </div>
                    <div className={styles.fieldWide}>
                      <dt>Sede Legale</dt>
                      <dd>{[selectedTarget.town, selectedTarget.province].filter(Boolean).join(' · ') || '-'}</dd>
                    </div>
                  </dl>
                );
              })()}
            </div>

            {/* Sidebar Column (Option 2 Integration) */}
            <div className={styles.modalSidebar}>
              <div className={styles.scoreCircleCard}>
                <span className={styles.scoreEyebrow}>Punteggio Match</span>
                <span className={styles.scoreLargeNumber}>{selectedTarget.score}</span>
                <span className={`${styles.statusPill} ${styles[`status_${selectedTarget.matchState === 'match' ? 'completed' : selectedTarget.matchState === 'match_parziale' ? 'running' : 'failed'}`]}`}>
                  {matchLabel(selectedTarget.matchState)}
                </span>
                <span className={`${styles.confBadge} ${confidenceClass(selectedTarget.confidence)}`} style={{ marginTop: 'var(--space-2)' }}>
                  {selectedTarget.confidence ? `confidenza ${selectedTarget.confidence}` : 'confidenza n.d.'}
                </span>
              </div>

              <div className={styles.sidebarFactCard}>
                <dl className={styles.sidebarFactList}>
                  <div>
                    <dt>Fatturato Recente</dt>
                    <dd>{selectedTarget.turnover != null ? moneyFormat.format(selectedTarget.turnover) : '-'}</dd>
                  </div>
                  <div>
                    <dt>Dipendenti</dt>
                    <dd>{selectedTarget.employees != null ? numberFormat.format(selectedTarget.employees) : '-'}</dd>
                  </div>
                  <div>
                    <dt>Codice ATECO</dt>
                    <dd>{selectedTarget.atecoCode || '-'}</dd>
                  </div>
                  <div>
                    <dt>Descrizione Settore</dt>
                    <dd style={{ fontSize: '0.8rem', lineHeight: '1.3', fontWeight: 'normal' }}>
                      {selectedTarget.atecoDescription || '-'}
                    </dd>
                  </div>
                </dl>
              </div>

              {selectedTarget.flags && selectedTarget.flags.length > 0 && (
                <div className={styles.sidebarFactCard}>
                  <div style={{ marginBottom: 'var(--space-2)', fontSize: '0.6875rem', fontWeight: 700, textTransform: 'uppercase', color: 'var(--color-text-muted)', letterSpacing: '0.06em' }}>
                    Alert di Rischio
                  </div>
                  <FlagChips flags={selectedTarget.flags} />
                </div>
              )}

              <Button variant="secondary" onClick={() => setIsFullDetailOpen(false)} style={{ width: '100%', marginTop: 'var(--space-3)' }}>
                Chiudi dettagli
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </main>
  );
}

function StrategyEditor({ strategy, onChange }: { strategy: MAStrategySpec; onChange: (patch: Partial<MAStrategySpec>) => void }) {
  const atecoText = strategy.atecoCandidates.map((item) => [item.code, item.description].filter(Boolean).join(' - ')).join('\n');

  function updateNumber(field: keyof Pick<MAStrategySpec, 'turnoverMin' | 'turnoverMax' | 'employeeMin' | 'employeeMax'>) {
    return (event: ChangeEvent<HTMLInputElement>) => {
      onChange({ [field]: optionalNumber(event.target.value) } as Partial<MAStrategySpec>);
    };
  }

  return (
    <div className={styles.strategyGrid}>
      <label className={styles.fieldWide}>
        <span>Settore</span>
        <textarea
          value={strategy.sectorDescription}
          onChange={(event) => onChange({ sectorDescription: event.target.value })}
          rows={2}
        />
      </label>
      <label className={styles.fieldWide}>
        <span>Codici ATECO</span>
        <textarea value={atecoText} onChange={(event) => onChange({ atecoCandidates: parseAtecoLines(event.target.value) })} rows={3} />
      </label>
      <label>
        <span>Province</span>
        <input
          value={strategy.provinces.join(', ')}
          onChange={(event) => onChange({ provinces: splitList(event.target.value).map((item) => item.toUpperCase()) })}
          placeholder="MI, BG"
        />
      </label>
      <label>
        <span>Parole chiave</span>
        <input
          value={strategy.keywords.join(', ')}
          onChange={(event) => onChange({ keywords: splitList(event.target.value) })}
          placeholder="software, gestionale"
        />
      </label>
      <label>
        <span>Fatturato min</span>
        <input type="number" min={0} value={strategy.turnoverMin ?? ''} onChange={updateNumber('turnoverMin')} />
      </label>
      <label>
        <span>Fatturato max</span>
        <input type="number" min={0} value={strategy.turnoverMax ?? ''} onChange={updateNumber('turnoverMax')} />
      </label>
      <label className={styles.fieldWide}>
        <span>Tesi d'acquisizione</span>
        <div className={styles.thesisContainer}>
          <select value={strategy.thesis ?? 'generico'} onChange={(event) => onChange({ thesis: event.target.value as MAThesis })}>
            <option value="generico">Generico</option>
            <option value="successione">Successione</option>
            <option value="crescita">Crescita</option>
            <option value="consolidamento">Consolidamento</option>
            <option value="tuck_in">Tuck-in</option>
          </select>
          <small className={styles.fieldHint}>{thesisDescriptions[strategy.thesis ?? 'generico']}</small>
        </div>
      </label>
      <label>
        <span>Forme giuridiche</span>
        <input
          value={(strategy.legalForms ?? []).join(', ')}
          onChange={(event) => onChange({ legalForms: splitList(event.target.value).map((item) => item.toUpperCase()) })}
          placeholder="solo se richiesto (es. SR)"
        />
      </label>
      <label>
        <span>Numero risultati</span>
        <input
          type="number"
          min={1}
          max={maxSearchLimit}
          value={normalizeSearchLimit(strategy.searchLimit)}
          onChange={(event) => onChange({ searchLimit: normalizeSearchLimit(optionalNumber(event.target.value) ?? defaultSearchLimit) })}
        />
      </label>
      <label className={styles.fieldWide}>
        <span>Razionale</span>
        <textarea value={strategy.rationale} onChange={(event) => onChange({ rationale: event.target.value })} rows={2} />
      </label>
    </div>
  );
}

function TargetShortlist({ rows, selectedId, onSelect }: { rows: MATarget[]; selectedId?: string; onSelect: (id: string) => void }) {
  const [activeFlags, setActiveFlags] = useState<string[]>([]);

  const flagOptions: { code: string; label: string }[] = [];
  const seen = new Set<string>();
  for (const target of rows) {
    for (const flag of target.flags ?? []) {
      if (!seen.has(flag.code)) {
        seen.add(flag.code);
        flagOptions.push({ code: flag.code, label: flag.label });
      }
    }
  }

  const filtered =
    activeFlags.length === 0
      ? rows
      : rows.filter((target) => activeFlags.every((code) => (target.flags ?? []).some((flag) => flag.code === code)));

  const toggle = (code: string) =>
    setActiveFlags((prev) => (prev.includes(code) ? prev.filter((item) => item !== code) : [...prev, code]));

  return (
    <div className={styles.shortlist}>
      {flagOptions.length > 0 ? (
        <div className={styles.flagFilter}>
          {flagOptions.map((option) => (
            <button
              key={option.code}
              type="button"
              className={`${styles.flagFilterChip} ${activeFlags.includes(option.code) ? styles.flagFilterActive : ''}`}
              onClick={() => toggle(option.code)}
            >
              {option.label}
            </button>
          ))}
        </div>
      ) : null}
      <TargetTable rows={filtered} selectedId={selectedId} onSelect={onSelect} />
    </div>
  );
}

function TargetTable({ rows, selectedId, onSelect }: { rows: MATarget[]; selectedId?: string; onSelect: (id: string) => void }) {
  return (
    <div className={styles.tableWrap}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th className={styles.accentCol}></th>
            <th>Target</th>
            <th>Provincia</th>
            <th>Settore</th>
            <th>Fatturato</th>
            <th>Punteggio</th>
            <th>Confidenza</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((target, index) => (
            <tr
              key={target.id}
              className={target.id === selectedId ? styles.rowSelected : undefined}
              style={{ animationDelay: `${Math.min(index, 10) * 30}ms` }}
              onClick={() => onSelect(target.id)}
            >
              <td className={styles.accentCell}>
                <div className={styles.accentBar} />
              </td>
              <td>
                <button type="button" className={styles.targetButton} onClick={() => onSelect(target.id)}>
                  <span>{target.companyName}</span>
                  <small>{[target.vatCode, target.taxCode].filter(Boolean).join(' · ') || '-'}</small>
                </button>
                <FlagChips flags={target.flags} />
              </td>
              <td>{[target.town, target.province].filter(Boolean).join(' · ') || '-'}</td>
              <td>
                <span className={styles.ateco}>{target.atecoCode || '-'}</span>
                <small className={styles.tableSubtext}>{target.atecoDescription || 'Settore non indicato'}</small>
              </td>
              <td>{target.turnover != null ? moneyFormat.format(target.turnover) : '-'}</td>
              <td>
                <span className={styles.score}>{target.score}</span>
              </td>
              <td>
                <span className={`${styles.confBadge} ${confidenceClass(target.confidence)}`}>{target.confidence ?? '-'}</span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function TargetDetail({ target }: { target: MATarget }) {
  return (
    <div className={styles.targetDetail}>
      <div className={styles.detailTitleBlock}>
        <h3>{target.companyName}</h3>
        <span className={`${styles.confBadge} ${confidenceClass(target.confidence)}`}>
          {target.confidence ? `confidenza ${target.confidence}` : 'confidenza n.d.'}
        </span>
      </div>
      <FlagChips flags={target.flags} />
      <p className={styles.detailRationale}>{target.rationale || 'Motivazione non disponibile.'}</p>
      <dl className={styles.detailFacts}>
        <div>
          <dt>Fatturato</dt>
          <dd>{target.turnover != null ? moneyFormat.format(target.turnover) : '-'}</dd>
        </div>
        <div>
          <dt>ATECO</dt>
          <dd>{[target.atecoCode, target.atecoDescription].filter(Boolean).join(' · ') || '-'}</dd>
        </div>
        <div>
          <dt>Sede</dt>
          <dd>{[target.town, target.province].filter(Boolean).join(' · ') || '-'}</dd>
        </div>
      </dl>
      {target.missingCriteria.length > 0 ? (
        <div className={styles.missingBox}>
          <span>Criteri mancanti</span>
          <p>{target.missingCriteria.join(', ')}</p>
        </div>
      ) : null}
      <div className={styles.evidenceList}>
        {evidenceFamilies.map((family) => {
          const items = target.evidence.filter((item) => item.family === family.key);
          if (items.length === 0) return null;
          return (
            <div key={family.key} className={styles.evidenceGroup}>
              <span className={styles.evidenceGroupHead}>{family.label}</span>
              {items.map((item) => (
                <EvidenceRow key={`${item.criterion}-${item.label}`} evidence={item} />
              ))}
            </div>
          );
        })}
        {target.evidence
          .filter((item) => !item.family)
          .map((item) => (
            <EvidenceRow key={`${item.criterion}-${item.label}`} evidence={item} />
          ))}
      </div>
    </div>
  );
}

const evidenceFamilies: { key: string; label: string }[] = [
  { key: 'aderenza', label: 'Aderenza strategia' },
  { key: 'opportunita', label: 'Opportunità deal' },
  { key: 'economico', label: 'Profilo economico' },
];

function FlagChips({ flags }: { flags?: MATargetFlag[] }) {
  if (!flags || flags.length === 0) return null;
  return (
    <div className={styles.flagRow}>
      {flags.map((flag) => (
        <span
          key={flag.code}
          className={`${styles.flagChip} ${flag.severity === 'warning' ? styles.flagWarning : styles.flagNeutral}`}
        >
          {flag.label}
        </span>
      ))}
    </div>
  );
}

function confidenceClass(value?: string) {
  if (value === 'alta') return styles.confHigh;
  if (value === 'bassa') return styles.confLow;
  return styles.confMid;
}

function EvidenceRow({ evidence }: { evidence: MATargetEvidence }) {
  const weight = evidence.weight ?? 0;
  return (
    <div className={styles.evidenceRow}>
      <span className={`${styles.evidenceDot} ${evidenceClass(evidence.status)}`} />
      <div>
        <div className={styles.evidenceHead}>
          <strong>{evidence.label}</strong>
          {weight > 0 ? (
            <small className={styles.evidencePoints}>
              {Math.round(evidence.points ?? 0)} / {Math.round(weight)}
            </small>
          ) : null}
        </div>
        <p>{evidence.value || evidenceStatusLabel(evidence.status)}</p>
      </div>
    </div>
  );
}

function EmptyState({ icon, title, text }: { icon: 'search' | 'file-text' | 'clipboard-check' | 'eye'; title: string; text: string }) {
  return (
    <div className={styles.emptyState}>
      <div className={styles.emptyIcon}>
        <Icon name={icon} size={30} />
      </div>
      <h2>{title}</h2>
      <p>{text}</p>
    </div>
  );
}

const sessionVisibilityOptions: { value: MASessionVisibility; label: string }[] = [
  { value: 'active', label: 'Attive' },
  { value: 'archived', label: 'Archivio' },
  { value: 'deleted', label: 'Cestino' },
];

function SessionActions({
  item,
  visibility,
  busy,
  onArchive,
  onRestore,
  onDelete,
}: {
  item: MASessionSummary;
  visibility: MASessionVisibility;
  busy: boolean;
  onArchive: (item: MASessionSummary) => void;
  onRestore: (item: MASessionSummary) => void;
  onDelete: (item: MASessionSummary) => void;
}) {
  if (visibility === 'deleted') {
    return (
      <div className={styles.sessionActions} aria-label="Azioni ricerca">
        <button
          type="button"
          className={styles.sessionIconAction}
          onClick={() => onRestore(item)}
          disabled={busy}
          aria-label="Ripristina ricerca"
          title="Ripristina ricerca"
          aria-busy={busy || undefined}
        >
          <Icon name="refresh-cw" size={15} />
        </button>
      </div>
    );
  }

  return (
    <div className={styles.sessionActions} aria-label="Azioni ricerca">
      {visibility === 'active' ? (
        <button
          type="button"
          className={styles.sessionIconAction}
          onClick={() => onArchive(item)}
          disabled={busy}
          aria-label="Archivia ricerca"
          title="Archivia ricerca"
          aria-busy={busy || undefined}
        >
          <Icon name="archive" size={15} />
        </button>
      ) : (
        <button
          type="button"
          className={styles.sessionIconAction}
          onClick={() => onRestore(item)}
          disabled={busy}
          aria-label="Ripristina ricerca"
          title="Ripristina ricerca"
          aria-busy={busy || undefined}
        >
          <Icon name="refresh-cw" size={15} />
        </button>
      )}
      <button
        type="button"
        className={`${styles.sessionIconAction} ${styles.sessionIconDanger}`}
        onClick={() => onDelete(item)}
        disabled={busy}
        aria-label="Sposta nel cestino"
        title="Sposta nel cestino"
        aria-busy={busy || undefined}
      >
        <Icon name="trash" size={15} />
      </button>
    </div>
  );
}

function sessionPanelSummary(visibility: MASessionVisibility, count: number): string {
  if (count === 0) {
    if (visibility === 'archived') return 'Nessuna ricerca archiviata';
    if (visibility === 'deleted') return 'Cestino vuoto';
    return 'Nessuna ricerca salvata';
  }
  if (count === 1) {
    if (visibility === 'archived') return '1 ricerca archiviata';
    if (visibility === 'deleted') return '1 ricerca nel cestino';
    return '1 ricerca attiva';
  }
  if (visibility === 'archived') return `${count} ricerche archiviate`;
  if (visibility === 'deleted') return `${count} ricerche nel cestino`;
  return `${count} ricerche attive`;
}

function sessionEmptyState(visibility: MASessionVisibility): { title: string; text: string } {
  if (visibility === 'archived') {
    return { title: 'Archivio vuoto', text: 'Le ricerche archiviate appariranno qui e potranno essere ripristinate.' };
  }
  if (visibility === 'deleted') {
    return { title: 'Cestino vuoto', text: 'Le ricerche spostate nel cestino resteranno recuperabili da questa vista.' };
  }
  return { title: 'Nessuna sessione', text: 'Prepara una nuova strategia dalla richiesta iniziale.' };
}

function sessionLifecycleMeta(item: MASessionSummary, visibility: MASessionVisibility): string {
  if (visibility === 'archived' && item.archivedAt) {
    const label = dateLabel(item.archivedAt);
    return label ? ` · archiviata ${label}` : '';
  }
  if (visibility === 'deleted' && item.deletedAt) {
    const label = dateLabel(item.deletedAt);
    return label ? ` · cestinata ${label}` : '';
  }
  return '';
}

function sessionVisibilityFor(session: { archivedAt?: string; deletedAt?: string }): MASessionVisibility {
  if (session.deletedAt) return 'deleted';
  if (session.archivedAt) return 'archived';
  return 'active';
}

function dateLabel(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return dateFormat.format(date);
}

function StatusPill({ status }: { status?: MASessionStatus }) {
  if (!status) return null;
  return <span className={`${styles.statusPill} ${styles[`status_${status}`]}`}>{sessionStatusLabel(status)}</span>;
}

function groupEstimates(estimates: MAEstimate[]): EstimateGroup[] {
  const groups: EstimateGroup[] = [
    { type: 'ateco', label: 'Strategia ATECO', count: 0, cost: 0, rows: [], selected: false, blocked: false, lowerBound: false, probeCount: 0 },
    { type: 'expanded', label: 'Ricerca espansa', count: 0, cost: 0, rows: [], selected: false, blocked: false, lowerBound: false, probeCount: 0 },
  ];
  for (const estimate of estimates) {
    const group = groups.find((item) => item.type === estimate.strategyType);
    if (!group) continue;
    group.count += estimate.estimatedCount;
    group.cost += estimate.estimatedCost;
    group.rows.push(estimate);
    group.selected ||= estimate.selected;
    group.blocked ||= estimate.surfaceStatus === 'too_broad';
    group.lowerBound ||= estimateUsesLowerBound(estimate);
    group.probeCount += estimate.probeCount ?? 1;
  }
  return groups.filter((group) => group.rows.length > 0);
}

function estimateUsesLowerBound(estimate: MAEstimate): boolean {
  return estimate.surfaceStatus === 'too_broad' && (estimate.probeCount ?? 1) > 1;
}

function filterStrategyModels(models: MALLMModelOption[]): MALLMModelOption[] {
  return models.filter((item) => {
    if (item.scope === 'ma_strategy') {
      return true;
    }
    if (item.scope === 'default') {
      return !models.some((m) => m.scope === 'ma_strategy' && m.model === item.model);
    }
    return false;
  });
}

function defaultOptionID<T extends { id: string; isDefault: boolean; scope: string }>(options: T[], preferredScope: string): string {
  const preferredOptions = options.filter((item) => item.scope === preferredScope);
  const fallbackOptions = options.filter((item) => item.scope === 'default');
  return (
    preferredOptions.find((item) => item.isDefault)?.id ??
    preferredOptions[0]?.id ??
    fallbackOptions.find((item) => item.isDefault)?.id ??
    fallbackOptions[0]?.id ??
    ''
  );
}

function normalizeSearchLimit(value?: number): number {
  if (!Number.isFinite(value) || !value || value < 1) return defaultSearchLimit;
  return Math.min(maxSearchLimit, Math.trunc(value));
}

function strategyKey(strategy: MAStrategySpec): string {
  return JSON.stringify({
    title: strategy.title ?? '',
    sectorDescription: strategy.sectorDescription,
    territoryLabel: strategy.territoryLabel ?? '',
    provinces: strategy.provinces,
    activityStatus: strategy.activityStatus,
    turnoverAround: strategy.turnoverAround ?? null,
    turnoverMin: strategy.turnoverMin ?? null,
    turnoverMax: strategy.turnoverMax ?? null,
    employeeMin: strategy.employeeMin ?? null,
    employeeMax: strategy.employeeMax ?? null,
    searchLimit: normalizeSearchLimit(strategy.searchLimit),
    atecoCandidates: strategy.atecoCandidates,
    keywords: strategy.keywords,
    scoringCriteria: strategy.scoringCriteria ?? [],
    rationale: strategy.rationale,
    missingCriteria: strategy.missingCriteria,
    expandedClassification: strategy.expandedClassification ?? '',
  });
}

function estimatesMatchSearchLimit(estimates: MAEstimate[], strategyType: MAStrategyType, limit: number): boolean {
  const matching = estimates.filter((estimate) => estimate.strategyType === strategyType);
  if (matching.length === 0) return false;
  return matching.every((estimate) => {
    const value = Number(estimate.executionLimit ?? estimate.params?.limit ?? 0);
    return Number.isFinite(value) && value === limit;
  });
}

function estimateLabel(estimate: MAEstimate): string {
  const parts = [strategyTypeLabel(estimate.strategyType)];
  if (estimate.atecoCode) parts.push(estimate.atecoCode);
  if (estimate.province) parts.push(estimate.province);
  return parts.join(' · ');
}

function formatEstimateCount(count: number, lowerBound: boolean): string {
  const formatted = numberFormat.format(count);
  return lowerBound ? `>= ${formatted}` : formatted;
}

function estimateCostLabel(cost: number, probeCount: number): string {
  const costText = cost > 0 ? `${eurFormat.format(cost)} enrichment` : 'costo non indicato';
  if (probeCount > 1) return `${costText} · ${probeCount} dry-run`;
  return `${costText} · 1 dry-run`;
}

function strategyTypeLabel(type: MAStrategyType): string {
  return type === 'ateco' ? 'ATECO' : 'Ricerca espansa';
}

function sessionStatusLabel(status: MASessionStatus): string {
  switch (status) {
    case 'draft':
      return 'Bozza strategia';
    case 'estimated':
      return 'Stima pronta';
    case 'running':
      return 'Ricerca in corso';
    case 'completed':
      return 'Shortlist pronta';
    case 'failed':
      return 'Da rivedere';
    default:
      return status;
  }
}

function matchLabel(value: string): string {
  switch (value) {
    case 'match':
      return 'match';
    case 'match_parziale':
      return 'match parziale';
    case 'fuori_criterio':
      return 'fuori criterio';
    default:
      return value;
  }
}

function evidenceClass(value: string) {
  if (value === 'match') return styles.evidenceMatch;
  if (value === 'fuori_criterio') return styles.evidenceWeak;
  return styles.evidencePartial;
}

function evidenceStatusLabel(value: string): string {
  if (value === 'match') return 'criterio soddisfatto';
  if (value === 'fuori_criterio') return 'fuori criterio';
  if (value === 'criterio_mancante') return 'criterio mancante';
  return 'match parziale';
}

function optionalNumber(value: string): number | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function splitList(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseAtecoLines(value: string): MAAtecoCandidate[] {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [code, ...description] = line.split(/\s+-\s+/);
      return {
        code: (code ?? '').trim(),
        description: description.join(' - ').trim(),
        rationale: '',
      };
    });
}

function errorLabel(error: unknown): string {
  if (error instanceof ApiError) {
    const code = errorCode(error);
    if (code === 'binocolo_workspace_not_configured') {
      return "L'area di lavoro salvata non e disponibile in questo ambiente.";
    }
    if (code === 'openrouter_not_configured') {
      return 'La preparazione della strategia non e disponibile in questo ambiente.';
    }
    if (code === 'binocolo_llm_config_not_configured') {
      return 'La configurazione IA di Binocolo non e completa.';
    }
    if (code === 'openapiit_not_configured') {
      return 'La ricerca aziende non e disponibile in questo ambiente.';
    }
    if (code === 'openapiit_credit_required') {
      return 'Credito ricerca insufficiente per completare la stima.';
    }
    if (code === 'estimate_too_large') {
      return 'La stima e troppo ampia: restringi territorio, fatturato o settore prima di confermare.';
    }
    if (code === 'estimate_over_budget') {
      return 'Il costo stimato supera il budget di sessione: restringi i criteri o conferma il costo prima di procedere.';
    }
    if (code === 'invalid_ma_visibility') {
      return 'Vista ricerche non valida.';
    }
    if (code === 'ma_session_archived') {
      return 'La ricerca è archiviata: ripristinala prima di modificarla o rieseguirla.';
    }
    if (code === 'ma_session_deleted') {
      return 'La ricerca è nel cestino: ripristinala prima di aprirla.';
    }
    if (code === 'ma_session_not_found') {
      return 'Ricerca non trovata.';
    }
    if (code === 'invalid_ma_request') {
      return 'Controlla i criteri della strategia e riprova.';
    }
    if (error.status === 401) return 'Sessione non valida.';
    if (error.status === 403) return 'Non hai accesso a Binocolo.';
    return 'Operazione non riuscita.';
  }
  if (error instanceof Error) return error.message;
  return 'Operazione non riuscita.';
}

function errorCode(error: ApiError): string | undefined {
  const body = error.body;
  if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') {
    return body.error;
  }
  return undefined;
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

function targetsToCSV(targets: MATarget[]): string {
  const rows = [
    ['Azienda', 'Partita IVA', 'Codice fiscale', 'Provincia', 'Comune', 'Fatturato', 'ATECO', 'Punteggio', 'Esito', 'Criteri mancanti'],
    ...targets.map((target) => [
      target.companyName,
      target.vatCode ?? '',
      target.taxCode ?? '',
      target.province ?? '',
      target.town ?? '',
      target.turnover != null ? String(target.turnover) : '',
      target.atecoCode ?? '',
      String(target.score),
      matchLabel(target.matchState),
      target.missingCriteria.join(', '),
    ]),
  ];
  return rows.map((row) => row.map(csvCell).join(',')).join('\n');
}

function csvCell(value: string) {
  return `"${value.replace(/"/g, '""')}"`;
}

function safeFilename(value: string): string {
  const normalized = value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48);
  return normalized || 'ricerca';
}
