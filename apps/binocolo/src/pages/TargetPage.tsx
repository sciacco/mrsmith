import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Skeleton } from '@mrsmith/ui';
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
  MAStrategySpec,
  MAStrategyType,
  MATarget,
  MATargetEvidence,
  MATargetFlag,
  MAThesis,
} from '../api/types';
import styles from './TargetPage.module.css';

const numberFormat = new Intl.NumberFormat('it-IT');
const moneyFormat = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  maximumFractionDigits: 0,
});
const costFormat = new Intl.NumberFormat('it-IT', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 4,
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
  const [sessions, setSessions] = useState<MASessionSummary[]>([]);
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
  const [activeTab, setActiveTab] = useState<'results' | 'config'>('results');

  useEffect(() => {
    if (detail) {
      if ((detail.targets?.length ?? 0) > 0) {
        setActiveTab('results');
      } else {
        setActiveTab('config');
      }
    }
  }, [detail?.session.id]);

  const startNewSearch = useCallback(() => {
    setDetail(null);
    setStrategy(emptyStrategy);
    setPrompt('');
    setChosenStrategy('');
    setSelectedTargetId(null);
    setError(null);
  }, []);

  const loadSessions = useCallback(async () => {
    setBusy((current) => current ?? 'sessions');
    setError(null);
    try {
      const data = await api.get<MASessionListResponse>('/binocolo/v1/ma/sessions');
      setSessions(data.items);
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setBusy((current) => (current === 'sessions' ? null : current));
    }
  }, [api]);

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
  const estimateMatchesStrategy =
    hasEstimate && detail?.strategy?.strategy && selectedEstimateType
      ? strategyKey(strategy) === strategyKey(detail.strategy.strategy) &&
        estimatesMatchSearchLimit(detail.estimates, selectedEstimateType, normalizeSearchLimit(strategy.searchLimit))
      : false;
  const strategyModels = useMemo(() => filterStrategyModels(llmOptions.models), [llmOptions.models]);
  const strategyPrompts = llmOptions.prompts.filter((item) => item.scope === 'ma_strategy' || item.scope === 'default');
  const canEstimate = hasStrategy && busy !== 'create' && busy !== 'estimate';
  const canExecute = hasStrategy && hasEstimate && estimateMatchesStrategy && !selectedEstimateGroup?.blocked && busy !== 'execute';

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
      setDetail(data);
      setPrompt('');
      await loadSessions();
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

      <div className={styles.workspace}>
        <aside className={styles.sessionsPanel} aria-label="Ricerche salvate">
          <div className={styles.panelHeader}>
            <div>
              <h2>Ricerche salvate</h2>
              <p>{sessions.length > 0 ? `${sessions.length} sessioni` : 'Nessuna ricerca salvata'}</p>
            </div>
            <Button
              variant="secondary"
              size="sm"
              onClick={startNewSearch}
              disabled={detail === null}
              leftIcon={<Icon name="plus" size={14} />}
            >
              Nuova
            </Button>
          </div>
          {busy === 'sessions' && sessions.length === 0 ? (
            <div className={styles.skeletonBlock}>
              <Skeleton rows={6} />
            </div>
          ) : sessions.length === 0 ? (
            <EmptyState icon="file-text" title="Nessuna sessione" text="Prepara una nuova strategia dalla richiesta iniziale." />
          ) : (
            <div className={styles.sessionList}>
              {sessions.map((item) => (
                <button
                  type="button"
                  key={item.id}
                  className={`${styles.sessionItem} ${detail?.session.id === item.id ? styles.sessionItemActive : ''}`}
                  onClick={() => void openSession(item.id)}
                >
                  <span className={styles.sessionTitle}>{item.title}</span>
                  <span className={styles.sessionMeta}>
                    {sessionStatusLabel(item.status)}
                    {item.resultCount > 0 ? ` · ${numberFormat.format(item.resultCount)} target` : ''}
                  </span>
                  <span className={styles.sessionPrompt}>{item.prompt}</span>
                </button>
              ))}
            </div>
          )}
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
                    Prepara strategia
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
                            {estimateLabel(estimate)} · {formatEstimateCount(estimate.estimatedCount, estimate.surfaceStatus === 'too_broad')}
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
                          Questa strategia satura due finestre di dry-run da 1.000 risultati: restringi i criteri prima di confermare.
                        </div>
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
        <span>Tesi d'acquisizione</span>
        <select value={strategy.thesis ?? 'generico'} onChange={(event) => onChange({ thesis: event.target.value as MAThesis })}>
          <option value="generico">Generico</option>
          <option value="successione">Successione</option>
          <option value="crescita">Crescita</option>
          <option value="consolidamento">Consolidamento</option>
          <option value="tuck_in">Tuck-in</option>
        </select>
        <small className={styles.fieldHint}>{thesisDescriptions[strategy.thesis ?? 'generico']}</small>
      </label>
      <label>
        <span>Forme giuridiche</span>
        <input
          value={(strategy.legalForms ?? []).join(', ')}
          onChange={(event) => onChange({ legalForms: splitList(event.target.value).map((item) => item.toUpperCase()) })}
          placeholder="solo se richiesto (es. SR)"
        />
      </label>
      <label className={styles.fieldWide}>
        <span>Codici ATECO</span>
        <textarea value={atecoText} onChange={(event) => onChange({ atecoCandidates: parseAtecoLines(event.target.value) })} rows={3} />
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
    group.lowerBound ||= estimate.surfaceStatus === 'too_broad';
    group.probeCount += estimate.probeCount ?? 1;
  }
  return groups.filter((group) => group.rows.length > 0);
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
  const costText = cost > 0 ? `${costFormat.format(cost)} costo stimato` : 'costo non indicato';
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
