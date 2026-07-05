import { Icon } from '@mrsmith/ui';
import type { MATarget } from '../../../../api/types';
import { HiddenField } from '../HiddenField';
import styles from '../Inspector.module.css';
import { formatDateTime, freshnessClass, freshnessLabel } from './format';

interface EvidenceRun {
  bucket: string;
  term: string;
  resultCount: number;
  bestScore?: number;
  matched: boolean;
  error?: string;
}

function runScoreClass(score?: number): string {
  if (score == null) return styles.runLow ?? '';
  if (score >= 0.7) return styles.runOk ?? '';
  if (score >= 0.4) return styles.runWarn ?? '';
  return styles.runLow ?? '';
}

function runScoreText(run: EvidenceRun): string {
  if (run.error) return 'errore';
  if (run.bestScore == null) return run.matched ? 'match' : '—';
  return run.bestScore.toLocaleString('it-IT', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function expiresShort(iso?: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return new Intl.DateTimeFormat('it-IT', { month: '2-digit', year: 'numeric' }).format(d);
}

export function WebValidationTab({ target }: { target: MATarget }) {
  const wv = target.webValidation;
  if (!wv) {
    return (
      <div className={styles.tabBody}>
        <p className={styles.muted}>Nessuna web validation disponibile per questo target.</p>
      </div>
    );
  }

  const cma = wv.candidateMatchAnalysis;
  const ks = wv.keywordSet;
  const runs = (wv.evidenceRuns ?? []) as EvidenceRun[];
  const summary = wv.summary;
  const hasError = Boolean(wv.candidateMatchError);

  return (
    <div className={styles.tabBody}>
      <div className={styles.grid2}>
        {/* Domain resolution */}
        <div className={`${styles.card} ${styles.cardCompact}`}>
          <p className={styles.lab}>Domain resolution</p>
          <dl className={styles.kv}>
            <dt>selectedDomain</dt>
            <dd className={styles.mono}>{wv.selectedDomain ?? '—'}</dd>
            <dt>domainConfidence <HiddenField label="domainConfidence" /></dt>
            <dd>{wv.domainConfidence ?? '—'}</dd>
            <dt>domainScore <HiddenField label="domainScore" /></dt>
            <dd>{wv.domainScore ?? '—'}</dd>
            <dt>identityState <HiddenField label="identityState" /></dt>
            <dd>{wv.webValidationState}</dd>
          </dl>
        </div>

        {/* Verdetto pipeline */}
        <div className={`${styles.card} ${styles.cardCompact}`}>
          <p className={styles.lab}>Verdetto pipeline</p>
          <dl className={styles.kv}>
            <dt>webValidationState</dt>
            <dd>{wv.webValidationState}</dd>
            <dt>finalAction</dt>
            <dd>{wv.finalAction}</dd>
            <dt>webScore / conf <HiddenField label="webScore / webConfidence" /></dt>
            <dd>{wv.webScore}{wv.webConfidence ? ` · ${wv.webConfidence}` : ''}</dd>
            <dt>freshness <HiddenField label="freshness" /></dt>
            <dd>
              <span className={`${styles.fresh} ${freshnessClass(wv.freshness)}`}>{freshnessLabel(wv.freshness)}</span>
              {expiresShort(wv.expiresAt) ? <small className={styles.muted}> · exp {expiresShort(wv.expiresAt)}</small> : null}
            </dd>
          </dl>
        </div>
      </div>

      {/* keyword set */}
      {ks ? (
        <div className={styles.card}>
          <p className={styles.lab}>Keyword set estratte dalla strategia <HiddenField label="keywordSet" /></p>
          {ks.intentLabel ? <p className={styles.cardSub} style={{ margin: '0 0 6px' }}>Intent: <b>{ks.intentLabel}</b></p> : null}
          <div className={styles.keywordset}>
            {(ks.coreTerms ?? []).map((t: string) => <span key={`c-${t}`} className={styles.kw}>{t}</span>)}
            {(ks.adjacentTerms ?? []).map((t: string) => <span key={`a-${t}`} className={`${styles.kw} ${styles.kwAdj}`}>{t}</span>)}
            {(ks.negativeTerms ?? []).map((t: string) => <span key={`n-${t}`} className={`${styles.kw} ${styles.kwNeg}`}>{t}</span>)}
          </div>
        </div>
      ) : null}

      {/* evidence runs */}
      <div className={styles.card}>
        <h3 className={styles.cardTitle}>Evidence runs <HiddenField label="evidenceRuns[]" /></h3>
        <p className={styles.cardSub}>Una riga per keyword/bucket: score di match sul dominio selezionato.</p>
        {runs.length === 0 ? (
          <p className={styles.muted}>Nessuna evidence run.</p>
        ) : (
          <div className={styles.rowsList}>
            {runs.map((r, i) => (
              <div key={`${r.bucket}-${r.term}-${i}`} className={`${styles.rowline} ${styles.rowlineNeutral}`}>
                <div>
                  <div className={styles.rlMain}>{r.term} <small className={styles.muted}>· {r.bucket}</small></div>
                  <div className={styles.rlSub}>
                    {r.error ? <span style={{ color: 'var(--color-danger-hover)' }}>{r.error}</span> : `${r.resultCount} risultati${r.matched ? ' · match' : ''}`}
                  </div>
                </div>
                <span className={`${styles.rlVal} ${runScoreClass(r.bestScore)}`}>{runScoreText(r)}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* summary */}
      {summary ? (
        <div className={styles.card}>
          <p className={styles.lab}>Summary <HiddenField label="summary" /></p>
          <div className={styles.summaryGrid}>
            <div className={styles.summaryCell}><span className={styles.summaryK}>Score</span><span className={styles.summaryV}>{summary.score}</span></div>
            <div className={styles.summaryCell}><span className={styles.summaryK}>Confidence</span><span className={styles.summaryV}>{summary.confidence}</span></div>
            <div className={styles.summaryCell}><span className={styles.summaryK}>Sector evidence</span><span className={styles.summaryV}>{summary.sectorEvidenceScore}</span></div>
            <div className={styles.summaryCell}><span className={styles.summaryK}>Coverage</span><span className={styles.summaryV}>{summary.coverageScore}</span></div>
            <div className={styles.summaryCell}><span className={styles.summaryK}>Domain</span><span className={styles.summaryV}>{summary.domainScore}</span></div>
            <div className={styles.summaryCell}><span className={styles.summaryK}>Negative penalty</span><span className={styles.summaryV}>{summary.negativePenalty}</span></div>
            <div className={styles.summaryCell}><span className={styles.summaryK}>Core match</span><span className={styles.summaryV}>{summary.coreMatches}/{summary.totalCoreTerms}</span></div>
            <div className={styles.summaryCell}><span className={styles.summaryK}>Adjacent match</span><span className={styles.summaryV}>{summary.adjacentMatches}/{summary.totalAdjacentTerms}</span></div>
          </div>
        </div>
      ) : null}

      {/* candidate match analysis */}
      <div className={styles.card}>
        <h3 className={styles.cardTitle}>Candidate match analysis <HiddenField label="candidateMatchAnalysis" /></h3>
        <p className={styles.cardSub}>Verdetto LLM sulla corrispondenza azienda↔perimetro. Payload intero nel T8.</p>

        {hasError ? (
          <div className={styles.dangerBox} role="alert">
            <Icon name="triangle-alert" size={16} />
            <span><b>candidateMatchError.</b> {wv.candidateMatchError}</span>
          </div>
        ) : null}

        {cma?.rationale || wv.finalDecision?.reason ? (
          <div className={`${styles.rationale} ${styles.rationaleAccent}`} style={hasError ? { marginTop: 12 } : undefined}>
            {cma?.rationale ? <><b>Rationale LLM.</b> {cma.rationale}</> : <><b>finalDecision.</b> {wv.finalDecision.reason}</>}
          </div>
        ) : null}

        <dl className={styles.kv} style={{ marginTop: 12 }}>
          <dt>verdict / confidence</dt>
          <dd>{cma ? `${cma.verdict} · ${cma.confidence}` : '—'}</dd>
          <dt>recommendedAction</dt>
          <dd>{cma?.recommendedAction ?? '—'}</dd>
          <dt>analystVerdict <HiddenField label="analystVerdict / Action / Confidence" /></dt>
          <dd>{[wv.analystVerdict, wv.analystAction, wv.analystConfidence].filter(Boolean).join(' · ') || '—'}</dd>
          <dt>llmModel / prompt</dt>
          <dd className={styles.mono}>{[wv.llmModel ?? wv.llmModelId, wv.llmPromptId].filter(Boolean).join(' · ') || '—'}</dd>
          <dt>updatedBy</dt>
          <dd>{formatDateTime(wv.updatedAt)}{wv.updatedByEmail ? ` · ${wv.updatedByEmail}` : ''}</dd>
        </dl>

        {cma?.evidenceFor?.length || cma?.evidenceAgainst?.length || cma?.negativeSignals?.length ? (
          <div className={styles.grid2} style={{ marginTop: 12 }}>
            {cma?.evidenceFor?.length ? (
              <div>
                <p className={styles.lab} style={{ margin: '0 0 6px' }}>Evidence for</p>
                <ul className={styles.rowsList} style={{ listStyle: 'none', padding: 0, gap: 4 }}>
                  {cma.evidenceFor.map((e: string, i: number) => <li key={i} className={styles.rlSub} style={{ paddingLeft: 12, position: 'relative' }}>{e}</li>)}
                </ul>
              </div>
            ) : null}
            {cma?.evidenceAgainst?.length || cma?.negativeSignals?.length ? (
              <div>
                <p className={styles.lab} style={{ margin: '0 0 6px' }}>Evidence against / negative</p>
                <ul className={styles.rowsList} style={{ listStyle: 'none', padding: 0, gap: 4 }}>
                  {(cma?.evidenceAgainst ?? []).map((e: string, i: number) => <li key={`a-${i}`} className={styles.rlSub} style={{ paddingLeft: 12 }}>{e}</li>)}
                  {(cma?.negativeSignals ?? []).map((e: string, i: number) => <li key={`n-${i}`} className={styles.rlSub} style={{ paddingLeft: 12, color: 'var(--color-danger-hover)' }}>{e}</li>)}
                </ul>
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    </div>
  );
}
