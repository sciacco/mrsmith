import { Button, Icon, Modal, Skeleton, useToast } from '@mrsmith/ui';
import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type {
  MAGatedProgressResponse,
  MASessionDetail,
  MATarget,
  MAVerificationQueueItem,
  MAVerificationQueueResponse,
  MAThesis,
} from '../../api/types';
import {
  bucketLabel,
  dateLabel,
  downloadBlob,
  errorLabel,
  gatedBucketCounts,
  isGateReject,
  normalizeSearchLimit,
  numberFormat,
  safeFilename,
  sessionStatusLabel,
  targetKey,
  targetsToCSV,
} from './helpers';
import styles from './Ricerche.module.css';

const thesisOptions: { value: MAThesis; label: string }[] = [
  { value: 'generico', label: 'Generico' },
  { value: 'successione', label: 'Successione' },
  { value: 'crescita', label: 'Crescita' },
  { value: 'consolidamento', label: 'Consolidamento' },
  { value: 'tuck_in', label: 'Competenze' },
];

type ActiveTab = 'results' | 'queue' | 'outside';

export function RicercaDetailPage() {
  const { id } = useParams<{ id: string }>();
  const api = useApiClient();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [detail, setDetail] = useState<MASessionDetail | null>(null);
  const [progress, setProgress] = useState<MAGatedProgressResponse | null>(null);
  const [queue, setQueue] = useState<MAVerificationQueueItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<ActiveTab>('results');
  const [selectedTarget, setSelectedTarget] = useState<MATarget | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [thesis, setThesis] = useState<MAThesis>('generico');
  const [associateFor, setAssociateFor] = useState<string | null>(null);
  const [associateDomain, setAssociateDomain] = useState('');
  const [noWebsiteItem, setNoWebsiteItem] = useState<MAVerificationQueueItem | null>(null);
  const [reprocessingKeys, setReprocessingKeys] = useState<Set<string>>(() => new Set());

  const loadAll = useCallback(async () => {
    if (!id) return;
    setError(null);
    try {
      const [detailData, progressData, queueData] = await Promise.all([
        api.get<MASessionDetail>(`/binocolo/v1/ma/sessions/${id}`),
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
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setLoading(false);
    }
  }, [api, id]);

  useEffect(() => {
    void loadAll();
  }, [loadAll]);

  useEffect(() => {
    if (!detail || !progress) return;
    const shouldPoll =
      detail.session.status === 'running' ||
      progress.stage === 'address' ||
      progress.stage === 'gate' ||
      progress.stage === 'enrich';
    if (!shouldPoll) return;
    const handle = setInterval(() => {
      void loadAll();
    }, 4000);
    return () => clearInterval(handle);
  }, [detail, loadAll, progress]);

  const targets = detail?.targets ?? [];
  const outsideTargets = useMemo(() => targets.filter(isGateReject), [targets]);
  const resultTargets = useMemo(
    () =>
      targets
        .filter((target) => !isGateReject(target))
        .sort((a, b) => {
          const ratingDelta = (b.rating ?? 0) - (a.rating ?? 0);
          if (ratingDelta !== 0) return ratingDelta;
          if (a.score !== b.score) return b.score - a.score;
          return a.companyName.localeCompare(b.companyName);
        }),
    [targets],
  );
  const buckets = gatedBucketCounts(progress);
  const isRunning = detail?.session.status === 'running' || progress?.stage === 'address' || progress?.stage === 'gate' || progress?.stage === 'enrich';
  const isFailed = detail?.session.status === 'failed' || progress?.stage === 'failed';

  async function resumeSearch() {
    if (!detail?.session.id) return;
    setBusy('resume');
    setError(null);
    try {
      await api.post<MASessionDetail>(`/binocolo/v1/ma/sessions/${detail.session.id}/gated-search`, {
        strategyType: 'expanded',
        limit: normalizeSearchLimit(detail.strategy?.strategy.searchLimit),
      });
      await loadAll();
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
      const data = await api.post<MASessionDetail>(`/binocolo/v1/ma/sessions/${detail.session.id}/rescore`, { thesis });
      setDetail(data);
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

  async function runDeepDive() {
    if (!detail?.session.id) return;
    setBusy('deep');
    setError(null);
    try {
      const data = await api.post<MASessionDetail>(`/binocolo/v1/ma/sessions/${detail.session.id}/deep-dive`, {
        acknowledgeCost: false,
      });
      setDetail(data);
      toast('Approfondimento avviato.', 'success');
    } catch (err) {
      setError(errorLabel(err));
      toast(errorLabel(err), 'error');
    } finally {
      setBusy(null);
    }
  }

  async function rateTarget(target: MATarget, rating: number) {
    if (!detail?.session.id) return;
    const nextRating = target.rating === rating ? 0 : rating;
    const sessionId = detail.session.id;
    setDetail((current) =>
      current
        ? {
            ...current,
            targets: current.targets.map((item) =>
              targetKey(item) === targetKey(target) ? { ...item, rating: nextRating === 0 ? undefined : nextRating } : item,
            ),
          }
        : current,
    );
    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${sessionId}/rating`, {
        companyKey: targetKey(target),
        rating: nextRating,
        ...(nextRating > 0 ? { scoreAtRating: target.score, confidenceAtRating: target.confidence } : {}),
      });
    } catch (err) {
      toast(errorLabel(err), 'error');
      await loadAll();
    }
  }

  async function submitAssociate(companyKey: string) {
    if (!detail?.session.id || !associateDomain.trim()) return;
    setBusy(`associate:${companyKey}`);
    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${detail.session.id}/associate-domain`, {
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
      await api.post<void>(`/binocolo/v1/ma/sessions/${detail.session.id}/confirm-group-site`, {
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
      await api.post<void>(`/binocolo/v1/ma/sessions/${detail.session.id}/no-website`, {
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
              running={isRunning}
              failed={isFailed}
              onResume={() => void resumeSearch()}
              resumeBusy={busy === 'resume'}
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
                  <span>Superficie <b>{numberFormat.format(progress?.surface.fetched || progress?.surface.expected || targets.length)}</b></span>
                  <span>→</span>
                  <span>oltre il gate <b>{numberFormat.format(progress?.enrich.survivors ?? buckets.keep + buckets.forse + buckets.review)}</b></span>
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
                  <Button variant="secondary" onClick={() => void rescore()} loading={busy === 'rescore'} leftIcon={<Icon name="refresh-cw" />}>
                    Cambia tesi e ricalcola
                  </Button>
                  <Button variant="secondary" onClick={() => void exportXLSX()} loading={busy === 'export'} leftIcon={<Icon name="download" />}>
                    Export XLSX
                  </Button>
                  <Button variant="secondary" onClick={exportCSV}>CSV</Button>
                  <Button onClick={() => void runDeepDive()} loading={busy === 'deep'} leftIcon={<Icon name="sparkles" />}>
                    Approfondimento
                  </Button>
                </div>
              </div>
              <div className={styles.tabs} role="tablist" aria-label="Viste risultati">
                <button type="button" role="tab" aria-selected={activeTab === 'results'} className={`${styles.tab} ${activeTab === 'results' ? styles.tabActive : ''}`} onClick={() => setActiveTab('results')}>
                  Risultati
                </button>
                <button type="button" role="tab" aria-selected={activeTab === 'queue'} className={`${styles.tab} ${activeTab === 'queue' ? styles.tabActive : ''}`} onClick={() => setActiveTab('queue')}>
                  Coda di verifica {queue.length}
                </button>
                <button type="button" role="tab" aria-selected={activeTab === 'outside'} className={`${styles.tab} ${activeTab === 'outside' ? styles.tabActive : ''}`} onClick={() => setActiveTab('outside')}>
                  Fuori tesi {outsideTargets.length}
                </button>
              </div>
              {activeTab === 'results' ? (
                <ResultsTable rows={resultTargets} onOpen={setSelectedTarget} onRate={(target, rating) => void rateTarget(target, rating)} />
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

      <TargetDetailModal target={selectedTarget} onClose={() => setSelectedTarget(null)} />

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
}: {
  progress: MAGatedProgressResponse;
  status: MASessionDetail['session']['status'];
  queueCount: number;
  running: boolean;
  failed: boolean;
  onResume: () => void;
  resumeBusy: boolean;
}) {
  const buckets = gatedBucketCounts(progress);
  const total = Math.max(1, buckets.total);
  return (
    <section className={styles.panel} aria-labelledby="progress-title">
      <div className={styles.panelHeader}>
        <div>
          <h2 id="progress-title">Avanzamento</h2>
          <p className={styles.hint}>La valutazione richiede tempo. La pagina si aggiorna da sola.</p>
        </div>
        <span className={statusPillClass(status, running, failed)}>
          {running ? <span className={styles.pulse} /> : null}
          {running ? 'Ricerca in corso' : failed ? 'Ricerca interrotta' : sessionStatusLabel(status)}
        </span>
      </div>
      <div className={styles.panelBody}>
        {failed ? (
          <div className={styles.stack}>
            <div className={styles.danger}>
              <Icon name="triangle-alert" size={18} />
              <span>
                <b>Ricerca interrotta</b> — errore del fornitore dati durante l'arricchimento. Le aziende già elaborate sono conservate: la ripresa riparte da dove si era fermata.
              </span>
            </div>
            <Button onClick={onResume} loading={resumeBusy} leftIcon={<Icon name="refresh-cw" />}>Riprendi la ricerca</Button>
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
              <BucketLegend className={styles.funnelKeep ?? ''} label="In tesi" count={buckets.keep} />
              <BucketLegend className={styles.funnelForse ?? ''} label="Da approfondire" count={buckets.forse} />
              <BucketLegend className={styles.funnelReview ?? ''} label="Da verificare" count={buckets.review || queueCount} />
              <BucketLegend className={styles.funnelReject ?? ''} label="Fuori tesi" count={buckets.reject} />
            </div>
          </>
        )}
      </div>
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

function BucketLegend({ className, label, count }: { className: string; label: string; count: number }) {
  return (
    <span className={styles.inlineActions}>
      <span className={`${styles.bucketDot} ${className}`} />
      {label} <b>{numberFormat.format(count)}</b>
    </span>
  );
}

function ResultsTable({ rows, onOpen, onRate }: { rows: MATarget[]; onOpen: (target: MATarget) => void; onRate: (target: MATarget, rating: number) => void }) {
  if (rows.length === 0) {
    return <div className={styles.emptyState}><span className={styles.emptyIcon}><Icon name="file-text" size={28} /></span><strong>Nessun risultato</strong></div>;
  }
  return (
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
          {rows.map((target) => (
            <tr key={target.id} className={styles.clickRow} onClick={() => onOpen(target)}>
              <td>
                <b>{target.companyName}</b>
                {target.webValidation?.webValidationState === 'no_website_declared' ? (
                  <span className={styles.badge}>nessuna evidenza web</span>
                ) : null}
              </td>
              <td>{target.province ?? '-'}</td>
              <td><span className={bucketClassName(target.bucket)}>{bucketLabel(target.bucket)}</span></td>
              <td><span className={styles.score}>{numberFormat.format(target.score)}</span></td>
              <td>
                <RatingStars
                  rating={target.rating ?? 0}
                  onRate={(rating) => onRate(target, rating)}
                />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function RatingStars({ rating, onRate }: { rating: number; onRate: (rating: number) => void }) {
  return (
    <span className={styles.stars} onClick={(event) => event.stopPropagation()}>
      {[1, 2, 3].map((value) => (
        <button
          key={value}
          type="button"
          className={`${styles.starButton} ${rating >= value ? styles.starOn : ''}`}
          onClick={() => onRate(value)}
          aria-label={`${value} stelle`}
        >
          ★
        </button>
      ))}
    </span>
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
  rows: MATarget[];
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

function TargetDetailModal({ target, onClose }: { target: MATarget | null; onClose: () => void }) {
  return (
    <Modal open={target !== null} onClose={onClose} title={target?.companyName ?? 'Dettaglio'} size="wide">
      {target ? (
        <div className={styles.detailModalBody}>
          <dl className={styles.facts}>
            <Fact label="Partita IVA" value={target.vatCode || '-'} />
            <Fact label="Provincia" value={target.province || '-'} />
            <Fact label="Comune" value={target.town || '-'} />
            <Fact label="ATECO" value={target.atecoCode || '-'} />
            <Fact label="Punteggio" value={numberFormat.format(target.score)} />
            <Fact label="Dominio" value={target.webValidation?.selectedDomain || '-'} />
          </dl>
          <div className={styles.infoNotice}>
            <Icon name="file-text" size={18} />
            <span>{target.rationale || target.webValidation?.finalDecision?.reason || 'Nessun razionale disponibile.'}</span>
          </div>
          {target.evidence.length > 0 ? (
            <div className={styles.stack}>
              <span className={styles.label}>Evidenze</span>
              {target.evidence.slice(0, 6).map((item) => (
                <div key={`${item.criterion}-${item.label}`} className={styles.notice}>
                  <span><b>{item.label}</b>{item.value ? ` · ${item.value}` : ''}</span>
                </div>
              ))}
            </div>
          ) : null}
          {target.vatCode ? (
            <a className={styles.linkButton} href={`/azienda?vat=${encodeURIComponent(target.vatCode)}`}>
              Apri dossier azienda
            </a>
          ) : null}
        </div>
      ) : null}
    </Modal>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className={styles.fact}>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
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
