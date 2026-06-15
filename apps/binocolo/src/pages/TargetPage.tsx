import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Skeleton } from '@mrsmith/ui';
import { useCallback, useEffect, useMemo, useState, type ChangeEvent, type FormEvent } from 'react';
import { useApiClient } from '../api/client';
import type {
  MAAtecoCandidate,
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

const emptyPrompt =
  'Es. target software B2B in Lombardia, fatturato intorno a 5 milioni, soci 50/50 prossimi al passaggio generazionale.';

const emptyStrategy: MAStrategySpec = {
  sectorDescription: '',
  territoryLabel: '',
  provinces: [],
  activityStatus: 'ATTIVA',
  atecoCandidates: [],
  keywords: [],
  shareholder: { requiresEqualSplit: false, tolerance: 2 },
  shareholderAge: { required: false },
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
        setSelectedModelId(defaultOptionID(data.models));
        setSelectedPromptId(defaultOptionID(data.prompts));
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
  const hasStrategy = Boolean(detail?.strategy);
  const hasEstimate = estimateGroups.length > 0;
  const hasTargets = (detail?.targets.length ?? 0) > 0;
  const strategyModels = llmOptions.models.filter((item) => item.scope === 'ma_strategy' || item.scope === 'default');
  const strategyPrompts = llmOptions.prompts.filter((item) => item.scope === 'ma_strategy' || item.scope === 'default');
  const canEstimate = hasStrategy && busy !== 'create' && busy !== 'estimate';
  const canExecute = hasStrategy && hasEstimate && busy !== 'execute';

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

        <section className={styles.mainColumn}>
          <section className={styles.requestPanel} aria-labelledby="request-title">
            <div className={styles.panelHeader}>
              <div>
                <h2 id="request-title">Nuova richiesta</h2>
                <p>Descrivi il target ideale e lascia che Binocolo prepari una strategia verificabile.</p>
              </div>
            </div>
            <form className={styles.requestForm} onSubmit={createSession}>
              <textarea
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder={emptyPrompt}
                rows={4}
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

          {hasStrategy ? (
            <section className={styles.strategyPanel} aria-labelledby="strategy-title">
              <div className={styles.panelHeader}>
                <div>
                  <h2 id="strategy-title">Strategia</h2>
                  <p>Rivedi criteri, territorio, fatturato e codici ATECO prima della stima.</p>
                </div>
                <StatusPill status={detail?.session.status} />
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
          ) : (
            <section className={styles.blankPanel}>
              <EmptyState icon="search" title="Strategia non ancora pronta" text="Inserisci una richiesta per generare criteri e stima." />
            </section>
          )}

          {hasEstimate ? (
            <section className={styles.estimatePanel} aria-labelledby="estimate-title">
              <div className={styles.panelHeader}>
                <div>
                  <h2 id="estimate-title">Stima</h2>
                  <p>Confronta strategia ATECO e ricerca espansa prima di confermare.</p>
                </div>
              </div>
              <div className={styles.estimateGrid}>
                {estimateGroups.map((group) => (
                  <button
                    type="button"
                    key={group.type}
                    className={`${styles.estimateChoice} ${selectedEstimateType === group.type ? styles.estimateChoiceActive : ''}`}
                    onClick={() => setChosenStrategy(group.type)}
                  >
                    <span>{group.label}</span>
                    <strong>{numberFormat.format(group.count)}</strong>
                    <small>{group.cost > 0 ? `${costFormat.format(group.cost)} costo stimato` : 'costo non indicato'}</small>
                    {group.selected ? <em>scelta proposta</em> : null}
                  </button>
                ))}
              </div>
              <div className={styles.estimateDetails}>
                {(detail?.estimates ?? []).map((estimate) => (
                  <span key={estimate.id}>
                    {estimateLabel(estimate)} · {numberFormat.format(estimate.estimatedCount)}
                  </span>
                ))}
              </div>
              <div className={styles.strategyActions}>
                <Button
                  onClick={executeSession}
                  loading={busy === 'execute'}
                  disabled={!canExecute || !selectedEstimateType}
                  leftIcon={<Icon name="check" />}
                >
                  Conferma e cerca target
                </Button>
              </div>
            </section>
          ) : null}

          <section className={styles.resultsPanel} aria-labelledby="results-title">
            <div className={styles.panelHeader}>
              <div>
                <h2 id="results-title">Shortlist</h2>
                <p>{hasTargets ? `${detail?.targets.length ?? 0} target ordinati per aderenza` : 'I target appariranno dopo la conferma.'}</p>
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
              <TargetTable rows={detail?.targets ?? []} selectedId={selectedTarget?.id} onSelect={setSelectedTargetId} />
            ) : detail?.session.status === 'completed' ? (
              <EmptyState icon="search" title="Nessun target emerso" text="Restringi o modifica i criteri e ripeti la stima." />
            ) : (
              <EmptyState icon="clipboard-check" title="In attesa di conferma" text="Completa la stima e avvia la ricerca dei target." />
            )}
          </section>
        </section>

        <aside className={styles.detailPanel} aria-label="Dettaglio target">
          <div className={styles.panelHeader}>
            <div>
              <h2>Dettaglio</h2>
              <p>{selectedTarget ? 'Evidenze e criteri mancanti' : 'Seleziona un target'}</p>
            </div>
          </div>
          {selectedTarget ? <TargetDetail target={selectedTarget} /> : <EmptyState icon="eye" title="Nessun target selezionato" text="Apri una riga della shortlist." />}
        </aside>
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
      <label className={styles.fieldWide}>
        <span>Codici ATECO</span>
        <textarea value={atecoText} onChange={(event) => onChange({ atecoCandidates: parseAtecoLines(event.target.value) })} rows={3} />
      </label>
      <div className={styles.toggleRow}>
        <label className={styles.checkField}>
          <input
            type="checkbox"
            checked={strategy.shareholder.requiresEqualSplit}
            onChange={(event) =>
              onChange({ shareholder: { ...strategy.shareholder, requiresEqualSplit: event.target.checked } })
            }
          />
          <span>Compagine 50/50</span>
        </label>
        <label className={styles.inlineNumber}>
          <span>Tolleranza</span>
          <input
            type="number"
            min={0}
            max={10}
            value={strategy.shareholder.tolerance ?? 2}
            onChange={(event) =>
              onChange({ shareholder: { ...strategy.shareholder, tolerance: optionalNumber(event.target.value) ?? 2 } })
            }
          />
        </label>
      </div>
      <div className={styles.toggleRow}>
        <label className={styles.checkField}>
          <input
            type="checkbox"
            checked={strategy.shareholderAge.required}
            onChange={(event) => onChange({ shareholderAge: { ...strategy.shareholderAge, required: event.target.checked } })}
          />
          <span>Eta soci da verificare</span>
        </label>
        <label className={styles.inlineNumber}>
          <span>Da</span>
          <input
            type="number"
            min={0}
            value={strategy.shareholderAge.min ?? ''}
            onChange={(event) => onChange({ shareholderAge: { ...strategy.shareholderAge, min: optionalNumber(event.target.value) } })}
          />
        </label>
        <label className={styles.inlineNumber}>
          <span>A</span>
          <input
            type="number"
            min={0}
            value={strategy.shareholderAge.max ?? ''}
            onChange={(event) => onChange({ shareholderAge: { ...strategy.shareholderAge, max: optionalNumber(event.target.value) } })}
          />
        </label>
      </div>
      <label className={styles.fieldWide}>
        <span>Razionale</span>
        <textarea value={strategy.rationale} onChange={(event) => onChange({ rationale: event.target.value })} rows={2} />
      </label>
    </div>
  );
}

function TargetTable({ rows, selectedId, onSelect }: { rows: MATarget[]; selectedId?: string; onSelect: (id: string) => void }) {
  return (
    <div className={styles.tableWrap}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>Target</th>
            <th>Provincia</th>
            <th>Settore</th>
            <th>Fatturato</th>
            <th>Punteggio</th>
            <th>Esito</th>
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
              <td>
                <button type="button" className={styles.targetButton} onClick={() => onSelect(target.id)}>
                  <span>{target.companyName}</span>
                  <small>{[target.vatCode, target.taxCode].filter(Boolean).join(' · ') || '-'}</small>
                </button>
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
                <span className={`${styles.matchBadge} ${matchClass(target.matchState)}`}>{matchLabel(target.matchState)}</span>
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
        <span className={`${styles.matchBadge} ${matchClass(target.matchState)}`}>{matchLabel(target.matchState)}</span>
      </div>
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
        {target.evidence.map((item) => (
          <EvidenceRow key={`${item.criterion}-${item.label}`} evidence={item} />
        ))}
      </div>
    </div>
  );
}

function EvidenceRow({ evidence }: { evidence: MATargetEvidence }) {
  return (
    <div className={styles.evidenceRow}>
      <span className={`${styles.evidenceDot} ${evidenceClass(evidence.status)}`} />
      <div>
        <strong>{evidence.label}</strong>
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
    { type: 'ateco', label: 'Strategia ATECO', count: 0, cost: 0, rows: [], selected: false },
    { type: 'expanded', label: 'Ricerca espansa', count: 0, cost: 0, rows: [], selected: false },
  ];
  for (const estimate of estimates) {
    const group = groups.find((item) => item.type === estimate.strategyType);
    if (!group) continue;
    group.count += estimate.estimatedCount;
    group.cost += estimate.estimatedCost;
    group.rows.push(estimate);
    group.selected ||= estimate.selected;
  }
  return groups.filter((group) => group.rows.length > 0);
}

function defaultOptionID<T extends { id: string; isDefault: boolean }>(options: T[]): string {
  return options.find((item) => item.isDefault)?.id ?? options[0]?.id ?? '';
}

function estimateLabel(estimate: MAEstimate): string {
  const parts = [strategyTypeLabel(estimate.strategyType)];
  if (estimate.atecoCode) parts.push(estimate.atecoCode);
  if (estimate.province) parts.push(estimate.province);
  return parts.join(' · ');
}

function strategyTypeLabel(type: MAStrategyType): string {
  return type === 'ateco' ? 'ATECO' : 'Ricerca espansa';
}

function sessionStatusLabel(status: MASessionStatus): string {
  switch (status) {
    case 'draft':
      return 'Strategia pronta';
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

function matchClass(value: string) {
  if (value === 'match') return styles.matchStrong;
  if (value === 'fuori_criterio') return styles.matchWeak;
  return styles.matchPartial;
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
